// k8s_workload.go — moved verbatim from k8s.go (Phase BCD, E5 seam #6).
package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"ops-admin/backend/model"
)

func (s *Service) GetK8sWorkloadDetail(clusterID uint, namespace string, workloadType string, workloadName string) (model.K8sWorkloadDetail, error) {
	_, runtime, client, err := s.k8sClientForCluster(clusterID)
	if err != nil {
		return model.K8sWorkloadDetail{}, err
	}

	typeName := strings.ToLower(strings.TrimSpace(workloadType))
	switch typeName {
	case "deployment":
		var item kubeDeployment
		path := fmt.Sprintf("/apis/apps/v1/namespaces/%s/deployments/%s", namespace, workloadName)
		if err := k8sGetJSON(client, runtime, path, &item); err != nil {
			return model.K8sWorkloadDetail{}, errors.New(k8sClusterConnectError)
		}
		return buildDeploymentDetail(client, runtime, item), nil
	case "statefulset":
		var item kubeStatefulSet
		path := fmt.Sprintf("/apis/apps/v1/namespaces/%s/statefulsets/%s", namespace, workloadName)
		if err := k8sGetJSON(client, runtime, path, &item); err != nil {
			return model.K8sWorkloadDetail{}, errors.New(k8sClusterConnectError)
		}
		return buildStatefulSetDetail(client, runtime, item), nil
	case "daemonset":
		var item kubeDaemonSet
		path := fmt.Sprintf("/apis/apps/v1/namespaces/%s/daemonsets/%s", namespace, workloadName)
		if err := k8sGetJSON(client, runtime, path, &item); err != nil {
			return model.K8sWorkloadDetail{}, errors.New(k8sClusterConnectError)
		}
		return buildDaemonSetDetail(client, runtime, item), nil
	case "job":
		var item kubeJob
		path := fmt.Sprintf("/apis/batch/v1/namespaces/%s/jobs/%s", namespace, workloadName)
		if err := k8sGetJSON(client, runtime, path, &item); err != nil {
			return model.K8sWorkloadDetail{}, errors.New(k8sClusterConnectError)
		}
		return buildJobDetail(client, runtime, item), nil
	case "cronjob":
		var item kubeCronJob
		path := fmt.Sprintf("/apis/batch/v1/namespaces/%s/cronjobs/%s", namespace, workloadName)
		if err := k8sGetJSON(client, runtime, path, &item); err != nil {
			return model.K8sWorkloadDetail{}, errors.New(k8sClusterConnectError)
		}
		return buildCronJobDetail(client, runtime, item), nil
	default:
		return model.K8sWorkloadDetail{}, errors.New("unsupported workload type")
	}
}

func (s *Service) ScaleK8sWorkload(payload model.K8sWorkloadActionPayload) (map[string]any, error) {
	if payload.ClusterID == 0 || strings.TrimSpace(payload.Namespace) == "" || strings.TrimSpace(payload.WorkloadType) == "" || strings.TrimSpace(payload.WorkloadName) == "" {
		return nil, errors.New("invalid workload payload")
	}
	if payload.Replicas < 0 {
		return nil, errors.New("replicas must be greater than or equal to 0")
	}

	_, runtime, client, err := s.k8sClientForCluster(payload.ClusterID)
	if err != nil {
		return nil, err
	}

	workloadType := strings.ToLower(strings.TrimSpace(payload.WorkloadType))
	var path string
	switch workloadType {
	case "deployment":
		path = fmt.Sprintf("/apis/apps/v1/namespaces/%s/deployments/%s/scale", payload.Namespace, payload.WorkloadName)
	case "statefulset":
		path = fmt.Sprintf("/apis/apps/v1/namespaces/%s/statefulsets/%s/scale", payload.Namespace, payload.WorkloadName)
	default:
		return nil, errors.New("only deployment and statefulset support scaling")
	}

	patchBody := map[string]any{
		"spec": map[string]any{
			"replicas": payload.Replicas,
		},
	}
	if err := k8sPatchJSON(client, runtime, path, patchBody, "application/merge-patch+json", nil); err != nil {
		return nil, errors.New(k8sClusterConnectError)
	}
	s.invalidateK8sClusterDetailCache(payload.ClusterID)

	return map[string]any{
		"namespace":    payload.Namespace,
		"workloadType": payload.WorkloadType,
		"workloadName": payload.WorkloadName,
		"replicas":     payload.Replicas,
	}, nil
}

