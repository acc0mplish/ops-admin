# 资产网关管理需求文档

## 1. 背景

当前资产管理已覆盖服务器、数据库、K8s 集群等资源，但这些资源可能位于内网、专线网络、办公网隔离区或云上 VPC 中，Ops Admin 后端无法直接访问目标地址。

典型场景：

- 服务器 B 只能从跳板机 A 通过 SSH 访问。
- MySQL 数据库只开放给内网机器 A，平台需要通过 A 转发后访问数据库。
- K8s API Server 使用内网地址，平台需要通过网关 A 才能访问集群 API。

因此需要在资产管理中新增“网关管理”，让主机、数据库、K8s 集群可以选择直连或通过指定网关访问。

## 2. 建设目标

第一阶段目标：

- 新增网关资源的增删改查、启用禁用、连通性测试。
- 支持一跳 SSH 网关，也就是平台后端先连网关 A，再通过网关 A 访问目标 B。
- 主机管理、数据库管理、K8s 管理都能配置网关。
- 平台所有实际连接行为必须复用网关配置，不能只在页面展示。
- 删除网关前检查引用关系，避免主机、数据库、K8s 集群连接配置失效。

暂不纳入第一阶段：

- 多级网关链路，例如 A -> B -> C。
- SOCKS5、HTTP Proxy 网关。
- 网关高可用自动切换。
- 凭据加密存储体系重构。

## 3. 菜单与页面

资产管理下新增二级菜单：

- 服务器管理
- 数据库管理
- K8s 管理
- 网关管理

网关管理页面包含：

- 网关列表
- 新增网关
- 编辑网关
- 查看详情
- 连通性测试
- 启用 / 禁用
- 删除

列表字段建议：

| 字段 | 说明 |
| --- | --- |
| 网关名称 | 用户可识别名称 |
| 网关地址 | SSH 地址，例如 192.168.1.10:22 |
| 认证凭据 | 复用资产凭据或独立选择网关凭据 |
| 网络区域 | 例如 prod-vpc、office、idc-a |
| 状态 | 启用、禁用 |
| 连通状态 | 未检测、正常、失败 |
| 引用数量 | 被多少主机、数据库、K8s 集群使用 |
| 最近检测时间 | 最后一次测试时间 |
| 操作 | 测试、详情、编辑、禁用、删除 |

## 4. 网关数据模型

新增 `asset_gateway` 表。

建议字段：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| id | uint | 主键 |
| name | string | 网关名称，必填 |
| code | string | 网关编码，可选，便于自动化引用 |
| gateway_type | string | 第一阶段固定为 `ssh` |
| host | string | 网关 IP / 域名，必填 |
| port | int | SSH 端口，默认 22 |
| credential_id | uint | 网关登录凭据，必填 |
| network_zone | string | 网络区域 |
| status | int | 1 启用，2 禁用 |
| connect_status | int | 0 未检测，1 正常，2 失败 |
| last_check_time | datetime | 最近检测时间 |
| description | string | 备注 |
| created_at | datetime | 创建时间 |
| updated_at | datetime | 更新时间 |

第一阶段网关凭据建议复用现有 `asset_credential`，避免重复做密码、密钥管理。

## 5. 连接模式

主机、数据库、K8s 集群都新增连接模式：

| 模式 | 说明 |
| --- | --- |
| direct | 直连，保持现有逻辑 |
| gateway | 通过指定网关访问 |

对应字段建议：

- `connection_mode`
- `gateway_id`

为了兼容历史数据：

- 旧数据默认 `connection_mode = direct`
- `gateway_id = null`
- 页面展示为“直连”

## 6. 主机管理接入网关

### 6.1 现状检查

当前主机 SSH 连接入口集中在：

- `backend/service/service.go` 的 `newSSHClient(host model.AssetHost)`
- `backend/service/ops.go` 的命令执行、脚本执行、文件分发都会间接调用 SSH 连接
- 主机终端、主机探活、采集主机信息也依赖 SSH 连接

当前逻辑是：

```text
Ops Admin 后端 -> 目标主机 SSH
```

网关模式应改为：

```text
Ops Admin 后端 -> 网关 SSH -> 目标主机 SSH
```

### 6.2 主机表改造

`asset_host` 新增：

- `connection_mode`
- `gateway_id`

主机新增 / 编辑页面增加：

- 连接方式：直连 / 通过网关
- 网关选择器：仅连接方式为“通过网关”时必填

主机列表增加：

- 连接方式
- 网关名称

### 6.3 SSH 连接逻辑

新增统一方法：

```go
func (s *Service) newSSHClientForHost(host model.AssetHost) (*ssh.Client, error)
```

直连：

```text
ssh.Dial("tcp", host.SSHIP:host.SSHPort, targetConfig)
```

网关：

