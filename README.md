English | [中文](README.zh-CN.md)

# stargate-suite

End-to-end integration test environment for **Stargate + Warden + Herald**: Compose setups, Web UI for config generation, and 50+ E2E tests (normal flow, errors, auth, idempotency, audit, metrics). Optional: **herald-totp**, **herald-dingtalk**, **herald-smtp**.

Go module: `github.com/soulteary/the-gate`. Repo name: **stargate-suite**.

## Docs

| Doc | Description |
|-----|-------------|
| [README](README.md) | This file — overview, quick start |
| [SCENARIOS](SCENARIOS.md) | Scenario presets (`scene:<id>`) and usage |
| [compose/README](compose/README.md) | Compose usage; [中文](compose/README.zh-CN.md) |
| [config/README](config/README.md) | Web UI config; [中文](config/README.zh-CN.md) |
| [compose/traefik/README](compose/traefik/README.md) | Traefik all-in-one / split; [中文](compose/traefik/README.zh-CN.md) |
| [e2e/README](e2e/README.md) | E2E tests; [中文](e2e/README.zh-CN.md) |

## Structure

```
stargate-suite/
├── compose/example/   # optional; image | build generated from canonical
├── compose/canonical/ # single source → Web UI / make gen
├── build/             # generated (make gen via Web API or Web UI)
├── config/             # page.yaml, scenarios
├── cmd/suite/          # Web UI (serve) + validate
├── e2e/                # E2E tests
├── fixtures/warden/    # test users (data.json)
└── scripts/run-e2e.sh
```

## Quick start

**Prerequisites:** Docker & Compose, Go 1.25+, ~1GB disk.

**Generate then start:**

```bash
make gen    # generates into build/ via Web API (no CLI gen command)
make up
# or: make up-build | make up-traefik
```

**CLI:** `go run ./cmd/suite help` — `validate`, `serve`. Config generation is **Web UI only** (or `make gen` which calls the Web API).  
**Web UI:** `go run ./cmd/suite serve` (default http://localhost:8085). No auth — localhost only.

**Test:**

```bash
./scripts/run-e2e.sh
# or: make test-wait && go test -v ./e2e/...
```

**Stop:** `make down` (or `make clean` for volumes).

## Ports & env

- **Stargate** 8080 · **Warden** 8081 · **Herald** 8082 · **Redis** 6379
- Copy `.env.example` → `.env` to override image versions, `AUTH_HOST`, `PASSWORDS`, `WARDEN_API_KEY`, `HERALD_API_KEY`, `HERALD_HMAC_SECRET`.

## Test users (fixtures/warden/data.json)

| Role | Phone | Email | User ID |
|------|-------|-------|---------|
| Admin | 13800138000 | admin@example.com | test-admin-001 |
| User | 13900139000 | user@example.com | test-user-002 |
| Guest | 13700137000 | guest@example.com | test-guest-003 |
| Inactive | 13600136000 | inactive@example.com | test-inactive-004 |
| Rate-limit | 13500135000 | ratelimit@example.com | test-ratelimit-005 |

## Test suite

50+ cases: normal login flow, error scenarios (wrong/expired/locked code, non-whitelist, inactive, rate limits, service down, auth), Herald/Warden API, idempotency, audit, provider, metrics.  
Run one: `go test -v ./e2e/... -run TestCompleteLoginFlow`

## Makefile (see `make help`)

Common: `make gen` (via Web API), `make up` / `make up-image` / `make up-build` / `make up-traefik`, `make down`, `make ps`, `make logs`, `make test-wait`, `make health`, `make serve`, `make suite-build`.

## Services (brief)

- **Stargate:** forwardAuth, session, login flow. `GET /_auth`, `POST /_send_verify_code`, `POST /_login`
- **Warden:** whitelist user lookup. `GET /user?phone=...|mail=...|user_id=...`
- **Herald:** OTP challenge/verify/revoke, rate limits, audit. `POST /v1/otp/challenges`, `POST /v1/otp/verifications`, `GET /v1/test/code/{id}` (test mode)
- **herald-totp (optional):** TOTP 2FA. Set `HERALD_TOTP_ENABLED=true` in Stargate; configure Herald with `HERALD_TOTP_BASE_URL` and API key so Herald proxies to herald-totp.

Full login flow is covered by e2e tests; see [e2e/README](e2e/README.md).

## Troubleshooting

- **Won’t start:** `lsof -i :8080 -i :8081 -i :8082 -i :6379`, `make logs`, `make health`
- **Tests fail:** `make ps` and `make health`; `go test -v ./e2e/...`; rate limits — tests clear Redis; lockout — check Redis cleanup
- **No verification code:** `HERALD_TEST_MODE=true`, check Herald logs
- **Redis:** must be localhost:6379 for test cleanup; `redis-cli -h localhost -p 6379 ping`

## Dev

- Test data: edit `fixtures/warden/data.json`, `make restart-warden`
- New tests: add under `e2e/`, use `ensureServicesReady(t)` and `test_helpers.go`
- Local build: `make up-build`, then rebuild/restart
- Lint: `golangci-lint run --max-same-issues=100000`

## License

Same as main project.
