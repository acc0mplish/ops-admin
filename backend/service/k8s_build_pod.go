// k8s_build_pod.go — moved verbatim from k8s.go (Phase BCD, E5 seam #9).
package service

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"ops-admin/backend/model"
)

type podWorkloadRef struct {
	Name string
	Type string
}

func podWorkloadKey(namespace, podName string) string {
	return namespace + "/" + podName
}

func buildPodItemsWithWorkloads(data k8sFetchedData) []model.K8sPodItem {
	refs := make(map[string]podWorkloadRef)
	workloadsByNamespace := make(map[string][]podWorkloadRef)
	addWorkload := func(namespace, name, workloadType string) {
		if name == "" {
			return
		}
		workloadsByNamespace[namespace] = append(workloadsByNamespace[namespace], podWorkloadRef{Name: name, Type: workloadType})
	}
	for _, item := range data.Deployments {
		addWorkload(item.Metadata.Namespace, item.Metadata.Name, "Deployment")
	}
	for _, item := range data.StatefulSet {
		addWorkload(item.Metadata.Namespace, item.Metadata.Name, "StatefulSet")
	}
	for _, item := range data.DaemonSets {
		addWorkload(item.Metadata.Namespace, item.Metadata.Name, "DaemonSet")
	}
	for _, item := range data.Jobs {
		addWorkload(item.Metadata.Namespace, item.Metadata.Name, "Job")
	}
	for _, item := range data.CronJobs {
		addWorkload(item.Metadata.Namespace, item.Metadata.Name, "CronJob")
	}

	replicaSetRefs := make(map[string]podWorkloadRef)
	for _, item := range data.ReplicaSets {
		for _, owner := range item.Metadata.OwnerReferences {
			if strings.EqualFold(owner.Kind, "Deployment") && owner.Name != "" {
				replicaSetRefs[podWorkloadKey(item.Metadata.Namespace, item.Metadata.Name)] = podWorkloadRef{Name: owner.Name, Type: "Deployment"}
				break
			}
		}
	}
	jobRefs := make(map[string]podWorkloadRef)
	for _, item := range data.Jobs {
		for _, owner := range item.Metadata.OwnerReferences {
			if strings.EqualFold(owner.Kind, "CronJob") && owner.Name != "" {
				jobRefs[podWorkloadKey(item.Metadata.Namespace, item.Metadata.Name)] = podWorkloadRef{Name: owner.Name, Type: "CronJob"}
				break
			}
		}
	}
	for _, pod := range data.Pods {
		for _, owner := range pod.Metadata.OwnerReferences {
			var ref podWorkloadRef
			switch {
			case strings.EqualFold(owner.Kind, "ReplicaSet"):
				ref = replicaSetRefs[podWorkloadKey(pod.Metadata.Namespace, owner.Name)]
			case strings.EqualFold(owner.Kind, "Job"):
				ref = jobRefs[podWorkloadKey(pod.Metadata.Namespace, owner.Name)]
				if ref.Name == "" {
					ref = podWorkloadRef{Name: owner.Name, Type: "Job"}
				}
			case strings.EqualFold(owner.Kind, "StatefulSet"), strings.EqualFold(owner.Kind, "DaemonSet"):
				ref = podWorkloadRef{Name: owner.Name, Type: owner.Kind}
			}
			if ref.Name != "" {
				refs[podWorkloadKey(pod.Metadata.Namespace, pod.Metadata.Name)] = ref
				break
			}
		}
	}
	assignBySelector := func(namespace, name, workloadType string, selector map[string]string) {
		for _, pod := range data.Pods {
			key := podWorkloadKey(namespace, pod.Metadata.Name)
			if pod.Metadata.Namespace == namespace && refs[key].Name == "" && len(selector) > 0 && matchLabels(pod.Metadata.Labels, selector) {
				refs[key] = podWorkloadRef{Name: name, Type: workloadType}
			}
		}
	}
	for _, item := range data.Deployments {
		assignBySelector(item.Metadata.Namespace, item.Metadata.Name, "Deployment", item.Spec.Selector.MatchLabels)
	}
	for _, item := range data.StatefulSet {
		assignBySelector(item.Metadata.Namespace, item.Metadata.Name, "StatefulSet", item.Spec.Selector.MatchLabels)
	}
	for _, item := range data.DaemonSets {
		assignBySelector(item.Metadata.Namespace, item.Metadata.Name, "DaemonSet", item.Spec.Selector.MatchLabels)
	}
	for _, item := range data.Jobs {
		selector := map[string]string{"job-name": item.Metadata.Name}
		if item.Spec.Selector != nil && len(item.Spec.Selector.MatchLabels) > 0 {
			selector = item.Spec.Selector.MatchLabels
		}
		assignBySelector(item.Metadata.Namespace, item.Metadata.Name, "Job", selector)
	}
	for _, item := range data.CronJobs {
		for _, pod := range data.Pods {
			if pod.Metadata.Namespace != item.Metadata.Namespace {
				continue
			}
			for _, owner := range pod.Metadata.OwnerReferences {
				if refs[podWorkloadKey(pod.Metadata.Namespace, pod.Metadata.Name)].Name == "" && strings.EqualFold(owner.Kind, "Job") && strings.HasPrefix(owner.Name, item.Metadata.Name+"-") {
					refs[podWorkloadKey(pod.Metadata.Namespace, pod.Metadata.Name)] = podWorkloadRef{Name: item.Metadata.Name, Type: "CronJob"}
				}
			}
		}
	}
	for _, pod := range data.Pods {
		key := podWorkloadKey(pod.Metadata.Namespace, pod.Metadata.Name)
		if refs[key].Name != "" {
			continue
		}
		// Some restricted clusters do not return ownerReferences. In that case, use Kubernetes
		// controller-generated Pod name prefixes as a fallback and prefer the longest matching workload name.
		for _, candidate := range workloadsByNamespace[pod.Metadata.Namespace] {
			if strings.HasPrefix(pod.Metadata.Name, candidate.Name+"-") && len(candidate.Name) > len(refs[key].Name) {
				refs[key] = candidate
			}
		}
	}
	return buildPodItemsWithRefs(data.Pods, refs)
}

