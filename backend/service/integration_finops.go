package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"ops-admin/backend/model"

	"gorm.io/gorm"
)

var finOpsProviders = map[string]bool{"aws": true, "azure": true, "gcp": true, "alicloud": true, "tencent": true, "custom": true}

type FinOpsAccountPayload struct {
	ID                uint   `json:"id"`
	Name              string `json:"name"`
	Provider          string `json:"provider"`
	AccountIdentifier string `json:"accountIdentifier"`
	AccessKey         string `json:"accessKey"`
	SecretKey         string `json:"secretKey"`
	Region            string `json:"region"`
	Currency          string `json:"currency"`
	BillingEndpoint   string `json:"billingEndpoint"`
	BillingToken      string `json:"billingToken"`
	SyncEnabled       bool   `json:"syncEnabled"`
	SyncFrequency     string `json:"syncFrequency"`
	Status            int    `json:"status"`
	Description       string `json:"description"`
}

type FinOpsCostInput struct {
	ExternalID    string            `json:"externalId"`
	BillingDate   string            `json:"billingDate"`
	Service       string            `json:"service"`
	Region        string            `json:"region"`
	ResourceID    string            `json:"resourceId"`
	ResourceName  string            `json:"resourceName"`
	ResourceType  string            `json:"resourceType"`
	Tags          map[string]string `json:"tags"`
	Amount        float64           `json:"amount"`
	Currency      string            `json:"currency"`
	UsageQuantity float64           `json:"usageQuantity"`
	UsageUnit     string            `json:"usageUnit"`
}

type FinOpsCostImportPayload struct {
	AccountID uint              `json:"accountId"`
	Records   []FinOpsCostInput `json:"records"`
}

type FinOpsScheduler struct {
	service *Service
	stop    chan struct{}
	once    sync.Once
}

func (s *Service) initFinOpsScheduler() {
	s.finOpsSchedulerOnce.Do(func() {
		s.finOpsScheduler = &FinOpsScheduler{service: s, stop: make(chan struct{})}
		go s.finOpsScheduler.run()
	})
}

func (scheduler *FinOpsScheduler) run() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			now := time.Now()
			var accounts []model.IntegrationFinOpsAccount
			if err := scheduler.service.db.Where("sync_enabled = ? AND status = ? AND (next_sync_at IS NULL OR next_sync_at <= ?)", true, 1, now).Find(&accounts).Error; err != nil {
				continue
			}
			for _, account := range accounts {
				_, _ = scheduler.service.SyncFinOpsAccount(account.ID, "schedule")
			}
		case <-scheduler.stop:
			return
		}
	}
}

func (s *Service) ListFinOpsAccounts(keyword, provider string) ([]map[string]any, error) {
	query := s.db.Model(&model.IntegrationFinOpsAccount{})
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		query = query.Where("name LIKE ? OR account_identifier LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	if provider = strings.TrimSpace(provider); provider != "" {
		query = query.Where("provider = ?", provider)
	}
	var accounts []model.IntegrationFinOpsAccount
	if err := query.Order("id DESC").Find(&accounts).Error; err != nil {
		return nil, err
	}
	result := make([]map[string]any, 0, len(accounts))
	for _, a := range accounts {
		result = append(result, finOpsAccountView(a))
	}
	return result, nil
}

func finOpsAccountView(a model.IntegrationFinOpsAccount) map[string]any {
	return map[string]any{
		"id": a.ID, "name": a.Name, "provider": a.Provider, "accountIdentifier": a.AccountIdentifier,
		"region": a.Region, "currency": a.Currency, "billingEndpoint": a.BillingEndpoint,
		"syncEnabled": a.SyncEnabled, "syncFrequency": a.SyncFrequency, "status": a.Status,
		"lastSyncAt": a.LastSyncAt, "nextSyncAt": a.NextSyncAt, "description": a.Description,
		"hasAccessKey": a.AccessKey != "", "hasSecretKey": a.SecretKey != "", "hasBillingToken": a.BillingToken != "",
		"createTime": a.CreatedAt, "updateTime": a.UpdatedAt,
	}
}

