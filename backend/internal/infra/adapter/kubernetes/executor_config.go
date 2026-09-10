// executor_config.go — P2-B: state-convergent mutation 2종(k8s.node.labels_update·
// k8s.service.update)의 OperationExecutor leg(계획 r3 §J-P1-6 확정표 — k8s.go:308
// UpdateK8sNodeLabels·k8s_detail.go:74 UpdateK8sService의 v1 원천 2라인). 이 2종은
// rollout이 없다 — 확정표의 handle·poll 형태 "발행+poll 1회"를 state handle
// 가족으로 착지한다:
//
//	state|<connUID>|<apiPath>|<base64url(expectation JSON)>
//
// handle은 동결된 기대 상태(node 레이블 집합·service type/selector/ports)를
// 자기서술하고, Poll은 GET 1회로 관측 상태가 기대를 포함하는지 판정한다(상태
// 에코). generation·resourceVersion을 수렴 신호로 쓰지 않는 것이 설계 판단이다:
// node의 metadata patch는 generation을 증가시키지 않을 수 있고, resourceVersion은
// 노드 하트비트만으로도 떠돈다 — 상태 에코만이 두 종 모두에서 단조 판정면이다.
// P2-C apply(provider_frozen_manifest)·P2-D traffic 2종이 같은 가족을 승계한다.
//
//   - node labels_update: LIST /api/v1/nodes 1회 → merge-patch 1회. V2 node URN은
//     uid 신원이다(buildURN — node·pod → uid; 이름은 display) — k8s API는 uid
//     직접 조회가 없어 execute가 LIST에서 이름을 확정하고 handle에 apiPath를
//     새긴다(poll은 재해석 없이 그 경로를 폴한다). v1과 동일하게 payload에 없는
//     기존 레이블은 null로 제거한다. kubelet이 시스템 레이블을 재부착할 수 있으므로
//     poll 판정은 "기대 레이블 전부 관측"(초집합)이다 — 제거 검증은 재부착과
//     비단조적이라 판정면에서 뺀다(구현 판단 기록).
//   - service update: GET 1회(현행 spec — v1과 동일) → wholesale-spec merge-patch
//     1회. v1의 delete(spec, …)는 로컬 복사본에만 유효하고 merge-patch는 부재 키를
//     지우지 않는다 — selector·clusterIP 계열은 v1과 동일한 upsert 와이어가 된다
//     (제거 무효 유지). ports 배열은 merge-patch가 배열을 통째로 대체한다.
//
// payload 검증은 실행기 자체(frozenRestartedAt 선례 — §J-P1-6). 멱등 근거
// (provider_state_convergent): 재실행은 동일 목표 상태의 재적용 — node 재실행
// patch는 null 제거항이 소멸할 뿐이고 service 재실행은 동일 값을 재 upsert한다.
// 어느 쪽도 실변경이 없으면 generation 무증가다(테스트가 실증).
package kubernetes

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"ops-admin/backend/internal/infra/contract"
)

// operation names — compose.go opdef 등록과 dispatch(executor.go)가 공유하는
// 단일 원천(§J-P1-6 — descriptors are code).
const (
	NodeLabelsUpdateOperationName = "k8s.node.labels_update"
	ServiceUpdateOperationName    = "k8s.service.update"
)

// --- state handle — 발행+poll 1회 가족의 ProviderRef(§J-P1-6 확정표). ---

// stateHandleMarker — rollout 가족(rollout|…)과 구별되는 ProviderRef 첫 세그먼트:
//
//	state|<connUID>|<apiPath>|<base64url(expectation)>
//
// base64url(RFC 4648 unpadded)을 쓰는 이유 — expectation JSON에는 '|'가 들어갈 수
// 있고 handle은 '|'-구분 세그먼트 계약이라다. connUID·apiPath는 공개 값이다
// (자격 물질 아님 — 보존 제약 #7).
const stateHandleMarker = "state"

