package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"ops-admin/backend/model"
)

type OpsJobPayload struct {
	ID             uint   `json:"id"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	Status         int    `json:"status"`
	TemplateID     uint   `json:"templateId"`
	NotifyEnabled  bool   `json:"notifyEnabled"`
	NotifyRuleID   uint   `json:"notifyRuleId"`
	GraphJSON      string `json:"graphJson"`
	DefinitionJSON string `json:"definitionJson"`
}

type OpsJobTemplatePayload struct {
	ID             uint   `json:"id"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	Status         int    `json:"status"`
	GraphJSON      string `json:"graphJson"`
	DefinitionJSON string `json:"definitionJson"`
}

type OpsJobStatusPayload struct {
	ID     uint `json:"id"`
	Status int  `json:"status"`
}

type OpsJobHistoryApprovalPayload struct {
	HistoryID uint   `json:"historyId"`
	StepID    string `json:"stepId"`
	Note      string `json:"note"`
}

type OpsJobDefinition struct {
	Nodes []OpsJobNode `json:"nodes"`
	Edges []OpsJobEdge `json:"edges"`
}

type OpsJobNode struct {
	ID     string         `json:"id"`
	Type   string         `json:"type"`
	Label  string         `json:"label"`
	Config map[string]any `json:"config"`
	Meta   map[string]any `json:"meta"`
}

type OpsJobEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

func normalizeJobStatus(value int) int {
	if value == 2 {
		return 2
	}
	return 1
}

func normalizeJobNodeType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "file":
		return "file"
	case "approval":
		return "approval"
	case "notify":
		return "notify"
	default:
		return "script"
	}
}

func parseOpsJobDefinition(raw string) (*OpsJobDefinition, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, errors.New("job orchestration definition is required")
	}
	var definition OpsJobDefinition
	if err := json.Unmarshal([]byte(value), &definition); err != nil {
		return nil, errors.New("invalid job orchestration definition format")
	}
	if len(definition.Nodes) == 0 {
		return nil, errors.New("add at least one job step")
	}
	for index := range definition.Nodes {
		definition.Nodes[index].Type = normalizeJobNodeType(definition.Nodes[index].Type)
		if strings.TrimSpace(definition.Nodes[index].ID) == "" {
			return nil, errors.New("a job step is missing its ID")
		}
		if strings.TrimSpace(definition.Nodes[index].Label) == "" {
			definition.Nodes[index].Label = fmt.Sprintf("Step %d", index+1)
		}
		if definition.Nodes[index].Config == nil {
			definition.Nodes[index].Config = map[string]any{}
		}
	}
	return &definition, nil
}

func stringConfigMap(value any) map[string]string {
	result := map[string]string{}
	values, ok := value.(map[string]any)
	if !ok {
		return result
	}
	for key, raw := range values {
		if text, ok := raw.(string); ok {
			result[key] = text
		}
	}
	return result
}

func (s *Service) normalizeOpsJobDefinitionVariables(raw, existingRaw string) (string, error) {
	definition, err := parseOpsJobDefinition(raw)
	if err != nil {
		return "", err
	}
	existingNodes := map[string]OpsJobNode{}
	if strings.TrimSpace(existingRaw) != "" {
		if existing, parseErr := parseOpsJobDefinition(existingRaw); parseErr == nil {
			for _, node := range existing.Nodes {
				existingNodes[node.ID] = node
			}
		}
	}
	for index := range definition.Nodes {
		node := &definition.Nodes[index]
		if node.Type != "script" {
			continue
		}
		scriptID := uint(numberConfig(node.Config, "scriptId"))
		if scriptID == 0 {
			continue
		}
		script, err := s.GetOpsScript(scriptID)
		if err != nil {
			return "", err
		}
		var existingValues model.OpsScriptVariableValues
		if existing, ok := existingNodes[node.ID]; ok {
			existingValues = model.OpsScriptVariableValues(stringConfigMap(existing.Config["variables"]))
		}
		stored, _, err := resolveScheduleScriptVariables(script, stringConfigMap(node.Config["variables"]), existingValues)
		if err != nil {
			return "", fmt.Errorf("step %q: %w", node.Label, err)
		}
		delete(node.Config, "parameters")
		node.Config["variables"] = map[string]string(stored)
	}
	data, err := json.Marshal(definition)
	return string(data), err
}

