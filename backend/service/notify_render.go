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
	"strconv"
	"strings"
	"time"

	"ops-admin/backend/model"

	"gorm.io/gorm"
)

func renderNotifyTemplate(template string, event NotifyEvent) string {
	statusColor, statusTone := notifyStatusStyle(event.Status)
	status := notifyStatusLabel(event.Scope, event.Status)
	values := map[string]string{
		"scope":       event.Scope,
		"event":       event.Event,
		"targetId":    fmt.Sprintf("%d", event.TargetID),
		"targetName":  event.TargetName,
		"status":      status,
		"summary":     event.Summary,
		"detail":      event.Detail,
		"startedAt":   formatNotifyTime(event.StartedAt),
		"finishedAt":  formatNotifyTime(event.FinishedAt),
		"statusColor": statusColor,
		"statusTone":  statusTone,
		// Alert fields have sensible fallbacks for job and schedule templates.
		// This prevents a wrongly selected template from exposing raw {{tokens}}.
		"alertName":      event.TargetName,
		"severity":       "-",
		"datasourceName": "-",
		"instance":       "-",
		"value":          "-",
		"threshold":      "-",
		"taskName":       event.TargetName,
		"taskType":       "-",
		"triggerType":    "-",
		"cronExpr":       "-",
		"duration":       "-",
		"durationMs":     "-",
		"httpStatus":     "-",
		"expectedStatus": "-",
		"jobName":        event.TargetName,
		"jobHistoryId":   "-",
		"stepName":       "-",
		"stepMessage":    event.Summary,
		"notifyAt":       formatNotifyTime(event.FinishedAt),
	}
	for key, value := range event.Extra {
		values[key] = value
	}
	return notifyTemplateVariablePattern.ReplaceAllStringFunc(template, func(token string) string {
		key := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(token, "{{"), "}}"))
		if value, ok := values[key]; ok {
			return value
		}
		return "-"
	})
}

func notifyStatusLabel(scope, status string) string {
	value := strings.ToLower(strings.TrimSpace(status))
	switch normalizeNotifyScope(scope) {
	case "monitor":
		switch value {
		case "firing":
			return "발생"
		case "recovered", "resolved":
			return "복구"
		case "claimed":
			return "인계됨"
		}
	case "job":
		switch value {
		case "notice", "notify":
			return "알림"
		case "success", "completed":
			return "성공"
		case "failed", "error":
			return "실패"
		case "running":
			return "실행 중"
		case "waiting_approval":
			return "수동 확인 대기"
		case "rejected":
			return "거부됨"
		}
	case "pipeline":
		switch value {
		case "notice", "notify":
			return "알림"
		case "success", "completed":
			return "성공"
		case "failed", "error":
			return "실패"
		case "running":
			return "실행 중"
		case "waiting_approval":
			return "수동 확인 대기"
		case "rejected":
			return "거부됨"
		}
	case "schedule":
		return scheduleNotifyStatusLabel(status)
	}
	return status
}

func normalizeNotifyTemplateForEvent(title, content string, event NotifyEvent) (string, string) {
	scope := normalizeNotifyScope(event.Scope)
	if scope != "job" && scope != "schedule" && scope != "pipeline" {
		return title, content
	}
	monitorTokens := []string{"{{severity}}", "{{alertName}}", "{{datasourceName}}", "{{instance}}", "{{value}}", "{{threshold}}"}
	combined := title + "\n" + content
	usesMonitorFields := false
	for _, token := range monitorTokens {
		if strings.Contains(combined, token) {
			usesMonitorFields = true
			break
		}
	}
	if !usesMonitorFields {
		return title, content
	}
	if scope == "schedule" {
		return "[정기 작업] {{taskName}} · {{status}}", "**실행 상태:** {{status}}\n\n**Task 이름:** {{taskName}}\n**Task Type:** {{taskType}}\n**Trigger 방식:** {{triggerType}}\n**Cron:** {{cronExpr}}\n**실행 시간:** {{duration}}\n**완료 시각:** {{finishedAt}}\n\n---\n\n**실행 요약**\n{{summary}}\n\n{{detail}}"
	}
	if scope == "pipeline" {
		return "[파이프라인 알림] {{pipelineName}} · {{stageName}}", "**실행 상태:** {{status}}\n\n**Pipeline:** {{pipelineName}}\n**Run ID:** #{{pipelineRunId}}\n**Application:** {{appName}}\n**Environment:** {{env}}\n**Branch:** {{branch}}\n**Image Version:** {{imageTag}}\n**알림 시각:** {{notifyAt}}\n\n---\n\n**실행 요약**\n{{summary}}\n\n{{detail}}"
	}
	return "[Job 알림] {{jobName}} · {{stepName}}", "**알림 Type:** {{status}}\n\n**Job 이름:** {{jobName}}\n**Run ID:** #{{jobHistoryId}}\n**현재 Step:** {{stepName}}\n**Trigger 방식:** {{triggerType}}\n**알림 시각:** {{notifyAt}}\n\n---\n\n**알림 요약**\n{{summary}}\n\n{{detail}}"
}

func scheduleNotifyStatusLabel(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "success", "completed":
		return "성공"
	case "failed", "error":
		return "실패"
	case "running":
		return "실행 중"
	default:
		return status
	}
}

