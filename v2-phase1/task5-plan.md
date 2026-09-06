# V2 Phase -1 · Task 5 계획 — 콘솔 원타임 티켓 Release B(WS 프론트 전환) + C(legacy query-token 제거)

- 원천: `/tmp/v2-rev/docs/architecture/multi-infrastructure-control-plane-v2.md` **§4.8(Console and terminal containment)** — Release A(백엔드 이중수용)는 T4 완료(`3129bdc`). 본 태스크가 B·C를 동일 릴리스 세트로 마감한다.
- 범위: ①티켓 발급 API 프론트 함수(신규 `web/src/api/console.js`) ②터미널 다이얼 2곳의 `?token=` → `?ticket=` 전환(발급→다이얼 시퀀스, 재접속·컨테이너 교체 시마다 재발급) ③백엔드 WS 게이트의 legacy query-token 폴백 블록·폐기 헤더·Sunset 상수 제거 ④legacy 테스트 전환 ⑤CHANGELOG 제거 항목. **라우트·opdef·아티팩트 무변경**(437/427/285 유지).
- 실측 기준: main `3129bdc`(T4 완료 상태). 본 문서의 라인 번호·grep 기준선은 2026-09-06 작업 트리에서 전수 확인.
- 개정 이력: r1 → r2 (team-lead 조건부 승인 3건 반영 — F-1 async suspension point 방어 계약·mint 후 disconnect 재실행, i18n 패리티 게이트 claim 추가, F-4 writeln 단일화·F-3 테스트 주석 갱신 기록).

---

## 0. 현재 코드 실측 (계획의 사실 기반)

### 프론트 — 다이얼 지점은 정확히 2곳 (grep 전수)
- **asset 터미널**: `web/src/views/assets/Terminal.vue:111-145` `connectSocket(id)` — :118 `getToken()` → :119 템플릿 문자열로 `?hostId=&rows=&cols=&token=` 조립 → :120 `new WebSocket`. 호출처 2곳: `openHost`(:76, 탭 오픈)·`reconnect`(:171, 수동 재접속 버튼). **모든 다이얼이 connectSocket을 경유** — 이 함수 한 곳만 고치면 재발급 계약이 성립한다. 멀티세션(호스트별 탭)은 `runtimes` Map에 runtime별 socket을 보관(:96-121, `if (runtime.socket !== currentSocket)` 가드) — 세션 간 상태 공유 없음, mint도 호출마다 독립 발급이면 상호 간섭 0.
- **k8s 터미널**: `web/src/api/k8s.js:90-113` `buildK8sPodTerminalWSUrl`가 URL 조립(:104 `token: getToken() || ''`, URLSearchParams). 소비처는 **유일하게** `web/src/views/assets/K8sPodTerminal.vue:34`(`connectTerminal`). 호출처 3곳: 초기 자동 접속(:48)·컨테이너 선택 변경(:42 `handleContainerChange`)·연결 버튼(:60). 컨테이너 교체마다 재다이얼 = **재발급 필요**.
- **경합 조건 실측 (F-1의 사실 기반)**: ①`Terminal.vue:291` 재접속 버튼은 loading/disabled 부재 — 더블클릭으로 `connectSocket` 중복 진입 가능. ②`K8sPodTerminal.vue:60` 연결 버튼은 `:loading="connecting"`이 있으나 컨테이너 select는 `:disabled="!containers.length"`뿐 — mint 대기 중 선택 변경으로 `connectTerminal` 중복 진입 가능. ③`K8sPodTerminal` 소켓 핸들러(:35-38)에는 **socket identity 가드가 없다**(`Terminal.vue:122-144`의 `if (runtime.socket !== currentSocket)` 대조) — 게다가 `disconnectTerminal`(:40)은 `onclose`만 null화하고 onopen/onmessage/onerror는 리셋하지 않아 구소켓 이벤트가 상태를 오염시킬 수 있다. 현재(동기 코드)는 소켓 생성 직전 disconnect가 즉시 완료되어 창이 없으나, **async화로 disconnect와 `new WebSocket` 사이에 mint await suspension point가 생기면** 먼저 resume된 호출의 소켓이 다음 호출의 것에 덮어쓰이며 영구 미폐쇄로 남는다(백엔드 SSH 세션 잔류 — 보안 민감).
- `getToken()`의 URL 조립 사용은 위 2곳뿐(`grep -rn "token=" web/src` 현재 **1라인** = Terminal.vue:119). `http.js:63-88`의 getToken은 Authorization 헤더 전용 — URL 노출 아님, 무관.
- **프론트 테스트 인프라 부재 실측**: `web/package.json` scripts = dev/build/preview 3개뿐, 테스트 러너·설정 파일 전무. → 검증은 grep 계약 + 백엔드 E2E + 수동 스모크(§3).
- **기존 결함 발견(본 태스크 편집면 위)**: `Terminal.vue`는 :125·:137·:143 등에서 `at(...)`(asset-i18n 헬퍼)를 호출하나 **`import { at }` 누락** — 타 뷰(Application.vue:4 등)는 모두 import한다. vite auto-import 플러그인 없음(vite.config.js 실측 — plugins: [vue()]뿐). 현재도 터미널 오픈 시 onopen의 `at('sshWelcome')`에서 ReferenceError 발생(상태 갱신·소켓 I/O는 이전 라인이라 동작). Release B의 실패 경로가 `at()`를 무조건 실행하므로 **Phase A에서 import 1줄 추가로 수선**(:6-7 인접).

