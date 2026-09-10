// k8s_projection_test.go — G0 읽기 3건 소스 전환 검증(phase6-plan §3.1·§J8,
// claims C52·C53·C69). 기대값 원천: flag OFF = legacy 동작(k8s.go 특성),
// flag ON = P 트랙이 3회 compare pass로 잠근 조립 계약(k8sassembly_test.go
// fixture를 관측 입력으로 재단). 응답 형상(§12 #1)은 반환 타입 자체
// (K8sClusterView·K8sClusterDetail·K8sCluster)으로 고정되므로 본 테스트는
// 값 매핑과 ID 공간을 단얫한다.
package service

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/ssh"

	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/inventory"
	infraModel "ops-admin/backend/internal/infra/model"
	"ops-admin/backend/model"
)

// newProjectionDB — G0 투영 경로가 읽는 테이블만 갖춘 sqlite 인메모리 DB.
// V2 테이블 5종(P 산출물 소비) + legacy 조인 대상(asset_gateway·
// monitor_datasource) + legacy 소스 행(k8s_cluster).
func newProjectionDB(t *testing.T) *gorm.DB {
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
		&infraModel.InventorySyncRun{},
		&infraModel.InfraResource{},
		&infraModel.ResourceObservation{},
		&model.K8sCluster{},
		&model.AssetGateway{},
		&model.MonitorDatasource{},
	); err != nil {
		t.Fatal(err)
	}
	return db
}

// newProjectionService — k8sState를 New()와 동일하게 채운 Service. 캐시·
// singleflight 껍질(GetK8sClusterDetail)을 지나는 분기 검증에 필요하다
// (k8s_clientstate.go 구성 계약 — D2 review MEDIUM #1).
func newProjectionService(t *testing.T) *Service {
	return &Service{
		db: newProjectionDB(t),
		k8sState: &k8sClientState{
			gatewaySSHClients: make(map[uint]*ssh.Client),
			k8sOverviewCache:  make(map[uint]k8sOverviewCacheEntry),
		},
	}
}

