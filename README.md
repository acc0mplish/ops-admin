# Ops Admin

Ops Admin 是一个基于 **Go + Vue 3** 的一体化运维管理平台，面向中小型运维团队提供资产管理、Kubernetes 管理、批量运维、任务编排、监控告警、消息通知和 CI/CD 发布能力。

平台围绕“资产、环境、应用、监控、执行”组织功能，支持通过 SSH 网关访问内网主机、数据库和 Kubernetes 集群。

## 核心能力

### 资产管理

- 资产概览、主机与主机组管理
- SSH 密码、密钥凭据管理
- 云账号与主机信息维护
- Web SSH 终端
- SSH 跳板网关管理
- MySQL 数据库资产与 DBMS 工作台
- SQL 编辑、补全、结果编辑、执行历史、回滚 SQL
- 数据库导入、导出任务

### Kubernetes 管理

- 多集群录入、连接校验和集群切换
- 支持直连或通过 SSH 网关访问 API Server
- 集群概览、证书信息和节点管理
- 命名空间、Pod、工作负载管理
- Pod Web Terminal
- Service、Ingress 和 Gateway API
- 配置与存储资源管理
- YAML 查看、搜索、差异预览和确认编辑
- 工作负载批量更新镜像版本

### 标准运维

- Shell、Bat、Perl、Python、PowerShell、SQL 脚本库
- 命令执行、脚本执行和文件分发
- 主机与主机组互斥选择
- 并发数、超时时间和实时执行结果
- 快速执行历史
- 脚本任务和 HTTP 探针定时任务
- 任务模板、任务日志和批量启停
- 六段 Cron 编辑器：`秒 分 时 日 月 周`
- 可视化作业编排
- 脚本执行、文件分发、人工确认、消息通知步骤
- 作业模板、作业历史和人工确认中心

### 应用中心

- Git、SVN 应用项目管理
- 应用、主机、Kubernetes、数据库、监控和发布拓扑
- 构建任务与构建历史
- 构建阶段日志
- CI/CD 流水线模板与自定义流水线
- 代码拉取、测试、构建、镜像构建、镜像推送和 K8s 发布
- Go、Maven、Vue 等常用流水线模板

### 监控与通知

- Prometheus、VictoriaMetrics 数据源
- PromQL 即时查询
- 告警规则和告警事件
- 告警屏蔽、聚合收敛和重复通知控制
- 告警触发诊断脚本或运维作业
- 主机与 Kubernetes 监控大屏
- 巡检大屏和巡检报告
- 钉钉、企业微信、飞书机器人
- 自定义 HTTP Webhook
- 消息模板、通知媒介、通知规则和发送日志

### 平台管理

- 用户、角色、部门、岗位和菜单权限
- 登录日志和操作日志
- 中文、英文界面切换
- `dev / test / prod` 环境模型

## 技术栈

| 层级 | 技术 |
| --- | --- |
| 后端 | Go 1.24、Gin、GORM、JWT |
| 数据库 | MySQL 8.x |
| Kubernetes | client-go、Kubernetes API |
| 远程连接 | SSH、WebSocket |
| 调度 | robfig/cron v3 |
| 前端 | Vue 3、Vite 5、Vue Router |
| UI | Element Plus |
| 编排画布 | AntV X6 |
| Web 终端 | XTerm.js |

## 项目结构

```text
ops-admin/
├── backend/
│   ├── auth/          # JWT 鉴权
│   ├── config/        # 配置加载
│   ├── controller/    # HTTP 控制器
│   ├── middleware/    # 鉴权、跨域、操作日志
│   ├── model/         # GORM 数据模型
│   ├── router/        # API 路由
│   ├── service/       # 业务逻辑
│   ├── store/         # 数据库连接、迁移和初始化
│   ├── util/          # 公共工具
│   ├── config.yaml    # 后端配置
│   └── main.go
├── web/
│   ├── src/api/       # API 封装
│   ├── src/composables/
│   ├── src/layouts/   # 平台布局
│   ├── src/router/    # 前端路由
│   ├── src/utils/     # 菜单、国际化和工具
│   └── src/views/     # 业务页面
├── docs/
├── scripts/
├── LICENSE
└── README.md
```

