# Release

[English](#english) | [中文](#中文)

---

## English

### Trigger

`release.yml` runs on a semver tag push (`vX.Y.Z`) or a controlled
`workflow_dispatch` (pass an existing tag). Non-semver refs are refused, and the
tag commit must be an ancestor of `main`.

### What it produces

- `-trimpath` binaries for linux / darwin / windows on amd64 + arm64, with
  version/commit/build-date injected via ldflags.
- A stable, self-excluding `checksums.txt`.
- A multi-arch (amd64 + arm64) container image, **Trivy-scanned before push**
  (CRITICAL/HIGH fail the release).
- An SPDX **SBOM**.
- A `components.lock.yaml` snapshot whose tags match `components.yaml` and
  whose images are resolved to immutable `sha256` digests.
- Keyless **Cosign** signatures for the image and for `checksums.txt`.
- GitHub **build-provenance attestations** for the image and the binaries.

### Tagging policy

- Stable tags update `latest` and the `{major}` alias.
- Pre-release tags (e.g. `v0.10.0-rc.1`) never move `latest`; the release is
  marked `prerelease`.

### Release notes

The release body is extracted from the matching section of `CHANGELOG.md`
(`## [x.y.z]`). Keep the changelog current — add entries under `[Unreleased]`
and promote them on release.

### Permissions & pinning

Workflows default to `permissions: contents: read`; only the release job
elevates (`contents`, `packages`, `id-token`, `attestations` write). All
third-party actions are pinned to commit SHAs (semantic version kept in a
trailing comment).

### Controlled re-run

Use `workflow_dispatch` with an existing tag to re-run a release safely (same
ancestry/semver checks apply). Do not push force tags or trigger two release
flows for the same tag.

---

## 中文

### 触发

`release.yml` 在推送语义化 tag（`vX.Y.Z`）或受控 `workflow_dispatch`（传入已存在的
tag）时运行。非语义化 ref 会被拒绝，且 tag 对应 commit 必须是 `main` 的祖先。

### 产物

- linux / darwin / windows 的 amd64 + arm64 `-trimpath` 二进制，经 ldflags 注入
  版本/commit/构建时间。
- 稳定且自排除的 `checksums.txt`。
- 多架构（amd64 + arm64）容器镜像，**推送前经 Trivy 扫描**（CRITICAL/HIGH 使发布失败）。
- SPDX **SBOM**。
- 与 `components.yaml` 标签一致、且所有镜像均解析为不可变 `sha256` digest 的
  `components.lock.yaml` 快照。
- 镜像与 `checksums.txt` 的 keyless **Cosign** 签名。
- 镜像与二进制的 GitHub **构建来源证明（attestation）**。

### 打标签策略

- stable tag 更新 `latest` 与 `{major}` 别名。
- 预发布 tag（如 `v0.10.0-rc.1`）不移动 `latest`，发布标记为 `prerelease`。

### 发布正文

发布正文取自 `CHANGELOG.md` 中匹配版本的段落（`## [x.y.z]`）。请保持 changelog
更新——在 `[Unreleased]` 下记录，发布时升格。

### 权限与固定

workflow 顶层默认 `permissions: contents: read`，仅发布 job 提权
（`contents`、`packages`、`id-token`、`attestations` 写）。所有第三方 Action 固定到
commit SHA（语义版本保留在行尾注释）。

### 受控重跑

用 `workflow_dispatch` 传入已存在 tag 可安全重跑发布（同样做祖先/语义化校验）。
不要强推 tag，也不要对同一 tag 触发两套发布流程。