// seedV2K8sCluster — 백필 체인 형상(source_model/source_id)의 V2 소스 1세트:
// 커넥션 + 컨텍스트 + 공개 generation(성공·커밋) + 노드 2(1 경보)·네임스페이스 1·
// 파드 1 관측. 라우팅 정보는 ConfigJSON(백필 승계 계약).
func seedV2K8sCluster(t *testing.T, db *gorm.DB, legacyID uint, opts func(*infraModel.ProviderConnection)) (connID, contextID uint) {
	t.Helper()
	gatewayID := uint(7)
	monitorID := 3
	// 조립 조인(buildClusterView)·info 프리로드 대응의 대상 행 — 중복 심음 방지.
	var joins int64
	if err := db.Model(&model.AssetGateway{}).Where("id = ?", gatewayID).Count(&joins).Error; err != nil {
		t.Fatal(err)
	}
	if joins == 0 {
		if err := db.Create(&model.AssetGateway{ID: gatewayID, Name: "bastion-gw"}).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Create(&model.MonitorDatasource{ID: uint(monitorID), Name: "prom-prod"}).Error; err != nil {
			t.Fatal(err)
		}
	}
	conn := infraModel.ProviderConnection{
		UID:          fmt.Sprintf("conn-k8s-g0-%d", legacyID),
		ProviderType: "kubernetes",
		Name:         fmt.Sprintf("seed-cluster-%d", legacyID),
		Endpoint:     "https://seed:6443",
		Status:       "running",
		Version:      "v1.29.4",
		GatewayID:    &gatewayID,
		SourceModel:  "k8s_cluster",
		SourceID:     legacyID,
		ConfigJSON: contract.JSONMap{
			"env":                   "prod",
			"tags":                  []any{"edge", "seed"},
			"connection_mode":       "direct",
			"version":               "v1.29.4",
			"node_count":            2,
			"description":           "g0 seed",
			"monitor_datasource_id": monitorID,
		},
	}
	if opts != nil {
		opts(&conn)
	}
	if err := db.Create(&conn).Error; err != nil {
		t.Fatal(err)
	}
	pctx := infraModel.ProviderContext{
		UID:          fmt.Sprintf("ctx-k8s-g0-%d", legacyID),
		ConnectionID: conn.ID,
		Kind:         "cluster",
		ExternalID:   fmt.Sprintf("%d", legacyID),
		Name:         conn.Name,
	}
	if err := db.Create(&pctx).Error; err != nil {
		t.Fatal(err)
	}

	committed := time.Date(2026, 9, 10, 9, 0, 0, 0, time.UTC)
	run := infraModel.InventorySyncRun{
		UID:          fmt.Sprintf("gen-g0-%d", legacyID),
		ConnectionID: conn.ID,
		ContextID:    pctx.ID,
		Mode:         "full",
		Status:       inventory.RunStatusSucceeded,
		StartedAt:    committed.Add(-time.Minute),
		CommittedAt:  &committed,
	}
	if err := db.Create(&run).Error; err != nil {
		t.Fatal(err)
	}

	const stamp = "2026-01-01T00:00:00Z"
	healthy := contract.JSONMap{
		"roles": []string{"control-plane"}, "healthState": "healthy", "readyCondition": "True",
		"kubeletVersion": "v1.29.1", "internalIP": "10.0.0.1", "osImage": "Ubuntu 22.04",
		"allocatableCoresGB": 4.0, "allocatableMemoryGB": 8.0, "capacityPods": 110,
		"podCIDRs": []string{"10.244.1.0/24"},
	}
	degraded := contract.JSONMap{
		"roles": []string{"worker"}, "healthState": "degraded",
		"readyCondition": "False", "unschedulable": true,
	}
	pod := contract.JSONMap{
		"phase": "Running", "nodeName": "n-1", "restartCount": 0,
		"containers": []any{map[string]any{
			"name": "c", "requests": map[string]any{"cpuMilli": 500, "memBytes": 134217728},
		}},
	}

	resources := []struct {
		kind, subtype, name, namespace string
		normalized                     contract.JSONMap
	}{
		{"orchestration.node", "", "n-1", "", healthy},
		{"orchestration.node", "", "n-2", "", degraded},
		{"orchestration.pod", "", "web-1", "app", pod},
		{"orchestration.namespace", "", "app", "", contract.JSONMap{"phase": "Active"}},
	}
	for _, res := range resources {
		row := infraModel.InfraResource{
			UID:         fmt.Sprintf("res-%s-%d", res.name, legacyID),
			ContextID:   pctx.ID,
			Kind:        res.kind,
			Subtype:     res.subtype,
			ExternalID:  res.name,
			ExternalURN: fmt.Sprintf("urn:k8s:g0:%s:%d", res.name, legacyID),
			DisplayName: res.name,
			FirstSeenAt: committed,
			LastSeenAt:  committed,
		}
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
		raw := contract.JSONMap{"name": res.name, "creationTimestamp": stamp}
		if res.namespace != "" {
			raw["namespace"] = res.namespace
		}
		if err := db.Create(&infraModel.ResourceObservation{
			ResourceID:     row.ID,
			GenerationUID:  run.UID,
			NormalizedJSON: res.normalized,
			RawJSON:        raw,
			ObservedAt:     committed,
		}).Error; err != nil {
			t.Fatal(err)
		}
	}
	return conn.ID, pctx.ID
}

// seedLegacyK8sCluster — flag OFF 회귀 비교용 legacy 행.
func seedLegacyK8sCluster(t *testing.T, db *gorm.DB, name, kubeconfig string) model.K8sCluster {
	t.Helper()
	cluster := model.K8sCluster{
		Name: name, Status: "running", APIServer: "https://legacy:6443",
		Version: "v1.28.0", Env: "dev", KubeConfig: kubeconfig,
	}
	if err := db.Create(&cluster).Error; err != nil {
		t.Fatal(err)
	}
	return cluster
}

