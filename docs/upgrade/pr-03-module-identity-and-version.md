# PR 3 · `refactor/module-identity-and-version`

> Phase B · 建立单一契约来源 · 无组件版本变更 · 可独立回滚

## 目标

统一模块标识与版本注入：把 module 从 `github.com/soulteary/the-gate` 改为 `github.com/soulteary/stargate-suite`，新增 `version` 命令与 ldflags 变量，使二进制携带版本/commit/构建时间与已验证组件组合。

## 允许修改文件

- `go.mod`（module 行）
- 所有 `.go` 文件的 import 路径（`cmd/suite/*.go`、`internal/composegen/*.go`、`e2e/*.go`）
- `cmd/suite/main.go`（新增 `version` 子命令与 ldflags 变量）
- `Dockerfile`（build args 注入 ldflags）
- `.github/workflows/release.yml`（版本注入）
- `README.md`、`README.zh-CN.md`

## 禁止修改

- 组件镜像版本、端口、健康路径、认证。
- 配置生成语义。
- 三个参考仓库。

## 实现要点（真实待改点）

1. **`go.mod` 第 1 行** `module github.com/soulteary/the-gate` 改为 `github.com/soulteary/stargate-suite`。
2. 全仓库 import 路径批量替换 `github.com/soulteary/the-gate` → `github.com/soulteary/stargate-suite`（影响 `cmd/suite/`、`internal/composegen/`、`e2e/`）。
3. **`cmd/suite/main.go`** 增加 `version` 子命令，输出：Suite 版本、Commit、构建时间、Go 版本、已验证组件组合。
4. 新增 ldflags 变量（如 `var Version, Commit, BuildDate string`），由构建阶段注入。
5. **`Dockerfile`（第 11-15 行）** 已有 `VERSION/COMMIT/BUILD_DATE` ARG 但当前 `go build -ldflags "-w -s"` 未注入这些值——改为注入 `-X` ldflags。
6. **`.github/workflows/release.yml`** 与 `ci.yml` 的构建步骤同步注入 ldflags（`ci.yml` 第 222 行 `LDFLAGS="-s -w"` 未注入版本）。
7. 确认没有外部 Go 包消费 `github.com/soulteary/the-gate`；如存在，README 增加迁移说明。

## 验证命令

```bash
go build ./cmd/suite
go run ./cmd/suite version
go test -race $(go list ./... | grep -v '/e2e$')
go vet ./...
git grep -n "soulteary/the-gate"   # 应为空
git diff --check
```

## 验收标准

- 全仓库无残留旧 module 路径；
- `suite version` 正确输出版本、commit、构建时间、Go 版本与组件组合；
- Release/CI 构建注入版本信息；
- 非 E2E 测试通过。

## 回滚方式

`git revert` 本 PR；module 改名为单一提交单元，回滚后 import 一致回退。
