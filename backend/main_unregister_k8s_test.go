package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"
	"ops-admin/backend/internal/infra/model"
)

// unregister-k8s 테스트 — del-plan 계약 단얫. 하니스(register mock·in-memory
// DB·체인 카운트)는 main_register_k8s_test.go 승계(같은 패키지 재사용 —
// 보존 제약상 그 파일 자체는 무변경). 삭제 경로는 material을 읽지 않으므로
// mock 서버 종료 후에도 삭제가 성공해야 한다(라이브 스캔 부재의 행동 단얫).

func registerK8sForUnregister(t *testing.T, db *gorm.DB, name, server string) k8sRegisterReport {
	t.Helper()
	report, err := registerK8sInDB(context.Background(), db, k8sRegisterOptions{
		Name: name, KubeconfigPath: writeTempKubeconfig(t, kubeconfigForServer(server)),
		ConnectionMode: k8sModeDirect,
	})
	if err != nil {
		t.Fatalf("registerK8sInDB: %v", err)
	}
	return report
}

func seedUnregisterTask(t *testing.T, db *gorm.DB, connUID, status string) model.ProviderTask {
	t.Helper()
	task := model.ProviderTask{
		UID:           "task-unreg-" + status,
		OperationName: "k8s.configmap.create",
		ResourceUID:   "conn:" + connUID + ":configmap:default/app",
		Status:        status,
		MaxAttempts:   1,
	}
	if err := db.Create(&task).Error; err != nil {
		t.Fatalf("seed provider_task(%s): %v", status, err)
	}
	return task
}

// TestUnregisterK8sRoundTripKeepsDeterministicUID — 등록→삭제→재등록 왕복.
// UID는 k8sRegisterUID 결정론이므로 재등록이 최초와 같은 UID로 복구하고,
// 삭제는 mock 서버가 닫힌 뒤에도 성공한다(삭제 경로 라이브 스캔 부재).
func TestUnregisterK8sRoundTripKeepsDeterministicUID(t *testing.T) {
	mock := &k8sRegisterMock{}
	srv := mock.serve(t)
	db := newRegisterK8sTestDB(t)

	first := registerK8sForUnregister(t, db, "k8s-mock", srv.URL)

	// 서버 종료 후 삭제 — 라이브 스캔이 있으면 여기서 실패한다.
	srv.Close()
	report, err := unregisterK8sInDB(db, "k8s-mock")
	if err != nil {
		t.Fatalf("unregisterK8sInDB: %v", err)
	}
	if !report.Existed {
		t.Fatalf("chain existed — report.Existed = false")
	}
	if report.ConnectionUID != first.ConnectionUID || report.ContextUID != first.ContextUID {
		t.Fatalf("unregister must target the registered chain UIDs: %+v vs %+v", first, report)
	}
	if report.TombstonedResources != 0 {
		t.Fatalf("no inventory resources were seeded — tombstonedResources = %d, want 0", report.TombstonedResources)
	}
	assertChainCounts(t, db, map[string]int64{
		"provider_connection": 0, "provider_context": 0, "secret_ref": 0, "provider_credential_binding": 0,
	})

	// stdout 보고에 kubeconfig 성분(토큰·서버 URL)이 타지 않는다 — 보고 구조는
	// 4필드로 고정되어 있다.
	blob, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	if strings.Contains(string(blob), "mock-k8s-token") || strings.Contains(string(blob), srv.URL) {
		t.Fatalf("report carries kubeconfig material: %s", blob)
	}

	// 재등록 — 같은 --name이 같은 UID로 체인을 복구한다(삭제→재등록 동일 UID).
	mock2 := &k8sRegisterMock{}
	srv2 := mock2.serve(t)
	second := registerK8sForUnregister(t, db, "k8s-mock", srv2.URL)
	if second.ConnectionUID != first.ConnectionUID || second.ContextUID != first.ContextUID {
		t.Fatalf("re-register must restore the same UIDs: %+v vs %+v", first, second)
	}
	assertChainCounts(t, db, map[string]int64{
		"provider_connection": 1, "provider_context": 1, "secret_ref": 1, "provider_credential_binding": 2,
	})
}

