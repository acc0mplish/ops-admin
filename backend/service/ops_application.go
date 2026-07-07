package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"ops-admin/backend/model"
)

type OpsApplicationPayload struct {
	ID           uint   `json:"id"`
	Name         string `json:"name"`
	Code         string `json:"code"`
	ServiceType  string `json:"serviceType"`
	RepoType     string `json:"repoType"`
	RepoURL      string `json:"repoUrl"`
	Branch       string `json:"branch"`
	Workspace    string `json:"workspace"`
	BuildScript  string `json:"buildScript"`
	DeployScript string `json:"deployScript"`
	Env          string `json:"env"`
	Status       int    `json:"status"`
	Description  string `json:"description"`
}

type OpsAppBuildTaskPayload struct {
	ID             uint   `json:"id"`
	AppID          uint   `json:"appId"`
	Name           string `json:"name"`
	Env            string `json:"env"`
	Branch         string `json:"branch"`
	BuildScript    string `json:"buildScript"`
	DeployScript   string `json:"deployScript"`
	TimeoutSeconds int    `json:"timeoutSeconds"`
	Status         int    `json:"status"`
	Description    string `json:"description"`
}

type OpsAppBuildTaskStatusPayload struct {
	ID     uint `json:"id"`
	Status int  `json:"status"`
}

type OpsAppBuildRunPayload struct {
	TaskID  uint   `json:"taskId"`
	Version string `json:"version"`
	Branch  string `json:"branch"`
}

type OpsAppReleasePayload struct {
	AppID   uint   `json:"appId"`
	Version string `json:"version"`
	Branch  string `json:"branch"`
}

type OpsAppPipelinePayload struct {
	ID             uint   `json:"id"`
	Name           string `json:"name"`
	AppID          uint   `json:"appId"`
	DefaultBranch  string `json:"defaultBranch"`
	Env            string `json:"env"`
	TechStack      string `json:"techStack"`
	TemplateID     uint   `json:"templateId"`
	Status         int    `json:"status"`
	Description    string `json:"description"`
	DefinitionJSON string `json:"definitionJson"`
}

type OpsAppPipelineStatusPayload struct {
	ID     uint `json:"id"`
	Status int  `json:"status"`
}

type OpsAppPipelineRunPayload struct {
	PipelineID uint              `json:"pipelineId"`
	Branch     string            `json:"branch"`
	Env        string            `json:"env"`
	ImageTag   string            `json:"imageTag"`
	Params     map[string]string `json:"params"`
}

type OpsAppPipelineStageDefinition struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	Type           string            `json:"type"`
	TimeoutSeconds int               `json:"timeoutSeconds"`
	FailurePolicy  string            `json:"failurePolicy"`
	Config         map[string]any    `json:"config"`
	Env            map[string]string `json:"env"`
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
}

type opsPipelineExecution struct {
	RunID      uint
	PipelineID uint
	App        model.OpsApplication
	Branch     string
	Env        string
	ImageTag   string
	Workspace  string
	Params     map[string]string
	Stages     []OpsAppPipelineStageDefinition
	StartedAt  time.Time
}

func normalizeOpsRepoType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "svn":
		return "svn"
	default:
		return "git"
	}
}

func normalizeOpsAppStatus(value int) int {
	if value == 2 {
		return 2
	}
	return 1
}

func normalizeOpsBuildTimeout(value int) int {
	if value < 60 {
		return 60
	}
	if value > 7200 {
		return 7200
	}
	return value
}

func (s *Service) ListOpsApplications(pageNum, pageSize int, keyword, repoType, status, serviceType string) (map[string]any, error) {
	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	query := s.db.Model(&model.OpsApplication{})
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("name LIKE ? OR code LIKE ? OR repo_url LIKE ? OR description LIKE ?", like, like, like, like)
	}
	if repoType = strings.ToLower(strings.TrimSpace(repoType)); repoType != "" {
		query = query.Where("repo_type = ?", normalizeOpsRepoType(repoType))
	}
	if status = strings.TrimSpace(status); status != "" {
		query = query.Where("status = ?", status)
	}
	if serviceType = strings.TrimSpace(serviceType); serviceType != "" {
		query = query.Where("service_type = ?", serviceType)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var list []model.OpsApplication
	if err := query.Order("id DESC").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, err
	}
	return map[string]any{"list": list, "total": total, "pageNum": pageNum, "pageSize": pageSize}, nil
}

func (s *Service) ListOpsApplicationOptions() ([]map[string]any, error) {
	var list []model.OpsApplication
	if err := s.db.Where("status = ?", 1).Order("name ASC").Find(&list).Error; err != nil {
		return nil, err
	}
	options := make([]map[string]any, 0, len(list))
	for _, item := range list {
		options = append(options, map[string]any{
			"id": item.ID, "name": item.Name, "code": item.Code, "repoType": item.RepoType,
			"repoUrl": item.RepoURL, "branch": item.Branch, "env": item.Env, "serviceType": item.ServiceType,
		})
	}
	return options, nil
}

