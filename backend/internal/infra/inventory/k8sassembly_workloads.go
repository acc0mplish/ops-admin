// k8sassembly_workloads.go — pods(워크로드 역추적 체인)·workloads(batch 유도
// 포함)·network·advancedNetwork·configStorage 섹션 조립. 산식 권위는 v1
// buildPodItemsWithWorkloads(k8s_build_pod.go:23)·buildWorkloadItems(:175)·
// buildNetworkSection(k8s_build_net.go:324)·buildAdvancedNetworkSection(:14)·
// buildConfigStorageSection(:423)이다.
package inventory

import (
	"fmt"
	"sort"
	"strings"

	"ops-admin/backend/internal/infra/contract"
	v1model "ops-admin/backend/model"
)

// containerEntry는 normalized.containers 배열의 원소 형상이다.
type containerEntry = contract.JSONMap

// orderedWorkloadRows는 5종 워크로드 행을 v1 순회 순서로 정렬해 돌려준다
// (deployment→statefulset→daemonset→job→cronjob — 동종 내 rows 순서 유지,
// stable). 셀렉터 배정·이름 접두 폴백의 우선순위가 이 순서를 따른다.
func orderedWorkloadRows(rows []ProjectedResource) []ProjectedResource {
	priority := map[string]int{"deployment": 0, "statefulset": 1, "daemonset": 2, "job": 3, "cronjob": 4}
	out := workloadRows(rows)
	sort.SliceStable(out, func(i, j int) bool { return priority[out[i].Subtype] < priority[out[j].Subtype] })
	return out
}

// --- pods 섹션 (집계: workloadName·workloadType — RS 체인) ---

type podWorkloadRef struct {
	Name string
	Type string
}

func podWorkloadKey(namespace, podName string) string {
	return namespace + "/" + podName
}

