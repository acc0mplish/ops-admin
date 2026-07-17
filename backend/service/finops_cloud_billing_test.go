package service

import (
	"context"
	"os"
	"testing"
	"time"

	"ops-admin/backend/config"
	"ops-admin/backend/model"
	"ops-admin/backend/store"
)

func TestFinOpsAliCloudRecordsUseRecordID(t *testing.T) {
	records := finOpsAliCloudRecords([]map[string]any{
		{"RecordID": "first", "ProductCode": "ecs", "PretaxAmount": "36.48"},
		{"RecordID": "second", "ProductCode": "ecs", "PretaxAmount": "116.49"},
	}, model.IntegrationFinOpsAccount{}, "2026-06")
	if len(records) != 2 || records[0].ExternalID == records[1].ExternalID {
		t.Fatalf("expected distinct stable external IDs, got %#v", records)
	}
	if records[0].ExternalID != "alicloud|2026-06|first" {
		t.Fatalf("unexpected external ID: %s", records[0].ExternalID)
	}
}

func TestFinOpsAliCloudEstimatedDailyRecordsKeepResourceFields(t *testing.T) {
	records := finOpsAliCloudEstimatedDailyRecords([]map[string]any{{
		"RecordID": "daily-1", "ProductName": "云服务器 ECS",
		"Region": "cn-hangzhou", "InstanceID": "i-abc", "InstanceName": "production-api", "PretaxAmount": "8.50",
	}}, model.IntegrationFinOpsAccount{}, "2026-06")
	if len(records) != 30 {
		t.Fatalf("expected one record for each day, got %d", len(records))
	}
	record := records[0]
	if record.BillingDate != "2026-06-01" || record.Region != "cn-hangzhou" || record.ResourceID != "i-abc" || record.ResourceName != "production-api" {
		t.Fatalf("unexpected daily record: %#v", record)
	}
}

func TestFinOpsRecordDataQuality(t *testing.T) {
	if quality := finOpsRecordDataQuality(`{"granularity":"daily_estimate"}`); quality != "estimated" {
		t.Fatalf("expected estimated, got %s", quality)
	}
	if quality := finOpsRecordDataQuality(`{"billingCycle":"2026-07"}`); quality != "monthly" {
		t.Fatalf("expected monthly, got %s", quality)
	}
	if quality := finOpsRecordDataQuality(`{}`); quality != "exact" {
		t.Fatalf("expected exact, got %s", quality)
	}
}

func TestFinOpsAliCloudBillItemsSupportsDocumentedEnvelopes(t *testing.T) {
	legacy, count := finOpsAliCloudBillItems(map[string]any{"Data": map[string]any{"TotalCount": "1", "Items": map[string]any{"Item": []any{map[string]any{"RecordID": "legacy"}}}}})
	if len(legacy) != 1 || count != 1 {
		t.Fatalf("legacy envelope parsed incorrectly: %#v, %d", legacy, count)
	}
	upgraded, count := finOpsAliCloudBillItems(map[string]any{"Data": map[string]any{"TotalCount": 1, "Items": []any{map[string]any{"RecordID": "upgraded"}}}})
	if len(upgraded) != 1 || count != 1 {
		t.Fatalf("upgraded envelope parsed incorrectly: %#v, %d", upgraded, count)
	}
}

func TestFinOpsLiveAliCloudBillProbe(t *testing.T) {
	if os.Getenv("FINOPS_LIVE_PROBE") != "1" {
		t.Skip("set FINOPS_LIVE_PROBE=1 to probe the configured AliCloud account")
	}
	cfg, err := config.Load("../config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	db, err := store.NewDB(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var account model.IntegrationFinOpsAccount
	if err := db.Where("provider = ?", "alicloud").First(&account).Error; err != nil {
		t.Fatal(err)
	}
	month := os.Getenv("FINOPS_LIVE_MONTH")
	if month == "" {
		month = time.Now().Format("2006-01")
	}
	records, err := (&Service{db: db}).fetchAliCloudMonthlyInstanceBill(context.Background(), account, month, 10)
	if err != nil {
		t.Fatal(err)
	}
	total := 0.0
	for _, record := range records {
		total += record.Amount
	}
	t.Logf("AliCloud bill probe parsed %d records, total %.2f", len(records), total)
}

func TestFinOpsResourceBreakdownProbe(t *testing.T) {
	if os.Getenv("FINOPS_RESOURCE_PROBE") != "1" {
		t.Skip("set FINOPS_RESOURCE_PROBE=1 to inspect persisted resource breakdown data")
	}
	cfg, err := config.Load("../config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	db, err := store.NewDB(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var account model.IntegrationFinOpsAccount
	if err := db.Where("provider = ?", "alicloud").First(&account).Error; err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.Local)
	end := time.Date(2026, 7, 18, 0, 0, 0, 0, time.Local)
	data, err := (&Service{db: db}).FinOpsResources(start, end, account.ID, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	items, _ := data["items"].([]map[string]any)
	t.Logf("resource breakdown probe returned %d resources", len(items))
}

func TestFinOpsAICostAnalysisProbe(t *testing.T) {
	if os.Getenv("FINOPS_AI_COST_PROBE") != "1" {
		t.Skip("set FINOPS_AI_COST_PROBE=1 to inspect the local AI FinOps analysis source")
	}
	cfg, err := config.Load("../config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	db, err := store.NewDB(cfg)
	if err != nil {
		t.Fatal(err)
	}
	data, err := (&Service{db: db}).queryAIFinOpsAnalysis(map[string]any{"trendMonths": 6})
	if err != nil {
		t.Fatal(err)
	}
	dashboard, _ := data["selectedMonthSummary"].(map[string]any)
	t.Logf("AI FinOps tool selected month=%v source=%v records=%v total=%v", data["analysisMonth"], data["monthSource"], dashboard["recordCount"], dashboard["totalCost"])
}
