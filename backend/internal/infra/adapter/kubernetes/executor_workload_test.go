// executor_workload_test — P2-A: workload mutation 3종(scale·image_update·
// resources_update)의 Execute 계약 테스트 + 왕복 하네스 소비
// (TestOperationContractRoundtrip — G-P2b deliverable, P2-B~D에서 10종으로 확장).
// 모의 서버·fixture는 executor_workload_mock_test.go가 담당한다.
package kubernetes

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/contracttest"
)

// TestOperationContractRoundtrip — G-P2b deliverable(계획 r3 §J-P1-6·§7): 오퍼레이션
// Execute→Poll 왕복을 공용 하네스(contracttest.RunOperationRoundtrip)로 운영한다.
// P2-A 4종(restart + workload 3종 — image fixture는 running 1경유로 수렴해 폴
// 순회 경로도 왕복한다)과 P2-B state-convergent 2종을 같은 표로 운영한다.
// P2-C~D가 나머지 4종을 확장한다(10종 왕복).
func TestOperationContractRoundtrip(t *testing.T) {
	adapter := NewAdapter()
	for _, op := range []string{
		RestartOperationName,
		ScaleOperationName,
		ImageUpdateOperationName,
		ResourcesUpdateOperationName,
		NodeLabelsUpdateOperationName,
		ServiceUpdateOperationName,
	} {
		t.Run(op, func(t *testing.T) {
			if f, ok := newWorkloadFixtures(t)[op]; ok {
				if op == ImageUpdateOperationName {
					f.mock.runningGET = 1
				}
				contracttest.RunOperationRoundtrip(t, adapter, f)
				return
			}
			contracttest.RunOperationRoundtrip(t, adapter, newConfigFixtures(t)[op])
		})
	}
}

// --- Execute — scale 계약. ---

// TestExecuteScalePatchesScaleSubresource — /scale merge-patch 1회: 경로·
// Content-Type·본문이 v1 ScaleK8sWorkload와 동일하고, handle은 관측 GET의
// generation을 expectGeneration으로 인코딩한다. 동일 payload 재실행은
// byte-identical patch + generation 무증가(provider_state_convergent).
func TestExecuteScalePatchesScaleSubresource(t *testing.T) {
	f := newWorkloadFixtures(t)[ScaleOperationName]

	handle, err := NewAdapter().Execute(context.Background(), f.Request())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if n := f.mock.patchCount(); n != 1 {
		t.Fatalf("patch count = %d, want 1", n)
	}
	p := f.mock.patches[0]
	if want := f.mock.basePath() + "/scale"; p.Path != want {
		t.Errorf("patch path = %q, want %q", p.Path, want)
	}
	if want := "application/merge-patch+json"; p.ContentType != want {
		t.Errorf("patch Content-Type = %q, want %q", p.ContentType, want)
	}
	var body map[string]any
	if err := json.Unmarshal(p.Body, &body); err != nil {
		t.Fatalf("patch body decode: %v\nbody: %s", err, p.Body)
	}
	assertKeySet(t, "scale patch top", body, "spec")
	assertKeySet(t, "scale patch spec", body["spec"].(map[string]any), "replicas")

	// 재실행 — byte-identical patch, generation 무증가, 동일 handle.
	before := string(f.mock.patches[0].Body)
	first := handle
	second, err := NewAdapter().Execute(context.Background(), f.Request())
	if err != nil {
		t.Fatalf("second Execute (convergent replay): %v", err)
	}
	if before != string(f.mock.patches[1].Body) {
		t.Errorf("replay patch differs:\nfirst:  %s\nsecond: %s", before, f.mock.patches[1].Body)
	}
	if first.ProviderRef != second.ProviderRef {
		t.Errorf("replay handle changed: %q → %q", first.ProviderRef, second.ProviderRef)
	}
	if got := f.mock.generationOf(); got != 2 {
		t.Errorf("generation after replay = %d, want 2 (identical scale must not bump generation)", got)
	}
	wantRef := strings.Join([]string{"rollout", execConnUID, execNamespace, "statefulset", execWorkloadName, "2"}, "|")
	if second.ProviderRef != wantRef {
		t.Errorf("handle ProviderRef = %q, want %q", second.ProviderRef, wantRef)
	}
}

