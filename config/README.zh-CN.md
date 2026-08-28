中文 | [English](README.md)

# 配置

Web UI 页面配置与场景预设（scenarios.json）。总览见 [../README.zh-CN.md](../README.zh-CN.md)，场景预设说明见 [../SCENARIOS.zh-CN.md](../SCENARIOS.zh-CN.md)。

## 页面配置（Web UI）

`serve` 加载 `page.yaml` 后合并：`config-sections.yaml`、`services.yaml`、`providers.yaml`、`i18n/zh.yaml`、`i18n/en.yaml`、`ports.yaml`。单文件 `page.yaml` 仍兼容。

- **ports.yaml**：所有服务的端口集中配置（服务名、容器端口、默认主机端口、i18n 键）。向导 step-2「暴露端口」表格由此生成；**界面中**的端口类输入（如 step-5 的 HERALD_TOTP_PORT、Herald 的 PORT）的默认值与占位符在加载时由此文件覆盖，与端口表一致。生成逻辑中容器端口、HERALD_TOTP_PORT 与 compose 端口映射均与 ports.yaml 一致。

## 组件清单（单一权威来源）

- **components.yaml**：组件**版本、镜像、容器端口、健康路径、契约版本**的权威登记表，用于修复版本/端口漂移（M-01）：`env-meta.yaml`、`.env.example`、`compose/canonical` 与 `composegen` 容器端口默认值都须与其一致，`suite version` 从其中的 `verifiedCombo` 读取已验证的目标组合。
  - `components.*` 登记**当前运行值**（迁移前旧值）。升级镜像到 v1 由后续 PR 原子完成，届时 `components.*` 与 `verifiedCombo` 才收敛一致。
  - `verifiedCombo` 是 `suite version` 展示的已验证**目标 v1** 组合（在两者不同的阶段有意与 `components.*` 分开）。
  - `dependencies` 登记非本套件的镜像（Redis、whoami），避免散落硬编码。
  - 漂移由 `internal/contract`（`manifest_test.go`）中的测试强制校验。生成的 `build/*` 是被 gitignore 的产物，**不是**权威来源：以非失败的 advisory 提示 `build/*` 镜像 pin 过期，便于重新生成。
- **components.lock.yaml**：镜像内容寻址 digest（`sha256:...`）的占位结构，正式发布时填入，使部署可复现。

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
4. **components.yaml**（仅版本/镜像/端口变更时）：组件版本、镜像、容器端口与健康路径须在 `config/components.yaml` 更新；若 `env-meta.yaml`、`.env.example`、`compose/canonical` 或 `ports.yaml` 与其不一致，`internal/contract` 的漂移测试会失败。

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
