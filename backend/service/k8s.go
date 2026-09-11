package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"ops-admin/backend/internal/infra/contract"
	infraModel "ops-admin/backend/internal/infra/model"
	"ops-admin/backend/internal/infra/secrets"
	"ops-admin/backend/model"
)

func (s *Service) ListK8sClusters() ([]model.K8sClusterView, error) {
	// G1 단일 경로(phase6-plan §J8·C36 2단): cluster/list의 소스는 V2 인벤토리
	// 투영뿐이다. G0의 전환 플래그와 legacy k8s_cluster 조회 분기는 같은 PR에서
	// 제거됐다 — 응답 형상(K8sClusterView)은 소스와 무관하게 동일하다(§12 #1).
	return s.projectK8sClusterList()
}

// GetK8sCluster resolves one cluster by its legacy id through the V2
// provider_connection chain (V2 Phase 6 I-a S5 — phase6-plan §J5). The
// k8s_cluster table lookup is retired: the legacy id stays the read handle
// for the S5 consumers (terminal·metrics·fetch·diagnosis·ops — 시그니처
// 무변경 계약 §12 #16), the source is the live (stale_source=false — §5.4c)
// kubernetes provider_connection carrying that source_id, and the kubeconfig
// is decrypted from the bound SecretRef through the §7.4 broker only (봉인
// 계약 — plaintext never leaves this struct in memory; no log or error
// carries it). Field procurement follows the F6 계약: env·connection_mode·
// gateway_id·monitor_datasource_id ride the ConfigJSON fixed keys, with the
// gateway_id column (backfill chains) taking precedence over the ConfigJSON
// string the register-k8s CLI records.
//
// Chain coverage judgment (구현 판정 기록 2026-09-11): the resolution follows
// the plan's sourceModel-무관 recommendation (§J5 — provider_type
// "kubernetes" + source_id) so any chain shape that carries the legacy id in
// the source_id space resolves. register-k8s chains carry no source pair, so
// they stay outside this legacy-id read surface until the id space
// transitions (the k8s_projection.go H0→I-a window note extends to I-b).
func (s *Service) GetK8sCluster(id uint) (model.K8sCluster, error) {
	// id=0 가드(리뷰 HIGH): register-k8s 체인은 source_id=0(미설정)이라
	// 무가드 조회가 모든 미설정 체인을 0번 클러스터로 승격시킨다 — 0은
	// legacy 공간에서 "클러스터 없음"과 같으므로 미해상(ErrRecordNotFound)로
	// 차단한다.
	if id == 0 {
		return model.K8sCluster{}, gorm.ErrRecordNotFound
	}
	var conn infraModel.ProviderConnection
	err := s.db.Where("provider_type = ? AND source_id = ? AND stale_source = ?",
		k8sProjectionProvider, id, false).First(&conn).Error
	if err != nil {
		return model.K8sCluster{}, err
	}
	resolved, err := secrets.NewBroker(s.db).Resolve(context.Background(), conn.UID, contract.CredentialPurposeInventory)
	if err != nil {
		return model.K8sCluster{}, err
	}
	cluster := model.K8sCluster{
		ID:                  id,
		Name:                conn.Name,
		Status:              conn.Status,
		APIServer:           conn.Endpoint,
		Version:             conn.Version,
		Env:                 k8sConfigString(&conn, "env"),
		Tags:                k8sConfigTags(&conn),
		ConnectionMode:      k8sConfigString(&conn, "connection_mode"),
		GatewayID:           k8sGatewayID(&conn),
		MonitorDatasourceID: k8sConfigUintRef(&conn, "monitor_datasource_id"),
		KubeConfig:          resolved.Value,
	}
	// gateway·monitor_datasource는 legacy Preload에 대응해 id로 조회하며,
	// 행 부재(ErrRecordNotFound)만 영값으로 용인하고 그 외 DB 에러는 전파한다
	// (k8sClusterFromProjection 선례 정합).
	if cluster.GatewayID != nil && *cluster.GatewayID > 0 {
		if err := s.db.First(&cluster.Gateway, *cluster.GatewayID).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return model.K8sCluster{}, fmt.Errorf("k8s source: gateway load: %w", err)
		}
	}
	if cluster.MonitorDatasourceID != nil && *cluster.MonitorDatasourceID > 0 {
		if err := s.db.First(&cluster.MonitorDatasource, *cluster.MonitorDatasourceID).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return model.K8sCluster{}, fmt.Errorf("k8s source: monitor datasource load: %w", err)
		}
	}
	return cluster, nil
}

