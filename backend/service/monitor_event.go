package service

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"ops-admin/backend/model"

	"gorm.io/gorm"
)

func monitorFingerprint(ruleID uint, metric map[string]string) string {
	keys := make([]string, 0, len(metric))
	for key := range metric {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := []string{fmt.Sprintf("rule=%d", ruleID)}
	for _, key := range keys {
		parts = append(parts, key+"="+metric[key])
	}
	sum := sha1.Sum([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(sum[:])
}

func monitorSilenceRuleCriteriaMatch(item model.MonitorSilenceRule, rule model.MonitorAlertRule, labels map[string]string) bool {
	if !monitorRuleMatch(item.MatchMode, item.RuleIDsJSON, item.RuleNamePattern, rule) || !monitorSeverityMatch(item.Severity, rule.Severity) || !monitorAlertTypeMatch(item.AlertType, rule.AlertType) {
		return false
	}
	return labels == nil || monitorMatchersMatch(item.MatchersJSON, labels)
}

func monitorAlertTypeMatch(expected, actual string) bool {
	expected = strings.TrimSpace(expected)
	return expected == "" || expected == strings.TrimSpace(actual)
}

func (s *Service) matchMonitorSilenceRule(rule model.MonitorAlertRule, labels map[string]string) (*model.MonitorSilenceRule, bool) {
	now := time.Now()
	var rules []model.MonitorSilenceRule
	if err := s.db.Where("status = ?", 1).Order("priority DESC, id DESC").Find(&rules).Error; err != nil {
		return nil, false
	}
	for _, item := range rules {
		if item.StartsAt != nil && now.Before(*item.StartsAt) {
			continue
		}
		if item.EndsAt != nil && now.After(*item.EndsAt) {
			continue
		}
		if !monitorSilenceRuleCriteriaMatch(item, rule, labels) {
			continue
		}
		return &item, true
	}
	return nil, false
}

func (s *Service) matchMonitorAggregationRule(rule model.MonitorAlertRule, labels map[string]string) (*model.MonitorAggregationRule, string, bool) {
	var rules []model.MonitorAggregationRule
	if err := s.db.Where("status = ?", 1).Order("id DESC").Find(&rules).Error; err != nil {
		return nil, "", false
	}
	for _, item := range rules {
		if !monitorRuleMatch(item.MatchMode, item.RuleIDsJSON, item.RuleNamePattern, rule) || !monitorSeverityMatch(item.Severity, rule.Severity) || !monitorAlertTypeMatch(item.AlertType, rule.AlertType) {
			continue
		}
		groupBy := decodeStringList(item.GroupByJSON)
		parts := []string{fmt.Sprintf("aggregation=%d", item.ID), "rule=" + rule.Name, "severity=" + rule.Severity}
		for _, key := range groupBy {
			key = strings.TrimSpace(key)
			if key != "" {
				parts = append(parts, key+"="+labels[key])
			}
		}
		return &item, strings.Join(parts, "|"), true
	}
	return nil, "", false
}

func (s *Service) shouldNotifyAggregatedEvent(event model.MonitorAlertEvent, aggregation *model.MonitorAggregationRule) bool {
	if aggregation == nil || event.AggregationKey == "" {
		return true
	}
	lookbackSeconds := aggregationLookbackSeconds(*aggregation)
	windowStart := time.Now().Add(-time.Duration(lookbackSeconds) * time.Second)
	var last model.MonitorAlertEvent
	err := s.db.Where("aggregation_key = ? AND id <> ? AND last_notify_at >= ?", event.AggregationKey, event.ID, windowStart).
		Order("last_notify_at DESC").First(&last).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return true
	}
	if err != nil || last.LastNotifyAt == nil {
		return true
	}
	repeatAfter := time.Duration(lookbackSeconds) * time.Second
	return time.Since(*last.LastNotifyAt) >= repeatAfter
}

func aggregationLookbackSeconds(aggregation model.MonitorAggregationRule) int {
	windowSeconds := normalizeAggregationWindow(aggregation.WindowSeconds)
	repeatSeconds := normalizeAggregationWindow(aggregation.RepeatIntervalSeconds)
	if repeatSeconds > windowSeconds {
		return repeatSeconds
	}
	return windowSeconds
}

// shouldNotifyAlertEvent applies the per-rule reminder interval before any
// cross-event aggregation suppression. Events are still persisted every time.
func (s *Service) shouldNotifyAlertEvent(event model.MonitorAlertEvent, rule model.MonitorAlertRule, aggregation *model.MonitorAggregationRule) bool {
	if rule.MaxNotifyCount > 0 && event.NotifyCount >= rule.MaxNotifyCount {
		return false
	}
	if event.LastNotifyAt != nil && time.Since(*event.LastNotifyAt) < time.Duration(normalizeNotifyRepeatInterval(rule.NotifyRepeatIntervalSeconds))*time.Second {
		return false
	}
	return s.shouldNotifyAggregatedEvent(event, aggregation)
}

func (s *Service) markMonitorAlertNotified(event *model.MonitorAlertEvent) bool {
	now := time.Now()
	if err := s.db.Model(&model.MonitorAlertEvent{}).Where("id = ?", event.ID).Updates(map[string]any{
		"last_notify_at": &now,
		"notify_count":   gorm.Expr("notify_count + ?", 1),
	}).Error; err == nil {
		event.LastNotifyAt = &now
		event.NotifyCount++
		return true
	}
	return false
}

func (s *Service) notifyMonitorAlertIfAllowed(event *model.MonitorAlertEvent, rule model.MonitorAlertRule, aggregation *model.MonitorAggregationRule, status string) bool {
	// A matching aggregation rule changes the delivery model from "send this
	// event" to "collect this group and send one summary after its window".
	// The scheduled flusher owns the notification marker for the whole group.
	if aggregation != nil && event.AggregationKey != "" && status == "firing" {
		return false
	}
	// Keep the aggregation decision and its persisted notification marker in one
	// critical section so concurrent rule evaluations cannot both notify.
	s.monitorNotifyMu.Lock()
	if !s.shouldNotifyAlertEvent(*event, rule, aggregation) || !s.markMonitorAlertNotified(event) {
		s.monitorNotifyMu.Unlock()
		return false
	}
	s.monitorNotifyMu.Unlock()
	s.dispatchMonitorNotification(rule, *event, status)
	s.appendMonitorAlertTimeline(event.ID, "notification", "notification submitted", fmt.Sprintf("status: %s; send attempt %d", status, event.NotifyCount), "System", map[string]any{
		"notifyRuleId": rule.NotifyRuleID, "notifyCount": event.NotifyCount, "status": status,
	})
	return true
}

// flushDueMonitorAggregationNotifications delivers one summary for every due
// aggregation bucket. Individual events remain visible in the event list, but
// are never sent one-by-one while they belong to an aggregation rule.
func (s *Service) flushDueMonitorAggregationNotifications() {
	if s.db == nil {
		return
	}
	now := time.Now()
	var aggregations []model.MonitorAggregationRule
	if err := s.db.Where("status = ?", 1).Find(&aggregations).Error; err != nil {
		return
	}
	for _, aggregation := range aggregations {
		windowStart := now.Add(-time.Duration(normalizeAggregationWindow(aggregation.WindowSeconds)) * time.Second)
		var candidates []model.MonitorAlertEvent
		if err := s.db.Where("aggregate_rule_id = ? AND aggregation_key <> '' AND status = ? AND silenced = ? AND first_trigger_at <= ?", aggregation.ID, "firing", false, windowStart).
			Order("aggregation_key ASC, first_trigger_at ASC").Find(&candidates).Error; err != nil {
			continue
		}
		groups := make(map[string][]model.MonitorAlertEvent)
		for _, event := range candidates {
			groups[event.AggregationKey] = append(groups[event.AggregationKey], event)
		}
		for key, events := range groups {
			s.flushMonitorAggregationGroup(aggregation, key, events, now)
		}
	}
}

func (s *Service) flushMonitorAggregationGroup(aggregation model.MonitorAggregationRule, key string, events []model.MonitorAlertEvent, now time.Time) {
	if len(events) == 0 {
		return
	}
	representative := events[0]
	var rule model.MonitorAlertRule
	if err := s.db.First(&rule, representative.RuleID).Error; err != nil || rule.Status != 1 || !rule.NotifyEnabled || rule.NotifyRuleID == 0 {
		return
	}
	repeatAfter := time.Duration(normalizeAggregationWindow(aggregation.RepeatIntervalSeconds)) * time.Second
	s.monitorNotifyMu.Lock()
	var latest model.MonitorAlertEvent
	err := s.db.Where("aggregation_key = ? AND status = ? AND last_notify_at IS NOT NULL", key, "firing").Order("last_notify_at DESC").First(&latest).Error
	if err == nil && latest.LastNotifyAt != nil && now.Sub(*latest.LastNotifyAt) < repeatAfter {
		s.monitorNotifyMu.Unlock()
		return
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		s.monitorNotifyMu.Unlock()
		return
	}
	if err := s.db.Model(&model.MonitorAlertEvent{}).Where("aggregation_key = ? AND status = ?", key, "firing").Updates(map[string]any{
		"last_notify_at": &now, "notify_count": gorm.Expr("notify_count + ?", 1),
	}).Error; err != nil {
		s.monitorNotifyMu.Unlock()
		return
	}
	s.monitorNotifyMu.Unlock()

	s.dispatchMonitorAggregationNotification(rule, aggregation, events, now)
	for _, event := range events {
		s.appendMonitorAlertTimeline(event.ID, "aggregation_notification", "aggregated alert notification sent", fmt.Sprintf("aggregation rule: %s; %d alerts in this batch", aggregation.Name, len(events)), "System", map[string]any{
			"aggregationKey": key, "eventCount": len(events), "notifyRuleId": rule.NotifyRuleID,
		})
	}
}

func (s *Service) dispatchMonitorAggregationNotification(rule model.MonitorAlertRule, aggregation model.MonitorAggregationRule, events []model.MonitorAlertEvent, now time.Time) {
	representative := events[0]
	samples := make([]string, 0, min(len(events), 5))
	for index, event := range events {
		if index == 5 {
			break
		}
		samples = append(samples, event.LabelsJSON)
	}
	summary := fmt.Sprintf("[Aggregated Alert] %s: %d related alerts", representative.RuleName, len(events))
	detail := fmt.Sprintf("Aggregation Rule: %s\nWindow: %d seconds\nMatched: %d\nSample Labels:\n%s", aggregation.Name, normalizeAggregationWindow(aggregation.WindowSeconds), len(events), strings.Join(samples, "\n"))
	s.DispatchNotifyRule(rule.NotifyRuleID, NotifyEvent{
		Scope: "monitor", Event: "firing", TargetID: representative.ID, TargetName: representative.RuleName + " (Aggregated)", Status: "firing",
		Summary: summary, Detail: detail, StartedAt: &representative.FirstTriggerAt,
		Extra: map[string]string{
			"alertName": representative.RuleName, "severity": representative.Severity, "aggregationRule": aggregation.Name,
			"aggregationCount": strconv.Itoa(len(events)), "aggregationWindowSeconds": strconv.Itoa(normalizeAggregationWindow(aggregation.WindowSeconds)),
			"labels": representative.LabelsJSON, "datasourceName": representative.DatasourceName, "sentAt": now.Format(time.RFC3339),
		},
	})
}

func (s *Service) appendMonitorAlertTimeline(eventID uint, eventType, title, detail, operator string, metadata map[string]any) {
	if eventID == 0 {
		return
	}
	metadataJSON := "{}"
	if len(metadata) > 0 {
		if data, err := json.Marshal(metadata); err == nil {
			metadataJSON = string(data)
		}
	}
	_ = s.db.Create(&model.MonitorAlertEventTimeline{
		AlertEventID: eventID,
		EventType:    strings.TrimSpace(eventType),
		Title:        strings.TrimSpace(title),
		Detail:       strings.TrimSpace(detail),
		Operator:     strings.TrimSpace(operator),
		MetadataJSON: metadataJSON,
	}).Error
}

func applyMonitorEventAggregation(event *model.MonitorAlertEvent, updates map[string]any, aggregation model.MonitorAggregationRule, key string) {
	updates["aggregation_key"] = key
	updates["aggregate_rule_id"] = aggregation.ID
	updates["aggregate_rule_name"] = aggregation.Name
	event.AggregationKey = key
	event.AggregateRuleID = aggregation.ID
	event.AggregateRuleName = aggregation.Name
}

func (s *Service) upsertMonitorAlertEvent(rule model.MonitorAlertRule, sample PromMetricSample, fp string, value float64) {
	now := time.Now()
	labelsBytes, _ := json.Marshal(sample.Metric)
	summary := fmt.Sprintf("%s current value %.4f %s %.4f", rule.Name, value, rule.Comparator, rule.Threshold)
	silenceRule, silenced := s.matchMonitorSilenceRule(rule, sample.Metric)
	aggregationRule, aggregationKey, aggregated := s.matchMonitorAggregationRule(rule, sample.Metric)
	var existing model.MonitorAlertEvent
	err := s.db.Where("rule_id = ? AND fingerprint = ? AND status IN ?", rule.ID, fp, []string{"pending", "firing", "claimed", "silenced"}).First(&existing).Error
	if err == nil {
		previousAggregateRuleID := existing.AggregateRuleID
		updates := map[string]any{
			"current_value": value, "last_trigger_at": now, "summary": summary,
		}
		shouldNotify := false
		if existing.Status == "pending" && !silenced && rule.ForSeconds > 0 && now.Sub(existing.FirstTriggerAt) >= time.Duration(rule.ForSeconds)*time.Second {
			updates["status"] = "firing"
			shouldNotify = true
		}
		if silenced && silenceRule != nil {
			updates["silenced"] = true
			updates["silence_rule_id"] = silenceRule.ID
			updates["silence_rule_name"] = silenceRule.Name
			updates["status"] = "silenced"
		} else if existing.Status == "silenced" {
			updates["silenced"] = false
			updates["silence_rule_id"] = 0
			updates["silence_rule_name"] = ""
			updates["status"] = "firing"
			shouldNotify = true
		}
		if aggregated && aggregationRule != nil {
			applyMonitorEventAggregation(&existing, updates, *aggregationRule, aggregationKey)
		}
		previousStatus := existing.Status
		if err := s.db.Model(&model.MonitorAlertEvent{}).Where("id = ?", existing.ID).Updates(updates).Error; err != nil {
			return
		}
		if nextStatus, ok := updates["status"].(string); ok && nextStatus != previousStatus {
			switch nextStatus {
			case "firing":
				if previousStatus == "silenced" {
					s.appendMonitorAlertTimeline(existing.ID, "firing", "silence ended while alert is still firing", summary, "System", nil)
				} else {
					s.appendMonitorAlertTimeline(existing.ID, "firing", "alert entered firing state", summary, "System", nil)
				}
			case "silenced":
				s.appendMonitorAlertTimeline(existing.ID, "silenced", "alert matched a silence rule", firstNonEmpty(existing.SilenceRuleName, silenceRule.Name), "System", nil)
			}
		}
		if aggregated && aggregationRule != nil && previousAggregateRuleID != aggregationRule.ID {
			s.appendMonitorAlertTimeline(existing.ID, "aggregated", "alert matched an aggregation rule", aggregationRule.Name, "System", map[string]any{"aggregationKey": aggregationKey})
		}
		if existing.Status == "firing" && !silenced {
			shouldNotify = true
		}
		if shouldNotify && rule.NotifyEnabled && rule.NotifyRuleID > 0 {
			existing.Status = "firing"
			existing.CurrentValue = value
			existing.Summary = summary
			s.notifyMonitorAlertIfAllowed(&existing, rule, aggregationRule, "firing")
		}
		return
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return
	}
	event := model.MonitorAlertEvent{
		RuleID: rule.ID, RuleName: rule.Name, DatasourceID: rule.DatasourceID, DatasourceName: rule.DatasourceName,
		Fingerprint: fp, Severity: rule.Severity, Status: "firing", Metric: firstNonEmpty(sample.Metric["__name__"], rule.PromQL),
		LabelsJSON: string(labelsBytes), AnnotationsJSON: rule.AnnotationsJSON, CurrentValue: value, Threshold: rule.Threshold,
		Summary: summary, FirstTriggerAt: now, LastTriggerAt: now,
	}
	if silenced && silenceRule != nil {
		event.Status = "silenced"
		event.Silenced = true
		event.SilenceRuleID = silenceRule.ID
		event.SilenceRuleName = silenceRule.Name
	}
	if !event.Silenced && rule.ForSeconds > 0 {
		event.Status = "pending"
	}
	if aggregated && aggregationRule != nil {
		event.AggregationKey = aggregationKey
		event.AggregateRuleID = aggregationRule.ID
		event.AggregateRuleName = aggregationRule.Name
	}
	if err := s.db.Create(&event).Error; err == nil {
		title := "alert event created"
		if event.Status == "pending" {
			title = "waiting for duration threshold"
		} else if event.Status == "silenced" {
			title = "alert silenced"
		}
		s.appendMonitorAlertTimeline(event.ID, event.Status, title, summary, "System", map[string]any{"fingerprint": event.Fingerprint})
		if aggregated && aggregationRule != nil {
			s.appendMonitorAlertTimeline(event.ID, "aggregated", "alert matched an aggregation rule", aggregationRule.Name, "System", map[string]any{"aggregationKey": aggregationKey})
		}
		if event.Status == "firing" && !event.Silenced && rule.NotifyEnabled && rule.NotifyRuleID > 0 {
			s.notifyMonitorAlertIfAllowed(&event, rule, aggregationRule, "firing")
		}
	}
}

func (s *Service) recoverInactiveMonitorEvents(rule model.MonitorAlertRule, active map[string]bool) {
	var events []model.MonitorAlertEvent
	if err := s.db.Where("rule_id = ? AND status IN ?", rule.ID, []string{"pending", "firing", "claimed", "silenced"}).Find(&events).Error; err != nil {
		return
	}
	now := time.Now()
	for _, event := range events {
		if active[event.Fingerprint] {
			continue
		}
		wasPending := event.Status == "pending"
		_ = s.db.Model(&model.MonitorAlertEvent{}).Where("id = ?", event.ID).Updates(map[string]any{
			"status": "recovered", "recovered_at": &now,
		}).Error
		if wasPending {
			s.appendMonitorAlertTimeline(event.ID, "recovered", "duration wait cancelled", "alert condition recovered before duration threshold; no recovery notification sent", "System", nil)
		} else {
			s.appendMonitorAlertTimeline(event.ID, "recovered", "alert recovered automatically", "metric no longer matches the alert condition", "System", nil)
		}
		event.Status = "recovered"
		event.RecoveredAt = &now
		// A recovery notification must correspond to an alert that emitted a firing notification. A pending event
		// that recovers during its duration window emitted no external alert and must not create an orphan recovery message.
		if shouldNotifyMonitorRecovery(rule, event) {
			if event.AggregateRuleID == 0 || event.AggregationKey == "" {
				s.dispatchMonitorNotification(rule, event, "recovered")
			} else {
				s.notifyMonitorAggregationRecovered(rule, event)
			}
		}
	}
}

func shouldNotifyMonitorRecovery(rule model.MonitorAlertRule, event model.MonitorAlertEvent) bool {
	return rule.NotifyEnabled && rule.NotifyRuleID > 0 && rule.NotifyRecoveryEnabled &&
		(event.LastNotifyAt != nil || event.NotifyCount > 0)
}

// A recovered event in an aggregation bucket only notifies when the final
// firing event in that bucket has recovered, preventing a recovery storm.
func (s *Service) notifyMonitorAggregationRecovered(rule model.MonitorAlertRule, event model.MonitorAlertEvent) {
	s.monitorNotifyMu.Lock()
	defer s.monitorNotifyMu.Unlock()
	var activeCount int64
	if err := s.db.Model(&model.MonitorAlertEvent{}).Where("aggregation_key = ? AND status IN ?", event.AggregationKey, []string{"pending", "firing", "claimed"}).Count(&activeCount).Error; err != nil || activeCount > 0 {
		return
	}
	s.DispatchNotifyRule(rule.NotifyRuleID, NotifyEvent{
		Scope: "monitor", Event: "recovered", TargetID: event.ID, TargetName: event.RuleName + " (Aggregated)", Status: "recovered",
		Summary: fmt.Sprintf("[Aggregated Recovery] %s: all alerts in the group recovered", event.RuleName), Detail: event.LabelsJSON,
		StartedAt: &event.FirstTriggerAt, FinishedAt: event.RecoveredAt,
		Extra: map[string]string{"alertName": event.RuleName, "severity": event.Severity, "aggregationRule": event.AggregateRuleName, "labels": event.LabelsJSON},
	})
	s.appendMonitorAlertTimeline(event.ID, "aggregation_recovered", "aggregated recovery notification sent", "all alerts in the aggregation group recovered", "System", map[string]any{"aggregationKey": event.AggregationKey})
}

func (s *Service) dispatchMonitorNotification(rule model.MonitorAlertRule, event model.MonitorAlertEvent, status string) {
	s.DispatchNotifyRule(rule.NotifyRuleID, NotifyEvent{
		Scope: "monitor", Event: status, TargetID: event.ID, TargetName: event.RuleName, Status: status,
		Summary: event.Summary, Detail: event.LabelsJSON, StartedAt: &event.FirstTriggerAt, FinishedAt: event.RecoveredAt,
		Extra: map[string]string{
			"alertName": event.RuleName, "severity": event.Severity, "instance": extractLabel(event.LabelsJSON, "instance"),
			"value": fmt.Sprintf("%.4f", event.CurrentValue), "threshold": fmt.Sprintf("%.4f", event.Threshold),
			"labels": event.LabelsJSON, "annotations": event.AnnotationsJSON, "datasourceName": event.DatasourceName,
		},
	})
}

func extractLabel(raw, key string) string {
	var labels map[string]string
	_ = json.Unmarshal([]byte(raw), &labels)
	return labels[key]
}

func (s *Service) ListMonitorAlertEvents(pageNum, pageSize int, keyword, status, severity string) (map[string]any, error) {
	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	query := s.db.Model(&model.MonitorAlertEvent{})
	if strings.TrimSpace(keyword) != "" {
		like := "%" + strings.TrimSpace(keyword) + "%"
		query = query.Where("rule_name LIKE ? OR metric LIKE ? OR summary LIKE ? OR labels_json LIKE ?", like, like, like, like)
	}
	if strings.TrimSpace(status) != "" {
		query = query.Where("status = ?", status)
	}
	if strings.TrimSpace(severity) != "" {
		query = query.Where("severity = ?", normalizeSeverity(severity))
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var list []model.MonitorAlertEvent
	if err := query.Order("last_trigger_at DESC, id DESC").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, err
	}
	return map[string]any{"list": list, "total": total, "pageNum": pageNum, "pageSize": pageSize}, nil
}

func (s *Service) GetMonitorAlertEventDetail(id uint) (map[string]any, error) {
	var event model.MonitorAlertEvent
	if err := s.db.First(&event, id).Error; err != nil {
		return nil, err
	}
	var timelines []model.MonitorAlertEventTimeline
	if err := s.db.Where("alert_event_id = ?", id).Order("created_at ASC, id ASC").Find(&timelines).Error; err != nil {
		return nil, err
	}
	if len(timelines) == 0 {
		timelines = append(timelines, model.MonitorAlertEventTimeline{
			AlertEventID: id, EventType: "firing", Title: "alert event created", Detail: event.Summary, Operator: "System", CreatedAt: event.FirstTriggerAt,
		})
		if event.ClaimedBy != "" {
			claimedAt := event.UpdatedAt
			if event.ClaimedAt != nil {
				claimedAt = *event.ClaimedAt
			}
			timelines = append(timelines, model.MonitorAlertEventTimeline{
				AlertEventID: id, EventType: "claimed", Title: "alert claimed", Detail: event.HandleNote, Operator: event.ClaimedBy, CreatedAt: claimedAt,
			})
		}
		if event.RecoveredAt != nil {
			title := "alert recovered automatically"
			if event.Status == "resolved" {
				title = "alert closed manually"
			}
			timelines = append(timelines, model.MonitorAlertEventTimeline{
				AlertEventID: id, EventType: event.Status, Title: title, Detail: event.ResolveNote, Operator: "System", CreatedAt: *event.RecoveredAt,
			})
		}
	}
	var actions []model.MonitorAlertAction
	if err := s.db.Where("alert_event_id = ?", id).Order("id DESC").Find(&actions).Error; err != nil {
		return nil, err
	}
	var notifyLogs []model.NotifySendLog
	if err := s.db.Where("scope = ? AND target_id = ?", "monitor", id).Order("id DESC").Limit(20).Find(&notifyLogs).Error; err != nil {
		return nil, err
	}

	var rule model.MonitorAlertRule
	_ = s.db.First(&rule, event.RuleID).Error
	notificationState := map[string]any{
		"allowed": true, "reason": "send is currently allowed", "notifyCount": event.NotifyCount,
		"maxNotifyCount": rule.MaxNotifyCount, "lastNotifyAt": event.LastNotifyAt,
	}
	if event.Status == "recovered" || event.Status == "resolved" {
		notificationState["allowed"] = false
		notificationState["reason"] = "alert has ended; no further notification is required"
	} else if event.Silenced {
		notificationState["allowed"] = false
		notificationState["reason"] = "matched silence rule: " + firstNonEmpty(event.SilenceRuleName, "Unnamed Rule")
	} else if rule.MaxNotifyCount > 0 && event.NotifyCount >= rule.MaxNotifyCount {
		notificationState["allowed"] = false
		notificationState["reason"] = "maximum send count reached"
	} else if event.LastNotifyAt != nil {
		nextNotifyAt := event.LastNotifyAt.Add(time.Duration(normalizeNotifyRepeatInterval(rule.NotifyRepeatIntervalSeconds)) * time.Second)
		notificationState["nextNotifyAt"] = nextNotifyAt
		if time.Now().Before(nextNotifyAt) {
			notificationState["allowed"] = false
			notificationState["reason"] = "waiting for repeat-notification interval"
		}
	}
	if event.AggregateRuleID > 0 {
		notificationState["aggregationRule"] = event.AggregateRuleName
		notificationState["aggregationKey"] = event.AggregationKey
	}
	return map[string]any{
		"event": event, "timelines": timelines, "actions": actions,
		"notifyLogs": notifyLogs, "notificationState": notificationState,
	}, nil
}

func (s *Service) ClaimMonitorAlertEvent(payload MonitorAlertEventActionPayload) error {
	if payload.ID == 0 {
		return errors.New("alert event ID is required")
	}
	now := time.Now()
	if err := s.db.Model(&model.MonitorAlertEvent{}).Where("id = ?", payload.ID).Updates(map[string]any{
		"status": "claimed", "claimed_by": strings.TrimSpace(payload.ClaimedBy), "claimed_at": &now, "handle_note": strings.TrimSpace(payload.HandleNote),
	}).Error; err != nil {
		return err
	}
	s.appendMonitorAlertTimeline(payload.ID, "claimed", "alert claimed", payload.HandleNote, payload.ClaimedBy, nil)
	return nil
}

func (s *Service) ResolveMonitorAlertEvent(payload MonitorAlertEventActionPayload) error {
	if payload.ID == 0 {
		return errors.New("alert event ID is required")
	}
	now := time.Now()
	if err := s.db.Model(&model.MonitorAlertEvent{}).Where("id = ?", payload.ID).Updates(map[string]any{
		"status": "resolved", "resolve_note": strings.TrimSpace(payload.HandleNote), "recovered_at": &now, "resolved_at": &now,
	}).Error; err != nil {
		return err
	}
	s.appendMonitorAlertTimeline(payload.ID, "resolved", "alert closed manually", payload.HandleNote, "Operator", nil)
	return nil
}

func (s *Service) BatchUpdateMonitorAlertEvents(payload MonitorAlertEventBatchPayload) error {
	ids, err := normalizeMonitorBatchIDs(payload.IDs)
	if err != nil {
		return err
	}
	action := strings.ToLower(strings.TrimSpace(payload.Action))
	updates := map[string]any{}
	query := s.db.Model(&model.MonitorAlertEvent{}).Where("id IN ?", ids)
	switch action {
	case "claim":
		now := time.Now()
		updates["status"] = "claimed"
		updates["claimed_by"] = strings.TrimSpace(payload.ClaimedBy)
		updates["claimed_at"] = &now
		updates["handle_note"] = strings.TrimSpace(payload.HandleNote)
		query = query.Where("status IN ?", []string{"pending", "firing"})
	case "resolve":
		now := time.Now()
		updates["status"] = "resolved"
		updates["resolve_note"] = strings.TrimSpace(payload.HandleNote)
		updates["recovered_at"] = &now
		updates["resolved_at"] = &now
		query = query.Where("status IN ?", []string{"pending", "firing", "claimed", "silenced"})
	case "delete":
		return s.db.Where("id IN ?", ids).Delete(&model.MonitorAlertEvent{}).Error
	default:
		return errors.New("unsupported batch operation")
	}
	result := query.Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		if action == "claim" {
			return errors.New("selected events contain no pending-duration or firing alerts that can be claimed")
		}
		return errors.New("selected events contain no open alerts that can be closed")
	}
	for _, id := range ids {
		if action == "claim" {
			s.appendMonitorAlertTimeline(id, "claimed", "alerts claimed in batch", payload.HandleNote, payload.ClaimedBy, nil)
		} else if action == "resolve" {
			s.appendMonitorAlertTimeline(id, "resolved", "alerts closed in batch", payload.HandleNote, payload.ClaimedBy, nil)
		}
	}
	return nil
}