// stateExpectation — handle이 운반하는 동결 기대 상태. Resource가 두 종을 가르는
// 판별자다. json.Marshal의 필드 순서 고정 + 맵 키 정렬로 인코딩이 결정적이다 —
// 동일 payload 재실행은 동일 handle이다(J1 전송 계약의 state 가족 형태).
type stateExpectation struct {
	Resource     string                 `json:"resource"` // node | service | manifest | delete
	Labels       map[string]string      `json:"labels,omitempty"`
	Type         string                 `json:"type,omitempty"`
	ExternalName string                 `json:"externalName,omitempty"`
	Selector     map[string]string      `json:"selector,omitempty"`
	Ports        []statePortExpectation `json:"ports,omitempty"`
	// Manifest — P2-C apply(provider_frozen_manifest)가 동결하는 목표 manifest.
	// 판정은 종별 디코드 없이 부분집합 에코(manifestSubsumes —
	// executor_resource.go)라 typed 성분이 없다.
	Manifest map[string]any `json:"manifest,omitempty"`
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
	// resource 화이트리스트 — node·service는 satisfiedBy(executor_config.go),
	// manifest·delete는 별도 폴 leg(pollManifestRef·pollAbsentRef —
	// executor_resource.go)가 판정한다.
	if expectation.Resource != "node" && expectation.Resource != "service" &&
		expectation.Resource != "manifest" && expectation.Resource != "delete" {
		return stateHandle{}, fmt.Errorf("kubernetes: state handle %q carries an unknown expectation resource %q", ref, expectation.Resource)
	}
	return stateHandle{ConnectionUID: parts[1], APIPath: parts[2], Expectation: expectation}, nil
}

// --- URN 파싱 — node는 uid 신원, service는 ns/name 성분(buildURN §8.1). ---

// parseNodeURN — urn:k8s:{ctxID}:node:{uid}. buildURN이 node·pod를 uid 신원으로
// 적은 것이 이 파서가 uid를 받는 이유다. k8s API는 uid 직접 조회가 없으므로
// execute가 LIST로 이름을 확정한다(위 파일 헤더).
func parseNodeURN(urn string) (string, error) {
	rest, ok := strings.CutPrefix(urn, "urn:k8s:")
	if !ok {
		return "", fmt.Errorf("kubernetes: resource URN %q is not a k8s URN (want urn:k8s:<ctx>:node:<uid>)", urn)
	}
	parts := strings.Split(rest, ":")
	if len(parts) != 3 || parts[1] != "node" {
		return "", fmt.Errorf("kubernetes: resource URN %q is not a node URN — this leg serves nodes only", urn)
	}
	if _, err := strconv.ParseUint(parts[0], 10, 64); err != nil {
		return "", fmt.Errorf("kubernetes: resource URN %q carries a non-numeric context id %q", urn, parts[0])
	}
	uid := strings.TrimSpace(parts[2])
	if uid == "" || strings.Contains(uid, "/") {
		return "", fmt.Errorf("kubernetes: node URN tail %q must be the node uid", parts[2])
	}
	return uid, nil
}

// parseServiceURN — urn:k8s:{ctxID}:service:{namespace}/{name}(
// buildURN 기타 네임스페이스 소속 성분).
func parseServiceURN(urn string) (string, string, error) {
	rest, ok := strings.CutPrefix(urn, "urn:k8s:")
	if !ok {
		return "", "", fmt.Errorf("kubernetes: resource URN %q is not a k8s URN (want urn:k8s:<ctx>:service:<ns>/<name>)", urn)
	}
	parts := strings.Split(rest, ":")
	if len(parts) != 3 || parts[1] != "service" {
		return "", "", fmt.Errorf("kubernetes: resource URN %q is not a service URN — this leg serves services only", urn)
	}
	if _, err := strconv.ParseUint(parts[0], 10, 64); err != nil {
		return "", "", fmt.Errorf("kubernetes: resource URN %q carries a non-numeric context id %q", urn, parts[0])
	}
	tail := strings.Split(parts[2], "/")
	if len(tail) != 2 {
		return "", "", fmt.Errorf("kubernetes: service URN tail %q must be <namespace>/<name>", parts[2])
	}
	ns, name := strings.TrimSpace(tail[0]), strings.TrimSpace(tail[1])
	if ns == "" || name == "" {
		return "", "", fmt.Errorf("kubernetes: service URN tail %q needs a non-empty namespace and name", parts[2])
	}
	return ns, name, nil
}

