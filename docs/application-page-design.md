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

## 14. 主机管理

- **应用 / 路由 / 实现**：资产管理 · `/assets/server/hosts` · `Host.vue`

### 1. 页面定位

- 核心用户：平台管理员、对应业务负责人。
- 主任务：筛选主机并执行查看、连接和管理。
- 高频任务：新增主机、筛选当前范围、进入详情或结果。
- 首要信息：主机名、IP、环境、在线、CPU/内存。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去环境、主机组、状态、指标。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 环境、主机组、状态、指标
├─ Title + 13px description
└─ Primary: 新增主机
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 环境、主机组、状态、指标 | 32px 高；范围变化立即刷新数据 |
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

- **Primary**：新增主机，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 表格设计

- 列顺序：名称/标识（220px）→ 环境、主机组、状态、指标（160px）→ 状态（100px）→ 最近时间/年龄（150px）→ 操作（140px fixed-right）。
- 关键列固定：名称与操作；长地址、表达式、响应和备注允许省略，Hover 显示完整值。
- 状态使用 Tag；ID、地址、命令、查询、版本和时间戳使用等宽字体；次级元信息在名称下第二行弱化。
- 行高 44px，Hover `#F8FAFD`；点击名称进入详情，勾选后显示批量操作条。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“新增主机”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 环境、主机组、状态、指标。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：环境、主机组、状态、指标。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“主机资源工作台”。保持列表工作台节奏：筛选、扫描、行内操作、详情回看；不堆叠无业务价值的 KPI。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 15. 主机组管理

- **应用 / 路由 / 实现**：资产管理 · `/assets/server/groups` · `HostGroup.vue`

### 1. 页面定位

- 核心用户：平台管理员、对应业务负责人。
- 主任务：维护批量运维与授权边界。
- 高频任务：新增主机组、筛选当前范围、进入详情或结果。
- 首要信息：分组、成员数、环境、描述。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去环境、成员数、负责人。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 环境、成员数、负责人
├─ Title + 13px description
└─ Primary: 新增主机组
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 环境、成员数、负责人 | 32px 高；范围变化立即刷新数据 |
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

- **Primary**：新增主机组，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 表格设计

- 列顺序：名称/标识（220px）→ 环境、成员数、负责人（160px）→ 状态（100px）→ 最近时间/年龄（150px）→ 操作（140px fixed-right）。
- 关键列固定：名称与操作；长地址、表达式、响应和备注允许省略，Hover 显示完整值。
- 状态使用 Tag；ID、地址、命令、查询、版本和时间戳使用等宽字体；次级元信息在名称下第二行弱化。
- 行高 44px，Hover `#F8FAFD`；点击名称进入详情，勾选后显示批量操作条。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“新增主机组”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 环境、成员数、负责人。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：环境、成员数、负责人。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“资源分组维护”。保持列表工作台节奏：筛选、扫描、行内操作、详情回看；不堆叠无业务价值的 KPI。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 16. 凭据管理

- **应用 / 路由 / 实现**：资产管理 · `/assets/server/credentials` · `Credential.vue`

### 1. 页面定位

- 核心用户：平台管理员、对应业务负责人。
- 主任务：维护密码和密钥凭据。
- 高频任务：新增凭据、筛选当前范围、进入详情或结果。
- 首要信息：名称、类型、关联范围、状态、更新时间。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去凭据类型、关联资源、审计。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 凭据类型、关联资源、审计
├─ Title + 13px description
└─ Primary: 新增凭据
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 凭据类型、关联资源、审计 | 32px 高；范围变化立即刷新数据 |
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

- **Primary**：新增凭据，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 表格设计

- 列顺序：名称/标识（220px）→ 凭据类型、关联资源、审计（160px）→ 状态（100px）→ 最近时间/年龄（150px）→ 操作（140px fixed-right）。
- 关键列固定：名称与操作；长地址、表达式、响应和备注允许省略，Hover 显示完整值。
- 状态使用 Tag；ID、地址、命令、查询、版本和时间戳使用等宽字体；次级元信息在名称下第二行弱化。
- 行高 44px，Hover `#F8FAFD`；点击名称进入详情，勾选后显示批量操作条。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“新增凭据”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 凭据类型、关联资源、审计。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：凭据类型、关联资源、审计。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“访问凭据管理”。保持列表工作台节奏：筛选、扫描、行内操作、详情回看；不堆叠无业务价值的 KPI。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 17. 云账号管理

- **应用 / 路由 / 实现**：资产管理 · `/assets/server/cloud-accounts` · `CloudAccount.vue`

### 1. 页面定位

- 核心用户：平台管理员、对应业务负责人。
- 主任务：维护云账号并检查连通性。
- 高频任务：新增账号、测试连接、筛选当前范围、进入详情或结果。
- 首要信息：厂商、账号、区域、同步状态、更新时间。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去厂商、区域、连接状态。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 厂商、区域、连接状态
├─ Title + 13px description
└─ Primary: 新增账号、测试连接
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 厂商、区域、连接状态 | 32px 高；范围变化立即刷新数据 |
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

- **Primary**：新增账号、测试连接，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 表格设计

- 列顺序：名称/标识（220px）→ 厂商、区域、连接状态（160px）→ 状态（100px）→ 最近时间/年龄（150px）→ 操作（140px fixed-right）。
- 关键列固定：名称与操作；长地址、表达式、响应和备注允许省略，Hover 显示完整值。
- 状态使用 Tag；ID、地址、命令、查询、版本和时间戳使用等宽字体；次级元信息在名称下第二行弱化。
- 行高 44px，Hover `#F8FAFD`；点击名称进入详情，勾选后显示批量操作条。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“新增账号、测试连接”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 厂商、区域、连接状态。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：厂商、区域、连接状态。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“云接入配置”。保持列表工作台节奏：筛选、扫描、行内操作、详情回看；不堆叠无业务价值的 KPI。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 18. 数据库管理

- **应用 / 路由 / 实现**：资产管理 · `/assets/databases` · `Database.vue`

### 1. 页面定位

- 核心用户：平台管理员、对应业务负责人。
- 主任务：定位连接状态并进入工作台。
- 高频任务：新增数据库、筛选当前范围、进入详情或结果。
- 首要信息：数据库、地址、类型、环境、连接状态。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去环境、类型、连接、备份。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 环境、类型、连接、备份
├─ Title + 13px description
└─ Primary: 新增数据库
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 环境、类型、连接、备份 | 32px 高；范围变化立即刷新数据 |
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

- **Primary**：新增数据库，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 表格设计

- 列顺序：名称/标识（220px）→ 环境、类型、连接、备份（160px）→ 状态（100px）→ 最近时间/年龄（150px）→ 操作（140px fixed-right）。
- 关键列固定：名称与操作；长地址、表达式、响应和备注允许省略，Hover 显示完整值。
- 状态使用 Tag；ID、地址、命令、查询、版本和时间戳使用等宽字体；次级元信息在名称下第二行弱化。
- 行高 44px，Hover `#F8FAFD`；点击名称进入详情，勾选后显示批量操作条。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“新增数据库”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 环境、类型、连接、备份。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：环境、类型、连接、备份。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“数据库资源管理”。保持列表工作台节奏：筛选、扫描、行内操作、详情回看；不堆叠无业务价值的 KPI。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 19. 数据库详情

- **应用 / 路由 / 实现**：资产管理 · `/assets/databases/:id/detail` · `AssetDetail.vue`

### 1. 页面定位

- 核心用户：平台管理员、对应业务负责人。
- 主任务：查看配置、关联和健康信息。
- 高频任务：进入工作台、编辑、筛选当前范围、进入详情或结果。
- 首要信息：连接、版本、环境、关联服务、事件。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去环境、连接、服务、告警。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 环境、连接、服务、告警
├─ Title + 13px description
└─ Primary: 进入工作台、编辑
Workspace Context / Control Bar\n├─ 当前范围、刷新或连接状态\n└─ 次级操作\nFilter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 环境、连接、服务、告警 | 32px 高；范围变化立即刷新数据 |
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

- **Primary**：进入工作台、编辑，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 表格设计

- 列顺序：名称/标识（220px）→ 环境、连接、服务、告警（160px）→ 状态（100px）→ 最近时间/年龄（150px）→ 操作（140px fixed-right）。
- 关键列固定：名称与操作；长地址、表达式、响应和备注允许省略，Hover 显示完整值。
- 状态使用 Tag；ID、地址、命令、查询、版本和时间戳使用等宽字体；次级元信息在名称下第二行弱化。
- 行高 44px，Hover `#F8FAFD`；点击名称进入详情，勾选后显示批量操作条。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“进入工作台、编辑”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 环境、连接、服务、告警。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：环境、连接、服务、告警。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“数据库资源详情”。主工作区优先，工具栏与辅助面板可折叠；不将其改造成 KPI + 表格页面。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 20. DBMS 工作台

- **应用 / 路由 / 实现**：资产管理 · `/assets/databases/:id/workbench` · `DatabaseWorkbench.vue`

### 1. 页面定位

- 核心用户：平台管理员、对应业务负责人。
- 主任务：查询、浏览结构并安全执行 SQL。
- 高频任务：执行查询、筛选当前范围、进入详情或结果。
- 首要信息：连接树、Schema、编辑器、结果、执行风险。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去数据库、Schema、会话、风险、审计。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 数据库、Schema、会话、风险、审计
├─ Title + 13px description
└─ Primary: 执行查询
Workspace Context / Control Bar\n├─ 当前范围、刷新或连接状态\n└─ 次级操作\nPrimary Content Area\n├─ 核心信息或可视化工作区\n└─ 详情、结果或辅助信息区
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 数据库、Schema、会话、风险、审计 | 32px 高；范围变化立即刷新数据 |
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

- **Primary**：执行查询，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 内容与状态设计

- 核心内容置于首个可视区域；辅助说明和原始数据放在第二层或可折叠区。
- 数字与时间右对齐或等宽显示；异常状态优先于健康状态。
- 图形、日志、画布和编辑器各自占用明确工作区，避免与普通表单混排。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“执行查询”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 数据库、Schema、会话、风险、审计。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：数据库、Schema、会话、风险、审计。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“数据库操作工作台”。主工作区优先，工具栏与辅助面板可折叠；不将其改造成 KPI + 表格页面。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 21. 数据导入

- **应用 / 路由 / 实现**：资产管理 · `/assets/databases/import` · `DatabaseImport.vue`

### 1. 页面定位

- 核心用户：平台管理员、对应业务负责人。
- 主任务：选择源文件、目标库并预检后导入。
- 高频任务：开始导入、筛选当前范围、进入详情或结果。
- 首要信息：源文件、目标库、预检结果、历史记录。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去数据库、目标表、预检、任务。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 数据库、目标表、预检、任务
├─ Title + 13px description
└─ Primary: 开始导入
Primary Content Area\n├─ 核心信息或可视化工作区\n└─ 详情、结果或辅助信息区
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 数据库、目标表、预检、任务 | 32px 高；范围变化立即刷新数据 |
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

- **Primary**：开始导入，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 内容与状态设计

- 核心内容置于首个可视区域；辅助说明和原始数据放在第二层或可折叠区。
- 数字与时间右对齐或等宽显示；异常状态优先于健康状态。
- 图形、日志、画布和编辑器各自占用明确工作区，避免与普通表单混排。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“开始导入”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 数据库、目标表、预检、任务。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：数据库、目标表、预检、任务。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“数据导入工作台”。用当前任务的主信息结构组织首屏，次级信息按阅读路径分层。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 22. 备份管理

- **应用 / 路由 / 实现**：资产管理 · `/assets/databases/backups` · `DatabaseBackup.vue`

