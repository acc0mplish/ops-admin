# V2 Phase 4 계획 — Aliyun/Tencent 읽기 전용 어댑터 (Slice B) (r2)

작성: 2026-09-07 (r1) · **개정 r2: 2026-09-07** · 티어 L (자격 통합 이중 전환·"코어 분기 0" 기계 증명이 게이트 직결 — review-pr-xhigh 병행) · 앵커(실측 시점 HEAD): `a6ccfe1` (main, PR #32 merge)

## r2 개정 이력 (2렌즈 병합 리뷰 반영 — 완전성 + 기술 통합·위험)

r1의 판정 골격(**J1–J10·§0 실측·자격 blob(J4)·step0006 근거·이월 정합 §0.7**)은 무결 — 전부 유지. r2는 병합 리뷰 발견을 다음 5그룹으로 반영한다. 전 항목 아래 명시된 라인 번호·파일 수·명령은 2026-09-07 본 repo 실측값이다.

| 그룹 | 발견 | 반영 위치 |
|---|---|---|
| **A. finops 평문 재기입 차단** (2렌즈 수렴 — 최우선) | `SyncFinOpsAccountMonths` 말미가 `s.db.Save(&account)` 전체 행 저장(`integration_finops.go:411-414`) — r1 안으로 브로커 평문을 주입한 사본이 Save되면 **평문이 DB에 재기입**된다 | 판정 J5 재서술·§6 C2 검증·보존 제약 2·3·§11 롤백·가정 A7 |
| **B. 게이트 증명 기구 정정** | R2 v2가 코어 `_test.go`의 합법 식별자(목업·플레인트 테스트)를 오탐(현 스크립트 47행 `--include`만)·compose_test 등록 수 단얫(41-42행)이 2→4 갱신 필요·tasks 라우트 등록 위치 미명시로 exactly-four 단얫(164행) 위험·§0.7-1 근거 부정확 | J8 (d)·M2·M8·§0.7-1·§6·보존 제약 6 |
| **C. 자격·보안** | 자격 변경 시 링크 해제 전파 부재·secret-scan claim 부재·P-class 평문 잔존 미등재·E-2 env 게이트 부재·비로그 grep 1경로 | M4·M10·claim 13·14·§13-9·E-2·보존 제약 3 |
| **D. 운영·시퀀스** | Phase 2 게이트(09-09 종결 예정)가 compare/sync CLI를 사용 중 — 머지 순서 충돌·step0006 dirty runbook·interim 마킹 규격·서명 3중 병존·백필 오류 격리·PR 29 롤백 런북 | §4 시퀀스 제약·§11·§13-10·11·J6·§3.3 |
| **E. 잔여 정밀화** | §0.7-5 근거(래치 불변)·J6 finops측 provider 정규화·Phase D/E 파일 수 초과(8·7파일)·E2E 범위 차이·§15.2 필드 열거 누락(11필드 중 9만)·캐시 부재 미명시·파일 수 과소(5→18) | §0.7-5·J6·J7·§5(D1/D2·E1/E2)·§6·§0.2 |

---

- 원천: `docs/architecture/multi-infrastructure-control-plane-v2.md` r2+r2.1–r2.5 — 핵심 절: **§20 Phase 4 게이트**·**§14.2 (Aliyun/Tencent 매핑)**·**§21 PR 27–30**·**§15 (비교 프로토콜 — mapped family)**·§16 (API V2)·§17 (UI)·**§3 (AssetCloudAccount/AssetCredential/FinOps 처분 — row 5/6/17/18)**·**§5.4 (전파 규칙)**·§2.2 (자격 삼중 중복)·§7.4 (자격 바인딩)·§4.4 (P-class 서순)·§10.2 (신규 의존 금지)·**§25 (Slice B 정의)**.
- 선행 Phase 자산: Phase -1 (§4.1 14필드 v2 봉인 완료 — 게이트 G-1~G-5), Phase 0 (registry·계약·arch-boundary v0), Phase 1 (13 테이블·브로커·엔진), Phase 2 (k8s Discoverer·SyncRunner·§15 비교·백필), Phase 3 (executor·엔진 배선·감사·slice-a E2E).
- 범위 (§21): **PR 27** aliyun inventory behind Discoverer · **PR 28** tencent inventory behind Discoverer · **PR 29** credential 통합(공유 SecretRef) + finops rewire (스케줄러 유지) · **PR 30** web compute inventory V2 전환 + Slice B E2E.
- 출구 게이트 (§20 Phase 4): ① 실계정 discovery — **코어 스키마 변경 0·코어에 provider명 분기 0 (arch-boundary 음성 증명)** ② legacy 클라우드 뷰와 §15 비교 (매핑된 family별) ③ UI compute inventory가 V2 읽기.
- 계획 계약: `phase2-plan.md`·`phase3-plan.md` 관례 승계 (§0 실측 → 판단 J → 배치표 → 인터페이스 계약 → Phase 분해 ≤5파일 → 테스트 계약 → claims → 보존 제약 → 리스크 → 롤백 → 소유권 → 이월 대장 → 가정 A).

---

## 게이트 실현가능성 판定 (최상단 의무 — §0 실측 근거)

| 게이트 (§20 Phase 4) | 판정 | 근거(§0 실측) | 전제 |
|---|---|---|---|
| ① 실계정 discovery — 코어 스키마 변경 0 | **기계 증명 실현 가능** (스키마 0) | 코어 V2 13테이블 모델(`internal/infra/model/`)은 Phase 4가 건드리지 않음 — 클라우드 어댑터는 contract 기존 형상(`DiscoverRequest.Connection`·`Material["inventory"]`)으로 충분(§0.4). 스키마 변경은 PR 29의 `integration_finops_account` 1컬럼뿐 — **v1 EXTEND**(§3.2 row 17)이지 코어가 아님 | 없음 |
| ① 코어에 provider명 분기 0 (arch-boundary 음성 증명) | **실현 가능하나 증명기구 확장이 선행** | 현 R2는 `CORE_PACKAGES=(internal/domain/dnsserver)`만 수비(§0.5) — V2 코어가 커버 밖. 또한 `contract/provider_type.go:42`가 어휘로 `aliyun`,`tencent`를 **선언**하므로 단순 grep은 스펙 §7.1 위박인 오탐(판정 J8) | R2 v2: 코어 패키지 편입 + 어휘 등록 라인 예외(파일·라인 핀포인트) + `_test.go` 제외(J8-d) + 카나리 플레인트 확장 |
| ① "실계정" 자체 | **실현 불가(현재) — 에스컬레이션 E-1** | `asset_cloud_account` **0행**·`integration_finops_account` **0행**·`domain_public_dns_account` 0행·env/배포 설정/코드 하드코딩 전무(§0.1) | 사용자가 읽기 전용 자격 제공 또는 interim 판정 승인 |
| ② §15 비교 (mapped family = vm) | **조건부 실현 가능** | 비교 기구(§15 프로토콜)는 Phase 2가 코드화 — 클라우드 변형은 legacy 캡처 경로(`fetchCloudInstances` 동일 함수)·매핑 표·페어링 키(instance_id)만 신규(판정 J7). 그러나 페어 실행 양측이 **살아 있는 클라우드 계정**을 읽어야 함 | E-1 해소(실자격) 후 3일 3회 페어. interim은 mock 엔드포인트 경로(E-2) |
| ③ UI compute inventory V2 읽기 | **실현 가능 (실자격 불요)** | `ListResources` 읽기 API·infra 뷰 패턴(K8sInventory.vue)·메뉴 부트 시드·i18n 2로케일 파리티 게이트 전부 실측 확립(§0.6). 시드된 V2 테이블(compute.vm 행)로 E2E 가능 | `kindPrefix` 읽기 파라미터 1개 추가(판정 J9 — 스키마·분기 아님) |

### 에스컬레이션 (사용자 승인 대상 — Phase 2 E-1/E-2 선례 형식)

- **E-1 (게이트 ①②의 실계정 — 최상위 블로커)**: 스펙 §25 Slice B는 "existing cloud account (**real credentials**, collapsed SecretRef)"를 요구. 실측 결과 리포·DB·env 어디에도 Aliyun/Tencent 자격이 없다(§0.1). 선택지 2안:
  - **(a) 권장 — 실자격 제공**: 읽기 전용(ECS/CVM Describe 권한만) Aliyun AccessKey·Tencent SecretId/Key 각 1건. 제공 시 게이트 ①②를 스펙 문언 그대로 종결(실행 절차 §7 claim 8–10).
  - **(b) interim 수락**: 로컬 mock 클라우드 엔드포인트(판정 J1 Path M)로 기계 증명 전량을 실증하고, 게이트 아티팩트에 interim 마킹(§13-10 규격). **M1 선언은 실계정 증명 완료 전 불가**로 기록.
- **E-2 (interim 선택 시 — legacy 경로 엔드포인트 오버라이드)**: mock 클라우드로 §15 페어를 돌리려면 legacy 캡처(v1 `fetchCloudInstances`)도 mock 엔드포인트를 봐야 한다. `asset_cloud_aliyun.go`·`util/tencentcloud.go`에 **테스트 전용 env 오버라이드**(기본 미설정 — 운영 동작 무변경, 판정 J7) 추가 승인. **활성 게이트(r2)**: 오버라이드는 `GO_ENV=development`가 명시적일 때만 활성화(`util/secret_guard.go:15-49` developmentEnvironment 선례 승계 — 프로덕션 프로세스에서 env가 설정돼 있어도 무시·운영 엔드포인트 경로 유지). 보조로 스킴 검증(http·127.0.0.1/localhost 호스트만 수용 — 원격 https 오버라이드 거부)을 병행한다. 미승인 시 비교 기구는 순수 함수 테스트 + V2 단측(mock) 실증으로 한정.
- **E-3 (vm 외 리소스 family 이월 — §14.2 목록)**: §14.2는 disk/snapshot/eip·lb/vpc·switch/security group/image도 나열하나, (a) §20 Phase 4 Work는 "**existing** aliyun/tencent inventory code behind Discoverer contracts"이고 기존 코드는 인스턴스만 다루며(§0.2), (b) 게이트 ②는 "mapped family별" 비교인데 legacy에 매핑 가능한 family는 vm뿐, (c) Tencent는 SDK 모듈이 cvm뿐(신규 API family = 신규 의존 또는 TC3 전면 확장). **본 Phase는 vm family로 한정, 잔여 family는 이월 대장 등재** — 승인 확인.
- **E-4 (arch-boundary-canary 잡 확장)**: 보존 관례 "기존 CI 잡 무변경"의 명시적 예외 — R2 v2 카나리가 코어 분기 플랜트를 심으려면 기존 `arch-boundary-canary` 잡에 플랜트 라인 추가 필요(§0.5). 게이트 ① 증명기구 요건으로 승인 요청.
- **E-5 (cost.read capability 선언)**: aliyun/tencent provider type에 `cost.read`(빈 인터페이스 요구 — §3.7 매핑 표)를 선언할지. 권장: 선언(provider-types API의 정직성 — finops 레인이 실제 서비스). 코드값 1줄, 되돌림 가능.

이 외 에스컬레이션 없음. 판정 J4(SecretRef 재질 JSON blob)·J5(finops 읽기 경로만 rewire·저장 경로는 M2 — r2: 재기입 차단 포함)는 계획 승인 시 확인 요망 — 둘 다 국소적이다.

---

## 0. 현재 코드·자산 실측

### 0.1 자격 존재 여부 (게이트 실현가능성 ① — 전수)

실측 (2026-09-07, dev MySQL `ops_admin`):

```text
asset_cloud_account            0행            (asset_host.cloud_account_id 경유도 0)
integration_finops_account     0행
domain_public_dns_account      0행            (§2.2 삼중 중복의 세 번째 사본도 부재)
provider_connection            2행            전부 kubernetes (kind-v2-p2·kind-v2-p3)
환경변수                        ALIYUN*/TENCENT* 0건
config.yaml·deploy/·compose    클라우드 자격 0건 (credential-key는 §4.2 마스터키 — 클라우드 자격 아님)
코드 하드코딩                   없음 (legacy는 전부 DB 行에서 자격 읽음 — service.go:1497-1509)
```

결론: **실계정 증명 경로의 입력이 존재하지 않는다.** Phase 2의 E-2 선례(legacy `k8s_cluster` 0행 → kind 시드 클러스터 등록으로 해소)와 다르게, 클라우드는 "실계정"을 국소 대체물로 완전 치환할 수 없다(kind는 진짜 Kubernetes지만 mock Aliyun API는 진짜 Aliyun이 아니다). → E-1.

### 0.2 기존 클라우드 인벤토리 코드 (PR 27/28의 이관 대상)

```text
service/service.go:2913   cloudInstance 구조체 — 11필드 전수 (r2 정정):
                          InstanceID/HostName/PrivateIP/PublicIP/CPU/Memory/Disk/
                          OS/Region + SSHUser(string)/SSHPort(int) — 전부 표시용
service/service.go:2927   fetchCloudInstances(provider, ak, sk, region) — provider명 switch
                          "tencent|tencentcloud" → util.NewTencentCloudService
                          "aliyun|alicloud"     → fetchAliyunCloudInstances
service/asset_cloud_aliyun.go (233줄)  Aliyun ECS RPC 직접 HTTP (SDK 없음 — HMAC-SHA1 서명,
                          DescribeRegions/DescribeInstances, PageSize=100·PageNumber
                          순회(102-103행), 리전별 순회, 엔드포인트 ecs.<region>.aliyuncs.com)
util/tencentcloud.go (101줄)           Tencent SDK(common+cvm) — DescribeInstances,
                          리전별 client 생성, 실패 누적 후 부분 반환
호출처    service.go:1511 (SyncAssetHosts 흐름 — v1 호스트 동기화·AssetHost upsert)
```

- 클라우드 도메인 파일 수 (r2 정정 — 렌즈 L4): 인벤토리·과금 **핵심 로직**은 위 3파일 + finops 2파일(`finops_cloud_billing.go` 465줄·`integration_finops.go` 1262줄)이 맞으나, 도메인 전체(계정 CRUD·finops 동기화·v1 프런트엔드)는 **18파일** — backend 11(`model/asset.go`·`model/integration_finops.go`·`service/{service,asset_cloud_aliyun,finops_cloud_billing,finops_cloud_billing_test,integration_finops}.go`·`util/tencentcloud.go`·`controller/{controller,integration_finops}.go`·`store/migrate.go`) + web 7(`api/asset.js`·`api/integration.js`·`views/assets/CloudAccount.vue`·`views/integration/finops/{FinOpsAccounts,FinOpsBreakdown,FinOpsSync,FinOpsResources}.vue`). 스펙 §14.2 "10 files each"는 이 기준으로도 정확하지 않으나 r1의 "5파일이 전부" 서술은 핵심 로직 기준임을 명시하기 위해 본 정정을 둔다.
- `internal/domain/provider/{aliyun,tencent}*.go`은 **DNS/인증서 도메인**(§3 row 13 REMAIN — V2 무접촉). inventory 소스가 아님.
- 리소스 종류: **인스턴스(vm)만.** disk/snapshot/eip/vpc/sg/image는 현행 코드에 없음 → E-3.

### 0.3 finops 구조 (PR 29 rewire 대상)

```text
model/integration_finops.go   IntegrationFinOpsAccount: AccessKey/SecretKey/BillingToken
                              (§4.1 row 9 — Phase -1에서 v2 envelope으로 봉인됨, json:"-")
service/integration_finops.go:106  initFinOpsScheduler — 1분 티커·NextSyncAt 도달 계정
                              SyncFinOpsAccount(account.ID, "schedule") (스케줄러 — 무변경 대상)
service/integration_finops.go:179  SaveFinOpsAccount — v1 CRUD 저장 경로 (링크 해제 훅 위치·r2)
service/integration_finops.go:369  SyncFinOpsAccountMonths — 계정 行 로드 → 월별 순회
service/integration_finops.go:411-414  말미 저장: account.LastSyncAt/NextSyncAt 설정 후
                              s.db.Save(&account) — **전체 행 UPDATE** (평문 재기입 경로 — r2 판정 J5)
service/integration_finops.go:418  syncFinOpsAccountMonth(account model.IntegrationFinOpsAccount, …)
                              — 값 매개변수(복사본 수신 — 주입 격리의 구조적 근거)
service/finops_cloud_billing.go    fetchAliCloudBill/fetchTencentBill — TC3-HMAC-SHA256
                              서명 구현 포함 (Tencent POST 서명의 in-tree 선례)
```

### 0.4 V2 코어 자산 (Phase 0–3 — 재사용 형상)

```text
contract   Discoverer/DiscoverRequest{ContextID,Cursor,Connection}·ConnectionView.
           Material["inventory"]·DiscoveredResource{ExternalID,ExternalURN,Kind,
           Subtype,DisplayName,Raw,Normalized}·ProviderSignalError 3종 — 전부 기존 형상,
           클라우드 어댑터에 계약 변경 불요 (게이트 ① 스키마 0의 근거)
registry   M1ProviderTypeNames에 이미 "aliyun","tencent" 포함(provider_type.go:42 —
           §7.1)·M1CapabilityVocabulary에 compute.vm.read 포함(OwnerPhase M1)·
           §3.7 매핑 표: compute.vm.read → Discoverer 필수, cost.read → 빈(선택)
compose    Build이 kubernetes+fake 등록 — aliyun/tencent 등록 란 부재(신규 4–6줄)
inventory  SyncRunner(레지스트리 조회만 — provider 분기 없음, arch rule 1)·
           RunK8sBackfill(§5.4 증분·stale 마킹·P-class 봉인 규칙 sourceKubeSecret —
           클라우드 백필의 패턴 원형. 오류 시 즉시 전체 반환(backfill.go:85) — 클라우드는
           계정 단위 격리로 차별화, 판정 J6)·compare.go(§15 순수 함수 — k8s 섹션 종속,
           캐시 계층 부재 실측)
contracttest  RunContractSuite 4단얫(페이징·에러 분류·rate-limit 신호·redaction)+
           Fixture{Seed/PageLimit/Scenario/Connection} — k8s가 이미 통과(T45)
api/v2     GET 4종(Register — infra_test.go:164 exactly-four 단얫이 집계 핀)·mutation
           7종+읽기 3종(RegisterOperations — operations.go:43. tasks 목록은 여기에
           등록해야 단얫 보존, r2 판정 M8). ListResources — kind 정확일치 필터만
           (infra.go:253). **GET /api/v2/infra/tasks 목록 부재** — infra.js가
           listInfraTasks 호출 중(라이브 404 — 이월 #1)
web        infra 뷰 6종·infra-i18n.js(ko/en)·메뉴 부트 시드(seed.go:363)·
           Playwright slice-a.spec.js + stack.sh (E2E 패턴 원형)
metrics    Counters{IncRateLimit/IncAPIError/Render} — provider 라벨 범용(클라우드
           어댑터가 WithCounters로 승계 가능). §18.2 나머지 행 미계측(이월 판정 §13.6)
```

### 0.5 "no provider-name branch in core" 증명 수단 실측 (게이트 ① — R2 연동)

```text
scripts/check-arch-boundary.sh (v0)
  R1/R2 수비 대상: CORE_PACKAGES=(internal/domain/dnsserver) — V2 코어 전부 커버 밖
  R2 규칙: grep -rnE '\b(aliyun|tencent)\b' --include='*.go' (47행 — *_test.go 제외
          장치 없음: 코어 테스트의 목업·플레인트 테스트 식별자가 오탐, r2 J8-d)
  문제 1: V2 코어(contract·registry·inventory·tasks·api/v2·compose)가 전부 밖이다
  문제 2: contract/provider_type.go:42 는 §7.1 이행으로 "aliyun","tencent"를 **선언** —
          R2를 코어에 그대로 적용하면 스펙 준수 코드가 오탐 FAIL
  문제 3: compose.go는 어댑터 등록을 위해 aliyun/tencent 패키지를 import해야 한다(§11.1
          "registered explicitly") — 조립 계층의 이름 언급은 설계 요건
카나리: arch-boundary-canary 잡은 R1/R3 플랜트만 보유 — provider 분기 플랜트 없음
```

→ 판정 J8: R2 v2 (코어 집합 확장 + 어휘 등록 예외 + `_test.go` 제외 + 조립 계층 제외 + 카나리 플랜트).

### 0.6 UI/API·E2E 실측

- `ListResources` 응답 `resourceView` — kind·subtype·URN 등 K8sInventory.vue가 소비하는 열 그대로 (Compute 뷰 재사용 가능).
- compute 종류 필터: `kind=compute.vm` 정확일치는 되지만 family 일괄(compute.*)은 불가 → `kindPrefix` 파라미터 1개 추가 (판정 J9).
- 메뉴: seed.go:363 infra 그룹 4항 — `infra:compute` 1행 추가(additive). i18n: infra-i18n.js에 신규 키 — 파리티 게이트(`node scripts/check-i18n-parity.mjs`)가 2로케일 강제.
- route-coverage: v2 GET 추가는 route-inventory.txt 골든 재생성만 수반(non-GET 아님 — sensitive 목록 무변경 예상, 재생성 커밋으로 처리).
- E2E: `web/e2e/stack.sh`+`slice-a.spec.js` 패턴 승계 — V2 테이블 시드 기반으로 실자격 불요.

### 0.7 Phase 3 이월 대장 — 본 계획 처리 여부 판정 (과제 의무)

| 이월 (p3-state followups_phase4) | 판정 | 근거 |
|---|---|---|
| 1. GET /tasks 목록 라우트 | **본 Phase 처리 (Phase D1)** | §16.2 목록에 누락된 스펙 갭 + 라이브 404 실측(infra.js 호출 중·§0.4) — 방치 시 게이트 ③ UI 증명의 신뢰성 훼손 (r2: "§16.2 규정" → 스펙 갭+실측으로 근거 정정) |
| 2. seed sts/ds | **본 Phase 처리 (Phase F말)** | k8s-fixture/seed.yaml 실측 deploy 1종뿐(v3-seed/restart-target) — slice-a E2E의 종별 커버 보험. additive 2매니페스트 |
| 3. normalizer 관계 링크 | **이월 (M2 cutover 전 재검)** | K8s family 소관 — Slice B 불요(§20 게이트 무요구). 현 reconcile.go의 kind별 raw-필드 판독은 코어 분기 증가 우려(J8 증명과 긴장) — 관계 링크 확장은 별도 설계 필요 |
| 4. R2 재설계 (크래시 인젝션 안정화) | **조건부 이월** | 플레이크 관찰 시에만 착수 — 현재 slice-a-e2e 안정(관찰 없음). 관찰 시 M1 준비 태스크에서 |
| 5. 커버리지 래치 | **실측 정정 — 이미 존재** | v2-ci backend-test 잡에 "opdef coverage latch (baseline 65.4, gate 60.0)" 라인 실측(2026-09-07). **래치 측정 대상은 opdef(작업 계약) 커버리지이며 본 계획은 opdef를 추가하지 않는다(읽기 전용 — §20) — 래치 수치 불변, 신규 구현 불요.** 신규 어댑터 패키지의 로컬 커버리지는 래치와 별개로 claim 15(≥80%)로 관리 (r2: opdef 래치 불변으로 근거 정정) |
| 6. §18.2 메트릭 앵커 (M1 선언 전 필수) | **부분 처리 + 이월 등재** | Phase 4 소유분만 완료: cloud 어댑터가 기존 Counters(provider_api_errors_total·provider_rate_limit_total — §18.2 라벨 정합)를 승계·발화. 나머지 행(provider_health·sync duration·worker_queue·secret_access 등 전면)은 M1 선언 준비 태스크로 명시적 이월 — "M1 선언 전 필수" 준수는 이 대장으로 추적 |
| 7. normalizer generation/annotation 투영 | **이월 (K8s family)** | Slice B 불요 |

### 0.8 SDK·의존성 실측 (§10.2 — 신규 의존 금지)

```text
go.mod  github.com/tencentcloud/tencentcloud-sdk-go — common v1.3.48 + cvm v1.1.31 만
        (util/tencentcloud.go가 사용 중 — 기존 런타임 의존)
       Aliyun SDK 없음 (legacy도 raw RPC — 서명 60줄 in-tree)
결론   vm family 범위에서 신규 의존 0으로 충분: aliyun=raw RPC 재구현·
       tencent=SDK 재사용 또는 TC3 직구현(판정 J2). disk/vpc 등 확장 시에도
       TC3 직구현이면 의존 추가 불요 (E-3 이월분의 재입구 근거)
```

---

## 1. 아키텍처 판단 (선택지 비교 + 선택 근거)

### J1 — 실계정 부재 대응: 이중 경로 (Path R / Path M) 〔임계〕

- **Path R (real — 게이트 정정 경로)**: E-1(a) 해소 시. 절차가 곧 claim 8–10: v1 클라우드 계정 등록 → `sync-inventory`(백필+싱크) → `compare-inventory --cloud-account` 페어 3일 3회. 코드는 Path M과 100% 동일 — 실자격은 입력일 뿐.
- **Path M (mock — interim 실증 경로)**: Go `httptest` 기반 mock 클라우드(Aliyun RPC JSON·Tencent CVM JSON 응답)를 어댑터 테스트·싱크 통합에 사용. V2 측 엔드포인트는 `Connection.Endpoint` 주입(계약 기존 필드 — 변경 0). legacy 캡처가 mock을 보려면 E-2(env 오버라이드 + GO_ENV 게이트).
- 기각안: "실계정 없이 게이트 ① 성공 선언" — §25 문언 위반. honest 마킹(E-1(b))이 유일한 대안.
- Path M의 가치: 계약 하네스 4단얫·정규화 정확성·싱크 회로·비교 순수 함수·백필·UI를 전부 실자격 대기 없이 완성 — 실자격 도착 시 게이트 실행이 "명령 3개"가 되게 한다.

### J2 — 어댑터 클라이언트: aliyun raw RPC / tencent TC3 직구현 〔임계〕

| 선택지 | 판정 | 근거 |
|---|---|---|
| (a) legacy service 함수 재사용(import) | 기각 | adapter→service import는 결계 위반(arch rule 2 — 어댑터는 무상태 잎). k8s 전례: Phase 2도 legacy REST 경로를 adapter/client.go로 재구현 |
| (b) Tencent SDK(common+cvm) 재사용 | **병행 허용** | 기존 의존 — 실계정 경로에서 검증된 서명/역직렬화. 단 `clientProfile.HttpProfile.Endpoint`의 커스텀 엔드포인트(http 스킴) 수용 여부가 구현 시점 검증 과제 |
| **(c) tencent TC3 직구현 (POST 서명)** | **선택 (기본)** | `finops_cloud_billing.go`의 TC3-HMAC-SHA256 구현이 in-tree 선례 — 어댑터로 일반화 이동. 엔드포인트·스킴 완전 자유(Path M 필수 조건), 향후 family 확장(cbs/vpc)도 단일 클라이언트(§10.2 무의존 유지). (b)가 실측에서 http 엔드포인트를 깔끔히 수용하면 교체 허용 — 하네스 단얫은 동일 |

aliyun은 선택지 없음(SDK 부재) — `asset_cloud_aliyun.go`의 RPC 규약(HMAC-SHA1·SignatureNonce·Version 2014-05-26)을 `adapter/aliyun/client.go`로 재구현하고 `Connection.Endpoint` 우선 적용.

### J3 — 디스커버리 범위: vm family (게이트 크리티컬) + 잔여 이월

- 본 Phase 정규화 대상 = `compute.vm` 1종 (기존 코드의 전량 — §20 Work 문언). kind 어휘는 이미 존재(`M1ResourceKinds`의 compute.vm) — **어휘 확장 0**.
- §14.2 잔여 목록(disk·snapshot·eip/lb·vpc/switch·sg·image)은 E-3으로 이월 등재(사유 3종 — §0.2·게이트 ②·§0.8).
- Discoverer 페이징: 커서 `<region>|<page>` — 리전×페이지 순회(k8s section 커서 관례 승계). 리전 출처: Context.MetadataJSON.regions 우선, 비면 aliyun=DescribeRegions·tencent=계정 리전 필수(legacy 동일 — Tencent는 리전 열거 API 미사용).

### J4 — 자격 통합 스키마: SecretRef 재질 JSON blob + 목적별 바인딩 〔임계〕

- **SecretRef 1건 = 자격 1벌**: `Ciphertext = v2봉인(JSON {"accessKey":"…","secretKey":"…"})`. k8s(kubeconfig 단일 문자열)과 달리 클라우드 자격은 2–3성분 — 컬럼별 envelope을 백필 시 복호→JSON 조립→재봉인(k8s의 verbatim 복사 불가, `sourceKubeSecret`의 P-class 규칙 승계: 평문 잔재는 봉인 후 복사, UNKNOWN은 halt).
- **바인딩**: 커넥션당 `(purpose)` 1행 원칙 — `inventory`(asset_cloud_account 유래)·`billing`(finops 유래, 동일 자격이면 같은 SecretRef 지목 — §7.4 "same SecretRef where shared"). 다른 자격이면 billing이 별도 SecretRef를 지목(브로커는 purpose별 first-binding 조회 — 목적 내 2행을 백필이 만들지 않음).
- **컨텍스트**: `Kind="account"`, `ExternalID=AccessKey`(계정 고유 식별의 실측 유일 대안 — 컬럼 부재), `MetadataJSON={"regions":[…],"sourceProvider":"alicloud"}` (§7.3·§14.2 "regions in MetadataJSON").
- **finops 링크**: `integration_finops_account.provider_connection_uid varchar(64) NULL` — **v1 EXTEND**(§3.2 row 17 "EXTEND -1/M1" 근거), V2 마이그레이션 러너 step0006 소유(phase3 step0005 감사 EXTEND 패턴 승계). nullable — 기존 행 무백필.
- 기각안: finops 테이블에 자격 컬럼 유지+참조만 — "triplication collapse" 미달(§2.2). 기각안: SecretRef 3건(성분별) — 브로커 Resolve가 1회 1값만 반환하는 계약과 불일치.

### J5 — finops rewire: 읽기 경로만 + **평문 재기입 차단** 〔임계 — r2 전면 재서술〕

r1의 설계 그대로 두면 결함: `SyncFinOpsAccountMonths` 말미(:411-414)가 `s.db.Save(&account)` **전체 행 저장**이다. 브로커 평문을 주입한 사본이 이 Save에 도달하면 평문 자격이 DB에 재기입된다(봉인 우회). r2는 다음 3층으로 차단한다.

1. **주입 격리 — 로컬 복사본 한정**: `SyncFinOpsAccountMonths`에서 브로커 해석(`account.ProviderConnectionUID != ""` → `secrets.NewBroker(s.db).Resolve(ctx, uid, "billing")` → JSON 파싱)은 **`syncFinOpsAccountMonth`에 넘기는 로컬 사본(`rewired := account`)에만** 적용한다. `syncFinOpsAccountMonth`가 값 매개변수로 수신(:418)하므로 원변수·원행은 구조적으로 무오염. 빈 링크 → rewired는 그대로(기존 행 직독 값) — 전환 창구(§16.4 호혜 구조).
2. **말미 저장 좁히기 — 자격 컬럼 미기입**: :411-414의 전체 `Save(&account)`를 `Model(&IntegrationFinOpsAccount{}).Where("id=?", account.ID).Select("last_sync_at","next_sync_at").Updates(...)` 로 변경 — UPDATE 문이 자격 3컬럼(access_key·secret_key·billing_token)을 아예 포함하지 않는다. 이 4행 변경은 보존 제약 2의 "시작부만" 범위에 대한 **명시적 예외로 승인**(자격 비침범·스케줄러 무변경 — GORM 필드 전송이 아니라 칼럼 명시만 바뀐다).
3. **저장 경로는 여전히 통합하지 않는다**: `SaveFinOpsAccount`(:179) 등 v1 CRUD·생성 UI는 §5.4 전파 규칙상 M1 내 계속 살아있고 통합은 M2 cutover 소관. 백필이 링크를 만든다(재실행·증분).

- 스케줄러(`initFinOpsScheduler`)·동기화 주기·과금 레코드 파이프라인 무변경. 과금 API 클라이언트(`finops_cloud_billing.go`)는 시그니처 무변경 — 사본 필드가 바뀌어 흐르는 것으로 충분(최소 diff).
- **링크 해제 전파 (r2 — F2)**: 자격 필드가 갱신되면 링크가 곧 기만(stale 링크가 새 자격이 아닌 옛 자격을 브로커에서 읽음) — `SaveFinOpsAccount`가 자격 필드(access_key/secret_key/billing_token) 변경을 감지하면 같은 트랜잭션에서 `provider_connection_uid=NULL` 화이트필드만 갱신(해제). `UpdateAssetCloudAccount`(service.go:2240)도 대칭으로 커넥션의 inventory 바인딩을 해제(삭제)한다. 운영 런북: "v1 화면에서 자격 교체 후에는 `sync-inventory`를 재실행해 체인을 재생성" — §11에 기재.

### J6 — 클라우드 백필: RunCloudAccountBackfill (k8s 패턴 승계 + r2 2항 정밀화)

`inventory/backfill_cloud.go` — `asset_cloud_account` 순회(status=1):
1. provider 정규화(`alicloud→aliyun`·`tencentcloud→tencent` — legacy switch와 동일 어휘),
2. 체인 upsert(커넥션+컨텍스트+SecretRef+inventory 바인딩) — `source_model='asset_cloud_account'`·source_id·`source_updated_at` 체크포인트(§5.4 증분 — 재실행은 변경행만),
3. finops 계정 순회: **양측 provider를 동일 규칙으로 정규화(finops측도 alicloud→aliyun 매핑 적용 — r2 M4)** 후 (복호된 accessKey) 일치 판정. 일치 → 커넥션 재사용 + billing 바인딩(자격 동일 시 동일 SecretRef) + `provider_connection_uid` 백필. **accessKey 일치·secretKey(또는 billingToken) 상이 → 동일 SecretRef 공유 불가(§7.4) — billing 바인딩이 별도 SecretRef를 지목·링크는 커넥션 재사용 유지. provider 또는 accessKey 불일치 → 별도 커넥션+SecretRef+billing 바인딩** (가정 A7과 정합),
4. v1 삭제 행 stale 마킹(k8s `markStaleSources` 승계).

- **오류 격리 (r2 — F12)**: 한 계정의 실패(봉인 불능·UNKNOWN 등)는 k8s 백필(backfill.go:85 즉시 전체 반환)과 달리 **해당 계정만 스킵+리포트(Report.Failed/Skipped에 계정 식별)하고 잔여 계정·finops 링크·stale 마킹을 계속**한다 — 클라우드는 계정이 다수·독립이므로 1계정 실패가 전체 싱크를 마비하지 않게 한다. UNKNOWN 자격의 "halt"는 그 계정의 체인 생성 중단(행 스코프)을 의미한다(R3 완화·§4.3 검역과 정합).
- 트리거: `main.go sync-inventory`가 k8s 백필을 이미 자동 수행(main_sync.go:91) — 클라우드 백필을 같은 지점에 추가(별도 서브커맨드 불필요).

### J7 — §15 비교의 클라우드 변형: legacy 캡처 = 같은 service 함수, mapped family = vm

- legacy 측(§15.1 "calls the same service methods the v1 API handlers call"): `Service.CaptureCloudInstancesForCompare(accountID)` — 신규 **exported 래퍼**가 `fetchCloudInstances`(v1 동기화 핸들러가 쓰는 동일 함수)를 DB 기록 없이 호출. 엔드포인트: E-2 승인 시 env 오버라이드(`OPS_ADMIN_CLOUD_ENDPOINT_OVERRIDE_ALIYUN`/`_TENCENT` — GO_ENV=development 게이트, 미설정/비활성 시 운영 엔드포인트, 동작 무변경).
- V2 측: 완료 generation의 `compute.vm` 투영(기존 projection 쿼리 재사용). 페어링 키 = `instance_id`(계정 스코프). 매핑 표 = `adapter/{aliyun,tencent}/mapping.md` — **legacy `cloudInstance` 11필드 전수 열거(r2 L1)**: 페어링 키 InstanceID + 매핑 필드 8종(HostName·PrivateIP·PublicIP·CPU·Memory·Disk·OS·Region) + **처분 명시 2종 — SSHUser·SSHPort는 legacy v1 화면 표시 전용 필드로 V2 정규화 스키마에 대응 필드가 없다(§3.2의 `sshHint`는 관측값이 아닌 프로바이더 기본 힌트) → §15.2 규칙에 따라 dropped(사유: display-only, v1 host 표시용)로 mapping.md·cloudcompare 양쪽에 기록**. 커버리지 규칙 §15.2: 매핑 또는 dropped(reason) — 11필드 전부 둘 중 하나로 처분되어야 claim이 성립한다.
- 분류 재사용: BLOCKER(identity·비휘발 필드)·VOLATILE(시각·순서 — CPU/Memory/Disk 표시 문자열은 **QuantityEpsilon 관례 승계**, k8s formatMemoryMB 왕복 올림 허용)·DRIFT·ABSENT. 클라우드 특성: 인스턴스 상태 변경은 휘발성이 아니라 관측 차이 — legacy cloudInstance엔 상태 필드가 없어 비교 불가(ABSENT-dropped로 명시).
- **캐시 부재 명시 (r2 L2)**: 비교 캡처에는 캐시 계층이 없다(compare.go 실측 — 캐시 0건). 각 페어는 양측 실시간 캡처이고 재실행은 새 `<time>.json 아티팩트를 만든다 — 페어 1회 비용 = 클라우드 API 호출 각 1회. 게이트 창구(3일 3회)의 호출 예산은 rate 카운터로 관찰한다.
- 게이트 창구: §15.4 그대로 — 3일 3회 페어(실계정 확보 후). 창 중 계정 리전 설정 변경 금지(phase2 게이트 윈도우 지침 승계).

### J8 — arch-boundary R2 v2: 게이트 ① "코어 분기 0"의 기계 증명 〔임계 — r2 (d) 추가〕

```text
CORE_PACKAGES v2 (R1/R2 수비 집합 — 코어 오케스트레이션):
  internal/infra/{contract, registry, inventory, secrets, model, policy, metrics}
  internal/tasks  internal/api/v2
제외 (사유 명시): internal/infra/compose — 조립 루트. §11.1 "registered explicitly"가
  어댑터 import·이름 언급을 요구하는 등록 지점. "core orchestration"(§25)이 아니다.
  adapter/** — 당연히 제외(구현체).
R2 v2 규칙:
  (a) 코어 패키지 내 aliyun|tencent|alicloud|tencentcloud 식별자 등장 → FAIL
  (b) 예외 허용 목록: contract/provider_type.go 의 §7.1 어휘 선언 라인만 —
      파일+심볼 핀포인트(M1ProviderTypeNames/ReservedProviderTypeNames)로 한정.
      어휘 파일의 다른 위치·다른 코어 파일은 예외 없음
  (c) 분기 형태 강화: '== "aliyun"'·case "aliyun"·ProviderType 비교 라인 감지 패턴
      (b)의 예외 라인도 (c) 패턴과 교차하면 FAIL — 선언과 분기의 구분
  (d) *_test.go 스캔 제외 (r2): grep에 --exclude='*_test.go' 병행(현 47행은
      --include='*.go'만 — 구현 시 find "$pkg" -name '*.go' -not -name '*_test.go'
      파이프라인 또는 grep --exclude 어느 쪽이든 동일 효과). 사유: 코어 패키지
      테스트의 합법 식별자 등장 — 계약 하네스 목업·카나리 플레인트 테스트·
      api/v2 테스트의 provider 문자열 목업은 "제품 분기"가 아니다. 제품 코드
      (_test.go 제외)에는 여전히 식별자 0을 요구한다 — 위음성 증가분은 (c)의
      분기 형태 검사가 _test.go 밖에서 그대로 굳혀 상쇄
카나리 (E-4): inventory/sync.go(제품 코드) 에 'if conn.ProviderType == "aliyun" {}'
  플레인트 → FAIL 발화 — 음성 증명의 반증 가능성. 추가로 _test.go 안의 같은
  플레인트는 (d)로 통과하는 것을 단얫(제외 규칙의 자기 검증)
추가 증거(이미 존재·변경 없음): SyncRunner가 registry 조회만으로 어댑터 접근
  (sync.go 주석 arch rule 1) — 방향성(코어→어댑터 import 부재)은 go list로 병행 단얫
```

기각안: "문자열 전면 부족" — §7.1 어휘 등록과 모순(오탐). 기각안: grep만 없이 리뷰 판정 — 게이트는 기계 증명 요구(§20). 기각안: _test.go까지 스캔 — 코어 테스트의 목업을 전부 우회 코딩(문자열 조립 등)하도록 강제하는 비용을 게이트 증명이 지불할 이유가 없다(오탐).

### J9 — UI: ComputeInventory + kindPrefix 읽기 파라미터

- `GET /api/v2/infra/resources?kindPrefix=compute.` — 읽기 전용 쿼리 파라미터 1개(기존 kind 정확일치 유지). **스키마 아님·분기 아님**(게이트 ① 위반 아님 — SQL where 접두 비교). ComputeInventory.vue는 K8sInventory.vue 패턴 승계(표+상세 드로어+정규화/Raw 탭).
- 메뉴: seed.go infra 그룹에 `infra:compute → /infra/compute` 1행(additive upsert). i18n: infra-i18n.js ko/en 쌍 추가(파리티 게이트). 라우터: `/infra/compute` 1줄.
- carryover 처리: `GET /api/v2/infra/tasks` 목록 핸들러(Phase D1 — §0.7-1). 상태 필터·페이징 포함, 읽기 전용(opdef 무관·route-inventory 골든만 재생성). **등록 위치는 `RegisterOperations`(operations.go:43 — r2 M2)**: Phase 3 관례("the Phase 2 GET subset in Register is untouched so its route-count pin stays meaningful" — operations.go:33 주석) 승계. `Register`(infra.go)의 GET 집계는 `TestV2RoutesRegisteredExactlyFour`(infra_test.go:164)가 핀으로 잡고 있어 여기에 등록하면 단얫이 깨진다 — RegisterOperations는 그 집계 밖(이미 GET `/tasks/:uid` 등 읽기 3종 선례)이므로 **exactly-four 단얫 무변경 보존**.

### J10 — capability 선언 (registry V5 정합)

- aliyun/tencent 각각: `inventory.full`(ResourceKinds=[compute.vm]) + `compute.vm.read`(동일) — 둘 다 매핑 표가 Discoverer 필수 → 어댑터가 Discoverer 구현으로 통과.
- `cost.read`(E-5 승인 시): ResourceKinds=[compute.vm], 빈 인터페이스 요구 — finops 레인 존재의 선언. 오퍼레이션 정의(신규) 없음 — Phase 4는 읽기 전용(§20).

---

## 2. 수정 대상 (배치표 — 전수)

### 신규 (17)

| # | 파일 | PR | 요약 |
|---|---|---|---|
| N1 | `backend/internal/infra/adapter/aliyun/adapter.go` | 27 | BaseAdapter+Discoverer·ProviderName "aliyun"·ContextKinds ["account"]·엔드포인트 주입·WithCounters |
| N2 | `backend/internal/infra/adapter/aliyun/client.go` | 27 | ECS RPC 클라이언트(HMAC-SHA1·DescribeRegions/DescribeInstances·Connection.Endpoint 우선) |
| N3 | `backend/internal/infra/adapter/aliyun/normalizer.go` | 27 | aliyunECSInstance→DiscoveredResource(compute.vm)·URN·Normalized(JSON)·Raw 크기 상한 |
| N4 | `backend/internal/infra/adapter/aliyun/mapping.md` | 27 | §15.2 매핑 표 — **11필드 전수(SSHUser/SSHPort dropped 사유 포함, r2 L1)** |
| N5 | `backend/internal/infra/adapter/aliyun/adapter_test.go` | 27 | contracttest.RunContractSuite + httptest 시나리오(페이징·429·권한·redaction 카나리) |
| N6 | `backend/internal/infra/adapter/tencent/adapter.go` | 28 | 대칭(N1)·ProviderName "tencent" |
| N7 | `backend/internal/infra/adapter/tencent/client.go` | 28 | TC3-HMAC-SHA256 POST 클라이언트(finops 서명 규약 승계·CVM DescribeInstances) |
| N8 | `backend/internal/infra/adapter/tencent/normalizer.go` | 28 | TencentInstanceSet→DiscoveredResource(compute.vm)·nil-safe |
| N9 | `backend/internal/infra/adapter/tencent/mapping.md` | 28 | 대칭(N4 — 11필드 전수) |
| N10 | `backend/internal/infra/adapter/tencent/adapter_test.go` | 28 | 대칭(N5) |
| N11 | `backend/internal/infra/inventory/backfill_cloud.go` | 29 | RunCloudAccountBackfill(체인 upsert·finops 링크·collapse·stale 마킹·증분·**계정 단위 오류 격리**) (판정 J6) |
| N12 | `backend/internal/infra/inventory/backfill_cloud_test.go` | 29 | 멱등 재실행·collapse 분기(동일/상이 자격·accessKey 일치 secretKey 상이)·stale·finops 링크 백필·**격리(1계정 실패 후 잔여 계정 계속)** |
| N13 | `backend/internal/infra/migrate/step0006_finops_cloud_link.go` | 29 | integration_finops_account.provider_connection_uid NULL 컬럼 |
| N14 | `backend/internal/infra/inventory/cloudcompare.go` | 30 | §15 순수 비교 함수(클라우드 vm 섹션·페어링 instance_id·QuantityEpsilon·**11필드 처분 완결 검증**) |
| N15 | `backend/internal/infra/inventory/cloudcompare_test.go` | 30 | BLOCKER/VOLATILE/DRIFT/ABSENT 분류·매핑 커버리지 단얫(11필드) |
| N16 | `web/src/views/infra/ComputeInventory.vue` | 30 | compute family 인벤토리 뷰(K8sInventory 패턴 승계) |
| N17 | `web/e2e/slice-b.spec.js` | 30 | Playwright — 시드 V2 compute.vm 행 렌더링·상세 드로어 |

### 수정 (16)

| # | 파일 | PR | 요약 |
|---|---|---|---|
| M1 | `backend/internal/infra/compose/compose.go` | 27/28 | aliyun·tencent 어댑터 등록+카운터 RegisterProviders+capability 선언(J10) |
| M2 | `backend/internal/infra/compose/compose_test.go` | 27/28 | 단얫 추가(등록 4종·capability·provider-types 노출). **명시적 예외 1건(r2 H3): 41-42행 등록 수 단얫 `len(names) != 2`→`!= 4`·want 문구 `[fake kubernetes]`→`[fake kubernetes aliyun tencent]` 갱신 — 어댑터 등록의 필연, 골든 재생성과 동일 취급·커밋 근거. 그 외 기존 단얫 무변경** |
| M3 | `backend/model/integration_finops.go` | 29 | ProviderConnectionUID 필드(json:"-"·NULL) |
| M4 | `backend/service/integration_finops.go` | 29 | SyncFinOpsAccountMonths 자격 해석(**로컬 복사본 주입 + 말미 Select("last_sync_at","next_sync_at") 좁은 Updates — 판정 J5 재기입 차단**)·SaveFinOpsAccount 자격 변경 시 provider_connection_uid=NULL 전파(J5-3) |
| M5 | `backend/main_sync.go` | 29 | sync-inventory에 RunCloudAccountBackfill 호출·리포트 병기. **머지 순서 제약 — §4 시퀀스** |
| M6 | `backend/main_compare.go` | 30 | `--cloud-account` 플래그·클라우드 페어 캡처·아티팩트 경로 `data/compare/cloud/<account>/<date>/`. **머지 순서 제약 — §4 시퀀스** |
| M7 | `backend/internal/api/v2/infra.go` | 30 | ListResources kindPrefix 파라미터(J9) |
| M8 | `backend/internal/api/v2/tasks.go` | 30 | ListTasks 핸들러(페이징·status 필터) — **등록은 RegisterOperations(operations.go)에 1줄, Register(GET 집계) 아님 — exactly-four 단얫 보존(J9·r2)** |
| M9 | `backend/internal/api/v2/infra_test.go` | 30 | kindPrefix·tasks 목록 단얫 추가(exactly-four 단얫은 무변경 — tasks가 Register 밖이라 깨지지 않음을 그대로 통과로 증명) |
| M10 | `backend/service/service.go` | 30 | CaptureCloudInstancesForCompare exported 래퍼(J7 — fetchCloudInstances 래핑, 기록 없음) + UpdateAssetCloudAccount(:2240) 자격 변경 시 inventory 바인딩 해제 전파(J5-3) |
| M11 | `backend/service/asset_cloud_aliyun.go` + `backend/util/tencentcloud.go` | 30 | env 엔드포인트 오버라이드 훅(E-2 승인분 — **GO_ENV=development 게이트·스킴 검증·기본 미설정·운영 무변경**) 2파일 |
| M12 | `web/src/api/infra.js` | 30 | listInfraComputeResources(kindPrefix) |
| M13 | `web/src/router/index.js` | 30 | `/infra/compute` 라우트 1줄 |
| M14 | `web/src/utils/infra-i18n.js` | 30 | ko/en 키 쌍(compute 뷰) |
| M15 | `backend/store/seed.go` | 30 | infra:compute 메뉴 1행(additive upsert) |
| M16 | `scripts/check-arch-boundary.sh` + `.github/workflows/v2-ci.yml`(arch-boundary-canary) | 30 | R2 v2(J8 — 코어 집합·어휘 예외·_test.go 제외·분기 형태) + 카나리 플레인트(E-4 승인분) |

부수 갱신: route-inventory.txt 골든 재생성(tasks 라우트 추가·GET 무상 민감목록 예상)·migration baseline fixture(step0006 반영) — 각 Phase 검증 항목. `v2-phase1/k8s-fixture/seed.yaml` sts/ds 2매니페스트(Phase F말). `scripts/secret-scan.py` allowlist 항목(모의 자격 리터럴 — claim 14·사전 명시).

---

## 3. 인터페이스 계약 (스펙 어휘의 정확한 코드화)

### 3.1 어댑터 공통 (PR 27/28 — 계약 변경 0)

```go
// adapter/aliyun/adapter.go (tencent 대칭)
type Adapter struct { metrics *metrics.Counters; pageSize int; endpointOverride func() string }
func NewAdapter(opts ...Option) *Adapter            // WithCounters·WithPageSize(하네스)
func (*Adapter) Descriptor() contract.ProviderTypeDescriptor
    // {Type:"aliyun", AdapterVersion:"1", ProtocolVersion:"1",
    //  ContextKinds:[]string{"account"}, BuiltIn:true}
func (a *Adapter) Discover(ctx, req contract.DiscoverRequest) (contract.DiscoverPage, error)
    // 자격: req.Connection.Material["inventory"] — JSON {"accessKey","secretKey"}
    // 파싱 실패/빈 값 → 에러("missing credential material" — k8s J2 문언 승계)
    // 커서: "<region>|<pageNumber>" — 페이징 단얫 1(하네스 PageLimit 강제)
    // 엔드포인트: req.Connection.Endpoint 우선, 빈 값이면 프로바이더 기본(리전 조합)
func (a *Adapter) Validate/Health — DescribeRegions(aliyun)/DescribeInstances 1리전(tencent)
    // HealthResult.Message에 계정 식별·리전 수 (자격 물질 미포함)
var _ contract.BaseAdapter = (*Adapter)(nil); var _ contract.Discoverer = (*Adapter)(nil)
```

- 에러 매핑: 429/Throttling → `ProviderSignalError{SignalRateLimited}`·권한 코드 → `SignalPermissionDenied`·접속류 → `SignalUnreachable` (§9.3 어휘 1:1 — 하네스 단얫 2).
- Raw/Normalized redaction: Raw에서 자격·서명 파라미터 제거, Normalized는 식별·용량·상태만 (하네스 MarkerValue 단얫 4).

### 3.2 정규화 (vm family — mapping.md와 1:1)

```text
ExternalID   instance_id (e.g. i-xxxx / ins-xxxx)
ExternalURN  urn:<provider_type>:<context_id>:compute.vm:<instance_id>
             (k8s normalizer URN 관례 승계 — 유니크 제약 UNIQUE(context,kind,urn) 성분)
Kind         "compute.vm"   Subtype: 인스턴스 패밀리(있으면)·없으면 ""
Normalized   {displayName, region, zone, cpu, memoryGB, diskGB, os,
              privateIps[], publicIps[], instanceType, status(프로바이더 상태 코드),
              sshHint:{user:"root",port:22}}
Raw          프로바이더 응답 중 필요 최소(64KiB 상한·시크릿 무)
```

주의(r2 L1): `sshHint`는 프로바이더 **기본 힌트**(표시 보조)이지 legacy `SSHUser/SSHPort` 관측값의 매핑 대상이 아니다 — 두 legacy 필드는 mapping.md에서 dropped(display-only)로 처분되고 §15 비교 대상에서 제외된다.

### 3.3 백필·자격 (PR 29)

```text
RunCloudAccountBackfill(ctx, db) (CloudBackfillReport, error)
  Report{Created/Updated/Unchanged/MarkedStale/Skipped/CollapsedShared/
         FinopsLinked/Failed int, FailedSources []string, Checkpoint time}
  규칙: (1) provider 정규화 매핑 {alicloud→aliyun, tencentcloud→tencent} —
            asset_cloud_account와 finops 계정 양측 동일 적용
        (2) 자격 재질: 컬럼 envelope 복호 → JSON 조립 → EncryptSecretV2 재봉인
            (평문 잔재는 classify→봉인 — sourceKubeSecret 승계·UNKNOWN은 그 계정
             halt+검역 리포트. 복호 평문은 백필 프로세스 메모리 한정 — 저장·로그 0)
        (3) finops collapse: (양측 정규화 provider, accessKey 평문) 일치 →
            동일 SecretRef 공유. accessKey 일치·secretKey 상이 → billing 바인딩
            별도 SecretRef(§7.4). provider/accessKey 불일치 → 별도 체인
        (4) 체크포인트 source_updated_at — 재실행은 변경행만 (§5.4a)
        (5) v1 소멸 행 stale_source 마킹 (§5.4b)
        (6) 오류 격리: 계정 단위 실패는 Failed/FailedSources에 기록 후 잔여
            파이프라인 계속 (k8s 백필의 즉시 반환과 차별 — 판정 J6)
broker 소비(J5): Resolve(ctx, connUID, "billing").Value → JSON →
  syncFinOpsAccountMonth 인자 로컬 사본에만 주입(원행 무수정·말미는 좁은 Updates)
```

### 3.4 비교 CLI (PR 30 — §15 클라우드 변형)

```text
ops-admin compare-inventory --cloud-account <id> [--data dir]
  legacy 측: Service.CaptureCloudInstancesForCompare(id) — fetchCloudInstances 래퍼
  v2 측:     해당 계정 커넥션의 최신 완료 generation compute.vm 투영
  페어 규칙: v2 sync 완료 직후 캡처·(legacy_ts,v2_ts) 기록·|delta|<=60s 평가
  아티팩트: data/compare/cloud/<account>/<date>/<time>.json (§15.1 규격 승계 —
            캐시 없음·실행마다 신규 파일. interim 시 최상위 필드 "interim":true — §13-10)
ops-admin compare-inventory --gate --cloud-account <id>   (3일 3회 게이트 평가)
```

### 3.5 API/UI (PR 30)

```text
GET /api/v2/infra/resources?kindPrefix=compute.  (기존 kind 파라미터 유지·병립)
GET /api/v2/infra/tasks?page=&pageSize=&status=  (carryover — §16.2 갭 메움.
  등록: RegisterOperations — Register GET 집계(exactly-four) 밖)
메뉴: {url:/infra/compute, value:infra:compute, icon:Grid} — infra 그룹 4→5항
```

---

## 4. 임계경로 식별 (직렬) + **시퀀스 제약 (r2 — F6)**

```text
J8(R2 v2 증명기구) ─┐
J1/J2 어댑터(aliyun→tencent 병행 가능, 하니스 통과가 합류점) ─→ M1 등록(compose)
                                          │
J4/J6 백필+step0006 ─→ J5 finops rewire ─→ 싱크 통합(어댑터×브로커×SyncRunner)
                                          │
J7 비교 CLI·cloudcompare ─→ (E-1 실자격 대기: 게이트 ② 페어 창구)
J9 UI+tasks 라우트 ─→ slice-b E2E (게이트 ③)
```

- 최장 직렬: 어댑터(aliyun) → 등록 → 백필 → 싱크 통합 → 비교 → UI/E2E. tencent 어댑터·finops rewire·UI 뷰는 병렬 가능.
- **실자격(E-1)은 임계경로 밖의 대기 조건** — 코드 전량을 Path M으로 완성한 뒤 실자격만 대기(블로커는 게이트 종결·M1 선언).
- **CLI 파일 freeze (r2 — F6)**: Phase 2 게이트가 §15 페어 아티팩트 생성을 진행 중이다(p2-state 실측: 게이트 ①-④ "1일차 pass·2·3일차 cron 예약" — 종결 예정 2026-09-09). 페어는 `main_compare.go`(M6 소유)·`main_sync.go`(M5 소유)의 CLI를 집행하므로, **두 파일의 main 머지는 Phase 2 게이트 종결(09-09 아티팩트 3회 완료) 전까지 금지**한다. 구현·검증은 브랜치에서 자유롭게 진행(로컬 빌드가 게이트 프로세스와 무관), 머지 순서만 "게이트 종결 확인 → E1/C1 머지"로 고정. 게이트 창 중 CLI 재빌드·재시작으로 아티팩트 일관성이 깨지는 사고를 차단하는 조치.

---

## 5. Phase 분해 (≤5파일·독립 검증 단위 — r2: D/E 2분할씩)

| Phase | 소관 | 파일 (≤5) | 검증 |
|---|---|---|---|
| **A (PR 27)** | aliyun 어댑터 | N1–N5 + M1/M2(aliyun분) | `go test ./internal/infra/adapter/aliyun/ -race` — 하네스 4단얫 GREEN·httptest 시나리오 |
| **B (PR 28)** | tencent 어댑터 | N6–N10 + M1/M2(tencent분) | 대칭 — 하네스 GREEN. A와 병렬 가능(M1/M2만 합류점 — 순차 커밋) |
| **C1 (PR 29a)** | 백필·자격 통합 | N11–N13 + M3 + M5 | `go test ./internal/infra/inventory/ ./internal/infra/migrate/ -race` — 멱등·collapse 3분기·stale·격리·migration-test baseline 갱신 GREEN. **M5 main 머지는 §4 freeze 준수** |
| **C2 (PR 29b)** | finops rewire | M4 (+기존 finops 테스트 파일 추가 단얫) | `go test ./service/ -run FinOps -race` — 링크/비링크 2경로·스케줄러 무변경 단얫·**자격 컬럼 재기입 부재 단얫(좁은 Updates)** |
| **D1 (PR 30a·커밋1)** | API — kindPrefix·tasks | M7–M9 (3파일) | `go test ./internal/api/v2/` — exactly-four 무변경 통과·kindPrefix·tasks 목록 단얫 |
| **D2 (PR 30a·커밋2)** | UI 뷰·메뉴 | N16 + M12–M15 (5파일) | `bun run build` + i18n 파리티 + 메뉴 시드 upsert |
| **E1 (PR 30b·커밋1)** | 비교 함수·CLI | N14–N15 + M6 + M10 (4파일) | cloudcompare 단얫(11필드 처분)·mock 경로(E-2) 리포트 아티팩트. **M6 main 머지는 §4 freeze 준수** |
| **E2 (PR 30b·커밋2)** | R2 v2·카나리·오버라이드 | M11(2파일) + M16(2파일) (4파일) | `bash scripts/check-arch-boundary.sh` PASS·카나리 플레인트 FAIL 발화(제품·_test 양측)·route 골든 재생성·env 게이트 단얫(GO_ENV별 2케이스) |
| **F (PR 30c)** | E2E·seed·종결 | N17 + seed.yaml(sts/ds) + 게이트 아티팩트 | Playwright slice-b 2 passed·(E-1 해소 시) 실계정 페어 1회차 |

r2 재분해 사유: r1의 D(8파일)·E(7파일)는 ≤5 위반. D1(백엔드 API)·D2(프런트)·E1(비교)·E2(증명기구·오버라이드)는 각각 독립 검증 가능(검증 명령이 다름). F는 E-1 상태와 무관하게 착수 가능(시드 기반) — 실계정 페어는 별도 창구.

---

## 6. 테스트 계약 (TDD — §23)

```text
계약(contracttest)  어댑터 2종 RunContractSuite — 페이징(종결·무중복·총수==시드)·
                    에러 분류(신호 3종)·rate-limit 카운터·redaction 카나리
유닛+통합           normalizer(URN 유니크·nil-safe 필드)·client(서명 왕복 — httptest가
                    재서명 검증까지 겸)·backfill(멱등 3회·collapse 3분기·UNKNOWN halt·
                    stale·계정 단위 격리)·finops rewire(링크→브로커 경로·비링크→기존·
                    스케줄러 파라미터 무변경·**싱크 후 자격 컬럼 무변경 단얫 — UPDATE
                    문이 3컬럼을 포함하지 않음을 SQL 관점에서 검증 + claim 16 DB 단얫**)
                    ·cloudcompare(BLOCKER/VOLATILE/DRIFT/ABSENT·QuantityEpsilon·
                    11필드 처분 완결)
E2E                 slice-b.spec.js — 시드 compute.vm 행 테이블 렌더·kindPrefix 요청·
                    상세 드로어·tasks 목록 페이지(404 해소). **범위 한계 명시(r2 M6):
                    게이트 ③(UI V2 읽기) 증명이 목적 — 실계정 discovery 회로(①)·§15
                    페어(②)는 커버하지 않는다. ①②의 보충 검증은 실계정 창구(claim
                    8-10·Validate/Health 스모크·rate 카운터 관찰)가 담당**
음성                미권한 kindPrefix 접근 불필요(GET 비민감)·대신 카나리: 코어 분기
                    플레인트(제품 코드) FAIL·_test.go 플레인트 통과(제외 규칙 자기
                    검증)·백필 UNKNOWN halt·브로커 billing 미바인딩 에러
```

기존 테스트 무변경 — 신규 파일·기존 파일 추가 단얫만. **명시적 예외 2건(골든 재생성과 동일 취급·커밋 메시지에 근거)**: (a) authz·route-inventory 골든·migration baseline 재생성(게이트 계약의 갱신), (b) **compose_test.go:41-42 등록 수 단얫 2→4 갱신(M2 — 어댑터 등록의 필연, r2 H3)**.

---

## 7. 검증 요구 (claims — 게이트 3조건 로컬 증명 명령)

1. `cd backend && go test ./internal/infra/adapter/aliyun/ ./internal/infra/adapter/tencent/ -race -count=1` — 하네스 4단얫 2어댑터 GREEN.
2. `cd backend && go test ./... -race -count=1` — 전 패키지 GREEN(기존 회귀 0).
3. (게이트 ①-스키마) `git diff main -- backend/internal/infra/model/provider.go backend/internal/infra/model/inventory.go backend/internal/infra/model/secretref.go backend/internal/infra/model/task.go` — **빈 diff** (V2 13테이블 모델 무변경). 스키마 신규는 `git diff main --stat -- backend/internal/infra/migrate/` — step0006 1파일뿐(v1 EXTEND).
4. (게이트 ①-분기) `bash scripts/check-arch-boundary.sh` — R1/R2 v2/R3 전부 PASS(코어 제품 코드 식별자 0 — 어휘 예외 라인·_test.go 제외만 통과).
5. (게이트 ①-반증) arch-boundary-canary 잡 — 코어 플레인트(`if conn.ProviderType == "aliyun"`, 제품 코드) FAIL 발화 로그 + _test.go 플레인트 통과(제외 규칙 검증).
6. `docker exec ops-admin-mysql-dev mysql -uroot -p*** ops_admin -e "SELECT pc.provider_type, pcb.purpose, COUNT(*) FROM provider_credential_binding pcb JOIN provider_connection pc ON pc.id=pcb.provider_connection_id GROUP BY 1,2"` — 백필 후(시드 행) inventory·billing 목적 행 존재·공유 시 동일 secret_ref_id(`SELECT purpose, secret_ref_id …` 병기).
7. (게이트 ③-전제) `cd web && bun run build && node scripts/check-i18n-parity.mjs` — 빌드·파리티 GREEN.
8. (게이트 ③) `cd web && bun run e2e` — slice-b 스펙 passed(compute.vm 시드 행 렌더·상세·tasks 목록). 백엔드 `curl '.../api/v2/infra/resources?kindPrefix=compute.'` items>0·`/api/v2/infra/tasks` 200(404 해소 — carryover).
9. (게이트 ② 준비) `go run . compare-inventory --cloud-account <id>` — mock 경로(E-2 승인 시) BLOCKER 0 리포트 아티팩트 생성·`--gate` 평가 동작.
10. (게이트 ①② 정식 — E-1 해소 후) 실계정: v1 계정 등록 → `go run . sync-inventory --connection <cloud-uid>`(백필+싱크+카운터) → claim 9 페어를 **서로 다른 날 3회** — 3회 전부 zero BLOCKER(§15.4). M1 선언은 여기까지 완료 후. **승격 절차(§13-10) 준수: 첫 페어 전 어댑터 Validate/Health 스모크를 선행(서명 3중 병존 상태의 이탈 조기 발견 — R2 완화).**
11. `git diff main -- backend/go.mod` — **빈 diff** (신규 의존 0).
12. `git diff main -- backend/service/integration_finops.go | grep -c "initFinOpsScheduler\|time.NewTicker"` — 스케줄러 블록 무변경(0 변경 라인·rewire는 SyncFinOpsAccountMonths 내 자격 해석 + 말미 좁은 Updates만).
13. (자격 비로그 — r2 3경로+rewire 확장) `grep -rn "accessKey\|secretKey" backend/internal/infra/adapter/aliyun/ backend/internal/infra/adapter/tencent/ backend/internal/infra/inventory/backfill_cloud.go --include="*.go" | grep -i "log\|print\|error"` — **0행**. 병기: `git diff main -- backend/service/integration_finops.go backend/main_sync.go | grep "^+" | grep -iE "log\.|print|fmt\.print"` — 평문 변수를 로그에 넘기는 +라인 0행.
14. (r2 — 스캐너) `python3 scripts/secret-scan.py; echo $?` — **exit 0**(`PASS: secret-scan clean` 라인). 어댑터 테스트·하네스가 사용하는 모의 자격 리터럴이 RULES(aws-secret-assignment·password/secret/token 계열)에 걸리는 경우, **allowlist 항목을 구현 전 계획 단계에서 사전 명시한다** — 형식은 value∥path glob(선례: secret-scan.py:105-108 secret_guard_test 4항목). 예정 항목: `("<모의 AccessKeyID 리터럴>", "backend/internal/infra/adapter/**")`·`("<모의 SecretKey 리터럴>", "backend/internal/infra/adapter/**")` — 리터럴 값은 하네스 Fixture 확정 시 본 claim에 갱신 기록 후 커밋. 테스트가 아닌 제품 코드 경로에는 어떤 allowlist도 부여하지 않는다.
15. (r2 — 신규 패키지 커버리지) `cd backend && go test ./internal/infra/adapter/aliyun/ ./internal/infra/adapter/tencent/ -cover` — **신규 어댑터 2패키지 로컬 커버리지 각 ≥80%** (opdef 래치(§0.7-5)와 별개 관리 — 래치 대상 아님).
16. (r2 — 자격 컬럼 byte 불변, 게이트 C2) 링크된 finops 계정 싱크 실행 전후:
    `docker exec ops-admin-mysql-dev mysql -uroot -p*** ops_admin -N -e "SELECT MD5(CONCAT_WS('|',access_key,secret_key,billing_token)) FROM integration_finops_account WHERE provider_connection_uid IS NOT NULL"` — **전후 해시 동일**(좁은 Updates가 자격 3컬럼을 기입하지 않음의 DB 단얫 — 판정 J5).

---

## 8. 보존 제약 (③구현 프롬프트에 verbatim 복사)

1. **v1 클라우드 뷰·동작 무변경**: `service/asset_cloud_aliyun.go`·`util/tencentcloud.go`·`service.go`의 v1 동기화 경로(SyncAssetHosts·fetchCloudInstances 본체)·라우트는 무접촉. 예외는 3건뿐 — (a) `CaptureCloudInstancesForCompare` exported 래퍼 추가(신규 함수, 기존 함수 미수정), (b) E-2 승인 시 env 엔드포인트 오버라이드 훅(**기본 미설정·GO_ENV=development 게이트 — env 없거나 게이트 미충족이면 기존 동작과 바이트 단위로 동일 경로**), (c) `UpdateAssetCloudAccount` 자격 변경 감지 시 inventory 바인딩 해제 1줄(판정 J5-3).
2. **finops 스케줄러 무변경 + 평문 재기입 금지**: `initFinOpsScheduler`·티커 주기·`SyncFinOpsAccount` 트리거 계약은 무접촉. rewire의 `SyncFinOpsAccountMonths` 변경은 2곳만 — (a) 시작부 자격 해석(브로커 billing → JSON 파싱 → **`syncFinOpsAccountMonth`에 넘기는 로컬 복사본에만 주입 — 함수 원변수·원행 무수정**), (b) 말미(:411-414) 전체 `Save(&account)`를 `Select("last_sync_at","next_sync_at")` 좁은 Updates로 변경(**자격 3컬럼 재기입 차단 — r2 승인 예외**). 비링크 계정은 기존 행 직독 폴백 유지.
3. **자격 평문 취급 (§4.4 — r2 일반화)**: 백필의 자격 재봉인은 복호→JSON 조립→EncryptSecretV2 1회 완결. **finops rewire가 브로커에서 꺼낸 평문도 포함해 모든 자격 평문은 프로세스 메모리 외 어디에도 기록 금지** — 로그·에러 메시지·감사·task_event·API 응답·테스트 출력 전부 미포함(claim 13 — 어댑터 2패키지·backfill_cloud·rewire diff 3경로+병기 grep). SecretRef 외 신규 저장 위치 금지. 링크 싱크 후 `integration_finops_account` 자격 컬럼은 byte 불변(claim 16).
4. **신규 런타임 의존성 0**: `backend/go.mod` 무변경(§10.2). Tencent는 in-tree TC3 구현 또는 기존 SDK(common+cvm) 재사용만.
5. **코어 스키마·분기 금지 (§25)**: V2 13테이블(+감사 확장)의 모델·마이그레이션 변경 금지 — 클라우드 어댑터에 필요하면 설계 실패로 에스컬레이션. 코어 패키지(contract·registry·inventory·secrets·model·policy·metrics·tasks·api/v2)의 **제품 코드**(_test.go 제외)에 provider 식별자 분기 추가 금지 — R2 v2가 거부한다.
6. **기존 테스트 무변경**: 기존 단얫·기대값 변경 금지 — 신규 파일·추가 단얫만. **명시적 예외 2건: (a) route-inventory/authz 골든·migration baseline 재생성(근거 커밋), (b) compose_test.go:41-42 등록 수 단얫 2→4(어댑터 등록의 필연 — r2 승인)**.
7. **어댑터 결계 (arch rule 2)**: 어댑터는 `req.Connection`만 읽는다 — DB·브로커·service 패키지 import 금지. 자격은 `Material["inventory"]`(JSON blob)로만.
8. **CI 기존 10잡 무변경**: v2-ci.yml 수정은 arch-boundary-canary의 클라우드 분기 플레인트 추가만(E-4 승인분). merge 판정은 로컬.
9. **게이트 자산 보호**: kind 클러스터 v2-p2/v2-p3·phase2 비교 아티팩트 무변경. 클라우드 게이트 창구 중 계정 리전 설정 변경 금지(phase2 게이트 윈도우 지침 승계). **`main_sync.go`·`main_compare.go`의 main 머지는 Phase 2 게이트 종결(2026-09-09 예정) 전 금지(§4 freeze).**
10. **§7.7 이연 테이블 신설 금지·carryover 외 범위 금지**: 본 계획 배치표(§2) 외 파일·vm 외 리소스 family(E-3)·관계 링크(이월 #3) 착수 금지.

---

## 9. 리스크 매트릭스 (가능성 × 파급) + 완화

| # | 리스크 | 가능성 | 파급 | 완화 |
|---|---|---|---|---|
| R1 | 실계정 미확보로 게이트 ①②·M1 선언 지연 | **높음** | 높음 | E-1 최상단 에스컬레이션·Path M으로 코드 전량 선완성(실자격 도착 시 명령 3개)·interim 마킹 옵션(E-1b·§13-10 규격) |
| R2 | TC3/RPC 서명 직구현 오류(실계정에서만 발현) | 중 | 높음 | httptest가 서명 재검증 겸함(응답 헤더·canonical 요청 단얫)·**실계정 승격 절차의 선행 필수로 Validate/Health 스모크(§7 claim 10·§13-11)**·SDK 병행 허용(J2) |
| R3 | 백필 자격 재봉인 실패(UNKNOWN·복호 불능) | 중 | 높음 | sourceKubeSecret 규칙 승계 — UNKNOWN은 **그 계정 halt**+검역 리포트(§4.3)·계정 단위 격리로 잔여 파이프라인 지속(J6)·현재 DB 0행이라 실데이터 리스크는 실계정 등록 시점으로 국한 |
| R4 | finops 이중 전환 혼재(링크/비링크 계정) 오동기·**평문 재기입** | 중 | 중 | 폴백 경로 명시적 유지·2경로 테스트·링크 백필 멱등·스케줄러 무변경·**J5 3층 차단(로컬 복사본·좁은 Updates·claim 16 byte 불변 단얫)** |
| R5 | R2 v2 위음성(어휘 예외 라인 남용·compose 제외·_test.go 제외 남용) | 낮음 | 높음(게이트 ① 증명 신뢰) | 예외는 심볼 핀포인트·카나리 플레인트로 반증 가능성 유지(제품·_test 양측 발화)·compose/_test 제외 사유 주석+본 계획 문서화 |
| R6 | route-coverage 골든 드리프트·migration baseline 미갱신 CI 적색 | 높음 | 낮음 | Phase D1/E2 검증 항목에 골든·baseline 재생성 커밋 명시(근거 기록) |
| R7 | kindPrefix가 "코어 변경"으로 게이트 ① 논쟁 | 낮음 | 중 | 읽기 쿼리 파라미터 — 스키마·분기 아님(J9 사유 문서화)·필요 시 kind 다중 조회로 대체 가능 명시 |
| R8 | Tencent 응답 필드 nil(포인터) 파닉·누락 | 중 | 중 | normalizer nil-safe 전면·하니스 Seed가 nil 필드 시나리오 포함 |
| R9 | 실계정 리전 대량(DescribeRegions 전역 순회 슬로우) | 중 | 중 | 계정 regions 설정 우선(J3)·커서 페이징으로 run 부분 완료 허용(§9.2)·rate 카운터로 예산 관찰 |
| R10 | E-2 미승인으로 legacy 캡처가 mock 미도달 — 비교 interim 공백 | 중 | 중 | 순수 함수+V2 단측 실증으로 격하·게이트 ②는 실계정 창구로 일원화(E-1과 운명 공동) |
| R11 | Phase 2 게이트 창구 중 CLI 머지로 아티팩트 일관성 붕괴 | 낮음 | 중 | §4 freeze — M5/M6 main 머지는 09-09 게이트 종결 후(스폰별 머지 체크리스트에 명시) |

---

## 10. CI 로컬 판정 방침

- merge 판정: **로컬** — `go test ./... -race`·`bun run build`·본 계획 claim 명령. 원격 CI 대기 금지(p3 관례 승계).
- CI 변경은 E-4 승인분 1건(arch-boundary-canary 클라우드 분기 플레인트) — 게이트 ① 증명기구의 상시 증거. slice-b E2E는 기존 slice-a-e2e 잡이 커버하지 않음 — **신규 잡 추가 없음**(로컬 판정·p3 승인 컨텍스트 유지; 상시 증거 요구 제기 시 별도 승인).
- 라이브 클라우드 자격을 CI에 두지 않는다(스펙 준수 — p3 "라이브 크레덴셜 도입 안 함" 승계). claim 14 secret-scan을 merge 전 필수 관문으로 편입(모의 리터럴 allowlist 사전 명시 분은 제외 대상 아님 — 경로가 제품 코드가 아니기 때문).

---

## 11. 롤백 가능성 판정 (r2 — PR 29 런북 상세화·step0006 runbook 추가)

- **PR 27/28 (완전 롤백 가능)**: 어댑터 패키지 삭제 + compose 등록 라인 제거 — 코어 무영향. 지속물 없음.
- **PR 29 (4단계 런북 — r2 F11)**:
  1. **링크 NULL화(최소 조치·즉시 폭발력 제거)**: `UPDATE integration_finops_account SET provider_connection_uid=NULL WHERE provider_connection_uid IS NOT NULL` — M4 revert 없이 폴백이 구경로 복귀(rewire가 빈 링크에서 봉인 컬럼 직독).
  2. **SecretRef 삭제 순서(참조 방향)**: `provider_credential_binding` 먼저 → `secret_ref` → `provider_context`/`provider_connection`(source_model='asset_cloud_account'). 바인딩이 SecretRef를 지목하므로 역순 삭제는 FK 잔류 오류.
  3. **골든/fixture 역재생성**: migration baseline fixture를 step0006 반영 전 커밋으로 revert(역재생성 커밋 — 근거 메시지). route 골든은 PR 29가 건드리지 않음(무동작).
  4. **revert 순서**: C2(finops rewire) → C1(백필·step0006) — 의존 역순. C1이 C2보다 먼저 revert되면 링크 열만 남은 백필 산출물과 rewire가 어긋난다.
  - **step0006 실패 runbook (r2 F5)**: 스텝 실패 시 러너가 dirty 행을 남기고 기동을 거부한다(`migrate/runner.go:136-138` — "repair or drop the torn DDL, delete its dirty schema_migration row (version %d), then re-run; automatic recovery is forbidden (R1)"). 절차: 찢어진 DDL을 수리 또는 드롭 → `DELETE FROM schema_migration WHERE version=<step0006>` → 재실행. 자동 복구 스크립트 금지.
  - **운영 런북 (자격 변경 — r2 F2)**: v1 화면에서 클라우드/finops 계정 자격을 교체하면 링크가 자동 해제된다(SaveFinOpsAccount·UpdateAssetCloudAccount 전파) — 이후 `go run . sync-inventory`를 재실행해 체인·링크를 재생성(증분이라 변경 행만).
- **PR 30**: additive 뷰·라우트·메뉴 시드 upsert(재실행으로 소거)·비교 CLI·스크립트 — revert로 종결. tasks 라우트 제거 시 UI 폴백은 기존 404 상태로 복귀(개악 아님).
- 게이트 실패 시 착구간 복귀: Phase 역적용(F→A). A/B(어댑터)와 C(자격)는 독립 롤백 가능.

---

## 12. 파일 소유권 — 구현 스폰 분리

| 스폰 | 소유 | 접근 금지 |
|---|---|---|
| impl-P27 (임계) | N1–N5 + M1/M2 aliyun분 | tencent 패키지·inventory 하위 |
| impl-P28 (임계) | N6–N10 + M1/M2 tencent분 | aliyun 패키지·inventory 하위 |
| impl-P29a (임계) | N11–N13 + M3 + M5 (C1) | service 하위·api·web. **M5 머지는 §4 freeze 대상** |
| impl-P29b | M4 (+finops 테스트 추가 단얫) (C2) | internal/infra 하위(브로커 인터페이스만 소비) |
| impl-P30a | M7–M9 (D1) + N16·M12–M15 (D2) | backend service·scripts |
| impl-P30b (임계) | N14–N15 + M6 + M10 (E1) + M11·M16 (E2) | internal/api·web 뷰. **M6 머지는 §4 freeze 대상** |
| impl-P30c | N17 + seed.yaml | 프로덕션 코드 전부(테스트·fixture만) |

---

## 13. 산출 외 확인사항 (이월 대장 증보 — r2: 9·10·11 추가)

1. **vm 외 리소스 family (E-3 승인분)**: disk·snapshot·eip/load balancer·vpc/switch·security group·image — §14.2 목록 잔여. 재입구: M2 cutover 전 또는 운영 요구 명명 시. Tencent는 SDK 모듈 부재 — TC3 직구현 경로(J2)가 전제.
2. **§18.2 메트릭 앵커 잔여 (M1 선언 전 필수 — p3 이월 승계)**: Phase 4는 provider_api_errors_total·provider_rate_limit_total의 클라우드 라벨 발화만 완료. provider_health·inventory_sync_duration·worker_queue_depth·secret_access_total·resource_stale_total 등 전면 계측 + `/internal/metrics` 노출은 **M1 선언 준비 태스크** 소관 — 본 대장이 추적 지점.
3. **커버리지 래치**: 실측 정정 — 이미 구현됨(v2-ci backend-test "opdef coverage latch baseline 65.4, gate 60.0"). **래치는 opdef 커버리지에 대한 것 — Phase 4는 opdef를 추가하지 않아 수치 불변.** 신규 어댑터 패키지는 claim 15(≥80% 로컬)로 별도 관리(§0.7-5).
4. **normalizer 관계 링크·generation/annotation 투영**: K8s family — M2 cutover 전 재검(§0.7-3/7). reconcile.go의 kind별 raw 판독 패턴은 관계 링크 확장 시 코어 분기 논점과 함께 재설계.
5. **POST 3종 + GET 3종 미구현 엔드포인트(phase3 §13-8 승계)**: `POST /provider-connections`·`validate`·`sync`·`GET /provider-connections/{uid}`·`GET /provider-contexts`·`GET /resources/{uid}/relationships` — 클라우드 계정 수명주기 UI 수요 시. Phase 4는 백필+CLI로 충족(계정 등록은 v1 화면).
6. **stale_source 상세 노출**: v1 클라우드 계정 삭제가 실제 발생하는 시점(실계정 운용 개시)에서 응답 형상 정의.
7. **incremental 싱크 모드**: phase2 §3.3 이월 승계 — 본 Phase는 full 모드(§20 Phase 4 게이트 무요구). 클라우드 리전 대량 계정에서 필요 시 재검.
8. **R2(phase3) 크래시 인젝션 안정화**: 플레이크 관찰 시에만 착수(§0.7-4).
9. **asset_cloud_account 평문 P-class 잔존 (r2 F4)**: `asset_cloud_account.access_key`·`secret_key`는 **ClassPlaintext**(util/secret_registry.go:63-64, row 6 — Phase -1 Step 1이 봉인 전환 대상에서 제외한 P-class). 본 Phase 백필은 **복사·재봉인일 뿐 v1 테이블의 평문을 소거하지 않는다** — §5.4 전파는 v1→v2 단방향이며 v1 데이터 폐기는 **M2 cutover**(v1 경로 폐기 시점) 소관. 백필 평문 분기 명시: 복호 평문은 백필 프로세스 메모리 한정(재봉인 직후 버림)·UNKNOWN/불능 행은 계정 단위 halt+검역 리포트(J6)·저장·로그 경로 0(보존 제약 3). 현재 0행 — 잔존 리스크는 실계정 등록과 함께 시작되며 본 항목이 추적 지점.
10. **실계정 증명 pending (interim) — 아티팩트 마킹 규격·승격 절차 (r2 F8)**: E-1(b) interim 수락 시 게이트 ①② 아티팩트는 interim 상태임을 기계 판별 가능하게 마킹한다. **규격(택일 — 필드 방식 채택)**: 비교 아티팩트 JSON 최상위에 `"interim": true` 필드 1개(파일명 접미 방식은 §15.1 규격 파일 체계를 침해하므로 기각). **승격 절차**: 실자격 제공 → 어댑터 Validate/Health 스모크(선행 필수 — §13-11) → claim 10(3일 3회 페어, A10 기산 규칙) 재실행 → interim 필드 없는 아티팩트 재생성 → M1 선언. interim 상태에서 M1 선언 불가(E-1(b) 기록 승계).
11. **서명 구현 3중 병존 (r2 F9)**: Phase 4 완료 시점에 클라우드 API 서명 구현은 3벌이 병존한다 — v1 aliyun RPC 원본(service)·v1 tencent SDK(util)·어댑터 TC3/RPC 재구현(adapter). 중복은 의도적(v1 무변경 보존 제약 1의 대가)이며 **M2 cutover에서 v1 경로 폐기 시 2벌로 수렴**. 병존 기간 이탈(어댑터와 v1이 다른 결과 반환) 조기 발견을 위해 **실계정 승격 절차의 선행 필수로 Validate/Health 스모크를 지정**(§7 claim 10·R2 완화) — 스모크 없는 페어 착수 금지.

---

## 14. 가정 명세 (불확실 요소의 명시적 처리 — r2: A7 정정)

| # | 가정 | 검증 시점 |
|---|---|---|
| A1 | E-1 승인 경로: (a) 실자격 제공 또는 (b) interim 수락 — 어느 쪽이든 코드 전량은 동일 | 계획 승인 시 |
| A2 | URN 형식 `urn:<provider_type>:<context_id>:<kind>:<external_id>` — k8s normalizer 관례 승계 | Phase A 매핑 리뷰 |
| A3 | `ProviderConnection.Endpoint` — 클라우드는 기본 빈 문자열(not null 충족)·mock/오버라이드 시에만 값 | Phase A/C1 |
| A4 | SecretRef 재질 JSON 키 `{accessKey, secretKey}`(+선택 billingToken) — 브로커 Value 단일 문자열 계약 준수 | Phase C1 |
| A5 | E-2 env 명: `OPS_ADMIN_CLOUD_ENDPOINT_OVERRIDE_ALIYUN`/`_TENCENT` — GO_ENV=development 게이트 하에서만 활성·미설정 시 운영 엔드포인트 | Phase E2 |
| A6 | Tencent TC3 직구현이 CVM DescribeInstances를 표준 헤더(X-TC-Action/Version/Timestamp/Region)로 수행 — finops 구현과 동일 규약 | Phase B httptest |
| A7 | finops collapse 매칭은 백필 프로세스 내 복호 평문 비교(**양측 provider 정규화 후** provider+accessKey) — 저장·로그 없음. **매칭 우선순위(r2 정정): 자격 전부 일치 → 동일 SecretRef 공유+링크. accessKey 일치·secretKey/billingToken 상이 → billing 바인딩만 별도 SecretRef(§7.4 자격 상이 공유 불가)·링크는 커넥션 재사용. provider/accessKey 불일치 → 별도 체인·링크. 어느 분기에서도 평문은 영속화되지 않는다** | Phase C1 |
| A8 | `infra:compute` 메뉴 Value는 시드 권한 체계와 무충돌(GET 비민감 — infra:resources와 동일 취급) | Phase D2 |
| A9 | `kindPrefix` 읽기 파라미터는 게이트 ① "코어 스키마 변경" 해당 없음(쿼리 파라미터·마이그레이션 무관) | 계획 승인 시 |
| A10 | 실계정 페어 3일 창은 E-1 해소일부터 기산(압축 금지 — phase2 E-3 승계). interim 승격 시에도 동일 규칙 재적용(§13-10) | 게이트 ② 창구 |
