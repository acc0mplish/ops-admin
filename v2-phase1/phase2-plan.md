# V2 Phase 2 계획 — Kubernetes/K3s 읽기 전용 수직 슬라이스 (r2)

작성: 2026-09-06 (r1) · 개정: 2026-09-06 (r2 — 3렌즈 적대검토[완전성·위험·검증가능성] 병합 발견 반영, 원 계획자 판정) · 티어 L (비교 프로토콜·정규화 정확도가 게이트 직결 → review-pr-xhigh 병행, 구현은 위험 비례) · 앵커(실측 시점 HEAD): `6cc1e72`

## r2 반영사항 (r1 대비 — 무결 판정부[§0 실측 게이트 산출 매핑·§5.4 전파·J9 어휘 경로·§15 요소 전 코드화·harness 4단얫]는 미변경)

- **A (C-1/H-1 수렴)**: `DiscoverRequest`에 `Connection contract.ConnectionView` 필드 추가로 어댑터 자격증명 경로 확정(§1 J2·§3.1). `contract/adapter.go` 수정 범위 갱신(§2 PR 20·보존 제약 #2 — 파일 주석 갱신 포함). N-1 잔존분(OperationRequest.Connection) Phase 3 인계 명시. per-connection 팩토리 대안 기각 사유 기록.
- **B (H-1/H-3)**: 종별 페어링 키 `namespace/name`(클러스터 스코프 종은 `name`) 확정 — legacy 직렬화에 uid 전무 실측(§0.6). §3.5 identity 비교 문단·비교 스코프 규칙 신설(ReplicaSet·Job·CronJob·StorageClass 처분표). J8 시드 5종 유지 판정(근거 §1 J8).
- **C (검증)**: claim 6 라벨 형식 `provider_rate_limit_total{provider="kubernetes"}`(§18.2 정합)·claim 14 grep에 `kubeConfig` 추가·아티팩트 경로 `data/compare/<cluster>/<date>/<time>.json` 단일화(§2·§3.5)·T50에 아티팩트 마셜 whitelist 단얫(R-3).
- **D (M-1~M-5)**: J7 "§16.1 GET 8종 중 4종 서브셋" 정정·미구현 GET 4종 §13 등재·§9.3 어휘 10종 정정(3곳)·unreachable outcome 매핑 정의(§3.3)·커서 지속은 이행·재개·incremental 소비 계약은 Phase 4 명시적 이연(§3.3·§13)·J1에 §14.1 facade 문구 인용·처분.
- **E (M-1/2/4/5)**: tolerance 상수 CountTolerance=0 정의(A11)·공개 generation 식별 규약(GenerationUID==RunUID)·투영 쿼리 `inventory/projection.go` PR 22 배치·§12 mapping_test.go P22 단일 소유·게이트웨이 모의 자격 입력 정의(§3.1·T45)·64KiB 계획 도입값으로 출처 정정(A12)·image-tag 죽은 예시 명시(§3.2).
- **F (R-1/R-4/R-5/R-6/R-7)**: §11 PR 23 조건부 롤백 정정(ensureMenu upsert-only 실측 §0.7)·게이트 윈도우 운영 지침 신설(§0.1)·EvaluateGate "최신 3개 무조건 전부 pass" 확정 + T54 streak 차단 케이스(§3.5·§6)·§13에 Phase 3 CI-kind 결정 인계·M1 선언 전 §18.2 앵커·§0.2 재실측(설치 완료 상태).
- **G (LOW)**: §0.6 "V1 REMAIN" 표기 §3.1 어휘 정합으로 정정·contract/adapter.go 주석 갱신 허용(보존 제약 #2)·mapping.md 경로 편차 명시(§2 각주)·sync 리포트 경로 규격(§2 운영 산출)·stale 행 가시 공백 운영 가정(§13).

- 원천: `/mnt/d/DEV/acc0mplish/ops-admin/docs/architecture/multi-infrastructure-control-plane-v2.md` r2 + 정정 r2.1–r2.5 — 핵심 절: **§15 (비교 프로토콜 — 본 Phase의 핵)**·§20 Phase 2 게이트·§21 PR 20–23·§14.1 (K8s 매핑)·§16.1 (API v2)·§17 (UI)·§5.4 (전파 규칙)·§8 (리소스 신원·관측)·§9 (동기화·조정 결과)·§23.1/23.2/23.4 (계약 하네스·kind·N8/N9/N12).
- Phase 1 자산 (p1-state "done", PR #14–#18): `backend/internal/infra/{contract,registry,adapter/fake,model,migrate,secrets}`·`backend/internal/tasks` — 12 테이블·브로커·엔진·`ConnectionView.Material` 확장 완료.
- Phase 1 인계 소유: **contract test harness (Phase 1 r2 E 이연분 — PR 20 소관)**. N-1~N-4(§13 r2 정의 — N-1의 Discover 경로는 Phase 2가 해소)·Phase 0 인계 2·3은 계속 이월(§13 대장 등재 완료).
- 범위 (§21): **PR 20** k8s discoverer + normalizer + mapping.md + contract test harness · **PR 21** shadow sync + reconciliation outcomes (§5.4 전파·백필 재실행·stale-source 마킹 포함 — §20 Phase 2 Work 문구) · **PR 22** compare-inventory 명령 + 리포트 아티팩트 (§15) · **PR 23** web Infrastructure 셸 + provider 목록 (additive — /api/v2).
- 출구 게이트 (§20 Phase 2): ① §15.4 pass — 3회 연속 clean paired runs ② identity-conflict 0 ③ 최대 클러스터 sync가 rate budget 내 (`provider_rate_limit_total` flat) ④ UI가 V2 K8s 인벤토리 표시 (additive routes).
- 계획 계약: `phase1-plan.md` 관례 승계 (§0 실측 → 판단 J → 배치표 → 인터페이스 계약 → Phase 분해 ≤5파일 → 테스트 계약 → claims → 보존 제약 → 리스크 → 롤백 → 소유권 → 가정 A).

---

## 0. 현재 코드·자산 실측 — 게이트 실현가능성 판정 (계획의 사실 기반)

### 0.1 게이트 실현가능성 판정 (요약 최상단 반복 내용)

| 게이트 | 판정 | 근거 | 전제 |
|---|---|---|---|
| ① 3회 연속 clean paired runs (§15.4) | **조건부 실현 가능** | kind+kubectl 설치 완료(§0.2 r2 재실측)·클러스터 구축은 R0 잔여 — 시드 클러스터 도입으로 실현 경로 확보 | R0 클러스터 구축·시딩·v1 등록·**서로 다른 날 3회 실행(달력 게이트 — 최소 3일, 압축 불가)**·게이트 윈도우 운영 지침 준수(§0.1) |
| ② identity-conflict 0 | **실현 가능** | 테스트(T48) + sync 리포트 아티팩트의 결과 카운트로 기계 증명 | 없음 |
| ③ rate budget (`provider_rate_limit_total` flat) | **실현 가능하나 계측이 먼저** | 메트릭이 어디에도 구현돼 있지 않음(§0.4) — Phase 2가 최소 계측을 PR 20에서 소유해야 증명 성립 | 카운터 구현 + 반증 가능성(429 모의 시 카운터 증가 단얫 — R5) |
| ④ UI V2 인벤토리 표시 (additive) | **실현 가능** | Vue3+vite·라우터 132·i18n 2로케일(ko-KR/en-US)·메뉴 DB 시드 관례 전부 실측(§0.7) — additive 경로 확립 | 골든 아티팩트(route-inventory.txt) 재생성 |

**에스컬레이션 (실측 불가·승인 필요 항목 — 2026-09-06 승인 완료)**:
- **E-1 (설치 단계 완료 — r2 재실측)**: k8s 클러스터 접근 도구 설치 승인·실행 완료. 실측(r2): `~/go/bin/kind`·`~/go/bin/kubectl` 존재(2026-09-06 23:39 타임스탬프)·`k3d`는 미설치(K3s 실증은 A3대로 유닛 테스트로 충분). `~/.kube/config` 부재 — **클러스터 구축·시딩은 R0 잔여**(§5 사전 Phase). 승인 당시 원문: 설치 거부 시 게이트 ①③의 실클러스터 증명 불능 → 대안은 httptest 모의 API 서버만으로 어댑터 검증 + 게이트 아티팩트는 "모의 대상"으로 한정 명시 후 재에스컬레이션.
- **E-2**: legacy `k8s_cluster` 테이블 **0행** (dev MySQL `ops_admin` 실측 2026-09-06). 페어 비교(§15.1)는 "같은 클러스터에 대한 legacy 파이프라인 vs V2 파이프라인" 비교이므로 프로덕션 데이터가 아니어도 성립 — kind 시드 클러스터를 v1 API(kubeconfig 등록)로 등록한 행이 legacy 측이 된다. 이 해석(가정 A1)의 승인.
- **E-3**: §15.4 "on different days" — 3회 페어 실행이 서로 다른 달력 날짜여야 한다. 아티팩트 파일명 날짜로 증명. Phase 종결 최소 3일 소요는 게이트의 성격이며 우회(같은 날 3회)는 게이트 위반으로 명시.

**게이트 윈도우 운영 지침 (r2 신설 — 게이트 ①의 3일 창 집행 규칙)**:

1. **창 중 kind 클러스터 재생성 금지.** `kind delete`→재생성은 클러스터 신원과 전 리소스 UID를 일괄 변동시킨다 — 비교 엔진 입장에서 구조적 BLOCKER(identity 집합 전면 불일치)이며 게이트 아티팩트를 무효화한다.
2. **부득이 재생성 시 3-run 클럭 리셋** (§15.4 "restart the 3-run clock"의 운용 착지): 기존 페어 아티팩트 무효 표시(디렉터리 삭제 또는 리포트 `voided=true` 마킹) → kubeconfig v1 재등록(신규 `k8s_cluster` 행) → 백필 재실행(§5.4 증분 — 신규 행 생성·기존 V2 행 stale_source 마킹) → sync 재실행 후 페어 1회차부터 재시작.
3. **시드 매니페스트 변경(리소스 증감)도 동일 취급** — 창 중 시딩 변경 금지. 리소스 수 변동은 DRIFT/재페어링이 흡수할 수 있으나 identity 집합 변동은 BLOCKER다.

### 0.2 로컬 k8s 접근 실측 (r2 재실측)

```text
kubectl  ~/go/bin/kubectl 존재 (2026-09-06 23:39 설치 — E-1 실행 완료)
kind     ~/go/bin/kind 존재 (2026-09-06 23:39 설치) · k3d 없음 (A3 — 불필요)
kube     ~/.kube/config 없음 — 클러스터 미구축 (R0 잔여)
docker   29.1.3 사용 가능 (Docker Engine on Ubuntu 24.04 WSL2, mem 32GB)
go       1.25.5 · bun 존재 (프론트 빌드 관례)
```

스펙 §23.2는 "Kubernetes: kind cluster in CI, seeded fixture (namespaces, workloads of each kind, pods in phases); K3s assertion via a k3d node job on a weekly cadence, not per-PR" — CI에 k8s 잡은 존재하지 않고(로컬 판정 방침 §0.8 승계) 로컬 등가가 게이트 경로다. K3s 실측은 선택(k3d 미설치 — distribution detection은 GitVersion 문자열 판정으로 유닛 테스트가 증명, 가정 A3).

### 0.3 데이터베이스 실측

- dev MySQL: 컨테이너 `ops-admin-mysql-dev` (mysql:8.0, 127.0.0.1:3306), DB `ops_admin` — v1 테이블 존재.
- `k8s_cluster` **0행** (E-2). `SHOW TABLES LIKE '%k8s%'` → `k8s_cluster` 단일 테이블 (관계 테이블 없음).
- **Phase 1 V2 12 테이블 미생성** (schema_migration·provider_* 전부 부재) — 러너는 main.go에 배선됐으나 새 바이너리가 이 DB로 아직 기동하지 않았다. Phase 2 착수 첫 단계에 앱 1회 기동(또는 동등 마이그레이션 실행)으로 0000–0003 적용이 필요 (Phase R0, §5).

### 0.4 `provider_rate_limit_total` 메트릭 구현 상태

**미구현.** `grep -rn "provider_rate_limit_total|provider_api_latency|provider_health" backend/` → Go 구현 0건. `/internal/metrics` 엔드포인트도 부재. §18.2 표의 "instrumented at adapter client wrapper"는 Phase 2가 첫 어댑터와 함께 최소 형상을 만드는 지점이다(판단 J6). 계측 없이는 게이트 ③의 "flat"이 증명 불가 — 측정 기구가 우선(리스크 R5).

### 0.5 스키마 갭 — 전파 규칙(§5.4)에 필요한 컬럼 부재

Phase 1 모델(`internal/infra/model/provider.go`)에 `stale_source`·소스 추적 컬럼 없음. §5.4 (a) 백필 "checkpointed per row, keyed on updated-at"·(b) v1 삭제 → "marks the V2 rows `stale_source=true`"·(c) "the V2 read side treats `stale_source` rows as absent" 를 수행하려면 소스 행 식별·체크포인트가 필요 → **step 0004 Expand**(판단 J3): `provider_connection`에 `source_model`·`source_id`·`source_updated_at`·`stale_source` 4컬럼.

### 0.6 legacy K8s 코드 실측 (§3 disposition matrix row 11 — K8sCluster = **SUPERSEDE**: "backfilled in shadow, authoritative only after cutover (M2)" — 백필의 원천이며 M2 컷오버까지 v1이 권위, shadow 병행)

- `backend/service/k8s.go` 5,062줄. `GetK8sClusterDetail`(:781) — 프로세스 캐시 `k8sOverviewCache` **TTL 15초**(:779), singleflight. 캐시 우회 경로 `getK8sClusterDetailUncached`는 미export.
- **§15.1 "with the process cache bypassed"의 무수정 실현**: compare CLI가 `service.New(db)`로 **fresh Service 인스턴스**를 만들면 캐시가 빈 상태로 시작해 첫 `GetK8sClusterDetail` 호출이 곧 uncached 경로다 (판단 J4). `service.New`는 스케줄러 5종+DNS 매니저를 기동하나 CLI는 종료 시 `svc.Shutdown(ctx)`로 정리(main.go 본체와 동일 패턴).
- `K8sCluster` 모델(model/k8s.go:5-24): APIServer·Version·NodeCount·Env·Tags·ConnectionMode·GatewayID·MonitorDatasourceID·KubeConfig·LastSyncAt — 백필 매핑의 원천 필드 전부 여기 있음.
- `K8sClusterDetail` DTO: Cluster·Overview·Nodes·Namespaces·Pods·Workloads·Network·AdvancedNetwork·ConfigStorage 9섹션 — §15.2 mapping.md의 coverage rule 대상("every field the legacy API response serializes").
- `fetchK8sData`(:2495): REST 경로 직접 조회(`/api/v1/nodes`·namespaces·pods·services·endpoints·`/apis/networking.k8s.io/v1/ingresses`·GatewayAPI…) — **client-go를 쓰지 않는 raw REST 클라이언트**. `kubeClusterRuntime`(:64) = kubeconfig 파싱 결과(Server·TLS·Token·Basic·ClientCert).
- `k8s.io/client-go v0.34.1` 등 k8s 의존은 go.mod에 **이미 존재** (신규 의존성 0 — 보존 제약 #5와 정합).
- `/version` GitVersion 조회 루틴 이미 존재(:4363) — distribution detection(k3s 감지)의 원천.
- **(r2) legacy workload 수집 종 = 6종 실측**: `fetchK8sData`가 `/apis/apps/v1/{deployments,replicasets,statefulsets,daemonsets}`·`/apis/batch/v1/{jobs,cronjobs}` 전부 조회(:2566–2595). V2 discoverer의 workload 3종(deploy·statefulset·daemonset)과의 차이 — **ReplicaSet·Job·CronJob은 "legacy 수집·V2 미수집" 종**으로 비교 스코프 규칙이 처분한다(§3.5).
- **(r2) legacy 직렬화에 `uid` 전무 실측**: `K8sNodeItem`(model/k8s.go:164 — Name·Role·Status·Version·InternalIP·OS·CPU·Memory·Pods)·`K8sPodItem`(:191)·`K8sWorkloadItem`(:281)·`K8sNamespaceItem`(:182) 어디에도 UID 필드가 없다(`json:"uid"` 검색 0건). §15.2 표의 "identity by UID"는 V2 내부 신원 원칙이며, **비교 페어링 키는 종별 `namespace/name`(클러스터 스코프 종은 `name`)이다** (§3.5).
- **(r2) `K8sWorkloadItem` 리스트 직렬화에 image 필드 부재 실측**(:281-292 — Name·Type·Namespace·Ready·Updated·Available·Age·Requests·Limits). §15.3 BLOCKER 예시의 "image tag"는 상세 DTO(`K8sWorkloadDetail.Containers[].Image`) 기준 예시로, 페어 비교 대상(9섹션 리스트 직렬화)에서는 비교 불가 — 죽은 예시. mapping.md에서 workload.image는 "V2 단독 필드(비교 집합 외)"로 명시(§3.2).
- **(r2) StorageClass는 legacy 독립 수집 종 아님 실측**: `K8sStorageItem`(:416)의 `StorageClass string`은 PV/PVC 행의 속성 필드일 뿐 storageclasses 객체 목록 수집 경로는 부재 — V2의 storageclass 수집은 "V2 단독 종"(§3.5 스코프 표).

### 0.7 라우트·프론트·시드 실측

- 라우터: `engine.Group("/api/v1")` 단일 그룹, `/api/v2` 부재. 민감 라우트는 `opdef.Middleware(db, opdef.Must(...))` 패턴, 일반 GET은 Auth+OperationLog만. **골든 아티팩트 2종** `docs/security/{route-inventory.txt,sensitive-routes.txt}` — `routes_inventory_test.go`가 `-update` 플래그로 재생성, 평시 byte-diff 게이트. **라우트 인벤토리 실측 437엔트리**(스펙 §2.6 기술 시점 433에서 증가) — 본 계획의 "v1 무변경" 판정 기준은 437이다.
- 프론트: Vue 3.5 + vite (`bun run build`), `web/src/router/index.js` 라우트 132, views/ 도메인 디렉터리 관례. i18n: `SUPPORTED_LOCALES = ['ko-KR','en-US']`, 사전 19파일(`web/src/utils/*-i18n.js`), `scripts/check-i18n-parity.mjs` 패리티 게이트(신규 사전도 키 동치 강제).
- 메뉴: `store/seed.go` `ensureMenu`(:585) 관례 + `CanonicalizeSeedLocalization` — §17.3 "seed rows for the new menu group + items (one migration)"의 착지점. seed.go는 v1 파일이나 **스펙이 PR 23에 명시적으로 예외를 부여** (보존 제약 #2 예외 조항). **(r2) `ensureMenu`는 upsert-only 실측** — 삭제 경로 부재(Delete/Remove 검색 0건). PR 23 롤백 시 메뉴 행은 DB에 잔존 → §11 조건부 롤백 판정의 근거.

### 0.8 CI 로컬 검증 방침 (승계)

merge 판정은 로컬 명령, 원격 CI 대기 없음. `cd backend && go test ./... -race -count=1` + `cd web && bun run build`가 전체 판정 기저. 워크플로 무변경(보존 제약 #9).

---

## 1. 아키텍처 판단 (선택지 비교 + 선정 근거)

### J1 — k8s 어댑터의 트랜스포트: legacy 로직의 충실한 재구현(신규 패키지) (선정) vs service 함수 export (기각) vs service 패키지 import (기각)

- **기각(export 추가)**: `getK8sClusterDetailUncached`·`parseKubeConfig` 등을 export하면 v1 K8s 코드 수정이 된다(§3 row 11는 M2 커트오버까지 V1 운영 자산 — 본 Phase 보존 제약 #1).
- **기각(service import)**: `internal/infra/adapter/kubernetes` → `backend/service` 의존은 어댑터가 God service에 매달리는 역방향 계층 침범. arch rule 2(어댑터는 제어폴면 직접 쓰기 금지) 정신 위반.
- **선책**: 어댑터 패키지가 kubeconfig 파싱·REST 클라이언트·직접 연결 트랜스포트를 **자체 구현**으로 갖는다(legacy `kubeClusterRuntime`/`k8sGetJSON`과 동일 의미론, 코드 중복 ~200줄). 동등성은 §15 비교 프로토콜 자체가 증명한다(정확히 이것을 위해 존재하는 프로토콜). M2 decomposition step 2("client state moves out of Service")에서 통합 — 명시적 부채 기록(§13).
- **(r2) §14.1 facade 문구의 처분**: 스펙 §14.1은 "Existing Kubernetes code is first wrapped behind a facade, then split into clients, normalizers, discoverers, and operations."라고 서술한다. 이 facade→split 경로는 **K8s 도메인의 M2 분해 로드맵 전체**를 서술한 것이지 Phase 2의 V2 측 구현 경로가 아니다. Phase 2는 v1 `service/k8s.go`를 감싸는 facade를 만들지 않는다(감싸면 어댑터가 God service에 매달리는 역방향 의존 — J1 기각 사유와 동일). 대신 V2 계약 측 discoverer/normalizer를 신규 작성하고, facade 단계를 건너뛴 것이 아니라 **facade와 무관하게 병렬로 완성된 V2 측을 §15 프로토콜로 동등성 증명 후 M2 step 1(facade)·step 2(클라이언트 통합)에서 v1 측과 합류**한다. 부채 기록(§13 트랜스포트 중복 통합)과 동일 지점.
- 게이트웨이 모드(`ConnectionMode=gateway` + `GatewayID`): Connection.Config에 게이트웨이 파라미터를 실고 어댑터가 SSH 홉을 구성하되, **Phase 2 게이트 증명은 kind 직접 연결로 수행**(가정 A4). 게이트웨이 경로는 모의 게이트웨이 유닛 테스트까지만 — 모의 테스트의 자격 입력은 백필이 만드는 형상과 동일하게 `ConnectionView{Config: {"connection_mode":"gateway","gateway_id":"…"}, Material: {"inventory":"<kubeconfig>"}}`로 정의한다(§3.4 매핑·T45 변형 케이스).

### J2 — kubeconfig 시크릿 전달: 백필 → SecretRef(v2 envelope 그대로) → 브로커 purpose `inventory` → **DiscoverRequest.Connection 자격 전달** (r2 판정 — C-1/H-1 수렴)

`K8sCluster.KubeConfig`는 Phase -1 완료로 이미 v2 envelope(§4.1 row 8, G-1). `SecretRef.Ciphertext`는 "v2 envelope만 수용"(Phase 1 계약)이므로 **백필은 envelope 문자열을 그대로 복사**하고 `KeyID`를 SecretRef 필드에 기록 — 재암호화 불필요(동일 키세트·동일 형식). 자격증명 바인딩 `Purpose="inventory"`로 생성. kubeconfig는 로그·아티팩트·JSON 응답 어디에도 노출 금지(보존 제약 #8).

**(r2) 어댑터 전달 경로 확정 — `DiscoverRequest.Connection` 필드 추가.** r1의 공백: `Validate`/`Health`는 `ConnectionView`를 받으나 `Discover(req DiscoverRequest)`는 `ContextID`·`Cursor`만 받아서 **어댑터가 Discover 시점에 kubeconfig를 얻을 합법적 경로가 없었다**(Discovery에 ConnectionView를 재조회하면 어댑터가 DB·브로커를 알아야 함 — arch rule 2 위반). 착지: Phase 0가 `contract/adapter.go:51-52` 주석으로 예고한 "정규화 형태는 Phase 2 PR 20" 확장의 일부로 **`DiscoverRequest`에 `Connection contract.ConnectionView` 필드를 추가**한다.

- 시그니처(§3.1): `DiscoverRequest{ContextID uint; Cursor string; Connection ConnectionView}` — 호출자(compose→SyncRunner)가 브로커 `Resolve(ctx, connUID, "inventory")` 결과를 `Connection.Material["inventory"]`에 채운 `ConnectionView`를 구성해 전달한다. 어댑터는 `req.Connection`만 본다(DB·브로커 무지 유지).
- **N-1(계약 자격 전달 확장)의 잔존분**: `OperationRequest`에도 실행 자격 `Connection` 확장이 대응 필요하나 실행 경로는 Phase 3 — **Phase 3 인계로 명시**(§13 N-1). Phase 2는 Discover 경로만 착지.
- **기각: per-connection 어댑터 팩토리**(`NewAdapter(conn) → Discoverer`). 기각 사유: (1) registry는 타입별 싱글톤 조회(Phase 0 형상 — `Registry.For("kubernetes")`)인데 팩토리는 registry 서명 전면 교체·fake·harness·향후 모든 어댑터의 구성 방식을 다시 쓰게 한다. (2) 연결별 상태를 어댑터 인스턴스에 두면 동시 다중 클러스터 sync에서 인스턴스 수 명·수거·경합 관리가 새로 생긴다. (3) 요청 매개 자격 전달(선책)은 인터페이스 1필드 추가로 끝나고 시그니처 변경 영향이 소비자(fake뿐 — 실측) 전부 Phase 2 배치표 안이다.
- Phase 1 `ConnectionView.Material` 확장의 **첫 실소비자 주장은 이 착지 이후 유효**하다: Material["inventory"]가 브로커 Resolve → compose → DiscoverRequest.Connection → 어댑터까지 실제로 흐르는 경로가 Phase 2에서 완성된다(계약만 있고 소비자 없던 상태 해소).

### J3 — 전파 규칙 스키마: step 0004 — provider_connection 4컬럼

§5.4의 최소 형상: `source_model`(size:32, 'k8s_cluster')·`source_id`(uint)·`source_updated_at`(datetime — 증분 체크포인트)·`stale_source`(bool, default false, index). 합성 인덱스 `(source_model, source_id)`. **`stale_source`는 connection에만 둔다** — k8s는 클러스터당 context 1:1이므로 context·resource는 조인 파생으로 부재 처리(compare·read 경로는 `JOIN provider_connection WHERE stale_source=false`)(가정 A5). W-4 승계: 0000–0003 불변, 0004는 신규 스텝이며 provider_task를 건드리지 않으므로 postlude 의무 없음.

### J4 — compare CLI의 legacy 캡처: fresh Service 인스턴스 (§15.1 무수정 이행)

§0.6 실측 근거. `main_compare.go`가 `store.NewDB → service.New(db) → GetK8sClusterDetail(id) → svc.Shutdown` — 캐시가 비어 있으므로 "the same service methods the v1 API handlers call, with the process cache bypassed"가 문자 그대로 성립. v1 코드 무수정.

### J5 — contract test harness (Phase 1 이연분 인수): `internal/infra/contracttest` + contract 신호 에러 확장

§23.1 contract 티어("every adapter runs the same harness: discovery paging, error taxonomy, rate-limit signaling, redaction of raw payloads")의 첫 형상:
- `contract.ProviderSignalError{Kind: rate_limited|permission_denied|unreachable}` 를 `contract/adapter.go`에 추가 — §9.3 outcome 어휘(rate_limited·permission_denied)와 1:1. sync runner가 이 신호를 outcome으로 매핑한다. (contract 패키지 확장은 Phase 1 `ConnectionView.Material` 확장과 동일 절차 — 소유 PR이 명시.)
- harness는 `Discoverer` + `Fixture`(시드·시나리오 주입 인터페이스)를 받아 4종 단얫을 떠는 공용 테스트 헬퍼. **fake가 즉시 소비**(하네스 자체의 측정 기구 검증 — fake에 rate-limit 발산 시나리오 추가)하고 k8s 어댑터가 같은 하네스를 통과(Q1의 fake·kubernetes 2행 확보).

### J6 — rate 메트릭 최소 계측: in-process 카운터 + 리포트 아티팩트 병기 (엔드포인트는 이연)

게이트 ③은 "`provider_rate_limit_total` flat" — 증명 대상은 값이지 엔드포인트가 아니다. PR 20이 `internal/infra/metrics`(카운터 + Prometheus text 렌더러)를 만들고 어댑터 REST 래퍼가 429/Retry-After를 `IncRateLimit("kubernetes")`로 계측, **sync·compare 리포트 아티팩트에 렌더링 값을 병기**한다. `/internal/metrics` 엔드포인트와 §18.2 전체 표는 Phase 3+ 소관으로 명시적 이연(§13) — v1 라우터 무변경을 지키는 최소 형상. 반증 가능성: 429 모의 → 카운터 증가 단얫(R5 완화).

### J7 — v2 API의 Phase 2 서브셋: §16.1 GET 8종 중 4종 (r2 정정 — r1 "GET 4종만"은 마치 §16.1에 GET이 4종만 있는 것처럼 읽힘)

§16.1의 읽기 라우트는 **GET 8종**(provider-types·provider-connections·provider-connections/{uid}·provider-contexts·resources·resources/{uid}·resources/{uid}/relationships·resources/{uid}/operations — 실측). PR 23이 여는 것은 그중 4종: `GET /api/v2/infra/provider-types`·`GET /api/v2/infra/provider-connections`·`GET /api/v2/infra/resources`·`GET /api/v2/infra/resources/{uid}`. **미구현 GET 4종**(provider-connections/{uid}·provider-contexts·resources/{uid}/relationships·resources/{uid}/operations)은 §13 이연 등재 — relationships는 관계 데이터가 PR 21 relationship 적재로 이미 있으나 상세 라우트는 운영 UI와 함께, operations는 Phase 3(실행 계약 인계 후) 소관. `POST /provider-connections`·`/validate`·`/sync`도 Phase 2 범위 밖 — 연결 생성은 백필 경로가 유일하고 sync 트리거는 CLI(섀도 운영 관례), API화는 Phase 3 운영 워크플로와 함께(가정 A8). 라우트 보호: Auth + OperationLog 미들웨어(GET-only·시크릿 물질 무노출 → §4.7 민감 정의 비해당). **route-inventory.txt 골든은 재생성 갱신, sensitive-routes.txt는 불변**(검증 claim 11).

### J8 — kind 시드 클러스터·픽스처: `v2-phase1/k8s-fixture/` (r2 — 시드 5종 유지 판정)

§23.2 시드 요건(namespaces·workloads of each kind·pods in phases)을 매니페스트+셋업 스크립트로: Deployment·StatefulSet·DaemonSet·Job·CronJob·독립 Pod(Running/Pending/Succeeded/Failed)·Service·Ingress·ConfigMap·Secret·PVC + 다중 namespace. kubeconfig를 v1 API로 등록하는 절차 포함(백필·페어 비교의 legacy 행). 스크립트는 `scripts/` 기존 파일 무수정 원칙 하에 `scripts/v2/` 신규(§0.8 보존 제약 #9는 check-arch-boundary.sh·workflow 파일 한정).

**(r2) 시드 workload 5종 vs V2 discoverer 3종 — 시드 5종 유지 판정.** 근거: (1) 시드 클러스터는 legacy 캡처의 실험 대상이기도 하다 — legacy는 workload 6종을 수집(§0.6 r2 실측)하므로 Job·CronJob이 시드에 존재해야 legacy 직렬화 필드(Ready/Updated/Available의 종별 형상 차이)가 실데이터로 검증되고 coverage rule(T51)이 공허 섹션 없이 성립한다. (2) 시드를 3종으로 축소하면 legacy 섹션은 남는데 V2·시드 양쪽에서 사라져 비교 프로토콜의 실증 폭이 줄어든다. (3) V2 미수집 종은 §3.5 비교 스코프 규칙이 "비교 집합 외"로 흡수하므로 게이트 ① 구조적 실패가 없다. (4) ReplicaSet은 시드에 명시 심지 않는다 — Deployment가 파생 생성하는 종이므로 자동 등장하며, 스코프 표에 "legacy 수집(파생)·V2 미수집"으로 미리 등재해 오탐을 방지한다(§3.5). V2의 Job·CronJob·ReplicaSet 수집 확장은 Phase 3(restart 대상 workload 확정)·M2(완전성) 소관(§13).

### J9 — 리소스 kind 어휘 확장 2종: `orchestration.configmap`·`orchestration.secret` (mapped-subtype 병용)

§14.1이 Phase 2 K8s 리소스로 configmap·secret metadata를 요구하나 §8.5 닫힌 어휘(IsKnownResourceKind)에 구성계열 종이 없다. 선택지: (a) 어휘 확장 2종 — `resource_kind.go` 주석이 명시한 "Vocabulary extensions land as reviewed diffs to M1ResourceKinds in the owning milestone's PR" 경로 그대로, 확장은 종 2개 최소. (b) configmap/secret을 관측 전용으로 떨구기(리소스행 미생성) — §14.1 "Resources: … configmap, secret metadata"가 리소스로 명시하므로 위반. (c) 남의 종에 우겨넣기 — kind 오용은 §22 #4(미검증 CMDB)로 회귀. **선책 (a)**: 2종 확장 + service/ingress는 `network.load_balancer`+Subtype, pv/pvc는 `storage.volume`+Subtype, storageclass는 `storage.pool`+Subtype으로 기존 어휘를 재사용(확장 최소화). 어휘 확장은 리뷰 게이트가 확인하는 diff가 된다(T42·T51).


---

## 2. 수정 대상 (배치표 — 전 파일 경로 명시)

### PR 20 — k8s discoverer + normalizer + mapping.md + contract harness (`feat(inventory)` · §21 #20)

| # | 파일 | 신규/수정 | 내용 요약 |
|---|---|---|---|
| 1 | `backend/internal/infra/adapter/kubernetes/client.go` | 신규 | kubeconfig 파싱(J1 재구현 — `kubeClusterRuntime` 동일 의미론)·REST 클라이언트·게이트웨이 모드 구성(직접 연결 게이트 증명, A4)·429 계측 래퍼(J6) |
| 2 | `backend/internal/infra/adapter/kubernetes/adapter.go` | 신규 | BaseAdapter(Descriptor "kubernetes"·ContextKinds ["cluster"]·Validate/Health — /version·distribution detection GitVersion `+k3s` 판정) + Discoverer(리소스 종별 커서 페이징) |
| 3 | `backend/internal/infra/adapter/kubernetes/normalizer.go` | 신규 | §14.1 리소스 11종의 `DiscoveredResource` 정규화·URN 빌더(`urn:k8s:{context}:{kind}:{scope}/{id}`)·단위 정규화(Ki→GB)·secret metadata 전용(데이터 미수집) |
| 4 | `backend/internal/infra/adapter/kubernetes/mapping.md` | 신규 | §15.2 권위 매핑 표 — legacy `K8sClusterDetail` 경로 ↔ V2 정규화 필드 ↔ 규칙/dropped(reason), coverage rule 전필드 |
| 5 | `backend/internal/infra/contract/adapter.go` | **수정** | `ProviderSignalError` 추가(J5) + `DiscoverRequest.Connection contract.ConnectionView` 필드 추가(J2 r2 판정 — 어댑터 자격 전달 경로) + :51-52 주석 갱신("정규화 형태는 Phase 2 PR 20" 예고의 착지 기록). Phase 1 Material 확장과 동일 소유 절차 |
| 6 | `backend/internal/infra/contract/resource_kind.go` | **수정** | §8.5 어휘 확장 2종(`orchestration.configmap`·`orchestration.secret`) — 파일 주석이 명시한 "reviewed diffs in the owning milestone's PR" 경로 (판단 J9) |
| 7 | `backend/internal/infra/contracttest/harness.go` | 신규 | §23.1 contract 티어 — discovery paging(커서 종결·중복 0)·error taxonomy·rate-limit signaling·redaction 4종 단얫 + Fixture 주입 인터페이스 |
| 8 | `backend/internal/infra/metrics/metrics.go` | 신규 | 카운터 최소 형상(`provider_rate_limit_total`·`provider_api_errors_total`) + Prometheus text 렌더러 (J6) |
| 9 | `backend/internal/infra/adapter/fake/fake.go` | **수정** | rate-limit/permission 시나리오 발산(ProviderSignalError) — 하네스 측정 기구의 카나리 |
| 10 | `backend/internal/infra/adapter/kubernetes/adapter_test.go` | 신규 | httptest 모의 K8s API 서버(legacy 응답 구조 재현) — 정규화·URN·distribution·하네스 소비 |
| 11 | `backend/internal/infra/adapter/fake/fake_test.go` | **수정** | 하네스 소비 단얫 추가 |

### PR 21 — shadow sync + reconciliation outcomes + 백필 (`feat(inventory)` · §21 #21 + §20 Work의 backfill/stale-source)

| # | 파일 | 신규/수정 | 내용 요약 |
|---|---|---|---|
| 1 | `backend/internal/infra/migrate/step0004_connection_source.go` | 신규 | version 4: provider_connection Expand — 소스 추적 4컬럼 + 합성 인덱스 (J3) |
| 2 | `backend/internal/infra/migrate/migrations.go` | **수정** | step 0004 등록 1줄 |
| 3 | `backend/internal/infra/model/provider.go` | **수정** | 4컬럼 모델 필드 |
| 4 | `backend/internal/infra/model/model_test.go` | **수정** | 컬럼·인덱스 존재 단얫 + T41 집합 동치 유지(12테이블 불변 확인) |
| 5 | `backend/internal/infra/inventory/sync.go` | 신규 | SyncRunner — InventorySyncRun 수명주기·Discoverer 페이징 소비·infra_resource upsert(신원 유니크 충돌 → identity_conflict 보고)·observation(GenerationUID·hash)·relationship(pod→node runs_on·pvc→pv backed_by)·§9.2 퍼블리케이션(전체 완료 후 authoritative) |
| 6 | `backend/internal/infra/inventory/reconcile.go` | 신규 | §9.3 결과 어휘 **10종** 집계(created·updated·unchanged·stale_candidate·tombstoned·identity_conflict·normalization_failed·permission_denied·rate_limited·partial — 스펙 §9.3 verbatim)·stale_candidate 판정(반복 부재)·tombstone 임계·ProviderSignalError→outcome/상태 매핑(§3.3) |
| 7 | `backend/internal/infra/inventory/backfill.go` | 신규 | k8s_cluster → connection/context/secret_ref/credential_binding 백필 — updated-at 증분·재실행 가능·v1 소실 행 stale_source=true (§5.4)·KubeConfig envelope 그대로 복사(J2) |
| 8 | `backend/internal/infra/compose/compose.go` | 신규 | 조립 루트 — Registry(fake+kubernetes 등록)·Broker·metrics·SyncRunner/Backfill 구성 (CLI·v2 API 공유) |
| 9 | `backend/internal/infra/inventory/sync_test.go` | 신규 | T46–T48 (N8·N9·I3) |
| 10 | `backend/internal/infra/inventory/backfill_test.go` | 신규 | T49 — 증분 재실행·stale 마킹·빈 kubeconfig 스킵 |
| 11 | `backend/main_sync.go` | 신규 | `sync-inventory` 서브커맨드 — 백필+sync 실행·리포트 아티팩트 출력(outcomes+메트릭 렌더) |
| 12 | `backend/main.go` | **수정** | 서브커맨드 dispatch 1블록(main.go 기존 관례 — §0.6 실측 3선례와 동일 형상) |

### PR 22 — compare-inventory 명령 + 리포트 아티팩트 (`feat(tools)` · §21 #22 · §15 전체)

| # | 파일 | 신규/수정 | 내용 요약 |
|---|---|---|---|
| 1 | `backend/internal/infra/inventory/compare.go` | 신규 | 페어링(\|Δts\|≤60s·60초 초과 시 재페어링 1회·2회차 BLOCKER)·§15.3 분류 엔진(BLOCKER/VOLATILE/DRIFT/ABSENT — tolerance 상수 CountTolerance=0 포함, A11)·§15.4 판정(0 BLOCKER=pass)·게이트 체커(최신 3 아티팩트 무조건 전부 pass·날짜 3상이·동일 클러스터 단얫 — §3.5) — 순수 함수 |
| 2 | `backend/internal/infra/inventory/projection.go` | 신규 | (r2) §15.1 "read back through the V2 projection queries (not internals)"의 투영 쿼리 — `LatestAuthoritativeGeneration(db, contextID)`(최신 CommittedAt NOT NULL·Status succeeded run의 GenerationUID)·`ProjectResources(db, contextID, generationUID)`(stale_source=false 조인 — §5.4c). **PR 23의 GET /resources가 같은 투영을 소비**(PR 23 이전 배치 필수 — 직렬 순서가 이미 보장) |
| 3 | `backend/internal/infra/inventory/compare_test.go` | 신규 | T50 — N12(BLOCKER identity mismatch → 게이트 거부)·VOLATILE 무조건 허용·DRIFT 재페어링·60s 경계·아티팩트 마셜 whitelist 단얫(R-3) |
| 4 | `backend/internal/infra/adapter/kubernetes/mapping_test.go` | 신규 | T51 — mapping.md ↔ Go 매핑 테이블 동치 + coverage rule(legacy 직렬화 필드 전수 — mapped 또는 dropped(reason)) + 페어링 키·비교 스코프 표 동치(§3.5) |
| 5 | `backend/main_compare.go` | 신규 | `compare-inventory` 서브커맨드 — fresh Service legacy 캡처(J4)·projection.go 투영 조회(공개 generation)·분류·`data/compare/<cluster>/<date>/<time>.json` 데이티드 아티팩트(마셜 whitelist)·exit code(BLOCKER=non-zero) |
| 6 | `backend/main.go` | **수정** | dispatch 1블록 |

### PR 23 — v2 읽기 API + web Infrastructure 셸 (`feat(web)` · §21 #23 · §16.1 서브셋·§17)

| # | 파일 | 신규/수정 | 내용 요약 |
|---|---|---|---|
| 1 | `backend/internal/api/v2/infra.go` | 신규 | GET 4종 핸들러(J7) — provider-types(registry)·connections(시크릿 미노출 DTO)·resources(페이징·stale_source 제외)·resource detail(latest observation) |
| 2 | `backend/internal/api/v2/infra_test.go` | 신규 | T52 — additive 단얫·응답에 kubeconfig/SecretRef.Ciphertext 부재(리덕션)·stale_source 행 제외 |
| 3 | `backend/router/router.go` | **수정** | `/api/v2` 그룹 등록 (Auth+OperationLog — v1 그룹 무변경) |
| 4 | `docs/security/route-inventory.txt` | **수정** | 골든 재생성 (`-update` — 리뷰가 diff 확인) |
| 5 | `backend/store/seed.go` | **수정** | Infrastructure 메뉴 그룹+항목 시드 rows (§17.3 명시 예외 — 보존 제약 #2) |
| 6 | `web/src/api/infra.js` | 신규 | v2 API 클라이언트 |
| 7 | `web/src/views/infra/InfraOverview.vue` | 신규 | Infrastructure Overview 셸 |
| 8 | `web/src/views/infra/Providers.vue` | 신규 | provider 연결 목록 (PR 23 본체) |
| 9 | `web/src/views/infra/K8sInventory.vue` | 신규 | K8s 인벤토리 — kind 필터·리소스 표 (게이트 ④ 표시면) |
| 10 | `web/src/router/index.js` | **수정** | 라우트 3건 additive (132 불변) |
| 11 | `web/src/utils/infra-i18n.js` | 신규 | ko-KR/en-US 2로케일 (check-i18n-parity 대상) |

운영 산출(코드 아님 — 게이트 실행 자산): `v2-phase1/k8s-fixture/{seed.yaml,kind-setup.sh,README.md}` (J8, PR 21 시점 도입)·비교 리포트 아티팩트는 **`backend/data/compare/<cluster>/<date>/<time>.json`로 단일화**(r2 — r1의 §2/§3.5 경로 불일치 해소. 클러스터 디렉터리가 `--all` 다중 클러스터·게이트 "동일 클러스터" 판정을 경로에서 기계 가독하게 함. §15.1 "deployment's data directory" — 커밋하지 않고 게이트 증거로 tracking 이슈에 요약 첨부, 시크릿 레드적 후)·sync 리포트 아티팩트는 **`backend/data/sync/<connection-uid>/<date>/<time>.json`**(r2 — main_sync.go 출력 규격. outcome 10종·메트릭 렌더·리소스/호출 수 병기, claim 5·6·16의 증거 파일).

**(r2) mapping.md 경로 편차 명시**: 스펙 §15.2는 권위 매핑 표 위치로 `inventory/k8s/mapping.md`를 예시로 든다. 본 계획은 **`adapter/kubernetes/mapping.md`**에 둔다 — §15.2 자신의 원칙이 "lives next to the normalizer"이며 normalizer는 `adapter/kubernetes/normalizer.go`에 존재(§2 PR 20 파일 3). 스펙 경로는 r2 작성 시점의 예상 배치이고 원칙(정규화기 옆·Phase 2 PR 리뷰)이 실제 결정권자다. 편차는 이 문장으로 명시하고 mapping.md 헤더에도 원칙 인용을 기록한다(스펙 무수정 — 보존 제약 #3).

총산: **신규 27 + 기존 파일 수정 12종(유니크)**. 신규 27 = r1의 26 + `inventory/projection.go`(r2). 수정 12종 — `contract/adapter.go`·`contract/resource_kind.go`·`fake.go`·`fake_test.go`(PR 20)·`migrations.go`·`model/provider.go`·`model_test.go`(PR 21)·`main.go`(PR 21+22 dispatch 누적, 유니크 1파일)·`router/router.go`·`route-inventory.txt`·`store/seed.go`·`web/src/router/index.js`(PR 23). 기존 **v1** 코드 수정은 `main.go`(dispatch 블록 — 기존 서브커맨드 관례 내)와 `router.go`(v2 그룹 추가)·`store/seed.go`(스펙 명시 예외) 3종뿐.

---

## 3. 인터페이스 계약 (스펙 어휘의 정확한 코드화)

원칙 승계: 스펙이 준 것만 코드화. §15·§9.3·§5.4 문구는 어휘 수준에서 그대로.

### 3.1 k8s 어댑터 (`internal/infra/adapter/kubernetes`)

```go
// BaseAdapter + Discoverer (contract §11). 어댑터는 제어평면 테이블·DB 핸들
// 무소유 (arch rule 2). kubeconfig는 ConnectionView.Material["inventory"]
// (브로커 Resolve — 목적 정합)로만 받는다 (J2).
// (r2) Discover의 자격 경로: req.Connection.ConnectionView — 호출자(compose→
// SyncRunner)가 브로커 Resolve 결과를 Material["inventory"]에 채운 뷰를
// DiscoverRequest에 실어 전달한다. 어댑터는 req.Connection만 읽는다
// (DB·브로커 무지 유지 — J2 판정).
func NewAdapter() *Adapter
func (a *Adapter) Descriptor() contract.ProviderTypeDescriptor
    // Type "kubernetes", AdapterVersion "1", ProtocolVersion "1",
    // ContextKinds ["cluster"], BuiltIn true
func (a *Adapter) Validate(ctx, conn contract.ConnectionView) error
    // kubeconfig 파싱 + /version 조회 (distribution: GitVersion "+k3s" → "k3s")
func (a *Adapter) Health(ctx, conn contract.ConnectionView) contract.HealthResult
func (a *Adapter) Discover(ctx, req contract.DiscoverRequest) (contract.DiscoverPage, error)
    // req.Connection.Material["inventory"] = kubeconfig (게이트 증명은 직접 연결,
    //  A4). 게이트웨이 모의(T45 변형): req.Connection.Config의
    //  "connection_mode"="gateway"·"gateway_id" 키로 SSH 홉 구성 판정.
```

- **(r2) `DiscoverRequest` 확장**(contract/adapter.go — PR 20): `{ContextID uint; Cursor string; Connection ConnectionView}`. 시그니처 변경의 소비자는 `fake.go:87` 유일(실측)·신규 k8s 어댑터·contracttest harness — 전부 Phase 2 배치표 안이라 파장 없음. OperationRequest.Connection은 Phase 3 인계(§13 N-1 잔존분).

- **페이징**: 커서는 `<section>|<continue-token>` (section 순서 고정: nodes→namespaces→pods→workloads(deploy/statefulset/daemonset)→services→ingresses→configmaps→secrets→pv→pvc→storageclasses — §14.1 리소스 목록 순). 각 section은 K8s `continue` 토큰 위임. 마지막 section 종결 시 `NextCursor=""`.
- **URN (§8.1 "enough native scope to avoid collisions")**:
  - node: `urn:k8s:{ctx}:node:{metadata.uid}` (§15.2 표 예시 그대로 — UID 신원, name 표시 전용)
  - namespace: `urn:k8s:{ctx}:namespace:{name}` (클러스터 스코프 고유)
  - workload: `urn:k8s:{ctx}:workload:{namespace}/{kind}/{name}`
  - pod: `urn:k8s:{ctx}:pod:{namespace}/{name}` (pod 이름은 재생성되므로 UID 신원: `{metadata.uid}` — mapping.md에 규칙 명시, name은 display)
  - service/ingress/configmap/secret/pvc: `{namespace}/{name}`·pv/storageclass: `{name}`
- **kind 매핑 + 어휘 확장 (J9 판단)**: node→orchestration.node·namespace→orchestration.namespace·workload→orchestration.workload·pod→orchestration.pod·service→network.load_balancer(Subtype "service")·ingress→network.load_balancer(Subtype "ingress")·configmap→**orchestration.configmap(신규)**·secret→**orchestration.secret(신규)**·pv→storage.volume(Subtype "persistent_volume")·pvc→storage.volume(Subtype "pvc")·storageclass→storage.pool(Subtype "storage_class"). 확장 2종은 resource_kind.go 주석이 허용하는 "reviewed diffs" 경로(§2 PR 20 파일 6).
- **리덕션**: secret은 metadata만(name·namespace·type·data 키 목록 — data 값 절대 미수집, §14.1 "secret metadata"). Raw는 크기 상한 적용(§8.2 "Raw payloads have size limits" — 상한값 64KiB는 **계획 도입값, A12**. 스펙은 수치를 정하지 않음. normalizer 상수 `MaxRawBytes = 64 << 10`, 초과분 절단 + `normalized.truncated=true` 마커) 후 관측으로.

### 3.2 normalizer 스키마 (`mapping.md`와 동치 — T51이 기계 검증)

`NormalizedJSON` 키 체계(§15.2 표 형상): `node.{capacityCoresGB, capacityMemoryGB, allocatableCoresGB, allocatableMemoryGB, roles, taints}` (Ki→GB 단위 정규화)·`workload.{replicas, readyReplicas, image}`·`pod.{phase, restartCount}`·`namespace.{phase}`·`volume.{capacityGB, storageClassName, accessModes}`·`service.{type, clusterIP?, ports}`·`secret.{type, dataKeys}`. 상태 어휘표(`lifecycle_state`·`health_state` — node conditions→healthy/degraded, pod phase 그대로)는 mapping.md에 1페이지로 수록.

**(r2) mapping.md 필수 섹션 증보** — T51 동치 검증 대상:
- **종별 페어링 키 표**(§3.5): 종별 비교 페어링 키·V2 URN 성분·legacy 대응 경로.
- **비교 스코프 표**(§3.5): 양측 매핑된 종·단독 종(ReplicaSet·Job·CronJob·StorageClass)의 처분.
- **`workload.image`는 "V2 단독 필드(비교 집합 외)"로 명시** — legacy 리스트 직렬화(`K8sWorkloadItem`)에 image 필드가 없다(§0.6 r2 실측). §15.3 BLOCKER 예시의 "image tag"는 상세 DTO 기준 예시로 페어 비교(9섹션 리스트)에서는 비교 불가 — 매핑 표에 이 사유를 dropped(V2-only)로 기록. V2 정규화 필드 자체는 유지(§15.2 표 형상 준수).

### 3.3 sync runner (`internal/infra/inventory`)

```go
type SyncRunner struct { db *gorm.DB; reg *registry.Registry; broker *secrets.Broker; m *metrics.Counters }
type SyncInput struct { ConnectionUID string; Mode string /* "full"|"incremental" (§9.1) */ }
type SyncReport struct {
    RunUID    string
    Outcomes  map[string]int // §9.3 verbatim 10종 (r2 정정 — 스펙 어휘 그대로):
                           // created updated unchanged stale_candidate
                           // tombstoned identity_conflict
                           // normalization_failed permission_denied
                           // rate_limited partial
    StartedAt, CommittedAt, FinishedAt time.Time
    MetricsText string  // Prometheus 렌더 (게이트 ③ 증거 — J6)
}
func (r *SyncRunner) RunSync(ctx context.Context, in SyncInput) (SyncReport, error)
```

- InventorySyncRun 행 수명주기: 생성(running)→전 section 완료 시 CommittedAt 세팅+Status succeeded(§9.2 "a full generation becomes authoritative only after all pages complete" — N8: 부분 종료는 Status=partial·커밋 없음)→실패 시 ErrorCode.
- **(r2) 커서 지속은 이행·재개·incremental 소비는 Phase 4 이연** — §9.2 "Adapter cursors and rate-limit state are persisted"의 최소 이행: (1) SyncRunner가 페이지 루프마다 `InventorySyncRun.Cursor`에 현재 커서를 저장(run 행 — §9.1 스키마 필드, 신규 컬럼 불필요). (2) rate-limit state는 metrics 카운터의 리포트 아티팩트 동봉(J6 — 리포트가 지속 형상). **커서로부터의 재개(resume)·incremental 소비 계약은 구현하지 않는다** — `SyncInput.Mode`는 기록·보존하나 Phase 2 실행 경로는 full 모드 단순화. 근거: 섀도 운영의 유일 트리거는 단발 full sync(A9) — 재개 소비자가 없는 시점에 재개 계약을 만들면 검증 없는 코드가 된다(§22 정신). 소비 계약 확정·구현은 Phase 4(페이징 대용량 클라우드 어댑터 — 재개가 실제로 필요해지는 시점)로 §13 명시적 이연. 종료 상태가 succeeded인 run의 커서는 ""(종결)·partial/failed run은 최종 커서가 잔존한다(지속 형상의 관측 가능성).
- **(r2) ProviderSignalError→outcome/상태 매핑 정의** (§9.3 어휘 밖 신호의 처분 — 어휘 확장 금지, 스펙이 준 것만):
  - `rate_limited` → outcome `rate_limited` 카운트 + run Status=partial(또는 failed — 전 section 개시 전이면 failed·개시 후 중단이면 partial).
  - `permission_denied` → outcome `permission_denied` 카운트 + run Status=failed.
  - `unreachable` → **§9.3 어휘에 대응 outcome 없음** → outcome 맵에는 미가산, run Status=failed·`ErrorCode="provider_unreachable"`로 기록. 이미 집계된 outcomes는 리포트에 유지(부분 완료면 `partial` 카운트 병기). tombstone 0·리소스 잔존(T47/N9 — "provider outage is not resource deletion").
- **identity_conflict (게이트 ②)**: upsert 시 기존 `(context_id, kind, external_urn)` 행과 ExternalID 불일치(이름 변경을 다른 신원으로 착각하는 사례) → 병합하지 않고 보고(I3 — "reported, not merged").
- stale_candidate: 완료 generation에서 2회 연속 부재(§9.2 "repeated absence") → ManagedState=orphaned 표경로 stale_candidate 카운트. tombstone은 임계+유예 후(기본 3회/그 이상 반복 부재 — sync.go 상수).
- provider 중단(ProviderSignalError{unreachable}) → tombstone 0 (N9 — "provider outage is not resource deletion").

### 3.4 백필 (`internal/infra/inventory/backfill.go`)

```go
type BackfillReport struct { Created, Updated, Unchanged, MarkedStale, Skipped int; Checkpoint time.Time }
func RunK8sBackfill(ctx context.Context, db *gorm.DB) (BackfillReport, error)
```

- 매핑: `k8s_cluster` → ProviderConnection{ProviderType "kubernetes", Name, Endpoint: APIServer, GatewayID 승계, ConfigJSON{env,tags,connection_mode,version,node_count,last_sync_at}, Version, source_model "k8s_cluster", source_id, source_updated_at} + ProviderContext{Kind "cluster", ExternalID: strconv(v1 ID), Name} + SecretRef{Ciphertext: KubeConfig envelope 그대로, KeyID 파싱} + CredentialBinding{Purpose "inventory"}.
- 증분(§5.4a): `source_updated_at < k8s_cluster.updated_at`인 행만 재처험 — 재실행 시 변경분만.
- stale 마킹(§5.4b): `source_model='k8s_cluster' AND source_id NOT IN (SELECT id FROM k8s_cluster)` → `stale_source=true`.
- KubeConfig 빈 클러스터 → Skipped 카운트(에러 아님 — 등록 검증 상 존재하지 않아야 하나 방어).
- 멱등: UID는 소스 키(`source_model|source_id`)의 결정적 해시 — 재실행 동일 UID(A-계승: Phase 1 UID 생성 관례에 소스 파생 규칙 추가).

### 3.5 비교 엔진 (`internal/infra/inventory/compare.go` — §15 verbatim)

```go
type PairInput struct {
    ClusterID   uint                 // v1 k8s_cluster id
    Legacy      CapturedLegacy       // sections JSON + capture-start ts (§15.1 legacy_ts)
    V2          ProjectedV2          // 공개 generation 투영 + sync 완료 ts (v2_ts)
}
type CompareReport struct {
    Verdict    string   // "pass" | "blocker" | "re-pair" (§15.4 — DRIFT→재페어링 1회)
    Blockers, Volatiles, Drifts, Absents []Mismatch
    TsDelta    time.Duration
}
func ComparePair(in PairInput) CompareReport                 // §15.3 분류 순수 함수
func EvaluateGate(artifacts []string) GateResult             // (r2 확정) 시간순 최신 3개 무조건 전부 pass + 날짜 3상이 + 동일 클러스터 (§15.4 "3 consecutive passing paired runs on different days" — 세부 §3.5 r2)
```

- 페어링 규칙(§15.1): v2 sync 완료 직후 legacy 캡처·`(legacy_ts, v2_ts)` 기록·`|Δ|≤60s`여야 count/identity 비교 평가 개시. 60초 초과 캡처는 섹션 직렬 캡처로 1회 재페어링·휘발 섹션 제외, 2회차 초과는 BLOCKER 리포트(무한 재캡처 루프 금지).
- **(r2) identity 비교는 매핑 키 기준 — URN 직접 대조 아님**: legacy 직렬화에 `metadata.uid`가 전무하다(§0.6 r2 실측) → §15.2 표의 "identity by UID"는 V2 내부 신원(`infra_resource.external_urn` 성분) 원칙이고, **비교 엔진의 페어링 키는 종별 `namespace/name`(클러스터 스코프 종은 `name`)** 이다. 종별 페어링 키(mapping.md에 표로 수록·T51 동치):

  | 종 | 페어링 키 | V2 URN 성분(uid)과의 관계 |
  |---|---|---|
  | node | `name` | URN은 `{uid}` — 매핑 키로 name↔V2 DisplayName 대조 |
  | namespace | `name` | URN `{name}` — 일치 |
  | workload(deploy/statefulset/daemonset) | `{namespace}/{kind}/{name}` | URN과 동일 성분 |
  | pod | `{namespace}/{name}` | URN은 `{uid}` — 매핑 키로 name 대조(재생성 pod은 name 정합) |
  | service/ingress/configmap/secret/pvc | `{namespace}/{name}` | URN과 동일 성분 |
  | pv/storageclass | `{name}` | URN과 동일 성분 |

  비교 엔진은 페어링 키 집합의 차이를 BLOCKER("identity sets differ")로 판정하고, V2 URN 문자열 자체는 legacy에 대응 필드가 없어 대조하지 않는다.
- **(r2) 비교 스코프 규칙 — "매핑 표에 양측 매핑된 종·필드만 비교 집합"**: 게이트 ①의 구조적 실패 방지. 스코프 표(mapping.md 수록·T51 동치):

  | 종 | legacy | V2 discoverer | 처분 |
  |---|---|---|---|
  | node·namespace·pod·deployment·statefulset·daemonset·service·ingress·configmap·secret·pv·pvc | 수집 | 수집 | **비교 집합** — coverage rule 적용 |
  | ReplicaSet | 수집(Deployment 파생 자동 등장 — §0.6 r2) | 미수집 | 스코프 외 — legacy 단독 종으로 매핑 표에 `dropped(v2-not-collected)` 명시 |
  | Job·CronJob | 수집(시드 존재 — J8) | 미수집 | 스코프 외 — 동일 처분 |
  | storageclass | 미수집(속성 필드로만 — §0.6 r2) | 수집 | 스코프 외 — V2 단독 종으로 매핑 표에 `v2-only` 명시, 비교 대상 필드 없음 |

  단독 종은 count·identity 비교 집합에서 제외되며(집합 차이 BLOCKER의 오염원이 아님), 매핑 표에 처분이 기록되므로 coverage rule(T51)의 검증 대상에는 포함된다("mapped 또는 dropped(reason)" — 단독 종 reason은 위 표). V2의 Job·CronJob·ReplicaSet 수집 확장은 Phase 3/M2 소관(§13).
- **(r2) tolerance 상수 정의(A11)**: §15.3 "count fields differ beyond tolerance"의 tolerance는 스펙이 값을 정하지 않았다. 본 계획: `CountTolerance = 0`(compare.go 상수 — count 필드는 정확 일치, 1 차이도 BLOCKER). 근거: kind 시드는 정지 상태에서 60s 페어링 윈도우 내 캡처되므로 count 변동은 실제 변경이며 그것은 DRIFT→재페어링이 흡수한다. tolerance>0의 근거(캡처 중 페이지 경계 변경이 상시적인 클라우드 페이징)는 Phase 2 대상에 없다. 상수화로 추후 프로바이더별 오버라이드 여지를 남긴다.
- **(r2) 공개 generation 식별 규약**: 투영(projection.go)이 공개 generation으로 삼는 것은 **`GenerationUID == InventorySyncRun.UID`**(관측은 run과 1:1 — §8.2 "Every normalized record carries a generation"). 선정 쿼리: 해당 context의 `CommittedAt IS NOT NULL AND Status='succeeded'`인 run 중 최신(CommittedAt DESC) 1건의 GenerationUID를 가진 관측 + `stale_source=false` 조인(§5.4c). N8(부분 generation 비공개)과 N9(중단 시 이전 generation 유지)가 이 규약의 정합성을 테스트한다.
- **(r2) EvaluateGate 판정 확정**: 아티팩트를 시간순 정렬해 **최신 3개가 무조건 전부 pass** + 3개의 날짜 3상이 + 동일 클러스터(경로의 cluster 성분)일 때만 통과. pass의 연속 streak(전체 이력 기준)이 아니다 — §15.4 abort의 "restart the 3-run clock"과 정합: fail이 최신 3개 창 밖으로 밀리면 클럭 리셋으로 재시작한 것으로 본다. 단, abort 기준(2연속 BLOCKER)은 EvaluateGate과 별개의 compare 리포트 판정이 유지한다.
- 분류(§15.3): BLOCKER(신원 집합 차이·tolerance 초과 count — CountTolerance=0·비휘발 필드 차이[단, 비교 집합 필드 한정 — 스코프 표])·VOLATILE(타임스탬프·ages·restart counts·observed-at·배열 순서 — 무조건 허용, 샘플 리포트)·DRIFT(Δ>60s count/status — 미평가·재페어링)·ABSENT(한쪽만 + mapping 표 dropped → 로그만).
- **(r2) 아티팩트 마셜 whitelist(R-3)**: 아티팩트 JSON 작성기는 고정 스키마(whitelist 키)로만 마셜한다 — 리포트·페어 요약·outcoem·해시·버전 문자열. 원문 legacy 섹션 JSON·V2 투영은 시크릿 레드적 후 해시로만 싣는다(kubeconfig·secret data 미포함 단얫은 T50 whitelist 단얫 + T52/리뷰). whitelist 외 키가 직렬화에 등장하면 T50이 실패한다.
- 아티팩트: `data/compare/<cluster>/<date>/<time>.json` — 리포트 + 양측 해시(원문 시크릿 레드적 — kubeconfig·secret data 미포함 단얫은 T50/T52/리뷰).
- abort(§15.4): (재페어링 후) 2연속 BLOCKER → 해당 클러스터 개발 중단 보고. sync 자체의 identity_conflict는 즉시 abort(게이트 ②와 동일 어휘).

### 3.6 metrics 최소 형상 (`internal/infra/metrics`)

```go
type Counters struct { /* mu + map[string]uint64 (provider) + map[errKey]uint64 */ }
func New() *Counters
func (c *Counters) IncRateLimit(provider string)      // 어댑터 REST 래퍼 — 429/Retry-After
func (c *Counters) IncAPIError(provider, op, code string)
func (c *Counters) Render() string                     // Prometheus text (§18.2 라벨 형식)
```

### 3.7 contracttest harness (`internal/infra/contracttest`)

```go
type Fixture interface {
    Seed() []contract.DiscoveredResource       // 시드 리소스
    PageLimit() int                            // 페이지 크기 강제 (페이징 단얫)
    Scenario(name string) error                // "rate_limited"|"permission_denied"|"unreachable" 발산 주입
}
func RunContractSuite(t testing.TB, d contract.Discoverer, fx Fixture)
// 단얫 4종 (§23.1): (1) discovery paging — 커서 순회 종결·리소스 중복 0·총 수 == 시드
// (2) error taxonomy — 시나리오별 ProviderSignalError.Kind 일치
// (3) rate-limit signaling — rate_limited 시나리오가 계측 카운터를 증가
// (4) redaction — Raw에 Fixture가 심은 가짜 시크릿 필드(marker) 미포함
```

### 3.8 v2 API (§16.1 서브셋 — J7)

```text
GET /api/v2/infra/provider-types        → 등록 디스크립터 목록 (registry)
GET /api/v2/infra/provider-connections  → 연결 목록 (UID·type·name·endpoint·status·version·source 요약; ConfigJSON에서 시크릿 소재 제외 — SecretRef UID만)
GET /api/v2/infra/resources?kind=&page= → stale_source 제외(§5.4c)·kind 필터·페이징
GET /api/v2/infra/resources/{uid}       → 리소스 + 최신 observation (Raw 크기 상한·레드적 후)
```

공통: 표준 JSON 봉투(v1 ApiResponse 관례 준수)·Auth+OperationLog 미들웨어. tenant 헤더 없음(§16.2 — 서버 상수 "default"는 Phase 3 운영 라우트와 함께). **(r2)** GET /resources의 "최신 관측" 선정은 PR 22의 `inventory/projection.go`(§3.5 공개 generation 규약)를 소비한다 — P23은 projection.go를 수정하지 않고 호출만 한다(§12 교차 소유).

---

## 4. 임계경로 식별

| 대상 | 판정 | 근거 |
|---|---|---|
| PR 20 normalizer·mapping.md | **최우선 직렬** | §15 비교의 정확도 = 이 표의 정확도. coverage rule("every field the legacy API response serializes")은 노동 집중이며 게이트 ①의 직접 원천 |
| PR 21 sync/reconcile | **직렬** | 게이트 ②·N8/N9/I3의 소유자. 신원 유니크(upsert 경쟁)는 Phase 1 엔진과 동일 위험 등급 |
| PR 22 compare 엔진 | **직렬** | N12·§15.4 판정 로직 — 게이트 ①의 측정 기구. 오탐(정상 mismatch를 BLOCKER로)도 게이트 오류다 |
| PR 20 어댑터 트랜스포트 | 위험 비례 | J1 재구현 — §15가 동등성 증명. 표준 REST라 난도는 중 |
| PR 23 전체 | 위험 비례 | additive 셸 — 게이트 ④ 표시면. 골든 재생성 절차 준수가 전부 |

직렬 순서(강제): **PR 20 → PR 21 → PR 22 → PR 23(D1 백엔드 → D2/D3 프론트)**. 병렬 창: PR 23 D1(v2 핸들러·골든)은 PR 21 B1(스키마) 머지 후 PR 22와 병렬 가능(파일 무교차). PR 22의 mapping_test.go는 PR 20의 mapping.md에 의존하나 작성은 PR 21 리뷰 중 사전 가능(실행은 PR 20 머지 후).

---

## 5. Phase 분해 (구현 단위 ≤5파일)와 작업 순서

### 사전 Phase R0 — 환경·스키마 준비 (게이트 전제 — 코드 아님 + 앱 1회 기동)

1. (r2 갱신) kind+kubectl 설치는 **완료**(§0.2 재실측) — 잔여: 시드 클러스터 구축(`v2-phase1/k8s-fixture/`)·kubeconfig v1 등록(legacy 행 확보 — E-2). 구축 후 게이트 윈도우 운영 지침(§0.1)이 착수 시점부터 적용.
2. dev MySQL에 Phase 1 마이그레이션 적용 확인(앱 1회 기동 — §0.3).

### PR 20 (신규 7 + 수정 4 → 3 Phase)

- **Phase A1 — 어댑터 코어 (4파일)**: `client.go`, `adapter.go`, `normalizer.go`, `mapping.md`. 의존: R0 없음(모의 서버 테스트). 검증: `go test ./internal/infra/adapter/kubernetes/`(T42·T43) + `go build ./...`.
- **Phase A2 — 하네스·메트릭·어휘 (5파일)**: `contracttest/harness.go`, `metrics/metrics.go`, `contract/adapter.go`(ProviderSignalError), `contract/resource_kind.go`(어휘 2종), `fake.go`(시나리오 발산). 의존: A1. 검증: `go test ./internal/infra/contracttest/ ./internal/infra/metrics/ ./internal/infra/contract/`.
- **Phase A3 — 하네스 소비 단얫 (2파일)**: `fake_test.go`(수정), `adapter_test.go`. 의존: A2. 검증: fake(T44)·모의 API 서버 k8s 어댑터(T45)가 동일 하네스 통과 — Q1 2행.

### PR 21 (신규 8 + 수정 4 → 3 Phase)

- **Phase B1 — 스키마 (4파일)**: `step0004_connection_source.go`, `migrations.go`(1줄), `model/provider.go`, `model_test.go`. 검증: 0003→0004 순차 적용·컬럼·인덱스 단얫·T41 집합 동치 유지.
- **Phase B2 — sync 엔진 (3파일)**: `sync.go`, `reconcile.go`, `sync_test.go` (fake discoverer로 — 어댑터 무관 계층). 의존: B1. 검증: N8·N9·I3·outcomes 10종·커서 지속(§3.3 r2)·unreachable 매핑(Status=failed·ErrorCode·tombstone 0).
- **Phase B3 — 백필·조립·CLI (5파일)**: `backfill.go`, `backfill_test.go`, `compose/compose.go`, `main_sync.go`, `main.go`(수정). 의존: B2 + PR 20. 검증: k8s-fixture 시드 클러스터 대상 실sync 1회 — 리포트 아티팩트(`data/sync/<conn-uid>/…` — 게이트 ②③ 증거의 첫 샘플).

### PR 22 (신규 5 + 수정 1 → 2 Phase — r2 projection.go 추가)

- **Phase C1 — 비교 엔진·투영 (4파일)**: `compare.go`, `projection.go`, `compare_test.go`, `adapter/kubernetes/mapping_test.go`. 검증: N12·60s 경계·VOLATILE 허용·매핑 동치(페어링 키·스코프 표 포함)·whitelist 마셜·투영이 부분 generation을 공개하지 않음.
- **Phase C2 — CLI (2파일)**: `main_compare.go`, `main.go`(수정). 검증: 시드 클러스터 대상 페어 실행 1회 — pass/BLOCKER 양상 확인·아티팩트 생성(`data/compare/<cluster>/<date>/<time>.json`).

### PR 23 (신규 7 + 수정 4 → 3 Phase)

- **Phase D1 — v2 읽기 API (4파일)**: `api/v2/infra.go`, `infra_test.go`, `router/router.go`(수정), `docs/security/route-inventory.txt`(재생성). 검증: T52·v1 라우트 437 불변·sensitive-routes.txt 무변경.
- **Phase D2 — 백엔드 시드·프론트 데이터층 (5파일)**: `store/seed.go`(메뉴 시드), `web/src/api/infra.js`, `views/infra/{InfraOverview,Providers,K8sInventory}.vue` 중 3 — 파일 수로 D2: seed.go+api.js+뷰 3종 = 5파일.
- **Phase D3 — 라우트·i18n (3파일)**: `web/src/router/index.js`, `web/src/utils/infra-i18n.js`, (필요시) 메뉴 라벨 사전 파일. 검증: `bun run build` + `node scripts/check-i18n-parity.mjs` + 브라우저 확인(게이트 ④).

작업 순서: `R0 → A1 → A2 → A3 → B1 → B2 → B3 → C1 → C2 → D1 → D2 → D3` (B1 이후 C1/C2와 D1 병렬 가능). 게이트 ①의 "다른 날 3회" 집행은 C2 완료 후 달력에 걸쳐 분산(§7 claim 4).

---

## 6. 테스트 계약 (TDD — 레드→그린, §23)

신규 V2 패키지(`infra/adapter/kubernetes`·`infra/inventory`·`infra/contracttest`·`infra/metrics`·`api/v2` + 확장되는 `contract`·`fake`)은 §23.3 문장 커버리지 ≥80%. 실클러스터 의존 테스트는 전부 httptest 모의 API 서버(§23.2 정신 — CI 무의존)로, kind 실증은 CLI 실행 아티팩트로 분리.

| # | 테스트 | 검증 | PR |
|---|---|---|---|
| T42 | `TestNormalizerBuildsURNAndUnits` | 종 11종 URN·Ki→GB·secret metadata 전용(data 값 부재) — 고정 JSON 픽스처 | 20 |
| T43 | `TestDistributionDetection` | GitVersion `v1.29.4+k3s1`→k3s·`v1.29.4`→kubernetes (C3 전주기) | 20 |
| T44 | `TestFakePassesContractHarness` | fake — 하네스 4종 단얫(페이징·분류·rate-limit 신호·리덕션) 통과 | 20 |
| T45 | `TestKubernetesAdapterPassesContractHarness` | 모의 API 서버 대상 동일 하네스 — Q1 fake·kubernetes 2행. **(r2)** + 게이트웨이 변형 케이스: `ConnectionView{Config:{connection_mode:gateway, gateway_id}, Material:{inventory:kubeconfig}}` 자격 입력으로 모의 게이트웨이 홉 구성 판정(§3.1·§3.4 형상과 동일 입력) | 20 |
| T46 | `TestSyncPublishesOnlyCompleteGenerations` | 부분 종료 시 이전 완료 generation 권위 유지·Status=partial·커밋 없음 (N8·I1) | 21 |
| T47 | `TestProviderOutageCreatesNoTombstones` | unreachable 신호 → tombstone 0·리소스 잔존 (N9·I2) | 21 |
| T48 | `TestIdentityConflictReportedNotMerged` | 신원 충돌 시 병합 없이 identity_conflict outcome 보고 (I3 — 게이트 ② 어휘) | 21 |
| T49 | `TestBackfillIncrementalAndStaleMarking` | 2회 실행 — 변경분만 재처리·소실 v1 행 stale_source=true·kubeconfig envelope 그대로·빈 kubeconfig skip (§5.4) | 21 |
| T50 | `TestCompareClassifiesPerSpec` | N12(BLOCKER identity → 게이트 거부)·VOLATILE 무조건 허용·Δ>60s DRIFT 재페어링 1회·2회차 BLOCKER abort·60s 경계값·ABSENT dropped 로그·**(r2)** 스코프 외 종(ReplicaSet·Job·CronJob legacy 단독 / storageclass V2 단독)이 identity 집합 BLOCKER를 유발하지 않음·아티팩트 마셜 whitelist 외 키 주입 시 실패(R-3)·CountTolerance=0 경계(1 차이 → BLOCKER) | 22 |
| T51 | `TestMappingTableCoversLegacyFields` | mapping.md ↔ Go 테이블 동치(종별 페어링 키 표·비교 스코프 표 포함 — §3.5 r2) + legacy 직렬화 필드 전수 mapped/dropped(reason)/v2-only (§15.2 coverage rule) | 22 |
| T52 | `TestV2RoutesAdditiveAndRedacted` | v1 437 라우트 불변·v2 4종 존재·응답에 kubeconfig/Ciphertext/SecretRef 물질 부재·stale_source 행 제외 (§5.4c) | 23 |
| T53 | `TestInfraMenuSeedAndI18nParity` | 메뉴 시드 멱등·기존 메뉴 무변경·infra-i18n ko/en 키 동치 | 23 |
| T54 | `TestEvaluateGateRequiresThreeDistinctDays` | 3 pass 아티팩트 같은 날짜 → 게이트 거부·서로 다른 날 → 통과 (§15.4 판정 로직). **(r2)** + streak 차단 케이스 2종(§3.5 EvaluateGate 확정): 시간순 [pass, fail, pass, pass] → 최신 3개 창에 fail → **거부** / [fail, pass, pass, pass] → fail이 창 밖 → **통과**(3-run 클럭 리셋 재시작과 정합) | 22 |

커버리지: `go test ./internal/... -cover` — 패키지별 수치 기재(claim 12).

---

## 7. 검증 요구 (claims — ④리뷰 주입용, 로컬 명령 형식)

1. `cd backend && go build ./...` exit 0.
2. `cd backend && go test ./internal/... -count=1` exit 0.
3. `cd backend && go test ./... -race -count=1` exit 0 (기존 회귀 0 포함 — CI backend-test 등가).
4. **게이트 ①**: ① `cd backend && go test ./internal/infra/inventory/ -run "TestCompare|TestEvaluateGate" -v -count=1` exit 0 + `=== RUN` ≥ 4 (T50·T54 — vacuous pass 봉쇄) ② 시드 클러스터 대상 `./ops-admin compare-inventory --cluster <id>`를 **서로 다른 날 3회** 실행 — 각 exit 0 + `ls backend/data/compare/<cluster>/ | sort -u | wc -l` ≥ 3(날짜 3상이) + `./ops-admin compare-inventory --gate --cluster <id>` exit 0 (EvaluateGate가 3 아티팩트 pass·날짜 3상이·동일 클러스터를 기계 판정). 아티팩트 요약을 tracking 이슈에 첨부.
5. **게이트 ②**: `./ops-admin sync-inventory --connection <uid>` 리포트 JSON에서 `identity_conflict == 0` — `jq '.outcomes.identity_conflict' <report>` → 0 **및** `go test ./internal/infra/inventory/ -run TestIdentityConflict -v -count=1` exit 0 (측정 기구 반증 가능성).
6. **게이트 ③**: 동일 리포트의 `MetricsText`에 `provider_rate_limit_total{provider="kubernetes"} 0` (r2 — §18.2 라벨 형식 정합) — `jq -r '.metricsText' <report> | grep 'provider_rate_limit_total{provider="kubernetes"}'` → 값 0 (flat) **및** 429 모의 하네스 단얫(T44 rate-limit signaling)이 카운터 증가를 잡음(반증 가능성 — R5).
7. **게이트 ④**: `cd web && bun run build` exit 0 + `node scripts/check-i18n-parity.mjs` exit 0 + `curl -s localhost:8082/api/v2/infra/resources?kind=orchestration.node` (시드 후·인증 토큰) — K8s 리소스 표시. 브라우저 확인 스크린샷 tracking 첨부.
8. `cd backend && go test ./store/ ./router/ -count=1` exit 0 — v1 회귀(fixture·라우트 인벤토리 포함, C1).
9. `bash scripts/check-arch-boundary.sh` exit 0 — 무발화 (k8s 어댑터가 provider 유형 분기 아님: 코어는 registry 조회만).
10. `git diff main -- backend/go.mod backend/go.sum` — 출력 없음 (client-go 기존 direct — 신규 의존성 0).
11. `cd backend && go test ./router/ -run TestSensitiveRoutes -count=1` exit 0 — sensitive-routes.txt 불변(v2 GET 4종은 비민감 판정·골든과 정합). `git diff main --name-only -- docs/security/` → route-inventory.txt만.
12. `cd backend && go test ./internal/... -cover -count=1 2>&1 | grep "coverage:" | grep -v "no statements" | awk '{pct=$NF; gsub(/%/,"",pct); if (pct+0 < 80) print "FAIL", $2, pct}'` — 출력 없음 (대상: adapter/kubernetes·inventory·contracttest·metrics·api/v2 + 확장 contract; 분모 왜곡 패키지는 수치+사유 기재).
13. `git diff main --name-only | grep -v "^backend/\|^web/\|^docs/security/route-inventory.txt\|^v2-phase1/"` — 출력 없음 (수정 범위 밖 이탈 0).
14. `grep -rn "kubeconfig\|KubeConfig\|kubeConfig\|Ciphertext" backend/data/ 2>/dev/null` — 출력 없음 (r2 — 카멜케이스 `kubeConfig` 변종 포함. 아티팩트 시크릿 무노출 — 리포트 레드적 단얫 + T50 whitelist 마셜).
15. `cd backend && go vet ./...` exit 0.
16. `cd backend && OPS_TASKENGINE_MYSQL_DSN='<dev DSN>' go test ./internal/infra/inventory/ ./internal/infra/migrate/ -run "TestMySQL|TestBackfill|TestSync" -v -count=1` exit 0 — 백필·sync의 MySQL 방언 검증(Phase 1 계층 2 관례 승계 — upsert·소프트델리트·created_at 방언. 게이트 종결 전 1회 실행 출력 첨부).

---

## 8. 보존 제약 (③구현 프롬프트에 verbatim 복사)

1. 기존 v1 Go 소스 중 수정 허용은 `backend/main.go`(서브커맨드 dispatch 블록 — 기존 inventory-secrets 관례 내 additive)·`backend/router/router.go`(`/api/v2` 그룹 등록 additive — v1 그룹·라우트 무변경)·`backend/store/seed.go`(Infrastructure 메뉴 시드 rows — §17.3 스펙 명시 예외, 기존 시드 무변경) 3종뿐이다. `service/`·`model/`·`controller/`·`opdef/`·`internal/domain/`·`internal/testutil/`·`util/`·기존 `web/` 파일은 무접촉 (특히 `service/k8s.go`·`model/k8s.go` — §3 disposition matrix row 11 K8sCluster = SUPERSEDE, M2 컷오버까지 v1이 권위이므로 본 Phase는 읽기 전용).
2. Phase 0/1 자산 중 수정 허용(r2 갱신): `internal/infra/contract/adapter.go`(**ProviderSignalError 1타입 + `DiscoverRequest.Connection ConnectionView` 필드 1개 + :51-52 주석의 착지 기록 갱신 — 이 3종 외 해당 파일 수정 금지**)·`contract/resource_kind.go`(어휘 2종 — 파일 주석의 reviewed-diff 경로)·`adapter/fake/fake.go`·`fake_test.go`(하네스 시나리오)·`internal/infra/model/provider.go`·`model_test.go`(0004 컬럼)·`internal/infra/migrate/migrations.go`(등록 1줄). §2 배치표와 동일 집합.
3. v2 스펙·에픽·ADR·`docs/architecture/**`·`docs/security/ci-baseline.md`·`sensitive-routes.txt`를 수정하지 않는다. 골든 재생성 대상은 `route-inventory.txt`뿐 (`go test ./router/ -run TestRouteInventory -update` — 리뷰가 diff 승인).
4. `backend/go.mod`·`go.sum`을 수정하지 않는다 — k8s.io/* 는 기존 direct 의존(실측 §0.6). kind/kubectl은 개발 도구이지 모듈 의존이 아니다.
5. 마이그레이션 스텝 0000–0003은 불변(W-4 승계) — 변경은 step 0004 신규 추가로만. 0004는 provider_connection만 건드린다(provider_task postlude 의무 비발생 — T41로 12테이블 집합 동치 유지 확인).
6. 어댑터(`internal/infra/adapter/kubernetes`)는 DB 핸들·제어평면 테이블 직접 쓰기 금지(arch rule 2) — inventory sync runner만 쓴다. 코어·sync는 provider 유형명 분기 금지 — registry 조회만(arch rule 1).
7. kubeconfig·SecretRef.Ciphertext·복호화 물질을 로그·에러 메시지·JSON 응답·비교 리포트 아티팩트에 노출하지 않는다 (claim 14). secret 관측은 metadata만.
8. `/api/v1` 437 라우트(라우트 인벤토리 실측 — §0.7)·기존 web 132 라우트·기존 메뉴는 무변경 — additive만 (§17.2 M1 additive 원칙).
9. `.github/workflows/*`·`scripts/check-arch-boundary.sh`·`scripts/check-i18n-parity.mjs`를 수정하지 않는다 — CI 잡 추가 없음(로컬 판정 방침). 신규 스크립트는 `scripts/v2/`·`v2-phase1/k8s-fixture/` 하위만.
10. §9.2 퍼블리케이션 규칙 위반(부분 generation 공개·중단 시 tombstone)을 구현하지 않는다 — N8/N9 테스트가 봉쇄한다. §15.4의 "서로 다른 날 3회"를 같은 날 실행으로 대체하지 않는다.
11. 상태·전파·비교 검증이 없는 "통과"를 보고하지 않는다 — 게이트 4조건(claims 4–7)은 명시된 명령 출력·아티팩트로만 증명한다.

---

## 9. 리스크 매트릭스 (가능성 × 파급) + 완화

| # | 리스크 | 가능성 | 파급 | 완화 |
|---|---|---|---|---|
| R1 | kind/kubectl 설치 불가(네트워크·정책) → 게이트 ①③ 실클러스터 증명 불능 | 낮음 | 높음 | E-1 사전 승인·설치는 착수 전 R0에서 실패 시 즉시 에스컬레이션(모의 서버 검증만으로는 게이트 불충족 명시) |
| R2 | legacy k8s_cluster 0행 — 백필·페어 대상 부재 | 확정(실측) | 중 | E-2 해석(§15는 파이프라인 비교 — 시드 클러스터 등록행이 legacy 측) 승인. 프로덕션 데이터 없이 Phase 종결 가능 |
| R3 | kubeconfig 시크릿 누출(아티팩트·로그·응답) | 중 | 높음 | 백필은 envelope 그대로 복사(평문 경유 없음 — J2)·브로커 Resolve만·claim 14 아티팩트 grep(kubeConfig 변종 포함 — r2)·T52 응답 레드적·**아티팩트 마셜 whitelist + T50 whitelist 단얫(r2 — whitelist 외 키 직렬화 시 실패)** |
| R4 | mapping.md coverage rule 노동 폭발(K8sClusterDetail 9섹션 전필드) | 높음 | 중 | 섹션별 dropped(reason)/v2-only 허용(§15.2 규칙 자체가 허용)·T51이 누락 필드를 기계 검출 — 매핑 누락은 "protocol bug"로 리뷰 게이트·**비교 스코프 표(§3.5 r2)가 단독 종 4종의 전수 등재 부담을 흡수** |
| R5 | "rate flat"이 자명해 증명이 공허(kind는 429 없음) | 높음 | 낮음 | 하네스 429 모의 → 카운터 증가 단얫(T44) — 측정 기구의 반증 가능성 확보. flat 주장은 카운터 존재 하의 0 관측으로만 |
| R6 | compare CLI의 fresh Service가 스케줄러·DNS 부작용 | 중 | 낮음 | 단발 프로세스 + Shutdown 정리(main.go 패턴 승계)·J4 문서화. 부작용 잔여는 프로세스 종료로 소멸 |
| R7 | J1 트랜스포트 중복(~200줄) — legacy와의 표류 | 중 | 낮음 | §15 비교가 동등성을 상시 검증·M2 decomposition step 2에서 통합(부채 기록 §13 — §14.1 facade 경로와의 합류 지점, J1 r2) |
| R8 | 달력 게이트(서로 다른 날 3회) — 일정 관리 실패 | 중 | 중 | T54가 게이트 체커를 기계 판정(같은 날 3회 거부·**streak 차단 케이스 포함 r2**)·C2 완료 즉시 1회차 실행 권고·Phase 종결 최소 3일은 계획에 명시(E-3)·**게이트 윈도우 운영 지침(§0.1 r2) — 클러스터 재생성 시 클럭 리셋·백필 재실행 절차 표준화** |
| R9 | v1 K8s 회귀(백필 읽기·compare가 v1 상태 변경) | 낮음 | 높음 | 백필·compare는 k8s_cluster **읽기 전용**(쓰기는 V2 테이블뿐) — claim 8 회귀로 봉쇄 |
| R10 | sqlite↔MySQL 방언(증분 upsert·stale 마킹 쿼리) | 중 | 중 | claim 16 계층 2 MySQL 1회 실행(Phase 1 관례 승계)·개발 DB가 MySQL이므로 B3 실sync가 자연 검증 |
| R11 | resource 어휘 확장(configmap·secret)이 레지스트리 가드와 충돌 | 낮음 | 낮음 | IsKnownResourceKind는 M1ResourceKinds 목록 조회뿐 — 확장과 동시에 자동 인지·T42가 종별 단얫 |
| R12 | kind 시드 클러스터의 휘발성(pod 재생성·노드 상태 변동)이 BLOCKER 오탐 | 중 | 중 | §15.3 VOLATILE/DRIFT 분류가 설계상 이것을 흡수(프로토콜의 존재 이유)·**페어링 키 namespace/name + pod name 정합(§3.5 r2 — UID 재발급은 페어링 불변)**·60s 페어링·스코프 표가 단독 종 오탐 차단 |

절충 요약: (a) 트랜스포트 중복(J1) — v1 무수정 안전성 vs 코드 이중화, M2 통합; (b) 메트릭 엔드포인트 이연(J6) — v1 라우터 무변경 vs §18.2 전체 계측, Phase 3+; (c) 게이트웨이 모드 실측 이연(A4) — kind direct 증명 vs 게이트웨이 경로 모의 검증, 실환경은 M2 운영; (d) API POST 라우트 이연(A8) — CLI 섀도 운영 vs §16.1 전체, Phase 3; **(e, r2) 커서 재개·incremental 소비 이연(§3.3) — 지속은 이행·소비는 Phase 4 대용량 페이징 시점, 검증 없는 계약 선행 방지**. 전부 재진입 시점 명시.

---

## 10. CI 로컬 검증 방침 (승계)

- merge 판정은 로컬 명령 완결: `go test ./... -race -count=1` + `bun run build` + §7 claims. v2-ci.yml 무수정 — 신규 패키지는 `./...` 와일드카드 자동 편입.
- 계층 2(MySQL) claim 16 — 게이트 종결 전 1회 실행·출력 전문 tracking 첨부(Phase 1 관례).
- 게이트 4조건 증거(claims 4–7 출력·아티팩트 요약)를 tracking 이슈 #4 체크리스트에 링크해 Phase 종결(§21 Resume protocol).

## 11. 롤백 가능성 판정

- **PR 20**: 신규 패키지 + contract/fake 확장 — PR revert로 완전 복귀. contract 어휘·`DiscoverRequest.Connection` 필드·fake 시나리오 제거와 함께 원상복귀(확장은 additive라 의존 역방향 없음 — 필드 제거의 소비자도 전부 Phase 2 내).
- **PR 21**: step 0004는 additive 컬럼 — revert 후 빈 컬럼 잔존 무해(소비자 부재). main.go dispatch 블록 제거로 CLI 소멸. 데이터(백필 행·sync 관측)는 V2 테이블 잔존 — §5.4가 M1 전 기간 v1 CRUD 병행을 전제하므로 잔존 행은 무해·재실행이 정합(증분).
- **PR 22**: CLI·투영 쿼리만 — 완전 롤백. 비교 아티팩트는 data dir 파일(삭제 가능).
- **PR 23 (r2 정정 — 조건부 롤백)**: v2 그룹·프론트·골든은 revert로 코드 완전 복귀(route-inventory.txt 재생성으로 복귀). 그러나 **메뉴 시드 행은 DB에 잔존** — `ensureMenu`는 upsert-only로 삭제 경로가 없다(§0.7 r2 실측). r1의 "재실행 부재로 정리됨" 서술은 오류였다. 잔존 행은 프론트 라우트 부재로 미노출(가시 영향 0)이나 DB 정리는 **수동(SQL delete) 필요** → PR 23은 "코드 완전 복귀 + DB 메뉴 행 수동 정리"의 조건부 롤백.
- 부분 롤백: 역순 스택(23→22→21→20) 전제 — 22는 21·20에, 23은 21(B1 모델)·22(projection.go)에 코드 의존. 최신 1 PR 단독 revert는 항상 성립.
- 판정(r2 갱신): **PR 20–22 완전 롤백 가능·PR 23 조건부(DB 메뉴 행 수동 정리)**. 유일 프로덕션 지속물: main.go·router.go dispatch/그룹·seed.go 시드·빈 V2 행.

## 12. 파일 소유권 — 구현 스폰 분리

| 스폰 | 전용 소유 경로 | 접근 금지 |
|---|---|---|
| impl-P20 | `internal/infra/adapter/kubernetes/**` — **단, `mapping_test.go`는 제외(P22 단일 소유 — r2)** — ·`contracttest/**`·`metrics/**`·`contract/{adapter,resource_kind}.go`·`fake/{fake,fake_test}.go` | inventory·migrate·main.go·api/v2·web |
| impl-P21 | `internal/infra/inventory/{sync,reconcile,backfill}*.go`·`migrate/step0004*`·`migrations.go` 0004 라인·`model/provider.go`·`model_test.go`·`compose/**`·`main_sync.go` + main.go dispatch | adapter/kubernetes 본체·compare·projection·api/v2·web |
| impl-P22 | `inventory/{compare,compare_test,projection}.go`·`adapter/kubernetes/mapping_test.go`(단일 소유 — r2)·`main_compare.go` + main.go dispatch | sync/backfill 본체·api/v2·web·mapping.md 본체(P20 소유 — mapping_test는 읽기·검증만) |
| impl-P23 | `api/v2/**`·`router/router.go` v2 그룹·`route-inventory.txt`·`store/seed.go` 메뉴 블록·`web/src/{api/infra.js,views/infra/**,router/index.js,utils/infra-i18n.js}` | internal/infra 하위 전부(**projection.go 소비만 — 수정 금지**)·main.go |

교차 소유: `main.go`(P21·P22 dispatch 누적 — 직렬 편집)·`mapping.md`(P20 작성, P22의 mapping_test가 검증). 직렬 순서(§4)로 동시 충돌 없음. (r2) `mapping_test.go`는 소유 충돌 소지를 제거하기 위해 P22 단일 소유로 확정 — 파일이 P20 디렉터리에 있으나 P20 스폰은 작성하지 않는다. `projection.go`는 PR 22가 소유·PR 23이 소비(§3.8).

## 13. 산출 외 확인사항 (구현자 불요, 인계 — r2 이월 대장 전면 증보)

**N-1~N-4 식별자 정의 (r2 신설 — r1 헤더가 참조만 하고 정의를 실지 않아 공중 식별자였음. 아래로 확정)**:

- **N-1 — 어댑터 계약의 자격증명 전달 확장**: Phase 2가 `DiscoverRequest.Connection`(§3.1 r2)으로 **해소분**을 착지했다. 잔존분: `OperationRequest.Connection`(실행 자격 — restart 등 변경 오퍼레이션이 provider 자격을 같은 경로로 받아야 함) — **Phase 3 인계**(계약 확장은 실행 소비자가 생기는 시점에).
- **N-2 — Material 목적 어휘 "operations" 바인딩**: Phase 2는 `Purpose="inventory"`만 소비. 오케스트레이션 mutation 자격의 목적 바인딩("operations" — §7.4 어휘) 생성·소비는 **Phase 3**(N-1과 동일 시점·같은 ADR 라인).
- **N-3 — 엔진 프로덕션 배선**: `main.go`에 Engine.Start·graceful shutdown 편입·첫 OperationDefinition(workload restart) 등록 — **Phase 3**(phase1 §13 승계).
- **N-4 — 크래시 주입 e2e**: 게이트의 프로세스 실크래시(SIGKILL) 버전 — **Phase 3 Slice A "crash-injected run"(§20 Phase 3)과 통합**(phase1 §13 승계).

**이월 대장 (재진입 시점 명시)**:

- **harness 성숙 (Phase 1 이연분 인수)**: PR 20이 첫 형상을 만들고 fake·k8s가 소비. aliyun/tencent 편입(Phase 4) 시 같은 하네스 확장 — Q1 CI 매트릭스는 그 시점.
- **/internal/metrics 엔드포인트·§18.2 전체 표**: Phase 3+ (J6 부채). 카운터 최소 형상은 구조적으로 레지스트리 확장 가능. **(r2) M1(마일스톤 1) 완료 선언 전 앵커**: M1 게이트의 sync 관련 메트릭 증명(`inventory_sync_*` 등)은 엔드포인트·전체 표가 전제되므로, M1 선언 절차에 §18.2 메트릭 앵커 구현 완료가 선행돼야 한다는 인계를 기록한다.
- **미구현 v2 GET 4종 (r2 등재 — §16.1 GET 8종 중 Phase 2가 연 4종)**: `GET /provider-connections/{uid}`·`GET /provider-contexts`·`GET /resources/{uid}/relationships`·`GET /resources/{uid}/operations` — Phase 3 운영 UI·실행 계약과 함께.
- **POST /provider-connections·/validate·/sync 라우트**: Phase 3 운영 워크플로(A8).
- **커서 재개(resume)·incremental 소비 계약 (r2 등재)**: 커서·rate-limit state의 지속은 Phase 2 이행(§3.3)·소비(재개·증분 선택)는 **Phase 4**(페이징 대용량 클라우드 어댑터 — 재개가 실제로 필요해지는 시점. 스펙 §9.2 위반분의 명시적 이연).
- **V2 workload 수집 확장(ReplicaSet·Job·CronJob) (r2 등재)**: §3.5 스코프 표의 "legacy 수집·V2 미수집" 종 — Phase 3(restart 대상 workload 종 확정)·M2(완전성) 소관.
- **Phase 3 CI-kind 도입 결정 (r2 등재)**: §23.2 "kind cluster in CI"의 Phase 2 로컬 우회(§0.8)는 편이며, CI 잡 편입 여부·형태는 Phase 3 착수 시 결정 인계(v2-ci.yml 무변경 보존 제약 #9는 그때까지 유효).
- **stale_source 행의 상세 가시 (r2 — 운영 가정)**: Phase 2는 stale 행 가시성을 sync 리포트의 `MarkedStale` 카운트까지만 제공한다. 행 단위 열람·운영 통보(무엇이 stale 됐는가)는 §5.4c의 부재 처리로 인해 V2 read side에 노출되지 않는다 — 이는 설계된 공백(비교·조회 오염 방지)이며 상세 가시 수단은 Phase 3+ 운영 워크플로와 함께 결정. 그 전까지는 리포트 카운트와 DB 직접 조회로 운영한다.
- **게이트웨이 모드 k8s 실측**: M2 운영·AccessRoute 전환 시(A4).
- **트랜스포트 중복 통합**: M2 decomposition step 2(R7 — §14.1 facade 경로와의 합류 지점, J1 r2).
- **k3d 주간 캐던스(K3s 실측)**: §23.2 "weekly, not per-PR" — Phase 2는 GitVersion 판정 유닛 테스트로 족하고 k3d 실증은 Phase 3 Slice A(kind)와 함께 재검.
- **Phase 0 인계 2·3 (phase1 §13 승계 재등재 r2)**: 인계 2 — arch-boundary R2 재설계 전까지 `internal/infra`·`internal/tasks`는 R1/R2 보호 밖(CORE_PACKAGES 확장은 R2 재설계 선행). 인계 3 — 신규 패키지 커버리지 ≥80% 래치·contract 순수성의 상시 CI 핀닝(본 계획은 로컬 claims 12로 대체). 완료 시점의 소유는 각각 재설계 PR·CI 편입 PR.
- **observation retention(§8.2 180일/30일 프루닝)**: 정기 잡은 관측 볼륨이 쌓이는 Phase 3+ 소관 — Phase 2는 latest 무조건 보존만 준수.
- 게이트 아티팩트(페어 리포트 3종·sync 리포트) tracking #4 첨부로 Phase 종결(§10).

## 14. 가정 명세 (불확실 요소의 명시적 처리)

- **A1 (페어 비교 클러스터)**: §15.1의 legacy 측은 "v1 API 핸들러가 호출하는 동일 service 메서드"이지 프로덕션 데이터가 아니다 — dev DB legacy 0행(§0.3)이므로 kind 시드 클러스터를 v1으로 등록한 행이 페어 대상이다. E-2 승인 전제.
- **A2 (달력 게이트)**: §15.4 "on different days"는 아티팩트 날짜 3상이로 증명하며 같은 날 3회는 게이트 위반(T54가 기계 봉쇄). Phase 종결 최소 3일.
- **A3 (K3s 증명 수준)**: distribution detection은 GitVersion `+k3s` 접미사 판정(T43)으로 증명 — k3d 실클러스터는 §23.2 주간 캐던스 소관으로 Phase 2 밖.
- **A4 (게이트웨이 모드)**: 어댑터는 게이트웨이 구성을 지원하되 Phase 2 게이트 증명은 kind 직접 연결로 수행. 게이트웨이 경로는 모의 유닛 테스트까음.
- **A5 (stale_source 배치)**: 컬럼은 provider_connection에만 — context(1:1)·resource는 조인 파생으로 부재 처리(§5.4c의 "V2 read side treats as absent"는 stale connection 조인 제외로 구현). 리소스行 자체는 유지(§5.4b "rather than silently keeping" 충족 — 마킹되어 조회 제외).
- **A6 (kubeconfig 백필 형식)**: K8sCluster.KubeConfig는 Phase -1 G-1 완료로 v2 envelope — SecretRef.Ciphertext에 문자열 그대로 복사·KeyID 파싱 기록. 재암호화 없음(동일 키세트·형식). 만약 legacy 잔존값 발견 시 백필은 해당 행을 Skipped+리포트(UNKNOWN halt 정신 — 절대 평문 복사 금지).
- **A7 (compare의 Service 수명)**: fresh `service.New(db)` = 캐시 우회 등가(§15.1)·스케줄러 부작용은 Shutdown 정리·단발 프로세스.
- **A8 (v2 라우트 서브셋)**: Phase 2는 §16.1 읽기 4종만. POST/validate/sync는 Phase 3. tenant 상수 헤더도 Phase 3 운영 라우트와 함께.
- **A9 (동기화 트리거 형상)**: 섀도 운영은 CLI(`sync-inventory`)가 유일 트리거 — 스케줄러(§8.2 ≥15분 간격)는 Phase 3+ 운영화 시점. §9.2 퍼블리케이션 규칙은 트리거 무관 동일.
- **A10 (시드 클러스터 규모)**: "최대 클러스터" 게이트 ③은 시드 클러스터(리소스 수백 수준)로 측정 — 단일 운영 클러스터 환경에서 시드가 곧 최대다. 리포트에 리소스 수·호출 수를 병기해 측정 맥락을 남긴다.
- **A11 (비교 tolerance 값 — r2)**: §15.3 "beyond tolerance"의 tolerance 값을 스펙이 정하지 않았다. 본 계획은 `CountTolerance = 0`(count 정확 일치 — §3.5 r2)으로 정의한다. 근거는 §3.5에 기록했고, 값은 compare.go 상수로 두어 추후 프로바이더별 오버라이드 여지를 남긴다. 스펙 개정 시 값 논의 정정 권고.
- **A12 (Raw 크기 상한 64KiB — r2 출처 정정)**: §8.2는 "Raw payloads have size limits"만 규정(수치 없음). 64KiB(`MaxRawBytes = 64 << 10`)는 계획 도입값이다 — r1이 §8.2에 수치가 있는 것처럼 인용한 것을 정정한다. 초과분 절단 + `normalized.truncated=true` 마커.
- **A13 (비교 스코프·시드 구성 — r2)**: 페어링 키 `namespace/name`·스코프 외 종 4종(ReplicaSet·Job·CronJob·StorageClass) 처분·시드 workload 5종 유지는 전부 §0.6/§3.5 실측에 근거한 계획 판정이다(J8 r2). 스펙 §15.2 표의 "identity by UID"와 legacy 직렬화(uid 부재)의 긴장을 페어링 키 분리로 푼다 — 스펙 개정 시 이 해석의 명문화 권고.
