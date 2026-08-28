中文 | [English](README.md)

# 配置

Web UI 页面配置与场景预设（scenarios.json）。总览见 [../README.zh-CN.md](../README.zh-CN.md)，场景预设说明见 [../SCENARIOS.zh-CN.md](../SCENARIOS.zh-CN.md)。

## 页面配置（Web UI）

`serve` 加载 `page.yaml` 后合并：`config-sections.yaml`、`services.yaml`、`providers.yaml`、`i18n/zh.yaml`、`i18n/en.yaml`、`ports.yaml`。单文件 `page.yaml` 仍兼容。

- **ports.yaml**：所有服务的端口集中配置（服务名、容器端口、默认主机端口、i18n 键）。向导 step-2「暴露端口」表格由此生成；**界面中**的端口类输入（如 step-5 的 HERALD_TOTP_PORT、Herald 的 PORT）的默认值与占位符在加载时由此文件覆盖，与端口表一致。生成逻辑中容器端口、HERALD_TOTP_PORT 与 compose 端口映射均与 ports.yaml 一致。

## 组件清单（单一权威来源）

- **components.yaml**：组件**版本、镜像、容器端口、健康路径、契约版本**的权威登记表，用于修复版本/端口漂移（M-01）：`env-meta.yaml`、`.env.example`、`compose/canonical` 与 `composegen` 容器端口默认值都须与其一致，`suite version` 从其中的 `verifiedCombo` 读取已验证的目标组合。
  - `components.*` 登记生成逻辑当前使用的 v1 镜像与运行契约。
  - `verifiedCombo` 是 `suite version` 展示的发布验证组合；漂移测试要求它与组件版本保持一致。
  - `dependencies` 登记非本套件的镜像（Redis、whoami），避免散落硬编码。
  - 漂移由 `internal/contract`（`manifest_test.go`）中的测试强制校验。生成的 `build/*` 是被 gitignore 的产物，**不是**权威来源：以非失败的 advisory 提示 `build/*` 镜像 pin 过期，便于重新生成。
- **components.lock.yaml**：镜像内容寻址 digest（`sha256:...`）的占位结构，正式发布时填入，使部署可复现。

## 预设与 compose 路径

- **Makefile/E2E 默认 compose**：`COMPOSE_FILE` 默认为 `build/image/docker-compose.yml`；所有 compose 由 canonical 生成到 `build/`。
- **生成方式**：`make gen` 调用原生 `suite generate` 命令；Web UI 与 CLI 共用策略和 compose 生成包。
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

## v1 配置字段与四层 Profile 校验

v1 契约（Stargate 1.0.0 / Warden 1.1.0 / Herald 1.1.0）新增了安全相关的环境变量，均已登记在 `env-meta.yaml`，并在 `config/schemas/env-fields.yaml` 声明（该 schema 与校验器保持一致；`internal/policy` 中的漂移测试会在 schema 引用了引擎未实现的 code 时失败）。

- **Stargate**：`COOKIE_SECURE`、`CALLBACK_ALLOWED_HOSTS`、`SESSION_EXCHANGE_SECRET`、`TRUSTED_PROXIES`、`PROXY_HEADER`、`PASSWORD_HEADER_AUTH_ENABLED`、`WARDEN_HMAC_KEY_ID` / `WARDEN_HMAC_SECRET`、`HERALD_HMAC_KEY_ID`、`WARDEN_TLS_*`。
- **Herald**：`REQUEST_AUTH_MODE`、`HERALD_HMAC_DEFAULT_KEY_ID`、`HMAC_MAX_DRIFT`、`HMAC_V1_ENABLED`、`HERALD_IDEMPOTENCY_SECRET`、`HERALD_PII_PEPPER`、`HERALD_TRUSTED_PROXIES` / `HERALD_TRUSTED_PROXY_HEADER`、`HERALD_TEST_API_KEY`、`HERALD_TEST_LISTENER_ADDR`。
- **Warden**：v1.1.0 配置已接入 `ENVIRONMENT`、`WARDEN_HMAC_ALLOW_V1` 与 `WARDEN_METRICS_REQUIRE_AUTH`。套件默认将 `WARDEN_HMAC_ALLOW_V1=false`，从不接受可重放的遗留 v1 规范串。

