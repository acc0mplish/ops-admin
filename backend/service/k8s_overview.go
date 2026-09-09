// k8s_overview.go — moved verbatim from k8s.go (Phase BCD, E5 seam #8b).
package service

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"sort"
	"strings"
	"time"

	"ops-admin/backend/model"
)

func buildOverviewDistribution(cluster model.K8sClusterView, nodes []kubeNode, configMaps []kubeConfigMap) []model.K8sKVTextItem {
	serviceCIDR, podCIDR := resolveK8sNetworkCIDRs(nodes, configMaps)
	return []model.K8sKVTextItem{
		{Label: "Cluster Status", Value: cluster.StatusText},
		{Label: "Cluster Version", Value: fallbackText(cluster.Version)},
		{Label: "Node Count", Value: intLabel(cluster.NodeCount, " nodes")},
		{Label: "Service CIDR", Value: serviceCIDR},
		{Label: "Pod Network", Value: podCIDR},
	}
}

// resolveK8sNetworkCIDRs reads the cluster-level CIDRs from kubeadm's ConfigMap
// when it is available, then falls back to the Pod CIDRs assigned to nodes.
// Kubernetes does not expose the Service CIDR from a stable core API, so an
// unavailable value is intentionally reported as unavailable instead of guessed.
func resolveK8sNetworkCIDRs(nodes []kubeNode, configMaps []kubeConfigMap) (string, string) {
	serviceCIDR, podCIDR := "Unknown", "Unknown"
	for _, configMap := range configMaps {
		if configMap.Metadata.Namespace != "kube-system" || configMap.Metadata.Name != "kubeadm-config" {
			continue
		}
		for _, content := range configMap.Data {
			for _, line := range strings.Split(content, "\n") {
				key, value, ok := strings.Cut(strings.TrimSpace(line), ":")
				if !ok {
					continue
				}
				value = strings.Trim(strings.TrimSpace(strings.Split(value, "#")[0]), "\"'")
				switch strings.TrimSpace(key) {
				case "serviceSubnet":
					if value != "" {
						serviceCIDR = value
					}
				case "podSubnet":
					if value != "" {
						podCIDR = value
					}
				}
			}
		}
	}
	if podCIDR == "Unknown" {
		cidrs := make([]string, 0)
		seen := map[string]struct{}{}
		for _, node := range nodes {
			values := append([]string{}, node.Spec.PodCIDRs...)
			if node.Spec.PodCIDR != "" {
				values = append(values, node.Spec.PodCIDR)
			}
			for _, cidr := range values {
				cidr = strings.TrimSpace(cidr)
				if cidr == "" {
					continue
				}
				if _, exists := seen[cidr]; !exists {
					seen[cidr] = struct{}{}
					cidrs = append(cidrs, cidr)
				}
			}
		}
		if len(cidrs) > 0 {
			sort.Strings(cidrs)
			podCIDR = strings.Join(cidrs, "、")
		}
	}
	return serviceCIDR, podCIDR
}

func buildOverviewCertificates(runtime kubeClusterRuntime) []model.K8sCertificate {
	certificates := make([]model.K8sCertificate, 0, 2)
	if certificate, ok := parseOverviewCertificate("CA Certificate", "certificate-authority", runtime.CertificateAuthority); ok {
		certificates = append(certificates, certificate)
	}
	if certificate, ok := parseOverviewCertificate("Client Certificate", "client-certificate", runtime.ClientCertificateData); ok {
		certificates = append(certificates, certificate)
	}
	return certificates
}

func parseOverviewCertificate(name string, certType string, encoded string) (model.K8sCertificate, bool) {
	encoded = strings.TrimSpace(encoded)
	if encoded == "" {
		return model.K8sCertificate{}, false
	}

	certBytes, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return model.K8sCertificate{}, false
	}

	block, _ := pem.Decode(certBytes)
	if block == nil {
		return model.K8sCertificate{}, false
	}

	certificate, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return model.K8sCertificate{}, false
	}

	daysRemaining := int(time.Until(certificate.NotAfter).Hours() / 24)
	status, statusText := k8sCertificateStatus(certificate.NotAfter)

	return model.K8sCertificate{
		Name:          name,
		Type:          certType,
		Subject:       certificateCommonName(certificate.Subject.CommonName, certificate.Subject.String()),
		Issuer:        certificateCommonName(certificate.Issuer.CommonName, certificate.Issuer.String()),
		NotBefore:     certificate.NotBefore.Local().Format("2006-01-02 15:04:05"),
		NotAfter:      certificate.NotAfter.Local().Format("2006-01-02 15:04:05"),
		DaysRemaining: daysRemaining,
		Status:        status,
		StatusText:    statusText,
	}, true
}

