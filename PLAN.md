# Stargate Suite 升级路线图（v0.10.0 → v1.0.0）

> 本文件是升级路线的**索引与总览**。逐 PR 可执行工作包、现状基线、架构与验证命令，见 [`docs/upgrade/`](docs/upgrade/)。
>
> 本轮目标：把 Suite 收敛为「Stargate 体系的版本锁定、配置生成、集成契约验证与可复现部署工具」，不把 Suite 扩展成长期运行的控制平面。
>
> **状态（v0.10 收官）：** PR 1-14 已实现落地。运行契约已原子迁移到 v1（Stargate `v1.0.0` / Warden `v1.0.0` / Herald `v1.1.0`，端口 8080，健康 `/healthz`+`/readyz`，HMAC v2、v1 默认关闭）。版本/端口/健康路径唯一来源为 [`config/components.yaml`](config/components.yaml)。运维文档见 [迁移](docs/migration-v0.10.md)、[部署](docs/deployment.md)、[安全](docs/security.md)、[开发](docs/development.md)、[发布](docs/release.md)。
> 注：Warden 上游最高稳定 Tag 为 `v1.0.0`（无 `v1.1.0`），文档与清单均以此为准。

## 现状基线（已核对真实仓库）

- 模块名为 `github.com/soulteary/the-gate`，`go.mod` 声明 `go 1.27.0`。
- `Dockerfile` 使用 `golang:1.25-alpine3.22`，与 `go.mod` 的 1.27 冲突（问题 B-01）。
- `.github/workflows/ci.yml` 三处硬编码 `1.25`（`env.GO_VERSION`、`test.matrix.go-version`、`build.matrix.go-version`）。
- **权威配置源**（`config/env-meta.yaml`、`.env.example`、`compose/canonical/docker-compose.yml`）默认镜像为 Stargate `v0.11.0` / Warden `v0.13.0` / Herald `v0.9.0`。
- **已生成的 `build/*` 输出已漂移**，默认镜像仍是旧版本：Stargate `v0.9.2` / Warden `v0.10.0` / Herald `v0.6.1`，且 `build/traefik` 中 `herald-dingtalk` 仍指向 `:latest`（这是 M-01「一个事实多处维护」的直接证据）。
- 配置分散在 `config/{env-meta,ports,services,config-sections,page,providers,scenarios,presets,keys-step}.*`，尚无 `components.yaml` / `profiles.yaml` / `schemas/`（问题 M-01）。
- `make gen` 依赖 `scripts/gen-via-api.sh`（Web API 生成）；`make health` 仍访问 Stargate `:80/health`（旧端口）。
- 三个稳定契约来源（只读）：Stargate `v1.0.0`、Warden `v1.1.0`、Herald `v1.1.0`。

## 14 个 PR 与 4 个 Phase

### Phase A：恢复可工作基线（PR 1-2）

| PR | 分支名 | 主题 | 工作包 |
|---|---|---|---|
| PR 1 | `fix/restore-go127-ci` | 恢复 Go 1.27 CI 契约 | [pr-01](docs/upgrade/pr-01-restore-go127-ci.md) |
| PR 2 | `fix/container-runtime-assets` | 容器运行资产自包含 | [pr-02](docs/upgrade/pr-02-container-runtime-assets.md) |

### Phase B：建立单一契约来源（PR 3-5）

| PR | 分支名 | 主题 | 工作包 |
|---|---|---|---|
| PR 3 | `refactor/module-identity-and-version` | 统一 module 名与版本注入 | [pr-03](docs/upgrade/pr-03-module-identity-and-version.md) |
| PR 4 | `refactor/component-manifest` | 建立组件清单 `components.yaml`（旧版本先登记） | [pr-04](docs/upgrade/pr-04-component-manifest.md) |
| PR 5 | `feat/deployment-profiles` | development/test/production 三类 Profile | [pr-05](docs/upgrade/pr-05-deployment-profiles.md) |

### Phase C：安全与 v1 契约迁移（PR 6-9）

| PR | 分支名 | 主题 | 工作包 |
|---|---|---|---|
| PR 6 | `security/compose-network-and-redis` | 网络拆分、内部服务不暴露、Redis 密码闭环 | [pr-06](docs/upgrade/pr-06-compose-network-and-redis.md) |
| PR 7 | `feat/v1-config-contracts` | 增加 v1 字段与跨字段/跨服务校验 | [pr-07](docs/upgrade/pr-07-v1-config-contracts.md) |
| PR 8 | `feat/upgrade-core-images-and-hmac-v2` | **原子**：升级三镜像 + 端口 + 健康路径 + HMAC v2 | [pr-08](docs/upgrade/pr-08-upgrade-core-images-and-hmac-v2.md) |
| PR 9 | `test/v1-failure-contracts` | 失败/重放/故障恢复契约测试 | [pr-09](docs/upgrade/pr-09-v1-failure-contracts.md) |

