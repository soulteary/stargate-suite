中文 | [English](README.md)

# Compose

静态示例 + 单一数据源；输出由 CLI 或 Web UI 生成到 `build/`。在项目根目录执行。总览见 [../README.zh-CN.md](../README.zh-CN.md)。

## 目录

| 目录 | 说明 |
|------|------|
| example/image/ | 预构建镜像 — 快速体验、CI |
| example/build/ | 从源码构建 — 本地开发、E2E |
| canonical/ | 单一数据源 — 生成 traefik 及三分开 |
| traefik/ | 说明：[traefik/README.zh-CN.md](./traefik/README.zh-CN.md) |

**生成目录（build/）：** image、build、traefik、traefik-herald、traefik-warden、traefik-stargate。

## 使用

```bash
make gen
make up    # 默认 build/image
# 或：docker compose -f build/image/docker-compose.yml up -d
```

- **预构建：** `build/image/` → `docker compose -f build/image/docker-compose.yml up -d`
- **源码构建：** `build/build/` → `docker compose -f build/build/docker-compose.yml up -d --build`
- **Traefik：** `docker network create traefik` 后 `docker compose -f build/traefik/docker-compose.yml up -d`

三分开由 canonical 生成；修改 canonical 后执行 `go run ./cmd/suite gen traefik`。  
Web UI：`go run ./cmd/suite serve` → 选择类型，下载 compose 与 .env。

**环境变量：** 根目录 `.env`（或 canonical）写入各 `build/<mode>/.env`。常用：`AUTH_HOST`、`STARGATE_DOMAIN`、`*_API_KEY`、`*_IMAGE`；可选钉钉/SMTP/OwlMail 见根目录 `.env.example`。

参见 [../README.zh-CN](../README.zh-CN.md) · [../config/README.zh-CN](../config/README.zh-CN.md) · [traefik/README.zh-CN](./traefik/README.zh-CN.md)。
