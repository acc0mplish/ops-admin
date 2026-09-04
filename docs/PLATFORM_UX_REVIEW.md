# Ops Admin 플랫폼 UX 리뷰 보고서

리뷰 일시: 2026-07-06  
리뷰 역할: 플랫폼 일반 운영 사용자 / Application 배포 사용자 / 모니터링 당직 사용자  
리뷰 범위: 현재 로컬 코드, Frontend Route, 주요 Page Component, Backend API 등록과 Frontend 운영 Build 1회.

> 안내: 이번 리뷰는 “사용자 경험 점검 + 코드 수준 기능 맵 + Build 검증”을 중심으로 진행했으며, 실제 MySQL, Kubernetes, Prometheus, SSH Asset, Git/SVN Repository에 연결한 완전한 End-to-End 통합 검증은 수행하지 않았습니다. 따라서 외부 System이 관련된 권고는 “통합 검증 필요”로 표기합니다.

## 1. 전체 결론

플랫폼은 단일 운영 Backend에서 비교적 완전한 운영 Platform의 모습으로 확장되었으며, 현재 다음을 포함합니다:

- Console: 대시보드, 시스템 관리, Log Audit.
- 자산 관리: Host, Host Group, Credential, Cloud Account, Database, K8s, Web SSH, DBMS Workbench.
- 표준 운영: Script Library, Quick Execution, Scheduled Task, Job Orchestration, Job History, Manual Approval.
- 애플리케이션 센터: Project 목록, Build Task, Build History.
- 메시지 통지: Template, Channel, Rule, Send Log.
- 모니터링 센터: Datasource, Instant Query, Alert Rule, Alert Event, Alert Silence, Alert Aggregation, Monitoring Dashboard, Inspection Dashboard.

전체 방향은 맞습니다. 플랫폼이 “Asset 등록 -> Change 실행 -> Job Orchestration -> Notification -> Monitoring 피드백 -> Application Build/배포”의 주요 Chain을 이미 커버하고 있습니다.  
현재 가장 큰 경험 문제는 기능 수가 아니라 “Information Architecture, 문구 일관성, Page 복잡도, 오류 Message 품질, 핵심 Flow 완결성”을 계속 다듬어야 한다는 점입니다.

## 2. 검증 결과

실행한 Command:

```bash
cd web
cmd /c npm run build
```

결과: Build 통과.

Build 안내:

- Vite/Rollup의 Dependency 주석 안내는 동작에 영향을 주지 않습니다.
- Frontend Bundle 크기가 500 KB를 초과하므로 이후 Route 단위 Code Splitting을 권장합니다.

## 3. 전체 경험 피드백

### P0: 중국어 Encoding과 오류 Message는 여전히 통일된 정리가 필요

사용자 경험 영향: 높음.

현상:

- `README.md`는 현재 Terminal에서 읽을 때 여전히 많은 깨진 문자를 표시합니다.
- `backend/service/ops_schedule.go`에도 깨진 문자 오류 Message가 남아 있습니다. 예: Request Header, Script 선택, Target 선택 안내.
- 이런 오류가 Frontend Toast나 API 응답에 한 번이라도 나타나면 사용자는 System이 “불안정”하거나 “신뢰할 수 없다”고 느낍니다.

권장:

- 프로젝트 전체를 UTF-8 Encoding으로 통일합니다.
- `README.md`, Backend 오류 Message, 구버전 Page 문구를 한 번에 집중 스캔하고 수정합니다.
- CI에 Encoding/깨진 문자 Scan을 추가합니다. 예: `�`, `U+7487`, `U+9D59`, `U+93CB`, `U+939B` 같은 고위험 문자 검출.

우선순위: P0.

### P0: 일부 Page 파일이 지나치게 커서 장기 유지보수 위험이 높음

사용자 경험 영향: 중상. 개발 유지보수 영향: 높음.

현재 규모가 큰 Page:

- `web/src/views/assets/K8s.vue`: 약 1772줄.
- `web/src/views/monitor/MonitorDashboard.vue`: 약 1578줄.
- `web/src/views/assets/DatabaseWorkbench.vue`: 약 1271줄.
- `web/src/views/ops/OpsJobDesigner.vue`: 약 873줄.

