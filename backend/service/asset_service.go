package service

import (
	"fmt"
	"net/url"
	"ops-admin/backend/apperr"
	"sort"
	"strconv"
	"strings"

	"gorm.io/gorm"
	infraModel "ops-admin/backend/internal/infra/model"
	"ops-admin/backend/model"
)

type AssetServicePayload struct {
	ID           uint                          `json:"id"`
	Name         string                        `json:"name"`
	K8sClusterID uint                          `json:"k8sClusterId"`
	Namespace    string                        `json:"namespace"`
	ServiceType  string                        `json:"serviceType"`
	Status       int                           `json:"status"`
	Description  string                        `json:"description"`
	Workloads    []AssetServiceWorkloadPayload `json:"workloads"`
}

type AssetServiceWorkloadPayload struct {
	WorkloadType string `json:"workloadType"`
	WorkloadName string `json:"workloadName"`
}

type AssetServiceWorkloadRollbackPayload struct {
	ServiceID    uint   `json:"serviceId"`
	WorkloadType string `json:"workloadType"`
	WorkloadName string `json:"workloadName"`
	Revision     string `json:"revision"`
}

func (s *Service) ListAssetServices(pageNum, pageSize int, keyword string) (map[string]any, error) {
	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	query := s.db.Model(&model.AssetService{})
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("name LIKE ? OR service_uid LIKE ? OR namespace LIKE ?", like, like, like)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var list []model.AssetService
	if err := query.Order("id DESC").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, err
	}
	if err := s.fillAssetServiceClusterIDs(list); err != nil {
		return nil, err
	}
	return map[string]any{"list": list, "total": total, "pageNum": pageNum, "pageSize": pageSize}, nil
}

// resolveAssetServiceClusterID — 공유 커넥션 스캔 위에서 단일 행의 역해상.
// 복수매치는 최초 매치를 반환한다 — 이 필드는 목록 표시·프론트 릴레이 전용이고,
// 단건 k8s 읽기 경로는 assetServiceClusterID가 모호 에러로 차단한다.
func resolveAssetServiceClusterID(conns []infraModel.ProviderConnection, service *model.AssetService) uint {
	for i := range conns {
		if assetServiceUID(conns[i].Endpoint, service.Namespace, service.Name) == service.ServiceUID {
			return k8sProjectionViewID(&conns[i])
		}
	}
	return 0
}

// fillAssetServiceClusterIDs — 행 슬라이스의 역해상 응답 필드
// (model.AssetService.K8sClusterID gorm:"-")를 채운다. 커넥션 집합은 1회
// 스캔해 공유한다.
func (s *Service) fillAssetServiceClusterIDs(rows []model.AssetService) error {
	if len(rows) == 0 {
		return nil
	}
	conns, err := s.k8sClusterConnections()
	if err != nil {
		return err
	}
	for i := range rows {
		rows[i].K8sClusterID = resolveAssetServiceClusterID(conns, &rows[i])
	}
	return nil
}

func (s *Service) GetAssetService(id uint) (*model.AssetService, error) {
	var item model.AssetService
	if err := s.db.Preload("Workloads").First(&item, id).Error; err != nil {
		return nil, err
	}
	conns, err := s.k8sClusterConnections()
	if err != nil {
		return nil, err
	}
	item.K8sClusterID = resolveAssetServiceClusterID(conns, &item)
	return &item, nil
}

