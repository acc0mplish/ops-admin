package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"ops-admin/backend/model"
)

func parseAIQueryTime(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"} {
		if parsed, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func (s *Service) queryAIHostHealth(args map[string]any) (map[string]any, error) {
	hostID := anyUint(args["hostId"])
	var host model.AssetHost
	if hostID > 0 {
		if err := s.db.First(&host, hostID).Error; err != nil {
			return nil, err
		}
	} else {
		keyword := strings.TrimSpace(anyString(args["keyword"]))
		if keyword == "" {
			return nil, errors.New("provide hostId or a host keyword")
		}
		like := "%" + keyword + "%"
		if err := s.db.Where("host_name LIKE ? OR alias LIKE ? OR private_ip LIKE ? OR public_ip LIKE ? OR ssh_ip LIKE ?", like, like, like, like, like).Order("id DESC").First(&host).Error; err != nil {
			return nil, err
		}
	}
	metrics, err := s.GetAssetHostMetrics(host.ID, firstNonEmpty(strings.TrimSpace(anyString(args["range"])), "24h"), "", "")
	if err != nil {
		return nil, err
	}
	alerts, err := s.queryAIMonitorAlertEvents(map[string]any{"keyword": firstNonEmpty(host.PrivateIP, host.PublicIP, host.HostName), "limit": 10})
	if err != nil {
		return nil, err
	}
	return map[string]any{"host": map[string]any{"id": host.ID, "name": host.HostName, "alias": host.Alias, "privateIp": host.PrivateIP, "publicIp": host.PublicIP, "environment": host.Environment, "online": host.AliveStatus == 1, "enabled": host.Status == 1, "lastCheckTime": host.LastCheckTime}, "metrics": metrics, "relatedAlerts": alerts}, nil
}

func (s *Service) queryAIOpsTroubleshooting(args map[string]any) (map[string]any, error) {
	result := map[string]any{"scope": "read_only", "evidence": map[string]any{}}
	evidence := result["evidence"].(map[string]any)
	if eventID := anyUint(args["alertEventId"]); eventID > 0 {
		detail, err := s.GetMonitorAlertEventDetail(eventID)
		if err != nil {
			return nil, err
		}
		evidence["alertEvent"] = detail
	}
	keyword := strings.TrimSpace(anyString(args["keyword"]))
	if keyword != "" {
		alerts, err := s.queryAIMonitorAlertEvents(map[string]any{"keyword": keyword, "limit": 10})
		if err != nil {
			return nil, err
		}
		evidence["matchingAlerts"] = alerts
	}
	if host := strings.TrimSpace(anyString(args["host"])); host != "" {
		health, err := s.queryAIHostHealth(map[string]any{"keyword": host, "range": anyString(args["range"])})
		if err != nil {
			return nil, err
		}
		evidence["hostHealth"] = health
	}
	datasources, err := s.queryAIMonitorDatasources(map[string]any{"limit": 20})
	if err != nil {
		return nil, err
	}
	evidence["datasources"] = datasources
	return result, nil
}

func (s *Service) queryAIMonitorDashboard(args map[string]any) (any, error) {
	if dashboardID := anyUint(args["dashboardId"]); dashboardID > 0 {
		return s.GetMonitorDashboard(dashboardID)
	}
	return s.ListMonitorDashboards(1, 20, anyString(args["keyword"]), "1")
}

func (s *Service) queryAIKnowledgeBase(args map[string]any) (map[string]any, error) {
	keyword := strings.TrimSpace(anyString(args["keyword"]))
	if keyword == "" {
		return nil, errors.New("knowledge-base search keyword is required")
	}
	limit := int(anyUint(args["limit"]))
	if limit == 0 {
		limit = 5
	}
	if limit < 1 || limit > 10 {
		return nil, errors.New("limit must be between 1 and 10")
	}
	query := s.db.Where("status = ?", 1)
	if documentID := anyUint(args["documentId"]); documentID > 0 {
		query = query.Where("id = ?", documentID)
	}
	like := "%" + keyword + "%"
	var documents []model.IntegrationAIKnowledgeDocument
	if err := query.Where("name LIKE ? OR content LIKE ?", like, like).Order("updated_at DESC, id DESC").Limit(limit).Find(&documents).Error; err != nil {
		return nil, err
	}
	items := make([]map[string]any, 0, len(documents))
	for _, item := range documents {
		items = append(items, map[string]any{
			"documentId": item.ID, "name": item.Name, "fileName": item.FileName,
			"excerpt": knowledgeExcerpt(item.Content, keyword, 560), "updatedAt": item.UpdatedAt,
		})
	}
	return map[string]any{"source": "local_knowledge_base", "keyword": keyword, "total": len(items), "items": items}, nil
}

func knowledgeExcerpt(content, keyword string, maxRunes int) string {
	runes := []rune(content)
	needle := []rune(strings.ToLower(keyword))
	start := 0
	if len(needle) > 0 {
		lower := []rune(strings.ToLower(content))
		for i := 0; i+len(needle) <= len(lower); i++ {
			matched := true
			for j := range needle {
				if lower[i+j] != needle[j] {
					matched = false
					break
				}
			}
			if matched {
				start = i - 120
				if start < 0 {
					start = 0
				}
				break
			}
		}
	}
	end := start + maxRunes
	if end > len(runes) {
		end = len(runes)
	}
	result := strings.TrimSpace(string(runes[start:end]))
	if start > 0 {
		result = "…" + result
	}
	if end < len(runes) {
		result += "…"
	}
	return result
}

// queryAIFinOpsAnalysis only reads the local FinOps tables populated by the
// account/synchronization workflows. It must never invoke a cloud billing API.
func (s *Service) queryAIFinOpsAnalysis(args map[string]any) (map[string]any, error) {
	analysisMonth := strings.TrimSpace(anyString(args["month"]))
	monthSource := "specified"
	if analysisMonth == "" {
		latestMonth, err := s.LatestFinOpsBreakdownMonth(anyUint(args["accountId"]))
		if err != nil {
			return nil, err
		}
		if latestMonth != "" {
			analysisMonth = latestMonth
			monthSource = "latest_synced"
		} else {
			analysisMonth = time.Now().Format("2006-01")
			monthSource = "current_month_no_local_bill"
		}
	}
	monthStart, err := parseFinOpsMonth(analysisMonth)
	if err != nil {
		return nil, errors.New("invalid month parameter; expected YYYY-MM")
	}
	accountID := anyUint(args["accountId"])
	trendMonths := int(anyUint(args["trendMonths"]))
	if trendMonths == 0 {
		trendMonths = 6
	}
	if trendMonths < 1 || trendMonths > 12 {
		return nil, errors.New("trendMonths must be between 1 and 12")
	}

	monthEnd := monthStart.AddDate(0, 1, 0)
	now := time.Now()
	if now.After(monthStart) && now.Before(monthEnd) {
		monthEnd = now
	}
	trendStart := monthStart.AddDate(0, -trendMonths+1, 0)
	dashboard, err := s.FinOpsDashboard(trendStart, monthEnd, accountID)
	if err != nil {
		return nil, err
	}
	selectedMonthSummary, err := s.FinOpsDashboard(monthStart, monthEnd, accountID)
	if err != nil {
		return nil, err
	}
	services, err := s.FinOpsBreakdown(monthStart, monthEnd, "service", accountID)
	if err != nil {
		return nil, err
	}
	regions, err := s.FinOpsBreakdown(monthStart, monthEnd, "region", accountID)
	if err != nil {
		return nil, err
	}
	serviceKeyword := strings.TrimSpace(anyString(args["service"]))
	includeResources := serviceKeyword != "" || anyBool(args["includeResourceBreakdown"])
	resourceBreakdown := map[string]any{"requested": includeResources}
	if includeResources {
		resourceBreakdown, err = s.queryAIFinOpsResourceBreakdown(monthStart, monthEnd, accountID, serviceKeyword)
		if err != nil {
			return nil, err
		}
	}
	return map[string]any{
		"source":               "local_synced_finops_database",
		"sourceDescription":    "로컬에 동기화된 Cloud Billing만 사용했으며 Cloud Provider API는 호출하지 않았습니다.",
		"analysisMonth":        analysisMonth,
		"monthSource":          monthSource,
		"accountId":            accountID,
		"trendMonths":          trendMonths,
		"trendDashboard":       dashboard,
		"selectedMonthSummary": selectedMonthSummary,
		"serviceBreakdown":     limitAIFinOpsRows(services, 12),
		"regionBreakdown":      limitAIFinOpsRows(regions, 12),
		"serviceFilter":        serviceKeyword,
		"resourceBreakdown":    resourceBreakdown,
	}, nil
}

func limitAIFinOpsRows(rows []map[string]any, limit int) []map[string]any {
	if len(rows) <= limit {
		return rows
	}
	return rows[:limit]
}

// queryAIFinOpsResourceBreakdown aggregates only persisted cost records. A row
// without resource ID/name is retained as unattributed cost instead of being
// invented as an instance.
func (s *Service) queryAIFinOpsResourceBreakdown(start, end time.Time, accountID uint, serviceKeyword string) (map[string]any, error) {
	records, _, err := s.finOpsRecords(start, end, accountID)
	if err != nil {
		return nil, err
	}
	keyword := strings.ToLower(strings.TrimSpace(serviceKeyword))
	rowsByResource := map[string]map[string]any{}
	unattributedCost := 0.0
	matchedRecordCount := 0
	for _, record := range records {
		if keyword != "" && !strings.Contains(strings.ToLower(record.Service), keyword) {
			continue
		}
		matchedRecordCount++
		resourceID, resourceName := strings.TrimSpace(record.ResourceID), strings.TrimSpace(record.ResourceName)
		if resourceID == "" && resourceName == "" {
			unattributedCost += record.Amount
			continue
		}
		key := resourceID
		if key == "" {
			key = "name:" + resourceName
		}
		row := rowsByResource[key]
		if row == nil {
			row = map[string]any{"resourceId": resourceID, "resourceName": resourceName, "service": record.Service, "region": record.Region, "amount": 0.0, "recordCount": 0}
			rowsByResource[key] = row
		}
		row["amount"] = row["amount"].(float64) + record.Amount
		row["recordCount"] = row["recordCount"].(int) + 1
	}
	items := make([]map[string]any, 0, len(rowsByResource))
	for _, row := range rowsByResource {
		items = append(items, row)
	}
	sort.Slice(items, func(i, j int) bool { return items[i]["amount"].(float64) > items[j]["amount"].(float64) })
	return map[string]any{
		"requested":          true,
		"serviceFilter":      serviceKeyword,
		"matchedRecordCount": matchedRecordCount,
		"resourceCount":      len(items),
		"unattributedCost":   unattributedCost,
		"items":              limitAIFinOpsRows(items, 20),
		"sourceDescription":  "Local Billing의 resourceId/resourceName 기준으로 집계했으며 Cloud Provider API는 조회하지 않습니다.",
	}, nil
}

func (s *Service) queryAIRealtimeLogs(args map[string]any, now time.Time) (map[string]any, error) {
	startAt, err := parseAILogTime(anyString(args["startTime"]), now)
	if err != nil {
		return nil, fmt.Errorf("invalid start time: %w", err)
	}
	endAt, err := parseAILogTime(anyString(args["endTime"]), now)
	if err != nil {
		return nil, fmt.Errorf("invalid end time: %w", err)
	}
	if !endAt.After(startAt) {
		return nil, errors.New("end time must be later than start time")
	}
	if endAt.Sub(startAt) > 31*24*time.Hour {
		return nil, errors.New("a single log query cannot span more than 31 days")
	}

	datasourceID := anyUint(args["datasourceId"])
	datasourceName := strings.TrimSpace(anyString(args["datasourceName"]))
	query := strings.TrimSpace(anyString(args["query"]))
	index := strings.TrimSpace(anyString(args["index"]))
	streams := splitAIQueryValues(anyString(args["streams"]))
	mode := strings.ToLower(strings.TrimSpace(anyString(args["mode"])))
	if mode == "" {
		mode = "count"
	}
	if mode != "count" && mode != "list" {
		return nil, errors.New("return mode supports only count or list")
	}
	limit := aiAssetQueryLimit(args["limit"])
	if mode == "count" {
		limit = 1
	}

	var datasources []model.MonitorDatasource
	datasourceQuery := s.db.Where("status = ? AND type IN ?", 1, []string{"elasticsearch", "victorialogs"})
	if datasourceID > 0 {
		datasourceQuery = datasourceQuery.Where("id = ?", datasourceID)
	} else if datasourceName != "" {
		datasourceQuery = datasourceQuery.Where("name LIKE ?", "%"+datasourceName+"%")
	}
	if err := datasourceQuery.Order("is_default DESC, id ASC").Find(&datasources).Error; err != nil {
		return nil, err
	}
	if len(datasources) == 0 {
		return nil, errors.New("no matching enabled Elasticsearch or VictoriaLogs datasource was found")
	}

	results := make([]map[string]any, 0, len(datasources))
	errorsByDatasource := make([]string, 0)
	var total int64
	for _, datasource := range datasources {
		datasourceQueryText := query
		if normalizeMonitorDatasourceType(datasource.Type) == "victorialogs" {
			datasourceQueryText = appendAILogStreamFilter(datasourceQueryText, streams, true)
		} else {
			datasourceQueryText = appendAILogStreamFilter(datasourceQueryText, streams, false)
		}
		payload := MonitorLogQueryPayload{
			DatasourceID:   datasource.ID,
			Index:          index,
			Query:          datasourceQueryText,
			StartAt:        startAt.UnixMilli(),
			EndAt:          endAt.UnixMilli(),
			PageSize:       limit,
			TrackTotalHits: mode == "count",
		}
		data, queryErr := s.QueryMonitorLogs(payload)
		if queryErr != nil {
			errorsByDatasource = append(errorsByDatasource, fmt.Sprintf("%s: %v", datasource.Name, queryErr))
			continue
		}
		count := aiLogTotal(data["total"])
		total += count
		item := map[string]any{
			"datasourceId": datasource.ID,
			"datasource":   datasource.Name,
			"type":         normalizeMonitorDatasourceType(datasource.Type),
			"count":        count,
			"tookMs":       data["took"],
		}
		if mode == "list" {
			item["items"] = data["items"]
		}
		results = append(results, item)
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("all log datasource queries failed: %s", strings.Join(errorsByDatasource, "; "))
	}
	return map[string]any{
		"mode": mode, "total": total, "query": query, "index": index, "streams": streams,
		"startTime": startAt.Format(time.RFC3339), "endTime": endAt.Format(time.RFC3339),
		"timezone": "Asia/Shanghai", "datasources": results, "errors": errorsByDatasource,
	}, nil
}

func parseAILogTime(value string, now time.Time) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, errors.New("time is required")
	}
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		location = time.Local
	}
	now = now.In(location)
	lower := strings.ToLower(value)
	for _, relative := range []struct {
		prefix string
		days   int
	}{
		{prefix: "어제", days: -1}, {prefix: "\u6628\u5929", days: -1}, {prefix: "yesterday", days: -1},
		{prefix: "오늘", days: 0}, {prefix: "\u4eca\u5929", days: 0}, {prefix: "today", days: 0},
	} {
		if strings.HasPrefix(lower, relative.prefix) {
			clock := strings.TrimSpace(value[len(relative.prefix):])
			if clock == "" {
				clock = "00:00"
			}
			parsedClock, parseErr := time.ParseInLocation("15:04:05", clock, location)
			if parseErr != nil {
				parsedClock, parseErr = time.ParseInLocation("15:04", clock, location)
			}
			if parseErr != nil {
				return time.Time{}, errors.New("relative time must use the form ‘어제 10:00’ or ‘today 10:00’")
			}
			date := now.AddDate(0, 0, relative.days)
			return time.Date(date.Year(), date.Month(), date.Day(), parsedClock.Hour(), parsedClock.Minute(), parsedClock.Second(), 0, location), nil
		}
	}
	if parsed, parseErr := time.Parse(time.RFC3339, value); parseErr == nil {
		return parsed.In(location), nil
	}
	for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02 15:04", "2006/01/02 15:04:05", "2006/01/02 15:04"} {
		if parsed, parseErr := time.ParseInLocation(layout, value, location); parseErr == nil {
			return parsed, nil
		}
	}
	return time.Time{}, errors.New("supported formats are RFC3339, YYYY-MM-DD HH:mm:ss, or ‘어제 10:00’")
}

