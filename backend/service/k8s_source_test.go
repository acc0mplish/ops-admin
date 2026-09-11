// k8s_source_test.go — I-a S5·S6 전환 검증 (phase6-plan §J5, claims C64).
// GetK8sCluster의 V2 소스 전환(provider_connection + SecretRef 브로커 복호)과
// F6 필드 조달 계약(ConfigJSON 고정키 — gateway_id 문자열→*uint 파싱 포함),
// S6 등가 조회(k8s_cluster gorm 직렬 조회의 provider_connection 치환)를
// 단얫한다. fixture DB는 k8s_cluster 테이블 자체를 두지 않는다 — 테이블
// 결여가 legacy 우회 부재의 음의 증명이다(k8s_projection_test.go 선례).
package service

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	infraModel "ops-admin/backend/internal/infra/model"
	"ops-admin/backend/model"
	"ops-admin/backend/util"
)

// newK8sSourceDB — S5·S6 경로가 읽는 테이블만 갖춘 sqlite 인메모리 DB.
func newK8sSourceDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(
		&infraModel.ProviderConnection{},
		&infraModel.ProviderContext{},
		&infraModel.SecretRef{},
		&infraModel.ProviderCredentialBinding{},
		&model.AssetGateway{},
		&model.MonitorDatasource{},
	); err != nil {
		t.Fatal(err)
	}
	return db
}

// seedK8sSourceChain — legacy source_id를 가진 kubernetes 커넥션 체인 1세트
// (커넥션 + 컨텍스트 + 봉인 SecretRef + inventory 바인딩). kubeconfig는 즉시
// 봉인된다(§12 #16 — 테스트도 평문을 테이블에 남기지 않는다).
func seedK8sSourceChain(t *testing.T, db *gorm.DB, legacyID uint, conn infraModel.ProviderConnection, kubeconfig string) infraModel.ProviderConnection {
	t.Helper()
	sealed, err := util.EncryptSecretV2(kubeconfig)
	if err != nil {
		t.Fatalf("seal kubeconfig: %v", err)
	}
	keyID, payload, ok := strings.Cut(strings.TrimPrefix(sealed, "v2:"), ":")
	if !ok || keyID == "" || payload == "" {
		t.Fatalf("sealed kubeconfig is not a v2 envelope: %q", sealed)
	}
	conn.UID = fmt.Sprintf("ia-src-%d", legacyID)
	conn.ProviderType = "kubernetes"
	if err := db.Create(&conn).Error; err != nil {
		t.Fatalf("seed connection: %v", err)
	}
	pctx := infraModel.ProviderContext{
		UID: conn.UID + "-ctx", ConnectionID: conn.ID, Kind: "cluster",
		Name: conn.Name, Status: "active",
	}
	if err := db.Create(&pctx).Error; err != nil {
		t.Fatalf("seed context: %v", err)
	}
	ref := infraModel.SecretRef{
		UID: conn.UID + "-ref", Backend: "internal",
		Path: "test/k8s_source/" + conn.UID, KeyID: keyID, Ciphertext: sealed,
	}
	if err := db.Create(&ref).Error; err != nil {
		t.Fatalf("seed secret ref: %v", err)
	}
	binding := infraModel.ProviderCredentialBinding{
		ProviderConnectionID: conn.ID, ProviderContextID: &pctx.ID,
		Purpose: "inventory", SecretRefID: ref.ID, Status: "active",
	}
	if err := db.Create(&binding).Error; err != nil {
		t.Fatalf("seed binding: %v", err)
	}
	return conn
}

func newK8sSourceService(t *testing.T) *Service {
	t.Helper()
	util.ConfigureCredentialKey("k8s-source-test-key")
	t.Cleanup(func() { util.ConfigureCredentialKey("") })
	return &Service{db: newK8sSourceDB(t)}
}

