package service

import (
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
)

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

func (s *Service) PrometheusInstantQuery(datasourceID uint, query string, ts time.Time) (map[string]any, error) {
	return s.MonitorInstantQuery(datasourceID, query, ts)
}

func (s *Service) MonitorRangeQuery(payload MonitorRangeQueryPayload) (map[string]any, error) {
	if strings.TrimSpace(payload.Query) == "" {
		return nil, errors.New("query is required")
	}
	ds, err := s.GetMonitorDatasource(payload.DatasourceID)
	if err != nil {
		return nil, err
	}
	if !isMonitorMetricDatasource(ds.Type) {
		return nil, errors.New("chart queries support only Prometheus or VictoriaMetrics datasources")
	}
	endAt := time.Unix(payload.EndAt, 0)
	if payload.EndAt <= 0 {
		endAt = time.Now()
	}
	startAt := time.Unix(payload.StartAt, 0)
	if payload.StartAt <= 0 || !startAt.Before(endAt) {
		startAt = endAt.Add(-time.Hour)
	}
	result, err := s.prometheusRangeQuery(*ds, payload.Query, startAt, endAt, payload.StepSeconds)
	if err != nil {
		return nil, err
	}
	return map[string]any{"resultType": result.Data.ResultType, "result": result.Data.Result, "startAt": startAt.Unix(), "endAt": endAt.Unix()}, nil
}

func (s *Service) MonitorInstantQuery(datasourceID uint, query string, ts time.Time) (map[string]any, error) {
	if strings.TrimSpace(query) == "" {
		return nil, errors.New("query is required")
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
	} else if normalizeMonitorDatasourceType(ds.Type) == "victorialogs" {
		return nil, errors.New("use LogsQL in Log Explorer for VictoriaLogs")
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
	s.trimMonitorQueryHistories(10)
	if err != nil {
		return nil, err
	}
	return response, nil
}

func (s *Service) trimMonitorQueryHistories(limit int) {
	if limit < 1 {
		limit = 10
	}
	var keepIDs []uint
	if err := s.db.Model(&model.MonitorQueryHistory{}).Order("id DESC").Limit(limit).Pluck("id", &keepIDs).Error; err != nil {
		return
	}
	if len(keepIDs) < limit {
		return
	}
	_ = s.db.Where("id NOT IN ?", keepIDs).Delete(&model.MonitorQueryHistory{}).Error
}

func (s *Service) ListMonitorQueryHistories(pageNum, pageSize int, keyword, status string) (map[string]any, error) {
	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	if pageSize > 10 {
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
		return nil, fmt.Errorf("Prometheus API returned status %d: %s", response.StatusCode, string(body))
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
		return nil, errors.New("query end time must be later than start time")
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
		return nil, fmt.Errorf("Prometheus API returned status %d: %s", response.StatusCode, string(body))
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
		return fmt.Errorf("Elasticsearch health check failed with status %d: %s", response.StatusCode, string(body))
	}
	return nil
}

// jaegerHealth verifies the Jaeger Query API is reachable. The services
// endpoint is available on both the all-in-one and query service deployments.
func (s *Service) jaegerHealth(ds model.MonitorDatasource) error {
	request, err := http.NewRequest(http.MethodGet, strings.TrimRight(ds.URL, "/")+"/api/services", nil)
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
		return fmt.Errorf("Jaeger health check failed with status %d: %s", response.StatusCode, string(body))
	}
	return nil
}

func (s *Service) ListMonitorJaegerServices(datasourceID uint) ([]string, error) {
	ds, err := s.GetMonitorDatasource(datasourceID)
	if err != nil {
		return nil, err
	}
	if !isMonitorTraceDatasource(ds.Type) {
		return nil, errors.New("current datasource is not Jaeger")
	}
	var data []string
	if err := s.jaegerGet(*ds, "/api/services", nil, &data); err != nil {
		return nil, err
	}
	sort.Strings(data)
	return data, nil
}

