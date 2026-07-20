package service

import (
	"fmt"
	"net"
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
