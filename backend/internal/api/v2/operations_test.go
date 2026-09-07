// Package v2 operation tests (plan D1 / §6 api tier): the §16.2 mutation
// contract — plan freezes the restartedAt value and changes no state, execute
// requires the Idempotency-Key header and replays (200 + Idempotency-Replayed)
// the same task, policy evaluation failures reject 403 fail-closed (VK-4),
// resource conflicts answer 409, approval verbs answer 409 off
// awaiting_approval, and the §18.1 audit rows land (request middleware +
// terminal helper) with hash/code-only request summaries (VK-11).
//
// The file is an INTERNAL test (package v2): the cancel seam (RequestCancel —
// plan N6 lands in the engine lane) is an unexported field, and the seam
// behaviour is pinned here with a stub so the route's mapping is already
// contractual before the engine method exists.
package v2

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/migrate"
	"ops-admin/backend/internal/infra/model"
	"ops-admin/backend/internal/infra/registry"
	"ops-admin/backend/internal/tasks"
	"ops-admin/backend/internal/testutil"
	v1model "ops-admin/backend/model"
	"ops-admin/backend/opdef"
)

const testOperation = "k8s.workload.restart"

// testCancelStarter is the N6 seam stub: the signature the engine lane's
// Engine.RequestCancel must satisfy for the route to activate.
type testCancelStarter struct {
	calls []string
}

func (s *testCancelStarter) RequestCancel(_ context.Context, taskUID, actor string) error {
	s.calls = append(s.calls, taskUID+"|"+actor)
	return nil
}

func newOperationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := testutil.OpenMemoryDB(t)
	if err := migrate.Run(context.Background(), db); err != nil {
		t.Fatalf("migrate.Run: %v", err)
	}
	return db
}

func newTestRegistry(t *testing.T) *registry.Registry {
	t.Helper()
	reg := registry.New()
	if err := reg.RegisterOperation(contract.OperationDefinition{
		Name:               testOperation,
		Version:            "1",
		ResourceKinds:      []string{"orchestration.workload"},
		RequiredCapability: "orchestration.kubernetes.apply",
		RequiredPermission: "assets:k8s:workload:restart",
		Mutating:           true,
		RiskLevel:          "medium",
		RequiresApproval:   true, // J8 — V2 레인 신중 posture
		IdempotencyPolicy:  "provider_frozen_annotation",
		TimeoutSeconds:     30,
		RetryPolicy:        contract.RetryPolicy{MaxAttempts: 3, BackoffSeconds: 5},
	}); err != nil {
		t.Fatal(err)
	}
	return reg
}

// seedWorkloadFixture writes one kubernetes connection (context + secret
// binding) with one workload resource and one observation.
func seedWorkloadFixture(t *testing.T, db *gorm.DB, providerType string) string {
	t.Helper()
	now := time.Now()
	conn := model.ProviderConnection{
		UID: "conn-" + providerType, ProviderType: providerType, Name: "seed-cluster",
		Endpoint: "https://127.0.0.1:6443", Status: "active", Version: "v1.34.0",
		CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&conn).Error; err != nil {
		t.Fatal(err)
	}
	contextRow := model.ProviderContext{UID: "ctx-" + providerType, ConnectionID: conn.ID, Kind: "cluster", Name: "seed-cluster"}
	if err := db.Create(&contextRow).Error; err != nil {
		t.Fatal(err)
	}
	secret := model.SecretRef{UID: "secr-" + providerType, Ciphertext: "ciphertext-marker", KeyID: "key-1"}
	if err := db.Create(&secret).Error; err != nil {
		t.Fatal(err)
	}
	binding := model.ProviderCredentialBinding{ProviderConnectionID: conn.ID, Purpose: "inventory", SecretRefID: secret.ID, Status: "active"}
	if err := db.Create(&binding).Error; err != nil {
		t.Fatal(err)
	}
	res := model.InfraResource{
		UID: "res-" + providerType, ContextID: contextRow.ID, Kind: "orchestration.workload", Subtype: "deployment",
		ExternalURN: "urn:k8s:1:workload:v2-seed/deployment/seed-nginx", DisplayName: "seed-nginx",
		LifecycleState: "running", HealthState: "healthy", ManagedState: "discovered",
		FirstSeenAt: now, LastSeenAt: now,
	}
	if err := db.Create(&res).Error; err != nil {
		t.Fatal(err)
	}
	observation := model.ResourceObservation{
		ResourceID: res.ID, GenerationUID: res.UID, ObservationHash: "hash-1", NormalizerVersion: "1",
		NormalizedJSON: map[string]any{"replicas": float64(2)},
		ObservedAt:     now,
	}
	if err := db.Create(&observation).Error; err != nil {
		t.Fatal(err)
	}
	return res.UID
}