// TestUnregisterK8sMissingChainIsIdempotent — 미존재는 오류가 아니라
// existed:false 보고다(upsert 멱등 철학 승계). 어느 체인 테이블에도 행이
// 생기지 않는다.
func TestUnregisterK8sMissingChainIsIdempotent(t *testing.T) {
	db := newRegisterK8sTestDB(t)

	report, err := unregisterK8sInDB(db, "k8s-absent")
	if err != nil {
		t.Fatalf("missing chain must not error: %v", err)
	}
	if report.Existed {
		t.Fatalf("report.Existed = true, want false for an absent chain")
	}
	if report.TombstonedResources != 0 {
		t.Fatalf("tombstonedResources = %d, want 0", report.TombstonedResources)
	}
	assertChainCounts(t, db, map[string]int64{
		"provider_connection": 0, "provider_context": 0, "secret_ref": 0, "provider_credential_binding": 0,
	})
}

// TestUnregisterK8sGuardBlocksNonTerminalTasks — 커넥션 스코프에 비종단
// provider_task가 남아 있으면 삭제를 거부하고 체인을 하나도 고치지 않는다.
func TestUnregisterK8sGuardBlocksNonTerminalTasks(t *testing.T) {
	mock := &k8sRegisterMock{}
	srv := mock.serve(t)
	db := newRegisterK8sTestDB(t)
	report := registerK8sForUnregister(t, db, "k8s-mock", srv.URL)
	task := seedUnregisterTask(t, db, report.ConnectionUID, "queued")

	if _, err := unregisterK8sInDB(db, "k8s-mock"); err == nil {
		t.Fatalf("non-terminal task must block unregister")
	}
	assertChainCounts(t, db, map[string]int64{
		"provider_connection": 1, "provider_context": 1, "secret_ref": 1, "provider_credential_binding": 2,
	})
	var still model.ProviderTask
	if err := db.First(&still, task.ID).Error; err != nil {
		t.Fatalf("guard must leave the task in place: %v", err)
	}
}

// TestUnregisterK8sTerminalTasksDoNotBlockAndHistorySurvives — 종단 태스크는
// 가드를 통과시키고, task·task_event·task_attempt 이력 행은 삭제 후에도
// 보존된다(톰스톤 경로는 이력 테이블을 건드리지 않는다).
func TestUnregisterK8sTerminalTasksDoNotBlockAndHistorySurvives(t *testing.T) {
	mock := &k8sRegisterMock{}
	srv := mock.serve(t)
	db := newRegisterK8sTestDB(t)
	report := registerK8sForUnregister(t, db, "k8s-mock", srv.URL)
	task := seedUnregisterTask(t, db, report.ConnectionUID, "succeeded")
	event := model.TaskEvent{TaskID: task.ID, Type: "status_changed", At: time.Now().UTC()}
	if err := db.Create(&event).Error; err != nil {
		t.Fatalf("seed task_event: %v", err)
	}
	attempt := model.TaskAttempt{TaskID: task.ID, AttemptNo: 1, StartedAt: time.Now().UTC()}
	if err := db.Create(&attempt).Error; err != nil {
		t.Fatalf("seed task_attempt: %v", err)
	}

	if _, err := unregisterK8sInDB(db, "k8s-mock"); err != nil {
		t.Fatalf("terminal task must not block unregister: %v", err)
	}
	assertChainCounts(t, db, map[string]int64{
		"provider_connection": 0, "provider_context": 0, "secret_ref": 0, "provider_credential_binding": 0,
	})
	var survived model.ProviderTask
	if err := db.First(&survived, task.ID).Error; err != nil {
		t.Fatalf("task history must survive the delete: %v", err)
	}
	var events, attempts int64
	if err := db.Model(&model.TaskEvent{}).Count(&events).Error; err != nil {
		t.Fatalf("count task_event: %v", err)
	}
	if err := db.Model(&model.TaskAttempt{}).Count(&attempts).Error; err != nil {
		t.Fatalf("count task_attempt: %v", err)
	}
	if events != 1 || attempts != 1 {
		t.Fatalf("task_event = %d, task_attempt = %d, want 1·1 preserved", events, attempts)
	}
}

