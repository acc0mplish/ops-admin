// executor_workload.go — P2-A: workload mutation 3종(k8s.workload.scale·
// image_update·resources_update)의 OperationExecutor leg(계획 r3 §J-P1-6 확정표
// — k8s_workload.go의 v1 원천 3라인). restart leg는 executor.go가 담당하고 4종이
// rollout handle·Poll(rolloutConverged)을 공유한다 — handle 형식은 restart 선례
// 그대로(6세그먼트 rollout|connUID|ns|kind|name|expectGeneration)이므로 Poll은
// 무변경이다.
//
//   - scale: /scale 서브리소스에 merge-patch 1회(v1 ScaleK8sWorkload와 동일 방식)
//     → 메인 리소스 GET 1회로 expectGeneration을 확정한다(Scale 응답의 metadata는
//     부모 generation을 보증하지 않는다).
//   - image_update·resources_update: GET 1회(현행 template 관측) → strategic-
//     merge-patch 1회 — v1 buildWorkloadImagePatchBody·buildWorkloadContainerPatchBody
//     의 경험식을 재구현한다(§J-P1-5 — 쓰기 경험식은 Z 동치가 아니라 본 왕복
//     contracttest가 대체 판정한다).
//
// payload 검증은 실행기 자체(frozenRestartedAt 선례 — §J-P1-6). plan 응답의
// restartedAt·resourceRevision 키가 payload에 붙어 있어도 무시한다(기존 동작
// 무해 — payloadDecode). 멱등 근거: image patch는 현행 관측 재계산으로
// byte-identical이 되고, resources patch는 동일 상태 재적용(실변경 없음)이 된다 —
// 어느 쪽도 재실행 시 generation이 무증가다(A1 선례 — 재롤아웃 없음).
package kubernetes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"

	"ops-admin/backend/internal/infra/contract"
)

// operation names — compose.go opdef 등록과 dispatch(executor.go)가 공유하는
// 단일 원천(§J-P1-6 — descriptors are code).
const (
	ScaleOperationName           = "k8s.workload.scale"
	ImageUpdateOperationName     = "k8s.workload.image_update"
	ResourcesUpdateOperationName = "k8s.workload.resources_update"
)

// executionClient — workload mutation 3종이 공유하는 실행 자격 리프. UID 가드는
// restart leg(executor.go)와 동일 문구다: handle이 자기서술하는 connUID의 원천
// 이므로 UID 부재는 patch 전에 즉시 거부된다(J12).
func (a *Adapter) executionClient(req contract.OperationRequest) (*k8sClient, error) {
	if req.Connection.UID == "" {
		return nil, fmt.Errorf("kubernetes: execution connection carries no UID — the rollout handle must encode the connection it targets (J12)")
	}
	return a.buildExecutorClient(req.Connection)
}

// payloadDecode — contract.JSONMap을 typed shape로 정규화한다. 엔진은 payload를
// task 레코드(JSON)로 스냅숏하므로 프로그램 호출도 JSON 형태로 정규화되고, 수치는
// float64로 통일된다. 알 수 없는 키는 무시한다 — plan 응답의 restartedAt 키가 타
// op payload에 붙는 것은 기존 동작(무해, §J-P1-6)이고 실행기는 필요 키만 검증한다.
func payloadDecode(payload contract.JSONMap, out any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("kubernetes: payload encode: %w", err)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("kubernetes: payload shape rejected: %w", err)
	}
	return nil
}

// --- k8s.workload.scale (v1 k8s_workload.go:61 ScaleK8sWorkload). ---

// scalePatchBody — /scale 서브리소스 본문. 구조체 → json.Marshal의 필드 순서
// 고정 — 동일 payload 재실행은 byte-identical 본문이다(J1 전송 계약 선례).
type scalePatchBody struct {
	Spec struct {
		Replicas int `json:"replicas"`
	} `json:"spec"`
}

func newScalePatch(replicas int) scalePatchBody {
	var p scalePatchBody
	p.Spec.Replicas = replicas
	return p
}

