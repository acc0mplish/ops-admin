// Package v2 tests (plan §6 T52): the V2 read routes are additive GET-only
// surface over the V2 tables — responses carry no secret material (claim 14
// redaction), stale_source rows are absent from every read (§5.4c), the kind
// filter and paging behave, and the registered set is exactly the four planned
// routes. The v1 437-route invariance is enforced separately by the router
// golden (routes_inventory_test.go byte diff — only the v2 additions appear).
package v2_test

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

	"ops-admin/backend/internal/api/v2"
	"ops-admin/backend/internal/infra/migrate"
	"ops-admin/backend/internal/infra/model"
	"ops-admin/backend/internal/infra/registry"
	"ops-admin/backend/internal/testutil"
)

func newRouter(t *testing.T, db *gorm.DB) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	if err := migrate.Run(context.Background(), db); err != nil {
		t.Fatalf("migrate.Run: %v", err)
	}
	engine := gin.New()
	v2.NewInfraAPI(db).Register(engine.Group("/api/v2/infra"))
	return engine
}

func doGet(t *testing.T, engine *gin.Engine, path string) (int, *httptest.ResponseRecorder) {
	t.Helper()
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec.Code, rec
}

// decode unwraps the v1 JSON envelope (httpx.Success — {code,message,data}).
func decode(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
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

// seedInfraFixture writes one live kubernetes connection (context + secret
// binding) and one stale-source connection with its own context and resource.
// The stale side must never appear in any v2 read (§5.4c).
func seedInfraFixture(t *testing.T, db *gorm.DB) (liveResourceUID, staleResourceUID string) {
	t.Helper()
	now := time.Now()
	live := model.ProviderConnection{
		UID: "conn-live", ProviderType: "kubernetes", Name: "seed-cluster",
		Endpoint: "https://127.0.0.1:6443", Status: "active", Version: "v1.29.4",
		StaleSource: false, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&live).Error; err != nil {
		t.Fatal(err)
	}
	stale := model.ProviderConnection{
		UID: "conn-stale", ProviderType: "kubernetes", Name: "retired-cluster",
		Endpoint: "https://127.0.0.2:6443", Status: "active",
		SourceModel: "k8s_cluster", SourceID: 7, StaleSource: true,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&stale).Error; err != nil {
		t.Fatal(err)
	}
	liveContext := model.ProviderContext{UID: "ctx-live", ConnectionID: live.ID, Kind: "cluster", Name: "seed-cluster"}
	if err := db.Create(&liveContext).Error; err != nil {
		t.Fatal(err)
	}
	staleContext := model.ProviderContext{UID: "ctx-stale", ConnectionID: stale.ID, Kind: "cluster", Name: "retired-cluster"}
	if err := db.Create(&staleContext).Error; err != nil {
		t.Fatal(err)
	}
	secret := model.SecretRef{UID: "secr-live", Ciphertext: "v2-envelope-ciphertext-marker", KeyID: "key-1"}
	if err := db.Create(&secret).Error; err != nil {
		t.Fatal(err)
	}
	binding := model.ProviderCredentialBinding{
		ProviderConnectionID: live.ID, Purpose: "inventory", SecretRefID: secret.ID, Status: "active",
	}
	if err := db.Create(&binding).Error; err != nil {
		t.Fatal(err)
	}
	liveResource := model.InfraResource{
		UID: "res-live", ContextID: liveContext.ID, Kind: "orchestration.node",
		ExternalURN: "urn:k8s:seed-cluster:node:uid-1", DisplayName: "seed-control-plane",
		LifecycleState: "running", HealthState: "healthy", ManagedState: "discovered",
		FirstSeenAt: now, LastSeenAt: now,
	}
	staleResource := model.InfraResource{
		UID: "res-stale", ContextID: staleContext.ID, Kind: "orchestration.node",
		ExternalURN: "urn:k8s:retired-cluster:node:uid-2", DisplayName: "retired-node",
		FirstSeenAt: now, LastSeenAt: now,
	}
	otherKind := model.InfraResource{
		UID: "res-live-pod", ContextID: liveContext.ID, Kind: "orchestration.pod",
		ExternalURN: "urn:k8s:seed-cluster:pod:default/worker", DisplayName: "worker",
		FirstSeenAt: now, LastSeenAt: now,
	}
	for _, res := range []*model.InfraResource{&liveResource, &staleResource, &otherKind} {
		if err := db.Create(res).Error; err != nil {
			t.Fatal(err)
		}
	}
	observation := model.ResourceObservation{
		ResourceID: liveResource.ID, GenerationUID: "res-live", ObservationHash: "hash-1",
		NormalizerVersion: "1", NormalizedJSON: map[string]any{"pod.phase": "Running"},
		ObservedAt: now,
	}
	if err := db.Create(&observation).Error; err != nil {
		t.Fatal(err)
	}
	return liveResource.UID, staleResource.UID
}

func newFixtureRouter(t *testing.T) (*gin.Engine, string, string) {
	t.Helper()
	db := testutil.OpenMemoryDB(t)
	engine := newRouter(t, db)
	liveUID, staleUID := seedInfraFixture(t, db)
	return engine, liveUID, staleUID
}

// TestV2RoutesRegisteredExactlyFour pins the additive surface: the planned
// J7 GET subset and nothing else (§16.1 — the remaining four GETs and the
// POST routes are Phase 3, §13).
func TestV2RoutesRegisteredExactlyFour(t *testing.T) {
	engine, _, _ := newFixtureRouter(t)
	got := map[string]bool{}
	for _, route := range engine.Routes() {
		if route.Method != http.MethodGet {
			t.Fatalf("non-GET v2 route registered: %s %s", route.Method, route.Path)
		}
		got[route.Path] = true
	}
	for _, path := range []string{
		"/api/v2/infra/provider-types",
		"/api/v2/infra/provider-connections",
		"/api/v2/infra/resources",
		"/api/v2/infra/resources/:uid",
	} {
		if !got[path] {
			t.Fatalf("missing planned v2 route: %s", path)
		}
	}
	if len(got) != 4 {
		t.Fatalf("expected exactly 4 v2 routes, got %d: %v", len(got), got)
	}
}

// TestProviderTypesListsRegistryDescriptors checks the registry read.
func TestProviderTypesListsRegistryDescriptors(t *testing.T) {
	engine, _, _ := newFixtureRouter(t)
	code, rec := doGet(t, engine, "/api/v2/infra/provider-types")
	if code != http.StatusOK {
		t.Fatalf("status %d: %s", code, rec.Body.String())
	}
	body := decode(t, rec)
	items, ok := body["items"].([]any)
	if !ok || len(items) == 0 {
		t.Fatalf("expected registered provider types, got %v", body)
	}
	found := false
	for _, item := range items {
		row, _ := item.(map[string]any)
		if row["type"] == "kubernetes" {
			found = true
		}
	}
	if !found {
		t.Fatalf("kubernetes descriptor missing from %v", items)
	}
}

// TestProviderConnectionsRedactsSecrets asserts the claim-14 redaction
// surface: connection rows expose the SecretRef UID only — no ciphertext, no
// key material, no kubeconfig, no ConfigJSON dump.
func TestProviderConnectionsRedactsSecrets(t *testing.T) {
	engine, _, _ := newFixtureRouter(t)
	code, rec := doGet(t, engine, "/api/v2/infra/provider-connections")
	if code != http.StatusOK {
		t.Fatalf("status %d: %s", code, rec.Body.String())
	}
	raw := rec.Body.String()
	for _, banned := range []string{"Ciphertext", "ciphertext", "kubeconfig", "kubeConfig", "v2-envelope-ciphertext-marker", "configJson"} {
		if contains(raw, banned) {
			t.Fatalf("response leaks %q: %s", banned, raw)
		}
	}
	body := decode(t, rec)
	items := body["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("stale_source connection must be absent (§5.4c), got %d items: %s", len(items), raw)
	}
	row := items[0].(map[string]any)
	if row["uid"] != "conn-live" || row["secretRefUid"] != "secr-live" {
		t.Fatalf("unexpected connection row: %v", row)
	}
	if row["sourceModel"] != "" {
		t.Fatalf("backfill-sourced connection should carry sourceModel, got %v", row)
	}
}

// TestResourcesExcludeStaleAndFilterByKind covers the §5.4c join exclusion
// and the kind filter + paging contract (§3.8).
func TestResourcesExcludeStaleAndFilterByKind(t *testing.T) {
	engine, _, _ := newFixtureRouter(t)

	code, rec := doGet(t, engine, "/api/v2/infra/resources")
	if code != http.StatusOK {
		t.Fatalf("status %d: %s", code, rec.Body.String())
	}
	body := decode(t, rec)
	if total := body["total"].(float64); total != 2 {
		t.Fatalf("expected 2 live resources (stale excluded), got %v — %s", total, rec.Body.String())
	}

	code, rec = doGet(t, engine, "/api/v2/infra/resources?kind=orchestration.pod")
	if code != http.StatusOK {
		t.Fatalf("status %d: %s", code, rec.Body.String())
	}
	body = decode(t, rec)
	items := body["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["kind"] != "orchestration.pod" {
		t.Fatalf("kind filter broken: %s", rec.Body.String())
	}

	// Paging: page 1 with pageSize 1 must return the first row only and the
	// total stays the unfiltered (by kind) count.
	code, rec = doGet(t, engine, "/api/v2/infra/resources?page=1&pageSize=1")
	if code != http.StatusOK {
		t.Fatalf("status %d: %s", code, rec.Body.String())
	}
	body = decode(t, rec)
	if items := body["items"].([]any); len(items) != 1 {
		t.Fatalf("pageSize not applied: %s", rec.Body.String())
	}
	if body["pageSize"].(float64) != 1 {
		t.Fatalf("pageSize echo missing: %s", rec.Body.String())
	}
}

// TestResourceDetailCarriesLatestObservation covers the detail route: 200 for
// a live resource with the newest observation attached, 404 for unknown UID
// and for a stale-source resource (§5.4c — treated absent, not error-prone).
func TestResourceDetailCarriesLatestObservation(t *testing.T) {
	engine, liveUID, staleUID := newFixtureRouter(t)

	code, rec := doGet(t, engine, "/api/v2/infra/resources/"+liveUID)
	if code != http.StatusOK {
		t.Fatalf("status %d: %s", code, rec.Body.String())
	}
	body := decode(t, rec)
	resource := body["resource"].(map[string]any)
	if resource["uid"] != liveUID {
		t.Fatalf("unexpected resource: %v", resource)
	}
	observation, ok := body["observation"].(map[string]any)
	if !ok || observation["generationUid"] != "res-live" {
		t.Fatalf("latest observation missing: %s", rec.Body.String())
	}

	for _, uid := range []string{"res-unknown", staleUID} {
		code, rec = doGet(t, engine, "/api/v2/infra/resources/"+uid)
		if code != http.StatusNotFound {
			t.Fatalf("expected 404 for %s, got %d: %s", uid, code, rec.Body.String())
		}
	}
}

// TestResourcesFilterByKindPrefix covers the PR 30 family filter (plan J9 —
// M7): kindPrefix narrows the list to every kind under the prefix (the
// compute family read the ComputeInventory view rides), composes with the
// exact kind filter, and treats LIKE wildcards in the user input literally.
func TestResourcesFilterByKindPrefix(t *testing.T) {
	db := testutil.OpenMemoryDB(t)
	engine := newRouter(t, db)
	seedInfraFixture(t, db)
	var liveContext model.ProviderContext
	if err := db.Where("uid = ?", "ctx-live").First(&liveContext).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	for _, res := range []*model.InfraResource{
		{UID: "res-vm", ContextID: liveContext.ID, Kind: "compute.vm",
			ExternalURN: "urn:aliyun:mock-aliyun:compute.vm:i-bp1mock0001", DisplayName: "mock-vm-web-01",
			LifecycleState: "running", HealthState: "healthy", ManagedState: "discovered",
			FirstSeenAt: now, LastSeenAt: now},
		{UID: "res-volume", ContextID: liveContext.ID, Kind: "compute.volume",
			ExternalURN: "urn:aliyun:mock-aliyun:compute.volume:d-bp1mock0001", DisplayName: "mock-disk-01",
			FirstSeenAt: now, LastSeenAt: now},
	} {
		if err := db.Create(res).Error; err != nil {
			t.Fatal(err)
		}
	}

	cases := []struct {
		name      string
		query     string
		wantTotal int
	}{
		{"family prefix", "kindPrefix=compute.", 2},
		{"full kind as prefix", "kindPrefix=compute.vm", 1},
		{"no prefix lists everything", "kindPrefix=", 4},
		{"prefix composes with kind", "kindPrefix=compute.&kind=compute.volume", 1},
		{"LIKE wildcards are literal (no match, not everything)", "kindPrefix=compute.%25", 0},
		{"unknown prefix is empty", "kindPrefix=storage.", 0},
	}
	for _, tc := range cases {
		code, rec := doGet(t, engine, "/api/v2/infra/resources?"+tc.query)
		if code != http.StatusOK {
			t.Fatalf("%s: status %d: %s", tc.name, code, rec.Body.String())
		}
		body := decode(t, rec)
		if total := int(body["total"].(float64)); total != tc.wantTotal {
			t.Fatalf("%s: expected %d rows, got %d — %s", tc.name, tc.wantTotal, total, rec.Body.String())
		}
		for _, item := range body["items"].([]any) {
			kind := item.(map[string]any)["kind"].(string)
			if tc.query != "kindPrefix=" && !strings.HasPrefix(kind, "compute.") {
				t.Fatalf("%s: non-compute kind leaked through: %s", tc.name, kind)
			}
		}
	}
}

// TestStackFailureDegradesToUnavailable pins the nil-registry degradation:
// the process keeps booting and the registry read reports the outage instead
// of panicking.
func TestStackFailureDegradesToUnavailable(t *testing.T) {
	db := testutil.OpenMemoryDB(t)
	api := v2.NewInfraAPIWithRegistry(db, nil)
	engine := gin.New()
	api.Register(engine.Group("/api/v2/infra"))
	code, rec := doGet(t, engine, "/api/v2/infra/provider-types")
	if code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 from nil registry, got %d: %s", code, rec.Body.String())
	}
}

