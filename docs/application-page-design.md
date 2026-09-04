# OPS Admin 페이지별 UI 설계 구현 문서

> 범위: 직접 접근 가능한 실제 사용자 Route 90개. Redirect와 호환 Alias는 별도 장으로 다루지 않고, 동일 Component가 서로 다른 실제 Route에서 서로 다른 역할을 하면 각각 한 장으로 다룹니다. 본 문서는 설계 구현 방안만 정의하며 Vue, API, 상호작용 로직을 수정하지 않습니다.

## 사용 방법

각 장은 독립 구현 단위입니다. 브라우저에서 해당 Route를 열어 스크린샷을 기록한 뒤 이 장에 따라 Page를 조정하고, 마지막에 정상/로딩/Empty/Error·권한/리스크 작업을 각각 검수합니다. 수치 규범은 이 장을 기준으로 합니다.

## UI/UX Pro Max 설계 결정(이번 회차 보강)

이번 회차는 UI/UX Pro Max의 “Real-Time / Operations, Data 집약, 저동적” 설계 System을 기준선으로 삼습니다. 모든 Page를 동일한 KPI 보드로 만드는 것이 아니라 List, Canvas, 실행기, 상세, Log, Dashboard가 각자의 작업 Task에 맞게 동작해야 합니다. 아래 개념도는 구현 참조도이며 신규 기능이나 기존 상호작용을 대체하는 설계안이 아닙니다.

### 전역 시각 Contract

| 계층 | 규범 | 사용 경계 |
| --- | --- | --- |
| 워크벤치 바탕색 | `#F6F8FB`; Content Surface는 `#FFFFFF` | 여백과 1px `#E3E8F0` 테두리로 계층을 나누고 Card에 두꺼운 그림자를 쌓지 않습니다. |
| Primary·Action 색 | Health/연결 `#0F766E`; 실행 가능한 주 Action `#0369A1` | 동일 View에는 Solid 주 Action 하나만 두고 Blue는 위험 작업에 쓰지 않습니다. |
| 리스크 상태 | 성공 `#15803D`, 주의 `#B45309`, 실패 `#B91C1C`, 미확인 `#64748B` | 색은 반드시 글자, Icon 또는 상태 단어와 함께 나타내야 하며 색 블록만으로 의미를 전달하지 않습니다. |
| 서체와 Data | 제목 24/30, Block 제목 16/24, 본문 14/22, 보조 12/18; 숫자/시각은 고정폭 숫자 사용 | 한국어 우선 System Sans-serif; Log, Command, ID, Resource량은 `ui-monospace` 사용. |
| 밀도와 리듬 | Page 24px, Module 16px, Control/행 내 8px; 테이블 행 높이 44px, Toolbar 40px | 1366px 첫 화면에서 제목, 범위, 주 Action, 첫 구간 핵심 Content를 동시에 보여야 합니다. |
| 모션과 상태 | hover/focus 150–220ms; Drawer/Dialog 220–280ms; 300ms 초과 로딩은 Skeleton Screen 사용 | opacity/transform만 사용하고 `prefers-reduced-motion`을 존중하며 실시간 차트는 일시정지 진입점을 제공합니다. |

### 개념도 A: 범용 OPS Page 골격

```text
┌ Sidebar / Application 전환 ┐ ┌──────────────────────────────────────────────────────────────┐
│ 현재 Application            │ │ Breadcrumb / Environment 범위 / 마지막 새로고침                 [새로고침] [주 Action] │
│ Group 메뉴            │ ├──────────────────────────────────────────────────────────────┤
│ · 현재 항목 강조        │ │ Page 제목                                      간단한 Task 설명 │
│ · 이상 건수 Badge    │ ├──────────────────────────────────────────────────────────────┤
│                     │ │ 필터 / 검색 / 선택된 범위     [View 저장] [내보내기]                 │
│                     │ ├──────────────────────────────────────────────────────────────┤
│                     │ │ 주 작업 영역: 이 Page 전용 Content(테이블 / Canvas / 실행기 / 타임라인)      │
│                     │ └──────────────────────────────────────────────────────────────┘
└─────────────────────┘
```

### 개념도 B: Resource와 구성 List(관리형 Page)

```text
┌ Host 관리 ──────────────────────────────────────────────── [자산 동기화] [Host 추가] ┐
│  214대 · 이상 8대 · 마지막 동기화 14:32       검색 IP / 이름  상태⌄  Environment⌄  [필터] │
├────────────────────────────────────────────────────────────────────────────────┤
│ 이름 / IP                  Environment      연결성      인증       CPU      마지막 확인  ⋯ │
│ prod-api-01 / 10.0.1.12    PROD      ● 연결됨    ● 유효      72%      14:31   │
│ prod-api-02 / 10.0.1.13    PROD      ▲ 시간 초과      ! 무효      —        14:27   │
├────────────────────────────────────────────────────────────────────────────────┤
│ 2대 선택                                      [일괄 작업⌄]        ‹ 1 2 3 › │
└────────────────────────────────────────────────────────────────────────────────┘
```

적용 원칙: 이상 행은 왼쪽 3px 상태 띠 + 상태 단어로 표현하고, 행 내에는 “보기”와 하나의 고빈도 동작만 남기며 저빈도 동작은 `⋯`에 넣습니다. 모바일에서는 이름, 상태, 마지막 확인을 유지하고 나머지 열은 가로 스크롤 또는 상세 Drawer로 넘깁니다.

### 개념도 C: 토폴로지와 관계 Canvas(탐색형 Page)

```text
┌ 비즈니스 토폴로지 ─ Environment: Production ⌄ ─────────────────────── [새로고침] [Canvas 맞춤] [범례] ┐
│ 필터: Health / Alert / 미편입              확대/축소 −  100%  +                       │
├───────────────┬─────────────────────────────────────────────────────────────┤
│ Service 목록      │   ┌──────────┐       ┌──────────┐       ┌──────────┐         │
│ ● api-gw      │   │ Gateway  │──────▶│ API      │──────▶│ MySQL    │         │
│ ▲ order       │   │ healthy  │       │ warning  │       │ healthy  │         │
│ ! payment     │   └──────────┘       └──────────┘       └──────────┘         │
│               │   Node 클릭: 오른쪽 상세 Drawer; Alert 연결은 선 스타일 + 마커 사용, 색상만 바꾸지 않음      │
└───────────────┴─────────────────────────────────────────────────────────────┘
```

### 개념도 D: 상세 + 오른쪽 Context Drawer(진단형 Page)

```text
┌ Service 상세 / order-api ───────────────────────────────────────────────────────┐
│ 개요  Metric  Event  Log                               [진단] [더보기⌄]          │
├─────────────────────────────────────┬────────────────────────────────────────┤
│ Health 요약 / 핵심 Metric / 추이 차트         │ Node 상세 Drawer(720px, 제목과 작업 고정)  │
│ ┌───────────────┐ ┌───────────────┐ │ 이름, 상태, Environment, 소유자               │
│ │ 오류율  2.1%  │ │ P95  380 ms   │ │ ───────────────────────────────────── │
│ └───────────────┘ └───────────────┘ │ 최근 Event / 원본 Response / 감사 진입점         │
│ 추이 차트: 이상 지점에 형상 마커 + 텍스트 주석 │ [닫기]                 [전체 Log 보기]  │
└─────────────────────────────────────┴────────────────────────────────────────┘
```

### 개념도 E: 표준 운영 실행기(고위험 Workflow)

```text
┌ Command 실행 ─────────────────────────────────────────────────── [실행Command] ┐
│ 1 대상 범위        2 실행 내용                 3 결과 / 감사               │
├───────────────────┬────────────────────────────────────────────────────┤
│ Host Group / Label      │ Command 편집기(고정폭; 구문 영역과 도움말 혼배열 없음)             │
│ 12대 선택         │ ─────────────────────────────────────────────────  │
│ 리스크: ProductionEnvironment     │ 실행 Parameter / 시간 초과 / 동시성                              │
│ [대상 미리보기]         │ [Template 저장]                         [실행 전 확인]      │
├───────────────────┴────────────────────────────────────────────────────┤
│ 실행 중: 12 / 12  ·  성공 10  · 실패 1  · 진행 1         [새로고침 일시정지] [상세] │
└─────────────────────────────────────────────────────────────────────────┘
```

실행 확인 Dialog에는 “대상 범위, 되돌릴 수 없는 영향, 실행자, 시작 시각”을 명시해야 하며, 위험 확인 버튼은 빨간색을 사용하고 기본 포커스로 쓰지 않습니다. 완료 후에는 결과, 실패 재시도, 감사 링크를 제공합니다.

### 개념도 F: 관측 가능성 시계열과 Alert(모니터링형 Page)

```text
┌ 모니터링 개요 ─ Production / 최근 1시간 ⌄ ───────────────── [자동 새로고침 15s] [일시정지] ┐
│  정상 126   경고 7   심각 2      조회 조건 / Service / Cluster / Label           │
├──────────────────────────────────────────────────────────────────────────┤
│  현재 오류율 2.1%   ───╮     ▲ 14:12 timeout                              │
│  ─────────────────────╰─╲___╱────────────────────────                    │
│  범례: 실선 요청량 / 점선 오류율; 각 이상에 텍스트 주석과 Event 목록 링크 첨부        │
├──────────────────────────────────────────────────────────────────────────┤
│  활성 Alert: 심각 우선 → Alert 요약 → 권장 처리 → 소속 Service → 확인 / 차단        │
└──────────────────────────────────────────────────────────────────────────┘
```

### 개념도 G: Project, Pipeline과 Build(전달형 Page)

```text
┌ Project / payment-service ─────────────────────────────── [새 Pipeline] ┐
│ Repository main  · Production  · 담당자 SRE Team     최근 성공 #482 / 14:31        │
├──────────────────────────────────────────────────────────────────────┤
│ ┌ Source Checkout ✓ ┐ → ┌ Image Build ✓ ┐ → ┌ 보안 스캔 ▲ ┐ → ┌ Deploy ○ ┐      │
│ └ 1m 12s ────┘   └ 2m 40s ────┘   └ 43s ───────┘   └ 확인 대기 ─┘      │
├──────────────────────────────────────────────────────────────────────┤
│ 최근 Build: 번호 / Branch / Committer / 소요 시간 / 결과 / 실패 Stage / [Log 보기]   │
└──────────────────────────────────────────────────────────────────────┘
```

### Page–개념도 매핑(실제 Page 90개)

각 장은 기존 “페이지 전용 설계”를 유지합니다. 아래 표는 구현 시 참조할 개념도, 첫 화면 우선순위, 생략할 수 없는 상태 피드백을 명확히 보강해 서로 다른 Application의 메뉴 Page가 동일한 Card 템플릿으로 덮이지 않게 합니다.

| Page 장 | 개념도 / Page 역할 | 첫 화면 필수 표시 | 전용 피드백 |
| --- | --- | --- | --- |
| 1 Dashboard, 12 자산 개요, 72 FinOps 비용 보드, 78 모니터링 개요 | A + F, 총괄 | 범위, 이상 건수, 추이, 처리 진입점 진입 | 새로고침 시각, 실시간 새로고침 일시정지, 이상 이동 |
| 2 비즈니스 토폴로지, 27 Service Resource 토폴로지, 56 Application 토폴로지 | C, 관계 탐색 | Environment, 범례, 확대/축소, 핵심 이상 Node | Node 선택, 로딩 Skeleton, 빈 토폴로지 안내 |
| 3 내 정보 | D, 계정 상세 | 신원, Role, 최근 보안 활동 | 수정성공, Session/비밀번호 리스크 확인 |
| 4–11 사용자/Role/메뉴/부서/직무/설정/로그인 로그/작업 로그 | B, 거버넌스와 감사 List | 검색, 필터, 주 Action, 첫 행 Data | 권한 없음, 필터 결과 없음, 내보내기 진행률 |
| 13 터미널 로그인, 41 Pod 터미널 | E, 대화형 터미널 | 대상, 연결 상태, 터미널 영역 | 연결 끊김 재시도, 복사성공, 위험 Command 안내 |
| 14–18 Host/Host Group/Credential/Cloud Account/Database 관리, 21 Data Import, 22 Backup 관리, 23 Gateway 관리 | B, Resource 관리 | Resource 범위, 이상 건수, 필터, 일괄 진입점 | 동기화 진행률, Credential 만료, Import 검증 |
| 19 Database 상세, 20 DBMS 워크벤치, 24 Environment 모델, 25 Service 관리, 26 Service Health 진단, 28–29 Service 상세/Log | D, 진단 상세 | Entity 신원, Health, 핵심 Metric, Context 작업 | 연결 실패, 조회 소요 시간, Log tail 일시정지 |
| 30–40 Cluster/Node/Namespace/Workload/Network/Storage | B + D, Kubernetes Resource 워크벤치 | Cluster Context, Namespace, Resource 상태, 상세 진입점 | Resource 누락, 스크롤 로딩, 리스크 변경 확인 |
| 42 Script Library, 43–45 Command/Script/파일 배포, 46–54 실행 History/예약/Job | E, 고위험 운영 | 대상, 실행 내용, 리스크, 결과 집계 | 실행 중, 부분 성공, 시간 초과, 감사 링크 |
| 55 Project 목록, 57–60 Build/Image/CI/CD | G, 전달 협업 | Project/Environment, 최근 실행, Stage 결과 | Build 중, 실패 Stage, 재실행 확인 |
| 61–64 Message/Channel/Rule/전송 Log | B, Notification 거버넌스 | 트리거 범위, Channel, 활성/비활성 상태, 전송 결과 | 테스트 전송, 실패 원인, Silence/속도 제한 설명 |
| 65–66 내비게이션 관리/Public Navigation | B, 디렉터리 구성 | Group, 가시성, 정렬, 미리보기 진입점 | 게시 알림, 충돌 검증, 빈 내비게이션 안내 |
| 67–71 AI 대화/Session/Model/Knowledge Base/Tool Set | D, 보조 워크벤치 | 현재 Context, Model/Knowledge 출처, 주 작업 영역 | 스트리밍 Response, 인용 누락, 권한/한도 피드백 |
| 73–77 FinOps Cloud Account/분할/최적화/Resource/Billing | B + F, 비용 처리 | 통계 주기, 비용 범위, 이상/권고, 드릴다운 진입점 | 동기화 상태, Data 지연, 권고 채택 확인 |
| 79 스마트 Dashboard, 89 모니터링 Dashboard, 90 점검 Dashboard | F, Dashboard 상황판 | 시각, Environment, 심각 리스크, 새로고침/전체 화면 상태 | 폴백 Data, 자동 로테이션 일시정지, 오프라인 안내 |
| 80 Datasource 관리, 81 즉시 Query, 82 Log 조회, 83 Trace, 84 Trace 상세 | D + F, 관측 검색 | 조회 범위, 시간 창, 결과량, 상세 드릴다운 | 조회 중, 취소, 결과 없음, 샘플링/절단 설명 |
| 85–88 Alert Rule/Event/차단/수렴 | B + F, Alert 처리 | 심각도, 소속, 권장 동작, 현재상태 | 확인/차단 원인, 복구 Event, 일괄리스크확인 |

### 차트, 테이블과 피드백의 통합 검수

- 추이 차트는 시계열에만 사용하고 지점이 4개 미만이면 수치 Card로 전환합니다. 여러 시리즈는 색상, 실선/점선, 읽을 수 있는 범례를 함께 사용하고 이상 지점에 형상 마커와 텍스트 Event를 반드시 붙입니다.
- Resource/Log 테이블의 숫자와 시각은 우측 정렬 및 고정폭 숫자를 사용합니다. 좁은 화면에서 테이블 컨테이너의 가로 스크롤은 허용하되 Page를 넘치게 하지 않습니다.
- Loading은 제목, 범위, Toolbar를 유지하고 Content 영역만 동일 구조의 Skeleton으로 교체합니다. Empty는 범위 변경 또는 Resource 생성의 다음 단계를 반드시 제공하고, Error는 원인과 재시도를 제공합니다. 위험 작업은 되돌릴 수 있으면 Undo를, 아니면 감사 진입점을 제공합니다.
- Dialog는 단일 짧은 Form 또는 되돌릴 수 없는 확인에 사용하고, 현재 Context를 유지하는 상세, Log, YAML, Trace는 Drawer를 사용합니다. Drawer를 열어도 필터와 스크롤 위치를 지우지 않습니다.


# Page 목록과 방안

## 1. Dashboard

- **Application / Route / 구현**: Console · `/dashboard` · `Dashboard.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 해당 비즈니스 담당자.
- 주요 Task: KPI와 리스크 총괄.
- 고빈도 Task: 비즈니스 Page 이동, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: 총괄 KPI, Health 리스크, 최근 Task.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 Console을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / Console
├─ Title + 13px description
└─ Primary: 비즈니스 Page 이동
Workspace Context / Control Bar\n├─ 현재 범위, 새로고침 또는 연결 상태\n└─ 보조 작업\nPrimary Content Area\n├─ 핵심 정보 또는 시각화 작업 영역\n└─ 상세, 결과 또는 보조 정보 영역
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, Console 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| WorkspaceToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| PrimaryWorkspace | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: 비즈니스 Page 이동, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 내용과 상태 설계

- 핵심 Content는 첫 번째 가시 영역에 배치하고, 보조 설명과 원본 Data는 두 번째 계층 또는 접기 영역에 배치합니다.
- 숫자와 시각은 우측 정렬 또는 고정폭 표시하며, 이상 상태를 Health 상태보다 우선 표시합니다.
- 그래프, Log, Canvas, 편집기는 각자 명확한 작업 영역을 차지하며 일반 Form과 혼배열하지 않습니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “비즈니스 Page 이동” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 Console 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(Console)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “상태 총괄”로 정의합니다. 주 작업 영역을 우선하고 Toolbar와 보조 Panel은 접을 수 있게 하며, KPI + 테이블 Page로 바꾸지 않습니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 2. 비즈니스 토폴로지

- **Application / Route / 구현**: Console · `/business-topology` · `BusinessTopology.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 해당 비즈니스 담당자.
- 주요 Task: 비즈니스 Capability와 Resource 의존성 파악.
- 고빈도 Task: Environment 전환, Node 상세 보기, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: Environment, Application Node, Health 상태, 의존성 체인.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 Business, Environment, Service, Health을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / Business, Environment, Service, Health
├─ Title + 13px description
└─ Primary: Environment 전환, Node 상세 보기
Workspace Context / Control Bar\n├─ 현재 범위, 새로고침 또는 연결 상태\n└─ 보조 작업\nPrimary Content Area\n├─ 핵심 정보 또는 시각화 작업 영역\n└─ 상세, 결과 또는 보조 정보 영역
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, Business, Environment, Service, Health 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| WorkspaceToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| PrimaryWorkspace | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: Environment 전환, Node 상세 보기, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 내용과 상태 설계

- 핵심 Content는 첫 번째 가시 영역에 배치하고, 보조 설명과 원본 Data는 두 번째 계층 또는 접기 영역에 배치합니다.
- 숫자와 시각은 우측 정렬 또는 고정폭 표시하며, 이상 상태를 Health 상태보다 우선 표시합니다.
- 그래프, Log, Canvas, 편집기는 각자 명확한 작업 영역을 차지하며 일반 Form과 혼배열하지 않습니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “Environment 전환, Node 상세 보기” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 Business, Environment, Service, Health 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(Business, Environment, Service, Health)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “의존성 탐색”로 정의합니다. 주 작업 영역을 우선하고 Toolbar와 보조 Panel은 접을 수 있게 하며, KPI + 테이블 Page로 바꾸지 않습니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 3. 내 정보

- **Application / Route / 구현**: Console · `/profile` · `Profile.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 해당 비즈니스 담당자.
- 주요 Task: 내 정보 확인 및 갱신.
- 고빈도 Task: 프로필 수정, 비밀번호 변경, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: 아바타, 계정, Role, 최근 보안 정보.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 계정, Role, 최근 로그인을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / 계정, Role, 최근 로그인
├─ Title + 13px description
└─ Primary: 프로필 수정, 비밀번호 변경
Primary Content Area\n├─ 핵심 정보 또는 시각화 작업 영역\n└─ 상세, 결과 또는 보조 정보 영역
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, 계정, Role, 최근 로그인 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| WorkspaceToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| PrimaryWorkspace | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: 프로필 수정, 비밀번호 변경, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 내용과 상태 설계

- 핵심 Content는 첫 번째 가시 영역에 배치하고, 보조 설명과 원본 Data는 두 번째 계층 또는 접기 영역에 배치합니다.
- 숫자와 시각은 우측 정렬 또는 고정폭 표시하며, 이상 상태를 Health 상태보다 우선 표시합니다.
- 그래프, Log, Canvas, 편집기는 각자 명확한 작업 영역을 차지하며 일반 Form과 혼배열하지 않습니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “프로필 수정, 비밀번호 변경” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 계정, Role, 최근 로그인 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(계정, Role, 최근 로그인)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “개인 계정 관리”로 정의합니다. 현재 Task의 주요 정보 구조로 첫 화면을 구성하고 보조 정보는 읽기 경로에 따라 계층화합니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 4. 사용자 관리

- **Application / Route / 구현**: Console · `/system/admin` · `Admin.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 해당 비즈니스 담당자.
- 주요 Task: 계정 조회, 생성, 활성/비활성화, 초기화.
- 고빈도 Task: 사용자 추가, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: 계정, Role, 부서, 상태, 로그인 정보.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 계정, Role, 상태, 감사을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / 계정, Role, 상태, 감사
├─ Title + 13px description
└─ Primary: 사용자 추가
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, 계정, Role, 상태, 감사 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| FilterToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| DataTable | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: 사용자 추가, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 테이블 설계

- 열 순서: 이름/식별자(220px) → 계정, Role, 상태, 감사(160px) → 상태(100px) → 최근 시각/경과(150px) → 작업(140px fixed-right).
- 핵심 열 고정: 이름과 작업. 긴 주소, 표현식, Response, 비고는 생략을 허용하며 Hover로 전체 값을 표시합니다.
- 상태는 Tag를 사용합니다. ID, 주소, Command, 조회, Version, Timestamp는 고정폭 서체를 사용하고, 보조 Meta 정보는 이름 아래 두 번째 줄에서 약화합니다.
- 행 높이 44px, Hover `#F8FAFD`. 이름 클릭으로 상세에 진입하고 체크 후 일괄 작업 바를 표시합니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “사용자 추가” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 계정, Role, 상태, 감사 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(계정, Role, 상태, 감사)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “계정과 권한 관리”로 정의합니다. List 워크벤치 리듬을 유지합니다: 필터, 훑어보기, 행 내 작업, 상세 재확인. 비즈니스 가치 없는 KPI를 쌓지 않습니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 5. Role 관리