// BuildPodItemsWithWorkloads mirrors legacy buildPodItemsWithWorkloads — 5단계
// 역추적: (1) RS→Deployment 체인 (2) Job→CronJob 체인 (3) pod ownerReferences
// 직접 (4) 셀렉터 매칭 폴백 (5) cronjob 접두·이름 접두 최장 매칭.
func BuildPodItemsWithWorkloads(rows []ProjectedResource) []v1model.K8sPodItem {
	podRows := rowsOfKind(rows, "orchestration.pod")
	// 워크로드 순회는 v1 순서(Deployment→StatefulSet→DaemonSet→Job→CronJob)로
	// 정규화한다 — 셀렉터 배정·접두 폴백의 우선순위가 순회 순서에 의존하기
	// 때문이다(v1 k8sFetchedData 필드 순회 순서; rows 자체는 임의 순서).
	orderedWorkloads := orderedWorkloadRows(rows)
	refs := make(map[string]podWorkloadRef)
	workloadsByNamespace := make(map[string][]podWorkloadRef)
	addWorkload := func(namespace, name, workloadType string) {
		if name == "" {
			return
		}
		workloadsByNamespace[namespace] = append(workloadsByNamespace[namespace], podWorkloadRef{Name: name, Type: workloadType})
	}
	for _, row := range orderedWorkloads {
		workloadType := workloadItemType(row.Subtype)
		addWorkload(rawNamespace(row), row.DisplayName, workloadType)
	}

	// (1) RS의 Deployment 소유자, (2) Job의 CronJob 소유자.
	replicaSetRefs := make(map[string]podWorkloadRef)
	jobRefs := make(map[string]podWorkloadRef)
	for _, row := range rows {
		switch row.Subtype {
		case "replicaset":
			for _, owner := range rowOwners(row) {
				if strings.EqualFold(ownerField(owner, "kind"), "Deployment") && ownerField(owner, "name") != "" {
					replicaSetRefs[podWorkloadKey(rawNamespace(row), row.DisplayName)] = podWorkloadRef{Name: ownerField(owner, "name"), Type: "Deployment"}
					break
				}
			}
		case "job":
			for _, owner := range rowOwners(row) {
				if strings.EqualFold(ownerField(owner, "kind"), "CronJob") && ownerField(owner, "name") != "" {
					jobRefs[podWorkloadKey(rawNamespace(row), row.DisplayName)] = podWorkloadRef{Name: ownerField(owner, "name"), Type: "CronJob"}
					break
				}
			}
		}
	}

	// (3) pod ownerReferences 직접.
	for _, pod := range podRows {
		key := podWorkloadKey(rawNamespace(pod), pod.DisplayName)
		for _, owner := range rowOwners(pod) {
			var ref podWorkloadRef
			switch {
			case strings.EqualFold(ownerField(owner, "kind"), "ReplicaSet"):
				ref = replicaSetRefs[podWorkloadKey(rawNamespace(pod), ownerField(owner, "name"))]
			case strings.EqualFold(ownerField(owner, "kind"), "Job"):
				ref = jobRefs[podWorkloadKey(rawNamespace(pod), ownerField(owner, "name"))]
				if ref.Name == "" {
					ref = podWorkloadRef{Name: ownerField(owner, "name"), Type: "Job"}
				}
			case strings.EqualFold(ownerField(owner, "kind"), "StatefulSet"), strings.EqualFold(ownerField(owner, "kind"), "DaemonSet"):
				ref = podWorkloadRef{Name: ownerField(owner, "name"), Type: ownerField(owner, "kind")}
			}
			if ref.Name != "" {
				refs[key] = ref
				break
			}
		}
	}

	// (4) 셀렉터 매칭 폴백 — 소유자 근거가 없는 pod.
	assignBySelector := func(namespace, name, workloadType string, selector map[string]string) {
		for _, pod := range podRows {
			key := podWorkloadKey(namespace, pod.DisplayName)
			if rawNamespace(pod) != namespace || refs[key].Name != "" || len(selector) == 0 {
				continue
			}
			if matchLabels(jsonStrMap(pod.Raw["labels"]), selector) {
				refs[key] = podWorkloadRef{Name: name, Type: workloadType}
			}
		}
	}
	for _, row := range orderedWorkloads {
		selector := jsonStrMap(row.Normalized["selector"])
		if row.Subtype == "job" && len(selector) == 0 {
			// legacy job 폴백 셀렉터(합성은 조립 소유 — k8s_build_pod.go:104).
			selector = map[string]string{"job-name": row.DisplayName}
		}
		if row.Subtype == "cronjob" {
			continue // cronjob은 셀렉터 대신 접두 경로(아래 5)를 쓴다.
		}
		assignBySelector(rawNamespace(row), row.DisplayName, workloadItemType(row.Subtype), selector)
	}

	// (5) cronjob 접두 매칭 → 이름 접두 최장 매칭.
	for _, row := range orderedWorkloads {
		if row.Subtype != "cronjob" {
			continue
		}
		for _, pod := range podRows {
			if rawNamespace(pod) != rawNamespace(row) {
				continue
			}
			key := podWorkloadKey(rawNamespace(pod), pod.DisplayName)
			if refs[key].Name != "" {
				continue
			}
			for _, owner := range rowOwners(pod) {
				if strings.EqualFold(ownerField(owner, "kind"), "Job") && strings.HasPrefix(ownerField(owner, "name"), row.DisplayName+"-") {
					refs[key] = podWorkloadRef{Name: row.DisplayName, Type: "CronJob"}
				}
			}
		}
	}
	for _, pod := range podRows {
		key := podWorkloadKey(rawNamespace(pod), pod.DisplayName)
		if refs[key].Name != "" {
			continue
		}
		// 제한된 클러스터는 ownerReferences를 비우기도 한다 — 컨트롤러가 만든
		// pod 이름 접두로 폴백하고 가장 긴 워크로드 이름을 선호한다.
		for _, candidate := range workloadsByNamespace[rawNamespace(pod)] {
			if strings.HasPrefix(pod.DisplayName, candidate.Name+"-") && len(candidate.Name) > len(refs[key].Name) {
				refs[key] = candidate
			}
		}
	}
	return buildPodItems(podRows, refs)
}

// workloadItemType — V2 subtype → legacy 표시 Type 문자열.
func workloadItemType(subtype string) string {
	switch subtype {
	case "deployment":
		return "Deployment"
	case "statefulset":
		return "StatefulSet"
	case "daemonset":
		return "DaemonSet"
	case "job":
		return "Job"
	case "cronjob":
		return "CronJob"
	}
	return subtype
}

