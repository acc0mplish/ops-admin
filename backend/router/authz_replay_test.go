package router

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"ops-admin/backend/auth"
	"ops-admin/backend/model"
	"ops-admin/backend/opdef"
)

// apiPrefix is the engine-level prefix of every authGroup route.
const apiPrefix = "/api/v1"

// publicRouteKeys are the 7 public routes of the api group (A9): the three
// authentication calls happen before any authorization can exist, the public
// config and integration reads are unauthenticated by design, and the two
// websocket endpoints belong to the console-ticket task. Plus the ping route
// and the static uploads pair registered outside the api group.
var publicRouteKeys = map[string]struct{}{
	"POST " + apiPrefix + "/login":                    {},
	"POST " + apiPrefix + "/auth/refresh":             {},
	"POST " + apiPrefix + "/auth/logout":              {},
	"GET " + apiPrefix + "/systemConfig/public":       {},
	"GET " + apiPrefix + "/integration/public/:token": {},
	"GET " + apiPrefix + "/asset/terminal/ws":         {},
	"GET " + apiPrefix + "/k8s/pod/terminal/ws":       {},
	"GET /ping":               {},
	"GET /uploads/*filepath":  {},
	"HEAD /uploads/*filepath": {},
}

// authGroupRoutes filters the live engine's route table down to the 425
// authenticated routes: the api-group routes minus the public set.
func authGroupRoutes(routes gin.RoutesInfo) []gin.RouteInfo {
	out := make([]gin.RouteInfo, 0, 425)
	for _, route := range routes {
		if !strings.HasPrefix(route.Path, apiPrefix+"/") && route.Path != apiPrefix {
			continue
		}
		if _, public := publicRouteKeys[route.Method+" "+route.Path]; public {
			continue
		}
		out = append(out, route)
	}
	return out
}

// replaySession mints an access token plus the matching AuthSession row for a
// synthetic admin, the same state the login flow leaves behind.
func replaySession(t *testing.T, db *gorm.DB, adminID uint, username string) string {
	t.Helper()
	sessionID := fmt.Sprintf("replay-session-%d", adminID)
	session := model.AuthSession{
		ID:               sessionID,
		AdminID:          adminID,
		RefreshTokenHash: fmt.Sprintf("replay-refresh-hash-%d", adminID),
		LastActivityAt:   time.Now(),
		ExpiresAt:        time.Now().Add(time.Hour),
	}
	if err := db.Create(&session).Error; err != nil {
		t.Fatal(err)
	}
	token, _, err := auth.GenerateToken(adminID, username, sessionID)
	if err != nil {
		t.Fatal(err)
	}
	return token
}

// replayRole creates a plain role; copyGrants decides whether it starts from
// the pre-migration full grant set.
func replayRole(t *testing.T, db *gorm.DB, roleKey string) model.Role {
	t.Helper()
	role := model.Role{RoleName: roleKey, RoleKey: roleKey, Status: 1, Description: "authz replay role", CreatedAt: time.Now()}
	if err := db.Create(&role).Error; err != nil {
		t.Fatal(err)
	}
	return role
}

// copyRoleGrants clones every sys_role_menu row of the source role onto the
// target role — the explicit procedure that re-creates a role that existed
// before the migration and therefore held every permission.
func copyRoleGrants(t *testing.T, db *gorm.DB, targetRoleID uint, sourceRoleID uint) int {
	t.Helper()
	var grants []model.RoleMenu
	if err := db.Where("role_id = ?", sourceRoleID).Find(&grants).Error; err != nil {
		t.Fatal(err)
	}
	copied := 0
	for _, grant := range grants {
		row := model.RoleMenu{RoleID: targetRoleID, MenuID: grant.MenuID}
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
		copied++
	}
	return copied
}

// replayAdminRole wires a synthetic admin to a role through sys_admin_role.
func replayAdminRole(t *testing.T, db *gorm.DB, adminID uint, roleID uint) {
	t.Helper()
	if err := db.Create(&model.AdminRole{AdminID: adminID, RoleID: roleID}).Error; err != nil {
		t.Fatal(err)
	}
}

// doRequest issues one request against the live engine.
func doRequest(engine *gin.Engine, token string, method string, path string, body []byte) (int, string) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	return w.Code, w.Body.String()
}

