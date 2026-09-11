// k8s_detail.go — moved verbatim from k8s.go (Phase BCD, E5 seam #7).
package service

import (
	"encoding/base64"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"ops-admin/backend/model"
)

func (s *Service) GetK8sServiceDetail(clusterID uint, namespace string, serviceName string) (model.K8sServiceDetail, error) {
	_, runtime, client, err := s.k8sClientForCluster(clusterID)
	if err != nil {
		return model.K8sServiceDetail{}, err
	}

	var service kubeService
	if err := k8sGetJSON(client, runtime, fmt.Sprintf("/api/v1/namespaces/%s/services/%s", namespace, serviceName), &service); err != nil {
		return model.K8sServiceDetail{}, errors.New(k8sClusterConnectError)
	}

	endpointCount := 0
	var endpoints kubeEndpoints
	if err := k8sGetJSON(client, runtime, fmt.Sprintf("/api/v1/namespaces/%s/endpoints/%s", namespace, serviceName), &endpoints); err == nil {
		for _, subset := range endpoints.Subsets {
			endpointCount += len(subset.Addresses)
		}
	}

	ports := make([]model.K8sKVTextItem, 0, len(service.Spec.Ports))
	portSpecs := make([]model.K8sServicePort, 0, len(service.Spec.Ports))
	for _, port := range service.Spec.Ports {
		label := strconv.Itoa(port.Port)
		if strings.TrimSpace(port.Name) != "" {
			label = port.Name
		}
		target := stringifyTargetPort(port.TargetPort)
		if target == "" {
			target = strconv.Itoa(port.Port)
		}
		ports = append(ports, model.K8sKVTextItem{
			Label: label,
			Value: formatServiceDetailPort(port.Port, port.NodePort, port.Protocol, target),
		})
		protocol := strings.TrimSpace(port.Protocol)
		if protocol == "" {
			protocol = "TCP"
		}
		portSpecs = append(portSpecs, model.K8sServicePort{Name: port.Name, Protocol: protocol, Port: port.Port, TargetPort: target, NodePort: port.NodePort})
	}

	return model.K8sServiceDetail{
		Name:         service.Metadata.Name,
		Namespace:    service.Metadata.Namespace,
		Type:         serviceDisplayType(service),
		ClusterIP:    fallbackText(service.Spec.ClusterIP),
		ExternalIP:   serviceExternalIP(service),
		ExternalName: service.Spec.ExternalName,
		Ports:        ports,
		PortSpecs:    portSpecs,
		Selector:     service.Spec.Selector,
		Labels:       service.Metadata.Labels,
		Annotations:  service.Metadata.Annotations,
		Endpoints:    endpointCount,
		Age:          humanizeAge(service.Metadata.CreationTimestamp),
		YAML:         marshalK8sYAML(service),
	}, nil
}

func serviceTargetPort(value string, fallback int) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	if port, err := strconv.Atoi(value); err == nil {
		return port
	}
	return value
}

