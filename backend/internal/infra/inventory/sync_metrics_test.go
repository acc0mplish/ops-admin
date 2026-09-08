package inventory_test

// §18.2 C/D/E·K 계기 델타 단얫(계획 T-2 — 행위 1회 → 해당 패밀리 라인 +1).
// 헨리스는 sync_test.go를 공유한다(newInventoryDB·seedConnection·stubAdapter·
// twoPages). Runner 배선은 compose.Build와 동일 형상 — runner와 broker가 같은
// counter set을 공유한다.

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"gorm.io/gorm"

	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/inventory"
	"ops-admin/backend/internal/infra/metrics"
	"ops-admin/backend/internal/infra/registry"
	"ops-admin/backend/internal/infra/secrets"
)

// newCountedRunner wires the runner onto an explicit counter set — the same
// shape compose.Build uses, including the broker sharing the same set.
func newCountedRunner(t *testing.T, db *gorm.DB, d *stubAdapter, counters *metrics.Counters) *inventory.SyncRunner {
	t.Helper()
	reg := registry.New()
	if err := reg.RegisterProviderType(d.Descriptor(), d); err != nil {
		t.Fatalf("register stub provider type: %v", err)
	}
	return inventory.NewSyncRunner(db, reg, secrets.NewBrokerWithCounters(db, counters), counters)
}

// TestRunSyncObservesSyncMetrics — 확정점 계기 C/D/E: a succeeded run observes
// one duration and created+updated changes; a partial run feeds the partial
// family and nothing else.
func TestRunSyncObservesSyncMetrics(t *testing.T) {
	db := newInventoryDB(t)
	conn, _ := seedConnection(t, db, "conn-1")

	counters := metrics.New()
	counters.RegisterProviders("stub")
	runner := newCountedRunner(t, db, &stubAdapter{pages: twoPages}, counters)

	report, err := runner.RunSync(context.Background(), inventory.SyncInput{ConnectionUID: conn.UID, Mode: "full"})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if report.Status != inventory.RunStatusSucceeded {
		t.Fatalf("status = %q, want succeeded", report.Status)
	}
	render := counters.Render()
	// C — one duration observation per run, {connection, mode} labeled.
	if got := renderLineValue(t, render, `inventory_sync_duration_seconds_count{connection="conn-1",mode="full"}`); got != 1 {
		t.Errorf("sync duration count = %v, want 1 after one run", got)
	}
	// D — changes = created+updated (가정 A2): the first run creates all 3.
	if got := renderLineValue(t, render, `inventory_sync_resource_changes_total{connection="conn-1"}`); got != 3 {
		t.Errorf("sync changes = %v, want 3", got)
	}
	// E — a succeeded run must not feed the partial family (no line rendered).
	if strings.Contains(render, "inventory_sync_partial_total") {
		t.Errorf("successful run must not feed the partial family:\n%s", render)
	}

	// Partial run — page 2 rate-limited with prior sightings → partial 가산,
	// duration re-observed, changes contribute nothing.
	partial := &stubAdapter{
		pages: twoPages,
		errs:  []error{nil, &contract.ProviderSignalError{Kind: contract.SignalRateLimited, Message: "429"}},
	}
	preport, err := newCountedRunner(t, db, partial, counters).
		RunSync(context.Background(), inventory.SyncInput{ConnectionUID: conn.UID, Mode: "full"})
	if err != nil {
		t.Fatalf("partial run: %v", err)
	}
	if preport.Status != inventory.RunStatusPartial {
		t.Fatalf("partial status = %q, want partial", preport.Status)
	}
	render = counters.Render()
	if got := renderLineValue(t, render, `inventory_sync_duration_seconds_count{connection="conn-1",mode="full"}`); got != 2 {
		t.Errorf("sync duration count = %v, want 2 after the partial run", got)
	}
	if !strings.Contains(render, `inventory_sync_partial_total{connection="conn-1"} 1`) {
		t.Errorf("render missing the partial line with value 1:\n%s", render)
	}
	if got := renderLineValue(t, render, `inventory_sync_resource_changes_total{connection="conn-1"}`); got != 3 {
		t.Errorf("sync changes after partial = %v, want still 3 (partial contributes nothing)", got)
	}
}

// TestReconcileObservesStaleByKind — §18.2 K: a stale_candidate transition
// counts by kind (가정 A3), the first absence (counter-only) and the seen
// resources never do.
func TestReconcileObservesStaleByKind(t *testing.T) {
	db := newInventoryDB(t)
	conn, _ := seedConnection(t, db, "conn-1")

	counters := metrics.New()
	counters.RegisterProviders("stub")

	nodesOnly := []contract.DiscoverPage{{Resources: []contract.DiscoveredResource{
		res("urn:t:node-1", "node-1", "orchestration.node", "", contract.JSONMap{"healthState": "healthy"}, nil),
		res("urn:t:node-2", "node-2", "orchestration.node", "", contract.JSONMap{"healthState": "degraded"}, nil),
	}}}

	run := func(d *stubAdapter) {
		t.Helper()
		if _, err := newCountedRunner(t, db, d, counters).
			RunSync(context.Background(), inventory.SyncInput{ConnectionUID: conn.UID, Mode: "full"}); err != nil {
			t.Fatalf("run: %v", err)
		}
	}

	// gen 1 — both nodes sighted; gen 2 — workload-1's first absence records
	// only the ladder counter: no stale line yet.
	run(&stubAdapter{pages: twoPages})
	run(&stubAdapter{pages: nodesOnly})
	if render := counters.Render(); strings.Contains(render, "resource_stale_total") {
		t.Errorf("1st absence must not feed the stale family:\n%s", render)
	}
	// gen 3 — the 2nd consecutive absence fires stale_candidate once, labeled
	// by the absent resource's kind.
	run(&stubAdapter{pages: nodesOnly})
	render := counters.Render()
	if !strings.Contains(render, `resource_stale_total{kind="orchestration.workload"} 1`) {
		t.Errorf("render missing the stale-by-kind line:\n%s", render)
	}
	if strings.Contains(render, `kind="orchestration.node"`) {
		t.Errorf("re-sighted nodes must never count as stale:\n%s", render)
	}
}

// renderLineValue — "{labels} value" 형태 렌더 라인의 마지막 필드를 수치로
// 파싱한다(값 단얫 전용 — 존재하지 않으면 테스트 실패).
func renderLineValue(t *testing.T, render, key string) float64 {
	t.Helper()
	for _, line := range strings.Split(render, "\n") {
		if strings.HasPrefix(line, key+" ") {
			fields := strings.Fields(line)
			v, err := strconv.ParseFloat(fields[len(fields)-1], 64)
			if err != nil {
				t.Fatalf("parse %q: %v", line, err)
			}
			return v
		}
	}
	t.Fatalf("render missing %q:\n%s", key, render)
	return 0
}
