# Ops Admin 前后端联调测试报告

- **执行日期：** 2026-08-12
- **验证环境：** Web `http://localhost:8080`；API `http://localhost:8082`
- **测试方式：** Go 服务测试、Vite 生产构建、真实浏览器登录与路由遍历、HTTP API 冒烟与负向验证、前后端静态契约核对、受控性能冒烟。
- **数据策略：** 未修改既有业务数据；环境模型 CRUD 仅创建并清理带 `qa-e2e-*` 标识的临时数据。

## 结论

**当前结论：No-Go（不建议直接作为生产发布候选）。**

主要功能链路、91 个已验证页面路由、环境模型端到端 CRUD、后端测试和前端生产构建均通过。但鉴权与错误响应全部使用 HTTP 200 承载业务错误码，违反 HTTP API 语义，可能让网关、监控、SDK 重试与外部客户端将未授权和参数错误误判为成功。这是 P1 发布阻断项。

## 覆盖范围与结果

| 类别 | 覆盖内容 | 结果 | 证据 |
|---|---|---|---|
| 后端回归 | `backend` 全量 Go 测试 | 通过 | `go test ./...` 退出码 0；`service` 包通过 |
| 前端构建 | Vite 生产构建 | 通过但有性能告警 | 2,182 modules；产物 JS 3,114.54 kB，gzip 919.14 kB |
| 服务健康 | Web 与 API 可达 | 通过 | Web `8080` 与 API `8082/ping` 均返回成功 |
| 认证主流程 | 真实浏览器登录、会话建立、仪表盘加载 | 通过 | 登录后到达 `/dashboard`，未捕获控制台 error |
| 页面联调 | 控制台、资产、容器、标准运维、应用、通知、监控、集成、个人中心 | 通过，存在 2 个异常详情页 | 91 个路由均未回退登录页；详见“问题清单” |
| API 读取冒烟 | profile、资产、K8s、环境、应用、通知、监控、集成 9 组接口 | 通过 | 带有效会话 token 均返回业务码 200 且 data 存在 |
| 环境模型 CRUD | 新增临时环境 → 关键词查询 → 删除 → 再次查询 | 通过 | 临时记录创建成功，删除后查询结果为 0；未自动重建 |
| 删除参数保护 | 环境删除接口 `id=0` | 通过 | 返回业务码 400，未删除数据 |
| 前后端接口映射 | 前端 `web/src/api` 静态路径与后端 router 路由 | 通过 | 前端 375 条静态 API 路径均能在后端 379 条路由字面量中找到映射 |
| API 性能冒烟 | `/ping` 30 并发无副作用请求 | 通过 | 30/30 成功，错误率 0%，P50 52.69 ms，P95 57.39 ms，P99 58.24 ms |
| 安全负向 | 无 token、伪 token、非法登录 JSON、TRACE 方法、响应头 | 失败，存在 P1/P2 | 详见“问题清单” |
| 静态检查 | `go vet ./...` | 失败 | 2 个 IPv6 地址格式问题、1 段不可达代码 |

## 浏览器联调详情

已使用真实浏览器登录，并对 91 个具有页面组件的路由做只读加载验证，涵盖：

- 控制台、业务拓扑、系统管理和审计日志。
- 资产概览、主机、凭据、云账号、数据库、DBMS、网关与环境模型。
- Kubernetes 集群、节点、命名空间、工作负载、Pod、Service、Ingress、网络与存储。
- 服务管理、健康诊断、标准运维、作业、定时任务。
- 应用项目、构建、流水线、镜像仓库、应用拓扑与应用中心。
- 消息通知、监控查询/告警/大屏、AI 与 FinOps 集成页面。

所有被测页面均保持已登录状态并生成可见页面内容。验证了环境模型“新增环境”对话框可打开和关闭，未产生业务数据。

以下重定向与当前路由配置一致：`/assets/hosts`、`/assets/tags`、`/assets/server/databases`、`/applications/center` 等历史/别名路径会跳转到对应当前页面。

## 问题清单

### P1：错误响应始终返回 HTTP 200

- **现象：** 未携带 token 调用 `/api/v1/profile`、`/api/v1/ops/environment/list` 等受保护接口，HTTP 状态均为 200，body 内才带 `code: 401`。伪 token 同样返回 HTTP 200 + `code: 401`；非法登录 JSON 返回 HTTP 200 + `code: 400`。
- **影响：** 反向代理、APM、告警、浏览器外客户端和通用 SDK 可能把失败计为成功；基于 HTTP 状态的重试、缓存与访问审计不可靠。
- **根因：** [backend/httpx/response.go](D:\go\ops-admin\backend\httpx\response.go:11) 的 `Failed` 无条件调用 `c.JSON(200, ...)`。
- **建议：** 改为 `c.JSON(code, ...)`，保留 body 中的业务 code 以兼容前端；回归验证 400/401/403/404/5xx 以及 Axios 拦截器行为。

### P1：初始化超级管理员凭据可预测且在登录页预填

- **现象：** 浏览器登录页预填可用的初始化管理员凭据；初始化逻辑在 [backend/store/seed.go](D:\go\ops-admin\backend\store\seed.go:461) 固定创建管理员及固定密码。
- **影响：** 若部署后未立即改密，攻击者可获得最高权限。
- **建议：** 首次部署改为从环境变量/一次性随机密码初始化；强制首登改密；生产模式禁止预填密码并在 README 标注轮换流程。

