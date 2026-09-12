package service

import (
	"sort"
	"time"

	"ops-admin/backend/model"

	"gorm.io/gorm"
)

func (s *Service) GetMonitorOverview(startAt, endAt *time.Time) (map[string]any, error) {
	finishedStatuses := []string{"recovered", "resolved", "closed"}
	// Earlier event records can have only resolved_at (or, for imported legacy
	// records, only updated_at). Always use the first available end timestamp so
	// the overview and alert-event list describe the same history.
	finishedAt := "COALESCE(recovered_at, resolved_at, updated_at)"
	withinRange := func(query *gorm.DB, column string) *gorm.DB {
		if startAt != nil {
			query = query.Where(column+" >= ?", *startAt)
		}
		if endAt != nil {
			query = query.Where(column+" < ?", *endAt)
		}
		return query
	}
	var datasourceCount, healthyDatasourceCount, ruleCount, activeRuleCount, successfulRuleCount int64
	var firingCount, recoveredCount, rangeRecoveredCount int64
	var unclaimedCount, criticalCount, unhealthyDatasourceCount, evalFailedRuleCount int64
	var notificationFailedCount, notificationTotalCount, notificationSuccessCount, rangeTriggeredCount int64
	_ = s.db.Model(&model.MonitorDatasource{}).Count(&datasourceCount).Error
	_ = s.db.Model(&model.MonitorDatasource{}).Where("status = ? AND health_status IN ?", 1, []string{"healthy", "normal", "ok"}).Count(&healthyDatasourceCount).Error
	_ = s.db.Model(&model.MonitorAlertRule{}).Count(&ruleCount).Error
	_ = s.db.Model(&model.MonitorAlertRule{}).Where("status = ?", 1).Count(&activeRuleCount).Error
	_ = s.db.Model(&model.MonitorAlertRule{}).Where("status = ? AND (last_eval_status IN ? OR last_eval_status = '' OR last_eval_status IS NULL)", 1, []string{"success", "ok"}).Count(&successfulRuleCount).Error
	// Current risk always represents the platform now; the selected range is only for historical statistics.
	_ = s.db.Model(&model.MonitorAlertEvent{}).Where("status IN ?", []string{"pending", "firing", "claimed"}).Count(&firingCount).Error
	_ = s.db.Model(&model.MonitorAlertEvent{}).Where("status IN ? AND (claimed_by = '' OR claimed_by IS NULL)", []string{"pending", "firing"}).Count(&unclaimedCount).Error
	_ = s.db.Model(&model.MonitorAlertEvent{}).Where("status IN ? AND severity IN ?", []string{"pending", "firing", "claimed"}, []string{"P0", "P1"}).Count(&criticalCount).Error
	_ = withinRange(s.db.Model(&model.MonitorAlertEvent{}).Where("status IN ?", finishedStatuses), finishedAt).Count(&recoveredCount).Error
	_ = s.db.Model(&model.MonitorDatasource{}).Where("status = ? AND health_status = ?", 1, "unhealthy").Count(&unhealthyDatasourceCount).Error
	_ = s.db.Model(&model.MonitorAlertRule{}).Where("status = ? AND last_eval_status = ?", 1, "failed").Count(&evalFailedRuleCount).Error
	notifyQuery := s.db.Model(&model.NotifySendLog{}).Where("scope = ?", "monitor")
	if startAt == nil && endAt == nil {
		notifyQuery = notifyQuery.Where("created_at >= ?", time.Now().Add(-24*time.Hour))
	} else {
		notifyQuery = withinRange(notifyQuery, "created_at")
	}
	_ = notifyQuery.Count(&notificationTotalCount).Error
	_ = notifyQuery.Where("status = ?", "failed").Count(&notificationFailedCount).Error
	_ = notifyQuery.Where("status IN ?", []string{"success", "sent"}).Count(&notificationSuccessCount).Error
	now := time.Now()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	triggeredQuery := s.db.Model(&model.MonitorAlertEvent{})
	recoveredQuery := s.db.Model(&model.MonitorAlertEvent{}).Where("status IN ?", finishedStatuses)
	if startAt == nil && endAt == nil {
		triggeredQuery = triggeredQuery.Where("created_at >= ?", dayStart)
		recoveredQuery = recoveredQuery.Where(finishedAt+" >= ?", dayStart)
	} else {
		triggeredQuery = withinRange(triggeredQuery, "created_at")
		recoveredQuery = withinRange(recoveredQuery, finishedAt)
	}
	_ = triggeredQuery.Count(&rangeTriggeredCount).Error
	_ = recoveredQuery.Count(&rangeRecoveredCount).Error
	severityRows := []map[string]any{}
	for _, severity := range []string{"P0", "P1", "P2", "P3"} {
		var count int64
		_ = s.db.Model(&model.MonitorAlertEvent{}).Where("status IN ? AND severity = ?", []string{"pending", "firing", "claimed"}, severity).Count(&count).Error
		severityRows = append(severityRows, map[string]any{"severity": severity, "count": count})
	}
	var recent []model.MonitorAlertEvent
	_ = s.db.Where("status IN ?", []string{"pending", "firing", "claimed"}).Order("last_trigger_at DESC, id DESC").Limit(8).Find(&recent).Error
	var recentHandled []model.MonitorAlertEvent
	handledQuery := s.db.Where("claimed_at IS NOT NULL OR recovered_at IS NOT NULL")
	if startAt == nil && endAt == nil {
		handledQuery = handledQuery.Where("created_at >= ?", time.Now().Add(-30*24*time.Hour))
	} else {
		handledQuery = withinRange(handledQuery, "created_at")
	}
	_ = handledQuery.Find(&recentHandled).Error
	var totalAckSeconds, totalRecoverSeconds int64
	var ackSamples, recoverSamples int64
	for _, event := range recentHandled {
		if event.ClaimedAt != nil && event.ClaimedAt.After(event.FirstTriggerAt) {
			totalAckSeconds += int64(event.ClaimedAt.Sub(event.FirstTriggerAt).Seconds())
			ackSamples++
		}
		if event.RecoveredAt != nil && event.RecoveredAt.After(event.FirstTriggerAt) {
			totalRecoverSeconds += int64(event.RecoveredAt.Sub(event.FirstTriggerAt).Seconds())
			recoverSamples++
		}
	}
	average := func(total, count int64) int64 {
		if count == 0 {
			return 0
		}
		return total / count
	}
	trend := make([]map[string]any, 0)
	trendStart, trendEnd := dayStart, dayStart.AddDate(0, 0, 1)
	if startAt != nil {
		trendStart = time.Date(startAt.Year(), startAt.Month(), startAt.Day(), 0, 0, 0, 0, startAt.Location())
	}
	if endAt != nil {
		trendEnd = *endAt
	}
	for cursor := trendStart; cursor.Before(trendEnd); cursor = cursor.AddDate(0, 0, 1) {
		next := cursor.AddDate(0, 0, 1)
		var triggered, recovered int64
		_ = s.db.Model(&model.MonitorAlertEvent{}).Where("created_at >= ? AND created_at < ?", cursor, next).Count(&triggered).Error
		_ = s.db.Model(&model.MonitorAlertEvent{}).Where("status IN ? AND "+finishedAt+" >= ? AND "+finishedAt+" < ?", finishedStatuses, cursor, next).Count(&recovered).Error
		trend = append(trend, map[string]any{"date": cursor.Format("01-02"), "triggered": triggered, "recovered": recovered})
	}

	activities := make([]map[string]any, 0, 10)
	var recoveredEvents []model.MonitorAlertEvent
	_ = withinRange(s.db.Where("status IN ?", finishedStatuses), finishedAt).Order(finishedAt + " DESC").Limit(3).Find(&recoveredEvents).Error
	for _, item := range recoveredEvents {
		activities = append(activities, map[string]any{"type": "recovered", "title": item.RuleName, "detail": firstNonEmpty(item.Summary, "alert recovered"), "time": monitorAlertFinishedAt(item)})
	}
	var unhealthySources []model.MonitorDatasource
	_ = s.db.Where("status = ? AND health_status = ?", 1, "unhealthy").Order("updated_at DESC").Limit(3).Find(&unhealthySources).Error
	for _, item := range unhealthySources {
		activities = append(activities, map[string]any{"type": "datasource", "title": item.Name, "detail": firstNonEmpty(item.LastError, "datasource health check failed"), "time": item.UpdatedAt})
	}
	var failedNotifications []model.NotifySendLog
	_ = notifyQuery.Where("status = ?", "failed").Order("created_at DESC").Limit(3).Find(&failedNotifications).Error
	for _, item := range failedNotifications {
		activities = append(activities, map[string]any{"type": "notification", "title": firstNonEmpty(item.RuleName, item.ChannelName), "detail": firstNonEmpty(item.ErrorText, item.Summary, "notification delivery failed"), "time": item.CreatedAt})
	}
	var failedRules []model.MonitorAlertRule
	_ = s.db.Where("status = ? AND last_eval_status = ?", 1, "failed").Order("last_eval_at DESC").Limit(3).Find(&failedRules).Error
	for _, item := range failedRules {
		activities = append(activities, map[string]any{"type": "rule", "title": item.Name, "detail": firstNonEmpty(item.LastEvalMessage, "rule execution failed"), "time": item.LastEvalAt})
	}
	sort.SliceStable(activities, func(i, j int) bool {
		leftTime, leftOK := overviewActivityTime(activities[i]["time"])
		rightTime, rightOK := overviewActivityTime(activities[j]["time"])
		return leftOK && (!rightOK || leftTime.After(rightTime))
	})
	if len(activities) > 8 {
		activities = activities[:8]
	}
	return map[string]any{
		"datasourceCount": datasourceCount, "healthyDatasourceCount": healthyDatasourceCount,
		"ruleCount": ruleCount, "activeRuleCount": activeRuleCount, "successfulRuleCount": successfulRuleCount,
		"firingCount": firingCount, "recoveredCount": recoveredCount, "rangeRecoveredCount": rangeRecoveredCount, "severity": severityRows, "recentEvents": recent,
		"unclaimedCount": unclaimedCount, "criticalCount": criticalCount,
		"unhealthyDatasourceCount": unhealthyDatasourceCount, "evalFailedRuleCount": evalFailedRuleCount,
		"notificationFailedCount": notificationFailedCount, "notificationTotalCount": notificationTotalCount, "notificationSuccessCount": notificationSuccessCount, "rangeTriggeredCount": rangeTriggeredCount,
		"mttaSeconds": average(totalAckSeconds, ackSamples), "mttrSeconds": average(totalRecoverSeconds, recoverSamples),
		"trend": trend, "recentActivities": activities, "refreshedAt": time.Now(),
	}, nil
}

