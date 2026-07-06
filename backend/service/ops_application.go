package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
