# Application Center CI/CD Pipeline 요구사항 분해

## 1. 배경과 목표

Application Center에는 현재 Project 관리, Build Task, Build History 기능이 있지만 Build와 Deploy는 아직 “단일 Task Script 실행”에 가깝습니다. 이번에 `CI/CD Pipeline`을 신규 추가하며, Jenkins, BlueKing Pipeline, Cloud Native Delivery Platform과 유사한 시각화 Pipeline 관리 기능을 제공하는 것이 목표입니다.

핵심 목표:

- Application Center에 독립된 2차 페이지 `CI/CD Pipeline`을 추가합니다.
- Template에서 Pipeline을 생성하는 것과 빈 Pipeline 생성을 모두 지원합니다.
- Application, 언어/Tech Stack, Environment, 상태로 Pipeline을 필터링합니다.
- Pipeline Stage Orchestration, Parameter 구성, 즉시 실행, 실행 History, Log 확인을 지원합니다.
- 이후 Manual Approval, Notification, Image Artifact, K8s Deploy, Rollback 등의 기능으로 단계적으로 확장할 수 있습니다.

## 2. 페이지 구조

Application Center는 다음 페이지로 분리할 것을 권장합니다:

- `Project 목록`: Application 기본 정보를 유지 관리하며 Git/SVN Repository Address를 포함합니다.
- `Build Task`: 현재의 단일 Task Build 기능을 유지합니다.
- `Build History`: 현재의 Build History와 Log 기능을 유지합니다.
- `CI/CD Pipeline`: 신규 추가되며 Pipeline 진입점입니다.
- `Pipeline 실행 History`: 신규 추가되며 Pipeline 목록에서 진입하거나, 모든 Pipeline Run 기록을 독립적으로도 확인할 수 있습니다.

## 3. CI/CD Pipeline 홈

### 3.1 상단 Overview

제목과 플랫폼 설명을 표시합니다:

- 제목: `CI/CD Pipeline`
- 설명: `Source Checkout부터 Build, Test, Artifact, Deploy, Notification까지 Application Delivery Flow를 통합 Orchestration합니다.`

우측 통계 Card:

- 전체 Pipeline
- 활성 건수
- 최근 실패 건수

### 3.2 상단 Tab

권장 Tab:

- `Code Repository`
- `Pipeline 목록`
- `Pipeline Template`
- `실행 기록`
- `Credential 관리`
- `Global Variable`

이번 단계에서 우선 구현:

- `Pipeline 목록`
- `Pipeline Template`
- `실행 기록`

### 3.3 Query 영역

필드:

- Keyword: Pipeline 이름, Application 이름, Repository Address
- Application: Dropdown 선택, Application Center Project 목록 재사용
- Environment: 전체, dev, test, staging, prod
- Tech Stack: 전체, Java, Node.js, Go, Python, Vue, 빈 Template
- 상태: 전체, 활성화, 비활성화

작업:

- Query
- 초기화
- 새 Pipeline

### 3.4 Pipeline 목록

Table 필드:

- Pipeline 이름
- 소속 Application
- Repository Address
- Default Branch
- Tech Stack
- Default Environment
- Stage 수
- 상태
- 최근 실행
- 생성 시각
- 작업

작업:

- 즉시 실행
- 수정
- 복제
- 실행 History
- 활성화/비활성화
- 삭제

## 4. Pipeline Template 선택 대화상자

Screenshot을 참고해 중앙 정렬의 큰 대화상자로 구현합니다.

### 4.1 대화상자 Layout

제목: `Pipeline Template 선택`

좌측 Category:

- 전체 Template
- Java
- Node.js
- Go
- Python
- Vue
- 빈 Template

우측 Template Card:

- Template 이름
- Template 설명
- Tech Stack 식별자
- Stage 수
- 권장 시나리오

하단 Button:

- 취소
- 빈 Pipeline
- 선택한 Template 사용

### 4.2 내장 Template

#### Go 백엔드 공통 Template

설명: `Go Compile, Image Push, Workload Update`

Default Stage:

1. Source Checkout
2. Go Dependency 설치
3. Unit Test
4. Go Compile
5. Docker Image Build
6. Image Push
7. K8s Workload Update

#### Maven Java 공통 Template

설명: `Maven Package, Jar Image, K8s Deploy`

Default Stage:

1. Source Checkout
2. Maven Dependency 설치
3. Unit Test
4. Maven Package
5. Docker Image Build
6. Image Push
7. K8s Deploy

#### Vue 프론트엔드 공통 Template

설명: `npm Build, Image Package, K8s Rolling Deploy`

