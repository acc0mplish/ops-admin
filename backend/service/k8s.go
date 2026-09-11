package service

import (
	"errors"
	"fmt"
	"time"

	"ops-admin/backend/model"
)

func (s *Service) ListK8sClusters() ([]model.K8sClusterView, error) {
	// G1 단일 경로(phase6-plan §J8·C36 2단): cluster/list의 소스는 V2 인벤토리
	// 투영뿐이다. G0의 전환 플래그와 legacy k8s_cluster 조회 분기는 같은 PR에서
	// 제거됐다 — 응답 형상(K8sClusterView)은 소스와 무관하게 동일하다(§12 #1).
	return s.projectK8sClusterList()
}

func (s *Service) GetK8sCluster(id uint) (model.K8sCluster, error) {
	var cluster model.K8sCluster
	if err := s.db.Preload("Gateway").Preload("MonitorDatasource").First(&cluster, id).Error; err != nil {
		return cluster, err
	}
	return cluster, nil
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
