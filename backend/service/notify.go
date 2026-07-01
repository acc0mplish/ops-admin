package service

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"ops-admin/backend/model"

	"gorm.io/gorm"
)

type NotifyTemplatePayload struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	ChannelType string `json:"channelType"`
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

func (s *Service) ListNotifyTemplates(pageNum, pageSize int, keyword, channelType, status string) (map[string]any, error) {
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

func (s *Service) ListNotifyTemplateOptions(channelType string) ([]model.NotifyTemplate, error) {
	query := s.db.Where("status = ?", 1)
	if strings.TrimSpace(channelType) != "" {
		query = query.Where("channel_type IN ?", []string{normalizeNotifyChannelType(channelType), "webhook"})
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
	events := payload.Events
	if len(events) == 0 {
		events = []string{"success", "failed"}
	}
	updates := map[string]any{
		"name":             Trimmed(payload.Name),
		"scope":            normalizeNotifyScope(payload.Scope),
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

func (s *Service) DispatchNotifyRule(ruleID uint, event NotifyEvent) {
	if ruleID == 0 {
		return
	}
	var rule model.NotifyRule
	if err := s.db.First(&rule, ruleID).Error; err != nil || rule.Status != 1 {
		return
	}
	if rule.Scope != "all" && rule.Scope != normalizeNotifyScope(event.Scope) {
		return
	}
	if event.Event != "notify" && !notifyEventMatch(decodeStringList(rule.EventsJSON), event.Event, event.Status) {
		return
	}

	var tmpl model.NotifyTemplate
	if err := s.db.First(&tmpl, rule.TemplateID).Error; err != nil || tmpl.Status != 1 {
		return
	}

	var channels []model.NotifyChannel
	ids := decodeUintList(rule.ChannelIDsJSON)
	if len(ids) == 0 {
		return
	}
	if err := s.db.Where("id IN ? AND status = ?", ids, 1).Find(&channels).Error; err != nil {
		return
	}
	for _, channel := range channels {
		channel := channel
		go s.sendNotifyMessage(rule, tmpl, channel, event)
	}
}

func notifyEventMatch(events []string, event, status string) bool {
	if len(events) == 0 {
		return true
	}
	for _, item := range events {
		value := strings.ToLower(strings.TrimSpace(item))
		if value == "all" || value == strings.ToLower(event) || value == strings.ToLower(status) {
			return true
		}
	}
	return false
}

func (s *Service) sendNotifyMessage(rule model.NotifyRule, tmpl model.NotifyTemplate, channel model.NotifyChannel, event NotifyEvent) {
	title := renderNotifyTemplate(firstNonEmpty(tmpl.Title, event.TargetName), event)
	content := renderNotifyTemplate(tmpl.Content, event)
	body, err := buildNotifyBody(channel.ChannelType, title, content, event)
	requestBody := string(body)
	status := "success"
	responseText := ""
	errorText := ""
	if err == nil {
		responseText, err = postNotifyWebhook(channel, body)
	}
	if err != nil {
		status = "failed"
		errorText = err.Error()
	}
	_ = s.db.Create(&model.NotifySendLog{
		RuleID: rule.ID, RuleName: rule.Name,
		ChannelID: channel.ID, ChannelName: channel.Name, ChannelType: channel.ChannelType,
		Event: event.Event, Scope: event.Scope, TargetID: event.TargetID, TargetName: event.TargetName,
		Status: status, RequestBody: requestBody, Response: responseText, ErrorText: errorText,
	}).Error
}

func renderNotifyTemplate(template string, event NotifyEvent) string {
	replacer := strings.NewReplacer(
		"{{scope}}", event.Scope,
		"{{event}}", event.Event,
		"{{targetId}}", fmt.Sprintf("%d", event.TargetID),
		"{{targetName}}", event.TargetName,
		"{{status}}", event.Status,
		"{{summary}}", event.Summary,
		"{{detail}}", event.Detail,
		"{{startedAt}}", formatNotifyTime(event.StartedAt),
		"{{finishedAt}}", formatNotifyTime(event.FinishedAt),
	)
	output := replacer.Replace(template)
	for key, value := range event.Extra {
		output = strings.ReplaceAll(output, "{{"+key+"}}", value)
	}
	return output
}

func formatNotifyTime(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.Format("2006-01-02 15:04:05")
}

func buildNotifyBody(channelType, title, content string, event NotifyEvent) ([]byte, error) {
	switch normalizeNotifyChannelType(channelType) {
	case "dingtalk":
		return json.Marshal(map[string]any{
			"msgtype":  "markdown",
			"markdown": map[string]string{"title": title, "text": content},
		})
	case "wecom":
		return json.Marshal(map[string]any{
			"msgtype":  "markdown",
			"markdown": map[string]string{"content": content},
		})
	case "feishu":
		return json.Marshal(map[string]any{
			"msg_type": "interactive",
			"card": map[string]any{
				"header":   map[string]any{"title": map[string]string{"tag": "plain_text", "content": title}},
				"elements": []map[string]any{{"tag": "markdown", "content": content}},
			},
		})
	default:
		return json.Marshal(map[string]any{
			"title": title, "content": content,
			"scope": event.Scope, "event": event.Event, "status": event.Status,
			"targetId": event.TargetID, "targetName": event.TargetName,
			"summary": event.Summary, "detail": event.Detail,
		})
	}
}

func postNotifyWebhook(channel model.NotifyChannel, body []byte) (string, error) {
	webhookURL := strings.TrimSpace(channel.WebhookURL)
	if webhookURL == "" {
		return "", errors.New("webhook url is empty")
	}
	if channel.Secret != "" && normalizeNotifyChannelType(channel.ChannelType) == "dingtalk" {
		signedURL, err := signDingTalkURL(webhookURL, channel.Secret)
		if err != nil {
			return "", err
		}
		webhookURL = signedURL
	}
	request, err := http.NewRequest(http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/json")
	for key, value := range parseHeaderMap(channel.HeadersJSON) {
		request.Header.Set(key, value)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	responseBody, _ := io.ReadAll(io.LimitReader(response.Body, 32768))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return string(responseBody), fmt.Errorf("webhook returned status %d", response.StatusCode)
	}
	return string(responseBody), nil
}

func signDingTalkURL(rawURL, secret string) (string, error) {
	timestamp := time.Now().UnixMilli()
	stringToSign := fmt.Sprintf("%d\n%s", timestamp, secret)
	h := hmac.New(sha256.New, []byte(secret))
	_, _ = h.Write([]byte(stringToSign))
	sign := url.QueryEscape(base64.StdEncoding.EncodeToString(h.Sum(nil)))
	sep := "?"
	if strings.Contains(rawURL, "?") {
		sep = "&"
	}
	return fmt.Sprintf("%s%stimestamp=%d&sign=%s", rawURL, sep, timestamp, sign), nil
}

func (s *Service) ListNotifySendLogs(pageNum, pageSize int, keyword, status string) (map[string]any, error) {
	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	query := s.db.Model(&model.NotifySendLog{})
	if strings.TrimSpace(keyword) != "" {
		like := "%" + strings.TrimSpace(keyword) + "%"
		query = query.Where("rule_name LIKE ? OR channel_name LIKE ? OR target_name LIKE ? OR summary LIKE ?", like, like, like, like)
	}
	if strings.TrimSpace(status) != "" {
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var list []model.NotifySendLog
	if err := query.Order("id DESC").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	return map[string]any{"list": list, "total": total, "pageNum": pageNum, "pageSize": pageSize}, nil
}
