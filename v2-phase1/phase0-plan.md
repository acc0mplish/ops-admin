# V2 Phase 0 계획 — V2 Architecture Contract (ADR 6종 + 레지스트리 as code + fake adapter) (r2)

작성: 2026-09-06 · 개정 r2 (3렌즈 적대검토 전원 조건부 승인 + 메인 세션 판정 병합) · 티어 L · 앵커(실측 시점 HEAD): `24f3db6`

**r2 반영사항** (r1 대비 — 무결 판정 부분[어휘·인터페이스 verbatim·배치·PR 분해]은 미변경 원칙):
1. **V5 재설계**: ReadOnly→인터페이스 이분법 삭제 — §11 원문 기반 "등록 시점 대응 검증"으로 (§3.6·§3.7·T8·R6·A4·§13 인계 4).
2. **V6 RequiredPermission 정규식 완화**: `^[a-z0-9_-]+(:[a-z0-9_-]+){1,3}$` — 2~4세그 수용(실측 175건 전량 커버), A14 신설, T9 재정의, claim 16 실측 검증 추가 (§3.4·§3.6·§0 실측). 단 검토 지시안 문자클래스 `[a-z0-9_]`는 실측 3건(하이픈 세그)을 미커버 — 지시 의도 "측정된 v1 전체 커버" 충족을 위해 `-`를 추가했고 근거는 A14에 기록.
3. **게이트 2 카나리 잡 Phase 0 반입**: Phase D-2 신설(v2-ci.yml 잡 1개 추가, §21 밖 보조 PR), 보존 제약 #3 한정 완화, claim 17 추가 (§1·§2·§5·§7·§11·§12).
4. **CORE_PACKAGES 인계 정정**: "1줄 상수 변경" 비용 평가 철회 — R2 문자열 grep이 contract의 합법 유형 어휘에 발화하므로 규칙 재설계가 선행 (§5·§13·A12·R5·ADR-0001 Enforcement).
5. **claim 13 교체**: dot-grep(`grep -c "\."`)은 내부 import 우회(④review HIGH-1 기각 휴리스틱) — check-arch-boundary.sh R3 분류 로직 준용 검사로, 출력값 기준 명시 (§8·§6 T12 노트).
6. **OperationStatus.State 주석 개방형화**: 폐쇄 열거 주석 → §13.5 종단 상태 Phase 1 확정 명시 (§3.5).
7. **잔여 반영**: CI 게이트 핀닝 인계(§13 인계 6)·ADR-0001 CORE_PACKAGES 한정문(§4)·ADR-0005 secret 형상 유사 리터럴 금지(§4)·ADR-0003 "cited in review templates" 해당없음 판정(§4)·claim 11 unstaged 조건·T7 V4 귀속·T10/T13 reflect.DeepEqual·state.json 인용값 정정("gap→superseded")·에픽 언어 오류 정정(§0)·PR 13 리뷰 개시 절차 명시(§12).

