package v2

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"ops-admin/backend/httpx"
	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/model"
	"ops-admin/backend/internal/infra/policy"
	"ops-admin/backend/internal/tasks"
	"ops-admin/backend/opdef"

	"github.com/gin-gonic/gin"
)

// CancelStarter is the RequestCancel seam (plan §3.6, N6): the engine lane
// owns tasks/cancel.go, and *tasks.Engine satisfies this interface the moment
// Engine.RequestCancel lands. Until then the field stays nil and the cancel
// route degrades 503 — the route, its opdef row and its audit coverage exist
// from this commit; the delegation activates without further edits here.
type CancelStarter interface {
	RequestCancel(ctx context.Context, taskUID, actor string) error
}

// RegisterOperations wires the §16.2 mutation surface onto the v2 group
// (plan M8 — five mutation POSTs plus four reads, the tasks 목록 carrying
// the Phase 3/4 gap; the Phase 2 GET subset in Register is untouched so its
// route-count pin stays meaningful). The group
// already carries Auth + OperationLog from the router; this adds the v2 audit
// middleware (A9) first — before any route-level middleware, so permission
// denials stay audited (§3.4 "미권한 403(감사와 함께)") — and then each route
// with the grant middleware the caller supplies.
//
// grants turns an opdef row into the route-level permission middleware; the
// router passes opdef.V2DynamicMiddleware (J4 — plan/execute resolve the
// registry permission, the task verbs fall back to their ops:job:approve
// representative), mirroring the v1 convention that permission middleware is
// a router concern. Unit tests pass nil.
func (a *InfraAPI) RegisterOperations(group *gin.RouterGroup, grants func(def opdef.Def) gin.HandlerFunc) {
	group.Use(AuditMiddleware(a.db))
	grant := func(path string) gin.HandlerFunc {
		if grants == nil {
			return func(c *gin.Context) { c.Next() }
		}
		return grants(opdef.Must(http.MethodPost, path))
	}

	group.GET("/resources/:uid/operations", a.ListResourceOperations)
	group.POST("/resources/:uid/operations/:name/plan", grant("/infra/resources/:uid/operations/:name/plan"), a.PlanOperation)
	group.POST("/resources/:uid/operations/:name/execute", grant("/infra/resources/:uid/operations/:name/execute"), a.ExecuteOperation)
	group.GET("/tasks", a.ListTasks)
	group.GET("/tasks/:uid", a.GetTask)
	group.GET("/tasks/:uid/events", a.GetTaskEvents)
	group.POST("/tasks/:uid/approve", grant("/infra/tasks/:uid/approve"), a.ApproveTask)
	group.POST("/tasks/:uid/reject", grant("/infra/tasks/:uid/reject"), a.RejectTask)
	group.POST("/tasks/:uid/cancel", grant("/infra/tasks/:uid/cancel"), a.CancelTask)
}

// ResolveOperationPermission is the registry lookup injected into the opdef
// dynamic middleware (opdef's R3 purity boundary forbids importing the
// registry itself).
func (a *InfraAPI) ResolveOperationPermission(name string) (string, bool) {
	if a.registry == nil {
		return "", false
	}
	def, ok := a.registry.Operation(name)
	if !ok {
		return "", false
	}
	return def.RequiredPermission, true
}

// resolveOperationDefinition loads the registry operation for the route's
// :name parameter — unknown operations 404 (§3.4), a degraded registry 503.
func (a *InfraAPI) resolveOperationDefinition(c *gin.Context) (contract.OperationDefinition, bool) {
	if a.registry == nil {
		httpx.FailedCode(c, http.StatusServiceUnavailable, "INFRA_V2_STACK_UNAVAILABLE", nil)
		return contract.OperationDefinition{}, false
	}
	def, ok := a.registry.Operation(c.Param("name"))
	if !ok {
		stashAudit(c, &auditInfo{ErrorCode: "OPERATION_NOT_REGISTERED"})
		httpx.FailedCode(c, http.StatusNotFound, "OPERATION_NOT_REGISTERED", nil)
		return contract.OperationDefinition{}, false
	}
	return def, true
}

