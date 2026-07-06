package service

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"path"
	"slices"
	"strings"
	"sync"
	"time"

	"ops-admin/backend/model"

	"golang.org/x/crypto/ssh"
)

type OpsScriptPayload struct {
	ID             uint   `json:"id"`
	Name           string `json:"name"`
	ScriptType     string `json:"scriptType"`
	Interpreter    string `json:"interpreter"`
	Content        string `json:"content"`
	DefaultParams  string `json:"defaultParams"`
	TimeoutSeconds int    `json:"timeoutSeconds"`
	Status         int    `json:"status"`
	Description    string `json:"description"`
}

type OpsScriptStatusPayload struct {
	ID     uint `json:"id"`
	Status int  `json:"status"`
}

type OpsExecCommandPayload struct {
	Title          string `json:"title"`
	CommandText    string `json:"commandText"`
	Parameters     string `json:"parameters"`
	HostIDs        []uint `json:"hostIds"`
	GroupIDs       []uint `json:"groupIds"`
	Concurrency    int    `json:"concurrency"`
	TimeoutSeconds int    `json:"timeoutSeconds"`
}

type OpsExecScriptPayload struct {
	Title          string `json:"title"`
	ScriptID       uint   `json:"scriptId"`
	Parameters     string `json:"parameters"`
	HostIDs        []uint `json:"hostIds"`
	GroupIDs       []uint `json:"groupIds"`
	Concurrency    int    `json:"concurrency"`
	TimeoutSeconds int    `json:"timeoutSeconds"`
}

type OpsFileDispatchPayload struct {
	Title          string `json:"title"`
	SourceType     string `json:"sourceType"`
	SourceHostID   uint   `json:"sourceHostId"`
	SourcePath     string `json:"sourcePath"`
	TargetPath     string `json:"targetPath"`
	HostIDs        []uint `json:"hostIds"`
	GroupIDs       []uint `json:"groupIds"`
	Concurrency    int    `json:"concurrency"`
	TimeoutSeconds int    `json:"timeoutSeconds"`
	Overwrite      bool   `json:"overwrite"`
}

func normalizeOpsScriptType(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "python", "python3":
		return "python"
	default:
		return "shell"
	}
}

func normalizeOpsInterpreter(v string, scriptType string) string {
	interpreter := strings.ToLower(strings.TrimSpace(v))
	allowed := []string{"bash", "sh", "python", "python3"}
	if slices.Contains(allowed, interpreter) {
		return interpreter
	}
	if normalizeOpsScriptType(scriptType) == "python" {
		return "python3"
	}
	return "bash"
}

func normalizeOpsConcurrency(v int) int {
	if v <= 0 {
		return 5
	}
	if v > 10 {
		return 10
	}
	return v
}

func normalizeOpsTimeout(v int) int {
	if v <= 0 {
		return 10
	}
	if v < 10 {
		return 10
	}
	if v > 3600 {
		return 3600
	}
	return v
}

func shellQuote(v string) string {
	return "'" + strings.ReplaceAll(v, "'", `'\''`) + "'"
}

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
	item := model.OpsScript{
		Name:           Trimmed(payload.Name),
		ScriptType:     normalizeOpsScriptType(payload.ScriptType),
		Interpreter:    normalizeOpsInterpreter(payload.Interpreter, payload.ScriptType),
		Content:        strings.TrimSpace(payload.Content),
		DefaultParams:  strings.TrimSpace(payload.DefaultParams),
		TimeoutSeconds: payload.TimeoutSeconds,
		Status:         payload.Status,
		Description:    Trimmed(payload.Description),
	}
	if item.Name == "" {
		return errors.New("脚本名称不能为空")
	}
	if item.Content == "" {
		return errors.New("脚本内容不能为空")
	}
	if item.TimeoutSeconds <= 0 {
		item.TimeoutSeconds = 300
	}
	if item.Status == 0 {
		item.Status = 1
	}
	return s.db.Create(&item).Error
}

