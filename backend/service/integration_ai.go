package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"ops-admin/backend/model"

	"gorm.io/gorm"
)

type IntegrationAIModelPayload struct {
	ID             uint    `json:"id"`
	Name           string  `json:"name"`
	Provider       string  `json:"provider"`
	BaseURL        string  `json:"baseUrl"`
	APIKey         string  `json:"apiKey"`
	Model          string  `json:"model"`
	SystemPrompt   string  `json:"systemPrompt"`
	Temperature    float64 `json:"temperature"`
	MaxTokens      int     `json:"maxTokens"`
	TimeoutSeconds int     `json:"timeoutSeconds"`
	IsDefault      bool    `json:"isDefault"`
	Status         int     `json:"status"`
	Description    string  `json:"description"`
}

type IntegrationAIConversationPayload struct {
	ID      uint   `json:"id"`
	ModelID uint   `json:"modelId"`
	Title   string `json:"title"`
	Pinned  bool   `json:"pinned"`
}

type IntegrationAIChatPayload struct {
	ConversationID uint   `json:"conversationId"`
	ModelID        uint   `json:"modelId"`
	Content        string `json:"content"`
}

type IntegrationAIToolUpdatePayload struct {
	ToolKey             string `json:"toolKey"`
	Enabled             bool   `json:"enabled"`
	RequireConfirmation bool   `json:"requireConfirmation"`
}

type IntegrationAIToolExecutePayload struct {
	ToolKey   string         `json:"toolKey"`
	Arguments map[string]any `json:"arguments"`
}

type IntegrationAIKnowledgeDocumentPayload struct {
	ID         uint   `json:"id"`
	Name       string `json:"name"`
	FileName   string `json:"fileName"`
	Content    string `json:"content"`
	Status     int    `json:"status"`
	SourceType string `json:"sourceType"`
}

type aiToolDefinition struct {
	Key                 string
	Name                string
	Category            string
	Description         string
	Permission          string
	RequireConfirmation bool
	Parameters          map[string]any
}

var integrationAIToolDefinitions = []aiToolDefinition{
	{Key: "knowledge_base_search", Name: "Knowledge Base 검색", Category: "Knowledge Base", Description: "Knowledge Base 관리에서 활성화된 Local Markdown 문서만 검색합니다. Internal Standard, Runbook, Technical Document 답변에 사용하며 외부 File 또는 Cloud Service에는 접근하지 않습니다.", Permission: "read", Parameters: knowledgeBaseToolSchema()},
	{Key: "prometheus_query", Name: "PromQL Instant Query", Category: "Monitoring Center", Description: "Prometheus 또는 VictoriaMetrics Datasource에서 Instant PromQL Query를 실행합니다.", Permission: "read", Parameters: objectSchema(map[string]any{"datasourceId": integerProperty("Datasource ID. 비워두면 Default Datasource를 사용합니다."), "query": stringProperty("PromQL Query")}, []string{"query"})},
	{Key: "monitor_log_query", Name: "Log Instant Query", Category: "Monitoring Center", Description: "Elasticsearch 또는 VictoriaLogs에서 Time Range 기준으로 Log를 조회하고 Match Count와 일부 Detail을 반환합니다.", Permission: "read", Parameters: logQueryToolSchema()},
	{Key: "monitor_dashboard_list", Name: "Monitoring Dashboard 조회", Category: "Grafana Visualization", Description: "Platform Monitoring Dashboard와 Panel Overview를 조회해 Visualization 기반 Troubleshooting Entry를 제공합니다.", Permission: "read", Parameters: objectSchema(map[string]any{"keyword": stringProperty("Dashboard 이름 Keyword")}, nil)},
	{Key: "monitor_datasource_query", Name: "Monitoring Datasource 조회", Category: "Monitoring Skill", Description: "연결된 Monitoring 및 Log Datasource의 Type, Health, Latency, Last Check를 조회하며 Credential은 반환하지 않습니다.", Permission: "read", Parameters: datasourceQueryToolSchema()},
	{Key: "monitor_alert_event_query", Name: "Alert Event 조회", Category: "Monitoring Skill", Description: "Keyword, Status, Severity, Time Range 기준으로 Alert Event를 조회하고 Count와 제한된 Detail을 반환합니다.", Permission: "read", Parameters: alertEventQueryToolSchema()},
	{Key: "host_health_diagnose", Name: "Host 상태 진단", Category: "Monitoring Skill", Description: "CMDB Host 정보, 최근 24시간 CPU/Memory/Disk Metric, 연관 Alert를 결합해 상태 Evidence를 반환합니다. 수정 작업은 실행하지 않습니다.", Permission: "read", Parameters: hostHealthToolSchema()},
	{Key: "ops_troubleshooting", Name: "지능형 Troubleshooting", Category: "Monitoring Skill", Description: "Alert ID, Host 또는 Issue Keyword를 기준으로 Alert, Host 상태, Datasource 상태를 수집해 Evidence 기반 Troubleshooting Context를 구성합니다.", Permission: "read", Parameters: troubleshootingToolSchema()},
	{Key: "monitor_dashboard_analyze", Name: "Monitoring Dashboard 분석", Category: "Monitoring Skill", Description: "Dashboard와 Panel Definition, Datasource, PromQL, Description을 읽어 Metric 의미와 Troubleshooting Entry를 분석합니다. Dashboard는 수정하지 않습니다.", Permission: "read", Parameters: dashboardAnalyzeToolSchema()},
	{Key: "monitor_alert_rule_draft", Name: "Alert Rule Draft", Category: "Monitoring Skill", Description: "기본 비활성 및 Notification 미전송 상태의 Alert Rule Draft를 생성합니다. 사용자 확인 후 저장하며 이후 Alert Rule Page에서 검토하고 활성화해야 합니다.", Permission: "write", RequireConfirmation: true, Parameters: alertRuleDraftToolSchema()},
	{Key: "finops_cost_analysis", Name: "Cloud 비용 분석", Category: "Cloud Cost FinOps", Description: "Billing Sync를 통해 Local Database에 저장된 Cloud Cost만 조회합니다. Overview, Trend, Product/Region Breakdown을 반환하며 Cloud Provider API를 호출하거나 Billing을 동기화하지 않습니다.", Permission: "read", Parameters: finOpsAnalysisToolSchema()},
	{Key: "asset_host_list", Name: "Server Asset", Category: "Asset Management", Description: "CMDB Server, IP, Environment, Host Group, Online Status를 조회하며 Login Credential은 반환하지 않습니다.", Permission: "read", Parameters: assetQuerySchema("Server 이름, Alias 또는 IP Keyword")},
	{Key: "asset_mysql_list", Name: "MySQL Asset", Category: "Asset Management", Description: "관리 중인 MySQL Connection, Environment, Version, Health Status를 조회합니다.", Permission: "read", Parameters: assetQuerySchema("Database 이름, 주소 또는 Default Database Keyword")},
	{Key: "asset_postgresql_list", Name: "PostgreSQL Asset", Category: "Asset Management", Description: "관리 중인 PostgreSQL Connection, Environment, Version, Health Status를 조회합니다.", Permission: "read", Parameters: assetQuerySchema("Database 이름, 주소 또는 Default Database Keyword")},
	{Key: "asset_redis_list", Name: "Redis Asset", Category: "Asset Management", Description: "관리 중인 Redis Instance, Logical DB, Environment, Version, Health Status를 조회합니다.", Permission: "read", Parameters: assetQuerySchema("Redis 이름, 주소 또는 Logical DB Keyword")},
	{Key: "asset_mongodb_list", Name: "MongoDB Asset", Category: "Asset Management", Description: "관리 중인 MongoDB Connection, Environment, Version, Health Status를 조회합니다.", Permission: "read", Parameters: assetQuerySchema("Database 이름, 주소 또는 Default Database Keyword")},
	{Key: "k8s_list_clusters", Name: "Kubernetes Cluster 목록", Category: "Kubernetes", Description: "연결된 Cluster의 Status, Version, Node Count를 조회합니다.", Permission: "read", Parameters: objectSchema(map[string]any{}, nil)},
	{Key: "k8s_cluster_overview", Name: "Kubernetes Cluster Overview", Category: "Kubernetes", Description: "지정한 Cluster의 Node, Workload, Pod, Health Overview를 조회합니다.", Permission: "read", Parameters: objectSchema(map[string]any{"clusterId": integerProperty("Kubernetes Cluster ID")}, []string{"clusterId"})},
	{Key: "k8s_restart_workload", Name: "Kubernetes Workload Restart", Category: "Kubernetes", Description: "Deployment, StatefulSet 또는 DaemonSet에 Rolling Restart를 실행합니다.", Permission: "write", RequireConfirmation: true, Parameters: workloadActionSchema(false)},
	{Key: "k8s_scale_workload", Name: "Kubernetes Workload Scale", Category: "Kubernetes", Description: "Deployment 또는 StatefulSet의 Replica 수를 변경합니다.", Permission: "write", RequireConfirmation: true, Parameters: workloadActionSchema(true)},
}

