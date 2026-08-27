# PR 14 · `docs/v010-migration-and-operations`

> Phase D · 工具与交付质量 · 收官文档 · 可独立回滚

## 目标

同步中英文文档，覆盖 v0.9 → v0.10 迁移、部署、安全、开发与发布，校正项目名/module/CLI/Web UI 能力与示例版本。

## 允许修改文件

- `README.md`、`README.zh-CN.md`
- `SCENARIOS.md`、`SCENARIOS.zh-CN.md`
- `docs/migration-v0.10.md`（新增）
- `docs/deployment.md`、`docs/security.md`、`docs/development.md`、`docs/release.md`（新增，中英文）
- `docs/upgrade/*`（如需回填最终事实）
- `compose/README*.md`、`config/README*.md`、`e2e/README*.md`
- `PLAN.md`（收官：更新或指向发布状态）

## 禁止修改

- 代码与运行契约（本 PR 纯文档）。
- 三个参考仓库。

## 实现要点（真实待改点，对照源计划 11 · PR 14）

1. **更新中英文 README**：Go 版本要求 1.27（承接 PR 1）、module 名 `github.com/soulteary/stargate-suite`（承接 PR 3）、CLI 原生 generate/validate/doctor 能力（承接 PR 10，删除「只能 Web API 生成」的旧表述）。
2. **新增 `docs/migration-v0.10.md`**：说明端口 80→8080、健康 `/health`→`/healthz` + `/readyz`、Warden `/health`→`/healthcheck`、HMAC v1→v2、Herald 显式 `REQUEST_AUTH_MODE`、旧字段 deprecated 列表、Redis 密码闭环、内部服务不再默认暴露端口。
3. **新增 deployment / security / development / release 文档**（中英文）。
4. **补充故障诊断**（对照 `doctor` 与故障矩阵）。
5. **校正示例版本**：所有文档示例镜像统一为 Stargate 1.0.0 / Warden 1.1.0 / Herald 1.1.0；清理任何遗留的 `v0.9.2`/`v0.10.0`/`v0.6.1`/`:latest` 示例。
6. **填充或收敛 `PLAN.md`**：本轮已用 `PLAN.md` 作为升级索引；发布时更新其状态或指向已完成的迁移文档。

## 验证命令

```bash
# 文档链接/示例版本核对（可用脚本或人工）
git grep -n "the-gate"                 # 应无残留旧 module 引用
git grep -nE "stargate:v0\.|warden:v0\.|herald:v0\." docs README* SCENARIOS*  # 应无旧版本示例
git diff --check
```

## 验收标准

- 中英文 README、迁移、部署、安全、开发、发布文档与实际行为一致；
- 无残留旧 module 名与旧版本示例；
- 迁移文档覆盖端口/健康/HMAC/字段/Redis/端口暴露变化；
- `PLAN.md` 状态收敛。

## 回滚方式

`git revert` 本 PR（纯文档，回滚无运行影响）。