func splitAIQueryValues(value string) []string {
	fields := strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == '，' || r == '\n' })
	result := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field != "" && !aiStringSliceContains(result, field) {
			result = append(result, field)
		}
	}
	return result
}

func appendAILogStreamFilter(query string, streams []string, victoriaLogs bool) string {
	query = strings.TrimSpace(query)
	if len(streams) == 0 {
		return query
	}
	if victoriaLogs {
		return appendAIVictoriaLogsStreamFilter(query, streams)
	}
	filters := make([]string, 0, len(streams))
	for _, rawStream := range streams {
		stream := strings.TrimSpace(rawStream)
		exact := strings.HasPrefix(stream, "=")
		if exact {
			stream = strings.TrimSpace(strings.TrimPrefix(stream, "="))
		}
		if stream == "" {
			continue
		}
		if exact {
			escaped := strings.ReplaceAll(strings.ReplaceAll(stream, "\\", "\\\\"), "\"", "\\\"")
			filters = append(filters, "kafka_topic:\""+escaped+"\"")
			continue
		}
		filters = append(filters, "kafka_topic:*"+escapeAILuceneWildcardValue(stream)+"*")
	}
	if len(filters) == 0 {
		return query
	}
	streamQuery := "(" + strings.Join(filters, " OR ") + ")"
	if query == "" {
		return streamQuery
	}
	return "(" + query + ") AND " + streamQuery
}