func (s *Service) GetK8sIstioResourceDetail(clusterID uint, resourceType string, namespace string, name string) (model.K8sIstioResourceDetail, error) {
	_, runtime, client, err := s.k8sClientForCluster(clusterID)
	if err != nil {
		return model.K8sIstioResourceDetail{}, err
	}

	resourceType = strings.ToLower(strings.TrimSpace(resourceType))
	switch resourceType {
	case "gatewayapi":
		var item kubeGatewayAPI
		if err := k8sGetGatewayAPIJSON(client, runtime, "gateways", namespace, name, &item); err != nil {
			return model.K8sIstioResourceDetail{}, errors.New(k8sClusterConnectError)
		}
		summary := []model.K8sKVTextItem{
			{Label: "GatewayClass", Value: fallbackText(item.Spec.GatewayClassName)},
			{Label: "Hosts", Value: joinAndLimit(collectGatewayAPIHosts(item), 0)},
			{Label: "Address", Value: joinAndLimit(collectGatewayAPIAddresses(item), 0)},
			{Label: "Ports", Value: joinAndLimit(collectGatewayAPIPorts(item), 0)},
		}
		items := make([]model.K8sKVTextItem, 0, len(item.Spec.Listeners))
		for _, listener := range item.Spec.Listeners {
			label := firstNonEmpty(listener.Name, formatIstioPort(listener.Port, listener.Protocol))
			items = append(items, model.K8sKVTextItem{
				Label: label,
				Value: firstNonEmpty(listener.Hostname, "*"),
			})
		}
		return model.K8sIstioResourceDetail{
			Name:        item.Metadata.Name,
			Namespace:   item.Metadata.Namespace,
			Kind:        "Gateway",
			Labels:      item.Metadata.Labels,
			Annotations: item.Metadata.Annotations,
			Summary:     summary,
			Items:       items,
			Age:         humanizeAge(item.Metadata.CreationTimestamp),
			YAML:        marshalK8sYAML(item),
		}, nil
	case "httproute":
		var item kubeHTTPRoute
		if err := k8sGetGatewayAPIJSON(client, runtime, "httproutes", namespace, name, &item); err != nil {
			return model.K8sIstioResourceDetail{}, errors.New(k8sClusterConnectError)
		}
		summary := []model.K8sKVTextItem{
			{Label: "Hosts", Value: joinAndLimit(item.Spec.Hostnames, 0)},
			{Label: "Gateways", Value: joinAndLimit(collectHTTPRouteParents(item), 0)},
			{Label: "Targets", Value: joinAndLimit(collectHTTPRouteTargets(item), 0)},
		}
		items := make([]model.K8sKVTextItem, 0, len(item.Spec.Rules))
		for _, rule := range item.Spec.Rules {
			matchText := "/"
			if len(rule.Matches) > 0 {
				matchParts := make([]string, 0, len(rule.Matches))
				for _, match := range rule.Matches {
					if strings.TrimSpace(match.Path.Value) != "" {
						matchParts = append(matchParts, match.Path.Value)
					}
				}
				if len(matchParts) > 0 {
					matchText = strings.Join(matchParts, ", ")
				}
			}
			items = append(items, model.K8sKVTextItem{
				Label: matchText,
				Value: joinAndLimit(collectHTTPRouteTargetsFromRule(rule), 0),
			})
		}
		return model.K8sIstioResourceDetail{
			Name:        item.Metadata.Name,
			Namespace:   item.Metadata.Namespace,
			Kind:        "HTTPRoute",
			Labels:      item.Metadata.Labels,
			Annotations: item.Metadata.Annotations,
			Summary:     summary,
			Items:       items,
			Traffic:     buildHTTPRouteTrafficItems(item),
			Age:         humanizeAge(item.Metadata.CreationTimestamp),
			YAML:        marshalK8sYAML(item),
		}, nil
	case "gateway":
		var item kubeIstioGateway
		if err := k8sGetIstioJSON(client, runtime, "gateways", namespace, name, &item); err != nil {
			return model.K8sIstioResourceDetail{}, errors.New(k8sClusterConnectError)
		}
		summary := []model.K8sKVTextItem{
			{Label: "Selector", Value: joinSelector(item.Spec.Selector)},
			{Label: "Hosts", Value: joinAndLimit(flattenGatewayHosts(item), 0)},
			{Label: "Ports", Value: joinAndLimit(flattenGatewayPorts(item), 0)},
		}
		items := make([]model.K8sKVTextItem, 0, len(item.Spec.Servers))
		for _, server := range item.Spec.Servers {
			items = append(items, model.K8sKVTextItem{
				Label: formatIstioPort(server.Port.Number, server.Port.Protocol),
				Value: joinAndLimit(server.Hosts, 0),
			})
		}
		return model.K8sIstioResourceDetail{
			Name:        item.Metadata.Name,
			Namespace:   item.Metadata.Namespace,
			Kind:        "Gateway",
			Labels:      item.Metadata.Labels,
			Annotations: item.Metadata.Annotations,
			Summary:     summary,
			Items:       items,
			Age:         humanizeAge(item.Metadata.CreationTimestamp),
			YAML:        marshalK8sYAML(item),
		}, nil
	case "virtualservice":
		var item kubeIstioVirtualService
		if err := k8sGetIstioJSON(client, runtime, "virtualservices", namespace, name, &item); err != nil {
			return model.K8sIstioResourceDetail{}, errors.New(k8sClusterConnectError)
		}
		summary := []model.K8sKVTextItem{
			{Label: "Hosts", Value: joinAndLimit(item.Spec.Hosts, 0)},
			{Label: "Gateways", Value: joinAndLimit(item.Spec.Gateways, 0)},
			{Label: "Targets", Value: joinAndLimit(collectVirtualServiceTargets(item), 0)},
		}
		items := make([]model.K8sKVTextItem, 0)
		for _, httpRoute := range item.Spec.HTTP {
			routeTarget := make([]string, 0, len(httpRoute.Route))
			for _, route := range httpRoute.Route {
				target := route.Destination.Host
				if route.Destination.Subset != "" {
					target += ":" + route.Destination.Subset
				}
				if route.Destination.Port.Number > 0 {
					target += ":" + strconv.Itoa(route.Destination.Port.Number)
				}
				routeTarget = append(routeTarget, target)
			}
			matchText := "/"
			if len(httpRoute.Match) > 0 {
				matchParts := make([]string, 0, len(httpRoute.Match))
				for _, match := range httpRoute.Match {
					value := firstNonEmpty(match.URI.Exact, match.URI.Prefix)
					if value != "" {
						matchParts = append(matchParts, value)
					}
				}
				if len(matchParts) > 0 {
					matchText = strings.Join(matchParts, ", ")
				}
			}
			items = append(items, model.K8sKVTextItem{
				Label: matchText,
				Value: joinAndLimit(routeTarget, 0),
			})
		}
		return model.K8sIstioResourceDetail{
			Name:        item.Metadata.Name,
			Namespace:   item.Metadata.Namespace,
			Kind:        "VirtualService",
			Labels:      item.Metadata.Labels,
			Annotations: item.Metadata.Annotations,
			Summary:     summary,
			Items:       items,
			Traffic:     buildVirtualServiceTrafficItems(item),
			Age:         humanizeAge(item.Metadata.CreationTimestamp),
			YAML:        marshalK8sYAML(item),
		}, nil
	case "destinationrule":
		var item kubeIstioDestinationRule
		if err := k8sGetIstioJSON(client, runtime, "destinationrules", namespace, name, &item); err != nil {
			return model.K8sIstioResourceDetail{}, errors.New(k8sClusterConnectError)
		}
		summary := []model.K8sKVTextItem{
			{Label: "Host", Value: fallbackText(item.Spec.Host)},
			{Label: "Subsets", Value: strconv.Itoa(len(item.Spec.Subsets))},
		}
		items := make([]model.K8sKVTextItem, 0, len(item.Spec.Subsets))
		for _, subset := range item.Spec.Subsets {
			items = append(items, model.K8sKVTextItem{
				Label: subset.Name,
				Value: "subset",
			})
		}
		return model.K8sIstioResourceDetail{
			Name:        item.Metadata.Name,
			Namespace:   item.Metadata.Namespace,
			Kind:        "DestinationRule",
			Labels:      item.Metadata.Labels,
			Annotations: item.Metadata.Annotations,
			Summary:     summary,
			Items:       items,
			Age:         humanizeAge(item.Metadata.CreationTimestamp),
			YAML:        marshalK8sYAML(item),
		}, nil
	case "serviceentry":
		var item kubeIstioServiceEntry
		if err := k8sGetIstioJSON(client, runtime, "serviceentries", namespace, name, &item); err != nil {
			return model.K8sIstioResourceDetail{}, errors.New(k8sClusterConnectError)
		}
		summary := []model.K8sKVTextItem{
			{Label: "Hosts", Value: joinAndLimit(item.Spec.Hosts, 0)},
			{Label: "Addresses", Value: joinAndLimit(item.Spec.Addresses, 0)},
			{Label: "Resolution", Value: fallbackText(item.Spec.Resolution)},
		}
		items := make([]model.K8sKVTextItem, 0, len(item.Spec.Ports))
		for _, port := range item.Spec.Ports {
			label := firstNonEmpty(port.Name, strconv.Itoa(port.Number))
			items = append(items, model.K8sKVTextItem{
				Label: label,
				Value: formatIstioPort(port.Number, port.Protocol),
			})
		}
		return model.K8sIstioResourceDetail{
			Name:        item.Metadata.Name,
			Namespace:   item.Metadata.Namespace,
			Kind:        "ServiceEntry",
			Labels:      item.Metadata.Labels,
			Annotations: item.Metadata.Annotations,
			Summary:     summary,
			Items:       items,
			Age:         humanizeAge(item.Metadata.CreationTimestamp),
			YAML:        marshalK8sYAML(item),
		}, nil
	default:
		return model.K8sIstioResourceDetail{}, errors.New("unsupported istio resource type")
	}
}

