# V2 Phase -1 · Task 4 계획 — 콘솔 원타임 티켓 Release A(§4.8) + mutating GET 410 전환(§4.9)

- 원천: `/tmp/v2-rev/docs/architecture/multi-infrastructure-control-plane-v2.md` **§4.8(Console and terminal containment)·§4.9(Mutation semantics)** — Release A(백엔드 이중수용)만. Release B(**WS 프론트 전환**)·C(legacy 제거)는 후속 태스크.
- 범위: ①`POST /api/v1/console-sessions` 티켓 발급 엔드포인트 ②기존 WS 2종의 ticket 파라미터 우선 이중수용 + legacy 경로 폐기 헤더 ③`GET /asset/service/diagnosis/run` → 410 Gone + POST 대체 라우트 동시 제공 ④CHANGELOG 항목 ⑤프론트 diagnosis/run 호출 **1줄** POST 전환(동일 릴리스, 판정 Q3). §4.9의 Idempotency-Key(V2 전용)·OperationLog 확장(row-2)은 **제외** — v1은 "keeps current behavior" 원문.
- 실측 기준: main `7705845` + 작업 트리(T3 완료 상태, opdef 283정의·아티팩트 2종 커밋됨). 본 세션 전 수치는 코드 재확인 완료.
- 개정 이력: r1 → r2 (team-lead 조건부 승인 4건 반영 — Q3 프론트 동반 1줄 전환 포함, Q2 Release B/C 후속 등록 계약·WS 공격면 정직 기록, Q1 resourceId canonical 계약, F-1/F-2 아티팩트 diff 예측 정정. r1의 파일 수 17은 table 실제 18의 오계산 — r2에서 19로 정정).

---

## 0. 현재 코드 실측 (계획의 사실 기반)

### WS 2종 — public 그룹, 토큰만 검사, 권한 검사 전무
- `router/router.go:45-46` — `api.GET("/asset/terminal/ws")`·`api.GET("/k8s/pod/terminal/ws")`는 **public api 그룹**(Auth 미들웨어 밖). §3.3 지적대로 URL에 메인 토큰 노출.
- `controller/controller.go:981-993` `AssetTerminalWS`: `c.Query("token")` → `auth.ParseToken`(서명·클레임만 검사, 세션 DB 미조회) → `hostId`·`rows`·`cols` 파라미터 → `service.OpenAssetTerminal` **호출 후** `terminalUpgrader.Upgrade(c.Writer, c.Request, nil)`(controller.go:1002).
- `controller/k8s.go:261-272` `K8sPodTerminalWS`: 동일 토큰 게이트 → query 바인딩(clusterId·namespace·podName·container·command·rows·cols) → **업그레이드 먼저**(k8s.go:291) → `service.OpenK8sPodTerminal`(실패 시 에러 프레임 전송).
- **핵심 실측 — gorilla/websocket `Upgrade`는 101을 raw byte로 직접 기록**한다(핀 버전 `v1.5.4-0.20250319132907` server.go: `HTTP/1.1 101 Switching Protocols\r\n...` 고정 헤더 + **세 번째 인자 `responseHeader`만 merge**). `c.Writer`에 미리 세운 헤더는 소실된다. → 폐기 헤더는 **반드시 `Upgrade(..., headers)` 인자로** 전달해야 한다(구현 계약 C-3).
- `terminalUpgrader`(controller.go:31)는 `CheckOrigin: true` — 테스트에서 Origin 무관 다이얼 가능.
- **정직 기록(Q2)**: Release A 단독 배포의 WS 공격면 개선은 **0** — 프론트가 legacy query-token 경로를 계속 사용하므로 URL 토큰 노출(§3.3)·무권한 접속이 그대로다. 티켓 경로·폐기 헤더는 Release B(프론트 전환)까지 도달하지 못하는 준비물이며, 실질 개선은 B+C 완료 시점에 발생한다(§9 등록 계약 참조).