func buildPodItems(pods []kubePod) []model.K8sPodItem {
	return buildPodItemsWithRefs(pods, nil)
}

func buildPodItemsWithRefs(pods []kubePod, refs map[string]podWorkloadRef) []model.K8sPodItem {
	items := make([]model.K8sPodItem, 0, len(pods))
	for _, pod := range pods {
		restarts := 0
		for _, status := range pod.Status.ContainerStatuses {
			restarts += status.RestartCount
		}

		workload := refs[podWorkloadKey(pod.Metadata.Namespace, pod.Metadata.Name)]
		if workload.Name == "" {
			for _, owner := range pod.Metadata.OwnerReferences {
				if owner.Name != "" && (owner.Kind == "StatefulSet" || owner.Kind == "DaemonSet" || owner.Kind == "Job") {
					workload = podWorkloadRef{Name: owner.Name, Type: owner.Kind}
					break
				}
			}
		}
		items = append(items, model.K8sPodItem{
			Name: pod.Metadata.Name, Namespace: pod.Metadata.Namespace, WorkloadName: workload.Name, WorkloadType: workload.Type,
			Status: fallbackText(pod.Status.Phase), Node: fallbackText(pod.Spec.NodeName), NodeIP: fallbackText(pod.Status.HostIP),
			Restarts: restarts, Age: humanizeAge(pod.Metadata.CreationTimestamp), IP: fallbackText(pod.Status.PodIP),
		})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Namespace == items[j].Namespace {
			return items[i].Name < items[j].Name
		}
		return items[i].Namespace < items[j].Namespace
	})
	return items
}