// TestExecuteScaleValidationRejects — 검증 실패는 patch 0발행. daemonset 거부는
// v1 문언("only deployment and statefulset support scaling")과 동일하다.
func TestExecuteScaleValidationRejects(t *testing.T) {
	f := newWorkloadFixtures(t)[ScaleOperationName]
	for _, bad := range f.BadRequests() {
		_, err := NewAdapter().Execute(context.Background(), bad)
		if err == nil {
			t.Fatalf("scale request %v succeeded, want error", bad.Payload)
		}
	}
	if n := f.mock.patchCount(); n != 0 {
		t.Errorf("validation failures issued %d patch(es), want 0", n)
	}
	if _, err := NewAdapter().Execute(context.Background(), contract.OperationRequest{
		OperationName: ScaleOperationName,
		ResourceURN:   "urn:k8s:3:workload:" + execNamespace + "/daemonset/" + execWorkloadName,
		Payload:       contract.JSONMap{"replicas": 2},
		Connection:    f.Connection(),
	}); err == nil || !strings.Contains(err.Error(), "only deployment and statefulset support scaling") {
		t.Errorf("daemonset scale error = %v, want the v1 wording", err)
	}
}

// --- Execute — image_update 계약. ---

// TestExecuteImageUpdatePatchesTemplateImages — GET→strategic-merge-patch:
// replaceImageVersion 경험식(digest·tag 절단 후 :version 부착)이 patch 엔트리에
// 그대로 나타난다. 재실행은 byte-identical + generation 무증가.
func TestExecuteImageUpdatePatchesTemplateImages(t *testing.T) {
	f := newWorkloadFixtures(t)[ImageUpdateOperationName]

	handle, err := NewAdapter().Execute(context.Background(), f.Request())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if n := f.mock.patchCount(); n != 1 {
		t.Fatalf("patch count = %d, want 1", n)
	}
	p := f.mock.patches[0]
	if want := f.mock.basePath(); p.Path != want {
		t.Errorf("patch path = %q, want %q", p.Path, want)
	}
	if want := "application/strategic-merge-patch+json"; p.ContentType != want {
		t.Errorf("patch Content-Type = %q, want %q", p.ContentType, want)
	}
	var body struct {
		Spec struct {
			Template struct {
				Spec struct {
					Containers []struct {
						Name  string `json:"name"`
						Image string `json:"image"`
					} `json:"containers"`
				} `json:"spec"`
			} `json:"template"`
		} `json:"spec"`
	}
	if err := json.Unmarshal(p.Body, &body); err != nil {
		t.Fatalf("patch body decode: %v\nbody: %s", err, p.Body)
	}
	if len(body.Spec.Template.Spec.Containers) != 1 {
		t.Fatalf("patched containers = %d, want 1 (single-container seed)", len(body.Spec.Template.Spec.Containers))
	}
	got := body.Spec.Template.Spec.Containers[0]
	if got.Name != "app" || got.Image != "registry.local/app:2.0.0" {
		t.Errorf("patched container = {%s %s}, want {app registry.local/app:2.0.0}", got.Name, got.Image)
	}
	if f.mock.generationOf() != 2 {
		t.Errorf("generation after image patch = %d, want 2", f.mock.generationOf())
	}
	wantRef := strings.Join([]string{"rollout", execConnUID, execNamespace, "deployment", execWorkloadName, "2"}, "|")
	if handle.ProviderRef != wantRef {
		t.Errorf("handle ProviderRef = %q, want %q", handle.ProviderRef, wantRef)
	}

	// 재실행 — 적용된 tag 절단으로 byte-identical, generation 무증가.
	second, err := NewAdapter().Execute(context.Background(), f.Request())
	if err != nil {
		t.Fatalf("second Execute (frozen payload replay): %v", err)
	}
	if string(f.mock.patches[0].Body) != string(f.mock.patches[1].Body) {
		t.Errorf("replay patch differs:\nfirst:  %s\nsecond: %s", f.mock.patches[0].Body, f.mock.patches[1].Body)
	}
	if second.ProviderRef != handle.ProviderRef {
		t.Errorf("replay handle changed: %q → %q", handle.ProviderRef, second.ProviderRef)
	}
	if got := f.mock.generationOf(); got != 2 {
		t.Errorf("generation after replay = %d, want 2 (identical patch must not bump generation)", got)
	}
}

