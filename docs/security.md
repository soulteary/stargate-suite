# Security

[English](#english) | [中文](#中文)

---

## English

### Service-to-service auth (Herald)

- `REQUEST_AUTH_MODE=hmac_v2` is set **explicitly**; there is no implicit fallback
  between API key and HMAC.
- **HMAC v1 is disabled by default** (`HMAC_V1_ENABLED=false`). Enable it only for
  a controlled migration window and disable it again afterwards.
- HMAC v2 uses a signing secret (`HERALD_HMAC_SECRET`) with a bounded clock drift
  (`HMAC_MAX_DRIFT`, default 60s) to resist replay.

### Test verification-code endpoint

The test code endpoint runs on a **separate listener** (`HERALD_TEST_LISTENER_ADDR`)
guarded by a dedicated key (`HERALD_TEST_API_KEY`). The main listener can never
expose test codes. `HERALD_TEST_MODE` defaults to `false`.

### Secrets & Redis

- Redis instances require a password (`HERALD_REDIS_PASSWORD` /
  `WARDEN_REDIS_PASSWORD`); compose fails fast if unset.
- PII/idempotency key derivation (`HERALD_PII_PEPPER`, `HERALD_IDEMPOTENCY_SECRET`)
  should use strong values in production.
- The `production` profile is strict: plaintext passwords, placeholder/test keys,
  published internal ports, Cookie Secure off, or HMAC v1 are **hard errors** and
  cannot be bypassed. Supply real secrets via process env or `--set KEY=VALUE`;
  never commit `.env`, generated `build/`, or production credentials.

### Network segmentation

Generated production/segmented output replaces a flat network with
purpose-scoped internal networks (auth vs. each service's Redis vs. channel
adapters). Internal services do not publish host ports; only the Traefik edge is
reachable. A compromised channel adapter cannot reach the Warden data plane.

### Web UI

`suite serve` binds `127.0.0.1:8085` (loopback) by default. Off-host exposure is
opt-in and always authenticated:

- `--listen 0.0.0.0:8085 --allow-remote` refuses to start without `--allow-remote`.
- Remote mode requires an access token (auto-generated and printed if `--token`
  is omitted).
- State-changing POSTs are Origin/CSRF-checked.
- Cookies are HttpOnly + SameSite=Strict (Secure on off-loopback).
- Operator secrets are dropped from the server session after artifacts return.
- No silent port switching — a busy port is a hard error.

CLI-generated `.env` files contain deployment secrets and are written with
owner-only `0600` permissions. Values are Compose-dotenv encoded so `$`, `#`,
quotes, whitespace, and newlines cannot be reinterpreted as interpolation or
additional assignments. Keep the generated directory out of source control.

### Supply chain

Third-party GitHub Actions are pinned to commit SHAs; workflows default to
`permissions: contents: read`. Releases produce an SBOM, a Trivy-scanned
multi-arch image, keyless Cosign signatures (image + checksums), and GitHub
build-provenance attestations. See [release.md](release.md).

---

## 中文

### 服务间鉴权（Herald）

- **显式**设置 `REQUEST_AUTH_MODE=hmac_v2`，不在 API Key 与 HMAC 间隐式回退。
- **HMAC v1 默认关闭**（`HMAC_V1_ENABLED=false`）。仅在受控迁移窗口开启，之后关闭。
- HMAC v2 使用签名密钥（`HERALD_HMAC_SECRET`）并限制时钟漂移
  （`HMAC_MAX_DRIFT`，默认 60s）以抗重放。

### 测试验证码端点

测试验证码端点运行在**独立监听器**（`HERALD_TEST_LISTENER_ADDR`）上，由专用密钥
（`HERALD_TEST_API_KEY`）守护。主监听器永不暴露测试码。`HERALD_TEST_MODE` 默认为
`false`。

### 机密与 Redis

- Redis 需密码（`HERALD_REDIS_PASSWORD` / `WARDEN_REDIS_PASSWORD`），未设置时
  compose 直接失败。
- PII/幂等派生密钥（`HERALD_PII_PEPPER`、`HERALD_IDEMPOTENCY_SECRET`）在生产应
  使用强随机值。
- `production` profile 为 strict：明文口令、占位/测试密钥、暴露内部端口、
  Cookie Secure 关闭、HMAC v1 均为**硬错误**，不可绕过。真实机密经环境变量或
  `--set KEY=VALUE` 提供；切勿提交 `.env`、生成的 `build/` 或生产凭据。

### 网络分段

生产/分段输出以按用途隔离的内部网络取代扁平网络（鉴权、各服务 Redis、通道
适配器彼此隔离）。内部服务不发布宿主端口，仅 Traefik edge 可达。被攻陷的通道
适配器无法触及 Warden 数据面。

### Web UI

`suite serve` 默认绑定 `127.0.0.1:8085`（loopback）。对外暴露需显式开启且始终鉴权：

- `--listen 0.0.0.0:8085 --allow-remote`，缺 `--allow-remote` 则拒绝启动。
- 远程模式需 access token（未传 `--token` 则自动生成并打印）。
- 变更类 POST 做 Origin/CSRF 校验。
- Cookie 为 HttpOnly + SameSite=Strict（非 loopback 时 Secure）。
- 生成后从服务端会话清除机密，不落盘。
- 无静默切换端口——端口占用即硬错误。

CLI 生成的 `.env` 包含部署机密，文件权限固定为仅所有者可读写的 `0600`。值会按
Compose dotenv 规则编码，避免 `$`、`#`、引号、空白和换行被重新解释为插值或额外
赋值。不要将生成目录提交到版本库。

### 供应链

第三方 GitHub Action 固定到 commit SHA；workflow 顶层默认
`permissions: contents: read`。发布产出 SBOM、经 Trivy 扫描的多架构镜像、keyless
Cosign 签名（镜像 + checksum）与 GitHub 构建来源证明。详见 [release.md](release.md)。