### Phase D：工具与交付质量（PR 10-14）

| PR | 分支名 | 主题 | 工作包 |
|---|---|---|---|
| PR 10 | `feat/cli-generate-validate-doctor` | CLI 原生 generate/validate/doctor，Makefile 改调 CLI | [pr-10](docs/upgrade/pr-10-cli-generate-validate-doctor.md) |
| PR 11 | `security/local-web-ui` | Web UI loopback、CSRF、secret 掩码 | [pr-11](docs/upgrade/pr-11-local-web-ui.md) |
| PR 12 | `ci/tiered-workflows` | PR/main/nightly/tag 四层 CI | [pr-12](docs/upgrade/pr-12-tiered-workflows.md) |
| PR 13 | `release/sbom-signing-provenance` | 固定 Action SHA、SBOM、签名、证明 | [pr-13](docs/upgrade/pr-13-sbom-signing-provenance.md) |
| PR 14 | `docs/v010-migration-and-operations` | 中英文迁移/部署/安全/发布文档 | [pr-14](docs/upgrade/pr-14-docs-migration.md) |

## 依赖关系

```text
PR1 ─┐
PR2 ─┴─▶ PR3 ─▶ PR4 ─▶ PR5 ─▶ PR6 ─▶ PR7 ─▶ PR8 ─▶ PR9 ─▶ PR10 ─▶ PR11 ─▶ PR12 ─▶ PR13 ─▶ PR14
```

- **PR 1、PR 2** 是一切的前置：CI 未恢复前不做任何契约迁移（原则 1）。
- **PR 4（组件清单）** 是 PR 5/PR 8 的单一数据源前置；PR 8 的默认版本必须来自清单（原则 3）。
- **PR 8 是本轮最关键的原子 PR**：镜像升级、端口 80→8080、健康路径迁移、HMAC v2 与 E2E 签名迁移必须同一 PR 完成、同一单元回滚，禁止拆成可独立合并的两个 PR。
- **PR 9** 依赖 PR 8 落地的 v1 运行行为。
- **第一批只执行 PR 1-4**，全绿后再进入 PR 5 起的较大改造，详见 [first-batch](docs/upgrade/first-batch.md)。

## 三道验收门槛

| 门槛 | 关键条件（详见 [00-overview](docs/upgrade/00-overview.md) 与源计划第 13 节） |
|---|---|
| **v0.10.0-rc.1** | CI 恢复；Release 二进制/容器可独立运行；组件清单与三类 Profile 建立；三镜像完成 v1/v1.1 升级；HMAC v2 正常/失败/重放测试通过；Redis 密码闭环；三类 Profile 均通过 `docker compose config`；登录/健康/readiness 测试通过；迁移文档覆盖端口/健康/HMAC 变化 |
| **v0.10.0** | main 完整 E2E 稳定；故障恢复测试通过；production 无固定密钥/明文密码/test mode/内部宿主机端口；strict validator 覆盖主要跨字段规则；CLI generate/validate/doctor 稳定；Web UI 默认 loopback；Release 含 checksum/SBOM/签名/证明；中英文文档同步；无 P0/P1 缺陷 |
| **v1.0.0** | v0.10.x 真实使用无破坏性问题；schema 与 CLI 输出封版；组件清单升级流程至少验证一次；可选 TOTP/SMTP/DingTalk 核心场景通过；兼容与废弃策略入档；发布与回滚演练完成 |

## 执行约束

- 只修改 `soulteary/stargate-suite`；Stargate/Warden/Herald 稳定 Tag 为只读契约来源。
- 每个 PR 单一主题、可独立 revert；不在迁移 PR 中混入无关格式化或前端重构（原则 8）。
- Cursor 执行规则见 [`.cursor/rules/stargate-suite-upgrade.mdc`](.cursor/rules/stargate-suite-upgrade.mdc)，每 PR 提示词模板见 [cursor-prompt-template](docs/upgrade/cursor-prompt-template.md)。
- 未经明确指示，不得 commit、push、merge、重跑付费 workflow 或创建 Release。