### 1. 页面定位

- 核心用户：平台管理员、对应业务负责人。
- 主任务：查看备份、创建备份或恢复。
- 高频任务：创建备份、筛选当前范围、进入详情或结果。
- 首要信息：数据库、备份大小、时间、状态、保留期。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去数据库、备份、恢复、风险。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 数据库、备份、恢复、风险
├─ Title + 13px description
└─ Primary: 创建备份
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 数据库、备份、恢复、风险 | 32px 高；范围变化立即刷新数据 |
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

- **Primary**：创建备份，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 表格设计

- 列顺序：名称/标识（220px）→ 数据库、备份、恢复、风险（160px）→ 状态（100px）→ 最近时间/年龄（150px）→ 操作（140px fixed-right）。
- 关键列固定：名称与操作；长地址、表达式、响应和备注允许省略，Hover 显示完整值。
- 状态使用 Tag；ID、地址、命令、查询、版本和时间戳使用等宽字体；次级元信息在名称下第二行弱化。
- 行高 44px，Hover `#F8FAFD`；点击名称进入详情，勾选后显示批量操作条。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“创建备份”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 数据库、备份、恢复、风险。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：数据库、备份、恢复、风险。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“备份与恢复管理”。保持列表工作台节奏：筛选、扫描、行内操作、详情回看；不堆叠无业务价值的 KPI。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 23. 网关管理

- **应用 / 路由 / 实现**：资产管理 · `/assets/gateways` · `Gateway.vue`

### 1. 页面定位

- 核心用户：平台管理员、对应业务负责人。
- 主任务：维护跳板网关与连通状态。
- 高频任务：新增网关、测试连接、筛选当前范围、进入详情或结果。
- 首要信息：网关、地址、可用性、关联资源、更新时间。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去网关、连接、关联范围。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 网关、连接、关联范围
├─ Title + 13px description
└─ Primary: 新增网关、测试连接
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 网关、连接、关联范围 | 32px 高；范围变化立即刷新数据 |
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

- **Primary**：新增网关、测试连接，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 表格设计

- 列顺序：名称/标识（220px）→ 网关、连接、关联范围（160px）→ 状态（100px）→ 最近时间/年龄（150px）→ 操作（140px fixed-right）。
- 关键列固定：名称与操作；长地址、表达式、响应和备注允许省略，Hover 显示完整值。
- 状态使用 Tag；ID、地址、命令、查询、版本和时间戳使用等宽字体；次级元信息在名称下第二行弱化。
- 行高 44px，Hover `#F8FAFD`；点击名称进入详情，勾选后显示批量操作条。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“新增网关、测试连接”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 网关、连接、关联范围。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：网关、连接、关联范围。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“访问网关管理”。保持列表工作台节奏：筛选、扫描、行内操作、详情回看；不堆叠无业务价值的 KPI。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 24. 环境模型

- **应用 / 路由 / 实现**：资产管理 · `/assets/environments` · `OpsEnvironment.vue`

### 1. 页面定位

- 核心用户：平台管理员、对应业务负责人。
- 主任务：管理环境与资源归属。
- 高频任务：新增环境、筛选当前范围、进入详情或结果。
- 首要信息：环境名、等级、区域、资源计数、状态。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去环境、区域、资源范围。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 环境、区域、资源范围
├─ Title + 13px description
└─ Primary: 新增环境
Primary Content Area\n├─ 核心信息或可视化工作区\n└─ 详情、结果或辅助信息区
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 环境、区域、资源范围 | 32px 高；范围变化立即刷新数据 |
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

- **Primary**：新增环境，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 内容与状态设计

- 核心内容置于首个可视区域；辅助说明和原始数据放在第二层或可折叠区。
- 数字与时间右对齐或等宽显示；异常状态优先于健康状态。
- 图形、日志、画布和编辑器各自占用明确工作区，避免与普通表单混排。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“新增环境”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 环境、区域、资源范围。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：环境、区域、资源范围。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“环境模型维护”。用当前任务的主信息结构组织首屏，次级信息按阅读路径分层。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 25. 服务管理

- **应用 / 路由 / 实现**：容器管理 · `/containers/services` · `Application.vue`

### 1. 页面定位

- 核心用户：平台管理员、对应业务负责人。
- 主任务：发现并维护业务服务与工作负载。
- 高频任务：新建服务、查看详情、筛选当前范围、进入详情或结果。
- 首要信息：服务、环境、集群、命名空间、健康。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去服务、集群、命名空间、健康。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 服务、集群、命名空间、健康
├─ Title + 13px description
└─ Primary: 新建服务、查看详情
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 服务、集群、命名空间、健康 | 32px 高；范围变化立即刷新数据 |
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

- **Primary**：新建服务、查看详情，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 表格设计

- 列顺序：名称/标识（220px）→ 服务、集群、命名空间、健康（160px）→ 状态（100px）→ 最近时间/年龄（150px）→ 操作（140px fixed-right）。
- 关键列固定：名称与操作；长地址、表达式、响应和备注允许省略，Hover 显示完整值。
- 状态使用 Tag；ID、地址、命令、查询、版本和时间戳使用等宽字体；次级元信息在名称下第二行弱化。
- 行高 44px，Hover `#F8FAFD`；点击名称进入详情，勾选后显示批量操作条。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“新建服务、查看详情”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 服务、集群、命名空间、健康。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：服务、集群、命名空间、健康。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“服务资源目录”。保持列表工作台节奏：筛选、扫描、行内操作、详情回看；不堆叠无业务价值的 KPI。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 26. 服务健康诊断

- **应用 / 路由 / 实现**：容器管理 · `/containers/services/health-diagnosis` · `ServiceHealthDiagnosis.vue`

### 1. 页面定位

- 核心用户：平台管理员、对应业务负责人。
- 主任务：选择目标并观察 JVM/容器诊断结果。
- 高频任务：开始诊断、筛选当前范围、进入详情或结果。
- 首要信息：服务、Pod、容器、进程、诊断输出。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去服务、Pod、进程、健康、诊断。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 服务、Pod、进程、健康、诊断
├─ Title + 13px description
└─ Primary: 开始诊断
Workspace Context / Control Bar\n├─ 当前范围、刷新或连接状态\n└─ 次级操作\nFilter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 服务、Pod、进程、健康、诊断 | 32px 高；范围变化立即刷新数据 |
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

- **Primary**：开始诊断，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 表格设计

- 列顺序：名称/标识（220px）→ 服务、Pod、进程、健康、诊断（160px）→ 状态（100px）→ 最近时间/年龄（150px）→ 操作（140px fixed-right）。
- 关键列固定：名称与操作；长地址、表达式、响应和备注允许省略，Hover 显示完整值。
- 状态使用 Tag；ID、地址、命令、查询、版本和时间戳使用等宽字体；次级元信息在名称下第二行弱化。
- 行高 44px，Hover `#F8FAFD`；点击名称进入详情，勾选后显示批量操作条。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“开始诊断”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 服务、Pod、进程、健康、诊断。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：服务、Pod、进程、健康、诊断。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“运行时诊断工作台”。主工作区优先，工具栏与辅助面板可折叠；不将其改造成 KPI + 表格页面。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 27. 服务资源拓扑

- **应用 / 路由 / 实现**：容器管理 · `/containers/services/topology` · `ApplicationTopology.vue`

### 1. 页面定位

- 核心用户：平台管理员、对应业务负责人。
- 主任务：查看服务、工作负载和资源关系。
- 高频任务：查看节点详情、筛选当前范围、进入详情或结果。
- 首要信息：服务节点、实例、健康、依赖方向。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去服务、工作负载、依赖、健康。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 服务、工作负载、依赖、健康
├─ Title + 13px description
└─ Primary: 查看节点详情
Workspace Context / Control Bar\n├─ 当前范围、刷新或连接状态\n└─ 次级操作\nFilter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 服务、工作负载、依赖、健康 | 32px 高；范围变化立即刷新数据 |
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

- **Primary**：查看节点详情，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 表格设计

- 列顺序：名称/标识（220px）→ 服务、工作负载、依赖、健康（160px）→ 状态（100px）→ 最近时间/年龄（150px）→ 操作（140px fixed-right）。
- 关键列固定：名称与操作；长地址、表达式、响应和备注允许省略，Hover 显示完整值。
- 状态使用 Tag；ID、地址、命令、查询、版本和时间戳使用等宽字体；次级元信息在名称下第二行弱化。
- 行高 44px，Hover `#F8FAFD`；点击名称进入详情，勾选后显示批量操作条。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“查看节点详情”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 服务、工作负载、依赖、健康。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：服务、工作负载、依赖、健康。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“服务依赖拓扑”。主工作区优先，工具栏与辅助面板可折叠；不将其改造成 KPI + 表格页面。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 28. 服务详情

- **应用 / 路由 / 实现**：容器管理 · `/containers/services/workload` · `ServiceWorkloadDetail.vue`

### 1. 页面定位

- 核心用户：平台管理员、对应业务负责人。
- 主任务：查看版本、Pod、ReplicaSet 与资源关系。
- 高频任务：回滚、查看日志、筛选当前范围、进入详情或结果。
- 首要信息：工作负载、版本、Pod、就绪、事件。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去服务、工作负载、版本、Pod。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 服务、工作负载、版本、Pod
├─ Title + 13px description
└─ Primary: 回滚、查看日志
Workspace Context / Control Bar\n├─ 当前范围、刷新或连接状态\n└─ 次级操作\nFilter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 服务、工作负载、版本、Pod | 32px 高；范围变化立即刷新数据 |
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

- **Primary**：回滚、查看日志，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 表格设计

- 列顺序：名称/标识（220px）→ 服务、工作负载、版本、Pod（160px）→ 状态（100px）→ 最近时间/年龄（150px）→ 操作（140px fixed-right）。
- 关键列固定：名称与操作；长地址、表达式、响应和备注允许省略，Hover 显示完整值。
- 状态使用 Tag；ID、地址、命令、查询、版本和时间戳使用等宽字体；次级元信息在名称下第二行弱化。
- 行高 44px，Hover `#F8FAFD`；点击名称进入详情，勾选后显示批量操作条。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“回滚、查看日志”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 服务、工作负载、版本、Pod。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：服务、工作负载、版本、Pod。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“服务工作负载详情”。主工作区优先，工具栏与辅助面板可折叠；不将其改造成 KPI + 表格页面。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 29. 服务日志

- **应用 / 路由 / 实现**：容器管理 · `/containers/services/logs` · `ServiceWorkloadLogs.vue`

### 1. 页面定位

- 核心用户：平台管理员、对应业务负责人。
- 主任务：按 Pod/容器检索运行日志。
- 高频任务：刷新、下载、筛选当前范围、进入详情或结果。
- 首要信息：服务、Pod、容器、时间范围、日志流。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去服务、Pod、容器、时间。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 服务、Pod、容器、时间
├─ Title + 13px description
└─ Primary: 刷新、下载
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 服务、Pod、容器、时间 | 32px 高；范围变化立即刷新数据 |
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

- **Primary**：刷新、下载，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 表格设计

- 列顺序：名称/标识（220px）→ 服务、Pod、容器、时间（160px）→ 状态（100px）→ 最近时间/年龄（150px）→ 操作（140px fixed-right）。
- 关键列固定：名称与操作；长地址、表达式、响应和备注允许省略，Hover 显示完整值。
- 状态使用 Tag；ID、地址、命令、查询、版本和时间戳使用等宽字体；次级元信息在名称下第二行弱化。
- 行高 44px，Hover `#F8FAFD`；点击名称进入详情，勾选后显示批量操作条。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“刷新、下载”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 服务、Pod、容器、时间。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：服务、Pod、容器、时间。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“服务日志查看”。保持列表工作台节奏：筛选、扫描、行内操作、详情回看；不堆叠无业务价值的 KPI。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 30. 集群管理