// TestExecuteImageUpdateValidationRejects — version 부재·공백과 컨테이너 0 워크로드
// ("no containers found in selected workload" — v1 문언)를 거부한다.
func TestExecuteImageUpdateValidationRejects(t *testing.T) {
	f := newWorkloadFixtures(t)[ImageUpdateOperationName]
	for _, bad := range f.BadRequests() {
		_, err := NewAdapter().Execute(context.Background(), bad)
		if err == nil {
			t.Fatalf("image request %v succeeded, want error", bad.Payload)
		}
	}
	if n := f.mock.patchCount(); n != 0 {
		t.Errorf("validation failures issued %d patch(es), want 0", n)
	}
	empty := newMutationMock(t, "deployment", nil, []map[string]any{})
	emptyFixture := &mutationFixture{t: t, mock: empty, op: ImageUpdateOperationName, payload: contract.JSONMap{"version": "2.0.0"}}
	_, err := NewAdapter().Execute(context.Background(), emptyFixture.Request())
	if err == nil || !strings.Contains(err.Error(), "no containers found in selected workload") {
		t.Errorf("empty template error = %v, want the v1 wording", err)
	}
	if n := empty.patchCount(); n != 0 {
		t.Errorf("no-container rejection issued %d patch(es), want 0", n)
	}
}

// --- Execute — resources_update 계약. ---

// TestExecuteResourcesUpdatePatchesContainers — requests·limits·pull policy는
// 치환, env는 v1 병합(KEEP 갱신·NEW 추가·DROP $patch:delete). 재실행은 동일 상태의
// 재적용 — 실변경 없음(generation 무증가).
func TestExecuteResourcesUpdatePatchesContainers(t *testing.T) {
	f := newWorkloadFixtures(t)[ResourcesUpdateOperationName]

	handle, err := NewAdapter().Execute(context.Background(), f.Request())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if n := f.mock.patchCount(); n != 1 {
		t.Fatalf("patch count = %d, want 1", n)
	}
	p := f.mock.patches[0]
	var body map[string]any
	if err := json.Unmarshal(p.Body, &body); err != nil {
		t.Fatalf("patch body decode: %v\nbody: %s", err, p.Body)
	}
	containers := body["spec"].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)["containers"].([]any)
	if len(containers) != 1 {
		t.Fatalf("patched containers = %d, want 1", len(containers))
	}
	c := containers[0].(map[string]any)
	if c["name"] != "app" {
		t.Errorf("patched container name = %v, want app", c["name"])
	}
	if c["imagePullPolicy"] != "IfNotPresent" {
		t.Errorf("patched imagePullPolicy = %v, want IfNotPresent", c["imagePullPolicy"])
	}
	resources := c["resources"].(map[string]any)
	if !sameJSON(resources["requests"], map[string]any{"cpu": "250m", "memory": "256Mi"}) {
		t.Errorf("patched requests = %v, want cpu 250m·memory 256Mi", resources["requests"])
	}
	if !sameJSON(resources["limits"], map[string]any{"memory": "512Mi"}) {
		t.Errorf("patched limits = %v, want memory 512Mi", resources["limits"])
	}
	env := c["env"].([]any)
	wantEnv := []string{"KEEP", "NEW", "DROP"}
	if len(env) != len(wantEnv) {
		t.Fatalf("env patch entries = %v, want %v", env, wantEnv)
	}
	for i, w := range wantEnv {
		entry := env[i].(map[string]any)
		if entry["name"] != w {
			t.Errorf("env entry %d name = %v, want %s", i, entry["name"], w)
		}
	}
	if env[2].(map[string]any)["$patch"] != "delete" {
		t.Errorf("DROP entry = %v, want $patch: delete", env[2])
	}
	if f.mock.generationOf() != 2 {
		t.Errorf("generation after resources patch = %d, want 2", f.mock.generationOf())
	}
	wantRef := strings.Join([]string{"rollout", execConnUID, execNamespace, "daemonset", execWorkloadName, "2"}, "|")
	if handle.ProviderRef != wantRef {
		t.Errorf("handle ProviderRef = %q, want %q", handle.ProviderRef, wantRef)
	}

	// 재실행 — 동일 상태의 재적용: 첫 재실행만 delete 대상 소멸로 본문이 줄고,
	// 그 이후 재실행은 byte-identical이다. 어느 재실행도 실변경이 없어 generation
	// 무증가(재롤아웃 없음 — provider_frozen_payload의 수렴 계약).
	second, err := NewAdapter().Execute(context.Background(), f.Request())
	if err != nil {
		t.Fatalf("second Execute (frozen payload replay): %v", err)
	}
	if string(f.mock.patches[0].Body) == string(f.mock.patches[1].Body) {
		t.Error("replay patch equals the first — the delete entry should have been retired by the applied env state")
	}
	third, err := NewAdapter().Execute(context.Background(), f.Request())
	if err != nil {
		t.Fatalf("third Execute (stable replay): %v", err)
	}
	if string(f.mock.patches[1].Body) != string(f.mock.patches[2].Body) {
		t.Errorf("stable replay differs:\nsecond: %s\nthird:  %s", f.mock.patches[1].Body, f.mock.patches[2].Body)
	}
	if second.ProviderRef != handle.ProviderRef || third.ProviderRef != handle.ProviderRef {
		t.Errorf("replay handle changed: %q → %q → %q (generation must not advance)", handle.ProviderRef, second.ProviderRef, third.ProviderRef)
	}
	if got := f.mock.generationOf(); got != 2 {
		t.Errorf("generation after replays = %d, want 2 (no double rollout)", got)
	}
}

