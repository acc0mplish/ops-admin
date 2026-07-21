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

type FinOpsRecommendationGeneratePayload struct {
	ModelID   uint   `json:"modelId"`
	Strategy  string `json:"strategy"`
	AccountID uint   `json:"account_id"`
	Month     string `json:"month"`
}

type FinOpsMonthSyncResult struct {
	Month       string  `json:"month"`
	Status      string  `json:"status"`
	RecordCount int     `json:"recordCount"`
	TotalAmount float64 `json:"totalAmount"`
	Error       string  `json:"error,omitempty"`
}

type FinOpsSyncResult struct {
	AccountID   uint                    `json:"accountId"`
	Provider    string                  `json:"provider"`
	StartMonth  string                  `json:"startMonth"`
	EndMonth    string                  `json:"endMonth"`
	Status      string                  `json:"status"`
	RecordCount int                     `json:"recordCount"`
	TotalAmount float64                 `json:"totalAmount"`
	Months      []FinOpsMonthSyncResult `json:"months"`
}

type finOpsRecommendationAggregate struct {
	AccountID                  uint
	Provider, ResourceID, Name string
	Cost                       float64
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
	capability := finOpsProviderCapability(a.Provider)
	return map[string]any{
		"id": a.ID, "name": a.Name, "provider": a.Provider, "accountIdentifier": a.AccountIdentifier,
		"region": a.Region, "currency": a.Currency, "billingEndpoint": a.BillingEndpoint,
		"syncEnabled": a.SyncEnabled, "syncFrequency": a.SyncFrequency, "status": a.Status,
		"lastSyncAt": a.LastSyncAt, "nextSyncAt": a.NextSyncAt, "description": a.Description,
		"hasAccessKey": a.AccessKey != "", "hasSecretKey": a.SecretKey != "", "hasBillingToken": a.BillingToken != "",
		"createTime": a.CreatedAt, "updateTime": a.UpdatedAt,
		"billingCapability": capability,
	}
}

