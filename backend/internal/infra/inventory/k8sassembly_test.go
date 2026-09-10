// k8sassembly_test.go — 조립 라이브러리 검증(계획 P1-D — TestAssembleK8sClusterDetail
// 명명 고정). 기대값의 권위는 v1 특성 표(service/k8s_*_test.go)와 그 산식이다:
// 본 테스트는 v1 특성 계약을 V2 관측 입력으로 재단한다.
package inventory_test

import (
	"testing"

	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/inventory"
	"ops-admin/backend/internal/infra/model"
)

// asmRow plants one projected observation row.
func asmRow(kind, subtype, name, namespace string, normalized contract.JSONMap, raw contract.JSONMap) inventory.ProjectedResource {
	if raw == nil {
		raw = contract.JSONMap{"name": name}
	}
	if namespace != "" {
		raw["namespace"] = namespace
	}
	return inventory.ProjectedResource{
		UID: "u-" + name, Kind: kind, Subtype: subtype, ExternalID: name, DisplayName: name,
		Normalized: normalized, Raw: raw,
	}
}

const asmTimestamp = "2026-01-01T00:00:00Z" // >30일 — humanizeAge 날짜 폴백(결정적)

// TestAssembleK8sClusterDetail — 전 섹션 조립: cluster 속성(conn+조인), overview
// 집계(①②), distribution(⑤ 화이트리스트), nodes 3치 상태, pod RS 체인, batch
// 유도(Updated=Active·Available=Succeeded), network nodePort·endpoints,
// advancedNetwork, configStorage 행 형상.
func TestAssembleK8sClusterDetail(t *testing.T) {
	db := newInventoryDB(t)
	// v1 도메인 테이블(조립 조인 대상) — 인프라 migrate 체인 밖이라 최소형 생성.
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db.DB: %v", err)
	}
	for _, stmt := range []string{
		"CREATE TABLE asset_gateway (id INTEGER PRIMARY KEY, name TEXT)",
		"CREATE TABLE monitor_datasource (id INTEGER PRIMARY KEY, name TEXT)",
		"INSERT INTO asset_gateway (id, name) VALUES (7, 'bastion-gw')",
		"INSERT INTO monitor_datasource (id, name) VALUES (3, 'prom-prod')",
	} {
		if _, err := sqlDB.Exec(stmt); err != nil {
			t.Fatalf("exec %q: %v", stmt, err)
		}
	}

	gatewayID := uint(7)
	conn := model.ProviderConnection{
		UID: "asm-conn", ProviderType: "kubernetes", Name: "seed cluster", Endpoint: "https://seed:6443",
		Status: "active", Version: "v1.29.4", GatewayID: &gatewayID,
		ConfigJSON: contract.JSONMap{
			"env": "prod", "tags": []any{"edge", "seed"}, "connection_mode": "gateway",
			"version": "v1.29.4", "node_count": 2, "description": "seed",
			"monitor_datasource_id": 3, // 판정 ③ — 등록 경로 흡수(Id), Name은 조인.
		},
	}

	rows := []inventory.ProjectedResource{
		// nodes — b-node Ready, c-node NotReady+unschedulable(경보 1).
		asmRow("orchestration.node", "", "b-node", "", contract.JSONMap{
			"roles": []string{"control-plane"}, "healthState": "healthy", "readyCondition": "True",
			"kubeletVersion": "v1.29.1", "internalIP": "10.0.0.1", "osImage": "Ubuntu 22.04",
			"allocatableCoresGB": 4.0, "allocatableMemoryGB": 8.0, "capacityPods": 110,
			"podCIDRs": []string{"10.244.1.0/24"},
		}, contract.JSONMap{"creationTimestamp": asmTimestamp}),
		asmRow("orchestration.node", "", "c-node", "", contract.JSONMap{
			"roles": []string{"worker"}, "healthState": "degraded", "readyCondition": "False",
			"unschedulable": true,
		}, contract.JSONMap{"creationTimestamp": asmTimestamp}),
		// pods — web-1은 접두 폴백, web-2는 RS 체인.
		asmRow("orchestration.pod", "", "web-1", "app", contract.JSONMap{
			"phase": "Running", "nodeName": "b-node", "restartCount": 0,
			"containers": []any{map[string]any{"name": "c", "requests": map[string]any{"cpuMilli": 500, "memBytes": 134217728}}},
		}, contract.JSONMap{"creationTimestamp": asmTimestamp}),
		asmRow("orchestration.pod", "", "web-2", "app", contract.JSONMap{
			"phase": "Running", "restartCount": 2,
		}, contract.JSONMap{
			"creationTimestamp": asmTimestamp,
			"ownerReferences":   []any{map[string]any{"uid": "u-rs", "kind": "ReplicaSet", "name": "web-rs"}},
		}),
		asmRow("orchestration.workload", "replicaset", "web-rs", "app", nil, contract.JSONMap{
			"creationTimestamp": asmTimestamp,
			"ownerReferences":   []any{map[string]any{"uid": "u-dp", "kind": "Deployment", "name": "web"}},
		}),
		// workloads — apps 1종 + job(합산 분모) + cronjob(suspend).
		asmRow("orchestration.workload", "deployment", "web", "app", contract.JSONMap{
			"replicas": 3, "readyReplicas": 2, "updatedReplicas": 1, "availableReplicas": 2,
			"containers": []any{map[string]any{"name": "c", "requests": map[string]any{"cpuMilli": 100, "memBytes": 1000}}},
		}, contract.JSONMap{"creationTimestamp": asmTimestamp}),
		asmRow("orchestration.workload", "job", "nightly", "app", contract.JSONMap{
			"completions": 4, "active": 1, "succeeded": 2, "failed": 1,
			"replicas": 0, "readyReplicas": 0,
		}, contract.JSONMap{"creationTimestamp": asmTimestamp}),
		asmRow("orchestration.workload", "cronjob", "cleanup", "app", contract.JSONMap{
			"schedule": "@daily", "active": 0, "suspend": true, "replicas": 0, "readyReplicas": 0,
		}, contract.JSONMap{"creationTimestamp": asmTimestamp}),
		// namespace·network·endpoint.
		asmRow("orchestration.namespace", "", "app", "", contract.JSONMap{"phase": "Active"}, contract.JSONMap{"creationTimestamp": asmTimestamp}),
		asmRow("network.load_balancer", "service", "alpha", "default", contract.JSONMap{
			"type": "ClusterIP", "clusterIP": "10.0.0.1", "externalIP": "1.2.3.4",
			"ports": []any{map[string]any{"port": 80, "nodePort": 30080, "protocol": "TCP"}},
		}, contract.JSONMap{"creationTimestamp": asmTimestamp}),
		asmRow("network.endpoint", "", "alpha", "default", contract.JSONMap{"readyAddresses": 3}, contract.JSONMap{"creationTimestamp": asmTimestamp}),
		asmRow("network.load_balancer", "ingress", "b-ing", "default", contract.JSONMap{
			"hosts": []string{"a.example"}, "address": "1.1.1.1", "tls": "Enabled",
		}, contract.JSONMap{"creationTimestamp": asmTimestamp}),
		// advancedNetwork.
		asmRow("network.gateway", "", "gw1", "default", contract.JSONMap{
			"gatewayClassName": "istio", "hosts": []string{"*.example"}, "addresses": []string{"10.0.0.9"}, "ports": []string{"8080/TCP"},
		}, contract.JSONMap{"creationTimestamp": asmTimestamp}),
		asmRow("network.http_route", "", "rt1", "default", contract.JSONMap{
			"hostnames": []string{"rt.example"}, "parents": []string{"gw1"}, "targets": []string{"svc:80 (10%)"},
		}, contract.JSONMap{"creationTimestamp": asmTimestamp}),
		// configStorage — ⑤ 화이트리스트 configmap + secret + pv/pvc.
		asmRow("orchestration.configmap", "", "kubeadm-config", "kube-system", contract.JSONMap{
			"dataKeys": []string{"ClusterConfiguration"}, "serviceCIDR": "10.96.0.0/12", "podSubnetCIDR": "10.244.0.0/16",
		}, contract.JSONMap{"creationTimestamp": asmTimestamp}),
		asmRow("orchestration.secret", "", "sec1", "default", contract.JSONMap{"type": "Opaque", "dataKeys": []string{"tls.key"}}, contract.JSONMap{"creationTimestamp": asmTimestamp}),
		asmRow("storage.volume", "pvc", "claim", "team-a", contract.JSONMap{
			"phase": "Bound", "capacityGB": 5.0, "capacityRaw": "5Gi", "storageClassName": "standard", "accessModes": []string{"RWO"},
		}, contract.JSONMap{"creationTimestamp": asmTimestamp}),
		asmRow("storage.volume", "persistent_volume", "vol", "", contract.JSONMap{
			"phase": "Available", "capacityGB": 10.0, "capacityRaw": "10Gi", "storageClassName": "standard",
			"accessModes": []string{"RWO"}, "namespaceScope": "team-a", "sourceType": "hostPath",
			"sourcePath": "/mnt/data", "reclaimPolicy": "Retain",
		}, contract.JSONMap{"creationTimestamp": asmTimestamp}),
	}

	detail, err := inventory.AssembleK8sClusterDetail(db, &conn, rows)
	if err != nil {
		t.Fatalf("AssembleK8sClusterDetail: %v", err)
	}

	// --- cluster (conn + 조인 + 경보 승격) ---
	cluster := detail.Cluster
	if cluster.Name != "seed cluster" || cluster.APIServer != "https://seed:6443" || cluster.Version != "v1.29.4" {
		t.Errorf("cluster head = %+v", cluster)
	}
	// v1 getK8sClusterDetailUncached: AlertCount>0이면 Status를 warning으로
	// 승격하고 StatusText는 k8sStatusText("warning")다.
	if cluster.Status != "warning" || cluster.StatusText != "Partial Alerts" {
		t.Errorf("cluster status = %q/%q, want warning/Partial Alerts (경보 1 — v1 승격 계약)", cluster.Status, cluster.StatusText)
	}
	if cluster.GatewayName != "bastion-gw" {
		t.Errorf("gatewayName = %q, want bastion-gw", cluster.GatewayName)
	}
	if cluster.MonitorDatasourceID == nil || *cluster.MonitorDatasourceID != 3 || cluster.MonitorDatasourceName != "prom-prod" {
		t.Errorf("monitor = %v/%q, want 3/prom-prod (판정 ③)", cluster.MonitorDatasourceID, cluster.MonitorDatasourceName)
	}
	if cluster.NodeCount != 2 || cluster.ConnectionMode != "gateway" || cluster.Env != "prod" || len(cluster.Tags) != 2 {
		t.Errorf("cluster config projection = count:%d mode:%q env:%q tags:%v", cluster.NodeCount, cluster.ConnectionMode, cluster.Env, cluster.Tags)
	}

	// --- overview (집계 — 경보: c-node 1건. pod는 전부 Running) ---
	overview := detail.Overview
	if overview.AlertCount != 1 {
		t.Errorf("alertCount = %d, want 1", overview.AlertCount)
	}
	if overview.HealthScore != 92 {
		t.Errorf("healthScore = %d, want 92 (100-8)", overview.HealthScore)
	}
	// 요청치 합: cpu 500m pod1 / alloc 4000m → 12.5% · mem 128Mi / 8Gi → 1.6%.
	if overview.CPUUsage != "12.5%" || overview.MemoryUsage != "1.6%" {
		t.Errorf("usage = %q/%q, want 12.5%%/1.6%%", overview.CPUUsage, overview.MemoryUsage)
	}
	if overview.PodUsage != "2 Pods" || overview.RequestRate != "3 Workloads" {
		t.Errorf("counts = %q/%q, want 2 Pods/3 Workloads", overview.PodUsage, overview.RequestRate)
	}
	wantDist := [][2]string{
		{"Cluster Status", "Partial Alerts"}, {"Cluster Version", "v1.29.4"}, {"Node Count", "2 nodes"},
		{"Service CIDR", "10.96.0.0/12"}, {"Pod Network", "10.244.0.0/16"},
	}
	for i, pair := range wantDist {
		if got := overview.Distribution[i]; got.Label != pair[0] || got.Value != pair[1] {
			t.Errorf("distribution[%d] = %q/%q, want %q/%q", i, got.Label, got.Value, pair[0], pair[1])
		}
	}
	if overview.Certificates != nil {
		t.Errorf("certificates = %v, want nil (판정 ④ 결착 전 부재)", overview.Certificates)
	}

	// --- nodes (3치 상태·worker 기본·MB 반올림·파드 카운트) ---
	if len(detail.Nodes) != 2 {
		t.Fatalf("nodes = %d, want 2", len(detail.Nodes))
	}
	bNode := detail.Nodes[0]
	if bNode.Name != "b-node" || bNode.Role != "control-plane" || bNode.Status != "Ready" ||
		bNode.Version != "v1.29.1" || bNode.InternalIP != "10.0.0.1" || bNode.OS != "Ubuntu 22.04" ||
		bNode.CPU != "4" || bNode.Memory != "8590 MB" || bNode.Pods != "1/110" {
		t.Errorf("node[0] = %+v", bNode)
	}
	cNode := detail.Nodes[1]
	if cNode.Status != "NotReady" || cNode.CPU != "-" || cNode.Memory != "-" || cNode.Pods != "0/-" {
		t.Errorf("node[1] = %+v", cNode)
	}

	// --- pods (RS 체인·접두 폴백·nodeName) ---
	if len(detail.Pods) != 2 {
		t.Fatalf("pods = %d, want 2", len(detail.Pods))
	}
	if detail.Pods[0].Name != "web-1" || detail.Pods[0].WorkloadName != "web" || detail.Pods[0].WorkloadType != "Deployment" {
		t.Errorf("pods[0] = %+v", detail.Pods[0])
	}
	if detail.Pods[0].Node != "b-node" || detail.Pods[0].NodeIP != "-" || detail.Pods[0].Status != "Running" {
		t.Errorf("pods[0] fields = %+v", detail.Pods[0])
	}
	if detail.Pods[1].Name != "web-2" || detail.Pods[1].WorkloadName != "web" || detail.Pods[1].Restarts != 2 {
		t.Errorf("pods[1] = %+v (RS 체인 — ReplicaSet web-rs의 Deployment 소유자)", detail.Pods[1])
	}

	// --- workloads (batch 유도: Ready=succeeded/total, Updated=Active, Available=Succeeded) ---
	if len(detail.Workloads) != 3 {
		t.Fatalf("workloads = %d, want 3", len(detail.Workloads))
	}
	cron := detail.Workloads[0] // app/cleanup 정렬 선행
	if cron.Name != "cleanup" || cron.Ready != "Suspended" || cron.Updated != 0 || cron.Available != 0 {
		t.Errorf("cronjob item = %+v", cron)
	}
	job := detail.Workloads[1]
	if job.Name != "nightly" || job.Ready != "2/4" || job.Updated != 1 || job.Available != 2 {
		t.Errorf("job item = %+v (active 1·succeeded 2·failed 1·completions 4)", job)
	}
	deployment := detail.Workloads[2]
	if deployment.Name != "web" || deployment.Ready != "2/3" || deployment.Updated != 1 || deployment.Available != 2 {
		t.Errorf("deployment item = %+v", deployment)
	}
	if deployment.Requests != "100m / 0Mi" || deployment.Limits != "-" {
		t.Errorf("deployment requests/limits = %q/%q, want 100m / 0Mi / -", deployment.Requests, deployment.Limits)
	}

	// --- network (nodePort 포맷·endpoints 집계·Headless 아님) ---
	if len(detail.Network.Services) != 1 {
		t.Fatalf("services = %d, want 1", len(detail.Network.Services))
	}
	svc := detail.Network.Services[0]
	if svc.Name != "alpha" || svc.Type != "ClusterIP" || svc.Ports != "80:30080/TCP" || svc.Endpoints != 3 || svc.ExternalIP != "1.2.3.4" {
		t.Errorf("service item = %+v", svc)
	}
	if len(detail.Network.Ingresses) != 1 || detail.Network.Ingresses[0].Host != "a.example" ||
		detail.Network.Ingresses[0].Address != "1.1.1.1" || detail.Network.Ingresses[0].TLS != "Enabled" {
		t.Errorf("ingress items = %+v", detail.Network.Ingresses)
	}

	// --- advancedNetwork ---
	if len(detail.AdvancedNetwork.GatewayAPIGateways) != 1 {
		t.Fatalf("gateways = %d, want 1", len(detail.AdvancedNetwork.GatewayAPIGateways))
	}
	gw := detail.AdvancedNetwork.GatewayAPIGateways[0]
	if gw.Kind != "Gateway" || gw.Hosts != "*.example" || gw.Address != "10.0.0.9" || gw.Ports != "8080/TCP" || gw.Target != "istio" {
		t.Errorf("gateway item = %+v", gw)
	}
	if len(detail.AdvancedNetwork.HTTPRoutes) != 1 {
		t.Fatalf("httproutes = %d, want 1", len(detail.AdvancedNetwork.HTTPRoutes))
	}
	rt := detail.AdvancedNetwork.HTTPRoutes[0]
	if rt.Kind != "HTTPRoute" || rt.Hosts != "rt.example" || rt.Gateways != "gw1" || rt.Target != "svc:80 (10%)" {
		t.Errorf("httproute item = %+v", rt)
	}

	// --- namespaces (집계 — app ns: pod 2·service 0·workload 3) ---
	if len(detail.Namespaces) != 1 {
		t.Fatalf("namespaces = %d, want 1", len(detail.Namespaces))
	}
	ns := detail.Namespaces[0]
	if ns.Name != "app" || ns.Status != "Active" || ns.Pods != 2 || ns.Services != 0 || ns.Workloads != 3 || ns.CreatedAt != "2026-01-01 00:00" {
		t.Errorf("namespace item = %+v", ns)
	}

	// --- configStorage (pvc 공란 성분·pv 전성분·dataKeys 카운트) ---
	if len(detail.ConfigStorage.ConfigMaps) != 1 || detail.ConfigStorage.ConfigMaps[0].Keys != 1 {
		t.Errorf("configmaps = %+v", detail.ConfigStorage.ConfigMaps)
	}
	if len(detail.ConfigStorage.Secrets) != 1 || detail.ConfigStorage.Secrets[0].Type != "Opaque" {
		t.Errorf("secrets = %+v", detail.ConfigStorage.Secrets)
	}
	if len(detail.ConfigStorage.Storage) != 2 {
		t.Fatalf("storage = %d, want 2", len(detail.ConfigStorage.Storage))
	}
	pv, pvc := detail.ConfigStorage.Storage[0], detail.ConfigStorage.Storage[1]
	if pv.Kind != "PV" || pv.Capacity != "10Gi" || pv.NamespaceScope != "team-a" || pv.SourceType != "hostPath" ||
		pv.Path != "/mnt/data" || pv.NFSServer != "-" || pv.ReclaimPolicy != "Retain" || pv.AccessModes != "RWO" {
		t.Errorf("storage[0](PV) = %+v", pv)
	}
	if pvc.Kind != "PVC" || pvc.Namespace != "team-a" || pvc.Status != "Bound" || pvc.Capacity != "5Gi" ||
		pvc.StorageClass != "standard" || pvc.NamespaceScope != "" || pvc.SourceType != "" {
		t.Errorf("storage[1](PVC) = %+v (pvc 행은 source·scope 공란 — legacy 행 형상)", pvc)
	}
}

