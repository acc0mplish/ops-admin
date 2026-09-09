// k8s_chartest_helper_test.go — Phase Z1 characterization 공유 헬퍼.
// 계획 §3.1이 `k8s_chartest_helper.go`로 명기했으나 _test.go로 작성한 것이 명시된 이탈이다:
// 같은 패키지 _test 파일끼리 심볼을 공유해 Z2·Z3에서 재사용 가능하고, service 패키지에
// 프로덕션 심볼 추가가 0이 되어 behavior-neutral을 유지한다 (팀리드 지시, 2026-09-09).
// 전부 in-process — 실제 클러스터·DB·SSH에 접속하지 않는다.
package service

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"ops-admin/backend/model"
)

// charWantErr는 wantErr("" = 에러 없음 기대)를 검사한다. 기대 에러면 true.
func charWantErr(t *testing.T, name string, err error, wantErr string) bool {
	t.Helper()
	if wantErr == "" {
		if err != nil {
			t.Fatalf("%s: unexpected err: %v", name, err)
		}
		return false
	}
	if err == nil || !strings.Contains(err.Error(), wantErr) {
		t.Fatalf("%s: err = %v, want contains %q", name, err, wantErr)
	}
	return true
}

// charStub은 스텁 kube API의 단일 경로 응답이다. status 0 + body ""는 미등록(404 취급).
type charStub struct {
	status int
	body   string
}

// charStubKube는 routes에 등록된 경로만 응답하는 in-process kube API 스텁을 띄운다.
// 경로 매칭은 RequestURI(쿼리 포함) 우선, 없으면 Path. 미등록 경로는 바디 없는
// 404 ("unexpected status: 404")로 답한다 — kube API의 NotFound와 같은 형태라
// isK8sNotFoundError 경로도 함께 커버된다.
func charStubKube(t *testing.T, routes map[string]charStub) (*http.Client, kubeClusterRuntime) {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stub, ok := routes[r.URL.RequestURI()]
		if !ok {
			stub, ok = routes[r.URL.Path]
		}
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(stub.status)
		_, _ = io.WriteString(w, stub.body)
	}))
	t.Cleanup(ts.Close)
	return ts.Client(), kubeClusterRuntime{Server: ts.URL}
}

// charDeadKube는 항상 접속 실패하는 클라이언트·런타임 쌍을 돌려준다 (포트 1 = refused).
func charDeadKube() (*http.Client, kubeClusterRuntime) {
	return &http.Client{Timeout: 2 * time.Second}, kubeClusterRuntime{Server: "http://127.0.0.1:1"}
}

// charJSON은 픽스처 직렬화용 — 실패는 패닉(테스트 픽스처 버그이므로).
func charJSON(v any) string {
	body, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(body)
}

// charPodsStubFor는 build*Detail이 쓰는 네임스페이스 파드 목록 단일 경로 스텁이다.
func charPodsStubFor(t *testing.T, namespace string, pods []kubePod) (*http.Client, kubeClusterRuntime) {
	t.Helper()
	return charStubKube(t, map[string]charStub{
		"/api/v1/namespaces/" + namespace + "/pods": {status: http.StatusOK, body: charJSON(kubePodListResponse{Items: pods})},
	})
}

func charMeta(name, namespace string) kubeMetadata {
	return kubeMetadata{Name: name, Namespace: namespace, CreationTimestamp: "2026-01-01T00:00:00Z"}
}

func charPod(name, namespace string) kubePod {
	return kubePod{Metadata: charMeta(name, namespace)}
}

func charPodWithLabels(name, namespace string, labels map[string]string) kubePod {
	pod := charPod(name, namespace)
	pod.Metadata.Labels = labels
	return pod
}

// charOwner는 kubeMetadata.OwnerReferences 요소를 만든다 (익명 구조체라 태그 일치 필수).
func charOwner(kind, name string) struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
} {
	return struct {
		Kind string `json:"kind"`
		Name string `json:"name"`
	}{Kind: kind, Name: name}
}

func charPodWithOwner(name, namespace, kind, owner string) kubePod {
	pod := charPod(name, namespace)
	pod.Metadata.OwnerReferences = append(pod.Metadata.OwnerReferences, charOwner(kind, owner))
	return pod
}

func charNode(name string) kubeNode {
	return kubeNode{Metadata: charMeta(name, "")}
}

func charNamespace(name string) kubeNamespace {
	return kubeNamespace{Metadata: charMeta(name, "")}
}

