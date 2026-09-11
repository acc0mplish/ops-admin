// executor_resource.go — P2-C: high-risk resource mutation 2종(k8s.resource.apply
// update 한정·k8s.resource.delete)의 OperationExecutor leg(계획 r3 §J-P1-6 확정표
// — v1 UpdateK8sResourceYAML·DeleteK8sResource가 원천으로, phase6 H2에서 v1
// 경로와 함께 제거됐다).
// apply는 확정표 수정 1건대로 update 한정이다 — create는 uid-스코프 오퍼레이션
// 모델(SubmitInput.ResourceUID·관측 리프레시가 존재 자원의 uid를 요구)에 안 맞아
// 이월이다(I-P1·Phase H 블로커). handle·poll은 확정표 "발행+poll 1회"대로 state
// handle 가족(executor_config.go — P2-B가 "이후 apply·traffic이 승계"라고 적어둔
// 승계)이다:
//
//	apply  → state|<connUID>|<apiPath>|<base64url({resource:"manifest", …동결
//	         manifest})> — Poll은 GET 1회로 관측이 동결 manifest를 포함하는지
//	         판정한다(부분집합 상태 에코 — 임의 종이라 종별 디코드가 없다).
//	delete → state|<connUID>|<apiPath>|<base64url({resource:"delete"})> — Poll은
//	         GET 1회의 404가 성공 종단이다(provider_terminal_404 — P1-C 판단 6의
//	         client.go errNotFound 센티넬 재사용).
//
// 서빙 면(판단 기록 — §J-P1-6 확정표가 ResourceKinds를 못 박지 않아 v1 apply·
// delete face의 V2 URN 대응 전수로 확정했다):
//   - namespace·workload(deployment/statefulset/daemonset/replicaset/job/cronjob)·
//     service·ingress·configmap·secret·pv·pvc·gateway·httproute — v1
//     buildK8sYAMLResourcePath face 중 V2 URN(buildURN)이 name-주소 가능한 전부.
//     workload subtype은 v1에 없던 replicaset을 포함한다(URN이 이미 유일하게
//     주소 가능 — 서빙하지 않을 이유가 없다).
//   - node·pod 제외: buildURN이 이 둘을 uid 신원으로 적어 경로에 쓸 name이 URN에
//     없다(이름 확정에 클러스터 전수 LIST가 필요). node는 labels_update op가,
//     pod는 v1에서도 사실상 불변(friendlyK8sYAMLError의 pod immutable 특수문구)
//     이라 면에서 빠진다.
//   - istio 가족(virtualservice 등)·endpoints·storageclass 제외: istio는 P2-D가
//     virtualservice 앵커를 수집하며 재판정하고, endpoints·storageclass는 v1
//     face에 없다 — high-risk op의 면을 v1을 넘어 넓히지 않는다.
//   - gateway·httproute는 수집기의 버전 선호 시맨틱(J-P1-1 — v1 → 404 시
//     v1beta1)을 후보 경로 순회로 승계한다.
//
// payload 검증은 실행기 자체(frozenRestartedAt 선례 — §J-P1-6). 멱등 근거: apply는
// provider_frozen_manifest — 동일 payload 재실행은 동일 목표 manifest의 재 PUT이고
// (RV 주입만 현행 관측을 따라간다 — v1 와이어), delete는 provider_terminal_404 —
// 재실행 DELETE의 404는 실패가 아니라 이미 도달한 목표 상태의 확인이다.
package kubernetes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"ops-admin/backend/internal/infra/contract"

	ksyaml "sigs.k8s.io/yaml"
)

// operation names — compose_k8s_ops.go opdef 등록과 dispatch(executor.go)가
// 공유하는 단일 원천(§J-P1-6 — descriptors are code).
const (
	ApplyOperationName  = "k8s.resource.apply"
	DeleteOperationName = "k8s.resource.delete"
)

// --- URN 파싱 — name-주소 가능 종 면(buildURN §8.1 성분). ---

// resourceServedSingulars — 본 leg가 서빙하는 URN singular 면(파일 헤더 판단
// 기록). 오류 메시지가 이 면을 그대로 보고한다.
var resourceServedSingulars = []string{
	"namespace", "workload", "service", "ingress", "configmap", "secret",
	"pv", "pvc", "gateway", "httproute",
}