- **Application / Route / 구현**: Console · `/system/role` · `Role.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 해당 비즈니스 담당자.
- 주요 Task: Role 범위와 권한 부여 관리.
- 고빈도 Task: Role 추가, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: Role 이름, Data Scope, 멤버 수, 상태.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 Role, Data Scope, 감사을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / Role, Data Scope, 감사
├─ Title + 13px description
└─ Primary: Role 추가
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, Role, Data Scope, 감사 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| FilterToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| DataTable | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: Role 추가, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 테이블 설계

- 열 순서: 이름/식별자(220px) → Role, Data Scope, 감사(160px) → 상태(100px) → 최근 시각/경과(150px) → 작업(140px fixed-right).
- 핵심 열 고정: 이름과 작업. 긴 주소, 표현식, Response, 비고는 생략을 허용하며 Hover로 전체 값을 표시합니다.
- 상태는 Tag를 사용합니다. ID, 주소, Command, 조회, Version, Timestamp는 고정폭 서체를 사용하고, 보조 Meta 정보는 이름 아래 두 번째 줄에서 약화합니다.
- 행 높이 44px, Hover `#F8FAFD`. 이름 클릭으로 상세에 진입하고 체크 후 일괄 작업 바를 표시합니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “Role 추가” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 Role, Data Scope, 감사 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(Role, Data Scope, 감사)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “Role 권한 부여 관리”로 정의합니다. List 워크벤치 리듬을 유지합니다: 필터, 훑어보기, 행 내 작업, 상세 재확인. 비즈니스 가치 없는 KPI를 쌓지 않습니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 6. 메뉴 관리

- **Application / Route / 구현**: Console · `/system/menu` · `Menu.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 해당 비즈니스 담당자.
- 주요 Task: 메뉴 계층과 권한 포인트 관리.
- 고빈도 Task: 메뉴 추가, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: 메뉴 Tree, Route, 유형, 가시성.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 메뉴, 권한, Route을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / 메뉴, 권한, Route
├─ Title + 13px description
└─ Primary: 메뉴 추가
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, 메뉴, 권한, Route 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| FilterToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| DataTable | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: 메뉴 추가, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 테이블 설계

- 열 순서: 이름/식별자(220px) → 메뉴, 권한, Route(160px) → 상태(100px) → 최근 시각/경과(150px) → 작업(140px fixed-right).
- 핵심 열 고정: 이름과 작업. 긴 주소, 표현식, Response, 비고는 생략을 허용하며 Hover로 전체 값을 표시합니다.
- 상태는 Tag를 사용합니다. ID, 주소, Command, 조회, Version, Timestamp는 고정폭 서체를 사용하고, 보조 Meta 정보는 이름 아래 두 번째 줄에서 약화합니다.
- 행 높이 44px, Hover `#F8FAFD`. 이름 클릭으로 상세에 진입하고 체크 후 일괄 작업 바를 표시합니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “메뉴 추가” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 메뉴, 권한, Route 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(메뉴, 권한, Route)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “내비게이션 권한 구성”로 정의합니다. List 워크벤치 리듬을 유지합니다: 필터, 훑어보기, 행 내 작업, 상세 재확인. 비즈니스 가치 없는 KPI를 쌓지 않습니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 7. 부서 관리

- **Application / Route / 구현**: Console · `/system/dept` · `Dept.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 해당 비즈니스 담당자.
- 주요 Task: 부서 계층과 담당자 관리.
- 고빈도 Task: 부서 추가, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: 부서 Tree, 담당자, 정렬, 상태.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 조직, 담당자, 상태을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / 조직, 담당자, 상태
├─ Title + 13px description
└─ Primary: 부서 추가
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, 조직, 담당자, 상태 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| FilterToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| DataTable | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: 부서 추가, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 테이블 설계

- 열 순서: 이름/식별자(220px) → 조직, 담당자, 상태(160px) → 상태(100px) → 최근 시각/경과(150px) → 작업(140px fixed-right).
- 핵심 열 고정: 이름과 작업. 긴 주소, 표현식, Response, 비고는 생략을 허용하며 Hover로 전체 값을 표시합니다.
- 상태는 Tag를 사용합니다. ID, 주소, Command, 조회, Version, Timestamp는 고정폭 서체를 사용하고, 보조 Meta 정보는 이름 아래 두 번째 줄에서 약화합니다.
- 행 높이 44px, Hover `#F8FAFD`. 이름 클릭으로 상세에 진입하고 체크 후 일괄 작업 바를 표시합니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “부서 추가” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 조직, 담당자, 상태 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(조직, 담당자, 상태)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “조직 구조 관리”로 정의합니다. List 워크벤치 리듬을 유지합니다: 필터, 훑어보기, 행 내 작업, 상세 재확인. 비즈니스 가치 없는 KPI를 쌓지 않습니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 8. 직무 관리

- **Application / Route / 구현**: Console · `/system/post` · `Post.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 해당 비즈니스 담당자.
- 주요 Task: 직무 및 상태 관리.
- 고빈도 Task: 직무 추가, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: 직무 Code, 이름, 정렬, 상태.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 직무, 상태, 감사을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / 직무, 상태, 감사
├─ Title + 13px description
└─ Primary: 직무 추가
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, 직무, 상태, 감사 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| FilterToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| DataTable | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: 직무 추가, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 테이블 설계

- 열 순서: 이름/식별자(220px) → 직무, 상태, 감사(160px) → 상태(100px) → 최근 시각/경과(150px) → 작업(140px fixed-right).
- 핵심 열 고정: 이름과 작업. 긴 주소, 표현식, Response, 비고는 생략을 허용하며 Hover로 전체 값을 표시합니다.
- 상태는 Tag를 사용합니다. ID, 주소, Command, 조회, Version, Timestamp는 고정폭 서체를 사용하고, 보조 Meta 정보는 이름 아래 두 번째 줄에서 약화합니다.
- 행 높이 44px, Hover `#F8FAFD`. 이름 클릭으로 상세에 진입하고 체크 후 일괄 작업 바를 표시합니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “직무 추가” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 직무, 상태, 감사 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(직무, 상태, 감사)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “직무 사전 관리”로 정의합니다. List 워크벤치 리듬을 유지합니다: 필터, 훑어보기, 행 내 작업, 상세 재확인. 비즈니스 가치 없는 KPI를 쌓지 않습니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 9. 시스템 설정

- **Application / Route / 구현**: Console · `/system/settings` · `SystemSettings.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 해당 비즈니스 담당자.
- 주요 Task: 브랜드, 로그인, LDAP, 보안 구성 조정.
- 고빈도 Task: 설정 저장, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: 구성 Group, 현재 값, 저장 상태.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 구성 적용, 감사을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / 구성 적용, 감사
├─ Title + 13px description
└─ Primary: 설정 저장
Primary Content Area\n├─ 핵심 정보 또는 시각화 작업 영역\n└─ 상세, 결과 또는 보조 정보 영역
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, 구성 적용, 감사 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| WorkspaceToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| PrimaryWorkspace | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: 설정 저장, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 내용과 상태 설계

- 핵심 Content는 첫 번째 가시 영역에 배치하고, 보조 설명과 원본 Data는 두 번째 계층 또는 접기 영역에 배치합니다.
- 숫자와 시각은 우측 정렬 또는 고정폭 표시하며, 이상 상태를 Health 상태보다 우선 표시합니다.
- 그래프, Log, Canvas, 편집기는 각자 명확한 작업 영역을 차지하며 일반 Form과 혼배열하지 않습니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “설정 저장” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 구성 적용, 감사 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(구성 적용, 감사)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “시스템 구성”로 정의합니다. 현재 Task의 주요 정보 구조로 첫 화면을 구성하고 보조 정보는 읽기 경로에 따라 계층화합니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 10. 로그인 로그

- **Application / Route / 구현**: Console · `/logs/login` · `LoginLog.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 해당 비즈니스 담당자.
- 주요 Task: 로그인 성공/실패 기록 검색.
- 고빈도 Task: 내보내기, 조회, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: 계정, IP, 위치, 결과, 시각.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 계정, IP, 결과, 시각을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / 계정, IP, 결과, 시각
├─ Title + 13px description
└─ Primary: 내보내기, 조회
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, 계정, IP, 결과, 시각 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| FilterToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| DataTable | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: 내보내기, 조회, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 테이블 설계

- 열 순서: 이름/식별자(220px) → 계정, IP, 결과, 시각(160px) → 상태(100px) → 최근 시각/경과(150px) → 작업(140px fixed-right).
- 핵심 열 고정: 이름과 작업. 긴 주소, 표현식, Response, 비고는 생략을 허용하며 Hover로 전체 값을 표시합니다.
- 상태는 Tag를 사용합니다. ID, 주소, Command, 조회, Version, Timestamp는 고정폭 서체를 사용하고, 보조 Meta 정보는 이름 아래 두 번째 줄에서 약화합니다.
- 행 높이 44px, Hover `#F8FAFD`. 이름 클릭으로 상세에 진입하고 체크 후 일괄 작업 바를 표시합니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “내보내기, 조회” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 계정, IP, 결과, 시각 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(계정, IP, 결과, 시각)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “접근 감사”로 정의합니다. List 워크벤치 리듬을 유지합니다: 필터, 훑어보기, 행 내 작업, 상세 재확인. 비즈니스 가치 없는 KPI를 쌓지 않습니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 11. 작업 로그

- **Application / Route / 구현**: Console · `/logs/operation` · `OperationLog.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 해당 비즈니스 담당자.
- 주요 Task: 작업자와 대상 변경 추적.
- 고빈도 Task: 조회, 내보내기, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: 작업자, 모듈, 동작, 결과, 시각.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 작업자, 대상, 결과, 시각을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / 작업자, 대상, 결과, 시각
├─ Title + 13px description
└─ Primary: 조회, 내보내기
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, 작업자, 대상, 결과, 시각 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| FilterToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| DataTable | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: 조회, 내보내기, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 테이블 설계

- 열 순서: 이름/식별자(220px) → 작업자, 대상, 결과, 시각(160px) → 상태(100px) → 최근 시각/경과(150px) → 작업(140px fixed-right).
- 핵심 열 고정: 이름과 작업. 긴 주소, 표현식, Response, 비고는 생략을 허용하며 Hover로 전체 값을 표시합니다.
- 상태는 Tag를 사용합니다. ID, 주소, Command, 조회, Version, Timestamp는 고정폭 서체를 사용하고, 보조 Meta 정보는 이름 아래 두 번째 줄에서 약화합니다.
- 행 높이 44px, Hover `#F8FAFD`. 이름 클릭으로 상세에 진입하고 체크 후 일괄 작업 바를 표시합니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “조회, 내보내기” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 작업자, 대상, 결과, 시각 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(작업자, 대상, 결과, 시각)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “변경 감사”로 정의합니다. List 워크벤치 리듬을 유지합니다: 필터, 훑어보기, 행 내 작업, 상세 재확인. 비즈니스 가치 없는 KPI를 쌓지 않습니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 12. 자산 개요

- **Application / Route / 구현**: 자산 관리 · `/assets/overview` · `AssetOverview.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 해당 비즈니스 담당자.
- 주요 Task: 오프라인, 인증 또는 연결 이상 Resource 파악.
- 고빈도 Task: Resource 관리 이동, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: 온라인 Host, Database, K8s, 자료 완전성.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 Environment, Resource Type, Health, Alert을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / Environment, Resource Type, Health, Alert
├─ Title + 13px description
└─ Primary: Resource 관리 이동
Workspace Context / Control Bar\n├─ 현재 범위, 새로고침 또는 연결 상태\n└─ 보조 작업\nPrimary Content Area\n├─ 핵심 정보 또는 시각화 작업 영역\n└─ 상세, 결과 또는 보조 정보 영역
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, Environment, Resource Type, Health, Alert 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| WorkspaceToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| PrimaryWorkspace | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: Resource 관리 이동, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 내용과 상태 설계

- 핵심 Content는 첫 번째 가시 영역에 배치하고, 보조 설명과 원본 Data는 두 번째 계층 또는 접기 영역에 배치합니다.
- 숫자와 시각은 우측 정렬 또는 고정폭 표시하며, 이상 상태를 Health 상태보다 우선 표시합니다.
- 그래프, Log, Canvas, 편집기는 각자 명확한 작업 영역을 차지하며 일반 Form과 혼배열하지 않습니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “Resource 관리 이동” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 Environment, Resource Type, Health, Alert 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(Environment, Resource Type, Health, Alert)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “Resource Health 총괄”로 정의합니다. 주 작업 영역을 우선하고 Toolbar와 보조 Panel은 접을 수 있게 하며, KPI + 테이블 Page로 바꾸지 않습니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 13. 터미널 로그인

- **Application / Route / 구현**: 자산 관리 · `/assets/terminal` · `Terminal.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 해당 비즈니스 담당자.
- 주요 Task: Host와 Credential을 선택해 Session을 수립.
- 고빈도 Task: 터미널 연결, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: 자산 Tree, Credential, Session 상태, 터미널 출력.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 Host, Credential, Gateway, Session을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / Host, Credential, Gateway, Session
├─ Title + 13px description
└─ Primary: 터미널 연결
Workspace Context / Control Bar\n├─ 현재 범위, 새로고침 또는 연결 상태\n└─ 보조 작업\nPrimary Content Area\n├─ 핵심 정보 또는 시각화 작업 영역\n└─ 상세, 결과 또는 보조 정보 영역
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, Host, Credential, Gateway, Session 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| WorkspaceToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| PrimaryWorkspace | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: 터미널 연결, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 내용과 상태 설계

- 핵심 Content는 첫 번째 가시 영역에 배치하고, 보조 설명과 원본 Data는 두 번째 계층 또는 접기 영역에 배치합니다.
- 숫자와 시각은 우측 정렬 또는 고정폭 표시하며, 이상 상태를 Health 상태보다 우선 표시합니다.
- 그래프, Log, Canvas, 편집기는 각자 명확한 작업 영역을 차지하며 일반 Form과 혼배열하지 않습니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “터미널 연결” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 Host, Credential, Gateway, Session 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(Host, Credential, Gateway, Session)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “SSH 터미널 진입점”로 정의합니다. 주 작업 영역을 우선하고 Toolbar와 보조 Panel은 접을 수 있게 하며, KPI + 테이블 Page로 바꾸지 않습니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 14. Host 관리

- **Application / Route / 구현**: 자산 관리 · `/assets/server/hosts` · `Host.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 해당 비즈니스 담당자.
- 주요 Task: Host를 필터링하고 보기, 연결, 관리를 수행.
- 고빈도 Task: Host 추가, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: Host 이름, IP, Environment, 온라인, CPU/Memory.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 Environment, Host Group, 상태, Metric을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / Environment, Host Group, 상태, Metric
├─ Title + 13px description
└─ Primary: Host 추가
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, Environment, Host Group, 상태, Metric 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| FilterToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| DataTable | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: Host 추가, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 테이블 설계

- 열 순서: 이름/식별자(220px) → Environment, Host Group, 상태, Metric(160px) → 상태(100px) → 최근 시각/경과(150px) → 작업(140px fixed-right).
- 핵심 열 고정: 이름과 작업. 긴 주소, 표현식, Response, 비고는 생략을 허용하며 Hover로 전체 값을 표시합니다.
- 상태는 Tag를 사용합니다. ID, 주소, Command, 조회, Version, Timestamp는 고정폭 서체를 사용하고, 보조 Meta 정보는 이름 아래 두 번째 줄에서 약화합니다.
- 행 높이 44px, Hover `#F8FAFD`. 이름 클릭으로 상세에 진입하고 체크 후 일괄 작업 바를 표시합니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “Host 추가” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 Environment, Host Group, 상태, Metric 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(Environment, Host Group, 상태, Metric)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “Host 리소스 워크벤치”로 정의합니다. List 워크벤치 리듬을 유지합니다: 필터, 훑어보기, 행 내 작업, 상세 재확인. 비즈니스 가치 없는 KPI를 쌓지 않습니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 15. Host Group 관리

- **Application / Route / 구현**: 자산 관리 · `/assets/server/groups` · `HostGroup.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 해당 비즈니스 담당자.
- 주요 Task: 일괄 운영과 권한 부여 경계를 유지.
- 고빈도 Task: Host Group 추가, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: 그룹, 구성원 수, Environment, 설명.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 Environment, 구성원 수, 담당자을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / Environment, 구성원 수, 담당자
├─ Title + 13px description
└─ Primary: Host Group 추가
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, Environment, 구성원 수, 담당자 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| FilterToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| DataTable | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: Host Group 추가, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 테이블 설계

- 열 순서: 이름/식별자(220px) → Environment, 구성원 수, 담당자(160px) → 상태(100px) → 최근 시각/경과(150px) → 작업(140px fixed-right).
- 핵심 열 고정: 이름과 작업. 긴 주소, 표현식, Response, 비고는 생략을 허용하며 Hover로 전체 값을 표시합니다.
- 상태는 Tag를 사용합니다. ID, 주소, Command, 조회, Version, Timestamp는 고정폭 서체를 사용하고, 보조 Meta 정보는 이름 아래 두 번째 줄에서 약화합니다.
- 행 높이 44px, Hover `#F8FAFD`. 이름 클릭으로 상세에 진입하고 체크 후 일괄 작업 바를 표시합니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “Host Group 추가” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 Environment, 구성원 수, 담당자 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(Environment, 구성원 수, 담당자)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “Resource 그룹 관리”로 정의합니다. List 워크벤치 리듬을 유지합니다: 필터, 훑어보기, 행 내 작업, 상세 재확인. 비즈니스 가치 없는 KPI를 쌓지 않습니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 16. Credential 관리

- **Application / Route / 구현**: 자산 관리 · `/assets/server/credentials` · `Credential.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 해당 비즈니스 담당자.
- 주요 Task: Password와 Key Credential을 유지.
- 고빈도 Task: Credential 추가, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: 이름, Type, 연관 범위, 상태, 업데이트 시각.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 Credential Type, 연관 Resource, 감사을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / Credential Type, 연관 Resource, 감사
├─ Title + 13px description
└─ Primary: Credential 추가
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, Credential Type, 연관 Resource, 감사 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| FilterToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| DataTable | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: Credential 추가, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 테이블 설계

- 열 순서: 이름/식별자(220px) → Credential Type, 연관 Resource, 감사(160px) → 상태(100px) → 최근 시각/경과(150px) → 작업(140px fixed-right).
- 핵심 열 고정: 이름과 작업. 긴 주소, 표현식, Response, 비고는 생략을 허용하며 Hover로 전체 값을 표시합니다.
- 상태는 Tag를 사용합니다. ID, 주소, Command, 조회, Version, Timestamp는 고정폭 서체를 사용하고, 보조 Meta 정보는 이름 아래 두 번째 줄에서 약화합니다.
- 행 높이 44px, Hover `#F8FAFD`. 이름 클릭으로 상세에 진입하고 체크 후 일괄 작업 바를 표시합니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “Credential 추가” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 Credential Type, 연관 Resource, 감사 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(Credential Type, 연관 Resource, 감사)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “접근 Credential 관리”로 정의합니다. List 워크벤치 리듬을 유지합니다: 필터, 훑어보기, 행 내 작업, 상세 재확인. 비즈니스 가치 없는 KPI를 쌓지 않습니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 17. Cloud Account 관리

- **Application / Route / 구현**: 자산 관리 · `/assets/server/cloud-accounts` · `CloudAccount.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 해당 비즈니스 담당자.
- 주요 Task: Cloud Account를 유지하고 연결성을 검사.
- 고빈도 Task: Account 추가, 연결 테스트, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: 벤더, 계정, Region, 동기화 상태, 업데이트 시각.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 벤더, Region, 연결 상태을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / 벤더, Region, 연결 상태
├─ Title + 13px description
└─ Primary: Account 추가, 연결 테스트
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, 벤더, Region, 연결 상태 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| FilterToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| DataTable | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: Account 추가, 연결 테스트, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 테이블 설계

- 열 순서: 이름/식별자(220px) → 벤더, Region, 연결 상태(160px) → 상태(100px) → 최근 시각/경과(150px) → 작업(140px fixed-right).
- 핵심 열 고정: 이름과 작업. 긴 주소, 표현식, Response, 비고는 생략을 허용하며 Hover로 전체 값을 표시합니다.
- 상태는 Tag를 사용합니다. ID, 주소, Command, 조회, Version, Timestamp는 고정폭 서체를 사용하고, 보조 Meta 정보는 이름 아래 두 번째 줄에서 약화합니다.
- 행 높이 44px, Hover `#F8FAFD`. 이름 클릭으로 상세에 진입하고 체크 후 일괄 작업 바를 표시합니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “Account 추가, 연결 테스트” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 벤더, Region, 연결 상태 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(벤더, Region, 연결 상태)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “Cloud 접속 구성”로 정의합니다. List 워크벤치 리듬을 유지합니다: 필터, 훑어보기, 행 내 작업, 상세 재확인. 비즈니스 가치 없는 KPI를 쌓지 않습니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 18. Database 관리

- **Application / Route / 구현**: 자산 관리 · `/assets/databases` · `Database.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 해당 비즈니스 담당자.
- 주요 Task: 연결 상태를 파악하고 워크벤치로 진입.
- 고빈도 Task: Database 추가, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: Database, 주소, Type, Environment, 연결 상태.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 Environment, Type, 연결, Backup을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / Environment, Type, 연결, Backup
├─ Title + 13px description
└─ Primary: Database 추가
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, Environment, Type, 연결, Backup 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| FilterToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| DataTable | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: Database 추가, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 테이블 설계

- 열 순서: 이름/식별자(220px) → Environment, Type, 연결, Backup(160px) → 상태(100px) → 최근 시각/경과(150px) → 작업(140px fixed-right).
- 핵심 열 고정: 이름과 작업. 긴 주소, 표현식, Response, 비고는 생략을 허용하며 Hover로 전체 값을 표시합니다.
- 상태는 Tag를 사용합니다. ID, 주소, Command, 조회, Version, Timestamp는 고정폭 서체를 사용하고, 보조 Meta 정보는 이름 아래 두 번째 줄에서 약화합니다.
- 행 높이 44px, Hover `#F8FAFD`. 이름 클릭으로 상세에 진입하고 체크 후 일괄 작업 바를 표시합니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “Database 추가” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 Environment, Type, 연결, Backup 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(Environment, Type, 연결, Backup)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “Database Resource 관리”로 정의합니다. List 워크벤치 리듬을 유지합니다: 필터, 훑어보기, 행 내 작업, 상세 재확인. 비즈니스 가치 없는 KPI를 쌓지 않습니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 19. Database 상세

