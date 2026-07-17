package service

import (
	"testing"

	"ops-admin/backend/model"
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

func TestFinOpsAliCloudDailyRecordsKeepBillDateAndResourceFields(t *testing.T) {
	records := finOpsAliCloudDailyRecords([]map[string]any{{
		"RecordID": "daily-1", "BillingDate": "2026-07-16", "ProductName": "云服务器 ECS",
		"Region": "cn-hangzhou", "InstanceID": "i-abc", "InstanceName": "production-api", "PretaxAmount": "8.50",
	}}, model.IntegrationFinOpsAccount{}, "2026-07", "2026-07-16")
	if len(records) != 1 {
		t.Fatalf("expected one daily record, got %d", len(records))
	}
	record := records[0]
	if record.BillingDate != "2026-07-16" || record.Region != "cn-hangzhou" || record.ResourceID != "i-abc" || record.ResourceName != "production-api" {
		t.Fatalf("unexpected daily record: %#v", record)
	}
}
