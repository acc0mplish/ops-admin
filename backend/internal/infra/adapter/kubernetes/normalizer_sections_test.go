package kubernetes

// P2-D istio VirtualService 앵커의 단위 고정 — 앵커 최소형(J-P1-1 "스코프 외
// 앵커 — istio 오퍼레이션 uid 닻"·R-P6 "S-2 읽기 패리티 제외와 무관")을 단얫한다:
// 수집은 신원 계열(ExternalID·URN·DisplayName·Raw metadata)만이고 Normalized 키는
// 없다. 비교 집합 불참이라 비교 필드는 다루지 않는다(gw 테스트와 동일 태세).
// istio CRD 부재 404 섹션 스킵은 fetchSectionPage 공용 경로의 기존 단얫
// (TestGatewayAPISectionSkipsWhenCRDAbsent)이 커버한다.

import (
	"context"
	"testing"

	"ops-admin/backend/internal/infra/contract"
)

func TestNormalizeVirtualServiceAnchor(t *testing.T) {
	const ctxID = uint(13)

	res, err := normalizeSection(ctxID, "virtualservices", []byte(`{
		"apiVersion": "networking.istio.io/v1beta1",
		"kind": "VirtualService",
		"metadata": {"name": "vs-1", "namespace": "team-a", "uid": "u-vs",
			"creationTimestamp": "2026-09-01T00:00:00Z", "labels": {"app": "seed"}},
		"spec": {"hosts": ["seed.example.com"], "http": [{"route": [
			{"destination": {"host": "a"}, "weight": 50},
			{"destination": {"host": "b"}, "weight": 50}
		]}]}
	}`))
	if err != nil {
		t.Fatalf("normalizeSection(virtualservices): %v", err)
	}
	if res.Kind != "network.virtual_service" || res.Subtype != "" {
		t.Errorf("kind/subtype = %q/%q, want network.virtual_service/(empty)", res.Kind, res.Subtype)
	}
	if res.ExternalID != "team-a/vs-1" {
		t.Errorf("ExternalID = %q, want team-a/vs-1 (네임스페이스 소속 종 — default 분기)", res.ExternalID)
	}
	if want := "urn:k8s:13:virtualservice:team-a/vs-1"; res.ExternalURN != want {
		t.Errorf("ExternalURN = %q, want %q (오퍼레이션 uid 닻 — executor_traffic.go parseTrafficURN 대응)", res.ExternalURN, want)
	}
	if res.DisplayName != "vs-1" {
		t.Errorf("DisplayName = %q, want vs-1", res.DisplayName)
	}
	// 앵커 최소형 — Raw는 신원 metadata, Normalized는 비어 있다. spec의
	// hosts·http 가중치는 유도 키를 만들지 않는다(R-P6 — 읽기 패리티 확장 오해
	// 방지).
	if res.Raw["name"] != "vs-1" || res.Raw["uid"] != "u-vs" || res.Raw["namespace"] != "team-a" {
		t.Errorf("Raw = %v, want identity metadata (name·uid·namespace)", res.Raw)
	}
	if len(res.Normalized) != 0 {
		t.Errorf("Normalized = %v, want empty (앵커 최소형 — J-P1-1·R-P6)", res.Normalized)
	}
}

// TestVirtualServiceAnchorDiscovered — Discover 체인(J-P1-1 섹션 행)이
// virtualservices를 수집해 앵커 종으로 돌려주는 종단 경로.
func TestVirtualServiceAnchorDiscovered(t *testing.T) {
	mock := newMockK8s(t, "v1.29.4+k3s")
	mock.lists["/apis/networking.istio.io/v1beta1/virtualservices"] = []map[string]any{
		mock.obj(map[string]any{
			"metadata": map[string]any{"name": "vs-seed", "namespace": "default", "uid": "u-vs-seed"},
			"spec": map[string]any{
				"http": []map[string]any{{"route": []map[string]any{
					{"destination": map[string]any{"host": "a"}, "weight": 100},
				}}},
			},
		}),
	}
	fixture := &k8sFixture{t: t, mock: mock, adapter: NewAdapter(WithPageSize(4)), kubeconfig: kubeconfigFor(mock.srv.URL)}

	var anchors []contract.DiscoveredResource
	cursor := ""
	for pages := 0; pages < 64; pages++ {
		page, err := fixture.adapter.Discover(context.Background(), contract.DiscoverRequest{
			ContextID: 1, Cursor: cursor,
			Connection: contract.ConnectionView{Material: map[string]string{"inventory": fixture.kubeconfig}},
		})
		if err != nil {
			t.Fatalf("Discover(cursor=%q): %v", cursor, err)
		}
		for _, res := range page.Resources {
			if res.Kind == "network.virtual_service" {
				anchors = append(anchors, res)
			}
		}
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}
	if len(anchors) != 1 || anchors[0].DisplayName != "vs-seed" {
		t.Fatalf("virtualservice anchors = %+v, want 1 vs-seed", anchors)
	}
	if want := "urn:k8s:1:virtualservice:default/vs-seed"; anchors[0].ExternalURN != want {
		t.Errorf("ExternalURN = %q, want %q", anchors[0].ExternalURN, want)
	}
	if len(anchors[0].Normalized) != 0 {
		t.Errorf("Normalized = %v, want empty (앵커 최소형)", anchors[0].Normalized)
	}
}
