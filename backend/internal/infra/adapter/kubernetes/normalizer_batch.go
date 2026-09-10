package kubernetes

// P1 수집 확장의 분할 파일(R-P3 선제 분할 — normalizer.go 650행 계약 유지).
// 소관은 둘이다:
//
//  1. P1-A 확장 종(batch job·cronjob + v2-only 보조종 endpoints)의 디코드+키.
//  2. P1-B(J-P1-3) 신규 normalized 키 19건의 빌더 — 기존 종(node·pod·workload·
//     service·ingress·pv)의 디코드 객체는 normalizer.go에 두되, P1-B 확장 필드는
//     익명 임베드 companion(nodeStatusFields·podFields·serviceFields·
//     ingressFields·pvFields)으로 본 파일에 둔다 — encoding/json이 임베드 필드를
//     승격하므로 디코드·접근 모두 동일하다.
//
// 비교 스코프는 P1-C2까지 불변(§9-10 — 스코프 변경 단일 착지점).

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"ops-admin/backend/internal/infra/contract"
)

// --- P1-B 공통 원천 타입 ---

// containerSpec is the spec.containers subset pod·workload·batch 섹션이 함께
// 디코드하는 원천 — J-P1-3 컨테이너 원시량(milli/bytes)의 근거.
type containerSpec struct {
	Name      string `json:"name"`
	Image     string `json:"image"`
	Resources struct {
		Requests map[string]string `json:"requests"`
		Limits   map[string]string `json:"limits"`
	} `json:"resources"`
}

// --- P1-B 임베드 companion — 디코드 객체의 P1-B 확장 필드(json 승격) ---

// nodeStatusFields is nodeObject's status — P1-B 원천(kubeletVersion·osImage·
// addresses·allocatable["pods"])을 기존 conditions·capacity와 한 몸으로 둔다.
type nodeStatusFields struct {
	Status struct {
		Conditions []struct {
			Type   string `json:"type"`
			Status string `json:"status"`
		} `json:"conditions"`
		Capacity    map[string]string `json:"capacity"`
		Allocatable map[string]string `json:"allocatable"`
		NodeInfo    struct {
			KubeletVersion string `json:"kubeletVersion"`
			OSImage        string `json:"osImage"`
		} `json:"nodeInfo"`
		Addresses []struct {
			Type    string `json:"type"`
			Address string `json:"address"`
		} `json:"addresses"`
	} `json:"status"`
}

// podFields is podObject's spec·status — P1-B 원천(hostIP·podIP·컨테이너
// 원시량)과 P1-A 이전의 phase·restartCount 원천을 함께 옮겨 왔다.
type podFields struct {
	Spec struct {
		Containers []containerSpec `json:"containers"`
		// P1-D 조립 원천 — legacy K8sPodItem.Node·노드별 파드 카운트의 원천
		// (조립은 관측 행만으로 순수하게 돈다 — R-P8, P1-D 판단 기록).
		NodeName string `json:"nodeName"`
	} `json:"spec"`
	Status struct {
		Phase             string `json:"phase"`
		HostIP            string `json:"hostIP"` // legacy nodeIP
		PodIP             string `json:"podIP"`  // legacy ip
		ContainerStatuses []struct {
			RestartCount int `json:"restartCount"`
		} `json:"containerStatuses"`
	} `json:"status"`
}

// serviceFields is serviceObject's spec·status — externalIP 결합원
// (spec.externalIPs + status.loadBalancer.ingress)을 옮겨 왔다.
type serviceFields struct {
	Spec struct {
		Type        string   `json:"type"`
		ClusterIP   string   `json:"clusterIP"`
		ExternalIPs []string `json:"externalIPs"`
		Ports       []struct {
			Name     string `json:"name"`
			Port     int    `json:"port"`
			Protocol string `json:"protocol"`
			// P1-D 조립 원천 — legacy formatServiceListPort("80:30080/TCP")의
			// 원천. 조립이 0보다 클 때만 표시에 반영한다(v1 분기 동일).
			NodePort int `json:"nodePort"`
		} `json:"ports"`
	} `json:"spec"`
	Status struct {
		LoadBalancer struct {
			Ingress []struct {
				IP       string `json:"ip"`
				Hostname string `json:"hostname"`
			} `json:"ingress"`
		} `json:"loadBalancer"`
	} `json:"status"`
}