func buildWorkloadItems(data k8sFetchedData) []model.K8sWorkloadItem {
	items := make([]model.K8sWorkloadItem, 0, len(data.Deployments)+len(data.StatefulSet)+len(data.DaemonSets)+len(data.Jobs)+len(data.CronJobs))

	for _, item := range data.Deployments {
		replicas := intValue(item.Spec.Replicas)
		items = append(items, model.K8sWorkloadItem{
			Name:      item.Metadata.Name,
			Type:      "Deployment",
			Namespace: item.Metadata.Namespace,
			Ready:     fmt.Sprintf("%d/%d", item.Status.ReadyReplicas, replicas),
			Updated:   item.Status.UpdatedReplicas,
			Available: item.Status.AvailableReplicas,
			Age:       humanizeAge(item.Metadata.CreationTimestamp),
			Requests:  formatWorkloadResourceSummary(item.Spec.Template.Spec.Containers, true),
			Limits:    formatWorkloadResourceSummary(item.Spec.Template.Spec.Containers, false),
		})
	}

	for _, item := range data.StatefulSet {
		replicas := intValue(item.Spec.Replicas)
		items = append(items, model.K8sWorkloadItem{
			Name:      item.Metadata.Name,
			Type:      "StatefulSet",
			Namespace: item.Metadata.Namespace,
			Ready:     fmt.Sprintf("%d/%d", item.Status.ReadyReplicas, replicas),
			Updated:   item.Status.UpdatedReplicas,
			Available: item.Status.AvailableReplicas,
			Age:       humanizeAge(item.Metadata.CreationTimestamp),
			Requests:  formatWorkloadResourceSummary(item.Spec.Template.Spec.Containers, true),
			Limits:    formatWorkloadResourceSummary(item.Spec.Template.Spec.Containers, false),
		})
	}

	for _, item := range data.DaemonSets {
		items = append(items, model.K8sWorkloadItem{
			Name:      item.Metadata.Name,
			Type:      "DaemonSet",
			Namespace: item.Metadata.Namespace,
			Ready:     fmt.Sprintf("%d/%d", item.Status.NumberReady, item.Status.DesiredNumberScheduled),
			Updated:   item.Status.UpdatedNumberScheduled,
			Available: item.Status.NumberAvailable,
			Age:       humanizeAge(item.Metadata.CreationTimestamp),
			Requests:  formatWorkloadResourceSummary(item.Spec.Template.Spec.Containers, true),
			Limits:    formatWorkloadResourceSummary(item.Spec.Template.Spec.Containers, false),
		})
	}

	for _, item := range data.Jobs {
		total := intValue(item.Spec.Completions)
		if total == 0 {
			total = item.Status.Active + item.Status.Succeeded + item.Status.Failed
		}
		items = append(items, model.K8sWorkloadItem{
			Name:      item.Metadata.Name,
			Type:      "Job",
			Namespace: item.Metadata.Namespace,
			Ready:     fmt.Sprintf("%d/%d", item.Status.Succeeded, total),
			Updated:   item.Status.Active,
			Available: item.Status.Succeeded,
			Age:       humanizeAge(item.Metadata.CreationTimestamp),
			Requests:  formatWorkloadResourceSummary(item.Spec.Template.Spec.Containers, true),
			Limits:    formatWorkloadResourceSummary(item.Spec.Template.Spec.Containers, false),
		})
	}

	for _, item := range data.CronJobs {
		active := len(item.Status.Active)
		items = append(items, model.K8sWorkloadItem{
			Name:      item.Metadata.Name,
			Type:      "CronJob",
			Namespace: item.Metadata.Namespace,
			Ready:     cronJobReadyText(item.Spec.Suspend, active),
			Updated:   active,
			Available: active,
			Age:       humanizeAge(item.Metadata.CreationTimestamp),
			Requests:  formatWorkloadResourceSummary(item.Spec.JobTemplate.Spec.Template.Spec.Containers, true),
			Limits:    formatWorkloadResourceSummary(item.Spec.JobTemplate.Spec.Template.Spec.Containers, false),
		})
	}

	return items
}

func buildEndpointCounts(endpoints []kubeEndpoints) map[string]int {
	result := make(map[string]int, len(endpoints))
	for _, endpoint := range endpoints {
		key := endpoint.Metadata.Namespace + "/" + endpoint.Metadata.Name
		count := 0
		for _, subset := range endpoint.Subsets {
			count += len(subset.Addresses)
		}
		result[key] = count
	}
	return result
}

