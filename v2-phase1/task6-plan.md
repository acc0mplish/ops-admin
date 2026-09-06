# V2 Phase -1 · Task 6 계획 — CI 베이스라인 (7 required checks + 게이트 자기검증 카나리)

- 원천: `/tmp/v2-rev/docs/architecture/multi-infrastructure-control-plane-v2.md` **§4.10(CI baseline and its self-verification)** — 7종 required checks + secret-scan·route-coverage 카나리("발화하지 않는 카나리는 CI를 실패시킨다").
- 범위: CI 워크플로 2종(신규 1 + 기존 1에 잡 추가), 검사 스크립트 2종(신규), 마이그레이션 fixture 테스트 1종(신규 _test.go + 커밋된 sqlite 바이너리 fixture), 문서 1종. **프로덕션 코드·기존 테스트·docs/security 아티팩트 무변경**.
- 실측 기준: main `7705845` 작업 트리, 2026-09-06 전수 확인(아래 §0).
- 개정 이력: r1 → **r2** (team-lead 적대 리뷰 반영 — H1 R3 허용집합 gorm 추가·H2 스캐너 자기배제·H3 keyword+entropy 규칙 추가(89b81b0 사고 클래스 커버)·H4 branch protection 수동 단계+A8·M1 fixture 재생성 남용 가드·M2 R1 패키지 긴장 가정화·L1 gitattributes 표기 통일·L2 A7 근거 축소).

---

## 0. 현재 코드 실측 (계획의 사실 기반)

### 현행 CI — 1개 워크플로 1개 잡
- `.github/workflows/korean-localization-guard.yml`: 잡 `no-hardcoded-chinese` 단일. python3 heredoc CJK 스캔 → `localization-findings.txt` 아티팩트 업로드 → 비면 exit 1. 트리거 `push: branches: [main]` + `pull_request`. 외부 액션은 checkout@v4·upload-artifact@v4만 사용(러너 네트워크 정상 동작 증거).

### 라우트 커버리 3각은 이미 go test에 내장 (신규 로직 불필요)
- `backend/router/routes_inventory_test.go` — `TestRouteInventoryArtifact`(:123): 실제 gin 엔진 덤프 vs 커밋된 `docs/security/route-inventory.txt` **바이트 골든**(437 계약) + `-update` 재생성 플래그. `TestSensitiveRoutesArtifact`(:140): opdef 테이블 렌더 vs `sensitive-routes.txt`(285) 골든 + 선언 순서 셔플 결정성.
- `backend/router/authz_replay_test.go:282` — `TestOperationTableCoversRouter`(T17/CR-3): 모든 opdef가 라이브 라우터에 존재 + 커밋 아티팩트 == 테이블 + **역방향: 비-GET authGroup 라우트는 반드시 opdef 보유**(주석 :277-281가 정확히 "무등재 mutation 라우트" 탐지를 목적 명시). §4.10 "route list vs sensitive-routes.txt vs router" 3방향이 전부 존재.
- `newArtifactEngine`(routes_inventory_test.go:31): sqlite `:memory:` + `store.AutoMigrate` + `store.Seed`으로 실제 엔진 부팅 — **클린 인스톨 경로가 go test만으로 이미 실행됨**.

### 마이그레이션·시드 현황
- `backend/store/migrate.go:11` `AutoMigrate`, `store/seed.go:29` `Seed`. `store/seed_route_permissions_test.go` 6테스트(마커 생성·전 롤 부여·**멱등**·신규 롤 0부여·철회 생존·슈퍼어드민 재부여) — 전부 sqlite `:memory:`.
- 멱등 테스트와 fixture 업그레이드 테스트의 차이: 전자는 동일 DB 인-프로세스 재실행, 후자는 **고정된 파일 바이트에서 재오픈** — 스키마 변경 시점부터 "구버전 데이터 → 신규 스키마" 업그레이드를 증명한다.

