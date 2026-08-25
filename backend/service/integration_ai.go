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
	{Key: "knowledge_base_search", Name: "知识库检索", Category: "知识库", Description: "仅检索知识库管理中已启用的本地 Markdown 文档，用于回答内部规范、运行手册和技术文档；不会访问外部文件或云服务。", Permission: "read", Parameters: knowledgeBaseToolSchema()},
	{Key: "prometheus_query", Name: "PromQL 即时查询", Category: "监控中心", Description: "在 Prometheus 或 VictoriaMetrics 数据源执行即时 PromQL 查询。", Permission: "read", Parameters: objectSchema(map[string]any{"datasourceId": integerProperty("数据源 ID，可留空使用默认数据源"), "query": stringProperty("PromQL 查询语句")}, []string{"query"})},
	{Key: "monitor_log_query", Name: "日志即时查询", Category: "监控中心", Description: "在 Elasticsearch 或 VictoriaLogs 中按时间范围查询日志，支持统计命中数和查看少量明细。例如统计昨天 10:00 到 11:00 err.log 中 ERROR 日志数量。", Permission: "read", Parameters: logQueryToolSchema()},
	{Key: "monitor_dashboard_list", Name: "监控大屏查询", Category: "Grafana 可视化", Description: "查询平台监控大屏与面板概况，为可视化排障提供入口。", Permission: "read", Parameters: objectSchema(map[string]any{"keyword": stringProperty("大屏名称关键词")}, nil)},
	{Key: "monitor_datasource_query", Name: "监控数据源查询", Category: "夜莺监控技能", Description: "查询已接入的监控和日志数据源、类型、健康状态、延迟与最近检查时间；不会返回地址中的敏感凭据。", Permission: "read", Parameters: datasourceQueryToolSchema()},
	{Key: "monitor_alert_event_query", Name: "告警事件查询", Category: "夜莺监控技能", Description: "按关键词、状态、等级和时间范围查询监控告警事件，并返回数量和有限明细。", Permission: "read", Parameters: alertEventQueryToolSchema()},
	{Key: "host_health_diagnose", Name: "主机健康诊断", Category: "夜莺监控技能", Description: "结合 CMDB 主机信息、最近 24 小时 CPU/内存/磁盘指标和关联告警，返回主机健康证据；仅查询，不执行任何修复操作。", Permission: "read", Parameters: hostHealthToolSchema()},
	{Key: "ops_troubleshooting", Name: "智能排障", Category: "夜莺监控技能", Description: "围绕告警 ID、主机或问题关键词汇集告警、主机健康和数据源状态，形成有证据来源的排障上下文；模型必须区分事实与推测。", Permission: "read", Parameters: troubleshootingToolSchema()},
	{Key: "monitor_dashboard_analyze", Name: "监控大屏分析", Category: "夜莺监控技能", Description: "读取大屏和面板的定义、数据源、PromQL 与说明，供模型分析指标含义和排障入口；不会修改大屏。", Permission: "read", Parameters: dashboardAnalyzeToolSchema()},
	{Key: "monitor_alert_rule_draft", Name: "告警规则草稿", Category: "夜莺监控技能", Description: "创建一条默认停用且不发送通知的告警规则草稿。必须由用户确认后才会保存，保存后仍需在告警规则页面人工审核和启用。", Permission: "write", RequireConfirmation: true, Parameters: alertRuleDraftToolSchema()},
	{Key: "finops_cost_analysis", Name: "云费用分析", Category: "云费用 FinOps", Description: "仅查询本地数据库中已通过账单同步导入的云费用数据。可返回费用总览、趋势、产品/地域拆分；询问某云产品的实例数或每实例费用时，传入 service 和 includeResourceBreakdown=true，按本地账单的资源 ID/名称聚合。绝不调用云厂商接口、不会同步账单。", Permission: "read", Parameters: finOpsAnalysisToolSchema()},
	{Key: "asset_host_list", Name: "服务器资产", Category: "资产管理", Description: "查询 CMDB 中的服务器、IP、环境、主机组与在线状态，不返回登录凭据。", Permission: "read", Parameters: assetQuerySchema("服务器名称、别名或 IP 关键词")},
	{Key: "asset_mysql_list", Name: "MySQL 资产", Category: "资产管理", Description: "查询已纳管的 MySQL 数据库连接、环境、版本和健康状态。", Permission: "read", Parameters: assetQuerySchema("数据库名称、地址或默认库关键词")},
	{Key: "asset_postgresql_list", Name: "PostgreSQL 资产", Category: "资产管理", Description: "查询已纳管的 PostgreSQL 数据库连接、环境、版本和健康状态。", Permission: "read", Parameters: assetQuerySchema("数据库名称、地址或默认库关键词")},
	{Key: "asset_redis_list", Name: "Redis 资产", Category: "资产管理", Description: "查询已纳管的 Redis 实例、逻辑库、环境、版本和健康状态。", Permission: "read", Parameters: assetQuerySchema("Redis 名称、地址或逻辑库关键词")},
	{Key: "asset_mongodb_list", Name: "MongoDB 资产", Category: "资产管理", Description: "查询已纳管的 MongoDB 数据库连接、环境、版本和健康状态。", Permission: "read", Parameters: assetQuerySchema("数据库名称、地址或默认库关键词")},
	{Key: "k8s_list_clusters", Name: "K8s 集群列表", Category: "Kubernetes", Description: "查询已接入集群、状态、版本和节点数量。", Permission: "read", Parameters: objectSchema(map[string]any{}, nil)},
	{Key: "k8s_cluster_overview", Name: "K8s 集群概览", Category: "Kubernetes", Description: "查询指定集群的节点、工作负载、Pod 与健康概况。", Permission: "read", Parameters: objectSchema(map[string]any{"clusterId": integerProperty("K8s 集群 ID")}, []string{"clusterId"})},
	{Key: "k8s_restart_workload", Name: "重启 K8s 工作负载", Category: "Kubernetes", Description: "对 Deployment、StatefulSet 或 DaemonSet 执行滚动重启。", Permission: "write", RequireConfirmation: true, Parameters: workloadActionSchema(false)},
	{Key: "k8s_scale_workload", Name: "扩缩容 K8s 工作负载", Category: "Kubernetes", Description: "修改 Deployment 或 StatefulSet 的副本数。", Permission: "write", RequireConfirmation: true, Parameters: workloadActionSchema(true)},
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
		"environment": stringProperty("环境编码，例如 dev、test、prod；留空查询全部环境"),
		"limit":       integerProperty("最多返回多少条，范围 1 到 50，默认 20"),
	}, nil)
}

