package kubernetes

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
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
	// P1-B (J-P1-3) — storage[].namespaceScope의 annotation 원천(pv).
	Annotations map[string]string `json:"annotations"`
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
		// P1-D 조립 원천 — legacy 경보 판정(unschedulable 노드 알림,
		// calculateK8sAggregateMetrics k8s_build_net.go:512)의 원천.
		Unschedulable bool `json:"unschedulable"`
	} `json:"spec"`
	nodeStatusFields // P1-B 확장 포함 status — normalizer_batch.go (R-P3 분할)
}

type namespaceObject struct {
	Metadata objectMeta `json:"metadata"`
	Status   struct {
		Phase string `json:"phase"`
	} `json:"status"`
}

type podObject struct {
	Metadata  objectMeta `json:"metadata"`
	podFields            // P1-B 확장 포함 spec·status — normalizer_batch.go (R-P3 분할)
}

type workloadObject struct {
	Metadata objectMeta `json:"metadata"`
	Spec     struct {
		Replicas *int `json:"replicas"`
		// P1-D 조립 원천 — ownerReferences 부재 pod의 셀렉터 매칭 폴백
		// (legacy buildPodItemsWithWorkloads assignBySelector)의 원천.
		Selector *struct {
			MatchLabels map[string]string `json:"matchLabels"`
		} `json:"selector"`
		Template struct {
			Spec struct {
				Containers []containerSpec `json:"containers"`
			} `json:"spec"`
		} `json:"template"`
	} `json:"spec"`
	Status struct {
		ReadyReplicas          int `json:"readyReplicas"`
		DesiredNumberScheduled int `json:"desiredNumberScheduled"`
		NumberReady            int `json:"numberReady"`
		// P1-B (J-P1-3) — legacy workloads.updated·available.
		UpdatedReplicas   int `json:"updatedReplicas"`
		AvailableReplicas int `json:"availableReplicas"`
		// P1-D 조립 원천 — daemonset만 legacy updated·available의 원천이
		// 다르다(UpdatedNumberScheduled·NumberAvailable — buildWorkloadItems
		// k8s_build_pod.go:214-215). apps 나머지는 위 두 필드가 원천.
		UpdatedNumberScheduled int `json:"updatedNumberScheduled"`
		NumberAvailable        int `json:"numberAvailable"`
	} `json:"status"`
}

type serviceObject struct {
	Metadata      objectMeta `json:"metadata"`
	serviceFields            // P1-B 확장 포함 spec·status — normalizer_batch.go (R-P3 분할)
}

type ingressObject struct {
	Metadata      objectMeta `json:"metadata"`
	ingressFields            // P1-B 확장 포함 spec·status — normalizer_batch.go (R-P3 분할)
}

type configMapObject struct {
	Metadata objectMeta        `json:"metadata"`
	Data     map[string]string `json:"data"`
	// legacy Keys = len(Data)+len(Binary)(K8sConfigMapItem) — 바이너리는 키
	// 이름만 dataKeys 목록에 합류하고 값은 여전히 폐기된다(보존 제약 #7).
	BinaryData map[string]string `json:"binaryData"`
}

type secretObject struct {
	Metadata objectMeta        `json:"metadata"`
	Type     string            `json:"type"`
	Data     map[string]string `json:"data"`
}

