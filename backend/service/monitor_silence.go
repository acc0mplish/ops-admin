package service

import (
	"errors"
	"strings"

	"ops-admin/backend/model"
)

func (s *Service) ListMonitorSilenceRules(pageNum, pageSize int, keyword, status string) (map[string]any, error) {
	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	query := s.db.Model(&model.MonitorSilenceRule{})
	if strings.TrimSpace(keyword) != "" {
		like := "%" + strings.TrimSpace(keyword) + "%"
		query = query.Where("name LIKE ? OR rule_name_pattern LIKE ? OR description LIKE ?", like, like, like)
	}
	if strings.TrimSpace(status) != "" {
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var list []model.MonitorSilenceRule
	if err := query.Order("id DESC").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, err
	}
	rows := make([]map[string]any, 0, len(list))
	for _, item := range list {
		rows = append(rows, map[string]any{
			"id": item.ID, "name": item.Name, "matchMode": firstNonEmpty(item.MatchMode, "regex"),
			"ruleIds": decodeUintList(item.RuleIDsJSON), "ruleIdsJson": item.RuleIDsJSON,
			"ruleNamePattern": item.RuleNamePattern, "severity": item.Severity, "matchersJson": item.MatchersJSON,
			"startsAt": item.StartsAt, "endsAt": item.EndsAt, "priority": item.Priority, "status": item.Status, "description": item.Description,
			"createTime": item.CreatedAt, "updateTime": item.UpdatedAt,
		})
	}
	return map[string]any{"list": rows, "total": total, "pageNum": pageNum, "pageSize": pageSize}, nil
}

func (s *Service) GetMonitorSilenceRule(id uint) (map[string]any, error) {
	var item model.MonitorSilenceRule
	if err := s.db.First(&item, id).Error; err != nil {
		return nil, err
	}
	return map[string]any{
		"id": item.ID, "name": item.Name, "matchMode": firstNonEmpty(item.MatchMode, "regex"),
		"ruleIds": decodeUintList(item.RuleIDsJSON), "ruleIdsJson": item.RuleIDsJSON,
		"ruleNamePattern": item.RuleNamePattern, "severity": item.Severity, "alertType": item.AlertType, "matchersJson": item.MatchersJSON,
		"startsAt": item.StartsAt, "endsAt": item.EndsAt, "priority": item.Priority, "status": item.Status, "description": item.Description,
		"createTime": item.CreatedAt, "updateTime": item.UpdatedAt,
	}, nil
}

func (s *Service) SaveMonitorSilenceRule(payload MonitorSilenceRulePayload) error {
	if strings.TrimSpace(payload.Name) == "" {
		return errors.New("silence rule name is required")
	}
	if payload.StartsAt > 0 && payload.EndsAt > 0 && payload.EndsAt <= payload.StartsAt {
		return errors.New("end time must be later than start time")
	}
	if normalizeRuleMatchMode(payload.MatchMode) == "regex" && strings.TrimSpace(payload.RuleNamePattern) == "" {
		return errors.New("rule-name regular expression is required")
	}
	matchersJSON, err := normalizeMatcherJSON(payload.MatchersJSON)
	if err != nil {
		return err
	}
	updates := map[string]any{
		"name":              Trimmed(payload.Name),
		"match_mode":        normalizeRuleMatchMode(payload.MatchMode),
		"rule_ids_json":     encodeUintList(payload.RuleIDs),
		"rule_name_pattern": strings.TrimSpace(payload.RuleNamePattern),
		"severity":          strings.TrimSpace(payload.Severity),
		"alert_type":        strings.TrimSpace(payload.AlertType),
		"matchers_json":     matchersJSON,
		"starts_at":         unixPtr(payload.StartsAt),
		"ends_at":           unixPtr(payload.EndsAt),
		"priority":          normalizeSilencePriority(payload.Priority),
		"status":            normalizeMonitorStatus(payload.Status),
		"description":       Trimmed(payload.Description),
	}
	if payload.ID > 0 {
		return s.db.Model(&model.MonitorSilenceRule{}).Where("id = ?", payload.ID).Updates(updates).Error
	}
	return s.db.Model(&model.MonitorSilenceRule{}).Create(updates).Error
}

