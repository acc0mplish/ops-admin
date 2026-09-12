package service

import (
	"errors"
	"fmt"
	"path"
	"strings"
	"time"

	"ops-admin/backend/model"
)

func buildOpsJobNodeOrder(definition OpsJobDefinition) ([]OpsJobNode, error) {
	nodeMap := make(map[string]OpsJobNode, len(definition.Nodes))
	inDegree := make(map[string]int, len(definition.Nodes))
	nextMap := make(map[string][]string)
	for _, node := range definition.Nodes {
		nodeMap[node.ID] = node
		inDegree[node.ID] = 0
	}
	for _, edge := range definition.Edges {
		if edge.Source == "" || edge.Target == "" {
			continue
		}
		if _, ok := nodeMap[edge.Source]; !ok {
			continue
		}
		if _, ok := nodeMap[edge.Target]; !ok {
			continue
		}
		inDegree[edge.Target]++
		nextMap[edge.Source] = append(nextMap[edge.Source], edge.Target)
	}
	queue := make([]string, 0)
	for _, node := range definition.Nodes {
		if inDegree[node.ID] == 0 {
			queue = append(queue, node.ID)
		}
	}
	order := make([]OpsJobNode, 0, len(definition.Nodes))
	for len(queue) > 0 {
		currentID := queue[0]
		queue = queue[1:]
		order = append(order, nodeMap[currentID])
		for _, nextID := range nextMap[currentID] {
			inDegree[nextID]--
			if inDegree[nextID] == 0 {
				queue = append(queue, nextID)
			}
		}
	}
	if len(order) != len(definition.Nodes) {
		return nil, errors.New("job orchestration contains a cyclic dependency; review step connections")
	}
	return order, nil
}

func (s *Service) runOpsJobDefinition(historyID uint, definition OpsJobDefinition) {
	order, err := buildOpsJobNodeOrder(definition)
	if err != nil {
		s.finishOpsJobHistory(historyID, "failed", err.Error(), "", "")
		return
	}
	for _, node := range order {
		paused, failed := s.runSingleOpsJobNode(historyID, node)
		if paused || failed {
			return
		}
	}
	s.finishOpsJobHistory(historyID, "success", "job execution completed", "", "")
}

func (s *Service) resumeOpsJobDefinition(historyID uint, definition OpsJobDefinition, approvedStepID string) {
	order, err := buildOpsJobNodeOrder(definition)
	if err != nil {
		s.finishOpsJobHistory(historyID, "failed", err.Error(), "", "")
		return
	}
	afterApproved := false
	for _, node := range order {
		if !afterApproved {
			if node.ID == approvedStepID {
				afterApproved = true
			}
			continue
		}
		paused, failed := s.runSingleOpsJobNode(historyID, node)
		if paused || failed {
			return
		}
	}
	s.finishOpsJobHistory(historyID, "success", "job execution completed", "", "")
}