- **Application / Route / 구현**: 자산 관리 · `/assets/databases/:id/detail` · `AssetDetail.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 해당 비즈니스 담당자.
- 주요 Task: 구성, 연관 정보와 Health 정보를 조회.
- 고빈도 Task: 워크벤치 진입, 수정, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: 연결, Version, Environment, 연관 Service, Event.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 Environment, 연결, Service, Alert을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / Environment, 연결, Service, Alert
├─ Title + 13px description
└─ Primary: 워크벤치 진입, 수정
Workspace Context / Control Bar\n├─ 현재 범위, 새로고침 또는 연결 상태\n└─ 보조 작업\nFilter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, Environment, 연결, Service, Alert 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| FilterToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| DataTable | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: 워크벤치 진입, 수정, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 테이블 설계

- 열 순서: 이름/식별자(220px) → Environment, 연결, Service, Alert(160px) → 상태(100px) → 최근 시각/경과(150px) → 작업(140px fixed-right).
- 핵심 열 고정: 이름과 작업. 긴 주소, 표현식, Response, 비고는 생략을 허용하며 Hover로 전체 값을 표시합니다.
- 상태는 Tag를 사용합니다. ID, 주소, Command, 조회, Version, Timestamp는 고정폭 서체를 사용하고, 보조 Meta 정보는 이름 아래 두 번째 줄에서 약화합니다.
- 행 높이 44px, Hover `#F8FAFD`. 이름 클릭으로 상세에 진입하고 체크 후 일괄 작업 바를 표시합니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “워크벤치 진입, 수정” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 Environment, 연결, Service, Alert 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(Environment, 연결, Service, Alert)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “Database Resource 상세”로 정의합니다. 주 작업 영역을 우선하고 Toolbar와 보조 Panel은 접을 수 있게 하며, KPI + 테이블 Page로 바꾸지 않습니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 20. DBMS 워크벤치

- **Application / Route / 구현**: 자산 관리 · `/assets/databases/:id/workbench` · `DatabaseWorkbench.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 해당 비즈니스 담당자.
- 주요 Task: Query 실행, 구조 탐색 및 안전한 SQL 실행.
- 고빈도 Task: Query 실행, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: 연결 Tree, Schema, 편집기, 결과, 실행 리스크.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 Database, Schema, Session, 리스크, 감사을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / Database, Schema, Session, 리스크, 감사
├─ Title + 13px description
└─ Primary: Query 실행
Workspace Context / Control Bar\n├─ 현재 범위, 새로고침 또는 연결 상태\n└─ 보조 작업\nPrimary Content Area\n├─ 핵심 정보 또는 시각화 작업 영역\n└─ 상세, 결과 또는 보조 정보 영역
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, Database, Schema, Session, 리스크, 감사 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| WorkspaceToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| PrimaryWorkspace | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: Query 실행, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 내용과 상태 설계

- 핵심 Content는 첫 번째 가시 영역에 배치하고, 보조 설명과 원본 Data는 두 번째 계층 또는 접기 영역에 배치합니다.
- 숫자와 시각은 우측 정렬 또는 고정폭 표시하며, 이상 상태를 Health 상태보다 우선 표시합니다.
- 그래프, Log, Canvas, 편집기는 각자 명확한 작업 영역을 차지하며 일반 Form과 혼배열하지 않습니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “Query 실행” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 Database, Schema, Session, 리스크, 감사 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(Database, Schema, Session, 리스크, 감사)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “Database 조작 워크벤치”로 정의합니다. 주 작업 영역을 우선하고 Toolbar와 보조 Panel은 접을 수 있게 하며, KPI + 테이블 Page로 바꾸지 않습니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 21. Data Import

- **Application / Route / 구현**: 자산 관리 · `/assets/databases/import` · `DatabaseImport.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 해당 비즈니스 담당자.
- 주요 Task: Source File과 Target Database를 선택하고 사전 검증 후 Import.
- 고빈도 Task: Import 시작, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: Source File, Target Database, 사전 검증 결과, History.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 Database, Target Table, 사전 검증, Task을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / Database, Target Table, 사전 검증, Task
├─ Title + 13px description
└─ Primary: Import 시작
Primary Content Area\n├─ 핵심 정보 또는 시각화 작업 영역\n└─ 상세, 결과 또는 보조 정보 영역
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, Database, Target Table, 사전 검증, Task 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| WorkspaceToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| PrimaryWorkspace | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: Import 시작, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 내용과 상태 설계

- 핵심 Content는 첫 번째 가시 영역에 배치하고, 보조 설명과 원본 Data는 두 번째 계층 또는 접기 영역에 배치합니다.
- 숫자와 시각은 우측 정렬 또는 고정폭 표시하며, 이상 상태를 Health 상태보다 우선 표시합니다.
- 그래프, Log, Canvas, 편집기는 각자 명확한 작업 영역을 차지하며 일반 Form과 혼배열하지 않습니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “Import 시작” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 Database, Target Table, 사전 검증, Task 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(Database, Target Table, 사전 검증, Task)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “Data Import 워크벤치”로 정의합니다. 현재 Task의 주요 정보 구조로 첫 화면을 구성하고 보조 정보는 읽기 경로에 따라 계층화합니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 22. Backup 관리

- **Application / Route / 구현**: 자산 관리 · `/assets/databases/backups` · `DatabaseBackup.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 해당 비즈니스 담당자.
- 주요 Task: Backup을 조회하고 Backup 생성 또는 복구를 수행.
- 고빈도 Task: Backup 생성, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: Database, Backup 크기, 시각, 상태, 보존 기간.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 Database, Backup, 복구, 리스크을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / Database, Backup, 복구, 리스크
├─ Title + 13px description
└─ Primary: Backup 생성
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, Database, Backup, 복구, 리스크 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| FilterToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| DataTable | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: Backup 생성, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 테이블 설계

- 열 순서: 이름/식별자(220px) → Database, Backup, 복구, 리스크(160px) → 상태(100px) → 최근 시각/경과(150px) → 작업(140px fixed-right).
- 핵심 열 고정: 이름과 작업. 긴 주소, 표현식, Response, 비고는 생략을 허용하며 Hover로 전체 값을 표시합니다.
- 상태는 Tag를 사용합니다. ID, 주소, Command, 조회, Version, Timestamp는 고정폭 서체를 사용하고, 보조 Meta 정보는 이름 아래 두 번째 줄에서 약화합니다.
- 행 높이 44px, Hover `#F8FAFD`. 이름 클릭으로 상세에 진입하고 체크 후 일괄 작업 바를 표시합니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “Backup 생성” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 Database, Backup, 복구, 리스크 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(Database, Backup, 복구, 리스크)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “Backup 및 복구 관리”로 정의합니다. List 워크벤치 리듬을 유지합니다: 필터, 훑어보기, 행 내 작업, 상세 재확인. 비즈니스 가치 없는 KPI를 쌓지 않습니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 23. Gateway 관리

- **Application / Route / 구현**: 자산 관리 · `/assets/gateways` · `Gateway.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 해당 비즈니스 담당자.
- 주요 Task: 점프 Gateway와 연결 상태를 유지.
- 고빈도 Task: Gateway 추가, 연결 테스트, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: Gateway, 주소, 가용성, 연관 Resource, 업데이트 시각.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 Gateway, 연결, 연관 범위을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / Gateway, 연결, 연관 범위
├─ Title + 13px description
└─ Primary: Gateway 추가, 연결 테스트
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, Gateway, 연결, 연관 범위 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| FilterToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| DataTable | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: Gateway 추가, 연결 테스트, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 테이블 설계

- 열 순서: 이름/식별자(220px) → Gateway, 연결, 연관 범위(160px) → 상태(100px) → 최근 시각/경과(150px) → 작업(140px fixed-right).
- 핵심 열 고정: 이름과 작업. 긴 주소, 표현식, Response, 비고는 생략을 허용하며 Hover로 전체 값을 표시합니다.
- 상태는 Tag를 사용합니다. ID, 주소, Command, 조회, Version, Timestamp는 고정폭 서체를 사용하고, 보조 Meta 정보는 이름 아래 두 번째 줄에서 약화합니다.
- 행 높이 44px, Hover `#F8FAFD`. 이름 클릭으로 상세에 진입하고 체크 후 일괄 작업 바를 표시합니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “Gateway 추가, 연결 테스트” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 Gateway, 연결, 연관 범위 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(Gateway, 연결, 연관 범위)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “접근 Gateway 관리”로 정의합니다. List 워크벤치 리듬을 유지합니다: 필터, 훑어보기, 행 내 작업, 상세 재확인. 비즈니스 가치 없는 KPI를 쌓지 않습니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 24. Environment 모델

- **Application / Route / 구현**: 자산 관리 · `/assets/environments` · `OpsEnvironment.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 해당 비즈니스 담당자.
- 주요 Task: Environment와 Resource 소속을 관리.
- 고빈도 Task: Environment 추가, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: Environment 이름, 등급, Region, Resource 수, 상태.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 Environment, Region, Resource 범위을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / Environment, Region, Resource 범위
├─ Title + 13px description
└─ Primary: Environment 추가
Primary Content Area\n├─ 핵심 정보 또는 시각화 작업 영역\n└─ 상세, 결과 또는 보조 정보 영역
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, Environment, Region, Resource 범위 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| WorkspaceToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| PrimaryWorkspace | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: Environment 추가, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 내용과 상태 설계

- 핵심 Content는 첫 번째 가시 영역에 배치하고, 보조 설명과 원본 Data는 두 번째 계층 또는 접기 영역에 배치합니다.
- 숫자와 시각은 우측 정렬 또는 고정폭 표시하며, 이상 상태를 Health 상태보다 우선 표시합니다.
- 그래프, Log, Canvas, 편집기는 각자 명확한 작업 영역을 차지하며 일반 Form과 혼배열하지 않습니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “Environment 추가” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 Environment, Region, Resource 범위 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(Environment, Region, Resource 범위)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “Environment 모델 관리”로 정의합니다. 현재 Task의 주요 정보 구조로 첫 화면을 구성하고 보조 정보는 읽기 경로에 따라 계층화합니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 25. Service 관리

- **Application / Route / 구현**: 컨테이너 관리 · `/containers/services` · `Application.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 해당 비즈니스 담당자.
- 주요 Task: 비즈니스 Service와 Workload를 발견하고 유지.
- 고빈도 Task: Service 생성, 상세 보기, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: Service, Environment, Cluster, Namespace, Health.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 Service, Cluster, Namespace, Health을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / Service, Cluster, Namespace, Health
├─ Title + 13px description
└─ Primary: Service 생성, 상세 보기
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, Service, Cluster, Namespace, Health 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| FilterToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| DataTable | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: Service 생성, 상세 보기, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 테이블 설계

- 열 순서: 이름/식별자(220px) → Service, Cluster, Namespace, Health(160px) → 상태(100px) → 최근 시각/경과(150px) → 작업(140px fixed-right).
- 핵심 열 고정: 이름과 작업. 긴 주소, 표현식, Response, 비고는 생략을 허용하며 Hover로 전체 값을 표시합니다.
- 상태는 Tag를 사용합니다. ID, 주소, Command, 조회, Version, Timestamp는 고정폭 서체를 사용하고, 보조 Meta 정보는 이름 아래 두 번째 줄에서 약화합니다.
- 행 높이 44px, Hover `#F8FAFD`. 이름 클릭으로 상세에 진입하고 체크 후 일괄 작업 바를 표시합니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “Service 생성, 상세 보기” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 Service, Cluster, Namespace, Health 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(Service, Cluster, Namespace, Health)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “Service Resource 디렉터리”로 정의합니다. List 워크벤치 리듬을 유지합니다: 필터, 훑어보기, 행 내 작업, 상세 재확인. 비즈니스 가치 없는 KPI를 쌓지 않습니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 26. Service 상태 진단

- **Application / Route / 구현**: 컨테이너 관리 · `/containers/services/health-diagnosis` · `ServiceHealthDiagnosis.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 해당 비즈니스 담당자.
- 주요 Task: Target을 선택하고 JVM/Container 진단 결과를 관찰합니다.
- 고빈도 Task: 진단 시작, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: Service, Pod, Container, 프로세스, 진단 Output.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 Service, Pod, 프로세스, Health, 진단을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / Service, Pod, 프로세스, Health, 진단
├─ Title + 13px description
└─ Primary: 진단 시작
Workspace Context / Control Bar\n├─ 현재 범위, 새로고침 또는 연결 상태\n└─ 보조 작업\nFilter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, Service, Pod, 프로세스, Health, 진단 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| FilterToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| DataTable | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: 진단 시작, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 테이블 설계

- 열 순서: 이름/식별자(220px) → Service, Pod, 프로세스, Health, 진단(160px) → 상태(100px) → 최근 시각/경과(150px) → 작업(140px fixed-right).
- 핵심 열 고정: 이름과 작업. 긴 주소, 표현식, Response, 비고는 생략을 허용하며 Hover로 전체 값을 표시합니다.
- 상태는 Tag를 사용합니다. ID, 주소, Command, 조회, Version, Timestamp는 고정폭 서체를 사용하고, 보조 Meta 정보는 이름 아래 두 번째 줄에서 약화합니다.
- 행 높이 44px, Hover `#F8FAFD`. 이름 클릭으로 상세에 진입하고 체크 후 일괄 작업 바를 표시합니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “진단 시작” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 Service, Pod, 프로세스, Health, 진단 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(Service, Pod, 프로세스, Health, 진단)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “런타임 진단 워크벤치”로 정의합니다. 메인 작업 영역을 우선하고 Toolbar와 보조 Panel은 접을 수 있습니다. KPI + 테이블 Page로 개조하지 않습니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 27. Service 리소스 토폴로지

- **Application / Route / 구현**: 컨테이너 관리 · `/containers/services/topology` · `ApplicationTopology.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 해당 비즈니스 담당자.
- 주요 Task: Service, Workload와 리소스 관계를 봅니다.
- 고빈도 Task: Node 상세 보기, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: Service Node, Instance, Health, 의존 방향.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 Service, Workload, 의존 관계, Health을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / Service, Workload, 의존 관계, Health
├─ Title + 13px description
└─ Primary: Node 상세 보기
Workspace Context / Control Bar\n├─ 현재 범위, 새로고침 또는 연결 상태\n└─ 보조 작업\nFilter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, Service, Workload, 의존 관계, Health 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| FilterToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| DataTable | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: Node 상세 보기, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 테이블 설계

- 열 순서: 이름/식별자(220px) → Service, Workload, 의존 관계, Health(160px) → 상태(100px) → 최근 시각/경과(150px) → 작업(140px fixed-right).
- 핵심 열 고정: 이름과 작업. 긴 주소, 표현식, Response, 비고는 생략을 허용하며 Hover로 전체 값을 표시합니다.
- 상태는 Tag를 사용합니다. ID, 주소, Command, 조회, Version, Timestamp는 고정폭 서체를 사용하고, 보조 Meta 정보는 이름 아래 두 번째 줄에서 약화합니다.
- 행 높이 44px, Hover `#F8FAFD`. 이름 클릭으로 상세에 진입하고 체크 후 일괄 작업 바를 표시합니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “Node 상세 보기” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 Service, Workload, 의존 관계, Health 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(Service, Workload, 의존 관계, Health)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “Service 의존 토폴로지”로 정의합니다. 메인 작업 영역을 우선하고 Toolbar와 보조 Panel은 접을 수 있습니다. KPI + 테이블 Page로 개조하지 않습니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 28. Service 상세

- **Application / Route / 구현**: 컨테이너 관리 · `/containers/services/workload` · `ServiceWorkloadDetail.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 해당 비즈니스 담당자.
- 주요 Task: Version, Pod, ReplicaSet과 리소스 관계를 봅니다.
- 고빈도 Task: Rollback, Log 보기, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: Workload, Version, Pod, Ready, Event.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 Service, Workload, Version, Pod을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / Service, Workload, Version, Pod
├─ Title + 13px description
└─ Primary: Rollback, Log 보기
Workspace Context / Control Bar\n├─ 현재 범위, 새로고침 또는 연결 상태\n└─ 보조 작업\nFilter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, Service, Workload, Version, Pod 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| FilterToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| DataTable | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: Rollback, Log 보기, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 테이블 설계

- 열 순서: 이름/식별자(220px) → Service, Workload, Version, Pod(160px) → 상태(100px) → 최근 시각/경과(150px) → 작업(140px fixed-right).
- 핵심 열 고정: 이름과 작업. 긴 주소, 표현식, Response, 비고는 생략을 허용하며 Hover로 전체 값을 표시합니다.
- 상태는 Tag를 사용합니다. ID, 주소, Command, 조회, Version, Timestamp는 고정폭 서체를 사용하고, 보조 Meta 정보는 이름 아래 두 번째 줄에서 약화합니다.
- 행 높이 44px, Hover `#F8FAFD`. 이름 클릭으로 상세에 진입하고 체크 후 일괄 작업 바를 표시합니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “Rollback, Log 보기” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 Service, Workload, Version, Pod 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(Service, Workload, Version, Pod)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “Service Workload 상세”로 정의합니다. 메인 작업 영역을 우선하고 Toolbar와 보조 Panel은 접을 수 있습니다. KPI + 테이블 Page로 개조하지 않습니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 29. Service 로그

- **Application / Route / 구현**: 컨테이너 관리 · `/containers/services/logs` · `ServiceWorkloadLogs.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 해당 비즈니스 담당자.
- 주요 Task: Pod/Container별로 실행 Log를 검색합니다.
- 고빈도 Task: 새로고침, Download, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: Service, Pod, Container, Time Range, Log Stream.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 Service, Pod, Container, Time을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / Service, Pod, Container, Time
├─ Title + 13px description
└─ Primary: 새로고침, Download
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, Service, Pod, Container, Time 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| FilterToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| DataTable | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: 새로고침, Download, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 테이블 설계

- 열 순서: 이름/식별자(220px) → Service, Pod, Container, Time(160px) → 상태(100px) → 최근 시각/경과(150px) → 작업(140px fixed-right).
- 핵심 열 고정: 이름과 작업. 긴 주소, 표현식, Response, 비고는 생략을 허용하며 Hover로 전체 값을 표시합니다.
- 상태는 Tag를 사용합니다. ID, 주소, Command, 조회, Version, Timestamp는 고정폭 서체를 사용하고, 보조 Meta 정보는 이름 아래 두 번째 줄에서 약화합니다.
- 행 높이 44px, Hover `#F8FAFD`. 이름 클릭으로 상세에 진입하고 체크 후 일괄 작업 바를 표시합니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “새로고침, Download” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 Service, Pod, Container, Time 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(Service, Pod, Container, Time)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “Service Log 조회”로 정의합니다. List 워크벤치 리듬을 유지합니다: 필터, 훑어보기, 행 내 작업, 상세 재확인. 비즈니스 가치 없는 KPI를 쌓지 않습니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 30. Cluster 관리

- **Application / Route / 구현**: 컨테이너 관리 · `/containers/k8s/clusters` · `K8sClusterManage.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 해당 비즈니스 담당자.
- 주요 Task: Cluster를 유지하고 연결 상태를 점검합니다.
- 고빈도 Task: Cluster 추가, 연결 테스트, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: Cluster, API 주소, Version, Node, 연결 상태.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 Cluster, Version, Node, 연결을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / Cluster, Version, Node, 연결
├─ Title + 13px description
└─ Primary: Cluster 추가, 연결 테스트
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, Cluster, Version, Node, 연결 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| FilterToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| DataTable | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: Cluster 추가, 연결 테스트, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 테이블 설계

- 열 순서: 이름/식별자(220px) → Cluster, Version, Node, 연결(160px) → 상태(100px) → 최근 시각/경과(150px) → 작업(140px fixed-right).
- 핵심 열 고정: 이름과 작업. 긴 주소, 표현식, Response, 비고는 생략을 허용하며 Hover로 전체 값을 표시합니다.
- 상태는 Tag를 사용합니다. ID, 주소, Command, 조회, Version, Timestamp는 고정폭 서체를 사용하고, 보조 Meta 정보는 이름 아래 두 번째 줄에서 약화합니다.
- 행 높이 44px, Hover `#F8FAFD`. 이름 클릭으로 상세에 진입하고 체크 후 일괄 작업 바를 표시합니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “Cluster 추가, 연결 테스트” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 Cluster, Version, Node, 연결 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(Cluster, Version, Node, 연결)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “Cluster 연결 관리”로 정의합니다. List 워크벤치 리듬을 유지합니다: 필터, 훑어보기, 행 내 작업, 상세 재확인. 비즈니스 가치 없는 KPI를 쌓지 않습니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 31. Cluster 상세

- **Application / Route / 구현**: 컨테이너 관리 · `/containers/k8s/clusters/:id/detail` · `AssetDetail.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 해당 비즈니스 담당자.
- 주요 Task: Cluster 속성, Health와 연관 Service를 봅니다.
- 고빈도 Task: Cluster Overview 진입, 수정, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: Cluster, Version, Node, Namespace, Event.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 Cluster, Version, Node, Health을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / Cluster, Version, Node, Health
├─ Title + 13px description
└─ Primary: Cluster Overview 진입, 수정
Workspace Context / Control Bar\n├─ 현재 범위, 새로고침 또는 연결 상태\n└─ 보조 작업\nPrimary Content Area\n├─ 핵심 정보 또는 시각화 작업 영역\n└─ 상세, 결과 또는 보조 정보 영역
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, Cluster, Version, Node, Health 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| WorkspaceToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| PrimaryWorkspace | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: Cluster Overview 진입, 수정, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 내용과 상태 설계

- 핵심 Content는 첫 번째 가시 영역에 배치하고, 보조 설명과 원본 Data는 두 번째 계층 또는 접기 영역에 배치합니다.
- 숫자와 시각은 우측 정렬 또는 고정폭 표시하며, 이상 상태를 Health 상태보다 우선 표시합니다.
- 그래프, Log, Canvas, 편집기는 각자 명확한 작업 영역을 차지하며 일반 Form과 혼배열하지 않습니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “Cluster Overview 진입, 수정” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 Cluster, Version, Node, Health 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(Cluster, Version, Node, Health)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “Cluster 리소스 상세”로 정의합니다. 메인 작업 영역을 우선하고 Toolbar와 보조 Panel은 접을 수 있습니다. KPI + 테이블 Page로 개조하지 않습니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 32. Cluster Overview

- **Application / Route / 구현**: 컨테이너 관리 · `/containers/k8s/overview` · `K8s.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 해당 비즈니스 담당자.
- 주요 Task: Cluster 용량, Node, Workload Health를 빠르게 판단합니다.
- 고빈도 Task: 새로고침, Cluster 전환, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: Cluster, Namespace, Node, Workload, Alert.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 Cluster, Namespace, Health, Event을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / Cluster, Namespace, Health, Event
├─ Title + 13px description
└─ Primary: 새로고침, Cluster 전환
Workspace Context / Control Bar\n├─ 현재 범위, 새로고침 또는 연결 상태\n└─ 보조 작업\nPrimary Content Area\n├─ 핵심 정보 또는 시각화 작업 영역\n└─ 상세, 결과 또는 보조 정보 영역
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, Cluster, Namespace, Health, Event 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| WorkspaceToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| PrimaryWorkspace | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: 새로고침, Cluster 전환, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 내용과 상태 설계