// --- v1 특성 계약 미러 (P1-E Z 스택 대비) ---

// TestAssembleNodeItemsV1Contract mirrors TestCharBuildNodeItems(k8s_fetch_test.go)
// — 이름 정렬·역할 없으면 worker·InternalIP 없으면 "-"·메모리 MB(10^6) 반올림·
// 파드 분모 capacity.pods·조건 없는 노드 Unknown. 기대값은 v1 표와 1:1이다.
func TestAssembleNodeItemsV1Contract(t *testing.T) {
	nodeB := asmRow("orchestration.node", "", "b-node", "", contract.JSONMap{
		"roles": []string{"control-plane"}, "readyCondition": "True", "kubeletVersion": "v1.29.1",
		"internalIP": "10.0.0.1", "osImage": "Ubuntu 22.04",
		"allocatableCoresGB": 4.0, "allocatableMemoryGB": 8.0, "capacityPods": 110,
	}, nil)
	nodeC := asmRow("orchestration.node", "", "c-node", "", contract.JSONMap{
		"readyCondition": "False",
	}, nil)
	nodeA := asmRow("orchestration.node", "", "a-node", "", contract.JSONMap{}, nil) // 무조건·무역할
	pods := []inventory.ProjectedResource{
		asmRow("orchestration.pod", "", "p1", "default", contract.JSONMap{"nodeName": "b-node"}, nil),
		asmRow("orchestration.pod", "", "p2", "default", contract.JSONMap{"nodeName": "b-node"}, nil),
		asmRow("orchestration.pod", "", "p3", "default", contract.JSONMap{}, nil),
	}
	items := inventory.BuildNodeItems([]inventory.ProjectedResource{nodeC, nodeB, nodeA}, pods)
	want := []v1node{
		{Name: "a-node", Role: "worker", Status: "Unknown", Version: "-", InternalIP: "-", OS: "-", CPU: "-", Memory: "-", Pods: "0/-"},
		{Name: "b-node", Role: "control-plane", Status: "Ready", Version: "v1.29.1", InternalIP: "10.0.0.1", OS: "Ubuntu 22.04", CPU: "4", Memory: "8590 MB", Pods: "2/110"},
		{Name: "c-node", Role: "worker", Status: "NotReady", Version: "-", InternalIP: "-", OS: "-", CPU: "-", Memory: "-", Pods: "0/-"},
	}
	if len(items) != len(want) {
		t.Fatalf("items = %d, want %d", len(items), len(want))
	}
	for i, w := range want {
		got := items[i]
		if got.Name != w.Name || got.Role != w.Role || got.Status != w.Status || got.Version != w.Version ||
			got.InternalIP != w.InternalIP || got.OS != w.OS || got.CPU != w.CPU || got.Memory != w.Memory || got.Pods != w.Pods {
			t.Errorf("items[%d] = %+v, want %+v", i, got, w)
		}
	}
}