func (s *Service) SaveFinOpsAccount(payload FinOpsAccountPayload) (map[string]any, error) {
	payload.Name = strings.TrimSpace(payload.Name)
	payload.Provider = strings.ToLower(strings.TrimSpace(payload.Provider))
	if payload.Name == "" || !finOpsProviders[payload.Provider] {
		return nil, errors.New("云账号名称和有效的云厂商不能为空")
	}
	if payload.SyncFrequency == "" {
		payload.SyncFrequency = "daily"
	}
	if !map[string]bool{"manual": true, "hourly": true, "daily": true, "weekly": true, "monthly": true}[payload.SyncFrequency] {
		return nil, errors.New("无效的账单同步频率")
	}
	var account model.IntegrationFinOpsAccount
	if payload.ID > 0 {
		if err := s.db.First(&account, payload.ID).Error; err != nil {
			return nil, err
		}
	}
	account.Name = payload.Name
	account.Provider = payload.Provider
	account.AccountIdentifier = strings.TrimSpace(payload.AccountIdentifier)
	if payload.AccessKey != "" {
		account.AccessKey = payload.AccessKey
	}
	if payload.SecretKey != "" {
		account.SecretKey = payload.SecretKey
	}
	if payload.BillingToken != "" {
		account.BillingToken = payload.BillingToken
	}
	account.Region = strings.TrimSpace(payload.Region)
	account.Currency = strings.ToUpper(strings.TrimSpace(payload.Currency))
	if account.Currency == "" {
		account.Currency = "CNY"
	}
	account.BillingEndpoint = strings.TrimSpace(payload.BillingEndpoint)
	account.SyncEnabled = payload.SyncEnabled
	account.SyncFrequency = payload.SyncFrequency
	account.Status = payload.Status
	if account.Status != 2 {
		account.Status = 1
	}
	account.Description = strings.TrimSpace(payload.Description)
	if account.SyncEnabled {
		next := nextFinOpsSync(time.Now(), account.SyncFrequency)
		account.NextSyncAt = &next
	} else {
		account.NextSyncAt = nil
	}
	if err := s.db.Save(&account).Error; err != nil {
		return nil, err
	}
	return finOpsAccountView(account), nil
}

func (s *Service) DeleteFinOpsAccount(id uint) error {
	if id == 0 {
		return errors.New("云账号 ID 不能为空")
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("account_id = ?", id).Delete(&model.IntegrationFinOpsCostRecord{}).Error; err != nil {
			return err
		}
		if err := tx.Where("account_id = ?", id).Delete(&model.IntegrationFinOpsRecommendation{}).Error; err != nil {
			return err
		}
		if err := tx.Where("account_id = ?", id).Delete(&model.IntegrationFinOpsSyncLog{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.IntegrationFinOpsAccount{}, id).Error
	})
}

func (s *Service) TestFinOpsAccount(payload FinOpsAccountPayload) (map[string]any, error) {
	account := model.IntegrationFinOpsAccount{Provider: strings.TrimSpace(payload.Provider), AccessKey: strings.TrimSpace(payload.AccessKey), SecretKey: strings.TrimSpace(payload.SecretKey), Region: strings.TrimSpace(payload.Region), BillingEndpoint: strings.TrimSpace(payload.BillingEndpoint), BillingToken: strings.TrimSpace(payload.BillingToken)}
	if payload.ID > 0 {
		var existing model.IntegrationFinOpsAccount
		if err := s.db.First(&existing, payload.ID).Error; err != nil {
			return nil, err
		}
		if account.Provider == "" {
			account.Provider = existing.Provider
		}
		if account.AccessKey == "" {
			account.AccessKey = existing.AccessKey
		}
		if account.SecretKey == "" {
			account.SecretKey = existing.SecretKey
		}
		if account.Region == "" {
			account.Region = existing.Region
		}
		if account.BillingEndpoint == "" {
			account.BillingEndpoint = existing.BillingEndpoint
		}
		if account.BillingToken == "" {
			account.BillingToken = existing.BillingToken
		}
	}
	start := time.Now()
	records, err := s.fetchFinOpsCosts(context.Background(), account, 1)
	if err != nil {
		return nil, err
	}
	return map[string]any{"connected": true, "latencyMs": time.Since(start).Milliseconds(), "recordCount": len(records), "source": finOpsBillingSource(account.Provider)}, nil
}

