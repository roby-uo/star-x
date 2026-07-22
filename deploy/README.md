# star-X Docker deployment

star-X supports two Docker Compose deployment methods. Both create the same
application, PostgreSQL, and Redis services; the difference is only where the
application image comes from.

## Use the published image (recommended)

```bash
cp .env.example .env
# Edit every CHANGE_ME value.
docker compose pull
docker compose up -d
docker compose ps
```

This pulls `ghcr.io/roby-uo/star-x:latest`, which is published automatically
whenever the `main` branch is updated.

## Build from the cloned source

```bash
cp .env.example .env
# Edit every CHANGE_ME value.
docker compose -f docker-compose.yml -f docker-compose.source.yml up -d --build
docker compose -f docker-compose.yml -f docker-compose.source.yml ps
```

The source-build override uses the repository root Dockerfile and creates the
local image `local/star-x:source`. It does not require GitHub Container Registry
access.

For either method, inspect logs with `docker compose logs -f star-x`.

See the repository [README](../README.md) for first-use, security, and license information.
