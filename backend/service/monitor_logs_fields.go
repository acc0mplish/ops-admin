package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"ops-admin/backend/model"
)

func monitorLogFieldValue(source map[string]any, path string) string {
	if value, ok := source[path]; ok {
		return monitorSourceString(value)
	}
	current := any(source)
	for _, part := range strings.Split(path, ".") {
		object, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		current = object[part]
	}
	return monitorSourceString(current)
}

func (s *Service) ListMonitorVictoriaLogsStreams(datasourceID uint, field, query string, startAt, endAt int64, limit int) ([]map[string]any, error) {
	if datasourceID == 0 {
		return nil, errors.New("select a VictoriaLogs datasource")
	}
	ds, err := s.GetMonitorDatasource(datasourceID)
	if err != nil {
		return nil, err
	}
	if normalizeMonitorDatasourceType(ds.Type) != "victorialogs" {
		return nil, errors.New("current datasource is not VictoriaLogs")
	}
	field = firstNonEmpty(strings.TrimSpace(field), "kafka_topic")
	limit = normalizeMonitorLogFieldValueLimit(limit)
	end := time.UnixMilli(endAt)
	if endAt <= 0 {
		end = time.Now()
	}
	start := time.UnixMilli(startAt)
	if startAt <= 0 || startAt >= end.UnixMilli() {
		start = end.Add(-time.Hour)
	}
	logsQL := strings.TrimSpace(query)
	if logsQL == "" {
		logsQL = "*"
	}
	form := url.Values{}
	form.Set("query", logsQL)
	form.Set("field", field)
	form.Set("start", start.UTC().Format(time.RFC3339Nano))
	form.Set("end", end.UTC().Format(time.RFC3339Nano))
	// Do not pass limit to VictoriaLogs field_values. Some versions return field values but
	// report every hit count as zero when the parameter is present; truncate only after receiving real counts.
	request, err := http.NewRequest(http.MethodPost, strings.TrimRight(ds.URL, "/")+"/select/logsql/field_values", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	applyMonitorDatasourceAuth(request, *ds)
	response, err := (&http.Client{Timeout: 20 * time.Second}).Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(response.Body, 4*1024*1024))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("failed to retrieve VictoriaLogs streams; status %d: %s", response.StatusCode, string(body))
	}
	var raw struct {
		Values []struct {
			Value string `json:"value"`
			Hits  int64  `json:"hits"`
		} `json:"values"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	items := make([]map[string]any, 0, len(raw.Values))
	for _, item := range raw.Values {
		if strings.TrimSpace(item.Value) == "" {
			continue
		}
		items = append(items, map[string]any{"value": item.Value, "hits": item.Hits, "field": field})
	}
	sort.Slice(items, func(i, j int) bool {
		left, _ := items[i]["hits"].(int64)
		right, _ := items[j]["hits"].(int64)
		if left == right {
			return monitorSourceString(items[i]["value"]) < monitorSourceString(items[j]["value"])
		}
		return left > right
	})
	if len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func (s *Service) ListMonitorLogFields(datasourceID uint, index, query string, startAt, endAt int64) ([]map[string]any, error) {
	if datasourceID == 0 {
		return nil, errors.New("select a log datasource")
	}
	ds, err := s.GetMonitorDatasource(datasourceID)
	if err != nil {
		return nil, err
	}
	switch normalizeMonitorDatasourceType(ds.Type) {
	case "elasticsearch":
		return s.elasticsearchLogFields(*ds, index)
	case "victorialogs":
		return s.victoriaLogsFields(*ds, query, startAt, endAt)
	default:
		return nil, errors.New("log fields support only Elasticsearch or VictoriaLogs datasources")
	}
}

func (s *Service) ListMonitorLogFieldValues(datasourceID uint, index, field, query string, startAt, endAt int64, limit int) ([]map[string]any, error) {
	if datasourceID == 0 {
		return nil, errors.New("select a log datasource")
	}
	field = strings.TrimSpace(field)
	if !isCommonMonitorLogField(field) {
		ds, err := s.GetMonitorDatasource(datasourceID)
		if err != nil {
			return nil, err
		}
		if normalizeMonitorDatasourceType(ds.Type) == "victorialogs" && isSafeVictoriaLogsField(field) {
			return s.ListMonitorVictoriaLogsStreams(datasourceID, field, query, startAt, endAt, limit)
		}
	}
	if !isCommonMonitorLogField(field) {
		return nil, errors.New("unsupported log filter field")
	}
	ds, err := s.GetMonitorDatasource(datasourceID)
	if err != nil {
		return nil, err
	}
	switch normalizeMonitorDatasourceType(ds.Type) {
	case "elasticsearch":
		return s.elasticsearchLogFieldValues(*ds, index, field, query, startAt, endAt, limit)
	case "victorialogs":
		return s.ListMonitorVictoriaLogsStreams(datasourceID, field, query, startAt, endAt, limit)
	default:
		return nil, errors.New("log fields support only Elasticsearch or VictoriaLogs datasources")
	}
}

func normalizeMonitorLogFieldValueLimit(limit int) int {
	if limit <= 0 {
		return 100
	}
	if limit > 200 {
		return 200
	}
	return limit
}

func isCommonMonitorLogField(field string) bool {
	for _, item := range []string{
		"kubernetes.pod_namespace", "kubernetes.pod_name", "kubernetes.container_name", "kafka_topic", "level",
	} {
		if field == item {
			return true
		}
	}
	return false
}

func isSafeVictoriaLogsField(field string) bool {
	if field == "" || len(field) > 128 {
		return false
	}
	for _, char := range field {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '_' || char == '.' || char == '-' {
			continue
		}
		return false
	}
	return true
}

func (s *Service) elasticsearchLogFieldValues(ds model.MonitorDatasource, index, field, query string, startAt, endAt int64, limit int) ([]map[string]any, error) {
	index = firstNonEmpty(strings.TrimSpace(index), "_all")
	if strings.Contains(index, "/") || strings.Contains(index, "\\") {
		return nil, errors.New("index must not contain path separators")
	}
	endAt = firstNonEmptyInt64(endAt, time.Now().UnixMilli())
	if startAt <= 0 || startAt >= endAt {
		startAt = time.UnixMilli(endAt).Add(-time.Hour).UnixMilli()
	}
	must := []any{map[string]any{"match_all": map[string]any{}}}
	if strings.TrimSpace(query) != "" {
		must = []any{map[string]any{"query_string": map[string]any{"query": strings.TrimSpace(query), "analyze_wildcard": true}}}
	}
	search := func(aggregationField string) ([]map[string]any, error) {
		body := map[string]any{
			"size": 0,
			"query": map[string]any{
				"bool": map[string]any{
					"must": must,
					"filter": []any{map[string]any{
						"range": map[string]any{
							"@timestamp": map[string]any{"gte": startAt, "lte": endAt, "format": "epoch_millis"},
						},
					}},
				},
			},
			"aggs": map[string]any{
				"values": map[string]any{
					"terms": map[string]any{"field": aggregationField, "size": 100, "order": map[string]any{"_count": "desc"}},
				},
			},
		}
		response, err := s.elasticsearchSearch(ds, index, body)
		if err != nil {
			return nil, err
		}
		aggs, _ := response["aggregations"].(map[string]any)
		values, _ := aggs["values"].(map[string]any)
		buckets, _ := values["buckets"].([]any)
		items := make([]map[string]any, 0, len(buckets))
		for _, raw := range buckets {
			bucket, _ := raw.(map[string]any)
			value := monitorSourceString(bucket["key"])
			if value != "" {
				items = append(items, map[string]any{"value": value, "hits": bucket["doc_count"], "field": field})
			}
		}
		return items, nil
	}
	var lastErr error
	for _, aggregationField := range []string{field, field + ".keyword"} {
		items, err := search(aggregationField)
		if err != nil {
			lastErr = err
			continue
		}
		if len(items) > 0 {
			return items, nil
		}
	}

	// Some log indices keep these fields in _source without a terms-aggregatable
	// mapping. Fall back to the current matching documents so the selector stays
	// consistent with the log rows displayed in the workbench.
	body := map[string]any{
		"size":    1000,
		"_source": []string{field},
		"query": map[string]any{
			"bool": map[string]any{
				"must": must,
				"filter": []any{map[string]any{
					"range": map[string]any{
						"@timestamp": map[string]any{"gte": startAt, "lte": endAt, "format": "epoch_millis"},
					},
				}},
			},
		},
	}
	response, err := s.elasticsearchSearch(ds, index, body)
	if err != nil {
		if lastErr != nil {
			return nil, lastErr
		}
		return nil, err
	}
	hits, _ := response["hits"].(map[string]any)
	rawItems, _ := hits["hits"].([]any)
	counts := make(map[string]int64)
	for _, rawItem := range rawItems {
		hit, _ := rawItem.(map[string]any)
		source, _ := hit["_source"].(map[string]any)
		if value := monitorLogFieldValue(source, field); value != "" {
			counts[value]++
		}
	}
	items := make([]map[string]any, 0, len(counts))
	for value, hits := range counts {
		items = append(items, map[string]any{"value": value, "hits": hits, "field": field})
	}
	sort.Slice(items, func(i, j int) bool {
		left, _ := items[i]["hits"].(int64)
		right, _ := items[j]["hits"].(int64)
		if left == right {
			return monitorSourceString(items[i]["value"]) < monitorSourceString(items[j]["value"])
		}
		return left > right
	})
	limit = normalizeMonitorLogFieldValueLimit(limit)
	if len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func firstNonEmptyInt64(value, fallback int64) int64 {
	if value > 0 {
		return value
	}
	return fallback
}

func (s *Service) elasticsearchLogFields(ds model.MonitorDatasource, index string) ([]map[string]any, error) {
	index = firstNonEmpty(strings.TrimSpace(index), "_all")
	endpoint := strings.TrimRight(ds.URL, "/") + "/_mapping"
	if index != "_all" && index != "*" {
		endpoint = strings.TrimRight(ds.URL, "/") + "/" + url.PathEscape(index) + "/_mapping"
	}
	request, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	applyMonitorDatasourceAuth(request, ds)
	response, err := (&http.Client{Timeout: 20 * time.Second}).Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(response.Body, 8*1024*1024))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("failed to retrieve Elasticsearch fields; status %d: %s", response.StatusCode, string(body))
	}
	var mappings map[string]struct {
		Mappings struct {
			Properties map[string]any `json:"properties"`
		} `json:"mappings"`
	}
	if err := json.Unmarshal(body, &mappings); err != nil {
		return nil, err
	}
	fieldTypes := map[string]string{}
	for _, mapping := range mappings {
		collectElasticsearchFields("", mapping.Mappings.Properties, fieldTypes)
	}
	return monitorLogFields(fieldTypes), nil
}

func collectElasticsearchFields(prefix string, properties map[string]any, fieldTypes map[string]string) {
	for name, raw := range properties {
		path := name
		if prefix != "" {
			path = prefix + "." + name
		}
		config, _ := raw.(map[string]any)
		fieldType := monitorSourceString(config["type"])
		if fieldType == "" {
			fieldType = "object"
		}
		fieldTypes[path] = fieldType
		if children, ok := config["properties"].(map[string]any); ok {
			collectElasticsearchFields(path, children, fieldTypes)
		}
	}
}

func (s *Service) victoriaLogsFields(ds model.MonitorDatasource, query string, startAt, endAt int64) ([]map[string]any, error) {
	end := time.UnixMilli(endAt)
	if endAt <= 0 {
		end = time.Now()
	}
	start := time.UnixMilli(startAt)
	if startAt <= 0 || startAt >= end.UnixMilli() {
		start = end.Add(-time.Hour)
	}
	form := url.Values{}
	form.Set("query", firstNonEmpty(strings.TrimSpace(query), "*"))
	form.Set("start", start.UTC().Format(time.RFC3339Nano))
	form.Set("end", end.UTC().Format(time.RFC3339Nano))
	request, err := http.NewRequest(http.MethodPost, strings.TrimRight(ds.URL, "/")+"/select/logsql/field_names", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	applyMonitorDatasourceAuth(request, ds)
	response, err := (&http.Client{Timeout: 20 * time.Second}).Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(response.Body, 4*1024*1024))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("failed to retrieve VictoriaLogs fields; status %d: %s", response.StatusCode, string(body))
	}
	var raw struct {
		Fields []string `json:"fields"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	fieldTypes := map[string]string{}
	for _, field := range raw.Fields {
		field = strings.TrimSpace(field)
		if field != "" {
			fieldTypes[field] = "field"
		}
	}
	return monitorLogFields(fieldTypes), nil
}

func monitorLogFields(fieldTypes map[string]string) []map[string]any {
	items := make([]map[string]any, 0, len(fieldTypes))
	for name, fieldType := range fieldTypes {
		items = append(items, map[string]any{"name": name, "type": fieldType})
	}
	sort.Slice(items, func(i, j int) bool {
		return monitorSourceString(items[i]["name"]) < monitorSourceString(items[j]["name"])
	})
	if len(items) > 500 {
		items = items[:500]
	}
	return items
}

func (s *Service) ListMonitorElasticsearchIndices(datasourceID uint) ([]map[string]any, error) {
	if datasourceID == 0 {
		return nil, errors.New("select an Elasticsearch datasource")
	}
	ds, err := s.GetMonitorDatasource(datasourceID)
	if err != nil {
		return nil, err
	}
	if normalizeMonitorDatasourceType(ds.Type) != "elasticsearch" {
		return nil, errors.New("current datasource is not Elasticsearch")
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
		return nil, fmt.Errorf("failed to retrieve Elasticsearch indices; status %d: %s", response.StatusCode, string(body))
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