### 터미널 권한 어휘 — 이미 시드에 존재 (신규 문자열 불필요)
- `store/seed.go` 시드 어휘에 **`assets:terminal`(페이지)·`assets:host:terminal`·`assets:k8s:pod:terminal`(버튼, seed.go:370·380 페어 표)** 가 이미 존재 — 메뉴 row 생성 완료 상태.
- `grantRoutePermissionMenus`(seed.go)는 `value IN opdef.PermissionStrings() AND menu_status=1` 만 그랜트: T4가 이 문자열들을 opdef에 추가하면 **super-admin은 매 부팅 재그랜트**(`seedSuperAdminRoutePermissions`), 신규 설치 DB는 one-shot 전체 그랜트, **기존 배포 DB의 비super 역할은 미그랜트**(마커 `route-permissions:granted:v1` 소진) — T3 §2.5 정책 승계("tightening is an operator task"). Release A에서 프론트가 legacy 경로 쓰므로 사용자 영향 0(가정 A-7).
- `middleware/permission.go` — `RequireAnyPermission`가 권한 쿼리(`sys_admin_role→sys_role_menu→sys_menu`, `c.GetUint("userID")`)의 유일 구현. 핸들러 내 자원별 재검증에 재사용 가능한 형태로 **함수 헬퍼가 없음** → M-1에서 추출.

### mutating GET — 유일 실측 1건
- `router/router.go:207` `GET /asset/service/diagnosis/run` → `RunAssetServiceDiagnosis`(controller/asset_service.go:38) — `ShouldBindQuery`로 `service.AssetServiceDiagnosisTarget` 바인딩. 이 구조체는 **json·form 태그를 모두 보유**(service/asset_service_diagnosis.go:19-27) → POST JSON 바인딩에 그대로 재사용 가능, 서비스 로직 무변경.
- opdef 정의: `GET /asset/service/diagnosis/run, Permission: "assets:service:diagnosis", Risk: RiskLow`(defs_asset.go:68). **POST 대체 라우트가 같은 문자열을 재사용하면 그랜트 그대로 승계 — 제로 락아웃.**
- 프론트 사용처: **단 1곳** — `web/src/api/asset.js:18`(GET, params, timeout 240000) ← `web/src/views/assets/ServiceHealthDiagnosis.vue:97`. 본 태스크가 동일 릴리스에 `http.post`로 전환(D-4·Q3). 동일 파일의 arthas download 라인이 이미 `http.post(url, data, { timeout })` 시그니처 사용 중임을 실측 — 1줄 교환이 정확히 성립.

### 라우트·테스트 계약 (T3가 세운 하드 카운트 — 갱신 대상)
- 총 라우트 **435** = 1 ping + 7 public + **425 authGroup** + 2 uploads(routes_inventory_test.go:121-127 주석·단언).
- `authz_replay_test.go` `TestZeroGrantCoverage`: `len(authRoutes) != 425` 단언(~:205). `publicRouteKeys`에 WS 2종 명시("belong to the console-ticket task" 주석 — public 집합은 T4 후에도 불변).
- `TestPermissionReplay`(전그랜트 역할): `opdef.All()` 전수 리플레이, **403만 실패로 간주**(401은 fatal, 400/410은 관통) → 신규 2 라우트가 바인딩 실패 400으로 끝나면 자동 통과. `TestZeroGrantCoverage`(제로그랜트): opdef에 추가된 라우트는 deny 기대 — opdef 부착 시 자동 정합.
- 아티팩트 재생성: `cd backend && go test ./router/ -run 'TestRouteInventoryArtifact|TestSensitiveRoutesArtifact' -update`.
- WS 핸들러를 직접 때리는 기존 테스트는 **전무**(grep 실측) — 신규 테스트가 첫 커버.

### 티켓 저장 인프라
- `model.AuthSession`(sys_auth_session)이 사실상 동형 선례: string PK·토큰해시 uniqueIndex·만료 index. `store.AutoMigrate`(migrate.go, 모델 90개)에 1줄 추가로 스키마 반영 — 별도 마이그레이션 파일 체계 없음.

---

## 1. 설계 결정 (구현 계약)

### D-1 티켓 모델 — `sys_console_ticket`
```
ID string PK(32hex) · TicketHash string(64hex, uniqueIndex, not null)
ResourceType string(32) — "asset_host" | "k8s_pod"
ResourceID string(512)   — asset_host: "123" / k8s_pod: "3/default/nginx-abc" (cluster/namespace/pod)
Protocol string(32)      — "asset-terminal" | "k8s-pod-terminal" (엔드포인트별 고정)
UserID uint(index) · ExpiresAt time(index) · ConsumedAt *time(index) · CreatedAt
```
- 티켓 값: `crypto/rand` 32바이트 → base64 RawURL(43자). 저장은 sha256 hex만 — 원문 미저장.
- TTL **30초 고정**(§4.8 "≤30s"). 소비 성공 여부와 무관 **1회**(A-5): 소비는 서비스 호출 이전, 이후 실패해도 재사용 불가.
- **원자적 단일 사용**(§4.8 원문): `UPDATE sys_console_ticket SET consumed_at=? WHERE ticket_hash=? AND consumed_at IS NULL AND expires_at>?` — `RowsAffected==1`만 성공. 조회-수정 2-step 금지.
- 만료 청소: 발급 시 `DELETE WHERE expires_at < now-24h`(best-effort, 에러 무시) — 별도 크론 없음.

