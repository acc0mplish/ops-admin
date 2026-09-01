# Ops Admin Backend

Ops Admin Backend는 Go, Gin, GORM, MySQL 기반으로 Web Console에 `/api/v1` REST API를 제공하며 자산 관리, Observability, Automation, AI Assistant, 클라우드 비용(FinOps) 기능을 담당합니다.

## 디렉터리 및 역할

| 디렉터리 | 역할 |
| --- | --- |
| `router/` | Route 등록, 인증 Middleware, API Group 구성 |
| `controller/` | Request Parameter 검증 및 HTTP Response 처리 |
| `service/` | 비즈니스 오케스트레이션, Domain Rule, 외부 시스템 연동 |
| `model/` | GORM Entity 및 데이터 구조 |
| `store/` | MySQL 초기화, Migration, Seed Data |
| `config/` | 설정 로딩 |

진입점은 `main.go`입니다. `config.yaml`을 로드하고 MySQL에 연결한 뒤 Migration과 Seed Data를 적용하고 Gin Route를 생성합니다.

## 실행 환경

- Go 1.24+
- MySQL 8.0+ 또는 호환 MySQL 인스턴스
- Frontend 개발 시 Node.js 18+

## 설정

환경에 맞게 `config.yaml`을 수정하십시오. 최소한 애플리케이션 Port와 MySQL 연결 정보가 필요합니다.

```yaml
app:
  name: ops-admin
  port: "8082"
  mode: debug

db:
  host: 127.0.0.1
  port: "3306"
  user: your_user
  password: your_password
  name: ops_admin
  log-mode: false
```

`config.yaml`에는 로컬 개발 Credential이 포함될 수 있습니다. 실제 운영 Password, Cloud Access Key, Model API Key를 저장소에 Commit하지 마십시오. 운영 환경에서는 통제된 방식으로 설정을 주입하고 최소 권한 DB 계정을 사용해야 합니다.

운영 환경에서는 `OPS_ADMIN_JWT_SECRET` 환경 변수로 충분히 긴 Random JWT Signing Key를 주입해야 합니다. 로그인 Session은 60분 Access Token, 연속 6시간 비활성 Timeout, 최대 7일 Lifetime을 사용하며 Frontend는 Access Token 만료 5분 전에 Silent Refresh를 수행합니다.

도메인 관리는 `config.yaml`의 `security.credential-key`를 사용해 Public DNS Credential과 SSL Private Key를 AES-GCM으로 암호화합니다. 운영 환경에서는 최소 32byte의 안정적이고 독립적인 Random Key를 사용하십시오. Private DNS 기능은 기본적으로 비활성화되어 있습니다. Linux에서 Port 53을 Listen해야 할 경우 플랫폼 전체를 root로 실행하지 말고 Binary에 최소 Capability만 부여하는 방식을 권장합니다.

```bash
setcap 'cap_net_bind_service=+ep' /path/to/ops-admin
```

systemd에서는 다음과 같이 설정할 수 있습니다.

```ini
AmbientCapabilities=CAP_NET_BIND_SERVICE
CapabilityBoundingSet=CAP_NET_BIND_SERVICE
```

## 로컬 개발

```powershell
cd backend
go mod download
go run .
```

기본 Listen 주소는 `http://127.0.0.1:8082`입니다. 시작 시 Database Migration과 필요한 Seed Data 초기화를 자동으로 수행합니다.

검증:

```powershell
cd backend
go test ./...
```

## 주요 모듈

### 자산 및 Observability

- Host, Database, Kubernetes, Credential, Gateway 자산은 CMDB 모델에서 통합 관리합니다.
- Host CPU, Memory, Disk 사용률은 설정된 Prometheus 또는 VictoriaMetrics에서 node_exporter Metric을 조회합니다. 일치하는 Monitoring Target이 없으면 임의 값을 생성하지 않고 명확하게 사용 불가 상태를 반환합니다.
- Monitoring, Log, Alert 데이터 접근은 Service Layer에서 통합 처리하며 Frontend가 Data Source에 직접 접근하지 않습니다.

### AI Assistant

- Model, Conversation, Tool 설정 및 Tool 실행 이력은 로컬 Database에 저장합니다.
- Read-only Tool은 자동 실행할 수 있지만 외부 상태를 변경하는 Tool은 반드시 Confirm Pending 상태를 거쳐 사용자 승인 후 실행합니다.
- OpenAI Native Tool Calling과 일부 Model이 반환하는 DSML Tool Marker를 지원합니다. 무한 Loop를 방지하기 위해 한 Conversation에서 최대 3회의 Tool Calling Round를 허용합니다.

### 클라우드 비용 FinOps

FinOps의 데이터 경계는 명확하게 유지해야 합니다.

1. **Cloud Account 연결 테스트와 Billing Sync**만 Cloud Provider Billing API를 호출할 수 있습니다.
2. 동기화는 Calendar Month 단위로 수행합니다. 범위를 지정하지 않으면 현재 월을 포함한 최근 6개월을 대상으로 하며, 한 달의 동기화 실패가 다른 월의 처리를 중단시키지 않습니다.
3. 동일 Account의 동일 Billing Record는 idempotent upsert로 갱신하며 중복 합산하지 않습니다.
4. Cost Dashboard, Cost Breakdown, Resource Breakdown, Optimization Recommendation, AI Cloud Cost Tool은 모두 로컬에 동기화된 Billing Data만 조회하며 Cloud Provider에서 실시간으로 데이터를 가져오지 않습니다.
5. 현재 월 Billing은 불완전할 수 있습니다. Alibaba Cloud 월간 Instance Billing의 일 단위 표시는 로컬 일평균 배분 추정치이며 UI와 API에서 해당 산정 기준을 명시해야 합니다.

주요 API는 `/api/v1/integration/finops/*`에 있습니다.

| 분류 | 설명 |
| --- | --- |
| `account/*` | Cloud Account 관리 및 연결 테스트 |
| `sync/trigger`, `sync/logs` | 월 단위 Billing Sync 및 Sync History |
| `dashboard`, `breakdown`, `resource/list` | 로컬 Billing Data Aggregation Query |
| `recommendation/*` | 기본 또는 AI 전략 기반 Recommendation 생성, 조회, 수정, 삭제 |
| `cost/import` | 표준 JSON Billing Import |

AI Tool `finops_cost_analysis`는 동기화된 Cost Record만 읽습니다. Account, Month, Service, Region, Resource 기준 Aggregation은 가능하지만 Cloud Provider의 실시간 Billing 조회 용도로 사용해서는 안 됩니다.

## 변경 원칙

- Controller는 Validation과 Protocol Conversion을 담당하고 비즈니스 Rule은 `service/`에 둡니다. Cloud Provider 호출을 Controller나 AI Tool에 분산시키지 마십시오.
- Billing Amount 관련 변경은 Record Count, Amount, Billing Period, Currency, 현재 월 데이터의 불완전 여부를 함께 검증해야 합니다.
- AI Tool 추가 시 Permission, Parameter Schema, Confirm 필요 여부, Data Source를 명확히 선언해야 합니다. Cloud Cost 관련 Tool은 기본적으로 로컬 Database 조회만 허용합니다.

## 관련 문서

- [아키텍처 문서 인덱스](../docs/architecture/README.md)
- [FinOps 및 AI 데이터 흐름](../docs/architecture/finops-ai-data-flow.md)
- [FinOps 최적화 방안](../docs/FINOPS_OPTIMIZATION_PLAN.md)