```text
1. ssh.Dial("tcp", gateway.host:gateway.port, gatewayConfig)
2. gatewayClient.Dial("tcp", host.SSHIP:host.SSHPort)
3. ssh.NewClientConn(targetConn, host.SSHIP:host.SSHPort, targetConfig)
4. ssh.NewClient(...)
```

所有主机 SSH 操作统一走这个方法：

- 主机连通性测试
- 主机终端登录
- 快速执行：命令执行
- 快速执行：脚本执行
- 快速执行：文件分发
- 定时任务脚本执行
- 作业编排中的脚本执行、文件分发

## 7. 数据库管理接入网关

### 7.1 现状检查

当前数据库连接入口主要在：

- `backend/service/database.go`
- `inspectMySQLDatabase(...)`
- `openDatabaseByID(...)`

当前逻辑是：

```text
sql.Open("mysql", user:pass@tcp(dbHost:dbPort)/schema)
```

这意味着数据库现在只能直连。

### 7.2 数据库表改造

`asset_database` 新增：

- `connection_mode`
- `gateway_id`

数据库新增 / 编辑页面增加：

- 连接方式：直连 / 通过网关
- 网关选择器：通过网关时必填

数据库列表增加：

- 连接方式
- 网关名称

### 7.3 MySQL 连接逻辑

第一阶段建议使用 MySQL Driver 自定义 Dialer。

方案：

```text
1. 根据 database.gateway_id 获取网关
2. 建立到网关的 SSH Client
3. 通过 gatewayClient.DialContext 连接 dbHost:dbPort
4. 使用 mysql.RegisterDialContext 注册带网关标识的 network
5. DSN 使用该 network 访问 MySQL
```

注意点：

- 自定义 network 名称需要包含 gatewayID 和 databaseID，避免不同数据库连接池串线。
- `sql.DB` 关闭时要释放网关 SSH client。
- DBMS 查询、表数据编辑、SQL 执行、导出任务、导入任务都必须复用同一个连接工厂。

需要接入的能力：

- 数据库连接测试
- SQL 编辑器执行
- 表结构读取
- 表数据分页查询
- 单元格编辑
- SQL 执行历史
- 数据库导出
- 跨数据库导入

## 8. K8s 管理接入网关

### 8.1 现状检查

K8s 常规资源管理入口集中在：

- `backend/service/k8s.go` 的 `k8sClientForCluster(...)`

但 Pod 终端还有独立入口：

- `backend/service/k8s_terminal.go`
- 目前直接使用 `clientcmd.RESTConfigFromKubeConfig`

应用中心 CI/CD 的 K8s 发布使用：

- `backend/service/ops_application.go`
- 当前通过临时 kubeconfig 调用 `kubectl`

### 8.2 K8s 集群表改造

`k8s_cluster` 新增：

- `connection_mode`
- `gateway_id`

K8s 集群录入 / 编辑页面增加：

- 连接方式：直连 / 通过网关
- 网关选择器：通过网关时必填

集群保存校验逻辑：

- 直连：保持当前校验。
- 网关：先测试网关，再通过网关访问 API Server。
- 失败提示仍使用：`集群连接失败，请检查 kubeconfig`
- 可以在详情里额外展示失败原因，便于排查。

### 8.3 K8s API 访问逻辑

常规资源管理应在 `k8sClientForCluster(...)` 中统一接入网关。

建议：

```text
1. 解析 kubeconfig 得到 API Server host:port
2. 如果 connection_mode = gateway：
   - 建立 SSH 网关连接
   - http.Transport.DialContext 使用 gatewayClient.DialContext
   - TLS ServerName 保持 kubeconfig 中原始 server 的主机名
3. 所有 k8sGetJSON / k8sDo / YAML / 删除 / 更新 都复用该 http.Client
```

需要覆盖：

- 集群概览
- 节点管理
- 命名空间
- Pod 管理
- 工作负载
- 服务
- Ingress
- 高级网络
- 配置与存储
- YAML 查看 / 编辑
- Pod 日志
- Pod 终端

### 8.4 Pod 终端改造

`k8s_terminal.go` 当前没有复用 `k8sClientForCluster(...)`，需要单独处理。

建议新增：

```go
func (s *Service) restConfigForCluster(cluster model.K8sCluster) (*rest.Config, func(), error)
```

直连返回原始 rest.Config。

网关模式：

- 给 `rest.Config.Transport` 或 `rest.Config.Dial` 注入网关 Dialer。
- 或创建带网关 DialContext 的 `http.Transport`。
- cleanup 负责关闭网关 SSH client。

### 8.5 CI/CD K8s 发布改造

当前应用中心 K8s 发布阶段使用 `kubectl --kubeconfig tempFile`，如果 API Server 只能通过网关访问，kubectl 本身不会自动走平台网关。