// TestUnregisterK8sTombstonesResourcesPreservingObservations — context 산하
// infra_resource는 톰스톤(ManagedState "tombstoned" + DeletedAt)되고 행은
// 남는다. 이미 톰스톤인 행은 세지 않고 그대로 두며, resource_observation과
// inventory_sync_run은 건드리지 않는다.
func TestUnregisterK8sTombstonesResourcesPreservingObservations(t *testing.T) {
	mock := &k8sRegisterMock{}
	srv := mock.serve(t)
	db := newRegisterK8sTestDB(t)
	report := registerK8sForUnregister(t, db, "k8s-mock", srv.URL)

	var pctx model.ProviderContext
	if err := db.Where("uid = ?", report.ContextUID).First(&pctx).Error; err != nil {
		t.Fatalf("load context: %v", err)
	}
	past := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	seed := func(uid, urn, managed string, deleted *time.Time) model.InfraResource {
		t.Helper()
		res := model.InfraResource{
			UID: uid, ContextID: pctx.ID, Kind: "orchestration.node",
			ExternalURN: urn, Name: uid, ManagedState: managed, DeletedAt: deleted,
		}
		if err := db.Create(&res).Error; err != nil {
			t.Fatalf("seed infra_resource(%s): %v", uid, err)
		}
		return res
	}
	live1 := seed("res-live-1", "k8s://mock/node/a", "managed", nil)
	live2 := seed("res-live-2", "k8s://mock/node/b", "discovered", nil)
	gone := seed("res-gone", "k8s://mock/node/c", "tombstoned", &past)
	for _, res := range []model.InfraResource{live1, live2} {
		obs := model.ResourceObservation{ResourceID: res.ID, GenerationUID: "gen-1", ObservedAt: time.Now().UTC()}
		if err := db.Create(&obs).Error; err != nil {
			t.Fatalf("seed resource_observation: %v", err)
		}
	}
	syncRun := model.InventorySyncRun{UID: "run-unreg", ConnectionID: 0, ContextID: pctx.ID, Mode: "full", Status: "succeeded", StartedAt: time.Now().UTC()}
	if err := db.Create(&syncRun).Error; err != nil {
		t.Fatalf("seed inventory_sync_run: %v", err)
	}

	out, err := unregisterK8sInDB(db, "k8s-mock")
	if err != nil {
		t.Fatalf("unregisterK8sInDB: %v", err)
	}
	if out.TombstonedResources != 2 {
		t.Fatalf("tombstonedResources = %d, want 2 (live rows only)", out.TombstonedResources)
	}
	var resources []model.InfraResource
	if err := db.Where("context_id = ?", pctx.ID).Find(&resources).Error; err != nil {
		t.Fatalf("load infra_resource: %v", err)
	}
	if len(resources) != 3 {
		t.Fatalf("infra_resource rows = %d, want 3 (톰스톤은 행 소멸이 아니다)", len(resources))
	}
	byUID := map[string]model.InfraResource{}
	for _, res := range resources {
		byUID[res.UID] = res
	}
	for _, uid := range []string{"res-live-1", "res-live-2", "res-gone"} {
		res := byUID[uid]
		if res.ManagedState != "tombstoned" || res.DeletedAt == nil {
			t.Fatalf("%s must be tombstoned with DeletedAt: %+v", uid, res)
		}
	}
	if !gone.DeletedAt.Equal(past) {
		t.Fatalf("already-tombstoned row must keep its original DeletedAt: %v", gone.DeletedAt)
	}
	var obsCount, runCount int64
	if err := db.Model(&model.ResourceObservation{}).Count(&obsCount).Error; err != nil {
		t.Fatalf("count resource_observation: %v", err)
	}
	if err := db.Model(&model.InventorySyncRun{}).Count(&runCount).Error; err != nil {
		t.Fatalf("count inventory_sync_run: %v", err)
	}
	if obsCount != 2 || runCount != 1 {
		t.Fatalf("observations = %d, sync runs = %d, want 2·1 preserved", obsCount, runCount)
	}
	// 소각 — secret_ref ciphertext 행은 사라진다.
	var refCount int64
	if err := db.Model(&model.SecretRef{}).Where("uid = ?", k8sRegisterUID("k8s-mock", "secret")).Count(&refCount).Error; err != nil {
		t.Fatalf("count secret_ref: %v", err)
	}
	if refCount != 0 {
		t.Fatalf("secret_ref rows = %d, want 0 (ciphertext 소각)", refCount)
	}
}