func (s *Service) ListMonitorJaegerOperations(datasourceID uint, service string) ([]string, error) {
	if strings.TrimSpace(service) == "" {
		return []string{}, nil
	}
	ds, err := s.GetMonitorDatasource(datasourceID)
	if err != nil {
		return nil, err
	}
	if !isMonitorTraceDatasource(ds.Type) {
		return nil, errors.New("current datasource is not Jaeger")
	}
	var data []string
	if err := s.jaegerGet(*ds, "/api/services/"+url.PathEscape(strings.TrimSpace(service))+"/operations", nil, &data); err != nil {
		return nil, err
	}
	sort.Strings(data)
	return data, nil
}

func (s *Service) QueryMonitorTraces(payload MonitorTraceQueryPayload) ([]map[string]any, error) {
	if payload.DatasourceID == 0 {
		return nil, errors.New("select a Jaeger datasource")
	}
	if strings.TrimSpace(payload.Service) == "" {
		return nil, errors.New("select a service")
	}
	ds, err := s.GetMonitorDatasource(payload.DatasourceID)
	if err != nil {
		return nil, err
	}
	if !isMonitorTraceDatasource(ds.Type) {
		return nil, errors.New("trace queries support only Jaeger datasources")
	}
	endAt := payload.EndAt
	if endAt <= 0 {
		endAt = time.Now().UnixMilli()
	}
	startAt := payload.StartAt
	if startAt <= 0 || startAt >= endAt {
		startAt = endAt - int64(time.Hour/time.Millisecond)
	}
	limit := payload.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 1000 {
		limit = 1000
	}
	params := url.Values{
		"service": {strings.TrimSpace(payload.Service)},
		"start":   {strconv.FormatInt(startAt*1000, 10)},
		"end":     {strconv.FormatInt(endAt*1000, 10)},
		"limit":   {strconv.Itoa(limit)},
	}
	if operation := strings.TrimSpace(payload.Operation); operation != "" {
		params.Set("operation", operation)
	}
	if tags := strings.TrimSpace(payload.Tags); tags != "" {
		var parsed map[string]any
		if err := json.Unmarshal([]byte(tags), &parsed); err != nil {
			return nil, errors.New("tag filter must be a JSON object")
		}
		params.Set("tags", tags)
	}
	var data []map[string]any
	if err := s.jaegerGet(*ds, "/api/traces", params, &data); err != nil {
		return nil, err
	}
	return data, nil
}

func (s *Service) GetMonitorTrace(datasourceID uint, traceID string) (map[string]any, error) {
	if datasourceID == 0 || strings.TrimSpace(traceID) == "" {
		return nil, errors.New("datasource and Trace ID are required")
	}
	if strings.ContainsAny(traceID, "/\\") {
		return nil, errors.New("invalid Trace ID format")
	}
	ds, err := s.GetMonitorDatasource(datasourceID)
	if err != nil {
		return nil, err
	}
	if !isMonitorTraceDatasource(ds.Type) {
		return nil, errors.New("trace queries support only Jaeger datasources")
	}
	var data []map[string]any
	if err := s.jaegerGet(*ds, "/api/traces/"+url.PathEscape(strings.TrimSpace(traceID)), nil, &data); err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, errors.New("trace was not found")
	}
	return data[0], nil
}

func (s *Service) jaegerGet(ds model.MonitorDatasource, path string, params url.Values, target any) error {
	endpoint := strings.TrimRight(ds.URL, "/") + path
	if len(params) > 0 {
		endpoint += "?" + params.Encode()
	}
	request, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	applyMonitorDatasourceAuth(request, ds)
	response, err := (&http.Client{Timeout: 30 * time.Second}).Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(response.Body, 8*1024*1024))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("Jaeger API returned status %d: %s", response.StatusCode, string(body))
	}
	var result struct {
		Data   json.RawMessage `json:"data"`
		Errors []any           `json:"errors"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return err
	}
	if len(result.Errors) > 0 {
		return fmt.Errorf("Jaeger query failed: %v", result.Errors[0])
	}
	if len(result.Data) == 0 {
		return errors.New("Jaeger API returned no data")
	}
	return json.Unmarshal(result.Data, target)
}
