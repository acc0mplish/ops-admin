# Ops Admin Docker Compose 部署手册

本文档面向 Ubuntu 服务器上的单机部署。Compose 会启动独立的 MySQL、Ops Admin API 和 Web 容器，适合个人运维平台、中小团队和功能验证环境。

## 1. 部署拓扑

```text
浏览器
  │ HTTP/HTTPS
  ▼
宿主机 8080
  │
  ▼
ops-admin-web (Nginx)
  ├── /             → Vue 前端
  ├── /api/v1/      → ops-admin-api:8082
  └── /uploads/     → ops-admin-api:8082
                         │
                         ▼
                   ops-admin-mysql:3306

内网 DNS 客户端（仅加载 docker-compose.dns.yml 时）
  │ UDP/TCP 53
  ▼
宿主机内网地址:53 → ops-admin-api:53
```

| 服务 | 容器名 | 对外端口 | 数据持久化 |
| --- | --- | --- | --- |
| Web 控制台 | `ops-admin-web` | `8080` | 无 |
| API | `ops-admin-api` | HTTP 仅 Compose 内网 `8082`；加载 DNS 覆盖文件后发布 `53/UDP`、`53/TCP` | `ops-admin-uploads` |
| MySQL 8 | `ops-admin-mysql` | 仅 Compose 内网 `3306` | `ops-admin-mysql-data` |

MySQL 不映射宿主机端口，不会占用宿主机已有的 `3306`。API 也不直接暴露，浏览器统一通过 Web 容器访问。

## 2. 前置条件

### 2.1 服务器建议

- Ubuntu 22.04 或 24.04
- 最低 2 vCPU、4 GB 内存、20 GB 可用磁盘
- 生产环境建议 4 vCPU、8 GB 内存，并为 Docker 数据目录预留独立磁盘空间
- 能访问代码仓库及 Docker 镜像仓库
- 能访问需要管理的 SSH 主机、Kubernetes API、数据库和监控数据源
- 防火墙或云安全组允许访问 TCP `8080`
- 使用内网 DNS 时，允许可信内网客户端访问宿主机 `53/UDP` 和 `53/TCP`，并允许 API 容器访问上游 DNS 的 `53/UDP` 和 `53/TCP`

### 2.2 软件检查

服务器需要安装 Git、Docker Engine 和 Docker Compose V2：

```bash
git --version
docker version
docker compose version
docker info
```

确认 `8080` 未被占用：

```bash
ss -lntp | grep ':8080' || true
```

如果准备启用内网 DNS，还要确认宿主机 TCP、UDP `53` 均未被占用：

```bash
sudo ss -lntup '( sport = :53 )'
```

存在 `systemd-resolved`、dnsmasq、BIND 或其他 DNS 监听时，必须先调整其监听地址或停用冲突服务；不要在端口仍被占用时启动 Compose。

## 3. 获取项目

```bash
git clone https://github.com/qishu321/ops-admin.git
cd ops-admin
```

生产环境建议部署固定 tag 或 commit，不要长期跟随不确定的开发分支。

## 4. 准备配置

复制模板：

```bash
cp deploy/.env.example deploy/.env
cp deploy/config.yaml.example deploy/config.yaml
chmod 600 deploy/.env deploy/config.yaml
```

使用以下命令分别生成随机值，每次生成结果用于一个配置项，不要复用：

```bash
openssl rand -base64 36
```

### 4.1 配置 `deploy/.env`

至少替换以下内容：

```dotenv
TZ=Asia/Shanghai
MYSQL_DATABASE=ops_admin
MYSQL_USER=ops_admin
MYSQL_PASSWORD=<数据库业务账号密码>
MYSQL_ROOT_PASSWORD=<不同于业务账号的 root 密码>
OPS_ADMIN_JWT_SECRET=<至少 32 字节的稳定随机值>
OPS_ADMIN_INITIAL_USERNAME=admin
OPS_ADMIN_INITIAL_PASSWORD=<首次管理员强密码>
OPS_ADMIN_CORS_ORIGINS=
OPS_ADMIN_DNS_BIND_ADDRESS=<宿主机内网 IP>
```

配置说明：

- `MYSQL_PASSWORD`：Ops Admin 访问 MySQL 使用的密码。
- `MYSQL_ROOT_PASSWORD`：MySQL root 密码，必须与业务账号密码不同。
- `OPS_ADMIN_JWT_SECRET`：登录令牌签名密钥。生产环境必须设置并稳定保存；修改后所有现有登录会话都会失效。
- `OPS_ADMIN_INITIAL_PASSWORD`：仅在数据库中还没有管理员时用于初始化，后续修改该环境变量不会重置已有账号密码。
- `OPS_ADMIN_CORS_ORIGINS`：同源部署保持为空；只有 API 被其他浏览器源直接访问时才填写逗号分隔的完整 Origin。
- `OPS_ADMIN_DNS_BIND_ADDRESS`：内网 DNS 发布到宿主机的地址，生产环境建议填写宿主机内网 IP，不建议直接使用公网 IP。

