// k8s_mutate.go — moved verbatim from k8s.go (Phase BCD, E5 seam #5).
// I10 J3: the v1 YAML create service method is gone — create lives on the
// §16.1 connection-scoped k8s.resource.create operation (v2). The namespace
// event read stays (live event read, D-12 surface).
package service

import (
	"fmt"

	"ops-admin/backend/model"
)

func (s *Service) GetK8sNamespaceEvents(clusterID uint, namespace string) ([]model.K8sEventItem, error) {
	_, runtime, client, err := s.k8sClientForCluster(clusterID)
	if err != nil {
		return nil, err
	}
	return fetchNamespacedEvents(client, runtime, namespace, fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=Namespace", namespace))
}
