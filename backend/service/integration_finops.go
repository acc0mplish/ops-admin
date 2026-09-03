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
	ExternalID     string            `json:"externalId"`
	BillingDate    string            `json:"billingDate"`
	Service        string            `json:"service"`
	Region         string            `json:"region"`
	ResourceID     string            `json:"resourceId"`
	ResourceName   string            `json:"resourceName"`
	ResourceType   string            `json:"resourceType"`
	ResourceConfig string            `json:"resourceConfig"`
	Tags           map[string]string `json:"tags"`
	Amount         float64           `json:"amount"`
	OriginalPrice  float64           `json:"originalPrice"`
	Discount       float64           `json:"discount"`
	ActualPayment  float64           `json:"actualPayment"`
	Currency       string            `json:"currency"`
	UsageQuantity  float64           `json:"usageQuantity"`
	UsageUnit      string            `json:"usageUnit"`
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
	Month             string  `json:"month"`
	Status            string  `json:"status"`
	SourceRecordCount int     `json:"sourceRecordCount"`
	SourceTotalAmount float64 `json:"sourceTotalAmount"`
	RecordCount       int     `json:"recordCount"`
	TotalAmount       float64 `json:"totalAmount"`
	DeduplicatedCount int     `json:"deduplicatedCount"`
	SnapshotVerified  bool    `json:"snapshotVerified"`
	Error             string  `json:"error,omitempty"`
}

type FinOpsSyncResult struct {
	AccountID         uint                    `json:"accountId"`
	Provider          string                  `json:"provider"`
	StartMonth        string                  `json:"startMonth"`
	EndMonth          string                  `json:"endMonth"`
	Status            string                  `json:"status"`
	SourceRecordCount int                     `json:"sourceRecordCount"`
	SourceTotalAmount float64                 `json:"sourceTotalAmount"`
	RecordCount       int                     `json:"recordCount"`
	TotalAmount       float64                 `json:"totalAmount"`
	DeduplicatedCount int                     `json:"deduplicatedCount"`
	Months            []FinOpsMonthSyncResult `json:"months"`
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
		return map[string]any{"mode": "builtin", "label": "Built-in Official Billing API", "supportsOfficialSync": true}
	case "tencent":
		return map[string]any{"mode": "builtin", "label": "Built-in Official Billing API", "supportsOfficialSync": true}
	case "aws", "azure", "gcp":
		return map[string]any{"mode": "adapter", "label": "Billing Adapter", "supportsOfficialSync": false}
	default:
		return map[string]any{"mode": "adapter", "label": "Custom Billing Adapter", "supportsOfficialSync": false}
	}
}