func stringProperty(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}
func integerProperty(description string) map[string]any {
	return map[string]any{"type": "integer", "description": description}
}
func booleanProperty(description string) map[string]any {
	return map[string]any{"type": "boolean", "description": description}
}
func objectSchema(properties map[string]any, required []string) map[string]any {
	result := map[string]any{"type": "object", "properties": properties, "additionalProperties": false}
	if len(required) > 0 {
		result["required"] = required
	}
	return result
}

func assetQuerySchema(keywordDescription string) map[string]any {
	return objectSchema(map[string]any{
		"keyword":     stringProperty(keywordDescription),
		"environment": stringProperty("Environment Code(예: dev, test, prod). 비워두면 전체 Environment를 조회합니다."),
		"limit":       integerProperty("최대 반환 건수. 범위 1~50, 기본값 20"),
	}, nil)
}

func finOpsAnalysisToolSchema() map[string]any {
	return objectSchema(map[string]any{
		"accountId":                integerProperty("Cloud Account ID. 비워두면 동기화된 모든 Cloud Account를 분석합니다."),
		"month":                    stringProperty("분석 Billing Month(YYYY-MM). 비워두면 Local Billing이 있는 최근 Calendar Month를 사용합니다."),
		"trendMonths":              integerProperty("Trend Month 수. 범위 1~12, 기본값 6"),
		"service":                  stringProperty("Cloud Product Keyword(예: Load Balancer, NAT Gateway, ECS)"),
		"includeResourceBreakdown": booleanProperty("Cloud Product 비용을 Instance/Resource ID 기준으로 집계할지 여부. Instance 수 또는 Instance별 비용 질의에는 true를 사용합니다."),
	}, nil)
}

func knowledgeBaseToolSchema() map[string]any {
	return objectSchema(map[string]any{
		"keyword":    stringProperty("검색할 Keyword 또는 Question(예: Release Process, Redis 장애 대응)"),
		"documentId": integerProperty("선택 사항. 특정 Knowledge Base 문서 ID로 검색 범위를 제한합니다."),
		"limit":      integerProperty("최대 Match Fragment 수. 범위 1~10, 기본값 5"),
	}, []string{"keyword"})
}

func logQueryToolSchema() map[string]any {
	return objectSchema(map[string]any{
		"datasourceId":   integerProperty("Log Datasource ID. 비워두면 활성화된 모든 Elasticsearch와 VictoriaLogs Datasource를 조회합니다."),
		"datasourceName": stringProperty("Datasource 이름 Keyword. datasourceId를 지정하지 않았을 때만 사용합니다."),
		"index":          stringProperty("Elasticsearch Index 또는 Pattern(예: logs-*). 비워두면 전체 Index를 조회합니다."),
		"streams":        stringProperty("Stream/Topic. 여러 값은 comma로 구분합니다. 기본값은 contains match이며 =app.err.log.1053 형식은 exact match입니다."),
		"query":          stringProperty("Log 조건. Elasticsearch는 Lucene, VictoriaLogs는 LogsQL을 사용합니다. 예: level:ERROR. 비워두면 전체입니다."),
		"startTime":      stringProperty("시작 시각. RFC3339, YYYY-MM-DD HH:mm:ss, 어제 10:00 또는 yesterday 10:00을 지원하며 Timezone은 Asia/Seoul입니다."),
		"endTime":        stringProperty("종료 시각. startTime과 같은 형식이며 시작 시각보다 늦어야 합니다."),
		"mode":           stringProperty("반환 Mode. count는 건수만, list는 일부 Log Detail을 반환합니다. 기본값은 count입니다."),
		"limit":          integerProperty("list Mode의 최대 Log 수. 범위 1~50, 기본값 20"),
	}, []string{"startTime", "endTime"})
}

func datasourceQueryToolSchema() map[string]any {
	return objectSchema(map[string]any{"keyword": stringProperty("Datasource 이름 Keyword"), "type": stringProperty("Datasource Type: prometheus, victoriametrics, elasticsearch 또는 victorialogs"), "healthStatus": stringProperty("상태(예: healthy, unhealthy, unknown)"), "limit": integerProperty("반환 건수. 범위 1~50, 기본값 20")}, nil)
}

func alertEventQueryToolSchema() map[string]any {
	return objectSchema(map[string]any{"keyword": stringProperty("Rule 이름, Metric, Summary 또는 Label Keyword"), "status": stringProperty("Alert 상태(예: firing, claimed, recovered, resolved)"), "severity": stringProperty("Alert Severity: P0, P1, P2, P3"), "startTime": stringProperty("선택 사항. RFC3339 또는 YYYY-MM-DD HH:mm:ss"), "endTime": stringProperty("선택 사항. RFC3339 또는 YYYY-MM-DD HH:mm:ss"), "limit": integerProperty("반환 건수. 범위 1~50, 기본값 20")}, nil)
}

func hostHealthToolSchema() map[string]any {
	return objectSchema(map[string]any{"hostId": integerProperty("CMDB Host ID. keyword와 둘 중 하나를 사용합니다."), "keyword": stringProperty("Host 이름, Alias 또는 IP"), "range": stringProperty("Metric Time Range: 1h, 6h, 24h, 7d. 기본값 24h")}, nil)
}

func troubleshootingToolSchema() map[string]any {
	return objectSchema(map[string]any{"alertEventId": integerProperty("선택 사항. Alert Event ID"), "host": stringProperty("선택 사항. Host 이름, Alias 또는 IP"), "keyword": stringProperty("선택 사항. Issue, Rule 또는 Metric Keyword"), "range": stringProperty("Host Metric Time Range: 1h, 6h, 24h, 7d. 기본값 24h")}, nil)
}

func dashboardAnalyzeToolSchema() map[string]any {
	return objectSchema(map[string]any{"dashboardId": integerProperty("Monitoring Dashboard ID"), "keyword": stringProperty("Dashboard 이름 Keyword. ID가 없을 때 사용합니다.")}, nil)
}

func alertRuleDraftToolSchema() map[string]any {
	return objectSchema(map[string]any{
		"name": stringProperty("Rule 이름"), "datasourceId": integerProperty("Prometheus/VictoriaMetrics Datasource ID"), "promql": stringProperty("검증된 PromQL"),
		"comparator": stringProperty("Comparator: >, >=, <, <=, ==, !="), "threshold": stringProperty("Threshold 숫자"), "forSeconds": integerProperty("지속 시간(초). 기본값 300"),
		"severity": stringProperty("Severity: P0, P1, P2, P3. 기본값 P2"), "description": stringProperty("Rule 설명"), "env": stringProperty("Environment(예: prod)"),
	}, []string{"name", "datasourceId", "promql"})
}

func workloadActionSchema(withReplicas bool) map[string]any {
	properties := map[string]any{
		"clusterId": integerProperty("Kubernetes Cluster ID"), "namespace": stringProperty("Namespace"),
		"workloadType": stringProperty("Workload Type(예: deployment)"), "workloadName": stringProperty("Workload 이름"),
	}
	required := []string{"clusterId", "namespace", "workloadType", "workloadName"}
	if withReplicas {
		properties["replicas"] = integerProperty("Target Replica 수")
		required = append(required, "replicas")
	}
	return objectSchema(properties, required)
}

func (s *Service) ListIntegrationAIKnowledgeDocuments(keyword string) ([]map[string]any, error) {
	var documents []model.IntegrationAIKnowledgeDocument
	query := s.db.Order("updated_at DESC, id DESC")
	keyword = strings.TrimSpace(keyword)
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("name LIKE ? OR file_name LIKE ?", like, like)
	}
	if err := query.Find(&documents).Error; err != nil {
		return nil, err
	}
	result := make([]map[string]any, 0, len(documents))
	for _, item := range documents {
		result = append(result, map[string]any{
			"id": item.ID, "name": item.Name, "fileName": item.FileName, "sourceType": item.SourceType,
			"content": item.Content, "status": item.Status, "createTime": item.CreatedAt, "updateTime": item.UpdatedAt,
		})
	}
	return result, nil
}