### 시크릿 스캔 환경
- `backend/config.yaml` untrack 완료(§4.6, 커밋 `b43aa6d`): `git check-ignore` IGNORED, tracked는 `backend/config.example.yaml`·`deploy/config.yaml.example`뿐. **단 히스토리에 5커밋 존재**(`89b81b0` 등) — 전체 히스토리 gitleaks 스캔은 영구 레드 각태(§6 A4).
- 고정밀 패턴 프리뷰(`AKIA[0-9A-Z]{16}|-----BEGIN …PRIVATE KEY|ghp_|xox[baprs]-` 전수 grep): 유일 히트 `web/src/views/domains/SSLCertificates.vue:141` UI placeholder `"-----BEGIN PRIVATE KEY-----"`(키 본체 없음 — allowlist 대상).
- **이 리포의 실제 사고 클래스는 위 6규칙 어디도 발화하지 않았다**: `89b81b0` 히스토리 `config.yaml`의 `credential-key` 값이 len=44·`[A-Za-z0-9+/=-]` 순수 문자열(형태만 실측, 원문 미기록) — prefix 규칙·PRIVATE KEY 규칙 모두 무관. → keyword+entropy 규칙이 필요(H3, D-2).
- keyword+entropy 위양성 실측(2026-09-06): `(password|secret|token|credential[-_]?key)\s*[:=]\s*["'][A-Za-z0-9+/=_-]{32,}["']` 리포 전수 grep — 유일 히트 `backend/config.example.yaml:27` `credential-key: "REPLACE_WITH_RANDOM_32BYTE_BASE64_KEY"`(placeholder — allowlist 대상). 하이픈 불포함 클래스로는 히트 0건이나, 89b81b0 유형(하이픈 포함 가능) 커버를 위해 값 클래스에 `-`·`_` 포함 확정.

### 그 외
- backend: `go 1.24.0`, `go list ./...` 15패키지(테스트 보유 13, -race 통과 상태 — team-lead 리컨). **opdef 커버리지 실측 65.4%**(2026-09-06 `go test ./opdef/ -cover`).
- web: `package.json` scripts = dev/build/preview 3개뿐(**테스트 러너 부재** — task5 플랜 §0와 동일 실측), `bun.lock` 존재.
- `node scripts/check-i18n-parity.mjs` 실행 **exit 0 "RESULT: PASS"**(2026-09-06 실측). 순수 node(외부 의존 0) — 잡 구성이 간단하다.
- arch: `internal/domain/dnsserver`(코어)는 `model`+외부 라이브러리만 import — **provider 미임포트, `aliyun|tencent` 문자열 0건**(전수 grep). `internal/domain/provider`는 단일 Go 패키지에 인터페이스(`provider.go`)와 제품(`aliyun.go`·`tencent.go`)이 공존 → 패키지 import 경계만으로 제품-인터페이스 분리 불가능. `opdef` 직접 import 실측(`go list -f '{{join .Imports "\n"}}' ./opdef`): **middleware + gin + `gorm.io/gorm` + stdlib**(fmt·net/http·regexp·sort·strconv·strings) — `opdef.Middleware(db *gorm.DB, …)` 시그니처가 gorm을 직접 받는다.
- `docs/security/route-inventory.txt` 마지막 라인은 데이터 라인. `router/router.go:52` 앵커 `authGroup.GET("/profile", ctl.Profile)` — 라우터 첫 authGroup 등록(카나리 주입 지점).
- remote `github.com:acc0mplish/ops-admin.git` — 첫 PR 러너 관찰 가능. `.gitattributes` 존재(linguist 1조항).

---

## 1. 설계 결정 (구현 계약)

### D-1 워크플로 구성 — 신규 `v2-ci.yml` 8잡 + 기존 yml에 1잡
spec "each is a CI workflow"의 실용 구현: GitHub required status check의 단위는 **잡**이다. 7체크 중 l10n-guard만 제외한 6체크(+카나리 2)를 `.github/workflows/v2-ci.yml`에, l10n-guard는 같은 로컬리제이션 도메인의 기존 `korean-localization-guard.yml`에 잡 `i18n-parity`로 추가(파일 분열·트리거 중복 방지; 기존 잡은 바이트 불변 — 보존 제약). 트리거는 기존 관행과 동일(`push: branches: [main]` + `pull_request`, path filter 없음 — required check가 path filter와 결합하면 미대상 PR에서 pending 고착). 잡별 `timeout-minutes`(5–15), `permissions: contents: read`, 루트 `concurrency` cancel-in-progress.

