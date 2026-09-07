package main

// main_register_pve.go — V2 Phase 5 (PR 31c N8): the "register-pve"
// subcommand (plan J6·§3.3). It is the only registration path for a Proxmox
// VE connection (E-3 — no v1 source row exists to backfill), and it seeds the
// full control-plane chain in one shot: provider_connection → provider_context
// → 2 sealed SecretRefs → 2 credential bindings, then exits. Dispatched
// before any server startup path like the other offline subcommands.
//
// Credentials never touch the flag/env surface twice: the token secrets are
// read once (env first, flag fallback — claim 13), sealed immediately with
// util.EncryptSecretV2, and only ciphertexts are persisted. Re-running with
// the same --name is an idempotent upsert that refreshes the endpoint,
// metadata, and sealed materials (k8s backfill idempotency convention).

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"ops-admin/backend/config"
	"ops-admin/backend/internal/infra/adapter/proxmox"
	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/migrate"
	"ops-admin/backend/internal/infra/model"
	"ops-admin/backend/store"
	"ops-admin/backend/util"

	"gorm.io/gorm"
)

// Environment variables the runbook (v2-phase1/pve-provisioning.md) sources
// from the untracked backend/data/p5-proxmox.env — the only file home for
// real connection material (plan §E-1 계약 2).
const (
	pveROTokenSecretEnv  = "OPS_ADMIN_PVE_RO_TOKEN_SECRET"
	pveOpsTokenSecretEnv = "OPS_ADMIN_PVE_OPS_TOKEN_SECRET"
)

// pveRegisterTimeout bounds each registration probe. Registration is a
// one-shot CLI — a hung provider should fail the run, not the shell.
const pveRegisterTimeout = 15 * time.Second

type pveRegisterOptions struct {
	Name                  string // idempotency key — the upsert looks the chain up by its derived UID
	Endpoint              string
	TokenUser             string
	ROTokenID, OpsTokenID string
	// ROTokenSecretFlag/OpsTokenSecretFlag are the one-shot flag fallbacks;
	// the environment (pveROTokenSecretEnv/pveOpsTokenSecretEnv) wins over
	// them inside registerPVEInDB — the single precedence rule lives there.
	ROTokenSecretFlag         string
	OpsTokenSecretFlag        string
	ReverseProxy, InsecureTLS bool
}

// pveRegisterReport is the stdout artifact: chain identifiers and the
// resolved cluster identity. No credential material rides it (claim 13).
type pveRegisterReport struct {
	ConnectionUID string   `json:"connectionUid"`
	ContextUID    string   `json:"contextUid"`
	ClusterName   string   `json:"clusterName"`
	Standalone    bool     `json:"standalone"`
	Nodes         int      `json:"nodes"`
	Warnings      []string `json:"warnings,omitempty"`
}