func finOpsAnalysisToolSchema() map[string]any {
	return objectSchema(map[string]any{
		"accountId":                integerProperty("云账号 ID；留空则分析全部已同步云账号"),
		"month":                    stringProperty("分析账期，格式 YYYY-MM；留空默认最近一个有本地同步账单的自然月"),
		"trendMonths":              integerProperty("趋势月份数，1 到 12，默认 6"),
		"service":                  stringProperty("云产品名称关键词；例如 负载均衡、NAT网关、ECS"),
		"includeResourceBreakdown": booleanProperty("是否返回该云产品按实例/资源 ID 聚合的费用；询问实例数量或每实例费用时设为 true"),
	}, nil)
}

func knowledgeBaseToolSchema() map[string]any {
	return objectSchema(map[string]any{
		"keyword":    stringProperty("需要检索的关键词或问题，例如：发布流程、Redis 故障处理"),
		"documentId": integerProperty("可选，限定检索某篇知识库文档的 ID"),
		"limit":      integerProperty("最多返回多少个匹配片段，范围 1 到 10，默认 5"),
	}, []string{"keyword"})
}

func logQueryToolSchema() map[string]any {
	return objectSchema(map[string]any{
		"datasourceId":   integerProperty("日志数据源 ID；留空则查询所有启用的 Elasticsearch 和 VictoriaLogs 数据源"),
		"datasourceName": stringProperty("数据源名称关键词，仅在未指定 datasourceId 时使用"),
		"index":          stringProperty("Elasticsearch 索引或索引模式，例如 logs-*；留空查询全部索引"),
		"streams":        stringProperty("Stream/Topic，多个值用逗号分隔；默认按包含关系匹配，例如 err.log 可匹配 szfc.err.log.1053；使用 =szfc.err.log.1053 可精确匹配"),
		"query":          stringProperty("日志条件：Elasticsearch 使用 Lucene，VictoriaLogs 使用 LogsQL；例如 level:ERROR，留空表示全部"),
		"startTime":      stringProperty("开始时间，支持 RFC3339、YYYY-MM-DD HH:mm:ss、昨天 10:00 或 yesterday 10:00，时区 Asia/Shanghai"),
		"endTime":        stringProperty("结束时间，格式与 startTime 相同，必须晚于开始时间"),
		"mode":           stringProperty("返回模式：count 仅统计数量，list 返回少量日志明细；默认 count"),
		"limit":          integerProperty("list 模式最多返回的日志条数，范围 1 到 50，默认 20"),
	}, []string{"startTime", "endTime"})
}