type operationFixture struct {
	db       *gorm.DB
	engine   *tasks.Engine
	resource string
}

func newOperationFixture(t *testing.T) (*InfraAPI, *operationFixture) {
	t.Helper()
	db := newOperationTestDB(t)
	reg := newTestRegistry(t)
	engine := tasks.NewEngine(db, reg, tasks.Config{WorkerID: "test-worker", PollInterval: time.Hour, LeaseSeconds: 30, ReaperGrace: time.Second})
	resource := seedWorkloadFixture(t, db, "kubernetes")
	api := NewInfraAPIWithEngine(db, reg, engine)
	return api, &operationFixture{db: db, engine: engine, resource: resource}
}

// newOperationRouter mounts the mutation surface the way the router does
// (audit middleware included, no auth — actor context stays empty).
func newOperationRouter(api *InfraAPI) *gin.Engine {
	engine := gin.New()
	api.RegisterOperations(engine.Group("/api/v2/infra"), nil)
	return engine
}

func doOperationRequest(t *testing.T, engine *gin.Engine, method, path string, body []byte, headers map[string]string) (int, *httptest.ResponseRecorder) {
	t.Helper()
	var reader *strings.Reader
	if body != nil {
		reader = strings.NewReader(string(body))
	} else {
		reader = strings.NewReader("")
	}
	req := httptest.NewRequest(method, path, reader)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec.Code, rec
}

func decodeEnvelope(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not JSON: %v — %s", err, rec.Body.String())
	}
	data, _ := body["data"].(map[string]any)
	if data == nil {
		t.Fatalf("envelope without data object: %s", rec.Body.String())
	}
	return data
}

// TestPlanOperationFreezesRestartedAtAndChangesNoState pins the §3.4 plan
// contract: 200 with the full field set (including the policy version and a
// parseable frozen restartedAt), zero task rows — plan is stateless.
func TestPlanOperationFreezesRestartedAtAndChangesNoState(t *testing.T) {
	api, fx := newOperationFixture(t)
	engine := newOperationRouter(api)

	code, rec := doOperationRequest(t, engine, http.MethodPost, "/api/v2/infra/resources/"+fx.resource+"/operations/"+testOperation+"/plan", nil, nil)
	if code != http.StatusOK {
		t.Fatalf("plan returned %d: %s", code, rec.Body.String())
	}
	data := decodeEnvelope(t, rec)
	for _, key := range []string{"operation", "version", "resourceUid", "resourceRevision", "requiresApproval", "riskLevel", "permission", "policyVersion", "restartedAt"} {
		if _, ok := data[key]; !ok {
			t.Fatalf("plan response missing %q: %s", key, rec.Body.String())
		}
	}
	if data["operation"] != testOperation || data["resourceUid"] != fx.resource {
		t.Fatalf("unexpected plan echo: %s", rec.Body.String())
	}
	if data["requiresApproval"] != true || data["riskLevel"] != "medium" || data["permission"] != "assets:k8s:workload:restart" {
		t.Fatalf("unexpected posture fields: %s", rec.Body.String())
	}
	if data["policyVersion"] != "builtin:default-allow" {
		t.Fatalf("policy version must be the evaluated M1 ruleset: %s", rec.Body.String())
	}
	if _, err := time.Parse(time.RFC3339, data["restartedAt"].(string)); err != nil {
		t.Fatalf("restartedAt must be RFC3339 (J1 frozen value): %v", err)
	}
	var tasks int64
	fx.db.Model(&model.ProviderTask{}).Count(&tasks)
	if tasks != 0 {
		t.Fatalf("plan must not create tasks, found %d", tasks)
	}
}

