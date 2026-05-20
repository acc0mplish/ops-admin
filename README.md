# ops-admin

一个从 `AutoOps` 精简拆分出来的个人运维后台，目前只保留 `system` 领域：

- 用户
- 角色
- 菜单
- 部门
- 岗位
- 登录日志
- 操作日志

## 目录结构

```text
ops-admin/
├─ backend/
└─ web/
```

## 后端

- 端口：`8081`
- 配置文件：`backend/config.yaml`
- 默认管理员：`admin / 123456`

启动：

```bash
cd backend
go run .
```

## 前端

- 端口：`8080`
- 开发代理：`/api/v1 -> http://127.0.0.1:8081`

启动：

```bash
cd web
npm install
npm run dev
```

生产构建：

```bash
cd web
npm run build
```
