package opdef

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"ops-admin/backend/model"
)

// snapshotEntry pins one pre-existing domains grant exactly as router.go
// declared it before the opdef migration.
type snapshotEntry struct {
	method      string
	path        string
	kind        string // "single" | "anyof" | "createedit"
	permissions []string
}

// domainDefSnapshot pins the 36 pre-existing domains permission grants exactly
// as they appeared in router.go at the r1 baseline (the former lines 52-87).
// It is the single regression baseline for the string migration (H-2): after
// the migration router.go holds no raw permission strings, so a router diff
// cannot serve as the baseline — this constant does. Changing a permission
// string in defs_domain.go must fail here.
var domainDefSnapshot = []snapshotEntry{
	{"GET", "/domain/public/accounts", "single", []string{"domains:account:list"}},
	{"GET", "/domain/public/accounts/options", "anyof", []string{"domains:account:list", "domains:public:list", "domains:ssl:view"}},
	{"GET", "/domain/public/accounts/detail", "single", []string{"domains:account:list"}},
	{"POST", "/domain/public/accounts/save", "createedit", []string{"domains:account:add", "domains:account:edit"}},
	{"DELETE", "/domain/public/accounts/delete", "single", []string{"domains:account:delete"}},
	{"POST", "/domain/public/accounts/test", "single", []string{"domains:account:test"}},
	{"GET", "/domain/public/domains", "single", []string{"domains:public:list"}},
	{"POST", "/domain/public/domains/sync", "single", []string{"domains:public:sync"}},
	{"GET", "/domain/public/records", "anyof", []string{"domains:public:list", "domains:public:record"}},
	{"POST", "/domain/public/records/mutate", "single", []string{"domains:public:record"}},
	{"POST", "/domain/public/records/batch", "single", []string{"domains:public:batch"}},
	{"GET", "/domain/internal/settings", "anyof", []string{"domains:settings:view", "domains:internal:list", "domains:query:test"}},
	{"PUT", "/domain/internal/settings", "single", []string{"domains:settings:save"}},
	{"GET", "/domain/internal/zones", "single", []string{"domains:internal:list"}},
	{"POST", "/domain/internal/zones/save", "createedit", []string{"domains:internal:zone:add", "domains:internal:zone:edit"}},
	{"DELETE", "/domain/internal/zones/delete", "single", []string{"domains:internal:zone:delete"}},
	{"GET", "/domain/internal/records", "anyof", []string{"domains:internal:list", "domains:internal:record"}},
	{"POST", "/domain/internal/records/save", "single", []string{"domains:internal:record"}},
	{"DELETE", "/domain/internal/records/delete", "single", []string{"domains:internal:record"}},
	{"POST", "/domain/internal/records/batch", "single", []string{"domains:internal:record"}},
	{"POST", "/domain/internal/query", "single", []string{"domains:query:test"}},
	{"GET", "/domain/audit", "single", []string{"domains:audit:list"}},
	{"GET", "/domain/public/certificates", "single", []string{"domains:ssl:view"}},
	{"GET", "/domain/public/certificates/detail", "single", []string{"domains:ssl:view"}},
	{"GET", "/domain/public/certificates/domain-options", "single", []string{"domains:ssl:view"}},
	{"GET", "/domain/public/certificates/tasks", "single", []string{"domains:ssl:view"}},
	{"GET", "/domain/public/certificates/audits", "single", []string{"domains:ssl:view"}},
	{"POST", "/domain/public/certificates/sync", "single", []string{"domains:ssl:sync"}},
	{"POST", "/domain/public/certificates/upload", "single", []string{"domains:ssl:upload"}},
	{"POST", "/domain/public/certificates/apply", "single", []string{"domains:ssl:apply"}},
	{"POST", "/domain/public/certificates/renew", "single", []string{"domains:ssl:renew"}},
	{"POST", "/domain/public/certificates/resync", "single", []string{"domains:ssl:sync"}},
	{"PUT", "/domain/public/certificates/renew-settings", "single", []string{"domains:ssl:settings"}},
	{"DELETE", "/domain/public/certificates/delete", "single", []string{"domains:ssl:delete"}},
	{"GET", "/domain/public/certificates/download", "single", []string{"domains:ssl:download"}},
	{"GET", "/domain/public/certificates/download-private", "single", []string{"domains:ssl:download-key"}},
}

// defKind reports which of the three permission shapes a definition uses.
func defKind(d Def) string {
	switch {
	case d.Permission != "":
		return "single"
	case len(d.AnyOf) > 0:
		return "anyof"
	default:
		return "createedit"
	}
}

// TestUniqueMethodPath guards the (Method, Path) uniqueness invariant of the
// operation table — the replay matrix and the router attach both key on it.
func TestUniqueMethodPath(t *testing.T) {
	seen := map[string]string{}
	for _, d := range All() {
		key := d.Method + " " + d.Path
		if prev, dup := seen[key]; dup {
			t.Fatalf("duplicate (Method,Path) %q (first seen as %q)", key, prev)
		}
		seen[key] = key
	}
}

