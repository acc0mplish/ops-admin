package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"ops-admin/backend/model"
	"ops-admin/backend/util"

	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

type OpsScheduleTemplatePayload struct {
	ID             uint              `json:"id"`
	Name           string            `json:"name"`
	TaskType       string            `json:"taskType"`
	ScriptID       uint              `json:"scriptId"`
	Variables      map[string]string `json:"variables"`
	HTTPMethod     string            `json:"httpMethod"`
	URL            string            `json:"url"`
	HeadersJSON    string            `json:"headersJson"`
	Body           string            `json:"body"`
	ExpectedStatus int               `json:"expectedStatus"`
	TimeoutSeconds int               `json:"timeoutSeconds"`
	CronExpr       string            `json:"cronExpr"`
	Description    string            `json:"description"`
	Status         int               `json:"status"`
}

type OpsScheduleTaskPayload struct {
	ID                  uint              `json:"id"`
	Name                string            `json:"name"`
	TaskType            string            `json:"taskType"`
	TemplateID          uint              `json:"templateId"`
	ScriptID            uint              `json:"scriptId"`
	Variables           map[string]string `json:"variables"`
	HostIDs             []uint            `json:"hostIds"`
	GroupIDs            []uint            `json:"groupIds"`
	Concurrency         int               `json:"concurrency"`
	HTTPMethod          string            `json:"httpMethod"`
	URL                 string            `json:"url"`
	HeadersJSON         string            `json:"headersJson"`
	Body                string            `json:"body"`
	ExpectedStatus      int               `json:"expectedStatus"`
	TimeoutSeconds      int               `json:"timeoutSeconds"`
	CronExpr            string            `json:"cronExpr"`
	Description         string            `json:"description"`
	Status              int               `json:"status"`
	NotifyEnabled       bool              `json:"notifyEnabled"`
	NotifyRuleID        uint              `json:"notifyRuleId"`
	NotifyOnFailureOnly bool              `json:"notifyOnFailureOnly"`
}

type OpsScheduleTaskStatusPayload struct {
	IDs    []uint `json:"ids"`
	Status int    `json:"status"`
}

type OpsScheduler struct {
	cron    *cron.Cron
	mu      sync.Mutex
	entries map[uint]cron.EntryID
}

func normalizeScheduleTaskType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "http", "probe", "http_probe":
		return "http"
	default:
		return "script"
	}
}

func normalizeScheduleStatus(value int) int {
	if value == 1 {
		return 1
	}
	return 2
}

func normalizeHTTPMethod(value string) string {
	method := strings.ToUpper(strings.TrimSpace(value))
	switch method {
	case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD":
		return method
	default:
		return "GET"
	}
}

func normalizeExpectedStatus(value int) int {
	if value <= 0 {
		return 200
	}
	return value
}

func normalizeCronExpr(value string) string {
	expr := strings.TrimSpace(value)
	fields := strings.Fields(expr)
	if len(fields) == 5 {
		return "0 " + expr
	}
	return expr
}

func encodeUintList(list []uint) string {
	if len(list) == 0 {
		return "[]"
	}
	data, _ := json.Marshal(list)
	return string(data)
}

func decodeUintList(raw string) []uint {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil
	}
	var list []uint
	_ = json.Unmarshal([]byte(value), &list)
	return list
}

