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