func buildPodItems(podRows []ProjectedResource, refs map[string]podWorkloadRef) []v1model.K8sPodItem {
	items := make([]v1model.K8sPodItem, 0, len(podRows))
	for _, pod := range podRows {
		workload := refs[podWorkloadKey(rawNamespace(pod), pod.DisplayName)]
		if workload.Name == "" {
			for _, owner := range rowOwners(pod) {
				if name := ownerField(owner, "name"); name != "" {
					switch ownerField(owner, "kind") {
					case "StatefulSet", "DaemonSet", "Job":
						workload = podWorkloadRef{Name: name, Type: ownerField(owner, "kind")}
					}
					if workload.Name != "" {
						break
					}
				}
			}
		}
		restarts, _ := jsonInt(pod.Normalized["restartCount"])
		items = append(items, v1model.K8sPodItem{
			Name:         pod.DisplayName,
			Namespace:    rawNamespace(pod),
			WorkloadName: workload.Name,
			WorkloadType: workload.Type,
			Status:       FallbackText(jsonString(pod.Normalized["phase"])),
			Node:         FallbackText(jsonString(pod.Normalized["nodeName"])),
			NodeIP:       FallbackText(jsonString(pod.Normalized["hostIP"])),
			Restarts:     int(restarts),
			Age:          HumanizeAge(rawCreationTimestamp(pod)),
			IP:           FallbackText(jsonString(pod.Normalized["podIP"])),
		})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Namespace == items[j].Namespace {
			return items[i].Name < items[j].Name
		}
		return items[i].Namespace < items[j].Namespace
	})
	return items
}

// --- workloads 섹션 (조립: updated·available batch 유도·requests/limits 포맷) ---

// BuildWorkloadItems mirrors legacy buildWorkloadItems — 5종 워크로드 행을
// legacy 목록 형상으로. batch 유도는 v1 그대로다: job Ready = succeeded/total
// (total = completions, 0이면 active+succeeded+failed), Updated = Active,
// Available = Succeeded. cronjob Updated = Available = active, Ready는
// cronJobReadyText 텍스트.
func BuildWorkloadItems(rows []ProjectedResource) []v1model.K8sWorkloadItem {
	items := make([]v1model.K8sWorkloadItem, 0)
	for _, row := range workloadRows(rows) {
		n := row.Normalized
		replicas, _ := jsonInt(n["replicas"])
		ready, _ := jsonInt(n["readyReplicas"])
		updated, _ := jsonInt(n["updatedReplicas"])
		available, _ := jsonInt(n["availableReplicas"])
		item := v1model.K8sWorkloadItem{
			Name:      row.DisplayName,
			Type:      workloadItemType(row.Subtype),
			Namespace: rawNamespace(row),
			Age:       HumanizeAge(rawCreationTimestamp(row)),
			Requests:  formatWorkloadResourceSummary(jsonObjectList(n["containers"]), true),
			Limits:    formatWorkloadResourceSummary(jsonObjectList(n["containers"]), false),
		}
		switch row.Subtype {
		case "job":
			active, _ := jsonInt(n["active"])
			succeeded, _ := jsonInt(n["succeeded"])
			failed, _ := jsonInt(n["failed"])
			total, hasCompletions := jsonInt(n["completions"])
			if !hasCompletions || total == 0 {
				total = active + succeeded + failed
			}
			item.Ready = fmt.Sprintf("%d/%d", succeeded, total)
			item.Updated = int(active)
			item.Available = int(succeeded)
		case "cronjob":
			active, _ := jsonInt(n["active"])
			item.Ready = CronJobReadyText(jsonBool(n["suspend"]), int(active))
			item.Updated = int(active)
			item.Available = int(active)
		default:
			item.Ready = fmt.Sprintf("%d/%d", ready, replicas)
			item.Updated = int(updated)
			item.Available = int(available)
		}
		items = append(items, item)
	}
	return items
}

