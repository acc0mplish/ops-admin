package kubernetes

// P1-A 신규 확장 종의 정규화 단위 고정 — J-P1-1 섹션 4종(replicasets·jobs·
// cronjobs·endpoints)과 J-P1-3의 P1-A 소관 필드(pod Raw ownerReferences·node
// podCIDRs). 비교 스코프는 P1-C2까지 불변(§9-10)이므로 본 테스트는 수집 형면
// (kind/subtype/URN/Raw/Normalized 키)만 단얫하고 비교 필드는 다루지 않는다.

import (
	"testing"

	"ops-admin/backend/internal/infra/contract"
)

func TestNormalizeBatchAndAuxiliarySections(t *testing.T) {
	const ctxID = uint(9)

	t.Run("pod_raw_ownerReferences", func(t *testing.T) {
		raw := []byte(`{
			"metadata": {
				"name": "web-1", "namespace": "default", "uid": "u-pod",
				"ownerReferences": [{"uid": "rs-uid-1", "kind": "ReplicaSet", "name": "web-rs"}]
			},
			"status": {"phase": "Running"}
		}`)
		res, err := normalizeSection(ctxID, "pods", raw)
		if err != nil {
			t.Fatalf("normalizeSection(pods): %v", err)
		}
		owners, ok := res.Raw["ownerReferences"].([]ownerReference)
		if !ok || len(owners) != 1 {
			t.Fatalf("Raw[ownerReferences] = %#v, want 1 entry (J-P1-3 pod Raw — workloadName/Type 집계 체인 원천)", res.Raw["ownerReferences"])
		}
		if owners[0].UID != "rs-uid-1" || owners[0].Kind != "ReplicaSet" || owners[0].Name != "web-rs" {
			t.Errorf("ownerReference = %+v, want {rs-uid-1 ReplicaSet web-rs} (uid·kind·name 3성분)", owners[0])
		}
	})

	t.Run("node_podCIDRs_and_fallback", func(t *testing.T) {
		withList := []byte(`{
			"metadata": {"name": "n1", "uid": "u-node"},
			"spec": {"podCIDRs": ["10.244.0.0/24", "10.244.1.0/24"]},
			"status": {"conditions": [{"type": "Ready", "status": "True"}]}
		}`)
		res, err := normalizeSection(ctxID, "nodes", withList)
		if err != nil {
			t.Fatalf("normalizeSection(nodes): %v", err)
		}
		cidrs, ok := res.Normalized["podCIDRs"].([]string)
		if !ok || len(cidrs) != 2 || cidrs[0] != "10.244.0.0/24" {
			t.Errorf("podCIDRs = %#v, want [10.244.0.0/24 10.244.1.0/24]", res.Normalized["podCIDRs"])
		}

		fallback := []byte(`{
			"metadata": {"name": "n2", "uid": "u-node-2"},
			"spec": {"podCIDR": "10.245.0.0/24"},
			"status": {"conditions": [{"type": "Ready", "status": "True"}]}
		}`)
		res, err = normalizeSection(ctxID, "nodes", fallback)
		if err != nil {
			t.Fatalf("normalizeSection(nodes, podCIDR): %v", err)
		}
		cidrs, ok = res.Normalized["podCIDRs"].([]string)
		if !ok || len(cidrs) != 1 || cidrs[0] != "10.245.0.0/24" {
			t.Errorf("podCIDRs = %#v, want [10.245.0.0/24] (단일 podCIDR 폴백 — J-P1-3)", res.Normalized["podCIDRs"])
		}
	})

	t.Run("workload_family_subtypes", func(t *testing.T) {
		cases := []struct {
			section string
			subtype string
			ready   int
			raw     string
		}{
			{"replicasets", "replicaset", 3, `{
				"metadata": {"name": "x", "namespace": "default", "uid": "u-x"},
				"spec": {"replicas": 3, "template": {"spec": {"containers": [{"name": "app", "image": "app:1"}]}}},
				"status": {"readyReplicas": 3}
			}`},
			// 실제 batch 종 페이로드에는 spec.replicas·status.readyReplicas가
			// 없다 → workload 형면 영값(P1-A 수집 진입 확인 — batch 고유 키는
			// P1-B가 completions·parallelism·schedule 등으로 확장).
			{"jobs", "job", 0, `{
				"metadata": {"name": "x", "namespace": "default", "uid": "u-x"},
				"spec": {"template": {"spec": {"containers": [{"name": "app", "image": "busybox:1"}]}}},
				"status": {"succeeded": 1}
			}`},
			{"cronjobs", "cronjob", 0, `{
				"metadata": {"name": "x", "namespace": "default", "uid": "u-x"},
				"spec": {"schedule": "*/5 * * * *"}
			}`},
		}
		for _, tc := range cases {
			res, err := normalizeSection(ctxID, tc.section, []byte(tc.raw))
			if err != nil {
				t.Fatalf("normalizeSection(%s): %v", tc.section, err)
			}
			if res.Kind != "orchestration.workload" || res.Subtype != tc.subtype {
				t.Errorf("%s: Kind/Subtype = %q/%q, want orchestration.workload/%s", tc.section, res.Kind, res.Subtype, tc.subtype)
			}
			wantURN := "urn:k8s:9:workload:default/" + tc.subtype + "/x"
			if res.ExternalURN != wantURN {
				t.Errorf("%s: URN = %q, want %q", tc.section, res.ExternalURN, wantURN)
			}
			if got, _ := res.Normalized["readyReplicas"].(int); got != tc.ready {
				t.Errorf("%s: readyReplicas = %#v, want %d", tc.section, res.Normalized["readyReplicas"], tc.ready)
			}
		}
	})

	t.Run("endpoints_readyAddresses", func(t *testing.T) {
		raw := []byte(`{
			"metadata": {"name": "web-svc", "namespace": "default", "uid": "u-ep"},
			"subsets": [
				{"addresses": [{"ip": "10.244.0.5"}, {"ip": "10.244.0.6"}]},
				{"addresses": [{"ip": "10.244.0.7"}]}
			]
		}`)
		res, err := normalizeSection(ctxID, "endpoints", raw)
		if err != nil {
			t.Fatalf("normalizeSection(endpoints): %v", err)
		}
		if res.Kind != "network.endpoint" || res.Subtype != "" {
			t.Errorf("Kind/Subtype = %q/%q, want network.endpoint/\"\" (Phase6ResourceKindExtensions)", res.Kind, res.Subtype)
		}
		if res.ExternalURN != "urn:k8s:9:endpoint:default/web-svc" || res.ExternalID != "default/web-svc" {
			t.Errorf("URN/ExternalID = %q/%q", res.ExternalURN, res.ExternalID)
		}
		if got, ok := res.Normalized["readyAddresses"].(int); !ok || got != 3 {
			t.Errorf("readyAddresses = %#v, want 3 (J-P1-3 — subset ready 주소 수)", res.Normalized["readyAddresses"])
		}
		// 주소 값 자체는 정규화에 진입하지 않는다(v2-only 보조종 — 개수만, 소비는 P1-D 집계).
		if _, present := res.Normalized["addresses"]; present {
			t.Errorf("endpoints must not carry address VALUES in Normalized")
		}
	})
}

