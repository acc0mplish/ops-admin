package router

import (
	"net/http"
	"testing"
)

// TestConsoleSessionMintStatusMapping (LOW-1) pins the mint failure mapping of
// POST /console-sessions: an invalid resource payload is the caller's fault
// and stays 400, while a mint that fails internally (here: the ticket table
// is unavailable) must surface as a 5xx instead of masquerading as a bad
// request.
func TestConsoleSessionMintStatusMapping(t *testing.T) {
	engine, db := newArtifactEngine(t)
	grantSinglePermission(t, db, 9201, "assets:host:terminal")
	token := replaySession(t, db, 9201, "mint-mapping-admin")

	status, body := doRequest(engine, token, http.MethodPost, apiPrefix+"/console-sessions",
		[]byte(`{"resourceType":"bogus","resourceId":"12","protocol":"asset-terminal"}`))
	if status != http.StatusBadRequest {
		t.Fatalf("invalid resource mint returned %d: %s, want 400", status, body)
	}

	if err := db.Migrator().DropTable("sys_console_ticket"); err != nil {
		t.Fatalf("drop sys_console_ticket: %v", err)
	}
	status, body = doRequest(engine, token, http.MethodPost, apiPrefix+"/console-sessions",
		[]byte(`{"resourceType":"asset_host","resourceId":"12","protocol":"asset-terminal"}`))
	if status < 500 || status > 599 {
		t.Fatalf("internal mint failure returned %d: %s, want 5xx", status, body)
	}
}
