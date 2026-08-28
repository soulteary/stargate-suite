English | [中文](README.zh-CN.md)

# stargate-suite

End-to-end integration test environment for **Stargate + Warden + Herald**: Compose setups, Web UI for config generation, and 50+ E2E tests (normal flow, errors, auth, idempotency, audit, metrics). Optional: **herald-totp**, **herald-dingtalk**, **herald-smtp**.

Go module: `github.com/soulteary/stargate-suite`. Repo name: **stargate-suite**.

> **Module rename:** the Go module path changed from `github.com/soulteary/the-gate` to `github.com/soulteary/stargate-suite`. This module hosts an internal tool (`cmd/suite`) and is not intended as an importable library, but if any external code imported the old path, update its imports to the new path.

## Docs

| Doc | Description |
|-----|-------------|
| [README](README.md) | This file — overview, quick start |
| [SCENARIOS](SCENARIOS.md) | Scenario presets (`scene:<id>`) and usage |
| [compose/README](compose/README.md) | Compose usage; [中文](compose/README.zh-CN.md) |
| [config/README](config/README.md) | Web UI config; [中文](config/README.zh-CN.md) |
| [compose/traefik/README](compose/traefik/README.md) | Traefik all-in-one / split; [中文](compose/traefik/README.zh-CN.md) |
| [e2e/README](e2e/README.md) | E2E tests; [中文](e2e/README.zh-CN.md) |
| [docs/migration-v0.10](docs/migration-v0.10.md) | v0.9 → v0.10 breaking-change migration |
| [docs/deployment](docs/deployment.md) | Deployment & profiles |
| [docs/security](docs/security.md) | Security model & hardening |
| [docs/development](docs/development.md) | Local development & testing |
| [docs/release](docs/release.md) | Release process & supply chain |

## Structure

```
stargate-suite/
├── compose/example/   # optional; image | build generated from canonical
├── compose/canonical/ # single source → CLI / Web UI / make gen
├── build/             # generated (make gen via CLI, or CLI / Web UI)
├── config/             # page.yaml, scenarios
├── cmd/suite/          # Web UI (serve) + generate + validate + doctor
├── e2e/                # E2E tests
├── fixtures/warden/    # test users (data.json)
└── scripts/run-e2e.sh
```

## Quick start

**Prerequisites:** Docker & Compose, Go 1.27+, ~1GB disk.

**Generate then start:**

```bash
make gen    # generates into build/ natively via the CLI (no Web server, no jq)
make up
# or: make up-build | make up-traefik
```

**CLI:** `go run ./cmd/suite help` — `generate`, `validate`, `doctor`, `serve`. The default config and canonical compose are **embedded in the binary** (`go:embed`), so all subcommands run without the repo source tree (e.g. from a release binary or the container). Use `--config-dir=<dir>` (or `CONFIG_DIR`) to override the embedded `config/` with an on-disk directory; anything missing there falls back to the embedded assets. `generate` and `validate` call the **same** `internal/composegen` / `internal/policy` functions as the Web UI's `/api/generate`, so the CLI produces byte-identical output without ever starting a Web server.

**Deployment profiles** (`config/profiles.yaml`): `development` / `test` / `production` are **security & runtime policy**, not just prefilled forms — they set port binding, secret source, password algorithm, Herald auth, Redis password, Cookie Secure, HMAC v1, container privileges and validation mode (see [docs/upgrade/00-overview.md](docs/upgrade/00-overview.md) §5.4). The CLI and Web UI share one model (`internal/policy` + `internal/composegen`):

```bash
# development: current defaults (loopback ports, plaintext test password, dev keys).
# --seed makes auto-generated dev keys byte-stable (dev/test only, never a real seed).
go run ./cmd/suite generate --profile development --output build/dev --seed pr5-golden

# production is experimental and STRICT: plaintext passwords, test/placeholder keys,
# published internal ports, Cookie Secure off, or HMAC v1 are hard errors (never bypassable).
# Supply real secrets via the process env or repeated --set KEY=VALUE:
go run ./cmd/suite generate --profile production --output build/prod \
  --set PASSWORDS=bcrypt:... --set HERALD_API_KEY=... --set WARDEN_API_KEY=... \
  --set HERALD_HMAC_SECRET=... --set HERALD_REDIS_PASSWORD=... --set WARDEN_REDIS_PASSWORD=...

# validate a profile's policy; production/test are strict (errors, not warnings):
go run ./cmd/suite validate --profile production --strict

# read-only diagnostics for a generated compose (parse, image↔manifest drift,
# published ports & local port usage, networks; --probe adds liveness/readiness):
go run ./cmd/suite doctor --compose build/dev/docker-compose.yml

# every subcommand supports --json for CI / Cursor; exit codes are stable
# (0 ok, non-zero on validation failure or doctor hard failure).
```

Config generation is also available via the **Web UI** (first step selects the profile) or `make gen` (native CLI, no Web server).