- **应用 / 路由 / 实现**：容器管理 · `/containers/k8s/clusters` · `K8sClusterManage.vue`

### 1. 页面定位

- 核心用户：平台管理员、对应业务负责人。
- 主任务：维护集群并检查连接状态。
- 高频任务：新增集群、测试连接、筛选当前范围、进入详情或结果。
- 首要信息：集群、API 地址、版本、节点、连接状态。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去集群、版本、节点、连接。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 集群、版本、节点、连接
├─ Title + 13px description
└─ Primary: 新增集群、测试连接
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 集群、版本、节点、连接 | 32px 高；范围变化立即刷新数据 |
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

- **Primary**：新增集群、测试连接，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 表格设计

- 列顺序：名称/标识（220px）→ 集群、版本、节点、连接（160px）→ 状态（100px）→ 最近时间/年龄（150px）→ 操作（140px fixed-right）。
- 关键列固定：名称与操作；长地址、表达式、响应和备注允许省略，Hover 显示完整值。
- 状态使用 Tag；ID、地址、命令、查询、版本和时间戳使用等宽字体；次级元信息在名称下第二行弱化。
- 行高 44px，Hover `#F8FAFD`；点击名称进入详情，勾选后显示批量操作条。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“新增集群、测试连接”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 集群、版本、节点、连接。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：集群、版本、节点、连接。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“集群接入管理”。保持列表工作台节奏：筛选、扫描、行内操作、详情回看；不堆叠无业务价值的 KPI。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 31. 集群详情

- **应用 / 路由 / 实现**：容器管理 · `/containers/k8s/clusters/:id/detail` · `AssetDetail.vue`

### 1. 页面定位

- 核心用户：平台管理员、对应业务负责人。
- 主任务：查看集群属性、健康和关联服务。
- 高频任务：进入集群概览、编辑、筛选当前范围、进入详情或结果。
- 首要信息：集群、版本、节点、命名空间、事件。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去集群、版本、节点、健康。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 集群、版本、节点、健康
├─ Title + 13px description
└─ Primary: 进入集群概览、编辑
Workspace Context / Control Bar\n├─ 当前范围、刷新或连接状态\n└─ 次级操作\nPrimary Content Area\n├─ 核心信息或可视化工作区\n└─ 详情、结果或辅助信息区
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 集群、版本、节点、健康 | 32px 高；范围变化立即刷新数据 |
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

- **Primary**：进入集群概览、编辑，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 内容与状态设计

- 核心内容置于首个可视区域；辅助说明和原始数据放在第二层或可折叠区。
- 数字与时间右对齐或等宽显示；异常状态优先于健康状态。
- 图形、日志、画布和编辑器各自占用明确工作区，避免与普通表单混排。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“进入集群概览、编辑”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 集群、版本、节点、健康。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：集群、版本、节点、健康。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“集群资源详情”。主工作区优先，工具栏与辅助面板可折叠；不将其改造成 KPI + 表格页面。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 32. 集群概览

- **应用 / 路由 / 实现**：容器管理 · `/containers/k8s/overview` · `K8s.vue`

### 1. 页面定位

- 核心用户：平台管理员、对应业务负责人。
- 主任务：快速判断集群容量、节点和工作负载健康。
- 高频任务：刷新、切换集群、筛选当前范围、进入详情或结果。
- 首要信息：集群、命名空间、节点、工作负载、告警。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去集群、命名空间、健康、事件。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 集群、命名空间、健康、事件
├─ Title + 13px description
└─ Primary: 刷新、切换集群
Workspace Context / Control Bar\n├─ 当前范围、刷新或连接状态\n└─ 次级操作\nPrimary Content Area\n├─ 核心信息或可视化工作区\n└─ 详情、结果或辅助信息区
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 集群、命名空间、健康、事件 | 32px 高；范围变化立即刷新数据 |
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

- **Primary**：刷新、切换集群，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 内容与状态设计

- 核心内容置于首个可视区域；辅助说明和原始数据放在第二层或可折叠区。
- 数字与时间右对齐或等宽显示；异常状态优先于健康状态。
- 图形、日志、画布和编辑器各自占用明确工作区，避免与普通表单混排。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“刷新、切换集群”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 集群、命名空间、健康、事件。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：集群、命名空间、健康、事件。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“集群运行总览”。主工作区优先，工具栏与辅助面板可折叠；不将其改造成 KPI + 表格页面。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 33. 节点管理

- **应用 / 路由 / 实现**：容器管理 · `/containers/k8s/nodes` · `K8s.vue`

### 1. 页面定位

- 核心用户：平台管理员、对应业务负责人。
- 主任务：筛选节点并查看资源与条件。
- 高频任务：刷新、查看详情、筛选当前范围、进入详情或结果。
- 首要信息：节点、Ready、CPU、内存、Pod 数、版本。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去集群、节点、资源、条件。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 集群、节点、资源、条件
├─ Title + 13px description
└─ Primary: 刷新、查看详情
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 集群、节点、资源、条件 | 32px 高；范围变化立即刷新数据 |
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

- **Primary**：刷新、查看详情，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 表格设计

- 列顺序：名称/标识（220px）→ 集群、节点、资源、条件（160px）→ 状态（100px）→ 最近时间/年龄（150px）→ 操作（140px fixed-right）。
- 关键列固定：名称与操作；长地址、表达式、响应和备注允许省略，Hover 显示完整值。
- 状态使用 Tag；ID、地址、命令、查询、版本和时间戳使用等宽字体；次级元信息在名称下第二行弱化。
- 行高 44px，Hover `#F8FAFD`；点击名称进入详情，勾选后显示批量操作条。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“刷新、查看详情”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 集群、节点、资源、条件。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：集群、节点、资源、条件。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“K8s 节点工作台”。保持列表工作台节奏：筛选、扫描、行内操作、详情回看；不堆叠无业务价值的 KPI。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 34. 命名空间

- **应用 / 路由 / 实现**：容器管理 · `/containers/k8s/namespaces` · `K8s.vue`

### 1. 页面定位

- 核心用户：平台管理员、对应业务负责人。
- 主任务：查看资源隔离范围与配额。
- 高频任务：新建、查看详情、筛选当前范围、进入详情或结果。
- 首要信息：命名空间、状态、资源数、配额、年龄。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去集群、命名空间、配额。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 集群、命名空间、配额
├─ Title + 13px description
└─ Primary: 新建、查看详情
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 集群、命名空间、配额 | 32px 高；范围变化立即刷新数据 |
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

- **Primary**：新建、查看详情，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 表格设计

- 列顺序：名称/标识（220px）→ 集群、命名空间、配额（160px）→ 状态（100px）→ 最近时间/年龄（150px）→ 操作（140px fixed-right）。
- 关键列固定：名称与操作；长地址、表达式、响应和备注允许省略，Hover 显示完整值。
- 状态使用 Tag；ID、地址、命令、查询、版本和时间戳使用等宽字体；次级元信息在名称下第二行弱化。
- 行高 44px，Hover `#F8FAFD`；点击名称进入详情，勾选后显示批量操作条。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“新建、查看详情”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 集群、命名空间、配额。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：集群、命名空间、配额。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“命名空间管理”。保持列表工作台节奏：筛选、扫描、行内操作、详情回看；不堆叠无业务价值的 KPI。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 35. 工作负载

- **应用 / 路由 / 实现**：容器管理 · `/containers/k8s/workloads` · `K8s.vue`

### 1. 页面定位

- 核心用户：平台管理员、对应业务负责人。
- 主任务：按类型与命名空间观察发布健康。
- 高频任务：刷新、扩缩容、筛选当前范围、进入详情或结果。
- 首要信息：名称、类型、Ready、镜像、重启、年龄。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去集群、命名空间、类型、版本。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 集群、命名空间、类型、版本
├─ Title + 13px description
└─ Primary: 刷新、扩缩容
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 集群、命名空间、类型、版本 | 32px 高；范围变化立即刷新数据 |
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

- **Primary**：刷新、扩缩容，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 表格设计

- 列顺序：名称/标识（220px）→ 集群、命名空间、类型、版本（160px）→ 状态（100px）→ 最近时间/年龄（150px）→ 操作（140px fixed-right）。
- 关键列固定：名称与操作；长地址、表达式、响应和备注允许省略，Hover 显示完整值。
- 状态使用 Tag；ID、地址、命令、查询、版本和时间戳使用等宽字体；次级元信息在名称下第二行弱化。
- 行高 44px，Hover `#F8FAFD`；点击名称进入详情，勾选后显示批量操作条。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“刷新、扩缩容”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 集群、命名空间、类型、版本。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：集群、命名空间、类型、版本。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“工作负载资源工作台”。保持列表工作台节奏：筛选、扫描、行内操作、详情回看；不堆叠无业务价值的 KPI。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 36. Pod 管理

- **应用 / 路由 / 实现**：容器管理 · `/containers/k8s/pods` · `K8s.vue`

### 1. 页面定位

- 核心用户：平台管理员、对应业务负责人。
- 主任务：筛选 Pod 并进入日志、终端和 YAML。
- 高频任务：刷新、批量操作、筛选当前范围、进入详情或结果。
- 首要信息：名称、Ready、状态、重启、节点、IP、年龄。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去集群、命名空间、Pod、节点、状态。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 集群、命名空间、Pod、节点、状态
├─ Title + 13px description
└─ Primary: 刷新、批量操作
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 集群、命名空间、Pod、节点、状态 | 32px 高；范围变化立即刷新数据 |
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

- **Primary**：刷新、批量操作，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 表格设计

- 列顺序：名称/标识（220px）→ 集群、命名空间、Pod、节点、状态（160px）→ 状态（100px）→ 最近时间/年龄（150px）→ 操作（140px fixed-right）。
- 关键列固定：名称与操作；长地址、表达式、响应和备注允许省略，Hover 显示完整值。
- 状态使用 Tag；ID、地址、命令、查询、版本和时间戳使用等宽字体；次级元信息在名称下第二行弱化。
- 行高 44px，Hover `#F8FAFD`；点击名称进入详情，勾选后显示批量操作条。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“刷新、批量操作”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 集群、命名空间、Pod、节点、状态。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：集群、命名空间、Pod、节点、状态。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“Pod 资源工作台”。保持列表工作台节奏：筛选、扫描、行内操作、详情回看；不堆叠无业务价值的 KPI。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 37. Service

- **应用 / 路由 / 实现**：容器管理 · `/containers/k8s/services` · `K8s.vue`

### 1. 页面定位

- 核心用户：平台管理员、对应业务负责人。
- 主任务：查看 Service、Selector 与端点。
- 高频任务：刷新、查看 YAML、筛选当前范围、进入详情或结果。
- 首要信息：名称、类型、Cluster IP、端口、端点、年龄。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去集群、命名空间、端点。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 集群、命名空间、端点
├─ Title + 13px description
└─ Primary: 刷新、查看 YAML
Primary Content Area\n├─ 核心信息或可视化工作区\n└─ 详情、结果或辅助信息区
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 集群、命名空间、端点 | 32px 高；范围变化立即刷新数据 |
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

- **Primary**：刷新、查看 YAML，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 内容与状态设计

- 核心内容置于首个可视区域；辅助说明和原始数据放在第二层或可折叠区。
- 数字与时间右对齐或等宽显示；异常状态优先于健康状态。
- 图形、日志、画布和编辑器各自占用明确工作区，避免与普通表单混排。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“刷新、查看 YAML”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 集群、命名空间、端点。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：集群、命名空间、端点。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“服务发现管理”。用当前任务的主信息结构组织首屏，次级信息按阅读路径分层。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 38. Ingress