// resolveScheduleScriptVariables validates task/template values against the
// selected script and encrypts values declared as secret before persistence.
// Empty secret inputs preserve an existing configured value on edit.
func resolveScheduleScriptVariables(script *model.OpsScript, supplied map[string]string, existing model.OpsScriptVariableValues) (model.OpsScriptVariableValues, map[string]string, error) {
	if supplied == nil {
		supplied = map[string]string{}
	}
	declared := make(map[string]model.OpsScriptVariable, len(script.Variables))
	for _, variable := range script.Variables {
		declared[variable.Name] = variable
	}
	for name := range supplied {
		if _, ok := declared[name]; !ok {
			return nil, nil, fmt.Errorf("variable VARIABLE_%s is not declared by the script", name)
		}
	}
	stored := model.OpsScriptVariableValues{}
	runtimeValues := map[string]string{}
	for name, variable := range declared {
		rawValue, suppliedValue := supplied[name]
		value := strings.TrimSpace(rawValue)
		if existingValue := strings.TrimSpace(existing[name]); existingValue != "" && (!suppliedValue || (variable.Secret && value == "")) {
			if variable.Secret {
				// variable.Secret is the declared-secret gate: the schedule
				// column is mixed-declaration, so the script metadata decides.
				field, fieldErr := registeredSecretField("ops_schedule_task", "variables")
				if fieldErr != nil {
					return nil, nil, fieldErr
				}
				plain, err := util.ReadSecretField(existingValue, field, variable.Secret)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to read variable VARIABLE_%s: %w", name, err)
				}
				value = plain
			} else {
				value = existingValue
			}
		}
		if value == "" {
			value = variable.DefaultValue
		}
		if variable.Required && value == "" {
			return nil, nil, fmt.Errorf("configure required variable VARIABLE_%s", name)
		}
		if value == "" {
			continue
		}
		runtimeValues[name] = value
		if variable.Secret {
			encrypted, err := util.EncryptSecretV2(value)
			if err != nil {
				return nil, nil, err
			}
			stored[name] = encrypted
		} else {
			stored[name] = value
		}
	}
	return stored, runtimeValues, nil
}

// scheduleVariableResponse never sends encrypted values (or plaintext secrets)
// back to the browser. An empty value means a secret is already configured.
func scheduleVariableResponse(script *model.OpsScript, stored model.OpsScriptVariableValues) map[string]string {
	if script == nil || len(stored) == 0 {
		return map[string]string{}
	}
	result := map[string]string{}
	for _, variable := range script.Variables {
		if value, ok := stored[variable.Name]; ok && !variable.Secret {
			result[variable.Name] = value
		}
	}
	return result
}

func normalizeHeadersJSON(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "{}", nil
	}
	var headers map[string]string
	if err := json.Unmarshal([]byte(value), &headers); err != nil {
		return "", errors.New("request headers must be a JSON object")
	}
	data, _ := json.Marshal(headers)
	return string(data), nil
}

func parseHeaderMap(raw string) map[string]string {
	var headers map[string]string
	if strings.TrimSpace(raw) == "" {
		return map[string]string{}
	}
	if err := json.Unmarshal([]byte(raw), &headers); err != nil {
		return map[string]string{}
	}
	return headers
}

func parseCronExpr(expr string) (cron.Schedule, error) {
	expr = normalizeCronExpr(expr)
	if expr == "" {
		return nil, errors.New("Cron expression is required")
	}
	parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	return parser.Parse(expr)
}

func (s *Service) initOpsScheduler() {
	s.opsSchedulerOnce.Do(func() {
		s.opsScheduler = &OpsScheduler{
			cron: cron.New(
				cron.WithSeconds(),
				cron.WithLocation(time.Local),
				cron.WithChain(cron.SkipIfStillRunning(cron.DefaultLogger)),
			),
			entries: map[uint]cron.EntryID{},
		}
		s.opsScheduler.cron.Start()
		s.reloadOpsScheduleTasks()
	})
}

func (s *Service) reloadOpsScheduleTasks() {
	if s.opsScheduler == nil {
		return
	}
	var tasks []model.OpsScheduleTask
	if err := s.db.Where("status = ?", 1).Find(&tasks).Error; err != nil {
		return
	}
	for _, task := range tasks {
		_ = s.registerOpsScheduleTask(task)
	}
}