type persistentVolumeObject struct {
	Metadata objectMeta `json:"metadata"`
	pvFields            // P1-B 확장 포함 spec·status — normalizer_batch.go (R-P3 분할)
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

// --- 종별 매핑 (§3.1 kind 매핑 + J9 어휘) — mapping.md와 동치(T51 검증 대상). ---

type kindMapping struct {
	kind     string
	subtype  string
	singular string // URN 성분
}

// sectionKind — P2-D 분할로 normalizer_sections.go로 옮겨졌다(섹션 → kind 표 +
// istio 앵커 디코드가 같은 봉합선이다 — R-P3 동반 분할).

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
		// 역할은 legacy joinNodeRoles(k8s_path.go:375)의 수집측 승계다: 접두
		// 라벨의 접미가 역할이되 빈 접미 키는 "worker", 역할 라벨이 아예 없는
		// 노드도 ["worker"] 1개다(TestCharJoinNodeRoles 계약 — 조립 P1-D 판단
		// 기록: 표시 어휘를 healthState·roles[] 어휘에 흡수시켰다). 중복 제거는
		// 하지 않는다 — legacy도 하지 않는다.
		roles := make([]string, 0)
		for label := range o.Metadata.Labels {
			role, ok := strings.CutPrefix(label, "node-role.kubernetes.io/")
			if !ok {
				continue
			}
			if role == "" {
				role = "worker"
			}
			roles = append(roles, role)
		}
		if len(roles) == 0 {
			roles = append(roles, "worker")
		}
		sort.Strings(roles)
		n["roles"] = roles
		taints := make([]string, 0, len(o.Spec.Taints))
		for _, t := range o.Spec.Taints {
			taints = append(taints, t.Key+":"+t.Effect)
		}
		n["taints"] = taints
		// P1-A (J-P1-3) — podCIDRs. legacy 폴백(k8s_overview.go:55)은 PodCIDRs와
		// 단일 podCIDR의 합집합을 수집하고 중복 제거는 조립 소관이다 — either/or가
		// 아니라 항상 단일 값을 뒤에 붙는다(양쪽 모두 채워진 관측에서 1개를
		// 유실하는 결함은 P1-E 승계 스왑 TestCharResolveK8sNetworkCIDRs로 검출,
		// R-P2 구현 수정 처분).
		cidrs := make([]string, 0, len(o.Spec.PodCIDRs)+1)
		cidrs = append(cidrs, o.Spec.PodCIDRs...)
		if o.Spec.PodCIDR != "" {
			cidrs = append(cidrs, o.Spec.PodCIDR)
		}
		if len(cidrs) > 0 {
			n["podCIDRs"] = cidrs
		}
		// P1-B (J-P1-3) — kubeletVersion·internalIP·osImage·allocatablePods.
		applyNodeFieldKeys(n, o)
		// P1-D 조립 원천(J-P1-9) — legacy 표시 3치 상태(Ready/NotReady/Unknown,
		// TestCharNodeReadyStatus "조건 없음 → Unknown" 계약)와 경보 판정
		// (unschedulable 노드 알림), 파드 카운트 분모(Status.Capacity["pods"] —
		// TestCharBuildNodeItems "2/110")는 healthState 단일 키로 재현 불가라
		// 상태 원천을 함께 보존한다(P1-D 판단 기록).
		for _, c := range o.Status.Conditions {
			if c.Type == "Ready" {
				n["readyCondition"] = c.Status
				break
			}
		}
		if o.Spec.Unschedulable {
			n["unschedulable"] = true
		}
		if v, ok := quantityToCount(o.Status.Capacity["pods"]); ok {
			n["capacityPods"] = v
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
		n := contract.JSONMap{
			"phase":          o.Status.Phase,
			"lifecycleState": o.Status.Phase, // 상태 어휘: pod phase 그대로(mapping.md)
			"restartCount":   restarts,
		}
		// P1-D 조립 원천 — legacy K8sPodItem.Node와 노드별 파드 카운트의
		// 원천. pod→node는 runs_on 관계로도 적재되지만 조립은 관측 행만으로
		// 순수하게 돌아야 한다(R-P8 — P1-D 판단 기록).
		if o.Spec.NodeName != "" {
			n["nodeName"] = o.Spec.NodeName
		}
		// P1-B (J-P1-3) — legacy nodeIP·ip + 컨테이너 원시량(milli/bytes).
		if o.Status.HostIP != "" {
			n["hostIP"] = o.Status.HostIP
		}
		if o.Status.PodIP != "" {
			n["podIP"] = o.Status.PodIP
		}
		if cs := containerQuantities(o.Spec.Containers, false); cs != nil {
			n["containers"] = cs
		}
		res = contract.DiscoveredResource{
			ExternalID: externalID(km, o.Metadata), ExternalURN: buildURN(ctxID, km, o.Metadata), DisplayName: o.Metadata.Name,
			Raw: buildRaw(o.Metadata), Normalized: n,
		}

	case "deployments", "statefulsets", "daemonsets", "replicasets":
		var o workloadObject
		if err := json.Unmarshal(raw, &o); err != nil {
			return res, fmt.Errorf("kubernetes: workload decode: %w", err)
		}
		res = workloadResource(ctxID, km, section, o)

	case "jobs", "cronjobs", "endpoints":
		// P1-A 확장 종 — 디코드+키는 normalizer_batch.go에 분할(J-P1-1·R-P3 선제
		// 분할). batch 고유 키 확장은 P1-B 착지(J-P1-3).
		var err error
		if res, err = normalizeBatchSection(ctxID, km, section, raw); err != nil {
			return contract.DiscoveredResource{}, err
		}

	case "gateways", "httproutes":
		// P1-C1 확장 종 — 디코드+키는 normalizer_gw.go에 분할(R-P3 재분할).
		// 비교 집합 불참(I-P5 이월 — J-P1-2).
		var err error
		if res, err = normalizeGWSection(ctxID, km, section, raw); err != nil {
			return contract.DiscoveredResource{}, err
		}

	case "virtualservices":
		// P2-D — istio 앵커(J-P1-1 — 스코프 외 종, 비교 집합 불참 R-P6).
		// 디코드는 normalizer_sections.go로 분할(R-P3 동반 — 본 파일 650행 계약).
		var err error
		if res, err = normalizeAnchorSection(ctxID, km, raw); err != nil {
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
			// P1-D 조립 원천 — legacy formatServiceListPort 표시("80:30080/TCP")
			// 의 원천. nodePort>0일 때만 싣는다(v1 표시 분기와 동일; 비교 엔진은
			// port/proto만 읽어 무영향 — compare canonicalV2Ports).
			if p.NodePort > 0 {
				entry["nodePort"] = p.NodePort
			}
			ports = append(ports, entry)
		}
		n := contract.JSONMap{
			"type":  o.Spec.Type,
			"ports": ports,
		}
		if o.Spec.ClusterIP != "" {
			n["clusterIP"] = o.Spec.ClusterIP
		}
		// P1-B (J-P1-3) — legacy externalIP(v1 serviceExternalIP 결합 의미론).
		if v := serviceExternalIP(o); v != "" {
			n["externalIP"] = v
		}
		res = contract.DiscoveredResource{
			ExternalID: externalID(km, o.Metadata), ExternalURN: buildURN(ctxID, km, o.Metadata), DisplayName: o.Metadata.Name,
			Raw: buildRaw(o.Metadata), Normalized: n,
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
		n := contract.JSONMap{"hosts": hosts} // V2-only field(mapping.md)
		// P1-B (J-P1-3) — legacy address·tls(상태 어휘 Enabled/Disabled 그대로).
		if v := ingressAddress(o); v != "" {
			n["address"] = v
		}
		if len(o.Spec.TLS) > 0 {
			n["tls"] = "Enabled"
		} else {
			n["tls"] = "Disabled"
		}
		res = contract.DiscoveredResource{
			ExternalID: externalID(km, o.Metadata), ExternalURN: buildURN(ctxID, km, o.Metadata), DisplayName: o.Metadata.Name,
			Raw: buildRaw(o.Metadata), Normalized: n,
		}

	case "configmaps":
		var o configMapObject
		if err := json.Unmarshal(raw, &o); err != nil {
			return res, fmt.Errorf("kubernetes: configmap decode: %w", err)
		}
		dataKeys := sortedKeys(o.Data)
		if len(o.BinaryData) > 0 {
			// legacy Keys = len(Data)+len(Binary)(K8sConfigMapItem) — 바이너리
			// 키 이름만 목록에 합류한다(값은 폐기 — 보존 제약 #7).
			dataKeys = append(append(make([]string, 0, len(o.Data)+len(o.BinaryData)), dataKeys...), sortedKeys(o.BinaryData)...)
			sort.Strings(dataKeys)
		}
		n := contract.JSONMap{"dataKeys": dataKeys} // 키 목록만 — 값 미수집
		// 판정 ⑤ (b) — kube-system/kubeadm-config 1종의 네트워크 위상 2키만
		// data 값에서 추출한다(사용자 승인 2026-09-10 — 보존 제약 #7의 유일
		// 예외). 확장은 리뷰 승인 전제(mapping.md §2·§6).
		if o.Metadata.Namespace == kubeadmConfigNamespace && o.Metadata.Name == kubeadmConfigName {
			applyKubeadmNetworkKeys(n, o.Data)
		}
		res = contract.DiscoveredResource{
			ExternalID: externalID(km, o.Metadata), ExternalURN: buildURN(ctxID, km, o.Metadata), DisplayName: o.Metadata.Name,
			Raw: buildRaw(o.Metadata), Normalized: n,
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
		n := volumeNormalized(capacity, o.Spec.StorageClassName, o.Spec.AccessModes)
		// P1-D 조립 원천 — legacy K8sStorageItem.Capacity(pv) 표시는 status
		// 우선 원문량 문자열(k8s_build_net.go:477). GB 역변환은 원문을 복원하지
		// 못하므로("5Gi" vs "5120Mi") 표시 원천을 별도 보존한다(P1-D 판단 기록).
		if v := firstQuantityText(o.Status.Capacity["storage"], o.Spec.Capacity["storage"]); v != "" {
			n["capacityRaw"] = v
		}
		// P1-B (J-P1-3) — phase·namespaceScope(annotation)·sourceType·
		// sourcePath·nfsServer·reclaimPolicy — legacy는 pv 행에만 채운다.
		applyVolumeFieldKeys(n, o)
		res = contract.DiscoveredResource{
			ExternalID: externalID(km, o.Metadata), ExternalURN: buildURN(ctxID, km, o.Metadata), DisplayName: o.Metadata.Name,
			Raw: buildRaw(o.Metadata), Normalized: n,
		}

	case "persistentvolumeclaims":
		var o persistentVolumeClaimObject
		if err := json.Unmarshal(raw, &o); err != nil {
			return res, fmt.Errorf("kubernetes: pvc decode: %w", err)
		}
		// legacy K8sStorageItem.Capacity(pvc)는 requests 우선("Prefer requested
		// capacity" 주석 — k8s_build_net.go:463)이고 비교 엔진의 legacy측 원천도
		// 그 표시 문자열이므로 capacityGB·capacityRaw 모두 동일 우선순위로 승계
		// 한다(P1-D 판단 기록 — 기존 status 우선은 표시 원문과 갈리는 우선순위).
		capacity := o.Spec.Resources.Requests
		if len(capacity) == 0 {
			capacity = o.Status.Capacity
		}
		n := volumeNormalized(capacity, o.Spec.StorageClassName, o.Spec.AccessModes)
		// legacy pvc 행 Capacity 표시 원문량 — requests 우선(위와 동일 근거).
		if v := firstQuantityText(o.Spec.Resources.Requests["storage"], o.Status.Capacity["storage"]); v != "" {
			n["capacityRaw"] = v
		}
		// P1-B (J-P1-3) — pvc는 phase만(legacy pvc 행의 source·reclaimPolicy·
		// namespaceScope는 공란 — 조립 P1-D가 행 형상을 소유).
		if o.Status.Phase != "" {
			n["phase"] = o.Status.Phase
		}
		res = contract.DiscoveredResource{
			ExternalID: externalID(km, o.Metadata), ExternalURN: buildURN(ctxID, km, o.Metadata), DisplayName: o.Metadata.Name,
			Raw: buildRaw(o.Metadata), Normalized: n,
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
