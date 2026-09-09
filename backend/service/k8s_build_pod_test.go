// k8s_build_pod_test.go — Phase Z1 characterization (k8s_build_pod 시맨 순수 함수 21개).
// 목적은 "옳음"이 아니라 "불변": 현재 동작을 그대로 기록해 B~D2 파일 분해의 유일한
// 검출기이자 P의 V2 동치 오라클이 되게 한다 (계획 §J0·§12 #16, R18 — legacy 호출 1행).
package service

import (
	"net/http"
	"reflect"
	"testing"

	"ops-admin/backend/model"
)

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// 요청/limit 맵의 "cpu"·"memory" 키만 옮기고 env 소스 문자열은 formatK8sEnvSource 위임이 현재 계약.
func TestCharBuildContainerItems(t *testing.T) {
	container := kubeContainer{Name: "app", Image: "nginx:1", ImagePullPolicy: "IfNotPresent",
		Env: []kubeEnvVar{{Name: "A", Value: "1"}, {Name: "B", ValueFrom: map[string]any{"configMapKeyRef": map[string]any{"name": "cm", "key": "k"}}}}}
	container.Resources.Requests = map[string]string{"cpu": "500m", "memory": "1Gi"}
	container.Resources.Limits = map[string]string{"cpu": "1", "memory": "2Gi"}
	cases := []struct {
		name       string
		containers []kubeContainer
		want       []model.K8sContainerItem
	}{
		{name: "요청·limit·env 소스 매핑", containers: []kubeContainer{container},
			want: []model.K8sContainerItem{{Name: "app", Image: "nginx:1", RequestCPU: "500m", LimitCPU: "1", RequestMemory: "1Gi", LimitMemory: "2Gi", ImagePullPolicy: "IfNotPresent",
				Env: []model.K8sEnvVarItem{{Name: "A", Value: "1"}, {Name: "B", ValueFrom: container.Env[1].ValueFrom, Source: "configMapKeyRef: cm/k"}}}}},
		{name: "nil 입력 → 빈 슬라이스", containers: nil, want: []model.K8sContainerItem{}},
	}
	for _, tc := range cases {
		if items := buildContainerItems(tc.containers); !reflect.DeepEqual(items, tc.want) { // legacy 1행
			t.Errorf("%s: items = %+v, want %+v", tc.name, items, tc.want)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// Ready 산식 "%d/%d"(복제본 nil → 0), 셀렉터 매칭 파드만 수집, 파드 조회 실패 무시가 현재 계약.
func TestCharBuildDeploymentDetail(t *testing.T) {
	dep := charDeploymentStatus("web", "default", 2, 1, 1, 1)
	dep.Spec.Selector.MatchLabels = map[string]string{"app": "web"}
	dep.Spec.Template.Spec.Containers = append(dep.Spec.Template.Spec.Containers, kubeContainer{Name: "app", Image: "nginx:1"})
	noReplicas := charDeploymentStatus("web", "default", 0, 3, 0, 0)
	noReplicas.Spec.Replicas = nil
	cases := []struct {
		name  string
		setup func(*testing.T) (*http.Client, kubeClusterRuntime)
		item  kubeDeployment
		want  charWorkloadHead
	}{
		{name: "셀렉터 매칭 파드만", setup: func(t *testing.T) (*http.Client, kubeClusterRuntime) {
			return charPodsStubFor(t, "default", []kubePod{charPodWithLabels("web-1", "default", map[string]string{"app": "web"}), charPodWithLabels("other-1", "default", map[string]string{"app": "api"})})
		}, item: dep, want: charWorkloadHead{name: "web", typ: "Deployment", namespace: "default", ready: "1/2", age: "2026-01-01", updated: 1, available: 1,
			podNames: []string{"web-1"}, selector: map[string]string{"app": "web"}, firstImage: "nginx:1", yamlContains: "name: web"}},
		{name: "복제본 nil → 0", setup: func(t *testing.T) (*http.Client, kubeClusterRuntime) { return charStubKube(t, nil) },
			item: noReplicas, want: charWorkloadHead{name: "web", typ: "Deployment", namespace: "default", ready: "3/0", age: "2026-01-01"}},
		{name: "파드 조회 실패에도 디테일 유지", setup: func(t *testing.T) (*http.Client, kubeClusterRuntime) { return charDeadKube() },
			item: dep, want: charWorkloadHead{name: "web", typ: "Deployment", namespace: "default", ready: "1/2", age: "2026-01-01", updated: 1, available: 1,
				selector: map[string]string{"app": "web"}, firstImage: "nginx:1"}},
	}
	for _, tc := range cases {
		client, runtime := tc.setup(t)
		got := buildDeploymentDetail(client, runtime, tc.item) // legacy 1행
		charWantWorkloadDetail(t, tc.name, got, tc.want)
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
func TestCharBuildStatefulSetDetail(t *testing.T) {
	sts := charStatefulSet("db", "default")
	sts.Status.ReadyReplicas = 2
	sts.Spec.Selector.MatchLabels = map[string]string{"app": "db"}
	noReplicas := charStatefulSet("db", "default")
	noReplicas.Spec.Replicas = nil
	noReplicas.Status.ReadyReplicas = 2
	cases := []struct {
		name  string
		setup func(*testing.T) (*http.Client, kubeClusterRuntime)
		item  kubeStatefulSet
		want  charWorkloadHead
	}{
		{name: "기본", setup: func(t *testing.T) (*http.Client, kubeClusterRuntime) { return charStubKube(t, nil) },
			item: sts, want: charWorkloadHead{name: "db", typ: "StatefulSet", namespace: "default", ready: "2/1", age: "2026-01-01",
				selector: map[string]string{"app": "db"}}},
		{name: "복제본 nil → 0", setup: func(t *testing.T) (*http.Client, kubeClusterRuntime) { return charDeadKube() },
			item: noReplicas, want: charWorkloadHead{name: "db", typ: "StatefulSet", namespace: "default", ready: "2/0", age: "2026-01-01"}},
		{name: "셀렉터 매칭 파드만", setup: func(t *testing.T) (*http.Client, kubeClusterRuntime) {
			return charPodsStubFor(t, "default", []kubePod{charPodWithLabels("db-1", "default", map[string]string{"app": "db"}), charPodWithLabels("noise", "default", map[string]string{"app": "api"})})
		}, item: sts, want: charWorkloadHead{name: "db", typ: "StatefulSet", namespace: "default", ready: "2/1", age: "2026-01-01",
			podNames: []string{"db-1"}, selector: map[string]string{"app": "db"}, yamlContains: "name: db"}},
	}
	for _, tc := range cases {
		client, runtime := tc.setup(t)
		got := buildStatefulSetDetail(client, runtime, tc.item) // legacy 1행
		charWantWorkloadDetail(t, tc.name, got, tc.want)
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// Ready는 NumberReady/DesiredNumberScheduled 산식이 현재 계약.
func TestCharBuildDaemonSetDetail(t *testing.T) {
	daemonSet := charDaemonSetStatus("agent", "default", 3, 1, 1, 1)
	daemonSet.Spec.Selector.MatchLabels = map[string]string{"app": "agent"}
	cases := []struct {
		name  string
		setup func(*testing.T) (*http.Client, kubeClusterRuntime)
		item  kubeDaemonSet
		want  charWorkloadHead
	}{
		{name: "셀렉터 매칭 파드만", setup: func(t *testing.T) (*http.Client, kubeClusterRuntime) {
			return charPodsStubFor(t, "default", []kubePod{charPodWithLabels("agent-p1", "default", map[string]string{"app": "agent"}), charPod("noise", "default")})
		}, item: daemonSet, want: charWorkloadHead{name: "agent", typ: "DaemonSet", namespace: "default", ready: "1/3", age: "2026-01-01", updated: 1, available: 1,
			podNames: []string{"agent-p1"}, selector: map[string]string{"app": "agent"}}},
		{name: "파드 조회 실패에도 디테일 유지", setup: func(t *testing.T) (*http.Client, kubeClusterRuntime) { return charDeadKube() },
			item: daemonSet, want: charWorkloadHead{name: "agent", typ: "DaemonSet", namespace: "default", ready: "1/3", age: "2026-01-01", updated: 1, available: 1,
				selector: map[string]string{"app": "agent"}}},
	}
	for _, tc := range cases {
		client, runtime := tc.setup(t)
		got := buildDaemonSetDetail(client, runtime, tc.item) // legacy 1행
		charWantWorkloadDetail(t, tc.name, got, tc.want)
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// completions nil → active+succeeded+failed 합산, 셀렉터 기본 {"job-name": 이름}, 오너 매칭이 현재 계약.
func TestCharBuildJobDetail(t *testing.T) {
	job := charJobStatus("nightly", "default", 2, 1, 1)
	job.Spec.Template.Spec.Containers = append(job.Spec.Template.Spec.Containers, kubeContainer{Name: "w", Image: "busy:1"})
	withCompletions := charJobStatus("b", "default", 1, 0, 0)
	five := 5
	withCompletions.Spec.Completions = &five
	cases := []struct {
		name  string
		setup func(*testing.T) (*http.Client, kubeClusterRuntime)
		item  kubeJob
		want  charWorkloadHead
	}{
		{name: "completions nil → 합산·오너 매칭", setup: func(t *testing.T) (*http.Client, kubeClusterRuntime) {
			return charPodsStubFor(t, "default", []kubePod{charPodWithOwner("job-pod", "default", "Job", "nightly"), charPod("other", "default")})
		}, item: job, want: charWorkloadHead{name: "nightly", typ: "Job", namespace: "default", ready: "2/4", age: "2026-01-01", updated: 1, available: 2,
			podNames: []string{"job-pod"}, selector: map[string]string{"job-name": "nightly"}, firstImage: "busy:1"}},
		{name: "completions 지정", setup: func(t *testing.T) (*http.Client, kubeClusterRuntime) { return charDeadKube() },
			item: withCompletions, want: charWorkloadHead{name: "b", typ: "Job", namespace: "default", ready: "1/5", age: "2026-01-01", available: 1,
				selector: map[string]string{"job-name": "b"}}},
	}
	for _, tc := range cases {
		client, runtime := tc.setup(t)
		got := buildJobDetail(client, runtime, tc.item) // legacy 1행
		charWantWorkloadDetail(t, tc.name, got, tc.want)
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// 파드는 이름 접두사 또는 Job 오너 접두사로 수집, schedule 빈 값 → "-", suspend → "Suspended"가 현재 계약.
func TestCharBuildCronJobDetail(t *testing.T) {
	cronJob := charCronJob("web-a", "default")
	cronJob.Spec.Schedule = "0 * * * *"
	cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers = append(cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers, kubeContainer{Name: "c", Image: "cron:1"})
	suspended := charCronJob("web-a", "default")
	suspend := true
	suspended.Spec.Suspend = &suspend
	cases := []struct {
		name  string
		setup func(*testing.T) (*http.Client, kubeClusterRuntime)
		item  kubeCronJob
		want  charWorkloadHead
	}{
		{name: "이름 접두사 매칭", setup: func(t *testing.T) (*http.Client, kubeClusterRuntime) {
			return charPodsStubFor(t, "default", []kubePod{charPod("web-a-1", "default"), charPod("unrelated", "default")})
		}, item: cronJob, want: charWorkloadHead{name: "web-a", typ: "CronJob", namespace: "default", ready: "Scheduled", age: "2026-01-01",
			podNames: []string{"web-a-1"}, selector: map[string]string{"schedule": "0 * * * *"}, firstImage: "cron:1"}},
		{name: "schedule 빈 값·suspend", setup: func(t *testing.T) (*http.Client, kubeClusterRuntime) { return charDeadKube() },
			item: suspended, want: charWorkloadHead{name: "web-a", typ: "CronJob", namespace: "default", ready: "Suspended", age: "2026-01-01",
				selector: map[string]string{"schedule": "-"}}},
	}
	for _, tc := range cases {
		client, runtime := tc.setup(t)
		got := buildCronJobDetail(client, runtime, tc.item) // legacy 1행
		charWantWorkloadDetail(t, tc.name, got, tc.want)
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// cronjob(대소문자·공백 무시)만 jobTemplate 경로를 쓰는 것이 현재 계약.
func TestCharBuildWorkloadContainerPatchBody(t *testing.T) {
	containers := []map[string]any{{"name": "app"}}
	cases := []struct {
		name         string
		workloadType string
		want         map[string]any
	}{
		{name: "cronjob은 jobTemplate 경로", workloadType: " CronJob ",
			want: map[string]any{"spec": map[string]any{"jobTemplate": map[string]any{"spec": map[string]any{"spec": map[string]any{"containers": containers}}}}}},
		{name: "그 외는 template 경로", workloadType: "deployment",
			want: map[string]any{"spec": map[string]any{"spec": map[string]any{"containers": containers}}}},
	}
	for _, tc := range cases {
		if got := buildWorkloadContainerPatchBody(tc.workloadType, containers); !reflect.DeepEqual(got, tc.want) { // legacy 1행
			t.Errorf("%s: = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// desired 뒤에 기존 env 중 제외분을 "$patch":"delete"로 덧붙이는 순서가 현재 계약.
func TestCharBuildWorkloadEnvPatch(t *testing.T) {
	existing := map[string]any{"env": []any{map[string]any{"name": "OLD", "value": "x"}, map[string]any{"name": "KEEP", "value": "y"}, "junk"}}
	keepFrom := map[string]any{"secretKeyRef": map[string]any{"name": "sec"}}
	items := []model.K8sEnvVarItem{{Name: "NEW", Value: "1"}, {Name: "KEEP", ValueFrom: keepFrom}}
	want := []map[string]any{{"name": "NEW", "value": "1"}, {"name": "KEEP", "valueFrom": keepFrom}, {"name": "OLD", "$patch": "delete"}}
	cases := []struct {
		name     string
		existing map[string]any
		items    []model.K8sEnvVarItem
		want     []map[string]any
		wantErr  string
	}{
		{name: "신규 유지·기존 삭제·비맵 스킵", existing: existing, items: items, want: want},
		{name: "기존 env 없음 → desired만", existing: nil, items: items[:1], want: []map[string]any{{"name": "NEW", "value": "1"}}},
		{name: "빈 이름 → 에러", items: []model.K8sEnvVarItem{{Name: "  "}}, wantErr: "environment variable name is required"},
		{name: "trim 후 중복 → 에러", items: []model.K8sEnvVarItem{{Name: "A"}, {Name: " A "}}, wantErr: "duplicate environment variable: A"},
	}
	for _, tc := range cases {
		got, err := buildWorkloadEnvPatch(tc.existing, tc.items) // legacy 1행
		if charWantErr(t, tc.name, err, tc.wantErr) {
			continue
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: patch = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// 컨테이너 슬라이스를 그대로 spec.template.spec.containers에 넣는 형상이 현재 계약.
func TestCharBuildWorkloadImagePatchBody(t *testing.T) {
	containers := []map[string]any{{"name": "app", "image": "nginx:2"}}
	want := map[string]any{"spec": map[string]any{"template": map[string]any{"spec": map[string]any{"containers": containers}}}}
	if got := buildWorkloadImagePatchBody(containers); !reflect.DeepEqual(got, want) { // legacy 1행
		t.Errorf("body = %v, want %v", got, want)
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// 에러 문언과 "맵 아닌 컨테이너 항목은 조용히 건너뛴다"가 현재 계약.
func TestCharExtractWorkloadContainers(t *testing.T) {
	depWith := map[string]any{"spec": map[string]any{"template": map[string]any{"spec": map[string]any{"containers": []any{map[string]any{"name": "app"}, "junk"}}}}}
	cronWith := map[string]any{"spec": map[string]any{"jobTemplate": map[string]any{"spec": map[string]any{"template": map[string]any{"spec": map[string]any{"containers": []any{map[string]any{"name": "c"}}}}}}}}
	cases := []struct {
		name        string
		resource    map[string]any
		wantNames   []string
		wantErrText string
	}{
		{name: "deployment template", resource: depWith, wantNames: []string{"app"}},
		{name: "cronjob jobTemplate", resource: cronWith, wantNames: []string{"c"}},
		{name: "spec 없음", resource: map[string]any{}, wantErrText: "invalid workload spec"},
		{name: "template 없음", resource: map[string]any{"spec": map[string]any{}}, wantErrText: "invalid workload template"},
		{name: "pod template spec 없음", resource: map[string]any{"spec": map[string]any{"template": map[string]any{}}}, wantErrText: "invalid pod template spec"},
		{name: "cronjob template 없음", resource: map[string]any{"spec": map[string]any{"jobTemplate": map[string]any{}}}, wantErrText: "invalid cronjob template"},
		{name: "cronjob pod template 없음", resource: map[string]any{"spec": map[string]any{"jobTemplate": map[string]any{"spec": map[string]any{}}}}, wantErrText: "invalid cronjob pod template"},
		{name: "containers 없음", resource: map[string]any{"spec": map[string]any{"template": map[string]any{"spec": map[string]any{}}}}, wantErrText: "no containers found in workload"},
		{name: "containers가 배열 아님", resource: map[string]any{"spec": map[string]any{"template": map[string]any{"spec": map[string]any{"containers": "x"}}}}, wantErrText: "no containers found in workload"},
	}
	for _, tc := range cases {
		containers, err := extractWorkloadContainers(tc.resource) // legacy 1행
		if tc.wantErrText != "" {
			if err == nil || err.Error() != tc.wantErrText {
				t.Errorf("%s: err = %v, want %q", tc.name, err, tc.wantErrText)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%s: unexpected err: %v", tc.name, err)
		}
		names := make([]string, 0, len(containers))
		for _, container := range containers {
			name, _ := container["name"].(string)
			names = append(names, name)
		}
		if !reflect.DeepEqual(names, tc.wantNames) {
			t.Errorf("%s: containers = %v, want %v", tc.name, names, tc.wantNames)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// 1000으로 나누어떨어질 때만 cores, 그 외 m 접미("1 cores" 단수 오류 포함)가 현재 계약.
func TestCharFormatCPUMilli(t *testing.T) {
	cases := []struct {
		value int64
		want  string
	}{
		{2000, "2 cores"}, {1500, "1500m"}, {1000, "1 cores"}, {0, "0m"}, {999, "999m"}, {-1000, "-1000m"},
	}
	for _, tc := range cases {
		if got := formatCPUMilli(tc.value); got != tc.want { // legacy 1행
			t.Errorf("%d: = %q, want %q", tc.value, got, tc.want)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// GiB 경계(>= 1024^3만 Gi, 그 외 Mi 반내림)가 현재 계약.
func TestCharFormatMemoryBytes(t *testing.T) {
	gib := int64(1024 * 1024 * 1024)
	cases := []struct {
		value int64
		want  string
	}{
		{2 * gib, "2.0Gi"}, {gib + gib/2, "1.5Gi"}, {128 * 1024 * 1024, "128Mi"}, {0, "0Mi"}, {gib - 1, "1024Mi"},
	}
	for _, tc := range cases {
		if got := formatMemoryBytes(tc.value); got != tc.want { // legacy 1행
			t.Errorf("%d: = %q, want %q", tc.value, got, tc.want)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// 2키 이상 맵은 순회 순서가 비결정적이라 단일 키 맵만 기록한다. name 없는 맵은 소스 타입만.
func TestCharFormatK8sEnvSource(t *testing.T) {
	cases := []struct {
		name      string
		valueFrom map[string]any
		want      string
	}{
		{name: "빈 맵", valueFrom: nil, want: ""},
		{name: "name+key", valueFrom: map[string]any{"configMapKeyRef": map[string]any{"name": "cm", "key": "k"}}, want: "configMapKeyRef: cm/k"},
		{name: "name만", valueFrom: map[string]any{"secretKeyRef": map[string]any{"name": "sec"}}, want: "secretKeyRef: sec"},
		{name: "name 없음 → 소스 타입만", valueFrom: map[string]any{"fieldRef": map[string]any{"fieldPath": "metadata.name"}}, want: "fieldRef"},
		{name: "값이 맵 아님", valueFrom: map[string]any{"resourceFieldRef": "str"}, want: "Provided by Kubernetes reference"},
	}
	for _, tc := range cases {
		if got := formatK8sEnvSource(tc.valueFrom); got != tc.want { // legacy 1행
			t.Errorf("%s: = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// targetPort가 공백이거나 port와 같은 숫자면 base를 그대로 두는 것이 현재 계약.
func TestCharFormatServiceDetailPort(t *testing.T) {
	cases := []struct {
		name       string
		port       int
		nodePort   int
		protocol   string
		targetPort string
		want       string
	}{
		{name: "targetPort 없음", port: 80, protocol: "TCP", want: "80/TCP"},
		{name: "targetPort == port", port: 80, nodePort: 30080, protocol: "TCP", targetPort: "80", want: "80:30080/TCP"},
		{name: "named targetPort", port: 80, nodePort: 30080, targetPort: "web", want: "80:30080/TCP -> web"},
		{name: "공백 targetPort", port: 443, protocol: "TCP", targetPort: "   ", want: "443/TCP"},
	}
	for _, tc := range cases {
		if got := formatServiceDetailPort(tc.port, tc.nodePort, tc.protocol, tc.targetPort); got != tc.want { // legacy 1행
			t.Errorf("%s: = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// nodePort > 0일 때만 "port:nodePort/proto", 빈 protocol → TCP가 현재 계약.
func TestCharFormatServiceListPort(t *testing.T) {
	cases := []struct {
		name     string
		port     int
		nodePort int
		protocol string
		want     string
	}{
		{name: "nodePort 없음", port: 80, protocol: "TCP", want: "80/TCP"},
		{name: "nodePort 있음", port: 80, nodePort: 30080, protocol: "TCP", want: "80:30080/TCP"},
		{name: "빈 protocol → TCP", port: 443, want: "443/TCP"},
		{name: "UDP", port: 53, nodePort: 30053, protocol: "UDP", want: "53:30053/UDP"},
		{name: "protocol 공백 트림", port: 8080, protocol: "  TCP  ", want: "8080/TCP"},
	}
	for _, tc := range cases {
		if got := formatServiceListPort(tc.port, tc.nodePort, tc.protocol); got != tc.want { // legacy 1행
			t.Errorf("%s: = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// 컨테이너 간 합산, cpu·memory 파트를 " / " 결합, 둘 다 없으면 "-"가 현재 계약.
func TestCharFormatWorkloadResourceSummary(t *testing.T) {
	res := func(requests, limits map[string]string) kubeContainer {
		var container kubeContainer
		container.Resources.Requests = requests
		container.Resources.Limits = limits
		return container
	}
	reqCPUMem := map[string]string{"cpu": "500m", "memory": "1Gi"}
	limCPUMem := map[string]string{"cpu": "1", "memory": "2Gi"}
	pair := []kubeContainer{res(reqCPUMem, limCPUMem), res(reqCPUMem, limCPUMem)}
	cases := []struct {
		name       string
		containers []kubeContainer
		requests   bool
		want       string
	}{
		{name: "requests 합산", containers: pair, requests: true, want: "1 cores / 2.0Gi"},
		{name: "limits 합산", containers: pair, requests: false, want: "2 cores / 4.0Gi"},
		{name: "빈 입력 → -", containers: nil, requests: true, want: "-"},
		{name: "cpu만", containers: []kubeContainer{res(map[string]string{"cpu": "500m"}, nil)}, requests: true, want: "500m"},
		{name: "memory만(limits)", containers: []kubeContainer{res(nil, map[string]string{"memory": "2Gi"})}, requests: false, want: "2.0Gi"},
	}
	for _, tc := range cases {
		if got := formatWorkloadResourceSummary(tc.containers, tc.requests); got != tc.want { // legacy 1행
			t.Errorf("%s: = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// 대소문자·공백 정규화와 미지원 타입 에러 문언이 현재 계약.
func TestCharK8sWorkloadResourcePath(t *testing.T) {
	cases := []struct {
		name         string
		namespace    string
		workloadType string
		workloadName string
		want         string
		wantErrText  string
	}{
		{name: "deployment", namespace: "default", workloadType: "Deployment", workloadName: "web", want: "/apis/apps/v1/namespaces/default/deployments/web"},
		{name: "statefulset 소문자+공백", namespace: "prod", workloadType: " statefulset ", workloadName: "db", want: "/apis/apps/v1/namespaces/prod/statefulsets/db"},
		{name: "daemonset", namespace: "ns", workloadType: "DaemonSet", workloadName: "agent", want: "/apis/apps/v1/namespaces/ns/daemonsets/agent"},
		{name: "job", namespace: "ns", workloadType: "job", workloadName: "m", want: "/apis/batch/v1/namespaces/ns/jobs/m"},
		{name: "cronjob 대문자", namespace: "ns", workloadType: "CRONJOB", workloadName: "c", want: "/apis/batch/v1/namespaces/ns/cronjobs/c"},
		{name: "미지원", namespace: "ns", workloadType: "ReplicaSet", workloadName: "r", wantErrText: "unsupported workload type"},
	}
	for _, tc := range cases {
		got, err := k8sWorkloadResourcePath(tc.namespace, tc.workloadType, tc.workloadName) // legacy 1행
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
// @다이제스트 우선 제거, 마지막 콜론이 마지막 슬래시 뒤일 때만 태그 제거가 현재 계약.
func TestCharReplaceImageVersion(t *testing.T) {
	cases := []struct {
		name    string
		image   string
		version string
		want    string
	}{
		{name: "태그 교체", image: "nginx:1.19", version: "2.0", want: "nginx:2.0"},
		{name: "다이제스트 제거", image: "nginx@sha256:abc", version: "2.0", want: "nginx:2.0"},
		{name: "레지스트리 포트 유지", image: "registry:5000/app:1.0", version: "2.0", want: "registry:5000/app:2.0"},
		{name: "태그 없음", image: "nginx", version: "2.0", want: "nginx:2.0"},
		{name: "공백 트림", image: "  nginx:1.19  ", version: "2.0", want: "nginx:2.0"},
		{name: "빈 이미지", image: "   ", version: "2.0", want: ""},
	}
	for _, tc := range cases {
		if got := replaceImageVersion(tc.image, tc.version); got != tc.want { // legacy 1행
			t.Errorf("%s: = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// "-"와 빈값 제외, IP 우선 hostname 폴백, 없으면 "<none>"이 현재 계약.
func TestCharServiceExternalIP(t *testing.T) {
	cases := []struct {
		name        string
		externalIPs []string
		lbIPs       []string
		lbHostnames []string
		want        string
	}{
		{name: "전부 수집", externalIPs: []string{"1.1.1.1", "  ", "-", " 2.2.2.2 "}, lbIPs: []string{"3.3.3.3"}, lbHostnames: []string{"lb.example.com"},
			want: "1.1.1.1, 2.2.2.2, 3.3.3.3, lb.example.com"},
		{name: "없음 → <none>", want: "<none>"},
		{name: "hostname 폴백", lbHostnames: []string{"h"}, want: "h"},
	}
	for _, tc := range cases {
		var svc kubeService
		svc.Spec.ExternalIPs = tc.externalIPs
		for _, ip := range tc.lbIPs {
			svc.Status.LoadBalancer.Ingress = append(svc.Status.LoadBalancer.Ingress, struct {
				IP       string `json:"ip"`
				Hostname string `json:"hostname"`
			}{IP: ip})
		}
		for _, hostname := range tc.lbHostnames {
			svc.Status.LoadBalancer.Ingress = append(svc.Status.LoadBalancer.Ingress, struct {
				IP       string `json:"ip"`
				Hostname string `json:"hostname"`
			}{Hostname: hostname})
		}
		if got := serviceExternalIP(svc); got != tc.want { // legacy 1행
			t.Errorf("%s: = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
func TestCharAnyToString(t *testing.T) {
	cases := []struct {
		name  string
		value any
		want  string
	}{
		{name: "문자열", value: "s", want: "s"},
		{name: "nil", value: nil, want: "<nil>"},
		{name: "정수", value: 42, want: "42"},
		{name: "불", value: true, want: "true"},
		{name: "실수", value: 1.5, want: "1.5"},
		{name: "맵", value: map[string]int{"a": 1}, want: "map[a:1]"},
	}
	for _, tc := range cases {
		if got := anyToString(tc.value); got != tc.want { // legacy 1행
			t.Errorf("%s: = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// Source는 valueFrom 유무로만 갈리고 Value는 원문 보존이 현재 계약.
func TestCharBuildContainerEnvItems(t *testing.T) {
	cmSource := map[string]any{"configMapKeyRef": map[string]any{"name": "cm", "key": "k"}}
	badSource := map[string]any{"resourceFieldRef": "str"}
	cases := []struct {
		name string
		envs []kubeEnvVar
		want []model.K8sEnvVarItem
	}{
		{name: "값·소스 혼합", envs: []kubeEnvVar{{Name: "A", Value: "1"}, {Name: "B", ValueFrom: cmSource}, {Name: "C", ValueFrom: badSource}},
			want: []model.K8sEnvVarItem{
				{Name: "A", Value: "1"},
				{Name: "B", ValueFrom: cmSource, Source: "configMapKeyRef: cm/k"},
				{Name: "C", ValueFrom: badSource, Source: "Provided by Kubernetes reference"}}},
		{name: "빈 입력 → 빈 슬라이스", envs: nil, want: []model.K8sEnvVarItem{}},
	}
	for _, tc := range cases {
		if items := buildContainerEnvItems(tc.envs); !reflect.DeepEqual(items, tc.want) { // legacy 1행
			t.Errorf("%s: items = %+v, want %+v", tc.name, items, tc.want)
		}
	}
}
