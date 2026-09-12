package service

type MonitorDatasourcePayload struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	URL         string `json:"url"`
	AuthType    string `json:"authType"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	Token       string `json:"token"`
	IsDefault   bool   `json:"isDefault"`
	Env         string `json:"env"`
	Status      int    `json:"status"`
	Description string `json:"description"`
}

type MonitorAlertRulePayload struct {
	ID                          uint    `json:"id"`
	Name                        string  `json:"name"`
	AlertType                   string  `json:"alertType"`
	DatasourceScope             string  `json:"datasourceScope"`
	DatasourceID                uint    `json:"datasourceId"`
	PromQL                      string  `json:"promql"`
	Query                       string  `json:"query"`
	LogIndex                    string  `json:"logIndex"`
	LogTimeRangeSeconds         int     `json:"logTimeRangeSeconds"`
	Comparator                  string  `json:"comparator"`
	Threshold                   float64 `json:"threshold"`
	ForSeconds                  int     `json:"forSeconds"`
	EvalIntervalSeconds         int     `json:"evalIntervalSeconds"`
	NotifyRepeatIntervalSeconds int     `json:"notifyRepeatIntervalSeconds"`
	MaxNotifyCount              int     `json:"maxNotifyCount"`
	Severity                    string  `json:"severity"`
	LabelsJSON                  string  `json:"labelsJson"`
	AnnotationsJSON             string  `json:"annotationsJson"`
	NotifyEnabled               bool    `json:"notifyEnabled"`
	NotifyRuleID                uint    `json:"notifyRuleId"`
	NotifyRecoveryEnabled       bool    `json:"notifyRecoveryEnabled"`
	Env                         string  `json:"env"`
	Status                      int     `json:"status"`
	Description                 string  `json:"description"`
}

type MonitorAlertTemplatePayload struct {
	ID                  uint    `json:"id"`
	GroupID             uint    `json:"groupId"`
	Name                string  `json:"name"`
	Category            string  `json:"category"`
	Collector           string  `json:"collector"`
	ObjectType          string  `json:"objectType"`
	DatasourceType      string  `json:"datasourceType"`
	QueryText           string  `json:"queryText"`
	Comparator          string  `json:"comparator"`
	Threshold           float64 `json:"threshold"`
	ForSeconds          int     `json:"forSeconds"`
	EvalIntervalSeconds int     `json:"evalIntervalSeconds"`
	Severity            string  `json:"severity"`
	LabelsJSON          string  `json:"labelsJson"`
	AnnotationsJSON     string  `json:"annotationsJson"`
	Description         string  `json:"description"`
	Status              int     `json:"status"`
}

type MonitorAlertTemplateGroupPayload struct {
	ID       uint   `json:"id"`
	ParentID uint   `json:"parentId"`
	Name     string `json:"name"`
}

type MonitorAlertTemplateImportItem struct {
	Name                string  `json:"name"`
	PrometheusGroup     string  `json:"prometheusGroup"`
	OriginalExpression  string  `json:"originalExpression"`
	QueryText           string  `json:"queryText"`
	Comparator          string  `json:"comparator"`
	Threshold           float64 `json:"threshold"`
	ForSeconds          int     `json:"forSeconds"`
	EvalIntervalSeconds int     `json:"evalIntervalSeconds"`
	Severity            string  `json:"severity"`
	LabelsJSON          string  `json:"labelsJson"`
	AnnotationsJSON     string  `json:"annotationsJson"`
	Description         string  `json:"description"`
}

type MonitorAlertTemplateImportPayload struct {
	GroupID           uint                             `json:"groupId"`
	DuplicateStrategy string                           `json:"duplicateStrategy"`
	Items             []MonitorAlertTemplateImportItem `json:"items"`
}

type MonitorAlertTemplateExportPayload struct {
	IDs []uint `json:"ids"`
}

type MonitorAlertRuleBatchPayload struct {
	IDs                         []uint `json:"ids"`
	Action                      string `json:"action"`
	NotifyRuleID                uint   `json:"notifyRuleId"`
	NotifyRepeatIntervalSeconds *int   `json:"notifyRepeatIntervalSeconds"`
	MaxNotifyCount              *int   `json:"maxNotifyCount"`
	NotifyRecoveryEnabled       *bool  `json:"notifyRecoveryEnabled"`
	ForSeconds                  *int   `json:"forSeconds"`
	EvalIntervalSeconds         *int   `json:"evalIntervalSeconds"`
}

type MonitorAlertEventActionPayload struct {
	ID         uint   `json:"id"`
	ClaimedBy  string `json:"claimedBy"`
	HandleNote string `json:"handleNote"`
}

type MonitorAlertEventBatchPayload struct {
	IDs        []uint `json:"ids"`
	Action     string `json:"action"`
	ClaimedBy  string `json:"claimedBy"`
	HandleNote string `json:"handleNote"`
}

type MonitorRuleBatchPayload struct {
	IDs    []uint `json:"ids"`
	Action string `json:"action"`
}

type MonitorSilenceRulePayload struct {
	ID              uint   `json:"id"`
	Name            string `json:"name"`
	MatchMode       string `json:"matchMode"`
	RuleIDs         []uint `json:"ruleIds"`
	RuleNamePattern string `json:"ruleNamePattern"`
	Severity        string `json:"severity"`
	AlertType       string `json:"alertType"`
	MatchersJSON    string `json:"matchersJson"`
	StartsAt        int64  `json:"startsAt"`
	EndsAt          int64  `json:"endsAt"`
	Priority        int    `json:"priority"`
	Status          int    `json:"status"`
	Description     string `json:"description"`
}

func normalizeSilencePriority(value int) int {
	if value <= 0 {
		return 100
	}
	if value > 1000 {
		return 1000
	}
	return value
}

type MonitorAggregationRulePayload struct {
	ID                    uint     `json:"id"`
	Name                  string   `json:"name"`
	MatchMode             string   `json:"matchMode"`
	RuleIDs               []uint   `json:"ruleIds"`
	RuleNamePattern       string   `json:"ruleNamePattern"`
	Severity              string   `json:"severity"`
	AlertType             string   `json:"alertType"`
	GroupBy               []string `json:"groupBy"`
	WindowSeconds         int      `json:"windowSeconds"`
	RepeatIntervalSeconds int      `json:"repeatIntervalSeconds"`
	Status                int      `json:"status"`
	Description           string   `json:"description"`
}

type MonitorDashboardPayload struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Layout      string `json:"layout"`
	Status      int    `json:"status"`
	Description string `json:"description"`
}

type MonitorDashboardPanelPayload struct {
	ID           uint   `json:"id"`
	DashboardID  uint   `json:"dashboardId"`
	Title        string `json:"title"`
	DatasourceID uint   `json:"datasourceId"`
	PromQL       string `json:"promql"`
	Unit         string `json:"unit"`
	ChartType    string `json:"chartType"`
	Span         int    `json:"span"`
	Sort         int    `json:"sort"`
	Status       int    `json:"status"`
	Description  string `json:"description"`
}

type MonitorDashboardPanelQueryPayload struct {
	ID           uint  `json:"id"`
	DatasourceID uint  `json:"datasourceId"`
	StartAt      int64 `json:"startAt"`
	EndAt        int64 `json:"endAt"`
	StepSeconds  int   `json:"stepSeconds"`
}

type MonitorLogQueryPayload struct {
	DatasourceID   uint   `json:"datasourceId"`
	Index          string `json:"index"`
	Query          string `json:"query"`
	StartAt        int64  `json:"startAt"`
	EndAt          int64  `json:"endAt"`
	PageNum        int    `json:"pageNum"`
	PageSize       int    `json:"pageSize"`
	TrackTotalHits bool   `json:"trackTotalHits"`
}

type MonitorTraceQueryPayload struct {
	DatasourceID uint   `json:"datasourceId"`
	Service      string `json:"service"`
	Operation    string `json:"operation"`
	Tags         string `json:"tags"`
	StartAt      int64  `json:"startAt"`
	EndAt        int64  `json:"endAt"`
	Limit        int    `json:"limit"`
}

type MonitorRangeQueryPayload struct {
	DatasourceID uint   `json:"datasourceId"`
	Query        string `json:"query"`
	StartAt      int64  `json:"startAt"`
	EndAt        int64  `json:"endAt"`
	StepSeconds  int    `json:"stepSeconds"`
}

type MonitorLogShortcutPayload struct {
	ID             uint   `json:"id"`
	DatasourceType string `json:"datasourceType"`
	Name           string `json:"name"`
	Query          string `json:"query"`
	IndexName      string `json:"indexName"`
	TimeRange      string `json:"timeRange"`
	Sort           int    `json:"sort"`
}
