# OPS Admin 애플리케이션별 로컬 디자인 규범

## 목적과 경계

본 규범은 기존 Vue Page를 페이지 단위로 최적화하는 것을 안내합니다. 목표는 전문적이고 현대적이며 절제된 높은 정보 밀도의 OPS/SRE Platform입니다. 기존 Route·API·권한·상호작용을 유지하며 새로운 Business Flow를 추가하지 않습니다.

모든 Application이 공유하는 기본 언어: `#18243A` 주 텍스트, `#66758D` 보조 텍스트, `#E3E8F0` 경계, `#F5F7FB` Page 배경색, 12px Content Card 모서리 반경, 7px Control 모서리 반경. 각 Application은 "현재 상태, 핵심 Metric, 위험 등급"에만 이 Application 색을 사용해 Page 전체의 고채도 사용을 피합니다.

## 1. Console

**Page 범위**: Dashboard, Business Topology, 사용자/Role/메뉴/부서/직무, 시스템 설정, 로그인 Log, 작업 Log, 개인정보.

**로컬 방향**: "관리 진입점"과 "시스템 제어 가능성"을 강조합니다. Dashboard는 저채도 짙은 파란 상태 배너 1개 + 핵심 KPI 4개를 채택하고, 관리 목록은 제목·필터 Tool bar·Data Table 3단 구조를 채택하며, 감사 Log는 수량·위험·시간을 우선 훑어볼 정보로 사용합니다.

**Page 규칙**:

- 시스템 관리 목록의 생성 작업은 제목 영역 오른쪽에 고정하고, Query·동기화 등 보조 작업은 연회색 필터 Panel에 둡니다.
- Role·메뉴·부서 Tree Table에서는 계층을 들여쓰기·가는 구분선·아이콘으로 표현하고 넓은 색 블록을 사용하지 않습니다.
- Log Page의 성공·경고·실패는 상태 Label과 왼쪽 가는 색 띠만 사용하고, 상세는 Drawer로 Table 문맥을 유지합니다.
- 시스템 설정은 분할 Tabs + 2열 Config 영역을 사용하며, 미리보기는 Config 결과에만 기여하고 Form의 시각적 비중을 빼앗지 않습니다.

## 2. Asset 관리

**Page 범위**: Asset Overview, Environment Model, 터미널 로그인, Host/Host Group/Credential/Cloud Account, Database, Gateway, Database 상세·Tool Page.

**로컬 방향**: "Resource 상태"와 "조작 가능성"을 강조합니다. Asset Overview는 온라인·인증·연결·정보 완전도를 우선 표시하고, Resource 목록은 이름·주소·Environment·상태·최근 업데이트 시각을 우선 표시합니다.

**Page 규칙**:

- Overview Page 첫 화면은 Resource KPI·핵심 Resource 진입점·Health 알림으로 구성하고, Health 알림은 이상 또는 처리 대기 항목만 표시합니다.
- Host·Database·Gateway 목록은 온라인/연결 상태를 이름 오른쪽 또는 독립 상태 열에 두고, 주소·Label 등 부수 정보는 약화하여 표시합니다.
- Credential·Cloud Account 등 민감 Resource의 작업 영역은 "보기 / 수정 / 더 보기"를 사용하고, 위험 작업은 더 보기 메뉴에 넣어 위험 확인을 유지합니다.
- Terminal·DBMS·Import·Backup Page는 Tool Bench 레이아웃을 채택합니다: 고정 제목과 문맥, 명확한 편집 영역, 접을 수 있는 보조 영역, 안정적인 하단 작업 영역.

## 3. Container 관리

**Page 범위**: Service 관리, Service Resource Topology, Service 상세/Log/Health 진단, K8s Cluster, Node, Namespace, Workload, Pod, Network, Config·Storage, Terminal.

**로컬 방향**: "Cluster 문맥"과 "실행 상태 판단"을 강조합니다. 모든 Page는 제목 근처에 Cluster·Namespace·Resource Type을 상시 표시해 사용자가 깊은 Page에서 문맥을 잃지 않게 합니다.

**Page 규칙**:

- K8s Page는 Cluster context bar → 2차 Resource Tabs → 필터 Tool bar → Data 영역으로 통일합니다.
- Workload·Pod Table의 첫 열은 이름 + Namespace/Image 등 요약을 사용하고, Ready·재시작 횟수·Age·상태가 우선순위가 높은 열입니다.
- YAML·Log·Terminal 등 어두운 작업 영역은 Content 편집/출력 영역에만 사용하고, 주변 Navigation·필터·작업은 밝게 유지합니다.
- Resource Topology는 Health 상태·Service 관계·클릭 가능한 상세를 핵심으로 하며, 연결선과 장식은 정보 자체의 시각적 비중을 넘지 않습니다.

## 4. 표준 Ops

**Page 범위**: Script Library, Command 실행, Script 실행, 파일 배포, 실행 History, Schedule Task, Job Orchestration, Job 목록, Manual Approval, Job History·Template.

**로컬 방향**: "실행 전 확인, 실행 중 가시성, 실행 후 추적 가능성"을 강조합니다. Form은 마케팅식 큰 Card를 만들지 않으며, 위험과 실행 범위가 장식보다 중요합니다.

**Page 규칙**:

- Command·Script·파일 배포는 Task 정보 → 대상 범위 → 실행 Parameter → 위험 안내 → 주 작업 순서를 채택합니다.
- Command 편집기는 고정폭 Font와 가벼운 행 번호를 사용하며, 편집기만 어둡게 하고 Page의 나머지 영역은 밝게 유지합니다.
- 실행 History·Task Log·Job History는 상태·대상 수·시작 시각·소요 시간·실패 수를 가로로 훑을 수 있는 요약으로 만듭니다.
- Job Orchestration Canvas는 중성 Canvas를 채택하고, Node는 Type 색의 작은 면적으로 식별하며, 선택·오류·Manual Approval 대기는 충돌 없이 명확한 상태여야 합니다.

## 5. Application Center

**Page 범위**: Project 목록, Application Topology, Build Task, Build History, Image Registry, CI/CD Pipeline.

**로컬 방향**: "Delivery Chain"을 강조합니다. Project·Build·Image·Pipeline은 Environment·Version·상태·다음 작업을 한눈에 보여 주어야 합니다.

**Page 규칙**:

- Project·Build Task Page는 제목 작업 영역·필터 영역·결과 Table 3층 구조를 사용하고, Repository 주소와 Branch는 고정폭 보조 텍스트를 사용합니다.
- Build History는 Stage Timeline을 사용하고 성공/진행 중/실패 상태의 색 의미를 일관되게 유지하며, Log Drawer 또는 Dialog는 Log 본문에만 어둡게 합니다.
- Image Registry는 Card 목록으로 연결 상태와 주소를 표현하고, Credential 내용은 기본으로 가립니다.
- Pipeline 편집 Page는 Template 선택·Stage 편성·Stage Config·Run 상세를 명확히 구분하고, Deploy와 Manual Approval Node는 위험 안내를 사용합니다.

## 6. Notification

**Page 범위**: Message Template, Notification Channel, Notification Rule, Send Log.

**로컬 방향**: "Message가 정확히 도달하는가"를 강조합니다. 주 시각은 Channel·Routing·실패 원인·발송 시각에 기여해야 합니다.

**Page 규칙**:

- Template Page는 편집/미리보기 2단을 채택하고, 미리보기는 해당 Channel의 실제 정보 밀도를 모방하며 무의미한 Device Frame을 사용하지 않습니다.
- Notification Rule은 "Event → Template → Channel" Routing Chain으로 표시하고 현재 활성 상태를 명확히 보입니다.
- Send Log의 결과·응답 Code·소요 시간·실패 원인은 첫 화면에서 훑을 수 있어야 하고, 상세 요청/응답은 상세 Panel에 둡니다.

## 7. Integration Center

**Page 범위**: Navigation 관리, 공개 Navigation, AI 대화/Session/Model/Tool/Knowledge Base, FinOps Dashboard/Cloud Account/비용 분할/최적화 권고/Resource 분할/Billing 동기화.

**로컬 방향**: Config와 분석이 섞인 영역입니다. Navigation 관리는 진입점 조직을, AI는 Session·Tool 경계를, FinOps는 금액·추세·절감 가능 금액을 강조합니다.

**Page 규칙**:

- Navigation 관리는 Group Sidebar + 진입점 Card 작업 영역으로 표현하고, 진입점 Card는 아이콘·이름·주소 요약·상태만 사용합니다.
- AI 대화는 명확한 Session 바·Message 읽기 영역·고정 입력 영역을 사용하고, Tool 호출·생각 중·실패 상태는 간결한 상태 Block으로 표시합니다.
- Model·Tool·Knowledge Base는 Config형 Page로 Table/편집기 구조를 채택하며 연결 상태와 기본 Config를 보입니다.
- FinOps Page의 금액은 고정폭 숫자·Billing Month·전년 동기 설명을 채택하고, 추세·분할 Chart는 범례·빈 상태·필터 문맥을 갖추어야 합니다.

## 8. 모니터링 센터

**Page 범위**: 모니터링 개요, 스마트 대시보드, Datasource, 즉시 Query, Log, Trace, Alert Rule/Event/차단/집약, 모니터링 대시보드, 점검 대시보드.

**로컬 방향**: "위험 우선"과 "빠른 원인 파악"을 강조합니다. 현재 위험·담당자·지속 시간·영향 범위가 추세 장식보다 더 돋보여야 합니다.

**Page 규칙**:

- 모니터링 개요 첫 화면은 현재 위험·모니터링 품질·추세·최근 Event로 고정하고, 이상을 우선하며 Health 상태는 잡음을 줄여 표시합니다.
- Datasource·Rule·Alert Event는 상태 열·최근 업데이트 시각·담당자·바로가기 작업을 사용하고, 일괄 작업은 독립 Tool bar에 둡니다.
- PromQL·Log·Trace Page는 Query 영역·결과 영역·상세 영역의 Workbench 레이아웃을 채택하며, Query History와 Field Panel은 접을 수 있습니다.
- Command Center와 대시보드는 실시간 정보를 우선하고 일반 목록 Page의 큰 여백을 재사용하지 않지만, 명확한 제목·새로고침 상태·Time Range·Alert 진입점은 유지합니다.

## 페이지별 수용 체크리스트

각 Page를 구현한 뒤 Browser에서 확인해야 합니다:

1. 제목·설명·주 작업·필터 영역을 첫 화면에서 빠르게 찾을 수 있는가.
2. Table/Card가 통일된 경계·행 높이·빈 상태·로딩 상태·상태 색 의미를 갖추었는가.
3. Dialog·Drawer·Log/Terminal Tool 영역이 명확한 초점과 닫기 경로를 유지하는가.
4. 현재 Application·메뉴 계층·Breadcrumb·Tab이 위치감을 지속적으로 제공하는가.
5. 1366px 데스크톱 Viewport에서 불필요한 가로 Scroll이 없고 정보 밀도가 운영 시나리오에 맞는가.
