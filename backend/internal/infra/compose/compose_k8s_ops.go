// compose_k8s_ops.go — k8s 오퍼레이션 정의 등록부(P2-C 분할 — 리뷰 권고:
// compose.go 595행에 opdef 2종을 더하면 650 경고선을 넘으므로 k8s opdef 등록부를
// 별도 파일로 떼어낸다 — §9-7 분할 규칙의 결과다). 정의의 소재는 코드다(§3.3 —
// descriptors are code). pve opdef(registerProxmoxMutations 계열)는 compose.go에
// 남는다 — P2-C 성장분은 k8s 옆이고 분할 봉합선은 provider 단위다(판단 기록).
package compose

import (
	"ops-admin/backend/internal/infra/adapter/kubernetes"
	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/registry"
)

// restartOperation — k8s.workload.restart 정의(§3.3).
var restartOperation = contract.OperationDefinition{
	Name:    kubernetes.RestartOperationName,
	Version: "1",
	// §10.3 "role grants carry over" — 이미 시드된 v1 권한 재사용(J4,
	// 신규 권한 문자열 0 — 보존 제약 #2).
	RequiredPermission: "assets:k8s:workload:restart",
	RequiredCapability: "orchestration.kubernetes.apply",
	ResourceKinds:      []string{"orchestration.workload"},
	Mutating:           true,
	RiskLevel:          "medium",
	// J8 — V2 레인 신중 posture(첫 프로덕션 mutation의 승인 실증).
	RequiresApproval: true,
	// J1 — provider 수준 멱등: 동결 restartedAt의 byte-identical patch.
	IdempotencyPolicy: "provider_frozen_annotation",
	TimeoutSeconds:    30,
	RetryPolicy:       contract.RetryPolicy{MaxAttempts: 3, BackoffSeconds: 5},
	// §10.2 typed redaction spec — 결과 detail의 허용 필드를 타입으로 명시.
	// executor의 성공 detail 키와 1:1(generation·restartedAt·readyReplicas·
	// updated·serverURL) — 그 밖의 키는 허용되지 않는다.
	Redaction: func() any { return rolloutResultRedaction{} },
}

// registerKubernetesMutations — P2-A~D(계획 r3 §J-P1-6 확정표): restart를
// 제외한 workload mutation 9종 opdef 등록(descriptors are code). 권한 문자열은
// v1 sensitive-routes.txt 재사용 — 신규 0(보존 제약 #6). 전 opdef 승인 필수
// (확정표 수정 2건 — restart 선례 compose.go RequiresApproval 승계). 등록은
// pve 선례(registerProxmoxMutations)와 같은 명시 콜 — P2-A는 workload 3종을,
// P2-B는 state-convergent 2종을, P2-C가 resource 2종을 넣었고 P2-D가 나머지를
// 추가한다(G-P2a 콜사이트 계수의 궤도).
func registerKubernetesMutations(reg *registry.Registry) error {
	if err := reg.RegisterOperation(workloadScaleOperation); err != nil {
		return err
	}
	if err := reg.RegisterOperation(workloadImageUpdateOperation); err != nil {
		return err
	}
	if err := reg.RegisterOperation(workloadResourcesUpdateOperation); err != nil {
		return err
	}
	if err := reg.RegisterOperation(nodeLabelsUpdateOperation); err != nil {
		return err
	}
	if err := reg.RegisterOperation(serviceUpdateOperation); err != nil {
		return err
	}
	if err := reg.RegisterOperation(resourceApplyOperation); err != nil {
		return err
	}
	if err := reg.RegisterOperation(resourceDeleteOperation); err != nil {
		return err
	}
	if err := reg.RegisterOperation(istioTrafficUpdateOperation); err != nil {
		return err
	}
	return reg.RegisterOperation(httpRouteTrafficUpdateOperation)
}

// rolloutResultRedaction — rollout 4종(restart 포함)이 공유하는 결과 detail 허용
// 필드(§10.2). Poll(executor.go)이 상시 싣는 공통 5키와 1:1이고, restart 외
// op의 restartedAt는 부재 관측값(공백 문자열)이다 — plan 응답의 restartedAt 키가
// 타 op에 붙는 것과 같은 기존 동작(무해)의 승계(§J-P1-6).
type rolloutResultRedaction struct {
	Generation    int64  `json:"generation"`
	RestartedAt   string `json:"restartedAt"`
	ReadyReplicas int    `json:"readyReplicas"`
	Updated       int    `json:"updated"`
	ServerURL     string `json:"serverURL"`
}

