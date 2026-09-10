// executor.go — Phase 3 B(N1): k8s workload restart leg + operation dispatch
// (§14.3 비동기 듀얼 모드). P2-A(계획 r3 §J-P1-6)에서 dispatch가 operation-name
// switch로 일반화되어 workload mutation 3종(executor_workload.go)과 4종이
// rollout handle·Poll을 공유한다. 계약(J1/J2/J12 — 계획 r2 §3.2):
//
//   - Execute: 서버가 동결한 restartedAt(Payload — RFC3339 문자열)를 pod template
//     어노테이션으로 strategic-merge-patch 1회 발행. 동일 payload 재실행(크래시 →
//     리퍼 재큐 → attempt 2)은 byte-identical patch → k8s가 spec 무변화로 판정 →
//     generation 무증가 → 재롤아웃(이중 restart) 없음.
//   - Poll: GET 1회 → 종별 완료 판정식(r2 교정판) → Succeeded{detail} | Running.
//     폴 자격은 엔진이 매 폴 조립해 PollRequest.Connection으로 주입한다(J12) —
//     어댑터는 자격을 스스로 획득하지 않는다(보존 제약 #10).
//
// 이 어댑터의 실행 경로는 stateless다("HTTP clients are per-request") — handle에
// 필요한 상태 전부를 ProviderRef가 자기서술한다.
package kubernetes

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"ops-admin/backend/internal/infra/contract"
)

// RestartOperationName — 본 executor가 수용하는 유일 오퍼레이션(§3.2 "유일 수용").
const RestartOperationName = "k8s.workload.restart"

// RestartAnnotationKey — v1 RestartK8sWorkload와 동일한 롤아웃 트리거 어노테이션
// (§0.1 — pod template에 이 값을 strategic-merge-patch하면 롤아웃이 유발된다.
// v1 레인은 무변경이며 이 키의 재사용이 두 레인을 병립시킨다).
const RestartAnnotationKey = "kubectl.kubernetes.io/restartedAt"

// rolloutHandleMarker — ProviderRef 형식의 첫 세그먼트:
//
//	rollout|<connUID>|<ns>|<kind>|<name>|<expectGeneration>
//
// handle은 task_attempt.handle_ref에 지속되는 값 — 어느 커넥션을 향하는지
// 스스로 서술해야 한다(감사·디버깅·오남용 추적, J12). connUID는 공개 식별자다
// (비밀 아님 — 자격 물질의 인코딩은 보존 제약 #7/#10이 금지).
const rolloutHandleMarker = "rollout"

// Compile-time interface conformance — registry V5가 apply capability 선언에
// 요구하는 OperationExecutor(§3.7 매핑 표)와 §14.3 듀얼 모드의 TaskPoller.
var (
	_ contract.OperationExecutor = (*Adapter)(nil)
	_ contract.TaskPoller        = (*Adapter)(nil)
)

// --- 파싱 — URN(§8.1 형상)과 ProviderRef(J2 인코딩). ---

// workloadTarget — URN/ProviderRef가 가리키는 단일 워크로드.
type workloadTarget struct {
	Namespace string
	Kind      string // deployment | statefulset | daemonset
	Name      string
}

// parseWorkloadTarget validates the target triple shared by the URN tail and
// the rollout handle.
func parseWorkloadTarget(namespace, kind, name string) (workloadTarget, error) {
	if namespace == "" || name == "" {
		return workloadTarget{}, fmt.Errorf("kubernetes: workload target needs a non-empty namespace and name (got namespace=%q name=%q)", namespace, name)
	}
	switch kind {
	case "deployment", "statefulset", "daemonset":
		return workloadTarget{Namespace: namespace, Kind: kind, Name: name}, nil
	}
	return workloadTarget{}, fmt.Errorf("kubernetes: unsupported workload kind %q (want deployment, statefulset or daemonset)", kind)
}

// parseWorkloadURN — urn:k8s:{ctxID}:workload:{namespace}/{kind}/{name}.
// ctxID는 정의상 프로바이더 컨텍스트 식별자(숫자)지만 어댑터는 검증만 하고
// 사용하지 않는다 — 클러스터 주소는 Connection이 결정한다(arch rule 2).
func parseWorkloadURN(urn string) (workloadTarget, error) {
	rest, ok := strings.CutPrefix(urn, "urn:k8s:")
	if !ok {
		return workloadTarget{}, fmt.Errorf("kubernetes: resource URN %q is not a k8s URN (want urn:k8s:<ctx>:workload:<ns>/<kind>/<name>)", urn)
	}
	parts := strings.Split(rest, ":")
	if len(parts) != 3 || parts[1] != "workload" {
		return workloadTarget{}, fmt.Errorf("kubernetes: resource URN %q is not a workload URN — this executor serves deployments, statefulsets and daemonsets only", urn)
	}
	if _, err := strconv.ParseUint(parts[0], 10, 64); err != nil {
		return workloadTarget{}, fmt.Errorf("kubernetes: resource URN %q carries a non-numeric context id %q", urn, parts[0])
	}
	tail := strings.Split(parts[2], "/")
	if len(tail) != 3 {
		return workloadTarget{}, fmt.Errorf("kubernetes: workload URN tail %q must be <namespace>/<kind>/<name>", parts[2])
	}
	return parseWorkloadTarget(tail[0], tail[1], tail[2])
}

