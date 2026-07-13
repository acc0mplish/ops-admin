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
	Env         string `json:"env"`
	Status      int    `json:"status"`
	Description string `json:"description"`
}

type MonitorAlertRulePayload struct {
	ID                          uint    `json:"id"`
	Name                        string  `json:"name"`
	AlertType                   string  `json:"alertType"`
	DatasourceScope             string  `json:"datasourceScope"`
	DatasourceID                uint    `json:"datasourceId"`
	PromQL                      string  `json:"promql"`
	Query                       string  `json:"query"`
	LogIndex                    string  `json:"logIndex"`
	LogTimeRangeSeconds         int     `json:"logTimeRangeSeconds"`
	Comparator                  string  `json:"comparator"`
	Threshold                   float64 `json:"threshold"`
	ForSeconds                  int     `json:"forSeconds"`
	EvalIntervalSeconds         int     `json:"evalIntervalSeconds"`
	NotifyRepeatIntervalSeconds int     `json:"notifyRepeatIntervalSeconds"`
	MaxNotifyCount              int     `json:"maxNotifyCount"`
	Severity                    string  `json:"severity"`
	LabelsJSON                  string  `json:"labelsJson"`
	AnnotationsJSON             string  `json:"annotationsJson"`
	NotifyEnabled               bool    `json:"notifyEnabled"`
	NotifyRuleID                uint    `json:"notifyRuleId"`
	NotifyRecoveryEnabled       bool    `json:"notifyRecoveryEnabled"`
	Env                         string  `json:"env"`
	Status                      int     `json:"status"`
	Description                 string  `json:"description"`
}

type MonitorAlertRuleBatchPayload struct {
	IDs          []uint `json:"ids"`
	Action       string `json:"action"`
	NotifyRuleID uint   `json:"notifyRuleId"`
}

type MonitorAlertEventActionPayload struct {
	ID         uint   `json:"id"`
	ClaimedBy  string `json:"claimedBy"`
	HandleNote string `json:"handleNote"`
}

type MonitorAlertEventBatchPayload struct {
	IDs        []uint `json:"ids"`
	Action     string `json:"action"`
	ClaimedBy  string `json:"claimedBy"`
	HandleNote string `json:"handleNote"`
}

type MonitorRuleBatchPayload struct {
	IDs    []uint `json:"ids"`
	Action string `json:"action"`
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

type MonitorDashboardPanelQueryPayload struct {
	ID           uint  `json:"id"`
	DatasourceID uint  `json:"datasourceId"`
	StartAt      int64 `json:"startAt"`
	EndAt        int64 `json:"endAt"`
	StepSeconds  int   `json:"stepSeconds"`
}

type MonitorLogQueryPayload struct {
	DatasourceID uint   `json:"datasourceId"`
	Index        string `json:"index"`
	Query        string `json:"query"`
	StartAt      int64  `json:"startAt"`
	EndAt        int64  `json:"endAt"`
	PageSize     int    `json:"pageSize"`
}

type MonitorLogShortcutPayload struct {
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	Query     string `json:"query"`
	IndexName string `json:"indexName"`
	TimeRange string `json:"timeRange"`
	Sort      int    `json:"sort"`
}

type MonitorScheduler struct {
	cron        *cron.Cron
	mu          sync.Mutex
	entries     map[uint]cron.EntryID
	running     map[uint]bool
	healthEntry cron.EntryID
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

var springLogPattern = regexp.MustCompile(`(?s)^\s*(\S+\s+\S+)\s+(TRACE|DEBUG|INFO|WARN|ERROR|FATAL)\s+(\S+)\s+---\s+\[([^\]]*)\]\s+(.+?)\s*:\s*(.*)$`)

func normalizeMonitorDatasourceType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "victoriametrics", "victoria-metrics", "vm":
		return "victoriametrics"
	case "elasticsearch", "elastic", "es":
		return "elasticsearch"
	default:
		return "prometheus"
	}
}

func normalizeAlertType(value string) string {
	if strings.EqualFold(strings.TrimSpace(value), "log") {
		return "log"
	}
	return "metric"
}

func normalizeDatasourceScope(value string) string {
	if strings.EqualFold(strings.TrimSpace(value), "all") {
		return "all"
	}
	return "specific"
}

