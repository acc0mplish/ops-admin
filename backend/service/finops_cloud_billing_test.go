package service

import (
	"context"
	"encoding/json"
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
	resourceData, err := (&Service{db: db}).queryAIFinOpsAnalysis(map[string]any{"month": data["analysisMonth"], "service": "负载均衡", "includeResourceBreakdown": true})
	if err != nil {
		t.Fatal(err)
	}
	breakdown, _ := resourceData["resourceBreakdown"].(map[string]any)
	t.Logf("load balancer local resource breakdown: instances=%v matchedRecords=%v unattributedCost=%v", breakdown["resourceCount"], breakdown["matchedRecordCount"], breakdown["unattributedCost"])
	for _, item := range breakdown["items"].([]map[string]any) {
		t.Logf("load balancer instance id=%v name=%v amount=%.2f", item["resourceId"], item["resourceName"], item["amount"].(float64))
	}
}

func TestAssetHostUsageMetricsProbe(t *testing.T) {
	if os.Getenv("ASSET_HOST_METRICS_PROBE") != "1" {
		t.Skip("set ASSET_HOST_METRICS_PROBE=1 to inspect local host monitoring metrics")
	}
	cfg, err := config.Load("../config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	db, err := store.NewDB(cfg)
	if err != nil {
		t.Fatal(err)
	}
	data, err := (&Service{db: db}).ListAssetHosts(1, 10, "", 0, "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	hosts, _ := data["list"].([]model.AssetHost)
	for _, host := range hosts {
		t.Logf("host=%s ip=%s metrics=%s cpu=%s memory=%s disk=%s", host.HostName, host.SSHIP, host.MetricsStatus, host.CPUUsage, host.MemoryUsage, host.DiskUsage)
	}
}

func TestIntegrationAIDSMLToolProbe(t *testing.T) {
	if os.Getenv("AI_DSML_TOOL_PROBE") != "1" {
		t.Skip("set AI_DSML_TOOL_PROBE=1 to make one controlled local AI tool-call probe")
	}
	cfg, err := config.Load("../config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	db, err := store.NewDB(cfg)
	if err != nil {
		t.Fatal(err)
	}
	service := &Service{db: db}
	var aiModel model.IntegrationAIModel
	if err := db.Where("status = ?", 1).Order("is_default DESC, id ASC").First(&aiModel).Error; err != nil {
		t.Fatal(err)
	}
	tools, _, err := service.openAIToolDefinitions()
	if err != nil {
		t.Fatal(err)
	}
	messages := []map[string]any{
		{"role": "system", "content": integrationAISystemPrompt(aiModel.SystemPrompt)},
		{"role": "user", "content": "检查当前 K8s 集群健康状态。"},
	}
	response, err := service.callOpenAICompatible(aiModel, messages, tools)
	if err != nil {
		t.Fatal(err)
	}
	if len(response.ToolCalls) == 0 {
		t.Fatalf("expected native or DSML-mapped tool calls, got content=%q", truncateRunes(response.Content, 300))
	}
	for round := 0; round < 3 && len(response.ToolCalls) > 0; round++ {
		messages = append(messages, map[string]any{"role": "assistant", "content": response.Content, "tool_calls": response.RawToolCalls})
		for _, call := range response.ToolCalls {
			if call.Name == "k8s_restart_workload" || call.Name == "k8s_scale_workload" {
				t.Fatalf("probe refuses to execute non-read tool %s", call.Name)
			}
			var args map[string]any
			if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
				t.Fatal(err)
			}
			result, toolErr := service.executeIntegrationAITool(call.Name, args)
			if toolErr != nil {
				result = map[string]any{"error": toolErr.Error()}
			}
			rawResult, _ := json.Marshal(result)
			messages = append(messages, map[string]any{"role": "tool", "tool_call_id": call.ID, "content": string(rawResult)})
			t.Logf("mapped AI tool call: round=%d name=%s arguments=%s", round+1, call.Name, call.Arguments)
		}
		response, err = service.callOpenAICompatible(aiModel, messages, tools)
		if err != nil {
			t.Fatal(err)
		}
	}
	if hasUnsupportedAIToolProtocol(response.Content) {
		t.Fatalf("DSML protocol leaked into final answer: %q", truncateRunes(response.Content, 300))
	}
	t.Logf("final AI answer: %s", truncateRunes(response.Content, 500))
}