권장:

- K8s: Page 상태 Composable, Resource Table Component, Yaml Editor Component, Istio/Gateway Component로 계속 분리합니다.
- Monitoring Dashboard: Dashboard 목록, Panel Renderer, Panel Editor, Report Exporter, Data Query Composable로 분리합니다.
- DBMS: SchemaTree, SqlEditor, ResultGrid, TransferTaskPanel, RowEditor로 분리합니다.
- Job Orchestration: Canvas, NodePalette, NodeConfigPanel, TemplateImporter로 분리합니다.

우선순위: P0/P1.

### P1: Application 전환과 Module 메뉴는 명확해졌지만 최상위 Application이 계속 늘어남

현재 최상위 Application은 Console, 자산 관리, 표준 운영, 애플리케이션 센터, 메시지 통지, 모니터링 센터입니다.

경험 장점:

- “애플리케이션 센터”가 독립적으로 분리되어 표준 운영에 섞이지 않게 되었고 사용자 Mental Model에 부합합니다.
- K8s, Database, Monitoring 같은 전문 Module도 비교적 명확한 Entry를 갖습니다.

문제:

- 이후 CMDB, Audit, 보안, 비용, Ticket 등을 계속 추가하면 Application 전환 Popup이 길어집니다.

권장:

- Application 전환 Popup에 검색을 지원합니다.
- 자주 쓰는 Application 고정(Pin)을 지원합니다.
- 각 Application Entry 아래에 한 줄 역할 설명과 최근 접속 시각을 표시합니다.

우선순위: P1.

### P1: 고위험 작업에는 더 강력한 “결과 경고”가 필요

이미 확인 대화상자가 있지만 대부분 일반 확인입니다.

다음 작업에 대해 경고를 강화합니다:

- DBMS 데이터 Row 삭제, Import 덮어쓰기, DDL/DML 실행.
- K8s Resource 삭제, YAML 편집, 일괄 Image 수정.
- Command/Script Quick Execution, File Distribution.
- Job Orchestration 실행, Scheduled Task 즉시 실행.
- 애플리케이션 센터 즉시 Build, 배포 Script 실행.

권장 형식:

- 영향 범위를 명확히 표시: Target Host, Namespace, Database, Table, Build Task.
- 되돌릴 수 없는 위험을 표시합니다.
- 고위험 작업은 Resource 이름 재입력 확인을 요구합니다.

우선순위: P1.

## 4. 자산 관리 경험 피드백

### 자산 개요

장점:

- Host, Database, K8s를 하나의 Overview 시각으로 통합하기 시작했습니다.
- Health 알림, Resource 분포, K8s 정보가 사용자가 Platform 연동 상태를 빠르게 판단하는 데 도움이 됩니다.

문제:

- “Health 알림”에는 더 많은 실행 가능한 Entry가 필요합니다. 예: 오프라인 Host 클릭 시 필터링된 Host 목록으로 이동.
- Resource 분포는 “Host 출처, Environment, Cloud Provider, Business Group”을 명확히 구분해야 하며, 현재는 Data가 누락되면 오해를 일으키기 쉽습니다.

권장:

- 모든 통계 Card를 클릭 가능한 Entry로 만듭니다.
- Data가 없을 때 “왜 비어 있는지”와 “Config로 이동” Button을 표시합니다.
- K8s 통계에는 Cluster 수, Node 수, 오류 Pod, Workload 수 추가를 권장합니다.

우선순위: P1.

### Host 관리 / Host Group 관리

장점:

- Host Group에서 그룹 내 Server를 조회하는 기능을 지원합니다.
- “Host 삭제”와 “Host Group에서 제외”를 구분했으며, 이는 매우 중요한 의미 수정입니다.

문제:

- 일괄 작업이 많아 사용자가 현재 “전체 Host 목록”인지 “특정 Host Group View”인지 혼동하기 쉽습니다.

권장:

- Host Group View 상단에 현재 Host Group 이름, Host 수, 돌아가기 Entry를 고정 표시합니다.
- 일괄 Button 문구는 Context에 따라 바꿉니다. 예: “현재 그룹에서 일괄 제외”를 “Host 일괄 삭제”와 섞어 쓰지 않습니다.
- Host 상세에는 “소속 Host Group” Label 목록 추가를 권장합니다.