### 백엔드 — T4 완료 상태 (Release A)
- 발급: `POST /api/v1/console-sessions`(router.go:95, authGroup + opdef AnyOf) → `controller/console.go:41-75` `CreateConsoleSession` — canonical 바인딩(:55) → M-1 개별 권한(:60-64, `consoleResourcePermissions` 맵 :32-35) → mint. 응답 envelope `{code:200, data:{ticket, expiresAt, expiresIn}}`(httpx.Success 실측) — 프론트 http 인터셉터가 `res.data`를 반환하므로 `mint(...)`의 결과에서 `.ticket` 직접 접근 가능.
- 상수(T4 구현 확정값 — 프론트가 그대로 써야 할 계약): `service.ConsoleResourceAssetHost = "asset_host"`·`ConsoleResourceK8sPod = "k8s_pod"`·프로토콜 `"asset-terminal"`·`"k8s-pod-terminal"`(console_ticket.go:20-29). 바인딩 키: asset = hostId 십진 문자열(canonical `ParseUint` 정규화 — `"012"`→`"123"`), k8s = `"clusterId/namespace/podName"`(container·command 미바인딩, T4 A-2).
- 게이트: `controller/console.go:84-108` `authorizeTerminalWS(c, protocol, resourceType, resourceID) (http.Header, bool)` — ticket 있으면 원자 소비(불일치 403·무효 401, 폴백 없음), **ticket 없으면 legacy `auth.ParseToken` 폴백**(:98-107) + 폐기 헤더 반환.
- legacy 잔여물(Release C 제거 대상, 전부 실측): `legacyTerminalSunset` 상수(console.go:17)·`legacyTerminalWSHeaders`(console.go:24-27)·폴백 블록(console.go:98-107)·`strings`·`auth` import(console.go:5,8 — 제거 후 미사용)·호출부 `headers, ok :=` 형태(controller.go:996·k8s.go:289)·`Upgrade(c.Writer, c.Request, headers)`(controller.go:1005·k8s.go:297).
- 테스트: `router/console_ticket_test.go` — `TestTerminalWSLegacyTokenPath`(:218)이 legacy 101 + `Deprecation`/`Sunset` 헤더를 단언(제거 대상). 나머지 6개 테스트(왕복 :90·권한 :116·payload :146·mismatch :172·never-falls-back :197·410 :248)는 티켓 경로만 검증 — **Release C 후에도 무변경 통과**.
- CHANGELOG: `docs/CHANGELOG.md` 2026-09 항에 legacy 경로 폐기 예고(Deprecated 섹션) 이미 게시 — Release A가 §4.8의 "one release cycle" 창을 이미 이행했다.