**Web UI:** `go run ./cmd/suite serve` binds **`127.0.0.1:8085` by default** (loopback only, no auth needed locally). Exposing it off-host is opt-in and always authenticated: `serve --listen 0.0.0.0:8085 --allow-remote` refuses to start without `--allow-remote` and, in remote mode, requires an access token (auto-generated and printed if you don't pass `--token`). State-changing POSTs are Origin/CSRF-checked, cookies are HttpOnly + SameSite=Strict (Secure off loopback), and operator secrets are dropped from the server session after the artifacts are returned. The listener never silently switches ports — a busy port is a hard error.

**Container (self-contained, no source mount):**

```bash
docker build -t stargate-suite:local .
docker run --rm -p 8085:8085 stargate-suite:local        # Web UI, no repo mount
docker run --rm --read-only --tmpfs /tmp -p 8085:8085 stargate-suite:local  # read-only root fs
```

**Test:**

```bash
./scripts/run-e2e.sh
# or: make test-wait && go test -v ./e2e/...
```

**Stop:** `make down` (or `make clean` for volumes).

## Ports & env

- **Stargate**: no host port — the `stargate` service uses `ports: []` and listens on backend port **8080** inside the container (health: `/healthz` liveness, `/readyz` readiness); it is reachable only via Traefik (see `compose/canonical/docker-compose.yml` and `config/ports.yaml`).
- **Warden** 8081 (health `/healthcheck`) · **Herald** 8082 (`/healthz`) · **Herald-TOTP** 8084 · **Herald-DingTalk** 8083 · **Herald-SMTP** 8085 · **Redis** 6379 (host ports only when the port is exposed / mapped). Component versions, ports and health paths come from `config/components.yaml` (single source of truth) — current pinned combo: Stargate `v1.0.0`, Warden `v1.0.0`, Herald `v1.1.0`.
- **Web UI** defaults to **8085** (`make serve`), which is the same default port as **herald-smtp**. The default scenarios do not run herald-smtp, so there is no conflict out of the box; but if you enable herald-smtp and also run `make serve` on the same host, the ports collide — change one of them (e.g. `SERVE_PORT` for the Web UI or the herald-smtp host port).
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

Common: `make gen` (native CLI), `make up` / `make up-image` / `make up-build` / `make up-traefik`, `make down`, `make ps`, `make logs`, `make test-wait`, `make health`, `make serve`, `make suite-build`.

## Services (brief)

- **Stargate:** forwardAuth, session, login flow. `GET /_auth`, `POST /_send_verify_code`, `POST /_login`
- **Warden:** whitelist user lookup. `GET /user?phone=...|mail=...|user_id=...`
- **Herald:** OTP challenge/verify/revoke, rate limits, audit. `POST /v1/otp/challenges`, `POST /v1/otp/verifications`, `GET /v1/test/code/{id}` (test mode)
- **herald-totp (optional):** TOTP 2FA. Set `HERALD_TOTP_ENABLED=true` in Stargate; configure Herald with `HERALD_TOTP_BASE_URL` and API key so Herald proxies to herald-totp.

Full login flow is covered by e2e tests; see [e2e/README](e2e/README.md).

## Troubleshooting

- **Won’t start:** `lsof -i :8081 -i :8082 -i :6379` (Stargate has no host port — check it via `make health`), `make logs`, `make health`
- **Tests fail:** `make ps` and `make health`; `go test -v ./e2e/...`; rate limits — tests clear Redis; lockout — check Redis cleanup
- **No verification code:** `HERALD_TEST_MODE=true`, check Herald logs
- **Redis:** must be localhost:6379 for test cleanup; `redis-cli -h localhost -p 6379 ping`

## Dev

- Test data: edit `fixtures/warden/data.json`, `make restart-warden`
- New tests: add under `e2e/`, use `ensureServicesReady(t)` and `test_helpers.go`
- Local build: `make up-build`, then rebuild/restart
- Lint: `golangci-lint run --max-same-issues=100000`

## Releases & supply chain

CI is split into tiers: `ci.yml` (fast PR feedback), `main.yml` (full E2E +
image build + Trivy), and `nightly.yml` (multi-arch + cross-OS). All third-party
GitHub Actions are pinned to commit SHAs; workflows default to
`permissions: contents: read`.

`release.yml` runs on a semver tag (`vX.Y.Z`) or a controlled
`workflow_dispatch` re-run. It builds `-trimpath` binaries for
linux/darwin/windows amd64+arm64, publishes a multi-arch image, and produces:
SBOM (SPDX), a signed `checksums.txt` (keyless Cosign), keyless Cosign image
signature, and GitHub build-provenance attestations. Stable tags update
`latest`; pre-release tags (e.g. `v0.10.0-rc.1`) never move it. Release notes are
extracted from the matching section of `CHANGELOG.md`.

## License

Same as main project.
