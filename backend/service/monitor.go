package service

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"ops-admin/backend/model"

	"gorm.io/gorm"
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

func (s *Service) RunMonitorAlertRule(id uint) error {
	go s.evaluateMonitorAlertRule(id)
	return nil
}

func (s *Service) PreviewMonitorAlertRule(payload MonitorAlertRulePayload) (map[string]any, error) {
	queryText := firstNonEmpty(strings.TrimSpace(payload.Query), strings.TrimSpace(payload.PromQL))
	if queryText == "" {
		return nil, errors.New("query is required")
	}
	rule := model.MonitorAlertRule{
		ID: payload.ID, Name: firstNonEmpty(strings.TrimSpace(payload.Name), "Rule Preview"),
		AlertType: normalizeAlertType(payload.AlertType), DatasourceScope: normalizeDatasourceScope(payload.DatasourceScope),
		DatasourceID: payload.DatasourceID, PromQL: queryText, LogIndex: firstNonEmpty(strings.TrimSpace(payload.LogIndex), "_all"),
		LogTimeRangeSeconds: normalizeLogTimeRangeSeconds(payload.LogTimeRangeSeconds), Comparator: normalizeComparator(payload.Comparator),
		Threshold: payload.Threshold, Severity: normalizeSeverity(payload.Severity),
	}
	datasources, err := s.monitorRuleDatasources(rule)
	if err != nil {
		return nil, err
	}
	results := make([]map[string]any, 0, len(datasources))
	totalSeries := 0
	totalMatched := 0
	failedDatasources := 0
	for _, ds := range datasources {
		item := map[string]any{"datasourceId": ds.ID, "datasourceName": ds.Name, "status": "success", "samples": []map[string]any{}}
		samples := make([]map[string]any, 0)
		if isMonitorLogAlertType(rule.AlertType) {
			value, sample, queryErr := s.monitorLogAlertValue(rule, ds)
			if queryErr != nil {
				item["status"] = "failed"
				item["error"] = queryErr.Error()
				failedDatasources++
			} else {
				matched := compareFloat(value, rule.Comparator, rule.Threshold)
				totalSeries++
				if matched {
					totalMatched++
				}
				samples = append(samples, map[string]any{"labels": sample.Metric, "value": value, "matched": matched})
			}
		} else {
			result, queryErr := s.prometheusQuery(ds, rule.PromQL, time.Now())
			if queryErr != nil {
				item["status"] = "failed"
				item["error"] = queryErr.Error()
				failedDatasources++
			} else {
				item["resultType"] = result.Data.ResultType
				totalSeries += len(result.Data.Result)
				for _, sample := range result.Data.Result {
					value, ok := promSampleValue(sample)
					if !ok {
						continue
					}
					matched := compareFloat(value, rule.Comparator, rule.Threshold)
					if matched {
						totalMatched++
					}
					if len(samples) < 50 {
						samples = append(samples, map[string]any{"labels": sample.Metric, "value": value, "matched": matched})
					}
				}
			}
		}
		item["samples"] = samples
		item["seriesCount"] = len(samples)
		results = append(results, item)
	}
	explanation := fmt.Sprintf("queried %d datasources and returned %d series; %d matched %s %.4f", len(datasources), totalSeries, totalMatched, rule.Comparator, rule.Threshold)
	if failedDatasources > 0 {
		explanation += fmt.Sprintf("; %d datasource queries failed", failedDatasources)
	}
	return map[string]any{
		"datasourceCount": len(datasources), "failedDatasourceCount": failedDatasources,
		"totalSeries": totalSeries, "totalMatched": totalMatched, "explanation": explanation, "results": results,
	}, nil
}