// charCondition은 kubeNode.Status.Conditions 요소를 만든다.
func charCondition(conditionType, status string) struct {
	Type   string `json:"type"`
	Status string `json:"status"`
} {
	return struct {
		Type   string `json:"type"`
		Status string `json:"status"`
	}{Type: conditionType, Status: status}
}

func charEndpoint(name, namespace string, addressCounts ...int) kubeEndpoints {
	var endpoint kubeEndpoints
	endpoint.Metadata = charMeta(name, namespace)
	for _, count := range addressCounts {
		endpoint.Subsets = append(endpoint.Subsets, struct {
			Addresses         []struct{} `json:"addresses"`
			NotReadyAddresses []struct{} `json:"notReadyAddresses"`
		}{Addresses: make([]struct{}, count)})
	}
	return endpoint
}

func charDeployment(name, namespace string) kubeDeployment {
	var deployment kubeDeployment
	deployment.Metadata = charMeta(name, namespace)
	replicas := 1
	deployment.Spec.Replicas = &replicas
	return deployment
}

func charStatefulSet(name, namespace string) kubeStatefulSet {
	var set kubeStatefulSet
	set.Metadata = charMeta(name, namespace)
	replicas := 1
	set.Spec.Replicas = &replicas
	return set
}

func charDaemonSet(name, namespace string) kubeDaemonSet {
	var set kubeDaemonSet
	set.Metadata = charMeta(name, namespace)
	return set
}

func charJob(name, namespace string) kubeJob {
	var job kubeJob
	job.Metadata = charMeta(name, namespace)
	return job
}

func charCronJob(name, namespace string) kubeCronJob {
	var job kubeCronJob
	job.Metadata = charMeta(name, namespace)
	return job
}

// charContainerStatus는 kubePod.Status.ContainerStatuses 요소를 만든다.
func charContainerStatus(name string, restarts int) struct {
	Name         string `json:"name"`
	RestartCount int    `json:"restartCount"`
	Ready        bool   `json:"ready"`
} {
	return struct {
		Name         string `json:"name"`
		RestartCount int    `json:"restartCount"`
		Ready        bool   `json:"ready"`
	}{Name: name, RestartCount: restarts, Ready: true}
}

func charKubePodNames(pods []kubePod) []string {
	names := make([]string, 0, len(pods))
	for _, pod := range pods {
		names = append(names, pod.Metadata.Name)
	}
	return names
}

func charPodItemNames(items []model.K8sPodItem) []string {
	names := make([]string, 0, len(items))
	for _, item := range items {
		names = append(names, item.Name)
	}
	return names
}

// charNodeReady는 InternalIP·Ready 조건·kubelet·OS·allocatable·capacity pods를 채운 노드.
// 빈 값은 그대로 비워 둔다(소스의 "-" 폴백을 검증하는 데 쓴다).
func charNodeReady(name, internalIP, readyCondition, kubelet, osImage, allocCPU, allocMem, capacityPods string) kubeNode {
	node := charNode(name)
	if internalIP != "" {
		node.Status.Addresses = append(node.Status.Addresses, struct {
			Type    string `json:"type"`
			Address string `json:"address"`
		}{Type: "InternalIP", Address: internalIP})
	}
	if readyCondition != "" {
		node.Status.Conditions = append(node.Status.Conditions, charCondition("Ready", readyCondition))
	}
	node.Status.NodeInfo.KubeletVersion = kubelet
	node.Status.NodeInfo.OSImage = osImage
	node.Status.Allocatable = map[string]string{"cpu": allocCPU, "memory": allocMem}
	node.Status.Capacity = map[string]string{"pods": capacityPods}
	return node
}

func charDeploymentStatus(name, namespace string, replicas, ready, updated, available int) kubeDeployment {
	dep := charDeployment(name, namespace)
	dep.Spec.Replicas = &replicas
	dep.Status.ReadyReplicas = ready
	dep.Status.UpdatedReplicas = updated
	dep.Status.AvailableReplicas = available
	return dep
}

func charDaemonSetStatus(name, namespace string, desired, updated, ready, available int) kubeDaemonSet {
	set := charDaemonSet(name, namespace)
	set.Status.DesiredNumberScheduled = desired
	set.Status.UpdatedNumberScheduled = updated
	set.Status.NumberReady = ready
	set.Status.NumberAvailable = available
	return set
}

func charJobStatus(name, namespace string, succeeded, active, failed int) kubeJob {
	job := charJob(name, namespace)
	job.Status.Succeeded = succeeded
	job.Status.Active = active
	job.Status.Failed = failed
	return job
}