- **应用 / 路由 / 实现**：容器管理 · `/containers/k8s/ingresses` · `K8s.vue`

### 1. 页面定位

- 核心用户：平台管理员、对应业务负责人。
- 主任务：查看域名、规则、后端和 TLS。
- 高频任务：刷新、查看 YAML、筛选当前范围、进入详情或结果。
- 首要信息：名称、主机、路径、后端、TLS、年龄。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去集群、命名空间、域名、后端。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 集群、命名空间、域名、后端
├─ Title + 13px description
└─ Primary: 刷新、查看 YAML
Primary Content Area\n├─ 核心信息或可视化工作区\n└─ 详情、结果或辅助信息区
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 集群、命名空间、域名、后端 | 32px 高；范围变化立即刷新数据 |
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

- **Primary**：刷新、查看 YAML，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 内容与状态设计

- 核心内容置于首个可视区域；辅助说明和原始数据放在第二层或可折叠区。
- 数字与时间右对齐或等宽显示；异常状态优先于健康状态。
- 图形、日志、画布和编辑器各自占用明确工作区，避免与普通表单混排。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“刷新、查看 YAML”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 集群、命名空间、域名、后端。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：集群、命名空间、域名、后端。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“入口路由管理”。用当前任务的主信息结构组织首屏，次级信息按阅读路径分层。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 39. 高级网络

- **应用 / 路由 / 实现**：容器管理 · `/containers/k8s/advanced-network` · `K8s.vue`

### 1. 页面定位

- 核心用户：平台管理员、对应业务负责人。
- 主任务：检查网络策略与扩展网络资源。
- 高频任务：刷新、查看 YAML、筛选当前范围、进入详情或结果。
- 首要信息：资源名、类型、命名空间、状态、年龄。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去集群、命名空间、网络策略。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 集群、命名空间、网络策略
├─ Title + 13px description
└─ Primary: 刷新、查看 YAML
Primary Content Area\n├─ 核心信息或可视化工作区\n└─ 详情、结果或辅助信息区
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 集群、命名空间、网络策略 | 32px 高；范围变化立即刷新数据 |
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

- **Primary**：刷新、查看 YAML，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 内容与状态设计

- 核心内容置于首个可视区域；辅助说明和原始数据放在第二层或可折叠区。
- 数字与时间右对齐或等宽显示；异常状态优先于健康状态。
- 图形、日志、画布和编辑器各自占用明确工作区，避免与普通表单混排。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“刷新、查看 YAML”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 集群、命名空间、网络策略。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：集群、命名空间、网络策略。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“高级网络资源”。用当前任务的主信息结构组织首屏，次级信息按阅读路径分层。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 40. 配置与存储

- **应用 / 路由 / 实现**：容器管理 · `/containers/k8s/config-storage` · `K8s.vue`

### 1. 页面定位

- 核心用户：平台管理员、对应业务负责人。
- 主任务：维护 ConfigMap、Secret、PVC 与 StorageClass。
- 高频任务：新建资源、筛选当前范围、进入详情或结果。
- 首要信息：名称、类型、作用域、容量、状态、年龄。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去集群、命名空间、存储、配置。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 集群、命名空间、存储、配置
├─ Title + 13px description
└─ Primary: 新建资源
Primary Content Area\n├─ 核心信息或可视化工作区\n└─ 详情、结果或辅助信息区
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 集群、命名空间、存储、配置 | 32px 高；范围变化立即刷新数据 |
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

- **Primary**：新建资源，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 内容与状态设计

- 核心内容置于首个可视区域；辅助说明和原始数据放在第二层或可折叠区。
- 数字与时间右对齐或等宽显示；异常状态优先于健康状态。
- 图形、日志、画布和编辑器各自占用明确工作区，避免与普通表单混排。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“新建资源”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 集群、命名空间、存储、配置。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：集群、命名空间、存储、配置。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“配置与存储资源”。用当前任务的主信息结构组织首屏，次级信息按阅读路径分层。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 41. Pod 终端

- **应用 / 路由 / 实现**：容器管理 · `/containers/k8s/pod-terminal/:clusterId/:namespace/:podName` · `K8sPodTerminal.vue`

### 1. 页面定位

- 核心用户：平台管理员、对应业务负责人。
- 主任务：在明确上下文中连接容器 Shell。
- 高频任务：连接、切换容器、筛选当前范围、进入详情或结果。
- 首要信息：集群、命名空间、Pod、容器、会话状态。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去集群、命名空间、Pod、容器、审计。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 集群、命名空间、Pod、容器、审计
├─ Title + 13px description
└─ Primary: 连接、切换容器
Workspace Context / Control Bar\n├─ 当前范围、刷新或连接状态\n└─ 次级操作\nFilter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 集群、命名空间、Pod、容器、审计 | 32px 高；范围变化立即刷新数据 |
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

- **Primary**：连接、切换容器，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 表格设计

- 列顺序：名称/标识（220px）→ 集群、命名空间、Pod、容器、审计（160px）→ 状态（100px）→ 最近时间/年龄（150px）→ 操作（140px fixed-right）。
- 关键列固定：名称与操作；长地址、表达式、响应和备注允许省略，Hover 显示完整值。
- 状态使用 Tag；ID、地址、命令、查询、版本和时间戳使用等宽字体；次级元信息在名称下第二行弱化。
- 行高 44px，Hover `#F8FAFD`；点击名称进入详情，勾选后显示批量操作条。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“连接、切换容器”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 集群、命名空间、Pod、容器、审计。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：集群、命名空间、Pod、容器、审计。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“Pod 交互终端”。主工作区优先，工具栏与辅助面板可折叠；不将其改造成 KPI + 表格页面。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 42. 脚本库

- **应用 / 路由 / 实现**：标准运维 · `/ops/scripts/library` · `OpsScriptLibrary.vue`

### 1. 页面定位

- 核心用户：平台管理员、运维执行人。
- 主任务：查找、编辑和启用标准脚本。
- 高频任务：新建脚本、筛选当前范围、进入详情或结果。
- 首要信息：脚本名、分类、参数、状态、更新时间。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去脚本、版本、权限、审计。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 脚本、版本、权限、审计
├─ Title + 13px description
└─ Primary: 新建脚本
Primary Content Area\n├─ 核心信息或可视化工作区\n└─ 详情、结果或辅助信息区
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 脚本、版本、权限、审计 | 32px 高；范围变化立即刷新数据 |
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

- **Primary**：新建脚本，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 内容与状态设计

- 核心内容置于首个可视区域；辅助说明和原始数据放在第二层或可折叠区。
- 数字与时间右对齐或等宽显示；异常状态优先于健康状态。
- 图形、日志、画布和编辑器各自占用明确工作区，避免与普通表单混排。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“新建脚本”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 脚本、版本、权限、审计。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：脚本、版本、权限、审计。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“运维脚本资产”。用当前任务的主信息结构组织首屏，次级信息按阅读路径分层。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 43. 命令执行

- **应用 / 路由 / 实现**：标准运维 · `/ops/quick-exec/command` · `OpsCommandExecute.vue`

### 1. 页面定位

- 核心用户：平台管理员、运维执行人。
- 主任务：选择目标并安全批量下发命令。
- 高频任务：立即执行、筛选当前范围、进入详情或结果。
- 首要信息：任务名、命令、目标、并发、超时、预览。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去环境、目标、命令、风险、审计。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 环境、目标、命令、风险、审计
├─ Title + 13px description
└─ Primary: 立即执行
Primary Content Area\n├─ 核心信息或可视化工作区\n└─ 详情、结果或辅助信息区
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 环境、目标、命令、风险、审计 | 32px 高；范围变化立即刷新数据 |
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

- **Primary**：立即执行，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 内容与状态设计

- 核心内容置于首个可视区域；辅助说明和原始数据放在第二层或可折叠区。
- 数字与时间右对齐或等宽显示；异常状态优先于健康状态。
- 图形、日志、画布和编辑器各自占用明确工作区，避免与普通表单混排。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“立即执行”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 环境、目标、命令、风险、审计。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：环境、目标、命令、风险、审计。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“即时命令执行”。用当前任务的主信息结构组织首屏，次级信息按阅读路径分层。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 44. 脚本执行

- **应用 / 路由 / 实现**：标准运维 · `/ops/quick-exec/script` · `OpsScriptExecute.vue`

### 1. 页面定位

- 核心用户：平台管理员、运维执行人。
- 主任务：选择脚本与目标后执行。
- 高频任务：立即执行、筛选当前范围、进入详情或结果。
- 首要信息：脚本、参数、目标、并发、超时。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去脚本、目标、参数、审计。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 脚本、目标、参数、审计
├─ Title + 13px description
└─ Primary: 立即执行
Primary Content Area\n├─ 核心信息或可视化工作区\n└─ 详情、结果或辅助信息区
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 脚本、目标、参数、审计 | 32px 高；范围变化立即刷新数据 |
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

- **Primary**：立即执行，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 内容与状态设计

- 核心内容置于首个可视区域；辅助说明和原始数据放在第二层或可折叠区。
- 数字与时间右对齐或等宽显示；异常状态优先于健康状态。
- 图形、日志、画布和编辑器各自占用明确工作区，避免与普通表单混排。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“立即执行”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 脚本、目标、参数、审计。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：脚本、目标、参数、审计。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“标准脚本执行”。用当前任务的主信息结构组织首屏，次级信息按阅读路径分层。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 45. 文件分发

- **应用 / 路由 / 实现**：标准运维 · `/ops/quick-exec/file-dispatch` · `OpsFileDispatch.vue`

### 1. 页面定位

- 核心用户：平台管理员、运维执行人。
- 主任务：选择来源、目标路径和主机范围。
- 高频任务：开始分发、筛选当前范围、进入详情或结果。
- 首要信息：源文件、目标路径、目标、校验、并发。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去文件、目标、校验、审计。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 文件、目标、校验、审计
├─ Title + 13px description
└─ Primary: 开始分发
Primary Content Area\n├─ 核心信息或可视化工作区\n└─ 详情、结果或辅助信息区
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 文件、目标、校验、审计 | 32px 高；范围变化立即刷新数据 |
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

- **Primary**：开始分发，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 内容与状态设计

- 核心内容置于首个可视区域；辅助说明和原始数据放在第二层或可折叠区。
- 数字与时间右对齐或等宽显示；异常状态优先于健康状态。
- 图形、日志、画布和编辑器各自占用明确工作区，避免与普通表单混排。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“开始分发”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 文件、目标、校验、审计。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：文件、目标、校验、审计。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“批量文件分发”。用当前任务的主信息结构组织首屏，次级信息按阅读路径分层。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 46. 快速执行历史

- **应用 / 路由 / 实现**：标准运维 · `/ops/quick-exec/history` · `OpsExecutionHistory.vue`

### 1. 页面定位

- 核心用户：平台管理员、运维执行人。
- 主任务：检索任务结果与失败明细。
- 高频任务：查看详情、重试、筛选当前范围、进入详情或结果。
- 首要信息：任务、类型、目标数、结果、开始时间、耗时。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去任务、目标、结果、耗时。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 任务、目标、结果、耗时
├─ Title + 13px description
└─ Primary: 查看详情、重试
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 任务、目标、结果、耗时 | 32px 高；范围变化立即刷新数据 |
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

- **Primary**：查看详情、重试，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 表格设计

