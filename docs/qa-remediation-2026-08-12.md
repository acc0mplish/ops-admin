# QA 조치 및 재검증 기록 (2026-08-12)

이 기록은 `qa-integration-test-report-2026-08-12.md`의 Release Blocking 항목에 대한 조치 결과입니다. 원본 Report는 조치 전 Audit Baseline으로 유지합니다.

## 완료 항목

| Priority | 조치 항목 | 재검증 근거 | 상태 |
| --- | --- | --- | --- |
| P1 | HTTP 실패 응답에 실제 HTTP Status Code 적용 | Token 없이 `/api/v1/profile`에 접근하면 HTTP 401을 반환하고 Response Body의 Business Code도 401로 일치 | 통과 |
| P1 | Login Page 사전 입력 제거 | 초기 Password는 `OPS_ADMIN_INITIAL_PASSWORD`로 설정하며 브라우저 Login Page의 두 Input은 모두 빈 상태 | 통과 |
| P2 | Detail 및 Log Page의 Context 누락 처리 | `/containers/services/workload`, `/containers/services/logs`에 직접 접근하면 안내 Empty State를 표시하고 Browser Console Error 없음 | 통과 |
| P2 | CORS 및 Security Response Header | `OPS_ADMIN_CORS_ORIGINS`에 명시한 Origin만 허용하고 미등록 Origin의 Preflight에는 Allow-Origin을 반환하지 않음. API는 nosniff, DENY, Referrer/Permissions/CSP Header를 반환 | 통과 |
| P2 | `go vet` | IPv6 Address를 `net.JoinHostPort`로 조합하고 Unreachable Code를 제거함. `go vet ./...` Exit Code 0 | 통과 |
| P3 | Element Plus Radio 호환성 | 기존 `el-radio`의 `label` Value를 `value`로 Migration | 통과 |

## 검증 결과

- Backend: `go test ./...`, `go vet ./...` 모두 통과했습니다.
- Frontend: `npm run build` 통과, 총 2,182개 Module을 Build했습니다. 초기 JavaScript Bundle이 3.1 MB라는 Warning은 남아 있으며 이번 조치 범위와 무관한 Refactoring을 피하기 위해 Code Splitting은 수행하지 않았습니다.
- Browser: Logout, 빈 Login Page 확인, 재로그인을 완료했습니다. Service Workload와 Log Page의 Direct Access Empty State에서도 Console Error가 없었습니다.

## Deployment 주의사항

- Deployment Template의 `deploy/.env`에는 기본적으로 `OPS_ADMIN_INITIAL_USERNAME=admin`, `OPS_ADMIN_INITIAL_PASSWORD=admin@123`이 설정됩니다. Production 환경에서는 반드시 기본 Password를 변경합니다.
- API를 Cross-Origin으로 호출해야 할 때는 comma로 구분한 `OPS_ADMIN_CORS_ORIGINS` Allowlist를 구성합니다. Same-Origin Nginx Deployment에서는 이 변수가 필요하지 않습니다.

## 남은 Release 점검 항목

1. E2E, API Contract Test, Security Scan, Performance Budget을 CI에 연결합니다.
2. Route 기반 Code Splitting으로 Frontend Bundle을 분할하고 Lighthouse Metric Budget을 설정합니다.
3. 실제 Domain을 사용하는 HTTPS Reverse Proxy Layer에서 HSTS를 활성화합니다.