// formatWorkloadResourceSummary mirrors legacy formatWorkloadResourceSummary —
// 컨테이너 합(값 관측 컨테이너만 — 수집측 quantityPair가 키 생략으로 흡수) 후
// "CPU / MEM" 결합, 전부 무관측이면 "-".
func formatWorkloadResourceSummary(containers []containerEntry, requests bool) string {
	var cpuMilli, memoryBytes int64
	hasCPU, hasMemory := false, false
	for _, container := range containers {
		values := jsonAnyMap(container["limits"])
		if requests {
			values = jsonAnyMap(container["requests"])
		}
		if v, ok := jsonInt(values["cpuMilli"]); ok {
			cpuMilli += v
			hasCPU = true
		}
		if v, ok := jsonInt(values["memBytes"]); ok {
			memoryBytes += v
			hasMemory = true
		}
	}
	parts := make([]string, 0, 2)
	if hasCPU {
		parts = append(parts, FormatCPUMilli(cpuMilli))
	}
	if hasMemory {
		parts = append(parts, FormatMemoryBytes(memoryBytes))
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, " / ")
}

// --- network 섹션 (집계: endpoints · 조립: age·"<none>"·nodePort 포맷) ---

// BuildEndpointCounts mirrors legacy buildEndpointCounts — 키는 "ns/name",
// 값은 subsets 주소 합(수집측 readyAddresses 합계와 동치).
func BuildEndpointCounts(endpointRows []ProjectedResource) map[string]int {
	result := make(map[string]int, len(endpointRows))
	for _, row := range endpointRows {
		key := rawNamespace(row) + "/" + row.DisplayName
		ready, _ := jsonInt(row.Normalized["readyAddresses"])
		result[key] = int(ready)
	}
	return result
}

// BuildNetworkSection mirrors legacy buildNetworkSection.
func BuildNetworkSection(serviceRows []ProjectedResource, ingressRows []ProjectedResource, endpointCounts map[string]int) v1model.K8sNetworkSection {
	serviceItems := make([]v1model.K8sServiceItem, 0, len(serviceRows))
	for _, row := range serviceRows {
		ports := make([]string, 0)
		for _, entry := range jsonObjectList(row.Normalized["ports"]) {
			port, _ := jsonInt(entry["port"])
			nodePort, _ := jsonInt(entry["nodePort"])
			ports = append(ports, FormatServiceListPort(int(port), int(nodePort), jsonString(entry["protocol"])))
		}
		externalIP := jsonString(row.Normalized["externalIP"])
		if externalIP == "" {
			externalIP = "<none>"
		}
		clusterIP := jsonString(row.Normalized["clusterIP"])
		serviceType := FallbackText(jsonString(row.Normalized["type"]))
		if strings.EqualFold(strings.TrimSpace(clusterIP), "None") {
			serviceType = "Headless"
		}
		serviceItems = append(serviceItems, v1model.K8sServiceItem{
			Name:       row.DisplayName,
			Namespace:  rawNamespace(row),
			Type:       serviceType,
			ClusterIP:  FallbackText(clusterIP),
			ExternalIP: externalIP,
			Ports:      strings.Join(ports, ", "),
			Endpoints:  endpointCounts[rawNamespace(row)+"/"+row.DisplayName],
			Age:        HumanizeAge(rawCreationTimestamp(row)),
		})
	}
	sort.Slice(serviceItems, func(i, j int) bool {
		if serviceItems[i].Namespace == serviceItems[j].Namespace {
			return serviceItems[i].Name < serviceItems[j].Name
		}
		return serviceItems[i].Namespace < serviceItems[j].Namespace
	})

	ingressItems := make([]v1model.K8sIngressItem, 0, len(ingressRows))
	for _, row := range ingressRows {
		ingressItems = append(ingressItems, v1model.K8sIngressItem{
			Name:      row.DisplayName,
			Namespace: rawNamespace(row),
			Host:      FallbackText(strings.Join(jsonStrList(row.Normalized["hosts"]), ", ")),
			Address:   FallbackText(jsonString(row.Normalized["address"])),
			TLS:       jsonString(row.Normalized["tls"]),
			Age:       HumanizeAge(rawCreationTimestamp(row)),
		})
	}
	sort.Slice(ingressItems, func(i, j int) bool {
		if ingressItems[i].Namespace == ingressItems[j].Namespace {
			return ingressItems[i].Name < ingressItems[j].Name
		}
		return ingressItems[i].Namespace < ingressItems[j].Namespace
	})

	return v1model.K8sNetworkSection{
		Services:  serviceItems,
		Ingresses: ingressItems,
	}
}

