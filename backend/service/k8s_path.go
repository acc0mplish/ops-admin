// k8s_path.go — moved verbatim from k8s.go (Phase D2, E5 seam #12).
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"ops-admin/backend/model"
)

func buildK8sYAMLResourcePath(payload model.K8sResourceYAMLPayload) (string, error) {
	resourceType := strings.ToLower(Trimmed(payload.ResourceType))
	namespace := Trimmed(payload.Namespace)
	name := Trimmed(payload.Name)

	switch resourceType {
	case "namespace":
		if name == "" {
			return "", errors.New("namespace name is required")
		}
		return "/api/v1/namespaces/" + name, nil
	case "pod":
		if namespace == "" || name == "" {
			return "", errors.New("pod namespace and name are required")
		}
		return fmt.Sprintf("/api/v1/namespaces/%s/pods/%s", namespace, name), nil
	case "service":
		if namespace == "" || name == "" {
			return "", errors.New("service namespace and name are required")
		}
		return fmt.Sprintf("/api/v1/namespaces/%s/services/%s", namespace, name), nil
	case "ingress":
		if namespace == "" || name == "" {
			return "", errors.New("ingress namespace and name are required")
		}
		return fmt.Sprintf("/apis/networking.k8s.io/v1/namespaces/%s/ingresses/%s", namespace, name), nil
	case "configmap":
		if namespace == "" || name == "" {
			return "", errors.New("configmap namespace and name are required")
		}
		return fmt.Sprintf("/api/v1/namespaces/%s/configmaps/%s", namespace, name), nil
	case "secret":
		if namespace == "" || name == "" {
			return "", errors.New("secret namespace and name are required")
		}
		return fmt.Sprintf("/api/v1/namespaces/%s/secrets/%s", namespace, name), nil
	case "pvc":
		if namespace == "" || name == "" {
			return "", errors.New("pvc namespace and name are required")
		}
		return fmt.Sprintf("/api/v1/namespaces/%s/persistentvolumeclaims/%s", namespace, name), nil
	case "pv":
		if name == "" {
			return "", errors.New("pv name is required")
		}
		return "/api/v1/persistentvolumes/" + name, nil
	case "workload":
		if namespace == "" || name == "" {
			return "", errors.New("workload namespace and name are required")
		}
		switch strings.ToLower(Trimmed(payload.WorkloadType)) {
		case "deployment":
			return fmt.Sprintf("/apis/apps/v1/namespaces/%s/deployments/%s", namespace, name), nil
		case "statefulset":
			return fmt.Sprintf("/apis/apps/v1/namespaces/%s/statefulsets/%s", namespace, name), nil
		case "daemonset":
			return fmt.Sprintf("/apis/apps/v1/namespaces/%s/daemonsets/%s", namespace, name), nil
		case "job":
			return fmt.Sprintf("/apis/batch/v1/namespaces/%s/jobs/%s", namespace, name), nil
		case "cronjob":
			return fmt.Sprintf("/apis/batch/v1/namespaces/%s/cronjobs/%s", namespace, name), nil
		default:
			return "", errors.New("unsupported workload type")
		}
	default:
		return "", errors.New("unsupported resource type")
	}
}

