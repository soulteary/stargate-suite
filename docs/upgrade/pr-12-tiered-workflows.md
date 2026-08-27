# PR 12 · `ci/tiered-workflows`

> Phase D · 工具与交付质量 · 可独立回滚

## 目标

将 CI 重构为 PR / main / nightly / tag 四层，控制 GitHub Actions 消耗，同时保证主分支与发布标签的完整验证。修复 PR 快速反馈缺失与成本问题。

## 允许修改文件

- `.github/workflows/ci.yml`（拆为 PR / main 两层或多文件）
- `.github/workflows/nightly.yml`（新增）
- `.github/workflows/release.yml`（tag 层，签名/SBOM 归 PR 13）
- `.github/workflows/*`（path filter、concurrency）

## 禁止修改

- 组件版本与运行契约。
- 三个参考仓库。

## 实现要点（真实待改点，对照源计划 10.1-10.3）

1. **当前 `ci.yml` 为单层**：`test` → `e2e` / `docker-build` / `build`（push 与 PR 全触发）。拆分为：
   - **PR 层（5-8 分钟反馈）**：变更范围检测；gofmt/vet/golangci-lint（`.golangci.yml` 已存在）；Go 版本契约测试；非 E2E 单元测试 + race；覆盖率门槛；schema/config/golden test；`docker compose config`；**仅当组件清单/canonical Compose/E2E/认证代码变化时**跑 smoke E2E；**文档修改不跑容器构建与 E2E**；`concurrency` 取消同 PR 旧运行。
   - **main 层**：PR 全部检查 + 完整 E2E + 故障恢复测试 + Linux 容器构建 + Trivy 扫描 + govulncheck；覆盖率上传失败不作为唯一质量门槛；artifact 保留 3-7 天。
   - **nightly/手工层**：稳定组合 + 最近候选组合 + Redis 已验证/下一主版本 + TOTP/SMTP/DingTalk 场景 + amd64/arm64；Windows/macOS 仅构建 CLI，不跑 Docker E2E。
2. 增加 `paths` / `paths-ignore` 过滤：文档-only 变更跳过重活。
3. 增加 `concurrency` 组，取消同一 PR/分支的旧运行。
4. 保留现有 `os` 三平台构建矩阵（`ci.yml` 第 190 行），但 Go 版本用 `go-version-file: go.mod`（承接 PR 1）。

## 验证命令

```bash
# 本地静态验证 workflow（如安装 actionlint）
actionlint || true
# 逻辑验证：文档-only PR 不应触发 e2e/docker-build（通过 paths filter 断言）
git diff --check
```

## 验收标准

- PR 层 5-8 分钟给出主要反馈，文档 PR 不跑容器/E2E；
- main 层完整 E2E + 故障恢复 + 扫描；
- nightly 覆盖版本矩阵与可选场景；
- concurrency 生效，不使用 self-hosted runner。

## 回滚方式

`git revert` 本 PR，恢复单层 CI。
