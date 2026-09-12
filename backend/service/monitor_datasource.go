package service

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"ops-admin/backend/model"

	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

type MonitorScheduler struct {
	cron        *cron.Cron
	mu          sync.Mutex
	entries     map[uint]cron.EntryID
	running     map[uint]bool
	healthEntry cron.EntryID
}

func (s *Service) initMonitorScheduler() {
	s.monitorSchedulerOnce.Do(func() {
		s.monitorScheduler = &MonitorScheduler{
			cron: cron.New(
				cron.WithSeconds(),
				cron.WithLocation(time.Local),
				cron.WithChain(cron.SkipIfStillRunning(cron.DefaultLogger)),
			),
			entries: map[uint]cron.EntryID{},
			running: map[uint]bool{},
		}
		s.monitorScheduler.cron.Start()
		s.reloadMonitorAlertRules()
		// Repair historical events left firing by rules that were disabled before
		// the automatic-close behavior existed.
		s.closeInactiveMonitorAlertRuleEvents()
		// Aggregated alerts are intentionally delayed until their convergence
		// window closes, so a short cadence is needed even when no rule happens
		// to be evaluated at that exact moment.
		_, _ = s.monitorScheduler.cron.AddFunc("@every 15s", s.flushDueMonitorAggregationNotifications)
		entryID, err := s.monitorScheduler.cron.AddFunc("@every 60s", s.checkAllMonitorDatasources)
		if err == nil {
			s.monitorScheduler.healthEntry = entryID
		}
		go s.checkAllMonitorDatasources()
	})
}

func (s *Service) reloadMonitorAlertRules() {
	if s.monitorScheduler == nil {
		return
	}
	var rules []model.MonitorAlertRule
	if err := s.db.Where("status = ?", 1).Find(&rules).Error; err != nil {
		return
	}
	for _, rule := range rules {
		_ = s.registerMonitorAlertRule(rule)
	}
}

func (s *Service) registerMonitorAlertRule(rule model.MonitorAlertRule) error {
	if s.monitorScheduler == nil {
		return nil
	}
	s.removeMonitorAlertRule(rule.ID)
	interval := normalizeEvalInterval(rule.EvalIntervalSeconds)
	entryID, err := s.monitorScheduler.cron.AddFunc(fmt.Sprintf("@every %ds", interval), func() {
		s.evaluateMonitorAlertRule(rule.ID)
	})
	if err != nil {
		return err
	}
	s.monitorScheduler.mu.Lock()
	s.monitorScheduler.entries[rule.ID] = entryID
	s.monitorScheduler.mu.Unlock()
	return nil
}

func (s *Service) removeMonitorAlertRule(id uint) {
	if s.monitorScheduler == nil {
		return
	}
	s.monitorScheduler.mu.Lock()
	entryID, ok := s.monitorScheduler.entries[id]
	if ok {
		delete(s.monitorScheduler.entries, id)
	}
	s.monitorScheduler.mu.Unlock()
	if ok {
		s.monitorScheduler.cron.Remove(entryID)
	}
}

func (s *Service) ListMonitorDatasources(pageNum, pageSize int, keyword, dsType, status, env string) (map[string]any, error) {
	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	query := s.db.Model(&model.MonitorDatasource{})
	if strings.TrimSpace(keyword) != "" {
		like := "%" + strings.TrimSpace(keyword) + "%"
		query = query.Where("name LIKE ? OR url LIKE ? OR description LIKE ?", like, like, like)
	}
	if strings.TrimSpace(dsType) != "" {
		query = query.Where("type = ?", normalizeMonitorDatasourceType(dsType))
	}
	if strings.TrimSpace(status) != "" {
		query = query.Where("status = ?", status)
	}
	if strings.TrimSpace(env) != "" {
		query = query.Where("env = ?", normalizeEnvCode(env))
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var list []model.MonitorDatasource
	if err := query.Order("is_default DESC, id DESC").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, err
	}
	return map[string]any{"list": list, "total": total, "pageNum": pageNum, "pageSize": pageSize}, nil
}