잡 목록(전부 ubuntu-latest). 병합 후 수동 단계: **branch protection required checks에 9개 체크 등록**(v2-ci 8잡 + i18n-parity, `main` 브랜치). GitHub 설정은 리포 파일이 아니므로 Phase 3의 수동 후속 작업이며, 등록 전까지 모든 체크는 advisory로만 동작한다(§6 A8).
1. `backend-test` — setup-go@v5(go 1.24, cache). `go test ./... -race -count=1`. 이어 커버리지 게이트: `go test ./opdef/ -cover` 출력 파싱 ≥ **60.0%**(실측 65.4%의 하향 마진, 문서에 래칫 명시).
2. `frontend-build` — oven-sh/setup-bun@v2. `cd web && bun install --frozen-lockfile && bun run build`(unit tests 부재 → §6 A2).
3. `migration-test` — setup-go. `go test ./store/ -run TestMigrationFixture -count=1 -v`(D-4).
4. `route-coverage` — setup-go. `go test ./router/ -run 'TestRouteInventoryArtifact|TestSensitiveRoutesArtifact|TestOperationTableCoversRouter' -count=1`. **새 검사 로직 0** — 내장 골든 3종의 실행 보장 + required check 이름 가시성이 이 잡의 전부(과잉 워크플로 지양 원칙의 직접 적용).
5. `route-coverage-canary` — D-5.
6. `secret-scan` — `python3 scripts/secret-scan.py`(스캔 + config.yaml 히스토리 가드).
7. `secret-scan-canary` — D-5.
8. `arch-boundary` — `bash scripts/check-arch-boundary.sh`(D-3).

### D-2 secret-scan — 자체 python3 스크립트 (외부 액션 불채택)
`scripts/secret-scan.py [target-dir=.]` — 인자 디렉터리(기본 repo root)를 스캔. **채택 근거(2개)**: ① 카나리가 exit code를 직접 capture해야 한다(외부 액션은 실패가 잡을 즉시 중단시켜 발화 단언 구조가 어색해진다) ② 규칙·allowlist의 버전 고정을 리포가 소유한다. spec 표현은 "gitleaks-style"(도구 미지정)이므로 준수(A7).

- 규칙 v0(고정밀 한정 — 위양성 차단, 확장은 allowlist와 함께 문서화): `AKIA[0-9A-Z]{16}` / AWS secret 할당(`(?i)aws.{0,30}(secret|sk).{0,20}['"][A-Za-z0-9/+=]{40}['"]`) / `-----BEGIN (RSA |EC |OPENSSH )?PRIVATE KEY-----` / `ghp_|gho_|ghs_[A-Za-z0-9]{36}` / `xox[baprs]-` / `sk-[A-Za-z0-9]{20,}` / **keyword+entropy(H3)**: `(?i)(password|secret|token|credential[-_]?key)["']?\s*[:=]\s*["'][A-Za-z0-9+/=_-]{32,}["']`. 값 클래스에 `-`·`_` 포함은 89b81b0 유형(§0 실측: len=44 하이픈 포함 가능) 발화에 필수 — 하이픈 불포함 클래스는 이 리포의 유일한 실제 사고 클래스를 놓친다. 위양성은 실측 완료(§0: 유일 히트 config.example.yaml placeholder). 일반(키워드 없는) 엔트로피 규칙은 v0 의도적 배제.
- **allowlist는 "값 ∥ 경로 glob" 쌍**(`값이 경로 밖에서 발견되면 결함`): v0 항목 — `AKIAIOSFODNN7EXAMPLE ∥ .github/workflows/*`, `wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY ∥ .github/workflows/*`(워크플로 카나리 리터럴 — AWS 공식 문서 예시값으로 실효성 0), `-----BEGIN PRIVATE KEY----- ∥ web/src/views/domains/SSLCertificates.vue`(placeholder), `REPLACE_WITH_RANDOM_32BYTE_BASE64_KEY ∥ backend/config.example.yaml`(keyword+entropy 유일 위양성 — 구현 시 example 파일 전체 placeholder 값을 열거 등재), **스캐너 자기배제(H2)**: `scripts/secret-scan.py`에 등장하는 모든 allowlist 값·규칙 리터럴에 대해 `… ∥ scripts/secret-scan.py` 항목(또는 스크립내 "자기 경로는 값 검사에서 제외" 선언)을 둔다. 경로 기반이므로 카나리 scratch 사본(`scripts/secret-scan.py`의 사본)에서도 동일하게 자기배제가 작동한다. 이 경로 범위성이 카나리 교차(D-5)를 성립시키는 핵심 메커니즘이다.
- **config.yaml 히스토리 가드**: ① `git ls-files`에 `backend/config.yaml`·`deploy/config.yaml`(비 example) 부재 단언 ② `.gitignore` 해당 조항 존재 단언 ③ 트리 스캔 자체. 전체 git 히스토리 스캔은 명시적 거부(A4).
- 스캔 제외: `.git`, `node_modules`, `dist`, `uploads`, 디코드 실패 바이너리(fixture sqlite 포함).

