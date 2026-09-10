package compose_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"ops-admin/backend/internal/infra/adapter/fake"
	"ops-admin/backend/internal/infra/compose"
	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/metrics"
	"ops-admin/backend/internal/infra/migrate"
	"ops-admin/backend/internal/infra/model"
	"ops-admin/backend/internal/infra/registry"
	"ops-admin/backend/internal/infra/secrets"
	"ops-admin/backend/internal/testutil"
	"ops-admin/backend/util"
)

// healthloop_test — P6 계기 델타 단얫(T-2): sweep 1회 → provider_health gauge
// 라인과 last_health_at 기록의 형상. fake 어댑터는 항상 Healthy=true를 반환
// 하므로 "1"은 바인딩이 맞아 해석된 커넥션, "0"은 바인딩 부재 커넥션과 DB
// 실패 회에서 나온다(계획 claim 10).

// seedHealthChain — connection(fake provider) → secret_ref → inventory
// binding. bind=false는 바인딩 없는 커넥션(해석 실패 갈래)을 남긴다. 형상은
// secrets 패키지 broker_test의 seedChain과 동일하다.
func seedHealthChain(t *testing.T, db *gorm.DB, connectionUID string, bind bool) {
	t.Helper()
	conn := model.ProviderConnection{UID: connectionUID, ProviderType: fake.ProviderName, Name: "seed", Endpoint: "https://seed:8006"}
	if err := db.Create(&conn).Error; err != nil {
		t.Fatalf("seed connection: %v", err)
	}
	if !bind {
		return
	}
	sealed, err := util.EncryptSecretV2("sweep-material")
	if err != nil {
		t.Fatalf("EncryptSecretV2: %v", err)
	}
	ref := model.SecretRef{UID: "sec-" + connectionUID, Backend: "internal", Ciphertext: sealed}
	if err := db.Create(&ref).Error; err != nil {
		t.Fatalf("seed secret_ref: %v", err)
	}
	binding := model.ProviderCredentialBinding{
		ProviderConnectionID: conn.ID,
		Purpose:              "inventory",
		SecretRefID:          ref.ID,
	}
	if err := db.Create(&binding).Error; err != nil {
		t.Fatalf("seed binding: %v", err)
	}
}

// newSweepStack migrates the full schema and assembles the stack — the
// sweeper shares the stack's registry·broker·counters (등록 정확 1회 계약).
func newSweepStack(t *testing.T, db *gorm.DB) (*compose.HealthSweeper, *metrics.Counters) {
	t.Helper()
	if err := migrate.Run(context.Background(), db); err != nil {
		t.Fatalf("migrate.Run: %v", err)
	}
	stack, err := compose.Build(db)
	if err != nil {
		t.Fatalf("compose.Build: %v", err)
	}
	return compose.NewHealthSweeper(db, stack.Registry, stack.Broker, stack.Counters), stack.Counters
}

func healthLine(render, uid, value string) string {
	return `provider_health{connection="` + uid + `"} ` + value
}

// fake 어댑터(항상 healthy) + inventory 바인딩 → gauge 1과 last_health_at
// 기록. 이 sweep이 그 컬럼의 기록 주체다(api/v2 읽기의 증분 원천).
func TestHealthSweepHealthyConnectionRecordsGaugeOne(t *testing.T) {
	testutil.PinSecretKeys(t)
	db := testutil.OpenMemoryDB(t)
	sweeper, counters := newSweepStack(t, db)
	seedHealthChain(t, db, "conn-healthy", true)

	if err := sweeper.SweepOnce(context.Background()); err != nil {
		t.Fatalf("SweepOnce: %v", err)
	}

	render := counters.Render()
	if want := healthLine(render, "conn-healthy", "1"); !strings.Contains(render, want) {
		t.Errorf("render missing %q:\n%s", want, render)
	}
	var conn model.ProviderConnection
	if err := db.Where("uid = ?", "conn-healthy").First(&conn).Error; err != nil {
		t.Fatalf("reload connection: %v", err)
	}
	if conn.LastHealthAt == nil {
		t.Error("last_health_at not recorded — the sweep must own the column on a healthy probe")
	}
}

// 바인딩 부재 커넥션 → gauge 0. last_health_at은 기록되지 않는다(healthy가
// 아니므로).
func TestHealthSweepMissingBindingRecordsZero(t *testing.T) {
	testutil.PinSecretKeys(t)
	db := testutil.OpenMemoryDB(t)
	sweeper, counters := newSweepStack(t, db)
	seedHealthChain(t, db, "conn-unbound", false)

	if err := sweeper.SweepOnce(context.Background()); err != nil {
		t.Fatalf("SweepOnce: %v", err)
	}

	render := counters.Render()
	if want := healthLine(render, "conn-unbound", "0"); !strings.Contains(render, want) {
		t.Errorf("render missing %q:\n%s", want, render)
	}
	var conn model.ProviderConnection
	if err := db.Where("uid = ?", "conn-unbound").First(&conn).Error; err != nil {
		t.Fatalf("reload connection: %v", err)
	}
	if conn.LastHealthAt != nil {
		t.Errorf("last_health_at = %v, want nil — an unhealthy probe must not stamp the column", conn.LastHealthAt)
	}
}

