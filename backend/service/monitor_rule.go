package service

import (
	"errors"
	"strings"
	"time"

	"ops-admin/backend/model"

	"gorm.io/gorm"
)

func (s *Service) ListMonitorAlertRules(pageNum, pageSize int, keyword, status, severity, env, alertType string) (map[string]any, error) {
	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	query := s.db.Model(&model.MonitorAlertRule{})
	if strings.TrimSpace(keyword) != "" {
		like := "%" + strings.TrimSpace(keyword) + "%"
		query = query.Where("name LIKE ? OR prom_ql LIKE ? OR description LIKE ?", like, like, like)
	}
	if strings.TrimSpace(status) != "" {
		switch strings.TrimSpace(status) {
		case "unclaimed":
			query = query.Where("status IN ? AND (claimed_by = '' OR claimed_by IS NULL)", []string{"pending", "firing"})
		default:
			query = query.Where("status = ?", status)
		}
	}
	if strings.TrimSpace(severity) != "" {
		if strings.EqualFold(strings.TrimSpace(severity), "critical") {
			query = query.Where("severity IN ?", []string{"P0", "P1"})
		} else {
			query = query.Where("severity = ?", normalizeSeverity(severity))
		}
	}
	if strings.TrimSpace(env) != "" {
		query = query.Where("env = ?", normalizeEnvCode(env))
	}
	if strings.TrimSpace(alertType) != "" {
		query = query.Where("alert_type = ?", normalizeAlertType(alertType))
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var list []model.MonitorAlertRule
	if err := query.Order("id DESC").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, err
	}
	return map[string]any{"list": list, "total": total, "pageNum": pageNum, "pageSize": pageSize}, nil
}

func (s *Service) GetMonitorAlertRule(id uint) (*model.MonitorAlertRule, error) {
	var item model.MonitorAlertRule
	if err := s.db.First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Service) SaveMonitorAlertRule(payload MonitorAlertRulePayload) error {
	if strings.TrimSpace(payload.Name) == "" {
		return errors.New("rule name is required")
	}
	alertType := normalizeAlertType(payload.AlertType)
	datasourceScope := normalizeDatasourceScope(payload.DatasourceScope)
	queryText := firstNonEmpty(strings.TrimSpace(payload.Query), strings.TrimSpace(payload.PromQL))
	if queryText == "" {
		if isMonitorLogAlertType(alertType) {
			return errors.New("Elasticsearch query is required")
		}
		return errors.New("PromQL is required")
	}
	labelsJSON, err := ensureJSONObject(payload.LabelsJSON)
	if err != nil {
		return err
	}
	annotationsJSON, err := ensureJSONObject(payload.AnnotationsJSON)
	if err != nil {
		return err
	}
	datasourceName := ""
	datasourceID := payload.DatasourceID
	if datasourceScope == "specific" {
		if datasourceID == 0 {
			return errors.New("select a datasource")
		}
		ds, err := s.GetMonitorDatasource(datasourceID)
		if err != nil {
			return err
		}
		if alertType == "log" && normalizeMonitorDatasourceType(ds.Type) != "elasticsearch" {
			return errors.New("log alerts require an Elasticsearch datasource")
		}
		if alertType == "victorialogs" && normalizeMonitorDatasourceType(ds.Type) != "victorialogs" {
			return errors.New("VictoriaLogs alerts require a VictoriaLogs datasource")
		}
		if alertType == "metric" && !isMonitorMetricDatasource(ds.Type) {
			return errors.New("metric alerts require a Prometheus or VictoriaMetrics datasource")
		}
		datasourceName = ds.Name
	} else {
		var count int64
		if alertType == "log" {
			err = s.db.Model(&model.MonitorDatasource{}).Where("status = ? AND type = ?", 1, "elasticsearch").Count(&count).Error
			datasourceName = "All Log Datasources"
		} else if alertType == "victorialogs" {
			err = s.db.Model(&model.MonitorDatasource{}).Where("status = ? AND type = ?", 1, "victorialogs").Count(&count).Error
			datasourceName = "All VictoriaLogs Datasources"
		} else {
			err = s.db.Model(&model.MonitorDatasource{}).Where("status = ? AND type IN ?", 1, []string{"prometheus", "victoriametrics"}).Count(&count).Error
			datasourceName = "All Metric Datasources"
		}
		if err != nil {
			return err
		}
		if count == 0 {
			return errors.New("no matching datasource is available")
		}
		datasourceID = 0
	}
	updates := map[string]any{
		"name":                           Trimmed(payload.Name),
		"alert_type":                     alertType,
		"datasource_scope":               datasourceScope,
		"datasource_id":                  datasourceID,
		"datasource_name":                datasourceName,
		"prom_ql":                        queryText,
		"log_index":                      firstNonEmpty(strings.TrimSpace(payload.LogIndex), "_all"),
		"log_time_range_seconds":         normalizeLogTimeRangeSeconds(payload.LogTimeRangeSeconds),
		"comparator":                     normalizeComparator(payload.Comparator),
		"threshold":                      payload.Threshold,
		"for_seconds":                    normalizeForSeconds(payload.ForSeconds),
		"eval_interval_seconds":          normalizeEvalInterval(payload.EvalIntervalSeconds),
		"notify_repeat_interval_seconds": normalizeNotifyRepeatInterval(payload.NotifyRepeatIntervalSeconds),
		"max_notify_count":               normalizeMaxNotifyCount(payload.MaxNotifyCount),
		"severity":                       normalizeSeverity(payload.Severity),
		"labels_json":                    labelsJSON,
		"annotations_json":               annotationsJSON,
		"notify_enabled":                 payload.NotifyEnabled,
		"notify_rule_id":                 payload.NotifyRuleID,
		"notify_recovery_enabled":        payload.NotifyRecoveryEnabled,
		"env":                            normalizeEnvCode(payload.Env),
		"status":                         normalizeMonitorStatus(payload.Status),
		"description":                    Trimmed(payload.Description),
	}
	var current model.MonitorAlertRule
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if payload.ID > 0 {
			if err := tx.Model(&model.MonitorAlertRule{}).Where("id = ?", payload.ID).Updates(updates).Error; err != nil {
				return err
			}
			return tx.First(&current, payload.ID).Error
		}
		if err := tx.Model(&model.MonitorAlertRule{}).Create(updates).Error; err != nil {
			return err
		}
		return tx.Last(&current).Error
	})
	if err != nil {
		return err
	}
	if current.Status == 1 {
		return s.registerMonitorAlertRule(current)
	}
	s.removeMonitorAlertRule(current.ID)
	s.closeActiveMonitorAlertEventsForRule(current.ID, "alert rule disabled; open events were closed automatically")
	return nil
}