// loadLiveResource loads the §5.4c live resource (stale sources are absent)
// plus its context and connection — the plan/execute authorization inputs.
func (a *InfraAPI) loadLiveResource(c *gin.Context) (model.InfraResource, model.ProviderContext, model.ProviderConnection, bool) {
	var res model.InfraResource
	if err := liveResources(a.db).Where("infra_resource.uid = ?", c.Param("uid")).First(&res).Error; err != nil {
		stashAudit(c, &auditInfo{ErrorCode: "RESOURCE_NOT_FOUND"})
		httpx.FailedCode(c, http.StatusNotFound, "RESOURCE_NOT_FOUND", nil)
		return model.InfraResource{}, model.ProviderContext{}, model.ProviderConnection{}, false
	}
	var contextRow model.ProviderContext
	if err := a.db.First(&contextRow, res.ContextID).Error; err != nil {
		httpx.Failed(c, http.StatusInternalServerError, err.Error())
		return model.InfraResource{}, model.ProviderContext{}, model.ProviderConnection{}, false
	}
	var conn model.ProviderConnection
	if err := a.db.First(&conn, contextRow.ConnectionID).Error; err != nil {
		httpx.Failed(c, http.StatusInternalServerError, err.Error())
		return model.InfraResource{}, model.ProviderContext{}, model.ProviderConnection{}, false
	}
	return res, contextRow, conn, true
}

// evaluateOperationPolicy runs the §10.3 second authorization gate (J5):
// an evaluation ERROR is a 403 fail-closed (VK-4), an evaluated deny is a
// plain 403. The evaluated decision's version feeds the audit row.
func (a *InfraAPI) evaluateOperationPolicy(c *gin.Context, info *auditInfo, def contract.OperationDefinition, res model.InfraResource, contextRow model.ProviderContext, conn model.ProviderConnection) (policy.Decision, bool) {
	decision, err := policy.Evaluate(policy.PolicyInput{
		ProviderType: conn.ProviderType,
		ContextKind:  contextRow.Kind,
		ResourceKind: res.Kind,
		Risk:         def.RiskLevel,
		Mutating:     def.Mutating,
	})
	info.ProviderConnectionUID = conn.UID
	info.ProviderContextUID = contextRow.UID
	info.ResourceUID = res.UID
	info.Operation = def.Name
	info.OperationVersion = def.Version
	info.RiskLevel = def.RiskLevel
	if err != nil {
		info.ErrorCode = "POLICY_EVALUATION_FAILED"
		stashAudit(c, info)
		httpx.FailedCode(c, http.StatusForbidden, "POLICY_EVALUATION_FAILED", nil)
		return policy.Decision{}, false
	}
	info.PolicyVersion = decision.PolicyVersion
	if !decision.Allow {
		info.ErrorCode = "POLICY_DENIED"
		stashAudit(c, info)
		httpx.FailedCode(c, http.StatusForbidden, "POLICY_DENIED", nil)
		return policy.Decision{}, false
	}
	return decision, true
}

// PlanOperation — POST /resources/:uid/operations/:name/plan (§3.4): pure
// computation, no state change, no task. The response freezes restartedAt
// (J1 — the provider-side idempotency anchor) and snapshots the latest
// observation's k8s revision as resourceRevision (J7).
func (a *InfraAPI) PlanOperation(c *gin.Context) {
	def, ok := a.resolveOperationDefinition(c)
	if !ok {
		return
	}
	res, contextRow, conn, ok := a.loadLiveResource(c)
	if !ok {
		return
	}
	info := &auditInfo{}
	decision, ok := a.evaluateOperationPolicy(c, info, def, res, contextRow, conn)
	if !ok {
		return
	}

	restartedAt := time.Now().UTC().Format(time.RFC3339)
	revision, hasRevision := a.latestResourceRevision(res.ID)
	response := gin.H{
		"operation":        def.Name,
		"version":          def.Version,
		"resourceUid":      res.UID,
		"requiresApproval": def.RequiresApproval,
		"riskLevel":        def.RiskLevel,
		"permission":       def.RequiredPermission,
		"policyVersion":    decision.PolicyVersion,
		"restartedAt":      restartedAt,
	}
	if hasRevision {
		response["resourceRevision"] = revision
	} else {
		response["resourceRevision"] = nil
	}
	stashAudit(c, info)
	httpx.Success(c, response)
}