func formatServiceListPort(port int, nodePort int, protocol string) string {
	proto := strings.TrimSpace(protocol)
	if proto == "" {
		proto = "TCP"
	}
	if nodePort > 0 {
		return fmt.Sprintf("%d:%d/%s", port, nodePort, proto)
	}
	return fmt.Sprintf("%d/%s", port, proto)
}

func formatServiceDetailPort(port int, nodePort int, protocol string, targetPort string) string {
	base := formatServiceListPort(port, nodePort, protocol)
	targetPort = strings.TrimSpace(targetPort)
	if targetPort == "" || targetPort == strconv.Itoa(port) {
		return base
	}
	return fmt.Sprintf("%s -> %s", base, targetPort)
}

func serviceExternalIP(service kubeService) string {
	values := make([]string, 0, len(service.Spec.ExternalIPs)+len(service.Status.LoadBalancer.Ingress))
	for _, value := range service.Spec.ExternalIPs {
		value = strings.TrimSpace(value)
		if value != "" && value != "-" {
			values = append(values, value)
		}
	}
	for _, item := range service.Status.LoadBalancer.Ingress {
		value := firstNonEmpty(strings.TrimSpace(item.IP), strings.TrimSpace(item.Hostname))
		if value != "" && value != "-" {
			values = append(values, value)
		}
	}
	if len(values) == 0 {
		return "<none>"
	}
	return strings.Join(values, ", ")
}

func k8sWorkloadResourcePath(namespace string, workloadType string, workloadName string) (string, error) {
	// The UI lists Kubernetes kinds such as Deployment and StatefulSet, while API calls may use lowercase values.
	// Normalize to lowercase before selecting the resource path.
	switch strings.ToLower(strings.TrimSpace(workloadType)) {
	case "deployment":
		return fmt.Sprintf("/apis/apps/v1/namespaces/%s/deployments/%s", namespace, workloadName), nil
	case "statefulset":
		return fmt.Sprintf("/apis/apps/v1/namespaces/%s/statefulsets/%s", namespace, workloadName), nil
	case "daemonset":
		return fmt.Sprintf("/apis/apps/v1/namespaces/%s/daemonsets/%s", namespace, workloadName), nil
	case "job":
		return fmt.Sprintf("/apis/batch/v1/namespaces/%s/jobs/%s", namespace, workloadName), nil
	case "cronjob":
		return fmt.Sprintf("/apis/batch/v1/namespaces/%s/cronjobs/%s", namespace, workloadName), nil
	default:
		return "", errors.New("unsupported workload type")
	}
}

func extractWorkloadContainers(resource map[string]any) ([]map[string]any, error) {
	spec, ok := resource["spec"].(map[string]any)
	if !ok {
		return nil, errors.New("invalid workload spec")
	}
	template, ok := spec["template"].(map[string]any)
	if !ok {
		jobTemplate, jobTemplateOK := spec["jobTemplate"].(map[string]any)
		if !jobTemplateOK {
			return nil, errors.New("invalid workload template")
		}
		jobSpec, jobSpecOK := jobTemplate["spec"].(map[string]any)
		if !jobSpecOK {
			return nil, errors.New("invalid cronjob template")
		}
		template, ok = jobSpec["template"].(map[string]any)
		if !ok {
			return nil, errors.New("invalid cronjob pod template")
		}
	}
	templateSpec, ok := template["spec"].(map[string]any)
	if !ok {
		return nil, errors.New("invalid pod template spec")
	}
	rawContainers, ok := templateSpec["containers"].([]any)
	if !ok {
		return nil, errors.New("no containers found in workload")
	}
	containers := make([]map[string]any, 0, len(rawContainers))
	for _, item := range rawContainers {
		container, ok := item.(map[string]any)
		if ok {
			containers = append(containers, container)
		}
	}
	return containers, nil
}

