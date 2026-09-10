package kubernetes

// normalizer_sections.go — P2-D 분할(R-P3 동반 — normalizer.go에 virtualservices
// 케이스와 앵커 디코드를 더하면 650행 경고선을 넘어 sectionKind 표와 P2-D istio
// 앵커 디코드를 이 파일로 떼어낸다 — §9-7·normalizer_batch/normalizer_gw 선례).
// 소관:
//   - sectionKind — 섹션 → kind/subtype/URN 성분 표(§3.1·mapping.md §1 동치)
//   - normalizeAnchorSection — istio VirtualService 앵커 디코드(P2-D)

import (
	"encoding/json"
	"fmt"
	"strings"

	"ops-admin/backend/internal/infra/contract"
)

// sectionKind maps a discovery section name to its kind/subtype/URN segment.
func sectionKind(section string) kindMapping {
	switch section {
	case "nodes":
		return kindMapping{kind: "orchestration.node", singular: "node"}
	case "namespaces":
		return kindMapping{kind: "orchestration.namespace", singular: "namespace"}
	case "pods":
		return kindMapping{kind: "orchestration.pod", singular: "pod"}
	case "deployments", "statefulsets", "daemonsets",
		"replicasets", "jobs", "cronjobs": // P1-A 확장 — orchestration.workload subtype 계열(J-P1-0: 어휘 슬라이스 불필요)
		return kindMapping{kind: "orchestration.workload", subtype: strings.TrimSuffix(section, "s"), singular: "workload"}
	case "services":
		return kindMapping{kind: "network.load_balancer", subtype: "service", singular: "service"}
	case "ingresses":
		return kindMapping{kind: "network.load_balancer", subtype: "ingress", singular: "ingress"}
	case "configmaps":
		return kindMapping{kind: "orchestration.configmap", singular: "configmap"}
	case "secrets":
		return kindMapping{kind: "orchestration.secret", singular: "secret"}
	case "persistentvolumes":
		return kindMapping{kind: "storage.volume", subtype: "persistent_volume", singular: "pv"}
	case "persistentvolumeclaims":
		return kindMapping{kind: "storage.volume", subtype: "pvc", singular: "pvc"}
	case "storageclasses":
		return kindMapping{kind: "storage.pool", subtype: "storage_class", singular: "storageclass"}
	case "endpoints":
		// P1-A — Phase6ResourceKindExtensions "network.endpoint"(v2-only 보조종).
		return kindMapping{kind: "network.endpoint", singular: "endpoint"}
	case "gateways":
		// P1-C1 — Phase6ResourceKindExtensions "network.gateway". Gateway·
		// HTTPRoute는 네임스페이스 소속 종이라 URN은 default 분기
		// {namespace}/{name} 성분을 따른다.
		return kindMapping{kind: "network.gateway", singular: "gateway"}
	case "httproutes":
		return kindMapping{kind: "network.http_route", singular: "httproute"}
	case "virtualservices":
		// P2-D — Phase6ResourceKindExtensions "network.virtual_service". istio
		// 오퍼레이션(k8s.istio.traffic_update) uid 앵커다(J-P1-1) — 스코프 외
		// 종이라 비교 집합 불참(R-P6·I-P5 이월).
		return kindMapping{kind: "network.virtual_service", singular: "virtualservice"}
	default:
		return kindMapping{}
	}
}

// normalizeAnchorSection — istio VirtualService 앵커의 디코드+키(P2-D). 앵커
// 최소형: 신원 계열(ExternalID·URN·DisplayName·Raw metadata)만 실고 Normalized는
// 비운다 — 이 수집의 유일 소관은 쓰기 uid 닻이지 읽기 패리티가 아니기 때문이다
// (J-P1-1 "스코프 외 앵커 — istio 오퍼레이션 uid 닻"·R-P6 "PR에 명기"). istio CRD
// 부재 클러스터는 fetchSectionPage의 404 섹션 스킵으로 본 함수에 도달하지 않는다
// (M-6 — gateways와 동일 처리).
func normalizeAnchorSection(ctxID uint, km kindMapping, raw json.RawMessage) (contract.DiscoveredResource, error) {
	var o struct {
		Metadata objectMeta `json:"metadata"`
	}
	if err := json.Unmarshal(raw, &o); err != nil {
		return contract.DiscoveredResource{}, fmt.Errorf("kubernetes: virtualservice decode: %w", err)
	}
	return contract.DiscoveredResource{
		ExternalID: externalID(km, o.Metadata), ExternalURN: buildURN(ctxID, km, o.Metadata), DisplayName: o.Metadata.Name,
		Raw:        buildRaw(o.Metadata),
		Normalized: contract.JSONMap{},
	}, nil
}
