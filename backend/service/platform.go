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
		{Name: "Development", Code: "dev", Sort: 10, Status: 1, Description: "Development integration and routine validation environment"},
		{Name: "Test", Code: "test", Sort: 20, Status: 1, Description: "Functional, integration, and pre-release validation environment"},
		{Name: "Production", Code: "prod", Sort: 30, Status: 1, Description: "Production environment serving external traffic"},
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
		return errors.New("environment name and code are required")
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
			return errors.New("environment code cannot be changed after creation; create a new environment and migrate resources")
		}
		return s.db.Model(&model.OpsEnvironment{}).Where("id = ?", payload.ID).Updates(item).Error
	}
	return s.db.Create(&item).Error
}

func (s *Service) DeleteOpsEnvironment(id uint) error {
	if id == 0 {
		return errors.New("select an environment to delete")
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
		{name: "application", entity: &model.OpsApplication{}, field: "env"},
		{name: "host", entity: &model.AssetHost{}, field: "environment"},
		{name: "database", entity: &model.AssetDatabase{}, field: "env"},
		{name: "Kubernetes cluster", entity: &model.K8sCluster{}, field: "env"},
		{name: "monitoring datasource", entity: &model.MonitorDatasource{}, field: "env"},
		{name: "alert rule", entity: &model.MonitorAlertRule{}, field: "env"},
	}
	for _, reference := range references {
		var count int64
		if err := s.db.Model(reference.entity).Where(reference.field+" = ?", environment.Code).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return fmt.Errorf("environment %q is still referenced by %d %s resource(s); migrate them first", environment.Name, count, reference.name)
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
			return nil, errors.New("select a job to trigger")
		}
		if err := s.RunOpsJob(payload.TargetID); err != nil {
			action.Status = "failed"
			action.Result = err.Error()
			_ = s.db.Create(&action).Error
			s.appendMonitorAlertTimeline(event.ID, "action_failed", "linked remediation trigger failed", err.Error(), action.Operator, map[string]any{"actionType": actionType, "targetId": payload.TargetID})
			return nil, err
		}
		action.Result = "job execution triggered"
	} else {
		action.Result = "diagnostic remediation action recorded; continue from Quick Execution or Jobs"
	}
	if err := s.db.Create(&action).Error; err != nil {
		return nil, err
	}
	s.appendMonitorAlertTimeline(event.ID, "action", "linked remediation triggered", firstNonEmpty(action.Summary, action.Result), action.Operator, map[string]any{
		"actionType": action.ActionType, "targetId": action.TargetID, "targetName": action.TargetName,
	})
	return map[string]any{"action": action}, nil
}

func normalizeEnvCode(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "dev", "development", "\u5f00\u53d1", "\u5f00\u53d1\u73af\u5883":
		return "dev"
	case "test", "testing", "\u6d4b\u8bd5", "\u6d4b\u8bd5\u73af\u5883":
		return "test"
	case "prod", "production", "\u751f\u4ea7", "\u751f\u4ea7\u73af\u5883":
		return "prod"
	default:
		return value
	}
}
