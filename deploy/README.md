# star-X Docker deployment

The supported public deployment method is Docker Compose.

```bash
cp .env.example .env
# Edit every CHANGE_ME value.
docker compose up -d
docker compose ps
docker compose logs -f star-x
```

See the repository [README](../README.md) for first-use, security, and license information.
