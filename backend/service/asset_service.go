package service

import (
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"gorm.io/gorm"
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
	return map[string]any{"list": list, "total": total, "pageNum": pageNum, "pageSize": pageSize}, nil
}

func (s *Service) GetAssetService(id uint) (*model.AssetService, error) {
	var item model.AssetService
	if err := s.db.Preload("Workloads").First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Service) SaveAssetService(payload AssetServicePayload) error {
	name, namespace := Trimmed(payload.Name), Trimmed(payload.Namespace)
	if name == "" || payload.K8sClusterID == 0 || namespace == "" {
		return errors.New("service name, kubernetes cluster and namespace are required")
	}
	if len(payload.Workloads) == 0 {
		return errors.New("at least one workload is required")
	}
	var cluster model.K8sCluster
	if err := s.db.First(&cluster, payload.K8sClusterID).Error; err != nil {
		return errors.New("selected kubernetes cluster does not exist")
	}
	item := model.AssetService{Name: name, ServiceUID: assetServiceUID(cluster.APIServer, namespace, name), K8sClusterID: payload.K8sClusterID, Namespace: namespace, ServiceType: Trimmed(payload.ServiceType), Status: payload.Status, Description: Trimmed(payload.Description)}
	if item.ServiceType == "" {
		item.ServiceType = "业务服务"
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
			result := tx.Model(&model.AssetService{}).Where("id = ?", serviceID).Updates(map[string]any{"name": item.Name, "service_uid": item.ServiceUID, "k8s_cluster_id": item.K8sClusterID, "namespace": item.Namespace, "service_type": item.ServiceType, "status": item.Status, "description": item.Description})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return errors.New("service does not exist")
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
		return nil, errors.New("kubernetes cluster is required")
	}
	cluster, runtime, client, err := s.k8sClientForCluster(clusterID)
	if err != nil {
		return nil, err
	}
	// The service form only needs namespaces and workload controllers. Avoid the
	// expensive full overview (pods, secrets, storage, services, gateway APIs).
	var namespaceResp kubeNamespaceListResponse
	if err := k8sGetJSON(client, runtime, "/api/v1/namespaces", &namespaceResp); err != nil {
		return nil, errors.New(k8sClusterConnectError)
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
	var cluster model.K8sCluster
	if err := s.db.First(&cluster, service.K8sClusterID).Error; err != nil {
		return nil, err
	}
	result := map[string]any{"service": service, "cluster": toK8sClusterView(cluster), "namespace": service.Namespace, "source": "saved", "workloads": service.Workloads}
	workloads := make([]model.K8sWorkloadItem, 0, len(service.Workloads))
	for _, item := range service.Workloads {
		detail, detailErr := s.GetK8sWorkloadDetail(service.K8sClusterID, service.Namespace, item.WorkloadType, item.WorkloadName)
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

// GetAssetServiceWorkloadRuntime returns runtime data only for a workload that
// belongs to the selected asset service.  It deliberately does not expose the
// generic K8s endpoint as an asset-service API.
func (s *Service) GetAssetServiceWorkloadRuntime(serviceID uint, workloadType, workloadName string) (model.K8sWorkloadDetail, error) {
	service, err := s.GetAssetService(serviceID)
	if err != nil {
		return model.K8sWorkloadDetail{}, err
	}
	if !assetServiceContainsWorkload(service, workloadType, workloadName) {
		return model.K8sWorkloadDetail{}, errors.New("workload does not belong to this service")
	}
	return s.GetK8sWorkloadDetail(service.K8sClusterID, service.Namespace, workloadType, workloadName)
}

// GetAssetServiceWorkloadTopology exposes the actual Kubernetes objects for a
// selected service workload: matching Service, workload, ReplicaSets and Pods.
func (s *Service) GetAssetServiceWorkloadTopology(serviceID uint, workloadType, workloadName string) (map[string]any, error) {
	service, err := s.GetAssetService(serviceID)
	if err != nil {
		return nil, err
	}
	if !assetServiceContainsWorkload(service, workloadType, workloadName) {
		return nil, errors.New("workload does not belong to this service")
	}
	detail, err := s.GetK8sWorkloadDetail(service.K8sClusterID, service.Namespace, workloadType, workloadName)
	if err != nil {
		return nil, err
	}
	result := map[string]any{"workload": detail, "services": []map[string]any{}, "replicaSets": []map[string]any{}, "statefulSet": nil}
	_, runtime, client, err := s.k8sClientForCluster(service.K8sClusterID)
	if err != nil {
		return result, nil
	}

	var serviceList kubeServiceListResponse
	if err := k8sGetJSON(client, runtime, "/api/v1/namespaces/"+service.Namespace+"/services", &serviceList); err == nil {
		items := make([]map[string]any, 0)
		for _, item := range serviceList.Items {
			// A Service targets Pod labels. The workload selector is preferred,
			// while workload labels provide a fallback for StatefulSets.
			if labelsMatch(item.Spec.Selector, detail.Selector) || labelsMatch(item.Spec.Selector, detail.Labels) {
				items = append(items, map[string]any{"name": item.Metadata.Name, "type": item.Spec.Type, "clusterIP": item.Spec.ClusterIP, "age": humanizeAge(item.Metadata.CreationTimestamp), "healthy": true})
			}
		}
		result["services"] = items
	}
	if strings.EqualFold(workloadType, "statefulset") {
		result["statefulSet"] = map[string]any{
			"name":      detail.Name,
			"ready":     detail.Ready,
			"available": detail.Available,
			"age":       detail.Age,
			"healthy":   workloadDetailHealthy(detail),
			"pods":      detail.Pods,
		}
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

func workloadDetailHealthy(detail model.K8sWorkloadDetail) bool {
	parts := strings.Split(detail.Ready, "/")
	if len(parts) != 2 {
		return false
	}
	ready, readyErr := strconv.Atoi(strings.TrimSpace(parts[0]))
	desired, desiredErr := strconv.Atoi(strings.TrimSpace(parts[1]))
	return readyErr == nil && desiredErr == nil && desired > 0 && ready == desired && detail.Available >= desired
}

// GetAssetServiceWorkloadRolloutHistory returns Deployment revisions which are
// represented by ReplicaSets controlled by the selected Deployment.
func (s *Service) GetAssetServiceWorkloadRolloutHistory(serviceID uint, workloadType, workloadName string) (map[string]any, error) {
	service, err := s.GetAssetService(serviceID)
	if err != nil {
		return nil, err
	}
	if !assetServiceContainsWorkload(service, workloadType, workloadName) {
		return nil, errors.New("workload does not belong to this service")
	}
	if !strings.EqualFold(Trimmed(workloadType), "deployment") {
		return nil, errors.New("only deployment supports version rollback")
	}
	_, runtime, client, err := s.k8sClientForCluster(service.K8sClusterID)
	if err != nil {
		return nil, err
	}
	var deployment kubeDeployment
	deploymentPath := "/apis/apps/v1/namespaces/" + service.Namespace + "/deployments/" + workloadName
	if err := k8sGetJSON(client, runtime, deploymentPath, &deployment); err != nil {
		return nil, errors.New(k8sClusterConnectError)
	}
	var replicaSets kubeReplicaSetListResponse
	if err := k8sGetJSON(client, runtime, "/apis/apps/v1/namespaces/"+service.Namespace+"/replicasets", &replicaSets); err != nil {
		return nil, errors.New(k8sClusterConnectError)
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
		history = append(history, map[string]any{
			"revision": revision, "replicaSet": item.Metadata.Name, "age": humanizeAge(item.Metadata.CreationTimestamp),
			"images": workloadTemplateImages(item.Spec.Template), "current": revision == currentRevision,
			"ready": fmt.Sprintf("%d/%d", item.Status.ReadyReplicas, intValue(item.Spec.Replicas)),
		})
	}
	sort.Slice(history, func(i, j int) bool {
		left, _ := strconv.Atoi(fmt.Sprint(history[i]["revision"]))
		right, _ := strconv.Atoi(fmt.Sprint(history[j]["revision"]))
		return left > right
	})
	return map[string]any{"workloadName": workloadName, "currentRevision": currentRevision, "history": history}, nil
}

// RollbackAssetServiceWorkload restores the pod template from the requested
// Deployment ReplicaSet revision. Kubernetes creates a new revision for this
// operation, equivalent to kubectl rollout undo --to-revision.
func (s *Service) RollbackAssetServiceWorkload(payload AssetServiceWorkloadRollbackPayload) (map[string]any, error) {
	service, err := s.GetAssetService(payload.ServiceID)
	if err != nil {
		return nil, err
	}
	if !assetServiceContainsWorkload(service, payload.WorkloadType, payload.WorkloadName) {
		return nil, errors.New("workload does not belong to this service")
	}
	if !strings.EqualFold(Trimmed(payload.WorkloadType), "deployment") || Trimmed(payload.Revision) == "" {
		return nil, errors.New("only deployment supports version rollback")
	}
	_, runtime, client, err := s.k8sClientForCluster(service.K8sClusterID)
	if err != nil {
		return nil, err
	}
	var replicaSets kubeReplicaSetListResponse
	if err := k8sGetJSON(client, runtime, "/apis/apps/v1/namespaces/"+service.Namespace+"/replicasets", &replicaSets); err != nil {
		return nil, errors.New(k8sClusterConnectError)
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
		return nil, errors.New("rollback revision does not exist")
	}
	path := "/apis/apps/v1/namespaces/" + service.Namespace + "/deployments/" + payload.WorkloadName
	if err := k8sPatchJSON(client, runtime, path, map[string]any{"spec": map[string]any{"template": target.Spec.Template}}, "application/strategic-merge-patch+json", nil); err != nil {
		return nil, errors.New(k8sClusterConnectError)
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

// GetAssetServiceWorkloadLogs limits pod logs to pods returned by the selected
// service workload, so service management never becomes a free-form cluster
// log browser.
func (s *Service) GetAssetServiceWorkloadLogs(serviceID uint, workloadType, workloadName, podName, container string, tailLines int) (map[string]any, error) {
	service, err := s.GetAssetService(serviceID)
	if err != nil {
		return nil, err
	}
	if !assetServiceContainsWorkload(service, workloadType, workloadName) {
		return nil, errors.New("workload does not belong to this service")
	}
	detail, err := s.GetK8sWorkloadDetail(service.K8sClusterID, service.Namespace, workloadType, workloadName)
	if err != nil {
		return nil, err
	}
	podName = Trimmed(podName)
	for _, pod := range detail.Pods {
		if pod.Name == podName {
			return s.GetK8sPodLogs(service.K8sClusterID, service.Namespace, podName, container, tailLines)
		}
	}
	return nil, errors.New("pod does not belong to this workload")
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