func (s *Service) SaveIntegrationAIKnowledgeDocument(payload IntegrationAIKnowledgeDocumentPayload) (map[string]any, error) {
	payload.Name = strings.TrimSpace(payload.Name)
	payload.Content = strings.TrimSpace(payload.Content)
	if payload.Name == "" {
		return nil, errors.New("knowledge-base document name is required")
	}
	if payload.Content == "" {
		return nil, errors.New("Markdown content is required")
	}
	if len([]rune(payload.Content)) > 500000 {
		return nil, errors.New("Markdown content must not exceed 500000 characters")
	}
	if payload.FileName == "" {
		payload.FileName = payload.Name + ".md"
	}
	if !strings.EqualFold(filepathExt(payload.FileName), ".md") {
		payload.FileName += ".md"
	}
	if payload.SourceType == "" {
		payload.SourceType = "manual"
	}
	if payload.Status != 2 {
		payload.Status = 1
	}
	item := model.IntegrationAIKnowledgeDocument{ID: payload.ID}
	if payload.ID > 0 {
		if err := s.db.First(&item, payload.ID).Error; err != nil {
			return nil, errors.New("knowledge-base document does not exist")
		}
	}
	item.Name, item.FileName, item.Content, item.Status, item.SourceType = payload.Name, payload.FileName, payload.Content, payload.Status, payload.SourceType
	var err error
	if item.ID == 0 {
		err = s.db.Create(&item).Error
	} else {
		err = s.db.Save(&item).Error
	}
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": item.ID, "name": item.Name, "fileName": item.FileName, "sourceType": item.SourceType, "status": item.Status, "updateTime": item.UpdatedAt}, nil
}

func (s *Service) DeleteIntegrationAIKnowledgeDocument(id uint) error {
	if id == 0 {
		return errors.New("knowledge-base document ID is required")
	}
	return s.db.Delete(&model.IntegrationAIKnowledgeDocument{}, id).Error
}

func filepathExt(name string) string {
	idx := strings.LastIndex(name, ".")
	if idx < 0 {
		return ""
	}
	return name[idx:]
}

func (s *Service) ListIntegrationAIModels() ([]map[string]any, error) {
	var list []model.IntegrationAIModel
	if err := s.db.Order("is_default DESC, id ASC").Find(&list).Error; err != nil {
		return nil, err
	}
	result := make([]map[string]any, 0, len(list))
	for _, item := range list {
		result = append(result, aiModelView(item))
	}
	return result, nil
}

func aiModelView(item model.IntegrationAIModel) map[string]any {
	return map[string]any{"id": item.ID, "name": item.Name, "provider": item.Provider, "baseUrl": item.BaseURL,
		"model": item.Model, "systemPrompt": item.SystemPrompt, "temperature": item.Temperature, "maxTokens": item.MaxTokens,
		"timeoutSeconds": item.TimeoutSeconds, "isDefault": item.IsDefault, "status": item.Status, "description": item.Description,
		"hasApiKey": strings.TrimSpace(item.APIKey) != "", "apiKeyMasked": maskSecret(item.APIKey), "createTime": item.CreatedAt, "updateTime": item.UpdatedAt}
}

func maskSecret(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if len(value) <= 8 {
		return "********"
	}
	return value[:3] + strings.Repeat("*", 8) + value[len(value)-4:]
}

func (s *Service) SaveIntegrationAIModel(payload IntegrationAIModelPayload) (map[string]any, error) {
	payload.Name = strings.TrimSpace(payload.Name)
	payload.BaseURL = strings.TrimRight(strings.TrimSpace(payload.BaseURL), "/")
	payload.Model = strings.TrimSpace(payload.Model)
	if payload.Name == "" || payload.BaseURL == "" || payload.Model == "" {
		return nil, errors.New("model name, API URL, and model identifier are required")
	}
	if parsed, err := url.Parse(payload.BaseURL); err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, errors.New("enter a valid OpenAI-compatible API URL")
	}
	if payload.Provider == "" {
		payload.Provider = "openai_compatible"
	}
	if payload.Status == 0 {
		payload.Status = 1
	}
	if payload.TimeoutSeconds < 5 {
		payload.TimeoutSeconds = 60
	}
	if payload.TimeoutSeconds > 600 {
		payload.TimeoutSeconds = 600
	}
	if payload.MaxTokens <= 0 {
		payload.MaxTokens = 2048
	}
	if payload.MaxTokens > 393216 {
		payload.MaxTokens = 393216
	}
	if payload.Temperature < 0 {
		payload.Temperature = 0
	}
	if payload.Temperature > 2 {
		payload.Temperature = 2
	}
	var item model.IntegrationAIModel
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if payload.IsDefault {
			if err := tx.Model(&model.IntegrationAIModel{}).Where("is_default = ?", true).Update("is_default", false).Error; err != nil {
				return err
			}
		}
		if payload.ID > 0 {
			if err := tx.First(&item, payload.ID).Error; err != nil {
				return err
			}
			item.Name, item.Provider, item.BaseURL, item.Model = payload.Name, payload.Provider, payload.BaseURL, payload.Model
			item.SystemPrompt, item.Temperature, item.MaxTokens = payload.SystemPrompt, payload.Temperature, payload.MaxTokens
			item.TimeoutSeconds, item.IsDefault, item.Status, item.Description = payload.TimeoutSeconds, payload.IsDefault, payload.Status, payload.Description
			if strings.TrimSpace(payload.APIKey) != "" {
				item.APIKey = strings.TrimSpace(payload.APIKey)
			}
			return tx.Save(&item).Error
		}
		item = model.IntegrationAIModel{Name: payload.Name, Provider: payload.Provider, BaseURL: payload.BaseURL, APIKey: strings.TrimSpace(payload.APIKey), Model: payload.Model,
			SystemPrompt: payload.SystemPrompt, Temperature: payload.Temperature, MaxTokens: payload.MaxTokens, TimeoutSeconds: payload.TimeoutSeconds,
			IsDefault: payload.IsDefault, Status: payload.Status, Description: payload.Description}
		return tx.Create(&item).Error
	})
	if err != nil {
		return nil, err
	}
	return aiModelView(item), nil
}

func (s *Service) DeleteIntegrationAIModel(id uint) error {
	if id == 0 {
		return errors.New("model ID is required")
	}
	var count int64
	if err := s.db.Model(&model.IntegrationAIConversation{}).Where("model_id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return errors.New("model is used by conversations and cannot be deleted directly; disable it first")
	}
	return s.db.Delete(&model.IntegrationAIModel{}, id).Error
}

func (s *Service) TestIntegrationAIModel(payload IntegrationAIModelPayload) (map[string]any, error) {
	var item model.IntegrationAIModel
	if payload.ID > 0 {
		if err := s.db.First(&item, payload.ID).Error; err != nil {
			return nil, err
		}
	}
	if strings.TrimSpace(payload.BaseURL) != "" {
		item.BaseURL = strings.TrimRight(strings.TrimSpace(payload.BaseURL), "/")
	}
	if strings.TrimSpace(payload.Model) != "" {
		item.Model = strings.TrimSpace(payload.Model)
	}
	if strings.TrimSpace(payload.APIKey) != "" {
		item.APIKey = strings.TrimSpace(payload.APIKey)
	}
	if payload.TimeoutSeconds > 0 {
		item.TimeoutSeconds = payload.TimeoutSeconds
	}
	if payload.MaxTokens > 0 {
		item.MaxTokens = payload.MaxTokens
	} else if item.MaxTokens <= 0 {
		item.MaxTokens = 2048
	}
	if payload.Temperature >= 0 && payload.Temperature <= 2 {
		item.Temperature = payload.Temperature
	}
	started := time.Now()
	response, err := s.callOpenAICompatible(item, []map[string]any{{"role": "user", "content": "Reply only with OK"}}, nil)
	if err != nil {
		return nil, err
	}
	return map[string]any{"success": true, "latencyMs": time.Since(started).Milliseconds(), "response": response.Content}, nil
}

