package kubernetes

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"ops-admin/backend/internal/infra/contract"
)

// MaxRawBytes is the §8.2 "Raw payloads have size limits" bound. 64KiB is the
// PLAN's introduced value, not a spec number (가정 A12) — the spec leaves the
// figure open. Overflow truncates Raw and stamps normalized["truncated"]=true.
const MaxRawBytes = 64 << 10

// objectMeta is the metadata subset every section decode shares.
type objectMeta struct {
	Name              string            `json:"name"`
	Namespace         string            `json:"namespace"`
	UID               string            `json:"uid"`
	CreationTimestamp string            `json:"creationTimestamp"`
	Labels            map[string]string `json:"labels"`
	// P1-A (J-P1-3) — pod Raw ownerReferences: workloadName/Type 집계 체인
	// (P1-D)이 역추적할 uid·kind·name 3성분만 보존한다.
	OwnerReferences []ownerReference `json:"ownerReferences"`
}

// ownerReference is the metadata.ownerReferences subset Raw carries.
type ownerReference struct {
	UID  string `json:"uid"`
	Kind string `json:"kind"`
	Name string `json:"name"`
}

// listMeta/listEnvelope is the Kubernetes collection shape — the continue
// token is what section paging delegates to (계획 §3.1 페이징).
type listMeta struct {
	Continue string `json:"continue"`
}

type listEnvelope struct {
	Metadata listMeta          `json:"metadata"`
	Items    []json.RawMessage `json:"items"`
}

// --- 섹션별 디코딩 객체 — legacy kube* 응답 구조와 동일 필드. ---

type nodeObject struct {
	Metadata objectMeta `json:"metadata"`
	Spec     struct {
		Taints []struct {
			Key    string `json:"key"`
			Effect string `json:"effect"`
		} `json:"taints"`
		// P1-A (J-P1-3) — podCIDRs(단일 podCIDR 폴백 포함).
		PodCIDRs []string `json:"podCIDRs"`
		PodCIDR  string   `json:"podCIDR"`
	} `json:"spec"`
	Status struct {
		Conditions []struct {
			Type   string `json:"type"`
			Status string `json:"status"`
		} `json:"conditions"`
		Capacity    map[string]string `json:"capacity"`
		Allocatable map[string]string `json:"allocatable"`
	} `json:"status"`
}

type namespaceObject struct {
	Metadata objectMeta `json:"metadata"`
	Status   struct {
		Phase string `json:"phase"`
	} `json:"status"`
}

type podObject struct {
	Metadata objectMeta `json:"metadata"`
	Status   struct {
		Phase             string `json:"phase"`
		ContainerStatuses []struct {
			RestartCount int `json:"restartCount"`
		} `json:"containerStatuses"`
	} `json:"status"`
}

type workloadObject struct {
	Metadata objectMeta `json:"metadata"`
	Spec     struct {
		Replicas *int `json:"replicas"`
		Template struct {
			Spec struct {
				Containers []struct {
					Image string `json:"image"`
				} `json:"containers"`
			} `json:"spec"`
		} `json:"template"`
	} `json:"spec"`
	Status struct {
		ReadyReplicas          int `json:"readyReplicas"`
		DesiredNumberScheduled int `json:"desiredNumberScheduled"`
		NumberReady            int `json:"numberReady"`
	} `json:"status"`
}

type serviceObject struct {
	Metadata objectMeta `json:"metadata"`
	Spec     struct {
		Type      string `json:"type"`
		ClusterIP string `json:"clusterIP"`
		Ports     []struct {
			Name     string `json:"name"`
			Port     int    `json:"port"`
			Protocol string `json:"protocol"`
		} `json:"ports"`
	} `json:"spec"`
}

type ingressObject struct {
	Metadata objectMeta `json:"metadata"`
	Spec     struct {
		Rules []struct {
			Host string `json:"host"`
		} `json:"rules"`
	} `json:"spec"`
}

type configMapObject struct {
	Metadata objectMeta        `json:"metadata"`
	Data     map[string]string `json:"data"`
}

type secretObject struct {
	Metadata objectMeta        `json:"metadata"`
	Type     string            `json:"type"`
	Data     map[string]string `json:"data"`
}