// apiPath — 종별 이름공간 스코프 REST 경로.
func (t workloadTarget) apiPath() string {
	return "/apis/apps/v1/namespaces/" + t.Namespace + "/" + t.Kind + "s/" + t.Name
}

// rolloutHandle — decodeRolloutRef의 결과.
type rolloutHandle struct {
	ConnectionUID    string
	Target           workloadTarget
	ExpectGeneration int64
}

func encodeRolloutRef(connUID string, target workloadTarget, expectGeneration int64) string {
	return strings.Join([]string{
		rolloutHandleMarker, connUID, target.Namespace, target.Kind, target.Name,
		strconv.FormatInt(expectGeneration, 10),
	}, "|")
}

func decodeRolloutRef(ref string) (rolloutHandle, error) {
	parts := strings.Split(ref, "|")
	if len(parts) != 6 {
		return rolloutHandle{}, fmt.Errorf("kubernetes: malformed rollout handle %q (want rollout|<connUID>|<ns>|<kind>|<name>|<expectGeneration>)", ref)
	}
	if parts[0] != rolloutHandleMarker {
		return rolloutHandle{}, fmt.Errorf("kubernetes: handle %q is not a rollout handle", ref)
	}
	if parts[1] == "" {
		return rolloutHandle{}, fmt.Errorf("kubernetes: rollout handle %q carries no connection uid", ref)
	}
	generation, err := strconv.ParseInt(parts[5], 10, 64)
	if err != nil || generation <= 0 {
		return rolloutHandle{}, fmt.Errorf("kubernetes: rollout handle %q carries an invalid expectGeneration %q", ref, parts[5])
	}
	target, err := parseWorkloadTarget(parts[2], parts[3], parts[4])
	if err != nil {
		return rolloutHandle{}, err
	}
	return rolloutHandle{ConnectionUID: parts[1], Target: target, ExpectGeneration: generation}, nil
}

// --- 자격 — 실행 경로 Material 키는 "operations"(§7.4 — Discover의 inventory와 대칭). ---

// resolveOperationsRuntime parses the execution kubeconfig out of the
// connection view. 부재는 즉시 에러(§3.2 — 엔진의 credential_error 분기와
// 정합하는 메시지). 에러 문구에 kubeconfig 평문이 실리는 일은 없다(보존 제약 #7).
func resolveOperationsRuntime(conn contract.ConnectionView) (clusterRuntime, error) {
	kubeconfig := ""
	if conn.Material != nil {
		kubeconfig = strings.TrimSpace(conn.Material[contract.CredentialPurposeOperations])
	}
	if kubeconfig == "" {
		return clusterRuntime{}, fmt.Errorf(
			"kubernetes: missing credential material Material[%q] (broker purpose %q resolve — engine classifies this path as credential_error)",
			contract.CredentialPurposeOperations, contract.CredentialPurposeOperations)
	}
	return parseKubeConfig(kubeconfig)
}

func (a *Adapter) buildExecutorClient(conn contract.ConnectionView) (*k8sClient, error) {
	rt, err := resolveOperationsRuntime(conn)
	if err != nil {
		return nil, err
	}
	dialer, err := a.dialContextFor(conn)
	if err != nil {
		return nil, err
	}
	return newK8sClient(rt, dialer, a.metrics)
}

// --- Execute (J1/J2). ---

// restartPatchBody — strategic-merge-patch 본문. 구조체 → json.Marshal은 필드
// 순서가 선언 순으로 고정되고 어노테이션은 키 1개 — 재실행이 byte-identical
// 이다(J1의 전송 계약, httptest가 단얫).
type restartPatchBody struct {
	Spec struct {
		Template struct {
			Metadata struct {
				Annotations map[string]string `json:"annotations"`
			} `json:"metadata"`
		} `json:"template"`
	} `json:"spec"`
}

func newRestartPatch(restartedAt string) restartPatchBody {
	var p restartPatchBody
	p.Spec.Template.Metadata.Annotations = map[string]string{RestartAnnotationKey: restartedAt}
	return p
}

