// k8s_build_net.go — moved verbatim from k8s.go (Phase BCD, E5 seam #10).
package service

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"ops-admin/backend/model"
)

func buildAdvancedNetworkSection(
	gatewayAPIGateways []kubeGatewayAPI,
	httpRoutes []kubeHTTPRoute,
	services []kubeService,
) model.K8sAdvancedNetworkSection {
	result := model.K8sAdvancedNetworkSection{
		GatewayAPIGateways: make([]model.K8sIstioResourceItem, 0, len(gatewayAPIGateways)),
		HTTPRoutes:         make([]model.K8sIstioResourceItem, 0, len(httpRoutes)),
	}

	for _, item := range gatewayAPIGateways {
		result.GatewayAPIGateways = append(result.GatewayAPIGateways, model.K8sIstioResourceItem{
			Name:      item.Metadata.Name,
			Namespace: item.Metadata.Namespace,
			Kind:      "Gateway",
			Hosts:     joinAndLimit(collectGatewayAPIHosts(item), 3),
			Address:   resolveGatewayAPIAddress(item, services),
			Ports:     joinAndLimit(collectGatewayAPIPorts(item), 4),
			Target:    fallbackText(item.Spec.GatewayClassName),
			Age:       humanizeAge(item.Metadata.CreationTimestamp),
		})
	}

	for _, item := range httpRoutes {
		result.HTTPRoutes = append(result.HTTPRoutes, model.K8sIstioResourceItem{
			Name:      item.Metadata.Name,
			Namespace: item.Metadata.Namespace,
			Kind:      "HTTPRoute",
			Hosts:     joinAndLimit(uniqueNonEmptyStrings(item.Spec.Hostnames), 3),
			Gateways:  joinAndLimit(collectHTTPRouteParents(item), 3),
			Target:    joinAndLimit(collectHTTPRouteTargets(item), 3),
			Age:       humanizeAge(item.Metadata.CreationTimestamp),
		})
	}

	sort.Slice(result.GatewayAPIGateways, func(i, j int) bool {
		return compareIstioItems(result.GatewayAPIGateways[i], result.GatewayAPIGateways[j])
	})
	sort.Slice(result.HTTPRoutes, func(i, j int) bool { return compareIstioItems(result.HTTPRoutes[i], result.HTTPRoutes[j]) })

	return result
}

func compareIstioItems(left, right model.K8sIstioResourceItem) bool {
	if left.Namespace == right.Namespace {
		return left.Name < right.Name
	}
	return left.Namespace < right.Namespace
}

func joinAndLimit(values []string, limit int) string {
	values = uniqueNonEmptyStrings(values)
	if len(values) == 0 {
		return "-"
	}
	if limit > 0 && len(values) > limit {
		return strings.Join(values[:limit], ", ") + fmt.Sprintf(" +%d", len(values)-limit)
	}
	return strings.Join(values, ", ")
}

func uniqueNonEmptyStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || value == "-" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func formatIstioPort(number int, protocol string) string {
	if number <= 0 {
		return fallbackText(protocol)
	}
	proto := strings.TrimSpace(protocol)
	if proto == "" {
		proto = "TCP"
	}
	return fmt.Sprintf("%d/%s", number, proto)
}

func joinSelector(selector map[string]string) string {
	if len(selector) == 0 {
		return "-"
	}
	keys := make([]string, 0, len(selector))
	for key := range selector {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+selector[key])
	}
	return strings.Join(parts, ", ")
}

func collectVirtualServiceTargets(item kubeIstioVirtualService) []string {
	result := make([]string, 0)
	for _, httpRoute := range item.Spec.HTTP {
		for _, route := range httpRoute.Route {
			target := route.Destination.Host
			if route.Destination.Subset != "" {
				target += ":" + route.Destination.Subset
			}
			if route.Destination.Port.Number > 0 {
				target += ":" + strconv.Itoa(route.Destination.Port.Number)
			}
			result = append(result, target)
		}
	}
	for _, tcpRoute := range item.Spec.TCP {
		for _, route := range tcpRoute.Route {
			target := route.Destination.Host
			if route.Destination.Port.Number > 0 {
				target += ":" + strconv.Itoa(route.Destination.Port.Number)
			}
			result = append(result, target)
		}
	}
	return result
}