### 4.2 配置 `deploy/config.yaml`

将数据库密码改成与 `MYSQL_PASSWORD` 完全相同，并替换凭据加密密钥：

```yaml
app:
  name: ops-admin
  port: "8082"
  mode: release

db:
  host: mysql
  port: "3306"
  user: ops_admin
  password: "<与 MYSQL_PASSWORD 完全相同>"
  name: ops_admin
  log-mode: false

security:
  credential-key: "<至少 32 字节的稳定独立随机值>"
```

`security.credential-key` 用于加密平台保存的云凭据、证书私钥等敏感信息。投入使用后不得随意更换，否则既有密文将无法解密。请把 `deploy/.env` 和 `deploy/config.yaml` 纳入受控备份，但不要提交到 Git。

## 5. 启动服务

先检查 Compose 配置，再构建并启动：

```bash
docker compose --env-file deploy/.env config --quiet
docker compose --env-file deploy/.env up -d --build
docker compose ps
```

首次构建会下载 Go、Node.js、Nginx、MySQL 等基础镜像和依赖，耗时取决于网络环境。API 启动时会自动执行数据库迁移和基础数据初始化。

查看启动日志：

```bash
docker compose logs -f mysql
docker compose logs -f api
docker compose logs -f web
```

使用 `Ctrl+C` 退出日志不会停止容器。

## 6. 部署验收

### 6.1 服务状态

```bash
docker compose ps
curl -fsS http://127.0.0.1:8080/api/v1/systemConfig/public
```

三个容器应为 `running` 或 `healthy`。浏览器访问：

```text
http://<服务器 IP>:8080
```

使用 `deploy/.env` 中配置的 `OPS_ADMIN_INITIAL_USERNAME` 和 `OPS_ADMIN_INITIAL_PASSWORD` 登录，首次登录后立即修改管理员密码。

### 6.2 页面验收

登录后应能打开应用平台导航，并切换资产管理、容器管理、标准运维、应用中心、消息通知、集成中心、监控中心和域名管理等工作台。

![Ops Admin 应用平台导航](./screenshots/01-platform-navigation.png)

进入“资产管理 → 资产概览”，确认页面、菜单与接口请求均能正常加载：

![Ops Admin 资产概览](./screenshots/02-asset-overview.png)

进入“容器管理 → K8s 管理 → 集群概览”，选择一个已录入的集群，确认集群基础信息、资源使用率、网络配置和证书信息能够加载：

![Ops Admin Kubernetes 集群概览](./screenshots/03-kubernetes-cluster-overview.png)

进入“标准运维 → 作业中心 → 作业编排”，确认步骤库、编排画布和步骤配置区域能够正常显示：

![Ops Admin 作业编排](./screenshots/04-job-orchestration.png)

进入“应用中心 → 构建与部署 → 构建历史”，确认筛选区、构建状态、当前阶段、耗时和详情入口能够加载：

![Ops Admin 构建历史](./screenshots/05-build-history.png)

进入“消息通知 → 消息模板”，确认模板列表、媒介类型、适用场景和状态筛选能够加载：

![Ops Admin 消息模板](./screenshots/06-message-templates.png)

进入“集成中心 → 导航管理”，确认导航分组、公开访问状态和系统入口卡片能够加载：

![Ops Admin 集成导航](./screenshots/07-integration-navigation.png)

进入“监控中心 → 告警管理 → 告警模板”，确认模板分组、数据源、等级和规则创建入口能够加载：

![Ops Admin 告警模板](./screenshots/08-alert-templates.png)

进入“域名管理 → 内网域名 → Zone 管理”，确认 DNS 服务状态、监听地址、Zone 列表和解析记录入口能够加载：

![Ops Admin 内网 DNS Zone](./screenshots/09-private-dns-zones.png)

建议继续完成以下最小验收：

- 新建一个测试主机并执行连接检查。
- 配置一个测试 Kubernetes 集群并验证 kubeconfig 连接。
- 打开命令执行、作业编排、消息模板和监控模板页面。
- 上传一个小文件，确认 `ops-admin-uploads` 卷可以正常写入。
- 检查浏览器控制台和 `docker compose logs api` 中没有持续错误。

## 7. 网络与可选能力

### 7.1 目标资源连通性

平台发起 SSH、Kubernetes、数据库和监控查询的源端是 `ops-admin-api` 容器。目标网络、防火墙和白名单需要允许该容器经宿主机网络访问相应地址。

如果目标位于隔离网络，可在平台的“资产管理 → 网关管理”配置 SSH 跳板网关，并在对应主机、数据库或 Kubernetes 集群上选择网关访问。