func contains(haystack, needle string) bool {
	return strings.Contains(haystack, needle)
}

// TestResourcesClampPagingBounds pins the paging clamps: non-positive pages
// floor at 1 and oversized pages cap at 100 (the default 20 stays untouched).
func TestResourcesClampPagingBounds(t *testing.T) {
	engine, _, _ := newFixtureRouter(t)

	code, rec := doGet(t, engine, "/api/v2/infra/resources?page=0&pageSize=500")
	if code != http.StatusOK {
		t.Fatalf("status %d: %s", code, rec.Body.String())
	}
	body := decode(t, rec)
	if body["page"].(float64) != 1 || body["pageSize"].(float64) != 100 {
		t.Fatalf("paging clamps broken: %s", rec.Body.String())
	}
}

// TestDBFailureReturnsServerError covers the infrastructure-error paths: with
// the database handle closed every read route degrades to a 500 instead of
// panicking or returning partial data.
func TestDBFailureReturnsServerError(t *testing.T) {
	db := testutil.OpenMemoryDB(t)
	gin.SetMode(gin.TestMode)
	if err := migrate.Run(context.Background(), db); err != nil {
		t.Fatalf("migrate.Run: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}
	engine := gin.New()
	v2.NewInfraAPIWithRegistry(db, registry.New()).Register(engine.Group("/api/v2/infra"))

	for _, path := range []string{
		"/api/v2/infra/provider-connections",
		"/api/v2/infra/resources",
		"/api/v2/infra/resources/res-live",
	} {
		code, rec := doGet(t, engine, path)
		if code != http.StatusInternalServerError {
			t.Fatalf("expected 500 on closed DB for %s, got %d: %s", path, code, rec.Body.String())
		}
	}
}
