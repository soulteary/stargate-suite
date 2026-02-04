中文 | [English](README.md)

# Traefik

**三合一：** `build/traefik/docker-compose.yml`。**三分开：** 生成目录 `build/traefik-herald/`、`build/traefik-warden/`、`build/traefik-stargate/`。Compose 见 [../README.zh-CN.md](../README.zh-CN.md)，项目见 [../../README.zh-CN.md](../../README.zh-CN.md)。

## 三合一（build/traefik）

```bash
docker network create traefik
docker compose -f build/traefik/docker-compose.yml up -d
# 或 make up-traefik
```

停止：`make down-traefik` 或 `docker compose -f build/traefik/docker-compose.yml down`。

## 三分开

由 canonical 生成，勿手改。修改 canonical 后：`go run ./cmd/suite gen traefik`。

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
