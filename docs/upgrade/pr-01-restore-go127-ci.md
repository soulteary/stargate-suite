# PR 1 · `fix/restore-go127-ci`

> Phase A · 恢复可工作基线 · 无运行契约变更 · 可独立回滚

## 目标

恢复当前主分支 CI：让 `go.mod`（Go 1.27）与 CI、Dockerfile、README 的 Go 版本一致，使依赖下载与构建重新通过。**不升级任何组件镜像。**

## 允许修改文件

- `.github/workflows/ci.yml`
- `.github/workflows/release.yml`
- `Dockerfile`
- `README.md`、`README.zh-CN.md`
- Go 版本契约测试（新增，如 `internal/contract/goversion_test.go` 或 `cmd/suite` 下的测试）

## 禁止修改

- 任何组件镜像版本（`config/env-meta.yaml`、`.env.example`、`compose/canonical/*`、`build/*` 的镜像 tag 一律不动）。
- 端口、健康路径、认证、Compose 语义。
- 三个参考仓库。

## 实现要点（真实待改点）

1. **`.github/workflows/ci.yml` 三处硬编码 `1.25` 全部消除**：
   - `env.GO_VERSION: '1.25'`（第 10 行）；
   - `test` job `strategy.matrix.go-version: ['1.25']`（第 20 行）；
   - `build` job `strategy.matrix.go-version: ['1.25']`（第 189 行）。
   - `e2e` job 的 `setup-go` 使用 `${{ env.GO_VERSION }}`（第 76 行），改用 `go-version-file: go.mod` 后可移除 `GO_VERSION`。
   - 建议全部 `actions/setup-go` 改为 `go-version-file: go.mod`，删除 matrix 中的 Go 版本维度（保留 `os` 维度）。
2. **`Dockerfile` 第 2 行** `FROM golang:1.25-alpine3.22` 改为 Go 1.27 的 builder（如 `golang:1.27-alpine3.22`，验证 tag 可用）。这是问题 **B-01** 的核心。
3. **`README.md` / `README.zh-CN.md`** 前置条件中的 Go 版本要求更新为 1.27。
4. **`.github/workflows/release.yml`** 若存在硬编码 Go 版本，改为从 `go.mod` 读取。
5. **新增契约测试**：解析 `go.mod` 的 `go` 指令，断言 `Dockerfile`、`ci.yml`、`release.yml`、README 中出现的 Go 版本不低于 `go.mod`，防止再次漂移。
6. 暂不触碰 `ci.yml` 第 111 行的 `8080/health` / `8081/health` / `8082/healthz` 探测（属于 PR 8 端口/健康迁移范围）。

## 验证命令

```bash
go mod download
go fmt ./...
go vet ./...
go test -race $(go list ./... | grep -v '/e2e$')
go build ./cmd/suite
git diff --check
```

（PR 1 不因旧 E2E 尚未迁移而被阻塞，但非 E2E 测试与三平台构建必须通过。）

## 验收标准

- `go mod download` 成功；
- 非 E2E 单元测试通过；
- Linux/macOS/Windows 构建通过；
- 新增 Go 版本契约测试通过，能捕捉「文档/Workflow/Dockerfile 低于 go.mod」；
- CI 全绿。

## 回滚方式

单独 `git revert` 本 PR 即可，不改变任何运行配置和协议。
