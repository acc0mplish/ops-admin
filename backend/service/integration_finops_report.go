package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"ops-admin/backend/model"

	"gorm.io/gorm"
)

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