func (s *Service) ImportFinOpsCosts(payload FinOpsCostImportPayload) (map[string]any, error) {
	var account model.IntegrationFinOpsAccount
	if err := s.db.First(&account, payload.AccountID).Error; err != nil {
		return nil, errors.New("云账号不存在")
	}
	count, amount, err := s.upsertFinOpsCosts(account, payload.Records)
	if err != nil {
		return nil, err
	}
	return map[string]any{"recordCount": count, "totalAmount": amount}, nil
}

func (s *Service) upsertFinOpsCosts(account model.IntegrationFinOpsAccount, inputs []FinOpsCostInput) (int, float64, error) {
	count, total := 0, 0.0
	err := s.db.Transaction(func(tx *gorm.DB) error {
		for i, input := range inputs {
			date, err := parseFinOpsDate(input.BillingDate)
			if err != nil {
				return fmt.Errorf("第 %d 条账单日期无效: %w", i+1, err)
			}
			externalID := strings.TrimSpace(input.ExternalID)
			if externalID == "" {
				externalID = fmt.Sprintf("%s|%s|%s|%s|%.6f", date.Format("2006-01-02"), input.Service, input.ResourceID, input.UsageUnit, input.Amount)
			}
			tags, _ := json.Marshal(input.Tags)
			record := model.IntegrationFinOpsCostRecord{AccountID: account.ID, Provider: account.Provider, ExternalID: externalID,
				BillingDate: date, Service: input.Service, Region: input.Region, ResourceID: input.ResourceID,
				ResourceName: input.ResourceName, ResourceType: input.ResourceType, Tags: string(tags), Amount: input.Amount,
				Currency: strings.ToUpper(input.Currency), UsageQuantity: input.UsageQuantity, UsageUnit: input.UsageUnit}
			if record.Currency == "" {
				record.Currency = account.Currency
			}
			var existing model.IntegrationFinOpsCostRecord
			err = tx.Where("account_id = ? AND external_id = ?", account.ID, externalID).First(&existing).Error
			if err == nil {
				record.ID = existing.ID
				// Updating through Save would overwrite the original creation time with
				// Go's zero time, which MySQL rejects as 0000-00-00 in strict mode.
				if err := tx.Model(&existing).
					Select("*").
					Omit("id", "created_at").
					Updates(&record).Error; err != nil {
					return err
				}
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			} else {
				if err := tx.Create(&record).Error; err != nil {
					return err
				}
			}
			count++
			total += input.Amount
		}
		return nil
	})
	return count, total, err
}

func (s *Service) SyncFinOpsAccount(accountID uint, trigger string) (model.IntegrationFinOpsSyncLog, error) {
	var account model.IntegrationFinOpsAccount
	if err := s.db.First(&account, accountID).Error; err != nil {
		return model.IntegrationFinOpsSyncLog{}, err
	}
	now := time.Now()
	logEntry := model.IntegrationFinOpsSyncLog{AccountID: account.ID, Provider: account.Provider, TriggerType: trigger, Status: "running", StartedAt: now, CreatedAt: now}
	if err := s.db.Create(&logEntry).Error; err != nil {
		return logEntry, err
	}
	finish := func(status, message string, count int, amount float64) {
		now := time.Now()
		logEntry.Status, logEntry.Message, logEntry.RecordCount, logEntry.TotalAmount, logEntry.FinishedAt = status, message, count, amount, &now
		_ = s.db.Save(&logEntry).Error
		account.LastSyncAt = &now
		next := nextFinOpsSync(now, account.SyncFrequency)
		account.NextSyncAt = &next
		_ = s.db.Save(&account).Error
	}
	records, err := s.fetchFinOpsCosts(context.Background(), account, 10)
	if err != nil {
		finish("failed", err.Error(), 0, 0)
		return logEntry, err
	}
	count, amount, err := s.upsertFinOpsCosts(account, records)
	if err != nil {
		finish("failed", err.Error(), 0, 0)
		return logEntry, err
	}
	finish("success", finOpsBillingSource(account.Provider)+"账单同步完成", count, amount)
	return logEntry, nil
}

