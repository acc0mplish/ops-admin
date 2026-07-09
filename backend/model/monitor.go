package model

import "time"

type MonitorDatasource struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	Name        string    `json:"name" gorm:"size:128;not null;index"`
	Type        string    `json:"type" gorm:"size:32;not null;index"`
	URL         string    `json:"url" gorm:"size:1024;not null"`
	AuthType    string    `json:"authType" gorm:"size:32;default:none"`
	Username    string    `json:"username" gorm:"size:128"`
	Password    string    `json:"password" gorm:"size:255"`
	Token       string    `json:"token" gorm:"type:text"`
	IsDefault   bool      `json:"isDefault" gorm:"default:false;index"`
	Env         string    `json:"env" gorm:"size:64;index"`
	Status      int       `json:"status" gorm:"default:1;index"`
	Description string    `json:"description" gorm:"size:255"`
	CreatedAt   time.Time `json:"createTime"`
	UpdatedAt   time.Time `json:"updateTime"`
}

func (MonitorDatasource) TableName() string {
	return "monitor_datasource"
}

type MonitorAlertRule struct {
	ID                    uint       `json:"id" gorm:"primaryKey"`
	Name                  string     `json:"name" gorm:"size:128;not null;index"`
	DatasourceID          uint       `json:"datasourceId" gorm:"index;not null"`
	DatasourceName        string     `json:"datasourceName" gorm:"size:128"`
	PromQL                string     `json:"promql" gorm:"type:longtext;not null"`
	Comparator            string     `json:"comparator" gorm:"size:16;not null"`
	Threshold             float64    `json:"threshold"`
	ForSeconds            int        `json:"forSeconds" gorm:"default:60"`
	EvalIntervalSeconds   int        `json:"evalIntervalSeconds" gorm:"default:60"`
	Severity              string     `json:"severity" gorm:"size:16;default:P2;index"`
	LabelsJSON            string     `json:"labelsJson" gorm:"type:text"`
	AnnotationsJSON       string     `json:"annotationsJson" gorm:"type:text"`
	NotifyEnabled         bool       `json:"notifyEnabled" gorm:"default:false;index"`
	NotifyRuleID          uint       `json:"notifyRuleId" gorm:"index"`
	NotifyRecoveryEnabled bool       `json:"notifyRecoveryEnabled" gorm:"default:true"`
	Env                   string     `json:"env" gorm:"size:64;index"`
	Status                int        `json:"status" gorm:"default:1;index"`
	LastEvalAt            *time.Time `json:"lastEvalAt"`
	LastEvalStatus        string     `json:"lastEvalStatus" gorm:"size:32"`
	LastEvalMessage       string     `json:"lastEvalMessage" gorm:"type:text"`
	Description           string     `json:"description" gorm:"size:255"`
	CreatedAt             time.Time  `json:"createTime"`
	UpdatedAt             time.Time  `json:"updateTime"`
}

func (MonitorAlertRule) TableName() string {
	return "monitor_alert_rule"
}

type MonitorAlertEvent struct {
	ID                uint       `json:"id" gorm:"primaryKey"`
	RuleID            uint       `json:"ruleId" gorm:"index;not null"`
	RuleName          string     `json:"ruleName" gorm:"size:128"`
	DatasourceID      uint       `json:"datasourceId" gorm:"index"`
	DatasourceName    string     `json:"datasourceName" gorm:"size:128"`
	Fingerprint       string     `json:"fingerprint" gorm:"size:128;index"`
	Severity          string     `json:"severity" gorm:"size:16;index"`
	Status            string     `json:"status" gorm:"size:32;index"`
	Metric            string     `json:"metric" gorm:"size:255"`
	LabelsJSON        string     `json:"labelsJson" gorm:"type:text"`
	AnnotationsJSON   string     `json:"annotationsJson" gorm:"type:text"`
	CurrentValue      float64    `json:"currentValue"`
	Threshold         float64    `json:"threshold"`
	Summary           string     `json:"summary" gorm:"type:text"`
	Silenced          bool       `json:"silenced" gorm:"default:false;index"`
	SilenceRuleID     uint       `json:"silenceRuleId" gorm:"index"`
	SilenceRuleName   string     `json:"silenceRuleName" gorm:"size:128"`
	AggregationKey    string     `json:"aggregationKey" gorm:"size:255;index"`
	AggregateRuleID   uint       `json:"aggregateRuleId" gorm:"index"`
	AggregateRuleName string     `json:"aggregateRuleName" gorm:"size:128"`
	LastNotifyAt      *time.Time `json:"lastNotifyAt"`
	FirstTriggerAt    time.Time  `json:"firstTriggerAt"`
	LastTriggerAt     time.Time  `json:"lastTriggerAt"`
	RecoveredAt       *time.Time `json:"recoveredAt"`
	ClaimedBy         string     `json:"claimedBy" gorm:"size:128"`
	HandleNote        string     `json:"handleNote" gorm:"type:text"`
	CreatedAt         time.Time  `json:"createTime"`
	UpdatedAt         time.Time  `json:"updateTime"`
}

func (MonitorAlertEvent) TableName() string {
	return "monitor_alert_event"
}

