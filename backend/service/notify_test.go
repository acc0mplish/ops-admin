package service

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestNormalizeNotifyEvents(t *testing.T) {
	if got := normalizeNotifyEvents([]string{"failed", "failed", "all"}, "job"); !reflect.DeepEqual(got, []string{"all"}) {
		t.Fatalf("expected all event to be exclusive, got %#v", got)
	}
	if got := normalizeNotifyEvents(nil, "monitor"); !reflect.DeepEqual(got, []string{"firing", "recovered"}) {
		t.Fatalf("unexpected monitor defaults: %#v", got)
	}
}

func TestParseNotifyBusinessResponse(t *testing.T) {
	tests := []struct {
		name        string
		channelType string
		body        string
		code        string
		message     string
		exists      bool
	}{
		{name: "dingtalk success", channelType: "dingtalk", body: `{"errcode":0,"errmsg":"ok"}`, code: "0", message: "ok", exists: true},
		{name: "wecom failure", channelType: "wecom", body: `{"errcode":40001,"errmsg":"invalid credential"}`, code: "40001", message: "invalid credential", exists: true},
		{name: "feishu failure", channelType: "feishu", body: `{"code":19001,"msg":"invalid webhook"}`, code: "19001", message: "invalid webhook", exists: true},
		{name: "custom webhook", channelType: "webhook", body: `{"code":500}`, exists: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			code, message, exists := parseNotifyBusinessResponse(test.channelType, []byte(test.body))
			if code != test.code || message != test.message || exists != test.exists {
				t.Fatalf("got code=%q message=%q exists=%v", code, message, exists)
			}
		})
	}
}

func TestNotifyRetryDelay(t *testing.T) {
	if notifyRetryDelay(1) != 10*time.Second || notifyRetryDelay(2) != 30*time.Second || notifyRetryDelay(3) != 2*time.Minute {
		t.Fatal("unexpected retry schedule")
	}
}

func TestRenderNotifyTemplateUsesBusinessStatusLabels(t *testing.T) {
	job := renderNotifyTemplate("{{status}}/{{jobName}}/{{stepName}}", NotifyEvent{
		Scope: "job", Status: "notice", TargetName: "\u53d1\u5e03\u4f5c\u4e1a",
		Extra: map[string]string{"stepName": "\u6d88\u606f\u901a\u77e5"},
	})
	if job != "알림/\u53d1\u5e03\u4f5c\u4e1a/\u6d88\u606f\u901a\u77e5" {
		t.Fatalf("unexpected job rendering: %q", job)
	}
	monitor := renderNotifyTemplate("{{status}}", NotifyEvent{Scope: "monitor", Status: "recovered"})
	if monitor != "복구" {
		t.Fatalf("unexpected monitor status: %q", monitor)
	}
}

func TestNormalizeNotifyTemplateForJobRejectsMonitorLayout(t *testing.T) {
	title, content := normalizeNotifyTemplateForEvent("\u3010{{severity}}\u3011{{alertName}} -- {{status}}", "\u5b9e\u4f8b\uff1a{{instance}}", NotifyEvent{Scope: "job"})
	if !strings.Contains(title, "[Job 알림]") || !strings.Contains(content, "{{jobHistoryId}}") {
		t.Fatalf("monitor template was not replaced for job event: %q %q", title, content)
	}
}

func TestNotifyTemplateScopeCompatible(t *testing.T) {
	if !notifyTemplateScopeCompatible("all", "job") {
		t.Fatal("generic template should support job events")
	}
	if !notifyTemplateScopeCompatible("job", "job") {
		t.Fatal("job template should support job events")
	}
	if notifyTemplateScopeCompatible("monitor", "job") {
		t.Fatal("monitor template must not support job events")
	}
	if notifyTemplateScopeCompatible("job", "all") {
		t.Fatal("specific-scope template must not bind to an all-scope rule")
	}
}
