package tencent

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"ops-admin/backend/internal/infra/contract"
)

// KindVM is the discoverable resource kind (계획 판정 J3 — vm family 한정;
// M1ResourceKinds에 이미 존재 — 어휘 확장 0).
const KindVM = "compute.vm"

// MaxRawBytes is the §8.2 "Raw payloads have size limits" bound — the same
// 64KiB plan-introduced figure the kubernetes adapter applies (가정 A12).
// Overflow truncates Raw and stamps normalized["truncated"]=true.
const MaxRawBytes = 64 << 10

// rawLimitOverhead — truncation 봉투의 고정 오버헤드(kubernetes 선례 승계).
const rawLimitOverhead = 64

// cvmPlacement / cvmDisk / cvmInstance — CVM DescribeInstances InstanceSet
// 항목의 와이어 형상. Tencent CVM은 포인터 필드가 전부라서(R8) nil-safe 전면
// 디레퍼런스가 계약이다 — 어떤 필드가 빠져도 파닉·과잉 수집 없이 정규화된다.
//
// Tags는 의도적으로 디코드하지 않는다(매핑 표 §4 — tag 값은 어댑터 경계에서
// 폐기): 태그 값에 사용자 시크릿이 실리는 사례가 있어 Raw 진입을 차단한다
// (보존 제약 3 — 하네스 리댁션 캐나리 T-B13이 지키는 경계).
type cvmPlacement struct {
	Zone   *string `json:"Zone"`
	Region *string `json:"Region"`
}

type cvmDisk struct {
	DiskSize *int64  `json:"DiskSize"`
	DiskType *string `json:"DiskType"`
}

type cvmInstance struct {
	InstanceId    *string       `json:"InstanceId"`
	InstanceName  *string       `json:"InstanceName"`
	InstanceType  *string       `json:"InstanceType"`
	InstanceState *string       `json:"InstanceState"`
	Cpu           *int64        `json:"Cpu"`
	Memory        *int64        `json:"Memory"` // API 단위 GB (CVM DescribeInstances)
	OsName        *string       `json:"OsName"`
	Placement     *cvmPlacement `json:"Placement"`
	// PrivateIpAddresses/PublicIpAddresses — Primary 주소가 첫 성분.
	PrivateIpAddresses []*string `json:"PrivateIpAddresses"`
	PublicIpAddresses  []*string `json:"PublicIpAddresses"`
	SystemDisk         *cvmDisk  `json:"SystemDisk"`
	DataDisks          []cvmDisk `json:"DataDisks"`
}

// deref — nil-safe 문자열.
func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefInt(i *int64) int64 {
	if i == nil {
		return 0
	}
	return *i
}

// familyOf derives the instance family (§3.2 Subtype): "S5.LARGE8" → "S5",
// 그 외(빈 값·점 없음)는 "".
func familyOf(instanceType string) string {
	for i := 0; i < len(instanceType); i++ {
		if instanceType[i] == '.' {
			return instanceType[:i]
		}
	}
	return instanceType
}

// ipList — *string 목록을 빈 성분 없는 []string로.
func ipList(addrs []*string) []string {
	out := make([]string, 0, len(addrs))
	for _, a := range addrs {
		if a != nil && *a != "" {
			out = append(out, *a)
		}
	}
	return out
}

// diskGB — system + data 디스크 합계(legacy util/tencentcloud.go와 동일 집계
// 규칙 — nil DiskSize 스킵).
func diskGB(system *cvmDisk, data []cvmDisk) int64 {
	var total int64
	if system != nil {
		total += derefInt(system.DiskSize)
	}
	for _, d := range data {
		total += derefInt(d.DiskSize)
	}
	return total
}

// zoneOf / regionOf — Placement 래퍼까지 nil-safe한 접근(R8).
func zoneOf(p *cvmPlacement) string {
	if p == nil {
		return ""
	}
	return deref(p.Zone)
}

func regionOf(p *cvmPlacement) string {
	if p == nil {
		return ""
	}
	return deref(p.Region)
}

// normalizeInstance decodes ONE CVM instance into a DiscoveredResource —
// §14.2 매핑의 코드화(mapping.md와 1:1). region은 계정 리전(호출자 제공)이
// 기본이고 Placement.Region이 있으면 그것을 따른다.
func normalizeInstance(ctxID uint, region string, inst cvmInstance) (contract.DiscoveredResource, error) {
	id := deref(inst.InstanceId)
	if id == "" {
		return contract.DiscoveredResource{}, fmt.Errorf("tencent: instance without InstanceId — identity는 URN 필수 성분이다")
	}

	displayName := deref(inst.InstanceName)
	if displayName == "" {
		displayName = id
	}
	if placed := regionOf(inst.Placement); placed != "" {
		region = placed
	}

	res := contract.DiscoveredResource{
		ExternalID:  id,
		ExternalURN: fmt.Sprintf("urn:%s:%d:%s:%s", ProviderName, ctxID, KindVM, id),
		Kind:        KindVM,
		Subtype:     familyOf(deref(inst.InstanceType)),
		DisplayName: displayName,
		// Raw는 최소 필드 집합만(매핑 표 §4) — Tags·자격·시크릿 무.
		Raw: contract.JSONMap{
			"instanceId":    id,
			"instanceName":  displayName,
			"instanceType":  deref(inst.InstanceType),
			"instanceState": deref(inst.InstanceState),
			"zone":          zoneOf(inst.Placement),
			"region":        region,
			"cpu":           derefInt(inst.Cpu),
			"memory":        derefInt(inst.Memory),
			"osName":        deref(inst.OsName),
		},
		Normalized: contract.JSONMap{
			"displayName":  displayName,
			"region":       region,
			"zone":         zoneOf(inst.Placement),
			"cpu":          float64(derefInt(inst.Cpu)),
			"memoryGB":     float64(derefInt(inst.Memory)),
			"diskGB":       float64(diskGB(inst.SystemDisk, inst.DataDisks)),
			"os":           deref(inst.OsName),
			"privateIps":   ipList(inst.PrivateIpAddresses),
			"publicIps":    ipList(inst.PublicIpAddresses),
			"instanceType": deref(inst.InstanceType),
			"status":       deref(inst.InstanceState),
			// §3.2 — sshHint는 프로바이더 기본 힌트(표시 보조)이지 legacy
			// SSHUser/SSHPort 관측값의 매핑 대상이 아니다(mapping.md §4 dropped).
			"sshHint": contract.JSONMap{"user": "root", "port": 22},
		},
	}
	return applyRawLimit(res), nil
}

// applyRawLimit — Raw 직렬화가 64KiB를 넘으면 base64 접두 절단 보존 +
// normalized.truncated 마커(kubernetes normalizer와 동일 규약).
func applyRawLimit(res contract.DiscoveredResource) contract.DiscoveredResource {
	b, err := json.Marshal(res.Raw)
	if err == nil && len(b) > MaxRawBytes {
		cut := ((MaxRawBytes - rawLimitOverhead) / 4) * 3
		if cut > len(b) {
			cut = len(b)
		}
		res.Raw = contract.JSONMap{
			"truncated": true,
			"rawB64":    base64.StdEncoding.EncodeToString(b[:cut]),
		}
		if res.Normalized == nil {
			res.Normalized = contract.JSONMap{}
		}
		res.Normalized["truncated"] = true
	}
	return res
}
