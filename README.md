# cofiswarm-launcher

Stack launcher — **Docker Compose** instead of `posix_spawn` (`proxy_configure_spawn.cpp`).

- Migration: Sprint 7 in [MIGRATION-SPRINTS](https://github.com/keepdevops/cofiswarm-docs/blob/main/MIGRATION-SPRINTS.md)
- Legacy: `legacy/cpp/`, `internal/legacy/*.py`, `scripts/brewctl`

## Profile gate (8 GB)

```bash
make build
./bin/cofiswarm-launcher -compose-dir compose profile up 8gb
# dry-run (no docker):
./bin/cofiswarm-launcher -compose-dir compose -dry-run profile up 8gb
```

`8gb` profile starts the **nats** broker only; RAG is serverless (sqlite-vec, a local `.db` file — no container) and llama/MLX infer runs on host (Metal) via `cofiswarm-infer-*` profiles.

## Compose layout

```
compose/
├── docker-compose.yml      # base services + profile tags
└── profiles/8gb.yml        # 8 GB overlay
```

## Subcommands

| Command | Action |
|---------|--------|
| `profile up 8gb` | `docker compose --profile 8gb up -d` |
| `profile down 8gb` | `docker compose down` |
| `profile config 8gb` | validate merged compose |

## FHS

| Path | Purpose |
|------|---------|
| `/etc/cofiswarm/profiles/` | hardware profile metadata |
| `/var/lib/cofiswarm/launcher/` | launch state |