func (s *Service) UpdateOpsScript(payload OpsScriptPayload) error {
	existing, err := s.GetOpsScript(payload.ID)
	if err != nil {
		return err
	}
	updates := map[string]any{
		"name":            Trimmed(payload.Name),
		"script_type":     normalizeOpsScriptType(payload.ScriptType),
		"interpreter":     normalizeOpsInterpreter(payload.Interpreter, payload.ScriptType),
		"content":         strings.TrimSpace(payload.Content),
		"default_params":  strings.TrimSpace(payload.DefaultParams),
		"timeout_seconds": payload.TimeoutSeconds,
		"status":          payload.Status,
		"description":     Trimmed(payload.Description),
	}
	if updates["name"] == "" {
		return errors.New("脚本名称不能为空")
	}
	if updates["content"] == "" {
		return errors.New("脚本内容不能为空")
	}
	if payload.TimeoutSeconds <= 0 {
		updates["timeout_seconds"] = 300
	}
	if payload.Status == 0 {
		updates["status"] = existing.Status
	}
	return s.db.Model(&model.OpsScript{}).Where("id = ?", payload.ID).Updates(updates).Error
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
		return nil, errors.New("执行命令不能为空")
	}
	hosts, err := s.resolveOpsTargetHosts(payload.HostIDs, payload.GroupIDs)
	if err != nil {
		return nil, err
	}
	task := model.OpsExecTask{
		TaskType:       "command",
		Title:          opsTaskTitle(payload.Title, "命令执行"),
		CommandText:    command,
		Parameters:     strings.TrimSpace(payload.Parameters),
		Concurrency:    normalizeOpsConcurrency(payload.Concurrency),
		TimeoutSeconds: normalizeOpsTimeout(payload.TimeoutSeconds),
		Status:         "running",
		Summary:        "任务创建成功，正在执行中",
		HostCount:      len(hosts),
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
		return nil, errors.New("请选择脚本")
	}
	script, err := s.GetOpsScript(payload.ScriptID)
	if err != nil {
		return nil, err
	}
	if script.Status != 1 {
		return nil, errors.New("当前脚本已禁用，无法执行")
	}
	hosts, err := s.resolveOpsTargetHosts(payload.HostIDs, payload.GroupIDs)
	if err != nil {
		return nil, err
	}
	task := model.OpsExecTask{
		TaskType:       "script",
		Title:          opsTaskTitle(payload.Title, "脚本执行"),
		ScriptID:       script.ID,
		ScriptName:     script.Name,
		Parameters:     strings.TrimSpace(payload.Parameters),
		Concurrency:    normalizeOpsConcurrency(payload.Concurrency),
		TimeoutSeconds: normalizeOpsTimeout(script.TimeoutSeconds),
		Status:         "running",
		Summary:        "任务创建成功，正在执行中",
		HostCount:      len(hosts),
	}
	return s.runOpsTaskAsync(task, hosts, func(host model.AssetHost) model.OpsExecTargetResult {
		params := strings.TrimSpace(payload.Parameters)
		if params == "" {
			params = strings.TrimSpace(script.DefaultParams)
		}
		return s.execScriptOnHost(host, *script, params, task.TimeoutSeconds)
	})
}

