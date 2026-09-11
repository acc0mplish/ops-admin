// executor_traffic.go — P2-D: traffic mutation 2종(k8s.istio.traffic_update·
// k8s.httproute.traffic_update)의 OperationExecutor leg(계획 r3 §J-P1-6 확정표 —
// v1 UpdateK8sIstioTraffic·UpdateK8sHTTPRouteTraffic이 원천으로, phase6 H2에서
// v1 경로와 함께 제거됐다). 두 v1 함수는 검증 순서·문언만 다르고 와이어가 동일 형태라(배열 원소 가중치
// splice PUT) 선언형 종 스펙 2건 + 단일 실행 코어로 착지한다 — v1의 2중 복제를
// 그대로 이월하지 않은 구조 판단 기록. handle·poll은 확정표 "발행+poll 1회"대로
// state handle 가족(executor_state.go — 동결 엔트리 위치·가중치의 에코 판정)이다.
//
// 검증 순서는 v1과 1:1이다: payload 존재(→ "invalid … traffic payload") → GET
// (후보 경로 순회 — v1 buildIstioResourcePathsWithPreferred·
// buildGatewayAPIResourcePathsWithPreferred 기본 순서 v1 선호, 404만 다음 후보) →
// 첫 조정 가능 엔트리 부재(→ "has no adjustable …") → 경로 수 불일치(→ "… changed,
// please refresh and try again") → 가중치 음수(→ "greater than or equal to 0") →
// 합계 100(→ "must total 100") → PUT 1회.
//
// v1 대비 2건의 의도적 편차(판단 기록 — 둘 다 safety-positive다):
//  1. PUT 본문은 GET 관측의 전수 복사에 가중치만 반영한다. v1은 kubeIstioVirtualService
//     ·kubeHTTPRoute typed 구조체 왕복이라 관측에만 있는 spec·metadata 필드를
//     (구조체에 없어) 탈락시킨다 — 폼 편집기 와이어의 정보 손실이다. V2는
//     buildServicePatchBody(wholesale 현행 복사) 선례로 손실을 없앤다.
//  2. GET 관측의 metadata.resourceVersion을 PUT 본문에 그대로 실어 낙관 동시성을
//     켠다. v1의 kubeMetadata에는 resourceVersion이 없어(실측 k8s_types.go:115)
//     무조건 덮어쓰기였다 — 동시 가중치 변경을 소리 없이 유기한다. 재실행은 매번
//     fresh GET의 RV를 따라가므로(provider_frozen_payload) 충돌 재현이 없다.
//
// payload 검증은 실행기 자체(frozenRestartedAt 선례 — §J-P1-6). 멱등 근거
// (provider_frozen_payload): 재실행은 동일 가중치 배열의 재 PUT — 실변경이 없으면
// 서버가 무증가로 판정한다(테스트가 실증).
package kubernetes

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"ops-admin/backend/internal/infra/contract"
)

// operation names — compose_k8s_ops.go opdef 등록과 dispatch(executor.go)가
// 공유하는 단일 원천(§J-P1-6 — descriptors are code).
const (
	IstioTrafficUpdateOperationName     = "k8s.istio.traffic_update"
	HTTPRouteTrafficUpdateOperationName = "k8s.httproute.traffic_update"
)

// --- 종 스펙 — 두 opdef가 선언하는 면(v1 두 함수의 차이점 전부). ---

// trafficKindSpec — istio VirtualService와 GatewayAPI HTTPRoute의 차이만 담는
// 선언형 스펙. 메시지 문언은 v1 원문(kubernetes: 접두만 부착 — node label 선례)이다.
type trafficKindSpec struct {
	singular   string // URN 성분(buildURN §8.1 기타 네임스페이스 소속)
	resource   string // 경로 plural
	candidates func(resource, namespace, name string) []string
	entryField string // spec.http | spec.rules
	routeField string // entry.route | entry.backendRefs
	// stateExpectation.Resource — handle 판별자(executor_state.go satisfiedBy).
	expectationResource string
	invalidPayloadMsg   string
	noAdjustableMsg     string
	countChangedMsg     string
}

var istioTrafficSpec = trafficKindSpec{
	singular:            "virtualservice",
	resource:            "virtualservices",
	candidates:          istioCandidatePaths,
	entryField:          "http",
	routeField:          "route",
	expectationResource: "traffic_istio",
	invalidPayloadMsg:   "kubernetes: invalid istio traffic payload",
	noAdjustableMsg:     "kubernetes: virtualservice has no adjustable HTTP routes",
	countChangedMsg:     "kubernetes: virtualservice route count changed, please refresh and try again",
}

var httpRouteTrafficSpec = trafficKindSpec{
	singular:            "httproute",
	resource:            "httproutes",
	candidates:          gatewayCandidatePaths,
	entryField:          "rules",
	routeField:          "backendRefs",
	expectationResource: "traffic_httproute",
	invalidPayloadMsg:   "kubernetes: invalid http route traffic payload",
	noAdjustableMsg:     "kubernetes: httproute has no adjustable backend refs",
	countChangedMsg:     "kubernetes: httproute backend refs changed, please refresh and try again",
}