func (s *Service) GetOpsApplication(id uint) (*model.OpsApplication, error) {
	var item model.OpsApplication
	if err := s.db.First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Service) SaveOpsApplication(payload OpsApplicationPayload) error {
	item := model.OpsApplication{
		Name:         Trimmed(payload.Name),
		Code:         Trimmed(payload.Code),
		ServiceType:  Trimmed(payload.ServiceType),
		RepoType:     normalizeOpsRepoType(payload.RepoType),
		RepoURL:      Trimmed(payload.RepoURL),
		Branch:       Trimmed(payload.Branch),
		Workspace:    Trimmed(payload.Workspace),
		BuildScript:  strings.TrimSpace(payload.BuildScript),
		DeployScript: strings.TrimSpace(payload.DeployScript),
		Env:          Trimmed(payload.Env),
		Status:       normalizeOpsAppStatus(payload.Status),
		Description:  Trimmed(payload.Description),
	}
	if item.Name == "" {
		return errors.New("应用名称不能为空")
	}
	if item.Code == "" {
		return errors.New("应用编码不能为空")
	}
	if item.RepoURL == "" {
		return errors.New("仓库地址不能为空")
	}
	if item.ServiceType == "" {
		item.ServiceType = "后端服务"
	}
	if item.Branch == "" && item.RepoType == "git" {
		item.Branch = "master"
	}
	if payload.ID == 0 {
		return s.db.Create(&item).Error
	}
	return s.db.Model(&model.OpsApplication{}).Where("id = ?", payload.ID).Updates(map[string]any{
		"name": item.Name, "code": item.Code, "service_type": item.ServiceType, "repo_type": item.RepoType,
		"repo_url": item.RepoURL, "branch": item.Branch, "workspace": item.Workspace, "build_script": item.BuildScript,
		"deploy_script": item.DeployScript, "env": item.Env, "status": item.Status, "description": item.Description,
	}).Error
}

func (s *Service) DeleteOpsApplication(id uint) error {
	var count int64
	if err := s.db.Model(&model.OpsAppBuildTask{}).Where("app_id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return errors.New("该应用下存在构建任务，请先删除构建任务")
	}
	if err := s.db.Model(&model.OpsAppPipeline{}).Where("app_id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return errors.New("该应用下存在 CI/CD 流水线，请先删除流水线")
	}
	return s.db.Delete(&model.OpsApplication{}, id).Error
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
	return map[string]any{"list": list, "total": total, "pageNum": pageNum, "pageSize": pageSize}, nil
}

