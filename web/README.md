# Ops Admin 프런트엔드

Ops Admin 프런트엔드는 Vue 3, Vite, Vue Router, Element Plus 기반의 SPA Console입니다. `/api/v1`을 통해 Backend API를 호출하며, Cloud Provider Credential을 브라우저에 직접 저장하거나 사용하지 않습니다.

## 요구 환경

- Node.js 18+
- npm 9+
- 접근 가능한 Ops Admin Backend. 개발 환경 기본 주소는 `http://127.0.0.1:8082`입니다.

## 설치 및 실행

```powershell
cd web
npm ci
npm run dev
```

Vite 개발 Server는 기본적으로 `http://127.0.0.1:8080`에서 실행됩니다. `vite.config.js`는 `/api/v1`과 `/uploads` 요청을 Backend의 `8082` Port로 Proxy합니다.

Production Build:

```powershell
cd web
npm run build
npm run preview
```

## 디렉터리 구성

| 경로 | 설명 |
| --- | --- |
| `src/views/` | 업무 Domain별 Page Component |
| `src/views/integration/finops/` | Cloud Account, 비용 Dashboard, 비용 분석, 최적화 권고, Resource 비용 분석, Billing 동기화 Page |
| `src/views/integration/` | AI Assistant, Model, Toolset, Conversation Page |
| `src/api/` | Backend API Wrapper. 공통 HTTP Client와 인증 처리를 사용합니다. |
| `src/router/` | Page Route 및 Navigation 등록 |
| `src/utils/` | 공통 Request, Format, Application Navigation Logic |

## FinOps 프런트엔드 규칙

- 비용 Dashboard는 기본적으로 현재 월을 포함한 최근 6개 Calendar Month를 표시합니다. Trend의 월은 연속되어야 하며 Billing이 없는 월은 `0`으로 표시합니다.
- 현재 월 비용에는 `오늘까지` 또는 `is_partial` 상태를 명시하여 마감된 월과 혼동되지 않게 합니다.
- 비용 분석은 하나의 `YYYY-MM` Billing Month를 기준으로 조회합니다. Resource 비용 분석은 Cloud Account와 Date Range를 먼저 선택해야 하며 Region과 Resource Type Filter는 Resource 상세 영역에 둡니다.
- 대용량 Billing으로 인한 Page 및 Request Timeout을 방지하기 위해 일별 Billing 상세 Table을 비용 분석 Page에 직접 표시하지 않습니다.
- 최적화 권고 이름은 Cloud Account, 분석 Billing Month, Strategy를 식별할 수 있어야 합니다. AI Strategy와 기본 Strategy는 동일한 Trace 범위를 표시합니다.

프런트엔드는 Backend에만 데이터를 요청합니다. **브라우저에서 Cloud Provider API를 직접 호출하지 않으며 AI Conversation에서 Billing 동기화를 실행하지 않습니다.** Cloud 비용 데이터는 Backend의 Billing 동기화 과정에서 먼저 저장된 뒤 Dashboard, 비용 분석, 권고, AI Tool에서 조회합니다.

## AI Assistant Page

- Toolset Page는 Tool Permission, 활성 상태, Human Confirmation 필요 여부를 표시합니다.
- Cloud 비용 분석 Tool은 로컬에 동기화된 Billing만 Source로 사용하며 Trend, Product, Region, Resource 집계를 조회할 수 있습니다.
- Conversation 결과에는 Markdown이 포함될 수 있습니다. Page는 이를 읽기 쉬운 본문으로 Render하고 Model의 Tool Protocol 원문을 사용자에게 직접 노출하지 않아야 합니다.

## 개발 검증

현재 프로젝트에는 별도의 lint 또는 TypeScript Script가 구성되어 있지 않습니다. Commit 전 최소 한 번 Build를 실행합니다.

```powershell
cd web
npm run build
```

## 관련 문서

- [Architecture 문서 Index](../docs/architecture/README.md)
- [FinOps 및 AI Data Flow](../docs/architecture/finops-ai-data-flow.md)
