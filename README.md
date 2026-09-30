# star-X

> Self-hosted AI API gateway for teams and individuals.

star-X is a self-hosted, privately branded distribution for managing authorized upstream AI accounts, API keys, routing, quotas, and usage. It is designed to run under your control with Docker Compose.

![star-X logo](frontend/public/brand/star-x-logo-original.jpg)

## 最新更新

### 2026-09-30

#### 新增

- MiniMax 视频接入扩展到 H3 / H3-Max：分别支持 768P／2K、4–15 秒和 480P／768P、5–15 秒，可独立配置开放规格与每秒售价。
- MiniMax 账号可一键添加两个视频模型，语言模型预设补充 M3、M2.7 等系列；用户可从统一详情复制 API 示例或在线测试。

#### 优化

- 管理端统一模型服务目录，在模型详情管理规格、基础售价和分组开放；分组倍率与任务账务有独立入口。
- 模型配置应用前确认影响范围，已发布模型保持开放，暂停独立操作。
- 用户端使用模型卡片目录和统一 API 接入／在线测试详情，移除价格与限制面板及“我的视频任务”入口。
- 视频测试在当前窗口查询结果，不确定提交保留编号并阻止重复生成；后台任务与结算继续保留。

#### 修复

- H3 与 H3-Max 分别校验模型、分组和密钥允许的规格，避免将 H3 的 2K 或 4 秒选项用于 H3-Max。
- MiniMax 官方 M3 的优先处理按基础价格的 1.5 倍计费，并继续应用现有分组倍率。

[查看完整更新记录](docs/releases/release-notes.md)

## Quick start with Docker

Requirements: Docker Engine / Docker Desktop with Docker Compose v2.

```bash
git clone https://github.com/roby-uo/star-x.git
cd star-x/deploy
cp .env.example .env
# Edit .env and set the required secrets before the first start.
docker compose up -d
docker compose ps
```

Open `http://localhost:8080`. Use the administrator email and password you set in `.env`.

The default application image is:

```text
ghcr.io/roby-uo/star-x:latest
```

### Build from source instead

If you cloned the repository and prefer to build the exact checked-out source
on that machine, use the source-build Compose override:

```bash
git clone https://github.com/roby-uo/star-x.git
cd star-x/deploy
cp .env.example .env
# Edit .env and set the required secrets before the first start.
docker compose -f docker-compose.yml -f docker-compose.source.yml up -d --build
```

This creates `local/star-x:source` locally and uses the same PostgreSQL and
Redis services as the published-image deployment.

Data is stored in named Docker volumes (`star_x_data`, `postgres_data`, and `redis_data`). Updating the application image does not remove those volumes. For an existing deployment, back up PostgreSQL before changing configuration or upgrading.

## First-use chain

1. Sign in as the administrator.
2. Create a group.
3. Add only AI-provider accounts and API credentials you are authorized to use.
4. Create an API key for the desired group.
5. Call the compatible endpoint through your star-X URL.

An installed gateway cannot forward model requests until an authorized upstream account is configured.

## Development

Build the image locally from the repository root:

```bash
docker build -t local/star-x:dev .
```

For public releases, pushing to `main` builds `ghcr.io/roby-uo/star-x:latest`. Pushing a tag such as `v0.1.4` also produces a versioned image and a GitHub Release.

## Security notes

- Never commit `.env`, database dumps, API keys, OAuth tokens, or the `data/` directory.
- Set strong, unique `POSTGRES_PASSWORD`, `ADMIN_PASSWORD`, `JWT_SECRET`, and `TOTP_ENCRYPTION_KEY` values before first startup.
- Put the service behind HTTPS and restrict public access before using it outside a trusted network.
- Respect each upstream provider's terms, account rules, and applicable laws.

## License and attribution

star-X is a modified distribution of [Wei-Shaw/sub2api](https://github.com/Wei-Shaw/sub2api). It remains licensed under the [GNU Lesser General Public License v3.0 or later](LICENSE). The original license and applicable notices are retained.
