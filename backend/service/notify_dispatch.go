package service

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"ops-admin/backend/model"
)

func (s *Service) DispatchNotifyRule(ruleID uint, event NotifyEvent) {
	_, _ = s.enqueueNotifyRule(ruleID, event, false)
}

func (s *Service) TestNotifyRule(ruleID uint) (map[string]any, error) {
	now := time.Now()
	var rule model.NotifyRule
	if err := s.db.First(&rule, ruleID).Error; err != nil {
		return nil, err
	}
	count, err := s.enqueueNotifyRule(ruleID, NotifyEvent{
		Scope:      normalizeNotifyScope(rule.Scope),
		Event:      "notify",
		TargetID:   ruleID,
		TargetName: "Notification Rule Test",
		Status:     "firing",
		Summary:    "Ops Admin에서 발송한 Notification Rule 테스트 메시지입니다.",
		Detail:     "이 메시지를 받았다면 Template, Notification Channel, 영속 Delivery Pipeline이 정상적으로 동작합니다.",
		StartedAt:  &now,
		Extra:      map[string]string{"operator": "System Administrator"},
	}, true)
	if err != nil {
		return nil, err
	}
	return map[string]any{"queued": count}, nil
}

func (s *Service) enqueueNotifyRule(ruleID uint, event NotifyEvent, allowDisabledRule bool) (int, error) {
	if ruleID == 0 {
		return 0, errors.New("notification rule is required")
	}
	var rule model.NotifyRule
	if err := s.db.First(&rule, ruleID).Error; err != nil {
		return 0, err
	}
	if !allowDisabledRule && rule.Status != 1 {
		return 0, nil
	}
	if rule.Scope != "all" && rule.Scope != normalizeNotifyScope(event.Scope) {
		return 0, nil
	}
	if event.Event != "notify" {
		if !notifyEventMatch(decodeStringList(rule.EventsJSON), event.Event, event.Status) {
			return 0, nil
		}
	}

	var tmpl model.NotifyTemplate
	if err := s.db.First(&tmpl, rule.TemplateID).Error; err != nil {
		return 0, fmt.Errorf("failed to read message template: %w", err)
	}
	if tmpl.Status != 1 {
		return 0, errors.New("message template is disabled")
	}
	if !notifyTemplateScopeCompatible(tmpl.Scope, event.Scope) {
		return 0, fmt.Errorf("message template is scoped to %s and cannot process a %s event", notifyScopeLabel(tmpl.Scope), notifyScopeLabel(event.Scope))
	}

	ids := decodeUintList(rule.ChannelIDsJSON)
	if len(ids) == 0 {
		return 0, errors.New("notification rule has no configured channel")
	}
	var channels []model.NotifyChannel
	if err := s.db.Where("id IN ?", ids).Find(&channels).Error; err != nil {
		return 0, err
	}
	if len(channels) == 0 {
		return 0, errors.New("a channel referenced by the notification rule does not exist")
	}

	now := time.Now()
	queued := 0
	for _, channel := range channels {
		titleTemplate, contentTemplate := normalizeNotifyTemplateForEvent(firstNonEmpty(tmpl.Title, event.TargetName), tmpl.Content, event)
		title := renderNotifyTemplate(titleTemplate, event)
		content := renderNotifyTemplate(contentTemplate, event)
		body, buildErr := buildNotifyBody(channel.ChannelType, title, content, event)
		status := notifyStatusPending
		errorText := ""
		if channel.Status != 1 {
			status = notifyStatusFailed
			errorText = "notification channel is disabled"
		} else if normalizeNotifyChannelType(channel.ChannelType) != normalizeNotifyChannelType(tmpl.ChannelType) {
			status = notifyStatusFailed
			errorText = "message template and notification channel types are incompatible"
		} else if buildErr != nil {
			status = notifyStatusFailed
			errorText = buildErr.Error()
		}
		item := model.NotifySendLog{
			DeliveryID: newNotifyDeliveryID(),
			RuleID:     rule.ID, RuleName: rule.Name,
			ChannelID: channel.ID, ChannelName: channel.Name, ChannelType: channel.ChannelType,
			Event: event.Event, Scope: firstNonEmpty(event.Scope, rule.Scope),
			TargetID: event.TargetID, TargetName: event.TargetName, Summary: event.Summary,
			Status: status, MaxAttempts: defaultNotifyRetries,
			NextRetryAt: &now, RequestBody: string(body), ErrorText: errorText,
		}
		if err := s.db.Create(&item).Error; err != nil {
			return queued, err
		}
		if status == notifyStatusPending {
			queued++
		}
	}
	return queued, nil
}

func notifyTemplateScopeCompatible(templateScope, targetScope string) bool {
	templateScope = normalizeNotifyScope(templateScope)
	targetScope = normalizeNotifyScope(targetScope)
	if templateScope == "all" {
		return true
	}
	return targetScope != "all" && templateScope == targetScope
}

