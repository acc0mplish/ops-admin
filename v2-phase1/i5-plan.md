# I5·I6 하드캡 분할 계획 (plan r2 — 2026-09-12)

task: >800 하드캡 위반 파일 **25건** behavior-neutral 분할(전부 ≤650) · tier XL ·
원천: phase6-plan.md :585(I5·I6 이월)·i5-recon.md(리컨)·i5-adversary-r1.md(5렌즈 — **r2는 전 발견 반영판**)·실측 HEAD d0f5cd3
state: p6-state.json `i5` phase=plan → 본 계획 승인 후 implement

**r2 개정 이력** (발견 번호 ↔ 반영 위치 — 병합 문서 `i5-adversary-r1.md` 참조):

| 발견 | 반영 |
|---|---|
| 교차 ① 산술 위반(기술 C1·단순 H1) | §3 전수 재작성 — 산출 방식 명시·전 파일 ≤650 산술 성립(ops.go 3분할·database.go 6파일·ssl 3파일·service 11파일 등)·§2.1 분할안 열 갱신(73파일) |
| 교차 ② CS-1/2 ↔ §3.2 충돌(기술 H1/H2·위험 F3·검증 C2) | §3.2 헬퍼 처분표 좌표 단위 신설 — CS-1/2와 단일 원천화 |
| 교차 ③ pathspec vacuous(기술 M1·검증 C1) | §5 #6·CI-B4를 `backend/internal/`로 수정 |
| 교차 ④ Phase 라벨 오기(완전 M2·기술 H4·검증 H1) | §3.6→D·§3.8→H~J 정정 |
| 검증 H5·단순 M2 | CI-B7 기계 전수 승격(선언 블록 멀티셋 diff — §6.1 스크립트 명세)·인간 3중은 보조 강등 |
| 검증 H3 | CI-B10 변경파일 집합 동등 클레임 신설 |
| 검증 H2·완전 M4 | §4 독립검증열 재배선(A/B/G 오배선 해소·CO 심벌 사전 확정 §6.3·J 클레임 0 보강) |
| 기술 H3·위험 F1 | §3.8·§5 #11 웹 "행동 보존+재배선" 재분류·CW-K8s 정정·CI-W6 스모크 체크리스트 신설 |
| 기술 H4 | CS-2·CS-3 정정(AssetTerminalSession 소비자 부재·main_cloudcompare) |
| 완전 H1 | §8 테스트 >800 4건 등재·판정 8 정정 |
| 위험 F2 | §2.3 무테스트 영역 명시·CI-B7 전수 대조 명문 |
| 완전 M1 | §5 #14 kubeconfig 계약 추가 |
| 위험 F4 | §5 #2 blame 완화(원본 좌표 커밋 명기) |
| 단순 M1·M3·M4·검증 M1~M5 | monitor 15파일 병합 기본화·Phase H 2-PR 허용·카운트 "잔류 포함" 통일·수치 재실측(31패키지·66함수·20타입·2069 i18n·43파일)·실행 cwd 명기·waiver pathspec·태그 검사 |
| LOW 22건 | §9 처분표 |

---

## 0. 판정 요약

