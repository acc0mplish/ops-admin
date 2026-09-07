//go:build e2e

package slicea

// The §25 Slice A trace legs (plan r2 §6 e2e(Go), 게이트 ①②③ 증명):
//
//	happy path    — plan(동결 restartedAt) → execute(201) → approve → 종단
//	                succeeded + 클러스터 단얫 3종(§0.1) + 감사 task_uid 조인
//	                (claim 8 합집합) + refreshed observation(J6) + 멱등 재생
//	                200+header(§13.4)
//	crash recovery — running+패치 적출 관찰 → SIGKILL → lease+grace 경과 대기
//	                → 재기동 → reaper_requeued → attempt 2 재실행 → 클러스터에서
//	                이중 restart 부재 단얫 (J1 동결값 멱등 — 게이트 ②)
//
// Every cluster-side number comes from the L3 instrument in harness_test.go:
// generation 증분은 2회 스냅샷의 차, 신규 RS는 creationTimestamp 기준, 어느
// 절대값 기준도 아니다 (재시딩·재실행 누적에 견고).

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

// skewMargin absorbs kube-apiserver's second-granular creationTimestamp
// truncation against the test host clock (same host — kind).
const skewMargin = 2 * time.Second

func testHappyPath(t *testing.T) {
	before := h.deployState(t)
	legStart := time.Now().UTC().Add(-skewMargin)

	// 1. plan — stateless freeze (J1): the response issues restartedAt and
	//    changes nothing on the cluster.
	plan := h.planRestart(t)
	assertPlanContract(t, plan)
	frozen, _ := plan["restartedAt"].(string)
	if mid := h.deployState(t); mid.Generation != before.Generation || mid.RestartedAt != before.RestartedAt {
		t.Fatalf("plan mutated the cluster: before gen=%d ann=%q, after gen=%d ann=%q",
			before.Generation, before.RestartedAt, mid.Generation, mid.RestartedAt)
	}

	// 2. execute — first submit 201, task lands awaiting_approval (J8).
	idempotencyKey := "slicea-happy-" + time.Now().Format("20060102T150405.000000000")
	status, replayed, uid := h.executeRestart(t, frozen, idempotencyKey)
	if status != 201 || replayed {
		t.Fatalf("execute first submit: want 201 not-replayed, got %d replayed=%v", status, replayed)
	}
	fmt.Printf("slicea: [happy] task %s (frozen restartedAt %s)\n", uid, frozen)
	task := h.getTask(t, uid)
	if s, _ := task["status"].(string); s != "awaiting_approval" {
		t.Fatalf("post-execute status: want awaiting_approval, got %q (task %+v)", s, task)
	}
	if ra, _ := task["requiresApproval"].(bool); !ra {
		t.Fatal("task.requiresApproval = false — J8 posture missing")
	}
	if payload, _ := task["payload"].(map[string]any); payload["restartedAt"] != frozen {
		t.Fatalf("task payload did not freeze the plan value: payload=%+v want restartedAt=%s", payload, frozen)
	}

	// 3. approve → queued → claim → rollout → converged poll.
	h.approveTask(t, uid)
	waitFor(t, "task "+uid+" terminal succeeded", terminalBudget, 250*time.Millisecond, func() (bool, string) {
		current := h.getTask(t, uid)
		status, _ := current["status"].(string)
		return isTerminal(status), fmt.Sprintf("status=%s", status)
	})
	final := h.getTask(t, uid)
	if s, _ := final["status"].(string); s != "succeeded" {
		errorCode, _ := final["errorCode"].(string)
		t.Fatalf("terminal status: want succeeded, got %q errorCode=%q task=%+v", s, errorCode, final)
	}
	if attempts, _ := final["attemptCount"].(float64); attempts != 1 {
		t.Fatalf("happy leg must single-attempt, got attemptCount=%v", attempts)
	}

	// 4. 클러스터 단얫 3종 (§0.1 — 게이트 ②의 측정 기구).
	after := h.deployState(t)
	assertRolloutTrace(t, "happy", before, after, frozen, legStart, 1)

	// 5. events: the §13.5 chain in write order.
	assertEventTypes(t, uid, []string{"created", "approved", "claimed", "succeeded"}, nil)

	// 6. 감사 (게이트 ③ — claim 8): request+terminal rows joined on task_uid,
	//    §18.1 fields asserted over the UNION with conditional non-null.
	assertAuditTrail(t, uid, true)

	// 7. refreshed observation (J6): the terminal hook runs one whole-connection
	//    sync AFTER the terminal commit. Caveat the assertion encodes (F-9
	//    adjacency): the Phase 2 normalizer's projection for workloads carries
	//    only name/labels/image/replicas — generation and the restartedAt
	//    annotation are invisible to it, and the observation hash-dedup then
	//    suppresses a new row for a rollout. The provable refresh signal is the
	//    post-terminal InventorySyncRun + a steady-state read through the API
	//    (readyReplicas==replicas==2). Enriching the projection (generation,
	//    template annotations) is a normalizer change — Phase 4 handoff.
	finalTask := final
	startedAt, _ := finalTask["startedAt"].(string)
	taskStart, err := time.Parse(time.RFC3339Nano, startedAt)
	if err != nil {
		t.Fatalf("task startedAt not parseable: %q", startedAt)
	}
	waitFor(t, "terminal-hook observation refresh (post-terminal sync run)", 30*time.Second, 500*time.Millisecond, func() (bool, string) {
		runs := h.syncRunsSince(t, taskStart)
		_, obs := h.getResource(t)
		replicas, _ := dig(obs, "normalized", "replicas").(float64)
		ready, _ := dig(obs, "normalized", "readyReplicas").(float64)
		return runs >= 1 && replicas == 2 && ready == 2,
			fmt.Sprintf("syncRunsAfterStart=%d normalized replicas=%v readyReplicas=%v", runs, replicas, ready)
	})

	// 8. 멱등 재생 (§13.4 r2 L2): same key → 200 + Idempotency-Replayed + same task.
	status, replayed, replayUID := h.executeRestart(t, frozen, idempotencyKey)
	if status != 200 || !replayed || replayUID != uid {
		t.Fatalf("idempotent replay: want 200 replayed=true uid=%s, got %d replayed=%v uid=%s",
			uid, status, replayed, replayUID)
	}
	fmt.Printf("slicea: [happy] idempotent replay 200 + Idempotency-Replayed: true (uid %s)\n", replayUID)
}