우선순위: P1.

### Database 관리 / DBMS Workbench

장점:

- Database Asset 등록, Connection Test, SQL 편집, 실행 Record, Result Set 편집, Import/Export Task를 커버합니다.
- 직접 연결과 SSH Gateway 접속을 지원해 Database가 내부망에 있는 일반적인 운영 시나리오를 커버합니다.
- SQL Editor는 이미 Syntax Highlight, 기본 Autocomplete, Result Set 표시 기능을 갖추고 있습니다.
- 실행 History, Rollback SQL, Data Import/Export로 경량 DBMS의 기본 Loop를 형성했습니다.
- 전체 기능 목표는 경량 Navicat에 가깝고 운영 Platform 내부의 통합 Database 작업 Entry로 적합합니다.

문제:

- DBMS는 Platform에서 가장 위험도가 높은 Module 중 하나이지만 현재 Permission Granularity가 여전히 거칠어 “연결, Query, 수정, DDL, Import/Export, 관리 Task” 같은 작업 수준 Authorization이 없습니다.
- Database Asset에 저장된 Account는 Query와 쓰기 Permission을 동시에 가질 수 있으며 Platform은 아직 읽기 전용 Connection과 읽기/쓰기 Connection을 명확히 구분하지 않습니다.
- SQL Editor는 Highlight와 Autocomplete를 갖췄지만 실행 전 Target Environment, Database, SQL 유형, 예상 영향 범위를 충분히 표시하지 않습니다.
- 여러 SQL을 동시에 실행할 때 사용자가 실제 실행 경계, 실행 순서, Transaction 상태, 실패 후 처리 방식을 확인하기 어렵습니다.
- SQL History에는 실행자, 발신 IP, Target Environment, Target Instance, Schema, 영향 Row 수, 실패 원인을 더 보완해야 문제 추적 요구를 충족할 수 있습니다.
- 자동 생성 Rollback SQL은 “안전하게 복구할 수 있다”는 착각을 주기 쉽지만 UPDATE, DELETE, DDL, 일괄 Import는 반드시 신뢰할 수 있게 Rollback되는 것은 아닙니다.
- Result Set 직접 편집에는 Primary Key, Unique Key, 동시 수정 검사가 없어 잘못된 Row 업데이트나 다른 사용자 수정 덮어쓰기 위험이 있습니다.
- Import Task에는 완전한 실행 전 Pre-check가 없어 Field 불일치, Charset, 중복 Key, 대용량 Data Import 문제가 실행 단계에서야 드러날 수 있습니다.
- Export Task는 민감 Field, Data 양, File 유효 기간, Download Permission을 고려해 새로운 Data 유출 Entry가 되지 않도록 합니다.

최적화 권장:

#### P0: Permission과 실행 보안

- Database Connection Mode를 추가합니다: `읽기 전용`, `읽기/쓰기`. 읽기 전용 Connection은 Backend에서 INSERT, UPDATE, DELETE, REPLACE, DDL, Stored Procedure 호출, 다중 Statement 쓰기 작업을 강제 차단하며 Frontend Button 숨김에만 의존해서는 안 됩니다.
- DBMS Permission Point를 분리합니다: Asset 조회, Connection Test, Data Query, Result Set 편집, DML 실행, DDL 실행, Import, Export, SQL History 조회, Rollback 실행.
- 운영 Environment에는 더 엄격한 Policy를 적용합니다: 기본 읽기 전용, 쓰기 작업 2차 확인, DDL 강제 확인, 대량 Update/Delete 제한.
- 실행 전 Backend가 SQL을 해석해 SELECT, DML, DDL, DCL, Transaction Statement, 알 수 없는 Statement를 식별합니다. 문자열 Prefix만으로 판단해서는 안 됩니다.
- DML/DDL 확인 대화상자에는 Environment, Instance, Schema, SQL 유형, Statement 수, Transaction 사용 여부, 위험 등급, 전체 SQL 요약을 표시해야 합니다.
- UPDATE/DELETE에 WHERE 조건이 없으면 고위험으로 표시하고 DROP, TRUNCATE, ALTER, GRANT, REVOKE는 기본적으로 고위험으로 표시합니다.
- 고위험 작업은 Database 이름 또는 Table 이름 입력 확인을 요구하며 Backend가 Permission과 위험 Rule을 다시 검증합니다.
- Query Timeout, 최대 반환 Row 수, 최대 영향 Row 수, 최대 Export Row 수를 설정해 실수로 Database나 Platform이 무너지지 않게 합니다.