// resourceWorkloadSubtypes — workload URN 가운데 서빙하는 subtype(v1 face 5종 +
// replicaset — 파일 헤더).
var resourceWorkloadSubtypes = []string{
	"deployment", "statefulset", "daemonset", "replicaset", "job", "cronjob",
}

// resourceTarget — URN이 가리키는 단일 자원(이름 성분 — 경로 조립 입력).
type resourceTarget struct {
	Singular  string
	Subtype   string // workload만
	Namespace string // 클러스터 스코프(namespace·pv)는 공백
	Name      string
}

// parseResourceURN — urn:k8s:{ctxID}:{singular}:{tail}. tail은 singular별로
// <name>(namespace·pv)·<ns>/<subtype>/<name>(workload)·<ns>/<name>(그 외)이다.
// ctxID는 정의상 프로바이더 컨텍스트 식별자(숫자)지만 어댑터는 검증만 하고 사용하지
// 않는다 — 클러스터 주소는 Connection이 결정한다(arch rule 2).
func parseResourceURN(urn string) (resourceTarget, error) {
	rest, ok := strings.CutPrefix(urn, "urn:k8s:")
	if !ok {
		return resourceTarget{}, fmt.Errorf("kubernetes: resource URN %q is not a k8s URN (want urn:k8s:<ctx>:<singular>:<tail>)", urn)
	}
	parts := strings.Split(rest, ":")
	if len(parts) != 3 {
		return resourceTarget{}, fmt.Errorf("kubernetes: resource URN %q does not carry <ctx>:<singular>:<tail>", urn)
	}
	if _, err := strconv.ParseUint(parts[0], 10, 64); err != nil {
		return resourceTarget{}, fmt.Errorf("kubernetes: resource URN %q carries a non-numeric context id %q", urn, parts[0])
	}
	if !slices.Contains(resourceServedSingulars, parts[1]) {
		return resourceTarget{}, fmt.Errorf(
			"kubernetes: resource URN %q carries singular %q, which this leg does not serve (serves %s)",
			urn, parts[1], strings.Join(resourceServedSingulars, ", "))
	}
	tail := strings.Split(parts[2], "/")
	target := resourceTarget{Singular: parts[1]}
	switch target.Singular {
	case "namespace", "pv":
		if len(tail) != 1 || strings.TrimSpace(tail[0]) == "" {
			return resourceTarget{}, fmt.Errorf("kubernetes: %s URN tail %q must be <name>", target.Singular, parts[2])
		}
		target.Name = tail[0]
	case "workload":
		if len(tail) != 3 {
			return resourceTarget{}, fmt.Errorf("kubernetes: workload URN tail %q must be <namespace>/<subtype>/<name>", parts[2])
		}
		if !slices.Contains(resourceWorkloadSubtypes, tail[1]) {
			return resourceTarget{}, fmt.Errorf("kubernetes: workload URN subtype %q is not served (serves %s)", tail[1], strings.Join(resourceWorkloadSubtypes, ", "))
		}
		target.Namespace, target.Subtype, target.Name = tail[0], tail[1], tail[2]
	default:
		if len(tail) != 2 || strings.TrimSpace(tail[0]) == "" || strings.TrimSpace(tail[1]) == "" {
			return resourceTarget{}, fmt.Errorf("kubernetes: %s URN tail %q must be <namespace>/<name>", target.Singular, parts[2])
		}
		target.Namespace, target.Name = tail[0], tail[1]
	}
	return target, nil
}