func (s *Service) runSingleOpsJobNode(historyID uint, node OpsJobNode) (paused bool, failed bool) {
	stepName := firstNonEmpty(strings.TrimSpace(node.Label), node.Type)
	now := time.Now()
	step := model.OpsJobHistoryStep{
		HistoryID: historyID,
		StepID:    node.ID,
		StepName:  stepName,
		StepType:  node.Type,
		Status:    "running",
		Summary:   "step execution started",
		StartedAt: &now,
	}
	if err := s.db.Create(&step).Error; err != nil {
		s.finishOpsJobHistory(historyID, "failed", err.Error(), node.ID, stepName)
		return false, true
	}
	_ = s.db.Model(&model.OpsJobHistory{}).Where("id = ?", historyID).Updates(map[string]any{
		"status":            "running",
		"summary":           fmt.Sprintf("executing step: %s", stepName),
		"current_step_id":   node.ID,
		"current_step_name": stepName,
	}).Error

	switch node.Type {
	case "notify":
		status, summary, output, err := s.executeOpsJobNotifyNode(historyID, stepName, node)
		finishedAt := time.Now()
		duration := finishedAt.Sub(now).Milliseconds()
		_ = s.db.Model(&model.OpsJobHistoryStep{}).Where("id = ?", step.ID).Updates(map[string]any{
			"status":      status,
			"summary":     summary,
			"output":      output,
			"finished_at": &finishedAt,
			"duration_ms": duration,
		}).Error
		if err != nil {
			s.finishOpsJobHistory(historyID, "failed", firstNonEmpty(summary, err.Error()), node.ID, stepName)
			return false, true
		}
	case "approval":
		_ = s.db.Model(&model.OpsJobHistoryStep{}).Where("id = ?", step.ID).Updates(map[string]any{
			"status":  "waiting_approval",
			"summary": firstNonEmpty(stringConfig(node.Config, "message"), "awaiting manual approval"),
			"output":  firstNonEmpty(stringConfig(node.Config, "content"), "approve to continue job execution"),
		}).Error
		_ = s.db.Model(&model.OpsJobHistory{}).Where("id = ?", historyID).Updates(map[string]any{
			"status":            "waiting_approval",
			"summary":           fmt.Sprintf("awaiting manual approval: %s", stepName),
			"current_step_id":   node.ID,
			"current_step_name": stepName,
		}).Error
		return true, false
	case "file":
		status, summary, output, execTaskID, err := s.executeOpsJobFileNode(stepName, node.Config)
		finishedAt := time.Now()
		duration := finishedAt.Sub(now).Milliseconds()
		updates := map[string]any{
			"status":       status,
			"summary":      summary,
			"output":       output,
			"exec_task_id": execTaskID,
			"finished_at":  &finishedAt,
			"duration_ms":  duration,
		}
		_ = s.db.Model(&model.OpsJobHistoryStep{}).Where("id = ?", step.ID).Updates(updates).Error
		if err != nil || status == "failed" {
			s.finishOpsJobHistory(historyID, "failed", firstNonEmpty(summary, errString(err)), node.ID, stepName)
			return false, true
		}
	default:
		status, summary, output, execTaskID, err := s.executeOpsJobScriptNode(stepName, node.Config)
		finishedAt := time.Now()
		duration := finishedAt.Sub(now).Milliseconds()
		updates := map[string]any{
			"status":       status,
			"summary":      summary,
			"output":       output,
			"exec_task_id": execTaskID,
			"finished_at":  &finishedAt,
			"duration_ms":  duration,
		}
		_ = s.db.Model(&model.OpsJobHistoryStep{}).Where("id = ?", step.ID).Updates(updates).Error
		if err != nil || status == "failed" {
			s.finishOpsJobHistory(historyID, "failed", firstNonEmpty(summary, errString(err)), node.ID, stepName)
			return false, true
		}
	}
	return false, false
}

func (s *Service) executeOpsJobScriptNode(stepName string, config map[string]any) (string, string, string, uint, error) {
	scriptID := uint(numberConfig(config, "scriptId"))
	if scriptID == 0 {
		return "failed", "missing script configuration", "", 0, errors.New("missing script configuration")
	}
	script, err := s.GetOpsScript(scriptID)
	if err != nil {
		return "failed", err.Error(), "", 0, err
	}
	hostIDs, groupIDs := opsJobTargetIDs(config)
	hosts, err := s.resolveOpsTargetHosts(hostIDs, groupIDs)
	if err != nil {
		return "failed", err.Error(), "", 0, err
	}
	params := strings.TrimSpace(stringConfig(config, "parameters"))
	task := model.OpsExecTask{
		TaskType:       "script",
		Title:          stepName,
		ScriptID:       script.ID,
		ScriptName:     script.Name,
		Parameters:     params,
		Concurrency:    normalizeOpsConcurrency(numberConfig(config, "concurrency")),
		TimeoutSeconds: normalizeOpsTimeout(script.TimeoutSeconds),
		Status:         "running",
		Summary:        "job step running",
		HostCount:      len(hosts),
		Operator:       "job-engine",
		Source:         "job",
		RiskLevel:      opsRiskLevel(script.Content),
		ScriptVersion:  script.CurrentVersion,
		TargetSnapshot: opsTargetSnapshot(hosts),
	}
	result, err := s.runOpsTaskLegacy(task, hosts, func(host model.AssetHost) model.OpsExecTargetResult {
		finalParams := params
		if finalParams == "" {
			finalParams = strings.TrimSpace(script.DefaultParams)
		}
		return s.execScriptOnHost(host, *script, finalParams, nil, task.TimeoutSeconds)
	})
	if err != nil {
		return "failed", err.Error(), "", 0, err
	}
	taskInfo, _ := result["task"].(model.OpsExecTask)
	rows, _ := result["results"].([]model.OpsExecTargetResult)
	outputs := make([]string, 0, len(rows))
	for _, row := range rows {
		line := fmt.Sprintf("%s [%s] exit=%d", row.HostName, row.Status, row.ExitCode)
		if strings.TrimSpace(row.ErrorText) != "" {
			line += " " + strings.TrimSpace(row.ErrorText)
		}
		outputs = append(outputs, line)
	}
	return taskInfo.Status, taskInfo.Summary, strings.Join(outputs, "\n"), taskInfo.ID, nil
}