func (s *Service) ListIntegrationAIConversations(userID uint, keyword string) ([]map[string]any, error) {
	query := s.db.Model(&model.IntegrationAIConversation{}).Where("user_id = ?", userID)
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		query = query.Where("title LIKE ?", "%"+keyword+"%")
	}
	var list []model.IntegrationAIConversation
	if err := query.Order("pinned DESC, COALESCE(last_message_at, created_at) DESC").Find(&list).Error; err != nil {
		return nil, err
	}
	modelNames := map[uint]string{}
	var models []model.IntegrationAIModel
	_ = s.db.Find(&models).Error
	for _, item := range models {
		modelNames[item.ID] = item.Name
	}
	result := make([]map[string]any, 0, len(list))
	for _, item := range list {
		result = append(result, map[string]any{
			"id": item.ID, "title": item.Title, "modelId": item.ModelID, "modelName": modelNames[item.ModelID], "username": item.Username,
			"status": item.Status, "pinned": item.Pinned, "messageCount": item.MessageCount, "lastMessageAt": item.LastMessageAt, "createTime": item.CreatedAt, "updateTime": item.UpdatedAt})
	}
	return result, nil
}

func (s *Service) SaveIntegrationAIConversation(userID uint, username string, payload IntegrationAIConversationPayload) (*model.IntegrationAIConversation, error) {
	title := strings.TrimSpace(payload.Title)
	if title == "" {
		title = "새 대화"
	}
	if payload.ModelID == 0 {
		payload.ModelID = s.defaultAIModelID()
	}
	if payload.ID > 0 {
		var item model.IntegrationAIConversation
		if err := s.db.Where("id = ? AND user_id = ?", payload.ID, userID).First(&item).Error; err != nil {
			return nil, err
		}
		item.Title, item.ModelID, item.Pinned = title, payload.ModelID, payload.Pinned
		if err := s.db.Save(&item).Error; err != nil {
			return nil, err
		}
		return &item, nil
	}
	item := model.IntegrationAIConversation{UserID: userID, Username: username, ModelID: payload.ModelID, Title: title, Status: 1, Pinned: payload.Pinned}
	if err := s.db.Create(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Service) defaultAIModelID() uint {
	var item model.IntegrationAIModel
	if s.db.Where("status = ?", 1).Order("is_default DESC, id ASC").First(&item).Error == nil {
		return item.ID
	}
	return 0
}

func (s *Service) GetIntegrationAIConversation(userID, id uint) (map[string]any, error) {
	var conversation model.IntegrationAIConversation
	if err := s.db.Where("id = ? AND user_id = ?", id, userID).First(&conversation).Error; err != nil {
		return nil, err
	}
	var messages []model.IntegrationAIMessage
	if err := s.db.Where("conversation_id = ?", id).Order("id ASC").Find(&messages).Error; err != nil {
		return nil, err
	}
	var actions []model.IntegrationAIToolAction
	_ = s.db.Where("conversation_id = ?", id).Order("id ASC").Find(&actions).Error
	for index := range messages {
		messages[index].Content = sanitizeAIMessageContent(messages[index].Content)
	}
	return map[string]any{"conversation": conversation, "messages": messages, "actions": actions}, nil
}

func (s *Service) DeleteIntegrationAIConversation(userID, id uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var item model.IntegrationAIConversation
		if err := tx.Where("id = ? AND user_id = ?", id, userID).First(&item).Error; err != nil {
			return err
		}
		if err := tx.Where("conversation_id = ?", id).Delete(&model.IntegrationAIToolAction{}).Error; err != nil {
			return err
		}
		if err := tx.Where("conversation_id = ?", id).Delete(&model.IntegrationAIMessage{}).Error; err != nil {
			return err
		}
		return tx.Delete(&item).Error
	})
}

func (s *Service) SendIntegrationAIChat(userID uint, username string, payload IntegrationAIChatPayload) (map[string]any, error) {
	content := strings.TrimSpace(payload.Content)
	if content == "" {
		return nil, errors.New("conversation content is required")
	}
	var conversation model.IntegrationAIConversation
	if payload.ConversationID == 0 {
		item, err := s.SaveIntegrationAIConversation(userID, username, IntegrationAIConversationPayload{ModelID: payload.ModelID, Title: truncateRunes(content, 40)})
		if err != nil {
			return nil, err
		}
		conversation = *item
	} else if err := s.db.Where("id = ? AND user_id = ?", payload.ConversationID, userID).First(&conversation).Error; err != nil {
		return nil, err
	}
	if payload.ModelID > 0 && payload.ModelID != conversation.ModelID {
		conversation.ModelID = payload.ModelID
		_ = s.db.Model(&conversation).Update("model_id", payload.ModelID).Error
	}
	var aiModel model.IntegrationAIModel
	if err := s.db.Where("id = ? AND status = ?", conversation.ModelID, 1).First(&aiModel).Error; err != nil {
		return nil, errors.New("select an enabled AI model")
	}
	userMessage := model.IntegrationAIMessage{ConversationID: conversation.ID, Role: "user", Content: content, Status: "completed"}
	if err := s.db.Create(&userMessage).Error; err != nil {
		return nil, err
	}
	var history []model.IntegrationAIMessage
	if err := s.db.Where("conversation_id = ? AND role IN ?", conversation.ID, []string{"user", "assistant"}).Order("id DESC").Limit(30).Find(&history).Error; err != nil {
		return nil, err
	}
	sort.Slice(history, func(i, j int) bool { return history[i].ID < history[j].ID })
	messages := []map[string]any{{"role": "system", "content": integrationAISystemPrompt(aiModel.SystemPrompt)}}
	for _, item := range history {
		messages = append(messages, map[string]any{"role": item.Role, "content": sanitizeAIMessageContent(item.Content)})
	}
	tools, configs, err := s.openAIToolDefinitions()
	if err != nil {
		return nil, err
	}
	response, err := s.callOpenAICompatible(aiModel, messages, tools)
	if err != nil {
		return nil, err
	}
	if len(response.ToolCalls) == 0 && hasUnsupportedAIToolProtocol(response.Content) {
		repairMessages := append([]map[string]any{}, messages...)
		repairMessages = append(repairMessages,
			map[string]any{"role": "assistant", "content": response.Content},
			map[string]any{"role": "user", "content": "Internal tool-call markers and XML/DSML must not be exposed. Use only provided native tools; when no tool is available, answer concisely in Korean."},
		)
		response, err = s.callOpenAICompatible(aiModel, repairMessages, tools)
		if err != nil {
			return nil, err
		}
	}
	actions := make([]model.IntegrationAIToolAction, 0)
	finOpsToolUsed := false
	finOpsInstructionAdded := false
	knowledgeBaseToolUsed := false
	knowledgeBaseInstructionAdded := false
	for round := 0; round < 3 && len(response.ToolCalls) > 0; round++ {
		assistantCall := map[string]any{"role": "assistant", "content": response.Content, "tool_calls": response.RawToolCalls}
		messages = append(messages, assistantCall)
		for _, call := range response.ToolCalls {
			config, exists := configs[call.Name]
			if !exists || !config.Enabled {
				continue
			}
			var args map[string]any
			if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
				args = map[string]any{}
			}
			var toolResult any
			if config.RequireConfirmation {
				rawArgs, _ := json.Marshal(args)
				action := model.IntegrationAIToolAction{ConversationID: conversation.ID, UserID: userID, ToolKey: call.Name, ArgumentsJSON: string(rawArgs), Status: "pending"}
				if err := s.db.Create(&action).Error; err != nil {
					return nil, err
				}
				actions = append(actions, action)
				toolResult = map[string]any{"status": "pending_confirmation", "actionId": action.ID, "message": "사용자 확인 후 실행할 수 있습니다."}
			} else {
				toolResult, err = s.executeIntegrationAITool(call.Name, args)
				if err != nil {
					toolResult = map[string]any{"error": err.Error()}
				}
			}
			if call.Name == "finops_cost_analysis" {
				finOpsToolUsed = true
			}
			if call.Name == "knowledge_base_search" {
				knowledgeBaseToolUsed = true
			}
			rawResult, _ := json.Marshal(toolResult)
			messages = append(messages, map[string]any{"role": "tool", "tool_call_id": call.ID, "content": string(rawResult)})
		}
		if finOpsToolUsed && !finOpsInstructionAdded {
			messages = append(messages, map[string]any{"role": "system", "content": finOpsChatResponseInstruction})
			finOpsInstructionAdded = true
		}
		if knowledgeBaseToolUsed && !knowledgeBaseInstructionAdded {
			messages = append(messages, map[string]any{"role": "system", "content": knowledgeBaseChatResponseInstruction})
			knowledgeBaseInstructionAdded = true
		}
		// A knowledge-base lookup already returns the relevant local excerpt.  Do
		// not let a tool-capable model repeatedly issue the same search instead of
		// composing an answer from that excerpt.
		nextTools := tools
		if knowledgeBaseToolUsed {
			nextTools = nil
		}
		response, err = s.callOpenAICompatible(aiModel, messages, nextTools)
		if err != nil {
			return nil, err
		}
	}
	if len(response.ToolCalls) > 0 && strings.TrimSpace(response.Content) == "" {
		response.Content = "Tool 호출이 안전 제한에 도달해 실행을 중단했습니다. Query 범위를 줄여 다시 시도하십시오."
	}
	if strings.TrimSpace(response.Content) == "" {
		response.Content = "작업을 생성했습니다. 아래에서 확인한 뒤 실행하십시오."
	}
	if hasUnsupportedAIToolProtocol(response.Content) {
		response.Content = "Model이 지원되지 않는 내부 Tool 호출 형식을 반환하여 아무 작업도 실행하지 않았습니다. 질문을 다시 작성하거나 Native Tool Calling을 지원하는 Model로 전환하십시오."
	}
	assistantMessage := model.IntegrationAIMessage{ConversationID: conversation.ID, Role: "assistant", Content: response.Content, Status: "completed"}
	if err := s.db.Create(&assistantMessage).Error; err != nil {
		return nil, err
	}
	for i := range actions {
		actions[i].MessageID = assistantMessage.ID
		_ = s.db.Model(&actions[i]).Update("message_id", assistantMessage.ID).Error
	}
	now := time.Now()
	updates := map[string]any{"message_count": gorm.Expr("message_count + ?", 2), "last_message_at": &now}
	if conversation.Title == "새 대화" {
		updates["title"] = truncateRunes(content, 40)
	}
	_ = s.db.Model(&conversation).Updates(updates).Error
	return map[string]any{"conversationId": conversation.ID, "message": assistantMessage, "actions": actions}, nil
}