// ingressFields is ingressObject's spec·status — legacy address(LB status)와
// tls(spec.tls 유무)의 원천.
type ingressFields struct {
	Spec struct {
		Rules []struct {
			Host string `json:"host"`
		} `json:"rules"`
		TLS []struct {
			Hosts []string `json:"hosts"`
		} `json:"tls"`
	} `json:"spec"`
	Status struct {
		LoadBalancer struct {
			Ingress []struct {
				IP       string `json:"ip"`
				Hostname string `json:"hostname"`
			} `json:"ingress"`
		} `json:"loadBalancer"`
	} `json:"status"`
}

// volumeSource is one spec.persistentVolumeSource member — legacy
// persistentVolumeSource의 nil-분기와 동일한 포인터 의미론.
type volumeSource struct {
	Path   string `json:"path"`
	Server string `json:"server"`
}

// pvFields is persistentVolumeObject's spec·status — P1-B 원천(phase·
// namespaceScope annotation 결합용은 objectMeta.Annotations)을 옮겨 왔다.
type pvFields struct {
	Spec struct {
		StorageClassName string            `json:"storageClassName"`
		AccessModes      []string          `json:"accessModes"`
		Capacity         map[string]string `json:"capacity"`
		HostPath         *volumeSource     `json:"hostPath"`
		NFS              *volumeSource     `json:"nfs"`
		ReclaimPolicy    string            `json:"persistentVolumeReclaimPolicy"`
	} `json:"spec"`
	Status struct {
		Phase    string            `json:"phase"`
		Capacity map[string]string `json:"capacity"`
	} `json:"status"`
}

// --- P1-B 원시량 파서 — v1 service 오라클(k8s_path.go)과 동치 ---

// parseCPUToMilli mirrors legacy parseCPUToMilli — "500m"→500, "2"→2000.
// 파싱 실패는 0(legacy 무시 의미론 그대로 — Z 오라클 대응).
func parseCPUToMilli(value string) int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if strings.HasSuffix(value, "m") {
		parsed, _ := strconv.ParseFloat(strings.TrimSuffix(value, "m"), 64)
		return int64(parsed)
	}
	parsed, _ := strconv.ParseFloat(value, 64)
	return int64(parsed * 1000)
}

// parseBytesQuantity mirrors legacy parseBytesQuantity — Ki/Mi/Gi/Ti/Pi와
// SI K/M/G/T 접미(대소문자 구분)·무접미 바이트. 파싱 실패는 0.
func parseBytesQuantity(value string) int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	suffixes := []struct {
		suffix     string
		multiplier float64
	}{
		{"Ki", 1024}, {"Mi", 1024 * 1024}, {"Gi", 1024 * 1024 * 1024},
		{"Ti", 1024 * 1024 * 1024 * 1024}, {"Pi", 1024 * 1024 * 1024 * 1024 * 1024},
		{"K", 1000}, {"M", 1000 * 1000}, {"G", 1000 * 1000 * 1000}, {"T", 1000 * 1000 * 1000 * 1000},
	}
	for _, sf := range suffixes {
		if strings.HasSuffix(value, sf.suffix) {
			parsed, _ := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(value, sf.suffix)), 64)
			return int64(parsed * sf.multiplier)
		}
	}
	parsed, _ := strconv.ParseFloat(value, 64)
	return int64(parsed)
}

// quantityToCount parses a plain count quantity ("110" — allocatable pods).
// 개수형은 실패 시 키를 생략한다(원시량 파서와 달리 값이 없으면 관측이 없다).
func quantityToCount(s string) (int, bool) {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0, false
	}
	return n, true
}

// quantityPair extracts {cpuMilli, memBytes} — 키는 값이 관측될 때만 싣는다
// (legacy formatWorkloadResourceSummary의 hasCPU·hasMemory 분기와 동치 —
// 조립 P1-D의 포맷터 오라클이 "-" 포맷을 소유한다).
func quantityPair(values map[string]string) contract.JSONMap {
	pair := contract.JSONMap{}
	if v := strings.TrimSpace(values["cpu"]); v != "" {
		pair["cpuMilli"] = parseCPUToMilli(v)
	}
	if v := strings.TrimSpace(values["memory"]); v != "" {
		pair["memBytes"] = parseBytesQuantity(v)
	}
	return pair
}