// runRegisterPVE implements the "register-pve" command: parse flags, resolve
// secrets, open the database through the standard startup path (config →
// secret-key gate → migrate), then hand the chain write to registerPVEInDB.
func runRegisterPVE(args []string) int {
	flags := flag.NewFlagSet("register-pve", flag.ContinueOnError)
	configPath := flags.String("config", "config.yaml", "path to config.yaml")
	name := flags.String("name", "", "connection name — the upsert idempotency key (required)")
	endpoint := flags.String("endpoint", "", "PVE API endpoint, e.g. https://<host>:8006 (required)")
	tokenUser := flags.String("token-user", "root@pam", "PVE API token owner as user@realm")
	roTokenID := flags.String("ro-token-id", "", "read-only (inventory) token id (required)")
	opsTokenID := flags.String("ops-token-id", "", "operations token id (required)")
	roSecretFlag := flags.String("ro-token-secret", "", "read-only token secret (fallback — env "+pveROTokenSecretEnv+" wins)")
	opsSecretFlag := flags.String("ops-token-secret", "", "operations token secret (fallback — env "+pveOpsTokenSecretEnv+" wins)")
	reverseProxy := flags.Bool("reverse-proxy", false, "endpoint is a fixed reverse proxy — adapter node failover stays disabled")
	insecureTLS := flags.Bool("insecure-tls", false, "skip TLS verification (private-network premise — plan A7·MEDIUM-8)")
	if err := flags.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "register-pve: %v\n", err)
		return 1
	}
	if *name == "" || *endpoint == "" {
		fmt.Fprintln(os.Stderr, "register-pve: --name and --endpoint are required")
		return 1
	}
	if *roTokenID == "" || *opsTokenID == "" {
		fmt.Fprintln(os.Stderr, "register-pve: --ro-token-id and --ops-token-id are required")
		return 1
	}
	// Fail fast on missing secrets before touching config or the database —
	// the actual resolution runs again inside registerPVEInDB (single rule).
	for _, check := range []struct{ envKey, flagValue, flagName string }{
		{pveROTokenSecretEnv, *roSecretFlag, "--ro-token-secret"},
		{pveOpsTokenSecretEnv, *opsSecretFlag, "--ops-token-secret"},
	} {
		if _, err := pveResolveSecret(check.envKey, check.flagValue, check.flagName); err != nil {
			fmt.Fprintf(os.Stderr, "register-pve: %v\n", err)
			return 1
		}
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "register-pve: load config: %v\n", err)
		return 1
	}
	util.ConfigureCredentialKey(cfg.Security.CredentialKey)
	if err := util.EnsureSecretKeySource(cfg.Security.CredentialKey); err != nil {
		fmt.Fprintf(os.Stderr, "register-pve: secret key source: %v\n", err)
		return 1
	}
	db, err := store.NewDB(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "register-pve: connect db: %v\n", err)
		return 1
	}
	if err := migrate.Run(context.Background(), db); err != nil {
		fmt.Fprintf(os.Stderr, "register-pve: v2 schema migration: %v\n", err)
		return 1
	}

	report, err := registerPVEInDB(context.Background(), db, pveRegisterOptions{
		Name: *name, Endpoint: *endpoint, TokenUser: *tokenUser,
		ROTokenID: *roTokenID, OpsTokenID: *opsTokenID,
		ROTokenSecretFlag: *roSecretFlag, OpsTokenSecretFlag: *opsSecretFlag,
		ReverseProxy: *reverseProxy, InsecureTLS: *insecureTLS,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "register-pve: %v\n", err)
		return 1
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		fmt.Fprintf(os.Stderr, "register-pve: encode report: %v\n", err)
		return 1
	}
	return 0
}

// pveResolveSecret prefers the environment over the one-shot flag: the env
// value comes from a sourced file (no shell history), the flag is only the
// documented fallback. Whitespace is trimmed so a trailing newline in the env
// file cannot poison the sealed material.
func pveResolveSecret(envKey, flagValue, flagName string) (string, error) {
	secret := strings.TrimSpace(os.Getenv(envKey))
	if secret == "" {
		secret = strings.TrimSpace(flagValue)
	}
	if secret == "" {
		return "", fmt.Errorf("token secret is required: set %s or pass %s", envKey, flagName)
	}
	return secret, nil
}

// pveClusterIdentity is what /cluster/status says about the registration
// target: the cluster name, or — standalone (A11) — the only node name with
// the marker set.
type pveClusterIdentity struct {
	Name       string
	Standalone bool
	Nodes      int
	Quorate    bool
}