- 列顺序：名称/标识（220px）→ 任务、目标、结果、耗时（160px）→ 状态（100px）→ 最近时间/年龄（150px）→ 操作（140px fixed-right）。
- 关键列固定：名称与操作；长地址、表达式、响应和备注允许省略，Hover 显示完整值。
- 状态使用 Tag；ID、地址、命令、查询、版本和时间戳使用等宽字体；次级元信息在名称下第二行弱化。
- 行高 44px，Hover `#F8FAFD`；点击名称进入详情，勾选后显示批量操作条。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“查看详情、重试”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 任务、目标、结果、耗时。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：任务、目标、结果、耗时。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“即时任务追溯”。保持列表工作台节奏：筛选、扫描、行内操作、详情回看；不堆叠无业务价值的 KPI。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 47. 定时任务

- **应用 / 路由 / 实现**：标准运维 · `/ops/schedule/tasks` · `OpsScheduleTaskList.vue`

### 1. 页面定位

- 核心用户：平台管理员、运维执行人。
- 主任务：维护脚本和 HTTP 任务调度。
- 高频任务：新建任务、筛选当前范围、进入详情或结果。
- 首要信息：任务、类型、Cron、状态、下次执行、负责人。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去环境、Cron、状态、负责人。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 环境、Cron、状态、负责人
├─ Title + 13px description
└─ Primary: 新建任务
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 环境、Cron、状态、负责人 | 32px 高；范围变化立即刷新数据 |
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

- **Primary**：新建任务，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 表格设计

- 列顺序：名称/标识（220px）→ 环境、Cron、状态、负责人（160px）→ 状态（100px）→ 最近时间/年龄（150px）→ 操作（140px fixed-right）。
- 关键列固定：名称与操作；长地址、表达式、响应和备注允许省略，Hover 显示完整值。
- 状态使用 Tag；ID、地址、命令、查询、版本和时间戳使用等宽字体；次级元信息在名称下第二行弱化。
- 行高 44px，Hover `#F8FAFD`；点击名称进入详情，勾选后显示批量操作条。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“新建任务”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 环境、Cron、状态、负责人。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：环境、Cron、状态、负责人。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“任务调度管理”。保持列表工作台节奏：筛选、扫描、行内操作、详情回看；不堆叠无业务价值的 KPI。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 48. 任务日志

- **应用 / 路由 / 实现**：标准运维 · `/ops/schedule/logs` · `OpsScheduleLog.vue`

### 1. 页面定位

- 核心用户：平台管理员、运维执行人。
- 主任务：定位调度结果与失败输出。
- 高频任务：查看详情、重试、筛选当前范围、进入详情或结果。
- 首要信息：任务、触发时间、结果、耗时、输出摘要。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去任务、触发、结果、耗时。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 任务、触发、结果、耗时
├─ Title + 13px description
└─ Primary: 查看详情、重试
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 任务、触发、结果、耗时 | 32px 高；范围变化立即刷新数据 |
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

- **Primary**：查看详情、重试，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 表格设计

- 列顺序：名称/标识（220px）→ 任务、触发、结果、耗时（160px）→ 状态（100px）→ 最近时间/年龄（150px）→ 操作（140px fixed-right）。
- 关键列固定：名称与操作；长地址、表达式、响应和备注允许省略，Hover 显示完整值。
- 状态使用 Tag；ID、地址、命令、查询、版本和时间戳使用等宽字体；次级元信息在名称下第二行弱化。
- 行高 44px，Hover `#F8FAFD`；点击名称进入详情，勾选后显示批量操作条。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“查看详情、重试”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 任务、触发、结果、耗时。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：任务、触发、结果、耗时。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“调度执行追溯”。保持列表工作台节奏：筛选、扫描、行内操作、详情回看；不堆叠无业务价值的 KPI。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 49. 任务模板

- **应用 / 路由 / 实现**：标准运维 · `/ops/schedule/templates` · `OpsScheduleTemplate.vue`

### 1. 页面定位

- 核心用户：平台管理员、运维执行人。
- 主任务：选择或维护可复用调度模板。
- 高频任务：新建模板、筛选当前范围、进入详情或结果。
- 首要信息：模板、类型、Cron、参数、状态。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去模板、Cron、参数。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 模板、Cron、参数
├─ Title + 13px description
└─ Primary: 新建模板
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 模板、Cron、参数 | 32px 高；范围变化立即刷新数据 |
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

- **Primary**：新建模板，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 表格设计

- 列顺序：名称/标识（220px）→ 模板、Cron、参数（160px）→ 状态（100px）→ 最近时间/年龄（150px）→ 操作（140px fixed-right）。
- 关键列固定：名称与操作；长地址、表达式、响应和备注允许省略，Hover 显示完整值。
- 状态使用 Tag；ID、地址、命令、查询、版本和时间戳使用等宽字体；次级元信息在名称下第二行弱化。
- 行高 44px，Hover `#F8FAFD`；点击名称进入详情，勾选后显示批量操作条。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“新建模板”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 模板、Cron、参数。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：模板、Cron、参数。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“调度模板库”。保持列表工作台节奏：筛选、扫描、行内操作、详情回看；不堆叠无业务价值的 KPI。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 50. 作业编排

- **应用 / 路由 / 实现**：标准运维 · `/ops/jobs/designer` · `OpsJobDesigner.vue`

### 1. 页面定位

- 核心用户：平台管理员、运维执行人。
- 主任务：组合执行、分发与人工确认节点。
- 高频任务：保存、发布、运行、筛选当前范围、进入详情或结果。
- 首要信息：作业名、节点、连线、校验、版本。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去作业、节点、审批、版本、审计。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 作业、节点、审批、版本、审计
├─ Title + 13px description
└─ Primary: 保存、发布、运行
Workspace Context / Control Bar\n├─ 当前范围、刷新或连接状态\n└─ 次级操作\nPrimary Content Area\n├─ 核心信息或可视化工作区\n└─ 详情、结果或辅助信息区
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 作业、节点、审批、版本、审计 | 32px 高；范围变化立即刷新数据 |
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

- **Primary**：保存、发布、运行，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 内容与状态设计

- 核心内容置于首个可视区域；辅助说明和原始数据放在第二层或可折叠区。
- 数字与时间右对齐或等宽显示；异常状态优先于健康状态。
- 图形、日志、画布和编辑器各自占用明确工作区，避免与普通表单混排。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“保存、发布、运行”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 作业、节点、审批、版本、审计。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：作业、节点、审批、版本、审计。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“可视化作业设计”。主工作区优先，工具栏与辅助面板可折叠；不将其改造成 KPI + 表格页面。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 51. 作业列表

- **应用 / 路由 / 实现**：标准运维 · `/ops/jobs/list` · `OpsJobList.vue`

### 1. 页面定位

- 核心用户：平台管理员、运维执行人。
- 主任务：维护作业状态并发起运行。
- 高频任务：新建作业、运行、筛选当前范围、进入详情或结果。
- 首要信息：作业、版本、状态、负责人、更新时间。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去作业、版本、负责人、审计。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 作业、版本、负责人、审计
├─ Title + 13px description
└─ Primary: 新建作业、运行
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 作业、版本、负责人、审计 | 32px 高；范围变化立即刷新数据 |
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

- **Primary**：新建作业、运行，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 表格设计

- 列顺序：名称/标识（220px）→ 作业、版本、负责人、审计（160px）→ 状态（100px）→ 最近时间/年龄（150px）→ 操作（140px fixed-right）。
- 关键列固定：名称与操作；长地址、表达式、响应和备注允许省略，Hover 显示完整值。
- 状态使用 Tag；ID、地址、命令、查询、版本和时间戳使用等宽字体；次级元信息在名称下第二行弱化。
- 行高 44px，Hover `#F8FAFD`；点击名称进入详情，勾选后显示批量操作条。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“新建作业、运行”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 作业、版本、负责人、审计。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：作业、版本、负责人、审计。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“作业定义管理”。保持列表工作台节奏：筛选、扫描、行内操作、详情回看；不堆叠无业务价值的 KPI。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 52. 人工确认

- **应用 / 路由 / 实现**：标准运维 · `/ops/jobs/approvals` · `OpsJobApprovals.vue`

### 1. 页面定位

- 核心用户：平台管理员、运维执行人。
- 主任务：处理需要人工确认的作业步骤。
- 高频任务：确认、拒绝、筛选当前范围、进入详情或结果。
- 首要信息：作业、步骤、目标、等待时间、风险。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去作业、步骤、风险、审计。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 作业、步骤、风险、审计
├─ Title + 13px description
└─ Primary: 确认、拒绝
Primary Content Area\n├─ 核心信息或可视化工作区\n└─ 详情、结果或辅助信息区
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 作业、步骤、风险、审计 | 32px 高；范围变化立即刷新数据 |
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

- **Primary**：确认、拒绝，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 内容与状态设计

- 核心内容置于首个可视区域；辅助说明和原始数据放在第二层或可折叠区。
- 数字与时间右对齐或等宽显示；异常状态优先于健康状态。
- 图形、日志、画布和编辑器各自占用明确工作区，避免与普通表单混排。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“确认、拒绝”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 作业、步骤、风险、审计。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：作业、步骤、风险、审计。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“人工审批待办”。用当前任务的主信息结构组织首屏，次级信息按阅读路径分层。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 53. 作业历史

- **应用 / 路由 / 实现**：标准运维 · `/ops/jobs/history` · `OpsJobHistory.vue`

### 1. 页面定位

- 核心用户：平台管理员、运维执行人。
- 主任务：查看作业步骤、结果和审批状态。
- 高频任务：查看详情、重试、筛选当前范围、进入详情或结果。
- 首要信息：作业、运行、步骤、结果、耗时、审批。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去作业、步骤、结果、审批。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 作业、步骤、结果、审批
├─ Title + 13px description
└─ Primary: 查看详情、重试
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 作业、步骤、结果、审批 | 32px 高；范围变化立即刷新数据 |
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

- **Primary**：查看详情、重试，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 表格设计

- 列顺序：名称/标识（220px）→ 作业、步骤、结果、审批（160px）→ 状态（100px）→ 最近时间/年龄（150px）→ 操作（140px fixed-right）。
- 关键列固定：名称与操作；长地址、表达式、响应和备注允许省略，Hover 显示完整值。
- 状态使用 Tag；ID、地址、命令、查询、版本和时间戳使用等宽字体；次级元信息在名称下第二行弱化。
- 行高 44px，Hover `#F8FAFD`；点击名称进入详情，勾选后显示批量操作条。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“查看详情、重试”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 作业、步骤、结果、审批。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：作业、步骤、结果、审批。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“作业运行追溯”。保持列表工作台节奏：筛选、扫描、行内操作、详情回看；不堆叠无业务价值的 KPI。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 54. 作业模板

- **应用 / 路由 / 实现**：标准运维 · `/ops/jobs/templates` · `OpsJobTemplate.vue`

### 1. 页面定位

- 核心用户：平台管理员、运维执行人。
- 主任务：维护可导入的作业模板。
- 高频任务：新建模板、筛选当前范围、进入详情或结果。
- 首要信息：模板、节点数、版本、状态、更新时间。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去模板、版本、节点。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 模板、版本、节点
├─ Title + 13px description
└─ Primary: 新建模板
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 模板、版本、节点 | 32px 高；范围变化立即刷新数据 |
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

- **Primary**：新建模板，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 表格设计

- 列顺序：名称/标识（220px）→ 模板、版本、节点（160px）→ 状态（100px）→ 最近时间/年龄（150px）→ 操作（140px fixed-right）。
- 关键列固定：名称与操作；长地址、表达式、响应和备注允许省略，Hover 显示完整值。
- 状态使用 Tag；ID、地址、命令、查询、版本和时间戳使用等宽字体；次级元信息在名称下第二行弱化。
- 行高 44px，Hover `#F8FAFD`；点击名称进入详情，勾选后显示批量操作条。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“新建模板”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 模板、版本、节点。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：模板、版本、节点。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“作业模板库”。保持列表工作台节奏：筛选、扫描、行内操作、详情回看；不堆叠无业务价值的 KPI。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 55. 项目列表