func (s *Service) GetOpsAppBuildTask(id uint) (*model.OpsAppBuildTask, error) {
	var item model.OpsAppBuildTask
	if err := s.db.First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Service) SaveOpsAppBuildTask(payload OpsAppBuildTaskPayload) error {
	if payload.AppID == 0 {
		return errors.New("请选择所属应用")
	}
	app, err := s.GetOpsApplication(payload.AppID)
	if err != nil {
		return err
	}
	task := model.OpsAppBuildTask{
		Name:           Trimmed(payload.Name),
		AppID:          app.ID,
		AppName:        app.Name,
		AppCode:        app.Code,
		Env:            Trimmed(payload.Env),
		Branch:         Trimmed(payload.Branch),
		BuildScript:    strings.TrimSpace(payload.BuildScript),
		DeployScript:   strings.TrimSpace(payload.DeployScript),
		TimeoutSeconds: normalizeOpsBuildTimeout(payload.TimeoutSeconds),
		Status:         normalizeOpsAppStatus(payload.Status),
		Description:    Trimmed(payload.Description),
	}
	if task.Name == "" {
		return errors.New("构建任务名称不能为空")
	}
	if task.BuildScript == "" {
		return errors.New("构建脚本不能为空")
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
	if payload.ID == 0 {
		return s.db.Create(&task).Error
	}
	return s.db.Model(&model.OpsAppBuildTask{}).Where("id = ?", payload.ID).Updates(map[string]any{
		"name": task.Name, "app_id": task.AppID, "app_name": task.AppName, "app_code": task.AppCode,
		"env": task.Env, "branch": task.Branch, "build_script": task.BuildScript, "deploy_script": task.DeployScript,
		"timeout_seconds": task.TimeoutSeconds, "status": task.Status, "description": task.Description,
	}).Error
}

func (s *Service) UpdateOpsAppBuildTaskStatus(payload OpsAppBuildTaskStatusPayload) error {
	if payload.ID == 0 {
		return errors.New("构建任务ID不能为空")
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
		return nil, errors.New("当前构建任务已禁用")
	}
	app, err := s.GetOpsApplication(task.AppID)
	if err != nil {
		return nil, err
	}
	if app.Status != 1 {
		return nil, errors.New("当前应用已禁用，无法构建")
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
	now := time.Now()
	workspace := s.resolveOpsAppWorkspace(*app)
	release := model.OpsAppRelease{
		AppID: app.ID, AppName: app.Name, AppCode: app.Code, BuildTaskID: task.ID, BuildTaskName: task.Name,
		Env: task.Env, Version: version, RepoType: app.RepoType, RepoURL: app.RepoURL, Branch: branch,
		Workspace: workspace, Status: "running", Stage: "checkout", Summary: "构建任务已创建，正在拉取代码", StartedAt: &now,
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
		Branch: branch, Workspace: workspace,
	})
	return map[string]any{"releaseId": release.ID, "status": release.Status, "summary": release.Summary}, nil
}

func (s *Service) ListOpsAppReleases(pageNum, pageSize int, appID uint, keyword, status, env string) (map[string]any, error) {
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

func (s *Service) GetOpsAppRelease(id uint) (*model.OpsAppRelease, error) {
	var item model.OpsAppRelease
	if err := s.db.First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func builtinOpsAppPipelineTemplates() []map[string]any {
	templates := []struct {
		ID          uint
		Name        string
		Category    string
		TechStack   string
		Description string
		Stages      []OpsAppPipelineStageDefinition
	}{
		{
			ID: 1, Name: "Go 后端通用模板", Category: "Go", TechStack: "go",
			Description: "Go 编译、镜像推送、工作负载更新",
			Stages: []OpsAppPipelineStageDefinition{
				{ID: "checkout", Name: "代码拉取", Type: "checkout", TimeoutSeconds: 600, FailurePolicy: "stop"},
				{ID: "deps", Name: "Go 依赖安装", Type: "command", TimeoutSeconds: 600, FailurePolicy: "stop", Config: map[string]any{"script": "go mod download"}},
				{ID: "test", Name: "单元测试", Type: "test", TimeoutSeconds: 900, FailurePolicy: "stop", Config: map[string]any{"script": "go test ./..."}},
				{ID: "build", Name: "Go 编译", Type: "build", TimeoutSeconds: 900, FailurePolicy: "stop", Config: map[string]any{"script": "go build ./..."}},
				{ID: "docker-build", Name: "Docker 镜像构建", Type: "dockerBuild", TimeoutSeconds: 1200, FailurePolicy: "stop"},
				{ID: "docker-push", Name: "镜像推送", Type: "dockerPush", TimeoutSeconds: 1200, FailurePolicy: "stop"},
				{ID: "k8s-deploy", Name: "K8s 工作负载更新", Type: "k8sDeploy", TimeoutSeconds: 900, FailurePolicy: "stop"},
			},
		},
		{
			ID: 2, Name: "Maven Java 通用模板", Category: "Java", TechStack: "maven",
			Description: "Maven 打包、Jar 镜像、K8s 发布",
			Stages: []OpsAppPipelineStageDefinition{
				{ID: "checkout", Name: "代码拉取", Type: "checkout", TimeoutSeconds: 600, FailurePolicy: "stop"},
				{ID: "deps", Name: "Maven 依赖安装", Type: "command", TimeoutSeconds: 900, FailurePolicy: "stop", Config: map[string]any{"script": "mvn dependency:go-offline"}},
				{ID: "test", Name: "单元测试", Type: "test", TimeoutSeconds: 1200, FailurePolicy: "stop", Config: map[string]any{"script": "mvn test"}},
				{ID: "package", Name: "Maven 打包", Type: "build", TimeoutSeconds: 1200, FailurePolicy: "stop", Config: map[string]any{"script": "mvn clean package -DskipTests"}},
				{ID: "docker-build", Name: "Docker 镜像构建", Type: "dockerBuild", TimeoutSeconds: 1200, FailurePolicy: "stop"},
				{ID: "docker-push", Name: "镜像推送", Type: "dockerPush", TimeoutSeconds: 1200, FailurePolicy: "stop"},
				{ID: "k8s-deploy", Name: "K8s 发布", Type: "k8sDeploy", TimeoutSeconds: 900, FailurePolicy: "stop"},
			},
		},
		{
			ID: 3, Name: "Vue 前端通用模板", Category: "Node.js", TechStack: "vue",
			Description: "npm 构建、镜像打包、K8s 滚动发布",
			Stages: []OpsAppPipelineStageDefinition{
				{ID: "checkout", Name: "代码拉取", Type: "checkout", TimeoutSeconds: 600, FailurePolicy: "stop"},
				{ID: "install", Name: "npm install", Type: "command", TimeoutSeconds: 900, FailurePolicy: "stop", Config: map[string]any{"script": "npm install"}},
				{ID: "build", Name: "npm run build", Type: "build", TimeoutSeconds: 900, FailurePolicy: "stop", Config: map[string]any{"script": "npm run build"}},
				{ID: "docker-build", Name: "Docker 镜像构建", Type: "dockerBuild", TimeoutSeconds: 1200, FailurePolicy: "stop"},
				{ID: "docker-push", Name: "镜像推送", Type: "dockerPush", TimeoutSeconds: 1200, FailurePolicy: "stop"},
				{ID: "k8s-deploy", Name: "K8s 滚动发布", Type: "k8sDeploy", TimeoutSeconds: 900, FailurePolicy: "stop"},
				{ID: "notify", Name: "发布通知", Type: "notify", TimeoutSeconds: 60, FailurePolicy: "ignore"},
			},
		},
	}
	result := make([]map[string]any, 0, len(templates))
	for _, item := range templates {
		definition, _ := json.Marshal(map[string]any{"stages": item.Stages})
		result = append(result, map[string]any{
			"id": item.ID, "name": item.Name, "category": item.Category, "techStack": item.TechStack,
			"description": item.Description, "stageCount": len(item.Stages), "definitionJson": string(definition), "builtin": true, "status": 1,
		})
	}
	return result
}

func (s *Service) ListOpsAppPipelineTemplates(category string) ([]map[string]any, error) {
	category = strings.TrimSpace(category)
	all := builtinOpsAppPipelineTemplates()
	if category == "" || category == "全部模板" {
		return all, nil
	}
	filtered := make([]map[string]any, 0)
	for _, item := range all {
		if strings.EqualFold(fmt.Sprint(item["category"]), category) || strings.EqualFold(fmt.Sprint(item["techStack"]), category) {
			filtered = append(filtered, item)
		}
	}
	return filtered, nil
}

func normalizeOpsPipelineStages(definitionJSON string) ([]OpsAppPipelineStageDefinition, string, error) {
	definitionJSON = strings.TrimSpace(definitionJSON)
	if definitionJSON == "" {
		return []OpsAppPipelineStageDefinition{}, `{"stages":[]}`, nil
	}
	var wrapper struct {
		Stages []OpsAppPipelineStageDefinition `json:"stages"`
	}
	if err := json.Unmarshal([]byte(definitionJSON), &wrapper); err != nil {
		return nil, "", errors.New("流水线阶段配置不是有效 JSON")
	}
	for index := range wrapper.Stages {
		if wrapper.Stages[index].ID == "" {
			wrapper.Stages[index].ID = fmt.Sprintf("stage-%d", index+1)
		}
		if wrapper.Stages[index].Name == "" {
			wrapper.Stages[index].Name = fmt.Sprintf("阶段 %d", index+1)
		}
		if wrapper.Stages[index].Type == "" {
			wrapper.Stages[index].Type = "command"
		}
		if wrapper.Stages[index].TimeoutSeconds <= 0 {
			wrapper.Stages[index].TimeoutSeconds = 1800
		}
		if wrapper.Stages[index].FailurePolicy == "" {
			wrapper.Stages[index].FailurePolicy = "stop"
		}
	}
	normalized, _ := json.Marshal(map[string]any{"stages": wrapper.Stages})
	return wrapper.Stages, string(normalized), nil
}

func (s *Service) ListOpsAppPipelines(pageNum, pageSize int, appID uint, keyword, env, status, techStack string) (map[string]any, error) {
	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	query := s.db.Model(&model.OpsAppPipeline{})
	if appID > 0 {
		query = query.Where("app_id = ?", appID)
	}
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("name LIKE ? OR app_name LIKE ? OR app_code LIKE ? OR repo_url LIKE ? OR description LIKE ?", like, like, like, like, like)
	}
	if env = strings.TrimSpace(env); env != "" {
		query = query.Where("env = ?", env)
	}
	if status = strings.TrimSpace(status); status != "" {
		query = query.Where("status = ?", status)
	}
	if techStack = strings.TrimSpace(techStack); techStack != "" {
		query = query.Where("tech_stack = ?", techStack)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var list []model.OpsAppPipeline
	if err := query.Order("id DESC").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, err
	}
	var enabled, failed int64
	_ = s.db.Model(&model.OpsAppPipeline{}).Where("status = ?", 1).Count(&enabled).Error
	_ = s.db.Model(&model.OpsAppPipeline{}).Where("last_status = ?", "failed").Count(&failed).Error
	return map[string]any{
		"list": list, "total": total, "pageNum": pageNum, "pageSize": pageSize,
		"stats": map[string]any{"total": total, "enabled": enabled, "failed": failed},
	}, nil
}

func (s *Service) GetOpsAppPipeline(id uint) (map[string]any, error) {
	var item model.OpsAppPipeline
	if err := s.db.First(&item, id).Error; err != nil {
		return nil, err
	}
	stages, _, err := normalizeOpsPipelineStages(item.DefinitionJSON)
	if err != nil {
		stages = []OpsAppPipelineStageDefinition{}
	}
	return map[string]any{"pipeline": item, "stages": stages}, nil
}

func (s *Service) SaveOpsAppPipeline(payload OpsAppPipelinePayload) error {
	if payload.AppID == 0 {
		return errors.New("请选择所属应用")
	}
	app, err := s.GetOpsApplication(payload.AppID)
	if err != nil {
		return err
	}
	stages, normalizedDefinition, err := normalizeOpsPipelineStages(payload.DefinitionJSON)
	if err != nil {
		return err
	}
	item := model.OpsAppPipeline{
		Name:           Trimmed(payload.Name),
		AppID:          app.ID,
		AppName:        app.Name,
		AppCode:        app.Code,
		RepoType:       app.RepoType,
		RepoURL:        app.RepoURL,
		DefaultBranch:  Trimmed(payload.DefaultBranch),
		Env:            Trimmed(payload.Env),
		TechStack:      Trimmed(payload.TechStack),
		TemplateID:     payload.TemplateID,
		StageCount:     len(stages),
		Status:         normalizeOpsAppStatus(payload.Status),
		Description:    Trimmed(payload.Description),
		DefinitionJSON: normalizedDefinition,
	}
	if item.Name == "" {
		return errors.New("流水线名称不能为空")
	}
	if item.DefaultBranch == "" {
		item.DefaultBranch = app.Branch
	}
	if item.DefaultBranch == "" && app.RepoType == "git" {
		item.DefaultBranch = "master"
	}
	if item.Env == "" {
		item.Env = app.Env
	}
	if item.Env == "" {
		item.Env = "test"
	}
	if item.TechStack == "" {
		item.TechStack = "custom"
	}
	if payload.ID == 0 {
		return s.db.Create(&item).Error
	}
	return s.db.Model(&model.OpsAppPipeline{}).Where("id = ?", payload.ID).Updates(map[string]any{
		"name": item.Name, "app_id": item.AppID, "app_name": item.AppName, "app_code": item.AppCode,
		"repo_type": item.RepoType, "repo_url": item.RepoURL, "default_branch": item.DefaultBranch,
		"env": item.Env, "tech_stack": item.TechStack, "template_id": item.TemplateID,
		"stage_count": item.StageCount, "status": item.Status, "description": item.Description,
		"definition_json": item.DefinitionJSON,
	}).Error
}

func (s *Service) UpdateOpsAppPipelineStatus(payload OpsAppPipelineStatusPayload) error {
	if payload.ID == 0 {
		return errors.New("流水线 ID 不能为空")
	}
	return s.db.Model(&model.OpsAppPipeline{}).Where("id = ?", payload.ID).Update("status", normalizeOpsAppStatus(payload.Status)).Error
}

func (s *Service) DeleteOpsAppPipeline(id uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var runIDs []uint
		if err := tx.Model(&model.OpsAppPipelineRun{}).Where("pipeline_id = ?", id).Pluck("id", &runIDs).Error; err != nil {
			return err
		}
		if len(runIDs) > 0 {
			if err := tx.Where("run_id IN ?", runIDs).Delete(&model.OpsAppPipelineRunStage{}).Error; err != nil {
				return err
			}
			if err := tx.Where("pipeline_id = ?", id).Delete(&model.OpsAppPipelineRun{}).Error; err != nil {
				return err
			}
		}
		return tx.Delete(&model.OpsAppPipeline{}, id).Error
	})
}

