English | [中文](README.zh-CN.md)

# Config

CLI presets (`presets.json`) and Web UI page config. Overview: [../README.md](../README.md).

## Page config (Web UI)

`serve` loads `page.yaml` then merges: `config-sections.yaml`, `services.yaml`, `providers.yaml`, `i18n/en.yaml`, `i18n/zh.yaml`. Single monolithic `page.yaml` still works.

## Presets & compose path

- **Default compose file used by CLI/Makefile/E2E**: `COMPOSE_FILE` defaults to `build/image/docker-compose.yml`; generated output lives under `build/`.
- **presets.json semantics**:
  - `default`: **Example compose path** only — points to `compose/example/image/docker-compose.yml` (in-repo sample, not generated output).
  - `image`, `build`, `traefik`, `traefik-herald`, `traefik-warden`, `traefik-stargate`: paths under `build/` for generated artifacts.
- After `./suite gen all`, the files you use are under `build/`; `compose/example/` is for reference only.

## Sensitive options & production

- **API_KEY, HMAC_SECRET, passwords** and other secrets have no default values in config; only empty or descriptive placeholders.
- **Production deployments must override** all keys and API credentials; do not use test placeholders. Use the Web UI "密钥生成" / Keys tab or set strong values in `.env` before deploy.

## Adding or changing env vars (config/code sync)

When adding or changing a service’s environment variables, keep these in sync or the UI and generated compose/.env will diverge:

1. **Compose source**: In `compose/canonical/docker-compose.yml`, add or update `environment` entries (e.g. `- VAR=${VAR:-default}`) for that service.
2. **Web UI config**: In `services.yaml` or `providers.yaml`, add an entry under the service’s `sections[].envVars` (`env`, `type`, `labelKey`, `descKey`, etc.).
3. **composegen allowlist**: In `internal/composegen/composegen.go`, add the new variable name to the service’s map in `serviceAllowedEnvKeys`.
4. **Optional**: Add a comment in `envComments`; add the key to the `order` slice in `EnvBodyFromVars` to control .env output order.

## Config validation (optional)

Run `./suite validate` to check that `page.yaml` and the merged config (config-sections, services, providers, i18n) load correctly; useful in CI or for a quick local check. For stricter checks, you can add a JSON Schema or validate required/optional fields per type (imageEnv, redisPaths, checkbox, etc.) later.

## Commands

```bash
./suite validate                        # validate that config loads
./suite gen [image|build|traefik|all]   # default all → build/
./suite -o dist gen traefik
./suite serve   # http://localhost:8085, -port or SERVE_PORT
```

See [../README](../README.md) · [../compose/README](../compose/README.md).