func testCrashRecovery(t *testing.T) {
	before := h.deployState(t) // == happy leg's after (gen already +1)
	legStart := time.Now().UTC().Add(-skewMargin)

	// 1. second mutation on the same resource (the happy leg is terminal, so
	//    the per-resource active unique is free — §13.4).
	plan := h.planRestart(t)
	frozen, _ := plan["restartedAt"].(string)
	idempotencyKey := "slicea-crash-" + time.Now().Format("20060102T150405.000000000")
	status, replayed, uid := h.executeRestart(t, frozen, idempotencyKey)
	if status != 201 || replayed {
		t.Fatalf("execute first submit: want 201 not-replayed, got %d replayed=%v", status, replayed)
	}
	h.approveTask(t, uid)
	fmt.Printf("slicea: [crash] task %s (frozen restartedAt %s)\n", uid, frozen)

	// 2. deterministic kill window: task running AND attempt 1's patch already
	//    on the cluster (generation bumped). minReadySeconds=10 keeps the
	//    rollout unconverged — the task cannot have terminated yet.
	waitFor(t, "task running with attempt-1 patch applied", 30*time.Second, 50*time.Millisecond, func() (bool, string) {
		current := h.getTask(t, uid)
		s, _ := current["status"].(string)
		state := h.deployState(t)
		return s == "running" && state.Generation == before.Generation+1 && state.RestartedAt == frozen,
			fmt.Sprintf("task=%s gen=%d(->%d) ann=%q", s, state.Generation, before.Generation+1, state.RestartedAt)
	})

	// 3. crash injection — SIGKILL mid-rollout, no drain.
	h.killBackend9(t)

	// 4. the durable row is the only survivor: assert attempt 1 held the lease,
	//    then wait PAST lease+grace before rebooting (H1 — the reaper's
	//    requeue condition lease < now-grace must already hold at boot, so the
	//    first reap tick requeues deterministically).
	rowStatus, lease, attempts := h.taskRow(t, uid)
	if rowStatus != "running" || lease == nil {
		t.Fatalf("post-kill row: want running with a lease, got status=%q lease=%v", rowStatus, lease)
	}
	if attempts != 1 {
		t.Fatalf("post-kill attempt count: want 1 (killed mid-attempt), got %d", attempts)
	}
	reapReadyAt := lease.Add(time.Duration(engineGraceMS)*time.Millisecond + 2*time.Second)
	if wait := time.Until(reapReadyAt); wait > 0 {
		fmt.Printf("slicea: [crash] waiting out lease+grace (%s) before reboot\n", wait.Truncate(time.Millisecond))
		time.Sleep(wait)
	}

	// 5. reboot: same binary, same config, same env — the recovery is entirely
	//    durable-state driven.
	genAtBoot := h.deployState(t).Generation // attempt 2 must not move this (J1)
	h.startBackend(t)

	// 6. recovery: reaper requeues, attempt 2 re-executes the byte-identical
	//    patch, the converged poll terminates the task.
	waitFor(t, "task "+uid+" recovered to succeeded", 90*time.Second, 250*time.Millisecond, func() (bool, string) {
		current := h.getTask(t, uid)
		s, _ := current["status"].(string)
		return isTerminal(s), fmt.Sprintf("status=%s", s)
	})
	final := h.getTask(t, uid)
	if s, _ := final["status"].(string); s != "succeeded" {
		errorCode, _ := final["errorCode"].(string)
		t.Fatalf("crash leg terminal: want succeeded, got %q errorCode=%q task=%+v", s, errorCode, final)
	}
	if attempts, _ := final["attemptCount"].(float64); attempts != 2 {
		t.Fatalf("crash leg must recover on attempt 2, got attemptCount=%v", attempts)
	}

	// 7. 게이트 ② — the cluster itself testifies there was NO double restart:
	//    attempt 2's byte-identical patch was a no-op.
	assertEventTypes(t, uid, []string{"created", "approved", "claimed"}, []string{"reaper_requeued", "claimed", "succeeded"})
	assertAuditTrail(t, uid, true)
	after := h.deployState(t)
	if after.Generation != genAtBoot {
		t.Fatalf("DOUBLE RESTART — generation moved across attempt 2: boot=%d final=%d (J1 violated)", genAtBoot, after.Generation)
	}
	assertRolloutTrace(t, "crash", before, after, frozen, legStart, 1)
	fmt.Printf("slicea: [crash] J1 proof — attempt 2 patch was a no-op (generation %d stable across reboot)\n", genAtBoot)
}