func finOpsProviderCapability(provider string) map[string]any {
	provider = strings.ToLower(strings.TrimSpace(provider))
	switch provider {
	case "alicloud":
		return map[string]any{"mode": "builtin", "label": "内置官方账单 API", "supportsOfficialSync": true}
	case "tencent":
		return map[string]any{"mode": "builtin", "label": "内置官方账单 API", "supportsOfficialSync": true}
	case "aws", "azure", "gcp":
		return map[string]any{"mode": "adapter", "label": "账单适配器", "supportsOfficialSync": false}
	default:
		return map[string]any{"mode": "adapter", "label": "自定义账单适配器", "supportsOfficialSync": false}
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

// SyncFinOpsAccount synchronizes the current month and the preceding five natural
// months. It is kept for scheduled callers; API callers can supply a range via
// SyncFinOpsAccountMonths.
func (s *Service) SyncFinOpsAccount(accountID uint, trigger string) (FinOpsSyncResult, error) {
	now := time.Now()
	return s.SyncFinOpsAccountMonths(accountID, trigger, now.AddDate(0, -5, 0).Format("2006-01"), now.Format("2006-01"))
}

func (s *Service) SyncFinOpsAccountMonths(accountID uint, trigger, startMonth, endMonth string) (FinOpsSyncResult, error) {
	var account model.IntegrationFinOpsAccount
	if err := s.db.First(&account, accountID).Error; err != nil {
		return FinOpsSyncResult{}, err
	}
	start, err := parseFinOpsMonth(startMonth)
	if err != nil {
		return FinOpsSyncResult{}, err
	}
	end, err := parseFinOpsMonth(endMonth)
	if err != nil {
		return FinOpsSyncResult{}, err
	}
	if start.After(end) {
		return FinOpsSyncResult{}, errors.New("start_month cannot be after end_month")
	}

	result := FinOpsSyncResult{AccountID: account.ID, Provider: account.Provider, StartMonth: start.Format("2006-01"), EndMonth: end.Format("2006-01")}
	for month := start; !month.After(end); month = month.AddDate(0, 1, 0) {
		monthly := s.syncFinOpsAccountMonth(account, trigger, month)
		result.Months = append(result.Months, monthly)
		result.RecordCount += monthly.RecordCount
		result.TotalAmount += monthly.TotalAmount
	}
	failed := 0
	for _, monthly := range result.Months {
		if monthly.Status != "success" {
			failed++
		}
	}
	switch {
	case failed == 0:
		result.Status = "success"
	case failed == len(result.Months):
		result.Status = "failed"
	default:
		result.Status = "partial_failed"
	}
	now := time.Now()
	account.LastSyncAt = &now
	next := nextFinOpsSync(now, account.SyncFrequency)
	account.NextSyncAt = &next
	_ = s.db.Save(&account).Error
	return result, nil
}

func (s *Service) syncFinOpsAccountMonth(account model.IntegrationFinOpsAccount, trigger string, month time.Time) FinOpsMonthSyncResult {
	monthText := month.Format("2006-01")
	now := time.Now()
	logEntry := model.IntegrationFinOpsSyncLog{AccountID: account.ID, Provider: account.Provider, TriggerType: trigger, BillingMonth: monthText, Status: "running", StartedAt: now, CreatedAt: now}
	result := FinOpsMonthSyncResult{Month: monthText, Status: "failed"}
	if err := s.db.Create(&logEntry).Error; err != nil {
		result.Error = err.Error()
		return result
	}
	finish := func(status, message string, count int, amount float64) {
		finished := time.Now()
		logEntry.Status, logEntry.Message, logEntry.RecordCount, logEntry.TotalAmount, logEntry.FinishedAt = status, message, count, amount, &finished
		_ = s.db.Save(&logEntry).Error
	}
	records, err := s.fetchFinOpsCostsForMonth(context.Background(), account, monthText, 10)
	if err != nil {
		result.Error = err.Error()
		finish("failed", result.Error, 0, 0)
		return result
	}
	if len(records) == 0 {
		// An empty provider response can be a legitimate zero-cost month, but it
		// must never erase a previously successful snapshot.  Keep old data and
		// make the condition explicit in the execution history.
		result.Status = "success"
		finish("success", finOpsBillingSource(account.Provider)+"账单同步完成 "+monthText+"（未返回账单明细，未覆盖历史数据）", 0, 0)
		return result
	}
	count, amount, err := s.upsertFinOpsCosts(account, records)
	if err != nil {
		result.Error = err.Error()
		finish("failed", result.Error, 0, 0)
		return result
	}
	if err := s.removeStaleFinOpsCostsForMonth(account.ID, month, records); err != nil {
		result.Error = err.Error()
		finish("failed", result.Error, 0, 0)
		return result
	}
	result.Status, result.RecordCount, result.TotalAmount = "success", count, amount
	finish("success", finOpsBillingSource(account.Provider)+"账单同步完成 "+monthText, count, amount)
	return result
}

// removeStaleFinOpsCostsForMonth keeps a monthly sync as a complete snapshot.
// It is intentionally called only after all rows for that month were upserted.
func (s *Service) removeStaleFinOpsCostsForMonth(accountID uint, month time.Time, inputs []FinOpsCostInput) error {
	start := time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, month.Location())
	end := start.AddDate(0, 1, 0)
	ids := make([]string, 0, len(inputs))
	for _, input := range inputs {
		if id := strings.TrimSpace(input.ExternalID); id != "" {
			ids = append(ids, id)
		}
	}
	query := s.db.Where("account_id = ? AND billing_date >= ? AND billing_date < ?", accountID, start, end)
	if len(ids) > 0 {
		query = query.Where("external_id NOT IN ?", ids)
	}
	return query.Delete(&model.IntegrationFinOpsCostRecord{}).Error
}

func (s *Service) FinOpsDashboard(start, end time.Time, accountID uint) (map[string]any, error) {
	records, accounts, err := s.finOpsRecords(start, end, accountID)
	if err != nil {
		return nil, err
	}
	provider, daily := map[string]float64{}, map[string]float64{}
	estimatedRecordCount := 0
	monthlyRecordCount := 0
	total := 0.0
	for _, r := range records {
		total += r.Amount
		provider[r.Provider] += r.Amount
		daily[r.BillingDate.Format("2006-01-02")] += r.Amount
		var tags map[string]string
		_ = json.Unmarshal([]byte(r.Tags), &tags)
		if tags["granularity"] == "daily_estimate" {
			estimatedRecordCount++
		} else if tags["billingCycle"] != "" {
			monthlyRecordCount++
		}
	}
	trend := make([]map[string]any, 0, len(daily))
	for d, v := range daily {
		trend = append(trend, map[string]any{"date": d, "amount": v})
	}
	sort.Slice(trend, func(i, j int) bool { return trend[i]["date"].(string) < trend[j]["date"].(string) })
	providerRows := mapToDimensionRows(provider)
	var saving float64
	savingQuery := s.db.Model(&model.IntegrationFinOpsRecommendation{}).
		Where("status = ?", "open").
		Where("analysis_month >= ? AND analysis_month <= ?", start.Format("2006-01"), end.Add(-time.Nanosecond).Format("2006-01"))
	if accountID > 0 {
		savingQuery = savingQuery.Where("analysis_account_id = ?", accountID)
	}
	savingQuery.Select("COALESCE(SUM(saving),0)").Scan(&saving)
	var latestSyncAt *time.Time
	for _, account := range accounts {
		if account.LastSyncAt != nil && (latestSyncAt == nil || account.LastSyncAt.After(*latestSyncAt)) {
			latestSyncAt = account.LastSyncAt
		}
	}
	return map[string]any{"totalCost": total, "accountCount": len(accounts), "recordCount": len(records), "estimatedRecordCount": estimatedRecordCount, "monthlyRecordCount": monthlyRecordCount, "exactRecordCount": len(records) - estimatedRecordCount - monthlyRecordCount, "estimatedSaving": saving, "trend": trend, "providerDistribution": providerRows, "latestSyncAt": latestSyncAt}, nil
}

func (s *Service) FinOpsBreakdown(start, end time.Time, dimension string, accountID uint) ([]map[string]any, error) {
	records, accounts, err := s.finOpsRecords(start, end, accountID)
	if err != nil {
		return nil, err
	}
	if dimension == "detail" {
		total := 0.0
		for _, r := range records {
			total += r.Amount
		}
		rows := make([]map[string]any, 0, len(records))
		for _, r := range records {
			percent := 0.0
			if total != 0 {
				percent = r.Amount / total * 100
			}
			instance := r.ResourceName
			if instance == "" {
				instance = r.ResourceID
			}
			rows = append(rows, map[string]any{
				"billingDate": r.BillingDate.Format("2006-01-02"), "service": r.Service,
				"region": r.Region, "resourceId": r.ResourceID, "resourceName": instance,
				"amount": r.Amount, "currency": r.Currency, "percent": percent, "dataQuality": finOpsRecordDataQuality(r.Tags),
			})
		}
		sort.Slice(rows, func(i, j int) bool {
			return rows[i]["billingDate"].(string) > rows[j]["billingDate"].(string)
		})
		return rows, nil
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

func (s *Service) LatestFinOpsBreakdownMonth(accountID uint) (string, error) {
	var record model.IntegrationFinOpsCostRecord
	query := s.db.Order("billing_date DESC")
	if accountID > 0 {
		query = query.Where("account_id = ?", accountID)
	}
	if err := query.First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil
		}
		return "", err
	}
	return record.BillingDate.Format("2006-01"), nil
}

func (s *Service) FinOpsResources(start, end time.Time, accountID uint, regions, resourceTypes []string) (map[string]any, error) {
	records, accounts, err := s.finOpsRecords(start, end, accountID)
	if err != nil {
		return nil, err
	}
	regionSet, typeSet := map[string]bool{}, map[string]bool{}
	for _, value := range regions {
		regionSet[strings.TrimSpace(value)] = true
	}
	for _, value := range resourceTypes {
		typeSet[strings.TrimSpace(value)] = true
	}
	allRegions, allTypes := map[string]bool{}, map[string]bool{}
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
		if r.Region != "" {
			allRegions[r.Region] = true
		}
		if r.ResourceType != "" {
			allTypes[r.ResourceType] = true
		}
		if len(regionSet) > 0 && !regionSet[r.Region] {
			continue
		}
		if len(typeSet) > 0 && !typeSet[r.ResourceType] {
			continue
		}
		key := r.ResourceID
		if key == "" {
			key = r.ResourceName
		}
		if key == "" {
			key = "未关联资源|" + r.Service
		}
		key = fmt.Sprintf("%d|%s|%s", r.AccountID, key, r.ResourceType)
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
	regionOptions, typeOptions := make([]string, 0, len(allRegions)), make([]string, 0, len(allTypes))
	for value := range allRegions {
		regionOptions = append(regionOptions, value)
	}
	for value := range allTypes {
		typeOptions = append(typeOptions, value)
	}
	sort.Strings(regionOptions)
	sort.Strings(typeOptions)
	return map[string]any{"items": rows, "regions": regionOptions, "resourceTypes": typeOptions}, nil
}

func (s *Service) GenerateFinOpsRecommendations(modelID uint, strategy string, accountID uint, analysisMonth string) (int, string, error) {
	now := time.Now()
	if strings.TrimSpace(analysisMonth) == "" {
		analysisMonth = now.Format("2006-01")
	}
	start, err := parseFinOpsMonth(analysisMonth)
	if err != nil {
		return 0, "", errors.New("month 参数格式无效，格式为 YYYY-MM")
	}
	end := start.AddDate(0, 1, 0)
	if now.After(start) && now.Before(end) {
		end = now
	}
	records, accounts, err := s.finOpsRecords(start, end, accountID)
	if err != nil {
		return 0, "", err
	}
	m := map[string]*finOpsRecommendationAggregate{}
	for _, r := range records {
		key := fmt.Sprintf("%d|%s", r.AccountID, r.ResourceID)
		if r.ResourceID == "" {
			key = fmt.Sprintf("%d|service:%s", r.AccountID, r.Service)
		}
		a := m[key]
		if a == nil {
			a = &finOpsRecommendationAggregate{AccountID: r.AccountID, Provider: r.Provider, ResourceID: r.ResourceID, Name: r.ResourceName}
			m[key] = a
		}
		a.Cost += r.Amount
	}
	strategy = strings.TrimSpace(strategy)
	if strategy == "" {
		strategy = "default"
	}
	if strategy != "default" && strategy != "ai" {
		return 0, "", errors.New("无效的建议生成策略")
	}
	analysisScope := "全部云账号"
	if accountID > 0 {
		for _, account := range accounts {
			if account.ID == accountID {
				analysisScope = account.Name
				break
			}
		}
	}
	var aiModel *model.IntegrationAIModel
	if strategy == "ai" {
		aiModel, err = s.finOpsRecommendationAIModel(modelID)
	} else {
		aiModel, err = nil, nil
	}
	if err != nil {
		return 0, "", err
	}
	var recommendations []model.IntegrationFinOpsRecommendation
	mode := "default"
	if aiModel != nil {
		// Reuse the AI assistant's read-only FinOps tool as the single source of
		// aggregate billing context. This reads only synchronized local records.
		analysisData, analysisErr := s.queryAIFinOpsAnalysis(map[string]any{
			"accountId": accountID, "month": analysisMonth, "trendMonths": 6,
		})
		if analysisErr != nil {
			return 0, "", analysisErr
		}
		recommendations, err = s.generateFinOpsAIRecommendations(*aiModel, m, analysisData)
		if err != nil {
			// A model may occasionally ignore the JSON-only instruction. The local
			// billing analysis is still valid, so return a deterministic report
			// instead of failing the whole user request.
			recommendations = defaultFinOpsRecommendations(m)
			mode = "ai_fallback"
		} else {
			mode = "ai"
		}
	} else {
		recommendations = defaultFinOpsRecommendations(m)
	}
	for _, rec := range recommendations {
		rec.Strategy = mode
		rec.AnalysisMonth = analysisMonth
		rec.AnalysisAccountID = accountID
		rec.Title = finOpsRecommendationTitle(analysisScope, analysisMonth, mode)
		// A recommendation is an account-and-billing-period analysis report, not a resource record.
		rec.ResourceID = ""
		if aiModel != nil && mode == "ai" {
			rec.ModelName = aiModel.Name
		}
		if err := s.db.Create(&rec).Error; err != nil {
			return 0, "", err
		}
	}
	return len(recommendations), mode, nil
}

func finOpsRecommendationTitle(scope, analysisMonth, strategy string) string {
	if strings.TrimSpace(scope) == "" {
		scope = "全部云账号"
	}
	if strings.TrimSpace(analysisMonth) == "" {
		analysisMonth = time.Now().Format("2006-01")
	}
	strategyName := "默认策略"
	if strategy == "ai" {
		strategyName = "AI 分析"
	} else if strategy == "ai_fallback" {
		strategyName = "AI 分析降级（默认策略）"
	}
	return fmt.Sprintf("%s｜%s｜%s优化建议", scope, analysisMonth, strategyName)
}

func defaultFinOpsRecommendations(values map[string]*finOpsRecommendationAggregate) []model.IntegrationFinOpsRecommendation {
	items := make([]*finOpsRecommendationAggregate, 0, len(values))
	for _, item := range values {
		if item.Cost > 0 {
			items = append(items, item)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Cost > items[j].Cost })
	if len(items) == 0 {
		return nil
	}
	var description strings.Builder
	description.WriteString("## 执行摘要\n以下为基于本月账单的默认 FinOps 分析结果。账单不包含 CPU、内存和连接数等实时监控指标，因此空闲与低利用率项属于待核查对象。")
	total, saving := 0.0, 0.0
	for index, item := range items {
		total += item.Cost
		if index >= 10 {
			continue
		}
		recommendationSaving := item.Cost * 0.15
		saving += recommendationSaving
		name := item.Name
		if name == "" {
			name = item.ResourceID
		}
		if name == "" {
			name = "服务资源"
		}
		description.WriteString(fmt.Sprintf("\n%d. %s：本月成本 %.2f，建议核查使用率、闲置时段与承诺折扣；预计可节省 %.2f。", index+1, name, item.Cost, recommendationSaving))
	}
	description.WriteString("\n\n## 空闲资源\n优先核查本月仍产生费用、但无对应运行负载或业务访问的资源；确认后可停止、释放或设置定时启停。")
	description.WriteString("\n\n## 低利用率资源\n对上述高成本计算、数据库和中间件资源核对 CPU、内存、连接数和 IOPS；连续低利用率时考虑降配。")
	description.WriteString("\n\n## 计费方式优化\n对稳定运行资源评估包年包月、节省计划或预留实例，并避免重复购买资源包。")
	description.WriteString("\n\n## 闲置磁盘/快照/IP\n盘点未挂载云盘、长期快照、未绑定 EIP 和闲置负载均衡；确认无依赖后清理。")
	description.WriteString(fmt.Sprintf("\n\n## 预计可节省金额\n本月纳入分析成本 %.2f；按保守 15%% 估算，预计月节省金额 %.2f。", total, saving))
	return []model.IntegrationFinOpsRecommendation{{Provider: "multi-cloud", Category: "cost_review", Priority: "P2", Title: "本月云费用优化建议", Description: description.String(), CurrentCost: total, Saving: saving, Status: "open"}}
}

func (s *Service) finOpsRecommendationAIModel(modelID uint) (*model.IntegrationAIModel, error) {
	var item model.IntegrationAIModel
	query := s.db.Where("status = ?", 1)
	if modelID > 0 {
		query = query.Where("id = ?", modelID)
	} else {
		query = query.Order("is_default DESC, id ASC")
	}
	if err := query.First(&item).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Service) generateFinOpsAIRecommendations(aiModel model.IntegrationAIModel, values map[string]*finOpsRecommendationAggregate, analysisData map[string]any) ([]model.IntegrationFinOpsRecommendation, error) {
	// Keep all numeric facts deterministic. AI is asked only for the narrative
	// analysis, so a conversational model never needs to serialize cost data.
	base := defaultFinOpsRecommendations(values)
	if len(base) == 0 {
		return nil, errors.New("没有可用于生成 AI 建议的费用记录")
	}
	contextJSON, _ := json.Marshal(map[string]any{"finopsToolResult": analysisData})
	prompt := "根据以下本地已同步云账单数据，生成简洁、可执行的中文 FinOps 优化分析。不要调用云接口，也不要编造资源、金额或监控指标。请使用 Markdown 标题，覆盖：执行摘要、空闲资源核查、低利用率资源核查、计费方式优化、闲置磁盘/快照/IP、预计可节省金额。没有实时监控数据时，必须写“建议核查”。不要输出 JSON、代码块或表格；总长度不超过 900 个中文字符。数据：" + string(contextJSON)
	response, err := s.callOpenAICompatible(aiModel, []map[string]any{{"role": "system", "content": "你是严谨的 FinOps 分析师。只输出简洁中文 Markdown 分析报告，不输出 JSON。"}, {"role": "user", "content": prompt}}, nil)
	if err != nil {
		return nil, err
	}
	content := strings.TrimSpace(response.Content)
	if content == "" || hasUnsupportedAIToolProtocol(content) {
		return nil, errors.New("AI 未返回可展示的 FinOps 分析内容")
	}
	recommendation := base[0]
	recommendation.Category = "ai_finops"
	recommendation.Title = "本月云费用 AI 优化建议"
	recommendation.Description = "## AI 分析结论\n" + truncateRunes(content, 12000)
	return []model.IntegrationFinOpsRecommendation{recommendation}, nil

	/*
		items := make([]map[string]any, 0, len(values))
		for _, value := range values {
			if value.Cost > 0 {
				items = append(items, map[string]any{"accountId": value.AccountID, "provider": value.Provider, "resourceId": value.ResourceID, "resourceName": value.Name, "currentCost": value.Cost})
			}
		}
		sort.Slice(items, func(i, j int) bool { return items[i]["currentCost"].(float64) > items[j]["currentCost"].(float64) })
		if len(items) > 30 {
			items = items[:30]
		}
		contextJSON, _ := json.Marshal(map[string]any{"finopsToolResult": analysisData, "costItems": items})
		prompt := "根据以下云费用分析工具结果生成不超过 5 条可执行优化建议。工具结果只来自本地已同步账单数据库，绝不代表实时云端数据。必须覆盖：空闲资源、低利用率资源、计费方式优化、闲置磁盘/快照/IP、预计可节省金额。账单没有实时监控指标时，明确说明需要核查而不得断言资源闲置。每条 description 不超过 80 个中文字符。只返回一个完整 JSON 对象：{\"recommendations\":[{\"accountId\":1,\"provider\":\"...\",\"resourceId\":\"...\",\"priority\":\"P1|P2|P3\",\"title\":\"...\",\"description\":\"...\",\"currentCost\":0,\"saving\":0}]}。不要使用 Markdown 代码块、标题或任何 JSON 之外的文字。不得编造资源；saving 必须非负且不超过 currentCost。数据：" + string(contextJSON)
		response, err := s.callOpenAICompatibleJSON(aiModel, []map[string]any{{"role": "system", "content": "你是严谨的 FinOps 分析师，只输出符合要求的 JSON。"}, {"role": "user", "content": prompt}})
		if err != nil {
			return nil, err
		}
		var payload struct {
			Recommendations []struct {
				AccountID   uint    `json:"accountId"`
				Provider    string  `json:"provider"`
				ResourceID  string  `json:"resourceId"`
				Priority    string  `json:"priority"`
				Title       string  `json:"title"`
				Description string  `json:"description"`
				CurrentCost float64 `json:"currentCost"`
				Saving      float64 `json:"saving"`
			} `json:"recommendations"`
		}
		content, parseErr := extractFinOpsRecommendationJSON(response.Content)
		if parseErr != nil {
			repairPrompt := "将以下 FinOps 分析结果转换为一个完整 JSON 对象。只返回 JSON，不要 Markdown 或解释。格式必须是 {\"recommendations\":[{\"accountId\":1,\"provider\":\"...\",\"resourceId\":\"...\",\"priority\":\"P1|P2|P3\",\"title\":\"...\",\"description\":\"...\",\"currentCost\":0,\"saving\":0}]}。若原内容缺少字段，使用已知账号和成本的保守值，不得编造资源。原内容：\n" + truncateRunes(response.Content, 12000)
			repaired, repairErr := s.callOpenAICompatibleJSON(aiModel, []map[string]any{{"role": "system", "content": "你是严格的 JSON 修复器，只输出一个有效 JSON 对象。"}, {"role": "user", "content": repairPrompt}})
			if repairErr == nil {
				content, parseErr = extractFinOpsRecommendationJSON(repaired.Content)
			}
			if parseErr != nil {
				return nil, parseErr
			}
		}
		if err := json.Unmarshal([]byte(content), &payload); err != nil {
			return nil, fmt.Errorf("AI 返回的建议 JSON 无法解析: %w", err)
		}
		fallbackAccountID, fallbackProvider := uint(0), "multi-cloud"
		for _, value := range values {
			if value.AccountID > 0 {
				fallbackAccountID = value.AccountID
				if strings.TrimSpace(value.Provider) != "" {
					fallbackProvider = value.Provider
				}
				break
			}
		}
		analysis := make([]model.IntegrationFinOpsRecommendation, 0, len(payload.Recommendations))
		for _, item := range payload.Recommendations {
			if item.AccountID == 0 {
				item.AccountID = fallbackAccountID
			}
			if strings.TrimSpace(item.Provider) == "" {
				item.Provider = fallbackProvider
			}
			if item.AccountID == 0 || strings.TrimSpace(item.Title) == "" || item.CurrentCost < 0 {
				continue
			}
			if item.Saving < 0 {
				item.Saving = 0
			}
			if item.Saving > item.CurrentCost {
				item.Saving = item.CurrentCost
			}
			priority := item.Priority
			if !map[string]bool{"P1": true, "P2": true, "P3": true}[priority] {
				priority = "P2"
			}
			analysis = append(analysis, model.IntegrationFinOpsRecommendation{AccountID: item.AccountID, Provider: strings.TrimSpace(item.Provider), Category: "ai_finops", Priority: priority, Title: strings.TrimSpace(item.Title), Description: strings.TrimSpace(item.Description), ResourceID: strings.TrimSpace(item.ResourceID), CurrentCost: item.CurrentCost, Saving: item.Saving, Status: "open"})
			if len(analysis) >= 20 {
				break
			}
		}
		if len(analysis) == 0 {
			return nil, errors.New("AI 未生成有效的优化建议")
		}
		priority, total, saving := "P3", 0.0, 0.0
		var description strings.Builder
		description.WriteString("## 执行摘要\n以下为 AI 基于本月账单生成的综合优化建议：")
		for index, item := range analysis {
			total += item.CurrentCost
			saving += item.Saving
			if item.Priority == "P1" || (item.Priority == "P2" && priority == "P3") {
				priority = item.Priority
			}
			description.WriteString(fmt.Sprintf("\n%d. %s：%s（当前成本 %.2f，预计节省 %.2f）", index+1, item.Title, item.Description, item.CurrentCost, item.Saving))
		}
		description.WriteString("\n\n## 空闲资源\n请优先验证停止但仍计费、无业务访问或无监控负载的资源。")
		description.WriteString("\n\n## 低利用率资源\n结合 CPU、内存、IOPS、连接数等监控数据确认是否应降配。")
		description.WriteString("\n\n## 计费方式优化\n评估稳定工作负载是否适合包年包月、节省计划或预留实例。")
		description.WriteString("\n\n## 闲置磁盘/快照/IP\n清点未挂载磁盘、长期快照、未绑定 IP 和无后端服务的负载均衡。")
		description.WriteString(fmt.Sprintf("\n\n## 预计可节省金额\n本报告覆盖成本 %.2f，AI 估算可节省金额 %.2f。", total, saving))
		return []model.IntegrationFinOpsRecommendation{{Provider: "multi-cloud", Category: "ai_finops", Priority: priority, Title: "本月云费用 AI 优化建议", Description: description.String(), CurrentCost: total, Saving: saving, Status: "open"}}, nil
	*/
}

// extractFinOpsRecommendationJSON accepts OpenAI-compatible models that wrap
// their JSON in a Markdown fence or add a short preface/suffix. It deliberately
// extracts one balanced JSON object instead of accepting arbitrary text.
func extractFinOpsRecommendationJSON(content string) (string, error) {
	content = strings.TrimSpace(strings.TrimPrefix(content, "\ufeff"))
	for start := strings.IndexByte(content, '{'); start >= 0; {
		depth := 0
		inString := false
		escaped := false
		for index := start; index < len(content); index++ {
			char := content[index]
			if inString {
				if escaped {
					escaped = false
					continue
				}
				if char == '\\' {
					escaped = true
				} else if char == '"' {
					inString = false
				}
				continue
			}
			switch char {
			case '"':
				inString = true
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					candidate := content[start : index+1]
					var document json.RawMessage
					if json.Unmarshal([]byte(candidate), &document) == nil {
						return candidate, nil
					}
					break
				}
			}
		}
		next := strings.IndexByte(content[start+1:], '{')
		if next < 0 {
			break
		}
		start += next + 1
	}
	return "", errors.New("AI 未按约定返回完整 JSON，可能包含说明文字或因输出过长被截断")
}