func (s *Service) FinOpsDashboard(start, end time.Time) (map[string]any, error) {
	records, accounts, err := s.finOpsRecords(start, end, 0)
	if err != nil {
		return nil, err
	}
	provider, daily := map[string]float64{}, map[string]float64{}
	total := 0.0
	for _, r := range records {
		total += r.Amount
		provider[r.Provider] += r.Amount
		daily[r.BillingDate.Format("2006-01-02")] += r.Amount
	}
	trend := make([]map[string]any, 0, len(daily))
	for d, v := range daily {
		trend = append(trend, map[string]any{"date": d, "amount": v})
	}
	sort.Slice(trend, func(i, j int) bool { return trend[i]["date"].(string) < trend[j]["date"].(string) })
	providerRows := mapToDimensionRows(provider)
	var saving float64
	s.db.Model(&model.IntegrationFinOpsRecommendation{}).Where("status = ?", "open").Select("COALESCE(SUM(saving),0)").Scan(&saving)
	return map[string]any{"totalCost": total, "accountCount": len(accounts), "recordCount": len(records), "estimatedSaving": saving, "trend": trend, "providerDistribution": providerRows}, nil
}

func (s *Service) FinOpsBreakdown(start, end time.Time, dimension string, accountID uint) ([]map[string]any, error) {
	records, accounts, err := s.finOpsRecords(start, end, accountID)
	if err != nil {
		return nil, err
	}
	accountNames := map[uint]string{}
	for _, a := range accounts {
		accountNames[a.ID] = a.Name
	}
	values := map[string]float64{}
	for _, r := range records {
		key := "未分类"
		switch dimension {
		case "provider":
			key = r.Provider
		case "account":
			key = accountNames[r.AccountID]
		case "region":
			key = r.Region
		case "service":
			key = r.Service
		case "resource":
			key = r.ResourceName
			if key == "" {
				key = r.ResourceID
			}
		case "tag":
			var tags map[string]string
			_ = json.Unmarshal([]byte(r.Tags), &tags)
			for k, v := range tags {
				values[k+"="+v] += r.Amount
			}
			continue
		}
		if key == "" {
			key = "未分类"
		}
		values[key] += r.Amount
	}
	return mapToDimensionRows(values), nil
}

func (s *Service) FinOpsResources(start, end time.Time, accountID uint) ([]map[string]any, error) {
	records, accounts, err := s.finOpsRecords(start, end, accountID)
	if err != nil {
		return nil, err
	}
	names := map[uint]string{}
	for _, a := range accounts {
		names[a.ID] = a.Name
	}
	type aggregate struct {
		Cost                                  float64
		Provider, Account, Region, Type, Name string
		Count                                 int
	}
	values := map[string]*aggregate{}
	for _, r := range records {
		key := r.ResourceID
		if key == "" {
			key = r.ResourceName
		}
		if key == "" {
			key = "未关联资源|" + r.Service
		}
		a := values[key]
		if a == nil {
			a = &aggregate{Provider: r.Provider, Account: names[r.AccountID], Region: r.Region, Type: r.ResourceType, Name: r.ResourceName}
			values[key] = a
		}
		a.Cost += r.Amount
		a.Count++
	}
	rows := make([]map[string]any, 0, len(values))
	for id, a := range values {
		rows = append(rows, map[string]any{"resourceId": id, "resourceName": a.Name, "resourceType": a.Type, "provider": a.Provider, "accountName": a.Account, "region": a.Region, "cost": a.Cost, "recordCount": a.Count})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i]["cost"].(float64) > rows[j]["cost"].(float64) })
	return rows, nil
}