// v1node는 K8sNodeItem의 표시 필드 미러다(테스트 가독성).
type v1node = struct {
	Name, Role, Status, Version, InternalIP, OS, CPU, Memory, Pods string
}

// TestAssembleResolveK8sNetworkCIDRs mirrors TestCharResolveK8sNetworkCIDRs —
// kubeadm-config 2키 화이트리스트만 소비, 이름 다른 configmap 무시, 노드 폴백은
// 정렬 후 "、" 결합, 부재는 Unknown(추측 금지).
func TestAssembleResolveK8sNetworkCIDRs(t *testing.T) {
	kubeadm := asmRow("orchestration.configmap", "", "kubeadm-config", "kube-system", contract.JSONMap{
		"serviceCIDR": "10.96.0.0/12", "podSubnetCIDR": "10.244.0.0/16",
	}, nil)
	node := asmRow("orchestration.node", "", "n1", "", contract.JSONMap{"podCIDRs": []string{"10.244.1.0/24"}}, nil)
	other := asmRow("orchestration.configmap", "", "other-cm", "kube-system", contract.JSONMap{
		"serviceCIDR": "10.0.0.0/8", // 이름이 다른 configmap의 값은 소비되지 않는다
	}, nil)

	cases := []struct {
		name           string
		nodes          []inventory.ProjectedResource
		cms            []inventory.ProjectedResource
		wantService    string
		wantPodNetwork string
	}{
		{name: "kubeadm-config 2키", nodes: []inventory.ProjectedResource{node}, cms: []inventory.ProjectedResource{kubeadm}, wantService: "10.96.0.0/12", wantPodNetwork: "10.244.0.0/16"},
		{name: "이름 다른 configmap 무시 → 노드 폴백", nodes: []inventory.ProjectedResource{node}, cms: []inventory.ProjectedResource{other}, wantService: "Unknown", wantPodNetwork: "10.244.1.0/24"},
		{name: "노드 간 중복 제거·정렬·、 결합", nodes: []inventory.ProjectedResource{
			asmRow("orchestration.node", "", "n1", "", contract.JSONMap{"podCIDRs": []string{"10.244.2.0/24", "10.244.1.0/24"}}, nil),
			asmRow("orchestration.node", "", "n2", "", contract.JSONMap{"podCIDRs": []string{"10.244.1.0/24"}}, nil),
		}, cms: nil, wantService: "Unknown", wantPodNetwork: "10.244.1.0/24、10.244.2.0/24"},
		{name: "둘 다 부재", wantService: "Unknown", wantPodNetwork: "Unknown"},
	}
	for _, tc := range cases {
		serviceCIDR, podCIDR := inventory.ResolveK8sNetworkCIDRs(tc.nodes, tc.cms)
		if serviceCIDR != tc.wantService || podCIDR != tc.wantPodNetwork {
			t.Errorf("%s: = (%q, %q), want (%q, %q)", tc.name, serviceCIDR, podCIDR, tc.wantService, tc.wantPodNetwork)
		}
	}
}