### D-3 arch-boundary — v0 규칙 3종 (현재 전부 그린, 전이 아닌 직접 의존 검사)
`scripts/check-arch-boundary.sh`(bash — go list + grep):
- R1(임포트): `internal/domain/dnsserver`의 **직접** import에 `internal/domain/provider` 금지 — `go list -f '{{join .Imports "\n"}}' ./internal/domain/dnsserver`. `-deps`는 전이 의존까지 포함(opdef→middleware→store 경로가 오경보)되어 부적합.
- R2(제품명): 코어 디렉터리 목록(현재 `internal/domain/dnsserver` 단일 — 스크립트 상수로 관리) 내 `aliyun|tencent` 식별자 grep 금지(spec :140 "core may reference adapter interfaces and capability names but not provider product names"의 오늘 실행 가능 형태).
- R3(opdef 순도): `opdef` 직접 import가 {middleware, gin, **gorm.io/gorm**, stdlib} 외 금지(service/controller/router/store/model 차단 — opdef는 "single source of truth" 순수 테이블 유지). gorm 허용은 필수: `opdef.Middleware(db *gorm.DB, …)`가 gorm 타입을 직접 받는다(§0 실측).
- arch 전용 카나리는 spec이 Phase 0 게이트(§:1594 "canary", §:1660 "arch-boundary rules + canary")로 배치 — 본 과제는 규칙만. 발화 증명은 구현 시 로컬 1회 실증으로 claim(§7-6).

### D-4 migration-test — 커밋된 baseline fixture 업그레이드
- 신규 `backend/store/migrate_fixture_test.go` — `TestMigrationFixtureUpgrade`: ① `OPS_MIGRATION_FIXTURE_UPDATE=1` 환경변수 시 클린 DB에서 AutoMigrate+Seed 후 fixture 재생성(기존 `-update` 골든 관례 준용) ② 통상 모드: `backend/store/testdata/migration-fixture/baseline.sqlite` 존재 단언 → t.TempDir()로 복사 → glebarez sqlite 파일 모드 오픈 → 사전 스냅샷(roles·users·menus 카운트, 시드 마커) → AutoMigrate + Seed → 단언: 에러 없음 / users ≥ 사전 / roles == 사전(중복 시드 아님) / 마커 존속 / menus ≥ 사전.
- fixture는 **오늘 스키마+시드 상태의 동결** — 최초 schema 변경 시점부터 "구버전 → 신규" 업그레이드 게이트로 유효화된다(오늘은 멱등+데이터 보존 증명). DB는 sqlite 한정(A3). `.gitattributes`에 `backend/store/testdata/**/*.sqlite binary` 조항 추가(WSL/Windows 개발 환경 개행 변조 방지).
- **재생성 남용 가드(M1)**: 스키마 변경 PR이 `OPS_MIGRATION_FIXTURE_UPDATE=1` 재생성을 함께 커밋하면 fixture가 신규 스키마로 갱신되어 **업그레이드 증명이 멱등 증명으로 퇴화**한다. 따라서 ① 재생성 커밋은 스키마 변경 커밋과 **별도 커밋으로 분리**를 의무화하고 ② 재생성 커밋은 직전 커밋 시점의 코드로 재생성했음을 메시지에 명기한다. 이 규칙은 Phase 4 문서에 운영 절차로 기록한다.

