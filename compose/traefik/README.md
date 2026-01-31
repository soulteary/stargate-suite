English | [中文](README.zh-CN.md)

# Traefik integration

This section describes **all-in-one** deployment; **split** deployment uses the generated dirs `build/traefik-herald/`, `build/traefik-warden/`, `build/traefik-stargate/`. Compose overview: [../README.md](../README.md), project overview: [../../README.md](../../README.md).

## Option 1: All-in-one (build/traefik)

Stargate, Warden, Herald and the sample protected service in one compose; good for local one-shot run.

**File**: after generation use `build/traefik/docker-compose.yml`.

**Prerequisite**: Create Traefik network if needed

```bash
docker network create traefik
```

**Start** (from project root):

```bash
docker compose -f build/traefik/docker-compose.yml up -d
# or
make up-traefik
```

**Stop**:

```bash
docker compose -f build/traefik/docker-compose.yml down
# or
make down-traefik
```

---

## Option 2: Split (separate compose dirs)

Split compose files are **generated** from canonical into `build/`. After editing `compose/canonical/docker-compose.yml`, run `go run ./cmd/suite gen-split` or `gen traefik` to regenerate `build/traefik-herald/`, `build/traefik-warden/`, `build/traefik-stargate/`; do not edit those by hand.

| Dir | Contents |
|-----|----------|
| build/traefik-herald/ | Herald + herald-redis |
| build/traefik-warden/ | Warden + warden-redis |
| build/traefik-stargate/ | Stargate + protected-service (whoami) |

**Prerequisite**: Create shared networks (once)

```bash
docker network create the-gate-network
docker network create traefik
# or
make net-traefik-split
```

**Start order**: Herald → Warden → Stargate (Stargate depends on the other two)

```bash
docker compose -f build/traefik-herald/docker-compose.yml up -d
docker compose -f build/traefik-warden/docker-compose.yml up -d
docker compose -f build/traefik-stargate/docker-compose.yml up -d
```

Or with Makefile:

```bash
make up-traefik-herald
make up-traefik-warden
make up-traefik-stargate
```

**Stop**: in reverse order if desired

```bash
make down-traefik-stargate
make down-traefik-warden
make down-traefik-herald
```

Split mode uses container names for Warden/Herald (`the-gate-warden:8081`, `the-gate-herald:8082`); all three must join the same external network `the-gate-network`.

---

## Environment variables

Both modes support overrides via `.env` or environment, e.g.:

- `STARGATE_DOMAIN`, `PROTECTED_DOMAIN`, `AUTH_HOST`
- `HERALD_API_KEY`, `HERALD_HMAC_SECRET`, `WARDEN_API_KEY`
- `*_IMAGE`, `*_REDIS_IMAGE`

See root `.env.example` or `.env` for examples.

## See also

- [../README.md](../README.md) — Compose directory layout
- [../../README.md](../../README.md) — Project overview