### 7.2 HTTPS

标准 Compose 仅监听宿主机 `8080`。生产环境建议在其前方配置已有的 Nginx、HAProxy、Traefik 或云负载均衡，并完成：

- TLS 证书终止
- HTTP 跳转 HTTPS
- WebSocket 转发
- `/api/v1/` 和 `/uploads/` 的请求体及超时时间配置
- 仅允许可信网段访问管理后台

如果反向代理与 Ops Admin 位于同一台服务器，可将 `docker-compose.yml` 中 Web 端口改为仅监听回环地址：

```yaml
ports:
  - "127.0.0.1:8080:80"
```

### 7.3 开启内网 DNS

默认的 `docker-compose.yml` 不占用宿主机 `53` 端口；未开启内网 DNS 时按前文命令部署即可。需要开启时，再叠加 `docker-compose.dns.yml`，它会将 API 容器的 `53/UDP` 和 `53/TCP` 映射到宿主机，并仅向非 root API 进程增加 `NET_BIND_SERVICE` 能力。DNS 通常先使用 UDP，但较大响应或 UDP 截断后的重试会使用 TCP，因此两个协议必须同时放通。

1. 在 `deploy/.env` 中把 DNS 发布地址设为宿主机的内网 IP，避免直接监听所有公网网卡：

   ```dotenv
   OPS_ADMIN_DNS_BIND_ADDRESS=192.168.10.20
   ```

   没有单独内网地址时可使用 `0.0.0.0`，但必须通过云安全组、边界防火墙或宿主机规则将来源限制到可信网段。Docker 发布端口会修改 iptables/nftables 转发规则，不应只依赖 UFW 作为唯一访问边界。

2. 使用 DNS 覆盖文件重新创建 API 容器，使端口映射和能力生效。后续对这套启用 DNS 的部署执行 `up`、`ps`、`logs`、`down` 等命令时，也应携带相同的两个 `-f` 参数：

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

3. 放通客户端到服务器的入站 DNS 流量。以可信网段 `192.168.10.0/24` 为例：

   ```bash
   sudo ufw allow from 192.168.10.0/24 to any port 53 proto udp
   sudo ufw allow from 192.168.10.0/24 to any port 53 proto tcp
   ```

   云服务器还需要在安全组中添加同样的两条入站规则。不要向 `0.0.0.0/0` 开放递归 DNS，否则可能成为 DNS 放大攻击入口。

4. 确保服务器及 Docker 网络允许 API 容器访问配置的上游 DNS，例如 `223.5.5.5:53`。网络有出站 ACL 时，显式允许到上游 DNS 地址的 `UDP/53` 和 `TCP/53`。

5. 登录 Ops Admin，进入“域名管理 → 内网域名 → DNS 设置”，填写：

   - 状态：启用
   - 监听地址：`0.0.0.0`（这是容器内监听地址，不要填写宿主机 IP）
   - 监听端口：`53`
   - 上游 DNS：填写企业 DNS 或允许访问的公共 DNS

   保存后应显示 UDP、TCP 均运行。然后创建并启用 Zone 与解析记录。

6. 分别从宿主机和另一台内网客户端验证 UDP、TCP：

   ```bash
   # 将 192.168.10.20 和 ops.com 替换为实际地址与 Zone
   dig @192.168.10.20 ops.com A
   dig @192.168.10.20 ops.com A +tcp

   docker compose -f docker-compose.yml -f docker-compose.dns.yml port api 53/udp
   docker compose -f docker-compose.yml -f docker-compose.dns.yml port api 53/tcp
   ```

如果保存后页面显示启动失败，优先检查 `docker compose logs api`、宿主机 `53` 端口冲突、容器的 `NET_BIND_SERVICE` 能力，以及到上游 DNS 的出站策略。

### 7.4 从应用中心执行构建

构建主机不需要安装 Go 或 Node.js。`backend/Dockerfile` 会在 Go 构建阶段生成 Linux 二进制，`web/Dockerfile` 会在 Node.js 构建阶段生成前端产物；构建主机只需要 Git、Docker Engine 和 Docker Compose V2。

在“应用中心 → 构建任务”选择资产主机后，执行路径填写 SSH 用户可写的绝对路径，例如 `/home/ops/ops-admin`。使用 Docker Compose 构建模板时，首次执行会在代码工作目录创建被 Git 忽略的 `deploy/.env` 和 `deploy/config.yaml`；后续构建会复用配置与已有数据卷。不要删除这两个文件，也不要把其中的密钥写入构建日志。

## 8. 常用运维命令