func (s *Service) SaveFinOpsAccount(payload FinOpsAccountPayload) (map[string]any, error) {
	payload.Name = strings.TrimSpace(payload.Name)
	payload.Provider = strings.ToLower(strings.TrimSpace(payload.Provider))
	if payload.Name == "" || !finOpsProviders[payload.Provider] {
		return nil, errors.New("cloud account name and a valid provider are required")
	}
	if payload.SyncFrequency == "" {
		payload.SyncFrequency = "daily"
	}
	if !map[string]bool{"manual": true, "hourly": true, "daily": true, "weekly": true, "monthly": true}[payload.SyncFrequency] {
		return nil, errors.New("invalid billing synchronization frequency")
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
		return errors.New("cloud account ID is required")
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
		return nil, errors.New("cloud account does not exist")
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
				return fmt.Errorf("billing record %d has an invalid date: %w", i+1, err)
			}
			externalID := strings.TrimSpace(input.ExternalID)
			if externalID == "" {
				externalID = fmt.Sprintf("%s|%s|%s|%s|%.6f", date.Format("2006-01-02"), input.Service, input.ResourceID, input.UsageUnit, input.Amount)
			}
			tags, _ := json.Marshal(input.Tags)
			actualPayment := input.ActualPayment
			if actualPayment == 0 {
				actualPayment = input.Amount
			}
			originalPrice := input.OriginalPrice
			if originalPrice == 0 {
				originalPrice = actualPayment + input.Discount
			}
			discount := input.Discount
			if discount == 0 && originalPrice > actualPayment {
				discount = originalPrice - actualPayment
			}
			resourceConfig := strings.TrimSpace(input.ResourceConfig)
			if resourceConfig == "" && input.UsageQuantity != 0 {
				resourceConfig = fmt.Sprintf("%.2f %s", input.UsageQuantity, strings.TrimSpace(input.UsageUnit))
			}
			record := model.IntegrationFinOpsCostRecord{AccountID: account.ID, Provider: account.Provider, ExternalID: externalID,
				BillingDate: date, Service: input.Service, Region: input.Region, ResourceID: input.ResourceID,
				ResourceName: input.ResourceName, ResourceType: input.ResourceType, ResourceConfig: resourceConfig, Tags: string(tags), Amount: actualPayment, OriginalPrice: originalPrice, Discount: discount, ActualPayment: actualPayment,
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
			total += actualPayment
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
		result.SourceRecordCount += monthly.SourceRecordCount
		result.SourceTotalAmount += monthly.SourceTotalAmount
		result.RecordCount += monthly.RecordCount
		result.TotalAmount += monthly.TotalAmount
		result.DeduplicatedCount += monthly.DeduplicatedCount
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
	finish := func(status, message string, sourceCount int, sourceAmount float64, count int, amount float64, snapshotVerified bool) {
		finished := time.Now()
		deduplicatedCount := 0
		if snapshotVerified && sourceCount > count {
			deduplicatedCount = sourceCount - count
		}
		logEntry.Status, logEntry.Message, logEntry.SourceRecordCount, logEntry.SourceTotalAmount = status, message, sourceCount, sourceAmount
		logEntry.RecordCount, logEntry.TotalAmount, logEntry.DeduplicatedCount, logEntry.SnapshotVerified, logEntry.FinishedAt = count, amount, deduplicatedCount, snapshotVerified, &finished
		_ = s.db.Save(&logEntry).Error
	}
	records, err := s.fetchFinOpsCostsForMonth(context.Background(), account, monthText, 10)
	if err != nil {
		result.Error = err.Error()
		finish("failed", result.Error, 0, 0, 0, 0, false)
		return result
	}
	if len(records) == 0 {
		// An empty provider response can be a legitimate zero-cost month, but it
		// must never erase a previously successful snapshot.  Keep old data and
		// make the condition explicit in the execution history.
		count, amount, statErr := s.finOpsMonthSnapshot(account.ID, month)
		if statErr != nil {
			result.Error = statErr.Error()
			finish("failed", result.Error, 0, 0, 0, 0, false)
			return result
		}
		result.Status, result.RecordCount, result.TotalAmount, result.SnapshotVerified = "success", count, amount, true
		finish("success", finOpsBillingSource(account.Provider)+" billing synchronization completed for "+monthText+"; no detail rows were returned, so the existing stored snapshot was retained", 0, 0, count, amount, true)
		return result
	}
	sourceCount, sourceAmount := finOpsInputSummary(records)
	_, _, err = s.upsertFinOpsCosts(account, records)
	if err != nil {
		result.Error = err.Error()
		result.SourceRecordCount, result.SourceTotalAmount = sourceCount, sourceAmount
		finish("failed", result.Error, sourceCount, sourceAmount, 0, 0, false)
		return result
	}
	if err := s.removeStaleFinOpsCostsForMonth(account.ID, month, records); err != nil {
		result.Error = err.Error()
		result.SourceRecordCount, result.SourceTotalAmount = sourceCount, sourceAmount
		finish("failed", result.Error, sourceCount, sourceAmount, 0, 0, false)
		return result
	}
	count, amount, err := s.finOpsMonthSnapshot(account.ID, month)
	if err != nil {
		result.Error = err.Error()
		result.SourceRecordCount, result.SourceTotalAmount = sourceCount, sourceAmount
		finish("failed", result.Error, sourceCount, sourceAmount, 0, 0, false)
		return result
	}
	deduplicatedCount := sourceCount - count
	if deduplicatedCount < 0 {
		deduplicatedCount = 0
	}
	result.Status, result.SourceRecordCount, result.SourceTotalAmount = "success", sourceCount, sourceAmount
	result.RecordCount, result.TotalAmount, result.DeduplicatedCount, result.SnapshotVerified = count, amount, deduplicatedCount, true
	finish("success", finOpsBillingSource(account.Provider)+" billing synchronization completed for "+monthText, sourceCount, sourceAmount, count, amount, true)
	return result
}

func finOpsInputSummary(inputs []FinOpsCostInput) (int, float64) {
	total := 0.0
	for _, input := range inputs {
		amount := input.ActualPayment
		if amount == 0 {
			amount = input.Amount
		}
		total += amount
	}
	return len(inputs), total
}

func (s *Service) finOpsMonthSnapshot(accountID uint, month time.Time) (int, float64, error) {
	start := time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, month.Location())
	end := start.AddDate(0, 1, 0)
	var summary struct {
		RecordCount int
		TotalAmount float64
	}
	err := s.db.Model(&model.IntegrationFinOpsCostRecord{}).
		Select("COUNT(*) AS record_count, COALESCE(SUM(amount), 0) AS total_amount").
		Where("account_id = ? AND billing_date >= ? AND billing_date < ?", accountID, start, end).
		Scan(&summary).Error
	return summary.RecordCount, summary.TotalAmount, err
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
		key := "Uncategorized"
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
			key = "Uncategorized"
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
	assetConfigs := map[string]string{}
	var hosts []model.AssetHost
	if err := s.db.Select("instance_id", "cpu", "memory", "disk").Where("instance_id <> ''").Find(&hosts).Error; err == nil {
		for _, host := range hosts {
			if config := finOpsHostResourceConfig(host); config != "" {
				assetConfigs[host.InstanceID] = config
			}
		}
	}
	type aggregate struct {
		OriginalPrice, Discount, ActualPayment        float64
		Provider, Account, Region, Type, Name, Config string
		Count                                         int
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
			key = "Unlinked Resource|" + r.Service
		}
		key = fmt.Sprintf("%d|%s|%s", r.AccountID, key, r.ResourceType)
		a := values[key]
		if a == nil {
			a = &aggregate{Provider: r.Provider, Account: names[r.AccountID], Region: r.Region, Type: r.ResourceType, Name: r.ResourceName, Config: firstNonEmpty(r.ResourceConfig, assetConfigs[r.ResourceID])}
			values[key] = a
		}
		actualPayment := r.ActualPayment
		if actualPayment == 0 {
			actualPayment = r.Amount
		}
		originalPrice := r.OriginalPrice
		if originalPrice == 0 {
			originalPrice = actualPayment + r.Discount
		}
		a.OriginalPrice += originalPrice
		a.Discount += r.Discount
		a.ActualPayment += actualPayment
		a.Count++
	}
	rows := make([]map[string]any, 0, len(values))
	for id, a := range values {
		rows = append(rows, map[string]any{"resourceId": id, "resourceName": a.Name, "resourceType": a.Type, "resourceConfig": a.Config, "provider": a.Provider, "accountName": a.Account, "region": a.Region, "originalPrice": a.OriginalPrice, "discount": a.Discount, "actualPayment": a.ActualPayment, "cost": a.ActualPayment, "recordCount": a.Count})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i]["actualPayment"].(float64) > rows[j]["actualPayment"].(float64) })
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

