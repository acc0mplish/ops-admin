//go:build e2e

package slicea

// Fixture registration + sync legs of the N16 harness — split out of
// harness_test.go to keep each file under the 800-line cap (the I-a S7
// redesign of registerCluster grew the harness; phase6-plan §J5 S7). The
// build tag and package stay identical, so the harness API is unchanged.

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"ops-admin/backend/util"
)

// --- fixture registration + sync (same paths run-fixture.sh drives) --------

// registerCluster seeds the register-shaped V2 chain (provider_connection →
// provider_context → sealed SecretRef → inventory + operations bindings)
// directly into the dedicated schema — the raw v1 k8s_cluster INSERT with the
// plaintext kubeconfig (§4.4 P-class detour) and the backfill UID derivation
// it relied on were retired with the §5.4 backfill itself (V2 Phase 6 I-a,
// phase6-plan §J5 S7). The kubeconfig is sealed here under the same master
// keys the child receives (childEnv), so the broker's inventory resolve
// round-trips inside the child. The operations binding is recreated by this
// fixture because the retired backfill was its only producer (J12(1)) —
// without it the engine's restart leg classifies as credential_error. The
// UID derives from a fixture-local domain so it can never collide with a
// register-k8s chain.
func (h *harness) registerCluster(t *testing.T) {
	t.Helper()
	kubeconfig, err := exec.Command("kubectl", "config", "view", "--minify", "--flatten", "--context", h.kubeContext).Output()
	if err != nil {
		t.Fatalf("minify kubeconfig: %v", err)
	}
	apiServer, err := exec.Command("kubectl", "config", "view", "--minify", "--flatten", "--context", h.kubeContext,
		"-o", `jsonpath={.clusters[0].cluster.server}`).Output()
	if err != nil {
		t.Fatalf("kubeconfig api server: %v", err)
	}
	verRaw, err := exec.Command("kubectl", "--context", h.kubeContext, "version", "-o", "json").Output()
	if err != nil {
		t.Fatalf("kubectl version: %v", err)
	}
	var ver struct {
		ServerVersion struct {
			GitVersion string `json:"gitVersion"`
		} `json:"serverVersion"`
	}
	if err := json.Unmarshal(verRaw, &ver); err != nil {
		t.Fatalf("decode kubectl version: %v", err)
	}
	nodesRaw, err := exec.Command("kubectl", "--context", h.kubeContext, "get", "nodes", "-o", "json").Output()
	if err != nil {
		t.Fatalf("kubectl get nodes: %v", err)
	}
	var nodes struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(nodesRaw, &nodes); err != nil {
		t.Fatalf("decode nodes: %v", err)
	}

	// Seal under the child's key chain: EncryptSecretV2 reads
	// OPS_SECRET_MASTER_KEYS, and childEnv passes the same value to the
	// child. The plaintext kubeconfig lives only in this process memory and
	// inside the sealed envelope (§12 #16).
	t.Setenv("OPS_SECRET_MASTER_KEYS", h.masterKeys)
	sealed, err := util.EncryptSecretV2(string(kubeconfig))
	if err != nil {
		t.Fatalf("seal kubeconfig: %v", err)
	}
	keyID, payload, ok := strings.Cut(strings.TrimPrefix(sealed, "v2:"), ":")
	if !ok || keyID == "" || payload == "" {
		t.Fatalf("sealed kubeconfig is not a v2 envelope")
	}

	sum := sha256.Sum256([]byte("e2e-slicea|" + h.kubeContext))
	h.connUID = hex.EncodeToString(sum[:])[:32]
	name := h.kubeContext
	endpoint := strings.TrimSpace(string(apiServer))
	if _, err := h.db.Exec(`INSERT INTO provider_connection
		(uid, provider_type, name, endpoint, status, version, config_json, stale_source, created_at, updated_at)
		VALUES (?, 'kubernetes', ?, ?, 'active', ?, ?, 0, NOW(3), NOW(3))`,
		h.connUID, name, endpoint, ver.ServerVersion.GitVersion,
		fmt.Sprintf(`{"env":"dev","connection_mode":"direct","node_count":%d}`, len(nodes.Items))); err != nil {
		t.Fatalf("insert provider_connection: %v", err)
	}
	if _, err := h.db.Exec(`INSERT INTO provider_context
		(uid, connection_id, kind, external_id, name, status, created_at, updated_at)
		SELECT ?, id, 'cluster', ?, ?, 'active', NOW(3), NOW(3) FROM provider_connection WHERE uid = ?`,
		h.connUID+"-ctx", name, name, h.connUID); err != nil {
		t.Fatalf("insert provider_context: %v", err)
	}
	if _, err := h.db.Exec(`INSERT INTO secret_ref
		(uid, backend, path, key_id, ciphertext, created_at, updated_at)
		VALUES (?, 'internal', 'e2e-slicea/inventory', ?, ?, NOW(3), NOW(3))`,
		h.connUID+"-ref", keyID, sealed); err != nil {
		t.Fatalf("insert secret_ref: %v", err)
	}
	for _, purpose := range []string{"inventory", "operations"} {
		if _, err := h.db.Exec(`INSERT INTO provider_credential_binding
			(provider_connection_id, provider_context_id, purpose, secret_ref_id, status)
			SELECT c.id, p.id, ?, r.id, 'active'
			FROM provider_connection c
			JOIN provider_context p ON p.connection_id = c.id
			JOIN secret_ref r ON r.uid = ?
			WHERE c.uid = ?`, purpose, h.connUID+"-ref", h.connUID); err != nil {
			t.Fatalf("insert provider_credential_binding(%s): %v", purpose, err)
		}
	}
	fmt.Printf("slicea: cluster registered connection_uid=%s\n", h.connUID)
}

