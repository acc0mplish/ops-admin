// Package v2 — operations_connection_test.go (I10 J1c)는 §16.1 커넥션-스코프
// create 면의 계약을 소유한다: plan 200(매니페스트 계약·restartedAt 미발급),
// execute 201(합성 uid·payload 동결), Idempotency-Key 미지정 400, 불소속 kind
// 400(policy 사전 차단), 미등록 커넥션 404. CI17 길이 계약도 본 파일이 소유한다
// — sqlite는 varchar를 강제하지 않으므로 최악 길이 왕복·상한 초과 거부는 Go
// 빌더 가드가 유일한 소유자다(§3.3 r3).
package v2

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/inventory" // CI17 LOW② — 생성기 어휘 단얫용 (테스트 전용 import)
	"ops-admin/backend/internal/infra/model"
	"ops-admin/backend/internal/infra/registry"
	"ops-admin/backend/internal/tasks"
)

const (
	connectionCreateOperation = "k8s.resource.create"
	// testConnectionUID — 32-hex(newSyncUID·k8sRegisterUID 어휘) — §3.3 합성
	// uid의 커넥션 분절이 취하는 실제 어휘다.
	testConnectionUID = "c0ffee00c0ffee00c0ffee00c0ffee00"
)

// connectionNamespaceYAML·connectionConfigMapYAML — cluster-scope형과
// namespaced형 target 2형태의 fixture 매니페스트(§3.3).
const connectionNamespaceYAML = `apiVersion: v1
kind: Namespace
metadata:
  name: web-demo
`

const connectionConfigMapYAML = `apiVersion: v1
kind: ConfigMap
metadata:
  name: app-config
  namespace: demo-ns
data:
  level: info
`

// newConnectionCreateRegistry — create def(J0 compose 행과 동일 값)만 담는
// 레지스트리. ResourceKinds는 공란(§0 인코딩 (i) — 커넥션-스코프 부호화).
func newConnectionCreateRegistry(t *testing.T) *registry.Registry {
	t.Helper()
	reg := registry.New()
	if err := reg.RegisterOperation(contract.OperationDefinition{
		Name:               connectionCreateOperation,
		Version:            "1",
		RequiredPermission: "assets:k8s:workload:yaml",
		RequiredCapability: "orchestration.kubernetes.apply",
		ResourceKinds:      nil,
		Mutating:           true,
		RiskLevel:          "high",
		RequiresApproval:   true,
		IdempotencyPolicy:  "provider_create_convergent",
		TimeoutSeconds:     30,
		RetryPolicy:        contract.RetryPolicy{MaxAttempts: 3, BackoffSeconds: 5},
	}); err != nil {
		t.Fatal(err)
	}
	return reg
}