func (s *Service) ExecuteOpsFileDispatch(payload OpsFileDispatchPayload, uploadName string, uploadBytes []byte) (map[string]any, error) {
	targetPath := strings.TrimSpace(payload.TargetPath)
	if targetPath == "" {
		return nil, errors.New("目标路径不能为空")
	}
	hosts, err := s.resolveOpsTargetHosts(payload.HostIDs, payload.GroupIDs)
	if err != nil {
		return nil, err
	}

	sourceType := strings.ToLower(strings.TrimSpace(payload.SourceType))
	var fileName string
	var content []byte
	var sourceHostName string

	switch sourceType {
	case "upload":
		if len(uploadBytes) == 0 {
			return nil, errors.New("请上传待分发文件")
		}
		content = uploadBytes
		fileName = strings.TrimSpace(uploadName)
	case "server":
		if payload.SourceHostID == 0 {
			return nil, errors.New("请选择源服务器")
		}
		if strings.TrimSpace(payload.SourcePath) == "" {
			return nil, errors.New("源文件路径不能为空")
		}
		sourceHost, readBytes, err := s.readRemoteFile(payload.SourceHostID, payload.SourcePath)
		if err != nil {
			return nil, err
		}
		sourceHostName = sourceHost.HostName
		content = readBytes
		fileName = path.Base(strings.TrimSpace(payload.SourcePath))
	default:
		return nil, errors.New("不支持的文件来源类型")
	}

	task := model.OpsExecTask{
		TaskType:       "file",
		Title:          opsTaskTitle(payload.Title, "文件分发"),
		SourceType:     sourceType,
		SourceHostID:   payload.SourceHostID,
		SourceHostName: sourceHostName,
		SourcePath:     strings.TrimSpace(payload.SourcePath),
		TargetPath:     targetPath,
		FileName:       fileName,
		Concurrency:    normalizeOpsConcurrency(payload.Concurrency),
		TimeoutSeconds: normalizeOpsTimeout(payload.TimeoutSeconds),
		Status:         "running",
		Summary:        "任务创建成功，正在执行中",
		HostCount:      len(hosts),
	}
	return s.runOpsTaskAsync(task, hosts, func(host model.AssetHost) model.OpsExecTargetResult {
		return s.dispatchFileToHost(host, fileName, targetPath, content, payload.Overwrite, task.TimeoutSeconds)
	})
}