// charSelfSignedCertPEM은 지정 CN·유효기간의 자가서명 인증서를 base64(PEM)로 돌려준다.
func charSelfSignedCertPEM(commonName string, notBefore, notAfter time.Time) string {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(err)
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: commonName},
		Issuer:       pkix.Name{CommonName: commonName},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		panic(err)
	}
	encoded := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	return base64.StdEncoding.EncodeToString(encoded)
}

// ---- 시나리오 픽스처 (fetch 군 테스트 공용 — Z2·Z3에서도 재사용 가능) ----

// charCoreRoutes는 fetchK8sData의 필수 경로 7종 + deployments 응답을 채운 스텁 라우트.
func charCoreRoutes() map[string]charStub {
	return map[string]charStub{
		"/api/v1/nodes":             {status: 200, body: charJSON(kubeNodeListResponse{Items: []kubeNode{charNode("node-1")}})},
		"/api/v1/namespaces":        {status: 200, body: charJSON(kubeNamespaceListResponse{Items: []kubeNamespace{charNamespace("default")}})},
		"/api/v1/pods":              {status: 200, body: charJSON(kubePodListResponse{Items: []kubePod{charPod("p", "default")}})},
		"/api/v1/services":          {status: 200, body: charJSON(kubeServiceListResponse{Items: make([]kubeService, 2)})},
		"/api/v1/endpoints":         {status: 200, body: charJSON(kubeEndpointListResponse{Items: make([]kubeEndpoints, 1)})},
		"/api/v1/configmaps":        {status: 200, body: charJSON(kubeConfigMapListResponse{Items: make([]kubeConfigMap, 1)})},
		"/api/v1/secrets":           {status: 200, body: charJSON(kubeSecretListResponse{Items: make([]kubeSecret, 1)})},
		"/apis/apps/v1/deployments": {status: 200, body: charJSON(kubeDeploymentListResponse{Items: make([]kubeDeployment, 1)})},
	}
}

// charFetchedCounts는 fetchK8sData 결과의 리스트별 건수를 필드명 키로 돌려준다.
func charFetchedCounts(data k8sFetchedData) map[string]int {
	return map[string]int{"Nodes": len(data.Nodes), "Namespaces": len(data.Namespaces), "Pods": len(data.Pods),
		"Services": len(data.Services), "Endpoints": len(data.Endpoints), "ConfigMaps": len(data.ConfigMaps),
		"Secrets": len(data.Secrets), "Deployments": len(data.Deployments), "Ingresses": len(data.Ingresses),
		"PVCs": len(data.PVCs), "GatewayAPIGateways": len(data.GatewayAPIGateways), "HTTPRoutes": len(data.HTTPRoutes)}
}

// charNSWorkloadRoutes는 네임스페이스별 워크로드 수 조회 5경로에 n건씩 응답한다.
func charNSWorkloadRoutes(deployments, statefulsets, daemonsets, jobs, cronjobs int) map[string]charStub {
	items := func(n int) string { return charJSON(kubeDeploymentListResponse{Items: make([]kubeDeployment, n)}) }
	return map[string]charStub{
		"/apis/apps/v1/namespaces/prod/deployments":  {status: 200, body: items(deployments)},
		"/apis/apps/v1/namespaces/prod/statefulsets": {status: 200, body: items(statefulsets)},
		"/apis/apps/v1/namespaces/prod/daemonsets":   {status: 200, body: items(daemonsets)},
		"/apis/batch/v1/namespaces/prod/jobs":        {status: 200, body: items(jobs)},
		"/apis/batch/v1/namespaces/prod/cronjobs":    {status: 200, body: items(cronjobs)},
	}
}

// charEventsBody는 이벤트 2건(정상 1·빈값 1)의 응답 바디.
func charEventsBody() string {
	return `{"items":[{"type":"Warning","reason":"BackOff","message":"back-off","count":5,` +
		`"firstTimestamp":"2026-01-02T03:04:05Z","lastTimestamp":"2026-01-02T03:05:05Z"},{"type":"","reason":"","message":"","count":0,"firstTimestamp":"","lastTimestamp":"2026-01-02T03:04:05Z"}]}`
}

// charKubeadmConfigMap은 serviceSubnet·podSubnet을 담은 kube-system/kubeadm-config.
func charKubeadmConfigMap(data string) kubeConfigMap {
	return kubeConfigMap{Metadata: charMeta("kubeadm-config", "kube-system"), Data: map[string]string{"ClusterConfiguration": data}}
}

// charCIDRNode는 PodCIDR·PodCIDRs를 지정한 노드.
func charCIDRNode(cidr string, cidrs ...string) kubeNode {
	node := charNode("n")
	node.Spec.PodCIDR = cidr
	node.Spec.PodCIDRs = cidrs
	return node
}