// TestAssemblePodSelectorFallback mirrors TestCharBuildPodItemsWithWorkloads의
// 셀렉터·최장 접두 케이스 — ownerReferences 부재 pod의 폴백 체인.
func TestAssemblePodSelectorFallback(t *testing.T) {
	// 셀렉터 매칭: pod 라벨이 워크로드 selector와 맞으면 배정.
	web1 := asmRow("orchestration.pod", "", "web-1", "default", contract.JSONMap{}, contract.JSONMap{"labels": map[string]string{"app": "web"}})
	webDep := asmRow("orchestration.workload", "deployment", "web", "default", contract.JSONMap{
		"selector": map[string]string{"app": "web"},
	}, nil)
	// 최장 접두: 근거 없는 pod "webapp-9"는 web보다 webapp에 배정된다.
	webapp9 := asmRow("orchestration.pod", "", "webapp-9", "default", contract.JSONMap{}, nil)
	webDepNoSelector := asmRow("orchestration.workload", "deployment", "web", "default", contract.JSONMap{}, nil)
	webappDep := asmRow("orchestration.workload", "deployment", "webapp", "default", contract.JSONMap{}, nil)

	cases := []struct {
		name     string
		rows     []inventory.ProjectedResource
		pod      string
		wantName string
		wantType string
	}{
		{name: "오너 없으면 셀렉터 매칭", rows: []inventory.ProjectedResource{web1, webDep}, pod: "web-1", wantName: "web", wantType: "Deployment"},
		{name: "근거 없으면 이름 접두사 최장 매칭", rows: []inventory.ProjectedResource{webapp9, webDepNoSelector, webappDep}, pod: "webapp-9", wantName: "webapp", wantType: "Deployment"},
	}
	for _, tc := range cases {
		items := inventory.BuildPodItemsWithWorkloads(tc.rows)
		for _, item := range items {
			if item.Name != tc.pod {
				continue
			}
			if item.WorkloadName != tc.wantName || item.WorkloadType != tc.wantType {
				t.Errorf("%s: = %q/%q, want %q/%q", tc.name, item.WorkloadName, item.WorkloadType, tc.wantName, tc.wantType)
			}
		}
	}
}

