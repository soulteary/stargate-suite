中文 | [English](SCENARIOS.md)

# 场景预设（Scenarios）

本文件对应 `config/scenarios.json`，用于按场景生成 compose 与 `.env`。

## 使用方式

场景生成**仅支持 Web UI**。在 Web UI（`go run ./cmd/suite serve`）第一步选择场景预设（S1–S5），生成器会按该场景的 modes/options/env 填充并生成 compose，在「回顾」步骤下载或复制即可。

若只需生成默认模式集合（image、build、traefik 等）而不指定场景，可执行 `make gen`（经 Web API）。

无 CLI 的 `gen "scene:<id>"` 或 `make gen-scenarios`，场景产物仅通过 Web UI 生成。

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
