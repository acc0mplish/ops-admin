# Asset Gateway 관리 요구사항 문서

## 1. 배경

현재 Asset Management는 Server, Database, K8s Cluster 등 리소스를 커버하지만, 이들 리소스는 내부망, 전용선 네트워크, 사무망 DMZ 또는 Cloud VPC에 위치할 수 있어 Ops Admin Backend가 대상 주소에 직접 접근할 수 없습니다.

대표 시나리오:

- Server B는 점프 Gateway A를 통해서만 SSH로 접근할 수 있습니다.
- MySQL Database는 내부망 머신 A에만 개방되어 있으며, 플랫폼은 A를 통해 Forwarding한 뒤 Database에 접근해야 합니다.
- K8s API Server는 내부망 주소를 사용하며, 플랫폼은 Gateway A를 경유해야 Cluster API에 접근할 수 있습니다.

따라서 Asset Management에 "Gateway 관리"를 추가해 Host, Database, K8s Cluster가 직접 연결 또는 지정한 Gateway 경유 접근을 선택할 수 있도록 합니다.

## 2. 구축 목표

1단계 목표:

- Gateway 리소스의 CRUD, 활성화/비활성화, 연결 테스트를 추가합니다.
- 1-hop SSH Gateway를 지원합니다. 즉 플랫폼 Backend가 먼저 Gateway A에 연결한 뒤 Gateway A를 통해 대상 B에 접근합니다.
- Host 관리, Database 관리, K8s 관리 모두에서 Gateway를 구성할 수 있습니다.
- 플랫폼의 모든 실제 연결 동작은 Gateway 구성을 재사용해야 하며 화면 표시에만 그쳐서는 안 됩니다.
- Gateway 삭제 전 참조 관계를 확인해 Host, Database, K8s Cluster의 연결 구성이 무효화되지 않도록 합니다.

1단계에 포함하지 않는 항목:

- 다단계 Gateway 체인(예: A -> B -> C).
- SOCKS5, HTTP Proxy Gateway.
- Gateway 고가용성(HA) 자동 전환.
- Credential 암호화 저장 체계 재구축.

## 3. 메뉴와 페이지

Asset Management 아래에 하위 메뉴를 추가합니다:

- Server 관리
- Database 관리
- K8s 관리
- Gateway 관리

Gateway 관리 페이지 구성:

- Gateway 목록
- Gateway 추가
- Gateway 수정
- 상세 보기
- 연결 테스트
- 활성화 / 비활성화
- 삭제

목록 필드 제안:

| 필드 | 설명 |
| --- | --- |
| Gateway 이름 | 사용자가 식별할 수 있는 이름 |
| Gateway 주소 | SSH 주소(예: 192.168.1.10:22) |
| 인증 Credential | Asset Credential을 재사용하거나 Gateway Credential을 별도 선택 |
| 네트워크 Zone | 예: prod-vpc, office, idc-a |
| 상태 | 활성화, 비활성화 |
| 연결 상태 | 미검사, 정상, 실패 |
| 참조 수 | 사용하는 Host, Database, K8s Cluster 수 |
| 최근 검사 시각 | 마지막 테스트 시각 |
| 작업 | 테스트, 상세, 수정, 비활성화, 삭제 |

## 4. Gateway 데이터 모델

`asset_gateway` 테이블을 추가합니다.

제안 필드:

| 필드 | 타입 | 설명 |
| --- | --- | --- |
| id | uint | 기본 키 |
| name | string | Gateway 이름, 필수 |
| code | string | Gateway Code, 선택, 자동화 참조에 용이 |
| gateway_type | string | 1단계는 `ssh`로 고정 |
| host | string | Gateway IP/Domain, 필수 |
| port | int | SSH Port, 기본값 22 |
| credential_id | uint | Gateway 로그인 Credential, 필수 |
| network_zone | string | 네트워크 Zone |
| status | int | 1 활성화, 2 비활성화 |
| connect_status | int | 0 미검사, 1 정상, 2 실패 |
| last_check_time | datetime | 최근 검사 시각 |
| description | string | 비고 |
| created_at | datetime | 생성 시각 |
| updated_at | datetime | 수정 시각 |