#### P1: Audit과 결과 추적

- SQL History는 최소한 실행 시각, 실행자, Client IP, Environment, Database Asset, 연결 방식, Schema, SQL 유형, SQL 내용 요약, 영향 Row 수, 반환 Row 수, 소요 시간, 실행 상태, 오류 Message를 기록합니다.
- 실행마다 고유 실행 번호를 생성해 SQL Editor, Result Set, Import/Export Task, Audit Record 사이를 상호 이동할 수 있게 합니다.
- 다중 Statement 실행은 Statement별로 결과를 분리 표시해 몇 번째가 성공/실패했는지, Rollback 여부, Transaction 최종 상태를 명확히 합니다.
- SQL 내용의 Password, Token, 주민등록번호 같은 민감 정보는 Rule에 따라 Masking한 뒤 Log에 기록합니다.
- Operation Log와 SQL History는 구분합니다. Operation Log는 “누가 어떤 기능을 호출했는지”를, SQL History는 “Database가 실제로 무엇을 실행했는지”를 기록합니다.
- 실행자, Environment, Instance, Schema, SQL 유형, 상태, 시간 범위로 SQL History를 필터링합니다.

#### P1: Rollback SQL 신뢰도

- Rollback SQL에 신뢰 등급을 추가합니다:
  - `높음`: 실행 전 완전한 이전 값과 명확한 Primary Key를 기반으로 생성해 유일한 Record를 특정할 수 있습니다.
  - `중간`: Primary Key 조건은 있지만 일부 Field나 Context가 변경되었을 수 있습니다.
  - `낮음`: 원본 SQL에서 유추한 것이며 Data 완전 복구를 보장할 수 없습니다.
  - `Rollback 불가`: DDL, TRUNCATE, 이전 값 없는 DELETE, 외부 Side Effect 작업.
- Rollback Entry는 기본적으로 “조회와 복사”만 제공하며 자동 생성 SQL을 원클릭 안전 복구로 표현해서는 안 됩니다.
- Rollback 실행 전 Target Environment, Schema, Primary Key 조건, 현재 Data Version을 다시 검증하고 위험 확인을 한 번 더 수행합니다.
- Result Set 편집은 Primary Key 또는 Unique Key로 WHERE 조건을 생성합니다. 유일하게 특정할 Field가 없으면 직접 편집을 금지합니다.
- 선택 사항으로 Optimistic Lock 검사를 추가해 원본 Field 값을 Update 조건에 넣어 다른 Session이 수정한 Data를 덮어쓰지 않게 합니다.

#### P1: Import와 Export

- Import Task에 Pre-check Stage를 추가해 Target Table, File Encoding, 구분자, Field Mapping, Field 유형, Null Rule, 중복 Key Strategy, 예상 영향 Row 수를 표시합니다.
- 중복 Key Strategy는 중지, 무시, 덮어쓰기 Update를 명확히 지원하고 실행 전 해당 위험을 표시합니다.
- 소량 Data Preview와 오류 샘플 Download를 제공하며 Pre-check 미통과 시 정식 실행을 금지합니다.
- 대용량 Data Import는 Background Task로 처리해 진행률, 성공 수, 실패 수, Skip 수, Task 취소, 실패 상세를 지원합니다.
- Database 간 Import는 Source DB, Target DB, Field Mapping, Type Conversion Rule을 명확히 하고 묵시적 Data Truncation을 금지합니다.
- Export Task는 Field 선택, Query 조건, Data Masking, File Format, 최대 Row 수 설정을 지원합니다.
- Export File에 유효 기간과 접근 Permission을 설정하고 Download 행위를 Operation Log에 기록하며 만료 File은 자동 정리합니다.

#### P2: Editor와 상호작용 경험