// requestBody follows the replay payload strategy: an empty JSON object for
// the body-bearing methods so binding/validation rejects early (A3), nothing
// otherwise. The CreateEdit edit branch (id > 0) is excluded from the matrix
// per A10 — its selection logic is verified directly by opdef's middleware
// test, and the replay role holds both strings anyway.
func requestBody(method string) []byte {
	if method == http.MethodPost || method == http.MethodPut {
		return []byte("{}")
	}
	return nil
}

// isPermissionDenied recognizes the exact deny marker of the permission
// middleware: HTTP 403 with the AUTH_PERMISSION_DENIED error code that
// httpx.Failed(403, "Permission denied") is mapped to by the apperr layer.
func isPermissionDenied(status int, body string) bool {
	return status == http.StatusForbidden && strings.Contains(body, "AUTH_PERMISSION_DENIED")
}

// TestPermissionReplay is T14 (G-4): a role holding the full pre-migration
// grant set must keep access to every sensitive route after the enforcement
// was attached — the replay matrix asserts the "before == after" sentence of
// §4.7 step 4. Any 403 permission denial on the matrix is a migration bug.
func TestPermissionReplay(t *testing.T) {
	engine, db := newArtifactEngine(t)
	super := replayFindRole(t, db, "super-admin")
	replayRoleRow := replayRole(t, db, "authz-replay-full")
	if copied := copyRoleGrants(t, db, replayRoleRow.ID, super.ID); copied == 0 {
		t.Fatal("grant copy produced zero rows — replay baseline is empty")
	}
	replayAdminRole(t, db, 9001, replayRoleRow.ID)
	token := replaySession(t, db, 9001, "replay-admin")

	defs := opdef.All()
	denied := []string{}
	for _, d := range defs {
		status, body := doRequest(engine, token, d.Method, apiPrefix+d.Path, requestBody(d.Method))
		if status == http.StatusUnauthorized {
			t.Fatalf("replay %s %s returned 401 — session setup is broken", d.Method, d.Path)
		}
		if isPermissionDenied(status, body) {
			denied = append(denied, fmt.Sprintf("%s %s -> %d", d.Method, d.Path, status))
		}
	}
	if len(denied) > 0 {
		t.Fatalf("replay matrix: %d of %d sensitive routes denied to the pre-migration role (must be 0): %v", len(denied), len(defs), denied)
	}
}

// replayFindRole is the replay-side role lookup.
func replayFindRole(t *testing.T, db *gorm.DB, roleKey string) model.Role {
	t.Helper()
	var role model.Role
	if err := db.Where("role_key = ?", roleKey).First(&role).Error; err != nil {
		t.Fatal(err)
	}
	return role
}

// TestZeroGrantCoverage is T15 (S3, the behavioural attach-rate oracle): a
// role with zero grants must be denied on exactly the sensitive set and must
// not hit the permission marker anywhere else. It detects both missing
// attachments and over-attachments on the whole authGroup surface.
func TestZeroGrantCoverage(t *testing.T) {
	engine, db := newArtifactEngine(t)
	zeroRoleRow := replayRole(t, db, "authz-zero-grants")
	replayAdminRole(t, db, 9002, zeroRoleRow.ID)
	token := replaySession(t, db, 9002, "zero-admin")

	defKeys := map[string]struct{}{}
	for _, d := range opdef.All() {
		defKeys[d.Method+" "+d.Path] = struct{}{}
	}
	authRoutes := authGroupRoutes(engine.Routes())
	if len(authRoutes) != 425 {
		t.Fatalf("authGroup holds %d routes, contract is 425", len(authRoutes))
	}
	missedDenials := []string{}
	unexpectedDenials := []string{}
	for _, route := range authRoutes {
		status, body := doRequest(engine, token, route.Method, route.Path, requestBody(route.Method))
		denied := isPermissionDenied(status, body)
		_, sensitive := defKeys[route.Method+" "+strings.TrimPrefix(route.Path, apiPrefix)]
		if sensitive && !denied {
			missedDenials = append(missedDenials, fmt.Sprintf("%s %s -> %d", route.Method, route.Path, status))
		}
		if !sensitive && denied {
			unexpectedDenials = append(unexpectedDenials, fmt.Sprintf("%s %s -> %d", route.Method, route.Path, status))
		}
	}
	if len(missedDenials) > 0 {
		t.Fatalf("zero-grant scan: %d sensitive routes not enforcing a permission (must be 0): %v", len(missedDenials), missedDenials)
	}
	if len(unexpectedDenials) > 0 {
		t.Fatalf("zero-grant scan: %d non-sensitive routes answered with the permission marker (must be 0): %v", len(unexpectedDenials), unexpectedDenials)
	}
}

