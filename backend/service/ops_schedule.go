package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"ops-admin/backend/model"

	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

type OpsScheduleTemplatePayload struct {
	ID             uint   `json:"id"`
	Name           string `json:"name"`
	TaskType       string `json:"taskType"`
	ScriptID       uint   `json:"scriptId"`
	Parameters     string `json:"parameters"`
	HTTPMethod     string `json:"httpMethod"`
	URL            string `json:"url"`
	HeadersJSON    string `json:"headersJson"`
	Body           string `json:"body"`
	ExpectedStatus int    `json:"expectedStatus"`
	TimeoutSeconds int    `json:"timeoutSeconds"`
	CronExpr       string `json:"cronExpr"`
	Description    string `json:"description"`
	Status         int    `json:"status"`
}

type OpsScheduleTaskPayload struct {
	ID             uint   `json:"id"`
	Name           string `json:"name"`
	TaskType       string `json:"taskType"`
	TemplateID     uint   `json:"templateId"`
	ScriptID       uint   `json:"scriptId"`
	Parameters     string `json:"parameters"`
	HostIDs        []uint `json:"hostIds"`
	GroupIDs       []uint `json:"groupIds"`
	Concurrency    int    `json:"concurrency"`
	HTTPMethod     string `json:"httpMethod"`
	URL            string `json:"url"`
	HeadersJSON    string `json:"headersJson"`
	Body           string `json:"body"`
	ExpectedStatus int    `json:"expectedStatus"`
	TimeoutSeconds int    `json:"timeoutSeconds"`
	CronExpr       string `json:"cronExpr"`
	Description    string `json:"description"`
	Status         int    `json:"status"`
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

func normalizeHeadersJSON(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "{}", nil
	}
	var headers map[string]string
	if err := json.Unmarshal([]byte(value), &headers); err != nil {
		return "", errors.New("璇锋眰澶村繀椤绘槸 JSON 瀵硅薄")
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
		return nil, errors.New("Cron 表达式不能为空")
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
		"id":             item.ID,
		"name":           item.Name,
		"taskType":       item.TaskType,
		"templateId":     item.TemplateID,
		"scriptId":       item.ScriptID,
		"scriptName":     item.ScriptName,
		"parameters":     item.Parameters,
		"hostIds":        decodeUintList(item.HostIDsJSON),
		"groupIds":       decodeUintList(item.GroupIDsJSON),
		"concurrency":    item.Concurrency,
		"httpMethod":     item.HTTPMethod,
		"url":            item.URL,
		"headersJson":    firstNonEmpty(item.HeadersJSON, "{}"),
		"body":           item.Body,
		"expectedStatus": item.ExpectedStatus,
		"timeoutSeconds": item.TimeoutSeconds,
		"cronExpr":       item.CronExpr,
		"description":    item.Description,
		"status":         item.Status,
		"lastStatus":     item.LastStatus,
		"lastSummary":    item.LastSummary,
		"lastRunAt":      item.LastRunAt,
		"nextRunAt":      item.NextRunAt,
		"createTime":     item.CreatedAt,
		"updateTime":     item.UpdatedAt,
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
	return mapScheduleTaskItem(item), nil
}

func (s *Service) buildOpsScheduleTaskUpdates(payload OpsScheduleTaskPayload, existing *model.OpsScheduleTask) (map[string]any, error) {
	taskType := normalizeScheduleTaskType(payload.TaskType)
	name := Trimmed(payload.Name)
	if name == "" {
		return nil, errors.New("浠诲姟鍚嶇О涓嶈兘涓虹┖")
	}
	if _, err := parseCronExpr(payload.CronExpr); err != nil {
		return nil, errors.New("Cron 琛ㄨ揪寮忔牸寮忎笉姝ｇ‘")
	}
	headersJSON, err := normalizeHeadersJSON(payload.HeadersJSON)
	if err != nil {
		return nil, err
	}

	updates := map[string]any{
		"name":            name,
		"task_type":       taskType,
		"template_id":     payload.TemplateID,
		"parameters":      strings.TrimSpace(payload.Parameters),
		"host_ids_json":   encodeUintList(payload.HostIDs),
		"group_ids_json":  encodeUintList(payload.GroupIDs),
		"concurrency":     normalizeOpsConcurrency(payload.Concurrency),
		"http_method":     normalizeHTTPMethod(payload.HTTPMethod),
		"url":             strings.TrimSpace(payload.URL),
		"headers_json":    headersJSON,
		"body":            payload.Body,
		"expected_status": normalizeExpectedStatus(payload.ExpectedStatus),
		"timeout_seconds": normalizeOpsTimeout(payload.TimeoutSeconds),
		"cron_expr":       normalizeCronExpr(payload.CronExpr),
		"description":     Trimmed(payload.Description),
		"status":          normalizeScheduleStatus(payload.Status),
		"next_run_at":     nil,
	}

	switch taskType {
	case "script":
		if payload.ScriptID == 0 {
			return nil, errors.New("璇烽€夋嫨鑴氭湰")
		}
		script, err := s.GetOpsScript(payload.ScriptID)
		if err != nil {
			return nil, err
		}
		if script.Status != 1 {
			return nil, errors.New("鑴氭湰宸茬鐢紝涓嶈兘鐢ㄤ簬瀹氭椂浠诲姟")
		}
		if len(payload.HostIDs) == 0 && len(payload.GroupIDs) == 0 {
			return nil, errors.New("璇烽€夋嫨鐩爣涓绘満鎴栦富鏈虹粍")
		}
		if len(payload.HostIDs) > 0 && len(payload.GroupIDs) > 0 {
			return nil, errors.New("鐩爣涓绘満鍜屼富鏈虹粍鍙兘浜岄€変竴")
		}
		updates["script_id"] = script.ID
		updates["script_name"] = script.Name
		updates["timeout_seconds"] = normalizeOpsTimeout(script.TimeoutSeconds)
		updates["http_method"] = ""
		updates["url"] = ""
		updates["headers_json"] = "{}"
		updates["body"] = ""
		updates["expected_status"] = 0
	case "http":
		if strings.TrimSpace(payload.URL) == "" {
			return nil, errors.New("HTTP 鍦板潃涓嶈兘涓虹┖")
		}
		updates["script_id"] = 0
		updates["script_name"] = ""
		updates["host_ids_json"] = "[]"
		updates["group_ids_json"] = "[]"
		updates["concurrency"] = 1
	default:
		return nil, errors.New("涓嶆敮鎸佺殑浠诲姟绫诲瀷")
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
		return errors.New("璇烽€夋嫨浠诲姟")
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

func (s *Service) ListOpsScheduleTemplates(pageNum, pageSize int, keyword, taskType, status string) (map[string]any, error) {
	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	query := s.db.Model(&model.OpsScheduleTemplate{})
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
	var list []model.OpsScheduleTemplate
	if err := query.Order("id DESC").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, err
	}
	rows := make([]map[string]any, 0, len(list))
	for _, item := range list {
		rows = append(rows, mapScheduleTemplateItem(item))
	}
	return map[string]any{
		"list":     rows,
		"total":    total,
		"pageNum":  pageNum,
		"pageSize": pageSize,
	}, nil
}

func (s *Service) GetOpsScheduleTemplate(id uint) (map[string]any, error) {
	var item model.OpsScheduleTemplate
	if err := s.db.First(&item, id).Error; err != nil {
		return nil, err
	}
	return mapScheduleTemplateItem(item), nil
}

func (s *Service) buildOpsScheduleTemplateUpdates(payload OpsScheduleTemplatePayload) (map[string]any, error) {
	taskType := normalizeScheduleTaskType(payload.TaskType)
	name := Trimmed(payload.Name)
	if name == "" {
		return nil, errors.New("妯℃澘鍚嶇О涓嶈兘涓虹┖")
	}
	headersJSON, err := normalizeHeadersJSON(payload.HeadersJSON)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(payload.CronExpr) != "" {
		if _, err := parseCronExpr(payload.CronExpr); err != nil {
			return nil, errors.New("Cron 琛ㄨ揪寮忔牸寮忎笉姝ｇ‘")
		}
	}
	updates := map[string]any{
		"name":            name,
		"task_type":       taskType,
		"parameters":      strings.TrimSpace(payload.Parameters),
		"http_method":     normalizeHTTPMethod(payload.HTTPMethod),
		"url":             strings.TrimSpace(payload.URL),
		"headers_json":    headersJSON,
		"body":            payload.Body,
		"expected_status": normalizeExpectedStatus(payload.ExpectedStatus),
		"timeout_seconds": normalizeOpsTimeout(payload.TimeoutSeconds),
		"cron_expr":       normalizeCronExpr(payload.CronExpr),
		"description":     Trimmed(payload.Description),
		"status":          normalizeScheduleStatus(payload.Status),
	}
	switch taskType {
	case "script":
		if payload.ScriptID == 0 {
			return nil, errors.New("璇烽€夋嫨鑴氭湰")
		}
		script, err := s.GetOpsScript(payload.ScriptID)
		if err != nil {
			return nil, err
		}
		updates["script_id"] = script.ID
		updates["script_name"] = script.Name
		updates["timeout_seconds"] = normalizeOpsTimeout(script.TimeoutSeconds)
		updates["http_method"] = ""
		updates["url"] = ""
		updates["headers_json"] = "{}"
		updates["body"] = ""
		updates["expected_status"] = 0
	case "http":
		if strings.TrimSpace(payload.URL) == "" {
			return nil, errors.New("HTTP 鍦板潃涓嶈兘涓虹┖")
		}
		updates["script_id"] = 0
		updates["script_name"] = ""
	default:
		return nil, errors.New("涓嶆敮鎸佺殑妯℃澘绫诲瀷")
	}
	return updates, nil
}

func (s *Service) CreateOpsScheduleTemplate(payload OpsScheduleTemplatePayload) error {
	updates, err := s.buildOpsScheduleTemplateUpdates(payload)
	if err != nil {
		return err
	}
	return s.db.Model(&model.OpsScheduleTemplate{}).Create(updates).Error
}

func (s *Service) UpdateOpsScheduleTemplate(payload OpsScheduleTemplatePayload) error {
	updates, err := s.buildOpsScheduleTemplateUpdates(payload)
	if err != nil {
		return err
	}
	return s.db.Model(&model.OpsScheduleTemplate{}).Where("id = ?", payload.ID).Updates(updates).Error
}

func (s *Service) DeleteOpsScheduleTemplate(id uint) error {
	var count int64
	if err := s.db.Model(&model.OpsScheduleTask{}).Where("template_id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return errors.New("璇ユā鏉夸粛琚换鍔″紩鐢紝涓嶈兘鍒犻櫎")
	}
	return s.db.Delete(&model.OpsScheduleTemplate{}, id).Error
}

func (s *Service) ListOpsScheduleTaskLogs(pageNum, pageSize int, keyword, taskType, status string) (map[string]any, error) {
	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	query := s.db.Model(&model.OpsScheduleTaskLog{})
	if strings.TrimSpace(keyword) != "" {
		like := "%" + strings.TrimSpace(keyword) + "%"
		query = query.Where("task_name LIKE ? OR summary LIKE ?", like, like)
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
	var list []model.OpsScheduleTaskLog
	if err := query.Order("id DESC").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, err
	}
	return map[string]any{
		"list":     list,
		"total":    total,
		"pageNum":  pageNum,
		"pageSize": pageSize,
	}, nil
}

func (s *Service) GetOpsScheduleTaskLog(id uint) (*model.OpsScheduleTaskLog, error) {
	var item model.OpsScheduleTaskLog
	if err := s.db.First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Service) executeScheduledTask(taskID uint, triggerType string) {
	var task model.OpsScheduleTask
	if err := s.db.First(&task, taskID).Error; err != nil {
		return
	}
	if strings.TrimSpace(triggerType) == "" {
		triggerType = "schedule"
	}
	startedAt := time.Now()
	logItem := model.OpsScheduleTaskLog{
		TaskID:      task.ID,
		TaskName:    task.Name,
		TaskType:    task.TaskType,
		TriggerType: triggerType,
		Status:      "running",
		Summary:     "??????????",
		StartedAt:   &startedAt,
	}
	_ = s.db.Create(&logItem).Error

	var (
		status      string
		summary     string
		detail      string
		execTaskID  uint
		httpCode    int
		responseRaw string
	)

	switch task.TaskType {
	case "http":
		status, summary, detail, httpCode, responseRaw = s.runScheduledHTTPTask(task)
	default:
		status, summary, detail, execTaskID = s.runScheduledScriptTask(task)
	}

	finishedAt := time.Now()
	nextRunAt := nextRunTime(task.CronExpr)
	_ = s.db.Model(&model.OpsScheduleTaskLog{}).Where("id = ?", logItem.ID).Updates(map[string]any{
		"status":          status,
		"summary":         summary,
		"detail":          detail,
		"exec_task_id":    execTaskID,
		"expected_status": task.ExpectedStatus,
		"actual_status":   httpCode,
		"response_body":   responseRaw,
		"finished_at":     &finishedAt,
		"duration_ms":     finishedAt.Sub(startedAt).Milliseconds(),
	}).Error
	_ = s.db.Model(&model.OpsScheduleTask{}).Where("id = ?", task.ID).Updates(map[string]any{
		"last_status":  status,
		"last_summary": summary,
		"last_run_at":  &finishedAt,
		"next_run_at":  nextRunAt,
	}).Error
}

func nextRunTime(expr string) *time.Time {
	schedule, err := parseCronExpr(expr)
	if err != nil {
		return nil
	}
	next := schedule.Next(time.Now())
	return &next
}

func (s *Service) runScheduledScriptTask(task model.OpsScheduleTask) (string, string, string, uint) {
	hostIDs := decodeUintList(task.HostIDsJSON)
	groupIDs := decodeUintList(task.GroupIDsJSON)
	hosts, err := s.resolveOpsTargetHosts(hostIDs, groupIDs)
	if err != nil {
		return "failed", err.Error(), err.Error(), 0
	}
	script, err := s.GetOpsScript(task.ScriptID)
	if err != nil {
		return "failed", err.Error(), err.Error(), 0
	}
	execTask := model.OpsExecTask{
		TaskType:       "script",
		Title:          fmt.Sprintf("瀹氭椂浠诲姟 - %s", task.Name),
		ScriptID:       script.ID,
		ScriptName:     script.Name,
		Parameters:     task.Parameters,
		Concurrency:    normalizeOpsConcurrency(task.Concurrency),
		TimeoutSeconds: normalizeOpsTimeout(script.TimeoutSeconds),
		Status:         "running",
		Summary:        "定时任务执行中",
		HostCount:      len(hosts),
	}
	data, err := s.runOpsTaskLegacy(execTask, hosts, func(host model.AssetHost) model.OpsExecTargetResult {
		params := strings.TrimSpace(task.Parameters)
		if params == "" {
			params = strings.TrimSpace(script.DefaultParams)
		}
		return s.execScriptOnHost(host, *script, params, execTask.TimeoutSeconds)
	})
	if err != nil {
		return "failed", err.Error(), err.Error(), 0
	}
	taskInfo, _ := data["task"].(model.OpsExecTask)
	results, _ := data["results"].([]model.OpsExecTargetResult)
	lines := make([]string, 0, len(results))
	for _, row := range results {
		lines = append(lines, fmt.Sprintf("%s [%s] exit=%d", row.HostName, row.Status, row.ExitCode))
	}
	status := firstNonEmpty(taskInfo.Status, "success")
	return status, firstNonEmpty(taskInfo.Summary, "鎵ц瀹屾垚"), strings.Join(lines, "\n"), taskInfo.ID
}

func (s *Service) runScheduledHTTPTask(task model.OpsScheduleTask) (string, string, string, int, string) {
	client := &http.Client{Timeout: time.Duration(normalizeOpsTimeout(task.TimeoutSeconds)) * time.Second}
	request, err := http.NewRequest(normalizeHTTPMethod(task.HTTPMethod), task.URL, strings.NewReader(task.Body))
	if err != nil {
		return "failed", err.Error(), err.Error(), 0, ""
	}
	for key, value := range parseHeaderMap(task.HeadersJSON) {
		request.Header.Set(key, value)
	}
	response, err := client.Do(request)
	if err != nil {
		return "failed", err.Error(), err.Error(), 0, ""
	}
	defer response.Body.Close()
	bodyBytes, _ := io.ReadAll(io.LimitReader(response.Body, 32768))
	responseBody := string(bodyBytes)
	summary := fmt.Sprintf("%s %s -> %d", request.Method, task.URL, response.StatusCode)
	if response.StatusCode == normalizeExpectedStatus(task.ExpectedStatus) {
		return "success", summary, responseBody, response.StatusCode, responseBody
	}
	return "failed", fmt.Sprintf("%s锛屾湡鏈涚姸鎬佺爜 %d", summary, normalizeExpectedStatus(task.ExpectedStatus)), responseBody, response.StatusCode, responseBody
}
