package service

import (
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"ops-admin/backend/model"

	"gorm.io/gorm"
)

const (
	hostCPUUsagePromQL    = `100 - (avg by (instance) (rate(node_cpu_seconds_total{mode="idle"}[5m])) * 100)`
	hostMemoryUsagePromQL = `(1 - (node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes)) * 100`
	hostDiskUsagePromQL   = `max by (instance) (100 * (1 - node_filesystem_avail_bytes{fstype!~"tmpfs|overlay|squashfs"} / node_filesystem_size_bytes{fstype!~"tmpfs|overlay|squashfs"}))`
	hostIOUsagePromQL     = `avg by (instance) (rate(node_cpu_seconds_total{mode="iowait"}[5m])) * 100`
)

// enrichAssetHostUsageMetrics enriches a host list from every enabled local
// metrics datasource. A host can be collected by a non-default Prometheus, so
// querying only the default datasource silently drops valid host metrics.
// Metrics are optional: an unavailable datasource must never prevent CMDB
// hosts from being listed.
func (s *Service) enrichAssetHostUsageMetrics(hosts []model.AssetHost) {
	for index := range hosts {
		hosts[index].MetricsStatus = "unavailable"
	}
	if len(hosts) == 0 {
		return
	}

	datasources, err := s.assetHostMetricDatasources()
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			for index := range hosts {
				hosts[index].MetricsStatus = "not_configured"
			}
		}
		return
	}

	type queryResult struct {
		datasourceIndex int
		name            string
		result          *PromQueryResult
	}
	queries := map[string]string{
		"cpu":    hostCPUUsagePromQL,
		"memory": hostMemoryUsagePromQL,
		"disk":   hostDiskUsagePromQL,
	}
	results := make(chan queryResult, len(queries)*len(datasources))
	var waitGroup sync.WaitGroup
	for datasourceIndex, datasource := range datasources {
		for name, query := range queries {
			waitGroup.Add(1)
			go func(datasourceIndex int, datasource model.MonitorDatasource, name, query string) {
				defer waitGroup.Done()
				result, queryErr := s.prometheusQuery(datasource, query, time.Now())
				if queryErr == nil {
					results <- queryResult{datasourceIndex: datasourceIndex, name: name, result: result}
				}
			}(datasourceIndex, datasource, name, query)
		}
	}
	waitGroup.Wait()
	close(results)

	valuesByDatasource := make([]map[string]map[string]float64, len(datasources))
	for index := range valuesByDatasource {
		valuesByDatasource[index] = map[string]map[string]float64{"cpu": {}, "memory": {}, "disk": {}}
	}
	for queryResult := range results {
		for _, sample := range queryResult.result.Data.Result {
			value, ok := promSampleValue(sample)
			if !ok {
				continue
			}
			instance := normalizeHostMetricKey(sample.Metric["instance"])
			if instance != "" {
				valuesByDatasource[queryResult.datasourceIndex][queryResult.name][instance] = value
			}
		}
	}
	for index := range hosts {
		keys := assetHostMetricKeys(hosts[index])
		cpu, cpuOK := findAssetHostMetricAcrossDatasources(valuesByDatasource, "cpu", keys)
		memory, memoryOK := findAssetHostMetricAcrossDatasources(valuesByDatasource, "memory", keys)
		disk, diskOK := findAssetHostMetricAcrossDatasources(valuesByDatasource, "disk", keys)
		if cpuOK {
			hosts[index].CPUUsage = formatAssetHostUsage(cpu)
		}
		if memoryOK {
			hosts[index].MemoryUsage = formatAssetHostUsage(memory)
		}
		if diskOK {
			hosts[index].DiskUsage = formatAssetHostUsage(disk)
		}
		if cpuOK || memoryOK || diskOK {
			hosts[index].MetricsStatus = "available"
		}
	}
}

func (s *Service) assetHostMetricDatasources() ([]model.MonitorDatasource, error) {
	var datasources []model.MonitorDatasource
	err := s.db.Where("status = ? AND type IN ?", 1, []string{"prometheus", "victoriametrics"}).
		Order("is_default DESC, id ASC").Find(&datasources).Error
	if err != nil {
		return nil, err
	}
	if len(datasources) == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return datasources, nil
}

func assetHostMetricKeys(host model.AssetHost) []string {
	values := []string{host.PrivateIP, host.PublicIP, host.SSHIP, host.HostName, host.Alias}
	keys := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		key := normalizeHostMetricKey(value)
		if key != "" && !seen[key] {
			seen[key] = true
			keys = append(keys, key)
		}
	}
	return keys
}

func normalizeHostMetricKey(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if host, _, err := net.SplitHostPort(value); err == nil {
		value = host
	}
	return strings.Trim(value, "[]")
}

func findAssetHostMetric(values map[string]float64, keys []string) (float64, bool) {
	for _, key := range keys {
		if value, ok := values[key]; ok {
			return value, true
		}
	}
	return 0, false
}

func findAssetHostMetricAcrossDatasources(valuesByDatasource []map[string]map[string]float64, metric string, keys []string) (float64, bool) {
	for _, values := range valuesByDatasource {
		if value, ok := findAssetHostMetric(values[metric], keys); ok {
			return value, true
		}
	}
	return 0, false
}

func formatAssetHostUsage(value float64) string {
	if value < 0 {
		value = 0
	}
	if value > 100 {
		value = 100
	}
	return fmt.Sprintf("%.1f%%", value)
}

