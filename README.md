# star-X

> Self-hosted AI API gateway for teams and individuals.

star-X is a self-hosted, privately branded distribution for managing authorized upstream AI accounts, API keys, routing, quotas, and usage. It is designed to run under your control with Docker Compose.

![star-X logo](frontend/public/brand/star-x-logo-original.jpg)

## 最新更新

### 2026-10-12

#### 新增

- 管理端新增简化计费管理，在一页查看模型人民币基准价、售价、来源和待确认规格，并提供不调用模型、不扣费的算费预览。
- 按量 API 使用基准价 × 统一倍率，默认 1.4；授权账号和包月订阅使用参考价 × 分组倍率，每次只应用一条售价规则。

#### 优化

- 用户使用记录、仪表盘和密钥用量展示请求数与真实 token 消耗，记录优先显示，图表按需展开。
- 用户端移除费用明细、成本、售价及倍率，保留余额、充值购买金额与个人额度管理。
- 站内金额统一人民币，美元基准价固定按 6.2 换算；旧余额保留原有购买力，新人民币充值默认支付 ¥100、到账 ¥100。
- 官方维护价表按供应商、地区和规格匹配；第三方单价与合同折扣按需登记，详细采购设置折叠展示。
- 模型目录明确区分 Agent 主模型候选与图片、视频工具接入；客户端导入过滤已识别的生成型号。

#### 修复

- 更新 Go 1.26.9 和相关安全依赖，升级到 SheetJS 官方修复版本；管理端极小费用不再显示成零。
- 用户接口及导出同步移除财务字段，管理员继续查看完整费用；未知采购成本明确标为待核对。
- 采购配置保存检查版本冲突，算费输入变更后旧结果失效，编辑有效期保留原始精度。
- 用户图表悬浮提示隐藏费用并兼容缺失费用字段；生成模型误用对话接口时提前提示正确接入方式。
- 未确认价格的模型或规格在付费调用前拒绝；历史支付退款、旧套餐和已提交媒体任务保持兼容，修正充值与低价用量的换算精度。

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
