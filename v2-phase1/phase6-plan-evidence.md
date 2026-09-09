# Phase 6 계획 — 증거층 (evidence)

본문: `v2-phase1/phase6-plan.md`. 본 문서는 본문이 인용하는 실측 원자료다 — 판정은 하지 않고
측정만 기록한다. 측정 시점 HEAD: `3a3a106` (main). 측정일: 2026-09-09.

표기: **실측** = 이 세션에서 파일/DB/명령으로 확인 · **추정** = 계산·추론 · **미확인** = 확인 못 함.

---

## E1. 라우터 골든의 관측 범위 (step 1 근거)

### E1.1 골든 생성기 — 정렬·최종 핸들러만

`backend/router/routes_inventory_test.go`:

- `:66-74` `routeInventoryLines`: `engine.Routes()`를 순회해
  `fmt.Sprintf("%s %s -> %s", route.Method, route.Path, route.Handler)` 한 줄씩 만들고
  **`sort.Strings(lines)`** 로 정렬한다. → **등록 순서는 골든 바이트에 들어가지 않는다** (실측).
- `:118-121` `TestRouteInventoryArtifact`: `len(lines) != 450`이면 즉시 실패. 450 = 1 ping
  + 7 public + 440 authGroup + 2 uploads(GET/HEAD).
- `:96-113` `writeOrCompareArtifact`: 커밋된 `docs/security/route-inventory.txt`와 **바이트 동일**
  요구. 재생성은 `-update`.

### E1.2 골든은 미들웨어를 보지 않는다 — 자체 증명

`router/router.go:75`는 다음처럼 opdef 미들웨어를 **앞에** 붙인다:

```
authGroup.GET("/domain/public/accounts", opdef.Middleware(db, opdef.Must(...)), ctl.ListPublicDNSAccounts)
```

그런데 `docs/security/route-inventory.txt:145`는:

```
GET /api/v1/k8s/cluster/detail -> ops-admin/backend/controller.(*Controller).GetK8sClusterDetail-fm
```

미들웨어가 붙은 라우트도 **최종 핸들러 이름만** 기록된다 (실측). → 골든은 미들웨어의
존재·순서·개수에 **완전히 무감각**하다. behavior-neutral 증명 도구로 골든 단독은 불충분.

### E1.3 미들웨어 부착 지점 전수 (router.go — 이 5개 지점이 순서 계약의 전부)

| line | 내용 |
|---|---|
| `router/router.go:43` | `engine.UseRawPath = true` (URN %2F 라우팅 계약 — Slice A E2E가 근거) |
| `router/router.go:44` | `engine.Use(gin.Logger(), gin.Recovery(), middleware.CORS())` — 전역 3종 순서 |
| `router/router.go:61` | `api := engine.Group("/api/v1")` — v1 그룹 접두사 |
| `router/router.go:72-73` | `authGroup := api.Group("")` + `authGroup.Use(middleware.Auth(db), middleware.OperationLog(db))` |
| `router/router.go:535-536` | `v2Group := engine.Group("/api/v2/infra")` + `v2Group.Use(middleware.Auth(db), middleware.OperationLog(db))` |

그 밖의 `.Use(` 호출 없음 (실측 — `grep -n "\.Use(" router/router.go` 결과 3건: 44·73·536).

### E1.4 함께 도는 중립성 계측기 3종

| 테스트 | 파일:line | 무엇을 잠그는가 |
|---|---|---|
| `TestRouteInventoryArtifact` | `router/routes_inventory_test.go:118` | 라우트 집합 450 + 골든 바이트 |
| `TestSensitiveRoutesArtifact` | `router/routes_inventory_test.go:135` | opdef 표 → `sensitive-routes.txt` 바이트 + 선언순서 비의존 |
| `TestOperationTableCoversRouter` | `router/authz_replay_test.go:305` | ① 모든 opdef def에 **실 라우트가 존재** ② `sensitive-routes.txt` 행수 == `len(opdef.All())` ③ authGroup 비-GET 라우트 전부가 opdef 표에 등재 |
| authz replay (권한 재생) | `router/authz_replay_test.go` (356줄 전체) | 역할×민감라우트 allow 결과가 기준선과 동일 |
| `engine_injection_test.go` | `router/engine_injection_test.go` (135줄) | v2API 주입/비주입 두 경로 모두에서 라우트 표면 불변 (R11) |