func (s *Service) CopyOpsAppPipeline(id uint) (map[string]any, error) {
	var item model.OpsAppPipeline
	if err := s.db.First(&item, id).Error; err != nil {
		return nil, err
	}
	item.ID = 0
	item.Name = item.Name + "-copy"
	item.LastRunID = 0
	item.LastStatus = ""
	item.LastRunAt = nil
	if err := s.db.Create(&item).Error; err != nil {
		return nil, err
	}
	return map[string]any{"id": item.ID}, nil
}

func (s *Service) RunOpsAppPipeline(payload OpsAppPipelineRunPayload) (map[string]any, error) {
	var pipeline model.OpsAppPipeline
	if err := s.db.First(&pipeline, payload.PipelineID).Error; err != nil {
		return nil, err
	}
	if pipeline.Status != 1 {
		return nil, errors.New("当前流水线已禁用，无法执行")
	}
	app, err := s.GetOpsApplication(pipeline.AppID)
	if err != nil {
		return nil, err
	}
	stages, normalizedDefinition, err := normalizeOpsPipelineStages(pipeline.DefinitionJSON)
	if err != nil {
		return nil, err
	}
	branch := Trimmed(payload.Branch)
	if branch == "" {
		branch = pipeline.DefaultBranch
	}
	env := Trimmed(payload.Env)
	if env == "" {
		env = pipeline.Env
	}
	imageTag := Trimmed(payload.ImageTag)
	if imageTag == "" {
		imageTag = time.Now().Format("20060102150405")
	}
	workspace := s.resolveOpsAppWorkspace(*app)
	paramsJSON, _ := json.Marshal(payload.Params)
	now := time.Now()
	run := model.OpsAppPipelineRun{
		PipelineID: pipeline.ID, PipelineName: pipeline.Name, AppID: pipeline.AppID, AppName: pipeline.AppName,
		AppCode: pipeline.AppCode, Env: env, Branch: branch, ImageTag: imageTag, TriggerType: "manual",
		TriggerUser: "系统管理员", Status: "running", Summary: "流水线已创建，等待阶段执行",
		ParamsJSON: string(paramsJSON), DefinitionJSON: normalizedDefinition, StartedAt: &now,
	}
	if err := s.db.Create(&run).Error; err != nil {
		return nil, err
	}
	for _, stage := range stages {
		_ = s.db.Create(&model.OpsAppPipelineRunStage{
			RunID: run.ID, StageID: stage.ID, StageName: stage.Name, StageType: stage.Type,
			Status: "waiting", Summary: "等待执行",
		}).Error
	}
	_ = s.db.Model(&model.OpsAppPipeline{}).Where("id = ?", pipeline.ID).Updates(map[string]any{
		"last_run_id": run.ID, "last_status": "running", "last_run_at": &now,
	})
	go s.runOpsAppPipeline(opsPipelineExecution{
		RunID: run.ID, PipelineID: pipeline.ID, App: *app, Branch: branch, Env: env,
		ImageTag: imageTag, Workspace: workspace, Params: payload.Params, Stages: stages, StartedAt: now,
	})
	return map[string]any{"runId": run.ID, "status": run.Status, "summary": run.Summary}, nil
}