func (s *Service) executeOpsJobFileNode(stepName string, config map[string]any) (string, string, string, uint, error) {
	sourceHostID := uint(numberConfig(config, "sourceHostId"))
	sourcePath := strings.TrimSpace(stringConfig(config, "sourcePath"))
	targetPath := strings.TrimSpace(stringConfig(config, "targetPath"))
	if sourceHostID == 0 || sourcePath == "" || targetPath == "" {
		return "failed", "file-distribution configuration is incomplete", "", 0, errors.New("file-distribution configuration is incomplete")
	}
	hostIDs, groupIDs := opsJobTargetIDs(config)
	hosts, err := s.resolveOpsTargetHosts(hostIDs, groupIDs)
	if err != nil {
		return "failed", err.Error(), "", 0, err
	}
	sourceHost, content, err := s.readRemoteFile(sourceHostID, sourcePath)
	if err != nil {
		return "failed", err.Error(), "", 0, err
	}
	task := model.OpsExecTask{
		TaskType:       "file",
		Title:          stepName,
		SourceType:     "server",
		SourceHostID:   sourceHostID,
		SourceHostName: sourceHost.HostName,
		SourcePath:     sourcePath,
		TargetPath:     targetPath,
		FileName:       path.Base(sourcePath),
		Concurrency:    normalizeOpsConcurrency(numberConfig(config, "concurrency")),
		TimeoutSeconds: normalizeOpsTimeout(numberConfig(config, "timeoutSeconds")),
		Status:         "running",
		Summary:        "job step running",
		HostCount:      len(hosts),
		Operator:       "job-engine",
		Source:         "job",
		RiskLevel:      "normal",
		TargetSnapshot: opsTargetSnapshot(hosts),
	}
	overwrite := boolConfig(config, "overwrite")
	result, err := s.runOpsTaskLegacy(task, hosts, func(host model.AssetHost) model.OpsExecTargetResult {
		return s.dispatchFileToHost(host, task.FileName, targetPath, content, overwrite, task.TimeoutSeconds)
	})
	if err != nil {
		return "failed", err.Error(), "", 0, err
	}
	taskInfo, _ := result["task"].(model.OpsExecTask)
	rows, _ := result["results"].([]model.OpsExecTargetResult)
	outputs := make([]string, 0, len(rows))
	for _, row := range rows {
		line := fmt.Sprintf("%s [%s] exit=%d", row.HostName, row.Status, row.ExitCode)
		if strings.TrimSpace(row.ErrorText) != "" {
			line += " " + strings.TrimSpace(row.ErrorText)
		}
		outputs = append(outputs, line)
	}
	return taskInfo.Status, taskInfo.Summary, strings.Join(outputs, "\n"), taskInfo.ID, nil
}

func (s *Service) executeOpsJobNotifyNode(historyID uint, stepName string, node OpsJobNode) (string, string, string, error) {
	ruleID := uint(numberConfig(node.Config, "notifyRuleId"))
	if ruleID == 0 {
		return "failed", "missing notification rule", "", errors.New("missing notification rule")
	}
	var history model.OpsJobHistory
	if err := s.db.First(&history, historyID).Error; err != nil {
		return "failed", err.Error(), "", err
	}
	var job model.OpsJob
	if err := s.db.First(&job, history.JobID).Error; err != nil {
		return "failed", err.Error(), "", err
	}
	now := time.Now()
	summary := firstNonEmpty(stringConfig(node.Config, "message"), stepName)
	detail := firstNonEmpty(stringConfig(node.Config, "content"), history.Summary)
	s.DispatchNotifyRule(ruleID, NotifyEvent{
		Scope:      "job",
		Event:      "notify",
		TargetID:   job.ID,
		TargetName: job.Name,
		Status:     "notice",
		Summary:    summary,
		Detail:     detail,
		StartedAt:  history.StartedAt,
		FinishedAt: &now,
		Extra: map[string]string{
			"jobName":         job.Name,
			"jobHistoryId":    fmt.Sprintf("%d", history.ID),
			"historyId":       fmt.Sprintf("%d", history.ID),
			"triggerType":     history.TriggerType,
			"stepName":        stepName,
			"stepMessage":     summary,
			"notifyAt":        now.Format("2006-01-02 15:04:05"),
			"currentStepId":   node.ID,
			"currentStepName": stepName,
		},
	})
	return "success", "notification triggered", detail, nil
}

