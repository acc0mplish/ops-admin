package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"

	"ops-admin/backend/auth"
	"ops-admin/backend/model"
)

// assetTerminalWSPath and k8sTerminalWSPath are the two public websocket
// endpoints under test.
const (
	assetTerminalWSPath = apiPrefix + "/asset/terminal/ws"
	k8sTerminalWSPath   = apiPrefix + "/k8s/pod/terminal/ws"
)

// mintConsoleTicket drives the real POST /console-sessions endpoint and
// returns the minted ticket value.
func mintConsoleTicket(t *testing.T, engine *gin.Engine, token string, resourceType string, resourceID string, protocol string) string {
	t.Helper()
	status, body := doRequest(engine, token, http.MethodPost, apiPrefix+"/console-sessions",
		[]byte(`{"resourceType":"`+resourceType+`","resourceId":"`+resourceID+`","protocol":"`+protocol+`"}`))
	if status != http.StatusOK {
		t.Fatalf("mint %s %s returned %d: %s", resourceType, resourceID, status, body)
	}
	var payload struct {
		Data struct {
			Ticket    string `json:"ticket"`
			ExpiresIn int    `json:"expiresIn"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &payload); err != nil || payload.Data.Ticket == "" {
		t.Fatalf("mint response carries no ticket: %v %s", err, body)
	}
	if payload.Data.ExpiresIn != 30 {
		t.Fatalf("mint expiresIn %d, want 30", payload.Data.ExpiresIn)
	}
	return payload.Data.Ticket
}

// grantSinglePermission wires adminID to a role holding exactly one granted
// permission menu — the M-1 recheck fixture.
func grantSinglePermission(t *testing.T, db *gorm.DB, adminID uint, permission string) {
	t.Helper()
	var menu model.Menu
	if err := db.Where("value = ?", permission).First(&menu).Error; err != nil {
		t.Fatalf("seeded permission menu %q missing: %v", permission, err)
	}
	role := replayRole(t, db, "only-"+permission)
	if err := db.Create(&model.RoleMenu{RoleID: role.ID, MenuID: menu.ID}).Error; err != nil {
		t.Fatal(err)
	}
	replayAdminRole(t, db, adminID, role.ID)
}

// dialTerminalWS dials one terminal websocket endpoint against a live server;
// on a failed handshake the returned response still carries the HTTP status.
func dialTerminalWS(t *testing.T, engine *gin.Engine, path string, query string) (*http.Response, *websocket.Conn, error) {
	t.Helper()
	server := httptest.NewServer(engine)
	t.Cleanup(server.Close)
	conn, resp, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+path+"?"+query, nil)
	return resp, conn, err
}

// legacyToken mints a syntactically valid access token. Since Release C the
// websocket gate accepts no query token, so dials presenting only this token
// must be refused with 401.
func legacyToken(t *testing.T) string {
	t.Helper()
	token, _, err := auth.GenerateToken(4242, "legacy-ws-user", "legacy-ws-session")
	if err != nil {
		t.Fatal(err)
	}
	return token
}

// TestConsoleSessionMintTicketRoundtrip is T1/T7 at the router level: a
// terminal-permitted admin mints tickets through the real endpoint and the
// websocket gate accepts them — asset evidence is the 400 of the service
// miss behind the gate (no live SSH host exists in tests), k8s evidence is
// the 101 itself because the upgrade precedes the service call.
func TestConsoleSessionMintTicketRoundtrip(t *testing.T) {
	engine, db := newArtifactEngine(t)
	admin := replayRole(t, db, "console-roundtrip")
	if copied := copyRoleGrants(t, db, admin.ID, replayFindRole(t, db, "super-admin").ID); copied == 0 {
		t.Fatal("grant copy produced zero rows")
	}
	replayAdminRole(t, db, 9101, admin.ID)
	token := replaySession(t, db, 9101, "console-roundtrip-admin")

	assetTicket := mintConsoleTicket(t, engine, token, "asset_host", "999999", "asset-terminal")
	resp, _, err := dialTerminalWS(t, engine, assetTerminalWSPath, "ticket="+assetTicket+"&hostId=999999")
	if err == nil || resp == nil || resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("asset ticket path must pass the gate and fail in the service with 400, got resp=%v err=%v", resp, err)
	}

	k8sTicket := mintConsoleTicket(t, engine, token, "k8s_pod", "3/default/pod-x", "k8s-pod-terminal")
	resp, conn, err := dialTerminalWS(t, engine, k8sTerminalWSPath, "ticket="+k8sTicket+"&clusterId=3&namespace=default&podName=pod-x")
	if err != nil || resp == nil || resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("k8s ticket path must reach the 101 upgrade, got resp=%v err=%v", resp, err)
	}
	_ = conn.Close()
}

// TestConsoleSessionPermissions is T8: the AnyOf gate denies zero-grant
// roles, and the handler re-verifies the resource-specific permission so a
// pod-only role cannot mint host tickets.
func TestConsoleSessionPermissions(t *testing.T) {
	engine, db := newArtifactEngine(t)

	zeroToken := replaySession(t, db, 9102, "zero-console-admin")

	podOnlyAdmin := uint(9103)
	grantSinglePermission(t, db, podOnlyAdmin, "assets:k8s:pod:terminal")
	podOnlyToken := replaySession(t, db, podOnlyAdmin, "pod-only-admin")

	status, body := doRequest(engine, zeroToken, http.MethodPost, apiPrefix+"/console-sessions",
		[]byte(`{"resourceType":"asset_host","resourceId":"1","protocol":"asset-terminal"}`))
	if !isPermissionDenied(status, body) {
		t.Fatalf("zero-grant mint must be 403, got %d: %s", status, body)
	}

	status, body = doRequest(engine, podOnlyToken, http.MethodPost, apiPrefix+"/console-sessions",
		[]byte(`{"resourceType":"asset_host","resourceId":"1","protocol":"asset-terminal"}`))
	if !isPermissionDenied(status, body) {
		t.Fatalf("pod-only admin minting a host ticket must be 403 (M-1), got %d: %s", status, body)
	}

	status, _ = doRequest(engine, podOnlyToken, http.MethodPost, apiPrefix+"/console-sessions",
		[]byte(`{"resourceType":"k8s_pod","resourceId":"3/default/pod-x","protocol":"k8s-pod-terminal"}`))
	if status != http.StatusOK {
		t.Fatalf("pod-only admin minting a pod ticket returned %d, want 200", status)
	}
}

// TestConsoleSessionInvalidPayload rejects unknown resource types and foreign
// protocol pairs with 400 before any permission lookup.
func TestConsoleSessionInvalidPayload(t *testing.T) {
	engine, db := newArtifactEngine(t)
	role := replayRole(t, db, "console-invalid-payload")
	if copied := copyRoleGrants(t, db, role.ID, replayFindRole(t, db, "super-admin").ID); copied == 0 {
		t.Fatal("grant copy produced zero rows")
	}
	replayAdminRole(t, db, 9104, role.ID)
	token := replaySession(t, db, 9104, "console-invalid-admin")

	cases := []string{
		`{"resourceType":"serverless_fn","resourceId":"1","protocol":"asset-terminal"}`,
		`{"resourceType":"asset_host","resourceId":"1","protocol":"k8s-pod-terminal"}`,
		`{"resourceType":"asset_host","resourceId":"-1","protocol":"asset-terminal"}`,
		`{"resourceType":"k8s_pod","resourceId":"3/default","protocol":"k8s-pod-terminal"}`,
	}
	for _, body := range cases {
		status, respBody := doRequest(engine, token, http.MethodPost, apiPrefix+"/console-sessions", []byte(body))
		if status != http.StatusBadRequest {
			t.Fatalf("payload %s returned %d, want 400: %s", body, status, respBody)
		}
	}
}

// TestTerminalWSTicketMismatchRejected is T4/T5: a valid ticket bound to one
// resource (or one protocol) is refused with 403 on any other endpoint or
// resource key.
func TestTerminalWSTicketMismatchRejected(t *testing.T) {
	engine, db := newArtifactEngine(t)
	role := replayRole(t, db, "console-mismatch")
	if copied := copyRoleGrants(t, db, role.ID, replayFindRole(t, db, "super-admin").ID); copied == 0 {
		t.Fatal("grant copy produced zero rows")
	}
	replayAdminRole(t, db, 9105, role.ID)
	token := replaySession(t, db, 9105, "console-mismatch-admin")

	hostTicket := mintConsoleTicket(t, engine, token, "asset_host", "12", "asset-terminal")

	resp, _, err := dialTerminalWS(t, engine, assetTerminalWSPath, "ticket="+hostTicket+"&hostId=13")
	if err == nil || resp == nil || resp.StatusCode != http.StatusForbidden {
		t.Fatalf("host ticket on another host must be 403, got resp=%v err=%v", resp, err)
	}

	resp, _, err = dialTerminalWS(t, engine, k8sTerminalWSPath, "ticket="+hostTicket+"&clusterId=3&namespace=default&podName=pod-x")
	if err == nil || resp == nil || resp.StatusCode != http.StatusForbidden {
		t.Fatalf("asset ticket on the k8s endpoint must be 403, got resp=%v err=%v", resp, err)
	}
}

// TestTerminalWSTicketNeverFallsBack is the A-1/R-5 negative proof: with no
// fallback left in the gate, a present but invalid ticket is rejected with
// 401 immediately — a valid access token riding along buys nothing.
func TestTerminalWSTicketNeverFallsBack(t *testing.T) {
	engine, _ := newArtifactEngine(t)
	token := legacyToken(t)

	resp, _, err := dialTerminalWS(t, engine, assetTerminalWSPath, "ticket=bogus-ticket&token="+token+"&hostId=12")
	if err == nil || resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("invalid ticket must be 401 without legacy fallback, got resp=%v err=%v", resp, err)
	}

	resp, _, err = dialTerminalWS(t, engine, assetTerminalWSPath, "ticket=bogus-ticket&hostId=12")
	if err == nil || resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("invalid ticket alone must be 401, got resp=%v err=%v", resp, err)
	}
}

// TestTerminalWSLegacyTokenPath is T6 at Release C: the legacy query-token
// path is gone — a token-only dial on either endpoint is refused with 401,
// and so is a dial with no credential at all. The one-time ticket is the
// only way through the gate.
func TestTerminalWSLegacyTokenPath(t *testing.T) {
	engine, _ := newArtifactEngine(t)
	token := legacyToken(t)

	resp, _, err := dialTerminalWS(t, engine, assetTerminalWSPath, "token="+token+"&hostId=12")
	if err == nil || resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("legacy token-only asset dial must be 401, got resp=%v err=%v", resp, err)
	}

	resp, _, err = dialTerminalWS(t, engine, k8sTerminalWSPath, "token="+token+"&clusterId=3&namespace=default&podName=pod-x")
	if err == nil || resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("legacy token-only k8s dial must be 401, got resp=%v err=%v", resp, err)
	}

	resp, _, err = dialTerminalWS(t, engine, assetTerminalWSPath, "hostId=12")
	if err == nil || resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("dial without any credential must be 401, got resp=%v err=%v", resp, err)
	}
}

// TestDiagnosisRunGoneAndPostReplacement is T9: the retired GET answers 410
// with the replacement pointers, the POST route reaches the handler, and a
// fully-granted role is not permission-denied on either.
func TestDiagnosisRunGoneAndPostReplacement(t *testing.T) {
	engine, db := newArtifactEngine(t)
	role := replayRole(t, db, "diagnosis-run")
	if copied := copyRoleGrants(t, db, role.ID, replayFindRole(t, db, "super-admin").ID); copied == 0 {
		t.Fatal("grant copy produced zero rows")
	}
	replayAdminRole(t, db, 9106, role.ID)
	token := replaySession(t, db, 9106, "diagnosis-run-admin")

	status, body := doRequest(engine, token, http.MethodGet, apiPrefix+"/asset/service/diagnosis/run", nil)
	if status != http.StatusGone {
		t.Fatalf("GET diagnosis/run returned %d, want 410: %s", status, body)
	}
	// Headers are read from a recorder replay of the same request because
	// doRequest only returns status and body.
	req := httptest.NewRequest(http.MethodGet, apiPrefix+"/asset/service/diagnosis/run", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if allow := w.Header().Get("Allow"); allow != http.MethodPost {
		t.Fatalf("410 must advertise Allow: POST, got %q", allow)
	}
	if loc := w.Header().Get("Location"); loc != "/api/v1/asset/service/diagnosis/run" {
		t.Fatalf("410 must advertise the replacement path, got %q", loc)
	}

	status, body = doRequest(engine, token, http.MethodPost, apiPrefix+"/asset/service/diagnosis/run", []byte(`{}`))
	if status != http.StatusBadRequest || strings.Contains(body, "AUTH_PERMISSION_DENIED") {
		t.Fatalf("granted POST must reach the handler (400 service miss), got %d: %s", status, body)
	}
}