// ExecuteOperation — POST /resources/:uid/operations/:name/execute (§3.4):
// Idempotency-Key mandatory (the task-creation idempotency of §13.4;
// approve/reject/cancel stay key-free by design — they converge on the same
// terminal state on reissue, so a mandatory key there would add client
// burden without a duplicate-creation surface to protect — r2 L1). The
// frozen restartedAt rides in the payload the engine snapshots (J1), the
// first submit answers 201, a key replay answers 200 + Idempotency-Replayed
// (r2 L2), and resource_busy fast-fails 409 (N11).
func (a *InfraAPI) ExecuteOperation(c *gin.Context) {
	idempotencyKey := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	if idempotencyKey == "" {
		httpx.FailedCode(c, http.StatusBadRequest, "IDEMPOTENCY_KEY_REQUIRED", nil)
		return
	}
	def, ok := a.resolveOperationDefinition(c)
	if !ok {
		return
	}
	res, contextRow, conn, ok := a.loadLiveResource(c)
	if !ok {
		return
	}
	info := &auditInfo{IdempotencyPresent: true}
	decision, ok := a.evaluateOperationPolicy(c, info, def, res, contextRow, conn)
	if !ok {
		return
	}
	if a.engine == nil {
		info.ErrorCode = "ENGINE_UNAVAILABLE"
		stashAudit(c, info)
		httpx.FailedCode(c, http.StatusServiceUnavailable, "ENGINE_UNAVAILABLE", nil)
		return
	}

	payload, err := a.executePayload(c, res)
	if err != nil {
		info.ErrorCode = "INVALID_OPERATION_PAYLOAD"
		stashAudit(c, info)
		httpx.FailedCode(c, http.StatusBadRequest, "INVALID_OPERATION_PAYLOAD", nil)
		return
	}
	info.RequestHash = canonicalHash(payload)

	task, replayed, err := a.engine.Submit(c.Request.Context(), tasks.SubmitInput{
		OperationName:    def.Name,
		OperationVersion: def.Version,
		ResourceUID:      res.UID,
		Payload:          payload,
		IdempotencyKey:   idempotencyKey,
	})
	if err != nil {
		if errors.Is(err, tasks.ErrResourceBusy) {
			info.ErrorCode = "RESOURCE_BUSY"
			stashAudit(c, info)
			httpx.FailedCode(c, http.StatusConflict, "RESOURCE_BUSY", nil)
			return
		}
		info.ErrorCode = "SUBMIT_FAILED"
		stashAudit(c, info)
		httpx.Failed(c, http.StatusInternalServerError, err.Error())
		return
	}
	_ = decision
	info.TaskUID = task.UID
	stashAudit(c, info)

	if replayed {
		// §13.4 (r2 L2): the replayed submission is a 200 that says so.
		c.Header("Idempotency-Replayed", "true")
		c.JSON(http.StatusOK, gin.H{"code": http.StatusOK, "message": "success", "data": gin.H{"task": newTaskView(task)}})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"code": http.StatusCreated, "message": "success", "data": gin.H{"task": newTaskView(task)}})
}

