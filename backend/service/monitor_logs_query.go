package service

import (
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
	"time"

	"ops-admin/backend/model"
)

var springLogPattern = regexp.MustCompile(`(?s)^\s*(\S+\s+\S+)\s+(TRACE|DEBUG|INFO|WARN|ERROR|FATAL)\s+(\S+)\s+---\s+\[([^\]]*)\]\s+(.+?)\s*:\s*(.*)$`)

func (s *Service) elasticsearchQuery(ds model.MonitorDatasource, query string) (map[string]any, error) {
	payload := map[string]any{}
	if err := json.Unmarshal([]byte(query), &payload); err != nil {
		return nil, errors.New("Elasticsearch DSL must be a valid JSON object")
	}
	index := strings.TrimSpace(fmt.Sprint(payload["index"]))
	delete(payload, "index")
	if index == "" || index == "<nil>" {
		index = "_all"
	}
	if strings.Contains(index, "/") || strings.Contains(index, "\\") {
		return nil, errors.New("Elasticsearch index must not contain path separators")
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
		return nil, fmt.Errorf("Elasticsearch query failed with status %d: %s", response.StatusCode, string(responseBody))
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
		return nil, errors.New("select a log datasource")
	}
	ds, err := s.GetMonitorDatasource(payload.DatasourceID)
	if err != nil {
		return nil, err
	}
	switch normalizeMonitorDatasourceType(ds.Type) {
	case "elasticsearch":
		return s.queryElasticsearchMonitorLogs(payload)
	case "victorialogs":
		return s.queryVictoriaLogs(*ds, payload)
	default:
		return nil, errors.New("log queries support only Elasticsearch or VictoriaLogs datasources")
	}
}

func (s *Service) queryElasticsearchMonitorLogs(payload MonitorLogQueryPayload) (map[string]any, error) {
	if payload.DatasourceID == 0 {
		return nil, errors.New("select an Elasticsearch datasource")
	}
	ds, err := s.GetMonitorDatasource(payload.DatasourceID)
	if err != nil {
		return nil, err
	}
	if normalizeMonitorDatasourceType(ds.Type) != "elasticsearch" {
		return nil, errors.New("log query supports only Elasticsearch datasources")
	}
	index := strings.TrimSpace(payload.Index)
	if index == "" {
		index = "_all"
	}
	if strings.Contains(index, "/") || strings.Contains(index, "\\") {
		return nil, errors.New("index must not contain path separators")
	}
	pageNum, pageSize := normalizeMonitorLogPagination(payload.PageNum, payload.PageSize)
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
		"from": (pageNum - 1) * pageSize,
		"size": pageSize,
		"sort": []any{map[string]any{"@timestamp": map[string]any{"order": "desc", "unmapped_type": "date"}}},
		"query": map[string]any{"bool": map[string]any{
			"must":   must,
			"filter": []any{map[string]any{"range": map[string]any{"@timestamp": map[string]any{"gte": startAt, "lte": endAt, "format": "epoch_millis"}}}},
		}},
		"aggs": map[string]any{"histogram": map[string]any{"date_histogram": map[string]any{
			"field": "@timestamp", "fixed_interval": monitorLogHistogramInterval(startAt, endAt), "min_doc_count": 0,
		}}},
	}
	if payload.TrackTotalHits {
		body["track_total_hits"] = true
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
		displayMessage := messageFields["content"]
		if opLogMessage := formatMonitorOpLogContent(source, monitorSourceString(hit["_index"]), rawMessage); opLogMessage != "" {
			displayMessage = opLogMessage
		}
		items = append(items, map[string]any{
			"index":      hit["_index"],
			"id":         hit["_id"],
			"timestamp":  firstNonEmpty(monitorSourceString(source["@timestamp"]), monitorSourceString(source["timestamp"])),
			"namespace":  firstNonEmpty(monitorSourceString(kubernetes["pod_namespace"]), monitorSourceString(source["namespace"])),
			"pod":        firstNonEmpty(monitorSourceString(kubernetes["pod_name"]), monitorSourceString(source["pod"])),
			"container":  firstNonEmpty(monitorSourceString(kubernetes["container_name"]), monitorSourceString(source["container"])),
			"level":      level,
			"message":    displayMessage,
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
		"startAt": startAt, "endAt": endAt, "pageNum": pageNum, "pageSize": pageSize,
	}, nil
}

func (s *Service) victoriaLogsHealth(ds model.MonitorDatasource) error {
	request, err := http.NewRequest(http.MethodGet, strings.TrimRight(ds.URL, "/")+"/metrics", nil)
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
		return fmt.Errorf("VictoriaLogs health check failed with status %d: %s", response.StatusCode, string(body))
	}
	return nil
}