func appendAIVictoriaLogsStreamFilter(query string, streams []string) string {
	filters := make([]string, 0, len(streams))
	for _, rawStream := range streams {
		stream := strings.TrimSpace(rawStream)
		exact := strings.HasPrefix(stream, "=")
		if exact {
			stream = strings.TrimSpace(strings.TrimPrefix(stream, "="))
		}
		if stream == "" {
			continue
		}
		if exact {
			escaped := strings.ReplaceAll(strings.ReplaceAll(stream, "\\", "\\\\"), "\"", "\\\"")
			filters = append(filters, `{kafka_topic="`+escaped+`"}`)
		} else {
			filters = append(filters, "kafka_topic:*"+escapeAILuceneWildcardValue(stream)+"*")
		}
	}
	if len(filters) == 0 {
		return query
	}
	streamQuery := "(" + strings.Join(filters, " OR ") + ")"
	if query == "" || query == "*" {
		return streamQuery
	}
	return streamQuery + " AND (" + query + ")"
}

func escapeAILuceneWildcardValue(value string) string {
	const reserved = `+-=&|><!(){}[]^"~*?:\/`
	var builder strings.Builder
	for _, char := range value {
		if strings.ContainsRune(reserved, char) {
			builder.WriteByte('\\')
		}
		builder.WriteRune(char)
	}
	return builder.String()
}

func aiLogTotal(value any) int64 {
	switch typed := value.(type) {
	case int64:
		return typed
	case int:
		return int64(typed)
	case float64:
		return int64(typed)
	case json.Number:
		result, _ := typed.Int64()
		return result
	case map[string]any:
		return aiLogTotal(typed["value"])
	default:
		return 0
	}
}