// TestNormalizedKeySchema — P1-B deliverable(계획 §6 명명 고정): J-P1-3의
// 신규 normalized 키 19건(P1-B 소관분)이 스키마대로 착지하는지 단얫한다.
// 특히 컨테이너 원시량은 milli·bytes **정수**(round3 실수 금지 — J-P1-3),
// age·VOLATILE 필드는 수집하지 않는다(J-P1-3 각주).
func TestNormalizedKeySchema(t *testing.T) {
	const ctxID = uint(11)

	normalize := func(t *testing.T, section, raw string) contract.DiscoveredResource {
		t.Helper()
		res, err := normalizeSection(ctxID, section, []byte(raw))
		if err != nil {
			t.Fatalf("normalizeSection(%s): %v", section, err)
		}
		return res
	}
	// mustInt는 원시량(milli·bytes) 키의 int64 단얫 — float64 착지는
	// round3 실수 금지 위반이다(J-P1-3).
	mustInt := func(t *testing.T, m contract.JSONMap, key string) int64 {
		t.Helper()
		v, ok := m[key].(int64)
		if !ok {
			t.Fatalf("normalized[%q] = %#v, want int64", key, m[key])
		}
		return v
	}
	// mustCount는 개수 키(replicas·status 카운트 계열)의 int 단얫 — 기존
	// replicas·restartCount와 동일 정수형.
	mustCount := func(t *testing.T, m contract.JSONMap, key string) int {
		t.Helper()
		v, ok := m[key].(int)
		if !ok {
			t.Fatalf("normalized[%q] = %#v, want int", key, m[key])
		}
		return v
	}

	t.Run("node_field_keys", func(t *testing.T) {
		res := normalize(t, "nodes", `{
			"metadata": {"name": "n1", "uid": "u-n1",
				"annotations": {"ops-admin.io/namespace-scope": ""}},
			"spec": {"podCIDRs": ["10.244.0.0/24"]},
			"status": {
				"conditions": [{"type": "Ready", "status": "True"}],
				"capacity": {"cpu": "8", "memory": "31457280Ki", "pods": "110"},
				"allocatable": {"cpu": "7500m", "memory": "31457280Ki", "pods": "110"},
				"nodeInfo": {"kubeletVersion": "v1.29.4", "osImage": "Ubuntu 22.04"},
				"addresses": [
					{"type": "Hostname", "address": "n1"},
					{"type": "InternalIP", "address": "192.168.10.2"}
				]
			}
		}`)
		n := res.Normalized
		if n["kubeletVersion"] != "v1.29.4" {
			t.Errorf("kubeletVersion = %#v", n["kubeletVersion"])
		}
		if n["internalIP"] != "192.168.10.2" {
			t.Errorf("internalIP = %#v — InternalIP 타입 주소(J-P1-3)", n["internalIP"])
		}
		if n["osImage"] != "Ubuntu 22.04" {
			t.Errorf("osImage = %#v", n["osImage"])
		}
		if got := mustCount(t, n, "allocatablePods"); got != 110 {
			t.Errorf("allocatablePods = %d, want 110", got)
		}

		// 주소 부재 노드 — internalIP 키 생략(관측 부재 = 키 부재, "-" 포맷은 조립).
		absent := normalize(t, "nodes", `{
			"metadata": {"name": "n2", "uid": "u-n2"},
			"status": {"nodeInfo": {"kubeletVersion": "v1.29.4"}}
		}`)
		if _, present := absent.Normalized["internalIP"]; present {
			t.Errorf("internalIP must be omitted when no InternalIP address")
		}
		if _, present := absent.Normalized["allocatablePods"]; present {
			t.Errorf("allocatablePods must be omitted when allocatable.pods absent")
		}
	})

	t.Run("pod_ip_and_container_quantities", func(t *testing.T) {
		res := normalize(t, "pods", `{
			"metadata": {"name": "web-1", "namespace": "default", "uid": "u-p1"},
			"spec": {"containers": [{
				"name": "app", "image": "nginx:1",
				"resources": {"requests": {"cpu": "500m", "memory": "256Mi"},
					"limits": {"cpu": "1", "memory": "1Gi"}}
			}]},
			"status": {"phase": "Running", "hostIP": "192.168.10.2", "podIP": "10.244.0.5",
				"containerStatuses": [{"restartCount": 2}]}
		}`)
		n := res.Normalized
		if n["hostIP"] != "192.168.10.2" || n["podIP"] != "10.244.0.5" {
			t.Errorf("hostIP/podIP = %#v/%#v — legacy nodeIP/ip 착지", n["hostIP"], n["podIP"])
		}
		cs, ok := n["containers"].([]any)
		if !ok || len(cs) != 1 {
			t.Fatalf("containers = %#v, want 1 entry", n["containers"])
		}
		c := cs[0].(contract.JSONMap)
		if c["name"] != "app" {
			t.Errorf("container name = %#v", c["name"])
		}
		if _, has := c["image"]; has {
			t.Errorf("pod containers carry no image (J-P1-3 pod 스키마는 name+requests+limits)")
		}
		req := c["requests"].(contract.JSONMap)
		if got := mustInt(t, req, "cpuMilli"); got != 500 {
			t.Errorf("requests.cpuMilli = %d, want 500", got)
		}
		if got := mustInt(t, req, "memBytes"); got != 256*1024*1024 {
			t.Errorf("requests.memBytes = %d, want %d", got, 256*1024*1024)
		}
		lim := c["limits"].(contract.JSONMap)
		if got := mustInt(t, lim, "cpuMilli"); got != 1000 {
			t.Errorf("limits.cpuMilli = %d, want 1000 (코어→milli 정수)", got)
		}
		if got := mustInt(t, lim, "memBytes"); got != 1024*1024*1024 {
			t.Errorf("limits.memBytes = %d, want 1Gi", got)
		}

		// 미설정 원시량 — 서브키 생략(legacy hasCPU/hasMemory 분기 동치).
		res = normalize(t, "pods", `{
			"metadata": {"name": "free-1", "namespace": "default", "uid": "u-p2"},
			"spec": {"containers": [{"name": "app"}]},
			"status": {"phase": "Pending"}
		}`)
		cs = res.Normalized["containers"].([]any)
		c = cs[0].(contract.JSONMap)
		if len(c["requests"].(contract.JSONMap)) != 0 || len(c["limits"].(contract.JSONMap)) != 0 {
			t.Errorf("unset resources must yield empty pairs, got %#v", c)
		}
	})

	t.Run("workload_updated_available_containers", func(t *testing.T) {
		res := normalize(t, "deployments", `{
			"metadata": {"name": "web", "namespace": "default", "uid": "u-w1"},
			"spec": {"replicas": 3, "template": {"spec": {"containers": [{
				"name": "app", "image": "nginx:1",
				"resources": {"requests": {"cpu": "250m", "memory": "512Mi"}}
			}]}}},
			"status": {"readyReplicas": 3, "updatedReplicas": 2, "availableReplicas": 3}
		}`)
		n := res.Normalized
		if got := mustCount(t, n, "updatedReplicas"); got != 2 {
			t.Errorf("updatedReplicas = %d, want 2", got)
		}
		if got := mustCount(t, n, "availableReplicas"); got != 3 {
			t.Errorf("availableReplicas = %d, want 3", got)
		}
		c := n["containers"].([]any)[0].(contract.JSONMap)
		if c["image"] != "nginx:1" {
			t.Errorf("workload container image = %#v (J-P1-3 {name,image,requests,limits})", c["image"])
		}
		if got := mustInt(t, c["requests"].(contract.JSONMap), "memBytes"); got != 512*1024*1024 {
			t.Errorf("requests.memBytes = %d", got)
		}
	})

	t.Run("job_batch_keys", func(t *testing.T) {
		res := normalize(t, "jobs", `{
			"metadata": {"name": "migrate", "namespace": "default", "uid": "u-j1"},
			"spec": {"completions": 3, "parallelism": 2,
				"template": {"spec": {"containers": [{"name": "app", "image": "busybox:1"}]}}},
			"status": {"active": 1, "succeeded": 1, "failed": 0}
		}`)
		n := res.Normalized
		if got := mustCount(t, n, "completions"); got != 3 {
			t.Errorf("completions = %d", got)
		}
		if got := mustCount(t, n, "parallelism"); got != 2 {
			t.Errorf("parallelism = %d", got)
		}
		if got := mustCount(t, n, "active"); got != 1 {
			t.Errorf("active = %d", got)
		}
		if got := mustCount(t, n, "succeeded"); got != 1 {
			t.Errorf("succeeded = %d", got)
		}
		if got := mustCount(t, n, "failed"); got != 0 {
			t.Errorf("failed = %d", got)
		}
		if _, has := n["schedule"]; has {
			t.Errorf("job must not carry schedule (cronjob 전용)")
		}
		if _, has := n["updatedReplicas"]; has {
			t.Errorf("batch 종은 updatedReplicas 구조적 부재 — legacy는 active/succeeded 파생(조립 P1-D)")
		}
		// 컨테이너는 job의 spec.template 경로.
		if cs, ok := n["containers"].([]any); !ok || len(cs) != 1 {
			t.Errorf("job containers from spec.template = %#v", n["containers"])
		}
	})

	t.Run("cronjob_batch_keys", func(t *testing.T) {
		res := normalize(t, "cronjobs", `{
			"metadata": {"name": "backup", "namespace": "default", "uid": "u-c1"},
			"spec": {"schedule": "*/5 * * * *",
				"jobTemplate": {"spec": {"completions": 1, "parallelism": 1,
					"template": {"spec": {"containers": [{"name": "app", "image": "backup:2"}]}}}}},
			"status": {"active": [{"name": "backup-1", "uid": "u-j2", "namespace": "default"}]}
		}`)
		n := res.Normalized
		if n["schedule"] != "*/5 * * * *" {
			t.Errorf("schedule = %#v", n["schedule"])
		}
		if got := mustCount(t, n, "active"); got != 1 {
			t.Errorf("active = %d, want 1 (JobReference 배열 카운트 — legacy len(Status.Active))", got)
		}
		if got := mustCount(t, n, "completions"); got != 1 {
			t.Errorf("jobTemplate completions = %d", got)
		}
		if _, has := n["succeeded"]; has {
			t.Errorf("cronjob must not carry succeeded (job status 전용 — 구조적 부재)")
		}
		// 컨테이너·image는 cronjob의 jobTemplate.spec.template 경로.
		c := n["containers"].([]any)[0].(contract.JSONMap)
		if c["image"] != "backup:2" {
			t.Errorf("cronjob container image = %#v — jobTemplate 경계 아래 원천", c["image"])
		}
	})

	t.Run("service_externalIP", func(t *testing.T) {
		res := normalize(t, "services", `{
			"metadata": {"name": "lb", "namespace": "default", "uid": "u-s1"},
			"spec": {"type": "LoadBalancer", "clusterIP": "10.96.0.10",
				"externalIPs": ["198.51.100.4"]},
			"status": {"loadBalancer": {"ingress": [
				{"ip": "203.0.113.7"}, {"hostname": "lb.example.com"}]}}
		}`)
		if got, _ := res.Normalized["externalIP"].(string); got != "198.51.100.4, 203.0.113.7, lb.example.com" {
			t.Errorf("externalIP = %q, want legacy serviceExternalIP 결합(spec.externalIPs + LB ip/hostname)", got)
		}
		// 공백 — 키 생략("<none>"은 조립 포맷).
		res = normalize(t, "services", `{
			"metadata": {"name": "svc", "namespace": "default", "uid": "u-s2"},
			"spec": {"type": "ClusterIP"}
		}`)
		if _, present := res.Normalized["externalIP"]; present {
			t.Errorf("externalIP must be omitted when no external address")
		}
	})

	t.Run("ingress_address_tls", func(t *testing.T) {
		res := normalize(t, "ingresses", `{
			"metadata": {"name": "web", "namespace": "default", "uid": "u-i1"},
			"spec": {"rules": [{"host": "app.example.com"}],
				"tls": [{"hosts": ["app.example.com"]}]},
			"status": {"loadBalancer": {"ingress": [{"ip": "203.0.113.9"}]}}
		}`)
		n := res.Normalized
		if n["address"] != "203.0.113.9" {
			t.Errorf("address = %#v", n["address"])
		}
		if n["tls"] != "Enabled" {
			t.Errorf("tls = %#v, want Enabled(spec.tls 유무 — legacy 상태 어휘)", n["tls"])
		}
		// tls 무설치 — Disabled.
		res = normalize(t, "ingresses", `{
			"metadata": {"name": "plain", "namespace": "default", "uid": "u-i2"},
			"spec": {"rules": [{"host": "plain.example.com"}]}
		}`)
		if res.Normalized["tls"] != "Disabled" {
			t.Errorf("tls = %#v, want Disabled", res.Normalized["tls"])
		}
		if _, present := res.Normalized["address"]; present {
			t.Errorf("address must be omitted without LB ingress(legacy \"-\" 포맷은 조립)")
		}
	})

	t.Run("pv_pvc_volume_keys", func(t *testing.T) {
		pv := normalize(t, "persistentvolumes", `{
			"metadata": {"name": "pv-nfs", "uid": "u-v1",
				"annotations": {"ops-admin.io/namespace-scope": "team-a"}},
			"spec": {"storageClassName": "nfs", "accessModes": ["ReadWriteMany"],
				"capacity": {"storage": "10Gi"},
				"nfs": {"server": "10.0.0.9", "path": "/exports/team-a"},
				"persistentVolumeReclaimPolicy": "Retain"},
			"status": {"phase": "Bound"}
		}`)
		n := pv.Normalized
		for key, want := range map[string]any{
			"phase": "Bound", "namespaceScope": "team-a", "sourceType": "NFS",
			"sourcePath": "/exports/team-a", "nfsServer": "10.0.0.9", "reclaimPolicy": "Retain",
		} {
			if n[key] != want {
				t.Errorf("pv normalized[%q] = %#v, want %#v", key, n[key], want)
			}
		}
		// annotation 부재 pv — 기본 "Cluster-scoped"(legacy 동치).
		pv2 := normalize(t, "persistentvolumes", `{
			"metadata": {"name": "pv-host", "uid": "u-v2"},
			"spec": {"hostPath": {"path": "/data"}, "persistentVolumeReclaimPolicy": "Delete"}
		}`)
		if pv2.Normalized["namespaceScope"] != "Cluster-scoped" {
			t.Errorf("pv namespaceScope = %#v, want Cluster-scoped 기본", pv2.Normalized["namespaceScope"])
		}
		if pv2.Normalized["sourceType"] != "hostPath" || pv2.Normalized["sourcePath"] != "/data" {
			t.Errorf("hostPath source = %#v/%#v", pv2.Normalized["sourceType"], pv2.Normalized["sourcePath"])
		}
		if _, has := pv2.Normalized["nfsServer"]; has {
			t.Errorf("hostPath pv must not carry nfsServer(legacy \"-\" 포맷은 조립)")
		}
		// pvc — phase만(source·reclaimPolicy·namespaceScope는 legacy 공란).
		pvc := normalize(t, "persistentvolumeclaims", `{
			"metadata": {"name": "claim", "namespace": "default", "uid": "u-v3"},
			"spec": {"storageClassName": "standard", "accessModes": ["ReadWriteOnce"]},
			"status": {"phase": "Bound"}
		}`)
		if pvc.Normalized["phase"] != "Bound" {
			t.Errorf("pvc phase = %#v", pvc.Normalized["phase"])
		}
		for _, key := range []string{"sourceType", "sourcePath", "nfsServer", "reclaimPolicy", "namespaceScope"} {
			if _, has := pvc.Normalized[key]; has {
				t.Errorf("pvc must not carry %q (legacy pvc 행 공란 — 조립이 행 형상 소유)", key)
			}
		}
	})

	t.Run("age_not_collected", func(t *testing.T) {
		// J-P1-3 각주 — age 6건·VOLATILE 4건은 수집하지 않는다. Raw
		// creationTimestamp 원천은 조립(P1-D)이 포맷한다.
		raws := map[string]string{
			"pods":        `{"metadata": {"name": "x", "namespace": "d", "uid": "u", "creationTimestamp": "2026-09-01T00:00:00Z"}, "status": {"phase": "Running"}}`,
			"jobs":        `{"metadata": {"name": "x", "namespace": "d", "uid": "u", "creationTimestamp": "2026-09-01T00:00:00Z"}, "status": {}}`,
			"cronjobs":    `{"metadata": {"name": "x", "namespace": "d", "uid": "u", "creationTimestamp": "2026-09-01T00:00:00Z"}, "spec": {"schedule": "* * * * *"}}`,
			"services":    `{"metadata": {"name": "x", "namespace": "d", "uid": "u", "creationTimestamp": "2026-09-01T00:00:00Z"}, "spec": {}}`,
			"ingresses":   `{"metadata": {"name": "x", "namespace": "d", "uid": "u", "creationTimestamp": "2026-09-01T00:00:00Z"}, "spec": {}}`,
			"deployments": `{"metadata": {"name": "x", "namespace": "d", "uid": "u", "creationTimestamp": "2026-09-01T00:00:00Z"}, "spec": {}, "status": {}}`,
		}
		for section, raw := range raws {
			res := normalize(t, section, raw)
			if _, has := res.Normalized["age"]; has {
				t.Errorf("%s: normalized.age must not exist — age는 조립 포맷(J-P1-3 각주)", section)
			}
		}
	})
}
