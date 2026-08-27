# PR 13 · `release/sbom-signing-provenance`

> Phase D · 工具与交付质量 · 可独立回滚

## 目标

强化发布供应链：固定 Action 到 Commit SHA、生成 SBOM、Trivy 扫描、GitHub attestation、Cosign 签名，修正 stable `latest` 判断，从 CHANGELOG 提取 Release 内容。修复问题 **R-01**、**R-02**。

## 允许修改文件

- `.github/workflows/release.yml`
- `.github/workflows/*`（Action SHA 固定、permissions 收敛）
- `Dockerfile`（如需多阶段/scan 友好）
- `CHANGELOG.md`（若存在，或新增）
- `README.md`、`README.zh-CN.md`

## 禁止修改

- 组件版本与运行契约。
- 三个参考仓库。

## 实现要点（真实待改点，对照源计划 10.4-10.5）

1. **所有第三方 Action 固定完整 Commit SHA**（当前 `ci.yml`/`release.yml` 使用 `actions/checkout@v6`、`actions/setup-go@v6`、`docker/build-push-action@v6` 等浮动 tag），注释保留语义版本；不使用 `latest` Action；不使用 self-hosted runner。
2. **Workflow 顶层默认 `permissions: contents: read`**；仅发布 Job 提升 `contents: write`、`packages: write`、`id-token: write`、`attestations: write`。
3. **发布流程**：支持 Tag 与受控 `workflow_dispatch`；验证 Tag Commit 属于 `main`；从 `go.mod` 读取 Go 版本；运行 lint、govulncheck、race、完整 E2E；构建 Linux/macOS/Windows amd64/arm64；`-trimpath` + 注入 version/commit/build date（承接 PR 3 ldflags）。
4. **`checksums.txt`** 排序稳定且不包含自身。
5. **镜像 amd64/arm64 构建并扫描后再推送**；**stable Tag 才更新 `latest`，预发布 Tag 不更新**（修复 R-01）。
6. 生成 **SBOM**、**GitHub artifact attestation**、**keyless Cosign 签名**（镜像与 checksum）。
7. Release 正文使用 CHANGELOG 对应版本内容；正式产物进入 GitHub Release，不长期保存重复 artifact。
8. Docker、Go 各自缓存，禁止缓存构建产物形成重复上传；避免同一 Tag 触发两套发布流程。

> 注意：本地验证不应实际执行付费发布 workflow；未经明确指示不得创建 Release 或推送镜像。

## 验证命令

```bash
actionlint || true
go run ./cmd/suite version        # 确认 ldflags 注入
# 干跑：本地构建镜像与 checksum，不推送
docker build -t stargate-suite:local .
git diff --check
```

## 验收标准

- 所有 Action 固定 SHA，permissions 最小化；
- 发布产物含 checksum、SBOM、签名、attestation；
- stable/latest 判断正确；
- Release 正文来自 CHANGELOG；
- 发布流程支持受控重跑。

## 回滚方式

`git revert` 本 PR，恢复原发布流程。
