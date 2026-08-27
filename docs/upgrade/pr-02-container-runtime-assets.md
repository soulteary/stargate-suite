# PR 2 · `fix/container-runtime-assets`

> Phase A · 恢复可工作基线 · 无组件版本变更 · 可独立回滚

## 目标

确保 Release 二进制和容器不依赖仓库源码目录即可运行 `serve` / `validate` / `generate`：把默认配置与 canonical Compose 使用 `go:embed` 嵌入二进制，容器镜像自包含。修复问题 **B-02**。

## 允许修改文件

- `Dockerfile`
- `cmd/suite/*.go`（新增 embed 入口，如 `cmd/suite/assets.go`）
- `internal/composegen/*.go`（读取资产处改为可回退到嵌入资产）
- 新增 embed 与只读文件系统的测试
- `README.md`、`README.zh-CN.md`（运行方式说明）
- 可能新增 `//go:embed` 指令引用的资产目录（如把 `config/`、`compose/canonical/` 通过 embed 打包）

## 禁止修改

- 组件镜像版本、端口、健康路径、认证语义。
- Web UI 的功能行为（仅改资产加载来源，不改交互）。
- 三个参考仓库。

## 实现要点（真实待改点）

1. **当前 `Dockerfile`（第 20 行）只 `COPY --from=builder /app/stargate-suite`**，未包含 `config/` 与 `compose/`。运行时 `serve`/`validate` 依赖磁盘上的 `config/*.yaml`，容器内不存在，故默认入口不可独立工作。
2. 使用 `go:embed` 把 `config/`（`env-meta.yaml`、`services.yaml`、`ports.yaml`、`config-sections.yaml`、`page.yaml`、`providers.yaml`、`scenarios.json`、`presets.json`、`keys-step.yaml`、`i18n/`）与 `compose/canonical/docker-compose.yml` 嵌入二进制。
3. 增加 `--config-dir` 标志，允许外部目录覆盖嵌入资产；未指定时使用嵌入资产。
4. 修正 `Dockerfile`：runtime stage 不再需要挂载源码；确认 `curl` 等 healthcheck 依赖（当前 runtime 为 `alpine:3.22`，仅装了 `ca-certificates`）。
5. 增加容器启动 smoke test：`docker run` 不挂载仓库即可打开 Web UI 并调用生成 API。
6. 增加只读根文件系统运行测试（`read_only: true` 场景，写入落到 tmpfs）。

## 验证命令

```bash
go test -race $(go list ./... | grep -v '/e2e$')
go build ./cmd/suite
docker build -t stargate-suite:local .
docker run --rm -p 8085:8085 stargate-suite:local &   # 不挂载仓库
curl -sf http://127.0.0.1:8085/ >/dev/null            # Web UI 可访问
go run ./cmd/suite validate --config-dir=/nonexistent  # 回退到嵌入资产
git diff --check
```

## 验收标准

- `docker run` 不挂载仓库即可打开 Web UI 并调用生成 API；
- CLI `validate` 可使用嵌入资产；
- 外部 `--config-dir` 覆盖仍然可用；
- 镜像不包含源码和测试数据；
- 嵌入资产有启动测试与版本 checksum。

## 回滚方式

`git revert` 本 PR；由于未改运行契约，回滚后仍可用挂载方式运行旧镜像。
