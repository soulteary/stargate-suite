English | [中文](README.zh-CN.md)

# Config

Web UI page config and scenario presets (`scenarios.json`). Overview: [../README.md](../README.md). Scenario usage is documented in [../SCENARIOS.md](../SCENARIOS.md).

## Page config (Web UI)

`serve` loads `page.yaml` then merges: `config-sections.yaml`, `services.yaml`, `providers.yaml`, `i18n/en.yaml`, `i18n/zh.yaml`, `ports.yaml`. Single monolithic `page.yaml` still works.

- **ports.yaml**: Centralized port config for all services (service name, container port, default host port, i18n keys). The wizard step-2 "exposed ports" table is generated from it; port-type inputs in the UI (e.g. step-5 `HERALD_TOTP_PORT`, Herald `PORT`) have their defaults/placeholders overridden from this file on load, kept consistent with the port table. Container ports, `HERALD_TOTP_PORT`, and compose port mappings in the generation logic all follow `ports.yaml`.
- Image fields in `config-sections.yaml`, `services.yaml`, and `providers.yaml` intentionally omit defaults. At startup the Web UI resolves their current defaults and placeholders from `components.yaml`; imported or previously saved session values take precedence.
- Every config-driven `labelKey` and `descKey` must have non-empty `zh` and `en` entries. Startup validation rejects missing text, while templates render the Chinese entry as a server-side fallback before JavaScript applies the selected language.

## Component manifest (single source of truth)

- **components.yaml**: The authoritative registry of component **versions, images, container ports, health paths, and contract versions**. It fixes version/port drift (M-01): `env-meta.yaml`, `.env.example`, `compose/canonical`, and `composegen` container-port defaults must all match it, and `suite version` reads its `verifiedCombo` for the validated target combination.
  - `components.*` registers the current v1 image and runtime contracts used by generation.
  - `verifiedCombo` is the release-tested core combination shown by `suite version`; drift tests require it to match the registered component versions.
  - `dependencies` registers non-suite images (Redis, whoami) so they are not re-hardcoded elsewhere.
  - Drift is enforced by tests in `internal/contract` (`manifest_test.go`). Generated `build/*` is a gitignored artifact and is **not** a source of truth: a non-failing advisory flags stale `build/*` image pins so they can be regenerated.
- **components.lock.yaml**: Placeholder structure for image content-address digests (`sha256:...`), filled at release time so deployments are reproducible.

## Presets & compose path

- **Default compose file used by Makefile/E2E**: `COMPOSE_FILE` defaults to `build/image/docker-compose.yml`; all compose output is generated under `build/` from canonical.
- **Generation**: `make gen` invokes the native `suite generate` command. The Web UI uses the same policy and compose-generation packages.
- **Modes**: `image`, `build`, `traefik`, `traefik-herald`, `traefik-warden`, `traefik-stargate` — outputs under `build/<mode>/`.
- **scenarios.json**: Defines scenario presets (`modes` + `options` + `envOverrides`) for the Web UI; scenario output is produced only via the Web UI (choose preset and generate).
- **canonical**: `compose/canonical/docker-compose.yml` is the base template; Web UI scenario presets (S1~S5) select modes and options.
- **Web UI behavior**:
  - In step 1 you choose a scenario preset to auto-fill options and env overrides; compose outputs use the scenario’s modes.
  - In "Import and parse config", paste content or drop local Compose/`.env` text files into the corresponding fields. The app suggests and applies the best-matched scenario preset, then overlays imported values.
  - Clicking any labeled wizard step submits and saves the current form before navigating to that step. The Keys step also saves populated keys before leaving.

## Sensitive options & production

- **API_KEY, HMAC_SECRET, passwords** and other secrets have no default values in config; only empty or descriptive placeholders.
- **Production deployments must override** all keys and API credentials; do not use test placeholders. Use the Web UI "密钥生成" / Keys tab or set strong values in `.env` before deploy.

## Adding or changing env vars (config/code sync)

When adding or changing a service’s environment variables, keep these in sync or the UI and generated compose/.env will diverge:

1. **Compose source**: In `compose/canonical/docker-compose.yml`, add or update `environment` entries (e.g. `- VAR=${VAR:-default}`) for that service.
2. **Web UI config**: In `services.yaml` or `providers.yaml`, add an entry under the service’s `sections[].envVars` (`env`, `type`, `labelKey`, `descKey`, etc.).
3. **env-meta** (single source for .env order/comments/defaults): In `config/env-meta.yaml`, add the key to `order` and under `vars` with `comment`, `services`, and optional `default`.
4. **components.yaml** (for version/image/port changes only): Component versions, images, container ports, and health paths must be updated in `config/components.yaml`; the drift tests in `internal/contract` fail if `env-meta.yaml`, `.env.example`, `compose/canonical`, or `ports.yaml` diverge from it.
5. See also: **Adding a scenario or global option** below for `scenarios.json` and `scenarioOptionSetters` / `optionToComposeGenJSONSetters`.

## Adding a scenario or global option

- **New scenario**: Add an entry in `config/scenarios.json` with `modes`, `envOverrides`, and `options` (keys must exist in `scenarioOptionSetters` in `cmd/suite/cmd_gen.go`).
- **New scenario option key**: Add the key to `scenarioOptionSetters` in `cmd/suite/cmd_gen.go` and (if used by Web UI) to `optionToComposeGenJSONSetters` and the corresponding field in `composeGenOptionsJSON` / `composegen.Options`; then add it to scenario presets in `scenarios.json` as needed.

## Config validation (optional)