func truncateRunes(value string, limit int) string {
	chars := []rune(strings.TrimSpace(value))
	if len(chars) <= limit {
		return string(chars)
	}
	return string(chars[:limit]) + "..."
}

func integrationAISystemPrompt(custom string) string {
	base := "당신은 Ops Admin Platform의 DevOps/SRE Assistant입니다. 답변은 한국어로 작성하고 결론, Evidence, 권고 작업 순서로 설명하십시오. 표준 Markdown을 사용하되 내부 XML, DSML, tool_calls, invoke 또는 기타 Tool Protocol을 노출하지 마십시오. Production 변경은 Risk를 명시하고 실제로 실행하지 않은 작업을 실행했다고 주장하지 마십시오. Cloud 비용 Question에는 Local Billing Data만 사용하고 Cloud Provider API를 호출하거나 Billing Data를 실시간 Cloud 상태로 표현하지 마십시오."
	base += "\n\nMonitoring Skill 규칙: PromQL 요청은 Expression을 먼저 생성하고 필요하면 prometheus_query로 검증합니다. Alert 조회는 monitor_alert_event_query, Datasource 조회는 monitor_datasource_query, Host Issue는 host_health_diagnose, 종합 장애는 ops_troubleshooting, Dashboard Issue는 monitor_dashboard_analyze를 우선 사용합니다. 분석은 Tool Evidence를 근거로 하고 확인된 사실과 추정을 구분합니다. Alert 생성은 monitor_alert_rule_draft만 사용하며 사용자 확인 후 비활성 Draft로 저장합니다."
	if strings.TrimSpace(custom) != "" {
		return base + "\n\n추가 지침:\n" + strings.TrimSpace(custom)
	}
	return base
}

func hasUnsupportedAIToolProtocol(content string) bool {
	value := strings.ToLower(content)
	compact := strings.NewReplacer(" ", "", "\n", "", "\t", "").Replace(value)
	return strings.Contains(value, "dsml") || strings.Contains(compact, "<tool_calls>") || strings.Contains(compact, "<invokename=")
}

func sanitizeAIMessageContent(content string) string {
	if !hasUnsupportedAIToolProtocol(content) {
		return content
	}
	return "이 History Message에는 Model이 지원하지 않는 내부 Tool 호출 형식이 포함되어 있어 아무 작업도 실행하지 않았습니다. Query를 다시 실행하십시오."
}

const finOpsChatResponseInstruction = "Cloud Cost Tool이 결과를 반환했습니다. 8줄 이내의 간결한 한국어 Markdown으로 답하십시오. 일반 비용 Question은 Billing Month와 Account, 총 비용, Top 3 Product, Region 요약, 최대 3개 확인 항목을 포함합니다. 실시간 Monitoring Data가 없으면 유휴 상태를 단정하지 말고 검증 필요성을 명시하십시오. Instance 수 또는 Instance별 비용 Question에는 resourceBreakdown을 우선 사용하고 최대 5개 Instance 이름/ID와 비용을 보여주십시오. Data Source가 Local Billing임을 마지막에 명시하십시오."

const knowledgeBaseChatResponseInstruction = "Knowledge Base Search Tool이 Local Markdown Fragment를 반환했습니다. 문서 목차를 나열하지 말고 User Question에 맞게 재구성하십시오. 1~2문장 결론 뒤에 Platform Position, 즉시 사용 가능한 Capability, 권장 사용 경로 순서로 설명하고 구현 완료와 제안을 명확히 구분하십시오. Source 또는 Original Text를 요청한 경우에만 문서 이름이나 Quote를 표시하십시오."

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

