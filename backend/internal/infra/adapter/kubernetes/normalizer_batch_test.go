package kubernetes

// P1-A 신규 확장 종의 정규화 단위 고정 — J-P1-1 섹션 4종(replicasets·jobs·
// cronjobs·endpoints)과 J-P1-3의 P1-A 소관 필드(pod Raw ownerReferences·node
// podCIDRs). 비교 스코프는 P1-C2까지 불변(§9-10)이므로 본 테스트는 수집 형면
// (kind/subtype/URN/Raw/Normalized 키)만 단얫하고 비교 필드는 다루지 않는다.

import (
	"testing"
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