// stateResultRedaction — state-convergent 가족(P2-B node·service, P2-C apply가
// 승계)이 공유하는 결과 detail 허용 필드(§10.2). Poll(executor_config.go
// pollStateRef·pollManifestRef)이 싣는 2키와 1:1이다 — rollout 가족의 5키에서
// rollout 전용 성분(restartedAt·replica 카운트)을 뺀 최소면이다. generation이
// 없는 종(configmap 등)은 부재 관측값 0(restartedAt 공백 선례).
type stateResultRedaction struct {
	Generation int64  `json:"generation"`
	ServerURL  string `json:"serverURL"`
}

// deleteResultRedaction — resource delete(P2-C)의 폴 종단(404=Succeeded) detail
// 허용 필드(§10.2). 부재 판정에는 관측 성분이 없다 — serverURL(어느 클러스터에서
// 수렴했는가)만 싣는다(pollAbsentRef — executor_resource.go).
type deleteResultRedaction struct {
	ServerURL string `json:"serverURL"`
}

// workload mutation 3종 opdef(P2-A — 계획 r3 §J-P1-6 확정표). 공통 posture:
// capability orchestration.kubernetes.apply · kind orchestration.workload ·
// mutating · risk medium · 승인 필수 · rollout 수렴 폴(restart leg와 handle·
// Poll 공유). RetryPolicy는 restart 선례(MaxAttempts 3 · BackoffSeconds 5 —
// frozen annotation/payload·state-convergent 모두 provider 수준 멱등이라
// 재시도가 재롤아웃을 유발하지 않는다).

// workloadScaleOperation — k8s.workload.scale(v1 k8s_workload.go:61). /scale
// 서브리소스 merge-patch — 동일 목표 replicas로의 재실행은 상태 재수렴.
var workloadScaleOperation = contract.OperationDefinition{
	Name:               kubernetes.ScaleOperationName,
	Version:            "1",
	RequiredPermission: "assets:k8s:workload:scale", // v1 sensitive-routes.txt:184
	RequiredCapability: "orchestration.kubernetes.apply",
	ResourceKinds:      []string{"orchestration.workload"},
	Mutating:           true,
	RiskLevel:          "medium",
	RequiresApproval:   true,
	IdempotencyPolicy:  "provider_state_convergent",
	TimeoutSeconds:     30,
	RetryPolicy:        contract.RetryPolicy{MaxAttempts: 3, BackoffSeconds: 5},
	Redaction:          func() any { return rolloutResultRedaction{} },
}

// workloadImageUpdateOperation — k8s.workload.image_update(v1 원천
// UpdateK8sWorkloadImages — phase6 H2에서 제거). 동결 version으로의 replaceImageVersion 경험식 patch —
// 기존 tag 절단 때문에 재실행은 byte-identical이 된다.
var workloadImageUpdateOperation = contract.OperationDefinition{
	Name:               kubernetes.ImageUpdateOperationName,
	Version:            "1",
	RequiredPermission: "assets:k8s:workload:image", // v1 sensitive-routes.txt:183
	RequiredCapability: "orchestration.kubernetes.apply",
	ResourceKinds:      []string{"orchestration.workload"},
	Mutating:           true,
	RiskLevel:          "medium",
	RequiresApproval:   true,
	IdempotencyPolicy:  "provider_frozen_payload",
	TimeoutSeconds:     30,
	RetryPolicy:        contract.RetryPolicy{MaxAttempts: 3, BackoffSeconds: 5},
	Redaction:          func() any { return rolloutResultRedaction{} },
}

// workloadResourcesUpdateOperation — k8s.workload.resources_update(v1 원천
// UpdateK8sWorkloadResources — phase6 H2에서 제거). 동결 containers payload로의 resources·env·pull policy
// patch — 재실행은 동일 상태의 재적용(실변경 없음 → generation 무증가)이라
// 재롤아웃이 없다.
var workloadResourcesUpdateOperation = contract.OperationDefinition{
	Name:               kubernetes.ResourcesUpdateOperationName,
	Version:            "1",
	RequiredPermission: "assets:k8s:workload:yaml", // v1 sensitive-routes.txt:272
	RequiredCapability: "orchestration.kubernetes.apply",
	ResourceKinds:      []string{"orchestration.workload"},
	Mutating:           true,
	RiskLevel:          "medium",
	RequiresApproval:   true,
	IdempotencyPolicy:  "provider_frozen_payload",
	TimeoutSeconds:     30,
	RetryPolicy:        contract.RetryPolicy{MaxAttempts: 3, BackoffSeconds: 5},
	Redaction:          func() any { return rolloutResultRedaction{} },
}