`authGroupRoutes`(`authz_replay_test.go:47-63`)는 440개 인증 라우트를 v1+v2 접두사로 걸러낸다 —
라우트가 **다른 그룹으로 옮겨지면 이 필터가 먼저 깨진다**.

### E1.5 라우트 골든·민감 골든 규모

- `docs/security/route-inventory.txt` — 453줄 (헤더 3 + 라우트 450) (실측 `grep -c .`)
- `docs/security/sensitive-routes.txt` — 293줄 (헤더 3 + def 290) (실측)

---

## E2. v1 k8s 라우트 전수 (36행 — step 3·4 대상 판정 근거)

`docs/security/route-inventory.txt`에서 `k8s` 매칭 **36행** (실측 `grep -c k8s`). `asset/service/k8s/catalog`(:90)는
자산-서비스 도메인이므로 k8s family 밖 — 실제 k8s family 라우트는 **35건**(GET 21 · 비-GET 14).

### E2.1 읽기 (GET) 21건 — step 3 대상 후보

| inv line | 라우트 | 민감표 등재 |
|---|---|---|
| 145 | `GET /k8s/cluster/detail` | ✅ `assets:k8s:cluster` risk=high |
| 146 | `GET /k8s/cluster/info` | ✅ `assets:k8s:cluster` |
| 147 | `GET /k8s/cluster/list` | ✅ `assets:k8s:cluster` |
| 148 | `GET /k8s/configmap/detail` | ✅ `assets:k8s:configstorage` |
| 149 | `GET /k8s/ingress/detail` | — |
| 150 | `GET /k8s/istio/detail` | — |
| 151 | `GET /k8s/namespace/detail` | — |
| 152 | `GET /k8s/namespace/events` | — |
| 153 | `GET /k8s/node/detail` | — |
| 154 | `GET /k8s/node/pods` | — |
| 155 | `GET /k8s/pod/containers` | — |
| 156 | `GET /k8s/pod/detail` | — |
| 157 | `GET /k8s/pod/events` | — |
| 158 | `GET /k8s/pod/logs` | ✅ `assets:k8s:pod` |
| 159 | `GET /k8s/pod/metrics` | — |
| 160 | `GET /k8s/pod/terminal/ws` | (public 라우트 — 콘솔 티켓) |
| 161 | `GET /k8s/secret/detail` | ✅ `assets:k8s:configstorage` risk=high |
| 162 | `GET /k8s/service/detail` | — |
| 163 | `GET /k8s/storage/detail` | — |
| 164 | `GET /k8s/workload/detail` | — |
| 165 | `GET /k8s/workload/metrics` | — |

### E2.2 쓰기 (비-GET) **14건** — step 4 대상 후보. 전부 민감표 등재

| inv line | 라우트 | 권한 | V2 대응 |
|---|---|---|---|
| 28 | `DELETE /k8s/cluster/delete` | `assets:k8s:cluster:delete` | ✗ |
| 29 | `DELETE /k8s/resource/delete` | `assets:k8s:resource:delete` | ✗ |
| 333 | `POST /k8s/cluster/add` | `assets:k8s:cluster:add` | ✗ |
| 334 | `POST /k8s/httproute/traffic` | `assets:k8s:advancednetwork` | ✗ |
| 335 | `POST /k8s/istio/traffic` | `assets:k8s:advancednetwork` | ✗ |
| 336 | `POST /k8s/resource/yaml/create` | `assets:k8s:workload:yaml` | ✗ |
| 337 | `POST /k8s/workload/images` | `assets:k8s:workload:image` | ✗ |
| **338** | **`POST /k8s/workload/restart`** | **`assets:k8s:workload:restart`** | **✅ §3.3 M1 흡수 대상 — 유일** |
| 339 | `POST /k8s/workload/scale` | `assets:k8s:workload:scale` | ✗ |
| 429 | `PUT /k8s/cluster/update` | `assets:k8s:cluster:edit` | ✗ |
| 430 | `PUT /k8s/node/labels` | `assets:k8s:workload:yaml` | ✗ |
| 431 | `PUT /k8s/resource/yaml` | `assets:k8s:workload:yaml` | ✗ |
| 432 | `PUT /k8s/service/update` | `assets:k8s:workload:yaml` | ✗ |
| 433 | `PUT /k8s/workload/resources` | `assets:k8s:workload:yaml` | ✗ |

