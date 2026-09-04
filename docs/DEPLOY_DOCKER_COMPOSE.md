# Ops Admin Docker Compose Deploy 매뉴얼

본 문서는 Ubuntu 서버의 단일 장비 Deploy를 대상으로 합니다. Compose는 독립적인 MySQL, Ops Admin API, Web Container를 시작하며 개인 운영 플랫폼, 중소 팀, 기능 검증 Environment에 적합합니다.

## 1. Deploy 토폴로지

```text
브라우저
  │ HTTP/HTTPS
  ▼
Host 8080
  │
  ▼
ops-admin-web (Nginx)
  ├── /             → Vue 프론트엔드
  ├── /api/v1/      → ops-admin-api:8082
  └── /uploads/     → ops-admin-api:8082
                         │
                         ▼
                   ops-admin-mysql:3306

내부망 DNS 클라이언트 (docker-compose.dns.yml 로드 시에만)
  │ UDP/TCP 53
  ▼
Host 내부망 주소:53 → ops-admin-api:53
```

| 서비스 | Container 이름 | 외부 포트 | 데이터 영속화 |
| --- | --- | --- | --- |
| Web 콘솔 | `ops-admin-web` | `8080` | 없음 |
| API | `ops-admin-api` | HTTP는 Compose 내부망 `8082`에 한함; DNS 오버라이드 파일 로드 시 `53/UDP`, `53/TCP` 노출 | `ops-admin-uploads` |
| MySQL 8 | `ops-admin-mysql` | Compose 내부망에 한해 `3306` | `ops-admin-mysql-data` |

MySQL은 Host 포트를 매핑하지 않으므로 Host의 기존 `3306`을 점유하지 않습니다. API도 직접 노출하지 않고 브라우저는 Web Container를 경유해 접근합니다.

## 2. 전제 조건

### 2.1 서버 권장 사양

- Ubuntu 22.04 또는 24.04
- 최소 2 vCPU, 4 GB 메모리, 20 GB 가용 디스크
- Production Environment에서는 4 vCPU, 8 GB 메모리를 권장하며 Docker 데이터 디렉터리용 독립 디스크 공간을 확보하십시오
- 코드 Repository 및 Docker Image Registry 접근 가능
- 관리 대상 SSH Host, Kubernetes API, Database 및 모니터링 Datasource 접근 가능
- 방화벽 또는 Cloud 보안 그룹에서 TCP `8080` 접근 허용
- 내부망 DNS 사용 시 신뢰할 수 있는 내부망 클라이언트의 Host `53/UDP`, `53/TCP` 접근을 허용하고, API Container가 업스트림 DNS의 `53/UDP`, `53/TCP`에 접근하도록 허용

### 2.2 소프트웨어 확인

서버에 Git, Docker Engine, Docker Compose V2를 설치해야 합니다:

```bash
git --version
docker version
docker compose version
docker info
```

`8080`이 점유되지 않았는지 확인:

```bash
ss -lntp | grep ':8080' || true
```

내부망 DNS 활성화를 준비하는 경우 Host의 TCP, UDP `53`도 점유되지 않았는지 확인:

```bash
sudo ss -lntup '( sport = :53 )'
```

`systemd-resolved`, dnsmasq, BIND 또는 다른 DNS 리스너가 존재하면 Listen 주소를 조정하거나 충돌 서비스를 먼저 중지해야 합니다; 포트가 점유된 상태로는 Compose를 시작하지 마십시오.

## 3. 프로젝트 가져오기

```bash
git clone https://github.com/qishu321/ops-admin.git
cd ops-admin
```

Production Environment에서는 고정 tag 또는 commit을 Deploy하고 불확실한 개발 Branch를 장기간 따라가지 않기를 권장합니다.

## 4. Config 준비

템플릿을 복사합니다:

```bash
cp deploy/.env.example deploy/.env
cp deploy/config.yaml.example deploy/config.yaml
chmod 600 deploy/.env deploy/config.yaml
```

다음 명령으로 무작위 값을 각각 생성하고, 생성 결과는 설정 항목마다 하나씩 사용하며 재사용하지 마십시오:

```bash
openssl rand -base64 36
```

### 4.1 `deploy/.env` 설정

최소한 다음 항목을 교체하십시오:

```dotenv
TZ=Asia/Shanghai
MYSQL_DATABASE=ops_admin
MYSQL_USER=ops_admin
MYSQL_PASSWORD=<Database 비즈니스 계정 비밀번호>
MYSQL_ROOT_PASSWORD=<비즈니스 계정과 다른 root 비밀번호>
OPS_ADMIN_JWT_SECRET=<32바이트 이상의 안정적인 무작위 값>
OPS_ADMIN_INITIAL_USERNAME=admin
OPS_ADMIN_INITIAL_PASSWORD=<최초 관리자 강력 비밀번호>
OPS_ADMIN_CORS_ORIGINS=
OPS_ADMIN_DNS_BIND_ADDRESS=<Host 내부망 IP>
```

Config 설명:

- `MYSQL_PASSWORD`: Ops Admin이 MySQL 접속에 사용하는 비밀번호.
- `MYSQL_ROOT_PASSWORD`: MySQL root 비밀번호로, 비즈니스 계정 비밀번호와 달라야 합니다.
- `OPS_ADMIN_JWT_SECRET`: Login Token 서명 Key. Production Environment에서는 반드시 설정하고 안정적으로 보관해야 합니다; 변경 시 기존의 모든 Login Session이 무효화됩니다.
- `OPS_ADMIN_INITIAL_PASSWORD`: Database에 아직 관리자가 없을 때 초기화에만 사용되며, 이후 이 Environment Variable을 변경해도 기존 계정 비밀번호는 재설정되지 않습니다.
- `OPS_ADMIN_CORS_ORIGINS`: 동일 Origin Deploy에서는 비워 둡니다; API가 다른 Browser Origin에서 직접 접근될 때만 쉼표로 구분된 전체 Origin을 입력합니다.
- `OPS_ADMIN_DNS_BIND_ADDRESS`: 내부망 DNS가 Host에 노출하는 주소. Production Environment에서는 Host 내부망 IP를 입력하고 공인 IP 직접 사용은 권장하지 않습니다.

### 4.2 `deploy/config.yaml` 설정

Database 비밀번호를 `MYSQL_PASSWORD`와 완전히 동일하게 변경하고 Credential 암호화 Key를 교체합니다:

```yaml
app:
  name: ops-admin
  port: "8082"
  mode: release

db:
  host: mysql
  port: "3306"
  user: ops_admin
  password: "<MYSQL_PASSWORD와 완전히 동일>"
  name: ops_admin
  log-mode: false

security:
  credential-key: "<32바이트 이상의 안정적이고 독립적인 무작위 값>"
```

`security.credential-key`는 플랫폼에 저장된 Cloud Credential, Certificate 개인 Key 등 민감 정보를 암호화하는 데 사용합니다. 사용을 시작한 뒤에는 임의로 교체할 수 없으며 교체하면 기존 암호문을 복호화할 수 없습니다. `deploy/.env`와 `deploy/config.yaml`를 통제된 백업에 포함하되 Git에 커밋하지 마십시오.

## 5. 서비스 시작

Compose Config를 먼저 확인한 뒤 Build하고 시작합니다:

```bash
docker compose --env-file deploy/.env config --quiet
docker compose --env-file deploy/.env up -d --build
docker compose ps
```

첫 Build 시 Go, Node.js, Nginx, MySQL 등 Base Image와 의존성을 내려받으며 소요 시간은 네트워크 환경에 따라 달라집니다. API 시작 시 Database Migration과 기본 데이터 초기화가 자동 실행됩니다.

시작 Log 확인:

```bash
docker compose logs -f mysql
docker compose logs -f api
docker compose logs -f web
```

`Ctrl+C`로 Log 확인을 종료해도 Container는 중지되지 않습니다.

## 6. Deploy 검증

### 6.1 서비스 상태

```bash
docker compose ps
curl -fsS http://127.0.0.1:8080/api/v1/systemConfig/public
```

세 Container가 `running` 또는 `healthy`여야 합니다. Browser 접근:

```text
http://<서버 IP>:8080
```

`deploy/.env`에 설정한 `OPS_ADMIN_INITIAL_USERNAME`과 `OPS_ADMIN_INITIAL_PASSWORD`로 Login하고 첫 Login 직후 관리자 비밀번호를 변경하십시오.

### 6.2 Page 검증

Login 후 Application Platform Navigation이 열리고 자산 관리, Container 관리, 표준 운영, Application Center, Notification, Integration Center, Monitor Center, Domain 관리 등 각 화면으로 전환할 수 있어야 합니다.

![Ops Admin Application Platform Navigation](./screenshots/01-platform-navigation.png)

"자산 관리 → Asset Overview"로 이동해 Page, 메뉴, API Request가 모두 정상 로딩되는지 확인합니다:

![Ops Admin Asset Overview](./screenshots/02-asset-overview.png)

"Container 관리 → K8s 관리 → Cluster Overview"로 이동해 등록된 Cluster를 하나 선택하고 Cluster 기본 정보, 리소스 사용률, 네트워크 설정, Certificate 정보가 로딩되는지 확인합니다:

![Ops Admin Kubernetes Cluster Overview](./screenshots/03-kubernetes-cluster-overview.png)

"표준 운영 → Job Center → Job Orchestration"으로 이동해 Step Library, Orchestration Canvas, Step 설정 영역이 정상 표시되는지 확인합니다:

![Ops Admin Job Orchestration](./screenshots/04-job-orchestration.png)

"Application Center → Build & Deploy → Build History"로 이동해 Filter 영역, Build 상태, 현재 Stage, 소요 시간, Detail 진입점이 로딩되는지 확인합니다:

![Ops Admin Build History](./screenshots/05-build-history.png)

"Notification → Message Template"으로 이동해 Template 목록, Channel 유형, 적용 시나리오, 상태 Filter가 로딩되는지 확인합니다:

![Ops Admin Message Template](./screenshots/06-message-templates.png)

"Integration Center → Navigation 관리"로 이동해 Navigation 그룹, 공개 접근 상태, 시스템 진입 카드가 로딩되는지 확인합니다:

![Ops Admin Integration Navigation](./screenshots/07-integration-navigation.png)

"Monitor Center → Alert 관리 → Alert Template"으로 이동해 Template 그룹, Datasource, Severity, Rule 생성 진입점이 로딩되는지 확인합니다:

![Ops Admin Alert Template](./screenshots/08-alert-templates.png)

"Domain 관리 → 내부망 Domain → Zone 관리"로 이동해 DNS 서비스 상태, Listen 주소, Zone 목록, Record 진입점이 로딩되는지 확인합니다:

![Ops Admin 내부망 DNS Zone](./screenshots/09-private-dns-zones.png)

다음 최소 검증 항목을 이어서 완료하시기를 권장합니다:

- 테스트 Host를 새로 만들고 연결 검사를 실행합니다.
- 테스트 Kubernetes Cluster를 구성하고 kubeconfig 연결을 검증합니다.
- Command 실행, Job Orchestration, Message Template, 모니터링 Template Page를 엽니다.
- 작은 파일을 Upload하여 `ops-admin-uploads` Volume에 정상 기록되는지 확인합니다.
- Browser 콘솔과 `docker compose logs api`에 지속 오류가 없는지 확인합니다.

## 7. 네트워크 및 선택 기능

### 7.1 대상 리소스 연결성

플랫폼이 SSH, Kubernetes, Database, 모니터링 Query를 시작하는 소스는 `ops-admin-api` Container입니다. 대상 네트워크, 방화벽, 화이트리스트는 해당 Container가 Host 네트워크를 통해 대상 주소에 접근하도록 허용해야 합니다.

대상이 격리 네트워크에 있다면 플랫폼의 "자산 관리 → Gateway 관리"에서 SSH Jump Gateway를 구성하고 해당 Host, Database, Kubernetes Cluster에서 Gateway 접근을 선택하십시오.

### 7.2 HTTPS

표준 Compose는 Host `8080`만 Listen합니다. Production Environment에서는 앞단에 기존 Nginx, HAProxy, Traefik 또는 Cloud Load Balancer를 구성하고 다음을 완료하시기를 권장합니다:

- TLS Certificate Termination
- HTTP → HTTPS Redirect
- WebSocket Forwarding
- `/api/v1/` 및 `/uploads/`의 Request Body 및 Timeout 설정
- 관리 콘솔은 신뢰할 수 있는 네트워크 대역에서만 접근 허용

Reverse Proxy와 Ops Admin이 같은 서버에 있다면 `docker-compose.yml`의 Web 포트를 루프백 주소만 Listen하도록 변경할 수 있습니다:

```yaml
ports:
  - "127.0.0.1:8080:80"
```

### 7.3 내부망 DNS 활성화

기본 `docker-compose.yml`은 Host `53` 포트를 점유하지 않습니다; 내부망 DNS를 활성화하지 않는다면 앞선 명령으로 Deploy하면 됩니다. 활성화할 때는 `docker-compose.dns.yml`을 추가로 겹쳐 사용합니다. 이 파일은 API Container의 `53/UDP`, `53/TCP`를 Host에 매핑하고 비 root API 프로세스에 `NET_BIND_SERVICE` Capability만 추가합니다. DNS는 보통 UDP를 먼저 사용하지만 큰 Response 또는 UDP 절단 후 재시도에서는 TCP를 사용하므로 두 프로토콜을 모두 허용해야 합니다.

1. `deploy/.env`에서 DNS 노출 주소를 Host의 내부망 IP로 설정해 모든 공인 NIC의 직접 Listen을 피합니다:

   ```dotenv
   OPS_ADMIN_DNS_BIND_ADDRESS=192.168.10.20
   ```

   별도 내부망 주소가 없을 때는 `0.0.0.0`을 사용할 수 있지만 Cloud 보안 그룹, 경계 방화벽 또는 Host 규칙으로 소스를 신뢰할 수 있는 네트워크 대역으로 제한해야 합니다. Docker 포트 노출은 iptables/nftables 포워딩 규칙을 수정하므로 UFW를 유일한 접근 경계로 삼아서는 안 됩니다.

2. DNS 오버라이드 파일로 API Container를 다시 생성해 포트 매핑과 Capability를 적용합니다. 이후 이 DNS 활성화 Deploy에 대해 `up`, `ps`, `logs`, `down` 등의 명령을 실행할 때도 동일한 두 `-f` 파라미터를 함께 사용해야 합니다:

   ```bash
   docker compose \
     -f docker-compose.yml \
     -f docker-compose.dns.yml \
     --env-file deploy/.env \
     up -d --build

   docker compose \
     -f docker-compose.yml \
     -f docker-compose.dns.yml \
     --env-file deploy/.env \
     ps
   ```

3. 클라이언트에서 서버로의 인바운드 DNS 트래픽을 허용합니다. 신뢰할 수 있는 네트워크 대역 `192.168.10.0/24` 예시:

   ```bash
   sudo ufw allow from 192.168.10.0/24 to any port 53 proto udp
   sudo ufw allow from 192.168.10.0/24 to any port 53 proto tcp
   ```

   Cloud 서버는 보안 그룹에 동일한 인바운드 규칙 두 개도 추가해야 합니다. `0.0.0.0/0`에 Recursive DNS를 개방하면 DNS Amplification 공격 진입로가 될 수 있으므로 개방하지 마십시오.

4. 서버와 Docker 네트워크가 API Container가 설정한 업스트림 DNS(예: `223.5.5.5:53`)에 접근하도록 보장합니다. 네트워크에 Outbound ACL이 있다면 업스트림 DNS 주소의 `UDP/53`과 `TCP/53`을 명시적으로 허용합니다.

5. Ops Admin에 Login하여 "Domain 관리 → 내부망 Domain → DNS 설정"으로 이동해 다음을 입력합니다:

   - 상태: 활성화
   - Listen 주소: `0.0.0.0` (Container 내부 Listen 주소이며 Host IP를 입력하지 마십시오)
   - Listen 포트: `53`
   - 업스트림 DNS: 사내 DNS 또는 접근을 허용할 공용 DNS를 입력

   저장 후 UDP, TCP 모두 Running으로 표시되어야 합니다. 그런 다음 Zone과 Record를 생성하고 활성화합니다.