- **应用 / 路由 / 实现**：应用中心 · `/applications/projects` · `AppProjectList.vue`

### 1. 页面定位

- 核心用户：平台管理员、对应业务负责人。
- 主任务：维护项目仓库与资源绑定。
- 高频任务：新建应用、筛选当前范围、进入详情或结果。
- 首要信息：项目、服务类别、环境、仓库、状态。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去应用、环境、仓库、状态。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 应用、环境、仓库、状态
├─ Title + 13px description
└─ Primary: 新建应用
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 应用、环境、仓库、状态 | 32px 高；范围变化立即刷新数据 |
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

- **Primary**：新建应用，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 表格设计

- 列顺序：名称/标识（220px）→ 应用、环境、仓库、状态（160px）→ 状态（100px）→ 最近时间/年龄（150px）→ 操作（140px fixed-right）。
- 关键列固定：名称与操作；长地址、表达式、响应和备注允许省略，Hover 显示完整值。
- 状态使用 Tag；ID、地址、命令、查询、版本和时间戳使用等宽字体；次级元信息在名称下第二行弱化。
- 行高 44px，Hover `#F8FAFD`；点击名称进入详情，勾选后显示批量操作条。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“新建应用”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 应用、环境、仓库、状态。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：应用、环境、仓库、状态。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“应用资产目录”。保持列表工作台节奏：筛选、扫描、行内操作、详情回看；不堆叠无业务价值的 KPI。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 56. 应用拓扑

- **应用 / 路由 / 实现**：应用中心 · `/applications/topology` · `AppTopology.vue`

### 1. 页面定位

- 核心用户：平台管理员、对应业务负责人。
- 主任务：浏览应用到资源、发布和告警关系。
- 高频任务：切换应用、查看详情、筛选当前范围、进入详情或结果。
- 首要信息：应用、环境、资源、发布、告警。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去应用、环境、资源、告警。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 应用、环境、资源、告警
├─ Title + 13px description
└─ Primary: 切换应用、查看详情
Workspace Context / Control Bar\n├─ 当前范围、刷新或连接状态\n└─ 次级操作\nPrimary Content Area\n├─ 核心信息或可视化工作区\n└─ 详情、结果或辅助信息区
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 应用、环境、资源、告警 | 32px 高；范围变化立即刷新数据 |
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

- **Primary**：切换应用、查看详情，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 内容与状态设计

- 核心内容置于首个可视区域；辅助说明和原始数据放在第二层或可折叠区。
- 数字与时间右对齐或等宽显示；异常状态优先于健康状态。
- 图形、日志、画布和编辑器各自占用明确工作区，避免与普通表单混排。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“切换应用、查看详情”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 应用、环境、资源、告警。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：应用、环境、资源、告警。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“应用交付拓扑”。主工作区优先，工具栏与辅助面板可折叠；不将其改造成 KPI + 表格页面。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 57. 构建任务

- **应用 / 路由 / 实现**：应用中心 · `/applications/build-tasks` · `AppBuildTaskList.vue`

### 1. 页面定位

- 核心用户：平台管理员、对应业务负责人。
- 主任务：创建任务并触发构建。
- 高频任务：新建构建任务、筛选当前范围、进入详情或结果。
- 首要信息：任务、项目、分支、构建方式、状态。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去项目、分支、镜像、状态。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 项目、分支、镜像、状态
├─ Title + 13px description
└─ Primary: 新建构建任务
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 项目、分支、镜像、状态 | 32px 高；范围变化立即刷新数据 |
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

- **Primary**：新建构建任务，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 表格设计

- 列顺序：名称/标识（220px）→ 项目、分支、镜像、状态（160px）→ 状态（100px）→ 最近时间/年龄（150px）→ 操作（140px fixed-right）。
- 关键列固定：名称与操作；长地址、表达式、响应和备注允许省略，Hover 显示完整值。
- 状态使用 Tag；ID、地址、命令、查询、版本和时间戳使用等宽字体；次级元信息在名称下第二行弱化。
- 行高 44px，Hover `#F8FAFD`；点击名称进入详情，勾选后显示批量操作条。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“新建构建任务”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 项目、分支、镜像、状态。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：项目、分支、镜像、状态。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“构建任务管理”。保持列表工作台节奏：筛选、扫描、行内操作、详情回看；不堆叠无业务价值的 KPI。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 58. 构建历史

- **应用 / 路由 / 实现**：应用中心 · `/applications/build-history` · `AppBuildHistory.vue`

### 1. 页面定位

- 核心用户：平台管理员、对应业务负责人。
- 主任务：查看阶段、日志和产物。
- 高频任务：查看日志、重试、筛选当前范围、进入详情或结果。
- 首要信息：构建号、项目、分支、阶段、结果、耗时。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去项目、分支、阶段、产物。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 项目、分支、阶段、产物
├─ Title + 13px description
└─ Primary: 查看日志、重试
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 项目、分支、阶段、产物 | 32px 高；范围变化立即刷新数据 |
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

- **Primary**：查看日志、重试，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 表格设计

- 列顺序：名称/标识（220px）→ 项目、分支、阶段、产物（160px）→ 状态（100px）→ 最近时间/年龄（150px）→ 操作（140px fixed-right）。
- 关键列固定：名称与操作；长地址、表达式、响应和备注允许省略，Hover 显示完整值。
- 状态使用 Tag；ID、地址、命令、查询、版本和时间戳使用等宽字体；次级元信息在名称下第二行弱化。
- 行高 44px，Hover `#F8FAFD`；点击名称进入详情，勾选后显示批量操作条。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“查看日志、重试”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 项目、分支、阶段、产物。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：项目、分支、阶段、产物。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“构建运行追溯”。保持列表工作台节奏：筛选、扫描、行内操作、详情回看；不堆叠无业务价值的 KPI。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 59. 镜像仓库

- **应用 / 路由 / 实现**：应用中心 · `/applications/image-registries` · `AppImageRegistry.vue`

### 1. 页面定位

- 核心用户：平台管理员、对应业务负责人。
- 主任务：维护镜像仓库接入与状态。
- 高频任务：新增仓库、测试连接、筛选当前范围、进入详情或结果。
- 首要信息：仓库名、地址、类型、连接状态、更新时间。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去仓库、连接、凭据。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 仓库、连接、凭据
├─ Title + 13px description
└─ Primary: 新增仓库、测试连接
Primary Content Area\n├─ 核心信息或可视化工作区\n└─ 详情、结果或辅助信息区
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 仓库、连接、凭据 | 32px 高；范围变化立即刷新数据 |
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

- **Primary**：新增仓库、测试连接，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 内容与状态设计

- 核心内容置于首个可视区域；辅助说明和原始数据放在第二层或可折叠区。
- 数字与时间右对齐或等宽显示；异常状态优先于健康状态。
- 图形、日志、画布和编辑器各自占用明确工作区，避免与普通表单混排。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“新增仓库、测试连接”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 仓库、连接、凭据。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：仓库、连接、凭据。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“镜像仓库管理”。用当前任务的主信息结构组织首屏，次级信息按阅读路径分层。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 60. CI/CD 流水线

- **应用 / 路由 / 实现**：应用中心 · `/applications/pipelines` · `AppPipelineCenter.vue`

### 1. 页面定位

- 核心用户：平台管理员、对应业务负责人。
- 主任务：选择模板、配置阶段并追踪运行。
- 高频任务：新建流水线、运行、筛选当前范围、进入详情或结果。
- 首要信息：流水线、阶段、环境、运行状态、日志。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去应用、环境、阶段、发布、审计。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 应用、环境、阶段、发布、审计
├─ Title + 13px description
└─ Primary: 新建流水线、运行
Primary Content Area\n├─ 核心信息或可视化工作区\n└─ 详情、结果或辅助信息区
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 应用、环境、阶段、发布、审计 | 32px 高；范围变化立即刷新数据 |
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

- **Primary**：新建流水线、运行，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 内容与状态设计

- 核心内容置于首个可视区域；辅助说明和原始数据放在第二层或可折叠区。
- 数字与时间右对齐或等宽显示；异常状态优先于健康状态。
- 图形、日志、画布和编辑器各自占用明确工作区，避免与普通表单混排。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“新建流水线、运行”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 应用、环境、阶段、发布、审计。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：应用、环境、阶段、发布、审计。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“交付流水线工作台”。用当前任务的主信息结构组织首屏，次级信息按阅读路径分层。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 61. 消息模板

- **应用 / 路由 / 实现**：消息通知 · `/notify/templates` · `NotifyTemplate.vue`

### 1. 页面定位

- 核心用户：平台管理员、对应业务负责人。
- 主任务：维护多渠道消息模板并实时预览。
- 高频任务：新建模板、保存、筛选当前范围、进入详情或结果。
- 首要信息：渠道、变量、模板状态、预览。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去渠道、变量、模板状态。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 渠道、变量、模板状态
├─ Title + 13px description
└─ Primary: 新建模板、保存
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 渠道、变量、模板状态 | 32px 高；范围变化立即刷新数据 |
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

- **Primary**：新建模板、保存，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 表格设计

- 列顺序：名称/标识（220px）→ 渠道、变量、模板状态（160px）→ 状态（100px）→ 最近时间/年龄（150px）→ 操作（140px fixed-right）。
- 关键列固定：名称与操作；长地址、表达式、响应和备注允许省略，Hover 显示完整值。
- 状态使用 Tag；ID、地址、命令、查询、版本和时间戳使用等宽字体；次级元信息在名称下第二行弱化。
- 行高 44px，Hover `#F8FAFD`；点击名称进入详情，勾选后显示批量操作条。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“新建模板、保存”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 渠道、变量、模板状态。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：渠道、变量、模板状态。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“通知内容编辑”。保持列表工作台节奏：筛选、扫描、行内操作、详情回看；不堆叠无业务价值的 KPI。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 62. 通知媒介

- **应用 / 路由 / 实现**：消息通知 · `/notify/channels` · `NotifyChannel.vue`

### 1. 页面定位

- 核心用户：平台管理员、对应业务负责人。
- 主任务：维护机器人和 Webhook 媒介。
- 高频任务：新增媒介、测试、筛选当前范围、进入详情或结果。
- 首要信息：通道、类型、地址、状态、最近测试。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去通道、状态、测试。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 通道、状态、测试
├─ Title + 13px description
└─ Primary: 新增媒介、测试
Primary Content Area\n├─ 核心信息或可视化工作区\n└─ 详情、结果或辅助信息区
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 通道、状态、测试 | 32px 高；范围变化立即刷新数据 |
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

- **Primary**：新增媒介、测试，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 内容与状态设计

- 核心内容置于首个可视区域；辅助说明和原始数据放在第二层或可折叠区。
- 数字与时间右对齐或等宽显示；异常状态优先于健康状态。
- 图形、日志、画布和编辑器各自占用明确工作区，避免与普通表单混排。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“新增媒介、测试”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 通道、状态、测试。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：通道、状态、测试。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“通知通道管理”。用当前任务的主信息结构组织首屏，次级信息按阅读路径分层。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 63. 通知规则

- **应用 / 路由 / 实现**：消息通知 · `/notify/rules` · `NotifyRule.vue`

### 1. 页面定位

