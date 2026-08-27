# 第一批任务（First Batch）：PR 1-4，暂不升级镜像

> 对应源计划第 18 节。目标：先得到一个**可构建、可运行、可回滚**的基线，再进入 Profile、Compose 安全和 v1/HMAC v2 原子迁移。

## 原则

**不要立即修改三个核心镜像版本。** 第一批只做基线恢复与单一来源建立，运行组合仍保持旧版本。三镜像升级到 Stargate 1.0.0 / Warden 1.1.0 / Herald 1.1.0 属于 [PR 8](pr-08-upgrade-core-images-and-hmac-v2.md)，须在第一批全绿后原子进行。

## 执行顺序

| 顺序 | PR | 主题 | 是否改运行行为 | 工作包 |
|---|---|---|---|---|
| 1 | PR 1 | 修复 Go 1.27 CI 契约 | 否（仅构建/CI/文档） | [pr-01](pr-01-restore-go127-ci.md) |
| 2 | PR 2 | 修复容器运行资产（go:embed 自包含） | 否（仅资产加载来源） | [pr-02](pr-02-container-runtime-assets.md) |
| 3 | PR 3 | 统一 module 与版本注入 | 否（module 改名 + version 命令） | [pr-03](pr-03-module-identity-and-version.md) |
| 4 | PR 4 | 建立组件清单，但暂保持旧镜像 | 否（先登记旧版本，来源统一） | [pr-04](pr-04-component-manifest.md) |

## 各步关键真实待改点速览

1. **PR 1**：`go.mod` 为 `go 1.27.0`，但 `Dockerfile` 用 `golang:1.25-alpine3.22`、`ci.yml` 三处硬编码 `1.25`（`env.GO_VERSION` 第 10 行、`test.matrix` 第 20 行、`build.matrix` 第 189 行）。改为 `go-version-file: go.mod` 并删除 1.25，加 Go 版本契约测试。
2. **PR 2**：`Dockerfile` 只复制二进制，`serve`/`validate` 依赖的 `config/`、`compose/` 未进镜像。用 `go:embed` 打包并支持 `--config-dir` 覆盖。
3. **PR 3**：module `github.com/soulteary/the-gate` → `github.com/soulteary/stargate-suite`，加 `version` 子命令与 ldflags（`Dockerfile` 已有 `VERSION/COMMIT/BUILD_DATE` ARG 但未注入）。
4. **PR 4**：新增 `config/components.yaml`，**先登记当前旧版本**（Stargate `v0.11.0`、Warden `v0.13.0`、Herald `v0.9.0`）与当前端口/健康路径，并加漂移测试。此测试应能捕捉当前 `build/*` 已漂移的旧值（Stargate `v0.9.2` / Warden `v0.10.0` / Herald `v0.6.1`、`herald-dingtalk:latest`）。

## 第一批完成的判定

- PR 1-4 CI 全绿；
- `go mod download`、非 E2E 单元测试、三平台构建通过；
- 容器不挂载源码即可运行 `serve`/`validate`；
- `suite version` 输出版本信息；
- 组件清单成为端口/健康/默认镜像单一来源，漂移测试通过；
- **运行组合仍是旧版本**（未进入 v1 升级）。

达成后，方进入 [PR 5](pr-05-deployment-profiles.md)（Profile）、[PR 6](pr-06-compose-network-and-redis.md)（Compose 安全）与 [PR 8](pr-08-upgrade-core-images-and-hmac-v2.md)（v1/HMAC v2 原子迁移）。
