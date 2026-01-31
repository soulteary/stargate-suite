中文 | [English](README.md)

# 配置目录

本目录存放 CLI 预设配置（`presets.json`），用于解析默认 compose 文件与 `--preset` 名称。项目总览见 [README.zh-CN.md](../README.zh-CN.md)。

## 与 compose 的对应关系

| 预设名 | compose 文件路径 | 说明 |
|--------|------------------|------|
| `default` | `compose/example/image/docker-compose.yml` | 静态示例：预构建镜像 |
| `image` | `build/image/docker-compose.yml` | 生成后：预构建镜像（需先执行 gen） |
| `build` | `build/build/docker-compose.yml` | 生成后：从源码构建 |
| `traefik` | `build/traefik/docker-compose.yml` | 生成后：接入 Traefik（三合一） |
| `traefik-herald` | `build/traefik-herald/docker-compose.yml` | 生成后：三分开，仅 Herald |
| `traefik-warden` | `build/traefik-warden/docker-compose.yml` | 生成后：三分开，仅 Warden |
| `traefik-stargate` | `build/traefik-stargate/docker-compose.yml` | 生成后：三分开，Stargate + 受保护服务 |

更多说明见项目根目录 [compose/README.zh-CN.md](../compose/README.zh-CN.md)。

## 使用方式

- **默认**：不指定时使用 `presets.json` 中的 `default` 对应路径（示例为 compose/example/image，生成后可用 build/image）。
- **环境变量**：`COMPOSE_FILE=<路径>` 可覆盖默认（优先级高于默认，低于命令行）。
- **命令行**：
  - `-f <路径>` / `--file <路径>`：直接指定 compose 文件路径。
  - `--preset <名称>`：使用 `presets.json` 中的预设名（如 `traefik`、`build`）。

优先级：**命令行 -f / --preset > 环境变量 COMPOSE_FILE > 默认（presets.default）**。

示例：

```bash
# 使用默认 compose（示例或生成后的 build/image）
./suite up

# 使用环境变量
COMPOSE_FILE=build/traefik/docker-compose.yml ./suite up

# 使用 -f
./suite -f build/traefik/docker-compose.yml up

# 使用预设名
./suite --preset traefik up
```

## 生成 build 目录（gen 命令）

将各使用方式的 `docker-compose.yml` 与 `.env` 输出到指定目录（默认 `build`），便于分发或 CI：

```bash
./suite gen [image|build|traefik|all]   # 默认 all
./suite -o dist gen traefik             # 输出到 dist/，含 dist/traefik/、dist/traefik-herald/、dist/traefik-warden/、dist/traefik-stargate/
```

## Web UI（serve 命令）

启动网页生成器，勾选 compose 类型后下载配置：

```bash
./suite serve
# 默认 http://localhost:8085，可通过 -port 或 SERVE_PORT 指定端口
```

## 相关文档

- [README.zh-CN.md](../README.zh-CN.md) — 项目总览
- [compose/README.zh-CN.md](../compose/README.zh-CN.md) — Compose 示例与生成