V2 대응 근거: 스펙 §3.3 "Milestone 1 absorbs (ProviderTask): Kubernetes workload restart
(the Phase 3 proof) / future V2 provider operations as they are added (**nothing else**)"
(`docs/architecture/multi-infrastructure-control-plane-v2.md:257-260`).
`docs/security/sensitive-routes.txt:157-158`이 V2 plan/execute 쌍에 동일 권한
`assets:k8s:workload:restart`를 부착한 것을 확인 (실측).

### E2.3 프론트 호출 지점

| 파일:line | 호출 |
|---|---|
| `web/src/api/k8s.js:3` | `queryK8sClusterList` → `/api/v1/k8s/cluster/list` |
| `web/src/api/k8s.js:5` | `queryK8sClusterInfo` → `/api/v1/k8s/cluster/info` |
| `web/src/api/k8s.js:14` | `queryK8sClusterDetail` → `/api/v1/k8s/cluster/detail` |
| `web/src/api/k8s.js:73` | `restartK8sWorkload` → **`POST /api/v1/k8s/workload/restart`** |
| `web/src/api/infra.js:22,26` | V2 `…/operations/:name/plan`·`/execute` (별도 UI 경로) |

→ 콘솔은 restart를 **여전히 v1 라우트로** 호출한다 (실측). V2 오퍼레이션 UI는 병존하는
별개 화면이다. step 4는 프론트 변경을 수반한다.

---

## E3. legacy ↔ V2 필드 대응 전수 (step 3 판정의 결정적 근거)

원천: `backend/internal/infra/adapter/kubernetes/mapping.md` §4 커버리지 표 (§15.2 권위 표).
대상 legacy DTO: `backend/model/k8s.go:491-501` `K8sClusterDetail` — 9섹션.

### E3.1 섹션별 이관율

| 섹션 | legacy 구조 | mapping.md 처분 | V2 투영 제공 가능? |
|---|---|---|---|
| `cluster` | `K8sClusterView` (model/k8s.go:114) | 13필드 중 **mapped 1**(version), 나머지 dropped(v2-source-key/derived/volatile) | ✗ — connection/context 속성이지 리소스 행 아님 |
| `overview` | `K8sOverview` | **전 필드 dropped** (healthScore·cpuUsage·memoryUsage·podUsage·requestRate·alertCount·distribution[]·certificates[]) | ✗ **전무** |
| `nodes` | `K8sNodeItem` (model/k8s.go:164) | 9필드 중 mapped 4 (name·role·status·cpu·memory), dropped 5 (version·internalIP·os·pods) | 부분 |
| `namespaces` | `K8sNamespaceItem` (:182) | 5필드 중 mapped 2 (name·status), createdAt은 VOLATILE mapped, dropped 3 (pods·services·workloads) | 부분 |
| `pods` | `K8sPodItem` (:191) | mapped 4 (name/namespace·status·node(관계)·restarts), dropped 4 (workloadName·workloadType·nodeIP·ip·age) | 부분 |
| `workloads` | `K8sWorkloadItem` (:281) | mapped 4 (name·namespace·type·ready), dropped 4 (updated·available·age·requests·limits) | 부분 |
| `network` | `K8sNetworkSection` | services mapped 4 / dropped 3, ingresses mapped 1 / dropped 3 | 부분 |
| `advancedNetwork` | `K8sAdvancedNetworkSection` | **전 필드 dropped(v2-not-collected)** — Istio·GatewayAPI 수집 자체가 V2 어댑터 스코프 밖 | ✗ **전무** |
| `configStorage` | `K8sConfigStorageSection` | configMaps·secrets·storage 대부분 mapped, dropped 5 (namespaceScope·status·sourceType/path/nfsServer·reclaimPolicy) | 부분 |

### E3.2 종(kind) 수집 차이

`mapping.md` §3.2 비교 스코프 표 (실측 인용):

| 종 | legacy | V2 discoverer | 처분 |
|---|---|---|---|
| node·namespace·pod·deployment·statefulset·daemonset·service·ingress·configmap·secret·pv·pvc | 수집 | 수집 | 비교 집합 |
| **ReplicaSet** | 수집 | **미수집** | `dropped(v2-not-collected)` |
| **Job·CronJob** | 수집 | **미수집** | 동일 |
| storageclass | 미수집 | 수집 | `v2-only` |

