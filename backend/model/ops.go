package model

import "time"

type OpsScript struct {
	ID             uint      `json:"id" gorm:"primaryKey"`
	Name           string    `json:"name" gorm:"size:128;not null;index"`
	ScriptType     string    `json:"scriptType" gorm:"size:32;not null;index"`
	Interpreter    string    `json:"interpreter" gorm:"size:32;not null"`
	Content        string    `json:"content" gorm:"type:longtext"`
	DefaultParams  string    `json:"defaultParams" gorm:"type:text"`
	TimeoutSeconds int       `json:"timeoutSeconds" gorm:"default:300"`
	Status         int       `json:"status" gorm:"default:1;not null;index"`
	Description    string    `json:"description" gorm:"size:255"`
	CreatedAt      time.Time `json:"createTime"`
	UpdatedAt      time.Time `json:"updateTime"`
}

func (OpsScript) TableName() string {
	return "ops_script"
}

type OpsExecTask struct {
	ID             uint       `json:"id" gorm:"primaryKey"`
	TaskType       string     `json:"taskType" gorm:"size:32;not null;index"`
	Title          string     `json:"title" gorm:"size:255"`
	ScriptID       uint       `json:"scriptId" gorm:"index"`
	ScriptName     string     `json:"scriptName" gorm:"size:128"`
	CommandText    string     `json:"commandText" gorm:"type:longtext"`
	Parameters     string     `json:"parameters" gorm:"type:text"`
	SourceType     string     `json:"sourceType" gorm:"size:32"`
	SourceHostID   uint       `json:"sourceHostId" gorm:"index"`
	SourceHostName string     `json:"sourceHostName" gorm:"size:128"`
	SourcePath     string     `json:"sourcePath" gorm:"size:512"`
	TargetPath     string     `json:"targetPath" gorm:"size:512"`
	FileName       string     `json:"fileName" gorm:"size:255"`
	Concurrency    int        `json:"concurrency" gorm:"default:5"`
	TimeoutSeconds int        `json:"timeoutSeconds" gorm:"default:10"`
	HostCount      int        `json:"hostCount" gorm:"default:0"`
	SuccessCount   int        `json:"successCount" gorm:"default:0"`
	FailedCount    int        `json:"failedCount" gorm:"default:0"`
	Status         string     `json:"status" gorm:"size:32;not null;index"`
	Summary        string     `json:"summary" gorm:"type:text"`
	StartedAt      *time.Time `json:"startedAt"`
	FinishedAt     *time.Time `json:"finishedAt"`
	CreatedAt      time.Time  `json:"createTime"`
	UpdatedAt      time.Time  `json:"updateTime"`
}

func (OpsExecTask) TableName() string {
	return "ops_exec_task"
}

type OpsExecTargetResult struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	TaskID     uint      `json:"taskId" gorm:"index;not null"`
	HostID     uint      `json:"hostId" gorm:"index"`
	HostName   string    `json:"hostName" gorm:"size:128"`
	GroupName  string    `json:"groupName" gorm:"size:128"`
	SSHIP      string    `json:"sshIp" gorm:"size:64"`
	Status     string    `json:"status" gorm:"size:32;not null;index"`
	ExitCode   int       `json:"exitCode" gorm:"default:0"`
	DurationMs int64     `json:"durationMs" gorm:"default:0"`
	Stdout     string    `json:"stdout" gorm:"type:longtext"`
	Stderr     string    `json:"stderr" gorm:"type:longtext"`
	ErrorText  string    `json:"errorText" gorm:"type:text"`
	CreatedAt  time.Time `json:"createTime"`
}

func (OpsExecTargetResult) TableName() string {
	return "ops_exec_target_result"
}

type OpsScheduleTemplate struct {
	ID             uint      `json:"id" gorm:"primaryKey"`
	Name           string    `json:"name" gorm:"size:128;not null;index"`
	TaskType       string    `json:"taskType" gorm:"size:32;not null;index"`
	ScriptID       uint      `json:"scriptId" gorm:"index"`
	ScriptName     string    `json:"scriptName" gorm:"size:128"`
	Parameters     string    `json:"parameters" gorm:"type:text"`
	HTTPMethod     string    `json:"httpMethod" gorm:"size:16"`
	URL            string    `json:"url" gorm:"size:1024"`
	HeadersJSON    string    `json:"headersJson" gorm:"type:text"`
	Body           string    `json:"body" gorm:"type:longtext"`
	ExpectedStatus int       `json:"expectedStatus" gorm:"default:200"`
	TimeoutSeconds int       `json:"timeoutSeconds" gorm:"default:10"`
	CronExpr       string    `json:"cronExpr" gorm:"size:128"`
	Description    string    `json:"description" gorm:"size:255"`
	Status         int       `json:"status" gorm:"default:1;index"`
	CreatedAt      time.Time `json:"createTime"`
	UpdatedAt      time.Time `json:"updateTime"`
}

func (OpsScheduleTemplate) TableName() string {
	return "ops_schedule_template"
}