// frozenRestartedAt — Payload["restartedAt"] 검증(J1): 값은 plan 응답이 동결한
// RFC3339 문자열이며 execute가 그대로 전달한다(재수집 금지 — 클라이언트 임의값
// 불식, 존재·형식만 검증하고 값은 무수정 패스스루).
// Payload["resourceRevision"]은 감사용 선택 키(§3.2) — 본 executor는 소비하지
// 않고 거부하지도 않는다.
func frozenRestartedAt(payload contract.JSONMap) (string, error) {
	raw, ok := payload["restartedAt"]
	if !ok {
		return "", fmt.Errorf("kubernetes: payload carries no %q — the plan response must freeze the restart timestamp (J1)", "restartedAt")
	}
	value, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("kubernetes: payload %q must be an RFC3339 string, got %T", "restartedAt", raw)
	}
	if _, err := time.Parse(time.RFC3339, value); err != nil {
		return "", fmt.Errorf("kubernetes: payload %q is not RFC3339: %w", "restartedAt", err)
	}
	return value, nil
}

// servedOperations — dispatch가 수용하는 operation name set(오류 메시지용 —
// P2-A에서 3종 확장, §J-P1-6 확정표).
var servedOperations = []string{
	RestartOperationName,
	ScaleOperationName,
	ImageUpdateOperationName,
	ResourcesUpdateOperationName,
}

// Execute — operation-name dispatch(§J-P1-6: restart 1종 검사문의 일반화).
// 각 leg는 동일 검증 순서(URN → payload → UID·client — executionClient)를
// 유지하고, 4종 모두 rollout handle·Poll을 공유한다.
func (a *Adapter) Execute(ctx context.Context, req contract.OperationRequest) (contract.OperationHandle, error) {
	switch req.OperationName {
	case RestartOperationName:
		return a.executeRestart(ctx, req)
	case ScaleOperationName:
		return a.executeScale(ctx, req)
	case ImageUpdateOperationName:
		return a.executeImageUpdate(ctx, req)
	case ResourcesUpdateOperationName:
		return a.executeResourcesUpdate(ctx, req)
	}
	return contract.OperationHandle{}, fmt.Errorf("kubernetes: operation %q is not served by this executor (serves %s)", req.OperationName, strings.Join(servedOperations, ", "))
}

// executeRestart — restart leg(J1/J2): 검증(왕복 0회) → PATCH 1회 → ProviderRef
// 반환. expectGeneration은 patch 응답의 metadata.generation이다(J2): 이 시점
// 롤아웃은 관측 전이므로 Succeeded가 아니라 항상 handle(비동기)을 반환한다.
func (a *Adapter) executeRestart(ctx context.Context, req contract.OperationRequest) (contract.OperationHandle, error) {
	target, err := parseWorkloadURN(req.ResourceURN)
	if err != nil {
		return contract.OperationHandle{}, err
	}
	restartedAt, err := frozenRestartedAt(req.Payload)
	if err != nil {
		return contract.OperationHandle{}, err
	}
	client, err := a.executionClient(req)
	if err != nil {
		return contract.OperationHandle{}, err
	}

	var resp struct {
		Metadata struct {
			Generation int64 `json:"generation"`
		} `json:"metadata"`
	}
	if err := client.patchJSON(ctx, target.apiPath(), newRestartPatch(restartedAt), "execute", &resp); err != nil {
		return contract.OperationHandle{}, err
	}
	if resp.Metadata.Generation <= 0 {
		return contract.OperationHandle{}, fmt.Errorf("kubernetes: patch response for %s carried no metadata.generation — cannot encode the rollout handle", target.apiPath())
	}
	return contract.OperationHandle{
		ProviderRef: encodeRolloutRef(req.Connection.UID, target, resp.Metadata.Generation),
	}, nil
}

// --- Poll (J2 r2 교정 판정식). ---

// workloadRolloutStatus — 판정식 3종과 성공 detail에 필요한 관측 전체. 단일
// 디코드 형태: 어느 종의 응답에도 없는 필드는 영값으로 디코드된다.
type workloadRolloutStatus struct {
	Metadata struct {
		Generation int64 `json:"generation"`
	} `json:"metadata"`
	Spec struct {
		Replicas *int `json:"replicas"`
		Template struct {
			Metadata struct {
				Annotations map[string]string `json:"annotations"`
			} `json:"metadata"`
		} `json:"template"`
	} `json:"spec"`
	Status struct {
		ObservedGeneration     int64  `json:"observedGeneration"`
		ReadyReplicas          int    `json:"readyReplicas"`
		AvailableReplicas      int    `json:"availableReplicas"`
		UpdatedReplicas        int    `json:"updatedReplicas"`
		CurrentRevision        string `json:"currentRevision"`
		UpdateRevision         string `json:"updateRevision"`
		DesiredNumberScheduled int    `json:"desiredNumberScheduled"`
		UpdatedNumberScheduled int    `json:"updatedNumberScheduled"`
		NumberReady            int    `json:"numberReady"`
	} `json:"status"`
}

