# V2 Phase -1 · Task 3 계획 — 권한 봉쇄·시드 마이그레이션 (§4.7) · r2

- 원천: `/tmp/v2-rev/docs/architecture/multi-infrastructure-control-plane-v2.md` **§4.7 (5단계, 재판단 금지)** + 게이트 G-3/G-4/S3 (§20 Phase -1, §24).
- 범위: §4.7 5단계 전체 — ①열거 ②민감 분류(`docs/security/sensitive-routes.txt`) ③권한 문자열 할당(operation definitions) ④역할 시드 마이그레이션 ⑤리플레이 검증. 콘솔 WS 티켓(§4.8)·mutating-GET 전환(§4.8)·CI 워크플로 7종 구축(§4.10)은 **제외** — 단, route-coverage 게이트가 요구하는 *artifact와 go test 내장 게이트*는 T3가 만든다(§2.2). **레드액션 구현도 제외** — opdef 메타데이터 기록까지만(§2.3, CR-2).
- 개정 이력: r1 → r2 (2렌즈 적대 반영 — CR-1 그랜트 one-shot 분리, CR-2 민감 GET 누락 3건, CR-3 역방향 단언, H-1 라우트 수 435, H-2 claim 8 재설계, H-3 역할 생성 순서 계약, M-1 결정적 정렬, M-2 단일커넥션, M-3 해석 기록).
- 실측 기준 커밋: `7705845` (main, clean). CR-2·H-1 근거는 본 세션 코드 재확인 완료.

---

## 0. 현재 코드 실측 (계획의 사실 기반)

### 라우트 면적 (router/router.go 495행 전수 열람)
- 총 **435 라우트** = `/ping` 1 + public api 그룹 7(login/refresh/logout/systemConfig public/integration public/WS 2) + **authGroup 425** (GET 187 · POST 135 · PUT 44 · DELETE 59) + **`engine.Static("/uploads")`의 GET·HEAD 2** (gin v1.11.0 — Static이 GET+HEAD 등록, 리드 실측·go.mod:6 확인).
- authGroup 전 라우트에 `middleware.Auth(db)` + `middleware.OperationLog(db)` 부착 (router.go:49). **자원 단위 권한 검사는 domains 구간 36개뿐** (라인 52–87: RequirePermission 30 + RequireAnyPermission 4 + RequireCreateOrEditPermission 2). 나머지 **389개는 인증만**.
- 비-GET(authGroup 기준) = 238개 — §4.7 규칙상 전부 민감. GET 187개 중 민감은 시크릿 소유 응답 서브셋(§2.3). 민감 총량 추정 **258–265**, 이 중 신규 부착 ≈ **225**.
- **SENSITIVE 집합의 표면 정의 = authGroup 425로 한정** (M-3 해석 기록): §4.7이 말하는 "400+ currently authentication-only routes"는 authGroup이며, public 비-GET 3종(login/refresh/logout)은 인증 전 단계라 권한 요구가 성립하지 않고, WS 2종은 §4.8 콘솔 티켓 태스크 소관, uploads 정적 2종(GET/HEAD 파일 서빙)은 API가 아니다. 인벤토리 아티팩트는 **435 전수를 필터 없이** 수록한다(§2.1).

### 권한 메커니즘 (변경 대상 아님)
- `middleware/permission.go` — `RequirePermission(db, perm)` → `RequireAnyPermission`: 쿼리 `sys_admin_role → sys_role_menu → sys_menu WHERE m.value IN ? AND m.menu_status = 1` (permission.go:60-64). **슈퍼어드민 코드 바이패스 없음** — 권한은 오직 menu-value 그랜트로만 결정된다.
- 헬퍼 2종 추가 존재: `RequireCreateOrEditPermission(db, create, edit)` (payload id 유무 분기, permission.go:23), `RequireAnyPermission(db, perms...)`.
- 거부 응답: `httpx.Failed(c, 403, "Permission denied")` — 리플레이 테스트의 부정 마커.

### 권한 어휘 — 이미 풍부하게 시드됨 (핵심 발견)
`store/seed.go` `seedApplicationMenus`가 **전 도메인 type-3 버튼 권한값을 이미 sys_menu에 시드**한다: `system:*`(admin/role/menu/dept/post/config/loginlog/operationlog), `assets:*`(host/hostgroup/credential/cloudaccount/database/gateway/k8s), `ops:*`(script/quickexec/schedule/job), `applications:*`, `notify:*`, `monitor:*`, `domains:*`. 이 값들은 프론트 `v-permission`이 이미 참조한다. **라우트에 부착된 건 domains 하위집합뿐** — 즉 신규 문자열 최소화: 기존 어휘 재사용이 우선, 갭만 신규(§2.4).
- `ensureMenu`는 value 기준 idempotent (seed.go:445 근방) — 시드 재실행 안전.
- `seedSuperRolePermissions`(seed.go:431)가 super-admin 역할에 **모든 메뉴를 매 부팅마다 재그랜트** — super-admin "전 권한 상시" 불변식의 현행 구현. **r1의 오류 교정(CR-1)**: 이 관례를 일반 역할까지 확장하면 신규 역할 자동 전그랜트·운영자 박탈 되돌리기가 발생 — §2.5처럼 분리한다.
- 메뉴 트리 UI 영향 실측: `Service.CurrentMenus`(service/service.go:548)는 `menu_status = 1 AND menu_type IN (1,2)`만 사이드바로 내보낸다 → **type-3 row는 사이드바에 절대 노출 안 됨**. 프론트 `web/src/layouts/MainLayout.vue:192`도 `menuType !== 3` 필터로 동일 보장(이중).
- `model.SystemConfig`은 고정 컬럼 단일 row(model/system_config.go:5-19) — 확장 필드 없음 → **원샷 마커는 sys_menu row로 구현**(§2.5, 스키마 변경 회피).

