package service

import (
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"

	"ops-admin/backend/model"
)

func normalizeMonitorDatasourceType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "victoriametrics", "victoria-metrics", "vm":
		return "victoriametrics"
	case "victorialogs", "victoria-logs", "vl":
		return "victorialogs"
	case "elasticsearch", "elastic", "es":
		return "elasticsearch"
	case "jaeger", "jaeger-query", "tracing":
		return "jaeger"
	default:
		return "prometheus"
	}
}

func isMonitorLogDatasource(value string) bool {
	datasourceType := normalizeMonitorDatasourceType(value)
	return datasourceType == "elasticsearch" || datasourceType == "victorialogs"
}

func isMonitorMetricDatasource(value string) bool {
	datasourceType := normalizeMonitorDatasourceType(value)
	return datasourceType == "prometheus" || datasourceType == "victoriametrics"
}

func isMonitorTraceDatasource(value string) bool {
	return normalizeMonitorDatasourceType(value) == "jaeger"
}

func normalizeAlertType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "victorialogs", "victoria-logs", "vl":
		return "victorialogs"
	case "log", "elasticsearch", "es":
		return "log"
	default:
		return "metric"
	}
}

func isMonitorLogAlertType(alertType string) bool {
	return normalizeAlertType(alertType) == "log" || normalizeAlertType(alertType) == "victorialogs"
}

func normalizeDatasourceScope(value string) string {
	if strings.EqualFold(strings.TrimSpace(value), "all") {
		return "all"
	}
	return "specific"
}

func normalizeLogTimeRangeSeconds(value int) int {
	if value < 60 {
		return 300
	}
	if value > 86400 {
		return 86400
	}
	return value
}

func normalizeMonitorStatus(value int) int {
	if value == 2 {
		return 2
	}
	return 1
}

func normalizeMonitorAuthType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "basic":
		return "basic"
	case "bearer":
		return "bearer"
	case "apikey", "api_key", "api-key":
		return "apikey"
	default:
		return "none"
	}
}

func normalizeComparator(value string) string {
	switch strings.TrimSpace(value) {
	case ">", ">=", "<", "<=", "==", "!=":
		return strings.TrimSpace(value)
	default:
		return ">"
	}
}

func normalizeSeverity(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	switch value {
	case "P0", "P1", "P2", "P3":
		return value
	default:
		return "P2"
	}
}

func normalizeEvalInterval(value int) int {
	if value < 15 {
		return 15
	}
	if value > 3600 {
		return 3600
	}
	return value
}

func normalizeForSeconds(value int) int {
	if value < 0 {
		return 0
	}
	if value > 86400 {
		return 86400
	}
	return value
}

func normalizeNotifyRepeatInterval(value int) int {
	if value < 60 {
		return 60
	}
	if value > 604800 {
		return 604800
	}
	return value
}

func normalizeMaxNotifyCount(value int) int {
	if value < 0 {
		return 0
	}
	if value > 1000 {
		return 1000
	}
	return value
}

func ensureJSONObject(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "{}", nil
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(value), &obj); err != nil {
		return "", errors.New("invalid JSON format")
	}
	data, _ := json.Marshal(obj)
	return string(data), nil
}

func normalizeMatcherJSON(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "{}", nil
	}
	matchers := map[string]string{}
	if err := json.Unmarshal([]byte(value), &matchers); err != nil {
		return "", errors.New("matchers must be valid JSON object")
	}
	data, _ := json.Marshal(matchers)
	return string(data), nil
}

func unixPtr(value int64) *time.Time {
	if value <= 0 {
		return nil
	}
	t := time.Unix(value, 0)
	return &t
}

func normalizeAggregationWindow(value int) int {
	if value < 60 {
		return 60
	}
	if value > 86400 {
		return 86400
	}
	return value
}

func normalizeRuleMatchMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "select", "selected", "rules":
		return "select"
	default:
		return "regex"
	}
}

func monitorRuleNameMatch(pattern, ruleName string) bool {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return true
	}
	re, err := regexp.Compile(pattern)
	if err == nil {
		return re.MatchString(ruleName)
	}
	return strings.Contains(strings.ToLower(ruleName), strings.ToLower(pattern))
}

func monitorRuleMatch(matchMode, ruleIDsJSON, pattern string, rule model.MonitorAlertRule) bool {
	if normalizeRuleMatchMode(matchMode) == "select" {
		ids := decodeUintList(ruleIDsJSON)
		if len(ids) == 0 {
			return true
		}
		for _, id := range ids {
			if id == rule.ID {
				return true
			}
		}
		return false
	}
	return monitorRuleNameMatch(pattern, rule.Name)
}

func monitorSeverityMatch(pattern, severity string) bool {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" || strings.EqualFold(pattern, "all") {
		return true
	}
	return strings.EqualFold(pattern, severity)
}

func decodeLabelMap(raw string) map[string]string {
	labels := map[string]string{}
	_ = json.Unmarshal([]byte(raw), &labels)
	return labels
}

func monitorMatchersMatch(matchersJSON string, labels map[string]string) bool {
	matchers := map[string]string{}
	_ = json.Unmarshal([]byte(matchersJSON), &matchers)
	for key, expected := range matchers {
		if labels[key] != expected {
			return false
		}
	}
	return true
}