// --- k8s.node.labels_update (v1 k8s.go:308 UpdateK8sNodeLabels). ---

// nodeLabelsOf — Payload["labels"] 검증: 빈 키 거부(v1 "node label key is required"
// 문언), 키·값 trim. 빈 집합 자체는 v1과 동일하게 허용한다(전 레이블 제거 의미).
func nodeLabelsOf(payload contract.JSONMap) (map[string]string, error) {
	var spec struct {
		Labels map[string]string `json:"labels"`
	}
	if err := payloadDecode(payload, &spec); err != nil {
		return nil, err
	}
	if spec.Labels == nil {
		return nil, fmt.Errorf("kubernetes: payload carries no %q — the plan request must freeze the desired label set", "labels")
	}
	labels := make(map[string]string, len(spec.Labels))
	for key, value := range spec.Labels {
		key = strings.TrimSpace(key)
		if key == "" {
			return nil, errors.New("kubernetes: node label key is required")
		}
		labels[key] = strings.TrimSpace(value)
	}
	return labels, nil
}

// executeNodeLabelsUpdate — LIST(이름 확정 + 현행 레이블 관측) → merge-patch 1회.
// v1은 GET 1회로 이름·레이블을 함께 얻지만 V2 URN이 uid라 LIST가 그 자리를 대신한다
// — HTTP 호출 수는 v1과 같다. patch 본문은 v1 경험식 그대로: payload에 없는 기존
// 레이블은 null(제거), payload 항목은 trim 값(설정).
func (a *Adapter) executeNodeLabelsUpdate(ctx context.Context, req contract.OperationRequest) (contract.OperationHandle, error) {
	uid, err := parseNodeURN(req.ResourceURN)
	if err != nil {
		return contract.OperationHandle{}, err
	}
	labels, err := nodeLabelsOf(req.Payload)
	if err != nil {
		return contract.OperationHandle{}, err
	}
	client, err := a.executionClient(req)
	if err != nil {
		return contract.OperationHandle{}, err
	}

	var list struct {
		Items []struct {
			Metadata struct {
				Name   string            `json:"name"`
				UID    string            `json:"uid"`
				Labels map[string]string `json:"labels"`
			} `json:"metadata"`
		} `json:"items"`
	}
	if err := client.getJSONOp(ctx, "/api/v1/nodes", nil, "execute", &list); err != nil {
		return contract.OperationHandle{}, err
	}
	name := ""
	var existing map[string]string
	for _, item := range list.Items {
		if item.Metadata.UID == uid {
			name = item.Metadata.Name
			existing = item.Metadata.Labels
			break
		}
	}
	if name == "" {
		return contract.OperationHandle{}, fmt.Errorf("kubernetes: node with uid %q was not found", uid)
	}

	patch := make(map[string]any, len(existing)+len(labels))
	for key := range existing {
		if _, keep := labels[key]; !keep {
			patch[key] = nil
		}
	}
	for key, value := range labels {
		patch[key] = value
	}
	apiPath := "/api/v1/nodes/" + url.PathEscape(name)
	body := map[string]any{"metadata": map[string]any{"labels": patch}}
	if err := client.doJSON(ctx, http.MethodPatch, apiPath, nil, "application/merge-patch+json", body, "execute", nil); err != nil {
		return contract.OperationHandle{}, err
	}
	ref, err := encodeStateRef(req.Connection.UID, apiPath, stateExpectation{Resource: "node", Labels: labels})
	if err != nil {
		return contract.OperationHandle{}, err
	}
	return contract.OperationHandle{ProviderRef: ref}, nil
}