可选方案：

1. 推荐：后端改为使用 Kubernetes API 执行镜像更新和 rollout status，不再依赖 kubectl。
2. 兼容：执行前创建本地临时端口转发，将 kubeconfig server 改写为 `https://127.0.0.1:localPort`，kubectl 走本地隧道访问集群。

第一阶段建议选择方案 1，和 K8s 管理模块共享同一套网关访问逻辑。

## 9. 后端服务设计

新增 `backend/service/gateway.go`。

核心能力：

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

删除网关前必须检查：

- `asset_host.gateway_id`
- `asset_database.gateway_id`
- `k8s_cluster.gateway_id`

如存在引用，禁止删除，并返回：

```text
该网关正在被资产使用，无法删除
```

## 10. API 设计

新增接口：

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/asset/gateway/list` | 网关列表 |
| GET | `/asset/gateway/options` | 网关下拉选项 |
| GET | `/asset/gateway/info` | 网关详情 |
| POST | `/asset/gateway/create` | 新增网关 |
| POST | `/asset/gateway/update` | 更新网关 |
| POST | `/asset/gateway/delete` | 删除网关 |
| POST | `/asset/gateway/status` | 启用 / 禁用 |
| POST | `/asset/gateway/test` | 连通性测试 |

主机、数据库、K8s 集群现有创建 / 更新接口增加：

```json
{
  "connectionMode": "direct",
  "gatewayId": 0
}
```

## 11. 前端接入点

需要改造：

- `web/src/utils/apps.js`
  - 资产管理应用菜单新增“网关管理”。
- `web/src/router/index.js`
  - 新增 `/assets/gateways`。
- 主机管理页面
  - 新增连接方式和网关选择器。
  - 列表展示网关。
- 数据库管理页面
  - 新增连接方式和网关选择器。
  - 列表展示网关。
- K8s 集群管理页面
  - 新增连接方式和网关选择器。
  - 保存前按连接方式校验。

新增页面建议：

- `web/src/views/assets/Gateway.vue`

## 12. 权限与审计

建议增加操作审计：

- 创建网关
- 修改网关
- 删除网关
- 测试网关
- 主机 / 数据库 / K8s 集群绑定网关
- 主机 / 数据库 / K8s 集群解绑网关

安全要求：

- 网关凭据不在接口响应中返回明文。
- 网关测试失败信息前端展示简要原因，详细错误写入后端日志。
- 禁用网关后，引用该网关的资产连接应直接失败，并提示网关已禁用。

## 13. 验收标准

### 网关管理

- 可以新增 SSH 网关。
- 可以编辑、禁用、启用、删除网关。
- 可以测试网关 SSH 连接。
- 被资产引用的网关不能删除。

### 主机管理

- 主机可以选择直连或网关连接。
- 网关模式下，主机连通性测试成功。
- 网关模式下，主机终端可以打开。
- 网关模式下，快速命令、脚本执行、文件分发可用。

### 数据库管理

- 数据库可以选择直连或网关连接。
- 网关模式下，连接测试成功。
- 网关模式下，SQL 编辑器可执行查询。
- 网关模式下，表数据编辑、导出、导入可用。

### K8s 管理

- K8s 集群可以选择直连或网关连接。
- 网关模式下，保存集群前必须校验 kubeconfig。
- 网关模式下，集群概览、节点、命名空间、Pod、工作负载、服务、Ingress、配置存储可正常读取。
- 网关模式下，Pod 终端、日志、YAML 查看与编辑可用。
- 应用中心 K8s 发布阶段可以选择通过网关访问目标集群。

## 14. 开发优先级

P0：

- 网关模型、迁移、CRUD、测试。
- 主机 SSH 连接接入网关。
- 数据库连接测试和 SQL 执行接入网关。
- K8s `k8sClientForCluster` 接入网关。

P1：

- Pod 终端接入网关。
- DBMS 导出、导入任务接入网关。
- 快速执行、定时任务、作业编排接入网关。

P2：

- 应用中心 K8s 发布阶段彻底去 kubectl 化，改为 Kubernetes API。
- 网关连接池和复用优化。
- 网关健康巡检和引用资产拓扑。

## 15. 风险与注意事项

- MySQL 自定义 Dialer 必须避免全局 network 名称冲突。
- SSH 网关连接不能长期泄漏，需要严格 cleanup。
- K8s TLS 校验需要保留原始 API Server 的 ServerName，不能因为走本地隧道破坏证书校验。
- `kubectl` 不支持自动走平台内部网关，需要改造 CI/CD K8s 发布链路。
- 批量任务通过网关执行时，要控制并发，避免网关连接数过高。
- 如果后续支持多级网关，当前 `gateway_id` 需要演进为 `gateway_chain_id` 或网关链路表。