func parseAIQueryTime(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"} {
		if parsed, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func (s *Service) queryAIHostHealth(args map[string]any) (map[string]any, error) {
	hostID := anyUint(args["hostId"])
	var host model.AssetHost
	if hostID > 0 {
		if err := s.db.First(&host, hostID).Error; err != nil {
			return nil, err
		}
	} else {
		keyword := strings.TrimSpace(anyString(args["keyword"]))
		if keyword == "" {
			return nil, errors.New("provide hostId or a host keyword")
		}
		like := "%" + keyword + "%"
		if err := s.db.Where("host_name LIKE ? OR alias LIKE ? OR private_ip LIKE ? OR public_ip LIKE ? OR ssh_ip LIKE ?", like, like, like, like, like).Order("id DESC").First(&host).Error; err != nil {
			return nil, err
		}
	}
	metrics, err := s.GetAssetHostMetrics(host.ID, firstNonEmpty(strings.TrimSpace(anyString(args["range"])), "24h"), "", "")
	if err != nil {
		return nil, err
	}
	alerts, err := s.queryAIMonitorAlertEvents(map[string]any{"keyword": firstNonEmpty(host.PrivateIP, host.PublicIP, host.HostName), "limit": 10})
	if err != nil {
		return nil, err
	}
	return map[string]any{"host": map[string]any{"id": host.ID, "name": host.HostName, "alias": host.Alias, "privateIp": host.PrivateIP, "publicIp": host.PublicIP, "environment": host.Environment, "online": host.AliveStatus == 1, "enabled": host.Status == 1, "lastCheckTime": host.LastCheckTime}, "metrics": metrics, "relatedAlerts": alerts}, nil
}

func (s *Service) queryAIOpsTroubleshooting(args map[string]any) (map[string]any, error) {
	result := map[string]any{"scope": "read_only", "evidence": map[string]any{}}
	evidence := result["evidence"].(map[string]any)
	if eventID := anyUint(args["alertEventId"]); eventID > 0 {
		detail, err := s.GetMonitorAlertEventDetail(eventID)
		if err != nil {
			return nil, err
		}
		evidence["alertEvent"] = detail
	}
	keyword := strings.TrimSpace(anyString(args["keyword"]))
	if keyword != "" {
		alerts, err := s.queryAIMonitorAlertEvents(map[string]any{"keyword": keyword, "limit": 10})
		if err != nil {
			return nil, err
		}
		evidence["matchingAlerts"] = alerts
	}
	if host := strings.TrimSpace(anyString(args["host"])); host != "" {
		health, err := s.queryAIHostHealth(map[string]any{"keyword": host, "range": anyString(args["range"])})
		if err != nil {
			return nil, err
		}
		evidence["hostHealth"] = health
	}
	datasources, err := s.queryAIMonitorDatasources(map[string]any{"limit": 20})
	if err != nil {
		return nil, err
	}
	evidence["datasources"] = datasources
	return result, nil
}

func (s *Service) queryAIMonitorDashboard(args map[string]any) (any, error) {
	if dashboardID := anyUint(args["dashboardId"]); dashboardID > 0 {
		return s.GetMonitorDashboard(dashboardID)
	}
	return s.ListMonitorDashboards(1, 20, anyString(args["keyword"]), "1")
}

func (s *Service) queryAIKnowledgeBase(args map[string]any) (map[string]any, error) {
	keyword := strings.TrimSpace(anyString(args["keyword"]))
	if keyword == "" {
		return nil, errors.New("knowledge-base search keyword is required")
	}
	limit := int(anyUint(args["limit"]))
	if limit == 0 {
		limit = 5
	}
	if limit < 1 || limit > 10 {
		return nil, errors.New("limit must be between 1 and 10")
	}
	query := s.db.Where("status = ?", 1)
	if documentID := anyUint(args["documentId"]); documentID > 0 {
		query = query.Where("id = ?", documentID)
	}
	like := "%" + keyword + "%"
	var documents []model.IntegrationAIKnowledgeDocument
	if err := query.Where("name LIKE ? OR content LIKE ?", like, like).Order("updated_at DESC, id DESC").Limit(limit).Find(&documents).Error; err != nil {
		return nil, err
	}
	items := make([]map[string]any, 0, len(documents))
	for _, item := range documents {
		items = append(items, map[string]any{
			"documentId": item.ID, "name": item.Name, "fileName": item.FileName,
			"excerpt": knowledgeExcerpt(item.Content, keyword, 560), "updatedAt": item.UpdatedAt,
		})
	}
	return map[string]any{"source": "local_knowledge_base", "keyword": keyword, "total": len(items), "items": items}, nil
}

func knowledgeExcerpt(content, keyword string, maxRunes int) string {
	runes := []rune(content)
	needle := []rune(strings.ToLower(keyword))
	start := 0
	if len(needle) > 0 {
		lower := []rune(strings.ToLower(content))
		for i := 0; i+len(needle) <= len(lower); i++ {
			matched := true
			for j := range needle {
				if lower[i+j] != needle[j] {
					matched = false
					break
				}
			}
			if matched {
				start = i - 120
				if start < 0 {
					start = 0
				}
				break
			}
		}
	}
	end := start + maxRunes
	if end > len(runes) {
		end = len(runes)
	}
	result := strings.TrimSpace(string(runes[start:end]))
	if start > 0 {
		result = "…" + result
	}
	if end < len(runes) {
		result += "…"
	}
	return result
}

// queryAIFinOpsAnalysis only reads the local FinOps tables populated by the
// account/synchronization workflows. It must never invoke a cloud billing API.
func (s *Service) queryAIFinOpsAnalysis(args map[string]any) (map[string]any, error) {
	analysisMonth := strings.TrimSpace(anyString(args["month"]))
	monthSource := "specified"
	if analysisMonth == "" {
		latestMonth, err := s.LatestFinOpsBreakdownMonth(anyUint(args["accountId"]))
		if err != nil {
			return nil, err
		}
		if latestMonth != "" {
			analysisMonth = latestMonth
			monthSource = "latest_synced"
		} else {
			analysisMonth = time.Now().Format("2006-01")
			monthSource = "current_month_no_local_bill"
		}
	}
	monthStart, err := parseFinOpsMonth(analysisMonth)
	if err != nil {
		return nil, errors.New("invalid month parameter; expected YYYY-MM")
	}
	accountID := anyUint(args["accountId"])
	trendMonths := int(anyUint(args["trendMonths"]))
	if trendMonths == 0 {
		trendMonths = 6
	}
	if trendMonths < 1 || trendMonths > 12 {
		return nil, errors.New("trendMonths must be between 1 and 12")
	}

	monthEnd := monthStart.AddDate(0, 1, 0)
	now := time.Now()
	if now.After(monthStart) && now.Before(monthEnd) {
		monthEnd = now
	}
	trendStart := monthStart.AddDate(0, -trendMonths+1, 0)
	dashboard, err := s.FinOpsDashboard(trendStart, monthEnd, accountID)
	if err != nil {
		return nil, err
	}
	selectedMonthSummary, err := s.FinOpsDashboard(monthStart, monthEnd, accountID)
	if err != nil {
		return nil, err
	}
	services, err := s.FinOpsBreakdown(monthStart, monthEnd, "service", accountID)
	if err != nil {
		return nil, err
	}
	regions, err := s.FinOpsBreakdown(monthStart, monthEnd, "region", accountID)
	if err != nil {
		return nil, err
	}
	serviceKeyword := strings.TrimSpace(anyString(args["service"]))
	includeResources := serviceKeyword != "" || anyBool(args["includeResourceBreakdown"])
	resourceBreakdown := map[string]any{"requested": includeResources}
	if includeResources {
		resourceBreakdown, err = s.queryAIFinOpsResourceBreakdown(monthStart, monthEnd, accountID, serviceKeyword)
		if err != nil {
			return nil, err
		}
	}
	return map[string]any{
		"source":               "local_synced_finops_database",
		"sourceDescription":    "로컬에 동기화된 Cloud Billing만 사용했으며 Cloud Provider API는 호출하지 않았습니다.",
		"analysisMonth":        analysisMonth,
		"monthSource":          monthSource,
		"accountId":            accountID,
		"trendMonths":          trendMonths,
		"trendDashboard":       dashboard,
		"selectedMonthSummary": selectedMonthSummary,
		"serviceBreakdown":     limitAIFinOpsRows(services, 12),
		"regionBreakdown":      limitAIFinOpsRows(regions, 12),
		"serviceFilter":        serviceKeyword,
		"resourceBreakdown":    resourceBreakdown,
	}, nil
}

func limitAIFinOpsRows(rows []map[string]any, limit int) []map[string]any {
	if len(rows) <= limit {
		return rows
	}
	return rows[:limit]
}

// queryAIFinOpsResourceBreakdown aggregates only persisted cost records. A row
// without resource ID/name is retained as unattributed cost instead of being
// invented as an instance.
func (s *Service) queryAIFinOpsResourceBreakdown(start, end time.Time, accountID uint, serviceKeyword string) (map[string]any, error) {
	records, _, err := s.finOpsRecords(start, end, accountID)
	if err != nil {
		return nil, err
	}
	keyword := strings.ToLower(strings.TrimSpace(serviceKeyword))
	rowsByResource := map[string]map[string]any{}
	unattributedCost := 0.0
	matchedRecordCount := 0
	for _, record := range records {
		if keyword != "" && !strings.Contains(strings.ToLower(record.Service), keyword) {
			continue
		}
		matchedRecordCount++
		resourceID, resourceName := strings.TrimSpace(record.ResourceID), strings.TrimSpace(record.ResourceName)
		if resourceID == "" && resourceName == "" {
			unattributedCost += record.Amount
			continue
		}
		key := resourceID
		if key == "" {
			key = "name:" + resourceName
		}
		row := rowsByResource[key]
		if row == nil {
			row = map[string]any{"resourceId": resourceID, "resourceName": resourceName, "service": record.Service, "region": record.Region, "amount": 0.0, "recordCount": 0}
			rowsByResource[key] = row
		}
		row["amount"] = row["amount"].(float64) + record.Amount
		row["recordCount"] = row["recordCount"].(int) + 1
	}
	items := make([]map[string]any, 0, len(rowsByResource))
	for _, row := range rowsByResource {
		items = append(items, row)
	}
	sort.Slice(items, func(i, j int) bool { return items[i]["amount"].(float64) > items[j]["amount"].(float64) })
	return map[string]any{
		"requested":          true,
		"serviceFilter":      serviceKeyword,
		"matchedRecordCount": matchedRecordCount,
		"resourceCount":      len(items),
		"unattributedCost":   unattributedCost,
		"items":              limitAIFinOpsRows(items, 20),
		"sourceDescription":  "Local Billing의 resourceId/resourceName 기준으로 집계했으며 Cloud Provider API는 조회하지 않습니다.",
	}, nil
}

func (s *Service) queryAIRealtimeLogs(args map[string]any, now time.Time) (map[string]any, error) {
	startAt, err := parseAILogTime(anyString(args["startTime"]), now)
	if err != nil {
		return nil, fmt.Errorf("invalid start time: %w", err)
	}
	endAt, err := parseAILogTime(anyString(args["endTime"]), now)
	if err != nil {
		return nil, fmt.Errorf("invalid end time: %w", err)
	}
	if !endAt.After(startAt) {
		return nil, errors.New("end time must be later than start time")
	}
	if endAt.Sub(startAt) > 31*24*time.Hour {
		return nil, errors.New("a single log query cannot span more than 31 days")
	}

	datasourceID := anyUint(args["datasourceId"])
	datasourceName := strings.TrimSpace(anyString(args["datasourceName"]))
	query := strings.TrimSpace(anyString(args["query"]))
	index := strings.TrimSpace(anyString(args["index"]))
	streams := splitAIQueryValues(anyString(args["streams"]))
	mode := strings.ToLower(strings.TrimSpace(anyString(args["mode"])))
	if mode == "" {
		mode = "count"
	}
	if mode != "count" && mode != "list" {
		return nil, errors.New("return mode supports only count or list")
	}
	limit := aiAssetQueryLimit(args["limit"])
	if mode == "count" {
		limit = 1
	}

	var datasources []model.MonitorDatasource
	datasourceQuery := s.db.Where("status = ? AND type IN ?", 1, []string{"elasticsearch", "victorialogs"})
	if datasourceID > 0 {
		datasourceQuery = datasourceQuery.Where("id = ?", datasourceID)
	} else if datasourceName != "" {
		datasourceQuery = datasourceQuery.Where("name LIKE ?", "%"+datasourceName+"%")
	}
	if err := datasourceQuery.Order("is_default DESC, id ASC").Find(&datasources).Error; err != nil {
		return nil, err
	}
	if len(datasources) == 0 {
		return nil, errors.New("no matching enabled Elasticsearch or VictoriaLogs datasource was found")
	}

	results := make([]map[string]any, 0, len(datasources))
	errorsByDatasource := make([]string, 0)
	var total int64
	for _, datasource := range datasources {
		datasourceQueryText := query
		if normalizeMonitorDatasourceType(datasource.Type) == "victorialogs" {
			datasourceQueryText = appendAILogStreamFilter(datasourceQueryText, streams, true)
		} else {
			datasourceQueryText = appendAILogStreamFilter(datasourceQueryText, streams, false)
		}
		payload := MonitorLogQueryPayload{
			DatasourceID:   datasource.ID,
			Index:          index,
			Query:          datasourceQueryText,
			StartAt:        startAt.UnixMilli(),
			EndAt:          endAt.UnixMilli(),
			PageSize:       limit,
			TrackTotalHits: mode == "count",
		}
		data, queryErr := s.QueryMonitorLogs(payload)
		if queryErr != nil {
			errorsByDatasource = append(errorsByDatasource, fmt.Sprintf("%s: %v", datasource.Name, queryErr))
			continue
		}
		count := aiLogTotal(data["total"])
		total += count
		item := map[string]any{
			"datasourceId": datasource.ID,
			"datasource":   datasource.Name,
			"type":         normalizeMonitorDatasourceType(datasource.Type),
			"count":        count,
			"tookMs":       data["took"],
		}
		if mode == "list" {
			item["items"] = data["items"]
		}
		results = append(results, item)
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("all log datasource queries failed: %s", strings.Join(errorsByDatasource, "; "))
	}
	return map[string]any{
		"mode": mode, "total": total, "query": query, "index": index, "streams": streams,
		"startTime": startAt.Format(time.RFC3339), "endTime": endAt.Format(time.RFC3339),
		"timezone": "Asia/Shanghai", "datasources": results, "errors": errorsByDatasource,
	}, nil
}

func parseAILogTime(value string, now time.Time) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, errors.New("time is required")
	}
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		location = time.Local
	}
	now = now.In(location)
	lower := strings.ToLower(value)
	for _, relative := range []struct {
		prefix string
		days   int
	}{
		{prefix: "어제", days: -1}, {prefix: "\u6628\u5929", days: -1}, {prefix: "yesterday", days: -1},
		{prefix: "오늘", days: 0}, {prefix: "\u4eca\u5929", days: 0}, {prefix: "today", days: 0},
	} {
		if strings.HasPrefix(lower, relative.prefix) {
			clock := strings.TrimSpace(value[len(relative.prefix):])
			if clock == "" {
				clock = "00:00"
			}
			parsedClock, parseErr := time.ParseInLocation("15:04:05", clock, location)
			if parseErr != nil {
				parsedClock, parseErr = time.ParseInLocation("15:04", clock, location)
			}
			if parseErr != nil {
				return time.Time{}, errors.New("relative time must use the form ‘어제 10:00’ or ‘today 10:00’")
			}
			date := now.AddDate(0, 0, relative.days)
			return time.Date(date.Year(), date.Month(), date.Day(), parsedClock.Hour(), parsedClock.Minute(), parsedClock.Second(), 0, location), nil
		}
	}
	if parsed, parseErr := time.Parse(time.RFC3339, value); parseErr == nil {
		return parsed.In(location), nil
	}
	for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02 15:04", "2006/01/02 15:04:05", "2006/01/02 15:04"} {
		if parsed, parseErr := time.ParseInLocation(layout, value, location); parseErr == nil {
			return parsed, nil
		}
	}
	return time.Time{}, errors.New("supported formats are RFC3339, YYYY-MM-DD HH:mm:ss, or ‘어제 10:00’")
}