func datasourceQueryToolSchema() map[string]any {
	return objectSchema(map[string]any{"keyword": stringProperty("数据源名称关键词"), "type": stringProperty("数据源类型：prometheus、victoriametrics、elasticsearch 或 victorialogs"), "healthStatus": stringProperty("健康状态，例如 healthy、unhealthy、unknown"), "limit": integerProperty("返回条数，1 到 50，默认 20")}, nil)
}

func alertEventQueryToolSchema() map[string]any {
	return objectSchema(map[string]any{"keyword": stringProperty("规则名、指标、摘要或标签关键词"), "status": stringProperty("告警状态，例如 firing、claimed、recovered、resolved"), "severity": stringProperty("告警等级：P0、P1、P2、P3"), "startTime": stringProperty("可选，RFC3339 或 YYYY-MM-DD HH:mm:ss"), "endTime": stringProperty("可选，RFC3339 或 YYYY-MM-DD HH:mm:ss"), "limit": integerProperty("返回条数，1 到 50，默认 20")}, nil)
}

func hostHealthToolSchema() map[string]any {
	return objectSchema(map[string]any{"hostId": integerProperty("CMDB 主机 ID；与 keyword 二选一"), "keyword": stringProperty("主机名、别名或 IP"), "range": stringProperty("指标时间范围：1h、6h、24h、7d，默认 24h")}, nil)
}

func troubleshootingToolSchema() map[string]any {
	return objectSchema(map[string]any{"alertEventId": integerProperty("可选，告警事件 ID"), "host": stringProperty("可选，主机名、别名或 IP"), "keyword": stringProperty("可选，问题、规则或指标关键词"), "range": stringProperty("主机指标时间范围：1h、6h、24h、7d，默认 24h")}, nil)
}

func dashboardAnalyzeToolSchema() map[string]any {
	return objectSchema(map[string]any{"dashboardId": integerProperty("监控大屏 ID"), "keyword": stringProperty("大屏名称关键词；未填写 ID 时使用")}, nil)
}

func alertRuleDraftToolSchema() map[string]any {
	return objectSchema(map[string]any{
		"name": stringProperty("规则名称"), "datasourceId": integerProperty("Prometheus/VictoriaMetrics 数据源 ID"), "promql": stringProperty("已校验的 PromQL"),
		"comparator": stringProperty("比较符：>、>=、<、<=、==、!="), "threshold": stringProperty("阈值数字"), "forSeconds": integerProperty("持续秒数，默认 300"),
		"severity": stringProperty("等级：P0、P1、P2、P3，默认 P2"), "description": stringProperty("规则说明"), "env": stringProperty("环境，例如 prod"),
	}, []string{"name", "datasourceId", "promql"})
}