### 라우트·계약 불변 근거
- Release C는 게이트 내부 교체뿐 — `router.go:45-46`(WS 2종 public 그룹)·:95(POST /console-sessions) 무동. 라우트 수 437·authGroup 427·opdef 285·아티팩트 2종 모두 불변. `authz_replay_test.go`의 `publicRouteKeys` WS 2종 명시도 불변.

---

## 1. 설계 결정 (구현 계약)

### D-1 티켓 발급 API — 신규 `web/src/api/console.js`
바인딩 계약(resourceType·resourceId 형식·protocol 페어링)을 **한 파일에만** 둔다 — 뷰에 문자열 상수가 흩어지는 것을 방지:
```js
import http from './http'

// §4.8 one-time console tickets: minted per dial, consumed by the terminal
// websocket gate. Single use — every (re)connect and container switch mints
// a fresh ticket (30s TTL).
export const mintAssetHostTerminalTicket = (hostId) =>
  http.post('/api/v1/console-sessions', {
    resourceType: 'asset_host',
    resourceId: String(hostId),
    protocol: 'asset-terminal'
  })

export const mintK8sPodTerminalTicket = ({ clusterId, namespace, podName }) =>
  http.post('/api/v1/console-sessions', {
    resourceType: 'k8s_pod',
    resourceId: `${clusterId}/${namespace}/${podName}`,
    protocol: 'k8s-pod-terminal'
  })
```
- 인터셉터가 `{ticket, expiresAt, expiresIn}`을 반환 — 뷰는 `(await mintX(...)).ticket`.
- resourceType/protocol 문자열은 백엔드 상수(§0)와 값 일치가 계약 — claims 2·3이 grep으로 수호.

### D-2 asset 터미널 전환 — `Terminal.vue` connectSocket
`connectSocket`을 async로: 기존 prologue(disconnectSession·status 'connecting') 유지 → **mint await** → **mint await 직후·`new WebSocket` 직전에 `disconnectSession(id, false)` 재실행**(F-1 방어 계약) → 실패 시 `catch`에서 `status:'error'` + `term.writeln(at('ticketIssueFailed'))` 후 return(재시도는 기존 재접속 버튼 = 새 mint) → 성공 시 :119 URL의 `token=`를 `ticket=${encodeURIComponent(ticket)}`로 교체. 이하 onopen/onmessage/onerror/onclose(:122-144)·나머지 함수 무변경.
- **F-1 방어의 작동 원리**: mint await 중 재호발(재접속 더블클릭)이 있어도, 각 호출의 resume 지점이 직전에 존재하는 소켓을 무조건 정리하고 자기 소켓만 남긴다 — 최종적으로 살아남는 소켓은 마지막 resume의 것 정확히 1개, 이전 소켓은 모두 close(백엔드 SSH 세션 잔류 없음). 소켓 identity 검사 방식보다 단순하며 Terminal.vue의 기존 `currentSocket` 가드(:122-144)와 이중으로 방어된다.
- mint 실패 표시는 **터미널 writeln 1회로 단일화**(F-4) — `ElMessage`는 http 인터셉터 소관, 뷰에서 중복 호출 금지(보존 제약 2).
- 호출처(`openHost`·`reconnect`)는 반환값 미사용 — async화의 파급 0.
- 발급 에러 UX 승계: http 인터셉터가 이미 `ElMessage.error`(api-error-i18n 현지화) 토스트 — 뷰는 터미널 내 writeln + 상태 점 'error'만.
- 신규 i18n 키 1개 `ticketIssueFailed`(asset-i18n.js ko/en 각 1행) — 기존 `sshConnectError`는 SSH 자격증명 오류 문구라 의미가 틀려 재사용하지 않는다.
- **동일 커밋에 `import { at } from '../../utils/asset-i18n'` 1줄 추가**(§0 기존 결함 — 본 태스크 실패 경로가 at()를 실행).