1단계 Gateway Credential은 기존 `asset_credential`을 재사용해 비밀번호와 Key 관리를 중복 구현하지 않는 것을 권장합니다.

## 5. 연결 모드

Host, Database, K8s Cluster에 연결 모드를 추가합니다:

| Mode | 설명 |
| --- | --- |
| direct | 직접 연결(기존 로직 유지) |
| gateway | 지정한 Gateway로 접근 |

대응 필드 제안:

- `connection_mode`
- `gateway_id`

기존 데이터 호환을 위해:

- 기존 데이터는 기본값 `connection_mode = direct`
- `gateway_id = null`
- 화면에는 "직접 연결"로 표시합니다

## 6. Host 관리에 Gateway 접속

### 6.1 현황 점검

현재 Host SSH 연결 진입점은 다음에 집중되어 있습니다:

- `backend/service/service.go`의 `newSSHClient(host model.AssetHost)`
- `backend/service/ops.go`의 Command 실행, Script 실행, 파일 배포는 모두 간접적으로 SSH 연결을 호출합니다
- Host Terminal, Host 생존 확인, Host 정보 수집도 SSH 연결에 의존합니다

현재 로직:

```text
Ops Admin Backend -> 대상 Host SSH
```

Gateway 모드는 다음으로 변경합니다:

```text
Ops Admin Backend -> Gateway SSH -> 대상 Host SSH
```

### 6.2 Host 테이블 변경

`asset_host`에 추가:

- `connection_mode`
- `gateway_id`

Host 추가 / 수정 페이지에 추가:

- 연결 방식: 직접 연결 / Gateway 경유
- Gateway 선택기: 연결 방식이 "Gateway 경유"일 때만 필수

Host 목록에 추가:

- 연결 방식
- Gateway 이름

### 6.3 SSH 연결 로직

통합 메서드를 추가합니다:

```go
func (s *Service) newSSHClientForHost(host model.AssetHost) (*ssh.Client, error)
```

직접 연결:

```text
ssh.Dial("tcp", host.SSHIP:host.SSHPort, targetConfig)
```

Gateway 경유:

```text
1. ssh.Dial("tcp", gateway.host:gateway.port, gatewayConfig)
2. gatewayClient.Dial("tcp", host.SSHIP:host.SSHPort)
3. ssh.NewClientConn(targetConn, host.SSHIP:host.SSHPort, targetConfig)
4. ssh.NewClient(...)
```

모든 Host SSH 작업은 이 메서드를 통해 수행합니다:

- Host 연결 테스트
- Host Terminal 로그인
- Quick Execute: Command 실행
- Quick Execute: Script 실행
- Quick Execute: 파일 배포
- Schedule Task Script 실행
- Job 오케스트레이션의 Script 실행, 파일 배포

## 7. Database 관리에 Gateway 접속

### 7.1 현황 점검

현재 Database 연결 진입점은 주로 다음 위치입니다:

- `backend/service/database.go`
- `inspectMySQLDatabase(...)`
- `openDatabaseByID(...)`

현재 로직:

```text
sql.Open("mysql", user:pass@tcp(dbHost:dbPort)/schema)
```

즉 현재 Database는 직접 연결만 가능합니다.

### 7.2 Database 테이블 변경

`asset_database`에 추가:

- `connection_mode`
- `gateway_id`

Database 추가 / 수정 페이지에 추가:

- 연결 방식: 직접 연결 / Gateway 경유
- Gateway 선택기: Gateway 경유 시 필수

Database 목록에 추가:

- 연결 방식
- Gateway 이름

### 7.3 MySQL 연결 로직

1단계는 MySQL Driver의 사용자 정의 Dialer 사용을 권장합니다.

방안:

```text
1. database.gateway_id로 Gateway를 조회합니다
2. Gateway로의 SSH Client 연결을 수립합니다
3. gatewayClient.DialContext로 dbHost:dbPort에 연결합니다
4. mysql.RegisterDialContext로 Gateway 식별자가 포함된 network를 등록합니다
5. DSN은 해당 network로 MySQL에 접근합니다
```

유의 사항:

- 사용자 정의 network 이름에는 gatewayID와 databaseID를 포함해 서로 다른 Database 연결 풀이 섞이지 않도록 합니다.
- `sql.DB`를 닫을 때 Gateway SSH Client를 해제합니다.
- DBMS 조회, 테이블 데이터 편집, SQL 실행, Export Task, Import Task는 모두 동일한 연결 팩토리를 재사용해야 합니다.

접속이 필요한 기능:

- Database 연결 테스트
- SQL 편집기 실행
- 테이블 구조 조회
- 테이블 데이터 페이징 조회
- 셀 편집
- SQL 실행 이력
- Database Export
- 크로스 Database Import

## 8. K8s 관리에 Gateway 접속

### 8.1 현황 점검

K8s 일반 리소스 관리 진입점은 다음에 집중되어 있습니다:

- `backend/service/k8s.go`의 `k8sClientForCluster(...)`

단 Pod Terminal에는 별도 진입점이 있습니다:

- `backend/service/k8s_terminal.go`
- 현재는 `clientcmd.RESTConfigFromKubeConfig`를 직접 사용합니다

Application Center CI/CD의 K8s Deploy는 다음을 사용합니다:

- `backend/service/ops_application.go`
- 현재는 임시 kubeconfig로 `kubectl`을 호출합니다

### 8.2 K8s Cluster 테이블 변경

`k8s_cluster`에 추가:

- `connection_mode`
- `gateway_id`

K8s Cluster 등록 / 수정 페이지에 추가:

- 연결 방식: 직접 연결 / Gateway 경유
- Gateway 선택기: Gateway 경유 시 필수

Cluster 저장 검증 로직:

- 직접 연결: 현재 검증을 유지합니다.
- Gateway 경유: 먼저 Gateway를 테스트한 뒤 Gateway를 통해 API Server에 접근합니다.
- 실패 안내는 기존 문구를 유지합니다: `Cluster 연결에 실패했습니다. kubeconfig를 확인하십시오`
- 상세에서 실패 원인을 추가로 표시해 원인 파악을 돕습니다.

### 8.3 K8s API 접근 로직

일반 리소스 관리는 `k8sClientForCluster(...)`에서 Gateway 접속을 통합해야 합니다.

권장:

```text
1. kubeconfig를 파싱해 API Server host:port를 얻습니다
2. connection_mode = gateway인 경우:
   - SSH Gateway 연결을 수립합니다
   - http.Transport.DialContext에 gatewayClient.DialContext를 사용합니다
   - TLS ServerName은 kubeconfig의 원본 server 호스트명을 유지합니다
3. 모든 k8sGetJSON / k8sDo / YAML / 삭제 / 업데이트는 해당 http.Client를 재사용합니다
```

커버 범위:

- Cluster Overview
- Node 관리
- Namespace
- Pod 관리
- Workload
- Service
- Ingress
- 고급 네트워크
- Config·Storage
- YAML 조회 / 편집
- Pod Log
- Pod Terminal

### 8.4 Pod Terminal 변경

`k8s_terminal.go`는 현재 `k8sClientForCluster(...)`를 재사용하지 않으므로 별도 처리가 필요합니다.

추가 권장:

```go
func (s *Service) restConfigForCluster(cluster model.K8sCluster) (*rest.Config, func(), error)
```

직접 연결은 원본 rest.Config를 반환합니다.

Gateway 모드:

- `rest.Config.Transport` 또는 `rest.Config.Dial`에 Gateway Dialer를 주입합니다.
- 또는 Gateway DialContext가 포함된 `http.Transport`를 생성합니다.
- cleanup은 Gateway SSH Client를 닫는 것을 담당합니다.