6. Host와 다른 내부망 클라이언트에서 각각 UDP, TCP를 검증합니다:

   ```bash
   # 192.168.10.20과 ops.com을 실제 주소와 Zone으로 교체
   dig @192.168.10.20 ops.com A
   dig @192.168.10.20 ops.com A +tcp

   docker compose -f docker-compose.yml -f docker-compose.dns.yml port api 53/udp
   docker compose -f docker-compose.yml -f docker-compose.dns.yml port api 53/tcp
   ```

저장 후 Page에 시작 실패가 표시되면 `docker compose logs api`, Host `53` 포트 충돌, Container의 `NET_BIND_SERVICE` Capability, 업스트림 DNS Outbound 정책을 우선 확인하십시오.

### 7.4 Application Center에서 Build 실행

Build Host에는 Go나 Node.js를 설치할 필요가 없습니다. `backend/Dockerfile`은 Go Build 단계에서 Linux 바이너리를 생성하고 `web/Dockerfile`은 Node.js Build 단계에서 프론트엔드 산출물을 생성합니다; Build Host에는 Git, Docker Engine, Docker Compose V2만 있으면 됩니다.

"Application Center → Build Task"에서 자산 Host를 선택한 뒤 실행 경로에 SSH 사용자가 쓸 수 있는 절대 경로(예: `/home/ops/ops-admin`)를 입력합니다. Docker Compose Build Template 사용 시 첫 실행에서 코드 작업 디렉터리에 Git 무시 대상인 `deploy/.env`와 `deploy/config.yaml`이 생성되며 이후 Build는 Config와 기존 데이터 Volume을 재사용합니다. 이 두 파일을 삭제하지 말고 포함된 Key를 Build Log에 기록하지 마십시오.

## 8. 자주 쓰는 운영 Command

```bash
# 상태 확인
docker compose ps

# 최근 200행 Log 확인
docker compose logs --tail=200 api
docker compose logs --tail=200 web
docker compose logs --tail=200 mysql

# 전체 서비스 Log 추적
docker compose logs -f

# 개별 서비스 재시작
docker compose restart api

# 데이터를 유지한 채 서비스 중지
docker compose down

# 다시 시작
docker compose --env-file deploy/.env up -d
```

`docker compose down -v`를 실행하지 마십시오. 이 명령은 Database와 Upload 파일 Volume을 삭제합니다.

## 9. 백업과 복구

### 9.1 Database 백업

```bash
mkdir -p backup
docker compose exec -T mysql sh -c \
  'exec mysqldump --single-transaction --routines --triggers -u"$MYSQL_USER" -p"$MYSQL_PASSWORD" "$MYSQL_DATABASE"' \
  > "backup/ops-admin-$(date +%F-%H%M%S).sql"
```

백업이 빈 파일이 아닌지 확인:

```bash
ls -lh backup/*.sql
```

### 9.2 Upload 파일 백업

```bash
docker run --rm \
  -v ops-admin-uploads:/data:ro \
  -v "$PWD/backup:/backup" \
  alpine:3.21 \
  tar czf "/backup/ops-admin-uploads-$(date +%F-%H%M%S).tar.gz" -C /data .
```

동시에 다음 파일을 안전하게 백업합니다:

- `deploy/.env`
- `deploy/config.yaml`
- Database SQL 백업
- Upload 파일 압축 파일
- 현재 Deploy에서 사용하는 Git tag 또는 commit ID

Database 복구 전 유지보수 시간대에 진입하고 대상 Database가 올바른지 확인해야 합니다:

```bash
docker compose exec -T mysql sh -c \
  'exec mysql -u"$MYSQL_USER" -p"$MYSQL_PASSWORD" "$MYSQL_DATABASE"' \
  < backup/<백업 파일>.sql
```

## 10. 업그레이드

업그레이드 전 Database, Upload 파일, Config 백업을 먼저 완료한 뒤 다음을 실행합니다:

```bash
git pull --ff-only
docker compose --env-file deploy/.env build --pull
docker compose --env-file deploy/.env up -d
docker compose ps
docker compose logs --tail=200 api
```

업그레이드 후 "Deploy 검증"을 반복합니다. Backend는 Database Migration을 자동 실행하지만 Database 구조 변경이 반드시 이전 Image로의 역방향 Rollback을 지원하는 것은 아닙니다; Rollback이 필요하면 업그레이드 전 코드 Version과 Database 백업을 함께 사용해야 합니다.

## 11. 문제 해결

### MySQL이 계속 Healthy하지 않은 경우

```bash
docker compose logs --tail=200 mysql
```

디스크 공간, 데이터 Volume 권한과 `MYSQL_PASSWORD`, `MYSQL_ROOT_PASSWORD` 교체 여부를 확인합니다. 기존 데이터 Volume은 `.env` 수정만으로 Database 내부 비밀번호가 자동 변경되지 않습니다.

### API 시작 실패

```bash
docker compose logs --tail=200 api
```

핵심 확인 항목:

- `deploy/config.yaml`의 Database 비밀번호가 `MYSQL_PASSWORD`와 일치하는지.
- `security.credential-key`가 32바이트 이상인지.
- `OPS_ADMIN_JWT_SECRET`이 존재하고 길이가 충분한지.
- MySQL이 이미 Healthy한지.

### Page는 열리지만 API가 실패하는 경우

```bash
curl -v http://127.0.0.1:8080/api/v1/systemConfig/public
docker compose logs --tail=200 web
docker compose logs --tail=200 api
```

Reverse Proxy가 `/api/v1/` 경로를 유지하는지, Browser 접근 Domain이 CORS Config와 일치하는지 확인합니다.

### SSH, Kubernetes 또는 모니터링 연결 실패

API Container 내에서 DNS와 대상 포트를 확인합니다:

```bash
docker compose exec api sh
```

대상 주소가 Deploy 서버에서 접근 가능한지 확인하고 자산 Credential, Gateway 경로, Kubernetes API Server 주소, 모니터링 Datasource 인증 정보를 점검합니다.

### `8080` 포트 점유

`docker-compose.yml`에서 Web 서비스의 Host 포트를 변경합니다. 예시:

```yaml
ports:
  - "18080:80"
```

이후 `http://<서버 IP>:18080`에 접근합니다.

## 12. 보안 점검 체크리스트

- MySQL, 관리자, JWT, Credential 암호화 Key를 교체했습니다.
- `deploy/.env`, `deploy/config.yaml` 권한이 `600`이고 Git에 커밋하지 않았습니다.
- 첫 Login 후 관리자 비밀번호를 변경했습니다.
- 관리 포트는 신뢰할 수 있는 네트워크 대역에만 개방했습니다.
- Production Environment는 HTTPS로 접근합니다.
- Database와 Upload 파일 백업이 사용 가능한지 검증했습니다.
- Deploy Version, Config 변경, 업그레이드 시각을 기록했습니다.
- MySQL `3306`과 API `8082`를 공인망에 노출하지 않았습니다.
- 내부망 DNS 활성화 시 `53/UDP`, `53/TCP`는 신뢰할 수 있는 네트워크 대역에만 개방하고 업스트림 DNS로의 듀얼 프로토콜 Outbound 연결성을 검증했습니다.

## 13. 현재 제한 사항

- 본 Compose는 단일 장비 Deploy이며 MySQL 또는 Web/API의 고가용성을 제공하지 않습니다.
- 현재 개봉 즉시 사용 가능한 오프라인 설치 패키지가 없습니다; 완전 오프라인 환경은 Build Image와 의존성 캐시를 사전에 준비해야 합니다.
- 기본 Compose는 DNS 포트를 노출하지 않습니다; `docker-compose.dns.yml` 로드 시에만 `53/UDP`와 `53/TCP`를 노출합니다. 활성화 전에 Host 포트 충돌을 반드시 처리하고 접근을 허용할 소스 네트워크 대역을 제한해야 합니다.
- Production Environment의 외부 HTTPS, 백업 스케줄, Log 수집은 기존 인프라에 연동해야 합니다.
