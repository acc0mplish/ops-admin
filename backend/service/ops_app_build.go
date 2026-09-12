package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"ops-admin/backend/model"
)

type OpsAppBuildTaskPayload struct {
	ID             uint                             `json:"id"`
	AppID          uint                             `json:"appId"`
	Name           string                           `json:"name"`
	Env            string                           `json:"env"`
	Branch         string                           `json:"branch"`
	BuildScript    string                           `json:"buildScript"`
	DeployScript   string                           `json:"deployScript"`
	BuildParams    []OpsAppBuildParameterDefinition `json:"buildParams"`
	RunnerType     string                           `json:"runnerType"`
	RunnerHostID   uint                             `json:"runnerHostId"`
	ExecutionPath  string                           `json:"executionPath"`
	ArtifactType   string                           `json:"artifactType"`
	ArtifactPath   string                           `json:"artifactPath"`
	TimeoutSeconds int                              `json:"timeoutSeconds"`
	Status         int                              `json:"status"`
	Description    string                           `json:"description"`
}

type OpsAppBuildTaskStatusPayload struct {
	ID     uint `json:"id"`
	Status int  `json:"status"`
}

type OpsAppBuildRunPayload struct {
	TaskID  uint           `json:"taskId"`
	Version string         `json:"version"`
	Branch  string         `json:"branch"`
	Params  map[string]any `json:"params"`
}

type OpsAppBuildParameterDefinition struct {
	Name        string   `json:"name"`
	Label       string   `json:"label"`
	Type        string   `json:"type"`
	Default     any      `json:"default"`
	Options     []string `json:"options"`
	Required    bool     `json:"required"`
	Description string   `json:"description"`
}

type OpsAppReleasePayload struct {
	AppID   uint   `json:"appId"`
	Version string `json:"version"`
	Branch  string `json:"branch"`
}