### D-3 k8s 터미널 전환 — `k8s.js` 빌더 + `K8sPodTerminal.vue`
- `buildK8sPodTerminalWSUrl` 시그니처에 `ticket` 추가·`token: getToken()` 제거·`params.set('ticket', ticket)`·파일 상단 `getToken` import 제거(k8s.js:2 — 미사용화).
- `connectTerminal`을 async로: `disconnectTerminal(); connecting.value=true` 유지 → mint await(`mintK8sPodTerminalTicket({clusterId: clusterId.value, namespace: namespace.value, podName: podName.value})`) → **mint await 직후·`new WebSocket` 직전에 `disconnectTerminal()` 재실행**(F-1 방어 계약 — mint 대기 중 컨테이너 select 변경·버튼 재클릭의 중복 진입을 흡수) → 실패 시 `connecting.value=false; connected.value=false; term?.writeln(kt('terminalConnectionFailed'))` 후 return → 성공 시 빌더에 `ticket` 전달.
- **소켓 핸들러 identity 가드 수선(F-1)**: `const currentSocket = new WebSocket(...)` 캡처 후 `socket = currentSocket`, 각 핸들러(:35-38) 첫 줄에 `if (socket !== currentSocket) return` — `Terminal.vue:122-144` 관례 승계. 구소켓의 지연 이벤트(onopen/onerror/onclose)가 신규 연결의 `connected`/`connecting` 상태를 오염시키는 것을 차단한다. `disconnectTerminal`은 무변경(재실행 계약이 소켓 정리를 보장하므로).
- mint 실패 표시는 **터미널 writeln 1회로 단일화**(F-4) — ElMessage는 인터셉터 소관, 뷰에서 중복 토스트 금지.
- 컨테이너 교체(`handleContainerChange`)·재연결 버튼은 connectTerminal 재호출 = 자동 재발급 — TTL 30s 내 다이얼이므로 만료 경합 없음.

### D-4 Release C — legacy 게이트 제거 (`controller/console.go` 외 2파일)
- `authorizeTerminalWS`를 `consumeTerminalTicket(c, protocol, resourceType, resourceID) bool`로 축소: ticket 부재 → **401 "console ticket required"**(폴백 제거), 불일치 403·무효 401 분기 유지. `legacyTerminalSunset`·`legacyTerminalWSHeaders`·`strings`/`auth` import 삭제.
- 호출부(`controller.go:996`·`k8s.go:289`): `headers, ok := ...` → `if !ctl.consumeTerminalTicket(...) { return }`, `Upgrade(c.Writer, c.Request, nil)`(controller.go:1005·k8s.go:297). **게이트 이후 스트림 로직은 무변경.**
- T4 A-3의 "Release B에서 Sunset 확정" 의무는 **제거로 이행** — 폐기 헤더가 다시는 발행되지 않으므로 날짜 확정이 무의미해지고, CHANGELOG가 실제 제거를 기록한다.

### D-5 Release B·C 동일 릴리스 세트 (T4 Q3 판정 승계)
- 커밋 순서 **B(프론트) → C(백엔드) 인접 순서, 동일 릴리스**. 근거: ①레거시 query-token 경로의 유일한 in-repo 소비자는 번들 웹 UI 2곳(grep 전수 실측) — 모노레포 compose 단일 빌드에서 창을 열 이유가 없다(T4 D-4/Q3의 "웹 UI 고장 창 0" 논리 승계). ②§4.8 "one release cycle is the honest window"은 Release A가 이미 이행(폐기 헤더 + CHANGELOG 예고 게시). ③대화형 xterm WS를 외부 스크립트가 소비하는 시나리오는 비현실적이며, 예고된 CHANGELOG 항목이 제거를 통지한다.
- B와 C 사이 중간 상태(프론트 티켓·legacy 잔존)는 정상 동작 — 커밋 분리는 revert 단위 확보가 목적이다.

---

## 2. 파일 배치 · Phase 분해 (10파일 → 3 Phase)