type persistentVolumeObject struct {
	Metadata objectMeta `json:"metadata"`
	Spec     struct {
		StorageClassName string            `json:"storageClassName"`
		AccessModes      []string          `json:"accessModes"`
		Capacity         map[string]string `json:"capacity"`
	} `json:"spec"`
	Status struct {
		Phase    string            `json:"phase"`
		Capacity map[string]string `json:"capacity"`
	} `json:"status"`
}

type persistentVolumeClaimObject struct {
	Metadata objectMeta `json:"metadata"`
	Spec     struct {
		StorageClassName string   `json:"storageClassName"`
		AccessModes      []string `json:"accessModes"`
		Resources        struct {
			Requests map[string]string `json:"requests"`
		} `json:"resources"`
	} `json:"spec"`
	Status struct {
		Phase    string            `json:"phase"`
		Capacity map[string]string `json:"capacity"`
	} `json:"status"`
}

type storageClassObject struct {
	Metadata    objectMeta `json:"metadata"`
	Provisioner string     `json:"provisioner"`
}

// --- 단위 정규화 (§3.2 — Ki→GB, 코어 소수) ---

const (
	kib = 1024.0
	mib = 1024.0 * 1024.0
	gib = 1024.0 * 1024.0 * 1024.0
)

// parseQuantityBytes parses a Kubernetes storage quantity (Ki/Mi/Gi/Ti or
// plain bytes) into bytes.
func parseQuantityBytes(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	lower := strings.ToLower(s)
	suffixes := []struct {
		suffix     string
		multiplier float64
	}{
		{"ki", kib}, {"mi", mib}, {"gi", gib},
		{"ti", gib * 1024}, {"k", 1000}, {"m", 1e6}, {"g", 1e9},
	}
	for _, sf := range suffixes {
		if strings.HasSuffix(lower, sf.suffix) {
			n, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(lower, sf.suffix)), 64)
			if err != nil {
				return 0, false
			}
			return n * sf.multiplier, true
		}
	}
	n, err := strconv.ParseFloat(lower, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

// quantityToGB converts a storage quantity to GB (GiB-scale, 3 decimals) —
// mapping.md 단위 규칙과 동치(T51 검증 대상).
func quantityToGB(s string) (float64, bool) {
	bytes, ok := parseQuantityBytes(s)
	if !ok {
		return 0, false
	}
	return round3(bytes / gib), true
}

// quantityToCores parses a CPU quantity ("8", "7500m") into cores.
func quantityToCores(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	if strings.HasSuffix(s, "m") {
		n, err := strconv.ParseFloat(strings.TrimSuffix(s, "m"), 64)
		if err != nil {
			return 0, false
		}
		return round3(n / 1000), true
	}
	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return round3(n), true
}

func round3(v float64) float64 { return math.Round(v*1000) / 1000 }

// --- 종별 매핑 (§3.1 kind 매핑 + J9 어휘) — mapping.md와 동치(T51 검증 대상). ---

type kindMapping struct {
	kind     string
	subtype  string
	singular string // URN 성분
}

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
	default:
		return kindMapping{}
	}
}

// buildURN assembles §8.1 "enough native scope to avoid collisions" URNs —
// 규칙은 mapping.md의 identity 표와 동치다:
//
//	node·pod   → uid 신원 (pod 이름은 재생성되므로; name은 display)
//	namespace  → name (클러스터 스코프 고유)
//	workload   → {namespace}/{kind}/{name}
//	기타 네임스페이스 소속 → {namespace}/{name}, 클러스터 스코프 → {name}
func buildURN(ctxID uint, km kindMapping, meta objectMeta) string {
	prefix := fmt.Sprintf("urn:k8s:%d:%s:", ctxID, km.singular)
	switch km.singular {
	case "node", "pod":
		return prefix + meta.UID
	case "namespace", "pv", "storageclass":
		return prefix + meta.Name
	case "workload":
		return prefix + meta.Namespace + "/" + km.subtype + "/" + meta.Name
	default:
		return prefix + meta.Namespace + "/" + meta.Name
	}
}

