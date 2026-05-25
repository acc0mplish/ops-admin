# ops-admin

一个基于 `Golang + Vue 3` 的前后端分离运维管理后台，适合做轻量级内部运维平台、资产台账和权限后台。

项目当前已经具备以下两类核心能力：

- 系统管理：用户、角色、菜单、部门、岗位、基础配置
- 运维资产：主机、主机组、凭据、云账号、主机信息同步、Web SSH 终端

## 技术栈

### 后端

- Golang 1.24
- Gin
- GORM
- MySQL
- JWT
- Gorilla WebSocket
- Excelize
- Tencent Cloud Go SDK

### 前端

- Vue 3
- Vite 5
- Vue Router 4
- Element Plus
- Axios
- XTerm.js

## 功能概览

### 1. 系统管理

- 用户管理
- 角色管理
- 菜单管理
- 部门管理
- 岗位管理
- 基础配置
- 登录日志
- 操作日志

### 2. 资产管理

- 主机管理
- 主机分组管理
- SSH 凭据管理
- 云账号管理
- Excel 批量导入主机
- 主机信息同步
- 云主机同步
- Web SSH 终端登录

## 项目结构

```text
ops-admin/
├── backend/   # Go 后端
└── web/       # Vue 前端
```

更细一点的目录说明：

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
├── util/         # 工具方法与云厂商封装
├── config.yaml   # 后端配置
└── main.go       # 启动入口

web/
├── src/api/      # 前端接口封装
├── src/layouts/  # 布局
├── src/router/   # 路由
├── src/views/    # 页面
└── src/utils/    # 工具方法
```

## 环境要求

- Go `1.24+`
- Node.js `18+`，建议 `20+`
- npm `9+`
- MySQL `8.x`

## 快速开始

### 1. 初始化数据库

先创建数据库：

```sql
CREATE DATABASE ops_admin DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;
```

项目启动时会自动执行：

- 数据表迁移
- 系统基础数据初始化
- 默认管理员、角色、菜单初始化

不需要手动导入 SQL 文件。

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

- 后端默认端口是 `8082`
- 前端开发代理默认转发到 `http://127.0.0.1:8082`

### 3. 启动后端

```bash
cd backend
go mod tidy
go run .
```

启动成功后，服务地址为：

- API: `http://127.0.0.1:8082`
- 健康检查: `http://127.0.0.1:8082/ping`

### 4. 启动前端

```bash
cd web
npm install
npm run dev
```

启动成功后，访问：

- 前端地址: `http://127.0.0.1:8080`

### 5. 默认账号

- 用户名：`admin`
- 密码：`123456`

## 前后端联调说明

前端开发服务器配置见 [web/vite.config.js](D:/go/ops-admin/web/vite.config.js)：

- `/api/v1` -> `http://127.0.0.1:8082`
- `/uploads` -> `http://127.0.0.1:8082`
- 已开启 WebSocket 代理，支持 SSH Web Terminal

## 生产构建

### 前端构建

```bash
cd web
npm run build
```

构建产物默认输出到：

- [web/dist](D:/go/ops-admin/web/dist)

### 后端部署建议

- 将前端静态资源交给 Nginx 托管
- Go 后端单独以二进制方式运行
- 通过 Nginx 反向代理 `/api/v1` 和 `/uploads`
- 如果使用 Web SSH 终端，记得同时代理 WebSocket

## 主要接口前缀

- 登录与公共接口：`/api/v1`
- 系统管理：`/api/v1/admin`、`/api/v1/role`、`/api/v1/menu`、`/api/v1/dept`、`/api/v1/post`
- 日志审计：`/api/v1/sysLoginInfo`、`/api/v1/sysOperationLog`
- 资产管理：`/api/v1/asset/*`

## 特色能力说明

### 1. 自动迁移与初始化

后端启动时会自动执行 `AutoMigrate` 和 `Seed`，适合本地快速起项目。

### 2. Web SSH 终端

项目内置了基于 WebSocket + XTerm.js 的 Web SSH 终端，可直接从浏览器连接已录入的主机。

### 3. 主机批量导入

支持下载 Excel 模板并批量导入主机资产。

### 4. 云主机同步

当前代码已接入腾讯云实例查询能力，可用于将云上主机同步到资产库。

## 当前已知情况

- 仓库里有一些中文文案存在编码异常，README 已修复，但代码与页面内仍有部分乱码文本待进一步清理
- `backend/config.yaml`、`backend/go.mod`、`backend/go.sum` 当前在工作区里已经有未提交修改，完善 README 时未改动这些文件
- 暂未看到自动化测试或 CI 配置，当前更偏向快速启动型后台项目

## 后续建议

- 增加 `.env` 或多环境配置
- 补充 Docker / Docker Compose 部署方案
- 增加初始化 SQL 或一键启动脚本
- 补充单元测试和基础集成测试
- 统一修复项目中的中文编码问题

## License

本项目使用 [LICENSE](D:/go/ops-admin/LICENSE) 中声明的许可证。
