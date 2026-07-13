package service

import (
	"testing"

	"ops-admin/backend/model"
)

func TestParseMonitorLogMessage(t *testing.T) {
	fields := parseMonitorLogMessage("2026-07-10 14:20:56.431 INFO 1 --- [worker-5] c.example.CrossTransportHandler : keepalive received")
	if fields["level"] != "INFO" || fields["thread"] != "worker-5" {
		t.Fatalf("unexpected parsed fields: %#v", fields)
	}
	if fields["logger"] != "c.example.CrossTransportHandler" || fields["content"] != "keepalive received" {
		t.Fatalf("message content was not split correctly: %#v", fields)
	}
}

func TestParseMonitorLogMessageFallsBackToRawContent(t *testing.T) {
	fields := parseMonitorLogMessage("plain log message")
	if fields["content"] != "plain log message" {
		t.Fatalf("expected raw message fallback, got %#v", fields)
	}
}

func TestApplyMonitorEventAggregationUpdatesPersistedAndInMemoryState(t *testing.T) {
	event := model.MonitorAlertEvent{}
	updates := map[string]any{}
	aggregation := model.MonitorAggregationRule{ID: 7, Name: "按实例收敛"}

	applyMonitorEventAggregation(&event, updates, aggregation, "aggregation=7|instance=10.0.0.1:9100")

	if event.AggregationKey == "" || event.AggregateRuleID != 7 || event.AggregateRuleName != aggregation.Name {
		t.Fatalf("in-memory aggregation state was not updated: %#v", event)
	}
	if updates["aggregation_key"] != event.AggregationKey || updates["aggregate_rule_id"] != aggregation.ID {
		t.Fatalf("database updates and in-memory state differ: %#v", updates)
	}
}

func TestAggregationLookbackCoversRepeatInterval(t *testing.T) {
	aggregation := model.MonitorAggregationRule{WindowSeconds: 300, RepeatIntervalSeconds: 1800}
	if got := aggregationLookbackSeconds(aggregation); got != 1800 {
		t.Fatalf("expected repeat interval lookback, got %d", got)
	}
}

func TestAggregationLookbackCoversConvergenceWindow(t *testing.T) {
	aggregation := model.MonitorAggregationRule{WindowSeconds: 900, RepeatIntervalSeconds: 300}
	if got := aggregationLookbackSeconds(aggregation); got != 900 {
		t.Fatalf("expected convergence window lookback, got %d", got)
	}
}

func TestMonitorRuleEvaluationCannotOverlap(t *testing.T) {
	service := &Service{monitorScheduler: &MonitorScheduler{running: map[uint]bool{}}}
	if !service.beginMonitorAlertRuleEvaluation(9) {
		t.Fatal("first evaluation should start")
	}
	if service.beginMonitorAlertRuleEvaluation(9) {
		t.Fatal("overlapping evaluation should be rejected")
	}
	service.endMonitorAlertRuleEvaluation(9)
	if !service.beginMonitorAlertRuleEvaluation(9) {
		t.Fatal("evaluation should start after the previous run finishes")
	}
}
