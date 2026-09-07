# V2 Phase 3 계획 — One Kubernetes mutation end to end (Slice A) (r2)

- 티어 XL · 앵커 `8a97b58` (main) · 스펙 §20 Phase 3·§21 PR 24–26·§25 Slice A·§13·§18.1·§10·§16.2·§17·§4.7/4.9·§23
- 상태: `v2-phase1/p3-state.json`
- CI 방침: 로컬 판정 유지. **승인됨 — CI kind 잡 1개 추가 (2026-09-07)**: 스펙 §20 "green in CI against kind"의 'in CI' 충족·상시 증거용. merge 판정은 로컬 명령 그대로.

## r2 반영사항 (2026-09-07 — 5렌즈 병합 발견)

| 구역 | 반영 |
|---|---|
| **A. 자격 경로 (CRITICAL)** | 판단 J12 신설 — ① 백필 체인에 operations purpose 바인딩 1행 생성(M18, 실측: `inventory/backfill.go:195` `createClusterChain`이 `Purpose:"inventory"`만 생성) ② `TaskPoller.Poll` 시그니처를 `PollRequest{Handle, Connection}`로 확장(M1/M2/M19, 실측: `contract/adapter.go:24`·재폴 `engine.go:826`이 `attempt.HandleRef`만으로 handle 재조립 — 재폴 시 자격 소재 부재). kubeconfig ProviderRef 인코딩·어댑터 DB 주입 기각 사유 J12에 명시 |
| **B. 완료판정식 (HIGH)** | J2 판정식 교정 — deploy `updatedReplicas==spec` 추가·sts `updateRevision==currentRevision`·ds `updatedNumberScheduled` (렌즈2 실측 반증: available/readyReplicas만으론 구 세대 병존 중 롤아웃 미완을 구분 불가). 가정 A2/A8 갱신 |
| **C. 게이트 자산 보호 (HIGH·VK-1)** | 판정 — Phase 3 실클러스터 실측(F/G1)은 **kind 2호기 `v2-p3`**. phase2 게이트 클러스터 `v2-p2`는 무변경 (phase2 게이트 윈도우 운영 지침 — 실측 인용 §0.2a). 계획 내 `--context kind-v2-p2` 전부 교정(§5·N16·claims 6/7) |
| **D. 회복시간·엔진 (HIGH)** | J10 "회복 ≤10s"·§0.7 "수초" 삭제 → 실측 하한식 `max(LeaseSeconds, CallTimeout) + 2×ReaperGrace + 틱` (기본값 ~36s). TimeoutSeconds 런타임 오버라이드 기각(근거 J10). F-4 deadline 재서술(실측: 폴 사이클은 리스 미갱신 — §3.6a). VK-7 E2E 전용 스키마·F-7 셧다운 순서(M9)·VK-8 Start 설정 로그 반영 |
| **E. 검증 계약 (HIGH)** | claim 6 절대값→증분·claim 8 조건부 non-null·claim 9 순서 문언·claim 11 잔여 0 보강·J11 compare-inventory 스텝(§25 "+§15 comparisons passing" 세그먼트)·Playwright 브라우저 설치+webServer·**잡 수 정정: 기존 9잡·10번째 추가**(실측 §0.2b — r1 "8잡" 오기)·보존 제약 #8 갱신. L1 run-fixture.sh(N19)·L3 측정 기구(kubectl 서브프로세스·RS creationTimestamp) 명시 |
| **F. 설계·보안 잔여 (MEDIUM)** | J6 `OnTaskSucceeded`→`OnTaskTerminal`·VK-4 정책 에러 403 fail-closed·VK-3 server URL 기록+loopback 단얫·VK-10 누출 스캔 대상 확대·VK-11 request_summary 해시/코드만·M3 메뉴 시드 구현(M20 — 스펙 §17.3 준수, 웨이버 철회)·M2(완전) 탈락 4건 §13 재등재·M4 커버리지 ≥80% claim 13·F-8 nullable+JSON 정규화·F-5 "next claim boundary" 정정(스펙 §13.4 문언)·VK-12 dirty runbook·VK-9 잔존 태스크 거동·L1 Idempotency-Key 축소 근거·L2 재생 200(스펙 §13.4 실측)·L7 N-4 원 정의 참조·F-9 이중 검색 한계 주석·PolicyInput.Tenant 제거(§10.3 어휘 실측)·R6 ds 시드 추가(N17)·R3 완화 실체화·§4/§5 표기 통일(cancel·fixture 파일 명시) |

무결 판정부 미변경: §0 실측 방식·게이트 ② 측정 기구(§0.1)·승인 체인(에스컬레이션 없음)·J9 2층 분리·J3 최소설계(EXTEND+JSON)·신규 의존성 0.

## 게이트 실현가능성 판정 (요약 — 최상단 의무)

| 게이트 (§20 Phase 3) | 판정 | 근거(§0 실측) | 전제 |
|---|---|---|---|
| ① Slice A 트레이스 §25 green in CI against kind | **실현 가능** | CI는 매 실행 신규 kind 클러스터 자체 구축(J11) — 라이브 클러스터 무의존. 엔진 실행 회로 완비(§0.4) — 부재분(operations 바인딩·Connection 전달·k8s executor·API 5종·엔진 main 배선)은 전부 계획됨 | CI 잡 절차 자동화(G2)·**시드 fixture 커밋** (§0.2 — 현재 리포에 없음) |
| ② crash-injected run 회복·이중 restart 부재 — **클러스터에서 단얫** | **실현 가능** | restart = pod template `restartedAt` patch → `metadata.generation` 증가·신규 RS 1개로 **클러스터 측 정량 단얫 가능**(§0.1)·리퍼 회복 경로 PR 19에서 이미 GREEN | 동결 restartedAt 멱등(판단 J1). E2E는 **kind 2호기 v2-p3**에서 실행(§0.2a — v2-p2 게이트 자산 보호) |
| ③ 감사 레코드 §18.1 완전 | **실현 가능하나 확장이 먼저** | sys_operation_log에 task_uid·policy_version·mutating·V2 맥락 필드 전부 부재·OperationLog 미들웨어는 `/api/v1`만 기록(§0.5) | step0005 EXTEND(§3.2 row 2 준수) + V2 mutation 전용 감사 기록 경로 |

에스컬레이션 필요 항목(실측 불가·사용자 승인 대상): 없음. 단 판단 J4(승인 동사 권한 `ops:job:approve` 재사용)·J8(restart opdef `RequiresApproval=true` posture)은 계획 승인 시 확인 요망 — 둘 다 되돌림 1줄짜리 코드 값이다.

---

## 0. 현재 코드·자산 실측

### 0.1 restart의 클러스터 측 단얫 수단 (게이트 ②의 측정 기구)

- v1 구현(`service/k8s.go:1700` RestartK8sWorkload)은 pod template에 `kubectl.kubernetes.io/restartedAt: <RFC3339>` 어노테이션을 strategic-merge-patch. 이것이 롤아웃을 유발한다.
- **롤아웃 restart는 container `restartCount`를 증가시키지 않는다** (pod 교체이지 in-place 재시작이 아님) — 스펙의 "restart count asserted from the cluster, not from logs"의 정확한 착지는 다음 3종 단얫이다:
  1. `deployment.metadata.generation` 증분 == 1 (spec 변경 시에만 증가 — 동일값 재 patch는 무변화)
  2. 태스크 시작 이후 생성된 ReplicaSet(신규 revision) 정확히 1개
  3. 현행 pod template의 `restartedAt` 값 == 태스크가 동결한 값
- 이중 restart = generation +2 또는 상이한(더 늦은) restartedAt — 전부 클러스터 상태만으로 기계 판별. 실측 기준선: `v2-seed/seed-nginx` generation=1, 어노테이션 없음(2026-09-07 확인 — v2-p2 관측치. Phase 3 실행 기선은 v2-p3 스냅샷, claim 6/7).
- **측정 기구의 구현 형식 (L3)**: E2E·리허설의 단얫 1·2는 `kubectl --context kind-v2-p3 -n <ns> get deploy <name> -o json` / `get rs -o json` **서브프로세스 1회 호출로 JSON을 채취해 Go에서 판정** — 신규 RS 판별은 어노테이션·이름 접두(알파벳 순 최신)가 아닌 **`metadata.creationTimestamp` > 테스트 시작 시각** 기준(롤백이 이전 RS를 재활성화하는 k8s 시맨틱에서 이름 순서는 증거가 아니다). generation 증분은 (실행 전 스냅샷, 종단 후 스냅샷) 2회 채취의 차로 산출 — 절대값 미의존(재시딩·재실행 누적에 견고).

### 0.2 kind 클러스터·시드·DB 실측 · 0.2a 게이트 자산 보호 (r2 — C 판정) · 0.2b CI 잡 수 (r2 정정)

```text
클러스터  kind v2-p2 (container v2-p2-control-plane, kindest/node:v1.34.0, 127.0.0.1:38407)
시드      ns v2-seed: deploy/seed-nginx(replicas 2, generation 1)·sts/seed-sts(1)
          ns default: deploy/seed-default-web(1) · kube-system 3종(coredns/kindnet/kube-proxy)
시드 소스  리포에 매니페스트 없음 (find seed.yaml 0건) → CI 잡·로컬 재현 모두 커밋된 fixture 선행 (Phase F·N17)
DB        provider_connection 1행: uid b23f3f6ab673c31ead416fe4948a9124, kubernetes,
          kind-v2-p2, source k8s_cluster/1, active. provider_context 1행(cluster).
          infra_resource 51행(workload 7행 — urn:k8s:1:workload:v2-seed/deployment/seed-nginx 등)
credential_binding  inventory 1건뿐 — operations 바인딩 부재. 실측(r2): 생성 소스는
          inventory/backfill.go:195 createClusterChain — SecretRef 생성 후
          ProviderCredentialBinding{Purpose:"inventory"} 1행만 Write (§7.4 어휘 "operations" 미사용)
```

**0.2a — Phase 3 실클러스터 실행 환경: kind 2호기 `v2-p3` (판정, VK-1)**