// TestAssembleWorkloadItemBatchTable — batch 유도 표(cronJobReadyText 4케이스·
// job 분모 폴백)와 requests/limits hasCPU·hasMemory 독립성.
func TestAssembleWorkloadItemBatchTable(t *testing.T) {
	cronRows := []inventory.ProjectedResource{
		asmRow("orchestration.workload", "cronjob", "a-suspended", "app", contract.JSONMap{"active": 2, "suspend": true}, nil),
		asmRow("orchestration.workload", "cronjob", "b-active", "app", contract.JSONMap{"active": 2}, nil),
		asmRow("orchestration.workload", "cronjob", "c-idle", "app", contract.JSONMap{}, nil),
		asmRow("orchestration.workload", "job", "d-fallback", "app", contract.JSONMap{"active": 1, "succeeded": 2, "failed": 1}, nil),
		asmRow("orchestration.workload", "deployment", "e-resources", "app", contract.JSONMap{
			"replicas": 1, "readyReplicas": 1, "updatedReplicas": 1, "availableReplicas": 1,
			"containers": []any{
				map[string]any{"requests": map[string]any{"cpuMilli": 2000}},
				map[string]any{"requests": map[string]any{"memBytes": 1610612736}},
			},
		}, nil),
	}
	items := inventory.BuildWorkloadItems(cronRows)
	byName := map[string]v1workload{}
	for _, item := range items {
		byName[item.Name] = v1workload{ready: item.Ready, updated: item.Updated, available: item.Available, requests: item.Requests}
	}
	want := map[string]v1workload{
		"a-suspended": {ready: "Suspended", updated: 2, available: 2, requests: "-"},         // suspend 우선(active 2여도)
		"b-active":    {ready: "2 Active", updated: 2, available: 2, requests: "-"},          // active>0
		"c-idle":      {ready: "Scheduled", updated: 0, available: 0, requests: "-"},         // suspend 부재=무중단(nil 동치)
		"d-fallback":  {ready: "2/4", updated: 1, available: 2, requests: "-"},               // completions 부재 → active+succeeded+failed
		"e-resources": {ready: "1/1", updated: 1, available: 1, requests: "2 cores / 1.5Gi"}, // hasCPU·hasMemory 독립 합산
	}
	for name, w := range want {
		got, ok := byName[name]
		if !ok {
			t.Fatalf("item %q 부재", name)
		}
		if got != w {
			t.Errorf("%s = %+v, want %+v", name, got, w)
		}
	}
}