```bash
# 查看状态
docker compose ps

# 查看最近 200 行日志
docker compose logs --tail=200 api
docker compose logs --tail=200 web
docker compose logs --tail=200 mysql

# 跟踪所有服务日志
docker compose logs -f

# 重启单个服务
docker compose restart api

# 停止服务但保留数据
docker compose down

# 再次启动
docker compose --env-file deploy/.env up -d
```

不要执行 `docker compose down -v`，该命令会删除数据库和上传文件卷。

## 9. 备份与恢复

### 9.1 数据库备份

```bash
mkdir -p backup
docker compose exec -T mysql sh -c \
  'exec mysqldump --single-transaction --routines --triggers -u"$MYSQL_USER" -p"$MYSQL_PASSWORD" "$MYSQL_DATABASE"' \
  > "backup/ops-admin-$(date +%F-%H%M%S).sql"
```

确认备份不是空文件：

```bash
ls -lh backup/*.sql
```

### 9.2 上传文件备份

```bash
docker run --rm \
  -v ops-admin-uploads:/data:ro \
  -v "$PWD/backup:/backup" \
  alpine:3.21 \
  tar czf "/backup/ops-admin-uploads-$(date +%F-%H%M%S).tar.gz" -C /data .
```

同时安全备份以下文件：

- `deploy/.env`
- `deploy/config.yaml`
- 数据库 SQL 备份
- 上传文件压缩包
- 当前部署使用的 Git tag 或 commit ID

恢复数据库前应进入维护窗口并确认目标数据库正确：

```bash
docker compose exec -T mysql sh -c \
  'exec mysql -u"$MYSQL_USER" -p"$MYSQL_PASSWORD" "$MYSQL_DATABASE"' \
  < backup/<备份文件>.sql
```

## 10. 升级

升级前先完成数据库、上传文件和配置备份，然后执行：

```bash
git pull --ff-only
docker compose --env-file deploy/.env build --pull
docker compose --env-file deploy/.env up -d
docker compose ps
docker compose logs --tail=200 api
```

升级后重复“部署验收”。后端会自动执行数据库迁移，但数据库结构变更不一定支持仅靠旧镜像逆向回滚；需要回滚时，应同时使用升级前的代码版本和数据库备份。

## 11. 故障排查

### MySQL 一直不健康

```bash
docker compose logs --tail=200 mysql
```

检查磁盘空间、数据卷权限以及 `MYSQL_PASSWORD`、`MYSQL_ROOT_PASSWORD` 是否已替换。已有数据卷不会因为修改 `.env` 自动更改数据库内部密码。

### API 启动失败

```bash
docker compose logs --tail=200 api
```

重点检查：

- `deploy/config.yaml` 中数据库密码是否与 `MYSQL_PASSWORD` 一致。
- `security.credential-key` 是否至少 32 字节。
- `OPS_ADMIN_JWT_SECRET` 是否存在且长度足够。
- MySQL 是否已经健康。

### 页面能打开但接口失败

```bash
curl -v http://127.0.0.1:8080/api/v1/systemConfig/public
docker compose logs --tail=200 web
docker compose logs --tail=200 api
```

检查反向代理是否保留 `/api/v1/` 路径，以及浏览器访问域名是否与 CORS 配置匹配。

### SSH、Kubernetes 或监控连接失败

在 API 容器内检查 DNS 和目标端口：

```bash
docker compose exec api sh
```

确认目标地址可从部署服务器访问，并检查资产凭据、网关路径、Kubernetes API Server 地址及监控数据源认证信息。

### `8080` 端口被占用

修改 `docker-compose.yml` 中 Web 服务的宿主机端口，例如：

```yaml
ports:
  - "18080:80"
```

随后访问 `http://<服务器 IP>:18080`。

## 12. 安全检查清单

- 已替换 MySQL、管理员、JWT 和凭据加密密钥。
- `deploy/.env`、`deploy/config.yaml` 权限为 `600`，且未提交 Git。
- 首次登录后已修改管理员密码。
- 管理端口仅对可信网段开放。
- 生产环境通过 HTTPS 访问。
- 已验证数据库和上传文件备份可用。
- 已记录部署版本、配置变更和升级时间。
- 未暴露 MySQL `3306` 和 API `8082` 到公网。
- 启用内网 DNS 时，`53/UDP`、`53/TCP` 仅允许可信网段访问，且已验证到上游 DNS 的双协议出站连通性。

## 13. 当前限制

- 本 Compose 是单机部署，不提供 MySQL 或 Web/API 的高可用。
- 当前没有开箱即用的离线安装包；完全离线环境需要提前准备构建镜像和依赖缓存。
- 默认 Compose 不发布 DNS 端口；加载 `docker-compose.dns.yml` 后才发布 `53/UDP` 和 `53/TCP`。启用前必须处理宿主机端口冲突，并限制允许访问的来源网段。
- 生产环境的外部 HTTPS、备份调度和日志采集需要接入现有基础设施。
