package service

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"ops-admin/backend/model"
)

type opsPipelineExecution struct {
	RunID        uint
	PipelineID   uint
	App          model.OpsApplication
	Branch       string
	Env          string
	ImageTag     string
	Workspace    string
	ExecutorHost model.AssetHost
	Params       map[string]string
	Stages       []OpsAppPipelineStageDefinition
	StartedAt    time.Time
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
	status, summary := "success", "pipeline execution completed"
	for _, stage := range execInfo.Stages {
		var persisted model.OpsAppPipelineRunStage
		_ = s.db.Where("run_id = ? AND stage_id = ?", execInfo.RunID, stage.ID).First(&persisted).Error
		if persisted.Status == "success" {
			continue
		}
		if stage.Type == "manual" {
			now := time.Now()
			_ = s.db.Model(&model.OpsAppPipelineRunStage{}).Where("run_id = ? AND stage_id = ?", execInfo.RunID, stage.ID).Updates(map[string]any{
				"status": "waiting_approval", "summary": "Awaiting manual approval", "started_at": &now,
			}).Error
			_ = s.db.Model(&model.OpsAppPipelineRun{}).Where("id = ?", execInfo.RunID).Updates(map[string]any{
				"status": "waiting_approval", "summary": "Awaiting manual approval: " + stage.Name, "approval_status": "pending",
			}).Error
			return
		}
		started := time.Now()
		_ = s.db.Model(&model.OpsAppPipelineRunStage{}).
			Where("run_id = ? AND stage_id = ?", execInfo.RunID, stage.ID).
			Updates(map[string]any{"status": "running", "summary": "Executing", "started_at": &started}).Error
		_ = s.db.Model(&model.OpsAppPipelineRun{}).Where("id = ?", execInfo.RunID).Update("summary", "Executing: "+stage.Name).Error

		stageStatus, stageSummary := "success", "Execution succeeded"
		logText, err := s.executeOpsAppPipelineStage(execInfo, stage)
		if err != nil {
			stageStatus, stageSummary = "failed", err.Error()
			if strings.EqualFold(stage.FailurePolicy, "ignore") {
				stageStatus, stageSummary = "success", "Execution failed and was ignored by policy: "+err.Error()
				logText += "\n[WARN] " + stageSummary + "\n"
			} else {
				status, summary = "failed", stage.Name+" execution failed: "+err.Error()
			}
		}
		finished := time.Now()
		if strings.TrimSpace(logText) == "" {
			logText = fmt.Sprintf("[%s] Stage completed: %s\n", finished.Format("2006-01-02 15:04:05"), stage.Name)
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

func (s *Service) ApproveOpsAppPipelineRun(payload OpsAppPipelineApprovalPayload) error {
	if payload.RunID == 0 {
		return errors.New("pipeline run ID is required")
	}
	var run model.OpsAppPipelineRun
	if err := s.db.First(&run, payload.RunID).Error; err != nil {
		return err
	}
	if run.Status != "waiting_approval" {
		return errors.New("the current pipeline is not awaiting approval")
	}
	var stage model.OpsAppPipelineRunStage
	if err := s.db.Where("run_id = ? AND status = ?", run.ID, "waiting_approval").Order("id ASC").First(&stage).Error; err != nil {
		return errors.New("no stage awaiting approval was found")
	}
	decision := strings.ToLower(strings.TrimSpace(payload.Decision))
	operator := firstNonEmpty(payload.Operator, "System Administrator")
	now := time.Now()
	if decision != "approve" {
		_ = s.db.Model(&model.OpsAppPipelineRunStage{}).Where("id = ?", stage.ID).Updates(map[string]any{"status": "failed", "summary": "Approval rejected: " + payload.Note, "finished_at": &now}).Error
		return s.db.Model(&model.OpsAppPipelineRun{}).Where("id = ?", run.ID).Updates(map[string]any{
			"status": "failed", "summary": "manual approval was rejected", "approval_status": "rejected", "approver": operator,
			"approval_note": payload.Note, "finished_at": &now,
		}).Error
	}
	if err := s.db.Model(&model.OpsAppPipelineRunStage{}).Where("id = ?", stage.ID).Updates(map[string]any{
		"status": "success", "summary": "Approved: " + payload.Note, "finished_at": &now,
	}).Error; err != nil {
		return err
	}
	if err := s.db.Model(&model.OpsAppPipelineRun{}).Where("id = ?", run.ID).Updates(map[string]any{
		"status": "running", "summary": "approved; continuing execution", "approval_status": "approved", "approver": operator, "approval_note": payload.Note,
	}).Error; err != nil {
		return err
	}
	var app model.OpsApplication
	if err := s.db.First(&app, run.AppID).Error; err != nil {
		return err
	}
	executorHost, err := s.getOpsPipelineExecutorHost(run.ExecutorHostID)
	if err != nil {
		return err
	}
	stages, _, err := normalizeOpsPipelineStages(run.DefinitionJSON)
	if err != nil {
		return err
	}
	params := map[string]string{}
	_ = json.Unmarshal([]byte(run.ParamsJSON), &params)
	started := time.Now()
	if run.StartedAt != nil {
		started = *run.StartedAt
	}
	go s.runOpsAppPipeline(opsPipelineExecution{
		RunID: run.ID, PipelineID: run.PipelineID, App: app, Branch: run.Branch, Env: run.Env,
		ImageTag: run.ImageTag, Workspace: s.resolveOpsAppWorkspace(app), ExecutorHost: executorHost, Params: params, Stages: stages, StartedAt: started,
	})
	return nil
}

func (s *Service) RollbackOpsAppPipelineRun(runID uint, operator string) (map[string]any, error) {
	var current model.OpsAppPipelineRun
	if err := s.db.First(&current, runID).Error; err != nil {
		return nil, err
	}
	executorHost, err := s.getOpsPipelineExecutorHost(current.ExecutorHostID)
	if err != nil {
		return nil, err
	}
	var previous model.OpsAppPipelineRun
	if err := s.db.Where("pipeline_id = ? AND status = ? AND id < ? AND image_tag <> ''", current.PipelineID, "success", current.ID).Order("id DESC").First(&previous).Error; err != nil {
		return nil, errors.New("no successful historical version is available for rollback")
	}
	definitions, _, err := normalizeOpsPipelineStages(current.DefinitionJSON)
	if err != nil {
		return nil, err
	}
	stages := make([]OpsAppPipelineStageDefinition, 0)
	for _, stage := range definitions {
		if stage.Type == "k8sDeploy" || stage.Type == "notify" {
			stages = append(stages, stage)
		}
	}
	if len(stages) == 0 {
		return nil, errors.New("the current pipeline has no Kubernetes deployment stage and cannot be rolled back automatically")
	}
	normalized, _ := json.Marshal(map[string]any{"stages": stages})
	now := time.Now()
	run := model.OpsAppPipelineRun{
		PipelineID: current.PipelineID, PipelineName: current.PipelineName, AppID: current.AppID, AppName: current.AppName,
		AppCode: current.AppCode, Env: current.Env, Branch: previous.Branch, ImageTag: previous.ImageTag,
		ArtifactID: previous.ArtifactID, ExecutorHostID: current.ExecutorHostID, TriggerType: "rollback", TriggerUser: firstNonEmpty(operator, "System Administrator"),
		Status: "running", Summary: fmt.Sprintf("rolling back to run #%d / %s", previous.ID, previous.ImageTag),
		DefinitionJSON: string(normalized), StartedAt: &now,
	}
	if err := s.db.Create(&run).Error; err != nil {
		return nil, err
	}
	for _, stage := range stages {
		_ = s.db.Create(&model.OpsAppPipelineRunStage{RunID: run.ID, StageID: stage.ID, StageName: stage.Name, StageType: stage.Type, Status: "waiting", Summary: "Waiting"}).Error
	}
	var app model.OpsApplication
	if err := s.db.First(&app, run.AppID).Error; err != nil {
		return nil, err
	}
	go s.runOpsAppPipeline(opsPipelineExecution{RunID: run.ID, PipelineID: run.PipelineID, App: app, Branch: run.Branch, Env: run.Env, ImageTag: run.ImageTag, Workspace: s.resolveOpsAppWorkspace(app), ExecutorHost: executorHost, Stages: stages, StartedAt: now})
	return map[string]any{"runId": run.ID, "rollbackFromRunId": current.ID, "rollbackToRunId": previous.ID, "imageTag": previous.ImageTag}, nil
}

func (s *Service) executeOpsAppPipelineStage(execInfo opsPipelineExecution, stage OpsAppPipelineStageDefinition) (string, error) {
	header := fmt.Sprintf("[%s] Stage started: %s\nStage type: %s\nExecution policy: %s\nExecutor: %s (%s)\nWorkspace: %s\n\n",
		time.Now().Format("2006-01-02 15:04:05"), stage.Name, stage.Type, stage.FailurePolicy, execInfo.ExecutorHost.HostName, execInfo.ExecutorHost.SSHIP, execInfo.Workspace)
	timeout := time.Duration(normalizeOpsBuildTimeout(stage.TimeoutSeconds)) * time.Second
	// Commands, checkout, Docker and Kubernetes deployment intentionally execute through
	// the selected SSH host. This prevents an Ops Admin container from becoming an
	// accidental CI runner.
	if stage.Type != "manual" && stage.Type != "notify" {
		return s.executeOpsAppPipelineRemoteStage(execInfo, stage, header, timeout)
	}
	switch stage.Type {
	case "checkout":
		if err := os.MkdirAll(filepath.Dir(execInfo.Workspace), 0o755); err != nil {
			return header, err
		}
		output, err := s.checkoutOpsAppCode(execInfo.App, execInfo.Workspace, execInfo.Branch)
		return header + sectionLog("Source Checkout", output), err
	case "command", "test", "build":
		script := opsPipelineConfigString(stage.Config, "script")
		if script == "" {
			return header, errors.New("stage script is required")
		}
		if err := ensureOpsAppPipelineWorkspace(execInfo.Workspace); err != nil {
			return header, err
		}
		output, err := runOpsAppShell(script, execInfo.Workspace, timeout)
		return header + sectionLog("Execution Script", script) + sectionLog(stage.Name+" Output", appendOpsAppCommandError(output, err)), err
	case "dockerBuild":
		if err := ensureOpsAppPipelineWorkspace(execInfo.Workspace); err != nil {
			return header, err
		}
		image, imageErr := s.opsPipelineImageName(stage.Config, execInfo)
		if imageErr != nil {
			return header, imageErr
		}
		loginOutput, loginErr := s.loginOpsPipelineRegistry(stage.Config, execInfo.Workspace, timeout)
		if loginErr != nil {
			return header + sectionLog("Image Registry Login", loginOutput), loginErr
		}
		dockerfile := opsPipelineConfigStringDefault(stage.Config, "dockerfile", "Dockerfile")
		contextDir := opsPipelineConfigStringDefault(stage.Config, "context", ".")
		args := []string{"build", "-t", image, "-f", dockerfile, contextDir}
		output, err := runOpsAppCommand(execInfo.Workspace, timeout, "docker", args...)
		return header + sectionLog("Image Registry Login", loginOutput) + sectionLog("Docker Build: "+image, appendOpsAppCommandError(output, err)), err
	case "dockerPush":
		if err := ensureOpsAppPipelineWorkspace(execInfo.Workspace); err != nil {
			return header, err
		}
		image, imageErr := s.opsPipelineImageName(stage.Config, execInfo)
		if imageErr != nil {
			return header, imageErr
		}
		loginOutput, loginErr := s.loginOpsPipelineRegistry(stage.Config, execInfo.Workspace, timeout)
		if loginErr != nil {
			return header + sectionLog("Image Registry Login", loginOutput), loginErr
		}
		output, err := runOpsAppCommand(execInfo.Workspace, timeout, "docker", "push", image)
		return header + sectionLog("Image Registry Login", loginOutput) + sectionLog("Push Image to Registry: "+image, appendOpsAppCommandError(output, err)), err
	case "k8sDeploy":
		clusterID := opsPipelineConfigUint(stage.Config, "clusterId")
		if clusterID == 0 {
			return header, errors.New("Kubernetes deployment requires a target cluster")
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
			return header, fmt.Errorf("unsupported Kubernetes workload type: %s", workloadType)
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
		image, imageErr := s.opsPipelineImageName(stage.Config, execInfo)
		if imageErr != nil {
			return header, imageErr
		}
		target := workloadType + "/" + workload
		output, err := runOpsAppCommand(".", timeout, "kubectl", "--kubeconfig", kubeconfigPath, "-n", namespace, "set", "image", target, container+"="+image)
		if err != nil {
			return header + sectionLog("kubectl set image", appendOpsAppCommandError(output, err)), err
		}
		rollout, rolloutErr := runOpsAppCommand(".", timeout, "kubectl", "--kubeconfig", kubeconfigPath, "-n", namespace, "rollout", "status", target, "--timeout="+fmt.Sprintf("%ds", stage.TimeoutSeconds))
		logText := header + sectionLog("kubectl set image", output) + sectionLog("kubectl rollout status", appendOpsAppCommandError(rollout, rolloutErr))
		if rolloutErr != nil {
			return logText, rolloutErr
		}
		healthURL := opsPipelineConfigString(stage.Config, "healthUrl")
		if healthURL == "" {
			return logText, nil
		}
		healthOutput, healthErr := runOpsAppHealthCheck(healthURL, timeout)
		return logText + sectionLog("Post-deployment Health Check: "+healthURL, appendOpsAppCommandError(healthOutput, healthErr)), healthErr
	case "manual":
		return header + "The pipeline scheduler handles manual approval stages: execution pauses until approval is granted.\n", nil
	case "notify":
		ruleID := opsPipelineConfigUint(stage.Config, "notifyRuleId")
		now := time.Now()
		queued, err := s.enqueueNotifyRule(ruleID, NotifyEvent{
			Scope: "pipeline", Event: "notify", TargetID: execInfo.RunID,
			TargetName: execInfo.App.Name + " / " + stage.Name,
			Status:     "notify",
			Summary:    fmt.Sprintf("Pipeline #%d reached notification stage %q", execInfo.RunID, stage.Name),
			Detail:     fmt.Sprintf("Application: %s\nEnvironment: %s\nBranch: %s\nImage Version: %s", execInfo.App.Name, execInfo.Env, execInfo.Branch, execInfo.ImageTag),
			StartedAt:  &now, FinishedAt: &now,
			Extra: map[string]string{
				"pipelineName": fmt.Sprintf("Pipeline #%d", execInfo.PipelineID), "pipelineRunId": fmt.Sprintf("%d", execInfo.RunID),
				"appName": execInfo.App.Name, "env": execInfo.Env, "branch": execInfo.Branch, "imageTag": execInfo.ImageTag,
				"stageName": stage.Name, "notifyAt": now.Format("2006-01-02 15:04:05"),
			},
		}, false)
		if err != nil {
			return header, fmt.Errorf("failed to send notification: %w", err)
		}
		if queued == 0 {
			return header, errors.New("selected notification rule produced no valid delivery; verify the rule, template, and channel status")
		}
		return header + fmt.Sprintf("Created delivery tasks through notification rule #%d: %d task(s); review actual results in Notifications / Send Logs.\n", ruleID, queued), nil
	default:
		return header, fmt.Errorf("unsupported stage type: %s", stage.Type)
	}
}

func (s *Service) executeOpsAppPipelineRemoteStage(execInfo opsPipelineExecution, stage OpsAppPipelineStageDefinition, header string, timeout time.Duration) (string, error) {
	if execInfo.ExecutorHost.ID == 0 {
		return header, errors.New("pipeline has no configured executor")
	}
	seconds := normalizeOpsBuildTimeout(stage.TimeoutSeconds)
	run := func(name, command string) (string, error) {
		result := s.execCommandOnHost(execInfo.ExecutorHost, command, seconds)
		output := strings.TrimSpace(result.Stdout + "\n" + result.Stderr)
		return sectionLog(name, output), func() error {
			if result.Status == "success" {
				return nil
			}
			return errors.New(firstNonEmpty(result.ErrorText, "remote execution failed"))
		}()
	}
	workspace := filepath.ToSlash(execInfo.Workspace)
	switch stage.Type {
	case "checkout":
		output, err := run("Remote Source Checkout", s.remoteOpsAppCheckoutCommand(execInfo.App, workspace, execInfo.Branch))
		return header + output, err
	case "command", "test", "build":
		script := opsPipelineConfigString(stage.Config, "script")
		if script == "" {
			return header, errors.New("stage script is required")
		}
		output, err := run("Remote Script Execution", remoteOpsPipelineScriptCommand(workspace, script, execInfo))
		return header + sectionLog("Execution Script", script) + output, err
	case "dockerBuild":
		image, err := s.opsPipelineImageName(stage.Config, execInfo)
		if err != nil {
			return header, err
		}
		login, err := s.opsPipelineRegistryLoginCommand(stage.Config)
		if err != nil {
			return header, err
		}
		dockerfile := opsPipelineConfigStringDefault(stage.Config, "dockerfile", "Dockerfile")
		contextDir := opsPipelineConfigStringDefault(stage.Config, "context", ".")
		command := "cd " + shellQuote(workspace) + " && " + login + " && docker build -t " + shellQuote(image) + " -f " + shellQuote(dockerfile) + " " + shellQuote(contextDir)
		output, runErr := run("Remote Docker Build: "+image, command)
		return header + output, runErr
	case "dockerPush":
		image, err := s.opsPipelineImageName(stage.Config, execInfo)
		if err != nil {
			return header, err
		}
		login, err := s.opsPipelineRegistryLoginCommand(stage.Config)
		if err != nil {
			return header, err
		}
		output, runErr := run("Remote Registry Push: "+image, "cd "+shellQuote(workspace)+" && "+login+" && docker push "+shellQuote(image))
		return header + output, runErr
	case "k8sDeploy":
		clusterID := opsPipelineConfigUint(stage.Config, "clusterId")
		if clusterID == 0 {
			return header, errors.New("Kubernetes deployment requires a target cluster")
		}
		kubeconfigPath, cleanup, err := s.opsPipelineKubeconfigFile(clusterID)
		if err != nil {
			return header, err
		}
		defer cleanup()
		kubeconfig, err := os.ReadFile(kubeconfigPath)
		if err != nil {
			return header, err
		}
		namespace := firstNonEmpty(opsPipelineConfigString(stage.Config, "namespace"), execInfo.Env, "default")
		workloadType := strings.ToLower(opsPipelineConfigStringDefault(stage.Config, "workloadType", "deployment"))
		if workloadType != "deployment" && workloadType != "statefulset" && workloadType != "daemonset" {
			return header, fmt.Errorf("unsupported Kubernetes workload type: %s", workloadType)
		}
		workload := firstNonEmpty(opsPipelineConfigString(stage.Config, "workload"), execInfo.App.Code)
		container := firstNonEmpty(opsPipelineConfigString(stage.Config, "container"), execInfo.App.Code)
		image, err := s.opsPipelineImageName(stage.Config, execInfo)
		if err != nil {
			return header, err
		}
		target := workloadType + "/" + workload
		encoded := base64.StdEncoding.EncodeToString(kubeconfig)
		command := "kcfg=$(mktemp) && trap 'rm -f \"$kcfg\"' EXIT && base64 -d <<'__OPS_KUBECONFIG__' > \"$kcfg\"\n" + encoded + "\n__OPS_KUBECONFIG__\nkubectl --kubeconfig \"$kcfg\" -n " + shellQuote(namespace) + " set image " + shellQuote(target) + " " + shellQuote(container+"="+image) + " && kubectl --kubeconfig \"$kcfg\" -n " + shellQuote(namespace) + " rollout status " + shellQuote(target) + " --timeout=" + fmt.Sprintf("%ds", seconds)
		output, runErr := run("Remote kubectl Deployment: "+target, command)
		if runErr != nil {
			return header + output, runErr
		}
		healthURL := opsPipelineConfigString(stage.Config, "healthUrl")
		if healthURL == "" {
			return header + output, nil
		}
		healthOutput, healthErr := run("Remote Post-deployment Health Check: "+healthURL, "curl -fsS --max-time "+strconv.Itoa(seconds)+" "+shellQuote(healthURL))
		return header + output + healthOutput, healthErr
	default:
		return header, fmt.Errorf("unsupported stage type: %s", stage.Type)
	}
}

func remoteOpsPipelineScriptCommand(workspace, script string, execInfo opsPipelineExecution) string {
	variables := map[string]string{"BRANCH": execInfo.Branch, "ENVIRONMENT": execInfo.Env, "IMAGE_TAG": execInfo.ImageTag, "PROJECT_NAME": execInfo.App.Name, "PROJECT_CODE": execInfo.App.Code, "PIPELINE_RUN_ID": strconv.FormatUint(uint64(execInfo.RunID), 10)}
	for key, value := range execInfo.Params {
		variables[key] = value
	}
	return remoteOpsAppScriptCommand(workspace, script, variables)
}

func (s *Service) opsPipelineRegistryLoginCommand(config map[string]any) (string, error) {
	registryID := opsPipelineConfigUint(config, "registryId")
	if registryID == 0 {
		return "", errors.New("select an image registry")
	}
	var registry model.OpsImageRegistry
	if err := s.db.First(&registry, registryID).Error; err != nil {
		return "", errors.New("selected image registry does not exist")
	}
	if registry.Status != 1 {
		return "", errors.New("selected image registry is disabled")
	}
	if strings.EqualFold(opsPipelineConfigString(config, "loginMode"), "executor") {
		return "echo 'Using the Docker login session already available on the executor'", nil
	}
	if strings.TrimSpace(registry.Username) == "" || strings.TrimSpace(registry.Password) == "" {
		return "", errors.New("image registry has no username or password configured; use the executor existing login session or configure registry credentials")
	}
	return "printf %s " + shellQuote(registry.Password) + " | docker login " + shellQuote(registry.Address) + " --username " + shellQuote(registry.Username) + " --password-stdin", nil
}

func (s *Service) opsPipelineKubeconfigFile(clusterID uint) (string, func(), error) {
	cluster, err := s.GetK8sCluster(clusterID)
	if err != nil {
		return "", func() {}, err
	}
	if strings.TrimSpace(cluster.KubeConfig) == "" {
		return "", func() {}, errors.New("target cluster kubeconfig is empty")
	}
	content := cluster.KubeConfig
	tunnelCleanup := func() {}
	if normalizeConnectionMode(cluster.ConnectionMode) == "gateway" && cluster.GatewayID != nil && *cluster.GatewayID > 0 {
		rewritten, cleanup, err := s.gatewayKubeconfigForKubectl(cluster)
		if err != nil {
			return "", func() {}, err
		}
		content = rewritten
		tunnelCleanup = cleanup
	}
	file, err := os.CreateTemp("", fmt.Sprintf("ops-admin-kubeconfig-%d-*.yaml", clusterID))
	if err != nil {
		tunnelCleanup()
		return "", func() {}, err
	}
	path := file.Name()
	if _, err := file.WriteString(content); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		tunnelCleanup()
		return "", func() {}, err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		tunnelCleanup()
		return "", func() {}, err
	}
	return path, func() {
		_ = os.Remove(path)
		tunnelCleanup()
	}, nil
}

func (s *Service) gatewayKubeconfigForKubectl(cluster model.K8sCluster) (string, func(), error) {
	var cfg kubeConfig
	if err := yaml.Unmarshal([]byte(cluster.KubeConfig), &cfg); err != nil {
		return "", func() {}, err
	}
	runtime, err := parseKubeConfig(cluster.KubeConfig)
	if err != nil {
		return "", func() {}, err
	}
	parsed, err := url.Parse(runtime.Server)
	if err != nil {
		return "", func() {}, err
	}
	targetAddress := parsed.Host
	if !strings.Contains(targetAddress, ":") {
		if parsed.Scheme == "http" {
			targetAddress += ":80"
		} else {
			targetAddress += ":443"
		}
	}
	localAddress, cleanup, err := s.startGatewayTunnel(*cluster.GatewayID, targetAddress)
	if err != nil {
		return "", func() {}, err
	}
	localScheme := parsed.Scheme
	if localScheme == "" {
		localScheme = "https"
	}
	for i := range cfg.Clusters {
		cfg.Clusters[i].Cluster.Server = localScheme + "://" + localAddress
		cfg.Clusters[i].Cluster.InsecureSkipTLSVerify = true
		cfg.Clusters[i].Cluster.CertificateAuthorityData = ""
	}
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		cleanup()
		return "", func() {}, err
	}
	return string(data), cleanup, nil
}