- phase2 게이트 ①(페어 비교 3일 창)의 측정 자산은 클러스터 v2-p2 그 자체다. phase2 계획의 게이트 윈도우 운영 지침(§0.1) — 실측 인용: "**창 중 kind 클러스터 재생성 금지**"(identity 집합 변동 = 구조적 BLOCKER)·"**시드 매니페스트 변경(리소스 증감)도 동일 취급**". Phase 3 트레이스의 restart는 generation·RS·pod 전면을 변동시키는 mutation — v2-p2에서 실행하면 그 자체로 게이트 아티팩트 오염이다.
- 따라서 Phase F(리허설)·G1(Go trace E2E)·claim 6/7은 전부 **`kind create cluster --name v2-p3`** 2호기에서 실행 (N17/N18/N19 fixture 절차 준용 — 시드·v1 등록(kubeconfig)·백필·sync). v2-p2는 phase2 게이트 종결까지 무변경.
- CI 잡(J11)은 매 실행 고유 클러스터를 생성·파기하므로 본 판정과 독립 — 무관.

**0.2b — v2-ci 잡 수 정정 (r2)**

실측 `grep -nE "^  [a-zA-Z0-9_-]+:" .github/workflows/v2-ci.yml` (2026-09-07): backend-test·frontend-build·migration-test·route-coverage·route-coverage-canary·secret-scan·secret-scan-canary·arch-boundary·arch-boundary-canary = **기존 9잡**. r1의 "기존 8잡"·"9번째 잡" 표기는 오기 — slice-a-e2e는 **10번째 잡**. 보존 제약 #8 갱신.

### 0.3 v1 restart 구현·권한 (§3 disposition row 11 — K8sCluster SUPERSEDE, M2까지 v1이 권위)

- `service/k8s.go:1700` RestartK8sWorkload — deploy/statefulset/daemonset 3종 경로, `time.Now()`로 매번 새 restartedAt. **무멱등** (재호출 = 재롤아웃). controller `k8s.go:386`, 라우터 `router.go:488`.
- 권한: `assets:k8s:workload:restart` (opdef `defs_monitor.go:83`, 4세그 — registry permissionPattern `^[a-z0-9_-]+(:[a-z0-9_-]+){1,3}$` 충족). **이미 시드된 권한 메뉴 — 재사용 시 신규 시드 0.**
- 승인 동사 권한 관례: v1 OpsJob approve/reject가 동일 권한 `ops:job:approve` 공유(`defs_ops.go:39-40`). 취소(cancel) 권한은 v1에 부재(실측 grep 0건).
- route-coverage 게이트: `authz_replay_test.go:25` `apiPrefix="/api/v1"` — **/api/v2 non-GET은 현재 게이트 사각**. baseline non-GET 240·sensitive-routes.txt 285엔트리·route-inventory.txt 444라인(골든, 재생성 필요).

### 0.4 엔진·계약 갭 실측 (임계경로의 실체)

- `tasks.Engine` 실행 회로 완비: Submit(멱등 리플레이 포함)·ClaimNext(CAS)·executeClaimed(체인 조인→registry→브로커→Execute→async Poll)·pollAsyncAttempts·ReapOnce·Approve/Reject·상태머신 9종. **main.go 배선 부재** — state.go 주석 "main.go wiring is Phase 3's (plan A6)".
- **N-1 잔여의 정확한 위치**: `engine.go:716` `if _, err := e.connectionView(ctx, chain); err != nil` — 뷰를 조립해놓고 **결과를 버린다**. `contract.OperationRequest`(adapter.go:108)에 `Connection` 필드 자체가 없다(주석 "Phase 3 인계").
- **재폴 경로의 자격 공백 (r2 실측 — J12의 근거)**: 첫 폴은 executeClaimed가 조립한 handle로 `poller.Poll(execCtx, handle)`(engine.go:760). 재폴 `pollAsyncAttempts`는 **DB의 `attempt.HandleRef`만으로 handle을 재조립**해 `poller.Poll(ctx, OperationHandle{ProviderRef: attempt.HandleRef})`(engine.go:826) — 이 경로에는 Connection이 아예 없고, 어댑터는 stateless("HTTP clients are per-request")다. 크래시 후 attempt 2의 재폴이 자격을 스스로 확보할 수 없다. 단 재폴 루프는 이미 `resolveExecutionChain(ctx, task.ResourceUID)`(engine.go:810)을 호출 중 — ConnectionView 재조립에 필요한 체인 조인이 이미 존재한다 (전파 비용 미미).
- k8s 어댑터: BaseAdapter+Discoverer만 구현(adapter.go:303 compile-time 단얫). Execute/Poll 부재. `k8sClient`에 getJSON만 있음 — patch 부재.
- **registry에 operation이 0건 등록** (compose.Build는 provider 2종만 등록, RegisterCapabilities/RegisterOperation 호출 부재 — kubernetes는 capability도 미선언). engine Submit은 미등록 op 하드에러 → 현재 어떤 V2 mutation도 제출 불가.
- registry V5: `orchestration.kubernetes.apply` capability 등록은 OperationExecutor 구현을 요구(매핑 표) — capability 등록과 executor 구현은 같은 Phase.

### 0.5 감사·요청 맥락 실측

- `middleware/operation_log.go:25`: `if !strings.HasPrefix(c.Request.URL.Path, "/api/v1") { return }` — **/api/v2/infra POST는 현재 감사 0건**.
- sys_operation_log 컬럼 실측(§0.2 DB): admin_id·username·method·ip·url·description·risk_level·status_code·success·duration_ms·request_summary·created_at — **task_uid·policy_version·mutating·request_id·trace_id·resource_uid 등 §18.1 필드 전부 부재**. §3.2 row 2 EXTEND는 Phase 3 소유.
- **request_summary 현행 처리(r2 실측 — VK-11 근거)**: v1 미들웨어는 요청 본문을 그대로 실어 저장(`RequestSummary string, gorm:"type:text"`). V2 mutation 요청 본문은 kubeconfig 등 자격 물질과 실행 payload를 담을 수 있어 **v2 레인에서 본문 직렬화 금지** — 해시·에러 코드만(§3.5).
- 요청 맥락(§16.2): X-Request-ID·traceparent를 다루는 미들웨어 부재. Idempotency-Key 처리 계층 부재(엔진 멱등은 구현됨 — API 전달만 부재).

### 0.6 프론트·E2E 실측