func (s *Service) queryVictoriaLogs(ds model.MonitorDatasource, payload MonitorLogQueryPayload) (map[string]any, error) {
	pageNum, pageSize := normalizeMonitorLogPagination(payload.PageNum, payload.PageSize)
	endAt := payload.EndAt
	if endAt <= 0 {
		endAt = time.Now().UnixMilli()
	}
	startAt := payload.StartAt
	if startAt <= 0 || startAt >= endAt {
		startAt = time.UnixMilli(endAt).Add(-24 * time.Hour).UnixMilli()
	}
	query := strings.TrimSpace(payload.Query)
	if query == "" {
		query = "*"
	}
	startedAt := time.Now()
	items, err := s.queryVictoriaLogsRows(ds, query, startAt, endAt, pageNum, pageSize)
	if err != nil {
		return nil, err
	}
	histogram, total := s.victoriaLogsHistogram(ds, query, startAt, endAt)
	// VictoriaLogs /query and /hits can occasionally disagree at the start-time boundary:
	// /hits may count a record while /query returns no rows. Only in this exceptional branch,
	// retry with the start time shifted back by one second to avoid a false empty result.
	if len(items) == 0 && total > 0 && pageNum == 1 && startAt > 1000 {
		if retryItems, retryErr := s.queryVictoriaLogsRows(ds, query, startAt-1000, endAt, pageNum, pageSize); retryErr == nil {
			items = retryItems
		}
	}
	if total == 0 && len(items) > 0 {
		total = int64(len(items))
	}
	return map[string]any{
		"items": items, "total": total, "took": time.Since(startedAt).Milliseconds(), "histogram": histogram,
		"startAt": startAt, "endAt": endAt, "pageNum": pageNum, "pageSize": pageSize,
	}, nil
}

func (s *Service) queryVictoriaLogsRows(ds model.MonitorDatasource, query string, startAt, endAt int64, pageNum, pageSize int) ([]map[string]any, error) {
	form := url.Values{}
	form.Set("query", query)
	form.Set("start", time.UnixMilli(startAt).UTC().Format(time.RFC3339Nano))
	form.Set("end", time.UnixMilli(endAt).UTC().Format(time.RFC3339Nano))
	form.Set("limit", strconv.Itoa(pageSize))
	form.Set("offset", strconv.Itoa((pageNum-1)*pageSize))
	request, err := http.NewRequest(http.MethodPost, strings.TrimRight(ds.URL, "/")+"/select/logsql/query", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	applyMonitorDatasourceAuth(request, ds)
	response, err := (&http.Client{Timeout: 30 * time.Second}).Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(response.Body, 32*1024*1024))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("VictoriaLogs query failed with status %d: %s", response.StatusCode, string(body))
	}
	items := make([]map[string]any, 0)
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var source map[string]any
		if err := json.Unmarshal([]byte(line), &source); err != nil {
			return nil, fmt.Errorf("failed to parse VictoriaLogs records: %w", err)
		}
		items = append(items, formatMonitorLogItem(source, "", ""))
	}
	return items, nil
}

func normalizeMonitorLogPagination(pageNum, pageSize int) (int, int) {
	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize < 1 {
		pageSize = 200
	}
	if pageSize > 1000 {
		pageSize = 1000
	}
	return pageNum, pageSize
}

func (s *Service) victoriaLogsHistogram(ds model.MonitorDatasource, query string, startAt, endAt int64) ([]map[string]any, int64) {
	form := url.Values{}
	form.Set("query", query)
	form.Set("start", time.UnixMilli(startAt).UTC().Format(time.RFC3339Nano))
	form.Set("end", time.UnixMilli(endAt).UTC().Format(time.RFC3339Nano))
	step := monitorLogHistogramStepSeconds(startAt, endAt)
	form.Set("step", strconv.FormatInt(step, 10)+"s")
	request, err := http.NewRequest(http.MethodPost, strings.TrimRight(ds.URL, "/")+"/select/logsql/hits", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, 0
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	applyMonitorDatasourceAuth(request, ds)
	response, err := (&http.Client{Timeout: 15 * time.Second}).Do(request)
	if err != nil {
		return nil, 0
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(response.Body, 2*1024*1024))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, 0
	}
	var result struct {
		Hits []struct {
			Timestamps []string `json:"timestamps"`
			Values     []int64  `json:"values"`
			Total      int64    `json:"total"`
		} `json:"hits"`
	}
	if json.Unmarshal(body, &result) != nil {
		return nil, 0
	}
	bucketCounts := map[string]int64{}
	var total int64
	for _, group := range result.Hits {
		total += group.Total
		for i, timestamp := range group.Timestamps {
			if i < len(group.Values) {
				bucketCounts[timestamp] += group.Values[i]
			}
		}
	}
	timestamps := make([]string, 0, len(bucketCounts))
	for timestamp := range bucketCounts {
		timestamps = append(timestamps, timestamp)
	}
	sort.Strings(timestamps)
	buckets := make([]map[string]any, 0, len(timestamps))
	for _, timestamp := range timestamps {
		parsed, err := time.Parse(time.RFC3339Nano, timestamp)
		key := int64(0)
		if err == nil {
			key = parsed.UnixMilli()
		}
		buckets = append(buckets, map[string]any{
			"key": key, "key_as_string": timestamp, "doc_count": bucketCounts[timestamp],
		})
	}
	return buckets, total
}