// resourceCandidatePaths — 종별 이름공간 스코프 REST 경로 후보(v1
// buildK8sYAMLResourcePath의 V2 면). 단일 후보가 기본이고 gateway·httproute만
// v1 → v1beta1 순서의 2후보다(수집기 sectionFallbacks — J-P1-1 — 와 동일 선호).
func resourceCandidatePaths(t resourceTarget) []string {
	switch t.Singular {
	case "namespace":
		return []string{"/api/v1/namespaces/" + t.Name}
	case "pv":
		return []string{"/api/v1/persistentvolumes/" + t.Name}
	case "workload":
		if t.Subtype == "job" || t.Subtype == "cronjob" {
			return []string{"/apis/batch/v1/namespaces/" + t.Namespace + "/" + t.Subtype + "s/" + t.Name}
		}
		return []string{"/apis/apps/v1/namespaces/" + t.Namespace + "/" + t.Subtype + "s/" + t.Name}
	case "service":
		return []string{"/api/v1/namespaces/" + t.Namespace + "/services/" + t.Name}
	case "ingress":
		return []string{"/apis/networking.k8s.io/v1/namespaces/" + t.Namespace + "/ingresses/" + t.Name}
	case "configmap":
		return []string{"/api/v1/namespaces/" + t.Namespace + "/configmaps/" + t.Name}
	case "secret":
		return []string{"/api/v1/namespaces/" + t.Namespace + "/secrets/" + t.Name}
	case "pvc":
		return []string{"/api/v1/namespaces/" + t.Namespace + "/persistentvolumeclaims/" + t.Name}
	case "gateway":
		return gatewayCandidatePaths("gateways", t.Namespace, t.Name)
	case "httproute":
		return gatewayCandidatePaths("httproutes", t.Namespace, t.Name)
	}
	return nil
}

// gatewayCandidatePaths — GatewayAPI 2종의 버전 후보(v1 선호 → v1beta1 폴백).
func gatewayCandidatePaths(resource, namespace, name string) []string {
	return []string{
		"/apis/gateway.networking.k8s.io/v1/namespaces/" + namespace + "/" + resource + "/" + name,
		"/apis/gateway.networking.k8s.io/v1beta1/namespaces/" + namespace + "/" + resource + "/" + name,
	}
}

// expectedManifestKind — URN 목표가 기대하는 manifest kind(실행 전 신원 가드의
// 기준값).
func expectedManifestKind(t resourceTarget) string {
	switch t.Singular {
	case "namespace":
		return "Namespace"
	case "pv":
		return "PersistentVolume"
	case "workload":
		switch t.Subtype {
		case "deployment":
			return "Deployment"
		case "statefulset":
			return "StatefulSet"
		case "daemonset":
			return "DaemonSet"
		case "replicaset":
			return "ReplicaSet"
		case "job":
			return "Job"
		case "cronjob":
			return "CronJob"
		}
	case "service":
		return "Service"
	case "ingress":
		return "Ingress"
	case "configmap":
		return "ConfigMap"
	case "secret":
		return "Secret"
	case "pvc":
		return "PersistentVolumeClaim"
	case "gateway":
		return "Gateway"
	case "httproute":
		return "HTTPRoute"
	}
	return ""
}

// --- k8s.resource.apply (v1 원천 UpdateK8sResourceYAML — update 한정 — phase6 H2에서 제거). ---