Run `./suite validate` to check that `page.yaml` and the merged config load correctly, and (when `config/env-meta.yaml` and `config/scenarios.json` exist) consistency between canonical compose env vars and env-meta, and scenario option keys. Useful in CI or for a quick local check.

## v1 config fields & four-layer profile validation

The v1 contracts (Stargate 1.0.0 / Warden 1.1.0 / Herald 1.1.0) add security-relevant env fields. They are registered for generation in `env-meta.yaml` and enforced by the runtime rules in `internal/policy`. The Go validator is the authority for field shape, secret strength, profile scope, and structured finding codes.

- **Stargate**: `COOKIE_SECURE`, `CALLBACK_ALLOWED_HOSTS`, `SESSION_EXCHANGE_SECRET`, `TRUSTED_PROXIES`, `PROXY_HEADER`, `PASSWORD_HEADER_AUTH_ENABLED`, `WARDEN_HMAC_KEY_ID` / `WARDEN_HMAC_SECRET`, `HERALD_HMAC_KEY_ID`, `WARDEN_TLS_*`.
- **Herald**: `REQUEST_AUTH_MODE`, `HERALD_HMAC_DEFAULT_KEY_ID`, `HMAC_MAX_DRIFT`, `HMAC_V1_ENABLED`, `HERALD_IDEMPOTENCY_SECRET`, `HERALD_PII_PEPPER`, `HERALD_TRUSTED_PROXIES` / `HERALD_TRUSTED_PROXY_HEADER`, `HERALD_TEST_API_KEY`, `HERALD_TEST_LISTENER_ADDR`.
- **Warden**: `ENVIRONMENT`, `WARDEN_HMAC_ALLOW_V1`, and `WARDEN_METRICS_REQUIRE_AUTH` are wired into the v1.1.0 configuration. The suite pins `WARDEN_HMAC_ALLOW_V1=false` by default so the legacy replayable v1 canonical form is never accepted.

`./suite validate --profile <development|test|production>` runs the same four-layer validator used by the Web UI (CLI and UI share `validateForProfile` → `policy.Validate`):

1. **Layer 1 — field type**: shape of a set field (port / URL / bool / duration / CIDR list / host list). Malformed shapes are hard errors in every profile.
2. **Layer 2 — single-field safety**: secret strength (≥32 chars, no placeholder), no plaintext passwords, Redis password present.
3. **Layer 3 — cross-field**: cross-domain callback/cookie requires a strong `SESSION_EXCHANGE_SECRET`; `STEP_UP_ENABLED` requires `STEP_UP_PATHS` + `TRUSTED_PROXIES`; TLS client cert/key must be a complete pair.
4. **Layer 4 — cross-service**: Stargate→Herald auth must resolve to one explicit mode; production rejects API-key-only and forbids HMAC v1.

Each finding carries a stable `code` (e.g. `HERALD_PII_PEPPER_WEAK`). Use `--json` for scriptable output:

```bash
./suite validate --profile production --json   # exits non-zero on any error finding
```

Production is always strict (cannot be relaxed with `--strict=false`); `--strict` promotes test/dev findings to hard errors too.

## v1 core images & HMAC v2

- **Images (from `components.yaml`, single source)**: Stargate `v1.0.0`, Warden `v1.1.0`, Herald `v1.1.0`; optional channel services are `v1.1.0`. `env-meta.yaml`, `.env.example`, `compose/canonical`, `ports.yaml` and the `internal/contract` drift tests all follow the manifest.
- **Stargate port 80 → 8080**: container port, host mapping, Traefik `loadbalancer.server.port`, and the `forwardauth.address` all move to `8080`.
- **Health/readiness paths**: Stargate liveness `/healthz` + readiness `/readyz`; Warden `/healthcheck`; Herald `/healthz`. `make health`, `.github/workflows/ci.yml`, and `scripts/run-e2e.sh` probe the new paths.
- **Herald explicit auth**: `REQUEST_AUTH_MODE=hmac_v2` is set explicitly (no implicit API-key/HMAC selection). With a single `HMAC_SECRET` (no `HERALD_HMAC_KEYS`) Herald resolves an implicit `default` key id, so clients may omit `X-Key-Id`.
- **HMAC v2 everywhere, v1 off**: `HMAC_V1_ENABLED=false` (Herald) and `WARDEN_HMAC_ALLOW_V1=false` (Warden) are pinned by policy; the legacy replayable v1 canonical form is never accepted.
- **Herald test-code listener**: the `/v1/test/code` endpoint is served only on a dedicated loopback-only listener (`HERALD_TEST_LISTENER_ADDR`, default `127.0.0.1:8092`) guarded by `HERALD_TEST_API_KEY`; the main `:8082` listener never exposes codes. The E2E helper reaches it via `docker compose exec herald curl` (`HERALD_COMPOSE_DIR`).
- **E2E signing migrated to HMAC v2**: the canonical string `HERALD-HMAC-V2\n<METHOD>\n<path>\n<query>\n<ts>\n<nonce>\n<service>\n<keyid>\n<sha256(body)>` mirrors `herald/internal/auth/hmac_v2.go` (v1.1.0) exactly and is cross-verified against the upstream `CanonicalRequest.Canonical()` field order. Former `X-API-Key` positive tests are inverted into negative tests asserting API keys are rejected under `hmac_v2`.

## Commands

```bash
./suite validate   # validate that config loads
./suite serve      # Web UI at http://localhost:8085 (-port or SERVE_PORT)
```

Generate compose with `make gen` (native CLI), or use the Web UI for interactive configuration.

See [../README](../README.md) · [../compose/README](../compose/README.md).