// --- S6 등가 조회 (phase6-plan §J5 S6·F9) — k8s_cluster gorm 직접 조회는
// --- provider_connection 소스로 치환한다. 두 체인 형상(백필 칼럼·ConfigJSON
// --- 문자열)을 하나의 스캔으로 모두 커버하고, stale 행은 부재로 읽는다.

// k8sClusterConnections lists every live kubernetes provider_connection —
// the single scan all S6 equivalent lookups are built on.
func (s *Service) k8sClusterConnections() ([]infraModel.ProviderConnection, error) {
	var conns []infraModel.ProviderConnection
	err := s.db.Where("provider_type = ? AND stale_source = ?", k8sProjectionProvider, false).
		Order("id").Find(&conns).Error
	if err != nil {
		return nil, fmt.Errorf("k8s source: list connections: %w", err)
	}
	return conns, nil
}

// countK8sClustersByGateway counts live kubernetes connections referencing a
// gateway — the gateway_id column (backfill chains) or the ConfigJSON string
// (register-k8s chains) count as the same reference for delete protection.
func (s *Service) countK8sClustersByGateway(gatewayID uint) (int64, error) {
	conns, err := s.k8sClusterConnections()
	if err != nil {
		return 0, err
	}
	var count int64
	for i := range conns {
		if gatewayID == k8sGatewayIDValue(&conns[i]) {
			count++
		}
	}
	return count, nil
}

// countK8sClustersByMonitorDatasource counts live kubernetes connections
// whose ConfigJSON monitor_datasource_id matches (the register-k8s CLI
// convention — the backfill chains never carried a monitor binding into V2).
func (s *Service) countK8sClustersByMonitorDatasource(datasourceID uint) (int64, error) {
	conns, err := s.k8sClusterConnections()
	if err != nil {
		return 0, err
	}
	var count int64
	for i := range conns {
		if id := k8sConfigUintRef(&conns[i], "monitor_datasource_id"); id != nil && *id == datasourceID {
			count++
		}
	}
	return count, nil
}

// countK8sClustersByEnv counts live kubernetes connections whose ConfigJSON
// env matches the environment code.
func (s *Service) countK8sClustersByEnv(env string) (int64, error) {
	conns, err := s.k8sClusterConnections()
	if err != nil {
		return 0, err
	}
	var count int64
	for i := range conns {
		if k8sConfigString(&conns[i], "env") == env {
			count++
		}
	}
	return count, nil
}

// --- F6 필드 조달 헬퍼 — ConfigJSON 고정키 계약 (phase6-plan §J5 F6).

// k8sConfigString reads a ConfigJSON fixed key as a trimmed string.
func k8sConfigString(conn *infraModel.ProviderConnection, key string) string {
	value, _ := conn.ConfigJSON[key].(string)
	return strings.TrimSpace(value)
}

// k8sGatewayID — the gateway_id column (backfill chains) wins; the
// ConfigJSON string the register-k8s CLI records (H0 규약 — 문자열 id) is
// parsed into *uint as the fallback.
func k8sGatewayID(conn *infraModel.ProviderConnection) *uint {
	if conn.GatewayID != nil && *conn.GatewayID > 0 {
		out := *conn.GatewayID
		return &out
	}
	return k8sConfigUintRef(conn, "gateway_id")
}

func k8sGatewayIDValue(conn *infraModel.ProviderConnection) uint {
	if id := k8sGatewayID(conn); id != nil {
		return *id
	}
	return 0
}