// TestGetK8sCluster — C64① 대상 단얫. 백필 체인 형상(source_model/source_id
// 쌍 + gateway_id 칼럼 + ConfigJSON)을 V2 소스에서 조달해 legacy
// model.K8sCluster로 되돌린다: kubeconfig는 SecretRef 복호로만, gateway·
// monitor_datasource 프리로드는 id 조회로 승계된다.
func TestGetK8sCluster(t *testing.T) {
	svc := newK8sSourceService(t)
	db := svc.db

	gateway := model.AssetGateway{Name: "gw-a", Host: "10.0.0.1", Port: 22}
	if err := db.Create(&gateway).Error; err != nil {
		t.Fatalf("seed gateway: %v", err)
	}
	datasource := model.MonitorDatasource{Name: "prom", Type: "prometheus", Status: 1}
	if err := db.Create(&datasource).Error; err != nil {
		t.Fatalf("seed datasource: %v", err)
	}
	gatewayID, monitorID := gateway.ID, datasource.ID
	conn := seedK8sSourceChain(t, db, 42, infraModel.ProviderConnection{
		Name: "kind-a", Endpoint: "https://a:6443", Status: "active", Version: "v1.30.0",
		GatewayID: &gatewayID, SourceModel: "k8s_cluster", SourceID: 42,
		ConfigJSON: map[string]any{
			"env": "dev", "connection_mode": "gateway", "node_count": 3,
			"monitor_datasource_id": fmt.Sprintf("%d", monitorID),
			"tags":                  []any{"a", "b"},
		},
	}, "kubeconfig-plaintext-material")

	cluster, err := svc.GetK8sCluster(42)
	if err != nil {
		t.Fatalf("GetK8sCluster(42): %v", err)
	}
	if cluster.ID != 42 || cluster.Name != "kind-a" || cluster.APIServer != "https://a:6443" || cluster.Version != "v1.30.0" {
		t.Errorf("identity fields wrong: %+v", cluster)
	}
	if cluster.Env != "dev" || cluster.ConnectionMode != "gateway" {
		t.Errorf("ConfigJSON fields wrong: env=%q mode=%q", cluster.Env, cluster.ConnectionMode)
	}
	if cluster.NodeCount != 0 {
		t.Errorf("node_count must not be claimed into the legacy column: %d", cluster.NodeCount)
	}
	if len(cluster.Tags) != 2 || cluster.Tags[0] != "a" || cluster.Tags[1] != "b" {
		t.Errorf("tags procurement wrong: %v", cluster.Tags)
	}
	if cluster.GatewayID == nil || *cluster.GatewayID != gatewayID {
		t.Errorf("gateway id not resolved from the column: %v", cluster.GatewayID)
	}
	if cluster.Gateway.Name != "gw-a" {
		t.Errorf("gateway preload wrong: %+v", cluster.Gateway)
	}
	if cluster.MonitorDatasourceID == nil || *cluster.MonitorDatasourceID != monitorID {
		t.Errorf("monitor datasource id not procured: %v", cluster.MonitorDatasourceID)
	}
	if cluster.MonitorDatasource.Name != "prom" {
		t.Errorf("monitor datasource preload wrong: %+v", cluster.MonitorDatasource)
	}
	if cluster.KubeConfig != "kubeconfig-plaintext-material" {
		t.Errorf("kubeconfig not decrypted from the SecretRef: %q", cluster.KubeConfig)
	}
	if cluster.LastSyncAt != nil {
		t.Errorf("LastSyncAt must stay nil (VOLATILE 제외 — P1-D-5): %v", cluster.LastSyncAt)
	}
	_ = conn
}

