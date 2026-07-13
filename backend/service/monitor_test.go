package service

import "testing"

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