// k8sConfigUintRef parses a ConfigJSON fixed key holding a decimal id string
// into an optional uint. Non-numeric or zero values read as absent.
func k8sConfigUintRef(conn *infraModel.ProviderConnection, key string) *uint {
	raw, _ := conn.ConfigJSON[key].(string)
	id, err := strconv.ParseUint(strings.TrimSpace(raw), 10, 64)
	if err != nil || id == 0 {
		return nil
	}
	out := uint(id)
	return &out
}

// k8sConfigUintValue reads a numeric ConfigJSON entry (node_count rides as a
// JSON number from the backfill projection; the string form is tolerated).
func k8sConfigUintValue(conn *infraModel.ProviderConnection, key string) (uint64, bool) {
	switch value := conn.ConfigJSON[key].(type) {
	case float64:
		if value > 0 {
			return uint64(value), true
		}
	case string:
		parsed, err := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
		if err == nil && parsed > 0 {
			return parsed, true
		}
	}
	return 0, false
}

// k8sConfigTags reads the backfill projection's JSON tag array.
func k8sConfigTags(conn *infraModel.ProviderConnection) []string {
	raw, _ := conn.ConfigJSON["tags"].([]any)
	if len(raw) == 0 {
		return nil
	}
	tags := make([]string, 0, len(raw))
	for _, item := range raw {
		if tag, ok := item.(string); ok {
			tags = append(tags, tag)
		}
	}
	return tags
}

func (s *Service) GetK8sClusterDetail(clusterID uint) (model.K8sClusterDetail, error) {
	// G1 단일 경로(phase6-plan §J8·C36 2단): singleflight 본문 소스는 V2 조립
	// (projectK8sClusterDetail)뿐이다. 캐시·singleflight 껍질과 키(legacy 클러스터
	// id)는 무변경(§J8 캐시 승계)이고 legacy 라이브 소스(kubeconfig 파싱 → k8s API
	// fetch 경로)는 제거됐다 — §15 compare 페어링(S2)도 같은 PR에서 종결됐다(D-16).
	source := s.projectK8sClusterDetail
	if detail, ok := s.cachedK8sClusterDetail(clusterID); ok {
		return detail, nil
	}
	result, err, _ := s.k8sState.k8sOverviewGroup.Do(fmt.Sprintf("cluster-overview:%d", clusterID), func() (any, error) {
		if detail, ok := s.cachedK8sClusterDetail(clusterID); ok {
			return detail, nil
		}
		detail, err := source(clusterID)
		if err != nil {
			return model.K8sClusterDetail{}, err
		}
		s.k8sState.k8sOverviewMu.Lock()
		s.k8sState.k8sOverviewCache[clusterID] = k8sOverviewCacheEntry{detail: detail, expiresAt: time.Now().Add(k8sOverviewCacheTTL)}
		s.k8sState.k8sOverviewMu.Unlock()
		return detail, nil
	})
	if err != nil {
		return model.K8sClusterDetail{}, err
	}
	return result.(model.K8sClusterDetail), nil
}

func (s *Service) cachedK8sClusterDetail(clusterID uint) (model.K8sClusterDetail, bool) {
	s.k8sState.k8sOverviewMu.Lock()
	defer s.k8sState.k8sOverviewMu.Unlock()
	entry, ok := s.k8sState.k8sOverviewCache[clusterID]
	if !ok || time.Now().After(entry.expiresAt) {
		if ok {
			delete(s.k8sState.k8sOverviewCache, clusterID)
		}
		return model.K8sClusterDetail{}, false
	}
	return entry.detail, true
}

func (s *Service) invalidateK8sClusterDetailCache(clusterID uint) {
	s.k8sState.k8sOverviewMu.Lock()
	delete(s.k8sState.k8sOverviewCache, clusterID)
	s.k8sState.k8sOverviewMu.Unlock()
}