// state-convergent 2종 opdef(P2-B — 계획 r3 §J-P1-6 확정표). rollout이 없는
// mutation — handle·poll은 state handle 가족(executor_config.go: 발행+poll 1회,
// 기대 상태 에코 판정)이다. 공통 posture는 workload 3종과 동일(capability
// orchestration.kubernetes.apply · mutating · risk medium · 승인 필수 · restart
// 선례 RetryPolicy — state-convergent 재시도는 동일 상태 재수렴이라 무해)이고,
// resource kind가 workload를 벗어난다(node·network.load_balancer — 등록 검증은
// 어휘 소속만 요구).

// nodeLabelsUpdateOperation — k8s.node.labels_update(v1 원천 UpdateK8sNodeLabels — phase6 H2에서 제거).
// payload에 없는 기존 레이블의 null 제거 포함 — 재실행은 제거항 소멸 후 동일 상태
// 재적용(provider_state_convergent).
var nodeLabelsUpdateOperation = contract.OperationDefinition{
	Name:               kubernetes.NodeLabelsUpdateOperationName,
	Version:            "1",
	RequiredPermission: "assets:k8s:workload:yaml", // v1 sensitive-routes.txt:269
	RequiredCapability: "orchestration.kubernetes.apply",
	ResourceKinds:      []string{"orchestration.node"},
	Mutating:           true,
	RiskLevel:          "medium",
	RequiresApproval:   true,
	IdempotencyPolicy:  "provider_state_convergent",
	TimeoutSeconds:     30,
	RetryPolicy:        contract.RetryPolicy{MaxAttempts: 3, BackoffSeconds: 5},
	Redaction:          func() any { return stateResultRedaction{} },
}

// serviceUpdateOperation — k8s.service.update(v1 원천 UpdateK8sService — phase6 H2에서 제거).
// wholesale-spec merge-patch(type·selector·ports·labels·annotations) — 재실행은
// 동일 값의 재 upsert(provider_state_convergent).
var serviceUpdateOperation = contract.OperationDefinition{
	Name:               kubernetes.ServiceUpdateOperationName,
	Version:            "1",
	RequiredPermission: "assets:k8s:workload:yaml", // v1 sensitive-routes.txt:271
	RequiredCapability: "orchestration.kubernetes.apply",
	ResourceKinds:      []string{"network.load_balancer"},
	Mutating:           true,
	RiskLevel:          "medium",
	RequiresApproval:   true,
	IdempotencyPolicy:  "provider_state_convergent",
	TimeoutSeconds:     30,
	RetryPolicy:        contract.RetryPolicy{MaxAttempts: 3, BackoffSeconds: 5},
	Redaction:          func() any { return stateResultRedaction{} },
}

// resource mutation 2종 opdef(P2-C — 계획 r3 §J-P1-6 확정표·§10-4 권한 검토
// 결론). risk high·승인 필수다(v1이 동일 권한으로 이미 임의 kind를 다뤘다 —
// risk·승인만 상향: pve config의 high 선례·restart의 승인 선례. 분리 권한 신설은
// 차후 하드닝 — §10-4). RetryPolicy는 restart 선례 승계 — provider_frozen_manifest
// 재 PUT·provider_terminal_404 재 DELETE 모두 provider 수준 멱등이라 재시도가
// 목표 상태를 흔들지 않는다.

// resourceMutationKinds — resource apply·delete가 통제하는 kind 면. §J-P1-6
// 확정표가 ResourceKinds를 못 박지 않아 v1 apply·delete face(buildK8sYAMLResourcePath
// 계열)의 V2 URN 대응 전수로 확정했다(executor_resource.go 판단 기록 — node·pod는
// uid 신원이라 제외, istio는 P2-D 앵커, endpoints·storageclass는 v1 face 부재).
// 등록 검증은 어휘 소속만 요구한다.
var resourceMutationKinds = []string{
	"orchestration.workload",
	"orchestration.namespace",
	"orchestration.configmap",
	"orchestration.secret",
	"network.load_balancer",
	"network.gateway",
	"network.http_route",
	"storage.volume",
}