- 핵심 Content는 첫 번째 가시 영역에 배치하고, 보조 설명과 원본 Data는 두 번째 계층 또는 접기 영역에 배치합니다.
- 숫자와 시각은 우측 정렬 또는 고정폭 표시하며, 이상 상태를 Health 상태보다 우선 표시합니다.
- 그래프, Log, Canvas, 편집기는 각자 명확한 작업 영역을 차지하며 일반 Form과 혼배열하지 않습니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “새로고침, Cluster 전환” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 Cluster, Namespace, Health, Event 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(Cluster, Namespace, Health, Event)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “Cluster 실행 Overview”로 정의합니다. 메인 작업 영역을 우선하고 Toolbar와 보조 Panel은 접을 수 있습니다. KPI + 테이블 Page로 개조하지 않습니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 33. Node 관리

- **Application / Route / 구현**: 컨테이너 관리 · `/containers/k8s/nodes` · `K8s.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 해당 비즈니스 담당자.
- 주요 Task: Node를 필터링하고 리소스와 Condition을 봅니다.
- 고빈도 Task: 새로고침, 상세 보기, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: Node, Ready, CPU, Memory, Pod 수, Version.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 Cluster, Node, 리소스, Condition을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / Cluster, Node, 리소스, Condition
├─ Title + 13px description
└─ Primary: 새로고침, 상세 보기
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, Cluster, Node, 리소스, Condition 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| FilterToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| DataTable | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: 새로고침, 상세 보기, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 테이블 설계

- 열 순서: 이름/식별자(220px) → Cluster, Node, 리소스, Condition(160px) → 상태(100px) → 최근 시각/경과(150px) → 작업(140px fixed-right).
- 핵심 열 고정: 이름과 작업. 긴 주소, 표현식, Response, 비고는 생략을 허용하며 Hover로 전체 값을 표시합니다.
- 상태는 Tag를 사용합니다. ID, 주소, Command, 조회, Version, Timestamp는 고정폭 서체를 사용하고, 보조 Meta 정보는 이름 아래 두 번째 줄에서 약화합니다.
- 행 높이 44px, Hover `#F8FAFD`. 이름 클릭으로 상세에 진입하고 체크 후 일괄 작업 바를 표시합니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “새로고침, 상세 보기” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 Cluster, Node, 리소스, Condition 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(Cluster, Node, 리소스, Condition)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “K8s Node 워크벤치”로 정의합니다. List 워크벤치 리듬을 유지합니다: 필터, 훑어보기, 행 내 작업, 상세 재확인. 비즈니스 가치 없는 KPI를 쌓지 않습니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 34. Namespace

- **Application / Route / 구현**: 컨테이너 관리 · `/containers/k8s/namespaces` · `K8s.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 해당 비즈니스 담당자.
- 주요 Task: 리소스 격리 범위와 Quota를 봅니다.
- 고빈도 Task: 생성, 상세 보기, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: Namespace, 상태, 리소스 수, Quota, Age.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 Cluster, Namespace, Quota을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / Cluster, Namespace, Quota
├─ Title + 13px description
└─ Primary: 생성, 상세 보기
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, Cluster, Namespace, Quota 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| FilterToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| DataTable | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: 생성, 상세 보기, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 테이블 설계

- 열 순서: 이름/식별자(220px) → Cluster, Namespace, Quota(160px) → 상태(100px) → 최근 시각/경과(150px) → 작업(140px fixed-right).
- 핵심 열 고정: 이름과 작업. 긴 주소, 표현식, Response, 비고는 생략을 허용하며 Hover로 전체 값을 표시합니다.
- 상태는 Tag를 사용합니다. ID, 주소, Command, 조회, Version, Timestamp는 고정폭 서체를 사용하고, 보조 Meta 정보는 이름 아래 두 번째 줄에서 약화합니다.
- 행 높이 44px, Hover `#F8FAFD`. 이름 클릭으로 상세에 진입하고 체크 후 일괄 작업 바를 표시합니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “생성, 상세 보기” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 Cluster, Namespace, Quota 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(Cluster, Namespace, Quota)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “Namespace 관리”로 정의합니다. List 워크벤치 리듬을 유지합니다: 필터, 훑어보기, 행 내 작업, 상세 재확인. 비즈니스 가치 없는 KPI를 쌓지 않습니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 35. Workload

- **Application / Route / 구현**: 컨테이너 관리 · `/containers/k8s/workloads` · `K8s.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 해당 비즈니스 담당자.
- 주요 Task: Type과 Namespace별로 배포 Health를 관찰합니다.
- 고빈도 Task: 새로고침, Scale 조정, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: 이름, Type, Ready, Image, 재시작, Age.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 Cluster, Namespace, Type, Version을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / Cluster, Namespace, Type, Version
├─ Title + 13px description
└─ Primary: 새로고침, Scale 조정
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, Cluster, Namespace, Type, Version 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| FilterToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| DataTable | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: 새로고침, Scale 조정, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 테이블 설계

- 열 순서: 이름/식별자(220px) → Cluster, Namespace, Type, Version(160px) → 상태(100px) → 최근 시각/경과(150px) → 작업(140px fixed-right).
- 핵심 열 고정: 이름과 작업. 긴 주소, 표현식, Response, 비고는 생략을 허용하며 Hover로 전체 값을 표시합니다.
- 상태는 Tag를 사용합니다. ID, 주소, Command, 조회, Version, Timestamp는 고정폭 서체를 사용하고, 보조 Meta 정보는 이름 아래 두 번째 줄에서 약화합니다.
- 행 높이 44px, Hover `#F8FAFD`. 이름 클릭으로 상세에 진입하고 체크 후 일괄 작업 바를 표시합니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “새로고침, Scale 조정” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 Cluster, Namespace, Type, Version 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(Cluster, Namespace, Type, Version)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “Workload 리소스 워크벤치”로 정의합니다. List 워크벤치 리듬을 유지합니다: 필터, 훑어보기, 행 내 작업, 상세 재확인. 비즈니스 가치 없는 KPI를 쌓지 않습니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 36. Pod 관리

- **Application / Route / 구현**: 컨테이너 관리 · `/containers/k8s/pods` · `K8s.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 해당 비즈니스 담당자.
- 주요 Task: Pod를 필터링하고 Log, Terminal, YAML로 진입합니다.
- 고빈도 Task: 새로고침, 일괄 작업, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: 이름, Ready, 상태, 재시작, Node, IP, Age.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 Cluster, Namespace, Pod, Node, 상태을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / Cluster, Namespace, Pod, Node, 상태
├─ Title + 13px description
└─ Primary: 새로고침, 일괄 작업
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, Cluster, Namespace, Pod, Node, 상태 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| FilterToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| DataTable | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: 새로고침, 일괄 작업, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 테이블 설계

- 열 순서: 이름/식별자(220px) → Cluster, Namespace, Pod, Node, 상태(160px) → 상태(100px) → 최근 시각/경과(150px) → 작업(140px fixed-right).
- 핵심 열 고정: 이름과 작업. 긴 주소, 표현식, Response, 비고는 생략을 허용하며 Hover로 전체 값을 표시합니다.
- 상태는 Tag를 사용합니다. ID, 주소, Command, 조회, Version, Timestamp는 고정폭 서체를 사용하고, 보조 Meta 정보는 이름 아래 두 번째 줄에서 약화합니다.
- 행 높이 44px, Hover `#F8FAFD`. 이름 클릭으로 상세에 진입하고 체크 후 일괄 작업 바를 표시합니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “새로고침, 일괄 작업” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 Cluster, Namespace, Pod, Node, 상태 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(Cluster, Namespace, Pod, Node, 상태)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “Pod 리소스 워크벤치”로 정의합니다. List 워크벤치 리듬을 유지합니다: 필터, 훑어보기, 행 내 작업, 상세 재확인. 비즈니스 가치 없는 KPI를 쌓지 않습니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 37. Service

- **Application / Route / 구현**: 컨테이너 관리 · `/containers/k8s/services` · `K8s.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 해당 비즈니스 담당자.
- 주요 Task: Service, Selector와 Endpoint를 봅니다.
- 고빈도 Task: 새로고침, YAML 보기, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: 이름, Type, Cluster IP, Port, Endpoint, Age.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 Cluster, Namespace, Endpoint을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / Cluster, Namespace, Endpoint
├─ Title + 13px description
└─ Primary: 새로고침, YAML 보기
Primary Content Area\n├─ 핵심 정보 또는 시각화 작업 영역\n└─ 상세, 결과 또는 보조 정보 영역
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, Cluster, Namespace, Endpoint 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| WorkspaceToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| PrimaryWorkspace | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: 새로고침, YAML 보기, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 내용과 상태 설계

- 핵심 Content는 첫 번째 가시 영역에 배치하고, 보조 설명과 원본 Data는 두 번째 계층 또는 접기 영역에 배치합니다.
- 숫자와 시각은 우측 정렬 또는 고정폭 표시하며, 이상 상태를 Health 상태보다 우선 표시합니다.
- 그래프, Log, Canvas, 편집기는 각자 명확한 작업 영역을 차지하며 일반 Form과 혼배열하지 않습니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “새로고침, YAML 보기” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 Cluster, Namespace, Endpoint 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(Cluster, Namespace, Endpoint)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “Service Discovery 관리”로 정의합니다. 현재 Task의 주요 정보 구조로 첫 화면을 구성하고, 보조 정보는 읽기 경로에 따라 계층화합니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 38. Ingress

- **Application / Route / 구현**: 컨테이너 관리 · `/containers/k8s/ingresses` · `K8s.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 해당 비즈니스 담당자.
- 주요 Task: Domain, Rule, Backend와 TLS를 봅니다.
- 고빈도 Task: 새로고침, YAML 보기, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: 이름, Host, Path, Backend, TLS, Age.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 Cluster, Namespace, Domain, Backend을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / Cluster, Namespace, Domain, Backend
├─ Title + 13px description
└─ Primary: 새로고침, YAML 보기
Primary Content Area\n├─ 핵심 정보 또는 시각화 작업 영역\n└─ 상세, 결과 또는 보조 정보 영역
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, Cluster, Namespace, Domain, Backend 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| WorkspaceToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| PrimaryWorkspace | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: 새로고침, YAML 보기, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 내용과 상태 설계

- 핵심 Content는 첫 번째 가시 영역에 배치하고, 보조 설명과 원본 Data는 두 번째 계층 또는 접기 영역에 배치합니다.
- 숫자와 시각은 우측 정렬 또는 고정폭 표시하며, 이상 상태를 Health 상태보다 우선 표시합니다.
- 그래프, Log, Canvas, 편집기는 각자 명확한 작업 영역을 차지하며 일반 Form과 혼배열하지 않습니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “새로고침, YAML 보기” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 Cluster, Namespace, Domain, Backend 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(Cluster, Namespace, Domain, Backend)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “Ingress Route 관리”로 정의합니다. 현재 Task의 주요 정보 구조로 첫 화면을 구성하고, 보조 정보는 읽기 경로에 따라 계층화합니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 39. 고급 네트워크

- **Application / Route / 구현**: 컨테이너 관리 · `/containers/k8s/advanced-network` · `K8s.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 해당 비즈니스 담당자.
- 주요 Task: Network Policy와 확장 Network Resource를 점검합니다.
- 고빈도 Task: 새로고침, YAML 보기, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: Resource 이름, Type, Namespace, 상태, Age.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 Cluster, Namespace, Network Policy을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / Cluster, Namespace, Network Policy
├─ Title + 13px description
└─ Primary: 새로고침, YAML 보기
Primary Content Area\n├─ 핵심 정보 또는 시각화 작업 영역\n└─ 상세, 결과 또는 보조 정보 영역
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, Cluster, Namespace, Network Policy 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| WorkspaceToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| PrimaryWorkspace | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: 새로고침, YAML 보기, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 내용과 상태 설계

- 핵심 Content는 첫 번째 가시 영역에 배치하고, 보조 설명과 원본 Data는 두 번째 계층 또는 접기 영역에 배치합니다.
- 숫자와 시각은 우측 정렬 또는 고정폭 표시하며, 이상 상태를 Health 상태보다 우선 표시합니다.
- 그래프, Log, Canvas, 편집기는 각자 명확한 작업 영역을 차지하며 일반 Form과 혼배열하지 않습니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “새로고침, YAML 보기” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 Cluster, Namespace, Network Policy 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(Cluster, Namespace, Network Policy)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “고급 네트워크 Resource”로 정의합니다. 현재 Task의 주요 정보 구조로 첫 화면을 구성하고, 보조 정보는 읽기 경로에 따라 계층화합니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 40. Config와 Storage

- **Application / Route / 구현**: 컨테이너 관리 · `/containers/k8s/config-storage` · `K8s.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 해당 비즈니스 담당자.
- 주요 Task: ConfigMap, Secret, PVC, StorageClass를 관리합니다.
- 고빈도 Task: Resource 생성, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: 이름, Type, Scope, 용량, 상태, Age.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 Cluster, Namespace, Storage, Config을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / Cluster, Namespace, Storage, Config
├─ Title + 13px description
└─ Primary: Resource 생성
Primary Content Area\n├─ 핵심 정보 또는 시각화 작업 영역\n└─ 상세, 결과 또는 보조 정보 영역
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, Cluster, Namespace, Storage, Config 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| WorkspaceToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| PrimaryWorkspace | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: Resource 생성, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 내용과 상태 설계

- 핵심 Content는 첫 번째 가시 영역에 배치하고, 보조 설명과 원본 Data는 두 번째 계층 또는 접기 영역에 배치합니다.
- 숫자와 시각은 우측 정렬 또는 고정폭 표시하며, 이상 상태를 Health 상태보다 우선 표시합니다.
- 그래프, Log, Canvas, 편집기는 각자 명확한 작업 영역을 차지하며 일반 Form과 혼배열하지 않습니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “Resource 생성” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 Cluster, Namespace, Storage, Config 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(Cluster, Namespace, Storage, Config)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “Config와 Storage Resource”로 정의합니다. 현재 Task의 주요 정보 구조로 첫 화면을 구성하고, 보조 정보는 읽기 경로에 따라 계층화합니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 41. Pod 터미널

- **Application / Route / 구현**: 컨테이너 관리 · `/containers/k8s/pod-terminal/:clusterId/:namespace/:podName` · `K8sPodTerminal.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 해당 비즈니스 담당자.
- 주요 Task: 명확한 Context에서 Container Shell에 연결합니다.
- 고빈도 Task: 연결, Container 전환, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: Cluster, Namespace, Pod, Container, 세션 상태.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 Cluster, Namespace, Pod, Container, 감사을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / Cluster, Namespace, Pod, Container, 감사
├─ Title + 13px description
└─ Primary: 연결, Container 전환
Workspace Context / Control Bar\n├─ 현재 범위, 새로고침 또는 연결 상태\n└─ 보조 작업\nFilter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, Cluster, Namespace, Pod, Container, 감사 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| FilterToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| DataTable | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: 연결, Container 전환, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 테이블 설계

- 열 순서: 이름/식별자(220px) → Cluster, Namespace, Pod, Container, 감사(160px) → 상태(100px) → 최근 시각/경과(150px) → 작업(140px fixed-right).
- 고정 Column: 이름과 작업. 긴 주소, 표현식, Response, 메모는 생략을 허용하며 Hover로 전체 값을 표시합니다.
- 상태는 Tag를 사용합니다. ID, 주소, Command, Query, Version, Timestamp는 고정폭 Font를 사용하고 보조 Meta 정보는 이름 아래 두 번째 줄에서 강조를 낮춥니다.
- 행 높이 44px, Hover `#F8FAFD`. 이름 클릭으로 상세에 진입하고 체크 후 일괄 작업 바를 표시합니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “연결, Container 전환” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 Cluster, Namespace, Pod, Container, 감사 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(Cluster, Namespace, Pod, Container, 감사)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “Pod 상호작용 터미널”로 정의합니다. 주요 작업 영역을 우선하고 Toolbar와 보조 Panel은 접을 수 있습니다. KPI + 테이블 Page로 바꾸지 않습니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 42. Script Library

- **Application / Route / 구현**: 표준 운영 · `/ops/scripts/library` · `OpsScriptLibrary.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 운영 실행 담당자.
- 주요 Task: 표준 Script를 검색하고 수정하고 활성화합니다.
- 고빈도 Task: 새 Script, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: Script 이름, 분류, Parameter, 상태, 업데이트 시각.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 Script, Version, 권한, 감사을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / Script, Version, 권한, 감사
├─ Title + 13px description
└─ Primary: 새 Script
Primary Content Area\n├─ 핵심 정보 또는 시각화 작업 영역\n└─ 상세, 결과 또는 보조 정보 영역
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, Script, Version, 권한, 감사 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| WorkspaceToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| PrimaryWorkspace | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: 새 Script, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 내용과 상태 설계

- 핵심 Content는 첫 번째 가시 영역에 배치하고, 보조 설명과 원본 Data는 두 번째 계층 또는 접기 영역에 배치합니다.
- 숫자와 시각은 우측 정렬 또는 고정폭 표시하며, 이상 상태를 Health 상태보다 우선 표시합니다.
- 그래프, Log, Canvas, 편집기는 각자 명확한 작업 영역을 차지하며 일반 Form과 혼배열하지 않습니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “새 Script” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 Script, Version, 권한, 감사 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(Script, Version, 권한, 감사)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “운영 Script Asset”로 정의합니다. 현재 Task의 주요 정보 구조로 첫 화면을 구성하고, 보조 정보는 읽기 경로에 따라 계층화합니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 43. Command 실행

- **Application / Route / 구현**: 표준 운영 · `/ops/quick-exec/command` · `OpsCommandExecute.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 운영 실행 담당자.
- 주요 Task: 대상을 선택하고 Command를 안전하게 일괄 전송합니다.
- 고빈도 Task: 지금 실행, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: Task 이름, Command, 대상, Concurrency, Timeout, 미리보기.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 Environment, 대상, Command, 리스크, 감사을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / Environment, 대상, Command, 리스크, 감사
├─ Title + 13px description
└─ Primary: 지금 실행
Primary Content Area\n├─ 핵심 정보 또는 시각화 작업 영역\n└─ 상세, 결과 또는 보조 정보 영역
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, Environment, 대상, Command, 리스크, 감사 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| WorkspaceToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| PrimaryWorkspace | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: 지금 실행, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 내용과 상태 설계

- 핵심 Content는 첫 번째 가시 영역에 배치하고, 보조 설명과 원본 Data는 두 번째 계층 또는 접기 영역에 배치합니다.
- 숫자와 시각은 우측 정렬 또는 고정폭 표시하며, 이상 상태를 Health 상태보다 우선 표시합니다.
- 그래프, Log, Canvas, 편집기는 각자 명확한 작업 영역을 차지하며 일반 Form과 혼배열하지 않습니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “지금 실행” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 Environment, 대상, Command, 리스크, 감사 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(Environment, 대상, Command, 리스크, 감사)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “즉시 Command 실행”로 정의합니다. 현재 Task의 주요 정보 구조로 첫 화면을 구성하고, 보조 정보는 읽기 경로에 따라 계층화합니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 44. Script 실행

- **Application / Route / 구현**: 표준 운영 · `/ops/quick-exec/script` · `OpsScriptExecute.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 운영 실행 담당자.
- 주요 Task: Script와 대상을 선택한 뒤 실행합니다.
- 고빈도 Task: 지금 실행, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: Script, Parameter, 대상, Concurrency, Timeout.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 Script, 대상, Parameter, 감사을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / Script, 대상, Parameter, 감사
├─ Title + 13px description
└─ Primary: 지금 실행
Primary Content Area\n├─ 핵심 정보 또는 시각화 작업 영역\n└─ 상세, 결과 또는 보조 정보 영역
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, Script, 대상, Parameter, 감사 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| WorkspaceToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| PrimaryWorkspace | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: 지금 실행, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 내용과 상태 설계

- 핵심 Content는 첫 번째 가시 영역에 배치하고, 보조 설명과 원본 Data는 두 번째 계층 또는 접기 영역에 배치합니다.
- 숫자와 시각은 우측 정렬 또는 고정폭 표시하며, 이상 상태를 Health 상태보다 우선 표시합니다.
- 그래프, Log, Canvas, 편집기는 각자 명확한 작업 영역을 차지하며 일반 Form과 혼배열하지 않습니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “지금 실행” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 Script, 대상, Parameter, 감사 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(Script, 대상, Parameter, 감사)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “표준 Script 실행”로 정의합니다. 현재 Task의 주요 정보 구조로 첫 화면을 구성하고, 보조 정보는 읽기 경로에 따라 계층화합니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 45. File Distribution

- **Application / Route / 구현**: 표준 운영 · `/ops/quick-exec/file-dispatch` · `OpsFileDispatch.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 운영 실행 담당자.
- 주요 Task: Source, Target Path, Host 범위를 선택합니다.
- 고빈도 Task: 지금 배포, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: Source File, Target Path, 대상, 검증, Concurrency.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 File, 대상, 검증, 감사을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / File, 대상, 검증, 감사
├─ Title + 13px description
└─ Primary: 지금 배포
Primary Content Area\n├─ 핵심 정보 또는 시각화 작업 영역\n└─ 상세, 결과 또는 보조 정보 영역
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, File, 대상, 검증, 감사 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| WorkspaceToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| PrimaryWorkspace | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: 지금 배포, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 내용과 상태 설계

- 핵심 Content는 첫 번째 가시 영역에 배치하고, 보조 설명과 원본 Data는 두 번째 계층 또는 접기 영역에 배치합니다.
- 숫자와 시각은 우측 정렬 또는 고정폭 표시하며, 이상 상태를 Health 상태보다 우선 표시합니다.
- 그래프, Log, Canvas, 편집기는 각자 명확한 작업 영역을 차지하며 일반 Form과 혼배열하지 않습니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “지금 배포” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 File, 대상, 검증, 감사 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(File, 대상, 검증, 감사)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “일괄 File Distribution”로 정의합니다. 현재 Task의 주요 정보 구조로 첫 화면을 구성하고, 보조 정보는 읽기 경로에 따라 계층화합니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 46. 실행 History

