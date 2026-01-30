# Traefik 接入

本目录仅保留 **三合一** 部署；**三分开** 部署见独立子目录 `compose/traefik-herald/`、`compose/traefik-warden/`、`compose/traefik-stargate/`。

## 版本一：三合一（本目录）

Stargate、Warden、Herald 及示例受保护服务写在同一文件中，适合本地/单机一键启动。

**文件**：`compose/traefik/docker-compose.yml`

**前置**：创建 Traefik 外部网络（若尚未存在）

```bash
docker network create traefik
```

**启动**（在项目根目录 `stargate-suite` 下）：

```bash
docker compose -f compose/traefik/docker-compose.yml up -d
# 或
make up-traefik
```

**停止**：

```bash
docker compose -f compose/traefik/docker-compose.yml down
# 或
make down-traefik
```

---

## 版本二：三分开（独立子目录）

每种用法对应一个子目录、一个 `docker-compose.yml`，便于分机器部署或独立扩缩容。

| 子目录 | 内容 |
|--------|------|
| `compose/traefik-herald/` | Herald + herald-redis |
| `compose/traefik-warden/` | Warden + warden-redis |
| `compose/traefik-stargate/` | Stargate + protected-service（whoami） |

**前置**：创建共享网络（执行一次）

```bash
docker network create the-gate-network
docker network create traefik
# 或
make net-traefik-split
```

**启动顺序**：Herald → Warden → Stargate（Stargate 依赖前两者）

```bash
docker compose -f compose/traefik-herald/docker-compose.yml up -d
docker compose -f compose/traefik-warden/docker-compose.yml up -d
docker compose -f compose/traefik-stargate/docker-compose.yml up -d
```

或使用 Makefile：

```bash
make up-traefik-herald
make up-traefik-warden
make up-traefik-stargate
```

**停止**：按需分别停止

```bash
make down-traefik-stargate
make down-traefik-warden
make down-traefik-herald
```

**说明**：三分开时 Stargate 通过容器名访问 Warden/Herald（`the-gate-warden:8081`、`the-gate-herald:8082`），需保证三者都接入同一外部网络 `the-gate-network`。

---

## 环境变量

两种方式均支持通过 `.env` 或环境变量覆盖，例如：

- `STARGATE_DOMAIN`、`PROTECTED_DOMAIN`、`AUTH_HOST`
- `HERALD_API_KEY`、`HERALD_HMAC_SECRET`
- `WARDEN_API_KEY`
- `*_IMAGE`、`*_REDIS_IMAGE` 等

详见项目根目录 `.env` 示例。
