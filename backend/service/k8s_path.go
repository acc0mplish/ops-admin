// k8s_path.go — moved verbatim from k8s.go (Phase D2, E5 seam #12).
// I10 J3: the v1 create/delete YAML path builders, manifest identity parser
// and friendly-error mapper were removed with the v1 create face (§3.7 death
// boundary) — isK8sNotFoundError (live callers in k8s_transport.go) and the
// formatter/status families (view builders) stay.
package service

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

func isK8sNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unexpected status: 404") || strings.Contains(message, "\"code\":404")
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