// scaleReplicas — Payload["replicas"] 검증: 정수 ≥ 0(v1 문언 그대로). 값은 plan
// 요청이 동결한 것으로 execute는 재수집 없이 그대로 patch한다(provider_state_
// convergent — 동일 목표 replicas로의 재수렴은 무해).
func scaleReplicas(payload contract.JSONMap) (int, error) {
	var spec struct {
		Replicas *float64 `json:"replicas"`
	}
	if err := payloadDecode(payload, &spec); err != nil {
		return 0, err
	}
	if spec.Replicas == nil {
		return 0, fmt.Errorf("kubernetes: payload carries no %q — the plan request must freeze the replica count", "replicas")
	}
	value := *spec.Replicas
	if value != math.Trunc(value) || math.IsInf(value, 0) {
		return 0, fmt.Errorf("kubernetes: payload %q must be a whole number, got %v", "replicas", value)
	}
	if value < 0 {
		return 0, errors.New("kubernetes: replicas must be greater than or equal to 0")
	}
	if value > math.MaxInt32 {
		return 0, fmt.Errorf("kubernetes: payload %q exceeds the kubernetes replicas range (int32), got %v", "replicas", value)
	}
	return int(value), nil
}

// executeScale — /scale 서브리소스 merge-patch(v1 ScaleK8sWorkload와 동일 patch
// 방식) → 메인 리소스 GET으로 expectGeneration 확정 → rollout handle. Scale
// 서브리소스 응답은 부모 generation을 실지 않는 k8s ObjectMeta(name/namespace/
// resourceVersion만 보증)라 관측 GET이 필요하다 — restart가 patch 응답에서 읽는
// 값과 동일 계약(표본 리소스만 다르다).
func (a *Adapter) executeScale(ctx context.Context, req contract.OperationRequest) (contract.OperationHandle, error) {
	target, err := parseWorkloadURN(req.ResourceURN)
	if err != nil {
		return contract.OperationHandle{}, err
	}
	replicas, err := scaleReplicas(req.Payload)
	if err != nil {
		return contract.OperationHandle{}, err
	}
	// v1 문언 그대로 — daemonset은 scale 면 밖이다(restart의 3종과 다른 점).
	if target.Kind == "daemonset" {
		return contract.OperationHandle{}, fmt.Errorf("kubernetes: only deployment and statefulset support scaling (got %s)", target.Kind)
	}
	client, err := a.executionClient(req)
	if err != nil {
		return contract.OperationHandle{}, err
	}

	if err := client.doJSON(ctx, http.MethodPatch, target.apiPath()+"/scale", nil, "application/merge-patch+json", newScalePatch(replicas), "execute", nil); err != nil {
		return contract.OperationHandle{}, err
	}
	var obj workloadRolloutStatus
	if err := client.getJSONOp(ctx, target.apiPath(), nil, "execute", &obj); err != nil {
		return contract.OperationHandle{}, err
	}
	if obj.Metadata.Generation <= 0 {
		return contract.OperationHandle{}, fmt.Errorf("kubernetes: observation for %s carried no metadata.generation — cannot encode the rollout handle", target.apiPath())
	}
	return contract.OperationHandle{
		ProviderRef: encodeRolloutRef(req.Connection.UID, target, obj.Metadata.Generation),
	}, nil
}

// --- image_update·resources_update가 공유하는 template 관측·patch 본문. ---

// workloadTemplateObservation — GET 1회로 읽는 pod template 관측. patch 대상
// 컨테이너를 현행 관측에서 확정하는 데 필요한 최소면(v1 extractWorkloadContainers
// — 워크로드 URN 면은 deployment/statefulset/daemonset이라 jobTemplate 중첩은
// 없다).
type workloadTemplateObservation struct {
	Metadata struct {
		Generation int64 `json:"generation"`
	} `json:"metadata"`
	Spec struct {
		Template struct {
			Spec struct {
				Containers []workloadContainerObservation `json:"containers"`
			} `json:"spec"`
		} `json:"template"`
	} `json:"spec"`
}

type workloadContainerObservation struct {
	Name  string                   `json:"name"`
	Image string                   `json:"image"`
	Env   []workloadEnvObservation `json:"env"`
}

type workloadEnvObservation struct {
	Name string `json:"name"`
}

// workloadContainerPatch — strategic-merge-patch의 컨테이너 1엔트리(merge key:
// name). image leg는 Name+Image만, resources leg는 Name+Resources(+Policy+Env)만
// 채운다 — omitempty라 미채움 필드는 본문에 실리지 않고 SMP에서 무변경이다.
type workloadContainerPatch struct {
	Name            string                  `json:"name"`
	Image           string                  `json:"image,omitempty"`
	Resources       *workloadResourcesPatch `json:"resources,omitempty"`
	ImagePullPolicy string                  `json:"imagePullPolicy,omitempty"`
	Env             []workloadEnvPatch      `json:"env,omitempty"`
}

type workloadResourcesPatch struct {
	Requests map[string]string `json:"requests,omitempty"`
	Limits   map[string]string `json:"limits,omitempty"`
}

