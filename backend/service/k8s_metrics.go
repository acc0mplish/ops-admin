// k8s_metrics.go — moved verbatim from k8s.go (Phase BCD, E5 seam #4).
package service

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"ops-admin/backend/model"
)

// GetK8sPodMetrics returns the basic CPU and memory history for one Pod. The
// datasource stays server-side so kube pages never expose monitoring endpoint
// credentials or need to choose a datasource themselves.
func (s *Service) GetK8sPodMetrics(clusterID uint, namespace string, podName string, rangeKey string) (map[string]any, error) {
	if clusterID == 0 || strings.TrimSpace(namespace) == "" || strings.TrimSpace(podName) == "" {
		return nil, errors.New("invalid pod metrics query")
	}
	cluster, err := s.GetK8sCluster(clusterID)
	if err != nil {
		return nil, err
	}
	startAt, endAt, stepSeconds, normalizedRange := resolveK8sPodMetricRange(rangeKey)
	response := map[string]any{
		"status": "unavailable",
		"range":  map[string]any{"key": normalizedRange, "startAt": startAt.Unix(), "endAt": endAt.Unix(), "stepSeconds": stepSeconds},
		"metrics": map[string]any{
			"cpu":    map[string]any{"latest": nil, "points": []map[string]any{}},
			"memory": map[string]any{"latest": nil, "points": []map[string]any{}},
		},
	}

	if cluster.MonitorDatasourceID == nil || *cluster.MonitorDatasourceID == 0 {
		response["status"] = "not_configured"
		return response, nil
	}
	var datasource model.MonitorDatasource
	if err := s.db.First(&datasource, *cluster.MonitorDatasourceID).Error; err != nil {
		response["status"] = "not_configured"
		return response, nil
	}
	if datasource.Status != 1 || !isMonitorMetricDatasource(datasource.Type) {
		response["status"] = "not_configured"
		return response, nil
	}
	selector := fmt.Sprintf(`namespace=%q,pod=%q,container!="",container!="POD"`, namespace, podName)
	queries := map[string]string{
		"cpu":    fmt.Sprintf("sum(rate(container_cpu_usage_seconds_total{%s}[5m]))", selector),
		"memory": fmt.Sprintf("sum(container_memory_working_set_bytes{%s})", selector),
	}
	metrics := response["metrics"].(map[string]any)
	queryErrors := make(map[string]string)
	for name, query := range queries {
		result, err := s.prometheusRangeQuery(datasource, query, startAt, endAt, stepSeconds)
		if err != nil {
			queryErrors[name] = err.Error()
			continue
		}
		points := k8sPodMetricPoints(result)
		metrics[name] = map[string]any{"latest": k8sPodMetricLatest(points), "points": points}
	}
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

// GetK8sWorkloadMetrics compares the runtime Pods that currently belong to a workload.
func (s *Service) GetK8sWorkloadMetrics(clusterID uint, namespace, workloadType, workloadName, rangeKey string) (map[string]any, error) {
	if clusterID == 0 || strings.TrimSpace(namespace) == "" || strings.TrimSpace(workloadType) == "" || strings.TrimSpace(workloadName) == "" {
		return nil, errors.New("invalid workload metrics query")
	}
	detail, err := s.GetK8sWorkloadDetail(clusterID, namespace, workloadType, workloadName)
	if err != nil {
		return nil, err
	}
	podNames := make([]string, 0, len(detail.Pods))
	for _, pod := range detail.Pods {
		podNames = append(podNames, pod.Name)
	}
	return s.getK8sPodMetricComparison(clusterID, namespace, podNames, rangeKey)
}

func (s *Service) getK8sPodMetricComparison(clusterID uint, namespace string, podNames []string, rangeKey string) (map[string]any, error) {
	cluster, err := s.GetK8sCluster(clusterID)
	if err != nil {
		return nil, err
	}
	startAt, endAt, stepSeconds, normalizedRange := resolveK8sPodMetricRange(rangeKey)
	response := map[string]any{
		"status": "unavailable",
		"range":  map[string]any{"key": normalizedRange, "startAt": startAt.Unix(), "endAt": endAt.Unix(), "stepSeconds": stepSeconds},
		"metrics": map[string]any{
			"cpu":        map[string]any{"series": []map[string]any{}},
			"memory":     map[string]any{"series": []map[string]any{}},
			"wss":        map[string]any{"series": []map[string]any{}},
			"networkIn":  map[string]any{"series": []map[string]any{}},
			"networkOut": map[string]any{"series": []map[string]any{}},
		},
	}
	if cluster.MonitorDatasourceID == nil || *cluster.MonitorDatasourceID == 0 || len(podNames) == 0 {
		response["status"] = "not_configured"
		return response, nil
	}
	var datasource model.MonitorDatasource
	if err := s.db.First(&datasource, *cluster.MonitorDatasourceID).Error; err != nil || datasource.Status != 1 || !isMonitorMetricDatasource(datasource.Type) {
		response["status"] = "not_configured"
		return response, nil
	}
	quotedNames := make([]string, 0, len(podNames))
	for _, name := range podNames {
		if name = strings.TrimSpace(name); name != "" {
			quotedNames = append(quotedNames, regexp.QuoteMeta(name))
		}
	}
	if len(quotedNames) == 0 {
		return response, nil
	}
	selector := fmt.Sprintf(`namespace=%q,pod=~%q,container!="",container!="POD"`, namespace, "^("+strings.Join(quotedNames, "|")+")$")
	queries := map[string]string{
		"cpu":    fmt.Sprintf("sum by (pod) (rate(container_cpu_usage_seconds_total{%s}[5m]))", selector),
		"memory": fmt.Sprintf("sum by (pod) (container_memory_rss{%s})", selector),
		"wss": fmt.Sprintf(
			"100 * (sum by (pod) (container_memory_working_set_bytes{%s}) / on (pod) sum by (pod) (kube_pod_container_resource_limits{namespace=%q,pod=~%q,resource=\"memory\"}))",
			selector, namespace, "^("+strings.Join(quotedNames, "|")+")$",
		),
		"networkIn":  fmt.Sprintf("sum by (pod) (rate(container_network_receive_bytes_total{namespace=%q,pod=~%q}[5m]))", namespace, "^("+strings.Join(quotedNames, "|")+")$"),
		"networkOut": fmt.Sprintf("sum by (pod) (rate(container_network_transmit_bytes_total{namespace=%q,pod=~%q}[5m]))", namespace, "^("+strings.Join(quotedNames, "|")+")$"),
	}
	metrics := response["metrics"].(map[string]any)
	queryErrors := make(map[string]string)
	for name, query := range queries {
		result, queryErr := s.prometheusRangeQuery(datasource, query, startAt, endAt, stepSeconds)
		if queryErr != nil {
			queryErrors[name] = queryErr.Error()
			continue
		}
		metrics[name] = map[string]any{"series": k8sPodMetricSeries(result)}
	}
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

func k8sPodMetricSeries(result *PromQueryResult) []map[string]any {
	series := make([]map[string]any, 0)
	if result == nil {
		return series
	}
	for _, sample := range result.Data.Result {
		points := make([]map[string]any, 0, len(sample.Values))
		for _, pair := range sample.Values {
			if len(pair) < 2 {
				continue
			}
			timestamp, timestampErr := strconv.ParseFloat(fmt.Sprint(pair[0]), 64)
			value, valueErr := strconv.ParseFloat(fmt.Sprint(pair[1]), 64)
			if timestampErr == nil && valueErr == nil {
				points = append(points, map[string]any{"timestamp": int64(timestamp * 1000), "value": value})
			}
		}
		if len(points) > 0 {
			series = append(series, map[string]any{"name": sample.Metric["pod"], "latest": points[len(points)-1]["value"], "points": points})
		}
	}
	sort.Slice(series, func(left, right int) bool {
		return fmt.Sprint(series[left]["name"]) < fmt.Sprint(series[right]["name"])
	})
	return series
}

func resolveK8sPodMetricRange(rangeKey string) (time.Time, time.Time, int, string) {
	definitions := map[string]struct {
		duration time.Duration
		step     int
	}{
		"1h":  {duration: time.Hour, step: 30},
		"6h":  {duration: 6 * time.Hour, step: 120},
		"24h": {duration: 24 * time.Hour, step: 300},
	}
	definition, ok := definitions[rangeKey]
	if !ok {
		rangeKey = "1h"
		definition = definitions[rangeKey]
	}
	endAt := time.Now()
	return endAt.Add(-definition.duration), endAt, definition.step, rangeKey
}

func k8sPodMetricPoints(result *PromQueryResult) []map[string]any {
	values := make(map[int64]float64)
	if result != nil {
		for _, sample := range result.Data.Result {
			for _, pair := range sample.Values {
				if len(pair) < 2 {
					continue
				}
				timestamp, timestampErr := strconv.ParseFloat(fmt.Sprint(pair[0]), 64)
				value, valueErr := strconv.ParseFloat(fmt.Sprint(pair[1]), 64)
				if timestampErr == nil && valueErr == nil {
					values[int64(timestamp*1000)] += value
				}
			}
		}
	}
	points := make([]map[string]any, 0, len(values))
	for timestamp, value := range values {
		points = append(points, map[string]any{"timestamp": timestamp, "value": value})
	}
	sort.Slice(points, func(left, right int) bool {
		return points[left]["timestamp"].(int64) < points[right]["timestamp"].(int64)
	})
	return points
}

func k8sPodMetricLatest(points []map[string]any) any {
	if len(points) == 0 {
		return nil
	}
	return points[len(points)-1]["value"]
}

func (s *Service) GetK8sPodLogs(clusterID uint, namespace string, podName string, container string, tailLines int) (map[string]any, error) {
	_, runtime, client, err := s.k8sClientForCluster(clusterID)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/log", namespace, podName)
	query := map[string]string{}
	if strings.TrimSpace(container) != "" {
		query["container"] = strings.TrimSpace(container)
	}
	if tailLines <= 0 {
		tailLines = 200
	}
	if tailLines > 1000 {
		tailLines = 1000
	}
	query["tailLines"] = strconv.Itoa(tailLines)
	body, err := k8sGetText(client, runtime, path, query)
	if err != nil {
		return nil, errors.New(k8sClusterConnectError)
	}

	return map[string]any{
		"namespace": namespace,
		"podName":   podName,
		"container": container,
		"tailLines": tailLines,
		"content":   body,
	}, nil
}

func (s *Service) GetK8sPodEvents(clusterID uint, namespace string, podName string) ([]model.K8sEventItem, error) {
	_, runtime, client, err := s.k8sClientForCluster(clusterID)
	if err != nil {
		return nil, err
	}

	return fetchNamespacedEvents(client, runtime, namespace, fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=Pod", podName))
}

func (s *Service) GetK8sNamespaceDetail(clusterID uint, namespace string) (model.K8sNamespaceDetail, error) {
	_, runtime, client, err := s.k8sClientForCluster(clusterID)
	if err != nil {
		return model.K8sNamespaceDetail{}, err
	}

	var ns kubeNamespace
	if err := k8sGetJSON(client, runtime, "/api/v1/namespaces/"+namespace, &ns); err != nil {
		return model.K8sNamespaceDetail{}, errors.New(k8sClusterConnectError)
	}

	var pods kubePodListResponse
	if err := k8sGetJSON(client, runtime, "/api/v1/namespaces/"+namespace+"/pods", &pods); err != nil {
		return model.K8sNamespaceDetail{}, errors.New(k8sClusterConnectError)
	}
	var services kubeServiceListResponse
	if err := k8sGetJSON(client, runtime, "/api/v1/namespaces/"+namespace+"/services", &services); err != nil {
		return model.K8sNamespaceDetail{}, errors.New(k8sClusterConnectError)
	}
	var configMaps kubeConfigMapListResponse
	if err := k8sGetJSON(client, runtime, "/api/v1/namespaces/"+namespace+"/configmaps", &configMaps); err != nil {
		return model.K8sNamespaceDetail{}, errors.New(k8sClusterConnectError)
	}
	var secrets kubeSecretListResponse
	if err := k8sGetJSON(client, runtime, "/api/v1/namespaces/"+namespace+"/secrets", &secrets); err != nil {
		return model.K8sNamespaceDetail{}, errors.New(k8sClusterConnectError)
	}

	storageCount := 0
	var pvcs kubePVCListResponse
	if err := k8sGetJSON(client, runtime, "/api/v1/namespaces/"+namespace+"/persistentvolumeclaims", &pvcs); err == nil {
		storageCount = len(pvcs.Items)
	}

	workloadCount := 0
	if count, err := fetchNamespaceWorkloadCount(client, runtime, namespace); err == nil {
		workloadCount = count
	}

	return model.K8sNamespaceDetail{
		Name:        ns.Metadata.Name,
		Status:      fallbackText(ns.Status.Phase),
		CreatedAt:   formatTimestamp(ns.Metadata.CreationTimestamp),
		Labels:      ns.Metadata.Labels,
		Annotations: ns.Metadata.Annotations,
		Pods:        len(pods.Items),
		Services:    len(services.Items),
		Workloads:   workloadCount,
		ConfigMaps:  len(configMaps.Items),
		Secrets:     len(secrets.Items),
		Storage:     storageCount,
		YAML:        marshalK8sYAML(ns),
	}, nil
}
