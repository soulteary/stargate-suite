中文 | [English](README.md)

# 配置

CLI 预设（`presets.json`）与 Web UI 页面配置。总览见 [../README.zh-CN.md](../README.zh-CN.md)。

## 页面配置（Web UI）

`serve` 加载 `page.yaml` 后合并：`config-sections.yaml`、`services.yaml`、`providers.yaml`、`i18n/zh.yaml`、`i18n/en.yaml`。单文件 `page.yaml` 仍兼容。

## 预设与 compose 路径

- **CLI/Makefile/E2E 默认使用的 compose 文件**：`COMPOSE_FILE` 默认为 `build/image/docker-compose.yml`，即生成输出在 `build/` 目录下。
- **presets.json 语义**：
  - `default`：表示**示例 compose 路径**，指向 `compose/example/image/docker-compose.yml`（仓库内预置示例，非生成结果）。
  - `image`、`build`、`traefik`、`traefik-herald`、`traefik-warden`、`traefik-stargate`：对应 `build/` 下各生成产物路径。
- 运行 `./suite gen all` 后，实际使用的文件在 `build/` 下；`compose/example/` 仅作参考或单独示例。

## 敏感项与生产环境

- **API_KEY、HMAC_SECRET、各类密码**等敏感项在配置中不设默认密钥，仅保留空占位或说明性 placeholder。
- **生产环境必须修改**所有密钥与 API 凭据，不得使用测试占位符。请在部署前在 Web UI「密钥生成」或 .env 中配置强随机值。

## 新增环境变量清单（配置与代码同步）

新增或修改某服务的环境变量时，需按顺序同步以下位置，否则会出现「界面有项但生成不生效」或「生成有 key 但 UI 不展示」：

1. **compose 源**：在 `compose/canonical/docker-compose.yml` 中为该服务添加或修改 `environment` 项（如 `- VAR=${VAR:-default}`）。
2. **Web UI 配置**：在 `services.yaml` 或 `providers.yaml` 中对应服务的 `sections[].envVars` 增加条目（`env`、`type`、`labelKey`、`descKey` 等）。
3. **composegen 白名单**：在 `internal/composegen/composegen.go` 的 `serviceAllowedEnvKeys` 中，为该服务名对应的 map 添加新变量名。
4. **可选**：在 `envComments` 中补充注释；在 `EnvBodyFromVars` 的 `order` 切片中加入新 key，以控制生成 .env 中的顺序。

## 配置校验（可选）

运行 `./suite validate` 可检查 `page.yaml` 与合并后的 config（config-sections、services、providers、i18n）是否能正确加载；用于 CI 或本地快速检查。若需更严格校验，可后续为 config 增加 JSON Schema 或各 type（imageEnv、redisPaths、checkbox 等）的必填/可选字段校验。

## 命令

```bash
./suite validate                        # 校验 config 是否可加载
./suite gen [image|build|traefik|all]   # 默认 all → build/
./suite -o dist gen traefik
./suite serve   # http://localhost:8085，-port 或 SERVE_PORT
```

参见 [../README.zh-CN](../README.zh-CN.md) · [../compose/README.zh-CN](../compose/README.zh-CN.md)。
