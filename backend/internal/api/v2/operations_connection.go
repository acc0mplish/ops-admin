// operations_connection.go — I10 J1c(§16.1): 커넥션-스코프 create 면
// `POST /api/v2/infra/provider-connections/:uid/operations/:name/plan|execute`.
//
// uid-스코프 오퍼레이션(operations.go)과 다른 스코프 앵커다 — create 대상은
// 아직 존재하지 않아 infra_resource 행이 없고(리소스 uid 조인 불가), 스코프는
// 커넥션 uid + 매니페스트 신원(I-P1 블로커 해소)이 된다. plan은 매니페스트를
// 파싱해 매핑 표 kind를 조달하고(policy.Evaluate 입력 — 공백 kind는
// fail-closed, VK-4) 검증 결과를 회신한다(restartedAt·resourceRevision
// 미발급 — create의 동결 앵커는 매니페스트 자신). execute는 합성 uid
// conn:<connectionUID>:<singular>:<target>을 도출해 엔진에 Submit한다
// (engine_resolve.go의 conn: 분기가 커넥션을 직조립).
//
// 매니페스트 검증은 실행기 leg(adapter/kubernetes executor_create.go
// createManifestOf)와 동일 강도를 여기서 다시 적용한다 — API 계층은 policy
// 평가 이전에 평가 불가 입력을 400으로 잘라내는 사전 차단이 소관(VK-4)이고,
// 종 어휘는 contract.K8sCreateFace 단일 원천 조회뿐이라 §3.2.1 파생 계약
// (이중 열거 금지)는 유지된다.
package v2

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"ops-admin/backend/httpx"
	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/model"
	"ops-admin/backend/internal/infra/policy"
	"ops-admin/backend/internal/tasks"
	"ops-admin/backend/opdef"

	"github.com/gin-gonic/gin"
	ksyaml "sigs.k8s.io/yaml"
)

// connectionScopedUIDPrefix — 합성 resource_uid 접두부. tasks 패키지
// connUIDPrefix(engine_resolve.go)와 동일 리터럴이다 — 두 계약의 정합은
// engine_resolve_test와 operations_connection_test의 왕복 단얫이 잠근다.
const connectionScopedUIDPrefix = "conn:"

// syntheticResourceUIDBudget — provider_task.resource_uid 칼럼 폭
// (step0008 확장 후 255, §3.3 r3·§5 #18). sqlite 테스트 러너는 varchar를
// 강제하지 않으므로 이 상한의 유일한 소유자는 본 빌더 가드다 — 초과 산출은
// 400 INVALID_OPERATION_PAYLOAD로 거부된다.
const syntheticResourceUIDBudget = 255

// RegisterConnectionOperations wires the §16.1 connection-scoped surface onto
// the v2 group. Mirror of RegisterOperations with one precondition: it must
// run on a group that RegisterOperations has already mounted — the audit
// middleware (A9) is attached there exactly once, and a second group.Use here
// would double-write every audit row on these routes. Route order in
// registerV2 (routes_v2.go) pins the precondition.
func (a *InfraAPI) RegisterConnectionOperations(group *gin.RouterGroup, grants func(def opdef.Def) gin.HandlerFunc) {
	grant := func(path string) gin.HandlerFunc {
		if grants == nil {
			return func(c *gin.Context) { c.Next() }
		}
		return grants(opdef.Must(http.MethodPost, path))
	}

	group.POST("/provider-connections/:uid/operations/:name/plan", grant("/infra/provider-connections/:uid/operations/:name/plan"), a.PlanConnectionOperation)
	group.POST("/provider-connections/:uid/operations/:name/execute", grant("/infra/provider-connections/:uid/operations/:name/execute"), a.ExecuteConnectionOperation)
}

