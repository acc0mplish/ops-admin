// executor_workload_mock_test — P2-A: workload mutation 테스트의 모의 k8s API
// 서버와 왕복 하네스 fixture(650라인 분할 — mock/fixture와 계약 테스트를 분리).
// 모의 서버는 strategic/merge patch를 A1 선례(executor_test.go rolloutMock)와
// 같은 k8s 시맨틱으로 반영한다: 실제 spec 변경만 generation을 증가시킨다 —
// 재실행 patch의 수렴(J1 멱등·provider_state_convergent/frozen_payload)을
// 실클러스터 없이 실증하는 수단.
package kubernetes

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"ops-admin/backend/internal/infra/contract"
)

// --- 모의 워크로드 API 서버 — main resource GET/PATCH + /scale subresource. ---

type mutationMock struct {
	t   *testing.T
	srv *httptest.Server

	mu         sync.Mutex
	kind       string
	replicas   *int
	generation int64
	containers []map[string]any
	ann        map[string]any
	runningGET int // 남은 running 상태 GET 수(수렴 지연 시나리오 주입면)
	patches    []rolloutPatchRecord
}

func newMutationMock(t *testing.T, kind string, replicas *int, containers []map[string]any) *mutationMock {
	m := &mutationMock{
		t: t, kind: kind, replicas: replicas, containers: containers,
		generation: 1,
		ann:        map[string]any{},
	}
	m.srv = httptest.NewServer(http.HandlerFunc(m.handle))
	t.Cleanup(m.srv.Close)
	return m
}

func (m *mutationMock) basePath() string {
	return "/apis/apps/v1/namespaces/" + execNamespace + "/" + m.kind + "s/" + execWorkloadName
}