→ `/k8s/cluster/detail`을 V2 투영으로 넘기면 workloads 리스트에서 Job·CronJob·ReplicaSet이
**사라진다**. 응답 스키마·내용 양쪽이 바뀐다.

### E3.3 V2 투영이 실제로 반환하는 형상

`backend/internal/infra/inventory/projection.go:43-55` `ProjectedResource`:
`UID · Kind · Subtype · ExternalID · DisplayName · ExternalURN · GenerationUID · ObservedAt ·
Normalized(JSONMap) · Raw(JSONMap)`.

`ProjectResources`(`:67-125`)는 **평평한 리소스 행 목록**을 반환한다 — legacy의 9섹션 중첩
DTO가 아니다. 섹션 조립·파생 카운트(namespace의 pods/services/workloads 수 등)는 어디에도 없다.

### E3.4 술어 (보존 제약 #7 관련)

`projection.go:32`·`projection.go:98` 두 곳이 동일 술어를 쓴다 (실측):

```
"context_id = ? AND status = ? AND committed_at IS NOT NULL"
```

`metrics-ext-plan.md`의 부팅 복원 질의가 같은 술어를 쓴다 — 본 계획은 이 술어를 **변경하지 않는다**.

---

## E4. step 2 — 클라이언트 상태 실측

### E4.1 Service 구조체가 들고 있는 k8s/게이트웨이 상태

`backend/service/service.go:29-55` (실측):

```go
type Service struct {
    ...
    gatewaySSHMu      sync.Mutex
    gatewaySSHClients map[uint]*ssh.Client     // 게이트웨이 SSH 다중화
    k8sOverviewMu    sync.Mutex
    k8sOverviewCache map[uint]k8sOverviewCacheEntry
    k8sOverviewGroup singleflight.Group        // 클러스터 개요 캐시 + singleflight
}
```

`k8sOverviewCacheEntry`(`service.go:57-60`), `New`(`service.go:70`)에서 두 맵 초기화.
TTL: `service/k8s.go:779` `k8sOverviewCacheTTL = 15 * time.Second`.

### E4.2 V2 어댑터와의 관계 — **중복(reimplementation)**, 대체 아님

`backend/internal/infra/adapter/kubernetes/client.go:1-12` 패키지 주석 (실측 원문 인용):

> "The transport is a faithful REIMPLEMENTATION of the legacy semantics in service/k8s.go
> (kubeClusterRuntime / parseKubeConfig / newK8sHTTPClientWithDial / k8sGetJSON) — **not an
> import**: adapters must not reach into the God service (arch rule 2, 계획 J1). Equivalence
> is what the §15 compare protocol proves; **consolidation lands with M2 decomposition step 2**
> (§13 트랜스포트 중복 통합)."

→ 어댑터 자신이 step 2를 자기 통합 시점으로 지목해 뒀다. 판정은 **중복**이며 step 2가 통합
후보다.

### E4.3 통합을 막는 실측 사실

- `service/k8s.go:4341-4360` `newK8sHTTPClientForCluster` — 게이트웨이 모드일 때
  `s.dialThroughGateway(ctx, gatewayID, …)` 를 dialer로 주입한다. 이는 `*Service` 리시버
  = **DB 핸들 + `gatewaySSHClients` 맵**을 요구한다.
