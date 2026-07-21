# star-X

> Self-hosted AI API gateway for teams and individuals.

star-X is a self-hosted, privately branded distribution for managing authorized upstream AI accounts, API keys, routing, quotas, and usage. It is designed to run under your control with Docker Compose.

![star-X logo](frontend/public/brand/star-x-logo-original.png)

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
