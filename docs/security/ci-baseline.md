# CI 베이라인 — 9 required checks와 게이트 자기검증 카나리

V2 Phase -1 Task 6가 도입한 CI 베이스라인의 운영 문서. 원천 스펙은 V2 아키텍처
문서 §4.10(CI baseline and its self-verification)이며, 핵심 원칙은 하나다:

> 발화하지 않는 카나리는 CI를 실패시킨다.

검사 로직이 조용히 무력화(앵커 표류·규칙 삭제·allowlist 남발)되는 것을
탐지하는 카나리 잡 2종이 일반 체크 7종과 함께 돈다.

## 1. 체크 구성

required check의 단위는 잡이다. `main` 브랜치 보호에 아래 9개를 등록한다.

| 잡 (context) | 워크플로 | 내용 |
|---|---|---|
| `backend-test` | v2-ci.yml | `go test ./... -race -count=1` + opdef 커버리지 래치(§5) |
| `frontend-build` | v2-ci.yml | `bun install --frozen-lockfile && bun run build` |
| `migration-test` | v2-ci.yml | 커밋된 baseline fixture 재오픈 + AutoMigrate+Seed(§4) |
| `route-coverage` | v2-ci.yml | 라우트 골든 3종 — inventory 아티팩트·sensitive-routes 표·opdef 커버리지 |
| `route-coverage-canary` | v2-ci.yml | 카나리(§3) — 무등재 non-GET 라우트 플랜트 |
| `secret-scan` | v2-ci.yml | `scripts/secret-scan.py` 트리 스캔 + config.yaml 가드(§2) |
| `secret-scan-canary` | v2-ci.yml | 카나리(§3) — 시크릿 플랜트 |
| `arch-boundary` | v2-ci.yml | `scripts/check-arch-boundary.sh` R1–R3(§6) |
| `i18n-parity` | korean-localization-guard.yml | `node scripts/check-i18n-parity.mjs` |

트리거는 `push: branches: [main]` + `pull_request`이며 path filter를 두지
않는다. required check가 path filter와 결합하면 대상 밖 PR에서 체크가
pending에 고착된다. 루트 `concurrency`가 동일 ref의 구실행을 취소하고, 모든
잡은 `contents: read` 권한과 잡별 `timeout-minutes`를 가진다.

## 2. 시크릿 스캔 운영

`python3 scripts/secret-scan.py [target-dir]` — 고정밀 규칙 7종(AWS 키 페어,
AWS secret 할당형, PEM 개인키 헤더, GitHub/Slack/OpenAI 토큰, keyword+entropy).

- **keyword+entropy 규칙**은 역사상 실제 발생한 사고 클래스(credential-key에
  44자 순수 base64 계열 값)를 커버하기 위해 존재한다. 값 클래스에 하이픈·
  언더스코어가 포함된 이유다. 이 규칙의 유일한 위양성은 example 설정의
  placeholder이며 allowlist로 관리된다.
- **allowlist는 "값 ∥ 경로 glob" 쌍**이다. 값이 지정 경로 밖에서 발견되면
  결함이다. 신규 등재는 실측된 무해 발생에 한정하고, 규칙 추가와 함께 커밋
  메시지에 근거를 남긴다. 카나리 리터럴은 `.github/workflows/*`에서만,
  계획 문서 인용분은 `v2-phase1/*`에서만 허용된다.
- **스캐너 자기배제**: 스캐너 소스(`scripts/secret-scan.py`)는 규칙 리터럴을
  담는 유일한 프로덕션 외 파일이므로 값 검사에서 제외된다. 경로 기반이라
  카나리 scratch 사본에서도 동일하게 작동한다.
- **git-ignored 파일은 스캔 제외**다. `backend/config.yaml`은 설계상 로컬
  시크릿 보관 위치(§4.6 untrack)이며, 리포에 들어올 수 없는 파일은 검사
  대상이 아니다. 대신 히스토리 가드가 두 설정 파일의 무추적과 .gitignore
  조항 존재를 단언한다.
- **전체 git 히스토리 스캔은 의도적으로 하지 않는다.** untrack 이전의
  `backend/config.yaml` 5커밋이 존재해 영구 레드가 된다. 이력의 실시크릿
  여부는 T1 키 로테이션이 근본 대응이다(히스토리 재작성 아님).
- 위양성이 발견되면: 규칙을 약화시키지 말고 값∥경로 allowlist 항목을
  추가한다. 제거는 그 규칙이 커버하던 사고 클래스를 함께 제거하는 일이다.

## 3. 카나리 — 자기검증 계약

두 카나리는 scratch 사본(`cp -r "$GITHUB_WORKSPACE" "$RUNNER_TEMP/canary"`)에
결함을 심고, 검사가 **반드시 실패해야** 잡이 통과한다. 검사가 통과하면 카나리
잡이 레드가 된다 — 검사 로직의 무력화가 조용히 지나가지 않는다.

- **secret-scan-canary**: scratch 루트에 결함 파일을 심는다(워크플로 내
  리터럴은 allowlist 경로 안에 있으므로, 같은 값이 scratch 루트에서는
  발화해야 한다 — 경로 범위 allowlist가 이 양립을 성립시킨다). 잡은 ① 스캐너
  비-0 exit ② 보고 라인이 플랜트 파일 경로를 지명하는지 grep 단언을 요구한다.
  ②가 자기히트·다른 원인의 실패를 가려낸다.