func (s *Service) finishOpsJobHistory(historyID uint, status, summary, currentStepID, currentStepName string) {
	finishedAt := time.Now()
	_ = s.db.Model(&model.OpsJobHistory{}).Where("id = ?", historyID).Updates(map[string]any{
		"status":            status,
		"summary":           summary,
		"current_step_id":   currentStepID,
		"current_step_name": currentStepName,
		"finished_at":       &finishedAt,
	}).Error
}

func (s *Service) dispatchOpsJobNotification(historyID uint, event, summary, detail string) {
	var history model.OpsJobHistory
	if err := s.db.First(&history, historyID).Error; err != nil {
		return
	}
	var job model.OpsJob
	if err := s.db.First(&job, history.JobID).Error; err != nil {
		return
	}
	if !job.NotifyEnabled || job.NotifyRuleID == 0 {
		return
	}
	s.DispatchNotifyRule(job.NotifyRuleID, NotifyEvent{
		Scope:      "job",
		Event:      event,
		TargetID:   job.ID,
		TargetName: job.Name,
		Status:     event,
		Summary:    summary,
		Detail:     detail,
		StartedAt:  history.StartedAt,
		FinishedAt: history.FinishedAt,
		Extra: map[string]string{
			"jobName":         job.Name,
			"jobHistoryId":    fmt.Sprintf("%d", history.ID),
			"historyId":       fmt.Sprintf("%d", history.ID),
			"triggerType":     history.TriggerType,
			"stepName":        history.CurrentStepName,
			"currentStepId":   history.CurrentStepID,
			"currentStepName": history.CurrentStepName,
		},
	})
}

func stringConfig(config map[string]any, key string) string {
	if config == nil {
		return ""
	}
	value, ok := config[key]
	if !ok || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprintf("%v", value))
}

func numberConfig(config map[string]any, key string) int {
	if config == nil {
		return 0
	}
	value, ok := config[key]
	if !ok || value == nil {
		return 0
	}
	switch typed := value.(type) {
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case float32:
		return int(typed)
	default:
		var result int
		_, _ = fmt.Sscanf(fmt.Sprintf("%v", value), "%d", &result)
		return result
	}
}

func uintSliceConfig(config map[string]any, key string) []uint {
	if config == nil {
		return nil
	}
	raw, ok := config[key]
	if !ok || raw == nil {
		return nil
	}
	list := make([]uint, 0)
	switch typed := raw.(type) {
	case []any:
		for _, item := range typed {
			number := numberConfig(map[string]any{"value": item}, "value")
			if number > 0 {
				list = append(list, uint(number))
			}
		}
	case []uint:
		return typed
	case []int:
		for _, item := range typed {
			if item > 0 {
				list = append(list, uint(item))
			}
		}
	}
	return list
}

// Older job definitions may retain groupIds after the user switches to explicit hosts.
// Explicit host selection is the more specific scope, so it wins during execution.
func opsJobTargetIDs(config map[string]any) ([]uint, []uint) {
	hostIDs := uintSliceConfig(config, "hostIds")
	groupIDs := uintSliceConfig(config, "groupIds")
	if len(hostIDs) > 0 {
		return hostIDs, nil
	}
	return nil, groupIDs
}

func boolConfig(config map[string]any, key string) bool {
	if config == nil {
		return false
	}
	value, ok := config[key]
	if !ok || value == nil {
		return false
	}
	switch typed := value.(type) {
	case bool:
		return typed
	default:
		return strings.EqualFold(fmt.Sprintf("%v", value), "true")
	}
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