// --- shared assertions ------------------------------------------------------

// assertRolloutTrace checks the three cluster-side assertions (§0.1):
//
//	① generation increment == wantDelta (two-snapshot diff — absolute-free)
//	② ReplicaSets created after legStart == wantDelta (creationTimestamp basis)
//	③ live pod-template restartedAt == frozen (the J1 anchor)
func assertRolloutTrace(t *testing.T, leg string, before, after deployState, frozen string, legStart time.Time, wantDelta int64) {
	t.Helper()
	if delta := after.Generation - before.Generation; delta != wantDelta {
		t.Fatalf("[%s] generation increment: want %d, got %d (before=%d after=%d)",
			leg, wantDelta, delta, before.Generation, after.Generation)
	}
	if after.RestartedAt != frozen {
		t.Fatalf("[%s] pod-template restartedAt: want frozen %q, cluster says %q", leg, frozen, after.RestartedAt)
	}
	newRS := replicaSetsCreatedAfter(h.replicaSets(t), legStart)
	if int64(len(newRS)) != wantDelta {
		t.Fatalf("[%s] new ReplicaSets since leg start: want %d, got %d (%s)",
			leg, wantDelta, len(newRS), rsNames(newRS))
	}
	fmt.Printf("slicea: [%s] cluster assertions OK — generation %d→%d (+%d), new RS %s, restartedAt==frozen\n",
		leg, before.Generation, after.Generation, after.Generation-before.Generation, rsNames(newRS))
}