- 원천: `/mnt/d/DEV/acc0mplish/ops-admin/docs/architecture/multi-infrastructure-control-plane-v2.md` **r2 + 정정 r2.1–r2.5** — 핵심 절: §3(도메인 처분 경계), §6(목표 아키텍처), §7(코어 도메인 모델), §8.5(리소스 종 어휘), §10(역량·연산 계약), §11(어댑터 인터페이스), §13(태스크 엔진), §15(비교 프로토콜), §20(Phase 0 게이트), §21(PR 분할 #12/#13/#14), §22(금지 지름길), §23(테스트 전략). 에픽 `multi-infrastructure-control-plane-epic.md` §4(공통 데이터 모델)·§5(Provider 계약 — r1 형태는 ADR-0002의 "기각된 대안" 소재) 보조 참조.
- 범위: 스펙 §20 Phase 0 Work 중 잔여 2개 PR. **PR 12 — ADR 6종**(§21 #12), **PR 13 — provider/capability/operation/resource-kind 레지스트리 as code + fake adapter**(§21 #13). **PR 14(arch-boundary 규칙 R1–R3·상시 `arch-boundary` 잡)은 362581c로 이미 인도** — 게이트 매핑은 §5. 단 **스탠딩 arch-boundary-canary 잡은 r2 판정으로 본 계획 범위에 반입**한다(Phase D-2 보조 PR, §2·§5 게이트 2). Phase 1 이상 자산(모델·브로커·엔진·harness) 일절 착수 없음.
- 브리핑 §번호 보정: 브리핑의 "§5 Provider 계약·§12 Phase 0 게이트"는 실제 스펙에서 **§10/§11(계약)·§20(게이트)**이다. 본 계획의 모든 절 인용은 실측한 실제 스펙 번호 기준(가정 A1).
- 실측 기준 커밋: `24f3db6` (main HEAD).

---

## 0. 현재 코드·자산 실측 (계획의 사실 기반)

### backend/internal/ 구조와 이름 충돌 판정
- `backend/internal/domain/provider/` — **DNS PublicDNSProvider 도메인**(provider.go:37 `PublicDNSProvider` 인터페이스, aliyun/tencent DNS 구현). 스펙 §3 row 13에서 "REMAIN, V2 무접촉"으로 처분된 V1 자산이다. **V2 인프라 provider 레지스트리와 개념이 다르고 패키지명(`provider`)이 같다.**
- `backend/internal/domain/dnsserver/` — 내장 DNS 서버(§3 row 13 REMAIN). arch-boundary R1/R2가 현재 보호하는 유일한 "코어" 패키지(`server.go`의 import 블록이 카나리 플랜트 앵커로 쓰인다 — §2 Phase D-2).
- `backend/internal/testutil/` — Phase -1에서 분리된 공유 테스트 헬퍼(`PinSecretKeys`·`OpenMemoryDB`). 본 태스크는 시크릿 무관이라 미사용.
- **판정**: V2 레지스트리는 `backend/internal/infra/` 하위 신규 경로로 배치한다(§1). 근거: (a) `internal/domain/provider`와의 폴더·패키지명 충돌 회피, (b) 스펙 §21 PR 라벨이 PR 13/16 모두 `feat(infra)`, (c) API 네임스페이스가 `/api/v2/infra/*`(§16). Go import 경로가 다르므로 `internal/domain/provider`와 공존 가능하지만, 동일 패키지명 2개는 인간·grep 도구 혼동을 만든다 — 경로 분리로 원천 차단.

### opdef 선행 사례 (R3 순수성 관례)
- `backend/opdef/` — Phase -1 T6의 민감 라우트 권한 어휘 테이블(`Def{Method, Path, Permission, Risk, Mutating, Redaction}`). `scripts/check-arch-boundary.sh` R3가 opdef의 직접 import를 `{middleware, gin, gorm, stdlib}`으로 제한한다 — **"어휘 패키지는 의존성을 갖지 않는다"는 본 저장소 확립 관례**. V2 contract 패키지도 동일 순수성 게이트를 적용한다(표준 라이브러리만 import — T12·claim 13).
- opdef(v1 라우트 어휘)와 V2 OperationDefinition(인프라 연산 어휘)은 **다른 어휘 체계** — 통합하지 않는다. 접점은 §10.3의 "연산 정의 ↔ v1 권한 문자열 1:1" 규칙뿐이고, 그 매핑 검증도 Phase 3 소관이다.
- **opdef 권한 어휘 실측(r2)**: `Permission:` 리터럴 unique **175건** — 2세그 4건(`monitor:logs`·`monitor:query`·`monitor:traces`·테스트 픽스처 `t5:first`)·3세그 135건·4세그 36건. 세그 내 **하이픈 3건**(`domains:ssl:download-key`·`system:admin:ldap-sync`·`system:config:ldap-test`). §10.3 "domain:resource:action" 3세그 전제와 **22.9%(40/175) 불일치**, `assets:k8s:workload:restart` 등 4세그 실존 — V6 완화 정규식(A14)·claim 16 실측 검증의 근거.

### Go·빌드 환경
- `backend/go.mod` — module `ops-admin/backend`, `go 1.24.0`. CI(`v2-ci.yml` backend-test·arch-boundary 잡)도 go 1.24. 신규 패키지는 표준 라이브러리만 사용 → **go.mod/go.sum 무변경**(보존 제약 #8, §10.2 "No new runtime dependencies in M1").
- 테스트 관례: in-package 테스트(`package dnsserver`형)가 기본, 교차 패키지 검증은 external `_test` 패키지(`package X_test`). `t.Setenv`·`t.Cleanup` 관례는 시크릿 계열 한정으로 본 태스크와 무관.

### PR 14 인도 자산 (게이트 매핑용)
- `scripts/check-arch-boundary.sh` — R1(코어→provider 직접 import 금지), R2(코어 소스에 aliyun/tencent 식별자 금지), R3(opdef import 순수성). `CORE_PACKAGES=(internal/domain/dnsserver)` 상수 보호. R3의 import 분류는 **명시적**이다: `ops-admin/backend/` 접두사=내부 모듈, 첫 세그먼트에 점=외부 모듈, 그 외=stdlib(스크립트 주석이 점 휴리스틱 기각 사유를 명기 — claim 13이 이 분류를 준용).
- `.github/workflows/v2-ci.yml` 8잡 — `arch-boundary` 잡이 스크립트를 매 PR 실행. **스탠딩 arch-boundary-canary 잡은 없다**(route-coverage·secret-scan만 카나리 잡 보유) — r2 판정: 게이트 2의 "(canary)" 충족 근거로 스탠딩 잡을 Phase 0 범위에 반입한다(Phase D-2, §2·§5). R1–R3 플랜트 발화의 기 실증: state.json "R1-R3 플랜트 발화" **gap→superseded**(362581c 수정 후 재실증 — r2 인용값 정정). `CORE_PACKAGES` 미확장은 R2 재설계와 결부되어 §13 인계.

### 문서 관행
- `docs/architecture/adr/` 디렉터리 **부재** — 신규 생성. 스펙은 영문이고 **에픽은 한국어다**(r2 정정 — 종전 "스펙·에픽은 영문"은 오기). ADR의 원언어는 원천 스펙을 따라 **영문**으로 작성한다.
- `docs/architecture/README.md`는 "현재 코드 기준 문서만"을 규칙으로 명시하는 인덱스다 — ADR은 결정 기록이므로 이 인덱스에 편입하지 않고, README도 수정하지 않는다(완전 additive 유지).

---

## 1. 수정 대상 (우선순위·파일 배치표 — 신규 파일 전부 경로 명시)

### PR 12 — ADR 6종 (`docs(arch): ADR set` · 스펙 §21 #12)

| 우선순위 | 파일 | 신규/수정 | 내용 요약 |
|---|---|---|---|
| 1 | `docs/architecture/adr/0001-modular-monolith.md` | 신규 | 모듈러 모놀리스 결정 계약화 — 원천 §1·§6·§3·§11.2/§12 |
| 2 | `docs/architecture/adr/0002-provider-model.md` | 신규 | provider 모델(연결/컨텍스트/자격증명바인딩 분리 + 역량/연산 계약) — 원천 §7.1–7.4·§10·§11·§2.2/2.4/2.7, 매핑 §14 상호참조 |
| 3 | `docs/architecture/adr/0003-single-tenant.md` | 신규 | 단일 테넌트 명시적 결정 — 원천 §7.6·§1 비목표 |
| 4 | `docs/architecture/adr/0004-task-engine-scope.md` | 신규 | 경량 내구 태스크 엔진 범위(3테이블·폴링·리퍼)와 이연 기계 — 원천 §13·§3.3·§13.6 |
| 5 | `docs/architecture/adr/0005-secret-envelope-v2.md` | 신규 | v2 envelope·키세트·분류·재암호화 순서 계약(Phase -1 인도분의 계약화) — 원천 §4.1–4.5 |
| 6 | `docs/architecture/adr/0006-comparison-protocol.md` | 신규 | Legacy/V2 비교 프로토콜(페어링·허용오차·분류·판정) — 원천 §15 |

우선순위 근거: 스펙 §20의 나열 순서 그대로. 0001(구조)→0002(provider 모델)가 나머지의 인용 기반이고, 0003–0006은 이 둘을 참조하는 독립 결정이다. ADR 상세 계약은 §4.

### PR 13 — 레지스트리 as code + fake adapter (`feat(infra)` · 스펙 §21 #13)

| 우선순위 | 파일 | 신규/수정 | 내용 요약 |
|---|---|---|---|
| 1 | `backend/internal/infra/contract/provider_type.go` | 신규 | `JSONMap`, `ProviderTypeDescriptor`(§7.1 verbatim), `ConfigSpec`/`ConfigFieldSpec` 최소 형상, provider 유형 어휘(M1 4종+예약 4종), 컨텍스트 종 어휘(§7.3) |
| 2 | `backend/internal/infra/contract/resource_kind.go` | 신규 | `M1ResourceKinds` — §8.5의 19종 전량, `IsKnownResourceKind` |
| 3 | `backend/internal/infra/contract/capability.go` | 신규 | `Capability`(§10.1 verbatim), `CapabilityVocabularyEntry`, `M1CapabilityVocabulary` — §10.1의 7종 |
| 4 | `backend/internal/infra/contract/operation.go` | 신규 | `OperationDefinition`(§10.2 verbatim), `RetryPolicy` 최소 형상 |
| 5 | `backend/internal/infra/contract/adapter.go` | 신규 | §11 인터페이스 5종(`BaseAdapter`·`Discoverer`·`OperationExecutor`·`TaskPoller`·`TaskCanceller`) + 지원 타입 최소 형상. `ConsoleBroker`(M2+)·`EventSubscriber`(§11 삭제)는 미선언 |
| 6 | `backend/internal/infra/registry/registry.go` | 신규 | `Registry` — provider/capability/operation 등록·조회·검증(§11 registry validation rules의 구현) |
| 7 | `backend/internal/infra/adapter/fake/fake.go` | 신규 | fake 어댑터 — `BaseAdapter` 구현, 유형 `fake` 등록(`Register(*registry.Registry) error`). Phase 0은 등록 전용(가정 A7) |
| 8 | `backend/internal/infra/contract/contract_test.go` | 신규 | 어휘 무결성 테스트 T1–T4 |
| 9 | `backend/internal/infra/registry/registry_test.go` | 신규 | 등록·조회·검증 테스트 T5–T11 (external `_test`) |
| 10 | `backend/internal/infra/adapter/fake/fake_test.go` | 신규 | fake 등록·조회 단언 T13 (게이트 3 요소) |

수정 파일: **0건**. main.go·router·service·store·model·opdef·기존 internal 전부 무접촉(레지스트리 소비자가 Phase 1부터 생기므로 배선도 없음 — 보존 제약 #9).

### 카나리 PR — arch-boundary-canary 잡 (`ci:` · Phase D-2, r2 반입)

| 우선순위 | 파일 | 신규/수정 | 내용 요약 |
|---|---|---|---|
| 1 | `.github/workflows/v2-ci.yml` | **수정**(잡 1개 추가) | 스탠딩 `arch-boundary-canary` 잡(8잡→9잡) — 스크래치 복사본에 R1(import)·R2(제품 식별자) 플랜트를 심고 `check-arch-boundary.sh`의 실패 + `R1 FAIL`/`R2 FAIL` 귀속 출력을 요구(route-coverage-canary 패턴 준용, 잡 형상은 §2 Phase D-2) |

스펙 §21 Phase 0 번호(12/13/14) 밖의 **보조 PR**이다 — §20 게이트 2 "(canary)"·§4.10 카나리 계약("발화 안 하면 CI 실패"·"게이트 인용 아티팩트는 저장·참조")이 §21 분할에 우선한다는 r2 판정. 기존 8잡 정의 라인과 `scripts/check-arch-boundary.sh`은 무수정(보존 제약 #3의 한정 완화). 전체 산출에서 수정 파일은 이 1건뿐이다.

---

## 2. Phase 분해 (구현 단위 ≤5파일)와 작업 순서

스펙 §21의 PR 분할 준수 — ADR과 코드는 **별도 PR**으로 인도한다(한 PR이 문서와 구현을 섞지 않는다).

### PR 12 (6파일 → 2 Phase)

- **Phase D — 구조 결정 ADR (3파일)**: `0001`, `0002`, `0003`. 의존: 없음. 검증: 파일 존재 + §8 claim 5–7(grep: 스펙 절 상호참조·시행 메커니즘 인용·템플릿 섹션 전수).
- **Phase E — 범위·프로토콜 ADR (3파일)**: `0004`, `0005`, `0006`. 의존: Phase D(0004가 0001의 모놀리스 결정을 인용). 검증: 동일 grep 계약.

### 카나리 PR (1파일 → 1 Phase)

- **Phase D-2 — arch-boundary-canary 잡 (1파일, r2 반입)**: `v2-ci.yml`에 잡 1개 추가. 의존: 없음(스크립트·기존 잡 무수정). 검증: claim 17(잡 존재·3요소 구조·9잡 카운트). 게이트 2 "(canary)"의 충족 근거는 이 잡의 **상시 실행**이다(§5) — 1회성 수동 실증(state.json)은 저장된 아티팩트가 아니므로 대체한다.

잡 형상(구현 계약 — route-coverage-canary 패턴 준용):

```yaml
arch-boundary-canary:
  # Standing canary — a canary that does not fire fails CI (spec §4.10).
  runs-on: ubuntu-latest
  timeout-minutes: 10
  steps:
    - uses: actions/checkout@v4
    - uses: actions/setup-go@v5
      with:
        go-version: '1.24'
        cache-dependency-path: backend/go.sum
    - name: Plant R1+R2 violations in a scratch copy
      run: |
        cp -r "$GITHUB_WORKSPACE" "$RUNNER_TEMP/canary"
        cd "$RUNNER_TEMP/canary/backend"
        # R1 plant: valid blank import, sed-inserted into the existing import
        # block (an appended `import` after declarations is a syntax error and
        # would fire for the wrong reason).
        sed -i 's|^\t"context"$|\t"context"\n\t_ "ops-admin/backend/internal/domain/provider"|' internal/domain/dnsserver/server.go
        grep -q 'internal/domain/provider' internal/domain/dnsserver/server.go || {
          echo "CANARY PLANT FAILED: /context anchor drifted — injection did not land"; exit 1
        }
        # R2 plant: a comment is enough — R2 greps source text, so the plant
        # cannot fail for a syntax reason (구문 오류 우발 실패 원천 차단).
        printf '\n// canary plant: tencent product identifier\n' >> internal/domain/dnsserver/server.go
    - name: The planted violations must be caught (failure is required)
      run: |
        cd "$RUNNER_TEMP/canary"
        if bash scripts/check-arch-boundary.sh > out.txt 2>&1; then
          echo "CANARY DID NOT FIRE: arch-boundary passed with planted violations"; exit 1
        fi
        grep -q 'R1 FAIL' out.txt || { echo "CANARY ATTRIBUTION FAILED: no R1 FAIL"; exit 1; }
        grep -q 'R2 FAIL' out.txt || { echo "CANARY ATTRIBUTION FAILED: no R2 FAIL"; exit 1; }
        grep -E 'R[12] FAIL' out.txt
```

설계 근거: (i) 플랜트는 스크래치 복사본에서만 — 리포 트리 무오염(route-coverage-canary 동일), (ii) R1 플랜트는 유효한 blank import라 `go list`가 정상 열거하고 R1 감시가 실제로 발화, (iii) R2 플랜트는 주석 1줄 — R2가 소스 텍스트 grep이므로 발화하며 구문 파손 경로 원천 차단, (iv) 앵커 드리프트(`"context"` import 부재 시)는 자가검증 grep이 "CANARY PLANT FAILED"로 실패.

### PR 13 (10파일 → 3 Phase)

- **Phase A — contract 타입·어휘 (5파일)**: `provider_type.go`, `resource_kind.go`, `capability.go`, `operation.go`, `adapter.go`. 의존: 없음. 검증: `go build ./internal/infra/...` + `go vet ./internal/infra/...` exit 0(타입·상수만 — 행동 검증은 Phase B 테스트가 담당, TDD 순서는 §6 참조).
- **Phase B — 레지스트리 + 어휘·동작 검증 (3파일)**: `registry.go`, `contract_test.go`, `registry_test.go`. 의존: Phase A. 검증: `go test ./internal/infra/...` exit 0 (T1–T11).
- **Phase C — fake 어댑터 (2파일)**: `fake.go`, `fake_test.go`. 의존: Phase A+B. 검증: `go test ./internal/infra/adapter/fake/` exit 0 + fake의 Registry 조회 단언(T13 — 게이트 3의 "fake adapter registers").

### 작업 순서 (의존관계)

```
Phase D ──┐ (PR 12)
Phase E ──┤
Phase D-2 ┼─→ 통합 검증 (claims 1–17 전수, PR 12 머지 후 PR 13 리뷰 개시 — §12 절차)
          │    단, D/E/D-2/A/B는 파일이 전혀 겹치지 않아 병렬 스폰 가능(§12)
Phase A → Phase B → Phase C   (PR 13)
```

- PR 12가 선행하는 이유: PR 13은 ADR-0002가 문서화하는 계약의 코드화다. 리뷰 순서를 계약→구현으로 유지하면 PR 13 리뷰에서 ADR을 판정 기준으로 쓸 수 있다.
- 병렬 운영 시에도 PR 단위는 유지: ADR 스폰은 `docs/architecture/adr/**`만, 코드 스폰은 `backend/internal/infra/**`만, 카나리 스폰은 `v2-ci.yml` 잡 블록만 생성·수정(§12).

---

## 3. 인터페이스 계약 — 코어 계약 심층 (스펙 어휘의 정확한 코드화)

원칙: **스펙이 준 것만 코드화한다.** 시그니처는 §7.1·§10·§11에서 한 글자도 수정 없이 옮기고(주석으로 절 인용), 스펙이 필드를 정의하지 않은 지원 타입은 "최소 형상 + 확장 시점 명시"로 만든다(가정 A6). 임의 필드 추가는 보존 제약 #6으로 금지.

### 3.1 ProviderTypeDescriptor (contract/provider_type.go — §7.1 verbatim)

```go
// JSONMap is the spec's map-typed payload container (§7.2 TLSProfile/
// ConfigJSON). Values crossing this type must be secret-free.
type JSONMap map[string]any

type ProviderTypeDescriptor struct {
    Type            string
    AdapterVersion  string
    ProtocolVersion string
    ConfigSchema    func() ConfigSpec // typed builder, not a JSON blob (§7.1)
    ContextKinds    []string
    BuiltIn         bool
}

// ConfigSpec/ConfigFieldSpec — 최소 형상(가정 A11): §7.2 ConfigJSON·TLSProfile이
// "시크릿 없는 설정 + SecretRef 참조"임을 표현할 수 있는 최소 집합.
// 필드 확장은 Phase 1(PR 16, connection 모델) 소관.
type ConfigSpec struct {
    Fields []ConfigFieldSpec
}
type ConfigFieldSpec struct {
    Name     string
    Type     string // string | bool | int | secretref
    Required bool
    Secret   bool // true면 값은 SecretRef UID로만 전달(§7.2: ConfigJSON에 시크릿 값 금지)
}
```

어휘(폐쇄 집합 — 가정 A9):

```go
// §7.1 "Registered types in Milestone 1" + §11.1 fake
var M1ProviderTypeNames = []string{"kubernetes", "aliyun", "tencent", "fake"}
// §7.1 "Reserved (descriptors land with their milestone, not before)"
var ReservedProviderTypeNames = []string{"proxmox", "vcenter", "cloudstack", "openstack"}
// §7.3 Kind 주석: cluster, account, project, subscription
var ProviderContextKinds = []string{"cluster", "account", "project", "subscription"}
```

Phase 0에서 실제 등록되는 유형은 **`fake` 단독**(kubernetes/aliyun/tencent는 어댑터가 도착하는 Phase 2/4에 등록 — §22 #17 "테스트 환경 없는 provider 코드 금지" 준수).

### 3.2 ResourceKind 어휘 (contract/resource_kind.go — §8.5 전량 19종)

```go
// §8.5 "Initial common kinds" — 순서·철자는 스펙 그대로.
var M1ResourceKinds = []string{
    "machine.baremetal",
    "compute.hypervisor_node",
    "compute.vm",
    "compute.system_container",
    "compute.image",
    "compute.template",
    "orchestration.cluster",
    "orchestration.node",
    "orchestration.namespace",
    "orchestration.workload",
    "orchestration.pod",
    "network.segment",
    "network.interface",
    "network.ip",
    "network.load_balancer",
    "storage.pool",
    "storage.volume",
    "storage.snapshot",
    "identity.account",
}

func IsKnownResourceKind(kind string) bool
```

### 3.3 Capability (contract/capability.go — §10.1 verbatim + 어휘)

```go
type Capability struct {
    Name          string
    Version       string
    ResourceKinds []string
    ReadOnly      bool
    RequestSpec   func() any // typed Go builders, not runtime JSON Schema (§10.1)
    ResultSpec    func() any
    Constraints   JSONMap
}

// §10.1 "Initial names (M1)" 7종 — ReadOnly는 가정 A4 표, OwnerPhase는
// "r1의 vnc/spice… move to their owning phases" 문장의 계약화.
type CapabilityVocabularyEntry struct {
    Name       string
    ReadOnly   bool
    OwnerPhase string
}

var M1CapabilityVocabulary = []CapabilityVocabularyEntry{
    {Name: "inventory.full", ReadOnly: true, OwnerPhase: "M1"},
    {Name: "inventory.incremental", ReadOnly: true, OwnerPhase: "M1"},
    {Name: "orchestration.kubernetes.read", ReadOnly: true, OwnerPhase: "M1"},
    {Name: "orchestration.kubernetes.apply", ReadOnly: false, OwnerPhase: "M1/Phase3"}, // §10.1 "(Phase 3: restart only)"
    {Name: "compute.vm.read", ReadOnly: true, OwnerPhase: "M1"},
    {Name: "cost.read", ReadOnly: true, OwnerPhase: "M1"},
    {Name: "console.web_terminal", ReadOnly: true, OwnerPhase: "M1"}, // 가정 A4 — Phase 1 콘솔 배선 시 재확정
}
```

역량의 `ResourceKinds`·`RequestSpec` 등 상세 메타데이터는 **선언 주체(각 페이즈의 어댑터)** 가 채운다 — Phase 0은 어휘(이름·ReadOnly·소유 페이즈)만 코드화한다(과잉 추상화 방지, §10 R1). ReadOnly는 어휘 메타데이터 값일 뿐 **서빙 인터페이스 도출(V5)에 사용하지 않는다**(r2 — §3.6).

### 3.4 OperationDefinition (contract/operation.go — §10.2 verbatim)

```go
type OperationDefinition struct {
    Name               string
    Version            string
    ResourceKinds      []string
    RequiredCapability string
    RequiredPermission string // v1-namespace string — 허용 형식 2~4세그·하이픈 허용(§10.3 서술과 v1 실측의 불일치는 가정 A14)
    Mutating           bool
    RiskLevel          string
    RequiresApproval   bool
    IdempotencyPolicy  string
    TimeoutSeconds     int
    RetryPolicy        RetryPolicy
    Redaction          func() any // typed redaction spec per result field (§10.2)
}

// 최소 형상(가정 A10) — §10.2가 타입명만 언급. 필드 확장은 Phase 1 태스크 엔진이
// 실제 재시도 동작을 구현할 때 그 시점의 요구로 결정.
type RetryPolicy struct {
    MaxAttempts    int
    BackoffSeconds int
}
```

내장 연산 정의는 **0건**(가정 A8) — M1이 흡수하는 유일 변이는 Kubernetes workload restart(§3.3)뿐이고 그 명칭·정의는 Phase 3(PR 24)이 소유한다. Phase 0은 타입·레지스트리·검증 규칙만 인도한다.

### 3.5 어댑터 인터페이스 (contract/adapter.go — §11 verbatim)

```go
type BaseAdapter interface {
    Descriptor() ProviderTypeDescriptor
    Validate(ctx context.Context, connection ConnectionView) error
    Health(ctx context.Context, connection ConnectionView) HealthResult
    Close() error
}

type Discoverer interface {
    Discover(ctx context.Context, req DiscoverRequest) (DiscoverPage, error)
}

type OperationExecutor interface {
    Execute(ctx context.Context, req OperationRequest) (OperationHandle, error)
}

type TaskPoller interface {
    Poll(ctx context.Context, handle OperationHandle) (OperationStatus, error)
}

type TaskCanceller interface {
    Cancel(ctx context.Context, handle OperationHandle) error
}
// 미선언: ConsoleBroker(§11 "M2+, with the agent ADR"), EventSubscriber(§11에서 삭제됨)
```

지원 타입 최소 형상(전부 가정 A6 — 각 필드의 출처 절을 주석으로 기록, 확장 시점 명시):

```go
// §7.2에서 도출 — 시크릿 값 결계: Config에 시크릿 소재 포함 금지.
// ConnectionView 필드 확장(브로커 목적 자격증명 등)은 Phase 1 PR 16.
type ConnectionView struct {
    ProviderType string
    Endpoint     string
    Config       JSONMap
}

type HealthResult struct {
    Healthy bool
    Message string
}

// §9.1/9.2에서 도출 — 커서 페이징. 리소스 초안 레코드 정규화 형태는 Phase 2 PR 20.
type DiscoverRequest struct {
    ContextID uint
    Cursor    string
}

type DiscoveredResource struct { // §8.1(신원) + §8.2(관측) 필드의 초안 병합
    ExternalID  string
    ExternalURN string // §8.1 UNIQUE(provider_context_id, kind, external_urn)의 성분
    Kind        string // IsKnownResourceKind 충족
    Subtype     string
    DisplayName string
    Raw         JSONMap
    Normalized  JSONMap
}

type DiscoverPage struct {
    Resources  []DiscoveredResource
    NextCursor string
}

// §13(엔진)에서 도출 — 핸들은 provider-네이티브 작업 참조(예: UPID).
type OperationRequest struct {
    OperationName string
    ResourceURN   string
    Payload       JSONMap
}

type OperationHandle struct {
    ProviderRef string
}

type OperationStatus struct {
    State  string // §13.5 종단 상태(TimedOut·Cancelling·Cancelled 포함)는 Phase 1 폴러 설계 시점에 확정
    Detail JSONMap
}
```

### 3.6 Registry (registry/registry.go)

§6 다이어그램의 "Provider Type Registry" + §11 "Registry validation rules" 구현. 전역 상태 없음 — **명시적 구성**(§11.1 "registered explicitly"): 값으로 생성해 등록 후 사용한다. 등록 완료 후 조회의 병행 안전만 내부 `sync.RWMutex`로 보장한다.

```go
type Registry struct { /* mut: provider types, capabilities by type, operations */ }

func New() *Registry

func (r *Registry) RegisterProviderType(d contract.ProviderTypeDescriptor, a contract.BaseAdapter) error
func (r *Registry) ProviderTypeNames() []string
func (r *Registry) ProviderType(name string) (contract.ProviderTypeDescriptor, contract.BaseAdapter, bool)

func (r *Registry) RegisterCapabilities(providerType string, caps ...contract.Capability) error
func (r *Registry) Capabilities(providerType string) []contract.Capability

func (r *Registry) RegisterOperation(def contract.OperationDefinition) error
func (r *Registry) Operation(name string) (contract.OperationDefinition, bool)

func (r *Registry) ResourceKinds() []string // contract.M1ResourceKinds 위임
```

등록 시 검증 규칙(§11 registry validation rules의 Phase 0 구현 범위):

| # | 규칙 | 근거 |
|---|---|---|
| V1 | provider 유형명은 `M1ProviderTypeNames`의 원소여야 하고 `ReservedProviderTypeNames`면 "descriptor lands with its milestone" 에러 | §7.1 |
| V2 | provider 유형 중복 등록 거부 | §11 (descriptor는 코드=단일 등록점) |
| V3 | descriptor `ContextKinds` ⊆ `ProviderContextKinds` | §7.3 |
| V4 | capability 이름은 `M1CapabilityVocabulary`에 존재 + 유형 내 중복 거부 + `ResourceKinds` 전부 `IsKnownResourceKind` | §10.1·§8.5 |
| V5 | **capability→인터페이스 대응 검증**: 등록 시점(실질 Phase 1+)에 declared capability가 어댑터가 구현한 인터페이스에 대응하는지 검증한다. Phase 0이 구현하는 것은 최소 가드 — capability를 선언하는 어댑터는 capability-수용 인터페이스(`Discoverer`·`OperationExecutor`) 중 **최소 1개를 구현**해야 하며, 아니면 어떤 capability도 등록 거부. **ReadOnly→인터페이스 이분법(true→Discoverer, false→OperationExecutor)은 채택하지 않는다**(r2 — `console.web_terminal`의 실제 인터페이스는 ConsoleBroker M2+, `cost.read`는 ReadOnly이지만 discovery가 아님. capability별 매핑 표는 Phase 1 계획 소유, §13 인계 4) | §11 "A declared capability must map to an implemented interface" |
| V6 | operation: 이름+버전 중복 거부(동일 이름 재등록도 Phase 0에선 거부 — 가정 A13), `RequiredCapability`는 어휘 내 존재, `ResourceKinds` 전부 알려진 종, `RequiredPermission`은 `^[a-z0-9_-]+(:[a-z0-9_-]+){1,3}$` 형식(2~4세그·세그 내 하이픈 허용 — v1 실측 전량 커버, 가정 A14), `Mutating=true`인데 `ReadOnly=true` capability만 참조하면 거부 | §10.2·§10.3(형식은 A14 실측 완화)·§11 |

### 3.7 fake 어댑터 (adapter/fake/fake.go)

```go
// Package fake is the first-class test adapter (§11.1 built-in list,
// §11 "fake is a first-class adapter, not an afterthought").
// Phase 0: registration only — Discoverer/inventory capabilities and
// behavior land with the Phase 1 contract test harness (가정 A7).
type Adapter struct{}

func (Adapter) Descriptor() contract.ProviderTypeDescriptor
// Type:"fake", AdapterVersion:"1", ProtocolVersion:"1"(가정 A5),
// ConfigSchema: 빈 ConfigSpec, ContextKinds: 없음, BuiltIn: true
func (Adapter) Validate(ctx context.Context, c contract.ConnectionView) error // 항상 nil
func (Adapter) Health(ctx context.Context, c contract.ConnectionView) contract.HealthResult
func (Adapter) Close() error // nil

func Register(r *registry.Registry) error // r.RegisterProviderType(Adapter{}.Descriptor(), Adapter{})
```

Phase 0의 fake는 capability 0건으로 등록된다(가정 A7) — V5 최소 가드상 fake는 capability-수용 인터페이스를 구현하지 않으므로 어떤 capability 선언도 거부되며, T8이 이 거부를 단언한다.

---

## 4. ADR 계약 (PR 12 — 스펙 결정의 격상, 신규 결정 창조 아님)

형식(6종 공통 템플릿, 영문 작성 — §0 실측 관행):

```text
# ADR-000N: <제목>
## Status        — Accepted (2026-09-XX, elevates spec r2 §X; implemented in <Phase/PR or "Phase 0 contract only">)
## Context       — 상황: 측정된 사실과 계기 (스펙 해당 절 요약 + 코드 실측 인용)
## Decision      — 결정: 스펙의 결정 명세 (불릿)
## Rationale     — 근거
## Alternatives considered — 대안: 기각된 선택과 기각 사유 (r1/에픽 형태 포함)
## Consequences  — 결과: 긍정/부작용/되돌림 비용
## Enforcement   — 시행: §22 금지 항목 #N + 게이트/CI/리뷰 수단 명칭
## References    — multi-infrastructure-control-plane-v2.md §X, §Y / epic §Z
```

**ADR은 스펙의 요약·판정 기준이지 복사본이 아니다** — 결정 명세는 스펙 절 링크로 참조하고 원문 전사를 하지 않는다(§10 R4: 이중 진실원 방지). 각 ADR의 원천·대안·시행 계약:

| ADR | Context 원천 | 결정 코어 (1줄) | 기각 대안 | Enforcement (§22) |
|---|---|---|---|---|
| 0001 modular monolith | §1 결정 블릿, §6 단일 프로세스 코어+in-process 어댑터, §3 경계 행렬 | 변환 1단계는 모놀리스 유지, 코어-어댑터 경계는 패키지·인터페이스로 시행, 분해는 Phase 6 조건부 | 마이크로서비스 조기 분해(r1 §2.5/2.6 God object 비판의 과잉 반응), out-of-process 어댑터 런타임(§11.2 이연) | §22 #10(third-party adapter 사내 프로세스 금지), #18(REMAIN 침투 PR 거부), arch-boundary R1–R3 — **한정문(r2): 현행 CORE_PACKAGES는 dnsserver 한정이며 infra 코어 확장은 R2 재설계 후(§13 인계)** |
| 0002 provider model | §7.1–7.4(연결/컨텍스트/바인딩/SecretRef 분리), §10(역량·연산이 진실원), §11(인터페이스 분할), §2.2/2.4/2.7(진단) | provider 지원은 descriptor+레지스트리+역량으로; 코어는 제품명 분기 금지; UI/API 는 capability 기반 | 범용 Provider 인터페이스(에픽 §5 "금지" 예시형, §22 #3), switch문 팩토리(현행 §2.7), AssetCloudAccount 확장(§2.2) | §22 #1(provider 전용 FK), #2(프로바이더별 정적 세트), #3(만능 인터페이스); arch-boundary R2; §14 매항 매트릭스 상호참조(가정 A2) |
| 0003 single-tenant | §7.6(측정: 전 모델에 TenantID 0건), §1 비목표 | 테넌트 없음은 설계 불변수 — 정책 입력 tenant="default" 상수, 감사 무테넌트, 재진입은 전용 ADR로만 | r1의 ProviderTask.TenantID·토큰 테넌트 클레임·테넌트 감사 컬럼 | §22 #13(provider project/account ≠ platform tenant), 리뷰 체크리스트(테넌트 컬럼 부재) — "cited in review templates"는 **리포에 리뷰 템플릿 메커니즘 부재로 해당 없음 판정(r2)**: ADR 본문 Enforcement 섹션에 체크리스트 항목을 기록하는 방식으로 대체 |
| 0004 task-engine scope | §13(3테이블·폴링·리퍼·리스), §3.3(흡수 목록 — M1은 k8s restart뿐), §13.6(이연 기계+트리거 표) | V2 변이는 경량 내구 엔진으로; 다중 복제 기계는 트리거 도래까지 이연, 흡수 가족은 명시 목록으로만 | r1의 fencing+heartbeat+outbox+inbox+4타임아웃(다중 복제 설계), 외부 브로커/큐 | §22 #7(프로세스 로컬 고루틴 금지 — 신규 V2 코드 한정), §23 engine 티어 crash/lease 시나리오 |
| 0005 secret envelope v2 | §4.1(14행 인벤토리), §4.2(형식), §4.3(삼중 판정), §4.4(순서 계약), §4.5(롤백) | `v2:<key_id>:<base64url(nonce‖ct)>` + 키세트 + UNKNOWN halt + 재암호화→검증→폐기 순서. **구현은 Phase -1 인도 완료 — ADR은 기인도 설계의 계약화(Status에 명기)** | 평문 폴백 허용(§2.9 결함), 무키 식별 회전, 외부 볼트 백엔드(M1 비목표) | §20 G-1~G-5 게이트, v2-ci secret-scan(+canary)·migration-test 잡, §23 N4/N5 |
| 0006 comparison protocol | §15.1(스냅샷 캡처·페어링 60s), §15.2(필드 매핑), §15.3(BLOCKER/VOLATILE/DRIFT/ABSENT), §15.4(3회 연속 판정) | Legacy/V2 비교는 페어링·허용오차·분류·판정 기준을 가진 프로토콜; 매핑 테이블 미기재 필드는 프로토콜 버그 | 무페어링 즉석 diff(r1 형태 — 검증 불가), 동시성 보장 가정(§15 명시적 거부) | §23 N12(BLOCKER identity → 게이트 거부), §20 Phase 2 게이트(§15.4 저장 리포트 인용), §19 M2 섀도우 리드 정의 |

작성 지침: 각 ADR은 60–120줄 내외. "신규 결정 창조 금지 — 스펙에 없는 근거·수치는 스펙 인용으로만"(보존 제약 #10). ADR-0005는 Phase -1 구현 커밋(T1–T5·0a17a77 회전 실행)을 References에 나열한다. **ADR-0005 본문에는 secret 형상과 유사한 리터럴(키 접두·nonce·base64 예시 문자열 등)을 기재하지 않는다**(r2) — secret-scan 오탐·카나리 오염 방지.

---

## 5. Phase 0 출구 게이트 매핑 (§20)

| 게이트 | 소유 PR | 상태/인도 수단 |
|---|---|---|
| 1. ADR set merged | PR 12 (Phase D+E) | 본 계획으로 인도 — claims 5–7로 검증 |
| 2. arch-boundary 워크플로가 플랜트된 core→provider-product import 거부(카나리) | PR 14(스크립트·`arch-boundary` 잡 — 362581c 인도 완료) + **본 계획 Phase D-2(스탠딩 카나리 잡, r2 반입)** | 상시 감시: `arch-boundary` 잡이 `check-arch-boundary.sh` R1–R3을 매 PR 실행. 카나리: Phase D-2가 `v2-ci.yml`에 추가하는 `arch-boundary-canary` 잡(8잡→9잡)이 스크래치 복사본에 R1 import·R2 제품 식별자 플랜트를 심고 스크립트 실패 + `R1 FAIL`/`R2 FAIL` 귀속 출력을 요구(route-coverage-canary 패턴 준용, §2 잡 형상). 반입 근거: 스펙 §4.10 "발화 안 하면 CI 실패"·§20 게이트 2 "(canary)"·§4.10 "게이트 인용 아티팩트는 저장·참조" — 1회성 수동 실증(state.json "R1-R3 플랜트 발화" gap→superseded, 362581c)은 저장된 아티팩트가 아니므로 상시 잡으로 대체한다. 잔여: `CORE_PACKAGES` 미확장은 R2 재설계와 결부되어 §13 인계 |
| 3. registries compile + fake adapter registers | PR 13 (Phase A+B+C) | `go build`/`go test`로 등록·조회 단언(T13) — claims 1–4·9–10 |

게이트 3의 검증 수단(스펙 §20 명시): go test로 (a) 레지스트리가 컴파일되고, (b) fake가 `Register` 후 `ProviderType("fake")` 조회에 등장하는 단언. 이것이 본 태스크의 테스트 없는 영역(ADR)과 구분되는 기계 검증 계약이다.

---

## 6. 테스트 계약 (TDD — 테스트 먼저 작성, 레드→그린)

`contract_test.go` (T1–T4, Phase B에서 먼저 작성해 Phase A 산출물을 검사):

| # | 테스트 | 검증 |
|---|---|---|
| T1 | `TestM1ResourceKindVocabularyMatchesSpec` | 19종·스펙 §8.5 순서·철자 정합 + 중복 0 |
| T2 | `TestProviderTypeVocabulary` | M1 4종(kubernetes/aliyun/tencent/fake) + 예약 4종(proxmox/vcenter/cloudstack/openstack) + 컨텍스트 종 4종 + 두 집합 교집합 0 |
| T3 | `TestCapabilityVocabularyMatchesSpec` | 7종 이름·ReadOnly(apply만 false)·중복 0 |
| T4 | `TestContractSupportTypesAreMinimal` | 어휘 슬라이스가 전부 비어 있지 않은 원소만 갖고, descriptor 스펙 필드(§7.1 6필드) 전수 존재 — 리플렉션 불필요, 컴파일 시점 필드 + 값 단언 |

`registry_test.go` (T5–T11, external `package registry_test` — fake·contract import):

| # | 테스트 | 검증 |
|---|---|---|
| T5 | `TestRegisterProviderRejectsReservedAndUnknown` | 예약명(proxmox 등)·M1 외 이름 등록 → 에러에 "milestone"/"unknown" 근거 포함 (V1) |
| T6 | `TestRegisterProviderDuplicateRejected` | fake 유형 이중 등록 → 에러 (V2) |
| T7 | `TestRegisterCapabilityValidation` | 미등록 유형·어휘 외 이름·유형 내 중복·알려지지 않은 ResourceKinds → 전부 에러 (V4 — 4케이스 전부 V4에 귀속: 미등록 유형 거부는 V4의 등록 전제 검사, r2 명시) |
| T8 | `TestCapabilityInterfaceMappingRule` | fake(`Discoverer`·`OperationExecutor` 모두 미구현)에 `inventory.full` 선언 → 에러 (V5 최소 가드 — capability-수용 인터페이스 0개인 어댑터의 capability 선언 거부, r2 재정의). capability별 인터페이스 매핑 단언은 Phase 1 계획 소유(§13 인계 4) |
| T9 | `TestRegisterOperationValidation` | 중복 (name,version)·동일 이름 재등록·어휘 외 RequiredCapability·알려지지 않은 종·파손된 권한 문자열(A14 완화 정규식 `^[a-z0-9_-]+(:[a-z0-9_-]+){1,3}$` 미충족: 1세그·5세그·대문자·빈 세그)·Mutating↔ReadOnly 불일치 → 전부 에러; 4세그·하이픈 세그(`assets:k8s:workload:restart`·`system:admin:ldap-sync`)는 수용 (V6·A14, r2) |
| T10 | `TestOperationDefinitionRoundTripLookup` | 정합 operation 등록 → `Operation(name)` 조회로 동일 값 회수 — `reflect.DeepEqual` 단언 (r2 명시) |
| T11 | `TestRegistryLookupMissesAndEmptyState` | 미등록 유형/연산 조회 → ok=false, 패닉 없음; `New()` 직후 `ProviderTypeNames()` 빈 슬라이스 |

`fake_test.go` (T13 — 게이트 3 단언):

| # | 테스트 | 검증 |
|---|---|---|
| T13 | `TestFakeAdapterRegistersAndLooksUp` | `registry.New()` + `fake.Register(r)` → `r.ProviderTypeNames()`에 "fake" 존재 + `r.ProviderType("fake")`가 descriptor `{Type:"fake", BuiltIn:true}`와 `BaseAdapter` 구현체 회수(descriptor 비교는 `reflect.DeepEqual` — r2 명시) + `r.Capabilities("fake")` 빈(Phase 0 계약, 가정 A7) |

T12(번호 예약 — contract 패키지 순수성)은 단위 테스트가 아닌 claim 13의 `go list` 검사로 대체한다(r2) — 검사 분류는 `check-arch-boundary.sh` R3 로직(`ops-admin/backend/` 접두사=내부 모듈·첫 세그먼트 점=외부 모듈·그 외=stdlib)을 준용한다. arch-boundary 스크립트 확장은 PR 14 자산이라 무수정이며, CI 게이트 핀닝은 §13 인계 6이다.

커버리지: §23.3 "new V2 packages ≥80% statement, measured per package on changed code" — `go test ./internal/infra/... -coverprofile`로 `contract`+`registry`+`adapter/fake` 신규 패키지 전부 측정(claim 14). 어휘 상수·타입만으로 분모가 작은 `contract`는 조회 함수(`IsKnownResourceKind`)와 테스트 가능 표면 전부 커버로 측정한다.

---

## 7. 보존 제약 (③구현 프롬프트에 verbatim 복사)

1. 기존 Go 소스·테스트 파일을 한 건도 수정하지 않는다 — PR 13은 `backend/internal/infra/` 하위 신규 파일만 생성한다(additive). `main.go`·`router/`·`controller/`·`service/`·`store/`·`model/`·`opdef/`·`internal/domain/`·`internal/testutil/` 무접촉.
2. v2 스펙(`docs/architecture/multi-infrastructure-control-plane-v2.md`)과 에픽(`multi-infrastructure-control-plane-epic.md`)·`docs/architecture/README.md`를 수정하지 않는다 — ADR은 `docs/architecture/adr/` 신규 파일로만 인도한다.
3. `scripts/check-arch-boundary.sh`와 `.github/workflows/*.yml`을 수정하지 않는다(PR 14 인도 자산). **r2 한정 완화**: Phase D-2는 `.github/workflows/v2-ci.yml`에 `arch-boundary-canary` 잡 1개를 추가하는 것만 허용된다 — 기존 8잡 정의·다른 워크플로·스크립트 본체는 여전히 무수정이며, 추가도 잡 블록의 신규 삽입 한정이다(기존 라인 편집 불가).
4. 시크릿 체인(`util/secret*.go`, `OPS_SECRET_MASTER_KEYS` 배선, 재암호화 CLI)과 콘솔 티켓 체인에 접촉하지 않는다.
5. `backend/internal/domain/provider`(DNS V1)·`backend/internal/domain/dnsserver`는 무수정 — V2 패키지 경로는 반드시 `backend/internal/infra/` 하위다(이름 충돌 회피, §0 판정).
6. 스펙이 정의하지 않은 타입 필드·메서드·어휘 항목을 임의로 추가하지 않는다 — 계획 §3의 "최소 형상" 표 외의 확장은 금지며, 구현 중 불가피한 이탈은 구현 보고서에 명시한다.
7. `ConsoleBroker`·`EventSubscriber` 인터페이스를 선언하지 않는다(M2+/삭제됨). 예약 provider 유형(proxmox/vcenter/cloudstack/openstack)의 descriptor 등록 코드를 작성하지 않는다 — 어휘 상수의 "거부 목록" 등재만 한다.
8. `backend/go.mod`·`go.sum`을 수정하지 않는다 — 신규 의존성 추가 금지(§10.2 "No new runtime dependencies in M1").
9. `main.go`에서 신규 레지스트리를 배선하지 않는다 — 프로덕션 조합(root registry)은 Phase 1(PR 16/17) 소관. Phase 0의 등록 호출은 테스트만.
10. ADR은 영문·§4 템플릿 준수로 작성하고 스펙에 없는 결정·수치·근거를 창조하지 않는다 — 해당 스펙 절의 격상이며, 인용은 절 단위로 한다. ADR-0005 본문에 secret 형상과 유사한 리터럴을 기재하지 않는다(§4 작성 지침).

---

## 8. 검증 요구 (claims — ④리뷰 주입용)

1. `cd backend && go build ./...` exit 0.
2. `cd backend && go test ./internal/infra/...` exit 0.
3. `cd backend && go test ./...` exit 0 (기존 테스트 회귀 0).
4. `cd backend && go vet ./...` exit 0.
5. `ls docs/architecture/adr/000{1,2,3,4,5,6}-*.md` — 6파일 존재(이름: modular-monolith, provider-model, single-tenant, task-engine-scope, secret-envelope-v2, comparison-protocol).
6. `grep -l "multi-infrastructure-control-plane-v2.md" docs/architecture/adr/*.md | wc -l` = 6 (전 ADR 스펙 상호참조).
7. `grep -c "## Enforcement" docs/architecture/adr/*.md` — 6파일 전부 ≥1 (§22 "Enforced by" 인용 계약).
8. `grep -n "M1ResourceKinds" backend/internal/infra/contract/resource_kind.go` 존재 + T1이 §8.5 19종·순서·무중복을 단언 (테스트명·수치 리뷰 확인).
9. `grep -n "\"fake\"" backend/internal/infra/adapter/fake/fake.go` 존재 + `BuiltIn: true` 동반.
10. `grep -n "ProviderType(\"fake\")" backend/internal/infra/adapter/fake/fake_test.go` 존재 — fake 등록 후 Registry 조회 단언(게이트 3).
11. `git status --porcelain | grep -v "^??"` — **PR 12·PR 13 스폰 산출 기준** 출력 없음(전부 신규 파일 — 기존 파일 0수정). 판정 조건(r2): 커밋 전 작업영역(unstaged·staged 전부 포함)에서 커밋 직전에 실행한다 — 산출 커밋 후에는 porcelain이 비어 이 검사는 무의미하다. 예외 수정 파일: Phase D-2의 `.github/workflows/v2-ci.yml` 1건은 별도 커밋 단위이며 claim 17이 별도 검증한다.
12. `bash scripts/check-arch-boundary.sh` exit 0 — 신규 패키지가 R1–R3 무발화(스크립트 자체는 무변경).
13. `cd backend && go list -f '{{join .Imports "\n"}}' ./internal/infra/contract | grep -E '^ops-admin/backend/|^[^/]+\.' | wc -l` = 0 — contract의 import에 내부 모듈(`ops-admin/backend/` 접두사)·외부 모듈(첫 세그먼트에 점)이 전부 없다(`check-arch-boundary.sh` R3 분류 로직 준용 — 모듈명 `ops-admin/backend`에 점이 없어 점 휴리스틱 `grep -c "\."`은 내부 import를 놓치는 기각된 방법, r2 교체). 판정은 **wc 출력값 기준**: `grep -c`는 매칭 0에서 exit 1을 반환해 `&&` 체인을 깬다.
14. `cd backend && go test ./internal/infra/... -coverprofile=c.out && go tool cover -func=c.out` — 신규 3패키지(`contract`·`registry`·`adapter/fake`) 각 ≥80% 수치 기재(§23.3).
15. `git diff --stat backend/go.mod backend/go.sum` — 출력 없음(신규 의존성 0).
16. `grep -ho 'Permission: *"[^"]*"' backend/opdef/*.go | sed 's/.*"\([^"]*\)"/\1/' | sort -u | grep -vE '^[a-z0-9_-]+(:[a-z0-9_-]+){1,3}$' | wc -l` = 0 — v1 opdef 권한 unique 175건이 V6 완화 정규식(A14)을 **전량 충족**(§0 실측 분포 2세그 4·3세그 135·4세그 36·하이픈 3건의 재현 검증). 출력값(wc) 기준 판정(r2 신설).
17. `grep -n "arch-boundary-canary:" .github/workflows/v2-ci.yml` 존재 + `sed -n '/^jobs:/,$p' .github/workflows/v2-ci.yml | grep -c "^  [a-z0-9-][a-z0-9-]*:$"` = 9(8잡→9잡 — 잡 키만 세도록 `jobs:` 섹션 한정, `on:`·`concurrency:` 하위 키 제외) + 잡 정의 3요소 포함(리뷰 확인, r2 신설): (i) 스크래치 복사본 플랜트(R1 sed import 삽입 + R2 주석, 앵커 드리프트 자가검증 grep 동반), (ii) 스크립트 exit 0 시 "CANARY DID NOT FIRE"로 잡 실패, (iii) "R1 FAIL"·"R2 FAIL" 귀속 출력 grep. 기존 8잡 정의 라인은 diff상 불변(잡 블록 추가만).

---

## 9. 가정 명세 (불확실 요소의 명시적 처리)

- **A1 브리핑 절번호 보정**: 브리핑 인용 "§5(Provider 계약)·§12(Phase 0 게이트)"는 실측 스펙에서 §10·§11(계약)·§20(게이트)에 대응한다. 본 계획은 실제 스펙 번호를 사용한다.
- **A2 provider mapping matrix**: §20 Phase 0 Work의 "provider mapping matrix"는 스펙 §14 자체(기인도 승인 문서)로 판정 — 별도 PR이 없으므로(§21 Phase 0 = 12/13/14뿐) 신규 산출물 없이 ADR-0002가 §14를 상호참조하는 것으로 충족한다.
- **A3 패키지 위치**: `backend/internal/infra/{contract,registry,adapter/fake}`. 스펙 §23.3의 "(provider, inventory, tasks, secrets, policy)" 패키지명 표기는 등가성 예시로 읽는다(secrets는 Phase -1에서 `util/`로 이미 인도됨 — 실례). `infra` 접두사는 §21 PR 라벨 `feat(infra)`·§16 `/api/v2/infra/*`와 정합.
- **A4 Capability ReadOnly 값**: `orchestration.kubernetes.apply`=false(§10.1 "(Phase 3: restart only)"), `console.web_terminal`=true(리소스를 변경하지 않는 대화형 접근 — 티켓 게이트는 §4.8이 담당; Phase 1 콘솔 배선 시 재확정 인계). 나머지 5종 이름이 read/inventory/cost여서 true. **r2: ReadOnly는 어휘 메타데이터 값일 뿐 서빙 인터페이스 도출(V5)에 사용하지 않는다**(§3.6 — `cost.read`·`console.web_terminal` 반례).
- **A5 Version 관례**: 스펙이 Capability/OperationDefinition/descriptor의 Version 값을 정하지 않으므로 `"1"`로 통일(2번째 버전이 실제로 등장할 때 버전 체계 ADR 필요 — 임의 semver 체계 조기 설계 금지).
- **A6 지원 타입 최소 형상**: §3.5의 ConnectionView/HealthResult/DiscoverRequest/DiscoveredResource/DiscoverPage/OperationRequest/OperationHandle/OperationStatus 필드는 스펙 §7.2·§8·§9·§13에서 도출된 최소 집합이다. 필드 확장 시점: ConnectionView→Phase 1(PR 16), DiscoverPage/DiscoveredResource 정규화 상세→Phase 2(PR 20), OperationStatus 결과 구조→Phase 3(PR 24). 각 타입 주석에 이 시점을 명기한다.
- **A7 fake의 Phase 0 형상**: 등록 전용 — capability 0건, Discoverer/executor 미구현. 기능(인메모리 디스커버리·실행 시뮬레이션)은 Phase 1 contract test harness가 추가한다. 브리핑 계약("FakeProvider는 등록만 되는 테스트용 어댑터") 준수.
- **A8 내장 operation 0건**: M1 유일 흡수 변이(k8s workload restart, §3.3)의 명칭·정의는 Phase 3 소관. Phase 0 연산 레지스트리는 타입·검증·조회 API만 인도.
- **A9 어휘 폐쇄 집합**: 리소스 종 19·역량 7·provider 유형 M1 4(+예약 4)를 닫힌 집합으로 강제한다(집합 밖 등록 = 에러). 이는 게이트가 아니라 가드다 — 확장은 해당 페이즈 PR에서 어휘 목록 자체를 diff로 늘려 리뷰받는다(§10.1이 r1의 vnc/spice 등을 "owning phases로 이동"시킨 방식의 코드화).
- **A10 RetryPolicy 최소 형상**: `{MaxAttempts int, BackoffSeconds int}` — §10.2가 타입명만 언급. 상세(지수 백오프·지터)는 Phase 1 엔진이 실제 재시도를 구현하며 요구 기반으로 확장.
- **A11 ConfigSpec/ConfigFieldSpec 최소 형상**: §7.1 "typed builder, not a JSON blob"의 최소 구현. 필드 스키마 확장은 Phase 1(PR 16) connection 모델 시점.
- **A12 게이트 2 잔여 판정(r2 갱신)**: (a) 스탠딩 arch-boundary-canary 잡은 r2에서 Phase 0 범위로 반입되어 Phase D-2가 인도한다(§2·§5 — 더 이상 인계 항목이 아님). (b) `CORE_PACKAGES` 확장은 R2 규칙 재설계와 결부되어 본 계획 밖 인계다(§13 — 문자열 grep 재설계 전에 상수만 늘리면 contract의 합법 유형 어휘에 발화).
- **A13 operation 단일 버전 레지스트리**: 동일 이름 재등록(다른 버전 포함)도 Phase 0에선 거부한다 — 다중 버전 공존 요구가 실제로 생기기 전에 `name`별 단일 정의가 진실원 1개를 보장한다(§11 "immutable within a version"의 보수적 해석).
- **A14 RequiredPermission 허용 형식(실측 완화, r2 신설)**: §10.3의 "domain:resource:action" 기술은 v1 실측과 불일치 — opdef 권한 unique 175건 중 3세그 135건·4세그 36건·2세그 4건(3세그 전제 대비 22.9% 불일치), `assets:k8s:workload:restart` 등 4세그 실존(§0 실측). 허용 형식은 `^[a-z0-9_-]+(:[a-z0-9_-]+){1,3}$`(2~4세그)로 완화한다. 검토 지시안 문자클래스 `[a-z0-9_]`에서 `-`를 추가한 것은 실측 3건(`domains:ssl:download-key`·`system:admin:ldap-sync`·`system:config:ldap-test`)이 하이픈을 포함하기 때문 — `-`가 없으면 합법 v1 권한이 등록 거부된다(지시 의도 "측정된 v1 전체 커버" 충족). claim 16이 175건 전량 충족을 기계 검증한다.

---

## 10. 리스크·트레이드오프

| # | 리스크 | 검증 / 완화 |
|---|---|---|
| R1 | **조기 설계의 과잉 추상화**(브리핑 리스크) — 범용 인터페이스·JSONSchema 런타임으로 변질 | §22 #3·§10.2 계약 준수: 인터페이스는 §11의 5종만, 스키마는 typed Go constructor, 신규 의존성 0(claim 15). 지원 타입은 최소 형상+확장 시점 명시(A6). 테스트 T1–T3이 어휘 폐쇄 집합을 단언해 범위 밖 확장을 에러로 가드 |
| R2 | `internal/domain/provider`(DNS)와 V2 패키지의 이름 혼동 | 경로 분리(`internal/infra/`) + fake/contract 패키지 doc 주석에 "not the DNS provider domain (§3 row 13 REMAIN)" 명기. R1 검사 문자열(`internal/domain/provider`)과 신규 import 경로(`internal/infra/...`)는 서브스트링 비충돌 실측 완료 |
| R3 | 어휘 조기 동결로 Phase 2/4가 레지스트리에 막힘 | A9: 폐쇄 집합은 "등록 거부"가 아니라 "리뷰 가능한 diff로 확장"하는 가드. Phase 2/4 계획 시점에 어휘 확장이 자연스러운 PR 구성이 된다 |
| R4 | ADR-스펙 이중 진실원(스펙 개정 시 ADR만 낡음) | §4 작성 지침: 결정 명세는 스펙 절 링크+요약만, 원문 전사 금지. ADR 권위는 Status(Accepted + 스펙 개정판)로만 관리 |
| R5 | 게이트 2 사각지대 — 신규 코어가 R1/R2 보호 밖 | r2: 스탠딩 arch-boundary-canary 잡을 Phase 0 범위로 반입(Phase D-2) — 규칙 자기검증이 상시화되고 게이트 2 "(canary)" 요소가 저장된 아티팩트(워크플로 잡)로 존재한다. 코어 패키지 집합 확장은 R2 재설계와 결부되어 §13 인계. 본 태스크 내 방어선: claim 12(현행 R1–R3 무발화)·claim 13(contract 순수성) |
| R6 | `console.web_terminal`·`cost.read` — ReadOnly 값과 서빙 인터페이스의 혼동 | r2: V5에서 ReadOnly 이분법을 삭제(§3.6) — 인터페이스 오매핑 경로 자체를 제거. Phase 0에서 ReadOnly는 어휘 메타데이터(T3)와 V6 Mutating↔ReadOnly 정합에만 소비된다. capability→인터페이스 매핑 표는 Phase 1 계획 소유(§13 인계 4), 콘솔 미배선이라 Phase 0 무영향 |
| R7 | 커버리지 분모 왜곡(타입·상수 중심 패키지) | §23.3 "changed code per package" 기준으로 3패키지 각각 측정·수치 기재(claim 14). 순수 상수는 커버리지 분모에서 비중이 크지 않도록 검증 가능 표면(lookup·검증 규칙)이 `registry`에 집중되게 배치 |
| R8 | ADR 6종이 계약 요소 누락(얇은 계획의 전형 리스크) | §4 배치표가 6종 전부의 원천 절·기각 대안·시행 메커니즘을 지정 + claims 5–7이 형식·상호참조를 기계 검증. 누락 원천(§14 매핑 등)은 A2로 명시적 판정 |

트레이드오프 요약: 레지스트리를 Phase 0에 코드화하는 비용(유지보수할 어휘·검증 코드) vs 수익(arch-boundary가 지키는 경계의 실체, Phase 1 harness·Phase 2 디스커버리가 즉시 소비할 계약, §22 금지사항의 시행 대상 존재). 스펙 §21이 이 순서를 명시했으므로 조기 코드화는 스펙 계약이고, 본 계획의 과잉 억제(최소 형상·폐쇄 집합·0배선)가 그 비용을 줄인다. r2의 D-2 반입은 여기에 카나리 잡 1개의 유지비를 추가한다 — 게이트 2 "(canary)" 계약의 저장된 아티팩트 확보가 그 대가다.

---

## 11. 롤백 가능성 판정

- **PR 12(ADR)**: 전 파일 신규 문서 — PR 단위 revert 무위험. 소비자 없음.
- **카나리 PR(Phase D-2)**: `v2-ci.yml` 잡 1개 추가 — PR 단위 revert 무위험(스크립트 무변경, 잡 제거는 나머지 CI에 영향 없음).
- **PR 13(레지스트리+fake)**: 전 파일 신규 + 기존 코드 0수정 + 배선 0(보존 제약 #9) — PR 단위 revert로 빌드·테스트·기동 경로 전부 원상복귀. 데이터·스키마 무접촉이므로 데이터 롤백 개념 자체가 없다.
- **부분 롤백**: Phase A만 남고 B/C를 revert해도 컴파일 가능(타입 패키지는 자기완결). 단 게이트 3(fake registers)은 C까지 완료해야 충족 — Phase 단위 부분 인도는 게이트 미충족 상태로 명시적으로 관리해야 한다. D-2는 다른 산출과 무의존이라 임의 시점 revert 가능(단 revert 시 게이트 2 "(canary)" 근거가 소멸하므로 페이즈 종결 전에는 유지).
- 판정: **전 산출 단위 완전 롤백 가능**(additive-only + 잡 1개 추가의 구조적 결과).

---

## 12. 파일 소유권 — 구현 스폰 분리 (충돌 방지)

| 스폰 | 전용 소유 경로 | 접근 금지 |
|---|---|---|
| impl-PR12 (ADR) | `docs/architecture/adr/**` | backend 전체, 스펙/에픽/README, scripts, workflows |
| impl-D2 (카나리 잡) | `.github/workflows/v2-ci.yml` — `arch-boundary-canary` 잡 블록 추가만 | 그 외 전부(스크립트 본체·기존 8잡 정의 라인·backend·docs·스펙) |
| impl-PR13-A (contract) | `backend/internal/infra/contract/**` | registry·adapter 하위를 제외한 전부 |
| impl-PR13-B (registry) | `backend/internal/infra/registry/**` | contract는 읽기·import만(수정 금지 — 소유는 A) |
| impl-PR13-C (fake) | `backend/internal/infra/adapter/fake/**` | contract·registry는 읽기·import만 |

- 교차 소유 파일 0건 — PR 12·PR 13·D-2 세 산출 단위는 병렬 스폰 가능(§2 순서).
- 공통 무접촉: `v2-phase1/state.json`·`v2-phase1/p0-state.json`(메인 관리), 스펙/에픽/스크립트(보존 제약 #2·#3).
- **절차(r2 명시): PR 13의 리뷰 개시는 PR 12 머지 후다** — 구현 스폰은 병렬이어도 리뷰만 직렬(리뷰가 ADR-0002를 PR 13의 판정 기준으로 사용). 직렬 운영 시 PR 12(D→E) 머지 후 PR 13(A→B→C) 리뷰 개시. D-2는 리뷰 순서 제약 없음(독립 CI 자산).

---

## 13. 산출 외 확인사항 (구현자에게 불요, 팀 리드 인계)

- **arch-boundary 자산 후속(PR 14 소유자 판정)**: (a) 스탠딩 arch-boundary-canary 잡은 r2에서 Phase 0 범위로 반입되어 Phase D-2가 인도한다(§2·§5 — 더 이상 인계 항목이 아니다). (b) **R2 규칙 재설계 필요(r2 정정)**: `CORE_PACKAGES`에 `internal/infra/contract`·`internal/infra/registry`를 추가하는 것만으로 신규 코어 보호가 확장되지 않는다 — R2는 `grep -rnE '\b(aliyun|tencent)\b'` 전체 소스 grep이라 contract 패키지의 정당한 유형 어휘(`M1ProviderTypeNames`의 `"aliyun"`·`"tencent"` 문자열 리터럴)에 발화한다(종전 "1줄 상수 변경" 비용 평가는 철회). 제품 식별자 검사를 문자열 전체 grep이 아닌 import·타입 참조 기반으로 재설계해야 확장이 성립한다 — PR 14 소유자 재검토. (F10) 재설계·확장 PR 착지 시 PR 13과의 revert 순서 조건을 병기한다(보호 규칙이 아직 없는 패키지를 참조하거나, PR 13 revert 시 보호 사각이 생기는 순서를 사전에 못박는다). 본 계획의 claim 12는 현행 스크립트 무발화만 검증한다.
- **인계 1 — Phase 1(PR 16/17)**: 레지스트리 프로덕션 배선(root Registry 조합·`fake.Register` 포함)과 `ConnectionView` 필드 확장. fake에 Discoverer 구현·`inventory.*` capability 선언이 붙는 시점(가정 A7 해소).
- **인계 2 — Phase 2(PR 20)**: `DiscoverPage`/`DiscoveredResource` 정규화 상세·kubernetes descriptor 등록·리소스 종 어휘 확장 심사.
- **인계 3 — Phase 3(PR 24)**: 첫 `OperationDefinition` 등록(workload restart) — RequiredPermission 문자열이 기존 v1 권한(§10.3)과 정합하는지 시드 기반 검증.
- **인계 4 — Phase 1 capability→인터페이스 매핑 표(Phase 1 계획 소유, r2 확장)**: ReadOnly 이분법 없이 각 capability의 서빙 인터페이스를 명시하는 매핑 표를 Phase 1 계획이 소유한다. 등재할 예외 시나리오: `console.web_terminal`의 실제 인터페이스는 ConsoleBroker(M2+, §11 — Phase 0 미선언), `cost.read`는 ReadOnly=true이지만 Discoverer가 아님. `console.web_terminal` ReadOnly 값 재확정(가정 A4) 포함.
- **인계 5 — Version 체계**: 2번째 Capability/OperationDefinition 버전 등장 시 다중 버전 레지스트리 정책(가정 A13 해소) 필요.
- **인계 6 — internal/infra CI 게이트 핀닝(PR 14 소유자, r2 신설)**: §23.3 커버리지 래치(신규 패키지 ≥80% 문장)와 contract 순수성 검사(claim 13의 `go list` R3 분류 준용)를 상시 CI 게이트로 핀닝한다 — 본 계획의 claims 14·13은 수동 검증이므로 머지 후 시점부터는 CI가 임계 하회·비표준 import를 실패시켜야 한다.
- PR 12·13·D-2 머지 후 §20 Phase 0 게이트 3요소의 충족 근거(이 문서 §5 표 + claims 결과)를 tracking issue #4 체크리스트에 링크해야 페이즈 종결이 성립한다(§21 "Resume protocol").
