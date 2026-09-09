# V2 Phase 6 계획 — Legacy cutover and God-object decomposition (r3)

작성: 2026-09-09 (r1) · r2 (전제 변경 2건) · **개정 r3 (2026-09-09 — 4렌즈 적대검토 반려 상환:
CRITICAL 10·HIGH 17·MEDIUM 22·LOW 8)** · 티어 **XL** · 앵커: `3a3a106` (main)

- 증거층: `v2-phase1/phase6-plan-evidence.md`(E1~E10) + `v2-phase1/phase6-coupling-census.md`(E12·E13) · 인계 문서:
  `v2-phase1/phase6-parity-handover.md` (**P**n) · 런북: `v2-phase1/phase6-cutover-runbook.md`
- 검토 원천: `docs/task-id/v2-phase6-cutover/gaps-r1.md` (부호 C-n·H-n·V-n·T-n·S-n·M-n으로 인용)
- 원천 스펙: `docs/architecture/multi-infrastructure-control-plane-v2.md` — §19.1·§19·§3.2/§3.3·
  §5.4·§15·§16.1·**§17.2·§17.3**·§18.2·**§20**·**§21**·§24

**r3에서 뒤집힌 판정 3건**: ① step 4a — "블록 1 즉시 착수" → 블로커 발견(C-2·T-2) 후
**E-1 (가) 확정으로 E0→E2→E1 3단 착지**(§J4). ② step 3 대상 — "읽기 21건" → **3건**
(`cluster/list`·`info`·`detail`); 나머지는 라이브 패스스루라 인벤토리 투영이 원리적으로 서빙
불가(§J3, M-3·H-1). ③ B~D2 회귀 탐지기 — C21 → **characterization test**; C21은 무변경 HEAD에서
이미 blocker라 픽스처 선행조건부로 **유지**한다(§J6, C-4).

---

## 0. 결론 먼저 (BLUF)

**블록 1은 step 1과 step 4a를 완료하고 step 2를 2a까지 진행하지만, 나머지 컷오버(step 3·4b·5)는
하나도 완료하지 않는다** — 그것들은 선행 과제 P 대기다. r2가 "4a 즉시 착수"라 한 것은 틀렸으나,
**E-1 (가) 확정(2026-09-09 사용자)으로 4a가 3단 착지(E0→E2→E1)로 되살아났다**.

| §19.1 step | r3 판정 | 블록 | 결정적 근거 |
|---|---|---|---|
| **1** 라우트 등록 분할 | **즉시 착수 가능** | 1 | 골든이 정렬 + 최종 핸들러명만 기록 (E1.1·E1.2) |
| **2a** client state 캡슐화 | **즉시 착수 가능** | 1 | 어댑터 자기주석이 step 2를 통합 시점으로 지목 (E4.2). 게이트웨이 dialer가 DB 요구 → 2b 이월 |
| **4a** restart 쓰기 중단 | **착수 가능 — E0·E2·E1 3단** | 1 | E-1 (가) 확정. `connectionUid` 필터 1개 + V2 4단 흐름 수용 (§J4) |
| **3** 읽기 → V2 투영 (**3건**) | 선행 P1 후 | 2 | §J3 |
| **4b** 잔여 쓰기 중단 | 선행 P2 후 | 2 | V2 오퍼레이션 10종 부재 + §16.1 POST 미구현 |
| **5** 컬럼/테이블 삭제 | 선행 P1·P2 + 승인 후 | 2 | `k8s_cluster` 결합 **31참조·7파일** (§J5) |

**신설 — Phase Z (characterization test, 순수 함수 120개).** 사용자 확정. B~D2가 옮기는
4,650줄에 현재 **검출기가 `go build`뿐**이고(V-1) 골든은 서비스 계층에 **구조적 무발화**
(V-2)다. §J0.

**r3 신설 원칙 — "선언된 표면이 아니라 실제 결합을 센다"(§2.0).** CRITICAL 4건(C-1~C-4)이
전부 같은 실패 모드였다: 계획이 경계를 **라우트와 화면으로만** 그었다. 모든 삭제·이동 대상에
**6축 전수**(라우트 / 서비스 내부 호출자 / 프론트 / DB·마이그레이션 / 테스트 단얫 / CI 잡)를
의무화한다.

**부수 효과**: `service/k8s.go` 5,062줄 → 13파일 전부 ≤800 (E5 — 렌즈가 12경계 전부 재검증).

---

## 1. 게이트 실현가능성 (§20 문언 대조)

§20 원문: *"per-family §15.4 pass; restore rehearsal completed; observation period elapsed
with zero legacy-path traffic before drops."* — 앞 둘은 **충족**(E9 #1·#2), 셋째는 **적용 불가**
(운영 배포본 부재로 관찰 대상 트래픽이 존재할 수 없다). **§7 D-11로 §20 문언 자체를 개정 대상에
넣었다**(H-7 상환 — r2는 §19.1 조항만 담아 Phase 6이 문언상 영구 미충족이었다).
§19.1 전제조건 4항 전부 녹색 (`phase6-entry-assessment.md` 편입 — E9).

---

## 2. 아키텍처 판단 (J판정)

### 2.0 경계 원칙 — 6축 결합 census 〔r3 신설·임계〕

**원칙: 선언된 표면이 아니라 실제 결합을 센다.** 삭제·이동 대상마다 아래 6축을 전수 조사하고
결과를 PR 설명에 표로 첨부한다. 축이 하나라도 미조사면 그 PR은 반려다.

6축: ① 라우트 골든 2종 · ② **서비스 내부 호출자**(`grep -rn "s\.X(\|svc\.X(" backend/ --include=*.go
| grep -v _test`) · ③ 프론트(`grep -rniln` — **대소문자 무시**, V-8: 프론트는 `restartK8sWorkload`) ·
④ DB·마이그레이션(테이블 grep + `util/secret_registry.go` 행) · ⑤ 테스트 단얫(하드코딩 수치) ·
⑥ CI(`.github/workflows/*.yml` **2파일 12잡**). **명령 원문은 런북 §4.**

**소급 적용 결과 (실측 2026-09-09)** — 원자료 E13:

- ② k8s 쓰기 메서드 **14종 중 내부 호출자는 2종뿐** — `RestartK8sWorkload`·`ScaleK8sWorkload`
  ← `service/integration_ai.go:1732,1734`. 나머지 12종 0건.
- ⑤ **하드코딩 라우트 수치 3곳** — `routes_inventory_test.go:130`(450) ·
  `authz_replay_test.go:234`(440) · `:353`(245). 뒤 둘은 `t.Fatalf`로 끝나는 **하드 단얫**이며
  r2는 이를 "주석·용량 힌트"로 오분류했다 (H-1·H-2·M-1).
- ③ 프론트 k8s API 소비 **9파일** (r2 배치표는 2 — H-6). 그중 restart 소비는 2파일.
- ⑥ 워크플로는 **2파일 12잡** (r2는 `v2-ci.yml` 10잡만) — `korean-localization-guard.yml`의
  `no-hardcoded-chinese`(:13)·`i18n-parity`(:80) 누락 (M-6·V-11).

### J0 — Phase Z: characterization test, **순수 함수 120개 한정** 〔r3 신설·임계〕

**왜 필요한가 (V-1·V-2 상환)**: `service/*_test.go` 테스트 함수 90개 중 k8s/kube/gateway 매칭
**0**. B~D2는 4,650줄을 옮기는데 판정 수단이 `go build`·`go vet`뿐이고, 골든은 (method, path,
최종 핸들러명)만 기록하므로 **서비스 내부 이동에 대해 diff가 원리적으로 0**이다. C21도 HEAD에서
이미 죽어 있다(C-4). 즉 **탐지기가 하나도 없다.**

**왜 120개인가**: `service/k8s.go` 함수 **161 = `*Service` 메서드 41 + 패키지 순수 함수 120**
(실측 `grep -c '^func '` / `grep -c '^func (s \*Service)'`).

| 군 | 수 | 판정 |
|---|---|---|
| `*Service` 메서드 | 41 | **제외** — 살아있는 클러스터·DB·SSH 하네스 필요. 하네스 비용 > 이관 비용이고 그 계층은 블록 2에서 소멸 |
| 패키지 순수 함수 | **120** | **대상** — 하네스 불필요, 파일 경계 이동에서 실제로 깨질 수 있다 |

**목적 2가지 (명시 요구)**: ① **B~D2 4,650줄 이동의 유일한 검출기**(V-1·V-2 상환 — C21을
대체하지 않고 보완, §J6). ② **P의 동치 판정 오라클로 승계** — 인계 문서가 이 함수들
(`buildOverviewDistribution`·`formatWorkloadResourceSummary`·`serviceExternalIP`·
`buildK8sDeleteResourcePaths` 등)을 V2 재구현의 이관 원천으로 지목하므로, 테스트는 블록 2에서
**버려지지 않고** V2 정규화기 동치를 판정하는 골든으로 재사용된다(인계 §5 규약).