// Keep the log histogram readable regardless of the selected range. Around
// 20–40 buckets make spikes visible without turning the chart into a solid bar.
func monitorLogHistogramStepSeconds(startAt, endAt int64) int64 {
	span := endAt - startAt
	switch {
	case span <= time.Hour.Milliseconds():
		return 60
	case span <= 6*time.Hour.Milliseconds():
		return 5 * 60
	case span <= 24*time.Hour.Milliseconds():
		return 15 * 60
	case span <= 3*24*time.Hour.Milliseconds():
		return 60 * 60
	case span <= 7*24*time.Hour.Milliseconds():
		return 3 * 60 * 60
	default:
		return 6 * 60 * 60
	}
}

func monitorLogHistogramInterval(startAt, endAt int64) string {
	return strconv.FormatInt(monitorLogHistogramStepSeconds(startAt, endAt)/60, 10) + "m"
}

func formatMonitorLogItem(source map[string]any, index, id string) map[string]any {
	rawMessage := cleanMonitorLogMessage(firstNonEmpty(
		monitorLogFieldValue(source, "_msg"), monitorLogFieldValue(source, "message"), monitorLogFieldValue(source, "log"),
	))
	messageFields := parseMonitorLogMessage(rawMessage)
	level := firstNonEmpty(monitorLogFieldValue(source, "level"), messageFields["level"], detectMonitorLogLevel(rawMessage))
	displayMessage := messageFields["content"]
	if opLogMessage := formatMonitorOpLogContent(source, index, rawMessage); opLogMessage != "" {
		displayMessage = opLogMessage
	}
	return map[string]any{
		"index": index, "id": id,
		"timestamp": firstNonEmpty(monitorLogFieldValue(source, "_time"), monitorLogFieldValue(source, "@timestamp"), monitorLogFieldValue(source, "timestamp")),
		"namespace": firstNonEmpty(monitorLogFieldValue(source, "kubernetes.pod_namespace"), monitorLogFieldValue(source, "namespace")),
		"pod":       firstNonEmpty(monitorLogFieldValue(source, "kubernetes.pod_name"), monitorLogFieldValue(source, "pod")),
		"container": firstNonEmpty(monitorLogFieldValue(source, "kubernetes.container_name"), monitorLogFieldValue(source, "container")),
		"level":     level, "message": displayMessage, "messageRaw": rawMessage,
		"logTime": messageFields["timestamp"], "processId": messageFields["processId"], "thread": messageFields["thread"], "logger": messageFields["logger"], "source": source,
	}
}

func cleanMonitorLogMessage(value string) string {
	value = strings.TrimSpace(value)
	ansi := regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)
	return ansi.ReplaceAllString(value, "")
}

var monitorOpLogHiddenContentFields = map[string]struct{}{
	"@timestamp":  {},
	"source_type": {},
	"timestamp":   {},
	"ts":          {},
}

func formatMonitorOpLogContent(source map[string]any, index, rawMessage string) string {
	var messageDocument map[string]any
	if strings.TrimSpace(rawMessage) != "" {
		_ = json.Unmarshal([]byte(rawMessage), &messageDocument)
	}

	if !isMonitorOpLogDocument(source, index) && !isMonitorOpLogDocument(messageDocument, index) {
		return ""
	}
	document := messageDocument
	if len(document) == 0 {
		document = source
	}
	if len(document) == 0 {
		return ""
	}

	content := make(map[string]any, len(document))
	for name, value := range document {
		if _, hidden := monitorOpLogHiddenContentFields[name]; hidden {
			continue
		}
		content[name] = value
	}
	encoded, err := json.Marshal(content)
	if err != nil {
		return ""
	}
	return string(encoded)
}

func isMonitorOpLogDocument(source map[string]any, index string) bool {
	indexName := strings.ToLower(strings.TrimSpace(index))
	if strings.Contains(indexName, "oplog") || strings.Contains(indexName, "op-log") {
		return true
	}
	if len(source) == 0 {
		return false
	}
	topic := strings.ToLower(monitorLogFieldValue(source, "kafka_topic"))
	logType := strings.ToLower(monitorLogFieldValue(source, "log_type"))
	return strings.Contains(topic, "op.log") || logType == "op"
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
