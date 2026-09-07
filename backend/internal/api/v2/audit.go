package v2

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/model"
	"ops-admin/backend/internal/infra/policy"
	"ops-admin/backend/internal/tasks"
	v1model "ops-admin/backend/model"
	"ops-admin/backend/opdef"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// V2 audit record path (plan §3.5 — J3, N9). Two writers land here:
//
//   (a) AuditMiddleware — one request row per v2 mutation verb
//       (execute/approve/reject/cancel; plan is stateless and stays
//       unaudited per J3(a), A9 keeps the middleware attached to every POST).
//       Denied requests are audited too (§3.4 "미권한 403(감사와 함께)").
//   (b) RecordTaskTerminal — the terminal row the engine's OnTaskTerminal
//       hook consumes (J6 wiring lives in compose): url carries the
//       task://<uid> convention so the path spaces of v1 HTTP rows and v2
//       terminal rows never collide.
//
// Both rows join on task_uid (the only §18.1 field promoted to a real
// column — J3). Everything else rides in v2_context as JSON and is asserted
// key-by-key after a Go-side decode (F-8 — MySQL normalisation forbids byte
// comparison). request_summary in this lane carries hashes, codes and
// booleans ONLY — never a serialized request body (VK-11; the v1 lane's
// body-carrying middleware in middleware/operation_log.go is untouched and
// skips /api/v2 by its own prefix guard).

// auditContextKey is the gin key under which handlers stash the §18.1
// context they assembled (policy version, resolved uids, hashes). The
// middleware reads it after c.Next() — handlers may fail before stashing
// everything, and denied requests stash nothing: every missing field is
// simply absent from v2_context (row completeness is not contractual; the
// task↔audit union is — claim 8).
const auditContextKey = "v2.audit"

// auditInfo is the handler-assembled §18.1 context. All fields optional.
type auditInfo struct {
	Operation             string
	OperationVersion      string
	RiskLevel             string
	PolicyVersion         string
	ProviderConnectionUID string
	ProviderContextUID    string
	ResourceUID           string
	TaskUID               string
	RequestHash           string
	ErrorCode             string
	IdempotencyPresent    bool
}

// AuditMiddleware returns the v2 request-audit middleware. Attach it to the
// /api/v2/infra group (A9): GETs and the stateless plan verb return without
// writing; the four mutation verbs write one sys_operation_log row each.
func AuditMiddleware(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		if c.Request.Method != http.MethodPost {
			return
		}
		if !strings.HasPrefix(c.Request.URL.Path, "/api/v2/infra/") {
			return
		}
		if strings.HasSuffix(c.Request.URL.Path, "/plan") {
			return // J3(a): plan is stateless — the four verbs only
		}

		stash, _ := c.Get(auditContextKey)
		info, _ := stash.(*auditInfo)
		if info == nil {
			info = &auditInfo{}
		}
		if info.Operation == "" {
			info.Operation = c.Param("name") // operations routes derive from the path
		}
		if info.TaskUID == "" {
			info.TaskUID = c.Param("uid") // tasks routes carry it in the path
		}
		if def, ok := v2DefForRequest(c); ok {
			info.RiskLevel = def.Risk
		}

		requestID := strings.TrimSpace(c.GetHeader("X-Request-ID"))
		if requestID == "" {
			requestID = randomHex(16)
		}
		traceID := traceIDFromHeader(c.GetHeader("traceparent"))
		if traceID == "" {
			traceID = randomHex(16) // claim 8: presence is the assertion — key 존재 자체가 단얫 대상
		}

		contextMap := map[string]any{
			"request_id": requestID,
			"trace_id":   traceID,
			"mutating":   true,
		}
		putIfNotEmpty(contextMap,
			[2]string{"operation", info.Operation},
			[2]string{"operation_version", info.OperationVersion},
			[2]string{"policy_version", info.PolicyVersion},
			[2]string{"provider_connection_uid", info.ProviderConnectionUID},
			[2]string{"provider_context_uid", info.ProviderContextUID},
			[2]string{"resource_uid", info.ResourceUID},
			[2]string{"task_uid", info.TaskUID},
			[2]string{"request_hash", info.RequestHash},
			[2]string{"error_code", info.ErrorCode},
		)
		contextJSON, err := json.Marshal(contextMap)
		if err != nil {
			contextJSON = []byte("{}")
		}

		statusCode := c.Writer.Status()
		taskUID, policyVersion, mutating := info.TaskUID, info.PolicyVersion, true
		row := v1model.OperationLog{
			AdminID:     adminID(c),
			Username:    username(c),
			Method:      c.Request.Method,
			IP:          c.ClientIP(),
			URL:         c.Request.URL.Path,
			Description: "v2 " + mutationVerb(c.Request.URL.Path),
			RiskLevel:   info.RiskLevel,
			StatusCode:  statusCode,
			Success:     statusCode >= 200 && statusCode < 400,
			DurationMs:  time.Since(start).Milliseconds(),
			// VK-11: hashes, codes, booleans — no request body, no key values.
			RequestSummary: fmt.Sprintf("request_hash=%s;error_code=%s;idempotency_key=%s",
				info.RequestHash, info.ErrorCode, map[bool]string{true: "present", false: "absent"}[info.IdempotencyPresent]),
			TaskUID:       &taskUID,
			PolicyVersion: &policyVersion,
			Mutating:      &mutating,
			V2Context:     strPtr(string(contextJSON)),
			CreatedAt:     time.Now(),
		}
		db.Create(&row)
	}
}

