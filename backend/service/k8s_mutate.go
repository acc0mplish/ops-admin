// k8s_mutate.go — moved verbatim from k8s.go (Phase BCD, E5 seam #5).
package service

import (
	"errors"
	"fmt"
	"net/http"

	"ops-admin/backend/model"

	ksyaml "sigs.k8s.io/yaml"
)

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

func (s *Service) GetK8sNamespaceEvents(clusterID uint, namespace string) ([]model.K8sEventItem, error) {
	_, runtime, client, err := s.k8sClientForCluster(clusterID)
	if err != nil {
		return nil, err
	}
	return fetchNamespacedEvents(client, runtime, namespace, fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=Namespace", namespace))
}
