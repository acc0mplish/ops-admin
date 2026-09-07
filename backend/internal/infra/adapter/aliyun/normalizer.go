package aliyun

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"ops-admin/backend/internal/infra/contract"
)

// MaxRawBytes is the §8.2 "Raw payloads have size limits" bound — the same
// 64KiB figure the kubernetes adapter introduced (가정 A12). Overflow truncates
// Raw and stamps normalized["truncated"]=true.
const MaxRawBytes = 64 << 10

// aliyunECSInstance is the DescribeInstances.Instance element — the legacy
// struct (service/asset_cloud_aliyun.go) plus the fields V2 normalization
// consumes (ZoneId·Status·InstanceType). nil-safe by design: every field
// decodes to its zero value when absent (mock i-mock-3·R8 대응).
type aliyunECSInstance struct {
	InstanceID      string `json:"InstanceId"`
	InstanceName    string `json:"InstanceName"`
	InstanceType    string `json:"InstanceType"`
	Status          string `json:"Status"`
	CPU             int    `json:"Cpu"`
	Memory          int    `json:"Memory"`
	OSName          string `json:"OSName"`
	RegionID        string `json:"RegionId"`
	ZoneID          string `json:"ZoneId"`
	PublicIPAddress struct {
		IPAddress []string `json:"IpAddress"`
	} `json:"PublicIpAddress"`
	InnerIPAddress struct {
		IPAddress []string `json:"IpAddress"`
	} `json:"InnerIpAddress"`
	EIPAddresses struct {
		IPAddress string `json:"IpAddress"`
	} `json:"EipAddress"`
	VPCAttributes struct {
		PrivateIPAddress struct {
			IPAddress []string `json:"IpAddress"`
		} `json:"PrivateIpAddress"`
	} `json:"VpcAttributes"`
	NetworkInterfaces struct {
		NetworkInterface []struct {
			PrimaryIPAddress string `json:"PrimaryIpAddress"`
			PrivateIPSets    struct {
				PrivateIPSet []struct {
					PrivateIPAddress string `json:"PrivateIpAddress"`
				} `json:"PrivateIpSet"`
			} `json:"PrivateIpSets"`
		} `json:"NetworkInterface"`
	} `json:"NetworkInterfaces"`
	SystemDisk struct {
		Size int `json:"Size"`
	} `json:"SystemDisk"`
	DataDisks struct {
		Disk []struct {
			Size int `json:"Size"`
		} `json:"Disk"`
	} `json:"DataDisks"`
}

// normalizeInstance decodes one DescribeInstances.Instance element into a
// DiscoveredResource — §14.2 vm 매핑의 코드화(mapping.md와 1:1). 반환 필드
// 집합은 mapping.md 정규화 표와 동치다. legacy SSHUser/SSHPort는 display-only로
// dropped(§14.2 — J7)이고 sshHint는 프로바이더 기본 힌트(관측값 아님)다.
func normalizeInstance(ctxID uint, raw json.RawMessage, fallbackRegion string) (contract.DiscoveredResource, error) {
	var item aliyunECSInstance
	if err := json.Unmarshal(raw, &item); err != nil {
		return contract.DiscoveredResource{}, fmt.Errorf("aliyun: instance decode: %w", err)
	}
	id := strings.TrimSpace(item.InstanceID)
	if id == "" {
		return contract.DiscoveredResource{}, fmt.Errorf("aliyun: instance decode: empty InstanceId")
	}

	displayName := strings.TrimSpace(item.InstanceName)
	if displayName == "" {
		displayName = id
	}

	privateIPs := append([]string{},
		firstNonEmptyIPs(item.VPCAttributes.PrivateIPAddress.IPAddress, item.InnerIPAddress.IPAddress)...)
	if len(item.NetworkInterfaces.NetworkInterface) > 0 {
		nic := item.NetworkInterfaces.NetworkInterface[0]
		if v := strings.TrimSpace(nic.PrimaryIPAddress); v != "" {
			privateIPs = append(privateIPs, v)
		}
		for _, set := range nic.PrivateIPSets.PrivateIPSet {
			if v := strings.TrimSpace(set.PrivateIPAddress); v != "" {
				privateIPs = append(privateIPs, v)
			}
		}
	}
	publicIPs := firstNonEmptyIPs(item.PublicIPAddress.IPAddress)
	if v := strings.TrimSpace(item.EIPAddresses.IPAddress); v != "" {
		publicIPs = append(publicIPs, v)
	}

	diskGB := item.SystemDisk.Size
	for _, disk := range item.DataDisks.Disk {
		diskGB += disk.Size
	}

	// Normalized는 식별·용량·상태만 — 자격·서명 성분은 구조적으로 진입 불가
	// (§3.1 redaction, 하네스 단얫 4).
	res := contract.DiscoveredResource{
		ExternalID:  id,
		ExternalURN: fmt.Sprintf("urn:%s:%d:%s:%s", ProviderName, ctxID, "compute.vm", id),
		Kind:        "compute.vm",
		Subtype:     instanceFamily(item.InstanceType),
		DisplayName: displayName,
		Raw: contract.JSONMap{
			"instanceId":   id,
			"instanceName": displayName,
			"regionId":     firstNonEmpty(item.RegionID, fallbackRegion),
			"zoneId":       item.ZoneID,
			"instanceType": item.InstanceType,
			"status":       item.Status,
		},
		Normalized: contract.JSONMap{
			"displayName":  displayName,
			"region":       firstNonEmpty(item.RegionID, fallbackRegion),
			"zone":         item.ZoneID,
			"cpu":          item.CPU,
			"memoryGB":     float64(item.Memory) / 1024.0,
			"diskGB":       diskGB,
			"os":           item.OSName,
			"privateIps":   dedupeIPs(privateIPs),
			"publicIps":    dedupeIPs(publicIPs),
			"instanceType": item.InstanceType,
			"status":       item.Status,
			"sshHint":      map[string]any{"user": "root", "port": 22},
		},
	}
	return applyRawLimit(res), nil
}

// instanceFamily extracts the instance family from an instance type
// ("ecs.g6.large" → "g6"); without a type the subtype stays "" (§3.2).
func instanceFamily(instanceType string) string {
	family := strings.TrimSpace(instanceType)
	family = strings.TrimPrefix(family, "ecs.")
	if family == "" {
		return ""
	}
	if idx := strings.IndexByte(family, '.'); idx >= 0 {
		family = family[:idx]
	}
	return family
}

// firstNonEmptyIPs keeps the non-blank trimmed values across the given lists,
// preserving order (legacy firstAliyunIPAddress의 리스트 확장).
func firstNonEmptyIPs(lists ...[]string) []string {
	out := make([]string, 0)
	for _, list := range lists {
		for _, value := range list {
			if v := strings.TrimSpace(value); v != "" {
				out = append(out, v)
			}
		}
	}
	return out
}

func dedupeIPs(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, v := range values {
		if seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

// rawLimitOverhead is the fixed envelope cost of the truncation marker itself
// ({"rawB64":"…","truncated":true} plus escape slack) — k8s 어댑터와 동일 산식.
const rawLimitOverhead = 64

// applyRawLimit enforces MaxRawBytes: overflow truncates Raw and stamps
// normalized["truncated"]=true. The kept prefix is base64-wrapped so the
// re-marshalled Raw is bounded by construction.
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
