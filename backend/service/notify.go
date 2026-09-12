package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"ops-admin/backend/model"
)

const (
	notifyStatusPending  = "pending"
	notifyStatusSending  = "sending"
	notifyStatusRetrying = "retrying"
	notifyStatusSuccess  = "success"
	notifyStatusFailed   = "failed"
	defaultNotifyRetries = 3
)

var notifyTemplateVariablePattern = regexp.MustCompile(`{{\s*[^{}]+\s*}}`)

type NotifyTemplatePayload struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	ChannelType string `json:"channelType"`
	Scope       string `json:"scope"`
	Title       string `json:"title"`
	Content     string `json:"content"`
	Status      int    `json:"status"`
	Description string `json:"description"`
}

type NotifyChannelPayload struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	ChannelType string `json:"channelType"`
	WebhookURL  string `json:"webhookUrl"`
	Secret      string `json:"secret"`
	HeadersJSON string `json:"headersJson"`
	Status      int    `json:"status"`
	Description string `json:"description"`
}

type NotifyRulePayload struct {
	ID          uint     `json:"id"`
	Name        string   `json:"name"`
	Scope       string   `json:"scope"`
	Events      []string `json:"events"`
	TemplateID  uint     `json:"templateId"`
	ChannelIDs  []uint   `json:"channelIds"`
	Status      int      `json:"status"`
	Description string   `json:"description"`
}

type NotifyEvent struct {
	Scope      string
	Event      string
	TargetID   uint
	TargetName string
	Status     string
	Summary    string
	Detail     string
	StartedAt  *time.Time
	FinishedAt *time.Time
	Extra      map[string]string
}

func normalizeNotifyStatus(value int) int {
	if value == 2 {
		return 2
	}
	return 1
}

func normalizeNotifyChannelType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "dingtalk", "ding_talk", "dingding":
		return "dingtalk"
	case "wecom", "wechat", "wechat_work":
		return "wecom"
	case "feishu", "lark":
		return "feishu"
	default:
		return "webhook"
	}
}

func normalizeNotifyScope(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "schedule", "cron":
		return "schedule"
	case "job":
		return "job"
	case "pipeline", "cicd", "ci/cd":
		return "pipeline"
	case "monitor", "alert":
		return "monitor"
	default:
		return "all"
	}
}

func encodeStringList(list []string) string {
	filtered := make([]string, 0, len(list))
	for _, item := range list {
		value := strings.TrimSpace(item)
		if value != "" {
			filtered = append(filtered, value)
		}
	}
	data, _ := json.Marshal(filtered)
	return string(data)
}

func decodeStringList(raw string) []string {
	var list []string
	if strings.TrimSpace(raw) == "" {
		return list
	}
	_ = json.Unmarshal([]byte(raw), &list)
	return list
}