// connectionCreateTarget — plan·execute가 공유하는 검증 결과: 동결 매니페스트,
// 매핑 표 행(단일 원천), 그리고 매니페스트 신원(name·namespace)과 원문 yaml.
type connectionCreateTarget struct {
	Manifest  map[string]any              // 동결 매니페스트 — plan 검증·execute 동결 본문의 근거
	Entry     contract.K8sCreateFaceEntry // 매핑 표 행 — kind 면의 단일 원천(§3.2.1)
	Name      string                      // metadata.name(trimmed)
	Namespace string                      // namespaced 종의 metadata.namespace — 클러스터 스코프는 공백
	YAML      string                      // 요청 원문 — execute payload는 {yaml} 그대로 동결(주입 없음)
}

// PlanConnectionOperation — POST /provider-connections/:uid/operations/:name/plan
// (§3.5): 매니페스트 파싱·매핑 표 조회를 통과한 policy 평가 결과를 회신하는
// stateless 계산이다. task 행 없음·restartedAt/resourceRevision 미발급.
func (a *InfraAPI) PlanConnectionOperation(c *gin.Context) {
	def, ok := a.resolveOperationDefinition(c)
	if !ok {
		return
	}
	conn, contextRow, ok := a.loadConnectionForOperation(c)
	if !ok {
		return
	}
	info := &auditInfo{}
	target, err := parseConnectionCreatePayload(c)
	if err != nil {
		info.ErrorCode = "INVALID_OPERATION_PAYLOAD"
		stashAudit(c, info)
		httpx.FailedCode(c, http.StatusBadRequest, "INVALID_OPERATION_PAYLOAD", nil)
		return
	}
	decision, ok := a.evaluateConnectionPolicy(c, info, def, target.Entry, contextRow, conn)
	if !ok {
		return
	}
	stashAudit(c, info)
	httpx.Success(c, gin.H{
		"operation":        def.Name,
		"version":          def.Version,
		"connectionUid":    conn.UID,
		"targetKind":       target.Entry.ResourceKind,
		"requiresApproval": def.RequiresApproval,
		"riskLevel":        def.RiskLevel,
		"permission":       def.RequiredPermission,
		"policyVersion":    decision.PolicyVersion,
	})
}