**순수 함수의 E5 시맨별 분포 (실측)** — 테스트 파일이 소스를 미러링한다:

| 시맨 | 순수 함수 | Phase |
|---|---|---|
| `k8s_fetch` 24 · `k8s_build_pod` 21 | 45 | **Z1** |
| `k8s_build_net` 26 · `k8s_transport` 19 | 45 | **Z2** |
| `k8s_path` 25 · `k8s_metrics` 4 · `k8s_detail` 1 | 30 | **Z3** |
| types·types_mesh·mutate·workload·잔류 k8s | 0 | — |

Phase Z는 **분할 전에** 착지한다 — 테스트가 현재 `k8s.go`에 있는 함수를 같은 패키지에서
호출하므로 컴파일이 서고, 분할 후 파일명이 자연히 짝을 이룬다.

### J1 — step 1 분할 시맨 〔임계〕

선정 (a) — `router` 패키지 유지, `registerXxx(g *gin.RouterGroup, db *gorm.DB, ctl *controller.Controller)`.
계측기 4종이 전부 `package router`에 있어 패키지를 유지해야 무변경으로 회귀를 잡는다.

**r3 정정 2건**:

- **T-11**: `routes_v2.go`는 위 시그니처로 담기지 않는다 — v2 등록부는 `v2API *v2.InfraAPI`
  주입 + `router.go:537-539`의 nil 폴백(`v2.NewInfraAPI(db)`)을 추가 요구한다. 전용 시그니처
  `registerV2(engine *gin.Engine, db *gorm.DB, v2API *v2.InfraAPI)`를 별도로 둔다.
- **T-5**: r2의 "미들웨어 부착 지점 5곳이 순서 계약의 전부"는 **거짓**이다. 그룹 수준 5곳은
  실측 일치하나 **라우트별 `opdef.Middleware(...)` 285건**(실측 — `grep -h -c 'opdef.Middleware(' router/*.go` 합)이 별도로 있고 **Phase A가
  옮기는 게 바로 그것**이다.

**behavior-neutral 증명 계약 (정정판)** — 전제 A(운영 부재)가 **완화하지 않는다**:

| 무엇을 잠그나 | 수단 | 사각 |
|---|---|---|
| 라우트 집합 + 최종 핸들러명 | 골든 2종 diff 0 (C1·C2) | 미들웨어 전부 |
| **그룹 수준** 미들웨어 순서 | `backend/router/` **디렉터리 전체** grep diff 0 (C4 — **T-4 상환**: r2는 `router.go` 1파일만 봐서 신규 `routes_v1_*.go` 안의 `.Use()`를 무검출) | 라우트별 grant |
| **라우트별 `opdef.Middleware` 285건** | zero-grant replay(`authz_replay_test.go:225-250`) + `TestOperationTableCoversRouter` + 개수 대조 (C48) | — |
| 역할×라우트 allow 결과 | authz replay 전체 (C6) | — |

**내장 카나리**: 등록 함수가 최종 핸들러를 클로저로 감싸면 gin 기록명이 바뀌어 골든이 즉시
깨진다 — "래핑 금지"는 이미 기계 강제다.

### J2 — step 2 착지점: 2a(v1 내부 캡슐화), 2b는 이월 〔임계 · r2 유지 · 렌즈 재검증 완료〕

- **중복이다** — `adapter/kubernetes/client.go:1-12` *"faithful REIMPLEMENTATION … not an
  import … consolidation lands with M2 decomposition step 2"*.
- **2b 불가** — `newK8sHTTPClientForCluster`(`service/k8s.go:4341`)가 게이트웨이 모드에서
  `s.dialThroughGateway`를 주입 → DB 핸들 + `gatewaySSHClients` 요구 → arch rule 2 위반.
  개발 DB의 k8s 2개는 전부 `direct` — **미검증이지 고장은 아니다**(E4.4). → I1 이월.
- **arch-boundary 무영향** — `service/`·`adapter/kubernetes/` 둘 다 `CORE_PACKAGES` 밖
  (`check-arch-boundary.sh:34`). **이 실측이 M-1 삭감 근거이기도 하다**(§J7).
- **인용 정정 (T-10)** — `service.New` = `service.go:72`(r2 `:70`), `k8sOverviewCacheEntry` =
  `:58`, 캡슐 대상 5필드는 `:49-56`.

### J3 — step 3: 대상은 **3 엔드포인트** 〔r3 재판정·임계〕

**r2의 "읽기 21건"은 틀렸다.** ① **M-3 원리적 불가** — `pod/logs`·`events`·`containers`·
`metrics`·`workload/metrics`·`namespace/events`·`terminal/ws`·라이브 YAML `*/detail`은 인벤토리
투영이 서빙할 수 있는 데이터가 아니다(주기 스냅샷 vs 요청 시점 라이브 조회). ② **H-1 모집단
불일치** — `mapping.md`는 `K8sClusterDetail` **1 엔드포인트**만 덮고 나머지 20개는 매핑이 없다.

| 군 | 건수 | 처분 |
|---|---|---|
| `GET /k8s/cluster/list` · `/cluster/info` · `/cluster/detail` | **3** | **step 3 전환 대상.** `mapping.md`가 덮는 유일 모집단 |
| 라이브 패스스루 읽기 (logs·events·containers·metrics·namespace/events·라이브 YAML detail) | **17** | **전환 대상 아님 — v1 무기한 존치.** §3.2 REMAIN에 준하는 명시 처분. §7 **D-12**에 기록 |

**M-5 상환 — 모집단 확정**: `route-inventory.txt`의 k8s GET은 **22행**(:90, :145–165). 그중
`:90 /asset/service/k8s/catalog`는 자산-서비스 도메인 → k8s family 밖. **k8s 읽기 21 = 전환 3 + 라이브 패스스루 17 +
`pod/terminal/ws` 1.** 세 문서의 모집단 단위를 인계 문서에서 통일했다(V-16).

**하이브리드는 여전히 채택하지 않는다 — r1 판단 유지.** 사용자 결정(확정 B)과 같은 방향이다.
하이브리드는 "이중 조회 + 무성 UI 열화"이고, 무엇보다 **패리티 폐쇄를 영구히 미루는 경로**다 —
결손을 가려 버리면 채울 동기가 사라진다.

**플래그 (r2 유지)**: 롤백용 보존 폐기, **전환 검증용 임시 env** `V2_READ_SOURCE_K8S`. Phase G의
마지막 커밋이 플래그와 legacy 분기를 함께 제거한다(C36 — **2단 형태**).

### J4 — step 4a: **에스컬레이션 E-1** 〔r3 재판정·임계〕

렌즈가 블로커 3건을 찾았고 **성격이 서로 다르다** — 분리해 판정한다.

**(가) C-1 — 블로커가 아니다. 범위 재정의로 해소된다.**
`integration_ai.go:1732,1734`가 `s.RestartK8sWorkload`·`s.ScaleK8sWorkload`를 호출한다. 그런데
§19.1 step 4의 문언은 "legacy write **paths** stop"이고 path = **라우트 + 컨트롤러 + opdef def**다.
**서비스 메서드는 남긴다** — 삭제 후 유일한 도달 경로가 AI 툴 경로(§3.2 row 16 OUT-OF-SCOPE,
v1 존치)가 되므로 빌드도 서고 범위 밖 도메인도 침범하지 않는다. §2.0 ② census가 **14종 중
내부 호출자는 이 2종뿐**임을 전수로 보였다.

**(나) C-2 · T-2 — **해소됨. E-1 (가) 확정 (2026-09-09 사용자).**
프론트는 `(clusterId, ns, workloadType, name)`만 갖는데 V2 오퍼레이션 라우트는 **불투명
`infra_resource.uid`(32-hex)**로 조회한다(`internal/api/v2/operations.go:98,309`). `web/src`에
`urn:` 출현 0건이고 URN의 `{ContextID}` 성분을 노출하는 API도 없다. `ListResources` 필터는
`page`·`pageSize`·`kind`·`kindPrefix` 4개뿐(`internal/api/v2/infra.go:250-276`).

**확정: `ListResources`에 읽기 필터 파라미터 `connectionUid` 1개를 추가한다.**

**필터 키 선택 근거 (`connectionUid` vs `externalUrn` vs `name`+`namespace`)** — 실측 기반:

| 후보 | 판정 |
|---|---|
| `externalUrn` (완전 일치) | **불가.** URN은 `urn:k8s:{ContextID}:workload:{ns}/{종}/{name}`인데 프론트는 `{ContextID}`를 알 수 없다 — `/provider-contexts` 라우트가 미구현이고 `resourceView`(`infra.go:198-208`)에도 contextId가 없다 |
| `externalUrnSuffix` (꼬리 일치) | **불충분.** 프론트가 만들 수 있는 꼬리 `workload:{ns}/{종}/{name}`는 **클러스터 간 중복**이 가능하다(현재 k8s 커넥션 2개). 클러스터 스코프를 위해 두 번째 파라미터가 필요해져 "1개" 제약을 깬다 |
| `name`+`namespace` | **2개**이고 위와 같은 중복 문제가 남는다 |
| **`connectionUid`** ✅ | **채택** |

`connectionUid` 채택 이유 (전부 실측):

1. **1개로 완전 스코프** — 커넥션:클러스터가 1:1이라 중복이 사라진다.
2. **신규 API 0** — `providerConnectionView`(`infra.go:131-145`)가 이미 `sourceModel`·`sourceId`를
   노출하므로 프론트가 `GET /provider-connections`에서 `sourceModel=="k8s_cluster" &&
   sourceId==clusterId`로 uid를 얻는다.
3. **구현 한 줄** — `liveResources`(`infra.go:231-236`)가 이미 `provider_connection pconn`을
   JOIN하므로 `pconn.uid = ?` WHERE 하나면 된다.
4. **§16.1 정합** — 커넥션은 §16.1 일급 자원이고 UID로 지목된다. `{ContextID}`는 어떤 §16.1
   라우트도 노출하지 않는 내부 식별자라 새 표면을 여는 셈이 된다.

**프론트 해석 (왕복 2회)**: `GET /provider-connections` → uid → `GET /resources?
kind=orchestration.workload&connectionUid=<uid>&pageSize=100` → `externalUrn`이
`:workload:{ns}/{종}/{name}`로 끝나는 행의 `uid`. 꼬리 일치는 **한 클러스터 안에서 유일**하다.

**범위 자기한정 (팀리드 정정 반영)**: `internal/api/v2/`는 **보존 제약 #6("`internal/infra/`·
`internal/tasks/`") 범위 밖**이다 — r2가 이를 #6 위반으로 분류한 것은 과잉 적용이었다. 다만
본 계획이 `internal/api/v2/`에서 건드리는 것은 **`ListResources`의 읽기 필터 파라미터
`connectionUid` 1개뿐**이며 **응답 형상·기존 4개 필터·다른 핸들러는 전부 불변**이다. 이 한정을
**C51**이 잠근다.

**Phase 배치**: 필터는 프론트보다 먼저 있어야 하므로 **E0(백엔드 필터) → E2(프론트 전환) →
E1(레거시 경로 삭제)** 순이다.

**(다) T-3 — 블로커가 아니라 과소 기술이었다.** restart는 `RequiresApproval: true`
(`compose.go:287`)이므로 실제 흐름은 **plan → execute(`Idempotency-Key` 헤더 필수 —
`web/src/api/infra.js:26`) → approve → 폴링**이다. r2의 "2단계 UX 변화"는 틀렸다. 더구나
`K8s.vue:693`이 `Promise.all(targets.map(restartK8sWorkload…))` **일괄 재시작**이라 N건이
**N개 승인 대기 태스크**로 분해된다. **E2 확정 (2026-09-09 사용자): V2 4단 흐름을 그대로 간다.** 일괄 재시작이 **N개 승인 대기
태스크로 분해되는 것도 수용**한다 — V2 오퍼레이션 모델(정책 평가·승인 경유)의 설계 의도와
정합하므로 우회하지 않는다. E2 산출물의 UI 계약은 §3.5.

### J5 — step 5: `k8s_cluster` 결합 **31참조·7파일** 〔r3 확장〕

r2의 S1은 결합을 `backfill.go:69` **1곳**으로만 기술했다. 실측은 다르다 (E13.3):

| # | 결합 | 파괴 결과 |
|---|---|---|
| **S1a** | `markStaleSources`(`backfill.go:297-303`) — `source_id NOT IN (SELECT id FROM k8s_cluster)`로 stale 표시 | drop 후 `sync-inventory` 1회 → **전 행 `stale_source=true`** → §5.4c에 의해 **V2 읽기 전면 공백** (C-3) |
| **S1b** | `SourceKeyUID("k8s_cluster", id)`(`sync.go:478-481`)가 `provider_connection`·`provider_context`·`secret_ref` UID의 **파생 성분** | UID 의미가 죽은 테이블 참조. source_key 의미 재정의 필요 |
| **S1c** | `backfill.go` 소스 읽기 13지점 | 백필 전체 무효 → **등록이 CLI로 대체된 뒤(Phase H)에만 해소** |
| **S2** | `main_compare.go:37,76,83` legacy 캡처 | §19.1 "shadow-read discontinuation" 해당 |
| **S3** | `AssetService.K8sClusterID`(`model/asset.go:12`)·`OpsApplicationEnvironmentBinding.K8sClusterID`(`model/ops.go:383`) — 둘 다 §3.2 REMAIN | **DB FK 제약 0건**(실측: `referenced_table_name='k8s_cluster'` 0행). dev 행수 0. 코드 경로 정리만 |
| **S4** 〔r3 신설〕 | `util/secret_registry.go:65` — Phase -1 §4.1 secret 인벤토리 **14행 중 8행**(`K8sCluster.kube_config`, ClassPlaintext) | drop이 인벤토리 행을 무효화 → **G-1 게이트("14 inventory rows 전부 zero PLAINTEXT")의 모집단이 바뀐다.** 레지스트리 행 제거를 drop과 같은 PR에 |

거짓 양성 1건: `integration_ai.go:99,705,706,965`의 `k8s_cluster_overview`는 AI 툴 키 문자열이지
테이블 참조가 아니다.

**착수 조건**: P1·P2 완료 → Phase H 완료(S1c 해소) → S1a·S1b·S2·S3·S4 처리 → 파괴 전 백업
(C34) → **사용자 명시 승인**. 백업 restore는 유일 수단이 아니다(실데이터 0 — 마이그레이션
재실행으로 재생성 가능).

### J6 — C21 판정: **폐기하지 않는다 — 픽스처 선행조건을 명시한다** 〔명시 판정 요구 상환〕

**사실 (C-4)**: 무변경 HEAD에서 `sync-inventory` + `compare-inventory --cluster 1` 3회 전부
`verdict "blocker"` exit 1. 원인은 게이트 클러스터의 `seed-cron` CronJob(`*/10 * * * *`)이
완료 파드를 회전시키는데 V2가 Job/CronJob 미수집 → identity-sets-differ 상시.

**판정: 유지 + 픽스처 선행조건 명시.** "폐기 → characterization 이관"은 **거부**한다 —
§19.1 step 2가 "§15 comparisons re-run after each step"을 명시 요구하므로 폐기는 스펙 요구를
조용히 없애는 것이고 §7 이탈 행이 하나 더 필요해진다. 그럴 이유가 없다.

**근거 — 원인이 코드가 아니라 픽스처다 (실측 2026-09-09)**: `seed-cron`·`seed-job`은 커밋된
`v2-phase1/k8s-fixture/seed.yaml`에 **없다**(그 파일의 `kind:`는 Namespace·Deployment·
StatefulSet·DaemonSet 4종뿐). 라이브 kind 클러스터 `kind-v2-p2`의 `v2-seed` 네임스페이스에
임시로 적용된 객체다(`SUSPEND=False`, age 2d13h). **저장소 픽스처를 고칠 필요가 없고 kubectl
한 줄이면 된다.**

**C21 선행조건 (Phase Z 착수 시 1회, 런북 §2에 기재)**:
```bash
kubectl --context kind-v2-p2 -n v2-seed delete cronjob seed-cron
kubectl --context kind-v2-p2 -n v2-seed delete pod --field-selector status.phase=Succeeded
```
그 뒤 C21 베이스라인 1회로 `verdict: pass`를 확인하고, 안 나오면 **그 Phase에서 C21을
비활성화하고 사유를 PR에 적는다**(탐지기 부재를 조용히 두지 않는다).

**characterization test는 C21을 대체하지 않고 보완한다** — C21은 V2 인벤토리 동치를, Phase Z는
v1 순수 함수의 이동 무결성을 본다. 서로 다른 것을 잡는다.