type OpsImageRegistryPayload struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Address     string `json:"address"`
	Namespace   string `json:"namespace"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	Status      int    `json:"status"`
	Description string `json:"description"`
}

type opsBuildExecution struct {
	ReleaseID      uint
	App            model.OpsApplication
	TaskID         uint
	TaskName       string
	Env            string
	BuildScript    string
	DeployScript   string
	TimeoutSeconds int
	Branch         string
	Workspace      string
	RunnerType     string
	RunnerHostID   uint
	ArtifactType   string
	ArtifactPath   string
	Version        string
	Params         map[string]string
}

// opsBuildLogWriter keeps the persisted log usable while a build is still
// running. The database write is throttled so noisy build tools do not cause a
// write per output chunk.
type opsBuildLogWriter struct {
	service     *Service
	releaseID   uint
	column      string
	mu          sync.Mutex
	content     strings.Builder
	lastPersist time.Time
}

func newOpsBuildLogWriter(service *Service, releaseID uint, column string) *opsBuildLogWriter {
	return &opsBuildLogWriter{service: service, releaseID: releaseID, column: column}
}

func (writer *opsBuildLogWriter) Append(text string) {
	if text == "" {
		return
	}
	writer.mu.Lock()
	defer writer.mu.Unlock()
	writer.content.WriteString(text)
	if time.Since(writer.lastPersist) >= 500*time.Millisecond {
		writer.persistLocked()
	}
}

func (writer *opsBuildLogWriter) Flush() {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	writer.persistLocked()
}

func (writer *opsBuildLogWriter) String() string {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	return writer.content.String()
}

func (writer *opsBuildLogWriter) persistLocked() {
	_ = writer.service.db.Model(&model.OpsAppRelease{}).Where("id = ?", writer.releaseID).Update(writer.column, writer.content.String()).Error
	writer.lastPersist = time.Now()
}

var opsAppBuildParamNamePattern = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)

func normalizeOpsAppBuildParameters(input []OpsAppBuildParameterDefinition) ([]OpsAppBuildParameterDefinition, string, error) {
	result := make([]OpsAppBuildParameterDefinition, 0, len(input))
	seen := map[string]struct{}{}
	for _, item := range input {
		item.Name = strings.ToUpper(strings.TrimSpace(item.Name))
		item.Label = strings.TrimSpace(item.Label)
		item.Description = strings.TrimSpace(item.Description)
		if item.Name == "" {
			continue
		}
		if !opsAppBuildParamNamePattern.MatchString(item.Name) {
			return nil, "", fmt.Errorf("invalid build parameter %s; use uppercase letters, digits, and underscores only", item.Name)
		}
		if _, exists := seen[item.Name]; exists {
			return nil, "", fmt.Errorf("duplicate build parameter %s", item.Name)
		}
		seen[item.Name] = struct{}{}
		switch item.Type {
		case "select", "multiSelect", "boolean":
		default:
			item.Type = "text"
		}
		if item.Label == "" {
			item.Label = item.Name
		}
		options := make([]string, 0, len(item.Options))
		for _, option := range item.Options {
			if value := strings.TrimSpace(option); value != "" {
				options = append(options, value)
			}
		}
		item.Options = options
		result = append(result, item)
	}
	encoded, err := json.Marshal(result)
	return result, string(encoded), err
}

func normalizeOpsAppBuildParametersJSON(raw string) ([]OpsAppBuildParameterDefinition, string, error) {
	if strings.TrimSpace(raw) == "" {
		return []OpsAppBuildParameterDefinition{}, "[]", nil
	}
	var input []OpsAppBuildParameterDefinition
	if err := json.Unmarshal([]byte(raw), &input); err != nil {
		return nil, "", errors.New("invalid build parameter configuration format")
	}
	return normalizeOpsAppBuildParameters(input)
}

func resolveOpsAppBuildParams(definitions []OpsAppBuildParameterDefinition, values map[string]any) (map[string]string, error) {
	result := make(map[string]string, len(definitions))
	for _, definition := range definitions {
		value, exists := values[definition.Name]
		if !exists || value == nil {
			value = definition.Default
		}
		text := opsAppBuildParamValue(value)
		if definition.Required && strings.TrimSpace(text) == "" {
			return nil, fmt.Errorf("build parameter %s is required", definition.Label)
		}
		if definition.Type == "select" && text != "" && len(definition.Options) > 0 && !containsString(definition.Options, text) {
			return nil, fmt.Errorf("value for build parameter %s is outside the allowed options", definition.Label)
		}
		result[definition.Name] = text
	}
	return result, nil
}

func opsAppBuildParamValue(value any) string {
	switch typed := value.(type) {
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			parts = append(parts, fmt.Sprint(item))
		}
		return strings.Join(parts, ",")
	case []string:
		return strings.Join(typed, ",")
	case bool:
		return strconv.FormatBool(typed)
	case nil:
		return ""
	default:
		return fmt.Sprint(typed)
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func (s *Service) ListOpsAppBuildTasks(pageNum, pageSize int, appID uint, keyword, env, status string) (map[string]any, error) {
	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	query := s.db.Model(&model.OpsAppBuildTask{})
	if appID > 0 {
		query = query.Where("app_id = ?", appID)
	}
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("name LIKE ? OR app_name LIKE ? OR app_code LIKE ? OR description LIKE ?", like, like, like, like)
	}
	if env = strings.TrimSpace(env); env != "" {
		query = query.Where("env = ?", env)
	}
	if status = strings.TrimSpace(status); status != "" {
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var list []model.OpsAppBuildTask
	if err := query.Order("id DESC").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, err
	}
	s.hydrateOpsAppBuildTaskExecutionPaths(list)
	return map[string]any{"list": list, "total": total, "pageNum": pageNum, "pageSize": pageSize}, nil
}

func (s *Service) GetOpsAppBuildTask(id uint) (*model.OpsAppBuildTask, error) {
	var item model.OpsAppBuildTask
	if err := s.db.First(&item, id).Error; err != nil {
		return nil, err
	}
	if strings.TrimSpace(item.ExecutionPath) == "" {
		if app, err := s.GetOpsApplication(item.AppID); err == nil {
			item.ExecutionPath = s.resolveOpsAppWorkspace(*app)
		}
	}
	return &item, nil
}

func (s *Service) hydrateOpsAppBuildTaskExecutionPaths(list []model.OpsAppBuildTask) {
	appIDs := make([]uint, 0)
	seen := make(map[uint]struct{})
	for i := range list {
		if strings.TrimSpace(list[i].ExecutionPath) != "" || list[i].AppID == 0 {
			continue
		}
		if _, exists := seen[list[i].AppID]; !exists {
			seen[list[i].AppID] = struct{}{}
			appIDs = append(appIDs, list[i].AppID)
		}
	}
	if len(appIDs) == 0 {
		return
	}
	var apps []model.OpsApplication
	if err := s.db.Where("id IN ?", appIDs).Find(&apps).Error; err != nil {
		return
	}
	byID := make(map[uint]model.OpsApplication, len(apps))
	for _, app := range apps {
		byID[app.ID] = app
	}
	for i := range list {
		if strings.TrimSpace(list[i].ExecutionPath) == "" {
			if app, exists := byID[list[i].AppID]; exists {
				list[i].ExecutionPath = s.resolveOpsAppWorkspace(app)
			}
		}
	}
}

func (s *Service) SaveOpsAppBuildTask(payload OpsAppBuildTaskPayload) error {
	if payload.AppID == 0 {
		return errors.New("select an application")
	}
	app, err := s.GetOpsApplication(payload.AppID)
	if err != nil {
		return err
	}
	_, buildParamsJSON, err := normalizeOpsAppBuildParameters(payload.BuildParams)
	if err != nil {
		return err
	}
	task := model.OpsAppBuildTask{
		Name:            Trimmed(payload.Name),
		AppID:           app.ID,
		AppName:         app.Name,
		AppCode:         app.Code,
		Env:             Trimmed(payload.Env),
		Branch:          Trimmed(payload.Branch),
		BuildScript:     strings.TrimSpace(payload.BuildScript),
		DeployScript:    strings.TrimSpace(payload.DeployScript),
		BuildParamsJSON: buildParamsJSON,
		RunnerType:      strings.ToLower(Trimmed(payload.RunnerType)),
		RunnerHostID:    payload.RunnerHostID,
		ExecutionPath:   Trimmed(payload.ExecutionPath),
		ArtifactType:    strings.ToLower(Trimmed(payload.ArtifactType)),
		ArtifactPath:    Trimmed(payload.ArtifactPath),
		TimeoutSeconds:  normalizeOpsBuildTimeout(payload.TimeoutSeconds),
		Status:          normalizeOpsAppStatus(payload.Status),
		Description:     Trimmed(payload.Description),
	}
	if task.Name == "" {
		return errors.New("build task name is required")
	}
	if task.BuildScript == "" {
		return errors.New("build script is required")
	}
	if task.Env == "" {
		task.Env = app.Env
	}
	if task.Env == "" {
		task.Env = "test"
	}
	if task.Branch == "" {
		task.Branch = app.Branch
	}
	if task.Branch == "" && app.RepoType == "git" {
		task.Branch = "master"
	}
	if task.RunnerType == "" {
		task.RunnerType = "local"
	}
	if task.RunnerType != "local" && task.RunnerType != "host" {
		return errors.New("unsupported build executor type")
	}
	if task.RunnerType == "host" && task.RunnerHostID == 0 {
		return errors.New("select a build host")
	}
	if task.ExecutionPath == "" {
		task.ExecutionPath = s.resolveOpsAppWorkspace(*app)
	}
	if task.ArtifactType == "" {
		task.ArtifactType = "file"
	}
	if payload.ID == 0 {
		return s.db.Create(&task).Error
	}
	return s.db.Model(&model.OpsAppBuildTask{}).Where("id = ?", payload.ID).Updates(map[string]any{
		"name": task.Name, "app_id": task.AppID, "app_name": task.AppName, "app_code": task.AppCode,
		"env": task.Env, "branch": task.Branch, "build_script": task.BuildScript, "deploy_script": task.DeployScript,
		"build_params_json": task.BuildParamsJSON,
		"runner_type":       task.RunnerType, "runner_host_id": task.RunnerHostID, "execution_path": task.ExecutionPath,
		"artifact_type": task.ArtifactType, "artifact_path": task.ArtifactPath,
		"timeout_seconds": task.TimeoutSeconds, "status": task.Status, "description": task.Description,
	}).Error
}

func (s *Service) UpdateOpsAppBuildTaskStatus(payload OpsAppBuildTaskStatusPayload) error {
	if payload.ID == 0 {
		return errors.New("build task ID is required")
	}
	return s.db.Model(&model.OpsAppBuildTask{}).Where("id = ?", payload.ID).Update("status", normalizeOpsAppStatus(payload.Status)).Error
}

func (s *Service) DeleteOpsAppBuildTask(id uint) error {
	return s.db.Delete(&model.OpsAppBuildTask{}, id).Error
}

func (s *Service) RunOpsAppBuildTask(payload OpsAppBuildRunPayload) (map[string]any, error) {
	task, err := s.GetOpsAppBuildTask(payload.TaskID)
	if err != nil {
		return nil, err
	}
	if task.Status != 1 {
		return nil, errors.New("the current build task is disabled")
	}
	app, err := s.GetOpsApplication(task.AppID)
	if err != nil {
		return nil, err
	}
	if app.Status != 1 {
		return nil, errors.New("the current application is disabled and cannot be built")
	}
	branch := Trimmed(payload.Branch)
	if branch == "" {
		branch = task.Branch
	}
	if branch == "" {
		branch = app.Branch
	}
	version := Trimmed(payload.Version)
	if version == "" {
		version = time.Now().Format("20060102150405")
	}
	definitions, _, err := normalizeOpsAppBuildParametersJSON(task.BuildParamsJSON)
	if err != nil {
		return nil, err
	}
	params, err := resolveOpsAppBuildParams(definitions, payload.Params)
	if err != nil {
		return nil, err
	}
	paramsJSON, _ := json.Marshal(params)
	now := time.Now()
	workspace := strings.TrimSpace(task.ExecutionPath)
	if workspace == "" {
		workspace = s.resolveOpsAppWorkspace(*app)
	}
	release := model.OpsAppRelease{
		AppID: app.ID, AppName: app.Name, AppCode: app.Code, BuildTaskID: task.ID, BuildTaskName: task.Name,
		Env: task.Env, Version: version, RepoType: app.RepoType, RepoURL: app.RepoURL, Branch: branch,
		Workspace: workspace, Status: "running", Stage: "checkout", Summary: "build task created; checking out source code", ParamsJSON: string(paramsJSON), StartedAt: &now,
	}
	if err := s.db.Create(&release).Error; err != nil {
		return nil, err
	}
	_ = s.db.Model(&model.OpsAppBuildTask{}).Where("id = ?", task.ID).Updates(map[string]any{
		"last_release_id": release.ID, "last_status": "running", "last_run_at": &now,
	})
	_ = s.db.Model(&model.OpsApplication{}).Where("id = ?", app.ID).Updates(map[string]any{
		"last_release_id": release.ID, "last_status": "running",
	})
	go s.runOpsAppBuild(opsBuildExecution{
		ReleaseID: release.ID, App: *app, TaskID: task.ID, TaskName: task.Name, Env: task.Env,
		BuildScript: task.BuildScript, DeployScript: task.DeployScript, TimeoutSeconds: task.TimeoutSeconds,
		Branch: branch, Workspace: workspace, RunnerType: task.RunnerType, RunnerHostID: task.RunnerHostID,
		ArtifactType: task.ArtifactType, ArtifactPath: task.ArtifactPath, Version: version, Params: params,
	})
	return map[string]any{"releaseId": release.ID, "status": release.Status, "summary": release.Summary}, nil
}

func (s *Service) ListOpsAppReleases(pageNum, pageSize int, appID uint, keyword, status, env, startTime, endTime string) (map[string]any, error) {
	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	query := s.db.Model(&model.OpsAppRelease{})
	if appID > 0 {
		query = query.Where("app_id = ?", appID)
	}
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("app_name LIKE ? OR app_code LIKE ? OR build_task_name LIKE ? OR version LIKE ? OR summary LIKE ?", like, like, like, like, like)
	}
	if status = strings.TrimSpace(status); status != "" {
		query = query.Where("status = ?", status)
	}
	if env = strings.TrimSpace(env); env != "" {
		query = query.Where("env = ?", env)
	}
	if value, ok := parseOpsReleaseQueryTime(startTime); ok {
		query = query.Where("created_at >= ?", value)
	}
	if value, ok := parseOpsReleaseQueryTime(endTime); ok {
		query = query.Where("created_at <= ?", value)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var list []model.OpsAppRelease
	if err := query.Order("id DESC").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, err
	}
	return map[string]any{"list": list, "total": total, "pageNum": pageNum, "pageSize": pageSize}, nil
}

func parseOpsReleaseQueryTime(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"} {
		if parsed, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func (s *Service) RetryOpsAppRelease(id uint) (map[string]any, error) {
	if id == 0 {
		return nil, errors.New("build record ID is required")
	}
	release, err := s.GetOpsAppRelease(id)
	if err != nil {
		return nil, err
	}
	if release.BuildTaskID == 0 {
		return nil, errors.New("the build history is not linked to a build task and cannot be retried with the original configuration")
	}
	params := map[string]any{}
	if strings.TrimSpace(release.ParamsJSON) != "" {
		if err := json.Unmarshal([]byte(release.ParamsJSON), &params); err != nil {
			return nil, errors.New("original build parameters cannot be read; retry is unavailable")
		}
	}
	return s.RunOpsAppBuildTask(OpsAppBuildRunPayload{TaskID: release.BuildTaskID, Branch: release.Branch, Params: params})
}

func (s *Service) GetOpsAppRelease(id uint) (*model.OpsAppRelease, error) {
	var item model.OpsAppRelease
	if err := s.db.First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Service) ListOpsAppArtifacts(appID uint, env, status string) ([]model.OpsAppArtifact, error) {
	query := s.db.Model(&model.OpsAppArtifact{})
	if appID > 0 {
		query = query.Where("app_id = ?", appID)
	}
	if env = normalizeEnvCode(env); env != "" {
		query = query.Where("env = ?", env)
	}
	if status = strings.TrimSpace(status); status != "" {
		query = query.Where("status = ?", status)
	}
	var list []model.OpsAppArtifact
	err := query.Order("id DESC").Limit(500).Find(&list).Error
	return list, err
}

func normalizeOpsImageRegistryAddress(value string) string {
	return strings.TrimSuffix(strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(value, "https://"), "http://")), "/")
}

func mapOpsImageRegistry(item model.OpsImageRegistry) map[string]any {
	return map[string]any{"id": item.ID, "name": item.Name, "address": item.Address, "namespace": item.Namespace, "username": item.Username, "hasPassword": strings.TrimSpace(item.Password) != "", "status": item.Status, "description": item.Description, "createTime": item.CreatedAt, "updateTime": item.UpdatedAt}
}

func (s *Service) ListOpsImageRegistries(enabledOnly bool) ([]map[string]any, error) {
	query := s.db.Model(&model.OpsImageRegistry{})
	if enabledOnly {
		query = query.Where("status = ?", 1)
	}
	var list []model.OpsImageRegistry
	if err := query.Order("id DESC").Find(&list).Error; err != nil {
		return nil, err
	}
	result := make([]map[string]any, 0, len(list))
	for _, item := range list {
		result = append(result, mapOpsImageRegistry(item))
	}
	return result, nil
}

func (s *Service) SaveOpsImageRegistry(payload OpsImageRegistryPayload) error {
	name, address := Trimmed(payload.Name), normalizeOpsImageRegistryAddress(payload.Address)
	if name == "" || address == "" {
		return errors.New("image registry name and address are required")
	}
	status := 1
	if payload.Status == 2 {
		status = 2
	}
	updates := map[string]any{"name": name, "address": address, "namespace": strings.Trim(strings.TrimSpace(payload.Namespace), "/"), "username": Trimmed(payload.Username), "status": status, "description": Trimmed(payload.Description)}
	if strings.TrimSpace(payload.Password) != "" {
		updates["password"] = payload.Password
	}
	if payload.ID > 0 {
		return s.db.Model(&model.OpsImageRegistry{}).Where("id = ?", payload.ID).Updates(updates).Error
	}
	if _, ok := updates["password"]; !ok {
		updates["password"] = ""
	}
	return s.db.Create(&model.OpsImageRegistry{Name: name, Address: address, Namespace: updates["namespace"].(string), Username: updates["username"].(string), Password: updates["password"].(string), Status: updates["status"].(int), Description: updates["description"].(string)}).Error
}

func (s *Service) DeleteOpsImageRegistry(id uint) error {
	if id == 0 {
		return errors.New("image registry ID is required")
	}
	return s.db.Delete(&model.OpsImageRegistry{}, id).Error
}