func (s *Service) registerOpsScheduleTask(task model.OpsScheduleTask) error {
	if s.opsScheduler == nil {
		return nil
	}
	schedule, err := parseCronExpr(task.CronExpr)
	if err != nil {
		return err
	}
	s.removeOpsScheduleTask(task.ID)
	entryID, err := s.opsScheduler.cron.AddFunc(task.CronExpr, func() {
		s.executeScheduledTask(task.ID, "schedule")
	})
	if err != nil {
		return err
	}
	next := schedule.Next(time.Now())
	s.opsScheduler.mu.Lock()
	s.opsScheduler.entries[task.ID] = entryID
	s.opsScheduler.mu.Unlock()
	return s.db.Model(&model.OpsScheduleTask{}).Where("id = ?", task.ID).Update("next_run_at", &next).Error
}

func (s *Service) removeOpsScheduleTask(taskID uint) {
	if s.opsScheduler == nil {
		return
	}
	s.opsScheduler.mu.Lock()
	entryID, ok := s.opsScheduler.entries[taskID]
	if ok {
		delete(s.opsScheduler.entries, taskID)
	}
	s.opsScheduler.mu.Unlock()
	if ok {
		s.opsScheduler.cron.Remove(entryID)
	}
	_ = s.db.Model(&model.OpsScheduleTask{}).Where("id = ?", taskID).Update("next_run_at", nil).Error
}

func mapScheduleTaskItem(item model.OpsScheduleTask) map[string]any {
	return map[string]any{
		"id":                  item.ID,
		"name":                item.Name,
		"taskType":            item.TaskType,
		"templateId":          item.TemplateID,
		"scriptId":            item.ScriptID,
		"scriptName":          item.ScriptName,
		"parameters":          item.Parameters,
		"hostIds":             decodeUintList(item.HostIDsJSON),
		"groupIds":            decodeUintList(item.GroupIDsJSON),
		"concurrency":         item.Concurrency,
		"httpMethod":          item.HTTPMethod,
		"url":                 item.URL,
		"headersJson":         firstNonEmpty(item.HeadersJSON, "{}"),
		"body":                item.Body,
		"expectedStatus":      item.ExpectedStatus,
		"timeoutSeconds":      item.TimeoutSeconds,
		"cronExpr":            item.CronExpr,
		"description":         item.Description,
		"status":              item.Status,
		"notifyEnabled":       item.NotifyEnabled,
		"notifyRuleId":        item.NotifyRuleID,
		"notifyOnFailureOnly": item.NotifyOnFailureOnly,
		"lastStatus":          item.LastStatus,
		"lastSummary":         item.LastSummary,
		"lastRunAt":           item.LastRunAt,
		"nextRunAt":           item.NextRunAt,
		"createTime":          item.CreatedAt,
		"updateTime":          item.UpdatedAt,
	}
}

func mapScheduleTemplateItem(item model.OpsScheduleTemplate) map[string]any {
	return map[string]any{
		"id":             item.ID,
		"name":           item.Name,
		"taskType":       item.TaskType,
		"scriptId":       item.ScriptID,
		"scriptName":     item.ScriptName,
		"parameters":     item.Parameters,
		"httpMethod":     item.HTTPMethod,
		"url":            item.URL,
		"headersJson":    firstNonEmpty(item.HeadersJSON, "{}"),
		"body":           item.Body,
		"expectedStatus": item.ExpectedStatus,
		"timeoutSeconds": item.TimeoutSeconds,
		"cronExpr":       item.CronExpr,
		"description":    item.Description,
		"status":         item.Status,
		"createTime":     item.CreatedAt,
		"updateTime":     item.UpdatedAt,
	}
}

func (s *Service) ListOpsScheduleTasks(pageNum, pageSize int, keyword, taskType, status string) (map[string]any, error) {
	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	query := s.db.Model(&model.OpsScheduleTask{})
	if strings.TrimSpace(keyword) != "" {
		like := "%" + strings.TrimSpace(keyword) + "%"
		query = query.Where("name LIKE ? OR description LIKE ? OR script_name LIKE ? OR url LIKE ?", like, like, like, like)
	}
	if strings.TrimSpace(taskType) != "" {
		query = query.Where("task_type = ?", normalizeScheduleTaskType(taskType))
	}
	if strings.TrimSpace(status) != "" {
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var list []model.OpsScheduleTask
	if err := query.Order("id DESC").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, err
	}
	rows := make([]map[string]any, 0, len(list))
	for _, item := range list {
		rows = append(rows, mapScheduleTaskItem(item))
	}
	return map[string]any{
		"list":     rows,
		"total":    total,
		"pageNum":  pageNum,
		"pageSize": pageSize,
	}, nil
}

