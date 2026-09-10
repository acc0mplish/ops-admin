// executor_state.go — P2-D 분할(P2-C 권고 상환 — executor_config.go가 646행으로
// 650 경고선에 닿아 state handle encode/decode와 가족 공용분을 떼어낸다 — §9-7
// 동반 분할). 소관은 state handle 가족의 "발행+poll 1회" 공용면이다(§J-P1-6
// 확정표 — P2-B node·service, P2-C apply·delete, P2-D traffic 2종이 승계):
//
//	state|<connUID>|<apiPath>|<base64url(expectation JSON)>
//
// handle은 동결된 기대 상태를 자기서술하고, Poll은 GET 1회로 관측 상태가 기대를
// 포함하는지 판정한다(상태 에코). generation·resourceVersion을 수렴 신호로 쓰지
// 않는 것이 설계 판단이다: node의 metadata patch는 generation을 증가시키지 않을
// 수 있고, resourceVersion은 노드 하트비트만으로도 떠돈다 — 상태 에코만이 두 종
// 모두에서 단조 판정면이다. 종별 실행 leg는 executor_config.go(node·service)·
// executor_resource.go(manifest·delete)·executor_traffic.go(traffic 2종)가
// 소유하고, 본 파일은 encode/decode·관측 디코드·satisfiedBy·pollStateRef만
// 운영한다(판단 기록 — 분할 봉합선은 "handle 계약"이다).
//
// P2-D 확장 — traffic 2종의 expectation 성분(EntryIndex·Weights)과 판정 케이스.
package kubernetes

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"ops-admin/backend/internal/infra/contract"
)

// stateHandleMarker — rollout 가족(rollout|…)과 구별되는 ProviderRef 첫 세그먼트:
//
//	state|<connUID>|<apiPath>|<base64url(expectation)>
//
// base64url(RFC 4648 unpadded)을 쓰는 이유 — expectation JSON에는 '|'가 들어갈 수
// 있고 handle은 '|'-구분 세그먼트 계약이라다. connUID·apiPath는 공개 값이다
// (자격 물질 아님 — 보존 제약 #7).
const stateHandleMarker = "state"

// stateExpectation — handle이 운반하는 동결 기대 상태. Resource가 종을 가르는
// 판별자다. json.Marshal의 필드 순서 고정 + 맵 키 정렬로 인코딩이 결정적이다 —
// 동일 payload 재실행은 동일 handle이다(J1 전송 계약의 state 가족 형태).
type stateExpectation struct {
	Resource     string                 `json:"resource"` // node | service | manifest | delete | traffic_istio | traffic_httproute
	Labels       map[string]string      `json:"labels,omitempty"`
	Type         string                 `json:"type,omitempty"`
	ExternalName string                 `json:"externalName,omitempty"`
	Selector     map[string]string      `json:"selector,omitempty"`
	Ports        []statePortExpectation `json:"ports,omitempty"`
	// Manifest — P2-C apply(provider_frozen_manifest)가 동결하는 목표 manifest.
	// 판정은 종별 디코드 없이 부분집합 에코(manifestSubsumes —
	// executor_resource.go)라 typed 성분이 없다.
	Manifest map[string]any `json:"manifest,omitempty"`
	// EntryIndex·Weights — P2-D traffic 2종이 동결하는 목표 엔트리(첫 조정 가능
	// http/rule — execute가 확정한 위치)와 위치 가중치 배열. 판정은 동일 위치의
	// 관측 가중치 에코다(satisfiedBy traffic 케이스).
	EntryIndex int   `json:"entryIndex,omitempty"`
	Weights    []int `json:"weights,omitempty"`
}

// statePortExpectation — targetPort는 정규형(숫자는 10진 문자열)으로 동결한다 —
// 관측값(intOrString)과의 비교를 문자열 동치 하나로 만든다.
type statePortExpectation struct {
	Port       int    `json:"port"`
	Protocol   string `json:"protocol"`
	TargetPort string `json:"targetPort"`
}

// stateHandle — decodeStateRef의 결과.
type stateHandle struct {
	ConnectionUID string
	APIPath       string
	Expectation   stateExpectation
}

func encodeStateRef(connUID, apiPath string, expectation stateExpectation) (string, error) {
	raw, err := json.Marshal(expectation)
	if err != nil {
		return "", fmt.Errorf("kubernetes: state expectation encode: %w", err)
	}
	return strings.Join([]string{
		stateHandleMarker, connUID, apiPath, base64.RawURLEncoding.EncodeToString(raw),
	}, "|"), nil
}

