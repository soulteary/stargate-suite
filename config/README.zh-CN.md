中文 | [English](README.md)

# 配置

Web UI 页面配置与场景预设（scenarios.json）。总览见 [../README.zh-CN.md](../README.zh-CN.md)，场景预设说明见 [../SCENARIOS.zh-CN.md](../SCENARIOS.zh-CN.md)。

## 页面配置（Web UI）

`serve` 加载 `page.yaml` 后合并：`config-sections.yaml`、`services.yaml`、`providers.yaml`、`i18n/zh.yaml`、`i18n/en.yaml`。单文件 `page.yaml` 仍兼容。

## 预设与 compose 路径

- **Makefile/E2E 默认 compose**：`COMPOSE_FILE` 默认为 `build/image/docker-compose.yml`；所有 compose 由 canonical 生成到 `build/`。
- **生成仅通过 Web UI**（或 `make gen`，内部调用 Web API 脚本 `scripts/gen-via-api.sh`）。无 CLI 的 `gen` / `gen-split` 子命令。
- **模式**：`image`、`build`、`traefik`、`traefik-herald`、`traefik-warden`、`traefik-stargate`，输出在 `build/<mode>/`。
- **scenarios.json**：定义场景预设（`modes` + `options` + `envOverrides`），仅在 Web UI 中选择预设并生成；不提供 CLI 按场景生成。
- **canonical**：`compose/canonical/docker-compose.yml` 为生成基础模板；Web UI 场景 S1~S5 选择模式与选项。
- **Web UI**：第一步选择场景预设自动填充选项与 env 覆盖；生成类型由场景模式决定。
- **导入**：在「导入并解析配置」中加载后，会推荐并套用最匹配场景预设，再叠加导入值。

## 敏感项与生产环境

- **API_KEY、HMAC_SECRET、各类密码**等敏感项在配置中不设默认密钥，仅保留空占位或说明性 placeholder。
- **生产环境必须修改**所有密钥与 API 凭据，不得使用测试占位符。请在部署前在 Web UI「密钥生成」或 .env 中配置强随机值。

## 新增环境变量清单（配置与代码同步）

新增或修改某服务的环境变量时，需按顺序同步以下位置，否则会出现「界面有项但生成不生效」或「生成有 key 但 UI 不展示」：

1. **compose 源**：在 `compose/canonical/docker-compose.yml` 中为该服务添加或修改 `environment` 项（如 `- VAR=${VAR:-default}`）。
2. **Web UI 配置**：在 `services.yaml` 或 `providers.yaml` 中对应服务的 `sections[].envVars` 增加条目（`env`、`type`、`labelKey`、`descKey` 等）。
3. **env-meta**：在 `config/env-meta.yaml` 的 `order` 中加入新 key，并在 `vars` 下为该 key 配置 `comment`、`services`（所属服务列表）及可选 `default`。由此统一 .env 顺序、注释与默认内容，无需再改 `internal/composegen/composegen.go`。

## 新增场景或全局选项

- **新增场景**：在 `config/scenarios.json` 中增加一项，填写 `modes`、`envOverrides`、`options`（options 的键须已在 `cmd/suite/cmd_gen.go` 的 `scenarioOptionSetters` 中定义）。
- **新增场景选项键**：在 `cmd/suite/cmd_gen.go` 的 `scenarioOptionSetters` 中增加该键，若 Web UI 也使用则需同步加入 `optionToComposeGenJSONSetters` 及 `composeGenOptionsJSON`/`composegen.Options` 对应字段，再在 `scenarios.json` 的预设中按需使用。

## 配置校验（可选）

运行 `./suite validate` 可检查 `page.yaml` 与合并后的 config 是否能正确加载，并在存在 `config/env-meta.yaml` 与 `config/scenarios.json` 时做一致性检查（canonical compose 与 env-meta、场景 options 键集合）；用于 CI 或本地快速检查。

## 命令

```bash
./suite validate   # 校验 config 是否可加载
./suite serve     # Web UI，http://localhost:8085（-port 或 SERVE_PORT）
```

生成 compose：在 Web UI 中操作，或执行 `make gen`（经 `scripts/gen-via-api.sh` 调用 Web API）。

参见 [../README.zh-CN](../README.zh-CN.md) · [../compose/README.zh-CN](../compose/README.zh-CN.md)。