func (s *Service) DeleteMonitorAlertRule(id uint) error {
	s.removeMonitorAlertRule(id)
	return s.db.Delete(&model.MonitorAlertRule{}, id).Error
}

func (s *Service) UpdateMonitorAlertRuleStatus(id uint, status int) error {
	status = normalizeMonitorStatus(status)
	if err := s.db.Model(&model.MonitorAlertRule{}).Where("id = ?", id).Update("status", status).Error; err != nil {
		return err
	}
	rule, err := s.GetMonitorAlertRule(id)
	if err != nil {
		return err
	}
	if status == 1 {
		return s.registerMonitorAlertRule(*rule)
	}
	s.removeMonitorAlertRule(id)
	s.closeActiveMonitorAlertEventsForRule(id, "alert rule disabled; open events were closed automatically")
	return nil
}

// Disabling a rule is terminal for its in-flight events. Keeping them firing
// would let a later aggregation flush notify an alert whose rule is inactive.
func (s *Service) closeActiveMonitorAlertEventsForRule(ruleID uint, reason string) {
	if ruleID == 0 || s.db == nil {
		return
	}
	var events []model.MonitorAlertEvent
	if err := s.db.Where("rule_id = ? AND status IN ?", ruleID, []string{"pending", "firing", "claimed", "silenced"}).Find(&events).Error; err != nil || len(events) == 0 {
		return
	}
	now := time.Now()
	if err := s.db.Model(&model.MonitorAlertEvent{}).Where("id IN ?", monitorAlertEventIDs(events)).Updates(map[string]any{
		"status": "resolved", "resolved_at": &now, "resolve_note": reason,
	}).Error; err != nil {
		return
	}
	for _, event := range events {
		s.appendMonitorAlertTimeline(event.ID, "resolved", "alert rule disabled; event closed automatically", reason, "System", nil)
	}
}

func (s *Service) closeInactiveMonitorAlertRuleEvents() {
	if s.db == nil {
		return
	}
	var rules []model.MonitorAlertRule
	if err := s.db.Select("id").Where("status <> ?", 1).Find(&rules).Error; err != nil {
		return
	}
	for _, rule := range rules {
		s.closeActiveMonitorAlertEventsForRule(rule.ID, "alert rule disabled; open events were closed automatically")
	}
}

