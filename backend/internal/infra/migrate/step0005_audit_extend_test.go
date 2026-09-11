package migrate

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"ops-admin/backend/internal/testutil"
	"ops-admin/backend/model"
)

// newAuditExtendDB boots an in-memory sqlite through the step list including
// step 0005, seeds one pre-extend-style v1 audit row (the columns stay NULL)
// and returns the handle.
func newAuditExtendDB(t *testing.T) *gorm.DB {
	t.Helper()
	testutil.PinSecretKeys(t)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := Run(context.Background(), db); err != nil {
		t.Fatalf("migrate.Run: %v", err)
	}
	return db
}

// TestStep0005AuditExtendNullableColumns pins the J3/F-8 schema contract: the
// four §18.1 columns exist on sys_operation_log, every one of them is NULL
// for a v1-style row written without them, and v2 rows can carry the values
// back.
func TestStep0005AuditExtendNullableColumns(t *testing.T) {
	db := newAuditExtendDB(t)

	// The v1 write path (middleware/operation_log.go) sets none of the new
	// columns — the row must land with NULLs (F-8: no backfill, no defaults).
	v1Row := model.OperationLog{Method: "POST", URL: "/api/v1/k8s/workload/restart", CreatedAt: time.Now()}
	if err := db.Create(&v1Row).Error; err != nil {
		t.Fatalf("create v1-style audit row: %v", err)
	}
	var reloaded model.OperationLog
	if err := db.First(&reloaded, v1Row.ID).Error; err != nil {
		t.Fatal(err)
	}
	if reloaded.TaskUID != nil || reloaded.PolicyVersion != nil || reloaded.Mutating != nil || reloaded.V2Context != nil {
		t.Fatalf("v1 row must keep the extend columns NULL, got %+v", reloaded)
	}

	// A v2 row carries the named triplet plus the JSON context.
	taskUID, policyVersion := "task-uid-1", "builtin:default-allow"
	mutating := true
	context := `{"request_id":"req-1","operation":"k8s.workload.restart"}`
	v2Row := model.OperationLog{Method: "POST", URL: "/api/v2/infra/resources/r/operations/k8s.workload.restart/execute", TaskUID: &taskUID, PolicyVersion: &policyVersion, Mutating: &mutating, V2Context: &context, CreatedAt: time.Now()}
	if err := db.Create(&v2Row).Error; err != nil {
		t.Fatalf("create v2-style audit row: %v", err)
	}
	var joined model.OperationLog
	if err := db.Where("task_uid = ?", taskUID).First(&joined).Error; err != nil {
		t.Fatalf("task_uid join (claim 8's access path): %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(*joined.V2Context), &decoded); err != nil {
		t.Fatalf("v2_context must be JSON-decodable (F-8 — no byte compare): %v", err)
	}
	if decoded["request_id"] != "req-1" {
		t.Fatalf("unexpected v2_context payload: %v", decoded)
	}
}

// TestStep0005ListedLast pins the W-4 append-only step list contract: the
// list grew by exactly one line per PR and the pin advances with each new
// step (0007/drop_k8s_cluster — see step0007_drop_k8s_cluster_test.go).
func TestStep0005ListedLast(t *testing.T) {
	if len(steps) == 0 {
		t.Fatal("step list is empty")
	}
	last := steps[len(steps)-1]
	if last.Version != 7 || last.Name != "drop_k8s_cluster" {
		t.Fatalf("last step is %d/%q, want 7/drop_k8s_cluster", last.Version, last.Name)
	}
}
