English | [中文](README.zh-CN.md)

# Config

CLI presets (`presets.json`) and Web UI page config. Overview: [../README.md](../README.md). Scenario usage is documented in [../SCENARIOS.md](../SCENARIOS.md).

## Page config (Web UI)

`serve` loads `page.yaml` then merges: `config-sections.yaml`, `services.yaml`, `providers.yaml`, `i18n/en.yaml`, `i18n/zh.yaml`. Single monolithic `page.yaml` still works.

## Presets & compose path

- **Default compose file used by Makefile/E2E**: `COMPOSE_FILE` defaults to `build/image/docker-compose.yml`; all compose output is generated under `build/` from canonical.
- **Generation is Web UI only** (or `make gen`, which runs `scripts/gen-via-api.sh` and calls the Web API). There is no CLI `gen` or `gen-split` command.
- **Modes**: `image`, `build`, `traefik`, `traefik-herald`, `traefik-warden`, `traefik-stargate` — outputs under `build/<mode>/`.
- **scenarios.json**: Defines scenario presets (`modes` + `options` + `envOverrides`) for the Web UI; scenario output is produced only via the Web UI (choose preset and generate).
- **canonical**: `compose/canonical/docker-compose.yml` is the base template; Web UI scenario presets (S1~S5) select modes and options.
- **Web UI behavior**:
  - In step 1 you choose a scenario preset to auto-fill options and env overrides; compose outputs use the scenario’s modes.
  - In "Import and parse config", the app suggests and applies the best-matched scenario preset, then overlays imported values.

## Sensitive options & production

- **API_KEY, HMAC_SECRET, passwords** and other secrets have no default values in config; only empty or descriptive placeholders.
- **Production deployments must override** all keys and API credentials; do not use test placeholders. Use the Web UI "密钥生成" / Keys tab or set strong values in `.env` before deploy.

## Adding or changing env vars (config/code sync)

When adding or changing a service’s environment variables, keep these in sync or the UI and generated compose/.env will diverge:

1. **Compose source**: In `compose/canonical/docker-compose.yml`, add or update `environment` entries (e.g. `- VAR=${VAR:-default}`) for that service.
2. **Web UI config**: In `services.yaml` or `providers.yaml`, add an entry under the service’s `sections[].envVars` (`env`, `type`, `labelKey`, `descKey`, etc.).
3. **env-meta** (single source for .env order/comments/defaults): In `config/env-meta.yaml`, add the key to `order` and under `vars` with `comment`, `services`, and optional `default`.
4. See also: **Adding a scenario or global option** below for `scenarios.json` and `scenarioOptionSetters` / `optionToComposeGenJSONSetters`.

## Adding a scenario or global option

- **New scenario**: Add an entry in `config/scenarios.json` with `modes`, `envOverrides`, and `options` (keys must exist in `scenarioOptionSetters` in `cmd/suite/cmd_gen.go`).
- **New scenario option key**: Add the key to `scenarioOptionSetters` in `cmd/suite/cmd_gen.go` and (if used by Web UI) to `optionToComposeGenJSONSetters` and the corresponding field in `composeGenOptionsJSON` / `composegen.Options`; then add it to scenario presets in `scenarios.json` as needed.

## Config validation (optional)

Run `./suite validate` to check that `page.yaml` and the merged config load correctly, and (when `config/env-meta.yaml` and `config/scenarios.json` exist) consistency between canonical compose env vars and env-meta, and scenario option keys. Useful in CI or for a quick local check.

## Commands

```bash
./suite validate   # validate that config loads
./suite serve      # Web UI at http://localhost:8085 (-port or SERVE_PORT)
```

Generate compose: use the Web UI, or run `make gen` (calls Web API via `scripts/gen-via-api.sh`).

See [../README](../README.md) · [../compose/README](../compose/README.md).
