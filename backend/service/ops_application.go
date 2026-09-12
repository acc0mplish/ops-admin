package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	pathpkg "path"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"ops-admin/backend/model"
)

type OpsApplicationPayload struct {
	ID               uint                                      `json:"id"`
	Name             string                                    `json:"name"`
	Code             string                                    `json:"code"`
	ServiceType      string                                    `json:"serviceType"`
	RepoType         string                                    `json:"repoType"`
	RepoURL          string                                    `json:"repoUrl"`
	RepoCredentialID uint                                      `json:"repoCredentialId"`
	Branch           string                                    `json:"branch"`
	Workspace        string                                    `json:"workspace"`
	BuildScript      string                                    `json:"buildScript"`
	DeployScript     string                                    `json:"deployScript"`
	Env              string                                    `json:"env"`
	Status           int                                       `json:"status"`
	Description      string                                    `json:"description"`
	Bindings         []OpsApplicationEnvironmentBindingPayload `json:"bindings"`
}

type OpsApplicationEnvironmentBindingPayload struct {
	Env                 string `json:"env"`
	HostGroupID         uint   `json:"hostGroupId"`
	Namespace           string `json:"namespace"`
	WorkloadType        string `json:"workloadType"`
	WorkloadName        string `json:"workloadName"`
	DatabaseID          uint   `json:"databaseId"`
	MonitorDatasourceID uint   `json:"monitorDatasourceId"`
	GatewayID           uint   `json:"gatewayId"`
}

func normalizeOpsRepoType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "svn":
		return "svn"
	default:
		return "git"
	}
}