### D-2 발급 엔드포인트 — `POST /api/v1/console-sessions` (authGroup)
- 배치: `authGroup`(= Auth + OperationLog 자동 부착 → 발급이 감사로그에 기록됨, A-6) + opdef 미들웨어.
- opdef 정의(defs_system.go에 배치 — cross-domain 콘솔 엔드포인트):
  `{POST, /console-sessions, AnyOf: ["assets:host:terminal","assets:k8s:pod:terminal"], Mutating: true, Risk: RiskHigh}` — 어휘 100% 재사용(보존 제약 5).
- **M-1 자원별 재검증**: 엔드포인트 AnyOf는 느슨하므로(포드 권한만 있는 사용자가 호스트 티켓 발급 가능) 핸들러에서 resourceType에 맞는 개별 문자열을 재검사. `middleware/permission.go`에서 권한 쿼리를 `AdminHasPermission(db, adminID, perm) bool` 함수로 추출(미들웨어는 이를 호출하도록 리팩터 — 어플리케이션 유일의 인가 SQL 단일화) → service 경유 `svc.AdminHasPermission`으로 controller에서 사용.
- 요청/응답 스키마: `{resourceType, resourceId, protocol}` (protocol↔resourceType 짝 검증 서버 강제) → `{ticket, expiresAt, expiresIn: 30}`.
- **canonical 바인딩 키(Q1)**: resourceId의 정수 성분(hostId·clusterId)은 파싱 후 `strconv.FormatUint(uint64(parsed), 10)`로 정규화한 문자열로 저장·비교한다 — raw `"0123"`·`"-1"` 랩어라운드가 별도 자원처럼 매핑되거나 우연히 일치하는 것을 방지. 발급(요청 resourceId 검증)과 소비(WS query 파라미터)는 **동일한 정규화 함수**를 경유한다(D-3 바인딩 키와 한 몸).
- 검증 순서: 바인딩 → protocol↔resourceType 짝 → resourceId canonical화 → M-1 개별 권한 → 발급.

### D-3 WS 이중수용 — ticket 우선, legacy 폴백 + 폐기 헤더
`controller/console.go`에 공용 게이트 추가, 두 핸들러의 **토큰 검사 블록만 교체**:
```
authorizeTerminalWS(c, db경유 service, protocol, resourceType, resourceID) (http.Header, bool)
```
1. `ticket` 파라미터 **존재** → 소비 시도: 해시 조회 원자 소비 → (a) 무효/만료/이미 소비 → **401, legacy 폴백 없음**(A-1); (b) 유효하나 protocol·resource 불일치 → **403 거부**(티켓-자원 바인딩 강제, §4.8 "bind to one resource and protocol"); (c) 일치 → 통과, 반환 헤더 nil.
2. `ticket` **부재** → 기존 `auth.ParseToken` 경로 그대로(보존 제약 3) + 반환 헤더에 `Deprecation: true`·`Sunset: <상수>`.
3. 핸들러는 `Upgrade(c.Writer, c.Request, 반환된헤더)` — 세 번째 인자 전달(실측 R-1). nil이면 종전 동작.
- 바인딩 키: asset=**canonical 정수 문자열**(Q1 — 파싱 후 `strconv.FormatUint`, raw '0123'·'-1' 랩어라운드 매핑 방지), k8s=`"canonical cluster 정수/namespace/pod"`(container·command는 미바인딩 — pod가 인가 단위, A-2).
- 파라미터 파싱 순서 변경: 바인딩 키 계산을 위해 query 파싱이 게이트보다 선행(k8s은 query 바인딩→게이트, asset은 hostId 파싱→게이트). 유효 토큰+쓰레기 query 조합의 응답 코드는 종전과 동일(400) — 불일치 없음, 무효 토큰+쓰레기 query는 종전 401→400으로 바뀔 수 있음(기록됨, 무해).
- Sunset 상수: `"Thu, 31 Dec 2026 23:59:59 GMT"`(placeholder, Release B에서 확정 — A-3).