**T-12 주의**: C21 실행 자체가 `backend/data/compare/`에 아티팩트를 쓴다 → bare `--gate`가
newest-3로 읽어 **현재 이미 `Passed: false`**. **waiver 게이트는 `"Passed": true` 유지**(3개를
이름으로 고정 — R6). 보존 제약 #5가 bare `--gate`를 금지하는 이유가 이것이다. `backend/data/`는
`.gitignore:25`라 저장소 오염은 아니다.

### J7 — 삭감 판정 (단순성 렌즈 — 위험 증가 0)

| # | 삭감 | 판정 |
|---|---|---|
| **S-1** | B+C+D1 = **1 PR**(Phase BCD), D2 별도 | **채택.** 스펙 §21:1687-1689가 `32. router split per module` / `33. service decomposition steps 2-5`로 **step 2~5를 1 PR**로 잡았다. D-3의 revert 단위는 **step이지 파일 묶음이 아니다**. B·C·D1은 순차 필수라 단독 revert가 애초에 불가능했다. 검증 배터리 4회 → 2회. **≤5파일 초과(BCD=10)는 팀리드 지시로 승인된 이탈**임을 명기 |
| **S-2** | P1 신규 kind 어휘 6종 → **2종** | **채택.** `service/k8s.go:630-633`의 `IstioGateways`·`IstioVirtualServices`·`IstioDestinationRules`·`IstioServiceEntries`는 **백엔드 전체 참조가 선언 4행뿐**. `advancedNetwork`는 `GatewayAPIGateways`+`HTTPRoutes` 2리스트만(`model/k8s.go:480-483`), 프론트도 `'gatewayapi'`·`'httproute'`로만 호출 |
| **M-1** | C32 카나리를 블록 1 **전체 1회** | **채택.** 계획 자신의 E4.5가 `service/`·`adapter/kubernetes/` 둘 다 `CORE_PACKAGES` 밖이라 실측했다 → **블록 1이 R1/R2/R3를 발동시킬 수 없다.** 7회×5~10분 → 1회 |
| **M-2** | claims 포함관계 정리 | **부분 채택** — §6.1 표로 명시해 리뷰어가 상위 1개만 돌리게. 하위 claim은 Phase 중 반복용 빠른 부분집합으로 남긴다 |

---

## 3. 변경 범위 (블록 1 배치표 — 6축 census 반영)

### 3.1 신규

| Phase | 파일 | 내용 |
|---|---|---|
| Z1 | `service/k8s_fetch_test.go`·`k8s_build_pod_test.go`·`k8s_chartest_helper.go` | 순수 함수 45개 |
| Z2 | `service/k8s_build_net_test.go`·`k8s_transport_test.go` | 45개 |
| Z3 | `service/k8s_path_test.go`·`k8s_metrics_test.go`·`k8s_detail_test.go` | 30개 |
| A | `router/routes_v1_infra.go`·`routes_v1_ops.go`·`routes_v1_system.go`·`routes_v2.go` | 등록 함수 4분할 (T-11) |
| BCD | `service/k8s_types.go`·`k8s_types_mesh.go`·`k8s_metrics.go`·`k8s_mutate.go`·`k8s_workload.go`·`k8s_detail.go`·`k8s_fetch.go`·`k8s_build_pod.go`·`k8s_build_net.go` | E5 #2~#10 |
| D2 | `service/k8s_transport.go`·`k8s_path.go`·`k8s_clientstate.go` | E5 #11~#13 |

### 3.2 수정

| 파일 | Phase | 변경 |
|---|---|---|
| `router/router.go` | A | 등록 블록 → 함수 호출. **:43,44,61,72,73,535,536 불변** |
| `service/k8s.go` | BCD·D2 | 5,062 → 약 412줄 잔류 |
| `service/service.go` | D2 | 5필드(`:49-56`) → 캡슐 포인터. `New`(**`:72`**) 초기화 |
| `service/gateway.go` | D2 | 캡슐 경유 접근 |
| `docs/architecture/…-v2.md` | F | §19.1·**§20** 조건화 개정 (§7) |
| `v2-phase1/RESUME.md` | F | 상태 갱신 |

### 3.3 Phase E0·E2·E1 (step 4a) 배치표 — E-1 (가) 확정 반영

**E0** (1파일): `internal/api/v2/infra.go` — `ListResources`에 `connectionUid` 필터 1개.
`liveResources`의 기존 JOIN에 `pconn.uid = ?` WHERE 한 줄 + 쿼리 파라미터 파싱.
**E2** (프론트) · **E1** (레거시 삭제): `router/routes_v1_infra.go` ·
`controller/*.go` · `opdef/*.go` · **테스트 단얫 3곳**(`routes_inventory_test.go:130` 450→449 ·
`authz_replay_test.go:234` 440→439 + 실패 메시지 `(427 v1 + 13 v2)`→`(426 v1 + 13 v2)` ·
`:353` 245→**244**) · 골든 2종 · `web/src/api/k8s.js` · `web/src/views/assets/K8s.vue` ·
`web/e2e/slice-a.spec.js`(:15,:45) · `web/e2e/slice-c.spec.js`(:116) · i18n 21사전
(`web/src/locales/` — 4단 흐름 문구가 21개 사전 전부에 들어가야 `i18n-parity`가 통과한다, C42).
**`service/k8s.go`의 `RestartK8sWorkload`는 남긴다**(J4 가).

### 3.5 E2 UI 계약 (사용자 확정 — V2 4단 흐름 그대로)

| 항목 | 계약 |
|---|---|
| 흐름 | `POST …/operations/k8s.workload.restart/plan` → `POST …/execute`(`Idempotency-Key` 헤더 필수 — `web/src/api/infra.js:26`) → `POST /tasks/:uid/approve` → `GET /tasks/:uid` 폴링 |
| `Idempotency-Key` 생성 | **건별**로 `<resourceUid>:restart:<UI 발화 timestamp(ms)>`. 같은 버튼 연타는 같은 키를 재사용해 중복 태스크를 만들지 않는다(§13.4 유니크 인덱스가 서버측 최종 방어) |
| 폴링 | 2초 간격, 종단 상태(`succeeded`/`failed`/`cancelled`)에서 정지. 5분 초과 시 폴링을 멈추고 "태스크 상세에서 확인" 링크로 전환 |
| 승인 대기 표시 | `awaiting_approval` 상태를 별도 배지로. 승인 권한이 없는 사용자에게는 승인 버튼 대신 대기 안내 |
| **N건 일괄** | `K8s.vue:693`의 `Promise.all` 벌크는 **N개 태스크로 분해**하고 **행별 진행 표시**(대기/승인대기/실행중/성공/실패)를 갖는다. 전체 진행률 + 부분 실패 목록을 함께 낸다 |
| 프론트 소비 정합 | 변경은 `api/k8s.js`·`views/assets/K8s.vue` **2파일**. 나머지 7파일(E13.4 census)은 restart 미호출 → 무변경. C28이 잠근다 |
| E2E | `slice-a.spec.js:15,45`가 **이미** restart의 plan/승인/이벤트 흐름을 단얫한다. E2는 그 흐름을 바꾸는 게 아니라 **v1 버튼을 그 흐름에 연결**하는 것이라 스펙은 안 깨진다 — 진입 지점 추가에 따른 셀렉터 보강 필요 여부를 E2가 판정하고 필요하면 같은 PR에서 수정(C50) |

### 3.4 절대 건드리지 않는 것