// DB 실패 회(claim 10의 DB 실패 갈래): 정상 sweep이 관측한 커넥션이 다음 회의
// 행 로드 실패에서 0으로 뒤집힌다. gauge 적립이 실패 소화 경로다.
func TestHealthSweepDBFailureMarksObservedZero(t *testing.T) {
	testutil.PinSecretKeys(t)
	db := testutil.OpenMemoryDB(t)
	sweeper, counters := newSweepStack(t, db)
	seedHealthChain(t, db, "conn-gone", true)

	if err := sweeper.SweepOnce(context.Background()); err != nil {
		t.Fatalf("first SweepOnce: %v", err)
	}
	if err := db.Migrator().DropTable(&model.ProviderConnection{}); err != nil {
		t.Fatalf("DropTable provider_connection: %v", err)
	}
	if err := sweeper.SweepOnce(context.Background()); err == nil {
		t.Fatal("SweepOnce over a dropped table returned nil — the load failure must surface")
	}

	render := counters.Render()
	if want := healthLine(render, "conn-gone", "0"); !strings.Contains(render, want) {
		t.Errorf("render missing %q after the DB failure:\n%s", want, render)
	}
}

// Stop은 기동 루프를 취소하고 goroutine 종료를 기다린다 — 두 번째 Stop은
// no-op다(Start→Stop 순서 계약은 호출부 소유).
func TestHealthSweeperStartStopCancelsLoop(t *testing.T) {
	testutil.PinSecretKeys(t)
	db := testutil.OpenMemoryDB(t)
	sweeper, _ := newSweepStack(t, db)

	sweeper.Start(context.Background())
	done := make(chan struct{})
	go func() {
		sweeper.Stop()
		sweeper.Stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Stop did not return — the loop goroutine must exit on stop")
	}
}

// TestSweepOnceRemovesVanishedSeries — ④리뷰 LOW(gauge.go): 관측 후 DB 행이
// 사라진 커넥션의 provider_health 시리즈는 다음 sweep에서 소거된다 — 마지막
// 값 라인이 프로세스 수명 내내 잔존하지 않는다.
func TestSweepOnceRemovesVanishedSeries(t *testing.T) {
	testutil.PinSecretKeys(t)
	db := testutil.OpenMemoryDB(t)
	sweeper, counters := newSweepStack(t, db)
	seedHealthChain(t, db, "conn-vanish", true)

	if err := sweeper.SweepOnce(context.Background()); err != nil {
		t.Fatalf("first sweep: %v", err)
	}
	if want := healthLine(counters.Render(), "conn-vanish", "1"); !strings.Contains(counters.Render(), want) {
		t.Fatalf("seeded connection must render healthy=1:\n%s", counters.Render())
	}

	// 행 삭제 → 재 sweep → 시리즈 소거.
	if err := db.Unscoped().Where("uid = ?", "conn-vanish").Delete(&model.ProviderConnection{}).Error; err != nil {
		t.Fatalf("delete connection: %v", err)
	}
	if err := sweeper.SweepOnce(context.Background()); err != nil {
		t.Fatalf("second sweep: %v", err)
	}
	if render := counters.Render(); strings.Contains(render, `provider_health{connection="conn-vanish"}`) {
		t.Fatalf("vanished connection series must be pruned:\n%s", render)
	}
}

// TestStopBeforeStartIsSafe — ④리뷰 LOW(healthloop.go:92-100): Start 이전
// Stop은 교착 없이 즉시 반환한다.
func TestStopBeforeStartIsSafe(t *testing.T) {
	testutil.PinSecretKeys(t)
	db := testutil.OpenMemoryDB(t)
	sweeper, _ := newSweepStack(t, db)
	done := make(chan struct{})
	go func() {
		sweeper.Stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop before Start must not block")
	}
}

// observationAdapter — 관측 보고 어댑터(compose_test 한정, ④ A′-1 P1-G1).
// Health가 Healthy + 파생 관측 고정치를 반환한다. "fake"는 M1 어휘의 유일한
// 테스트 슬롯이라 새 레지스트리에 이 어댑터로 등록한다(compose.Build의 stack
// 레지스트리는 fake를 점유 — 재등록은 중복 에러).
type observationAdapter struct{}