// TestProjectK8sClusterList — cluster/list V2 소스: 백필 체인만 source_id
// 오름차순(legacy Order("id asc") 대응)으로 노출하고, 뷰의 id는 conn.ID가
// 아니라 legacy source_id다(ID 공간 판단 — k8s_projection.go 패키지 문서).
// stale 커넥션(§5.4c)과 source 쌍이 없는 커넥션(register-k8s 체인 — I-a S5
// 전까지 목록 제외 판정)은 부재로 읽힌다.
func TestProjectK8sClusterList(t *testing.T) {
	t.Setenv("V2_READ_SOURCE_K8S", "1")
	svc := newProjectionService(t)

	// 일부러 conn.ID 오름차순과 어긋나게: legacy 5 → 2 순서로 심는다.
	seedV2K8sCluster(t, svc.db, 5, nil)
	seedV2K8sCluster(t, svc.db, 2, nil)
	seedV2K8sCluster(t, svc.db, 9, func(c *infraModel.ProviderConnection) { c.StaleSource = true })
	seedV2K8sCluster(t, svc.db, 11, func(c *infraModel.ProviderConnection) { c.SourceModel = "" })

	list, err := svc.ListK8sClusters()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("list length = %d, want 2 (stale·비source 체인 제외): %+v", len(list), list)
	}
	if list[0].ID != 2 || list[1].ID != 5 {
		t.Fatalf("list ids = [%d %d], want [2 5] — legacy source_id 공간·오름차순", list[0].ID, list[1].ID)
	}
	byID := map[uint]model.K8sClusterView{list[0].ID: list[0], list[1].ID: list[1]}
	view := byID[5]
	if view.Name != "seed-cluster-5" {
		t.Fatalf("name = %q, want conn.Name 승계", view.Name)
	}
	if view.NodeCount != 2 {
		t.Fatalf("nodeCount = %d, want ConfigJSON node_count 2", view.NodeCount)
	}
	if view.Env != "prod" || view.APIServer != "https://seed:6443" || view.Version != "v1.29.4" {
		t.Fatalf("conn 속성 승계 누락: %+v", view)
	}
	if view.ConnectionMode != "direct" {
		t.Fatalf("connectionMode = %q, want ConfigJSON connection_mode", view.ConnectionMode)
	}
	if view.GatewayName != "bastion-gw" || view.MonitorDatasourceName != "prom-prod" {
		t.Fatalf("조립 조인 누락: gw=%q ds=%q", view.GatewayName, view.MonitorDatasourceName)
	}
	// 경보 승격은 P가 잠근 조립 계약(alertCount>0 → warning) — fixture에
	// degraded 노드(n-2)가 있으므로 목록 뷰도 warning이다.
	if view.Status != "warning" || view.StatusText == "" {
		t.Fatalf("status = %q/%q, want 경보 승격 warning", view.Status, view.StatusText)
	}
}