- SQL Editor Autocomplete Context에는 현재 Schema, Table, Field, Function, SQL Keyword가 포함되어야 합니다.
- 실행 Button은 “선택 SQL 실행”과 “전체 SQL 실행”을 구분하고 텍스트를 선택하지 않으면 실제 실행 범위를 명확히 안내합니다.
- SQL Format, 주석, 찾기/바꾸기, Execution Plan, 단축키 안내를 추가합니다.
- Result Set은 Field 필터, 정렬, Column 고정, NULL 표시, 긴 텍스트 Preview, 현재 결과 Export를 지원합니다.
- Result Set 편집에 미저장 상태 안내를 추가해 Page를 떠나기 전 수정을 버릴지 확인합니다.
- 오류 Message는 Database 원본 오류와 사용자가 이해할 수 있는 처리 권고를 함께 표시하되 Database Password 같은 민감한 연결 정보를 노출해서는 안 됩니다.

검수 기준:

1. 읽기 전용 Connection은 Frontend Button, 수동 API 요청, 다중 Statement SQL로도 쓰기 작업을 실행할 수 없습니다.
2. DML/DDL 실행 전 Target Environment, Database, Schema, SQL 유형, 위험 등급을 정확히 표시합니다.
3. WHERE 없는 UPDATE/DELETE와 고위험 DDL은 강화된 확인을 거쳐야 합니다.
4. SQL History로 실행 번호 기준 실행자, Client IP, Target DB, 영향 Row 수, 소요 시간, 실패 원인을 완전히 추적할 수 있습니다.
5. 신뢰할 수 있게 복구할 수 없는 SQL은 “안전하게 Rollback 가능”으로 표시하지 않고 신뢰 등급 또는 Rollback 불가를 명확히 표기합니다.
6. Result Set 편집은 Primary Key 또는 Unique Key로 특정해야 하며 유일하게 특정할 수 없는 Result Set은 읽기 전용을 유지합니다.
7. Import Task는 Pre-check를 먼저 완료해야 하며 정식 실행 후 진행률과 Row별 오류 상세를 확인할 수 있습니다.
8. Export Task는 Permission 검증, 민감 Field 처리, File 유효 기간, Download Audit를 갖춥니다.

종합 우선순위: **P0/P1**. 읽기 전용 Mode, SQL 위험 식별, Backend Permission 검증, Audit Field를 먼저 완성한 뒤 고급 편집 기능 확장을 계속할 것을 권장합니다.

### K8s 관리

장점:

- Kuboard 스타일 Console을 형성했습니다.
- Cluster 관리, Cluster 전환, Overview, Node, Namespace, Workload, Pod, Service, Ingress, 고급 Network, Config Storage를 지원합니다.
- YAML Editor는 좌우 편집/Preview, 검색, Line 번호, 현재 Line Highlight를 구현했습니다.
- Workload 일괄 Image 수정을 지원하며 배포 운영에 매우 실용적입니다.

문제:

- `K8s.vue`는 여전히 지나치게 커서 이후 Iteration에서 서로 영향을 주기 쉽습니다.
- K8s Resource 작업이 많아 Permission 경계를 더 명확히 해야 합니다.
- Image 수정이 Argo에 의해 Rollback될 수 있으므로 Platform이 GitOps 시나리오를 식별해야 합니다.

권장:

- GitOps 안내 추가: Argo CD/Flux 관리 Label 또는 owner가 감지되면 “Cluster를 직접 수정하기보다 Git Repository 수정을 권장”한다고 안내합니다.
- Workload 일괄 Image 수정은 “GitOps Patch/YAML 생성”을 지원합니다.
- Pod Terminal Entry에 Audit 안내와 Session 기록을 추가합니다.
- YAML 편집에 “변경 Line만 표시/전체 표시” 전환을 추가합니다.

우선순위: P1.

## 5. 표준 운영 경험 피드백

### Script Library

장점:

- Script 추가/삭제/수정/조회, 활성화/비활성화, Script 유형, 기본 Parameter, Timeout을 지원합니다.
- Editor는 Code Highlight 방향으로 최적화되고 있습니다.

문제:

- Script Library는 Platform에서 재사용성이 높은 Asset이므로 Version 관리가 필요합니다.

권장:

- Script Version History를 추가합니다.
- Script 배포 상태를 추가합니다: Draft, 배포됨, 폐기됨.
- Script Parameter Schema를 추가합니다. 예: Parameter 이름, 유형, 기본값, 필수 여부.
- Script 시험 실행 Entry를 추가합니다.

우선순위: P1.

### Quick Execution

장점:

- Command Execution, Script Execution, File Distribution, Quick Execution History로 분리되어 있습니다.
- Target Host와 Host Group의 상호 배타적 선택은 올바른 설계입니다.
- 실행 시 Popup으로 Task 결과를 표시하고 닫은 뒤 History 확인으로 안내하는 것은 사용자 기대에 부합합니다.

문제:

- Quick Execution은 고위험 기능이므로 더 강한 Target 가시성이 필요합니다.

권장:

- 실행 전 최종 Target Host 수, 동시 실행 수, Timeout, 예상 위험을 표시합니다.
- 실행 전 임시 Template 저장을 지원합니다.
- 실패 재시도를 지원하며 실패한 Host만 재시도합니다.
- File Distribution에는 Checksum 검증 추가를 권장합니다.

우선순위: P1.

### Scheduled Task

장점:

- Task 목록, Task Log, Task Template이 있습니다.
- Script Task와 HTTP Probe Task를 지원합니다.
- 일괄 활성화, 비활성화, 삭제를 지원합니다.
- Notification Rule을 지원합니다.

문제:

- Cron Expression은 일반 운영 사용자에게 충분히 친화적이지 않습니다.

권장:

- Cron 시각화 Generator를 추가합니다.
- “다음 실행 시각”을 표시합니다.
- Task Log에 “예약 Trigger / 수동 Trigger” 출처 Field를 추가합니다.
- HTTP Probe는 응답 내용 Assertion, Header Assertion, 지연 Threshold를 지원합니다.

우선순위: P1.

### Job Orchestration

장점:

- Job Orchestration, Job 목록, Manual Approval, Job History, Job Template 다섯 Page가 있습니다.
- Script Execution, File Distribution, Manual Approval, Message Notification Node를 지원합니다.
- Node 삭제, 선택 Config 같은 핵심 Canvas 상호작용이 점진적으로 수정되고 있습니다.

문제:

- Job Orchestration은 복잡한 상호작용이라 현재 더 강한 Flow 검증과 Preview가 필요합니다.

권장:

- 저장 전 검증: 고립 Node 존재 여부, Cycle 존재 여부, 미구성 Node 존재 여부.
- “시험 실행/Dry Run” Mode를 추가합니다.
- Node는 복사, 붙여넣기, 실행 취소, 다시 실행을 지원합니다.
- Job History에 목록만이 아니라 DAG 실행 상태를 표시합니다.
- Manual Approval Page에 Timeout Policy를 추가합니다: Timeout 실패, Timeout Skip, Timeout 자동 거부.

우선순위: P1/P2.

## 6. 애플리케이션 센터 경험 피드백

### Project 목록

장점:

- 애플리케이션 센터가 최상위 Application으로 독립 존재하며 Entry가 올바릅니다.
- Project 목록은 Repository Meta 정보를, Build Task는 Script를 담당하는 역할 분리가 올바릅니다.

문제:

- Project Entity는 현재 “Source Repository”에 가깝고 앞으로 “Application, Service, Module, Repository”를 구분해야 할 수 있습니다.

권장:

- Application 담당자, Business Line, Environment 목록, 기본 배포 방식을 추가합니다.
- Repository 주소에 Connection Test를 지원합니다.
- Git Credential 구성을 지원하고 실행 Environment의 기존 Permission에만 의존하지 않습니다.

우선순위: P1.

### Build Task

장점:

- Build Task는 Project Repository 주소에 의존하며 CI/CD Model에 부합합니다.
- 새 Build Task Popup은 Wide Workbench로 바뀌어 Script 편집 경험이 더 좋습니다.
- 즉시 Build, 활성화/비활성화, 복제, Log 조회를 지원합니다.

문제:

- Build Script와 배포 Script는 여전히 자유 텍스트라 표준 Stage Model이 없습니다.

권장:

- Pipeline Stage를 도입합니다: Source Checkout, Dependency 설치, Test, Build, Image Build, 배포, Health Check.
- Build Parameter를 지원합니다. 예: Branch, tag, Image Version, Environment Variable.
- Build Task는 수동 Trigger 시 Branch와 Version Override를 지원합니다.
- Build Queue와 Build 취소를 추가합니다.
- Build Runner/작업 Directory를 격리해 여러 Task가 Directory를 공유하며 서로 오염되지 않게 합니다.

우선순위: P1.

### Build History

장점:

- Build History는 Stage를 Card로 표시하며 Log를 조회하고 Download할 수 있습니다.
- Build Task에서 필터 조건을 포함해 이동하는 것을 지원합니다.

문제:

- 현재 Stage 표시는 여전히 Log Field에서 유추하므로 Backend가 구조화된 Stage 상세를 저장하는 것이 좋습니다.

권장:

- `build_stage` Table 또는 JSON Field를 추가해 Stage 상태, 시작 시각, 종료 시각, 소요 시간, Log를 기록합니다.
- Build Log는 실시간 Streaming 조회를 지원합니다.
- Build 실패 시 실패 Stage, 마지막 50줄 Log, 재시도 Button을 표시합니다.
- Rollback Button은 현재 비활성화되어 있으며 이후 Rollback Strategy를 명확히 합니다: 이전 Version 재배포, Rollback Script 실행, GitOps Rollback.

우선순위: P1.

## 7. 메시지 통지 경험 피드백

장점:

- Message Template, Notification Channel, Notification Rule, Send Log를 추상화했습니다.
- DingTalk, WeCom, Feishu, 사용자 정의 HTTP Webhook을 지원합니다.
- Job Orchestration에서 Message Notification이 전역 Switch가 아니라 Step Node로 동작하는 것은 Orchestration 모델에 부합합니다.

문제:

- 사용자는 Template 구성 시 사용 가능한 Variable을 모를 수 있습니다.

권장:

- Template Editor 오른쪽에 Variable Panel을 표시합니다. 예: `{{title}}`, `{{status}}`, `{{detail}}`.
- Notification Rule은 “Test 발송”을 지원합니다.
- Send Log는 상태, Channel, Rule, 시간으로 필터링을 지원합니다.
- Webhook 실패 시 응답 Status Code, 응답 Body, 재시도 횟수를 표시합니다.

우선순위: P1.

## 8. 모니터링 센터 경험 피드백

### Datasource와 Instant Query

장점:

- 다중 Datasource 전환을 지원합니다.
- Instant Query는 PromQL Debug Entry로 쓸 수 있습니다.

권장:

- Datasource 목록에 Health 상태, 최근 Probe 시각, Version을 표시합니다.
- PromQL Editor는 자주 쓰는 Metric Template을 지원합니다.
- Instant Query 결과는 Table/Chart 전환을 지원합니다.

우선순위: P2.

### Alert Rule / Alert Silence / Alert Aggregation

장점:

- Alert Rule이 Notification Rule과 연동되어 있습니다.
- Alert Silence와 Alert Aggregation은 Rule 이름 Regex, Rule Dropdown 다중 선택을 지원하며 방향이 올바릅니다.

문제:

- Alert Rule의 Life Cycle은 더 완전해질 수 있습니다.

권장:

- Alert Rule에 Preview를 추가합니다: 현재 PromQL로 한 번 조회해 Hit한 Series를 표시합니다.
- Alert Event에 Timeline을 추가합니다: 발생, Notification, 인계, 복구, 종료.
- Alert Silence Rule에 “현재 어떤 Alert Rule에 Hit하는지” Preview를 추가합니다.
- Alert Aggregation Rule에 “수렴 효과 예측”을 추가합니다. 예: N개 Notification이 M개로 수렴할 것으로 예상.

우선순위: P1/P2.

### Monitoring Dashboard / Inspection Dashboard

장점:

- Monitoring Dashboard와 Inspection Dashboard로 분리되어 “하나의 Page에서 Mode를 전환”하던 인지 부담을 해소했습니다.
- Datasource 전환을 지원합니다.
- K8s Dashboard는 Grafana 스타일로 진화하고 있습니다.

문제:

- 현재 Dashboard 기능은 Grafana/Nightingale에 비해 Layout 편집 자유도가 부족합니다.

