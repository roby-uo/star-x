# star-X

> 面向个人与团队的自托管 AI API 网关。

star-X 用于在你自己掌控的服务器上管理已授权的上游 AI 账号、分组、API Key、路由、额度与用量。

## Docker Compose 一键部署

前提：已安装 Docker Desktop（或 Docker Engine）和 Docker Compose v2。

```bash
git clone https://github.com/roby-uo/star-x.git
cd star-x/deploy
cp .env.example .env
# 首次启动前，编辑 .env 并填写必填密钥。
docker compose up -d
docker compose ps
```

浏览器访问 `http://localhost:8080`，用 `.env` 中设置的管理员邮箱和密码登录。

默认镜像为：

```text
ghcr.io/roby-uo/star-x:latest
```

Docker 命名卷会保存应用数据、PostgreSQL 和 Redis 数据；正常更新镜像不会删除它们。升级或调整配置前，请先备份 PostgreSQL。

## 实际调用链路

1. 管理员登录；
2. 创建分组；
3. 添加你有权使用的上游 AI 账号或 API 凭据；
4. 为分组创建 API Key；
5. 客户端通过 star-X 地址调用兼容接口。

只完成部署并不能转发模型请求；还需要配置合法授权的上游账号。

## 安全提醒

- 不要提交 `.env`、数据库备份、API Key、OAuth Token 或 `data/` 目录。
- 首次启动前必须为 `POSTGRES_PASSWORD`、`ADMIN_PASSWORD`、`JWT_SECRET`、`TOTP_ENCRYPTION_KEY` 设置不同的高强度随机值。
- 对外提供服务时，请配置 HTTPS、反向代理和访问控制。
- 请遵守上游服务商的条款、账号规则及适用法律。

## 许可证与致谢

star-X 是 [Wei-Shaw/sub2api](https://github.com/Wei-Shaw/sub2api) 的修改发行版，继续依照 [GNU LGPL v3.0 或更高版本](LICENSE) 发布；原始许可证与适用声明均予以保留。
