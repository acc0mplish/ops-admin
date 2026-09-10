package service

import (
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"ops-admin/backend/model"

	"gorm.io/gorm"
)

func (s *Service) ListK8sClusters() ([]model.K8sClusterView, error) {
	// V2_READ_SOURCE_K8S — G0 전환 검증 플래그(phase6-plan §J8·§12 #12). 정규화
	// 판정(k8sReadSourceV2 — ""·"0"·"false"는 OFF)으로 ON이면 읽기 소스를 V2
	// 인벤토리 투영으로 전환한다. 기본값은 legacy(§11 롤백 계약)이며
	// 플래그와 legacy 분기는 G1에서 함께 제거된다(C36 2단).
	if k8sReadSourceV2("V2_READ_SOURCE_K8S") {
		return s.projectK8sClusterList()
	}
	var list []model.K8sCluster
	if err := s.db.Preload("Gateway").Preload("MonitorDatasource").Order("id asc").Find(&list).Error; err != nil {
		return nil, err
	}

	result := make([]model.K8sClusterView, 0, len(list))
	for _, item := range list {
		result = append(result, toK8sClusterView(item))
	}
	return result, nil
}

func (s *Service) GetK8sCluster(id uint) (model.K8sCluster, error) {
	var cluster model.K8sCluster
	if err := s.db.Preload("Gateway").Preload("MonitorDatasource").First(&cluster, id).Error; err != nil {
		return cluster, err
	}
	return cluster, nil
}

func (s *Service) CreateK8sCluster(payload model.K8sClusterPayload) (model.K8sCluster, error) {
	monitorDatasourceID, err := s.resolveK8sMonitorDatasource(payload.MonitorDatasourceID)
	if err != nil {
		return model.K8sCluster{}, err
	}
	cluster := model.K8sCluster{
		Name:                strings.TrimSpace(payload.Name),
		Description:         strings.TrimSpace(payload.Description),
		KubeConfig:          strings.TrimSpace(payload.KubeConfig),
		Env:                 normalizeEnvCode(payload.Env),
		Tags:                normalizeAssetTags(payload.Tags),
		ConnectionMode:      normalizeConnectionMode(payload.ConnectionMode),
		GatewayID:           optionalGatewayID(payload.ConnectionMode, payload.GatewayID),
		MonitorDatasourceID: monitorDatasourceID,
	}
	if err := validateK8sClusterPayload(cluster); err != nil {
		return cluster, err
	}

	var count int64
	if err := s.db.Model(&model.K8sCluster{}).Where("name = ?", cluster.Name).Count(&count).Error; err != nil {
		return cluster, err
	}
	if count > 0 {
		return cluster, errors.New("Kubernetes cluster name already exists")
	}

	probe, err := s.probeK8sCluster(cluster)
	if err != nil {
		return cluster, err
	}

	now := time.Now()
	cluster.APIServer = probe.APIServer
	cluster.Version = probe.Version
	cluster.NodeCount = probe.NodeCount
	cluster.Status = probe.Status
	cluster.LastSyncAt = &now

	if err := s.db.Create(&cluster).Error; err != nil {
		return cluster, err
	}
	s.recordAssetChange("k8s", cluster.ID, cluster.Name, "create", "Create Kubernetes Cluster", payload.Operator)
	return cluster, nil
}

func (s *Service) UpdateK8sCluster(payload model.K8sClusterPayload) (model.K8sCluster, error) {
	cluster, err := s.GetK8sCluster(payload.ID)
	if err != nil {
		return cluster, err
	}

	cluster.Name = strings.TrimSpace(payload.Name)
	cluster.Description = strings.TrimSpace(payload.Description)
	cluster.KubeConfig = strings.TrimSpace(payload.KubeConfig)
	cluster.Env = normalizeEnvCode(payload.Env)
	cluster.Tags = normalizeAssetTags(payload.Tags)
	cluster.ConnectionMode = normalizeConnectionMode(payload.ConnectionMode)
	cluster.GatewayID = optionalGatewayID(payload.ConnectionMode, payload.GatewayID)
	monitorDatasourceID, err := s.resolveK8sMonitorDatasource(payload.MonitorDatasourceID)
	if err != nil {
		return cluster, err
	}
	cluster.MonitorDatasourceID = monitorDatasourceID

	if err := validateK8sClusterPayload(cluster); err != nil {
		return cluster, err
	}

	var count int64
	if err := s.db.Model(&model.K8sCluster{}).Where("name = ? AND id <> ?", cluster.Name, cluster.ID).Count(&count).Error; err != nil {
		return cluster, err
	}
	if count > 0 {
		return cluster, errors.New("Kubernetes cluster name already exists")
	}

	probe, err := s.probeK8sCluster(cluster)
	if err != nil {
		return cluster, err
	}

	now := time.Now()
	cluster.APIServer = probe.APIServer
	cluster.Version = probe.Version
	cluster.NodeCount = probe.NodeCount
	cluster.Status = probe.Status
	cluster.LastSyncAt = &now

	if err := s.db.Save(&cluster).Error; err != nil {
		return cluster, err
	}
	s.recordAssetChange("k8s", cluster.ID, cluster.Name, "update", "Update Kubernetes Cluster Configuration and Validate Connection", payload.Operator)
	return cluster, nil
}

func (s *Service) DeleteK8sCluster(id uint) error {
	cluster, _ := s.GetK8sCluster(id)
	if err := s.db.Delete(&model.K8sCluster{}, id).Error; err != nil {
		return err
	}
	s.recordAssetChange("k8s", id, cluster.Name, "delete", "Delete Kubernetes Cluster", "system")
	return nil
}

