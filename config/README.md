English | [中文](README.zh-CN.md)

# Config directory

This directory holds CLI preset config (`presets.json`) and Web UI page config. Project overview: [README.md](../README.md).

## Page config (Web UI)

The compose generator page is driven by YAML under `config/`. It is split by concern for easier maintenance:

| File | Purpose |
|------|--------|
| `page.yaml` | Entry: compose types (modes) and this index. |
| `config-sections.yaml` | Options: image versions, health check, Traefik network, ports, Redis storage. |
| `i18n/zh.yaml` | Chinese copy. |
| `i18n/en.yaml` | English copy. |
| `services.yaml` | Stargate / Warden / Herald / herald-totp env vars. |
| `providers.yaml` | Herald channels (e.g. herald-dingtalk). |

`serve` loads `page.yaml` first, then merges the above fragments when present. A single monolithic `page.yaml` (with all keys) still works for backward compatibility.

## Preset vs compose path (Makefile / docker compose)

The Makefile and `scripts/run-e2e.sh` use `COMPOSE_FILE` (default `build/image/docker-compose.yml`). Presets in `presets.json` map preset names to compose paths; use them with `COMPOSE_FILE` or `make up-*`:

| Preset | Compose path | Description |
|--------|--------------|-------------|
| `default` | `compose/example/image/docker-compose.yml` | Static example: pre-built images |
| `image` | `build/image/docker-compose.yml` | After gen: pre-built (run gen first) |
| `build` | `build/build/docker-compose.yml` | After gen: build from source |
| `traefik` | `build/traefik/docker-compose.yml` | After gen: Traefik all-in-one |
| `traefik-herald` | `build/traefik-herald/docker-compose.yml` | After gen: split, Herald only |
| `traefik-warden` | `build/traefik-warden/docker-compose.yml` | After gen: split, Warden only |
| `traefik-stargate` | `build/traefik-stargate/docker-compose.yml` | After gen: split, Stargate + protected service |

Examples: `make up`, `COMPOSE_FILE=build/traefik/docker-compose.yml make up`, or `docker compose -f build/image/docker-compose.yml up -d`. See [compose/README.md](../compose/README.md).

## Generate build dir (gen)

Output `docker-compose.yml` and `.env` for each mode to a directory (default `build`):

```bash
./suite gen [image|build|traefik|all]   # default: all
./suite -o dist gen traefik             # output to dist/
```

## Web UI (serve)

Start the web generator and download config:

```bash
./suite serve
# default http://localhost:8085; use -port or SERVE_PORT to change
```

## See also

- [README.md](../README.md) — Project overview
- [compose/README.md](../compose/README.md) — Compose examples and generation
