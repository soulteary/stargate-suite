English | [中文](README.zh-CN.md)

# Traefik

**All-in-one:** `build/traefik/docker-compose.yml`. **Split:** generated `build/traefik-herald/`, `build/traefik-warden/`, `build/traefik-stargate/`. Compose: [../README.md](../README.md), project: [../../README.md](../../README.md).

## All-in-one (build/traefik)

```bash
docker network create traefik
docker compose -f build/traefik/docker-compose.yml up -d
# or make up-traefik
```

Stop: `make down-traefik` or `docker compose -f build/traefik/docker-compose.yml down`.

## Split

Generated from canonical; do not edit by hand. After changing canonical: `go run ./cmd/suite gen traefik`.

| Dir | Contents |
|-----|----------|
| build/traefik-herald/ | Herald + herald-redis |
| build/traefik-warden/ | Warden + warden-redis |
| build/traefik-stargate/ | Stargate + whoami |

```bash
make net-traefik-split   # once: create the-gate-network, traefik
make up-traefik-herald && make up-traefik-warden && make up-traefik-stargate
# stop: make down-traefik-stargate, down-traefik-warden, down-traefik-herald
```

Split uses container names (the-gate-warden:8081, the-gate-herald:8082) on shared network `the-gate-network`.

**Env:** .env or env vars — STARGATE_DOMAIN, PROTECTED_DOMAIN, AUTH_HOST, *_API_KEY, *_IMAGE. See root .env.example.

See [../README](../README.md) · [../../README](../../README.md).