- 核心用户：平台管理员、对应业务负责人。
- 主任务：把事件、模板和媒介组合为规则。
- 高频任务：新建规则、筛选当前范围、进入详情或结果。
- 首要信息：事件、模板、媒介、启用状态、优先级。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去事件、媒介、优先级、审计。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 事件、媒介、优先级、审计
├─ Title + 13px description
└─ Primary: 新建规则
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 事件、媒介、优先级、审计 | 32px 高；范围变化立即刷新数据 |
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

- **Primary**：新建规则，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 表格设计

- 列顺序：名称/标识（220px）→ 事件、媒介、优先级、审计（160px）→ 状态（100px）→ 最近时间/年龄（150px）→ 操作（140px fixed-right）。
- 关键列固定：名称与操作；长地址、表达式、响应和备注允许省略，Hover 显示完整值。
- 状态使用 Tag；ID、地址、命令、查询、版本和时间戳使用等宽字体；次级元信息在名称下第二行弱化。
- 行高 44px，Hover `#F8FAFD`；点击名称进入详情，勾选后显示批量操作条。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“新建规则”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 事件、媒介、优先级、审计。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：事件、媒介、优先级、审计。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“通知路由编排”。保持列表工作台节奏：筛选、扫描、行内操作、详情回看；不堆叠无业务价值的 KPI。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 64. 发送日志

- **应用 / 路由 / 实现**：消息通知 · `/notify/send-logs` · `NotifySendLog.vue`

### 1. 页面定位

- 核心用户：平台管理员、对应业务负责人。
- 主任务：定位发送失败与响应异常。
- 高频任务：查看详情、重试、筛选当前范围、进入详情或结果。
- 首要信息：通道、事件、结果、响应码、耗时、时间。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去通道、结果、耗时、响应。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 通道、结果、耗时、响应
├─ Title + 13px description
└─ Primary: 查看详情、重试
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 通道、结果、耗时、响应 | 32px 高；范围变化立即刷新数据 |
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

- **Primary**：查看详情、重试，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 表格设计

- 列顺序：名称/标识（220px）→ 通道、结果、耗时、响应（160px）→ 状态（100px）→ 最近时间/年龄（150px）→ 操作（140px fixed-right）。
- 关键列固定：名称与操作；长地址、表达式、响应和备注允许省略，Hover 显示完整值。
- 状态使用 Tag；ID、地址、命令、查询、版本和时间戳使用等宽字体；次级元信息在名称下第二行弱化。
- 行高 44px，Hover `#F8FAFD`；点击名称进入详情，勾选后显示批量操作条。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“查看详情、重试”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 通道、结果、耗时、响应。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：通道、结果、耗时、响应。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“消息送达追溯”。保持列表工作台节奏：筛选、扫描、行内操作、详情回看；不堆叠无业务价值的 KPI。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 65. 导航管理

- **应用 / 路由 / 实现**：集成中心 · `/integration/navigation` · `IntegrationNavigation.vue`

### 1. 页面定位

- 核心用户：平台管理员、对应业务负责人。
- 主任务：按分组维护常用系统入口。
- 高频任务：新增入口、生成公开链接、筛选当前范围、进入详情或结果。
- 首要信息：分组、入口、地址、状态、公开链接。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去分组、入口、公开状态。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 分组、入口、公开状态
├─ Title + 13px description
└─ Primary: 新增入口、生成公开链接
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 分组、入口、公开状态 | 32px 高；范围变化立即刷新数据 |
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

- **Primary**：新增入口、生成公开链接，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 表格设计

- 列顺序：名称/标识（220px）→ 分组、入口、公开状态（160px）→ 状态（100px）→ 最近时间/年龄（150px）→ 操作（140px fixed-right）。
- 关键列固定：名称与操作；长地址、表达式、响应和备注允许省略，Hover 显示完整值。
- 状态使用 Tag；ID、地址、命令、查询、版本和时间戳使用等宽字体；次级元信息在名称下第二行弱化。
- 行高 44px，Hover `#F8FAFD`；点击名称进入详情，勾选后显示批量操作条。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“新增入口、生成公开链接”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 分组、入口、公开状态。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：分组、入口、公开状态。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“系统入口编排”。保持列表工作台节奏：筛选、扫描、行内操作、详情回看；不堆叠无业务价值的 KPI。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 66. 公开导航

- **应用 / 路由 / 实现**：集成中心 · `/public/navigation/:token` · `PublicNavigation.vue`

### 1. 页面定位

- 核心用户：平台管理员、对应业务负责人。
- 主任务：搜索并打开授权的系统入口。
- 高频任务：打开入口、搜索、筛选当前范围、进入详情或结果。
- 首要信息：品牌、分组、入口、可用状态。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去访问令牌、入口、可用性。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 访问令牌、入口、可用性
├─ Title + 13px description
└─ Primary: 打开入口、搜索
Primary Content Area\n├─ 核心信息或可视化工作区\n└─ 详情、结果或辅助信息区
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 访问令牌、入口、可用性 | 32px 高；范围变化立即刷新数据 |
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

- **Primary**：打开入口、搜索，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 内容与状态设计

- 核心内容置于首个可视区域；辅助说明和原始数据放在第二层或可折叠区。
- 数字与时间右对齐或等宽显示；异常状态优先于健康状态。
- 图形、日志、画布和编辑器各自占用明确工作区，避免与普通表单混排。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“打开入口、搜索”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 访问令牌、入口、可用性。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：访问令牌、入口、可用性。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“只读入口门户”。用当前任务的主信息结构组织首屏，次级信息按阅读路径分层。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 67. AI 智能对话

- **应用 / 路由 / 实现**：集成中心 · `/integration/ai/chat` · `AIAssistantChat.vue`

### 1. 页面定位

- 核心用户：平台管理员、对应业务负责人。
- 主任务：发起会话、调用工具并阅读结果。
- 高频任务：发送消息、新建会话、筛选当前范围、进入详情或结果。
- 首要信息：会话、模型、消息、工具调用、执行状态。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去会话、模型、工具、审计。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 会话、模型、工具、审计
├─ Title + 13px description
└─ Primary: 发送消息、新建会话
Workspace Context / Control Bar\n├─ 当前范围、刷新或连接状态\n└─ 次级操作\nPrimary Content Area\n├─ 核心信息或可视化工作区\n└─ 详情、结果或辅助信息区
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 会话、模型、工具、审计 | 32px 高；范围变化立即刷新数据 |
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

- **Primary**：发送消息、新建会话，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 内容与状态设计

- 核心内容置于首个可视区域；辅助说明和原始数据放在第二层或可折叠区。
- 数字与时间右对齐或等宽显示；异常状态优先于健康状态。
- 图形、日志、画布和编辑器各自占用明确工作区，避免与普通表单混排。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“发送消息、新建会话”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 会话、模型、工具、审计。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：会话、模型、工具、审计。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“AI 运维协作台”。主工作区优先，工具栏与辅助面板可折叠；不将其改造成 KPI + 表格页面。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 68. AI 会话管理

- **应用 / 路由 / 实现**：集成中心 · `/integration/ai/conversations` · `AIConversations.vue`

### 1. 页面定位

- 核心用户：平台管理员、对应业务负责人。
- 主任务：检索、归档和追溯会话。
- 高频任务：新建、归档、查看、筛选当前范围、进入详情或结果。
- 首要信息：会话标题、模型、消息数、更新时间、状态。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去会话、模型、审计。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 会话、模型、审计
├─ Title + 13px description
└─ Primary: 新建、归档、查看
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 会话、模型、审计 | 32px 高；范围变化立即刷新数据 |
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

- **Primary**：新建、归档、查看，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 表格设计

- 列顺序：名称/标识（220px）→ 会话、模型、审计（160px）→ 状态（100px）→ 最近时间/年龄（150px）→ 操作（140px fixed-right）。
- 关键列固定：名称与操作；长地址、表达式、响应和备注允许省略，Hover 显示完整值。
- 状态使用 Tag；ID、地址、命令、查询、版本和时间戳使用等宽字体；次级元信息在名称下第二行弱化。
- 行高 44px，Hover `#F8FAFD`；点击名称进入详情，勾选后显示批量操作条。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“新建、归档、查看”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 会话、模型、审计。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：会话、模型、审计。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“AI 会话治理”。保持列表工作台节奏：筛选、扫描、行内操作、详情回看；不堆叠无业务价值的 KPI。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 69. AI 模型管理

- **应用 / 路由 / 实现**：集成中心 · `/integration/ai/models` · `AIModels.vue`

### 1. 页面定位

- 核心用户：平台管理员、对应业务负责人。
- 主任务：维护 OpenAI 兼容模型与默认参数。
- 高频任务：新增模型、测试连接、筛选当前范围、进入详情或结果。
- 首要信息：模型、地址、连接状态、默认标识、更新时间。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去模型、连接、默认配置。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 模型、连接、默认配置
├─ Title + 13px description
└─ Primary: 新增模型、测试连接
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 模型、连接、默认配置 | 32px 高；范围变化立即刷新数据 |
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

- **Primary**：新增模型、测试连接，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 表格设计

- 列顺序：名称/标识（220px）→ 模型、连接、默认配置（160px）→ 状态（100px）→ 最近时间/年龄（150px）→ 操作（140px fixed-right）。
- 关键列固定：名称与操作；长地址、表达式、响应和备注允许省略，Hover 显示完整值。
- 状态使用 Tag；ID、地址、命令、查询、版本和时间戳使用等宽字体；次级元信息在名称下第二行弱化。
- 行高 44px，Hover `#F8FAFD`；点击名称进入详情，勾选后显示批量操作条。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“新增模型、测试连接”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 模型、连接、默认配置。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：模型、连接、默认配置。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“模型接入配置”。保持列表工作台节奏：筛选、扫描、行内操作、详情回看；不堆叠无业务价值的 KPI。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 70. AI 知识库

- **应用 / 路由 / 实现**：集成中心 · `/integration/ai/knowledge-base` · `AIKnowledgeBase.vue`

### 1. 页面定位

- 核心用户：平台管理员、对应业务负责人。
- 主任务：维护 Markdown 文档与检索配置。
- 高频任务：新增文档、重新索引、筛选当前范围、进入详情或结果。
- 首要信息：文档、来源、更新时间、索引状态、命中配置。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去知识库、索引、来源。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 知识库、索引、来源
├─ Title + 13px description
└─ Primary: 新增文档、重新索引
Primary Content Area\n├─ 核心信息或可视化工作区\n└─ 详情、结果或辅助信息区
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 知识库、索引、来源 | 32px 高；范围变化立即刷新数据 |
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

- **Primary**：新增文档、重新索引，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 内容与状态设计

- 核心内容置于首个可视区域；辅助说明和原始数据放在第二层或可折叠区。
- 数字与时间右对齐或等宽显示；异常状态优先于健康状态。
- 图形、日志、画布和编辑器各自占用明确工作区，避免与普通表单混排。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“新增文档、重新索引”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 知识库、索引、来源。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：知识库、索引、来源。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“检索知识维护”。用当前任务的主信息结构组织首屏，次级信息按阅读路径分层。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 71. AI 工具集

- **应用 / 路由 / 实现**：集成中心 · `/integration/ai/tools` · `AITools.vue`

### 1. 页面定位

- 核心用户：平台管理员、对应业务负责人。
- 主任务：启用、配置并约束运维工具。
- 高频任务：新增工具、启停、筛选当前范围、进入详情或结果。
- 首要信息：工具、类型、权限、状态、调用范围。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去工具、权限、审计。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 工具、权限、审计
├─ Title + 13px description
└─ Primary: 新增工具、启停
Primary Content Area\n├─ 核心信息或可视化工作区\n└─ 详情、结果或辅助信息区
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 工具、权限、审计 | 32px 高；范围变化立即刷新数据 |
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