func splitAIQueryValues(value string) []string {
	fields := strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == '，' || r == '\n' })
	result := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field != "" && !aiStringSliceContains(result, field) {
			result = append(result, field)
		}
	}
	return result
}

func appendAILogStreamFilter(query string, streams []string, victoriaLogs bool) string {
	query = strings.TrimSpace(query)
	if len(streams) == 0 {
		return query
	}
	if victoriaLogs {
		return appendAIVictoriaLogsStreamFilter(query, streams)
	}
	filters := make([]string, 0, len(streams))
	for _, rawStream := range streams {
		stream := strings.TrimSpace(rawStream)
		exact := strings.HasPrefix(stream, "=")
		if exact {
			stream = strings.TrimSpace(strings.TrimPrefix(stream, "="))
		}
		if stream == "" {
			continue
		}
		if exact {
			escaped := strings.ReplaceAll(strings.ReplaceAll(stream, "\\", "\\\\"), "\"", "\\\"")
			filters = append(filters, "kafka_topic:\""+escaped+"\"")
			continue
		}
		filters = append(filters, "kafka_topic:*"+escapeAILuceneWildcardValue(stream)+"*")
	}
	if len(filters) == 0 {
		return query
	}
	streamQuery := "(" + strings.Join(filters, " OR ") + ")"
	if query == "" {
		return streamQuery
	}
	return "(" + query + ") AND " + streamQuery
}

func appendAIVictoriaLogsStreamFilter(query string, streams []string) string {
	filters := make([]string, 0, len(streams))
	for _, rawStream := range streams {
		stream := strings.TrimSpace(rawStream)
		exact := strings.HasPrefix(stream, "=")
		if exact {
			stream = strings.TrimSpace(strings.TrimPrefix(stream, "="))
		}
		if stream == "" {
			continue
		}
		if exact {
			escaped := strings.ReplaceAll(strings.ReplaceAll(stream, "\\", "\\\\"), "\"", "\\\"")
			filters = append(filters, `{kafka_topic="`+escaped+`"}`)
		} else {
			filters = append(filters, "kafka_topic:*"+escapeAILuceneWildcardValue(stream)+"*")
		}
	}
	if len(filters) == 0 {
		return query
	}
	streamQuery := "(" + strings.Join(filters, " OR ") + ")"
	if query == "" || query == "*" {
		return streamQuery
	}
	return streamQuery + " AND (" + query + ")"
}

func escapeAILuceneWildcardValue(value string) string {
	const reserved = `+-=&|><!(){}[]^"~*?:\/`
	var builder strings.Builder
	for _, char := range value {
		if strings.ContainsRune(reserved, char) {
			builder.WriteByte('\\')
		}
		builder.WriteRune(char)
	}
	return builder.String()
}

func aiLogTotal(value any) int64 {
	switch typed := value.(type) {
	case int64:
		return typed
	case int:
		return int64(typed)
	case float64:
		return int64(typed)
	case json.Number:
		result, _ := typed.Int64()
		return result
	case map[string]any:
		return aiLogTotal(typed["value"])
	default:
		return 0
	}
}

