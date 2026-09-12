package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"ops-admin/backend/model"
)

type openAIResponse struct {
	Content      string
	ToolCalls    []openAIToolCall
	RawToolCalls []any
}
type openAIToolCall struct{ ID, Name, Arguments string }

var (
	dsmlInvokePattern    = regexp.MustCompile(`(?s)<\|DSML\|invoke\s+name="([^"]+)">(.*?)</\|DSML\|invoke>`)
	dsmlParameterPattern = regexp.MustCompile(`(?s)<\|DSML\|parameter\s+name="([^"]+)"[^>]*>(.*?)</\|DSML\|parameter>`)
)

func parseDSMLToolCalls(content string) []openAIToolCall {
	aliases := map[string]string{
		"k8s_get_nodes":         "k8s_cluster_overview",
		"k8s_get_control_plane": "k8s_cluster_overview",
	}
	seen := map[string]bool{}
	calls := make([]openAIToolCall, 0)
	for _, match := range dsmlInvokePattern.FindAllStringSubmatch(content, -1) {
		name := aliases[match[1]]
		if name == "" {
			continue
		}
		args := map[string]any{}
		for _, parameter := range dsmlParameterPattern.FindAllStringSubmatch(match[2], -1) {
			args[parameter[1]] = strings.TrimSpace(parameter[2])
		}
		rawArgs, _ := json.Marshal(args)
		key := name + "|" + string(rawArgs)
		if seen[key] {
			continue
		}
		seen[key] = true
		calls = append(calls, openAIToolCall{ID: fmt.Sprintf("dsml-%d", len(calls)+1), Name: name, Arguments: string(rawArgs)})
	}
	return calls
}

func (s *Service) callOpenAICompatible(item model.IntegrationAIModel, messages []map[string]any, tools []map[string]any) (*openAIResponse, error) {
	return s.callOpenAICompatibleWithJSONMode(item, messages, tools, false)
}

// callOpenAICompatibleJSON requests JSON Object mode for model calls that must
// be machine-parsed. The caller can decide how to degrade if a provider does
// not support this OpenAI-compatible option.
func (s *Service) callOpenAICompatibleJSON(item model.IntegrationAIModel, messages []map[string]any) (*openAIResponse, error) {
	return s.callOpenAICompatibleWithJSONMode(item, messages, nil, true)
}