func workloadActionSchema(withReplicas bool) map[string]any {
	properties := map[string]any{
		"clusterId": integerProperty("K8s 集群 ID"), "namespace": stringProperty("命名空间"),
		"workloadType": stringProperty("工作负载类型，例如 deployment"), "workloadName": stringProperty("工作负载名称"),
	}
	required := []string{"clusterId", "namespace", "workloadType", "workloadName"}
	if withReplicas {
		properties["replicas"] = integerProperty("目标副本数")
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
		return nil, errors.New("知识库文档名称不能为空")
	}
	if payload.Content == "" {
		return nil, errors.New("Markdown 内容不能为空")
	}
	if len([]rune(payload.Content)) > 500000 {
		return nil, errors.New("Markdown 内容不能超过 50 万字符")
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
			return nil, errors.New("知识库文档不存在")
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
		return errors.New("知识库文档 ID 不能为空")
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
		return nil, errors.New("模型名称、API 地址和模型标识不能为空")
	}
	if parsed, err := url.Parse(payload.BaseURL); err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, errors.New("请输入有效的 OpenAI 兼容 API 地址")
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
		return errors.New("模型 ID 不能为空")
	}
	var count int64
	if err := s.db.Model(&model.IntegrationAIConversation{}).Where("model_id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return errors.New("该模型已被会话使用，请先停用模型，不能直接删除")
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
	response, err := s.callOpenAICompatible(item, []map[string]any{{"role": "user", "content": "只回复 OK"}}, nil)
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
		title = "新会话"
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
		return nil, errors.New("请输入对话内容")
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
		return nil, errors.New("请选择一个已启用的 AI 模型")
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
			map[string]any{"role": "user", "content": "不要输出内部工具调用标记或 XML/DSML。只能调用已提供的原生工具；如果没有可调用工具，请直接用简洁中文回答。"},
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
				toolResult = map[string]any{"status": "pending_confirmation", "actionId": action.ID, "message": "该操作需要用户确认后执行"}
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
		response.Content = "工具调用轮次已达到安全上限，未继续执行。请缩小查询范围后重试。"
	}
	if strings.TrimSpace(response.Content) == "" {
		response.Content = "操作已生成，请在下方确认后执行。"
	}
	if hasUnsupportedAIToolProtocol(response.Content) {
		response.Content = "模型返回了不受支持的内部工具调用格式，未执行任何操作。请重新提问，或切换支持原生工具调用的模型。"
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
	if conversation.Title == "新会话" {
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
	base := "你是 Ops Admin 平台的 DevOps/SRE 助手。回答必须使用中文，先给结论，再给证据和操作建议。回复必须使用标准 Markdown：按内容需要使用二、三级标题、无序/有序列表、加粗、引用、表格和代码块；不要把 Markdown 标记当作普通文本转义输出。涉及生产变更时说明风险，不得声称已执行未实际执行的操作。优先使用平台工具获取数据。只能调用本请求提供的原生工具名称；绝不在回复正文输出 XML、DSML、tool_calls、invoke 或其他内部工具协议。云费用相关问题只能使用云费用分析工具返回的本地已同步账单数据；绝不调用云厂商接口，也不得把账单数据表述为实时云端数据。"
	base += "\n\n夜莺监控技能规范：遇到 PromQL 需求先生成表达式，必要时用 prometheus_query 验证；查询告警使用 monitor_alert_event_query；查询数据源使用 monitor_datasource_query；主机问题优先使用 host_health_diagnose；综合故障使用 ops_troubleshooting；大屏问题使用 monitor_dashboard_analyze。分析结论必须基于工具返回的证据，并明确区分已证实的现象和推测。创建告警时只能调用 monitor_alert_rule_draft，等待用户确认后创建停用草稿，不能承诺已启用或已发送通知。"
	if strings.TrimSpace(custom) != "" {
		return base + "\n\n附加指令：\n" + strings.TrimSpace(custom)
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
	return "该历史消息包含模型未支持的内部工具调用格式，未执行任何操作。请重新发起查询。"
}

const finOpsChatResponseInstruction = "云费用工具已返回结果。请使用简洁 Markdown 回答，不超过 8 行：可使用二、三级标题和无序列表，但不要使用表格、引用、代码块、漏斗图或长段落。普通费用问题格式：\n## 账期｜账号\n- **总费用**：金额（如为当前月须注明截至日期）\n- **主要产品**：仅列 Top 3，产品 + 金额 + 占比\n- **地域**：一句结论\n- **关注项**：最多 3 条，仅基于账单可验证的现象；没有监控数据时使用“建议核查”，不得断言资源闲置。\n当用户询问某产品的实例数量或每实例费用时：优先读取 resourceBreakdown，回答“实例数”和最多 5 个“实例名称/ID：费用”；若 resourceBreakdown.resourceCount 为 0，只能说明本地同步账单缺少可关联的资源 ID/名称及未关联金额，不能建议用户去云厂商控制台。不要推算整月费用、不要给出节省金额区间，除非用户明确要求。最后可用一句话说明“数据来自本地已同步账单”。"

const knowledgeBaseChatResponseInstruction = "知识库检索工具已返回本地 Markdown 片段。必须围绕用户问题重新归纳，不得把文档目录、标题或章节逐条复述，也不要以“根据知识库文档”开头。先用 1 至 2 句说明与提问最相关的结论，再按“平台定位 / 当前可直接使用的能力 / 推荐使用路径”组织，优先解释用户能获得什么价值、下一步能做什么。除非用户明确要求清单，否则最多列 5 个能力组；把相关模块合并为业务闭环，不要罗列所有菜单。明确区分“已实现/已具备”和“建议/规划”，不得把评审意见当作已实现功能。答案保持 6 至 10 行中文要点；只在用户要求来源或原文时才注明文档名或引用原文。"

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
		return nil, fmt.Errorf("模型请求失败: %w", err)
	}
	defer response.Body.Close()
	responseBody, _ := io.ReadAll(io.LimitReader(response.Body, 8*1024*1024))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("模型 API 返回 %d: %s", response.StatusCode, strings.TrimSpace(string(responseBody)))
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
		return nil, errors.New("模型 API 未返回有效内容")
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
	return errors.New("未知的 AI 工具")
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
			return nil, errors.New("PromQL 不能为空")
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
			return nil, errors.New("没有可用的 Prometheus/VictoriaMetrics 数据源")
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
		return nil, errors.New("该工具只能通过待确认动作执行")
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
			return nil, errors.New("请提供 hostId 或主机关键词")
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
		return nil, errors.New("知识库检索关键词不能为空")
	}
	limit := int(anyUint(args["limit"]))
	if limit == 0 {
		limit = 5
	}
	if limit < 1 || limit > 10 {
		return nil, errors.New("limit 必须在 1 到 10 之间")
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
		return nil, errors.New("month 参数格式无效，格式为 YYYY-MM")
	}
	accountID := anyUint(args["accountId"])
	trendMonths := int(anyUint(args["trendMonths"]))
	if trendMonths == 0 {
		trendMonths = 6
	}
	if trendMonths < 1 || trendMonths > 12 {
		return nil, errors.New("trendMonths 必须在 1 到 12 之间")
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
		"sourceDescription":    "仅使用本地已同步云账单数据；未调用云厂商接口",
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
		"sourceDescription":  "按本地已同步账单的 resourceId/resourceName 聚合；不会查询云厂商接口",
	}, nil
}

