# 架构文档

本目录记录 Ops Admin 的系统边界、关键数据流和可验证的图形化架构资产。文档以当前代码实现为准；需求、计划与未实施项应放在 `docs/` 的专项文档中，不应被描述为已上线能力。

## 文档导航

| 文档 | 用途 |
| --- | --- |
| [FinOps 与 AI 数据流](finops-ai-data-flow.md) | 云账单入库、费用消费端、AI 查询与权限边界 |
| [FinOps 数据流图（HTML）](finops-ai-data-flow.html) | 可离线打开、支持主题与导出的可视化图 |
| [FinOps 数据流图源文件](finops-ai-data-flow.dataflow.json) | Archify JSON 源文件，修改图形时编辑此文件 |
| `ops-admin-platform.html` | 既有平台总览图 |
| `ops-admin-platform.architecture.json` | 既有平台总览图的源文件 |

## 系统分层

| 分层 | 实现 | 职责 |
| --- | --- | --- |
| 控制台 | `web/` | Vue 页面、导航、输入校验、API 调用和结果呈现 |
| API 与领域服务 | `backend/router`、`controller`、`service` | 鉴权、协议处理、业务规则、外部系统访问 |
| 持久化 | `backend/model`、`store`、MySQL | 资产、配置、历史记录、账单与 AI 会话的持久化 |
| 外部能力 | 云厂商、监控数据源、Kubernetes、Git/LDAP/通知 | 仅由后端受控访问 |

## 维护规则

1. 修改图形时先编辑对应 `*.json`，再通过 Archify 渲染 HTML 并执行校验；不要手改生成的 SVG。
2. 新增跨域能力时，在本目录补充数据来源、持久化位置、调用方向和权限要求。
3. 任何“云侧读取”都要在图和文档标明调用入口。FinOps 的云账单 API 入口只能是云账号测试和账单同步。
4. 对 AI，必须标注工具是只读还是需要确认，且不能把模型产生的协议文本当作用户可读结果。

## 生成图形

以下命令在安装了 Archify skill 的环境中执行：

```powershell
$archify = 'C:\Users\Administrator\.codex\skills\archify'
node "$archify\bin\archify.mjs" render dataflow finops-ai-data-flow.dataflow.json finops-ai-data-flow.html
node "$archify\bin\archify.mjs" validate dataflow finops-ai-data-flow.dataflow.json --json
```
