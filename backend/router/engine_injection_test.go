package router

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"ops-admin/backend/config"
	"ops-admin/backend/internal/api/v2"
	"ops-admin/backend/internal/infra/compose"
	"ops-admin/backend/internal/infra/migrate"
	inframodel "ops-admin/backend/internal/infra/model"
	"ops-admin/backend/internal/tasks"
	"ops-admin/backend/store"
)

// newEngineLaneFixture boots the router fixture with BOTH schemas: the v1
// AutoMigrate+Seed of newArtifactEngine plus the v2 versioned migrations —
// the injected engine lane writes provider_task rows and the v2 audit
// middleware writes sys_operation_log rows.
func newEngineLaneFixture(t *testing.T) (*config.Config, *gorm.DB) {
	t.Helper()
	t.Setenv("OPS_ADMIN_INITIAL_PASSWORD", "injection-test-password")
	t.Setenv("OPS_ADMIN_JWT_SECRET", "injection-test-jwt-secret")
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := store.AutoMigrate(db); err != nil {
		t.Fatalf("store.AutoMigrate: %v", err)
	}
	if err := store.Seed(db); err != nil {
		t.Fatalf("store.Seed: %v", err)
	}
	if err := migrate.Run(context.Background(), db); err != nil {
		t.Fatalf("migrate.Run: %v", err)
	}
	return &config.Config{}, db
}

// seedQueuedTask plants one cancellable queued task row directly (the engine
// lane's Submit path is not under test here — the wiring is).
func seedQueuedTask(t *testing.T, db *gorm.DB, uid string) {
	t.Helper()
	now := time.Now()
	task := inframodel.ProviderTask{
		UID: uid, OperationName: "k8s.workload.restart", OperationVersion: "1",
		ResourceUID: "res-inject", Status: tasks.TaskStatusQueued,
		MaxAttempts: 2, NextAttemptAt: &now, CallTimeoutSeconds: 30,
		RequiresApproval: false, ApprovalStatus: tasks.ApprovalStatusNotRequired,
	}
	if err := db.Create(&task).Error; err != nil {
		t.Fatal(err)
	}
}

// TestEngineInjectionActivatesCancelThroughRouter is the D2 end-to-end wiring
// proof over the REAL router: the same authenticated POST
// /api/v2/infra/tasks/:uid/cancel answers 503 (ENGINE_CANCEL_UNAVAILABLE)
// when the API is self-assembled without an engine, and 200 — with the task
// terminal-cancelled by Engine.RequestCancel (N6) — once main's assembly
// (stack.BuildEngine + NewInfraAPIWithEngine) is injected. The CancelStarter
// seam activated the moment *tasks.Engine grew RequestCancel (A2 인계);
// nothing in the handler changed.
func TestEngineInjectionActivatesCancelThroughRouter(t *testing.T) {
	cfg, db := newEngineLaneFixture(t)

	seedQueuedTask(t, db, "task-inject-cancel")
	super := replayFindRole(t, db, "super-admin")
	granted := replayRole(t, db, "engine-injection-role")
	if copied := copyRoleGrants(t, db, granted.ID, super.ID); copied == 0 {
		t.Fatal("grant copy produced zero rows")
	}
	replayAdminRole(t, db, 9101, granted.ID)
	token := replaySession(t, db, 9101, "inject-admin")

	// Control leg — nil injection keeps the Phase 2 self-assembly: the route
	// exists, the engine seam does not, the verb degrades 503 (v1 boot
	// surface unchanged — the route inventory golden pins the rest, R11).
	plainEngine, plainSvc := New(cfg, db, nil)
	t.Cleanup(func() { _ = plainSvc.Shutdown(context.Background()) })
	status, body := doRequest(plainEngine, token, http.MethodPost, v2APIPrefix+"/infra/tasks/task-inject-cancel/cancel", nil)
	if status != http.StatusServiceUnavailable || !strings.Contains(body, "ENGINE_CANCEL_UNAVAILABLE") {
		t.Fatalf("engine-less cancel = (%d, %s), want 503 ENGINE_CANCEL_UNAVAILABLE", status, body)
	}
	survived := inframodel.ProviderTask{}
	if err := db.Where("uid = ?", "task-inject-cancel").First(&survived).Error; err != nil {
		t.Fatal(err)
	}
	if survived.Status != tasks.TaskStatusQueued {
		t.Fatalf("degraded cancel mutated the task to %q, want still queued", survived.Status)
	}

	// Injected leg — main's M9 assembly (startEngineLane's shape): compose
	// once → BuildEngine (nil audit writer: the cancel leg's terminal is out
	// of OnTaskTerminal's scope — J6 — so no audit row is expected here) →
	// NewInfraAPIWithEngine.
	stack, err := compose.Build(db)
	if err != nil {
		t.Fatalf("compose.Build: %v", err)
	}
	taskEngine := stack.BuildEngine(tasks.Config{WorkerID: "inject-test", PollInterval: time.Hour, LeaseSeconds: 30, ReaperGrace: time.Second}, nil)
	api := v2.NewInfraAPIWithEngine(db, stack.Registry, taskEngine)
	injectedEngine, injectedSvc := New(cfg, db, api)
	t.Cleanup(func() { _ = injectedSvc.Shutdown(context.Background()) })

	status, body = doRequest(injectedEngine, token, http.MethodPost, v2APIPrefix+"/infra/tasks/task-inject-cancel/cancel", nil)
	if status != http.StatusOK {
		t.Fatalf("injected cancel returned %d, want 200: %s", status, body)
	}
	if !strings.Contains(body, `"status":"cancelled"`) {
		t.Fatalf("cancel response does not carry the terminal: %s", body)
	}
	cancelled := inframodel.ProviderTask{}
	if err := db.Where("uid = ?", "task-inject-cancel").First(&cancelled).Error; err != nil {
		t.Fatal(err)
	}
	if cancelled.Status != tasks.TaskStatusCancelled || cancelled.FinishedAt == nil {
		t.Fatalf("task = (%q, finished=%v), want (cancelled, set) — RequestCancel ran through the injected seam", cancelled.Status, cancelled.FinishedAt)
	}
	if !cancelled.CancelRequested {
		t.Error("cancel_requested flag not persisted through the HTTP verb (§13.4)")
	}
}