func (observationAdapter) Descriptor() contract.ProviderTypeDescriptor {
	return contract.ProviderTypeDescriptor{Type: fake.ProviderName, AdapterVersion: "test", ProtocolVersion: "test"}
}

func (observationAdapter) Validate(_ context.Context, _ contract.ConnectionView) error { return nil }

func (observationAdapter) Health(_ context.Context, _ contract.ConnectionView) contract.HealthResult {
	return contract.HealthResult{
		Healthy: true,
		Message: "observation fixture",
		Observation: contract.JSONMap{"certificates": []contract.JSONMap{
			{"name": "CA Certificate", "type": "certificate-authority", "subject": "sweep-ca", "daysRemaining": 364},
		}},
	}
}

func (observationAdapter) Close() error { return nil }

// newObservationSweepStack — observationAdapter를 등록한 새 레지스트리로 조립한
// sweeper다. 생성자는 compose.Build와 같은 계약(registry.New·secrets.NewBroker·
// metrics.New)이다.
func newObservationSweepStack(t *testing.T, db *gorm.DB) *compose.HealthSweeper {
	t.Helper()
	counters := metrics.New()
	reg := registry.New()
	adapter := observationAdapter{}
	if err := reg.RegisterProviderType(adapter.Descriptor(), adapter); err != nil {
		t.Fatalf("register observationAdapter: %v", err)
	}
	return compose.NewHealthSweeper(db, reg, secrets.NewBroker(db), counters)
}

// P1-G1 (④ A′-1): sweep이 어댑터 관측을 provider_connection.ConfigJSON의
// health_observation 키에 병합 기록한다 — 기존 ConfigJSON 키는 보존되고
// last_health_at도 같은 회에 기록된다(markLastHealthAt와 같은 쓰기 패턴).
func TestHealthSweepRecordsObservation(t *testing.T) {
	testutil.PinSecretKeys(t)
	db := testutil.OpenMemoryDB(t)
	if err := migrate.Run(context.Background(), db); err != nil {
		t.Fatalf("migrate.Run: %v", err)
	}
	sweeper := newObservationSweepStack(t, db)
	seedHealthChain(t, db, "conn-observed", true)
	if err := db.Model(&model.ProviderConnection{}).Where("uid = ?", "conn-observed").
		Select("config_json").Updates(model.ProviderConnection{ConfigJSON: contract.JSONMap{"monitor_datasource_id": "ds-1", "node_count": 3}}).Error; err != nil {
		t.Fatalf("seed config_json: %v", err)
	}

	if err := sweeper.SweepOnce(context.Background()); err != nil {
		t.Fatalf("SweepOnce: %v", err)
	}

	var conn model.ProviderConnection
	if err := db.Where("uid = ?", "conn-observed").First(&conn).Error; err != nil {
		t.Fatalf("reload connection: %v", err)
	}
	observation, ok := conn.ConfigJSON["health_observation"].(map[string]any)
	if !ok {
		t.Fatalf("config_json = %#v, want health_observation key", conn.ConfigJSON)
	}
	entries, ok := observation["certificates"].([]any)
	if !ok || len(entries) != 1 {
		t.Fatalf("observation = %#v, want 1 certificate entry", observation)
	}
	entry, _ := entries[0].(map[string]any)
	if entry["name"] != "CA Certificate" || entry["subject"] != "sweep-ca" {
		t.Errorf("certificate entry = %#v, want CA Certificate/sweep-ca", entry)
	}
	// 병합 — 기존 키는 보존된다(관측 전용 키 덮어쓰기가 ConfigJSON을 몰아내지 않는다).
	if conn.ConfigJSON["monitor_datasource_id"] != "ds-1" {
		t.Errorf("monitor_datasource_id = %v, want preserved", conn.ConfigJSON["monitor_datasource_id"])
	}
	if conn.LastHealthAt == nil {
		t.Error("last_health_at not recorded — observation and stamp share the healthy sweep")
	}
}

// 관측 미산출 어댑터(stack의 fake — Observation nil)는 키를 기록하지 않는다.
func TestHealthSweepWithoutObservationLeavesConfigUntouched(t *testing.T) {
	testutil.PinSecretKeys(t)
	db := testutil.OpenMemoryDB(t)
	sweeper, _ := newSweepStack(t, db)
	seedHealthChain(t, db, "conn-no-obs", true)

	if err := sweeper.SweepOnce(context.Background()); err != nil {
		t.Fatalf("SweepOnce: %v", err)
	}

	var conn model.ProviderConnection
	if err := db.Where("uid = ?", "conn-no-obs").First(&conn).Error; err != nil {
		t.Fatalf("reload connection: %v", err)
	}
	if _, exists := conn.ConfigJSON["health_observation"]; exists {
		t.Errorf("config_json = %#v, want no health_observation key from an observation-less adapter", conn.ConfigJSON)
	}
}