### 8.5 CI/CD K8s Deploy 변경

현재 Application Center K8s Deploy Stage는 `kubectl --kubeconfig tempFile`을 사용하며, API Server가 Gateway로만 접근 가능한 경우 kubectl 자체는 플랫폼 Gateway를 자동으로 경유하지 않습니다.

선택 가능한 방안:

1. 권장: Backend가 Kubernetes API로 Image Update와 rollout status를 실행하도록 변경해 kubectl 의존성을 제거합니다.
2. 호환: 실행 전 로컬 임시 Port Forwarding을 생성해 kubeconfig server를 `https://127.0.0.1:localPort`로 재작성하고, kubectl은 로컬 터널로 Cluster에 접근합니다.

1단계는 방안 1을 권장하며, K8s 관리 모듈과 동일한 Gateway 접근 로직을 공유합니다.

## 9. Backend Service 설계

`backend/service/gateway.go`를 추가합니다.

핵심 기능:

```go
type GatewayDialer struct {
    Gateway model.AssetGateway
    Client  *ssh.Client
}

func (s *Service) GetAssetGateway(id uint) (*model.AssetGateway, error)
func (s *Service) ListAssetGateways(...) (map[string]any, error)
func (s *Service) CreateAssetGateway(payload AssetGatewayPayload) error
func (s *Service) UpdateAssetGateway(payload AssetGatewayPayload) error
func (s *Service) DeleteAssetGateway(id uint) error
func (s *Service) TestAssetGatewayConnection(id uint) error

func (s *Service) newGatewaySSHClient(gateway model.AssetGateway) (*ssh.Client, error)
func (s *Service) dialThroughGateway(ctx context.Context, gatewayID uint, network, address string) (net.Conn, func(), error)
func (s *Service) newGatewayHTTPTransport(gatewayID uint, base *http.Transport) (*http.Transport, func(), error)
```

Gateway 삭제 전 반드시 확인합니다:

- `asset_host.gateway_id`
- `asset_database.gateway_id`
- `k8s_cluster.gateway_id`

참조가 존재하면 삭제를 금지하고 다음을 반환합니다:

```text
해당 Gateway는 Asset에서 사용 중이므로 삭제할 수 없습니다
```

## 10. API 설계

신규 API:

| 메서드 | 경로 | 설명 |
| --- | --- | --- |
| GET | `/asset/gateway/list` | Gateway 목록 |
| GET | `/asset/gateway/options` | Gateway Dropdown 옵션 |
| GET | `/asset/gateway/info` | Gateway 상세 |
| POST | `/asset/gateway/create` | Gateway 추가 |
| POST | `/asset/gateway/update` | Gateway 업데이트 |
| POST | `/asset/gateway/delete` | Gateway 삭제 |
| POST | `/asset/gateway/status` | 활성화 / 비활성화 |
| POST | `/asset/gateway/test` | 연결 테스트 |

Host, Database, K8s Cluster의 기존 생성 / 업데이트 API에 추가:

```json
{
  "connectionMode": "direct",
  "gatewayId": 0
}
```

## 11. Frontend 연동 지점

변경 대상:

- `web/src/utils/apps.js`
  - Asset Management 앱 메뉴에 "Gateway 관리"를 추가합니다.
- `web/src/router/index.js`
  - `/assets/gateways`를 추가합니다.
- Host 관리 페이지
  - 연결 방식과 Gateway 선택기를 추가합니다.
  - 목록에 Gateway를 표시합니다.
- Database 관리 페이지
  - 연결 방식과 Gateway 선택기를 추가합니다.
  - 목록에 Gateway를 표시합니다.
- K8s Cluster 관리 페이지
  - 연결 방식과 Gateway 선택기를 추가합니다.
  - 저장 전 연결 방식에 따라 검증합니다.

신규 페이지 제안:

- `web/src/views/assets/Gateway.vue`

## 12. 권한과 감사

작업 감사 추가를 권장합니다:

- Gateway 생성
- Gateway 수정
- Gateway 삭제
- Gateway 테스트
- Host / Database / K8s Cluster에 Gateway 바인딩
- Host / Database / K8s Cluster의 Gateway 바인딩 해제

보안 요구:

- Gateway Credential은 API 응답에 평문으로 반환하지 않습니다.
- Gateway 테스트 실패 정보는 Frontend에 간략한 원인을 표시하고 상세 오류는 Backend Log에 기록합니다.
- Gateway 비활성화 후 해당 Gateway를 참조하는 Asset 연결은 즉시 실패하며 Gateway가 비활성화되었다고 안내합니다.

## 13. 검수 기준

### Gateway 관리

- SSH Gateway를 추가할 수 있습니다.
- Gateway를 수정, 비활성화, 활성화, 삭제할 수 있습니다.
- Gateway SSH 연결을 테스트할 수 있습니다.
- Asset에서 참조하는 Gateway는 삭제할 수 없습니다.

### Host 관리

- Host는 직접 연결 또는 Gateway 연결을 선택할 수 있습니다.
- Gateway 모드에서 Host 연결 테스트가 성공합니다.
- Gateway 모드에서 Host Terminal을 열 수 있습니다.
- Gateway 모드에서 Quick Command, Script 실행, 파일 배포를 사용할 수 있습니다.

### Database 관리

- Database는 직접 연결 또는 Gateway 연결을 선택할 수 있습니다.
- Gateway 모드에서 연결 테스트가 성공합니다.
- Gateway 모드에서 SQL 편집기로 Query를 실행할 수 있습니다.
- Gateway 모드에서 테이블 데이터 편집, Export, Import를 사용할 수 있습니다.

### K8s 관리

- K8s Cluster는 직접 연결 또는 Gateway 연결을 선택할 수 있습니다.
- Gateway 모드에서 Cluster 저장 전 kubeconfig를 반드시 검증합니다.
- Gateway 모드에서 Cluster Overview, Node, Namespace, Pod, Workload, Service, Ingress, Config·Storage를 정상적으로 읽을 수 있습니다.
- Gateway 모드에서 Pod Terminal, Log, YAML 조회와 편집을 사용할 수 있습니다.
- Application Center K8s Deploy Stage에서 Gateway를 통한 대상 Cluster 접근을 선택할 수 있습니다.

## 14. 개발 우선순위

P0:

- Gateway Model, Migration, CRUD, 테스트.
- Host SSH 연결에 Gateway 접속.
- Database 연결 테스트와 SQL 실행에 Gateway 접속.
- K8s `k8sClientForCluster`에 Gateway 접속.

P1:

- Pod Terminal에 Gateway 접속.
- DBMS Export, Import Task에 Gateway 접속.
- Quick Execute, Schedule Task, Job 오케스트레이션에 Gateway 접속.

P2:

- Application Center K8s Deploy Stage에서 kubectl을 완전히 제거하고 Kubernetes API로 전환.
- Gateway 연결 풀과 재사용 최적화.
- Gateway Health 점검과 참조 Asset 토폴로지.

## 15. 위험과 유의 사항

- MySQL 사용자 정의 Dialer는 전역 network 이름 충돌을 반드시 피해야 합니다.
- SSH Gateway 연결은 장기간 누수되어서는 안 되며 엄격한 cleanup이 필요합니다.
- K8s TLS 검증은 원본 API Server의 ServerName을 유지해야 하며, 로컬 터널 경유로 인증서 검증을 깨뜨려서는 안 됩니다.
- `kubectl`은 플랫폼 내부 Gateway를 자동 경유하지 않으므로 CI/CD K8s Deploy 체인을 변경해야 합니다.
- 일괄 Task를 Gateway로 실행할 때는 동시 실행을 제어해 Gateway 연결 수가 과도해지지 않도록 합니다.
- 추후 다단계 Gateway를 지원하면 현재 `gateway_id`는 `gateway_chain_id` 또는 Gateway 체인 테이블로 발전해야 합니다.
