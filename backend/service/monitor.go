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
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"ops-admin/backend/model"

	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

type MonitorDatasourcePayload struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	URL         string `json:"url"`
	AuthType    string `json:"authType"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	Token       string `json:"token"`
	IsDefault   bool   `json:"isDefault"`
	Status      int    `json:"status"`
	Description string `json:"description"`
}

type MonitorAlertRulePayload struct {
	ID                    uint    `json:"id"`
	Name                  string  `json:"name"`
	DatasourceID          uint    `json:"datasourceId"`
	PromQL                string  `json:"promql"`
	Comparator            string  `json:"comparator"`
	Threshold             float64 `json:"threshold"`
	ForSeconds            int     `json:"forSeconds"`
	EvalIntervalSeconds   int     `json:"evalIntervalSeconds"`
	Severity              string  `json:"severity"`
	LabelsJSON            string  `json:"labelsJson"`
	AnnotationsJSON       string  `json:"annotationsJson"`
	NotifyEnabled         bool    `json:"notifyEnabled"`
	NotifyRuleID          uint    `json:"notifyRuleId"`
	NotifyRecoveryEnabled bool    `json:"notifyRecoveryEnabled"`
	Status                int     `json:"status"`
	Description           string  `json:"description"`
}

type MonitorAlertEventActionPayload struct {
	ID         uint   `json:"id"`
	ClaimedBy  string `json:"claimedBy"`
	HandleNote string `json:"handleNote"`
}

type MonitorSilenceRulePayload struct {
	ID              uint   `json:"id"`
	Name            string `json:"name"`
	MatchMode       string `json:"matchMode"`
	RuleIDs         []uint `json:"ruleIds"`
	RuleNamePattern string `json:"ruleNamePattern"`
	Severity        string `json:"severity"`
	MatchersJSON    string `json:"matchersJson"`
	StartsAt        int64  `json:"startsAt"`
	EndsAt          int64  `json:"endsAt"`
	Status          int    `json:"status"`
	Description     string `json:"description"`
}

type MonitorAggregationRulePayload struct {
	ID                    uint     `json:"id"`
	Name                  string   `json:"name"`
	MatchMode             string   `json:"matchMode"`
	RuleIDs               []uint   `json:"ruleIds"`
	RuleNamePattern       string   `json:"ruleNamePattern"`
	Severity              string   `json:"severity"`
	GroupBy               []string `json:"groupBy"`
	WindowSeconds         int      `json:"windowSeconds"`
	RepeatIntervalSeconds int      `json:"repeatIntervalSeconds"`
	Status                int      `json:"status"`
	Description           string   `json:"description"`
}

type MonitorDashboardPayload struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Layout      string `json:"layout"`
	Status      int    `json:"status"`
	Description string `json:"description"`
}

type MonitorDashboardPanelPayload struct {
	ID           uint   `json:"id"`
	DashboardID  uint   `json:"dashboardId"`
	Title        string `json:"title"`
	DatasourceID uint   `json:"datasourceId"`
	PromQL       string `json:"promql"`
	Unit         string `json:"unit"`
	ChartType    string `json:"chartType"`
	Span         int    `json:"span"`
	Sort         int    `json:"sort"`
	Status       int    `json:"status"`
	Description  string `json:"description"`
}

type MonitorScheduler struct {
	cron    *cron.Cron
	mu      sync.Mutex
	entries map[uint]cron.EntryID
}

type PromQueryResult struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string             `json:"resultType"`
		Result     []PromMetricSample `json:"result"`
	} `json:"data"`
	Error string `json:"error"`
}

type PromMetricSample struct {
	Metric map[string]string `json:"metric"`
	Value  []any             `json:"value"`
	Values [][]any           `json:"values"`
}

func normalizeMonitorDatasourceType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "victoriametrics", "victoria-metrics", "vm":
		return "victoriametrics"
	default:
		return "prometheus"
	}
}

func normalizeMonitorStatus(value int) int {
	if value == 2 {
		return 2
	}
	return 1
}

func normalizeMonitorAuthType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "basic":
		return "basic"
	case "bearer":
		return "bearer"
	default:
		return "none"
	}
}

func normalizeComparator(value string) string {
	switch strings.TrimSpace(value) {
	case ">", ">=", "<", "<=", "==", "!=":
		return strings.TrimSpace(value)
	default:
		return ">"
	}
}

func normalizeSeverity(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	switch value {
	case "P0", "P1", "P2", "P3":
		return value
	default:
		return "P2"
	}
}

func normalizeEvalInterval(value int) int {
	if value < 15 {
		return 15
	}
	if value > 3600 {
		return 3600
	}
	return value
}

func normalizeForSeconds(value int) int {
	if value < 0 {
		return 0
	}
	if value > 86400 {
		return 86400
	}
	return value
}

func ensureJSONObject(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "{}", nil
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(value), &obj); err != nil {
		return "", errors.New("JSON 格式不正确")
	}
	data, _ := json.Marshal(obj)
	return string(data), nil
}

func normalizeMatcherJSON(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "{}", nil
	}
	matchers := map[string]string{}
	if err := json.Unmarshal([]byte(value), &matchers); err != nil {
		return "", errors.New("matchers must be valid JSON object")
	}
	data, _ := json.Marshal(matchers)
	return string(data), nil
}

func unixPtr(value int64) *time.Time {
	if value <= 0 {
		return nil
	}
	t := time.Unix(value, 0)
	return &t
}

func normalizeAggregationWindow(value int) int {
	if value < 60 {
		return 60
	}
	if value > 86400 {
		return 86400
	}
	return value
}

func normalizeRuleMatchMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "select", "selected", "rules":
		return "select"
	default:
		return "regex"
	}
}

func monitorRuleNameMatch(pattern, ruleName string) bool {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return true
	}
	re, err := regexp.Compile(pattern)
	if err == nil {
		return re.MatchString(ruleName)
	}
	return strings.Contains(strings.ToLower(ruleName), strings.ToLower(pattern))
}

func monitorRuleMatch(matchMode, ruleIDsJSON, pattern string, rule model.MonitorAlertRule) bool {
	if normalizeRuleMatchMode(matchMode) == "select" {
		ids := decodeUintList(ruleIDsJSON)
		if len(ids) == 0 {
			return true
		}
		for _, id := range ids {
			if id == rule.ID {
				return true
			}
		}
		return false
	}
	return monitorRuleNameMatch(pattern, rule.Name)
}

func monitorSeverityMatch(pattern, severity string) bool {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" || strings.EqualFold(pattern, "all") {
		return true
	}
	return strings.EqualFold(pattern, severity)
}

func decodeLabelMap(raw string) map[string]string {
	labels := map[string]string{}
	_ = json.Unmarshal([]byte(raw), &labels)
	return labels
}

func monitorMatchersMatch(matchersJSON string, labels map[string]string) bool {
	matchers := map[string]string{}
	_ = json.Unmarshal([]byte(matchersJSON), &matchers)
	for key, expected := range matchers {
		if labels[key] != expected {
			return false
		}
	}
	return true
}

func (s *Service) initMonitorScheduler() {
	s.monitorSchedulerOnce.Do(func() {
		s.monitorScheduler = &MonitorScheduler{
			cron: cron.New(
				cron.WithSeconds(),
				cron.WithLocation(time.Local),
				cron.WithChain(cron.SkipIfStillRunning(cron.DefaultLogger)),
			),
			entries: map[uint]cron.EntryID{},
		}
		s.monitorScheduler.cron.Start()
		s.reloadMonitorAlertRules()
	})
}

func (s *Service) reloadMonitorAlertRules() {
	if s.monitorScheduler == nil {
		return
	}
	var rules []model.MonitorAlertRule
	if err := s.db.Where("status = ?", 1).Find(&rules).Error; err != nil {
		return
	}
	for _, rule := range rules {
		_ = s.registerMonitorAlertRule(rule)
	}
}

func (s *Service) registerMonitorAlertRule(rule model.MonitorAlertRule) error {
	if s.monitorScheduler == nil {
		return nil
	}
	s.removeMonitorAlertRule(rule.ID)
	interval := normalizeEvalInterval(rule.EvalIntervalSeconds)
	entryID, err := s.monitorScheduler.cron.AddFunc(fmt.Sprintf("@every %ds", interval), func() {
		s.evaluateMonitorAlertRule(rule.ID)
	})
	if err != nil {
		return err
	}
	s.monitorScheduler.mu.Lock()
	s.monitorScheduler.entries[rule.ID] = entryID
	s.monitorScheduler.mu.Unlock()
	return nil
}

func (s *Service) removeMonitorAlertRule(id uint) {
	if s.monitorScheduler == nil {
		return
	}
	s.monitorScheduler.mu.Lock()
	entryID, ok := s.monitorScheduler.entries[id]
	if ok {
		delete(s.monitorScheduler.entries, id)
	}
	s.monitorScheduler.mu.Unlock()
	if ok {
		s.monitorScheduler.cron.Remove(entryID)
	}
}

func (s *Service) ListMonitorDatasources(pageNum, pageSize int, keyword, dsType, status string) (map[string]any, error) {
	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	query := s.db.Model(&model.MonitorDatasource{})
	if strings.TrimSpace(keyword) != "" {
		like := "%" + strings.TrimSpace(keyword) + "%"
		query = query.Where("name LIKE ? OR url LIKE ? OR description LIKE ?", like, like, like)
	}
	if strings.TrimSpace(dsType) != "" {
		query = query.Where("type = ?", normalizeMonitorDatasourceType(dsType))
	}
	if strings.TrimSpace(status) != "" {
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var list []model.MonitorDatasource
	if err := query.Order("is_default DESC, id DESC").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, err
	}
	return map[string]any{"list": list, "total": total, "pageNum": pageNum, "pageSize": pageSize}, nil
}

func (s *Service) ListMonitorDatasourceOptions() ([]model.MonitorDatasource, error) {
	var list []model.MonitorDatasource
	if err := s.db.Where("status = ?", 1).Order("is_default DESC, id DESC").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

func (s *Service) GetMonitorDatasource(id uint) (*model.MonitorDatasource, error) {
	var item model.MonitorDatasource
	if err := s.db.First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Service) SaveMonitorDatasource(payload MonitorDatasourcePayload) error {
	if strings.TrimSpace(payload.Name) == "" {
		return errors.New("数据源名称不能为空")
	}
	if strings.TrimSpace(payload.URL) == "" {
		return errors.New("数据源地址不能为空")
	}
	updates := map[string]any{
		"name":        Trimmed(payload.Name),
		"type":        normalizeMonitorDatasourceType(payload.Type),
		"url":         strings.TrimRight(strings.TrimSpace(payload.URL), "/"),
		"auth_type":   normalizeMonitorAuthType(payload.AuthType),
		"username":    strings.TrimSpace(payload.Username),
		"password":    payload.Password,
		"token":       strings.TrimSpace(payload.Token),
		"is_default":  payload.IsDefault,
		"status":      normalizeMonitorStatus(payload.Status),
		"description": Trimmed(payload.Description),
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		if payload.IsDefault {
			if err := tx.Model(&model.MonitorDatasource{}).Where("id <> ?", payload.ID).Update("is_default", false).Error; err != nil {
				return err
			}
		}
		if payload.ID > 0 {
			return tx.Model(&model.MonitorDatasource{}).Where("id = ?", payload.ID).Updates(updates).Error
		}
		return tx.Model(&model.MonitorDatasource{}).Create(updates).Error
	})
}

func (s *Service) DeleteMonitorDatasource(id uint) error {
	var count int64
	if err := s.db.Model(&model.MonitorAlertRule{}).Where("datasource_id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return errors.New("数据源已被告警规则引用，不能删除")
	}
	return s.db.Delete(&model.MonitorDatasource{}, id).Error
}

func (s *Service) TestMonitorDatasource(id uint, payload MonitorDatasourcePayload) error {
	var ds model.MonitorDatasource
	if id > 0 {
		item, err := s.GetMonitorDatasource(id)
		if err != nil {
			return err
		}
		ds = *item
	} else {
		ds = model.MonitorDatasource{
			Name: payload.Name, Type: normalizeMonitorDatasourceType(payload.Type), URL: strings.TrimRight(strings.TrimSpace(payload.URL), "/"),
			AuthType: normalizeMonitorAuthType(payload.AuthType), Username: payload.Username, Password: payload.Password, Token: payload.Token,
		}
	}
	_, err := s.prometheusQuery(ds, "up", time.Now())
	return err
}

func (s *Service) PrometheusInstantQuery(datasourceID uint, query string, ts time.Time) (map[string]any, error) {
	if strings.TrimSpace(query) == "" {
		return nil, errors.New("PromQL 不能为空")
	}
	ds, err := s.GetMonitorDatasource(datasourceID)
	if err != nil {
		return nil, err
	}
	result, err := s.prometheusQuery(*ds, query, ts)
	status := "success"
	errorText := ""
	if err != nil {
		status = "failed"
		errorText = err.Error()
	}
	_ = s.db.Create(&model.MonitorQueryHistory{
		DatasourceID: ds.ID, DatasourceName: ds.Name, Query: query, QueryType: "instant", Status: status, ErrorText: errorText,
	}).Error
	if err != nil {
		return nil, err
	}
	return map[string]any{"resultType": result.Data.ResultType, "result": result.Data.Result}, nil
}

func (s *Service) ListMonitorQueryHistories(pageNum, pageSize int, keyword, status string) (map[string]any, error) {
	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	query := s.db.Model(&model.MonitorQueryHistory{})
	if strings.TrimSpace(keyword) != "" {
		like := "%" + strings.TrimSpace(keyword) + "%"
		query = query.Where("datasource_name LIKE ? OR query LIKE ? OR error_text LIKE ?", like, like, like)
	}
	if strings.TrimSpace(status) != "" {
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var list []model.MonitorQueryHistory
	if err := query.Order("id DESC").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, err
	}
	return map[string]any{"list": list, "total": total, "pageNum": pageNum, "pageSize": pageSize}, nil
}

func (s *Service) prometheusQuery(ds model.MonitorDatasource, query string, ts time.Time) (*PromQueryResult, error) {
	endpoint := strings.TrimRight(ds.URL, "/") + "/api/v1/query"
	params := url.Values{}
	params.Set("query", query)
	if !ts.IsZero() {
		params.Set("time", strconv.FormatFloat(float64(ts.Unix()), 'f', -1, 64))
	}
	request, err := http.NewRequest(http.MethodGet, endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	applyMonitorDatasourceAuth(request, ds)
	client := &http.Client{Timeout: 15 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(response.Body, 4*1024*1024))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("Prometheus API 返回状态码 %d: %s", response.StatusCode, string(body))
	}
	var result PromQueryResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	if result.Status != "success" {
		return nil, errors.New(firstNonEmpty(result.Error, "Prometheus query failed"))
	}
	return &result, nil
}

func applyMonitorDatasourceAuth(request *http.Request, ds model.MonitorDatasource) {
	switch normalizeMonitorAuthType(ds.AuthType) {
	case "basic":
		request.SetBasicAuth(ds.Username, ds.Password)
	case "bearer":
		if strings.TrimSpace(ds.Token) != "" {
			request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(ds.Token))
		}
	}
}

func (s *Service) ListMonitorAlertRules(pageNum, pageSize int, keyword, status, severity string) (map[string]any, error) {
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
		query = query.Where("status = ?", status)
	}
	if strings.TrimSpace(severity) != "" {
		query = query.Where("severity = ?", normalizeSeverity(severity))
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
		return errors.New("规则名称不能为空")
	}
	if payload.DatasourceID == 0 {
		return errors.New("请选择数据源")
	}
	if strings.TrimSpace(payload.PromQL) == "" {
		return errors.New("PromQL 不能为空")
	}
	labelsJSON, err := ensureJSONObject(payload.LabelsJSON)
	if err != nil {
		return err
	}
	annotationsJSON, err := ensureJSONObject(payload.AnnotationsJSON)
	if err != nil {
		return err
	}
	ds, err := s.GetMonitorDatasource(payload.DatasourceID)
	if err != nil {
		return err
	}
	updates := map[string]any{
		"name":                    Trimmed(payload.Name),
		"datasource_id":           ds.ID,
		"datasource_name":         ds.Name,
		"prom_ql":                 strings.TrimSpace(payload.PromQL),
		"comparator":              normalizeComparator(payload.Comparator),
		"threshold":               payload.Threshold,
		"for_seconds":             normalizeForSeconds(payload.ForSeconds),
		"eval_interval_seconds":   normalizeEvalInterval(payload.EvalIntervalSeconds),
		"severity":                normalizeSeverity(payload.Severity),
		"labels_json":             labelsJSON,
		"annotations_json":        annotationsJSON,
		"notify_enabled":          payload.NotifyEnabled,
		"notify_rule_id":          payload.NotifyRuleID,
		"notify_recovery_enabled": payload.NotifyRecoveryEnabled,
		"status":                  normalizeMonitorStatus(payload.Status),
		"description":             Trimmed(payload.Description),
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
	return nil
}

func (s *Service) DeleteMonitorAlertRule(id uint) error {
	s.removeMonitorAlertRule(id)
	return s.db.Delete(&model.MonitorAlertRule{}, id).Error
}

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
			"startsAt": item.StartsAt, "endsAt": item.EndsAt, "status": item.Status, "description": item.Description,
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
		"ruleNamePattern": item.RuleNamePattern, "severity": item.Severity, "matchersJson": item.MatchersJSON,
		"startsAt": item.StartsAt, "endsAt": item.EndsAt, "status": item.Status, "description": item.Description,
		"createTime": item.CreatedAt, "updateTime": item.UpdatedAt,
	}, nil
}

func (s *Service) SaveMonitorSilenceRule(payload MonitorSilenceRulePayload) error {
	if strings.TrimSpace(payload.Name) == "" {
		return errors.New("silence rule name is required")
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
		"matchers_json":     matchersJSON,
		"starts_at":         unixPtr(payload.StartsAt),
		"ends_at":           unixPtr(payload.EndsAt),
		"status":            normalizeMonitorStatus(payload.Status),
		"description":       Trimmed(payload.Description),
	}
	if payload.ID > 0 {
		return s.db.Model(&model.MonitorSilenceRule{}).Where("id = ?", payload.ID).Updates(updates).Error
	}
	return s.db.Model(&model.MonitorSilenceRule{}).Create(updates).Error
}

func (s *Service) DeleteMonitorSilenceRule(id uint) error {
	return s.db.Delete(&model.MonitorSilenceRule{}, id).Error
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
		"ruleNamePattern": item.RuleNamePattern, "severity": item.Severity,
		"groupBy": decodeStringList(item.GroupByJSON), "groupByJson": item.GroupByJSON,
		"windowSeconds": item.WindowSeconds, "repeatIntervalSeconds": item.RepeatIntervalSeconds,
		"status": item.Status, "description": item.Description, "createTime": item.CreatedAt, "updateTime": item.UpdatedAt,
	}, nil
}

func (s *Service) SaveMonitorAggregationRule(payload MonitorAggregationRulePayload) error {
	if strings.TrimSpace(payload.Name) == "" {
		return errors.New("aggregation rule name is required")
	}
	groupBy := payload.GroupBy
	if len(groupBy) == 0 {
		groupBy = []string{"alertname", "instance"}
	}
	updates := map[string]any{
		"name":                    Trimmed(payload.Name),
		"match_mode":              normalizeRuleMatchMode(payload.MatchMode),
		"rule_ids_json":           encodeUintList(payload.RuleIDs),
		"rule_name_pattern":       strings.TrimSpace(payload.RuleNamePattern),
		"severity":                strings.TrimSpace(payload.Severity),
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
	return nil
}

func (s *Service) RunMonitorAlertRule(id uint) error {
	go s.evaluateMonitorAlertRule(id)
	return nil
}

func (s *Service) evaluateMonitorAlertRule(id uint) {
	rule, err := s.GetMonitorAlertRule(id)
	if err != nil || rule.Status != 1 {
		return
	}
	ds, err := s.GetMonitorDatasource(rule.DatasourceID)
	if err != nil {
		s.updateMonitorRuleEval(*rule, "failed", err.Error())
		return
	}
	result, err := s.prometheusQuery(*ds, rule.PromQL, time.Now())
	if err != nil {
		s.updateMonitorRuleEval(*rule, "failed", err.Error())
		return
	}
	activeFingerprints := map[string]bool{}
	for _, sample := range result.Data.Result {
		value, ok := promSampleValue(sample)
		if !ok {
			continue
		}
		if !compareFloat(value, rule.Comparator, rule.Threshold) {
			continue
		}
		fp := monitorFingerprint(rule.ID, sample.Metric)
		activeFingerprints[fp] = true
		s.upsertMonitorAlertEvent(*rule, sample, fp, value)
	}
	s.recoverInactiveMonitorEvents(*rule, activeFingerprints)
	s.updateMonitorRuleEval(*rule, "success", fmt.Sprintf("matched %d series", len(activeFingerprints)))
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

func (s *Service) matchMonitorSilenceRule(rule model.MonitorAlertRule, labels map[string]string) (*model.MonitorSilenceRule, bool) {
	now := time.Now()
	var rules []model.MonitorSilenceRule
	if err := s.db.Where("status = ?", 1).Order("id DESC").Find(&rules).Error; err != nil {
		return nil, false
	}
	for _, item := range rules {
		if item.StartsAt != nil && now.Before(*item.StartsAt) {
			continue
		}
		if item.EndsAt != nil && now.After(*item.EndsAt) {
			continue
		}
		if !monitorRuleMatch(item.MatchMode, item.RuleIDsJSON, item.RuleNamePattern, rule) || !monitorSeverityMatch(item.Severity, rule.Severity) {
			continue
		}
		if !monitorMatchersMatch(item.MatchersJSON, labels) {
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
		if !monitorRuleMatch(item.MatchMode, item.RuleIDsJSON, item.RuleNamePattern, rule) || !monitorSeverityMatch(item.Severity, rule.Severity) {
			continue
		}
		groupBy := decodeStringList(item.GroupByJSON)
		if len(groupBy) == 0 {
			groupBy = []string{"alertname", "instance"}
		}
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
	windowStart := time.Now().Add(-time.Duration(normalizeAggregationWindow(aggregation.WindowSeconds)) * time.Second)
	var last model.MonitorAlertEvent
	err := s.db.Where("aggregation_key = ? AND id <> ? AND last_notify_at >= ?", event.AggregationKey, event.ID, windowStart).
		Order("last_notify_at DESC").First(&last).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return true
	}
	if err != nil || last.LastNotifyAt == nil {
		return true
	}
	repeatAfter := time.Duration(normalizeAggregationWindow(aggregation.RepeatIntervalSeconds)) * time.Second
	return time.Since(*last.LastNotifyAt) >= repeatAfter
}

func (s *Service) upsertMonitorAlertEvent(rule model.MonitorAlertRule, sample PromMetricSample, fp string, value float64) {
	now := time.Now()
	labelsBytes, _ := json.Marshal(sample.Metric)
	summary := fmt.Sprintf("%s 当前值 %.4f %s %.4f", rule.Name, value, rule.Comparator, rule.Threshold)
	silenceRule, silenced := s.matchMonitorSilenceRule(rule, sample.Metric)
	aggregationRule, aggregationKey, aggregated := s.matchMonitorAggregationRule(rule, sample.Metric)
	var existing model.MonitorAlertEvent
	err := s.db.Where("rule_id = ? AND fingerprint = ? AND status IN ?", rule.ID, fp, []string{"firing", "claimed"}).First(&existing).Error
	if err == nil {
		updates := map[string]any{
			"current_value": value, "last_trigger_at": now, "summary": summary,
		}
		if silenced && silenceRule != nil {
			updates["silenced"] = true
			updates["silence_rule_id"] = silenceRule.ID
			updates["silence_rule_name"] = silenceRule.Name
			updates["status"] = "silenced"
		}
		if aggregated && aggregationRule != nil {
			updates["aggregation_key"] = aggregationKey
			updates["aggregate_rule_id"] = aggregationRule.ID
			updates["aggregate_rule_name"] = aggregationRule.Name
		}
		_ = s.db.Model(&model.MonitorAlertEvent{}).Where("id = ?", existing.ID).Updates(updates).Error
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
	if aggregated && aggregationRule != nil {
		event.AggregationKey = aggregationKey
		event.AggregateRuleID = aggregationRule.ID
		event.AggregateRuleName = aggregationRule.Name
	}
	if err := s.db.Create(&event).Error; err == nil && !event.Silenced && rule.NotifyEnabled && rule.NotifyRuleID > 0 {
		if s.shouldNotifyAggregatedEvent(event, aggregationRule) {
			notifyAt := time.Now()
			_ = s.db.Model(&model.MonitorAlertEvent{}).Where("id = ?", event.ID).Update("last_notify_at", &notifyAt).Error
			s.dispatchMonitorNotification(rule, event, "firing")
		}
	}
}

func (s *Service) recoverInactiveMonitorEvents(rule model.MonitorAlertRule, active map[string]bool) {
	var events []model.MonitorAlertEvent
	if err := s.db.Where("rule_id = ? AND status IN ?", rule.ID, []string{"firing", "claimed", "silenced"}).Find(&events).Error; err != nil {
		return
	}
	now := time.Now()
	for _, event := range events {
		if active[event.Fingerprint] {
			continue
		}
		_ = s.db.Model(&model.MonitorAlertEvent{}).Where("id = ?", event.ID).Updates(map[string]any{
			"status": "recovered", "recovered_at": &now,
		}).Error
		event.Status = "recovered"
		event.RecoveredAt = &now
		if rule.NotifyEnabled && rule.NotifyRuleID > 0 && rule.NotifyRecoveryEnabled {
			s.dispatchMonitorNotification(rule, event, "recovered")
		}
	}
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

func (s *Service) ClaimMonitorAlertEvent(payload MonitorAlertEventActionPayload) error {
	if payload.ID == 0 {
		return errors.New("告警事件 ID 不能为空")
	}
	return s.db.Model(&model.MonitorAlertEvent{}).Where("id = ?", payload.ID).Updates(map[string]any{
		"status": "claimed", "claimed_by": strings.TrimSpace(payload.ClaimedBy), "handle_note": strings.TrimSpace(payload.HandleNote),
	}).Error
}

func (s *Service) ResolveMonitorAlertEvent(payload MonitorAlertEventActionPayload) error {
	if payload.ID == 0 {
		return errors.New("告警事件 ID 不能为空")
	}
	now := time.Now()
	return s.db.Model(&model.MonitorAlertEvent{}).Where("id = ?", payload.ID).Updates(map[string]any{
		"status": "resolved", "handle_note": strings.TrimSpace(payload.HandleNote), "recovered_at": &now,
	}).Error
}

func (s *Service) GetMonitorOverview() (map[string]any, error) {
	var datasourceCount, ruleCount, firingCount, recoveredCount int64
	_ = s.db.Model(&model.MonitorDatasource{}).Count(&datasourceCount).Error
	_ = s.db.Model(&model.MonitorAlertRule{}).Count(&ruleCount).Error
	_ = s.db.Model(&model.MonitorAlertEvent{}).Where("status IN ?", []string{"firing", "claimed"}).Count(&firingCount).Error
	_ = s.db.Model(&model.MonitorAlertEvent{}).Where("status = ?", "recovered").Count(&recoveredCount).Error
	severityRows := []map[string]any{}
	for _, severity := range []string{"P0", "P1", "P2", "P3"} {
		var count int64
		_ = s.db.Model(&model.MonitorAlertEvent{}).Where("status IN ? AND severity = ?", []string{"firing", "claimed"}, severity).Count(&count).Error
		severityRows = append(severityRows, map[string]any{"severity": severity, "count": count})
	}
	var recent []model.MonitorAlertEvent
	_ = s.db.Order("last_trigger_at DESC, id DESC").Limit(8).Find(&recent).Error
	return map[string]any{
		"datasourceCount": datasourceCount, "ruleCount": ruleCount, "firingCount": firingCount,
		"recoveredCount": recoveredCount, "severity": severityRows, "recentEvents": recent,
	}, nil
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

func (s *Service) SaveMonitorDashboard(payload MonitorDashboardPayload) error {
	if strings.TrimSpace(payload.Name) == "" {
		return errors.New("dashboard name is required")
	}
	updates := map[string]any{
		"name":        Trimmed(payload.Name),
		"layout":      normalizeDashboardLayout(payload.Layout),
		"status":      normalizeMonitorStatus(payload.Status),
		"description": Trimmed(payload.Description),
	}
	if payload.ID > 0 {
		return s.db.Model(&model.MonitorDashboard{}).Where("id = ?", payload.ID).Updates(updates).Error
	}
	return s.db.Model(&model.MonitorDashboard{}).Create(updates).Error
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

func (s *Service) QueryMonitorDashboardPanel(id uint) (map[string]any, error) {
	var panel model.MonitorDashboardPanel
	if err := s.db.First(&panel, id).Error; err != nil {
		return nil, err
	}
	ds, err := s.GetMonitorDatasource(panel.DatasourceID)
	if err != nil {
		return nil, err
	}
	result, err := s.prometheusQuery(*ds, panel.PromQL, time.Now())
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"panel": panel, "resultType": result.Data.ResultType, "result": result.Data.Result,
	}, nil
}