### D-4 mutating GET 전환 — 동일 릴리스 POST 제공 + GET 410
스펙 원문("for one release the converted endpoints answer GET with 410 plus a header naming the replacement verb/path")·team-lead 판정에 따라 **같은 릴리스에**:
- `POST /asset/service/diagnosis/run` 신규 등록 — opdef `Permission: "assets:service:diagnosis"` 재사용(그랜트 승계, RiskMedium으로 상향 — 실행 트리거의 정직한 분류). 핸들러는 동일 서비스 호출에 `ShouldBindJSON`만 교체(json 태그 보유 실측).
- `GET /asset/service/diagnosis/run` 라우트·opdef 정의는 **잔존**(아티팩트 라인 불변, zero-grant 정합 유지), 핸들러만 `410 Gone` 스텁으로 교체: `Allow: POST` + `Location: /api/v1/asset/service/diagnosis/run` 헤더 + 안내 메시지.
- `docs/CHANGELOG.md` 신규 — 항목 2건: ①diagnosis/run GET→POST 전환(**외부 스크립트·북마크** 안내, §4.8 명시 요구) ②`/console-sessions` 신설 + WS legacy 토큰 경로 폐기 예고.
- **프론트 동반 전환(Q3, team-lead 판정 수용)**: `web/src/api/asset.js` 단일 행을 동일 릴리스에 GET→POST로 전환한다: `queryAssetServiceDiagnosisRun = (params) => http.post('/api/v1/asset/service/diagnosis/run', params, { timeout: 240000 })`. 근거 기록: 스펙의 A/B 분리 전제("frontend and backend cannot flip atomically")는 본 저장소의 compose 단일 빌드·모노레포 구조와 불일치 — 모노레포에서 창 제로화가 가능하며, 회피 가능한 웹 UI 고장을 스펙 인용으로 정당화할 수 없다. **410 창은 외부 스크립트·북마크에만 적용되는 스펙 의지로 정정** — 웹 UI 고장 창 없음. Release B(WS 프론트 전환)는 여전히 별도 — 티켓 플로우 프론트 구현이 1줄이 아니기 때문.

### D-5 카운트 계약 갱신 (기존 테스트의 정당한 수정)
- 총 라우트 435→**437**, authGroup 425→**427**, opdef 283→**285**(defs 2개 추가). `routes_inventory_test.go`(단언+주석)·`authz_replay_test.go`(단언+`make` capacity) 수정 + 아티팩트 2종 `-update` 재생성.
- **아티팩트 diff 예측(F-1/F-2 정정)**: route-inventory 본문 라인은 `"%s %s -> %s"` 포맷(**핸들러명 포함**, routes_inventory_test.go:69 실측)이므로 — 본문 **+2 신규**(POST /console-sessions·POST /diagnosis/run) **+1 변경**(GET /diagnosis/run 라인의 핸들러명이 410 스텁명으로 교체) + 헤더 `# N entries.` 카운트 라인 435→437(헤더는 3라인 고정: 제목·재생성 안내·카운트). sensitive-routes.txt는 opdef 렌더라 POST 2라인 순수 추가 + `# 285 entries.` 카운트 라인 변경.

---

## 2. 파일 배치 · Phase 분해 (19파일 → 4 Phase, 각 ≤5파일)

