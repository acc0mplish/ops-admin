package service

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