// --- k8s.service.update (v1 k8s_detail.go:74 UpdateK8sService). ---

// serviceUpdateSpec — payload 형태: v1 K8sServiceUpdatePayload의 JSON 형태 그대로
// (clusterId·namespace·name 키는 v1 원천에 있지만 V2에서는 URN이 목표를 결정하므로
// 실행기가 소비하지 않는다 — payloadDecode가 무시한다).
type serviceUpdateSpec struct {
	Type         string
	Headless     bool
	ExternalName string
	Selector     map[string]string
	Labels       map[string]string
	Annotations  map[string]string
	Ports        []servicePortSpec
}

type servicePortSpec struct {
	Name       string
	Port       int
	TargetPort string
	Protocol   string
	NodePort   int
}

// serviceSpecOf — v1 :76-99의 검증 순서·문언(k8s 접두만 부착): type 화이트리스트 →
// headless는 ClusterIP 한정 → ExternalName은 이름 필수 → 비-ExternalName은 포트 ≥1
// → 포트 범위 1..65535. type 공백은 v1과 동일하게 ClusterIP 기본값.
func serviceSpecOf(payload contract.JSONMap) (serviceUpdateSpec, error) {
	var raw struct {
		Type         string            `json:"type"`
		Headless     bool              `json:"headless"`
		ExternalName string            `json:"externalName"`
		Selector     map[string]string `json:"selector"`
		Labels       map[string]string `json:"labels"`
		Annotations  map[string]string `json:"annotations"`
		Ports        []struct {
			Name       string `json:"name"`
			Port       int    `json:"port"`
			TargetPort string `json:"targetPort"`
			Protocol   string `json:"protocol"`
			NodePort   int    `json:"nodePort"`
		} `json:"ports"`
	}
	if err := payloadDecode(payload, &raw); err != nil {
		return serviceUpdateSpec{}, err
	}
	spec := serviceUpdateSpec{
		Type:         strings.TrimSpace(raw.Type),
		Headless:     raw.Headless,
		ExternalName: raw.ExternalName,
		Selector:     raw.Selector,
		Labels:       raw.Labels,
		Annotations:  raw.Annotations,
	}
	if spec.Type == "" {
		spec.Type = "ClusterIP"
	}
	switch spec.Type {
	case "ClusterIP", "NodePort", "LoadBalancer", "ExternalName":
	default:
		return serviceUpdateSpec{}, errors.New("kubernetes: unsupported service type")
	}
	if spec.Headless && spec.Type != "ClusterIP" {
		return serviceUpdateSpec{}, errors.New("kubernetes: headless service must use ClusterIP")
	}
	if spec.Type == "ExternalName" && strings.TrimSpace(spec.ExternalName) == "" {
		return serviceUpdateSpec{}, errors.New("kubernetes: external name is required")
	}
	if spec.Type != "ExternalName" && len(raw.Ports) == 0 {
		return serviceUpdateSpec{}, errors.New("kubernetes: at least one service port is required")
	}
	for _, port := range raw.Ports {
		if port.Port < 1 || port.Port > 65535 {
			return serviceUpdateSpec{}, errors.New("kubernetes: service port must be between 1 and 65535")
		}
		spec.Ports = append(spec.Ports, servicePortSpec{
			Name: port.Name, Port: port.Port, TargetPort: port.TargetPort,
			Protocol: port.Protocol, NodePort: port.NodePort,
		})
	}
	return spec, nil
}