### D-5 카나리 — scratch 체크아웃 조작 + "발화 안 하면 CI 실패"
공통 패턴: `cp -r "$GITHUB_WORKSPACE" "$RUNNER_TEMP/canary"` → 결함 플랜트 → 검사 실행 → **exit 0이면 잡 실패**(§4.10 "A canary that does not fire fails CI"의 직역).
- **secret-scan-canary**: scratch 루트에 `canary-leak.txt` 생성(`AKIAIOSFODNN7EXAMPLE` + 40자 secret 할당 라인) → `python3 scripts/secret-scan.py "$RUNNER_TEMP/canary"` → 비-0 exit 단언. 같은 값이 워크플로 파일 안(allowlist 경로)에서는 통과하고 scratch 플랜트 경로에서는 발화 — D-2의 경로 범위 allowlist가 양립을 성립(claim 1·2·3의 삼각). **발화 주체 단언(H2)**: 스캐너 출력의 결함 보고 라인이 플랜트된 `canary-leak.txt` 경로를 지정하는지 잡이 grep 검증한다. scratch 사본의 `scripts/secret-scan.py` 자기히트만으로 실패했다면 exit != 0은 지나가지만 **플랜트 검증의 공허 경로**가 된다 — 자기배제(D-2)와 이 경로 단언이 함께 그 헛점을 막는다.
- **route-coverage-canary**: scratch의 `backend/router/router.go`에서 앵커 `authGroup.GET("/profile", ctl.Profile)` 직후 `authGroup.POST("/canary/unlisted", func(c *gin.Context) {})` 주입(sed, GNU) → **주입 성공 grep 단언**(앵커 표류 시 침묵이 아니라 잡 실패) → `go test ./router/ -run TestOperationTableCoversRouter -count=1` → 비-0 exit 단언. 비-GET 무-opdef 라우트가 곧 §4.10의 "known unlisted route"이며, `TestOperationTableCoversRouter`가 정확히 이 결함 탐지 목적의 테스트다(동일 결함이 437 골든·인벤토리 드리프트도 동시 발화).

### D-6 l10n-guard — 기존 yml에 `i18n-parity` 잡 추가
setup-node@v4(node 20) + `node scripts/check-i18n-parity.mjs`(외부 의존 0, exit code 게이트). spec "for touched dictionaries"의 스코핑 대신 **리포 전체 검사**(상위집합 — 스크립트 무변경 원칙下 가장 얇은 실현).

---

## 2. 파일 배치 · Phase 분해 (8파일 → 4 Phase)

| # | 파일 | 신규/편집 | 요약 |
|---|------|-----------|------|
| 1 | `.github/workflows/v2-ci.yml` | 신규 | 8잡(D-1) — 5체크 + 카나리 2 + arch |
| 2 | `.github/workflows/korean-localization-guard.yml` | 편집(append-only) | `i18n-parity` 잡 1개 추가, 기존 잡 바이트 불변 |
| 3 | `scripts/secret-scan.py` | 신규 | 고정밀 규칙 6종 + keyword+entropy + 값∥경로 allowlist(자기배제 포함) + config.yaml 가드(D-2) |
| 4 | `scripts/check-arch-boundary.sh` | 신규 | R1·R2·R3 import/식별자 규칙(D-3) |
| 5 | `backend/store/migrate_fixture_test.go` | 신규(_test.go) | fixture 업그레이드 테스트 + 재생성 모드(D-4) |
| 6 | `backend/store/testdata/migration-fixture/baseline.sqlite` | 신규(바이너리) | 오늘 스키마+시드 동결 — 생성 산출물 |
| 7 | `.gitattributes` | 편집 | `backend/store/testdata/**/*.sqlite binary` 조항 1줄(경로 한정 — 리포 전역 `*.sqlite` 과잉 배제, D-4와 표기 통일) |
| 8 | `docs/security/ci-baseline.md` | 신규 | 7체크·카나리 철학·fixture 재생성 절차(별도 커밋 의무 포함)·allowlist 운영·커버리지 래칫·branch protection 등록 절차 기록 |

