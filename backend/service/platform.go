package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"ops-admin/backend/model"

	"gorm.io/gorm"
)

type OpsEnvironmentPayload struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Code        string `json:"code"`
	Sort        int    `json:"sort"`
	Status      int    `json:"status"`
	Description string `json:"description"`
}

type OpsChangeRecordPayload struct {
	Title      string
	ChangeType string
	SourceType string
	SourceID   uint
	SourceName string
	AppID      uint
	AppName    string
	AppCode    string
	Env        string
	RiskLevel  string
	Status     string
	Operator   string
	Summary    string
	Detail     string
	StartedAt  *time.Time
	FinishedAt *time.Time
}

type MonitorAlertActionPayload struct {
	ID         uint   `json:"id"`
	ActionType string `json:"actionType"`
	TargetID   uint   `json:"targetId"`
	TargetName string `json:"targetName"`
	Operator   string `json:"operator"`
	Summary    string `json:"summary"`
	ClaimedBy  string `json:"claimedBy"`
	HandleNote string `json:"handleNote"`
}

func (s *Service) ensureDefaultEnvironments() {
	defaults := []model.OpsEnvironment{
		{Name: "开发环境", Code: "dev", Sort: 10, Status: 1, Description: "开发联调与日常验证环境"},
		{Name: "测试环境", Code: "test", Sort: 20, Status: 1, Description: "功能测试、集成测试与预发布验证环境"},
		{Name: "生产环境", Code: "prod", Sort: 30, Status: 1, Description: "正式对外服务环境"},
	}
	for _, item := range defaults {
		var count int64
		if err := s.db.Model(&model.OpsEnvironment{}).Where("code = ?", item.Code).Count(&count).Error; err == nil && count == 0 {
			_ = s.db.Create(&item).Error
		}
	}
}