func (s *Service) GenerateFinOpsRecommendations() (int, error) {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	records, _, err := s.finOpsRecords(start, now, 0)
	if err != nil {
		return 0, err
	}
	type agg struct {
		AccountID                  uint
		Provider, ResourceID, Name string
		Cost                       float64
	}
	m := map[string]*agg{}
	for _, r := range records {
		key := fmt.Sprintf("%d|%s", r.AccountID, r.ResourceID)
		if r.ResourceID == "" {
			key = fmt.Sprintf("%d|service:%s", r.AccountID, r.Service)
		}
		a := m[key]
		if a == nil {
			a = &agg{AccountID: r.AccountID, Provider: r.Provider, ResourceID: r.ResourceID, Name: r.ResourceName}
			m[key] = a
		}
		a.Cost += r.Amount
	}
	if err := s.db.Where("status = ?", "open").Delete(&model.IntegrationFinOpsRecommendation{}).Error; err != nil {
		return 0, err
	}
	count := 0
	for _, a := range m {
		if a.Cost <= 0 {
			continue
		}
		title := "高成本资源待核查"
		if a.Name != "" {
			title = "核查资源 " + a.Name + " 的使用率"
		}
		rec := model.IntegrationFinOpsRecommendation{AccountID: a.AccountID, Provider: a.Provider, Category: "cost_review", Priority: "P2", Title: title, Description: "该资源在本月产生了较高费用，建议结合监控用量核查规格、闲置时段和承诺折扣。", ResourceID: a.ResourceID, CurrentCost: a.Cost, Saving: a.Cost * 0.15, Status: "open"}
		if err := s.db.Create(&rec).Error; err != nil {
			return count, err
		}
		count++
		if count >= 20 {
			break
		}
	}
	return count, nil
}

func (s *Service) ListFinOpsRecommendations(status string) ([]model.IntegrationFinOpsRecommendation, error) {
	var rows []model.IntegrationFinOpsRecommendation
	q := s.db.Order("saving DESC,id DESC")
	if status != "" {
		q = q.Where("status = ?", status)
	}
	err := q.Find(&rows).Error
	return rows, err
}
func (s *Service) UpdateFinOpsRecommendation(id uint, status string) error {
	if !map[string]bool{"open": true, "accepted": true, "ignored": true, "done": true}[status] {
		return errors.New("无效的建议状态")
	}
	return s.db.Model(&model.IntegrationFinOpsRecommendation{}).Where("id = ?", id).Update("status", status).Error
}
func (s *Service) ListFinOpsSyncLogs(accountID uint) ([]model.IntegrationFinOpsSyncLog, error) {
	var rows []model.IntegrationFinOpsSyncLog
	q := s.db.Order("id DESC").Limit(200)
	if accountID > 0 {
		q = q.Where("account_id = ?", accountID)
	}
	err := q.Find(&rows).Error
	return rows, err
}

func (s *Service) finOpsRecords(start, end time.Time, accountID uint) ([]model.IntegrationFinOpsCostRecord, []model.IntegrationFinOpsAccount, error) {
	var records []model.IntegrationFinOpsCostRecord
	var accounts []model.IntegrationFinOpsAccount
	q := s.db.Where("billing_date >= ? AND billing_date < ?", start, end)
	if accountID > 0 {
		q = q.Where("account_id = ?", accountID)
	}
	if err := q.Find(&records).Error; err != nil {
		return nil, nil, err
	}
	if err := s.db.Find(&accounts).Error; err != nil {
		return nil, nil, err
	}
	return records, accounts, nil
}
func mapToDimensionRows(values map[string]float64) []map[string]any {
	total := 0.0
	for _, v := range values {
		total += v
	}
	rows := make([]map[string]any, 0, len(values))
	for name, v := range values {
		percent := 0.0
		if total > 0 {
			percent = v / total * 100
		}
		rows = append(rows, map[string]any{"name": name, "amount": v, "percent": percent})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i]["amount"].(float64) > rows[j]["amount"].(float64) })
	return rows
}
func parseFinOpsDate(value string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"} {
		if t, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, errors.New("支持 YYYY-MM-DD 或 RFC3339")
}
func nextFinOpsSync(now time.Time, frequency string) time.Time {
	switch frequency {
	case "hourly":
		return now.Add(time.Hour)
	case "weekly":
		return now.AddDate(0, 0, 7)
	case "monthly":
		return now.AddDate(0, 1, 0)
	case "manual":
		return now.AddDate(100, 0, 0)
	default:
		return now.AddDate(0, 0, 1)
	}
}
