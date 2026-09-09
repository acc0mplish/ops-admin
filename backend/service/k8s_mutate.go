// k8s_mutate.go — moved verbatim from k8s.go (Phase BCD, E5 seam #5).
package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"ops-admin/backend/model"

	ksyaml "sigs.k8s.io/yaml"
)

func (s *Service) UpdateK8sResourceYAML(payload model.K8sResourceYAMLPayload) (map[string]any, error) {
	if payload.ClusterID == 0 || Trimmed(payload.ResourceType) == "" || Trimmed(payload.YAML) == "" {
		return nil, errors.New("invalid yaml payload")
	}

	_, runtime, client, err := s.k8sClientForCluster(payload.ClusterID)
	if err != nil {
		return nil, err
	}

	body, err := ksyaml.YAMLToJSON([]byte(payload.YAML))
	if err != nil {
		return nil, errors.New("invalid yaml content")
	}

	paths, err := buildK8sYAMLResourcePaths(payload)
	if err != nil {
		return nil, err
	}
	// Kubernetes PUT requires metadata.resourceVersion. Form editors submit a
	// concise manifest, so retrieve the current version server-side.
	var submitted map[string]any
	if err := json.Unmarshal(body, &submitted); err != nil {
		return nil, errors.New("invalid yaml content")
	}
	var updateErr error
	for _, path := range paths {
		var current map[string]any
		if err := k8sGetJSONWithQuery(client, runtime, path, nil, &current); err != nil {
			updateErr = err
			if isK8sNotFoundError(updateErr) {
				continue
			}
			break
		}
		if metadata, ok := current["metadata"].(map[string]any); ok {
			if version, ok := metadata["resourceVersion"].(string); ok && version != "" {
				if submittedMetadata, ok := submitted["metadata"].(map[string]any); ok {
					submittedMetadata["resourceVersion"] = version
				}
			}
		}
		requestBody, marshalErr := json.Marshal(submitted)
		if marshalErr != nil {
			return nil, errors.New("invalid yaml content")
		}
		updateErr = k8sDoJSON(client, runtime, http.MethodPut, path, nil, requestBody, "application/json", nil)
		if updateErr == nil {
			break
		}
		if !isK8sNotFoundError(updateErr) {
			break
		}
	}
	if updateErr != nil {
		return nil, friendlyK8sYAMLError(payload, updateErr)
	}
	return map[string]any{
		"resourceType": payload.ResourceType,
		"namespace":    payload.Namespace,
		"name":         payload.Name,
		"workloadType": payload.WorkloadType,
		"updated":      true,
	}, nil
}

func (s *Service) CreateK8sResourceYAML(payload model.K8sResourceYAMLPayload) (map[string]any, error) {
	if payload.ClusterID == 0 || Trimmed(payload.ResourceType) == "" || Trimmed(payload.YAML) == "" {
		return nil, errors.New("invalid yaml payload")
	}

	_, runtime, client, err := s.k8sClientForCluster(payload.ClusterID)
	if err != nil {
		return nil, err
	}

	body, err := ksyaml.YAMLToJSON([]byte(payload.YAML))
	if err != nil {
		return nil, errors.New("invalid yaml content")
	}

	manifest, err := parseK8sManifestIdentity(body)
	if err != nil {
		return nil, err
	}
	if Trimmed(payload.Name) == "" {
		payload.Name = manifest.Metadata.Name
	}
	if Trimmed(payload.Namespace) == "" {
		payload.Namespace = manifest.Metadata.Namespace
	}

	paths, err := buildK8sCreateResourcePaths(payload, manifest)
	if err != nil {
		return nil, err
	}
	if err := k8sDoJSONAnyPath(client, runtime, http.MethodPost, paths, nil, body, "application/json", nil); err != nil {
		return nil, friendlyK8sYAMLError(payload, err)
	}
	return map[string]any{
		"resourceType": payload.ResourceType,
		"namespace":    payload.Namespace,
		"name":         firstNonEmpty(payload.Name, manifest.Metadata.Name),
		"created":      true,
	}, nil
}

func (s *Service) DeleteK8sResource(payload model.K8sResourceDeletePayload) (map[string]any, error) {
	if payload.ClusterID == 0 || Trimmed(payload.ResourceType) == "" || Trimmed(payload.Name) == "" {
		return nil, errors.New("invalid delete payload")
	}

	_, runtime, client, err := s.k8sClientForCluster(payload.ClusterID)
	if err != nil {
		return nil, err
	}

	paths, err := buildK8sDeleteResourcePaths(payload)
	if err != nil {
		return nil, err
	}
	if err := k8sDoJSONAnyPath(client, runtime, http.MethodDelete, paths, nil, nil, "application/json", nil); err != nil {
		if isK8sNotFoundError(err) {
			return nil, errors.New("resource not found")
		}
		return nil, err
	}

	return map[string]any{
		"resourceType": payload.ResourceType,
		"namespace":    payload.Namespace,
		"name":         payload.Name,
		"deleted":      true,
	}, nil
}

