package service

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"ops-admin/backend/model"
)

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
	result := mapScheduleTemplateItem(item)
	if item.ScriptID > 0 {
		if script, err := s.GetOpsScript(item.ScriptID); err == nil {
			result["variables"] = scheduleVariableResponse(script, item.Variables)
		}
	}
	return result, nil
}

func (s *Service) buildOpsScheduleTemplateUpdates(payload OpsScheduleTemplatePayload, existing *model.OpsScheduleTemplate) (map[string]any, error) {
	taskType := normalizeScheduleTaskType(payload.TaskType)
	name := Trimmed(payload.Name)
	if name == "" {
		return nil, errors.New("template name is required")
	}
	headersJSON, err := normalizeHeadersJSON(payload.HeadersJSON)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(payload.CronExpr) != "" {
		if _, err := parseCronExpr(payload.CronExpr); err != nil {
			return nil, errors.New("invalid Cron expression format")
		}
	}
	updates := map[string]any{
		"name":            name,
		"task_type":       taskType,
		"parameters":      "",
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
			return nil, errors.New("select a script")
		}
		script, err := s.GetOpsScript(payload.ScriptID)
		if err != nil {
			return nil, err
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
	default:
		return nil, errors.New("unsupported template type")
	}
	return updates, nil
}

func (s *Service) CreateOpsScheduleTemplate(payload OpsScheduleTemplatePayload) error {
	updates, err := s.buildOpsScheduleTemplateUpdates(payload, nil)
	if err != nil {
		return err
	}
	return s.db.Model(&model.OpsScheduleTemplate{}).Create(updates).Error
}

func (s *Service) UpdateOpsScheduleTemplate(payload OpsScheduleTemplatePayload) error {
	var existing model.OpsScheduleTemplate
	if err := s.db.First(&existing, payload.ID).Error; err != nil {
		return err
	}
	updates, err := s.buildOpsScheduleTemplateUpdates(payload, &existing)
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
		return errors.New("template is still referenced by tasks and cannot be deleted")
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
	if task.NotifyEnabled && task.NotifyRuleID > 0 && (!task.NotifyOnFailureOnly || !strings.EqualFold(status, "success")) {
		duration := finishedAt.Sub(startedAt)
		s.DispatchNotifyRule(task.NotifyRuleID, NotifyEvent{
			Scope:      "schedule",
			Event:      status,
			TargetID:   task.ID,
			TargetName: task.Name,
			Status:     status,
			Summary:    summary,
			// The execution log keeps the complete response body. Notifications
			// intentionally use a compact result, otherwise a successful HTTP
			// probe can push an entire HTML page into chat.
			Detail:     compactScheduleNotifyDetail(task, status, detail, httpCode),
			StartedAt:  &startedAt,
			FinishedAt: &finishedAt,
			Extra: map[string]string{
				"taskName":       task.Name,
				"taskType":       scheduleTaskTypeLabel(task.TaskType),
				"triggerType":    scheduleTriggerTypeLabel(triggerType),
				"cronExpr":       task.CronExpr,
				"duration":       formatScheduleDuration(duration),
				"durationMs":     fmt.Sprintf("%d", duration.Milliseconds()),
				"httpStatus":     formatScheduleHTTPStatus(httpCode),
				"expectedStatus": fmt.Sprintf("%d", normalizeExpectedStatus(task.ExpectedStatus)),
				"alertName":      task.Name,
				"severity":       "Scheduled Task",
			},
		})
	}
}

func compactScheduleNotifyDetail(task model.OpsScheduleTask, status, detail string, httpCode int) string {
	if task.TaskType == "http" {
		return scheduleHTTPNotifyDetail(status, httpCode, normalizeExpectedStatus(task.ExpectedStatus))
	}
	return trimNotifyText(detail, 1200)
}

func scheduleHTTPNotifyDetail(status string, httpCode, expectedStatus int) string {
	if strings.EqualFold(status, "success") {
		return fmt.Sprintf("HTTP probe returned %d, matching expected status %d.", httpCode, expectedStatus)
	}
	if httpCode > 0 {
		return fmt.Sprintf("HTTP probe returned %d instead of expected status %d. See task logs for the complete response.", httpCode, expectedStatus)
	}
	return "HTTP probe request failed. See task logs for complete error details."
}

func trimNotifyText(value string, limit int) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "\n...(execution details truncated; see task logs for complete output)"
}

func scheduleTaskTypeLabel(value string) string {
	if value == "http" {
		return "HTTP Probe"
	}
	return "Script Task"
}

func scheduleTriggerTypeLabel(value string) string {
	if value == "manual" {
		return "Manual"
	}
	return "Scheduled"
}

func formatScheduleDuration(value time.Duration) string {
	if value < time.Second {
		return fmt.Sprintf("%d ms", value.Milliseconds())
	}
	return value.Round(time.Millisecond).String()
}

func formatScheduleHTTPStatus(value int) string {
	if value <= 0 {
		return "-"
	}
	return strconv.Itoa(value)
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
		Title:          fmt.Sprintf("Scheduled Task - %s", task.Name),
		ScriptID:       script.ID,
		ScriptName:     script.Name,
		Parameters:     "",
		Concurrency:    normalizeOpsConcurrency(task.Concurrency),
		TimeoutSeconds: normalizeOpsTimeout(script.TimeoutSeconds),
		Status:         "running",
		Summary:        "scheduled task running",
		HostCount:      len(hosts),
		Operator:       "scheduler",
		Source:         "schedule",
		RiskLevel:      opsRiskLevel(script.Content),
		ScriptVersion:  script.CurrentVersion,
		TargetSnapshot: opsTargetSnapshot(hosts),
	}
	data, err := s.runOpsTaskLegacy(execTask, hosts, func(host model.AssetHost) model.OpsExecTargetResult {
		_, variables, resolveErr := resolveScheduleScriptVariables(script, nil, task.Variables)
		if resolveErr != nil {
			return model.OpsExecTargetResult{HostID: host.ID, HostName: host.HostName, SSHIP: host.SSHIP, Status: "failed", ErrorText: resolveErr.Error()}
		}
		return s.execScriptOnHost(host, *script, "", variables, execTask.TimeoutSeconds)
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
	return status, firstNonEmpty(taskInfo.Summary, "execution completed"), strings.Join(lines, "\n"), taskInfo.ID
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
	return "failed", fmt.Sprintf("%s; expected HTTP status %d", summary, normalizeExpectedStatus(task.ExpectedStatus)), responseBody, response.StatusCode, responseBody
}