| # | 파일 | 신규/수정 | 내용 |
|---|---|---|---|
| 1 | `web/src/api/console.js` | 신규 | D-1 mint 래퍼 2개(바인딩 계약 단일화) |
| 2 | `web/src/utils/asset-i18n.js` | 수정 | `ticketIssueFailed` 키 ko/en 각 1행 |
| 3 | `web/src/views/assets/Terminal.vue` | 수정 | connectSocket async+mint+`ticket=` 교체, `at` import 추가 |
| 4 | `web/src/api/k8s.js` | 수정 | 빌더 `ticket` 파라미터·`getToken` 제거 |
| 5 | `web/src/views/assets/K8sPodTerminal.vue` | 수정 | connectTerminal async+mint |
| 6 | `backend/controller/console.go` | 수정 | D-4 게이트 축소·legacy 잔여물·import 제거 |
| 7 | `backend/controller/controller.go` | 수정 | :996 호출부·:1005 `Upgrade(..., nil)` |
| 8 | `backend/controller/k8s.go` | 수정 | :289 호출부·:297 `Upgrade(..., nil)` |
| 9 | `backend/router/console_ticket_test.go` | 수정 | `TestTerminalWSLegacyTokenPath` → legacy 제거 단언(401)으로 재작성 + `TestTerminalWSTicketNeverFallsBack` 주석 갱신(F-3 — 폴백 제거 후 "폴백 없음"이 동어반복 되므로 "폴백이 없는 게이트에서 무효 티켓 즉시 거부" 취지로; 테스트명·단언은 유지) |
| 10 | `docs/CHANGELOG.md` | 수정 | 2026-09 Removed 섹션 — legacy query-token 제거·외부 소비자 안내 |

**Phase (의존 순서, 각 독립 검증)**

- **Phase A — 발급 API + asset 터미널 (3파일)**: #1·#2·#3. 의존: 없음(T4 머지 완료). 검증: claims 1·2·3·9 grep + 수동 스모크(asset 터미널 1호스트 접속·재접속 — vite 라이브 + 백엔드, legacy 게이트가 아직 있으므로 티켓 경로만 정상이면 통과).
- **Phase B — k8s 터미널 (2파일)**: #4·#5. 의존: A(#1). 검증: claims 4·5 grep + 수동 스모크(k8s 포드 터미널 1포드 접속·컨테이너 교체).
- **Phase C — legacy 제거 (5파일)**: #6-#10. 의존: A·B(프론트 전환 완료 후에만 제거 가능 — D-5). 검증: `cd backend && go test ./... -race` 전 패키지 녹색 + claims 6·7·8·10 + `git diff`로 router/opdef/docs/security 무변경 확인 + 수동 스모크 재확인(양쪽 터미널).

