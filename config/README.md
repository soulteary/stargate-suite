# 配置目录

本目录存放 CLI 预设配置（`presets.json`），用于解析默认 compose 文件与 `--preset` 名称。项目总览见 [README.md](../README.md)。

## 与 compose 的对应关系

| 预设名 | compose 文件路径 | 说明 |
|--------|------------------|------|
| `default` / `image` | `compose/image/docker-compose.yml` | 预构建镜像，默认场景 |
| `build` | `compose/build/docker-compose.yml` | 从源码构建 |
| `traefik` | `compose/traefik/docker-compose.yml` | 接入 Traefik（三合一） |
| `traefik-herald` | `compose/traefik-herald/docker-compose.yml` | 三分开：仅 Herald |
| `traefik-warden` | `compose/traefik-warden/docker-compose.yml` | 三分开：仅 Warden |
| `traefik-stargate` | `compose/traefik-stargate/docker-compose.yml` | 三分开：Stargate + 受保护服务 |

更多说明见项目根目录 [compose/README.md](../compose/README.md)。

## 使用方式

- **默认**：不指定时使用 `presets.json` 中的 `default` 对应路径。
- **环境变量**：`COMPOSE_FILE=<路径>` 可覆盖默认（优先级高于默认，低于命令行）。
- **命令行**：
  - `-f <路径>` / `--file <路径>`：直接指定 compose 文件路径。
  - `--preset <名称>`：使用 `presets.json` 中的预设名（如 `traefik`、`build`）。

优先级：**命令行 -f / --preset > 环境变量 COMPOSE_FILE > 默认（presets.default）**。

示例：

```bash
# 使用默认 compose
./suite up

# 使用环境变量
COMPOSE_FILE=compose/traefik/docker-compose.yml ./suite up

# 使用 -f
./suite -f compose/traefik/docker-compose.yml up

# 使用预设名
./suite --preset traefik up
```

## 生成 build 目录（gen 命令）

将各使用方式的 `docker-compose.yml` 与 `.env` 输出到指定目录（默认 `build`），便于分发或 CI：

```bash
./suite gen [image|build|traefik|all]   # 默认 all
./suite -o dist gen traefik             # 输出到 dist/，含 dist/traefik/、dist/traefik-herald/、dist/traefik-warden/、dist/traefik-stargate/
```

## 相关文档

- [README.md](../README.md) — 项目总览
- [compose/README.md](../compose/README.md) — Compose 各子目录用法