- **Primary**：新增工具、启停，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 内容与状态设计

- 核心内容置于首个可视区域；辅助说明和原始数据放在第二层或可折叠区。
- 数字与时间右对齐或等宽显示；异常状态优先于健康状态。
- 图形、日志、画布和编辑器各自占用明确工作区，避免与普通表单混排。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“新增工具、启停”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 工具、权限、审计。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：工具、权限、审计。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“AI 工具治理”。用当前任务的主信息结构组织首屏，次级信息按阅读路径分层。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 72. FinOps 费用看板

- **应用 / 路由 / 实现**：集成中心 · `/integration/finops/dashboard` · `FinOpsDashboard.vue`

### 1. 页面定位

- 核心用户：平台管理员、对应业务负责人。
- 主任务：判断成本、趋势与异常。
- 高频任务：切换账期、同步、筛选当前范围、进入详情或结果。
- 首要信息：账期、总费用、同比、云厂商、趋势。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去账期、账号、成本、异常。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 账期、账号、成本、异常
├─ Title + 13px description
└─ Primary: 切换账期、同步
Primary Content Area\n├─ 核心信息或可视化工作区\n└─ 详情、结果或辅助信息区
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 账期、账号、成本、异常 | 32px 高；范围变化立即刷新数据 |
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

- **Primary**：切换账期、同步，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 内容与状态设计

- 核心内容置于首个可视区域；辅助说明和原始数据放在第二层或可折叠区。
- 数字与时间右对齐或等宽显示；异常状态优先于健康状态。
- 图形、日志、画布和编辑器各自占用明确工作区，避免与普通表单混排。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“切换账期、同步”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 账期、账号、成本、异常。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：账期、账号、成本、异常。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“云成本总览”。用当前任务的主信息结构组织首屏，次级信息按阅读路径分层。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 73. FinOps 云账号

- **应用 / 路由 / 实现**：集成中心 · `/integration/finops/accounts` · `FinOpsAccounts.vue`

### 1. 页面定位

- 核心用户：平台管理员、对应业务负责人。
- 主任务：维护云账单接入与同步。
- 高频任务：新增账号、同步、筛选当前范围、进入详情或结果。
- 首要信息：账号、厂商、账期、同步状态、更新时间。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去厂商、账号、账期、同步。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 厂商、账号、账期、同步
├─ Title + 13px description
└─ Primary: 新增账号、同步
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 厂商、账号、账期、同步 | 32px 高；范围变化立即刷新数据 |
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

- **Primary**：新增账号、同步，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 表格设计

- 列顺序：名称/标识（220px）→ 厂商、账号、账期、同步（160px）→ 状态（100px）→ 最近时间/年龄（150px）→ 操作（140px fixed-right）。
- 关键列固定：名称与操作；长地址、表达式、响应和备注允许省略，Hover 显示完整值。
- 状态使用 Tag；ID、地址、命令、查询、版本和时间戳使用等宽字体；次级元信息在名称下第二行弱化。
- 行高 44px，Hover `#F8FAFD`；点击名称进入详情，勾选后显示批量操作条。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“新增账号、同步”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 厂商、账号、账期、同步。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：厂商、账号、账期、同步。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“账单账号管理”。保持列表工作台节奏：筛选、扫描、行内操作、详情回看；不堆叠无业务价值的 KPI。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 74. FinOps 费用拆分

- **应用 / 路由 / 实现**：集成中心 · `/integration/finops/breakdown` · `FinOpsBreakdown.vue`

### 1. 页面定位

- 核心用户：平台管理员、对应业务负责人。
- 主任务：按维度拆分成本并定位增长。
- 高频任务：切换维度、导出、筛选当前范围、进入详情或结果。
- 首要信息：维度、金额、占比、环比、筛选条件。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去账期、账号、区域、服务、标签。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 账期、账号、区域、服务、标签
├─ Title + 13px description
└─ Primary: 切换维度、导出
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 账期、账号、区域、服务、标签 | 32px 高；范围变化立即刷新数据 |
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

- **Primary**：切换维度、导出，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 表格设计

- 列顺序：名称/标识（220px）→ 账期、账号、区域、服务、标签（160px）→ 状态（100px）→ 最近时间/年龄（150px）→ 操作（140px fixed-right）。
- 关键列固定：名称与操作；长地址、表达式、响应和备注允许省略，Hover 显示完整值。
- 状态使用 Tag；ID、地址、命令、查询、版本和时间戳使用等宽字体；次级元信息在名称下第二行弱化。
- 行高 44px，Hover `#F8FAFD`；点击名称进入详情，勾选后显示批量操作条。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“切换维度、导出”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 账期、账号、区域、服务、标签。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：账期、账号、区域、服务、标签。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“成本归因分析”。保持列表工作台节奏：筛选、扫描、行内操作、详情回看；不堆叠无业务价值的 KPI。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 75. FinOps 优化建议

- **应用 / 路由 / 实现**：集成中心 · `/integration/finops/recommendations` · `FinOpsRecommendations.vue`

### 1. 页面定位

- 核心用户：平台管理员、对应业务负责人。
- 主任务：处理闲置、规格偏高和异常建议。
- 高频任务：采纳、忽略、查看资源、筛选当前范围、进入详情或结果。
- 首要信息：建议、节省金额、影响资源、优先级、状态。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去成本、资源、优先级、负责人。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 成本、资源、优先级、负责人
├─ Title + 13px description
└─ Primary: 采纳、忽略、查看资源
Primary Content Area\n├─ 核心信息或可视化工作区\n└─ 详情、结果或辅助信息区
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 成本、资源、优先级、负责人 | 32px 高；范围变化立即刷新数据 |
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

- **Primary**：采纳、忽略、查看资源，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 内容与状态设计

- 核心内容置于首个可视区域；辅助说明和原始数据放在第二层或可折叠区。
- 数字与时间右对齐或等宽显示；异常状态优先于健康状态。
- 图形、日志、画布和编辑器各自占用明确工作区，避免与普通表单混排。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“采纳、忽略、查看资源”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 成本、资源、优先级、负责人。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：成本、资源、优先级、负责人。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“成本优化工作台”。用当前任务的主信息结构组织首屏，次级信息按阅读路径分层。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 76. FinOps 资源拆分

- **应用 / 路由 / 实现**：集成中心 · `/integration/finops/resources` · `FinOpsResources.vue`

### 1. 页面定位

- 核心用户：平台管理员、对应业务负责人。
- 主任务：定位资源利用率与成本。
- 高频任务：筛选、查看详情、筛选当前范围、进入详情或结果。
- 首要信息：资源、账号、区域、成本、利用率、状态。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去资源、成本、利用率、账期。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 资源、成本、利用率、账期
├─ Title + 13px description
└─ Primary: 筛选、查看详情
Filter Toolbar\n├─ Search + core filters\n└─ Reset / export / batch actions\nData Area\n├─ Primary data table or result list\n└─ Pagination + selection feedback
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 资源、成本、利用率、账期 | 32px 高；范围变化立即刷新数据 |
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

- **Primary**：筛选、查看详情，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 表格设计

- 列顺序：名称/标识（220px）→ 资源、成本、利用率、账期（160px）→ 状态（100px）→ 最近时间/年龄（150px）→ 操作（140px fixed-right）。
- 关键列固定：名称与操作；长地址、表达式、响应和备注允许省略，Hover 显示完整值。
- 状态使用 Tag；ID、地址、命令、查询、版本和时间戳使用等宽字体；次级元信息在名称下第二行弱化。
- 行高 44px，Hover `#F8FAFD`；点击名称进入详情，勾选后显示批量操作条。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“筛选、查看详情”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 资源、成本、利用率、账期。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：资源、成本、利用率、账期。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“资源成本分析”。保持列表工作台节奏：筛选、扫描、行内操作、详情回看；不堆叠无业务价值的 KPI。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

## 77. FinOps 账单同步

- **应用 / 路由 / 实现**：集成中心 · `/integration/finops/sync` · `FinOpsSync.vue`

### 1. 页面定位

- 核心用户：平台管理员、对应业务负责人。
- 主任务：配置同步并追踪同步历史。
- 高频任务：立即同步、保存设置、筛选当前范围、进入详情或结果。
- 首要信息：账期、账号、同步范围、结果、时间。

### 2. 当前 UI 问题

- 标题、上下文和首要操作需要固定在同一视觉带，避免用户只看见表格或内容主体后失去账号、账期、同步、错误。
- 辅助筛选与主操作不应同权；当前页必须只有一个高强调主操作，其余为描边或文字按钮。
- 状态信息需从大面积高饱和色块收敛为标签、数字和局部色条，避免影响异常扫描。
- 内容区需在 1366px 视口下保持首屏可读，避免空白、长字段和行操作挤压关键数据。

### 3. 整体布局设计

```text
Page Header（24px page padding）
├─ Breadcrumb / 账号、账期、同步、错误
├─ Title + 13px description
└─ Primary: 立即同步、保存设置
Primary Content Area\n├─ 核心信息或可视化工作区\n└─ 详情、结果或辅助信息区
```

### 4. 关键组件

| 组件 | 用途与位置 | 视觉层级 / 交互 |
| --- | --- | --- |
| PageHeader | 顶部，承载标题、说明与主操作 | 白底，无大阴影；主操作位于右侧 |
| ContextBar | Header 下方，展示 账号、账期、同步、错误 | 32px 高；范围变化立即刷新数据 |
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

- **Primary**：立即同步、保存设置，实心蓝色按钮；同一屏最多一个。
- **Secondary**：查询、刷新、筛选、导出，描边按钮或工具栏控件。
- **Tertiary**：查看、复制、跳转、展开，表格行内文字按钮。
- **More**：不常用操作收进下拉菜单；**Danger**：删除、终止、关闭或不可逆执行必须二次确认。

### 7. 内容与状态设计

- 核心内容置于首个可视区域；辅助说明和原始数据放在第二层或可折叠区。
- 数字与时间右对齐或等宽显示；异常状态优先于健康状态。
- 图形、日志、画布和编辑器各自占用明确工作区，避免与普通表单混排。

### 8. 状态设计

- Loading：内容区使用骨架屏，保留 Header 与筛选上下文。
- Empty：说明当前范围无数据，提供“立即同步、保存设置”或调整筛选的下一步。
- Error / Permission Denied：显示原因、重试入口与必要的权限申请说明。
- Running / Success / Warning / Failed / Disabled：使用统一 StatusTag；运维状态补充 Pending、Timeout、Disconnected、Unknown、Partial Success、Terminating。

### 9. Dialog / Drawer

- 创建/编辑：字段少于 10 个使用 640px Dialog；字段多、需保留当前对比时使用 720px Drawer。
- 详情、日志、原始响应和 YAML：使用 80vw Drawer；正文可滚动，Header 固定显示 账号、账期、同步、错误。
- 删除、终止或高风险执行：480px Confirm Dialog；Footer 左侧风险说明，右侧为取消与危险确认，确认按钮不使用默认主色。

### 10. SRE 专业设计

- 页面仅持续展示对当前任务有价值的上下文：账号、账期、同步、错误。
- 所有会改变运行状态的操作必须展示目标范围、执行时间、结果与审计入口。
- 对异常、未知、部分成功和断连，优先给出可执行的下一步，而不是只显示颜色。

### 11. 页面专属设计

- 工作模式定位为“账单同步控制”。用当前任务的主信息结构组织首屏，次级信息按阅读路径分层。
- 实施后使用 1366px 桌面视口核验标题、主操作、首屏内容、弹层与空/错状态；不改变现有路由、接口与交互逻辑。

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