// containerQuantities projects containers into the J-P1-3 raw-quantity schema
// — [{name, (image), requests{cpuMilli, memBytes}, limits{…}}]. 원시량은
// **milli·bytes 정수**로 저장한다(round3 실수 금지 — 포맷 시 왕복 손실 방지).
// withImage는 workload·batch 컨테이너의 image 성분(pod는 미포함).
func containerQuantities(cs []containerSpec, withImage bool) []any {
	if len(cs) == 0 {
		return nil
	}
	out := make([]any, 0, len(cs))
	for _, c := range cs {
		entry := contract.JSONMap{
			"name":     c.Name,
			"requests": quantityPair(c.Resources.Requests),
			"limits":   quantityPair(c.Resources.Limits),
		}
		if withImage {
			entry["image"] = c.Image
		}
		out = append(out, entry)
	}
	return out
}

// --- P1-B 키 빌더 — 종별 착지(mapping.md §4와 1:1) ---

// applyNodeFieldKeys lands kubeletVersion·internalIP·osImage·allocatablePods
// (podCIDRs는 P1-A 착지). 주소·버전은 관측 부재 시 키 생략 — "-" 포맷은 조립
// (P1-D) 소유.
func applyNodeFieldKeys(n contract.JSONMap, o nodeObject) {
	if v := o.Status.NodeInfo.KubeletVersion; v != "" {
		n["kubeletVersion"] = v
	}
	if v := o.Status.NodeInfo.OSImage; v != "" {
		n["osImage"] = v
	}
	for _, a := range o.Status.Addresses {
		if a.Type == "InternalIP" && strings.TrimSpace(a.Address) != "" {
			n["internalIP"] = strings.TrimSpace(a.Address)
			break
		}
	}
	if v, ok := quantityToCount(o.Status.Allocatable["pods"]); ok {
		n["allocatablePods"] = v
	}
}

// serviceExternalIP mirrors legacy serviceExternalIP — spec.externalIPs와
// status.loadBalancer.ingress(IP 우선, hostname 대체)를 ", "로 결합. 계획
// J-P1-3 원천란의 "status.loadBalancer.ingress"는 지배 원천 표기이며, Z 오라클
// 동치를 위해 legacy 결합원 전체(spec.externalIPs)를 포함한다. 공백이면 키
// 생략 — "<none>" 센티넬은 조립(P1-D) 포맷.
func serviceExternalIP(o serviceObject) string {
	ingress := o.Status.LoadBalancer.Ingress
	values := make([]string, 0, len(o.Spec.ExternalIPs)+len(ingress))
	for _, v := range o.Spec.ExternalIPs {
		if v = strings.TrimSpace(v); v != "" && v != "-" {
			values = append(values, v)
		}
	}
	for _, item := range ingress {
		v := strings.TrimSpace(item.IP)
		if v == "" {
			v = strings.TrimSpace(item.Hostname)
		}
		if v != "" && v != "-" {
			values = append(values, v)
		}
	}
	return strings.Join(values, ", ")
}

// ingressAddress mirrors legacy — status.loadBalancer.ingress[0]의 IP 우선
// 값. 부재 시 키 생략(legacy "-" 포맷은 조립 소유).
func ingressAddress(o ingressObject) string {
	if len(o.Status.LoadBalancer.Ingress) == 0 {
		return ""
	}
	first := o.Status.LoadBalancer.Ingress[0]
	if v := strings.TrimSpace(first.IP); v != "" {
		return v
	}
	return strings.TrimSpace(first.Hostname)
}

// namespaceScopeAnnotation is the platform-level PVC scope annotation the
// legacy storageNamespaceScope reads.
const namespaceScopeAnnotation = "ops-admin.io/namespace-scope"

