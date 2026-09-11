package router

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"ops-admin/backend/internal/api/v2"
	"ops-admin/backend/internal/infra/compose"
	inframodel "ops-admin/backend/internal/infra/model"
	"ops-admin/backend/internal/tasks"
	"ops-admin/backend/model"
)

// connectionAuditUID — 32-hex 어휘(newSyncUID·k8sRegisterUID와 동일 형식)의
// 테스트 커넥션 uid. 합성 resource_uid의 커넥션 분절이 실제로 취하는 형태다.
const connectionAuditUID = "audit1234audit1234audit1234audit1234"

// connectionAuditPath — §16.1 execute 라우트(감사 대상 동사 — plan은
// stateless라 미기록, audit.go "the four verbs only").
func connectionAuditPath() string {
	return v2APIPrefix + "/infra/provider-connections/" + connectionAuditUID + "/operations/k8s.resource.create/execute"
}

// seedConnectionForAudit — bare 커넥션+클러스터 컨텍스트(operations_connection_test
// seedConnectionForCreate의 라우터 패키지 사본 — create 대상 infra_resource 행은
// 원래 존재하지 않는다, I-P1).
func seedConnectionForAudit(t *testing.T, db *gorm.DB) {
	t.Helper()
	conn := inframodel.ProviderConnection{
		UID: connectionAuditUID, ProviderType: "kubernetes", Name: "audit-cluster",
		Endpoint: "https://127.0.0.1:6443", Status: "active",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := db.Create(&conn).Error; err != nil {
		t.Fatal(err)
	}
	contextRow := inframodel.ProviderContext{
		UID: connectionAuditUID + "-ctx", ConnectionID: conn.ID, Kind: "cluster", Name: "audit-cluster",
	}
	if err := db.Create(&contextRow).Error; err != nil {
		t.Fatal(err)
	}
}

// doConnectionExecute — 인증+멱등키 헤더를 실은 단일 POST(doRequest는
// Authorization만 실으므로 로컬 사본).
func doConnectionExecute(engine *gin.Engine, token string, path string, body []byte) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Idempotency-Key", "conn-audit-pin-key")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	return w
}

