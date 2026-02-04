English | [中文](README.zh-CN.md)

# Compose

Static examples + single canonical source. Output is generated into `build/` by CLI or Web UI. Run from project root. Overview: [../README.md](../README.md).

## Layout

| Dir | Purpose |
|-----|---------|
| example/image/ | Pre-built images — quick try, CI |
| example/build/ | Build from source — local dev, E2E |
| canonical/ | Single source → gen traefik + split (traefik-herald, traefik-warden, traefik-stargate) |
| traefik/ | Notes: [traefik/README.md](./traefik/README.md) |

**Generated (build/):** image, build, traefik, traefik-herald, traefik-warden, traefik-stargate.

## Usage

```bash
make gen   # or: go run ./cmd/suite gen all
make up    # default: build/image
# or: docker compose -f build/image/docker-compose.yml up -d
```

- **Pre-built:** `build/image/` → `docker compose -f build/image/docker-compose.yml up -d`
- **From source:** `build/build/` → `docker compose -f build/build/docker-compose.yml up -d --build`
- **Traefik:** `docker network create traefik` then `docker compose -f build/traefik/docker-compose.yml up -d`

Split is generated from canonical; after editing canonical run `go run ./cmd/suite gen traefik`.  
Web UI: `go run ./cmd/suite serve` → select type, download compose + .env.

**Env:** Root `.env` (or canonical) → each `build/<mode>/.env`. Common: `AUTH_HOST`, `STARGATE_DOMAIN`, `*_API_KEY`, `*_IMAGE`; optional DingTalk/SMTP/OwlMail — see root `.env.example`.

See [../README](../README.md) · [../config/README](../config/README.md) · [traefik/README](./traefik/README.md).