func decodeStateRef(ref string) (stateHandle, error) {
	parts := strings.Split(ref, "|")
	if len(parts) != 4 || parts[0] != stateHandleMarker {
		return stateHandle{}, fmt.Errorf("kubernetes: handle %q is not a state handle (want state|<connUID>|<apiPath>|<expectation>)", ref)
	}
	if parts[1] == "" {
		return stateHandle{}, fmt.Errorf("kubernetes: state handle %q carries no connection uid", ref)
	}
	// apiPath 가드 — "/api" 접두(core 그룹 /api/v1/…·그룹 /apis/<group>/… 양쪽).
	// P2-C resource 가족이 /apis/… 경로(workload·GatewayAPI apply)를 도입하며
	// "/api/"에서 완화했다 — garbage("notaapi" 등)는 계속 거부된다(판단 기록).
	if !strings.HasPrefix(parts[2], "/api") {
		return stateHandle{}, fmt.Errorf("kubernetes: state handle %q carries a malformed apiPath %q", ref, parts[2])
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[3])
	if err != nil {
		return stateHandle{}, fmt.Errorf("kubernetes: state handle %q carries an undecodable expectation: %w", ref, err)
	}
	var expectation stateExpectation
	if err := json.Unmarshal(raw, &expectation); err != nil {
		return stateHandle{}, fmt.Errorf("kubernetes: state handle %q carries a malformed expectation: %w", ref, err)
	}
	// resource 화이트리스트 — node·service·traffic 2종은 satisfiedBy, manifest·
	// delete는 별도 폴 leg(pollManifestRef·pollAbsentRef — executor_resource.go)가
	// 판정한다. traffic 2종은 encodeStateRef가 만들지 않는 무가중치 expectation을
	// decode에서 거부한다 — 통과시키면 영원히 Running으로 붙잡힌 handle이 되므로
	// 변형 입력은 여기서 끊는다(P2-D 판단 기록).
	switch expectation.Resource {
	case "node", "service", "manifest", "delete":
	case "traffic_istio", "traffic_httproute":
		if len(expectation.Weights) == 0 || expectation.EntryIndex < 0 {
			return stateHandle{}, fmt.Errorf("kubernetes: state handle %q carries a %s expectation without weights", ref, expectation.Resource)
		}
	default:
		return stateHandle{}, fmt.Errorf("kubernetes: state handle %q carries an unknown expectation resource %q", ref, expectation.Resource)
	}
	return stateHandle{ConnectionUID: parts[1], APIPath: parts[2], Expectation: expectation}, nil
}

// --- Poll — state handle 가족의 발행+poll 1회 판정면. ---

// stateObservation — node·service·traffic 공용 GET 디코드 형태. 어느 종의 응답에도
// 없는 필드는 영값(배열은 nil)으로 디코드된다(workloadRolloutStatus 선례 — 단일
// 디코드 형태).
type stateObservation struct {
	Metadata struct {
		Generation int64             `json:"generation"`
		Labels     map[string]string `json:"labels"`
	} `json:"metadata"`
	Spec struct {
		Type         string                `json:"type"`
		ExternalName string                `json:"externalName"`
		Selector     map[string]string     `json:"selector"`
		Ports        []observedServicePort `json:"ports"`
		// P2-D — traffic 2종 관측 성분. VirtualService는 spec.http[].route[].weight,
		// HTTPRoute는 spec.rules[].backendRefs[].weight다 — 어느 쪽 expectation인지는
		// Resource 판별자가 결정하고, 대상 종에 없는 배열은 nil로 디코드된다.
		HTTP  []observedTrafficEntry `json:"http"`
		Rules []observedTrafficRule  `json:"rules"`
	} `json:"spec"`
}

// observedServicePort — targetPort는 intOrString이라 원시 JSON으로 받아 정규형으로
// 비교한다.
type observedServicePort struct {
	Port       int             `json:"port"`
	Protocol   string          `json:"protocol"`
	TargetPort json.RawMessage `json:"targetPort"`
}

// observedTrafficEntry — istio VirtualService spec.http 항목 중 판정에 필요한
// route 가중치만 디코드한다(v1 kubeIstioVirtualService의 route[].weight 성분).
type observedTrafficEntry struct {
	Route []struct {
		Weight int `json:"weight"`
	} `json:"route"`
}

// observedTrafficRule — GatewayAPI HTTPRoute spec.rules 항목 중 backendRefs
// 가중치만 디코드한다(v1 kubeHTTPRoute의 backendRefs[].weight 성분).
type observedTrafficRule struct {
	BackendRefs []struct {
		Weight int `json:"weight"`
	} `json:"backendRefs"`
}

// canonicalTargetPort — 기대측 정규형(숫자 → 10진 문자열, 문자열 → trim).
func canonicalTargetPort(value any) string {
	switch typed := value.(type) {
	case int:
		return strconv.Itoa(typed)
	case string:
		return strings.TrimSpace(typed)
	}
	return fmt.Sprintf("%v", value)
}

func canonicalObservedTargetPort(raw json.RawMessage) string {
	var asInt int
	if err := json.Unmarshal(raw, &asInt); err == nil {
		return strconv.Itoa(asInt)
	}
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		return strings.TrimSpace(asString)
	}
	return string(raw)
}

