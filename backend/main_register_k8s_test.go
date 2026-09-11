package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
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

// register-k8s 테스트 하니스 — phase6-plan §3.2 H0 (C58 봉인·C59 UID 도메인·
// 멱등·검증 실패 롤백 단얫). mock K8s API 서버는 /version만 서빙한다 — 등록
// Validate가 닿는 유일 엔드포인트. 모의 kubeconfig의 토큰은 합성값이며
// keyword-entropy 스캔 규칙을 통과하는 형식이다(pve 선례 주석 승계).

type k8sRegisterMock struct {
	mu    sync.Mutex
	calls int
	fail  bool // true면 /version이 500 — 검증 실패 롤백 단얫용
}

func (m *k8sRegisterMock) serve(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.mu.Lock()
		m.calls++
		fail := m.fail
		m.mu.Unlock()
		if r.URL.Path != "/version" {
			http.Error(w, `{"kind":"Status","status":"Failure"}`, http.StatusNotFound)
			return
		}
		if fail {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"major":"1","minor":"34","gitVersion":"v1.34.0"}`)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func kubeconfigForServer(server string) string {
	return fmt.Sprintf(`apiVersion: v1
kind: Config
current-context: kind-test
clusters:
  - name: test
    cluster:
      server: %s
contexts:
  - name: kind-test
    context:
      cluster: test
      user: test
users:
  - name: test
    user:
      token: mock-k8s-token-not-real
`, server)
}

func writeTempKubeconfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "kubeconfig")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write kubeconfig fixture: %v", err)
	}
	return path
}

// newRegisterK8sTestDB opens the in-memory database with both schema layers
// the CLI touches (pve 하니스 승계): the V2 tables (migrate.Run) and the v1
// tables (AutoMigrate).
func newRegisterK8sTestDB(t *testing.T) *gorm.DB {
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

// assertChainCounts pins the row counts of the four chain tables — the
// idempotent rerun shape (1·1·1·1) and the rollback shape (all zero) both
// ride on it.
func assertChainCounts(t *testing.T, db *gorm.DB, want map[string]int64) {
	t.Helper()
	for _, table := range []string{"provider_connection", "provider_context", "secret_ref", "provider_credential_binding"} {
		var count int64
		if err := db.Table(table).Count(&count).Error; err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if count != want[table] {
			t.Fatalf("%s rows = %d, want %d", table, count, want[table])
		}
	}
}

func TestRegisterK8sSealsKubeconfigChain(t *testing.T) {
	mock := &k8sRegisterMock{}
	srv := mock.serve(t)
	db := newRegisterK8sTestDB(t)

	kubeconfig := kubeconfigForServer(srv.URL)
	report, err := registerK8sInDB(context.Background(), db, k8sRegisterOptions{
		Name: "k8s-mock", KubeconfigPath: writeTempKubeconfig(t, kubeconfig),
		Env: "dev", ConnectionMode: k8sModeDirect, MonitorDatasourceID: "42",
	})
	if err != nil {
		t.Fatalf("registerK8sInDB: %v", err)
	}

	// UID 도메인 — register-scoped sha256[:32], C59의 공식 그 자체.
	if want := k8sRegisterUID("k8s-mock", ""); report.ConnectionUID != want {
		t.Fatalf("connectionUid = %q, want the register domain value %q", report.ConnectionUID, want)
	}
	if want := k8sRegisterUID("k8s-mock", "context"); report.ContextUID != want {
		t.Fatalf("contextUid = %q, want %q", report.ContextUID, want)
	}

	var conn inframodel.ProviderConnection
	if err := db.Where("uid = ?", report.ConnectionUID).First(&conn).Error; err != nil {
		t.Fatalf("load connection: %v", err)
	}
	if conn.ProviderType != "kubernetes" || conn.Status != "active" || conn.Endpoint != "" {
		t.Fatalf("connection shape: %+v", conn)
	}
	if conn.SourceModel != "" {
		t.Fatalf("register-k8s has no v1 source row: %+v", conn)
	}
	if conn.ConfigJSON["connection_mode"] != "direct" || conn.ConfigJSON["env"] != "dev" || conn.ConfigJSON["monitor_datasource_id"] != "42" {
		t.Fatalf("ConfigJSON fixed keys (§J9 F6): %+v", conn.ConfigJSON)
	}

	var pctx inframodel.ProviderContext
	if err := db.Where("uid = ?", report.ContextUID).First(&pctx).Error; err != nil {
		t.Fatalf("load context: %v", err)
	}
	if pctx.Kind != "cluster" || pctx.ExternalID != "k8s-mock" || pctx.ConnectionID != conn.ID {
		t.Fatalf("context shape: %+v", pctx)
	}

	var refs []inframodel.SecretRef
	if err := db.Find(&refs).Error; err != nil {
		t.Fatalf("load secret refs: %v", err)
	}
	if len(refs) != 1 {
		t.Fatalf("secret_ref rows = %d, want 1 (kubeconfig 단일 자격)", len(refs))
	}
	ref := refs[0]
	if ref.Backend != "internal" || ref.KeyID == "" || !util.IsV2Envelope(ref.Ciphertext) {
		t.Fatalf("secret ref posture (v2 봉인·KeyID 기록): %+v", ref)
	}
	plain, err := util.DecryptSecretV2(ref.Ciphertext)
	if err != nil {
		t.Fatalf("decrypt sealed kubeconfig: %v", err)
	}
	if plain != kubeconfig {
		t.Fatalf("sealed material must round-trip to the kubeconfig file content")
	}

	var bindings []inframodel.ProviderCredentialBinding
	if err := db.Find(&bindings).Error; err != nil {
		t.Fatalf("load bindings: %v", err)
	}
	if len(bindings) != 2 {
		t.Fatalf("binding rows = %d, want 2 (inventory + operations — §7.4 purpose-per-row)", len(bindings))
	}
	byPurpose := map[string]inframodel.ProviderCredentialBinding{}
	for _, b := range bindings {
		byPurpose[b.Purpose] = b
	}
	if _, ok := byPurpose["inventory"]; !ok {
		t.Fatalf("binding purposes = %v, want inventory present", byPurpose)
	}
	if _, ok := byPurpose["operations"]; !ok {
		t.Fatalf("binding purposes = %v, want operations present", byPurpose)
	}
	for purpose, b := range byPurpose {
		if b.ProviderConnectionID != conn.ID || b.SecretRefID != ref.ID ||
			b.ProviderContextID == nil || *b.ProviderContextID != pctx.ID {
			t.Fatalf("binding(%s) chain pointers: %+v", purpose, b)
		}
	}

	// C58 평문 무잔존 — 봉인 행 밖 어느 체인 행에도 kubeconfig 성분(토큰·
	// 서버 URL)이 남지 않는다. backfill의 P-class 예외를 승계하지 않는다.
	token := "mock-k8s-token-not-real"
	residueRows := map[string]any{"connection": conn, "context": pctx}
	for i, b := range bindings {
		residueRows[fmt.Sprintf("binding[%d]", i)] = b
	}
	for name, row := range residueRows {
		blob, err := json.Marshal(row)
		if err != nil {
			t.Fatalf("marshal %s: %v", name, err)
		}
		if strings.Contains(string(blob), token) || strings.Contains(string(blob), srv.URL) {
			t.Fatalf("%s row carries kubeconfig material (C58): %s", name, blob)
		}
	}
	assertChainCounts(t, db, map[string]int64{
		"provider_connection": 1, "provider_context": 1, "secret_ref": 1, "provider_credential_binding": 2,
	})
}

func TestRegisterK8sIdempotentRerunKeepsUIDsAndReSeals(t *testing.T) {
	mock := &k8sRegisterMock{}
	srv := mock.serve(t)
	db := newRegisterK8sTestDB(t)

	base := k8sRegisterOptions{
		Name: "k8s-mock", KubeconfigPath: writeTempKubeconfig(t, kubeconfigForServer(srv.URL)),
		ConnectionMode: k8sModeDirect,
	}
	first, err := registerK8sInDB(context.Background(), db, base)
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	var sealedFirst string
	if err := db.Table("secret_ref").Select("ciphertext").Where("uid = ?", k8sRegisterUID("k8s-mock", "secret")).Scan(&sealedFirst).Error; err != nil {
		t.Fatalf("load first ciphertext: %v", err)
	}

	// 재실행 — 같은 --name, 다른 kubeconfig(토큰 로테이션 시나리오). UID는
	// 유지되고 SecretRef는 재봉인된다(CLI 규약 멱등).
	rotated := kubeconfigForServer(srv.URL)
	rotated = strings.Replace(rotated, "mock-k8s-token-not-real", "mock-k8s-token-rotated", 1)
	rotatedOpts := base
	rotatedOpts.KubeconfigPath = writeTempKubeconfig(t, rotated)
	second, err := registerK8sInDB(context.Background(), db, rotatedOpts)
	if err != nil {
		t.Fatalf("rerun: %v", err)
	}
	if second.ConnectionUID != first.ConnectionUID || second.ContextUID != first.ContextUID {
		t.Fatalf("rerun must keep the chain UIDs: %+v vs %+v", first, second)
	}
	assertChainCounts(t, db, map[string]int64{
		"provider_connection": 1, "provider_context": 1, "secret_ref": 1, "provider_credential_binding": 2,
	})

	var ref inframodel.SecretRef
	if err := db.Where("uid = ?", k8sRegisterUID("k8s-mock", "secret")).First(&ref).Error; err != nil {
		t.Fatalf("load re-sealed ref: %v", err)
	}
	if ref.Ciphertext == sealedFirst {
		t.Fatalf("rerun must re-seal the rotated kubeconfig — ciphertext unchanged")
	}
	plain, err := util.DecryptSecretV2(ref.Ciphertext)
	if err != nil {
		t.Fatalf("decrypt re-sealed kubeconfig: %v", err)
	}
	if plain != rotated {
		t.Fatalf("re-sealed material must be the rotated kubeconfig content")
	}
}

// TestRegisterK8sSealsBothCredentialPurposesAtTheSameSecretRef — I10 J4 결함 3:
// k8s 자격은 kubeconfig 하나지만 바인딩은 물질 단위가 아니라 purpose 단위다
// (§7.4 same-SecretRef 명문 패턴). register-k8s는 inventory·operations 2행을
// 봉인해 실행기 assembly가 조회하는 Material["operations"]가 비지 않게 한다
// (engine_resolve·executor 자격 계약은 무편집 — §5 #19). 재실행은 2행을
// 그대로 유지한다(멱등 upsert — 기존 dev 체인의 자격 보강 경로).
func TestRegisterK8sSealsBothCredentialPurposesAtTheSameSecretRef(t *testing.T) {
	mock := &k8sRegisterMock{}
	srv := mock.serve(t)
	db := newRegisterK8sTestDB(t)

	run := func() k8sRegisterReport {
		t.Helper()
		report, err := registerK8sInDB(context.Background(), db, k8sRegisterOptions{
			Name: "k8s-mock", KubeconfigPath: writeTempKubeconfig(t, kubeconfigForServer(srv.URL)),
			ConnectionMode: k8sModeDirect,
		})
		if err != nil {
			t.Fatalf("registerK8sInDB: %v", err)
		}
		return report
	}
	report := run()

	var conn inframodel.ProviderConnection
	if err := db.Where("uid = ?", report.ConnectionUID).First(&conn).Error; err != nil {
		t.Fatalf("load connection: %v", err)
	}
	bindingRefs := func() map[string]uint {
		var rows []inframodel.ProviderCredentialBinding
		if err := db.Where("provider_connection_id = ?", conn.ID).Find(&rows).Error; err != nil {
			t.Fatalf("load bindings: %v", err)
		}
		refs := map[string]uint{}
		for _, b := range rows {
			refs[b.Purpose] = b.SecretRefID
		}
		return refs
	}

	refs := bindingRefs()
	if len(refs) != 2 {
		t.Fatalf("binding purposes = %v, want exactly {inventory, operations}", refs)
	}
	if refs["inventory"] == 0 || refs["operations"] == 0 {
		t.Fatalf("binding purposes = %v, want both inventory and operations sealed", refs)
	}
	if refs["inventory"] != refs["operations"] {
		t.Fatalf("one kubeconfig credential — both purposes must point at the same secret_ref (§7.4), got inventory=%d operations=%d", refs["inventory"], refs["operations"])
	}
	var refCount int64
	if err := db.Model(&inframodel.SecretRef{}).Count(&refCount).Error; err != nil {
		t.Fatalf("count secret refs: %v", err)
	}
	if refCount != 1 {
		t.Fatalf("secret_ref rows = %d, want 1 (single material shared by both purposes)", refCount)
	}

	// 멱등 재실행 — 2행 유지·같은 SecretRef 지목(기존 dev 체인 자격 보강).
	run()
	if again := bindingRefs(); len(again) != 2 || again["inventory"] != refs["inventory"] || again["operations"] != refs["operations"] {
		t.Fatalf("rerun must keep the two purpose rows pointed at the same secret_ref: %v", again)
	}
}

func TestRegisterK8sValidateFailureWritesNothing(t *testing.T) {
	mock := &k8sRegisterMock{fail: true}
	srv := mock.serve(t)
	db := newRegisterK8sTestDB(t)

	empty := map[string]int64{
		"provider_connection": 0, "provider_context": 0, "secret_ref": 0, "provider_credential_binding": 0,
	}

	// 빈 DB — 검증 실패 시 4테이블 전부 0행 (CLI 규약: 검증 실패 시 롤백 —
	// 실패한 등록이 부분 체인을 남기지 않는다).
	_, err := registerK8sInDB(context.Background(), db, k8sRegisterOptions{
		Name: "k8s-mock", KubeconfigPath: writeTempKubeconfig(t, kubeconfigForServer(srv.URL)),
	})
	if err == nil || !strings.Contains(err.Error(), "validate") {
		t.Fatalf("validate failure must surface as an error, got %v", err)
	}
	assertChainCounts(t, db, empty)

	// 기존 체인 보존 — 성공한 등록 뒤 검증 실패 재실행은 기존 행을 하나도
	// 고치지 않는다(갱신도 재봉인도 없다).
	goodMock := &k8sRegisterMock{}
	goodSrv := goodMock.serve(t)
	if _, err := registerK8sInDB(context.Background(), db, k8sRegisterOptions{
		Name: "k8s-good", KubeconfigPath: writeTempKubeconfig(t, kubeconfigForServer(goodSrv.URL)),
	}); err != nil {
		t.Fatalf("seeding run: %v", err)
	}
	var sealed string
	if err := db.Table("secret_ref").Select("ciphertext").Where("uid = ?", k8sRegisterUID("k8s-good", "secret")).Scan(&sealed).Error; err != nil {
		t.Fatalf("load sealed ciphertext: %v", err)
	}

	// 재실행 kubeconfig는 여전히 실패 중인 srv를 가리킨다(fail=true 유지).
	if _, err := registerK8sInDB(context.Background(), db, k8sRegisterOptions{
		Name: "k8s-good", KubeconfigPath: writeTempKubeconfig(t, kubeconfigForServer(srv.URL)),
	}); err == nil {
		t.Fatalf("failing rerun must error")
	}
	assertChainCounts(t, db, map[string]int64{
		"provider_connection": 1, "provider_context": 1, "secret_ref": 1, "provider_credential_binding": 2,
	})
	var after string
	if err := db.Table("secret_ref").Select("ciphertext").Where("uid = ?", k8sRegisterUID("k8s-good", "secret")).Scan(&after).Error; err != nil {
		t.Fatalf("reload sealed ciphertext: %v", err)
	}
	if after != sealed {
		t.Fatalf("failed rerun must not touch the existing chain")
	}
}

func TestRegisterK8sGatewayModeFailsWhileA4GateStands(t *testing.T) {
	mock := &k8sRegisterMock{}
	srv := mock.serve(t)
	db := newRegisterK8sTestDB(t)

	// gateway 모드는 기록된 posture 그대로 Validate에 들어가고, 어댑터의 A4
	// 게이트(M2 이전 dialer 부재)가 등록 자체를 막는다 — 봉인 없이 실패가
	// 정답이다(등록 probe가 런타임 posture에서 드리프트 금지).
	_, err := registerK8sInDB(context.Background(), db, k8sRegisterOptions{
		Name: "k8s-gw", KubeconfigPath: writeTempKubeconfig(t, kubeconfigForServer(srv.URL)),
		ConnectionMode: k8sModeGateway, GatewayID: "3",
	})
	if err == nil || !strings.Contains(err.Error(), "gateway") {
		t.Fatalf("gateway-mode registration must fail on the A4 gate, got %v", err)
	}
	assertChainCounts(t, db, map[string]int64{
		"provider_connection": 0, "provider_context": 0, "secret_ref": 0, "provider_credential_binding": 0,
	})
}

func TestRunRegisterK8sRejectsBadInput(t *testing.T) {
	if code := runRegisterK8s([]string{"--undefined-flag"}); code != 1 {
		t.Fatalf("undefined flag must exit 1, got %d", code)
	}
	if code := runRegisterK8s(nil); code != 1 {
		t.Fatalf("missing required flags must exit 1, got %d", code)
	}
	if code := runRegisterK8s([]string{"--name", "k8s-x"}); code != 1 {
		t.Fatalf("missing --kubeconfig must exit 1, got %d", code)
	}
	if code := runRegisterK8s([]string{"--name", "k8s-x", "--kubeconfig", "/tmp/none", "--connection-mode", "sideways"}); code != 1 {
		t.Fatalf("invalid --connection-mode must exit 1, got %d", code)
	}
	if code := runRegisterK8s([]string{"--name", "k8s-x", "--kubeconfig", "/tmp/none", "--connection-mode", "gateway"}); code != 1 {
		t.Fatalf("gateway mode without --gateway-id must exit 1, got %d", code)
	}
	if code := runRegisterK8s([]string{"--name", "k8s-x", "--kubeconfig", "/tmp/p6-h0-no-such-file"}); code != 1 {
		t.Fatalf("missing kubeconfig file must exit 1, got %d", code)
	}
}

// TestK8sRegisterUIDVocabularyNeverCollidesWithConnPrefix — CI17 LOW②(I10
// J1c): 등록 체인의 UID 생성기 산출은 전수 ^[0-9a-f]{32}$ — 콜론을 포함하지
// 않아 conn: 합성 resource_uid 접두(engine_resolve.go 예약)와 구조적으로
// 충돌하지 않는다(생성기 수준 잠금). newSyncUID(inventory)는 unexported +
// inventory/** 무변경(§5 #3)라 직접 단얫 불가 — 동일 근거(crypto/rand 16B
// hex32)와 데이터 수준 가드(tasks TestInfraResourceUIDsNeverUseConnPrefix)로
// 간접 잠금하며 직접 단얫 부재는 J1c 보고에 기록된다.
func TestK8sRegisterUIDVocabularyNeverCollidesWithConnPrefix(t *testing.T) {
	hex32 := regexp.MustCompile(`^[0-9a-f]{32}$`)
	for _, pair := range [][2]string{
		{"k8s-prod", ""},
		{"k8s-prod", "context"},
		{"k8s-prod", "secret"},
		{"k8s-dev", ""},
	} {
		if uid := k8sRegisterUID(pair[0], pair[1]); !hex32.MatchString(uid) {
			t.Errorf("k8sRegisterUID(%q, %q) = %q, want ^[0-9a-f]{32}$ (conn: 비충돌 생성기 잠금)", pair[0], pair[1], uid)
		}
	}
}