// filterTrimmedKV — v1의 selector·labels·annotations 필터(키·값 trim, 빈 쌍 탈락).
func filterTrimmedKV(values map[string]string) map[string]string {
	out := make(map[string]string, len(values))
	for key, value := range values {
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if key != "" && value != "" {
			out[key] = value
		}
	}
	return out
}

// serviceTargetPort — v1 k8s_detail.go serviceTargetPort 경험식의 V2 재구현
// (§J-P1-5 — 승계 원장 밖, 본 왕복 contracttest가 동치를 판정한다): 공백 → port
// 폴백, 숫자 → int, 그 외 → 문자열.
func serviceTargetPort(value string, fallback int) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	if port, err := strconv.Atoi(value); err == nil {
		return port
	}
	return value
}

// servicePortProtocol — protocol 공백은 TCP 기본(v1). patch 본문과 expectation이
// 같은 정규형을 쓰도록 단일 헬퍼로 뽑았다.
func servicePortProtocol(protocol string) string {
	if trimmed := strings.TrimSpace(protocol); trimmed != "" {
		return trimmed
	}
	return "TCP"
}

// buildServicePatchBody — v1 요청 본문의 V2 재구성: 현행 spec 최상위 복사 → type·
// externalName·selector·ports·clusterIP 재기록 → metadata는 name·namespace·
// resourceVersion(현행값 보존 — v1과 동일)·labels·annotations. v1의 delete(spec,
// …)가 로컬 복사본에만 유효한 것(merge-patch 제거 무효)까지 그대로 재현한다.
func buildServicePatchBody(current map[string]any, spec serviceUpdateSpec, ns, name string) map[string]any {
	rawSpec, _ := current["spec"].(map[string]any)
	specCopy := make(map[string]any, len(rawSpec)+4)
	for key, value := range rawSpec {
		specCopy[key] = value
	}
	rawMeta, _ := current["metadata"].(map[string]any)

	specCopy["type"] = spec.Type
	if spec.Type == "ExternalName" {
		specCopy["externalName"] = strings.TrimSpace(spec.ExternalName)
		delete(specCopy, "selector")
		delete(specCopy, "clusterIP")
		delete(specCopy, "clusterIPs")
	} else {
		delete(specCopy, "externalName")
		specCopy["selector"] = filterTrimmedKV(spec.Selector)
		if spec.Headless {
			specCopy["clusterIP"] = "None"
			specCopy["clusterIPs"] = []string{"None"}
		}
	}
	ports := make([]map[string]any, 0, len(spec.Ports))
	for _, port := range spec.Ports {
		item := map[string]any{
			"port":       port.Port,
			"protocol":   servicePortProtocol(port.Protocol),
			"targetPort": serviceTargetPort(port.TargetPort, port.Port),
		}
		if trimmed := strings.TrimSpace(port.Name); trimmed != "" {
			item["name"] = trimmed
		}
		if spec.Type == "NodePort" || spec.Type == "LoadBalancer" {
			if port.NodePort > 0 {
				item["nodePort"] = port.NodePort
			}
		}
		ports = append(ports, item)
	}
	if spec.Type != "ExternalName" {
		specCopy["ports"] = ports
	}
	metadata := map[string]any{
		"name":            name,
		"namespace":       ns,
		"resourceVersion": rawMeta["resourceVersion"],
		"labels":          filterTrimmedKV(spec.Labels),
		"annotations":     filterTrimmedKV(spec.Annotations),
	}
	return map[string]any{"apiVersion": "v1", "kind": "Service", "metadata": metadata, "spec": specCopy}
}