func (s *Service) ListOpsExecTasks(pageNum, pageSize int, keyword, taskType, status string) (map[string]any, error) {
	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	query := s.db.Model(&model.OpsExecTask{})
	if strings.TrimSpace(keyword) != "" {
		like := "%" + strings.TrimSpace(keyword) + "%"
		query = query.Where("title LIKE ? OR script_name LIKE ? OR file_name LIKE ? OR command_text LIKE ?", like, like, like, like)
	}
	if taskType != "" {
		query = query.Where("task_type = ?", taskType)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var list []model.OpsExecTask
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

func (s *Service) GetOpsExecTaskDetail(id uint) (map[string]any, error) {
	var task model.OpsExecTask
	if err := s.db.First(&task, id).Error; err != nil {
		return nil, err
	}
	var results []model.OpsExecTargetResult
	if err := s.db.Where("task_id = ?", id).Order("id ASC").Find(&results).Error; err != nil {
		return nil, err
	}
	return map[string]any{
		"task":    task,
		"results": results,
	}, nil
}

func opsTaskTitle(title string, fallback string) string {
	value := strings.TrimSpace(title)
	if value != "" {
		return value
	}
	return fmt.Sprintf("%s %s", fallback, time.Now().Format("2006-01-02 15:04:05"))
}

func (s *Service) resolveOpsTargetHosts(hostIDs []uint, groupIDs []uint) ([]model.AssetHost, error) {
	if len(hostIDs) > 0 && len(groupIDs) > 0 {
		return nil, errors.New("目标主机和主机组只能二选一")
	}
	hostMap := map[uint]model.AssetHost{}

	if len(hostIDs) > 0 {
		var hosts []model.AssetHost
		if err := s.db.Preload("Group").Preload("Credential").Where("id IN ?", hostIDs).Find(&hosts).Error; err != nil {
			return nil, err
		}
		for _, host := range hosts {
			hostMap[host.ID] = host
		}
	}

	if len(groupIDs) > 0 {
		var relationIDs []uint
		if err := s.db.Model(&model.AssetHostGroupRelation{}).Where("group_id IN ?", groupIDs).Distinct().Pluck("host_id", &relationIDs).Error; err != nil {
			return nil, err
		}
		if len(relationIDs) > 0 {
			var hosts []model.AssetHost
			if err := s.db.Preload("Group").Preload("Credential").Where("id IN ?", relationIDs).Find(&hosts).Error; err != nil {
				return nil, err
			}
			for _, host := range hosts {
				hostMap[host.ID] = host
			}
		}
	}

	if len(hostMap) == 0 {
		return nil, errors.New("请至少选择一台主机或一个主机组")
	}

	hosts := make([]model.AssetHost, 0, len(hostMap))
	for _, host := range hostMap {
		hosts = append(hosts, host)
	}
	slices.SortFunc(hosts, func(a, b model.AssetHost) int {
		return int(a.ID) - int(b.ID)
	})
	return hosts, nil
}

func (s *Service) runOpsTaskLegacy(task model.OpsExecTask, hosts []model.AssetHost, runner func(host model.AssetHost) model.OpsExecTargetResult) (map[string]any, error) {
	now := time.Now()
	task.StartedAt = &now
	if err := s.db.Create(&task).Error; err != nil {
		return nil, err
	}

	semaphore := make(chan struct{}, task.Concurrency)
	results := make([]model.OpsExecTargetResult, len(hosts))
	var wg sync.WaitGroup

	for index, host := range hosts {
		wg.Add(1)
		go func(i int, item model.AssetHost) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			result := runner(item)
			result.TaskID = task.ID
			results[i] = result
			_ = s.db.Create(&result).Error
		}(index, host)
	}
	wg.Wait()

	successCount := 0
	failedCount := 0
	for _, result := range results {
		if result.Status == "success" {
			successCount++
		} else {
			failedCount++
		}
	}
	finishedAt := time.Now()
	status := "success"
	switch {
	case successCount == 0:
		status = "failed"
	case failedCount > 0:
		status = "partial"
	}
	summary := fmt.Sprintf("成功 %d 台，失败 %d 台", successCount, failedCount)
	_ = s.db.Model(&model.OpsExecTask{}).Where("id = ?", task.ID).Updates(map[string]any{
		"success_count": successCount,
		"failed_count":  failedCount,
		"status":        status,
		"summary":       summary,
		"finished_at":   &finishedAt,
	}).Error

	return s.GetOpsExecTaskDetail(task.ID)
}

func (s *Service) runOpsTaskAsync(task model.OpsExecTask, hosts []model.AssetHost, runner func(host model.AssetHost) model.OpsExecTargetResult) (map[string]any, error) {
	now := time.Now()
	task.StartedAt = &now
	if err := s.db.Create(&task).Error; err != nil {
		return nil, err
	}
	pendingRows := make([]model.OpsExecTargetResult, 0, len(hosts))
	for _, host := range hosts {
		pendingRows = append(pendingRows, model.OpsExecTargetResult{
			TaskID:    task.ID,
			HostID:    host.ID,
			HostName:  host.HostName,
			GroupName: host.Group.Name,
			SSHIP:     host.SSHIP,
			Status:    "running",
		})
	}
	if len(pendingRows) > 0 {
		_ = s.db.Create(&pendingRows).Error
	}
	go s.processOpsTask(task, hosts, runner)
	return s.GetOpsExecTaskDetail(task.ID)
}

func (s *Service) processOpsTask(task model.OpsExecTask, hosts []model.AssetHost, runner func(host model.AssetHost) model.OpsExecTargetResult) {
	semaphore := make(chan struct{}, task.Concurrency)
	var wg sync.WaitGroup

	for _, host := range hosts {
		wg.Add(1)
		go func(item model.AssetHost) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			result := runner(item)
			_ = s.db.Model(&model.OpsExecTargetResult{}).
				Where("task_id = ? AND host_id = ?", task.ID, item.ID).
				Updates(map[string]any{
					"status":      result.Status,
					"exit_code":   result.ExitCode,
					"duration_ms": result.DurationMs,
					"stdout":      result.Stdout,
					"stderr":      result.Stderr,
					"error_text":  result.ErrorText,
				}).Error
			s.refreshOpsTaskProgress(task.ID)
		}(host)
	}
	wg.Wait()
	s.finishOpsTask(task.ID)
}

