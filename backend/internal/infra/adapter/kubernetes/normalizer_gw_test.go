package kubernetes

// P1-C1 확장 종(gateways·httproutes)의 단위 고정 — J-P1-3 normalized 키
// (gatewayClassName·hosts·addresses·ports / parents·targets)와 legacy 유도
// 동치(collectGatewayAPIHosts/Ports/Addresses·collectHTTPRouteParents/
// Targets), 그리고 J-P1-1 버전 폴백(v1 404 → v1beta1)·CRD 부재 섹션 스킵의
// Discover 체인 경로를 단얫한다. 비교 집합 불참(I-P5 이월)이라 비교 필드는
// 다루지 않는다.

import (
	"context"
	"testing"

	"ops-admin/backend/internal/infra/contract"
)

func TestNormalizeGatewayAPISections(t *testing.T) {
	const ctxID = uint(13)

	normalize := func(t *testing.T, section, raw string) contract.DiscoveredResource {
		t.Helper()
		res, err := normalizeSection(ctxID, section, []byte(raw))
		if err != nil {
			t.Fatalf("normalizeSection(%s): %v", section, err)
		}
		return res
	}

	t.Run("gateway_keys", func(t *testing.T) {
		res := normalize(t, "gateways", `{
			"metadata": {"name": "gw-1", "namespace": "default", "uid": "u-gw",
				"creationTimestamp": "2026-09-01T00:00:00Z"},
			"spec": {
				"gatewayClassName": "istio",
				"listeners": [
					{"name": "http", "hostname": "app.example.com", "port": 80, "protocol": "HTTP"},
					{"name": "https", "hostname": "app.example.com", "port": 443, "protocol": "HTTPS"},
					{"name": "wildcard", "port": 8080}
				]
			},
			"status": {"addresses": [
				{"type": "IPAddress", "value": "203.0.113.7"},
				{"type": "IPAddress", "value": "203.0.113.7"},
				{"type": "Hostname", "value": "gw-lb.example.com"}
			]}
		}`)
		if res.Kind != "network.gateway" || res.Subtype != "" {
			t.Errorf("kind/subtype = %q/%q, want network.gateway/(empty)", res.Kind, res.Subtype)
		}
		if res.ExternalID != "default/gw-1" {
			t.Errorf("ExternalID = %q, want default/gw-1 (네임스페이스 소속 종 — default 분기)", res.ExternalID)
		}
		n := res.Normalized
		if n["gatewayClassName"] != "istio" {
			t.Errorf("gatewayClassName = %#v", n["gatewayClassName"])
		}
		// hosts — hostname 부재 listener는 "*" 치환(legacy firstNonEmpty 동치),
		// 중복 제거는 순서 보존.
		hosts, ok := n["hosts"].([]string)
		if !ok || len(hosts) != 2 || hosts[0] != "app.example.com" || hosts[1] != "*" {
			t.Errorf("hosts = %#v, want [app.example.com *]", n["hosts"])
		}
		ports, ok := n["ports"].([]string)
		if !ok || len(ports) != 3 || ports[0] != "80/HTTP" || ports[1] != "443/HTTPS" || ports[2] != "8080/TCP" {
			t.Errorf("ports = %#v, want [80/HTTP 443/HTTPS 8080/TCP] (protocol 공란 → TCP)", n["ports"])
		}
		addresses, ok := n["addresses"].([]string)
		if !ok || len(addresses) != 2 || addresses[0] != "203.0.113.7" || addresses[1] != "gw-lb.example.com" {
			t.Errorf("addresses = %#v, want dedup [203.0.113.7 gw-lb.example.com]", n["addresses"])
		}
	})

	t.Run("gateway_absent_observation_omits_keys", func(t *testing.T) {
		// listeners·status.addresses·gatewayClassName 부재 — 키 전부 생략
		// (관측 부재 = 키 부재, P1-B 규약).
		res := normalize(t, "gateways", `{
			"metadata": {"name": "gw-bare", "namespace": "default", "uid": "u-gw2"}
		}`)
		for _, key := range []string{"gatewayClassName", "hosts", "ports", "addresses"} {
			if _, present := res.Normalized[key]; present {
				t.Errorf("normalized[%q] must be omitted when absent", key)
			}
		}
	})

	t.Run("httproute_keys", func(t *testing.T) {
		res := normalize(t, "httproutes", `{
			"metadata": {"name": "route-1", "namespace": "team-a", "uid": "u-rt",
				"creationTimestamp": "2026-09-01T00:00:00Z"},
			"spec": {
				"hostnames": ["route.example.com"],
				"parentRefs": [
					{"name": "gw-1"},
					{"name": "gw-2", "namespace": "gateway-infra"}
				],
				"rules": [
					{"backendRefs": [{"name": "svc-a", "port": 8080, "weight": 90}]},
					{"backendRefs": [{"name": "svc-b", "namespace": "team-b", "port": 9090, "weight": 10},
						{"name": "svc-c"}]}
				]
			}
		}`)
		if res.Kind != "network.http_route" {
			t.Errorf("kind = %q, want network.http_route", res.Kind)
		}
		parents, ok := res.Normalized["parents"].([]string)
		if !ok || len(parents) != 2 || parents[0] != "gw-1" || parents[1] != "gateway-infra/gw-2" {
			t.Errorf("parents = %#v, want [gw-1 gateway-infra/gw-2] (namespace 있으면 ns/ 접두)", parents)
		}
		targets, ok := res.Normalized["targets"].([]string)
		if !ok || len(targets) != 3 ||
			targets[0] != "svc-a:8080 (90%)" || targets[1] != "team-b/svc-b:9090 (10%)" || targets[2] != "svc-c" {
			t.Errorf("targets = %#v, want legacy collectHTTPRouteTargets 형식", targets)
		}
		// hostnames는 P1-D 조립 원천으로 확장됐다(판단 기록 P1-D-2): legacy
		// K8sIstioResourceItem.Hosts = joinAndLimit(spec.hostnames, 3)가 v1
		// 특성 표(buildAdvancedNetworkSection)로 고정돼 있어 J-P1-3의
		// parents·targets 2키만으로는 조립 동치가 불성립한다. mapping.md §4.8
		// 과 계획 J-P1-3 각주에 같은 근거로 기록된다.
		hostnames, ok := res.Normalized["hostnames"].([]string)
		if !ok || len(hostnames) != 1 || hostnames[0] != "route.example.com" {
			t.Errorf("hostnames = %#v, want [route.example.com] (P1-D 조립 원천 확장)", hostnames)
		}
	})
}

