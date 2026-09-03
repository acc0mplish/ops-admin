# Architecture 문서

이 디렉터리는 Ops Admin의 System Boundary, 핵심 Data Flow, 검증 가능한 Architecture Diagram Asset을 관리합니다. 문서는 현재 Code 구현을 기준으로 작성합니다. 요구사항, 계획, 미구현 항목은 `docs/`의 별도 문서에 두고 이미 제공되는 기능처럼 표현하지 않습니다.

## 문서 Index

| 문서 | 용도 |
| --- | --- |
| [FinOps 및 AI Data Flow](finops-ai-data-flow.md) | Cloud Billing 저장, 비용 Consumer, AI Query, Permission Boundary |
| [FinOps Data Flow Diagram (HTML)](finops-ai-data-flow.html) | Offline으로 열 수 있고 Theme 및 Export를 지원하는 Diagram |
| [FinOps Data Flow Source](finops-ai-data-flow.dataflow.json) | Archify JSON Source. Diagram 변경 시 이 파일을 수정합니다. |
| `ops-admin-platform.html` | 기존 Platform Overview Diagram |
| `ops-admin-platform.architecture.json` | 기존 Platform Overview Diagram Source |

## System Layer

| Layer | 구현 위치 | 책임 |
| --- | --- | --- |
| Console | `web/` | Vue Page, Navigation, Input Validation, API 호출, 결과 표시 |
| API 및 Domain Service | `backend/router`, `controller`, `service` | 인증·인가, Protocol 처리, Business Rule, 외부 System 접근 |
| Persistence | `backend/model`, `store`, MySQL | Asset, Configuration, History, Billing, AI Conversation 저장 |
| External Capability | Cloud Provider, Monitoring Datasource, Kubernetes, Git, LDAP, Notification | Backend에서만 통제된 방식으로 접근 |

## 유지보수 규칙

1. Diagram을 변경할 때는 해당 `*.json` Source를 먼저 수정한 뒤 Archify로 HTML을 Render하고 Validation을 실행합니다. 생성된 SVG를 직접 수정하지 않습니다.
2. 새로운 Cross-domain 기능을 추가할 때 Data Source, Persistence 위치, 호출 방향, Permission 요구사항을 이 디렉터리에 기록합니다.
3. Cloud-side Read 작업은 Diagram과 문서에 호출 Entry Point를 명시합니다. FinOps Cloud Billing API는 Cloud Account Test와 Billing Sync에서만 호출할 수 있습니다.
4. AI Tool은 Read-only인지 Human Confirmation이 필요한지 명시해야 합니다. Model이 생성한 Protocol Text를 사용자용 결과로 직접 노출하지 않습니다.

## Diagram 생성

다음 명령은 Archify skill이 설치된 환경에서 실행합니다.

```powershell
$archify = 'C:\Users\Administrator\.codex\skills\archify'
node "$archify\bin\archify.mjs" render dataflow finops-ai-data-flow.dataflow.json finops-ai-data-flow.html
node "$archify\bin\archify.mjs" validate dataflow finops-ai-data-flow.dataflow.json --json
```