func normalizeNotifyEvents(events []string, scope string) []string {
	seen := make(map[string]struct{}, len(events))
	result := make([]string, 0, len(events))
	for _, item := range events {
		value := strings.ToLower(strings.TrimSpace(item))
		if value == "" {
			continue
		}
		if value == "all" {
			return []string{"all"}
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	if len(result) > 0 {
		return result
	}
	switch normalizeNotifyScope(scope) {
	case "monitor":
		return []string{"firing", "recovered"}
	case "job":
		return []string{"failed", "waiting_approval", "rejected"}
	case "pipeline":
		return []string{"success", "failed", "waiting_approval", "rejected"}
	default:
		return []string{"success", "failed"}
	}
}

func normalizeNotifyHeaders(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "{}", nil
	}
	var headers map[string]string
	if err := json.Unmarshal([]byte(value), &headers); err != nil {
		return "", errors.New("headers must be valid JSON object")
	}
	data, _ := json.Marshal(headers)
	return string(data), nil
}

func mapNotifyTemplate(item model.NotifyTemplate) map[string]any {
	return map[string]any{
		"id":          item.ID,
		"name":        item.Name,
		"channelType": item.ChannelType,
		"scope":       normalizeNotifyScope(item.Scope),
		"title":       item.Title,
		"content":     item.Content,
		"status":      item.Status,
		"description": item.Description,
		"createTime":  item.CreatedAt,
		"updateTime":  item.UpdatedAt,
	}
}

func mapNotifyChannel(item model.NotifyChannel) map[string]any {
	return map[string]any{
		"id":          item.ID,
		"name":        item.Name,
		"channelType": item.ChannelType,
		"webhookUrl":  item.WebhookURL,
		"secret":      item.Secret,
		"headersJson": firstNonEmpty(item.HeadersJSON, "{}"),
		"status":      item.Status,
		"description": item.Description,
		"createTime":  item.CreatedAt,
		"updateTime":  item.UpdatedAt,
	}
}

func mapNotifyRule(item model.NotifyRule) map[string]any {
	return map[string]any{
		"id":          item.ID,
		"name":        item.Name,
		"scope":       item.Scope,
		"events":      decodeStringList(item.EventsJSON),
		"eventsJson":  firstNonEmpty(item.EventsJSON, "[]"),
		"templateId":  item.TemplateID,
		"channelIds":  decodeUintList(item.ChannelIDsJSON),
		"status":      item.Status,
		"description": item.Description,
		"createTime":  item.CreatedAt,
		"updateTime":  item.UpdatedAt,
	}
}

func (s *Service) ListNotifyTemplates(pageNum, pageSize int, keyword, channelType, scope, status string) (map[string]any, error) {
	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	query := s.db.Model(&model.NotifyTemplate{})
	if strings.TrimSpace(keyword) != "" {
		like := "%" + strings.TrimSpace(keyword) + "%"
		query = query.Where("name LIKE ? OR title LIKE ? OR description LIKE ?", like, like, like)
	}
	if strings.TrimSpace(channelType) != "" {
		query = query.Where("channel_type = ?", normalizeNotifyChannelType(channelType))
	}
	if strings.TrimSpace(scope) != "" {
		query = query.Where("scope = ?", normalizeNotifyScope(scope))
	}
	if strings.TrimSpace(status) != "" {
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var list []model.NotifyTemplate
	if err := query.Order("id DESC").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, err
	}
	rows := make([]map[string]any, 0, len(list))
	for _, item := range list {
		rows = append(rows, mapNotifyTemplate(item))
	}
	return map[string]any{"list": rows, "total": total, "pageNum": pageNum, "pageSize": pageSize}, nil
}

func (s *Service) ListNotifyTemplateOptions(channelType, scope string) ([]model.NotifyTemplate, error) {
	query := s.db.Where("status = ?", 1)
	if strings.TrimSpace(channelType) != "" {
		query = query.Where("channel_type IN ?", []string{normalizeNotifyChannelType(channelType), "webhook"})
	}
	if strings.TrimSpace(scope) != "" {
		normalizedScope := normalizeNotifyScope(scope)
		query = query.Where("scope IN ? OR scope = '' OR scope IS NULL", []string{normalizedScope, "all"})
	}
	var list []model.NotifyTemplate
	if err := query.Order("id DESC").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

func (s *Service) GetNotifyTemplate(id uint) (map[string]any, error) {
	var item model.NotifyTemplate
	if err := s.db.First(&item, id).Error; err != nil {
		return nil, err
	}
	return mapNotifyTemplate(item), nil
}

func (s *Service) SaveNotifyTemplate(payload NotifyTemplatePayload) error {
	if strings.TrimSpace(payload.Name) == "" {
		return errors.New("template name is required")
	}
	if strings.TrimSpace(payload.Content) == "" {
		return errors.New("template content is required")
	}
	updates := map[string]any{
		"name":         Trimmed(payload.Name),
		"channel_type": normalizeNotifyChannelType(payload.ChannelType),
		"scope":        normalizeNotifyScope(payload.Scope),
		"title":        Trimmed(payload.Title),
		"content":      payload.Content,
		"status":       normalizeNotifyStatus(payload.Status),
		"description":  Trimmed(payload.Description),
	}
	if payload.ID > 0 {
		return s.db.Model(&model.NotifyTemplate{}).Where("id = ?", payload.ID).Updates(updates).Error
	}
	return s.db.Model(&model.NotifyTemplate{}).Create(updates).Error
}

func (s *Service) DeleteNotifyTemplate(id uint) error {
	var count int64
	if err := s.db.Model(&model.NotifyRule{}).Where("template_id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return errors.New("template is used by notification rules")
	}
	return s.db.Delete(&model.NotifyTemplate{}, id).Error
}

func (s *Service) ListNotifyChannels(pageNum, pageSize int, keyword, channelType, status string) (map[string]any, error) {
	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	query := s.db.Model(&model.NotifyChannel{})
	if strings.TrimSpace(keyword) != "" {
		like := "%" + strings.TrimSpace(keyword) + "%"
		query = query.Where("name LIKE ? OR description LIKE ?", like, like)
	}
	if strings.TrimSpace(channelType) != "" {
		query = query.Where("channel_type = ?", normalizeNotifyChannelType(channelType))
	}
	if strings.TrimSpace(status) != "" {
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var list []model.NotifyChannel
	if err := query.Order("id DESC").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, err
	}
	rows := make([]map[string]any, 0, len(list))
	for _, item := range list {
		rows = append(rows, mapNotifyChannel(item))
	}
	return map[string]any{"list": rows, "total": total, "pageNum": pageNum, "pageSize": pageSize}, nil
}

func (s *Service) ListNotifyChannelOptions(channelType string) ([]model.NotifyChannel, error) {
	query := s.db.Where("status = ?", 1)
	if strings.TrimSpace(channelType) != "" {
		query = query.Where("channel_type = ?", normalizeNotifyChannelType(channelType))
	}
	var list []model.NotifyChannel
	if err := query.Order("id DESC").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

func (s *Service) GetNotifyChannel(id uint) (map[string]any, error) {
	var item model.NotifyChannel
	if err := s.db.First(&item, id).Error; err != nil {
		return nil, err
	}
	return mapNotifyChannel(item), nil
}

func (s *Service) SaveNotifyChannel(payload NotifyChannelPayload) error {
	if strings.TrimSpace(payload.Name) == "" {
		return errors.New("channel name is required")
	}
	if strings.TrimSpace(payload.WebhookURL) == "" {
		return errors.New("webhook url is required")
	}
	headersJSON, err := normalizeNotifyHeaders(payload.HeadersJSON)
	if err != nil {
		return err
	}
	updates := map[string]any{
		"name":         Trimmed(payload.Name),
		"channel_type": normalizeNotifyChannelType(payload.ChannelType),
		"webhook_url":  strings.TrimSpace(payload.WebhookURL),
		"secret":       strings.TrimSpace(payload.Secret),
		"headers_json": headersJSON,
		"status":       normalizeNotifyStatus(payload.Status),
		"description":  Trimmed(payload.Description),
	}
	if payload.ID > 0 {
		return s.db.Model(&model.NotifyChannel{}).Where("id = ?", payload.ID).Updates(updates).Error
	}
	return s.db.Model(&model.NotifyChannel{}).Create(updates).Error
}

func (s *Service) DeleteNotifyChannel(id uint) error {
	var rules []model.NotifyRule
	if err := s.db.Select("id", "name", "channel_ids_json").Find(&rules).Error; err != nil {
		return err
	}
	for _, rule := range rules {
		for _, channelID := range decodeUintList(rule.ChannelIDsJSON) {
			if channelID == id {
				return fmt.Errorf("notification channel is used by rule %q; update the rule first", rule.Name)
			}
		}
	}
	return s.db.Delete(&model.NotifyChannel{}, id).Error
}

func (s *Service) ListNotifyRules(pageNum, pageSize int, keyword, scope, status string) (map[string]any, error) {
	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	query := s.db.Model(&model.NotifyRule{})
	if strings.TrimSpace(keyword) != "" {
		like := "%" + strings.TrimSpace(keyword) + "%"
		query = query.Where("name LIKE ? OR description LIKE ?", like, like)
	}
	if strings.TrimSpace(scope) != "" {
		query = query.Where("scope IN ?", []string{normalizeNotifyScope(scope), "all"})
	}
	if strings.TrimSpace(status) != "" {
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var list []model.NotifyRule
	if err := query.Order("id DESC").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, err
	}
	rows := make([]map[string]any, 0, len(list))
	for _, item := range list {
		rows = append(rows, mapNotifyRule(item))
	}
	return map[string]any{"list": rows, "total": total, "pageNum": pageNum, "pageSize": pageSize}, nil
}

func (s *Service) ListNotifyRuleOptions(scope string) ([]map[string]any, error) {
	query := s.db.Where("status = ?", 1)
	if strings.TrimSpace(scope) != "" {
		query = query.Where("scope IN ?", []string{normalizeNotifyScope(scope), "all"})
	}
	var list []model.NotifyRule
	if err := query.Order("id DESC").Find(&list).Error; err != nil {
		return nil, err
	}
	rows := make([]map[string]any, 0, len(list))
	for _, item := range list {
		rows = append(rows, mapNotifyRule(item))
	}
	return rows, nil
}

func (s *Service) GetNotifyRule(id uint) (map[string]any, error) {
	var item model.NotifyRule
	if err := s.db.First(&item, id).Error; err != nil {
		return nil, err
	}
	return mapNotifyRule(item), nil
}

func (s *Service) SaveNotifyRule(payload NotifyRulePayload) error {
	if strings.TrimSpace(payload.Name) == "" {
		return errors.New("rule name is required")
	}
	if payload.TemplateID == 0 {
		return errors.New("notification template is required")
	}
	if len(payload.ChannelIDs) == 0 {
		return errors.New("notification channel is required")
	}
	var tmpl model.NotifyTemplate
	if err := s.db.First(&tmpl, payload.TemplateID).Error; err != nil {
		return errors.New("selected message template does not exist")
	}
	ruleScope := normalizeNotifyScope(payload.Scope)
	if !notifyTemplateScopeCompatible(tmpl.Scope, ruleScope) {
		return fmt.Errorf("message template is scoped to %s and cannot be used by a %s notification rule", notifyScopeLabel(tmpl.Scope), notifyScopeLabel(ruleScope))
	}
	var channels []model.NotifyChannel
	if err := s.db.Where("id IN ?", payload.ChannelIDs).Find(&channels).Error; err != nil {
		return err
	}
	if len(channels) != len(payload.ChannelIDs) {
		return errors.New("some selected notification channels do not exist; select them again")
	}
	templateType := normalizeNotifyChannelType(tmpl.ChannelType)
	for _, channel := range channels {
		if normalizeNotifyChannelType(channel.ChannelType) != templateType {
			return fmt.Errorf("message template type %s cannot be sent to channel %q (%s)", templateType, channel.Name, channel.ChannelType)
		}
	}
	events := normalizeNotifyEvents(payload.Events, payload.Scope)
	updates := map[string]any{
		"name":             Trimmed(payload.Name),
		"scope":            ruleScope,
		"events_json":      encodeStringList(events),
		"template_id":      payload.TemplateID,
		"channel_ids_json": encodeUintList(payload.ChannelIDs),
		"status":           normalizeNotifyStatus(payload.Status),
		"description":      Trimmed(payload.Description),
	}
	if payload.ID > 0 {
		return s.db.Model(&model.NotifyRule{}).Where("id = ?", payload.ID).Updates(updates).Error
	}
	return s.db.Model(&model.NotifyRule{}).Create(updates).Error
}

func (s *Service) DeleteNotifyRule(id uint) error {
	return s.db.Delete(&model.NotifyRule{}, id).Error
}