func (s *Service) PreviewMonitorSilenceRule(payload MonitorSilenceRulePayload) (map[string]any, error) {
	if strings.TrimSpace(payload.Name) == "" {
		return nil, errors.New("silence rule name is required")
	}
	matchersJSON, err := normalizeMatcherJSON(payload.MatchersJSON)
	if err != nil {
		return nil, err
	}
	if normalizeRuleMatchMode(payload.MatchMode) == "regex" && strings.TrimSpace(payload.RuleNamePattern) == "" {
		return nil, errors.New("rule-name regular expression is required")
	}
	preview := model.MonitorSilenceRule{
		MatchMode: normalizeRuleMatchMode(payload.MatchMode), RuleIDsJSON: encodeUintList(payload.RuleIDs),
		RuleNamePattern: strings.TrimSpace(payload.RuleNamePattern), Severity: strings.TrimSpace(payload.Severity), AlertType: strings.TrimSpace(payload.AlertType), MatchersJSON: matchersJSON,
	}
	var rules []model.MonitorAlertRule
	if err := s.db.Order("id DESC").Find(&rules).Error; err != nil {
		return nil, err
	}
	matchedRules := make([]map[string]any, 0)
	matchedRuleIDs := map[uint]bool{}
	for _, rule := range rules {
		if !monitorSilenceRuleCriteriaMatch(preview, rule, nil) {
			continue
		}
		matchedRuleIDs[rule.ID] = true
		if len(matchedRules) < 20 {
			matchedRules = append(matchedRules, map[string]any{"id": rule.ID, "name": rule.Name, "severity": rule.Severity, "alertType": rule.AlertType})
		}
	}
	var events []model.MonitorAlertEvent
	if err := s.db.Where("status IN ?", []string{"pending", "firing", "claimed", "silenced"}).Order("last_trigger_at DESC").Find(&events).Error; err != nil {
		return nil, err
	}
	matchedEvents := make([]map[string]any, 0)
	for _, event := range events {
		if !matchedRuleIDs[event.RuleID] {
			continue
		}
		var rule model.MonitorAlertRule
		if err := s.db.First(&rule, event.RuleID).Error; err != nil {
			continue
		}
		labels := decodeLabelMap(event.LabelsJSON)
		if !monitorSilenceRuleCriteriaMatch(preview, rule, labels) {
			continue
		}
		if len(matchedEvents) < 20 {
			matchedEvents = append(matchedEvents, map[string]any{"id": event.ID, "ruleName": event.RuleName, "severity": event.Severity, "status": event.Status, "summary": event.Summary})
		}
	}
	return map[string]any{
		"matchedRuleCount": len(matchedRuleIDs), "matchedRules": matchedRules,
		"matchedActiveEventCount": len(matchedEvents), "matchedActiveEvents": matchedEvents,
	}, nil
}

func (s *Service) DeleteMonitorSilenceRule(id uint) error {
	return s.db.Delete(&model.MonitorSilenceRule{}, id).Error
}

func normalizeMonitorBatchIDs(ids []uint) ([]uint, error) {
	seen := map[uint]bool{}
	result := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id > 0 && !seen[id] {
			seen[id] = true
			result = append(result, id)
		}
	}
	if len(result) == 0 {
		return nil, errors.New("select at least one rule")
	}
	return result, nil
}

func (s *Service) BatchUpdateMonitorSilenceRules(payload MonitorRuleBatchPayload) error {
	ids, err := normalizeMonitorBatchIDs(payload.IDs)
	if err != nil {
		return err
	}
	switch strings.ToLower(strings.TrimSpace(payload.Action)) {
	case "enable":
		return s.db.Model(&model.MonitorSilenceRule{}).Where("id IN ?", ids).Update("status", 1).Error
	case "disable":
		return s.db.Model(&model.MonitorSilenceRule{}).Where("id IN ?", ids).Update("status", 2).Error
	case "delete":
		return s.db.Where("id IN ?", ids).Delete(&model.MonitorSilenceRule{}).Error
	default:
		return errors.New("unsupported batch operation")
	}
}

