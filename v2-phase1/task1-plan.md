# V2 Phase -1 · Task 1 계획 — v2 Envelope Writer + Dual-Key Reader (§4.4 Step 1) · r2

- 원천: `/tmp/v2-rev/docs/architecture/multi-infrastructure-control-plane-v2.md` **r2.4** (d5d734f) §4.1–§4.5 — 15행 인벤토리(r2.3), 혼재 컬럼 declared-secret 게이트·P-class 순서 규칙·Step 1 P-class bullet 교체(r2.4) 반영본 기준.
- 범위: §4.4 **Step 1 "SHIP WRITER + READER (dual-key)"만**. Step 2(데이터 재암호화)·Step 3(검증 게이트)·Step 4(legacy 폐기·폴백 삭제)·Step 5(백업 폐기) 모두 제외.
- 계약 요약: E-class 신규 쓰기는 `v2:` envelope, E-class 판독은 v2+legacy 병행 수용, **15행** 레지스트리+classify(혼재 컬럼 declared 게이트 포함) 단일 구현, 사전 비행 인벤토리 CLI. 데이터 재작성 없음, 키 제거 없음, 폴백 삭제 없음. P-class writer 전환은 원계획 r2.4가 Step 1 외 전제 태스크로 지정(§3).
- 개정 이력: r1 → r2 (3렌즈 적대 반영 13건 — CR-1/CR-2/H-1~H-4/M-1~M-6/L-1~L-3).
- 실측 기준 커밋: `7705845` (state.json anchor 88f5a7b 이후 main).

---

## 0. 현재 코드 실측 (계획의 사실 기반)

### 기존 envelope (변경 대상 아님, 보존)
- `backend/util/secret.go` — AES-256-GCM, `base64.RawURLEncoding(nonce‖ct)`, key = `sha256(seed)`, seed 체인: `ConfigureCredentialKey(config.yaml)` → env `OPS_ADMIN_CREDENTIAL_KEY` → env `OPS_ADMIN_JWT_SECRET` → dev 폴백 `"ops-admin-development-credential-key"` (secret.go:29–46).
- 테스트 관례: `util/secret_test.go` — `t.Setenv` + `ConfigureCredentialKey` + `t.Cleanup` 패턴.

### E-class(legacy envelope) 소비처 — 전수 (`grep -rn "util\.\(Encrypt\|Decrypt\)Secret("`, 비테스트)
| 사이트 | 현재 호출 | Step 1 후 |
|---|---|---|
| service/domain.go:149,155 (DNS 계정 저장) | `EncryptSecret` | `EncryptSecretV2` |
| service/domain.go:102 (마스크 힌트, `err == nil` 폴백) | `DecryptSecret` | `ReadSecretField` — **폴백 원문 유지** |
| service/domain.go:222,226 (DNS 자격 증명, fail-closed) | `DecryptSecret` | `ReadSecretField` |
| service/ops_schedule.go:176 (스케줄 비밀 변수 저장) | `EncryptSecret` | `EncryptSecretV2` |
| service/ops_schedule.go:156 (기존 비밀 변수 재사용) | `DecryptSecret` | `ReadSecretField` (+ declared 게이트, §2) |
| service/ssl_certificate.go:246 (수동 인증서 키 암호화) | `EncryptSecret` | `EncryptSecretV2` |
| service/ssl_certificate.go:522,689,695 (인증서 키 판독) | `DecryptSecret` | `ReadSecretField` |
| service/ssl_certificate.go:832,836 (DNS 계정 판독) | `DecryptSecret` | `ReadSecretField` |
| service/ssl_acme.go:275 (ACME 인증서 키 암호화) | `EncryptSecret` | `EncryptSecretV2` |
| **무변경** service/domain.go:145–167 — payload 키 비제출 시 `old.AccessKeyCipher/SecretKeyCipher`를 그대로 이어받는 **암호문 복사·보존 경로** (포맷 무관: legacy든 v2든 그대로 전달) | — | — |
| **무변경** service/ssl_acme.go:286 — cert→version 암호문 **복사** (포맷 무관 대입) | — | — |

위 10개 호출지가 E-class 소비처의 전부다. `SSLCertificateVersion.PrivateKeyCipher`는 쓰기(복사)만 존재하고 판독 소비처가 없다(실측) — Step 2에서 분류·재암호화 대상이 됨.

### 기존 테스트 커버 실측 (r2 교정 — CR-1)
- 시크릿 암호화 경로를 건드리는 **기존 서비스 테스트는 `service/ops_script_test.go:31`**(`TestResolveScheduleScriptVariablesKeepsSecretsEncrypted`, 스케줄 변수) **1건뿐**. DNS 계정·SSL 인증서 경로는 기존 테스트 커버 0 → Phase B 신규 왕복 테스트 **필수**(T16/T17).
- service 테스트는 mock 기반 순수 함수 패턴(`domain_test.go` mockDNSProvider) — **DB 하neus 부재**, go.mod에는 mysql 드라이버만 존재(sqlite 없음). 왕복 테스트를 위한 테스트 전용 의존성 결정은 §1.

### P-class(평문) 실측
- 비테스트 service+model에서 P-class 유사 필드 참조 **157곳**(원계획 r2.4도 동일 수치 인용).
- **UI 원문 회신 경로(이중 암호화 부패 위험 — 원계획 r2.4 Step 1 문안이 근거로 채택)**:
  - `model/k8s.go:20` `KubeConfig string json:"kubeConfig"` — `GetK8sClusterInfo`(controller/k8s.go:23)·`Create/UpdateK8sCluster`가 `model.K8sCluster`를 그대로 반환, `service/k8s.go:728`이 payload를 무조건 재저장 → UI 재제출 시 envelope 재암호화 = 데이터 부패.
  - `service/notify.go:205` `mapNotifyChannel`이 `item.Secret` 원문 회신 → 동일 구조.