### 테스트·CLI 인프라
- T1 선례: `glebarez/sqlite` pure-Go 테스트 의존성(go.mod에 이미 존재, 프로덕션 import 금지가 T1 보존제약 #12), **`newRoundtripDB`의 `SetMaxOpenConns(1)` 단일커넥션 패턴**(service/secretv2_roundtrip_test.go:52-55 — M-2에서 리플레이 DB가 그대로 재사용), main.go 서브커맨드 디스패치 선례(`inventory-secrets`, main.go:21).
- `store.AutoMigrate`(migrate.go, 모델 90개)·`store.Seed`(9 스텝) — 리플레이 테스트가 sqlite에서 그대로 재사용 가능.
- 인증: `auth.GenerateToken(userID, username, sessionID)`(HS256, env `OPS_ADMIN_JWT_SECRET`) + `model.AuthSession` row — 미들웨어가 session 만료·idle 검사(auth.go)하므로 테스트는 미래 만료 session row를 만든다.
- 기존 CI: `.github/workflows/korean-localization-guard.yml` 1종뿐 — route-coverage 워크플로 파일 자체는 §4.10 CI-베이스라인 태스크 소관(§8 인계).

### 민감 GET 후보 — 실측 근거 (r2: CR-2 3건 추가, 전부 본 세션 코드 재확인)
| 후보 | 근거(실측) |
|---|---|
| **`/asset/host/list`·`/info` (r2 추가, CR-2)** | `GetAssetHostList`(service/service.go:1083)·`GetAssetHost`(:1113)가 `Preload("Credential").Preload("Gateway").Preload("Gateway.Credential").Preload("CloudAccount")` 후 `model.AssetHost`를 그대로 반환 — 연관 `Password`(asset.go:65, omitempty지만 저장값 직렬화)·`SecretKey`(asset.go:83) 무마스킹 노출 |
| **`/monitor/datasource/list`·`/options`·`/info` (r2 추가, CR-2)** | 모니터 데이터소스 구조체 `Password string json:"password"`·`Token string json:"token"`(service/monitor.go:33-34) — `json:"-"` 조차 없어 항상 직렬화 |
| `/notify/channel/list`·`/info`·`/options` | `mapNotifyChannel`이 `secret` 원문 회신 (service/notify.go:205) |
| `/k8s/cluster/*`(list/info/detail) | `model.K8sCluster.KubeConfig` json tag로 원문 회신 (T1 계획 §0 실측 인용 — model/k8s.go:20) |
| `/asset/credential/list`·`/info`·`/options` | 스펙 명시 "asset credential reads" (마스킹 `******` 있으나 분류 규칙상 편입) |
| `/k8s/secret/detail` | K8s Secret 객체 본체 판독 |
| `/k8s/configmap/detail`·`/k8s/pod/logs`·`/asset/service/workload/logs` (r2 후보군, CR-2) | 설정 맵·컨테이너 로그 원문 — 자격증명 상시 혼재. 큐레이터 편입(§2.3 원칙: 애매하면 편입) |
| `/asset/service/diagnosis/environment`·`/processes` | 환경변수·프로세스 표면(자격증명 상시 혼재) |
| `/asset/service/diagnosis/run` | GET이지만 진단 **실행** 트리거(사실상 non-GET — 큐레이터 편입) |
| `/asset/cloudAccount/list`·`/info` | `SecretKey` 필드 존재(model/asset.go:83), 응답 마스킹 미확인 → 편입 보수 |
| `/dbms/task/download`·`/dbms/backup/download` | 데이터 덤프 반출 |
| `/admin/list`·`/admin/info` | 계정 자료 회신(스펙 "account/password management") |
| **제외 확정**: `/system/ldap/config` (GET) | 응답이 `bindPasswordSet` bool뿐 — 비밀값 미회신 (service/ldap.go:168) |
| **제외**: `/profile`, 각 도메인 일반 list/info GET | 자기 프로필·비발발 목록 — NORMAL (S1 DTO 스캐너가 별도 감시) |

---

## 1. 파일 배치 · Phase 분해 (15파일 → 6 Phase, 각 ≤5파일)

| 파일 | 신규/수정 | 내용 |
|---|---|---|
| `backend/opdef/opdef.go` | 신규 | `Def` 구조체·`All()`·검증(`Validate`)+`Middleware(db, def)` 팩토리. 의존: middleware만(사이클 없음 — middleware는 opdef 미참조) |
| `backend/opdef/defs_domain.go` | 신규 | 기존 domains 36라우트의 정의 테이블(권한 문자열 **byte 동일** 이관) |
| `backend/opdef/defs_system.go` | 신규 | systemConfig·ldap·admin·role·menu·dept·post·로그 2종 (≈53 라우트 중 민감분) |
| `backend/opdef/defs_asset.go` | 신규 | asset 전 구간 + dbms + monitor (≈160 라우트 중 민감분 — CR-2로 monitor datasource GET 3종·asset host GET 2종 편입) |
| `backend/opdef/defs_ops.go` | 신규 | ops·applications·notify (≈94 라우트 중 민감분) |
| `backend/opdef/defs_monitor.go` | 신규 | monitor + k8s (≈99 라우트 중 민감분) |
| `backend/opdef/defs_integration.go` | 신규 | integration(nav·ai)·finops (40 라우트 중 민감분) — 어휘 전무 → 신규 문자열 집중지 |
| `backend/opdef/opdef_test.go` | 신규 | 테이블 불변식(T1–T5, §4) |
| `backend/store/seed.go` | 수정 | Seed 스텝 3개 추가(§2.5): `seedRoutePermissionMenus`(상시), `seedSuperAdminRoutePermissions`(상시), `migrateRoleRoutePermissionsOnce`(원샷·마커 게이트) |
| `backend/store/seed_route_permissions_test.go` | 신규 | 시드 마이그레이션 단위 테스트(T6–T11) |
| `backend/router/router.go` | 수정 | 민감 라우트에 `opdef.Middleware(db, opdef.XXX)` 부착 — 경로·핸들러 무변경, 약 250줄 기계적 편집 |
| `backend/router/routes_inventory_test.go` | 신규 | 아티팩트 2종 골든 diff 게이트(T12–T13) |
| `backend/router/authz_replay_test.go` | 신규 | 리플레이 + 제로그랜트 커버리지 스캔 + 역방향 단언(T14–T17) |
| `docs/security/sensitive-routes.txt` | 신규(생성) | opdef 테이블에서 생성·커밋하는 분류 artifact (§2.2 형식) |
| `docs/security/route-inventory.txt` | 신규(생성) | `engine.Routes()` 전수 **435** dump — G-3 "empty diff"의 커밋 기준선 |

**Phase (의존 순서, 각 독립 검증)**

- **Phase A — 정의 커널 + domains 이관 (3파일)**: `opdef.go`, `defs_domain.go`, `opdef_test.go`. 검증: `go build ./...` + `go test ./opdef/...`. 아무 라우트도 안 바꾼 상태로 커널 불변식 선확보.
- **Phase B1 — 정의 확장 1 (3파일)**: `defs_system.go`, `defs_asset.go`, `defs_integration.go`. 검증: `go test ./opdef/...` (전체 테이블 불변식 — 키 유일성 등은 누적 검증).
- **Phase B2 — 정의 확장 2 (2파일)**: `defs_ops.go`, `defs_monitor.go`. 검증: 동일.
- **Phase C — 시드 마이그레이션 (2파일)**: `seed.go`(수정 — 스텝 3개), `seed_route_permissions_test.go`. 검증: `go test ./store/...` + `go test ./...` (기존 회귀 0) — **T6–T11이 CR-1 회귝(신규 역할 미자동그랜트·박탈 생존)을 데이터 단면에서 단언**. 이 시점까지 라우트가 검사 안 하므로 배포 무해 단독 커밋 가능.
- **Phase D — 라우터 부착 + 아티팩트 (3파일)**: `router.go`(수정), `routes_inventory_test.go`, 아티팩트 2종 생성(커밋). 검증: `go build` + 골든 diff 통과 + 기존 `go test ./...` 녹색. 행동 검증(403/allow)은 Phase E 테스트가 폐쇄.
- **Phase E — 리플레이·커버리지 검증 (1파일)**: `authz_replay_test.go`. 검증: T14–T17 전 통과 = **G-3·G-4·S3의 go test 내장 성립**.

---

## 2. 계약 설계 (계획이 확정하는 8요소)

### 2.1 열거 메커니즘 (§4.7 step 1)
- **테스트 기반 채택**("a route-inventory command (or test)"의 test 분기 — 서브커맨드 불필요): `routes_inventory_test.go`가 glebarez sqlite에 엔진을 **실등록**해 `router.New` 후 `engine.Routes()`를 walk, `"{METHOD} {fullPath} -> {handlerName}"` 직렬화.
- **라인 수 계약(H-1)**: dump는 **435본문 라인 = 1(/ping) + 7(public) + 425(authGroup) + 2(uploads GET·HEAD — `engine.Static`이 gin v1.11.0에서 GET+HEAD 등록)**. **필터 없이 전수 수록** — uploads 쌍을 제외하는 필터를 만들면 생성기가 복잡해지고 진짜 라우트 추가를 흡사 은폐할 수 있어 제외하지 않는다(해석 명시, M-3). 아티팩트 파일 = 헤더 주석 3행 + 본문 435라인.
- Gin `Routes()`는 등록된 전 라우트의 Method/Path/핸들러명을 주지만 **미들웨어 체인은 노출하지 않는다** — 따라서 미들웨어 부착 사실은 정적 덤프가 아니라 **행동 검증**(Phase E 제로그랜트 스캔, §2.6)으로 증명한다. 이 분업이 G-3의 두 조건을 각각 담당: `route-inventory.txt` diff = 라우트 목록 무드리프트, 스캔 = RequirePermission 부착률 100%. 라우트 목록→민감 분류 누락은 **T17 역방향 단언**(CR-3)이 닫는다.
- CI diff 형태: 커밋된 아티팩트 2종 vs 테스트 시점 재생성의 **byte 단위 골든 비교**(`-update` 플래그로 재생성, 통상 golden-file 패턴). 향후 route-coverage 워크플로는 이 테스트를 실행하기만 하면 된다(§8 인계).

### 2.2 아티팩트 형식 (M-1: 결정적 생성 강제)
- `docs/security/route-inventory.txt` — 헤더 주석 3행(생성 방법·재생성 명령·총 라인 수) + 라인. 
- `docs/security/sensitive-routes.txt` — opdef 테이블에서 생성: `{METHOD} {path}\tpermission={p}\tmutating={bool}\trisk={low|medium|high}\treduction={bool}` (RequireAny/CreateEdit는 `permission={a|b}` / `permission={create|edit}`). 이 파일이 §24의 `SENSITIVE` 집합 정의 그 자체.
- **결정성 계약(M-1)**: 두 아티팩트의 생성 함수는 포맷된 라인 슬라이스를 **정렬 후 join**한다(`sort.Strings` 또는 `slices.Sort` — 구현 표준 라이브러리로 고정). opdef 테이블의 선언 순서가 바뀌어도 아티팩트 byte 불변. 생성기는 순수 함수(라인 조립→정렬→join)로만 구성하고 라인 내 삽입 순서 의존을 금지한다 — 테스트 T13이 선언 순서 셔플 후에도 동일 byte 출력을 단언.

### 2.3 민감 분류 규칙 구현 (step 2)
- 규칙(스펙 verbatim): **non-GET 전부** + **GET 중 시크릿 소유 응답**(자격증명 판독·kubeconfig·AI tool execute/confirm·계정/비밀번호 관리).
- 실체화: non-GET은 opdef 테이블에서 `Mutating: true`로 기계 판정(테이블 검증 T4: Method ≠ GET ⇔ Mutating). GET 민감은 §0 표의 후보군을 큐레이터(구현자)가 컨트롤러 응답 매핑을 grep 검증해 확정 → 테이블 반영 → 아티팩트로 커밋. **과잉 편입은 안전**(원샷 그랜트가 기존 역할 전부 허용, 거부 없음), **과소 편입이 게이트 위반** — 애매하면 편입.
- 계정/비밀번호 관리 해석: `/admin/list`·`/admin/info`(GET)는 계정 자료 회신이므로 편입. `/admin/updatePersonal*` 등 자기 관리는 non-GET이라 규칙상 자동 편입.
- **레드액션 플래그(CR-2)**: 시크릿 무마스킹 회신이 실측된 라우트(asset host 2종·monitor datasource 3종·notify channel 3종·k8s cluster·credential 3종 등)의 Def는 `Redaction`에 대상 필드명(예: `credential.password`, `datasource.token`)을 기록하고 아티팩트 `reduction=true`로 표시한다. **레드액션 구현 자체는 T3 범위 밖** — 이 메타데이터가 후속 S1(DTO 스캐너)·V2 §10 정책 입력의 원천이 된다. r1의 "Redaction 전부 빈 슬라이스"는 폐기.

### 2.4 권한 문자열 할당 — 소유 위치와 규칙 (step 3)
- **소유 위치: `backend/opdef` 패키지 테이블이 단일 원천**. router.go 인라인 raw 문자열은 폐지(기존 domains 36곳도 상수로 이관하되 **문자열 값은 byte 동일** — 그랜트 그대로 이어짐). 이유: ① sensitive-routes.txt 생성 원천 ② 리플레이 행렬 원천 ③ §4.7 step 3가 요구하는 operation definition(문자열·mutating·risk·redaction)의 물리적 소재 ④ V2 §10 정책 입력이 이 테이블에서 파생(스펙 §965 `RequiredPermission`). **domains 36 문자열의 회귀 기준점은 opdef 내부 스냅샷 테스트(T4)가 단일 원천으로 보유한다(H-2 — router.go에는 이관 후 문자열이 0개가 되므로 router diff 기반 검증은 불가).**
- 할당 규칙(우선순위): ① **기존 시드 어휘 정확 매칭**(`ops:script:add`에 `POST /ops/script/add` 등 — add/edit/delete/status/run/test/execute 대응) ② `/save` 공용 엔드포인트는 `RequireCreateOrEditPermission` 쌍 (기존 domains 선례) ③ 공용 판독 GET은 `AnyOf` (기존 `domains:account:options` 선례) ④ 갭만 신규 — `domain:resource:action` 관례, 도메인은 **URL 1차 세그먼트가 아닌 기존 어휘의 도메인을 따름**(`/dbms/*`→`assets:database:*` 재사용, `/k8s/*`→`assets:k8s:*` 재사용).
- 신규 문자열 집중지(기존 어휘 전무): `integration:navigation:*`(7), `integration:ai:*`(18 — execute/confirm 포함, 스펙 §3.2 row 16 명시), `integration:finops:*`(15), `monitor:query|logs|traces` 계열, `system:config:upload` 등 — 추정 60–90개 신규값.
- 구조:
```go
type Def struct {
    Method, Path string
    Permission string   // 단일 (RequiredPermission)
    AnyOf      []string // 다중 허용(공용 판독)
    CreateEdit [2]string // {create, edit} — 공용 save
    Mutating bool
    Risk      string   // low|medium|high
    Redaction []string // 시크릿 응답 필드명 — CR-2 실측 라우트는 필수 기입(§2.3)
}
```
검증(T4): 셋 중 정확히 1개 지정·Method≠GET⇔Mutating·권한 문자열 정규식 `^[a-z][a-z0-9]*(?::[a-z][a-z0-9]*){1,3}$`·(Method,Path) 유일.

### 2.5 시드 마이그레이션 — 상시/원샷 분리 (step 4, r2 전면 재서술)
- **r1 결함(CR-1)**: 역할 그랜트를 상시 스텝으로 두면 (a) 신규 역할이 다음 재부팅에 전그랜트 자동 취득, (b) 운영자 권한 박탈이 재부팅마다 전량 복원 — §4.7-4 "tightening is an operator task after Phase -1" 경로를 영구 봉쇄한다. **분리로 해소한다.**
- 구현: `store/seed.go` Seed 슬라이스 말미(`seedSuperRolePermissions` 뒤)에 스텝 3개:
  1. **`seedRoutePermissionMenus` (상시)** — opdef 등장 권한 문자열 전수에 대해 `ensureMenu`(idempotent)로 type-3 row 확보. 부모: 기존 페이지 메뉴 값이 있으면 그 페이지 밑(기존 버튼 관례 동일), 없는 경우(integration/finops 등)는 **신규 은닉 루트** `{Value: "route-permissions", MenuType: 1, MenuStatus: 0, URL: "", ParentID: 0}` 밑. 근거: `CurrentMenus`가 `menu_status=1 AND menu_type IN (1,2)`만 내보내므로(실측 §0) status-0 루트와 type-3 잎은 사이드바 무노출, 권한 쿼리는 잎의 `menu_status=1`만 본다.
  2. **`seedSuperAdminRoutePermissions` (상시)** — super-admin 역할에 opdef 메뉴 전수를 **매 부팅 재그랜트**(존재 확인 루프 패턴 — `seedSuperRolePermissions` 관례 승계). 이유: 현행 시스템의 super-admin 불변식("매 부팅 전 메뉴 재그랜트")을 그대로 유지해야, 미래에 opdef에 정의가 추가되어도 super-admin이 새 권한을 즉시 취득한다(원샷만 있으면 마커 이후 신규 메뉴가 super-admin에게 못 온다). 기존 `seedSuperRolePermissions`가 우리 스텝보다 앞서 돌고 메뉴는 그 뒤에 생기므로 이 재그랜트가 필요하다.
  3. **`migrateRoleRoutePermissionsOnce` (원샷·마커 게이트)** — 게이트: **sys_menu 마커 row** `value = "route-permissions:granted:v1"` (type 3, status 1, 부모 = 은닉 루트). row가 없으면: *현존 전 역할*(super-admin 포함)에 opdef 메뉴 전수 그랜트 + 마커 row 생성. row가 있으면: **완전 no-op**. 결과: ① 기존 역할 록아웃 0(사전=사후) ② 이후 생성되는 역할은 어떤 그랜트도 자동 취득하지 않음 ③ 운영자의 박탈·조정이 재부팅에 생존.
- **마커 저장소 선정 근거**: `SystemConfig`는 고정 컬럼 단일 row(실측 §0)라 마커 필드 추가는 스키마 변경(보존 제약 위반). OperationLog 기반 마커는 `sysOperationLog/clean` 라우트가 지울 수 있어 기각. sys_menu 마커는 스키마 제로·idempotent 확인 1쿼리. 잔여 위험(메뉴 관리 UI에서 마커 row 수동 삭제 시 재부팅 재그랜트)은 A8에 기록.
- **super-admin tightening 불가는 현행 유지**: 오늘의 `seedSuperRolePermissions`도 매 부팅 재그랜트하므로 super-admin 권한 축소는 원래 불가능했다 — T3는 이 의미론을 바꾸지 않는다(일반 역할만 tightening 가능해진다).
- 슈퍼어드민 바이패스: **없음을 실측 확정**(§0) — 별도 바이패스 코드를 추가하지도 않는다.

### 2.6 리플레이 검증 (step 5, r2: H-3 역할 순서 계약·M-2·M-3 반영)
- 구조(`authz_replay_test.go`, glebarez sqlite — **`SetMaxOpenConns(1)` 단일커넥션(T1 `newRoundtripDB` 패턴 승계, M-2)** + `store.AutoMigrate` + `store.Seed` + `router.New`):
  - 헬퍼: 토큰 = `auth.GenerateToken` + `AuthSession` row(미래 만료·LastActivityAt=now). 엔진 구동 config는 `App.Mode` 최소 구성.
  - **역할 생성 순서 계약(H-3) — 순서가 곧 시맨틱스다**:
    1. `store.Seed` 완료 → 이 시점에 마이그레이션은 super-admin(유일한 시드 역할)에 대해 이미 실행됨.
    2. **R_replay(일반 역할, 전그랜트)** = Seed 완료 **후** 생성 + **super-admin의 role_menu row를 명시적 절차로 전량 복사**(grants simulation of pre-migration role). 원샷 그랜트는 이미 지났으므로 자동 그랜트는 오지 않는다 — 리플레이의 "사전에 전부 도달 가능했던 역할" 재현.
    3. **R_zero(제로그랜트)** = Seed 완료 후 생성, 그랜트 없음 — T15 커버리지 스캔용.
  - **T14 리플레이(사전=사후)**: R_replay × 민감 라우트 전수 요청 → `403 "Permission denied"` 아님을 단언(400/404/500는 허용 — 검증 실패 응답도 "허용됨"의 증거). deny 1건 = 마이그레이션 버그(스펙 문장 그대로).
  - **T15 제로그랜트 스캔(S3 부착률 오라클)**: R_zero 토큰으로 authGroup **425 전수** 요청 → 민감 = 403 Permission denied **정확히** 일치, 비민감 = 해당 마커 부재. 이것이 "민감 라우트 전수 RequirePermission 부착 grep"의 행동 버전(양방향: 부착 누락·과잉 모두 탐지).
  - **T16 미인증**: 민감 샘플 + 비민감 샘플 무토큰 → 401.
  - **T17 아티팩트↔라우터 정합 + 역방향 단언(CR-3)**: ① opdef의 (Method,Path) 전수가 `engine.Routes()`에 존재 ② sensitive-routes.txt 라인 수 = opdef 길이 ③ **역방향: authGroup 비-GET 전수 ⊆ opdef 테이블** — 새 POST/PUT/DELETE 라우트가 정의 없이 추가되면 즉시 실패. 라우트의 authGroup 소속은 Routes() 경로(`/api/v1/*`)에서 public 7종 경로 집합을 차감해 산출(public 집합은 테스트 내 상수로 고정·주석으로 근거 명시).
- 페이로드 전략: POST/PUT은 `{}` 바디, GET은 쿼리 무첨부 — 핸들러가 파라미터 검증에서 조기 4xx로 끝나야 함. 외부 네트워크를 검증 앞에 시도하는 핸들러가 있으면 해당 라우트만 예외 페이로드 테이블로 지정(위험 R3).
- **CreateEdit edit 분기 제외 사유(M-3 기록)**: 리플레이 `{}` 바디는 id=0 → create 분기만 통과한다. edit 분기(id>0)는 유효한 대상 행 존재가 전제라 전 라우트 재현이 불균형하게 비싸고, 분기 **선택 로직 자체는 T5가 미들웨어 단위에서 직접 검증**하며 두 권한 문자열 모두 R_replay가 보유하므로 edit 분기 실패 경로가 리플레이 결과를 바꿀 여지가 없다. `domains:account`·`domains:internal:zone` 2개 공용 save 엔드포인트에 한한다.

---

## 3. 위험 지점과 검증 방법

| # | 위험 | 완화·검증 |
|---|---|---|
| R1 | **240줄+ router.go 대편집에서 경로·핸들러 오타** | 아티팩트 diff가 1차 검지(경로 변화 = route-inventory.txt 불일치). 핸들러 무변경은 diff 리뷰 + `go vet`. 편집 전략: 도메인 구간별 순차 치환(`authGroup.X(path, ctl.H)` → `authGroup.X(path, opdef.Middleware(db, opdef.Y), ctl.H)`), 기존 domains 36줄은 문자열 동일 상수 치환 |
| R2 | **AutoMigrate 90모델의 sqlite 호환성** (T1은 선택 마이그레이션만 함) | Phase E 착수 첫 걸음으로 `store.AutoMigrate(sqlite)` 단돔 테스트. 실패 시 폴백: 리플레이에 필요한 모델만 명시 목록(AuthSession·Admin·Role·Menu·RoleMenu·AdminRole·OperationLog·SystemConfig + 부수). 어느 쪽인지 구현 보고서에 기록 |
| R3 | **매트릭스 중 외부 호출 핸들러**(ldap test·gateway test·ai model test 등) | `{}` 바디 → 바인딩/검증 4xx 선행 가정. 시간 초과 라우트 발견 시 예외 페이로드 테이블 + 필요하면 해당 서비스 생성자 주입 지점 확인. CI 타임아웃(10m) 모니터 |
| R4 | **원샷 그랜트의 실패 양상** (r2 재서술): (a) 마커 없이 부팅 중단 → 그랜트 미실행 채로 라우터만 부착되면 록아웃 (b) 마커 오동작(중복 실행/미실행) (c) super-admin 재그랜트 누락으로 미래 정의 미도달 | (a) Phase C→D 순서 + 동일 병합(단일 태스크 전제) — 운영 안전망으로 마이그레이션 스텝은 그랜트 전 에러 시 Seed 실패(부팅 중단)시켜 반쪽 상태를 만들지 않는다(트랜잭션 또는 스텝 순서로 보장, T6–T8이 검증). (b) T8(idempotency)·T9(신규 역할 미자동그랜트)·T10(박탈 생존)이 데이터 단면 단언. (c) T11(super-admin 상시 재그랜트 — 신규 메뉴 row 주입 후 Seed 재실행으로 단언) |
| R5 | **시드가 프론트에 미치는 부수 효과** — 기존 역할의 전 그랜트로 v-permission 버튼 일부 **새로 표시**됨(숨김→표시 방향만, 기능은 변화 없음: 이전에도 API 직접 호출은 가능했음) | §4.7 step 4의 의도된 결과(봉쇄+제로 록아웃)로 기록. 사이드바 무변화는 CurrentMenus 필터 실측으로 보장(§2.5). 스모크 claim 13 |
| R6 | **동시 배포 순서** — 라우터 부착(Phase D)이 먼저 배포되고 시드(Phase C)가 안 돌면 록아웃 | 단일 태스크 내 단일 브랜치 병합이 전제(스펙 Phase -1 일괄). 그럼에도 안전망: C가 D보다 앞선 Phase 순서 + 같은 PR 반영 |
| R7 | **OperationLog 미들웨어가 425 라우트 × 매트릭스에서 대량 insert** | sqlite in-memory + 단일커넥션(M-2)으로 무경쟁. CI 시간 증가 시 T15를 샘플링이 아닌 전수 유지(게이트의 실체이므로)하되 `-short` 스킵은 금지 |
| R8 | **기존 domains 36 라우트 거동 회귀** (문자열 이관 실수) | T14/T15가 동일 커버(그들도 민감 목록에 포함). 문자열 보존의 단일 원천은 **opdef 스냅샷 테스트 T4**(H-2 — router.go에는 문자열이 남지 않으므로) |
| R9 | **아티팩트 비결정성** — 선언 순서·맵 반복으로 diff 폭발 | M-1 결정성 계약(정렬 후 join·순수 생성기) + T13 셔플 불변 단언 |

**롤백 가능성 판정**: Phase A/B(신규 패키지)·C(시드·마커 추가)·D·E 전부 코드 revert로 거동 완전 복원(auth-only 상태). **DB 잔존물**(신규 menu row·role_menu 그랜트·마커 row)은 revert 후에도 남으나 미참조라 무해 — 수동 정리 불필요, 명시적으로 비자동화. 배포 후 락아웃 발생 시 되돌리기보다 **전진 수정**(마이그레이션 재실행은 마커 삭제 후 부팅 한 번으로 재현 가능 — A8) 원칙 — 스펙이 마이그레이션 버그로 규정하는 영역.

---

## 4. 테스트 계약 (TDD — 작성 순서 = 구현 순서)

Phase A — `backend/opdef/opdef_test.go`:
- **T1** `All()` 비었음 단언 금지(≥255) + (Method,Path) 유일.
- **T2** 권한 지정 정확히 1개(Permission/AnyOf/CreateEdit 상호배타) + 빈 권한 거부.
- **T3** 권한 문자열 정규식 전수 준수 + CR-2 실측 라우트(asset host 2·monitor datasource 3·notify channel 3·k8s cluster·credential 3)의 `Redaction` 비어있지 않음 단언.
- **T4** Method≠GET ⇔ Mutating=true + Risk enum + **domains 36개 기존 문자열·형태(단일/AnyOf/CreateEdit) 스냅샷 단언 — r1 시점 router.go 원문에서 추출한 목록을 테스트 코드에 상수로 보유(회귀 기준점의 단일 원천, H-2)**.
- **T5** `Middleware()`가 세 형태(단일/AnyOf/CreateEdit)에서 기대 미들웨어 반환(라벨 필드로 구분) + **CreateEdit의 id 유무 분기 직접 검증**(더미 gin 컨텍스트·바디 — §2.6 제외 사유의 보상).

Phase C — `backend/store/seed_route_permissions_test.go` (sqlite, 단일커넥션):
- **T6** Seed 후 opdef 전 권한 문자열이 sys_menu에 존재 + type-3 + status 1 + 고아 권한은 은닉 루트 밑.
- **T7** 첫 Seed: 현존 전 역할(super-admin)이 opdef 메뉴 전수 그랜트 보유 + **마커 row 생성**.
- **T8** Seed 2회 실행 = row·그랜트 수 불변(idempotency — 마커 중복 생성 없음).
- **T9 (CR-1a)** Seed 완료 후 신규 역할 생성 → opdef 그랜트 **0건** (자동 그랜트 없음).
- **T10 (CR-1b)** 기존 역할에서 그랜트 1개 삭제 후 Seed 재실행 → **미복원** (박탈 생존).
- **T11** 테스트 내 opdef 메뉴 1개 추가 삽입 → Seed 재실행 → super-admin만 신규 보유(상시 재그랜트), 일반 역할은 미보유.

Phase D/E — `backend/router/routes_inventory_test.go`·`authz_replay_test.go`:
- **T12** 재생성 route-inventory.txt ≡ 커밋본 (byte diff) — 본문 435라인 — `-update`로 재생성.
- **T13** 재생성 sensitive-routes.txt ≡ 커밋본 + **opdef 선언 순서 셔플 후에도 동일 byte**(M-1 결정성).
- **T14–T17** §2.6 명세 그대로 (리플레이 / 제로그랜트 스캔 / 미인증 / 정합+역방향).

기존 테스트는 전부 무수정 통과해야 한다(회귀 0의 실측 기준점). 커버리지: 신규 패키지 opdef ≥80%(TDD 규칙), router 테스트 파일은 엔진 수명 주기 헬퍼 제외 전 경로 커버 매트릭스로 갈음(수치 기재).

---

## 5. 보존 제약 (③구현 프롬프트에 verbatim 복사)

1. 기존 30 `RequirePermission` + 4 `RequireAnyPermission` + 2 `RequireCreateOrEditPermission` 호출(domains 구간)의 **권한 문자열과 거부 거동은 byte 동일하게 유지**한다 — 상수 이관만 허용, 값 변경·순서 변경 금지.
2. 라우트의 **메서드·경로·핸들러·등록 순서를 변경하지 않는다** — 변경은 미들웨어 인자 삽입뿐. public 그룹 7개 라우트와 WS 2개는 건드리지 않는다.
3. `middleware/permission.go`의 세 함수는 시그니처·본체 무수정 — opdef는 이들을 **호출**만 한다.
4. 프론트엔드(`web/`)는 어떤 파일도 수정하지 않는다 — 사이드바·v-permission은 기존 데이터 계약(CurrentMenus 필터)으로 무변경임을 코드로 보장한다.
5. 기존 테스트 파일은 무수정 통과해야 한다(회귀 0). 실패 시 구현 수정이 우선.
6. `glebarez/sqlite`는 `_test.go`에서만 import한다(T1 보존제약 승계).
7. 기존 Seed 9스텝의 순서·내용을 변경하지 않고 말미에 3스텝만 추가한다. 기존 메뉴 row의 부모·이름·status를 변경하지 않는다.
8. **스키마 변경(모델 컬럼 추가 포함)·권한 박탈 코드·레드액션 구현을 포함하지 않는다.** 허용되는 데이터 쓰기는 신규 메뉴 row(type-3·은닉 루트)·원샷 그랜트·마커 row 3종뿐이다. 권한 축소(tightening)는 운영자 후속 과제다(§4.7 step 4).
9. 콘솔 WS 티켓·mutating-GET 전환·CI 워크플로 파일은 범위 밖이다 — router.go에서 WS 라우트 라인은 무편집.
10. 원샷 마이그레이션(`migrateRoleRoutePermissionsOnce`)은 마커 존재 시 **어떤 그랜트도 쓰지 않는다** — 상시 재그랜트는 super-admin 스텝에만 존재한다.

---

## 6. 검증 요구 (claims — ④리뷰 주입용)

1. `cd backend && go build ./... && go vet ./...` exit 0.
2. `cd backend && go test ./...` exit 0 — 기존 테스트 회귀 0 포함.
3. `ls backend/opdef/` — go 파일 8종 존재 + `grep -c "Def{" backend/opdef/defs_*.go` 합산 ≥ 255 (민감 정의 전수, 수치 기재).
4. `grep -n "seedRoutePermissionMenus\|seedSuperAdminRoutePermissions\|migrateRoleRoutePermissionsOnce" backend/store/seed.go` 3스텝 존재 + Seed 슬라이스 말미 등록 확인.
5. `ls docs/security/sensitive-routes.txt docs/security/route-inventory.txt` 존재 + `wc -l docs/security/route-inventory.txt` = **헤더 3 + 본문 435 = 438** (산정 근거 M-3/H-1: /ping 1 + public 7 + authGroup 425 + uploads GET·HEAD 2, gin v1.11.0 Static).
6. `cd backend && go test ./router/ -run "TestRouteInventoryArtifact"` 통과 — 재생성 diff 0 (커밋본과 byte 일치).
7. `go test ./router/ -run "TestSensitiveRoutesArtifact"` 통과 — sensitive-routes.txt ≡ opdef 테이블 + 셔플 불변(T13).
8. **(H-2 재설계)** domains 36 문자열 무변경의 단일 원천 = opdef 스냅샷: `go test ./opdef/ -run "TestDomainDefsSnapshot"` 통과 + `grep -c 'Permission:\|AnyOf:\|CreateEdit:' backend/opdef/defs_domain.go` = 36 (r1 시점 router.go 라인 52–87 원문 대응, 테스트 코드 내 상수와 일치).
9. `go test ./router/ -run "TestPermissionReplay"` 통과 — R_replay × 민감 라우트 전수 allow(403 Permission denied 0건, 수치 기재: 역할 수 × 라우트 수).
10. `go test ./router/ -run "TestZeroGrantCoverage"` 통과 — R_zero가 민감 전수 403·비민감 authGroup 라우트 403-퍼미션 0건 (S3 부착률 100%의 행동 증명, 수치 기재).
11. `go test ./router/ -run "TestOperationTableCoversRouter"` 통과 — opdef ⊆ Routes() + **authGroup 비-GET 전수 ⊆ opdef** (CR-3 역방향, 수치 기재).
12. `go test ./store/ -run "TestRoutePermissionSeed"` 통과 — T6–T11 포함: **T9 신규 역할 자동 그랜트 0건·T10 박탈 후 재 Seed 미복원** (CR-1 회귀, 결과 수치 기재) + `grep -n 'route-permissions:granted' backend/store/seed.go` 마커 상수 존재.
13. 스모크: sqlite 시드 DB에서 `POST /api/v1/login` + `GET /api/v1/profile` 200 (테스트 내 주장으로 대체 가능) — 기존 로그인 동작 무영향.
14. `grep -rn "glebarez" backend --include="*.go" | grep -v _test.go` 매치 0.
15. `git diff --stat -- web/` 출력 없음 (프론트 무변경).

---

## 7. 가정 명세 (불확실 요소의 명시적 처리)

- **A1**: 원샷 그랜트 대상 "모든 기존 역할" = 마커 미존재 시점의 sys_role 전 행. 운영 1인 도구라 super-admin 1개가 사실상 전부이나 코드는 역할 수 무관하게 작성.
- **A2**: 비민감 GET에도 403-퍼미션 유사 응답을 내는 기존 경로는 없다(실측: 권한 미들웨어는 domains 36곳뿐). T15의 부정 마커가 오탐 없음의 근거.
- **A3**: `{}` 바디·무쿼리 요청이 외부 호출 없이 4xx로 끝난다 — T15 실행으로 검증, 위반 라우트는 예외 페이로드 테이블로 명시적 처리(R3).
- **A4**: 리플레이 "사전" 기준선은 코드 실측(authGroup = Auth+OperationLog만)으로 정의 — 스냅샷 캡처를 별도로 돌리지 않는다. git history가 사전 상태의 증거.
- **A5**: 신규 은닉 루트 `route-permissions`(status 0)와 **마커 row**가 권한 관리 UI(Menu.vue 트리)에 표시되어도 수용 — 사이드바에는 CurrentMenus 필터로 미노출(실측 근거 §2.5). role-assign 트리에서 신규 권한이 보이는 것은 의도(운영자 tightening 입력).
- **A6**: 민감 GET 큐레이션(§0 표)은 구현 중 컨트롤러 응답 재검으로 ±소수 조정될 수 있다 — 조정은 테이블+아티팩트에 반영되어 게이트가 추적. 과잉 편입 허용·과소 편입 금지 원칙은 고정.
- **A7**: Phase E에서 `store.AutoMigrate` 전체가 sqlite에 성공한다(R2 폴백 명시).
- **A8 (r2)**: sys_menu 마커 row가 메뉴 관리 UI에서 수동 삭제되면 다음 부팅에 원샷 그랜트가 재실행된다 — 이는 롤백·재수행의 의도된 뒷문(§3 롤백 판정)이자 운영자가 건드리지 않는 한 비활성(은닉 루트 밑 type-3). 문서화로 대응, 코드 방어는不加.
- **A9 (r2, M-3)**: SENSITIVE = authGroup 한정 해석(§0). public 비-GET 3종(로그인 계열)은 권한 요구가 성립하지 않는 인증 전 단계, uploads GET/HEAD 2종은 정적 파일 서빙으로 NORMAL 정의 — 인벤토리에는 전수 수록하되 분류에서 제외. 향후 해석을 바꾸면 T17 역방향 단언의 public 집합 상수가 추적 지점이다.
- **A10 (r2)**: CreateEdit edit 분기(id>0)는 리플레이 매트릭스에서 제외 — 사유와 보상 검증(T5 분기 직접 검증)은 §2.6.

---

## 8. 산출 외 확인사항 (구현자에게 불요, 팀 리드 인계)

- **CI 워크플로 `route-coverage`(§4.10)**: T12/T13/T17 골든 diff·역방향 테스트가 이미 게이트 실체 — 워크플로 파일과 카나리(미등록 라우트 심기 → 게이트 실패 단언)는 CI-베이스라인 태스크가 `go test ./router/ -run "TestRouteInventoryArtifact|TestSensitiveRoutesArtifact|TestOperationTableCoversRouter"`를 감싸기만 하면 성립.
- **운영자 tightening 가이드 (r2 갱신)**: 이제 권한 박탈은 **재부팅에 생존**한다. 신규 역할은 라우트 권한 0으로 시작하므로 생성 시 role-assign UI/API로 할당. super-admin은 상시 전체 재그랜트(현행 의미론 유지) — super-admin 축소가 필요하면 별도 태스크. 마커 row(`route-permissions:granted:v1`) 삭제 + 재부팅 = 원샷 그랜트 재수행의 공식 재현 절차.
- **레드액션 후속 (CR-2)**: opdef `Redaction` 메타데이터가 S1(DTO 스캐너)·응답 마스킹 구현의 입력 — 별도 태스크. T3는 기록까지만.
- **AI tool execute/confirm**: 이 태스크에서 권한+부착까지만 — 감사 강화·operation definition 상세화는 스펙 §18.4 후속 ADR.
- **Phase -1 게이트 매핑**: G-3 ← claim 5/6/7/10/11, G-4 ← claim 9, S3 ← claim 10/11, 록아웃 0 ← claim 9/12.

## 9. ④리뷰 인계 (r2.1 — rev-t3-authz 승인 조건 문서화)

- **N1 운영 가이드**: 마이그레이션(원샷) 이후 생성되는 역할은 기본 그랜트가 없다. 신규 역할 생성 시 최소 `system:admin:personal`(자기 비밀번호·프로필 변경, Profile.vue:61,92 사용)을 포함할 것 — 자기 비밀번호 로테이션을 막는 권한 배치는 보안상 부정적 인센티브.
- **N2 방어 후속**: ListMenus가 은닉 루트(`route-permissions`, status 0)·마커·type-3 권한 row를 무필터 반환 — 메뉴 관리 UI에서 삭제 가능하다. DeleteMenu의 cascade가 자식 role_menu를 지우지 않아 루트/마커 삭제 시 다음 부팅 전 역할 재그랜트+고아 row가 발생한다(방향은 안전·록아웃 없음). 후속 방어: ListMenus에서 status 0·마커 row 필터링 또는 삭제 보호 — 별도 소과제로 등록.
- **메트릭 정정**: 신규 권한 문자열 172는 "라우트 부착 기준"(199−27)이며, 시드 어휘 외 진짜 신규는 53(예상 60-90 내).
- **N3 기록**: 원샷 그랜트로 기존 제한 역할 사이드바에 type-2 페이지 최대 20개 신규 노출(일회성).
