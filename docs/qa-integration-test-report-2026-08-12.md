# Ops Admin 프론트엔드·백엔드 Integration Test Report

- **실행일:** 2026-08-12
- **검증 Environment:** Web `http://localhost:8080`; API `http://localhost:8082`
- **Test 방식:** Go 서비스 Test, Vite Production Build, 실제 Browser Login과 Route 순회, HTTP API Smoke 및 Negative 검증, 프론트엔드·백엔드 정적 Contract 대조, 통제된 Performance Smoke.
- **데이터 정책:** 기존 비즈니스 데이터는 수정하지 않았습니다; Environment Model CRUD는 `qa-e2e-*` 식별자가 붙은 임시 데이터만 생성·정리했습니다.

## 결론

**현재 결론: No-Go (Production 릴리스 후보로 직접 사용은 비권장).**

주요 기능 경로, 검증된 91개 Page Route, Environment Model End-to-End CRUD, Backend Test, Frontend Production Build는 모두 통과했습니다. 하지만 인증과 오류 Response가 모두 HTTP 200으로 비즈니스 오류 코드를 전달해 HTTP API 시맨틱을 위반하며, Gateway, 모니터링, SDK Retry, 외부 클라이언트가 미인증·파라미터 오류를 성공으로 오판할 수 있습니다. 이는 P1 Release 차단 항목입니다.

## 커버리지와 결과

| 범주 | 커버리지 내용 | 결과 | 근거 |
|---|---|---|---|
| Backend Regression | `backend` 전체 Go Test | 통과 | `go test ./...` exit code 0; `service` Package 통과 |
| Frontend Build | Vite Production Build | 통과하나 Performance 경고 있음 | 2,182 modules; 산출물 JS 3,114.54 kB, gzip 919.14 kB |
| 서비스 Health | Web·API 도달 가능 | 통과 | Web `8080`과 API `8082/ping` 모두 성공 반환 |
| 인증 주요 Flow | 실제 Browser Login, Session 수립, Dashboard 로딩 | 통과 | Login 후 `/dashboard` 도달, Console error 미검출 |
| Page Integration | 콘솔, 자산, Container, 표준 운영, Application, Notification, 모니터링, Integration, 개인 화면 | 통과, 예외 Detail Page 2건 존재 | 91개 Route 모두 Login Page로 Fallback 없음; 자세한 내용은 "문제 목록" 참조 |
| API Read Smoke | profile, 자산, K8s, Environment, Application, Notification, 모니터링, Integration 9개 API 그룹 | 통과 | 유효 Session Token으로 모두 비즈니스 코드 200 반환 및 data 존재 |
| Environment Model CRUD | 임시 Environment 생성 → Keyword 조회 → 삭제 → 재조회 | 통과 | 임시 Record 생성 성공, 삭제 후 조회 결과 0; 자동 재생성 없음 |
| 삭제 파라미터 보호 | Environment 삭제 API `id=0` | 통과 | 비즈니스 코드 400 반환, 데이터 미삭제 |
| Frontend·Backend API 매핑 | Frontend `web/src/api` 정적 경로와 Backend router Route | 통과 | Frontend 375개 정적 API 경로 모두 Backend 379개 Route 리터럴에서 매핑 확인 |
| API Performance Smoke | `/ping` 30 동시성 무부작용 Request | 통과 | 30/30 성공, 오류율 0%, P50 52.69 ms, P95 57.39 ms, P99 58.24 ms |
| 보안 Negative | Token 없음, 가짜 Token, 잘못된 Login JSON, TRACE Method, Response Header | 실패, P1/P2 존재 | 자세한 내용은 "문제 목록" 참조 |
| 정적 검사 | `go vet ./...` | 실패 | IPv6 주소 형식 문제 2건, 도달 불가 코드 1건 |

## Browser Integration 상세

실제 Browser로 Login하고 Page Component가 있는 91개 Route에 대해 읽기 전용 로딩 검증을 수행했습니다. 커버리지:

- 콘솔, 비즈니스 토폴로지, 시스템 관리, 감사 Log.
- Asset Overview, Host, Credential, Cloud Account, Database, DBMS, Gateway, Environment Model.
- Kubernetes Cluster, Node, Namespace, Workload, Pod, Service, Ingress, 네트워크와 Storage.
- 서비스 관리, Health Diagnosis, 표준 운영, Job, Scheduled Task.
- Application Project, Build, Pipeline, Image Registry, Application 토폴로지와 Application Center.
- Notification, 모니터링 Query/Alert/Dashboard, AI 및 FinOps Integration Page.