// resourceApplyOperation — k8s.resource.apply(update 한정, v1
// sensitive-routes.txt:270 PUT /k8s/resource/yaml). create는 uid-스코프
// 오퍼레이션 모델에 안 맞아 이월이다(I-P1 — Phase H가 v1 create 라우트를 지우려면
// 선행 필수, R-P5). provider_frozen_manifest — 동일 payload 재실행은 동일 목표
// manifest의 재 PUT(RV 주입만 현행 관측을 따라간다).
var resourceApplyOperation = contract.OperationDefinition{
	Name:               kubernetes.ApplyOperationName,
	Version:            "1",
	RequiredPermission: "assets:k8s:workload:yaml", // v1 sensitive-routes.txt:270
	RequiredCapability: "orchestration.kubernetes.apply",
	ResourceKinds:      resourceMutationKinds,
	Mutating:           true,
	RiskLevel:          "high",
	RequiresApproval:   true,
	IdempotencyPolicy:  "provider_frozen_manifest",
	TimeoutSeconds:     30,
	RetryPolicy:        contract.RetryPolicy{MaxAttempts: 3, BackoffSeconds: 5},
	Redaction:          func() any { return stateResultRedaction{} },
}

// resourceDeleteOperation — k8s.resource.delete(v1 sensitive-routes.txt:29
// DELETE /k8s/resource/delete — v1부터 risk high). 비가역 op다. 폴 404가 성공
// 종단이다(provider_terminal_404 — client.go errNotFound 센티넬 재사용, P1-C
// 판단 6).
var resourceDeleteOperation = contract.OperationDefinition{
	Name:               kubernetes.DeleteOperationName,
	Version:            "1",
	RequiredPermission: "assets:k8s:resource:delete", // v1 sensitive-routes.txt:29
	RequiredCapability: "orchestration.kubernetes.apply",
	ResourceKinds:      resourceMutationKinds,
	Mutating:           true,
	RiskLevel:          "high",
	RequiresApproval:   true,
	IdempotencyPolicy:  "provider_terminal_404",
	TimeoutSeconds:     30,
	RetryPolicy:        contract.RetryPolicy{MaxAttempts: 3, BackoffSeconds: 5},
	Redaction:          func() any { return deleteResultRedaction{} },
}

// traffic mutation 2종 opdef(P2-D — 계획 r3 §J-P1-6 확정표). 확정표 행 그대로:
// 권한 assets:k8s:advancednetwork(v1 재사용 — sensitive-routes.txt:180-181
// httproute·istio traffic 라인, 신규 문자열 0 — 보존 제약 #6)·risk medium·승인
// 필수·provider_frozen_payload. handle·poll은 확정표 "발행+poll 1회"대로 state
// handle 가족(executor_state.go — 동결 엔트리 위치·가중치의 에코 판정)이고
// detail은 stateResultRedaction 2키다. RetryPolicy는 restart 선례 — 동일 가중치
// 배열의 재 PUT은 실변경이 없으면 무증가다(provider_frozen_payload).

// istioTrafficUpdateOperation — k8s.istio.traffic_update(v1 원천 UpdateK8sIstioTraffic — phase6 H2에서 제거).
// VirtualService의 첫 조정 가능 http 항목 경로에 위치 가중치 splice PUT.
var istioTrafficUpdateOperation = contract.OperationDefinition{
	Name:               kubernetes.IstioTrafficUpdateOperationName,
	Version:            "1",
	RequiredPermission: "assets:k8s:advancednetwork", // v1 sensitive-routes.txt:181
	RequiredCapability: "orchestration.kubernetes.apply",
	ResourceKinds:      []string{"network.virtual_service"},
	Mutating:           true,
	RiskLevel:          "medium",
	RequiresApproval:   true,
	IdempotencyPolicy:  "provider_frozen_payload",
	TimeoutSeconds:     30,
	RetryPolicy:        contract.RetryPolicy{MaxAttempts: 3, BackoffSeconds: 5},
	Redaction:          func() any { return stateResultRedaction{} },
}

// httpRouteTrafficUpdateOperation — k8s.httproute.traffic_update(v1 원천
// UpdateK8sHTTPRouteTraffic — phase6 H2에서 제거). HTTPRoute의 첫 조정 가능 rule backendRefs에 위치 가중치
// splice PUT.
var httpRouteTrafficUpdateOperation = contract.OperationDefinition{
	Name:               kubernetes.HTTPRouteTrafficUpdateOperationName,
	Version:            "1",
	RequiredPermission: "assets:k8s:advancednetwork", // v1 sensitive-routes.txt:180
	RequiredCapability: "orchestration.kubernetes.apply",
	ResourceKinds:      []string{"network.http_route"},
	Mutating:           true,
	RiskLevel:          "medium",
	RequiresApproval:   true,
	IdempotencyPolicy:  "provider_frozen_payload",
	TimeoutSeconds:     30,
	RetryPolicy:        contract.RetryPolicy{MaxAttempts: 3, BackoffSeconds: 5},
	Redaction:          func() any { return stateResultRedaction{} },
}