## 环境要求

- Go `1.24+`
- Node.js `18+`，建议使用 `20+`
- npm `9+`
- MySQL `8.x`

按实际使用的模块，还需要准备：

- 可访问的 Linux SSH 主机
- Kubernetes `kubeconfig`
- Prometheus 或 VictoriaMetrics
- Git 或 SVN 客户端
- Docker、kubectl 等流水线命令行工具

## 快速开始

### 1. 创建数据库

```sql
CREATE DATABASE ops_admin
  DEFAULT CHARACTER SET utf8mb4
  COLLATE utf8mb4_general_ci;
```

当前项目使用的排序规则为 `utf8mb4_general_ci`。后端启动时会自动执行 GORM 数据表迁移和基础数据初始化。

### 2. 配置后端

修改 `backend/config.yaml`：

```yaml
app:
  name: ops-admin
  port: "8082"
  mode: debug

db:
  host: 127.0.0.1
  port: "3306"
  user: root
  password: "请替换为数据库密码"
  name: ops_admin
  log-mode: false
```

### 3. 启动后端

```bash
cd backend
go mod download
go run .
```

后端默认监听：

- API：`http://127.0.0.1:8082`
- 健康检查：`http://127.0.0.1:8082/ping`

### 4. 启动前端

```bash
cd web
npm install
npm run dev
```

访问 `http://127.0.0.1:8080`。Vite 会将 `/api/v1` 和 `/uploads` 代理到后端 `8082` 端口。

### 5. 初始账号

```text
用户名：admin
密码：123456
```

首次登录后请立即修改默认密码。

## Cron 规则

定时任务使用六段 Cron：

```text
秒 分 时 日 月 周
```

默认表达式：

```text
0 */5 * * * *
```

表示每隔 5 分钟，在第 0 秒执行。系统也兼容传统五段表达式，并自动在最前面补充 `0` 秒。

## 网关访问

当目标资源位于内网时，可以先在“资产管理 → 网关管理”中配置 SSH 跳板机，再为以下资源选择网关访问：

- 主机 SSH
- MySQL 数据库
- Kubernetes API Server

网关自身必须能从 Ops Admin 后端所在服务器访问，并且能够连通最终目标地址。

## 开发与验证

后端格式化和测试：

```bash
cd backend
go fmt ./...
go test ./...
```

前端生产构建：

```bash
cd web
npm run build
```

构建产物输出到 `web/dist/`。

## 安全建议

- 生产环境不要继续使用默认管理员密码。
- 不要将真实数据库密码、SSH 私钥、Token 或 kubeconfig 提交到 Git。
- 限制 Ops Admin 后端对生产网络的访问范围。
- 为高风险 SQL、文件覆盖、K8s YAML 修改和发布操作保留二次确认。
- 定期检查操作日志、执行历史和通知发送日志。
- 建议通过 HTTPS 反向代理访问平台。
- 建议在生产环境关闭 Gin debug 模式。

## 常见问题

### K8s 集群连接失败

检查 kubeconfig、API Server 地址、证书有效期、后端网络连通性以及网关配置。

### SSH 执行失败

检查主机 SSH IP、端口、用户名、凭据、网关路径和服务器防火墙。

### 监控页面没有数据

确认数据源连接正常，并检查 PromQL 对应指标是否存在。

### CI/CD 阶段执行失败

确认后端运行环境已安装阶段所需的 Git、SVN、Docker、kubectl、Go、Node.js 或 Maven。

## 文档

- [平台体验评审](docs/PLATFORM_UX_REVIEW.md)

## License

本项目基于 [GNU General Public License v3.0](LICENSE) 开源。