func buildVirtualServiceTrafficItems(item kubeIstioVirtualService) []model.K8sIstioTrafficRoute {
	httpIndex := firstVirtualServiceHTTPRouteIndex(item)
	if httpIndex < 0 {
		return nil
	}

	result := make([]model.K8sIstioTrafficRoute, 0, len(item.Spec.HTTP[httpIndex].Route))
	for index, route := range item.Spec.HTTP[httpIndex].Route {
		label := route.Destination.Host
		if route.Destination.Subset != "" {
			label += " / " + route.Destination.Subset
		}
		if route.Destination.Port.Number > 0 {
			label += ":" + strconv.Itoa(route.Destination.Port.Number)
		}
		result = append(result, model.K8sIstioTrafficRoute{
			Index:  index,
			Host:   route.Destination.Host,
			Subset: route.Destination.Subset,
			Port:   route.Destination.Port.Number,
			Weight: route.Weight,
			Label:  label,
		})
	}
	return result
}

func firstVirtualServiceHTTPRouteIndex(item kubeIstioVirtualService) int {
	for index, route := range item.Spec.HTTP {
		if len(route.Route) > 0 {
			return index
		}
	}
	return -1
}

func collectGatewayAPIHosts(item kubeGatewayAPI) []string {
	hosts := make([]string, 0, len(item.Spec.Listeners))
	for _, listener := range item.Spec.Listeners {
		hosts = append(hosts, firstNonEmpty(listener.Hostname, "*"))
	}
	return uniqueNonEmptyStrings(hosts)
}

func collectGatewayAPIPorts(item kubeGatewayAPI) []string {
	ports := make([]string, 0, len(item.Spec.Listeners))
	for _, listener := range item.Spec.Listeners {
		ports = append(ports, formatIstioPort(listener.Port, listener.Protocol))
	}
	return uniqueNonEmptyStrings(ports)
}

func collectGatewayAPIAddresses(item kubeGatewayAPI) []string {
	values := make([]string, 0, len(item.Status.Addresses))
	for _, address := range item.Status.Addresses {
		values = append(values, address.Value)
	}
	return uniqueNonEmptyStrings(values)
}