Phase C가 5파일인 까닭: 게이트 제거(#6-#8)와 legacy 동작을 단언하던 테스트 재작성(#9)은 원자적 — 분리 시 게이트 제거 커밋에서 테스트가 red로 남는다. CHANGELOG(#10)은 제거 커밋과 동반(§4.8 changelog 의무). 전역 규칙(≤5파일/Phase) 내.

---

## 3. 테스트 계약 (프론트 테스트 인프라 부재 — 대체 검증 체계)

| ID | 검증 | 수단 |
|---|---|---|
| V-1 | 티켓 다이얼 E2E | **T4 기존 테스트 승계**: `TestConsoleSessionMintTicketRoundtrip`(mint→소비→서비스 도달)·`TestTerminalWSTicketNeverFallsBack`·`TestTerminalWSTicketMismatchRejected`·`TestConsoleSessionPermissions` — Release C 후에도 무변경 통과(게이트 축소가 티켓 경로 분기(:85-95)를 그대로 보존) |
| V-2 | legacy 제거 | `TestTerminalWSLegacyTokenPath` 재작성: token-only 다이얼 양 엔드포인트 → 401·티켓 부재 → 401. Deprecation/Sunset 헤더 단언 삭제(존재하지 않는 메커니즘) |
| V-3 | 회귀 | `go test ./... -race` 전 패키지 — routes_inventory(437)·authz_replay(427/285)·opdef 불변식이 라우트 무변경을 기계적으로 수호 |
| V-4 | 프론트 계약 | grep(§7 claims): `token=` URL 조립 0·mint 호출 존재·`getToken` 잔여는 http.js/router 가드뿐 |
| V-4b | i18n 패리티 | `node scripts/check-i18n-parity.mjs`(repo 루트 실행 — web/ 아님 실측) exit 0 — `ticketIssueFailed` 키가 asset-i18n ko/en 대칭(현재 기준선 ko=773/en=773, 스크립트 :213 커버 확인) |
| V-5 | 수동 스모크 | `cd web && bun run dev`(vite :8080, /api/v1 프록시 ws:true 실측) + 백엔드 :8082 — asset 터미널 1호스트(접속·재접속 버튼 = 재발급 경로)·k8s 터미널 1포드(접속·컨테이너 교체 = 재발급 경로). 무권한 계정으로 403 토스트 확인(선택) |

프론트 단위/E2E 테스트 부재는 기존 제약(패키지 scripts 실측) — 본 태스크가 인프라를 도입하지 않는다(범위 외, 가정 A-6).

---

## 4. 위험 지점과 검증

| ID | 위험 | 완화·검증 |
|---|---|---|
| R-1 | mint 실패 시 터미널 무반응(연결 중 상태 방치) | D-2·D-3 catch 블록이 status/ connecting/connected를 확실히 되돌린다 — 수동 스모크에서 무권한·무효 host 케이스 확인(선택) |
| R-2 | **F-1: async화의 suspension point에서 선행 소켓 영구 미폐쇄** — 재접속 더블클릭(Terminal.vue:291 loading 부재)·mint 대기 중 컨테이너 select 변경(K8sPodTerminal select 비활성화 부재) 시 먼저 resume된 호출의 소켓이 덮어쓰이며 백엔드 SSH 세션 잔류 | D-2·D-3 방어 계약: **mint await 직후·new WebSocket 직전 disconnect 재실행** — 각 resume이 직전 소켓을 정리하고 자기 소켓만 남긴다(최종 생존 소켓 1개 보장). K8sPodTerminal 핸들러 identity 가드 수선(D-3)이 구소켓 이벤트 상태 오염을 차단. 수동 스모크에서 재접속 더블클릭·컨테이너 빠른 교체 케이스 확인 |
| R-2b | 이중 mint 경합(구 티켓 잔여) | 티켓은 1회용·30s TTL — 미소비 티켓은 자연 소멸(T4 청소 계약 승계). Terminal.vue의 기존 `runtime.socket !== currentSocket` 가드(무변경)가 구소켓 이벤트 차단 |
| R-3 | C 적용 직후 브라우저에 남은 구 프론트(JS 캐시)가 token 다이얼 → 401 | 단일 운영자 도구 — 새로고침으로 해결. CHANGELOG Removed 항목에 명시(§4.8 통지 의무) |
| R-4 | C 제거 후 미사용 import(`strings`·`auth`)로 컴파일 실패 | `go build ./...`가 즉시 포착 — Phase C 검증 첫 단계 |
| R-5 | 라우트/아티팩트 드리프트 | 본 태스크는 router.go·opdef·docs/security 무편집 — V-3 골든 테스트 + git diff 이중 확인(claims 8) |
| R-6 | k8s resourceId 형식 드리프트(프론트 조립 vs 백엔드 canonical) | 형식을 console.js 래퍼 1곳에 격리(D-1). 백엔드 canonical이 사소한 공백/선행 0을 흡수(console_ticket.go:60-87) — 불일치 시 400/403으로 즉시 가시적 |
| R-7 | ElMessage 이중 토스트·이중 메시지(mint 실패) | 인터셉터만 토스트, 뷰는 **터미널 writeln 1회로 단일화**(F-4 반영 — D-2·D-3·보존 제약 2에 명문화) |
| R-8 | at() import 누락 기존 결함 방치 시 신규 실패 경로 ReferenceError | Phase A에서 1줄 수선(§0 근거) — claims 9로 검증 |

---

## 5. 롤백 가능성 판단

- **revert 순서는 반드시 C → B → A**(역순). C 단독 revert: 프론트는 티켓 경로 유지 → T4 이중수용 게이트 복원되어 정상 동작(안전). **B만 남기고 A를 revert하면 안 됨**: 프론트가 legacy token을 다이얼하는데 게이트가 제거됨 → 터미널 전면 401. C→B 역순 revert면 언제나 중간 상태가 동작 가능 조합이다.
- Phase A·B revert: 프론트가 token 경로 복귀 → Release A 게이트가 수용(이중수용 창) — 안전.
- DB: `sys_console_ticket` 테이블·발급 엔드포인트는 T4 소관 — 본 태스크 rollback과 무관.
- CHANGELOG 항목은 revert 시 함께 되돌리는 것을 권장(문서-코드 정합).

---

## 6. 가정 (불확실 요소의 명시적 판정)

- **A-1**: B·C 동일 릴리스 세트(D-5) — T4 Q3의 모노레포 판정 승계. 배포는 compose 단일 빌드로 프론트·백엔트가 항상 동반됨(분리 배포 시나리오 없음).
- **A-2**: legacy query-token WS의 외부 소비자는 존재하지 않는다고 본다(in-repo 소비자 2곳 grep 실측·대화형 프로토콜). 존재 시 CHANGELOG가 유일 통지 채널 — 단일 운영자 도구의 정직한 창.
- **A-3**: TTL 30s 내 mint→다이얼이므로 만료 재시도 루프 불요 — 만료(이론상 탭 백그라운딩)는 401 → 사용자 재접속 → 새 mint.
- **A-4**: 티켓도 URL query bearer다(§4.8 설계) — 액세스 로그 유출 시 최대 30s 재생 창은 T4 ④리뷰 INFO로 기록된 잔여 위험. 본 태스크의 개선은 **메인 액세스 토큰이 URL에서 사라지는 것**(§3.3 위반 해소).
- **A-5**: mint 403(권한)/400(자원 무효)의 에러 문구는 인터셉터의 api-error-i18n 현지화에 맡긴다 — 뷰에서 별도 코드 분기·번역을 만들지 않는다(얇은 계획).
- **A-6**: 프론트 테스트 인프라 도입(vitest·Playwright)은 범위 외 — §3 대체 검증 체계로 수호.
- **A-7**: `Terminal.vue` at() import 수선은 T5 발견 기존 결함의 최소 수선이다(신규 결함 아님·1줄) — 스펙 범위 확장이 아니라 편집면 위 결함의 해소.

---

## 7. 검증 요구 (claims — 구현 완료 후 리뷰가 검증)

1. `grep -rn "token=" web/src` → **0라인**(기준선 1라인 Terminal.vue:119 제거).
2. `web/src/api/console.js` 존재 + `console-sessions`·`asset_host`·`k8s_pod`·`asset-terminal`·`k8s-pod-terminal` 문자열 포함(grep).
3. `web/src/views/assets/Terminal.vue`에 `mintAssetHostTerminalTicket` 호출 + URL에 `ticket=` 존재 + `getToken` 0회(grep) + `import { at } from '../../utils/asset-i18n'` 라인 존재 + **F-1 방어: `connectSocket`에서 `disconnectSession(id, false)`가 mint await를 기준으로 전후 2회 실행**(코드 라인 제시).
4. `web/src/api/k8s.js`에 `getToken` 0회(import 포함) + 빌더가 `ticket` 파라미터 수용; `web/src/views/assets/K8sPodTerminal.vue`에 `mintK8sPodTerminalTicket` 호출 + **F-1 방어: `disconnectTerminal()`가 mint await를 기준으로 전후 2회 실행 + `const currentSocket` 캡처·`if (socket !== currentSocket) return` 가드가 onopen/onmessage/onerror/onclose 4핸들러에 존재**(grep -c = 4).
5. `web/src/utils/asset-i18n.js`에 `ticketIssueFailed` ko·en 각 1회(grep -c = 2) + **repo 루트에서 `node scripts/check-i18n-parity.mjs` exit 0**(asset-i18n ko=en 대칭 기계 검증 — 현재 773/773에서 774/774로 증가).
6. `backend/controller/console.go`에 `legacyTerminalSunset`·`legacyTerminalWSHeaders`·`ParseToken`·`strings`·`authorizeTerminalWS` 0회(grep) + `consumeTerminalTicket` 정의 존재.
7. `backend/controller/controller.go`·`k8s.go`가 `consumeTerminalTicket` 호출 + `Upgrade(c.Writer, c.Request, nil)`(grep) — 게이트 이후 스트림 블록은 diff로 무변경 확인.
8. `git diff <base>..HEAD -- backend/router/router.go backend/opdef/ docs/security/` → **빈 diff**(라우트 437·427·285·아티팩트 불변).
9. `cd backend && go test ./... -race` 전 패키지 녹색(출력 제시) — `TestTerminalWSLegacyTokenPath` 재작성본 포함.
10. `backend/router/console_ticket_test.go`에 token-only 다이얼 401 단언 존재(함수명·코드 라인 제시).
11. `docs/CHANGELOG.md`에 legacy query-token 제거 항목(Removed) 존재 — 양 WS 경로 명시(grep).
12. 수동 스모크 결과 제시: asset 터미널 1호스트 접속+재접속, k8s 터미널 1포드 접속+컨테이너 교체 — 각 접속 성공·`token=` 미포함(브라우저 네트워크 탭 또는 서버 로그). **F-1 경합 케이스 포함**: 재접속 버튼 더블클릭·k8s 컨테이너 빠른 연속 교체 후에도 터미널 I/O가 최종 연결 1개로 정상 동작하고 백엔드에 잔류 세션이 없음(서버 로그 또는 세션 카운트로 확인) 확인.

## 8. 보존 제약 (구현 프롬프트에 verbatim 복사)

1. 터미널 스트림 로직은 변경하지 않는다 — `Terminal.vue`의 onopen/onmessage/onerror/onclose 핸들러·runtimes/세션 탭 관리·리사이즈 동기화, `K8sPodTerminal.vue`의 소켓 핸들러·JSON stdin/resize 프로토콜, 백엔드의 `OpenAssetTerminal` 이후 파이프라인·`OpenK8sPodTerminal` 호출부와 에러 프레임 전송. 허용된 편집은 mint 호출 삽입·URL 조립 교체·게이트 호출부 형태 교체·`Upgrade` 세 번째 인자 nil화·관련 import에 더해 **F-1 방어 편집 2종 — mint await 직후·new WebSocket 직전의 disconnect 재실행(`Terminal.vue` `disconnectSession(id, false)`·`K8sPodTerminal.vue` `disconnectTerminal()`), `K8sPodTerminal.vue` 소켓 핸들러의 `currentSocket` 캡처·identity 가드(`if (socket !== currentSocket) return` — Terminal.vue:122-144 관례 승계)**뿐이다. 핸들러 본문의 writeln·프로토콜 payload는 무변경이다.
2. mint 실패 시 뷰는 `ElMessage`를 호출하지 않는다 — 토스트는 http 인터셉터가 담당하고 뷰는 상태 점·터미널 writeln만 변경한다.
3. 라우트·opdef·권한 어휘·아티팩트는 한 줄도 변경하지 않는다 — `backend/router/router.go`·`backend/opdef/`·`docs/security/route-inventory.txt`·`docs/security/sensitive-routes.txt`는 무편집, 신규 권한 문자열 금지.
4. 기존 백엔드 테스트 회귀 0 — 유일한 예외는 `TestTerminalWSLegacyTokenPath`의 재작성(단언 대상인 legacy 동작 자체를 본 태스크가 제거)이다.
5. WS 2종 엔드포인트는 public api 그룹(`router.go:45-46`)에 그대로 둔다 — 티켓이 곧 인증이다.
6. 커밋 순서는 B(프론트 Phase A·B) → C(백엔드 Phase C)이며, C는 프론트 전환이 머지된 뒤에만 반영한다 — revert는 반드시 역순(C→B→A)으로 한다.