| # | 쟁점 | 판정 |
|---|---|---|
| 1 | 범위 — 계획 문구 "14건" vs 실측 26건 | **25건 확정**(backend 16 + web 9). "14건"은 블록 1 이전 시점 측정 — 이후 BCD가 k8s.go 5,062 해소(목록 감소)·I10 J계열 성장·리컨 재측정으로 26건이 진실. 26건 중 `compare.go` 1건 제외(판정 3) |
| 2 | I5(backend)/I6(web) 분리 vs 1계획 | **1계획 10 Phase**(A~J, 백엔드 A~G 선행·웹 H~J 후행). phase6 :585가 I5·I6 병기 — 과업 단위가 동일하고 검증 체계가 백엔드/웹 2종이므로 문서 분할은 중복. Phase 경계가 I5/I6 실체 경계 |
| 3 | compare.go 1,243(internal/infra/inventory) 포함 여부 | **제외 — §8 이탈 등재**. 보존 제약(phase6 §12 #6 "V2 무변경 — `backend/internal/infra/**`")의 명문 대상. compare 게이트·waiver·Z 오라클 체인의 기반 인프라라 분할 이득(−443줄) 대비 리스크가 크다. 착지 예외화는 별도 승인 사항 |
| 4 | service.go 분할 = God object 분해? | **아니오 — 패키지 내 파일 분할만**. 아키텍처 §2.5 Decision은 "God-object decomposition 자체를 M2 스텝"으로 예약. I5는 같은 `service` 패키지 안에서 시맨별 파일로 **바이트 보존 이동**(BCD·J1a 선례) — 구조체·메서드 수신자·패키지 경계 무변경 |
| 5 | OTEL 조건 승계 | **충돌 0 — 형상 보존으로 승계**. C트랙 기각(2026-09-08)의 근거 로직 `trimMonitorQueryHistories`(monitor.go:839 — query_history 전역 DELETE, MonitorInstantQuery :801가 호출)는 본체 바이트 무변경 이동(→ monitor_query.go). A트랙(Collector→LGTM 직결·ops-admin은 semconv 매핑만)은 backend 수집기 형상과 무관 |
| 6 | M2 재판정 조건 승계 | **바이트 보존으로 정합**. I-a 기록 "monitor 미이관·재등록 `--monitor-datasource-id`" — monitor datasource의 ConfigJSON 소스·DTO 존속이 전제. 분할은 스키마·ConfigJSON·직렬화를 건드리지 않는다. §3.2 row 4·7·22(아키텍처 disposition matrix — row 4 AssetHost SUPERSEDE M2·row 7 AssetGateway SUPERSEDE M2·row 22 CI/CD REMAIN+M2 typed binding)가 지정하는 M2 재판정 도메인(asset·gateway·CI/CD)의 파일은 **M2 전환의 착지점이 될 시맨 파일 경계**로 분할한다(§3 비고) |
| 7 | controller.go "미들웨어 체인"(리컨 :50) | **리컨 정정** — controller.go에 middleware 0매치(grep 실측). thin 핸들러 80+ 모음(setRefreshCookie 등 쿠키 헬퍼 포함). 미들웨어는 router/router.go 소관. 분할 위험은 "핸들러→서비스 호출 배선 누락"이며 라우트 무변경 grep·골든 diff 0으로 검증 |
| 8 | 리컨 수치 판정 | 원본 파일 26건 행수 전부 일치. **단 프로덕션 외 누락이 있었다(완전 H1)**: 테스트 >800 4건(engine_test 1,282·proxmox client_test 844·tencent adapter_test 838·kubernetes executor_test 831)은 §8 등재하고 본 과업 밖 유지 — 분할 대상은 프로덕션 파일 25건, 테스트 파일은 §2.3 매핑 참조 |

---

## 1. 목표 / 비목표

### 목표

1. 25개 원본 파일을 전부 ≤650줄로 분할(신규·잔류 전부)
2. behavior-neutral: 라우트·권한·opdef·API 형상·골든 상수·UX 렌더 무변경
3. 백엔드 이동은 **바이트 보존**(BCD C47 3중·J1a 선례) — 웹은 **행동 보존+재배선**(§3.8 — 기술 H3 재분류)
4. 각 Phase PR 직렬·각 PR 독립 검증(이전 PR 미병합 상태 빌드·테스트 통과)
5. M2 재판정 도메인(asset·CI/CD)·OTEL 조건과 정합하는 시맨 파일 경계

### 비목표

- God object 패키지 분해(§2.5 — M2 소관), 모듈 재배치, 인터페이스 도입
- 함수 시그니처·수신자·직렬화 형상·DB 스키마·설정 변경
- `backend/internal/infra/**`(compare.go 포함 — §8)·`backend/internal/tasks/**`·`backend/internal/api/v2/**` — 단 engine_test 1,282 등 **테스트 파일 4건도 무변경**(§8)
- 경고대(650~800) 3건 — model/ops.go 658·AppBuildTaskList.vue 683·OpsScriptLibrary.vue 698 (재실측). 본 과업 밖. 성장 금지 계약만 승계(§5 #9)
- 골든 재생성·char-baseline 재생성(§6 CI-B8)
- **배포·원격 환경 전제 없음** — 전 과업이 로컬 검증으로 종결(LOW 처분: 무배포 전제 명문화)

---

## 2. 현황 — 25파일 실측 (2026-09-12 HEAD d0f5cd3)

### 2.1 backend 16파일 (service 14 + controller 2, 총 26,635줄 → 분할 후 **73파일**)

| 행수 | 파일 | 시맨 실측(함수 군) | 분할 후 파일 수 |
|---|---|---|---|
| 4,801 | service/monitor.go | payload 타입 20종(26~257)·normalizer/매처(283~527)·스케줄러+datasource(529~770)·PromQL+query history(771~953)·Jaeger(954~1137)·ES/VictoriaLogs 쿼리·히스토그램·포맷(1138~1508·2153~2292)·스트림/필드(1509~1959)·바로가기(1960~2152)·Prom 템플릿(2294~2593)·템플릿 CRUD(2594~2817)·룰 CRUD+batch/실행(2818~2984·3261~3418)·사일런스+어그리게이션(2985~3260)·평가 엔진(3419~3735)·이벤트/알림(3736~4352)·오버뷰(4353~4606)·대시보드(4607~4801) | 15 |
| 3,061 | service/service.go | struct+New(29~79)·payload(80~329)·auth/세션(330~535)·admin/role/menu/dept/post(546~1110)·로그인/운영 로그(1112~1211)·AssetHost CRUD(1213~1302)·sync/import/cloud(1303~1605)·터미널(1606~1672)·batch/그룹편집(1673~1805)·HostGroup(1806~2039)·Credential+CloudAccount(2040~2289)·Overview(2290~2537)·헬퍼(2538~2770)·SSH(2772~2950)·클라우드 캡처(2951~3046) | 11 |
| 2,488 | service/ops_application.go | 앱 CRUD+바인딩+normalize(1~549)·빌드 태스크/릴리즈(550~927)·파이프라인 CRUD/스테이지(928~1415)·실행 엔진(1416~1973)·원격 실행/유틸(1974~2488) | 5 |
| 2,230 | service/database.go | MySQL import SQL(45~152)·normalize/DSN/인스펙트(153~296)·CRUD/워크벤치/스키마트리(297~724)·테이블 데이터/SQL 실행(725~1204)·전송 태스크(1205~1805)·SQL 파서(1806~2043)·행 편집/롤백/태스크 실행(2044~2230) | 6 |
| 1,783 | service/integration_ai.go | 헬퍼+스키마 12종(1~265)·모델/대화/지식(266~741)·프로바이더 호출·파싱(742~1033)·툴 실행기 쿼리계(1034~1562)·툴 변환계(1563~1783) | 5 |
| 1,338 | controller/controller.go | 인프라(auth/config/upload)(37~216)·시스템 관리+로그(217~784)·자산 8도메인+터미널WS(785~1291)·유틸(1292~1338) | 3 |
| 1,336 | service/integration_finops.go | 스케줄러+계정 CRUD+동기화(1~605)·대시보드/리소스(606~834)·추천/로그/유틸(835~1336) | 3 |
| 1,242 | service/database_multidb.go | 공용/PG 접속·인스펙트(1~483)·Postgres CRUD/롤백(484~925)·Mongo+Redis(926~1242) | 3 |
| 1,197 | service/database_backup.go | 플랜+레코드+복원 태스크(56~343·940~1197)·MySQL 덤프(344~420·664~923)·PG 덤프(421~653) | 3 |
| 1,169 | service/notify.go | normalize+템플릿/채널/룰 CRUD(1~551)·디스패처(552~785)·렌더/웹훅/사인+발송 로그(786~1169) | 3 |
| 1,112 | service/ops.go | normalize/변수(1~269)·스크립트 CRUD+실행 진입(270~632)·태스크 실행기·SSH(633~1112) | 3 |
| 1,023 | service/ops_schedule.go | CRUD/매핑(1~560)·템플릿+실행기(561~1023) | 2 |
| 1,016 | service/ops_job.go | 잡/템플릿 CRUD·승인(1~594)·노드 실행기(595~1016) | 2 |
| 1,007 | service/ssl_certificate.go | CRUD+삭제/다운로드/감사(1~78·549~764)·클라우드 동기 태스크(326~548)·파싱/도메인(765~1007) | 3 |
| 1,001 | service/domain.go | 퍼블릭 DNS(1~500)·내부 DNS+검증/감사(501~1001) | 2 |
| 831 | controller/monitor.go | overview/커맨드센터/datasource(1~129)·쿼리/로그/트레이스/바로가기(130~316)·룰 4도메인(317~686)·이벤트/대시보드(687~831) — 함수 **66개**(r1 67→66 정정) | 4 |

### 2.2 web 9파일 (총 14,297줄) — 합계 25파일 40,932줄

| 행수 | 파일 | 구조 실측 | 분할안 |
|---|---|---|---|
| 2,803 | views/assets/K8s.vue | script 1~2772 + template 30줄(2774~2803) — 자식 8종(`k8s/`)이 `:page` 객체 주입으로 상태 수신. 잔여 script는 page 객체 정의부 | composable 5~7(`web/src/composables/` — useK8sOperationProgress 선례) + K8s.vue 잔류 ≤650 |
| 2,600 | views/monitor/MonitorDashboard.vue | script 1~751 + template 753~2600 | 자식 4~5 + composable 1 |
| 2,456 | views/assets/DatabaseWorkbench.vue | script 1~1308 + template 1310~2456 | 자식 4~5(`database/` — DatabaseConnectionTree 선례) |
| 1,397 | layouts/MainLayout.vue | script 1~516 + template 518~1397 — 전 라우트 공유 | 자식 2~3(사이드바·헤더) |
| 1,076 | views/applications/AppPipelineCenter.vue | script 1~583 + template 585~1076 | 자식 2 |
| 1,070 | views/assets/Host.vue | script 1~525 + template 527~1070 | 자식 2 |
| 1,037 | views/monitor/MonitorAlertRule.vue | script 1~718 + template 720~1037 | 자식 2 |
| 1,009 | views/ops/OpsJobDesigner.vue | script 1~732 + template 734~1009 | 자식 2 |
| 849 | views/assets/AssetOverview.vue | script 1~198 + template 200~849 | 자식 2 |

### 2.3 검증 기반 인프라·테스트 매핑 (위험 F2 — 무테스트 영역 명시)

- 골든 2종: `docs/security/route-inventory.txt` **441행**·`sensitive-routes.txt` **281행**
- char: `service/testdata/char-baseline.txt` **114행**·`char-exclude.txt` **40행** — 모집단은 k8s.go 순수함수 고정. 본 과업 25파일은 char 모집단 아님(CI-B8 잠금)
- 테스트 패키지 **31개**(`go list ./...` 33 중 _test.go 보유 — r1 "30" 정정). CI-B2는 31패키지 ok
- **직접 테스트 보유 원본**: monitor.go(monitor 관련 test 있음 — 구현 시 `service/monitor*_test.go` 존재 확인)·k8s 계열 외 대부분 service 파일은 `service/*_test.go` 일부만. **무테스트(전수 기계 대조만으로 검증)**: controller/controller.go·controller/monitor.go·ops_application.go·ops_schedule.go·ops_job.go·database_multidb.go — 이 6종은 CI-B7(선언 블록 멀티셋)이 유일 검출기이므로 **기계 전수를 생략 불가**(§6.1)
- CI 워크플로 2종 — 원격 CI 대기 금지, 로컬 대응이 claims(실행 cwd: `/mnt/d/DEV/acc0mplish/ops-admin` — 백엔드 명령은 `backend/` 기준, 웹은 `web/` 기준으로 명시)
- e2e: `slice-a.spec.js`는 **V2 면만 커버**(infra/providers·infra/resources·containers/k8s — 실측). 본 과업 9뷰 중 MainLayout만 간접 커버, **8/9 미커버** → CI-W6 스모크 체크리스트로 보완(§6.2). slice-b/c는 serve-b/serve-c 전용·병렬 금지(메모리 web-e2e-stack-modes)
- i18n: `web/src/utils/*-i18n.js` 합 **2,069줄**(r1 2,971 정정 — glob 오계)·`scripts/check-i18n-parity.mjs`
- service 패키지 기존 비테스트 파일 **43개**(r1 42 정정)·controller 19개 — 신규 파일명 충돌 0(전수 대조)

---

## 3. 파일별 분할 설계

**산출 방식(전 파일 공통 — 교차 ①)**: `목표 행수 = Σ(이동 블록 줄수) + 파일 헤더(package+imports+빈줄 약 13줄)`. 블록 줄수는 함수 시작 라인 차(마지막 블록은 파일 끝까지)로 실측했다. 이하 표의 "Σ+H" 표기가 그 합계이며 **전부 ≤650 산술 성립**. 이동은 같은 패키지 내 신규 파일로 **바이트 보존**(가감은 import 블록·package/doc 줄만).

### 3.1 monitor.go 4,801 → 15파일(잔류 포함) (Phase B) — 단순 M1 병합 기본화

| 신규 파일 | 이동 범위(실측 라인) | Σ+H |
|---|---|---|
| monitor.go (잔류) | 헤더·doc — 본체 전부 이동 | ~25 |
| monitor_payload.go | payload 타입 20종 26~257 | ~245 |
| monitor_normalize.go | normalize/매처 283~527 | ~258 |
| monitor_datasource.go | 스케줄러+datasource CRUD/health 529~770 | ~255 |
| monitor_query.go | PromQL+history 771~953 **+ Jaeger 954~1137 병합** — trimMonitorQueryHistories(:839) 포함, 판정 5 | ~380 |
| monitor_logs_query.go | ES/VL 쿼리·히스토그램 1138~1508 + 포맷/auth 2153~2292 | ~525 |
| monitor_logs_fields.go | 스트림·필드·필드값 1509~1959 | ~464 |
| monitor_shortcut.go | 바로가기 CRUD+monitorLogShortcutDefault(:2085)+elasticsearchSearch 1960~2152 | ~206 |
| monitor_template.go | **Prom 템플릿(2294~2593·prometheusRule* 타입 포함) + 템플릿 CRUD 2594~2817 병합** | ~537 |
| monitor_rule.go | 룰 CRUD 2818~2984 + batch/상태/실행/프리뷰 3261~3418 | ~338 |
| monitor_silence.go | **사일런스 2985~3125 + 어그리게이션 3126~3260 병합** | ~289 |
| monitor_eval.go | 평가 엔진 3419~3735 | ~330 |
| monitor_event.go | 지문/통지/타임라인/업서트/회수+이벤트 API 3736~4352 | ~630 |
| monitor_overview.go | 오버뷰·커맨드센터 4353~4606 | ~267 |
| monitor_dashboard.go | 대시보드/패널 4607~4801 | ~208 |

인접 시맨 재병합·재분할 허용(650 초과 금지·심볼 집합 불변 전제). 시맨별 커밋 의무(BCD 선례). 주의: `service/monitor_*.go`와 `controller/monitor_*.go`(Phase A)가 **동명** — 서로 다른 패키지라 충돌 없으나 커밋 메시지에 패키지 한정자 필수(LOW 처분).

### 3.2 service.go 3,061 → 11파일(잔류 포함) (Phase C)

| 신규 파일 | 이동 범위(실측 라인) | Σ+H |
|---|---|---|
| service.go (잔류) | struct·New 29~79 + 범용 payload(IDPayload 140~147·BatchIDPayload 148~152) + 공용 헬퍼(아래 처분표 잔류분) | ~120 |
| service_auth.go | LoginRequest 80~84 + normalizeBrowser/OS 2575~2610 + auth/세션/Profile/SystemConfig 330~535 + 로그인/운영 로그 1112~1211 | ~360 |
| service_admin.go | payload(85~139·159~173) + admin/role/menu/dept/post+hidden 유틸 631~1110 | ~563 |
| service_asset_host.go | AssetHostPayload/ImportRow 187~230 + CRUD/그룹편집/batch 1213~1302·1673~1805 + payload 변환 2622~2689 | ~348 |
| service_asset_sync.go | SyncAssetHost·ImportAssetHosts·SyncAssetHostsFromCloud·BatchSync 1303~1605 | ~316 |
| service_asset_terminal.go | OpenAssetTerminal 1606~1672 + AssetTerminalSession 타입(:52) | ~90 |
| service_asset_group.go | group payload·Node 248~257·303~322 + HostGroup 1806~2039 + group 유틸 2691~2749 | ~336 |
| service_asset_cloud.go | credential/cloud payload 231~247·258~302 + Credential/CloudAccount CRUD 2040~2289 + 마스킹 2750~2771 | ~347 |
| service_overview.go | GetAssetOverview·paginateLogs 2290~2570 | ~294 |
| service_ssh.go | SSH 클라·수집·명령·formatConfig 2772~2925 | ~167 |
| service_cloud_capture.go | cloudInstance·CaptureCloudInstancesForCompare·fetchCloudInstances 2951~3046 | ~109 |

**공용 헬퍼·타입 처분표(좌표 단위 — 교차 ②, CS-1/2와 단일 원천)**:

| 심볼 | 원본 좌표 | 처분 |
|---|---|---|
| AssetTerminalSession | :52 | → service_asset_terminal.go(소비자는 service.go 내 OpenAssetTerminal뿐 — 실측 grep 유일. CS-2 정정: r1 "k8s_terminal 소비자"는 오기) |
| Trimmed | :2571 | 잔류 service.go(전 패키지 범용 — k8s 등 다수 소비) |
| shortenText | :2611 | 잔류 service.go |
| optionalUint | :2650 | 잔류 service.go |
| lastField | :2926 | 잔류 service.go |
| firstNonEmpty | :3047 | 잔류 service.go(cloud_capture·ssh 공용) |
| defaultSSHPort | :3056 | 잔류 service.go(ssh·asset 공용) |
| normalizeBrowser/OS | :2575~2610 | → service_auth.go(LoginLog 전용) |
| paginateLogs | :2538 | → service_overview.go |
| maskedSecret/maskCredential/normalizedAuthType | :2750~2771 | → service_asset_cloud.go |

**row 4 비고**: service_asset_host/sync/terminal/group/cloud의 시맨 경계가 M2 InfraResource SUPERSEDE 전환의 착지점.

### 3.3 ops_application.go 2,488 → 5파일(잔류 포함) (Phase D)

| 신규 파일 | 이동 범위 | Σ+H |
|---|---|---|
| ops_application.go (잔류) | 앱 payload·CRUD·바인딩·normalize 1~549 | ~550 |
| ops_app_build.go | 빌드 태스크·로그 라이터·릴리즈·레지스트리 550~927 | ~391 |
| ops_app_pipeline.go | 파이프라인 payload·CRUD·템플릿·스테이지 928~1415 | ~501 |
| ops_app_pipeline_exec.go | 실행 엔진 1416~1973 | ~571 |
| ops_app_pipeline_run.go | 원격 실행·스크립트·변수·유틸 1974~2488 | ~528 |

**row 22(CI/CD) 비고**: OpsApplicationEnvironmentBinding의 K8sClusterID→typed binding(M2) — 바인딩 CRUD는 잔류에 남겨 착지점 단일화. **kubeconfig 계약(§5 #14)**: `opsPipelineKubeconfigFile`(실측 정의 :1891 — r1 :1892·병합 문서 :1893에서 정정)은 pipeline_exec/run 어느 파일로 이동하든 계약(0600·cleanup·단일 세션) 주석을 그 파일에 동반.

### 3.4 database군 → 12파일(잔류 포함) (Phase E)

| 원본 | 신규 분할(Σ+H) |
|---|---|
| database.go 2,230 | 잔류(normalize/DSN/인스펙트 153~296 + MySQL import SQL 45~152 ≈ 404)·database_crud.go(297~724 ≈ 441)·database_data.go(725~1204 ≈ 493)·database_transfer.go(1205~1805 ≈ 614)·database_sql.go(1806~2043 ≈ 251)·database_rows.go(2044~2230 ≈ 200) |
| database_multidb.go 1,242 | 잔류(공용·PG 접속/인스펙트 ≈ 483)·database_postgres.go(484~925 ≈ 455)·database_nosql.go(926~1242 ≈ 330) |
| database_backup.go 1,197 | 잔류(플랜 56~343 + 레코드/복원 940~1197 ≈ 613)·database_backup_mysql.go(344~420·664~923 ≈ 350)·database_backup_pg.go(421~653 ≈ 246) |

r1 대비 정정: database.go 4→**6파일**(교차 ① — "CRUD/워크벤치 ~600"은 실측 297~1805=1,509의 미배정 ~900을 숨겼음). writeTableData·listSchemaObjects(664~753)는 mysql/pg 공용 — mysql 파일에 배정하되 pg 소비 시 import만(같은 패키지라 참조 무변경).

### 3.5 integration·notify·ssl·domain → 16파일(잔류 포함) (Phase F)

| 원본 | 신규 분할(Σ+H) |
|---|---|
| integration_ai.go 1,783 | 잔류(헬퍼+스키마 1~265 ≈ 265)·integration_ai_chat.go(266~741 ≈ 476)·integration_ai_provider.go(742~1033 ≈ 292)·integration_ai_tools_query.go(1034~1562 ≈ 529)·integration_ai_tools_util.go(1563~1783 ≈ 221) — tools 경계는 구현 시 재실측 |
| integration_finops.go 1,336 | 잔류(계정·동기화 ≈ 605)·integration_finops_report.go(606~834 ≈ 229)·integration_finops_advice.go(835~1336 ≈ 502) |
| notify.go 1,169 | 잔류(normalize+CRUD ≈ 551)·notify_dispatch.go(552~785 ≈ 234)·notify_render.go(786~1169 ≈ 384) |
| ssl_certificate.go 1,007 | 잔류(1~78 + 549~764 ≈ 294)·ssl_certificate_cloud.go(326~548 ≈ 236)·ssl_certificate_parse.go(765~1007 ≈ 256) — r1 2파일에서 **3파일로 정정**(교차 ①: 2분할 시 잔류 760) |
| domain.go 1,001 | 잔류(퍼블릭 DNS ≈ 512)·domain_internal.go(501~1001 ≈ 514) |

### 3.6 ops군 → 7파일(잔류 포함) (Phase D — 라벨 정정 교차 ④)

| 원본 | 신규 분할(Σ+H) |
|---|---|
| ops.go 1,112 | 잔류(normalize/변수 1~269 ≈ 269)·ops_script.go(CRUD+실행 진입 270~632 ≈ 363)·ops_exec.go(태스크 실행기·SSH 633~1112 ≈ 480) — **3분할로 정정**(교차 ①: r1 잔류 666 초과) |
| ops_schedule.go 1,023 | 잔류(CRUD/매핑 1~560 ≈ 560)·ops_schedule_exec.go(템플릿+실행기 561~1023 ≈ 463) |
| ops_job.go 1,016 | 잔류(CRUD·승인 1~594 ≈ 594)·ops_job_exec.go(노드 실행기 595~1016 ≈ 422) |

### 3.7 controller → 7파일(잔류 포함)

| 원본 | 신규 분할(Σ+H) | Phase |
|---|---|---|
| controller/monitor.go 831 | 잔류(overview·커맨드센터·datasource 1~129 ≈ 129)·monitor_query.go(130~316 ≈ 187)·monitor_rule.go(317~686 ≈ 370)·monitor_event.go(687~831 ≈ 145) | **A** |
| controller/controller.go 1,338 | 잔류(New·auth·config·업로드+유틸 37~216·1292~1338 ≈ 263)·controller_system.go(217~784 ≈ 568)·controller_asset.go(785~1291 ≈ 507) | **G** |

AssetTerminalWS(**:980**~1043 — r1 :938 정정)·ImportAssetHosts(861~938) 대형 핸들러도 바이트 보존 이동. `BuildMenuTree`(:1304)는 잔류 유틸 유지(LOW 처분).

### 3.8 web 9파일 (Phase H~J — 라벨 정정 교차 ④)

**계약 재분류(기술 H3)**: 웹 분할은 **행동 보존+재배선**이지 바이트 보존이 아니다. (a) composable 추출 시 자유변수(다른 반응형 참조·API 클라이언트)는 인자/반환으로 **주입 재배선**하고 (b) 템플릿 블록 이동 시 식별자는 `page.x` 형태로 **재배선**된다. 보존 대상: 렌더 출력·이벤트 처리·API 호출 순서·i18n 키 소비(행동). 바이트 동일성은 요구하지 않는다(§5 #11·CW-K8s 정정).

| 원본 | 분할안 |
|---|---|
| K8s.vue 2,803 | `web/src/composables/`에 K8s 전용 5~7: useK8sClusterState(클러스터·탭)·useK8sFilters(필터)·useK8sYamlEditor(YAML 편집·검색·diff)·useK8sResourceActions(scale·재시작·이미지 버전·istio·트래픽)·useK8sServiceEdit(service/ingress 편집)·(필요 시 useK8sPodLogs) — K8s.vue 잔류 ≤650(page 조립·자식 배선). 기존 자식 8종(`k8s/`)·`:page` 주입은 무변경 |
| DatabaseWorkbench.vue 2,456 | `database/` 자식 4~5(툴바·리소스 트리·테이블 데이터·쿼리 결과·Mongo/Redis 뷰) |
| MonitorDashboard.vue 2,600 | 자식 4~5(패널 편집 다이얼로그·패널 리스트·그리드·툴바) + composable 1 — 신규 파일명은 기존 monitor/ 뷰 13종과 충돌 0 실측 후(Dashboard* 접두 권장) |
| MainLayout.vue 1,397 | 자식 2~3(사이드바·헤더) — 최소 분할(§7-R5) |
| MonitorAlertRule.vue 1,037 | 자식 2(룰 폼·프리뷰/매처) |
| OpsJobDesigner.vue 1,009 | 자식 2(노드 폼·캔버스) |
| Host.vue 1,070 | 자식 2(호스트 폼·메트릭/터미널 패널) |
| AppPipelineCenter.vue 1,076 | 자식 2(파이프라인 폼·실행 이력) |
| AssetOverview.vue 849 | 자식 2(요약 카드·변경 로그) |

**row 4 비고**: Host.vue·AssetOverview.vue는 M2 SUPERSEDE 전환의 착지점 — 파일명·시맨 유지.

---

## 4. Phase 분해 (직렬 10~12 PR — 원본 파일 수 전부 ≤5)

```
[완결: P6 블록2·I10] ─▶ A ─▶ B ─▶ C ─▶ D ─▶ E ─▶ F ─▶ G ─▶ H(1~2 PR) ─▶ I ─▶ J
  A monitor 콘트롤러(레시피)     E database군        H K8s·DB 웹(최대·최고위험)
  B monitor.go〔임계〕           F integration군     I 모니터·레이아웃 웹
  C service.go〔임계〕           G controller.go     J 자산·앱·잡 웹
  D ops+database군 병행 가능 검토 후 직렬 유지
```

| Phase | 원본(수) | 분할 후(잔류 포함) | 성격 | 독립 검증(재배선 — 검증 H2) |
|---|---|---|---|---|
| **A** | controller/monitor.go (1) | 4 | 레시피 확립 — 기계 대조 스크립트 첫 실증 | CI-B 전종 + CM-1~4 |
| **B** 〔임계〕 | service/monitor.go (1) | 15 | 최대 파일·OTEL 좌표 | CI-B 전종 + CM-5~6 |
| **C** 〔임계〕 | service/service.go (1) | 11 | God object 파일 분할·공용 타입 처분 | CI-B 전종 + CS-1~4 |
| **D** | ops_application·ops·ops_schedule·ops_job (4) | 12 | ops 도메인 | CI-B 전종 + CO-1(ops군) |
| **E** | database·database_multidb·database_backup (3) | 12 | database 도메인 | CI-B 전종 + CO-2(database군) |
| **F** | integration_ai·integration_finops·notify·ssl_certificate·domain (5) | 16 | 통합·통지·인증서·DNS | CI-B 전종 + CO-3~6 |
| **G** | controller/controller.go (1) | 3 | 백엔드 최종 | CI-B 전종 + CC-1 |
| **H** (1~2 PR — 단순 M3) | K8s.vue·DatabaseWorkbench.vue (2) | 11~14 | 웹 최대 2(합 5,259행·R4 최고위험) — K8s와 Workbench 2 PR 분할 허용 | CI-W 전종 + CW-K8s·CW-WB |
| **I** | MonitorDashboard·MonitorAlertRule·MainLayout (3) | 9~11 | 모니터 웹+레이아웃 | CI-W 전종 + CW-ML·CW-DB |
| **J** | Host·AssetOverview·AppPipelineCenter·OpsJobDesigner (4) | 8~10 | 자산·앱·잡 웹 | CI-W 전종 + CW-4(r1 클레임 0 보강 — §6.3) |

**임계경로**: **B → C** 순차 필수(monitor 상호 호출 경계 → service 공용 타입 처분이 D~G 전제). A는 B의 저비용 예행. H는 R4(K8s 반응성)로 웹 임계 — 2 PR 분할로 반경 리스크 축소. revert는 역순, 각 PR 완전 독립 revert(같은 패키지 내 이동이라 revert 후 원본 복원으로 컴파일 회복).

**Phase 태그(검증 M5)**: 각 Phase 병합 직후 `i5-<phase>` 태그 생성이 의무. 다음 Phase 착수 claim이 `git tag --list 'i5-*'`로 직전 태그 존재를 확인한다(§6.1 CI-B11).

---

## 5. 보존 제약 (③구현 프롬프트에 **verbatim** 복사)

1. **behavior-neutral 최상위**: 이미 동작하는 라우트·권한·opdef·API 응답 형상·골든 상수·UX 렌더를 깨뜨리지 않는다. `backend/router/`·`backend/opdef/`·`backend/model/`·`backend/store/` 파일은 diff 0.
2. **백엔드 이동은 바이트 보존**(BCD·J1a 선례): 함수·타입 블록을 재작성하지 않고 이동만. 가감 허용 범위는 import 블록 정리와 package/doc 줄뿐. **각 커밋 메시지에 이동 블록의 원본 `파일:시작-끝` 좌표를 명기한다**(blame 단절 완화 — 위험 F4). `.git-blame-ignore-revs`는 이동 커밋 성격상 불필요(좌표 명기로 대체).
3. **시그니처·수신자 무변경**: Go 메서드 리시버·시그니처·export 여부·Vue defineProps/emits 형상 유지. 패키지 이동·이름 변경 금지.
4. **골든 2종 diff 0**: `docs/security/route-inventory.txt` 441행·`sensitive-routes.txt` 281행 무변경. 재생성 금지.
5. **char-baseline 114행·char-exclude 40행·v2-swap-ledger 무변경** — 본 과업 파일은 char 모집단 아님. `backend/service/testdata/` diff 0.
6. **`backend/internal/infra/`·`backend/internal/tasks/`·`backend/internal/api/v2/` 무변경**(pathspec 정정 — 교차 ③) — compare.go(1,243)·테스트 4건(engine_test 1,282 포함) 제외 대상. `backend/util/`·`backend/main*.go`도 diff 0.
7. **DB 스키마·설정 무변경**: 마이그레이션·AutoMigrate·config.yaml·seed 무변경. monitor ConfigJSON 소스·DTO 존속.
8. **모니터 OTEL 좌표 보존**: `MonitorInstantQuery`(:801)·`trimMonitorQueryHistories`(:839) 본체는 바이트 무변경 이동(→ monitor_query.go).
9. **650 라인캡**: 만지거나 만든 모든 파일 ≤650. 경고대 3건(model/ops.go 658·AppBuildTaskList.vue 683·OpsScriptLibrary.vue 698)을 성장시키지 않는다.
10. **원격 CI 대기 금지·무배포 전제** — §6 claims가 전부 로컬 대응(실행 cwd: 백엔드 `/mnt/d/DEV/acc0mplish/ops-admin/backend`·웹 `/mnt/d/DEV/acc0mplish/ops-admin/web`·저장소 루트 명령은 별기). 병합은 로컬 검증 후 보호 임시해제.
11. **웹은 행동 보존+재배선**(§3.8): 반응형 선언·이벤트 처리·API 호출·i18n 키 소비의 행동을 보존하고, composable 자유변수 주입·템플릿 `page.x` 재배선을 허용한다. 자식 컴포넌트는 `:page` 주입 패턴을 승계하며 v-model·slot·provide/inject 형상은 바꾸지 않는다. 신규 composable은 `web/src/composables/`에 배치(선례 정합).
12. **waiver·아티팩트 무삭제**: `v2-phase1/compare-gate-waiver-*.json` 포함 기존 검증 아티팩트 diff 0(§6.1 CI-B12).
13. **스펙 이탈은 조용히 두지 않는다** — §8 이탈 외 신규 이탈은 PR 설명 등재 후 머지.
14. **kubeconfig 임시 파일 계약 승계**(완전 M1): `opsPipelineKubeconfigFile`(정의 :1891)는 기존 존속 경로 — 권한 0600·cleanup 반환·단일 kubectl 세션 수명. 이동 시 계약 주석을 동반하고 신규 임시 파일 경로 추가 금지(phase6 §12 #16 승계).

---

## 6. 검증 요구 (claims — vacuity 금지·존재 증명 포함)

### 6.1 공통 — 백엔드 PR 전종 (A~G)

| ID | claim (검증 명령·cwd `backend/`) |
|---|---|
| CI-B1 | `go build ./...` exit 0 (구현자·리뷰 2회) |
| CI-B2 | `go test ./... -race` **31패키지** ok·exit 0 (테스트 보유 패키지 실측값) |
| CI-B3 | `git diff <base>..HEAD -- docs/security/` 0행 — 골든 441/281 무변경 (cwd 저장소 루트) |
| CI-B4 | `git diff <base>..HEAD -- backend/router/ backend/opdef/ backend/model/ backend/store/ backend/util/ backend/main*.go backend/internal/` **0행** (pathspec 정정 — `backend/internal/` 전체) |
| CI-B5 | 신규+잔류 파일 `wc -l` 전부 ≤650 — 실측치 수치 열거(§3 "Σ+H" 목표와 비교) |
| CI-B6 | 심볼 보존 — `grep -h '^func ' <원본>` 정렬본 vs `grep -h '^func ' <신규+잔류>` 정렬본 diff 0 (`^type ` 동일) |
| CI-B7 | **기계 전수(주 검출기)** — top-level 선언 블록 멀티셋 diff: 원본과 신규+잔류 전 파일에서 각 `^func /^type /^var /^const` 선언부터 닫는 `^}`까지 블록을 추출해 **정렬 후 diff 0**(블록 내부 바이트 동일·선언 순서 무관). 스크립트 명세(프로토타입 — Phase A에서 실체화·§10-4): `awk '/^(func|type|var|const) /{buf=$0;inblk=1;next} inblk{buf=buf"\n"$0} /^}$/{if(inblk){print buf"\n}";inblk=0}}' <files> | sort | diff - <baseline>` — **알려진 한계**: 한 줄 선언(`type X int` 등 `}` 없는 블록)은 잡지 못하므로 실체화 시 CI-B6(정렬 라인 diff)을 병행 실행해 보완. 구현자가 PR마다 baseline(원본 블록 덤프)과 비교 결과를 첨부, 리뷰가 재실행. 인간 3중 무작위 대조는 보조(강등 — 검증 H5). 무테스트 6종(§2.3)은 이것이 유일 검출기 |
| CI-B8 | `git diff <base>..HEAD -- backend/service/testdata/` 0행 |
| CI-B9 | `secret-scan` exit 0 |
| CI-B10 | **변경파일 집합 동등**(검증 H3) — `git diff --name-only <base>..HEAD`가 계획 §3 선언 집합(신규+원본+테스트 갱신 분)과 정확히 일치. 과계획 파일(선언 밖 수정)은 반려 사유 |
| CI-B11 | `git tag --list 'i5-*'` — 직전 Phase 태그 존재 확인(Phase A는 생략)·병합 후 `i5-<phase>` 태그 생성 |
| CI-B12 | `git diff <base>..HEAD -- v2-phase1/compare-gate-waiver-2026-09-08.json` 0행 포함 waiver 무변경 |

### 6.2 공통 — 웹 PR 전종 (H~J)

| ID | claim (cwd `web/` — CI-W3만 저장소 루트) |
|---|---|
| CI-W1 | `bun run build` exit 0 |
| CI-W2 | `playwright test slice-a` 통과(서빙 stack.sh 계약 — slice-b/c serve 전용·병렬 금지). MainLayout 회귀의 기계 방어 |
| CI-W3 | `node scripts/check-i18n-parity.mjs`(cwd 저장소 루트) PASS·korean-localization-guard 로컬 대응 PASS — 신규 자식은 기존 사전 키만 소비(재정의 금지) |
| CI-W4 | `git diff <base>..HEAD -- web/src/router/ web/src/api/ web/src/utils/` 0행 |
| CI-W5 | 신규+잔류 .vue/.js 전부 `wc -l` ≤650 (실측치 열거) |
| CI-W6 | **뷰 스모크 체크리스트(위험 F1 승격)** — e2e 미커버 8/9에 대한 보완: (a) `bun run build` 산출 `dist/assets/`에 분할 뷰 청크 존재(manifest/파일명 grep — 기계) (b) 구현자·리뷰 각 1회 로컬 serve로 분할 뷰 로드·콘솔 에러 0·주요 인터랙션 1개(탭 전환 또는 다이얼로그 오픈) 확인 — PR 설명에 뷰×항목 체크리스트 표로 기록(반기계·명시적 수용). 누락 뷰는 머지 불가 |

### 6.3 파일별 존재 증명 (전 Phase 배선 — 검증 H2)

| ID | claim (각 grep로 존재·행수·심볼 증명) |
|---|---|
| CM-1 | Phase A 분할 후 4파일 존재 — `grep -c '^func '` 4파일 합계 = 원본 **66** (r1 67 정정) |
| CM-2 | `trimMonitorQueryHistories`+`MonitorInstantQuery` 본체 monitor_query.go 이동(각 1매치)·본체 diff 0 |
| CM-3 | monitor.go 잔류 wc -l ≤50 |
| CM-4 | controller/monitor.go 잔류가 GetMonitorOverview(:14)·GetMonitorCommandCenter(:41) 보유 |
| CM-5 | Phase B 신규 14파일 각 wc -l ≤650 열거·합계 = 원본 4,801 − 잔류 − 중복 헤더(산술 정합) |
| CM-6 | `initMonitorScheduler`·`reloadMonitorAlertRules` 본체 이동 — service.New 호출부 diff 0 |
| CS-1 | 헬퍼 처분표(§3.2) 전량 이행 — Trimmed(:2571)·firstNonEmpty(:3047)·lastField(:2926)·optionalUint(:2650)·defaultSSHPort(:3056)·shortenText(:2611) 잔류 service.go에 각 1매치 |
| CS-2 | `AssetTerminalSession`·`OpenAssetTerminal` service_asset_terminal.go 이동 — 소비자는 이동 본체뿐(타 소비 0 — 원래 service.go 유일, 실측) |
| CS-3 | `CaptureCloudInstancesForCompare` service_cloud_capture.go 이동 — 소비자 **`main_cloudcompare.go:131`**(r1 main_compare 정정)·`main_cloudcompare_test.go:137` diff 0 |
| CS-4 | service.go 잔류 wc -l ≤200 — payload/도메인 전부 이동 |
| CO-1 | Phase D(ops군): ops.go 3분할 — `ExecuteOpsScript`→ops_script.go·`processOpsTask`→ops_exec.go·`runScheduledHTTPTask`→ops_schedule_exec.go·`executeOpsJobScriptNode`→ops_job_exec.go 각 1매치·잔류 4종 ≤650 |
| CO-2 | Phase E(database군): `ExecuteDatabaseSQL`→database_data.go·`runBatchSQLTask`→database_transfer.go·`getPostgresResourceData`→database_postgres.go·`getMongoResourceData`→database_nosql.go·`exportMySQLLogicalBackup`→database_backup_mysql.go 각 1매치·잔류 3종 ≤650 |
| CO-3 | Phase F(ai): `ParsePrometheusAlertTemplates`는 monitor 아님 — ai 계열 `parseDSMLToolCalls`→integration_ai_provider.go·툴 실행기 대표 `appendAILogStreamFilter`→integration_ai_tools_query.go 각 1매치 |
| CO-4 | Phase F(finops·notify): `GenerateFinOpsRecommendations`→integration_finops_advice.go·`dispatchPendingNotifications`→notify_dispatch.go·`renderNotifyTemplate`→notify_render.go 각 1매치 |
| CO-5 | Phase F(ssl·domain): `syncCertificateToCloudTask`→ssl_certificate_cloud.go·`parseCertificateAndKey`→ssl_certificate_parse.go·`MutatePublicRecord` 잔류 domain.go·`SaveInternalRecord`→domain_internal.go 각 1매치 |
| CO-6 | Phase D/E/F 잔류 파일 전수 wc -l ≤650 열거(ops_application·ops·ops_schedule·ops_job·database·multidb·backup·ai·finops·notify·ssl·domain) |
| CC-1 | controller.go 잔류가 New·Login 배선 유지·`AssetTerminalWS`(:980)·`ImportAssetHosts`(:861) controller_asset.go 이동 1매치·`BuildMenuTree`(:1304) 잔류 |
| CW-K8s | `web/src/composables/`에 K8s composable 5~7 존재·K8s.vue 잔류 ≤650 — **배선 형상**: template의 `:page="page"` 4회 그대로(grep 4매치 — r1 "배선 4줄 원문" 정정: 자식 내부 식별자는 page.x 재배선 허용) |
| CW-WB | DatabaseWorkbench 자식 4~5 존재(`database/`)·잔류 ≤650 |
| CW-ML | MainLayout 자식 2~3 존재·잔류 ≤650 — `displayTitle(menu)` 사이드바 자식 내 1매치 |
| CW-DB | MonitorDashboard 자식 4~5+composable 1·MonitorAlertRule 자식 2 — 각 잔류 ≤650 |
| CW-4 | Phase J 4뷰 각 자식 2 존재·잔류 ≤650·파일명 기존 뷰와 충돌 0 |

### 6.4 리뷰어 지침

- CI-B7은 리뷰가 **스크립트 재실행**으로 독립 재현 — 구현자 첨부 결과만으로 승인 금지. CI-B6(정렬 라인 diff)은 CI-B7(블록 멀티셋)에 흡수 포함 관계.
- CI-B10 변경 집합이 계획과 어긋나면 그 PR은 재계획 또는 반려 — "선언한 파일만"이 behavior-neutral의 폐쇄 조건.
- 웹 렌더 무변형 한계: slice-a(MainLayout 간접)+CI-W6 스모크가 전부. 스모크는 반기계적임을 PR에 명시(§7-R6).

---

## 7. 리스크 매트릭스 · 롤백

| # | 리스크 | 가능성 | 파급 | 완화 |
|---|---|---|---|---|
| R1 | monitor.go eval↔event 상호 호출 경계 파열 | 중 | 중 | 같은 패키지 — 컴파일러 전수 검증·시맨별 커밋으로 국소화. `evaluateMonitorAlertRule`(3419)은 §3.1 표의 rule/eval 경계에서 구현 시 호출그래프 재실측 |
| R2 | service.go 공용 타입·헬퍼 오판 | 중 | 중 | §3.2 처분표가 좌표 단위로 확정(교차 ② 해소)·컴파일러 검증·CS-1 존재 증명 |
| R3 | import 정리 범위 오버(본체 수정 혼입) | 중 | 높음 | CI-B7 기계 전수(주)·리뷰 무작위 보조(강등)·시맨별 커밋 국소화 |
| R4 | K8s.vue script 2,772줄 재조립 반응성 상실 | 중 | 높음 | 제약 #11(행동 보존+재배선)·`:page` 주입 무변경·composable은 page 객체 조립만·CI-W6 스모크·CW-K8s 배선 grep·**Phase H 2 PR 분할**(K8s 단독 PR로 회귀 국소화) |
| R5 | MainLayout 분할 회귀(전 라우트) | 중 | 높음 | 최소 분할·CI-W2 slice-a 전 라우트 통과·I Phase 배치(H 패턴 확립 후) |
| R6 | e2e 커버 밖 8/9 뷰 시각 회귀 | 높음 | 중 | **CI-W6 슴모크 체크리스트를 머지 게이트로 승격**(위험 F1) — 청크 존재(기계)+로드·인터랙션(반기계·PR 기록). 한계 잔존은 §6.4 명시 |
| R7 | 골든·라우트 무의미 변경 | 저 | 높음 | CI-B3·B4·B10 기계 차단 |
| R8 | 대형 PR 리뷰 피로(B 15·F 16·D 12·E 12) | 높음 | 저 | 시맨별 커밋·리뷰 커밋 단위 병행. **B·D·E·F·H 전부 2 PR 분할 허용**(r1 B·F에서 확대 — 각각 독립 빌드·검증·태그) |
| R9 | monitor 스케줄러 init 타이밍 변화 | 저 | 중 | `initMonitorScheduler`는 Service 메서드 — New 호출부 diff 0·본체만 이동(CM-6) |
| R10 | 650 초과 재등장 | 중 | 저 | CI-B5·CI-W5 PR마다 전수 잠금 |
| R11 | blame 단절로 이후 추적 곤란 | 중 | 저 | 제약 #2 — 커밋 메시지 원본 좌표 명기(위험 F4) |
| R12 | 무테스트 6종(§2.3)에서 이동 누락·변형 미검출 | 저 | 높음 | CI-B7 기계 전수가 유일 검출기 — 스킵 금지(§2.3 명문) |

**롤백**: 각 Phase PR 완전 독립 revert(같은 패키지 이동 — revert 시 원본 복원으로 컴파일 회복·선행 Phase 의존 없음). Phase 태그 `i5-<phase>`로 직전 안정점 reset(CI-B11이 태그 존재 보증 — revert 후 `git describe`로 클린 판정). DB·설정 무변경이라 데이터 롤백 불필요.

---

## 8. 이탈 기록 (스펙 대비)

| # | 항목 | 내용 |
|---|---|---|
| D-I5-1 | compare.go 1,243 미분할 | 하드캡 위반 +443줄 존치. 보존 제약 #6(`backend/internal/infra/**`) 명문 대상 + compare 게이트·waiver·Z 오라클 기반. 분할 착수는 별도 승인(기술 가능 — inventory 테스트 존재·정책 보류) |
| D-I5-2 | "하드캡 14건" → 실측 25건 | 블록 1 이전 측정치. 이월 대장 갱신은 종결 기록 시 |
| D-I5-3 | 경고대 3건 미처리 | model/ops.go 658·AppBuildTaskList 683·OpsScriptLibrary 698 (재실측 확정). 본 과업 밖 — 성장 금지만 승계 |
| D-I5-4 | **테스트 >800 4건 미분할**(완전 H1 — r2 신규 등재) | `internal/tasks/engine_test.go` 1,282·`internal/infra/adapter/proxmox/client_test.go` 844·`tencent/adapter_test.go` 838·`kubernetes/executor_test.go` 831. 전부 보존 제약 #6 영역(`backend/internal/**`)이라 제외와 정합. 프로덕션 25건 분할 후에도 테스트 파일 4건이 800 초과로 잔존함을 기록 |
| D-I5-5 | 리컨 정정 4건 | ① controller.go "미들웨어 체인" → thin 핸들러 모음 ② MonitorInstantQuery trim 실체는 `trimMonitorQueryHistories`(:839) ③ 테스트 파일 누락(판정 8 "전부 일치" 부정확) ④ 리컨 26행 수치는 프로덕션 한정 일치 |
| D-I5-6 | 웹 바이트 보존 불가 | 웹은 "행동 보존+재배선"(§3.8) — 원문 바이트 동일성을 요구하지 않는다(구조적 불가능: composable 주입·page.x 재배선) |

---

## 9. LOW 발견 처분표 (22건 — 재량·I10 r2 §9 형식)

| LOW 발견 | 처분 | 사유 |
|---|---|---|
| i18n 합 2,971→2,069 | 채택 | 재실측(cat 합계). §2.3 갱신 |
| service 패키지 42→43 | 채택 | 재실측. §2.3 갱신 |
| 경고대 "10파일"→3건 | 정정 채택 | 재실측 — 비테스트 650~800은 3건뿐(§1 비목표). 10건은 테스트 포함 오산으로 판단 |
| AssetTerminalWS :938→:980 | 채택 | grep 실측. §3.7 갱신 |
| query_history→trimMonitorQueryHistories | 채택 | 실측(:839). 판정 5·CI-B8 인접 서술·CM-2 갱신 |
| CI-W3 cwd 명시 | 채택 | §6.2 표기(cwd 저장소 루트) — 검증 M2 |
| CM 패키지 한정자 | 채택 | §3.1 주의 문구(service/ vs controller/ monitor_* 동명) |
| monitor_* 동명 충돌 명시 | 채택 | 동상 — 서로 다른 패키지·커밋 메시지 한정자 의무 |
| CW-K8s composable 위치 | 채택 | `web/src/composables/`(useK8sOperationProgress 선례) — §3.8·#11 갱신 |
| 웹 파일명 충돌 0 실측 문구 | 채택 | CW-DB·CW-4에 "충돌 0 실측" 포함 |
| prometheusRule/monitorLogShortcut 이중배정 | 채택 | prometheusRule*→monitor_template.go(병합본)·monitorLogShortcutDefault→monitor_shortcut.go — §3.1 갱신 |
| BuildMenuTree :1304 귀속 | 채택 | 잔류 controller.go 유틸 — CC-1 갱신 |
| R6 e2e 밖 열거 | 채택 | §2.3에 8/9 명시(실측 slice-a V2 면 전용) |
| R8 D 허용 | 채택 | R8 확대 — B·D·E·F·H 전부 2 PR 허용 |
| 무배포 전제 | 채택 | §1 비목표·§5 #10 명문 |
| 종결 시 state i5.task 26→25 | 채택 | 종결 기록 시 의무로 §8 D-I5-2에 병기 |
| r1 수치 5건(30패키지·67함수·28타입·:1892·:938) | 채택 | 전부 재실측 정정 반영(31·66·20·:1891·:980) |
| 판정 8 "차이 없음" 표현 | 채택 | 판정 8 재작성 — 프로덕션 한정 일치·테스트 누락 명시 |

---

## 10. 잔여 판단 (구현 착수 전 확정)

1. **Phase B 커밋 순서** — 시맨 의존 방향(payload→normalize→datasource→query→logs→…→event) 순차 권장. 구현자 호출그래프 실측 후 착지.
2. **integration_ai_tools 경계(1034/1563)** — query계/변환계 가부는 구현 시 재실측(§3.5).
3. **MonitorDashboard template 1,847줄 자식 경계** — 패널 편집 다이얼로그 우선·그리드·리스트 순.
4. **CI-B7 스크립트 실체화** — Phase A에서 awk 명세(§6.1)를 검증 스크립트로 만들어 첫 실증, 이후 전 Phase 재사용(레시피의 핵심 산출물).
5. **database_backup 공용 함수(664~753)** — listSchemaObjects·writeTableData의 pg 소비 여부 실측 후 mysql/pg 배정 확정.