- **Application / Route / 구현**: 표준 운영 · `/ops/quick-exec/history` · `OpsExecutionHistory.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 운영 실행 담당자.
- 주요 Task: Task 결과와 실패 상세를 검색합니다.
- 고빈도 Task: 상세 보기, 재시도, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: Task, Type, 대상 수, 결과, 시작 시각, 소요 시간.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 Task, 대상, 결과, 소요 시간을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / Task, 대상, 결과, 소요 시간
├─ Title + 13px description
└─ Primary: 상세 보기, 재시도
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, Task, 대상, 결과, 소요 시간 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| FilterToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| DataTable | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: 상세 보기, 재시도, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 테이블 설계

- 열 순서: 이름/식별자(220px) → Task, 대상, 결과, 소요 시간(160px) → 상태(100px) → 최근 시각/경과(150px) → 작업(140px fixed-right).
- 고정 Column: 이름과 작업. 긴 주소, 표현식, Response, 메모는 생략을 허용하며 Hover로 전체 값을 표시합니다.
- 상태는 Tag를 사용합니다. ID, 주소, Command, Query, Version, Timestamp는 고정폭 Font를 사용하고 보조 Meta 정보는 이름 아래 두 번째 줄에서 강조를 낮춥니다.
- 행 높이 44px, Hover `#F8FAFD`. 이름 클릭으로 상세에 진입하고 체크 후 일괄 작업 바를 표시합니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “상세 보기, 재시도” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 Task, 대상, 결과, 소요 시간 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(Task, 대상, 결과, 소요 시간)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “즉시 Task 추적”로 정의합니다. List 워크벤치 리듬을 유지합니다: 필터, 훑어보기, 행 내 작업, 상세 재확인. 비즈니스 가치 없는 KPI를 쌓지 않습니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 47. 예약 Task

- **Application / Route / 구현**: 표준 운영 · `/ops/schedule/tasks` · `OpsScheduleTaskList.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 운영 실행 담당자.
- 주요 Task: Script와 HTTP Task 스케줄링을 관리합니다.
- 고빈도 Task: 새 Task, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: Task, Type, Cron, 상태, 다음 실행, 담당자.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 Environment, Cron, 상태, 담당자을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / Environment, Cron, 상태, 담당자
├─ Title + 13px description
└─ Primary: 새 Task
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, Environment, Cron, 상태, 담당자 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| FilterToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| DataTable | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: 새 Task, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 테이블 설계

- 열 순서: 이름/식별자(220px) → Environment, Cron, 상태, 담당자(160px) → 상태(100px) → 최근 시각/경과(150px) → 작업(140px fixed-right).
- 고정 Column: 이름과 작업. 긴 주소, 표현식, Response, 메모는 생략을 허용하며 Hover로 전체 값을 표시합니다.
- 상태는 Tag를 사용합니다. ID, 주소, Command, Query, Version, Timestamp는 고정폭 Font를 사용하고 보조 Meta 정보는 이름 아래 두 번째 줄에서 강조를 낮춥니다.
- 행 높이 44px, Hover `#F8FAFD`. 이름 클릭으로 상세에 진입하고 체크 후 일괄 작업 바를 표시합니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “새 Task” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 Environment, Cron, 상태, 담당자 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(Environment, Cron, 상태, 담당자)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “Task 스케줄링 관리”로 정의합니다. List 워크벤치 리듬을 유지합니다: 필터, 훑어보기, 행 내 작업, 상세 재확인. 비즈니스 가치 없는 KPI를 쌓지 않습니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 48. Task Log

- **Application / Route / 구현**: 표준 운영 · `/ops/schedule/logs` · `OpsScheduleLog.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 운영 실행 담당자.
- 주요 Task: 스케줄링 결과와 실패 Output을 찾아냅니다.
- 고빈도 Task: 상세 보기, 재시도, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: Task, 트리거 시각, 결과, 소요 시간, Output Summary.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 Task, 트리거, 결과, 소요 시간을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / Task, 트리거, 결과, 소요 시간
├─ Title + 13px description
└─ Primary: 상세 보기, 재시도
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, Task, 트리거, 결과, 소요 시간 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| FilterToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| DataTable | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: 상세 보기, 재시도, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 테이블 설계

- 열 순서: 이름/식별자(220px) → Task, 트리거, 결과, 소요 시간(160px) → 상태(100px) → 최근 시각/경과(150px) → 작업(140px fixed-right).
- 고정 Column: 이름과 작업. 긴 주소, 표현식, Response, 메모는 생략을 허용하며 Hover로 전체 값을 표시합니다.
- 상태는 Tag를 사용합니다. ID, 주소, Command, Query, Version, Timestamp는 고정폭 Font를 사용하고 보조 Meta 정보는 이름 아래 두 번째 줄에서 강조를 낮춥니다.
- 행 높이 44px, Hover `#F8FAFD`. 이름 클릭으로 상세에 진입하고 체크 후 일괄 작업 바를 표시합니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “상세 보기, 재시도” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 Task, 트리거, 결과, 소요 시간 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(Task, 트리거, 결과, 소요 시간)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “스케줄링 실행 추적”로 정의합니다. List 워크벤치 리듬을 유지합니다: 필터, 훑어보기, 행 내 작업, 상세 재확인. 비즈니스 가치 없는 KPI를 쌓지 않습니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 49. Task Template

- **Application / Route / 구현**: 표준 운영 · `/ops/schedule/templates` · `OpsScheduleTemplate.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 운영 실행 담당자.
- 주요 Task: 재사용 가능한 스케줄링 Template을 선택하거나 관리합니다.
- 고빈도 Task: 새 Template, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: Template, Type, Cron, Parameter, 상태.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 Template, Cron, Parameter을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / Template, Cron, Parameter
├─ Title + 13px description
└─ Primary: 새 Template
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, Template, Cron, Parameter 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| FilterToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| DataTable | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: 새 Template, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 테이블 설계

- 열 순서: 이름/식별자(220px) → Template, Cron, Parameter(160px) → 상태(100px) → 최근 시각/경과(150px) → 작업(140px fixed-right).
- 고정 Column: 이름과 작업. 긴 주소, 표현식, Response, 메모는 생략을 허용하며 Hover로 전체 값을 표시합니다.
- 상태는 Tag를 사용합니다. ID, 주소, Command, Query, Version, Timestamp는 고정폭 Font를 사용하고 보조 Meta 정보는 이름 아래 두 번째 줄에서 강조를 낮춥니다.
- 행 높이 44px, Hover `#F8FAFD`. 이름 클릭으로 상세에 진입하고 체크 후 일괄 작업 바를 표시합니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “새 Template” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 Template, Cron, Parameter 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(Template, Cron, Parameter)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “스케줄링 Template Library”로 정의합니다. List 워크벤치 리듬을 유지합니다: 필터, 훑어보기, 행 내 작업, 상세 재확인. 비즈니스 가치 없는 KPI를 쌓지 않습니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 50. Job Orchestration

- **Application / Route / 구현**: 표준 운영 · `/ops/jobs/designer` · `OpsJobDesigner.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 운영 실행 담당자.
- 주요 Task: 실행, 배포, 수동 승인 Node를 조합합니다.
- 고빈도 Task: 저장, 게시, 실행, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: Job 이름, Node, 연결선, 검증, Version.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 Job, Node, Approval, Version, 감사을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / Job, Node, Approval, Version, 감사
├─ Title + 13px description
└─ Primary: 저장, 게시, 실행
Workspace Context / Control Bar\n├─ 현재 범위, 새로고침 또는 연결 상태\n└─ 보조 작업\nPrimary Content Area\n├─ 핵심 정보 또는 시각화 작업 영역\n└─ 상세, 결과 또는 보조 정보 영역
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, Job, Node, Approval, Version, 감사 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| WorkspaceToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| PrimaryWorkspace | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: 저장, 게시, 실행, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 내용과 상태 설계

- 핵심 Content는 첫 번째 가시 영역에 배치하고, 보조 설명과 원본 Data는 두 번째 계층 또는 접기 영역에 배치합니다.
- 숫자와 시각은 우측 정렬 또는 고정폭 표시하며, 이상 상태를 Health 상태보다 우선 표시합니다.
- 그래프, Log, Canvas, 편집기는 각자 명확한 작업 영역을 차지하며 일반 Form과 혼배열하지 않습니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “저장, 게시, 실행” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 Job, Node, Approval, Version, 감사 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(Job, Node, Approval, Version, 감사)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “시각화 Job 설계”로 정의합니다. 주요 작업 영역을 우선하고 Toolbar와 보조 Panel은 접을 수 있습니다. KPI + 테이블 Page로 바꾸지 않습니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 51. Job 목록

- **Application / Route / 구현**: 표준 운영 · `/ops/jobs/list` · `OpsJobList.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 운영 실행 담당자.
- 주요 Task: Job 상태를 관리하고 실행을 시작합니다.
- 고빈도 Task: 새 Job, 실행, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: Job, Version, 상태, 담당자, 업데이트 시각.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 Job, Version, 담당자, 감사을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / Job, Version, 담당자, 감사
├─ Title + 13px description
└─ Primary: 새 Job, 실행
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, Job, Version, 담당자, 감사 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| FilterToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| DataTable | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: 새 Job, 실행, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 테이블 설계

- 열 순서: 이름/식별자(220px) → Job, Version, 담당자, 감사(160px) → 상태(100px) → 최근 시각/경과(150px) → 작업(140px fixed-right).
- 고정 Column: 이름과 작업. 긴 주소, 표현식, Response, 메모는 생략을 허용하며 Hover로 전체 값을 표시합니다.
- 상태는 Tag를 사용합니다. ID, 주소, Command, Query, Version, Timestamp는 고정폭 Font를 사용하고 보조 Meta 정보는 이름 아래 두 번째 줄에서 강조를 낮춥니다.
- 행 높이 44px, Hover `#F8FAFD`. 이름 클릭으로 상세에 진입하고 체크 후 일괄 작업 바를 표시합니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “새 Job, 실행” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 Job, Version, 담당자, 감사 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(Job, Version, 담당자, 감사)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “Job 정의 관리”로 정의합니다. List 워크벤치 리듬을 유지합니다: 필터, 훑어보기, 행 내 작업, 상세 재확인. 비즈니스 가치 없는 KPI를 쌓지 않습니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 52. 수동 승인

- **Application / Route / 구현**: 표준 운영 · `/ops/jobs/approvals` · `OpsJobApprovals.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 운영 실행 담당자.
- 주요 Task: 수동 승인이 필요한 Job Step을 처리합니다.
- 고빈도 Task: 승인, 거부, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: Job, Step, 대상, 대기 시간, 리스크.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 Job, Step, 리스크, 감사을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / Job, Step, 리스크, 감사
├─ Title + 13px description
└─ Primary: 승인, 거부
Primary Content Area\n├─ 핵심 정보 또는 시각화 작업 영역\n└─ 상세, 결과 또는 보조 정보 영역
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, Job, Step, 리스크, 감사 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| WorkspaceToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| PrimaryWorkspace | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: 승인, 거부, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 내용과 상태 설계

- 핵심 Content는 첫 번째 가시 영역에 배치하고, 보조 설명과 원본 Data는 두 번째 계층 또는 접기 영역에 배치합니다.
- 숫자와 시각은 우측 정렬 또는 고정폭 표시하며, 이상 상태를 Health 상태보다 우선 표시합니다.
- 그래프, Log, Canvas, 편집기는 각자 명확한 작업 영역을 차지하며 일반 Form과 혼배열하지 않습니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “승인, 거부” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 Job, Step, 리스크, 감사 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(Job, Step, 리스크, 감사)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “수동 승인 대기 항목”로 정의합니다. 현재 Task의 주요 정보 구조로 첫 화면을 구성하고, 보조 정보는 읽기 경로에 따라 계층화합니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 53. Job History

- **Application / Route / 구현**: 표준 운영 · `/ops/jobs/history` · `OpsJobHistory.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 운영 실행 담당자.
- 주요 Task: Job Step, 결과, Approval 상태를 확인합니다.
- 고빈도 Task: 상세 보기, 재시도, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: Job, Run, Step, 결과, 소요 시간, Approval.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 Job, Step, 결과, Approval을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / Job, Step, 결과, Approval
├─ Title + 13px description
└─ Primary: 상세 보기, 재시도
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, Job, Step, 결과, Approval 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| FilterToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| DataTable | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: 상세 보기, 재시도, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 테이블 설계

- 열 순서: 이름/식별자(220px) → Job, Step, 결과, Approval(160px) → 상태(100px) → 최근 시각/경과(150px) → 작업(140px fixed-right).
- 고정 Column: 이름과 작업. 긴 주소, 표현식, Response, 메모는 생략을 허용하며 Hover로 전체 값을 표시합니다.
- 상태는 Tag를 사용합니다. ID, 주소, Command, Query, Version, Timestamp는 고정폭 Font를 사용하고 보조 Meta 정보는 이름 아래 두 번째 줄에서 강조를 낮춥니다.
- 행 높이 44px, Hover `#F8FAFD`. 이름 클릭으로 상세에 진입하고 체크 후 일괄 작업 바를 표시합니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “상세 보기, 재시도” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 Job, Step, 결과, Approval 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(Job, Step, 결과, Approval)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “Job 실행 추적”로 정의합니다. List 워크벤치 리듬을 유지합니다: 필터, 훑어보기, 행 내 작업, 상세 재확인. 비즈니스 가치 없는 KPI를 쌓지 않습니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 54. Job Template

- **Application / Route / 구현**: 표준 운영 · `/ops/jobs/templates` · `OpsJobTemplate.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 운영 실행 담당자.
- 주요 Task: Import 가능한 Job Template을 관리합니다.
- 고빈도 Task: 새 Template, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: Template, Node 수, Version, 상태, 업데이트 시각.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 Template, Version, Node을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / Template, Version, Node
├─ Title + 13px description
└─ Primary: 새 Template
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, Template, Version, Node 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| FilterToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| DataTable | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: 새 Template, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 테이블 설계

- 열 순서: 이름/식별자(220px) → Template, Version, Node(160px) → 상태(100px) → 최근 시각/경과(150px) → 작업(140px fixed-right).
- 고정 Column: 이름과 작업. 긴 주소, 표현식, Response, 메모는 생략을 허용하며 Hover로 전체 값을 표시합니다.
- 상태는 Tag를 사용합니다. ID, 주소, Command, Query, Version, Timestamp는 고정폭 Font를 사용하고 보조 Meta 정보는 이름 아래 두 번째 줄에서 강조를 낮춥니다.
- 행 높이 44px, Hover `#F8FAFD`. 이름 클릭으로 상세에 진입하고 체크 후 일괄 작업 바를 표시합니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “새 Template” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 Template, Version, Node 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(Template, Version, Node)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “Job Template Library”로 정의합니다. List 워크벤치 리듬을 유지합니다: 필터, 훑어보기, 행 내 작업, 상세 재확인. 비즈니스 가치 없는 KPI를 쌓지 않습니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 55. Project 목록

- **Application / Route / 구현**: 애플리케이션 센터 · `/applications/projects` · `AppProjectList.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 해당 비즈니스 담당자.
- 주요 Task: Project Repository와 Resource Binding을 관리합니다.
- 고빈도 Task: 새 Application, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: Project, Service 유형, Environment, Repository, 상태.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 Application, Environment, Repository, 상태을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / Application, Environment, Repository, 상태
├─ Title + 13px description
└─ Primary: 새 Application
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, Application, Environment, Repository, 상태 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| FilterToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| DataTable | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: 새 Application, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 테이블 설계

- 열 순서: 이름/식별자(220px) → Application, Environment, Repository, 상태(160px) → 상태(100px) → 최근 시각/경과(150px) → 작업(140px fixed-right).
- 고정 Column: 이름과 작업. 긴 주소, 표현식, Response, 메모는 생략을 허용하며 Hover로 전체 값을 표시합니다.
- 상태는 Tag를 사용합니다. ID, 주소, Command, Query, Version, Timestamp는 고정폭 Font를 사용하고 보조 Meta 정보는 이름 아래 두 번째 줄에서 강조를 낮춥니다.
- 행 높이 44px, Hover `#F8FAFD`. 이름 클릭으로 상세에 진입하고 체크 후 일괄 작업 바를 표시합니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “새 Application” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 Application, Environment, Repository, 상태 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(Application, Environment, Repository, 상태)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “Application Asset Directory”로 정의합니다. List 워크벤치 리듬을 유지합니다: 필터, 훑어보기, 행 내 작업, 상세 재확인. 비즈니스 가치 없는 KPI를 쌓지 않습니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 56. Application 토폴로지

- **Application / Route / 구현**: 애플리케이션 센터 · `/applications/topology` · `AppTopology.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 해당 비즈니스 담당자.
- 주요 Task: Application과 Resource, Deploy, Alert 관계를 둘러봅니다.
- 고빈도 Task: Application 전환, 상세 보기, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: Application, Environment, Resource, Deploy, Alert.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 Application, Environment, Resource, Alert을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / Application, Environment, Resource, Alert
├─ Title + 13px description
└─ Primary: Application 전환, 상세 보기
Workspace Context / Control Bar\n├─ 현재 범위, 새로고침 또는 연결 상태\n└─ 보조 작업\nPrimary Content Area\n├─ 핵심 정보 또는 시각화 작업 영역\n└─ 상세, 결과 또는 보조 정보 영역
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, Application, Environment, Resource, Alert 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| WorkspaceToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| PrimaryWorkspace | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: Application 전환, 상세 보기, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 내용과 상태 설계

- 핵심 Content는 첫 번째 가시 영역에 배치하고, 보조 설명과 원본 Data는 두 번째 계층 또는 접기 영역에 배치합니다.
- 숫자와 시각은 우측 정렬 또는 고정폭 표시하며, 이상 상태를 Health 상태보다 우선 표시합니다.
- 그래프, Log, Canvas, 편집기는 각자 명확한 작업 영역을 차지하며 일반 Form과 혼배열하지 않습니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “Application 전환, 상세 보기” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 Application, Environment, Resource, Alert 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(Application, Environment, Resource, Alert)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “Application Delivery 토폴로지”로 정의합니다. 주요 작업 영역을 우선하고 Toolbar와 보조 Panel은 접을 수 있습니다. KPI + 테이블 Page로 바꾸지 않습니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 57. Build Task

- **Application / Route / 구현**: 애플리케이션 센터 · `/applications/build-tasks` · `AppBuildTaskList.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 해당 비즈니스 담당자.
- 주요 Task: Task를 생성하고 Build를 시작합니다.
- 고빈도 Task: 새 Build Task, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: Task, Project, Branch, Build 방식, 상태.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 Project, Branch, Image, 상태을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / Project, Branch, Image, 상태
├─ Title + 13px description
└─ Primary: 새 Build Task
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, Project, Branch, Image, 상태 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| FilterToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| DataTable | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: 새 Build Task, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 테이블 설계

- 열 순서: 이름/식별자(220px) → Project, Branch, Image, 상태(160px) → 상태(100px) → 최근 시각/경과(150px) → 작업(140px fixed-right).
- 고정 Column: 이름과 작업. 긴 주소, 표현식, Response, 메모는 생략을 허용하며 Hover로 전체 값을 표시합니다.
- 상태는 Tag를 사용합니다. ID, 주소, Command, Query, Version, Timestamp는 고정폭 Font를 사용하고 보조 Meta 정보는 이름 아래 두 번째 줄에서 강조를 낮춥니다.
- 행 높이 44px, Hover `#F8FAFD`. 이름 클릭으로 상세에 진입하고 체크 후 일괄 작업 바를 표시합니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “새 Build Task” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 Project, Branch, Image, 상태 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(Project, Branch, Image, 상태)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “Build Task 관리”로 정의합니다. List 워크벤치 리듬을 유지합니다: 필터, 훑어보기, 행 내 작업, 상세 재확인. 비즈니스 가치 없는 KPI를 쌓지 않습니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 58. Build History

- **Application / Route / 구현**: 애플리케이션 센터 · `/applications/build-history` · `AppBuildHistory.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 해당 비즈니스 담당자.
- 주요 Task: Stage, Log, Artifact를 확인합니다.
- 고빈도 Task: 상세 보기, 재시도, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: Build 번호, Project, Branch, Stage, 결과, 소요 시간.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 Project, Branch, Stage, Artifact을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / Project, Branch, Stage, Artifact
├─ Title + 13px description
└─ Primary: 상세 보기, 재시도
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, Project, Branch, Stage, Artifact 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| FilterToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| DataTable | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: 상세 보기, 재시도, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 테이블 설계

- 열 순서: 이름/식별자(220px) → Project, Branch, Stage, Artifact(160px) → 상태(100px) → 최근 시각/경과(150px) → 작업(140px fixed-right).
- 고정 Column: 이름과 작업. 긴 주소, 표현식, Response, 메모는 생략을 허용하며 Hover로 전체 값을 표시합니다.
- 상태는 Tag를 사용합니다. ID, 주소, Command, Query, Version, Timestamp는 고정폭 Font를 사용하고 보조 Meta 정보는 이름 아래 두 번째 줄에서 강조를 낮춥니다.
- 행 높이 44px, Hover `#F8FAFD`. 이름 클릭으로 상세에 진입하고 체크 후 일괄 작업 바를 표시합니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “상세 보기, 재시도” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 Project, Branch, Stage, Artifact 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(Project, Branch, Stage, Artifact)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “Build 실행 추적”로 정의합니다. List 워크벤치 리듬을 유지합니다: 필터, 훑어보기, 행 내 작업, 상세 재확인. 비즈니스 가치 없는 KPI를 쌓지 않습니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 59. Image Registry

