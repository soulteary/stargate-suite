中文 | [English](README.md)

# Compose 示例与生成

本目录仅保留**静态示例**与**单一数据源**，其余 compose 由 CLI 或 Web UI 生成到 `build/`。所有命令均在项目根目录 `stargate-suite` 下执行。项目总览见 [README.zh-CN.md](../README.zh-CN.md)。

## 目录说明

| 目录 | 说明 |
|------|------|
| **example/image/** | 静态示例：使用预构建镜像运行，适合快速体验与 CI |
| **example/build/** | 静态示例：从源码构建 Stargate、Warden、Herald，适合本地开发与 E2E 测试 |
| **canonical/** | 单一数据源：完整 Traefik 三合一 compose，用于解析生成 traefik / traefik-herald / traefik-warden / traefik-stargate |
| **traefik/** | 可选：Traefik 三合一与三分开部署说明（见 [traefik/README.zh-CN.md](./traefik/README.zh-CN.md)） |

**生成输出**均在 `build/` 目录（由 `go run ./cmd/suite gen all` 或 Web UI 生成）：

| 生成目录 | 说明 | 启动命令 |
|----------|------|----------|
| build/image/ | 来自 example/image + .env | `docker compose -f build/image/docker-compose.yml up -d` |
| build/build/ | 来自 example/build + .env | `docker compose -f build/build/docker-compose.yml up -d --build` |
| build/traefik/ | 三合一：接入 Traefik（含 Stargate、Warden、Herald、herald-totp、herald-dingtalk、Redis） | `docker compose -f build/traefik/docker-compose.yml up -d` |
| build/traefik-herald/ | 三分开：仅 Herald + herald-totp + Redis | `docker compose -f build/traefik-herald/docker-compose.yml up -d` |
| build/traefik-warden/ | 三分开：仅 Warden + Redis | `docker compose -f build/traefik-warden/docker-compose.yml up -d` |
| build/traefik-stargate/ | 三分开：仅 Stargate + 受保护服务（若启用则依赖 Herald/Warden/herald-totp 已启动） | `docker compose -f build/traefik-stargate/docker-compose.yml up -d` |

## 使用方式

### 首次使用：生成到 build/

```bash
# 在项目根目录
go run ./cmd/suite gen all
# 或
make gen
```

### 从预构建镜像启动（build/image）

```bash
docker compose -f build/image/docker-compose.yml up -d
```

### 从源码构建启动（build/build）

需要 `herald`、`warden`、`stargate` 与 `stargate-suite` 处于同级目录。

```bash
docker compose -f build/build/docker-compose.yml up -d --build
```

### 接入 Traefik（build/traefik）

1. 创建 Traefik 网络：`docker network create traefik`
2. 确保 Traefik 已运行
3. 启动：`docker compose -f build/traefik/docker-compose.yml up -d`

可在各 `build/<mode>/.env` 中配置 `STARGATE_DOMAIN`、`PROTECTED_DOMAIN` 等。

## 三分开与单一数据源

- **canonical**（`compose/canonical/docker-compose.yml`）为唯一维护的“完整 Traefik” compose。
- **三分开**（traefik-herald / traefik-warden / traefik-stargate）由 canonical **解析生成**到 `build/`，无需手改。
- 修改 canonical 后执行 `go run ./cmd/suite gen traefik` 或 `go run ./cmd/suite gen-split` 即可重新生成。

## Web UI 生成

执行 `go run ./cmd/suite serve`（默认 http://localhost:8085），在网页上勾选要生成的 compose 类型，点击生成即可下载 `docker-compose.yml` 与 `.env`。

## 环境变量与 .env

生成时会将根目录 `.env`（若存在）或从 canonical 推断的变量写入各 `build/<mode>/.env`。常用变量：

- `AUTH_HOST`、`STARGATE_DOMAIN`、`PROTECTED_DOMAIN`
- `HERALD_API_KEY`、`HERALD_HMAC_SECRET`、`WARDEN_API_KEY`
- `*_IMAGE`：覆盖默认镜像
- 钉钉通道（可选）：`HERALD_DINGTALK_*`、`DINGTALK_*`（含 `DINGTALK_LOOKUP_MODE`）

## 相关文档

- [README.zh-CN.md](../README.zh-CN.md) — 项目总览与快速开始
- [config/README.zh-CN.md](../config/README.zh-CN.md) — CLI 预设与 compose 路径
- [traefik/README.zh-CN.md](./traefik/README.zh-CN.md) — Traefik 三合一/三分开详细说明