// assetServiceClusterID — I-b(step0007) 칼럼 drop 이후의 행→클러스터 역해상
// (S3 소비자 재판정 — 구현 판정 기록 2026-09-11). 링크 칼럼이 사라져도
// ServiceUID가 링크를 결정론적으로 인코딩한다: SaveAssetService는
// assetServiceUID(cluster.APIServer, namespace, name)로 파생하고 — I-a S5
// 이후 APIServer == provider_connection.Endpoint — 읽기는 같은 함수를 live
// kubernetes 커넥션 집합(k8sClusterConnections)에 재적용해 역해상한다.
// 무매치는 not-found, 복수매치는 모호 에러로 차단한다 — 칼럼 시절의 1:1 링크
// 보장을 live 체인 간 API 서버 hostname 충돌(동일 hostname, port만 다른
// 멀티클러스터)에서도 조용히 깨지 않기 위해서다. 노출 id는 k8s_projection.go의
// id 공간 판단(source 쌍 체인은 source_id, register-k8s 체인은 conn.ID)을
// 따른다.
func (s *Service) assetServiceClusterID(service *model.AssetService) (uint, error) {
	conns, err := s.k8sClusterConnections()
	if err != nil {
		return 0, err
	}
	matches := make([]uint, 0, 1)
	for i := range conns {
		if assetServiceUID(conns[i].Endpoint, service.Namespace, service.Name) == service.ServiceUID {
			matches = append(matches, k8sProjectionViewID(&conns[i]))
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return 0, apperr.New("K8S_CLUSTER_NOT_FOUND", nil)
	default:
		return 0, apperr.New("K8S_CLUSTER_AMBIGUOUS", nil)
	}
}

func (s *Service) SaveAssetService(payload AssetServicePayload) error {
	name, namespace := Trimmed(payload.Name), Trimmed(payload.Namespace)
	if name == "" || payload.K8sClusterID == 0 || namespace == "" {
		return apperr.New("ASSET_SERVICE_FIELDS_REQUIRED", nil)
	}
	if len(payload.Workloads) == 0 {
		return apperr.New("ASSET_SERVICE_WORKLOAD_REQUIRED", nil)
	}
	cluster, err := s.GetK8sCluster(payload.K8sClusterID)
	if err != nil {
		return apperr.New("K8S_CLUSTER_NOT_FOUND", nil)
	}
	// 클러스터 링크 칼럼은 I-b에서 제거됐다(C66) — payload.K8sClusterID는
	// 존재 검증과 ServiceUID 파생에만 쓰이고 행에는 저장되지 않는다. 링크는
	// ServiceUID에 인코딩되며 읽기는 assetServiceClusterID로 역해상한다.
	item := model.AssetService{Name: name, ServiceUID: assetServiceUID(cluster.APIServer, namespace, name), Namespace: namespace, ServiceType: Trimmed(payload.ServiceType), Status: payload.Status, Description: Trimmed(payload.Description)}
	if item.ServiceType == "" {
		item.ServiceType = "Business Service"
	}
	if item.Status == 0 {
		item.Status = 1
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		serviceID := payload.ID
		if serviceID == 0 {
			if err := tx.Create(&item).Error; err != nil {
				return err
			}
			serviceID = item.ID
		} else {
			result := tx.Model(&model.AssetService{}).Where("id = ?", serviceID).Updates(map[string]any{"name": item.Name, "service_uid": item.ServiceUID, "namespace": item.Namespace, "service_type": item.ServiceType, "status": item.Status, "description": item.Description})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return apperr.New("ASSET_SERVICE_NOT_FOUND", nil)
			}
		}
		if err := tx.Where("service_id = ?", serviceID).Delete(&model.AssetServiceWorkload{}).Error; err != nil {
			return err
		}
		seen := map[string]struct{}{}
		for _, workload := range payload.Workloads {
			typeName, workloadName := strings.ToLower(Trimmed(workload.WorkloadType)), Trimmed(workload.WorkloadName)
			if workloadName == "" {
				continue
			}
			if typeName == "" {
				typeName = "deployment"
			}
			key := typeName + ":" + workloadName
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			if err := tx.Create(&model.AssetServiceWorkload{ServiceID: serviceID, WorkloadType: typeName, WorkloadName: workloadName}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Service) DeleteAssetService(id uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("service_id = ?", id).Delete(&model.AssetServiceWorkload{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.AssetService{}, id).Error
	})
}

func (s *Service) GetAssetServiceK8sCatalog(clusterID uint, namespace string) (map[string]any, error) {
	if clusterID == 0 {
		return nil, apperr.New("K8S_CLUSTER_REQUIRED", nil)
	}
	cluster, runtime, client, err := s.k8sClientForCluster(clusterID)
	if err != nil {
		return nil, err
	}
	// The service form only needs namespaces and workload controllers. Avoid the
	// expensive full overview (pods, secrets, storage, services, gateway APIs).
	var namespaceResp kubeNamespaceListResponse
	if err := k8sGetJSON(client, runtime, "/api/v1/namespaces", &namespaceResp); err != nil {
		return nil, apperr.New("K8S_CLUSTER_CONNECTION_FAILED", nil)
	}
	data := k8sFetchedData{Namespaces: namespaceResp.Items}
	var deploymentResp kubeDeploymentListResponse
	if err := k8sGetJSON(client, runtime, "/apis/apps/v1/deployments", &deploymentResp); err == nil {
		data.Deployments = deploymentResp.Items
	}
	var statefulSetResp kubeStatefulSetListResponse
	if err := k8sGetJSON(client, runtime, "/apis/apps/v1/statefulsets", &statefulSetResp); err == nil {
		data.StatefulSet = statefulSetResp.Items
	}
	var daemonSetResp kubeDaemonSetListResponse
	if err := k8sGetJSON(client, runtime, "/apis/apps/v1/daemonsets", &daemonSetResp); err == nil {
		data.DaemonSets = daemonSetResp.Items
	}
	var jobResp kubeJobListResponse
	if err := k8sGetJSON(client, runtime, "/apis/batch/v1/jobs", &jobResp); err == nil {
		data.Jobs = jobResp.Items
	}
	var cronJobResp kubeCronJobListResponse
	if err := k8sGetJSON(client, runtime, "/apis/batch/v1/cronjobs", &cronJobResp); err == nil {
		data.CronJobs = cronJobResp.Items
	}
	namespace = Trimmed(namespace)
	workloads := make([]model.K8sWorkloadItem, 0)
	for _, workload := range buildWorkloadItems(data) {
		if namespace == "" || workload.Namespace == namespace {
			workloads = append(workloads, workload)
		}
	}
	return map[string]any{"cluster": toK8sClusterView(cluster), "namespaces": buildNamespaceItems(data.Namespaces, nil), "workloads": workloads}, nil
}

func (s *Service) GetAssetServiceRuntimeTopology(serviceID uint) (map[string]any, error) {
	service, err := s.GetAssetService(serviceID)
	if err != nil {
		return nil, err
	}
	clusterID, err := s.assetServiceClusterID(service)
	if err != nil {
		return nil, err
	}
	cluster, err := s.GetK8sCluster(clusterID)
	if err != nil {
		return nil, err
	}
	result := map[string]any{"service": service, "cluster": toK8sClusterView(cluster), "namespace": service.Namespace, "source": "saved", "workloads": service.Workloads}
	workloads := make([]model.K8sWorkloadItem, 0, len(service.Workloads))
	for _, item := range service.Workloads {
		detail, detailErr := s.GetK8sWorkloadDetail(clusterID, service.Namespace, item.WorkloadType, item.WorkloadName)
		if detailErr != nil {
			workloads = append(workloads, model.K8sWorkloadItem{Name: item.WorkloadName, Type: item.WorkloadType, Namespace: service.Namespace, Ready: "0/0"})
			continue
		}
		workloads = append(workloads, model.K8sWorkloadItem{Name: detail.Name, Type: detail.Type, Namespace: detail.Namespace, Ready: detail.Ready, Updated: detail.Updated, Available: detail.Available, Age: detail.Age})
	}
	sort.Slice(workloads, func(i, j int) bool { return workloads[i].Name < workloads[j].Name })
	result["workloads"], result["source"] = workloads, "live"
	return result, nil
}

func (s *Service) GetAssetServiceWorkloadRuntime(serviceID uint, workloadType, workloadName string) (model.K8sWorkloadDetail, error) {
	service, err := s.GetAssetService(serviceID)
	if err != nil {
		return model.K8sWorkloadDetail{}, err
	}
	if !assetServiceContainsWorkload(service, workloadType, workloadName) {
		return model.K8sWorkloadDetail{}, apperr.New("ASSET_SERVICE_WORKLOAD_MISMATCH", nil)
	}
	clusterID, err := s.assetServiceClusterID(service)
	if err != nil {
		return model.K8sWorkloadDetail{}, err
	}
	return s.GetK8sWorkloadDetail(clusterID, service.Namespace, workloadType, workloadName)
}

func (s *Service) GetAssetServiceWorkloadTopology(serviceID uint, workloadType, workloadName string) (map[string]any, error) {
	service, err := s.GetAssetService(serviceID)
	if err != nil {
		return nil, err
	}
	if !assetServiceContainsWorkload(service, workloadType, workloadName) {
		return nil, apperr.New("ASSET_SERVICE_WORKLOAD_MISMATCH", nil)
	}
	clusterID, err := s.assetServiceClusterID(service)
	if err != nil {
		return nil, err
	}
	detail, err := s.GetK8sWorkloadDetail(clusterID, service.Namespace, workloadType, workloadName)
	if err != nil {
		return nil, err
	}
	result := map[string]any{"workload": detail, "services": []map[string]any{}, "replicaSets": []map[string]any{}, "statefulSet": nil}
	_, runtime, client, err := s.k8sClientForCluster(clusterID)
	if err != nil {
		return result, nil
	}
	var serviceList kubeServiceListResponse
	if err := k8sGetJSON(client, runtime, "/api/v1/namespaces/"+service.Namespace+"/services", &serviceList); err == nil {
		items := make([]map[string]any, 0)
		for _, item := range serviceList.Items {
			if labelsMatch(item.Spec.Selector, detail.Selector) || labelsMatch(item.Spec.Selector, detail.Labels) {
				items = append(items, map[string]any{"name": item.Metadata.Name, "type": item.Spec.Type, "clusterIP": item.Spec.ClusterIP, "age": humanizeAge(item.Metadata.CreationTimestamp), "healthy": true})
			}
		}
		result["services"] = items
	}
	if strings.EqualFold(workloadType, "statefulset") {
		result["statefulSet"] = map[string]any{"name": detail.Name, "ready": detail.Ready, "available": detail.Available, "age": detail.Age, "healthy": workloadDetailHealthy(detail), "pods": detail.Pods}
		return result, nil
	}
	if !strings.EqualFold(workloadType, "deployment") {
		return result, nil
	}
	currentRevision := ""
	var deployment kubeDeployment
	if err := k8sGetJSON(client, runtime, "/apis/apps/v1/namespaces/"+service.Namespace+"/deployments/"+workloadName, &deployment); err == nil {
		currentRevision = deployment.Metadata.Annotations["deployment.kubernetes.io/revision"]
	}
	var replicaSetList kubeReplicaSetListResponse
	if err := k8sGetJSON(client, runtime, "/apis/apps/v1/namespaces/"+service.Namespace+"/replicasets", &replicaSetList); err != nil {
		return result, nil
	}
	pods, _ := fetchPodsByNamespace(client, runtime, service.Namespace)
	replicaSets := make([]map[string]any, 0)
	for _, item := range replicaSetList.Items {
		if !hasK8sOwner(item.Metadata.OwnerReferences, "Deployment", workloadName) {
			continue
		}
		relatedPods := make([]kubePod, 0)
		for _, pod := range pods {
			if hasK8sOwner(pod.Metadata.OwnerReferences, "ReplicaSet", item.Metadata.Name) {
				relatedPods = append(relatedPods, pod)
			}
		}
		desired := intValue(item.Spec.Replicas)
		revision := strings.TrimSpace(item.Metadata.Annotations["deployment.kubernetes.io/revision"])
		replicaSets = append(replicaSets, map[string]any{"name": item.Metadata.Name, "revision": revision, "current": revision != "" && revision == currentRevision, "ready": fmt.Sprintf("%d/%d", item.Status.ReadyReplicas, desired), "available": item.Status.AvailableReplicas, "age": humanizeAge(item.Metadata.CreationTimestamp), "healthy": desired > 0 && item.Status.ReadyReplicas == desired && item.Status.AvailableReplicas >= desired, "pods": buildPodItems(relatedPods)})
	}
	result["replicaSets"] = replicaSets
	return result, nil
}

func (s *Service) GetAssetServiceWorkloadMetrics(serviceID uint, workloadType, workloadName, rangeKey string) (map[string]any, error) {
	service, err := s.GetAssetService(serviceID)
	if err != nil {
		return nil, err
	}
	if !assetServiceContainsWorkload(service, workloadType, workloadName) {
		return nil, apperr.New("ASSET_SERVICE_WORKLOAD_MISMATCH", nil)
	}
	clusterID, err := s.assetServiceClusterID(service)
	if err != nil {
		return nil, err
	}
	detail, err := s.GetK8sWorkloadDetail(clusterID, service.Namespace, workloadType, workloadName)
	if err != nil {
		return nil, err
	}
	podNames := make([]string, 0, len(detail.Pods))
	for _, pod := range detail.Pods {
		podNames = append(podNames, pod.Name)
	}
	return s.getK8sPodMetricComparison(clusterID, service.Namespace, podNames, rangeKey)
}

func workloadDetailHealthy(detail model.K8sWorkloadDetail) bool {
	parts := strings.Split(detail.Ready, "/")
	if len(parts) != 2 {
		return false
	}
	ready, readyErr := strconv.Atoi(strings.TrimSpace(parts[0]))
	desired, desiredErr := strconv.Atoi(strings.TrimSpace(parts[1]))
	return readyErr == nil && desiredErr == nil && desired > 0 && ready == desired && detail.Available >= desired
}

func (s *Service) GetAssetServiceWorkloadRolloutHistory(serviceID uint, workloadType, workloadName string) (map[string]any, error) {
	service, err := s.GetAssetService(serviceID)
	if err != nil {
		return nil, err
	}
	if !assetServiceContainsWorkload(service, workloadType, workloadName) {
		return nil, apperr.New("ASSET_SERVICE_WORKLOAD_MISMATCH", nil)
	}
	if !strings.EqualFold(Trimmed(workloadType), "deployment") {
		return nil, apperr.New("ASSET_SERVICE_ROLLBACK_DEPLOYMENT_ONLY", nil)
	}
	clusterID, err := s.assetServiceClusterID(service)
	if err != nil {
		return nil, err
	}
	_, runtime, client, err := s.k8sClientForCluster(clusterID)
	if err != nil {
		return nil, err
	}
	var deployment kubeDeployment
	deploymentPath := "/apis/apps/v1/namespaces/" + service.Namespace + "/deployments/" + workloadName
	if err := k8sGetJSON(client, runtime, deploymentPath, &deployment); err != nil {
		return nil, apperr.New("K8S_CLUSTER_CONNECTION_FAILED", nil)
	}
	var replicaSets kubeReplicaSetListResponse
	if err := k8sGetJSON(client, runtime, "/apis/apps/v1/namespaces/"+service.Namespace+"/replicasets", &replicaSets); err != nil {
		return nil, apperr.New("K8S_CLUSTER_CONNECTION_FAILED", nil)
	}
	currentRevision := deployment.Metadata.Annotations["deployment.kubernetes.io/revision"]
	history := make([]map[string]any, 0)
	for _, item := range replicaSets.Items {
		if !hasK8sOwner(item.Metadata.OwnerReferences, "Deployment", workloadName) {
			continue
		}
		revision := strings.TrimSpace(item.Metadata.Annotations["deployment.kubernetes.io/revision"])
		if revision == "" || len(item.Spec.Template) == 0 {
			continue
		}
		history = append(history, map[string]any{"revision": revision, "replicaSet": item.Metadata.Name, "age": humanizeAge(item.Metadata.CreationTimestamp), "images": workloadTemplateImages(item.Spec.Template), "current": revision == currentRevision, "ready": fmt.Sprintf("%d/%d", item.Status.ReadyReplicas, intValue(item.Spec.Replicas))})
	}
	sort.Slice(history, func(i, j int) bool {
		left, _ := strconv.Atoi(fmt.Sprint(history[i]["revision"]))
		right, _ := strconv.Atoi(fmt.Sprint(history[j]["revision"]))
		return left > right
	})
	return map[string]any{"workloadName": workloadName, "currentRevision": currentRevision, "history": history}, nil
}

func (s *Service) RollbackAssetServiceWorkload(payload AssetServiceWorkloadRollbackPayload) (map[string]any, error) {
	service, err := s.GetAssetService(payload.ServiceID)
	if err != nil {
		return nil, err
	}
	if !assetServiceContainsWorkload(service, payload.WorkloadType, payload.WorkloadName) {
		return nil, apperr.New("ASSET_SERVICE_WORKLOAD_MISMATCH", nil)
	}
	if !strings.EqualFold(Trimmed(payload.WorkloadType), "deployment") || Trimmed(payload.Revision) == "" {
		return nil, apperr.New("ASSET_SERVICE_ROLLBACK_DEPLOYMENT_ONLY", nil)
	}
	clusterID, err := s.assetServiceClusterID(service)
	if err != nil {
		return nil, err
	}
	_, runtime, client, err := s.k8sClientForCluster(clusterID)
	if err != nil {
		return nil, err
	}
	var replicaSets kubeReplicaSetListResponse
	if err := k8sGetJSON(client, runtime, "/apis/apps/v1/namespaces/"+service.Namespace+"/replicasets", &replicaSets); err != nil {
		return nil, apperr.New("K8S_CLUSTER_CONNECTION_FAILED", nil)
	}
	var target *kubeReplicaSet
	for i := range replicaSets.Items {
		item := &replicaSets.Items[i]
		if hasK8sOwner(item.Metadata.OwnerReferences, "Deployment", payload.WorkloadName) && item.Metadata.Annotations["deployment.kubernetes.io/revision"] == Trimmed(payload.Revision) {
			target = item
			break
		}
	}
	if target == nil || len(target.Spec.Template) == 0 {
		return nil, apperr.New("ASSET_SERVICE_ROLLBACK_REVISION_NOT_FOUND", nil)
	}
	path := "/apis/apps/v1/namespaces/" + service.Namespace + "/deployments/" + payload.WorkloadName
	if err := k8sPatchJSON(client, runtime, path, map[string]any{"spec": map[string]any{"template": target.Spec.Template}}, "application/strategic-merge-patch+json", nil); err != nil {
		return nil, apperr.New("K8S_CLUSTER_CONNECTION_FAILED", nil)
	}
	return map[string]any{"workloadName": payload.WorkloadName, "rollbackRevision": payload.Revision, "replicaSet": target.Metadata.Name}, nil
}

func workloadTemplateImages(template map[string]any) []string {
	spec, _ := template["spec"].(map[string]any)
	containers, _ := spec["containers"].([]any)
	images := make([]string, 0, len(containers))
	for _, raw := range containers {
		container, _ := raw.(map[string]any)
		name, image := strings.TrimSpace(fmt.Sprint(container["name"])), strings.TrimSpace(fmt.Sprint(container["image"]))
		if image != "" {
			images = append(images, strings.Trim(strings.TrimSpace(name+": "+image), ": "))
		}
	}
	return images
}

func (s *Service) GetAssetServiceWorkloadLogs(serviceID uint, workloadType, workloadName, podName, container string, tailLines int) (map[string]any, error) {
	service, err := s.GetAssetService(serviceID)
	if err != nil {
		return nil, err
	}
	if !assetServiceContainsWorkload(service, workloadType, workloadName) {
		return nil, apperr.New("ASSET_SERVICE_WORKLOAD_MISMATCH", nil)
	}
	clusterID, err := s.assetServiceClusterID(service)
	if err != nil {
		return nil, err
	}
	detail, err := s.GetK8sWorkloadDetail(clusterID, service.Namespace, workloadType, workloadName)
	if err != nil {
		return nil, err
	}
	podName = Trimmed(podName)
	for _, pod := range detail.Pods {
		if pod.Name == podName {
			return s.GetK8sPodLogs(clusterID, service.Namespace, podName, container, tailLines)
		}
	}
	return nil, apperr.New("ASSET_SERVICE_POD_MISMATCH", nil)
}

func assetServiceContainsWorkload(service *model.AssetService, workloadType, workloadName string) bool {
	workloadType, workloadName = strings.ToLower(Trimmed(workloadType)), Trimmed(workloadName)
	for _, item := range service.Workloads {
		if strings.EqualFold(item.WorkloadType, workloadType) && item.WorkloadName == workloadName {
			return true
		}
	}
	return false
}

func labelsMatch(required, actual map[string]string) bool {
	if len(required) == 0 {
		return false
	}
	for key, value := range required {
		if actual[key] != value {
			return false
		}
	}
	return true
}

func hasK8sOwner(owners []struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}, kind, name string) bool {
	for _, owner := range owners {
		if strings.EqualFold(owner.Kind, kind) && owner.Name == name {
			return true
		}
	}
	return false
}

func assetServiceUID(apiServer, namespace, name string) string {
	host := strings.TrimSpace(apiServer)
	if parsed, err := url.Parse(host); err == nil && parsed.Hostname() != "" {
		host = parsed.Hostname()
	}
	host = strings.ReplaceAll(strings.Trim(strings.TrimSpace(host), "/"), ":", "-")
	return strings.Join([]string{host, Trimmed(namespace), Trimmed(name)}, "-")
}