func (s *Service) queryAIRealtimeLogs(args map[string]any, now time.Time) (map[string]any, error) {
	startAt, err := parseAILogTime(anyString(args["startTime"]), now)
	if err != nil {
		return nil, fmt.Errorf("开始时间无效: %w", err)
	}
	endAt, err := parseAILogTime(anyString(args["endTime"]), now)
	if err != nil {
		return nil, fmt.Errorf("结束时间无效: %w", err)
	}
	if !endAt.After(startAt) {
		return nil, errors.New("结束时间必须晚于开始时间")
	}
	if endAt.Sub(startAt) > 31*24*time.Hour {
		return nil, errors.New("单次日志查询时间范围不能超过 31 天")
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
		return nil, errors.New("返回模式仅支持 count 或 list")
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
		return nil, errors.New("没有匹配且已启用的 Elasticsearch/VictoriaLogs 数据源")
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
		return nil, fmt.Errorf("所有日志数据源查询均失败: %s", strings.Join(errorsByDatasource, "; "))
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
		return time.Time{}, errors.New("时间不能为空")
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
		{prefix: "昨天", days: -1}, {prefix: "yesterday", days: -1},
		{prefix: "今天", days: 0}, {prefix: "today", days: 0},
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
				return time.Time{}, errors.New("相对时间应为‘昨天 10:00’或‘today 10:00’")
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
	return time.Time{}, errors.New("支持 RFC3339、YYYY-MM-DD HH:mm:ss 或‘昨天 10:00’")
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
		return nil, errors.New("该动作已处理")
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
		err = errors.New("不支持的待确认动作")
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
		return nil, errors.New("规则名称、数据源和 PromQL 不能为空")
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
	return map[string]any{"id": rule.ID, "name": rule.Name, "status": "draft_disabled", "message": "告警规则草稿已保存为停用状态，未启用通知；请在告警规则页面审核后再启用。"}, nil
}

func (s *Service) RejectIntegrationAIToolAction(userID, id uint) error {
	return s.db.Model(&model.IntegrationAIToolAction{}).Where("id = ? AND user_id = ? AND status = ?", id, userID, "pending").Update("status", "rejected").Error
}