func (s *Service) GetK8sIngressDetail(clusterID uint, namespace string, ingressName string) (model.K8sIngressDetail, error) {
	_, runtime, client, err := s.k8sClientForCluster(clusterID)
	if err != nil {
		return model.K8sIngressDetail{}, err
	}

	var ingress kubeIngress
	if err := k8sGetJSON(client, runtime, fmt.Sprintf("/apis/networking.k8s.io/v1/namespaces/%s/ingresses/%s", namespace, ingressName), &ingress); err != nil {
		return model.K8sIngressDetail{}, errors.New(k8sClusterConnectError)
	}

	address := "-"
	if len(ingress.Status.LoadBalancer.Ingress) > 0 {
		address = firstNonEmpty(ingress.Status.LoadBalancer.Ingress[0].IP, ingress.Status.LoadBalancer.Ingress[0].Hostname)
	}

	tls := "Disabled"
	if len(ingress.Spec.TLS) > 0 {
		tls = "Enabled"
	}

	hosts := make([]string, 0, len(ingress.Spec.Rules))
	rules := make([]model.K8sKVTextItem, 0)
	for _, rule := range ingress.Spec.Rules {
		if strings.TrimSpace(rule.Host) != "" {
			hosts = append(hosts, rule.Host)
		}
		if len(rule.HTTP.Paths) == 0 {
			rules = append(rules, model.K8sKVTextItem{
				Label: fallbackText(rule.Host),
				Value: "/",
			})
			continue
		}
		for _, path := range rule.HTTP.Paths {
			backendTarget := path.Backend.Service.Name
			portText := firstNonEmpty(path.Backend.Service.Port.Name, strconv.Itoa(path.Backend.Service.Port.Number))
			if backendTarget != "" && portText != "" && portText != "0" {
				backendTarget = backendTarget + ":" + portText
			}
			rules = append(rules, model.K8sKVTextItem{
				Label: fmt.Sprintf("%s %s", fallbackText(rule.Host), fallbackText(path.Path)),
				Value: fallbackText(backendTarget),
			})
		}
	}

	return model.K8sIngressDetail{
		Name:        ingress.Metadata.Name,
		Namespace:   ingress.Metadata.Namespace,
		Host:        fallbackText(strings.Join(hosts, ", ")),
		Address:     fallbackText(address),
		TLS:         tls,
		ClassName:   fallbackText(ingress.Spec.IngressClassName),
		Labels:      ingress.Metadata.Labels,
		Annotations: ingress.Metadata.Annotations,
		Rules:       rules,
		Age:         humanizeAge(ingress.Metadata.CreationTimestamp),
		YAML:        marshalK8sYAML(ingress),
	}, nil
}