// charDeployWithSelector는 셀렉터를 지정한 default 네임스페이스 Deployment.
func charDeployWithSelector(name string, selector map[string]string) kubeDeployment {
	dep := charDeployment(name, "default")
	dep.Spec.Selector.MatchLabels = selector
	return dep
}

// charReplicaSetOwnedByDeployment는 Deployment 오너를 가진 ReplicaSet.
func charReplicaSetOwnedByDeployment(name, owner string) kubeReplicaSet {
	rs := kubeReplicaSet{Metadata: charMeta(name, "default")}
	rs.Metadata.OwnerReferences = append(rs.Metadata.OwnerReferences, charOwner("Deployment", owner))
	return rs
}

// charJobOwnedByCronJob은 CronJob 오너를 가진 Job.
func charJobOwnedByCronJob(name, owner string) kubeJob {
	job := charJob(name, "default")
	job.Metadata.OwnerReferences = append(job.Metadata.OwnerReferences, charOwner("CronJob", owner))
	return job
}

// charRunningPod는 상태·노드·IP·재시작 카운트가 채워진 파드.
func charRunningPod(name, namespace string) kubePod {
	pod := charPod(name, namespace)
	pod.Status.Phase = "Running"
	pod.Spec.NodeName = "node-1"
	pod.Status.PodIP = "10.1.0.5"
	pod.Status.HostIP = "10.0.0.1"
	pod.Status.ContainerStatuses = append(pod.Status.ContainerStatuses, charContainerStatus("app", 2), charContainerStatus("sidecar", 3))
	return pod
}

// charAllKindWorkloads는 5종 워크로드가 각 1개씩 있는 상태 채움 데이터(Sts는 nil 복제본).
func charAllKindWorkloads() k8sFetchedData {
	sts := charStatefulSet("db", "app")
	sts.Spec.Replicas = nil
	return k8sFetchedData{
		Deployments: []kubeDeployment{charDeploymentStatus("web", "app", 3, 2, 1, 2)},
		StatefulSet: []kubeStatefulSet{sts},
		DaemonSets:  []kubeDaemonSet{charDaemonSetStatus("agent", "app", 3, 1, 1, 1)},
		Jobs:        []kubeJob{charJobStatus("nightly", "app", 2, 1, 1)},
		CronJobs:    []kubeCronJob{charCronJob("cleanup", "app")},
	}
}

// charWorkloadHead는 build*Detail 결과에서 고정할 머리 필드 묶음.
type charWorkloadHead struct {
	name, typ, namespace, ready, age string
	updated, available               int
	podNames                         []string
	selector                         map[string]string
	firstImage                       string
	yamlContains                     string // ""면 YAML 내용 미검사
}

// charWantWorkloadDetail은 build*Detail 결과의 머리 필드·파드 이름·첫 컨테이너 이미지·YAML 비어있지 않음을 검사한다.
func charWantWorkloadDetail(t *testing.T, caseName string, got model.K8sWorkloadDetail, want charWorkloadHead) {
	t.Helper()
	if got.Name != want.name || got.Type != want.typ || got.Namespace != want.namespace || got.Ready != want.ready ||
		got.Updated != want.updated || got.Available != want.available || got.Age != want.age {
		t.Errorf("%s: head = %s/%s/%s ready=%q up=%d avail=%d age=%q, want %s/%s/%s ready=%q up=%d avail=%d age=%q",
			caseName, got.Name, got.Type, got.Namespace, got.Ready, got.Updated, got.Available, got.Age,
			want.name, want.typ, want.namespace, want.ready, want.updated, want.available, want.age)
	}
	if names := charPodItemNames(got.Pods); len(names) != len(want.podNames) || (len(names) > 0 && !reflect.DeepEqual(names, want.podNames)) {
		t.Errorf("%s: pods = %v, want %v", caseName, names, want.podNames)
	}
	if !reflect.DeepEqual(got.Selector, want.selector) {
		t.Errorf("%s: selector = %v, want %v", caseName, got.Selector, want.selector)
	}
	firstImage := ""
	if len(got.Containers) > 0 {
		firstImage = got.Containers[0].Image
	}
	if firstImage != want.firstImage {
		t.Errorf("%s: firstImage = %q, want %q", caseName, firstImage, want.firstImage)
	}
	if got.YAML == "" {
		t.Errorf("%s: yaml 비어 있음", caseName)
	}
	if want.yamlContains != "" && !strings.Contains(got.YAML, want.yamlContains) {
		t.Errorf("%s: yaml에 %q 없음", caseName, want.yamlContains)
	}
}