// registerPVEInDB writes the connection/context/secret/binding chain. Kept
// separate from flag parsing so the tests can drive it against an in-memory
// database and a mock PVE endpoint (J8 — harness only).
func registerPVEInDB(ctx context.Context, db *gorm.DB, opts pveRegisterOptions) (pveRegisterReport, error) {
	report := pveRegisterReport{}
	if opts.TokenUser == "" {
		opts.TokenUser = "root@pam"
	}
	if strings.TrimSpace(opts.Name) == "" || strings.TrimSpace(opts.Endpoint) == "" {
		return report, fmt.Errorf("name and endpoint are required")
	}
	// Secret precedence here, not in the flag handler — env first, flag
	// fallback (§3.3·claim 13). This is the single place the rule lives.
	roSecret, err := pveResolveSecret(pveROTokenSecretEnv, opts.ROTokenSecretFlag, "--ro-token-secret")
	if err != nil {
		return report, err
	}
	opsSecret, err := pveResolveSecret(pveOpsTokenSecretEnv, opts.OpsTokenSecretFlag, "--ops-token-secret")
	if err != nil {
		return report, err
	}
	roMaterial, err := pveTokenMaterial(opts.TokenUser, opts.ROTokenID, roSecret)
	if err != nil {
		return report, fmt.Errorf("read-only token: %w", err)
	}
	opsMaterial, err := pveTokenMaterial(opts.TokenUser, opts.OpsTokenID, opsSecret)
	if err != nil {
		return report, fmt.Errorf("operations token: %w", err)
	}

	configJSON := contract.JSONMap{}
	if opts.ReverseProxy {
		configJSON["deployment_mode"] = "reverse_proxy"
	}
	if opts.InsecureTLS {
		configJSON["tls_insecure"] = true
	}

	// 1) Validate — GET /version through the production adapter client, so
	// the registration smoke exercises the same transport posture (deployment
	// mode, TLS) the runtime sync will use (§13-1 0단계 재계약 — F8).
	view := contract.ConnectionView{
		ProviderType: proxmox.ProviderName,
		Endpoint:     opts.Endpoint,
		Config:       configJSON,
		Material:     map[string]string{"inventory": roMaterial},
	}
	adapter := proxmox.NewAdapter()
	if err := adapter.Validate(ctx, view); err != nil {
		return report, fmt.Errorf("validate: %w", err)
	}

	// 2) /cluster/status → cluster identity (standalone 폴백 A11).
	identity, err := pveResolveClusterIdentity(ctx, opts.Endpoint, roMaterial, opts.InsecureTLS)
	if err != nil {
		return report, fmt.Errorf("cluster identity: %w", err)
	}
	report.ClusterName = identity.Name
	report.Standalone = identity.Standalone
	report.Nodes = identity.Nodes
	if warning := pveEndpointPrivateWarn(opts.Endpoint, opts.InsecureTLS); warning != "" {
		report.Warnings = append(report.Warnings, warning)
	}

	// 3) upsert provider_connection — the UID is derived from --name so a
	// rerun lands on the same row (§3.3 멱등).
	connUID := pveRegisterUID(opts.Name, "")
	report.ConnectionUID = connUID
	var conn model.ProviderConnection
	err = db.Where("uid = ?", connUID).First(&conn).Error
	switch {
	case isRecordNotFound(err):
		conn = model.ProviderConnection{
			UID: connUID, ProviderType: proxmox.ProviderName,
			Name: opts.Name, Endpoint: opts.Endpoint,
			Status: "active", ConfigJSON: configJSON,
		}
		if err := db.Create(&conn).Error; err != nil {
			return report, fmt.Errorf("create provider_connection: %w", err)
		}
	case err != nil:
		return report, fmt.Errorf("load provider_connection: %w", err)
	default:
		// Struct + Select — config_json's column serializer applies to
		// model-field writes only (map updates double-encode; backfill 규약).
		patch := model.ProviderConnection{Name: opts.Name, Endpoint: opts.Endpoint, ConfigJSON: configJSON}
		if err := db.Model(&model.ProviderConnection{}).Where("id = ?", conn.ID).
			Select("name", "endpoint", "config_json").Updates(patch).Error; err != nil {
			return report, fmt.Errorf("update provider_connection: %w", err)
		}
	}

	// 4) upsert provider_context — Kind "cluster"; ExternalID is the cluster
	// name, or the only node name with the standalone marker (A11).
	ctxUID := pveRegisterUID(opts.Name, "context")
	report.ContextUID = ctxUID
	metadata := contract.JSONMap{"nodes": identity.Nodes, "quorum": identity.Quorate}
	if identity.Standalone {
		metadata = contract.JSONMap{"standalone": true}
	}
	var pctx model.ProviderContext
	err = db.Where("uid = ?", ctxUID).First(&pctx).Error
	switch {
	case isRecordNotFound(err):
		pctx = model.ProviderContext{
			UID: ctxUID, ConnectionID: conn.ID, Kind: "cluster",
			ExternalID: identity.Name, Name: opts.Name,
			Status: "active", MetadataJSON: metadata,
		}
		if err := db.Create(&pctx).Error; err != nil {
			return report, fmt.Errorf("create provider_context: %w", err)
		}
	case err != nil:
		return report, fmt.Errorf("load provider_context: %w", err)
	default:
		patch := model.ProviderContext{ExternalID: identity.Name, Name: opts.Name, Status: "active", MetadataJSON: metadata}
		if err := db.Model(&model.ProviderContext{}).Where("id = ?", pctx.ID).
			Select("external_id", "name", "status", "metadata_json").Updates(patch).Error; err != nil {
			return report, fmt.Errorf("update provider_context: %w", err)
		}
	}

	// 5) SecretRef 2건 — sealed here, re-sealed on every rerun (자격 갱신).
	roRef, err := pveUpsertSecretRef(db, pveRegisterUID(opts.Name, "ro-secret"),
		"register/pve/"+opts.Name+"/inventory", roMaterial)
	if err != nil {
		return report, err
	}
	opsRef, err := pveUpsertSecretRef(db, pveRegisterUID(opts.Name, "ops-secret"),
		"register/pve/"+opts.Name+"/operations", opsMaterial)
	if err != nil {
		return report, err
	}

	// 6) 바인딩 2행 — inventory는 RO 토큰, operations는 별도 RW 토큰을 지목한다.
	// 같은 SecretRef 공유 금지(판정 J5 — §14.3 권한 분리).
	if err := pveUpsertBinding(db, conn.ID, pctx.ID, contract.CredentialPurposeInventory, roRef.ID); err != nil {
		return report, err
	}
	if err := pveUpsertBinding(db, conn.ID, pctx.ID, contract.CredentialPurposeOperations, opsRef.ID); err != nil {
		return report, err
	}
	return report, nil
}