func (s *Service) GetK8sNodeDetail(clusterID uint, nodeName string) (model.K8sNodeDetail, error) {
	cluster, runtime, client, err := s.k8sClientForCluster(clusterID)
	if err != nil {
		return model.K8sNodeDetail{}, err
	}

	var node kubeNode
	if err := k8sGetJSON(client, runtime, "/api/v1/nodes/"+nodeName, &node); err != nil {
		return model.K8sNodeDetail{}, errors.New(k8sClusterConnectError)
	}

	pods, err := fetchPodsForNode(client, runtime, nodeName)
	if err != nil {
		return model.K8sNodeDetail{}, errors.New(k8sClusterConnectError)
	}

	_ = cluster
	return model.K8sNodeDetail{
		Name:           node.Metadata.Name,
		Status:         nodeReadyStatus(node),
		Roles:          joinNodeRoles(node.Metadata.Labels),
		Version:        fallbackText(node.Status.NodeInfo.KubeletVersion),
		InternalIP:     firstNodeInternalIP(node),
		OS:             fallbackText(node.Status.NodeInfo.OSImage),
		Kernel:         fallbackText(node.Status.NodeInfo.KernelVersion),
		ContainerRT:    fallbackText(node.Status.NodeInfo.ContainerRuntimeVersion),
		Architecture:   fallbackText(node.Status.NodeInfo.Architecture),
		Labels:         node.Metadata.Labels,
		CapacityCPU:    fallbackText(node.Status.Capacity["cpu"]),
		CapacityMem:    fallbackText(node.Status.Capacity["memory"]),
		AllocatableCPU: fallbackText(node.Status.Allocatable["cpu"]),
		AllocatableMem: fallbackText(node.Status.Allocatable["memory"]),
		Pods:           buildPodItems(pods),
	}, nil
}

func (s *Service) GetK8sNodePods(clusterID uint, nodeName string) ([]model.K8sPodItem, error) {
	_, runtime, client, err := s.k8sClientForCluster(clusterID)
	if err != nil {
		return nil, err
	}
	pods, err := fetchPodsForNode(client, runtime, nodeName)
	if err != nil {
		return nil, errors.New(k8sClusterConnectError)
	}
	return buildPodItems(pods), nil
}

func (s *Service) GetK8sPodDetail(clusterID uint, namespace string, podName string) (model.K8sPodDetail, error) {
	_, runtime, client, err := s.k8sClientForCluster(clusterID)
	if err != nil {
		return model.K8sPodDetail{}, err
	}

	var pod kubePod
	if err := k8sGetJSON(client, runtime, "/api/v1/namespaces/"+namespace+"/pods/"+podName, &pod); err != nil {
		return model.K8sPodDetail{}, errors.New(k8sClusterConnectError)
	}

	containers := make([]model.K8sContainerItem, 0, len(pod.Spec.Containers))
	statusMap := make(map[string]struct {
		ready   bool
		restart int
	}, len(pod.Status.ContainerStatuses))
	for _, status := range pod.Status.ContainerStatuses {
		statusMap[status.Name] = struct {
			ready   bool
			restart int
		}{ready: status.Ready, restart: status.RestartCount}
	}
	for _, container := range pod.Spec.Containers {
		containerStatus := statusMap[container.Name]
		containers = append(containers, model.K8sContainerItem{
			Name:            container.Name,
			Image:           container.Image,
			Ready:           containerStatus.ready,
			Restart:         containerStatus.restart,
			RequestCPU:      container.Resources.Requests["cpu"],
			LimitCPU:        container.Resources.Limits["cpu"],
			RequestMemory:   container.Resources.Requests["memory"],
			LimitMemory:     container.Resources.Limits["memory"],
			ImagePullPolicy: container.ImagePullPolicy,
			Env:             buildContainerEnvItems(container.Env),
		})
	}

	return model.K8sPodDetail{
		Name:           pod.Metadata.Name,
		Namespace:      pod.Metadata.Namespace,
		Status:         fallbackText(pod.Status.Phase),
		Node:           fallbackText(pod.Spec.NodeName),
		PodIP:          fallbackText(pod.Status.PodIP),
		HostIP:         fallbackText(pod.Status.HostIP),
		QoSClass:       fallbackText(pod.Status.QoSClass),
		ServiceAccount: fallbackText(pod.Spec.ServiceAccountName),
		Labels:         pod.Metadata.Labels,
		Containers:     containers,
		CreatedAt:      formatTimestamp(pod.Metadata.CreationTimestamp),
		YAML:           marshalK8sYAML(pod),
	}, nil
}
