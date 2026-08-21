# Ubuntu Docker Compose 部署

本方案会创建独立的 `ops-admin-mysql` MySQL 8 容器，不使用也不暴露宿主机现有的 MySQL `3306`。平台仅通过宿主机 `8080` 提供访问：`http://<服务器 IP>:8080`。

## 前置条件

- Ubuntu 已安装 Docker Engine 和 Docker Compose V2：`docker compose version`
- 防火墙和云安全组允许 TCP `8080`
- 服务器能访问 Docker 镜像仓库；运行后能访问被平台管理的主机、Kubernetes API 和已配置的数据源
- 建议至少 2 vCPU / 4 GB 内存 / 20 GB 磁盘；生产环境建议 4 vCPU / 8 GB

## 首次部署

在项目根目录执行：

```bash
mkdir -p deploy
cp deploy/.env.example deploy/.env
cp deploy/config.yaml.example deploy/config.yaml
chmod 600 deploy/.env deploy/config.yaml
```

编辑 `deploy/.env`，设置两个不同的高强度随机密码。然后把 `deploy/config.yaml` 中 `db.password` 改为与 `MYSQL_PASSWORD` 完全相同的值，并将 `security.credential-key` 替换为至少 32 字节的独立随机密钥。该密钥投入使用后禁止更换。

启动：

```bash
docker compose --env-file deploy/.env up -d --build
docker compose ps
curl http://127.0.0.1:8080/api/v1/systemConfig/public
```

## 从应用中心执行构建

构建主机**不需要安装 Go 或 Node.js**。`backend/Dockerfile` 会在 `golang:1.24-alpine` 构建阶段交叉编译 Linux 二进制，`web/Dockerfile` 会在 Node 构建阶段生成前端产物；构建主机只需 Docker Engine、Docker Compose V2 与 Git。模板会优先直接调用 Docker；若当前 SSH 用户没有 Socket 权限，则自动尝试 `sudo -n docker`。

在“应用中心 → 构建任务”中选择“资产主机”后，执行路径建议填写该 SSH 用户可写的绝对路径，例如 `/home/testvm/ops-admin`。点击“套用 Docker Compose 模板”即可写入构建与构建后脚本。

无需提前登录构建主机准备文件。模板首次执行时会在代码工作目录自动创建被 Git 忽略的 `deploy/.env` 与 `deploy/config.yaml`，并生成两组随机 MySQL 密码；后续构建会复用这些配置和已有数据库卷。请将执行路径放在 SSH 用户可写位置，且不要手工删除这两个文件。

首次 API 启动会自动执行数据库迁移和初始化数据。登录后请立即修改默认管理员密码。

## 常用运维命令

```bash
# 查看日志
docker compose logs -f api
docker compose logs -f web

# 更新代码并重新构建
git pull
docker compose --env-file deploy/.env up -d --build

# 停止服务（保留数据）
docker compose down

# 备份数据库到当前目录
docker compose exec -T mysql sh -c 'exec mysqldump -u"$MYSQL_USER" -p"$MYSQL_PASSWORD" "$MYSQL_DATABASE"' > ops-admin-$(date +%F).sql
```

数据保存在 Docker 命名卷 `ops-admin-mysql-data` 与 `ops-admin-uploads`。不要执行 `docker compose down -v`，否则会删除数据库和上传文件。

## 说明

- MySQL 没有映射宿主机端口，因此不会与现有 `mysql` 容器冲突。
- Nginx 将 `/api/v1/` 和 `/uploads/` 代理到 API，并支持 WebSocket 终端连接。
- 当前配置以 `deploy/config.yaml` 只读挂载注入，密码不会写入镜像或 Git 仓库。
- 生产 HTTPS 可在现有反向代理前置转发到宿主机 `8080`；本 Compose 不占用已被使用的 `80/443`。
