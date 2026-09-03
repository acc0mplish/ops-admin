# Domain 관리 Module

## Database Table

| Table | 용도 |
|---|---|
| `domain_public_dns_account` | Public DNS Account. AccessKey와 Secret은 AES-GCM Ciphertext로 저장합니다. |
| `domain_public_snapshot` | Cloud Provider API에서 동기화한 Domain 목록 Snapshot |
| `domain_internal_dns_setting` | Internal DNS 활성 상태, Listen Address, Upstream, Timeout 설정 |
| `domain_internal_dns_zone` | Internal Authoritative Zone |
| `domain_internal_dns_record` | A 및 CNAME Record |
| `domain_dns_audit_log` | DNS 전용 변경 Audit |
| `ssl_certificates` | SSL Certificate Metadata, 암호화한 Private Key, Cloud Sync 상태 |
| `ssl_certificate_domains` | Certificate CN, SAN, Wildcard Domain 관계 |
| `ssl_certificate_versions` | ACME Renewal 전 Historical Certificate Version |
| `ssl_certificate_tasks` | APPLY, RENEW, SYNC, DELETE Async Task |
| `ssl_certificate_audit_logs` | Certificate 전용 작업 및 Sensitive Download Audit |

## Backend 구조

- `internal/domain/provider`: Public DNS Provider Interface, Aliyun DNS, Tencent Cloud DNSPod 구현
- `internal/domain/provider/*certificate.go`: Aliyun CAS와 Tencent Cloud SSL을 통합한 Certificate Provider
- `internal/domain/dnsserver`: Immutable Record Snapshot, UDP/TCP Server, Forwarding, TTL Cache
- `service/domain.go`: Account, Zone, Record, Batch Operation, Resolution Test, Audit Business Logic
- `service/ssl_certificate.go`: Sync, Upload, Download, Delete Security Rule, Certificate Audit
- `service/ssl_acme.go`: lego ACME DNS-01, Task Recovery, Automatic Renewal
- `controller/domain.go`: `/api/v1/domain/*` HTTP API

DNS Query는 `atomic.Pointer`가 가리키는 Memory Snapshot만 읽습니다. Zone 또는 Record가 변경되면 Database Transaction 안에서 새 Snapshot을 생성하고 Commit 성공 후 Atomic Replacement를 수행합니다. Query 과정에서는 MySQL에 접근하지 않습니다.

활성화된 Internal Zone에 속하지만 일치하는 Record가 없는 이름에는 NXDOMAIN을 반환합니다. 어떤 Internal Zone에도 속하지 않는 Query만 Upstream DNS에 순서대로 Forwarding합니다. UDP Response가 Truncated 상태이면 자동으로 TCP를 사용합니다.

## SSL Certificate

SSL Certificate는 Public DNS Account와 Provider를 재사용합니다. ACME 신청은 동기화되어 있고 활성 DNS Account와 연결된 Public Main Domain만 선택할 수 있습니다. DNS-01 과정에서 TXT Challenge Record를 자동 생성하고 정리합니다. Certificate Issuance와 Cloud Upload는 독립된 상태로 관리하므로 Cloud Upload 실패가 Local Valid Certificate를 덮어쓰지 않습니다.

Private Key는 프로젝트 공통 AES-256-GCM Secret Utility로 암호화합니다. 일반 List 및 Detail API는 Certificate Body 또는 Private Key를 반환하지 않습니다. Private Key와 ZIP Download에는 별도 Permission을 적용하고 전용 Audit를 기록합니다. Global Operation Log는 SSL Upload Body 전체를 기록 대상에서 제외합니다.

설정은 `backend/config.yaml`의 `ssl` Section에 둡니다.

```yaml
ssl:
  acme-email: "ops@example.com"
  production-ca: "https://acme-v02.api.letsencrypt.org/directory"
  staging-ca: "https://acme-staging-v02.api.letsencrypt.org/directory"
  dns-polling-seconds: 2
  dns-propagation-seconds: 120
  expiry-warning-days: 30
```

같은 설정은 `OPS_ADMIN_ACME_EMAIL`, `OPS_ADMIN_ACME_CA_PRODUCTION`, `OPS_ADMIN_ACME_CA_STAGING`, `OPS_ADMIN_ACME_DNS_POLLING_SECONDS`, `OPS_ADMIN_ACME_DNS_TIMEOUT_SECONDS`, `OPS_ADMIN_SSL_EXPIRY_WARNING_DAYS` Environment Variable로 Override할 수 있습니다.

## Deployment

Internal DNS는 기본적으로 비활성 상태입니다. Production 환경에서는 `backend/config.yaml`, Container Deployment에서는 `deploy/config.yaml`에 다음 값을 설정해야 합니다.

```yaml
security:
  credential-key: <32 byte 이상의 독립적인 random key>
```

Linux에서 Port 53을 Listen하려면 `CAP_NET_BIND_SERVICE`를 사용합니다. Web Platform 전체를 root 권한으로 실행하지 않습니다. DNS Module Start 실패는 Runtime Status와 Error Information만 갱신하며 HTTP Platform Process를 종료하지 않습니다.

Configuration File의 Read Permission을 제한해야 합니다. Encryption Key를 변경하면 기존 DNS Cloud Credential과 SSL Private Key를 복호화할 수 없습니다. ACME를 Production에서 활성화하기 전에 Staging에서 DNS-01 Validation을 완료합니다. Platform Process는 ACME CA, Authoritative DNS, Aliyun/Tencent Cloud Certificate API에 접근할 수 있어야 합니다.
