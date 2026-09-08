package secrets

// §18.2 L 계기 델타 단얫(계획 T-2): 성공 Resolve만 secret_access_total의
// {purpose, backend} 라인을 남긴다(가정 A5 — 실패는 미가산). 헨리스는
// broker_test.go를 공유한다(seedChain·testutil).

import (
	"context"
	"strings"
	"testing"

	"ops-admin/backend/internal/infra/metrics"
	"ops-admin/backend/internal/infra/model"
	"ops-admin/backend/internal/testutil"
	"ops-admin/backend/util"
)

// TestBrokerObservesSecretAccess — one successful Resolve increments the
// purpose·backend line once; a failed Resolve never does. NewBroker without
// counters keeps rendering nothing (nil 주입 보존).
func TestBrokerObservesSecretAccess(t *testing.T) {
	testutil.PinSecretKeys(t)
	db := testutil.OpenMemoryDB(t)
	if err := db.AutoMigrate(&model.ProviderConnection{}, &model.ProviderCredentialBinding{}, &model.SecretRef{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	sealed, err := util.EncryptSecretV2("test-material")
	if err != nil {
		t.Fatalf("EncryptSecretV2: %v", err)
	}
	seedChain(t, db, "conn-1", "inventory", sealed)

	counters := metrics.New()
	broker := NewBrokerWithCounters(db, counters)

	// 실패 경로 — 존재하지 않는 커넥션: 미가산(가정 A5).
	if _, err := broker.Resolve(context.Background(), "conn-missing", "inventory"); err == nil {
		t.Fatal("missing connection resolved, want an error")
	}
	if render := counters.Render(); strings.Contains(render, "secret_access_total") {
		t.Errorf("failed Resolve must not increment the family:\n%s", render)
	}

	// 성공 경로 — purpose·backend 라벨로 1회 가산(backend는 M1 "internal").
	if _, err := broker.Resolve(context.Background(), "conn-1", "inventory"); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	render := counters.Render()
	if !strings.Contains(render, `secret_access_total{purpose="inventory",backend="internal"} 1`) {
		t.Errorf("render missing the success line with value 1:\n%s", render)
	}

	// nil 주입 보존 — counters 없는 broker는 렌더에 기여하지 않는다.
	bare := NewBroker(db)
	if _, err := bare.Resolve(context.Background(), "conn-1", "inventory"); err != nil {
		t.Fatalf("bare Resolve: %v", err)
	}
	if got := counters.Render(); !strings.Contains(got, `secret_access_total{purpose="inventory",backend="internal"} 1`) {
		t.Errorf("bare broker must not touch the counter set, line value changed:\n%s", got)
	}
}
