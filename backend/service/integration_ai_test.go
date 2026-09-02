package service

import (
	"testing"
	"time"
)

func TestParseAILogTimeRelative(t *testing.T) {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 16, 15, 30, 0, 0, location)

	startAt, err := parseAILogTime("\u6628\u5929 10:00", now)
	if err != nil {
		t.Fatal(err)
	}
	endAt, err := parseAILogTime("yesterday 11:00", now)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := startAt.Format("2006-01-02 15:04:05 -07:00"), "2026-07-15 10:00:00 +08:00"; got != want {
		t.Fatalf("startAt = %s, want %s", got, want)
	}
	if got, want := endAt.Format("2006-01-02 15:04:05 -07:00"), "2026-07-15 11:00:00 +08:00"; got != want {
		t.Fatalf("endAt = %s, want %s", got, want)
	}
}

func TestAppendAILogStreamFilterVictoriaLogsContains(t *testing.T) {
	got := appendAILogStreamFilter("level:ERROR", []string{"err.log", "app.err.log"}, true)
	want := `(kafka_topic:*err.log* OR kafka_topic:*app.err.log*) AND (level:ERROR)`
	if got != want {
		t.Fatalf("query = %q, want %q", got, want)
	}
}

func TestAppendAILogStreamFilterElasticsearchContains(t *testing.T) {
	got := appendAILogStreamFilter("level:ERROR", []string{"err.log", "app.err.log"}, false)
	want := `(level:ERROR) AND (kafka_topic:*err.log* OR kafka_topic:*app.err.log*)`
	if got != want {
		t.Fatalf("query = %q, want %q", got, want)
	}
}

func TestAppendAILogStreamFilterExact(t *testing.T) {
	got := appendAILogStreamFilter("", []string{"=app.err.log.1053"}, true)
	want := `({kafka_topic="app.err.log.1053"})`
	if got != want {
		t.Fatalf("query = %q, want %q", got, want)
	}
}

func TestAppendAILogStreamFilterVictoriaLogsContainsCount(t *testing.T) {
	got := appendAILogStreamFilter("*", []string{"err.log"}, true)
	want := `(kafka_topic:*err.log*)`
	if got != want {
		t.Fatalf("query = %q, want %q", got, want)
	}
}

func TestAILogTotal(t *testing.T) {
	if got := aiLogTotal(map[string]any{"value": float64(18)}); got != 18 {
		t.Fatalf("total = %d, want 18", got)
	}
}