func (s *Service) ListOpsAppPipelineRuns(pageNum, pageSize int, pipelineID, appID uint, keyword, status, env string) (map[string]any, error) {
	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	query := s.db.Model(&model.OpsAppPipelineRun{})
	if pipelineID > 0 {
		query = query.Where("pipeline_id = ?", pipelineID)
	}
	if appID > 0 {
		query = query.Where("app_id = ?", appID)
	}
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("pipeline_name LIKE ? OR app_name LIKE ? OR app_code LIKE ? OR image_tag LIKE ? OR summary LIKE ?", like, like, like, like, like)
	}
	if status = strings.TrimSpace(status); status != "" {
		query = query.Where("status = ?", status)
	}
	if env = strings.TrimSpace(env); env != "" {
		query = query.Where("env = ?", env)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var list []model.OpsAppPipelineRun
	if err := query.Order("id DESC").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, err
	}
	return map[string]any{"list": list, "total": total, "pageNum": pageNum, "pageSize": pageSize}, nil
}

func (s *Service) GetOpsAppPipelineRun(id uint) (map[string]any, error) {
	var run model.OpsAppPipelineRun
	if err := s.db.First(&run, id).Error; err != nil {
		return nil, err
	}
	var stages []model.OpsAppPipelineRunStage
	if err := s.db.Where("run_id = ?", id).Order("id ASC").Find(&stages).Error; err != nil {
		return nil, err
	}
	return map[string]any{"run": run, "stages": stages}, nil
}