func (m *mutationMock) handle(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if want := m.basePath(); r.URL.Path != want && r.URL.Path != want+"/scale" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(m.object())
	case http.MethodPatch:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			m.t.Fatalf("mutationMock: read patch body: %v", err)
		}
		m.patches = append(m.patches, rolloutPatchRecord{Path: r.URL.Path, ContentType: r.Header.Get("Content-Type"), Body: body})
		if len(r.URL.Path) > len(m.basePath()) {
			m.applyScalePatch(body)
		} else {
			m.applyTemplatePatch(body)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(m.object())
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// object — 현재 상태의 리소스 표현. status는 폴 시점에 파생한다: runningGET이
// 남아 있으면 미수렴 관측(구 세대), 아니면 즉시 수렴 관측 — rollout 판정식 3종
// (executor.go rolloutConverged)의 종별 형태를 그대로 모사한다.
func (m *mutationMock) object() map[string]any {
	spec := map[string]any{
		"template": map[string]any{
			"metadata": map[string]any{"annotations": m.ann},
			"spec":     map[string]any{"containers": m.containers},
		},
	}
	if m.replicas != nil {
		spec["replicas"] = *m.replicas
	}
	desired := 1
	if m.replicas != nil {
		desired = *m.replicas
	}
	status := map[string]any{}
	if m.runningGET > 0 {
		m.runningGET--
		switch m.kind {
		case "deployment":
			status["observedGeneration"] = m.generation - 1
			status["replicas"] = desired
			status["readyReplicas"] = desired - 1
			status["availableReplicas"] = desired - 1
			status["updatedReplicas"] = 0
		case "statefulset":
			status["replicas"] = desired
			status["readyReplicas"] = desired
			status["updatedReplicas"] = 0
			status["currentRevision"] = fmt.Sprintf("rev-%d", m.generation-1)
			status["updateRevision"] = fmt.Sprintf("rev-%d", m.generation)
		case "daemonset":
			status["desiredNumberScheduled"] = 3
			status["updatedNumberScheduled"] = 1
			status["numberReady"] = 3
		}
	} else {
		switch m.kind {
		case "deployment":
			status["observedGeneration"] = m.generation
			status["replicas"] = desired
			status["readyReplicas"] = desired
			status["availableReplicas"] = desired
			status["updatedReplicas"] = desired
		case "statefulset":
			status["replicas"] = desired
			status["readyReplicas"] = desired
			status["updatedReplicas"] = desired
			status["currentRevision"] = fmt.Sprintf("rev-%d", m.generation)
			status["updateRevision"] = fmt.Sprintf("rev-%d", m.generation)
		case "daemonset":
			status["desiredNumberScheduled"] = 3
			status["updatedNumberScheduled"] = 3
			status["numberReady"] = 3
		}
	}
	return map[string]any{
		"metadata": map[string]any{"name": execWorkloadName, "namespace": execNamespace, "generation": m.generation},
		"spec":     spec,
		"status":   status,
	}
}

// applyScalePatch — /scale merge-patch의 k8s 시맨틱: replicas 변경만이 generation을
// 증가시킨다(A1 선례).
func (m *mutationMock) applyScalePatch(body []byte) {
	var patch struct {
		Spec struct {
			Replicas *int `json:"replicas"`
		} `json:"spec"`
	}
	if err := json.Unmarshal(body, &patch); err != nil {
		m.t.Fatalf("mutationMock: decode scale patch: %v", err)
	}
	if patch.Spec.Replicas != nil && (m.replicas == nil || *m.replicas != *patch.Spec.Replicas) {
		m.replicas = patch.Spec.Replicas
		m.generation++
	}
}

// applyTemplatePatch — strategic-merge-patch의 k8s 시맨틱: 어노테이션 값 변경,
// replicas 변경, containers 병합(merge key name — image·resources·imagePullPolicy
// 치환, env upsert+$patch:delete) 중 실제 변경만 generation을 증가시킨다.
func (m *mutationMock) applyTemplatePatch(body []byte) {
	var patch struct {
		Spec struct {
			Replicas *int `json:"replicas"`
			Template struct {
				Metadata struct {
					Annotations map[string]string `json:"annotations"`
				} `json:"metadata"`
				Spec struct {
					Containers []map[string]any `json:"containers"`
				} `json:"spec"`
			} `json:"template"`
		} `json:"spec"`
	}
	if err := json.Unmarshal(body, &patch); err != nil {
		m.t.Fatalf("mutationMock: decode template patch: %v", err)
	}
	changed := false
	for k, v := range patch.Spec.Template.Metadata.Annotations {
		if m.ann[k] != v {
			m.ann[k] = v
			changed = true
		}
	}
	if patch.Spec.Replicas != nil && (m.replicas == nil || *m.replicas != *patch.Spec.Replicas) {
		m.replicas = patch.Spec.Replicas
		changed = true
	}
	for _, pc := range patch.Spec.Template.Spec.Containers {
		name, _ := pc["name"].(string)
		for i, existing := range m.containers {
			if existing["name"] != name {
				continue
			}
			if image, ok := pc["image"].(string); ok && image != "" && existing["image"] != image {
				existing["image"] = image
				changed = true
			}
			if policy, ok := pc["imagePullPolicy"].(string); ok && policy != "" && existing["imagePullPolicy"] != policy {
				existing["imagePullPolicy"] = policy
				changed = true
			}
			if raw, ok := pc["resources"]; ok {
				if !sameJSON(existing["resources"], raw) {
					existing["resources"] = raw
					changed = true
				}
			}
			if raw, ok := pc["env"]; ok {
				merged := mergeEnv(existing["env"], raw)
				if !sameJSON(existing["env"], merged) {
					existing["env"] = merged
					changed = true
				}
			}
			m.containers[i] = existing
		}
	}
	if changed {
		m.generation++
	}
}

// mergeEnv — env 병합(merge key name): $patch:delete는 제거, 나머지는 upsert.
func mergeEnv(existingRaw, patchRaw any) []map[string]any {
	existing := mapList(existingRaw)
	merged := make([]map[string]any, 0, len(existing)+4)
	for _, env := range existing {
		merged = append(merged, env)
	}
	for _, raw := range mapList(patchRaw) {
		name, _ := raw["name"].(string)
		if directive, _ := raw["$patch"].(string); directive == "delete" {
			kept := merged[:0]
			for _, env := range merged {
				if env["name"] != name {
					kept = append(kept, env)
				}
			}
			merged = kept
			continue
		}
		replaced := false
		for i, env := range merged {
			if env["name"] == name {
				merged[i] = raw
				replaced = true
			}
		}
		if !replaced {
			merged = append(merged, raw)
		}
	}
	return merged
}

func mapList(raw any) []map[string]any {
	switch v := raw.(type) {
	case []map[string]any:
		return v
	case []any:
		out := make([]map[string]any, 0, len(v))
		for _, item := range v {
			if m, ok := item.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out
	}
	return nil
}

// sameJSON — 관측값과 patch값의 동질 판정(정규화 비교 — 맵 키 순서 무관).
func sameJSON(a, b any) bool {
	ab, aerr := json.Marshal(a)
	bb, berr := json.Marshal(b)
	if aerr != nil || berr != nil {
		return false
	}
	return string(ab) == string(bb)
}

func (m *mutationMock) patchCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.patches)
}