func buildWorkloadEnvPatch(existing map[string]any, envItems []model.K8sEnvVarItem) ([]map[string]any, error) {
	desired := make(map[string]struct{}, len(envItems))
	patch := make([]map[string]any, 0, len(envItems))
	for _, item := range envItems {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			return nil, errors.New("environment variable name is required")
		}
		if _, exists := desired[name]; exists {
			return nil, fmt.Errorf("duplicate environment variable: %s", name)
		}
		desired[name] = struct{}{}
		entry := map[string]any{"name": name}
		if len(item.ValueFrom) > 0 {
			entry["valueFrom"] = item.ValueFrom
		} else {
			entry["value"] = item.Value
		}
		patch = append(patch, entry)
	}
	if existingEnv, ok := existing["env"].([]any); ok {
		for _, raw := range existingEnv {
			env, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			name, _ := env["name"].(string)
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			if _, keep := desired[name]; !keep {
				patch = append(patch, map[string]any{"name": name, "$patch": "delete"})
			}
		}
	}
	return patch, nil
}

func buildContainerEnvItems(envs []kubeEnvVar) []model.K8sEnvVarItem {
	items := make([]model.K8sEnvVarItem, 0, len(envs))
	for _, env := range envs {
		item := model.K8sEnvVarItem{Name: env.Name, Value: env.Value, ValueFrom: env.ValueFrom}
		item.Source = formatK8sEnvSource(env.ValueFrom)
		items = append(items, item)
	}
	return items
}

func formatK8sEnvSource(valueFrom map[string]any) string {
	if len(valueFrom) == 0 {
		return ""
	}
	for sourceType, raw := range valueFrom {
		source, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		name, _ := source["name"].(string)
		key, _ := source["key"].(string)
		if name != "" && key != "" {
			return fmt.Sprintf("%s: %s/%s", sourceType, name, key)
		}
		if name != "" {
			return fmt.Sprintf("%s: %s", sourceType, name)
		}
		return sourceType
	}
	return "Provided by Kubernetes reference"
}

func buildWorkloadImagePatchBody(containers []map[string]any) map[string]any {
	return map[string]any{
		"spec": map[string]any{
			"template": map[string]any{
				"spec": map[string]any{
					"containers": containers,
				},
			},
		},
	}
}

func buildWorkloadContainerPatchBody(workloadType string, containers []map[string]any) map[string]any {
	template := map[string]any{"spec": map[string]any{"containers": containers}}
	if strings.EqualFold(strings.TrimSpace(workloadType), "cronjob") {
		return map[string]any{"spec": map[string]any{"jobTemplate": map[string]any{"spec": template}}}
	}
	return map[string]any{"spec": template}
}

func formatWorkloadResourceSummary(containers []kubeContainer, requests bool) string {
	var cpuMilli, memoryBytes int64
	hasCPU, hasMemory := false, false
	for _, container := range containers {
		values := container.Resources.Limits
		if requests {
			values = container.Resources.Requests
		}
		if value := strings.TrimSpace(values["cpu"]); value != "" {
			cpuMilli += parseCPUToMilli(value)
			hasCPU = true
		}
		if value := strings.TrimSpace(values["memory"]); value != "" {
			memoryBytes += parseBytesQuantity(value)
			hasMemory = true
		}
	}
	parts := make([]string, 0, 2)
	if hasCPU {
		parts = append(parts, formatCPUMilli(cpuMilli))
	}
	if hasMemory {
		parts = append(parts, formatMemoryBytes(memoryBytes))
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, " / ")
}

func formatCPUMilli(value int64) string {
	if value >= 1000 && value%1000 == 0 {
		return fmt.Sprintf("%d cores", value/1000)
	}
	return fmt.Sprintf("%dm", value)
}

func formatMemoryBytes(value int64) string {
	if value >= 1024*1024*1024 {
		return fmt.Sprintf("%.1fGi", float64(value)/(1024*1024*1024))
	}
	return fmt.Sprintf("%.0fMi", float64(value)/(1024*1024))
}

func replaceImageVersion(image string, version string) string {
	trimmed := strings.TrimSpace(image)
	if trimmed == "" {
		return trimmed
	}

	base := trimmed
	if at := strings.Index(base, "@"); at >= 0 {
		base = base[:at]
	}

	lastSlash := strings.LastIndex(base, "/")
	lastColon := strings.LastIndex(base, ":")
	if lastColon > lastSlash {
		base = base[:lastColon]
	}

	return base + ":" + version
}

func anyToString(value any) string {
	switch item := value.(type) {
	case string:
		return item
	default:
		return fmt.Sprintf("%v", value)
	}
}