func normalizeLogTimeRangeSeconds(value int) int {
	if value < 60 {
		return 300
	}
	if value > 86400 {
		return 86400
	}
	return value
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
	case "apikey", "api_key", "api-key":
		return "apikey"
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

func normalizeNotifyRepeatInterval(value int) int {
	if value < 60 {
		return 60
	}
	if value > 604800 {
		return 604800
	}
	return value
}

func normalizeMaxNotifyCount(value int) int {
	if value < 0 {
		return 0
	}
	if value > 1000 {
		return 1000
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
			running: map[uint]bool{},
		}
		s.monitorScheduler.cron.Start()
		s.reloadMonitorAlertRules()
		entryID, err := s.monitorScheduler.cron.AddFunc("@every 60s", s.checkAllMonitorDatasources)
		if err == nil {
			s.monitorScheduler.healthEntry = entryID
		}
		go s.checkAllMonitorDatasources()
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

func (s *Service) ListMonitorDatasources(pageNum, pageSize int, keyword, dsType, status, env string) (map[string]any, error) {
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
	if strings.TrimSpace(env) != "" {
		query = query.Where("env = ?", normalizeEnvCode(env))
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
		"env":         normalizeEnvCode(payload.Env),
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
	if err := s.db.Model(&model.MonitorDashboardPanel{}).Where("datasource_id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return errors.New("数据源已被监控大屏引用，不能删除")
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
	startedAt := time.Now()
	err := s.checkMonitorDatasourceHealth(ds)
	if id > 0 {
		s.persistMonitorDatasourceHealth(id, err, time.Since(startedAt).Milliseconds())
	}
	return err
}

func (s *Service) checkMonitorDatasourceHealth(ds model.MonitorDatasource) error {
	if normalizeMonitorDatasourceType(ds.Type) == "elasticsearch" {
		return s.elasticsearchHealth(ds)
	}
	_, err := s.prometheusQuery(ds, "up", time.Now())
	return err
}

func (s *Service) persistMonitorDatasourceHealth(id uint, healthErr error, latencyMs int64) {
	now := time.Now()
	updates := map[string]any{"last_check_at": &now, "latency_ms": latencyMs}
	if healthErr == nil {
		updates["health_status"] = "healthy"
		updates["last_success_at"] = &now
		updates["last_error"] = ""
		updates["consecutive_failures"] = 0
	} else {
		updates["health_status"] = "unhealthy"
		updates["last_error"] = healthErr.Error()
		updates["consecutive_failures"] = gorm.Expr("consecutive_failures + ?", 1)
	}
	_ = s.db.Model(&model.MonitorDatasource{}).Where("id = ?", id).Updates(updates).Error
}

func (s *Service) checkAllMonitorDatasources() {
	var datasources []model.MonitorDatasource
	if err := s.db.Where("status = ?", 1).Find(&datasources).Error; err != nil {
		return
	}
	for _, ds := range datasources {
		startedAt := time.Now()
		err := s.checkMonitorDatasourceHealth(ds)
		s.persistMonitorDatasourceHealth(ds.ID, err, time.Since(startedAt).Milliseconds())
	}
}

func (s *Service) PrometheusInstantQuery(datasourceID uint, query string, ts time.Time) (map[string]any, error) {
	return s.MonitorInstantQuery(datasourceID, query, ts)
}

func (s *Service) MonitorInstantQuery(datasourceID uint, query string, ts time.Time) (map[string]any, error) {
	if strings.TrimSpace(query) == "" {
		return nil, errors.New("查询语句不能为空")
	}
	ds, err := s.GetMonitorDatasource(datasourceID)
	if err != nil {
		return nil, err
	}
	queryType := "promql"
	var response map[string]any
	if normalizeMonitorDatasourceType(ds.Type) == "elasticsearch" {
		queryType = "elasticsearch"
		response, err = s.elasticsearchQuery(*ds, query)
	} else {
		var result *PromQueryResult
		result, err = s.prometheusQuery(*ds, query, ts)
		if err == nil {
			response = map[string]any{"resultType": result.Data.ResultType, "result": result.Data.Result}
		}
	}
	status := "success"
	errorText := ""
	if err != nil {
		status = "failed"
		errorText = err.Error()
	}
	_ = s.db.Create(&model.MonitorQueryHistory{
		DatasourceID: ds.ID, DatasourceName: ds.Name, Query: query, QueryType: queryType, Status: status, ErrorText: errorText,
	}).Error
	if err != nil {
		return nil, err
	}
	return response, nil
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

func (s *Service) prometheusRangeQuery(ds model.MonitorDatasource, query string, startAt, endAt time.Time, stepSeconds int) (*PromQueryResult, error) {
	if endAt.Before(startAt) || endAt.Equal(startAt) {
		return nil, errors.New("查询结束时间必须晚于开始时间")
	}
	if stepSeconds < 15 {
		stepSeconds = 15
	}
	if stepSeconds > 3600 {
		stepSeconds = 3600
	}
	endpoint := strings.TrimRight(ds.URL, "/") + "/api/v1/query_range"
	params := url.Values{}
	params.Set("query", query)
	params.Set("start", strconv.FormatInt(startAt.Unix(), 10))
	params.Set("end", strconv.FormatInt(endAt.Unix(), 10))
	params.Set("step", strconv.Itoa(stepSeconds))
	request, err := http.NewRequest(http.MethodGet, endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	applyMonitorDatasourceAuth(request, ds)
	response, err := (&http.Client{Timeout: 30 * time.Second}).Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(response.Body, 8*1024*1024))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("Prometheus API 返回状态码 %d: %s", response.StatusCode, string(body))
	}
	var result PromQueryResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	if result.Status != "success" {
		return nil, errors.New(firstNonEmpty(result.Error, "Prometheus range query failed"))
	}
	return &result, nil
}

func (s *Service) elasticsearchHealth(ds model.MonitorDatasource) error {
	request, err := http.NewRequest(http.MethodGet, strings.TrimRight(ds.URL, "/")+"/_cluster/health", nil)
	if err != nil {
		return err
	}
	applyMonitorDatasourceAuth(request, ds)
	response, err := (&http.Client{Timeout: 15 * time.Second}).Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(response.Body, 1024*1024))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("Elasticsearch 健康检查失败，状态码 %d: %s", response.StatusCode, string(body))
	}
	return nil
}

func (s *Service) elasticsearchQuery(ds model.MonitorDatasource, query string) (map[string]any, error) {
	payload := map[string]any{}
	if err := json.Unmarshal([]byte(query), &payload); err != nil {
		return nil, errors.New("Elasticsearch DSL 必须是有效的 JSON 对象")
	}
	index := strings.TrimSpace(fmt.Sprint(payload["index"]))
	delete(payload, "index")
	if index == "" || index == "<nil>" {
		index = "_all"
	}
	if strings.Contains(index, "/") || strings.Contains(index, "\\") {
		return nil, errors.New("Elasticsearch 索引不能包含路径分隔符")
	}
	if _, exists := payload["size"]; !exists {
		payload["size"] = 100
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	endpoint := strings.TrimRight(ds.URL, "/") + "/" + url.PathEscape(index) + "/_search"
	request, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	applyMonitorDatasourceAuth(request, ds)
	response, err := (&http.Client{Timeout: 30 * time.Second}).Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	responseBody, _ := io.ReadAll(io.LimitReader(response.Body, 8*1024*1024))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("Elasticsearch 查询失败，状态码 %d: %s", response.StatusCode, string(responseBody))
	}
	var raw map[string]any
	if err := json.Unmarshal(responseBody, &raw); err != nil {
		return nil, err
	}
	hits, _ := raw["hits"].(map[string]any)
	documents, _ := hits["hits"].([]any)
	return map[string]any{
		"resultType": "elasticsearch",
		"result":     documents,
		"total":      hits["total"],
		"took":       raw["took"],
	}, nil
}

func (s *Service) QueryMonitorLogs(payload MonitorLogQueryPayload) (map[string]any, error) {
	if payload.DatasourceID == 0 {
		return nil, errors.New("请选择 Elasticsearch 数据源")
	}
	ds, err := s.GetMonitorDatasource(payload.DatasourceID)
	if err != nil {
		return nil, err
	}
	if normalizeMonitorDatasourceType(ds.Type) != "elasticsearch" {
		return nil, errors.New("日志查询仅支持 Elasticsearch 数据源")
	}
	index := strings.TrimSpace(payload.Index)
	if index == "" {
		index = "_all"
	}
	if strings.Contains(index, "/") || strings.Contains(index, "\\") {
		return nil, errors.New("索引不能包含路径分隔符")
	}
	pageSize := payload.PageSize
	if pageSize < 1 {
		pageSize = 100
	}
	if pageSize > 500 {
		pageSize = 500
	}
	endAt := payload.EndAt
	if endAt <= 0 {
		endAt = time.Now().UnixMilli()
	}
	startAt := payload.StartAt
	if startAt <= 0 || startAt >= endAt {
		startAt = time.UnixMilli(endAt).Add(-24 * time.Hour).UnixMilli()
	}
	must := make([]any, 0, 1)
	if strings.TrimSpace(payload.Query) == "" {
		must = append(must, map[string]any{"match_all": map[string]any{}})
	} else {
		must = append(must, map[string]any{"query_string": map[string]any{"query": strings.TrimSpace(payload.Query), "analyze_wildcard": true}})
	}
	body := map[string]any{
		"size": pageSize,
		"sort": []any{map[string]any{"@timestamp": map[string]any{"order": "desc", "unmapped_type": "date"}}},
		"query": map[string]any{"bool": map[string]any{
			"must":   must,
			"filter": []any{map[string]any{"range": map[string]any{"@timestamp": map[string]any{"gte": startAt, "lte": endAt, "format": "epoch_millis"}}}},
		}},
		"aggs": map[string]any{"histogram": map[string]any{"date_histogram": map[string]any{
			"field": "@timestamp", "fixed_interval": "1h", "min_doc_count": 0,
		}}},
	}
	response, err := s.elasticsearchSearch(*ds, index, body)
	if err != nil {
		return nil, err
	}
	hits, _ := response["hits"].(map[string]any)
	rawItems, _ := hits["hits"].([]any)
	items := make([]map[string]any, 0, len(rawItems))
	for _, rawItem := range rawItems {
		hit, _ := rawItem.(map[string]any)
		source, _ := hit["_source"].(map[string]any)
		kubernetes, _ := source["kubernetes"].(map[string]any)
		rawMessage := cleanMonitorLogMessage(firstNonEmpty(monitorSourceString(source["message"]), monitorSourceString(source["log"])))
		messageFields := parseMonitorLogMessage(rawMessage)
		level := firstNonEmpty(monitorSourceString(source["level"]), messageFields["level"], detectMonitorLogLevel(rawMessage))
		items = append(items, map[string]any{
			"index":      hit["_index"],
			"id":         hit["_id"],
			"timestamp":  firstNonEmpty(monitorSourceString(source["@timestamp"]), monitorSourceString(source["timestamp"])),
			"namespace":  firstNonEmpty(monitorSourceString(kubernetes["pod_namespace"]), monitorSourceString(source["namespace"])),
			"pod":        firstNonEmpty(monitorSourceString(kubernetes["pod_name"]), monitorSourceString(source["pod"])),
			"container":  firstNonEmpty(monitorSourceString(kubernetes["container_name"]), monitorSourceString(source["container"])),
			"level":      level,
			"message":    messageFields["content"],
			"messageRaw": rawMessage,
			"logTime":    messageFields["timestamp"],
			"processId":  messageFields["processId"],
			"thread":     messageFields["thread"],
			"logger":     messageFields["logger"],
			"source":     source,
		})
	}
	aggs, _ := response["aggregations"].(map[string]any)
	histogram, _ := aggs["histogram"].(map[string]any)
	buckets, _ := histogram["buckets"].([]any)
	return map[string]any{
		"items": items, "total": hits["total"], "took": response["took"], "histogram": buckets,
		"startAt": startAt, "endAt": endAt,
	}, nil
}

func (s *Service) ListMonitorElasticsearchIndices(datasourceID uint) ([]map[string]any, error) {
	if datasourceID == 0 {
		return nil, errors.New("请选择 Elasticsearch 数据源")
	}
	ds, err := s.GetMonitorDatasource(datasourceID)
	if err != nil {
		return nil, err
	}
	if normalizeMonitorDatasourceType(ds.Type) != "elasticsearch" {
		return nil, errors.New("当前数据源不是 Elasticsearch")
	}
	endpoint := strings.TrimRight(ds.URL, "/") + "/_cat/indices?format=json&h=health,status,index,docs.count,store.size&s=index"
	request, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	applyMonitorDatasourceAuth(request, *ds)
	response, err := (&http.Client{Timeout: 15 * time.Second}).Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(response.Body, 4*1024*1024))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("获取 Elasticsearch 索引失败，状态码 %d: %s", response.StatusCode, string(body))
	}
	var raw []map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	result := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		name := monitorSourceString(item["index"])
		if name == "" || strings.HasPrefix(name, ".security") {
			continue
		}
		result = append(result, map[string]any{
			"name": name, "health": monitorSourceString(item["health"]), "status": monitorSourceString(item["status"]),
			"docsCount": monitorSourceString(item["docs.count"]), "storeSize": monitorSourceString(item["store.size"]),
		})
	}
	return result, nil
}

