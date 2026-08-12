# Ops Admin QA 项目上下文

## Product

- **产品：** Ops Admin，面向企业运维与 SRE 场景的内部运维控制台。
- **类型：** Vue 单页管理平台 + Go REST API。
- **当前验证环境：** Web `http://localhost:8080`，API `http://localhost:8082`。
- **关键业务链路：** 登录与鉴权；RBAC 菜单与权限控制；资产、主机与数据库管理；Kubernetes 集群与工作负载操作；标准运维任务执行；应用构建与发布；消息通知；监控查询、告警与事件处置。

## Tech Stack

- **前端：** Vue 3.5、Vue Router 4、Vite 5、Element Plus、Axios，代码位于 `web/`。
- **后端：** Go 1.24、Gin 1.11、REST API、GORM，代码位于 `backend/`。
- **数据与依赖：** MySQL 8；按功能可接入 Redis、MongoDB、Prometheus 或 VictoriaMetrics、Kubernetes、LDAP、腾讯云等外部系统。
- **部署：** Docker Compose 提供 MySQL、API 与 Web 服务；本次联调使用本地服务。

## Test Stack

- **后端：** 已有 Go 测试，主要在 `backend/service/*_test.go`；执行命令为 `go test ./...`。
- **前端构建：** `web/package.json` 提供 Vite build；未发现已配置的 Playwright、Cypress、Vitest 或 Jest 测试套件。
- **本轮联调：** 浏览器关键路径验证、HTTP API 冒烟与 Go 服务测试；后续自动化默认采用 Playwright。

## CI/CD

- 未检测到 GitHub Actions、GitLab CI 或 Jenkins 配置。
- 当前质量门禁以本地后端测试、前端构建与联调报告为准；尚未配置自动上传覆盖率、截图或报告工件。

## Environments

- 本地 Web：`http://localhost:8080`。
- 本地 API：`http://localhost:8082`。
- MySQL 由 Docker Compose 配置，容器内端口为 3306。
- 生产与预发地址、外部依赖凭据及数据脱敏策略未在仓库中提供；本轮不对外部云、集群、数据库或通知渠道实施写操作。

## Quality Goals

- 后端全量 Go 测试必须通过。
- 前端生产构建必须通过。
- 高风险 API 的未认证访问必须被拒绝；关键读取链路应返回可解析的成功响应。
- 关键浏览器流程在本地环境可访问且无阻断性前端控制台错误。
- 作为当前基线：关键 API 冒烟在 10 秒内完成；后续目标为关键 E2E 小于 15 分钟、失败用例可复现。

## Risk Areas

| 区域 | 影响 | 概率 | 分值 | 等级 | 原因与测试动作 |
|---|---:|---:|---:|---|---|
| 认证与 RBAC | 5 | 3 | 15 | Critical | 未授权访问或越权会暴露运维能力；验证认证拦截、登录、权限菜单与接口响应。 |
| 资产、数据库、K8s 的变更/删除 | 5 | 3 | 15 | Critical | 可能造成运行资源或数据误操作；验证参数校验、引用保护、确认交互和接口错误响应。 |
| 应用构建、发布与标准运维任务 | 5 | 3 | 15 | Critical | 会影响线上服务；验证列表、详情、状态反馈与受保护接口。 |
| 环境模型与配置关联 | 4 | 4 | 16 | Critical | 删除后重建曾发生；验证删除后的持久化与列表一致性。 |
| 监控、告警、通知 | 4 | 3 | 12 | High | 失效会延迟事故发现；验证页面加载、查询错误反馈和规则接口。 |
| 外部云、K8s、LDAP 与监控数据源 | 4 | 3 | 12 | High | 依赖可用性和凭据决定结果；本轮只进行无副作用的边界与错误路径检查。 |

## Team

- 当前由开发人员主导质量工作；未提供独立 QA 团队或比例信息。
- 自动化优先覆盖关键业务路径与回归风险，探索性测试用于高风险管理操作的异常场景。

## Conventions

- 前端页面位于 `web/src/views`，接口封装位于 `web/src/api`，后端路由位于 `backend/router`。
- 测试不删除或修改现有用户数据；写操作只在隔离数据或已有防护路径下验证。
- 浏览器自动化优先使用可访问名称、角色和明确页面文案定位；新增稳定自动化时使用 kebab-case `data-testid`。
- 测试工件与报告应放在 `.agents/` 或 `docs/`，不混入生产源码目录。