// TestUnauthenticatedSamplesRejected is T16: requests without a token stay
// rejected with 401 on both the sensitive and the normal surface.
func TestUnauthenticatedSamplesRejected(t *testing.T) {
	engine, _ := newArtifactEngine(t)
	samples := []struct{ method, path string }{
		{http.MethodGet, apiPrefix + "/asset/host/list"},
		{http.MethodPost, apiPrefix + "/dbms/sql/execute"},
		{http.MethodDelete, apiPrefix + "/admin/delete"},
		{http.MethodGet, apiPrefix + "/profile"},
		{http.MethodGet, apiPrefix + "/role/list"},
	}
	for _, sample := range samples {
		status, _ := doRequest(engine, "", sample.method, sample.path, requestBody(sample.method))
		if status != http.StatusUnauthorized {
			t.Fatalf("unauthenticated %s %s returned %d, want 401", sample.method, sample.path, status)
		}
	}
}

// TestLoginSmoke is claim 13: the seeded administrator can still log in and
// read the profile through the real endpoints — the migration left the
// unauthenticated login path and the authenticated normal reads intact.
func TestLoginSmoke(t *testing.T) {
	engine, _ := newArtifactEngine(t)
	status, body := doRequest(engine, "", http.MethodPost, apiPrefix+"/login", []byte(`{"username":"admin","password":"inventory-test-password"}`))
	if status != http.StatusOK {
		t.Fatalf("login returned %d, want 200: %s", status, body)
	}
	var payload struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &payload); err != nil || payload.Data.Token == "" {
		t.Fatalf("login response carries no token: %v %s", err, body)
	}
	status, _ = doRequest(engine, payload.Data.Token, http.MethodGet, apiPrefix+"/profile", nil)
	if status != http.StatusOK {
		t.Fatalf("profile after login returned %d, want 200", status)
	}
}

// TestOperationTableCoversRouter is T17 (CR-3): every operation definition
// exists on the live router, the committed sensitive artifact holds exactly
// the table, and — the reverse assertion — every non-GET authGroup route has
// an operation definition, so a future mutation route added without one fails
// here.
func TestOperationTableCoversRouter(t *testing.T) {
	engine, _ := newArtifactEngine(t)
	routeKeys := map[string]struct{}{}
	for _, route := range engine.Routes() {
		routeKeys[route.Method+" "+route.Path] = struct{}{}
	}
	defs := opdef.All()
	defKeys := map[string]struct{}{}
	for _, d := range defs {
		key := d.Method + " " + apiPrefix + d.Path
		if _, ok := routeKeys[key]; !ok {
			t.Fatalf("operation definition %s %s has no live route", d.Method, d.Path)
		}
		defKeys[d.Method+" "+d.Path] = struct{}{}
	}

	artifactPath := filepath.Join("..", "..", "docs", "security", "sensitive-routes.txt")
	raw, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatal(err)
	}
	artifactLines := 0
	for _, line := range strings.Split(strings.TrimRight(string(raw), "\n"), "\n") {
		if strings.HasPrefix(line, "#") {
			continue
		}
		artifactLines++
	}
	if artifactLines != len(defs) {
		t.Fatalf("sensitive-routes.txt holds %d entries, operation table has %d", artifactLines, len(defs))
	}

	missing := []string{}
	count := 0
	for _, route := range authGroupRoutes(engine.Routes()) {
		if route.Method == http.MethodGet {
			continue
		}
		count++
		if _, ok := defKeys[route.Method+" "+strings.TrimPrefix(route.Path, apiPrefix)]; !ok {
			missing = append(missing, route.Method+" "+route.Path)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("%d non-GET authGroup routes have no operation definition (must be 0): %v", len(missing), missing)
	}
	if count != 238 {
		t.Fatalf("authGroup non-GET count %d drifted from the 238 baseline", count)
	}
}
