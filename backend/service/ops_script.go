package service

import (
	"errors"
	"fmt"
	"path"
	"strings"

	"ops-admin/backend/model"

	"gorm.io/gorm"
)

func (s *Service) ListOpsScripts(pageNum, pageSize int, keyword string, status string) (map[string]any, error) {
	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	query := s.db.Model(&model.OpsScript{})
	if strings.TrimSpace(keyword) != "" {
		like := "%" + strings.TrimSpace(keyword) + "%"
		query = query.Where("name LIKE ? OR description LIKE ?", like, like)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var list []model.OpsScript
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

func (s *Service) ListOpsScriptOptions() ([]model.OpsScript, error) {
	var list []model.OpsScript
	if err := s.db.Where("status = ?", 1).Order("id DESC").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

func (s *Service) GetOpsScript(id uint) (*model.OpsScript, error) {
	var item model.OpsScript
	if err := s.db.First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Service) CreateOpsScript(payload OpsScriptPayload) error {
	variables, err := normalizeOpsScriptVariables(payload.Variables)
	if err != nil {
		return err
	}
	item := model.OpsScript{
		Name:           Trimmed(payload.Name),
		ScriptType:     normalizeOpsScriptType(payload.ScriptType),
		Interpreter:    normalizeOpsInterpreter(payload.Interpreter, payload.ScriptType),
		Content:        strings.TrimSpace(payload.Content),
		DefaultParams:  strings.TrimSpace(payload.DefaultParams),
		Variables:      model.OpsScriptVariables(variables),
		TimeoutSeconds: normalizeOpsScriptTimeout(payload.TimeoutSeconds),
		Status:         payload.Status,
		Description:    Trimmed(payload.Description),
		CurrentVersion: 1,
	}
	if item.Name == "" {
		return errors.New("script name is required")
	}
	if item.Content == "" {
		return errors.New("script content is required")
	}
	if item.Status == 0 {
		item.Status = 1
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&item).Error; err != nil {
			return err
		}
		return tx.Create(&model.OpsScriptVersion{
			ScriptID: item.ID, Version: 1, Content: item.Content, DefaultParams: item.DefaultParams, Variables: item.Variables,
			Interpreter: item.Interpreter, TimeoutSeconds: item.TimeoutSeconds,
			ChangeSummary: "Create Script", Operator: payload.Operator,
		}).Error
	})
}

func (s *Service) UpdateOpsScript(payload OpsScriptPayload) error {
	existing, err := s.GetOpsScript(payload.ID)
	if err != nil {
		return err
	}
	variables, err := normalizeOpsScriptVariables(payload.Variables)
	if err != nil {
		return err
	}
	updates := map[string]any{
		"name":            Trimmed(payload.Name),
		"script_type":     normalizeOpsScriptType(payload.ScriptType),
		"interpreter":     normalizeOpsInterpreter(payload.Interpreter, payload.ScriptType),
		"content":         strings.TrimSpace(payload.Content),
		"default_params":  strings.TrimSpace(payload.DefaultParams),
		"variables":       model.OpsScriptVariables(variables),
		"timeout_seconds": normalizeOpsScriptTimeout(payload.TimeoutSeconds),
		"status":          payload.Status,
		"description":     Trimmed(payload.Description),
	}
	if updates["name"] == "" {
		return errors.New("script name is required")
	}
	if updates["content"] == "" {
		return errors.New("script content is required")
	}
	if payload.Status == 0 {
		updates["status"] = existing.Status
	}
	nextVersion := existing.CurrentVersion + 1
	if nextVersion < 2 {
		nextVersion = 2
	}
	updates["current_version"] = nextVersion
	return s.db.Transaction(func(tx *gorm.DB) error {
		if existing.CurrentVersion <= 0 {
			if err := tx.Create(&model.OpsScriptVersion{ScriptID: existing.ID, Version: 1, Content: existing.Content, DefaultParams: existing.DefaultParams, Variables: existing.Variables, Interpreter: existing.Interpreter, TimeoutSeconds: existing.TimeoutSeconds, ChangeSummary: "Archive Historical Version", Operator: "system"}).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&model.OpsScript{}).Where("id = ?", payload.ID).Updates(updates).Error; err != nil {
			return err
		}
		return tx.Create(&model.OpsScriptVersion{
			ScriptID: payload.ID, Version: nextVersion, Content: updates["content"].(string),
			DefaultParams: updates["default_params"].(string), Variables: model.OpsScriptVariables(variables), Interpreter: updates["interpreter"].(string),
			TimeoutSeconds: updates["timeout_seconds"].(int), ChangeSummary: Trimmed(payload.ChangeSummary), Operator: payload.Operator,
		}).Error
	})
}

func (s *Service) ListOpsScriptVersions(scriptID uint) ([]model.OpsScriptVersion, error) {
	var list []model.OpsScriptVersion
	err := s.db.Where("script_id = ?", scriptID).Order("version DESC").Find(&list).Error
	if err == nil && len(list) == 0 {
		script, loadErr := s.GetOpsScript(scriptID)
		if loadErr != nil {
			return nil, loadErr
		}
		version := script.CurrentVersion
		if version <= 0 {
			version = 1
			_ = s.db.Model(&model.OpsScript{}).Where("id = ?", scriptID).Update("current_version", version).Error
		}
		row := model.OpsScriptVersion{ScriptID: scriptID, Version: version, Content: script.Content, DefaultParams: script.DefaultParams, Variables: script.Variables, Interpreter: script.Interpreter, TimeoutSeconds: script.TimeoutSeconds, ChangeSummary: "Archive Current Version", Operator: "system"}
		if createErr := s.db.Create(&row).Error; createErr != nil {
			return nil, createErr
		}
		list = []model.OpsScriptVersion{row}
	}
	return list, err
}

func (s *Service) RollbackOpsScript(scriptID uint, version int, operator string) error {
	var target model.OpsScriptVersion
	if err := s.db.Where("script_id = ? AND version = ?", scriptID, version).First(&target).Error; err != nil {
		return err
	}
	existing, err := s.GetOpsScript(scriptID)
	if err != nil {
		return err
	}
	nextVersion := existing.CurrentVersion + 1
	return s.db.Transaction(func(tx *gorm.DB) error {
		updates := map[string]any{"content": target.Content, "default_params": target.DefaultParams, "variables": target.Variables, "interpreter": target.Interpreter, "timeout_seconds": target.TimeoutSeconds, "current_version": nextVersion}
		if err := tx.Model(&model.OpsScript{}).Where("id = ?", scriptID).Updates(updates).Error; err != nil {
			return err
		}
		return tx.Create(&model.OpsScriptVersion{ScriptID: scriptID, Version: nextVersion, Content: target.Content, DefaultParams: target.DefaultParams, Variables: target.Variables, Interpreter: target.Interpreter, TimeoutSeconds: target.TimeoutSeconds, ChangeSummary: fmt.Sprintf("Rollback to v%d", version), Operator: operator}).Error
	})
}

func (s *Service) DeleteOpsScript(id uint) error {
	return s.db.Delete(&model.OpsScript{}, id).Error
}

func (s *Service) UpdateOpsScriptStatus(payload OpsScriptStatusPayload) error {
	if payload.Status == 0 {
		payload.Status = 2
	}
	return s.db.Model(&model.OpsScript{}).Where("id = ?", payload.ID).Update("status", payload.Status).Error
}

func (s *Service) ExecuteOpsCommand(payload OpsExecCommandPayload) (map[string]any, error) {
	command := strings.TrimSpace(payload.CommandText)
	if command == "" {
		return nil, errors.New("command is required")
	}
	riskLevel, err := requireOpsRiskConfirmation(command, payload.RiskConfirmed)
	if err != nil {
		return nil, err
	}
	hosts, err := s.resolveOpsTargetHosts(payload.HostIDs, payload.GroupIDs)
	if err != nil {
		return nil, err
	}
	if err := requireOpsProductionConfirmation(hosts, payload.RiskConfirmed); err != nil {
		return nil, err
	}
	task := model.OpsExecTask{
		TaskType:       "command",
		Title:          opsTaskTitle(payload.Title, "Command Execution"),
		CommandText:    command,
		Parameters:     strings.TrimSpace(payload.Parameters),
		Concurrency:    normalizeOpsConcurrency(payload.Concurrency),
		TimeoutSeconds: normalizeOpsTimeout(payload.TimeoutSeconds),
		Status:         "running",
		Summary:        "task created and running",
		HostCount:      len(hosts),
		Operator:       payload.Operator,
		SourceIP:       payload.SourceIP,
		Source:         opsTaskSource(payload.Source),
		RiskLevel:      riskLevel,
		TargetSnapshot: opsTargetSnapshot(hosts),
		RetryOfTaskID:  payload.RetryOfTaskID,
	}
	return s.runOpsTaskAsync(task, hosts, func(host model.AssetHost) model.OpsExecTargetResult {
		finalCommand := command
		if strings.TrimSpace(payload.Parameters) != "" {
			finalCommand += " " + strings.TrimSpace(payload.Parameters)
		}
		return s.execCommandOnHost(host, finalCommand, task.TimeoutSeconds)
	})
}

func (s *Service) ExecuteOpsScript(payload OpsExecScriptPayload) (map[string]any, error) {
	if payload.ScriptID == 0 {
		return nil, errors.New("select a script")
	}
	script, err := s.GetOpsScript(payload.ScriptID)
	if err != nil {
		return nil, err
	}
	if script.Status != 1 {
		return nil, errors.New("the selected script is disabled and cannot be executed")
	}
	riskLevel, err := requireOpsRiskConfirmation(script.Content, payload.RiskConfirmed)
	if err != nil {
		return nil, err
	}
	hosts, err := s.resolveOpsTargetHosts(payload.HostIDs, payload.GroupIDs)
	if err != nil {
		return nil, err
	}
	if err := requireOpsProductionConfirmation(hosts, payload.RiskConfirmed); err != nil {
		return nil, err
	}
	variables, err := resolveOpsScriptVariables([]model.OpsScriptVariable(script.Variables), payload.Variables)
	if err != nil {
		return nil, err
	}
	timeoutSeconds := payload.TimeoutSeconds
	if timeoutSeconds <= 0 {
		timeoutSeconds = script.TimeoutSeconds
	}
	task := model.OpsExecTask{
		TaskType:       "script",
		Title:          opsTaskTitle(payload.Title, "Script Execution"),
		ScriptID:       script.ID,
		ScriptName:     script.Name,
		Parameters:     strings.TrimSpace(payload.Parameters),
		Concurrency:    normalizeOpsConcurrency(payload.Concurrency),
		TimeoutSeconds: normalizeOpsTimeout(timeoutSeconds),
		Status:         "running",
		Summary:        "task created and running",
		HostCount:      len(hosts),
		Operator:       payload.Operator,
		SourceIP:       payload.SourceIP,
		Source:         opsTaskSource(payload.Source),
		RiskLevel:      riskLevel,
		ScriptVersion:  script.CurrentVersion,
		TargetSnapshot: opsTargetSnapshot(hosts),
		RetryOfTaskID:  payload.RetryOfTaskID,
	}
	return s.runOpsTaskAsync(task, hosts, func(host model.AssetHost) model.OpsExecTargetResult {
		params := strings.TrimSpace(payload.Parameters)
		if params == "" {
			params = strings.TrimSpace(script.DefaultParams)
		}
		return s.execScriptOnHost(host, *script, params, variables, task.TimeoutSeconds)
	})
}

func (s *Service) ExecuteOpsFileDispatch(payload OpsFileDispatchPayload, uploadName string, uploadBytes []byte) (map[string]any, error) {
	targetPath := strings.TrimSpace(payload.TargetPath)
	if targetPath == "" {
		return nil, errors.New("target path is required")
	}
	riskLevel := "normal"
	if strings.HasPrefix(targetPath, "/etc/") || strings.HasPrefix(targetPath, "/boot/") || strings.HasPrefix(targetPath, "/usr/") {
		riskLevel = "high"
		if !payload.RiskConfirmed {
			return nil, errors.New("target path is a system directory; confirm the target scope before execution")
		}
	}
	hosts, err := s.resolveOpsTargetHosts(payload.HostIDs, payload.GroupIDs)
	if err != nil {
		return nil, err
	}
	if err := requireOpsProductionConfirmation(hosts, payload.RiskConfirmed); err != nil {
		return nil, err
	}

	sourceType := strings.ToLower(strings.TrimSpace(payload.SourceType))
	var fileName string
	var content []byte
	var sourceHostName string

	switch sourceType {
	case "upload":
		if len(uploadBytes) == 0 {
			return nil, errors.New("upload a file to distribute")
		}
		content = uploadBytes
		fileName = strings.TrimSpace(uploadName)
	case "server":
		if payload.SourceHostID == 0 {
			return nil, errors.New("select a source server")
		}
		if strings.TrimSpace(payload.SourcePath) == "" {
			return nil, errors.New("source file path is required")
		}
		sourceHost, readBytes, err := s.readRemoteFile(payload.SourceHostID, payload.SourcePath)
		if err != nil {
			return nil, err
		}
		sourceHostName = sourceHost.HostName
		content = readBytes
		fileName = path.Base(strings.TrimSpace(payload.SourcePath))
	default:
		return nil, errors.New("unsupported file source type")
	}

	task := model.OpsExecTask{
		TaskType:       "file",
		Title:          opsTaskTitle(payload.Title, "File Distribution"),
		SourceType:     sourceType,
		SourceHostID:   payload.SourceHostID,
		SourceHostName: sourceHostName,
		SourcePath:     strings.TrimSpace(payload.SourcePath),
		TargetPath:     targetPath,
		FileName:       fileName,
		Concurrency:    normalizeOpsConcurrency(payload.Concurrency),
		TimeoutSeconds: normalizeOpsTimeout(payload.TimeoutSeconds),
		Status:         "running",
		Summary:        "task created and running",
		HostCount:      len(hosts),
		Operator:       payload.Operator,
		SourceIP:       payload.SourceIP,
		Source:         opsTaskSource(payload.Source),
		RiskLevel:      riskLevel,
		TargetSnapshot: opsTargetSnapshot(hosts),
	}
	return s.runOpsTaskAsync(task, hosts, func(host model.AssetHost) model.OpsExecTargetResult {
		return s.dispatchFileToHost(host, fileName, targetPath, content, payload.Overwrite, task.TimeoutSeconds)
	})
}