func (s *Service) ListMonitorAggregationRules(pageNum, pageSize int, keyword, status string) (map[string]any, error) {
	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	query := s.db.Model(&model.MonitorAggregationRule{})
	if strings.TrimSpace(keyword) != "" {
		like := "%" + strings.TrimSpace(keyword) + "%"
		query = query.Where("name LIKE ? OR rule_name_pattern LIKE ? OR description LIKE ?", like, like, like)
	}
	if strings.TrimSpace(status) != "" {
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var list []model.MonitorAggregationRule
	if err := query.Order("id DESC").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, err
	}
	rows := make([]map[string]any, 0, len(list))
	for _, item := range list {
		rows = append(rows, map[string]any{
			"id": item.ID, "name": item.Name, "matchMode": firstNonEmpty(item.MatchMode, "regex"),
			"ruleIds": decodeUintList(item.RuleIDsJSON), "ruleIdsJson": item.RuleIDsJSON,
			"ruleNamePattern": item.RuleNamePattern, "severity": item.Severity,
			"groupBy": decodeStringList(item.GroupByJSON), "groupByJson": item.GroupByJSON,
			"windowSeconds": item.WindowSeconds, "repeatIntervalSeconds": item.RepeatIntervalSeconds,
			"status": item.Status, "description": item.Description, "createTime": item.CreatedAt, "updateTime": item.UpdatedAt,
		})
	}
	return map[string]any{"list": rows, "total": total, "pageNum": pageNum, "pageSize": pageSize}, nil
}

func (s *Service) GetMonitorAggregationRule(id uint) (map[string]any, error) {
	var item model.MonitorAggregationRule
	if err := s.db.First(&item, id).Error; err != nil {
		return nil, err
	}
	return map[string]any{
		"id": item.ID, "name": item.Name, "matchMode": firstNonEmpty(item.MatchMode, "regex"),
		"ruleIds": decodeUintList(item.RuleIDsJSON), "ruleIdsJson": item.RuleIDsJSON,
		"ruleNamePattern": item.RuleNamePattern, "severity": item.Severity, "alertType": item.AlertType,
		"groupBy": decodeStringList(item.GroupByJSON), "groupByJson": item.GroupByJSON,
		"windowSeconds": item.WindowSeconds, "repeatIntervalSeconds": item.RepeatIntervalSeconds,
		"status": item.Status, "description": item.Description, "createTime": item.CreatedAt, "updateTime": item.UpdatedAt,
	}, nil
}

func (s *Service) SaveMonitorAggregationRule(payload MonitorAggregationRulePayload) error {
	if strings.TrimSpace(payload.Name) == "" {
		return errors.New("aggregation rule name is required")
	}
	// An empty list is intentional: it means aggregate all samples of the
	// matched rule and severity into one bucket, regardless of their labels.
	groupBy := payload.GroupBy
	updates := map[string]any{
		"name":                    Trimmed(payload.Name),
		"match_mode":              normalizeRuleMatchMode(payload.MatchMode),
		"rule_ids_json":           encodeUintList(payload.RuleIDs),
		"rule_name_pattern":       strings.TrimSpace(payload.RuleNamePattern),
		"severity":                strings.TrimSpace(payload.Severity),
		"alert_type":              strings.TrimSpace(payload.AlertType),
		"group_by_json":           encodeStringList(groupBy),
		"window_seconds":          normalizeAggregationWindow(payload.WindowSeconds),
		"repeat_interval_seconds": normalizeAggregationWindow(payload.RepeatIntervalSeconds),
		"status":                  normalizeMonitorStatus(payload.Status),
		"description":             Trimmed(payload.Description),
	}
	if payload.ID > 0 {
		return s.db.Model(&model.MonitorAggregationRule{}).Where("id = ?", payload.ID).Updates(updates).Error
	}
	return s.db.Model(&model.MonitorAggregationRule{}).Create(updates).Error
}

func (s *Service) DeleteMonitorAggregationRule(id uint) error {
	return s.db.Delete(&model.MonitorAggregationRule{}, id).Error
}

func (s *Service) BatchUpdateMonitorAggregationRules(payload MonitorRuleBatchPayload) error {
	ids, err := normalizeMonitorBatchIDs(payload.IDs)
	if err != nil {
		return err
	}
	switch strings.ToLower(strings.TrimSpace(payload.Action)) {
	case "enable":
		return s.db.Model(&model.MonitorAggregationRule{}).Where("id IN ?", ids).Update("status", 1).Error
	case "disable":
		return s.db.Model(&model.MonitorAggregationRule{}).Where("id IN ?", ids).Update("status", 2).Error
	case "delete":
		return s.db.Where("id IN ?", ids).Delete(&model.MonitorAggregationRule{}).Error
	default:
		return errors.New("unsupported batch operation")
	}
}