func (s *Service) jobDefinitionForView(raw string) string {
	definition, err := parseOpsJobDefinition(raw)
	if err != nil {
		return raw
	}
	for index := range definition.Nodes {
		node := &definition.Nodes[index]
		if node.Type != "script" || uint(numberConfig(node.Config, "scriptId")) == 0 {
			continue
		}
		if script, err := s.GetOpsScript(uint(numberConfig(node.Config, "scriptId"))); err == nil {
			node.Config["variables"] = scheduleVariableResponse(script, model.OpsScriptVariableValues(stringConfigMap(node.Config["variables"])))
		}
		delete(node.Config, "parameters")
	}
	data, err := json.Marshal(definition)
	if err != nil {
		return raw
	}
	return string(data)
}

func (s *Service) ListOpsJobs(pageNum, pageSize int, keyword, status string) (map[string]any, error) {
	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	query := s.db.Model(&model.OpsJob{})
	if strings.TrimSpace(keyword) != "" {
		like := "%" + strings.TrimSpace(keyword) + "%"
		query = query.Where("name LIKE ? OR description LIKE ?", like, like)
	}
	if strings.TrimSpace(status) != "" {
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var list []model.OpsJob
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

func (s *Service) GetOpsJob(id uint) (*model.OpsJob, error) {
	var item model.OpsJob
	if err := s.db.First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Service) GetOpsJobForView(id uint) (*model.OpsJob, error) {
	item, err := s.GetOpsJob(id)
	if err != nil {
		return nil, err
	}
	item.DefinitionJSON = s.jobDefinitionForView(item.DefinitionJSON)
	return item, nil
}

func (s *Service) CreateOpsJob(payload OpsJobPayload) error {
	if strings.TrimSpace(payload.Name) == "" {
		return errors.New("job name is required")
	}
	definitionJSON, err := s.normalizeOpsJobDefinitionVariables(payload.DefinitionJSON, "")
	if err != nil {
		return err
	}
	item := model.OpsJob{
		Name:           Trimmed(payload.Name),
		Description:    Trimmed(payload.Description),
		Status:         normalizeJobStatus(payload.Status),
		TemplateID:     payload.TemplateID,
		NotifyEnabled:  payload.NotifyEnabled,
		NotifyRuleID:   payload.NotifyRuleID,
		GraphJSON:      strings.TrimSpace(payload.GraphJSON),
		DefinitionJSON: definitionJSON,
	}
	return s.db.Create(&item).Error
}

func (s *Service) UpdateOpsJob(payload OpsJobPayload) error {
	if payload.ID == 0 {
		return errors.New("job ID is required")
	}
	if strings.TrimSpace(payload.Name) == "" {
		return errors.New("job name is required")
	}
	var existing model.OpsJob
	if err := s.db.First(&existing, payload.ID).Error; err != nil {
		return err
	}
	definitionJSON, err := s.normalizeOpsJobDefinitionVariables(payload.DefinitionJSON, existing.DefinitionJSON)
	if err != nil {
		return err
	}
	return s.db.Model(&model.OpsJob{}).Where("id = ?", payload.ID).Updates(map[string]any{
		"name":            Trimmed(payload.Name),
		"description":     Trimmed(payload.Description),
		"status":          normalizeJobStatus(payload.Status),
		"template_id":     payload.TemplateID,
		"notify_enabled":  payload.NotifyEnabled,
		"notify_rule_id":  payload.NotifyRuleID,
		"graph_json":      strings.TrimSpace(payload.GraphJSON),
		"definition_json": definitionJSON,
	}).Error
}

func (s *Service) DeleteOpsJob(id uint) error {
	return s.db.Delete(&model.OpsJob{}, id).Error
}

func (s *Service) UpdateOpsJobStatus(payload OpsJobStatusPayload) error {
	if payload.ID == 0 {
		return errors.New("job ID is required")
	}
	return s.db.Model(&model.OpsJob{}).Where("id = ?", payload.ID).Update("status", normalizeJobStatus(payload.Status)).Error
}

func (s *Service) ListOpsJobTemplates(pageNum, pageSize int, keyword, status string) (map[string]any, error) {
	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	query := s.db.Model(&model.OpsJobTemplate{})
	if strings.TrimSpace(keyword) != "" {
		like := "%" + strings.TrimSpace(keyword) + "%"
		query = query.Where("name LIKE ? OR description LIKE ?", like, like)
	}
	if strings.TrimSpace(status) != "" {
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var list []model.OpsJobTemplate
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

func (s *Service) ListOpsJobTemplateOptions() ([]model.OpsJobTemplate, error) {
	var list []model.OpsJobTemplate
	if err := s.db.Where("status = ?", 1).Order("id DESC").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

func (s *Service) GetOpsJobTemplate(id uint) (*model.OpsJobTemplate, error) {
	var item model.OpsJobTemplate
	if err := s.db.First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Service) GetOpsJobTemplateForView(id uint) (*model.OpsJobTemplate, error) {
	item, err := s.GetOpsJobTemplate(id)
	if err != nil {
		return nil, err
	}
	item.DefinitionJSON = s.jobDefinitionForView(item.DefinitionJSON)
	return item, nil
}

func (s *Service) CreateOpsJobTemplate(payload OpsJobTemplatePayload) error {
	if strings.TrimSpace(payload.Name) == "" {
		return errors.New("job template name is required")
	}
	definitionJSON, err := s.normalizeOpsJobDefinitionVariables(payload.DefinitionJSON, "")
	if err != nil {
		return err
	}
	item := model.OpsJobTemplate{
		Name:           Trimmed(payload.Name),
		Description:    Trimmed(payload.Description),
		Status:         normalizeJobStatus(payload.Status),
		GraphJSON:      strings.TrimSpace(payload.GraphJSON),
		DefinitionJSON: definitionJSON,
	}
	return s.db.Create(&item).Error
}

func (s *Service) UpdateOpsJobTemplate(payload OpsJobTemplatePayload) error {
	if payload.ID == 0 {
		return errors.New("job template ID is required")
	}
	if strings.TrimSpace(payload.Name) == "" {
		return errors.New("job template name is required")
	}
	var existing model.OpsJobTemplate
	if err := s.db.First(&existing, payload.ID).Error; err != nil {
		return err
	}
	definitionJSON, err := s.normalizeOpsJobDefinitionVariables(payload.DefinitionJSON, existing.DefinitionJSON)
	if err != nil {
		return err
	}
	return s.db.Model(&model.OpsJobTemplate{}).Where("id = ?", payload.ID).Updates(map[string]any{
		"name":            Trimmed(payload.Name),
		"description":     Trimmed(payload.Description),
		"status":          normalizeJobStatus(payload.Status),
		"graph_json":      strings.TrimSpace(payload.GraphJSON),
		"definition_json": definitionJSON,
	}).Error
}

func (s *Service) DeleteOpsJobTemplate(id uint) error {
	return s.db.Delete(&model.OpsJobTemplate{}, id).Error
}

func (s *Service) UpdateOpsJobTemplateStatus(payload OpsJobStatusPayload) error {
	if payload.ID == 0 {
		return errors.New("job template ID is required")
	}
	return s.db.Model(&model.OpsJobTemplate{}).Where("id = ?", payload.ID).Update("status", normalizeJobStatus(payload.Status)).Error
}

func (s *Service) ListOpsJobHistories(pageNum, pageSize int, keyword, status string) (map[string]any, error) {
	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	query := s.db.Model(&model.OpsJobHistory{})
	if strings.TrimSpace(keyword) != "" {
		like := "%" + strings.TrimSpace(keyword) + "%"
		query = query.Where("job_name LIKE ? OR summary LIKE ? OR current_step_name LIKE ?", like, like, like)
	}
	if strings.TrimSpace(status) != "" {
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var list []model.OpsJobHistory
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

func (s *Service) GetOpsJobHistoryDetail(id uint) (map[string]any, error) {
	var history model.OpsJobHistory
	if err := s.db.First(&history, id).Error; err != nil {
		return nil, err
	}
	var steps []model.OpsJobHistoryStep
	if err := s.db.Where("history_id = ?", id).Order("id ASC").Find(&steps).Error; err != nil {
		return nil, err
	}
	return map[string]any{
		"history": history,
		"steps":   steps,
	}, nil
}

func (s *Service) RunOpsJob(id uint) error {
	job, err := s.GetOpsJob(id)
	if err != nil {
		return err
	}
	if job.Status != 1 {
		return errors.New("the current job is disabled and cannot be executed")
	}
	definition, err := parseOpsJobDefinition(job.DefinitionJSON)
	if err != nil {
		return err
	}
	startedAt := time.Now()
	history := model.OpsJobHistory{
		JobID:          job.ID,
		JobName:        job.Name,
		TriggerType:    "manual",
		Status:         "running",
		Summary:        "job triggered and running",
		DefinitionJSON: job.DefinitionJSON,
		StartedAt:      &startedAt,
	}
	if err := s.db.Create(&history).Error; err != nil {
		return err
	}
	go s.runOpsJobDefinition(history.ID, *definition)
	return nil
}

func (s *Service) ApproveOpsJobHistoryStep(payload OpsJobHistoryApprovalPayload) error {
	var history model.OpsJobHistory
	if err := s.db.First(&history, payload.HistoryID).Error; err != nil {
		return err
	}
	if history.Status != "waiting_approval" {
		return errors.New("the current job is not awaiting approval")
	}
	var step model.OpsJobHistoryStep
	if err := s.db.Where("history_id = ? AND step_id = ?", payload.HistoryID, payload.StepID).First(&step).Error; err != nil {
		return err
	}
	if step.Status != "waiting_approval" {
		return errors.New("the current step is not awaiting approval")
	}
	finishedAt := time.Now()
	duration := int64(0)
	if step.StartedAt != nil {
		duration = finishedAt.Sub(*step.StartedAt).Milliseconds()
	}
	if err := s.db.Model(&model.OpsJobHistoryStep{}).Where("id = ?", step.ID).Updates(map[string]any{
		"status":            "success",
		"summary":           firstNonEmpty(step.Summary, "manual approval granted"),
		"approval_decision": "approved",
		"approval_note":     strings.TrimSpace(payload.Note),
		"finished_at":       &finishedAt,
		"duration_ms":       duration,
	}).Error; err != nil {
		return err
	}
	var definition OpsJobDefinition
	if err := json.Unmarshal([]byte(history.DefinitionJSON), &definition); err != nil {
		return errors.New("job definition is invalid and execution cannot continue")
	}
	go s.resumeOpsJobDefinition(payload.HistoryID, definition, payload.StepID)
	return nil
}

func (s *Service) RejectOpsJobHistoryStep(payload OpsJobHistoryApprovalPayload) error {
	var history model.OpsJobHistory
	if err := s.db.First(&history, payload.HistoryID).Error; err != nil {
		return err
	}
	var step model.OpsJobHistoryStep
	if err := s.db.Where("history_id = ? AND step_id = ?", payload.HistoryID, payload.StepID).First(&step).Error; err != nil {
		return err
	}
	finishedAt := time.Now()
	duration := int64(0)
	if step.StartedAt != nil {
		duration = finishedAt.Sub(*step.StartedAt).Milliseconds()
	}
	if err := s.db.Model(&model.OpsJobHistoryStep{}).Where("id = ?", step.ID).Updates(map[string]any{
		"status":            "rejected",
		"summary":           "manual approval rejected",
		"approval_decision": "rejected",
		"approval_note":     strings.TrimSpace(payload.Note),
		"finished_at":       &finishedAt,
		"duration_ms":       duration,
	}).Error; err != nil {
		return err
	}
	return s.db.Model(&model.OpsJobHistory{}).Where("id = ?", payload.HistoryID).Updates(map[string]any{
		"status":            "rejected",
		"summary":           "job was rejected at a manual approval step",
		"current_step_id":   step.StepID,
		"current_step_name": step.StepName,
		"finished_at":       &finishedAt,
	}).Error
}