### P2：服务详情与日志直达路由在缺少查询参数时抛出未处理异常

- **现象：** 直接访问 `/containers/services/workload` 与 `/containers/services/logs` 时，浏览器控制台出现 mounted hook 未处理异常与 `record not found`。
- **影响：** 用户通过书签、刷新或直接 URL 进入时得到不完整页面与控制台异常。
- **相关位置：** [router/index.js](D:\go\ops-admin\web\src\router\index.js:222)、[ServiceWorkloadDetail.vue](D:\go\ops-admin\web\src\views\assets\ServiceWorkloadDetail.vue:27)、[ServiceWorkloadLogs.vue](D:\go\ops-admin\web\src\views\assets\ServiceWorkloadLogs.vue:23)。
- **建议：** 参数缺失时展示“请选择服务/工作负载”的空态并阻止请求，或将直达路由重定向至服务管理页。

### P2：安全响应头缺失，CORS 范围过宽

- **现象：** `/ping` 响应只包含基础 CORS 与 Content-Type；未发现 `X-Content-Type-Options`、`X-Frame-Options`、`Content-Security-Policy`、HSTS 等安全头。`Access-Control-Allow-Origin: *` 对内部运维平台范围过宽。
- **影响：** 防御纵深不足；未来若引入跨域 token 使用或嵌入页面，攻击面扩大。
- **建议：** 按部署域名配置 CORS 白名单；在反向代理或 Gin 中添加安全响应头。HSTS 仅在 HTTPS 生产环境启用。

### P2：`go vet` 发现 IPv6 连接与不可达代码问题

- **现象：** `go vet ./...` 失败：两处 `fmt.Sprintf("%s:%d", host, port)` 传给 `net.Dial` 时不兼容 IPv6；主机组删除逻辑存在 `return` 后不可达代码。
- **相关位置：** [backend/service/service.go](D:\go\ops-admin\backend\service\service.go:1105)、[backend/service/service.go](D:\go\ops-admin\backend\service\service.go:1693)、[backend/service/service.go](D:\go\ops-admin\backend\service\service.go:2553)。
- **建议：** 使用 `net.JoinHostPort(host, strconv.Itoa(port))`；删除不可达重复检查；将 `go vet ./...` 加入 CI 门禁。

### P3：Element Plus 兼容性告警

- **现象：** 路由遍历中多次出现 `el-radio` 的 `label` 作为 value 即将废弃警告。
- **影响：** 当前功能未阻断，但升级 Element Plus 3 时存在兼容风险。
- **建议：** 将对应 `el-radio` 的 `label` value 用法迁移为 `value`。

### P3：前端首包体积超过 Vite 默认预算

- **现象：** 生产 JS 包 3.04 MB、gzip 919 KB，构建提示 chunk 超过 500 KB。
- **影响：** 弱网和首次加载体验存在风险。
- **建议：** 按应用路由动态导入，拆分 X6/DBMS/监控等重依赖；后续使用 Lighthouse CI 建立 LCP、CLS、TBT 门禁。

## 契约、自动化与发布准备度缺口

- 未发现 OpenAPI/Swagger 定义、Pact 合约、Playwright/Cypress/Vitest 前端测试配置或 CI 工作流。
- 当前前端静态 API 路径和后端路由的字符串映射完整，但这不能替代响应 schema、状态码、权限矩阵与版本兼容性测试。
- 未发现 OSV/Semgrep/ZAP/Secret Scan 等持续安全门禁，也未建立 Lighthouse 或 k6 的 CI 性能预算。
- 因存在 P1 问题，且缺少可重复的 E2E、合约与安全扫描证据，本次不能给出生产发布 Go 结论。

## 建议的修复与复测顺序

1. 修复 HTTP 状态码语义与初始化管理员策略，优先复测登录、401、403、400、删除/保存失败路径。
2. 修复服务工作负载和日志直达页的参数缺失空态，复测书签、刷新和无查询参数访问。
3. 修复 `go vet` 三项问题，并将 `go test ./...`、`go vet ./...`、`npm run build` 设为基础门禁。
4. 新增 Playwright 冒烟：登录、仪表盘、环境 CRUD、资产读取、监控读取、应用流水线只读页面与关键空态。
5. 建立 OpenAPI 或共享 JSON Schema，随后为前端消费者与 Go 提供方添加契约验证。
6. 在预发环境添加 OSV、Semgrep、ZAP baseline、Lighthouse 与 k6 阈值；修复完成后重新执行本报告的全部用例。

## 发布门禁结论

| 门禁 | 状态 |
|---|---|
| 后端单元/服务测试 | 通过 |
| 前端生产构建 | 通过，有 bundle 告警 |
| 浏览器关键页面可达性 | 通过，详情直达页有异常 |
| API 关键读取与环境 CRUD | 通过 |
| HTTP 错误语义与鉴权边界 | **失败，P1** |
| 安全基线 | **失败，P1/P2** |
| 静态检查 | **失败** |
| E2E、契约、性能、安全 CI 证据 | 未建立 |
| **最终决定** | **No-Go，完成 P1 修复与复测前不建议发布** |