// seedConnectionForCreate — bare 커넥션+클러스터 컨텍스트(infra_resource 행
// 없음 — create 대상은 제출 시점에 아직 존재하지 않는다, §3.3).
func seedConnectionForCreate(t *testing.T, db *gorm.DB, uid string) {
	t.Helper()
	conn := model.ProviderConnection{
		UID: uid, ProviderType: "kubernetes", Name: "create-cluster",
		Endpoint: "https://127.0.0.1:6443", Status: "active",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := db.Create(&conn).Error; err != nil {
		t.Fatal(err)
	}
	contextRow := model.ProviderContext{UID: uid + "-ctx", ConnectionID: conn.ID, Kind: "cluster", Name: "create-cluster"}
	if err := db.Create(&contextRow).Error; err != nil {
		t.Fatal(err)
	}
}

type connectionFixture struct {
	db      *gorm.DB
	engine  *tasks.Engine
	connUID string
}

func newConnectionFixture(t *testing.T) (*InfraAPI, *connectionFixture) {
	t.Helper()
	db := newOperationTestDB(t)
	reg := newConnectionCreateRegistry(t)
	engine := tasks.NewEngine(db, reg, tasks.Config{WorkerID: "test-worker", PollInterval: time.Hour, LeaseSeconds: 30, ReaperGrace: time.Second})
	seedConnectionForCreate(t, db, testConnectionUID)
	api := NewInfraAPIWithEngine(db, reg, engine)
	return api, &connectionFixture{db: db, engine: engine, connUID: testConnectionUID}
}

// newConnectionRouter mounts the connection-scoped surface the way the
// router does (no auth; grants nil — the unit tests resolve nothing).
func newConnectionRouter(api *InfraAPI) *gin.Engine {
	engine := gin.New()
	api.RegisterConnectionOperations(engine.Group("/api/v2/infra"), nil)
	return engine
}

func connectionPath(connUID, verb string) string {
	return fmt.Sprintf("/api/v2/infra/provider-connections/%s/operations/%s/%s", connUID, connectionCreateOperation, verb)
}

func yamlBody(t *testing.T, yaml string) []byte {
	t.Helper()
	raw, err := json.Marshal(map[string]string{"yaml": yaml})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func connectionExecuteHeaders(key string) map[string]string {
	return map[string]string{"Idempotency-Key": key, "Content-Type": "application/json"}
}

// --- §16.1 plan — 매니페스트 계약 회신. ---

// TestPlanConnectionOperationReturnsManifestContract — plan은 stateless다:
// 매핑 kind·승인 posture·policy 버전을 회신하고 restartedAt/resourceRevision은
// 미발급(create의 동결 앵커는 매니페스트 자신 — §3.5), task 행 0.
func TestPlanConnectionOperationReturnsManifestContract(t *testing.T) {
	api, fx := newConnectionFixture(t)
	engine := newConnectionRouter(api)

	status, rec := doOperationRequest(t, engine, http.MethodPost, connectionPath(fx.connUID, "plan"), yamlBody(t, connectionNamespaceYAML), nil)
	if status != http.StatusOK {
		t.Fatalf("plan returned %d, want 200: %s", status, rec.Body.String())
	}
	data := decodeEnvelope(t, rec)
	if data["operation"] != connectionCreateOperation {
		t.Errorf("plan operation = %v, want %q", data["operation"], connectionCreateOperation)
	}
	if data["connectionUid"] != testConnectionUID {
		t.Errorf("plan connectionUid = %v, want %q", data["connectionUid"], testConnectionUID)
	}
	if data["targetKind"] != "orchestration.namespace" {
		t.Errorf("plan targetKind = %v, want orchestration.namespace (매핑 표 파생)", data["targetKind"])
	}
	if data["requiresApproval"] != true {
		t.Errorf("plan requiresApproval = %v, want true (전 오퍼레이션 승인 posture)", data["requiresApproval"])
	}
	if version, _ := data["policyVersion"].(string); version == "" {
		t.Errorf("plan policyVersion = %v, want the evaluated policy version", data["policyVersion"])
	}
	if _, has := data["restartedAt"]; has {
		t.Errorf("plan must not issue restartedAt — the create anchor is the manifest itself (§3.5)")
	}
	var taskRows int64
	fx.db.Model(&model.ProviderTask{}).Count(&taskRows)
	if taskRows != 0 {
		t.Fatalf("plan must stay stateless — provider_task rows = %d, want 0", taskRows)
	}
}

// --- §16.1 execute — 합성 uid·멱등·재생. ---

// TestExecuteConnectionOperationSubmitsSyntheticUID — execute는 매니페스트
// 신원에서 결정적 합성 uid를 도출해 Submit한다: namespaced형은
// conn:<uid>:configmap:<ns>/<name>, cluster-scope형은 conn:<uid>:namespace:<name>.
// payload는 {yaml} 그대로 동결(주입 없음)·첫 제출 201·동일 키 재생 200 +
// Idempotency-Replayed(§13.4).
func TestExecuteConnectionOperationSubmitsSyntheticUID(t *testing.T) {
	api, fx := newConnectionFixture(t)
	engine := newConnectionRouter(api)

	// namespaced형 — target <ns>/<name>.
	status, rec := doOperationRequest(t, engine, http.MethodPost, connectionPath(fx.connUID, "execute"),
		yamlBody(t, connectionConfigMapYAML), connectionExecuteHeaders("conn-create-key-1"))
	if status != http.StatusCreated {
		t.Fatalf("execute returned %d, want 201: %s", status, rec.Body.String())
	}
	task := decodeEnvelope(t, rec)["task"].(map[string]any)
	wantUID := "conn:" + testConnectionUID + ":configmap:demo-ns/app-config"
	if task["resourceUid"] != wantUID {
		t.Fatalf("task resourceUid = %v, want %q", task["resourceUid"], wantUID)
	}
	if task["status"] != tasks.TaskStatusAwaitingApproval {
		t.Errorf("status = %v, want awaiting_approval (RequiresApproval posture)", task["status"])
	}
	if payload, _ := task["payload"].(map[string]any); payload["yaml"] != connectionConfigMapYAML {
		t.Errorf("payload must freeze the yaml verbatim (주입 없음): %v", payload["yaml"])
	}

	// cluster-scope형 — target <name>.
	status, rec = doOperationRequest(t, engine, http.MethodPost, connectionPath(fx.connUID, "execute"),
		yamlBody(t, connectionNamespaceYAML), connectionExecuteHeaders("conn-create-key-2"))
	if status != http.StatusCreated {
		t.Fatalf("cluster-scope execute returned %d, want 201: %s", status, rec.Body.String())
	}
	task = decodeEnvelope(t, rec)["task"].(map[string]any)
	if task["resourceUid"] != "conn:"+testConnectionUID+":namespace:web-demo" {
		t.Fatalf("cluster-scope resourceUid = %v, want conn:<uid>:namespace:web-demo", task["resourceUid"])
	}

	// 동일 키 재생 — 200 + Idempotency-Replayed(§13.4).
	status, rec = doOperationRequest(t, engine, http.MethodPost, connectionPath(fx.connUID, "execute"),
		yamlBody(t, connectionConfigMapYAML), connectionExecuteHeaders("conn-create-key-1"))
	if status != http.StatusOK {
		t.Fatalf("replayed execute returned %d, want 200: %s", status, rec.Body.String())
	}
	if rec.Header().Get("Idempotency-Replayed") != "true" {
		t.Fatalf("replay must set Idempotency-Replayed: true — %s", rec.Body.String())
	}
	var count int64
	fx.db.Model(&model.ProviderTask{}).Count(&count)
	if count != 2 {
		t.Fatalf("replay must not create a second task — provider_task rows = %d, want 2", count)
	}
}

// TestExecuteConnectionRequiresIdempotencyKey400 — 키 미지정은 400(§13.4 —
// 기존 ExecuteOperation과 동일 계약).
func TestExecuteConnectionRequiresIdempotencyKey400(t *testing.T) {
	api, fx := newConnectionFixture(t)
	engine := newConnectionRouter(api)

	status, rec := doOperationRequest(t, engine, http.MethodPost, connectionPath(fx.connUID, "execute"), yamlBody(t, connectionNamespaceYAML), nil)
	if status != http.StatusBadRequest {
		t.Fatalf("execute without Idempotency-Key returned %d, want 400", status)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body["message"] != "IDEMPOTENCY_KEY_REQUIRED" {
		t.Fatalf("error code = %v (%s), want IDEMPOTENCY_KEY_REQUIRED", body["message"], rec.Body.String())
	}
}

// --- §16.1 — payload 사전 차단(400)·미등록 커넥션(404). ---

// TestConnectionOperationRejectsForeignKind400 — 매핑 표 불소속(group×kind
// 교차 포함)은 policy 평가 이전에 400으로 차단된다(VK-4 정신 — Evaluate가
// 공백 kind를 fail-closed로 거부하므로 평가 불가 입력은 사전에 잘라낸다).
func TestConnectionOperationRejectsForeignKind400(t *testing.T) {
	api, fx := newConnectionFixture(t)
	engine := newConnectionRouter(api)

	foreign := `apiVersion: networking.istio.io/v1
kind: Pod
metadata:
  name: cross-kind
`
	for _, verb := range []string{"plan", "execute"} {
		status, rec := doOperationRequest(t, engine, http.MethodPost, connectionPath(fx.connUID, verb), yamlBody(t, foreign), connectionExecuteHeaders("conn-create-key-3"))
		if status != http.StatusBadRequest {
			t.Fatalf("%s of a foreign kind returned %d, want 400: %s", verb, status, rec.Body.String())
		}
		var body map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body["message"] != "INVALID_OPERATION_PAYLOAD" {
			t.Fatalf("%s error code = %v, want INVALID_OPERATION_PAYLOAD", verb, body["message"])
		}
	}
}

// TestConnectionOperationUnknownConnection404 — 미등록(또는 stale_source)
// 커넥션은 404 CONNECTION_NOT_FOUND다(§3.5 신규 404).
func TestConnectionOperationUnknownConnection404(t *testing.T) {
	api, _ := newConnectionFixture(t)
	engine := newConnectionRouter(api)

	status, rec := doOperationRequest(t, engine, http.MethodPost, connectionPath("nosuchconn", "plan"), yamlBody(t, connectionNamespaceYAML), nil)
	if status != http.StatusNotFound {
		t.Fatalf("unknown connection plan returned %d, want 404", status)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body["message"] != "CONNECTION_NOT_FOUND" {
		t.Fatalf("error code = %v, want CONNECTION_NOT_FOUND", body["message"])
	}
}

// --- CI17 — 길이 계약(sqlite 미강제를 Go 빌더가 소유, §3.3 r3·§5 #18). ---

// TestSyntheticConnectionUIDRoundTripAndBudget — 최악 fixture(singular 15 +
// ns 63 + name 63 + 32-hex 커넥션 분절 → uid 181자)의 왕복: 빌드 → 포맷 계약
// 파싱(tasks.connUIDPrefix·engine_resolve.go connectionUIDOfSynthetic과 동일
// 분절 계약) → 커넥션 uid·target 복원. 상한 초과는 빌더가 하드 거부하고
// 핸들러는 400 INVALID_OPERATION_PAYLOAD로 매핑한다.
func TestSyntheticConnectionUIDRoundTripAndBudget(t *testing.T) {
	worst := connectionCreateTarget{
		Manifest:  map[string]any{},
		Entry:     contract.K8sCreateFaceEntry{Kind: "DestinationRule", Plural: "destinationrules", Namespaced: true},
		Name:      strings.Repeat("n", 63),
		Namespace: strings.Repeat("s", 63),
		YAML:      "",
	}
	uid, err := connectionResourceUID(testConnectionUID, worst)
	if err != nil {
		t.Fatalf("worst-case build: %v", err)
	}
	if want := "conn:" + testConnectionUID + ":destinationrule:" + strings.Repeat("s", 63) + "/" + strings.Repeat("n", 63); uid != want {
		t.Fatalf("worst-case uid = %q, want the §3.3 format", uid)
	}
	if len(uid) != 181 {
		t.Fatalf("worst-case uid length = %d, want 181 (5+32+1+15+1+63+1+63)", len(uid))
	}

	// 왕복 — 포맷 계약 파싱(engine_resolve.go의 분절 계약과 동일):
	// conn:<connectionUID>:<singular>:<target> 3분절.
	trimmed := strings.TrimPrefix(uid, "conn:")
	segs := strings.Split(trimmed, ":")
	if len(segs) != 3 {
		t.Fatalf("segment count = %d, want 3 (§3.3)", len(segs))
	}
	if segs[0] != testConnectionUID {
		t.Errorf("connection segment = %q, want %q", segs[0], testConnectionUID)
	}
	if segs[1] != "destinationrule" {
		t.Errorf("singular segment = %q, want lowercase kind", segs[1])
	}
	if segs[2] != strings.Repeat("s", 63)+"/"+strings.Repeat("n", 63) {
		t.Errorf("target segment = %q, want <ns>/<name>", segs[2])
	}

	// 상한 초과 — 커넥션 분절이 길면(테스트 DB 연결 uid 240자 — sqlite는
	// varchar를 강제하지 않는다; 5+240+1+9+1+8 = 264 > 255) 빌더가 하드 거부하고
	// 핸들러는 400이다.
	api, _ := newConnectionFixture(t)
	longUID := strings.Repeat("f", 240)
	seedConnectionForCreate(t, api.db, longUID)
	engine := newConnectionRouter(api)
	status, rec := doOperationRequest(t, engine, http.MethodPost, connectionPath(longUID, "execute"),
		yamlBody(t, connectionNamespaceYAML), connectionExecuteHeaders("conn-create-key-long"))
	if status != http.StatusBadRequest {
		t.Fatalf("over-budget uid execute returned %d, want 400: %s", status, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body["message"] != "INVALID_OPERATION_PAYLOAD" {
		t.Fatalf("error code = %v, want INVALID_OPERATION_PAYLOAD (§5 #18 빌더 하드 거부)", body["message"])
	}
}

// TestConnectionUIDGeneratorsNeverCollideWithConnPrefix — CI17 LOW②: 커넥션·
// 자원 uid 생성기의 산출 어휘는 ^[0-9a-f]{32}$ — 콜론을 포함하지 않아
// conn: 합성 접두와 구조적으로 충돌하지 않는다(생성기 수준 잠금).
// SourceKeyUID·SourceKeyUIDSalted는 export라 여기서 직접 단얫한다. newSyncUID는
// unexported + inventory/** 무변경(§5 #3)라 직접 단얫 불가 — 동일 근거
// (crypto/rand 16B hex32)를 공유하며 데이터 수준 가드는 tasks 패키지
// TestInfraResourceUIDsNeverUseConnPrefix가 소유한다. k8sRegisterUID 단얫은
// main_register_k8s_test.go가 소유한다(main 패키지 비노출).
func TestConnectionUIDGeneratorsNeverCollideWithConnPrefix(t *testing.T) {
	hex32 := regexp.MustCompile(`^[0-9a-f]{32}$`)
	for _, uid := range []string{
		inventory.SourceKeyUID("asset_cloud_account", 1),
		inventory.SourceKeyUID("asset_cloud_account", 42),
		inventory.SourceKeyUIDSalted("asset_cloud_account", 1, "context"),
		inventory.SourceKeyUIDSalted("integration_finops_account", 7, "secret"),
	} {
		if !hex32.MatchString(uid) {
			t.Errorf("SourceKeyUID family produced %q, want ^[0-9a-f]{32}$ (conn: 비충돌 생성기 잠금)", uid)
		}
	}
}