// TestConnectionExecuteWritesExactlyOneAuditRow — j1c M-1 이월(I10 J3 착지):
// §16.1 커넥션-스코프 execute는 RegisterOperations가 그룹에 장착한
// AuditMiddleware를 "상속"받는다 — RegisterConnectionOperations는 두 번째 Use
// 없이 설계됐고(이중 기록 방지, routes_v2.go 등록 순서), 유닛 라우터
// (operations_connection_test newConnectionRouter)는 미들웨어를 우회하므로
// 상속 계약은 실제 라우터에서만 핀된다. 실제 라우터를 통과한 execute 1건이
// sys_operation_log에 정확히 1행을 남기고, 그 행이 §18.1 문맥(operation·
// task_uid·conn: resource_uid·커넥션 uid)을 운반한다.
func TestConnectionExecuteWritesExactlyOneAuditRow(t *testing.T) {
	cfg, db := newEngineLaneFixture(t)
	seedConnectionForAudit(t, db)

	stack, err := compose.Build(db)
	if err != nil {
		t.Fatalf("compose.Build: %v", err)
	}
	// nil terminal audit writer — 종단 행은 OnTaskTerminal 소관(J6)이고 이
	// 테스트의 대상은 요청 행 1건뿐이다(worker는 폴 없이 대기).
	taskEngine := stack.BuildEngine(tasks.Config{WorkerID: "audit-pin", PollInterval: time.Hour, LeaseSeconds: 30, ReaperGrace: time.Second}, nil)
	api := v2.NewInfraAPIWithEngine(db, stack.Registry, taskEngine)
	engineRouter, svc := New(cfg, db, api)
	t.Cleanup(func() { _ = svc.Shutdown(context.Background()) })

	super := replayFindRole(t, db, "super-admin")
	granted := replayRole(t, db, "connection-audit-role")
	if copied := copyRoleGrants(t, db, granted.ID, super.ID); copied == 0 {
		t.Fatal("grant copy produced zero rows — replay baseline is empty")
	}
	replayAdminRole(t, db, 9103, granted.ID)
	token := replaySession(t, db, 9103, "conn-audit-admin")

	path := connectionAuditPath()
	body, err := json.Marshal(map[string]string{
		"yaml": "apiVersion: v1\nkind: Namespace\nmetadata:\n  name: audit-pin-ns\n",
	})
	if err != nil {
		t.Fatal(err)
	}

	var rowsBefore int64
	if err := db.Model(&model.OperationLog{}).Where("url = ?", path).Count(&rowsBefore).Error; err != nil {
		t.Fatal(err)
	}
	if rowsBefore != 0 {
		t.Fatalf("pre-execute audit rows for %s = %d, want 0", path, rowsBefore)
	}

	w := doConnectionExecute(engineRouter, token, path, body)
	if w.Code != http.StatusCreated {
		t.Fatalf("connection execute = %d, want 201: %s", w.Code, w.Body.String())
	}
	var submitted struct {
		Data struct {
			Task struct {
				UID string `json:"uid"`
			} `json:"task"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &submitted); err != nil {
		t.Fatalf("execute body decode: %v — %s", err, w.Body.String())
	}
	if submitted.Data.Task.UID == "" {
		t.Fatalf("execute body carries no task uid: %s", w.Body.String())
	}

	var logs []model.OperationLog
	if err := db.Where("url = ?", path).Find(&logs).Error; err != nil {
		t.Fatal(err)
	}
	if len(logs) != 1 {
		t.Fatalf("sys_operation_log rows for connection execute = %d, want exactly 1 (audit inheritance, no double-write): %+v", len(logs), logs)
	}
	row := logs[0]
	if row.Method != http.MethodPost || !row.Success || row.StatusCode != http.StatusCreated {
		t.Fatalf("audit row = (method %s, success %v, status %d), want (POST, true, 201)", row.Method, row.Success, row.StatusCode)
	}
	if row.AdminID != 9103 {
		t.Errorf("audit row admin = %d, want 9103 (authenticated subject)", row.AdminID)
	}

	// §18.1 문맥 — Go 측 디코드 단얫(F-8 ②: MySQL 정규화 때문에 바이트 비교 불가).
	var auditContext map[string]any
	if row.V2Context == nil {
		t.Fatal("audit row carries no v2_context")
	}
	if err := json.Unmarshal([]byte(*row.V2Context), &auditContext); err != nil {
		t.Fatalf("v2_context decode: %v", err)
	}
	if op, _ := auditContext["operation"].(string); op != "k8s.resource.create" {
		t.Errorf("v2_context.operation = %v, want k8s.resource.create", auditContext["operation"])
	}
	if taskUID, _ := auditContext["task_uid"].(string); taskUID != submitted.Data.Task.UID {
		t.Errorf("v2_context.task_uid = %v, want the submitted task uid %q (request↔audit join)", auditContext["task_uid"], submitted.Data.Task.UID)
	}
	if resourceUID, _ := auditContext["resource_uid"].(string); !strings.HasPrefix(resourceUID, "conn:"+connectionAuditUID+":") {
		t.Errorf("v2_context.resource_uid = %v, want the synthetic conn:<uid>: prefix", auditContext["resource_uid"])
	}
	if connUID, _ := auditContext["provider_connection_uid"].(string); connUID != connectionAuditUID {
		t.Errorf("v2_context.provider_connection_uid = %v, want %q", auditContext["provider_connection_uid"], connectionAuditUID)
	}
	if _, hasError := auditContext["error_code"]; hasError {
		t.Errorf("v2_context.error_code = %v on a 201 — want absent", auditContext["error_code"])
	}
}