func resolveGatewayAPIAddress(item kubeGatewayAPI, services []kubeService) string {
	addresses := collectGatewayAPIAddresses(item)
	if len(addresses) > 0 {
		return joinAndLimit(addresses, 3)
	}

	candidates := []string{
		item.Metadata.Name,
		item.Metadata.Name + "-istio",
	}
	for _, service := range services {
		if service.Metadata.Namespace != item.Metadata.Namespace {
			continue
		}
		name := service.Metadata.Name
		matched := false
		for _, candidate := range candidates {
			if name == candidate || strings.HasPrefix(name, item.Metadata.Name+"-") {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		address := serviceExternalIP(service)
		if strings.TrimSpace(address) != "" && address != "<none>" && address != "-" {
			return address
		}
	}

	return "-"
}

func collectHTTPRouteParents(item kubeHTTPRoute) []string {
	values := make([]string, 0, len(item.Spec.ParentRefs))
	for _, ref := range item.Spec.ParentRefs {
		if strings.TrimSpace(ref.Namespace) != "" {
			values = append(values, ref.Namespace+"/"+ref.Name)
			continue
		}
		values = append(values, ref.Name)
	}
	return uniqueNonEmptyStrings(values)
}

func collectHTTPRouteTargets(item kubeHTTPRoute) []string {
	values := make([]string, 0)
	for _, rule := range item.Spec.Rules {
		values = append(values, collectHTTPRouteTargetsFromRule(rule)...)
	}
	return uniqueNonEmptyStrings(values)
}

func collectHTTPRouteTargetsFromRule(rule struct {
	Matches []struct {
		Path struct {
			Type  string `json:"type"`
			Value string `json:"value"`
		} `json:"path"`
	} `json:"matches"`
	BackendRefs []struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
		Port      int    `json:"port"`
		Weight    int    `json:"weight"`
	} `json:"backendRefs"`
}) []string {
	values := make([]string, 0, len(rule.BackendRefs))
	for _, backend := range rule.BackendRefs {
		target := backend.Name
		if strings.TrimSpace(backend.Namespace) != "" {
			target = backend.Namespace + "/" + target
		}
		if backend.Port > 0 {
			target += ":" + strconv.Itoa(backend.Port)
		}
		if backend.Weight > 0 {
			target += fmt.Sprintf(" (%d%%)", backend.Weight)
		}
		values = append(values, target)
	}
	return values
}

func buildHTTPRouteTrafficItems(item kubeHTTPRoute) []model.K8sIstioTrafficRoute {
	ruleIndex := firstHTTPRouteRuleIndex(item)
	if ruleIndex < 0 {
		return nil
	}
	result := make([]model.K8sIstioTrafficRoute, 0, len(item.Spec.Rules[ruleIndex].BackendRefs))
	for index, backend := range item.Spec.Rules[ruleIndex].BackendRefs {
		label := backend.Name
		if strings.TrimSpace(backend.Namespace) != "" {
			label = backend.Namespace + "/" + label
		}
		if backend.Port > 0 {
			label += ":" + strconv.Itoa(backend.Port)
		}
		result = append(result, model.K8sIstioTrafficRoute{
			Index:  index,
			Host:   backend.Name,
			Port:   backend.Port,
			Weight: backend.Weight,
			Label:  label,
		})
	}
	return result
}

func firstHTTPRouteRuleIndex(item kubeHTTPRoute) int {
	for index, rule := range item.Spec.Rules {
		if len(rule.BackendRefs) > 0 {
			return index
		}
	}
	return -1
}

func buildNetworkSection(services []kubeService, ingresses []kubeIngress, endpointCounts map[string]int) model.K8sNetworkSection {
	serviceItems := make([]model.K8sServiceItem, 0, len(services))
	for _, service := range services {
		ports := make([]string, 0, len(service.Spec.Ports))
		for _, port := range service.Spec.Ports {
			ports = append(ports, formatServiceListPort(port.Port, port.NodePort, port.Protocol))
		}

		key := service.Metadata.Namespace + "/" + service.Metadata.Name
		serviceItems = append(serviceItems, model.K8sServiceItem{
			Name:       service.Metadata.Name,
			Namespace:  service.Metadata.Namespace,
			Type:       serviceDisplayType(service),
			ClusterIP:  fallbackText(service.Spec.ClusterIP),
			ExternalIP: serviceExternalIP(service),
			Ports:      strings.Join(ports, ", "),
			Endpoints:  endpointCounts[key],
			Age:        humanizeAge(service.Metadata.CreationTimestamp),
		})
	}
	sort.Slice(serviceItems, func(i, j int) bool {
		if serviceItems[i].Namespace == serviceItems[j].Namespace {
			return serviceItems[i].Name < serviceItems[j].Name
		}
		return serviceItems[i].Namespace < serviceItems[j].Namespace
	})

	ingressItems := make([]model.K8sIngressItem, 0, len(ingresses))
	for _, ingress := range ingresses {
		hosts := make([]string, 0, len(ingress.Spec.Rules))
		for _, rule := range ingress.Spec.Rules {
			if strings.TrimSpace(rule.Host) != "" {
				hosts = append(hosts, rule.Host)
			}
		}

		address := "-"
		if len(ingress.Status.LoadBalancer.Ingress) > 0 {
			address = firstNonEmpty(ingress.Status.LoadBalancer.Ingress[0].IP, ingress.Status.LoadBalancer.Ingress[0].Hostname)
		}

		tls := "Disabled"
		if len(ingress.Spec.TLS) > 0 {
			tls = "Enabled"
		}

		ingressItems = append(ingressItems, model.K8sIngressItem{
			Name:      ingress.Metadata.Name,
			Namespace: ingress.Metadata.Namespace,
			Host:      fallbackText(strings.Join(hosts, ", ")),
			Address:   fallbackText(address),
			TLS:       tls,
			Age:       humanizeAge(ingress.Metadata.CreationTimestamp),
		})
	}
	sort.Slice(ingressItems, func(i, j int) bool {
		if ingressItems[i].Namespace == ingressItems[j].Namespace {
			return ingressItems[i].Name < ingressItems[j].Name
		}
		return ingressItems[i].Namespace < ingressItems[j].Namespace
	})

	return model.K8sNetworkSection{
		Services:  serviceItems,
		Ingresses: ingressItems,
	}
}

func serviceDisplayType(service kubeService) string {
	if strings.EqualFold(strings.TrimSpace(service.Spec.ClusterIP), "None") {
		return "Headless"
	}
	return fallbackText(service.Spec.Type)
}

func persistentVolumeSource(item kubePersistentVolume) (sourceType, path, nfsServer string) {
	if item.Spec.HostPath != nil {
		return "hostPath", fallbackText(item.Spec.HostPath.Path), "-"
	}
	if item.Spec.NFS != nil {
		return "NFS", fallbackText(item.Spec.NFS.Path), fallbackText(item.Spec.NFS.Server)
	}
	return "-", "-", "-"
}

const storageNamespaceScopeAnnotation = "ops-admin.io/namespace-scope"

// storageNamespaceScope records the platform-level PVC scope for a static PV.
// PersistentVolumes are cluster-scoped Kubernetes resources, so this annotation
// keeps an Ops Admin namespace restriction explicit.
func storageNamespaceScope(annotations map[string]string) string {
	if annotations != nil {
		if scope := strings.TrimSpace(annotations[storageNamespaceScopeAnnotation]); scope != "" {
			return scope
		}
	}
	return "Cluster-scoped"
}

func buildConfigStorageSection(configMaps []kubeConfigMap, secrets []kubeSecret, pvcs []kubePersistentVolumeClaim, pvs []kubePersistentVolume) model.K8sConfigStorageSection {
	configMapItems := make([]model.K8sConfigMapItem, 0, len(configMaps))
	for _, item := range configMaps {
		configMapItems = append(configMapItems, model.K8sConfigMapItem{
			Name:      item.Metadata.Name,
			Namespace: item.Metadata.Namespace,
			Keys:      len(item.Data) + len(item.Binary),
			Age:       humanizeAge(item.Metadata.CreationTimestamp),
		})
	}
	sort.Slice(configMapItems, func(i, j int) bool {
		if configMapItems[i].Namespace == configMapItems[j].Namespace {
			return configMapItems[i].Name < configMapItems[j].Name
		}
		return configMapItems[i].Namespace < configMapItems[j].Namespace
	})

	secretItems := make([]model.K8sSecretItem, 0, len(secrets))
	for _, item := range secrets {
		secretItems = append(secretItems, model.K8sSecretItem{
			Name:      item.Metadata.Name,
			Namespace: item.Metadata.Namespace,
			Type:      fallbackText(item.Type),
			Age:       humanizeAge(item.Metadata.CreationTimestamp),
		})
	}
	sort.Slice(secretItems, func(i, j int) bool {
		if secretItems[i].Namespace == secretItems[j].Namespace {
			return secretItems[i].Name < secretItems[j].Name
		}
		return secretItems[i].Namespace < secretItems[j].Namespace
	})

	storageItems := make([]model.K8sStorageItem, 0, len(pvcs)+len(pvs))
	for _, item := range pvcs {
		storageItems = append(storageItems, model.K8sStorageItem{
			Name:      item.Metadata.Name,
			Kind:      "PVC",
			Namespace: fallbackText(item.Metadata.Namespace),
			Status:    fallbackText(item.Status.Phase),
			// Prefer requested capacity over the bound PV capacity to avoid displaying PV capacity as the PVC request.
			Capacity:     fallbackText(firstNonEmpty(item.Spec.Resources.Requests["storage"], item.Status.Capacity["storage"])),
			StorageClass: fallbackText(item.Spec.StorageClassName),
			AccessModes:  strings.Join(item.Spec.AccessModes, ", "),
		})
	}
	for _, item := range pvs {
		sourceType, path, nfsServer := persistentVolumeSource(item)
		storageItems = append(storageItems, model.K8sStorageItem{
			Name:           item.Metadata.Name,
			Kind:           "PV",
			Namespace:      "Cluster-scoped",
			NamespaceScope: storageNamespaceScope(item.Metadata.Annotations),
			Status:         fallbackText(item.Status.Phase),
			Capacity:       fallbackText(firstNonEmpty(item.Status.Capacity["storage"], item.Spec.Capacity["storage"])),
			StorageClass:   fallbackText(item.Spec.StorageClassName),
			SourceType:     sourceType,
			Path:           path,
			NFSServer:      nfsServer,
			AccessModes:    strings.Join(item.Spec.AccessModes, ", "),
			ReclaimPolicy:  fallbackText(item.Spec.PersistentVolumeReclaimPolicy),
		})
	}
	sort.Slice(storageItems, func(i, j int) bool {
		if storageItems[i].Kind == storageItems[j].Kind {
			if storageItems[i].Namespace == storageItems[j].Namespace {
				return storageItems[i].Name < storageItems[j].Name
			}
			return storageItems[i].Namespace < storageItems[j].Namespace
		}
		return storageItems[i].Kind < storageItems[j].Kind
	})

	return model.K8sConfigStorageSection{
		ConfigMaps: configMapItems,
		Secrets:    secretItems,
		Storage:    storageItems,
	}
}

func calculateK8sAggregateMetrics(nodes []kubeNode, pods []kubePod) k8sAggregateMetrics {
	var metrics k8sAggregateMetrics

	for _, node := range nodes {
		metrics.TotalAllocCPUMilli += parseCPUToMilli(node.Status.Allocatable["cpu"])
		metrics.TotalAllocMemoryBytes += parseBytesQuantity(node.Status.Allocatable["memory"])
		if nodeReadyStatus(node) != "Ready" || node.Spec.Unschedulable {
			metrics.AlertCount++
		}
	}

	for _, pod := range pods {
		switch strings.ToLower(pod.Status.Phase) {
		case "failed", "pending", "unknown":
			metrics.AlertCount++
		}
		for _, container := range pod.Spec.Containers {
			metrics.TotalReqCPUMilli += parseCPUToMilli(container.Resources.Requests["cpu"])
			metrics.TotalReqMemoryBytes += parseBytesQuantity(container.Resources.Requests["memory"])
		}
	}

	return metrics
}

func toK8sClusterView(cluster model.K8sCluster) model.K8sClusterView {
	return model.K8sClusterView{
		ID:                    cluster.ID,
		Name:                  cluster.Name,
		Status:                cluster.Status,
		StatusText:            k8sStatusText(cluster.Status),
		APIServer:             cluster.APIServer,
		Version:               cluster.Version,
		NodeCount:             cluster.NodeCount,
		Env:                   cluster.Env,
		Tags:                  cluster.Tags,
		ConnectionMode:        normalizeConnectionMode(cluster.ConnectionMode),
		GatewayID:             cluster.GatewayID,
		GatewayName:           cluster.Gateway.Name,
		MonitorDatasourceID:   cluster.MonitorDatasourceID,
		MonitorDatasourceName: cluster.MonitorDatasource.Name,
		Description:           cluster.Description,
		LastSyncAt:            cluster.LastSyncAt,
		CreatedAt:             cluster.CreatedAt,
		UpdatedAt:             cluster.UpdatedAt,
	}
}

func validateK8sClusterPayload(cluster model.K8sCluster) error {
	if cluster.Name == "" {
		return errors.New("cluster name is required")
	}
	if cluster.KubeConfig == "" {
		return errors.New("kubeconfig is required")
	}
	if cluster.Env == "" {
		return errors.New("select an environment")
	}
	if err := validateGatewaySelection(cluster.ConnectionMode, cluster.GatewayID); err != nil {
		return err
	}
	return nil
}
