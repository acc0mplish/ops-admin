package main

// main_register_k8s.go — V2 Phase 6 H0 (phase6-plan §3.2·§J9): the
// "register-k8s" subcommand, the registration path a Kubernetes cluster keeps
// once H2 retires POST /k8s/cluster/add. It seeds the control-plane chain in
// one shot: provider_connection → provider_context → sealed kubeconfig
// SecretRef → inventory credential binding, then exits. Dispatched before any
// server startup path like the other offline subcommands.
//
// The kubeconfig travels by path only: the file is read once, validated
// against /version through the production adapter client, and sealed with
// util.EncryptSecretV2 into the secret_ref row inside the chain transaction.
// The backfill's P-class plaintext exemption (backfill.go:129) is NOT
// inherited — the new path is sealed from day one (plan §12 #16·C58), and no
// kubeconfig fragment reaches flags, logs, errors, or the stdout report.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"ops-admin/backend/config"
	"ops-admin/backend/internal/infra/adapter/kubernetes"
	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/migrate"
	"ops-admin/backend/internal/infra/model"
	"ops-admin/backend/store"
	"ops-admin/backend/util"

	"gorm.io/gorm"
)

// Connection-mode values recorded under the ConfigJSON fixed key
// "connection_mode" — the adapter's dialContextFor consumes the same key, so
// the registered posture is the runtime posture (plan §J9 F6). gateway_id is
// stored as a string because the adapter reads it with a string type
// assertion; monitor_datasource_id follows the same string-id convention
// (backfill strconvID).
const (
	k8sModeDirect  = "direct"
	k8sModeGateway = "gateway"
)

type k8sRegisterOptions struct {
	Name string // idempotency key — the chain is looked up by its derived UID
	// KubeconfigPath is the only credential input: a file path, never the
	// kubeconfig content itself (CLI 규약 — 시크릿 값은 인자로 받지 않는다).
	KubeconfigPath      string
	Env                 string
	ConnectionMode      string // direct|gateway — ConfigJSON fixed key
	GatewayID           string
	MonitorDatasourceID string
	InsecureTLS         bool
}

// k8sRegisterReport is the stdout artifact (pveRegisterReport 선례 — the
// connectionUid field name is the C39′ acquisition contract). No kubeconfig
// content rides it.
type k8sRegisterReport struct {
	ConnectionUID string `json:"connectionUid"`
	ContextUID    string `json:"contextUid"`
}

// runRegisterK8s implements the "register-k8s" command: parse flags, confirm
// the kubeconfig path exists, open the database through the standard startup
// path (config → secret-key gate → migrate), then hand the chain write to
// registerK8sInDB.
func runRegisterK8s(args []string) int {
	flags := flag.NewFlagSet("register-k8s", flag.ContinueOnError)
	configPath := flags.String("config", "config.yaml", "path to config.yaml")
	name := flags.String("name", "", "connection name — the upsert idempotency key (required)")
	kubeconfigPath := flags.String("kubeconfig", "", "path to the kubeconfig file (required — the file is read and sealed, never echoed)")
	env := flags.String("env", "", "environment label recorded under ConfigJSON[\"env\"]")
	connectionMode := flags.String("connection-mode", k8sModeDirect, "direct|gateway (ConfigJSON fixed key)")
	gatewayID := flags.String("gateway-id", "", "gateway id — required when --connection-mode gateway")
	monitorDatasourceID := flags.String("monitor-datasource-id", "", "monitoring datasource id recorded under ConfigJSON[\"monitor_datasource_id\"]")
	insecureTLS := flags.Bool("insecure-tls", false, "record the TLS-bypass posture under ConfigJSON[\"tls_insecure\"] — the kubeconfig's own insecure-skip-tls-verify still governs the wire")
	if err := flags.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "register-k8s: %v\n", err)
		return 1
	}
	if *name == "" || *kubeconfigPath == "" {
		fmt.Fprintln(os.Stderr, "register-k8s: --name and --kubeconfig are required")
		return 1
	}
	if *connectionMode != k8sModeDirect && *connectionMode != k8sModeGateway {
		fmt.Fprintf(os.Stderr, "register-k8s: --connection-mode must be %s or %s\n", k8sModeDirect, k8sModeGateway)
		return 1
	}
	if *connectionMode == k8sModeGateway && *gatewayID == "" {
		fmt.Fprintln(os.Stderr, "register-k8s: --gateway-id is required when --connection-mode gateway")
		return 1
	}
	// Pre-flight existence check only — the single credential read happens
	// inside registerK8sInDB. A missing file fails before config or the
	// database are touched (pve 선례 fail-fast).
	if _, err := os.Stat(*kubeconfigPath); err != nil {
		fmt.Fprintf(os.Stderr, "register-k8s: kubeconfig path: %v\n", err)
		return 1
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "register-k8s: load config: %v\n", err)
		return 1
	}
	util.ConfigureCredentialKey(cfg.Security.CredentialKey)
	if err := util.EnsureSecretKeySource(cfg.Security.CredentialKey); err != nil {
		fmt.Fprintf(os.Stderr, "register-k8s: secret key source: %v\n", err)
		return 1
	}
	db, err := store.NewDB(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "register-k8s: connect db: %v\n", err)
		return 1
	}
	if err := migrate.Run(context.Background(), db); err != nil {
		fmt.Fprintf(os.Stderr, "register-k8s: v2 schema migration: %v\n", err)
		return 1
	}

	report, err := registerK8sInDB(context.Background(), db, k8sRegisterOptions{
		Name: *name, KubeconfigPath: *kubeconfigPath, Env: *env,
		ConnectionMode: *connectionMode, GatewayID: *gatewayID,
		MonitorDatasourceID: *monitorDatasourceID, InsecureTLS: *insecureTLS,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "register-k8s: %v\n", err)
		return 1
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		fmt.Fprintf(os.Stderr, "register-k8s: encode report: %v\n", err)
		return 1
	}
	return 0
}