`internal/infra/**`·`internal/tasks/**`(보존 제약 #6) · `internal/infra/metrics/**` · 모든 DB
스키마 · `projection.go`. `internal/api/v2/`는 **#6 범위 밖**이나 본 계획이 스스로 한정한다 —
Phase E0의 `ListResources` **`connectionUid` 필터 1개**가 전부이고 응답 형상·기존 4개 필터·다른
핸들러는 불변이다 (C51).

---

## 4. Phase 분해

```
Z1 ─▶ Z2 ─▶ Z3 ─▶ A ─▶ BCD ─▶ D2 ─▶ F          ← 블록 1 (지금 착수 · step 완료 0)
                              │
                              ├─ E0 ─▶ E2 ─▶ E1 (step 4a — E-1 (가) 확정)
                              │
[선행 과제 P] P1(1 DTO·필드 ≥43·수집기 5종) · P2(오퍼레이션 10종)
                              └──▶ G(step3·3건) ─▶ H(step4b) ─▶ I(step5)   ← 블록 2
```

| Phase | step | 파일 | 성격 | 독립 검증 |
|---|---|---|---|---|
| **Z1·Z2·Z3** | — | 3·2·3 | 테스트 신설 | C44·C45·C46 |
| **A** 〔임계〕 | 1 | 5 | behavior-neutral | C1·C2·C4·C6·C48 |
| **BCD** | 2a | **10** ⚠ | behavior-neutral | **C47(Z 전량)** + C13·C15·C16·C21 |
| **D2** 〔임계〕 | 2a | 5 | behavior-neutral | 동일 + C13 copylocks + C14 |
| **F** | — | 3 | 문서·스펙 개정 | C33 |
| **E0** | 4a 선행 | 1 | 읽기 필터 추가 (behavior-additive) | **C51** · C7 · C19 |
| **E2** | 4a | 2 | behavior-changing | C28① → C12 · C42 · C43 · C50 |
| **E1** | 4a | 8 | behavior-changing | C1'·C2'·C27·C28②·C49·C8 |

⚠ **BCD 10파일은 ≤5 규칙 초과** — 팀리드가 S-1 삭감으로 명시 승인한 이탈이며 스펙 §21이 step
2~5를 1 PR로 잡은 것과 정합한다. 원 과제 브리프 ≤5 규칙에 대한 **의도된 편차**임을 기록한다.
E1 8파일도 같은 성격의 편차다 — 골든 2 + 테스트 단얫 2 + 라우터·컨트롤러·opdef·i18n으로,
**하나라도 빠지면 빌드나 테스트가 깨지는 원자 단위**다(§3.3).

**순서는 E0 ▶ E2 ▶ E1** — 필터가 있어야 프론트가 uid를 해석하고(E0), 프론트가 V2를 쓰고 있어야
백엔드 삭제에 기능 공백이 없다(E2). revert는 역순.

**임계경로**: A(모든 요청 경로) · D2(뮤텍스·singleflight 값 복사 시 상호배제 붕괴 — 포인터 강제,
C13) · E2(승인 체인 N건 분해 UX — §3.5 계약).

### 4.1 블록 2 — 진입 조건·형상 (상세 분해는 P 산출 후 — I2)

| Phase | 진입 조건 | 형상 | claims |
|---|---|---|---|
| **G** | P1 완료 (인계 §3 게이트) | 읽기 **3건** 소스 전환 + 임시 플래그 제거 | C36 |
| **H** | P2 완료 + G 완료 | `register-k8s` CLI 신설(규약은 인계 §4) → 쓰기 13건 삭제 | **C37~C40**(V-3 상환 — r2는 0개) |
| **I** | H 완료 + S1a·S1b·S2·S3·S4 처리 + 백업 + 승인 | `k8s_cluster` drop | C34·C35·C41 |

---

## 5. 파일 크기 (800 하드캡)

| 시점 | `service/k8s.go` | 신규 최대 |
|---|---|---|
| 현재 | 5,062 | — |
| Z 후 | 5,062 (소스 무변경) | 테스트 파일 ≤800 (C16) |
| BCD 후 | ~1,272 (추정) | `k8s_fetch.go` ~658 |
| **D2 후** | **~412 (추정)** | 동일 |

E5 근거 — **렌즈가 12경계 전부 재검증**(심볼 시작행·합계 5,061·최대 658). `k8s_fetch.go` ~658 >
650 경고선 → BCD 완료 조건에 `k8s_fetch.go`/`k8s_overview.go` **조건부 재분할** 포함. 범위 밖:
`monitor.go` 4,798·`service.go` 3,049 등 14건 — **증가 금지만** 보장(C17·C18).

---

## 6. 검증 요구 (claims — r3 전면 재작성)

**vacuity 처분 (검증성 렌즈 census 정면 수용)**: HEAD에서 이미 통과하던 **C26·C28**은
"대상 존재 증명 → 0" **2단 형태**로 재작성 · 대상 부재로 통과하던 **C15·C16**은 **파일 열거**로
대체(T-9 glob 오염: `k8s_*.go`가 `k8s.go`를 놓치고 기존 `k8s_terminal.go`를 셈) · B~D2에서
구조적 무발화이던 **C1·C2**는 그 구간 적용 범위에서 제거하고 검출기를 **C47**로 교체 ·
충족 불가이던 **C27**은 범위 한정 + 대소문자 무시로 해소(V-6·V-8, 삭제 불요) ·
claim이 0개이던 **Phase H에 C37~C40**, **Phase G에 C36 2단**, **Phase E0에 C51** 신설.

전제: `cd /mnt/d/DEV/acc0mplish/ops-admin`. **`HEAD~<n>` 전폐** — Phase 시작 시
`git tag p6-<phase>-base`를 찍고 태그를 쓴다(V-14).

```
--- 블록 1 공통 ---
C3  [Z·A·BCD·D2] `cd backend && go test ./router/ -run 'TestRouteInventoryArtifact|TestSensitiveRoutesArtifact|TestOperationTableCoversRouter' -count=1` → ok
C6  [Z·A·BCD·D2] `cd backend && go test ./router/ -count=1` → ok
C7  [각 Phase 종료] `cd backend && go test ./... -race -count=1` → ok
C8  [E] `cd backend && go test ./opdef/ -cover -count=1 | grep -o 'coverage: [0-9.]*%'` → 60.0 이상 (베이스라인 61.5% — restart def 제거가 마진 1.5pp를 좁힌다, T-8)
C10 [블록1 1회] `bash scripts/check-arch-boundary.sh` → 마지막 줄 `PASS: arch-boundary clean (R1+R2+R3).`
C11 [각 Phase 종료] `python3 scripts/secret-scan.py` → exit 0 (`PASS: secret-scan clean (773 files, 7 rules).`)
C12 [A·E·G·H] `cd web && bun install --frozen-lockfile && bun run build` → exit 0
C42 [Z·A·BCD·D2·E·F] i18n 사전 키 패리티 (`korean-localization-guard.yml:80` `i18n-parity` 잡의 run 원문): `node scripts/check-i18n-parity.mjs` → exit 0 (**2026-09-09 실행 확인**. V-11·M-6 상환. **E2가 `K8s.vue`에 plan→execute 문구를 넣으면 직격**)
C43 [Z·A·BCD·D2·E·F] 한자 혼입 금지 (`korean-localization-guard.yml:13` `no-hardcoded-chinese` 잡의 인라인 python heredoc을 그대로 실행 — `web/src`·`backend`·`docs` + README 2종을 `[\u3400-\u4DBF\u4E00-\u9FFF]`로 스캔해 `localization-findings.txt` 생성): 실행 후 `test ! -s localization-findings.txt && echo PASS` → `PASS`

--- Phase Z (characterization) ---
C44 [Z] 모집단 고정: `cd backend && echo $(( $(grep -c '^func ' service/k8s.go) - $(grep -c '^func (s \*Service)' service/k8s.go) ))` → `120`
C45 [Z 종료] 커버 수: `cd backend && go test ./service/ -run TestChar -count=1 -v 2>&1 | grep -c '^=== RUN   TestChar'` → 120 이상
C46 [Z 종료] 미커버 0: `cd backend && go test ./service/ -run TestChar -coverprofile=/tmp/z.out -count=1 >/dev/null && go tool cover -func=/tmp/z.out | grep '/service/k8s.go:' | awk '$3=="0.0%"{print $2}' | grep -vFf service/testdata/char-exclude.txt | wc -l` → `0` (`service/testdata/char-exclude.txt` = `*Service` 메서드 41개 이름 — **Z1에서 생성해 커밋**)
C47 [BCD·D2] **이동 무결성 — B~D2의 유일한 검출기** (L-1 상환 — 기계 판정): Z3 종료 시 베이스라인을 커밋해 둔다 —
      `cd backend && go test ./service/ -run TestChar -count=1 -v 2>&1 | grep -E '^(--- )?(PASS|FAIL): TestChar' | sed 's/ ([0-9.]*s)$//' | sort > service/testdata/char-baseline.txt`
      BCD·D2 종료 시: `cd backend && go test ./service/ -run TestChar -count=1 -v 2>&1 | grep -E '^(--- )?(PASS|FAIL): TestChar' | sed 's/ ([0-9.]*s)$//' | sort > /tmp/char-now.txt && diff service/testdata/char-baseline.txt /tmp/char-now.txt && echo CHAR_IDENTICAL` → `CHAR_IDENTICAL` (diff 0행)

--- Phase A ---
C4  [A] 그룹 미들웨어 불변 (**T-4 상환 — 디렉터리 전체**): `git diff p6-A-base -- backend/router/ | grep -E '^[-+].*(UseRawPath|engine\.Use\(|\.Group\(|authGroup\.Use\(|v2Group\.Use\()'` → 출력 0행
C1  [A] `git diff --stat p6-A-base -- docs/security/route-inventory.txt` → 출력 없음
C2  [A] `git diff --stat p6-A-base -- docs/security/sensitive-routes.txt` → 출력 없음
C48 [A] 라우트별 grant 285건 보존 (T-5): `cd backend && go test ./router/ -run 'TestZeroGrant|TestOperationTableCoversRouter' -count=1` → ok. 그리고 `grep -h -c 'opdef.Middleware(' router/*.go | awk '{s+=$1} END{print s}'` → p6-A-base 시점 값과 동일

--- Phase BCD·D2 ---
C13 [D2] `cd backend && go vet ./service/...` → 출력 0행
C14 [D2] `wc -l backend/service/k8s.go` → 800 이하
C15 [BCD·D2] 하드캡 (**T-9 상환 — 열거**): `cd backend && wc -l service/k8s.go service/k8s_types.go service/k8s_types_mesh.go service/k8s_metrics.go service/k8s_mutate.go service/k8s_workload.go service/k8s_detail.go service/k8s_fetch.go service/k8s_build_pod.go service/k8s_build_net.go service/k8s_transport.go service/k8s_path.go service/k8s_clientstate.go | grep -v total | awk '$1>800'` → 출력 0행
C16 [BCD·D2] 650 경고선: 위 13파일 + `router/routes_v1_infra.go routes_v1_ops.go routes_v1_system.go routes_v2.go` + Z 테스트 8파일에 `awk '$1>650'` → 출력 0행 (초과 시 조건부 재분할 후 재실행)
C17 [블록1 종료] `wc -l backend/service/monitor.go` → 4798 이하
C18 [블록1 종료] `wc -l backend/service/service.go` → 3049 이하
C19 [블록1 전 Phase] `git diff --stat p6-<phase>-base -- backend/internal/infra backend/internal/tasks` → 출력 없음 (보존 제약 #6 대상 — `internal/api/v2/`는 포함되지 않는다. 그쪽은 C51)
C51 [E0·이후 전 Phase] **`internal/api/v2/` 자기한정**: `git diff --stat p6-E0-base -- backend/internal/api/v2/` → `infra.go` **1파일만**. 그리고 `git diff p6-E0-base -- backend/internal/api/v2/infra.go | grep -E '^[-+]' | grep -vE '^(\+\+\+|---)' | grep -vcE 'connectionUid|pconn\.uid|^\+\s*//'` → `0` (필터 외 변경 0). 그리고 `grep -c 'json:"' backend/internal/api/v2/infra.go` → p6-E0-base 시점 값과 동일(응답 형상 불변)
C20 [블록1 전 Phase] `git diff --stat p6-<phase>-base -- backend/store backend/model backend/internal/infra/migrate` → 출력 없음
C21 [BCD·D2] §15 재실행 — **선행조건**: `kubectl --context kind-v2-p2 -n v2-seed get cronjob seed-cron` 이 `NotFound`(J6). 그 뒤 `cd backend && go run . sync-inventory --connection b23f3f6ab673c31ead416fe4948a9124 && go run . compare-inventory --cluster 1; python3 -c "import json,glob,os; f=max(glob.glob('data/compare/1/*/*.json'),key=os.path.getmtime); print(json.load(open(f))['verdict'])"` → `pass` (T-7 상환: 최상위 verdict를 JSON 파서로 고정 — grep 아님)
C22 [블록1 1회] `cd backend && go run . compare-inventory --cluster 1 --gate --gate-waiver ../v2-phase1/compare-gate-waiver-2026-09-08.json` → JSON에 `"Passed": true`

--- 카나리 (블록 1 전체 1회 — M-1 삭감) ---
C24 [블록1 1회] 라우트 카나리: `S=$(mktemp -d) && cp -r . $S/ && cd $S/backend && sed -i 's|authGroup.GET("/profile", ctl.Profile)|authGroup.GET("/profile", ctl.Profile)\n\t\tauthGroup.POST("/canary/unlisted", func(c *gin.Context) {})|' router/*.go && { grep -q '/canary/unlisted' router/*.go || { echo PLANT_FAILED; exit 1; }; }; ! go test ./router/ -run TestOperationTableCoversRouter -count=1 && echo CANARY_OK` → `CANARY_OK` (L-1 상환 — 앵커 표류 가드 포함)
C31 [블록1 1회] secret-scan 카나리: `D=$(mktemp -d) && printf 'aws_secret_access_key = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"\naws_access_key_id = "AKIAIOSFODNN7EXAMPLE"\n' > $D/canary-leak.txt; python3 scripts/secret-scan.py $D > $D/out.txt 2>&1; grep -q 'canary-leak.txt' $D/out.txt && echo CANARY_OK` → `CANARY_OK` (2026-09-09 확인)
C32 [블록1 1회] arch-boundary 카나리 **5종 전수** (V-10 상환 — r2는 1종만): `v2-ci.yml:156-273`의 5개 심기(R1+R2 / 클라우드 product / 클라우드 **test 음성** / inventory case / proxmox product + **test 음성**)를 그대로 재현해 **양성 3종 발화 · 음성 2종 통과** 확인 (양성 1종은 2026-09-09 확인. WSL 전체 복사 5~10분 — 배경 실행)

--- Phase E0·E2·E1 (E-1 (가) 확정) ---
C1' [E] `grep -c . docs/security/route-inventory.txt` → 452 이고 `git diff p6-E-base -- docs/security/route-inventory.txt | grep -c '^-.*workload/restart'` → 1
C2' [E] `grep -c . docs/security/sensitive-routes.txt` → 292 이고 `grep -c 'k8s/workload/restart' docs/security/sensitive-routes.txt` → 0
C49 [E] **테스트 단얫 3곳 동반 수정** (H-1·H-2·M-1 상환): `cd backend && grep -n '!= 450\|contract is 450' router/routes_inventory_test.go` → 0행(449로 변경됨), `grep -n '!= 440\|427 v1' router/authz_replay_test.go` → 0행(439·426 v1로), `grep -n '!= 245\|245 baseline' router/authz_replay_test.go` → 0행(244로)
C27 [E] 레거시 restart **경로** 삭제 (V-6·V-8 상환 — 범위 한정 + 대소문자 무시. **서비스 메서드는 존치**): `grep -rni 'k8s/workload/restart' backend/router backend/controller backend/opdef web/src docs/security` → 출력 0행. 그리고 `grep -c 'func (s \*Service) RestartK8sWorkload' backend/service/k8s*.go` → 1 (J4 가 — 삭제하면 `integration_ai.go`가 깨진다)
C28 [E] V2 전환 **2단 단얫** (V-7 상환): ① 착수 전 `grep -rlni 'restartK8sWorkload' web/src | wc -l` → **2** (대상 존재 증명) ② 완료 후 같은 명령 → **0** 이고 `grep -c 'operations/' web/src/views/assets/K8s.vue` → 1 이상
C50 [E] slice-a/slice-c E2E 정합 (M-6): `cd web && grep -n 'k8s.workload.restart' e2e/slice-a.spec.js e2e/slice-c.spec.js` 3지점이 신규 UI 흐름과 일치하도록 갱신됐고 `bun run e2e` 통과 (kind 필요 — 로컬 미가용 시 사유를 PR에 기재)

--- Phase F ---
C33 [F] 스펙 조건화 개정 착지 (V-9 상환 — **스펙 파일 한글 0자이므로 영문 앵커**): `grep -n 'Applicability (M1 development stage)' docs/architecture/multi-infrastructure-control-plane-v2.md` → 1건이고 그 행번호가 1520~1640 구간(§19.1~§20). 그리고 `grep -c 'deviation recorded' docs/architecture/multi-infrastructure-control-plane-v2.md` → **8 이상** (D-2·D-4·D-5·D-6·D-7·D-10·D-11·D-12)

--- 블록 2 골격 ---
C36 [G] 2단 (V-15): ① 착수 전 `grep -rn 'V2_READ_SOURCE_K8S' backend/` → 0행 ② 완료 후에도 0행 **그리고** `cluster/list`·`info`·`detail` 3핸들러가 `inventory.ProjectResources` 경유 (`grep -c 'ProjectResources' backend/service/k8s*.go` → 1 이상)
C37 [H] 삭제 전 census (리뷰어 체크리스트 — 명령 아님. 보존 제약 #15가 강제): 대상 13라우트 각각에 §2.0 6축 표가 PR 설명에 첨부됐는지 리뷰어가 확인
C38 [H] 골든 대조: `grep -c . docs/security/route-inventory.txt` → **439**, `sensitive-routes.txt` → **279**
C39 [H] `register-k8s` 왕복: `cd backend && go run . register-k8s --name <n> --kubeconfig <path> && go run . sync-inventory --connection <uid> && go run . compare-inventory --cluster 1` → `verdict: pass`
C40 [H] 하드코딩 수치 3곳 재수정: `cd backend && grep -n '!= 449\|!= 439\|!= 244' router/routes_inventory_test.go router/authz_replay_test.go` → 0행 (H 후 값 436/426/231로 갱신됨)
C34 [I] 파괴 전 백업: `mysqldump --single-transaction -h127.0.0.1 -uroot -p123456 ops_admin > /tmp/p6-pre-drop.sql && test -s /tmp/p6-pre-drop.sql && echo BACKUP_OK` → `BACKUP_OK` (L-1 상환 — 명령 원문)
C35 [I] 인바운드 FK 0: `docker exec ops-admin-mysql-dev mysql -uroot -p123456 -N -e "select count(*) from information_schema.key_column_usage where table_schema='ops_admin' and referenced_table_name='k8s_cluster';"` → `0`
C41 [I] S4 상환: `grep -c 'k8s_cluster' backend/util/secret_registry.go` → 0 이고 `cd backend && go test ./util/ -count=1` → ok (§4.1 인벤토리 모집단 14→13 반영)
```

### 6.1 포함관계 (M-2 — 리뷰어는 상위 1개만)

`C3⊂C6⊂C7` · `C15⊂C16` · `C29⊂C19`(폐기) · `C23⊂C22`(폐기) · `C5·C9` 폐기(C7 포함).
→ 각 Phase 종료 시 **C7·C16·C19·C20·C22·C11**만 돌리면 6개가 자동 충족. r2 35개 → r3 **46개**
(C1'·C2'·C51 포함 · 신설 15 · 폐기 4) — 증가분은 전부 vacuity 상환과 Phase Z·E0·H 신설이다.

---

## 7. §19.1·§20·§17.2·§21 이탈 표 + 개정 판정

| # | 조항 | 적용 | 사유 | 처분 |
|---|---|---|---|---|
| D-1 | §19.1 preconditions 3항 | 적용 | 품질 조건 | — |
| D-2 | §19.1 authority during dual-run | 미적용 | dual-run 자체가 없다 | 조건화 개정 |
| D-3 | §19.1 각 step은 separately revertible PR | 적용 | — | — |
| D-4 | §19.1 step 3 플래그 한 사이클 보존 | 부분 미적용 | 전환 검증용으로 축소 | 조건화 개정 |
| D-5 | §19.1 step 4 dark 보존 | 미적용(강화) | 삭제 | 조건화 개정 |
| D-6 | §19.1 step 5 관찰기간 log-verified | 미적용 | 관찰 대상이 존재할 수 없다 | 조건화 개정 |
| D-7 | §19.1 observation period one release cycle | 미적용 | 동일 | 조건화 개정 |
| D-8 | §19.1 rollback 1-2 revert PR | 적용 | — | — |
| D-9 | §19.1 rollback 5 백업 restore | 적용(위상 하락) | 실데이터 0 → 유일 수단 아님 | 개정 불요 |
| D-10 | §19.1 shadow-read 중단 = 주 2회 연속 | 미적용 | 주간 케이던스는 운영 전제 | 조건화 개정 |
| **D-11** 〔r3·H-7〕 | **§20 Phase 6 게이트** "observation period elapsed with zero legacy-path traffic before drops" (spec:1630) | **미적용** | D-6과 같은 근거. **개정하지 않으면 Phase 6이 문언상 영구 미충족** | **조건화 개정** |
| **D-12** 〔r3·M-3〕 | §19.1 step 3 "legacy read paths … flip to V2 projections" | **부분 적용 3/21** | 라이브 패스스루 17 + ws 1은 인벤토리 투영이 원리적으로 서빙 불가(§J3) | **조건화 개정** — "flip은 인벤토리로 서빙 가능한 읽기에 한한다" |
| **D-13** 〔r3·S-1〕 | §21 PR 33 "service decomposition steps 2-5" 1 PR | 부분 미적용 | step 2a만 BCD+D2 2 PR로, step 3~5는 블록 2 | 이탈 기록 (§21은 "shape for reference" — 개정 불요) |
| **D-14** 〔r3·H-6〕 | §21 PR 34 "feat(web): legacy route flips + 301s" · §17.2 route migration 301 | 블록 2로 이연 | 프론트 9파일·i18n 21사전·§17.3 메뉴 시드가 함께 움직인다 | 이탈 기록 + §13 I7 |

**판정: 삭제가 아니라 조건화.** 전제(운영 배포본)는 미래에 복원되므로 조항을 지우면 되살릴
근거가 없어진다. §19.1에 영문 **"Applicability (M1 development stage)"** 문단을 신설하고
D-2·D-4~D-7·D-10~D-12 각각에 `deviation recorded: <일자·사유>` 한 줄을 단다. §20 게이트 문언에도
동일 참조. Phase F 수행, **C33이 영문 앵커로 검증**(V-9 상환).

**파급 (보존 제약 #6 이행)** — Phase I의 `k8s_cluster` drop이 먼저 착지할 `metrics-ext-plan.md`를
깨는지 실측: `grep 'k8s_cluster'` → **0건**(히트는 전부 `provider_connection` — V2 테이블, drop
대상 아님). **충돌 없음.**

---

## 8. 선행 과제 P — 요약 (전문: `phase6-parity-handover.md`)

| 항목 | r2 | r3 |
|---|---|---|
| 모집단 | 엔드포인트 21 / 필드 39 / 수집기 9 — **3문서 3단위**(V-16) | **`K8sClusterDetail` 1 DTO로 통일** — step 3 대상 3건이 전부 이 DTO를 소비 |
| 필드 · 도메인 판정 | 39 · 2건 | **43**(누락 3·계수 오류·소분류 정정 — H-2) · **3건**(`monitorDatasourceId/Name` 추가 — H-3) |
| 수집기 | 9종 | **5종** — Istio 4종은 v1 참조 0건이라 제외(S-2). GatewayAPI+HTTPRoute 2 + ReplicaSet·Job·CronJob 3 |
| 완료 판정 | **0건**(V-5) | **게이트 10개 신설** + **V-4 상환**(`mapping_test.go:153-159`가 부분문자열이라 텍스트 편집으로 녹색 → 필드별 실제 정규화 키 존재 단얫 요구) |
| P2 · Z 승계 | 목록만 | + **권한·risk·승인 칸**(M-8) · **Z 테스트를 V2 동치 오라클로 쓰는 규약**(§5) |
| 보안 선행조건 | — | **`certificates[]`는 §1.6 ④ 판정 전 착수 금지** (팀리드 지시) |

---

## 9. 관측 (r2 유지 — 상세 E12)

G1·G2(v1 라우트 메트릭 0종·GET 미기록)는 관찰 기간 소멸로 **더 이상 결함이 아니다**. 배포본이
생기면 다시 문제가 되고 §7 D-6·D-11이 복원 지점이다. G3·G4·G5는 여전히 유효하며
`metrics-ext-plan.md` 소관이다(C21·C22가 대체 수단).

---

## 10. 위험 등록부 (r3 신설분 — r2 R1~R16 유지)

| # | 위험 | 파급 | 가능성 | 완화 | 검출 |
|---|---|---|---|---|---|
| **R17** | Z 테스트가 **버그까지 굳힘** | 중 | 높음 | characterization의 목적이 그것 — "옳음"이 아니라 "불변"을 잠근다. 취지 주석 의무화(#16) | 리뷰 |
| **R18** | Z 테스트가 P 승계 오라클로 못 쓰이게 형태가 굳음 | 중 | 중 | 입출력을 테이블 주도로, legacy 함수 호출을 1행으로 격리 → P가 V2 구현으로 그 1행만 교체 | P 착수 시 |
| **R19** 〔재작성 — #6 프레이밍 폐기〕 | E0의 `connectionUid` 필터가 **범위 확대의 발판**이 됨 — 같은 파일에 다른 필터·핸들러 변경이 딸려 들어감 | 중 | 중 | #6 위반이 아니라(범위 밖) 계획의 자기한정 문제다. 1파라미터로 못 박고 응답 형상 불변을 별도 단얫 | **C51** |
| **R20** | BCD 10파일 1 PR이 리뷰 불가 규모 | 중 | 중 | 순수 이동이라 diff가 기계적. PR 설명에 E5 경계표 첨부 + **커밋을 시맨별로 분리**(리뷰는 커밋 단위) | 리뷰 |
| **R21** | `seed-cron` 삭제가 다른 게이트 증거를 훼손 | 중 | 낮음 | waiver는 아티팩트 3개를 **이름으로** 고정 → 클러스터 변경 무관(R6 구조). 삭제 전후 `--gate-waiver` 재실행 | C22 |
| **R22** | 하드코딩 수치 3곳 중 일부만 수정 | 중 | **높음** | C49가 3곳 전부 대조. 6축 census ⑤가 동일 조사를 의무화 | C49·C7 |
| **R23** | Z 테스트 8파일이 800캡 초과 | 낮음 | 중 | 시맨별 분산(최대 45함수/Phase) | C16 |

---

## 11. 롤백

Z는 테스트만 추가하므로 revert가 자명하다. **BCD는 단일 PR이라 원자 revert** — r2가 우려한
"B·C·D1 부분 revert 불가"가 병합으로 해소됐다(S-1 부수 이득). D2는 구조체 시그니처 변경이라
`service.go`·`gateway.go`와 원자 착지·원자 revert. **E는 머지 E0→E2→E1, revert 역순**(E1을
먼저 되돌려야 프론트가 참조할 v1 경로가 살아난다). E0의 필터는 additive라 단독 revert 무해.
I는 백업 restore + 마이그레이션 재실행.

---

## 12. 보존 제약 (③구현 프롬프트에 **verbatim** 복사)

1. **behavior-neutral 단계(A·BCD·D2)는 관측 가능한 동작을 바꾸지 않는다** — 응답 바디·상태
   코드·헤더·미들웨어 순서·**라우트별 grant**·감사 행 형상 전부. **운영 배포본 부재는 이 요구를
   완화하지 않는다.**
2. **골든 2종 무단 변경 금지.** A·BCD·D2는 **diff 0**. Phase E의 변경은 §3.3 선언분(restart
   경로 1건 → 450→449·290→289)뿐이며, **테스트 하드코딩 3곳**(`routes_inventory_test.go:130`·
   `authz_replay_test.go:234`·`:353`)을 **같은 PR에서 함께** 고친다.
3. **arch-boundary R2** 통과. 4. **secret-scan** 통과.
5. **§15.4 비교 게이트** — 기존 판정을 무효화하지 않는다. `compare-inventory --cluster 1`의 새
   pair `verdict: pass`가 수용 기준이고 **bare `--gate`는 재실행하지 않는다**(newest-3가 기록된
   PASS를 뒤집는다 — E7.1). waiver JSON과 그 3개 아티팩트를 삭제하지 않는다. **C21 실행 전
   `seed-cron` 선행조건을 확인한다**(§J6).
6. **V2 무변경** — `internal/infra/`·`internal/tasks/`. `internal/infra/metrics/`는
   `metrics-ext-plan.md` 소유. **패리티 폐쇄(P1·P2)는 본 계획이 착수하지 않는다.**
   허용 예외 1건: backend 루트 CLI(`main_register_k8s.go`)가 `internal/infra`를 읽어 쓰기만
   하는 것(선례 `main_register_pve.go`).
   **`internal/api/v2/`는 #6 범위가 아니다** — 다만 본 계획이 스스로 한정한다: Phase E0의
   `ListResources` **`connectionUid` 필터 파라미터 1개**가 전부이고 **응답 형상·기존 4개 필터·
   다른 핸들러는 불변**이다. 그 외 `internal/api/v2/` 변경은 이 계획의 범위가 아니다 (C51).
7. **`projection.go` 술어 불변** — `metrics-ext-plan.md`와 공유. diff 0.
8. **원격 CI 대기 금지** — §6 claims가 **워크플로 2파일 12잡** 전부의 로컬 대응이다
   (C3·C6·C7·C8·C10·C11·C12·C24·C31·C32·C42·C43·C50).
9. **롤백 계약** — A·BCD·D2·E·G·H는 PR revert(BCD·D2는 원자 착지), I는 백업 restore.
10. **DB 파괴 금지** — 블록 1은 스키마를 안 바꾼다. Phase I의 drop은 **별도 승인 사항**이고
    **dev DB라도 파괴 전 백업 필수**(C34).
11. **Phase E는 레거시 restart의 *경로*(라우트·컨트롤러·opdef)만 삭제하고 *서비스 메서드는
    남긴다*** — `integration_ai.go:1732`가 호출하며 그 도메인은 OUT-OF-SCOPE다(§J4 가).
    **E2가 E1보다 먼저 머지된다.**
12. **뮤텍스·singleflight 값 복사 금지** — 캡슐은 포인터로만(C13).
13. **전환용 임시 플래그는 Phase G 안에서 생겼다 사라진다**(C36).
14. **스펙 이탈은 조용히 두지 않는다** — Phase F가 §19.1·**§20**에 영문 "Applicability" 문단과
    D-2·D-4~D-7·D-10~D-12 이탈 기록을 착지시킨다(C33). **삭제하지 않고 조건화**한다.
15. **〔r3〕 삭제·이동 대상마다 §2.0의 6축 census를 수행하고 PR 설명에 표로 첨부한다.** 축이
    하나라도 미조사면 반려다. **"선언된 표면이 아니라 실제 결합을 센다."**
16. **〔r3〕 Phase Z 테스트는 "옳음"이 아니라 "불변"을 잠근다** — 현재 동작을 그대로 기록하고
    버그로 보이는 것도 고치지 않는다. 각 테스트에 그 취지를 주석으로 남긴다.

---

## 13. 이월 대장

| # | 항목 | 재진입 트리거 |
|---|---|---|
| I1 | 트랜스포트 중복 제거 (step 2 "into adapters") | 게이트웨이 홉 seam 증명 + `internal/infra/` 변경 허용 |
| I2 | 블록 2(G·H·I) 파일 단위 분해 | P1·P2 완료 |
| I3 | §16.1 `POST /provider-connections`·`/validate`·`/sync` 미구현 | Phase H 착수 시 (그전까지 `register-k8s` CLI 대체) |
| I4 | 읽기 경로 트래픽 계측 (G1·G2) | 실사용 배포본 등장 — §7 D-6·D-11이 복원 지점 |
| I5·I6 | `monitor.go` 4,798 등 하드캡 위반 14건 · §3.2 row 4·7·22 | OTEL C트랙 후속 · M2 범위 재판정 |
| **I7** 〔r3·H-6·D-14〕 | §21 PR 34 프론트 301 전환 · §17.2 route migration · §17.3 메뉴 시드 · i18n 21사전 | 블록 2 G 완료 후 |
| **I8** 〔r3·H-4〕 | 선행 Phase가 명시 인계한 2건 — `phase4-plan.md:167,586`(normalizer 관계 링크: 인계 P1의 `workloadName`/`workloadType`가 의존) · `phase1-plan.md:791,808`(v1 AutoMigrate → versioned) | I8-a는 P1과, I8-b는 Phase I와 |
| **I9** 〔r3·M-4〕 | 쓰기 census 누락 2건 — `POST /console-sessions`(`sensitive-routes.txt:127` mutating·risk=high) · `GET /k8s/pod/terminal/ws`(exec 채널이나 GET) | 블록 2 H 착수 시 처분 확정 |

---

## 14. 결정 기록 · 미해결

### 14.1 해소 (결정 기록)

패리티 선행 확정 / release cycle·dark 보존·stdout 로그 관련 3건 소멸 / **C21 폐기 아님 —
픽스처 선행조건(§J6)** / **step 3 대상 3건(§J3)** / **C-1은 범위 재정의로 해소(§J4 가)** /
**BCD 병합 승인(§J7 S-1)** / **E-1 = (가) 확정 — `ListResources`에 `connectionUid` 필터 1개
(2026-09-09 사용자)** / **E2 = V2 4단 흐름 그대로 수용, N건→N태스크 분해도 수용 (2026-09-09
사용자)** / **보존 제약 #6 범위 정정 — `internal/api/v2/`는 애초에 #6 대상이 아니다(팀리드)**.

### 14.2 미해결

1. **[Phase I 착수 시 재상신] `k8s_cluster` drop 실행 승인** — Phase I는 선행 과제 P 뒤라 아직
   멀다. 팀리드 판정으로 **미해결 유지**하되 지금 결정하지 않는다.
2. **[P 착수 시 판정] 도메인 귀속 3건** — `overview.requestRate`·`overview.alertCount`·
   `cluster.monitorDatasourceId/Name`. §3.2 row 19(Monitor = REMAIN)와의 정합.
3. **[P 착수 시 판정 — 선행조건화] `overview.certificates[]` 보안 경계** — v1이 kubeconfig를
   파싱한다(`service/k8s.go:2963`). `mapping.md` §6과의 충돌이 **보안 계약 위반 가능성**이므로
   인계 문서에 **"이 판정 전에는 P1에서 `certificates[]`를 착수하지 않는다"**를 선행조건으로
   명문화했다.
4. [미확인] 향후 배포본의 게이트웨이 모드 k8s 사용 여부 (I1 난도에만 영향 · 판정 불변).
5. [미확인] 브랜치 보호 required 잡 집합 — claims에 12잡 전부의 로컬 대응을 넣어 실무 영향 차단.
