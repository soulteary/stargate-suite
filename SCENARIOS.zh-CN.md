中文 | [English](SCENARIOS.md)

# 场景预设（Scenarios）

本文件对应 `config/scenarios.json`，用于按场景生成 compose 与 `.env`。

## 使用方式

场景预设目前通过 **Web UI** 选择。在 Web UI（`go run ./cmd/suite serve`）第一步先选择**部署 Profile**（`development` / `test` / `production`），再选择场景预设（S1–S5）；生成器会按该场景的 modes/options/env 填充，并叠加 Profile 的安全与运行策略，最终生成 compose，在「回顾」步骤下载或复制即可。CLI 支持 Profile 与显式 modes，但目前没有 `--scenario` 参数。

若只需生成默认模式集合（image、build、traefik 等）而不指定场景，可执行 `make gen`（原生 CLI），或用 `suite generate --profile <profile> --output <目录>`。

## 部署 Profile（策略，而非单纯预设）

Profile 来自 `config/profiles.yaml`，由共享的 `internal/policy` 模型应用（CLI 与 Web UI 一致）。关键差异：

| 策略 | development | test | production |
|---|---|---|---|
| 端口绑定 | loopback（`127.0.0.1`） | loopback（`127.0.0.1`） | 仅反向代理入口（内部服务/Redis 不发布宿主端口） |
| 密钥 | 自动生成或输入 | 隔离的确定性测试值 | 必须用户提供 / secret file |
| 密码算法 | 允许 plaintext | 测试密码 | 禁止 plaintext |
| Cookie Secure | 可选 | 可选 | 必须开启 |
| HMAC v1 | 禁止 | 禁止 | 禁止 |
| 校验 | warning + error | strict | strict（错误为硬错误，不可绕过） |

`production` 初期为实验特性，但其严格规则均为真实错误：弱/测试密钥、plaintext 密码、发布内部端口、Cookie Secure 关闭、启用 HMAC v1 都会阻止生成。请通过进程环境变量或 `--set KEY=VALUE` 提供真实密钥。dev/test 若需字节稳定输出，可传 `--seed <种子>`（仅限 dev/test，切勿用作真实种子）。

## 场景列表

| Scene ID | 名称 | 一句话描述 | 适用场景 |
|---|---|---|---|
| `s1-solo-gate` | S1 Solo Gate | 仅 Stargate 本地账号认证，最少依赖，快速上线。 | 内网、小规模、临时环境 |
| `s2-solo-gate-session-redis` | S2 Solo Gate + Session Redis | Stargate + Redis 会话，提升多实例一致性。 | 多副本 Stargate、滚动升级 |
| `s3-gate-warden` | S3 Gate + Warden | 引入白名单与用户目录，认证与身份源解耦。 | 需要统一用户来源/禁用控制 |
| `s4-gate-warden-herald` | S4 Gate + Warden + Herald | OTP 主链路完整拆分，Stargate 专注 session。 | 生产推荐架构 |
| `s5-gate-warden-herald-plugins` | S5 Gate + Warden + Herald Plugins | 在 S4 基础上启用 SMTP/SMS/DingTalk/TOTP 插件。 | 多渠道通知与企业集成 |

## 设计说明

- 场景通过 `modes + options + envOverrides` 三部分组合实现。
- `modes` 决定生成哪些 compose（如 `traefik` 或 `traefik-stargate`）。
- `options` 控制 compose 结构能力（如 `includeSmtp`、`includeTotp`、`stargateSessionRedisUseBuiltin`）。
- `options.disableWardenRedisService=true` 时，生成结果会从 `traefik`/`traefik-warden` compose 中移除 `warden-redis` 服务。
- `envOverrides` 用于写入 `.env` 的默认覆盖值。
- 可选文案字段：`nameZh/nameEn`、`descriptionZh/descriptionEn`、`riskNoteZh/riskNoteEn`，用于 Web UI 多语言展示。

## 注意事项

- `canonical` 是 compose 生成的基础模板（`compose/canonical/docker-compose.yml`），不作为 Web UI 场景选项。
- 场景预设提供的是“可运行起点”，生产前请替换密钥、域名与 API 凭据。
- 如果你有自定义部署，可在场景生成结果上二次调整 `.env` 再启动。