func (s *Service) ListFinOpsRecommendations(status string, accountID uint, analysisMonth string) ([]model.IntegrationFinOpsRecommendation, error) {
	var rows []model.IntegrationFinOpsRecommendation
	q := s.db.Order("saving DESC,id DESC")
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if accountID > 0 {
		q = q.Where("analysis_account_id = ?", accountID)
	}
	if strings.TrimSpace(analysisMonth) != "" {
		q = q.Where("analysis_month = ?", analysisMonth)
	}
	err := q.Find(&rows).Error
	return rows, err
}

func finOpsRecordDataQuality(tagsJSON string) string {
	var tags map[string]string
	_ = json.Unmarshal([]byte(tagsJSON), &tags)
	if tags["granularity"] == "daily_estimate" {
		return "estimated"
	}
	if tags["billingCycle"] != "" {
		return "monthly"
	}
	return "exact"
}
func (s *Service) UpdateFinOpsRecommendation(id uint, status string) error {
	if !map[string]bool{"open": true, "accepted": true, "ignored": true, "done": true}[status] {
		return errors.New("无效的建议状态")
	}
	return s.db.Model(&model.IntegrationFinOpsRecommendation{}).Where("id = ?", id).Update("status", status).Error
}

func (s *Service) DeleteFinOpsRecommendation(id uint) error {
	if id == 0 {
		return errors.New("建议 ID 不能为空")
	}
	return s.db.Delete(&model.IntegrationFinOpsRecommendation{}, id).Error
}
func (s *Service) ListFinOpsSyncLogs(accountID uint) ([]map[string]any, error) {
	var rows []model.IntegrationFinOpsSyncLog
	q := s.db.Order("id DESC").Limit(200)
	if accountID > 0 {
		q = q.Where("account_id = ?", accountID)
	}
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	var accounts []model.IntegrationFinOpsAccount
	accountQuery := s.db
	if accountID > 0 {
		accountQuery = accountQuery.Where("id = ?", accountID)
	}
	if err := accountQuery.Find(&accounts).Error; err != nil {
		return nil, err
	}
	accountNames := make(map[uint]string, len(accounts))
	for _, account := range accounts {
		accountNames[account.ID] = account.Name
	}
	triggerLabels := map[string]string{"manual": "手动同步", "scheduled": "定时同步", "api": "接口触发"}
	result := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		finishedAt := row.FinishedAt
		durationSeconds := 0
		if finishedAt != nil {
			durationSeconds = int(finishedAt.Sub(row.StartedAt).Seconds())
			if durationSeconds < 0 {
				durationSeconds = 0
			}
		}
		month := row.BillingMonth
		if month == "" {
			month = finOpsMonthFromText(row.Message)
		}
		trigger := triggerLabels[row.TriggerType]
		if trigger == "" {
			trigger = row.TriggerType
		}
		result = append(result, map[string]any{
			"id": row.ID, "accountId": row.AccountID, "accountName": accountNames[row.AccountID], "provider": row.Provider,
			"trigger": trigger, "triggerType": row.TriggerType, "billingMonth": month, "status": row.Status,
			"recordCount": row.RecordCount, "totalAmount": row.TotalAmount, "message": row.Message,
			"startedAt": row.StartedAt, "finishedAt": finishedAt, "durationSeconds": durationSeconds,
		})
	}
	return result, nil
}

func finOpsMonthFromText(value string) string {
	for i := 0; i+7 <= len(value); i++ {
		part := value[i : i+7]
		if len(part) == 7 && part[4] == '-' && part[0] >= '0' && part[0] <= '9' && part[1] >= '0' && part[1] <= '9' && part[2] >= '0' && part[2] <= '9' && part[3] >= '0' && part[3] <= '9' && part[5] >= '0' && part[5] <= '9' && part[6] >= '0' && part[6] <= '9' {
			return part
		}
	}
	return ""
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
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02", "2006-01"} {
		if t, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, errors.New("支持 YYYY-MM-DD 或 RFC3339")
}

func parseFinOpsMonth(value string) (time.Time, error) {
	month, err := time.ParseInLocation("2006-01", strings.TrimSpace(value), time.Local)
	if err != nil {
		return time.Time{}, errors.New("month must use YYYY-MM")
	}
	return month, nil
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