// TestK8sClusterConfigGatewayProcurement — F6 문자열 파싱 계약: gateway_id
// 칼럼이 비어 있고 ConfigJSON에 문자열 id가 실린 형상(register-k8s CLI 규약 —
// H0가 문자열로 기록)에서 *uint로 파싱해 조달한다. gateway_id 칼럼 우선순위도
// 함께 단얫한다.
func TestK8sClusterConfigGatewayProcurement(t *testing.T) {
	svc := newK8sSourceService(t)
	db := svc.db

	seedK8sSourceChain(t, db, 7, infraModel.ProviderConnection{
		Name: "registered", Endpoint: "https://r:6443", Status: "active", Version: "v1.31.2",
		SourceModel: "k8s_cluster", SourceID: 7,
		ConfigJSON: map[string]any{
			"connection_mode": "gateway", "gateway_id": "9", "monitor_datasource_id": "3",
		},
	}, "registered-kubeconfig")

	cluster, err := svc.GetK8sCluster(7)
	if err != nil {
		t.Fatalf("GetK8sCluster(7): %v", err)
	}
	if cluster.GatewayID == nil || *cluster.GatewayID != 9 {
		t.Errorf("ConfigJSON gateway_id string not parsed: %v", cluster.GatewayID)
	}
	if cluster.MonitorDatasourceID == nil || *cluster.MonitorDatasourceID != 3 {
		t.Errorf("ConfigJSON monitor_datasource_id string not parsed: %v", cluster.MonitorDatasourceID)
	}

	// Malformed ids read as absent — never panic, never a bogus pointer.
	svc2 := newK8sSourceService(t)
	seedK8sSourceChain(t, svc2.db, 8, infraModel.ProviderConnection{
		Name: "broken-ids", Endpoint: "https://b:6443", Status: "active",
		SourceModel: "k8s_cluster", SourceID: 8,
		ConfigJSON: map[string]any{"gateway_id": "not-a-number", "monitor_datasource_id": ""},
	}, "broken-kubeconfig")
	cluster2, err := svc2.GetK8sCluster(8)
	if err != nil {
		t.Fatalf("GetK8sCluster(8): %v", err)
	}
	if cluster2.GatewayID != nil || cluster2.MonitorDatasourceID != nil {
		t.Errorf("malformed ids must read as absent: gw=%v ds=%v", cluster2.GatewayID, cluster2.MonitorDatasourceID)
	}
}

// TestK8sClusterAbsentAndStale — 오류 계약 승계: 미해상 id는
// gorm.ErrRecordNotFound(legacy First()와 동일), stale 행은 부재다(§5.4c).
func TestK8sClusterAbsentAndStale(t *testing.T) {
	svc := newK8sSourceService(t)
	db := svc.db

	seedK8sSourceChain(t, db, 5, infraModel.ProviderConnection{
		Name: "retired", Endpoint: "https://x:6443", Status: "active",
		SourceModel: "k8s_cluster", SourceID: 5, StaleSource: true,
		ConfigJSON: map[string]any{"connection_mode": "direct"},
	}, "stale-kubeconfig")

	if _, err := svc.GetK8sCluster(5); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("stale source must read as absent, got %v", err)
	}
	if _, err := svc.GetK8sCluster(999); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("missing id must surface gorm.ErrRecordNotFound, got %v", err)
	}
	// id=0 가드(리뷰 HIGH): register-k8s 체인은 source_id=0이므로 무가드면
	// 0번 요청이 미설정 체인에 매치된다 — 0은 항상 미해상이어야 한다.
	if _, err := svc.GetK8sCluster(0); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("id=0 must be rejected as not-found, got %v", err)
	}
}

// TestCountK8sClustersByGateway — S6 등가 조회: 칼럼·ConfigJSON 두 체인
// 형상을 모두 세고 stale 행은 제외한다(delete-protection 카운트).
func TestCountK8sClustersByGateway(t *testing.T) {
	svc := newK8sSourceService(t)
	db := svc.db

	columnGw := uint(11)
	seedK8sSourceChain(t, db, 1, infraModel.ProviderConnection{
		Name: "col", Endpoint: "https://c:6443", Status: "active",
		GatewayID: &columnGw, SourceModel: "k8s_cluster", SourceID: 1,
	}, "k1")
	seedK8sSourceChain(t, db, 2, infraModel.ProviderConnection{
		Name: "cfg", Endpoint: "https://f:6443", Status: "active",
		SourceModel: "k8s_cluster", SourceID: 2,
		ConfigJSON: map[string]any{"gateway_id": "11"},
	}, "k2")
	staleGw := uint(11)
	stale := seedK8sSourceChain(t, db, 3, infraModel.ProviderConnection{
		Name: "gone", Endpoint: "https://g:6443", Status: "active",
		GatewayID: &staleGw, SourceModel: "k8s_cluster", SourceID: 3, StaleSource: true,
	}, "k3")
	if err := db.Model(&infraModel.ProviderConnection{}).Where("id = ?", stale.ID).
		Update("stale_source", true).Error; err != nil {
		t.Fatalf("mark stale: %v", err)
	}

	count, err := svc.countK8sClustersByGateway(11)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 2 {
		t.Errorf("gateway 11 count = %d, want 2 (column + config, stale excluded)", count)
	}
	if other, err := svc.countK8sClustersByGateway(12); err != nil || other != 0 {
		t.Errorf("gateway 12 count = %d err=%v, want 0 nil", other, err)
	}
}