func (s *Service) callOpenAICompatibleWithJSONMode(item model.IntegrationAIModel, messages []map[string]any, tools []map[string]any, jsonMode bool) (*openAIResponse, error) {
	endpoint := strings.TrimRight(item.BaseURL, "/")
	if !strings.HasSuffix(endpoint, "/chat/completions") {
		if !strings.HasSuffix(endpoint, "/v1") {
			endpoint += "/v1"
		}
		endpoint += "/chat/completions"
	}
	maxTokens := item.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 2048
	}
	body := map[string]any{"model": item.Model, "messages": messages, "temperature": item.Temperature, "max_tokens": maxTokens}
	if len(tools) > 0 {
		body["tools"] = tools
		body["tool_choice"] = "auto"
	}
	if jsonMode {
		body["response_format"] = map[string]string{"type": "json_object"}
	}
	raw, _ := json.Marshal(body)
	request, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(item.APIKey) != "" {
		request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(item.APIKey))
	}
	timeout := item.TimeoutSeconds
	if timeout < 5 {
		timeout = 60
	}
	response, err := (&http.Client{Timeout: time.Duration(timeout) * time.Second}).Do(request)
	if err != nil {
		return nil, fmt.Errorf("model request failed: %w", err)
	}
	defer response.Body.Close()
	responseBody, _ := io.ReadAll(io.LimitReader(response.Body, 8*1024*1024))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("model API returned %d: %s", response.StatusCode, strings.TrimSpace(string(responseBody)))
	}
	var decoded struct {
		Choices []struct {
			Message struct {
				Content   any `json:"content"`
				ToolCalls []struct {
					ID       string `json:"id"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(responseBody, &decoded); err != nil {
		return nil, err
	}
	if decoded.Error != nil {
		return nil, errors.New(decoded.Error.Message)
	}
	if len(decoded.Choices) == 0 {
		return nil, errors.New("model API returned no valid content")
	}
	message := decoded.Choices[0].Message
	result := &openAIResponse{Content: openAIContentString(message.Content), RawToolCalls: make([]any, 0, len(message.ToolCalls))}
	for _, call := range message.ToolCalls {
		result.ToolCalls = append(result.ToolCalls, openAIToolCall{ID: call.ID, Name: call.Function.Name, Arguments: call.Function.Arguments})
		result.RawToolCalls = append(result.RawToolCalls, map[string]any{"id": call.ID, "type": "function", "function": map[string]any{"name": call.Function.Name, "arguments": call.Function.Arguments}})
	}
	if len(result.ToolCalls) == 0 {
		for _, call := range parseDSMLToolCalls(result.Content) {
			result.ToolCalls = append(result.ToolCalls, call)
			result.RawToolCalls = append(result.RawToolCalls, map[string]any{"id": call.ID, "type": "function", "function": map[string]any{"name": call.Name, "arguments": call.Arguments}})
		}
		if len(result.ToolCalls) > 0 {
			result.Content = ""
		}
	}
	return result, nil
}

func openAIContentString(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	if parts, ok := value.([]any); ok {
		var builder strings.Builder
		for _, part := range parts {
			if item, ok := part.(map[string]any); ok {
				if text, ok := item["text"].(string); ok {
					builder.WriteString(text)
				}
			}
		}
		return builder.String()
	}
	return ""
}

func (s *Service) openAIToolDefinitions() ([]map[string]any, map[string]model.IntegrationAIToolConfig, error) {
	configs, err := s.ensureIntegrationAIToolConfigs()
	if err != nil {
		return nil, nil, err
	}
	configMap := map[string]model.IntegrationAIToolConfig{}
	for _, item := range configs {
		configMap[item.ToolKey] = item
	}
	tools := make([]map[string]any, 0)
	for _, definition := range integrationAIToolDefinitions {
		config := configMap[definition.Key]
		if !config.Enabled {
			continue
		}
		tools = append(tools, map[string]any{"type": "function", "function": map[string]any{"name": definition.Key, "description": definition.Description, "parameters": definition.Parameters}})
	}
	return tools, configMap, nil
}

func (s *Service) ensureIntegrationAIToolConfigs() ([]model.IntegrationAIToolConfig, error) {
	for _, definition := range integrationAIToolDefinitions {
		var count int64
		if err := s.db.Model(&model.IntegrationAIToolConfig{}).Where("tool_key = ?", definition.Key).Count(&count).Error; err != nil {
			return nil, err
		}
		if count == 0 {
			item := model.IntegrationAIToolConfig{ToolKey: definition.Key, Enabled: true, RequireConfirmation: definition.RequireConfirmation}
			if err := s.db.Create(&item).Error; err != nil {
				return nil, err
			}
		}
	}
	var configs []model.IntegrationAIToolConfig
	err := s.db.Order("id ASC").Find(&configs).Error
	return configs, err
}

func (s *Service) ListIntegrationAITools() ([]map[string]any, error) {
	configs, err := s.ensureIntegrationAIToolConfigs()
	if err != nil {
		return nil, err
	}
	configMap := map[string]model.IntegrationAIToolConfig{}
	for _, item := range configs {
		configMap[item.ToolKey] = item
	}
	result := make([]map[string]any, 0, len(integrationAIToolDefinitions))
	for _, definition := range integrationAIToolDefinitions {
		config := configMap[definition.Key]
		result = append(result, map[string]any{"id": config.ID, "toolKey": definition.Key, "name": definition.Name, "category": definition.Category, "description": definition.Description, "permission": definition.Permission, "enabled": config.Enabled, "requireConfirmation": config.RequireConfirmation, "parameters": definition.Parameters, "updateTime": config.UpdatedAt})
	}
	return result, nil
}

func (s *Service) UpdateIntegrationAITool(payload IntegrationAIToolUpdatePayload) error {
	for _, definition := range integrationAIToolDefinitions {
		if definition.Key == payload.ToolKey {
			if definition.Permission == "write" {
				payload.RequireConfirmation = true
			}
			return s.db.Model(&model.IntegrationAIToolConfig{}).Where("tool_key = ?", payload.ToolKey).Updates(map[string]any{"enabled": payload.Enabled, "require_confirmation": payload.RequireConfirmation}).Error
		}
	}
	return errors.New("unknown AI tool")
}

func (s *Service) ExecuteIntegrationAITool(payload IntegrationAIToolExecutePayload) (any, error) {
	return s.executeIntegrationAITool(payload.ToolKey, payload.Arguments)
}

func (s *Service) executeIntegrationAITool(toolKey string, args map[string]any) (any, error) {
	switch toolKey {
	case "knowledge_base_search":
		return s.queryAIKnowledgeBase(args)
	case "prometheus_query":
		query := strings.TrimSpace(anyString(args["query"]))
		if query == "" {
			return nil, errors.New("PromQL is required")
		}
		datasourceID := anyUint(args["datasourceId"])
		var ds model.MonitorDatasource
		q := s.db.Where("status = ? AND type IN ?", 1, []string{"prometheus", "victoriametrics"})
		if datasourceID > 0 {
			q = s.db.Where("id = ? AND status = ?", datasourceID, 1)
		} else {
			q = q.Order("is_default DESC, id ASC")
		}
		if err := q.First(&ds).Error; err != nil {
			return nil, errors.New("no Prometheus or VictoriaMetrics datasource is available")
		}
		return s.prometheusQuery(ds, query, time.Now())
	case "monitor_log_query":
		return s.queryAIRealtimeLogs(args, time.Now())
	case "monitor_dashboard_list":
		return s.ListMonitorDashboards(1, 20, anyString(args["keyword"]), "1")
	case "monitor_datasource_query":
		return s.queryAIMonitorDatasources(args)
	case "monitor_alert_event_query":
		return s.queryAIMonitorAlertEvents(args)
	case "host_health_diagnose":
		return s.queryAIHostHealth(args)
	case "ops_troubleshooting":
		return s.queryAIOpsTroubleshooting(args)
	case "monitor_dashboard_analyze":
		return s.queryAIMonitorDashboard(args)
	case "finops_cost_analysis":
		return s.queryAIFinOpsAnalysis(args)
	case "asset_host_list":
		return s.queryAIAssetHosts(args)
	case "asset_mysql_list":
		return s.queryAIAssetDatabases("mysql", args)
	case "asset_postgresql_list":
		return s.queryAIAssetDatabases("postgresql", args)
	case "asset_redis_list":
		return s.queryAIAssetDatabases("redis", args)
	case "asset_mongodb_list":
		return s.queryAIAssetDatabases("mongodb", args)
	case "k8s_list_clusters":
		return s.ListK8sClusters()
	case "k8s_cluster_overview":
		return s.GetK8sClusterDetail(anyUint(args["clusterId"]))
	default:
		return nil, errors.New("this tool can run only through a pending confirmation action")
	}
}

func (s *Service) queryAIMonitorDatasources(args map[string]any) (map[string]any, error) {
	limit := aiAssetQueryLimit(args["limit"])
	query := s.db.Model(&model.MonitorDatasource{})
	if keyword := strings.TrimSpace(anyString(args["keyword"])); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("name LIKE ? OR description LIKE ?", like, like)
	}
	if dsType := strings.TrimSpace(anyString(args["type"])); dsType != "" {
		query = query.Where("type = ?", normalizeMonitorDatasourceType(dsType))
	}
	if health := strings.TrimSpace(anyString(args["healthStatus"])); health != "" {
		query = query.Where("health_status = ?", health)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var list []model.MonitorDatasource
	if err := query.Order("is_default DESC, id ASC").Limit(limit).Find(&list).Error; err != nil {
		return nil, err
	}
	items := make([]map[string]any, 0, len(list))
	for _, item := range list {
		items = append(items, map[string]any{"id": item.ID, "name": item.Name, "type": item.Type, "environment": item.Env, "enabled": item.Status == 1, "default": item.IsDefault, "healthStatus": item.HealthStatus, "lastCheckAt": item.LastCheckAt, "lastSuccessAt": item.LastSuccessAt, "latencyMs": item.LatencyMs, "consecutiveFailures": item.ConsecutiveFailures, "description": item.Description})
	}
	return map[string]any{"total": total, "returned": len(items), "items": items}, nil
}

func (s *Service) queryAIMonitorAlertEvents(args map[string]any) (map[string]any, error) {
	limit := aiAssetQueryLimit(args["limit"])
	query := s.db.Model(&model.MonitorAlertEvent{})
	if keyword := strings.TrimSpace(anyString(args["keyword"])); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("rule_name LIKE ? OR metric LIKE ? OR summary LIKE ? OR labels_json LIKE ?", like, like, like, like)
	}
	if status := strings.TrimSpace(anyString(args["status"])); status != "" {
		query = query.Where("status = ?", status)
	}
	if severity := strings.TrimSpace(anyString(args["severity"])); severity != "" {
		query = query.Where("severity = ?", normalizeSeverity(severity))
	}
	if startAt, ok := parseAIQueryTime(anyString(args["startTime"])); ok {
		query = query.Where("last_trigger_at >= ?", startAt)
	}
	if endAt, ok := parseAIQueryTime(anyString(args["endTime"])); ok {
		query = query.Where("last_trigger_at <= ?", endAt)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var list []model.MonitorAlertEvent
	if err := query.Order("last_trigger_at DESC, id DESC").Limit(limit).Find(&list).Error; err != nil {
		return nil, err
	}
	items := make([]map[string]any, 0, len(list))
	for _, item := range list {
		items = append(items, map[string]any{"id": item.ID, "ruleId": item.RuleID, "ruleName": item.RuleName, "datasource": item.DatasourceName, "severity": item.Severity, "status": item.Status, "metric": item.Metric, "currentValue": item.CurrentValue, "threshold": item.Threshold, "summary": item.Summary, "claimedBy": item.ClaimedBy, "firstTriggerAt": item.FirstTriggerAt, "lastTriggerAt": item.LastTriggerAt, "recoveredAt": item.RecoveredAt})
	}
	return map[string]any{"total": total, "returned": len(items), "items": items}, nil
}