func buildK8sCreateResourcePaths(payload model.K8sResourceYAMLPayload, manifest k8sManifestIdentity) ([]string, error) {
	resourceType := strings.ToLower(Trimmed(payload.ResourceType))
	namespace := firstNonEmpty(Trimmed(payload.Namespace), Trimmed(manifest.Metadata.Namespace))

	switch resourceType {
	case "namespace":
		return []string{"/api/v1/namespaces"}, nil
	case "pod":
		if namespace == "" {
			return nil, errors.New("pod namespace is required")
		}
		return []string{fmt.Sprintf("/api/v1/namespaces/%s/pods", namespace)}, nil
	case "service":
		if namespace == "" {
			return nil, errors.New("service namespace is required")
		}
		return []string{fmt.Sprintf("/api/v1/namespaces/%s/services", namespace)}, nil
	case "ingress":
		if namespace == "" {
			return nil, errors.New("ingress namespace is required")
		}
		return []string{fmt.Sprintf("/apis/networking.k8s.io/v1/namespaces/%s/ingresses", namespace)}, nil
	case "configmap":
		if namespace == "" {
			return nil, errors.New("configmap namespace is required")
		}
		return []string{fmt.Sprintf("/api/v1/namespaces/%s/configmaps", namespace)}, nil
	case "secret":
		if namespace == "" {
			return nil, errors.New("secret namespace is required")
		}
		return []string{fmt.Sprintf("/api/v1/namespaces/%s/secrets", namespace)}, nil
	case "pvc":
		if namespace == "" {
			return nil, errors.New("pvc namespace is required")
		}
		return []string{fmt.Sprintf("/api/v1/namespaces/%s/persistentvolumeclaims", namespace)}, nil
	case "pv":
		return []string{"/api/v1/persistentvolumes"}, nil
	case "workload":
		if namespace == "" {
			return nil, errors.New("workload namespace is required")
		}
		switch strings.ToLower(Trimmed(payload.WorkloadType)) {
		case "deployment":
			return []string{fmt.Sprintf("/apis/apps/v1/namespaces/%s/deployments", namespace)}, nil
		case "statefulset":
			return []string{fmt.Sprintf("/apis/apps/v1/namespaces/%s/statefulsets", namespace)}, nil
		case "daemonset":
			return []string{fmt.Sprintf("/apis/apps/v1/namespaces/%s/daemonsets", namespace)}, nil
		case "job":
			return []string{fmt.Sprintf("/apis/batch/v1/namespaces/%s/jobs", namespace)}, nil
		case "cronjob":
			return []string{fmt.Sprintf("/apis/batch/v1/namespaces/%s/cronjobs", namespace)}, nil
		default:
			return nil, errors.New("unsupported workload type")
		}
	case "gateway", "virtualservice", "destinationrule", "serviceentry":
		if namespace == "" {
			return nil, fmt.Errorf("%s namespace is required", resourceType)
		}
		resourceMap := map[string]string{
			"gateway":         "gateways",
			"virtualservice":  "virtualservices",
			"destinationrule": "destinationrules",
			"serviceentry":    "serviceentries",
		}
		return buildIstioResourcePathsWithPreferred(
			resourceMap[resourceType],
			namespace,
			"",
			manifest.APIVersion,
		), nil
	case "gatewayapi":
		if namespace == "" {
			return nil, errors.New("gateway namespace is required")
		}
		return buildGatewayAPIResourcePathsWithPreferred("gateways", namespace, "", manifest.APIVersion), nil
	case "httproute":
		if namespace == "" {
			return nil, errors.New("httproute namespace is required")
		}
		return buildGatewayAPIResourcePathsWithPreferred("httproutes", namespace, "", manifest.APIVersion), nil
	default:
		return nil, errors.New("unsupported resource type")
	}
}

func buildK8sYAMLResourcePaths(payload model.K8sResourceYAMLPayload) ([]string, error) {
	resourceType := strings.ToLower(Trimmed(payload.ResourceType))
	namespace := Trimmed(payload.Namespace)
	name := Trimmed(payload.Name)

	switch resourceType {
	case "gateway", "virtualservice", "destinationrule", "serviceentry":
		if namespace == "" || name == "" {
			return nil, fmt.Errorf("%s namespace and name are required", resourceType)
		}
		resourceMap := map[string]string{
			"gateway":         "gateways",
			"virtualservice":  "virtualservices",
			"destinationrule": "destinationrules",
			"serviceentry":    "serviceentries",
		}
		return buildIstioResourcePaths(resourceMap[resourceType], namespace, name), nil
	case "gatewayapi":
		if namespace == "" || name == "" {
			return nil, errors.New("gateway namespace and name are required")
		}
		return buildGatewayAPIResourcePaths("gateways", namespace, name), nil
	case "httproute":
		if namespace == "" || name == "" {
			return nil, errors.New("httproute namespace and name are required")
		}
		return buildGatewayAPIResourcePaths("httproutes", namespace, name), nil
	default:
		path, err := buildK8sYAMLResourcePath(payload)
		if err != nil {
			return nil, err
		}
		return []string{path}, nil
	}
}