// GetMonitorCommandCenter is the read model for the operations command
// center. It uses the platform's already-managed asset inventory rather than
// fabricating geographic data or depending on an external map service.
func (s *Service) GetMonitorCommandCenter() (map[string]any, error) {
	overview, err := s.GetMonitorOverview(nil, nil)
	if err != nil {
		return nil, err
	}

	var hostCount, onlineHostCount, databaseCount, connectedDatabaseCount, clusterCount, serviceCount int64
	_ = s.db.Model(&model.AssetHost{}).Where("status = ?", 1).Count(&hostCount).Error
	_ = s.db.Model(&model.AssetHost{}).Where("status = ? AND alive_status = ?", 1, 1).Count(&onlineHostCount).Error
	_ = s.db.Model(&model.AssetDatabase{}).Where("status = ?", 1).Count(&databaseCount).Error
	_ = s.db.Model(&model.AssetDatabase{}).Where("status = ? AND connect_status = ?", 1, 1).Count(&connectedDatabaseCount).Error
	if liveConns, connErr := s.k8sClusterConnections(); connErr == nil {
		clusterCount = int64(len(liveConns))
	}
	_ = s.db.Model(&model.AssetService{}).Where("status = ?", 1).Count(&serviceCount).Error

	type regionRow struct {
		Name  string `json:"name"`
		Count int64  `json:"count"`
	}
	regions := make([]regionRow, 0)
	_ = s.db.Model(&model.AssetHost{}).
		Select("COALESCE(NULLIF(region, ''), 'Unassigned Region') AS name, COUNT(*) AS count").
		Where("status = ?", 1).
		Group("COALESCE(NULLIF(region, ''), 'Unassigned Region')").
		Order("count DESC, name ASC").
		Limit(6).
		Scan(&regions).Error

	type ruleRow struct {
		Name  string `json:"name"`
		Count int64  `json:"count"`
	}
	topRules := make([]ruleRow, 0)
	_ = s.db.Model(&model.MonitorAlertEvent{}).
		Select("COALESCE(NULLIF(rule_name, ''), 'Unnamed Rule') AS name, COUNT(*) AS count").
		Where("status IN ?", []string{"pending", "firing", "claimed"}).
		Group("COALESCE(NULLIF(rule_name, ''), 'Unnamed Rule')").
		Order("count DESC, name ASC").
		Limit(5).
		Scan(&topRules).Error

	type hostRow struct {
		Name        string    `json:"name"`
		Region      string    `json:"region"`
		AliveStatus int       `json:"aliveStatus"`
		UpdatedAt   time.Time `json:"updatedAt"`
	}
	hotHosts := make([]hostRow, 0)
	_ = s.db.Model(&model.AssetHost{}).
		Select("host_name AS name, COALESCE(NULLIF(region, ''), 'Unassigned Region') AS region, alive_status, updated_at").
		Where("status = ?", 1).
		Order("alive_status ASC, updated_at DESC").
		Limit(5).
		Scan(&hotHosts).Error

	recentAlerts := make([]model.MonitorAlertEvent, 0)
	_ = s.db.Where("status IN ?", []string{"pending", "firing", "claimed"}).
		Order("last_trigger_at DESC, id DESC").
		Limit(6).
		Find(&recentAlerts).Error

	assetTotal := hostCount + databaseCount + clusterCount + serviceCount
	coverage := 0.0
	if hostCount > 0 {
		coverage = float64(onlineHostCount) * 100 / float64(hostCount)
	}
	return map[string]any{
		"overview": overview,
		"assetSummary": map[string]any{
			"total": assetTotal, "hosts": hostCount, "onlineHosts": onlineHostCount,
			"databases": databaseCount, "connectedDatabases": connectedDatabaseCount,
			"clusters": clusterCount, "services": serviceCount, "coverage": coverage,
		},
		"resourceComposition": []map[string]any{
			{"name": "Physical / Cloud Hosts", "count": hostCount, "tone": "cyan"},
			{"name": "Kubernetes Clusters", "count": clusterCount, "tone": "blue"},
			{"name": "Databases", "count": databaseCount, "tone": "amber"},
			{"name": "Services", "count": serviceCount, "tone": "violet"},
		},
		"regions": regions, "topRules": topRules, "recentAlerts": recentAlerts,
		"hotHosts": hotHosts, "refreshedAt": time.Now(),
	}, nil
}

func monitorAlertFinishedAt(event model.MonitorAlertEvent) time.Time {
	if event.RecoveredAt != nil && !event.RecoveredAt.IsZero() {
		return *event.RecoveredAt
	}
	if event.ResolvedAt != nil && !event.ResolvedAt.IsZero() {
		return *event.ResolvedAt
	}
	return event.UpdatedAt
}

func overviewActivityTime(value any) (time.Time, bool) {
	switch item := value.(type) {
	case time.Time:
		return item, !item.IsZero()
	case *time.Time:
		if item != nil {
			return *item, !item.IsZero()
		}
	}
	return time.Time{}, false
}