// TestProjectK8sClusterInfo — cluster/info 착지점(컨트롤러 :32 직접 호출).
// C69: V2 소스에서 kubeConfig는 항상 빈 값으로 직렬화된다(마스킹 계약 —
// §J8 R-I·§12 #1 유일 예외). flag OFF면 legacy 행을 그대로 돌려 기본 소스가
// legacy임을 고정한다(§11 롤백 계약).
func TestProjectK8sClusterInfo(t *testing.T) {
	t.Setenv("V2_READ_SOURCE_K8S", "1")
	svc := newProjectionService(t)
	seedV2K8sCluster(t, svc.db, 3, nil)

	cluster, err := svc.ProjectK8sClusterInfo(3)
	if err != nil {
		t.Fatal(err)
	}
	if cluster.ID != 3 || cluster.Name != "seed-cluster-3" {
		t.Fatalf("id/name = %d/%q, want legacy id 공간 3/seed-cluster-3", cluster.ID, cluster.Name)
	}
	if cluster.KubeConfig != "" {
		t.Fatalf("C69 위반: V2 info가 kubeConfig를 실어 반환 (%d자)", len(cluster.KubeConfig))
	}
	if cluster.Gateway.Name != "bastion-gw" {
		t.Fatalf("gateway 프리로드 대응 누락: %+v", cluster.Gateway)
	}
	if cluster.MonitorDatasource.Name != "prom-prod" || cluster.MonitorDatasourceID == nil || *cluster.MonitorDatasourceID != 3 {
		t.Fatalf("monitor datasource 프리로드 대응 누락: %+v", cluster.MonitorDatasource)
	}
	if cluster.APIServer != "https://seed:6443" || cluster.NodeCount != 2 || cluster.Env != "prod" {
		t.Fatalf("conn 속성 승계 누락: %+v", cluster)
	}

	// flag OFF — 기본 legacy: 같은 id로 legacy 행을 돌려준다(마스킹 없음).
	legacy := seedLegacyK8sCluster(t, svc.db, "legacy-info", "apiVersion: v1\nkind: Config")
	t.Setenv("V2_READ_SOURCE_K8S", "")
	got, err := svc.ProjectK8sClusterInfo(legacy.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != legacy.ID || got.Name != "legacy-info" || got.KubeConfig != "apiVersion: v1\nkind: Config" {
		t.Fatalf("flag OFF가 legacy 소스를 대체했다: %+v", got)
	}
}

// TestProjectK8sClusterDetail — cluster/detail V2 소스: 조립물 섹션과 경보
// 승격(degraded 노드 → warning), cluster.id의 legacy 공간 회복, 미공개
// generation 커넥션의 안내 에러를 단얫한다.
func TestProjectK8sClusterDetail(t *testing.T) {
	t.Setenv("V2_READ_SOURCE_K8S", "1")
	svc := newProjectionService(t)
	seedV2K8sCluster(t, svc.db, 4, nil)

	detail, err := svc.GetK8sClusterDetail(4)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Cluster.ID != 4 {
		t.Fatalf("detail.cluster.id = %d, want legacy source_id 4", detail.Cluster.ID)
	}
	if len(detail.Nodes) != 2 || len(detail.Namespaces) != 1 || len(detail.Pods) != 1 {
		t.Fatalf("섹션 행수 = nodes %d·ns %d·pods %d, want 2·1·1",
			len(detail.Nodes), len(detail.Namespaces), len(detail.Pods))
	}
	if detail.Cluster.Status != "warning" {
		t.Fatalf("경보 승격 누락(status=%q) — degraded 노드 1은 경보 1건", detail.Cluster.Status)
	}
	if detail.Overview.AlertCount != 1 {
		t.Fatalf("alertCount = %d, want 1", detail.Overview.AlertCount)
	}

	// 공개 generation이 없는 커넥션 — detail은 안내 에러로 끝난다.
	if _, err := svc.GetK8sClusterDetail(404); err == nil || !strings.Contains(err.Error(), "sync-inventory") {
		t.Fatalf("미동기화 커넥션 에러 = %v, want sync-inventory 안내", err)
	}
}

// TestK8sReadSourceFlagDefaultLegacy — 플래그 부재 기본값은 legacy(§11): list는
// k8s_cluster 행을, detail은 legacy 라이브 경로(kubeconfig 파싱 에러로 관측)를
// 쓴다. G1이 제거할 legacy 분기가 G0에서 그대로 보존됨을 고정한다(C36① 전제).
func TestK8sReadSourceFlagDefaultLegacy(t *testing.T) {
	svc := newProjectionService(t)
	legacy := seedLegacyK8sCluster(t, svc.db, "legacy-a", "not-a-valid-kubeconfig")
	seedV2K8sCluster(t, svc.db, 21, nil)

	list, err := svc.ListK8sClusters()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != legacy.ID || list[0].Name != "legacy-a" {
		t.Fatalf("flag OFF list = %+v, want legacy 행만", list)
	}

	if _, err := svc.GetK8sClusterDetail(legacy.ID); err == nil || !strings.Contains(err.Error(), "failed to parse kubeconfig") {
		t.Fatalf("flag OFF detail 에러 = %v, want legacy uncached 파싱 실패", err)
	}
}

// TestK8sClusterFromProjectionDBErrorPropagation — 리뷰 HIGH 수정 단얫:
// gateway·monitor_datasource 조회의 ErrRecordNotFound 외 DB 에러는 삼켜지지
// 않고 전파된다(빈 클러스터가 조용히 반환되면 안 된다). 행 부재만 영값으로
// 용인된다(Preload 대응 유지).
func TestK8sClusterFromProjectionDBErrorPropagation(t *testing.T) {
	svc := newProjectionService(t)
	gatewayID := uint(9)
	monitorID := uint(5)

	// gateway 테이블 결손 — "no such table"(ErrRecordNotFound 아님) 전파.
	view := model.K8sClusterView{ID: 6, Name: "seed-6", GatewayID: &gatewayID, MonitorDatasourceID: &monitorID}
	if err := svc.db.Migrator().DropTable(&model.AssetGateway{}); err != nil {
		t.Fatal(err)
	}
	_, err := svc.k8sClusterFromProjection(view)
	if err == nil {
		t.Fatal("gateway 조회 DB 에러가 삼켜졌다 — 전파 필요")
	}
	if !strings.Contains(err.Error(), "gateway load") {
		t.Fatalf("에러 = %v, want gateway load 전파", err)
	}

	// gateway는 멀쩡하고 monitor_datasource 테이블 결손 — 동일 전파.
	svc2 := newProjectionService(t)
	if err := svc2.db.Migrator().DropTable(&model.MonitorDatasource{}); err != nil {
		t.Fatal(err)
	}
	_, err = svc2.k8sClusterFromProjection(view)
	if err == nil || !strings.Contains(err.Error(), "monitor datasource load") {
		t.Fatalf("에러 = %v, want monitor datasource load 전파", err)
	}

	// 행 부재(ErrRecordNotFound)는 영값으로 용인 — 전파 아님.
	svc3 := newProjectionService(t)
	cluster, err := svc3.k8sClusterFromProjection(view)
	if err != nil {
		t.Fatalf("행 부재를 에러로 승격했다: %v", err)
	}
	if cluster.ID != 6 || cluster.Name != "seed-6" || cluster.Gateway.Name != "" || cluster.MonitorDatasource.Name != "" {
		t.Fatalf("행 부재 용인 경로가 값을 훼손했다: %+v", cluster)
	}
}

// TestK8sReadSourceFlagValues — 리뷰 MEDIUM 수정 단얫: 플래그 판정은
// trim·lower 정규화(k8sReadSourceV2)로 ""·"0"·"false" 전부 OFF(legacy 소스),
// "1"만 ON(V2 소스)이다. 세 착지점(list·detail·info)이 같은 판정을 쓴다.
func TestK8sReadSourceFlagValues(t *testing.T) {
	svc := newProjectionService(t)
	legacy := seedLegacyK8sCluster(t, svc.db, "legacy-f", "not-a-valid-kubeconfig")

	for _, value := range []string{"", "0", "false", " 0 ", "FALSE"} {
		t.Setenv("V2_READ_SOURCE_K8S", value)
		list, err := svc.ListK8sClusters()
		if err != nil {
			t.Fatal(err)
		}
		if len(list) != 1 || list[0].ID != legacy.ID {
			t.Fatalf("flag %q list = %+v, want legacy 소스(k8s_cluster 행)", value, list)
		}
		if _, err := svc.GetK8sClusterDetail(legacy.ID); err == nil || !strings.Contains(err.Error(), "failed to parse kubeconfig") {
			t.Fatalf("flag %q detail = %v, want legacy uncached 경로", value, err)
		}
		info, err := svc.ProjectK8sClusterInfo(legacy.ID)
		if err != nil || info.ID != legacy.ID || info.Name != "legacy-f" {
			t.Fatalf("flag %q info = %+v/%v, want legacy 행", value, info, err)
		}
	}

	// ON — "1": 같은 id가 V2 소스로 해상된다(V2 커넥션 부재 → 안내 에러).
	t.Setenv("V2_READ_SOURCE_K8S", "1")
	if _, err := svc.ProjectK8sClusterInfo(legacy.ID); err == nil || !strings.Contains(err.Error(), "no live V2 source") {
		t.Fatalf("flag 1 info = %v, want V2 소스 해상", err)
	}
	if list, err := svc.ListK8sClusters(); err != nil || len(list) != 0 {
		t.Fatalf("flag 1 list = %+v/%v, want V2 소스(빈 목록)", list, err)
	}
}