func aiAssetQueryLimit(value any) int {
	limit := int(anyUint(value))
	if limit < 1 {
		return 20
	}
	if limit > 50 {
		return 50
	}
	return limit
}

func (s *Service) queryAIAssetHosts(args map[string]any) (map[string]any, error) {
	keyword := strings.TrimSpace(anyString(args["keyword"]))
	environment := strings.TrimSpace(anyString(args["environment"]))
	limit := aiAssetQueryLimit(args["limit"])
	query := s.db.Model(&model.AssetHost{}).Preload("Group").Preload("HostGroups")
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("host_name LIKE ? OR alias LIKE ? OR private_ip LIKE ? OR public_ip LIKE ? OR ssh_ip LIKE ?", like, like, like, like, like)
	}
	if environment != "" {
		query = query.Where("environment = ?", normalizeEnvCode(environment))
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var hosts []model.AssetHost
	if err := query.Order("id DESC").Limit(limit).Find(&hosts).Error; err != nil {
		return nil, err
	}
	items := make([]map[string]any, 0, len(hosts))
	for _, host := range hosts {
		groupNames := make([]string, 0, len(host.HostGroups)+1)
		if host.Group.ID > 0 {
			groupNames = append(groupNames, host.Group.Name)
		}
		for _, group := range host.HostGroups {
			if group.Name != "" && !aiStringSliceContains(groupNames, group.Name) {
				groupNames = append(groupNames, group.Name)
			}
		}
		items = append(items, map[string]any{
			"id": host.ID, "name": host.HostName, "alias": host.Alias,
			"privateIp": host.PrivateIP, "publicIp": host.PublicIP, "sshPort": host.SSHPort,
			"os": host.OS, "arch": host.Arch, "cpu": host.CPU, "memory": host.Memory, "disk": host.Disk,
			"environment": host.Environment, "groups": groupNames,
			"enabled": host.Status == 1, "online": host.AliveStatus == 1, "lastCheckTime": host.LastCheckTime,
		})
	}
	return map[string]any{"resourceType": "server", "total": total, "returned": len(items), "items": items}, nil
}

func (s *Service) queryAIAssetDatabases(dbType string, args map[string]any) (map[string]any, error) {
	keyword := strings.TrimSpace(anyString(args["keyword"]))
	environment := strings.TrimSpace(anyString(args["environment"]))
	limit := aiAssetQueryLimit(args["limit"])
	normalizedType := normalizeDatabaseType(dbType)
	query := s.db.Model(&model.AssetDatabase{}).Where("db_type = ?", normalizedType)
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("name LIKE ? OR host LIKE ? OR db_name LIKE ?", like, like, like)
	}
	if environment != "" {
		query = query.Where("env = ?", normalizeEnvCode(environment))
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var databases []model.AssetDatabase
	if err := query.Order("id DESC").Limit(limit).Find(&databases).Error; err != nil {
		return nil, err
	}
	items := make([]map[string]any, 0, len(databases))
	for _, database := range databases {
		items = append(items, map[string]any{
			"id": database.ID, "name": database.Name, "type": normalizedType,
			"host": database.Host, "port": database.Port, "database": database.DBName,
			"environment": database.Env, "tags": database.Tags, "version": database.Version,
			"accessMode": database.AccessMode, "connectionMode": database.ConnectionMode,
			"enabled": database.Status == 1, "connected": database.ConnectStatus == 1,
			"connectStatus": database.ConnectStatus, "lastCheckTime": database.LastCheckTime,
		})
	}
	return map[string]any{"resourceType": "database", "databaseType": normalizedType, "total": total, "returned": len(items), "items": items}, nil
}

func aiStringSliceContains(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func anyString(value any) string {
	if value == nil {
		return ""
	}
	return fmt.Sprint(value)
}
func anyUint(value any) uint {
	switch item := value.(type) {
	case float64:
		return uint(item)
	case int:
		return uint(item)
	case uint:
		return item
	case json.Number:
		v, _ := item.Int64()
		return uint(v)
	default:
		var result uint
		_, _ = fmt.Sscan(fmt.Sprint(value), &result)
		return result
	}
}

func anyFloat(value any) float64 {
	if value == nil {
		return 0
	}
	switch item := value.(type) {
	case float64:
		return item
	case float32:
		return float64(item)
	case int:
		return float64(item)
	case json.Number:
		parsed, _ := item.Float64()
		return parsed
	default:
		parsed, _ := strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(value)), 64)
		return parsed
	}
}

func anyBool(value any) bool {
	switch item := value.(type) {
	case bool:
		return item
	case string:
		return strings.EqualFold(strings.TrimSpace(item), "true") || strings.TrimSpace(item) == "1"
	default:
		return false
	}
}

func (s *Service) ConfirmIntegrationAIToolAction(userID uint, username string, id uint) (map[string]any, error) {
	var action model.IntegrationAIToolAction
	if err := s.db.Where("id = ? AND user_id = ?", id, userID).First(&action).Error; err != nil {
		return nil, err
	}
	if action.Status != "pending" {
		return nil, errors.New("action has already been processed")
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(action.ArgumentsJSON), &args); err != nil {
		return nil, err
	}
	payload := model.K8sWorkloadActionPayload{ClusterID: anyUint(args["clusterId"]), Namespace: anyString(args["namespace"]), WorkloadType: anyString(args["workloadType"]), WorkloadName: anyString(args["workloadName"]), Replicas: int(anyUint(args["replicas"]))}
	var result map[string]any
	var err error
	switch action.ToolKey {
	case "k8s_restart_workload":
		result, err = s.RestartK8sWorkload(payload)
	case "k8s_scale_workload":
		result, err = s.ScaleK8sWorkload(payload)
	case "monitor_alert_rule_draft":
		result, err = s.createAIMonitorAlertRuleDraft(args)
	default:
		err = errors.New("unsupported pending confirmation action")
	}
	now := time.Now()
	action.ConfirmedAt, action.ConfirmedBy = &now, username
	if err != nil {
		action.Status = "failed"
		action.ResultJSON = err.Error()
	} else {
		action.Status = "completed"
		raw, _ := json.Marshal(result)
		action.ResultJSON = string(raw)
	}
	_ = s.db.Save(&action).Error
	if err != nil {
		return nil, err
	}
	return map[string]any{"action": action, "result": result}, nil
}

func (s *Service) createAIMonitorAlertRuleDraft(args map[string]any) (map[string]any, error) {
	name := strings.TrimSpace(anyString(args["name"]))
	promQL := strings.TrimSpace(anyString(args["promql"]))
	datasourceID := anyUint(args["datasourceId"])
	if name == "" || promQL == "" || datasourceID == 0 {
		return nil, errors.New("rule name, datasource, and PromQL are required")
	}
	payload := MonitorAlertRulePayload{
		Name: name, AlertType: "metric", DatasourceScope: "specific", DatasourceID: datasourceID, PromQL: promQL,
		Comparator: firstNonEmpty(strings.TrimSpace(anyString(args["comparator"])), ">"), Threshold: anyFloat(args["threshold"]),
		ForSeconds: int(anyUint(args["forSeconds"])), Severity: firstNonEmpty(strings.TrimSpace(anyString(args["severity"])), "P2"),
		Env: strings.TrimSpace(anyString(args["env"])), Description: strings.TrimSpace(anyString(args["description"])),
		LabelsJSON: "{}", AnnotationsJSON: "{}", NotifyEnabled: false, NotifyRecoveryEnabled: true, Status: 2,
	}
	if err := s.SaveMonitorAlertRule(payload); err != nil {
		return nil, err
	}
	var rule model.MonitorAlertRule
	if err := s.db.Where("name = ?", name).Order("id DESC").First(&rule).Error; err != nil {
		return nil, err
	}
	return map[string]any{"id": rule.ID, "name": rule.Name, "status": "draft_disabled", "message": "Alert Rule Draft를 비활성 상태로 저장했으며 Notification은 활성화하지 않았습니다. Alert Rule Page에서 검토한 뒤 활성화하십시오."}, nil
}

func (s *Service) RejectIntegrationAIToolAction(userID, id uint) error {
	return s.db.Model(&model.IntegrationAIToolAction{}).Where("id = ? AND user_id = ? AND status = ?", id, userID, "pending").Update("status", "rejected").Error
}