func (m *mutationMock) generationOf() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.generation
}

// --- Fixture — contracttest.OperationFixture의 kubernetes 구현(오퍼레이션별). ---

type mutationFixture struct {
	t       *testing.T
	mock    *mutationMock
	op      string
	payload contract.JSONMap
	bad     []contract.OperationRequest
}

func (f *mutationFixture) Request() contract.OperationRequest {
	return contract.OperationRequest{
		OperationName: f.op,
		ResourceURN:   "urn:k8s:3:workload:" + execNamespace + "/" + f.mock.kind + "/" + execWorkloadName,
		Payload:       f.payload,
		Connection:    f.Connection(),
	}
}

func (f *mutationFixture) BadRequests() []contract.OperationRequest { return f.bad }

func (f *mutationFixture) Mutations() int { return f.mock.patchCount() }

// AllowedDetailKeys — rollout 4종이 공유하는 Poll detail 5키(§10.2 —
// rolloutResultRedaction과 1:1).
func (f *mutationFixture) AllowedDetailKeys() []string {
	return []string{"generation", "restartedAt", "readyReplicas", "updated", "serverURL"}
}

func (f *mutationFixture) SecretMarker() string { return kubeconfigTokenStr }

func (f *mutationFixture) Connection() contract.ConnectionView {
	return contract.ConnectionView{
		UID:          execConnUID,
		ProviderType: ProviderName,
		Material:     map[string]string{contract.CredentialPurposeOperations: kubeconfigFor(f.mock.srv.URL)},
	}
}