func (s *Service) runOpsAppPipeline(execInfo opsPipelineExecution) {
	status, summary := "success", "流水线执行完成"
	for _, stage := range execInfo.Stages {
		started := time.Now()
		_ = s.db.Model(&model.OpsAppPipelineRunStage{}).
			Where("run_id = ? AND stage_id = ?", execInfo.RunID, stage.ID).
			Updates(map[string]any{"status": "running", "summary": "正在执行", "started_at": &started}).Error
		_ = s.db.Model(&model.OpsAppPipelineRun{}).Where("id = ?", execInfo.RunID).Update("summary", "正在执行："+stage.Name).Error

		stageStatus, stageSummary := "success", "执行成功"
		logText, err := s.executeOpsAppPipelineStage(execInfo, stage)
		if err != nil {
			stageStatus, stageSummary = "failed", err.Error()
			if strings.EqualFold(stage.FailurePolicy, "ignore") {
				stageStatus, stageSummary = "success", "执行失败，已按策略忽略："+err.Error()
				logText += "\n[WARN] " + stageSummary + "\n"
			} else {
				status, summary = "failed", stage.Name+" 执行失败："+err.Error()
			}
		}
		finished := time.Now()
		if strings.TrimSpace(logText) == "" {
			logText = fmt.Sprintf("[%s] 阶段执行完成：%s\n", finished.Format("2006-01-02 15:04:05"), stage.Name)
		}
		_ = s.db.Model(&model.OpsAppPipelineRunStage{}).
			Where("run_id = ? AND stage_id = ?", execInfo.RunID, stage.ID).
			Updates(map[string]any{
				"status": stageStatus, "summary": stageSummary, "log": logText, "finished_at": &finished,
				"duration_ms": finished.Sub(started).Milliseconds(),
			}).Error
		if status == "failed" {
			break
		}
	}
	finished := time.Now()
	_ = s.db.Model(&model.OpsAppPipelineRun{}).Where("id = ?", execInfo.RunID).Updates(map[string]any{
		"status": status, "summary": summary, "finished_at": &finished, "duration_ms": finished.Sub(execInfo.StartedAt).Milliseconds(),
	}).Error
	_ = s.db.Model(&model.OpsAppPipeline{}).Where("id = ?", execInfo.PipelineID).Update("last_status", status).Error
}

func (s *Service) executeOpsAppPipelineStage(execInfo opsPipelineExecution, stage OpsAppPipelineStageDefinition) (string, error) {
	header := fmt.Sprintf("[%s] 阶段开始：%s\n阶段类型：%s\n执行策略：%s\n工作目录：%s\n\n",
		time.Now().Format("2006-01-02 15:04:05"), stage.Name, stage.Type, stage.FailurePolicy, execInfo.Workspace)
	timeout := time.Duration(normalizeOpsBuildTimeout(stage.TimeoutSeconds)) * time.Second
	switch stage.Type {
	case "checkout":
		if err := os.MkdirAll(filepath.Dir(execInfo.Workspace), 0o755); err != nil {
			return header, err
		}
		output, err := s.checkoutOpsAppCode(execInfo.App, execInfo.Workspace, execInfo.Branch)
		return header + sectionLog("代码拉取", output), err
	case "command", "test", "build":
		script := opsPipelineConfigString(stage.Config, "script")
		if script == "" {
			return header, errors.New("阶段脚本不能为空")
		}
		if err := ensureOpsAppPipelineWorkspace(execInfo.Workspace); err != nil {
			return header, err
		}
		output, err := runOpsAppShell(script, execInfo.Workspace, timeout)
		return header + sectionLog("执行脚本", script) + sectionLog(stage.Name+" 输出", appendOpsAppCommandError(output, err)), err
	case "dockerBuild":
		if err := ensureOpsAppPipelineWorkspace(execInfo.Workspace); err != nil {
			return header, err
		}
		image := opsPipelineImageName(stage.Config, execInfo)
		dockerfile := opsPipelineConfigStringDefault(stage.Config, "dockerfile", "Dockerfile")
		contextDir := opsPipelineConfigStringDefault(stage.Config, "context", ".")
		args := []string{"build", "-t", image, "-f", dockerfile, contextDir}
		output, err := runOpsAppCommand(execInfo.Workspace, timeout, "docker", args...)
		return header + sectionLog("Docker Build: "+image, appendOpsAppCommandError(output, err)), err
	case "dockerPush":
		if err := ensureOpsAppPipelineWorkspace(execInfo.Workspace); err != nil {
			return header, err
		}
		image := opsPipelineImageName(stage.Config, execInfo)
		output, err := runOpsAppCommand(execInfo.Workspace, timeout, "docker", "push", image)
		return header + sectionLog("Docker Push: "+image, appendOpsAppCommandError(output, err)), err
	case "k8sDeploy":
		clusterID := opsPipelineConfigUint(stage.Config, "clusterId")
		if clusterID == 0 {
			return header, errors.New("K8s 发布需要选择目标集群")
		}
		kubeconfigPath, cleanup, err := s.opsPipelineKubeconfigFile(clusterID)
		if err != nil {
			return header, err
		}
		defer cleanup()
		namespace := opsPipelineConfigString(stage.Config, "namespace")
		workloadType := strings.ToLower(opsPipelineConfigStringDefault(stage.Config, "workloadType", "deployment"))
		workload := opsPipelineConfigString(stage.Config, "workload")
		container := opsPipelineConfigString(stage.Config, "container")
		switch workloadType {
		case "deployment", "statefulset", "daemonset":
		default:
			return header, fmt.Errorf("不支持的 K8s 工作负载类型：%s", workloadType)
		}
		if namespace == "" {
			namespace = execInfo.Env
		}
		if namespace == "" {
			namespace = "default"
		}
		if workload == "" {
			workload = execInfo.App.Code
		}
		if container == "" {
			container = execInfo.App.Code
		}
		image := opsPipelineImageName(stage.Config, execInfo)
		target := workloadType + "/" + workload
		output, err := runOpsAppCommand(".", timeout, "kubectl", "--kubeconfig", kubeconfigPath, "-n", namespace, "set", "image", target, container+"="+image)
		if err != nil {
			return header + sectionLog("kubectl set image", appendOpsAppCommandError(output, err)), err
		}
		rollout, rolloutErr := runOpsAppCommand(".", timeout, "kubectl", "--kubeconfig", kubeconfigPath, "-n", namespace, "rollout", "status", target, "--timeout="+fmt.Sprintf("%ds", stage.TimeoutSeconds))
		return header + sectionLog("kubectl set image", output) + sectionLog("kubectl rollout status", appendOpsAppCommandError(rollout, rolloutErr)), rolloutErr
	case "manual":
		return header + "当前版本的流水线人工确认阶段以记录方式通过，后续可接入审批中心。\n", nil
	case "notify":
		return header + "当前版本的流水线消息通知阶段以记录方式通过，后续可接入消息通知规则。\n", nil
	default:
		return header, fmt.Errorf("不支持的阶段类型：%s", stage.Type)
	}
}