// TestGatewayAPIVersionFallback — J-P1-1 폴백 체인: v1 404 → v1beta1 서빙이면
// 섹션이 수집된다.
func TestGatewayAPIVersionFallback(t *testing.T) {
	mock := newMockK8s(t, "v1.29.4+k3s")
	// v1 경로는 시드가 없어 404 — v1beta1만 서빙한다(폴백 경로 실측).
	mock.lists["/apis/gateway.networking.k8s.io/v1beta1/gateways"] = []map[string]any{
		mock.obj(map[string]any{
			"metadata": map[string]any{"name": "gw-beta", "namespace": "default", "uid": "u-gw-beta"},
			"spec": map[string]any{
				"gatewayClassName": "istio",
				"listeners": []map[string]any{
					{"name": "http", "hostname": "beta.example.com", "port": 80, "protocol": "HTTP"},
				},
			},
		}),
	}
	fixture := &k8sFixture{t: t, mock: mock, adapter: NewAdapter(WithPageSize(4)), kubeconfig: kubeconfigFor(mock.srv.URL)}

	var gateways []contract.DiscoveredResource
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
			if res.Kind == "network.gateway" {
				gateways = append(gateways, res)
			}
		}
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}
	if len(gateways) != 1 || gateways[0].DisplayName != "gw-beta" {
		t.Fatalf("v1beta1 fallback gateways = %+v, want 1 gw-beta (v1 404 → v1beta1 폴백)", gateways)
	}
	if hosts, ok := gateways[0].Normalized["hosts"].([]string); !ok || len(hosts) != 1 || hosts[0] != "beta.example.com" {
		t.Errorf("fallback hosts = %#v, want [beta.example.com]", gateways[0].Normalized["hosts"])
	}
}

// TestGatewayAPISectionSkipsWhenCRDAbsent — J-P1-8(M-6): 전 버전 404면 섹션
// 스킵(공허 녹색)이고 워크는 끝까지 오류 없이 통과한다.
func TestGatewayAPISectionSkipsWhenCRDAbsent(t *testing.T) {
	mock := newMockK8s(t, "v1.29.4+k3s")
	// gateways·httproutes 경로는 어느 버전도 시드하지 않는다 — 전부 404.
	fixture := &k8sFixture{t: t, mock: mock, adapter: NewAdapter(WithPageSize(4)), kubeconfig: kubeconfigFor(mock.srv.URL)}

	cursor := ""
	for pages := 0; pages < 64; pages++ {
		page, err := fixture.adapter.Discover(context.Background(), contract.DiscoverRequest{
			ContextID: 1, Cursor: cursor,
			Connection: contract.ConnectionView{Material: map[string]string{"inventory": fixture.kubeconfig}},
		})
		if err != nil {
			t.Fatalf("Discover(cursor=%q): %v — CRD 부재 404는 스킵 신호다(J-P1-8)", cursor, err)
		}
		for _, res := range page.Resources {
			if res.Kind == "network.gateway" || res.Kind == "network.http_route" {
				t.Fatalf("CRD 부재 클러스터에서 %s 리소스가 나왔다: %+v", res.Kind, res)
			}
		}
		if page.NextCursor == "" {
			return
		}
		cursor = page.NextCursor
	}
	t.Fatal("cursor chain did not terminate")
}
