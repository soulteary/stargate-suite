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

## Preset vs compose path

| Preset | Compose path | Description |
|--------|--------------|-------------|
| `default` | `compose/example/image/docker-compose.yml` | Static example: pre-built images |
| `image` | `build/image/docker-compose.yml` | After gen: pre-built (run gen first) |
| `build` | `build/build/docker-compose.yml` | After gen: build from source |
| `traefik` | `build/traefik/docker-compose.yml` | After gen: Traefik all-in-one |
| `traefik-herald` | `build/traefik-herald/docker-compose.yml` | After gen: split, Herald + herald-totp + Redis only |
| `traefik-warden` | `build/traefik-warden/docker-compose.yml` | After gen: split, Warden only |
| `traefik-stargate` | `build/traefik-stargate/docker-compose.yml` | After gen: split, Stargate + protected service |

See [compose/README.md](../compose/README.md) for more.

## Usage

- **Default**: When unspecified, the path from `presets.json` key `default` is used. That is `compose/example/image/docker-compose.yml` (static example, no `gen` required). After you run `gen`, use `--preset image` or `-f build/image/docker-compose.yml` to use the generated build dir; the CLI default does not switch to `build/` automatically.
- **Environment**: `COMPOSE_FILE=<path>` overrides default (higher than default, lower than CLI).
- **CLI**:
  - `-f <path>` / `--file <path>`: explicit compose file path.
  - `--preset <name>`: use a preset from `presets.json` (e.g. `image`, `traefik`, `build`).

Priority: **CLI -f / --preset > COMPOSE_FILE > default (presets.default)**.

Examples:

```bash
./suite up

COMPOSE_FILE=build/traefik/docker-compose.yml ./suite up

./suite -f build/traefik/docker-compose.yml up

./suite --preset traefik up
```

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