func (s *Service) GetOpsScheduleTask(id uint) (map[string]any, error) {
	var item model.OpsScheduleTask
	if err := s.db.First(&item, id).Error; err != nil {
		return nil, err
	}
	result := mapScheduleTaskItem(item)
	if item.ScriptID > 0 {
		if script, err := s.GetOpsScript(item.ScriptID); err == nil {
			result["variables"] = scheduleVariableResponse(script, item.Variables)
		}
	}
	return result, nil
}

func (s *Service) buildOpsScheduleTaskUpdates(payload OpsScheduleTaskPayload, existing *model.OpsScheduleTask) (map[string]any, error) {
	taskType := normalizeScheduleTaskType(payload.TaskType)
	name := Trimmed(payload.Name)
	if name == "" {
		return nil, errors.New("task name is required")
	}
	if _, err := parseCronExpr(payload.CronExpr); err != nil {
		return nil, errors.New("invalid Cron expression format")
	}
	headersJSON, err := normalizeHeadersJSON(payload.HeadersJSON)
	if err != nil {
		return nil, err
	}

	updates := map[string]any{
		"name":                   name,
		"task_type":              taskType,
		"template_id":            payload.TemplateID,
		"parameters":             "",
		"host_ids_json":          encodeUintList(payload.HostIDs),
		"group_ids_json":         encodeUintList(payload.GroupIDs),
		"concurrency":            normalizeOpsConcurrency(payload.Concurrency),
		"http_method":            normalizeHTTPMethod(payload.HTTPMethod),
		"url":                    strings.TrimSpace(payload.URL),
		"headers_json":           headersJSON,
		"body":                   payload.Body,
		"expected_status":        normalizeExpectedStatus(payload.ExpectedStatus),
		"timeout_seconds":        normalizeOpsTimeout(payload.TimeoutSeconds),
		"cron_expr":              normalizeCronExpr(payload.CronExpr),
		"description":            Trimmed(payload.Description),
		"status":                 normalizeScheduleStatus(payload.Status),
		"notify_enabled":         payload.NotifyEnabled,
		"notify_rule_id":         payload.NotifyRuleID,
		"notify_on_failure_only": payload.NotifyOnFailureOnly,
		"next_run_at":            nil,
	}

	switch taskType {
	case "script":
		if payload.ScriptID == 0 {
			return nil, errors.New("select a script")
		}
		script, err := s.GetOpsScript(payload.ScriptID)
		if err != nil {
			return nil, err
		}
		if script.Status != 1 {
			return nil, errors.New("disabled script cannot be used by a scheduled task")
		}
		if len(payload.HostIDs) == 0 && len(payload.GroupIDs) == 0 {
			return nil, errors.New("select target hosts or a host group")
		}
		if len(payload.HostIDs) > 0 && len(payload.GroupIDs) > 0 {
			return nil, errors.New("target hosts and host groups are mutually exclusive")
		}
		updates["script_id"] = script.ID
		updates["script_name"] = script.Name
		variables, _, err := resolveScheduleScriptVariables(script, payload.Variables, func() model.OpsScriptVariableValues {
			if existing == nil {
				return model.OpsScriptVariableValues{}
			}
			return existing.Variables
		}())
		if err != nil {
			return nil, err
		}
		updates["variables"] = variables
		updates["timeout_seconds"] = normalizeOpsTimeout(script.TimeoutSeconds)
		updates["http_method"] = ""
		updates["url"] = ""
		updates["headers_json"] = "{}"
		updates["body"] = ""
		updates["expected_status"] = 0
	case "http":
		if strings.TrimSpace(payload.URL) == "" {
			return nil, errors.New("HTTP URL is required")
		}
		updates["script_id"] = 0
		updates["script_name"] = ""
		updates["variables"] = model.OpsScriptVariableValues{}
		updates["host_ids_json"] = "[]"
		updates["group_ids_json"] = "[]"
		updates["concurrency"] = 1
	default:
		return nil, errors.New("unsupported task type")
	}

	if existing != nil {
		updates["last_status"] = existing.LastStatus
		updates["last_summary"] = existing.LastSummary
		updates["last_run_at"] = existing.LastRunAt
	}
	return updates, nil
}