func (s *Service) ListMonitorDatasourceOptions() ([]model.MonitorDatasource, error) {
	var list []model.MonitorDatasource
	if err := s.db.Where("status = ?", 1).Order("is_default DESC, id DESC").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

func (s *Service) GetMonitorDatasource(id uint) (*model.MonitorDatasource, error) {
	var item model.MonitorDatasource
	if err := s.db.First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Service) SaveMonitorDatasource(payload MonitorDatasourcePayload) error {
	if strings.TrimSpace(payload.Name) == "" {
		return errors.New("datasource name is required")
	}
	if strings.TrimSpace(payload.URL) == "" {
		return errors.New("datasource URL is required")
	}
	updates := map[string]any{
		"name":        Trimmed(payload.Name),
		"type":        normalizeMonitorDatasourceType(payload.Type),
		"url":         strings.TrimRight(strings.TrimSpace(payload.URL), "/"),
		"auth_type":   normalizeMonitorAuthType(payload.AuthType),
		"username":    strings.TrimSpace(payload.Username),
		"password":    payload.Password,
		"token":       strings.TrimSpace(payload.Token),
		"is_default":  payload.IsDefault,
		"env":         normalizeEnvCode(payload.Env),
		"status":      normalizeMonitorStatus(payload.Status),
		"description": Trimmed(payload.Description),
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		if payload.IsDefault {
			if err := tx.Model(&model.MonitorDatasource{}).Where("id <> ?", payload.ID).Update("is_default", false).Error; err != nil {
				return err
			}
		}
		if payload.ID > 0 {
			return tx.Model(&model.MonitorDatasource{}).Where("id = ?", payload.ID).Updates(updates).Error
		}
		return tx.Model(&model.MonitorDatasource{}).Create(updates).Error
	})
}

func (s *Service) DeleteMonitorDatasource(id uint) error {
	var count int64
	if err := s.db.Model(&model.MonitorAlertRule{}).Where("datasource_id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return errors.New("datasource is referenced by alert rules and cannot be deleted")
	}
	if err := s.db.Model(&model.MonitorDashboardPanel{}).Where("datasource_id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return errors.New("datasource is referenced by monitoring dashboards and cannot be deleted")
	}
	clusterRefs, err := s.countK8sClustersByMonitorDatasource(id)
	if err != nil {
		return err
	}
	if clusterRefs > 0 {
		return errors.New("datasource is bound to Kubernetes cluster monitoring and cannot be deleted")
	}
	return s.db.Delete(&model.MonitorDatasource{}, id).Error
}

func (s *Service) TestMonitorDatasource(id uint, payload MonitorDatasourcePayload) error {
	var ds model.MonitorDatasource
	if id > 0 {
		item, err := s.GetMonitorDatasource(id)
		if err != nil {
			return err
		}
		ds = *item
	} else {
		ds = model.MonitorDatasource{
			Name: payload.Name, Type: normalizeMonitorDatasourceType(payload.Type), URL: strings.TrimRight(strings.TrimSpace(payload.URL), "/"),
			AuthType: normalizeMonitorAuthType(payload.AuthType), Username: payload.Username, Password: payload.Password, Token: payload.Token,
		}
	}
	startedAt := time.Now()
	err := s.checkMonitorDatasourceHealth(ds)
	if id > 0 {
		s.persistMonitorDatasourceHealth(id, err, time.Since(startedAt).Milliseconds())
	}
	return err
}

func (s *Service) checkMonitorDatasourceHealth(ds model.MonitorDatasource) error {
	switch normalizeMonitorDatasourceType(ds.Type) {
	case "elasticsearch":
		return s.elasticsearchHealth(ds)
	case "victorialogs":
		return s.victoriaLogsHealth(ds)
	case "jaeger":
		return s.jaegerHealth(ds)
	}
	_, err := s.prometheusQuery(ds, "up", time.Now())
	return err
}

func (s *Service) persistMonitorDatasourceHealth(id uint, healthErr error, latencyMs int64) {
	now := time.Now()
	updates := map[string]any{"last_check_at": &now, "latency_ms": latencyMs}
	if healthErr == nil {
		updates["health_status"] = "healthy"
		updates["last_success_at"] = &now
		updates["last_error"] = ""
		updates["consecutive_failures"] = 0
	} else {
		updates["health_status"] = "unhealthy"
		updates["last_error"] = healthErr.Error()
		updates["consecutive_failures"] = gorm.Expr("consecutive_failures + ?", 1)
	}
	_ = s.db.Model(&model.MonitorDatasource{}).Where("id = ?", id).Updates(updates).Error
}

func (s *Service) checkAllMonitorDatasources() {
	var datasources []model.MonitorDatasource
	if err := s.db.Where("status = ?", 1).Find(&datasources).Error; err != nil {
		return
	}
	for _, ds := range datasources {
		startedAt := time.Now()
		err := s.checkMonitorDatasourceHealth(ds)
		s.persistMonitorDatasourceHealth(ds.ID, err, time.Since(startedAt).Milliseconds())
	}
}