func buildK8sDeleteResourcePaths(payload model.K8sResourceDeletePayload) ([]string, error) {
	return buildK8sYAMLResourcePaths(model.K8sResourceYAMLPayload{
		ResourceType: payload.ResourceType,
		Namespace:    payload.Namespace,
		Name:         payload.Name,
		WorkloadType: payload.WorkloadType,
	})
}

func parseK8sManifestIdentity(body []byte) (k8sManifestIdentity, error) {
	var manifest k8sManifestIdentity
	if err := json.Unmarshal(body, &manifest); err != nil {
		return manifest, errors.New("invalid yaml content")
	}
	if Trimmed(manifest.Kind) == "" {
		return manifest, errors.New("resource kind is required")
	}
	return manifest, nil
}

func isK8sNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unexpected status: 404") || strings.Contains(message, "\"code\":404")
}

func friendlyK8sYAMLError(payload model.K8sResourceYAMLPayload, err error) error {
	if err == nil {
		return nil
	}
	message := err.Error()
	lower := strings.ToLower(message)

	if strings.Contains(lower, "field is immutable") || strings.Contains(lower, "immutable") {
		switch strings.ToLower(Trimmed(payload.ResourceType)) {
		case "pod":
			return errors.New("the Pod contains immutable fields that Kubernetes cannot update directly; modify only mutable fields or recreate the Pod")
		case "pv", "pvc":
			return errors.New("the storage resource contains immutable fields and cannot be overwritten directly; modify only mutable fields or use the storage-change workflow")
		default:
			return errors.New("the resource contains immutable fields and cannot be overwritten directly; check whether metadata, selector, or volume fields were changed")
		}
	}

	if strings.Contains(lower, "already exists") {
		return errors.New("the YAML resource identity conflicts with an existing cluster resource; verify the name, namespace, and related objects")
	}
	if strings.Contains(lower, "not found") {
		return errors.New("target resource does not exist; it may have been deleted or moved to another namespace; refresh and retry")
	}
	if strings.Contains(lower, "invalid") || strings.Contains(lower, "unprocessable entity") {
		return errors.New("YAML validation failed; verify field formats, apiVersion, kind, and spec content")
	}
	return err
}

func k8sGetText(client *http.Client, runtime kubeClusterRuntime, path string, query map[string]string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	endpointURL := strings.TrimRight(runtime.Server, "/") + path
	if len(query) > 0 {
		values := url.Values{}
		for key, value := range query {
			values.Set(key, value)
		}
		endpointURL += "?" + values.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpointURL, nil)
	if err != nil {
		return "", err
	}

	if runtime.Token != "" {
		req.Header.Set("Authorization", "Bearer "+runtime.Token)
	}
	if runtime.Username != "" || runtime.Password != "" {
		req.SetBasicAuth(runtime.Username, runtime.Password)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func k8sStatusText(status string) string {
	switch normalizedK8sStatus(status) {
	case "warning":
		return "Partial Alerts"
	case "offline":
		return "Offline"
	default:
		return "Running"
	}
}

func normalizedK8sStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "warning":
		return "warning"
	case "offline":
		return "offline"
	default:
		return "running"
	}
}

func calculateHealthScore(alertCount int) int {
	if alertCount <= 0 {
		return 100
	}
	score := 100 - alertCount*8
	if score < 40 {
		return 40
	}
	return score
}

func formatUsagePercent(used int64, total int64) string {
	if used <= 0 || total <= 0 {
		return "-"
	}
	value := float64(used) / float64(total) * 100
	return fmt.Sprintf("%.1f%%", value)
}