// applyManifestOf — Payload["yaml"] 검증(v1 payload.YAML 키 승계 — resourceType·
// namespace·name·clusterId는 v1 payload에 있지만 URN이 목표를 결정하므로 V2
// 실행기가 소비하지 않는다): 공백 아닌 YAML 문자열 → JSON 객체 디코드(sigs.k8s.io/
// yaml — v1과 동일 변환·동일 "invalid yaml content" 문언). kind 필수는 v1
// parseK8sManifestIdentity 문언. 판단 기록 — v1은 manifest·경로의 신원 불일치를
// API 서버 거부에 맡겼지만, uid 스코프인 V2에서 cross-resource apply는 high-risk
// op의 오발사라 실행 전에 거부한다: kind는 URN singular 기대 kind와 대소문자
// 무시 일치, metadata.name은 URN name과 일치, namespaced 종의 manifest namespace는
// URN namespace와 일치(생략은 허용 — 서버가 경로에서 채운다), 클러스터 스코프
// 목표의 manifest namespace는 비어 있어야 한다.
func applyManifestOf(payload contract.JSONMap, t resourceTarget) (map[string]any, error) {
	var spec struct {
		YAML string `json:"yaml"`
	}
	if err := payloadDecode(payload, &spec); err != nil {
		return nil, err
	}
	if strings.TrimSpace(spec.YAML) == "" {
		return nil, errors.New(`kubernetes: payload carries no "yaml" — the plan request must freeze the resource manifest (v1 "invalid yaml payload")`)
	}
	raw, err := ksyaml.YAMLToJSON([]byte(spec.YAML))
	if err != nil {
		return nil, errors.New("kubernetes: invalid yaml content")
	}
	var manifest map[string]any
	if err := json.Unmarshal(raw, &manifest); err != nil || manifest == nil {
		return nil, errors.New("kubernetes: invalid yaml content")
	}
	kind, _ := manifest["kind"].(string)
	if strings.TrimSpace(kind) == "" {
		return nil, errors.New("kubernetes: resource kind is required")
	}
	if want := expectedManifestKind(t); want == "" || !strings.EqualFold(kind, want) {
		return nil, fmt.Errorf("kubernetes: manifest kind %q does not match the %s target — refusing a cross-kind apply", kind, want)
	}
	metadata, _ := manifest["metadata"].(map[string]any)
	name, _ := metadata["name"].(string)
	if strings.TrimSpace(name) != t.Name {
		return nil, fmt.Errorf("kubernetes: manifest metadata.name %q does not match the target name %q", name, t.Name)
	}
	namespace, _ := metadata["namespace"].(string)
	if t.Namespace == "" {
		if strings.TrimSpace(namespace) != "" {
			return nil, fmt.Errorf("kubernetes: target %q is cluster-scoped but the manifest carries namespace %q", t.Name, namespace)
		}
		return manifest, nil
	}
	if trimmed := strings.TrimSpace(namespace); trimmed != "" && trimmed != t.Namespace {
		return nil, fmt.Errorf("kubernetes: manifest namespace %q does not match the target namespace %q", namespace, t.Namespace)
	}
	return manifest, nil
}

// manifestForPut — PUT 본문: 동결 manifest의 복사에 현행 resourceVersion을
// 주입한다(v1 :41-50 와이어 — Kubernetes PUT은 metadata.resourceVersion을 요구하고
// 폼 편집기는 간결한 manifest를 내므로 서버측 관측으로 채운다). 주입이 복사본의
// metadata에서 일어나므로 expectation이 동결한 manifest는 버전 무관으로
// byte-identical이 유지된다(provider_frozen_manifest의 전송 근거). 관측에 버전이
// 없으면 v1과 동일하게 주입하지 않는다 — API 서버의 거부가 그대로 에러가 된다.
func manifestForPut(manifest map[string]any, resourceVersion string) map[string]any {
	put := make(map[string]any, len(manifest))
	for key, value := range manifest {
		put[key] = value
	}
	metadata, _ := put["metadata"].(map[string]any)
	metaCopy := make(map[string]any, len(metadata)+1)
	for key, value := range metadata {
		metaCopy[key] = value
	}
	if resourceVersion != "" {
		metaCopy["resourceVersion"] = resourceVersion
	}
	put["metadata"] = metaCopy
	return put
}

// currentResourceVersion — 현행 관측의 metadata.resourceVersion 취출.
func currentResourceVersion(current map[string]any) string {
	metadata, _ := current["metadata"].(map[string]any)
	version, _ := metadata["resourceVersion"].(string)
	return version
}