// workloadEnvPatch — env 1엔트리(merge key: name). v1 buildWorkloadEnvPatch와
// 동일: valueFrom 우선, 제거는 $patch: delete.
type workloadEnvPatch struct {
	Name      string         `json:"name"`
	Value     string         `json:"value,omitempty"`
	ValueFrom map[string]any `json:"valueFrom,omitempty"`
	Directive string         `json:"$patch,omitempty"`
}

// templatePatchBody — spec.template.spec 형태의 공통 봉투(restart의 restartPatch
// Body와 같은 층위). 구조체 직렬화로 필드 순서 고정 — 재실행 byte-identical(J1).
type templatePatchBody struct {
	Spec struct {
		Template struct {
			Spec struct {
				Containers []workloadContainerPatch `json:"containers"`
			} `json:"spec"`
		} `json:"template"`
	} `json:"spec"`
}

func newTemplatePatch(containers []workloadContainerPatch) templatePatchBody {
	var p templatePatchBody
	p.Spec.Template.Spec.Containers = containers
	return p
}

// fetchTemplateObservation — 현행 pod template GET 1회.
func (a *Adapter) fetchTemplateObservation(ctx context.Context, client *k8sClient, target workloadTarget) (*workloadTemplateObservation, error) {
	var obj workloadTemplateObservation
	if err := client.getJSONOp(ctx, target.apiPath(), nil, "execute", &obj); err != nil {
		return nil, err
	}
	return &obj, nil
}

// --- k8s.workload.image_update (v1 원천 UpdateK8sWorkloadImages — 단일 워크로드
// 축소 — phase6 H2에서 제거). ---

// imageUpdateVersion — Payload["version"] 검증: 공백 아닌 문자열(v1 "image
// version is required" 문언). 값은 replaceImageVersion 경험식의 유일 입력이다.
func imageUpdateVersion(payload contract.JSONMap) (string, error) {
	var spec struct {
		Version string `json:"version"`
	}
	if err := payloadDecode(payload, &spec); err != nil {
		return "", err
	}
	version := strings.TrimSpace(spec.Version)
	if version == "" {
		return "", errors.New("kubernetes: image version is required")
	}
	return version, nil
}

// replaceImageVersion — v1 k8s_build_pod.go:503 경험식의 V2 재구현(§J-P1-5 —
// 승계 원장 밖, 본 왕복 contracttest가 동치를 판정한다): @digest 절단 → 마지막
// / 뒤의 :tag 절단 → :version 부착.
func replaceImageVersion(image string, version string) string {
	trimmed := strings.TrimSpace(image)
	if trimmed == "" {
		return trimmed
	}

	base := trimmed
	if at := strings.Index(base, "@"); at >= 0 {
		base = base[:at]
	}

	lastSlash := strings.LastIndex(base, "/")
	lastColon := strings.LastIndex(base, ":")
	if lastColon > lastSlash {
		base = base[:lastColon]
	}

	return base + ":" + version
}

// imagePatches — 관측 컨테이너를 패치 엔트리로 변환(v1 :190-207 — name·image가
// 빈 컨테이너는 건너뛰고, 유효 엔트리가 하나도 없으면 에러).
func imagePatches(containers []workloadContainerObservation, version string) ([]workloadContainerPatch, error) {
	patches := make([]workloadContainerPatch, 0, len(containers))
	for _, container := range containers {
		name := strings.TrimSpace(container.Name)
		image := strings.TrimSpace(container.Image)
		if name == "" || image == "" {
			continue
		}
		patches = append(patches, workloadContainerPatch{
			Name:  name,
			Image: replaceImageVersion(image, version),
		})
	}
	if len(patches) == 0 {
		return nil, errors.New("kubernetes: no containers found in selected workload")
	}
	return patches, nil
}