- `AssetCredential`은 `maskCredential`(service.go:2581, `******` 치환) + 공백 제출 시 기존값 유지(service.go:1975–1980)로 라운드트립이 부분 보호 — P-class는 필드그룹별 응답 마스킹·갱신 의미론이 **비균질**.
- **row 15(r2.3 신설)**: `AssetDatabase.Password`(model/asset.go, service/database.go:228,266,289 등 평문 사용, 응답 전 공백 처리 database.go:329,344) — 레지스트리 P로 편입(가정 A4 철회, §9).

### CLI·DB 구조
- `backend/main.go`(56행) — 서브커맨드·flag 파싱 전무. `config.Load("config.yaml")` → `util.ConfigureCredentialKey` → `store.NewDB(cfg)` → `AutoMigrate` → `Seed` → `CanonicalizeSeedLocalization`(main.go:36) → 서버 기동.
- `backend/cmd/` 디렉터리 없음. inventory 명령은 main 패키지 내 파일 추가가 최소 (별도 바이너리·build 타깃 불필요).
- 스케줄 비밀 변수: `OpsScheduleTask.Variables`(`model/ops.go:212`, `type:text` JSON map, 커스텀 Valuer/Scanner). **비밀 선언은 행이 아닌 스크립트 메타데이터(`OpsScript.Variables[].Secret`)에 존재 — 혼재 선언 컬럼**(§2 classify 게이트 계약).

---

## 1. 파일 배치

| 파일 | 신규/수정 | 내용 |
|---|---|---|
| `backend/util/secret_registry.go` | 신규 | `SecretFieldClass` 상수(E-legacy/P), `SecretField` 구조체(Model/Table/Column/Class/Labels + row 4 `MixedDeclaration` 플래그 + row 14 webhook_url `Conditional` 플래그), `SecretFields` **15행** 슬라이스, `LookupSecretField(table, column)` |
| `backend/util/secretv2.go` | 신규 | envelope v2 writer/reader, 키 세트 파싱·보관, `ClassifySecret`(declared 게이트 포함), `ReadSecretField`, 집계 순수 함수(`AggregateFormats`) |
| `backend/util/secretv2_test.go` | 신규 | §6 T1–T15, T18 (util 레벨 전체) |
| `backend/service/domain.go` | 수정 | 위 표의 5개 호출지 교체 (102 폴백 유지) |
| `backend/service/ops_schedule.go` | 수정 | 156/176 교체 (156은 declared 게이트 전달) |
| `backend/service/ssl_certificate.go` | 수정 | 246/522/689/695/832/836 교체 |
| `backend/service/ssl_acme.go` | 수정 | 275 교체 (286·domain.go:145–167 무변경) |
| `backend/service/secretv2_roundtrip_test.go` | **신규(필수 — CR-1)** | T16 DNS 계정 저장→v2→판독, T17 SSL 수동 키 저장→판독. 테스트 전용 sqlite 하neus(아래 결정) |
| `backend/go.mod`/`go.sum` | 수정 | **테스트 전용 의존성** `gorm.io/driver/sqlite` 1개 추가 (프로덕션 코드는 미참조) |
| `backend/main.go` | 수정 | 디스패치: `len(os.Args) >= 2 && os.Args[1] == "inventory-secrets"` 가드(L-2) → `runSecretInventory(os.Args[2:])`, 그 외 기존 경로 무변경 |
| `backend/main_inventory.go` | 신규 | `runSecretInventory` (§4 초기화 시퀀스 포함 — H-4) |
| `backend/main_inventory_test.go` | 신규(선택) | 출력 조립 최소 테스트. 집계 계약 자체는 T18로 util 레벨에서 필수화(M-2) |
| `backend/util/secret_guard.go` | **신규(페이즈 B)** | G-5 기동 가드: `EnsureSecretKeySource` — 키 소스 전무 + GO_ENV 비-development → 기동 fatal. 개발 폴백 시드 자체는 Step 4까지 유지 |
| `backend/main_reencrypt.go` | **신규(페이즈 B)** | §4.4 Step 2 `reencrypt-secrets`: E-class row-by-row classify→read→v2 재작성→저장 바이트 재판정→(table, column, pk) 체크포인트. `--dry-run`·`--backup-acknowledged`·`--exclude-p-class`·`--json`. UNKNOWN halt+quarantine, P-class 거부(r2.4) |
| `backend/main_verify.go` | **신규(페이즈 B)** | §4.4 Step 3 `verify-secrets`: classifier 재실행(zero LEGACY/PLAINTEXT/UNKNOWN) + 테이블당 ≥10%·≥500행 stride 스팟 복호, `--out` JSON 리포트, 게이트 exit code |
| `backend/internal/testutil/secret_test_helpers.go` | **신규(페이즈 B)** | 공유 테스트 헤니스: `PinSecretKeys`(키 상태 고정·복원), `OpenMemoryDB`(단일 커넥션 in-memory sqlite). 아래 테스트 폴더 정책 참조 |
| `backend/main_secret_migration_test.go` | **신규(페이즈 B)** | Step 2 코어 10건: legacy→v2 왕복·재개 스킵·UNKNOWN quarantine·P-class 거부·백업 미확인 거부·mixed-declaration |
| `backend/main_secret_migration_coverage_test.go` | **신규(페이즈 B)** | 커버리지 강화 13건: CLI 플래그·dry-run 무쓰기·V2 오염값 스팟 복호 실패·≥500 샘플 하한·저장 바이트 소비 증명(tamper)·체크포인트 스키마 리빌드 |

