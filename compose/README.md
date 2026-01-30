# Compose 示例

不同目录对应不同使用场景的 Docker Compose 配置，**均在项目根目录 `stargate-suite` 下执行**（路径相对于各 compose 文件所在目录）。

## 目录说明

每种使用方式对应一个子目录，目录内仅包含该方式所需的 `docker-compose.yml`（及可选 `.env`）。

| 目录 | 说明 | 启动命令 |
|------|------|----------|
| **build/** | 从源码构建 Stargate、Warden、Herald，适合本地开发与 E2E 测试 | `docker compose -f compose/build/docker-compose.yml up -d --build` |
| **image/** | 使用预构建镜像运行，无需本地源码，适合快速体验与 CI | `docker compose -f compose/image/docker-compose.yml up -d` |
| **traefik/** | 三合一：接入外部 Traefik，Stargate Forward Auth + 示例受保护服务 | `docker compose -f compose/traefik/docker-compose.yml up -d` |
| **traefik-herald/** | 三分开：仅 Herald + Redis | `docker compose -f compose/traefik-herald/docker-compose.yml up -d` |
| **traefik-warden/** | 三分开：仅 Warden + Redis | `docker compose -f compose/traefik-warden/docker-compose.yml up -d` |
| **traefik-stargate/** | 三分开：仅 Stargate + 受保护服务（依赖 Herald/Warden 已启动） | `docker compose -f compose/traefik-stargate/docker-compose.yml up -d` |

## 使用方式

### 从源码构建（build）

```bash
# 在 stargate-suite 根目录
docker compose -f compose/build/docker-compose.yml up -d --build
```

需要 `herald`、`warden`、`stargate` 与 `stargate-suite` 处于同级目录（例如同一 repo 下）。

### 使用预构建镜像（image）

```bash
docker compose -f compose/image/docker-compose.yml up -d
```

### 接入 Traefik（traefik）

1. 创建 Traefik 网络：`docker network create traefik`
2. 确保 Traefik 已运行
3. 启动：

```bash
docker compose -f compose/traefik/docker-compose.yml up -d
```

可在 `.env` 中配置 `STARGATE_DOMAIN`、`PROTECTED_DOMAIN` 等。

## 通过 CLI 生成到 build 目录

在项目根目录执行 `go run ./cmd/suite gen [image|build|traefik|all]`（或 `./bin/suite gen ...`），可将各子目录的 compose 与根目录 `.env` 拷贝到 `build/<mode>/`。`gen traefik` 会生成 4 个子目录：`build/traefik/`、`build/traefik-herald/`、`build/traefik-warden/`、`build/traefik-stargate/`。输出目录可通过 `-o` 或 `GEN_OUT_DIR` 指定，默认 `build`。

## 环境变量与 .env

各示例会读取项目根目录的 `.env`（若存在）。常用变量：

- `AUTH_HOST`、`STARGATE_DOMAIN`、`PROTECTED_DOMAIN`
- `HERALD_API_KEY`、`HERALD_HMAC_SECRET`、`WARDEN_API_KEY`
- `*_IMAGE`：覆盖默认镜像
