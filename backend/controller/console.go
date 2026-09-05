package controller

import (
	"errors"
	"net/http"

	"ops-admin/backend/httpx"
	"ops-admin/backend/service"

	"github.com/gin-gonic/gin"
)

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

// consumeTerminalTicket gates both terminal websocket handlers. The ticket
// parameter is mandatory and consumed atomically: a missing ticket is rejected
// with 401 (the legacy query-token path was removed in Release C), an unknown,
// expired or reused ticket with 401, and a valid ticket bound to another
// resource or protocol with 403 — the ticket is bound to exactly one resource
// and one protocol.
func (ctl *Controller) consumeTerminalTicket(c *gin.Context, protocol string, resourceType string, resourceID string) bool {
	ticket := c.Query("ticket")
	if ticket == "" {
		httpx.Failed(c, http.StatusUnauthorized, "console ticket required")
		return false
	}
	err := ctl.service.ConsumeConsoleTicket(ticket, protocol, resourceType, resourceID)
	if err != nil {
		if errors.Is(err, service.ErrConsoleTicketMismatch) {
			httpx.Failed(c, http.StatusForbidden, err.Error())
		} else {
			httpx.Failed(c, http.StatusUnauthorized, "invalid or expired console ticket")
		}
		return false
	}
	return true
}