// desiredReplicas — spec.replicas 생략은 k8s 기본값 1(deploy/sts).
func (o *workloadRolloutStatus) desiredReplicas() int {
	if o.Spec.Replicas != nil {
		return *o.Spec.Replicas
	}
	return 1
}

// rolloutConverged — 종별 완료 판정식(J2 r2 교정판):
//
//	deploy: observedGeneration >= expect && availableReplicas == spec.replicas
//	        && updatedReplicas == spec.replicas
//	sts:    updateRevision == currentRevision && readyReplicas == spec.replicas
//	ds:     updatedNumberScheduled == desiredNumberScheduled
//
// available/ready만으로는 구 세대 pod 병존 중에도 기대값을 만족할 수 있어
// (렌즈2 반증 — 구 RS pod가 카운트에 포함) 세대 수렴 조건이 포함된다.
// deploy의 observedGeneration >= expect는 이후 추가 mutation으로 generation이
// expect를 넘어서도 참(롤아웃은 수렴했으므로) — 계획의 >= 문언 그대로.
func rolloutConverged(kind string, expect int64, o *workloadRolloutStatus) bool {
	switch kind {
	case "deployment":
		want := o.desiredReplicas()
		return o.Status.ObservedGeneration >= expect &&
			o.Status.AvailableReplicas == want &&
			o.Status.UpdatedReplicas == want
	case "statefulset":
		want := o.desiredReplicas()
		return o.Status.UpdateRevision == o.Status.CurrentRevision &&
			o.Status.ReadyReplicas == want
	case "daemonset":
		return o.Status.UpdatedNumberScheduled == o.Status.DesiredNumberScheduled
	}
	return false
}

// readyCount/updatedCount — 종별 관측을 detail의 공통 키로 정규화.
func readyCount(kind string, o *workloadRolloutStatus) int {
	if kind == "daemonset" {
		return o.Status.NumberReady
	}
	return o.Status.ReadyReplicas
}

func updatedCount(kind string, o *workloadRolloutStatus) int {
	if kind == "daemonset" {
		return o.Status.UpdatedNumberScheduled
	}
	return o.Status.UpdatedReplicas
}

// Poll — GET 1회 → 판정식. Succeeded의 detail은 restart 결과 redaction 허용
// 필드와 1:1다(generation·restartedAt·readyReplicas·updated·serverURL — §10.2).
// restartedAt은 관측값(현행 pod template) — 게이트 ② 단얫 3(동결값과의 일치)의
// 증거 재료. serverURL은 Connection에서 파싱한 API 서버 주소(VK-3 — 공개 값,
// kind는 loopback — 오발사 추적 증거). 판정 실패는 Running(다음 폴)이며 종단은
// 리스/재시도 메커니즘이 지킨다(§3.6a).
func (a *Adapter) Poll(ctx context.Context, req contract.PollRequest) (contract.OperationStatus, error) {
	handle, err := decodeRolloutRef(req.Handle.ProviderRef)
	if err != nil {
		return contract.OperationStatus{}, err
	}
	// J12 정합 가드: 엔진이 매 폴 조립하는 Connection이 handle이 자기서술하는
	// 커넥션과 다르면 조립 버그다 — 다른 클러스터를 폴하는 오발사(R13 계열)를
	// 늦은 수렴 오탐보다 빨리 잡는다.
	if req.Connection.UID != handle.ConnectionUID {
		return contract.OperationStatus{}, fmt.Errorf(
			"kubernetes: poll connection UID %q does not match the rollout handle's connection %q (assembly bug — J12)",
			req.Connection.UID, handle.ConnectionUID)
	}
	client, err := a.buildExecutorClient(req.Connection)
	if err != nil {
		return contract.OperationStatus{}, err
	}

	var obj workloadRolloutStatus
	if err := client.getJSONOp(ctx, handle.Target.apiPath(), nil, "poll", &obj); err != nil {
		return contract.OperationStatus{}, err
	}
	if !rolloutConverged(handle.Target.Kind, handle.ExpectGeneration, &obj) {
		return contract.OperationStatus{State: contract.OperationStateRunning}, nil
	}
	return contract.OperationStatus{
		State: contract.OperationStateSucceeded,
		Detail: contract.JSONMap{
			"generation":    obj.Metadata.Generation,
			"restartedAt":   obj.Spec.Template.Metadata.Annotations[RestartAnnotationKey],
			"readyReplicas": readyCount(handle.Target.Kind, &obj),
			"updated":       updatedCount(handle.Target.Kind, &obj),
			"serverURL":     client.rt.Server,
		},
	}, nil
}