// istioCandidatePaths — istio 2종의 버전 후보(v1 buildIstioResourcePathsWithPreferred
// 기본 순서 — v1 선호, 404 시 v1beta1). PUT은 발견 경로에 1회다(v1이 관측
// apiVersion으로 PUT 순서를 재정렬해도 착지 경로는 동일 — 판단 기록).
func istioCandidatePaths(resource, namespace, name string) []string {
	return []string{
		"/apis/networking.istio.io/v1/namespaces/" + namespace + "/" + resource + "/" + name,
		"/apis/networking.istio.io/v1beta1/namespaces/" + namespace + "/" + resource + "/" + name,
	}
}

// --- URN·payload 파싱. ---

// parseTrafficURN — urn:k8s:{ctxID}:{singular}:{namespace}/{name}(buildURN 기타
// 네임스페이스 소속 성분). ctxID는 정의상 프로바이더 컨텍스트 식별자(숫자)지만
// 어댑터는 검증만 하고 사용하지 않는다 — 클러스터 주소는 Connection이 결정한다
// (arch rule 2).
func parseTrafficURN(urn, singular string) (string, string, error) {
	rest, ok := strings.CutPrefix(urn, "urn:k8s:")
	if !ok {
		return "", "", fmt.Errorf("kubernetes: resource URN %q is not a k8s URN (want urn:k8s:<ctx>:%s:<ns>/<name>)", urn, singular)
	}
	parts := strings.Split(rest, ":")
	if len(parts) != 3 || parts[1] != singular {
		return "", "", fmt.Errorf("kubernetes: resource URN %q is not a %s URN — this leg serves %ss only", urn, singular, singular)
	}
	if _, err := strconv.ParseUint(parts[0], 10, 64); err != nil {
		return "", "", fmt.Errorf("kubernetes: resource URN %q carries a non-numeric context id %q", urn, parts[0])
	}
	tail := strings.Split(parts[2], "/")
	if len(tail) != 2 || strings.TrimSpace(tail[0]) == "" || strings.TrimSpace(tail[1]) == "" {
		return "", "", fmt.Errorf("kubernetes: %s URN tail %q must be <namespace>/<name>", singular, parts[2])
	}
	return strings.TrimSpace(tail[0]), strings.TrimSpace(tail[1]), nil
}

// trafficWeightsOf — Payload["routes"] 존재 검증(v1 payload.Routes 빈 배열 거부
// 게이트). 가중치 음수·합계 100 검증은 v1과 같은 순서 — GET·엔트리·수 불일치 판정
// 뒤다(validateTrafficWeights). v1 route 엔트리의 index·host·subset·port·label 키는
// mutation이 소비하지 않는 표시 성분(K8sIstioTrafficRoute)이라 V2도 weight만
// 디코드하고 위치로 대응한다(v1 splice 동일 — 판단 기록).
func trafficWeightsOf(payload contract.JSONMap, invalidMsg string) ([]int, error) {
	var spec struct {
		Routes []struct {
			Weight int `json:"weight"`
		} `json:"routes"`
	}
	if err := payloadDecode(payload, &spec); err != nil {
		return nil, err
	}
	if len(spec.Routes) == 0 {
		return nil, errors.New(invalidMsg)
	}
	weights := make([]int, len(spec.Routes))
	for i, route := range spec.Routes {
		weights[i] = route.Weight
	}
	return weights, nil
}

// validateTrafficWeights — v1의 GET 후 검증 2건(위치 순서 보존): 각 가중치 ≥ 0 →
// 합계 == 100.
func validateTrafficWeights(weights []int) error {
	total := 0
	for _, weight := range weights {
		if weight < 0 {
			return errors.New("kubernetes: traffic weight must be greater than or equal to 0")
		}
		total += weight
	}
	if total != 100 {
		return errors.New("kubernetes: traffic weights must total 100")
	}
	return nil
}

// --- 관측 splice — 원시 맵 왕복(파일 헤더 편차 1·2). ---

// firstTrafficEntryIndex — 첫 "조정 가능" 엔트리(v1 firstVirtualServiceHTTPRouteIndex
// ·firstHTTPRouteRuleIndex 동치): routeField 배열이 비어 있지 않은 첫 인덱스.
// 없으면 -1이다.
func firstTrafficEntryIndex(current map[string]any, spec trafficKindSpec) int {
	specMap, _ := current["spec"].(map[string]any)
	entries, _ := specMap[spec.entryField].([]any)
	for i, rawEntry := range entries {
		entry, _ := rawEntry.(map[string]any)
		if routes, _ := entry[spec.routeField].([]any); len(routes) > 0 {
			return i
		}
	}
	return -1
}