func (s *Service) ListOpsEnvironments(keyword, status string) ([]model.OpsEnvironment, error) {
	s.ensureDefaultEnvironments()
	query := s.db.Model(&model.OpsEnvironment{})
	if keyword != "" {
		query = query.Where("name LIKE ? OR code LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	var list []model.OpsEnvironment
	if err := query.Order("sort asc, id asc").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

func (s *Service) SaveOpsEnvironment(payload OpsEnvironmentPayload) error {
	item := model.OpsEnvironment{
		Name:        Trimmed(payload.Name),
		Code:        strings.ToLower(Trimmed(payload.Code)),
		Sort:        payload.Sort,
		Status:      payload.Status,
		Description: Trimmed(payload.Description),
	}
	if item.Name == "" || item.Code == "" {
		return errors.New("环境名称和环境标识不能为空")
	}
	if item.Status == 0 {
		item.Status = 1
	}
	if payload.ID > 0 {
		var existing model.OpsEnvironment
		if err := s.db.First(&existing, payload.ID).Error; err != nil {
			return err
		}
		if existing.Code != item.Code {
			return errors.New("环境标识创建后不可修改，请新建环境并迁移资源")
		}
		return s.db.Model(&model.OpsEnvironment{}).Where("id = ?", payload.ID).Updates(item).Error
	}
	return s.db.Create(&item).Error
}

func (s *Service) DeleteOpsEnvironment(id uint) error {
	if id == 0 {
		return errors.New("请选择要删除的环境")
	}
	var environment model.OpsEnvironment
	if err := s.db.First(&environment, id).Error; err != nil {
		return err
	}
	references := []struct {
		name   string
		entity any
		field  string
	}{
		{name: "应用", entity: &model.OpsApplication{}, field: "env"},
		{name: "主机", entity: &model.AssetHost{}, field: "environment"},
		{name: "数据库", entity: &model.AssetDatabase{}, field: "env"},
		{name: "K8s 集群", entity: &model.K8sCluster{}, field: "env"},
		{name: "监控数据源", entity: &model.MonitorDatasource{}, field: "env"},
		{name: "告警规则", entity: &model.MonitorAlertRule{}, field: "env"},
	}
	for _, reference := range references {
		var count int64
		if err := s.db.Model(reference.entity).Where(reference.field+" = ?", environment.Code).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return fmt.Errorf("环境“%s”仍被 %d 个%s引用，请先迁移相关资源", environment.Name, count, reference.name)
		}
	}
	return s.db.Delete(&model.OpsEnvironment{}, id).Error
}

func (s *Service) CreateChangeRecord(payload OpsChangeRecordPayload) {
	if Trimmed(payload.Title) == "" {
		payload.Title = payload.SourceName
	}
	if Trimmed(payload.Title) == "" {
		payload.Title = "平台变更"
	}
	if Trimmed(payload.Status) == "" {
		payload.Status = "success"
	}
	if Trimmed(payload.RiskLevel) == "" {
		payload.RiskLevel = "medium"
	}
	now := time.Now()
	if payload.StartedAt == nil {
		payload.StartedAt = &now
	}
	if payload.FinishedAt == nil {
		payload.FinishedAt = &now
	}
	record := model.OpsChangeRecord{
		Title:      Trimmed(payload.Title),
		ChangeType: Trimmed(payload.ChangeType),
		SourceType: Trimmed(payload.SourceType),
		SourceID:   payload.SourceID,
		SourceName: Trimmed(payload.SourceName),
		AppID:      payload.AppID,
		AppName:    Trimmed(payload.AppName),
		AppCode:    Trimmed(payload.AppCode),
		Env:        normalizeEnvCode(payload.Env),
		RiskLevel:  Trimmed(payload.RiskLevel),
		Status:     Trimmed(payload.Status),
		Operator:   Trimmed(payload.Operator),
		Summary:    Trimmed(payload.Summary),
		Detail:     payload.Detail,
		StartedAt:  payload.StartedAt,
		FinishedAt: payload.FinishedAt,
	}
	_ = s.db.Create(&record).Error
}

func (s *Service) ListChangeRecords(pageNum, pageSize int, keyword, env, changeType, status string) (map[string]any, error) {
	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	query := s.db.Model(&model.OpsChangeRecord{})
	if keyword != "" {
		query = query.Where("title LIKE ? OR source_name LIKE ? OR app_name LIKE ? OR summary LIKE ?", "%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
	}
	if env != "" {
		query = query.Where("env = ?", normalizeEnvCode(env))
	}
	if changeType != "" {
		query = query.Where("change_type = ?", changeType)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var list []model.OpsChangeRecord
	if err := query.Order("id desc").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, err
	}
	var allTotal, failed, highRisk int64
	_ = s.db.Model(&model.OpsChangeRecord{}).Count(&allTotal)
	_ = s.db.Model(&model.OpsChangeRecord{}).Where("status <> ?", "success").Count(&failed)
	_ = s.db.Model(&model.OpsChangeRecord{}).Where("risk_level IN ?", []string{"high", "critical"}).Count(&highRisk)
	stats := map[string]any{"total": allTotal, "failed": failed, "highRisk": highRisk}
	return map[string]any{"list": list, "total": total, "pageNum": pageNum, "pageSize": pageSize, "stats": stats}, nil
}

func (s *Service) GetApplicationTopology(appID uint, env string) (map[string]any, error) {
	normalizedEnv := normalizeEnvCode(env)
	var app model.OpsApplication
	appQuery := s.db.Model(&model.OpsApplication{})
	if appID > 0 {
		appQuery = appQuery.Where("id = ?", appID)
	} else if normalizedEnv != "" {
		appQuery = appQuery.Where("env = ? OR env = ''", normalizedEnv)
	}
	if err := appQuery.Order("id asc").First(&app).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	hostQuery := s.db.Model(&model.AssetHost{})
	databaseQuery := s.db.Model(&model.AssetDatabase{})
	k8sQuery := s.db.Model(&model.K8sCluster{})
	alertQuery := s.db.Model(&model.MonitorAlertEvent{})
	releaseQuery := s.db.Model(&model.OpsAppRelease{})
	pipelineQuery := s.db.Model(&model.OpsAppPipelineRun{})
	if normalizedEnv != "" {
		hostQuery = hostQuery.Where("environment = ? OR environment = ''", normalizedEnv)
		databaseQuery = databaseQuery.Where("env = ? OR env = ''", normalizedEnv)
		k8sQuery = k8sQuery.Where("env = ? OR env = ''", normalizedEnv)
		releaseQuery = releaseQuery.Where("env = ?", normalizedEnv)
		pipelineQuery = pipelineQuery.Where("env = ?", normalizedEnv)
	}
	if app.ID > 0 {
		releaseQuery = releaseQuery.Where("app_id = ?", app.ID)
		pipelineQuery = pipelineQuery.Where("app_id = ?", app.ID)
		if app.Code != "" {
			alertQuery = alertQuery.Where("rule_name LIKE ? OR labels_json LIKE ? OR annotations_json LIKE ? OR summary LIKE ?", "%"+app.Code+"%", "%"+app.Code+"%", "%"+app.Code+"%", "%"+app.Code+"%")
		}
	}
	var hosts []model.AssetHost
	var databases []model.AssetDatabase
	var clusters []model.K8sCluster
	var alerts []model.MonitorAlertEvent
	var releases []model.OpsAppRelease
	var pipelineRuns []model.OpsAppPipelineRun
	if err := hostQuery.Order("id desc").Limit(20).Find(&hosts).Error; err != nil {
		return nil, err
	}
	if err := databaseQuery.Order("id desc").Limit(20).Find(&databases).Error; err != nil {
		return nil, err
	}
	if err := k8sQuery.Order("id desc").Limit(20).Find(&clusters).Error; err != nil {
		return nil, err
	}
	if err := alertQuery.Order("id desc").Limit(20).Find(&alerts).Error; err != nil {
		return nil, err
	}
	if err := releaseQuery.Order("id desc").Limit(10).Find(&releases).Error; err != nil {
		return nil, err
	}
	if err := pipelineQuery.Order("id desc").Limit(10).Find(&pipelineRuns).Error; err != nil {
		return nil, err
	}
	return map[string]any{
		"app":          app,
		"env":          normalizedEnv,
		"hosts":        hosts,
		"databases":    databases,
		"k8sClusters":  clusters,
		"alerts":       alerts,
		"releases":     releases,
		"pipelineRuns": pipelineRuns,
		"summary": map[string]int{
			"hosts":        len(hosts),
			"databases":    len(databases),
			"k8sClusters":  len(clusters),
			"alerts":       len(alerts),
			"releases":     len(releases),
			"pipelineRuns": len(pipelineRuns),
		},
	}, nil
}

func (s *Service) TriggerMonitorAlertAction(payload MonitorAlertActionPayload) (map[string]any, error) {
	var event model.MonitorAlertEvent
	if err := s.db.First(&event, payload.ID).Error; err != nil {
		return nil, err
	}
	actionType := Trimmed(payload.ActionType)
	if actionType == "" {
		actionType = "script"
	}
	action := model.MonitorAlertAction{
		AlertEventID: event.ID,
		RuleName:     event.RuleName,
		ActionType:   actionType,
		TargetID:     payload.TargetID,
		TargetName:   Trimmed(payload.TargetName),
		Status:       "success",
		Operator:     firstNonEmpty(payload.Operator, payload.ClaimedBy),
		Summary:      firstNonEmpty(payload.Summary, payload.HandleNote),
	}
	if actionType == "job" {
		if payload.TargetID == 0 {
			return nil, errors.New("请选择要触发的作业")
		}
		if err := s.RunOpsJob(payload.TargetID); err != nil {
			action.Status = "failed"
			action.Result = err.Error()
			_ = s.db.Create(&action).Error
			return nil, err
		}
		action.Result = "已触发作业执行"
	} else {
		action.Result = "已记录诊断处置动作，可在快速执行或作业中继续处理"
	}
	if err := s.db.Create(&action).Error; err != nil {
		return nil, err
	}
	detail, _ := json.Marshal(map[string]any{"alertEventId": event.ID, "actionType": action.ActionType, "targetId": action.TargetID, "targetName": action.TargetName})
	s.CreateChangeRecord(OpsChangeRecordPayload{
		Title:      "告警联动处置：" + event.RuleName,
		ChangeType: "alert_action",
		SourceType: "monitor_alert",
		SourceID:   event.ID,
		SourceName: event.RuleName,
		RiskLevel:  "medium",
		Status:     action.Status,
		Operator:   action.Operator,
		Summary:    action.Result,
		Detail:     string(detail),
	})
	return map[string]any{"action": action}, nil
}

func normalizeEnvCode(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "开发", "开发环境":
		return "dev"
	case "测试", "测试环境":
		return "test"
	case "生产", "生产环境":
		return "prod"
	default:
		return value
	}
}