- **Phase 1 (2파일 — #3, #4)**: 검사 스크립트. 로컬 검증: main에서 둘 다 PASS + 플랜트 시 둘 다 발화. 워크플로 무관하게 독립 완결.
- **Phase 2 (3파일 — #5, #6, #7)**: 마이그레이션 fixture. 로컬 검증: `go test ./store/ -run TestMigrationFixture` PASS + 재생성 멱등(sha256 동일). Phase 1과 **병렬 가능**.
- **Phase 3 (2파일 — #1, #2 + 수동 단계 1)**: 워크플로. 의존: Phase 1·2 완료 후(잡이 스크립트·테스트 참조). 검증: YAML 파싱, 기존 잡 불변 diff, 각 잡 명령의 로컬 재현, 첫 PR 러너 관찰. 병합 후 수동 후속(H4): **branch protection required checks 9개 등록**(v2-ci 8잡 + i18n-parity) — GitHub Settings UI 또는 `gh api -X PUT repos/{owner}/{repo}/branches/main/protection`으로 설정, 등록 전까지 advisory(A8).
- **Phase 4 (1파일 — #8)**: 문서. Phase 3 확정 잡 구성·실측 임계값 기술(마지막에 두는 이유).

단계 순서(의존관계): P1 ∥ P2 → P3 → P4. 커밋은 Phase별 1개(`ci:` prefix)를 권장하나 롤백 단위는 파일별 revert로도 성립.

---

## 3. 검증 전략 (테스트 없는 과제의 대체 확인 체계)

- **로컬 실증 계층**(구현자가 전부 실행, §7 claims로 증거화): 스크립트 2종 clean/플랜트 실행, `go test ./... -race`, fixture 테스트 + 재생성 멱등, 카나리 시뮬레이션(scratch 사본 조작 후 go test 비-0 exit — 러너 잡이 수행할 명령의 1:1 로컬 재현), 워크플로 YAML 파싱, `bun install --frozen-lockfile && bun run build`.
- **러너 실증 계층**: 첫 PR에서 v2-ci 8잡 + i18n-parary 그린 관찰(claim 15). 병합 전 브랜치에서 카나리 2잡이 스스로를 증명한다는 점이 이 과제의 특수성 — 러너 관찰은 카나리 논리의 최종 확인일 뿐, 발화 논리 자체는 로컬에서 이미 실증된다.
- 커버리지 래칫: 문서에 기준선 65.4%·임계값 60.0%·상향 절차를 기록(자동 상향은 도입하지 않는다 — 얇게).

## 4. 위험 지점과 검증

| 위험 | 내용 | 완화 |
|------|------|------|
| W-1 카나리 앵커 표류 | router.go:52 `/profile` 라인 변경 시 sed 주입 무발생 | 주입 후 grep 단언으로 **CI 레드**(침묵 아님 — 안전 방향). 경미: 해당 라인은 라우터 최초·최소 등록 |
| W-2 secret-scan 위양성 | 신규 규칙(keyword+entropy 포함)이 main 기존 코드 적중 → 게이트 폐쇄 | 고정밀 규칙 + **keyword+entropy 규칙은 리포 전수 위양성 실측 완료(§0: 유일 히트 config.example.yaml placeholder → allowlist 처리)** + 병합 전 main 전수 실행(claim 1). 이후 적중 시 값∥경로 allowlist 추가 절차를 문서화 |
| W-3 러너 -race 소요 미지측 | 첫 실행 전 런타임 불명 | 잡별 timeout-minutes(backend-test 15, 카나리 10, 나머지 5) |
| W-4 컴파일 중복 비용 | backend-test·route-coverage·route-coverage-canary가 go 빌드 3회 | Phase -1 수용(정확성 > 비용), 통합은 Phase 0+ 과제 |
| W-5 fixture 바이너리 변조 | WSL(D: 드라이브) 개발 환경의 개행 오염 | `.gitattributes` binary 조항(Phase 2에 포함) |
| W-6 기존 잡 파손 | korean-localization-guard.yml 편집 | append-only — diff로 기존 잡 블록 바이트 동일 단언(claim 12) |
| W-7 워크플로 문법 오류 | YAML/액션 버전 실수 | python yaml 파싱 + 로컬 명령 재현 + 첫 PR 관찰. `act`는 미설치·미사용(로컬 재현으로 충분) |

## 5. 롤백 가능성 판단

**완전 가역.** 신규 파일 6종 삭제 + 편집 2종(yml, .gitattributes) revert = 사전 상태 복원. 프로덕션 코드·기존 테스트·아티팩트·데이터 무관(전부 무변경). 스크립트·fixture의 유일 소비자가 새 워크플로이므로 도미노 없음. 카나리 실패로 인한 CI 레드도 원인 커밋 revert로 즉시 해소(게이트 자체가 원인이면 Phase 3 revert).

## 6. 가정 (불확실 요소의 명시적 판정)

- **A1**: 신규 `_test.go`(마이그레이션 fixture)와 신규 스크립트는 "CI·스크립트" 허용 범주다(프로덕션 코드 아님 — 바이너리 미탑재). 기존 테스트 무변경 조건과 충돌 없음.
- **A2**: frontend unit test 부재 — spec "production build + unit tests"의 unit 래그는 build로 대체한다. 테스트 러너 도입은 web/package.json 변경을 수반해 본 과제(프로덕션 무변경) 범위 밖.
- **A3**: migration-test는 sqlite(현행 테스트 인프라와 동일). MySQL 서비스 컨테이너는 스키마 변경이 시작되는 Phase 0+로 연기.
- **A4**: secret 전체 히스토리 스캔 불가 — untrack 이전 `backend/config.yaml` 5커밋이 존재하여 영구 레드. 히스토리 가드는 "재추적 금지 + .gitignore 단언 + 트리 스캔"으로 실현한다. 이전 이력의 실시크릿 여부는 T1 키 로테이션이 근본 대응(히스토리 갱신 아님).
- **A5**: GitHub-hosted ubuntu 러너, actions/*·setup-bun 네트워크 접근 가능(기존 워크플로가 이미 동일 액션 사용 중).
- **A6**: "신규 V2 패키지" 커버리지 게이트 대상 = `opdef`(Phase -1 신설 유일 패키지). 임계값 60.0%는 실측 65.4% 대비 마진.
- **A7**: secret-scan 자체 스크립트 채택 — 근거 2개로 한정(① 카나리의 exit code 직접 capture ② 규칙·allowlist 버전 고정의 리포 소유). spec "gitleaks-style"은 검사 스타일 지정이지 도구 지정이 아니다. (초안의 "오프라인 결정성" 근거는 A5 러너 네트워크 전제와 모순되어 삭제 — L2.)
- **A8**: branch protection required checks 등록은 코드가 아닌 GitHub 설정이므로 Phase 3의 수동 후속 단계다. 등록 완료 전까지 9개 체크는 **advisory**로만 동작한다(레드여도 병합 차단 없음). 병합 즉시 등록을 전제하나 그 갭은 존재한다.
- **A9**: R1(dnsserver의 provider import 전체 금지)은 스펙 :140 "core may reference adapter interfaces"와 긴장을 품는다 — provider 패키지가 인터페이스와 제품(`aliyun.go`·`tencent.go`)을 한 패키지에 두는 한, 코어가 인터페이스를 참조하려면 그 import가 불가피하다. 오늘은 dnsserver가 provider를 전혀 import하지 않아 무충돌이므로 v0 규칙이 유효하다. Phase 0+에서 코어-인터페이스 참조가 필요해지면 provider 패키지 분리(인터페이스 패키지 / 제품 패키지)가 선행되고, 그 시점 R1은 "제품 패키지만 금지"로 세분화된다(M2).

## 7. 검증 요구 (claims — 구현 완료 후 리뷰가 검증)

1. `python3 scripts/secret-scan.py` main 클린 체크아웃에서 exit 0(PASS 라인 포함).
2. `mkdir -p /tmp/canary && printf 'aws_secret_access_key = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"\naws_access_key_id = "AKIAIOSFODNN7EXAMPLE"\n' > /tmp/canary/canary-leak.txt && python3 scripts/secret-scan.py /tmp/canary` → exit != 0, **그리고 결함 보고 라인이 `canary-leak.txt` 경로를 지정**(grep) — 자기히트가 아닌 플랜트 파일이 발화 주체임을 단언(H2).
3. `scripts/secret-scan.py` 소스에 `AKIAIOSFODNN7EXAMPLE`·`wJalrXUtnFEMI` allowlist 항목이 `.github/workflows/*` 경로 한정으로, 그리고 **스캐너 자기배제 항목이 `scripts/secret-scan.py` 경로 한정으로** 존재(grep) — claim 1 실행 시 워크플로 파일·스캐너 소스 내 동일 리터럴이 이미 무발화 통과였음이 전제.
4. `git ls-files | grep -E '^(backend|deploy)/config\.yaml$'` 빈 출력 + 스크립트 내 동일 단언 존재(grep).
5. `bash scripts/check-arch-boundary.sh` exit 0 — R1·R2·R3 각 PASS 라인 출력. R3의 허용집합이 {middleware, gin, `gorm.io/gorm`, stdlib}임을 스크립트 소스에서 확인(grep — gorm 누락 시 opdef 임포트가 거짓 결함으로 R3 레드).
6. scratch 사본의 `internal/domain/dnsserver/server.go`에 `import "ops-admin/backend/internal/domain/provider"` 1줄 추가 시 check-arch-boundary.sh exit != 0(로컬 1회 발화 실증 — 영구 잡 아님).
7. `cd backend && go test ./store/ -run TestMigrationFixture -count=1 -v` PASS + fixture 파일 존재(실측 크기 기록, 1MB 미만 예상).
8. `OPS_MIGRATION_FIXTURE_UPDATE=1` 재실행 후 fixture sha256 불변(재생성 멱등).
9. `cd backend && go test ./opdef/ -cover` ≥ 60.0%.
10. `cd backend && go test ./... -race` 로컬 PASS(13 테스트 패키지).
11. route 카나리 로컬 재현: router.go 사본에 POST /canary/unlisted 주입 → `go test ./router/ -run TestOperationTableCoversRouter -count=1` exit != 0; 미주입 시 exit 0.
12. `python3 -c` yaml.safe_load로 v2-ci.yml·korean-localization-guard.yml 파싱 성공 + `git diff`상 기존 `no-hardcoded-chinese` 잡 블록 불변(추가분만).
13. `node scripts/check-i18n-parity.mjs` exit 0(기실측 PASS — 구현 후 재확인).
14. `cd web && bun install --frozen-lockfile && bun run build` 성공(dist 생성).
15. 병합 첫 PR에서 v2-ci 8잡 + i18n-parity 전부 GitHub 러너 그린(러너 로그 링크/스크린샷) — 러너 실증 계층. **추가(H4)**: 병합 직후 branch protection 설정 열거(`gh api repos/acc0mplish/ops-admin/branches/main/protection --jq '.required_status_checks.contexts'`)로 9개 required check 등록 확인(스크린샷 대체 가능).
16. keyword+entropy 규칙 발화 실증: 89b81b0 사고와 동일 형태(키 `credential-key`, 값 len 44·`[A-Za-z0-9+/=-]` 순수)의 **합성 값**(역사 값 미재사용)을 scratch에 플랜트 → `scripts/secret-scan.py` exit != 0 — H3 규칙이 이 리포의 실제 사고 클래스를 커버함을 증명.
17. `backend/config.example.yaml`의 placeholder 값 전부가 allowlist에 등재되어 claim 1이 통과함(실측 유일 위양성 해소 — §0).

## 8. 보존 제약 (구현 프롬프트에 verbatim 복사)

- 기존 `.github/workflows/korean-localization-guard.yml`의 기존 잡(`no-hardcoded-chinese`)은 바이트 단위로 불변 — 새 `i18n-parity` 잡은 파일 말미에만 추가한다.
- 기존 테스트 파일(`backend/router/routes_inventory_test.go`, `authz_replay_test.go`, `console_ticket_test.go`, `store/seed_route_permissions_test.go`, `opdef/opdef_test.go` 등)은 일절 수정하지 않는다.
- 프로덕션 코드(비 `_test.go` 백엔드 소스, `web/src`, `web/package.json`, 기존 `scripts/*.mjs`)는 일절 수정하지 않는다 — 신규 파일만 생성한다.
- `docs/security/route-inventory.txt`·`sensitive-routes.txt` 아티팩트는 재생성하지 않는다(437/285 불변).
- 기존 `scripts/check-i18n-parity.mjs`·`check-encoding.mjs`는 수정하지 않는다.