// executeImageUpdate — GET(현행 images) → strategic-merge-patch 1회. 동일 version
// 재실행은 replaceImageVersion이 기존 tag를 절단하므로 byte-identical patch가
// 된다(provider_frozen_payload 멱등의 전송 근거).
func (a *Adapter) executeImageUpdate(ctx context.Context, req contract.OperationRequest) (contract.OperationHandle, error) {
	target, err := parseWorkloadURN(req.ResourceURN)
	if err != nil {
		return contract.OperationHandle{}, err
	}
	version, err := imageUpdateVersion(req.Payload)
	if err != nil {
		return contract.OperationHandle{}, err
	}
	client, err := a.executionClient(req)
	if err != nil {
		return contract.OperationHandle{}, err
	}

	obj, err := a.fetchTemplateObservation(ctx, client, target)
	if err != nil {
		return contract.OperationHandle{}, err
	}
	patches, err := imagePatches(obj.Spec.Template.Spec.Containers, version)
	if err != nil {
		return contract.OperationHandle{}, err
	}
	var resp struct {
		Metadata struct {
			Generation int64 `json:"generation"`
		} `json:"metadata"`
	}
	if err := client.patchJSON(ctx, target.apiPath(), newTemplatePatch(patches), "execute", &resp); err != nil {
		return contract.OperationHandle{}, err
	}
	if resp.Metadata.Generation <= 0 {
		return contract.OperationHandle{}, fmt.Errorf("kubernetes: patch response for %s carried no metadata.generation — cannot encode the rollout handle", target.apiPath())
	}
	return contract.OperationHandle{
		ProviderRef: encodeRolloutRef(req.Connection.UID, target, resp.Metadata.Generation),
	}, nil
}

// --- k8s.workload.resources_update (v1 원천 UpdateK8sWorkloadResources —
// 단일 워크로드 — phase6 H2에서 제거). ---

// resourcesUpdateSpec — payload 형태: {"containers":[{name, requests{cpu,memory},
// limits{…}, imagePullPolicy?, env[]}]} — v1 K8sWorkloadResourcesPayload의 JSON
// 형태. resource 키는 v1 면(cpu·memory) 한정이고, env는 v1 시맨틱 그대로다:
// 목록에 없는 기존 env는 $patch: delete로 제거된다(UI가 전체 env 상태를 보내는
// v1 계약의 승계 — 빈 env 목록은 전체 삭제).
type resourcesUpdateSpec struct {
	Containers []resourcesContainerSpec `json:"containers"`
}

type resourcesContainerSpec struct {
	Name            string             `json:"name"`
	Requests        map[string]string  `json:"requests"`
	Limits          map[string]string  `json:"limits"`
	ImagePullPolicy string             `json:"imagePullPolicy"`
	Env             []resourcesEnvSpec `json:"env"`
}

type resourcesEnvSpec struct {
	Name      string         `json:"name"`
	Value     string         `json:"value"`
	ValueFrom map[string]any `json:"valueFrom"`
}

// resourcesUpdateSpecOf — payload 형태 검증(v1 :233-302의 검증 순서): 컨테이너
// ≥ 1 · 이름 필수 · pull policy 화이트리스트 · env 이름 필수·중복 거부 · resource
// 키는 cpu·memory 한정.
func resourcesUpdateSpecOf(payload contract.JSONMap) (resourcesUpdateSpec, error) {
	var spec resourcesUpdateSpec
	if err := payloadDecode(payload, &spec); err != nil {
		return resourcesUpdateSpec{}, err
	}
	if len(spec.Containers) == 0 {
		return resourcesUpdateSpec{}, errors.New("kubernetes: invalid workload resource payload (containers is required)")
	}
	for _, container := range spec.Containers {
		if strings.TrimSpace(container.Name) == "" {
			return resourcesUpdateSpec{}, errors.New("kubernetes: container name is required")
		}
		for _, side := range []struct {
			label string
			values map[string]string
		}{{"requests", container.Requests}, {"limits", container.Limits}} {
			for key := range side.values {
				if key != "cpu" && key != "memory" {
					return resourcesUpdateSpec{}, fmt.Errorf("kubernetes: container %q %s resource key %q is not served (cpu and memory only)", container.Name, side.label, key)
				}
			}
		}
		if policy := strings.TrimSpace(container.ImagePullPolicy); policy != "" &&
			policy != "Always" && policy != "IfNotPresent" && policy != "Never" {
			return resourcesUpdateSpec{}, errors.New("kubernetes: invalid image pull policy")
		}
		seen := make(map[string]bool, len(container.Env))
		for _, env := range container.Env {
			name := strings.TrimSpace(env.Name)
			if name == "" {
				return resourcesUpdateSpec{}, errors.New("kubernetes: environment variable name is required")
			}
			if seen[name] {
				return resourcesUpdateSpec{}, fmt.Errorf("kubernetes: duplicate environment variable: %s", name)
			}
			seen[name] = true
		}
	}
	return spec, nil
}