// TestExactlyOnePermissionSpec enforces the mutual exclusivity of the three
// permission shapes on the whole table and on unit-level invalid definitions.
func TestExactlyOnePermissionSpec(t *testing.T) {
	for _, d := range All() {
		specs := 0
		if d.Permission != "" {
			specs++
		}
		if len(d.AnyOf) > 0 {
			specs++
		}
		if d.CreateEdit != [2]string{} {
			specs++
		}
		if specs != 1 {
			t.Fatalf("%s %s: exactly one of Permission/AnyOf/CreateEdit must be set, got %d", d.Method, d.Path, specs)
		}
		if err := Validate(d); err != nil {
			t.Fatalf("Validate(%s %s): %v", d.Method, d.Path, err)
		}
	}

	empty := Def{Method: http.MethodGet, Path: "/unit", Risk: RiskLow}
	if err := Validate(empty); err == nil {
		t.Fatal("Validate must reject a definition without any permission")
	}
	both := Def{Method: http.MethodGet, Path: "/unit", Permission: "t5:first", AnyOf: []string{"t5:second"}, Risk: RiskLow}
	if err := Validate(both); err == nil {
		t.Fatal("Validate must reject a definition with Permission and AnyOf together")
	}
	all := Def{Method: http.MethodGet, Path: "/unit", Permission: "t5:first", AnyOf: []string{"t5:second"}, CreateEdit: [2]string{"t5:third", "t5:fourth"}, Risk: RiskLow}
	if err := Validate(all); err == nil {
		t.Fatal("Validate must reject a definition with all three permission shapes")
	}
}

// TestPermissionStringFormat enforces the permission vocabulary pattern over
// the whole table and rejects malformed strings on unit level.
func TestPermissionStringFormat(t *testing.T) {
	for _, d := range All() {
		for _, p := range permissionStrings(d) {
			if !permissionPattern.MatchString(p) {
				t.Fatalf("%s %s: permission %q violates the vocabulary pattern", d.Method, d.Path, p)
			}
		}
	}
	for _, malformed := range []string{"", "Domains:account:list", "a", "a:b:c:d:e", "1domain:list", "a::b", "a:B:x", ":b"} {
		if permissionPattern.MatchString(malformed) {
			t.Fatalf("permissionPattern must reject %q", malformed)
		}
	}
	for _, valid := range []string{"domains:account:list", "assets:database:sql:execute", "integration:ai:tool:execute", "monitor:query"} {
		if !permissionPattern.MatchString(valid) {
			t.Fatalf("permissionPattern must accept %q", valid)
		}
	}
}

// TestMutatingFlagMatchesMethod enforces Method != GET <=> Mutating on the
// whole table and rejects a GET flagged mutating on unit level. Risk must stay
// inside the low/medium/high enum.
func TestMutatingFlagMatchesMethod(t *testing.T) {
	for _, d := range All() {
		if d.Mutating != (d.Method != http.MethodGet) {
			t.Fatalf("%s %s: Mutating=%v must equal (Method != GET)", d.Method, d.Path, d.Mutating)
		}
		switch d.Risk {
		case RiskLow, RiskMedium, RiskHigh:
		default:
			t.Fatalf("%s %s: Risk %q must be one of low|medium|high", d.Method, d.Path, d.Risk)
		}
	}
	if err := Validate(Def{Method: http.MethodGet, Path: "/unit", Permission: "t5:first", Mutating: true, Risk: RiskLow}); err == nil {
		t.Fatal("Validate must reject a GET definition flagged Mutating")
	}
	if err := Validate(Def{Method: http.MethodGet, Path: "/unit", Permission: "t5:first", Risk: "critical"}); err == nil {
		t.Fatal("Validate must reject a Risk outside the enum")
	}
}

// TestDomainDefsSnapshot proves the domains migration stayed byte-identical
// (claim 8, H-2): every snapshot entry matches defs_domain.go in shape and
// permission strings, and no extra domains definition exists.
func TestDomainDefsSnapshot(t *testing.T) {
	byKey := map[string]Def{}
	for _, d := range domainDefs {
		byKey[d.Method+" "+d.Path] = d
	}
	for _, s := range domainDefSnapshot {
		d, ok := byKey[s.method+" "+s.path]
		if !ok {
			t.Fatalf("snapshot %s %s missing from domainDefs", s.method, s.path)
		}
		if got := defKind(d); got != s.kind {
			t.Fatalf("%s %s: snapshot kind %s, table kind %s", s.method, s.path, s.kind, got)
		}
		if !slices.Equal(permissionStrings(d), s.permissions) {
			t.Fatalf("%s %s: permissions drifted: table %v, snapshot %v", s.method, s.path, permissionStrings(d), s.permissions)
		}
	}
	if len(domainDefs) != len(domainDefSnapshot) {
		t.Fatalf("domainDefs holds %d entries, snapshot pins %d", len(domainDefs), len(domainDefSnapshot))
	}
}

// newOpdefTestDB opens a single-connection in-memory sqlite database with the
// grant models migrated (test-only dependency, T1 constraint inherited).
func newOpdefTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(&model.Menu{}, &model.RoleMenu{}, &model.AdminRole{}); err != nil {
		t.Fatal(err)
	}
	return db
}

