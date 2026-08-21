# Ops Admin 后端

Ops Admin 后端基于 Go、Gin、GORM 和 MySQL，向 Web 控制台提供 `/api/v1` REST API，并承载资产、可观测性、自动化、AI 助手和云费用（FinOps）能力。

## 目录与职责

| 目录 | 职责 |
| --- | --- |
| `router/` | 路由注册、鉴权中间件与 API 分组 |
| `controller/` | 请求参数校验和 HTTP 响应 |
| `service/` | 业务编排、领域规则与外部系统访问 |
| `model/` | GORM 实体与数据结构 |
| `store/` | MySQL 初始化、迁移与种子数据 |
| `config/` | 配置加载 |

入口为 `main.go`：加载 `config.yaml`、连接 MySQL、执行迁移与种子数据，再创建 Gin 路由。

## 环境要求

- Go 1.24+
- MySQL 8.0+（或兼容的 MySQL 实例）
- 前端开发时使用 Node.js 18+

## 配置

复制并按环境修改 `config.yaml`。至少需要配置应用端口和 MySQL 连接信息：

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

`config.yaml` 可能包含本地开发凭据；不要将真实生产密码、云密钥或模型密钥提交到仓库。生产环境应使用受控的配置注入和最小权限数据库账号。

生产环境必须通过 `OPS_ADMIN_JWT_SECRET` 环境变量注入随机 JWT 签名密钥。登录会话采用 60 分钟 Access Token、连续 6 小时无操作超时和 7 天最长周期；前端会在 Access Token 到期前 5 分钟静默刷新。

域名管理使用 `config.yaml` 中的 `security.credential-key` 对公网 DNS 凭据和 SSL Private Key 进行 AES-GCM 加密；生产环境必须设置至少 32 字节的稳定独立随机密钥。内网 DNS 默认关闭。Linux 上监听 53 端口时不要让整个平台以 root 运行，可为二进制授予最小能力：

```bash
setcap 'cap_net_bind_service=+ep' /path/to/ops-admin
```

systemd 也可以使用：

```ini
AmbientCapabilities=CAP_NET_BIND_SERVICE
CapabilityBoundingSet=CAP_NET_BIND_SERVICE
```

## 本地开发

```powershell
cd backend
go mod download
go run .
```

默认监听 `http://127.0.0.1:8082`。启动会自动执行数据库迁移及必要的种子数据初始化。

验证：

```powershell
cd backend
go test ./...
```

## 关键模块

### 资产与可观测性

- 主机、数据库、Kubernetes、凭据和网关等资产由 CMDB 模型统一维护。
- 主机 CPU、内存、磁盘使用率从已配置的 Prometheus 或 VictoriaMetrics 查询 node_exporter 指标；没有匹配到监控目标时会明确返回不可用状态，而非伪造数值。
- 监控、日志和告警访问由服务层统一封装，前端不直接访问数据源。

### AI 助手

- 模型、会话、工具配置和工具执行记录持久化在本地数据库。
- 只读工具可自动执行；会造成外部状态变更的工具必须进入待确认状态，只有人工确认后才执行。
- 兼容 OpenAI 原生工具调用和部分模型返回的 DSML 工具标记。单次会话最多进行 3 轮工具调用，避免无界循环。

### 云费用 FinOps

FinOps 的边界必须保持清晰：

1. **云账号测试和账单同步**是唯一允许调用云厂商账单 API 的后端路径。
2. 同步按自然月逐月执行。未传范围时为含当前月的最近 6 个自然月；单月失败不会阻断其他月份。
3. 同一账号、同一账单记录通过幂等写入（upsert）更新，不重复累计。
4. 费用看板、费用拆分、资源拆分、优化建议和 AI 云费用工具都只查询本地已同步账单数据，绝不触发云侧拉取或同步。
5. 当前月允许为不完整账单；阿里云月度实例账单的日维度展示为本地日均摊估算，页面与接口需保留该口径说明。

主要接口位于 `/api/v1/integration/finops/*`：

| 类别 | 说明 |
| --- | --- |
| `account/*` | 云账号管理、连接测试 |
| `sync/trigger`、`sync/logs` | 按月同步与同步历史 |
| `dashboard`、`breakdown`、`resource/list` | 本地账单的聚合查询 |
| `recommendation/*` | 默认或 AI 策略生成、查看、更新和删除建议 |
| `cost/import` | 导入标准 JSON 账单 |

AI 工具 `finops_cost_analysis` 只读取已同步费用记录；可按账号、月份、服务、地域和资源维度聚合，不能用它读取云厂商实时账单。

## 变更约定

- Controller 负责校验和协议转换，业务规则放入 `service/`，不要把云厂商调用散落在 Controller 或 AI 工具中。
- 涉及账单金额的改动要同时校验：记录数、金额、账期、币种以及“当前月是否不完整”的标识。
- 新增 AI 工具时必须声明权限、参数模式、是否需要确认和数据来源；涉及云费用时默认只允许本地数据库查询。

## 相关文档

- [架构文档索引](../docs/architecture/README.md)
- [FinOps 与 AI 数据流](../docs/architecture/finops-ai-data-flow.md)
- [FinOps 优化方案](../docs/FINOPS_OPTIMIZATION_PLAN.md)
