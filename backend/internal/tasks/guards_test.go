// T28–T31 (plan §6, PR 18) — the guard suite: idempotent replay (§13.4/N6),
// approval transitions (§13.3), per-resource active uniqueness (§13.4/N11),
// and the step0003 guard shape itself (r2 C · E-3h · W-5).
package tasks

import (
	"context"
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/migrate"
	"ops-admin/backend/internal/infra/model"
	"ops-admin/backend/internal/infra/registry"
)

// newGuardedFixture builds an engine over the REAL production schema:
// migrate.Run applies steps 0000–0003, so the model-tag idempotency unique
// (uq_provider_task_idempotency) AND the step0003 raw-DDL guards (the
// active_flag generated column + uq_provider_task_resource_active) are all in
// force — unlike newTestDB's bare AutoMigrate, which creates no raw DDL.
func newGuardedFixture(t *testing.T, requiresApproval bool) *engineFixture {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open in-memory sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("access pooled handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := migrate.Run(context.Background(), db); err != nil {
		t.Fatalf("migrate.Run (steps 0000–0003): %v", err)
	}
	reg := registry.New()
	def := testOperation("fake.workload.restart")
	def.RequiresApproval = requiresApproval
	if err := reg.RegisterOperation(def); err != nil {
		t.Fatalf("seed operation: %v", err)
	}
	return &engineFixture{db: db, reg: reg, eng: NewEngine(db, reg, testConfig())}
}

// T28 — TestIdempotencyReplayReturnsSameTask: the same idempotency key
// submitted twice returns the SAME task with replayed=true and writes nothing
// (§13.4). The full N6 cycle — provider-double execution count — is PR 19's
// idempotency_suite; here the contract is the submit-level replay.
func TestIdempotencyReplayReturnsSameTask(t *testing.T) {
	f := newGuardedFixture(t, false)
	ctx := context.Background()

	first, replayed, err := f.eng.Submit(ctx, SubmitInput{
		OperationName:  "fake.workload.restart",
		ResourceUID:    "res-idem-1",
		Payload:        contract.JSONMap{"n": 1},
		IdempotencyKey: "idem-key-1",
	})
	if err != nil {
		t.Fatalf("first submit: %v", err)
	}
	if replayed {
		t.Error("first submit replayed = true, want false")
	}
	if first.IdempotencyKey == nil || *first.IdempotencyKey != "idem-key-1" {
		t.Fatalf("first submit IdempotencyKey = %v, want idem-key-1", first.IdempotencyKey)
	}

	// Same key, same resource — both uniques trip; the replay must win over
	// resource_busy (N6: the caller asked "has this already run?", not "is the
	// resource free?").
	second, replayed, err := f.eng.Submit(ctx, SubmitInput{
		OperationName:  "fake.workload.restart",
		ResourceUID:    "res-idem-1",
		Payload:        contract.JSONMap{"n": 1},
		IdempotencyKey: "idem-key-1",
	})
	if err != nil {
		t.Fatalf("replay submit: %v", err)
	}
	if !replayed {
		t.Error("replay submit replayed = false, want true (§13.4)")
	}
	if second.UID != first.UID || second.ID != first.ID {
		t.Errorf("replay returned task %s (id %d), want the original %s (id %d)",
			second.UID, second.ID, first.UID, first.ID)
	}
	var count int64
	f.db.Model(&model.ProviderTask{}).Where("uid = ?", first.UID).Count(&count)
	if count != 1 {
		t.Errorf("provider_task rows for uid %s = %d, want 1 (N6 same task)", first.UID, count)
	}
	// The replay is a pure recovery — no second created/approved event burst.
	events := eventsOf(t, f.db, first.ID)
	if len(events) != 1 || events[0].Type != TaskEventCreated {
		t.Errorf("events after replay = %v, want just [created] (replay writes nothing)", eventTypes(events))
	}

	// A different key is a different task — replay never bleeds across keys.
	third, replayed, err := f.eng.Submit(ctx, SubmitInput{
		OperationName:  "fake.workload.restart",
		ResourceUID:    "res-idem-2",
		Payload:        contract.JSONMap{"n": 3},
		IdempotencyKey: "idem-key-2",
	})
	if err != nil {
		t.Fatalf("different-key submit: %v", err)
	}
	if replayed || third.UID == first.UID {
		t.Errorf("different key: replayed=%v sameUID=%v, want false/false", replayed, third.UID == first.UID)
	}

	// An empty key means "no idempotency" — never a replay (§3.6 contract).
	fourth, replayed, err := f.eng.Submit(ctx, SubmitInput{
		OperationName: "fake.workload.restart",
		ResourceUID:   "res-idem-3",
	})
	if err != nil {
		t.Fatalf("no-key submit: %v", err)
	}
	if replayed {
		t.Error("no-key submit replayed = true, want false")
	}
	if fourth.IdempotencyKey != nil {
		t.Errorf("no-key submit IdempotencyKey = %v, want nil", fourth.IdempotencyKey)
	}
}