// v1workload는 workloads 목록 표시의 미러다.
type v1workload = struct {
	ready     string
	updated   int
	available int
	requests  string
}

// TestAssembleGatewayAddressFallback mirrors resolveGatewayAPIAddress — 관측
// 주소 우선, 동일 네임스페이스 서비스 이름 매칭(name·name-istio·name- 접두),
// "<none>" 서비스는 건너뛰고 최종 "-".
func TestAssembleGatewayAddressFallback(t *testing.T) {
	gw := asmRow("network.gateway", "", "edge", "default", contract.JSONMap{}, nil)
	svcLb := asmRow("network.load_balancer", "service", "edge-istio", "default", contract.JSONMap{"externalIP": "192.0.2.1"}, nil)
	svcNone := asmRow("network.load_balancer", "service", "edge", "default", contract.JSONMap{}, nil) // externalIP 부재 → "<none>"
	svcOther := asmRow("network.load_balancer", "service", "edge-istio", "other", contract.JSONMap{"externalIP": "203.0.113.9"}, nil)

	if got := inventory.BuildAdvancedNetworkSection([]inventory.ProjectedResource{gw}, nil, []inventory.ProjectedResource{svcOther, svcNone, svcLb}); got.GatewayAPIGateways[0].Address != "192.0.2.1" {
		t.Errorf("address = %q, want 192.0.2.1 (동 ns 매칭만 — other ns 무시, <none> 건너뛰기)", got.GatewayAPIGateways[0].Address)
	}
	if got := inventory.BuildAdvancedNetworkSection([]inventory.ProjectedResource{gw}, nil, nil); got.GatewayAPIGateways[0].Address != "-" {
		t.Errorf("address = %q, want - (폴백 부재)", got.GatewayAPIGateways[0].Address)
	}
}

// TestAssembleJoinAndLimitOverflow — joinAndLimit의 가산 표기 계약.
func TestAssembleJoinAndLimitOverflow(t *testing.T) {
	if got := inventory.JoinAndLimit([]string{"a", "b", "c", "d"}, 3); got != "a, b, c +1" {
		t.Errorf("overflow = %q, want \"a, b, c +1\"", got)
	}
	if got := inventory.JoinAndLimit(nil, 3); got != "-" {
		t.Errorf("empty = %q, want -", got)
	}
}

// TestAssembleNilConn — 엔트리포인트의 방어 경로.
func TestAssembleNilConn(t *testing.T) {
	if _, err := inventory.AssembleK8sClusterDetail(nil, nil, nil); err == nil {
		t.Errorf("nil connection은 에러여야 한다")
	}
}