func (s *Service) refreshOpsTaskProgress(taskID uint) {
	var successCount int64
	var failedCount int64
	var runningCount int64
	_ = s.db.Model(&model.OpsExecTargetResult{}).Where("task_id = ? AND status = ?", taskID, "success").Count(&successCount).Error
	_ = s.db.Model(&model.OpsExecTargetResult{}).Where("task_id = ? AND status = ?", taskID, "running").Count(&runningCount).Error
	_ = s.db.Model(&model.OpsExecTargetResult{}).Where("task_id = ? AND status NOT IN ?", taskID, []string{"success", "running"}).Count(&failedCount).Error
	completedCount := successCount + failedCount
	summary := fmt.Sprintf("已完成 %d 台，成功 %d 台，失败 %d 台，执行中 %d 台", completedCount, successCount, failedCount, runningCount)
	_ = s.db.Model(&model.OpsExecTask{}).Where("id = ?", taskID).Updates(map[string]any{
		"success_count": int(successCount),
		"failed_count":  int(failedCount),
		"summary":       summary,
	}).Error
}

func (s *Service) finishOpsTask(taskID uint) {
	var successCount int64
	var failedCount int64
	_ = s.db.Model(&model.OpsExecTargetResult{}).Where("task_id = ? AND status = ?", taskID, "success").Count(&successCount).Error
	_ = s.db.Model(&model.OpsExecTargetResult{}).Where("task_id = ? AND status NOT IN ?", taskID, []string{"success", "running"}).Count(&failedCount).Error
	finishedAt := time.Now()
	status := "success"
	switch {
	case successCount == 0:
		status = "failed"
	case failedCount > 0:
		status = "partial"
	}
	summary := fmt.Sprintf("执行完成，成功 %d 台，失败 %d 台", successCount, failedCount)
	_ = s.db.Model(&model.OpsExecTask{}).Where("id = ?", taskID).Updates(map[string]any{
		"success_count": int(successCount),
		"failed_count":  int(failedCount),
		"status":        status,
		"summary":       summary,
		"finished_at":   &finishedAt,
	}).Error
}

func (s *Service) execCommandOnHost(host model.AssetHost, command string, timeoutSeconds int) model.OpsExecTargetResult {
	started := time.Now()
	result := model.OpsExecTargetResult{
		HostID:    host.ID,
		HostName:  host.HostName,
		GroupName: host.Group.Name,
		SSHIP:     host.SSHIP,
		Status:    "failed",
	}
	client, err := newSSHClient(host)
	if err != nil {
		result.ErrorText = err.Error()
		result.DurationMs = time.Since(started).Milliseconds()
		return result
	}
	defer client.Close()

	stdout, stderr, exitCode, runErr := runSSHCommandDetailed(client, "bash -lc "+shellQuote(command), timeoutSeconds)
	result.Stdout = stdout
	result.Stderr = stderr
	result.ExitCode = exitCode
	result.DurationMs = time.Since(started).Milliseconds()
	if runErr == nil {
		result.Status = "success"
	} else {
		result.ErrorText = runErr.Error()
	}
	return result
}

func (s *Service) execScriptOnHost(host model.AssetHost, script model.OpsScript, params string, timeoutSeconds int) model.OpsExecTargetResult {
	encoded := base64.StdEncoding.EncodeToString([]byte(script.Content))
	interpreter := normalizeOpsInterpreter(script.Interpreter, script.ScriptType)
	command := fmt.Sprintf(
		"tmp=$(mktemp /tmp/ops-script-XXXXXX) && base64 -d <<'__OPS_SCRIPT__' > \"$tmp\"\n%s\n__OPS_SCRIPT__\nchmod +x \"$tmp\"\n%s \"$tmp\" %s\nstatus=$?\nrm -f \"$tmp\"\nexit $status",
		encoded,
		interpreter,
		strings.TrimSpace(params),
	)
	return s.execCommandOnHost(host, command, timeoutSeconds)
}

