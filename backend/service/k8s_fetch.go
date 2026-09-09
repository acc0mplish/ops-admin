// k8s_fetch.go — moved verbatim from k8s.go (Phase BCD, E5 seam #8).
package service

import (
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"ops-admin/backend/model"

	"gopkg.in/yaml.v3"
)

func fetchK8sData(client *http.Client, runtime kubeClusterRuntime) (k8sFetchedData, error) {
	var data k8sFetchedData

	var nodeResp kubeNodeListResponse
	if err := k8sGetJSON(client, runtime, "/api/v1/nodes", &nodeResp); err != nil {
		return data, err
	}
	data.Nodes = nodeResp.Items

	var namespaceResp kubeNamespaceListResponse
	if err := k8sGetJSON(client, runtime, "/api/v1/namespaces", &namespaceResp); err != nil {
		return data, err
	}
	data.Namespaces = namespaceResp.Items

	var podResp kubePodListResponse
	if err := k8sGetJSON(client, runtime, "/api/v1/pods", &podResp); err != nil {
		return data, err
	}
	data.Pods = podResp.Items

	var serviceResp kubeServiceListResponse
	if err := k8sGetJSON(client, runtime, "/api/v1/services", &serviceResp); err != nil {
		return data, err
	}
	data.Services = serviceResp.Items

	var endpointResp kubeEndpointListResponse
	if err := k8sGetJSON(client, runtime, "/api/v1/endpoints", &endpointResp); err != nil {
		return data, err
	}
	data.Endpoints = endpointResp.Items

	var ingressResp kubeIngressListResponse
	if err := k8sGetJSON(client, runtime, "/apis/networking.k8s.io/v1/ingresses", &ingressResp); err == nil {
		data.Ingresses = ingressResp.Items
	}

	var gatewayAPIResp kubeGatewayAPIListResponse
	if err := k8sGetGatewayAPIJSON(client, runtime, "gateways", "", "", &gatewayAPIResp); err == nil {
		data.GatewayAPIGateways = gatewayAPIResp.Items
	}

	var httpRouteResp kubeHTTPRouteListResponse
	if err := k8sGetGatewayAPIJSON(client, runtime, "httproutes", "", "", &httpRouteResp); err == nil {
		data.HTTPRoutes = httpRouteResp.Items
	}

	var configMapResp kubeConfigMapListResponse
	if err := k8sGetJSON(client, runtime, "/api/v1/configmaps", &configMapResp); err != nil {
		return data, err
	}
	data.ConfigMaps = configMapResp.Items

	var secretResp kubeSecretListResponse
	if err := k8sGetJSON(client, runtime, "/api/v1/secrets", &secretResp); err != nil {
		return data, err
	}
	data.Secrets = secretResp.Items

	var pvcResp kubePVCListResponse
	if err := k8sGetJSON(client, runtime, "/api/v1/persistentvolumeclaims", &pvcResp); err == nil {
		data.PVCs = pvcResp.Items
	}

	var pvResp kubePVListResponse
	if err := k8sGetJSON(client, runtime, "/api/v1/persistentvolumes", &pvResp); err == nil {
		data.PVs = pvResp.Items
	}

	var deploymentResp kubeDeploymentListResponse
	if err := k8sGetJSON(client, runtime, "/apis/apps/v1/deployments", &deploymentResp); err == nil {
		data.Deployments = deploymentResp.Items
	}

	var replicaSetResp kubeReplicaSetListResponse
	if err := k8sGetJSON(client, runtime, "/apis/apps/v1/replicasets", &replicaSetResp); err == nil {
		data.ReplicaSets = replicaSetResp.Items
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

	return data, nil
}

func (s *Service) k8sClientForCluster(clusterID uint) (model.K8sCluster, kubeClusterRuntime, *http.Client, error) {
	cluster, err := s.GetK8sCluster(clusterID)
	if err != nil {
		return model.K8sCluster{}, kubeClusterRuntime{}, nil, err
	}
	runtime, err := parseKubeConfig(cluster.KubeConfig)
	if err != nil {
		return cluster, kubeClusterRuntime{}, nil, errors.New(k8sClusterConnectError)
	}
	client, cleanup, err := s.newK8sHTTPClientForCluster(cluster, runtime)
	if err != nil {
		return cluster, kubeClusterRuntime{}, nil, errors.New(k8sClusterConnectError)
	}
	_ = cleanup
	return cluster, runtime, client, nil
}

func fetchPodsForNode(client *http.Client, runtime kubeClusterRuntime, nodeName string) ([]kubePod, error) {
	var payload kubePodListResponse
	if err := k8sGetJSONWithQuery(client, runtime, "/api/v1/pods", map[string]string{
		"fieldSelector": "spec.nodeName=" + nodeName,
	}, &payload); err != nil {
		return nil, err
	}
	return payload.Items, nil
}

func fetchPodsByNamespace(client *http.Client, runtime kubeClusterRuntime, namespace string) ([]kubePod, error) {
	var payload kubePodListResponse
	if err := k8sGetJSON(client, runtime, "/api/v1/namespaces/"+namespace+"/pods", &payload); err != nil {
		return nil, err
	}
	return payload.Items, nil
}

func fetchNamespacedEvents(client *http.Client, runtime kubeClusterRuntime, namespace string, fieldSelector string) ([]model.K8sEventItem, error) {
	var payload struct {
		Items []struct {
			Type      string `json:"type"`
			Reason    string `json:"reason"`
			Message   string `json:"message"`
			Count     int    `json:"count"`
			FirstTime string `json:"firstTimestamp"`
			LastTime  string `json:"lastTimestamp"`
		} `json:"items"`
	}
	if err := k8sGetJSONWithQuery(client, runtime, "/api/v1/namespaces/"+namespace+"/events", map[string]string{"fieldSelector": fieldSelector}, &payload); err != nil {
		return nil, errors.New(k8sClusterConnectError)
	}

	events := make([]model.K8sEventItem, 0, len(payload.Items))
	for _, item := range payload.Items {
		events = append(events, model.K8sEventItem{
			Type:      fallbackText(item.Type),
			Reason:    fallbackText(item.Reason),
			Message:   fallbackText(item.Message),
			Count:     item.Count,
			FirstTime: formatTimestamp(item.FirstTime),
			LastTime:  formatTimestamp(item.LastTime),
		})
	}
	sort.Slice(events, func(i, j int) bool { return events[i].LastTime > events[j].LastTime })
	return events, nil
}

func fetchNamespaceWorkloadCount(client *http.Client, runtime kubeClusterRuntime, namespace string) (int, error) {
	total := 0

	var deployments kubeDeploymentListResponse
	if err := k8sGetJSON(client, runtime, "/apis/apps/v1/namespaces/"+namespace+"/deployments", &deployments); err != nil {
		return 0, err
	}
	total += len(deployments.Items)

	var statefulsets kubeStatefulSetListResponse
	if err := k8sGetJSON(client, runtime, "/apis/apps/v1/namespaces/"+namespace+"/statefulsets", &statefulsets); err == nil {
		total += len(statefulsets.Items)
	}

	var daemonsets kubeDaemonSetListResponse
	if err := k8sGetJSON(client, runtime, "/apis/apps/v1/namespaces/"+namespace+"/daemonsets", &daemonsets); err == nil {
		total += len(daemonsets.Items)
	}

	var jobs kubeJobListResponse
	if err := k8sGetJSON(client, runtime, "/apis/batch/v1/namespaces/"+namespace+"/jobs", &jobs); err == nil {
		total += len(jobs.Items)
	}

	var cronjobs kubeCronJobListResponse
	if err := k8sGetJSON(client, runtime, "/apis/batch/v1/namespaces/"+namespace+"/cronjobs", &cronjobs); err == nil {
		total += len(cronjobs.Items)
	}

	return total, nil
}

func matchLabels(labels map[string]string, selector map[string]string) bool {
	if len(selector) == 0 {
		return false
	}
	for key, value := range selector {
		if labels[key] != value {
			return false
		}
	}
	return true
}

func filterPodsBySelector(pods []kubePod, namespace string, selector map[string]string) []kubePod {
	result := make([]kubePod, 0)
	for _, pod := range pods {
		if pod.Metadata.Namespace != namespace {
			continue
		}
		if matchLabels(pod.Metadata.Labels, selector) {
			result = append(result, pod)
		}
	}
	return result
}

func filterPodsByOwnerOrSelector(pods []kubePod, namespace string, selector map[string]string, ownerKind string, ownerName string) []kubePod {
	result := make([]kubePod, 0)
	for _, pod := range pods {
		if pod.Metadata.Namespace != namespace {
			continue
		}
		matched := false
		for _, owner := range pod.Metadata.OwnerReferences {
			if strings.EqualFold(owner.Kind, ownerKind) && owner.Name == ownerName {
				matched = true
				break
			}
		}
		if matched || matchLabels(pod.Metadata.Labels, selector) {
			result = append(result, pod)
		}
	}
	return result
}

func buildContainerItems(containers []kubeContainer) []model.K8sContainerItem {
	items := make([]model.K8sContainerItem, 0, len(containers))
	for _, container := range containers {
		items = append(items, model.K8sContainerItem{
			Name:            container.Name,
			Image:           container.Image,
			RequestCPU:      container.Resources.Requests["cpu"],
			LimitCPU:        container.Resources.Limits["cpu"],
			RequestMemory:   container.Resources.Requests["memory"],
			LimitMemory:     container.Resources.Limits["memory"],
			ImagePullPolicy: container.ImagePullPolicy,
			Env:             buildContainerEnvItems(container.Env),
		})
	}
	return items
}

func marshalK8sYAML(v any) string {
	body, err := yaml.Marshal(v)
	if err != nil {
		return ""
	}
	return string(body)
}

func buildDeploymentDetail(client *http.Client, runtime kubeClusterRuntime, item kubeDeployment) model.K8sWorkloadDetail {
	pods, _ := fetchPodsByNamespace(client, runtime, item.Metadata.Namespace)
	relatedPods := filterPodsBySelector(pods, item.Metadata.Namespace, item.Spec.Selector.MatchLabels)
	replicas := intValue(item.Spec.Replicas)
	return model.K8sWorkloadDetail{
		Name:        item.Metadata.Name,
		Type:        "Deployment",
		Namespace:   item.Metadata.Namespace,
		Ready:       fmt.Sprintf("%d/%d", item.Status.ReadyReplicas, replicas),
		Updated:     item.Status.UpdatedReplicas,
		Available:   item.Status.AvailableReplicas,
		Age:         humanizeAge(item.Metadata.CreationTimestamp),
		Labels:      item.Metadata.Labels,
		Annotations: item.Metadata.Annotations,
		Selector:    item.Spec.Selector.MatchLabels,
		Pods:        buildPodItems(relatedPods),
		Containers:  buildContainerItems(item.Spec.Template.Spec.Containers),
		YAML:        marshalK8sYAML(item),
	}
}

func buildStatefulSetDetail(client *http.Client, runtime kubeClusterRuntime, item kubeStatefulSet) model.K8sWorkloadDetail {
	pods, _ := fetchPodsByNamespace(client, runtime, item.Metadata.Namespace)
	relatedPods := filterPodsBySelector(pods, item.Metadata.Namespace, item.Spec.Selector.MatchLabels)
	replicas := intValue(item.Spec.Replicas)
	return model.K8sWorkloadDetail{
		Name:        item.Metadata.Name,
		Type:        "StatefulSet",
		Namespace:   item.Metadata.Namespace,
		Ready:       fmt.Sprintf("%d/%d", item.Status.ReadyReplicas, replicas),
		Updated:     item.Status.UpdatedReplicas,
		Available:   item.Status.AvailableReplicas,
		Age:         humanizeAge(item.Metadata.CreationTimestamp),
		Labels:      item.Metadata.Labels,
		Annotations: item.Metadata.Annotations,
		Selector:    item.Spec.Selector.MatchLabels,
		Pods:        buildPodItems(relatedPods),
		Containers:  buildContainerItems(item.Spec.Template.Spec.Containers),
		YAML:        marshalK8sYAML(item),
	}
}

func buildDaemonSetDetail(client *http.Client, runtime kubeClusterRuntime, item kubeDaemonSet) model.K8sWorkloadDetail {
	pods, _ := fetchPodsByNamespace(client, runtime, item.Metadata.Namespace)
	relatedPods := filterPodsBySelector(pods, item.Metadata.Namespace, item.Spec.Selector.MatchLabels)
	return model.K8sWorkloadDetail{
		Name:        item.Metadata.Name,
		Type:        "DaemonSet",
		Namespace:   item.Metadata.Namespace,
		Ready:       fmt.Sprintf("%d/%d", item.Status.NumberReady, item.Status.DesiredNumberScheduled),
		Updated:     item.Status.UpdatedNumberScheduled,
		Available:   item.Status.NumberAvailable,
		Age:         humanizeAge(item.Metadata.CreationTimestamp),
		Labels:      item.Metadata.Labels,
		Annotations: item.Metadata.Annotations,
		Selector:    item.Spec.Selector.MatchLabels,
		Pods:        buildPodItems(relatedPods),
		Containers:  buildContainerItems(item.Spec.Template.Spec.Containers),
		YAML:        marshalK8sYAML(item),
	}
}

func buildJobDetail(client *http.Client, runtime kubeClusterRuntime, item kubeJob) model.K8sWorkloadDetail {
	pods, _ := fetchPodsByNamespace(client, runtime, item.Metadata.Namespace)
	selector := map[string]string{"job-name": item.Metadata.Name}
	if item.Spec.Selector != nil && len(item.Spec.Selector.MatchLabels) > 0 {
		selector = item.Spec.Selector.MatchLabels
	}
	relatedPods := filterPodsByOwnerOrSelector(pods, item.Metadata.Namespace, selector, "Job", item.Metadata.Name)
	total := intValue(item.Spec.Completions)
	if total == 0 {
		total = item.Status.Active + item.Status.Succeeded + item.Status.Failed
	}
	return model.K8sWorkloadDetail{
		Name:        item.Metadata.Name,
		Type:        "Job",
		Namespace:   item.Metadata.Namespace,
		Ready:       fmt.Sprintf("%d/%d", item.Status.Succeeded, total),
		Updated:     item.Status.Active,
		Available:   item.Status.Succeeded,
		Age:         humanizeAge(item.Metadata.CreationTimestamp),
		Labels:      item.Metadata.Labels,
		Annotations: item.Metadata.Annotations,
		Selector:    selector,
		Pods:        buildPodItems(relatedPods),
		Containers:  buildContainerItems(item.Spec.Template.Spec.Containers),
		YAML:        marshalK8sYAML(item),
	}
}

func buildCronJobDetail(client *http.Client, runtime kubeClusterRuntime, item kubeCronJob) model.K8sWorkloadDetail {
	pods, _ := fetchPodsByNamespace(client, runtime, item.Metadata.Namespace)
	relatedPods := make([]kubePod, 0)
	for _, pod := range pods {
		if pod.Metadata.Namespace != item.Metadata.Namespace {
			continue
		}
		if strings.HasPrefix(pod.Metadata.Name, item.Metadata.Name+"-") {
			relatedPods = append(relatedPods, pod)
			continue
		}
		for _, owner := range pod.Metadata.OwnerReferences {
			if strings.EqualFold(owner.Kind, "Job") && strings.HasPrefix(owner.Name, item.Metadata.Name+"-") {
				relatedPods = append(relatedPods, pod)
				break
			}
		}
	}
	active := len(item.Status.Active)
	schedule := item.Spec.Schedule
	if strings.TrimSpace(schedule) == "" {
		schedule = "-"
	}
	return model.K8sWorkloadDetail{
		Name:        item.Metadata.Name,
		Type:        "CronJob",
		Namespace:   item.Metadata.Namespace,
		Ready:       cronJobReadyText(item.Spec.Suspend, active),
		Updated:     active,
		Available:   active,
		Age:         humanizeAge(item.Metadata.CreationTimestamp),
		Labels:      item.Metadata.Labels,
		Annotations: item.Metadata.Annotations,
		Selector:    map[string]string{"schedule": schedule},
		Pods:        buildPodItems(relatedPods),
		Containers:  buildContainerItems(item.Spec.JobTemplate.Spec.Template.Spec.Containers),
		YAML:        marshalK8sYAML(item),
	}
}