| 파일 | 신규/수정 | 내용 |
|---|---|---|
| `backend/model/console_ticket.go` | 신규 | `ConsoleTicket` 모델(D-1), TableName `sys_console_ticket` |
| `backend/store/migrate.go` | 수정 | AutoMigrate 리스트에 `&model.ConsoleTicket{}` 1줄 |
| `backend/service/console_ticket.go` | 신규 | `MintConsoleTicket`·`ConsumeConsoleTicket`(원자 UPDATE)·`AdminHasPermission`(M-1 경유)·발급 시 만료 청소 |
| `backend/service/console_ticket_test.go` | 신규 | 단위: 발급↔소비 왕복·만료·재사용·파지(T1-T3), sqlite 단일커넥션 패턴(M-2 승계) |
| `backend/opdef/defs_system.go` | 수정 | `POST /console-sessions` 정의(AnyOf, D-2) |
| `backend/opdef/defs_asset.go` | 수정 | `POST /asset/service/diagnosis/run` 정의(D-4) |
| `backend/controller/console.go` | 신규 | `CreateConsoleSession` 핸들러 + `authorizeTerminalWS` 공용 게이트 + 바인딩 키 생성·폐기 헤더 상수 |
| `backend/controller/asset_service.go` | 수정 | `RunAssetServiceDiagnosis`→JSON 바인딩(POST용) + `AssetServiceDiagnosisRunGone` 410 스텁 |
| `backend/router/router.go` | 수정 | authGroup에 POST 2건 등록(opdef 부착), GET diagnosis/run 핸들러만 교체 — **기존 라인 무변경 원칙** |
| `backend/router/routes_inventory_test.go` | 수정 | 435→437 단언·주석 |
| `backend/router/authz_replay_test.go` | 수정 | 425→427 단언·capacity |
| `web/src/api/asset.js` | 수정 | :18 단일 행 GET→POST 전환(D-4·Q3 — params POJO를 그대로 JSON 바디로, timeout 보존) |
| `docs/security/route-inventory.txt` | 재생성 | 본문 437라인 + 핸들러명 변경 1라인 + 카운트 라인(D-5 예측) |
| `docs/security/sensitive-routes.txt` | 재생성 | +2라인(285) |
| `backend/middleware/permission.go` | 수정 | M-1: 인가 쿼리를 `AdminHasPermission(db, adminID, perm)`으로 추출, `RequireAnyPermission`은 이를 호출(동작 byte 동일) |
| `backend/controller/controller.go` | 수정 | `AssetTerminalWS` 토큰 블록→게이트 교체(~:981-993) + Upgrade 3번째 인자(:1002) — **스트림 로직 무변경** |
| `backend/controller/k8s.go` | 수정 | `K8sPodTerminalWS` 동일(~:261-272, :291) |
| `backend/router/console_ticket_test.go` | 신규 | 라우트 계약: 티켓 왕복·불일치·legacy 폴백+헤더·410(T4-T10) |
| `docs/CHANGELOG.md` | 신규 | D-4 항목 2건 |

**Phase (의존 순서, 각 독립 검증)**

- **Phase A — 티켓 모델·서비스 (4파일)**: model/console_ticket.go, migrate.go, service/console_ticket.go(+test). 의존: 없음. 검증: `cd backend && go build ./... && go test ./service/ -run TestConsoleTicket -v` 녹색 + `./store/` 마이그레이션 기존 테스트 통과.
- **Phase B — 정의·엔드포인트·라우트 (5파일)**: defs_system.go, defs_asset.go, controller/console.go, controller/asset_service.go, router.go. 의존: A(서비스 메서드). 검증: `go build ./...` + `go test ./opdef/...`(테이블 불변식이 신규 2정의 자동 검증). **이 시점 router 골든 테스트는 의도적 red** — C가 즉시 뒤따름(린저 없음).
- **Phase C — 프론트 동반 전환·계약 갱신·아티팩트 (5파일)**: web/src/api/asset.js, routes_inventory_test.go, authz_replay_test.go, 아티팩트 2종. 의존: B. **B와 C는 동일 릴리스에 연속 반영** — B가 GET 410을 켜고 C가 프론트를 POST로 전환(커밋 순서상 인접, 릴리스 외부로 고장 창 노출 없음. 커밋 단위 순서는 반드시 B→C). 검증: asset.js diff 1행 확인 + `go test ./router/ -run 'TestRouteInventoryArtifact|TestSensitiveRoutesArtifact' -update` 후 diff 검토(D-5 예측: +2 신규·+1 핸들러 변경·카운트 라인) → `-update` 없이 재실행 녹색 + `TestPermissionReplay`·`TestZeroGrantCoverage` 녹색.
- **Phase D — WS 게이트 이중수용 + E2E 테스트 + CHANGELOG (5파일)**: middleware/permission.go, controller/controller.go, controller/k8s.go, router/console_ticket_test.go, docs/CHANGELOG.md. 의존: A·B. 검증: `cd backend && go test ./... -race` **전 패키지 녹색**(기존 회귀 0 포함).

단계 순서 요약: A → B → C → D (C는 B 직후 강제, D는 A/B와 독립 병렬 가능하나 게이트 교체 대상 핸들러가 B 라우트와 무관하므로 순차 권장 — diff 리뷰 단순성).

---

## 3. 테스트 계약

