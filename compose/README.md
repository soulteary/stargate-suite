English | [中文](README.zh-CN.md)

# Compose examples and generation

This directory keeps only **static examples** and a **single canonical source**. All other compose files are generated into `build/` by the CLI or Web UI. Run all commands from the project root `stargate-suite`. Overview: [README.md](../README.md).

## Directory layout

| Directory | Description |
|-----------|-------------|
| **example/image/** | Static: run with pre-built images; good for quick try and CI |
| **example/build/** | Static: build Stargate, Warden, Herald from source; good for local dev and E2E |
| **canonical/** | Single source: full Traefik all-in-one compose; used to generate traefik / traefik-herald / traefik-warden / traefik-stargate |
| **traefik/** | Optional: Traefik all-in-one and split deployment notes — [traefik/README.md](./traefik/README.md) |

**Generated output** goes to `build/` (via `go run ./cmd/suite gen all` or Web UI):

| Output | Description | Start command |
|--------|-------------|---------------|
| build/image/ | From example/image + .env | `docker compose -f build/image/docker-compose.yml up -d` |
| build/build/ | From example/build + .env | `docker compose -f build/build/docker-compose.yml up -d --build` |
| build/traefik/ | All-in-one with Traefik (Stargate, Warden, Herald, herald-totp, herald-dingtalk, herald-smtp, Redis) | `docker compose -f build/traefik/docker-compose.yml up -d` |
| build/traefik-herald/ | Split: Herald + herald-totp + herald-smtp + Redis only | `docker compose -f build/traefik-herald/docker-compose.yml up -d` |
| build/traefik-warden/ | Split: Warden + Redis only | `docker compose -f build/traefik-warden/docker-compose.yml up -d` |
| build/traefik-stargate/ | Split: Stargate + protected service (Herald/Warden/herald-totp must be up if enabled) | `docker compose -f build/traefik-stargate/docker-compose.yml up -d` |

## Usage

### First run: generate into build/

```bash
# From project root
go run ./cmd/suite gen all
# or
make gen
```

### Start from pre-built images (build/image)

```bash
docker compose -f build/image/docker-compose.yml up -d
```

### Start from source build (build/build)

Requires `herald`, `warden`, `stargate` and `stargate-suite` at the same level.

```bash
docker compose -f build/build/docker-compose.yml up -d --build
```

### With Traefik (build/traefik)

1. Create Traefik network: `docker network create traefik`
2. Ensure Traefik is running
3. Start: `docker compose -f build/traefik/docker-compose.yml up -d`

You can set `STARGATE_DOMAIN`, `PROTECTED_DOMAIN`, etc. in each `build/<mode>/.env`.

## Split vs single source

- **canonical** (`compose/canonical/docker-compose.yml`) is the only maintained “full Traefik” compose.
- **Split** (traefik-herald / traefik-warden / traefik-stargate) is **generated** from canonical into `build/`; do not edit by hand.
- After changing canonical, run `go run ./cmd/suite gen traefik` or `go run ./cmd/suite gen-split` to regenerate.

## Web UI

Run `go run ./cmd/suite serve` (default http://localhost:8085), select compose type(s), then download `docker-compose.yml` and `.env`.

## Environment and .env

Generation writes root `.env` (if present) or variables inferred from canonical into each `build/<mode>/.env`. Common variables:

- `AUTH_HOST`, `STARGATE_DOMAIN`, `PROTECTED_DOMAIN`
- `HERALD_API_KEY`, `HERALD_HMAC_SECRET`, `WARDEN_API_KEY`
- `*_IMAGE`: override default images
- DingTalk channel (optional): `HERALD_DINGTALK_*`, `DINGTALK_*` (including `DINGTALK_LOOKUP_MODE`)
- Email channel (optional): when "Enable SMTP channel" is checked, generated compose includes **herald-smtp**; Herald calls it via `HERALD_SMTP_API_URL` to send email verification codes. Configure `SMTP_HOST`, `SMTP_FROM`, and other SMTP vars in .env or the Web UI. See [herald-smtp](https://github.com/soulteary/herald-smtp) for details. When "Use OwlMail for testing" is also checked, the compose includes **OwlMail** and herald-smtp uses its SMTP; view test emails at http://localhost:1080 (or the port set by **OwlMail Web host port** / `PORT_OWLMAIL`).

CLI generation respects env vars: `SMTP_ENABLED`, `SMTP_USE_OWLMAIL`, `PORT_OWLMAIL`, `DINGTALK_ENABLED`, `TOTP_ENABLED` (e.g. `SMTP_ENABLED=1 SMTP_USE_OWLMAIL=1 go run ./cmd/suite gen traefik`).

## See also

- [README.md](../README.md) — Project overview and quick start
- [config/README.md](../config/README.md) — CLI presets and compose path
- [traefik/README.md](./traefik/README.md) — Traefik all-in-one / split details