- **route-coverage-canary**: scratch의 `backend/router/router.go`에서
  `/profile` 등록 라인(앵커) 직후에 opdef 없는 POST 라우트를 sed로 주입한다.
  주입 성공 grep 단언(앵커 표류 시 침묵 대신 잡 실패 — W-1의 안전 방향)과,
  `TestOperationTableCoversRouter` 비-0 exit 단언으로 구성된다. 무등재
  non-GET 라우트는 곧 스펙이 말하는 "known unlisted route"다.

카나리 수정 시 로컬 실증을 먼저 한다: 플랜트 → 비-0 exit, 무플랜트 → exit 0.

## 4. 마이그레이션 baseline fixture

`backend/store/testdata/migration-fixture/baseline.sqlite`는 **커밋 시점의
스키마+시드 상태를 바이트로 동결**한 fixture다. `TestMigrationFixtureUpgrade`
는 이 파일을 임시 사본으로 재오픈해 AutoMigrate+Seed가 데이터 손실 없이
통과함을 증명한다. 스키마가 변경되는 순간부터 이 테스트는 "구버전 데이터 →
신규 스키마" 업그레이드 게이트가 된다(동일 DB 인-프로세스 재실행은 멱등
증명일 뿐 — 이 구분이 fixture가 존재하는 이유다).

재생성:

```bash
cd backend && OPS_MIGRATION_FIXTURE_UPDATE=1 go test ./store/ -run TestMigrationFixture -count=1
```

**재생성 남용 가드(M1)** — 스키마 변경 PR이 재생성을 같이 커밋하면 fixture가
신규 스키마로 갱신되어 업그레이드 증명이 멱등 증명으로 퇴화한다.

1. 재생성 커밋은 스키마 변경 커밋과 **반드시 별도 커밋**으로 분리한다.
2. 재생성 커밋 메시지에 **직전 커밋 시점의 코드로 재생성했음을 명기**한다.

바이트 비결정성: 시드된 관리자 비밀번호가 랜덤 솔트 bcrypt 해시이고 행이
생성 시각을 갖는다. 재생성 안정성은 sha256이 아니라 **의미론적 지문**
(sys_admin/sys_role/sys_menu 행수 + route-permissions 마커)으로 검증한다
(테스트 주석에 동일 내용 기록). fixture는 `.gitattributes`의 binary 조항으로
개행 변조로부터 보호된다.

## 5. 커버리지 래칫

`backend-test` 잡이 `go test ./opdef/ -cover` 출력을 파싱해 게이트한다.

- 기준선(2026-09-06 실측): **65.4%**
- 래치 값: **60.0%** — 하향 마진을 둔 래치다. 이 미만이면 잡이 레드다.
- **래치는 올릴 수 있고 내릴 수 없다.** 커버리지가 래치를 넘어서면 새 값을
  기준선으로 기록하고 래치를 상향한다. 테스트 삭제로 커버리지가 감소하는
  PR은 래치 위반으로 막힌다.

## 6. 아키텍처 경계 규칙 (R1–R3)

`bash scripts/check-arch-boundary.sh` — 전이 의존이 아닌 직접 의존을 검사한다
(`go list .Imports`, `-deps` 금지).

- **R1**: 코어(`internal/domain/dnsserver`)의 직접 import에
  `internal/domain/provider` 금지. provider 패키지가 인터페이스와 제품
  구현을 한 패키지에 두는 동안 코어-인터페이스 참조가 필요해지면 먼저
  provider를 인터페이스/제품 패키지로 분리하고 R1을 "제품 패키지만 금지"로
  세분화한다.
- **R2**: 코어 소스에 제품 식별자(aliyun/tencent) 금지 — 인터페이스와
  capability 이름은 허용.
- **R3**: `opdef`의 직접 import는 {middleware, gin, gorm.io/gorm, stdlib}로
  한정(opdef는 순수 어휘 테이블 — service/controller/router/store/model
  도달 금지). gorm 허용은 `opdef.Middleware(db *gorm.DB, ...)` 시그니처 때문에
  필수다.

## 7. branch protection 등록 (병합 후 수동 단계)

GitHub 설정은 리포 파일이 아니므로 위 워크플로 병합 직후 수동으로 등록한다.
**등록 전까지 9개 체크는 advisory로만 동작한다**(레드여도 병합이 막히지
않음) — 이 갭을 두지 않는 것이 병합 직후 수행의 이유다. `gh api`(관리자
권한 필요):

```bash
cat <<'JSON' | gh api -X PUT repos/acc0mplish/ops-admin/branches/main/protection --input -
{
  "required_status_checks": {
    "strict": true,
    "contexts": [
      "backend-test", "frontend-build", "migration-test",
      "route-coverage", "route-coverage-canary",
      "secret-scan", "secret-scan-canary", "arch-boundary", "i18n-parity"
    ]
  },
  "enforce_admins": true,
  "required_pull_request_reviews": null,
  "restrictions": null,
  "allow_force_pushes": false,
  "allow_deletions": false
}
JSON
```

`required_pull_request_reviews`는 리뷰 정책에 맞게 채운다. 등록 확인:

```bash
gh api repos/acc0mplish/ops-admin/branches/main/protection --jq '.required_status_checks.contexts'
```

9개 context가 열거되면 완료. 이후 첫 PR에서 두 워크플로의 9잡이 전부 그린인지
러너 로그로 관찰한다.

## 8. 로컬 검증 명령

```bash
python3 scripts/secret-scan.py                      # 클린이면 exit 0
bash scripts/check-arch-boundary.sh                 # R1+R2+R3 PASS 라인
cd backend && go test ./... -race -count=1          # 전 패키지
cd backend && go test ./store/ -run TestMigrationFixture -count=1 -v
cd web && bun install --frozen-lockfile && bun run build
node scripts/check-i18n-parity.mjs
```