// registerK8sInDB validates the kubeconfig against the cluster (/version) and
// writes the connection/context/secret/binding chain. Kept separate from flag
// parsing so the tests can drive it against an in-memory database and a mock
// API server (pve 선례 — harness only).
func registerK8sInDB(ctx context.Context, db *gorm.DB, opts k8sRegisterOptions) (k8sRegisterReport, error) {
	report := k8sRegisterReport{}
	if strings.TrimSpace(opts.Name) == "" {
		return report, fmt.Errorf("name is required")
	}
	if strings.TrimSpace(opts.KubeconfigPath) == "" {
		return report, fmt.Errorf("kubeconfig path is required")
	}
	if opts.ConnectionMode == "" {
		opts.ConnectionMode = k8sModeDirect
	}
	if opts.ConnectionMode != k8sModeDirect && opts.ConnectionMode != k8sModeGateway {
		return report, fmt.Errorf("connection mode must be %s or %s", k8sModeDirect, k8sModeGateway)
	}
	if opts.ConnectionMode == k8sModeGateway && strings.TrimSpace(opts.GatewayID) == "" {
		return report, fmt.Errorf("gateway id is required in gateway mode")
	}

	// The single credential read: the kubeconfig content lives only as this
	// in-memory string until the transaction seals it into secret_ref (§12 #16).
	material, err := os.ReadFile(opts.KubeconfigPath)
	if err != nil {
		return report, fmt.Errorf("read kubeconfig: %w", err)
	}

	configJSON := contract.JSONMap{"connection_mode": opts.ConnectionMode}
	if strings.TrimSpace(opts.Env) != "" {
		configJSON["env"] = opts.Env
	}
	if strings.TrimSpace(opts.GatewayID) != "" {
		configJSON["gateway_id"] = opts.GatewayID
	}
	if strings.TrimSpace(opts.MonitorDatasourceID) != "" {
		configJSON["monitor_datasource_id"] = opts.MonitorDatasourceID
	}
	if opts.InsecureTLS {
		configJSON["tls_insecure"] = true
	}

	// Validate first — /version through the production adapter client, so the
	// registration smoke exercises the same transport posture (gateway mode
	// gate, Material["inventory"] resolve) the runtime sync will use (pve
	// 선례: 네트워크 probe는 트랜잭션 밖). A failed validate writes nothing,
	// and the chain write below is a single transaction — a half-written
	// chain is impossible either way (CLI 규약: 검증 실패 시 롤백).
	//
	// Gateway mode is passed through as recorded: today the adapter rejects
	// it (A4 — the gateway hop lands with M2), so a gateway registration
	// fails loudly here instead of sealing a posture the runtime cannot dial.
	view := contract.ConnectionView{
		ProviderType: kubernetes.ProviderName,
		Config:       configJSON,
		Material:     map[string]string{"inventory": string(material)},
	}
	adapter := kubernetes.NewAdapter()
	if err := adapter.Validate(ctx, view); err != nil {
		return report, fmt.Errorf("validate: %w", err)
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		// 1) upsert provider_connection — the UID is derived from --name so a
		// rerun lands on the same row (CLI 규약 멱등).
		connUID := k8sRegisterUID(opts.Name, "")
		report.ConnectionUID = connUID
		var conn model.ProviderConnection
		err := tx.Where("uid = ?", connUID).First(&conn).Error
		switch {
		case isRecordNotFound(err):
			conn = model.ProviderConnection{
				UID: connUID, ProviderType: kubernetes.ProviderName,
				Name: opts.Name, Status: "active", ConfigJSON: configJSON,
			}
			if err := tx.Create(&conn).Error; err != nil {
				return fmt.Errorf("create provider_connection: %w", err)
			}
		case err != nil:
			return fmt.Errorf("load provider_connection: %w", err)
		default:
			// Struct + Select — config_json's column serializer applies to
			// model-field writes only (map updates double-encode; backfill 규약).
			patch := model.ProviderConnection{Name: opts.Name, ConfigJSON: configJSON}
			if err := tx.Model(&model.ProviderConnection{}).Where("id = ?", conn.ID).
				Select("name", "config_json").Updates(patch).Error; err != nil {
				return fmt.Errorf("update provider_connection: %w", err)
			}
		}

		// 2) upsert provider_context — Kind "cluster". The k8s Validate
		// contract carries no identity probe, so the registration name stands
		// in as the external id (판단 기록 — I-a S5 필드 조달 계약에서 보강
		// 여지, F6).
		ctxUID := k8sRegisterUID(opts.Name, "context")
		report.ContextUID = ctxUID
		var pctx model.ProviderContext
		err = tx.Where("uid = ?", ctxUID).First(&pctx).Error
		switch {
		case isRecordNotFound(err):
			pctx = model.ProviderContext{
				UID: ctxUID, ConnectionID: conn.ID, Kind: "cluster",
				ExternalID: opts.Name, Name: opts.Name, Status: "active",
			}
			if err := tx.Create(&pctx).Error; err != nil {
				return fmt.Errorf("create provider_context: %w", err)
			}
		case err != nil:
			return fmt.Errorf("load provider_context: %w", err)
		default:
			patch := model.ProviderContext{ExternalID: opts.Name, Name: opts.Name, Status: "active"}
			if err := tx.Model(&model.ProviderContext{}).Where("id = ?", pctx.ID).
				Select("external_id", "name", "status").Updates(patch).Error; err != nil {
				return fmt.Errorf("update provider_context: %w", err)
			}
		}

		// 3) SecretRef — the kubeconfig is sealed here and re-sealed on every
		// rerun (kubeconfig rotation); the UID is stable across reruns.
		ref, err := k8sUpsertSecretRef(tx, k8sRegisterUID(opts.Name, "secret"),
			"register/k8s/"+opts.Name+"/inventory", string(material))
		if err != nil {
			return err
		}

		// 4) binding — inventory 1건 (CLI 규약). k8s는 자격이 kubeconfig 단일
		// 물질이라 operations 별도 자격이 존재하지 않는다.
		if err := k8sUpsertBinding(tx, conn.ID, pctx.ID, contract.CredentialPurposeInventory, ref.ID); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return report, err
	}
	return report, nil
}

