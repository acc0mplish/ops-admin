package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/secrets"
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
	// J5-3 (r2 — F2): a rotated credential makes the V2 link a lie — the
	// stale link would resolve the OLD material from the broker while the row
	// carries the new secret. Detect the rotation here and clear the link in
	// the same transaction as the save; sync-inventory re-creates the chain
	// from the new material (runbook, plan §11).
	previous := account
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
	credentialRotated := account.AccessKey != previous.AccessKey ||
		account.SecretKey != previous.SecretKey ||
		account.BillingToken != previous.BillingToken
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
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&account).Error; err != nil {
			return err
		}
		if credentialRotated {
			// White-field update only — NULL the link, touch nothing else.
			if err := tx.Model(&model.IntegrationFinOpsAccount{}).Where("id = ?", account.ID).
				Update("provider_connection_uid", gorm.Expr("NULL")).Error; err != nil {
				return err
			}
			account.ProviderConnectionUID = ""
		}
		return nil
	}); err != nil {
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

	// J5 (plan phase4 r2 §3.3): a linked account resolves its billing
	// credential through the V2 secrets broker. The resolved plaintext is
	// injected into the LOCAL copy only — `account` itself (and the v1 row it
	// was loaded from) stays untouched, and the trailing update below is a
	// narrow two-column UPDATE that can never write the credential columns
	// back (the r2 plaintext re-seal block, claim 16). An empty link keeps
	// the pre-rewire behavior: the row's own (sealed) columns feed the sync.
	rewired := account
	if account.ProviderConnectionUID != "" {
		resolved, err := secrets.NewBroker(s.db).Resolve(context.Background(), account.ProviderConnectionUID, contract.CredentialPurposeBilling)
		if err != nil {
			return FinOpsSyncResult{}, fmt.Errorf("finops: resolve billing credential through provider connection %q: %w", account.ProviderConnectionUID, err)
		}
		credential, err := decodeFinOpsCredentialBlob(resolved.Value)
		if err != nil {
			return FinOpsSyncResult{}, fmt.Errorf("finops: provider connection %q carries unusable credential material: %w", account.ProviderConnectionUID, err)
		}
		rewired.AccessKey = credential.AccessKey
		rewired.SecretKey = credential.SecretKey
		if credential.BillingToken != "" {
			rewired.BillingToken = credential.BillingToken
		}
	}

	result := FinOpsSyncResult{AccountID: account.ID, Provider: account.Provider, StartMonth: start.Format("2006-01"), EndMonth: end.Format("2006-01")}
	for month := start; !month.After(end); month = month.AddDate(0, 1, 0) {
		monthly := s.syncFinOpsAccountMonth(rewired, trigger, month)
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
	// Narrow update (J5-2, r2): the UPDATE statement names exactly these two
	// columns — access_key/secret_key/billing_token are structurally absent,
	// so no plaintext (or stale sealed material) can ever be re-written into
	// the credential columns by the sync itself (claim 16 — MD5 invariance).
	_ = s.db.Model(&model.IntegrationFinOpsAccount{}).Where("id = ?", account.ID).
		Select("last_sync_at", "next_sync_at").
		Updates(model.IntegrationFinOpsAccount{LastSyncAt: account.LastSyncAt, NextSyncAt: account.NextSyncAt}).Error
	return result, nil
}

// decodeFinOpsCredentialBlob parses the J4 SecretRef JSON blob
// {"accessKey","secretKey"[,"billingToken"]} — the same shape the cloud
// backfill seals (A4).
func decodeFinOpsCredentialBlob(value string) (finOpsCredentialBlob, error) {
	var blob finOpsCredentialBlob
	if err := json.Unmarshal([]byte(value), &blob); err != nil {
		return blob, err
	}
	if strings.TrimSpace(blob.AccessKey) == "" {
		return blob, errors.New("credential blob has no accessKey")
	}
	return blob, nil
}

type finOpsCredentialBlob struct {
	AccessKey    string `json:"accessKey"`
	SecretKey    string `json:"secretKey"`
	BillingToken string `json:"billingToken"`
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
