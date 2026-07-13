package service

import (
	"reflect"
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