// GetAssetHostMetrics returns local Prometheus/VictoriaMetrics history for one
// registered host. It intentionally does not contact the cloud provider.
func (s *Service) GetAssetHostMetrics(id uint, rangeKey, startValue, endValue string) (map[string]any, error) {
	var host model.AssetHost
	if err := s.db.First(&host, id).Error; err != nil {
		return nil, err
	}
	startAt, endAt, stepSeconds, normalizedRange, err := resolveAssetHostMetricRange(rangeKey, startValue, endValue)
	if err != nil {
		return nil, err
	}
	response := map[string]any{
		"status":  "unavailable",
		"range":   map[string]any{"key": normalizedRange, "startAt": startAt, "endAt": endAt, "stepSeconds": stepSeconds},
		"metrics": emptyAssetHostMetricSeries(),
	}
	datasources, err := s.assetHostMetricDatasources()
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			response["status"] = "not_configured"
			return response, nil
		}
		return response, nil
	}
	keys := assetHostMetricKeys(host)
	// Keep the range PromQL identical to the host-list instant queries. Each
	// enabled datasource is tried in priority order and the first one that has
	// series for this host supplies the history.
	queries := map[string]string{
		"cpu":    hostCPUUsagePromQL,
		"memory": hostMemoryUsagePromQL,
		"disk":   hostDiskUsagePromQL,
		"io":     hostIOUsagePromQL,
	}
	metrics := emptyAssetHostMetricSeries()
	queryErrors := make(map[string]string)
	matchedAnyMetric := false
	successfulQueries := 0
	for name, query := range queries {
		var errors []string
		for _, datasource := range datasources {
			result, queryErr := s.prometheusRangeQuery(datasource, query, startAt, endAt, stepSeconds)
			if queryErr != nil {
				errors = append(errors, datasource.Name+": "+queryErr.Error())
				continue
			}
			successfulQueries++
			points := assetHostMetricPoints(result, keys)
			if len(points) == 0 {
				continue
			}
			metrics[name] = map[string]any{"latest": points[len(points)-1]["value"], "points": points, "datasourceId": datasource.ID, "datasourceName": datasource.Name}
			matchedAnyMetric = true
			break
		}
		if len(errors) == len(datasources) {
			queryErrors[name] = strings.Join(errors, "; ")
		}
	}
	response["metrics"] = metrics
	if len(queryErrors) > 0 {
		response["errors"] = queryErrors
	}
	if successfulQueries == 0 {
		response["status"] = "query_failed"
	} else if matchedAnyMetric {
		response["status"] = "available"
	}
	return response, nil
}

func emptyAssetHostMetricSeries() map[string]any {
	return map[string]any{
		"cpu":    map[string]any{"latest": nil, "points": []map[string]any{}},
		"memory": map[string]any{"latest": nil, "points": []map[string]any{}},
		"disk":   map[string]any{"latest": nil, "points": []map[string]any{}},
		"io":     map[string]any{"latest": nil, "points": []map[string]any{}},
	}
}

func resolveAssetHostMetricRange(rangeKey, startValue, endValue string) (time.Time, time.Time, int, string, error) {
	now := time.Now()
	definitions := map[string]struct {
		duration time.Duration
		step     int
	}{
		"1h": {time.Hour, 30}, "3h": {3 * time.Hour, 60}, "6h": {6 * time.Hour, 120}, "12h": {12 * time.Hour, 180},
		"1d": {24 * time.Hour, 300}, "3d": {72 * time.Hour, 900}, "7d": {7 * 24 * time.Hour, 1800}, "14d": {14 * 24 * time.Hour, 3600},
	}
	if rangeKey == "custom" {
		startAt, startErr := parseAssetHostMetricTime(startValue)
		endAt, endErr := parseAssetHostMetricTime(endValue)
		if startErr != nil || endErr != nil || !endAt.After(startAt) {
			return time.Time{}, time.Time{}, 0, "custom", fmt.Errorf("invalid custom metric time range")
		}
		if endAt.Sub(startAt) > 14*24*time.Hour {
			return time.Time{}, time.Time{}, 0, "custom", fmt.Errorf("custom metric time range cannot exceed 14 days")
		}
		step := int(endAt.Sub(startAt).Seconds() / 240)
		if step < 30 {
			step = 30
		}
		if step > 3600 {
			step = 3600
		}
		return startAt, endAt, step, "custom", nil
	}
	definition, ok := definitions[rangeKey]
	if !ok {
		definition = definitions["1h"]
		rangeKey = "1h"
	}
	return now.Add(-definition.duration), now, definition.step, rangeKey, nil
}

func parseAssetHostMetricTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if milliseconds, err := strconv.ParseInt(value, 10, 64); err == nil {
		return time.UnixMilli(milliseconds), nil
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed, nil
	}
	return time.ParseInLocation("2006-01-02T15:04:05", value, time.Local)
}

func assetHostMetricPoints(result *PromQueryResult, keys []string) []map[string]any {
	if result == nil {
		return []map[string]any{}
	}
	points := make([]map[string]any, 0)
	for _, sample := range result.Data.Result {
		if _, ok := findAssetHostMetric(map[string]float64{normalizeHostMetricKey(sample.Metric["instance"]): 1}, keys); !ok {
			continue
		}
		for _, pair := range sample.Values {
			if len(pair) < 2 {
				continue
			}
			timestamp, timestampErr := strconv.ParseFloat(fmt.Sprint(pair[0]), 64)
			value, valueErr := strconv.ParseFloat(fmt.Sprint(pair[1]), 64)
			if timestampErr != nil || valueErr != nil {
				continue
			}
			points = append(points, map[string]any{"timestamp": int64(timestamp * 1000), "value": value})
		}
	}
	sort.Slice(points, func(left, right int) bool {
		return points[left]["timestamp"].(int64) < points[right]["timestamp"].(int64)
	})
	return points
}

func assetHostMetricLatest(result *PromQueryResult, keys []string) any {
	points := assetHostMetricPoints(result, keys)
	if len(points) == 0 {
		return nil
	}
	return points[len(points)-1]["value"]
}