권장:

- Panel은 Drag로 크기와 순서 조정을 지원합니다.
- Dashboard는 Variable을 지원합니다: Cluster, Node, Namespace, Pod, Time Range.
- Inspection Dashboard는 PDF Report Template 구성을 지원합니다.
- Panel은 Threshold 색상, 단위 Format, TopN, Table, Trend Chart, Stat, Gauge를 지원합니다.

우선순위: P1.

## 9. 시스템 관리 경험 피드백

장점:

- 기본 User, Role, Menu, 부서, 직무, Login Log, Operation Log가 완비되어 있습니다.

권장:

- Role Permission은 신규 Module의 세분화된 Action을 커버해야 합니다: 조회, 생성, 편집, 삭제, 실행, Approval, Terminal, Export.
- Operation Log는 고위험 작업의 Request 요약과 Resource 이름 저장을 권장합니다.
- Login Log에 실패 원인과 발신 IP 위험 표식을 추가합니다.

우선순위: P2.

## 10. 가장 우선 추진을 권장하는 10가지

1. 프로젝트 전체의 깨진 문자 문구를 수정합니다. 특히 Backend 오류 Message와 README.
2. 사용하지 않는 구버전 Page를 정리합니다. 예: 이전 `OpsApplicationCenter.vue`, 이후 오참조 방지.
3. K8s, Monitoring Dashboard, DBMS를 추가 Component로 분리합니다.
4. 고위험 작업의 2차 확인과 Audit Record를 통일합니다.
5. DBMS에 읽기 전용 Mode, SQL 위험 식별, 실행 전 확인을 추가합니다.
6. K8s에 GitOps 시나리오 식별을 추가해 Cluster를 직접 수정한 뒤 Argo에 Rollback되는 것을 방지합니다.
7. Quick Execution에 실패 재시도, Target 확인, Checksum을 추가합니다.
8. Build Center에 구조화 Stage, 실시간 Log, Build Parameter, Build 취소를 추가합니다.
9. Scheduled Task에 Cron 시각화와 다음 실행 시각을 추가합니다.
10. Monitoring Dashboard는 Variable, Drag Layout, PDF/Inspection Report Template을 지원합니다.

## 11. 권장 Iteration 리듬

### 1단계: 안정적 사용

- 깨진 문자를 수정합니다.
- 폐기된 구버전 Page를 삭제합니다.
- 고위험 작업 확인을 통일합니다.
- 핵심 API 오류 Message를 통일합니다.
- Frontend Route Lazy Loading으로 첫 화면 Bundle 크기를 줄입니다.

### 2단계: 핵심 Loop

- K8s GitOps 안내와 Image 변경 Loop.
- Build Task 실시간 Log와 Build Stage 구조화.
- Quick Execution 실패 재시도.
- DBMS SQL 위험 식별.
- 모니터링 알림 Preview와 Notification Test.

### 3단계: Platform화 역량

- Permission 세분화.
- Audit Center 통합.
- Job Orchestration DAG History View.
- Dashboard Drag Editor.
- 애플리케이션 센터의 완전한 CI/CD Pipeline 지원.

## 12. 사용자로서의 소감

사용자 입장에서 이 Platform은 이미 “내부 통합 운영 Platform”의 기본기를 갖추었다고 느낍니다. 특히 K8s, DBMS, 표준 운영, Monitoring, 애플리케이션 센터 Module을 조합하면 사용 시나리오가 매우 완전합니다.

하지만 두 가지가 걱정됩니다:

- 첫째는 안정성 신뢰입니다. 깨진 문자 오류 Message나 구버전 Page 잔재를 보면 Platform에 대한 신뢰가 약해집니다.
- 둘째는 고위험 작업 보호입니다. 이 Platform은 Host, Database, K8s, 배포 Pipeline을 다룰 수 있으므로 확인, Audit, Permission, Rollback을 계속 강화해야 합니다.

“깨진 문자 정리, 위험 작업 보호, 핵심 Flow 완결, Page 분리 유지보수성”을 먼저 잘 해내면 이 Platform은 “기능이 많은” 단계에서 “운영 Team이 매일 사용해도 정말 괜찮은” 단계로 올라설 것입니다.