func monitorAlertEventIDs(events []model.MonitorAlertEvent) []uint {
	ids := make([]uint, 0, len(events))
	for _, event := range events {
		ids = append(ids, event.ID)
	}
	return ids
}

func (s *Service) BatchUpdateMonitorAlertRules(payload MonitorAlertRuleBatchPayload) error {
	if len(payload.IDs) == 0 {
		return errors.New("select at least one alert rule")
	}
	uniqueIDs := make([]uint, 0, len(payload.IDs))
	seen := map[uint]bool{}
	for _, id := range payload.IDs {
		if id > 0 && !seen[id] {
			seen[id] = true
			uniqueIDs = append(uniqueIDs, id)
		}
	}
	if len(uniqueIDs) == 0 {
		return errors.New("alert rule is required")
	}
	updates := map[string]any{}
	action := strings.ToLower(strings.TrimSpace(payload.Action))
	switch action {
	case "enable":
		updates["status"] = 1
	case "disable":
		updates["status"] = 2
	case "update_for_seconds":
		if payload.ForSeconds == nil {
			return errors.New("duration is required")
		}
		if *payload.ForSeconds < 0 || *payload.ForSeconds > 86400 {
			return errors.New("duration must be between 0 and 86400 seconds")
		}
		var metricRuleCount int64
		if err := s.db.Model(&model.MonitorAlertRule{}).Where("id IN ? AND alert_type = ?", uniqueIDs, "metric").Count(&metricRuleCount).Error; err != nil {
			return err
		}
		if metricRuleCount == 0 {
			return errors.New("none of the selected metric alert rules support duration updates")
		}
		return s.db.Model(&model.MonitorAlertRule{}).Where("id IN ? AND alert_type = ?", uniqueIDs, "metric").Update("for_seconds", *payload.ForSeconds).Error
	case "update_eval_interval":
		if payload.EvalIntervalSeconds == nil {
			return errors.New("evaluation interval is required")
		}
		if *payload.EvalIntervalSeconds < 15 || *payload.EvalIntervalSeconds > 3600 {
			return errors.New("evaluation interval must be between 15 and 3600 seconds")
		}
		updates["eval_interval_seconds"] = *payload.EvalIntervalSeconds
	case "enable_notify":
		if payload.NotifyRuleID == 0 {
			return errors.New("select a notification rule")
		}
		if payload.NotifyRepeatIntervalSeconds == nil {
			return errors.New("repeat-notification interval is required")
		}
		if *payload.NotifyRepeatIntervalSeconds < 60 || *payload.NotifyRepeatIntervalSeconds > 604800 {
			return errors.New("repeat-notification interval must be between one minute and seven days")
		}
		if payload.MaxNotifyCount == nil {
			return errors.New("maximum send count is required")
		}
		if *payload.MaxNotifyCount < 0 || *payload.MaxNotifyCount > 1000 {
			return errors.New("maximum send count must be between 0 and 1000")
		}
		if payload.NotifyRecoveryEnabled == nil {
			return errors.New("configure whether recovery notifications are sent")
		}
		var notifyRule model.NotifyRule
		if err := s.db.Where("id = ? AND status = ?", payload.NotifyRuleID, 1).First(&notifyRule).Error; err != nil {
			return errors.New("notification rule does not exist or is disabled")
		}
		updates["notify_enabled"] = true
		updates["notify_rule_id"] = payload.NotifyRuleID
		updates["notify_repeat_interval_seconds"] = *payload.NotifyRepeatIntervalSeconds
		updates["max_notify_count"] = *payload.MaxNotifyCount
		updates["notify_recovery_enabled"] = *payload.NotifyRecoveryEnabled
	default:
		return errors.New("unsupported batch operation")
	}
	if err := s.db.Model(&model.MonitorAlertRule{}).Where("id IN ?", uniqueIDs).Updates(updates).Error; err != nil {
		return err
	}
	if action == "enable_notify" {
		return nil
	}
	var rules []model.MonitorAlertRule
	if err := s.db.Where("id IN ?", uniqueIDs).Find(&rules).Error; err != nil {
		return err
	}
	for _, rule := range rules {
		if rule.Status == 1 {
			if err := s.registerMonitorAlertRule(rule); err != nil {
				return err
			}
		} else if action == "enable" || action == "disable" {
			s.removeMonitorAlertRule(rule.ID)
			s.closeActiveMonitorAlertEventsForRule(rule.ID, "alert rules disabled in batch; open events were closed automatically")
		}
	}
	return nil
}
