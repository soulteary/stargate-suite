中文 | [English](README.md)

# 配置

CLI 预设（`presets.json`）与 Web UI 页面配置。总览见 [../README.zh-CN.md](../README.zh-CN.md)。

## 页面配置（Web UI）

`serve` 加载 `page.yaml` 后合并：`config-sections.yaml`、`services.yaml`、`providers.yaml`、`i18n/zh.yaml`、`i18n/en.yaml`。单文件 `page.yaml` 仍兼容。

## 预设与 compose 路径

Makefile / `run-e2e.sh` 使用 `COMPOSE_FILE`（默认 `build/image/docker-compose.yml`）。`presets.json` 中预设：default、image、build、traefik、traefik-herald、traefik-warden、traefik-stargate → 对应 `compose/example/` 或 `build/` 下路径。

## 命令

```bash
./suite gen [image|build|traefik|all]   # 默认 all → build/
./suite -o dist gen traefik
./suite serve   # http://localhost:8085，-port 或 SERVE_PORT
```

参见 [../README.zh-CN](../README.zh-CN.md) · [../compose/README.zh-CN](../compose/README.zh-CN.md)。