type MonitorAlertAction struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	AlertEventID uint      `json:"alertEventId" gorm:"index;not null"`
	RuleName     string    `json:"ruleName" gorm:"size:128;index"`
	ActionType   string    `json:"actionType" gorm:"size:32;index"`
	TargetID     uint      `json:"targetId" gorm:"index"`
	TargetName   string    `json:"targetName" gorm:"size:128"`
	Status       string    `json:"status" gorm:"size:32;index"`
	Operator     string    `json:"operator" gorm:"size:128"`
	Summary      string    `json:"summary" gorm:"type:text"`
	Result       string    `json:"result" gorm:"type:longtext"`
	CreatedAt    time.Time `json:"createTime"`
	UpdatedAt    time.Time `json:"updateTime"`
}

func (MonitorAlertAction) TableName() string {
	return "monitor_alert_action"
}

type MonitorSilenceRule struct {
	ID              uint       `json:"id" gorm:"primaryKey"`
	Name            string     `json:"name" gorm:"size:128;not null;index"`
	MatchMode       string     `json:"matchMode" gorm:"size:32;default:regex;index"`
	RuleIDsJSON     string     `json:"ruleIdsJson" gorm:"type:text"`
	RuleNamePattern string     `json:"ruleNamePattern" gorm:"size:128;index"`
	Severity        string     `json:"severity" gorm:"size:16;index"`
	MatchersJSON    string     `json:"matchersJson" gorm:"type:text"`
	StartsAt        *time.Time `json:"startsAt"`
	EndsAt          *time.Time `json:"endsAt"`
	Status          int        `json:"status" gorm:"default:1;index"`
	Description     string     `json:"description" gorm:"size:255"`
	CreatedAt       time.Time  `json:"createTime"`
	UpdatedAt       time.Time  `json:"updateTime"`
}

func (MonitorSilenceRule) TableName() string {
	return "monitor_silence_rule"
}

type MonitorAggregationRule struct {
	ID                    uint      `json:"id" gorm:"primaryKey"`
	Name                  string    `json:"name" gorm:"size:128;not null;index"`
	MatchMode             string    `json:"matchMode" gorm:"size:32;default:regex;index"`
	RuleIDsJSON           string    `json:"ruleIdsJson" gorm:"type:text"`
	RuleNamePattern       string    `json:"ruleNamePattern" gorm:"size:128;index"`
	Severity              string    `json:"severity" gorm:"size:16;index"`
	GroupByJSON           string    `json:"groupByJson" gorm:"type:text"`
	WindowSeconds         int       `json:"windowSeconds" gorm:"default:300"`
	RepeatIntervalSeconds int       `json:"repeatIntervalSeconds" gorm:"default:1800"`
	Status                int       `json:"status" gorm:"default:1;index"`
	Description           string    `json:"description" gorm:"size:255"`
	CreatedAt             time.Time `json:"createTime"`
	UpdatedAt             time.Time `json:"updateTime"`
}

func (MonitorAggregationRule) TableName() string {
	return "monitor_aggregation_rule"
}

type MonitorQueryHistory struct {
	ID             uint      `json:"id" gorm:"primaryKey"`
	DatasourceID   uint      `json:"datasourceId" gorm:"index"`
	DatasourceName string    `json:"datasourceName" gorm:"size:128"`
	Query          string    `json:"query" gorm:"type:longtext"`
	QueryType      string    `json:"queryType" gorm:"size:32"`
	Status         string    `json:"status" gorm:"size:32;index"`
	ErrorText      string    `json:"errorText" gorm:"type:text"`
	CreatedAt      time.Time `json:"createTime"`
}

func (MonitorQueryHistory) TableName() string {
	return "monitor_query_history"
}

type MonitorDashboard struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	Name        string    `json:"name" gorm:"size:128;not null;index"`
	Layout      string    `json:"layout" gorm:"size:32;default:grid"`
	Status      int       `json:"status" gorm:"default:1;index"`
	Description string    `json:"description" gorm:"size:255"`
	CreatedAt   time.Time `json:"createTime"`
	UpdatedAt   time.Time `json:"updateTime"`
}

func (MonitorDashboard) TableName() string {
	return "monitor_dashboard"
}

type MonitorDashboardPanel struct {
	ID             uint      `json:"id" gorm:"primaryKey"`
	DashboardID    uint      `json:"dashboardId" gorm:"index;not null"`
	Title          string    `json:"title" gorm:"size:128;not null"`
	DatasourceID   uint      `json:"datasourceId" gorm:"index;not null"`
	DatasourceName string    `json:"datasourceName" gorm:"size:128"`
	PromQL         string    `json:"promql" gorm:"type:longtext;not null"`
	Unit           string    `json:"unit" gorm:"size:32"`
	ChartType      string    `json:"chartType" gorm:"size:32;default:stat"`
	Span           int       `json:"span" gorm:"default:8"`
	Sort           int       `json:"sort" gorm:"default:0;index"`
	Status         int       `json:"status" gorm:"default:1;index"`
	Description    string    `json:"description" gorm:"size:255"`
	CreatedAt      time.Time `json:"createTime"`
	UpdatedAt      time.Time `json:"updateTime"`
}

func (MonitorDashboardPanel) TableName() string {
	return "monitor_dashboard_panel"
}