func (s *Service) opsPipelineKubeconfigFile(clusterID uint) (string, func(), error) {
	cluster, err := s.GetK8sCluster(clusterID)
	if err != nil {
		return "", func() {}, err
	}
	if strings.TrimSpace(cluster.KubeConfig) == "" {
		return "", func() {}, errors.New("目标集群 kubeconfig 为空")
	}
	file, err := os.CreateTemp("", fmt.Sprintf("ops-admin-kubeconfig-%d-*.yaml", clusterID))
	if err != nil {
		return "", func() {}, err
	}
	path := file.Name()
	if _, err := file.WriteString(cluster.KubeConfig); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return "", func() {}, err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", func() {}, err
	}
	return path, func() { _ = os.Remove(path) }, nil
}

func ensureOpsAppPipelineWorkspace(workspace string) error {
	workspace = strings.TrimSpace(workspace)
	if workspace == "" || workspace == "." {
		return nil
	}
	return os.MkdirAll(workspace, 0o755)
}

func appendOpsAppCommandError(output string, err error) string {
	if err == nil {
		return output
	}
	if strings.TrimSpace(output) == "" {
		return "[ERROR] " + err.Error() + "\n"
	}
	return output + "\n[ERROR] " + err.Error() + "\n"
}

func opsPipelineConfigString(config map[string]any, key string) string {
	if config == nil {
		return ""
	}
	value, ok := config[key]
	if !ok || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func opsPipelineConfigStringDefault(config map[string]any, key string, fallback string) string {
	if value := opsPipelineConfigString(config, key); value != "" {
		return value
	}
	return fallback
}

func opsPipelineConfigUint(config map[string]any, key string) uint {
	value := opsPipelineConfigString(config, key)
	if value == "" {
		return 0
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0
	}
	return uint(parsed)
}

func opsPipelineImageName(config map[string]any, execInfo opsPipelineExecution) string {
	image := opsPipelineConfigString(config, "image")
	if image != "" {
		return opsPipelineReplaceVars(image, execInfo)
	}
	repository := opsPipelineConfigString(config, "repository")
	if repository == "" {
		repository = execInfo.App.Code
	}
	repository = opsPipelineReplaceVars(repository, execInfo)
	if !strings.Contains(repository, ":") {
		repository += ":" + execInfo.ImageTag
	}
	return repository
}

func opsPipelineReplaceVars(value string, execInfo opsPipelineExecution) string {
	replacer := strings.NewReplacer(
		"{{appCode}}", execInfo.App.Code,
		"{{appName}}", execInfo.App.Name,
		"{{env}}", execInfo.Env,
		"{{branch}}", execInfo.Branch,
		"{{imageTag}}", execInfo.ImageTag,
	)
	return replacer.Replace(value)
}

func (s *Service) RunOpsAppRelease(payload OpsAppReleasePayload) (map[string]any, error) {
	app, err := s.GetOpsApplication(payload.AppID)
	if err != nil {
		return nil, err
	}
	if app.Status != 1 {
		return nil, errors.New("当前应用已禁用，无法发布")
	}
	buildScript := strings.TrimSpace(app.BuildScript)
	if buildScript == "" {
		var task model.OpsAppBuildTask
		err := s.db.Where("app_id = ? AND status = ?", app.ID, 1).Order("id DESC").First(&task).Error
		if err == nil {
			return s.RunOpsAppBuildTask(OpsAppBuildRunPayload{TaskID: task.ID, Version: payload.Version, Branch: payload.Branch})
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		return nil, errors.New("请先为应用创建构建任务")
	}
	branch := Trimmed(payload.Branch)
	if branch == "" {
		branch = app.Branch
	}
	version := Trimmed(payload.Version)
	if version == "" {
		version = time.Now().Format("20060102150405")
	}
	now := time.Now()
	workspace := s.resolveOpsAppWorkspace(*app)
	release := model.OpsAppRelease{
		AppID: app.ID, AppName: app.Name, AppCode: app.Code, Env: app.Env, Version: version, RepoType: app.RepoType,
		RepoURL: app.RepoURL, Branch: branch, Workspace: workspace, Status: "running", Stage: "checkout",
		Summary: "构建任务已创建，正在拉取代码", StartedAt: &now,
	}
	if err := s.db.Create(&release).Error; err != nil {
		return nil, err
	}
	_ = s.db.Model(&model.OpsApplication{}).Where("id = ?", app.ID).Updates(map[string]any{
		"last_release_id": release.ID, "last_status": "running",
	})
	go s.runOpsAppBuild(opsBuildExecution{
		ReleaseID: release.ID, App: *app, Env: app.Env, BuildScript: buildScript, DeployScript: app.DeployScript,
		TimeoutSeconds: 1800, Branch: branch, Workspace: workspace,
	})
	return map[string]any{"releaseId": release.ID, "status": release.Status, "summary": release.Summary}, nil
}

func (s *Service) resolveOpsAppWorkspace(app model.OpsApplication) string {
	if strings.TrimSpace(app.Workspace) != "" {
		return strings.TrimSpace(app.Workspace)
	}
	return filepath.Join("uploads", "apps", app.Code)
}

func (s *Service) runOpsAppBuild(execInfo opsBuildExecution) {
	started := time.Now()
	workspace := execInfo.Workspace
	buildLog, deployLog := "", ""
	status, stage, summary := "success", "done", "构建发布完成"
	commitID := ""
	timeout := time.Duration(normalizeOpsBuildTimeout(execInfo.TimeoutSeconds)) * time.Second

	if err := os.MkdirAll(filepath.Dir(workspace), 0o755); err != nil {
		status, stage, summary = "failed", "prepare", err.Error()
	} else {
		stage = "checkout"
		_ = s.db.Model(&model.OpsAppRelease{}).Where("id = ?", execInfo.ReleaseID).Updates(map[string]any{"stage": stage, "summary": "正在拉取代码"})
		checkoutLog, err := s.checkoutOpsAppCode(execInfo.App, workspace, execInfo.Branch)
		buildLog += sectionLog("Git Clone", checkoutLog)
		if err != nil {
			status, summary = "failed", err.Error()
		} else {
			commitID = s.detectOpsAppCommit(execInfo.App.RepoType, workspace)
			stage = "build"
			_ = s.db.Model(&model.OpsAppRelease{}).Where("id = ?", execInfo.ReleaseID).Updates(map[string]any{
				"stage": stage, "summary": "正在执行构建脚本", "build_log": buildLog, "commit_id": commitID,
			})
			buildOutput, buildErr := runOpsAppShell(execInfo.BuildScript, workspace, timeout)
			buildLog += sectionLog("Build", buildOutput)
			if buildErr != nil {
				status, summary = "failed", buildErr.Error()
			} else if strings.TrimSpace(execInfo.DeployScript) != "" {
				stage = "deploy"
				_ = s.db.Model(&model.OpsAppRelease{}).Where("id = ?", execInfo.ReleaseID).Updates(map[string]any{
					"stage": stage, "summary": "正在执行发布脚本", "build_log": buildLog,
				})
				var deployErr error
				deployLog, deployErr = runOpsAppShell(execInfo.DeployScript, workspace, timeout)
				deployLog = sectionLog("Deploy", deployLog)
				if deployErr != nil {
					status, summary = "failed", deployErr.Error()
				}
			}
		}
	}
	finished := time.Now()
	if status != "success" && summary == "" {
		summary = "构建发布失败"
	}
	_ = s.db.Model(&model.OpsAppRelease{}).Where("id = ?", execInfo.ReleaseID).Updates(map[string]any{
		"status": status, "stage": stage, "summary": summary, "build_log": buildLog, "deploy_log": deployLog,
		"commit_id": commitID, "finished_at": &finished, "duration_ms": finished.Sub(started).Milliseconds(),
	})
	updates := map[string]any{"last_status": status}
	if status == "success" {
		updates["last_released_at"] = &finished
	}
	_ = s.db.Model(&model.OpsApplication{}).Where("id = ?", execInfo.App.ID).Updates(updates)
	if execInfo.TaskID > 0 {
		taskUpdates := map[string]any{"last_status": status}
		if status == "success" {
			taskUpdates["success_count"] = gorm.Expr("success_count + 1")
		} else {
			taskUpdates["failed_count"] = gorm.Expr("failed_count + 1")
		}
		_ = s.db.Model(&model.OpsAppBuildTask{}).Where("id = ?", execInfo.TaskID).Updates(taskUpdates)
	}
}

func sectionLog(name string, output string) string {
	if strings.TrimSpace(output) == "" {
		return fmt.Sprintf("\n===== %s =====\n", name)
	}
	return fmt.Sprintf("\n===== %s =====\n%s\n", name, output)
}

func (s *Service) checkoutOpsAppCode(app model.OpsApplication, workspace string, branch string) (string, error) {
	if app.RepoType == "svn" {
		if _, err := os.Stat(filepath.Join(workspace, ".svn")); err == nil {
			return runOpsAppCommand(workspace, 15*time.Minute, "svn", "update")
		}
		return runOpsAppCommand(".", 15*time.Minute, "svn", "checkout", app.RepoURL, workspace)
	}
	if _, err := os.Stat(filepath.Join(workspace, ".git")); err == nil {
		output, checkoutErr := runOpsAppCommand(workspace, 15*time.Minute, "git", "checkout", branch)
		pullOutput, pullErr := runOpsAppCommand(workspace, 15*time.Minute, "git", "pull", "--ff-only")
		if checkoutErr != nil {
			return output + pullOutput, checkoutErr
		}
		return output + pullOutput, pullErr
	}
	args := []string{"clone"}
	if branch != "" {
		args = append(args, "-b", branch)
	}
	args = append(args, app.RepoURL, workspace)
	return runOpsAppCommand(".", 20*time.Minute, "git", args...)
}

func (s *Service) detectOpsAppCommit(repoType string, workspace string) string {
	if repoType == "svn" {
		output, err := runOpsAppCommand(workspace, 10*time.Second, "svn", "info", "--show-item", "revision")
		if err == nil {
			return strings.TrimSpace(output)
		}
		return ""
	}
	output, err := runOpsAppCommand(workspace, 10*time.Second, "git", "rev-parse", "--short", "HEAD")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(output)
}

func runOpsAppCommand(dir string, timeout time.Duration, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return output.String(), fmt.Errorf("%s 执行超时", name)
	}
	return output.String(), err
}

func runOpsAppShell(script string, dir string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", script)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", script)
	}
	cmd.Dir = dir
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return output.String(), errors.New("脚本执行超时")
	}
	return output.String(), err
}
