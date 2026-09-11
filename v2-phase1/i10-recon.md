# I10 착수 리컨 (2026-09-11, 메인 세션 직접 — §3 리컨 병렬화)

task: I10 create 오퍼레이션화 · tier XL · 원천: p6-state.json `next` + phase6-plan.md §J9·D-15·§14.2-3·I10 행

## 확정 좌표

### 현재 v1 create 면
- 라우트: `backend/router/routes_v1_infra.go:241` — `POST /k8s/resource/yaml/create`, opdef.Middleware(db, opdef.Must(...))
- opdef 행: `backend/opdef/defs_monitor.go:77` — Permission `assets:k8s:workload:yaml`, Mutating, RiskMedium
- 컨트롤러: `backend/controller/k8s.go:316` → 서비스 `backend/service/k8s_mutate.go:14` `CreateK8sResourceYAML`
- 서비스 흐름: payload{ClusterID, ResourceType, Namespace, Name, YAML} → `k8sClientForCluster(clusterID)`(k8s_fetch.go:119 → GetK8sCluster = S5/I-a 후 provider_connection+SecretRef 이중 해상) → YAMLToJSON → parseK8sManifestIdentity → buildK8sCreateResourcePaths → 클러스터 POST

### V2 면 (internal/api/v2)
- `infra.go:84 Register` — GET 4종만 (provider-types·provider-connections·resources·resources/:uid). 주석: "GET-only, non-sensitive — no opdef middleware (plan J7)"
- `operations.go:52 RegisterOperations` — §16.2: resources/:uid/operations/{name}/plan|execute + tasks 5종. AuditMiddleware 선행 + opdef.V2DynamicMiddleware grant
- 멱등성: operations.go에 Idempotency 처리 존재(§13.4) — grep 확인
- apply 정의: `internal/infra/compose/compose_k8s_ops.go:229` `k8s.resource.apply` — **update 한정**(주석: create는 uid-스코프 부적합), Permission `assets:k8s:workload:yaml`, capability `orchestration.kubernetes.apply`
- resourceMutationKinds(:218): workload·namespace·configmap·secret·network.load_balancer·network.gateway·network.http_route·storage.volume 8종 — **pv·istio kinds 면 밖**

### 프론트 소비 (web/src)
- `api/k8s.js:200` createK8sResourceYAML → POST /api/v1/k8s/resource/yaml/create. :65 주석 "stays on v1 by design — create has no uid-scoped"
- K8s.vue 4콜사이트:
  - ~1715 PV 생성(cluster-scope, namespace 없음)
  - ~1886 configmap/secret류 — **일부 kind는 이미 V2 resourceApply 분기 존재**(K8S_OPERATIONS.resourceApply, target=K8S_RESOURCE_TARGETS, payload yaml), 나머지는 v1 create 폴백
  - ~1904 namespace 생성
  - ~2099 istio 리소스 생성(destinationrule/virtualservice 등 — V2 면 밖 kinds)
- cluster.value.id: `queryK8sClusterList()` = v1 `/k8s/cluster/list`(G0 후 V2 투영) — id 공간 = provider_connection 이중 해상(I-b 전환)

### 골든·규약
- C38: `grep -c . docs/security/route-inventory.txt` → 440(현재)·**I10 후 439** · sensitive-routes.txt → 280→**279**
- C40: 하드코딩 수치 음의 증명(갱신 누락 기계 차단)
- register-k8s CLI: `backend/main_register_k8s.go` — 체인 seed(provider_connection→provider_context→봉인 SecretRef→credential binding)·EncryptSecretV2 봉인·stdout JSON report
- §16.1 스펙: POST /provider-connections·{uid}/validate·{uid}/sync 미구현(I3) — GET만 존재(infra.go:86)
- §16.3 extensions: `POST /provider-contexts/{uid}/extensions/{namespace}/{operation}` — "canonical representation 없는 기능"용 — YAML apply-by-manifest 후보 경로

## 계획이 판단할 설계 쟁점 (메인 예비 판단 — 계획 에이전트가 확정)
1. **커넥션-스코프 apply 라우트 형태**: (a) §16.1 확장 `POST /provider-connections/{uid}/operations/{name}/*`(스펙 개정 동반) vs (b) §16.3 extensions 경로(스펙 부합·registered capability 요건) vs (c) 신규 오퍼레션 def+라우트. plan:161은 (a) 서술했으나 §16.3 부합성 재검토 필요
2. **kinds 면**: v1 create face는 pv·istio 포함 — V2 resourceMutationKinds 8종 밖. 커넥션-스코프 apply의 kinds/검증 범위 (매니페스트 검증 vs kind 어휘)
3. **I3 경계**: POST /provider-connections(create·validate·sync) 포함 여부 — I10 본체는 불요(apply는 기존 커넥션 대상). R26 UI 회귀는 I3. 동시 계획·분리 구현 옵션
4. **동기 vs 태스크**: v1 create는 동기 응답(created:true). V2 plan/execute는 비동기 태스크(승인 정책). 프론트 UX(즉시 완료 토스트)와 V2 계약(§13.4 멱등·audit·policy 승인)의 조정 — H1 완료토스트 종단 대기 선례 있음
5. **클러스터 id 공간**: 프론트 cluster.value.id(legacy/이중 해상) → 커넥션 uid 변환 경로(프론트 resolveK8sResourceUid? 또는 API가 id 수용?)

## 프로세스 계약
- XL: plan-xhigh → adversary 5렌즈 직렬(GLM 상한) → implement → review-pr-xhigh → verify(메인 재실행)
- 상태: p6-state.json `i10` 섹션(프로젝트 관례 경로 — 스킬 기본 docs/task-id/ 대체, 커밋 관례)
- 보존 제약 승계: 골든 439/279 갱신·opdef create 행 처분·k8s.js:65 주석 제거·봉인 계약(kubeconfig 평문 금지)·§19.1 step4 13/13 완결(D-15 상환)
