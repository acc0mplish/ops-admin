package controller

import (
	"errors"
	"net/http"
	"strings"

	"ops-admin/backend/auth"
	"ops-admin/backend/httpx"
	"ops-admin/backend/service"

	"github.com/gin-gonic/gin"
)

// legacyTerminalSunset is the placeholder retirement date of the legacy
// query-token websocket path; Release B confirms the final date.
const legacyTerminalSunset = "Thu, 31 Dec 2026 23:59:59 GMT"

// legacyTerminalWSHeaders announces the deprecation of the legacy
// query-token path on the 101 handshake. They must reach the Upgrade call as
// its responseHeader argument: gorilla writes the 101 response itself and
// only merges this explicit header into the raw bytes — headers set on the
// gin writer beforehand are lost.
var legacyTerminalWSHeaders = http.Header{
	"Deprecation": []string{"true"},
	"Sunset":      []string{legacyTerminalSunset},
}

// consoleResourcePermissions maps a ticket resource type onto the terminal
// button permission that guards it — the same seeded vocabulary the
// /console-sessions AnyOf gate lists.
var consoleResourcePermissions = map[string]string{
	service.ConsoleResourceAssetHost: "assets:host:terminal",
	service.ConsoleResourceK8sPod:    "assets:k8s:pod:terminal",
}

// CreateConsoleSession mints a one-time console ticket. The endpoint-level
// AnyOf gate is deliberately loose (either terminal permission passes), so
// the handler re-verifies the resource-specific permission (M-1) before the
// mint; the audit log records the issuance through the authGroup pipeline.
func (ctl *Controller) CreateConsoleSession(c *gin.Context) {
	var req struct {
		ResourceType string `json:"resourceType"`
		ResourceID   string `json:"resourceId"`
		Protocol     string `json:"protocol"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Failed(c, http.StatusBadRequest, "invalid console session payload")
		return
	}
	// Contract order: bind → canonical binding key (normalizes the resource ID;
	// unknown resourceType pairs are rejected here, but protocol pairing is
	// validated inside mint, after the M-1 permission check) → resource
	// permission (M-1) → mint.
	resourceID, err := service.CanonicalConsoleResourceID(req.ResourceType, req.ResourceID)
	if err != nil {
		httpx.Failed(c, http.StatusBadRequest, err.Error())
		return
	}
	userID := c.GetUint("userID")
	if !ctl.service.AdminHasPermission(userID, consoleResourcePermissions[req.ResourceType]) {
		httpx.Failed(c, http.StatusForbidden, "Permission denied")
		return
	}
	data, err := ctl.service.MintConsoleTicket(req.ResourceType, resourceID, req.Protocol, userID)
	if err != nil {
		if errors.Is(err, service.ErrConsoleResourceInvalid) {
			httpx.Failed(c, http.StatusBadRequest, err.Error())
			return
		}
		httpx.FailedError(c, http.StatusBadRequest, err)
		return
	}
	httpx.Success(c, data)
}

// authorizeTerminalWS gates both terminal websocket handlers. A ticket
// parameter, when present, is consumed atomically: an unknown, expired or
// reused ticket is rejected with 401 and never falls back to the legacy path,
// and a valid ticket bound to another resource or protocol is rejected with
// 403 — the ticket is bound to exactly one resource and one protocol. Without
// a ticket the legacy query-token path answers (Release C removes it) and the
// returned headers announce its deprecation on the 101 handshake.
func (ctl *Controller) authorizeTerminalWS(c *gin.Context, protocol string, resourceType string, resourceID string) (http.Header, bool) {
	if ticket := c.Query("ticket"); ticket != "" {
		err := ctl.service.ConsumeConsoleTicket(ticket, protocol, resourceType, resourceID)
		if err != nil {
			if errors.Is(err, service.ErrConsoleTicketMismatch) {
				httpx.Failed(c, http.StatusForbidden, err.Error())
			} else {
				httpx.Failed(c, http.StatusUnauthorized, "invalid or expired console ticket")
			}
			return nil, false
		}
		return nil, true
	}

	token := c.Query("token")
	if strings.TrimSpace(token) == "" {
		httpx.Failed(c, http.StatusUnauthorized, "authentication required")
		return nil, false
	}
	if _, err := auth.ParseToken(token); err != nil {
		httpx.Failed(c, http.StatusUnauthorized, auth.TokenErrorMessage(err))
		return nil, false
	}
	return legacyTerminalWSHeaders, true
}