// externalID mirrors the URN tail — the identity key the sync upsert carries.
func externalID(km kindMapping, meta objectMeta) string {
	switch km.singular {
	case "node", "pod":
		return meta.UID
	case "namespace", "pv", "storageclass":
		return meta.Name
	case "workload":
		return meta.Namespace + "/" + km.subtype + "/" + meta.Name
	default:
		return meta.Namespace + "/" + meta.Name
	}
}

// buildRaw assembles the bounded metadata payload (identity + labels).
// Secret/configmap DATA values never enter Raw — keys only (보존 제약 #7).
func buildRaw(meta objectMeta) contract.JSONMap {
	raw := contract.JSONMap{
		"name": meta.Name,
		"uid":  meta.UID,
	}
	if meta.Namespace != "" {
		raw["namespace"] = meta.Namespace
	}
	if meta.CreationTimestamp != "" {
		raw["creationTimestamp"] = meta.CreationTimestamp
	}
	if len(meta.Labels) > 0 {
		raw["labels"] = meta.Labels
	}
	if len(meta.OwnerReferences) > 0 {
		owners := make([]ownerReference, len(meta.OwnerReferences))
		copy(owners, meta.OwnerReferences)
		raw["ownerReferences"] = owners
	}
	return raw
}

// rawLimitOverhead is the fixed envelope cost of the truncation marker
// itself ({"rawB64":"…","truncated":true} plus escape slack).
const rawLimitOverhead = 64

