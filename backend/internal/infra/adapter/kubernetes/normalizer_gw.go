package kubernetes

// P1-C1 수집 확장의 분할 파일(R-P3 재분할 — normalizer.go 650행 계약 유지).
// 소관은 GatewayAPI 확장 종(gateways·httproutes)의 디코드+키다. normalized 키는
// 계획 J-P1-3 표와 1:1이다:
//
//	gateway    → gatewayClassName·hosts·addresses·ports
//	httproute  → parents·targets
//
// 유도 의미론은 legacy 수집기(k8s_build_net.go — Z 승계 원장 33종 중 6종)와
// 동치다: collectGatewayAPIHosts(:181)·collectGatewayAPIPorts(:189)·
// collectGatewayAPIAddresses(:197)·collectHTTPRouteParents(:239)·
// collectHTTPRouteTargets(:251). 관측 부재는 키 생략(P1-B 규약) — "-" 포맷은
// 조립(P1-D) 소유다. 비교 집합 불참(I-P5 이월 — J-P1-2).

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"ops-admin/backend/internal/infra/contract"
)

// gatewayObject is the GatewayAPI Gateway item subset — legacy kubeGatewayAPI
// (k8s_types_mesh.go:105)와 동일 필드. v1·v1beta1 양 버전이 동일 형상이다
// (J-P1-1 폴백 대상).
type gatewayObject struct {
	Metadata objectMeta `json:"metadata"`
	Spec     struct {
		GatewayClassName string `json:"gatewayClassName"`
		Listeners        []struct {
			Name     string `json:"name"`
			Hostname string `json:"hostname"`
			Port     int    `json:"port"`
			Protocol string `json:"protocol"`
		} `json:"listeners"`
	} `json:"spec"`
	Status struct {
		Addresses []struct {
			Type  string `json:"type"`
			Value string `json:"value"`
		} `json:"addresses"`
	} `json:"status"`
}

// httpRouteObject is the GatewayAPI HTTPRoute item subset — legacy
// kubeHTTPRoute(k8s_types_mesh.go:126)와 동일 필드. matches는 유도 키가 없어
// 디코드에서 뺀다(원천 유지 가치 없음 — Raw는 metadata 한정).
type httpRouteObject struct {
	Metadata objectMeta `json:"metadata"`
	Spec     struct {
		Hostnames  []string `json:"hostnames"`
		ParentRefs []struct {
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
		} `json:"parentRefs"`
		Rules []struct {
			BackendRefs []struct {
				Name      string `json:"name"`
				Namespace string `json:"namespace"`
				Port      int    `json:"port"`
				Weight    int    `json:"weight"`
			} `json:"backendRefs"`
		} `json:"rules"`
	} `json:"spec"`
}

// uniqueNonEmptyStrings mirrors legacy uniqueNonEmptyStrings
// (k8s_build_net.go:75) — trim 후 ""·"-" 폐기, 순서 보존 중복 제거.
func uniqueNonEmptyStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || value == "-" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

// formatGatewayPort mirrors legacy formatIstioPort(k8s_build_net.go:92) —
// "port/proto"(protocol 공란 시 TCP, port ≤ 0이면 protocol 텍스트).
func formatGatewayPort(number int, protocol string) string {
	if number <= 0 {
		if strings.TrimSpace(protocol) == "" {
			return "-"
		}
		return strings.TrimSpace(protocol)
	}
	proto := strings.TrimSpace(protocol)
	if proto == "" {
		proto = "TCP"
	}
	return strconv.Itoa(number) + "/" + proto
}