// observedTrafficRouteCount — 목표 엔트리의 경로 수(v1 "route count changed"
// 판정의 관측측).
func observedTrafficRouteCount(current map[string]any, spec trafficKindSpec, entryIndex int) int {
	specMap, _ := current["spec"].(map[string]any)
	entries, _ := specMap[spec.entryField].([]any)
	if entryIndex >= len(entries) {
		return -1
	}
	entry, _ := entries[entryIndex].(map[string]any)
	routes, _ := entry[spec.routeField].([]any)
	return len(routes)
}

// trafficPutBody — 관측 맵은 그대로 두고 목표 엔트리의 경로 배열만 교체한 PUT 본문을
// 만든다(불변 규약 — buildServicePatchBody·manifestForPut 선례): 척추(최상위·spec·
// 엔트리 배열·목표 엔트리)를 복사하고 경로 항목 맵을 복사해 weight만 기록한다.
// 관측의 나머지 성분 — metadata.resourceVersion 포함(파일 헤더 편차 2) — 는 원형
// 그대로 실린다. 수 불일치는 executeTraffic이 먼저 거부하므로 여기선 불변이다.
func trafficPutBody(current map[string]any, spec trafficKindSpec, entryIndex int, weights []int) map[string]any {
	specMap, _ := current["spec"].(map[string]any)
	entries, _ := specMap[spec.entryField].([]any)
	entry, _ := entries[entryIndex].(map[string]any)
	routes, _ := entry[spec.routeField].([]any)

	routeCopy := make([]any, len(routes))
	for i, rawRoute := range routes {
		item := mapsClone(rawRoute.(map[string]any))
		item["weight"] = weights[i]
		routeCopy[i] = item
	}
	entryCopy := mapsClone(entry)
	entryCopy[spec.routeField] = routeCopy
	entriesCopy := make([]any, len(entries))
	copy(entriesCopy, entries)
	entriesCopy[entryIndex] = entryCopy
	specCopy := mapsClone(specMap)
	specCopy[spec.entryField] = entriesCopy
	body := mapsClone(current)
	body["spec"] = specCopy
	return body
}

// mapsClone — 얕은 맵 복사 헬퍼(copy-on-write 척추용).
func mapsClone(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

// --- 실행 코어. ---

// executeTraffic — 검증 순서 v1 1:1(파일 헤더) → GET(후보 순회) → splice PUT 1회
// → state handle 발행. 두 opdef가 같은 코어를 종 스펙으로 나눠 쓴다.
func (a *Adapter) executeTraffic(ctx context.Context, req contract.OperationRequest, spec trafficKindSpec) (contract.OperationHandle, error) {
	ns, name, err := parseTrafficURN(req.ResourceURN, spec.singular)
	if err != nil {
		return contract.OperationHandle{}, err
	}
	weights, err := trafficWeightsOf(req.Payload, spec.invalidPayloadMsg)
	if err != nil {
		return contract.OperationHandle{}, err
	}
	client, err := a.executionClient(req)
	if err != nil {
		return contract.OperationHandle{}, err
	}

	apiPath := ""
	var current map[string]any
	for _, candidate := range spec.candidates(spec.resource, ns, name) {
		var observed map[string]any
		getErr := client.getJSONOp(ctx, candidate, nil, "execute", &observed)
		if errors.Is(getErr, errNotFound) {
			continue
		}
		if getErr != nil {
			return contract.OperationHandle{}, getErr
		}
		apiPath, current = candidate, observed
		break
	}
	if apiPath == "" {
		return contract.OperationHandle{}, fmt.Errorf("kubernetes: %s target %q was not found on any served api version", spec.singular, req.ResourceURN)
	}

	entryIndex := firstTrafficEntryIndex(current, spec)
	if entryIndex < 0 {
		return contract.OperationHandle{}, errors.New(spec.noAdjustableMsg)
	}
	if observedTrafficRouteCount(current, spec, entryIndex) != len(weights) {
		return contract.OperationHandle{}, errors.New(spec.countChangedMsg)
	}
	if err := validateTrafficWeights(weights); err != nil {
		return contract.OperationHandle{}, err
	}
	if err := client.doJSON(ctx, http.MethodPut, apiPath, nil, "application/json", trafficPutBody(current, spec, entryIndex, weights), "execute", nil); err != nil {
		return contract.OperationHandle{}, err
	}
	ref, err := encodeStateRef(req.Connection.UID, apiPath, stateExpectation{
		Resource:   spec.expectationResource,
		EntryIndex: entryIndex,
		Weights:    weights,
	})
	if err != nil {
		return contract.OperationHandle{}, err
	}
	return contract.OperationHandle{ProviderRef: ref}, nil
}

// executeIstioTrafficUpdate / executeHTTPRouteTrafficUpdate — dispatch
// (executor.go)가 진입하는 2종 진입점.
func (a *Adapter) executeIstioTrafficUpdate(ctx context.Context, req contract.OperationRequest) (contract.OperationHandle, error) {
	return a.executeTraffic(ctx, req, istioTrafficSpec)
}

func (a *Adapter) executeHTTPRouteTrafficUpdate(ctx context.Context, req contract.OperationRequest) (contract.OperationHandle, error) {
	return a.executeTraffic(ctx, req, httpRouteTrafficSpec)
}