// newWorkloadFixtures — 왕복 하네스에 공급할 4종(restart 포함) fixture. 종 배치로
// rollout 판정식 3종이 모두 왕복된다: restart=deployment·scale=statefulset·
// image=deployment·resources=daemonset.
func newWorkloadFixtures(t *testing.T) map[string]*mutationFixture {
	appContainer := func() map[string]any {
		return map[string]any{
			"name":  "app",
			"image": "registry.local/app:1.0.0",
			"env": []any{
				map[string]any{"name": "KEEP", "value": "1"},
				map[string]any{"name": "DROP", "value": "2"},
			},
			"resources": map[string]any{"requests": map[string]any{"cpu": "100m"}},
		}
	}
	replicas := 3
	return map[string]*mutationFixture{
		RestartOperationName: {
			t: t, op: RestartOperationName,
			mock:    newMutationMock(t, "deployment", &replicas, []map[string]any{appContainer()}),
			payload: contract.JSONMap{"restartedAt": frozenRestartedAtValue},
			bad: []contract.OperationRequest{
				{OperationName: RestartOperationName, ResourceURN: "urn:k8s:3:workload:" + execNamespace + "/deployment/" + execWorkloadName, Payload: contract.JSONMap{}},
				{OperationName: RestartOperationName, ResourceURN: "urn:k8s:3:workload:" + execNamespace + "/deployment/" + execWorkloadName, Payload: contract.JSONMap{"restartedAt": 1759207323}},
			},
		},
		ScaleOperationName: {
			t: t, op: ScaleOperationName,
			mock:    newMutationMock(t, "statefulset", &replicas, []map[string]any{appContainer()}),
			payload: contract.JSONMap{"replicas": 5},
			bad: []contract.OperationRequest{
				{OperationName: ScaleOperationName, ResourceURN: "urn:k8s:3:workload:" + execNamespace + "/statefulset/" + execWorkloadName, Payload: contract.JSONMap{}},
				{OperationName: ScaleOperationName, ResourceURN: "urn:k8s:3:workload:" + execNamespace + "/statefulset/" + execWorkloadName, Payload: contract.JSONMap{"replicas": -1}},
				{OperationName: ScaleOperationName, ResourceURN: "urn:k8s:3:workload:" + execNamespace + "/statefulset/" + execWorkloadName, Payload: contract.JSONMap{"replicas": "3"}},
				{OperationName: ScaleOperationName, ResourceURN: "urn:k8s:3:workload:" + execNamespace + "/daemonset/" + execWorkloadName, Payload: contract.JSONMap{"replicas": 2}},
			},
		},
		ImageUpdateOperationName: {
			t: t, op: ImageUpdateOperationName,
			mock:    newMutationMock(t, "deployment", &replicas, []map[string]any{appContainer()}),
			payload: contract.JSONMap{"version": "2.0.0"},
			bad: []contract.OperationRequest{
				{OperationName: ImageUpdateOperationName, ResourceURN: "urn:k8s:3:workload:" + execNamespace + "/deployment/" + execWorkloadName, Payload: contract.JSONMap{}},
				{OperationName: ImageUpdateOperationName, ResourceURN: "urn:k8s:3:workload:" + execNamespace + "/deployment/" + execWorkloadName, Payload: contract.JSONMap{"version": "  "}},
			},
		},
		ResourcesUpdateOperationName: {
			t: t, op: ResourcesUpdateOperationName,
			mock:    newMutationMock(t, "daemonset", nil, []map[string]any{appContainer()}),
			payload: contract.JSONMap{"containers": []any{map[string]any{
				"name":            "app",
				"requests":        map[string]any{"cpu": "250m", "memory": "256Mi"},
				"limits":          map[string]any{"memory": "512Mi"},
				"imagePullPolicy": "IfNotPresent",
				"env": []any{
					map[string]any{"name": "KEEP", "value": "1new"},
					map[string]any{"name": "NEW", "valueFrom": map[string]any{"secretKeyRef": map[string]any{"name": "s", "key": "k"}}},
				},
			}}},
			bad: []contract.OperationRequest{
				{OperationName: ResourcesUpdateOperationName, ResourceURN: "urn:k8s:3:workload:" + execNamespace + "/daemonset/" + execWorkloadName, Payload: contract.JSONMap{}},
				{OperationName: ResourcesUpdateOperationName, ResourceURN: "urn:k8s:3:workload:" + execNamespace + "/daemonset/" + execWorkloadName, Payload: contract.JSONMap{"containers": []any{map[string]any{"name": "app", "imagePullPolicy": "Sometimes"}}}},
				{OperationName: ResourcesUpdateOperationName, ResourceURN: "urn:k8s:3:workload:" + execNamespace + "/daemonset/" + execWorkloadName, Payload: contract.JSONMap{"containers": []any{map[string]any{"name": "ghost"}}}},
				{OperationName: ResourcesUpdateOperationName, ResourceURN: "urn:k8s:3:workload:" + execNamespace + "/daemonset/" + execWorkloadName, Payload: contract.JSONMap{"containers": []any{map[string]any{"name": "app", "requests": map[string]any{"gpu": "1"}}}}},
				{OperationName: ResourcesUpdateOperationName, ResourceURN: "urn:k8s:3:workload:" + execNamespace + "/daemonset/" + execWorkloadName, Payload: contract.JSONMap{"containers": []any{map[string]any{"name": "app", "env": []any{map[string]any{"name": "A"}, map[string]any{"name": "A"}}}}}},
			},
		},
	}
}