// normalizeGWSection decodes ONE GatewayAPI section item — the normalizer.go
// switch delegates gateways·httproutes here. Kind/Subtype 스탬핑과 Raw 상한은
// normalizeSection 후미가 균일 적용한다(normalizer_batch.go와 동일 분할 경계).
func normalizeGWSection(ctxID uint, km kindMapping, section string, raw json.RawMessage) (contract.DiscoveredResource, error) {
	switch section {
	case "gateways":
		var o gatewayObject
		if err := json.Unmarshal(raw, &o); err != nil {
			return contract.DiscoveredResource{}, fmt.Errorf("kubernetes: gateway decode: %w", err)
		}
		return gatewayResource(ctxID, km, o), nil

	case "httproutes":
		var o httpRouteObject
		if err := json.Unmarshal(raw, &o); err != nil {
			return contract.DiscoveredResource{}, fmt.Errorf("kubernetes: httproute decode: %w", err)
		}
		return httpRouteResource(ctxID, km, o), nil
	}
	return contract.DiscoveredResource{}, fmt.Errorf("kubernetes: unknown gateway-api section %q", section)
}

// gatewayResource normalizes one Gateway item. hosts·ports는 listener 파생
// (hostname 부재 listener는 legacy firstNonEmpty(hostname, "*") 동치),
// addresses는 status.addresses 값 목록이다.
func gatewayResource(ctxID uint, km kindMapping, o gatewayObject) contract.DiscoveredResource {
	n := contract.JSONMap{}
	if o.Spec.GatewayClassName != "" {
		n["gatewayClassName"] = o.Spec.GatewayClassName
	}
	hosts := make([]string, 0, len(o.Spec.Listeners))
	for _, l := range o.Spec.Listeners {
		host := strings.TrimSpace(l.Hostname)
		if host == "" {
			host = "*"
		}
		hosts = append(hosts, host)
	}
	if h := uniqueNonEmptyStrings(hosts); len(h) > 0 {
		n["hosts"] = h
	}
	ports := make([]string, 0, len(o.Spec.Listeners))
	for _, l := range o.Spec.Listeners {
		ports = append(ports, formatGatewayPort(l.Port, l.Protocol))
	}
	if p := uniqueNonEmptyStrings(ports); len(p) > 0 {
		n["ports"] = p
	}
	addresses := make([]string, 0, len(o.Status.Addresses))
	for _, a := range o.Status.Addresses {
		addresses = append(addresses, a.Value)
	}
	if a := uniqueNonEmptyStrings(addresses); len(a) > 0 {
		n["addresses"] = a
	}
	return contract.DiscoveredResource{
		ExternalID: externalID(km, o.Metadata), ExternalURN: buildURN(ctxID, km, o.Metadata), DisplayName: o.Metadata.Name,
		Raw: buildRaw(o.Metadata), Normalized: n,
	}
}

// httpRouteResource normalizes one HTTPRoute item. parents는 parentRefs의
// [namespace/]name, targets은 rules[].backendRefs의 [ns/]name[:port]
// [" (W%)"] 목록이다(legacy collectHTTPRouteParents/Targets 동치).
func httpRouteResource(ctxID uint, km kindMapping, o httpRouteObject) contract.DiscoveredResource {
	n := contract.JSONMap{}
	parents := make([]string, 0, len(o.Spec.ParentRefs))
	for _, ref := range o.Spec.ParentRefs {
		if ns := strings.TrimSpace(ref.Namespace); ns != "" {
			parents = append(parents, ns+"/"+ref.Name)
			continue
		}
		parents = append(parents, ref.Name)
	}
	if p := uniqueNonEmptyStrings(parents); len(p) > 0 {
		n["parents"] = p
	}
	targets := make([]string, 0)
	for _, rule := range o.Spec.Rules {
		for _, backend := range rule.BackendRefs {
			target := backend.Name
			if ns := strings.TrimSpace(backend.Namespace); ns != "" {
				target = ns + "/" + target
			}
			if backend.Port > 0 {
				target += ":" + strconv.Itoa(backend.Port)
			}
			if backend.Weight > 0 {
				target += fmt.Sprintf(" (%d%%)", backend.Weight)
			}
			targets = append(targets, target)
		}
	}
	if t := uniqueNonEmptyStrings(targets); len(t) > 0 {
		n["targets"] = t
	}
	return contract.DiscoveredResource{
		ExternalID: externalID(km, o.Metadata), ExternalURN: buildURN(ctxID, km, o.Metadata), DisplayName: o.Metadata.Name,
		Raw: buildRaw(o.Metadata), Normalized: n,
	}
}
