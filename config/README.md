English | [中文](README.zh-CN.md)

# Config

CLI presets (`presets.json`) and Web UI page config. Overview: [../README.md](../README.md).

## Page config (Web UI)

`serve` loads `page.yaml` then merges: `config-sections.yaml`, `services.yaml`, `providers.yaml`, `i18n/en.yaml`, `i18n/zh.yaml`. Single monolithic `page.yaml` still works.

## Presets & compose path

Makefile / `run-e2e.sh` use `COMPOSE_FILE` (default `build/image/docker-compose.yml`). Presets in `presets.json`: default, image, build, traefik, traefik-herald, traefik-warden, traefik-stargate → paths under `compose/example/` or `build/`.

## Commands

```bash
./suite gen [image|build|traefik|all]   # default all → build/
./suite -o dist gen traefik
./suite serve   # http://localhost:8085, -port or SERVE_PORT
```

See [../README](../README.md) · [../compose/README](../compose/README.md).