// applyRawLimit enforces MaxRawBytes (A12): overflow truncates Raw and stamps
// normalized["truncated"]=true. The kept prefix is base64-wrapped so the
// re-marshalled Raw is bounded by construction (raw JSON escapes could
// otherwise regrow past the limit).
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

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// normalizeSection decodes ONE section item into a DiscoveredResource —
// §14.1 매핑의 코드화. 반환 필드 집합은 mapping.md 정규화 표와 동치다(T51).
func normalizeSection(ctxID uint, section string, raw json.RawMessage) (contract.DiscoveredResource, error) {
	km := sectionKind(section)
	if km.kind == "" {
		return contract.DiscoveredResource{}, fmt.Errorf("kubernetes: unknown section %q", section)
	}

	var res contract.DiscoveredResource
	switch section {
	case "nodes":
		var o nodeObject
		if err := json.Unmarshal(raw, &o); err != nil {
			return res, fmt.Errorf("kubernetes: node decode: %w", err)
		}
		n := contract.JSONMap{}
		if v, ok := quantityToCores(o.Status.Capacity["cpu"]); ok {
			n["capacityCoresGB"] = v
		}
		if v, ok := quantityToGB(o.Status.Capacity["memory"]); ok {
			n["capacityMemoryGB"] = v
		}
		if v, ok := quantityToCores(o.Status.Allocatable["cpu"]); ok {
			n["allocatableCoresGB"] = v
		}
		if v, ok := quantityToGB(o.Status.Allocatable["memory"]); ok {
			n["allocatableMemoryGB"] = v
		}
		roles := make([]string, 0)
		for label := range o.Metadata.Labels {
			if role, ok := strings.CutPrefix(label, "node-role.kubernetes.io/"); ok && role != "" {
				roles = append(roles, role)
			}
		}
		sort.Strings(roles)
		n["roles"] = roles
		taints := make([]string, 0, len(o.Spec.Taints))
		for _, t := range o.Spec.Taints {
			taints = append(taints, t.Key+":"+t.Effect)
		}
		n["taints"] = taints
		// P1-A (J-P1-3) — podCIDRs, 단일 podCIDR 폴백. 부재 시 키 생략.
		if len(o.Spec.PodCIDRs) > 0 {
			cidrs := make([]string, len(o.Spec.PodCIDRs))
			copy(cidrs, o.Spec.PodCIDRs)
			n["podCIDRs"] = cidrs
		} else if o.Spec.PodCIDR != "" {
			n["podCIDRs"] = []string{o.Spec.PodCIDR}
		}
		n["healthState"] = nodeHealthState(o.Status.Conditions)
		res = contract.DiscoveredResource{
			ExternalID: externalID(km, o.Metadata), ExternalURN: buildURN(ctxID, km, o.Metadata),
			DisplayName: o.Metadata.Name, Raw: buildRaw(o.Metadata), Normalized: n,
		}

	case "namespaces":
		var o namespaceObject
		if err := json.Unmarshal(raw, &o); err != nil {
			return res, fmt.Errorf("kubernetes: namespace decode: %w", err)
		}
		res = contract.DiscoveredResource{
			ExternalID: externalID(km, o.Metadata), ExternalURN: buildURN(ctxID, km, o.Metadata), DisplayName: o.Metadata.Name,
			Raw:        buildRaw(o.Metadata),
			Normalized: contract.JSONMap{"phase": o.Status.Phase},
		}

	case "pods":
		var o podObject
		if err := json.Unmarshal(raw, &o); err != nil {
			return res, fmt.Errorf("kubernetes: pod decode: %w", err)
		}
		restarts := 0
		for _, cs := range o.Status.ContainerStatuses {
			restarts += cs.RestartCount
		}
		res = contract.DiscoveredResource{
			ExternalID: externalID(km, o.Metadata), ExternalURN: buildURN(ctxID, km, o.Metadata), DisplayName: o.Metadata.Name,
			Raw: buildRaw(o.Metadata),
			Normalized: contract.JSONMap{
				"phase":          o.Status.Phase,
				"lifecycleState": o.Status.Phase, // 상태 어휘: pod phase 그대로(mapping.md)
				"restartCount":   restarts,
			},
		}

	case "deployments", "statefulsets", "daemonsets", "replicasets":
		var o workloadObject
		if err := json.Unmarshal(raw, &o); err != nil {
			return res, fmt.Errorf("kubernetes: workload decode: %w", err)
		}
		res = workloadResource(ctxID, km, section, o)

	case "jobs", "cronjobs", "endpoints":
		// P1-A 확장 종 — 디코드+키는 normalizer_batch.go에 분할(J-P1-1·R-P3 선제
		// 분할). job·cronjob의 batch 고유 키는 P1-B(TestNormalizedKeySchema) 소관.
		var err error
		if res, err = normalizeBatchSection(ctxID, km, section, raw); err != nil {
			return contract.DiscoveredResource{}, err
		}

	case "services":
		var o serviceObject
		if err := json.Unmarshal(raw, &o); err != nil {
			return res, fmt.Errorf("kubernetes: service decode: %w", err)
		}
		ports := make([]any, 0, len(o.Spec.Ports))
		for _, p := range o.Spec.Ports {
			entry := map[string]any{"port": p.Port, "protocol": p.Protocol}
			if p.Name != "" {
				entry["name"] = p.Name
			}
			ports = append(ports, entry)
		}
		res = contract.DiscoveredResource{
			ExternalID: externalID(km, o.Metadata), ExternalURN: buildURN(ctxID, km, o.Metadata), DisplayName: o.Metadata.Name,
			Raw: buildRaw(o.Metadata),
			Normalized: contract.JSONMap{
				"type":  o.Spec.Type,
				"ports": ports,
			},
		}
		if o.Spec.ClusterIP != "" {
			res.Normalized["clusterIP"] = o.Spec.ClusterIP
		}

	case "ingresses":
		var o ingressObject
		if err := json.Unmarshal(raw, &o); err != nil {
			return res, fmt.Errorf("kubernetes: ingress decode: %w", err)
		}
		hosts := make([]string, 0, len(o.Spec.Rules))
		for _, r := range o.Spec.Rules {
			if r.Host != "" {
				hosts = append(hosts, r.Host)
			}
		}
		res = contract.DiscoveredResource{
			ExternalID: externalID(km, o.Metadata), ExternalURN: buildURN(ctxID, km, o.Metadata), DisplayName: o.Metadata.Name,
			Raw:        buildRaw(o.Metadata),
			Normalized: contract.JSONMap{"hosts": hosts}, // V2-only field(mapping.md)
		}

	case "configmaps":
		var o configMapObject
		if err := json.Unmarshal(raw, &o); err != nil {
			return res, fmt.Errorf("kubernetes: configmap decode: %w", err)
		}
		res = contract.DiscoveredResource{
			ExternalID: externalID(km, o.Metadata), ExternalURN: buildURN(ctxID, km, o.Metadata), DisplayName: o.Metadata.Name,
			Raw:        buildRaw(o.Metadata),
			Normalized: contract.JSONMap{"dataKeys": sortedKeys(o.Data)}, // 키 목록만 — 값 미수집
		}

	case "secrets":
		var o secretObject
		if err := json.Unmarshal(raw, &o); err != nil {
			return res, fmt.Errorf("kubernetes: secret decode: %w", err)
		}
		// §14.1 "secret metadata" — type + data 키 목록. 데이터 값은 절대
		// Raw·Normalized 어디에도 들어가지 않는다(보존 제약 #7, T42 단얫).
		res = contract.DiscoveredResource{
			ExternalID: externalID(km, o.Metadata), ExternalURN: buildURN(ctxID, km, o.Metadata), DisplayName: o.Metadata.Name,
			Raw:        buildRaw(o.Metadata),
			Normalized: contract.JSONMap{"type": o.Type, "dataKeys": sortedKeys(o.Data)},
		}

	case "persistentvolumes":
		var o persistentVolumeObject
		if err := json.Unmarshal(raw, &o); err != nil {
			return res, fmt.Errorf("kubernetes: pv decode: %w", err)
		}
		capacity := o.Spec.Capacity
		if len(capacity) == 0 {
			capacity = o.Status.Capacity
		}
		res = contract.DiscoveredResource{
			ExternalID: externalID(km, o.Metadata), ExternalURN: buildURN(ctxID, km, o.Metadata), DisplayName: o.Metadata.Name,
			Raw:        buildRaw(o.Metadata),
			Normalized: volumeNormalized(capacity, o.Spec.StorageClassName, o.Spec.AccessModes),
		}

	case "persistentvolumeclaims":
		var o persistentVolumeClaimObject
		if err := json.Unmarshal(raw, &o); err != nil {
			return res, fmt.Errorf("kubernetes: pvc decode: %w", err)
		}
		capacity := o.Status.Capacity
		if len(capacity) == 0 {
			capacity = o.Spec.Resources.Requests
		}
		res = contract.DiscoveredResource{
			ExternalID: externalID(km, o.Metadata), ExternalURN: buildURN(ctxID, km, o.Metadata), DisplayName: o.Metadata.Name,
			Raw:        buildRaw(o.Metadata),
			Normalized: volumeNormalized(capacity, o.Spec.StorageClassName, o.Spec.AccessModes),
		}

	case "storageclasses":
		var o storageClassObject
		if err := json.Unmarshal(raw, &o); err != nil {
			return res, fmt.Errorf("kubernetes: storageclass decode: %w", err)
		}
		// V2 단독 종(비교 스코프 표 — §3.5): provisioner는 V2-only normalized 필드.
		res = contract.DiscoveredResource{
			ExternalID: externalID(km, o.Metadata), ExternalURN: buildURN(ctxID, km, o.Metadata), DisplayName: o.Metadata.Name,
			Raw:        buildRaw(o.Metadata),
			Normalized: contract.JSONMap{"provisioner": o.Provisioner},
		}
	}

	res.Kind = km.kind
	res.Subtype = km.subtype
	return applyRawLimit(res), nil
}

func volumeNormalized(capacity map[string]string, storageClass string, accessModes []string) contract.JSONMap {
	n := contract.JSONMap{
		"storageClassName": storageClass,
		"accessModes":      accessModes,
	}
	if len(accessModes) == 0 {
		n["accessModes"] = []string{}
	}
	if v, ok := quantityToGB(capacity["storage"]); ok {
		n["capacityGB"] = v
	}
	return n
}

// nodeHealthState — 상태 어휘: node conditions → healthy/degraded(mapping.md).
func nodeHealthState(conditions []struct {
	Type   string `json:"type"`
	Status string `json:"status"`
}) string {
	for _, c := range conditions {
		if c.Type == "Ready" {
			if c.Status == "True" {
				return "healthy"
			}
			return "degraded"
		}
	}
	return "degraded"
}