func notifyStatusStyle(status string) (string, string) {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "recovered", "resolved", "success", "completed":
		return "#00B42A", "info"
	case "firing", "failed", "error":
		return "#F53F3F", "warning"
	case "notice", "notify":
		return "#165DFF", "info"
	default:
		return "#FF7D00", "comment"
	}
}

func feishuHeaderTemplate(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "recovered", "resolved", "success", "completed":
		return "green"
	case "firing", "failed", "error":
		return "red"
	case "notice", "notify":
		return "blue"
	default:
		return "orange"
	}
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
				"header": map[string]any{
					"template": feishuHeaderTemplate(event.Status),
					"title":    map[string]string{"tag": "plain_text", "content": title},
				},
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

type notifyWebhookResult struct {
	Body         string
	HTTPStatus   int
	BusinessCode string
}

func postNotifyWebhook(channel model.NotifyChannel, body []byte) (notifyWebhookResult, error) {
	result := notifyWebhookResult{}
	webhookURL := strings.TrimSpace(channel.WebhookURL)
	if webhookURL == "" {
		return result, errors.New("webhook URL is empty")
	}
	if channel.Secret != "" && normalizeNotifyChannelType(channel.ChannelType) == "dingtalk" {
		signedURL, err := signDingTalkURL(webhookURL, channel.Secret)
		if err != nil {
			return result, err
		}
		webhookURL = signedURL
	}
	request, err := http.NewRequest(http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return result, err
	}
	request.Header.Set("Content-Type", "application/json")
	for key, value := range parseHeaderMap(channel.HeadersJSON) {
		request.Header.Set(key, value)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return result, err
	}
	defer response.Body.Close()
	responseBody, _ := io.ReadAll(io.LimitReader(response.Body, 32768))
	result.Body = string(responseBody)
	result.HTTPStatus = response.StatusCode
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return result, fmt.Errorf("webhook returned HTTP %d", response.StatusCode)
	}
	code, message, exists := parseNotifyBusinessResponse(channel.ChannelType, responseBody)
	result.BusinessCode = code
	if exists && code != "0" {
		return result, fmt.Errorf("platform returned business error code %s: %s", code, firstNonEmpty(message, "unknown error"))
	}
	return result, nil
}

func parseNotifyBusinessResponse(channelType string, body []byte) (string, string, bool) {
	var payload map[string]any
	if len(bytes.TrimSpace(body)) == 0 || json.Unmarshal(body, &payload) != nil {
		return "", "", false
	}
	var codeKey string
	switch normalizeNotifyChannelType(channelType) {
	case "dingtalk", "wecom":
		codeKey = "errcode"
	case "feishu":
		codeKey = "code"
	default:
		return "", "", false
	}
	value, exists := payload[codeKey]
	if !exists {
		return "", "", false
	}
	code := notifyBusinessCode(value)
	message := notifyResponseMessage(payload, "errmsg", "msg", "message")
	return code, message, true
}

func notifyResponseMessage(payload map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, exists := payload[key]; exists && value != nil {
			text := strings.TrimSpace(fmt.Sprint(value))
			if text != "" {
				return text
			}
		}
	}
	return ""
}

func notifyBusinessCode(value any) string {
	switch code := value.(type) {
	case float64:
		return strconv.FormatInt(int64(code), 10)
	case json.Number:
		return code.String()
	case string:
		return strings.TrimSpace(code)
	default:
		return strings.TrimSpace(fmt.Sprint(code))
	}
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

func (s *Service) ListNotifySendLogs(pageNum, pageSize int, keyword, status, channelType, scope, startTime, endTime string) (map[string]any, error) {
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
	if strings.TrimSpace(channelType) != "" {
		query = query.Where("channel_type = ?", normalizeNotifyChannelType(channelType))
	}
	if strings.TrimSpace(scope) != "" {
		query = query.Where("scope = ?", normalizeNotifyScope(scope))
	}
	if value, ok := parseNotifyQueryTime(startTime); ok {
		query = query.Where("created_at >= ?", value)
	}
	if value, ok := parseNotifyQueryTime(endTime); ok {
		query = query.Where("created_at <= ?", value)
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

func parseNotifyQueryTime(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{"2006-01-02 15:04:05", time.RFC3339} {
		if parsed, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func (s *Service) RetryNotifySendLog(id uint) (map[string]any, error) {
	var original model.NotifySendLog
	if err := s.db.First(&original, id).Error; err != nil {
		return nil, err
	}
	if original.Status != notifyStatusFailed {
		return nil, errors.New("only failed delivery records can be resent")
	}
	now := time.Now()
	retry := model.NotifySendLog{
		DeliveryID: newNotifyDeliveryID(),
		RuleID:     original.RuleID, RuleName: original.RuleName,
		ChannelID: original.ChannelID, ChannelName: original.ChannelName, ChannelType: original.ChannelType,
		Event: original.Event, Scope: original.Scope,
		TargetID: original.TargetID, TargetName: original.TargetName, Summary: original.Summary,
		Status: notifyStatusPending, MaxAttempts: defaultNotifyRetries, NextRetryAt: &now,
		RetryOfID: original.ID, RequestBody: original.RequestBody,
	}
	if err := s.db.Create(&retry).Error; err != nil {
		return nil, err
	}
	return map[string]any{"id": retry.ID, "deliveryId": retry.DeliveryID}, nil
}
