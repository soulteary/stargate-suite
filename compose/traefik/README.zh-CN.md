中文 | [English](README.md)

# Traefik

本目录**仅保留说明**。实际 Traefik compose 在 `build/traefik/`（三合一）与 `build/traefik-herald/`、`build/traefik-warden/`、`build/traefik-stargate/`（三分开），由 canonical 生成。使用前请先执行 `make gen`（经 Web API 生成），否则 `build/traefik/` 不存在。

Traefik 的 compose 位于 **build/traefik/**（三分开在 `build/traefik-herald/` 等）。均由 **canonical 生成** — 本目录仅保留说明，无手写 compose。Compose 见 [../README.zh-CN.md](../README.zh-CN.md)，项目见 [../../README.zh-CN.md](../../README.zh-CN.md)。

**使用 Traefik 前必须先执行 `make gen`**，否则 `build/traefik/` 不存在。

## 三合一（build/traefik）

```bash
docker network create traefik
docker compose -f build/traefik/docker-compose.yml up -d
# 或 make up-traefik
```

停止：`make down-traefik` 或 `docker compose -f build/traefik/docker-compose.yml down`。

## 三分开

由 canonical 生成，勿手改。修改 canonical 后执行 `make gen`。

| 目录 | 内容 |
|------|------|
| build/traefik-herald/ | Herald + herald-redis |
| build/traefik-warden/ | Warden + warden-redis |
| build/traefik-stargate/ | Stargate + whoami |

```bash
make net-traefik-split   # 一次：创建 the-gate-network、traefik
make up-traefik-herald && make up-traefik-warden && make up-traefik-stargate
# 停止：make down-traefik-stargate、down-traefik-warden、down-traefik-herald
```

三分开通过容器名（the-gate-warden:8081、the-gate-herald:8082）在共享网络 `the-gate-network` 上通信。

**环境变量：** .env 或环境变量 — STARGATE_DOMAIN、PROTECTED_DOMAIN、AUTH_HOST、*_API_KEY、*_IMAGE。见根目录 .env.example。

参见 [../README.zh-CN](../README.zh-CN.md) · [../../README.zh-CN](../../README.zh-CN.md)。
