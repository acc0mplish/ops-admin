# QA 整改与复测记录（2026-08-12）

本记录对应 `qa-integration-test-report-2026-08-12.md` 中的发布阻断项；原报告保留为整改前的审计基线。

## 已完成

| 优先级 | 整改项 | 复测证据 | 状态 |
| --- | --- | --- | --- |
| P1 | HTTP 失败响应使用实际 HTTP 状态码 | 未携带令牌访问 `/api/v1/profile` 返回 HTTP 401；响应体业务码同步为 401 | 通过 |
| P1 | 取消登录页预填 | 首次初始化密码由 `OPS_ADMIN_INITIAL_PASSWORD` 配置；浏览器登录页两个输入框均为空 | 通过 |
| P2 | 详情、日志页缺失上下文 | 直达 `/containers/services/workload` 与 `/containers/services/logs` 显示指引空态，浏览器控制台无错误 | 通过 |
| P2 | CORS 与安全响应头 | 仅接受 `OPS_ADMIN_CORS_ORIGINS` 明确列出的来源；未列出来源预检不返回允许源；API 返回 nosniff、DENY、Referrer/Permissions/CSP 头 | 通过 |
| P2 | `go vet` | IPv6 地址改为 `net.JoinHostPort`，删除不可达代码；`go vet ./...` 退出码为 0 | 通过 |
| P3 | Element Plus Radio 兼容性 | 将现存 `el-radio` 的 `label` 值迁移为 `value` | 通过 |

## 验证结果

- 后端：`go test ./...` 与 `go vet ./...` 均通过。
- 前端：`npm run build` 通过（2,182 个模块）；仍存在 3.1 MB 初始 JS bundle 的构建警告，未在本轮为避免无关重构而拆包。
- 浏览器：成功完成登出、空登录页检查、重新登录；服务工作负载和日志的直达空态均无控制台错误。

## 部署注意事项

- 部署模板默认在 `deploy/.env` 配置 `OPS_ADMIN_INITIAL_USERNAME=admin` 与 `OPS_ADMIN_INITIAL_PASSWORD=admin@123`；生产环境应修改默认密码。
- 如果 API 需被跨域调用，必须配置逗号分隔的 `OPS_ADMIN_CORS_ORIGINS` 白名单；同源 Nginx 部署不需要该变量。

## 剩余发布关注项

1. 为 E2E、接口契约、安全扫描和性能预算接入 CI。
2. 基于路由拆分前端大包，并以 Lighthouse 指标设定预算。
3. 在 HTTPS 反向代理层按实际域名启用 HSTS。