`./suite validate --profile <development|test|production>` 运行与 Web UI 相同的四层校验器（CLI 与 UI 共用 `validateForProfile` → `policy.Validate`）：

1. **第一层 · 字段类型**：已设置字段的形状（端口 / URL / 布尔 / 时长 / CIDR 列表 / Host 列表）。形状错误在任何 Profile 下都是硬错误。
2. **第二层 · 单字段安全**：密钥强度（≥32 字符、非占位符）、禁止明文密码、Redis 密码必填。
3. **第三层 · 跨字段**：跨域回跳/Cookie 需要强 `SESSION_EXCHANGE_SECRET`；`STEP_UP_ENABLED` 需要 `STEP_UP_PATHS` + `TRUSTED_PROXIES`；TLS 客户端证书/私钥必须成对。
4. **第四层 · 跨服务**：Stargate→Herald 鉴权必须收敛到唯一显式模式；生产拒绝仅 API Key 且禁止 HMAC v1。

每条结论带有稳定 `code`（如 `HERALD_PII_PEPPER_WEAK`）。用 `--json` 输出可脚本化：

```bash
./suite validate --profile production --json   # 存在任一 error 结论即非零退出
```

生产始终 strict（不可被 `--strict=false` 绕过）；`--strict` 会把 test/dev 的结论也提升为硬错误。

## v1 核心镜像与 HMAC v2

- **镜像（来自 `components.yaml` 单一来源）**：Stargate `v1.0.0`、Warden `v1.1.0`、Herald `v1.1.0`；可选通道服务统一为 `v1.1.0`。`env-meta.yaml`、`.env.example`、`compose/canonical`、`ports.yaml` 与 `internal/contract` 漂移测试全部以清单为准。
- **Stargate 端口 80 → 8080**：容器端口、Host 映射、Traefik `loadbalancer.server.port` 与 `forwardauth.address` 全部改为 `8080`。
- **健康/就绪路径**：Stargate liveness `/healthz` + readiness `/readyz`；Warden `/healthcheck`；Herald `/healthz`。`make health`、`.github/workflows/ci.yml`、`scripts/run-e2e.sh` 均探测新路径。
- **Herald 显式认证**：显式设置 `REQUEST_AUTH_MODE=hmac_v2`（不再隐式在 API Key/HMAC 间选择）。仅配置单个 `HMAC_SECRET`（无 `HERALD_HMAC_KEYS`）时 Herald 解析出隐式 `default` key id，客户端可省略 `X-Key-Id`。
- **全面 HMAC v2、v1 关闭**：策略层固定 `HMAC_V1_ENABLED=false`（Herald）与 `WARDEN_HMAC_ALLOW_V1=false`（Warden），从不接受可重放的 v1 规范串。
- **Herald 测试验证码独立监听器**：`/v1/test/code` 仅在独立的 loopback-only 监听器（`HERALD_TEST_LISTENER_ADDR`，默认 `127.0.0.1:8092`）上提供，由 `HERALD_TEST_API_KEY` 守护；主 `:8082` 监听器从不暴露验证码。E2E 通过 `docker compose exec herald curl`（`HERALD_COMPOSE_DIR`）访问。
- **E2E 签名迁移到 HMAC v2**：规范串 `HERALD-HMAC-V2\n<METHOD>\n<path>\n<query>\n<ts>\n<nonce>\n<service>\n<keyid>\n<sha256(body)>` 与 `herald/internal/auth/hmac_v2.go`（v1.1.0）完全一致，并与上游 `CanonicalRequest.Canonical()` 字段顺序交叉验证。原先的 `X-API-Key` 正向测试被反转为负向测试，断言 `hmac_v2` 下 API Key 会被拒绝。

## 命令

```bash
./suite validate   # 校验 config 是否可加载
./suite serve     # Web UI，http://localhost:8085（-port 或 SERVE_PORT）
```

使用 `make gen`（原生 CLI）生成 compose，或通过 Web UI 进行交互式配置。

参见 [../README.zh-CN](../README.zh-CN.md) · [../compose/README.zh-CN](../compose/README.zh-CN.md)。