func normalizeOpsSVNRevision(value string) (string, error) {
	revision := strings.ToUpper(strings.TrimSpace(value))
	if revision == "" || revision == "HEAD" {
		return "HEAD", nil
	}
	for _, char := range revision {
		if char < '0' || char > '9' {
			return "", errors.New("SVN revision supports only HEAD or a numeric revision")
		}
	}
	return revision, nil
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

func (s *Service) ListOpsApplications(pageNum, pageSize int, keyword, repoType, status, serviceType, env string) (map[string]any, error) {
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
	if env = normalizeEnvCode(env); env != "" {
		bindingAppIDs := s.db.Model(&model.OpsApplicationEnvironmentBinding{}).Select("app_id").Where("env = ? AND status = ?", env, 1)
		query = query.Where("env = ? OR id IN (?)", env, bindingAppIDs)
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
			"repoUrl": item.RepoURL, "branch": item.Branch, "workspace": item.Workspace, "env": item.Env, "serviceType": item.ServiceType,
			"repoCredentialId": item.RepoCredentialID,
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
		Name:             Trimmed(payload.Name),
		Code:             Trimmed(payload.Code),
		ServiceType:      Trimmed(payload.ServiceType),
		RepoType:         normalizeOpsRepoType(payload.RepoType),
		RepoURL:          Trimmed(payload.RepoURL),
		RepoCredentialID: payload.RepoCredentialID,
		Branch:           Trimmed(payload.Branch),
		BuildScript:      strings.TrimSpace(payload.BuildScript),
		DeployScript:     strings.TrimSpace(payload.DeployScript),
		Env:              Trimmed(payload.Env),
		Status:           normalizeOpsAppStatus(payload.Status),
		Description:      Trimmed(payload.Description),
	}
	if item.Name == "" {
		return errors.New("application name is required")
	}
	if item.Code == "" {
		return errors.New("application code is required")
	}
	if item.RepoURL == "" {
		return errors.New("repository URL is required")
	}
	if item.ServiceType == "" {
		item.ServiceType = "Backend Service"
	}
	if item.RepoType == "svn" {
		var err error
		item.Branch, err = normalizeOpsSVNRevision(item.Branch)
		if err != nil {
			return err
		}
	} else if item.Branch == "" {
		item.Branch = "master"
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		appID := payload.ID
		if appID == 0 {
			if err := tx.Create(&item).Error; err != nil {
				return err
			}
			appID = item.ID
		} else if err := tx.Model(&model.OpsApplication{}).Where("id = ?", appID).Updates(map[string]any{
			"name": item.Name, "code": item.Code, "service_type": item.ServiceType, "repo_type": item.RepoType,
			"repo_url": item.RepoURL, "repo_credential_id": item.RepoCredentialID, "branch": item.Branch,
			"build_script": item.BuildScript, "deploy_script": item.DeployScript, "env": item.Env,
			"status": item.Status, "description": item.Description,
		}).Error; err != nil {
			return err
		}
		if payload.Bindings == nil {
			return nil
		}
		if err := tx.Where("app_id = ?", appID).Delete(&model.OpsApplicationEnvironmentBinding{}).Error; err != nil {
			return err
		}
		seen := map[string]struct{}{}
		for _, binding := range payload.Bindings {
			env := normalizeEnvCode(binding.Env)
			if env == "" {
				continue
			}
			if _, ok := seen[env]; ok {
				return fmt.Errorf("environment %s can be bound only once", env)
			}
			seen[env] = struct{}{}
			row := model.OpsApplicationEnvironmentBinding{
				AppID: appID, Env: env, HostGroupID: binding.HostGroupID,
				Namespace: Trimmed(binding.Namespace), WorkloadType: strings.ToLower(Trimmed(binding.WorkloadType)),
				WorkloadName: Trimmed(binding.WorkloadName), DatabaseID: binding.DatabaseID,
				MonitorDatasourceID: binding.MonitorDatasourceID, GatewayID: binding.GatewayID, Status: 1,
			}
			if row.WorkloadType == "" {
				row.WorkloadType = "deployment"
			}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Service) ListOpsApplicationEnvironmentBindings(appID uint) ([]model.OpsApplicationEnvironmentBinding, error) {
	var list []model.OpsApplicationEnvironmentBinding
	err := s.db.Where("app_id = ?", appID).Order("env ASC").Find(&list).Error
	return list, err
}

func (s *Service) DeleteOpsApplication(id uint) error {
	var count int64
	if err := s.db.Model(&model.OpsAppBuildTask{}).Where("app_id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return errors.New("the application still has build tasks; delete them first")
	}
	if err := s.db.Model(&model.OpsAppPipeline{}).Where("app_id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return errors.New("the application still has CI/CD pipelines; delete them first")
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("app_id = ?", id).Delete(&model.OpsApplicationEnvironmentBinding{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.OpsApplication{}, id).Error
	})
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

func opsPipelineImageTag(branch string, now time.Time) string {
	branch = strings.ToLower(strings.TrimSpace(branch))
	branch = strings.NewReplacer("/", "-", "_", "-", " ", "-", ":", "-").Replace(branch)
	branch = strings.Trim(branch, "-.")
	if branch == "" {
		branch = "main"
	}
	return branch + "-" + now.Format("20060102150405")
}

func (s *Service) opsPipelineImageName(config map[string]any, execInfo opsPipelineExecution) (string, error) {
	image := opsPipelineConfigString(config, "image")
	if image != "" {
		return opsPipelineReplaceVars(image, execInfo), nil
	}
	registryID := opsPipelineConfigUint(config, "registryId")
	if registryID > 0 {
		var registry model.OpsImageRegistry
		if err := s.db.Where("id = ? AND status = ?", registryID, 1).First(&registry).Error; err != nil {
			return "", errors.New("selected image registry does not exist or is disabled")
		}
		parts := []string{strings.Trim(registry.Address, "/")}
		if namespace := strings.Trim(registry.Namespace, "/"); namespace != "" {
			parts = append(parts, namespace)
		}
		parts = append(parts, execInfo.App.Code)
		return strings.Join(parts, "/") + ":" + execInfo.ImageTag, nil
	}
	repository := opsPipelineConfigString(config, "repository")
	if repository == "" {
		repository = execInfo.App.Code
	}
	repository = opsPipelineReplaceVars(repository, execInfo)
	if !strings.Contains(repository, ":") {
		repository += ":" + execInfo.ImageTag
	}
	return repository, nil
}

func (s *Service) loginOpsPipelineRegistry(config map[string]any, workspace string, timeout time.Duration) (string, error) {
	registryID := opsPipelineConfigUint(config, "registryId")
	if registryID == 0 {
		return "Image registry credentials are not configured; skipping docker login.\n", nil
	}
	var registry model.OpsImageRegistry
	if err := s.db.Where("id = ? AND status = ?", registryID, 1).First(&registry).Error; err != nil {
		return "", errors.New("selected image registry does not exist or is disabled")
	}
	if strings.TrimSpace(registry.Username) == "" || strings.TrimSpace(registry.Password) == "" {
		return "Registry credentials are not configured; using the current Docker client session.\n", nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", "login", registry.Address, "--username", registry.Username, "--password-stdin")
	cmd.Dir = workspace
	cmd.Stdin = strings.NewReader(registry.Password)
	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return string(output), fmt.Errorf("docker login timed out (%s)", timeout)
	}
	return string(output), err
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
		return nil, errors.New("the current application is disabled and cannot be deployed")
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
		return nil, errors.New("create a build task for the application first")
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
		Summary: "build task created; checking out source code", StartedAt: &now,
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
	buildLogs := newOpsBuildLogWriter(s, execInfo.ReleaseID, "build_log")
	postBuildLogs := newOpsBuildLogWriter(s, execInfo.ReleaseID, "deploy_log")
	status, stage, summary := "success", "done", "build and post-build operations completed"
	commitID := ""
	timeout := time.Duration(normalizeOpsBuildTimeout(execInfo.TimeoutSeconds)) * time.Second

	if strings.EqualFold(execInfo.RunnerType, "host") {
		remoteWorkspace := filepath.ToSlash(workspace)
		var host model.AssetHost
		if err := s.db.Preload("Credential").Preload("Gateway").Preload("Gateway.Credential").First(&host, execInfo.RunnerHostID).Error; err != nil {
			status, stage, summary = "failed", "prepare", "build host does not exist or is unavailable"
		} else {
			checkoutCommand := s.remoteOpsAppCheckoutCommand(execInfo.App, remoteWorkspace, execInfo.Branch)
			buildLogs.Append(sectionLog("Remote Checkout", ""))
			_ = s.db.Model(&model.OpsAppRelease{}).Where("id = ?", execInfo.ReleaseID).Updates(map[string]any{"stage": "checkout", "summary": "Checking out source code"}).Error
			checkoutResult := s.execCommandOnHostStreaming(host, checkoutCommand, normalizeOpsBuildTimeout(execInfo.TimeoutSeconds), func(chunk string) {
				buildLogs.Append(s.sanitizeOpsAppLog(execInfo.App, chunk))
			})
			buildLogs.Flush()
			if checkoutResult.Status != "success" {
				if checkoutResult.ErrorText != "" {
					buildLogs.Append("\nERROR: " + checkoutResult.ErrorText + "\n")
				}
				status, stage, summary = "failed", "checkout", firstNonEmpty(checkoutResult.ErrorText, "Remote Source Checkout failed")
			} else {
				commitResult := s.execCommandOnHost(host, "cd "+shellQuote(remoteWorkspace)+" && (git rev-parse --short HEAD 2>/dev/null || svn info --show-item revision 2>/dev/null || true)", 30)
				commitID = strings.TrimSpace(commitResult.Stdout)
				variables := s.opsAppBuildEnvironment(execInfo, commitID, remoteWorkspace)
				stage = "build"
				buildLogs.Append(sectionLog("Remote Build", ""))
				_ = s.db.Model(&model.OpsAppRelease{}).Where("id = ?", execInfo.ReleaseID).Updates(map[string]any{"stage": stage, "summary": "Executing build script", "commit_id": commitID}).Error
				buildResult := s.execCommandOnHostStreaming(host, remoteOpsAppScriptCommand(remoteWorkspace, execInfo.BuildScript, variables), normalizeOpsBuildTimeout(execInfo.TimeoutSeconds), func(chunk string) {
					buildLogs.Append(s.sanitizeOpsAppLog(execInfo.App, chunk))
				})
				buildLogs.Flush()
				if buildResult.Status != "success" {
					if buildResult.ErrorText != "" {
						buildLogs.Append("\nERROR: " + buildResult.ErrorText + "\n")
					}
					status, summary = "failed", firstNonEmpty(buildResult.ErrorText, "remote build failed")
				} else if strings.TrimSpace(execInfo.DeployScript) != "" {
					stage = "post_build"
					postBuildLogs.Append(sectionLog("Remote Post Build", ""))
					_ = s.db.Model(&model.OpsAppRelease{}).Where("id = ?", execInfo.ReleaseID).Updates(map[string]any{"stage": stage, "summary": "Executing post-build operation"}).Error
					postResult := s.execCommandOnHostStreaming(host, remoteOpsAppScriptCommand(remoteWorkspace, execInfo.DeployScript, variables), normalizeOpsBuildTimeout(execInfo.TimeoutSeconds), func(chunk string) {
						postBuildLogs.Append(s.sanitizeOpsAppLog(execInfo.App, chunk))
					})
					postBuildLogs.Flush()
					if postResult.Status != "success" {
						if postResult.ErrorText != "" {
							postBuildLogs.Append("\nERROR: " + postResult.ErrorText + "\n")
						}
						status, summary = "failed", firstNonEmpty(postResult.ErrorText, "remote post-build operation failed")
					}
				}
			}
		}
	} else if err := os.MkdirAll(filepath.Dir(workspace), 0o755); err != nil {
		status, stage, summary = "failed", "prepare", err.Error()
	} else {
		stage = "checkout"
		_ = s.db.Model(&model.OpsAppRelease{}).Where("id = ?", execInfo.ReleaseID).Updates(map[string]any{"stage": stage, "summary": "Checking out source code"})
		checkoutLog, err := s.checkoutOpsAppCode(execInfo.App, workspace, execInfo.Branch)
		buildLogs.Append(sectionLog("Git Clone", s.sanitizeOpsAppLog(execInfo.App, checkoutLog)))
		if err != nil {
			status, summary = "failed", err.Error()
		} else {
			commitID = s.detectOpsAppCommit(execInfo.App.RepoType, workspace)
			variables := s.opsAppBuildEnvironment(execInfo, commitID, workspace)
			stage = "build"
			_ = s.db.Model(&model.OpsAppRelease{}).Where("id = ?", execInfo.ReleaseID).Updates(map[string]any{
				"stage": stage, "summary": "Executing build script", "build_log": buildLogs.String(), "commit_id": commitID,
			})
			buildOutput, buildErr := runOpsAppShellWithEnv(execInfo.BuildScript, workspace, timeout, variables)
			buildLogs.Append(sectionLog("Build", buildOutput))
			if buildErr != nil {
				status, summary = "failed", buildErr.Error()
			} else if strings.TrimSpace(execInfo.DeployScript) != "" {
				stage = "post_build"
				_ = s.db.Model(&model.OpsAppRelease{}).Where("id = ?", execInfo.ReleaseID).Updates(map[string]any{
					"stage": stage, "summary": "Executing post-build operation", "build_log": buildLogs.String(), "commit_id": commitID,
				})
				postOutput, postErr := runOpsAppShellWithEnv(execInfo.DeployScript, workspace, timeout, variables)
				postBuildLogs.Append(sectionLog("Post Build", postOutput))
				if postErr != nil {
					status, summary = "failed", postErr.Error()
				}
			}
		}
	}
	finished := time.Now()
	if status != "success" && summary == "" {
		summary = "Build failed"
	}
	buildLogs.Flush()
	postBuildLogs.Flush()
	_ = s.db.Model(&model.OpsAppRelease{}).Where("id = ?", execInfo.ReleaseID).Updates(map[string]any{
		"status": status, "stage": stage, "summary": summary, "build_log": buildLogs.String(), "deploy_log": postBuildLogs.String(),
		"commit_id": commitID, "finished_at": &finished, "duration_ms": finished.Sub(started).Milliseconds(),
	})
	updates := map[string]any{"last_status": status}
	if status == "success" {
		artifactURI := strings.TrimSpace(execInfo.ArtifactPath)
		if artifactURI == "" {
			artifactURI = workspace
			if strings.EqualFold(execInfo.RunnerType, "host") {
				artifactURI = filepath.ToSlash(workspace)
			}
		}
		_ = s.db.Create(&model.OpsAppArtifact{
			AppID: execInfo.App.ID, AppName: execInfo.App.Name, BuildTaskID: execInfo.TaskID,
			ReleaseID: execInfo.ReleaseID, Env: execInfo.Env, Version: execInfo.Version, CommitID: commitID,
			Type: firstNonEmpty(execInfo.ArtifactType, "file"), URI: artifactURI, Status: "ready",
		}).Error
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

func (s *Service) remoteOpsAppCheckoutCommand(app model.OpsApplication, workspace, branch string) string {
	target := filepath.ToSlash(workspace)
	parent := pathpkg.Dir(target)
	repoURL := s.opsAppAuthenticatedRepoURL(app)
	if app.RepoType == "svn" {
		revision := strings.TrimSpace(branch)
		revisionArg := ""
		if revision != "" && !strings.EqualFold(revision, "HEAD") {
			revisionArg = " -r " + shellQuote(revision)
		}
		return fmt.Sprintf("mkdir -p %s && if [ -d %s/.svn ]; then cd %s && svn update%s; else svn checkout%s %s %s; fi", shellQuote(parent), shellQuote(target), shellQuote(target), revisionArg, revisionArg, shellQuote(repoURL), shellQuote(target))
	}
	branchArg := ""
	if strings.TrimSpace(branch) != "" {
		branchArg = " -b " + shellQuote(branch)
	}
	gitRetry := "git_retry() { attempts=3; attempt=1; until GIT_TERMINAL_PROMPT=0 git \"$@\"; do rc=$?; if [ $attempt -ge $attempts ]; then exit $rc; fi; echo \"Git command failed (attempt $attempt/$attempts), retrying in 3 seconds...\" >&2; sleep 3; attempt=$((attempt + 1)); done; }; "
	checkoutCommand := "git_retry pull --ff-only"
	if strings.TrimSpace(branch) != "" {
		checkoutCommand = "git checkout " + shellQuote(branch) + " && git_retry pull --ff-only"
	}
	return fmt.Sprintf("%smkdir -p %s && if [ -d %s/.git ]; then cd %s && git_retry fetch --all --prune && %s; else git_retry clone%s %s %s; fi", gitRetry, shellQuote(parent), shellQuote(target), shellQuote(target), checkoutCommand, branchArg, shellQuote(repoURL), shellQuote(target))
}

func (s *Service) opsAppBuildEnvironment(execInfo opsBuildExecution, commitID, buildPath string) map[string]string {
	variables := make(map[string]string, len(execInfo.Params)+12)
	for key, value := range execInfo.Params {
		variables[key] = value
	}
	variables["BUILD_NUMBER"] = strconv.FormatUint(uint64(execInfo.ReleaseID), 10)
	variables["VERSION"] = execInfo.Version
	variables["COMMIT_ID"] = commitID
	variables["BRANCH"] = execInfo.Branch
	variables["PROJECT_NAME"] = execInfo.App.Name
	variables["PROJECT_ID"] = strconv.FormatUint(uint64(execInfo.App.ID), 10)
	variables["PROJECT_REPO"] = execInfo.App.RepoURL
	variables["TASK_NAME"] = execInfo.TaskName
	variables["TASK_ID"] = strconv.FormatUint(uint64(execInfo.TaskID), 10)
	variables["ENVIRONMENT"] = execInfo.Env
	variables["ENVIRONMENT_TYPE"] = execInfo.Env
	variables["BUILD_PATH"] = buildPath
	return variables
}

func remoteOpsAppScriptCommand(workspace, script string, variables map[string]string) string {
	keys := make([]string, 0, len(variables))
	for key := range variables {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	exports := make([]string, 0, len(keys))
	for _, key := range keys {
		exports = append(exports, "export "+key+"="+shellQuote(variables[key]))
	}
	return "cd " + shellQuote(workspace) + " && " + strings.Join(exports, " && ") + " && sh -c " + shellQuote(script)
}

func sectionLog(name string, output string) string {
	if strings.TrimSpace(output) == "" {
		return fmt.Sprintf("\n===== %s =====\n", name)
	}
	return fmt.Sprintf("\n===== %s =====\n%s\n", name, output)
}

func (s *Service) checkoutOpsAppCode(app model.OpsApplication, workspace string, branch string) (string, error) {
	repoURL := s.opsAppAuthenticatedRepoURL(app)
	if app.RepoType == "svn" {
		revision := strings.TrimSpace(branch)
		revisionArgs := []string{}
		if revision != "" && !strings.EqualFold(revision, "HEAD") {
			revisionArgs = append(revisionArgs, "-r", revision)
		}
		if _, err := os.Stat(filepath.Join(workspace, ".svn")); err == nil {
			return runOpsAppCommand(workspace, 15*time.Minute, "svn", append([]string{"update"}, revisionArgs...)...)
		}
		args := append([]string{"checkout"}, revisionArgs...)
		args = append(args, repoURL, workspace)
		return runOpsAppCommand(".", 15*time.Minute, "svn", args...)
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
	args = append(args, repoURL, workspace)
	return runOpsAppCommand(".", 20*time.Minute, "git", args...)
}

func (s *Service) opsAppAuthenticatedRepoURL(app model.OpsApplication) string {
	if app.RepoCredentialID == 0 {
		return app.RepoURL
	}
	var credential model.AssetCredential
	if err := s.db.First(&credential, app.RepoCredentialID).Error; err != nil {
		return app.RepoURL
	}
	parsed, err := url.Parse(app.RepoURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return app.RepoURL
	}
	username := credential.Username
	if username == "" {
		username = "token"
	}
	parsed.User = url.UserPassword(username, credential.Password)
	return parsed.String()
}

func (s *Service) sanitizeOpsAppLog(app model.OpsApplication, value string) string {
	if app.RepoCredentialID == 0 {
		return value
	}
	var credential model.AssetCredential
	if err := s.db.First(&credential, app.RepoCredentialID).Error; err != nil {
		return value
	}
	for _, secret := range []string{credential.Password, credential.Passphrase, credential.PrivateKey} {
		if strings.TrimSpace(secret) != "" {
			value = strings.ReplaceAll(value, secret, "******")
		}
	}
	return value
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
		return output.String(), fmt.Errorf("%s execution timed out", name)
	}
	return output.String(), err
}

func runOpsAppHealthCheck(rawURL string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	result := fmt.Sprintf("HTTP %d\n%s", resp.StatusCode, strings.TrimSpace(string(body)))
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return result, fmt.Errorf("health check returned HTTP %d", resp.StatusCode)
	}
	return result, nil
}

func runOpsAppShell(script string, dir string, timeout time.Duration) (string, error) {
	return runOpsAppShellWithEnv(script, dir, timeout, nil)
}

func runOpsAppShellWithEnv(script string, dir string, timeout time.Duration, variables map[string]string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", script)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", script)
	}
	cmd.Dir = dir
	cmd.Env = os.Environ()
	for key, value := range variables {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return output.String(), errors.New("script execution timed out")
	}
	return output.String(), err
}