func (s *Service) dispatchFileToHost(host model.AssetHost, fileName, targetPath string, content []byte, overwrite bool, timeoutSeconds int) model.OpsExecTargetResult {
	started := time.Now()
	result := model.OpsExecTargetResult{
		HostID:    host.ID,
		HostName:  host.HostName,
		GroupName: host.Group.Name,
		SSHIP:     host.SSHIP,
		Status:    "failed",
	}
	client, err := newSSHClient(host)
	if err != nil {
		result.ErrorText = err.Error()
		result.DurationMs = time.Since(started).Milliseconds()
		return result
	}
	defer client.Close()

	target := strings.TrimSpace(targetPath)
	dir := path.Dir(target)
	existsCheck := ""
	if !overwrite {
		existsCheck = fmt.Sprintf("if [ -e %s ]; then echo 'target file already exists'; exit 17; fi\n", shellQuote(target))
	}
	command := fmt.Sprintf(
		"mkdir -p %s\n%sbase64 -d <<'__OPS_FILE__' > %s\n%s\n__OPS_FILE__\n",
		shellQuote(dir),
		existsCheck,
		shellQuote(target),
		base64.StdEncoding.EncodeToString(content),
	)
	stdout, stderr, exitCode, runErr := runSSHCommandDetailed(client, "bash -lc "+shellQuote(command), timeoutSeconds)
	result.Stdout = stdout
	result.Stderr = stderr
	result.ExitCode = exitCode
	result.DurationMs = time.Since(started).Milliseconds()
	if runErr == nil {
		result.Status = "success"
		result.Stdout = strings.TrimSpace(strings.Join([]string{stdout, fmt.Sprintf("distributed to %s", target), fileName}, "\n"))
	} else {
		result.ErrorText = runErr.Error()
	}
	return result
}

func (s *Service) readRemoteFile(hostID uint, sourcePath string) (*model.AssetHost, []byte, error) {
	var host model.AssetHost
	if err := s.db.Preload("Credential").First(&host, hostID).Error; err != nil {
		return nil, nil, err
	}
	client, err := newSSHClient(host)
	if err != nil {
		return nil, nil, err
	}
	defer client.Close()

	command := fmt.Sprintf("if [ ! -f %s ]; then echo '__OPS_FILE_NOT_FOUND__'; exit 22; fi\nbase64 < %s", shellQuote(sourcePath), shellQuote(sourcePath))
	stdout, stderr, _, runErr := runSSHCommandDetailed(client, "bash -lc "+shellQuote(command), 30)
	if runErr != nil {
		return nil, nil, errors.New(strings.TrimSpace(strings.Join([]string{stderr, runErr.Error()}, "\n")))
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(stdout))
	if err != nil {
		return nil, nil, err
	}
	return &host, decoded, nil
}

func runSSHCommandDetailed(client *ssh.Client, command string, timeoutSeconds int) (string, string, int, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", "", -1, err
	}
	defer session.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	if timeoutSeconds <= 0 {
		timeoutSeconds = 10
	}
	if err := session.Start(command); err != nil {
		return "", "", -1, err
	}
	done := make(chan error, 1)
	go func() {
		done <- session.Wait()
	}()

	select {
	case err = <-done:
	case <-time.After(time.Duration(timeoutSeconds) * time.Second):
		_ = session.Signal(ssh.SIGKILL)
		_ = session.Close()
		err = fmt.Errorf("执行超时，已在 %d 秒后终止", timeoutSeconds)
	}
	exitCode := 0
	if err != nil {
		var exitErr *ssh.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitStatus()
		} else if strings.Contains(err.Error(), "执行超时") {
			exitCode = 124
		} else {
			exitCode = -1
		}
	}
	return stdout.String(), stderr.String(), exitCode, err
}