// assertPlanContract pins the §3.4 plan response vocabulary.
func assertPlanContract(t *testing.T, plan map[string]any) {
	t.Helper()
	want := map[string]any{
		"operation":        restartOperationName,
		"version":          "1",
		"resourceUid":      h.resourceUID,
		"requiresApproval": true,
		"riskLevel":        "medium",
		"permission":       "assets:k8s:workload:restart",
		"policyVersion":    "builtin:default-allow",
	}
	for key, expected := range want {
		if actual, ok := plan[key]; !ok || actual != expected {
			t.Fatalf("plan.%s: want %v, got %v", key, expected, actual)
		}
	}
	frozen, _ := plan["restartedAt"].(string)
	if _, err := time.Parse(time.RFC3339, frozen); err != nil {
		t.Fatalf("plan.restartedAt not RFC3339: %q", frozen)
	}
	if _, ok := plan["resourceRevision"]; !ok {
		t.Fatal("plan response missing resourceRevision key (J7)")
	}
}

// assertEventTypes requires every type in `mustContain` to appear (in order
// for the prefix) and forbids anything outside prefix∪extra.
func assertEventTypes(t *testing.T, uid string, orderedPrefix, extra []string) {
	t.Helper()
	events := h.getTaskEvents(t, uid)
	types := make([]string, 0, len(events))
	for _, event := range events {
		if typ, _ := event["type"].(string); typ != "" {
			types = append(types, typ)
		}
	}
	joined := strings.Join(types, ",")
	if len(types) < len(orderedPrefix) || strings.Join(types[:len(orderedPrefix)], ",") != strings.Join(orderedPrefix, ",") {
		t.Fatalf("task %s event prefix: want [%s], got [%s]", uid, strings.Join(orderedPrefix, ","), joined)
	}
	allowed := map[string]bool{}
	for _, typ := range append(append([]string{}, orderedPrefix...), extra...) {
		allowed[typ] = true
	}
	for _, typ := range types {
		if !allowed[typ] {
			t.Fatalf("task %s unexpected event %q (events: %s)", uid, typ, joined)
		}
	}
	for _, typ := range extra {
		if !strings.Contains(joined, typ) {
			t.Fatalf("task %s missing event %q (events: %s)", uid, typ, joined)
		}
	}
	fmt.Printf("slicea: task %s events: %s\n", uid, joined)
}