테스트한 모든 Page는 로그인 상태를 유지하고 화면에 보이는 Page 콘텐츠를 생성했습니다. Environment Model "Environment 추가" 대화상자의 열림·닫힘을 검증했으며 비즈니스 데이터를 생성하지 않았습니다.

다음 Redirect는 현재 Route 설정과 일치합니다: `/assets/hosts`, `/assets/tags`, `/assets/server/databases`, `/applications/center` 등 과거/Alias 경로는 해당하는 현재 Page로 이동합니다.

## 문제 목록

### P1: 오류 Response가 항상 HTTP 200 반환

- **현상:** Token 없이 `/api/v1/profile`, `/api/v1/ops/environment/list` 등 보호 API를 호출하면 HTTP 상태가 모두 200이고 body에야 `code: 401`이 담깁니다. 가짜 Token도 HTTP 200 + `code: 401`을 반환; 잘못된 Login JSON은 HTTP 200 + `code: 400`을 반환.
- **영향:** Reverse Proxy, APM, Alert, Browser 외부 클라이언트, 범용 SDK가 실패를 성공으로 집계할 수 있음; HTTP 상태 기반 Retry, Cache, 접근 감사가 신뢰할 수 없게 됨.
- **원인:** [backend/httpx/response.go](D:\go\ops-admin\backend\httpx\response.go:11)의 `Failed`가 무조건 `c.JSON(200, ...)`을 호출.
- **권고:** `c.JSON(code, ...)`으로 변경하고 body의 비즈니스 code는 Frontend 호환을 위해 유지; 400/401/403/404/5xx와 Axios interceptor 동작을 Regression 검증.

### P1: 초기화 Super Admin Credential이 예측 가능하고 Login Page에 사전 채워짐

- **현상:** Browser Login Page에 사용 가능한 초기화 관리자 Credential이 사전 채워짐; 초기화 로직은 [backend/store/seed.go](D:\go\ops-admin\backend\store\seed.go:461)에 고정 관리자 및 고정 비밀번호 생성.
- **영향:** Deploy 후 즉시 비밀번호를 변경하지 않으면 공격자가 최고 권한을 획득할 수 있음.
- **권고:** 첫 Deploy에서 Environment Variable/일회용 무작위 비밀번호 초기화로 전환; 첫 Login 시 비밀번호 변경 강제; Production Mode에서 비밀번호 사전 채우기 금지 및 README에 Rotation 절차 명시.

### P2: 서비스 Detail·Log 직접 Route가 Query Parameter 누락 시 처리되지 않은 Exception 발생

- **현상:** `/containers/services/workload`와 `/containers/services/logs`에 직접 접근하면 Browser Console에 mounted hook 미처리 Exception과 `record not found` 발생.
- **영향:** Bookmark, 새로고침, 직접 URL 진입 시 불완전한 Page와 Console Exception이 발생.
- **관련 위치:** [router/index.js](D:\go\ops-admin\web\src\router\index.js:222), [ServiceWorkloadDetail.vue](D:\go\ops-admin\web\src\views\assets\ServiceWorkloadDetail.vue:27), [ServiceWorkloadLogs.vue](D:\go\ops-admin\web\src\views\assets\ServiceWorkloadLogs.vue:23).
- **권고:** Parameter 누락 시 "서비스/Workload를 선택하십시오." 빈 상태를 표시하고 Request를 차단하거나, 직접 Route를 서비스 관리 Page로 Redirect.

### P2: 보안 Response Header 미비, CORS 범위 과도하게 넓음

- **현상:** `/ping` Response에는 기본 CORS와 Content-Type만 존재; `X-Content-Type-Options`, `X-Frame-Options`, `Content-Security-Policy`, HSTS 등 보안 헤더 미발견. `Access-Control-Allow-Origin: *`은 내부 운영 플랫폼에 과도하게 넓음.
- **영향:** 방어 깊이 부족; 향후 Cross-domain Token 사용 또는 Page 삽입이 도입되면 공격 표면 확대.
- **권고:** Deploy Domain별 CORS 화이트리스트 구성; Reverse Proxy 또는 Gin에 보안 Response Header 추가. HSTS는 HTTPS Production Environment에서만 활성화.