// executeResourceApply — GET(후보 경로 순회 — 발견 경로 확정) → RV 주입 → PUT 1회.
// v1도 후보 경로마다 GET을 시도해 발견된 경로에 PUT한다(404는 다음 후보, 그 외
// 오류는 중단 — GatewayAPI 버전 폴백의 원형 와이어). handle은 state 가족 형태로
// 동결 manifest를 자기서술한다. 재실행은 동일 목표 manifest의 재 PUT이다 — 실제
// 변경이 없으면 서버가 무증가로 판정한다(provider_frozen_manifest).
func (a *Adapter) executeResourceApply(ctx context.Context, req contract.OperationRequest) (contract.OperationHandle, error) {
	target, err := parseResourceURN(req.ResourceURN)
	if err != nil {
		return contract.OperationHandle{}, err
	}
	manifest, err := applyManifestOf(req.Payload, target)
	if err != nil {
		return contract.OperationHandle{}, err
	}
	client, err := a.executionClient(req)
	if err != nil {
		return contract.OperationHandle{}, err
	}

	apiPath := ""
	resourceVersion := ""
	for _, candidate := range resourceCandidatePaths(target) {
		var current map[string]any
		getErr := client.getJSONOp(ctx, candidate, nil, "execute", &current)
		if errors.Is(getErr, errNotFound) {
			continue
		}
		if getErr != nil {
			return contract.OperationHandle{}, getErr
		}
		apiPath = candidate
		resourceVersion = currentResourceVersion(current)
		break
	}
	if apiPath == "" {
		return contract.OperationHandle{}, fmt.Errorf("kubernetes: apply target %q was not found on any served api version", req.ResourceURN)
	}
	if err := client.doJSON(ctx, http.MethodPut, apiPath, nil, "application/json", manifestForPut(manifest, resourceVersion), "execute", nil); err != nil {
		return contract.OperationHandle{}, err
	}
	ref, err := encodeStateRef(req.Connection.UID, apiPath, stateExpectation{Resource: "manifest", Manifest: manifest})
	if err != nil {
		return contract.OperationHandle{}, err
	}
	return contract.OperationHandle{ProviderRef: ref}, nil
}

// --- k8s.resource.delete (v1 원천 DeleteK8sResource — phase6 H2에서 제거). ---

// executeResourceDelete — DELETE 1회(v1 와이어 — nil body·application/json).
// provider_terminal_404: 목표 상태는 "부재"다. 시도가 404를 만나도 그것은 이미
// 도달한 목표 상태의 확인이다(크래시 → 리퍼 재실행 경로 — v1의 동기 "resource not
// found" 오류와 여기가 다르다: v1은 1회 왕복 계약, V2는 폴 종결 계약) — handle을
// 반환해 폴이 종단을 판정하게 한다. GatewayAPI 2종의 미발견은 다음 버전 후보
// DELETE로 이어진다(404 = 이 버전에서 부재 — 수집기 폴백과 동일 선호).
func (a *Adapter) executeResourceDelete(ctx context.Context, req contract.OperationRequest) (contract.OperationHandle, error) {
	target, err := parseResourceURN(req.ResourceURN)
	if err != nil {
		return contract.OperationHandle{}, err
	}
	client, err := a.executionClient(req)
	if err != nil {
		return contract.OperationHandle{}, err
	}

	candidates := resourceCandidatePaths(target)
	apiPath := candidates[0]
	for _, candidate := range candidates {
		delErr := client.doJSON(ctx, http.MethodDelete, candidate, nil, "application/json", nil, "execute", nil)
		if delErr == nil {
			apiPath = candidate
			break
		}
		if errors.Is(delErr, errNotFound) {
			continue
		}
		return contract.OperationHandle{}, delErr
	}
	ref, err := encodeStateRef(req.Connection.UID, apiPath, stateExpectation{Resource: "delete"})
	if err != nil {
		return contract.OperationHandle{}, err
	}
	return contract.OperationHandle{ProviderRef: ref}, nil
}

// --- Poll — resource 2종의 폴 leg(pollStateRef — executor_config.go — 에서
// expectation resource로 분기되어 진입한다). ---

// pollManifestRef — apply의 폴 leg: GET 1회 → 동결 manifest 부분집합 판정. 임의
// 종이 대상이라 stateObservation 같은 종별 디코드가 없다 — 원시 맵으로 받아
// manifestSubsumes로 판정한다. Succeeded의 detail은 stateResultRedaction(
// compose_k8s_ops.go)과 1:1인 2키다 — generation이 없는 종(configmap·secret 등)은
// 부재 관측값 0(restartedAt 공백 선례).
func (a *Adapter) pollManifestRef(ctx context.Context, client *k8sClient, handle stateHandle) (contract.OperationStatus, error) {
	var obj map[string]any
	if err := client.getJSONOp(ctx, handle.APIPath, nil, "poll", &obj); err != nil {
		return contract.OperationStatus{}, err
	}
	if !manifestSubsumes(handle.Expectation.Manifest, obj) {
		return contract.OperationStatus{State: contract.OperationStateRunning}, nil
	}
	return contract.OperationStatus{
		State: contract.OperationStateSucceeded,
		Detail: contract.JSONMap{
			"generation": observedGeneration(obj),
			"serverURL":  client.rt.Server,
		},
	}, nil
}