// assertAuditTrail — 게이트 ③ (claim 8): the rows joined on task_uid must, as
// a UNION, cover the §18.1 vocabulary (J3: task_uid is the only promoted real
// column; the rest rides v2_context JSON — MySQL normalisation forbids byte
// comparison, so keys are asserted after a Go-side decode — F-8).
//
// Conditional non-null (계획 claim 8 예외 문언): on a succeeded leg error_code
// may be absent; approver/provider_task_id/result_hash are REQUIRED because
// this trace approved and carried a provider handle.
func assertAuditTrail(t *testing.T, uid string, succeeded bool) {
	t.Helper()
	// The terminal row is written by the OnTaskTerminal hook AFTER the
	// terminal commit (J6) — the API can surface 'succeeded' a beat before
	// the hook's full-connection sync finishes. Poll for row completeness.
	waitFor(t, "audit rows (request+terminal) for task "+uid, 30*time.Second, 500*time.Millisecond, func() (bool, string) {
		rows := h.auditRowsForTask(t, uid)
		hasRequest, hasTerminal := false, false
		for _, row := range rows {
			if strings.HasSuffix(row.URL, "/execute") {
				hasRequest = true
			}
			if row.URL == "task://"+uid {
				hasTerminal = true
			}
		}
		return hasRequest && hasTerminal, fmt.Sprintf("rows=%d hasRequest=%v hasTerminal=%v", len(rows), hasRequest, hasTerminal)
	})
	rows := h.auditRowsForTask(t, uid)
	if len(rows) < 2 {
		t.Fatalf("audit rows for task %s: want ≥2 (request+terminal), got %d", uid, len(rows))
	}
	merged := map[string]bool{}
	var requestRow, terminalRow *auditRow
	for i := range rows {
		row := &rows[i]
		if row.URL == "task://"+uid {
			terminalRow = row
		} else if strings.HasSuffix(row.URL, "/execute") {
			requestRow = row
		}
		if row.V2Context == "" {
			continue
		}
		var contextMap map[string]any
		if err := json.Unmarshal([]byte(row.V2Context), &contextMap); err != nil {
			t.Fatalf("audit row %d v2_context not JSON: %v (%s)", row.ID, err, row.V2Context)
		}
		for key, value := range contextMap {
			if value == nil {
				continue
			}
			if text, isString := value.(string); isString && text == "" {
				continue
			}
			merged[key] = true
		}
	}
	if requestRow == nil {
		t.Fatalf("no execute request audit row for task %s (rows: %+v)", uid, rows)
	}
	if terminalRow == nil {
		t.Fatalf("no terminal audit row (task://%s) for task %s", uid, uid)
	}
	if requestRow.Username != "admin" || requestRow.IP == "" || requestRow.Method != "POST" {
		t.Fatalf("request audit row columns: username=%q ip=%q method=%q", requestRow.Username, requestRow.IP, requestRow.Method)
	}

	// §18.1 union — request row + terminal row together must cover everything.
	required := []string{
		// request-row v2_context (§3.5)
		"request_id", "trace_id", "mutating", "operation", "operation_version",
		"policy_version", "provider_connection_uid", "provider_context_uid",
		"resource_uid", "task_uid", "request_hash",
		// terminal-row v2_context (§3.5)
		"result_hash", "approval_status", "approver", "provider_task_id",
	}
	var missing []string
	for _, key := range required {
		if !merged[key] {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("task %s audit union missing §18.1 keys %v (merged: %v)", uid, missing, keysOf(merged))
	}
	if !succeeded && !merged["error_code"] {
		t.Fatalf("failed leg must record error_code in the audit union")
	}
	fmt.Printf("slicea: task %s audit OK — %d rows joined on task_uid (execute %d + terminal %d%s), §18.1 union complete\n",
		uid, len(rows), requestRow.ID, terminalRow.ID, extraRowNote(rows, requestRow.ID, terminalRow.ID))
}

func extraRowNote(rows []auditRow, requestID, terminalID int) string {
	var extras []string
	for _, row := range rows {
		if row.ID != requestID && row.ID != terminalID {
			extras = append(extras, fmt.Sprintf("%d %s", row.ID, row.URL))
		}
	}
	if len(extras) == 0 {
		return ""
	}
	return " + " + strings.Join(extras, ", ")
}

// dig walks a decoded JSON object by keys.
func dig(value any, path ...string) any {
	current := value
	for _, key := range path {
		object, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = object[key]
	}
	return current
}

func isTerminal(status string) bool {
	switch status {
	case "succeeded", "failed", "timed_out", "cancelled":
		return true
	}
	return false
}

func keysOf(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for key := range m {
		out = append(out, key)
	}
	return out
}

func rsNames(items []rsInfo) string {
	names := make([]string, 0, len(items))
	for _, item := range items {
		names = append(names, item.Name)
	}
	return "[" + strings.Join(names, ", ") + "]"
}
