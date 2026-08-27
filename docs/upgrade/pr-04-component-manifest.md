# PR 4 · `refactor/component-manifest`

> Phase B · 建立单一契约来源 · **暂不改变运行行为**（仍登记旧版本）· 可独立回滚

## 目标

新增 `config/components.yaml` 作为组件版本、镜像、端口、健康路径、契约版本的**单一权威来源**，并由加载模型驱动其余配置。修复问题 **M-01**：消除版本事实散落在 `env-meta.yaml`、`.env.example`、`compose/canonical`、`build/*` 的漂移（见 [00-overview](00-overview.md) 第 1 节偏差证据）。

## 允许修改文件

- `config/components.yaml`（新增）
- `config/components.lock.yaml`（新增，或由生成产生）
- `internal/contract/*.go`（新增：清单加载与版本契约模型）
- `internal/composegen/*.go`（改为从清单读取端口/健康路径/默认镜像）
- `config/env-meta.yaml`、`config/ports.yaml`（改为引用清单，或加漂移测试约束）
- 漂移测试（新增）
- `README.md`、`README.zh-CN.md`、`config/README.md`、`config/README.zh-CN.md`

## 禁止修改

- **不立即升级镜像到 v1**：清单先登记当前旧版本。
- 端口、健康路径、认证的**运行**语义（本 PR 只做「来源统一」，不做「值变更」）。
- 三个参考仓库。

## 实现要点（真实待改点）

1. **先登记当前旧版本**，不改变运行行为：`stargate: v0.11.0`、`warden: v0.13.0`、`herald: v0.9.0`（与权威源 `config/env-meta.yaml` 第 172-174 行、`.env.example` 一致），`contractVersion` 暂标为对应旧契约。
2. 清单同时登记当前真实端口/健康路径（迁移前）：Stargate 容器端口 `80`、liveness `/health`；Warden `8081`、`/health`；Herald `8082`、`/healthz`（来自 `compose/canonical/docker-compose.yml` 与 `config/ports.yaml`）。
3. 由清单提供端口、健康路径、默认镜像给 `composegen`，取代散落的硬编码。
4. **增加漂移测试**：断言 `env-meta.yaml`、`.env.example`、`compose/canonical`、`build/*` 中的核心镜像版本与端口均来自清单，禁止在别处重新硬编码；此测试应能直接捕捉当前 `build/*` 旧版本（Stargate `v0.9.2` / Warden `v0.10.0` / Herald `v0.6.1`）与 `herald-dingtalk:latest` 的漂移。
5. 生成 `components.lock.yaml`（记录镜像 digest 的占位结构，正式发布时填充）。

## 验证命令

```bash
go test -race $(go list ./... | grep -v '/e2e$')
go run ./cmd/suite validate
go vet ./...
# 生成后核对 build/* 版本与清单一致（若本 PR 触发重新生成）
docker compose -f build/image/docker-compose.yml config
git diff --check
```

## 验收标准

- `config/components.yaml` 成为端口/健康路径/默认镜像的唯一来源；
- 漂移测试通过，且能在版本被重新硬编码时失败；
- 运行行为与本 PR 之前一致（旧版本、旧端口）；
- `components.lock.yaml` 结构就绪。

## 回滚方式

`git revert` 本 PR；因未改变运行值，回滚不影响现有部署。
