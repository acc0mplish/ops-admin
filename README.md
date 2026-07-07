# Ops Admin

Ops Admin 是一个基于 `Golang + Vue 3` 的前后端分离运维管理平台，面向内部运维、资产管理、Kubernetes 管理、标准运维、监控告警、消息通知和应用构建发布等场景。

## 技术栈

后端：

- Go 1.24+
- Gin
- GORM
- MySQL
- JWT
- Gorilla WebSocket
- Excelize

前端：

- Vue 3
- Vite 5
- Vue Router 4
- Element Plus
- Axios
- XTerm.js

## 功能模块

### 控制台

- 仪表盘
- 个人信息
- 系统管理
- 登录日志
- 操作日志

### 资产管理

- 资产概览
- 主机管理
- 主机组管理
- SSH 凭据管理
- 云账号管理
- Web SSH 终端
- 数据库资产管理
- DBMS 工作台
- Kubernetes 集群与资源管理

### 标准运维

- 脚本库
- 命令执行
- 脚本执行
- 文件分发
- 快速执行历史
- 定时任务
- 任务日志
- 任务模板
- 作业编排
- 作业列表
- 人工确认
- 作业历史
- 作业模板

### 应用中心

- 项目列表
- 构建任务
- 构建历史
- Git/SVN 仓库构建发布

### 消息通知

- 消息模板
- 通知媒介
- 通知规则
- 发送日志

支持钉钉机器人、企业微信机器人、飞书机器人和自定义 HTTP Webhook。

### 监控中心

- 监控概览
- 数据源管理
- PromQL 即时查询
- 告警规则
- 告警事件
- 告警屏蔽
- 聚合收敛
- 监控大屏
- 巡检大屏

## 项目结构

```text
ops-admin/
├── backend/   # Go 后端
├── web/       # Vue 前端
├── docs/      # 项目文档与体验评审
└── README.md
```

后端目录：

```text
backend/
├── auth/         # JWT 鉴权
├── config/       # 配置加载
├── controller/   # HTTP 控制器
├── middleware/   # 认证、跨域、操作日志
├── model/        # GORM 模型
├── router/       # 路由注册
├── service/      # 业务逻辑
├── store/        # 数据库连接、迁移、初始化数据
├── util/         # 工具方法
├── config.yaml   # 后端配置
└── main.go       # 启动入口
```

前端目录：

```text
web/
├── src/api/      # 前端接口封装
├── src/layouts/  # 布局
├── src/router/   # 路由
├── src/utils/    # 工具方法、应用菜单、i18n
└── src/views/    # 页面
```

## 环境要求

- Go `1.24+`
- Node.js `18+`，建议 `20+`
- npm `9+`
- MySQL `8.x`

## 快速开始

### 1. 初始化数据库

```sql
CREATE DATABASE ops_admin DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;
```

项目启动时会自动执行：

- 数据表迁移
- 基础数据初始化
- 默认管理员、角色和菜单初始化

### 2. 配置后端

编辑 [backend/config.yaml](D:/go/ops-admin/backend/config.yaml)：

```yaml
app:
  name: ops-admin
  port: "8082"
  mode: debug

db:
  host: 127.0.0.1
  port: "3306"
  user: root
  password: "123456"
  name: ops_admin
  log-mode: false
```

说明：

- 后端默认端口：`8082`
- 前端开发代理默认转发到：`http://127.0.0.1:8082`

### 3. 启动后端

```bash
cd backend
go mod tidy
go run .
```

启动成功后：

- API 地址：`http://127.0.0.1:8082`
- 健康检查：`http://127.0.0.1:8082/ping`

### 4. 启动前端

```bash
cd web
npm install
npm run dev
```

访问：

- 前端地址：`http://127.0.0.1:8080`

### 5. 默认账号

- 用户名：`admin`
- 密码：`123456`

## 常用命令

后端测试：

```bash
cd backend
go test ./...
```

前端构建：

```bash
cd web
npm run build
```

## 开发说明

- 数据库排序规则建议使用 `utf8mb4_general_ci`。
- 后端启动时会自动执行 GORM AutoMigrate。
- Web SSH、Pod Terminal 等能力依赖 WebSocket 代理。
- DBMS、K8s、监控、应用构建等能力需要连接真实外部系统后才能完整联调。

## 文档

- 平台体验评审：[docs/PLATFORM_UX_REVIEW.md](D:/go/ops-admin/docs/PLATFORM_UX_REVIEW.md)

## 后续优化方向

- 全项目中文编码与错误提示统一。
- 大型页面组件拆分。
- 高危操作二次确认与审计增强。
- K8s GitOps 场景识别。
- DBMS SQL 风险识别与只读模式。
- 应用中心构建阶段结构化与实时日志。
- 监控大屏拖拽布局与变量能力。

## License

请参考 [LICENSE](D:/go/ops-admin/LICENSE)。