- `web/package.json`: playwright **devDependency 존재**(`^1.63.0`) — 테스트 스크립트·설정·디렉터리 전무. 신규 의존성 아님(보존 제약 #4 정합). **브라우저 바이너리는 미설치** — J11/N14에 설치 스텝 포함(M2 검증).
- infra 뷰 3종(InfraOverview 89줄/InfraProviders 77/K8sInventory 125)·`api/infra.js` GET 4종·라우터 additive·i18n `infra-i18n.js` 2로케일(ko-KR/en-US, parity 게이트).
- 로그인: `/api/v1/login`(JWT) — E2E 인증 경로. 메뉴 DB 시드(store/seed.go — §17.3).

### 0.7 엔진 시간 파라미터 (r2 — 회복 시간의 실측 하한식)

- `Config{WorkerID, PollInterval(기본 2s·§13.1), LeaseSeconds, ReaperGrace}` — main 배선 시 값 필요. T-8 하한(engine.go:431 실측): `lease = now + max(LeaseSeconds, CallTimeoutSeconds) + ReaperGrace`.
- **r2 교정 — 회복 시간은 "수초"가 아니다**: 크래시(SIGKILL) 후 종단까지의 최단 경로는 ① 리스 만료 `max(LeaseSeconds, CallTimeout) + ReaperGrace` 경과 → ② 리퍼의 재큐 조건 `lease_expires_at < NOW() - ReaperGrace` 충족(Grace 2회차) → ③ 다음 폴 틱에서 재큐·재클레임. 즉 **회복 하한식 = max(LeaseSeconds, CallTimeoutSeconds) + 2×ReaperGrace + 틱** — 제안 기본값(Lease 30s·CallTimeout 30s·Grace 2s·틱 2s) 기준 **~36s**. r1의 "회복 ≤10s"·"수초" 서술은 삭제. E2E는 이 식으로 회복 예산을 잡는다(종단 대기 상한 120s) — CI 잡 15m 예산에 무해.
- **폴 사이클은 리스를 갱신하지 않는다**(r2 실측 — §3.6a·F-4): 실행 중 attempt의 리스는 클레임 시점 하한에서 고정. 슬로우 롤아웃(이미지 풀 지연 등)로 폴이 계속 Running이어도 리스가 연장되지 않는다 → 리스 만료 후 재큐(멱등 patch라 클러스터 무해)·attempts 소진 시 `lease_expired` 종단. 종단 보장 메커니즘은 이 경로뿐 — H1의 이미지 사전 pull이 완화(R12).
- Phase 1 crash 스위트는 결정적(리스 강제 만료) — 프로세스 SIGKILL 크래시 인젝션은 **계층 2로 미실시**(crash_suite_test.go 서문). 게이트 ②의 "crash-injected run"은 Phase 3가 실프로세스 증명을 소유한다.

---

## 1. 아키텍처 판단 (선택지 비교 + 선정 근거)

### J1 — 이중 restart 방지: provider 수준 멱등 (동결 restartedAt) 〔임계〕

문제: 크래시 → 리퍼 requeue → attempt 2 재실행이 동일한 PATCH를 재발행하면 **재롤아웃(이중 restart)** 이 된다. 태스크 수준 멱등(unique idempotency key)은 중복 제출만 막고 중복 실행은 못 막는다.

| 선택지 | 판정 | 근거 |
|---|---|---|
| (a) 엔진 exactly-once 보장 | 기각 | 단일 프로세스에서도 claim→Execute→commit 사이 크래시 창은 근본적으로 존재 — §13이 'crash recovery'를 목표로 삼는 이유 |
| (b) 실행 전 GET으로 기존 어노테이션 비교 후 skip | 기각 | TOCTOU race + 왕복 1회 추가 + "어느 시점 값과 같아야 하는가" 판정 규칙이 오히려 복잡 |
| **(c) restartedAt 값을 plan/submit 시점에 서버가 동결해 payload에 심음** | **선택** | 재실행이 **byte-identical patch** → k8s API가 spec 무변화로 판정 → generation 무증가 → 새 revision 없음 → 재시작 없음. 부수 효과: 게이트 ② 단얫 3(어노테이션 값 == 동결값)이 자연 발생 |

v1(`time.Now()` 매번)과의 차이는 V2 레인에만 적용 — v1 무변경. 어노테이션 값 출처는 **plan 응답이 생성·execute가 그대로 전달**(클라이언트가 임의값을 심는 것 금지 — execute 핸들러가 값의 존재·RFC3339 형식만 검증하고, 재수집 여부는 plan 경유 계약상 문서화).

### J2 — Execute/Poll 형태: 비동기 듀얼 모드 (§14.3), 상태 없는 ProviderRef 〔임계〕

- Execute: PATCH 1회 → `OperationHandle{ProviderRef: "rollout|<connUID>|<ns>|<kind>|<name>|<expectGeneration>"}` 반환. expectGeneration = patch 응답의 metadata.generation. connUID 인코딩 근거는 J12.
- Poll: GET 대상 워크로드 → 완료 판정 → Succeeded/Running. ProviderRef는 전부 인코딩된 상태 + 폴 자격은 엔진이 매번 조립해 주입(J12) → 프로세스 크래시 후 재시작해도 attempt 2가 동일 handle·동일 자격으로 재폴 가능.
- **완료 판정식 (r2 교정 — 렌즈2 실측 반증)**: 구 세대 pod가 병존하는 롤아웃 진행 중에도 `availableReplicas`/`readyReplicas`가 기대값을 만족할 수 있다(구 RS pod가 카운트에 포함). 세대 일관성까지 확정하는 판정식으로 교정한다:
  - **deploy**: `status.observedGeneration >= expect && status.availableReplicas == spec.replicas && status.updatedReplicas == spec.replicas` — updatedReplicas가 신규 RS가 전체를 대체했음을 확정.
  - **sts**: `status.updateRevision == status.currentRevision && status.readyReplicas == spec.replicas` — sts는 구 revision pod가 ready 카운트에 포함되므로 revision 수렴이 롤아웃 종결 신호.
  - **ds**: `status.updatedNumberScheduled == status.desiredNumberScheduled` — numberReady는 구 세대 파드 포함.
- 완료 판정 실패는 Running 유지(다음 폴). 태스크 종단은 리스/재시도 메커니즘이 지킨다(§3.6a — CallTimeout은 폴 1회의 실행 예산, 태스크 deadline 서술은 폐기).
- **교정 판정식과 킬 창 결정성 (r2)**: 교정식은 롤아웃이 실제 진행 중(`updatedReplicas < spec`)일 때만 Running을 유지한다. H1의 poll-interval 확대로 폴 관찰 창이 롤아웃 진행 창과 겹치게 되면, SIGKILL 시점의 태스크 상태가 (진행 중=running 확정 / 완료=종단 확정)으로 **결정적**이 된다 — r1 판정식(available만)이 롤아웃 완료 직후에도 Running을 유지해 킬 타이밍과 무관하게 통과하던 모호성 제거.

### J3 — 감사 확장: sys_operation_log EXTEND (step0005), V2 전용 기록 경로

- §3.2 row 2 지시 그대로: **신규 테이블 금지**. 신규 컬럼 4개 — `mutating`·`task_uid`·`policy_version`(§3.2 named 3종; risk_level은 기존 v1 컬럼 재사용) + §18.1 전 필드를 담는 `v2_context` JSON 컬럼 1개(쿼리 빈도가 낮은 식별 맥락 — request_id·trace_id·connection/context/resource uid·operation·operation_version·approval_status·approver·provider_task_id·error_code·request_hash·result_hash).
- 절충: 전 필드를 컬럼화하면 ALTER 15+열 — 단일 운영자 도구에서 그 쿼리 다양성은 과투자. task_uid만 실컬럼(태스크↔감사 조인용) + JSON 나머지. §18.1 "완전" 증명은 레코드 JSON 필드 존재 단얫(claim 8)으로 기계화.
- 기록 경로 2곳: (a) **V2 mutation 요청 감사**(execute/approve/reject/cancel 각 1행 — `/api/v2` 전용 audit 미들웨어·v1 OperationLog 미들웨어 무변경), (b) **태스크 종단 감사**(엔진 종단 확정 시 result_hash·error_code 포함 행 — 엔진 훅이 작성, url=`task://<uid>` 규약). 두 행이 task_uid로 조인된다.
- **r2 — F-8 두 가지 명시**: ① 신규 컬럼 4종은 전부 **nullable** (기존 v1 감사 행이 백필 없이 NULL을 유지 — NOT NULL 지정 시 마이그레이션이 기존 행에 기본값 강제). ② `v2_context`는 MySQL JSON 컬럼로 저장 시 키 순서·공백이 **정규화**된다 — 감사 단얫(claim 8)은 바이트 비교 금지, Go map 디코딩 후 **키별 존재·값 단얫**으로 기계화.

### J4 — 권한 어휘: 기존 2종 재사용, 신규 권한 문자열 0

- restart opdef `RequiredPermission = "assets:k8s:workload:restart"` (§10.3 "role grants carry over" — 이미 시드됨).
- V2 태스크 approve/reject/**cancel** = `ops:job:approve` 재사용(§13.3 OpsJob 재사용 정신 — 승인자 인구 동일·신규 시드·리플레이 재검증 0).
- 절충 명시: cancel까지 approve 권한에 두는 것은 어휘 순도를 낮춘다. 단일 운영자 도구에서 신규 권한의 시드·G-4 리플레이 비용이 가치를 초과한다. 세분화(`infra:task:*` 분리)는 다중 운영자 요구 시 이월 대장 항목으로 기록.
- plan/execute 라우트는 **동적 권한**: URL `{name}` → registry opdef → `def.RequiredPermission` → 기존 `middleware.RequirePermission` 위임(opdef.Middleware가 유일 경로라는 원칙의 V2 확장 — opdef 패키지에 V2 동적 헬퍼 추가).

### J5 — 정책(§10.3 policy_evaluate): 내장 최소 엔진, policy_version 문자열

- 신규 패키지 `internal/infra/policy`(§6 모듈 목록의 policy). **r2 — Tenant 필드 제거**: 스펙 §10.3의 policy_input 어휘는 `context/kind/environment/risk`이고 tenant는 §16.2에서 "constant default (server-set; no client header in M1)" — 정책 입력이 아니라 서버 고정 맥락이다. `PolicyInput{ProviderType, ContextKind, ResourceKind, Risk, Mutating}` → `Decision{Allow, PolicyVersion}`.
- M1 규칙집합 1개: `builtin:default-allow` (전부 허용, 판정 근거 문자열 반환). deny 어휘·환경 차단은 계약만 정의(§5.5 언어 그대로 — 실제 차단 규칙은 요구 발생 시). **정책 테이블 신설 없음**(§22-16).
- authorize 합성(`role_has_permission AND policy_evaluate`)은 plan/execute 핸들러에서 명시적 2단계로 구현 — 감사 policy_version은 이 판정의 것.
- **r2 — VK-4 fail-closed**: `Evaluate`가 에러를 반환하면 핸들러는 **403으로 거부**한다(허용 아님). 근거: 최소 허용 엔진에서 "평가 불가=통과"는 정책 계층의 부재를 무효화한다. D1 음성 케이스로 기계화(§6).

### J6 — 관찰 갱신·종단 감사: 엔진 종단 훅 (DI, 동기) — r2 명칭 교정

- `Engine`에 선택 필드 `OnTaskTerminal func(ctx, task, status, detail) error`(**r2: `OnTaskSucceeded`→`OnTaskTerminal`** — Complete/Fail 종단 공통 발화. nil 가능 — 엔진 테스트 무영향). r1의 성공 전용 훅은 **실패 종단 감사행이 빠지는 구멍**이었다(감사 계약은 성공·실패 모두의 종단 레코드를 요구).
- compose/main이 훅에 `SyncRunner.RunSync(ConnectionUID)` 램핑(관찰 갱신 — 실패 종단에서도 관찰은 부분 롤아웃의 실상을 반영하므로 상태 무관 실행) + 종단 감사 행 작성을 배선(전체 커넥션 1회 — kind 규모에서 충분·Incremental은 Phase 4 이월).
- 엔진은 inventory를 import하지 않는다(계층 결계 유지). 훅 실패는 태스크 종단을 되돌리지 않는다 — `task_event("observation_refresh_failed")` append + 로그. 훅은 Complete/Fail 커밋 **이후** 호출(종단 불변).
- 트레이스의 "refreshed observation" 증명: E2E가 태스크 종단 후 `GET /resources/{uid}` 최신 관찰의 generation/restartedAt 갱신을 단얫.
- cancel/reject 계열 종단(approve·reject·cancel의 즉시 종단)은 API 요청 감사 행(N9 미들웨어)이 이미 기록하므로 훅 범위에서 제외 — 감사 구멍 없음.

### J7 — 리소스 개정(§20 pipeline의 "resource revision")의 해석

- 명시 해석: **plan이 최신 관찰의 k8s revision(generation)을 스냅샷해 `resourceRevision`으로 반환, execute payload에 동결, 실행 결과 detail·감사 request_hash/result_hash에 반영**. 신규 컬럼·테이블 0(§8.3 Managed state는 M1 범위 밖 — plan 문서의 원 문장이 새 스키마를 요구하지 않는다).
- 기각 해석: (a) "리소스 행 revision 컬럼 신설" — §7.7/§22-16 정신 위반, 관찰(refreshed observation)이 이미 개정 정보를 담음. (b) "plan↔execute 낙관적 잠금(resourceRevision 불일치 거부)" — Slice A에 과설계, 이월 대장.

### J8 — 승인 posture: `RequiresApproval = true` (V2 레인 한정)

- 근거: V2 첫 프로덕션 mutation의 신중 posture + §25 트레이스("policy and optional approval")·§23.1 e2e("view → restart → approval → task result")·§18.1(approval_status/approver 실증)가 **승인이 실제로 발화할 때만 의미 있는 증거**를 요구. 승인이 한 번도 아니면 감사 approval 필드가 not_required로만 채워진다.
- 절충: 운영 클릭 +1. v1 레인은 무승인 그대로(§3.3). posture 변경은 정의 1줄(version bump 관례) — 되돌림 단순.

### J9 — E2E 2층 분리: Go trace 테스트(크래시 인젝션) + Playwright(UI 흐름)

- **Go trace 테스트**(build tag `e2e`): 백엔드 바이너리를 자식 프로세스로 관리 — API login → plan → execute → 승인 → task running 관찰 → **SIGKILL** → 재기동 → 종단 대기 → 클러스터 단얫 3종(§0.1) + 리퍼 이벤트(reaper_requeued) + 감사 SQL 단얫. 프로세스 제어·크래시 타이밍은 Go가 결정적.
- **Playwright**(§23.1 e2e tier): UI 흐름(재고 뷰 → 워크로드 → restart plan 다이얼로그 → execute → 승인 → 태스크 상세 → 결과) — 무크래시. 크래시는 Go 층이 이미 증명.
- 기각: 단일 Playwright 메가 테스트(브라우저 테스트가 SIGKILL 타이밍까지 소유) — 롤아웃 진행 창을 노리는 플레이크 원천. 두 테스트가 같은 kind 스택을 향해 §25 트레이스를 분담 증명한다.

### J10 — 엔진 설정: config 확장 + 환경변수 오버라이드 (r2 — 회복 시간 서술 교정)

- `config.yaml`에 `engine:` 섹션(worker-id·poll-interval-ms·lease-seconds·reaper-grace-ms) 추가 — `config.example.yaml` 갱신·기본값은 스펙 값(폴 2s·lease 30s·grace 2s). E2E/CI는 `OPS_ADMIN_ENGINE_*` 환경변수로 PollInterval·LeaseSeconds·ReaperGrace 오버라이드. T-8 하한 식은 그대로 존중.
- **r2 — "회복 ≤10s" 목표 삭제**: §0.7 하한식이 지배한다(기본값 ~36s). 종단 대기 예산 120s로 E2E가 회복을 증명.
- **r2 — opdef TimeoutSeconds 런타임 오버라이드는 기각**: TimeoutSeconds는 opdef 계약 값(감사·재시도 산수의 입력)이며 런타임 덮어쓰기는 정의-실행 정합을 깬다. 회복 시간은 리스/그레이스 오버라이드로만 단축하고, CallTimeout이 회복 하한을 지배하는 구조 자체가 T-8 계약이다. E2E에서 회복이 느려도 CI 15m 예산에 무해(실측 근거 — 하한식 36s × 최악 경로 포함 여유).

### J11 — CI kind 잡 형태 (승인분 — v2-ci **10번째** 잡 `slice-a-e2e`)

- runs-on `ubuntu-latest`·timeout 15m. 절차: setup-go/bun → **`bunx playwright install --with-deps chromium`** (브라우저 바이너리 — M2 검증) → kind create(config 고정) → **시드 nginx 이미지 사전 pull 후 노드 로드**(`docker pull nginx:<pin>` + `kind load docker-image` — 슬로우 롤아웃·리스 만료 리스크 R12 완화·H1) → MySQL service container → 커밋된 fixture 시딩(kubectl apply — N17) → v1 등록+백필+sync(`sync-inventory` 서브커맨드 재사용 — CLI가 이미 전부 수행) → **compare-inventory 캡처+비BLOCKER 판정 스텝** (`./ops-admin compare-inventory --cluster <id>` exit 0 — §25 트레이스 첫 세그먼트 "complete shadow inventory generation (+ §15 comparisons passing)"의 CI 증거; 아티팩트 업로드) → `go test -tags=e2e ./e2e/slicea/`(Go trace) → **Playwright**: `playwright.config.js` webServer 구성으로 dev 서버 기동·테스트 후 정리(N14 — 스택 기동을 잡이 소유). 병렬 회피(클러스터 1개)·path filter 없음(기존 관례). **merge 판정은 로컬 재현 명령으로**(§10 방침). k3d 주간 잡은 스펙 문언 유지·도입 안 함(선택 사항).

### J12 — 자격 경로 완성 (r2 신설 — CRITICAL: operations 바인딩 + Poll 자격) 〔임계〕

두 개의 독립적 공백이 실행 회로를 막고 있다 — §0.2·§0.4 실측.

**(1) operations purpose 바인딩 생성 — 백필 체인 1행 (M18)**

- 공백: `createClusterChain`(inventory/backfill.go:195)은 SecretRef 1개를 만들고 `Purpose:"inventory"` 바인딩 1행만 Write한다. 브로커의 `Resolve(operations)`는 영구 실패 — executor가 Material을 받을 수 없다.
- 선택: **동일 SecretRef를 향하는 `Purpose:"operations"` 바인딩 1행 추가** (§7.4 어휘 `inventory, operations, …`·동일 절 "bindings with purposes inventory and billing pointing at the same SecretRef"의 정확한 전례 — kubeconfig는 인벤토리·오케스트레이션이 공유하는 자격).
- 기각: (a) API/마이그레이션으로 운영자가 수동 생성 — 백필이 전파 규칙(§5.4)인 이상 자격 체인도 동일 규칙의 소유다. 수동 단계는 재현성을 깬다. (b) purpose 무시(브로커가 inventory 바인딩을 operations로 전용) — §7.4 다중 purpose 모델 자체를 무효화.
- 파급 통제(실측): `refreshClusterSatellites`의 바인딩 조회는 `purpose = "inventory"`로 필터(backfill.go 실측) — operations 행 추가에 무영향. 백필 재실행(증분)·stale_source 경로도 무영향.

**(2) Poll 자격 — ProviderRef 자기서술 + PollRequest 확장 (M1/M2/M19)**

```go
// contract/adapter.go (r2)
type PollRequest struct {
    Handle     OperationHandle
    Connection ConnectionView // §7.2 — 엔진이 매 폴마다 조립 (Execute의 Material 대칭)
}
type TaskPoller interface {
    Poll(ctx context.Context, req PollRequest) (OperationStatus, error)
}
```

- ProviderRef 형식에 connection UID 인코딩: `rollout|<connUID>|<ns>|<kind>|<name>|<expectGeneration>` — handle은 `task_attempt.handle_ref`에 **지속되는 값**이므로 어느 커넥션을 향하는지 자기서술적이어야 한다(감사·디버깅·오남용 추적). UID는 공개 식별자(비밀 아님 — 보존 제약 #7 무관).
- 엔진 조립 지점 2곳(둘 다 브로커 Resolve 경유 — 어댑터는 자격을 스스로 획득하지 않는다): ① executeClaimed 첫 폴(engine.go:760) — 이미 조립한 connectionView 재사용 ② pollAsyncAttempts 재폴(engine.go:826) — 이미 호출하는 `resolveExecutionChain`(810)에서 connectionView 조립으로 확장.
- 기각 사유 명시:
  - **kubeconfig의 ProviderRef 인코딩 — 기각**: handle은 DB 컬럼·감사·로그에 노출되는 지속값 — 자격 물질의 인코딩은 보존 제약 #7 직접 위반이다.
  - **어댑터에 DB 주입(재폴 시 스스로 체인 조회) — 기각**: arch rule 2(어댑터는 제어평면 테이블 직접 접근 금지 — `scripts/check-arch-boundary.sh` 검사 대상) 위반·stateless 어댑터 원칙("HTTP clients are per-request") 붕괴.
- 테스트: fake(`adapter/fake/fake.go:204`) 시그니처 전파 + PollRequest.Connection.UID == handle 인코딩 UID 정합 단얫(N11) + 재폴 경로 자격 주입 단얫(크래시 시뮬레이션 — 재폴이 Material을 받는다).

---

## 2. 수정 대상 (배치표 — 전수)

### 신규 (19)

| # | 파일 | PR | 내용 |
|---|---|---|---|
| N1 | `backend/internal/infra/adapter/kubernetes/executor.go` | 24 | Execute/Poll(PollRequest 소비)·URN 파서·patch 조립·완료 판정 3종(r2 교정식)·대상 server URL detail 기록(VK-3) |
| N2 | `backend/internal/infra/adapter/kubernetes/executor_test.go` | 24 | httptest 가짜 API 서버 계약 테스트(판정식 3종 교정 시나리오 포함) |
| N3 | `backend/internal/infra/policy/policy.go` | 24 | PolicyInput(Tenant 제거)·Evaluate·Decision·builtin:default-allow·에러=fail-closed 계약 |
| N4 | `backend/internal/infra/policy/policy_test.go` | 24 | 입력 어휘·판정·버전 단얫 |
| N5 | `backend/internal/infra/migrate/step0005_audit_extend.go` | 24 | sys_operation_log EXTEND(4종 nullable — F-8) |
| N6 | `backend/internal/tasks/cancel.go` + `cancel_test.go` | 24 | Engine.RequestCancel — cancel_requested=true + 이벤트(Phase 1 N-4 원 정의 참조 — §13 항목 1) |
| N7 | `backend/internal/api/v2/operations.go` | 24 | plan/execute 핸들러(동적 권한·정책 fail-closed·멱등 헤더·동결값·재생 200) + `GET /resources/:uid/operations`(§16.1 — registry opdef×kind 교집합·이중 소스 한계 주석 F-9) |
| N8 | `backend/internal/api/v2/tasks.go` | 24 | GET tasks/:uid(+events)·approve/reject/cancel 핸들러 |
| N9 | `backend/internal/api/v2/audit.go` | 24 | §18.1 레코드 조립·작성(audit middleware — request_summary 해시/코드만 VK-11 + 종단 감사 헬퍼) |
| N10 | `backend/internal/api/v2/operations_test.go` | 24 | 핸들러 계약(sqlite fixture·가짜 어댑터·정책 에러 403 음성) |
| N11 | `backend/internal/tasks/engine_circuit_test.go` | 24 | Connection 전달·PollRequest 자격(첫 폴·재폴 2경로)·handle-UID 정합·종단 훅(Complete/Fail 양계열)·N-1 이행 |
| N12 | `backend/opdef/defs_v2infra.go` | 24 | v2 non-GET 5종 Def(민감 라우트 전면 커버) |
| N13 | `web/src/views/infra/InfraTasks.vue` | 25 | 태스크 목록·상세·승인/기각 UI |
| N14 | `web/e2e/playwright.config.js` | 26 | 웹베이스 URL·타임아웃·webServer(dev 서버 기동·정리) |
| N15 | `web/e2e/slice-a.spec.js` | 26 | UI 흐름 테스트 |
| N16 | `backend/e2e/slicea/trace_test.go` | 26 | Go trace(크래시 인젝션·클러스터 단얫 — `//go:build e2e`·대상 `kind-v2-p3`·전용 스키마 DSN·오버라이드 단얫 포함) |
| N17 | `v2-phase1/k8s-fixture/seed.yaml` | 26(운영자산) | 커밋본 시드 — v2-seed ns: deploy/sts/**daemonset 1종 추가**(ds 판정식의 실증 — R6)·default ns deploy |
| N18 | `v2-phase1/k8s-fixture/README.md` | 26(운영자산) | **v2-p3 2호기** 구축·시딩·v1 등록·백필(operations 바인딩 확인)·게이트 재현 절차·재시딩 절차·dirty runbook 1줄(VK-12)·측정 기구 문구(§0.1) |
| N19 | `v2-phase1/k8s-fixture/run-fixture.sh` | 26(운영자산) | README 절차의 실행 가능 스크립트(2호기 생성→시딩→등록→백필→sync — L1: 문서가 아닌 명령으로 재현) |

### 수정 (20)

| # | 파일 | PR | 변경 |
|---|---|---|---|
| M1 | `backend/internal/infra/contract/adapter.go` | 24 | `OperationRequest.Connection` 필드(N-1) + `PollRequest` 타입 + `TaskPoller.Poll` 시그니처(J12) |
| M2 | `backend/internal/tasks/engine.go` | 24 | connectionView 결과 전달(716/729)·PollRequest 조립(첫 폴 760·재폴 826)·`OnTaskTerminal` 훅(Complete/Fail 공통) |
| M3 | `backend/internal/infra/adapter/kubernetes/client.go` | 24 | patchJSON(strategic-merge-patch)·getJSON 재사용 |
| M4 | `backend/internal/infra/compose/compose.go` | 24 | k8s capability 등록·restart opdef 등록·Stack에 Engine 조립(훅 배선 — 관찰 갱신+종단 감사) |
| M5 | `backend/internal/infra/compose/compose_test.go` | 24 | 등록 단얫 추가(기존 단얫 무변경) |
| M6 | `backend/internal/infra/migrate/migrations.go` | 24 | step0005 append 1줄 |
| M7 | `backend/model/log.go` | 24 | OperationLog 4필드 추가(nullable — 모델 반영) |
| M8 | `backend/router/router.go` | 24 | v2 그룹에 라우트 8종 등록(mutation 5종·`GET tasks/:uid`·`GET tasks/:uid/events`·`GET resources/:uid/operations` — opdef V2 동적 미들웨어)·router.New 시그니처(infraAPI·engine 주입) |
| M9 | `backend/main.go` | 24 | compose 1회 → Engine 조립·Start(**유효 설정 1행 로그 — VK-8**)·셧다운 순서 배선: `engine.Stop → 서비스 정리 → server.Shutdown(ctx 예산)` (F-7 — 엔진이 살아있는 동안 진행 중 요청이 종단을 보게 하는 순서) |
| M10 | `backend/config/config.go` + `backend/config.example.yaml` | 24 | engine 섹션+기본값 |
| M11 | `backend/opdef/opdef.go` | 24 | All()에 v2infraDefs 병합 + V2 동적 권한 헬퍼 |
| M12 | `backend/router/authz_replay_test.go` | 24 | authGroupRoutes에 /api/v2 포함·baseline 갱신(240→245) |
| M13 | `docs/security/sensitive-routes.txt` + `docs/security/route-inventory.txt` | 24 | 골든 재생성(-update) |
| M14 | `web/src/api/infra.js` | 25 | plan/execute/task/approve/reject/cancel 클라이언트 |
| M15 | `web/src/views/infra/K8sInventory.vue` | 25 | 리소스 오퍼레이션 패널(연산 목록→plan 다이얼로그→execute→태스크 링크) |
| M16 | `web/src/utils/infra-i18n.js`·`web/src/router/index.js` | 25 | 신규 키(2로케일 parity)·/infra/tasks 라우트 |
| M17 | `.github/workflows/v2-ci.yml` + `web/package.json`(e2e 스크립트) | 26 | slice-a-e2e 잡 추가(**기존 9잡 무변경·10번째**) |
| M18 | `backend/internal/infra/inventory/backfill.go` (+ `backfill_test.go` 단얫) | 24 | createClusterChain에 operations purpose 바인딩 1행(동일 SecretRef — J12(1))·기존 inventory 경로 무영향 단얫 |
| M19 | `backend/internal/infra/adapter/fake/fake.go`·`fake_test.go` | 24 | PollRequest 시그니처 전파(J12 — phase2 계약 승계 범위 내 수정) |
| M20 | `backend/store/seed.go` | 25 | Infrastructure 섹션에 "Tasks & Approvals" 메뉴 그룹 시드 1행 additive(§17.3 "seed rows for the new menu group + items" — r2: 웨이버 철회·구현. 부트 시드의 멱등 upsert가 "(one migration)"의 1회 적용 의도를 충족 — 근거 기록) |

기존 v1 도메인 파일 무접촉: `service/k8s.go`·`controller/k8s.go`·`model/k8s.go`·v1 라우트 그룹·`internal/domain/**` (보존 제약 #1).

---

## 3. 인터페이스 계약 (스펙 어휘의 정확한 코드화)

### 3.1 `contract` 확장 (N-1 + J12 이행)

```go
type OperationRequest struct {
    OperationName string
    ResourceURN   string
    Payload       JSONMap
    Connection    ConnectionView // §7.2 실행 자격 — 엔진이 브로커 Resolve(operations)로 조립
}
type PollRequest struct {
    Handle     OperationHandle
    Connection ConnectionView // 재폴 자격 — 엔진이 매 폴마다 조립 (J12)
}
```

- 엔진 `executeClaimed`: 기존 `connectionView(ctx, chain)` 결과를 `req.Connection`에 전달(폐기 라인 716 제거). Material 키 = `operations`(Discover의 `inventory`와 대칭 — §7.4 어휘).
- 재폴 `pollAsyncAttempts`: 기존 `resolveExecutionChain`(810)에 이어 connectionView를 조립해 `PollRequest`로 전달. 크래시 후 attempt 2도 동일 경로(체인 조인→뷰 재조립 — 핸들의 자격 재구성 불필요).
- 정합 단얫: `PollRequest.Connection.UID` == `Handle.ProviderRef`에 인코딩된 connUID (불일치 = 조립 버그 — N11이 기계 판정).

### 3.2 k8s 어댑터 OperationExecutor/TaskPoller (`executor.go`)

```text
OperationName 유일 수용: "k8s.workload.restart". ResourceURN 파싱:
  urn:k8s:{ctxID}:workload:{namespace}/{kind}/{name}  → (ns, kind∈{deployment,statefulset,daemonset}, name)
Connection.Material["operations"] = kubeconfig (부재 → 즉시 에러 — engine credential_error와 정합)
Payload["restartedAt"] 필수(RFC3339 — 서버 동결값, J1)·["resourceRevision"] 선택(감사용)
PATCH application/strategic-merge-patch+json:
  spec.template.metadata.annotations["kubectl.kubernetes.io/restartedAt"] = restartedAt
반환: OperationHandle{ProviderRef: "rollout|<connUID>|ns|kind|name|<expectGeneration>"} (expect=응답 metadata.generation)
Poll(PollRequest): GET → 종별 완료 판정(J2 교정식 — deploy updatedReplicas·sts revision 수렴·ds updatedNumberScheduled)
  → OperationStateSucceeded{detail: generation, restartedAt, readyReplicas/updated, serverURL} | Running
detail.serverURL = Connection에서 파싱한 API 서버 주소 (VK-3 — 비밀 아님·kind는 loopback. 오발사 추적 증거)
오류: 401/403 → ProviderSignalError(permission_denied)·429 → rate_limited·그 외 일반 error(엔진 executor_error/operation_failed 분기)
```

- Compile-time 단얫에 `OperationExecutor`/`TaskPoller` 추가. registry V5 요구(apply capability → OperationExecutor)가 같은 Phase에서 충족.

### 3.3 OperationDefinition 등록 (`compose.go` — 코드가 정의의 소재)

```go
contract.OperationDefinition{
    Name: "k8s.workload.restart", Version: "1",
    ResourceKinds:      []string{"orchestration.workload"},
    RequiredCapability: "orchestration.kubernetes.apply",
    RequiredPermission: "assets:k8s:workload:restart", // §10.3 v1 재사용 (J4)
    Mutating: true, RiskLevel: "medium",
    RequiresApproval:  true,                    // J8 — V2 레인 신중 posture
    IdempotencyPolicy: "provider_frozen_annotation", // J1
    TimeoutSeconds: 30, RetryPolicy: contract.RetryPolicy{MaxAttempts: 3, BackoffSeconds: 5},
    Redaction: func() any { return restartRedaction{} }, // 결과 detail 허용 필드 명시
}
// + RegisterCapabilities("kubernetes", orchestration.kubernetes.read/apply, inventory.full …)
```

### 3.4 API v2 mutation (§16.2 — 라우트 7종)

```text
POST /api/v2/infra/resources/:uid/operations/:name/plan     권한=def.RequiredPermission(동적)
   → 200 {operation, version, resourceUid, resourceRevision, requiresApproval, riskLevel,
          permission, policyVersion, restartedAt}          // 상태 무변경·태스크 미생성
POST /api/v2/infra/resources/:uid/operations/:name/execute  Idempotency-Key 헤더 필수
   → 첫 제출 201 {task} · 재생 200 {task} + Idempotency-Replayed: true (스펙 §13.4 문언 그대로 — r2 교정)
                                                              // restartedAt을 payload에 동결해 Submit
GET  /api/v2/infra/tasks/:uid                               → {task, events}
POST /api/v2/infra/tasks/:uid/approve | reject              권한=ops:job:approve → Engine.Approve/Reject
POST /api/v2/infra/tasks/:uid/cancel                        권한=ops:job:approve → Engine.RequestCancel
```

- 공통: Auth + V2 audit middleware(신규). execute는 정책 평가(J5 — Evaluate 에러 시 403 fail-closed) 후 Submit — resource_busy 409·unknown op 404·미권한 403(감사와 함께).
- **Idempotency-Key 축소 근거 (r2 — L1)**: 키는 execute에만 필수로 한다. §13.4의 멱등은 **태스크 생성**의 중복 방지이며, approve/reject/cancel은 재호출이 동일 종단으로 수렴하는 자연 멱등 상태 전이다 — 키 발급 의무를 4개 엔드포인트에 확장하면 클라이언트 부담만 가중된다. 근거는 핸들러 주석에 기록.
- 헤더 X-Request-ID(없으면 서버 생성)·traceparent(있다면 전파·**부재 시 trace_id는 서버 생성값 — 키 존재가 단얫 대상**, claim 8)를 요청 맥락에 보관 — 감사 필드.

### 3.5 감사 레코드 (§18.1 전 필드 — J3·VK-11)

```text
요청 감사(미들웨어): actor_id·username·source_ip·url·method·status·request_id·trace_id
                     ·operation·operation_version·mutating·risk_level·policy_version
                     ·provider_connection_uid·provider_context_uid·resource_uid·task_uid·request_hash
종단 감사(엔진 훅):  위 + approval_status·approver·provider_task_id·error_code·result_hash
                     (url="task://<uid>" 규약 — v1 미들웨어와 경로 공간 분리)
request_hash = sha256(정규화 payload)·result_hash = sha256(정규화 status detail) — 자격 물질 미포함(보존 제약 #7)
request_summary(v2 레인) = 해시·에러 코드 등 비밀 없는 요약 문자열만 — 요청 본문 직렬화 금지 (VK-11)
신규 컬럼 4종 전부 nullable — 기존 v1 행은 NULL 유지 (F-8)
v2_context 비교는 JSON 디코드 후 키 단얫 — MySQL 정규화로 바이트 비교 금지 (F-8)
```

### 3.6 엔진 확장 (`cancel.go`·`engine.go`)

- `RequestCancel(ctx, taskUID, actor)`: 상태 조회 → queued/awaiting_approval이면 즉시 종단 cancelled(기존 cancelAtClaimBoundary 경유)·running이면 cancel_requested=true + `cancel_requested` 이벤트 — **소비 시점은 "next claim boundary"(스펙 §13.4 문언 — r2 교정 F-5)**. 폴 사이클에서 소비하는 별도 경로는 구현하지 않는다: 종단 처리 경로가 2곳이 되면 D5(종단 원커밋) 검증 공간이 분할되고, k8s restart 어댑터는 부분 롤아웃 취소(cancel path)를 제공하지 않으므로 Slice A에서 running 중 취소의 즉시화는 요구 트레이스에 없다. §13.4 "persisted either way"는 준수(플래그·이벤트 영속).
- `OnTaskTerminal` 훅(J6): Complete/Fail 커밋 후 공통 호출 — compose가 SyncRunner(관찰 갱신)+종단 감사를 배선. 훅 시그니처는 status를 받아 감사 행에 성공/실패를 싣는다.

### 3.6a 태스크 종단 메커니즘의 정확한 서술 (r2 — F-4 재서술)

r1의 "태스크 deadline은 TimeoutSeconds×MaxAttempts 합산 범위에서 CallTimeout이 지킨다"는 부정확했다. 실제 메커니즘(실측 §0.7):

```text
폴 1회 = CallTimeout 컨텍스트로 제한 (executeClaimed execCtx)
폴 사이클은 리스를 갱신하지 않음 → 리스(클레임 시점 하한) 만료 후에도 폴이 Running 반환 가능
→ 리퍼 재큐 (attempts 잔여 시 재실행 — J1 멱등 patch로 클러스터 무해 / 소진 시 lease_expired 종단)
종단 보장: Succeeded/Failed 폴 판정 · attempts 소진(실패 재시도) · 리스 만료+소진(lease_expired)
```

- **슬로우 롤아웃 경로의 리스크 (기록)**: 이미지 풀 등으로 롤아웃이 CallTimeout+리스 하한(~36s)을 넘기면 폴이 계속 Running인 채 재큐된다 — 재실행은 무해하지만 attempts를 소진하면 `lease_expired`로 실패 종단된다. 완화: H1(J11 이미지 사전 pull)·리스크 R12.

### 3.7 UI (PR 25 — additive)

- `InfraTasks.vue`: 태스크 목록(상태 필터) + 상세 drawer(이벤트 타임라인) + 승인/기각 버튼(awaiting_approval에서만 활성) + 결과 detail 렌더. **폴링 2s**(WS 기각 — §4.8 A/B/C 배포 조정 문제를 V2가 재현할 이유 없음, 리스크 R4).
- `K8sInventory.vue` 오퍼레이션 패널: 행 클릭 → 리소스 drawer에 Operations 탭 → `GET /resources/{uid}/operations`(§16.1 — 응답은 registry opdef×kind 교집합. **F-9 주석**: opdef ResourceKinds와 관찰된 kind 어휘의 이중 소스는 M1에서 어느 한쪽이 갱신되면 교집합이 일시 축소될 수 있다 — kind 어휘 통합은 Phase 4 이월) → restart → plan 다이얼로그(개정·위험·승인 필요 표시) → execute → InfraTasks로 링크.
- i18n 신규 키는 ko-KR/en-US 동시(파리티 게이트). 메뉴: **"Tasks & Approvals" 그룹 시드 1행(M20)** — §17.1 IA에서 Inventory와 동등 레벨의 그룹이며 §17.3이 시드를 명시한다(r2: r1의 "기존 시드 자식 라우트로 족하다" 판단 철회).

---

## 4. 임계경로 식별 (직렬 — 심층 다룰 부분)

```text
① 자격 경로 완성 + 엔진 회로 (M1, M2, M18, M19, N11 — 판단 J12)
   └ operations 바인딩(존재)과 PollRequest(전달)이 만들어져야 어떤 실행도 시작된다.
     잘못하면 자격 우회·Material 누출. 코어 리뷰 집중.
② k8s Execute/Poll + registry 등록 (N1, N2, M3, M4, M5)
   └ J1 멱등·J2 교정 판정식이 게이트 ②를 결정. httptest 계약 + kind 실측 이중 검증.
③ Go trace E2E — 크래시 인젝션 (N16, N17, N18, N19 — Phase F→G1)
   └ ①②+감사+API 전부 완료돼야 green. 게이트 ①②③의 최종 증명 기구.
     대상 클러스터는 v2-p3 2호기(§0.2a) — v2-p2 무변경.
```

병렬 가능(①이후): 정책(N3/N4)·감사(N5·N6·M6/M7·N9)·API(N7/N8/N10)·opdef/게이트 골든(M11-M13)·메뉴 시드(M20)·웹(N13/M14-M16). ②→③ 창에서 CI 잡(M17)·Playwright(N14/N15) 병렬. B2(backfill 바인딩)는 B와 병렬.

## 5. Phase 분해 (≤5파일·독립 검증 단위)

| Phase | 파일 | 독립 검증 |
|---|---|---|
| **A — 계약·엔진 회로·자격 전달** (임계) | M1, M2, M19(2파일), N11 — 5파일 | `go test ./internal/tasks/ ./internal/infra/contract/ ./internal/infra/adapter/fake/ -race` — Connection 전달·PollRequest 자격(첫 폴·재폴)·handle-UID 정합·OnTaskTerminal 양계열 |
| **A2 — cancel** | N6(cancel.go+cancel_test.go — 2파일) | `go test ./internal/tasks/ -run Cancel -race` — 전이 3경로(queued/awaiting/running)·next claim boundary 소비 |
| **B — k8s executor·등록** (임계) | N1, N2, M3, M4, M5 | httptest: patch 본문·재실행 byte 동일(멱등 근거)·**교정 판정식 3종**(구 세대 병존 시 Running 포함)·403/429 신호·serverURL detail + registry 등록 단얫 |
| **B2 — operations 바인딩** | M18(backfill.go+backfill_test.go — 2파일) | sqlite 백필 fixture — 바인딩 2행(inventory+operations·동일 SecretRef)·기존 경로 무영향 단얫 |
| **C — 감사·정책** | N5, M6, M7, N3, N4 | step0005 적용(양 dialect·nullable)·레코드 필드 완전성 단얫·정책 판정·에러 fail-closed |
| **D1 — API v2 mutation** | N7, N8, N9, N10 | sqlite fixture: plan 무변경·execute 멱등 재생(동일 키 → 동일 task·**200**+헤더)·권한 거부 403·**정책 에러 403**·승인 비대상 409 |
| **D2 — 라우터·배선·권한** | M8, M9, M10(2파일), N12 — 5파일 | 엔진 Start/Stop 라이프사이클·Start 설정 로그·셧다운 순서·동적 권한 403 |
| **D3 — 게이트 골든** | M11, M12, M13(2파일) — 4파일 | route-coverage 확장 잡 green·골든 byte-diff |
| **E — 웹 (PR 25)** | N13, M14, M15, M16(2파일) — 5파일 | `bun run build` + i18n 파리티 + Playwright 수동 스모크 |
| **E2 — 메뉴 시드** | M20 — 1파일 | 시드 재실행 멱등·Tasks & Approvals 메뉴 1행·기존 메뉴 무변경 |
| **F — 2호기 리허설** (운영자산) | N17, N18, N19 (+D1 산출 `GET /resources/{uid}/operations` 실측·B2 산출 operations 바인딩 실측) | `bash v2-phase1/k8s-fixture/run-fixture.sh`로 **v2-p3** 구축 후 트레이스 리허설 1회 완주: 백필→sync→compare→plan→execute→승인→회복→클러스터 단얫(증분식 §0.1) — CI 잡 절차의 실측 근거 |
| **G1 — Go trace E2E** (임계) | N16 (+e2e fixture 헬퍼) | `go test -tags=e2e ./e2e/slicea/ -count=1` **on kind v2-p3** (전용 스키마 DSN·엔진 오버라이드 단얫 포함) |
| **G2 — Playwright·CI** | N14, N15, M17(2파일) | 로컬 Playwright green(브라우저 설치 후) → CI slice-a-e2e green |

순서: A→A2→B(‖B2)→(C‖D1‖D2)→D3→E→E2→F→G1→G2. F는 G1 직전 리허설(v2-p3에서 수분 내 1회 완주 — CI 잡 설계의 실측 근거).

## 6. 테스트 계약 (TDD — §23)

```text
unit        URN 파서·payload 검증(restartedAt 형식)·동결값 생성·정책 판정(Tenant 부재·에러 fail-closed)·
            감사 필드 완전성·해시 정규화·request_summary 비밀 부재(VK-11)
adapter     executor 계약(httptest): patch 본문·재실행 byte 동일(멱등 근거)·교정 판정식 3종
            (구 세대 병존=Running·세대 수렴=Succeeded 시나리오 포함)·403/429 신호·
            kubeconfig 누출 부재 — detail/payload/에러 본문 스캔 + task_event.data·
            provider_task 에러 컬럼 포함 (VK-10 — 보존 제약 #7)·serverURL loopback 단얫(VK-3)
engine      Connection·PollRequest 자격 전달(첫 폴 760·재폴 826 양경로)·handle-UID 정합·
            RequestCancel 전이(queued/awaiting/running 3경로)·OnTaskTerminal(성공/실패 공통 발화)
api         plan 무변경·execute 멱등 재생(동일 Idempotency-Key → 동일 task·200+Idempotency-Replayed)·
            권한 거부 403·정책 Evaluate 에러 403(fail-closed 음성)·승인 비대상 태스크 409
e2e(Go)     §25 트레이스 + 크래시: SIGKILL→(lease+grace 경과 대기)→재기동→reaper_requeued→종단→
            클러스터 단얫 3종(§0.1 측정 기구 — kubectl 서브프로세스·creationTimestamp)→감사 SQL 단얫
e2e(PW)     UI 흐름(view→restart→approval→task result) — §23.1 e2e tier (webServer 기동)
음성(N계)   N3(미권한 403+감사)·N6(멱등 재생·싱글 실행 — 클러스터 단얫으로)·N7(크래시 회복)·
            N11(resource_busy fast-fail — engine 게이트)·D1(정책 에러 403)
```

기존 테스트 무변경 원칙: 기존 단얫 약화·기대값 변경 금지 — 신규 파일·신규 함수 추가만. `authz_replay_test.go` baseline 240→245 갱신은 게이트 계약의 명시적 갱신(라우트 추가의 정당한 골든 리프레시 — 커밋 메시지에 근거 명시). `backfill_test.go`·`fake_test.go`·`compose_test.go`는 **단얫 추가**만(r2 — M18/M19/M5 산물).

## 7. 검증 요구 (claims — ④리뷰 주입용·로컬 명령)

1. `grep -n "Connection ConnectionView\|type PollRequest" backend/internal/infra/contract/adapter.go` — N-1 필드 + PollRequest 타입 존재.
2. `cd backend && go test ./internal/tasks/ -race -run 'Circuit|Cancel|Terminal' -count=1` — Connection·PollRequest 자격(양경로)·cancel·종단 훅 GREEN.
3. `cd backend && go test ./internal/infra/adapter/kubernetes/ -race -count=1` — executor 계약(멱등 patch byte 동일·교정 판정식 3종 포함) GREEN.
4. `cd backend && go test ./... -race -count=1` — 전 패키지 GREEN(기존 회귀 0)·opdef 커버리지 래치 통과.
5. `cd backend && go test ./router/ -run 'TestRouteInventoryArtifact|TestSensitiveRoutesArtifact|TestOperationTableCoversRouter' -count=1` — v2 non-GET 5종이 opdef 테이블에 등재(Def 5행 — plan/execute의 Permission은 `assets:k8s:workload:restart` 대표값, 실제 부여는 registry 동적 조회)·sensitive-routes.txt 285→290엔트리·route-inventory 재생성.
6. 로컬 트레이스 리허설: `bash v2-phase1/k8s-fixture/run-fixture.sh`(v2-p3 2호기 — N18/N19 절차) → plan→execute→approve → 태스크 `succeeded` → 실행 전후 스냅샷 대비 `kubectl --context kind-v2-p3 -n v2-seed get deploy seed-nginx -o jsonpath='{.metadata.generation}'` **증분 == 1** (기선 절대값 무의존 — §0.1 측정 기구. N18 재시딩 후에도 동일 명령).
7. (게이트 ②) `cd backend && go test -tags=e2e ./e2e/slicea/ -count=1` on kind-v2-p3 — 크래시 인젝션 후 (a) generation 증분==1 (b) creationTimestamp > 테스트 시작인 신규 RS==1 (c) pod template restartedAt==동결값 (d) task_event reaper_requeued 존재 — 테스트 출력으로 단얫.
8. (게이트 ③) `docker exec ops-admin-mysql-dev mysql ... -e "SELECT v2_context FROM sys_operation_log WHERE task_uid='<trace-uid>' ORDER BY id"` — **task_uid 조인 2행(요청+종단)의 합집합으로 §18.1 필드 전부 키 존재·non-null**. 조건부 예외: 종단 행이 succeeded면 error_code는 빈/NULL 허용·traceparent 미전송 요청의 trace_id는 서버 생성값(키 존재 자체가 단얫). 판정은 JSON 디코드 후 키별 검사(MySQL 정규화 — 바이트 비교 금지).
9. (게이트 ①·순서 명시) merge 판정은 claim 7 로컬 실행으로 한다. CI 증거는 PR 오픈 후 slice-a-e2e 잡의 Actions 로그 URL + J11의 compare-inventory 아티팩트로 게이트 종결 시점에 첨부 — "in CI against kind" 이중 증명.
10. `cd web && bun run build && node scripts/check-i18n-parity.mjs` — 프론트 빌드·파리티 GREEN.
11. `git diff main -- backend/ | grep -E '^\+.*(RequiredPermission|Permission\s*[:=])' | grep -vE 'assets:k8s:workload:restart|ops:job:approve'` — **출력 0행** (신규 권한 문자열 잔여 부재 — r2 보강) + `grep -n "RequiredPermission" backend/internal/infra/compose/compose.go` — registry 정의도 동일 어휘.
12. `git diff --stat main -- backend/service/k8s.go backend/controller/k8s.go backend/model/k8s.go` — 빈 diff(v1 무변경).
13. (r2 — M4) `cd backend && go test ./internal/infra/policy/ ./internal/infra/adapter/kubernetes/ ./internal/api/v2/ ./internal/tasks/ -cover -count=1 2>&1 | grep "coverage:"` — 신규 패키지·핵심 확장 패키지 커버리지 전부 ≥80% (phase2 claim 12 형식 승계).
14. (r2 — J12) `docker exec ops-admin-mysql-dev mysql ... -e "SELECT purpose, COUNT(*) FROM provider_credential_binding GROUP BY purpose"` — inventory·operations 2행 존재·동일 secret_ref_id(백필 후) — `grep -n '"operations"' backend/internal/infra/inventory/backfill.go`로 소스 위치 확인.
15. (r2 — M20) `grep -n "Tasks & Approvals" backend/store/seed.go` — 메뉴 그룹 시드 1행 존재.

## 8. 보존 제약 (③구현 프롬프트에 verbatim 복사)

1. **v1 무변경**: `backend/service/k8s.go`·`backend/controller/k8s.go`·`backend/model/k8s.go`·`backend/router/router.go`의 v1 라우트 그룹·`internal/domain/**`은 무접촉. V2 restart는 v1 경로와 병립(§3.3 — v1 lane exempt).
2. **기존 권한 어휘 재사용**: 신규 권한 문자열 0. restart=`assets:k8s:workload:restart`·승인/취소=`ops:job:approve` (§10.3·§13.3).
3. **민감 라우트 전면 커버**: /api/v2 non-GET 5종은 opdef 테이블+sensitive-routes.txt에 등재 — 등재 없는 mutation 라우트 추가 금지.
4. **신규 런타임 의존성 0**: `backend/go.mod` 무변경(§10.2). playwright는 기존 web devDependency.
5. **기존 테스트 무변경**: 기존 단얫·기대값 변경 금지 — 신규 테스트·기존 파일의 **추가 단얫**만(authz_replay baseline 갱신은 예외·근거 기록).
6. **§7.7 이연 테이블 신설 금지**: 감사는 sys_operation_log 확장(step0005) — 별도 감사 테이블·정책 테이블 생성 금지.
7. **자격 물질 보호**: kubeconfig·브로커 평문은 로그·감사·직렬화·UI 응답에 절대 미포함. 감사 해시는 정규화 payload만. 누출 스캔 대상은 detail/payload에 더해 **task_event.data·provider_task 에러 컬럼·API 에러 본문·request_summary** 포함 — request_summary는 해시/코드만 담는다(r2 확대).
8. **CI 기존 9잡 무변경**: v2-ci.yml에는 slice-a-e2e 1잡 추가만(10번째) — 기존 잡 수정 금지. merge 판정은 로컬.
9. **엔진 불변식 준수**: T-5(빈 ResourceUID 거부)·T-8(리스 하한)·D4(버전 CAS)·D5(종단 원커밋) 기존 계약 위반 변경 금지.
10. **어댑터 결계 (r2 — J12)**: 어댑터는 자격을 스스로 획득하지 않는다 — DB 주입·ProviderRef에 자격 물질 인코딩 금지. 자격은 엔진이 조립한 ConnectionView(OperationRequest·PollRequest)로만 전달.
11. **클러스터 결계 (r2 — C 판정)**: phase2 게이트 클러스터 `v2-p2`는 재생성·시딩·mutation 전부 금지. Phase 3의 실클러스터 작업(리허설·E2E·재시딩)은 `kind create cluster --name v2-p3` 2호기에서만.

## 9. 리스크 매트릭스 (가능성 × 파급) + 완화

| # | 리스크 | 가능성 | 파급 | 완화 |
|---|---|---|---|---|
| R1 | 재실행 시 이중 restart(크래시 창) | 중 | 치명(게이트 ②) | J1 동결값 멱등 + 클러스터 단얫 3종으로 매 실행 검증. httptest에서 patch byte 동일 단얿 |
| R2 | 크래시 인젝션 플레이크(타이밍) | 중 | 높음 | Go 층이 프로세스 관리 — 롤아웃 running 관찰 후 SIGKILL·재시도 1회 허용·회복 파라미터 env 오버라이드·**kill 후 lease+grace 경과 대기 후 재기동**(H1 — 재기동 즉시 재큐 결정성) |
| R3 | CI kind 잡 불안정·시간 초과 | 중 | 중 | 15m timeout·**이미지 사전 pull+노드 로드(J11 — 슬로우 풀 제거)·retry 1**·path filter 없음 관례 유지·F 리허설 소요 실측 후 잡 확정 (r2 실체화 — 추상적 "캐시" 서술 대체) |
| R4 | 태스크 상세 WS 유인 | 낮음 | 중 | 폴링 2s 확정(판단 기각 사유 문서화) — §4.8 A/B/C 문제 회피 |
| R5 | lease/timeout 파라미터 부조화(E2E 지연·이중 실행) | 중 | 높음 | T-8 하한 식 준수 + §0.7 하한식으로 회복 예산 산정·E2E env로 단축·Start 설정 로그로 적용값 단얫(VK-8) |
| R6 | 종별 완료 판정 부정확(ds/sts 조건) | 중 | 중 | **r2 교정 판정식**(세대 수렴 기준)·httptest 3종 시나리오(구 세대 병존 포함)·**N17 시드에 ds 추가 — kind 실측(F)이 3종 전부 커버** |
| R7 | route-coverage baseline 드리프트로 CI 적색 | 높음 | 낮음 | D3에서 골든 재생성 커밋·canary 작동 재확인(claim 5) |
| R8 | ops:job:approve 재사용 반대 의견 | 낮음 | 낮음 | J4 절충 문서화·세분화 이월 대장 — 코드값 1줄 교체 가능 |
| R9 | strategic-merge-patch가 daemonset/sts에서 거부 | 낮음 | 중 | 경로 3종은 v1이 이미 사용 중(실측 §0.3) — 동일 patch 방식 재사용 + kind 실측(ds 시드 포함) |
| R10 | 감사 JSON 컬럼 증거 약화(쿼리 불가 지적) | 낮음 | 낮음 | task_uid 실컬럼 조인 제공·필드 단얫 기계화(claim 8) — 전 컬럼화는 요구 시 이월 |
| R11 | engine main 배선이 v1 기동 실패 경로 추가(장애 전파) | 낮음 | 높음 | 스택 조립 실패 시 엔진만 비활성 로그(v2 API degradation 관례 승계)·v1 서비스는 기동 유지 |
| R12 | 슬로우 롤아웃(이미지 풀) 중 리스 만료→attempts 소진→lease_expired (r2 신설) | 중 | 중 | §3.6a 서술·이미지 사전 pull(J11/H1)·멱등 patch로 재큐 무해·E2E 종단 대기 120s |
| R13 | v2-p3 2호기 오퍼레이션 실수로 v2-p2 오염 (r2 신설) | 낮음 | 높음 | kubeconfig 컨텍스트 고정(`--context kind-v2-p3` — E2E·run-fixture.sh 상수)·보존 제약 #11·loopback 단얫(VK-3)이 우발 크로스 클러스터도 포착 |

## 10. CI 방침

- merge 판정: 로컬(`go test ./... -race`·`bun run build`·본 계획 claim 명령). 원격 대기 금지 승계.
- 승인 1잡 추가(`slice-a-e2e`, J11 — **10번째**) — 스펙 §20 "in CI against kind"·§23.2 "kind cluster in CI" 충족 + §25 "+§15 comparisons passing" 세그먼트의 CI 증거(compare-inventory 스텝). required check 등록 여부는 병합 보호 직접 조작(로컬 판정과 무관하게 상시 증거 목적 — 비-required 권장, 최초 2주 안정화 후 판단).
- k3d 주간 잡·라이브 클라우드 크레덴셜: 도입 안 함(스펙 옵션·금지 준수).

## 11. 롤백 가능성 판정

- **PR 24 완전 롤백 가능**(코드 관점): v2 그룹 라우트 제거·엔진 배선 제거로 v1 무영향. 지속물 3종 — step0005 컬럼(nullable — 방치 무해)·provider_task 행(데이터)·감사 행. 로컬/CI kind 클러스터는 외부 자산.
- **재기동 잔존 태스크 거동 (r2 — VK-9 명시)**: 롤백·재기동 시 running 잔존 태스크는 리스 만료 후 리퍼 재큐 → 엔진 부재 시 재소비 불가로 `lease_expires_at` 경과 상태로 잔존(폴러 무소유). 개발 정리 커맨드: `UPDATE provider_task SET status='failed', error_code='lease_expired', finished_at=NOW() WHERE status IN ('queued','running','awaiting_approval');` (runbook N18에 기록 — 운영 데이터 아닌 dev 전용).
- **PR 25**: additive 뷰 — 롤백 시 라우트·뷰 파일 삭제·메뉴 시드 1행 제거(부트 시드 upsert라 재실행으로 소거).
- **PR 26**: 테스트·CI 잡 제거로 롤백. CI 잡은 workflow 라인 삭제.
- 게이트 실패 시 착구간 복귀: Phase 단위 revert(A→G2 순서 역적용 — 이후 Phase가 이전 산출에 의존).

## 12. 파일 소유권 — 구현 스폰 분리

| 스폰 | 소유 | 접근 금지 |
|---|---|---|
| impl-P24a (임계) | M1·M2·M19·N11·N6(cancel 세트) | contract/registry 그 외·infra 하위 전부 |
| impl-P24b (임계) | N1·N2·M3·M4·M5·M18 | tasks·api·opdef |
| impl-P24c | N5·M6·M7·N3·N4 | tasks·api/operations |
| impl-P24d | N7–N10·M8–M10·N12 | internal/infra 수정(engine/compose는 소유 외 — 인터페이스만 소비) |
| impl-P24e | M11–M13 | 라우터 본체·opdef defs 본체(등재만) |
| impl-P25 | N13·M14–M16·M20 | backend 전부 (seed.go 예외 — E2) |
| impl-P26a (임계) | N16–N19 | 프로덕션 코드 전부(테스트·fixture만) |
| impl-P26b | N14·N15·M17 | backend·v2-ci 기존 9잡 라인 |

## 13. 산출 외 확인사항 (인계 — 이월 대장 증보)

1. **Phase 1 N-4 이행**: RequestCancel(N6)이 cancel_requested 발화 경로를 완성 — 원 정의는 phase1 계획 N-4(엔진 상태머신·이벤트 식별자 `cancel_requested`)를 참조(L7 — 구현은 해당 식별자를 그대로 발화).
2. **PR21 관계 링크 normalizer 확장**: 인계 검토 결과 — Slice A 증명에 불필요(트레이스·감사가 관계 링크 미요구). Phase 4 이월 확정.
3. 권한 세분화(`infra:task:*` 분리) — 다중 운영자 요구 시(J4).
4. 낙관적 plan↔execute 개정 잠금(J7 기각안) — 요구 시.
5. 관찰 갱신 incremental 모드 — Phase 4(§3.3 r2 이월 승계).
6. `GET /resources/{uid}/operations` 응답 캐시·UI 권한 필터링(클라이언트 측 숨김 아닌 서버 산출 — 본 Phase 구현, 향후 정책 deny 연동 시 확장).
7. 승인 알림(NotifyRule 연동) — M2(§3.2 row 23·§13.6).
8. **(r2 — M2 완전) §16 API 미구현 엔드포인트 이월 — 4건 재등재** (r1이 범위 밖으로 두면서 대장에서도 누락):
   - **POST 3종** (§16.1): `POST /provider-connections`(신규 등록)·`POST /provider-connections/{uid}/validate`·`POST /provider-connections/{uid}/sync` — Phase 2가 GET만 구현(실측 infra.go:63-66). 커넥션 수명주기 UI·백필 트리거 수요 시 Phase 4.
   - **GET 3종** (§16.1): `GET /provider-connections/{uid}`·`GET /provider-contexts`·`GET /resources/{uid}/relationships` — Slice A 트레이스가 요구하지 않음. Phase 4.
   - **`/internal/metrics` 노출** (§18.2): metrics 패키지(§18.2 표 M1 열)는 구현됐으나 **HTTP 엔드포인트 미노출**(실측 — router/main grep 0건). 게이트 ③의 metricsText 증명은 sync 리포트로 대체(phase2 관례). 노출은 운영 배포 요건 시.
   - **stale_source 상세 노출** (§5.4(c)): "read side는 stale_source 행을 absent 처리"의 API 응답 명시(필터·마킹 노출) — 백필·비교 엔진은 구현됐으나 리소스 API 응답 형상은 미정. v1 삭제 시나리오가 실제 발생하는 Phase 4에 정의.

## 14. 가정 명세 (불확실 요소의 명시적 처리)

| # | 가정 | 검증 시점 |
|---|---|---|
| A1 | 동일값 restartedAt 재 patch → generation 무증가·신규 RS 없음(k8s API 시맨틱) | Phase B httptest + Phase F kind 실측 — 위반 시 J1 대안(실행 전 annotation 비교 병용) 재검 |
| A2 | **(r2 교정)** deploy 완료 = observedGeneration≥expect && availableReplicas==spec.replicas && **updatedReplicas==spec.replicas** (구 RS pod가 available 카운트에 포함될 수 있어 세대 수렴이 필요) | B(httptest 구 세대 병존 시나리오)·F |
| A3 | `ops:job:approve`의 V2 재사용 승인(계획 승인 시 확인) | 계획 승인 |
| A4 | policy_version 문자열 `builtin:default-allow` — 스펙이 버전 형식 미규정 | 감사 필드 형식 단얫으로 고정 |
| A5 | restart `RequiresApproval=true`(J8) 승인 | 계획 승인 |
| A6 | 엔진 파라미터 env 오버라이드(`OPS_ADMIN_ENGINE_*` — PollInterval·LeaseSeconds·ReaperGrace. **TimeoutSeconds 오버라이드 없음** — J10 기각) | D2 |
| A7 | E2E 인증 = 시드 admin 계정(login API·UI 동일) | F |
| A8 | **(r2 교정)** sts 완료 = status.updateRevision==status.currentRevision && readyReplicas==spec.replicas·ds 완료 = updatedNumberScheduled==desiredNumberScheduled (ready/numberReady는 구 세대 포함) | B·F(ds 시드 N17 포함) |
| A9 | v2 audit middleware는 `/api/v2/infra` POST 전부에 부착(향후 V2 mutation 확장 시 동일 패턴) | D1 |
| A10 | **(r2)** §17.3 "(one migration)"의 메뉴 시드는 부트 시드의 멱등 upsert(additive 1행)로 충족 — 별도 마이그레이션 스텝 불요 | E2 |
| A11 | **(r2)** E2E 전용 MySQL 스키마(`ops_admin_p3e2e` — VK-7): N16이 스키마 생성·마이그레이션·시딩을 자체 수행. 대안(운영 dev 서버 정지 후 사용)은 스키마 충돌 위험으로 기각 | G1 착수 전 F에서 1회 확인 |