// TestPlanUnknownOperationAndResource404 pins the 404 mapping.
func TestPlanUnknownOperationAndResource404(t *testing.T) {
	api, fx := newOperationFixture(t)
	engine := newOperationRouter(api)

	code, _ := doOperationRequest(t, engine, http.MethodPost, "/api/v2/infra/resources/"+fx.resource+"/operations/unknown.operation/plan", nil, nil)
	if code != http.StatusNotFound {
		t.Fatalf("unknown operation returned %d, want 404", code)
	}
	code, _ = doOperationRequest(t, engine, http.MethodPost, "/api/v2/infra/resources/res-unknown/operations/"+testOperation+"/plan", nil, nil)
	if code != http.StatusNotFound {
		t.Fatalf("unknown resource returned %d, want 404", code)
	}
}

// TestPolicyEvaluationErrorFailsClosed403 is the VK-4 negative (D1): an
// unassemblable policy input (a connection without a provider type — the
// evaluation reports an error, not an allow) must answer 403, never fall
// through.
func TestPolicyEvaluationErrorFailsClosed403(t *testing.T) {
	api, fx := newOperationFixture(t)
	// A second resource whose connection carries no provider type: the policy
	// input assembly cannot complete → Evaluate returns an error.
	broken := seedWorkloadFixture(t, fx.db, "")
	engine := newOperationRouter(api)

	code, rec := doOperationRequest(t, engine, http.MethodPost, "/api/v2/infra/resources/"+broken+"/operations/"+testOperation+"/plan", nil, nil)
	if code != http.StatusForbidden {
		t.Fatalf("policy evaluation failure returned %d, want 403 (VK-4 fail-closed): %s", code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "POLICY_EVALUATION_FAILED") {
		t.Fatalf("expected the policy error code in the body: %s", rec.Body.String())
	}
	_ = fx
}

// TestExecuteRequiresIdempotencyKey400 pins the header contract.
func TestExecuteRequiresIdempotencyKey400(t *testing.T) {
	api, fx := newOperationFixture(t)
	engine := newOperationRouter(api)

	code, _ := doOperationRequest(t, engine, http.MethodPost, "/api/v2/infra/resources/"+fx.resource+"/operations/"+testOperation+"/execute", []byte(`{}`), nil)
	if code != http.StatusBadRequest {
		t.Fatalf("execute without Idempotency-Key returned %d, want 400", code)
	}
}

// TestExecuteSubmitsReplaysAndFreezesPayload pins the §13.4 idempotency
// surface: first submit 201, replay of the same key 200 + Idempotency-Replayed,
// one task row, and the frozen restartedAt from the plan flow carried into
// the task payload (J1/J7).
func TestExecuteSubmitsReplaysAndFreezesPayload(t *testing.T) {
	api, fx := newOperationFixture(t)
	engine := newOperationRouter(api)
	path := "/api/v2/infra/resources/" + fx.resource + "/operations/" + testOperation + "/execute"
	headers := map[string]string{"Idempotency-Key": "idem-key-1", "Content-Type": "application/json"}
	body := []byte(`{"restartedAt":"2026-09-07T00:00:00Z","resourceRevision":7}`)

	code, rec := doOperationRequest(t, engine, http.MethodPost, path, body, headers)
	if code != http.StatusCreated {
		t.Fatalf("first execute returned %d, want 201: %s", code, rec.Body.String())
	}
	first := decodeEnvelope(t, rec)["task"].(map[string]any)
	if first["status"] != tasks.TaskStatusAwaitingApproval {
		t.Fatalf("RequiresApproval=true must land awaiting_approval (J8), got %v", first["status"])
	}
	if first["payload"].(map[string]any)["restartedAt"] != "2026-09-07T00:00:00Z" {
		t.Fatalf("payload must carry the frozen restartedAt: %s", rec.Body.String())
	}

	code, rec = doOperationRequest(t, engine, http.MethodPost, path, body, headers)
	if code != http.StatusOK {
		t.Fatalf("replayed execute returned %d, want 200 (r2 L2): %s", code, rec.Body.String())
	}
	if rec.Header().Get("Idempotency-Replayed") != "true" {
		t.Fatalf("replay must set Idempotency-Replayed: true — %s", rec.Body.String())
	}
	replayed := decodeEnvelope(t, rec)["task"].(map[string]any)
	if replayed["uid"] != first["uid"] {
		t.Fatalf("replay must return the same task: %v vs %v", replayed["uid"], first["uid"])
	}
	var count int64
	fx.db.Model(&model.ProviderTask{}).Count(&count)
	if count != 1 {
		t.Fatalf("idempotent replay must not create a second task, found %d", count)
	}
}

// TestExecuteResourceBusy409 pins the N11 fast-fail mapping.
func TestExecuteResourceBusy409(t *testing.T) {
	api, fx := newOperationFixture(t)
	engine := newOperationRouter(api)
	path := "/api/v2/infra/resources/" + fx.resource + "/operations/" + testOperation + "/execute"
	base := map[string]string{"Content-Type": "application/json", "Idempotency-Key": "key-busy-1"}

	if code, rec := doOperationRequest(t, engine, http.MethodPost, path, []byte(`{}`), base); code != http.StatusCreated {
		t.Fatalf("first execute returned %d: %s", code, rec.Body.String())
	}
	busy := map[string]string{"Content-Type": "application/json", "Idempotency-Key": "key-busy-2"}
	code, rec := doOperationRequest(t, engine, http.MethodPost, path, []byte(`{}`), busy)
	if code != http.StatusConflict || !strings.Contains(rec.Body.String(), "RESOURCE_BUSY") {
		t.Fatalf("second active mutation returned %d, want 409 RESOURCE_BUSY: %s", code, rec.Body.String())
	}
}

// TestTaskReadsAndApprovalVerbs covers GET tasks/:uid (+events) and the
// approve/reject chain with the 409 mapping (D1: 승인 비대상 409).
func TestTaskReadsAndApprovalVerbs(t *testing.T) {
	api, fx := newOperationFixture(t)
	engine := newOperationRouter(api)
	path := "/api/v2/infra/resources/" + fx.resource + "/operations/" + testOperation + "/execute"
	headers := map[string]string{"Content-Type": "application/json", "Idempotency-Key": "key-approve-1"}
	code, rec := doOperationRequest(t, engine, http.MethodPost, path, []byte(`{}`), headers)
	if code != http.StatusCreated {
		t.Fatalf("execute returned %d: %s", code, rec.Body.String())
	}
	taskUID := decodeEnvelope(t, rec)["task"].(map[string]any)["uid"].(string)

	code, rec = doOperationRequest(t, engine, http.MethodGet, "/api/v2/infra/tasks/"+taskUID, nil, nil)
	if code != http.StatusOK {
		t.Fatalf("GET task returned %d: %s", code, rec.Body.String())
	}
	if got := decodeEnvelope(t, rec)["task"].(map[string]any)["uid"]; got != taskUID {
		t.Fatalf("GET task echo mismatch: %s", rec.Body.String())
	}
	code, rec = doOperationRequest(t, engine, http.MethodGet, "/api/v2/infra/tasks/"+taskUID+"/events", nil, nil)
	if code != http.StatusOK {
		t.Fatalf("GET events returned %d: %s", code, rec.Body.String())
	}
	events := decodeEnvelope(t, rec)["events"].([]any)
	if len(events) == 0 || events[0].(map[string]any)["type"] != tasks.TaskEventCreated {
		t.Fatalf("events must lead with the created event: %s", rec.Body.String())
	}

	code, _ = doOperationRequest(t, engine, http.MethodPost, "/api/v2/infra/tasks/"+taskUID+"/approve", nil, nil)
	if code != http.StatusOK {
		t.Fatalf("approve returned %d, want 200", code)
	}
	code, rec = doOperationRequest(t, engine, http.MethodPost, "/api/v2/infra/tasks/"+taskUID+"/approve", nil, nil)
	if code != http.StatusConflict || !strings.Contains(rec.Body.String(), "TASK_NOT_AWAITING_APPROVAL") {
		t.Fatalf("second approve returned %d, want 409: %s", code, rec.Body.String())
	}
	code, rec = doOperationRequest(t, engine, http.MethodPost, "/api/v2/infra/tasks/"+taskUID+"/reject", nil, nil)
	if code != http.StatusConflict {
		t.Fatalf("reject on a queued task returned %d, want 409: %s", code, rec.Body.String())
	}

	// A fresh awaiting task (inserted directly — the first task still holds
	// the resource's active slot): reject is terminal cancelled (§13.3).
	awaiting := model.ProviderTask{UID: "task-awaiting-reject", OperationName: testOperation, OperationVersion: "1", ResourceUID: "res-other-reject", Status: tasks.TaskStatusAwaitingApproval, ApprovalStatus: tasks.ApprovalStatusNotRequired, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := fx.db.Create(&awaiting).Error; err != nil {
		t.Fatal(err)
	}
	secondUID := awaiting.UID
	code, rec = doOperationRequest(t, engine, http.MethodPost, "/api/v2/infra/tasks/"+secondUID+"/reject", nil, nil)
	if code != http.StatusOK {
		t.Fatalf("reject returned %d: %s", code, rec.Body.String())
	}
	rejected := decodeEnvelope(t, rec)["task"].(map[string]any)
	if rejected["status"] != tasks.TaskStatusCancelled || rejected["approvalStatus"] != tasks.ApprovalStatusRejected {
		t.Fatalf("reject must land cancelled/rejected: %s", rec.Body.String())
	}

	// Unknown task uid → 404 on every task route.
	for _, verb := range []string{"GET:/tasks/", "POST:/tasks/"} {
		method, prefix, _ := strings.Cut(verb, ":")
		code, _ = doOperationRequest(t, engine, method, "/api/v2/infra"+prefix+"task-unknown"+map[string]string{"GET": "", "POST": "/approve"}[method], nil, nil)
		if code != http.StatusNotFound {
			t.Fatalf("%s unknown task returned %d, want 404", verb, code)
		}
	}
}

// TestCancelRouteContract pins the cancel mapping: unknown 404, terminal 409,
// and — until the engine lane's RequestCancel lands (N6) — the seam degrades
// 503. With the stub seam wired the delegation reaches RequestCancel.
func TestCancelRouteContract(t *testing.T) {
	api, fx := newOperationFixture(t)
	engine := newOperationRouter(api)

	code, _ := doOperationRequest(t, engine, http.MethodPost, "/api/v2/infra/tasks/task-unknown/cancel", nil, nil)
	if code != http.StatusNotFound {
		t.Fatalf("cancel unknown task returned %d, want 404", code)
	}

	// A terminal task (rejected above lifecycle) is not cancellable — 409
	// before the seam is consulted.
	rejected := model.ProviderTask{UID: "task-rejected", OperationName: testOperation, OperationVersion: "1", ResourceUID: "res-other", Status: tasks.TaskStatusCancelled, ApprovalStatus: tasks.ApprovalStatusRejected, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := fx.db.Create(&rejected).Error; err != nil {
		t.Fatal(err)
	}
	code, _ = doOperationRequest(t, engine, http.MethodPost, "/api/v2/infra/tasks/task-rejected/cancel", nil, nil)
	if code != http.StatusConflict {
		t.Fatalf("cancel terminal task returned %d, want 409", code)
	}

	// Awaiting task + seam not wired → 503 degradation.
	awaiting := model.ProviderTask{UID: "task-awaiting", OperationName: testOperation, OperationVersion: "1", ResourceUID: "res-other-2", Status: tasks.TaskStatusAwaitingApproval, ApprovalStatus: tasks.ApprovalStatusNotRequired, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := fx.db.Create(&awaiting).Error; err != nil {
		t.Fatal(err)
	}
	code, _ = doOperationRequest(t, engine, http.MethodPost, "/api/v2/infra/tasks/task-awaiting/cancel", nil, nil)
	if code != http.StatusServiceUnavailable {
		t.Fatalf("cancel without the engine seam returned %d, want 503", code)
	}

	// With the seam wired (the N6 signature), the delegation carries uid+actor.
	starter := &testCancelStarter{}
	api.cancelStarter = starter
	code, _ = doOperationRequest(t, engine, http.MethodPost, "/api/v2/infra/tasks/task-awaiting/cancel", nil, nil)
	if code != http.StatusOK {
		t.Fatalf("cancel with wired seam returned %d, want 200", code)
	}
	if len(starter.calls) != 1 || starter.calls[0] != "task-awaiting|" {
		t.Fatalf("RequestCancel delegation mismatch: %v", starter.calls)
	}
}

// TestListResourceOperationsIntersectsKind pins the §16.1 operations read:
// the registry definition set × the resource kind — a workload sees restart,
// a non-workload resource sees nothing (F-9 double-source note).
func TestListResourceOperationsIntersectsKind(t *testing.T) {
	api, fx := newOperationFixture(t)
	engine := newOperationRouter(api)

	code, rec := doOperationRequest(t, engine, http.MethodGet, "/api/v2/infra/resources/"+fx.resource+"/operations", nil, nil)
	if code != http.StatusOK {
		t.Fatalf("operations read returned %d: %s", code, rec.Body.String())
	}
	items := decodeEnvelope(t, rec)["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("workload must see exactly the restart operation, got %s", rec.Body.String())
	}
	row := items[0].(map[string]any)
	if row["name"] != testOperation || row["requiresApproval"] != true || row["permission"] != "assets:k8s:workload:restart" {
		t.Fatalf("unexpected operation row: %s", rec.Body.String())
	}

	node := model.InfraResource{UID: "res-node", ContextID: 1, Kind: "orchestration.node", ExternalURN: "urn:k8s:1:node:n1", DisplayName: "n1", FirstSeenAt: time.Now(), LastSeenAt: time.Now()}
	// ContextID 1 does not exist → liveResources join drops it; give it a real
	// context by reusing the fixture's context through a direct lookup.
	var ctxRow model.ProviderContext
	if err := fx.db.First(&ctxRow).Error; err != nil {
		t.Fatal(err)
	}
	node.ContextID = ctxRow.ID
	if err := fx.db.Create(&node).Error; err != nil {
		t.Fatal(err)
	}
	code, rec = doOperationRequest(t, engine, http.MethodGet, "/api/v2/infra/resources/res-node/operations", nil, nil)
	if code != http.StatusOK {
		t.Fatalf("node operations read returned %d: %s", code, rec.Body.String())
	}
	if items := decodeEnvelope(t, rec)["items"].([]any); len(items) != 0 {
		t.Fatalf("node must see no operations, got %s", rec.Body.String())
	}
}

// TestTerminalAuditRecord pins the J3(b) terminal row: url=task://<uid>,
// task_uid joins, and the §18.1 terminal-only fields (result_hash, error_code,
// provider_task_id from the attempt handle, approval fields) are present in
// the v2_context JSON.
func TestTerminalAuditRecord(t *testing.T) {
	_, fx := newOperationFixture(t)
	task := model.ProviderTask{UID: "task-term", OperationName: testOperation, OperationVersion: "1", ResourceUID: fx.resource, Status: tasks.TaskStatusSucceeded, ApprovalStatus: tasks.ApprovalStatusApproved, Approver: "operator-1", ErrorCode: "", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := fx.db.Create(&task).Error; err != nil {
		t.Fatal(err)
	}
	attempt := model.TaskAttempt{TaskID: task.ID, AttemptNo: 1, WorkerID: "w1", HandleRef: "rollout|conn-kubernetes|v2-seed|deployment|seed-nginx|2", StartedAt: time.Now()}
	if err := fx.db.Create(&attempt).Error; err != nil {
		t.Fatal(err)
	}

	if err := RecordTaskTerminal(context.Background(), fx.db, task, tasks.TaskStatusSucceeded, contract.JSONMap{"generation": float64(2), "restartedAt": "2026-09-07T00:00:00Z"}); err != nil {
		t.Fatalf("RecordTaskTerminal: %v", err)
	}
	var row v1model.OperationLog
	if err := fx.db.Where("task_uid = ?", task.UID).First(&row).Error; err != nil {
		t.Fatalf("terminal row must join on task_uid (claim 8): %v", err)
	}
	if row.URL != "task://"+task.UID || row.Method != "TASK" || !row.Success {
		t.Fatalf("unexpected terminal row shape: %+v", row)
	}
	if !strings.Contains(row.RequestSummary, "result_hash=") || strings.Contains(row.RequestSummary, "kubeconfig") {
		t.Fatalf("terminal request_summary must be hash/code only (VK-11): %q", row.RequestSummary)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(*row.V2Context), &decoded); err != nil {
		t.Fatalf("v2_context must be JSON: %v", err)
	}
	for _, key := range []string{"task_uid", "operation", "operation_version", "mutating", "policy_version", "resource_uid", "provider_task_id", "result_hash", "approval_status", "approver"} {
		if _, ok := decoded[key]; !ok {
			t.Fatalf("terminal v2_context missing %q: %v", key, decoded)
		}
	}
	if decoded["provider_task_id"] != attempt.HandleRef {
		t.Fatalf("provider_task_id must come from the attempt handle: %v", decoded["provider_task_id"])
	}
}

// TestAuditMiddlewareWritesRequestRows pins the J3(a) request row: execute
// writes one row (denied requests included — §3.4 "미권한 403(감사와 함께)"),
// plan writes none, and request_summary stays hash/code-only (VK-11).
func TestAuditMiddlewareWritesRequestRows(t *testing.T) {
	api, fx := newOperationFixture(t)
	engine := newOperationRouter(api)
	path := "/api/v2/infra/resources/" + fx.resource + "/operations/" + testOperation + "/execute"

	code, _ := doOperationRequest(t, engine, http.MethodPost, path, []byte(`{"restartedAt":"2026-09-07T00:00:00Z"}`), map[string]string{"Content-Type": "application/json", "Idempotency-Key": "key-audit-1", "X-Request-ID": "req-audit-1", "traceparent": "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"})
	if code != http.StatusCreated {
		t.Fatalf("execute returned %d", code)
	}
	var rows []v1model.OperationLog
	if err := fx.db.Where("url = ?", path).Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("execute must write exactly one request audit row, found %d", len(rows))
	}
	row := rows[0]
	if row.TaskUID == nil || *row.TaskUID == "" {
		t.Fatalf("execute audit row must carry task_uid: %+v", row)
	}
	if !strings.Contains(row.RequestSummary, "request_hash=") || strings.Contains(row.RequestSummary, "restartedAt") {
		t.Fatalf("request_summary must be hash/code only (VK-11): %q", row.RequestSummary)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(*row.V2Context), &decoded); err != nil {
		t.Fatalf("v2_context must be JSON: %v", err)
	}
	if decoded["request_id"] != "req-audit-1" {
		t.Fatalf("X-Request-ID must be honoured: %v", decoded["request_id"])
	}
	if decoded["trace_id"] != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Fatalf("traceparent trace_id must be propagated: %v", decoded["trace_id"])
	}
	if _, ok := decoded["policy_version"]; !ok {
		t.Fatalf("execute row must record the evaluated policy version: %v", decoded)
	}

	// plan writes no request row (J3(a) — the four verbs only).
	planPath := "/api/v2/infra/resources/" + fx.resource + "/operations/" + testOperation + "/plan"
	doOperationRequest(t, engine, http.MethodPost, planPath, nil, nil)
	var planRows int64
	fx.db.Model(&v1model.OperationLog{}).Where("url = ?", planPath).Count(&planRows)
	if planRows != 0 {
		t.Fatalf("plan must not write a request audit row, found %d", planRows)
	}
}

// TestAuditMiddlewareRecordsDenials pins the denied-but-audited posture: a
// 403 permission denial on a v2 mutation route still leaves a request row
// with the error code in the summary.
func TestAuditMiddlewareRecordsDenials(t *testing.T) {
	api, fx := newOperationFixture(t)
	engine := gin.New()
	group := engine.Group("/api/v2/infra")
	// The aborting grant sits route-level (the grant callback position), so
	// the audit middleware registered ahead of it still observes the denial.
	api.RegisterOperations(group, func(opdef.Def) gin.HandlerFunc {
		return func(c *gin.Context) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": 403, "message": "denied"})
		}
	})

	path := "/api/v2/infra/resources/" + fx.resource + "/operations/" + testOperation + "/execute"
	doOperationRequest(t, engine, http.MethodPost, path, []byte(`{}`), map[string]string{"Content-Type": "application/json"})
	var count int64
	fx.db.Model(&v1model.OperationLog{}).Where("url = ?", path).Count(&count)
	if count != 1 {
		t.Fatalf("denied mutation must still be audited, found %d rows", count)
	}
}

// TestResolveOperationPermissionSeam covers the router-injected registry
// lookup: registered → permission, unknown/unregistered → miss, nil registry
// → miss (fallback to the opdef representative happens in opdef).
func TestResolveOperationPermissionSeam(t *testing.T) {
	db := newOperationTestDB(t)
	api := NewInfraAPIWithEngine(db, newTestRegistry(t), nil)
	if perm, ok := api.ResolveOperationPermission(testOperation); !ok || perm != "assets:k8s:workload:restart" {
		t.Fatalf("registered operation resolved to %q ok=%v", perm, ok)
	}
	if _, ok := api.ResolveOperationPermission("unknown.operation"); ok {
		t.Fatal("unknown operation must not resolve")
	}
	nilAPI := NewInfraAPIWithRegistry(db, nil)
	if _, ok := nilAPI.ResolveOperationPermission(testOperation); ok {
		t.Fatal("nil registry must not resolve")
	}
}

// TestExecuteDegradedAndPayloadValidation pins the execute guards: a nil
// engine degrades 503, and a non-RFC3339 restartedAt is rejected 400 before
// any task exists.
func TestExecuteDegradedAndPayloadValidation(t *testing.T) {
	db := newOperationTestDB(t)
	reg := newTestRegistry(t)
	resource := seedWorkloadFixture(t, db, "kubernetes")
	nilEngine := NewInfraAPIWithEngine(db, reg, nil)
	engine := newOperationRouter(nilEngine)
	path := "/api/v2/infra/resources/" + resource + "/operations/" + testOperation + "/execute"
	headers := map[string]string{"Content-Type": "application/json", "Idempotency-Key": "key-nil-engine"}

	code, _ := doOperationRequest(t, engine, http.MethodPost, path, []byte(`{}`), headers)
	if code != http.StatusServiceUnavailable {
		t.Fatalf("execute without an engine returned %d, want 503", code)
	}

	api, fx := newOperationFixture(t)
	engine = newOperationRouter(api)
	path = "/api/v2/infra/resources/" + fx.resource + "/operations/" + testOperation + "/execute"
	code, _ = doOperationRequest(t, engine, http.MethodPost, path, []byte(`{"restartedAt":"not-a-time"}`), headers)
	if code != http.StatusBadRequest {
		t.Fatalf("malformed restartedAt returned %d, want 400", code)
	}
	var tasks int64
	fx.db.Model(&model.ProviderTask{}).Count(&tasks)
	if tasks != 0 {
		t.Fatalf("rejected payload must not create tasks, found %d", tasks)
	}
}

// TestPlanCarriesObservationGeneration covers the J7 hit path: an observation
// carrying a generation key flows into resourceRevision (and the execute
// payload freeze).
func TestPlanCarriesObservationGeneration(t *testing.T) {
	api, fx := newOperationFixture(t)
	now := time.Now()
	obs := model.ResourceObservation{
		ResourceID: 1, GenerationUID: fx.resource, ObservationHash: "hash-2", NormalizerVersion: "1",
		NormalizedJSON: map[string]any{"generation": float64(9)},
		ObservedAt:     now.Add(time.Second), // newer than the fixture's observation
	}
	if err := fx.db.Create(&obs).Error; err != nil {
		t.Fatal(err)
	}
	engine := newOperationRouter(api)
	code, rec := doOperationRequest(t, engine, http.MethodPost, "/api/v2/infra/resources/"+fx.resource+"/operations/"+testOperation+"/plan", nil, nil)
	if code != http.StatusOK {
		t.Fatalf("plan returned %d: %s", code, rec.Body.String())
	}
	if got := decodeEnvelope(t, rec)["resourceRevision"]; got != float64(9) {
		t.Fatalf("resourceRevision must snapshot the observation generation, got %v", got)
	}
}