func (s *Service) RestartK8sWorkload(payload model.K8sWorkloadActionPayload) (map[string]any, error) {
	if payload.ClusterID == 0 || strings.TrimSpace(payload.Namespace) == "" || strings.TrimSpace(payload.WorkloadType) == "" || strings.TrimSpace(payload.WorkloadName) == "" {
		return nil, errors.New("invalid workload payload")
	}

	_, runtime, client, err := s.k8sClientForCluster(payload.ClusterID)
	if err != nil {
		return nil, err
	}

	workloadType := strings.ToLower(strings.TrimSpace(payload.WorkloadType))
	var path string
	switch workloadType {
	case "deployment":
		path = fmt.Sprintf("/apis/apps/v1/namespaces/%s/deployments/%s", payload.Namespace, payload.WorkloadName)
	case "statefulset":
		path = fmt.Sprintf("/apis/apps/v1/namespaces/%s/statefulsets/%s", payload.Namespace, payload.WorkloadName)
	case "daemonset":
		path = fmt.Sprintf("/apis/apps/v1/namespaces/%s/daemonsets/%s", payload.Namespace, payload.WorkloadName)
	default:
		return nil, errors.New("only deployment, statefulset and daemonset support restart")
	}

	patchBody := map[string]any{
		"spec": map[string]any{
			"template": map[string]any{
				"metadata": map[string]any{
					"annotations": map[string]any{
						"kubectl.kubernetes.io/restartedAt": time.Now().Format(time.RFC3339),
					},
				},
			},
		},
	}
	if err := k8sPatchJSON(client, runtime, path, patchBody, "application/strategic-merge-patch+json", nil); err != nil {
		return nil, errors.New(k8sClusterConnectError)
	}
	s.invalidateK8sClusterDetailCache(payload.ClusterID)
	return map[string]any{
		"namespace":    payload.Namespace,
		"workloadType": payload.WorkloadType,
		"workloadName": payload.WorkloadName,
		"restarted":    true,
	}, nil
}

func (s *Service) UpdateK8sWorkloadImages(payload model.K8sWorkloadImageBatchPayload) (map[string]any, error) {
	if payload.ClusterID == 0 {
		return nil, errors.New("invalid cluster payload")
	}
	version := strings.TrimSpace(payload.Version)
	if version == "" {
		return nil, errors.New("image version is required")
	}
	if len(payload.Items) == 0 {
		return nil, errors.New("please select workloads first")
	}

	_, runtime, client, err := s.k8sClientForCluster(payload.ClusterID)
	if err != nil {
		return nil, err
	}

	updated := make([]map[string]any, 0, len(payload.Items))
	for _, item := range payload.Items {
		namespace := strings.TrimSpace(item.Namespace)
		workloadType := strings.ToLower(strings.TrimSpace(item.WorkloadType))
		workloadName := strings.TrimSpace(item.WorkloadName)
		if namespace == "" || workloadType == "" || workloadName == "" {
			return nil, errors.New("invalid workload item")
		}

		path, err := k8sWorkloadResourcePath(namespace, workloadType, workloadName)
		if err != nil {
			return nil, err
		}

		resource := map[string]any{}
		if err := k8sGetJSON(client, runtime, path, &resource); err != nil {
			return nil, errors.New(k8sClusterConnectError)
		}

		containers, err := extractWorkloadContainers(resource)
		if err != nil {
			return nil, err
		}

		patchedContainers := make([]map[string]any, 0, len(containers))
		images := make([]string, 0, len(containers))
		for _, container := range containers {
			name := strings.TrimSpace(anyToString(container["name"]))
			image := strings.TrimSpace(anyToString(container["image"]))
			if name == "" || image == "" {
				continue
			}
			nextImage := replaceImageVersion(image, version)
			patchedContainers = append(patchedContainers, map[string]any{
				"name":  name,
				"image": nextImage,
			})
			images = append(images, nextImage)
		}
		if len(patchedContainers) == 0 {
			return nil, errors.New("no containers found in selected workload")
		}

		patchBody := buildWorkloadImagePatchBody(patchedContainers)
		if err := k8sPatchJSON(client, runtime, path, patchBody, "application/strategic-merge-patch+json", nil); err != nil {
			return nil, errors.New(k8sClusterConnectError)
		}

		updated = append(updated, map[string]any{
			"namespace":    namespace,
			"workloadType": item.WorkloadType,
			"workloadName": workloadName,
			"images":       images,
		})
	}
	s.invalidateK8sClusterDetailCache(payload.ClusterID)
	return map[string]any{
		"version": payload.Version,
		"count":   len(updated),
		"items":   updated,
	}, nil
}

