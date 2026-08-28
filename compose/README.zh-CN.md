中文 | [English](README.md)

# Compose

单一数据源；所有输出由 Web UI 或原生 CLI `make gen` 从 `canonical/docker-compose.yml` 生成到 `build/`。在项目根目录执行。总览见 [../README.zh-CN.md](../README.zh-CN.md)。

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

## 安全加固（由 Profile 驱动）

生成的 compose 会按所选部署 Profile（`development` / `test` / `production`，见 [../config/profiles.yaml](../config/profiles.yaml)）加固。CLI（`generate --profile`）与 Web UI 调用同一套 `internal/policy` + `internal/composegen` 变换，因此两条路径生成的拓扑与控制项完全一致。

- **网络分段（S-03）：** 扁平的 `the-gate-network` 被拆分为按用途隔离的内部网络——`auth-internal`（Stargate ↔ Warden/Herald）、`warden-data`（Warden ↔ 其 Redis）、`herald-data`（Herald ↔ 其 Redis 及通道适配器）。外部 edge 网络仍为 Traefik 入口网络。被攻陷的通道适配器无法触及 Warden 数据面，且每个 Redis 仅其归属服务可达。
- **内部服务不发布宿主机端口（S-03）：** `production` 仅暴露反向代理入口；核心服务与 Redis 使用 `expose`（容器内部），不映射宿主机端口。`development` / `test` 保留宿主机访问，但仅绑定 `127.0.0.1`。
- **Redis 密码闭环（S-01）：** 两个 Redis 服务端均以 `--requirepass ${..._REDIS_PASSWORD:?...}` 启动，健康检查也用密码认证，密码不留空。`production` 下密码为必填（缺失时生成失败，`docker compose config` 也会在 `${VAR:?}` 插值处报错）；`development` / `test` 可自动生成或用隔离的测试值。
- **容器最小权限：** 每个服务丢弃全部 Linux capability（`cap_drop: [ALL]`）并禁止提权（`security_opt: [no-new-privileges:true]`）。`production` 额外挂载只读根文件系统（`read_only: true`）并提供可写的 `/tmp` tmpfs；Redis 数据持久化到具名 volume。

参见 [../README.zh-CN](../README.zh-CN.md) · [../config/README.zh-CN](../config/README.zh-CN.md) · [traefik/README.zh-CN](./traefik/README.zh-CN.md)。
