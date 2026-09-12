package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"ops-admin/backend/model"
)

type OpsAppPipelinePayload struct {
	ID             uint   `json:"id"`
	Name           string `json:"name"`
	AppID          uint   `json:"appId"`
	DefaultBranch  string `json:"defaultBranch"`
	Env            string `json:"env"`
	TechStack      string `json:"techStack"`
	TemplateID     uint   `json:"templateId"`
	BuildTaskID    uint   `json:"buildTaskId"`
	ExecutorHostID uint   `json:"executorHostId"`
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
	ArtifactID uint              `json:"artifactId"`
	Params     map[string]string `json:"params"`
}

type OpsAppPipelineApprovalPayload struct {
	RunID    uint   `json:"runId"`
	Decision string `json:"decision"`
	Note     string `json:"note"`
	Operator string `json:"operator"`
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
			ID: 1, Name: "Go Backend General Template", Category: "Go", TechStack: "go",
			Description: "Go compilation, image build, registry push, and workload update",
			Stages: []OpsAppPipelineStageDefinition{
				{ID: "checkout", Name: "Source Checkout", Type: "checkout", TimeoutSeconds: 600, FailurePolicy: "stop"},
				{ID: "deps", Name: "Go Dependency Installation", Type: "command", TimeoutSeconds: 600, FailurePolicy: "stop", Config: map[string]any{"script": "go mod download"}},
				{ID: "test", Name: "Unit Test", Type: "test", TimeoutSeconds: 900, FailurePolicy: "stop", Config: map[string]any{"script": "go test ./..."}},
				{ID: "build", Name: "Go Build", Type: "build", TimeoutSeconds: 900, FailurePolicy: "stop", Config: map[string]any{"script": "go build ./..."}},
				{ID: "docker-build", Name: "Docker Image Build", Type: "dockerBuild", TimeoutSeconds: 1200, FailurePolicy: "stop"},
				{ID: "docker-push", Name: "Push Image to Registry", Type: "dockerPush", TimeoutSeconds: 1200, FailurePolicy: "stop"},
				{ID: "k8s-deploy", Name: "Kubernetes Workload Update", Type: "k8sDeploy", TimeoutSeconds: 900, FailurePolicy: "stop"},
			},
		},
		{
			ID: 2, Name: "Maven Java General Template", Category: "Java", TechStack: "maven",
			Description: "Maven packaging, JAR image, and Kubernetes deployment",
			Stages: []OpsAppPipelineStageDefinition{
				{ID: "checkout", Name: "Source Checkout", Type: "checkout", TimeoutSeconds: 600, FailurePolicy: "stop"},
				{ID: "deps", Name: "Maven Dependency Installation", Type: "command", TimeoutSeconds: 900, FailurePolicy: "stop", Config: map[string]any{"script": "mvn dependency:go-offline"}},
				{ID: "test", Name: "Unit Test", Type: "test", TimeoutSeconds: 1200, FailurePolicy: "stop", Config: map[string]any{"script": "mvn test"}},
				{ID: "package", Name: "Maven Package", Type: "build", TimeoutSeconds: 1200, FailurePolicy: "stop", Config: map[string]any{"script": "mvn clean package -DskipTests"}},
				{ID: "docker-build", Name: "Docker Image Build", Type: "dockerBuild", TimeoutSeconds: 1200, FailurePolicy: "stop"},
				{ID: "docker-push", Name: "Push Image to Registry", Type: "dockerPush", TimeoutSeconds: 1200, FailurePolicy: "stop"},
				{ID: "k8s-deploy", Name: "Kubernetes Deployment", Type: "k8sDeploy", TimeoutSeconds: 900, FailurePolicy: "stop"},
			},
		},
		{
			ID: 3, Name: "Vue Frontend General Template", Category: "Node.js", TechStack: "vue",
			Description: "npm build, image packaging, and Kubernetes rolling deployment",
			Stages: []OpsAppPipelineStageDefinition{
				{ID: "checkout", Name: "Source Checkout", Type: "checkout", TimeoutSeconds: 600, FailurePolicy: "stop"},
				{ID: "install", Name: "npm install", Type: "command", TimeoutSeconds: 900, FailurePolicy: "stop", Config: map[string]any{"script": "npm install"}},
				{ID: "build", Name: "npm run build", Type: "build", TimeoutSeconds: 900, FailurePolicy: "stop", Config: map[string]any{"script": "npm run build"}},
				{ID: "docker-build", Name: "Docker Image Build", Type: "dockerBuild", TimeoutSeconds: 1200, FailurePolicy: "stop"},
				{ID: "docker-push", Name: "Push Image to Registry", Type: "dockerPush", TimeoutSeconds: 1200, FailurePolicy: "stop"},
				{ID: "k8s-deploy", Name: "Kubernetes Rolling Deployment", Type: "k8sDeploy", TimeoutSeconds: 900, FailurePolicy: "stop"},
				{ID: "notify", Name: "Deployment Notification", Type: "notify", TimeoutSeconds: 60, FailurePolicy: "ignore"},
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
	if category == "" || category == "All Templates" || category == "\u5168\u90e8\u6a21\u677f" {
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
		return nil, "", errors.New("pipeline stage configuration is not valid JSON")
	}
	for index := range wrapper.Stages {
		if wrapper.Stages[index].ID == "" {
			wrapper.Stages[index].ID = fmt.Sprintf("stage-%d", index+1)
		}
		if wrapper.Stages[index].Name == "" {
			wrapper.Stages[index].Name = fmt.Sprintf("Stage %d", index+1)
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

func validateOpsPipelineStages(stages []OpsAppPipelineStageDefinition, validateDeployTarget bool) error {
	if len(stages) == 0 {
		return errors.New("pipeline requires at least one execution stage")
	}
	allowedTypes := map[string]bool{
		"checkout": true, "command": true, "test": true, "build": true,
		"dockerBuild": true, "dockerPush": true, "k8sDeploy": true,
		"manual": true, "notify": true,
	}
	stageIDs := map[string]bool{}
	buildRegistryByStageID := map[string]uint{}
	buildStageByRegistryID := map[uint]string{}
	for index, stage := range stages {
		if !allowedTypes[stage.Type] {
			return fmt.Errorf("unsupported type for stage %d: %s", index+1, stage.Type)
		}
		if strings.TrimSpace(stage.ID) == "" || stageIDs[stage.ID] {
			return fmt.Errorf("stage %d has an empty or duplicate identifier", index+1)
		}
		stageIDs[stage.ID] = true
		if strings.TrimSpace(stage.Name) == "" {
			return fmt.Errorf("stage %d name is required", index+1)
		}
		if (stage.Type == "command" || stage.Type == "test" || stage.Type == "build") && strings.TrimSpace(opsPipelineConfigString(stage.Config, "script")) == "" {
			return fmt.Errorf("stage %q is missing an execution command", stage.Name)
		}
		if stage.Type == "notify" && opsPipelineConfigUint(stage.Config, "notifyRuleId") == 0 {
			return fmt.Errorf("notification stage %q requires a notification rule", stage.Name)
		}
		if stage.Type == "dockerBuild" {
			registryID := opsPipelineConfigUint(stage.Config, "registryId")
			if registryID == 0 {
				return fmt.Errorf("image-build stage %q requires an image registry", stage.Name)
			}
			buildRegistryByStageID[stage.ID] = registryID
			if _, exists := buildStageByRegistryID[registryID]; !exists {
				buildStageByRegistryID[registryID] = stage.ID
			}
		}
		if validateDeployTarget && stage.Type == "k8sDeploy" {
			if opsPipelineConfigUint(stage.Config, "clusterId") == 0 || opsPipelineConfigString(stage.Config, "namespace") == "" || opsPipelineConfigString(stage.Config, "workload") == "" || opsPipelineConfigString(stage.Config, "container") == "" {
				return fmt.Errorf("Kubernetes deployment stage %q is missing cluster, namespace, workload, or container configuration", stage.Name)
			}
		}
	}
	for index := range stages {
		stage := &stages[index]
		if stage.Type != "dockerPush" {
			continue
		}
		if stage.Config == nil {
			stage.Config = map[string]any{}
		}
		sourceStageID := opsPipelineConfigString(stage.Config, "sourceStageId")
		if sourceStageID == "" {
			// Compatibility for existing pipelines that selected the same registry in
			// both stages before source-stage binding was introduced.
			sourceStageID = buildStageByRegistryID[opsPipelineConfigUint(stage.Config, "registryId")]
			if sourceStageID != "" {
				stage.Config["sourceStageId"] = sourceStageID
			}
		}
		registryID := buildRegistryByStageID[sourceStageID]
		if sourceStageID == "" || registryID == 0 {
			return fmt.Errorf("image-push stage %q must reference a preceding image-build stage", stage.Name)
		}
		// Persist the resolved registry ID in the execution definition. This makes the
		// pushed tag exactly the image produced by the selected build stage.
		stage.Config["registryId"] = registryID
	}
	return nil
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
		return errors.New("select an application")
	}
	app, err := s.GetOpsApplication(payload.AppID)
	if err != nil {
		return err
	}
	stages, normalizedDefinition, err := normalizeOpsPipelineStages(payload.DefinitionJSON)
	if err != nil {
		return err
	}
	if err := validateOpsPipelineStages(stages, false); err != nil {
		return err
	}
	definition, _ := json.Marshal(map[string]any{"stages": stages})
	normalizedDefinition = string(definition)
	if payload.ExecutorHostID == 0 {
		return errors.New("select a pipeline executor; pipeline commands never run inside the Ops Admin container")
	}
	if _, err := s.getOpsPipelineExecutorHost(payload.ExecutorHostID); err != nil {
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
		BuildTaskID:    payload.BuildTaskID,
		ExecutorHostID: payload.ExecutorHostID,
		StageCount:     len(stages),
		Status:         normalizeOpsAppStatus(payload.Status),
		Description:    Trimmed(payload.Description),
		DefinitionJSON: normalizedDefinition,
	}
	if item.Name == "" {
		return errors.New("pipeline name is required")
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
		"env": item.Env, "tech_stack": item.TechStack, "template_id": item.TemplateID, "build_task_id": item.BuildTaskID, "executor_host_id": item.ExecutorHostID,
		"stage_count": item.StageCount, "status": item.Status, "description": item.Description,
		"definition_json": item.DefinitionJSON,
	}).Error
}

func (s *Service) UpdateOpsAppPipelineStatus(payload OpsAppPipelineStatusPayload) error {
	if payload.ID == 0 {
		return errors.New("pipeline ID is required")
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

func (s *Service) getOpsPipelineExecutorHost(id uint) (model.AssetHost, error) {
	if id == 0 {
		return model.AssetHost{}, errors.New("select a pipeline executor")
	}
	var host model.AssetHost
	if err := s.db.Preload("Group").Preload("Credential").Preload("Gateway").Preload("Gateway.Credential").First(&host, id).Error; err != nil {
		return model.AssetHost{}, errors.New("pipeline executor does not exist")
	}
	if host.Status != 1 {
		return model.AssetHost{}, errors.New("selected pipeline executor is disabled")
	}
	if host.CredentialID == nil || *host.CredentialID == 0 {
		return model.AssetHost{}, errors.New("selected pipeline executor has no SSH credential configured")
	}
	return host, nil
}

func (s *Service) RunOpsAppPipeline(payload OpsAppPipelineRunPayload) (map[string]any, error) {
	var pipeline model.OpsAppPipeline
	if err := s.db.First(&pipeline, payload.PipelineID).Error; err != nil {
		return nil, err
	}
	if pipeline.Status != 1 {
		return nil, errors.New("the current pipeline is disabled and cannot be executed")
	}
	executorHost, err := s.getOpsPipelineExecutorHost(pipeline.ExecutorHostID)
	if err != nil {
		return nil, err
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
	var bindingCount int64
	_ = s.db.Model(&model.OpsApplicationEnvironmentBinding{}).Where("app_id = ?", pipeline.AppID).Count(&bindingCount).Error
	if bindingCount > 0 {
		var binding model.OpsApplicationEnvironmentBinding
		if err := s.db.Where("app_id = ? AND env = ? AND status = ?", pipeline.AppID, normalizeEnvCode(env), 1).First(&binding).Error; err != nil {
			return nil, errors.New("the application has no resource binding for the selected environment")
		}
		for index := range stages {
			if stages[index].Type != "k8sDeploy" {
				continue
			}
			if stages[index].Config == nil {
				stages[index].Config = map[string]any{}
			}
			// clusterId 폴백(binding.K8sClusterID)은 I-b 칼럼 drop(C66)으로
			// 제거됐다 — k8sDeploy 스테이지는 스테이지 Config에 clusterId를
			// 명시해야 한다(런타임 검증 :1076).
			if opsPipelineConfigString(stages[index].Config, "namespace") == "" {
				stages[index].Config["namespace"] = binding.Namespace
			}
			if opsPipelineConfigString(stages[index].Config, "workloadType") == "" {
				stages[index].Config["workloadType"] = binding.WorkloadType
			}
			if opsPipelineConfigString(stages[index].Config, "workload") == "" {
				stages[index].Config["workload"] = binding.WorkloadName
			}
		}
		definition, _ := json.Marshal(map[string]any{"stages": stages})
		normalizedDefinition = string(definition)
	}
	if err := validateOpsPipelineStages(stages, true); err != nil {
		return nil, err
	}
	definition, _ := json.Marshal(map[string]any{"stages": stages})
	normalizedDefinition = string(definition)
	if strings.EqualFold(env, "prod") || strings.Contains(strings.ToLower(env), "production") || strings.Contains(env, "\u751f\u4ea7") {
		hasApproval := false
		for _, stage := range stages {
			if stage.Type == "manual" {
				hasApproval = true
				break
			}
		}
		if !hasApproval {
			return nil, errors.New("production pipelines must include a manual approval stage")
		}
	}
	if pipeline.BuildTaskID > 0 && payload.ArtifactID == 0 {
		return nil, errors.New("the pipeline is linked to a build task; select a successfully generated artifact")
	}
	imageTag := Trimmed(payload.ImageTag)
	var artifact model.OpsAppArtifact
	if payload.ArtifactID > 0 {
		artifactQuery := s.db.Where("id = ? AND app_id = ? AND status = ?", payload.ArtifactID, pipeline.AppID, "ready")
		if pipeline.BuildTaskID > 0 {
			artifactQuery = artifactQuery.Where("build_task_id = ?", pipeline.BuildTaskID)
		}
		if normalizedEnv := normalizeEnvCode(env); normalizedEnv != "" {
			artifactQuery = artifactQuery.Where("env = ?", normalizedEnv)
		}
		if err := artifactQuery.First(&artifact).Error; err != nil {
			return nil, errors.New("selected artifact does not exist, is unavailable, or does not belong to the current application")
		}
		if imageTag == "" {
			imageTag = artifact.Version
		}
	}
	if imageTag == "" {
		imageTag = opsPipelineImageTag(branch, time.Now())
	}
	workspace := s.resolveOpsAppWorkspace(*app)
	paramsJSON, _ := json.Marshal(payload.Params)
	now := time.Now()
	run := model.OpsAppPipelineRun{
		PipelineID: pipeline.ID, PipelineName: pipeline.Name, AppID: pipeline.AppID, AppName: pipeline.AppName,
		AppCode: pipeline.AppCode, Env: env, Branch: branch, ImageTag: imageTag, ArtifactID: payload.ArtifactID, TriggerType: "manual",
		ExecutorHostID: pipeline.ExecutorHostID, TriggerUser: "System Administrator", Status: "running", Summary: "pipeline created and awaiting stage execution",
		ParamsJSON: string(paramsJSON), DefinitionJSON: normalizedDefinition, StartedAt: &now,
	}
	if err := s.db.Create(&run).Error; err != nil {
		return nil, err
	}
	for _, stage := range stages {
		_ = s.db.Create(&model.OpsAppPipelineRunStage{
			RunID: run.ID, StageID: stage.ID, StageName: stage.Name, StageType: stage.Type,
			Status: "waiting", Summary: "Waiting",
		}).Error
	}
	_ = s.db.Model(&model.OpsAppPipeline{}).Where("id = ?", pipeline.ID).Updates(map[string]any{
		"last_run_id": run.ID, "last_status": "running", "last_run_at": &now,
	})
	go s.runOpsAppPipeline(opsPipelineExecution{
		RunID: run.ID, PipelineID: pipeline.ID, App: *app, Branch: branch, Env: env,
		ImageTag: imageTag, Workspace: workspace, ExecutorHost: executorHost, Params: payload.Params, Stages: stages, StartedAt: now,
	})
	return map[string]any{"runId": run.ID, "status": run.Status, "summary": run.Summary}, nil
}