type OpsScheduleTask struct {
	ID             uint       `json:"id" gorm:"primaryKey"`
	Name           string     `json:"name" gorm:"size:128;not null;index"`
	TaskType       string     `json:"taskType" gorm:"size:32;not null;index"`
	TemplateID     uint       `json:"templateId" gorm:"index"`
	ScriptID       uint       `json:"scriptId" gorm:"index"`
	ScriptName     string     `json:"scriptName" gorm:"size:128"`
	Parameters     string     `json:"parameters" gorm:"type:text"`
	HostIDsJSON    string     `json:"hostIdsJson" gorm:"type:text"`
	GroupIDsJSON   string     `json:"groupIdsJson" gorm:"type:text"`
	Concurrency    int        `json:"concurrency" gorm:"default:5"`
	HTTPMethod     string     `json:"httpMethod" gorm:"size:16"`
	URL            string     `json:"url" gorm:"size:1024"`
	HeadersJSON    string     `json:"headersJson" gorm:"type:text"`
	Body           string     `json:"body" gorm:"type:longtext"`
	ExpectedStatus int        `json:"expectedStatus" gorm:"default:200"`
	TimeoutSeconds int        `json:"timeoutSeconds" gorm:"default:10"`
	CronExpr       string     `json:"cronExpr" gorm:"size:128;not null"`
	Description    string     `json:"description" gorm:"size:255"`
	Status         int        `json:"status" gorm:"default:1;index"`
	NotifyEnabled  bool       `json:"notifyEnabled" gorm:"default:false;index"`
	NotifyRuleID   uint       `json:"notifyRuleId" gorm:"index"`
	LastStatus     string     `json:"lastStatus" gorm:"size:32"`
	LastSummary    string     `json:"lastSummary" gorm:"type:text"`
	LastRunAt      *time.Time `json:"lastRunAt"`
	NextRunAt      *time.Time `json:"nextRunAt"`
	CreatedAt      time.Time  `json:"createTime"`
	UpdatedAt      time.Time  `json:"updateTime"`
}

func (OpsScheduleTask) TableName() string {
	return "ops_schedule_task"
}

type OpsScheduleTaskLog struct {
	ID             uint       `json:"id" gorm:"primaryKey"`
	TaskID         uint       `json:"taskId" gorm:"index;not null"`
	TaskName       string     `json:"taskName" gorm:"size:128"`
	TaskType       string     `json:"taskType" gorm:"size:32;index"`
	TriggerType    string     `json:"triggerType" gorm:"size:32"`
	Status         string     `json:"status" gorm:"size:32;index"`
	Summary        string     `json:"summary" gorm:"type:text"`
	Detail         string     `json:"detail" gorm:"type:longtext"`
	ExecTaskID     uint       `json:"execTaskId" gorm:"index"`
	ExpectedStatus int        `json:"expectedStatus"`
	ActualStatus   int        `json:"actualStatus"`
	ResponseBody   string     `json:"responseBody" gorm:"type:longtext"`
	StartedAt      *time.Time `json:"startedAt"`
	FinishedAt     *time.Time `json:"finishedAt"`
	DurationMs     int64      `json:"durationMs" gorm:"default:0"`
	CreatedAt      time.Time  `json:"createTime"`
}

func (OpsScheduleTaskLog) TableName() string {
	return "ops_schedule_task_log"
}

type OpsJobTemplate struct {
	ID             uint      `json:"id" gorm:"primaryKey"`
	Name           string    `json:"name" gorm:"size:128;not null;index"`
	Description    string    `json:"description" gorm:"size:255"`
	Status         int       `json:"status" gorm:"default:1;index"`
	GraphJSON      string    `json:"graphJson" gorm:"type:longtext"`
	DefinitionJSON string    `json:"definitionJson" gorm:"type:longtext"`
	CreatedAt      time.Time `json:"createTime"`
	UpdatedAt      time.Time `json:"updateTime"`
}

func (OpsJobTemplate) TableName() string {
	return "ops_job_template"
}

type OpsJob struct {
	ID             uint      `json:"id" gorm:"primaryKey"`
	Name           string    `json:"name" gorm:"size:128;not null;index"`
	Description    string    `json:"description" gorm:"size:255"`
	Status         int       `json:"status" gorm:"default:1;index"`
	TemplateID     uint      `json:"templateId" gorm:"index"`
	NotifyEnabled  bool      `json:"notifyEnabled" gorm:"default:false;index"`
	NotifyRuleID   uint      `json:"notifyRuleId" gorm:"index"`
	GraphJSON      string    `json:"graphJson" gorm:"type:longtext"`
	DefinitionJSON string    `json:"definitionJson" gorm:"type:longtext"`
	CreatedAt      time.Time `json:"createTime"`
	UpdatedAt      time.Time `json:"updateTime"`
}

func (OpsJob) TableName() string {
	return "ops_job"
}