// runSyncCLI runs the sync-inventory subcommand against the dedicated schema:
// one shadow sync run against the seeded V2 chain — the §5.4 backfill it once
// ran first was retired in V2 Phase 6 I-a, so this fixture seeds the chain
// itself (registerCluster, J12(1) bindings included).
func (h *harness) runSyncCLI(t *testing.T) {
	t.Helper()
	cmd := exec.Command(h.binPath, "sync-inventory", "--config", h.configPath, "--connection", h.connUID, "--data", h.dataDir)
	cmd.Dir = h.runDir
	cmd.Env = h.childEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("sync-inventory failed: %v\n%s", err, string(out))
	}
	var report struct {
		Status   string         `json:"status"`
		Seen     int            `json:"seenCount"`
		Outcomes map[string]int `json:"outcomes"`
	}
	// The CLI prints the JSON report first, then a trailing "report artifact"
	// line — decode just the leading JSON value.
	if err := json.NewDecoder(bytes.NewReader(out)).Decode(&report); err != nil {
		t.Fatalf("decode sync report: %v\n%s", err, truncate(string(out), 300))
	}
	fmt.Printf("slicea: sync-inventory status=%s seen=%d outcomes=%v\n", report.Status, report.Seen, report.Outcomes)

	// J12(1) evidence in the dedicated schema: the operations binding must
	// exist or every execution would fail credential_error.
	var purposes string
	if err := h.db.QueryRow(`SELECT GROUP_CONCAT(purpose ORDER BY purpose)
		FROM provider_credential_binding b
		JOIN provider_connection c ON c.id = b.provider_connection_id
		WHERE c.uid = ?`, h.connUID).Scan(&purposes); err != nil {
		t.Fatalf("probe credential bindings: %v", err)
	}
	if !strings.Contains(purposes, "operations") {
		t.Fatalf("operations purpose binding missing from the seeded chain (got %q)", purposes)
	}
	fmt.Printf("slicea: credential purposes=%s\n", purposes)
}