func (s *Service) GetK8sClusterDetail(clusterID uint) (model.K8sClusterDetail, error) {
	// V2_READ_SOURCE_K8S — G0 읽기 소스 전환(phase6-plan §J8). 정규화 판정은
	// k8sReadSourceV2 단일 헬퍼. 캐시·singleflight 껍질과 키(legacy 클러스터 id)는
	// 무변경이고 singleflight 본문 소스만 교체한다. 기본값은 legacy(§11 롤백 계약)이며
	// G1에서 legacy 분기가 제거된다(C36 2단).
	source := s.getK8sClusterDetailUncached
	if k8sReadSourceV2("V2_READ_SOURCE_K8S") {
		source = s.projectK8sClusterDetail
	}
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

func (s *Service) getK8sClusterDetailUncached(clusterID uint) (model.K8sClusterDetail, error) {
	cluster, err := s.GetK8sCluster(clusterID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.K8sClusterDetail{}, errors.New("k8s cluster not found")
		}
		return model.K8sClusterDetail{}, err
	}

	runtime, err := parseKubeConfig(cluster.KubeConfig)
	if err != nil {
		return model.K8sClusterDetail{}, fmt.Errorf("failed to parse kubeconfig: %w", err)
	}
	// Cluster details must use a client that matches the configured connection mode.
	// In gateway mode, the client connects to the gateway before accessing the private Kubernetes API;
	// the Ops Admin host must not dial the private kubeconfig address directly.
	client, cleanup, err := s.newK8sHTTPClientForCluster(cluster, runtime)
	if err != nil {
		return model.K8sClusterDetail{}, fmt.Errorf("failed to create Kubernetes API client: %w", err)
	}
	defer cleanup()

	data, err := fetchK8sData(client, runtime)
	if err != nil {
		route := "direct connection"
		if normalizeConnectionMode(cluster.ConnectionMode) == "gateway" {
			route = "gateway relay"
		}
		return model.K8sClusterDetail{}, fmt.Errorf("failed to retrieve Kubernetes cluster details through %s: %w", route, err)
	}

	metrics := calculateK8sAggregateMetrics(data.Nodes, data.Pods)
	detailCluster := toK8sClusterView(cluster)
	if metrics.AlertCount > 0 {
		detailCluster.Status = "warning"
		detailCluster.StatusText = k8sStatusText("warning")
	}

	namespaceCounts := buildNamespaceCounts(data)
	endpointCounts := buildEndpointCounts(data.Endpoints)
	workloads := buildWorkloadItems(data)
	sort.Slice(workloads, func(i, j int) bool {
		if workloads[i].Namespace == workloads[j].Namespace {
			return workloads[i].Name < workloads[j].Name
		}
		return workloads[i].Namespace < workloads[j].Namespace
	})

	return model.K8sClusterDetail{
		Cluster: detailCluster,
		Overview: model.K8sOverview{
			HealthScore:  calculateHealthScore(metrics.AlertCount),
			CPUUsage:     formatUsagePercent(metrics.TotalReqCPUMilli, metrics.TotalAllocCPUMilli),
			MemoryUsage:  formatUsagePercent(metrics.TotalReqMemoryBytes, metrics.TotalAllocMemoryBytes),
			PodUsage:     fmt.Sprintf("%d Pods", len(data.Pods)),
			RequestRate:  fmt.Sprintf("%d Workloads", len(workloads)),
			AlertCount:   metrics.AlertCount,
			Distribution: buildOverviewDistribution(detailCluster, data.Nodes, data.ConfigMaps),
			Certificates: buildOverviewCertificates(runtime),
		},
		Nodes:      buildNodeItems(data.Nodes, data.Pods),
		Namespaces: buildNamespaceItems(data.Namespaces, namespaceCounts),
		Pods:       buildPodItemsWithWorkloads(data),
		Workloads:  workloads,
		Network:    buildNetworkSection(data.Services, data.Ingresses, endpointCounts),
		AdvancedNetwork: buildAdvancedNetworkSection(
			data.GatewayAPIGateways,
			data.HTTPRoutes,
			data.Services,
		),
		ConfigStorage: buildConfigStorageSection(data.ConfigMaps, data.Secrets, data.PVCs, data.PVs),
	}, nil
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

// UpdateK8sNodeLabels applies the submitted label set to a Kubernetes node.
// Existing labels not included in the set are explicitly removed via a merge patch.
func (s *Service) UpdateK8sNodeLabels(payload model.K8sNodeLabelsPayload) error {
	_, runtime, client, err := s.k8sClientForCluster(payload.ClusterID)
	if err != nil {
		return err
	}
	var node kubeNode
	if err := k8sGetJSON(client, runtime, "/api/v1/nodes/"+url.PathEscape(payload.NodeName), &node); err != nil {
		return errors.New(k8sClusterConnectError)
	}
	labelsPatch := make(map[string]any, len(node.Metadata.Labels)+len(payload.Labels))
	for key := range node.Metadata.Labels {
		if _, keep := payload.Labels[key]; !keep {
			labelsPatch[key] = nil
		}
	}
	for key, value := range payload.Labels {
		key = strings.TrimSpace(key)
		if key == "" {
			return errors.New("node label key is required")
		}
		labelsPatch[key] = strings.TrimSpace(value)
	}
	if err := k8sPatchJSON(client, runtime, "/api/v1/nodes/"+url.PathEscape(payload.NodeName), map[string]any{
		"metadata": map[string]any{"labels": labelsPatch},
	}, "application/merge-patch+json", nil); err != nil {
		return errors.New(k8sClusterConnectError)
	}
	return nil
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
