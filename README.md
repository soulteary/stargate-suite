English | [中文](README.zh-CN.md)

# stargate-suite — Three-Service End-to-End Integration Test Suite

This repository provides an end-to-end integration test environment for **Stargate + Warden + Herald**: multiple Compose setups, CLI orchestration, and automated tests covering normal flows, error scenarios, service-to-service auth, idempotency, audit, and metrics. The Web UI and canonical compose also support **herald-totp** (TOTP 2FA), **herald-dingtalk** (DingTalk channel), and **herald-smtp** (email/SMTP channel) as optional services.

The Go module is `github.com/soulteary/the-gate`; the repo and product name is **stargate-suite**.

## Documentation

| Document | Description |
|----------|-------------|
| [README.md](./README.md) (this file) | Overview, quick start, services, troubleshooting |
| [compose/README.md](./compose/README.md) | Compose usage (build / image / traefik); Chinese: [README.zh-CN.md](./compose/README.zh-CN.md) |
| [config/README.md](./config/README.md) | CLI presets and `-f` / `--preset`; Chinese: [README.zh-CN.md](./config/README.zh-CN.md) |
| [compose/traefik/README.md](./compose/traefik/README.md) | Traefik all-in-one and split deployment; Chinese: [README.zh-CN.md](./compose/traefik/README.zh-CN.md) |
| [e2e/README.md](./e2e/README.md) | E2E test cases and how to run; Chinese: [README.zh-CN.md](./e2e/README.zh-CN.md) |
| [MANUAL_TESTING.md](./MANUAL_TESTING.md) | Browser manual verification and health checks; Chinese: [MANUAL_TESTING.zh-CN.md](./MANUAL_TESTING.zh-CN.md) |

## Project structure

```
stargate-suite/
├── compose/                # Compose sources and examples
│   ├── README.md
│   ├── example/            # Static examples
│   │   ├── image/          # Pre-built images
│   │   │   └── docker-compose.yml
│   │   └── build/          # Build from source
│   │       └── docker-compose.yml
│   ├── canonical/          # Single source (generates traefik / split)
│   │   └── docker-compose.yml
│   └── traefik/            # Traefik deployment notes
│       └── README.md
├── build/                  # Generated output (gen or Web UI; not committed by default)
│   ├── image/              # From example/image + .env
│   ├── build/              # From example/build + .env
│   ├── traefik/            # From canonical
│   ├── traefik-herald/
│   ├── traefik-warden/
│   └── traefik-stargate/
├── go.mod
├── go.sum
├── Makefile
├── config/                 # CLI presets (presets.json), Web UI (page.yaml)
├── cmd/suite/              # Go CLI + Web UI
│   ├── main.go
│   ├── compose_split.go
│   └── static/index.html.tmpl
├── internal/composegen/
├── README.md
├── LICENSE
├── e2e/                    # E2E tests
│   ├── e2e_test.go
│   ├── error_scenarios_test.go
│   ├── auth_test.go
│   ├── herald_api_test.go
│   ├── warden_api_test.go
│   ├── idempotency_test.go
│   ├── audit_test.go
│   ├── provider_test.go
│   ├── metrics_test.go
│   ├── test_helpers.go
│   └── README.md
├── fixtures/
│   └── warden/
│       └── data.json
└── scripts/
    └── run-e2e.sh
```

Root docs: `README.md` (this file), `README.zh-CN.md` (Chinese). Same pattern in `compose/`, `config/`, `e2e/` and `MANUAL_TESTING.md` / `MANUAL_TESTING.zh-CN.md`.

## Quick start

### Prerequisites

- Docker and Docker Compose
- Go 1.25+ (see `go.mod`; use this version or newer for builds)
- ~1GB disk (Docker images and volumes)

### Default compose and presets

- **Before running `gen`**: The default compose path is `compose/example/image/docker-compose.yml` (or use `--preset default`). You can start services from this static example.
- **After running `make gen` or `go run ./cmd/suite gen all`**: Prefer `build/image/docker-compose.yml` for day-to-day use. Use `--preset image` or `-f build/image/docker-compose.yml` to select it. Do not assume the default is `build/` until you have run `gen` at least once.

See [config/README.md](./config/README.md) for the full preset list and override order.

### Start services

**First run: generate config into `build/`:**

```bash
make gen
# or
go run ./cmd/suite gen all
```

Then start:

```bash
# Option 1: Makefile (default: build/image)
make up

# Other compose targets
make up-image    # Pre-built (build/image)
make up-build    # Build from source (build/build)
make up-traefik  # With Traefik (build/traefik)

# Option 2: Direct compose file (from project root)
docker compose -f build/image/docker-compose.yml up -d
docker compose -f build/build/docker-compose.yml up -d --build
docker compose -f build/traefik/docker-compose.yml up -d

# Status and logs
make ps
make logs
# or
docker compose -f build/image/docker-compose.yml ps
docker compose -f build/image/docker-compose.yml logs -f
```