// k8sRegisterUID derives the deterministic UID of one chain row from the
// registration name — SourceKeyUID 관례(salt) 승계 with a register-scoped
// domain so it can never collide with a backfilled UID (C59 — the k8s_cluster
// derived UID stays out of this file; S1b retires that coupling).
func k8sRegisterUID(name, salt string) string {
	sum := sha256.Sum256([]byte("register-k8s|" + name + "|" + salt))
	return hex.EncodeToString(sum[:])[:32]
}

// k8sUpsertSecretRef seals the kubeconfig material and creates or re-seals
// the SecretRef. KeyID is recorded from the envelope (v2 봉인·KeyID 기록).
func k8sUpsertSecretRef(db *gorm.DB, uid, path, material string) (model.SecretRef, error) {
	var ref model.SecretRef
	sealed, err := util.EncryptSecretV2(material)
	if err != nil {
		return ref, fmt.Errorf("seal kubeconfig: %w", err)
	}
	keyID, err := k8sEnvelopeKeyID(sealed)
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

// k8sUpsertBinding points one purpose at its SecretRef; a rerun re-points the
// row when the SecretRef UID changed or the kubeconfig was rotated.
func k8sUpsertBinding(db *gorm.DB, connectionID, contextID uint, purpose string, secretRefID uint) error {
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

// k8sEnvelopeKeyID extracts the key id from a v2 envelope — the format is
// "v2:<key_id>:<base64url(nonce || ciphertext)>" (util/secretv2.go).
func k8sEnvelopeKeyID(sealed string) (string, error) {
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