// UpdateK8sWorkloadResources updates the editable pod-template settings while preserving
// container image and command configuration: CPU/memory resources, environment variables
// and image pull policy.
func (s *Service) UpdateK8sWorkloadResources(payload model.K8sWorkloadResourcesPayload) (map[string]any, error) {
	if payload.ClusterID == 0 || strings.TrimSpace(payload.Namespace) == "" || strings.TrimSpace(payload.WorkloadType) == "" || strings.TrimSpace(payload.WorkloadName) == "" || len(payload.Containers) == 0 {
		return nil, errors.New("invalid workload resource payload")
	}
	_, runtime, client, err := s.k8sClientForCluster(payload.ClusterID)
	if err != nil {
		return nil, err
	}
	path, err := k8sWorkloadResourcePath(payload.Namespace, payload.WorkloadType, payload.WorkloadName)
	if err != nil {
		return nil, err
	}
	resource := map[string]any{}
	if err := k8sGetJSON(client, runtime, path, &resource); err != nil {
		return nil, errors.New(k8sClusterConnectError)
	}
	existingContainers, err := extractWorkloadContainers(resource)
	if err != nil {
		return nil, err
	}
	existingByName := make(map[string]map[string]any, len(existingContainers))
	for _, container := range existingContainers {
		if name, ok := container["name"].(string); ok && strings.TrimSpace(name) != "" {
			existingByName[name] = container
		}
	}
	containers := make([]map[string]any, 0, len(payload.Containers))
	for _, item := range payload.Containers {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			return nil, errors.New("container name is required")
		}
		existing, ok := existingByName[name]
		if !ok {
			return nil, fmt.Errorf("container %s was not found in workload", name)
		}
		requests := map[string]any{}
		limits := map[string]any{}
		if value := strings.TrimSpace(item.RequestCPU); value != "" {
			requests["cpu"] = value
		}
		if value := strings.TrimSpace(item.RequestMemory); value != "" {
			requests["memory"] = value
		}
		if value := strings.TrimSpace(item.LimitCPU); value != "" {
			limits["cpu"] = value
		}
		if value := strings.TrimSpace(item.LimitMemory); value != "" {
			limits["memory"] = value
		}
		resources := map[string]any{}
		if len(requests) > 0 {
			resources["requests"] = requests
		}
		if len(limits) > 0 {
			resources["limits"] = limits
		}
		containerPatch := map[string]any{"name": name, "resources": resources}
		if policy := strings.TrimSpace(item.ImagePullPolicy); policy != "" {
			if policy != "Always" && policy != "IfNotPresent" && policy != "Never" {
				return nil, errors.New("invalid image pull policy")
			}
			containerPatch["imagePullPolicy"] = policy
		}
		envPatch, err := buildWorkloadEnvPatch(existing, item.Env)
		if err != nil {
			return nil, err
		}
		containerPatch["env"] = envPatch
		containers = append(containers, containerPatch)
	}
	patchBody := buildWorkloadContainerPatchBody(payload.WorkloadType, containers)
	if err := k8sPatchJSON(client, runtime, path, patchBody, "application/strategic-merge-patch+json", nil); err != nil {
		return nil, errors.New(k8sClusterConnectError)
	}
	s.invalidateK8sClusterDetailCache(payload.ClusterID)
	return map[string]any{
		"namespace": payload.Namespace, "workloadType": payload.WorkloadType,
		"workloadName": payload.WorkloadName, "containers": len(containers),
	}, nil
}