func (s *Service) CreateOpsScheduleTask(payload OpsScheduleTaskPayload) error {
	updates, err := s.buildOpsScheduleTaskUpdates(payload, nil)
	if err != nil {
		return err
	}
	item := model.OpsScheduleTask{}
	if err := s.db.Model(&item).Create(updates).Error; err != nil {
		return err
	}
	if err := s.db.Last(&item).Error; err != nil {
		return err
	}
	if item.Status == 1 {
		return s.registerOpsScheduleTask(item)
	}
	return nil
}

func (s *Service) UpdateOpsScheduleTask(payload OpsScheduleTaskPayload) error {
	var existing model.OpsScheduleTask
	if err := s.db.First(&existing, payload.ID).Error; err != nil {
		return err
	}
	updates, err := s.buildOpsScheduleTaskUpdates(payload, &existing)
	if err != nil {
		return err
	}
	if err := s.db.Model(&model.OpsScheduleTask{}).Where("id = ?", payload.ID).Updates(updates).Error; err != nil {
		return err
	}
	var current model.OpsScheduleTask
	if err := s.db.First(&current, payload.ID).Error; err != nil {
		return err
	}
	if current.Status == 1 {
		return s.registerOpsScheduleTask(current)
	}
	s.removeOpsScheduleTask(current.ID)
	return nil
}

func (s *Service) DeleteOpsScheduleTask(id uint) error {
	s.removeOpsScheduleTask(id)
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&model.OpsScheduleTask{}, id).Error; err != nil {
			return err
		}
		return tx.Where("task_id = ?", id).Delete(&model.OpsScheduleTaskLog{}).Error
	})
}

func (s *Service) BatchDeleteOpsScheduleTasks(ids []uint) error {
	if len(ids) == 0 {
		return nil
	}
	for _, id := range ids {
		s.removeOpsScheduleTask(id)
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id IN ?", ids).Delete(&model.OpsScheduleTask{}).Error; err != nil {
			return err
		}
		return tx.Where("task_id IN ?", ids).Delete(&model.OpsScheduleTaskLog{}).Error
	})
}

func (s *Service) UpdateOpsScheduleTaskStatus(payload OpsScheduleTaskStatusPayload) error {
	if len(payload.IDs) == 0 {
		return errors.New("select a task")
	}
	status := normalizeScheduleStatus(payload.Status)
	if err := s.db.Model(&model.OpsScheduleTask{}).Where("id IN ?", payload.IDs).Update("status", status).Error; err != nil {
		return err
	}
	var tasks []model.OpsScheduleTask
	if err := s.db.Where("id IN ?", payload.IDs).Find(&tasks).Error; err != nil {
		return err
	}
	for _, task := range tasks {
		if status == 1 {
			if err := s.registerOpsScheduleTask(task); err != nil {
				return err
			}
		} else {
			s.removeOpsScheduleTask(task.ID)
		}
	}
	return nil
}

func (s *Service) RunOpsScheduleTask(id uint) error {
	var task model.OpsScheduleTask
	if err := s.db.First(&task, id).Error; err != nil {
		return err
	}
	go s.executeScheduledTask(task.ID, "manual")
	return nil
}