// labelsSuperset — 기대 레이블 전부가 관측에 같은 값으로 있는지(초집합). node
// 시스템 레이블의 kubelet 재부착 때문에 동치가 아니라 초집합이다(executor_config.go
// 파일 헤더 판단 기록).
func labelsSuperset(observed, want map[string]string) bool {
	for key, value := range want {
		if observed[key] != value {
			return false
		}
	}
	return true
}

// trafficWeightsEcho — P2-D traffic 가족의 수렴 판정: 관측 경로 수가 기대와 같고
// 각 위치 가중치가 동결값과 일치한다(위치 대응 — v1 splice가 index로 쓴다). 타 종
// 판정 같은 초집합이 아니라 전체 동치다 — 가중치 배열은 PUT이 통째로 대체하는
// 값이고(merge-patch의 ports 배열 대체 선례), 관측에 여분 경로가 남는 것은
// 수렴이 아니다.
func trafficWeightsEcho(observed, want []int) bool {
	if len(observed) != len(want) {
		return false
	}
	for i := range want {
		if observed[i] != want[i] {
			return false
		}
	}
	return true
}

// satisfiedBy — state 가족의 수렴 판정식. 미지 Resource는 거짓(fail-closed —
// 알 수 없는 handle은 영원히 Running으로, 오수렴 오탐보다 안전하다).
func (e stateExpectation) satisfiedBy(o *stateObservation) bool {
	switch e.Resource {
	case "node":
		return labelsSuperset(o.Metadata.Labels, e.Labels)
	case "service":
		if o.Spec.Type != e.Type {
			return false
		}
		if e.Type == "ExternalName" {
			return o.Spec.ExternalName == e.ExternalName
		}
		if !labelsSuperset(o.Spec.Selector, e.Selector) {
			return false
		}
		for _, want := range e.Ports {
			found := false
			for _, got := range o.Spec.Ports {
				if got.Port == want.Port && got.Protocol == want.Protocol &&
					canonicalObservedTargetPort(got.TargetPort) == want.TargetPort {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
		return true
	case "traffic_istio":
		// P2-D k8s.istio.traffic_update — 동결 엔트리 위치의 route 가중치 에코.
		// 위치는 execute가 확정한 첫 조정 가능 http 항목이다(v1
		// firstVirtualServiceHTTPRouteIndex 선정 승계).
		if e.EntryIndex >= len(o.Spec.HTTP) {
			return false
		}
		routes := o.Spec.HTTP[e.EntryIndex].Route
		observed := make([]int, len(routes))
		for i := range routes {
			observed[i] = routes[i].Weight
		}
		return trafficWeightsEcho(observed, e.Weights)
	case "traffic_httproute":
		// P2-D k8s.httproute.traffic_update — 동결 rule 위치의 backendRefs 가중치
		// 에코(v1 firstHTTPRouteRuleIndex 선정 승계).
		if e.EntryIndex >= len(o.Spec.Rules) {
			return false
		}
		refs := o.Spec.Rules[e.EntryIndex].BackendRefs
		observed := make([]int, len(refs))
		for i := range refs {
			observed[i] = refs[i].Weight
		}
		return trafficWeightsEcho(observed, e.Weights)
	}
	return false
}

// pollStateRef — state handle의 폴 leg: GET 1회 → 상태 에코 판정. Succeeded의
// detail은 stateResultRedaction(compose_k8s_ops.go)과 1:1인 2키(generation·
// serverURL)다. 발급 시점에 발행(PATCH/PUT)은 이미 수용된 뒤라 1회 폴이 상수
// 경로고, 미수렴(동시 변경·복제 지연)은 Running으로 다음 폴을 기다린다 — 종단은
// restart leg와 같은 리스/재시도 메커니즘이 지킨다(§3.6a).
func (a *Adapter) pollStateRef(ctx context.Context, req contract.PollRequest, ref string) (contract.OperationStatus, error) {
	handle, err := decodeStateRef(ref)
	if err != nil {
		return contract.OperationStatus{}, err
	}
	// J12 정합 가드 — rollout leg와 동일 문구: 다른 클러스터를 폴하는 오발사를
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
	// P2-C resource 가족(manifest·delete)은 판정면이 종별 디코드와 다르다 —
	// 부분집합 에코·404 종단(executor_resource.go)으로 분기한다.
	switch handle.Expectation.Resource {
	case "manifest":
		return a.pollManifestRef(ctx, client, handle)
	case "delete":
		return a.pollAbsentRef(ctx, client, handle)
	}
	var obj stateObservation
	if err := client.getJSONOp(ctx, handle.APIPath, nil, "poll", &obj); err != nil {
		return contract.OperationStatus{}, err
	}
	if !handle.Expectation.satisfiedBy(&obj) {
		return contract.OperationStatus{State: contract.OperationStateRunning}, nil
	}
	return contract.OperationStatus{
		State: contract.OperationStateSucceeded,
		Detail: contract.JSONMap{
			"generation": obj.Metadata.Generation,
			"serverURL":  client.rt.Server,
		},
	}, nil
}