// applyVolumeFieldKeys lands the J-P1-3 volume fields. legacy는 pv 행에만
// sourceType·sourcePath·nfsServer·reclaimPolicy·namespaceScope를 채운다(pvc
// 행은 공란 — pvc는 phase만 싣는다). sourceType은 hostPath→NFS 순 검사로
// legacy persistentVolumeSource 분기와 동치.
func applyVolumeFieldKeys(n contract.JSONMap, o persistentVolumeObject) {
	if o.Status.Phase != "" {
		n["phase"] = o.Status.Phase
	}
	if scope := strings.TrimSpace(o.Metadata.Annotations[namespaceScopeAnnotation]); scope != "" {
		n["namespaceScope"] = scope
	} else {
		n["namespaceScope"] = "Cluster-scoped"
	}
	if o.Spec.HostPath != nil {
		// legacy persistentVolumeSource 우선순위: hostPath가 있으면 즉시
		// return — NFS는 else if로 (둘 다 있는 입력에서 V2가 NFS로 덮어쓰는
		// 비동치를 막는다, P1-B 리뷰 LOW #1).
		n["sourceType"] = "hostPath"
		if o.Spec.HostPath.Path != "" {
			n["sourcePath"] = o.Spec.HostPath.Path
		}
	} else if o.Spec.NFS != nil {
		n["sourceType"] = "NFS"
		n["sourcePath"] = o.Spec.NFS.Path
		n["nfsServer"] = o.Spec.NFS.Server
	}
	if o.Spec.ReclaimPolicy != "" {
		n["reclaimPolicy"] = o.Spec.ReclaimPolicy
	}
}

// --- P1-A 확장 종 디코드 + P1-B batch 고유 키 ---

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

// batchWorkloadObject is the job·cronjob decode — J-P1-3 batch 고유 키
// (completions·parallelism·schedule·active·succeeded·failed)의 원천. cronjob은
// containers·completions·parallelism이 jobTemplate 경계 아래 있다.
type batchWorkloadObject struct {
	Metadata objectMeta `json:"metadata"`
	Spec     struct {
		Completions *int   `json:"completions"`
		Parallelism *int   `json:"parallelism"`
		Schedule    string `json:"schedule"`
		// P1-D 조립 원천 — job의 셀렉터 매칭 폴백(legacy buildPodItemsWithWorkloads
		// assignBySelector — job은 selector 부재 시 {"job-name": name} 합성이
		// 조립 소유)과 cronjob의 suspend("Suspended" 표시 원천 — cronJobReadyText).
		Selector *struct {
			MatchLabels map[string]string `json:"matchLabels"`
		} `json:"selector"`
		Suspend  *bool `json:"suspend"`
		Template struct {
			Spec struct {
				Containers []containerSpec `json:"containers"`
			} `json:"spec"`
		} `json:"template"`
		JobTemplate struct {
			Spec struct {
				Completions *int `json:"completions"`
				Parallelism *int `json:"parallelism"`
				Template    struct {
					Spec struct {
						Containers []containerSpec `json:"containers"`
					} `json:"spec"`
				} `json:"template"`
			} `json:"spec"`
		} `json:"jobTemplate"`
	} `json:"spec"`
	Status struct {
		// Active는 job(정수)과 cronjob(JobReference 배열) 형상이 갈린다 —
		// activeCount가 동일 개수로 통일한다.
		Active    json.RawMessage `json:"active"`
		Succeeded int             `json:"succeeded"`
		Failed    int             `json:"failed"`
	} `json:"status"`
}

// activeCount unifies job status.active(정수)와 cronjob status.active(배열)를
// 실행 개수로 — legacy buildWorkloadItems의 len(Status.Active) 동치.
func activeCount(raw json.RawMessage) (int, bool) {
	if len(raw) == 0 {
		return 0, false
	}
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err == nil {
		return len(items), true
	}
	var n int
	if err := json.Unmarshal(raw, &n); err == nil {
		return n, true
	}
	return 0, false
}