- **Application / Route / 구현**: 애플리케이션 센터 · `/applications/image-registries` · `AppImageRegistry.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 해당 비즈니스 담당자.
- 주요 Task: Image Registry 연동과 상태를 관리합니다.
- 고빈도 Task: Registry 추가, 연결 테스트, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: Registry 이름, 주소, Type, 연결 상태, 업데이트 시각.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 Registry, 연결, Credential을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / Registry, 연결, Credential
├─ Title + 13px description
└─ Primary: Registry 추가, 연결 테스트
Primary Content Area\n├─ 핵심 정보 또는 시각화 작업 영역\n└─ 상세, 결과 또는 보조 정보 영역
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, Registry, 연결, Credential 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| WorkspaceToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| PrimaryWorkspace | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: Registry 추가, 연결 테스트, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 내용과 상태 설계

- 핵심 Content는 첫 번째 가시 영역에 배치하고, 보조 설명과 원본 Data는 두 번째 계층 또는 접기 영역에 배치합니다.
- 숫자와 시각은 우측 정렬 또는 고정폭 표시하며, 이상 상태를 Health 상태보다 우선 표시합니다.
- 그래프, Log, Canvas, 편집기는 각자 명확한 작업 영역을 차지하며 일반 Form과 혼배열하지 않습니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “Registry 추가, 연결 테스트” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 Registry, 연결, Credential 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(Registry, 연결, Credential)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “Image Registry 관리”로 정의합니다. 현재 Task의 주요 정보 구조로 첫 화면을 구성하고, 보조 정보는 읽기 경로에 따라 계층화합니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 60. CI/CD Pipeline

- **Application / Route / 구현**: 애플리케이션 센터 · `/applications/pipelines` · `AppPipelineCenter.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 해당 비즈니스 담당자.
- 주요 Task: Template을 선택하고 Stage를 구성하며 Run을 추적합니다.
- 고빈도 Task: 새 Pipeline, 실행, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: Pipeline, Stage, Environment, 실행 상태, Log.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 Application, Environment, Stage, Deploy, 감사을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / Application, Environment, Stage, Deploy, 감사
├─ Title + 13px description
└─ Primary: 새 Pipeline, 실행
Primary Content Area\n├─ 핵심 정보 또는 시각화 작업 영역\n└─ 상세, 결과 또는 보조 정보 영역
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, Application, Environment, Stage, Deploy, 감사 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| WorkspaceToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| PrimaryWorkspace | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: 새 Pipeline, 실행, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 내용과 상태 설계

- 핵심 Content는 첫 번째 가시 영역에 배치하고, 보조 설명과 원본 Data는 두 번째 계층 또는 접기 영역에 배치합니다.
- 숫자와 시각은 우측 정렬 또는 고정폭 표시하며, 이상 상태를 Health 상태보다 우선 표시합니다.
- 그래프, Log, Canvas, 편집기는 각자 명확한 작업 영역을 차지하며 일반 Form과 혼배열하지 않습니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “새 Pipeline, 실행” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 Application, Environment, Stage, Deploy, 감사 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(Application, Environment, Stage, Deploy, 감사)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “Delivery Pipeline 워크벤치”로 정의합니다. 현재 Task의 주요 정보 구조로 첫 화면을 구성하고, 보조 정보는 읽기 경로에 따라 계층화합니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 61. Message Template

- **Application / Route / 구현**: 메시지 통지 · `/notify/templates` · `NotifyTemplate.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 해당 비즈니스 담당자.
- 주요 Task: 다중 Channel Message Template을 관리하고 실시간 미리보기를 제공합니다.
- 고빈도 Task: 새 Template, 저장, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: Channel, Variable, Template 상태, 미리보기.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 Channel, Variable, Template 상태을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / Channel, Variable, Template 상태
├─ Title + 13px description
└─ Primary: 새 Template, 저장
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, Channel, Variable, Template 상태 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| FilterToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| DataTable | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: 새 Template, 저장, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 테이블 설계

- 열 순서: 이름/식별자(220px) → Channel, Variable, Template 상태(160px) → 상태(100px) → 최근 시각/경과(150px) → 작업(140px fixed-right).
- 고정 Column: 이름과 작업. 긴 주소, 표현식, Response, 메모는 생략을 허용하며 Hover로 전체 값을 표시합니다.
- 상태는 Tag를 사용합니다. ID, 주소, Command, Query, Version, Timestamp는 고정폭 Font를 사용하고 보조 Meta 정보는 이름 아래 두 번째 줄에서 강조를 낮춥니다.
- 행 높이 44px, Hover `#F8FAFD`. 이름 클릭으로 상세에 진입하고 체크 후 일괄 작업 바를 표시합니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “새 Template, 저장” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 Channel, Variable, Template 상태 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(Channel, Variable, Template 상태)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “Notification Content 편집”로 정의합니다. List 워크벤치 리듬을 유지합니다: 필터, 훑어보기, 행 내 작업, 상세 재확인. 비즈니스 가치 없는 KPI를 쌓지 않습니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 62. Notification Channel

- **Application / Route / 구현**: 메시지 통지 · `/notify/channels` · `NotifyChannel.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 해당 비즈니스 담당자.
- 주요 Task: Bot과 Webhook Channel을 관리합니다.
- 고빈도 Task: Channel 추가, 테스트, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: Channel, Type, 주소, 상태, 최근 테스트.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 Channel, 상태, 테스트을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / Channel, 상태, 테스트
├─ Title + 13px description
└─ Primary: Channel 추가, 테스트
Primary Content Area\n├─ 핵심 정보 또는 시각화 작업 영역\n└─ 상세, 결과 또는 보조 정보 영역
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, Channel, 상태, 테스트 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| WorkspaceToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| PrimaryWorkspace | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: Channel 추가, 테스트, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 내용과 상태 설계

- 핵심 Content는 첫 번째 가시 영역에 배치하고, 보조 설명과 원본 Data는 두 번째 계층 또는 접기 영역에 배치합니다.
- 숫자와 시각은 우측 정렬 또는 고정폭 표시하며, 이상 상태를 Health 상태보다 우선 표시합니다.
- 그래프, Log, Canvas, 편집기는 각자 명확한 작업 영역을 차지하며 일반 Form과 혼배열하지 않습니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “Channel 추가, 테스트” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 Channel, 상태, 테스트 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(Channel, 상태, 테스트)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “Notification Channel 관리”로 정의합니다. 현재 Task의 주요 정보 구조로 첫 화면을 구성하고, 보조 정보는 읽기 경로에 따라 계층화합니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 63. Notification Rule

- **Application / Route / 구현**: 메시지 통지 · `/notify/rules` · `NotifyRule.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 해당 비즈니스 담당자.
- 주요 Task: Event, Template, Channel을 Rule로 조합합니다.
- 고빈도 Task: 새 Rule, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: Event, Template, Channel, 활성 상태, 우선순위.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 Event, Channel, 우선순위, 감사을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / Event, Channel, 우선순위, 감사
├─ Title + 13px description
└─ Primary: 새 Rule
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, Event, Channel, 우선순위, 감사 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| FilterToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| DataTable | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: 새 Rule, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 테이블 설계

- 열 순서: 이름/식별자(220px) → Event, Channel, 우선순위, 감사(160px) → 상태(100px) → 최근 시각/경과(150px) → 작업(140px fixed-right).
- 고정 Column: 이름과 작업. 긴 주소, 표현식, Response, 메모는 생략을 허용하며 Hover로 전체 값을 표시합니다.
- 상태는 Tag를 사용합니다. ID, 주소, Command, Query, Version, Timestamp는 고정폭 Font를 사용하고 보조 Meta 정보는 이름 아래 두 번째 줄에서 강조를 낮춥니다.
- 행 높이 44px, Hover `#F8FAFD`. 이름 클릭으로 상세에 진입하고 체크 후 일괄 작업 바를 표시합니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “새 Rule” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 Event, Channel, 우선순위, 감사 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(Event, Channel, 우선순위, 감사)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “Notification Route Orchestration”로 정의합니다. List 워크벤치 리듬을 유지합니다: 필터, 훑어보기, 행 내 작업, 상세 재확인. 비즈니스 가치 없는 KPI를 쌓지 않습니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 64. Send Log

- **Application / Route / 구현**: 메시지 통지 · `/notify/send-logs` · `NotifySendLog.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 해당 비즈니스 담당자.
- 주요 Task: 전송 실패와 Response 이상을 찾아냅니다.
- 고빈도 Task: 상세 보기, 재시도, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: Channel, Event, 결과, Response Code, 소요 시간, 시각.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 Channel, 결과, 소요 시간, Response을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / Channel, 결과, 소요 시간, Response
├─ Title + 13px description
└─ Primary: 상세 보기, 재시도
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, Channel, 결과, 소요 시간, Response 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| FilterToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| DataTable | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: 상세 보기, 재시도, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 테이블 설계

- 열 순서: 이름/식별자(220px) → Channel, 결과, 소요 시간, Response(160px) → 상태(100px) → 최근 시각/경과(150px) → 작업(140px fixed-right).
- 고정 Column: 이름과 작업. 긴 주소, 표현식, Response, 메모는 생략을 허용하며 Hover로 전체 값을 표시합니다.
- 상태는 Tag를 사용합니다. ID, 주소, Command, Query, Version, Timestamp는 고정폭 Font를 사용하고 보조 Meta 정보는 이름 아래 두 번째 줄에서 강조를 낮춥니다.
- 행 높이 44px, Hover `#F8FAFD`. 이름 클릭으로 상세에 진입하고 체크 후 일괄 작업 바를 표시합니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “상세 보기, 재시도” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 Channel, 결과, 소요 시간, Response 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(Channel, 결과, 소요 시간, Response)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “Message Delivery 추적”로 정의합니다. List 워크벤치 리듬을 유지합니다: 필터, 훑어보기, 행 내 작업, 상세 재확인. 비즈니스 가치 없는 KPI를 쌓지 않습니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 65. 내비게이션 관리

- **Application / Route / 구현**: 통합 센터 · `/integration/navigation` · `IntegrationNavigation.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 해당 비즈니스 담당자.
- 주요 Task: Group별로 자주 쓰는 System 진입점을 관리합니다.
- 고빈도 Task: 진입점 추가, 공개 링크 생성, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: Group, 진입점, 주소, 상태, 공개 링크.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 Group, 진입점, 공개 상태을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / Group, 진입점, 공개 상태
├─ Title + 13px description
└─ Primary: 진입점 추가, 공개 링크 생성
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, Group, 진입점, 공개 상태 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| FilterToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| DataTable | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: 진입점 추가, 공개 링크 생성, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 테이블 설계

- 열 순서: 이름/식별자(220px) → Group, 진입점, 공개 상태(160px) → 상태(100px) → 최근 시각/경과(150px) → 작업(140px fixed-right).
- 고정 Column: 이름과 작업. 긴 주소, 표현식, Response, 메모는 생략을 허용하며 Hover로 전체 값을 표시합니다.
- 상태는 Tag를 사용합니다. ID, 주소, Command, Query, Version, Timestamp는 고정폭 Font를 사용하고 보조 Meta 정보는 이름 아래 두 번째 줄에서 강조를 낮춥니다.
- 행 높이 44px, Hover `#F8FAFD`. 이름 클릭으로 상세에 진입하고 체크 후 일괄 작업 바를 표시합니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “진입점 추가, 공개 링크 생성” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 Group, 진입점, 공개 상태 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(Group, 진입점, 공개 상태)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “System Entry Orchestration”로 정의합니다. List 워크벤치 리듬을 유지합니다: 필터, 훑어보기, 행 내 작업, 상세 재확인. 비즈니스 가치 없는 KPI를 쌓지 않습니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 66. Public Navigation

- **Application / Route / 구현**: 통합 센터 · `/public/navigation/:token` · `PublicNavigation.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 해당 비즈니스 담당자.
- 주요 Task: 승인된 System 진입점을 검색하고 엽니다.
- 고빈도 Task: 진입점 열기, 검색, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: Brand, Group, 진입점, 가용 상태.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 Access Token, 진입점, 가용성을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / Access Token, 진입점, 가용성
├─ Title + 13px description
└─ Primary: 진입점 열기, 검색
Primary Content Area\n├─ 핵심 정보 또는 시각화 작업 영역\n└─ 상세, 결과 또는 보조 정보 영역
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, Access Token, 진입점, 가용성 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| WorkspaceToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| PrimaryWorkspace | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: 진입점 열기, 검색, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 내용과 상태 설계

- 핵심 Content는 첫 번째 가시 영역에 배치하고, 보조 설명과 원본 Data는 두 번째 계층 또는 접기 영역에 배치합니다.
- 숫자와 시각은 우측 정렬 또는 고정폭 표시하며, 이상 상태를 Health 상태보다 우선 표시합니다.
- 그래프, Log, Canvas, 편집기는 각자 명확한 작업 영역을 차지하며 일반 Form과 혼배열하지 않습니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “진입점 열기, 검색” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 Access Token, 진입점, 가용성 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(Access Token, 진입점, 가용성)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “읽기 전용 Entry Portal”로 정의합니다. 현재 Task의 주요 정보 구조로 첫 화면을 구성하고 보조 정보는 읽기 경로에 따라 계층화합니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 67. AI 대화

- **Application / Route / 구현**: 통합 센터 · `/integration/ai/chat` · `AIAssistantChat.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 해당 비즈니스 담당자.
- 주요 Task: 대화를 시작하고 Tool을 호출한 뒤 결과를 확인합니다.
- 고빈도 Task: Message 전송, 새 대화 생성, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: 대화, Model, Message, Tool 호출, 실행 상태.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 대화, Model, Tool, 감사을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / 대화, Model, Tool, 감사
├─ Title + 13px description
└─ Primary: Message 전송, 새 대화 생성
Workspace Context / Control Bar\n├─ 현재 범위, 새로고침 또는 연결 상태\n└─ 보조 작업\nPrimary Content Area\n├─ 핵심 정보 또는 시각화 작업 영역\n└─ 상세, 결과 또는 보조 정보 영역
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, 대화, Model, Tool, 감사 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| WorkspaceToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| PrimaryWorkspace | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: Message 전송, 새 대화 생성, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 내용과 상태 설계

- 핵심 Content는 첫 번째 가시 영역에 배치하고, 보조 설명과 원본 Data는 두 번째 계층 또는 접기 영역에 배치합니다.
- 숫자와 시각은 우측 정렬 또는 고정폭 표시하며, 이상 상태를 Health 상태보다 우선 표시합니다.
- 그래프, Log, Canvas, 편집기는 각자 명확한 작업 영역을 차지하며 일반 Form과 혼배열하지 않습니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “Message 전송, 새 대화 생성” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 대화, Model, Tool, 감사 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(대화, Model, Tool, 감사)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “AI 운영 협업 워크벤치”로 정의합니다. 주 작업 영역을 우선하고 Toolbar와 보조 Panel은 접을 수 있으며, KPI + 테이블 Page로 개조하지 않습니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 68. AI Session 관리

- **Application / Route / 구현**: 통합 센터 · `/integration/ai/conversations` · `AIConversations.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 해당 비즈니스 담당자.
- 주요 Task: Session을 검색·보관하고 추적합니다.
- 고빈도 Task: 새로 만들기, 보관, 보기, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: Session 제목, Model, Message 수, 업데이트 시각, 상태.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 Session, Model, 감사을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / Session, Model, 감사
├─ Title + 13px description
└─ Primary: 새로 만들기, 보관, 보기
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, Session, Model, 감사 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| FilterToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| DataTable | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: 새로 만들기, 보관, 보기, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 테이블 설계

- 열 순서: 이름/식별자(220px) → Session, Model, 감사(160px) → 상태(100px) → 최근 시각/경과(150px) → 작업(140px fixed-right).
- 고정 Column: 이름과 작업. 긴 주소, 표현식, Response, 메모는 생략을 허용하며 Hover로 전체 값을 표시합니다.
- 상태는 Tag를 사용합니다. ID, 주소, Command, Query, Version, Timestamp는 고정폭 Font를 사용하고 보조 Meta 정보는 이름 아래 두 번째 줄에서 강조를 낮춥니다.
- 행 높이 44px, Hover `#F8FAFD`. 이름 클릭으로 상세에 진입하고 체크 후 일괄 작업 바를 표시합니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “새로 만들기, 보관, 보기” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 Session, Model, 감사 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(Session, Model, 감사)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “AI Session 거버넌스”로 정의합니다. List 워크벤치 리듬을 유지합니다: 필터, 훑어보기, 행 내 작업, 상세 재확인. 비즈니스 가치 없는 KPI를 쌓지 않습니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 69. AI Model 관리

- **Application / Route / 구현**: 통합 센터 · `/integration/ai/models` · `AIModels.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 해당 비즈니스 담당자.
- 주요 Task: OpenAI 호환 Model과 기본 Parameter를 관리합니다.
- 고빈도 Task: Model 추가, 연결 Test, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: Model, 주소, 연결 상태, 기본 표식, 업데이트 시각.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 Model, 연결, 기본 Config을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / Model, 연결, 기본 Config
├─ Title + 13px description
└─ Primary: Model 추가, 연결 Test
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, Model, 연결, 기본 Config 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| FilterToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| DataTable | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: Model 추가, 연결 Test, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 테이블 설계

- 열 순서: 이름/식별자(220px) → Model, 연결, 기본 Config(160px) → 상태(100px) → 최근 시각/경과(150px) → 작업(140px fixed-right).
- 고정 Column: 이름과 작업. 긴 주소, 표현식, Response, 메모는 생략을 허용하며 Hover로 전체 값을 표시합니다.
- 상태는 Tag를 사용합니다. ID, 주소, Command, Query, Version, Timestamp는 고정폭 Font를 사용하고 보조 Meta 정보는 이름 아래 두 번째 줄에서 강조를 낮춥니다.
- 행 높이 44px, Hover `#F8FAFD`. 이름 클릭으로 상세에 진입하고 체크 후 일괄 작업 바를 표시합니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “Model 추가, 연결 Test” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 Model, 연결, 기본 Config 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(Model, 연결, 기본 Config)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “Model 접속 Config”로 정의합니다. List 워크벤치 리듬을 유지합니다: 필터, 훑어보기, 행 내 작업, 상세 재확인. 비즈니스 가치 없는 KPI를 쌓지 않습니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 70. AI Knowledge Base

- **Application / Route / 구현**: 통합 센터 · `/integration/ai/knowledge-base` · `AIKnowledgeBase.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 해당 비즈니스 담당자.
- 주요 Task: Markdown 문서와 검색 Config를 관리합니다.
- 고빈도 Task: 문서 추가, 재색인, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: Document, Source, 업데이트 시각, Index 상태, Hit Config.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 Knowledge Base, Index, Source을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / Knowledge Base, Index, Source
├─ Title + 13px description
└─ Primary: 문서 추가, 재색인
Primary Content Area\n├─ 핵심 정보 또는 시각화 작업 영역\n└─ 상세, 결과 또는 보조 정보 영역
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, Knowledge Base, Index, Source 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| WorkspaceToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| PrimaryWorkspace | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: 문서 추가, 재색인, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 내용과 상태 설계

- 핵심 Content는 첫 번째 가시 영역에 배치하고, 보조 설명과 원본 Data는 두 번째 계층 또는 접기 영역에 배치합니다.
- 숫자와 시각은 우측 정렬 또는 고정폭 표시하며, 이상 상태를 Health 상태보다 우선 표시합니다.
- 그래프, Log, Canvas, 편집기는 각자 명확한 작업 영역을 차지하며 일반 Form과 혼배열하지 않습니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “문서 추가, 재색인” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 Knowledge Base, Index, Source 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(Knowledge Base, Index, Source)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “검색 Knowledge 유지보수”로 정의합니다. 현재 Task의 주요 정보 구조로 첫 화면을 구성하고 보조 정보는 읽기 경로에 따라 계층화합니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 71. AI Tool Set

- **Application / Route / 구현**: 통합 센터 · `/integration/ai/tools` · `AITools.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 해당 비즈니스 담당자.
- 주요 Task: Ops Tool을 활성화·구성하고 제약을 둡니다.
- 고빈도 Task: Tool 추가, 활성/비활성, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: Tool, Type, 권한, 상태, 호출 범위.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 Tool, 권한, 감사을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / Tool, 권한, 감사
├─ Title + 13px description
└─ Primary: Tool 추가, 활성/비활성
Primary Content Area\n├─ 핵심 정보 또는 시각화 작업 영역\n└─ 상세, 결과 또는 보조 정보 영역
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, Tool, 권한, 감사 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| WorkspaceToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| PrimaryWorkspace | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: Tool 추가, 활성/비활성, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 내용과 상태 설계

- 핵심 Content는 첫 번째 가시 영역에 배치하고, 보조 설명과 원본 Data는 두 번째 계층 또는 접기 영역에 배치합니다.
- 숫자와 시각은 우측 정렬 또는 고정폭 표시하며, 이상 상태를 Health 상태보다 우선 표시합니다.
- 그래프, Log, Canvas, 편집기는 각자 명확한 작업 영역을 차지하며 일반 Form과 혼배열하지 않습니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “Tool 추가, 활성/비활성” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 Tool, 권한, 감사 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(Tool, 권한, 감사)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “AI Tool 거버넌스”로 정의합니다. 현재 Task의 주요 정보 구조로 첫 화면을 구성하고 보조 정보는 읽기 경로에 따라 계층화합니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 72. FinOps 비용 보드

- **Application / Route / 구현**: 통합 센터 · `/integration/finops/dashboard` · `FinOpsDashboard.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 해당 비즈니스 담당자.
- 주요 Task: Cost, Trend, 이상을 판단합니다.
- 고빈도 Task: Billing Month 전환, 동기화, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: Billing Month, 총 비용, 전년 대비, Cloud Provider, Trend.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 Billing Month, Account, Cost, 이상을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / Billing Month, Account, Cost, 이상
├─ Title + 13px description
└─ Primary: Billing Month 전환, 동기화
Primary Content Area\n├─ 핵심 정보 또는 시각화 작업 영역\n└─ 상세, 결과 또는 보조 정보 영역
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, Billing Month, Account, Cost, 이상 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| WorkspaceToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| PrimaryWorkspace | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: Billing Month 전환, 동기화, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 내용과 상태 설계

- 핵심 Content는 첫 번째 가시 영역에 배치하고, 보조 설명과 원본 Data는 두 번째 계층 또는 접기 영역에 배치합니다.
- 숫자와 시각은 우측 정렬 또는 고정폭 표시하며, 이상 상태를 Health 상태보다 우선 표시합니다.
- 그래프, Log, Canvas, 편집기는 각자 명확한 작업 영역을 차지하며 일반 Form과 혼배열하지 않습니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “Billing Month 전환, 동기화” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 Billing Month, Account, Cost, 이상 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(Billing Month, Account, Cost, 이상)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “Cloud Cost Overview”로 정의합니다. 현재 Task의 주요 정보 구조로 첫 화면을 구성하고 보조 정보는 읽기 경로에 따라 계층화합니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 73. FinOps Cloud Account

- **Application / Route / 구현**: 통합 센터 · `/integration/finops/accounts` · `FinOpsAccounts.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 해당 비즈니스 담당자.
- 주요 Task: Cloud Billing 접속과 동기화를 관리합니다.
- 고빈도 Task: Account 추가, 동기화, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: Account, Provider, Billing Month, 동기화 상태, 업데이트 시각.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 Provider, Account, Billing Month, 동기화을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / Provider, Account, Billing Month, 동기화
├─ Title + 13px description
└─ Primary: Account 추가, 동기화
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, Provider, Account, Billing Month, 동기화 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| FilterToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| DataTable | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: Account 추가, 동기화, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 테이블 설계