| ID | 테스트 | 검증 지점 |
|---|---|---|
| T1 | mint→consume 왕복 | 발급 응답 ticket으로 소비 성공, 자원 필드 일치 |
| T2 | 만료 | expires_at 과거 조작 후 소비 → 거부 |
| T3 | 재사용 | 1회 소비 후 동일 티켓 재소비 → 거부(WHERE consumed_at IS NULL) |
| T4 | 티켓-자원 불일치 | host 티켓으로 hostId 다르게 접속 → 403 |
| T5 | 프로토콜 불일치 | asset 티켓을 k8s 엔드포인트에 → 403 |
| T6 | legacy 폴백+헤더 | token만으로 양 엔드포인트 접속 → 101 + 응답의 `Deprecation: true`·`Sunset:` 실측(gorilla Dialer resp.Header) |
| T7 | 티켓 경로 통과 입증 | asset: 존재 않는 host → **400**(=게이트 통과 후 서비스 도달); k8s: **101 수신**(업그레이드 선행 구조) |
| T8 | 발급 권한 | 터미널 버튼 권한 역할 → 200+ticket; 제로그랜트 역할 → 403(AnyOf)·resourceType 불일치 권한 → 403(M-1) |
| T9 | 410 전환 | GET diagnosis/run → 410+`Allow: POST`+`Location:`; POST → 무효 바디 400(핸들러 도달)·유효 그랜트로 403 아님 |
| T10 | 계약·아티팩트 | 기존 테스트 갱신분(437/427/285) + 골든 byte 일치 |

- WS 테스트 기법: `httptest.NewServer(engine)` + `websocket.Dialer`(Origin 불필요 — CheckOrigin true 실측). 서비스 목업 없음: 구체 타입 `*service.Service` 리팩터 금지(보존 제약 1) 대신, **게이트 이후 서비스 실패를 통과 증거로 사용**(T7) — DB 없는 host/cluster 조회 실패로 결정적 종료.
- 티켓 테스트 세션: `replaySession` 패턴 승계(AuthSession row + GenerateToken), DB는 sqlite 단일커넥션(M-2).
- 동시 경합 완전 재현은 단일커넥션 제약상 불가 — 원자성은 단일 UPDATE 문으로 논증 + 순차 재사용 테스트(T3)로 대체(한계 명시).

---

## 4. 위험 지점과 검증

| ID | 위험 | 완화·검증 |
|---|---|---|
| R-1 | gorilla `Upgrade`이 `c.Writer` 헤더 무시(raw 101 기록, 실측) → 폐기 헤더 소실 | 구현 계약: 헤더는 `Upgrade` 세 번째 인자로만 전달. T6이 응답 헤더를 직접 단언 — 미준수 시 즉시 red |
| R-2 | k8s는 업그레이드가 서비스 호출보다 선행 → 인증 게이트 통과 입증이 애매 | T7: 101 수신 자체를 통과 증거로(게이트 거부 시 401/403이면 다이얼 자체가 실패) |
| R-3 | 410 전환 파급 — 외부 스크립트·북마크가 GET 호출 시 410 수신 | 웹 UI는 동일 릴리스 프론트 1줄 전환으로 고장 창 **0**(Q3). 외부 소비자는 CHANGELOG 항목으로 안내(§4.8 명시 요구). 복구는 Phase B·C 커밋 revert로 즉시 |
| R-9 | Release A 단독 배포의 WS 공격면 개선 0 — URL 토큰 노출·무권한 접속 유지(Q2 정직 기록) | Release B·C 후속 태스크 등록을 머지 조건화(§9 계약). 티켓 게이트·테스트는 B 도입 즉시 유효하도록 선행 구축 |
| R-4 | 카운트 계약 갱신 누락/아티팩트 드리프트 | Phase C가 전용 단계 — 골든 테스트가 기계적으로 포착 |
| R-5 | legacy 폴백语义 혼동으로 무효 티켓까지 폴백(무한 우회) | A-1으로 봉쇄: ticket 부재시에만 폴백. T4·T5가 부정 단언 |
| R-6 | 기존 배포 DB 비super 역할의 신규 권한 미그랜트 | Release A 프론트 미전환으로 사용자 영향 0. Release B 전제조건으로 인계: 운영자 그랜트 또는 그랜트 스텝(별도 태스크) |
| R-7 | doc-blocker hook이 docs/CHANGELOG.md 생성 차단 가능 | 스펙 §4.8 요구 산출물 — 차단 시 사용자 승인으로 예외 처리(구현자 안내문 기재) |
| R-8 | 티켓 테이블 무한 증식 | 발급 시 만료 청소(24h 경과분 DELETE, best-effort) — 부하 무시 가능 수준 |