// pveTokenMaterial renders the sealed SecretRef blob (판정 J5): the client
// assembles the PVEAPIToken header from these three components. All three are
// required — a partial blob would fail only at first use.
func pveTokenMaterial(tokenUser, tokenID, tokenSecret string) (string, error) {
	material := pveTokenMaterialJSON{
		TokenUser: strings.TrimSpace(tokenUser), TokenID: strings.TrimSpace(tokenID),
		TokenSecret: strings.TrimSpace(tokenSecret),
	}
	if material.TokenUser == "" || material.TokenID == "" || material.TokenSecret == "" {
		return "", fmt.Errorf("incomplete credential (tokenUser, tokenID and tokenSecret are all required)")
	}
	b, err := json.Marshal(material)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

type pveTokenMaterialJSON struct {
	TokenUser   string `json:"tokenUser"`
	TokenID     string `json:"tokenID"`
	TokenSecret string `json:"tokenSecret"`
}

// pveRegisterUID derives the deterministic UID of one chain row from the
// registration name — SourceKeyUID 관례(salt) 승계 with a register-scoped
// domain so it can never collide with a backfilled UID.
func pveRegisterUID(name, salt string) string {
	sum := sha256.Sum256([]byte("register-pve|" + name + "|" + salt))
	return hex.EncodeToString(sum[:])[:32]
}

// pveUpsertSecretRef seals the material and creates or re-seals the SecretRef.
// KeyID is recorded from the envelope (§3.3 — v2 봉인·KeyID 기록).
func pveUpsertSecretRef(db *gorm.DB, uid, path, material string) (model.SecretRef, error) {
	var ref model.SecretRef
	sealed, err := util.EncryptSecretV2(material)
	if err != nil {
		return ref, fmt.Errorf("seal token material: %w", err)
	}
	keyID, err := pveEnvelopeKeyID(sealed)
	if err != nil {
		return ref, err
	}
	err = db.Where("uid = ?", uid).First(&ref).Error
	switch {
	case isRecordNotFound(err):
		ref = model.SecretRef{UID: uid, Backend: "internal", Path: path, KeyID: keyID, Ciphertext: sealed}
		if err := db.Create(&ref).Error; err != nil {
			return ref, fmt.Errorf("create secret_ref: %w", err)
		}
	case err != nil:
		return ref, fmt.Errorf("load secret_ref: %w", err)
	default:
		if err := db.Model(&model.SecretRef{}).Where("id = ?", ref.ID).
			Updates(map[string]any{"key_id": keyID, "ciphertext": sealed}).Error; err != nil {
			return ref, fmt.Errorf("refresh secret_ref: %w", err)
		}
	}
	return ref, nil
}

// pveUpsertBinding points one purpose at its SecretRef; a rerun re-points the
// row when the SecretRef UID changed or a token rotated.
func pveUpsertBinding(db *gorm.DB, connectionID, contextID uint, purpose string, secretRefID uint) error {
	var binding model.ProviderCredentialBinding
	err := db.Where("provider_connection_id = ? AND purpose = ?", connectionID, purpose).First(&binding).Error
	switch {
	case isRecordNotFound(err):
		binding = model.ProviderCredentialBinding{
			ProviderConnectionID: connectionID, ProviderContextID: &contextID,
			Purpose: purpose, SecretRefID: secretRefID, Status: "active",
		}
		if err := db.Create(&binding).Error; err != nil {
			return fmt.Errorf("create provider_credential_binding(%s): %w", purpose, err)
		}
	case err != nil:
		return fmt.Errorf("load provider_credential_binding(%s): %w", purpose, err)
	default:
		if binding.SecretRefID != secretRefID {
			if err := db.Model(&model.ProviderCredentialBinding{}).Where("id = ?", binding.ID).
				Updates(map[string]any{"secret_ref_id": secretRefID, "provider_context_id": contextID}).Error; err != nil {
				return fmt.Errorf("re-point provider_credential_binding(%s): %w", purpose, err)
			}
		}
	}
	return nil
}

// pveEnvelopeKeyID extracts the key id from a v2 envelope — the format is
// "v2:<key_id>:<base64url(nonce || ciphertext)>" (util/secretv2.go).
func pveEnvelopeKeyID(sealed string) (string, error) {
	rest, found := strings.CutPrefix(sealed, "v2:")
	if !found {
		return "", fmt.Errorf("sealed material is not a v2 envelope")
	}
	keyID, payload, ok := strings.Cut(rest, ":")
	if !ok || keyID == "" || payload == "" {
		return "", fmt.Errorf("sealed v2 envelope is malformed")
	}
	return keyID, nil
}

// pveClusterIdentity resolves the context ExternalID from /cluster/status
// (§3.3): the cluster row's name when present, or — A11 standalone — the
// only node row's name (no fake cluster identity). Uses the read-only
// credential; the header assembly mirrors the adapter's runtime client
// (§14.3 증류 계약 2) because the adapter surface is DB-free and unexported.
func pveResolveClusterIdentity(ctx context.Context, endpoint, roMaterial string, insecureTLS bool) (pveClusterIdentity, error) {
	var identity pveClusterIdentity
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if insecureTLS {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // private-network premise, plan A7
	}
	client := &http.Client{Transport: transport, Timeout: pveRegisterTimeout}
	probeURL := strings.TrimRight(endpoint, "/") + "/api2/json/cluster/status"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, probeURL, nil)
	if err != nil {
		return identity, err
	}
	header, err := pveAuthorizationHeader(roMaterial)
	if err != nil {
		return identity, err
	}
	req.Header.Set("Authorization", header)
	resp, err := client.Do(req)
	if err != nil {
		return identity, fmt.Errorf("GET /cluster/status: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		// Status only — the body may echo request internals; the credential never does.
		return identity, fmt.Errorf("GET /cluster/status: HTTP %d", resp.StatusCode)
	}
	var envelope struct {
		Data []struct {
			Type    string `json:"type"`
			Name    string `json:"name"`
			Quorate *int   `json:"quorate"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&envelope); err != nil {
		return identity, fmt.Errorf("decode /cluster/status: %w", err)
	}
	var firstNode string
	for _, entry := range envelope.Data {
		switch entry.Type {
		case "cluster":
			identity.Name = entry.Name
			identity.Quorate = entry.Quorate != nil && *entry.Quorate != 0
		case "node":
			identity.Nodes++
			if firstNode == "" {
				firstNode = entry.Name
			}
		}
	}
	if identity.Name != "" {
		return identity, nil
	}
	// A11 standalone 폴백: cluster행 부재 + node행 1개 → node명으로 신원 확정.
	if identity.Nodes == 1 && firstNode != "" {
		identity.Name = firstNode
		identity.Standalone = true
		return identity, nil
	}
	return identity, fmt.Errorf("/cluster/status reported %d node rows without a cluster row — cannot resolve a cluster identity", identity.Nodes)
}

// pveAuthorizationHeader assembles `PVEAPIToken=<tokenUser>!<tokenID>=<tokenSecret>`
// from the sealed blob components. The value flows into the request header
// only — never into logs, errors, or reports (claim 13).
func pveAuthorizationHeader(material string) (string, error) {
	var blob pveTokenMaterialJSON
	if err := json.Unmarshal([]byte(material), &blob); err != nil {
		return "", fmt.Errorf("credential material is not the expected {tokenUser,tokenID,tokenSecret} JSON blob")
	}
	if blob.TokenUser == "" || blob.TokenID == "" || blob.TokenSecret == "" {
		return "", fmt.Errorf("credential material is incomplete")
	}
	return "PVEAPIToken=" + blob.TokenUser + "!" + blob.TokenID + "=" + blob.TokenSecret, nil
}

// pveEndpointPrivateWarn is the MEDIUM-8 defense line: --insecure-tls
// presumes a private-network (RFC1918) endpoint. A public IP on the same
// flag combination gets a loud warning; a hostname cannot be classified
// without DNS, so it warns too — the operator confirms the premise instead
// of the code assuming it. Empty string means no warning.
func pveEndpointPrivateWarn(endpoint string, insecureTLS bool) string {
	if !insecureTLS {
		return ""
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Hostname() == "" {
		return fmt.Sprintf("register-pve: --insecure-tls presumes a private-network (RFC1918) endpoint, but %q has no parsable host", endpoint)
	}
	host := parsed.Hostname()
	if ip := net.ParseIP(host); ip != nil && (ip.IsPrivate() || ip.IsLoopback()) {
		return ""
	}
	return fmt.Sprintf("register-pve: --insecure-tls presumes a private-network (RFC1918) endpoint, but host %q is not a private address — TLS verification bypass on a routable path is a standing risk", host)
}

// isRecordNotFound mirrors the backfill's switch guard.
func isRecordNotFound(err error) bool {
	return err != nil && err == gorm.ErrRecordNotFound
}