// T29 — TestApproveRejectTransitions: §13.3 — a submit of an
// approval-requiring definition lands at awaiting_approval with the approval
// posture snapshotted; Approve moves it awaiting_approval→queued, Reject moves
// it to the cancelled TERMINAL (never a 'rejected' Status, r2 A) with
// ApprovalStatus='rejected', the Approver/ApprovalAt columns set, and the
// resource freed. Acting on a task that is not awaiting approval is refused.
func TestApproveRejectTransitions(t *testing.T) {
	f := newGuardedFixture(t, true)
	ctx := context.Background()

	task, _, err := f.eng.Submit(ctx, SubmitInput{
		OperationName: "fake.workload.restart",
		ResourceUID:   "res-approve-1",
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if task.Status != TaskStatusAwaitingApproval {
		t.Fatalf("submitted status = %q, want awaiting_approval (§13.3)", task.Status)
	}
	if !task.RequiresApproval {
		t.Error("RequiresApproval = false, want the def.RequiresApproval snapshot (true)")
	}
	if task.ApprovalStatus != ApprovalStatusNotRequired {
		t.Errorf("ApprovalStatus at creation = %q, want %q (no decision yet — pending lives in Status)", task.ApprovalStatus, ApprovalStatusNotRequired)
	}

	// Approve leg: awaiting_approval → queued.
	if err := f.eng.Approve(ctx, task.UID, "alice"); err != nil {
		t.Fatalf("Approve: %v", err)
	}
	approved := reloadTask(t, f.db, task.ID)
	if approved.Status != TaskStatusQueued {
		t.Errorf("post-Approve status = %q, want queued (§13.3)", approved.Status)
	}
	if approved.ApprovalStatus != ApprovalStatusApproved {
		t.Errorf("post-Approve ApprovalStatus = %q, want %q", approved.ApprovalStatus, ApprovalStatusApproved)
	}
	if approved.Approver != "alice" {
		t.Errorf("post-Approve Approver = %q, want alice", approved.Approver)
	}
	if approved.ApprovalAt == nil {
		t.Error("post-Approve ApprovalAt is nil, want set")
	}
	events := eventsOf(t, f.db, task.ID)
	if len(events) != 2 || events[1].Type != TaskEventApproved || events[1].Actor != "alice" {
		t.Fatalf("events after Approve = %v, want [created approved] with actor alice", eventTypes(events))
	}

	// The approved (queued) task is no longer awaiting — both verbs refuse.
	if err := f.eng.Approve(ctx, task.UID, "alice"); !errors.Is(err, ErrNotAwaitingApproval) {
		t.Errorf("second Approve err = %v, want ErrNotAwaitingApproval", err)
	}
	if err := f.eng.Reject(ctx, task.UID, "bob"); !errors.Is(err, ErrNotAwaitingApproval) {
		t.Errorf("Reject on queued task err = %v, want ErrNotAwaitingApproval", err)
	}

	// Reject leg — a fresh task on its own resource.
	f2 := newGuardedFixture(t, true)
	rejTask, _, err := f2.eng.Submit(ctx, SubmitInput{
		OperationName: "fake.workload.restart",
		ResourceUID:   "res-reject-1",
	})
	if err != nil {
		t.Fatalf("submit (reject leg): %v", err)
	}
	if err := f2.eng.Reject(ctx, rejTask.UID, "bob"); err != nil {
		t.Fatalf("Reject: %v", err)
	}
	rejected := reloadTask(t, f2.db, rejTask.ID)
	if rejected.Status != TaskStatusCancelled {
		t.Fatalf("post-Reject status = %q, want cancelled (rejection is terminal, §13.3; 'rejected' is never a Status, r2 A)", rejected.Status)
	}
	if !IsTerminalTaskStatus(rejected.Status) {
		t.Error("post-Reject status is not terminal — rejection must land in the 4-terminal set (r2 A)")
	}
	if rejected.ApprovalStatus != ApprovalStatusRejected {
		t.Errorf("post-Reject ApprovalStatus = %q, want %q", rejected.ApprovalStatus, ApprovalStatusRejected)
	}
	if rejected.Approver != "bob" {
		t.Errorf("post-Reject Approver = %q, want bob", rejected.Approver)
	}
	if rejected.ApprovalAt == nil {
		t.Error("post-Reject ApprovalAt is nil, want set")
	}
	if rejected.FinishedAt == nil {
		t.Error("post-Reject FinishedAt is nil, want set (terminal)")
	}
	events = eventsOf(t, f2.db, rejTask.ID)
	if len(events) != 2 || events[1].Type != TaskEventRejected || events[1].Actor != "bob" {
		t.Fatalf("events after Reject = %v, want [created rejected] with actor bob", eventTypes(events))
	}

	// Rejection is terminal — Approve afterwards is refused.
	if err := f2.eng.Approve(ctx, rejTask.UID, "alice"); !errors.Is(err, ErrNotAwaitingApproval) {
		t.Errorf("Approve after Reject err = %v, want ErrNotAwaitingApproval", err)
	}
	// ...and the terminal transition freed the resource for a new mutation.
	if _, _, err := f2.eng.Submit(ctx, SubmitInput{
		OperationName: "fake.workload.restart",
		ResourceUID:   "res-reject-1",
	}); err != nil {
		t.Fatalf("resubmit after rejected task freed the resource: %v", err)
	}

	// An unknown UID is a hard error, never a silent no-op.
	if err := f2.eng.Approve(ctx, "no-such-uid", "alice"); err == nil {
		t.Error("Approve on unknown UID = nil error, want hard error")
	}
	if err := f2.eng.Reject(ctx, "no-such-uid", "bob"); err == nil {
		t.Error("Reject on unknown UID = nil error, want hard error")
	}
}

// T30 — TestResourceUniquenessBlocksSecondActive: while one task is active on
// a resource, a second mutating submit for the same resource fails immediately
// with resource_busy (§13.4/N11) — and only for that resource.
func TestResourceUniquenessBlocksSecondActive(t *testing.T) {
	f := newGuardedFixture(t, false)
	ctx := context.Background()

	first, _, err := f.eng.Submit(ctx, SubmitInput{
		OperationName: "fake.workload.restart",
		ResourceUID:   "res-busy-1",
	})
	if err != nil {
		t.Fatalf("first submit: %v", err)
	}
	if first.Status != TaskStatusQueued {
		t.Fatalf("first submit status = %q, want queued", first.Status)
	}

	_, _, err = f.eng.Submit(ctx, SubmitInput{
		OperationName: "fake.workload.restart",
		ResourceUID:   "res-busy-1",
	})
	if !errors.Is(err, ErrResourceBusy) {
		t.Fatalf("second active submit err = %v, want ErrResourceBusy (N11)", err)
	}
	var count int64
	f.db.Model(&model.ProviderTask{}).Count(&count)
	if count != 1 {
		t.Errorf("provider_task rows = %d after busy rejection, want 1", count)
	}

	// A different resource is unaffected.
	if _, _, err := f.eng.Submit(ctx, SubmitInput{
		OperationName: "fake.workload.restart",
		ResourceUID:   "res-busy-2",
	}); err != nil {
		t.Fatalf("different-resource submit: %v", err)
	}

	// The busy block holds across every non-terminal status — the guard is
	// active_flag, not a status list.
	if _, _, err := f.eng.Submit(ctx, SubmitInput{
		OperationName: "fake.workload.restart",
		ResourceUID:   "res-busy-1",
	}); !errors.Is(err, ErrResourceBusy) {
		t.Errorf("third submit on busy resource err = %v, want ErrResourceBusy", err)
	}
}

// T31 — TestActiveFlagGeneratedColumn: the step0003 shape — active_flag is a
// generated column (visible to pragma_table_xinfo, hidden from
// pragma_table_info — E-3h), both guard uniques exist by name, terminal rows
// are NULL-excluded so the resource can go active again, and re-running the
// W-5 postlude is a no-op that leaves exactly one guard set.
func TestActiveFlagGeneratedColumn(t *testing.T) {
	f := newGuardedFixture(t, false)
	ctx := context.Background()

	// (a) generated column — asserted through pragma_table_xinfo because
	// generated columns are invisible to pragma_table_info ([실측 E-3h]).
	var xcol int64
	if err := f.db.Raw(
		"SELECT COUNT(*) FROM pragma_table_xinfo('provider_task') WHERE name = 'active_flag'",
	).Scan(&xcol).Error; err != nil {
		t.Fatalf("pragma_table_xinfo: %v", err)
	}
	if xcol != 1 {
		t.Fatalf("pragma_table_xinfo('provider_task') active_flag rows = %d, want 1 (generated column missing)", xcol)
	}
	// hidden ≥ 2 is the generated-column marker in xinfo (2=VIRTUAL, 3=STORED;
	// 0 would be a plain column — the guard must be computed, never writable).
	var generated int64
	if err := f.db.Raw(
		"SELECT COUNT(*) FROM pragma_table_xinfo('provider_task') WHERE name = 'active_flag' AND hidden >= 2",
	).Scan(&generated).Error; err != nil {
		t.Fatalf("pragma_table_xinfo hidden flag: %v", err)
	}
	if generated != 1 {
		t.Fatalf("active_flag generated-marker rows = %d, want 1 (plain column would defeat the invariant)", generated)
	}

	// (b) both guard uniques exist by name (T-6 index-name contract).
	assertIndexCount := func(index string) {
		t.Helper()
		var n int64
		if err := f.db.Raw(
			"SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND tbl_name = 'provider_task' AND name = ?",
			index,
		).Scan(&n).Error; err != nil {
			t.Fatalf("sqlite_master index %s: %v", index, err)
		}
		if n != 1 {
			t.Errorf("index %s count = %d, want exactly 1", index, n)
		}
	}
	assertIndexCount("uq_provider_task_resource_active")
	assertIndexCount("uq_provider_task_idempotency")

	// (c) NULL exclusion semantics: terminal rows coexist on one resource and
	// free it for a new active task. Direct row surgery keeps this a DB-level
	// invariant test, independent of engine paths (T30 owns the engine leg).
	first, _, err := f.eng.Submit(ctx, SubmitInput{
		OperationName: "fake.workload.restart",
		ResourceUID:   "res-gen-1",
	})
	if err != nil {
		t.Fatalf("active submit: %v", err)
	}
	if err := f.db.Model(&model.ProviderTask{}).Where("id = ?", first.ID).
		Update("status", TaskStatusSucceeded).Error; err != nil {
		t.Fatalf("terminalize first: %v", err)
	}
	secondTerminal := model.ProviderTask{
		UID:              "gen-terminal-2",
		OperationName:    "fake.workload.restart",
		OperationVersion: "1",
		ResourceUID:      "res-gen-1",
		Status:           TaskStatusFailed, // second terminal row on the same resource
		MaxAttempts:      1,
	}
	if err := f.db.Create(&secondTerminal).Error; err != nil {
		t.Fatalf("second terminal row on res-gen-1: %v (NULL rows must be unique-excluded, A5)", err)
	}
	// Reactivation after terminalization — the STORED value recomputes on UPDATE.
	if _, _, err := f.eng.Submit(ctx, SubmitInput{
		OperationName: "fake.workload.restart",
		ResourceUID:   "res-gen-1",
	}); err != nil {
		t.Fatalf("resubmit after terminal rows: %v", err)
	}

	// (d) W-5 postlude: a later step re-applying the guard DDL is a no-op that
	// leaves exactly one guard set (r2 C).
	if err := migrate.EnsureProviderTaskGuards(f.db); err != nil {
		t.Fatalf("postlude re-application (W-5): %v", err)
	}
	assertIndexCount("uq_provider_task_resource_active")
	if err := f.db.Raw(
		"SELECT COUNT(*) FROM pragma_table_xinfo('provider_task') WHERE name = 'active_flag'",
	).Scan(&xcol).Error; err != nil {
		t.Fatalf("pragma_table_xinfo after postlude: %v", err)
	}
	if xcol != 1 {
		t.Errorf("active_flag rows after postlude re-application = %d, want 1 (no duplicate column)", xcol)
	}
}