type OpsJobHistory struct {
	ID              uint       `json:"id" gorm:"primaryKey"`
	JobID           uint       `json:"jobId" gorm:"index;not null"`
	JobName         string     `json:"jobName" gorm:"size:128"`
	TriggerType     string     `json:"triggerType" gorm:"size:32"`
	Status          string     `json:"status" gorm:"size:32;index"`
	Summary         string     `json:"summary" gorm:"type:text"`
	CurrentStepID   string     `json:"currentStepId" gorm:"size:64"`
	CurrentStepName string     `json:"currentStepName" gorm:"size:128"`
	DefinitionJSON  string     `json:"definitionJson" gorm:"type:longtext"`
	StartedAt       *time.Time `json:"startedAt"`
	FinishedAt      *time.Time `json:"finishedAt"`
	CreatedAt       time.Time  `json:"createTime"`
}

func (OpsJobHistory) TableName() string {
	return "ops_job_history"
}

type OpsJobHistoryStep struct {
	ID               uint       `json:"id" gorm:"primaryKey"`
	HistoryID        uint       `json:"historyId" gorm:"index;not null"`
	StepID           string     `json:"stepId" gorm:"size:64;index"`
	StepName         string     `json:"stepName" gorm:"size:128"`
	StepType         string     `json:"stepType" gorm:"size:32;index"`
	Status           string     `json:"status" gorm:"size:32;index"`
	Summary          string     `json:"summary" gorm:"type:text"`
	Output           string     `json:"output" gorm:"type:longtext"`
	ExecTaskID       uint       `json:"execTaskId" gorm:"index"`
	ApprovalDecision string     `json:"approvalDecision" gorm:"size:32"`
	ApprovalNote     string     `json:"approvalNote" gorm:"type:text"`
	StartedAt        *time.Time `json:"startedAt"`
	FinishedAt       *time.Time `json:"finishedAt"`
	DurationMs       int64      `json:"durationMs" gorm:"default:0"`
	CreatedAt        time.Time  `json:"createTime"`
}

func (OpsJobHistoryStep) TableName() string {
	return "ops_job_history_step"
}

type NotifyTemplate struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	Name        string    `json:"name" gorm:"size:128;not null;index"`
	ChannelType string    `json:"channelType" gorm:"size:32;not null;index"`
	Title       string    `json:"title" gorm:"size:255"`
	Content     string    `json:"content" gorm:"type:longtext"`
	Status      int       `json:"status" gorm:"default:1;index"`
	Description string    `json:"description" gorm:"size:255"`
	CreatedAt   time.Time `json:"createTime"`
	UpdatedAt   time.Time `json:"updateTime"`
}

func (NotifyTemplate) TableName() string {
	return "notify_template"
}

type NotifyChannel struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	Name        string    `json:"name" gorm:"size:128;not null;index"`
	ChannelType string    `json:"channelType" gorm:"size:32;not null;index"`
	WebhookURL  string    `json:"webhookUrl" gorm:"size:1024;not null"`
	Secret      string    `json:"secret" gorm:"size:255"`
	HeadersJSON string    `json:"headersJson" gorm:"type:text"`
	Status      int       `json:"status" gorm:"default:1;index"`
	Description string    `json:"description" gorm:"size:255"`
	CreatedAt   time.Time `json:"createTime"`
	UpdatedAt   time.Time `json:"updateTime"`
}

func (NotifyChannel) TableName() string {
	return "notify_channel"
}

type NotifyRule struct {
	ID             uint      `json:"id" gorm:"primaryKey"`
	Name           string    `json:"name" gorm:"size:128;not null;index"`
	Scope          string    `json:"scope" gorm:"size:32;not null;index"`
	EventsJSON     string    `json:"eventsJson" gorm:"type:text"`
	TemplateID     uint      `json:"templateId" gorm:"index;not null"`
	ChannelIDsJSON string    `json:"channelIdsJson" gorm:"type:text"`
	Status         int       `json:"status" gorm:"default:1;index"`
	Description    string    `json:"description" gorm:"size:255"`
	CreatedAt      time.Time `json:"createTime"`
	UpdatedAt      time.Time `json:"updateTime"`
}

func (NotifyRule) TableName() string {
	return "notify_rule"
}

type NotifySendLog struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	RuleID      uint      `json:"ruleId" gorm:"index"`
	RuleName    string    `json:"ruleName" gorm:"size:128"`
	ChannelID   uint      `json:"channelId" gorm:"index"`
	ChannelName string    `json:"channelName" gorm:"size:128"`
	ChannelType string    `json:"channelType" gorm:"size:32;index"`
	Event       string    `json:"event" gorm:"size:64;index"`
	Scope       string    `json:"scope" gorm:"size:32;index"`
	TargetID    uint      `json:"targetId" gorm:"index"`
	TargetName  string    `json:"targetName" gorm:"size:128"`
	Status      string    `json:"status" gorm:"size:32;index"`
	RequestBody string    `json:"requestBody" gorm:"type:longtext"`
	Response    string    `json:"response" gorm:"type:longtext"`
	ErrorText   string    `json:"errorText" gorm:"type:text"`
	CreatedAt   time.Time `json:"createTime"`
}

func (NotifySendLog) TableName() string {
	return "notify_send_log"
}