// executePayload assembles the frozen execution payload: the request body
// may carry the plan-flow values (restartedAt from the plan response, the
// snapshotted resourceRevision — J7); restartedAt is validated RFC3339 when
// supplied and server-generated when not. Whatever lands here is what the
// engine freezes — every attempt re-applies the identical patch (J1).
func (a *InfraAPI) executePayload(c *gin.Context, res model.InfraResource) (contract.JSONMap, error) {
	payload := contract.JSONMap{}
	if c.Request.Body != nil && c.Request.ContentLength != 0 {
		if err := json.NewDecoder(c.Request.Body).Decode(&payload); err != nil {
			return nil, err
		}
	}
	if raw, ok := payload["restartedAt"]; ok {
		stamp, isString := raw.(string)
		if !isString {
			return nil, errors.New("restartedAt must be an RFC3339 string")
		}
		if _, err := time.Parse(time.RFC3339, stamp); err != nil {
			return nil, errors.New("restartedAt must be RFC3339")
		}
	} else {
		payload["restartedAt"] = time.Now().UTC().Format(time.RFC3339)
	}
	if _, ok := payload["resourceRevision"]; !ok {
		if revision, ok := a.latestResourceRevision(res.ID); ok {
			payload["resourceRevision"] = revision
		}
	}
	return payload, nil
}

// latestResourceRevision probes the latest observation for a k8s generation
// (J7's "resource revision" interpretation). F-9 double-source note: the
// current kubernetes normalizer does not emit a generation key, so the probe
// is best-effort and answers false when the observation carries none — the
// executor's terminal detail (generation) remains the authoritative revision
// evidence. Key set deliberately small: whatever the normalizer adds later
// ("generation" normalized or raw) flows through unchanged.
func (a *InfraAPI) latestResourceRevision(resourceID uint) (any, bool) {
	var obs model.ResourceObservation
	if err := a.db.Where("resource_id = ?", resourceID).Order("observed_at DESC, id DESC").First(&obs).Error; err != nil {
		return nil, false
	}
	if v, ok := obs.NormalizedJSON["generation"]; ok {
		return v, true
	}
	if v, ok := obs.RawJSON["generation"]; ok {
		return v, true
	}
	if rawMeta, ok := obs.RawJSON["metadata"].(map[string]any); ok {
		if v, ok := rawMeta["generation"]; ok {
			return v, true
		}
	}
	return nil, false
}

// ListResourceOperations — GET /resources/:uid/operations (§16.1): the
// intersection of the registry's operation definitions with the resource's
// kind. F-9 (이중 소스 한계): the registry definition is one source and the
// resource kind the other — what the route reports is the definitional
// intersection, not a capability probe; the execution path (engine → registry
// → adapter) re-validates capability/permission at submit time.
func (a *InfraAPI) ListResourceOperations(c *gin.Context) {
	if a.registry == nil {
		httpx.FailedCode(c, http.StatusServiceUnavailable, "INFRA_V2_STACK_UNAVAILABLE", nil)
		return
	}
	var res model.InfraResource
	if err := liveResources(a.db).Where("infra_resource.uid = ?", c.Param("uid")).First(&res).Error; err != nil {
		httpx.FailedCode(c, http.StatusNotFound, "RESOURCE_NOT_FOUND", nil)
		return
	}
	defs := a.registry.Operations()
	items := make([]gin.H, 0, len(defs))
	for _, def := range defs {
		matched := false
		for _, kind := range def.ResourceKinds {
			if kind == res.Kind {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		items = append(items, gin.H{
			"name":             def.Name,
			"version":          def.Version,
			"mutating":         def.Mutating,
			"riskLevel":        def.RiskLevel,
			"requiresApproval": def.RequiresApproval,
			"permission":       def.RequiredPermission,
		})
	}
	httpx.Success(c, gin.H{"items": items, "total": len(items)})
}

func stashAudit(c *gin.Context, info *auditInfo) {
	if existing, ok := c.Get(auditContextKey); ok {
		if merged, ok := existing.(*auditInfo); ok && merged != nil {
			// keep the first (richest) stash — later errors only annotate
			if info.ErrorCode != "" {
				merged.ErrorCode = info.ErrorCode
			}
			return
		}
	}
	c.Set(auditContextKey, info)
}