func (s *Service) evaluateMonitorAlertRule(id uint) {
	if !s.beginMonitorAlertRuleEvaluation(id) {
		return
	}
	defer s.endMonitorAlertRuleEvaluation(id)

	rule, err := s.GetMonitorAlertRule(id)
	if err != nil || rule.Status != 1 {
		return
	}
	datasources, err := s.monitorRuleDatasources(*rule)
	if err != nil {
		s.updateMonitorRuleEval(*rule, "failed", err.Error())
		return
	}
	activeFingerprints := map[string]bool{}
	matched := 0
	failed := 0
	for _, ds := range datasources {
		scopedRule := *rule
		scopedRule.DatasourceID = ds.ID
		scopedRule.DatasourceName = ds.Name
		if isMonitorLogAlertType(rule.AlertType) {
			value, sample, err := s.monitorLogAlertValue(scopedRule, ds)
			if err != nil {
				failed++
				continue
			}
			if !compareFloat(value, scopedRule.Comparator, scopedRule.Threshold) {
				continue
			}
			fp := monitorFingerprint(scopedRule.ID, sample.Metric)
			activeFingerprints[fp] = true
			matched++
			s.upsertMonitorAlertEvent(scopedRule, sample, fp, value)
			continue
		}
		result, err := s.prometheusQuery(ds, scopedRule.PromQL, time.Now())
		if err != nil {
			failed++
			continue
		}
		for _, sample := range result.Data.Result {
			value, ok := promSampleValue(sample)
			if !ok || !compareFloat(value, scopedRule.Comparator, scopedRule.Threshold) {
				continue
			}
			if sample.Metric == nil {
				sample.Metric = map[string]string{}
			}
			sample.Metric["datasource"] = ds.Name
			fp := monitorFingerprint(scopedRule.ID, sample.Metric)
			activeFingerprints[fp] = true
			matched++
			s.upsertMonitorAlertEvent(scopedRule, sample, fp, value)
		}
	}
	s.recoverInactiveMonitorEvents(*rule, activeFingerprints)
	if failed == len(datasources) {
		s.updateMonitorRuleEval(*rule, "failed", "evaluation failed for all matching datasources")
		return
	}
	s.updateMonitorRuleEval(*rule, "success", fmt.Sprintf("%d series matched; %d datasources failed", matched, failed))
}

func (s *Service) beginMonitorAlertRuleEvaluation(id uint) bool {
	if s.monitorScheduler == nil {
		return true
	}
	s.monitorScheduler.mu.Lock()
	defer s.monitorScheduler.mu.Unlock()
	if s.monitorScheduler.running[id] {
		return false
	}
	s.monitorScheduler.running[id] = true
	return true
}

func (s *Service) endMonitorAlertRuleEvaluation(id uint) {
	if s.monitorScheduler == nil {
		return
	}
	s.monitorScheduler.mu.Lock()
	delete(s.monitorScheduler.running, id)
	s.monitorScheduler.mu.Unlock()
}

func (s *Service) monitorRuleDatasources(rule model.MonitorAlertRule) ([]model.MonitorDatasource, error) {
	if normalizeDatasourceScope(rule.DatasourceScope) == "specific" {
		ds, err := s.GetMonitorDatasource(rule.DatasourceID)
		if err != nil {
			return nil, err
		}
		return []model.MonitorDatasource{*ds}, nil
	}
	query := s.db.Where("status = ?", 1)
	if normalizeAlertType(rule.AlertType) == "log" {
		query = query.Where("type = ?", "elasticsearch")
	} else if normalizeAlertType(rule.AlertType) == "victorialogs" {
		query = query.Where("type = ?", "victorialogs")
	} else {
		query = query.Where("type IN ?", []string{"prometheus", "victoriametrics"})
	}
	var list []model.MonitorDatasource
	if err := query.Order("is_default DESC, id DESC").Find(&list).Error; err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, errors.New("no matching datasource is available")
	}
	return list, nil
}

func (s *Service) monitorLogAlertValue(rule model.MonitorAlertRule, ds model.MonitorDatasource) (float64, PromMetricSample, error) {
	if normalizeMonitorDatasourceType(ds.Type) == "victorialogs" {
		return s.victoriaLogsAlertValue(rule, ds)
	}
	end := time.Now()
	start := end.Add(-time.Duration(normalizeLogTimeRangeSeconds(rule.LogTimeRangeSeconds)) * time.Second)
	must := []any{map[string]any{"match_all": map[string]any{}}}
	if strings.TrimSpace(rule.PromQL) != "" {
		must = []any{map[string]any{"query_string": map[string]any{"query": strings.TrimSpace(rule.PromQL), "analyze_wildcard": true}}}
	}
	body := map[string]any{
		"size":             0,
		"track_total_hits": true,
		"query": map[string]any{"bool": map[string]any{
			"must": must,
			"filter": []any{map[string]any{"range": map[string]any{"@timestamp": map[string]any{
				"gte": start.UnixMilli(), "lte": end.UnixMilli(), "format": "epoch_millis",
			}}}},
		}},
	}
	result, err := s.elasticsearchSearch(ds, firstNonEmpty(strings.TrimSpace(rule.LogIndex), "_all"), body)
	if err != nil {
		return 0, PromMetricSample{}, err
	}
	hits, _ := result["hits"].(map[string]any)
	value := elasticsearchHitTotal(hits["total"])
	return value, PromMetricSample{Metric: map[string]string{
		"__name__": "elasticsearch_log_count", "datasource": ds.Name, "index": firstNonEmpty(strings.TrimSpace(rule.LogIndex), "_all"), "alert_type": "log",
	}}, nil
}