// ExecuteConnectionOperation — POST /provider-connections/:uid/operations/:name/execute
// (§3.5): Idempotency-Key 필수(§13.4), 동일 검증·policy를 통과한 뒤 합성 uid를
// 도출해 engine.Submit한다. payload는 {yaml} 그대로(주입 없음 — 재실행은
// byte-identical POST, §3.4). 첫 제출 201·키 재생 200 + Idempotency-Replayed·
// 동일 목표 활성 충돌 409 RESOURCE_BUSY(합성 uid 단위 직렬화 — §3.3).
func (a *InfraAPI) ExecuteConnectionOperation(c *gin.Context) {
	idempotencyKey := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	if idempotencyKey == "" {
		httpx.FailedCode(c, http.StatusBadRequest, "IDEMPOTENCY_KEY_REQUIRED", nil)
		return
	}
	def, ok := a.resolveOperationDefinition(c)
	if !ok {
		return
	}
	conn, contextRow, ok := a.loadConnectionForOperation(c)
	if !ok {
		return
	}
	info := &auditInfo{IdempotencyPresent: true}
	target, err := parseConnectionCreatePayload(c)
	if err != nil {
		info.ErrorCode = "INVALID_OPERATION_PAYLOAD"
		stashAudit(c, info)
		httpx.FailedCode(c, http.StatusBadRequest, "INVALID_OPERATION_PAYLOAD", nil)
		return
	}
	decision, ok := a.evaluateConnectionPolicy(c, info, def, target.Entry, contextRow, conn)
	if !ok {
		return
	}
	if a.engine == nil {
		info.ErrorCode = "ENGINE_UNAVAILABLE"
		stashAudit(c, info)
		httpx.FailedCode(c, http.StatusServiceUnavailable, "ENGINE_UNAVAILABLE", nil)
		return
	}

	synthetic, err := connectionResourceUID(conn.UID, target)
	if err != nil {
		// §5 #18 — sqlite 미강제 환경에서 길이 계약의 소유자는 본 빌더다.
		info.ErrorCode = "INVALID_OPERATION_PAYLOAD"
		stashAudit(c, info)
		httpx.FailedCode(c, http.StatusBadRequest, "INVALID_OPERATION_PAYLOAD", nil)
		return
	}
	payload := contract.JSONMap{"yaml": target.YAML}
	info.RequestHash = canonicalHash(payload)
	info.ResourceUID = synthetic
	_ = decision // 평가 버전은 info.PolicyVersion에 이미 기록 — Submit 입력 불요(operations.go 동일 관례)

	task, replayed, err := a.engine.Submit(c.Request.Context(), tasks.SubmitInput{
		OperationName:    def.Name,
		OperationVersion: def.Version,
		ResourceUID:      synthetic,
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
	info.TaskUID = task.UID
	stashAudit(c, info)

	if replayed {
		c.Header("Idempotency-Replayed", "true")
		c.JSON(http.StatusOK, gin.H{"code": http.StatusOK, "message": "success", "data": gin.H{"task": newTaskView(task)}})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"code": http.StatusCreated, "message": "success", "data": gin.H{"task": newTaskView(task)}})
}

// loadConnectionForOperation — §5.4c: stale_source 커넥션은 읽기 면과 동일하게
// 부재 취급된다(미등록과 같은 404). 컨텍스트는 Kind="cluster"행 — k8s 등록
// 체인(main_register_k8s.go)이 유일한 context kind다. 체인 파손(컨텍스트
// 누락)은 loadLiveResource의 체인 오류와 같은 500이다.
func (a *InfraAPI) loadConnectionForOperation(c *gin.Context) (model.ProviderConnection, model.ProviderContext, bool) {
	var conn model.ProviderConnection
	if err := a.db.Where("uid = ? AND stale_source = ?", c.Param("uid"), false).First(&conn).Error; err != nil {
		stashAudit(c, &auditInfo{ErrorCode: "CONNECTION_NOT_FOUND"})
		httpx.FailedCode(c, http.StatusNotFound, "CONNECTION_NOT_FOUND", nil)
		return model.ProviderConnection{}, model.ProviderContext{}, false
	}
	var contextRow model.ProviderContext
	if err := a.db.Where("connection_id = ? AND kind = ?", conn.ID, "cluster").First(&contextRow).Error; err != nil {
		httpx.Failed(c, http.StatusInternalServerError, err.Error())
		return model.ProviderConnection{}, model.ProviderContext{}, false
	}
	return conn, contextRow, true
}

// parseConnectionCreatePayload — body {yaml} 필수·매니페스트 구조 검증
// (apiVersion·kind·name 필수, 매핑 표 소속 group×kind, namespace 규칙:
// namespaced 종은 필수·클러스터 스코프 종은 공백). 실패는 policy 평가 불가
// 입력이라 400 INVALID_OPERATION_PAYLOAD로 사전 차단한다(VK-4).
func parseConnectionCreatePayload(c *gin.Context) (connectionCreateTarget, error) {
	if c.Request.Body == nil || c.Request.ContentLength == 0 {
		return connectionCreateTarget{}, errors.New(`the body must carry a "yaml" manifest`)
	}
	var body struct {
		YAML string `json:"yaml"`
	}
	if err := json.NewDecoder(c.Request.Body).Decode(&body); err != nil {
		return connectionCreateTarget{}, errors.New("the body must be a JSON object with a yaml string")
	}
	if strings.TrimSpace(body.YAML) == "" {
		return connectionCreateTarget{}, errors.New(`the body must carry a "yaml" manifest`)
	}
	raw, err := ksyaml.YAMLToJSON([]byte(body.YAML))
	if err != nil {
		return connectionCreateTarget{}, errors.New("invalid yaml content")
	}
	var manifest map[string]any
	if err := json.Unmarshal(raw, &manifest); err != nil || manifest == nil {
		return connectionCreateTarget{}, errors.New("invalid yaml content")
	}
	apiVersion := strings.TrimSpace(jsonStringOf(manifest["apiVersion"]))
	kind := strings.TrimSpace(jsonStringOf(manifest["kind"]))
	if apiVersion == "" || kind == "" {
		return connectionCreateTarget{}, errors.New("resource apiVersion and kind are required")
	}
	entry, ok := contract.K8sCreateFace(contract.K8sAPIGroupOf(apiVersion), kind)
	if !ok {
		return connectionCreateTarget{}, fmt.Errorf("kind %q in apiVersion %q is not a k8s.resource.create face (mapping table contract)", kind, apiVersion)
	}
	metadata, _ := manifest["metadata"].(map[string]any)
	name := strings.TrimSpace(jsonStringOf(metadata["name"]))
	if name == "" {
		return connectionCreateTarget{}, errors.New("resource name is required")
	}
	namespace := strings.TrimSpace(jsonStringOf(metadata["namespace"]))
	if entry.Namespaced && namespace == "" {
		return connectionCreateTarget{}, fmt.Errorf("create of namespaced kind %q carries no metadata.namespace", kind)
	}
	if !entry.Namespaced && namespace != "" {
		return connectionCreateTarget{}, fmt.Errorf("create of cluster-scoped kind %q carries namespace %q", kind, namespace)
	}
	return connectionCreateTarget{Manifest: manifest, Entry: entry, Name: name, Namespace: namespace, YAML: body.YAML}, nil
}

// jsonStringOf — 매니페스트 스칼라 필드의 안전한 문자열 접근.
func jsonStringOf(v any) string {
	s, _ := v.(string)
	return s
}

// evaluateConnectionPolicy — §10.3 두 번째 인가 게이트(operations.go
// evaluateOperationPolicy의 커넥션-스코프 형태): ResourceKind는 매핑 표가
// 조달한다(공백이면 Evaluate가 fail-closed — VK-4). 평가 ERROR는 403
// fail-closed, deny는 403. 평가 버전은 감사 행의 canonical 원천.
func (a *InfraAPI) evaluateConnectionPolicy(c *gin.Context, info *auditInfo, def contract.OperationDefinition, entry contract.K8sCreateFaceEntry, contextRow model.ProviderContext, conn model.ProviderConnection) (policy.Decision, bool) {
	decision, err := policy.Evaluate(policy.PolicyInput{
		ProviderType: conn.ProviderType,
		ContextKind:  contextRow.Kind,
		ResourceKind: entry.ResourceKind,
		Risk:         def.RiskLevel,
		Mutating:     def.Mutating,
	})
	info.ProviderConnectionUID = conn.UID
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

// connectionResourceUID — §3.3 합성 resource_uid:
//
//	conn:<connectionUID>:<singular>:<target>
//
// singular은 매핑 표 kind의 소문자("DestinationRule" → "destinationrule"),
// target은 namespaced `<ns>/<name>` 또는 cluster-scope `<name>` 2형태다.
// 최악 길이(DNS-1123 상한 63+63·15 singular·32-hex 커넥션 분절)는 181자 —
// 255 예산 내. 초과 산출은 하드 거부한다(§5 #18 — sqlite 미강제 환경에서
// 길이 계약의 소유자).
func connectionResourceUID(connectionUID string, target connectionCreateTarget) (string, error) {
	uid := connectionScopedUIDPrefix + connectionUID + ":" + strings.ToLower(target.Entry.Kind) + ":" + connectionTargetOf(target)
	if len(uid) > syntheticResourceUIDBudget {
		return "", fmt.Errorf("synthetic resource_uid of %d chars exceeds the %d budget — refusing (§3.3 r3)", len(uid), syntheticResourceUIDBudget)
	}
	return uid, nil
}

// connectionTargetOf — 합성 uid의 target 분절: namespaced `<ns>/<name>`,
// cluster-scope `<name>`.
func connectionTargetOf(target connectionCreateTarget) string {
	if target.Namespace != "" {
		return target.Namespace + "/" + target.Name
	}
	return target.Name
}
