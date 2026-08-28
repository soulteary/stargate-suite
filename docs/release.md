# Release

[English](#english) | [中文](#中文)

---

## English

### Trigger

`release.yml` runs on a strict SemVer tag push (`vX.Y.Z`, with optional valid
pre-release/build metadata) or a controlled `workflow_dispatch` (pass an
existing tag). Malformed tags and leading-zero numeric identifiers are refused,
the tag commit must be an ancestor of `main`, and that exact commit must have a
successful `Main` workflow run.

### What it produces

- `-trimpath` binaries for linux / darwin / windows on amd64 + arm64, with
  version/commit/build-date injected via ldflags.
- A stable, self-excluding `checksums.txt`.
- A multi-arch (amd64 + arm64) container image, with **each architecture
  Trivy-scanned before push** (CRITICAL/HIGH fail the release).
- An SPDX **SBOM**.
- A `components.lock.yaml` snapshot whose tags match `components.yaml` and
  whose images are resolved to immutable `sha256` digests.
- Keyless **Cosign** signatures for the image and for `checksums.txt`.
- GitHub **build-provenance attestations** for the image and the binaries.

### Deploy with immutable images

Download `components.lock.yaml` from the same GitHub Release as the suite
binary, then pass it to generation:

```bash
go run ./cmd/suite generate --profile development --output build/locked \
  --lock ./components.lock.yaml
```

Production uses the same flag in addition to its required secret inputs. The
command rejects placeholder digests or a lock whose image tags/schema do not
match the embedded component manifest. Locked image references override image
values from the environment or `--set`.

### Tagging policy

- Stable tags update `latest` and the `{major}` alias.
- Pre-release tags (e.g. `v0.10.0-rc.1`) never move `latest`; the release is
  marked `prerelease`.

### Release notes

The release body is extracted from the matching section of `CHANGELOG.md`
(`## [x.y.z]`, exact match). A stable release fails if the section is missing;
pre-releases may use a short generated notice. Keep the changelog current — add
entries under `[Unreleased]` and promote them on release.

### Permissions & pinning

Workflows default to `permissions: contents: read`; only the release job
elevates (`contents`, `packages`, `id-token`, `attestations` write). All
third-party actions are pinned to commit SHAs (semantic version kept in a
trailing comment). `govulncheck` is also pinned to an explicit module version.

### Controlled re-run

Use `workflow_dispatch` with an existing tag to re-run a release safely (same
ancestry/semver checks apply). Do not push force tags or trigger two release
flows for the same tag.

---

## 中文

### 触发

`release.yml` 在推送严格 SemVer tag（`vX.Y.Z`，可带合法预发布/构建元数据）或受控
`workflow_dispatch`（传入已存在 tag）时运行。格式错误或数字前导零的 tag 会被拒绝，
tag 对应 commit 必须是 `main` 的祖先，并且该 commit 的 `Main` workflow 必须成功。

### 产物

- linux / darwin / windows 的 amd64 + arm64 `-trimpath` 二进制，经 ldflags 注入
  版本/commit/构建时间。
- 稳定且自排除的 `checksums.txt`。
- 多架构（amd64 + arm64）容器镜像，**两个架构均在推送前经 Trivy 扫描**
  （CRITICAL/HIGH 使发布失败）。
- SPDX **SBOM**。
- 与 `components.yaml` 标签一致、且所有镜像均解析为不可变 `sha256` digest 的
  `components.lock.yaml` 快照。
- 镜像与 `checksums.txt` 的 keyless **Cosign** 签名。
- 镜像与二进制的 GitHub **构建来源证明（attestation）**。

### 使用不可变镜像部署

从同一个 GitHub Release 下载 `components.lock.yaml` 与 suite 二进制，并在生成时
传入锁文件：

```bash
go run ./cmd/suite generate --profile development --output build/locked \
  --lock ./components.lock.yaml
```

production 同样使用该参数，并额外提供所需机密。命令会拒绝空 digest，或
image tag/schema 与内嵌组件清单不匹配的锁；锁定的镜像引用优先于环境变量和
`--set` 中的镜像值。

### 打标签策略

- stable tag 更新 `latest` 与 `{major}` 别名。
- 预发布 tag（如 `v0.10.0-rc.1`）不移动 `latest`，发布标记为 `prerelease`。

### 发布正文

发布正文取自 `CHANGELOG.md` 中精确匹配版本的段落（`## [x.y.z]`）。稳定版缺少该
段落时发布失败；预发布版本可使用简短的自动说明。请保持 changelog 更新——在
`[Unreleased]` 下记录，发布时升格。

### 权限与固定

workflow 顶层默认 `permissions: contents: read`，仅发布 job 提权
（`contents`、`packages`、`id-token`、`attestations` 写）。所有第三方 Action 固定到
commit SHA（语义版本保留在行尾注释）。
`govulncheck` 同样固定到明确的 module 版本。

### 受控重跑

用 `workflow_dispatch` 传入已存在 tag 可安全重跑发布（同样做祖先/语义化校验）。
不要强推 tag，也不要对同一 tag 触发两套发布流程。