func certificateCommonName(commonName string, fallback string) string {
	if strings.TrimSpace(commonName) != "" {
		return strings.TrimSpace(commonName)
	}
	return fallbackText(fallback)
}

func k8sCertificateStatus(notAfter time.Time) (string, string) {
	remaining := time.Until(notAfter)
	switch {
	case remaining <= 0:
		return "expired", "Expired"
	case remaining <= 30*24*time.Hour:
		return "warning", "Expiring Soon"
	default:
		return "valid", "Valid"
	}
}

func buildNodeItems(nodes []kubeNode, pods []kubePod) []model.K8sNodeItem {
	podCountByNode := map[string]int{}
	for _, pod := range pods {
		if pod.Spec.NodeName != "" {
			podCountByNode[pod.Spec.NodeName]++
		}
	}

	items := make([]model.K8sNodeItem, 0, len(nodes))
	for _, node := range nodes {
		internalIP := "-"
		for _, address := range node.Status.Addresses {
			if address.Type == "InternalIP" {
				internalIP = fallbackText(address.Address)
				break
			}
		}

		items = append(items, model.K8sNodeItem{
			Name:       node.Metadata.Name,
			Role:       joinNodeRoles(node.Metadata.Labels),
			Status:     nodeReadyStatus(node),
			Version:    fallbackText(node.Status.NodeInfo.KubeletVersion),
			InternalIP: internalIP,
			OS:         fallbackText(node.Status.NodeInfo.OSImage),
			CPU:        fallbackText(node.Status.Allocatable["cpu"]),
			Memory:     formatMemoryMB(node.Status.Allocatable["memory"]),
			Pods:       fmt.Sprintf("%d/%s", podCountByNode[node.Metadata.Name], fallbackText(node.Status.Capacity["pods"])),
		})
	}

	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items
}

func buildNamespaceCounts(data k8sFetchedData) map[string]struct {
	pods      int
	services  int
	workloads int
} {
	counts := map[string]struct {
		pods      int
		services  int
		workloads int
	}{}

	for _, pod := range data.Pods {
		item := counts[pod.Metadata.Namespace]
		item.pods++
		counts[pod.Metadata.Namespace] = item
	}
	for _, service := range data.Services {
		item := counts[service.Metadata.Namespace]
		item.services++
		counts[service.Metadata.Namespace] = item
	}

	addWorkload := func(namespace string) {
		item := counts[namespace]
		item.workloads++
		counts[namespace] = item
	}
	for _, item := range data.Deployments {
		addWorkload(item.Metadata.Namespace)
	}
	for _, item := range data.StatefulSet {
		addWorkload(item.Metadata.Namespace)
	}
	for _, item := range data.DaemonSets {
		addWorkload(item.Metadata.Namespace)
	}
	for _, item := range data.Jobs {
		addWorkload(item.Metadata.Namespace)
	}
	for _, item := range data.CronJobs {
		addWorkload(item.Metadata.Namespace)
	}

	return counts
}

func buildNamespaceItems(namespaces []kubeNamespace, counts map[string]struct {
	pods      int
	services  int
	workloads int
}) []model.K8sNamespaceItem {
	items := make([]model.K8sNamespaceItem, 0, len(namespaces))
	for _, namespace := range namespaces {
		stat := counts[namespace.Metadata.Name]
		items = append(items, model.K8sNamespaceItem{
			Name:      namespace.Metadata.Name,
			Status:    fallbackText(namespace.Status.Phase),
			Pods:      stat.pods,
			Services:  stat.services,
			Workloads: stat.workloads,
			CreatedAt: formatTimestamp(namespace.Metadata.CreationTimestamp),
		})
	}

	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items
}