func (s *Service) UpdateK8sIstioTraffic(payload model.K8sIstioTrafficPayload) (map[string]any, error) {
	if payload.ClusterID == 0 || Trimmed(payload.Namespace) == "" || Trimmed(payload.Name) == "" || len(payload.Routes) == 0 {
		return nil, errors.New("invalid istio traffic payload")
	}

	_, runtime, client, err := s.k8sClientForCluster(payload.ClusterID)
	if err != nil {
		return nil, err
	}

	var item kubeIstioVirtualService
	if err := k8sGetIstioJSON(client, runtime, "virtualservices", payload.Namespace, payload.Name, &item); err != nil {
		return nil, errors.New(k8sClusterConnectError)
	}

	httpIndex := firstVirtualServiceHTTPRouteIndex(item)
	if httpIndex < 0 {
		return nil, errors.New("virtualservice has no adjustable HTTP routes")
	}
	if len(item.Spec.HTTP[httpIndex].Route) != len(payload.Routes) {
		return nil, errors.New("virtualservice route count changed, please refresh and try again")
	}

	totalWeight := 0
	for index, route := range payload.Routes {
		if route.Weight < 0 {
			return nil, errors.New("traffic weight must be greater than or equal to 0")
		}
		totalWeight += route.Weight
		item.Spec.HTTP[httpIndex].Route[index].Weight = route.Weight
	}
	if totalWeight != 100 {
		return nil, errors.New("traffic weights must total 100")
	}

	body, err := json.Marshal(item)
	if err != nil {
		return nil, err
	}
	paths := buildIstioResourcePathsWithPreferred("virtualservices", payload.Namespace, payload.Name, item.APIVersion)
	if err := k8sDoJSONAnyPath(client, runtime, http.MethodPut, paths, nil, body, "application/json", nil); err != nil {
		return nil, errors.New(k8sClusterConnectError)
	}

	return map[string]any{
		"namespace": payload.Namespace,
		"name":      payload.Name,
		"updated":   true,
	}, nil
}

func (s *Service) UpdateK8sHTTPRouteTraffic(payload model.K8sIstioTrafficPayload) (map[string]any, error) {
	if payload.ClusterID == 0 || Trimmed(payload.Namespace) == "" || Trimmed(payload.Name) == "" || len(payload.Routes) == 0 {
		return nil, errors.New("invalid http route traffic payload")
	}

	_, runtime, client, err := s.k8sClientForCluster(payload.ClusterID)
	if err != nil {
		return nil, err
	}

	var item kubeHTTPRoute
	if err := k8sGetGatewayAPIJSON(client, runtime, "httproutes", payload.Namespace, payload.Name, &item); err != nil {
		return nil, errors.New(k8sClusterConnectError)
	}

	ruleIndex := firstHTTPRouteRuleIndex(item)
	if ruleIndex < 0 {
		return nil, errors.New("httproute has no adjustable backend refs")
	}
	if len(item.Spec.Rules[ruleIndex].BackendRefs) != len(payload.Routes) {
		return nil, errors.New("httproute backend refs changed, please refresh and try again")
	}

	totalWeight := 0
	for index, route := range payload.Routes {
		if route.Weight < 0 {
			return nil, errors.New("traffic weight must be greater than or equal to 0")
		}
		totalWeight += route.Weight
		item.Spec.Rules[ruleIndex].BackendRefs[index].Weight = route.Weight
	}
	if totalWeight != 100 {
		return nil, errors.New("traffic weights must total 100")
	}

	body, err := json.Marshal(item)
	if err != nil {
		return nil, err
	}
	paths := buildGatewayAPIResourcePathsWithPreferred("httproutes", payload.Namespace, payload.Name, item.APIVersion)
	if err := k8sDoJSONAnyPath(client, runtime, http.MethodPut, paths, nil, body, "application/json", nil); err != nil {
		return nil, errors.New(k8sClusterConnectError)
	}

	return map[string]any{
		"namespace": payload.Namespace,
		"name":      payload.Name,
		"updated":   true,
	}, nil
}

func (s *Service) GetK8sNamespaceEvents(clusterID uint, namespace string) ([]model.K8sEventItem, error) {
	_, runtime, client, err := s.k8sClientForCluster(clusterID)
	if err != nil {
		return nil, err
	}
	return fetchNamespacedEvents(client, runtime, namespace, fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=Namespace", namespace))
}