// serviceExpectationOf — patch가 수립할 목표 상태를 동결형으로 추출. selector는
// upsert 와이어(제거 무효)라 초집합 판정이고, ports는 배열 대체라 기대 3항목
// (port·protocol·targetPort 정규형) 전부 관측에 있으면 충분하다. clusterIP는
// 판정에서 뺀다(구현 판단 — headless 전환의 서버측 가변성, patch 실패는 execute가
// 이미 에러로 만든다).
func serviceExpectationOf(spec serviceUpdateSpec) stateExpectation {
	expectation := stateExpectation{Resource: "service", Type: spec.Type}
	if spec.Type == "ExternalName" {
		expectation.ExternalName = strings.TrimSpace(spec.ExternalName)
		return expectation
	}
	expectation.Selector = filterTrimmedKV(spec.Selector)
	for _, port := range spec.Ports {
		expectation.Ports = append(expectation.Ports, statePortExpectation{
			Port:       port.Port,
			Protocol:   servicePortProtocol(port.Protocol),
			TargetPort: canonicalTargetPort(serviceTargetPort(port.TargetPort, port.Port)),
		})
	}
	return expectation
}

// executeServiceUpdate — GET(현행) → wholesale-spec merge-patch 1회. 재실행은
// 동일 값의 재 upsert — metadata.resourceVersion만 현행값으로 갱신된다(v1 와이어와
// 동일하며, merge-patch가 자원 변경시마다 resourceVersion을 요구하지 않으므로
// 무해하다).
func (a *Adapter) executeServiceUpdate(ctx context.Context, req contract.OperationRequest) (contract.OperationHandle, error) {
	ns, name, err := parseServiceURN(req.ResourceURN)
	if err != nil {
		return contract.OperationHandle{}, err
	}
	spec, err := serviceSpecOf(req.Payload)
	if err != nil {
		return contract.OperationHandle{}, err
	}
	client, err := a.executionClient(req)
	if err != nil {
		return contract.OperationHandle{}, err
	}

	apiPath := "/api/v1/namespaces/" + ns + "/services/" + name
	var current map[string]any
	if err := client.getJSONOp(ctx, apiPath, nil, "execute", &current); err != nil {
		return contract.OperationHandle{}, err
	}
	if _, ok := current["spec"].(map[string]any); !ok {
		return contract.OperationHandle{}, errors.New("kubernetes: invalid service resource")
	}
	body := buildServicePatchBody(current, spec, ns, name)
	if err := client.doJSON(ctx, http.MethodPatch, apiPath, nil, "application/merge-patch+json", body, "execute", nil); err != nil {
		return contract.OperationHandle{}, err
	}
	ref, err := encodeStateRef(req.Connection.UID, apiPath, serviceExpectationOf(spec))
	if err != nil {
		return contract.OperationHandle{}, err
	}
	return contract.OperationHandle{ProviderRef: ref}, nil
}

// --- Poll — state handle 가족의 발행+poll 1회 판정면. ---

// stateObservation — node·service 공용 GET 디코드 형태. 어느 종의 응답에도 없는
// 필드는 영값으로 디코드된다(workloadRolloutStatus 선례).
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
	} `json:"spec"`
}

// observedServicePort — targetPort는 intOrString이라 원시 JSON으로 받아 정규형으로
// 비교한다.
type observedServicePort struct {
	Port       int             `json:"port"`
	Protocol   string          `json:"protocol"`
	TargetPort json.RawMessage `json:"targetPort"`
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
// 시스템 레이블의 kubelet 재부착 때문에 동치가 아니라 초집합이다(파일 헤더 판단
// 기록).
func labelsSuperset(observed, want map[string]string) bool {
	for key, value := range want {
		if observed[key] != value {
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
	}
	return false
}

// pollStateRef — state handle의 폴 leg: GET 1회 → 상태 에코 판정. Succeeded의
// detail은 stateResultRedaction(compose.go)과 1:1인 2키(generation·serverURL)다.
// 발급 시점에 patch는 이미 수용된 뒤라 1회 폴이 상수 경로고, 미수렴(동시 변경·
// 복제 지연)은 Running으로 다음 폴을 기다린다 — 종단은 restart leg와 같은
// 리스/재시도 메커니즘이 지킨다(§3.6a).
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