// seedGrantMenus creates type-3 status-1 menus for the given permission values
// — the same rows the permission query joins on.
func seedGrantMenus(t *testing.T, db *gorm.DB, values ...string) {
	t.Helper()
	for _, value := range values {
		menu := model.Menu{MenuName: value, Value: value, MenuType: 3, MenuStatus: 1}
		if err := db.Create(&menu).Error; err != nil {
			t.Fatal(err)
		}
	}
}

// grantPermissions gives adminID one role holding the menus of the given
// permission values, mirroring the sys_admin_role / sys_role_menu join.
func grantPermissions(t *testing.T, db *gorm.DB, adminID uint, roleID uint, values ...string) {
	t.Helper()
	if err := db.Create(&model.AdminRole{AdminID: adminID, RoleID: roleID}).Error; err != nil {
		t.Fatal(err)
	}
	for _, value := range values {
		var menu model.Menu
		if err := db.Where("value = ?", value).First(&menu).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Create(&model.RoleMenu{RoleID: roleID, MenuID: menu.ID}).Error; err != nil {
			t.Fatal(err)
		}
	}
}

// runMiddleware attaches Middleware(db, def) to a test context with the given
// body and userID, and returns 200 when the chain was allowed to continue or
// the recorder status when it aborted.
func runMiddleware(t *testing.T, db *gorm.DB, d Def, method string, body []byte, userID uint) int {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, "/target", bytes.NewReader(body))
	c.Set("userID", userID)
	Middleware(db, d)(c)
	if c.IsAborted() {
		return w.Code
	}
	return http.StatusOK
}

// TestMiddlewareSelection exercises the three permission shapes through the
// real middleware stack and pins the CreateEdit payload-ID branch directly —
// the compensation for excluding the edit branch from the replay matrix (A10).
func TestMiddlewareSelection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newOpdefTestDB(t)
	const (
		createPerm  = "t5:create"
		editPerm    = "t5:edit"
		sharedRead  = "t5:any"
		unknownPerm = "t5:other"
	)
	seedGrantMenus(t, db, createPerm, editPerm, sharedRead, unknownPerm)
	const (
		adminID = 7
		roleID  = 1
	)
	grantPermissions(t, db, adminID, roleID, createPerm, sharedRead)

	singleAllowed := Def{Method: http.MethodGet, Path: "/unit", Permission: createPerm, Risk: RiskLow}
	if got := runMiddleware(t, db, singleAllowed, http.MethodGet, nil, adminID); got != http.StatusOK {
		t.Fatalf("single shape: granted permission must pass, got %d", got)
	}
	singleDenied := Def{Method: http.MethodGet, Path: "/unit", Permission: unknownPerm, Risk: RiskLow}
	if got := runMiddleware(t, db, singleDenied, http.MethodGet, nil, adminID); got != http.StatusForbidden {
		t.Fatalf("single shape: ungranted permission must be 403, got %d", got)
	}

	anyAllowed := Def{Method: http.MethodGet, Path: "/unit", AnyOf: []string{sharedRead, unknownPerm}, Risk: RiskLow}
	if got := runMiddleware(t, db, anyAllowed, http.MethodGet, nil, adminID); got != http.StatusOK {
		t.Fatalf("anyof shape: one matching grant must pass, got %d", got)
	}
	anyDenied := Def{Method: http.MethodGet, Path: "/unit", AnyOf: []string{editPerm, unknownPerm}, Risk: RiskLow}
	if got := runMiddleware(t, db, anyDenied, http.MethodGet, nil, adminID); got != http.StatusForbidden {
		t.Fatalf("anyof shape: no matching grant must be 403, got %d", got)
	}

	createEdit := Def{Method: http.MethodPost, Path: "/unit", CreateEdit: [2]string{createPerm, editPerm}, Mutating: true, Risk: RiskMedium}
	if got := runMiddleware(t, db, createEdit, http.MethodPost, []byte(`{}`), adminID); got != http.StatusOK {
		t.Fatalf("createedit shape: id-less payload takes the create branch, got %d", got)
	}
	if got := runMiddleware(t, db, createEdit, http.MethodPost, []byte(`{"id":3}`), adminID); got != http.StatusForbidden {
		t.Fatalf("createedit shape: id-bearing payload takes the edit branch (edit not granted), got %d", got)
	}
	var menu model.Menu
	if err := db.Where("value = ?", editPerm).First(&menu).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.RoleMenu{RoleID: roleID, MenuID: menu.ID}).Error; err != nil {
		t.Fatal(err)
	}
	if got := runMiddleware(t, db, createEdit, http.MethodPost, []byte(`{"id":3}`), adminID); got != http.StatusOK {
		t.Fatalf("createedit shape: edit grant must satisfy the id-bearing branch, got %d", got)
	}

	unauthenticated := Def{Method: http.MethodGet, Path: "/unit", Permission: createPerm, Risk: RiskLow}
	if got := runMiddleware(t, db, unauthenticated, http.MethodGet, nil, 0); got != http.StatusForbidden {
		t.Fatalf("no admin-role row must be 403, got %d", got)
	}
}
