# 域名管理模块

## 数据表

| 表 | 用途 |
|---|---|
| `domain_public_dns_account` | 公网 DNS 账号；AccessKey/Secret 均为 AES-GCM 密文 |
| `domain_public_snapshot` | 从云厂商 API 同步的域名列表快照 |
| `domain_internal_dns_setting` | 内网 DNS 启停、监听地址、上游与超时 |
| `domain_internal_dns_zone` | 内网权威 Zone |
| `domain_internal_dns_record` | A/CNAME 记录 |
| `domain_dns_audit_log` | DNS 专项变更审计 |
| `ssl_certificates` | SSL 证书元数据、加密后的 Private Key 与云端同步状态 |
| `ssl_certificate_domains` | 证书 CN / SAN / Wildcard 域名关系 |
| `ssl_certificate_versions` | ACME 续签前的历史证书版本 |
| `ssl_certificate_tasks` | APPLY / RENEW / SYNC / DELETE 异步任务 |
| `ssl_certificate_audit_logs` | 证书专项操作与敏感下载审计 |

## 后端结构

- `internal/domain/provider`：公网 DNS Provider 接口、阿里云 DNS、腾讯云 DNSPod。
- `internal/domain/provider/*certificate.go`：阿里云 CAS、腾讯云 SSL 统一证书 Provider。
- `internal/domain/dnsserver`：不可变记录快照、UDP/TCP Server、转发与 TTL 缓存。
- `service/domain.go`：账号、Zone、Record、批量操作、解析测试和审计业务。
- `service/ssl_certificate.go`：同步、上传、下载、删除安全规则和证书审计。
- `service/ssl_acme.go`：lego ACME DNS-01、任务恢复与自动续签。
- `controller/domain.go`：`/api/v1/domain/*` HTTP API。

DNS 查询只读取 `atomic.Pointer` 指向的内存快照。Zone/Record 变更在数据库事务中生成新快照，提交成功后原子替换；查询过程不会访问 MySQL。

属于已启用内网 Zone 但未命中的名称返回 NXDOMAIN。只有不属于任何内网 Zone 的查询才会按顺序转发到上游 DNS；UDP 响应被截断时自动改用 TCP。

## SSL 证书

SSL 证书复用公网 DNS 账号和 Provider。ACME 申请只允许选择已同步并关联启用 DNS 账号的公网主域名，使用 DNS-01 自动创建/清理 TXT Challenge。签发成功与云端上传是独立状态，云端失败不会覆盖本地有效证书。

Private Key 使用项目统一 AES-256-GCM Secret 工具加密；普通列表/详情 API 永不返回证书正文或密钥。Private Key 与 ZIP 下载使用独立权限并写专项审计；全局操作日志会完全跳过 SSL 上传正文。

可配置项位于 `backend/config.yaml` 的 `ssl`：

```yaml
ssl:
  acme-email: "ops@example.com"
  production-ca: "https://acme-v02.api.letsencrypt.org/directory"
  staging-ca: "https://acme-staging-v02.api.letsencrypt.org/directory"
  dns-polling-seconds: 2
  dns-propagation-seconds: 120
  expiry-warning-days: 30
```

相同配置可通过 `OPS_ADMIN_ACME_EMAIL`、`OPS_ADMIN_ACME_CA_PRODUCTION`、`OPS_ADMIN_ACME_CA_STAGING`、`OPS_ADMIN_ACME_DNS_POLLING_SECONDS`、`OPS_ADMIN_ACME_DNS_TIMEOUT_SECONDS`、`OPS_ADMIN_SSL_EXPIRY_WARNING_DAYS` 覆盖。

## 部署

内网 DNS 默认关闭。生产环境必须在 `backend/config.yaml`（容器部署使用 `deploy/config.yaml`）设置：

```yaml
security:
  credential-key: <至少 32 字节的独立随机密钥>
```

Linux 监听 53 端口使用 `CAP_NET_BIND_SERVICE`，不要以 root 身份运行整个 Web 平台。DNS 模块启动失败只更新运行状态与错误信息，不会导致 HTTP 平台退出。

必须限制配置文件的读取权限。密钥变更会导致已保存的 DNS 云凭据和 SSL Private Key 无法解密。ACME 生产环境启用前应先在 Staging 完成 DNS-01 验证；平台进程还需能够访问 ACME CA、权威 DNS 以及阿里云/腾讯云证书 API。
