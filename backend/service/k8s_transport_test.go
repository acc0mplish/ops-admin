// k8s_transport_test.go — Phase Z2 characterization (k8s_transport 시맨 순수 함수 19개).
// 목적은 "옳음"이 아니라 "불변": 현재 동작을 그대로 기록해 B~D2 파일 분해의 유일한
// 검출기이자 P의 V2 동치 오라클이 되게 한다 (계획 §J0·§12 #16, R18 — legacy 호출 1행).
package service

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"
)

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// context 우선·공백 비어 있으면 첫 context, 각 단계 센티널 에러 문언이 현재 계약.
func TestCharParseKubeConfig(t *testing.T) {
	now := time.Now()
	caData := charSelfSignedCertPEM("char-ca", now.Add(-time.Hour), now.Add(365*24*time.Hour))
	insecureYAML := fmt.Sprintf(`current-context: ctx1
clusters:
  - name: kube1
    cluster:
      server: " https://api:6443 "
      insecure-skip-tls-verify: true
      certificate-authority-data: %s
contexts:
  - name: ctx1
    context: {cluster: kube1, user: admin}
users:
  - name: admin
    user: {token: tok-1}`, caData)
	switchYAML := `current-context: ctx2
clusters:
  - name: kube1
    cluster: {server: "https://one:6443"}
  - name: kube2
    cluster: {server: "https://two:6443"}
contexts:
  - name: ctx1
    context: {cluster: kube1, user: u1}
  - name: ctx2
    context: {cluster: kube2, user: u2}
users:
  - name: u1
    user: {username: a, password: b}
  - name: u2
    user: {token: t2, client-certificate-data: CERT, client-key-data: KEY}`
	cases := []struct {
		name    string
		content string
		want    kubeClusterRuntime
		wantErr string
	}{
		{name: "최소형", content: `current-context: ctx1
clusters:
  - name: kube1
    cluster: {server: "https://api:6443"}
contexts:
  - name: ctx1
    context: {cluster: kube1, user: admin}
users:
  - name: admin
    user: {token: tok-1}`,
			want: kubeClusterRuntime{Server: "https://api:6443", Token: "tok-1"}},
		{name: "context 전환", content: switchYAML,
			want: kubeClusterRuntime{Server: "https://two:6443", Token: "t2", ClientCertificateData: "CERT", ClientKeyData: "KEY"}},
		{name: "basic 자격 user 선택", content: `current-context: ctx1
clusters:
  - name: kube1
    cluster: {server: "https://api:6443"}
contexts:
  - name: ctx1
    context: {cluster: kube1, user: basic-user}
users:
  - name: basic-user
    user: {username: ops, password: secret}`,
			want: kubeClusterRuntime{Server: "https://api:6443", Username: "ops", Password: "secret"}},
		{name: "current-context 빈값 → 첫 context", content: `clusters:
  - name: kube1
    cluster: {server: "https://one:6443"}
contexts:
  - name: ctx1
    context: {cluster: kube1, user: u1}
users:
  - name: u1
    user: {token: t1}`,
			want: kubeClusterRuntime{Server: "https://one:6443", Token: "t1"}},
		{name: "insecure·CA 데이터·server trim", content: insecureYAML,
			want: kubeClusterRuntime{Server: "https://api:6443", InsecureSkipTLSVerify: true, CertificateAuthority: caData, Token: "tok-1"}},
		{name: "빈 내용 → missing context", content: "   ", wantErr: "missing context"},
		{name: "context의 cluster 빈값 → cluster not found", content: `current-context: ctx1
contexts:
  - name: ctx1
    context: {user: admin}`, wantErr: "cluster not found"},
		{name: "미등록 cluster 참조는 server not found(현재 동작)", content: `current-context: ctx1
contexts:
  - name: ctx1
    context: {cluster: ghost}`, wantErr: "server not found"},
		{name: "server 미발견", content: `current-context: ctx1
clusters:
  - name: kube1
contexts:
  - name: ctx1
    context: {cluster: kube1}`, wantErr: "server not found"},
		{name: "깨진 YAML", content: "clusters: [unclosed", wantErr: "yaml:"},
	}
	for _, tc := range cases {
		runtime, err := parseKubeConfig(tc.content) // legacy 1행
		if tc.wantErr != "" {
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("%s: err = %v, want contains %q", tc.name, err, tc.wantErr)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%s: unexpected err: %v", tc.name, err)
		}
		if !reflect.DeepEqual(runtime, tc.want) {
			t.Errorf("%s: runtime = %+v, want %+v", tc.name, runtime, tc.want)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// TLS 1.2 최소·타임아웃 8s, CA·클라이언트 인증서 구성, 인증서만 있고 키 없으면 조용히 생략이 현재 계약.
func TestCharNewK8sHTTPClient(t *testing.T) {
	now := time.Now()
	certPEM, keyPEM := charSelfSignedCertKeyPairPEM("char-client", now.Add(-time.Hour), now.Add(time.Hour))
	otherCertPEM, otherKeyPEM := charSelfSignedCertKeyPairPEM("char-other", now.Add(-time.Hour), now.Add(time.Hour))
	_ = otherCertPEM
	encode := func(pemText string) string { return base64.StdEncoding.EncodeToString([]byte(pemText)) }
	cases := []struct {
		name      string
		runtime   kubeClusterRuntime
		wantErr   string
		wantCheck func(t *testing.T, client *http.Client)
	}{
		{name: "빈 런타임 → 기본 구성", wantCheck: func(t *testing.T, client *http.Client) {
			if client.Timeout != 8*time.Second {
				t.Errorf("timeout = %v, want 8s", client.Timeout)
			}
			tlsConfig := client.Transport.(*http.Transport).TLSClientConfig
			if tlsConfig.MinVersion != tls.VersionTLS12 || tlsConfig.InsecureSkipVerify || tlsConfig.RootCAs != nil || len(tlsConfig.Certificates) != 0 {
				t.Errorf("tls = %+v", tlsConfig)
			}
		}},
		{name: "insecure 전달", runtime: kubeClusterRuntime{InsecureSkipTLSVerify: true}, wantCheck: func(t *testing.T, client *http.Client) {
			if !client.Transport.(*http.Transport).TLSClientConfig.InsecureSkipVerify {
				t.Errorf("InsecureSkipVerify = false, want true")
			}
		}},
		{name: "CA 등록", runtime: kubeClusterRuntime{CertificateAuthority: encode(certPEM)}, wantCheck: func(t *testing.T, client *http.Client) {
			if client.Transport.(*http.Transport).TLSClientConfig.RootCAs == nil {
				t.Errorf("RootCAs = nil, want 등록됨")
			}
		}},
		{name: "CA base64 깨짐", runtime: kubeClusterRuntime{CertificateAuthority: "!!!"}, wantErr: "illegal base64"},
		{name: "CA가 PEM 아님", runtime: kubeClusterRuntime{CertificateAuthority: encode("garbage")}, wantErr: "invalid certificate authority"},
		{name: "클라이언트 인증서+키", runtime: kubeClusterRuntime{ClientCertificateData: encode(certPEM), ClientKeyData: encode(keyPEM)}, wantCheck: func(t *testing.T, client *http.Client) {
			if len(client.Transport.(*http.Transport).TLSClientConfig.Certificates) != 1 {
				t.Errorf("certificates = %d, want 1", len(client.Transport.(*http.Transport).TLSClientConfig.Certificates))
			}
		}},
		{name: "인증서만 있으면 생략(무오류)", runtime: kubeClusterRuntime{ClientCertificateData: encode(certPEM)}, wantCheck: func(t *testing.T, client *http.Client) {
			if len(client.Transport.(*http.Transport).TLSClientConfig.Certificates) != 0 {
				t.Errorf("certificates = %d, want 0(생략)", len(client.Transport.(*http.Transport).TLSClientConfig.Certificates))
			}
		}},
		{name: "클라이언트 인증서 base64 깨짐", runtime: kubeClusterRuntime{ClientCertificateData: "!!", ClientKeyData: "!!"}, wantErr: "illegal base64"},
		{name: "cert/key 불일치", runtime: kubeClusterRuntime{ClientCertificateData: encode(certPEM), ClientKeyData: encode(otherKeyPEM)}, wantErr: "private key does not match public key"},
	}
	for _, tc := range cases {
		client, err := newK8sHTTPClient(tc.runtime) // legacy 1행
		if tc.wantErr != "" {
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("%s: err = %v, want contains %q", tc.name, err, tc.wantErr)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%s: unexpected err: %v", tc.name, err)
		}
		tc.wantCheck(t, client)
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// dialer 주입은 Transport.DialContext로 직결되고 nil이면 기본 다이얼러가 현재 계약.
func TestCharNewK8sHTTPClientWithDial(t *testing.T) {
	injected := func(ctx context.Context, network, address string) (net.Conn, error) { return nil, nil }
	cases := []struct {
		name         string
		dial         func(context.Context, string, string) (net.Conn, error)
		wantInjected bool
	}{
		{name: "dial 주입 → Transport 연결", dial: injected, wantInjected: true},
		{name: "nil dial → 기본 다이얼러", dial: nil},
	}
	for _, tc := range cases {
		client, err := newK8sHTTPClientWithDial(kubeClusterRuntime{}, tc.dial) // legacy 1행
		if err != nil {
			t.Fatalf("%s: unexpected err: %v", tc.name, err)
		}
		got := client.Transport.(*http.Transport).DialContext != nil
		if got != tc.wantInjected {
			t.Errorf("%s: DialContext 주입 = %v, want %v", tc.name, got, tc.wantInjected)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// gitVersion 빈값 → "empty version" 에러가 현재 계약.
func TestCharFetchK8sVersion(t *testing.T) {
	cases := []struct {
		name    string
		routes  map[string]charStub
		want    string
		wantErr string
	}{
		{name: "성공", routes: map[string]charStub{"/version": {status: 200, body: `{"gitVersion":"v1.29.0"}`}}, want: "v1.29.0"},
		{name: "빈 버전 → 에러", routes: map[string]charStub{"/version": {status: 200, body: `{"gitVersion":" "}`}}, wantErr: "empty version"},
		{name: "500", routes: map[string]charStub{"/version": {status: 500, body: "boom"}}, wantErr: "unexpected status: 500"},
	}
	for _, tc := range cases {
		client, runtime := charStubKube(t, tc.routes)
		version, err := fetchK8sVersion(client, runtime) // legacy 1행
		if charWantErr(t, tc.name, err, tc.wantErr) {
			continue
		}
		if version != tc.want {
			t.Errorf("%s: = %q, want %q", tc.name, version, tc.want)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
func TestCharFetchK8sNodeCount(t *testing.T) {
	cases := []struct {
		name    string
		routes  map[string]charStub
		want    int
		wantErr string
	}{
		{name: "성공", routes: map[string]charStub{"/api/v1/nodes": {status: 200, body: charJSON(kubeNodeListResponse{Items: make([]kubeNode, 3)})}}, want: 3},
		{name: "404", routes: map[string]charStub{}, wantErr: "unexpected status: 404"},
	}
	for _, tc := range cases {
		client, runtime := charStubKube(t, tc.routes)
		count, err := fetchK8sNodeCount(client, runtime) // legacy 1행
		if charWantErr(t, tc.name, err, tc.wantErr) {
			continue
		}
		if count != tc.want {
			t.Errorf("%s: = %d, want %d", tc.name, count, tc.want)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// 서버 간 hosts 평탄화 + uniqueNonEmptyStrings 적용이 현재 계약.
func TestCharFlattenGatewayHosts(t *testing.T) {
	gateway := charIstioGateway("gw", "istio-system")
	if err := json.Unmarshal([]byte(`{"servers":[{"hosts":["a.example","b.example"]},{"hosts":["b.example","","-"]}]}`), &gateway.Spec); err != nil {
		t.Fatal(err)
	}
	if got := flattenGatewayHosts(gateway); !reflect.DeepEqual(got, []string{"a.example", "b.example"}) { // legacy 1행
		t.Errorf("hosts = %v, want [a.example b.example]", got)
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// 포트 0·빈 프로토콜은 "-"가 되지만 "-"는 결과에서 제외되는 것이 현재 계약.
func TestCharFlattenGatewayPorts(t *testing.T) {
	gateway := charIstioGateway("gw", "istio-system")
	if err := json.Unmarshal([]byte(`{"servers":[{"port":{"number":80,"protocol":"HTTP"}},{"port":{"number":443,"protocol":"HTTPS"}},{"port":{}}]}`), &gateway.Spec); err != nil {
		t.Fatal(err)
	}
	if got := flattenGatewayPorts(gateway); !reflect.DeepEqual(got, []string{"80/HTTP", "443/HTTPS"}) { // legacy 1행
		t.Errorf("ports = %v, want [80/HTTP 443/HTTPS]", got)
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// GET·Accept 헤더·토큰 Bearer 헤더·바디 없으면 Content-Type 미설정이 현재 계약.
func TestCharK8sGetJSON(t *testing.T) {
	cases := []struct {
		name          string
		token         string
		status        int
		responseBody  string
		wantTarget    map[string]any
		wantErr       string
		wantAuth      string
		wantEmptyBody bool
	}{
		{name: "성공·헤더 고정", status: 200, responseBody: `{"ok":true}`, wantTarget: map[string]any{"ok": true}, wantEmptyBody: true},
		{name: "토큰 → Bearer", token: "tok-1", status: 200, responseBody: `{}`, wantTarget: map[string]any{}, wantAuth: "Bearer tok-1", wantEmptyBody: true},
		{name: "500", status: 500, responseBody: "boom", wantErr: "unexpected status: 500"},
	}
	for _, tc := range cases {
		var observed charObserved
		client, runtime := charCaptureStub(t, tc.status, tc.responseBody, &observed)
		runtime.Token = tc.token
		var target map[string]any
		err := k8sGetJSON(client, runtime, "/probe", &target) // legacy 1행
		if charWantErr(t, tc.name, err, tc.wantErr) {
			continue
		}
		if observed.Method != http.MethodGet || observed.Path != "/probe" || observed.Accept != "application/json" {
			t.Errorf("%s: 요청 = %s %s accept=%q", tc.name, observed.Method, observed.Path, observed.Accept)
		}
		if observed.Authorization != tc.wantAuth {
			t.Errorf("%s: authorization = %q, want %q", tc.name, observed.Authorization, tc.wantAuth)
		}
		if tc.wantEmptyBody && observed.ContentType != "" {
			t.Errorf("%s: content-type = %q, want 미설정", tc.name, observed.ContentType)
		}
		if !reflect.DeepEqual(target, tc.wantTarget) {
			t.Errorf("%s: target = %v, want %v", tc.name, target, tc.wantTarget)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// 404만 건너뛰고 다음 경로, 5xx는 즉시 중단이 현재 계약.
func TestCharK8sGetJSONAnyPath(t *testing.T) {
	paths := []string{"/apis/x/v1/r", "/apis/x/v1beta1/r"}
	cases := []struct {
		name    string
		routes  map[string]charStub
		wantErr string
	}{
		{name: "404 폴백 후 성공", routes: map[string]charStub{
			"/apis/x/v1/r":      {status: 404},
			"/apis/x/v1beta1/r": {status: 200, body: `{"ok":true}`}}},
		{name: "5xx는 즉시 중단", routes: map[string]charStub{"/apis/x/v1/r": {status: 500, body: "boom"}}, wantErr: "unexpected status: 500"},
		{name: "전부 404 → 에러", routes: map[string]charStub{}, wantErr: "unexpected status: 404"},
	}
	for _, tc := range cases {
		client, runtime := charStubKube(t, tc.routes)
		var target map[string]any
		err := k8sGetJSONAnyPath(client, runtime, paths, &target) // legacy 1행
		if charWantErr(t, tc.name, err, tc.wantErr) {
			continue
		}
		if !reflect.DeepEqual(target, map[string]any{"ok": true}) {
			t.Errorf("%s: target = %v", tc.name, target)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// v1 → v1beta1 순서 폴백과 istio 그룹 경로 형상이 현재 계약.
func TestCharK8sGetIstioJSON(t *testing.T) {
	v1Path := "/apis/networking.istio.io/v1/namespaces/istio-system/gateways/gw"
	betaPath := "/apis/networking.istio.io/v1beta1/namespaces/istio-system/gateways/gw"
	cases := []struct {
		name    string
		routes  map[string]charStub
		wantErr string
	}{
		{name: "v1 성공", routes: map[string]charStub{v1Path: {status: 200, body: `{"ok":true}`}}},
		{name: "v1 404 → v1beta1 폴백", routes: map[string]charStub{v1Path: {status: 404}, betaPath: {status: 200, body: `{"ok":true}`}}},
		{name: "둘 다 404", routes: map[string]charStub{}, wantErr: "unexpected status: 404"},
	}
	for _, tc := range cases {
		client, runtime := charStubKube(t, tc.routes)
		var target map[string]any
		err := k8sGetIstioJSON(client, runtime, "gateways", "istio-system", "gw", &target) // legacy 1행
		if charWantErr(t, tc.name, err, tc.wantErr) {
			continue
		}
		if !reflect.DeepEqual(target, map[string]any{"ok": true}) {
			t.Errorf("%s: target = %v", tc.name, target)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// gateway.networking.k8s.io 그룹 경로 형상이 현재 계약.
func TestCharK8sGetGatewayAPIJSON(t *testing.T) {
	v1Path := "/apis/gateway.networking.k8s.io/v1/gateways"
	cases := []struct {
		name    string
		routes  map[string]charStub
		wantErr string
	}{
		{name: "v1 성공(클러스터 스코프)", routes: map[string]charStub{v1Path: {status: 200, body: `{"ok":true}`}}},
		{name: "둘 다 404", routes: map[string]charStub{}, wantErr: "unexpected status: 404"},
	}
	for _, tc := range cases {
		client, runtime := charStubKube(t, tc.routes)
		var target map[string]any
		err := k8sGetGatewayAPIJSON(client, runtime, "gateways", "", "", &target) // legacy 1행
		if charWantErr(t, tc.name, err, tc.wantErr) {
			continue
		}
		if !reflect.DeepEqual(target, map[string]any{"ok": true}) {
			t.Errorf("%s: target = %v", tc.name, target)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// 기본 버전 순서 [v1, v1beta1], ns·name 있으면 경계 삽입이 현재 계약.
func TestCharBuildIstioResourcePaths(t *testing.T) {
	cases := []struct {
		name      string
		resource  string
		namespace string
		itemName  string
		want      []string
	}{
		{name: "클러스터 스코프", resource: "gateways", want: []string{"/apis/networking.istio.io/v1/gateways", "/apis/networking.istio.io/v1beta1/gateways"}},
		{name: "네임스페이스·이름", resource: "virtualservices", namespace: "istio-system", itemName: "vs",
			want: []string{"/apis/networking.istio.io/v1/namespaces/istio-system/virtualservices/vs", "/apis/networking.istio.io/v1beta1/namespaces/istio-system/virtualservices/vs"}},
	}
	for _, tc := range cases {
		if got := buildIstioResourcePaths(tc.resource, tc.namespace, tc.itemName); !reflect.DeepEqual(got, tc.want) { // legacy 1행
			t.Errorf("%s: = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// "networking.istio.io/" 접두 제거 후 v1beta1이면 순서 전환, 그 외 무시가 현재 계약.
func TestCharBuildIstioResourcePathsWithPreferred(t *testing.T) {
	cases := []struct {
		name      string
		preferred string
		wantFirst string
	}{
		{name: "기본 순서", preferred: "", wantFirst: "/apis/networking.istio.io/v1/gateways"},
		{name: "그룹 접두 포함 전환", preferred: "networking.istio.io/v1beta1", wantFirst: "/apis/networking.istio.io/v1beta1/gateways"},
		{name: "공백 트림 전환", preferred: " v1beta1 ", wantFirst: "/apis/networking.istio.io/v1beta1/gateways"},
		{name: "다른 그룹 접두는 무시", preferred: "gateway.networking.k8s.io/v1beta1", wantFirst: "/apis/networking.istio.io/v1/gateways"},
	}
	for _, tc := range cases {
		got := buildIstioResourcePathsWithPreferred("gateways", "", "", tc.preferred) // legacy 1행
		if len(got) != 2 || got[0] != tc.wantFirst {
			t.Errorf("%s: = %v, want [0]=%q", tc.name, got, tc.wantFirst)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
func TestCharBuildGatewayAPIResourcePaths(t *testing.T) {
	want := []string{"/apis/gateway.networking.k8s.io/v1/namespaces/default/httproutes/r", "/apis/gateway.networking.k8s.io/v1beta1/namespaces/default/httproutes/r"}
	if got := buildGatewayAPIResourcePaths("httproutes", "default", "r"); !reflect.DeepEqual(got, want) { // legacy 1행
		t.Errorf("= %v, want %v", got, want)
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// "gateway.networking.k8s.io/" 접두만 인식하는 것이 현재 계약.
func TestCharBuildGatewayAPIResourcePathsWithPreferred(t *testing.T) {
	cases := []struct {
		name      string
		preferred string
		wantFirst string
	}{
		{name: "기본 순서", preferred: "", wantFirst: "/apis/gateway.networking.k8s.io/v1/gateways"},
		{name: "그룹 접두 포함 전환", preferred: "gateway.networking.k8s.io/v1beta1", wantFirst: "/apis/gateway.networking.k8s.io/v1beta1/gateways"},
		{name: "다른 그룹 접두는 무시", preferred: "networking.istio.io/v1beta1", wantFirst: "/apis/gateway.networking.k8s.io/v1/gateways"},
	}
	for _, tc := range cases {
		got := buildGatewayAPIResourcePathsWithPreferred("gateways", "", "", tc.preferred) // legacy 1행
		if len(got) != 2 || got[0] != tc.wantFirst {
			t.Errorf("%s: = %v, want [0]=%q", tc.name, got, tc.wantFirst)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// PATCH 메서드·body 직렬화·Content-Type 전달이 현재 계약.
func TestCharK8sPatchJSON(t *testing.T) {
	cases := []struct {
		name       string
		status     int
		body       any
		wantErr    string
		wantBody   string
		wantTarget map[string]any
	}{
		{name: "PATCH 원문 고정", status: 200, body: map[string]any{"spec": "next"},
			wantBody: `{"spec":"next"}`, wantTarget: map[string]any{"patched": true}},
		{name: "직렬화 불가 body", status: 200, body: map[string]any{"bad": make(chan int)}, wantErr: "unsupported type"},
		{name: "409", status: 409, body: map[string]any{"spec": "x"}, wantErr: "unexpected status: 409"},
	}
	for _, tc := range cases {
		var observed charObserved
		client, runtime := charCaptureStub(t, tc.status, `{"patched":true}`, &observed)
		var target map[string]any
		err := k8sPatchJSON(client, runtime, "/apis/x/v1/r", tc.body, "application/merge-patch+json", &target) // legacy 1행
		if charWantErr(t, tc.name, err, tc.wantErr) {
			continue
		}
		if observed.Method != http.MethodPatch || observed.ContentType != "application/merge-patch+json" || observed.Body != tc.wantBody {
			t.Errorf("%s: 요청 = %s ct=%q body=%q", tc.name, observed.Method, observed.ContentType, observed.Body)
		}
		if !reflect.DeepEqual(target, tc.wantTarget) {
			t.Errorf("%s: target = %v, want %v", tc.name, target, tc.wantTarget)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// 쿼리 인코딩과 무쿼리 경로가 현재 계약.
func TestCharK8sGetJSONWithQuery(t *testing.T) {
	var observed charObserved
	client, runtime := charCaptureStub(t, 200, `{}`, &observed)
	err := k8sGetJSONWithQuery(client, runtime, "/api/v1/pods", map[string]string{"fieldSelector": "spec.nodeName=n1", "labelSelector": "app=web"}, nil) // legacy 1행
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if observed.Path != "/api/v1/pods" || observed.Query.Get("fieldSelector") != "spec.nodeName=n1" || observed.Query.Get("labelSelector") != "app=web" {
		t.Errorf("요청 = %s %v", observed.Path, observed.Query)
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// 404 폴백 시 method·query·body가 동일 재시도되고 5xx는 즉시 중단이 현재 계약.
func TestCharK8sDoJSONAnyPath(t *testing.T) {
	cases := []struct {
		name    string
		routes  map[string]charStub
		wantErr string
	}{
		{name: "404 폴백 후 성공", routes: map[string]charStub{
			"/p1": {status: 404},
			"/p2": {status: 200, body: `{"ok":true}`}}},
		{name: "5xx 즉시 중단", routes: map[string]charStub{"/p1": {status: 500, body: "boom"}}, wantErr: "unexpected status: 500"},
		{name: "전부 404", routes: map[string]charStub{}, wantErr: "unexpected status: 404"},
	}
	for _, tc := range cases {
		client, runtime := charStubKube(t, tc.routes)
		var target map[string]any
		err := k8sDoJSONAnyPath(client, runtime, http.MethodGet, []string{"/p1", "/p2"}, map[string]string{"k": "v"}, []byte(`{"x":1}`), "application/json", &target) // legacy 1행
		if charWantErr(t, tc.name, err, tc.wantErr) {
			continue
		}
		if !reflect.DeepEqual(target, map[string]any{"ok": true}) {
			t.Errorf("%s: target = %v", tc.name, target)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// method·쿼리·바디·Content-Type·Bearer/Basic 인증·상태 에러 문언·server 후행 슬래시 trim이 현재 계약.
func TestCharK8sDoJSON(t *testing.T) {
	basicAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("u:p"))
	cases := []struct {
		name          string
		method        string
		path          string
		query         map[string]string
		body          []byte
		contentType   string
		token         string
		username      string
		password      string
		trailingSlash bool
		status        int
		responseBody  string
		withTarget    bool
		wantTarget    map[string]any
		wantErr       string
		wantAuth      string
	}{
		{name: "POST 전체 원문", method: http.MethodPost, path: "/apis/x", query: map[string]string{"a": "b"}, body: []byte(`{"x":1}`),
			contentType: "application/merge-patch+json", token: "t1", status: 200, responseBody: `{"patched":true}`, withTarget: true,
			wantTarget: map[string]any{"patched": true}, wantAuth: "Bearer t1"},
		{name: "Basic 인증", method: http.MethodGet, path: "/p", username: "u", password: "p", status: 200, responseBody: `{}`, wantAuth: basicAuth},
		{name: "404 + 바디", method: http.MethodGet, path: "/p", status: 404, responseBody: "not found\n", wantErr: "unexpected status: 404, not found"},
		{name: "500 바디 없음", method: http.MethodGet, path: "/p", status: 500, wantErr: "unexpected status: 500"},
		{name: "target nil은 디코드 생략", method: http.MethodGet, path: "/p", status: 200, responseBody: `{"any":1}`},
		{name: "server 후행 슬래시 trim", method: http.MethodGet, path: "/p", trailingSlash: true, status: 200, responseBody: `{}`, withTarget: true, wantTarget: map[string]any{}},
	}
	for _, tc := range cases {
		var observed charObserved
		client, runtime := charCaptureStub(t, tc.status, tc.responseBody, &observed)
		runtime.Token = tc.token
		runtime.Username = tc.username
		runtime.Password = tc.password
		if tc.trailingSlash {
			runtime.Server += "/"
		}
		var target map[string]any
		var dest any
		if tc.withTarget {
			dest = &target
		}
		err := k8sDoJSON(client, runtime, tc.method, tc.path, tc.query, tc.body, tc.contentType, dest) // legacy 1행
		if charWantErr(t, tc.name, err, tc.wantErr) {
			continue
		}
		if observed.Method != tc.method || observed.Path != tc.path {
			t.Errorf("%s: 요청 = %s %s", tc.name, observed.Method, observed.Path)
		}
		if observed.Authorization != tc.wantAuth {
			t.Errorf("%s: authorization = %q, want %q", tc.name, observed.Authorization, tc.wantAuth)
		}
		if tc.body != nil && (observed.Body != string(tc.body) || observed.ContentType != tc.contentType) {
			t.Errorf("%s: body = %q ct = %q", tc.name, observed.Body, observed.ContentType)
		}
		if tc.query != nil && observed.Query.Get("a") != "b" {
			t.Errorf("%s: query = %v", tc.name, observed.Query)
		}
		if tc.wantTarget != nil && !reflect.DeepEqual(target, tc.wantTarget) {
			t.Errorf("%s: target = %v, want %v", tc.name, target, tc.wantTarget)
		}
	}
}