func notifyScopeLabel(scope string) string {
	switch normalizeNotifyScope(scope) {
	case "monitor":
		return "Monitoring Alert"
	case "schedule":
		return "Scheduled Task"
	case "job":
		return "Job Orchestration"
	case "pipeline":
		return "CI/CD Pipeline"
	default:
		return "All Scopes"
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

func newNotifyDeliveryID() string {
	random := make([]byte, 6)
	if _, err := rand.Read(random); err != nil {
		return fmt.Sprintf("NTF-%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("NTF-%d-%s", time.Now().UnixMilli(), strings.ToUpper(hex.EncodeToString(random)))
}

func (s *Service) initNotifyDispatcher() {
	s.notifyDispatcherOnce.Do(func() {
		s.notifyConcurrency = make(chan struct{}, 5)
		var legacyIDs []uint
		if err := s.db.Model(&model.NotifySendLog{}).Where("delivery_id = '' OR delivery_id IS NULL").Pluck("id", &legacyIDs).Error; err == nil {
			for _, id := range legacyIDs {
				_ = s.db.Model(&model.NotifySendLog{}).Where("id = ?", id).Update("delivery_id", fmt.Sprintf("NTF-LEGACY-%d", id)).Error
			}
		}
		now := time.Now()
		_ = s.db.Model(&model.NotifySendLog{}).
			Where("status = ? AND updated_at < ?", notifyStatusSending, now.Add(-2*time.Minute)).
			Updates(map[string]any{"status": notifyStatusRetrying, "next_retry_at": now, "error_text": "unfinished delivery resumed after service restart"}).Error
		go func() {
			s.dispatchPendingNotifications()
			ticker := time.NewTicker(2 * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				s.dispatchPendingNotifications()
			}
		}()
	})
}

func (s *Service) dispatchPendingNotifications() {
	now := time.Now()
	var list []model.NotifySendLog
	if err := s.db.Where("status IN ? AND (next_retry_at IS NULL OR next_retry_at <= ?)", []string{notifyStatusPending, notifyStatusRetrying}, now).
		Order("id ASC").Limit(20).Find(&list).Error; err != nil {
		return
	}
	for _, item := range list {
		claimed := s.db.Model(&model.NotifySendLog{}).
			Where("id = ? AND status IN ?", item.ID, []string{notifyStatusPending, notifyStatusRetrying}).
			Updates(map[string]any{"status": notifyStatusSending, "next_retry_at": nil})
		if claimed.Error != nil || claimed.RowsAffected == 0 {
			continue
		}
		s.notifyConcurrency <- struct{}{}
		go func(id uint) {
			defer func() { <-s.notifyConcurrency }()
			s.processNotifySendLog(id)
		}(item.ID)
	}
}

func (s *Service) processNotifySendLog(id uint) {
	var item model.NotifySendLog
	if err := s.db.First(&item, id).Error; err != nil {
		return
	}
	startedAt := time.Now()
	attempt := item.AttemptCount + 1
	result := notifyWebhookResult{}
	var sendErr error
	var channel model.NotifyChannel
	if err := s.db.First(&channel, item.ChannelID).Error; err != nil {
		sendErr = fmt.Errorf("notification channel does not exist: %w", err)
	} else if channel.Status != 1 {
		sendErr = errors.New("notification channel is disabled")
	} else {
		result, sendErr = postNotifyWebhook(channel, []byte(item.RequestBody))
	}

	updates := map[string]any{
		"attempt_count":   attempt,
		"last_attempt_at": startedAt,
		"duration_ms":     time.Since(startedAt).Milliseconds(),
		"http_status":     result.HTTPStatus,
		"business_code":   result.BusinessCode,
		"response":        result.Body,
	}
	if sendErr == nil {
		updates["status"] = notifyStatusSuccess
		updates["error_text"] = ""
		updates["next_retry_at"] = nil
	} else {
		updates["error_text"] = sendErr.Error()
		if attempt < maxInt(item.MaxAttempts, defaultNotifyRetries) {
			nextRetryAt := time.Now().Add(notifyRetryDelay(attempt))
			updates["status"] = notifyStatusRetrying
			updates["next_retry_at"] = nextRetryAt
		} else {
			updates["status"] = notifyStatusFailed
			updates["next_retry_at"] = nil
		}
	}
	_ = s.db.Model(&model.NotifySendLog{}).Where("id = ?", id).Updates(updates).Error
}

func notifyRetryDelay(attempt int) time.Duration {
	switch attempt {
	case 1:
		return 10 * time.Second
	case 2:
		return 30 * time.Second
	default:
		return 2 * time.Minute
	}
}

func maxInt(value, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}