func finOpsHostResourceConfig(host model.AssetHost) string {
	parts := make([]string, 0, 3)
	if value := strings.TrimSpace(host.CPU); value != "" {
		parts = append(parts, "CPU "+value)
	}
	if value := strings.TrimSpace(host.Memory); value != "" {
		parts = append(parts, "Memory "+value)
	}
	if value := strings.TrimSpace(host.Disk); value != "" {
		parts = append(parts, "Disk "+value)
	}
	return strings.Join(parts, " · ")
}

func (s *Service) GenerateFinOpsRecommendations(modelID uint, strategy string, accountID uint, analysisMonth string) (int, string, error) {
	now := time.Now()
	if strings.TrimSpace(analysisMonth) == "" {
		analysisMonth = now.Format("2006-01")
	}
	start, err := parseFinOpsMonth(analysisMonth)
	if err != nil {
		return 0, "", errors.New("invalid month parameter; expected YYYY-MM")
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
		return 0, "", errors.New("invalid recommendation generation strategy")
	}
	analysisScope := "전체 Cloud Account"
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
		scope = "전체 Cloud Account"
	}
	if strings.TrimSpace(analysisMonth) == "" {
		analysisMonth = time.Now().Format("2006-01")
	}
	strategyName := "기본 Strategy"
	if strategy == "ai" {
		strategyName = "AI 분석"
	} else if strategy == "ai_fallback" {
		strategyName = "AI Fallback (기본 Strategy)"
	}
	return fmt.Sprintf("%s | %s | %s 최적화 권고", scope, analysisMonth, strategyName)
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
	description.WriteString("## 실행 요약\n이번 달 Billing을 기준으로 생성한 기본 FinOps 분석 결과입니다. Billing에는 CPU, Memory, Connection 같은 실시간 Monitoring Metric이 없으므로 유휴 및 저활용 항목은 검증 대상입니다.")
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
			name = "Service Resource"
		}
		description.WriteString(fmt.Sprintf("\n%d. %s: 이번 달 비용 %.2f. 사용률, 유휴 시간대, 약정 할인 적용 가능성을 검토하십시오. 예상 절감액은 %.2f입니다.", index+1, name, item.Cost, recommendationSaving))
	}
	description.WriteString("\n\n## 유휴 Resource\n이번 달에도 비용이 발생하지만 대응하는 Runtime Workload 또는 업무 Access가 없는 Resource를 우선 확인하십시오. 검증 후 중지, 해제 또는 Scheduled Start/Stop을 적용할 수 있습니다.")
	description.WriteString("\n\n## 저활용 Resource\n고비용 Compute, Database, Middleware Resource의 CPU, Memory, Connection, IOPS를 확인하십시오. 지속적으로 사용률이 낮으면 Downsize를 검토합니다.")
	description.WriteString("\n\n## Billing 방식 최적화\n안정적으로 실행되는 Resource에는 Subscription, Savings Plan 또는 Reserved Instance를 평가하고 중복 Resource Package 구매를 방지하십시오.")
	description.WriteString("\n\n## 유휴 Disk / Snapshot / IP\n연결되지 않은 Cloud Disk, 장기 Snapshot, 미연결 EIP, 유휴 Load Balancer를 점검하고 Dependency가 없음을 확인한 뒤 정리하십시오.")
	description.WriteString(fmt.Sprintf("\n\n## 예상 절감액\n이번 달 분석 대상 비용은 %.2f이며 보수적인 15%% 기준 예상 월 절감액은 %.2f입니다.", total, saving))
	return []model.IntegrationFinOpsRecommendation{{Provider: "multi-cloud", Category: "cost_review", Priority: "P2", Title: "이번 달 Cloud 비용 최적화 권고", Description: description.String(), CurrentCost: total, Saving: saving, Status: "open"}}
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
		return nil, errors.New("no cost records are available for AI recommendation generation")
	}
	contextJSON, _ := json.Marshal(map[string]any{"finopsToolResult": analysisData})
	prompt := "Using the following locally synchronized cloud billing data, produce a concise and actionable Korean FinOps analysis. Do not call cloud APIs or invent resources, amounts, or monitoring metrics. Use Markdown headings covering: execution summary, idle-resource review, low-utilization review, billing-model optimization, idle disks/snapshots/IPs, and estimated savings. When real-time monitoring data is unavailable, explicitly state that validation is required. Do not output JSON, code fences, or tables. Keep the report under 900 Korean characters. Data: " + string(contextJSON)
	response, err := s.callOpenAICompatible(aiModel, []map[string]any{{"role": "system", "content": "You are a rigorous FinOps analyst. Output only a concise Korean Markdown analysis report and no JSON."}, {"role": "user", "content": prompt}}, nil)
	if err != nil {
		return nil, err
	}
	content := strings.TrimSpace(response.Content)
	if content == "" || hasUnsupportedAIToolProtocol(content) {
		return nil, errors.New("AI returned no displayable FinOps analysis")
	}
	recommendation := base[0]
	recommendation.Category = "ai_finops"
	recommendation.Title = "이번 달 Cloud 비용 AI 최적화 권고"
	recommendation.Description = "## AI 분석 결론\n" + truncateRunes(content, 12000)
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
		prompt := "Using the following cloud-cost analysis tool result, generate no more than five actionable Korean optimization recommendations. The tool result comes only from the locally synchronized billing database and does not represent real-time cloud state. Cover idle resources, low utilization, billing-model optimization, idle disks/snapshots/IPs, and estimated savings. When billing lacks real-time monitoring metrics, state that validation is required rather than claiming a resource is idle. Each description must be no longer than 80 Korean characters. Return exactly one complete JSON object: {\"recommendations\":[{\"accountId\":1,\"provider\":\"...\",\"resourceId\":\"...\",\"priority\":\"P1|P2|P3\",\"title\":\"...\",\"description\":\"...\",\"currentCost\":0,\"saving\":0}]}. Do not output Markdown fences, headings, or text outside JSON. Do not invent resources; saving must be non-negative and no greater than currentCost. Data: " + string(contextJSON)
		response, err := s.callOpenAICompatibleJSON(aiModel, []map[string]any{{"role": "system", "content": "You are a rigorous FinOps analyst. Output only JSON that satisfies the requested schema."}, {"role": "user", "content": prompt}})
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
			repairPrompt := "Convert the following FinOps analysis result into one complete JSON object. Return JSON only, without Markdown or explanation. The schema must be {\"recommendations\":[{\"accountId\":1,\"provider\":\"...\",\"resourceId\":\"...\",\"priority\":\"P1|P2|P3\",\"title\":\"...\",\"description\":\"...\",\"currentCost\":0,\"saving\":0}]}. If fields are missing, use conservative values from the known account and cost data and do not invent resources. Original content:\n" + truncateRunes(response.Content, 12000)
			repaired, repairErr := s.callOpenAICompatibleJSON(aiModel, []map[string]any{{"role": "system", "content": "You are a strict JSON repair tool. Output exactly one valid JSON object."}, {"role": "user", "content": repairPrompt}})
			if repairErr == nil {
				content, parseErr = extractFinOpsRecommendationJSON(repaired.Content)
			}
			if parseErr != nil {
				return nil, parseErr
			}
		}
		if err := json.Unmarshal([]byte(content), &payload); err != nil {
			return nil, fmt.Errorf("failed to parse AI recommendation JSON: %w", err)
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
			return nil, errors.New("AI generated no valid optimization recommendations")
		}
		priority, total, saving := "P3", 0.0, 0.0
		var description strings.Builder
		description.WriteString("## 실행 요약\n다음은 AI가 이번 달 Billing을 기준으로 생성한 종합 최적화 권고입니다.")
		for index, item := range analysis {
			total += item.CurrentCost
			saving += item.Saving
			if item.Priority == "P1" || (item.Priority == "P2" && priority == "P3") {
				priority = item.Priority
			}
			description.WriteString(fmt.Sprintf("\n%d. %s: %s (현재 비용 %.2f, 예상 절감액 %.2f)", index+1, item.Title, item.Description, item.CurrentCost, item.Saving))
		}
		description.WriteString("\n\n## 유휴 Resource\n중지 상태지만 비용이 발생하거나 업무 Access 또는 Monitoring Load가 없는 Resource를 우선 검증하십시오.")
		description.WriteString("\n\n## 저활용 Resource\nCPU, Memory, IOPS, Connection 등의 Monitoring 데이터를 함께 확인해 Downsize 여부를 결정하십시오.")
		description.WriteString("\n\n## Billing 방식 최적화\n안정적인 Workload에 Subscription, Savings Plan 또는 Reserved Instance가 적합한지 평가하십시오.")
		description.WriteString("\n\n## 유휴 Disk / Snapshot / IP\n연결되지 않은 Disk, 장기 Snapshot, 미연결 IP, Backend가 없는 Load Balancer를 점검하십시오.")
		description.WriteString(fmt.Sprintf("\n\n## 예상 절감액\n이 Report의 분석 대상 비용은 %.2f이며 AI 예상 절감액은 %.2f입니다.", total, saving))
		return []model.IntegrationFinOpsRecommendation{{Provider: "multi-cloud", Category: "ai_finops", Priority: priority, Title: "이번 달 Cloud 비용 AI 최적화 권고", Description: description.String(), CurrentCost: total, Saving: saving, Status: "open"}}, nil
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
	return "", errors.New("AI did not return complete JSON; the response may contain explanatory text or may have been truncated")
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
		return errors.New("invalid recommendation status")
	}
	return s.db.Model(&model.IntegrationFinOpsRecommendation{}).Where("id = ?", id).Update("status", status).Error
}

func (s *Service) DeleteFinOpsRecommendation(id uint) error {
	if id == 0 {
		return errors.New("recommendation ID is required")
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
	triggerLabels := map[string]string{"manual": "수동 동기화", "scheduled": "Scheduled 동기화", "api": "API Trigger"}
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
			"sourceRecordCount": row.SourceRecordCount, "sourceTotalAmount": row.SourceTotalAmount,
			"recordCount": row.RecordCount, "totalAmount": row.TotalAmount, "deduplicatedCount": row.DeduplicatedCount, "snapshotVerified": row.SnapshotVerified, "message": row.Message,
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
	return time.Time{}, errors.New("supported formats are YYYY-MM-DD or RFC3339")
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
