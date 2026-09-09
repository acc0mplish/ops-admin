// k8s_path_test.go — Phase Z3 characterization (k8s_path 시맨 순수 함수 25개).
// 목적은 "옳음"이 아니라 "불변": 현재 동작을 그대로 기록해 B~D2 파일 분해의 유일한
// 검출기이자 P의 V2 동치 오라클이 되게 한다 (계획 §J0·§12 #16, R18 — legacy 호출 1행).
package service

import (
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"ops-admin/backend/model"
)

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// 타입·ns·name trim+소문자화, 종류별 경로·에러 문언, workload 하위 스위치가 현재 계약.
func TestCharBuildK8sYAMLResourcePath(t *testing.T) {
	cases := []struct {
		name        string
		payload     model.K8sResourceYAMLPayload
		want        string
		wantErrText string
	}{
		{name: "namespace", payload: model.K8sResourceYAMLPayload{ResourceType: " Namespace ", Name: " prod "}, want: "/api/v1/namespaces/prod"},
		{name: "pod", payload: model.K8sResourceYAMLPayload{ResourceType: "pod", Namespace: "ns1", Name: "p1"}, want: "/api/v1/namespaces/ns1/pods/p1"},
		{name: "service", payload: model.K8sResourceYAMLPayload{ResourceType: "service", Namespace: "ns1", Name: "s1"}, want: "/api/v1/namespaces/ns1/services/s1"},
		{name: "ingress", payload: model.K8sResourceYAMLPayload{ResourceType: "ingress", Namespace: "ns1", Name: "i1"}, want: "/apis/networking.k8s.io/v1/namespaces/ns1/ingresses/i1"},
		{name: "pvc", payload: model.K8sResourceYAMLPayload{ResourceType: "pvc", Namespace: "ns1", Name: "c1"}, want: "/api/v1/namespaces/ns1/persistentvolumeclaims/c1"},
		{name: "pv", payload: model.K8sResourceYAMLPayload{ResourceType: "pv", Name: "v1"}, want: "/api/v1/persistentvolumes/v1"},
		{name: "workload deployment", payload: model.K8sResourceYAMLPayload{ResourceType: "workload", Namespace: "ns1", Name: "web", WorkloadType: "Deployment"}, want: "/apis/apps/v1/namespaces/ns1/deployments/web"},
		{name: "workload cronjob은 batch", payload: model.K8sResourceYAMLPayload{ResourceType: "workload", Namespace: "ns1", Name: "c", WorkloadType: "cronjob"}, want: "/apis/batch/v1/namespaces/ns1/cronjobs/c"},
		{name: "namespace 이름 없음", payload: model.K8sResourceYAMLPayload{ResourceType: "namespace"}, wantErrText: "namespace name is required"},
		{name: "pod ns 누락", payload: model.K8sResourceYAMLPayload{ResourceType: "pod", Name: "p1"}, wantErrText: "pod namespace and name are required"},
		{name: "pv 이름 없음", payload: model.K8sResourceYAMLPayload{ResourceType: "pv"}, wantErrText: "pv name is required"},
		{name: "workload 타입 미지원", payload: model.K8sResourceYAMLPayload{ResourceType: "workload", Namespace: "ns1", Name: "x", WorkloadType: "replicaset"}, wantErrText: "unsupported workload type"},
		{name: "리소스 타입 미지원", payload: model.K8sResourceYAMLPayload{ResourceType: "ghost"}, wantErrText: "unsupported resource type"},
	}
	for _, tc := range cases {
		got, err := buildK8sYAMLResourcePath(tc.payload) // legacy 1행
		if tc.wantErrText != "" {
			if err == nil || err.Error() != tc.wantErrText {
				t.Errorf("%s: err = %v, want %q", tc.name, err, tc.wantErrText)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Errorf("%s: = (%q, %v), want (%q, nil)", tc.name, got, err, tc.want)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// ns는 payload 우선·manifest 폴백, istio/gatewayapi는 manifest apiVersion으로 버전 선호가 현재 계약.
func TestCharBuildK8sCreateResourcePaths(t *testing.T) {
	istioManifest := k8sManifestIdentity{APIVersion: "networking.istio.io/v1beta1", Kind: "Gateway"}
	gatewayManifest := k8sManifestIdentity{APIVersion: "gateway.networking.k8s.io/v1beta1", Kind: "Gateway"}
	cases := []struct {
		name        string
		payload     model.K8sResourceYAMLPayload
		manifest    k8sManifestIdentity
		want        []string
		wantErrText string
	}{
		{name: "namespace는 단일 고정 경로", payload: model.K8sResourceYAMLPayload{ResourceType: "namespace"}, want: []string{"/api/v1/namespaces"}},
		{name: "pod payload ns", payload: model.K8sResourceYAMLPayload{ResourceType: "pod", Namespace: "ns1"}, want: []string{"/api/v1/namespaces/ns1/pods"}},
		{name: "pod manifest ns 폴백", payload: model.K8sResourceYAMLPayload{ResourceType: "pod"}, manifest: k8sManifestIdentity{Metadata: struct {
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
		}{Namespace: "ns2"}}, want: []string{"/api/v1/namespaces/ns2/pods"}},
		{name: "workload deployment", payload: model.K8sResourceYAMLPayload{ResourceType: "workload", Namespace: "ns1", WorkloadType: "deployment"}, want: []string{"/apis/apps/v1/namespaces/ns1/deployments"}},
		{name: "istio 선호 버전 전환", payload: model.K8sResourceYAMLPayload{ResourceType: "gateway", Namespace: "ns1"}, manifest: istioManifest,
			want: []string{"/apis/networking.istio.io/v1beta1/namespaces/ns1/gateways", "/apis/networking.istio.io/v1/namespaces/ns1/gateways"}},
		{name: "gatewayapi 선호 버전 전환", payload: model.K8sResourceYAMLPayload{ResourceType: "gatewayapi", Namespace: "ns1"}, manifest: gatewayManifest,
			want: []string{"/apis/gateway.networking.k8s.io/v1beta1/namespaces/ns1/gateways", "/apis/gateway.networking.k8s.io/v1/namespaces/ns1/gateways"}},
		{name: "pod ns 완전 누락", payload: model.K8sResourceYAMLPayload{ResourceType: "pod"}, wantErrText: "pod namespace is required"},
		{name: "workload 타입 미지원", payload: model.K8sResourceYAMLPayload{ResourceType: "workload", Namespace: "ns1", WorkloadType: "x"}, wantErrText: "unsupported workload type"},
		{name: "리소스 타입 미지원", payload: model.K8sResourceYAMLPayload{ResourceType: "ghost"}, wantErrText: "unsupported resource type"},
	}
	for _, tc := range cases {
		got, err := buildK8sCreateResourcePaths(tc.payload, tc.manifest) // legacy 1행
		if tc.wantErrText != "" {
			if err == nil || err.Error() != tc.wantErrText {
				t.Errorf("%s: err = %v, want %q", tc.name, err, tc.wantErrText)
			}
			continue
		}
		if err != nil || !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: = (%v, %v), want (%v, nil)", tc.name, got, err, tc.want)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// istio·gatewayapi·httproute는 버전 쌍 경로, 그 외는 단일 경로 위임이 현재 계약.
func TestCharBuildK8sYAMLResourcePaths(t *testing.T) {
	cases := []struct {
		name        string
		payload     model.K8sResourceYAMLPayload
		want        []string
		wantErrText string
	}{
		{name: "istio 버전 쌍", payload: model.K8sResourceYAMLPayload{ResourceType: "virtualservice", Namespace: "ns1", Name: "vs"},
			want: []string{"/apis/networking.istio.io/v1/namespaces/ns1/virtualservices/vs", "/apis/networking.istio.io/v1beta1/namespaces/ns1/virtualservices/vs"}},
		{name: "gatewayapi 버전 쌍", payload: model.K8sResourceYAMLPayload{ResourceType: "gatewayapi", Namespace: "ns1", Name: "gw"},
			want: []string{"/apis/gateway.networking.k8s.io/v1/namespaces/ns1/gateways/gw", "/apis/gateway.networking.k8s.io/v1beta1/namespaces/ns1/gateways/gw"}},
		{name: "기본은 단일 경로 위임", payload: model.K8sResourceYAMLPayload{ResourceType: "pod", Namespace: "ns1", Name: "p1"},
			want: []string{"/api/v1/namespaces/ns1/pods/p1"}},
		{name: "istio 이름 누락", payload: model.K8sResourceYAMLPayload{ResourceType: "gateway", Namespace: "ns1"}, wantErrText: "gateway namespace and name are required"},
		{name: "httproute 이름 누락", payload: model.K8sResourceYAMLPayload{ResourceType: "httproute", Namespace: "ns1"}, wantErrText: "httproute namespace and name are required"},
		{name: "위임 실패 전파", payload: model.K8sResourceYAMLPayload{ResourceType: "ghost"}, wantErrText: "unsupported resource type"},
	}
	for _, tc := range cases {
		got, err := buildK8sYAMLResourcePaths(tc.payload) // legacy 1행
		if tc.wantErrText != "" {
			if err == nil || err.Error() != tc.wantErrText {
				t.Errorf("%s: err = %v, want %q", tc.name, err, tc.wantErrText)
			}
			continue
		}
		if err != nil || !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: = (%v, %v), want (%v, nil)", tc.name, got, err, tc.want)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// 삭제 경로는 YAML 경로 위임(단일 경로)이 현재 계약.
func TestCharBuildK8sDeleteResourcePaths(t *testing.T) {
	cases := []struct {
		name        string
		payload     model.K8sResourceDeletePayload
		want        []string
		wantErrText string
	}{
		{name: "pod 단일 경로", payload: model.K8sResourceDeletePayload{ResourceType: "pod", Namespace: "ns1", Name: "p1"}, want: []string{"/api/v1/namespaces/ns1/pods/p1"}},
		{name: "namespace 전용 경로", payload: model.K8sResourceDeletePayload{ResourceType: "namespace", Name: "prod"}, want: []string{"/api/v1/namespaces/prod"}},
		{name: "미지원 위임 전파", payload: model.K8sResourceDeletePayload{ResourceType: "ghost"}, wantErrText: "unsupported resource type"},
	}
	for _, tc := range cases {
		got, err := buildK8sDeleteResourcePaths(tc.payload) // legacy 1행
		if tc.wantErrText != "" {
			if err == nil || err.Error() != tc.wantErrText {
				t.Errorf("%s: err = %v, want %q", tc.name, err, tc.wantErrText)
			}
			continue
		}
		if err != nil || !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: = (%v, %v), want (%v, nil)", tc.name, got, err, tc.want)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// 파서는 JSON이고 YAML 원문은 "invalid yaml content"로 떨어지는 현재 동작(문언 불일치 포함)을 고정.
func TestCharParseK8sManifestIdentity(t *testing.T) {
	cases := []struct {
		name        string
		body        []byte
		want        k8sManifestIdentity
		wantErrText string
	}{
		{name: "JSON 전체", body: []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"p1","namespace":"ns1"}}`),
			want: k8sManifestIdentity{APIVersion: "v1", Kind: "Pod", Metadata: struct {
				Name      string `json:"name"`
				Namespace string `json:"namespace"`
			}{Name: "p1", Namespace: "ns1"}}},
		{name: "YAML 원문 → invalid", body: []byte("kind: Pod\nmetadata:\n  name: p1"), wantErrText: "invalid yaml content"},
		{name: "kind 누락", body: []byte(`{"metadata":{"name":"p1"}}`), wantErrText: "resource kind is required"},
		{name: "kind 공백", body: []byte(`{"kind":"   "}`), wantErrText: "resource kind is required"},
		{name: "빈 바디", body: []byte(""), wantErrText: "invalid yaml content"},
	}
	for _, tc := range cases {
		identity, err := parseK8sManifestIdentity(tc.body) // legacy 1행
		if tc.wantErrText != "" {
			if err == nil || err.Error() != tc.wantErrText {
				t.Errorf("%s: err = %v, want %q", tc.name, err, tc.wantErrText)
			}
			continue
		}
		if err != nil || !reflect.DeepEqual(identity, tc.want) {
			t.Errorf("%s: = (%+v, %v), want (%+v, nil)", tc.name, identity, err, tc.want)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// 소문자화 매칭이며 "code":404 바디 형식도 404로 인정이 현재 계약.
func TestCharIsK8sNotFoundError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil → false", err: nil, want: false},
		{name: "정확 문언", err: errors.New("unexpected status: 404"), want: true},
		{name: "대소문자 무시", err: errors.New("Unexpected Status: 404, gone"), want: true},
		{name: "code 404 바디", err: errors.New(`request failed with {"code":404}`), want: true},
		{name: "500은 아님", err: errors.New("unexpected status: 500"), want: false},
		{name: "not found 문언만으론 아님", err: errors.New("resource not found"), want: false},
	}
	for _, tc := range cases {
		if got := isK8sNotFoundError(tc.err); got != tc.want { // legacy 1행
			t.Errorf("%s: = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// 매칭 순서 immutable → exists → not found → invalid, 미매칭은 원문 전파가 현재 계약.
func TestCharFriendlyK8sYAMLError(t *testing.T) {
	cases := []struct {
		name            string
		resourceType    string
		err             error
		wantContains    string
		wantPassthrough bool
		wantNil         bool
	}{
		{name: "pod immutable", resourceType: "pod", err: errors.New("field is immutable"), wantContains: "recreate the Pod"},
		{name: "pvc immutable", resourceType: "pvc", err: errors.New("Field is Immutable"), wantContains: "storage resource contains immutable"},
		{name: "기본 immutable", resourceType: "service", err: errors.New("immutable"), wantContains: "cannot be overwritten directly"},
		{name: "already exists", err: errors.New("already exists"), wantContains: "identity conflicts"},
		{name: "not found", err: errors.New("not found"), wantContains: "does not exist"},
		{name: "invalid", err: errors.New("invalid value"), wantContains: "YAML validation failed"},
		{name: "미매칭 원문 전파", err: errors.New("boom"), wantPassthrough: true},
		{name: "nil은 nil", wantNil: true},
	}
	for _, tc := range cases {
		friendly := friendlyK8sYAMLError(model.K8sResourceYAMLPayload{ResourceType: tc.resourceType}, tc.err) // legacy 1행
		if tc.wantNil {
			if friendly != nil {
				t.Errorf("%s: = %v, want nil", tc.name, friendly)
			}
			continue
		}
		if tc.wantPassthrough {
			if friendly == nil || friendly.Error() != tc.err.Error() {
				t.Errorf("%s: = %v, want 원문 %v", tc.name, friendly, tc.err)
			}
			continue
		}
		if friendly == nil || !strings.Contains(friendly.Error(), tc.wantContains) {
			t.Errorf("%s: = %v, want contains %q", tc.name, friendly, tc.wantContains)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// GET·query 인코딩·Bearer 인증·바디 원문 반환, 응답 Content-Type은 무시(dust)하는 것이 현재 계약.
func TestCharK8sGetText(t *testing.T) {
	cases := []struct {
		name         string
		token        string
		query        map[string]string
		status       int
		responseBody string
		want         string
		wantErr      string
	}{
		{name: "text 응답 원문", status: 200, responseBody: "cpu 1.5\nmem 2.0\n", want: "cpu 1.5\nmem 2.0\n"},
		{name: "쿼리·토큰", token: "tok", query: map[string]string{"range": "1h"}, status: 200, responseBody: "ok", want: "ok"},
		{name: "500", status: 500, responseBody: "boom", wantErr: "unexpected status: 500"},
	}
	for _, tc := range cases {
		var observed charObserved
		client, runtime := charCaptureStub(t, tc.status, tc.responseBody, &observed)
		runtime.Token = tc.token
		got, err := k8sGetText(client, runtime, "/metrics", tc.query) // legacy 1행
		if charWantErr(t, tc.name, err, tc.wantErr) {
			continue
		}
		if got != tc.want {
			t.Errorf("%s: = %q, want %q", tc.name, got, tc.want)
		}
		if observed.Method != http.MethodGet || observed.Path != "/metrics" {
			t.Errorf("%s: 요청 = %s %s", tc.name, observed.Method, observed.Path)
		}
		wantAuth := ""
		if tc.token != "" {
			wantAuth = "Bearer " + tc.token
		}
		if observed.Authorization != wantAuth {
			t.Errorf("%s: authorization = %q, want %q", tc.name, observed.Authorization, wantAuth)
		}
		if tc.query != nil && observed.Query.Get("range") != "1h" {
			t.Errorf("%s: query = %v", tc.name, observed.Query)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
func TestCharK8sStatusText(t *testing.T) {
	cases := []struct {
		status string
		want   string
	}{
		{status: "warning", want: "Partial Alerts"},
		{status: "OFFLINE", want: "Offline"},
		{status: " running ", want: "Running"},
		{status: "", want: "Running"},
	}
	for _, tc := range cases {
		if got := k8sStatusText(tc.status); got != tc.want { // legacy 1행
			t.Errorf("%q: = %q, want %q", tc.status, got, tc.want)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// 미지정 값은 전부 running 수렴이 현재 계약.
func TestCharNormalizedK8sStatus(t *testing.T) {
	cases := []struct {
		value string
		want  string
	}{
		{value: " Warning ", want: "warning"},
		{value: "offline", want: "offline"},
		{value: "running", want: "running"},
		{value: "bogus", want: "running"},
		{value: "", want: "running"},
	}
	for _, tc := range cases {
		if got := normalizedK8sStatus(tc.value); got != tc.want { // legacy 1행
			t.Errorf("%q: = %q, want %q", tc.value, got, tc.want)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// 감점은 건당 8점, 하한 40, 0 이하는 만점이 현재 계약.
func TestCharCalculateHealthScore(t *testing.T) {
	cases := []struct {
		alertCount int
		want       int
	}{
		{alertCount: 0, want: 100},
		{alertCount: -3, want: 100},
		{alertCount: 1, want: 92},
		{alertCount: 5, want: 60},
		{alertCount: 8, want: 40},
		{alertCount: 100, want: 40},
	}
	for _, tc := range cases {
		if got := calculateHealthScore(tc.alertCount); got != tc.want { // legacy 1행
			t.Errorf("%d: = %d, want %d", tc.alertCount, got, tc.want)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// 0·음수 입력은 "-", 100% 초과도 그대로 표기, 소수 1자리 고정이 현재 계약.
func TestCharFormatUsagePercent(t *testing.T) {
	cases := []struct {
		used  int64
		total int64
		want  string
	}{
		{used: 0, total: 10, want: "-"},
		{used: -1, total: 10, want: "-"},
		{used: 5, total: 0, want: "-"},
		{used: 1, total: 3, want: "33.3%"},
		{used: 1, total: 4, want: "25.0%"},
		{used: 2, total: 1, want: "200.0%"},
	}
	for _, tc := range cases {
		if got := formatUsagePercent(tc.used, tc.total); got != tc.want { // legacy 1행
			t.Errorf("(%d,%d): = %q, want %q", tc.used, tc.total, got, tc.want)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// Ready 조건의 Status True/False 분기와 부재 시 Unknown이 현재 계약.
func TestCharNodeReadyStatus(t *testing.T) {
	ready := charNode("n")
	ready.Status.Conditions = append(ready.Status.Conditions, charCondition("Ready", "True"))
	notReady := charNode("n")
	notReady.Status.Conditions = append(notReady.Status.Conditions, charCondition("Ready", "False"))
	otherOnly := charNode("n")
	otherOnly.Status.Conditions = append(otherOnly.Status.Conditions, charCondition("MemoryPressure", "True"))
	cases := []struct {
		name string
		node kubeNode
		want string
	}{
		{name: "Ready", node: ready, want: "Ready"},
		{name: "NotReady", node: notReady, want: "NotReady"},
		{name: "조건 없음 → Unknown", node: charNode("n"), want: "Unknown"},
		{name: "다른 조건만 → Unknown", node: otherOnly, want: "Unknown"},
	}
	for _, tc := range cases {
		if got := nodeReadyStatus(tc.node); got != tc.want { // legacy 1행
			t.Errorf("%s: = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// 공백 InternalIP는 건너뛰고 다음 후보, 없으면 "-"가 현재 계약.
func TestCharFirstNodeInternalIP(t *testing.T) {
	node := charNode("n")
	node.Status.Addresses = append(node.Status.Addresses, charNodeAddress("InternalIP", "  "), charNodeAddress("Hostname", "host1"), charNodeAddress("InternalIP", "10.0.0.1"))
	cases := []struct {
		name string
		node kubeNode
		want string
	}{
		{name: "공백 건너뛰기", node: node, want: "10.0.0.1"},
		{name: "없으면 -", node: charNode("n"), want: "-"},
	}
	for _, tc := range cases {
		if got := firstNodeInternalIP(tc.node); got != tc.want { // legacy 1행
			t.Errorf("%s: = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// 역할 키 정렬, 빈 접미 키는 "worker", 역할 없어도 "worker"가 현재 계약.
func TestCharJoinNodeRoles(t *testing.T) {
	cases := []struct {
		name   string
		labels map[string]string
		want   string
	}{
		{name: "정렬 결합", labels: map[string]string{"node-role.kubernetes.io/worker": "", "node-role.kubernetes.io/control-plane": ""}, want: "control-plane,worker"},
		{name: "빈 접미 키 → worker", labels: map[string]string{"node-role.kubernetes.io/": "", "node-role.kubernetes.io/master": ""}, want: "master,worker"},
		{name: "역할 없음 → worker", labels: map[string]string{"app": "x"}, want: "worker"},
		{name: "빈 맵 → worker", labels: nil, want: "worker"},
	}
	for _, tc := range cases {
		if got := joinNodeRoles(tc.labels); got != tc.want { // legacy 1행
			t.Errorf("%s: = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// "m" 접미는 소수 절사, 무접미는 *1000, 파싱 실패는 0이 현재 계약.
func TestCharParseCPUToMilli(t *testing.T) {
	cases := []struct {
		value string
		want  int64
	}{
		{value: "", want: 0},
		{value: "500m", want: 500},
		{value: "250.5m", want: 250},
		{value: "2", want: 2000},
		{value: "1.5", want: 1500},
		{value: " 1 ", want: 1000},
		{value: "-500m", want: -500},
		{value: "abc", want: 0},
		{value: "0.0005", want: 0},
	}
	for _, tc := range cases {
		if got := parseCPUToMilli(tc.value); got != tc.want { // legacy 1행
			t.Errorf("%q: = %d, want %d", tc.value, got, tc.want)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// 이진/십진 접미 혼용, Ei는 미지원(0), 파싱 실패 0이 현재 계약.
func TestCharParseBytesQuantity(t *testing.T) {
	ki := int64(1024)
	cases := []struct {
		value string
		want  int64
	}{
		{value: "", want: 0},
		{value: "1Ki", want: ki},
		{value: "128Mi", want: 128 * 1024 * 1024},
		{value: "1Gi", want: 1073741824},
		{value: "1Ti", want: 1099511627776},
		{value: "1Pi", want: 1125899906842624},
		{value: "1K", want: 1000},
		{value: "2M", want: 2000000},
		{value: "3G", want: 3000000000},
		{value: "1T", want: 1000000000000},
		{value: "42", want: 42},
		{value: "1.5Gi", want: 1610612736},
		{value: "1Ei", want: 0},
		{value: "abc", want: 0},
	}
	for _, tc := range cases {
		if got := parseBytesQuantity(tc.value); got != tc.want { // legacy 1행
			t.Errorf("%q: = %d, want %d", tc.value, got, tc.want)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// 십진 MB(10^6) 반올림, 0 이하 "-"가 현재 계약.
func TestCharFormatMemoryMB(t *testing.T) {
	cases := []struct {
		value string
		want  string
	}{
		{value: "", want: "-"},
		{value: "0", want: "-"},
		{value: "8Gi", want: "8590 MB"},
		{value: "1Mi", want: "1 MB"},
		{value: "999999", want: "1 MB"},
	}
	for _, tc := range cases {
		if got := formatMemoryMB(tc.value); got != tc.want { // legacy 1행
			t.Errorf("%q: = %q, want %q", tc.value, got, tc.want)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// 시그니처가 now 주입 불가라 분기 커버는 now 상대 입력으로 현실 범위만 고정한다.
// 미래 타임스탬프는 음수 duration이 "Just now"로 수렴하는 것까지 현재 계약.
func TestCharHumanizeAge(t *testing.T) {
	cases := []struct {
		name      string
		timestamp string
		want      string
	}{
		{name: "파싱 실패 → -", timestamp: "bogus", want: "-"},
		{name: "30일 이상 → 날짜", timestamp: "2026-01-01T00:00:00Z", want: "2026-01-01"},
		{name: "2시간 전 → h", timestamp: time.Now().Add(-2 * time.Hour).Format(time.RFC3339), want: "2h"},
		{name: "3일 전 → d", timestamp: time.Now().Add(-72 * time.Hour).Format(time.RFC3339), want: "3d"},
		{name: "40분 전 → m", timestamp: time.Now().Add(-40 * time.Minute).Format(time.RFC3339), want: "40m"},
		{name: "미래 → Just now", timestamp: "2030-01-01T00:00:00Z", want: "Just now"},
	}
	for _, tc := range cases {
		if got := humanizeAge(tc.timestamp); got != tc.want { // legacy 1행
			t.Errorf("%s: = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// RFC3339 파싱 실패 "-", 원본 위치 시각 유지(+09:00 오프셋 보존)가 현재 계약.
func TestCharFormatTimestamp(t *testing.T) {
	cases := []struct {
		value string
		want  string
	}{
		{value: "2026-01-02T03:04:05Z", want: "2026-01-02 03:04"},
		{value: " 2026-01-02T03:04:05Z ", want: "2026-01-02 03:04"},
		{value: "2026-01-02T03:04:05+09:00", want: "2026-01-02 03:04"},
		{value: "", want: "-"},
		{value: "bogus", want: "-"},
	}
	for _, tc := range cases {
		if got := formatTimestamp(tc.value); got != tc.want { // legacy 1행
			t.Errorf("%q: = %q, want %q", tc.value, got, tc.want)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// float64는 int 절사, 그 외 타입은 fmt.Sprint 경로가 현재 계약.
func TestCharStringifyTargetPort(t *testing.T) {
	cases := []struct {
		name  string
		value any
		want  string
	}{
		{name: "문자열", value: "web", want: "web"},
		{name: "float64 정수", value: float64(8080), want: "8080"},
		{name: "float64 소수 절사", value: 80.5, want: "80"},
		{name: "int는 default 경로", value: 80, want: "80"},
		{name: "nil", value: nil, want: "<nil>"},
		{name: "불", value: true, want: "true"},
	}
	for _, tc := range cases {
		if got := stringifyTargetPort(tc.value); got != tc.want { // legacy 1행
			t.Errorf("%s: = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
func TestCharIntValue(t *testing.T) {
	five := 5
	cases := []struct {
		name  string
		value *int
		want  int
	}{
		{name: "nil → 0", value: nil, want: 0},
		{name: "값 전달", value: &five, want: 5},
	}
	for _, tc := range cases {
		if got := intValue(tc.value); got != tc.want { // legacy 1행
			t.Errorf("%s: = %d, want %d", tc.name, got, tc.want)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// suspend 우선, active 수량 표기, 기본 Scheduled가 현재 계약.
func TestCharCronJobReadyText(t *testing.T) {
	suspended := true
	inactive := false
	cases := []struct {
		name    string
		suspend *bool
		active  int
		want    string
	}{
		{name: "nil·0 → Scheduled", suspend: nil, active: 0, want: "Scheduled"},
		{name: "false·0 → Scheduled", suspend: &inactive, active: 0, want: "Scheduled"},
		{name: "active → N Active", suspend: nil, active: 2, want: "2 Active"},
		{name: "suspend 우선", suspend: &suspended, active: 2, want: "Suspended"},
	}
	for _, tc := range cases {
		if got := cronJobReadyText(tc.suspend, tc.active); got != tc.want { // legacy 1행
			t.Errorf("%s: = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
func TestCharFallbackText(t *testing.T) {
	cases := []struct {
		value string
		want  string
	}{
		{value: "", want: "-"},
		{value: "   ", want: "-"},
		{value: "x", want: "x"},
	}
	for _, tc := range cases {
		if got := fallbackText(tc.value); got != tc.want { // legacy 1행
			t.Errorf("%q: = %q, want %q", tc.value, got, tc.want)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
func TestCharIntLabel(t *testing.T) {
	cases := []struct {
		value  int
		suffix string
		want   string
	}{
		{value: 3, suffix: " nodes", want: "3 nodes"},
		{value: 0, suffix: "", want: "0"},
		{value: -1, suffix: "x", want: "-1x"},
	}
	for _, tc := range cases {
		if got := intLabel(tc.value, tc.suffix); got != tc.want { // legacy 1행
			t.Errorf("(%d,%q): = %q, want %q", tc.value, tc.suffix, got, tc.want)
		}
	}
}