// --- advancedNetwork 섹션 (조립: joinAndLimit 표시·gateway 주소 서비스 폴백) ---

// BuildAdvancedNetworkSection mirrors legacy buildAdvancedNetworkSection —
// gateway 주소는 관측 부재 시 동일 네임스페이스의 매칭 서비스 externalIP 폴백
// (resolveGatewayAPIAddress 동치 — 후보 name·name-istio·name- 접두).
func BuildAdvancedNetworkSection(gatewayRows []ProjectedResource, httpRouteRows []ProjectedResource, serviceRows []ProjectedResource) v1model.K8sAdvancedNetworkSection {
	result := v1model.K8sAdvancedNetworkSection{
		GatewayAPIGateways: make([]v1model.K8sIstioResourceItem, 0, len(gatewayRows)),
		HTTPRoutes:         make([]v1model.K8sIstioResourceItem, 0, len(httpRouteRows)),
	}

	for _, row := range gatewayRows {
		address := resolveGatewayAddress(row, serviceRows)
		result.GatewayAPIGateways = append(result.GatewayAPIGateways, v1model.K8sIstioResourceItem{
			Name:      row.DisplayName,
			Namespace: rawNamespace(row),
			Kind:      "Gateway",
			Hosts:     JoinAndLimit(jsonStrList(row.Normalized["hosts"]), 3),
			Address:   address,
			Ports:     JoinAndLimit(jsonStrList(row.Normalized["ports"]), 4),
			Target:    FallbackText(jsonString(row.Normalized["gatewayClassName"])),
			Age:       HumanizeAge(rawCreationTimestamp(row)),
		})
	}

	for _, row := range httpRouteRows {
		result.HTTPRoutes = append(result.HTTPRoutes, v1model.K8sIstioResourceItem{
			Name:      row.DisplayName,
			Namespace: rawNamespace(row),
			Kind:      "HTTPRoute",
			Hosts:     JoinAndLimit(jsonStrList(row.Normalized["hostnames"]), 3),
			Gateways:  JoinAndLimit(jsonStrList(row.Normalized["parents"]), 3),
			Target:    JoinAndLimit(jsonStrList(row.Normalized["targets"]), 3),
			Age:       HumanizeAge(rawCreationTimestamp(row)),
		})
	}

	sort.Slice(result.GatewayAPIGateways, func(i, j int) bool {
		return compareIstioItems(result.GatewayAPIGateways[i], result.GatewayAPIGateways[j])
	})
	sort.Slice(result.HTTPRoutes, func(i, j int) bool { return compareIstioItems(result.HTTPRoutes[i], result.HTTPRoutes[j]) })
	return result
}

func compareIstioItems(left, right v1model.K8sIstioResourceItem) bool {
	if left.Namespace == right.Namespace {
		return left.Name < right.Name
	}
	return left.Namespace < right.Namespace
}

// resolveGatewayAddress mirrors legacy resolveGatewayAPIAddress — status
// 주소(joinAndLimit 3) 우선, 부재 시 서비스 이름 매칭 폴백, 최종 "-".
func resolveGatewayAddress(gateway ProjectedResource, serviceRows []ProjectedResource) string {
	if addresses := jsonStrList(gateway.Normalized["addresses"]); len(uniqueNonEmptyStrings(addresses)) > 0 {
		return JoinAndLimit(addresses, 3)
	}
	for _, service := range serviceRows {
		if rawNamespace(service) != rawNamespace(gateway) {
			continue
		}
		name := service.DisplayName
		if name != gateway.DisplayName && name != gateway.DisplayName+"-istio" && !strings.HasPrefix(name, gateway.DisplayName+"-") {
			continue
		}
		if address := serviceExternalIPText(service); address != "" && address != "<none>" && address != "-" {
			return address
		}
	}
	return "-"
}

func serviceExternalIPText(service ProjectedResource) string {
	return jsonString(service.Normalized["externalIP"])
}

