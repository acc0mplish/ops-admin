package kubernetes

// P 계획 J-P1-1·R-P3 — P1-A 선제 분할 파일: 신규 확장 종(batch job·cronjob +
// v2-only 보조종 endpoints)의 디코드+키. normalizer.go는 기존 종의 단일 소유를
// 유지하고 확장 종은 본 파일에 착지한다(P1-C1 gateway/httproute는 normalizer_gw.go
// 승계). 비교 스코프는 P1-C2까지 불변(§9-10 — 스코프 변경 단일 착지점).
//
// job·cronjob의 batch 고유 키(completions·parallelism·schedule·active·succeeded·
// failed — J-P1-3)는 P1-B(TestNormalizedKeySchema deliverable) 소관 — P1-A는
// workload 계열과 동일 형면으로 수집 체인에 진입시키는 것만이 스코프다.

import (
	"encoding/json"
	"fmt"

	"ops-admin/backend/internal/infra/contract"
)

// endpointsObject is the /api/v1/endpoints item subset — service.endpoints
// 집계(P1-D)의 원천. 주소 값 자체는 정규화에 진입하지 않고 ready 주소 수만
// 카운트한다(J-P1-3 "readyAddresses" — v2-only).
type endpointsObject struct {
	Metadata objectMeta `json:"metadata"`
	Subsets  []struct {
		Addresses []struct {
			IP string `json:"ip"`
		} `json:"addresses"`
	} `json:"subsets"`
}

// workloadResource normalizes one apps/batch workload-family item into the
// legacy K8sWorkloadItem 형면(replicas·readyReplicas + v2-only image).
// daemonsets만 desired/numberReady를 쓴다(job·cronjob의 batch 고유 키는 P1-B).
// job·cronjob과 workload 3종이 같은 형면을 공유하므로 batch 분할 파일에 함께
// 둔다(P1-A 선제 분할 — normalizer.go 650행 이내 유지, PC-L).
func workloadResource(ctxID uint, km kindMapping, section string, o workloadObject) contract.DiscoveredResource {
	var replicas, ready int
	if section == "daemonsets" {
		replicas = o.Status.DesiredNumberScheduled
		ready = o.Status.NumberReady
	} else {
		if o.Spec.Replicas != nil {
			replicas = *o.Spec.Replicas
		}
		ready = o.Status.ReadyReplicas
	}
	image := ""
	if len(o.Spec.Template.Spec.Containers) > 0 {
		image = o.Spec.Template.Spec.Containers[0].Image
	}
	return contract.DiscoveredResource{
		ExternalID: externalID(km, o.Metadata), ExternalURN: buildURN(ctxID, km, o.Metadata), DisplayName: o.Metadata.Name,
		Raw: buildRaw(o.Metadata),
		Normalized: contract.JSONMap{
			"replicas":      replicas,
			"readyReplicas": ready,
			// V2-only field: legacy 리스트 직렬화(K8sWorkloadItem)에 image가
			// 없어 비교 집합 밖(mapping.md dropped(v2-only)).
			"image": image,
		},
	}
}

// normalizeBatchSection decodes ONE P1-A extended section item — the
// normalizer.go switch delegates jobs·cronjobs·endpoints here. Kind/Subtype
// 스탬핑과 Raw 상한은 normalizeSection 후미가 균일 적용한다.
func normalizeBatchSection(ctxID uint, km kindMapping, section string, raw json.RawMessage) (contract.DiscoveredResource, error) {
	switch section {
	case "jobs", "cronjobs":
		// P1-A — workload 형면 재사용(spec.replicas·status.readyReplicas는 batch
		// 종에 부재 → 영값). batch 고유 키 확장은 P1-B가 본 파일에서 진행한다.
		var o workloadObject
		if err := json.Unmarshal(raw, &o); err != nil {
			return contract.DiscoveredResource{}, fmt.Errorf("kubernetes: %s decode: %w", section, err)
		}
		return workloadResource(ctxID, km, section, o), nil

	case "endpoints":
		var o endpointsObject
		if err := json.Unmarshal(raw, &o); err != nil {
			return contract.DiscoveredResource{}, fmt.Errorf("kubernetes: endpoints decode: %w", err)
		}
		ready := 0
		for _, s := range o.Subsets {
			ready += len(s.Addresses)
		}
		return contract.DiscoveredResource{
			ExternalID: externalID(km, o.Metadata), ExternalURN: buildURN(ctxID, km, o.Metadata), DisplayName: o.Metadata.Name,
			Raw:        buildRaw(o.Metadata),
			Normalized: contract.JSONMap{"readyAddresses": ready},
		}, nil
	}
	return contract.DiscoveredResource{}, fmt.Errorf("kubernetes: unknown batch section %q", section)
}