- 열 순서: 이름/식별자(220px) → Provider, Account, Billing Month, 동기화(160px) → 상태(100px) → 최근 시각/경과(150px) → 작업(140px fixed-right).
- 고정 Column: 이름과 작업. 긴 주소, 표현식, Response, 메모는 생략을 허용하며 Hover로 전체 값을 표시합니다.
- 상태는 Tag를 사용합니다. ID, 주소, Command, Query, Version, Timestamp는 고정폭 Font를 사용하고 보조 Meta 정보는 이름 아래 두 번째 줄에서 강조를 낮춥니다.
- 행 높이 44px, Hover `#F8FAFD`. 이름 클릭으로 상세에 진입하고 체크 후 일괄 작업 바를 표시합니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “Account 추가, 동기화” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 Provider, Account, Billing Month, 동기화 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(Provider, Account, Billing Month, 동기화)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “Billing Account 관리”로 정의합니다. List 워크벤치 리듬을 유지합니다: 필터, 훑어보기, 행 내 작업, 상세 재확인. 비즈니스 가치 없는 KPI를 쌓지 않습니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 74. FinOps 비용 분할

- **Application / Route / 구현**: 통합 센터 · `/integration/finops/breakdown` · `FinOpsBreakdown.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 해당 비즈니스 담당자.
- 주요 Task: Dimension별로 Cost를 분할하고 증가 지점을 찾아냅니다.
- 고빈도 Task: Dimension 전환, 내보내기, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: Dimension, 금액, 비중, 전월 대비, 필터 조건.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 Billing Month, Account, Region, Service, Label을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / Billing Month, Account, Region, Service, Label
├─ Title + 13px description
└─ Primary: Dimension 전환, 내보내기
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, Billing Month, Account, Region, Service, Label 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| FilterToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| DataTable | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: Dimension 전환, 내보내기, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 테이블 설계

- 열 순서: 이름/식별자(220px) → Billing Month, Account, Region, Service, Label(160px) → 상태(100px) → 최근 시각/경과(150px) → 작업(140px fixed-right).
- 고정 Column: 이름과 작업. 긴 주소, 표현식, Response, 메모는 생략을 허용하며 Hover로 전체 값을 표시합니다.
- 상태는 Tag를 사용합니다. ID, 주소, Command, Query, Version, Timestamp는 고정폭 Font를 사용하고 보조 Meta 정보는 이름 아래 두 번째 줄에서 강조를 낮춥니다.
- 행 높이 44px, Hover `#F8FAFD`. 이름 클릭으로 상세에 진입하고 체크 후 일괄 작업 바를 표시합니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “Dimension 전환, 내보내기” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 Billing Month, Account, Region, Service, Label 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(Billing Month, Account, Region, Service, Label)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “Cost 귀인 분석”으로 정의합니다. List 워크벤치 리듬을 유지합니다: 필터, 훑어보기, 행 내 작업, 상세 재확인. 비즈니스 가치 없는 KPI를 쌓지 않습니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 75. FinOps 최적화 권고

- **Application / Route / 구현**: 통합 센터 · `/integration/finops/recommendations` · `FinOpsRecommendations.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 해당 비즈니스 담당자.
- 주요 Task: 미사용, 과다 Spec, 이상 권고를 처리합니다.
- 고빈도 Task: 채택, 무시, Resource 보기, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: 권고, 절감 금액, 영향 Resource, 우선순위, 상태.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 Cost, Resource, 우선순위, 담당자을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / Cost, Resource, 우선순위, 담당자
├─ Title + 13px description
└─ Primary: 채택, 무시, Resource 보기
Primary Content Area\n├─ 핵심 정보 또는 시각화 작업 영역\n└─ 상세, 결과 또는 보조 정보 영역
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, Cost, Resource, 우선순위, 담당자 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| WorkspaceToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| PrimaryWorkspace | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: 채택, 무시, Resource 보기, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 내용과 상태 설계

- 핵심 Content는 첫 번째 가시 영역에 배치하고, 보조 설명과 원본 Data는 두 번째 계층 또는 접기 영역에 배치합니다.
- 숫자와 시각은 우측 정렬 또는 고정폭 표시하며, 이상 상태를 Health 상태보다 우선 표시합니다.
- 그래프, Log, Canvas, 편집기는 각자 명확한 작업 영역을 차지하며 일반 Form과 혼배열하지 않습니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “채택, 무시, Resource 보기” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 Cost, Resource, 우선순위, 담당자 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(Cost, Resource, 우선순위, 담당자)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “Cost 최적화 워크벤치”으로 정의합니다. 현재 Task의 주요 정보 구조로 첫 화면을 구성하고 보조 정보는 읽기 경로에 따라 계층화합니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 76. FinOps Resource 분할

- **Application / Route / 구현**: 통합 센터 · `/integration/finops/resources` · `FinOpsResources.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 해당 비즈니스 담당자.
- 주요 Task: Resource 사용률과 Cost를 찾아냅니다.
- 고빈도 Task: 필터링, 상세 보기, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: Resource, Account, Region, Cost, 사용률, 상태.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 Resource, Cost, 사용률, Billing Month을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / Resource, Cost, 사용률, Billing Month
├─ Title + 13px description
└─ Primary: 필터링, 상세 보기
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, Resource, Cost, 사용률, Billing Month 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| FilterToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| DataTable | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: 필터링, 상세 보기, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 테이블 설계

- 열 순서: 이름/식별자(220px) → Resource, Cost, 사용률, Billing Month(160px) → 상태(100px) → 최근 시각/경과(150px) → 작업(140px fixed-right).
- 고정 Column: 이름과 작업. 긴 주소, 표현식, Response, 메모는 생략을 허용하며 Hover로 전체 값을 표시합니다.
- 상태는 Tag를 사용합니다. ID, 주소, Command, Query, Version, Timestamp는 고정폭 Font를 사용하고 보조 Meta 정보는 이름 아래 두 번째 줄에서 강조를 낮춥니다.
- 행 높이 44px, Hover `#F8FAFD`. 이름 클릭으로 상세에 진입하고 체크 후 일괄 작업 바를 표시합니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “필터링, 상세 보기” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 Resource, Cost, 사용률, Billing Month 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(Resource, Cost, 사용률, Billing Month)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “Resource Cost 분석”으로 정의합니다. List 워크벤치 리듬을 유지합니다: 필터, 훑어보기, 행 내 작업, 상세 재확인. 비즈니스 가치 없는 KPI를 쌓지 않습니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 77. FinOps Billing 동기화

- **Application / Route / 구현**: 통합 센터 · `/integration/finops/sync` · `FinOpsSync.vue`

### 1. 페이지 목적

- 핵심 사용자: 플랫폼 관리자, 해당 비즈니스 담당자.
- 주요 Task: 동기화를 구성하고 동기화 History를 추적합니다.
- 고빈도 Task: 즉시 동기화, 설정 저장, 현재 범위 필터링, 상세 또는 결과 진입.
- 핵심 정보: Billing Month, Account, 동기화 범위, 결과, 시각.

### 2. 현재 UI 문제

- 제목, Context, 핵심 Action은 동일 시각 밴드에 고정해야 하며, 사용자가 테이블이나 Content 본문만 보다가 Account, Billing Month, 동기화, 오류을(를) 놓치지 않게 합니다.
- 보조 필터와 주 Action은 동급이 아니어야 합니다. 현재 Page에는 고강조 주 Action이 하나만 있고 나머지는 Outline 또는 Text 버튼입니다.
- 상태 정보는 대면적 고채도 색 블록에서 Label, 숫자, 부분 색 띠로 수렴해 이상 스캔을 방해하지 않게 합니다.
- Content 영역은 1366px Viewport에서 첫 화면 가독성을 유지하고, 공백·긴 필드·행 작업이 핵심 Data를 눌러 담지 않게 합니다.

### 3. 전체 레이아웃 설계

```text
Page Header(24px page padding)
├─ Breadcrumb / Account, Billing Month, 동기화, 오류
├─ Title + 13px description
└─ Primary: 즉시 동기화, 설정 저장
Primary Content Area\n├─ 핵심 정보 또는 시각화 작업 영역\n└─ 상세, 결과 또는 보조 정보 영역
```

### 4. 주요 Component

| Component | 용도와 위치 | 시각 계층 / 상호작용 |
| --- | --- | --- |
| PageHeader | 상단, 제목·설명·주 Action 담당 | 흰 배경, 큰 그림자 없음; 주 Action은 오른쪽 |
| ContextBar | Header 아래, Account, Billing Month, 동기화, 오류 표시 | 높이 32px; 범위 변경 시 즉시 Data 새로고침 |
| WorkspaceToolbar | 핵심 Content 앞 | 밝은 회색 배경; Enter 조회, 초기화는 주 Action을 뺏지 않음 |
| PrimaryWorkspace | Page 본문 | 테두리 Card; Empty와 Loading 상태는 영역 내 표시 |
| DetailDrawer / Dialog | 상세, 수정, Log 또는 확인 | 현재 Context를 벗어나지 않음; 닫은 뒤 필터와 스크롤 위치 유지 |

### 5. 구체 시각 Spec

- Page padding: 24px(좁은 화면 16px); Header 아래 간격: 16px; Block gap: 16px.
- 제목: 22px / 650 / `#18243A`; 설명: 13px / 400 / `#66758D`; 고정폭 필드는 `ui-monospace` 12px 사용.
- Card: 배경 `#FFFFFF`, 테두리 `#E3E8F0`, 모서리 반경 10px, 그림자 `0 2px 5px rgba(20,34,58,.035)`.
- Control: 높이 32px, 모서리 반경 7px, 간격 8px; Hover 배경 `#F8FAFD`; Focus 테두리 `#356AE6`.
- 상태는 Success `#1F9D62`, Warning `#D98C16`, Danger `#D94F4F`, Info `#356AE6`의 밝은 배경 + 진한 글자만 사용합니다.

### 6. 작업 계층

- **Primary**: 즉시 동기화, 설정 저장, Solid Blue 버튼; 화면당 최대 1개.
- **Secondary**: 조회, 새로고침, 필터, 내보내기, Outline 버튼 또는 Toolbar Control.
- **Tertiary**: 보기, 복사, 이동, 펼치기, 테이블 행 내 Text 버튼.
- **More**: 드물게 쓰는 작업은 드롭다운 메뉴로 수납합니다. **Danger**: 삭제, 중단, 닫기 또는 되돌릴 수 없는 실행은 반드시 재확인합니다.

### 7. 내용과 상태 설계

- 핵심 Content는 첫 번째 가시 영역에 배치하고, 보조 설명과 원본 Data는 두 번째 계층 또는 접기 영역에 배치합니다.
- 숫자와 시각은 우측 정렬 또는 고정폭 표시하며, 이상 상태를 Health 상태보다 우선 표시합니다.
- 그래프, Log, Canvas, 편집기는 각자 명확한 작업 영역을 차지하며 일반 Form과 혼배열하지 않습니다.

### 8. 상태 설계

- Loading: Content 영역에 Skeleton Screen을 사용하고 Header와 필터 Context는 유지합니다.
- Empty: 현재 범위에 Data가 없음을 안내하고 “즉시 동기화, 설정 저장” 또는 필터 조정 같은 다음 단계를 제공합니다.
- Error / Permission Denied: 원인, 재시도 진입점, 필요한 권한 요청 안내를 표시합니다.
- Running / Success / Warning / Failed / Disabled: 통일 StatusTag를 사용합니다. 운영 상태는 Pending, Timeout, Disconnected, Unknown, Partial Success, Terminating으로 보강합니다.

### 9. Dialog / Drawer

- 생성/수정: 필드가 10개 미만이면 640px Dialog를, 필드가 많거나 현재 대비 유지가 필요하면 720px Drawer를 사용합니다.
- 상세, Log, 원본 Response, YAML: 80vw Drawer 사용; 본문은 스크롤 가능, Header에 Account, Billing Month, 동기화, 오류 고정 표시.
- 삭제, 중단 또는 고위험 실행: 480px Confirm Dialog. Footer 왼쪽에 리스크 설명, 오른쪽에 취소와 위험 확인 버튼을 두고 확인 버튼은 기본 Primary Color를 쓰지 않습니다.

### 10. SRE 전문 설계

- Page는 현재 Task에 가치 있는 Context(Account, Billing Month, 동기화, 오류)만 지속 표시합니다.
- 실행 상태를 변경하는 모든 작업은 대상 범위, 실행 시각, 결과, 감사 진입점을 표시해야 합니다.
- 이상, 미확인, 부분 성공, 연결 끊김에는 색만 표시하지 말고 실행 가능한 다음 단계를 우선 제공합니다.

### 11. 페이지 전용 설계

- 작업 Mode는 “Billing 동기화 제어”로 정의합니다. 현재 Task의 주요 정보 구조로 첫 화면을 구성하고 보조 정보는 읽기 경로에 따라 계층화합니다.
- 구현 후 1366px 데스크톱 Viewport로 제목, 주 Action, 첫 화면 Content, 팝업 레이어, Empty/Error 상태를 검증하고 기존 Route, API, 상호작용 로직은 변경하지 않습니다.

## 78. 监控概览

- **应用 / 路由 / 实现**：监控中心 · `/monitor/overview` · `MonitorOverview.vue`

### 1. 页面定位

- 核心用户：平台管理员、值班 SRE。
- 主任务：判断当前风险、质量与待处理项。
- 高频任务：刷新、切换时间范围、筛选当前范围、进入详情或结果。
- 首要信息：活动告警、未认领、P0/P1、数据源质量。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去时间范围、告警、数据源、负责人。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 时间范围、告警、数据源、负责人
├─ Title + 13px description
└─ Primary: 刷新、切换时间范围
Workspace Context / Control Bar\n├─ 当前范围、刷新或连接状态\n└─ 次级操作\nPrimary Content Area\n├─ 核心信息或可视化工作区\n└─ 详情、结果或辅助信息区
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 时间范围、告警、数据源、负责人 | 32px 高；范围变化立即刷新数据 |
| WorkspaceToolbar | 核心内容前 | 浅灰底；Enter 查询，重置不抢主操作 |
| PrimaryWorkspace | 页面主体 | 边框卡片；空态与加载态在区域内出现 |
| DetailDrawer / Dialog | 详情、编辑、日志或确认 | 不离开当前上下文；关闭后保留筛选与滚动位置 |

### 5. 具体视觉规范

- 页面 padding：24px（窄屏 16px）；Header 下间距：16px；区块 gap：16px。
- 标题：22px / 650 / `#18243A`；说明：13px / 400 / `#66758D`；等宽字段使用 `ui-monospace` 12px。
- 卡片：背景 `#FFFFFF`、边框 `#E3E8F0`、圆角 10px、阴影 `0 2px 5px rgba(20,34,58,.035)`。
- 控件：高度 32px、圆角 7px、间距 8px；Hover 背景 `#F8FAFD`；Focus 边框 `#356AE6`。
- 状态仅使用 Success `#1F9D62`、Warning `#D98C16`、Danger `#D94F4F`、Info `#356AE6` 的浅色底 + 深色字。

### 6. 操作层级

- **Primary**：刷新、切换时间范围，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 内容与状态设计

- 核心内容置于首个可视区域；辅助说明和原始数据放在第二层或可折叠区。
- 数字与时间右对齐或等宽显示；异常状态优先于健康状态。
- 图形、日志、画布和编辑器各自占用明确工作区，避免与普通表单混排。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“刷新、切换时间范围”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 时间范围、告警、数据源、负责人。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：时间范围、告警、数据源、负责人。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“风险优先总览”。主工作区优先，工具栏与辅助面板可折叠；不将其改造成 KPI + 表格页面。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 79. 智能大屏

- **应用 / 路由 / 实现**：监控中心 · `/monitor/command-center` · `MonitorCommandCenter.vue`

### 1. 页面定位

- 核心用户：平台管理员、值班 SRE。
- 主任务：观察高优先级风险与资源热点。
- 高频任务：全屏、刷新、筛选当前范围、进入详情或结果。
- 首要信息：告警、资源、地域、热点主机、刷新状态。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去时间、告警、资源、地域。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 时间、告警、资源、地域
├─ Title + 13px description
└─ Primary: 全屏、刷新
Workspace Context / Control Bar\n├─ 当前范围、刷新或连接状态\n└─ 次级操作\nPrimary Content Area\n├─ 核心信息或可视化工作区\n└─ 详情、结果或辅助信息区
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 时间、告警、资源、地域 | 32px 高；范围变化立即刷新数据 |
| WorkspaceToolbar | 核心内容前 | 浅灰底；Enter 查询，重置不抢主操作 |
| PrimaryWorkspace | 页面主体 | 边框卡片；空态与加载态在区域内出现 |
| DetailDrawer / Dialog | 详情、编辑、日志或确认 | 不离开当前上下文；关闭后保留筛选与滚动位置 |

### 5. 具体视觉规范

- 页面 padding：24px（窄屏 16px）；Header 下间距：16px；区块 gap：16px。
- 标题：22px / 650 / `#18243A`；说明：13px / 400 / `#66758D`；等宽字段使用 `ui-monospace` 12px。
- 卡片：背景 `#FFFFFF`、边框 `#E3E8F0`、圆角 10px、阴影 `0 2px 5px rgba(20,34,58,.035)`。
- 控件：高度 32px、圆角 7px、间距 8px；Hover 背景 `#F8FAFD`；Focus 边框 `#356AE6`。
- 状态仅使用 Success `#1F9D62`、Warning `#D98C16`、Danger `#D94F4F`、Info `#356AE6` 的浅色底 + 深色字。

### 6. 操作层级

- **Primary**：全屏、刷新，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 内容与状态设计

- 核心内容置于首个可视区域；辅助说明和原始数据放在第二层或可折叠区。
- 数字与时间右对齐或等宽显示；异常状态优先于健康状态。
- 图形、日志、画布和编辑器各自占用明确工作区，避免与普通表单混排。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“全屏、刷新”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 时间、告警、资源、地域。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：时间、告警、资源、地域。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“实时运维驾驶舱”。主工作区优先，工具栏与辅助面板可折叠；不将其改造成 KPI + 表格页面。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 80. 数据源管理

- **应用 / 路由 / 实现**：监控中心 · `/monitor/datasources` · `MonitorDatasource.vue`

### 1. 页面定位

- 核心用户：平台管理员、值班 SRE。
- 主任务：维护指标、日志和 Trace 数据源。
- 高频任务：新增数据源、测试、筛选当前范围、进入详情或结果。
- 首要信息：数据源、类型、地址、健康、更新时间。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去数据源、健康、错误。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 数据源、健康、错误
├─ Title + 13px description
└─ Primary: 新增数据源、测试
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 数据源、健康、错误 | 32px 高；范围变化立即刷新数据 |
| FilterToolbar | 核心内容前 | 浅灰底；Enter 查询，重置不抢主操作 |
| DataTable | 页面主体 | 边框卡片；空态与加载态在区域内出现 |
| DetailDrawer / Dialog | 详情、编辑、日志或确认 | 不离开当前上下文；关闭后保留筛选与滚动位置 |

### 5. 具体视觉规范

- 页面 padding：24px（窄屏 16px）；Header 下间距：16px；区块 gap：16px。
- 标题：22px / 650 / `#18243A`；说明：13px / 400 / `#66758D`；等宽字段使用 `ui-monospace` 12px。
- 卡片：背景 `#FFFFFF`、边框 `#E3E8F0`、圆角 10px、阴影 `0 2px 5px rgba(20,34,58,.035)`。
- 控件：高度 32px、圆角 7px、间距 8px；Hover 背景 `#F8FAFD`；Focus 边框 `#356AE6`。
- 状态仅使用 Success `#1F9D62`、Warning `#D98C16`、Danger `#D94F4F`、Info `#356AE6` 的浅色底 + 深色字。

### 6. 操作层级

- **Primary**：新增数据源、测试，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 表格设计

- 列顺序：名称/标识（220px）→ 数据源、健康、错误（160px）→ 状态（100px）→ 最近时间/年龄（150px）→ 操作（140px fixed-right）。
- 关键列固定：名称与操作；长地址、表达式、响应和备注允许省略，Hover 显示完整值。
- 状态使用 Tag；ID、地址、命令、查询、版本和时间戳使用等宽字体；次级元信息在名称下第二行弱化。
- 行高 44px，Hover `#F8FAFD`；点击名称进入详情，勾选后显示批量操作条。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“新增数据源、测试”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 数据源、健康、错误。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：数据源、健康、错误。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“观测数据接入”。保持列表工作台节奏：筛选、扫描、行内操作、详情回看；不堆叠无业务价值的 KPI。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 81. 即时查询

- **应用 / 路由 / 实现**：监控中心 · `/monitor/query` · `MonitorQuery.vue`

### 1. 页面定位

- 核心用户：平台管理员、值班 SRE。
- 主任务：编辑查询并查看即时与范围结果。
- 高频任务：执行、保存查询、筛选当前范围、进入详情或结果。
- 首要信息：数据源、PromQL、时间、结果、标签。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去数据源、查询、时间、指标。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 数据源、查询、时间、指标
├─ Title + 13px description
└─ Primary: 执行、保存查询
Primary Content Area\n├─ 核心信息或可视化工作区\n└─ 详情、结果或辅助信息区
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 数据源、查询、时间、指标 | 32px 高；范围变化立即刷新数据 |
| WorkspaceToolbar | 核心内容前 | 浅灰底；Enter 查询，重置不抢主操作 |
| PrimaryWorkspace | 页面主体 | 边框卡片；空态与加载态在区域内出现 |
| DetailDrawer / Dialog | 详情、编辑、日志或确认 | 不离开当前上下文；关闭后保留筛选与滚动位置 |

### 5. 具体视觉规范

- 页面 padding：24px（窄屏 16px）；Header 下间距：16px；区块 gap：16px。
- 标题：22px / 650 / `#18243A`；说明：13px / 400 / `#66758D`；等宽字段使用 `ui-monospace` 12px。
- 卡片：背景 `#FFFFFF`、边框 `#E3E8F0`、圆角 10px、阴影 `0 2px 5px rgba(20,34,58,.035)`。
- 控件：高度 32px、圆角 7px、间距 8px；Hover 背景 `#F8FAFD`；Focus 边框 `#356AE6`。
- 状态仅使用 Success `#1F9D62`、Warning `#D98C16`、Danger `#D94F4F`、Info `#356AE6` 的浅色底 + 深色字。

### 6. 操作层级

- **Primary**：执行、保存查询，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 内容与状态设计

- 核心内容置于首个可视区域；辅助说明和原始数据放在第二层或可折叠区。
- 数字与时间右对齐或等宽显示；异常状态优先于健康状态。
- 图形、日志、画布和编辑器各自占用明确工作区，避免与普通表单混排。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“执行、保存查询”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 数据源、查询、时间、指标。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：数据源、查询、时间、指标。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“PromQL 查询工作台”。用当前任务的主信息结构组织首屏，次级信息按阅读路径分层。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 82. 日志查询