Default Stage:

1. Source Checkout
2. npm install
3. npm run build
4. Docker Image Build
5. Image Push
6. K8s Rolling Deploy
7. Deploy Notification

#### 빈 Pipeline

설명: `빈 Canvas에서 시작해 모든 Stage를 사용자 정의`

Default Stage: 없음

## 5. Pipeline Editor

### 5.1 기본 정보

필드:

- Pipeline 이름, 필수
- 소속 Application, 필수. 선택하면 Repository Address, Repository Type, Default Branch가 자동으로 채워집니다
- Default Environment, 필수
- Default Branch, 기본값은 Application 구성
- Tech Stack
- 상태: 활성화, 비활성화
- 설명

### 5.2 Stage Orchestration

가로형 Stage Card 또는 Flowchart Layout을 채택합니다.

Stage Type:

- Source Checkout
- Shell Script
- Build Command
- Unit Test
- Docker Build
- Image Push
- K8s Deploy
- File Distribution
- Manual Approval
- Notification

Stage 구성 공통 필드:

- Stage 이름
- 활성화 여부
- Timeout, 기본값 1800초
- 실패 Policy: Pipeline 중지, 무시하고 계속, Manual Approval
- 실행 Directory
- 환경 변수

### 5.3 주요 Stage 구성

#### Source Checkout

- Repository Type: Git, SVN
- Repository Address: Application에서 자동으로 가져옴
- Branch/Tag
- Credential

#### Build Command

- Build Script, Code 하이라이트
- Workspace
- Build Parameter
- Cache Policy

#### Docker Build

- Dockerfile 경로
- Build Context
- Image Registry
- Image 이름
- Image Tag Rule

#### Image Push

- Image Registry Credential
- Push Address
- Push 후 Artifact 기록 여부

#### K8s Deploy

- Cluster
- Namespace
- Workload Type
- Workload 이름
- Container 이름
- Image Version
- Deploy Policy: Rolling Deploy, Recreate Deploy
- rollout 완료 대기 여부

#### Manual Approval

- 확인 제목
- 확인 설명
- Approver
- Timeout Policy

#### Notification

- Notification Rule
- Notification 시점: Stage 시작, Stage 성공, Stage 실패, Pipeline 종료
- Template Variable Preview

## 6. Pipeline 실행

### 6.1 즉시 실행 대화상자

`즉시 실행`을 클릭하면 실행 Parameter 대화상자가 표시됩니다:

- Pipeline 이름
- 소속 Application
- 실행 Environment
- Branch/Tag
- Image Tag
- Custom Parameter
- Notification 활성화 여부
- Notification Rule

확인하면 Pipeline Run 기록이 한 건 생성됩니다.

### 6.2 실행 중 상세

실행 후 즉시 실행 상세 대화상자를 열거나 상세 Page로 이동합니다.

표시 항목:

- 현재 Run 상태
- Stage Progress Bar
- Stage별 상태: 대기 중, 실행 중, 성공, 실패, 건너뜀, 확인 대기
- 현재 Stage 실시간 Log
- 총 소요 시간

작업:

- 실행 중지
- 실패 Stage 재시도
- 전체 Log 보기
- Log Download

## 7. Pipeline 실행 History

목록 필드:

- 실행 번호
- Pipeline 이름
- 소속 Application
- Environment
- Branch/Tag
- Trigger User
- Trigger Type: 수동, Webhook, 스케줄, API
- 상태
- 시작 시각
- 소요 시간
- 작업

작업:

- 상세 보기
- Log 보기
- 재실행
- 이 Version으로 Rollback, 이후 확장

상세 필드:

- 기본 정보
- Stage Timeline
- Stage Log
- Artifact 정보
- Deploy 대상
- Notification 기록

## 8. Backend Model 권장

### 8.1 Pipeline 정의 Table

Table 이름: `ops_app_pipeline`

필드:

- `id`
- `name`
- `app_id`
- `app_name`
- `repo_type`
- `repo_url`
- `default_branch`
- `env`
- `tech_stack`
- `status`
- `template_id`
- `definition_json`
- `description`
- `created_at`
- `updated_at`

### 8.2 Pipeline Template Table

Table 이름: `ops_app_pipeline_template`

필드:

- `id`
- `name`
- `category`
- `tech_stack`
- `description`
- `stage_count`
- `definition_json`
- `builtin`
- `status`
- `created_at`
- `updated_at`

### 8.3 Pipeline 실행 History Table

Table 이름: `ops_app_pipeline_run`

필드:

- `id`
- `pipeline_id`
- `pipeline_name`
- `app_id`
- `app_name`
- `env`
- `branch`
- `image_tag`
- `trigger_type`
- `trigger_user`
- `status`
- `summary`
- `params_json`
- `definition_json`
- `started_at`
- `finished_at`
- `duration_ms`
- `created_at`

### 8.4 Pipeline Stage 실행 Table

Table 이름: `ops_app_pipeline_run_stage`

필드:

- `id`
- `run_id`
- `stage_id`
- `stage_name`
- `stage_type`
- `status`
- `summary`
- `log`
- `started_at`
- `finished_at`
- `duration_ms`
- `created_at`

## 9. API 권장

Pipeline:

- `GET /api/ops/app/pipeline/list`
- `GET /api/ops/app/pipeline/info`
- `POST /api/ops/app/pipeline/save`
- `POST /api/ops/app/pipeline/delete`
- `POST /api/ops/app/pipeline/status`
- `POST /api/ops/app/pipeline/copy`

Template:

- `GET /api/ops/app/pipeline/template/list`
- `GET /api/ops/app/pipeline/template/info`
- `POST /api/ops/app/pipeline/template/save`
- `POST /api/ops/app/pipeline/template/delete`

실행:

- `POST /api/ops/app/pipeline/run`
- `POST /api/ops/app/pipeline/run/cancel`
- `POST /api/ops/app/pipeline/run/retry-stage`
- `GET /api/ops/app/pipeline/run/list`
- `GET /api/ops/app/pipeline/run/info`
- `GET /api/ops/app/pipeline/run/stage-log`

## 10. 기존 Module과의 재사용 관계

재사용:

- Application Project: Application 이름, Repository Type, Repository Address, Default Branch.
- Build History Log 스타일: Pipeline Stage Log 대화상자에 사용.
- K8s 관리: Cluster, Namespace, Workload, Image Update 기능.
- Notification: Notification Rule, Notification Channel, Message Template.
- 표준 Ops Script Library: Shell Stage의 Script 출처로 사용 가능.

중복 방지:

- `Build Task`는 간단한 Build 진입점으로 계속 유지합니다.
- `CI/CD Pipeline`은 다중 Stage Orchestration과 Deploy Closed Loop을 지향합니다.

## 11. 권한과 보안

권한 Point:

- Pipeline 조회
- Pipeline 생성
- Pipeline 수정
- Pipeline 삭제
- Pipeline 실행
- Pipeline 중지
- Pipeline Template 관리
- Credential 사용

보안 요구사항:

- Repository Credential, Image Registry Credential, K8s Credential은 평문으로 Frontend에 반환할 수 없습니다.
- Log에서 token, password, secret, access key는 Masking해야 합니다.
- Production Environment 실행은 재확인이 필요하며 이후 Manual Approval을 연계할 수 있습니다.

## 12. 1차 단계 인도 범위

1차 단계에서는 다음을 우선 완료할 것을 권장합니다:

1. Application Center에 `CI/CD Pipeline` Menu를 추가합니다.
2. Pipeline 목록 Page.
3. Pipeline Template 선택 대화상자, Screenshot 스타일대로 구현.
4. 내장 Go, Java, Vue, 빈 Pipeline Template.
5. Pipeline 추가/수정 Page, 기본 정보와 Stage JSON 구성 지원.
6. Pipeline 즉시 실행, 실행 기록 생성.
7. 실행 History 목록과 Log 확인.

보류:

- Webhook 자동 Trigger.
- 실제 Docker build/push.
- 복잡한 DAG 병렬 Stage.
- Blue/Green, Canary Deploy.
- 자동 Rollback.

## 13. 검수 기준

- Application Center에서 `CI/CD Pipeline` Page를 볼 수 있습니다.
- `새 Pipeline`을 클릭하면 Screenshot과 동일한 Information Architecture가 표시됩니다: 좌측 Category, 우측 Template Card, 하단 취소/빈 Pipeline/Template 사용 Button.
- Template을 선택하면 대응하는 Stage가 생성됩니다.
- 빈 Pipeline은 Stage 없는 Pipeline을 생성할 수 있습니다.
- Pipeline 목록은 Query, 활성화, 비활성화, 복제, 삭제, 즉시 실행을 지원합니다.
- 실행 후 실행 History에서 기록을 확인할 수 있습니다.
- 실행 상세에서 Stage별 상태와 Log를 확인할 수 있습니다.
- Page 문구는 전부 한국어이며 깨진 글자가 없습니다.
- Frontend `npm run build` 통과, Backend `go test ./...` 통과.