func (s *Service) ListMonitorLogShortcuts(owner string) ([]model.MonitorLogShortcut, error) {
	owner = firstNonEmpty(strings.TrimSpace(owner), "admin")
	var count int64
	if err := s.db.Model(&model.MonitorLogShortcut{}).Where("owner = ?", owner).Count(&count).Error; err != nil {
		return nil, err
	}
	if count == 0 {
		defaults := []struct {
			name, query, index, rangeText string
		}{
			{"全部日志", "", "_all", "24h"},
			{"错误日志", "ERROR", "_all", "24h"},
			{"异常与堆栈", "(Exception OR ERROR OR Caused\\ by)", "_all", "24h"},
			{"告警与警告", "(WARN OR WARNING)", "_all", "24h"},
			{"超时请求", "(timeout OR timed\\ out OR TimeoutException)", "_all", "24h"},
			{"连接失败", "(connection\\ refused OR connection\\ reset OR connect\\ timeout)", "_all", "24h"},
			{"Kubernetes 重启", "(CrashLoopBackOff OR OOMKilled OR Back-off\\ restarting)", "_all", "24h"},
			{"应用启动", "(Started\\ .*Application OR application\\ started)", "_all", "24h"},
			{"数据库慢查询", "(slow\\ query OR SlowQuery OR SQL\\ took)", "_all", "24h"},
			{"指定命名空间", "kubernetes.pod_namespace:\"default\"", "_all", "6h"},
		}
		items := make([]model.MonitorLogShortcut, 0, len(defaults))
		for i, item := range defaults {
			items = append(items, model.MonitorLogShortcut{Owner: owner, Name: item.name, Query: item.query, IndexName: item.index, TimeRange: item.rangeText, Sort: i + 1})
		}
		if err := s.db.Create(&items).Error; err != nil {
			return nil, err
		}
	}
	var list []model.MonitorLogShortcut
	if err := s.db.Where("owner = ?", owner).Order("sort ASC, id ASC").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

func (s *Service) SaveMonitorLogShortcut(owner string, payload MonitorLogShortcutPayload) error {
	if strings.TrimSpace(payload.Name) == "" {
		return errors.New("快捷语句名称不能为空")
	}
	owner = firstNonEmpty(strings.TrimSpace(owner), "admin")
	updates := map[string]any{
		"name": Trimmed(payload.Name), "query": strings.TrimSpace(payload.Query),
		"index_name": firstNonEmpty(strings.TrimSpace(payload.IndexName), "_all"),
		"time_range": firstNonEmpty(strings.TrimSpace(payload.TimeRange), "24h"), "sort": payload.Sort,
	}
	if payload.ID > 0 {
		return s.db.Model(&model.MonitorLogShortcut{}).Where("id = ? AND owner = ?", payload.ID, owner).Updates(updates).Error
	}
	return s.db.Create(&model.MonitorLogShortcut{Owner: owner, Name: updates["name"].(string), Query: updates["query"].(string), IndexName: updates["index_name"].(string), TimeRange: updates["time_range"].(string), Sort: payload.Sort}).Error
}

func (s *Service) DeleteMonitorLogShortcut(owner string, id uint) error {
	if id == 0 {
		return errors.New("请选择快捷语句")
	}
	owner = firstNonEmpty(strings.TrimSpace(owner), "admin")
	result := s.db.Where("id = ? AND owner = ?", id, owner).Delete(&model.MonitorLogShortcut{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("快捷语句不存在或无权删除")
	}
	return nil
}

func (s *Service) elasticsearchSearch(ds model.MonitorDatasource, index string, payload map[string]any) (map[string]any, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	endpoint := strings.TrimRight(ds.URL, "/") + "/" + url.PathEscape(index) + "/_search"
	request, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	applyMonitorDatasourceAuth(request, ds)
	response, err := (&http.Client{Timeout: 30 * time.Second}).Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	responseBody, _ := io.ReadAll(io.LimitReader(response.Body, 8*1024*1024))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("Elasticsearch 查询失败，状态码 %d: %s", response.StatusCode, string(responseBody))
	}
	var result map[string]any
	if err := json.Unmarshal(responseBody, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func cleanMonitorLogMessage(value string) string {
	value = strings.TrimSpace(value)
	ansi := regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)
	return ansi.ReplaceAllString(value, "")
}

func parseMonitorLogMessage(value string) map[string]string {
	result := map[string]string{"content": value}
	matches := springLogPattern.FindStringSubmatch(value)
	if len(matches) != 7 {
		return result
	}
	result["timestamp"] = matches[1]
	result["level"] = matches[2]
	result["processId"] = matches[3]
	result["thread"] = matches[4]
	result["logger"] = strings.TrimSpace(matches[5])
	result["content"] = strings.TrimSpace(matches[6])
	return result
}

func monitorSourceString(value any) string {
	if value == nil {
		return ""
	}
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "<nil>" {
		return ""
	}
	return text
}

func detectMonitorLogLevel(message string) string {
	upper := strings.ToUpper(message)
	for _, level := range []string{"FATAL", "ERROR", "WARN", "INFO", "DEBUG", "TRACE"} {
		if strings.Contains(upper, level) {
			return level
		}
	}
	return "-"
}

func applyMonitorDatasourceAuth(request *http.Request, ds model.MonitorDatasource) {
	switch normalizeMonitorAuthType(ds.AuthType) {
	case "basic":
		request.SetBasicAuth(ds.Username, ds.Password)
	case "bearer":
		if strings.TrimSpace(ds.Token) != "" {
			request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(ds.Token))
		}
	case "apikey":
		if strings.TrimSpace(ds.Token) != "" {
			request.Header.Set("Authorization", "ApiKey "+strings.TrimSpace(ds.Token))
		}
	}
}

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
		return errors.New("规则名称不能为空")
	}
	alertType := normalizeAlertType(payload.AlertType)
	datasourceScope := normalizeDatasourceScope(payload.DatasourceScope)
	queryText := firstNonEmpty(strings.TrimSpace(payload.Query), strings.TrimSpace(payload.PromQL))
	if queryText == "" {
		if alertType == "log" {
			return errors.New("Elasticsearch 查询语句不能为空")
		}
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
	datasourceName := ""
	datasourceID := payload.DatasourceID
	if datasourceScope == "specific" {
		if datasourceID == 0 {
			return errors.New("请选择数据源")
		}
		ds, err := s.GetMonitorDatasource(datasourceID)
		if err != nil {
			return err
		}
		if alertType == "log" && normalizeMonitorDatasourceType(ds.Type) != "elasticsearch" {
			return errors.New("日志告警只能选择 Elasticsearch 数据源")
		}
		if alertType == "metric" && normalizeMonitorDatasourceType(ds.Type) == "elasticsearch" {
			return errors.New("监控告警只能选择 Prometheus 或 VictoriaMetrics 数据源")
		}
		datasourceName = ds.Name
	} else {
		var count int64
		if alertType == "log" {
			err = s.db.Model(&model.MonitorDatasource{}).Where("status = ? AND type = ?", 1, "elasticsearch").Count(&count).Error
			datasourceName = "全部日志数据源"
		} else {
			err = s.db.Model(&model.MonitorDatasource{}).Where("status = ? AND type IN ?", 1, []string{"prometheus", "victoriametrics"}).Count(&count).Error
			datasourceName = "全部监控数据源"
		}
		if err != nil {
			return err
		}
		if count == 0 {
			return errors.New("没有可用的匹配数据源")
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
		return nil, errors.New("请选择至少一条规则")
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
		return errors.New("不支持的批量操作")
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
		return errors.New("不支持的批量操作")
	}
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

func (s *Service) BatchUpdateMonitorAlertRules(payload MonitorAlertRuleBatchPayload) error {
	if len(payload.IDs) == 0 {
		return errors.New("请选择至少一条告警规则")
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
		return errors.New("告警规则不能为空")
	}
	updates := map[string]any{}
	action := strings.ToLower(strings.TrimSpace(payload.Action))
	switch action {
	case "enable":
		updates["status"] = 1
	case "disable":
		updates["status"] = 2
	case "enable_notify":
		if payload.NotifyRuleID == 0 {
			return errors.New("请选择通知规则")
		}
		var notifyRule model.NotifyRule
		if err := s.db.Where("id = ? AND status = ?", payload.NotifyRuleID, 1).First(&notifyRule).Error; err != nil {
			return errors.New("通知规则不存在或已禁用")
		}
		updates["notify_enabled"] = true
		updates["notify_rule_id"] = payload.NotifyRuleID
	default:
		return errors.New("不支持的批量操作")
	}
	if err := s.db.Model(&model.MonitorAlertRule{}).Where("id IN ?", uniqueIDs).Updates(updates).Error; err != nil {
		return err
	}
	if action != "enable" && action != "disable" {
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
		} else {
			s.removeMonitorAlertRule(rule.ID)
		}
	}
	return nil
}

func (s *Service) RunMonitorAlertRule(id uint) error {
	go s.evaluateMonitorAlertRule(id)
	return nil
}

func (s *Service) PreviewMonitorAlertRule(payload MonitorAlertRulePayload) (map[string]any, error) {
	queryText := firstNonEmpty(strings.TrimSpace(payload.Query), strings.TrimSpace(payload.PromQL))
	if queryText == "" {
		return nil, errors.New("查询语句不能为空")
	}
	rule := model.MonitorAlertRule{
		ID: payload.ID, Name: firstNonEmpty(strings.TrimSpace(payload.Name), "规则预览"),
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
		if rule.AlertType == "log" {
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
	explanation := fmt.Sprintf("共查询 %d 个数据源，返回 %d 条序列，其中 %d 条满足 %s %.4f", len(datasources), totalSeries, totalMatched, rule.Comparator, rule.Threshold)
	if failedDatasources > 0 {
		explanation += fmt.Sprintf("，%d 个数据源查询失败", failedDatasources)
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
		if normalizeAlertType(rule.AlertType) == "log" {
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
		s.updateMonitorRuleEval(*rule, "failed", "全部匹配数据源评估失败")
		return
	}
	s.updateMonitorRuleEval(*rule, "success", fmt.Sprintf("命中 %d 条序列，%d 个数据源失败", matched, failed))
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
	} else {
		query = query.Where("type IN ?", []string{"prometheus", "victoriametrics"})
	}
	var list []model.MonitorDatasource
	if err := query.Order("is_default DESC, id DESC").Find(&list).Error; err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, errors.New("没有可用的匹配数据源")
	}
	return list, nil
}

func (s *Service) monitorLogAlertValue(rule model.MonitorAlertRule, ds model.MonitorDatasource) (float64, PromMetricSample, error) {
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
	// Keep the aggregation decision and its persisted notification marker in one
	// critical section so concurrent rule evaluations cannot both notify.
	s.monitorNotifyMu.Lock()
	if !s.shouldNotifyAlertEvent(*event, rule, aggregation) || !s.markMonitorAlertNotified(event) {
		s.monitorNotifyMu.Unlock()
		return false
	}
	s.monitorNotifyMu.Unlock()
	s.dispatchMonitorNotification(rule, *event, status)
	s.appendMonitorAlertTimeline(event.ID, "notification", "已提交消息通知", fmt.Sprintf("状态：%s，第 %d 次发送", status, event.NotifyCount), "系统", map[string]any{
		"notifyRuleId": rule.NotifyRuleID, "notifyCount": event.NotifyCount, "status": status,
	})
	return true
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
	summary := fmt.Sprintf("%s 当前值 %.4f %s %.4f", rule.Name, value, rule.Comparator, rule.Threshold)
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
				s.appendMonitorAlertTimeline(existing.ID, "firing", "告警正式触发", summary, "系统", nil)
			case "silenced":
				s.appendMonitorAlertTimeline(existing.ID, "silenced", "命中告警屏蔽", firstNonEmpty(existing.SilenceRuleName, silenceRule.Name), "系统", nil)
			}
		}
		if aggregated && aggregationRule != nil && previousAggregateRuleID != aggregationRule.ID {
			s.appendMonitorAlertTimeline(existing.ID, "aggregated", "命中聚合收敛规则", aggregationRule.Name, "系统", map[string]any{"aggregationKey": aggregationKey})
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
		title := "告警事件已创建"
		if event.Status == "pending" {
			title = "等待持续时间"
		} else if event.Status == "silenced" {
			title = "告警已被屏蔽"
		}
		s.appendMonitorAlertTimeline(event.ID, event.Status, title, summary, "系统", map[string]any{"fingerprint": event.Fingerprint})
		if aggregated && aggregationRule != nil {
			s.appendMonitorAlertTimeline(event.ID, "aggregated", "命中聚合收敛规则", aggregationRule.Name, "系统", map[string]any{"aggregationKey": aggregationKey})
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
		_ = s.db.Model(&model.MonitorAlertEvent{}).Where("id = ?", event.ID).Updates(map[string]any{
			"status": "recovered", "recovered_at": &now,
		}).Error
		s.appendMonitorAlertTimeline(event.ID, "recovered", "告警已自动恢复", "指标已不再满足告警条件", "系统", nil)
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
			AlertEventID: id, EventType: "firing", Title: "告警事件已创建", Detail: event.Summary, Operator: "系统", CreatedAt: event.FirstTriggerAt,
		})
		if event.ClaimedBy != "" {
			claimedAt := event.UpdatedAt
			if event.ClaimedAt != nil {
				claimedAt = *event.ClaimedAt
			}
			timelines = append(timelines, model.MonitorAlertEventTimeline{
				AlertEventID: id, EventType: "claimed", Title: "告警已认领", Detail: event.HandleNote, Operator: event.ClaimedBy, CreatedAt: claimedAt,
			})
		}
		if event.RecoveredAt != nil {
			title := "告警已自动恢复"
			if event.Status == "resolved" {
				title = "告警已人工关闭"
			}
			timelines = append(timelines, model.MonitorAlertEventTimeline{
				AlertEventID: id, EventType: event.Status, Title: title, Detail: event.ResolveNote, Operator: "系统", CreatedAt: *event.RecoveredAt,
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
		"allowed": true, "reason": "当前允许发送", "notifyCount": event.NotifyCount,
		"maxNotifyCount": rule.MaxNotifyCount, "lastNotifyAt": event.LastNotifyAt,
	}
	if event.Status == "recovered" || event.Status == "resolved" {
		notificationState["allowed"] = false
		notificationState["reason"] = "告警已结束，无需继续通知"
	} else if event.Silenced {
		notificationState["allowed"] = false
		notificationState["reason"] = "命中屏蔽规则：" + firstNonEmpty(event.SilenceRuleName, "未命名规则")
	} else if rule.MaxNotifyCount > 0 && event.NotifyCount >= rule.MaxNotifyCount {
		notificationState["allowed"] = false
		notificationState["reason"] = "已达到最大发送次数"
	} else if event.LastNotifyAt != nil {
		nextNotifyAt := event.LastNotifyAt.Add(time.Duration(normalizeNotifyRepeatInterval(rule.NotifyRepeatIntervalSeconds)) * time.Second)
		notificationState["nextNotifyAt"] = nextNotifyAt
		if time.Now().Before(nextNotifyAt) {
			notificationState["allowed"] = false
			notificationState["reason"] = "等待重复通知间隔"
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
		return errors.New("告警事件 ID 不能为空")
	}
	now := time.Now()
	if err := s.db.Model(&model.MonitorAlertEvent{}).Where("id = ?", payload.ID).Updates(map[string]any{
		"status": "claimed", "claimed_by": strings.TrimSpace(payload.ClaimedBy), "claimed_at": &now, "handle_note": strings.TrimSpace(payload.HandleNote),
	}).Error; err != nil {
		return err
	}
	s.appendMonitorAlertTimeline(payload.ID, "claimed", "告警已认领", payload.HandleNote, payload.ClaimedBy, nil)
	return nil
}

func (s *Service) ResolveMonitorAlertEvent(payload MonitorAlertEventActionPayload) error {
	if payload.ID == 0 {
		return errors.New("告警事件 ID 不能为空")
	}
	now := time.Now()
	if err := s.db.Model(&model.MonitorAlertEvent{}).Where("id = ?", payload.ID).Updates(map[string]any{
		"status": "resolved", "resolve_note": strings.TrimSpace(payload.HandleNote), "recovered_at": &now, "resolved_at": &now,
	}).Error; err != nil {
		return err
	}
	s.appendMonitorAlertTimeline(payload.ID, "resolved", "告警已人工关闭", payload.HandleNote, "操作人", nil)
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
		return errors.New("不支持的批量操作")
	}
	result := query.Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		if action == "claim" {
			return errors.New("所选事件没有可认领的等待持续或触发中告警")
		}
		return errors.New("所选事件没有可关闭的未结束告警")
	}
	for _, id := range ids {
		if action == "claim" {
			s.appendMonitorAlertTimeline(id, "claimed", "告警已批量认领", payload.HandleNote, payload.ClaimedBy, nil)
		} else if action == "resolve" {
			s.appendMonitorAlertTimeline(id, "resolved", "告警已批量关闭", payload.HandleNote, payload.ClaimedBy, nil)
		}
	}
	return nil
}

func (s *Service) GetMonitorOverview() (map[string]any, error) {
	var datasourceCount, ruleCount, firingCount, recoveredCount, todayRecoveredCount int64
	var unclaimedCount, criticalCount, unhealthyDatasourceCount, evalFailedRuleCount, notificationFailedCount, todayTriggeredCount int64
	_ = s.db.Model(&model.MonitorDatasource{}).Count(&datasourceCount).Error
	_ = s.db.Model(&model.MonitorAlertRule{}).Count(&ruleCount).Error
	_ = s.db.Model(&model.MonitorAlertEvent{}).Where("status IN ?", []string{"firing", "claimed"}).Count(&firingCount).Error
	_ = s.db.Model(&model.MonitorAlertEvent{}).Where("status = ?", "recovered").Count(&recoveredCount).Error
	_ = s.db.Model(&model.MonitorAlertEvent{}).Where("status IN ? AND (claimed_by = '' OR claimed_by IS NULL)", []string{"pending", "firing"}).Count(&unclaimedCount).Error
	_ = s.db.Model(&model.MonitorAlertEvent{}).Where("status IN ? AND severity IN ?", []string{"pending", "firing", "claimed"}, []string{"P0", "P1"}).Count(&criticalCount).Error
	_ = s.db.Model(&model.MonitorDatasource{}).Where("status = ? AND health_status = ?", 1, "unhealthy").Count(&unhealthyDatasourceCount).Error
	_ = s.db.Model(&model.MonitorAlertRule{}).Where("status = ? AND last_eval_status = ?", 1, "failed").Count(&evalFailedRuleCount).Error
	_ = s.db.Model(&model.NotifySendLog{}).Where("scope = ? AND status = ? AND created_at >= ?", "monitor", "failed", time.Now().Add(-24*time.Hour)).Count(&notificationFailedCount).Error
	now := time.Now()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	_ = s.db.Model(&model.MonitorAlertEvent{}).Where("created_at >= ?", dayStart).Count(&todayTriggeredCount).Error
	_ = s.db.Model(&model.MonitorAlertEvent{}).Where("status IN ? AND recovered_at >= ?", []string{"recovered", "resolved"}, dayStart).Count(&todayRecoveredCount).Error
	severityRows := []map[string]any{}
	for _, severity := range []string{"P0", "P1", "P2", "P3"} {
		var count int64
		_ = s.db.Model(&model.MonitorAlertEvent{}).Where("status IN ? AND severity = ?", []string{"firing", "claimed"}, severity).Count(&count).Error
		severityRows = append(severityRows, map[string]any{"severity": severity, "count": count})
	}
	var recent []model.MonitorAlertEvent
	_ = s.db.Where("status IN ?", []string{"pending", "firing", "claimed"}).Order("last_trigger_at DESC, id DESC").Limit(8).Find(&recent).Error
	var recentHandled []model.MonitorAlertEvent
	_ = s.db.Where("created_at >= ? AND (claimed_at IS NOT NULL OR recovered_at IS NOT NULL)", time.Now().Add(-30*24*time.Hour)).Find(&recentHandled).Error
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
	return map[string]any{
		"datasourceCount": datasourceCount, "ruleCount": ruleCount, "firingCount": firingCount,
		"recoveredCount": recoveredCount, "todayRecoveredCount": todayRecoveredCount, "severity": severityRows, "recentEvents": recent,
		"unclaimedCount": unclaimedCount, "criticalCount": criticalCount,
		"unhealthyDatasourceCount": unhealthyDatasourceCount, "evalFailedRuleCount": evalFailedRuleCount,
		"notificationFailedCount": notificationFailedCount, "todayTriggeredCount": todayTriggeredCount,
		"mttaSeconds": average(totalAckSeconds, ackSamples), "mttrSeconds": average(totalRecoverSeconds, recoverSamples),
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
	if normalizeMonitorDatasourceType(ds.Type) == "elasticsearch" {
		return errors.New("Elasticsearch 数据源暂不支持 PromQL 监控面板，请在即时查询中使用 DSL")
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
	if normalizeMonitorDatasourceType(ds.Type) == "elasticsearch" {
		var fallback model.MonitorDatasource
		if err := s.db.Where("status = ? AND type IN ?", 1, []string{"prometheus", "victoriametrics"}).Order("is_default DESC, id DESC").First(&fallback).Error; err != nil {
			return nil, errors.New("监控面板仅支持 Prometheus 或 VictoriaMetrics，请先配置指标数据源")
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
