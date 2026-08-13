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

// enrichAssetHostUsageMetrics enriches a host list from the configured local
// monitoring datasource. Metrics are optional: an unavailable datasource must
// never prevent CMDB hosts from being listed.
func (s *Service) enrichAssetHostUsageMetrics(hosts []model.AssetHost) {
	for index := range hosts {
		hosts[index].MetricsStatus = "unavailable"
	}
	if len(hosts) == 0 {
		return
	}

	var datasource model.MonitorDatasource
	err := s.db.Where("status = ? AND type IN ?", 1, []string{"prometheus", "victoriametrics"}).
		Order("is_default DESC, id ASC").First(&datasource).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			for index := range hosts {
				hosts[index].MetricsStatus = "not_configured"
			}
		}
		return
	}

	type queryResult struct {
		name   string
		result *PromQueryResult
	}
	queries := map[string]string{
		"cpu":    hostCPUUsagePromQL,
		"memory": hostMemoryUsagePromQL,
		"disk":   hostDiskUsagePromQL,
	}
	results := make(chan queryResult, len(queries))
	var waitGroup sync.WaitGroup
	for name, query := range queries {
		waitGroup.Add(1)
		go func(name, query string) {
			defer waitGroup.Done()
			result, queryErr := s.prometheusQuery(datasource, query, time.Now())
			if queryErr == nil {
				results <- queryResult{name: name, result: result}
			}
		}(name, query)
	}
	waitGroup.Wait()
	close(results)

	values := map[string]map[string]float64{"cpu": {}, "memory": {}, "disk": {}}
	for queryResult := range results {
		for _, sample := range queryResult.result.Data.Result {
			value, ok := promSampleValue(sample)
			if !ok {
				continue
			}
			instance := normalizeHostMetricKey(sample.Metric["instance"])
			if instance != "" {
				values[queryResult.name][instance] = value
			}
		}
	}
	for index := range hosts {
		keys := assetHostMetricKeys(hosts[index])
		cpu, cpuOK := findAssetHostMetric(values["cpu"], keys)
		memory, memoryOK := findAssetHostMetric(values["memory"], keys)
		disk, diskOK := findAssetHostMetric(values["disk"], keys)
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
	var datasource model.MonitorDatasource
	if err := s.db.Where("status = ? AND type IN ?", 1, []string{"prometheus", "victoriametrics"}).Order("is_default DESC, id ASC").First(&datasource).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			response["status"] = "not_configured"
			return response, nil
		}
		return response, nil
	}
	keys := assetHostMetricKeys(host)
	// Keep the range PromQL identical to the host-list instant queries. The
	// host list already proves these expressions and its instance matching work
	// against the configured datasource.
	queries := map[string]string{
		"cpu":    hostCPUUsagePromQL,
		"memory": hostMemoryUsagePromQL,
		"disk":   hostDiskUsagePromQL,
		"io":     hostIOUsagePromQL,
	}
	metrics := emptyAssetHostMetricSeries()
	queryErrors := make(map[string]string)
	for name, query := range queries {
		result, queryErr := s.prometheusRangeQuery(datasource, query, startAt, endAt, stepSeconds)
		if queryErr != nil {
			queryErrors[name] = queryErr.Error()
			continue
		}
		metrics[name] = map[string]any{"latest": assetHostMetricLatest(result, keys), "points": assetHostMetricPoints(result, keys)}
	}
	response["metrics"] = metrics
	if len(queryErrors) > 0 {
		response["errors"] = queryErrors
	}
	if len(queryErrors) == len(queries) {
		response["status"] = "query_failed"
	} else {
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
