package service

import (
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
)

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