- **应用 / 路由 / 实现**：监控中心 · `/monitor/logs` · `MonitorLogQuery.vue`

### 1. 页面定位

- 核心用户：平台管理员、值班 SRE。
- 主任务：构造查询、查看直方图和日志上下文。
- 高频任务：查询、保存、展开上下文、筛选当前范围、进入详情或结果。
- 首要信息：数据源、时间、查询、字段、日志流。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去数据源、时间、服务、日志级别。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 数据源、时间、服务、日志级别
├─ Title + 13px description
└─ Primary: 查询、保存、展开上下文
Workspace Context / Control Bar\n├─ 当前范围、刷新或连接状态\n└─ 次级操作\nFilter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 数据源、时间、服务、日志级别 | 32px 高；范围变化立即刷新数据 |
| FilterToolbar | 核心内容前 | 浅灰底；Enter 查询，重置不抢主操作 |
| DataTable | 页面主体 | 边框卡片；空态与加载态在区域内出现 |
| DetailDrawer / Dialog | 详情、编辑、日志或确认 | 不离开当前上下文；关闭后保留筛选与滚动位置 |

### 5. 具体视觉规范

- 页面 padding：24px（窄屏 16px）；Header 下间距：16px；区块 gap：16px。
- 标题：22px / 650 / `#18243A`；说明：13px / 400 / `#66758D`；等宽字段使用 `ui-monospace` 12px。
- 卡片：背景 `#FFFFFF`、边框 `#E3E8F0`、圆角 10px、阴影 `0 2px 5px rgba(20,34,58,.035)`。
- 控件：高度 32px、圆角 7px、间距 8px；Hover 背景 `#F8FAFD`；Focus 边框 `#356AE6`。
- 状态仅使用 Success `#1F9D62`、Warning `#D98C16`、Danger `#D94F4F`、Info `#356AE6` 的浅色底 + 深色字。

### 6. 操作层级

- **Primary**：查询、保存、展开上下文，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 表格设计

- 列顺序：名称/标识（220px）→ 数据源、时间、服务、日志级别（160px）→ 状态（100px）→ 最近时间/年龄（150px）→ 操作（140px fixed-right）。
- 关键列固定：名称与操作；长地址、表达式、响应和备注允许省略，Hover 显示完整值。
- 状态使用 Tag；ID、地址、命令、查询、版本和时间戳使用等宽字体；次级元信息在名称下第二行弱化。
- 行高 44px，Hover `#F8FAFD`；点击名称进入详情，勾选后显示批量操作条。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“查询、保存、展开上下文”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 数据源、时间、服务、日志级别。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：数据源、时间、服务、日志级别。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“日志检索工作台”。主工作区优先，工具栏与辅助面板可折叠；不将其改造成 KPI + 表格页面。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 83. 链路追踪

- **应用 / 路由 / 实现**：监控中心 · `/monitor/traces` · `MonitorTraceQuery.vue`

### 1. 页面定位

- 核心用户：平台管理员、值班 SRE。
- 主任务：按服务、耗时和时间定位调用链。
- 高频任务：查询、查看详情、筛选当前范围、进入详情或结果。
- 首要信息：服务、操作、耗时、时间、Trace 列表。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去服务、时间、耗时、错误。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 服务、时间、耗时、错误
├─ Title + 13px description
└─ Primary: 查询、查看详情
Workspace Context / Control Bar\n├─ 当前范围、刷新或连接状态\n└─ 次级操作\nPrimary Content Area\n├─ 核心信息或可视化工作区\n└─ 详情、结果或辅助信息区
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 服务、时间、耗时、错误 | 32px 高；范围变化立即刷新数据 |
| WorkspaceToolbar | 核心内容前 | 浅灰底；Enter 查询，重置不抢主操作 |
| PrimaryWorkspace | 页面主体 | 边框卡片；空态与加载态在区域内出现 |
| DetailDrawer / Dialog | 详情、编辑、日志或确认 | 不离开当前上下文；关闭后保留筛选与滚动位置 |

### 5. 具体视觉规范

- 页面 padding：24px（窄屏 16px）；Header 下间距：16px；区块 gap：16px。
- 标题：22px / 650 / `#18243A`；说明：13px / 400 / `#66758D`；等宽字段使用 `ui-monospace` 12px。
- 卡片：背景 `#FFFFFF`、边框 `#E3E8F0`、圆角 10px、阴影 `0 2px 5px rgba(20,34,58,.035)`。
- 控件：高度 32px、圆角 7px、间距 8px；Hover 背景 `#F8FAFD`；Focus 边框 `#356AE6`。
- 状态仅使用 Success `#1F9D62`、Warning `#D98C16`、Danger `#D94F4F`、Info `#356AE6` 的浅色底 + 深色字。

### 6. 操作层级

- **Primary**：查询、查看详情，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 内容与状态设计

- 核心内容置于首个可视区域；辅助说明和原始数据放在第二层或可折叠区。
- 数字与时间右对齐或等宽显示；异常状态优先于健康状态。
- 图形、日志、画布和编辑器各自占用明确工作区，避免与普通表单混排。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“查询、查看详情”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 服务、时间、耗时、错误。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：服务、时间、耗时、错误。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“Trace 检索”。主工作区优先，工具栏与辅助面板可折叠；不将其改造成 KPI + 表格页面。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 84. Trace 详情

- **应用 / 路由 / 实现**：监控中心 · `/monitor/traces/:traceId` · `MonitorTraceDetail.vue`

### 1. 页面定位

- 核心用户：平台管理员、值班 SRE。
- 主任务：阅读瀑布图并定位异常 Span。
- 高频任务：返回、查看 Span、筛选当前范围、进入详情或结果。
- 首要信息：Trace、服务、Span、耗时、错误、日志。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去Trace、服务、Span、错误、日志。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / Trace、服务、Span、错误、日志
├─ Title + 13px description
└─ Primary: 返回、查看 Span
Workspace Context / Control Bar\n├─ 当前范围、刷新或连接状态\n└─ 次级操作\nPrimary Content Area\n├─ 核心信息或可视化工作区\n└─ 详情、结果或辅助信息区
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 Trace、服务、Span、错误、日志 | 32px 高；范围变化立即刷新数据 |
| WorkspaceToolbar | 核心内容前 | 浅灰底；Enter 查询，重置不抢主操作 |
| PrimaryWorkspace | 页面主体 | 边框卡片；空态与加载态在区域内出现 |
| DetailDrawer / Dialog | 详情、编辑、日志或确认 | 不离开当前上下文；关闭后保留筛选与滚动位置 |

### 5. 具体视觉规范

- 页面 padding：24px（窄屏 16px）；Header 下间距：16px；区块 gap：16px。
- 标题：22px / 650 / `#18243A`；说明：13px / 400 / `#66758D`；等宽字段使用 `ui-monospace` 12px。
- 卡片：背景 `#FFFFFF`、边框 `#E3E8F0`、圆角 10px、阴影 `0 2px 5px rgba(20,34,58,.035)`。
- 控件：高度 32px、圆角 7px、间距 8px；Hover 背景 `#F8FAFD`；Focus 边框 `#356AE6`。
- 状态仅使用 Success `#1F9D62`、Warning `#D98C16`、Danger `#D94F4F`、Info `#356AE6` 的浅色底 + 深色字。

### 6. 操作层级

- **Primary**：返回、查看 Span，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 内容与状态设计

- 核心内容置于首个可视区域；辅助说明和原始数据放在第二层或可折叠区。
- 数字与时间右对齐或等宽显示；异常状态优先于健康状态。
- 图形、日志、画布和编辑器各自占用明确工作区，避免与普通表单混排。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“返回、查看 Span”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 Trace、服务、Span、错误、日志。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：Trace、服务、Span、错误、日志。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“调用链诊断”。主工作区优先，工具栏与辅助面板可折叠；不将其改造成 KPI + 表格页面。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 85. 告警规则

- **应用 / 路由 / 实现**：监控中心 · `/monitor/alert-rules` · `MonitorAlertRule.vue`

### 1. 页面定位

- 核心用户：平台管理员、值班 SRE。
- 主任务：维护指标阈值、周期与通知策略。
- 高频任务：新增规则、批量操作、筛选当前范围、进入详情或结果。
- 首要信息：规则、数据源、表达式、等级、状态、更新时间。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去数据源、表达式、等级、通知。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 数据源、表达式、等级、通知
├─ Title + 13px description
└─ Primary: 新增规则、批量操作
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 数据源、表达式、等级、通知 | 32px 高；范围变化立即刷新数据 |
| FilterToolbar | 核心内容前 | 浅灰底；Enter 查询，重置不抢主操作 |
| DataTable | 页面主体 | 边框卡片；空态与加载态在区域内出现 |
| DetailDrawer / Dialog | 详情、编辑、日志或确认 | 不离开当前上下文；关闭后保留筛选与滚动位置 |

### 5. 具体视觉规范

- 页面 padding：24px（窄屏 16px）；Header 下间距：16px；区块 gap：16px。
- 标题：22px / 650 / `#18243A`；说明：13px / 400 / `#66758D`；等宽字段使用 `ui-monospace` 12px。
- 卡片：背景 `#FFFFFF`、边框 `#E3E8F0`、圆角 10px、阴影 `0 2px 5px rgba(20,34,58,.035)`。
- 控件：高度 32px、圆角 7px、间距 8px；Hover 背景 `#F8FAFD`；Focus 边框 `#356AE6`。
- 状态仅使用 Success `#1F9D62`、Warning `#D98C16`、Danger `#D94F4F`、Info `#356AE6` 的浅色底 + 深色字。

### 6. 操作层级

- **Primary**：新增规则、批量操作，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 表格设计

- 列顺序：名称/标识（220px）→ 数据源、表达式、等级、通知（160px）→ 状态（100px）→ 最近时间/年龄（150px）→ 操作（140px fixed-right）。
- 关键列固定：名称与操作；长地址、表达式、响应和备注允许省略，Hover 显示完整值。
- 状态使用 Tag；ID、地址、命令、查询、版本和时间戳使用等宽字体；次级元信息在名称下第二行弱化。
- 行高 44px，Hover `#F8FAFD`；点击名称进入详情，勾选后显示批量操作条。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“新增规则、批量操作”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 数据源、表达式、等级、通知。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：数据源、表达式、等级、通知。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“告警策略管理”。保持列表工作台节奏：筛选、扫描、行内操作、详情回看；不堆叠无业务价值的 KPI。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 86. 告警事件

- **应用 / 路由 / 实现**：监控中心 · `/monitor/alert-events` · `MonitorAlertEvent.vue`

### 1. 页面定位

- 核心用户：平台管理员、值班 SRE。
- 主任务：认领、关闭并追溯告警事件。
- 高频任务：认领、关闭、查看详情、筛选当前范围、进入详情或结果。
- 首要信息：等级、状态、服务、负责人、持续时间、摘要。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去服务、告警、负责人、持续时间。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 服务、告警、负责人、持续时间
├─ Title + 13px description
└─ Primary: 认领、关闭、查看详情
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 服务、告警、负责人、持续时间 | 32px 高；范围变化立即刷新数据 |
| FilterToolbar | 核心内容前 | 浅灰底；Enter 查询，重置不抢主操作 |
| DataTable | 页面主体 | 边框卡片；空态与加载态在区域内出现 |
| DetailDrawer / Dialog | 详情、编辑、日志或确认 | 不离开当前上下文；关闭后保留筛选与滚动位置 |

### 5. 具体视觉规范

- 页面 padding：24px（窄屏 16px）；Header 下间距：16px；区块 gap：16px。
- 标题：22px / 650 / `#18243A`；说明：13px / 400 / `#66758D`；等宽字段使用 `ui-monospace` 12px。
- 卡片：背景 `#FFFFFF`、边框 `#E3E8F0`、圆角 10px、阴影 `0 2px 5px rgba(20,34,58,.035)`。
- 控件：高度 32px、圆角 7px、间距 8px；Hover 背景 `#F8FAFD`；Focus 边框 `#356AE6`。
- 状态仅使用 Success `#1F9D62`、Warning `#D98C16`、Danger `#D94F4F`、Info `#356AE6` 的浅色底 + 深色字。

### 6. 操作层级

- **Primary**：认领、关闭、查看详情，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 表格设计

- 列顺序：名称/标识（220px）→ 服务、告警、负责人、持续时间（160px）→ 状态（100px）→ 最近时间/年龄（150px）→ 操作（140px fixed-right）。
- 关键列固定：名称与操作；长地址、表达式、响应和备注允许省略，Hover 显示完整值。
- 状态使用 Tag；ID、地址、命令、查询、版本和时间戳使用等宽字体；次级元信息在名称下第二行弱化。
- 行高 44px，Hover `#F8FAFD`；点击名称进入详情，勾选后显示批量操作条。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“认领、关闭、查看详情”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 服务、告警、负责人、持续时间。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：服务、告警、负责人、持续时间。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“告警处置工作台”。保持列表工作台节奏：筛选、扫描、行内操作、详情回看；不堆叠无业务价值的 KPI。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 87. 告警屏蔽

- **应用 / 路由 / 实现**：监控中心 · `/monitor/silences` · `MonitorSilenceRule.vue`

### 1. 页面定位

- 核心用户：平台管理员、值班 SRE。
- 主任务：按条件和时间窗屏蔽告警。
- 高频任务：新增屏蔽、筛选当前范围、进入详情或结果。
- 首要信息：规则、匹配条件、开始结束、状态、创建人。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去规则、时间窗、匹配标签。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 规则、时间窗、匹配标签
├─ Title + 13px description
└─ Primary: 新增屏蔽
Primary Content Area\n├─ 核心信息或可视化工作区\n└─ 详情、结果或辅助信息区
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 规则、时间窗、匹配标签 | 32px 高；范围变化立即刷新数据 |
| WorkspaceToolbar | 核心内容前 | 浅灰底；Enter 查询，重置不抢主操作 |
| PrimaryWorkspace | 页面主体 | 边框卡片；空态与加载态在区域内出现 |
| DetailDrawer / Dialog | 详情、编辑、日志或确认 | 不离开当前上下文；关闭后保留筛选与滚动位置 |

### 5. 具体视觉规范

- 页面 padding：24px（窄屏 16px）；Header 下间距：16px；区块 gap：16px。
- 标题：22px / 650 / `#18243A`；说明：13px / 400 / `#66758D`；等宽字段使用 `ui-monospace` 12px。
- 卡片：背景 `#FFFFFF`、边框 `#E3E8F0`、圆角 10px、阴影 `0 2px 5px rgba(20,34,58,.035)`。
- 控件：高度 32px、圆角 7px、间距 8px；Hover 背景 `#F8FAFD`；Focus 边框 `#356AE6`。
- 状态仅使用 Success `#1F9D62`、Warning `#D98C16`、Danger `#D94F4F`、Info `#356AE6` 的浅色底 + 深色字。

### 6. 操作层级

- **Primary**：新增屏蔽，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 内容与状态设计

- 核心内容置于首个可视区域；辅助说明和原始数据放在第二层或可折叠区。
- 数字与时间右对齐或等宽显示；异常状态优先于健康状态。
- 图形、日志、画布和编辑器各自占用明确工作区，避免与普通表单混排。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“新增屏蔽”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 规则、时间窗、匹配标签。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：规则、时间窗、匹配标签。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“静默规则管理”。用当前任务的主信息结构组织首屏，次级信息按阅读路径分层。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 88. 聚合收敛

- **应用 / 路由 / 实现**：监控中心 · `/monitor/aggregations` · `MonitorAggregationRule.vue`

### 1. 页面定位

- 核心用户：平台管理员、值班 SRE。
- 主任务：维护重复告警的分组与收敛窗口。
- 高频任务：新增规则、筛选当前范围、进入详情或结果。
- 首要信息：规则、分组标签、窗口、状态、更新时间。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去标签、窗口、通知。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 标签、窗口、通知
├─ Title + 13px description
└─ Primary: 新增规则
Primary Content Area\n├─ 核心信息或可视化工作区\n└─ 详情、结果或辅助信息区
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 标签、窗口、通知 | 32px 高；范围变化立即刷新数据 |
| WorkspaceToolbar | 核心内容前 | 浅灰底；Enter 查询，重置不抢主操作 |
| PrimaryWorkspace | 页面主体 | 边框卡片；空态与加载态在区域内出现 |
| DetailDrawer / Dialog | 详情、编辑、日志或确认 | 不离开当前上下文；关闭后保留筛选与滚动位置 |

### 5. 具体视觉规范

- 页面 padding：24px（窄屏 16px）；Header 下间距：16px；区块 gap：16px。
- 标题：22px / 650 / `#18243A`；说明：13px / 400 / `#66758D`；等宽字段使用 `ui-monospace` 12px。
- 卡片：背景 `#FFFFFF`、边框 `#E3E8F0`、圆角 10px、阴影 `0 2px 5px rgba(20,34,58,.035)`。
- 控件：高度 32px、圆角 7px、间距 8px；Hover 背景 `#F8FAFD`；Focus 边框 `#356AE6`。
- 状态仅使用 Success `#1F9D62`、Warning `#D98C16`、Danger `#D94F4F`、Info `#356AE6` 的浅色底 + 深色字。

### 6. 操作层级

- **Primary**：新增规则，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 内容与状态设计

- 核心内容置于首个可视区域；辅助说明和原始数据放在第二层或可折叠区。
- 数字与时间右对齐或等宽显示；异常状态优先于健康状态。
- 图形、日志、画布和编辑器各自占用明确工作区，避免与普通表单混排。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“新增规则”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 标签、窗口、通知。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：标签、窗口、通知。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“告警聚合策略”。用当前任务的主信息结构组织首屏，次级信息按阅读路径分层。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 89. 监控大屏

- **应用 / 路由 / 实现**：监控中心 · `/monitor/dashboards` · `MonitorDashboard.vue`

### 1. 页面定位

- 核心用户：平台管理员、值班 SRE。
- 主任务：切换面板并观察关键指标。
- 高频任务：切换面板、全屏、刷新、筛选当前范围、进入详情或结果。
- 首要信息：面板、时间范围、指标、状态、刷新时间。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去时间、面板、指标、健康。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 时间、面板、指标、健康
├─ Title + 13px description
└─ Primary: 切换面板、全屏、刷新
Workspace Context / Control Bar\n├─ 当前范围、刷新或连接状态\n└─ 次级操作\nPrimary Content Area\n├─ 核心信息或可视化工作区\n└─ 详情、结果或辅助信息区
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 时间、面板、指标、健康 | 32px 高；范围变化立即刷新数据 |
| WorkspaceToolbar | 核心内容前 | 浅灰底；Enter 查询，重置不抢主操作 |
| PrimaryWorkspace | 页面主体 | 边框卡片；空态与加载态在区域内出现 |
| DetailDrawer / Dialog | 详情、编辑、日志或确认 | 不离开当前上下文；关闭后保留筛选与滚动位置 |

### 5. 具体视觉规范

- 页面 padding：24px（窄屏 16px）；Header 下间距：16px；区块 gap：16px。
- 标题：22px / 650 / `#18243A`；说明：13px / 400 / `#66758D`；等宽字段使用 `ui-monospace` 12px。
- 卡片：背景 `#FFFFFF`、边框 `#E3E8F0`、圆角 10px、阴影 `0 2px 5px rgba(20,34,58,.035)`。
- 控件：高度 32px、圆角 7px、间距 8px；Hover 背景 `#F8FAFD`；Focus 边框 `#356AE6`。
- 状态仅使用 Success `#1F9D62`、Warning `#D98C16`、Danger `#D94F4F`、Info `#356AE6` 的浅色底 + 深色字。

### 6. 操作层级

- **Primary**：切换面板、全屏、刷新，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 内容与状态设计

- 核心内容置于首个可视区域；辅助说明和原始数据放在第二层或可折叠区。
- 数字与时间右对齐或等宽显示；异常状态优先于健康状态。
- 图形、日志、画布和编辑器各自占用明确工作区，避免与普通表单混排。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“切换面板、全屏、刷新”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 时间、面板、指标、健康。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：时间、面板、指标、健康。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“指标面板工作区”。主工作区优先，工具栏与辅助面板可折叠；不将其改造成 KPI + 表格页面。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 90. 巡检大屏

- **应用 / 路由 / 实现**：监控中心 · `/monitor/inspections` · `MonitorDashboard.vue`

### 1. 页面定位

- 核心用户：平台管理员、值班 SRE。
- 主任务：按健康状态巡检监控面板。
- 高频任务：筛选、导出报告、筛选当前范围、进入详情或结果。
- 首要信息：面板、健康、异常项、检查时间、负责人。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去时间、面板、健康、异常。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 时间、面板、健康、异常
├─ Title + 13px description
└─ Primary: 筛选、导出报告
Workspace Context / Control Bar\n├─ 当前范围、刷新或连接状态\n└─ 次级操作\nPrimary Content Area\n├─ 核心信息或可视化工作区\n└─ 详情、结果或辅助信息区
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 时间、面板、健康、异常 | 32px 高；范围变化立即刷新数据 |
| WorkspaceToolbar | 核心内容前 | 浅灰底；Enter 查询，重置不抢主操作 |
| PrimaryWorkspace | 页面主体 | 边框卡片；空态与加载态在区域内出现 |
| DetailDrawer / Dialog | 详情、编辑、日志或确认 | 不离开当前上下文；关闭后保留筛选与滚动位置 |

### 5. 具体视觉规范

- 页面 padding：24px（窄屏 16px）；Header 下间距：16px；区块 gap：16px。
- 标题：22px / 650 / `#18243A`；说明：13px / 400 / `#66758D`；等宽字段使用 `ui-monospace` 12px。
- 卡片：背景 `#FFFFFF`、边框 `#E3E8F0`、圆角 10px、阴影 `0 2px 5px rgba(20,34,58,.035)`。
- 控件：高度 32px、圆角 7px、间距 8px；Hover 背景 `#F8FAFD`；Focus 边框 `#356AE6`。
- 状态仅使用 Success `#1F9D62`、Warning `#D98C16`、Danger `#D94F4F`、Info `#356AE6` 的浅色底 + 深色字。

### 6. 操作层级

- **Primary**：筛选、导出报告，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 内容与状态设计

- 核心内容置于首个可视区域；辅助说明和原始数据放在第二层或可折叠区。
- 数字与时间右对齐或等宽显示；异常状态优先于健康状态。
- 图形、日志、画布和编辑器各自占用明确工作区，避免与普通表单混排。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“筛选、导出报告”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 时间、面板、健康、异常。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：时间、面板、健康、异常。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“巡检结果工作区”。主工作区优先，工具栏与辅助面板可折叠；不将其改造成 KPI + 表格页面。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

