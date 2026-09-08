package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"gorm.io/gorm"
	"ops-admin/backend/internal/infra/contract"
	inframodel "ops-admin/backend/internal/infra/model"
)

// p5 잔여 소화(④리뷰 LOW 3건) 테스트 — 신원 probe의 어댑터 조립 재사용과
// 체인 쓰기 트랜잭션 원자성을 핀다. 기존 main_register_pve_test.go의 하니스는
// 무변경 유지 — 이 파일은 그 보완 단얫만 담는다.

// pveProbeCountMock — 경로·Authorization 헤더·호출 수를 기록하는 최소 PVE
// 표면. probe 동작 동일성(RO 토큰·/api2/json 경로·단발성)을 관측한다.
type pveProbeCountMock struct {
	mu            sync.Mutex
	paths         []string
	auth          []string
	clusterStatus int // /cluster/status 응답 코드
	entries       string
}

func (m *pveProbeCountMock) serve(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.mu.Lock()
		m.paths = append(m.paths, r.URL.Path)
		m.auth = append(m.auth, r.Header.Get("Authorization"))
		m.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/version") {
			_, _ = w.Write([]byte(`{"data":{"version":"8.2.4","reporelease":"8.2","release":"8.2"}}`))
			return
		}
		if strings.HasSuffix(r.URL.Path, "/cluster/status") {
			w.WriteHeader(m.clusterStatus)
			_, _ = w.Write([]byte(`{"data":` + m.entries + `}`))
			return
		}
		http.Error(w, `{"data":null}`, http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func (m *pveProbeCountMock) countPath(suffix string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, p := range m.paths {
		if strings.HasSuffix(p, suffix) {
			n++
		}
	}
	return n
}

func pveLeftoversOptions(endpoint string) pveRegisterOptions {
	return pveRegisterOptions{
		Name: "pve-leftovers", Endpoint: endpoint,
		TokenUser: "root@pam", ROTokenID: "ro-id-1", OpsTokenID: "ops-id-1",
		ROTokenSecretFlag: "mock-pve-ro-not-real", OpsTokenSecretFlag: "mock-pve-ops-not-real",
	}
}

// probe 동작 동일성: Validate(/version)와 신원 probe(/cluster/status)가 같은
// 어댑터 조립 — 같은 /api2/json 경로, 같은 RO 토큰 헤더, 각 1회 단발.
func TestRegisterPVEIdentityProbeUsesAdapterAssembly(t *testing.T) {
	mock := &pveProbeCountMock{
		clusterStatus: http.StatusOK,
		entries:       clusterEntries(),
	}
	srv := mock.serve(t)
	db := newRegisterPVETestDB(t)

	report, err := registerPVEInDB(context.Background(), db, pveLeftoversOptions(srv.URL))
	if err != nil {
		t.Fatalf("registerPVEInDB: %v", err)
	}
	if report.ClusterName != "pve-cluster-mock" || report.Nodes != 2 || report.Standalone {
		t.Fatalf("probe identity: %+v", report)
	}
	// 단발성 — Validate 1회 + 신원 1회, 페일오버 재조회·중복 probe 없음.
	if got := mock.countPath("/api2/json/version"); got != 1 {
		t.Fatalf("/version probes = %d, want 1", got)
	}
	if got := mock.countPath("/api2/json/cluster/status"); got != 1 {
		t.Fatalf("/cluster/status probes = %d, want 1", got)
	}
	// 모든 probe가 RO 토큰 헤더 — 신원 probe가 Validate와 동일한 자격 조립
	// (§14.3 read-only token for discovery).
	if len(mock.auth) != 2 {
		t.Fatalf("authenticated probes = %d, want 2", len(mock.auth))
	}
	for i, header := range mock.auth {
		if !strings.HasPrefix(header, "PVEAPIToken=root@pam!ro-id-1=") {
			t.Fatalf("probe %d used a credential other than the read-only token: %q", i, header)
		}
	}
}

// 신원 probe 실패는 어댑터 §9.3 신호 어휘로 분류되고(런타임 조립 재사용의
// 관측 증거), 등록은 체인 기입 없이 끝난다.
func TestRegisterPVEIdentityProbeErrorIsSignalTaxonomy(t *testing.T) {
	mock := &pveProbeCountMock{
		clusterStatus: http.StatusServiceUnavailable,
		entries:       clusterEntries(),
	}
	srv := mock.serve(t)
	db := newRegisterPVETestDB(t)

	_, err := registerPVEInDB(context.Background(), db, pveLeftoversOptions(srv.URL))
	var signal *contract.ProviderSignalError
	if !errors.As(err, &signal) {
		t.Fatalf("identity probe error = %v, want a ProviderSignalError", err)
	}
	if signal.Kind != contract.SignalUnreachable {
		t.Fatalf("503 kind = %s, want %s", signal.Kind, contract.SignalUnreachable)
	}
	// 부분 체인 없음 — probe 실패 시 어떤 행도 남지 않는다.
	for _, table := range []string{"provider_connection", "provider_context", "secret_ref", "provider_credential_binding"} {
		var count int64
		if err := db.Table(table).Count(&count).Error; err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if count != 0 {
			t.Fatalf("%s rows = %d after a failed probe, want 0", table, count)
		}
	}
}

// 체인 쓰기는 단일 트랜잭션: 마지막 단계(바인딩) 실패 시 선행 단계의
// 커넥션·컨텍스트·SecretRef도 롤백된다. 콜백 제거 후 재실행은 같은 이름으로
// 온전한 체인을 만든다 — 멱등 upsert 의미론 불변.
func TestRegisterPVEChainWriteIsAtomic(t *testing.T) {
	mock := &pveProbeCountMock{clusterStatus: http.StatusOK, entries: clusterEntries()}
	srv := mock.serve(t)
	db := newRegisterPVETestDB(t)

	failBinding := func(tx *gorm.DB) {
		table := tx.Statement.Table
		if table == "" && tx.Statement.Schema != nil {
			table = tx.Statement.Schema.Table
		}
		if table == "provider_credential_binding" {
			_ = tx.AddError(fmt.Errorf("injected binding failure"))
		}
	}
	if err := db.Callback().Create().After("gorm:create").Register("test:fail_binding", failBinding); err != nil {
		t.Fatalf("register callback: %v", err)
	}
	if _, err := registerPVEInDB(context.Background(), db, pveLeftoversOptions(srv.URL)); err == nil ||
		!strings.Contains(err.Error(), "injected binding failure") {
		t.Fatalf("injected binding failure must surface, got %v", err)
	}
	for _, table := range []string{"provider_connection", "provider_context", "secret_ref", "provider_credential_binding"} {
		var count int64
		if err := db.Table(table).Count(&count).Error; err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if count != 0 {
			t.Fatalf("%s rows = %d after a mid-chain failure, want 0 (트랜잭션 롤백)", table, count)
		}
	}

	if err := db.Callback().Create().Remove("test:fail_binding"); err != nil {
		t.Fatalf("remove callback: %v", err)
	}
	report, err := registerPVEInDB(context.Background(), db, pveLeftoversOptions(srv.URL))
	if err != nil {
		t.Fatalf("rerun after rollback: %v", err)
	}
	want := map[string]int64{
		"provider_connection":         1,
		"provider_context":            1,
		"secret_ref":                  2,
		"provider_credential_binding": 2,
	}
	for table, rows := range want {
		var count int64
		if err := db.Table(table).Count(&count).Error; err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if count != rows {
			t.Fatalf("%s rows = %d, want %d after the clean rerun", table, count, rows)
		}
	}
	if report.ConnectionUID == "" || report.ContextUID == "" {
		t.Fatalf("report must carry the chain uids: %+v", report)
	}
	var refs []inframodel.SecretRef
	if err := db.Find(&refs).Error; err != nil {
		t.Fatal(err)
	}
	if len(refs) != 2 {
		t.Fatalf("secret refs = %d, want 2", len(refs))
	}
}
