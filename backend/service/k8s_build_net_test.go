// k8s_build_net_test.go — Phase Z2 characterization (k8s_build_net 시맨 순수 함수 26개).
// 목적은 "옳음"이 아니라 "불변": 현재 동작을 그대로 기록해 B~D2 파일 분해의 유일한
// 검출기이자 P의 V2 동치 오라클이 되게 한다 (계획 §J0·§12 #16, R18 — legacy 호출 1행).
package service

import (
	"encoding/json"
	"reflect"
	"testing"

	"ops-admin/backend/model"
)

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
func TestCharBuildAdvancedNetworkSection(t *testing.T) {
	gwLate := charGatewayAPI("zeta-gw", "ns2")
	gwLate.Spec.GatewayClassName = "istio"
	gwLate.Spec.Listeners = append(gwLate.Spec.Listeners, charGatewayListener("gw.example", 8443, "HTTPS"))
	gwLate.Status.Addresses = append(gwLate.Status.Addresses, charGatewayAddress("1.2.3.4"))
	gwEarly := charGatewayAPI("alpha-gw", "ns1")
	route := charHTTPRoute("route-b", "ns1")
	charDecode(t, `{"hostnames":["a.example","b.example","c.example","d.example"," "],"parentRefs":[{"name":"zeta-gw"}],"rules":[{}]}`, &route.Spec)
	cases := []struct {
		name     string
		gateways []kubeGatewayAPI
		routes   []kubeHTTPRoute
		check    func(*testing.T, model.K8sAdvancedNetworkSection)
	}{
		{name: "조립·정렬", gateways: []kubeGatewayAPI{gwLate, gwEarly}, routes: []kubeHTTPRoute{route}, check: func(t *testing.T, section model.K8sAdvancedNetworkSection) {
			if len(section.GatewayAPIGateways) != 2 || len(section.HTTPRoutes) != 1 {
				t.Fatalf("counts = %d/%d, want 2/1", len(section.GatewayAPIGateways), len(section.HTTPRoutes))
			}
			gateway := section.GatewayAPIGateways[0]
			if gateway.Name != "alpha-gw" || gateway.Namespace != "ns1" || gateway.Kind != "Gateway" || gateway.Hosts != "-" || gateway.Target != "-" {
				t.Errorf("gw[0] = %+v", gateway)
			}
			gateway = section.GatewayAPIGateways[1]
			if gateway.Name != "zeta-gw" || gateway.Kind != "Gateway" || gateway.Hosts != "gw.example" || gateway.Address != "1.2.3.4" ||
				gateway.Ports != "8443/HTTPS" || gateway.Target != "istio" {
				t.Errorf("gw[1] = %+v", gateway)
			}
			httpRoute := section.HTTPRoutes[0]
			if httpRoute.Name != "route-b" || httpRoute.Kind != "HTTPRoute" || httpRoute.Hosts != "a.example, b.example, c.example +1" ||
				httpRoute.Gateways != "zeta-gw" || httpRoute.Target != "-" {
				t.Errorf("route[0] = %+v", httpRoute)
			}
		}},
		{name: "빈 입력 → 빈 섹션", check: func(t *testing.T, section model.K8sAdvancedNetworkSection) {
			if section.GatewayAPIGateways == nil || len(section.GatewayAPIGateways) != 0 || section.HTTPRoutes == nil || len(section.HTTPRoutes) != 0 {
				t.Errorf("section = %+v, want 빈 슬라이스", section)
			}
		}},
	}
	for _, tc := range cases {
		section := buildAdvancedNetworkSection(tc.gateways, tc.routes, nil) // legacy 1행
		tc.check(t, section)
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
func TestCharCompareIstioItems(t *testing.T) {
	cases := []struct {
		name  string
		left  model.K8sIstioResourceItem
		right model.K8sIstioResourceItem
		want  bool
	}{
		{name: "네임스페이스 다름", left: model.K8sIstioResourceItem{Namespace: "a"}, right: model.K8sIstioResourceItem{Namespace: "b"}, want: true},
		{name: "네임스페이스 다름 역순", left: model.K8sIstioResourceItem{Namespace: "b"}, right: model.K8sIstioResourceItem{Namespace: "a"}, want: false},
		{name: "같은 네임스페이스 이름순", left: model.K8sIstioResourceItem{Namespace: "ns", Name: "a"}, right: model.K8sIstioResourceItem{Namespace: "ns", Name: "b"}, want: true},
		{name: "동일 항목", left: model.K8sIstioResourceItem{Namespace: "ns", Name: "a"}, right: model.K8sIstioResourceItem{Namespace: "ns", Name: "a"}, want: false},
	}
	for _, tc := range cases {
		if got := compareIstioItems(tc.left, tc.right); got != tc.want { // legacy 1행
			t.Errorf("%s: = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
func TestCharJoinAndLimit(t *testing.T) {
	cases := []struct {
		name   string
		values []string
		limit  int
		want   string
	}{
		{name: "빈 입력 → -", values: nil, limit: 3, want: "-"},
		{name: "제한 이하", values: []string{"a", "b"}, limit: 3, want: "a, b"},
		{name: "제한 초과", values: []string{"a", "b", "c", "d"}, limit: 3, want: "a, b, c +1"},
		{name: "중복·빈값·대시 제거", values: []string{"a", "a", " ", "", "-", "b"}, limit: 3, want: "a, b"},
		{name: "limit 0은 무제한", values: []string{"a", "b", "c"}, limit: 0, want: "a, b, c"},
	}
	for _, tc := range cases {
		if got := joinAndLimit(tc.values, tc.limit); got != tc.want { // legacy 1행
			t.Errorf("%s: = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
func TestCharUniqueNonEmptyStrings(t *testing.T) {
	cases := []struct {
		name   string
		values []string
		want   []string
	}{
		{name: "trim·제외·중복 제거·순서 보존", values: []string{" x ", "x", "", "  ", "-", "y"}, want: []string{"x", "y"}},
		{name: "빈 입력 → 빈 슬라이스", values: nil, want: []string{}},
	}
	for _, tc := range cases {
		if got := uniqueNonEmptyStrings(tc.values); !reflect.DeepEqual(got, tc.want) { // legacy 1행
			t.Errorf("%s: = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
func TestCharFormatIstioPort(t *testing.T) {
	cases := []struct {
		name     string
		number   int
		protocol string
		want     string
	}{
		{name: "일반", number: 80, protocol: "HTTP", want: "80/HTTP"},
		{name: "빈 프로토콜 → TCP", number: 443, protocol: "", want: "443/TCP"},
		{name: "number 0 → 프로토콜만", number: 0, protocol: "HTTPS", want: "HTTPS"},
		{name: "number 0 프로토콜 빈값 → -", number: 0, protocol: "  ", want: "-"},
	}
	for _, tc := range cases {
		if got := formatIstioPort(tc.number, tc.protocol); got != tc.want { // legacy 1행
			t.Errorf("%s: = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
func TestCharJoinSelector(t *testing.T) {
	cases := []struct {
		name     string
		selector map[string]string
		want     string
	}{
		{name: "빈 셀렉터 → -", selector: nil, want: "-"},
		{name: "키 정렬", selector: map[string]string{"app": "web", "tier": "fe"}, want: "app=web, tier=fe"},
	}
	for _, tc := range cases {
		if got := joinSelector(tc.selector); got != tc.want { // legacy 1행
			t.Errorf("%s: = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
func TestCharCollectVirtualServiceTargets(t *testing.T) {
	cases := []struct {
		name string
		spec string
		want []string
	}{
		{name: "http+tcp 혼합", spec: `{"http":[{"route":[{"destination":{"host":"reviews","subset":"v2","port":{"number":8080}}},{"destination":{"host":"reviews"}}]}],"tcp":[{"route":[{"destination":{"host":"db","port":{"number":5432}}}]}]}`,
			want: []string{"reviews:v2:8080", "reviews", "db:5432"}},
		{name: "빈 spec → 빈 슬라이스", spec: "", want: []string{}},
	}
	for _, tc := range cases {
		virtualService := charVirtualService("reviews", "default")
		if tc.spec != "" {
			charDecode(t, tc.spec, &virtualService.Spec)
		}
		if got := collectVirtualServiceTargets(virtualService); !reflect.DeepEqual(got, tc.want) { // legacy 1행
			t.Errorf("%s: targets = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
func TestCharBuildVirtualServiceTrafficItems(t *testing.T) {
	cases := []struct {
		name string
		spec string
		want []model.K8sIstioTrafficRoute
	}{
		{
			name: "route 있는 첫 http 항목",
			spec: `{"http":[{"route":[]},{"route":[{"destination":{"host":"reviews","subset":"v2","port":{"number":8080}},"weight":90},{"destination":{"host":"canary"},"weight":10}]}]}`,
			want: []model.K8sIstioTrafficRoute{
				{Index: 0, Host: "reviews", Subset: "v2", Port: 8080, Weight: 90, Label: "reviews / v2:8080"},
				{Index: 1, Host: "canary", Weight: 10, Label: "canary"}},
		},
		{name: "route 없으면 nil", spec: "", want: nil},
	}
	for _, tc := range cases {
		virtualService := charVirtualService("reviews", "default")
		if tc.spec != "" {
			charDecode(t, tc.spec, &virtualService.Spec)
		}
		if got := buildVirtualServiceTrafficItems(virtualService); !reflect.DeepEqual(got, tc.want) { // legacy 1행
			t.Errorf("%s: items = %+v, want %+v", tc.name, got, tc.want)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
func TestCharFirstVirtualServiceHTTPRouteIndex(t *testing.T) {
	cases := []struct {
		name string
		spec string
		want int
	}{
		{name: "route 있는 첫 항목", spec: `{"http":[{"route":[]},{"route":[{"destination":{"host":"x"}}]}]}`, want: 1},
		{name: "없으면 -1", spec: "", want: -1},
	}
	for _, tc := range cases {
		virtualService := charVirtualService("reviews", "default")
		if tc.spec != "" {
			charDecode(t, tc.spec, &virtualService.Spec)
		}
		if got := firstVirtualServiceHTTPRouteIndex(virtualService); got != tc.want { // legacy 1행
			t.Errorf("%s: = %d, want %d", tc.name, got, tc.want)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
func TestCharCollectGatewayAPIHosts(t *testing.T) {
	gateway := charGatewayAPI("gw", "default")
	gateway.Spec.Listeners = append(gateway.Spec.Listeners,
		charGatewayListener("a.example", 0, ""), charGatewayListener("a.example", 0, ""), charGatewayListener("", 0, ""))
	if got := v2CollectGatewayAPIHosts(gateway); !reflect.DeepEqual(got, []string{"a.example", "*"}) { // v2 oracle
		t.Errorf("hosts = %v, want [a.example *]", got)
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
func TestCharCollectGatewayAPIPorts(t *testing.T) {
	gateway := charGatewayAPI("gw", "default")
	gateway.Spec.Listeners = append(gateway.Spec.Listeners,
		charGatewayListener("", 80, "HTTP"), charGatewayListener("", 80, "HTTP"), charGatewayListener("", 0, "HTTPS"))
	if got := v2CollectGatewayAPIPorts(gateway); !reflect.DeepEqual(got, []string{"80/HTTP", "HTTPS"}) { // v2 oracle
		t.Errorf("ports = %v, want [80/HTTP HTTPS]", got)
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
func TestCharCollectGatewayAPIAddresses(t *testing.T) {
	gateway := charGatewayAPI("gw", "default")
	gateway.Status.Addresses = append(gateway.Status.Addresses,
		charGatewayAddress("1.1.1.1"), charGatewayAddress("1.1.1.1"), charGatewayAddress(" "))
	if got := v2CollectGatewayAPIAddresses(gateway); !reflect.DeepEqual(got, []string{"1.1.1.1"}) { // v2 oracle
		t.Errorf("addresses = %v, want [1.1.1.1]", got)
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// 주소 없으면 같은 네임스페이스의 "<name>"·"<name>-istio"·"<name>-*" 서비스 ExternalIP로 폴백, 최종 "-"가 현재 계약.
func TestCharResolveGatewayAPIAddress(t *testing.T) {
	serviceWith := func(name, namespace, externalIP string) kubeService {
		var service kubeService
		service.Metadata = charMeta(name, namespace)
		service.Spec.ExternalIPs = []string{externalIP}
		return service
	}
	gateway := charGatewayAPI("web-gw", "prod")
	withAddress := charGatewayAPI("addr-gw", "prod")
	withAddress.Status.Addresses = append(withAddress.Status.Addresses, charGatewayAddress("9.9.9.9"))
	services := []kubeService{
		serviceWith("unrelated", "prod", "3.3.3.3"),
		serviceWith("web-gw-istio", "other", "2.2.2.2"),
		serviceWith("web-gw-lb", "prod", "1.1.1.1"),
	}
	cases := []struct {
		name string
		item kubeGatewayAPI
		want string
	}{
		{name: "status 주소 우선", item: withAddress, want: "9.9.9.9"},
		{name: "서비스 폴백(같은 ns·접두 매칭)", item: gateway, want: "1.1.1.1"},
		{name: "매칭 없음 → -", item: charGatewayAPI("none", "other"), want: "-"},
	}
	for _, tc := range cases {
		if got := v2ResolveGatewayAPIAddress(tc.item, services); got != tc.want { // v2 oracle
			t.Errorf("%s: = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
func TestCharCollectHTTPRouteParents(t *testing.T) {
	route := charHTTPRoute("route", "default")
	charDecode(t, `{"parentRefs":[{"name":"gw","namespace":"istio"},{"name":"gw"},{"name":"  "}]}`, &route.Spec)
	if got := v2CollectHTTPRouteParents(route); !reflect.DeepEqual(got, []string{"istio/gw", "gw"}) { // v2 oracle
		t.Errorf("parents = %v, want [istio/gw gw]", got)
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
func TestCharCollectHTTPRouteTargets(t *testing.T) {
	route := charHTTPRoute("route", "default")
	specJSON := `{"rules":[{"backendRefs":[{"name":"a","port":80}]},{"backendRefs":[{"name":"b","namespace":"other","port":8080,"weight":90},{"name":"c"}]}]}`
	if err := json.Unmarshal([]byte(specJSON), &route.Spec); err != nil {
		t.Fatal(err)
	}
	if got := v2CollectHTTPRouteTargets(route); !reflect.DeepEqual(got, []string{"a:80", "other/b:8080 (90%)", "c"}) { // v2 oracle
		t.Errorf("targets = %v, want [a:80 other/b:8080 (90%%) c]", got)
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
func TestCharCollectHTTPRouteTargetsFromRule(t *testing.T) {
	route := charHTTPRoute("route", "default")
	specJSON := `{"rules":[{"backendRefs":[{"name":"b","namespace":"other","port":8080,"weight":90},{"name":"c"}]}]}`
	if err := json.Unmarshal([]byte(specJSON), &route.Spec); err != nil {
		t.Fatal(err)
	}
	got := collectHTTPRouteTargetsFromRule(route.Spec.Rules[0]) // legacy 1행
	if want := []string{"other/b:8080 (90%)", "c"}; !reflect.DeepEqual(got, want) {
		t.Errorf("targets = %v, want %v", got, want)
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
func TestCharBuildHTTPRouteTrafficItems(t *testing.T) {
	cases := []struct {
		name string
		spec string
		want []model.K8sIstioTrafficRoute
	}{
		{
			name: "backend 있는 첫 rule",
			spec: `{"rules":[{"backendRefs":[]},{"backendRefs":[{"name":"a","port":80,"weight":100},{"name":"b","namespace":"other"}]}]}`,
			want: []model.K8sIstioTrafficRoute{
				{Index: 0, Host: "a", Port: 80, Weight: 100, Label: "a:80"},
				{Index: 1, Host: "b", Label: "other/b"}},
		},
		{name: "backend 없으면 nil", spec: "", want: nil},
	}
	for _, tc := range cases {
		route := charHTTPRoute("route", "default")
		if tc.spec != "" {
			charDecode(t, tc.spec, &route.Spec)
		}
		// v2 미대응 사유 처분(J-P1-5 (iii)): HTTPRoute 트래픽 항목은 Index·Port·
		// Weight 구조체를 지니는 v1 상세 뷰다 — V2는 targets를 문자열 목록 키로
		// 정규화(lossy)해 구조체를 재구성할 수 없어 교체 불가.
		if got := buildHTTPRouteTrafficItems(route); !reflect.DeepEqual(got, tc.want) { // legacy 1행
			t.Errorf("%s: items = %+v, want %+v", tc.name, got, tc.want)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
func TestCharFirstHTTPRouteRuleIndex(t *testing.T) {
	cases := []struct {
		name string
		spec string
		want int
	}{
		{name: "backend 있는 첫 rule", spec: `{"rules":[{"backendRefs":[]},{"backendRefs":[{"name":"a"}]}]}`, want: 1},
		{name: "없으면 -1", spec: "", want: -1},
	}
	for _, tc := range cases {
		route := charHTTPRoute("route", "default")
		if tc.spec != "" {
			charDecode(t, tc.spec, &route.Spec)
		}
		if got := firstHTTPRouteRuleIndex(route); got != tc.want { // legacy 1행
			t.Errorf("%s: = %d, want %d", tc.name, got, tc.want)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
func TestCharBuildNetworkSection(t *testing.T) {
	headless := kubeService{Metadata: charMeta("zeta", "default")}
	headless.Spec.ClusterIP = "None"
	normal := kubeService{Metadata: charMeta("alpha", "default")}
	normal.Spec.Type = "ClusterIP"
	normal.Spec.ClusterIP = "10.0.0.1"
	charDecode(t, `{"ports":[{"port":80,"nodePort":30080,"protocol":"TCP"}]}`, &normal.Spec)
	tlsIngress := kubeIngress{Metadata: charMeta("b-ing", "default")}
	charDecode(t, `{"rules":[{"host":"a.example"}],"tls":[{}]}`, &tlsIngress.Spec)
	tlsIngress.Status.LoadBalancer.Ingress = append(tlsIngress.Status.LoadBalancer.Ingress, struct {
		IP       string `json:"ip"`
		Hostname string `json:"hostname"`
	}{IP: "1.1.1.1"})
	plainIngress := kubeIngress{Metadata: charMeta("a-ing", "other")}
	cases := []struct {
		name      string
		services  []kubeService
		ingresses []kubeIngress
		check     func(*testing.T, model.K8sNetworkSection)
	}{
		{name: "조립·정렬", services: []kubeService{headless, normal}, ingresses: []kubeIngress{tlsIngress, plainIngress}, check: func(t *testing.T, section model.K8sNetworkSection) {
			if len(section.Services) != 2 || len(section.Ingresses) != 2 {
				t.Fatalf("counts = %d/%d, want 2/2", len(section.Services), len(section.Ingresses))
			}
			service := section.Services[0]
			if service.Name != "alpha" || service.Type != "ClusterIP" || service.ClusterIP != "10.0.0.1" || service.Ports != "80:30080/TCP" || service.Endpoints != 2 || service.Age != "2026-01-01" {
				t.Errorf("service[0] = %+v", service)
			}
			if section.Services[1].Type != "Headless" || section.Services[1].ClusterIP != "None" {
				t.Errorf("service[1] = %+v", section.Services[1])
			}
			ingress := section.Ingresses[0]
			if ingress.Name != "b-ing" || ingress.Host != "a.example" || ingress.Address != "1.1.1.1" || ingress.TLS != "Enabled" {
				t.Errorf("ingress[0] = %+v", ingress)
			}
			ingress = section.Ingresses[1]
			if ingress.Name != "a-ing" || ingress.Namespace != "other" || ingress.Host != "-" || ingress.Address != "-" || ingress.TLS != "Disabled" {
				t.Errorf("ingress[1] = %+v", ingress)
			}
		}},
		{name: "빈 입력 → 빈 섹션", check: func(t *testing.T, section model.K8sNetworkSection) {
			if section.Services == nil || len(section.Services) != 0 || section.Ingresses == nil || len(section.Ingresses) != 0 {
				t.Errorf("section = %+v, want 빈 슬라이스", section)
			}
		}},
	}
	for _, tc := range cases {
		section := buildNetworkSection(tc.services, tc.ingresses, map[string]int{"default/alpha": 2}) // legacy 1행
		tc.check(t, section)
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
func TestCharServiceDisplayType(t *testing.T) {
	cases := []struct {
		name      string
		clusterIP string
		serviceTy string
		want      string
	}{
		{name: "None → Headless", clusterIP: "None", serviceTy: "ClusterIP", want: "Headless"},
		{name: "소문자 none도 Headless", clusterIP: "none", want: "Headless"},
		{name: "일반 타입", clusterIP: "10.0.0.1", serviceTy: "NodePort", want: "NodePort"},
		{name: "타입 빈값 → -", want: "-"},
	}
	for _, tc := range cases {
		var service kubeService
		service.Spec.ClusterIP = tc.clusterIP
		service.Spec.Type = tc.serviceTy
		if got := serviceDisplayType(service); got != tc.want { // legacy 1행
			t.Errorf("%s: = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// hostPath가 NFS보다 우선이고 둘 다 없으면 "-" 삼중이 현재 계약.
func TestCharPersistentVolumeSource(t *testing.T) {
	withHostPath := charPV("pv1")
	charDecode(t, `{"hostPath":{"path":"/mnt/data"}}`, &withHostPath.Spec)
	withNFS := charPV("pv2")
	charDecode(t, `{"nfs":{"server":"nfs.local","path":"/exports"}}`, &withNFS.Spec)
	cases := []struct {
		name          string
		item          kubePersistentVolume
		wantType      string
		wantPath      string
		wantNFSServer string
	}{
		{name: "hostPath 우선", item: withHostPath, wantType: "hostPath", wantPath: "/mnt/data", wantNFSServer: "-"},
		{name: "NFS", item: withNFS, wantType: "NFS", wantPath: "/exports", wantNFSServer: "nfs.local"},
		{name: "둘 다 없음", item: charPV("pv3"), wantType: "-", wantPath: "-", wantNFSServer: "-"},
	}
	for _, tc := range cases {
		sourceType, path, nfsServer := v2PersistentVolumeSource(tc.item) // v2 oracle
		if sourceType != tc.wantType || path != tc.wantPath || nfsServer != tc.wantNFSServer {
			t.Errorf("%s: = (%q, %q, %q), want (%q, %q, %q)", tc.name, sourceType, path, nfsServer, tc.wantType, tc.wantPath, tc.wantNFSServer)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// 어노테이션 없음·빈값은 "Cluster-scoped" 폴백이 현재 계약.
func TestCharStorageNamespaceScope(t *testing.T) {
	cases := []struct {
		name        string
		annotations map[string]string
		want        string
	}{
		{name: "어노테이션 값", annotations: map[string]string{"ops-admin.io/namespace-scope": "team-a"}, want: "team-a"},
		{name: "공백값 → 폴백", annotations: map[string]string{"ops-admin.io/namespace-scope": "  "}, want: "Cluster-scoped"},
		{name: "맵 nil → 폴백", annotations: nil, want: "Cluster-scoped"},
	}
	for _, tc := range cases {
		if got := v2StorageNamespaceScope(tc.annotations); got != tc.want { // v2 oracle
			t.Errorf("%s: = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// PVC는 요청 용량 우선, PV는 status 우선 용량·소스 산식, 정렬은 Kind(PV<PVC)→ns→name이 현재 계약.
func TestCharBuildConfigStorageSection(t *testing.T) {
	configMap := kubeConfigMap{Metadata: charMeta("cm1", "default"), Data: map[string]string{"a": "1", "b": "2"}, Binary: map[string]string{"c": "3"}}
	secret := kubeSecret{Metadata: charMeta("sec1", "default"), Type: "Opaque"}
	emptySecret := kubeSecret{Metadata: charMeta("sec2", "default")}
	var pvc kubePersistentVolumeClaim
	charDecode(t, `{"metadata":{"name":"claim","namespace":"team-a","creationTimestamp":"2026-01-01T00:00:00Z"},"spec":{"storageClassName":"standard","accessModes":["RWO"],"resources":{"requests":{"storage":"5Gi"}}},"status":{"phase":"Bound","capacity":{"storage":"10Gi"}}}`, &pvc)
	pv := charPV("vol")
	charDecode(t, `{"hostPath":{"path":"/mnt/data"}}`, &pv.Spec)
	pv.Metadata.Annotations = map[string]string{"ops-admin.io/namespace-scope": "team-a"}
	pv.Status.Phase = "Available"
	pv.Spec.Capacity = map[string]string{"storage": "10Gi"}
	pv.Spec.PersistentVolumeReclaimPolicy = "Retain"
	pv.Spec.AccessModes = []string{"RWO"}
	populated := []kubeConfigMap{configMap}
	populatedSecrets := []kubeSecret{secret, emptySecret}
	populatedClaims := []kubePersistentVolumeClaim{pvc}
	populatedVolumes := []kubePersistentVolume{pv}
	cases := []struct {
		name       string
		configMaps []kubeConfigMap
		secrets    []kubeSecret
		pvcs       []kubePersistentVolumeClaim
		pvs        []kubePersistentVolume
		check      func(*testing.T, model.K8sConfigStorageSection)
	}{
		{name: "조립·용량 우선순위·정렬", configMaps: populated, secrets: populatedSecrets, pvcs: populatedClaims, pvs: populatedVolumes, check: func(t *testing.T, section model.K8sConfigStorageSection) {
			if len(section.ConfigMaps) != 1 || section.ConfigMaps[0].Keys != 3 {
				t.Errorf("configMaps = %+v", section.ConfigMaps)
			}
			if len(section.Secrets) != 2 || section.Secrets[0].Type != "Opaque" || section.Secrets[1].Type != "-" {
				t.Errorf("secrets = %+v", section.Secrets)
			}
			if len(section.Storage) != 2 {
				t.Fatalf("storage = %+v", section.Storage)
			}
			volume := section.Storage[0]
			if volume.Kind != "PV" || volume.NamespaceScope != "team-a" || volume.SourceType != "hostPath" || volume.Path != "/mnt/data" ||
				volume.NFSServer != "-" || volume.Capacity != "10Gi" || volume.ReclaimPolicy != "Retain" || volume.AccessModes != "RWO" {
				t.Errorf("storage[0](PV) = %+v", volume)
			}
			claim := section.Storage[1]
			if claim.Kind != "PVC" || claim.Namespace != "team-a" || claim.Status != "Bound" || claim.Capacity != "5Gi" || claim.StorageClass != "standard" {
				t.Errorf("storage[1](PVC) = %+v", claim)
			}
		}},
		{name: "빈 입력 → 빈 섹션", check: func(t *testing.T, section model.K8sConfigStorageSection) {
			if section.ConfigMaps == nil || section.Secrets == nil || section.Storage == nil {
				t.Errorf("section = %+v, want 빈 슬라이스", section)
			}
		}},
	}
	for _, tc := range cases {
		section := buildConfigStorageSection(tc.configMaps, tc.secrets, tc.pvcs, tc.pvs) // legacy 1행
		tc.check(t, section)
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// 노드는 Ready 아니거나 unschedulable이면 알림 1, 파드는 failed/pending/unknown이 알림, 요청치는 컨테이너 합산이 현재 계약.
func TestCharCalculateK8sAggregateMetrics(t *testing.T) {
	readyNode := charNodeReady("ready", "", "True", "", "", "500m", "1Gi", "")
	unschedulable := charNodeReady("cordoned", "", "True", "", "", "1", "2Gi", "")
	unschedulable.Spec.Unschedulable = true
	notReady := charNodeReady("down", "", "False", "", "", "", "", "")
	runningPod := charPod("ok", "default")
	runningPod.Status.Phase = "Running"
	runningPod.Spec.Containers = append(runningPod.Spec.Containers, kubeContainer{Name: "c"})
	runningPod.Spec.Containers[0].Resources.Requests = map[string]string{"cpu": "250m", "memory": "128Mi"}
	pendingPod := charPod("stuck", "default")
	pendingPod.Status.Phase = "Pending"
	failedPod := charPod("dead", "default")
	failedPod.Status.Phase = "Failed"
	unknownPod := charPod("ghost", "default")
	unknownPod.Status.Phase = "Unknown"
	cases := []struct {
		name  string
		nodes []kubeNode
		pods  []kubePod
		want  k8sAggregateMetrics
	}{
		{
			name:  "알림 판정·요청치 합산",
			nodes: []kubeNode{readyNode, unschedulable, notReady},
			pods:  []kubePod{runningPod, pendingPod, failedPod, unknownPod},
			want:  k8sAggregateMetrics{TotalAllocCPUMilli: 1500, TotalAllocMemoryBytes: 3 * 1024 * 1024 * 1024, TotalReqCPUMilli: 250, TotalReqMemoryBytes: 128 * 1024 * 1024, AlertCount: 5},
		},
		{name: "빈 입력 → 제로값", want: k8sAggregateMetrics{}},
	}
	for _, tc := range cases {
		if got := v2CalculateK8sAggregateMetrics(tc.nodes, tc.pods); !reflect.DeepEqual(got, tc.want) { // v2 oracle
			t.Errorf("%s: metrics = %+v, want %+v", tc.name, got, tc.want)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
func TestCharToK8sClusterView(t *testing.T) {
	gatewayID := uint(7)
	datasourceID := uint(9)
	cases := []struct {
		name    string
		cluster model.K8sCluster
		want    model.K8sClusterView
	}{
		{
			name: "전체 매핑",
			cluster: model.K8sCluster{ID: 3, Name: "prod", Status: "warning", APIServer: "https://api:6443", Version: "v1.29.0",
				NodeCount: 2, Env: "prod", Tags: []string{"x"}, ConnectionMode: "Gateway", GatewayID: &gatewayID,
				MonitorDatasourceID: &datasourceID, Description: "d", KubeConfig: "secret"},
			want: model.K8sClusterView{ID: 3, Name: "prod", Status: "warning", StatusText: "Partial Alerts", APIServer: "https://api:6443",
				Version: "v1.29.0", NodeCount: 2, Env: "prod", Tags: []string{"x"}, ConnectionMode: "gateway", GatewayID: &gatewayID,
				GatewayName: "gw1", MonitorDatasourceID: &datasourceID, MonitorDatasourceName: "ds1", Description: "d"},
		},
		{name: "오프라인·빈 모드", cluster: model.K8sCluster{Status: "offline"},
			want: model.K8sClusterView{Status: "offline", StatusText: "Offline", ConnectionMode: "direct", GatewayName: "gw1", MonitorDatasourceName: "ds1"}},
	}
	for _, tc := range cases {
		tc.cluster.Gateway.Name = "gw1"
		tc.cluster.MonitorDatasource.Name = "ds1"
		if got := toK8sClusterView(tc.cluster); !reflect.DeepEqual(got, tc.want) { // legacy 1행
			t.Errorf("%s: view = %+v, want %+v", tc.name, got, tc.want)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// 검증 순서 name → kubeconfig → env → gateway 선택, 게이트웨이 모드는 ID 필요가 현재 계약.
func TestCharValidateK8sClusterPayload(t *testing.T) {
	gatewayID := uint(7)
	zero := uint(0)
	valid := model.K8sCluster{Name: "prod", KubeConfig: "cfg", Env: "prod"}
	cases := []struct {
		name    string
		cluster model.K8sCluster
		wantErr string
	}{
		{name: "이름 없음", wantErr: "cluster name is required"},
		{name: "kubeconfig 없음", cluster: model.K8sCluster{Name: "prod"}, wantErr: "kubeconfig is required"},
		{name: "env 없음", cluster: model.K8sCluster{Name: "prod", KubeConfig: "cfg"}, wantErr: "select an environment"},
		{name: "게이트웨이 모드 ID 없음", cluster: model.K8sCluster{Name: "prod", KubeConfig: "cfg", Env: "prod", ConnectionMode: "gateway"}, wantErr: "select an access gateway"},
		{name: "게이트웨이 모드 ID 0", cluster: model.K8sCluster{Name: "prod", KubeConfig: "cfg", Env: "prod", ConnectionMode: "gateway", GatewayID: &zero}, wantErr: "select an access gateway"},
		{name: "direct 유효", cluster: valid, wantErr: ""},
		{name: "게이트웨이 ID 있으면 유효", cluster: model.K8sCluster{Name: "prod", KubeConfig: "cfg", Env: "prod", ConnectionMode: "gateway", GatewayID: &gatewayID}, wantErr: ""},
	}
	for _, tc := range cases {
		err := validateK8sClusterPayload(tc.cluster) // legacy 1행
		if tc.wantErr == "" {
			if err != nil {
				t.Errorf("%s: err = %v, want nil", tc.name, err)
			}
			continue
		}
		if err == nil || err.Error() != tc.wantErr {
			t.Errorf("%s: err = %v, want %q", tc.name, err, tc.wantErr)
		}
	}
}
