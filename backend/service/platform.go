package service

import (
	"errors"
	"fmt"
	"strings"

	"ops-admin/backend/model"
)

type OpsEnvironmentPayload struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Code        string `json:"code"`
	Sort        int    `json:"sort"`
	Status      int    `json:"status"`
	Description string `json:"description"`
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
	var count int64
	if err := s.db.Model(&model.OpsEnvironment{}).Count(&count).Error; err != nil || count > 0 {
		return
	}

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
			s.appendMonitorAlertTimeline(event.ID, "action_failed", "联动处置触发失败", err.Error(), action.Operator, map[string]any{"actionType": actionType, "targetId": payload.TargetID})
			return nil, err
		}
		action.Result = "已触发作业执行"
	} else {
		action.Result = "已记录诊断处置动作，可在快速执行或作业中继续处理"
	}
	if err := s.db.Create(&action).Error; err != nil {
		return nil, err
	}
	s.appendMonitorAlertTimeline(event.ID, "action", "已触发联动处置", firstNonEmpty(action.Summary, action.Result), action.Operator, map[string]any{
		"actionType": action.ActionType, "targetId": action.TargetID, "targetName": action.TargetName,
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
