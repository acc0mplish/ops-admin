# Ops Admin QA 프로젝트 컨텍스트

## Product

- **제품:** Ops Admin — 기업 운영·SRE 시나리오를 위한 내부 운영 콘솔.
- **유형:** Vue SPA 관리 플랫폼 + Go REST API.
- **현재 검증 환경:** Web `http://localhost:8080`, API `http://localhost:8082`.
- **핵심 비즈니스 체인:** 로그인·인증; RBAC 메뉴·권한 제어; 자산·Host·Database 관리; Kubernetes Cluster·Workload 작업; 표준 운영 Job 실행; Application Build·배포; 알림; 모니터링 조회·Alert·이벤트 처리.

## Tech Stack

- **Frontend:** Vue 3.5, Vue Router 4, Vite 5, Element Plus, Axios — 코드 위치 `web/`.
- **Backend:** Go 1.24, Gin 1.11, REST API, GORM — 코드 위치 `backend/`.
- **데이터·의존성:** MySQL 8; 기능에 따라 Redis, MongoDB, Prometheus 또는 VictoriaMetrics, Kubernetes, LDAP, Tencent Cloud 등 외부 시스템 연동.
- **배포:** Docker Compose로 MySQL·API·Web 서비스 제공; 이번 통합 검증은 로컬 서비스 기준.

## Test Stack

- **Backend:** Go 테스트 존재 — 주로 `backend/service/*_test.go`; 실행 명령 `go test ./...`.
- **Frontend 빌드:** `web/package.json`에 Vite build 제공; Playwright·Cypress·Vitest·Jest 테스트 스위트 설정 미발견.
- **이번 통합 검증:** 브라우저 핵심 경로 검증, HTTP API 스모크, Go 서비스 테스트; 이후 자동화는 Playwright 기본 채택.

## CI/CD

- GitHub Actions·GitLab CI·Jenkins 설정 미탐지.
- 현재 품질 게이트는 로컬 Backend 테스트·Frontend 빌드·통합 검증 보고서 기준; 커버리지·스크린샷·보고서 아티팩트 자동 업로드 미구성.

## Environments

- 로컬 Web: `http://localhost:8080`.
- 로컬 API: `http://localhost:8082`.
- MySQL은 Docker Compose 구성 — 컨테이너 내부 포트 3306.
- Production·Staging 주소, 외부 의존성 Credential, 데이터 마스킹 정책은 저장소에 미제공; 이번 라운드는 외부 Cloud·Cluster·Database·알림 채널에 쓰기 작업 미수행.

## Quality Goals

- Backend 전체 Go 테스트 통과 필수.
- Frontend Production 빌드 통과 필수.
- 고위험 API 미인증 접근은 반드시 거부; 핵심 읽기 경로는 파싱 가능한 성공 응답 반환.
- 핵심 브라우저 플로우는 로컬 환경에서 접근 가능 + 블로킹 Frontend Console 오류 없음.
- 현재 기준선: 핵심 API 스모크 10초 내 완료; 이후 목표 — 핵심 E2E 15분 미만, 실패 케이스 재현 가능.

## Risk Areas

| 영역 | 영향 | 확률 | 점수 | 등급 | 원인과 테스트 동작 |
|---|---:|---:|---:|---|---|
| 인증·RBAC | 5 | 3 | 15 | Critical | 미인증 접근 또는 권한 초과가 운영 기능 노출로 이어짐; 인증 차단·로그인·권한 메뉴·API 응답 검증. |
| 자산·Database·K8s 변경/삭제 | 5 | 3 | 15 | Critical | 운영 Resource·데이터 오조작 가능; Parameter 검증·참조 보호·확인 인터랙션·API 오류 응답 검증. |
| Application Build·배포·표준 운영 Job | 5 | 3 | 15 | Critical | 라이브 서비스 영향; 목록·상세·상태 피드백·보호 API 검증. |
| 환경 모델·설정 연관 | 4 | 4 | 16 | Critical | 삭제 후 재생성 이력 있음; 삭제 후 영속화·목록 일관성 검증. |
| 모니터링·Alert·알림 | 4 | 3 | 12 | High | 장애 시 사고 발견 지연; 페이지 로드·조회 오류 피드백·Rule API 검증. |
| 외부 Cloud·K8s·LDAP·모니터링 Data Source | 4 | 3 | 12 | High | 가용성·Credential이 결과 결정; 이번 라운드는 부작용 없는 경계·오류 경로 검사만 수행. |

## Team

- 현재 개발자 주도 품질 작업; 독립 QA 팀·비율 정보 미제공.
- 자동화 우선 = 핵심 비즈니스 경로·회귀 리스크 커버; 탐색적 테스트는 고위험 관리 작업의 예외 시나리오에 사용.

## Conventions

- Frontend 페이지 `web/src/views`, API 래퍼 `web/src/api`, Backend 라우팅 `backend/router`.
- 테스트는 기존 사용자 데이터 삭제·수정 금지; 쓰기 작업은 격리 데이터 또는 기존 보호 경로에서만 검증.
- 브라우저 자동화는 접근 가능한 이름·Role·명확한 화면 문구 우선 사용; 새 안정 자동화는 kebab-case `data-testid`.
- 테스트 아티팩트·보고서는 `.agents/` 또는 `docs/`에 두고 Production 소스 디렉토리 혼입 금지.