// observedGeneration — 임의 종 관측의 metadata.generation 취출. generation이
// 없는 종은 0이다.
func observedGeneration(obj map[string]any) int64 {
	metadata, _ := obj["metadata"].(map[string]any)
	if value, ok := metadata["generation"].(float64); ok {
		return int64(value)
	}
	return 0
}

// pollAbsentRef — delete의 폴 leg: GET 1회 → 404 = Succeeded(client.go
// errNotFound 센티넬 — P1-C 판단 6 재사용). 존재는 Running(다음 폴), 그 외 오류는
// 전파다. Succeeded의 detail은 deleteResultRedaction(compose_k8s_ops.go)과 1:1인
// 1키다 — 부재 판정에는 관측 성분이 없고 serverURL(어느 클러스터에서 수렴했는가)
// 만 남는다.
func (a *Adapter) pollAbsentRef(ctx context.Context, client *k8sClient, handle stateHandle) (contract.OperationStatus, error) {
	var obj map[string]any
	getErr := client.getJSONOp(ctx, handle.APIPath, nil, "poll", &obj)
	if errors.Is(getErr, errNotFound) {
		return contract.OperationStatus{
			State:  contract.OperationStateSucceeded,
			Detail: contract.JSONMap{"serverURL": client.rt.Server},
		}, nil
	}
	if getErr != nil {
		return contract.OperationStatus{}, getErr
	}
	return contract.OperationStatus{State: contract.OperationStateRunning}, nil
}

// manifestSubsumes — 동결 manifest ⊆ 관측 판정(상태 에코의 임의 종 형태):
//   - 객체: expectation 키 전부가 관측에 있고 값이 valueSubsumes면 참. 관측이 서버
//     기본값·컨트롤러 필드를 더 갖는 것은 무관하다(단방향 부분집합).
//   - null expectation은 제약 없음(서버 정규화가 떨어뜨리는 "unset" 표기 — 제약을
//     걸지 않는다).
//   - 배열: expectation 원소 전부가 관측의 어느 원소로 충족되면 참(순서 무관
//     초집합 — 서버 기본값 주입·항목 추가를 무시하고 기대 항목의 존재만 판정).
//   - 스칼라: canonical JSON 동치(양측 모두 JSON 디코드 산물이라 표현이 정규화된다).
func manifestSubsumes(want, got map[string]any) bool {
	for key, expected := range want {
		if expected == nil {
			continue
		}
		observed, ok := got[key]
		if !ok || !valueSubsumes(expected, observed) {
			return false
		}
	}
	return true
}

// valueSubsumes — 부분집합 판정의 값 재귀(객체 → 재귀, 배열 → 초집합 매칭,
// 그 외 → 동치).
func valueSubsumes(expected, observed any) bool {
	if expectedMap, ok := expected.(map[string]any); ok {
		observedMap, ok := observed.(map[string]any)
		return ok && manifestSubsumes(expectedMap, observedMap)
	}
	if expectedList, ok := expected.([]any); ok {
		observedList, ok := observed.([]any)
		if !ok || len(observedList) < len(expectedList) {
			return false
		}
		for _, want := range expectedList {
			matched := false
			for _, got := range observedList {
				if valueSubsumes(want, got) {
					matched = true
					break
				}
			}
			if !matched {
				return false
			}
		}
		return true
	}
	return jsonEqual(expected, observed)
}

// jsonEqual — 두 값의 canonical JSON 동치(맵 키 정렬은 json.Marshal이 보장).
func jsonEqual(expected, observed any) bool {
	rawExpected, errExpected := json.Marshal(expected)
	rawObserved, errObserved := json.Marshal(observed)
	return errExpected == nil && errObserved == nil && string(rawExpected) == string(rawObserved)
}