### P2: `go vet`이 IPv6 연결과 도달 불가 코드 문제 발견

- **현상:** `go vet ./...` 실패: `fmt.Sprintf("%s:%d", host, port)`를 `net.Dial`에 전달하는 두 곳이 IPv6와 비호환; Host Group 삭제 로직에 `return` 이후 도달 불가 코드 존재.
- **관련 위치:** [backend/service/service.go](D:\go\ops-admin\backend\service\service.go:1105), [backend/service/service.go](D:\go\ops-admin\backend\service\service.go:1693), [backend/service/service.go](D:\go\ops-admin\backend\service\service.go:2553).
- **권고:** `net.JoinHostPort(host, strconv.Itoa(port))` 사용; 도달 불가 중복 검사 삭제; `go vet ./...`을 CI 게이트에 추가.

### P3: Element Plus 호환성 경고

- **현상:** Route 순회에서 `el-radio`의 `label`을 value로 쓰는 방식의 Deprecation 경고가 여러 번 발생.
- **영향:** 현재 기능은 차단되지 않지만 Element Plus 3 업그레이드 시 호환성 Risk 존재.
- **권고:** 해당 `el-radio`의 `label` value 용법을 `value`로 이전.

### P3: Frontend 첫 Bundle 크기가 Vite 기본 Budget 초과

- **현상:** Production JS Bundle 3.04 MB, gzip 919 KB, Build 시 chunk 500 KB 초과 경고.
- **영향:** 저속 네트워크와 첫 로딩 경험에 Risk 존재.
- **권고:** Application Route별 Dynamic Import로 X6/DBMS/모니터링 등 무거운 의존성 분할; 이후 Lighthouse CI로 LCP, CLS, TBT 게이트 구축.

## Contract, 자동화 및 Release 준비도 Gap

- OpenAPI/Swagger 정의, Pact Contract, Playwright/Cypress/Vitest Frontend Test 설정 또는 CI Workflow 미발견.
- 현재 Frontend 정적 API 경로와 Backend Route의 문자열 매핑은 완전하지만 Response Schema, 상태 코드, 권한 Matrix, Version 호환성 Test를 대체하지 못함.
- OSV/Semgrep/ZAP/Secret Scan 등 지속 보안 게이트 미발견, Lighthouse 또는 k6의 CI Performance Budget 미구축.
- P1 문제 존재와 반복 가능한 E2E, Contract, 보안 스캔 근거 부재로 이번에는 Production Release Go 결론을 낼 수 없음.

## 권장 수정·재검증 순서

1. HTTP 상태 코드 시맨틱과 초기화 관리자 Policy를 수정하고 Login, 401, 403, 400, 삭제/저장 실패 경로를 우선 재검증.
2. 서비스 Workload·Log 직접 Page의 Parameter 누락 빈 상태를 수정하고 Bookmark, 새로고침, Query Parameter 없는 접근을 재검증.
3. `go vet` 3건 문제를 수정하고 `go test ./...`, `go vet ./...`, `npm run build`를 기본 게이트로 설정.
4. Playwright Smoke 추가: Login, Dashboard, Environment CRUD, 자산 Read, 모니터링 Read, Application Pipeline 읽기 전용 Page와 주요 빈 상태.
5. OpenAPI 또는 공유 JSON Schema를 구축한 뒤 Frontend Consumer와 Go Provider에 Contract 검증 추가.
6. Staging Environment에 OSV, Semgrep, ZAP baseline, Lighthouse, k6 임계값 추가; 수정 완료 후 본 Report의 전체 Test Case 재실행.

## Release 게이트 결론

| 게이트 | 상태 |
|---|---|
| Backend Unit/서비스 Test | 통과 |
| Frontend Production Build | 통과, bundle 경고 있음 |
| Browser 주요 Page 도달성 | 통과, Detail 직접 Page에 예외 있음 |
| API 주요 Read와 Environment CRUD | 통과 |
| HTTP 오류 시맨틱과 인증 경계 | **실패, P1** |
| 보안 Baseline | **실패, P1/P2** |
| 정적 검사 | **실패** |
| E2E, Contract, Performance, 보안 CI 근거 | 미구축 |
| **최종 결정** | **No-Go, P1 수정·재검증 완료 전 Release 비권장** |