**Option 3: Go CLI (same as Makefile, cross-platform)**

```bash
go run ./cmd/suite help
make suite-build && ./bin/suite help

make suite ARGS="up"
make suite ARGS="health"

go run ./cmd/suite up
go run ./cmd/suite test-wait
go run ./cmd/suite health
```

Use `COMPOSE_FILE` to override the default compose file.

**Generate build dir (compose + .env)**

```bash
go run ./cmd/suite gen [mode]     # mode default: all
go run ./cmd/suite gen image      # copy example/image → build/image/
go run ./cmd/suite gen build      # copy example/build → build/build/
go run ./cmd/suite gen traefik    # from canonical → build/traefik, traefik-herald, etc.
go run ./cmd/suite gen all        # all of the above

go run ./cmd/suite -o dist gen traefik
GEN_OUT_DIR=dist go run ./cmd/suite gen all
```

Example after gen: `docker compose -f build/image/docker-compose.yml --env-file build/image/.env up -d`. See [compose/README.md](./compose/README.md).

**Web UI**

From project root: `go run ./cmd/suite serve` (default http://localhost:8085). Use `-port` or `SERVE_PORT` to change port.

- **Security**: The Web UI and `/api/generate` have no authentication. Use only on localhost or in a trusted environment; do not expose to the public internet.

### Run tests

After services are ready (~30s or use `make test-wait`):

```bash
make test-wait

go test -v ./e2e/...

./scripts/run-e2e.sh

go test -v ./e2e/... -run TestCompleteLoginFlow
```

### Stop services

```bash
make down
# or
docker compose -f build/image/docker-compose.yml down

make clean   # down + remove volumes
docker compose -f build/image/docker-compose.yml down -v
```

## Service configuration

### Ports

- **Stargate**: http://localhost:8080
- **Warden**: http://localhost:8081
- **Herald**: http://localhost:8082
- **Herald Redis**: localhost:6379 (for test cleanup)

### Environment variables

Optional: copy `.env.example` to `.env` in the project root to override image versions and keys before `make gen`. Override via `.env` or environment. Main options:

- `AUTH_HOST`: Stargate auth host (default: `auth.test.localhost`)
- `PASSWORDS`: Stargate password config (default: `plaintext:test1234|test1337`)
- `WARDEN_API_KEY`, `HERALD_API_KEY`, `HERALD_HMAC_SECRET`: API / HMAC keys (defaults for test)

### Test users

Defined in `fixtures/warden/data.json`:

1. **Admin** (`13800138000`) — admin@example.com, `test-admin-001`, scopes `read,write,admin`, role `admin`
2. **User** (`13900139000`) — user@example.com, `test-user-002`, scope `read`, role `user`
3. **Guest** (`13700137000`) — guest@example.com, `test-guest-003`, scope `read`, role `guest`
4. **Inactive** (`13600136000`) — inactive@example.com, `test-inactive-004`, status `inactive`
5. **Rate-limit test** (`13500135000`) — ratelimit@example.com, `test-ratelimit-005`

## Test suite

**50+ test cases**, including:

- **Normal flow**: `TestCompleteLoginFlow` (send code → get code → login → verify auth headers)
- **Error scenarios** (`error_scenarios_test.go`): wrong/expired/locked code, non-whitelist/inactive user, rate limits, service down, auth errors, edge cases
- **Service auth** (`auth_test.go`): Herald HMAC, Warden/Herald API keys
- **Herald API** (`herald_api_test.go`): challenges, verify, revoke, rate limit, HMAC
- **Warden API** (`warden_api_test.go`): user lookup by phone/email/user_id, API key
- **Idempotency** (`idempotency_test.go`): same/different Idempotency-Key
- **Audit** (`audit_test.go`), **Provider** (`provider_test.go`), **Metrics** (`metrics_test.go`)

**Isolation**: auto-clear rate-limit/challenge state in Redis, separate test users, service readiness checks.

**Run specific tests:**

```bash
go test -v ./e2e/... -run TestCompleteLoginFlow
go test -v ./e2e/... -run TestInvalid
go test -v ./e2e/... -run TestHeraldHMAC
go test -v ./e2e/... -run TestRateLimit
go test -v ./e2e/... -run TestHeraldUnavailable
go test -v ./e2e/... -run TestWardenUnavailable
```

## Makefile commands

Default compose: `build/image` (override with `COMPOSE_FILE`).

```bash
make help
make gen                  # Generate compose + .env into build/
make up
make up-build
make up-image
make up-traefik
make up-traefik-herald    # Split: Herald only
make up-traefik-warden    # Split: Warden only
make up-traefik-stargate  # Split: Stargate + protected service
make down
make down-build
make down-image
make down-traefik
make down-traefik-herald
make down-traefik-warden
make down-traefik-stargate
make net-traefik-split    # Create networks for split (run once)
make logs
make ps
make test
make test-wait
make clean
make restart
make restart-warden
make restart-herald
make restart-stargate
make health
make suite ARGS="..."     # Run CLI (e.g. make suite ARGS="up")
make suite-build          # Build bin/suite
make serve                # Web UI (default :8085)
```

## Services

### Stargate (gate)

Traefik forwardAuth service: session (Cookie/JWT), login flow (Warden + Herald send/verify code), forwardAuth check.

- `GET /_auth` — forwardAuth check
- `POST /_send_verify_code` — send code
- `POST /_login` — login

### Warden (warden)

Whitelist user service: user info (email/phone/user_id/status/scope/role), active/inactive.

- `GET /user?phone=...`, `?mail=...`, `?user_id=...`
- `GET /metrics`

### Herald (herald)

OTP/verification: create challenge, verify, revoke; rate limits; audit; SMS/Email providers. Optional DingTalk channel (herald-dingtalk): Herald calls it over HTTP; DingTalk credentials live only in herald-dingtalk.

- `POST /v1/otp/challenges` — create and send
- `POST /v1/otp/verifications` — verify
- `POST /v1/otp/challenges/{id}/revoke`
- `GET /v1/test/code/{challenge_id}` — test-only, get code when `HERALD_TEST_MODE=true`
- `GET /metrics`, `GET /healthz`

### Herald TOTP (herald-totp, optional)

TOTP 2FA: enroll (bind), verify, revoke; backup codes. Stargate calls herald-totp for per-user TOTP; users generate codes in an authenticator app (e.g. Google Authenticator). Enable via `HERALD_TOTP_ENABLED=true` and set `HERALD_TOTP_BASE_URL`, `HERALD_TOTP_API_KEY` in Stargate env.

- `POST /v1/enroll/start` — start enrollment (returns QR / otpauth_uri)
- `POST /v1/enroll/confirm` — confirm with one TOTP code
- `POST /v1/verify` — verify TOTP or backup code
- `POST /v1/revoke` — remove TOTP and backup codes
- `GET /v1/status?subject=...` — check if TOTP enabled
- `GET /healthz`

## Example: full login flow

1. **Send code**: `POST http://localhost:8080/_send_verify_code`, body `phone=13800138000` → `challenge_id`, `expires_in`
2. **Get code** (test): `GET http://localhost:8082/v1/test/code/{challenge_id}`, header `X-API-Key: test-herald-api-key` → `code`
3. **Login**: `POST http://localhost:8080/_login`, body `auth_method=warden&phone=...&challenge_id=...&verify_code=...` → `Set-Cookie: stargate_session_id=...`
4. **Auth**: `GET http://localhost:8080/_auth` with cookie → 200 and headers `X-Auth-User`, `X-Auth-Email`, `X-Auth-Scopes`, `X-Auth-Role`

## Troubleshooting

**Services won’t start**

- Check ports: `lsof -i :8080 -i :8081 -i :8082 -i :6379`
- Logs: `make logs` or `docker compose -f build/image/docker-compose.yml logs stargate|warden|herald`
- Health: `make health`

**Tests fail**

- Ensure services are up: `make ps` and `make health`
- Curl: `curl http://localhost:8080/_auth`, `.../8081/health`, `.../8082/healthz`
- Verbose: `go test -v ./e2e/...`
- Rate limit: tests clear Redis state; if still hitting limits, increase delay or adjust Herald config
- Lockout: tests clear lock keys; check Redis and cleanup

**Can’t get verification code**

- Ensure `HERALD_TEST_MODE=true` and check Herald logs for test endpoint.

**Redis**

- Herald Redis must be on `localhost:6379` for test cleanup. Check: `docker compose -f build/image/docker-compose.yml ps herald-redis`, `redis-cli -h localhost -p 6379 ping`

## Development

- **Test data**: edit `fixtures/warden/data.json`, then `make restart-warden`
- **New tests**: add `*_test.go` under `e2e/`, use `ensureServicesReady(t)` and helpers from `test_helpers.go`
- **Local code changes**: use `make up-build` (build from source), then rebuild and restart the service
- **Lint**: `golangci-lint run --max-same-issues=100000`

## See also

- [compose/README.md](./compose/README.md) — Compose usage
- [config/README.md](./config/README.md) — CLI presets and compose path
- [compose/traefik/README.md](./compose/traefik/README.md) — Traefik all-in-one / split
- [e2e/README.md](./e2e/README.md) — E2E test details
- [MANUAL_TESTING.md](./MANUAL_TESTING.md) — Manual browser verification

## License

Same as the main project.
