中文 | [English](README.md)

# Compose

单一数据源；所有输出由 Web UI 或 `make gen`（Web API）从 `canonical/docker-compose.yml` 生成到 `build/`。在项目根目录执行。总览见 [../README.zh-CN.md](../README.zh-CN.md)。

## 目录

| 目录 | 说明 |
|------|------|
| canonical/ | **唯一数据源。** 单份 compose 生成 image、build、traefik 及三分开（traefik-herald、traefik-warden、traefik-stargate）。仅在此修改。 |
| example/ | 可留空；image 与 build 由 canonical 生成，不再从此处复制。 |
| traefik/ | 仅说明：[traefik/README.zh-CN.md](./traefik/README.zh-CN.md)。compose 在 `build/traefik/`（生成）。 |

**生成目录（build/）：** image、build、traefik、traefik-herald、traefik-warden、traefik-stargate。均来自 `canonical/docker-compose.yml`。

## 使用

```bash
make gen
make up    # 默认 build/image
# 或：docker compose -f build/image/docker-compose.yml up -d
```

- **预构建：** `build/image/` → `docker compose -f build/image/docker-compose.yml up -d`
- **源码构建：** `build/build/` → `docker compose -f build/build/docker-compose.yml up -d --build`
- **Traefik：** 使用前先执行 `make gen`，以生成 `build/traefik/`。然后 `docker network create traefik` 再 `docker compose -f build/traefik/docker-compose.yml up -d`

三分开由 canonical 生成；修改 canonical 后执行 `make gen`。  
Web UI：`go run ./cmd/suite serve` → 选择类型，下载 compose 与 .env。

**环境变量：** 根目录 `.env`（或 canonical）写入各 `build/<mode>/.env`。常用：`AUTH_HOST`、`STARGATE_DOMAIN`、`*_API_KEY`、`*_IMAGE`；可选钉钉/SMTP/OwlMail 见根目录 `.env.example`。

参见 [../README.zh-CN](../README.zh-CN.md) · [../config/README.zh-CN](../config/README.zh-CN.md) · [traefik/README.zh-CN](./traefik/README.zh-CN.md)。