- arch rule 2: "adapters do not receive database handles" — 어댑터로 그대로 옮길 수 없다.
- `adapter/kubernetes/mapping.md` §6: "게이트웨이 모드는 `Connection.Config{"connection_mode",
  "gateway_id"}` + 주입 dialer로만 구성(A4) — Phase 2 게이트 증명은 직접 연결, **실환경 홉은 M2**".
  → V2 쪽 게이트웨이 홉은 **미증명**.

### E4.4 게이트웨이 모드 실제 사용 여부 (개발 DB 실측 2026-09-09)

```
mysql> select id,name,connection_mode,ifnull(gateway_id,0) from ops_admin.k8s_cluster;
1  kind-v2-p2  direct  0
2  kind-v2-p3  direct  0
```

```
mysql> select id,uid,provider_type,name,stale_source from ops_admin.provider_connection;
1  b23f3f6ab673c31ead416fe4948a9124  kubernetes  kind-v2-p2   0
2  dbdd3faa256a748f58aa7c4f155e074c  kubernetes  kind-v2-p3   0
3  27cbc4615ed67fba1db252d000797eb4  proxmox     pve-homelab  0
```

→ 개발 환경의 k8s 클러스터 2개는 **전부 direct**. 게이트웨이 경로는 이 환경에서 **미검증
상태이지 고장난 상태가 아니다**. 운영 환경의 게이트웨이 사용 여부는 **미확인**.

### E4.5 R2 게이트 적용 범위 (실측)

`scripts/check-arch-boundary.sh:34` `CORE_PACKAGES`:

```
internal/domain/dnsserver  internal/infra/contract  internal/infra/registry
internal/infra/inventory   internal/infra/secrets   internal/infra/model
internal/infra/policy      internal/infra/metrics   internal/tasks  internal/api/v2
```

→ `backend/service/`도 `internal/infra/adapter/kubernetes/`도 **CORE_PACKAGES에 없다**.
R2(`\b(aliyun|tencent|alicloud|tencentcloud|proxmox)\b` 비테스트 금지)는 두 경로 어디에도
적용되지 않는다. R1은 dnsserver→provider 한정. R3은 opdef 한정. → **step 1·2는 arch-boundary
3규칙 어느 것도 건드리지 않는다.**

### E4.6 의존 방향

- v1 → V2 참조 4행 (실측): `service/integration_finops.go:13,14` (contract·secrets),
  `service/service.go:23,24` (inventory·infra model). `router/router.go`도 `internal/api/v2` 참조.
- V2 → v1 참조: `grep -rn "backend/service" internal/ --include=*.go | grep -v _test` → **0건** (실측).

### E4.7 service/k8s.go 심볼의 패키지 내 교차 사용 (분할 시맨 제약)

| 심볼 | k8s.go 밖에서 쓰는 파일 |
|---|---|
| `parseKubeConfig` | `service/ops_application.go` |
| `k8sGetJSON` | `service/asset_service.go` |
| `cleanupConn` | `service/gateway.go` |
| `dialThroughGateway` | `service/gateway.go`, `service/service.go` |
| `normalizeConnectionMode` | `database.go`·`database_multidb.go`·`gateway.go`·`k8s_terminal.go`·`ops_application.go`·`service.go` (6파일) |
| `PromQueryResult` | `asset_host_metrics.go`, `monitor.go` |
| `gatewaySSHClients`/`gatewaySSHMu` | `service/gateway.go`, `service/service.go` |
| `k8sOverviewCache`/`Mu`/`Group` | `service/service.go` (선언·초기화) |

→ **별도 패키지로 뽑으면** 위 8종을 export하고 6~8개 호출 파일을 고쳐야 한다 —
behavior-neutral by construction이 아니다. **같은 패키지 내 파일 분할**은 호출부 diff 0.

---

## E5. step 2 파일 분할 시맨 — 측정된 라인 범위

`backend/service/k8s.go` 총 **5,062줄** (실측 `wc -l`). 결과 파일 수 = 잔류 1 + 신규 12 = **13**. 심볼 경계(`grep -n "^func \|^type \|^const "`)
기준 제안 시맨:

| 순번 | 파일(제안) | 원본 범위 | 줄 수 | 내용 |
|---|---|---|---|---|
| 1 | `k8s.go` (잔류) | 1–31, 653–1033 | ~412 | 클러스터 CRUD·detail 진입점·노드/파드 상세 |
| 2 | `k8s_types.go` | 32–454 | 423 | kube* 응답/엔티티 타입 |
| 3 | `k8s_types_mesh.go` | 455–651 | 197 | Istio·GatewayAPI 타입 + probe·fetched·metrics 구조체 |
| 4 | `k8s_metrics.go` | 1034–1363 | 330 | 파드/워크로드 메트릭·로그·이벤트·네임스페이스 상세 |
| 5 | `k8s_mutate.go` | 1364–1609 | 246 | YAML create/update/delete·Istio/HTTPRoute 트래픽 |
| 6 | `k8s_workload.go` | 1610–1910 | 301 | 워크로드 상세·scale·restart·images·resources |
| 7 | `k8s_detail.go` | 1911–2494 | 584 | service/istio/ingress/configmap/secret/storage 상세 |
| 8 | `k8s_fetch.go` | 2495–3132 | 638 | `fetchK8sData` + 페치 헬퍼 + 개요 빌더 |
| 9 | `k8s_build_pod.go` | 3133–3650 | 518 | pod/workload 아이템 빌더·엔드포인트·리소스 패치 |
| 10 | `k8s_build_net.go` | 3651–4203 | 553 | 고급 네트워크·Istio/GatewayAPI 빌더·네트워크/스토리지 섹션 |
| 11 | `k8s_transport.go` | 4204–4570 | 367 | probe·`parseKubeConfig`·HTTP 클라이언트·JSON 트랜스포트 |
| 12 | `k8s_path.go` | 4571–5062 | 492 | 리소스 경로 빌더·매니페스트 신원 |
| 13 | `k8s_clientstate.go` | (신규 작성 — 원본 범위 없음) | ~60 (추정) | `k8sClientState` — overview 캐시·singleflight·게이트웨이 SSH 맵 캡슐 + 접근자. `service.go:44-54`의 5필드를 흡수 |

합계 5,061줄 (원본 5,062 — 경계 공백 1) + 신규 캡슐 파일 약 60줄. **추정**: 파일당 package+import 헤더 15~25줄 추가 →
파일별 최종 +약 20줄. 전 파일 ≤800 (최대 `k8s_fetch.go` 약 658). 650 경고선 초과 파일 없음.

**주의(추정)**: 위 범위는 심볼 시작 라인 기준이며, 구현 시 함수 경계 정합을 다시 맞춰야 한다.
최종 수치는 구현이 `wc -l`로 실측해 보고한다.

### E5.1 다른 하드캡 위반 파일 (본 계획 범위 밖)

`wc -l backend/service/*.go | sort -rn` (실측 상위):

```
35779  총계 (service 패키지 전체)
 5062  k8s.go        ← 본 계획 대상
 4798  monitor.go    ← 범위 밖 (OTEL C트랙 후속)
 3049  service.go    ← 범위 밖 (step 2가 필드 5개만 건드림)
 2489  ops_application.go
 2230  database.go
 1783  integration_ai.go
 1336  integration_finops.go
 1242  database_multidb.go
 1197  database_backup.go
 1169  notify.go
 1112  ops.go
 1023  ops_schedule.go
 1016  ops_job.go
 1007  ssl_certificate.go
 1001  domain.go
```

→ 800 하드캡 위반 15파일. 본 계획은 그중 **1건(k8s.go)만** 해소한다.

---

## E6. 관측 수단 실측 (step 5 근거)

### E6.1 읽기 경로 트래픽 — 계수 수단 **없음**

`backend/middleware/operation_log.go:16-51`:

```go
c.Next()
if !strings.HasPrefix(c.Request.URL.Path, "/api/v1") { return }
if c.Request.Method == "GET" || c.Request.URL.Path == "/api/v1/login" { return }   // :25
```

→ **GET은 조기 반환** — `sys_operation_log`에 행이 남지 않는다 (실측). v1 읽기 경로 트래픽을
DB로 셀 수단이 **없다**.

대안 후보와 그 한계:
- `gin.Logger()` (`router.go:44`) — stdout. 배포 환경의 로그 보존·질의 가능성 **미확인**.
  보존 기간·수집기 존재 여부 확인 필요.
- §18.2 메트릭 14종 — 전부 V2 계측점(어댑터/싱크러너/태스크엔진/브로커). **v1 HTTP 라우트를
  세는 메트릭은 0종** (실측 — `internal/infra/metrics/` 렌더러 전수: `provider_health`,
  `provider_api_latency_seconds`, `provider_api_errors_total`, `provider_rate_limit_total`,
  `inventory_sync_duration_seconds`, `inventory_sync_resource_changes_total`,
  `inventory_sync_partial_total`, `provider_task_duration_seconds`, `provider_task_failures_total`,
  `provider_task_retries_total`, `worker_queue_depth`, `worker_lease_expired_total`,
  `resource_stale_total`, `secret_access_total`).

### E6.2 쓰기 경로 트래픽 — 계수 수단 **있음**

`sys_operation_log`는 비-GET `/api/v1/*` 전건에 행을 남긴다. 컬럼: `Method`·`URL`·`StatusCode`·
`Success`·`AdminID`·`Username`·`CreatedAt` (`middleware/operation_log.go:31-47`).

기계 판정 질의:
```sql
SELECT count(*) FROM sys_operation_log
 WHERE url = '/api/v1/k8s/workload/restart' AND created_at >= '<창 시작>';
```

개발 DB 실측 (2026-09-09):
```
select count(*) from sys_operation_log where url like '/api/v1/k8s/%';   -> 0
select count(*),min(created_at),max(created_at) from sys_operation_log;
 -> 27 | 2026-09-05 13:13:27 | 2026-09-08 10:36:49
```

**보존**: 자동 정리·retention 잡 **없음** (실측 — `service/service.go:1204-1213`에
`DeleteOperationLog(id)`·`(ids)`·전체삭제 3종이 있으나 전부 **관리자 수동 호출** 경로).
→ 관찰 창 동안 관리자가 로그를 지우지 않는 것이 전제. 이 전제는 착수 조건에 명문화 필요.

**실패도 기록됨**: `Success` 는 컬럼일 뿐 필터가 아니다 — 403/500도 행이 남는다.
"zero legacy-path traffic"은 **행 0**이지 성공 0이 아니다.

---

## E7. §15 비교 게이트 재실행의 안전성 (보존 제약 #5)

### E7.1 `--gate`의 기본 동작 — newest-3

`backend/internal/infra/inventory/compare.go:1046-1111` `EvaluateGate` (실측):

- `:1076-1078` 아티팩트를 `LegacyCapturedAt` **내림차순 정렬**
- `:1084` `newest := artifacts[:3]` — **최신 3개만** 평가
- `:1089-1091` 3개 전부 `Verdict == pass` 요구
- `:1103-1105` `len(dates) != 3` 이면 실패 — **3개 상이 일자**
- `:1106-1108` 단일 클러스터

→ **bare `--gate`를 재실행하면**, Phase 6 중 새로 만든 pair가 newest-3에 들어가고
같은 날 두 번 돌리는 순간 distinct-days가 깨져 **기록된 PASS가 뒤집힌 것처럼 보인다**.

### E7.2 waiver 경로는 지목한 3개만 본다 — 기존 판정 보존

`compare.go:1143-1215` `EvaluateGateWithWaiver` (실측):

- `:1144` waiver가 없거나 `WaivedCheck != "distinct_days"`거나 `len(Artifacts) != 3` 이면
  → 그냥 `EvaluateGate`
- `:1159` waiver가 **지목한 3개 경로만** 읽어 평가한다 — newest-3를 고르지 않는다
- `:1181` `len(artifacts) != 3` 이면 실패
- `:1188-1191` 지목 3개가 저장 집합에 있어야 하고 전부 pass여야 함
- distinct-days만 면제, 단일 클러스터·verdict·경로형식 검사는 유지
- `:1214` `result.Passed = len(result.Reasons) == 1` (유일한 reason = waiver 기록)

→ **판정**: 2026-09-08 waiver(`v2-phase1/compare-gate-waiver-2026-09-08.json`)와 그것이 가리키는
3개 아티팩트가 디스크에 남아 있고 `--gate-waiver`로 지정해 실행하는 한, **새 비교 실행은 기존
게이트 판정을 무효화하지 않는다**. 반대로 `--gate-waiver` 없이 bare `--gate`를 재실행하면
newest-3 규칙으로 판정이 뒤집힐 수 있다.

### E7.3 각 step 후 비교 재실행 절차 (RESUME.md 승계)

```bash
cd /mnt/d/DEV/acc0mplish/ops-admin/backend
docker ps | grep v2-p2-control-plane
go run . sync-inventory --connection b23f3f6ab673c31ead416fe4948a9124
go run . compare-inventory --cluster 1      # 기대: verdict pass
```
(세 번째 `--gate` 명령은 **재실행 금지** — E7.1. 필요 시 `--gate --gate-waiver <waiver.json>`.)

컨테이너 실측 (2026-09-09): `v2-p2-control-plane`·`v2-p3-control-plane`·`ops-admin-mysql-dev`
전부 Up (실측 `docker ps`).

**함정 (RESUME.md 승계)**: sync가 `unknown v2 master key id k20260906`으로 실패하면
`UPDATE k8s_cluster SET updated_at=NOW() WHERE id=1;` 후 재실행.

---

## E8. CI 잡 전수 ↔ 로컬 대응 명령

`.github/workflows/v2-ci.yml` 잡 10종 (실측). 파일 상단 주석: *"Required checks are registered
per job (branch protection, **post-merge manual step**)"* → **어느 잡이 required인지는 트리에서
도출 불가 — 미확인**. `slice-a-e2e`는 자기 주석에 "Not a required check"라고 적혀 있다 (실측).

| # | 잡 | 로컬 대응 명령 (cwd 표기) |
|---|---|---|
| 1 | `backend-test` | `cd backend && go test ./... -race -count=1` |
| 1b | (같은 잡) opdef coverage latch | `cd backend && go test ./opdef/ -cover` → 커버리지 ≥ 60.0 |
| 2 | `frontend-build` | `cd web && bun install --frozen-lockfile && bun run build` |
| 3 | `migration-test` | `cd backend && go test ./store/ -run TestMigrationFixture -count=1 -v` |
| 4 | `route-coverage` | `cd backend && go test ./router/ -run 'TestRouteInventoryArtifact\|TestSensitiveRoutesArtifact\|TestOperationTableCoversRouter' -count=1` |
| 5 | `route-coverage-canary` | 스크래치 복사본에 미등재 POST 심고 `TestOperationTableCoversRouter` **실패**를 요구 |
| 6 | `secret-scan` | `python3 scripts/secret-scan.py` |
| 7 | `secret-scan-canary` | 스크래치에 카나리 리터럴 심고 스캐너 **비영 종료 + 파일 귀속** 요구 |
| 8 | `arch-boundary` | `bash scripts/check-arch-boundary.sh` |
| 9 | `arch-boundary-canary` | R1+R2 심기 5종(dnsserver import·tencent 주석·aliyun 분기 product/test·inventory case·proxmox product/test) 발화 요구 |
| 10 | `slice-a-e2e` | kind + MySQL 서비스 필요. **non-required** (잡 주석) |

또 하나의 워크플로: `.github/workflows/korean-localization-guard.yml` (내용 미확인 — 본 계획
범위에서 문서 한국어 산출물에만 관계).

---

## E9. §19.1 전제조건 현황 (phase6-entry-assessment.md 편입)

`v2-phase1/phase6-entry-assessment.md` 표 전문을 §0 근거로 편입한다. 요약:

| # | 전제조건 (§19.1) | 상태 | 근거 |
|---|---|---|---|
| 1 | per-family §15.4 shadow 비교 통과 | ✅ (k8s) | 1·2일차 자연 통과 + 3일차 소유자 면제 waiver (`compare-gate-waiver-2026-09-08.json`, PR #51). cloud family는 **범위 제외 확정**(RESUME.md 사용자 결정) |
| 2 | backup + scratch DB 복구 리허설 | ✅ 2026-09-08 | 389KB 전체 덤프 → `ops_admin_rehearsal` 리스토어 exit 0 → 13테이블 COUNT·CHECKSUM 일치 |
| 3 | dual-write authority rule 공개 | ✅ | 스펙 §19.1 본문 (`multi-infrastructure-control-plane-v2.md:1533-1538`) |
| 4 | M1 soak (상위 게이트) | 기술 조건 충족 | §18.2 14종 완료(PR #48). M5 CLI freeze 2026-09-09 만료 |

---

## E10. 확인하지 못한 것 (미확인 목록)

1. **브랜치 보호의 required 잡 집합** — GitHub 설정이며 트리에서 도출 불가.
2. **운영 환경의 k8s 게이트웨이 모드 사용 여부** — 개발 DB는 direct 2건뿐.
3. **배포 환경의 stdout 로그 보존·질의 가능성** — `gin.Logger()` 출력의 수집기 존재 여부.
4. **"one release cycle"의 실제 길이** — 스펙은 "one release cycle per family, minimum"이라고만
   하고 일수를 정하지 않는다. 릴리스 케이던스는 저장소에서 도출 불가.
5. `.github/workflows/korean-localization-guard.yml` 내용.
6. `gin.RouteInfo.Handler`가 최종 핸들러명이라는 것은 E1.2의 **경험적 증명**(미들웨어 붙은
   라우트가 컨트롤러명으로 기록됨)으로 확인했다 — gin 소스 직접 확인은 하지 않았다.