// TestExecuteResourcesUpdateValidationRejects — 형태 검증은 HTTP 전, 미지
// 컨테이너는 GET 후 patch 전에 거부된다 — 전 케이스 patch 0발행.
func TestExecuteResourcesUpdateValidationRejects(t *testing.T) {
	f := newWorkloadFixtures(t)[ResourcesUpdateOperationName]
	for _, bad := range f.BadRequests() {
		_, err := NewAdapter().Execute(context.Background(), bad)
		if err == nil {
			t.Fatalf("resources request %v succeeded, want error", bad.Payload)
		}
	}
	if n := f.mock.patchCount(); n != 0 {
		t.Errorf("validation failures issued %d patch(es), want 0", n)
	}
}

// --- Poll — mutation 발행 handle의 공유 계약. ---

// TestPollMutationHandleSharedContract — 3종 mutation이 발행한 handle은 restart와
// 같은 Poll(rollout 수렴)을 통과하고, 성공 detail은 공통 5키를 실는다. restartedAt는
// restart 외 op에서 부재 관측값(공백)이다(§J-P1-6 — 기존 동작 무해).
func TestPollMutationHandleSharedContract(t *testing.T) {
	for _, op := range []string{ScaleOperationName, ImageUpdateOperationName, ResourcesUpdateOperationName} {
		t.Run(op, func(t *testing.T) {
			f := newWorkloadFixtures(t)[op]
			adapter := NewAdapter()
			handle, err := adapter.Execute(context.Background(), f.Request())
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			for {
				status, pollErr := adapter.Poll(context.Background(), contract.PollRequest{Handle: handle, Connection: f.Connection()})
				if pollErr != nil {
					t.Fatalf("Poll: %v", pollErr)
				}
				switch status.State {
				case contract.OperationStateRunning:
					if status.Detail != nil {
						t.Fatalf("running poll carried detail %v", status.Detail)
					}
				case contract.OperationStateSucceeded:
					assertDetailKeys(t, status.Detail)
					assertServerURL(t, status.Detail)
					if got := status.Detail["restartedAt"]; got != "" {
						t.Errorf("non-restart op detail restartedAt = %q, want the absent observation \"\"", got)
					}
					return
				default:
					t.Fatalf("unexpected poll state %q", status.State)
				}
			}
		})
	}
}