// --- configStorage 섹션 (조립: age·행 형상) ---

// BuildConfigStorageSection mirrors legacy buildConfigStorageSection — PVC 행은
// sourceType·namespaceScope 등을 공란으로, PV 행만 채운다(legacy 동일). 정렬은
// kind→namespace→name.
func BuildConfigStorageSection(configMapRows []ProjectedResource, secretRows []ProjectedResource, pvcRows []ProjectedResource, pvRows []ProjectedResource) v1model.K8sConfigStorageSection {
	configMapItems := make([]v1model.K8sConfigMapItem, 0, len(configMapRows))
	for _, row := range configMapRows {
		configMapItems = append(configMapItems, v1model.K8sConfigMapItem{
			Name:      row.DisplayName,
			Namespace: rawNamespace(row),
			Keys:      len(jsonStrList(row.Normalized["dataKeys"])),
			Age:       HumanizeAge(rawCreationTimestamp(row)),
		})
	}
	sort.Slice(configMapItems, func(i, j int) bool {
		if configMapItems[i].Namespace == configMapItems[j].Namespace {
			return configMapItems[i].Name < configMapItems[j].Name
		}
		return configMapItems[i].Namespace < configMapItems[j].Namespace
	})

	secretItems := make([]v1model.K8sSecretItem, 0, len(secretRows))
	for _, row := range secretRows {
		secretItems = append(secretItems, v1model.K8sSecretItem{
			Name:      row.DisplayName,
			Namespace: rawNamespace(row),
			Type:      FallbackText(jsonString(row.Normalized["type"])),
			Age:       HumanizeAge(rawCreationTimestamp(row)),
		})
	}
	sort.Slice(secretItems, func(i, j int) bool {
		if secretItems[i].Namespace == secretItems[j].Namespace {
			return secretItems[i].Name < secretItems[j].Name
		}
		return secretItems[i].Namespace < secretItems[j].Namespace
	})

	storageItems := make([]v1model.K8sStorageItem, 0, len(pvcRows)+len(pvRows))
	for _, row := range pvcRows {
		storageItems = append(storageItems, v1model.K8sStorageItem{
			Name:         row.DisplayName,
			Kind:         "PVC",
			Namespace:    FallbackText(rawNamespace(row)),
			Status:       FallbackText(jsonString(row.Normalized["phase"])),
			Capacity:     FallbackText(jsonString(row.Normalized["capacityRaw"])),
			StorageClass: FallbackText(jsonString(row.Normalized["storageClassName"])),
			AccessModes:  strings.Join(jsonStrList(row.Normalized["accessModes"]), ", "),
		})
	}
	for _, row := range pvRows {
		storageItems = append(storageItems, v1model.K8sStorageItem{
			Name:           row.DisplayName,
			Kind:           "PV",
			Namespace:      "Cluster-scoped",
			NamespaceScope: jsonString(row.Normalized["namespaceScope"]),
			Status:         FallbackText(jsonString(row.Normalized["phase"])),
			Capacity:       FallbackText(jsonString(row.Normalized["capacityRaw"])),
			StorageClass:   FallbackText(jsonString(row.Normalized["storageClassName"])),
			SourceType:     FallbackText(jsonString(row.Normalized["sourceType"])),
			Path:           FallbackText(jsonString(row.Normalized["sourcePath"])),
			NFSServer:      FallbackText(jsonString(row.Normalized["nfsServer"])),
			AccessModes:    strings.Join(jsonStrList(row.Normalized["accessModes"]), ", "),
			ReclaimPolicy:  FallbackText(jsonString(row.Normalized["reclaimPolicy"])),
		})
	}
	sort.Slice(storageItems, func(i, j int) bool {
		if storageItems[i].Kind == storageItems[j].Kind {
			if storageItems[i].Namespace == storageItems[j].Namespace {
				return storageItems[i].Name < storageItems[j].Name
			}
			return storageItems[i].Namespace < storageItems[j].Namespace
		}
		return storageItems[i].Kind < storageItems[j].Kind
	})

	return v1model.K8sConfigStorageSection{
		ConfigMaps: configMapItems,
		Secrets:    secretItems,
		Storage:    storageItems,
	}
}