// TestRunUnregisterK8sRequiresNameAndConfirm — 플래그·확인 게이트는 config나
// DB를 만지기 전에.exit 1. --confirm이 없으면 --name이 있어도 같은 자리에서
// 멈춘다(재암호화 --backup-acknowledged 선례의 소각판).
func TestRunUnregisterK8sRequiresNameAndConfirm(t *testing.T) {
	if code := runUnregisterK8s([]string{"--undefined-flag"}); code != 1 {
		t.Fatalf("undefined flag must exit 1, got %d", code)
	}
	if code := runUnregisterK8s(nil); code != 1 {
		t.Fatalf("missing required flags must exit 1, got %d", code)
	}
	if code := runUnregisterK8s([]string{"--confirm"}); code != 1 {
		t.Fatalf("missing --name must exit 1, got %d", code)
	}
	if code := runUnregisterK8s([]string{"--name", "k8s-x"}); code != 1 {
		t.Fatalf("missing --confirm must exit 1, got %d", code)
	}
	if code := runUnregisterK8s([]string{"--name", "k8s-x", "--config", "/nonexistent/config.yaml"}); code != 1 {
		t.Fatalf("--confirm gate must fire before config is read, got %d", code)
	}
	notice := captureStderr(t, func() {
		if code := runUnregisterK8s([]string{"--name", "k8s-x"}); code != 1 {
			t.Errorf("gate exit = %d, want 1", code)
		}
	})
	if !strings.Contains(notice, "--confirm") {
		t.Fatalf("gate notice must point at --confirm, got %q", notice)
	}
}

// TestUnregisterK8sGuardBlocksResourceScopedTasks — 가드 양분기(리뷰 M):
// conn: 합성 접두가 아닌 리소스 스코프(resource_uid = infra_resource.uid)의
// 비종단 태스크도 삭제를 막고, 그 태스크가 종단으로 전이하면 통과한다.
func TestUnregisterK8sGuardBlocksResourceScopedTasks(t *testing.T) {
	mock := &k8sRegisterMock{}
	srv := mock.serve(t)
	db := newRegisterK8sTestDB(t)
	report := registerK8sForUnregister(t, db, "k8s-mock", srv.URL)

	var pctx model.ProviderContext
	if err := db.Where("uid = ?", report.ContextUID).First(&pctx).Error; err != nil {
		t.Fatalf("load context: %v", err)
	}
	res := model.InfraResource{
		UID: "res-task", ContextID: pctx.ID, Kind: "orchestration.node",
		ExternalURN: "k8s://mock/node/t", Name: "res-task", ManagedState: "managed",
	}
	if err := db.Create(&res).Error; err != nil {
		t.Fatalf("seed infra_resource: %v", err)
	}
	task := model.ProviderTask{
		UID: "task-res-nonterminal", OperationName: "k8s.pod.restart",
		ResourceUID: res.UID, Status: "running", MaxAttempts: 1,
	}
	if err := db.Create(&task).Error; err != nil {
		t.Fatalf("seed provider_task: %v", err)
	}

	// 비종단 → 삭제 거부·체인 보존.
	if _, err := unregisterK8sInDB(db, "k8s-mock"); err == nil {
		t.Fatalf("resource-scoped non-terminal task must block unregister")
	}
	assertChainCounts(t, db, map[string]int64{
		"provider_connection": 1, "provider_context": 1, "secret_ref": 1, "provider_credential_binding": 2,
	})

	// 종단 전이 → 통과.
	if err := db.Model(&model.ProviderTask{}).Where("id = ?", task.ID).
		Update("status", "succeeded").Error; err != nil {
		t.Fatalf("settle task: %v", err)
	}
	if _, err := unregisterK8sInDB(db, "k8s-mock"); err != nil {
		t.Fatalf("terminal resource-scoped task must not block: %v", err)
	}
	assertChainCounts(t, db, map[string]int64{
		"provider_connection": 0, "provider_context": 0, "secret_ref": 0, "provider_credential_binding": 0,
	})
}