func (s *Service) GetK8sConfigMapDetail(clusterID uint, namespace string, configMapName string) (model.K8sConfigMapDetail, error) {
	_, runtime, client, err := s.k8sClientForCluster(clusterID)
	if err != nil {
		return model.K8sConfigMapDetail{}, err
	}

	var item kubeConfigMap
	if err := k8sGetJSON(client, runtime, fmt.Sprintf("/api/v1/namespaces/%s/configmaps/%s", namespace, configMapName), &item); err != nil {
		return model.K8sConfigMapDetail{}, errors.New(k8sClusterConnectError)
	}

	keys := make([]model.K8sKVTextItem, 0, len(item.Data)+len(item.Binary))
	for key, value := range item.Data {
		keys = append(keys, model.K8sKVTextItem{Label: key, Value: value})
	}
	for key, value := range item.Binary {
		keys = append(keys, model.K8sKVTextItem{Label: key, Value: value})
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].Label < keys[j].Label })

	return model.K8sConfigMapDetail{
		Name:        item.Metadata.Name,
		Namespace:   item.Metadata.Namespace,
		Keys:        keys,
		Labels:      item.Metadata.Labels,
		Annotations: item.Metadata.Annotations,
		Age:         humanizeAge(item.Metadata.CreationTimestamp),
		YAML:        marshalK8sYAML(item),
	}, nil
}