**[계획 결정] sqlite 테스트 의존성**: 왕복 테스트(T16/T17)가 실제 호출지(`SavePublicDNSAccount`→`publicProvider` 등)를 통과하려면 in-memory DB가 필요하다. 기존 service 테스트는 mock 기반으로 DB 하neus가 없고 mysql은 테스트 실행 불가. `gorm.io/driver/sqlite`를 **테스트 전용**으로 추가한다(프로덕션 import 금지 — 보존 제약 #10). 대안(util 경계 왕복만)은 CR-1의 "콜사이트 행동 검증 0"을 닫지 못해 기각. Service는 동일 패키지 테스트에서 `&Service{db: db}`로 구성(비내보내기 필드 직접 주입), 경로가 건드리는 모델(PublicDNSAccount·감사 로그 모델 등)만 `AutoMigrate`.

**[계획 결정 — 페이즈 B] 테스트 파일 조직 (test 폴더 분리)**: 시크릿 마이그레이션 테스트가 계속 늘어나므로 다음 규칙으로 분리한다.

- **공유 헤니스는 `backend/internal/testutil/` 패키지로 분리**: 키 상태 고정(`PinSecretKeys`)·in-memory fixture DB(`OpenMemoryDB`) 등 테스트 파일 간 중복되는 헬퍼는 여기에 모은다. 새 테스트 파일은 헬퍼를 자체 정의하지 않고 testutil을 import한다.
- **`_test.go` 파일 자체는 Go 언어 제약상 각 패키지 디렉터리에 유지**: 미내보내기 식별자(`reencryptSecretsInDB` 등)는 동일 패키지에서만 보이므로 `test/` 최상위 폴더로 옮기면 컴파일이 깨진다. 분리는 "파일 소재지"가 아니라 "중복 제거 단위"로 한다.
- **util 패키지 내부 테스트는 testutil을 import하지 않는다**: testutil이 util을 import하므로 `package util` 내부 테스트가 testutil을 참조하면 import 사이클. util의 `configureMasterKeys` 헬퍼는 `secretv2_test.go`에 유지.
- **후속 대상**: service 레이어 왕복 테스트(`secretv2_roundtrip_test.go`, T16/T17)가 추가될 때도 같은 규칙 — fixture DB 구성은 testutil 헬퍼를 재사용하고, 서비스별 시드 데이터 빌더가 늘어나면 testutil에 기능별 파일(`testutil/k8s_fixtures.go` 등)로 분리한다.

`util/secret.go`은 **한 글자도 수정하지 않는다** (보존 제약 #1). config·store·router·controller 무수정.

**Phase 분해 (12파일 > 5파일 → 3 Phase, 각 ≤5파일·독립 검증)**

- **Phase A — 순수 모듈 (3파일)**: `secret_registry.go`, `secretv2.go`, `secretv2_test.go`. 의존: 없음. 검증: `go test ./util/...` + 신규 코드 한정 커버리지(§8 claim 11). main/service 어디서도 안 쓰는 상태로 먼저 그린 확보.
- **Phase B — E-class 사이트 전환 (6파일)**: domain.go, ops_schedule.go, ssl_certificate.go, ssl_acme.go, secretv2_roundtrip_test.go, go.mod/go.sum(1묶음). 의존: Phase A. 검증: `go test ./...` — 기존 커버(ops_script_test.go:31) 회귀 0 + **신규 T16/T17 통과**(기존 테스트만으로는 12/14 호출지 무검증 — CR-1 교정).
- **Phase C — 인벤토리 CLI (3파일)**: main.go, main_inventory.go, main_inventory_test.go(선택). 의존: Phase A만(B와 병렬 가능). 검증: `go build ./...` + dry-run.

작업 순서: A → (B ∥ C) → 통합 검증.

---

## 2. API 설계 (util/secretv2.go)

### 키 세트
- **소스**: 환경변수 `OPS_SECRET_MASTER_KEYS` — 순서 보존 `current_key_id:key_material[,old_key_id:key_material...]`. config.yaml wiring은 하지 않는다(보존 제약 #2).
- `ConfigureSecretMasterKeys(spec string) error` — 파싱·보관. `ConfigureCredentialKey`와 동일한 뮤텍스 패턴. **`ConfigureSecretMasterKeys("")`는 암묵 세트로 리셋** — 테스트 상태 오염 방지용 공식 리셋 의미(단일 초기화 계약, M-4). main.go는 Task 1에서 호출하지 않는다(진입점에서 env 읽기·또는 첫 사용 시 파싱 — 구현자 선택, 테스트는 setter로).
- **암묵 legacy 항목(항상 포함)**: `legacy` → `sha256(기존 credentialKey() 체인의 seed)`. 즉 legacy AES 키는 별도 설정 없이 기존 체인에서 파생되어 세트에 `legacy`로 등록(원문 "legacy:<the current credential-key value>"). env 스펙에 명시적 `legacy` id가 오면 파싱 에러(충돌 방지).
- **[M-3 제약] 암묵 legacy 항목은 첫 사용 시점에 도출한다 — package init에서 미리 계산·물화하는 것 금지.** 근거: init 시점에는 `ConfigureCredentialKey`가 아직 호출되지 않아 공개 dev 시드로 세트가 물화되고, 프로덕션에서 그 키로 `v2:legacy:` 데이터가 무음 축적될 수 있다. 첫 사용 시 도출이면 main.go의 `ConfigureCredentialKey(cfg.Security.CredentialKey)` 이후 실제 설정 키를 반영한다. T7이 재구성 추적을 검증.
- **미설정 시**: 키 세트 = `{current: "legacy"}` 단일 항목 → writer가 `v2:legacy:…` 발산 (가정 A1).
- 파싱 거부 조건: 콜론 누락, 빈 key_id, 빈 key_material, 중복 key_id, 명시적 `legacy`, key_id에 `:`·공백 포함. 앞뒤 공백은 trim. 첫 항목이 current(writer가 사용).

### 함수 시그니처
```
EncryptSecretV2(plain string) (string, error)   // "" → "" (빈 값은 envelope 없이 유지, 선택적 비밀 보존)
DecryptSecretV2(value string) (string, error)   // 엄격: "v2:" 접두만 수용. 미지 key_id → key_id를 에러에 포함
ClassifySecret(value string, field SecretField, declaredSecret bool) SecretFormat
    // SecretFormat: FormatV2 | FormatLegacy | FormatPlaintext | FormatEmpty | FormatUnknown | FormatNotSecret
ReadSecretField(value string, field SecretField, declaredSecret bool) (string, error)
    // classify → V2: 복호 / LEGACY: 기존 DecryptSecret 경유 복호 / PLAINTEXT: 원문 반환 / EMPTY: "" / UNKNOWN: 에러
AggregateFormats(formats []SecretFormat) FieldCounts   // 인벤토리 집계 순수 함수 (T18 대상)
```
- `declaredSecret` 파라미터가 **혼재 선언 컬럼 게이트**다(원문 r2.4 §4.3: "classify takes a caller-supplied declared-secret gate per value; the registry alone must never decide"). row 4(스케줄 variables)에서만 의미를 가지고, 일반 컬럼에서는 무시된다(문서화+T15 검증). **게이트 우선**: 혼재 컬럼에서 declared=false인 값은 envelope 여부와 무관하게 `NOT_SECRET-by-declaration`.
- `ReadSecretField`에도 같은 게이트가 전달된다 — `ops_schedule.go:152`의 `variable.Secret` 선언이 런타임 게이트 값이다(H-2).
- `ReadSecretField`가 **런타임·마이그레이션 공용 단일 구현**이다. classify는 (레지스트리, 값)의 순수 함수 — **예외: 혼재 선언 컬럼**(원문 r2.4가 명시한 예외 조항). 전역 상태는 키 세트 조회만.
- envelope 파싱: `v2:<key_id>:<base64url(nonce‖ct)>` — `strings.SplitN(value, ":", 3)` 정확히 3분할, key_id는 `[^:]+`, ct는 RawURLEncoding 검증 + GCM open.
- v2 암호화는 키 파생(`sha256(seed)`)·nonce·GCM 구성을 기존 방식과 동일하게 — `secret.go` 코드를 복제하지 말고 crypto/rand·sha256 조립만 신규 파일에 (기존 함수 재사용은 불가능하므로 동일 로직 재작성, 주석으로 대응 관계 명시).

### 기존 EncryptSecret/DecryptSecret 호환 전략 — **병존, 래핑 아님**
- 기존 2함수는 시그니처·본체 무변경. Phase B 이후 프로덕션 호출지 0이 되지만 legacy 판독(`ReadSecretField` LEGACY 분기 내부 사용)과 기존 테스트가 계속 사용한다. Step 4에서 별도 폐기.

### classify 계약 (§4.3 r2.4 의사코드 준수)
```
field 미등재(lookup 실패)                       → NOT_SECRET (skip)
혼재 컬럼 + declared=false                      → NOT_SECRET (선언 기반 skip — 게이트가 레지스트리보다 우선)
NULL/빈 문자열                                  → EMPTY
"v2:" 접두 + 3분할 파싱 성공 + key_id 등록 + base64 유효 → V2 (L-1: 파식·등록 검증까지 통과해야 V2.
                                                       malformed v2("v2:", "v2:k1:!!!")는 V2가 아니라 UNKNOWN)
E-legacy + rawurl base64 디코드 + legacy GCM open 성공 → LEGACY
P-class                                         → PLAINTEXT
그 외                                           → UNKNOWN → 마이그레이션 halt·검역 보고
```
- base64 함정 테스트 필수(T10): E-class에서 디코드는 되지만 GCM이 실패하는 문자열은 UNKNOWN이지 PLAINTEXT가 아니다.

### UNKNOWN의 런타임 동작
- **E-class 런타임 판독**: fail-closed 에러 — `domain.go:222`의 현행 동작과 동일. UNKNOWN이 평문으로 흘러들지 않는다.
- **domain.go:102 마스크 힌트 경로**: 호출부의 `err == nil` 폴백을 원문 그대로 유지 → UNKNOWN/EMPTY 시 힌트 `"Configured"` (현행과 동일). 폴백 삭제는 Step 4.
- **halt는 Step 2 마이그레이션 도구에서만** 발동. Task 1의 인벤토리 CLI는 UNKNOWN을 카운트·보고하고 exit 0(가정 A7).
- **P-class 런타임**: 접두 없으면 무조건 PLAINTEXT 패스스루 — UNKNOWN이 발생하지 않는다(구조적으로).

---

## 3. P-class writer 전환 — **Step 1 제외는 원계획 계약, 이연은 승인 완료**

원문 r2.4 §4.4 Step 1 문안이 교체되어 "P-class fields: writer conversion is its own prerequisite task — NOT switched in Step 1"이 명문화되었다. 본 태스크의 P-class 제외는 더 이상 판단 사항이 아니라 원계획 준수다.

**사유(원문 r2.4가 채택한 실측 근거)**: (a) KubeConfig(`model/k8s.go:20` json 직렬화, `controller/k8s.go:23` 회신, `service/k8s.go:728` 무조건 재저장)·NotifyChannel Secret(`service/notify.go:205`)은 저장값이 UI에 원문 회신 후 재제출 — writer만 바꾸면 envelope이 평문으로 취급되어 재암호화된다; (b) P-class 참조 157곳이 `ReadSecretField`를 우회하는 평문 리더.

**순서 규칙(원문 r2.4 §4.3 "P-class ordering rule" — H-1 교정)**: P-class writer 전환 태스크는 **Step 2의 P-class 패스에 선행 필수**다. Step 3 게이트("모든 §4.1 필드가 V2, PLAINTEXT 0")는 P-class writer가 평문을 발산하는 한 통과 불가능하고, Step 2가 P-class 컬럼을 v2로 재작성하면 157곳 평문 리더가 파손된다. 런타임 듀얼 판독과는 직교 — **게이트가 순서 의존적**이다. 즉 본 이연은 "언제든 가능"이 아니라 "Step 2 P-class 패스 직전까지는 반드시 완료"라는 데드라인이 붙은 이연이다.

**승인 게이트(M-5)**: 이연은 메인이 승인 완료. 재개 조건 = Step 2 P-class 패스 착수 직전. state.json은 메인이 관리하므로 본 계획서에만 기록한다.

**후속 전환 태스크가 수행할 사전 감사 목록 (본 계획의 인계물 — 11그룹, r2.4 15행 기준)**: AssetCredential, AssetCloudAccount, AssetGateway, K8sCluster.KubeConfig, IntegrationFinOps, MonitorDatasource, OpsImageRegistry, IntegrationAIModel.APIKey, LDAPConfig.BindPassword, NotifyChannel(Secret+WebhookURL 조건부), AssetDatabase.Password — 각각 (a) 저장값의 API 응답 노출 여부, (b) 갱신 시 공백/마스크 처리 의미론, (c) 판독 경로 전수(DecryptSecret 비경유 원문 사용처).

Task 1이 제공하는 것: 레지스트리의 P 11행 등록 + classify PLAINTEXT 분기 + `ReadSecretField`의 P-class 패스스루 — 후속 전환은 호출지 교체만으로 끝나도록 기반 완성.

---

## 4. 인벤토리 CLI (`inventory-secrets`)

- **목적**: §4.1 15행 전 필드의 사전 비행 artifact — 필드별 v2/legacy/plaintext/empty/unknown/notsecret 카운트.
- **실행형태**: `./ops-admin inventory-secrets [--config config.yaml] [--json]`. main.go 디스패치는 config 로드 **이전**, `len(os.Args) >= 2 && os.Args[1] == "inventory-secrets"` 가드(L-2) — 일반 서버 기동 경로에 영향 0.
- **초기화 시퀀스(H-4 — 이 순서가 틀리면 전수 오판)**:
  1. `cfg, err := config.Load(path)` — 실패 시 exit 1.
  2. `util.ConfigureCredentialKey(cfg.Security.CredentialKey)` — **누락 시 legacy 키가 dev 폴백 시드로 도출되어 기존 데이터 전수가 UNKNOWN으로 오판된다.**
  3. `OPS_SECRET_MASTER_KEYS` env 읽어 `ConfigureSecretMasterKeys` — 파싱 에러 시 exit 1.
  4. `store.NewDB(cfg)` 재사용. **AutoMigrate·Seed·CanonicalizeSeedLocalization 미실행**(읽기 전용 명령). 오류 시 exit 1.
- **스캔**: 레지스트리 15행 순서(§4.1 행 순서)대로 `SELECT id, <column> FROM <table>` — 1000행 배치 스캔으로 메모리 보호. 모델 임포트 없이 레지스트리의 table/column 문자열로 직접 쿼리 (util 의존 역전 방지).
- **row 4 특수 처리(H-2 계약 준수)**: `ops_schedule_task.variables`는 행 단위가 아닌 map **값 단위**로 classify하되, **스크립트 메타데이터 조인으로 declared 게이트를 구성**한다(ops_schedule_task → ops_script `Variables[].Secret` 선언). declared=false 값은 UNKNOWN/PLAINTEXT가 아니라 **NOT_SECRET-by-declaration 버킷**에 계상(원문 r2.4: Step 3 "zero PLAINTEXT" 게이트도 declared 값만 계상).
- **출력**: 기본 = 사람이 읽는 표(모델·필드·class·총계·v2/legacy/plaintext/empty/unknown/notsecret) + 요약. `--json` = 기계 판독 JSON (필드별 카운트 + unknown 행 상세: table/id/field/값 길이).
- **UNKNOWN 해석 가이드(M-6 — 원문 r2.4 §4.3 반영)**: 보고서에 명시 — "E-class 필드의 과거 평문 데이터가 대량 UNKNOWN으로 집계되는 것은 **예상된 사전 마이그레이션 상태**이지 사고가 아니다. 보고서는 정확히 그 규모를 재기 위한 것이다." (E-class가 legacy envelope 전용이었으므로 평문이 섞여 있었다면 UNKNOWN이 정상 노출이다.)
- **종료코드**: 정상 종료는 언제나 0 (UNKNOWN이 있어도 — 측정이지 게이트가 아니다, 게이트는 Step 3. 가정 A7). config/DB/파싱 오류만 1.

---

## 5. 위험 지점과 검증 방법

| # | 위험 | 검증 / 완화 |
|---|---|---|
| R1 | v2로 쓴 값을 읽지 못하는 E-class 경로 누락 | 호출지 전수 조사 완료(§0 표, 10곳). Phase B 후 `grep -rn "util\.\(Encrypt\|Decrypt\)Secret(" backend/service` = 0으로 기계 확인. **기존 테스트 커버는 ops_script_test.go:31 1건뿐이므로(CR-1 실측) 신규 왕복 테스트 T16/T17이 DNS·SSL 경로의 실제 검증이다.** 암호문 복사 경로(domain.go:145–167, ssl_acme.go:286)는 포맷 무관 확인 완료. |
| R2 | 키 세트 미설정 환경에서 writer 장애 | 암묵 `{current: legacy}` 세트로 미설정 배포도 `v2:legacy:` 발산(A1). 테스트 T7. |
| R3 | 키 회전 후 구 key_id 데이터 판독 실패 | 세트에 old key를 남기면 key_id 라우팅으로 판독(원문 설계). 테스트 T5. Step 1에서는 키 제거 없음(계약). |
| R4 | base64 함정 — 평문이 LEGACY로 오분류 | classify는 GCM open 성공까지 확인. 테스트 T10. |
| R5 | schedule 혼재 컬럼 오분류 | declared 게이트 계약(H-2·원문 r2.4): 선언 기반 NOT_SECRET, 런타임은 ops_schedule.go:152 선언 사용. T15. |
| R6 | 롤백 시 v2로 쓰인 행을 구 빌드가 못 읽음 | §4.5: Step 1 PR은 v2 판독 지원을 영구히 가져감. revert가 필요하면 v2 쓰기 발생 후에는 revert보다 재전진이 정석 — 계획서에 운영 주의로 기록(코드 완화 없음, 원문 계약). |
| R7 | 인벤토리 CLI가 운영 DB에 부하 | 읽기 전용·배치 스캔·단발 명령. 인덱스 없는 전수 스캔은 의도된 동작(전수 분류가 목적). |
| R8 | 신규 코드가 기존 crypto 의미를 바꿔 새치기 | 기존 함수 무수정 + 신규 파일 격리 + `git diff backend/util/secret.go` 공백 claim으로 봉인. |
| R9 | 초기화 순서 오류(CLI에서 legacy 키 dev 폴백 도출) | H-4 시퀀스 1–3 고정 + 순서를 테스트/문서로 명시. dev 폴백 자체는 Step 4까지 현행 유지(보존 제약 #8). |
| R10 | sqlite 테스트 의존성 파급 | 테스트 전용(프로덉션 import 금지 — 보존 제약 #10). mysql 전용 GORM 모델의 sqlite AutoMigrate 호환성은 T16/T17 작성 시 필요한 모델만 마이그레이트해 최소화. |
| R11 | 암묵 legacy 키가 package init에서 물화(M-3 시나리오) | 첫 사용 시 도출 제약 + T7(ConfigureCredentialKey 재설정 후 도출 키 추적). |

**롤백 가능성 판정**: Phase A/C는 완전 롤백 가능(사용자 없는 신규 코드/별도 명령). Phase B는 코드 롤백(revert) 가능하나 **v2 쓰기가 발생한 후의 revert는 해당 행을 구 빌드가 판독 불가**로 만든다(R6) — 배포 후 되돌리기보다 전진 수정 원칙을 명시. DB 스키마·데이터는 Task 1에서 어떤 것도 건드리지 않으므로 데이터 롤백 자체가 불필요.

---

## 6. 테스트 계약 (TDD — 작성 순서 = 구현 순서)

Phase A — `backend/util/secretv2_test.go` (기존 `secret_test.go` 관례 준수: `t.Setenv`·`ConfigureCredentialKey`·Cleanup):

| # | 테스트 | 검증 |
|---|---|---|
| T1 | `TestEncryptSecretV2RoundTrip` | 복호 평문 일치 + 형식 `^v2:[^:]+:[A-Za-z0-9_-]+$` |
| T2 | `TestEncryptSecretV2EmptyStaysEmpty` | `""` 입력 → `""` 반환 (선택적 비밀 보존) |
| T3 | `TestDecryptSecretV2RejectsMalformed` | `"v2:"`, `"v2:k1"`, `"v2:k1:!!!"`(무효 base64), 잘린 ct, 접두 없음 → 전부 에러 |
| T4 | `TestDecryptSecretV2UnknownKeyID` | 미등록 key_id → 에러 메시지에 key_id 명시 |
| T5 | `TestKeySetRotationDualDecrypt` | key A로 쓰기 → 세트를 [B(current), A(old)]로 재구성 → 판독 성공 (key_id 라우팅) |
| T6 | `TestParseMasterKeys` | 정상 1개/복수·순서, 빈 스펙(암묵 legacy 세트), **`ConfigureSecretMasterKeys("")` = 암묵 세트 리셋(M-4)**, 중복 id·명시적 `legacy`·빈 id·빈 키·콜론 누락·공백 trim 거부 |
| T7 | `TestImplicitLegacyKeySetDerivesAtFirstUse` | master keys 미설정 → `v2:legacy:` 발산 + 자체 판독 성공. **보강(M-3): `ConfigureCredentialKey`를 K1→K2로 재설정하면 이후 발산하는 암묵 legacy envelope이 K2 도출 키로 판독된다(첫 사용 시 도출·init 물화 없음)** |
| T8 | `TestReadSecretFieldDualKey` | E-class에서 v2 값·legacy 값(`EncryptSecret` 산출) 모두 평문 복원 (declaredSecret=true) |
| T9 | `TestClassifyAllBranches` | NOT_SECRET·EMPTY·V2·LEGACY·PLAINTEXT·UNKNOWN 6분기 전수. **보강(L-1): V2 분기는 3분할 파싱+key_id 등록+base64 유효까지 통과해야 V2 — `"v2:"`, `"v2:k1:!!!"` 등 malformed v2는 V2가 아니라 UNKNOWN로 버킷 분리** |
| T10 | `TestClassifyBase64Trap` | E-class에서 base64로 디코딩되지만 GCM 실패하는 문자열 → UNKNOWN (PLAINTEXT 아님) |
| T11 | `TestReadSecretFieldUnknownFailsClosed` | E-class UNKNOWN → 에러 (fail-closed) |
| T12 | `TestReadSecretFieldPlaintextPassthrough` | P-class 무접두 평문 → 원문 그대로 |
| T13 | `TestSecretRegistryMatchesInventory` | **15행·table+column 유일·class 분포(E 4행/P 11행)·§4.1 r2.4 표와 대응·row 14 webhook_url Conditional 플래그·row 4 MixedDeclaration 플래그** |
| T14 | `TestConcurrentKeySetAccess` | `ConfigureSecretMasterKeys` 병행 갱신·판독 — `-race` 하에서 |
| T15 | `TestClassifyMixedDeclarationGate` | 혼재 컬럼: declared=false + legacy 값/v2 값/평문 → 전부 NOT_SECRET(선언 우선); declared=true + legacy → LEGACY; declared=true + v2 → V2; declared=true + 비복호 문자열 → UNKNOWN. 일반 컬럼은 declaredSecret 플래그 무시(true/false 결과 동일) |
| T18 | `TestAggregateFormats` (M-2 — 필수, util 레벨) | 집계 순수 함수: SecretFormat 슬라이스 → 카운트 버킷. 혼재 컬럼의 NOT_SECRET-by-declaration 계상 포함(A8 계약) — 인벤토리 CLI의 집계 근거 |

Phase B — `backend/service/secretv2_roundtrip_test.go` (신규·필수, sqlite in-memory 하neus — CR-1):

| # | 테스트 | 검증 |
|---|---|---|
| T16 | `TestSavePublicDNSAccountWritesV2EnvelopeAndReadsBack` | sqlite 하neus로 `SavePublicDNSAccount` 저장 → DB 저장값이 `^v2:` 포맷 → `ReadSecretField`(publicProvider 계열 판독 경로)로 평문 복원 왕복. 기존 legacy 값(사전 주입)도 동일 판독 경로에서 복원(듀얼 확인) |
| T17 | `TestManualCertificatePrivateKeyRoundTrip` | 테스트 내 자체 생성 키(PEM)로 수동 인증서 저장 경로(ssl_certificate.go:246 계열) → 저장값 `^v2:` → 판독 경로(522 계열)에서 원문과 바이트 일치 |

기존 `ops_script_test.go:31`(스케줄 변수 legacy 암호 유지)은 Phase B 교체 후에도 무수정 통과해야 한다(회귀 0의 실측 기준점).

Phase C — `main_inventory_test.go`(선택): 출력 조립 최소 테스트. 집계 계약 검증은 T18(util)이 담당.

**커버리지(H-3 — 측정 단위 확정)**: `-coverprofile` 기반 **신규 코드(`secretv2.go`·`secret_registry.go`) 한정 ≥80%**. 측정: `go test ./util/ -coverprofile=c.out && go tool cover -func=c.out`에서 신규 2파일 행의 합산. "util 패키지 전체 80%"는 현황(신규 도입 전 대부분 미커버)에서 불가하므로 채택하지 않는다.

---

## 7. 보존 제약 (③구현 프롬프트에 verbatim 복사)

1. `backend/util/secret.go`의 기존 함수(`ConfigureCredentialKey`, `credentialKey`, `EncryptSecret`, `DecryptSecret`)는 시그니처와 본체를 수정하지 않는다 — legacy 경로는 dual 판독·기존 테스트가 계속 사용한다.
2. config 패키지와 config.yaml을 변경하지 않는다 — `OPS_SECRET_MASTER_KEYS`는 환경변수에서만 읽는다 (config wiring은 후속 태스크).
3. DB 스키마·`store/migrate.go`·`store/seed.go`를 변경하지 않는다.
4. `router/`와 `controller/`를 변경하지 않는다 — HTTP 표면 무추가.
5. `service/domain.go:102`의 `err == nil` 마스크 힌트 폴백은 유지한다 (삭제는 Step 4).
6. P-class(평문) 필드의 저장 포맷은 바꾸지 않는다 — Step 1 제외는 원계획 r2.4 계약이며 순서 규칙(Step 2 P-class 패스 선행 전제)은 §3에 기록되어 있다.
7. 어떤 데이터 재작성(마이그레이션) 코드도 포함하지 않는다 — Step 2 소관이다.
8. dev 폴백 시드 체인(env `OPS_ADMIN_CREDENTIAL_KEY` → `OPS_ADMIN_JWT_SECRET` → dev 키)과 프로세스-로컬 폴백은 현행 유지 — Step 4/릴리스 C 소관.
9. 기존 E-class 소비처의 **동작**은 유지한다: legacy 데이터 판독 성공(회귀 0), fail-closed 경로는 여전히 fail-closed, 마스크 힌트는 여전히 폴백. `ops_script_test.go` 기존 테스트는 무수정 통과해야 한다.
10. `gorm.io/driver/sqlite`는 테스트 파일에서만 import한다 — 프로덕션 코드(main·service·util·store)에서 참조하면 안 된다.

---

## 8. 검증 요구 (claims — ④리뷰 주입용)

1. `cd backend && go build ./...` exit 0.
2. `cd backend && go test ./util/... ` exit 0, `go test ./... ` exit 0 (기존 테스트 회귀 0 포함).
3. `cd backend && go vet ./...` exit 0.
4. `git diff --stat backend/util/secret.go` — 출력 없음(무수정).
5. `git diff --stat backend/config backend/router backend/controller backend/store` — 출력 없음.
6. `grep -rn "util\.EncryptSecret(\|util\.DecryptSecret(" backend/service/ | grep -v _test.go` — 매치 0 (E-class 호출지 전량 V2/dual 전환).
7. `grep -c "func Test" backend/util/secretv2_test.go` ≥ 16 (T1–T15, T18).
8. `grep -n "SecretFields" backend/util/secret_registry.go` 존재 + T13이 **15행·class 분포(E 4/P 11)·row 4/14 플래그**를 단언.
9. `ls backend/service/secretv2_roundtrip_test.go` 존재 + `grep -c "func Test"` ≥ 2 (T16/T17).
10. `cd backend && go run . inventory-secrets --json` 가 dev DB에서 JSON을 내고 exit 0 (DB 불가 환경에선 `go run . inventory-secrets`의 인자 오류 경로 exit ≠0 확인으로 대체) — 구현자 환경 결과를 구현 보고서에 첨부.
11. `go test ./util/ -coverprofile=c.out && go tool cover -func=c.out` 리포트에서 `secretv2.go`·`secret_registry.go` 항목 합산 커버리지 ≥ 80% (수치 기재).
12. `grep -rn "gorm.io/driver/sqlite" backend --include="*.go" | grep -v _test.go` — 매치 0 (테스트 전용 의존성 준수).

---

## 9. 가정 명세 (불확실 요소의 명시적 처리)

- **A1 키 세트 미설정 시 암묵 legacy 세트**: `OPS_SECRET_MASTER_KEYS` 미설정 → 세트 `{current:"legacy"}` → `v2:legacy:…` 발산. 근거: Step 1은 "deploy; system runs normally" 상태여야 하므로 키 미설정 배포에서 쓰기가 죽으면 안 된다. 회전 도입 시 env 설정만으로 current 교체·legacy 판독 유지.
- **A2 key_id 문자 규칙**: 비어있지 않고 `:`·공백 불포함. base64url ct에는 `:`이 없어 3분할 파싱이 안전하다.
- **A3 P-class 이연 — 승인 완료**: 원계획 r2.4가 Step 1 제외를 명문화(본래 판정 사항이 계약으로 승격). 메인 승인 완료, 재개 조건 = Step 2 P-class 패스 착수 직전(순서 규칙, §3). state.json은 메인 관리 — 본 계획서에만 기록.
- **A4 철회(r2)**: AssetDatabase.Password는 원계획 r2.3에서 §4.1 15행째로 등재됨 → NOT_SECRET 처리 철회, 레지스트리 P로 편입, 후속 감사 11그룹에 포함.
- **A5 WebhookURL 조건부 편차(명시)**: §4.1 row 14는 "Secret (and WebhookURL when it embeds a path token)"로 조건부 필드를 포함한다. 레지스트리는 `notify_channel.webhook_url`을 `Conditional: true` 플래그로 **등재하되**, Task 1 인벤토리에서는 P-class 규칙 적용(오늘 저장값은 사실상 평문이므로 PLAINTEXT 계상이 정확한 규모 측정). path token 내장 여부 판정·마스킹·envelope 적용 여부는 후속 전환 태스크 판정으로 인계 — "레지스트리만으로 결정하지 않는다"는 혼재 컬럼 원칙과 같은 맥락의 조건부 편차임을 명시.
- **A6 SSLCertificateVersion.PrivateKeyCipher**: 판독 소비처 없음(§0 실측) — Task 1에서 호출지 교체 불필요. Step 2 분류 대상으로만 레지스트리 등록.
- **A7 인벤토리 exit 코드**: UNKNOWN이 있어도 exit 0 — 측정이지 게이트가 아니며 게이트는 Step 3. unknown 행 상세는 JSON에 포함해 수동 보수 가능하게 한다.
- **A8 혼재 선언 컬럼(r2 교정 — 원문 r2.4 §4.3 계약으로 대체)**: row 4의 비밀 여부는 컬럼이 아니라 값별 스크립트 선언(`OpsScript.Variables[].Secret`)에 있다. classify는 caller-supplied declared 게이트를 받고, declared=false 값은 NOT_SECRET-by-declaration(UNKNOWN 아님·PLAINTEXT도 아님). Step 3 "zero PLAINTEXT" 게이트는 declared 값만 계상. 인벤토리 CLI는 스크립트 메타데이터 조인으로 게이트를 구성한다. r1의 "비-접두 값 PLAINTEXT 버킷" 방식은 폐기(레지스트리/컬럼 단위 판정을 허용하는 설계였음).
- **A9 sqlite 테스트 의존성(r2 신설)**: 왕복 테스트가 실제 service 호출지를 통과하려면 in-memory DB 필수 — 기존 service 테스트는 mock 기반·mysql 드라이버만 존재. `gorm.io/driver/sqlite`를 테스트 전용으로 추가한다(프로덕션 import 금지). CR-1의 콜사이트 검증 요구를 닫는 최소 수단.

---

## 10. 산출 외 확인사항 (구현자에게 불요, 팀 리드 인계)

- **원계획 개정 인계(H-1)**: r2.4(d5d734f)에서 Step 1 P-class bullet 교체·§4.3 혼재 컬럼 계약·P-class 순서 규칙이 반영 완료 — 본 계획은 r2.4 기준으로 작성됨. 후속 P-class 전환 태스크 계획 시 §4.3 순서 규칙(Step 2 P-class 패스 선행 전제)이 착수 조건이다.
- Step 2 마이그레이션 명령은 본 계획의 `ClassifySecret`(declared 게이트 포함)·`ReadSecretField`·`EncryptSecretV2`·레지스트리를 그대로 소비한다(단일 구현 계약 — 단, classify 순수성 주장은 혼재 컬럼 예외를 포함하는 r2.4 계약문으로 정확히 서술해야 한다).
- Step 4에서 폐기할 것: `domain.go:102` 폴백, legacy 키 세트 항목, `util.EncryptSecret/DecryptSecret` 잔여 사용.
- §4.6(config.yaml git 추적 제거)·§4.7(권한 시드)은 본 태스크와 독립 — 착수하지 않는다.