// TestUnregisterK8sLeavesSiblingConnectionsUntouched — 타 커넥션과 그 리소스의
// 비종단 태스크가 있어도 본 커넥션 삭제는 성공하고, 타 체인·리소스·태스크는
// 한 행도 건드리지 않는다(가드 서브쿼리·톰스톤 WHERE의 교차 격리 단얫).
func TestUnregisterK8sLeavesSiblingConnectionsUntouched(t *testing.T) {
	mock := &k8sRegisterMock{}
	srv := mock.serve(t)
	db := newRegisterK8sTestDB(t)
	registerK8sForUnregister(t, db, "k8s-mock", srv.URL)
	sibling := registerK8sForUnregister(t, db, "k8s-sibling", srv.URL)

	var sibCtx model.ProviderContext
	if err := db.Where("uid = ?", sibling.ContextUID).First(&sibCtx).Error; err != nil {
		t.Fatalf("load sibling context: %v", err)
	}
	sibRes := model.InfraResource{
		UID: "res-sibling", ContextID: sibCtx.ID, Kind: "orchestration.node",
		ExternalURN: "k8s://mock/node/s", Name: "res-sibling", ManagedState: "managed",
	}
	if err := db.Create(&sibRes).Error; err != nil {
		t.Fatalf("seed sibling resource: %v", err)
	}
	sibTask := model.ProviderTask{
		UID: "task-sibling", OperationName: "k8s.pod.restart",
		ResourceUID: sibRes.UID, Status: "queued", MaxAttempts: 1,
	}
	if err := db.Create(&sibTask).Error; err != nil {
		t.Fatalf("seed sibling task: %v", err)
	}

	if _, err := unregisterK8sInDB(db, "k8s-mock"); err != nil {
		t.Fatalf("sibling activity must not block this unregister: %v", err)
	}
	// 남은 체인 행은 전부 sibling 것이다(mock 소거·sibling 불변).
	assertChainCounts(t, db, map[string]int64{
		"provider_connection": 1, "provider_context": 1, "secret_ref": 1, "provider_credential_binding": 2,
	})
	var kept model.ProviderTask
	if err := db.First(&kept, sibTask.ID).Error; err != nil {
		t.Fatalf("sibling task must survive: %v", err)
	}
	if kept.Status != "queued" {
		t.Fatalf("sibling task must be untouched: %+v", kept)
	}
	var alive model.InfraResource
	if err := db.Where("uid = ?", "res-sibling").First(&alive).Error; err != nil {
		t.Fatalf("sibling resource must survive: %v", err)
	}
	if alive.ManagedState != "managed" || alive.DeletedAt != nil {
		t.Fatalf("sibling resource must stay live: %+v", alive)
	}
}