func (s *Service) victoriaLogsAlertValue(rule model.MonitorAlertRule, ds model.MonitorDatasource) (float64, PromMetricSample, error) {
	end := time.Now()
	start := end.Add(-time.Duration(normalizeLogTimeRangeSeconds(rule.LogTimeRangeSeconds)) * time.Second)
	query := strings.TrimSpace(rule.PromQL)
	if query == "" {
		query = "*"
	}
	form := url.Values{}
	form.Set("query", query)
	form.Set("start", start.UTC().Format(time.RFC3339Nano))
	form.Set("end", end.UTC().Format(time.RFC3339Nano))
	form.Set("step", "1m")
	request, err := http.NewRequest(http.MethodPost, strings.TrimRight(ds.URL, "/")+"/select/logsql/hits", strings.NewReader(form.Encode()))
	if err != nil {
		return 0, PromMetricSample{}, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	applyMonitorDatasourceAuth(request, ds)
	response, err := (&http.Client{Timeout: 20 * time.Second}).Do(request)
	if err != nil {
		return 0, PromMetricSample{}, err
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(response.Body, 2*1024*1024))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return 0, PromMetricSample{}, fmt.Errorf("VictoriaLogs alert query failed with status %d: %s", response.StatusCode, string(body))
	}
	var result struct {
		Hits []struct {
			Total int64 `json:"total"`
		} `json:"hits"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, PromMetricSample{}, fmt.Errorf("failed to parse VictoriaLogs alert result: %w", err)
	}
	var value int64
	for _, group := range result.Hits {
		value += group.Total
	}
	return float64(value), PromMetricSample{Metric: map[string]string{
		"__name__": "victorialogs_log_count", "datasource": ds.Name, "alert_type": "victorialogs",
	}}, nil
}

func elasticsearchHitTotal(value any) float64 {
	switch item := value.(type) {
	case float64:
		return item
	case int:
		return float64(item)
	case map[string]any:
		return elasticsearchHitTotal(item["value"])
	default:
		parsed, _ := strconv.ParseFloat(fmt.Sprint(value), 64)
		return parsed
	}
}

func (s *Service) updateMonitorRuleEval(rule model.MonitorAlertRule, status, message string) {
	now := time.Now()
	_ = s.db.Model(&model.MonitorAlertRule{}).Where("id = ?", rule.ID).Updates(map[string]any{
		"last_eval_at": &now, "last_eval_status": status, "last_eval_message": message,
	}).Error
}

func promSampleValue(sample PromMetricSample) (float64, bool) {
	if len(sample.Value) < 2 {
		return 0, false
	}
	raw := fmt.Sprintf("%v", sample.Value[1])
	value, err := strconv.ParseFloat(raw, 64)
	return value, err == nil
}

func compareFloat(value float64, comparator string, threshold float64) bool {
	switch normalizeComparator(comparator) {
	case ">":
		return value > threshold
	case ">=":
		return value >= threshold
	case "<":
		return value < threshold
	case "<=":
		return value <= threshold
	case "==":
		return value == threshold
	case "!=":
		return value != threshold
	default:
		return value > threshold
	}
}

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

func normalizeDashboardLayout(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "list":
		return "list"
	default:
		return "grid"
	}
}

func normalizePanelChartType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "table":
		return "table"
	case "line":
		return "line"
	case "bar":
		return "bar"
	case "gauge":
		return "gauge"
	default:
		return "stat"
	}
}

func normalizePanelSpan(value int) int {
	if value < 6 {
		return 6
	}
	if value > 24 {
		return 24
	}
	return value
}

func (s *Service) ListMonitorDashboards(pageNum, pageSize int, keyword, status string) (map[string]any, error) {
	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	query := s.db.Model(&model.MonitorDashboard{})
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
	var list []model.MonitorDashboard
	if err := query.Order("id DESC").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, err
	}
	rows := make([]map[string]any, 0, len(list))
	for _, item := range list {
		var panelCount int64
		_ = s.db.Model(&model.MonitorDashboardPanel{}).Where("dashboard_id = ?", item.ID).Count(&panelCount).Error
		rows = append(rows, map[string]any{
			"id": item.ID, "name": item.Name, "layout": item.Layout, "status": item.Status,
			"description": item.Description, "panelCount": panelCount, "createTime": item.CreatedAt, "updateTime": item.UpdatedAt,
		})
	}
	return map[string]any{"list": rows, "total": total, "pageNum": pageNum, "pageSize": pageSize}, nil
}

func (s *Service) GetMonitorDashboard(id uint) (map[string]any, error) {
	var dashboard model.MonitorDashboard
	if err := s.db.First(&dashboard, id).Error; err != nil {
		return nil, err
	}
	var panels []model.MonitorDashboardPanel
	if err := s.db.Where("dashboard_id = ?", id).Order("sort ASC, id ASC").Find(&panels).Error; err != nil {
		return nil, err
	}
	return map[string]any{"dashboard": dashboard, "panels": panels}, nil
}

func (s *Service) SaveMonitorDashboard(payload MonitorDashboardPayload) (*model.MonitorDashboard, error) {
	if strings.TrimSpace(payload.Name) == "" {
		return nil, errors.New("dashboard name is required")
	}
	updates := map[string]any{
		"name":        Trimmed(payload.Name),
		"layout":      normalizeDashboardLayout(payload.Layout),
		"status":      normalizeMonitorStatus(payload.Status),
		"description": Trimmed(payload.Description),
	}
	if payload.ID > 0 {
		if err := s.db.Model(&model.MonitorDashboard{}).Where("id = ?", payload.ID).Updates(updates).Error; err != nil {
			return nil, err
		}
		var item model.MonitorDashboard
		if err := s.db.First(&item, payload.ID).Error; err != nil {
			return nil, err
		}
		return &item, nil
	}
	item := model.MonitorDashboard{
		Name:        Trimmed(payload.Name),
		Layout:      normalizeDashboardLayout(payload.Layout),
		Status:      normalizeMonitorStatus(payload.Status),
		Description: Trimmed(payload.Description),
	}
	if err := s.db.Create(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Service) DeleteMonitorDashboard(id uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("dashboard_id = ?", id).Delete(&model.MonitorDashboardPanel{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.MonitorDashboard{}, id).Error
	})
}

func (s *Service) SaveMonitorDashboardPanel(payload MonitorDashboardPanelPayload) error {
	if payload.DashboardID == 0 {
		return errors.New("dashboard is required")
	}
	if strings.TrimSpace(payload.Title) == "" {
		return errors.New("panel title is required")
	}
	if strings.TrimSpace(payload.PromQL) == "" {
		return errors.New("PromQL is required")
	}
	ds, err := s.GetMonitorDatasource(payload.DatasourceID)
	if err != nil {
		return err
	}
	if isMonitorLogDatasource(ds.Type) {
		return errors.New("log datasources do not support PromQL monitoring panels; select Prometheus or VictoriaMetrics")
	}
	updates := map[string]any{
		"dashboard_id":    payload.DashboardID,
		"title":           Trimmed(payload.Title),
		"datasource_id":   ds.ID,
		"datasource_name": ds.Name,
		"prom_ql":         strings.TrimSpace(payload.PromQL),
		"unit":            strings.TrimSpace(payload.Unit),
		"chart_type":      normalizePanelChartType(payload.ChartType),
		"span":            normalizePanelSpan(payload.Span),
		"sort":            payload.Sort,
		"status":          normalizeMonitorStatus(payload.Status),
		"description":     Trimmed(payload.Description),
	}
	if payload.ID > 0 {
		return s.db.Model(&model.MonitorDashboardPanel{}).Where("id = ?", payload.ID).Updates(updates).Error
	}
	return s.db.Model(&model.MonitorDashboardPanel{}).Create(updates).Error
}

func (s *Service) DeleteMonitorDashboardPanel(id uint) error {
	return s.db.Delete(&model.MonitorDashboardPanel{}, id).Error
}

func (s *Service) QueryMonitorDashboardPanel(payload MonitorDashboardPanelQueryPayload) (map[string]any, error) {
	var panel model.MonitorDashboardPanel
	if err := s.db.First(&panel, payload.ID).Error; err != nil {
		return nil, err
	}
	queryDatasourceID := panel.DatasourceID
	if payload.DatasourceID > 0 {
		queryDatasourceID = payload.DatasourceID
	}
	ds, err := s.GetMonitorDatasource(queryDatasourceID)
	if err != nil {
		return nil, err
	}
	if isMonitorLogDatasource(ds.Type) {
		var fallback model.MonitorDatasource
		if err := s.db.Where("status = ? AND type IN ?", 1, []string{"prometheus", "victoriametrics"}).Order("is_default DESC, id DESC").First(&fallback).Error; err != nil {
			return nil, errors.New("monitoring panels support only Prometheus or VictoriaMetrics; configure a metric datasource first")
		}
		ds = &fallback
	}
	var result *PromQueryResult
	if payload.StartAt > 0 && payload.EndAt > payload.StartAt {
		result, err = s.prometheusRangeQuery(*ds, panel.PromQL, time.Unix(payload.StartAt, 0), time.Unix(payload.EndAt, 0), payload.StepSeconds)
	} else {
		result, err = s.prometheusQuery(*ds, panel.PromQL, time.Now())
	}
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"panel": panel, "datasource": ds, "resultType": result.Data.ResultType, "result": result.Data.Result,
	}, nil
}