---

## 5. 롤백 가능성 판단

- **Phase별 독립 커밋** — 역순(D→C→B→A) revert로 단계 환원. 각 Phase가 자체 녹색 검증을 가져 중간 상태 환원도 안전.
- Phase D 단독 revert: WS는 즉시 legacy 전용으로 복귀(티켓 테이블·발급 엔드포인트 잔존하나 무해 — 미사용).
- Phase B·C revert(GET 핸들러 원복 + asset.js 원복)로 진단 기능 완전 복귀 — POST 라우트·티켓 인프라는 잔존 무해.
- DB: `sys_console_ticket` 테이블은 AutoMigrate 관례상 롤백하지 않음(빈 테이블 잔존 무해 — AuthSession과 동일 취급).
- 아티팩트는 언제든 `-update` 재생성으로 복구 가능.

---

## 6. 가정 (불확실 요소의 명시적 판정)

- **A-1**: team-lead 지시문의 "실패/부재 시 legacy 경로"와 "불일치 시 거부"의 상충은 **"부재 시에만 폴백"** 으로 해석 — ticket이 존재하면 무효(401)·불일치(403) 모두 거부. 근거: 무효 티켓 폴백은 이중수용 감지 불가·Release B 전환 디버깅 불가.
- **A-2**: k8s 바인딩은 cluster/namespace/pod. container·command는 미바인딩(같은 pod 내 컨테이너 교체는 인가 경계 아님 — pod가 침해 단위).
- **A-3**: Sunset 날짜는 placeholder 상수(2026-12-31) — Release B에서 확정 후 상수만 교체.
- **A-4**: TTL 30초 고정, 설정화하지 않음(§4.8 상한).
- **A-5**: 소비는 서비스 호출 성공과 무관하게 1회 — 재시도는 재발급으로.
- **A-6**: 발급 엔드포인트는 authGroup 배치 — Auth+OperationLog 자동 부착이 의도(발급 감사).
- **A-7**: Release A에서 WS 티켓 경로의 UI 사용자는 0(WS 프론트는 legacy 유지) — 권한 미그랜트(R-6)·티켓 경로 미사용 모두 사용자 영향 0. 단 이는 기능 보존이지 보안 개선이 아니다: WS 공격면 개선은 Release B 전까지 0(§0 정직 기록). 프론트 변경은 asset.js 1줄(진단 POST 전환)뿐.
- **A-8**: §4.8의 `WSS /api/v1/console/connect?ticket=...` 스펙 스케치에 대해 — "Phase -1: v1 path shape kept" 원문과 team-lead 지시(기존 WS 2종에 ticket 파라미터)에 따라 **기존 경로에 ticket 파라미터 추가**로 구현. `/console/connect` 신설은 Release B에서 필요 시 재검토.

---

## 7. 검증 요구 (claims — 구현 완료 후 리뷰가 검증)

1. `backend/model/console_ticket.go` 존재 + `sys_console_ticket` 문자열 포함(grep).
2. `backend/store/migrate.go` AutoMigrate 호출에 `&model.ConsoleTicket{}` 포함(grep).
3. `backend/router/router.go`에 `authGroup.POST("/console-sessions"`·`authGroup.POST("/asset/service/diagnosis/run"` 존재 + `GET /asset/service/diagnosis/run` 라우트 잔존(grep).
4. `grep -c "Method: http" backend/opdef/defs_*.go` 합계 = **285**(283+2).
5. `cd backend && go test ./... -race` 전 패키지 녹색(출력 제시).
6. `docs/security/sensitive-routes.txt`에 `POST /console-sessions`·`POST /asset/service/diagnosis/run` 라인 포함(헤더 `# 285 entries.`) + `docs/security/route-inventory.txt` **본문 라인 437 = `grep -c -- '-> ' docs/security/route-inventory.txt`**(헤더 3라인 제외, 파일 전체 wc -l은 440).
7. `routes_inventory_test.go`에 437·`authz_replay_test.go`에 427 단언 반영(grep).
8. `router/console_ticket_test.go`에 T4-T10 테스트 함수 존재 + 통과(함수명·출력 제시).
9. T6 테스트가 101 응답 헤더 `Deprecation`·`Sunset`을 직접 단언(코드 라인 제시).
10. GET `/asset/service/diagnosis/run` 410 + `Allow: POST` + `Location:` 단언 테스트 존재(함수명 제시).
11. `git diff`에서 `web/` 변경은 `web/src/api/asset.js` **단일 파일·단일 행**(GET→POST, timeout 240000 보존)에 한정 — 이외 web/ 파일 diff 0.
12. `controller/controller.go`·`controller/k8s.go`의 diff가 인증 게이트 블록·Upgrade 세 번째 인자·선행 파싱 라인에 한정 — `OpenAssetTerminal` 이후 스트림 파이프라인·`OpenK8sPodTerminal` 호출부 무변경(diff 확인).
13. `docs/CHANGELOG.md` 존재 + `diagnosis/run` 전환 항목 포함(grep).

