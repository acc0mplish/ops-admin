package service

import (
	"errors"
	"strings"
	"time"

	"ops-admin/backend/model"

	"gorm.io/gorm"
)

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
	if isMonitorLogDatasource(ds.Type) {
		return errors.New("log datasources do not support PromQL monitoring panels; select Prometheus or VictoriaMetrics")
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
	if isMonitorLogDatasource(ds.Type) {
		var fallback model.MonitorDatasource
		if err := s.db.Where("status = ? AND type IN ?", 1, []string{"prometheus", "victoriametrics"}).Order("is_default DESC, id DESC").First(&fallback).Error; err != nil {
			return nil, errors.New("monitoring panels support only Prometheus or VictoriaMetrics; configure a metric datasource first")
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
