//go:build e2e

// Package slicea is the N16 Go trace E2E of v2 Phase 3 Slice A (plan r2 §5
// Phase G1): §25's "One Kubernetes mutation end to end" against a real kind
// cluster, with process-level crash injection the Playwright lane (G2) never
// attempts (J9 two-layer split — Go owns process control and kill timing).
//
// Scope of this file (TestMain):
//
//   - prerequisite gate — the suite SKIPS (exit 0) when the kind context, the
//     seeded workload or dev MySQL is missing, and never compiles into the
//     ordinary `go test ./...` run (build tag e2e; CI-less environments skip)
//   - dedicated schema lifecycle (VK-7/A11): ops_admin_p3e2e is created here,
//     dropped on green and kept on red (dirty runbook — v2-phase1/k8s-fixture)
//   - backend binary build + child-process harness assembly (N16: "자식 프로세스
//     백엔드 바이너리")
//
// The trace itself lives in trace_test.go; the harness in harness_test.go.
//
// Target cluster is the kind 2호기 v2-p3 ONLY (plan §0.2a — the phase2 gate
// cluster v2-p2 stays untouched, 보존 제약 #11). Every kubectl call pins
// --context, and the executor's detail server URL is asserted loopback (VK-3)
// in the audit observation — an accidental cross-cluster fire shows up as a
// wrong connection UID long before it can mutate v2-p2.
package slicea

import (
	"fmt"
	"os"
	"testing"
)

// h is the assembled-once trace environment (see harness_test.go).
var h *harness

// TestMain assembles the harness once, runs the ordered trace, then releases
// the dedicated schema. A nil harness with a reason means "prerequisites
// missing" — that is a skip, not a failure.
func TestMain(m *testing.M) {
	var skipReason error
	h, skipReason = assembleHarness()
	if h == nil {
		fmt.Printf("slicea: SKIP — %s\n", skipReason)
		os.Exit(0)
	}
	code := m.Run()
	h.release(code)
	os.Exit(code)
}

// TestSliceATrace is the single ordered entry: environment boot (child
// process, registration, backfill+sync, discovery) then the legs in
// declaration order — the crash leg depends on the happy leg's terminal state
// releasing the per-resource active unique (§13.4).
func TestSliceATrace(t *testing.T) {
	h.bootTrace(t)
	t.Run("happy path", testHappyPath)
	t.Run("crash recovery", testCrashRecovery)
}