func (s *Service) GetK8sSecretDetail(clusterID uint, namespace string, secretName string) (model.K8sSecretDetail, error) {
	_, runtime, client, err := s.k8sClientForCluster(clusterID)
	if err != nil {
		return model.K8sSecretDetail{}, err
	}

	var item kubeSecret
	if err := k8sGetJSON(client, runtime, fmt.Sprintf("/api/v1/namespaces/%s/secrets/%s", namespace, secretName), &item); err != nil {
		return model.K8sSecretDetail{}, errors.New(k8sClusterConnectError)
	}

	keys := make([]model.K8sKVTextItem, 0, len(item.Data))
	for key, value := range item.Data {
		decoded, err := base64.StdEncoding.DecodeString(value)
		if err != nil {
			decoded = []byte(value)
		}
		keys = append(keys, model.K8sKVTextItem{Label: key, Value: string(decoded), Sensitive: true})
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].Label < keys[j].Label })

	return model.K8sSecretDetail{
		Name:        item.Metadata.Name,
		Namespace:   item.Metadata.Namespace,
		Type:        fallbackText(item.Type),
		Keys:        keys,
		Labels:      item.Metadata.Labels,
		Annotations: item.Metadata.Annotations,
		Age:         humanizeAge(item.Metadata.CreationTimestamp),
		YAML:        marshalK8sYAML(item),
	}, nil
}

func (s *Service) GetK8sStorageDetail(clusterID uint, kind string, namespace string, name string) (model.K8sStorageDetail, error) {
	_, runtime, client, err := s.k8sClientForCluster(clusterID)
	if err != nil {
		return model.K8sStorageDetail{}, err
	}

	switch strings.ToUpper(strings.TrimSpace(kind)) {
	case "PVC":
		var item kubePersistentVolumeClaim
		if strings.TrimSpace(namespace) == "" {
			return model.K8sStorageDetail{}, errors.New("namespace is required for pvc")
		}
		if err := k8sGetJSON(client, runtime, fmt.Sprintf("/api/v1/namespaces/%s/persistentvolumeclaims/%s", namespace, name), &item); err != nil {
			return model.K8sStorageDetail{}, errors.New(k8sClusterConnectError)
		}
		return model.K8sStorageDetail{
			Name:      item.Metadata.Name,
			Kind:      "PVC",
			Namespace: fallbackText(item.Metadata.Namespace),
			Status:    fallbackText(item.Status.Phase),
			// PVC lists and details should display the requested capacity declared by the user; after binding, status.capacity
			// represents the actual PV capacity and may be larger than the PVC request.
			Capacity:     fallbackText(firstNonEmpty(item.Spec.Resources.Requests["storage"], item.Status.Capacity["storage"])),
			StorageClass: fallbackText(item.Spec.StorageClassName),
			AccessModes:  strings.Join(item.Spec.AccessModes, ", "),
			Labels:       item.Metadata.Labels,
			Annotations:  item.Metadata.Annotations,
			Age:          humanizeAge(item.Metadata.CreationTimestamp),
			YAML:         marshalK8sYAML(item),
		}, nil
	case "PV":
		var item kubePersistentVolume
		if err := k8sGetJSON(client, runtime, "/api/v1/persistentvolumes/"+name, &item); err != nil {
			return model.K8sStorageDetail{}, errors.New(k8sClusterConnectError)
		}
		sourceType, path, nfsServer := persistentVolumeSource(item)
		return model.K8sStorageDetail{
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
			Labels:         item.Metadata.Labels,
			Annotations:    item.Metadata.Annotations,
			Age:            humanizeAge(item.Metadata.CreationTimestamp),
			YAML:           marshalK8sYAML(item),
		}, nil
	default:
		return model.K8sStorageDetail{}, errors.New("unsupported storage kind")
	}
}
