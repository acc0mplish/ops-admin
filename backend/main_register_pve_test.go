package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"gorm.io/gorm"
	"ops-admin/backend/internal/infra/migrate"
	inframodel "ops-admin/backend/internal/infra/model"
	"ops-admin/backend/internal/testutil"
	"ops-admin/backend/store"
	"ops-admin/backend/util"
)

// register-pve 테스트 하니스 — plan phase5 F 배치(§5)·판정 J6·§3.3. mock PVE는
// 테스트 측정 기구일 뿐 게이트 대체가 아니다(판정 J8 — 실엔드포인트 증명은 §13-1).
// 모의 시크릿 리터럴은 전부 합성값이며 32자 미만으로 keyword-entropy 스캔 규칙을
// 통과한다(claim 14 형식 — allowlist 불요).

// pveRegisterMock is a minimal PVE API2 surface: /version + /cluster/status
// only — exactly what the registration Validate (§13-1) touches. It records
// the Authorization headers so the tests can pin that the read-only token
// rode every probe and the operations token never left the sealed store.
type pveRegisterMock struct {
	mu      sync.Mutex
	auth    []string
	paths   []string
	entries string // raw JSON array served as the /cluster/status data payload
}

func (m *pveRegisterMock) serve(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.mu.Lock()
		m.auth = append(m.auth, r.Header.Get("Authorization"))
		m.paths = append(m.paths, r.URL.Path)
		entries := m.entries
		m.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/version") {
			_, _ = w.Write([]byte(`{"data":{"version":"8.2.4","reporelease":"8.2","release":"8.2"}}`))
			return
		}
		if strings.HasSuffix(r.URL.Path, "/cluster/status") {
			_, _ = w.Write([]byte(`{"data":` + entries + `}`))
			return
		}
		http.Error(w, `{"data":null}`, http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func clusterEntries() string {
	return `[{"type":"cluster","name":"pve-cluster-mock","quorate":1,"id":"cluster","nodes":2},` +
		`{"type":"node","name":"pve1","ip":"192.168.250.11","online":1,"id":"node/pve1"},` +
		`{"type":"node","name":"pve2","ip":"192.168.250.12","online":1,"id":"node/pve2"}]`
}

func standaloneEntries() string {
	// A11 실측 형상: standalone 노드는 cluster행 없이 node행만 반환한다.
	return `[{"type":"node","name":"pve1","ip":"127.0.0.1","online":1,"id":"node/pve1"}]`
}

// newRegisterPVETestDB opens the in-memory database with both schema layers
// the CLI touches: the V2 tables (migrate.Run) and the v1 permission tables
// (AutoMigrate — the seeder test reads sys_menu through them).
func newRegisterPVETestDB(t *testing.T) *gorm.DB {
	t.Helper()
	testutil.PinSecretKeys(t)
	db := testutil.OpenMemoryDB(t)
	if err := migrate.Run(context.Background(), db); err != nil {
		t.Fatalf("migrate.Run: %v", err)
	}
	if err := store.AutoMigrate(db); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	return db
}

func TestRegisterPVEMockClusterSeedsFiveRows(t *testing.T) {
	mock := &pveRegisterMock{entries: clusterEntries()}
	srv := mock.serve(t)
	db := newRegisterPVETestDB(t)

	roSecret := "mock-pve-ro-not-real"
	opsSecret := "mock-pve-ops-not-real"
	report, err := registerPVEInDB(context.Background(), db, pveRegisterOptions{
		Name: "pve-mock", Endpoint: srv.URL,
		TokenUser: "root@pam", ROTokenID: "ro-id-1", OpsTokenID: "ops-id-1",
		ROTokenSecretFlag: roSecret, OpsTokenSecretFlag: opsSecret,
	})
	if err != nil {
		t.Fatalf("registerPVEInDB: %v", err)
	}
	if report.ConnectionUID == "" || report.ContextUID == "" {
		t.Fatalf("report must carry the chain uids: %+v", report)
	}
	if report.ClusterName != "pve-cluster-mock" || report.Standalone || report.Nodes != 2 {
		t.Fatalf("cluster identity mismatch: %+v", report)
	}

	// DB 5행 — 커넥션 1·컨텍스트 1·SecretRef 2·바인딩 2 (§5 F 배치 단얫).
	var conn inframodel.ProviderConnection
	if err := db.Where("uid = ?", report.ConnectionUID).First(&conn).Error; err != nil {
		t.Fatalf("load connection: %v", err)
	}
	if conn.ProviderType != "proxmox" || conn.Endpoint != srv.URL || conn.Status != "active" {
		t.Fatalf("connection shape: %+v", conn)
	}
	if conn.SourceModel != "" {
		t.Fatalf("register-pve has no v1 source: %+v", conn)
	}
	var count int64
	if err := db.Model(&inframodel.ProviderConnection{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("connection rows = %d err=%v, want 1", count, err)
	}

	var pctx inframodel.ProviderContext
	if err := db.Where("uid = ?", report.ContextUID).First(&pctx).Error; err != nil {
		t.Fatalf("load context: %v", err)
	}
	if pctx.Kind != "cluster" || pctx.ExternalID != "pve-cluster-mock" || pctx.ConnectionID != conn.ID {
		t.Fatalf("context shape: %+v", pctx)
	}
	if pctx.MetadataJSON["nodes"] != float64(2) || pctx.MetadataJSON["quorum"] != true {
		t.Fatalf("cluster metadata (§3.3 nodes:n·quorum:bool): %+v", pctx.MetadataJSON)
	}
	if err := db.Model(&inframodel.ProviderContext{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("context rows = %d err=%v, want 1", count, err)
	}

	var refs []inframodel.SecretRef
	if err := db.Find(&refs).Error; err != nil {
		t.Fatalf("load secret refs: %v", err)
	}
	if len(refs) != 2 {
		t.Fatalf("secret_ref rows = %d, want 2", len(refs))
	}
	bySecret := map[string]pveTokenMaterialJSON{}
	for _, ref := range refs {
		if ref.Backend != "internal" || ref.KeyID == "" {
			t.Fatalf("secret ref posture (KeyID 기록 — §3.3): %+v", ref)
		}
		if !util.IsV2Envelope(ref.Ciphertext) {
			t.Fatalf("secret ref %d is not a v2 envelope", ref.ID)
		}
		plain, err := util.DecryptSecretV2(ref.Ciphertext)
		if err != nil {
			t.Fatalf("decrypt secret ref %d: %v", ref.ID, err)
		}
		var material pveTokenMaterialJSON
		if err := json.Unmarshal([]byte(plain), &material); err != nil {
			t.Fatalf("secret ref %d material is not the {tokenUser,tokenID,tokenSecret} blob: %v", ref.ID, err)
		}
		if material.TokenUser != "root@pam" {
			t.Fatalf("secret ref %d tokenUser = %q", ref.ID, material.TokenUser)
		}
		bySecret[material.TokenID] = material
	}
	if got := bySecret["ro-id-1"].TokenSecret; got != roSecret {
		t.Fatalf("RO material round trip failed: %q", got)
	}
	if got := bySecret["ops-id-1"].TokenSecret; got != opsSecret {
		t.Fatalf("OPS material round trip failed: %q", got)
	}

	var bindings []inframodel.ProviderCredentialBinding
	if err := db.Find(&bindings).Error; err != nil {
		t.Fatalf("load bindings: %v", err)
	}
	if len(bindings) != 2 {
		t.Fatalf("binding rows = %d, want 2", len(bindings))
	}
	purposes := map[string]uint{}
	for _, b := range bindings {
		purposes[b.Purpose] = b.SecretRefID
		if b.ProviderConnectionID != conn.ID || b.ProviderContextID == nil || *b.ProviderContextID != pctx.ID {
			t.Fatalf("binding chain pointers: %+v", b)
		}
	}
	if purposes["inventory"] == 0 || purposes["operations"] == 0 || purposes["inventory"] == purposes["operations"] {
		t.Fatalf("bindings must split inventory/operations onto distinct SecretRefs (판정 J5): %v", purposes)
	}

	// 검증 스모크는 읽기 전용 토큰으로만 간다 — OPS 토큰은 봉인 저장소 밖으로
	// 흐르지 않는다(§14.3 read-only token for discovery).
	if len(mock.auth) == 0 {
		t.Fatal("validate must issue at least one authenticated probe")
	}
	for i, header := range mock.auth {
		if !strings.HasPrefix(header, "PVEAPIToken=root@pam!ro-id-1=") {
			t.Fatalf("probe %d used a credential other than the read-only token", i)
		}
	}
	joined := strings.Join(mock.paths, ",")
	if !strings.Contains(joined, "/api2/json/version") || !strings.Contains(joined, "/api2/json/cluster/status") {
		t.Fatalf("validate must smoke /version + /cluster/status (§13-1), got %s", joined)
	}
}

func TestRegisterPVEIdempotentRerunUpdatesCredentials(t *testing.T) {
	mock := &pveRegisterMock{entries: clusterEntries()}
	srv := mock.serve(t)
	db := newRegisterPVETestDB(t)

	base := pveRegisterOptions{
		Name: "pve-mock", Endpoint: srv.URL,
		TokenUser: "root@pam", ROTokenID: "ro-id-1", OpsTokenID: "ops-id-1",
		ROTokenSecretFlag: "mock-pve-ro-not-real", OpsTokenSecretFlag: "mock-pve-ops-not-real",
	}
	if _, err := registerPVEInDB(context.Background(), db, base); err != nil {
		t.Fatalf("first run: %v", err)
	}

	rotated := base
	rotated.Endpoint = strings.Replace(srv.URL, "127.0.0.1", "localhost", 1)
	rotated.ROTokenSecretFlag = "mock-pve-ro-rotated"
	report, err := registerPVEInDB(context.Background(), db, rotated)
	if err != nil {
		t.Fatalf("rerun: %v", err)
	}

	// 재실행 후에도 체인 행 수는 불변 — 커넥션 1·컨텍스트 1·SecretRef 2·바인딩 2.
	for _, want := range []struct {
		table string
		rows  int64
	}{
		{"provider_connection", 1},
		{"provider_context", 1},
		{"secret_ref", 2},
		{"provider_credential_binding", 2},
	} {
		var count int64
		if err := db.Table(want.table).Count(&count).Error; err != nil {
			t.Fatalf("count %s: %v", want.table, err)
		}
		if count != want.rows {
			t.Fatalf("%s rows = %d, want %d after rerun", want.table, count, want.rows)
		}
	}

	// 자격·메타 갱신 — 재실행은 같은 체인 위에서 endpoint·시크릿을 갱신한다
	// (§3.3 멱등: k8s 백필 멱등 관례).
	var conn inframodel.ProviderConnection
	if err := db.Where("uid = ?", report.ConnectionUID).First(&conn).Error; err != nil {
		t.Fatalf("load connection: %v", err)
	}
	if conn.Endpoint != rotated.Endpoint {
		t.Fatalf("rerun must update the endpoint: %q", conn.Endpoint)
	}
	var binding inframodel.ProviderCredentialBinding
	if err := db.Where("provider_connection_id = ? AND purpose = ?", conn.ID, "inventory").First(&binding).Error; err != nil {
		t.Fatalf("load inventory binding: %v", err)
	}
	var ref inframodel.SecretRef
	if err := db.First(&ref, binding.SecretRefID).Error; err != nil {
		t.Fatalf("load ro secret ref: %v", err)
	}
	plain, err := util.DecryptSecretV2(ref.Ciphertext)
	if err != nil {
		t.Fatalf("decrypt rotated material: %v", err)
	}
	var material pveTokenMaterialJSON
	if err := json.Unmarshal([]byte(plain), &material); err != nil {
		t.Fatal(err)
	}
	if material.TokenSecret != rotated.ROTokenSecretFlag {
		t.Fatalf("rerun must re-seal the rotated secret, got %q", material.TokenSecret)
	}
}

func TestRegisterPVEStandaloneFallback(t *testing.T) {
	mock := &pveRegisterMock{entries: standaloneEntries()}
	srv := mock.serve(t)
	db := newRegisterPVETestDB(t)

	report, err := registerPVEInDB(context.Background(), db, pveRegisterOptions{
		Name: "pve-standalone", Endpoint: srv.URL,
		TokenUser: "root@pam", ROTokenID: "ro-id-1", OpsTokenID: "ops-id-1",
		ROTokenSecretFlag: "mock-pve-ro-not-real", OpsTokenSecretFlag: "mock-pve-ops-not-real",
		InsecureTLS: true,
	})
	if err != nil {
		t.Fatalf("registerPVEInDB: %v", err)
	}
	if !report.Standalone || report.ClusterName != "pve1" || report.Nodes != 1 {
		t.Fatalf("standalone identity (A11): %+v", report)
	}
	var pctx inframodel.ProviderContext
	if err := db.Where("uid = ?", report.ContextUID).First(&pctx).Error; err != nil {
		t.Fatalf("load context: %v", err)
	}
	if pctx.ExternalID != "pve1" {
		t.Fatalf("standalone ExternalID must be the only node name (A11), got %q", pctx.ExternalID)
	}
	if pctx.MetadataJSON["standalone"] != true {
		t.Fatalf("standalone marker missing: %+v", pctx.MetadataJSON)
	}
	if _, carries := pctx.MetadataJSON["quorum"]; carries {
		t.Fatalf("standalone context must not fake a quorum field: %+v", pctx.MetadataJSON)
	}
	if len(report.Warnings) != 0 {
		t.Fatalf("loopback endpoint + insecure-tls must not warn (private premise holds): %v", report.Warnings)
	}
}

// TestSeedPVEGuestOperatePermissionGrantsSuperAdmin pins plan claim 11(c): the
// seeder creates the status-1 menu leaf, grants it to super-admin, and reruns
// leave every row count unchanged (멱등 upsert — r1.4 HIGH-1 재계약).
func TestSeedPVEGuestOperatePermissionGrantsSuperAdmin(t *testing.T) {
	t.Setenv("OPS_ADMIN_INITIAL_PASSWORD", "seed-test-password")
	db := newRegisterPVETestDB(t)
	if err := store.Seed(db); err != nil {
		t.Fatalf("Seed: %v", err)
	}

	menuCount := func(t *testing.T) int64 {
		t.Helper()
		var n int64
		if err := db.Table("sys_menu").
			Where("value = ? AND menu_status = 1", "infra:pve:guest:operate").
			Count(&n).Error; err != nil {
			t.Fatal(err)
		}
		return n
	}
	grantCount := func(t *testing.T) int64 {
		t.Helper()
		var n int64
		if err := db.Table("sys_role_menu rm").
			Joins("JOIN sys_role r ON r.id = rm.role_id").
			Joins("JOIN sys_menu m ON m.id = rm.menu_id").
			Where("r.role_key = ? AND m.value = ?", "super-admin", "infra:pve:guest:operate").
			Count(&n).Error; err != nil {
			t.Fatal(err)
		}
		return n
	}

	if got := menuCount(t); got < 1 {
		t.Fatalf("sys_menu rows for infra:pve:guest:operate = %d, want >= 1", got)
	}
	if got := grantCount(t); got < 1 {
		t.Fatalf("super-admin grants for infra:pve:guest:operate = %d, want >= 1", got)
	}

	// 멱등: 재시드는 행 수를 바꾸지 않는다.
	if err := store.Seed(db); err != nil {
		t.Fatalf("re-Seed: %v", err)
	}
	if got := menuCount(t); got != 1 {
		t.Fatalf("after rerun sys_menu rows = %d, want unchanged 1", got)
	}
	if got := grantCount(t); got != 1 {
		t.Fatalf("after rerun grants = %d, want unchanged 1", got)
	}

	// The leaf hangs under the hidden route-permissions root (plan M8).
	var root, leaf struct{ ID, ParentID uint }
	if err := db.Table("sys_menu").Select("id, parent_id").
		Where("value = ?", store.RoutePermissionsRootValue).Scan(&root).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Table("sys_menu").Select("id, parent_id").
		Where("value = ?", "infra:pve:guest:operate").Scan(&leaf).Error; err != nil {
		t.Fatal(err)
	}
	if leaf.ParentID != root.ID {
		t.Fatalf("leaf parent = %d, want the route-permissions root %d", leaf.ParentID, root.ID)
	}
}

func TestRegisterPVEEnvSecretTakesPrecedenceOverFlag(t *testing.T) {
	mock := &pveRegisterMock{entries: clusterEntries()}
	srv := mock.serve(t)
	db := newRegisterPVETestDB(t)

	t.Setenv("OPS_ADMIN_PVE_RO_TOKEN_SECRET", "mock-pve-from-env-flag")
	report, err := registerPVEInDB(context.Background(), db, pveRegisterOptions{
		Name: "pve-mock", Endpoint: srv.URL,
		TokenUser: "root@pam", ROTokenID: "ro-id-1", OpsTokenID: "ops-id-1",
		// flag 폴백 값 — env가 설정된 동안은 무시되어야 한다 (§3.3).
		ROTokenSecretFlag: "mock-pve-ro-not-real", OpsTokenSecretFlag: "mock-pve-ops-not-real",
	})
	if err != nil {
		t.Fatalf("registerPVEInDB: %v", err)
	}
	var conn inframodel.ProviderConnection
	if err := db.Where("uid = ?", report.ConnectionUID).First(&conn).Error; err != nil {
		t.Fatal(err)
	}
	var binding inframodel.ProviderCredentialBinding
	if err := db.Where("provider_connection_id = ? AND purpose = ?", conn.ID, "inventory").First(&binding).Error; err != nil {
		t.Fatal(err)
	}
	var ref inframodel.SecretRef
	if err := db.First(&ref, binding.SecretRefID).Error; err != nil {
		t.Fatal(err)
	}
	plain, err := util.DecryptSecretV2(ref.Ciphertext)
	if err != nil {
		t.Fatal(err)
	}
	var material pveTokenMaterialJSON
	if err := json.Unmarshal([]byte(plain), &material); err != nil {
		t.Fatal(err)
	}
	if material.TokenSecret != "mock-pve-from-env-flag" {
		t.Fatalf("env secret must win over the flag fallback, got %q", material.TokenSecret)
	}
}

func TestPVEEndpointPrivateWarn(t *testing.T) {
	cases := []struct {
		endpoint string
		insecure bool
		want     bool
	}{
		{"https://192.168.250.43:8006", true, false},      // RFC1918 — 전제 충족, 경고 없음
		{"http://10.0.0.5:8006", true, false},             // RFC1918 10/8
		{"https://172.16.4.9:8006", true, false},          // RFC1918 172.16/12
		{"http://127.0.0.1:8006", true, false},            // loopback — 사설 전제 충족
		{"https://192.168.250.43:8006", false, false},     // TLS 검증 유지 — 경고 대상 아님
		{"https://203.0.113.10:8006", true, true},         // 비-RFC1918 공개 IP + 우회 → 경고 (MEDIUM-8)
		{"https://pve.example.internal:8006", true, true}, // 호스트명은 전제 검증 불가 → 경고
	}
	for _, tc := range cases {
		got := pveEndpointPrivateWarn(tc.endpoint, tc.insecure)
		if (got != "") != tc.want {
			t.Fatalf("pveEndpointPrivateWarn(%q, %v) = %q, wantWarn=%v", tc.endpoint, tc.insecure, got, tc.want)
		}
	}
}

func TestRunRegisterPVERejectsBadInput(t *testing.T) {
	if code := runRegisterPVE([]string{"--undefined-flag"}); code != 1 {
		t.Fatalf("undefined flag must exit 1, got %d", code)
	}
	if code := runRegisterPVE(nil); code != 1 {
		t.Fatalf("missing required flags must exit 1, got %d", code)
	}
	t.Setenv("OPS_ADMIN_PVE_RO_TOKEN_SECRET", "")
	t.Setenv("OPS_ADMIN_PVE_OPS_TOKEN_SECRET", "")
	code := runRegisterPVE([]string{
		"--name", "pve-x", "--endpoint", "https://192.168.250.43:8006",
		"--ro-token-id", "ro-id-1", "--ops-token-id", "ops-id-1",
	})
	if code != 1 {
		t.Fatalf("missing both secret sources must exit 1, got %d", code)
	}
}
