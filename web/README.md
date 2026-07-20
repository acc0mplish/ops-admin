# Ops Admin 前端

Ops Admin 前端是基于 Vue 3、Vite、Vue Router 与 Element Plus 的单页控制台。它通过 `/api/v1` 调用后端，不直接保存或使用云厂商凭据。

## 环境要求

- Node.js 18+
- npm 9+
- 可访问的 Ops Admin 后端（开发默认 `http://127.0.0.1:8082`）

## 安装与运行

```powershell
cd web
npm ci
npm run dev
```

Vite 开发服务器默认监听 `http://127.0.0.1:8080`。`vite.config.js` 将 `/api/v1` 与 `/uploads` 代理到后端 `8082` 端口。

生产构建：

```powershell
cd web
npm run build
npm run preview
```

## 目录说明

| 路径 | 说明 |
| --- | --- |
| `src/views/` | 按业务域组织的页面组件 |
| `src/views/integration/finops/` | 云账号、费用看板、费用拆分、优化建议、资源拆分与账单同步页面 |
| `src/views/integration/` | AI 助手、模型、工具集和会话页面 |
| `src/api/` | 后端 API 封装；统一使用 HTTP 客户端与认证处理 |
| `src/router/` | 页面路由与导航注册 |
| `src/utils/` | 通用请求、格式化和应用导航逻辑 |

## FinOps 前端约定

- 费用看板默认展示“含当前月的最近 6 个自然月”；趋势月份必须连续，缺少账单显示 `0`。
- 当前月费用需要标识“截至当前”或 `is_partial`，不能与已结账月份混淆。
- 费用拆分以单一 `YYYY-MM` 账期查询；资源拆分必须先选择云账号和日期范围，地域与资源类型筛选位于资源明细区域。
- 费用明细的逐日账单表不在费用拆分页展示，以免大账单造成页面与请求超时。
- 优化建议名称必须能辨识云账号、分析账期与策略；AI 策略与默认策略均展示相同的可追溯范围。

前端只向后端请求数据：**不会从浏览器直接调用云厂商 API，也不会在 AI 对话中触发账单同步**。云费用数据先由后端“账单同步”入库，再由看板、拆分、建议与 AI 工具查询。

## AI 助手页面

- 工具集页面展示工具权限、启用状态与是否需要人工确认。
- 云费用分析工具的数据来源固定为本地已同步账单，可查询趋势、产品、地域和资源聚合。
- 对话结果可能包含 Markdown；页面应渲染为可读正文，不能把模型的工具协议文本直接暴露给用户。

## 开发检查

当前项目未单独配置 lint 或 TypeScript 脚本；提交前至少执行一次构建：

```powershell
cd web
npm run build
```

## 相关文档

- [架构文档索引](../docs/architecture/README.md)
- [FinOps 与 AI 数据流](../docs/architecture/finops-ai-data-flow.md)
