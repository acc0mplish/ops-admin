package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"ops-admin/backend/model"
)

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
			{"All Logs", "", "_all", "24h"},
			{"Error Logs", "ERROR", "_all", "24h"},
			{"Exceptions and Stack Traces", "(Exception OR ERROR OR Caused\\ by)", "_all", "24h"},
			{"Alerts and Warnings", "(WARN OR WARNING)", "_all", "24h"},
			{"Timed-out Requests", "(timeout OR timed\\ out OR TimeoutException)", "_all", "24h"},
			{"Connection Failures", "(connection\\ refused OR connection\\ reset OR connect\\ timeout)", "_all", "24h"},
			{"Kubernetes Restarts", "(CrashLoopBackOff OR OOMKilled OR Back-off\\ restarting)", "_all", "24h"},
			{"Application Startup", "(Started\\ .*Application OR application\\ started)", "_all", "24h"},
			{"Database Slow Queries", "(slow\\ query OR SlowQuery OR SQL\\ took)", "_all", "24h"},
			{"Specific Namespace", "kubernetes.pod_namespace:\"default\"", "_all", "6h"},
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
		return errors.New("saved query name is required")
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
		return errors.New("select a saved query")
	}
	owner = firstNonEmpty(strings.TrimSpace(owner), "admin")
	result := s.db.Where("id = ? AND owner = ?", id, owner).Delete(&model.MonitorLogShortcut{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("saved query does not exist or cannot be deleted by the current user")
	}
	return nil
}

func (s *Service) ListMonitorLogShortcutsByType(owner, datasourceType string) ([]model.MonitorLogShortcut, error) {
	owner = firstNonEmpty(strings.TrimSpace(owner), "admin")
	datasourceType = normalizeLogShortcutDatasourceType(datasourceType)
	query := s.db.Where("owner = ?", owner)
	if datasourceType == "victorialogs" {
		query = query.Where("datasource_type = ?", datasourceType)
	} else {
		query = query.Where("datasource_type = ? OR datasource_type = ''", datasourceType)
	}
	var count int64
	if err := query.Model(&model.MonitorLogShortcut{}).Count(&count).Error; err != nil {
		return nil, err
	}
	if count == 0 {
		defaults := monitorLogShortcutDefaults(datasourceType)
		items := make([]model.MonitorLogShortcut, 0, len(defaults))
		for i, item := range defaults {
			items = append(items, model.MonitorLogShortcut{Owner: owner, DatasourceType: datasourceType, Name: item.name, Query: item.query, IndexName: item.indexName, TimeRange: item.timeRange, Sort: i + 1})
		}
		if len(items) > 0 {
			if err := s.db.Create(&items).Error; err != nil {
				return nil, err
			}
		}
	}
	query = s.db.Where("owner = ?", owner)
	if datasourceType == "victorialogs" {
		query = query.Where("datasource_type = ?", datasourceType)
	} else {
		query = query.Where("datasource_type = ? OR datasource_type = ''", datasourceType)
	}
	var list []model.MonitorLogShortcut
	if err := query.Order("sort ASC, id ASC").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

func (s *Service) SaveMonitorLogShortcutByType(owner string, payload MonitorLogShortcutPayload) error {
	if strings.TrimSpace(payload.Name) == "" {
		return errors.New("saved query name is required")
	}
	owner = firstNonEmpty(strings.TrimSpace(owner), "admin")
	datasourceType := normalizeLogShortcutDatasourceType(payload.DatasourceType)
	updates := map[string]any{
		"datasource_type": datasourceType,
		"name":            Trimmed(payload.Name),
		"query":           strings.TrimSpace(payload.Query),
		"index_name":      firstNonEmpty(strings.TrimSpace(payload.IndexName), "_all"),
		"time_range":      firstNonEmpty(strings.TrimSpace(payload.TimeRange), "1h"),
		"sort":            payload.Sort,
	}
	if payload.ID > 0 {
		return s.db.Model(&model.MonitorLogShortcut{}).Where("id = ? AND owner = ?", payload.ID, owner).Updates(updates).Error
	}
	return s.db.Create(&model.MonitorLogShortcut{Owner: owner, DatasourceType: datasourceType, Name: updates["name"].(string), Query: updates["query"].(string), IndexName: updates["index_name"].(string), TimeRange: updates["time_range"].(string), Sort: payload.Sort}).Error
}

type monitorLogShortcutDefault struct {
	name, query, indexName, timeRange string
}

func normalizeLogShortcutDatasourceType(value string) string {
	if normalizeMonitorDatasourceType(value) == "victorialogs" {
		return "victorialogs"
	}
	return "elasticsearch"
}

func monitorLogShortcutDefaults(datasourceType string) []monitorLogShortcutDefault {
	if datasourceType == "victorialogs" {
		return []monitorLogShortcutDefault{
			{"All Logs", "*", "_all", "1h"},
			{"Error Logs", "_msg:error", "_all", "1h"},
			{"Exceptions and Stack Traces", "_msg:(Exception OR ERROR OR Caused)", "_all", "6h"},
			{"Alerts and Warnings", "_msg:(WARN OR WARNING)", "_all", "6h"},
			{"Timed-out Requests", "_msg:(timeout OR timed OR TimeoutException)", "_all", "6h"},
			{"Connection Failures", "_msg:(connection refused OR connection reset OR connect timeout)", "_all", "6h"},
			{"Kubernetes Restarts", "_msg:(CrashLoopBackOff OR OOMKilled OR Back-off)", "_all", "24h"},
			{"Application Startup", "_msg:(Started OR application started)", "_all", "24h"},
			{"Specific Namespace", "kubernetes.pod_namespace:default", "_all", "6h"},
			{"Kafka Error Topic", "kafka_topic:* AND _msg:error", "_all", "1h"},
		}
	}
	return []monitorLogShortcutDefault{
		{"All Logs", "", "_all", "1h"},
		{"Error Logs", "ERROR", "_all", "1h"},
		{"Exceptions and Stack Traces", "(Exception OR ERROR OR Caused\\ by)", "_all", "6h"},
		{"Alerts and Warnings", "(WARN OR WARNING)", "_all", "6h"},
		{"Timed-out Requests", "(timeout OR timed\\ out OR TimeoutException)", "_all", "6h"},
		{"Connection Failures", "(connection\\ refused OR connection\\ reset OR connect\\ timeout)", "_all", "6h"},
		{"Kubernetes Restarts", "(CrashLoopBackOff OR OOMKilled OR Back-off\\ restarting)", "_all", "24h"},
		{"Application Startup", "(Started\\ .*Application OR application\\ started)", "_all", "24h"},
		{"Database Slow Queries", "(slow\\ query OR SlowQuery OR SQL\\ took)", "_all", "24h"},
		{"Specific Namespace", "kubernetes.pod_namespace:\"default\"", "_all", "6h"},
	}
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
		return nil, fmt.Errorf("Elasticsearch query failed with status %d: %s", response.StatusCode, string(responseBody))
	}
	var result map[string]any
	if err := json.Unmarshal(responseBody, &result); err != nil {
		return nil, err
	}
	return result, nil
}