// resourcesPatches — payload → patch 엔트리(v1 :258-302): 미지 컨테이너 거부,
// 빈 값 생략, env upsert+delete 병합. env는 v1과 동일하게 항상 병합된다(목록에서
// 빠진 기존 env가 delete된다).
func resourcesPatches(spec resourcesUpdateSpec, observed []workloadContainerObservation) ([]workloadContainerPatch, error) {
	existingByName := make(map[string]workloadContainerObservation, len(observed))
	for _, container := range observed {
		if name := strings.TrimSpace(container.Name); name != "" {
			existingByName[name] = container
		}
	}
	patches := make([]workloadContainerPatch, 0, len(spec.Containers))
	for _, item := range spec.Containers {
		existing, ok := existingByName[item.Name]
		if !ok {
			return nil, fmt.Errorf("kubernetes: container %s was not found in workload", item.Name)
		}
		patch := workloadContainerPatch{
			Name:      item.Name,
			Resources: resourcesPatchOf(item),
			Env:       envPatchOf(item, existing),
		}
		if policy := strings.TrimSpace(item.ImagePullPolicy); policy != "" {
			patch.ImagePullPolicy = policy
		}
		patches = append(patches, patch)
	}
	return patches, nil
}

// resourcesPatchOf — requests·limits의 비공백 값만 담는다(둘 다 비면 nil —
// 본문에서 생략되어 SMP 무변경).
func resourcesPatchOf(item resourcesContainerSpec) *workloadResourcesPatch {
	requests := nonEmptyValues(item.Requests)
	limits := nonEmptyValues(item.Limits)
	if len(requests) == 0 && len(limits) == 0 {
		return nil
	}
	return &workloadResourcesPatch{Requests: requests, Limits: limits}
}

func nonEmptyValues(values map[string]string) map[string]string {
	out := make(map[string]string, len(values))
	for k, v := range values {
		if trimmed := strings.TrimSpace(v); trimmed != "" {
			out[k] = trimmed
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// envPatchOf — v1 buildWorkloadEnvPatch: valueFrom 우선 엔트리 + 목록에서 빠진
// 기존 env의 $patch: delete.
func envPatchOf(item resourcesContainerSpec, existing workloadContainerObservation) []workloadEnvPatch {
	patch := make([]workloadEnvPatch, 0, len(item.Env))
	desired := make(map[string]bool, len(item.Env))
	for _, env := range item.Env {
		entry := workloadEnvPatch{Name: strings.TrimSpace(env.Name)}
		if len(env.ValueFrom) > 0 {
			entry.ValueFrom = env.ValueFrom
		} else {
			entry.Value = env.Value
		}
		patch = append(patch, entry)
		desired[entry.Name] = true
	}
	for _, env := range existing.Env {
		name := strings.TrimSpace(env.Name)
		if name == "" || desired[name] {
			continue
		}
		patch = append(patch, workloadEnvPatch{Name: name, Directive: "delete"})
	}
	return patch
}

// executeResourcesUpdate — GET(현행 containers) → strategic-merge-patch 1회.
// 멱등 근거(provider_frozen_payload): 동일 payload 재실행은 동일 상태를 재적용하는
// patch다 — 첫 재실행만 delete 대상 소멸로 본문이 줄어들 뿐, 어느 재실행도 실제
// spec 변경을 만들지 않는다 → generation 무증가(재롤아웃 없음).
func (a *Adapter) executeResourcesUpdate(ctx context.Context, req contract.OperationRequest) (contract.OperationHandle, error) {
	target, err := parseWorkloadURN(req.ResourceURN)
	if err != nil {
		return contract.OperationHandle{}, err
	}
	spec, err := resourcesUpdateSpecOf(req.Payload)
	if err != nil {
		return contract.OperationHandle{}, err
	}
	client, err := a.executionClient(req)
	if err != nil {
		return contract.OperationHandle{}, err
	}

	obj, err := a.fetchTemplateObservation(ctx, client, target)
	if err != nil {
		return contract.OperationHandle{}, err
	}
	patches, err := resourcesPatches(spec, obj.Spec.Template.Spec.Containers)
	if err != nil {
		return contract.OperationHandle{}, err
	}
	var resp struct {
		Metadata struct {
			Generation int64 `json:"generation"`
		} `json:"metadata"`
	}
	if err := client.patchJSON(ctx, target.apiPath(), newTemplatePatch(patches), "execute", &resp); err != nil {
		return contract.OperationHandle{}, err
	}
	if resp.Metadata.Generation <= 0 {
		return contract.OperationHandle{}, fmt.Errorf("kubernetes: patch response for %s carried no metadata.generation — cannot encode the rollout handle", target.apiPath())
	}
	return contract.OperationHandle{
		ProviderRef: encodeRolloutRef(req.Connection.UID, target, resp.Metadata.Generation),
	}, nil
}