func nodeReadyStatus(node kubeNode) string {
	for _, condition := range node.Status.Conditions {
		if condition.Type == "Ready" {
			if condition.Status == "True" {
				return "Ready"
			}
			return "NotReady"
		}
	}
	return "Unknown"
}

func firstNodeInternalIP(node kubeNode) string {
	for _, address := range node.Status.Addresses {
		if address.Type == "InternalIP" && strings.TrimSpace(address.Address) != "" {
			return address.Address
		}
	}
	return "-"
}

func joinNodeRoles(labels map[string]string) string {
	roles := make([]string, 0, 3)
	for key := range labels {
		if strings.HasPrefix(key, "node-role.kubernetes.io/") {
			role := strings.TrimPrefix(key, "node-role.kubernetes.io/")
			if role == "" {
				role = "worker"
			}
			roles = append(roles, role)
		}
	}
	if len(roles) == 0 {
		return "worker"
	}
	sort.Strings(roles)
	return strings.Join(roles, ",")
}

func parseCPUToMilli(value string) int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if strings.HasSuffix(value, "m") {
		number := strings.TrimSuffix(value, "m")
		parsed, _ := strconv.ParseFloat(number, 64)
		return int64(parsed)
	}
	parsed, _ := strconv.ParseFloat(value, 64)
	return int64(parsed * 1000)
}

func parseBytesQuantity(value string) int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}

	units := map[string]float64{
		"Ki": 1024,
		"Mi": 1024 * 1024,
		"Gi": 1024 * 1024 * 1024,
		"Ti": 1024 * 1024 * 1024 * 1024,
		"Pi": 1024 * 1024 * 1024 * 1024 * 1024,
		"K":  1000,
		"M":  1000 * 1000,
		"G":  1000 * 1000 * 1000,
		"T":  1000 * 1000 * 1000 * 1000,
	}

	for suffix, multiplier := range units {
		if strings.HasSuffix(value, suffix) {
			number := strings.TrimSpace(strings.TrimSuffix(value, suffix))
			parsed, _ := strconv.ParseFloat(number, 64)
			return int64(parsed * multiplier)
		}
	}

	parsed, _ := strconv.ParseFloat(value, 64)
	return int64(parsed)
}

func formatMemoryMB(value string) string {
	bytes := parseBytesQuantity(value)
	if bytes <= 0 {
		return "-"
	}
	return fmt.Sprintf("%d MB", int64(math.Round(float64(bytes)/1000/1000)))
}

func humanizeAge(timestamp string) string {
	createdAt, err := time.Parse(time.RFC3339, strings.TrimSpace(timestamp))
	if err != nil {
		return "-"
	}

	duration := time.Since(createdAt)
	if duration < time.Minute {
		return "Just now"
	}
	if duration < time.Hour {
		return fmt.Sprintf("%dm", int(duration.Minutes()))
	}
	if duration < 24*time.Hour {
		return fmt.Sprintf("%dh", int(duration.Hours()))
	}
	if duration < 30*24*time.Hour {
		return fmt.Sprintf("%dd", int(duration.Hours()/24))
	}
	return createdAt.Format("2006-01-02")
}

func formatTimestamp(value string) string {
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
	if err != nil {
		return "-"
	}
	return t.Format("2006-01-02 15:04")
}

func stringifyTargetPort(value interface{}) string {
	switch current := value.(type) {
	case string:
		return current
	case float64:
		return strconv.Itoa(int(current))
	default:
		return fmt.Sprint(current)
	}
}

func intValue(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

func cronJobReadyText(suspend *bool, active int) string {
	if suspend != nil && *suspend {
		return "Suspended"
	}
	if active > 0 {
		return fmt.Sprintf("%d Active", active)
	}
	return "Scheduled"
}

func fallbackText(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}

func intLabel(value int, suffix string) string {
	return strconv.Itoa(value) + suffix
}