// RecordTaskTerminal writes the §18.1 terminal row (J3(b)) — the consumer of
// the engine's OnTaskTerminal hook (J6; compose owns the wiring). The row
// joins the request rows on task_uid and carries the terminal-only fields:
// result_hash (over the normalised status detail), error_code, the approval
// snapshot, and provider_task_id resolved from the latest attempt's handle
// reference. url follows the task://<uid> convention so v1 HTTP audit rows
// and terminal rows never share a path space.
func RecordTaskTerminal(_ context.Context, db *gorm.DB, task model.ProviderTask, status string, detail contract.JSONMap) error {
	resultHash := canonicalHash(detail)
	providerTaskID := ""
	var attempt model.TaskAttempt
	if err := db.Where("task_id = ?", task.ID).Order("attempt_no DESC").First(&attempt).Error; err == nil {
		providerTaskID = attempt.HandleRef
	}

	contextMap := map[string]any{
		"task_uid":          task.UID,
		"operation":         task.OperationName,
		"operation_version": task.OperationVersion,
		"mutating":          true,
		"policy_version":    policy.BuiltinDefaultAllow, // M1: the only ruleset — the request row carries the evaluated decision's version
		"resource_uid":      task.ResourceUID,
		"result_hash":       resultHash,
		"approval_status":   task.ApprovalStatus,
	}
	if task.ErrorCode != "" {
		contextMap["error_code"] = task.ErrorCode
	}
	if task.Approver != "" {
		contextMap["approver"] = task.Approver
	}
	if providerTaskID != "" {
		contextMap["provider_task_id"] = providerTaskID
	}
	contextJSON, err := json.Marshal(contextMap)
	if err != nil {
		contextJSON = []byte("{}")
	}

	succeeded := status == tasks.TaskStatusSucceeded
	statusCode := http.StatusInternalServerError
	if succeeded {
		statusCode = http.StatusOK
	}
	taskUID, policyVersion, mutating := task.UID, policy.BuiltinDefaultAllow, true
	row := v1model.OperationLog{
		Method:     "TASK",
		URL:        "task://" + task.UID,
		StatusCode: statusCode,
		Success:    succeeded,
		// VK-11: hash, code, status word only.
		RequestSummary: fmt.Sprintf("result_hash=%s;error_code=%s;status=%s", resultHash, task.ErrorCode, status),
		TaskUID:        &taskUID,
		PolicyVersion:  &policyVersion,
		Mutating:       &mutating,
		V2Context:      strPtr(string(contextJSON)),
		CreatedAt:      time.Now(),
	}
	if err := db.Create(&row).Error; err != nil {
		return fmt.Errorf("v2 audit: terminal row for task %s: %w", task.UID, err)
	}
	return nil
}

// --- helpers ---

// v2DefForRequest resolves the opdef row of the requested v2 route (the
// route pattern minus the /api/v2 group prefix is the table's Path).
func v2DefForRequest(c *gin.Context) (opdef.Def, bool) {
	pattern := c.FullPath()
	if pattern == "" {
		return opdef.Def{}, false
	}
	path := strings.TrimPrefix(pattern, "/api/v2")
	for _, d := range opdef.All() {
		if d.Method == c.Request.Method && d.Path == path {
			return d, true
		}
	}
	return opdef.Def{}, false
}

// mutationVerb reduces the URL to the audit description word.
func mutationVerb(path string) string {
	switch {
	case strings.HasSuffix(path, "/execute"):
		return "operation execute"
	case strings.HasSuffix(path, "/plan"):
		return "operation plan"
	case strings.HasSuffix(path, "/approve"):
		return "task approve"
	case strings.HasSuffix(path, "/reject"):
		return "task reject"
	case strings.HasSuffix(path, "/cancel"):
		return "task cancel"
	default:
		return "mutation"
	}
}

// traceIDFromHeader extracts the trace-id from a W3C traceparent
// ("00-<32hex>-<16hex>-<flags>"); anything malformed yields "".
func traceIDFromHeader(header string) string {
	parts := strings.Split(strings.TrimSpace(header), "-")
	if len(parts) != 4 || len(parts[1]) != 32 {
		return ""
	}
	return strings.ToLower(parts[1])
}

func putIfNotEmpty(m map[string]any, pairs ...[2]string) {
	for _, pair := range pairs {
		if pair[1] != "" {
			m[pair[0]] = pair[1]
		}
	}
}

func canonicalHash(value contract.JSONMap) string {
	// encoding/json sorts map keys, so the marshal is canonical (compact,
	// sorted) — the same normalisation rule on both the request and the
	// result hash (§3.5).
	encoded, err := json.Marshal(value)
	if err != nil {
		encoded = []byte("{}")
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

func randomHex(bytes int) string {
	buf := make([]byte, bytes)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}

func strPtr(value string) *string { return &value }

func adminID(c *gin.Context) uint {
	if id, ok := c.Get("userID"); ok {
		if v, ok := id.(uint); ok {
			return v
		}
	}
	return 0
}

func username(c *gin.Context) string {
	if name, ok := c.Get("username"); ok {
		if v, ok := name.(string); ok {
			return v
		}
	}
	return ""
}
