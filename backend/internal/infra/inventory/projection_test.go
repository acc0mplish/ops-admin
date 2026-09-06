// Projection query tests (plan §2 PR 22 file 2, §3.5 r2 공개 generation
// 규약): the public read point is the latest committed succeeded run
// (GenerationUID == run UID), partial generations stay unpublished (N8), and
// stale_source connections read as absent (§5.4c / A5).
package inventory_test

import (
	"testing"
	"time"

	"gorm.io/gorm"

	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/inventory"
	"ops-admin/backend/internal/infra/model"
)

// seedProjection plants one connection (optionally stale), its context, two
// resources, and observations under the given generation names. It returns
// the seeded DB handle alongside the context id and the commit timestamp.
func seedProjection(t *testing.T, stale bool) (db *gorm.DB, contextID uint, committed time.Time) {
	t.Helper()
	db = newInventoryDB(t)

	conn := model.ProviderConnection{
		UID: "conn-k8s", ProviderType: "kubernetes", Name: "seed", Endpoint: "https://127.0.0.1:6443",
		StaleSource: stale,
	}
	if err := db.Create(&conn).Error; err != nil {
		t.Fatalf("create connection: %v", err)
	}
	pctx := model.ProviderContext{UID: "ctx-k8s", ConnectionID: conn.ID, Kind: "cluster", ExternalID: "1", Name: "seed"}
	if err := db.Create(&pctx).Error; err != nil {
		t.Fatalf("create context: %v", err)
	}

	committed = time.Date(2026, 9, 6, 9, 0, 0, 0, time.UTC)
	runA := model.InventorySyncRun{
		UID: "gen-a", ConnectionID: conn.ID, ContextID: pctx.ID, Mode: "full",
		Status: inventory.RunStatusSucceeded, StartedAt: committed.Add(-time.Minute), CommittedAt: &committed,
	}
	if err := db.Create(&runA).Error; err != nil {
		t.Fatalf("create run A: %v", err)
	}
	partialAt := committed.Add(10 * time.Minute)
	runB := model.InventorySyncRun{
		UID: "gen-b-partial", ConnectionID: conn.ID, ContextID: pctx.ID, Mode: "full",
		Status: inventory.RunStatusPartial, StartedAt: committed, FinishedAt: &partialAt,
	}
	if err := db.Create(&runB).Error; err != nil {
		t.Fatalf("create run B: %v", err)
	}

	resA := model.InfraResource{
		UID: "res-node", ContextID: pctx.ID, Kind: "orchestration.node",
		ExternalID: "kind-control", ExternalURN: "urn:k8s:1:node:uid-1", DisplayName: "kind-control",
		FirstSeenAt: committed, LastSeenAt: committed,
	}
	resB := model.InfraResource{
		UID: "res-ns", ContextID: pctx.ID, Kind: "orchestration.namespace",
		ExternalID: "default", ExternalURN: "urn:k8s:1:namespace:default", DisplayName: "default",
		FirstSeenAt: committed, LastSeenAt: committed,
	}
	if err := db.Create(&resA).Error; err != nil {
		t.Fatalf("create resource A: %v", err)
	}
	if err := db.Create(&resB).Error; err != nil {
		t.Fatalf("create resource B: %v", err)
	}

	// gen-a: node observed healthy, namespace observed. gen-b (partial): node
	// changed to degraded — must stay unpublished.
	if err := db.Create(&model.ResourceObservation{
		ResourceID: resA.ID, GenerationUID: "gen-a", NormalizedJSON: contract.JSONMap{"healthState": "healthy"},
		ObservedAt: committed,
	}).Error; err != nil {
		t.Fatalf("create observation A node: %v", err)
	}
	if err := db.Create(&model.ResourceObservation{
		ResourceID: resA.ID, GenerationUID: "gen-b-partial", NormalizedJSON: contract.JSONMap{"healthState": "degraded"},
		ObservedAt: partialAt,
	}).Error; err != nil {
		t.Fatalf("create observation B node: %v", err)
	}
	if err := db.Create(&model.ResourceObservation{
		ResourceID: resB.ID, GenerationUID: "gen-a", NormalizedJSON: contract.JSONMap{"phase": "Active"},
		ObservedAt: committed,
	}).Error; err != nil {
		t.Fatalf("create observation A namespace: %v", err)
	}
	return db, pctx.ID, committed
}

func TestLatestAuthoritativeGenerationPicksCommittedRun(t *testing.T) {
	db, contextID, committed := seedProjection(t, false)

	gen, err := inventory.LatestAuthoritativeGeneration(db, contextID)
	if err != nil {
		t.Fatalf("LatestAuthoritativeGeneration: %v", err)
	}
	if gen.GenerationUID != "gen-a" {
		t.Fatalf("generation = %q, want gen-a", gen.GenerationUID)
	}
	if !gen.CommittedAt.Equal(committed) {
		t.Fatalf("committedAt = %v, want %v", gen.CommittedAt, committed)
	}
}

func TestLatestAuthoritativeGenerationWithoutCommittedRun(t *testing.T) {
	db := newInventoryDB(t)
	if _, err := inventory.LatestAuthoritativeGeneration(db, 999); err == nil {
		t.Fatalf("expected error for context without a committed run")
	}
}

// The projection publishes the full live inventory as of the latest committed
// generation — a partial run's observation never leaks (N8).
func TestProjectResourcesHidesPartialGenerations(t *testing.T) {
	db, contextID, _ := seedProjection(t, false)

	gen, err := inventory.LatestAuthoritativeGeneration(db, contextID)
	if err != nil {
		t.Fatalf("LatestAuthoritativeGeneration: %v", err)
	}
	rows, err := inventory.ProjectResources(db, contextID, gen.GenerationUID)
	if err != nil {
		t.Fatalf("ProjectResources: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("projected %d rows, want 2", len(rows))
	}
	byKind := map[string]inventory.ProjectedResource{}
	for _, row := range rows {
		byKind[row.Kind] = row
	}
	node, ok := byKind["orchestration.node"]
	if !ok {
		t.Fatalf("node missing from projection: %+v", rows)
	}
	if node.Normalized["healthState"] != "healthy" {
		t.Fatalf("partial generation leaked: node healthState = %v", node.Normalized["healthState"])
	}
	if node.GenerationUID != "gen-a" {
		t.Fatalf("node observation generation = %q, want gen-a", node.GenerationUID)
	}
	if ns, ok := byKind["orchestration.namespace"]; !ok || ns.Normalized["phase"] != "Active" {
		t.Fatalf("namespace projection wrong: %+v", byKind["orchestration.namespace"])
	}
}

// §5.4c — a stale_source connection reads as absent.
func TestProjectResourcesTreatsStaleSourceAsAbsent(t *testing.T) {
	db, contextID, _ := seedProjection(t, true)

	gen, err := inventory.LatestAuthoritativeGeneration(db, contextID)
	if err != nil {
		t.Fatalf("LatestAuthoritativeGeneration: %v", err)
	}
	rows, err := inventory.ProjectResources(db, contextID, gen.GenerationUID)
	if err != nil {
		t.Fatalf("ProjectResources: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("stale_source context must read as absent, got %d rows", len(rows))
	}
}