## 8. 보존 제약 (구현 프롬프트에 verbatim 복사)

1. WS 핸들러의 터미널 스트림 로직(`OpenAssetTerminal` 호출 이후 블록 전체·`OpenK8sPodTerminal` 호출부와 에러 프레임 전송)은 변경하지 않는다 — 인증 게이트 블록 교체·Upgrade 세 번째 인자·바인딩 키 계산을 위한 선행 파싱만 허용된 편집이다.
2. `web/` 변경은 `web/src/api/asset.js`의 diagnosis/run 단일 행(GET→POST)에 한정한다 — 그 외 프론트 파일(WS 티켓 전환 포함)은 한 줄도 변경하지 않는다(Release B 소관).
3. legacy token WS 경로는 계속 동작한다(Release C까지 유지) — 토큰 검사는 기존 `auth.ParseToken` 경로 그대로, 폐기 헤더만 추가된다.
4. 기존 테스트 회귀 0 — 단 `routes_inventory_test.go`(435→437)·`authz_replay_test.go`(425→427)의 계약 수치 갱신과 아티팩트 2종 재생성은 본 태스크의 정당한 변경이다.
5. 기존 권한 어휘 외 신규 권한 문자열을 만들지 않는다 — `assets:service:diagnosis`·`assets:host:terminal`·`assets:k8s:pod:terminal`만 사용한다.
6. `glebarez/sqlite`는 테스트에서만 import한다(프로덕션 코드 금지 — T1 제약 승계).
7. `middleware/permission.go`의 M-1 추출은 동작 변경이 아니어야 한다 — `RequireAnyPermission`의 SQL·거부 응답은 byte 수준으로 동일하게 유지한다(기존 middleware 테스트가 수호).

---

## 9. Release B·C 인계 (본 태스크 범위 외 기록)

- **Release A 머지 시 계약(Q2)**: state.json에 후속 태스크 2건 등록을 **머지 조건**으로 한다 — 등록 누락 시 Release A는 미완료. 본 계획 수립 단계에서는 state.json 미기록(team-lead 관할).
- Release B(WS 티켓 프론트 전환): `Terminal.vue:119` URL 빌드를 ticket 플로우(POST 발급→`?ticket=`)로 교체·k8s 터미널 뷰 동일·Sunset 상수 확정(A-3)·비super 역할 그랜트 해소(R-6). asset.js 진단 POST 전환은 본 태스크(Release A)가 이미 수행.
- Release C(legacy query-token 제거): legacy token 게이트 제거·폐기 헤더 제거·`Deprecation` 관련 테스트 축소.

## 10. ④리뷰 인계 (r2.1 — rev-t4-console 승인, LOW 4)

- **F-1 계약 기록 정정**: 소비는 단일 UPDATE(WHERE hash·consumed·expires)의 RowsAffected==1이 권위 — 구현의 First() 선행 리드는 invalid/mismatch 분류 전용이며 이중 소비 창을 만들지 않는다. "조회-수정 2-step 금지"의 의도(경합 안전성)는 충족, 문언만 정정.
- **F-2 주석 정정 반영**: console.go 게이트 주석을 실제 검증 순서(프로토콜 짝은 mint 내부·M-1 이후)로 교체 완료.
- **F-4 길이 상한 반영**: CanonicalConsoleResourceID에 128자 cap 추가 — 권한자의 초과 길이 resourceId로 DB 에러 유도 차단.
- **F-3 기록**: k8s legacy 경로의 clusterId 누락이 101+에러프레임→400으로 변경 — 폐기 예정 경로의 외부 소비자만 이론상 영향.
- **INFO**: 티켓은 URL query bearer(스펙 설계) — 액세스 로그 유출 시 최대 30초 재생 창. asset 경로는 bindingKey에서 hostID 파생으로 티켓-터미널 구조적 일치(계획보다 강한 속성).
