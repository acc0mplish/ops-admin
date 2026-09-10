// k8s_fetch_test.go — Phase Z1 characterization (k8s_fetch 시맨 순수 함수 24개).
// 목적은 "옳음"이 아니라 "불변": 현재 동작을 그대로 기록해 B~D2 파일 분해의 유일한
// 검출기이자 P의 V2 동치 오라클이 되게 한다 (계획 §J0·§12 #16, R18 — legacy 호출 1행).
package service

import (
	"reflect"
	"testing"
	"time"

	"ops-admin/backend/internal/infra/adapter/kubernetes"
	"ops-admin/backend/model"
)

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// 미수집 선택 경로(ingresses·gatewayapi 등)는 404여도 에러 없이 빈 값으로 간주되는 것이 현재 계약.
func TestCharFetchK8sData(t *testing.T) {
	cases := []struct {
		name       string
		routes     map[string]charStub
		dead       bool
		wantErr    string
		wantCounts map[string]int
	}{
		{name: "필수 경로 200 + 선택 경로 일부", routes: charCoreRoutes(), wantCounts: map[string]int{"Nodes": 1, "Namespaces": 1, "Pods": 1,
			"Services": 2, "Endpoints": 1, "ConfigMaps": 1, "Secrets": 1, "Deployments": 1, "Ingresses": 0, "PVCs": 0, "GatewayAPIGateways": 0, "HTTPRoutes": 0}},
		{name: "필수 경로 404 → 에러", routes: map[string]charStub{}, wantErr: "unexpected status: 404"},
		{name: "접속 실패 → 에러", dead: true, wantErr: "connect"},
	}
	for _, tc := range cases {
		client, runtime := charDeadKube()
		if !tc.dead {
			client, runtime = charStubKube(t, tc.routes)
		}
		data, err := fetchK8sData(client, runtime) // legacy 1행
		if charWantErr(t, tc.name, err, tc.wantErr) {
			continue
		}
		counts := charFetchedCounts(data)
		for field, want := range tc.wantCounts {
			if counts[field] != want {
				t.Errorf("%s: %s = %d, want %d", tc.name, field, counts[field], want)
			}
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// fieldSelector 쿼리는 RequestURI 매칭으로 간접 고정: 쿼리가 정확히 이 형태여야만 200이 난다.
func TestCharFetchPodsForNode(t *testing.T) {
	cases := []struct {
		name      string
		nodeName  string
		routes    map[string]charStub
		wantNames []string
		wantErr   string
	}{
		{name: "fieldSelector로 노드 필터 요청", nodeName: "node-1", routes: map[string]charStub{
			"/api/v1/pods?fieldSelector=spec.nodeName%3Dnode-1": {status: 200, body: charJSON(kubePodListResponse{Items: []kubePod{charPod("n1-pod", "default")}})},
		}, wantNames: []string{"n1-pod"}},
		{name: "404 → 에러", nodeName: "node-1", routes: map[string]charStub{}, wantErr: "unexpected status: 404"},
	}
	for _, tc := range cases {
		client, runtime := charStubKube(t, tc.routes)
		pods, err := fetchPodsForNode(client, runtime, tc.nodeName) // legacy 1행
		if charWantErr(t, tc.name, err, tc.wantErr) {
			continue
		}
		if names := charKubePodNames(pods); !reflect.DeepEqual(names, tc.wantNames) {
			t.Errorf("%s: pods = %v, want %v", tc.name, names, tc.wantNames)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
func TestCharFetchPodsByNamespace(t *testing.T) {
	body := charJSON(kubePodListResponse{Items: []kubePod{charPod("p1", "prod"), charPod("p2", "prod")}})
	cases := []struct {
		name      string
		namespace string
		routes    map[string]charStub
		wantNames []string
		wantErr   string
	}{
		{name: "성공", namespace: "prod", routes: map[string]charStub{
			"/api/v1/namespaces/prod/pods": {status: 200, body: body},
		}, wantNames: []string{"p1", "p2"}},
		{name: "404 → 에러", namespace: "prod", routes: map[string]charStub{}, wantErr: "unexpected status: 404"},
		{name: "다른 네임스페이스 경로는 미조회", namespace: "dev", routes: map[string]charStub{
			"/api/v1/namespaces/prod/pods": {status: 200, body: body},
		}, wantErr: "unexpected status: 404"},
	}
	for _, tc := range cases {
		client, runtime := charStubKube(t, tc.routes)
		pods, err := fetchPodsByNamespace(client, runtime, tc.namespace) // legacy 1행
		if charWantErr(t, tc.name, err, tc.wantErr) {
			continue
		}
		if names := charKubePodNames(pods); !reflect.DeepEqual(names, tc.wantNames) {
			t.Errorf("%s: pods = %v, want %v", tc.name, names, tc.wantNames)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// 빈 문자열 폴백 "-", LastTime 내림 정렬, RFC3339 → "2006-01-02 15:04" 포맷이 현재 계약.
func TestCharFetchNamespacedEvents(t *testing.T) {
	eventsBody := charEventsBody()
	cases := []struct {
		name    string
		routes  map[string]charStub
		want    []model.K8sEventItem
		wantErr string
	}{
		{
			name:   "정렬·빈값 폴백·타임스탬프 포맷",
			routes: map[string]charStub{"/api/v1/namespaces/default/events": {status: 200, body: eventsBody}},
			want: []model.K8sEventItem{
				{Type: "Warning", Reason: "BackOff", Message: "back-off", Count: 5, FirstTime: "2026-01-02 03:04", LastTime: "2026-01-02 03:05"},
				{Type: "-", Reason: "-", Message: "-", FirstTime: "-", LastTime: "2026-01-02 03:04"}}},
		{name: "500 → 연결 오류 상수", routes: map[string]charStub{"/api/v1/namespaces/default/events": {status: 500, body: "boom"}}, wantErr: k8sClusterConnectError},
	}
	for _, tc := range cases {
		client, runtime := charStubKube(t, tc.routes)
		events, err := fetchNamespacedEvents(client, runtime, "default", "involvedObject.name=x") // legacy 1행
		if charWantErr(t, tc.name, err, tc.wantErr) {
			continue
		}
		if !reflect.DeepEqual(events, tc.want) {
			t.Errorf("%s: events = %+v, want %+v", tc.name, events, tc.want)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// deployments는 필수(실패 시 즉시 0과 에러), 나머지 4종은 실패해도 무시하고 합산하는 것이 현재 계약.
func TestCharFetchNamespaceWorkloadCount(t *testing.T) {
	cases := []struct {
		name      string
		routes    map[string]charStub
		wantTotal int
		wantErr   string
	}{
		{name: "일부만 성공", routes: charNSWorkloadRoutes(2, 1, 0, 0, 0), wantTotal: 3},
		{name: "전부 성공", routes: charNSWorkloadRoutes(2, 1, 1, 1, 1), wantTotal: 6},
		{name: "deployments 실패 → 0과 에러", routes: map[string]charStub{}, wantErr: "unexpected status: 404"},
	}
	for _, tc := range cases {
		client, runtime := charStubKube(t, tc.routes)
		total, err := fetchNamespaceWorkloadCount(client, runtime, "prod") // legacy 1행
		if charWantErr(t, tc.name, err, tc.wantErr) {
			continue
		}
		if total != tc.wantTotal {
			t.Errorf("%s: total = %d, want %d", tc.name, total, tc.wantTotal)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
func TestCharMatchLabels(t *testing.T) {
	cases := []struct {
		name     string
		labels   map[string]string
		selector map[string]string
		want     bool
	}{
		{name: "빈 셀렉터 → false", labels: map[string]string{"app": "web"}, selector: nil, want: false},
		{name: "완전 일치", labels: map[string]string{"app": "web", "v": "2"}, selector: map[string]string{"app": "web"}, want: true},
		{name: "불일치", labels: map[string]string{"app": "web"}, selector: map[string]string{"app": "api"}, want: false},
		{name: "labels에 여분 키 허용", labels: map[string]string{"app": "web", "extra": "x"}, selector: map[string]string{"app": "web"}, want: true},
		{name: "셀렉터 키 누락", labels: map[string]string{"app": "web"}, selector: map[string]string{"tier": "fe"}, want: false},
	}
	for _, tc := range cases {
		if got := matchLabels(tc.labels, tc.selector); got != tc.want { // legacy 1행
			t.Errorf("%s: = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// 빈 셀렉터는 matchLabels가 false를 내므로 필터 결과가 0이 되는 것도 현재 계약.
func TestCharFilterPodsBySelector(t *testing.T) {
	pods := []kubePod{
		charPodWithLabels("p1", "default", map[string]string{"app": "web"}),
		charPodWithLabels("p2", "default", map[string]string{"app": "api"}),
		charPodWithLabels("p3", "prod", map[string]string{"app": "web"}),
	}
	cases := []struct {
		name      string
		namespace string
		selector  map[string]string
		want      []string
	}{
		{name: "일치만", namespace: "default", selector: map[string]string{"app": "web"}, want: []string{"p1"}},
		{name: "네임스페이스 필터", namespace: "prod", selector: map[string]string{"app": "web"}, want: []string{"p3"}},
		{name: "불일치 → 0", namespace: "default", selector: map[string]string{"app": "x"}, want: []string{}},
		{name: "빈 셀렉터 → 0", namespace: "default", selector: nil, want: []string{}},
	}
	for _, tc := range cases {
		got := filterPodsBySelector(pods, tc.namespace, tc.selector) // legacy 1행
		if names := charKubePodNames(got); !reflect.DeepEqual(names, tc.want) {
			t.Errorf("%s: pods = %v, want %v", tc.name, names, tc.want)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// 오너 매치는 Kind 대소문자를 무시하고, 오너·셀렉터 중 하나만 맞아도 통과하는 것이 현재 계약.
func TestCharFilterPodsByOwnerOrSelector(t *testing.T) {
	pods := []kubePod{
		charPodWithOwner("p1", "default", "ReplicaSet", "web-rs"),
		charPodWithLabels("p2", "default", map[string]string{"app": "web"}),
		charPodWithOwner("p3", "prod", "Job", "j1"),
		charPod("p4", "default"),
	}
	cases := []struct {
		name      string
		namespace string
		selector  map[string]string
		ownerKind string
		ownerName string
		want      []string
	}{
		{name: "오너 매치(대소문자 무시)", namespace: "default", ownerKind: "replicaset", ownerName: "web-rs", want: []string{"p1"}},
		{name: "셀렉터 폴백", namespace: "default", selector: map[string]string{"app": "web"}, want: []string{"p2"}},
		{name: "오너+셀렉터 병합", namespace: "default", selector: map[string]string{"app": "web"}, ownerKind: "ReplicaSet", ownerName: "web-rs", want: []string{"p1", "p2"}},
		{name: "네임스페이스 다름 → 제외", namespace: "prod", selector: map[string]string{"app": "web"}, want: []string{}},
		{name: "둘 다 불일치 → 제외", namespace: "default", selector: map[string]string{"app": "x"}, ownerKind: "Job", ownerName: "j1", want: []string{}},
	}
	for _, tc := range cases {
		got := filterPodsByOwnerOrSelector(pods, tc.namespace, tc.selector, tc.ownerKind, tc.ownerName) // legacy 1행
		if names := charKubePodNames(got); !reflect.DeepEqual(names, tc.want) {
			t.Errorf("%s: pods = %v, want %v", tc.name, names, tc.want)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// yaml.v3 특성 고정: 필드명은 json 태그가 아니라 Go 필드명 소문자화, 문자열 숫자는 인용.
// 주의: 직렬화 불가 타입(chan 등)은 에러가 아니라 panic이라 yaml.Marshal의 에러 분기(return "")
// 는 사실상 도달 불가 — 이 분기는 기록하지 않는다.
func TestCharMarshalK8sYAML(t *testing.T) {
	cases := []struct {
		name  string
		value any
		want  string
	}{
		{name: "맵 정렬·문자열 인용", value: map[string]string{"b": "2", "a": "1"}, want: "a: \"1\"\nb: \"2\"\n"},
		{name: "빈 맵", value: map[string]string{}, want: "{}\n"},
		{name: "nil", value: nil, want: "null\n"},
		{name: "구조체 필드 소문자화(json 태그 무시)", value: kubeEnvVar{Name: "A", Value: "1"}, want: "name: A\nvalue: \"1\"\nvaluefrom: {}\n"},
	}
	for _, tc := range cases {
		if got := marshalK8sYAML(tc.value); got != tc.want { // legacy 1행
			t.Errorf("%s: yaml = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// 라벨 순서와 intLabel("%d"+suffix) 형상이 현재 계약.
func TestCharBuildOverviewDistribution(t *testing.T) {
	kubeadm := charKubeadmConfigMap("serviceSubnet: 10.96.0.0/12\npodSubnet: 10.244.0.0/16")
	node := charNode("n1")
	node.Spec.PodCIDR = "10.244.1.0/24"
	cases := []struct {
		name    string
		cluster model.K8sClusterView
		nodes   []kubeNode
		cms     []kubeConfigMap
		want    [][2]string
	}{
		{
			name: "전부 존재", cluster: model.K8sClusterView{StatusText: "Running", Version: "v1.29.0", NodeCount: 3},
			nodes: []kubeNode{node}, cms: []kubeConfigMap{kubeadm},
			want: [][2]string{{"Cluster Status", "Running"}, {"Cluster Version", "v1.29.0"}, {"Node Count", "3 nodes"}, {"Service CIDR", "10.96.0.0/12"}, {"Pod Network", "10.244.0.0/16"}}},
		{name: "빈 값 폴백(-)과 CIDR Unknown", want: [][2]string{{"Cluster Status", ""}, {"Cluster Version", "-"}, {"Node Count", "0 nodes"}, {"Service CIDR", "Unknown"}, {"Pod Network", "Unknown"}}},
		{name: "serviceSubnet만 → pod는 노드 CIDR", nodes: []kubeNode{node}, cms: []kubeConfigMap{charKubeadmConfigMap("serviceSubnet: 10.96.0.0/12")}, want: [][2]string{{"Cluster Status", ""}, {"Cluster Version", "-"}, {"Node Count", "0 nodes"}, {"Service CIDR", "10.96.0.0/12"}, {"Pod Network", "10.244.1.0/24"}}},
	}
	for _, tc := range cases {
		items := v2BuildOverviewDistribution(tc.cluster, tc.nodes, tc.cms) // v2 oracle
		if len(items) != len(tc.want) {
			t.Fatalf("%s: items = %d, want %d", tc.name, len(items), len(tc.want))
		}
		for i, pair := range tc.want {
			if items[i].Label != pair[0] || items[i].Value != pair[1] {
				t.Errorf("%s: [%d] = %q/%q, want %q/%q", tc.name, i, items[i].Label, items[i].Value, pair[0], pair[1])
			}
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// kube-system/kubeadm-config만 파싱하고, 주석·따옴표를 벗기며, 노드 폴백은 정렬 후 "、" 결합.
func TestCharResolveK8sNetworkCIDRs(t *testing.T) {
	cases := []struct {
		name           string
		nodes          []kubeNode
		cms            []kubeConfigMap
		wantService    string
		wantPodNetwork string
	}{
		{name: "kubeadm-config 파싱(주석·따옴표)", cms: []kubeConfigMap{charKubeadmConfigMap("serviceSubnet: 10.96.0.0/12 # cluster\npodSubnet: \"10.244.0.0/16\"\nnetworking:\n  dnsDomain: cluster.local")}, wantService: "10.96.0.0/12", wantPodNetwork: "10.244.0.0/16"},
		{name: "잘못된 configmap 무시 → 노드 CIDR 폴백(정렬·、 결합)", nodes: []kubeNode{charCIDRNode("10.244.1.0/24", "10.244.2.0/24")}, cms: []kubeConfigMap{charKubeadmConfigMap("other: value")}, wantService: "Unknown", wantPodNetwork: "10.244.1.0/24、10.244.2.0/24"},
		{name: "이름이 다른 configmap 무시", cms: []kubeConfigMap{{Metadata: charMeta("other-cm", "kube-system"), Data: map[string]string{"ClusterConfiguration": "serviceSubnet: 10.0.0.0/8"}}}, wantService: "Unknown", wantPodNetwork: "Unknown"},
		{name: "노드 간 중복 제거", nodes: []kubeNode{charCIDRNode("10.244.1.0/24"), charCIDRNode("10.244.1.0/24")}, wantService: "Unknown", wantPodNetwork: "10.244.1.0/24"},
		{name: "둘 다 부재", wantService: "Unknown", wantPodNetwork: "Unknown"},
		{name: "configmap에 키 없음 → service는 Unknown, pod는 노드에서", nodes: []kubeNode{charCIDRNode("10.244.1.0/24")}, cms: []kubeConfigMap{charKubeadmConfigMap("dnsDomain: cluster.local")}, wantService: "Unknown", wantPodNetwork: "10.244.1.0/24"},
	}
	for _, tc := range cases {
		serviceCIDR, podCIDR := v2ResolveK8sNetworkCIDRs(tc.nodes, tc.cms) // v2 oracle
		if serviceCIDR != tc.wantService || podCIDR != tc.wantPodNetwork {
			t.Errorf("%s: = (%q, %q), want (%q, %q)", tc.name, serviceCIDR, podCIDR, tc.wantService, tc.wantPodNetwork)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// 이름·타입은 호출부가 넘긴 리터럴("CA Certificate"/"certificate-authority")이 현재 계약.
func TestCharBuildOverviewCertificates(t *testing.T) {
	now := time.Now()
	caPEM := charSelfSignedCertPEM("char-ca", now.Add(-time.Hour), now.Add(365*24*time.Hour))
	clientPEM := charSelfSignedCertPEM("char-client", now.Add(-time.Hour), now.Add(60*24*time.Hour))
	type wantCert struct{ name, typ, subject string }
	cases := []struct {
		name    string
		runtime kubeClusterRuntime
		want    []wantCert
	}{
		{name: "CA+클라이언트 2장", runtime: kubeClusterRuntime{CertificateAuthority: caPEM, ClientCertificateData: clientPEM}, want: []wantCert{{"CA Certificate", "certificate-authority", "char-ca"}, {"Client Certificate", "client-certificate", "char-client"}}},
		{name: "CA만", runtime: kubeClusterRuntime{CertificateAuthority: caPEM}, want: []wantCert{{"CA Certificate", "certificate-authority", "char-ca"}}},
		{name: "빈 런타임 → 0장", runtime: kubeClusterRuntime{}, want: []wantCert{}},
	}
	for _, tc := range cases {
		certificates := v2BuildOverviewCertificates(tc.runtime) // v2 oracle
		if len(certificates) != len(tc.want) {
			t.Fatalf("%s: count = %d, want %d", tc.name, len(certificates), len(tc.want))
		}
		for i, cert := range certificates {
			if cert.Name != tc.want[i].name || cert.Type != tc.want[i].typ || cert.Subject != tc.want[i].subject {
				t.Errorf("%s: [%d] = %s/%s/%s, want %s/%s/%s", tc.name, i,
					cert.Name, cert.Type, cert.Subject, tc.want[i].name, tc.want[i].typ, tc.want[i].subject)
			}
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// DaysRemaining = int(time.Until(NotAfter).Hours()/24)이므로 +365일 인증서는 364가 현재 계약.
func TestCharParseOverviewCertificate(t *testing.T) {
	now := time.Now()
	notAfter := now.Add(365 * 24 * time.Hour)
	cases := []struct {
		name         string
		encoded      string
		wantOK       bool
		wantSubject  string
		wantNotAfter string
		wantDays     int
	}{
		{name: "유효 인증서", encoded: charSelfSignedCertPEM("char-ca.example", now.Add(-time.Hour), notAfter), wantOK: true, wantSubject: "char-ca.example", wantNotAfter: notAfter.Local().Format("2006-01-02 15:04:05"), wantDays: 364},
		{name: "빈 문자열", encoded: "", wantOK: false},
		{name: "공백만", encoded: "   ", wantOK: false},
		{name: "base64 깨짐", encoded: "!!!not-base64!!!", wantOK: false},
		{name: "PEM 아님(base64만)", encoded: "aGVsbG8=", wantOK: false},
	}
	for _, tc := range cases {
		certificate, ok := v2ParseOverviewCertificate("CA Certificate", "certificate-authority", tc.encoded) // v2 oracle
		if ok != tc.wantOK {
			t.Errorf("%s: ok = %v, want %v", tc.name, ok, tc.wantOK)
			continue
		}
		if !tc.wantOK {
			continue
		}
		if certificate.Subject != tc.wantSubject || certificate.NotAfter != tc.wantNotAfter || certificate.DaysRemaining != tc.wantDays {
			t.Errorf("%s: = %s/%s/%d, want %s/%s/%d", tc.name,
				certificate.Subject, certificate.NotAfter, certificate.DaysRemaining, tc.wantSubject, tc.wantNotAfter, tc.wantDays)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
func TestCharCertificateCommonName(t *testing.T) {
	cases := []struct {
		name       string
		commonName string
		fallback   string
		want       string
	}{
		{name: "공백 트림", commonName: "  api.k8s.local  ", fallback: "fallback", want: "api.k8s.local"},
		{name: "빈 CN → fallback", commonName: "", fallback: "fallback", want: "fallback"},
		{name: "공백 CN → fallback", commonName: "   ", fallback: "fallback", want: "fallback"},
		{name: "빈 fallback → -", commonName: "", fallback: "", want: "-"},
	}
	for _, tc := range cases {
		if got := kubernetes.CertificateCommonName(tc.commonName, tc.fallback); got != tc.want { // v2 oracle
			t.Errorf("%s: = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// 30일 경계(<= warning)와 만료(<= 0) 경계가 현재 계약.
func TestCharK8sCertificateStatus(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name           string
		notAfter       time.Time
		wantStatus     string
		wantStatusText string
	}{
		{name: "과거 → expired", notAfter: now.Add(-24 * time.Hour), wantStatus: "expired", wantStatusText: "Expired"},
		{name: "현재(경계 0) → expired", notAfter: now, wantStatus: "expired", wantStatusText: "Expired"},
		{name: "29일 → warning", notAfter: now.Add(29 * 24 * time.Hour), wantStatus: "warning", wantStatusText: "Expiring Soon"},
		{name: "30일 경계 → warning", notAfter: now.Add(30 * 24 * time.Hour), wantStatus: "warning", wantStatusText: "Expiring Soon"},
		{name: "31일 → valid", notAfter: now.Add(31 * 24 * time.Hour), wantStatus: "valid", wantStatusText: "Valid"},
	}
	for _, tc := range cases {
		status, statusText := kubernetes.CertificateStatus(tc.notAfter) // v2 oracle
		if status != tc.wantStatus || statusText != tc.wantStatusText {
			t.Errorf("%s: = (%q, %q), want (%q, %q)", tc.name, status, statusText, tc.wantStatus, tc.wantStatusText)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// 이름 정렬, 역할 없으면 "worker", InternalIP 없으면 "-", 메모리는 MB(10^6) 반올림이 현재 계약.
func TestCharBuildNodeItems(t *testing.T) {
	nodeB := charNodeReady("b-node", "10.0.0.1", "True", "v1.29.1", "Ubuntu 22.04", "4", "8Gi", "110")
	nodeB.Metadata.Labels = map[string]string{"node-role.kubernetes.io/control-plane": ""}
	pods := []kubePod{charPod("p1", "default"), charPod("p2", "default"), charPod("p3", "default")}
	pods[0].Spec.NodeName = "b-node"
	pods[1].Spec.NodeName = "b-node"
	items := buildNodeItems([]kubeNode{charNodeReady("c-node", "", "False", "", "", "", "", ""), nodeB, charNode("a-node")}, pods) // legacy 1행
	want := []model.K8sNodeItem{
		{Name: "a-node", Role: "worker", Status: "Unknown", Version: "-", InternalIP: "-", OS: "-", CPU: "-", Memory: "-", Pods: "0/-"},
		{Name: "b-node", Role: "control-plane", Status: "Ready", Version: "v1.29.1", InternalIP: "10.0.0.1", OS: "Ubuntu 22.04", CPU: "4", Memory: "8590 MB", Pods: "2/110"},
		{Name: "c-node", Role: "worker", Status: "NotReady", Version: "-", InternalIP: "-", OS: "-", CPU: "-", Memory: "-", Pods: "0/-"},
	}
	if !reflect.DeepEqual(items, want) {
		t.Errorf("items = %+v, want %+v", items, want)
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// 카운트 맵에 없는 네임스페이스는 제로값 삼중(pods·services·workloads)이 현재 계약.
func TestCharBuildNamespaceCounts(t *testing.T) {
	data := k8sFetchedData{
		Pods:        []kubePod{charPod("p1", "team-a"), charPod("p2", "team-a"), charPod("p3", "team-b")},
		Services:    []kubeService{{Metadata: charMeta("svc", "team-a")}},
		Deployments: []kubeDeployment{charDeployment("d", "team-a")},
		CronJobs:    []kubeCronJob{charCronJob("c", "team-b")},
	}
	counts := v2BuildNamespaceCounts(data) // v2 oracle
	want := map[string][3]int{"team-a": {2, 1, 1}, "team-b": {1, 0, 1}, "없는-네임스페이스": {0, 0, 0}}
	for namespace, triple := range want {
		stat := counts[namespace]
		if got := [3]int{stat.pods, stat.services, stat.workloads}; !reflect.DeepEqual(got, triple) {
			t.Errorf("%s: = %v, want %v", namespace, got, triple)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// 이름 정렬, 상태 폴백 "-", 생성시각 "2006-01-02 15:04" 포맷이 현재 계약.
func TestCharBuildNamespaceItems(t *testing.T) {
	type nsCount = struct {
		pods      int
		services  int
		workloads int
	}
	var nsActive kubeNamespace
	nsActive.Metadata = charMeta("team-b", "")
	nsActive.Status.Phase = "Active"
	nsPlain := charNamespace("team-a")
	cases := []struct {
		name       string
		namespaces []kubeNamespace
		counts     map[string]nsCount
		want       []model.K8sNamespaceItem
	}{
		{
			name:       "정렬·카운트·상태",
			namespaces: []kubeNamespace{nsActive, nsPlain},
			counts:     map[string]nsCount{"team-b": {2, 0, 3}},
			want: []model.K8sNamespaceItem{
				{Name: "team-a", Status: "-", CreatedAt: "2026-01-01 00:00"},
				{Name: "team-b", Status: "Active", Pods: 2, Services: 0, Workloads: 3, CreatedAt: "2026-01-01 00:00"}}},
		{name: "counts에 없으면 전부 0", namespaces: []kubeNamespace{nsPlain}, counts: map[string]nsCount{}, want: []model.K8sNamespaceItem{{Name: "team-a", Status: "-", CreatedAt: "2026-01-01 00:00"}}},
	}
	for _, tc := range cases {
		if items := buildNamespaceItems(tc.namespaces, tc.counts); !reflect.DeepEqual(items, tc.want) { // legacy 1행
			t.Errorf("%s: items = %+v, want %+v", tc.name, items, tc.want)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
func TestCharPodWorkloadKey(t *testing.T) {
	cases := []struct {
		name      string
		namespace string
		podName   string
		want      string
	}{
		{name: "기본", namespace: "default", podName: "web-1", want: "default/web-1"},
		{name: "빈 값", namespace: "", podName: "", want: "/"},
	}
	for _, tc := range cases {
		if got := podWorkloadKey(tc.namespace, tc.podName); got != tc.want { // legacy 1행
			t.Errorf("%s: = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// 해석 우선순위(오너 체인 → 셀렉터 → 이름 접두사 최장 매칭)와 각 단계 결과가 현재 계약.
func TestCharBuildPodItemsWithWorkloads(t *testing.T) {
	rs := charReplicaSetOwnedByDeployment("web-rs", "web")
	job := charJobOwnedByCronJob("nightly-1", "nightly")
	cases := []struct {
		name     string
		data     k8sFetchedData
		pod      string
		wantName string
		wantType string
	}{
		{name: "ReplicaSet → Deployment 오너 체인", data: k8sFetchedData{Pods: []kubePod{charPodWithOwner("web-abc", "default", "ReplicaSet", "web-rs")}, ReplicaSets: []kubeReplicaSet{rs}}, pod: "web-abc", wantName: "web", wantType: "Deployment"},
		{name: "Job 오너 직접(Job 미등록 → 오너 이름)", data: k8sFetchedData{Pods: []kubePod{charPodWithOwner("run-1", "default", "Job", "nightly")}}, pod: "run-1", wantName: "nightly", wantType: "Job"},
		{name: "Job → CronJob 오너 체인", data: k8sFetchedData{Pods: []kubePod{charPodWithOwner("j-1", "default", "Job", "nightly-1")}, Jobs: []kubeJob{job}}, pod: "j-1", wantName: "nightly", wantType: "CronJob"},
		{name: "StatefulSet 오너 직접", data: k8sFetchedData{Pods: []kubePod{charPodWithOwner("web-0", "default", "StatefulSet", "web")}}, pod: "web-0", wantName: "web", wantType: "StatefulSet"},
		{name: "오너 없으면 셀렉터 매칭", data: k8sFetchedData{Pods: []kubePod{charPodWithLabels("web-1", "default", map[string]string{"app": "web"})}, Deployments: []kubeDeployment{charDeployWithSelector("web", map[string]string{"app": "web"})}}, pod: "web-1", wantName: "web", wantType: "Deployment"},
		{name: "근거 없으면 이름 접두사 최장 매칭", data: k8sFetchedData{Pods: []kubePod{charPod("webapp-9", "default")}, Deployments: []kubeDeployment{charDeployWithSelector("web", nil), charDeployWithSelector("webapp", nil)}}, pod: "webapp-9", wantName: "webapp", wantType: "Deployment"},
		{name: "아무 근거 없음 → 빈 워크로드", data: k8sFetchedData{Pods: []kubePod{charPod("orphan", "default")}}, pod: "orphan", wantName: "", wantType: ""},
	}
	for _, tc := range cases {
		items := v2BuildPodItemsWithWorkloads(tc.data) // v2 oracle
		for _, item := range items {
			if item.Name != tc.pod {
				continue
			}
			if item.WorkloadName != tc.wantName || item.WorkloadType != tc.wantType {
				t.Errorf("%s: = %s/%s, want %s/%s", tc.name, item.WorkloadName, item.WorkloadType, tc.wantName, tc.wantType)
			}
			return
		}
		t.Errorf("%s: pod %q 없음", tc.name, tc.pod)
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// 오너 폴백은 STS/DS/Job만 하고(ReplicaSet은 안 함), 빈 값은 "-" 폴백이 현재 계약.
func TestCharBuildPodItems(t *testing.T) {
	podFull := charRunningPod("p2", "team-a")
	podNoAge := charPod("p1", "team-a")
	podNoAge.Metadata.CreationTimestamp = ""
	cases := []struct {
		name string
		pods []kubePod
		want []model.K8sPodItem
	}{
		{
			name: "재시작 합산·폴백·오너 폴백",
			pods: []kubePod{podFull, podNoAge, charPodWithOwner("p3", "team-b", "DaemonSet", "agent"), charPodWithOwner("p4", "team-b", "ReplicaSet", "web-rs")},
			want: []model.K8sPodItem{
				{Name: "p1", Namespace: "team-a", Status: "-", Node: "-", NodeIP: "-", Age: "-", IP: "-"},
				{Name: "p2", Namespace: "team-a", Status: "Running", Node: "node-1", NodeIP: "10.0.0.1", Restarts: 5, Age: "2026-01-01", IP: "10.1.0.5"},
				{Name: "p3", Namespace: "team-b", WorkloadName: "agent", WorkloadType: "DaemonSet", Status: "-", Node: "-", NodeIP: "-", Age: "2026-01-01", IP: "-"},
				{Name: "p4", Namespace: "team-b", Status: "-", Node: "-", NodeIP: "-", Age: "2026-01-01", IP: "-"}}},
		{name: "빈 입력 → 빈 슬라이스", pods: nil, want: []model.K8sPodItem{}},
	}
	for _, tc := range cases {
		items := buildPodItems(tc.pods) // legacy 1행
		if items == nil {
			t.Fatalf("%s: items = nil, want 빈 슬라이스", tc.name)
		}
		if !reflect.DeepEqual(items, tc.want) {
			t.Errorf("%s: items = %+v, want %+v", tc.name, items, tc.want)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// refs가 우선하고, refs에 없는 pod는 STS/DS/Job 오너 폴백, 네임스페이스→이름 정렬이 현재 계약.
func TestCharBuildPodItemsWithRefs(t *testing.T) {
	pods := []kubePod{charPod("p-b", "default"), charPod("p-a", "default"), charPodWithOwner("p-z", "other", "Job", "x")}
	cases := []struct {
		name  string
		refs  map[string]podWorkloadRef
		want  map[string]podWorkloadRef
		order []string
	}{
		{
			name:  "refs 우선 + 오너 폴백 + 정렬",
			refs:  map[string]podWorkloadRef{podWorkloadKey("default", "p-b"): {Name: "web", Type: "Deployment"}},
			want:  map[string]podWorkloadRef{"p-a": {}, "p-b": {Name: "web", Type: "Deployment"}, "p-z": {Name: "x", Type: "Job"}},
			order: []string{"p-a", "p-b", "p-z"}},
		{name: "nil refs → 오너 폴백만", refs: nil, want: map[string]podWorkloadRef{"p-a": {}, "p-b": {}, "p-z": {Name: "x", Type: "Job"}}, order: []string{"p-a", "p-b", "p-z"}},
	}
	for _, tc := range cases {
		items := buildPodItemsWithRefs(pods, tc.refs) // legacy 1행
		if names := charPodItemNames(items); !reflect.DeepEqual(names, tc.order) {
			t.Errorf("%s: order = %v, want %v", tc.name, names, tc.order)
		}
		for _, item := range items {
			ref := tc.want[item.Name]
			if item.WorkloadName != ref.Name || item.WorkloadType != ref.Type {
				t.Errorf("%s: %s = %s/%s, want %s/%s", tc.name, item.Name, item.WorkloadName, item.WorkloadType, ref.Name, ref.Type)
			}
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// 종류별 Ready 산식(복제본 nil → 0, Job 합산, CronJob Scheduled)과 입력 순서 보존이 현재 계약.
func TestCharBuildWorkloadItems(t *testing.T) {
	// charAllKindWorkloads: nil 복제본 STS, Job 합산, CronJob Scheduled가 한 번에 고정된다.
	data := charAllKindWorkloads()
	items := buildWorkloadItems(data) // legacy 1행
	want := []model.K8sWorkloadItem{
		{Name: "web", Type: "Deployment", Namespace: "app", Ready: "2/3", Updated: 1, Available: 2, Age: "2026-01-01", Requests: "-", Limits: "-"},
		{Name: "db", Type: "StatefulSet", Namespace: "app", Ready: "0/0", Age: "2026-01-01", Requests: "-", Limits: "-"},
		{Name: "agent", Type: "DaemonSet", Namespace: "app", Ready: "1/3", Updated: 1, Available: 1, Age: "2026-01-01", Requests: "-", Limits: "-"},
		{Name: "nightly", Type: "Job", Namespace: "app", Ready: "2/4", Updated: 1, Available: 2, Age: "2026-01-01", Requests: "-", Limits: "-"},
		{Name: "cleanup", Type: "CronJob", Namespace: "app", Ready: "Scheduled", Age: "2026-01-01", Requests: "-", Limits: "-"}}
	if !reflect.DeepEqual(items, want) {
		t.Errorf("items = %+v, want %+v", items, want)
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// 키는 "네임스페이스/이름", 서브셋 주소 합산, 서브셋 없으면 0이 현재 계약.
func TestCharBuildEndpointCounts(t *testing.T) {
	cases := []struct {
		name      string
		endpoints []kubeEndpoints
		want      map[string]int
	}{
		{name: "서브셋 합산·서브셋 없음", endpoints: []kubeEndpoints{charEndpoint("ep-a", "default", 2, 1), charEndpoint("ep-b", "default")},
			want: map[string]int{"default/ep-a": 3, "default/ep-b": 0}},
		{name: "빈 입력", endpoints: nil, want: map[string]int{}},
	}
	for _, tc := range cases {
		if counts := v2BuildEndpointCounts(tc.endpoints); !reflect.DeepEqual(counts, tc.want) { // v2 oracle
			t.Errorf("%s: counts = %v, want %v", tc.name, counts, tc.want)
		}
	}
}