// normalizeBatchSection decodes ONE P1-A extended section item — the
// normalizer.go switch delegates jobs·cronjobs·endpoints here. Kind/Subtype
// 스탬핑과 Raw 상한은 normalizeSection 후미가 균일 적용한다.
func normalizeBatchSection(ctxID uint, km kindMapping, section string, raw json.RawMessage) (contract.DiscoveredResource, error) {
	switch section {
	case "jobs", "cronjobs":
		var o batchWorkloadObject
		if err := json.Unmarshal(raw, &o); err != nil {
			return contract.DiscoveredResource{}, fmt.Errorf("kubernetes: %s decode: %w", section, err)
		}
		return batchWorkloadResource(ctxID, km, section, o), nil

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

// batchWorkloadResource normalizes one job·cronjob item. P1-A 형면(replicas·
// readyReplicas·image)을 유지하되 batch 고유 키를 P1-B로 확장한다 — succeeded·
// failed는 job status 전용, schedule은 cronjob 전용이며 구조적 부재는 키
// 생략으로 구분한다. updatedReplicas·availableReplicas는 batch 종에 존재하지
// 않는다(legacy는 active·succeeded로 Updated/Available을 파생 — 조립 P1-D).
func batchWorkloadResource(ctxID uint, km kindMapping, section string, o batchWorkloadObject) contract.DiscoveredResource {
	containers := o.Spec.Template.Spec.Containers
	completions, parallelism := o.Spec.Completions, o.Spec.Parallelism
	if section == "cronjobs" {
		containers = o.Spec.JobTemplate.Spec.Template.Spec.Containers
		completions = o.Spec.JobTemplate.Spec.Completions
		parallelism = o.Spec.JobTemplate.Spec.Parallelism
	}
	image := ""
	if len(containers) > 0 {
		image = containers[0].Image
	}
	n := contract.JSONMap{
		// batch 종에 replicas·readyReplicas는 구조적 부재 → 영값(P1-A 형면).
		"replicas":      0,
		"readyReplicas": 0,
		// V2-only field: legacy 리스트 직렬화(K8sWorkloadItem)에 image가 없어
		// 비교 집합 밖(mapping.md dropped(v2-only)).
		"image": image,
	}
	if completions != nil {
		n["completions"] = *completions
	}
	if parallelism != nil {
		n["parallelism"] = *parallelism
	}
	if section == "cronjobs" {
		n["schedule"] = o.Spec.Schedule
		// P1-D 조립 원천 — cronJobReadyText의 "Suspended" 분기. true일 때만
		// 키를 싣는다(nil·false는 legacy에서도 무중단 동치).
		if o.Spec.Suspend != nil && *o.Spec.Suspend {
			n["suspend"] = true
		}
	}
	if o.Spec.Selector != nil && len(o.Spec.Selector.MatchLabels) > 0 {
		// P1-D 조립 원천 — ownerReferences 부재 pod의 셀렉터 매칭 폴백.
		n["selector"] = o.Spec.Selector.MatchLabels
	}
	if active, ok := activeCount(o.Status.Active); ok {
		n["active"] = active
	}
	if section == "jobs" {
		n["succeeded"] = o.Status.Succeeded
		n["failed"] = o.Status.Failed
	}
	if cs := containerQuantities(containers, true); cs != nil {
		n["containers"] = cs
	}
	return contract.DiscoveredResource{
		ExternalID: externalID(km, o.Metadata), ExternalURN: buildURN(ctxID, km, o.Metadata), DisplayName: o.Metadata.Name,
		Raw: buildRaw(o.Metadata), Normalized: n,
	}
}

// workloadResource normalizes one apps workload-family item into the legacy
// K8sWorkloadItem 형면(replicas·readyReplicas + v2-only image)에 P1-B 확장
// (updatedReplicas·availableReplicas·컨테이너 원시량)을 얹는다. daemonsets만
// desired/numberReady를 쓴다.
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
	n := contract.JSONMap{
		"replicas":      replicas,
		"readyReplicas": ready,
		// V2-only field: legacy 리스트 직렬화(K8sWorkloadItem)에 image가
		// 없어 비교 집합 밖(mapping.md dropped(v2-only)).
		"image": image,
	}
	// P1-B (J-P1-3) — updated·available은 apps 계열 status 전용. daemonset만
	// 원천 필드가 다르다(UpdatedNumberScheduled·NumberAvailable — legacy
	// buildWorkloadItems k8s_build_pod.go:214-215 동치, P1-D 조립 원천).
	if section == "daemonsets" {
		n["updatedReplicas"] = o.Status.UpdatedNumberScheduled
		n["availableReplicas"] = o.Status.NumberAvailable
	} else {
		n["updatedReplicas"] = o.Status.UpdatedReplicas
		n["availableReplicas"] = o.Status.AvailableReplicas
	}
	if o.Spec.Selector != nil && len(o.Spec.Selector.MatchLabels) > 0 {
		// P1-D 조립 원천 — ownerReferences 부재 pod의 셀렉터 매칭 폴백
		// (legacy buildPodItemsWithWorkloads assignBySelector).
		n["selector"] = o.Spec.Selector.MatchLabels
	}
	if cs := containerQuantities(o.Spec.Template.Spec.Containers, true); cs != nil {
		n["containers"] = cs
	}
	return contract.DiscoveredResource{
		ExternalID: externalID(km, o.Metadata), ExternalURN: buildURN(ctxID, km, o.Metadata), DisplayName: o.Metadata.Name,
		Raw: buildRaw(o.Metadata), Normalized: n,
	}
}
