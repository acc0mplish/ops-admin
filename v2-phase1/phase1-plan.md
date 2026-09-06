# V2 Phase 1 계획 — Operational Foundation (마이그레이션 러너 + 시크릿 브로커 + 내구 태스크 엔진) (r2)

작성: 2026-09-06 · r2 개정: 2026-09-06 (5렌즈 적대검토 전원 조건부승인 — 병합 발견 반영) · 티어 XL (태스크 엔진 = 직렬 임계경로 코어 → core-xhigh, 나머지 위험 비례) · 앵커(실측 시점 HEAD): `362581c`

**r2 반영사항 (r1 대비 전량 — 무결 판정부는 미변경: §0.1 A1 12테이블 판정·§3.2–3.4 스키마 verbatim·§3.7 매핑 표 7종·게이트 ②④ 증명 구조)**:

- **A (T24 상태 어휘)**: provider_task.Status 종단 집합 4종(succeeded·failed·timed_out·cancelled) 확정 — `rejected`는 ApprovalStatus 값·task_event 타입 어휘로만 존재. §3.4 DDL CASE 목록·§3.6 Reject→cancelled·T29와 동치(전수 확인). §2·§6 T24 반영.
- **B (MySQL 검증 경로 확대)**: 계층 2(J5)에 마이그레이션 스위트(전 스텝 MySQLDDL) + §23.1 엔진 티어 5항목 중 실행 가능분(claim races·idempotency replay·resource-uniqueness rejection) 포함. 계층 2 실행을 Phase 1 게이트 산출물로 의무화(로컬 docker-compose MySQL 1회·claim 18·§10) — CI 잡 추가 없음(§0.8). R2 완화 문언을 실제 인도물과 정합화.
- **C (스텝 구성 방침 — 실측 기반)**: 신원 유니크를 GORM 태그 합성 uniqueIndex로 이동[실측 E-3a]. active_flag 유니크는 태그 표현 불가(생성 컬럼·단독 태그는 왜곡[실측]) → step0003 raw DDL + provider_task를 건드리는 이후 모든 스텝의 postlude 재적용 의무화 + T31 존재 단얫(sqlite_master·pragma_table_xinfo·information_schema). J6에 glebarez 재마이그레이션 인덱스 거동 실측 경고(재현 결과 병기) 명시.
- **D (배치표·소유권·허용 목록)**: §2 총산 정정 — 신규 27 + 수정 10종 완전 나열(구문 "22+6" 오기 폐지). `registry/registry.go`·`registry/registry_test.go`·`contract/capability.go`를 §2 배치표·§8 허용 목록·§12 소유권 표에 명시(claim 13 이행 가능화). PR16 "모델 9종"→8종 정정.
- **E (Work 탈락 복원)**: contract test harness(§23.1 contract 티어)는 **Phase 2 PR 20 소관 이연** 판정 — §13에 이연 기록·tracking 인계. ConnectionView 자격증명 전달(phase0 contract/adapter.go:36 계약)은 **PR 16 편입** 판정 — Material 필드 1건 확장(§3.5·§2·§8).
- **F (리스크·운영 계약)**: W-1 러너 실패=v1 기동 차단(fail-closed) 리스크 R12 신설+탈출구. W-3 §11 롤백 "역순 스택만 성립" 정정. W-4 적용 스텝 불변(§1 J6·§3.1·보존 제약 #11). W-5 raw DDL 존재 검사 멱등 가드. T-4/W-8 GET_LOCK — 런 전체 `sql.Conn(ctx)` 1개 핀 + T16 수제 ConnPool 래퍼 모의(§3.1).
- **G (엔진 설계 보강)**: T-5 Submit 빈 resource_uid 거부·T-6 유니크 위반 제약 판별(classifyUniqueViolation)·T-7 실행 체인 조인·UID→URN·soft-delete 거동·T8 시드 전제·T-8 비동기 폴 루프·리스≥타임아웃 계약·T-9 재시도 failed→queued 단일 트랜잭션·T-10 LOW 일괄(sqlite now() 부재 Go-side 바인딩·RequiresApproval 정준원천 registry 스냅샷으로 SubmitInput 플래그 제거·TaskAttempt 클레임 tx 생성·F-N Stop() 시그니처·F-O 교차참조 정정 3건: §6 서두 커버리지 claim 교차(claim 14→12)·§12/§3.7 매핑 표·registry 변경 소속 위상(T2→T4)·§4 잔류 "Phase A–D" 라벨(§5 실제 M·T 라벨로)).
- **H (검증 강화)**: claims 5–8에 `-v` + `=== RUN`/`--- PASS` 라인 카운트 추가(vacuous pass 봉쇄). 태스크 3테이블 존재·TableName 단얫 + 12테이블 집합 동치 테스트 T41 신설 — claim 4를 grep 카운트에서 집합 동치 검증으로 교체. T34 목적 문구 정정 + 결정적 인터리빙 주입(비원자 구현 반증) 대조군 추가. T39(Start/Stop 고루틴 누수) 신설. claim 11 제외 패턴에 `^v2-phase1/` 추가·claim 12 기계 판정 명령 원형·claim 13 식별자 이름 결합 제거·claim 16 토큰 한정형 확장(v1 cron `.Start(` 오탐 제거[실측]). F-B — J5에 자식 프로세스 SIGKILL 옵션 기록(R6 상향 시 최저비용 대응).

- 원천: `/mnt/d/DEV/acc0mplish/ops-admin/docs/architecture/multi-infrastructure-control-plane-v2.md` r2 + 정정 r2.1–r2.5 — 핵심 절: §3.2(M1 신규 테이블 목록), §4.2–4.3(envelope v2·삼중 판정 — 브로커가 재사용), §7(코어 도메인 모델), §8(리소스 신원·관측·관계), §9.1(동기화 런), §13(태스크 엔진 — 폴러·리퍼·승인·멱등·리소스 유일성·크래시 회복), §16.2(작업 API 형태 참조), §19(마이그레이션 전략), §20(Phase 1 게이트), §21(PR 15–19), §22(금지 지름길), §23(테스트 전략 -race).
- Phase 0 자산: `backend/internal/infra/{contract,registry,adapter/fake}` (PR #13 머지, p0-state.json "done"), ADR 6종 `docs/architecture/adr/000{1..6}-*.md` (PR #10 add62f2), `v2-phase1/phase0-plan.md` §13 인계 3건 중 **인계 1(capability→인터페이스 매핑 표)을 본 계획이 소유**한다(§3.7). 인계 2(R2 재설계)·인계 3(커버리지 래치 핀닝)은 §13 산출 외 인계로 이월.
- 범위 (스펙 §21 Phase 1, 5 PR): **PR 15** versioned migration runner + schema_migration · **PR 16** connection/context/credential/secretref(+inventory 4종) 모델 + 시크릿 브로커 · **PR 17** provider_task/attempt/event + 폴러 + 리퍼 · **PR 18** 승인 컬럼 + 멱등 + 리소스 유일성 · **PR 19** 크래시/충돌/멱등 스위트(-race).
- 출구 게이트 (§20 Phase 1): ① 크래시 회복(작업 중 워커 소실 → 리퍼 재큐) `-race` 하 green ② 동일 idempotency key → 동일 task 반환 ③ per-resource 유일성 — 두 번째 활성 mutation 차단 ④ fake adapter plan→approve→execute→audit 전주기(테스트에서). 각각 §7 claims로 기계 검증.
- 계획 계약: 본 문서는 `phase0-plan.md`의 관행(§0 실측 → 배치표 → 인터페이스 계약 → Phase 분해 ≤5파일 → 테스트 계약 → claims → 보존 제약 → 리스크 → 롤백 → 파일 소유권 → 가정 A번호)을 그대로 준수한다.

---

## 0. 현재 코드·자산 실측 (계획의 사실 기반)

### 0.1 스펙 테이블 목록 실측 — "13"은 선언, 실제 나열은 12

스펙 §3.2는 "New tables created in Milestone 1 **(13)**"로 선언하고 다음을 나열한다(3×4 행, 원문 배열 그대로):

```text
schema_migration           provider_connection        provider_context
provider_credential_binding  secret_ref                infra_resource
resource_observation        inventory_sync_run        infra_relationship
provider_task               task_attempt               task_event
```

행별로 세면 **12개**다. §7.7 이연 테이블 목록(11종)과 대조해도 누락된 13번째는 특정되지 않고, idempotency는 명시적으로 "별도 테이블 아님"(§3.2 괄호 — `provider_task(idempotency_key)` 유니크 인덱스 + 완료 체크). 본 계획은 **12 테이블이 실제 목록**이라 판정하고(가정 A1) 전 섹션에서 이 목록을 사용한다. 스펙 본문은 수정하지 않는다(보존 제약 #2). 리뷰·게이트 검수는 "12 테이블 전부 존재"를 기준으로 한다(claim 4).

### 0.2 기존 store/ 마이그레이션 구조

- `backend/store/migrate.go` — `AutoMigrate(db)`가 **91개 v1 모델을 명시적 리스트로** 마이그레이션. `main.go`가 매 기동 호출(§22 #12가 금지하는 "모든 API 복제본からの 자동 마이그레이션"의 현형 — 단일 인스턴스 배포라 실해는 없음).
- `backend/store/migrate_fixture_test.go` — 커밋된 sqlite baseline fixture(`store/testdata/migration-fixture/baseline.sqlite`)를 재오픈해 AutoMigrate+Seed의 업그레이드 경로를 증명. 재생성 커밋은 스키마 변경 커밋과 분리 규칙(파일 주석 + `docs/security/ci-baseline.md`). **V2 12 테이블은 AutoMigrate 리스트에 추가하지 않는다**(§1 판단 J2) — fixture 테스트는 무영향, 무수정 통과.
- `backend/store/db.go` — MySQL GORM 연결(`gorm.io/gorm v1.31.0`, `gorm.io/driver/mysql v1.6.0`).
- `backend/store/seed.go` — `RoutePermissionsMarkerValue = "route-permissions:granted:v1"` 마커 게이트 one-shot 시드 관례. V2 테이블은 시드 대상 아님.

### 0.3 model/ 관례 (V2 모델이 따를 형태)

- `TableName()` 명시 메서드 필수, `ID uint gorm:"primaryKey"`, gorm 태그에 `size:`/`type:text`/`serializer:json`(map·slice 컬럼, 예: `model/asset.go:86`), `CreatedAt/UpdatedAt`는 `json:"createTime"/"updateTime"` 태그 관례.
- UID 문자열 컬럼 관례 선례: `AssetService.ServiceUID`(uniqueIndex). ProviderTask.UID 등이 같은 패턴.
- ApprovalStatus/Approver 선례: `model/ops.go:538-539` (`gorm:"size:32;default:not_required;index"`, `gorm:"size:128"`) — §13.3 "OpsJob 의미 재사용"의 구체 구현 관례.

### 0.4 테스트 인프라와 -race 현실

- `backend/internal/testutil.OpenMemoryDB(t, ddl...)` — glebarez(pure-Go CGO-free) sqlite in-memory, **MaxOpenConns(1)** 고정(":memory:" 신규 커넥션 공백 DB 방지). `PinSecretKeys(t)`가 키세트 고정.
- `.github/workflows/v2-ci.yml` `backend-test` 잡이 이미 `go test ./... -race -count=1`로 전체 패키지를 돈다. **-race는 CI·로컬 전부 기본 동작** — 신규 패키지가 자동으로 -race 커버된다.
- `migration-test` 잡 = `go test ./store/ -run TestMigrationFixture`(sqlite fixture). **MySQL testcontainer 잡은 존재하지 않는다.** §23.1 engine 티어의 "against a real MySQL (testcontainer)"는 현재 CI에 인프라가 없다 → §1 판단 J5·리스크 R6에서 계층화.

### 0.5 폴링/리퍼 고루틴 수명 주기 — 기존 기동에 붙는 방식

- `router.New(cfg, db)` → `(*gin.Engine, *service.Service)`; Service 생성자(`service/service.go:75-78`)가 `initOpsScheduler()` 등 `sync.Once` lazy 스케줄러 4종을 내부 기동. `main.go` 종료 시 `svc.Shutdown(ctx)` + `server.Shutdown(ctx)`.
- 즉 기존 관례는 "생성자 안에서 lazy 시작, 컨텍스트·Shutdown으로 정지". 태스크 엔진은 이 관례를 따르되 **Phase 1은 main.go에 폴러를 배선하지 않는다**(§1 판단 J4) — Engine은 `Start(ctx)`/`Stop()` API를 갖고 테스트만이 기동한다.

### 0.6 Phase 0 인도 자산의 소비 지점

- `internal/infra/contract` — `JSONMap`, `ProviderTypeDescriptor`, `Capability`, `OperationDefinition`, `RetryPolicy{MaxAttempts,BackoffSeconds}`, 어댑터 인터페이스 5종, `ConnectionView{ProviderType,Endpoint,Config}`, `OperationStatus{State string; Detail JSONMap}`(주석: "종단 상태는 Phase 1 폴러 설계 시점에 확정"), 지원 타입. **전부 stdlib만 import**(순수성 게이트, phase0 claim 13).
- `internal/infra/registry` — `RegisterProviderType/RegisterCapabilities/RegisterOperation/ProviderType/Operation`, V5 최소 가드(`servesCapabilities`). capability별 인터페이스 매핑은 미구현(인계 1 → 본 계획 §3.7).
- `internal/infra/adapter/fake` — BaseAdapter만 구현(등록 전용). Discoverer/OperationExecutor는 Phase 1이 추가한다(phase0 가정 A7 해소).
- `util/secretv2.go` — `EncryptSecretV2/DecryptSecretV2/ReadSecretField/ClassifySecret`, `ConfigureSecretMasterKeys`. **브로커는 `DecryptSecretV2`만 사용**한다(§4.3 "마이그레이션 도구와 런타임 복호 경로는 하나의 구현을 공유" + Step 4 완료로 legacy 경로 폐기 — v2 전용 판독).
- arch-boundary: `scripts/check-arch-boundary.sh` R1–R3, `CORE_PACKAGES=(internal/domain/dnsserver)` — **신규 `internal/infra`·`internal/tasks` 패키지는 R1/R2 보호 밖**(phase0 §13 인계 2 미해결). 본 계획의 방어선은 claim 16(무발화)뿐 — 리스크 R5.

### 0.7 테이블명 충돌 검사

기존 90개 `TableName()` 리터럴과 V2 12 테이블명의 교집합 = **0건**(`grep -oP 'return "\K[a-z_]+' model/*.go` 실측 대조). 이름 충돌 없이 additive 생성 가능.

### 0.8 CI 로컬 검증 방침 (브리핑 §9 계약)

저장소 정책: **merge 판정은 로컬 명령, 원격 CI 대기 없음.** v2-ci.yml의 9잡은 참조용이며 본 계획의 claims(§7)는 전부 로컬 실행 명령 형태다. `go test ./... -race -count=1`이 백엔드 전체 판정 명령(CI `backend-test`와 동일).

---

## 1. 아키텍처 판단 (선택지 비교 + 선정 근거)

### J1 — 버전 관리 마이그레이션의 적용 범위: "V2 12 테이블만 versioned" (선정) vs "v1 91 테이블 전체 전환" (기각) vs "독립 서브커맨드 전용" (기각)

- **기각(전체 전환)**: 91 테이블의 AutoMigrate 이력을 버전 마이그레이션으로 재작성하는 것은 Phase 1 게이트(§20)가 요구하지 않는 대형 리스크. v1 스키마 변경 관리는 §3.2 처분 행렬이 REMAIN/EXTEND로 다루는 v1 자산 — M2 커트오버(§19.1) 전까지 기존 경로가 정당하다. 스펙 §19가 금지하는 것은 "래너 없는 자동 마이그레이션"의 **무결정성**이지 AutoMigrate 존재 자체가 아니다.
- **기각(서브커맨드 전용)**: 기동 시 자동 실행이 아니면 단일 운영자 저장소에서 " 까먹고 안 돌림" 상태가 상수가 된다. §19의 "database advisory lock"은 기동 시 자동 실행 + 락을 전제로 한다.
- **선택**: 러너는 **V2 12 테이블(+차후 V2 확장)만 소유**하고, main.go 기존 `store.AutoMigrate(db)` 직후에 호출한다(배선 최소 — §1 J4와 통합). v1 스키마는 기존 경로 무변경. 결과: v1 동작 무변경 + V2 스키마는 version·락·순서 강제. 절충 비용(두 마이그레이션 체계 공존)은 명시적 부채로 기록하며 해소 시점은 M2 커트오버다.

### J2 — V2 모델 배치: `backend/internal/infra/model/` 단일 모델 패키지 (선정) vs `backend/model/` 추가 (기각) vs 도메인별 분산 (기각)

- **기각(model/ 추가)**: `model`은 §3.2 처분 행렬이 다루는 v1 자산 덩어리다. V2 12 테이블을 섞으면 "이 PR이 §3 REMAIN 도메인을 넓히는가"(§22 #18) 리뷰 판정이 흐려진다. 또한 `store/migrate.go`의 명시적 리스트에 우발 등록될 여지를 원천 차단해야 한다.
- **기각(도메인별 분산 — infra/model, tasks/model 분리)**: `provider_task.resource_uid`↔`infra_resource.uid`, `provider_credential_binding.secret_ref_id`↔`secret_ref` 등 FK·유니크 관계가 패키지를 가로지르면 순환 import 위험이 생긴다. GORM 모델은 순수 데이터 구조이므로 한 패키지가 비용 0.
- **선택**: `backend/internal/infra/model/`에 12 테이블 모델 전부(schema_migration 포함 — 러너의 스키마 버전 표도 모델로 표현). 태스크 엔진 로직 패키지(`internal/tasks`)는 이 모델을 import해 사용 — 스펙 §23.3 "new V2 packages" 커버리지 측정 단위와도 정합.

### J3 — 패키지 레이아웃 (라벨→경로 매핑)

```text
backend/internal/infra/migrate/    PR 15 — versioned migration runner
backend/internal/infra/model/      PR 16/17 — 12 테이블 GORM 모델 (+schema_migration)
backend/internal/infra/secrets/    PR 16 — 시크릿 브로커
backend/internal/tasks/            PR 17/18 — 태스크 엔진(클레임·상태머신·폴러·리퍼·멱등·유일성)
backend/internal/infra/contract/   Phase 0 자산 — 본 Phase가 최소 확장(§3.6)
backend/internal/infra/adapter/fake/ Phase 0 자산 — 실행 인터페이스 구현 추가(§3.8)
```

근거: PR 17/18의 라벨 `feat(tasks)`와 스펙 §2.5 모듈 어휘(tasks는 provider와 독립인 모듈)에 정렬. 엔진이 provider 유형명을 모른 채 contract 인터페이스만 소비하는 경계(arch rule 1)가 패키지 구조로 강제된다. `internal/domain/provider`(DNS V1)와의 경로 충돌 회피는 phase0 §0 판정 승계.

### J4 — main.go 배선 최소 허용 범위 (보존 제약 #1의 구체화)

허용되는 main.go 변경은 **정확히 하나**: 기존 `store.AutoMigrate(db)` 성공 블록 직후에 V2 러너 호출 추가.

```go
if err := inframigrate.Run(db); err != nil {
    log.Fatalf("v2 migrate failed: %v", err)
}
```

(신규 import 1개 + 3–4줄.) **허용 외**: 폴러/리퍼 기동(`Engine.Start`) 배선 금지 — Phase 3이 plan→execute API와 함께 소유(가정 A6). 근거: §5.4 "Phase 1은 홀로 출하되지 않는다"(테이블만 존재·프로덕션 데이터 없음), §22 #7은 "새 V2 코드"의 변이 경로에 적용되는데 Phase 1에는 제출 API가 없다 — 빈 폴링 고루틴만 프로덕션 프로세스에 추가하는 것은 v1 무변경 원칙 위반 소지이고 게이트 증명도 없다. 폴러가 테스트 외 기동되지 않는 조건은 claim 17로 검증한다.

### J5 — 크래시 회복·MySQL 검증의 계층화: sqlite 결정적 시뮬레이션(기본) + MySQL 실측 경로(의무 1회)

§23.1은 "real MySQL (testcontainer): SIGKILL worker between claim and commit"을 명시하나 저장소 CI에 MySQL 잡이 없고(§0.4) 저장소 정책상 CI 잡 추가도 없다(§0.8). r2부터 계층 2는 선택이 아니라 **Phase 1 게이트 산출물**이다:

- **계층 1 (필수, CI/로컬 기본)**: sqlite에서 "클레임 확보 후 종단 커밋 없이 소유자 소실"을 결정적으로 시뮬레이션 — 클레이머가 `ClaimNext` 성공 후 아무 것도 커밋하지 않고 사라진 상태(= 크래시의 스키마적 본질: running 행 + 만료 리스 + 미완 attempt)를 만들고, 리스 만료 시점을 조작(lease_expires_at 과거 설정)해 `ReapOnce`가 재큐(`status='queued'`, `task_event 'reaper_requeued'`)·시도 소진 시 `failed ErrorCode='lease_expired'`·"유실 태스크 0"을 단언. 고루틴 타이밍에 의존하지 않는 `RunOnce`/`ReapOnce` 결정적 진입점이 전제다(§3.4).
- **계층 2 (의무, env-gated, 게이트 산출물 — r2 B)**: `OPS_TASKENGINE_MYSQL_DSN` 설정 시에만 동작(미설정 시 `t.Skip`)하는 MySQL 실DML 스위트. 라이브 자격증명 없음(§23.2 정신 — 로컬 docker-compose MySQL 대상; compose mysql 서비스 `ops-admin-mysql` 존재 실측, MySQL 8.0 — 호스트 포트 미노출 시 동등 로컬 인스턴스 허용, A16). 범위는 §23.1 엔진 티어 5항목 중 로컬 실행 가능분 3종 + migration 티어 "clean install" 등가:
  - `TestMySQLMigrationSuite` — 전 스텝(0000–0003) MySQLDDL 클린 스키마 적용·재실행 no-op(스킴 자체 생성·해제는 테스트가 자기 관리)
  - `TestMySQLParallelClaims` — 병렬 클레임 CAS 원자성(§23.1 "claim races" — REPEATABLE READ에서의 UPDATE WHERE 경쟁)
  - `TestMySQLResourceUniqueness` — 생성 컬럼+조합 유니크의 MySQL 거동(§23.1 "resource-uniqueness rejection")
  - `TestMySQLIdempotencyReplay` — 유니크 위반 에러 매핑(T-6)의 MySQL 방언(§23.1 "idempotency replay")
  - "reaper recovery"는 계층 1이 논리를 증명하고 MySQL 부가가치가 claim races와 겹치므로 필수분에서 제외. "crash injection"은 아래 F-B 옵션으로 기록만 둔다.
  **실행 계약: 게이트 종결 전 1회 실행 출력(전문)을 tracking 이슈에 첨부한다(claim 18·§10). CI 잡 추가 없음.**
- 게이트 ①은 계층 1으로 충족한다(가정 A7): 게이트 문구 "kill worker mid-task, reaper re-queues"의 검증 대상은 소유자 소실 회복이지 OS 시그널 전달 자체가 아니며, 리스+리퍼 회복은 클럭 조작 하에 결정적이다. SIGKILL 문자 그대로의 프로세스 크래시 증명은 계층 2 옵션·Phase 3 크래시 주입(e2e)으로 인계한다.
- **F-B (옵션 기록 — r2 H)**: R6 이견이 잔존해 계층 2를 SIGKILL 실증으로 상향해야 하는 경우의 최저비용 대응 — 자식 프로세스(테스트 바이너리 재실행)가 **파일 sqlite**에서 클레임 확보 후 부모가 SIGKILL·부모 프로세스에서 `ReapOnce` 재큐를 검증하는 테스트 1건(인메모리 sqlite는 프로세스 사멸로 불가 — 파일 DB 전제). 기본 형상이 아니며 도입 여부는 리뷰 판정.

### J6 — 마이그레이션 스텝의 구현 수단: GORM AutoMigrate(스텝 내) + raw SQL 보강 (선정) vs 전량 raw SQL (기각)

- **기각(전량 raw SQL)**: MySQL 8.0은 `ADD COLUMN IF NOT EXISTS`를 지원하지 않아 부분 실패 후 재실행이 비멱등 — 러너가 자체 멱등 DDL 생성기가 되어야 한다. 또한 sqlite(테스트)와 MySQL(프로덕션) DDL을 이중 유지해야 한다.
- **선택**: 각 마이그레이션 스텝은 `Models []any`(스텝이 지정한 테이블만 `db.AutoMigrate(models...)` — 컬럼 존재 검사 후 ADD, 멱등) + `MySQLDDL/SQLiteDDL []string`(AutoMigrate가 표현 못하는 **생성 컬럼·조합 유니크·부분 유니크만**, per-dialect). GORM이 dialect 차이를 흡수하고, raw 구간은 최소화된다. dirty 플래그(§3.2)가 raw 구간 실패를 가둔다.
- **r2 스텝 구성 계약 (실측 2026-09-06, glebarez v1.11.0 / go-sqlite v1.21.2)**:
  - **[E-3a]** 태그 합성 uniqueIndex(조합 포함)는 재 AutoMigrate에서 **생존 실측** — 표현 가능한 제약은 전부 모델 태그로 표현한다. 신원 유니크 `UNIQUE(context,kind,external_urn)`가 이 경로로 이동했다(§3.3).
  - **경고**: raw DDL로 만든 인덱스·생성 컬럼의 재마이그레이션 생존은 **계약이 아니다** — 리뷰 [E-3] 경고(미선언 인덱스 삭제)는 이 버전에서는 재현되지 않았으나([E-3c/d/g] 동일 모델 재마이그레이션·컬럼 타입 변경·컬럼 추가 전 시나리오에서 보존 관찰), glebarez/GORM 상향에서 언제든 깨질 수 있는 관찰일 뿐이다. 따라서 태그 불가능 제약(active_flag 생성 컬럼 §3.4)은 **provider_task를 건드리는 이후 모든 스텝이 raw DDL을 존재 검사 후 재적용하는 postlude를 의무 실행**한다(W-5). 태그로 표현 가능한 것을 raw로 두는 것은 금지(신원 유니크 raw 보강 폐지).
  - **[E-3h]** 생성 컬럼은 `pragma_table_info`에 미표시된다(hidden=3 — xinfo로만 보임) — 존재 단얫·존재 검사는 sqlite `pragma_table_xinfo`·MySQL `information_schema.columns`로만 작성한다(T31·W-5).
- **W-4 (불변 스텝)**: 한번 적용된 스텝 정의는 불변이다 — 수정·철회는 금지되고 스키마 변경은 항상 신규 버전 스텝(0004+)로만 반영된다. 보존 제약 #11로 구현 프롬프트에 복사된다.
- **W-5 (raw DDL 멱등 가드)**: 모든 raw DDL 실행기는 사전 존재 검사(information_schema·sqlite_master — 생성 컬럼은 E-3h에 따라 xinfo/information_schema.columns)를 통과해 멱등하며, postlude 재적용은 이미 존재하면 no-op다.

### J7 — 시크릿 브로커의 "short-lived purpose-scoped" M1 구현 수준

§7.5/§11 "purpose-scoped, short-lived material"을 M1 internal 백엔드에서 물리적 TTL로 구현하는 것은 의미가 없다(메모리 값은 회수 불가). M1 구현은: ① `Resolve`가 바인딩의 `Purpose`를 인자와 정합 검사(위조 목적 접근 차단) ② 결과를 **캐시하지 않는다**(호출 단위 일회 뷰) ③ `SecretRef.Ciphertext`는 v2 envelope만 수용(legacy 폐기 확정). TTL·발급 토큰화는 외부 백엔드 등장 시(§7.7) 재개. `secret_access_total` 메트릭(§18.2)은 관측 엔드포인트가 없어 미계측 — 브로커에 Observer 훅도 만들지 않는다(YAGNI, 절충 기록).

---

## 2. 수정 대상 (우선순위·파일 배치표 — 전 파일 경로 명시)

### PR 15 — 버전 관리 마이그레이션 러너 (`feat(store)` · 스펙 §21 #15)

| 우선순위 | 파일 | 신규/수정 | 내용 요약 |
|---|---|---|---|
| 1 | `backend/internal/infra/migrate/runner.go` | 신규 | 러너 코어 — advisory lock(**런 전체 `sql.Conn(ctx)` 1개 핀** T-4/W-8·MySQL `GET_LOCK`/sqlite 우회, A15), schema_migration 테이블 보증, pending 스텝 순서 실행, version 기록, dirty 플래그, raw DDL 존재 검사 멱등 실행기(W-5). 스텝 불변 계약(W-4 — §3.1) |
| 2 | `backend/internal/infra/migrate/migrations.go` | 신규 | `Step` 타입 + 등록 순서 리스트(§3.2). 이후 PR이 파일을 1줄씩 추가(순차 직렬 지점). 적용된 스텝은 불변(W-4) |
| 3 | `backend/internal/infra/migrate/step0000_bootstrap.go` | 신규 | version 0: schema_migration 자체 생성(`CREATE TABLE IF NOT EXISTS` — 부트스트랩은 반드시 raw) |
| 4 | `backend/internal/infra/migrate/runner_test.go` | 신규 | T14–T17 (§6) + `TestMySQLMigrationSuite` 골격(계층 2 env-gated — J5, 스텝 등록과 함께 성장) |
| 5 | `backend/main.go` | **수정** | V2 러너 호출 배선 (§1 J4 허용 범위 유일 수정) |

### PR 16 — 인프라 모델 8종 + 시크릿 브로커 (`feat(infra)` · §21 #16 + §20 Phase 1 Work "the 13 M1 tables"의 인벤토리 4종)

| 우선순위 | 파일 | 신규/수정 | 내용 요약 |
|---|---|---|---|
| 1 | `backend/internal/infra/model/provider.go` | 신규 | ProviderConnection(§7.2)·ProviderContext(§7.3)·ProviderCredentialBinding(§7.4) — 필드 verbatim |
| 2 | `backend/internal/infra/model/secretref.go` | 신규 | SecretRef(§7.5) — `Ciphertext`는 v2 envelope 전용 주석 계약 |
| 3 | `backend/internal/infra/model/inventory.go` | 신규 | InfraResource(§8.1 — 신원 유니크는 **GORM 태그 합성** `uq_infra_resource_identity`, r2 C)·ResourceObservation(§8.2)·InventorySyncRun(§9.1)·InfraRelationship(§8.4) |
| 4 | `backend/internal/infra/migrate/step0001_infra_foundation.go` | 신규 | version 1: 모델 AutoMigrate — **Phase M1에서 기반 4종으로 생성, Phase M2에서 인벤토리 4종을 합쳐 완성**(PR 16 머지 시점에 8종 확정 후 불변, W-4). 신원 유니크 raw 보강은 폐지 — 태그 합성으로 AutoMigrate가 생성(r2 C·E-3a) |
| 5 | `backend/internal/infra/model/model_test.go` | 신규 | T18–T20: TableName·태그·유니크 인덱스 존재 단언 |
| 6 | `backend/internal/infra/migrate/migrations.go` | 수정 | step 0001 등록 1줄 |
| 7 | `backend/internal/infra/secrets/broker.go` | 신규 | 시크릿 브로커(§3.5) — DecryptSecretV2 전용 |
| 8 | `backend/internal/infra/secrets/broker_test.go` | 신규 | T21–T23 |
| 9 | `backend/internal/infra/contract/adapter.go` | **수정** | ConnectionView.Material 필드 1건 확장 — phase0 `contract/adapter.go:36` 계약 이행(r2 E·§3.5). contract stdlib 순수성 유지(타입 `map[string]string`) |

### PR 17 — 태스크 모델 3종 + 엔진 코어 + 폴러 + 리퍼 (`feat(tasks)` · §21 #17)

| 우선순위 | 파일 | 신규/수정 | 내용 요약 |
|---|---|---|---|
| 1 | `backend/internal/infra/model/task.go` | 신규 | ProviderTask(§13 조립형, §3.3)·TaskAttempt·TaskEvent — 승인/멱등/유일성 컬럼 제외(PR 18이 Expand) |
| 2 | `backend/internal/infra/migrate/step0002_tasks.go` | 신규 | version 2: 태스크 3종 테이블 |
| 3 | `backend/internal/infra/migrate/migrations.go` | 수정 | step 0002 등록 1줄 |
| 4 | `backend/internal/tasks/state.go` | 신규 | 상태 상수·전이표(§13.5 verbatim)·**종단 판정 4종**(succeeded·failed·timed_out·cancelled — `rejected`는 ApprovalStatus 값·task_event 어휘 전용, r2 A)·task_event 타입 어휘 |
| 5 | `backend/internal/tasks/engine.go` | 신규 | Submit(**빈 ResourceUID 거부 T-5·registry 정의 스냅샷 T-10**)·ClaimNext(§13.2 CAS·**클레임 tx에서 TaskAttempt 생성·리스 하한 T-8**)·Complete/Fail(**재시도 failed→queued 단일 트랜잭션 T-9**·시도+이벤트 동일 트랜잭션)·**classifyUniqueViolation(T-6)**·**실행 체인 조인 resource_uid→infra_resource→context→connection + UID→URN 조립(T-7)**·버전 CAS 쓰기 |
| 6 | `backend/internal/tasks/loops.go` | 신규 | Engine.Start/Stop — 폴러+리퍼 고루틴, `RunOnce`/`ReapOnce` 결정적 진입점 |
| 7 | `backend/internal/tasks/reaper.go` | 신규 | 스테일 리스 리퍼(§13.2) — 재큐/소진 실패/이벤트 |
| 8 | `backend/internal/infra/adapter/fake/fake.go` | 수정 | Discoverer·OperationExecutor·TaskPoller 구현 추가(Phase 0 등록 전용 형상 확장, phase0 가정 A7 해소) |
| 9 | `backend/internal/tasks/engine_test.go` | 신규 | T24–T27·T40 |
| 10 | `backend/internal/infra/adapter/fake/fake_test.go` | 수정 | 실행 인터페이스 단언 추가 |
| 11 | `backend/internal/infra/contract/capability.go` | **수정** | §3.7 매핑 표 추가(어휘 7종 전수 — r2 D) |
| 12 | `backend/internal/infra/registry/registry.go` | **수정** | `RegisterCapabilities`가 매핑 표 기반 검증으로 강화(V5 최소 가드 → 표 기반, §3.7 — r2 D) |
| 13 | `backend/internal/infra/registry/registry_test.go` | **수정** | 매핑 표 검증 케이스 확장(기존 `TestCapabilityInterfaceMappingRule` 자리·phase0 주석 "Phase 1 계획 소관" 이행) |
| 14 | `backend/internal/infra/model/model_test.go` | **수정** | 태스크 3테이블(provider_task·task_attempt·task_event) 존재·TableName 단얫 + 12테이블 집합 동치 T41(r2 H) |

### PR 18 — 승인 컬럼 + 멱등 + 리소스 유일성 (`feat(tasks)` · §21 #18)

| 우선순위 | 파일 | 신규/수정 | 내용 요약 |
|---|---|---|---|
| 1 | `backend/internal/infra/migrate/step0003_task_guards.go` | 신규 | version 3: provider_task Expand — approval 3컬럼·idempotency_key 유니크·resource_uid+active_flag 생성컬럼 유니크(§3.4). active_flag raw DDL postlude의 원점 — 이후 provider_task를 건드리는 모든 스텝은 이 DDL을 존재 검사 후 재적용한다(r2 C·W-5) |
| 2 | `backend/internal/infra/migrate/migrations.go` | 수정 | step 0003 등록 1줄 |
| 3 | `backend/internal/infra/model/task.go` | 수정 | 신규 컬럼 모델 필드 반영 |
| 4 | `backend/internal/tasks/idempotency.go` | 신규 | 멱등 제출 — 유니크 충돌 감지 후 기존 task 회수·재생 플래그(§13.4) |
| 5 | `backend/internal/tasks/approval.go` | 신규 | Approve/Reject — planned→awaiting_approval→queued 전이(§13.3), OpsJob 어휘 재사용 |
| 6 | `backend/internal/tasks/guards_test.go` | 신규 | T28–T31 |

### PR 19 — 크래시/충돌/멱등 스위트 (`test(tasks)` · §21 #19)

| 우선순위 | 파일 | 신규/수정 | 내용 요약 |
|---|---|---|---|
| 1 | `backend/internal/tasks/crash_suite_test.go` | 신규 | N7 크래시 회복 시나리오(§1 J5 계층 1)·-race |
| 2 | `backend/internal/tasks/collision_suite_test.go` | 신규 | 배리어 동시 출발 N고루틴 클레임 경쟁 + 결정적 인터리빙 대조군(T34)·-race + `TestMySQLParallelClaims`(계층 2 — J5 의무) |
| 3 | `backend/internal/tasks/idempotency_suite_test.go` | 신규 | N6 — 동일 키→동일 task·실행 1회(어댑터 이중(faker) 이중 실행 카운트 단언) + `TestMySQLIdempotencyReplay`(계층 2) |
| 4 | `backend/internal/tasks/uniqueness_suite_test.go` | 신규 | N11 — 동일 리소스 두 번째 활성 mutation → `resource_busy` 즉시 실패 + `TestMySQLResourceUniqueness`(계층 2) |
| 5 | `backend/internal/tasks/cycle_suite_test.go` | 신규 | 게이트 ④ — fake로 plan→approve→execute→audit 전주기 + 동시성 종합 + T39(Start/Stop 고루틴 누수) |

총 산출 (r2 D 정정): **신규 27파일 + 기존 파일 수정 10종**. 수정 10종 전량 나열 — `backend/main.go`(PR 15 배선), `internal/infra/migrate/migrations.go`(PR 16/17/18 각 1줄), `internal/infra/model/model_test.go`(PR 17 태스크 단얫·T41), `internal/infra/model/task.go`(PR 18 컬럼 반영), `internal/infra/adapter/fake/fake.go`·`fake_test.go`(PR 17 실행 인터페이스), `internal/infra/contract/capability.go`(PR 17 매핑 표), `internal/infra/contract/adapter.go`(PR 16 Material 필드), `internal/infra/registry/registry.go`·`registry_test.go`(PR 17 매핑 검증). 기존 **v1** 코드 수정은 **main.go가 유일**하다 — 나머지 수정 9종은 전부 Phase 0 자산(`internal/infra/**`)이며, §8 허용 목록·§12 소유권 표와 동일한 집합이다.

---

## 3. 인터페이스 계약 — 코어 계약 심층 (스펙 어휘의 정확한 코드화)

원칙(phase0 §3 승계): 스펙이 준 것만 코드화한다. §7·§8·§9의 구조체 필드는 한 글자 수정 없이(주석으로 절 인용). 스펙이 산재 필드로만 정의하는 provider_task는 계획이 조립하며 각 필드에 근거 절을 단다(가정 A3). 임의 필드 추가는 보존 제약 #7로 금지.

### 3.1 schema_migration (§19 "schema version table" — 형상은 계획 정의, A8)

```go
// SchemaMigration is §19's schema version table. One row per applied step;
// dirty=true marks a partially-failed step requiring manual resolution
// (MySQL DDL is non-transactional — auto-retry of a torn step is riskier
// than stopping).
type SchemaMigration struct {
    Version   int64  `gorm:"primaryKey"`
    Name      string `gorm:"size:255;not null"`
    Dirty     bool   `gorm:"not null;default:false"`
    AppliedAt time.Time
}
func (SchemaMigration) TableName() string { return "schema_migration" }
```

**러너 계약 (r2 F — 구현 프롬프트에 그대로 반영되는 4계약)**:

- **T-4/W-8 (advisory lock 커넥션 핀)**: MySQL `GET_LOCK`은 세션 스코프라서 풀링된 커넥션 사이에서 저절로 해제된다 — 러너는 **런 전체를 통해 `sql.Conn(ctx)` 1개를 핀**하고 획득(`GET_LOCK('ops-admin:v2-migrate', 0)`, 실패=즉시 기동 오류, A15)·전 스텝 실행·`RELEASE_LOCK`을 전부 그 커넥션 위에서 수행한다(스텝 실행 쿼리도 동일 커넥션 — 락 커넥션과 별도 풀 사용 금지). sqlite(테스트)는 우회(단일 커넥션 in-memory).
- **W-4 (불변 스텝)**: 한번 적용된 스텝은 수정·철회 불가 — 변경은 항상 신규 버전 스텝(§1 J6·보존 제약 #11).
- **W-5 (raw DDL 멱등 가드)**: 모든 raw DDL은 존재 검사(information_schema·sqlite_master — 생성 컬럼은 [E-3h]에 따라 `pragma_table_xinfo`/`information_schema.columns`)를 통과해 멱등 실행된다.
- **T16 모의 수단**: GET_LOCK 획득/반환 호출 단얫은 gorm `ConnPool` 인터페이스의 **수제 테스트 래퍼**(`ExecContext` 가로채기 — 락 문 기록)로 검증한다. sqlmock 등 신규 의존 도입 금지(보존 제약 #4 준수).

### 3.2 인프라 기반 4종 (§7.2–7.5 verbatim — 모델 태그는 §0.3 관례 적용)

```go
type ProviderConnection struct { // §7.2 verbatim
    ID             uint      `json:"id" gorm:"primaryKey"`
    UID            string    `json:"uid" gorm:"size:64;not null;uniqueIndex"`
    ProviderType   string    `json:"providerType" gorm:"size:64;not null;index"`
    Name           string    `json:"name" gorm:"size:128;not null"`
    Endpoint       string    `json:"endpoint" gorm:"size:255;not null"`
    GatewayID      *uint     `json:"gatewayId" gorm:"index"` // legacy AssetGateway FK until AccessRoute (M2)
    TLSProfile     contract.JSONMap `json:"-" gorm:"serializer:json;type:text"` // CA/verify posture; no secret material
    ConfigJSON     contract.JSONMap `json:"configJson" gorm:"serializer:json;type:text"`
    Status         string    `json:"status" gorm:"size:32;index"`
    Version        string    `json:"version" gorm:"size:64"`
    CapabilityHash string    `json:"capabilityHash" gorm:"size:128"`
    LastHealthAt   *time.Time `json:"lastHealthAt"`
    CreatedAt      time.Time `json:"createTime"`
    UpdatedAt      time.Time `json:"updateTime"`
} // TableName "provider_connection" — 이하 전 모델 동일 관례

type ProviderContext struct { // §7.3 verbatim — ParentID 없음(§7.3 명시)
    ID           uint
    UID          string `gorm:"size:64;not null;uniqueIndex"`
    ConnectionID uint   `gorm:"not null;index"`
    Kind         string `gorm:"size:64;not null"` // contract.ProviderContextKinds
    ExternalID   string `gorm:"size:255;index"`
    Name         string `gorm:"size:128"`
    Status       string `gorm:"size:32"`
    MetadataJSON contract.JSONMap `gorm:"serializer:json;type:text"`
    CreatedAt    time.Time
    UpdatedAt    time.Time
}

type ProviderCredentialBinding struct { // §7.4 verbatim
    ID                   uint
    ProviderConnectionID uint   `gorm:"not null;index"`
    ProviderContextID    *uint  `gorm:"index"`
    Purpose              string `gorm:"size:32;not null;index"` // inventory, operations, billing, console, monitoring, backup (§7.4 주석)
    SecretRefID          uint   `gorm:"not null;index"`
    Status               string `gorm:"size:32"`
    LastValidatedAt      *time.Time
}

type SecretRef struct { // §7.5 verbatim — Ciphertext는 항상 v2 envelope(§4.2; Step 4 완료로 legacy 폐기)
    ID         uint
    UID        string `gorm:"size:64;not null;uniqueIndex"`
    Backend    string `gorm:"size:32;not null;default:internal"` // internal (M1); vault etc. deferred
    Path       string `gorm:"size:255"`
    Version    string `gorm:"size:64"`
    KeyID      string `gorm:"size:64"`  // v2 envelope key id (§4.2)
    Ciphertext string `gorm:"type:text;not null"`
    RotatedAt  *time.Time
    CreatedAt  time.Time
    UpdatedAt  time.Time
}
```

`TLSProfile`/`ConfigJSON`의 json 태그 — `ConfigJSON`은 API 노출 대상이나 시크릿 무결 결계 주석. 시크릿 값 소재는 ConfigFieldSpec.Secret=true 규약(Phase 0)으로 SecretRef UID만 허용.

### 3.3 인벤토리 4종 (§8.1·§8.2·§8.4·§9.1 verbatim)

```go
type InfraResource struct { // §8.1 verbatim
    ID             uint
    UID            string `gorm:"size:64;not null;uniqueIndex"`
    ContextID      uint   `gorm:"not null;uniqueIndex:uq_infra_resource_identity,priority:1"`
    Kind           string `gorm:"size:64;not null;uniqueIndex:uq_infra_resource_identity,priority:2"` // contract.IsKnownResourceKind
    Subtype        string `gorm:"size:128"`
    ExternalID     string `gorm:"size:255;index"`
    ExternalURN    string `gorm:"size:512;not null;uniqueIndex:uq_infra_resource_identity,priority:3"`
    Name           string `gorm:"size:255"`
    DisplayName    string `gorm:"size:255"`
    LifecycleState string `gorm:"size:32"`
    HealthState    string `gorm:"size:32"`
    ManagedState   string `gorm:"size:32;default:discovered"` // discovered, imported, managed, orphaned, tombstoned
    LabelsJSON     contract.JSONMap `gorm:"serializer:json;type:text"`
    FirstSeenAt    time.Time
    LastSeenAt     time.Time
    DeletedAt      *time.Time `gorm:"index"`
}
// 신원 규칙(§8.1): UNIQUE(provider_context_id, kind, external_urn) — r2 C: GORM 태그 합성
// uniqueIndex(uq_infra_resource_identity)로 표현한다([실측 E-3a] 태그 합성 유니크는 재
// AutoMigrate에서 생존). raw DDL 인덱스는 GORM 계약상 보존이 보장되지 않아(§1 J6 경고)
// 원천에서 배제한다.

type ResourceObservation struct { // §8.2 verbatim
    ID                uint
    ResourceID        uint   `gorm:"not null;index:idx_obs_res_time"`
    GenerationUID     string `gorm:"size:64;not null;index"`
    ObservationHash   string `gorm:"size:128"`
    NormalizerVersion string `gorm:"size:64"`
    NormalizedJSON    contract.JSONMap `gorm:"serializer:json;type:text"`
    RawJSON           contract.JSONMap `gorm:"serializer:json;type:text"`
    ObservedAt        time.Time `gorm:"index:idx_obs_res_time,sort:desc"` // (resource_id, observed_at DESC) §8.2
}

type InfraRelationship struct { // §8.4 verbatim — FK to infra_resource, cross-context 지원
    ID             uint
    FromResourceID uint `gorm:"not null;index"`
    ToResourceID   uint `gorm:"not null;index"`
    Type           string `gorm:"size:64;not null"` // runs_on, backed_by, deployed_to, points_to (§8.4 예시)
    Source         string `gorm:"size:32;not null"` // discovery, user, policy, application_binding
    GenerationUID  string `gorm:"size:64;not null;index"`
    AttributesJSON contract.JSONMap `gorm:"serializer:json;type:text"`
    LastSeenAt     time.Time
}

type InventorySyncRun struct { // §9.1 verbatim
    ID           uint
    UID          string `gorm:"size:64;not null;uniqueIndex"`
    ConnectionID uint   `gorm:"not null;index"`
    ContextID    uint   `gorm:"not null;index"`
    Mode         string `gorm:"size:32;not null"` // full, incremental, targeted
    Status       string `gorm:"size:32;not null;index"`
    Cursor       string `gorm:"type:text"`
    SeenCount    int
    CreatedCount int
    UpdatedCount int
    MissingCount int
    ErrorCode    string `gorm:"size:128"`
    StartedAt    time.Time
    CommittedAt  *time.Time
    FinishedAt   *time.Time
}
```

### 3.4 태스크 3종 (§13.1–13.5 조립형 — 필드별 근거 명시, A3)

```go
type ProviderTask struct { // §13.1 "the unit of work + lease column + approval columns"
    ID                uint
    UID               string `gorm:"size:64;not null;uniqueIndex"`          // §16.2 GET /tasks/{uid}
    OperationName     string `gorm:"size:128;not null"`                      // §10.2 def 이름
    OperationVersion  string `gorm:"size:64"`
    ResourceUID       string `gorm:"size:64;index"`                          // §13.4 유일성 대상
    PayloadJSON       contract.JSONMap `gorm:"serializer:json;type:text"`
    Status            string `gorm:"size:32;not null;index"`                 // §13.5 상태머신
    AttemptCount      int    `gorm:"default:0"`                              // §13.2 claim 증가
    MaxAttempts       int    `gorm:"default:1"`                              // §13.2 "when attempts remain" 판정 (def.RetryPolicy 스냅샷)
    NextAttemptAt     *time.Time `gorm:"index"`                              // §13.2 claim 조건
    LeaseExpiresAt    *time.Time                                            // §13.2 리스
    Version           int    `gorm:"default:0"`                              // §13.4 낙관적 동시성 — 모든 쓰기 WHERE version=?
    CancelRequested   bool   `gorm:"default:false"`                          // §13.4 "requested via status flag"
    ErrorCode         string `gorm:"size:128"`                               // §13.2 'lease_expired' / §13.4 'resource_busy'
    ErrorMessage      string `gorm:"type:text"`
    CallTimeoutSeconds int   `gorm:"default:0"`                              // §13.4 provider call timeout (per attempt)
    DeadlineAt        *time.Time                                             // §13.4 task deadline (overall)
    // step0003 (PR 18) Expand:
    RequiresApproval  bool   `gorm:"default:false"`                          // §13.3 전이 판단 스냅샷
    ApprovalStatus    string `gorm:"size:32;default:not_required;index"`     // §13.3 OpsJob 어휘 (model/ops.go:538 관례) — 값: not_required/approved/rejected.
                                                // r2 A: 'rejected'는 이 컬럼의 값·task_event 타입 어휘로만 존재하며
                                                // provider_task.Status 종단 집합(4종)에 속하지 않는다.
    Approver          string `gorm:"size:128"`                               // §13.3
    ApprovalAt        *time.Time                                             // §13.3
    IdempotencyKey    *string `gorm:"size:128;uniqueIndex"`                  // §13.4 — NULL 다수 허용(MySQL/sqlite 유니크 관례)
    StartedAt         *time.Time                                             // §18.2 claim→terminal 측정
    FinishedAt        *time.Time
    CreatedAt         time.Time
    UpdatedAt         time.Time
}
// step0003 raw DDL: 생성 컬럼 active_flag — §13.4 "generated column active_flag = 1 while
// status is non-terminal, unique index (resource_uid, active_flag) where active" 의 MySQL/sqlite 실구현:
//   active_flag TINYINT GENERATED ALWAYS AS
//     (CASE WHEN status IN ('succeeded','failed','timed_out','cancelled') THEN NULL ELSE 1 END) STORED
//   UNIQUE KEY uq_provider_task_resource_active (resource_uid, active_flag)
// 종단 행은 NULL → 유니크에서 제외(MySQL 부분 인덱스 미지원의 표준 우회, A5 — [실측 R7]
// 단일 활성·복수 종단·종단 전이(UPDATE) 후 재활성 전체 불변식이 glebarez sqlite에서 작동 확인).
// CASE 목록 4종이 provider_task.Status 종단 집합 그 자체다(r2 A — §6 T24와 동치).
//
// r2 C 스텝 계약: active_flag·조합 유니크는 모델 태그로 표현 불가 — 생성 컬럼은 모델 필드가
// 아니며 ResourceUID에 단독 유니크 태그를 붙이면 resource_uid 전역 유니크로 왜곡된다[실측].
// 따라서 step0003 raw DDL이 생성하고, provider_task를 건드리는 이후 모든 스텝은 같은 raw DDL을
// 존재 검사 후 재적용하는 postlude를 의무 실행한다(W-5). 존재 단얫은 T31 — sqlite_master·
// information_schema(인덱스)와 pragma_table_xinfo·information_schema.columns(생성 컬럼은
// pragma_table_info에 미표시, [실측 E-3h]).
// 빈 resource_uid 태스크는 유니크 우회 수단이 되므로 Submit이 ''를 하드 거부한다(T-5) —
// 컬럼 계약은 not null(빈 문자열 허용, A3), 엔진 계약이 상류에서 봉쇄한다.

type TaskAttempt struct { // §13.1 "one row per execution attempt (worker, timing, error)"
    ID           uint
    TaskID       uint   `gorm:"not null;index:idx_attempt_task_no,unique"`
    AttemptNo    int    `gorm:"index:idx_attempt_task_no,unique"` // (task, no) 1회성 보장
    WorkerID     string `gorm:"size:128"`
    HandleRef    string `gorm:"size:255"` // provider-네이티브 핸들(예: UPID, §14.3)
    StartedAt    time.Time
    FinishedAt   *time.Time
    ErrorCode    string `gorm:"size:128"`
    ErrorMessage string `gorm:"type:text"`
    CreatedAt    time.Time
}

type TaskEvent struct { // §13.1 "append-only state/event log, written transactionally" (D5)
    ID         uint64
    TaskID     uint  `gorm:"not null;index"`
    AttemptNo  int
    Type       string `gorm:"size:64;not null"` // state.go 어휘(A4): created, approved, rejected,
                                                // claimed, attempt_failed, reaper_requeued(§13.2 verbatim),
                                                // succeeded, failed, timed_out, cancel_requested, cancelled
    Actor      string `gorm:"size:128"` // 승인자 등
    DataJSON   contract.JSONMap `gorm:"serializer:json;type:text"`
    At         time.Time `gorm:"index"`
}
```

### 3.5 시크릿 브로커 (`internal/infra/secrets`)

```go
// Broker resolves purpose-scoped credential material from SecretRef rows.
// M1 internal backend: Ciphertext is always a v2 envelope (§4.2); the broker
// reads through util.DecryptSecretV2 only — the same implementation the
// migration tool used (§4.3). Results are never cached (short-lived = a
// per-call view, §1 J7). Secrets are never serialized through model JSON.
type Broker struct { db *gorm.DB }

func NewBroker(db *gorm.DB) *Broker

type ResolvedSecret struct {
    SecretRefUID string
    Purpose      string // §7.4 어휘로 정합 검사된 값
    Value        string // 평문 material — 로깅·직렬화 금지
}

// Resolve finds the binding for (connectionUID, purpose), loads the bound
// SecretRef, decrypts, and returns one-time material. Purpose mismatch and
// v2-envelope parse failure are hard errors (UNKNOWN never falls through,
// §4.3).
func (b *Broker) Resolve(ctx context.Context, connectionUID, purpose string) (ResolvedSecret, error)
```

**ConnectionView 자격증명 전달 (r2 E — phase0 계약 이행, PR 16 편입)**: `contract/adapter.go:36`의 선언("ConnectionView 필드 확장(브로커 목적 자격증명 등)은 Phase 1 PR 16")을 다음 확장으로 이행한다:

```go
// contract.ConnectionView — r2: Material 필드 1건 추가(나머지 불변).
type ConnectionView struct {
    ProviderType string
    Endpoint     string
    Config       JSONMap
    // Material — 브로커가 해석한 목적 스코프 자재. 키는 §7.4 목적 어휘(M1은
    // "operations" 1건). 평문이므로 로그·직렬화 금지. contract stdlib 순수성
    // 유지를 위해 타입은 map[string]string이다(secrets 패키지 import 불가).
    Material map[string]string
}
```

엔진 실행 경로(§3.6 T-7)가 실행 체인 조인으로 얻은 connection UID로 `Broker.Resolve(…, "operations")`를 호출해 이 필드를 채운다 — Phase 3 실행 API가 자격증명 없는 ConnectionView로 어댑터를 부르는 구멍을 원천 차단한다(판정 근거: 필드 확장 1건의 비용으로 Phase 3 실행 경로 확보 — Phase 1 게이트 소비자인 fake는 Config만으로 동작하므로 Phase 1 테스트에는 영향 없음).

목적 어휘(`inventory, operations, billing, console, monitoring, backup`)은 `contract` 패키지 상수로 승격(§3.6 확장에 포함).

### 3.6 태스크 엔진 (`internal/tasks`) — 시그니처 계약

```go
type Config struct {
    WorkerID       string        // 식별자(태스크 시도 기록)
    PollInterval   time.Duration // 기본 2s (§13.1 "default interval 2s, configurable")
    LeaseSeconds   int           // 클레임 리스
    ReaperGrace    time.Duration // §13.2 "lease_expires_at < NOW() - grace"
}

type Engine struct { /* db, registry, cfg */ }

func NewEngine(db *gorm.DB, reg *registry.Registry, cfg Config) *Engine

// 제출 (PR 17 기본형 / PR 18 멱등강화)
type SubmitInput struct {          // r2 T-10: RequiresApproval 플래그 제거 — 정준원천은
    OperationName    string        // registry.Operation이 스냅샷한 OperationDefinition이다.
    OperationVersion string
    ResourceUID      string        // '' 는 하드 거부(T-5) — active_flag 유니크 우회 봉쇄
    Payload          contract.JSONMap
    IdempotencyKey   string // "" 이면 미설정(단일 실행)
}
// Submit은 ①레지스트리에서 OperationDefinition 조회 — RequiresApproval·RetryPolicy.MaxAttempts를
// 태스크에 스냅샷(정의 미등록 = 하드 에러, 재시도 없음) ②빈 ResourceUID 거부(T-5) ③상태 planned로
// 생성, 스냅샷한 RequiresApproval이면 awaiting_approval로 전이(§13.3) ④유니크 충돌 시
// classifyUniqueViolation(T-6)으로 제약 판별 — idempotency 충돌이면 기존 task 반환 +
// replayed=true(§13.4, N6), resource 활성 충돌이면 resource_busy 하드 에러(N11). 폴백 없음.
func (e *Engine) Submit(ctx context.Context, in SubmitInput) (model.ProviderTask, replayed bool, err error)

// T-6 제약 판별 — dialect별 유니크 위반 에러를 인덱스명으로 매핑:
//   sqlite "UNIQUE constraint failed: <table>.<col,…>"[실측: 컬럼명 포함 확인]·
//   MySQL 1062 "Duplicate entry … for key 'uq_…'".
//   uq_provider_task_idempotency        → 멱등 재생 경로
//   uq_provider_task_resource_active    → resource_busy
//   uq_infra_resource_identity          → 리소스 신원 충돌(Phase 2 디스커버리 소관 — Phase 1 엔진 비발화)
func classifyUniqueViolation(err error) (indexName string, ok bool)

// 승인 (PR 18)
func (e *Engine) Approve(ctx context.Context, taskUID, approver string) error // awaiting_approval → queued
func (e *Engine) Reject(ctx context.Context, taskUID, approver string) error  // awaiting_approval → cancelled(종단, §13.3 "rejection is terminal") — ApprovalStatus='rejected'·task_event 'rejected' 기록. Status 종단은 cancelled 4종 집합 유지(r2 A)

// 클레임·실행 (PR 17) — §13.2의 단일 문장 CAS를 GORM Updates+RowsAffected==1로:
//   UPDATE provider_task SET status='running', lease_expires_at=?, attempt_count=attempt_count+1
//   WHERE id=? AND status='queued' AND next_attempt_at<=?  (+ version CAS, §13.4)
// 비교 시각은 Go-side time.Now() 파라미터 바인딩이다(sqlite는 now() 함수 부재 — T-10).
// TaskAttempt 행은 이 클레임 트랜잭션 안에서 생성한다(attempt_no = 증가된 attempt_count, T-10).
// 리스 하한 계약(T-8): LeaseExpiresAt = max(cfg.LeaseSeconds, CallTimeoutSeconds) + ReaperGrace —
// 실행·폴 완료 전에 리스가 만료하면 리퍼 재큐로 이중 실행이 열린다.
type ClaimedTask struct { Task model.ProviderTask; Attempt model.TaskAttempt }
func (e *Engine) ClaimNext(ctx context.Context) (*ClaimedTask, bool, error)

// 종단 처리 — 상태 전이·attempt 종결·task_event가 한 트랜잭션에 커밋(D5).
func (e *Engine) Complete(ctx context.Context, claim *ClaimedTask, detail contract.JSONMap) error // → succeeded
func (e *Engine) Fail(ctx context.Context, claim *ClaimedTask, code, msg string) error
//   → 재시도 잔여: failed가 아니라 queued 복귀(§13.5) — 재시도 판정·status='queued'·
//   NextAttemptAt 백오프·task_event(attempt_failed)는 **단일 트랜잭션**(T-9).
//   소진: failed 종단.

// 폴러·리퍼 (PR 17)
func (e *Engine) Start(ctx context.Context)  // ctx 취소 시 고루틴 정리 (프로덕션 미배선, A6)
func (e *Engine) Stop()                      // r2 F-N: Start 대칭 정지 — 폴러·리퍼 고루틴 대기·회수 후
                                              // 반환(누수는 T39로 단얫). RunOnce/ReapOnce 진행 중이면 완료 후 정지.
func (e *Engine) RunOnce(ctx context.Context) (processed bool, err error)  // 폴러 1사이클 결정적 진입점
func (e *Engine) ReapOnce(ctx context.Context) (requeued int, err error)   // 리퍼 1사이클 결정적 진입점
```

**실행기 결합 (T-7/T-8, r2 G)**: `RunOnce`는 클레임 → **실행 체인 조인** `resource_uid → infra_resource(uid) → context_id → provider_context → connection_id → provider_connection → ProviderType`(soft-delete 리소스는 기본 스코프에서 제외되어 조회 누락 = `unknown resource` 하드 에러, 재시도 없음) → 레지스트리에서 operation 정의·capability 조회 → `OperationRequest.ResourceURN`은 엔진이 `infra_resource.external_urn`으로 조립(**UID→URN 변환 책임은 엔진**) → 어댑터 `OperationExecutor.Execute` → `OperationStatus.State` 매핑(동기 완료/실패). **비동기(ProviderRef 넌널)는 같은 사이클에 `TaskPoller.Poll` 1회 시도 후 Running이면 태스크 running 유지 — 이후 매 폴 사이클에서 Poll하며, Poll은 새 attempt를 만들지 않는다**(attempt는 클레임 tx에서 1회 생성, §14.3 UPID 이중 모드 — 널 핸들은 동기 완료). 동기 완료가 보장되기 전까지 활성 리스의 만료 시각은 해당 시도의 타임아웃 이상이어야 한다(클레임 리스 하한 계약, T-8). 어댑터 선택은 provider 유형 스위치 없이 레지스트리 조회만으로(arch rule 1).

### 3.7 capability→인터페이스 매핑 표 (Phase 0 §13 인계 1 이행 — registry V5 확장)

`contract/capability.go`에 인터페이스 종 어휘와 매핑 표를 추가하고, `registry.RegisterCapabilities`가 이 표로 검증한다(선언 capability의 필수 인터페이스를 어댑터가 구현했는지 타입 단언). ReadOnly→인터페이스 이분법은 사용하지 않는다(phase0 r2 V5 판정 승계 — `cost.read`·`console.web_terminal` 반례).

| Capability (§10.1) | 필수 인터페이스 | 선택 인터페이스 | 근거·비고 |
|---|---|---|---|
| `inventory.full` | Discoverer | — | §9 동기화가 Discoverer 페이징 소비 |
| `inventory.incremental` | Discoverer | — | 동일(커서, §9.1) |
| `orchestration.kubernetes.read` | Discoverer | — | Phase 2 k8s read-only 슬라이스 = 디스커버리(A11) |
| `orchestration.kubernetes.apply` | OperationExecutor | TaskPoller, TaskCanceller | §14.3 UPID — 핸들 반환 시 폴 시도, 널 핸들은 동기 완료(이중 모드 r2.2) |
| `compute.vm.read` | Discoverer | — | Phase 4 클라우드 인벤토리 |
| `cost.read` | (없음 — 외부 동기) | — | §3.2 row 18: finops는 기존 스케줄러 유지 — 어댑터 인터페이스 없음 |
| `console.web_terminal` | (이연 — ConsoleBroker M2+) | — | §11 "M2+, with the agent ADR" — 미선언 |

`cost.read`·`console.web_terminal`의 "(없음)" 매핑은 **검증 제외**가 아니라 **매핑 표의 명시적 값**이다 — 이 capability를 선언하는 어댑터는 인터페이스 검사 없이 통과하되, 표에 근거 주석이 붙는다. fake는 Phase 1에서 `inventory.full`+`orchestration.kubernetes.apply`(Discoverer+OperationExecutor 구현)를 선언해 V5가 실제 매핑 표로 발화하는 경로를 만든다(§3.8).

구현 소속 (r2 D·F-O): 매핑 표는 `contract/capability.go`(수정)에 두고, 검증은 `registry/registry.go`(수정)의 `RegisterCapabilities` + `registry/registry_test.go`(수정) 케이스가 담당한다 — 전부 **PR 17 Phase T4** 소속(§2·§12). 매핑 표를 필요로 하는 시점은 엔진 코어(T2)가 아니라 fake capability 선언·`RegisterCapabilities` 검증이 처음 발화하는 T4다.

### 3.8 fake 어댑터 확장 (Phase 0 등록 전용 형상 → 실행 형상)

```go
// adapter/fake: BaseAdapter(기존) + 아래 구현 추가 — Phase 1 게이트 ④의 실행 주체.
// Discover: 고정 시드 인메모리 리소스 페이지(orchestration.* 종 2~3종, NextCursor 종결)
// Execute: 호출 카운트를 내부 원자 카운터로 기록(N6 "실행 1회" 단언의 근거) —
//          Payload["failAttempt"]==n 이면 n회째 시도에서 오류(재시도 경로 시뮬레이션)
//          Payload["async"]==true 이면 ProviderRef를 반환해 Poll 경로 강제(UPID 이중 모드)
// Poll: Execute가 남긴 핸들 상태를 반환 — succeeded/failed/running
```

`Execute` 카운터가 "provider 이중에서 실행 횟수 단언"(§23.4 N6 — "execution count asserted at the provider double")의 구현체다.

### 3.9 OperationStatus.State 확정 (Phase 0 인계 — "종단 상태는 Phase 1 폴러 설계 시점에 확정")

`contract/adapter.go`의 `OperationStatus.State` 주석이 가리킨 확정: 상수 3종 `OperationStateRunning` / `OperationStateSucceeded` / `OperationStateFailed` + `IsTerminalOperationState(state) bool`. 태스크 상태머신(§13.5 9종)과의 매핑은 엔진이 소유: Succeeded→succeeded, Failed→failed(재시도 정책 적용), Running→running 유지(다음 폴 시도 = 새 attempt, §13.5 "Polling collapses into Running"). `timed_out`은 엔진이 DeadlineAt/CallTimeout으로 파생(어댑터가 보고하지 않음). 취소(`cancelling`/`cancelled`)는 태스크 계층 상태 — 어댑터 상태 아님(TaskCanceller가 별도 인터페이스).

---

## 4. 임계경로 식별 — core-xhigh 대상 판정

§5 라우터 기준("시스템 전체가 의존하는 코어 모듈·정확도가 곧 전체 품질인 직렬 경로") 적용:

| 대상 | 판정 | 근거 |
|---|---|---|
| PR 17 T2/T3 — 엔진 코어(클레임 CAS·상태머신·동일 트랜잭션 이벤트)·폴러/리퍼 루프 | **core-xhigh 단독 직렬** | §2.8 고아 running 결함의 직접 수정. 파티션·이중 실행·유실은 곧 인프라 사고. D1/D4/D5 수용 기준이 이 모듈에 집중 |
| PR 18 전체 — 멱등·승인·리소스 유일성 | **core-xhigh 단독 직렬** | 동시 제출 경쟁·유니크 인덱스 의존 — 미세 오차가 이중 변이로 직결(§13.4, N6/N11) |
| PR 19 전체 — 게이트 스위트 | **core-xhigh 단독 직렬** | 게이트 4조건 그 자체. 엔진과 동일 리스크 등급 |
| PR 15 러너 | 위험 비례(implement-xhigh 1스폰) | dirty·락 논리는 중요하나 게이트 직접 대상 아님. 단 schema_migration은 이후 전부의 기반이므로 PR 16 전 선행 필수 |
| PR 16 모델 8종·브로커 | 위험 비례(implement-med~xhigh) | 필드 verbatim 전사 + 브로커 목적 게이트 — 산수적 작업. 브로커 오류는 치명적이나 표면 좁음 |
| fake 확장(PR 17 T4) | 위험 비례 | 테스트 자산 — 게이트의 측정 기구이므로 정확성 요구는 높으나 프로덕션 무접촉 |

직렬 순서(강제): **PR 15 → PR 16(M1→M2→M3) → PR 17(T1→T2→T3→T4) → PR 18 → PR 19**. 병렬 가능: PR 16 M1(모델)과 PR 17 T1(태스크 모델)은 파일 무교차하나 마이그레이션 버전 직렬성(0001→0002)과 러너 대기(스텝 등록은 러너 머지 후)때문에 병렬 이득이 없다 — 직렬 운영 권고. 유일한 병렬 창: PR 19 스위트의 케이스 골격(테스트 함수 시그니처·시드 헬퍼)은 PR 18 리뷰 중 사전 작성 가능하나 실행은 PR 18 머지 후(TDD라면 애초에 PR 17/18 단계에서 레드 상태로 존재 — §6 참조). r2 F-O: r1의 "Phase A–D" 라벨을 §5의 실제 라벨(M·T)로 정정했다.

---

## 5. Phase 분해 (구현 단위 ≤5파일)와 작업 순서

### PR 15 (5파일 → 1 Phase)

- **Phase R — 러너 (5파일)**: `runner.go`, `migrations.go`, `step0000_bootstrap.go`, `runner_test.go`, `main.go`(수정). 의존: 없음. 검증: `go test ./internal/infra/migrate/` + `go build ./...`.

### PR 16 (9파일 — 신규 7+수정 2 → 3 Phase)

- **Phase M1 — 프로바이더 모델+러너 스텝 기반 (5파일)**: `model/provider.go`, `model/secretref.go`, `step0001_infra_foundation.go`(기반 4종 — r2 C), `model/model_test.go`, `migrations.go`(1줄). 검증: `go test ./internal/infra/...` — 0001 적용·재실행 no-op·유니크 단얫.
- **Phase M2 — 인벤토리 모델·스텝 완성 (3파일)**: `model/inventory.go`, `step0001_infra_foundation.go`(수정 — 8종 완성), `model_test.go` 확장. 의존: M1. 검증: 동일 + 신원 유니크(태그 합성, E-3a) 단얫. 스텝 완성은 PR 16 머지 시점 확정 — 이후 불변(W-4).
- **Phase M3 — 시크릿 브로커+ConnectionView 확장 (3파일)**: `secrets/broker.go`, `broker_test.go`, `contract/adapter.go`(수정 — Material 필드, r2 E). 의존: M1(SecretRef). 검증: `go test ./internal/infra/secrets/ ./internal/infra/contract/`.

### PR 17 (14파일 — 신규 7+수정 7 → 4 Phase)

- **Phase T1 — 태스크 모델 (4파일)**: `model/task.go`(PR 18 컬럼 제외), `step0002_tasks.go`, `migrations.go`(1줄), `model_test.go`(수정 — 태스크 3테이블 존재·TableName 단얫 + 12테이블 집합 동치 T41, r2 H). 검증: 마이그레이션 적용·(task, attempt_no) 유니크·T41 집합 동치.
- **Phase T2 — 엔진 코어 (3파일)**: `state.go`, `engine.go`, `engine_test.go`(T24–T27·T40). 의존: T1. 검증: 상태 전이표(종단 4종)·클레임 CAS·D5 트랜잭션 단얫.
- **Phase T3 — 폴러·리퍼 (2파일)**: `loops.go`, `reaper.go` (+`reaper` 단언은 T4·PR 19 스위트로). 의존: T2. 검증: `RunOnce`/`ReapOnce` 결정적 단얫.
- **Phase T4 — fake 실행 경로·레지스트리 매핑 (5파일)**: `fake.go`(수정), `fake_test.go`(수정), `contract/capability.go`(수정 — 매핑 표), `registry/registry.go`(수정 — 표 기반 검증), `registry/registry_test.go`(수정) + `engine_test.go` 확장. 의존: T2+T3. 검증: 매핑 표 검증 발화(fake 선언)→엔진 실행 회로(fake). r2 F-O: 매핑 표·registry 변경은 T4 소속(T2 아님).

### PR 18 (6파일 → 2 Phase)

- **Phase G1 — 스키마 가드 (3파일)**: `step0003_task_guards.go`, `migrations.go`(1줄), `model/task.go`(수정). 검증: 0002→0003 순차 적용·active_flag 유니크·idempotency 유니크.
- **Phase G2 — 엔진 가드 API (2파일+테스트)**: `idempotency.go`, `approval.go`, `guards_test.go`. 의존: G1. 검증: 멱등 재생·승인 전이·resource_busy.

### PR 19 (5파일 → 1 Phase)

- **Phase S — 게이트 스위트 (5파일)**: crash/collision/idempotency/uniqueness/cycle. 의존: PR 18 전체. 검증: `go test ./internal/tasks/ -race -count=1` — 본 Phase가 곧 게이트.

작업 순서: `R → M1 → M2 → M3 → T1 → T2 → T3 → T4 → G1 → G2 → S` (완전 직렬 — §4 판정). 각 Phase는 독립 검증 가능 단위(마이그레이션 버전·테스트 그룹으로 잔존 상태 판별).

---

## 6. 테스트 계약 (TDD — 각 PR의 레드→그린, §23)

신규 V2 패키지 4종(`infra/migrate`, `infra/model`, `infra/secrets`, `tasks` + 확장되는 `infra/contract`·`infra/registry`·`adapter/fake`)은 §23.3 **문장 커버리지 ≥80% per package**(claim 12 — r2 F-O 교차참조 정정). 엔진·시크릿은 커버리지만으로 부족 — 크래시/충돌 시나리오가 green이어야(§23.3).

| # | 테스트 | 검증 | PR |
|---|---|---|---|
| T14 | `TestRunnerAppliesPendingInOrder` | 스텝 3개 순차 적용·schema_migration에 version 전량 기록·재실행 no-op | 15 |
| T15 | `TestRunnerDirtyBlocksSubsequentRuns` | 스텝 실패 주입 → dirty=true 기록·재실행 거부(에러에 dirty 근거) | 15 |
| T16 | `TestRunnerLockIsAdvisory` | MySQL 경로: GET_LOCK 획득/반환 호출 단언(수제 ConnPool 래퍼·§3.1)+**런 전체 1개 `sql.Conn` 핀** 단얫(T-4/W-8)·sqlite: 우회 무오류 | 15 |
| T17 | `TestBootstrapIsIdempotent` | step0000 재실행 — CREATE TABLE IF NOT EXISTS no-op | 15 |
| T18 | `TestInfraModelsMatchSpecVocab` | 테이블명 8종·필드 존재·어휘 정합(Kind/Purpose/Backend default) | 16 |
| T19 | `TestResourceIdentityUnique` | (context,kind,external_urn) 중복 INSERT 거부 — sqlite·**태그 합성 유니크 경유**(r2 C·E-3a 실측: 재마이그레이션 후에도 단얫) | 16 |
| T20 | `TestObservationCompositeIndex` | (resource_id, observed_at DESC) 인덱스 존재(§8.2) | 16 |
| T21 | `TestBrokerResolvesPurposeScoped` | 바인딩 목적 정합 시 복호화 성공·PinSecretKeys로 왕복 | 16 |
| T22 | `TestBrokerRejectsPurposeMismatch` | 목적 불일치 → 하드 에러(폴백 없음) | 16 |
| T23 | `TestBrokerRejectsNonV2Ciphertext` | legacy/평문 형식 → 하드 에러(§4.3 UNKNOWN 정신) | 16 |
| T24 | `TestStateMachineMatchesSpec` | §13.5 전이 11종 허용·나머지 전부 거부·**종단 4종**(succeeded·failed·timed_out·cancelled) — `rejected`는 Status 종단이 아니라 ApprovalStatus 값·task_event 타입 어휘로만 존재(r2 A — §3.4 DDL CASE 목록·§3.6 Reject→cancelled·T29와 동치 확인 완료) | 17 |
| T25 | `TestClaimIsCAS` | 대기 아님/next_attempt_at 미도달/타 워커 선점 → RowsAffected 0 → 클레임 실패 | 17 |
| T26 | `TestCompleteCommitsAttemptAndEventAtomically` | 종단 시 attempt·event 동일 트랜잭션 — 실패 주입 시 event 단독 잔존 없음(D5) | 17 |
| T27 | `TestVersionCASRejectsStaleWrite` | version 불일치 쓰기 거부(D4) | 17 |
| T28 | `TestIdempotencyReplayReturnsSameTask` | 동일 키 2회 → 동일 UID·replayed=true(N6 전주기는 PR 19 S) | 18 |
| T29 | `TestApproveRejectTransitions` | planned→awaiting_approval→(approve)queued / (reject)cancelled 종단(§13.3) | 18 |
| T30 | `TestResourceUniquenessBlocksSecondActive` | 활성 태스크 존재 시 두 번째 제출 → `resource_busy` 즉시 실패(N11) | 18 |
| T31 | `TestActiveFlagGeneratedColumn` | 종단화 후 동일 리소스 재제출 허용(NULL 제외) + active_flag **생성 컬럼·조합 유니크 존재 단얫**(sqlite_master·`pragma_table_xinfo` / MySQL information_schema — 생성 컬럼은 pragma_table_info 미표시, [실측 E-3h]) + provider_task 후속 스텝 postlude 재적용 시 잔존 단얫(r2 C·W-5) | 18 |
| T32 | `TestCrashRecoveryReaperRequeues` (crash_suite) | 클레임 후 커밋 없는 소유자 소실 → 리스 만료 조작 → ReapOnce 재큐 + `reaper_requeued` 이벤트 + 유실 0 — **-race (게이트 ①, N7)** | 19 |
| T33 | `TestCrashRecoveryAttemptsExhausted` | 소진 시 failed `lease_expired` 종단 + 이벤트 | 19 |
| T34 | `TestParallelClaimsSingleWinner` (collision_suite) | 배리어 동시 출발 N클레이머×M라운드 → 매 라운드 승자 정확히 1·나머지 no-task·attempt_no 단일 — **-race 하 승자 1 불변식**(r2 H 목적 문구 정정). + **결정적 인터리빙 주입 대조군**: 채널 강제 인터리빙(읽기→판정→쓰기 경계 동기화) 하에서 가드 없는 SELECT→UPDATE 비원자 구현이 이중 클레임을 일으킴을 단얫 — 측정 기구가 비원자성을 잡아낸다는 반증 가능성 확보. MySQL 계층 2 포함(J5 의무) | 19 |
| T35 | `TestIdempotencySingleExecutionAtProvider` | 동일 키 2회 제출 → fake Execute 카운터 ==1(§23.4 N6 "provider 이중 단언") — **게이트 ②** | 19 |
| T36 | `TestResourceBusyUnderConcurrency` | 동시 동일 리소스 제출 → 정확히 1개 생성·나머지 resource_busy | 19 |
| T37 | `TestFakeFullCyclePlanApproveExecuteAudit` | fake 시드 — **connection+context+infra_resource 행 전제**(r2 G·T-7: 실행 체인 조인이 3계층을 통과해야 함) → Submit(awaiting approval) → Approve → RunOnce(Execute) → succeeded + task_event 시퀀스(created→approved→claimed→succeeded) 전량 단언 — **게이트 ④** | 19 |
| T38 | `TestCancelRequestedPersists` | 취소 플래그 지속·다음 경계에서 반영(§13.4) | 19 |
| T39 | `TestStartStopNoGoroutineLeak` (cycle_suite) | Start→정지 신호→**Stop()**(§3.6 F-N) 후 고루틴 수 기저 복귀(±여유)·RunOnce/ReapOnce 진입점 병행 무교찰 — r2 H 신설 | 19 |
| T40 | `TestSubmitRejectsEmptyResourceUID` | ResourceUID='' 제출 → 하드 에러·행 미생성(T-5) | 17 |
| T41 | `TestMigrationAppliesExactV2TableSet` | 전 스텝 적용 후 sqlite_master 테이블 집합이 12종과 **동치**(초과·누락 0 — 스펙 §3.2 나열과 A1 판정 집합) — r2 H: claim 4의 grep 카운트를 이 검증으로 교체 | 17 |

커버리지 측정: `go test ./internal/... -coverprofile` + `go tool cover -func` — 패키지별 수치 기재(claim 14).

---

## 7. 검증 요구 (claims — ④리뷰 주입용, 로컬 명령 형식)

1. `cd backend && go build ./...` exit 0.
2. `cd backend && go test ./internal/... -count=1` exit 0.
3. `cd backend && go test ./... -race -count=1` exit 0 (기존 회귀 0 포함 — CI backend-test 등가).
4. **테이블 집합 동치 (r2 H 교체)**: `cd backend && go test ./internal/infra/model/ -run TestMigrationAppliesExactV2TableSet -v -count=1` exit 0 — T41이 전 스텝 적용 DB의 sqlite_master 테이블 집합을 스펙 §3.2 나열 12종과 **동치(초과·누락 0)**로 단언한다("13" 선언 대비 1종 부족은 A1 판정). 태스크 3테이블(provider_task·task_attempt·task_event) 존재·TableName 단얫도 model_test가 같이 담당. 기존 grep 카운트 방식은 폐지.
5. **게이트 ①**: `cd backend && go test ./internal/tasks/ -race -count=1 -run TestCrashRecovery -v 2>&1 | grep -cE "^(=== RUN|--- PASS)"` ≥ 4 (T32·T33) **및** `go test ./internal/tasks/ -race -count=1 -run TestCrashRecovery` exit 0 — T32가 클레임 후 소유자 소실→리퍼 재큐→유실 0을 `-race` 하 단언. `-v` 라인 카운트는 vacuous pass(매칭 0회 통과) 봉쇄다(r2 H).
6. **게이트 ②**: `cd backend && go test ./internal/tasks/ -race -count=1 -run TestIdempotency -v 2>&1 | grep -cE "^(=== RUN|--- PASS)"` ≥ 4 (T28·T35) **및** exit 0 — T35가 동일 키→동일 task·fake 실행 카운터 1을 단언.
7. **게이트 ③**: `cd backend && go test ./internal/tasks/ -race -count=1 -run TestResource -v 2>&1 | grep -cE "^(=== RUN|--- PASS)"` ≥ 4 (T30·T36) **및** exit 0 — T30/T36이 두 번째 활성 mutation `resource_busy` 차단을 단언.
8. **게이트 ④**: `cd backend && go test ./internal/tasks/ -race -count=1 -run TestFakeFullCycle -v 2>&1 | grep -cE "^(=== RUN|--- PASS)"` ≥ 2 (T37 + 서브테스트) **및** exit 0 — T37이 plan→approve→execute→audit 이벤트 시퀀스를 단언.
9. `git diff main -- backend/main.go | grep "^+" | grep -v "^+++"` — 추가 라인이 `inframigrate.Run(db)` 호출·에러 처리·import 3종 이내(§1 J4 허용 범위 초과 없음).
10. `git diff main -- backend/go.mod backend/go.sum` — 출력 없음(신규 의존성 0, §10.2).
11. `git diff main --name-only | grep -v "^backend/internal/\|^backend/main.go\|^v2-phase1/"` — 출력 없음(수정·신규가 허용 경로 밖으로 나가지 않음 — v1 코드 무접촉, main.go 예외는 claim 9, r2 H: `v2-phase1/` 상태 파일 제외 패턴 추가).
12. `cd backend && go test ./internal/... -cover -count=1 2>&1 | grep "coverage:" | grep -v "no statements"` — 출력 각 행 ≥80.0%. 기계 판정 원형(r2 H): `go test ./internal/... -cover -count=1 2>&1 | grep "coverage:" | grep -v "no statements" | awk '{pct=$NF; gsub(/%/,"",pct); if (pct+0 < 80) print "FAIL", $2, pct}'` — 출력 없음이 합격. 대상 패키지: `infra/migrate`·`infra/secrets`·`tasks`·(확장되는)`infra/contract`·(수정되는)`infra/registry`(§23.3; `infra/model`은 상수 중심 분모 왜곡 시 수치+사유 기재).
13. ① `grep -o "inventory\.full\|inventory\.incremental\|orchestration\.kubernetes\.read\|orchestration\.kubernetes\.apply\|compute\.vm\.read\|cost\.read\|console\.web_terminal" backend/internal/infra/contract/capability.go | sort -u | wc -l` == 7 — 매핑 표가 M1 어휘 7종 전수를 다룸(인계 1 이행 — §3.7 표; **식별자 이름과 무결결합**, r2 H. 실측: 이 grep은 현재 파일에서도 7 — M1CapabilityVocabulary가 이미 7종을 포함한다. 따라서 ①은 파일 수준 커버리지 확인이고 매핑 표의 동작 검증은 ②가 담당한다) ② `cd backend && go test ./internal/infra/registry/ -count=1` exit 0 — 매핑 검증 케이스(`registry_test.go` 수정 — §2 PR17) 포함.
14. `cd backend && go test ./store/ ./router/ -count=1` exit 0 — v1 회귀 0(fixture·라우트 인벤토리 포함, §C1 정신).
15. `bash scripts/check-arch-boundary.sh` exit 0 — 신규 패키지 R1–R3 무발화(보호 밖 판정은 R5).
16. `grep -rn "NewEngine\|NewBroker\|\.RunOnce\|\.ReapOnce" backend/main.go backend/router/ backend/service/ backend/controller/` — 출력 없음(폴러·브로커 프로덕션 미배선, A6). r2 H(L-1): 토큰 한정형으로 확장 — 엔진·브로커 구성·진입점 토큰 자체가 없음을 잡고, v1 cron 기존 `.Start(` 호출(service/ 4곳 실측)의 오탐을 원천 차단한다.
17. `cd backend && go vet ./...` exit 0.
18. **계층 2 MySQL 실행(의무 — r2 B)**: `cd backend && OPS_TASKENGINE_MYSQL_DSN='<로컬 docker-compose MySQL DSN>' go test ./internal/infra/migrate/ ./internal/tasks/ -run TestMySQL -v -count=1` exit 0 — `=== RUN` ≥ 4(`TestMySQLMigrationSuite`·`TestMySQLParallelClaims`·`TestMySQLResourceUniqueness`·`TestMySQLIdempotencyReplay`, §1 J5). **실행 출력 전문을 tracking 이슈에 첨부**한다(§10). env 미설정 환경의 일반 판정(claims 2·3)은 이 claim의 대상이 아니다(스킵이 정상 동작). CI 잡 추가 없음(§0.8).

---

## 8. 보존 제약 (③구현 프롬프트에 verbatim 복사)

1. 기존 Go 소스·테스트 파일 중 수정 허용은 `backend/main.go`(V2 러너 호출 배선 — 신규 import 1 + 3–4줄 한정)과 **Phase 0 자산 6종**(`internal/infra/adapter/fake/fake.go`·`fake_test.go`의 실행 인터페이스 추가, `internal/infra/contract/capability.go`의 매핑 표 추가, `internal/infra/contract/adapter.go`의 ConnectionView.Material 1필드 추가, `internal/infra/registry/registry.go`·`registry_test.go`의 매핑 검증)뿐이다(r2 D — §2 배치표·§12 소유권 표와 동일 집합). `router/`·`controller/`·`service/`·`store/`·`model/`·`opdef/`·`internal/domain/`·`internal/testutil/`·`util/`은 무접촉.
2. v2 스펙·에픽·ADR 6종·`docs/architecture/README.md`·`docs/security/*`를 수정하지 않는다. 본 계획 산출은 `backend/internal/` 하위 신규 파일 + §1에서 열거한 수정만이다.
3. `main.go`에 폴러·리퍼(`Engine.Start`)·시크릿 브로커·레지스트리 프로덕션 조합을 배선하지 않는다 — Phase 1의 엔진 기동은 테스트뿐이다(가정 A6). 배선 허용은 마이그레이션 러너 호출 단 한 곳.
4. `backend/go.mod`·`go.sum`을 수정하지 않는다 — 신규 의존성 금지(§10.2 "No new runtime dependencies in M1"). UID는 crypto/rand 자체 구현으로 생성한다(A12).
5. 스펙 §7·§8·§9·§13이 정의한 필드·상태·전이를 임의로 추가·변경하지 않는다 — §3의 조립형(provider_task) 외 필드 확장은 금지하며, 구현 중 불가피한 이탈은 구현 보고서에 명시한다.
6. 시크릿 브로커는 `util.DecryptSecretV2`만 사용한다 — legacy envelope·평문 폴백 금지(§4.3 UNKNOWN 정신). 복호화된 material을 로그·JSON 직렬화·에러 메시지에 노출하지 않는다.
7. 태스크 엔진은 provider 유형명 분기를 갖지 않는다 — 어댑터 획득은 레지스트리 조회만(arch rule 1). 어댑터는 제어평면 테이블에 직접 쓰지 않는다(arch rule 2).
8. `store/migrate.go`의 AutoMigrate 리스트·`store/seed.go`·`store/migrate_fixture_test.go`·fixture 바이너리를 수정하지 않는다 — V2 12 테이블은 버전 관리 러너 스텝으로만 생성된다(J1/J2).
9. `scripts/check-arch-boundary.sh`와 `.github/workflows/*.yml`을 수정하지 않는다 — CI 잡 추가·변경 없음(로컬 검증 방침, §0.8). MySQL testcontainer 계층은 env-gated 테스트 코드로만(J5).
10. 상태 전이·멱등·유일성 검증이 없는 "통과"를 보고하지 않는다 — 게이트 4조건(claims 5–8)은 반드시 명시된 테스트 실행 출력으로 증명한다.
11. 한번 적용된 마이그레이션 스텝 정의를 수정·철회하지 않는다(W-4) — 스키마 변경은 항상 신규 버전 스텝(0004+)으로만 반영하며, provider_task를 건드리는 신규 스텝은 active_flag raw DDL을 존재 검사 후 재적용하는 postlude를 포함한다(W-5·§1 J6·§3.1).

---

## 9. 리스크 매트릭스 (발생 가능성 × 파급력) + 완화책

| # | 리스크 | 가능성 | 파급 | 완화 |
|---|---|---|---|---|
| R1 | MySQL 비트랜잭셔널 DDL — 스텝 중간 실패로 스키마 찢어짐(브리핑 M-3 인계) | 중 | 높음 | J6(스텝 내 AutoMigrate 멱등 + raw 최소화)·dirty 플래그로 재실행 차단·수동 해결 에러(자동 복구 시도 금지)·T15 |
| R2 | sqlite(테스트)와 MySQL(프로덕션)의 유니크·생성컬럼·인덱스 거동 차이 | 중 | 높음 | 생성컬럼·조합 유니크는 per-dialect raw DDL 이중 정의 + sqlite 계층 1 스위트·**MySQL 계층 2 스위트(의무 1회 실행, J5·claim 18)**로 각 dialect를 실측(r2 B — 문언을 실제 인도물과 정합화); A5 NULL 패턴은 sqlite 실측 완료·MySQL은 계층 2가 검증 |
| R3 | -race 하 폴러/리퍼 고루틴 타이밍 플레이크 | 중 | 중 | `RunOnce`/`ReapOnce` 결정적 진입점 — 테스트는 루프 타이밍에 의존하지 않는다. `Start`는 정지·누수 단언만 |
| R4 | 멱등·유일성의 동시 제출 경쟁(유니크 위반 vs 성공 경합) | 중 | 높음 | 제출을 단일 트랜잭션 + INSERT로 먼저 시도 → 유니크 위반 에러를 감지해 기존 행 재조회(정합 패턴)·T34/T36 병렬 -race |
| R5 | 신규 코어 패키지가 arch-boundary R1/R2 보호 밖(phase0 인계 2 미해결 — CORE_PACKAGES 확장은 R2 재설계 선행) | 중 | 낮음 | 본 계획 내 방어선: claim 15(무발화)·보존 제약 #7(엔진 무분기). R2 재설계는 §13 이계 |
| R6 | 게이트 ①이 SIGKILL 프로세스 크래시가 아닌 시뮬레이션이라는 이견 | 중 | 중 | §1 J5 판정 근거 문서화(게이트 문장의 검증 대상은 소유자 소실 회복)·MySQL 계층 2(의무)·Phase 3 크래시 주입 인계. 이견 잔존 시 검토에서 원문 크래시 주입으로 상향 — 최저비용 대응은 J5 F-B(자식 프로세스 SIGKILL·파일 sqlite·부모에서 리퍼 검증)로 기록(r2 H) |
| R7 | generated column(STORED)이 glebarez sqlite에서 파싱 실패 | 매우 낮음 | 높음 | **실측 완료(r2, 2026-09-06 — glebarez v1.11.0)**: STORED 생성 컬럼 + NULL 제외 조합 유니크의 전체 불변식(단일 활성 거부·복수 종단 허용·종단 전이 UPDATE 후 재활성 허용 — 저장값 재계산 포함) 작동 확인. T31이 회귀 방지. 대체 설계(트리거·엔진 수명 검사)는 발동 조건 소멸 |
| R8 | "12 vs 13" 카운트 불일치로 게이트 검수 혼란 | 낮음 | 낮음 | §0.1+A1로 근거 제시. 검토자는 §3.2 나열 목록(12) 기준으로 판정 |
| R9 | 인벤토리 4종 테이블이 소비자 없는 "dead tables"(§5.4 invalid states)로 보임 | 낮음 | 중 | §5.4가 Phase 1 상태("tables exist and are covered by tests, no production data")를 명시 허용 + Phase 1–2 연속 블록 계약. claim 4의 존재·단언 검증이 "dead" 반론의 반증 |
| R10 | 브로커·엔진이 Phase 2/3 요구(연결 체인 조회·폴링)와 미정합 | 중 | 중 | §3의 최소 형상 원칙 + 각 확장 시점 주석. 미정합 발견 시 Expand 마이그레이션(0004+)으로 해결 — 러너가 그 상황을 상수 비용으로 만든다 |
| R11 | 커버리지 분모 왜곡(model·상수 패키지) | 중 | 낮음 | claim 12 수치+사유 기재 방식(phase0 R7 관례 승계) |
| R12 | v2 러너 실패가 v1 기동을 차단한다(fail-closed — main.go `log.Fatalf`, §1 J4 배선의 직접 귀결) (r2 F·W-1) | 낮음 | 높음 | 의도된 설계 — 찢어진 스키마로 v1을 기동하는 것보다 정직한 실패다(R1 dirty와 동일 철학). 탈출구: main.go 배선 revert(§11 — 1커밋)가 1차, 긴급 운영 지속이 필요하면 env 스킵 1줄(`OPS_V2_MIGRATION=off` 러너 진입 전 검사) — 도입은 리뷰에서 확정 후 J4 배선 범위에 합산. dirty 해제 절차는 §11 |

절충 요약: (a) v1 AutoMigrate 공존(J1) — 두 체계 유지비 vs v1 무변경 안전성; (b) 시뮬레이션 크래시 테스트(J5) — CI 인프라 비용 vs 게이트 문자 그대로 재현; (c) 브로커 TTL 미구현(J7) — 외부 백엔드 부재 시 물리 TTL 무의미 vs §7.7 재진입 시 재개. 세 절충 전부 재진입 트리거와 함께 문서화했다.

---

## 10. CI 로컬 검증 방침 (저장소 정책 준수)

- **merge 판정은 로컬 명령만으로 완결** — 원격 CI 완료 대기 없음: `cd backend && go test ./... -race -count=1`(전체) + §7 claims 순차 실행. 이 조합이 CI `backend-test`(같은 명령)·`migration-test`(claim 14가 등가 커버)·`arch-boundary`(claim 15)의 로컬 등가다.
- v2-ci.yml은 무수정(보존 제약 #9) — 신규 패키지는 기존 잡의 와일드카드(`./...`)에 자동 편입된다.
- **계층 2 MySQL 실행(의무 1회 — r2 B)**: 게이트 종결 전 로컬 docker-compose mysql 서비스(`ops-admin-mysql`, A16) 대상 claim 18을 1회 실행하고 출력 전문을 tracking 이슈에 첨부한다 — CI 잡 추가 없음(§0.8·§1 J5).
- 게이트 4조건의 증거는 로컬 테스트 출력(테스트명·카운트 단언 + claims 5–8의 `-v` 라인 카운트)로 충분하며, 페이즈 종결 시 그 결과를 tracking 이슈 체크리스트에 링크한다(§21 Resume protocol).

---

## 11. 롤백 가능성 판정

- **PR 15**: main.go 3–4줄 revert + 신규 패키지 제거로 완전 복귀. schema_migration 테이블이 DB에 남을 수 있으나 소비자 부재 — 무해. 데이터 무접촉.
- **PR 16/17/18**: 전부 신규 테이블·패키지 — PR 단위 revert로 빌드·기동 원상복귀(폴러 미배선이므로 프로덕션 런타임 경로 변화 자체가 없다). 생성된 12 테이블은 rm 없이 잔존 가능( dead but empty — §5.4의 금지는 "행동 없는 스키마 병합"인데 Phase 1은 테이블 존재 자체가 게이트 산물이므로 revert 시 테이블 drop는 선택).
- **부분 롤백 (r2 F·W-3 정정)**: 롤백은 **역순 스택(19→18→17→16→15) 전제로만 성립**한다. r1의 "PR 16만 revert해도 0002/0003이 동작"은 스키마 객체 소유 논리로는 참이나 코드 의존으로는 성립하지 않는다 — 중간 생략 revert는 (a) `migrations.go`·`model/task.go`·`model_test.go`의 후속 커밋과 충돌하고 (b) PR 17 코드(엔진·테스트)가 PR 16 모델 패키지·0001 스텝에 의존해 빌드가 깨진다. 역순 스택에서 스텝의 스키마 객체 소유 덕분에 DB 잔존물은 빈 테이블뿐이다(drop 불요). 최신 1개 PR만 revert하는 경우(PR 19 등)는 성립한다. 게이트 불충족 상태 관리는 phase0 §11 관례(명시적 미충족으로 종결 금지).
- **dirty 상태 롤백**: 러너 dirty=true는 수동 개입 전제 — `schema_migration`에서 해당 version 행 삭제 + 찢어진 DDL 수동 정리 후 재실행(에러 메시지에 절차 기술). 프로덕션 스키마 손상 자동 복구 시도는 금지(R1).
- 판정: **전 PR 단위 완전 롤백 가능**(신규 테이블·미배선 고루틴·v1 무접촉의 구조적 결과). 유일한 프로덕션 지속물은 main.go 배선과 빈 12 테이블이다.

---

## 12. 파일 소유권 — 구현 스폰 분리 (충돌 방지)

직렬 운영 권고(§4)이므로 스폰 분리는 시간 분리가 기본이나, 병렬 시:

| 스폰 | 전용 소유 경로 | 접근 금지 |
|---|---|---|
| impl-P15 (러너) | `backend/internal/infra/migrate/{runner,migrations,step0000}.go`·`runner_test.go` + `backend/main.go` 배선 | model·secrets·tasks·fake·contract·registry |
| impl-P16 (모델+브로커) | `backend/internal/infra/model/{provider,secretref,inventory,model_test}.go`·`backend/internal/infra/secrets/**`·`contract/adapter.go`(Material 필드)·`step0001_*.go`·`migrations.go` 0001 라인 | runner 본체·main.go·tasks·fake·registry |
| impl-P17 (엔진) | `backend/internal/tasks/**`·`step0002_*.go`·`model/task.go`·`model/model_test.go`(태스크 단얫·T41)·`fake.go`·`fake_test.go`·`contract/capability.go`(매핑 표)·`registry/registry.go`·`registry_test.go` | migrate 러너 본체·model의 비태스크 파일·main.go |
| impl-P18 (가드) | `step0003_*.go`·`tasks/idempotency.go`·`tasks/approval.go`·`guards_test.go`·`model/task.go`(컬럼 반영) | 위와 동일 경계 |
| impl-P19 (스위트) | `backend/internal/tasks/*_suite_test.go` 5종 | 프로덕션 코드 전부(테스트만) |

교차 소유: `model/task.go`(P17 생성→P18 컬럼 반영)·`model/model_test.go`(P16 생성→P17 태스크 단얫 확장)·`migrations.go`(P15 생성→P16/17/18 각 1줄)·`fake.go`(P17 확장) — **직렬 순서로만 편집**되므로 동시 충돌 없음. `contract/capability.go`·`registry/registry.go`(+테스트)는 P17 **Phase T4** 소속이다(r2 F-O 정정: 매핑 표를 필요로 하는 것은 엔진 코어(T2)가 아니라 fake capability 선언·`RegisterCapabilities` 검증의 첫 발화(T4)다 — §3.7). 공통 무접촉: `v2-phase1/*.json`·스펙·스크립트·워크플로.

---

## 13. 산출 외 확인사항 (구현자에게 불요, 팀 리드 인계)

- **contract test harness 이연 (r2 E 판정)**: §23.1 contract 티어의 공용 harness(discovery paging·error taxonomy·rate-limit signaling·redaction 단얫)은 **Phase 2 PR 20 소관으로 이연**한다. 판정 근거: Phase 1의 유일한 소비자인 fake는 내장 테스트(`fake_test.go`)로 충분하고, harness 형상은 첫 실 어댑터(k8s)와 함께 성숙시키는 것이 비용이 낮다 — Phase 1에서 추상 harness를 먼저 만들면 소비자 1개(fake)에 과적합된 계층이 된다. tracking 인계 사항으로 기록한다.
- **인계 2 이월(phase0 §13 (b))**: R2 규칙 재설계 전까지 `internal/infra`·`internal/tasks`는 arch-boundary 보호 밖 — 재설계 완료 시 CORE_PACKAGES 확장과 함께 신규 코어를 R1/R2로 핀닝. 본 계획 claim 15는 무발화만 검증.
- **인계 3 이월(phase0 §13 인계 6)**: 신규 패키지 커버리지 ≥80% 래치와 contract 순수성의 상시 CI 핀닝 — 본 계획은 로컬 claims(12·15)로 대체.
- **Phase 3 인계**: 엔진 프로덕션 배선(main.go에 Engine.Start + graceful shutdown 편입)·첫 OperationDefinition(workload restart) 등록·plan/execute API. 배선 시점에 가정 A6 해소.
- **M2 인계**: v1 AutoMigrate의 versioned 전환 여부(J1 절충의 해소 시점) — 커트오버 runbook이 소유.
- **크래시 주입 e2e**: 게이트 ①의 프로세스 실크래시(SIGKILL) 버전은 Phase 3 Slice A "crash-injected run"(§20 Phase 3 게이트)과 통합.
- **테이블 카운트 오기(12 vs 13) 스펙 반영**: 본 계획은 스펙 무수정 원칙으로 A1 판정에 둔다. 차후 스펙 개정 시 정정 권고 — 검토 임의.
- 게이트 4조건의 로컬 증거(claims 5–8 출력)를 tracking issue #4 체크리스트에 링크해야 페이즈 종결이 성립한다(§21 Resume protocol).

---

## 14. 가정 명세 (불확실 요소의 명시적 처리)

- **A1 (테이블 수 판정)**: 스펙 §3.2의 선언 "13"에 대해 실제 나열은 12종(schema_migration 포함). 누락 후보는 §7.7 이연 목록 대조로 특정되지 않음. **12종이 M1 전부로 판정**하며 게이트 검수도 12종 기준. 스펙 오기 가능성이며 본 계획은 스펙을 수정하지 않는다.
- **A2 (PR 17/18의 스키마 분할)**: §21이 "17. 테이블+폴러+리퍼 / 18. 승인 컬럼+멱등+유일성"으로 분할 — provider_task는 0002에서 기본형 생성 후 0003(Expand)이 가드 컬럼을 추가한다. 러너의 순차 적용이 이 분할을 자연스럽게 지원한다.
- **A3 (provider_task 필드 조립)**: 스펙은 완전한 구조체 대신 산재 근거(§13.1–13.5·§16.2·§18.2)를 준다. §3.4의 필드별 근거가 조립의 전부이며, ProviderConnectionUID 등 실행 체인 조회로 유도 가능한 컬럼은 의도적으로 미포함(중복 저장 최소화 — Phase 3 필요 시 Expand).
- **A4 (task_event 타입 어휘)**: 스펙 명시는 `reaper_requeued`(§13.2) 단 하나. 나머지(created/approved/rejected/claimed/attempt_failed/succeeded/failed/timed_out/cancel_requested/cancelled)는 상태머신(§13.5)에서 도출한 계획 정의 어휘.
- **A5 (active_flag NULL 패턴)**: MySQL은 부분 인덱스 미지원 — "unique … where active"(§13.4)를 생성컬럼 종단 시 NULL + 조합 유니크로 구현(양쪽 DB 표준 우회). 스펙 문장의 "active_flag = 1 while non-terminal"을 NULL 3치로 치환한 구현 해석.
- **A6 (폴러 프로덕션 미배선)**: Phase 1의 엔진은 테스트에서만 기동한다. 근거: 제출 API 부재(§3.3 흡수 목록 — M1 변이는 k8s restart뿐이고 그 경로는 Phase 3)·§5.4 Phase 1 상태 정의·v1 무변경. Phase 3가 배선 소유.
- **A7 (크래시 게이트의 증명 수준)**: 게이트 ①의 "kill worker mid-task"는 소유자 소실(클레임 확보·종단 미커밋·리스 만료)의 결정적 시뮬레이션으로 증명한다(§1 J5). 프로세스 SIGKILL은 계층 2 옵션(J5 F-B)·Phase 3로 인계 — r2 B 이후 계층 2 자체는 의무지만 SIGKILL 실증은 그 범위 밖이다.
- **A8 (schema_migration 형상)**: version/name/dirty/applied_at 4컬럼 — §19이 요구하는 기능(버전 표·단일 러너·락·체크포인트)의 최소 형상. golang-migrate 등 외부 도구 도입은 신규 의존성 금지(보존 제약 #4)로 기각.
- **A9 (v1 AutoMigrate 유지)**: J1의 판정 — 스펙 §19 첫 문단의 비판은 "래너 없는 자동 마이그레이션" 전반으로 읽을 수 있으나, §20 Phase 1 게이트가 요구하는 것은 러너 존재와 12 테이블이며 v1 91 테이블의 전면 전환은 게이트 밖이다. M2 커트오버에서 재결정.
- **A10 (OperationStatus.State 3종 확정)**: running/succeeded/failed + 종단 판정 함수. Phase 0 계약의 "확정 시점" 이행이며 §13.5 "Polling collapses into Running" 근거. 취소·타임아웃은 태스크 계층 파생(§3.9).
- **A11 (capability→인터페이스 매핑값)**: §3.7 표의 `orchestration.kubernetes.read → Discoverer`(Phase 2 k8s 슬라이스가 디스커버리로 서빙)·`cost.read → 없음`(§3.2 row 18 기존 스케줄러)은 계획 판정 — Phase 2/4 계획이 확정 시점에 재검한다.
- **A12 (UID 생성)**: crypto/rand 16바이트 hex(32자) 자체 유틸 — `google/uuid`는 k8s 의존으로 바이너리에 있으나 direct 승격이 go.mod diff를 만들므로 무변경 원칙 우선(보존 제약 #4).
- **A13 (게이트 ④ "audit"의 수준)**: Phase 1의 audit은 task_event 감사 시퀀스(created→…→succeeded)로 해석 — OperationLog 확장 컬럼(§3.2 row 2)·§18.1 전체 감사 레코드는 Phase 3 PR 24("audit extension") 소관. Phase 1 게이트 문구의 "audit cycle"은 이벤트 로그 완결성으로 충족.
- **A14 (브로커 short-lived 해석)**: J7 — M1 internal 백엔드에서는 목적 게이트·비캐시·v2 전용이 "purpose-scoped, short-lived"의 구현 전부. 물리 TTL은 외부 백엔드(§7.7) 재진입 시.
- **A15 (advisory lock 구현)**: MySQL `GET_LOCK('ops-admin:v2-migrate', 0)` 획득 실패 시 기동 오류(다른 러너 실행 중) — 단일 인스턴스 배포에서는 경합이 없으므로 대기 없이 실패가 정직한 동작. sqlite(테스트)는 락 우회(단일 커넥션 in-memory는 경합이 존재하지 않음). 세션 스코프 한계는 런 전체 `sql.Conn` 1개 핀으로 해결한다(§3.1 T-4/W-8 — r2 F).
- **A16 (계층 2 실행 환경 — r2 B)**: 로컬 docker-compose mysql 서비스(`ops-admin-mysql`, MySQL 8.0 — 존재 실측 2026-09-06)를 대상으로 한다. compose가 mysql에 호스트 포트를 게시하지 않으므로 실행 시점에 동등 로컬 인스턴스(예: `docker run -p 127.0.0.1:3306 … mysql:8.0`) 또는 포트 노출 오버라이드를 허용한다. 테스트 전용 스키마(예: `ops_v2gate`)의 생성·해제는 계층 2 테스트가 스스로 관리한다(운영 스키마 `ops_admin` 무접촉).
- **A17 (E-3 실측 판정 — r2 C)**: 신원 유니크의 태그 합성 이동은 [E-3a] 생존 실측(2026-09-06, glebarez v1.11.0)이 근거다. raw 인덱스·raw 생성 컬럼의 재마이그레이션 보존은 [E-3c/d/g] 현재 버전에서 관찰되었으나 GORM/드라이버 계약이 아니므로 — postlude 재적용(W-5)·T31 존재 단얫은 보존이 무보장이라는 전제로 방어로 유지한다. 생성 컬럼 존재 검사는 `pragma_table_xinfo`·`information_schema.columns`로만 유효하다([E-3h] `pragma_table_info`는 hidden=3 미노출).
