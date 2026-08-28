# Migration to v0.10 (v0.9 → v0.10)

[English](#english) | [中文](#中文)

> This release performs an **atomic upgrade to the v1 service contracts**. The
> changes below are breaking. `config/components.yaml` is the single source of
> truth for versions, ports, and health paths — the values documented here are
> derived from it, not hard-coded elsewhere.

---

## English

### Pinned component combination

| Service | v0.9 baseline (drifted) | v0.10 |
|---------|-------------------------|-------|
| Stargate | `v0.9.2` | **`v1.0.0`** |
| Warden | `v0.10.0` | **`v1.0.0`** |
| Herald | `v0.6.1` | **`v1.1.0`** |
| herald-dingtalk | `:latest` (drift) | `v0.5.0` |

> Warden's highest stable upstream tag is `v1.0.0` — there is **no `v1.1.0`**.
> The Warden v1 contract (HMAC v2, `/healthcheck`, `WARDEN_HMAC_ALLOW_V1`,
> `ENVIRONMENT`) already shipped in `v1.0.0`. Always take versions from
> `config/components.yaml`.

### Breaking changes

1. **Container port `80` → `8080`.** Stargate now listens on `8080` inside the
   container. It still has no host port and is reached only via Traefik. Update
   any custom Traefik service/loadbalancer port and any direct probes.
2. **Health paths.**
   - Stargate: `/health` → **`/healthz`** (liveness) and **`/readyz`** (readiness).
   - Warden: `/health` → **`/healthcheck`**.
   - Herald: **`/healthz`**.
3. **Herald requires explicit request auth.** `REQUEST_AUTH_MODE=hmac_v2` is now
   set explicitly; there is no implicit fallback between API key and HMAC.
4. **HMAC v1 disabled by default.** `HMAC_V1_ENABLED=false`. Only enable it for a
   controlled migration window; re-disable once callers use v2.
5. **Redis password is mandatory.** The generated Redis services require
   `HERALD_REDIS_PASSWORD` / `WARDEN_REDIS_PASSWORD` (compose fails fast if unset).
6. **Internal services are not exposed by default.** In segmented/production
   output, Warden/Herald and the Redis instances are on internal networks and do
   not publish host ports; only the Traefik edge is reachable.

### Deprecated / renamed fields

- Stargate `PORT=80` → `PORT=8080`.
- Health-check probes targeting `/health` on Stargate/Warden must move to the new
  paths above.
- Herald implicit-auth expectations (auto API key or HMAC) → explicit
  `REQUEST_AUTH_MODE`.

### Migration steps

1. Pull the new images from `config/components.yaml` (`make gen` regenerates
   compose/env from the manifest — do not hand-edit versions).
2. Set the required secrets (`HERALD_REDIS_PASSWORD`, `WARDEN_REDIS_PASSWORD`,
   `HERALD_HMAC_SECRET`, API keys). For `production` supply real secrets via env
   or `--set KEY=VALUE`; the strict validator rejects placeholders.
3. Regenerate: `go run ./cmd/suite generate --profile <profile>` (or `make gen`).
4. Validate: `go run ./cmd/suite validate --profile <profile> --strict`.
5. Diagnose the generated compose: `go run ./cmd/suite doctor --compose <path> --probe`.
6. Bring services up and confirm the new health paths respond.

### Troubleshooting

- **Health checks fail after upgrade:** you are probably still probing `/health`
  or port `80` — switch to `/healthz` + `/readyz` on `8080` (Warden: `/healthcheck`).
- **Herald returns 401/unauthorized:** the caller is still using HMAC v1 or an
  implicit API key. Configure HMAC v2 (or temporarily set `HMAC_V1_ENABLED=true`
  for the migration window only).
- **Compose refuses to start (Redis):** set `HERALD_REDIS_PASSWORD` /
  `WARDEN_REDIS_PASSWORD`.
- Use `suite doctor` for image↔manifest drift, published-port and health-probe
  diagnostics.

---

## 中文

### 锁定的组件组合

| 服务 | v0.9 基线（已漂移） | v0.10 |
|------|--------------------|-------|
| Stargate | `v0.9.2` | **`v1.0.0`** |
| Warden | `v0.10.0` | **`v1.0.0`** |
| Herald | `v0.6.1` | **`v1.1.0`** |
| herald-dingtalk | `:latest`（漂移） | `v0.5.0` |

> Warden 上游最高稳定 Tag 为 `v1.0.0`，**不存在 `v1.1.0`**。其 v1 契约
> （HMAC v2、`/healthcheck`、`WARDEN_HMAC_ALLOW_V1`、`ENVIRONMENT`）已在
> `v1.0.0` 落地。版本一律以 `config/components.yaml` 为准。

### 破坏性变更

1. **容器端口 `80` → `8080`。** Stargate 容器内改监听 `8080`，仍无宿主端口、
   仅经 Traefik 暴露。请同步更新自定义 Traefik service/loadbalancer 端口与直连探针。
2. **健康路径。**
   - Stargate：`/health` → **`/healthz`**（存活）+ **`/readyz`**（就绪）。
   - Warden：`/health` → **`/healthcheck`**。
   - Herald：**`/healthz`**。
3. **Herald 需显式请求鉴权。** 现显式设置 `REQUEST_AUTH_MODE=hmac_v2`，不再在
   API Key 与 HMAC 间隐式回退。
4. **HMAC v1 默认关闭。** `HMAC_V1_ENABLED=false`。仅在受控迁移窗口临时开启，
   调用方切到 v2 后立即关闭。
5. **Redis 密码必填。** 生成的 Redis 服务要求 `HERALD_REDIS_PASSWORD` /
   `WARDEN_REDIS_PASSWORD`（未设置时 compose 直接失败）。
6. **内部服务默认不暴露。** 在分段/生产输出中，Warden/Herald 与各 Redis 位于
   内部网络、不发布宿主端口，仅 Traefik edge 可达。

### 废弃 / 重命名字段

- Stargate `PORT=80` → `PORT=8080`。
- 针对 Stargate/Warden `/health` 的健康探针需迁移到上述新路径。
- Herald 隐式鉴权（自动 API Key 或 HMAC）→ 显式 `REQUEST_AUTH_MODE`。

### 迁移步骤

1. 按 `config/components.yaml` 拉取新镜像（`make gen` 会从清单重新生成
   compose/env——请勿手改版本）。
2. 设置必需机密（`HERALD_REDIS_PASSWORD`、`WARDEN_REDIS_PASSWORD`、
   `HERALD_HMAC_SECRET`、各 API Key）。`production` 需经环境变量或 `--set KEY=VALUE`
   提供真实机密，strict 校验器会拒绝占位值。
3. 重新生成：`go run ./cmd/suite generate --profile <profile>`（或 `make gen`）。
4. 校验：`go run ./cmd/suite validate --profile <profile> --strict`。
5. 诊断生成的 compose：`go run ./cmd/suite doctor --compose <path> --probe`。
6. 启动服务并确认新健康路径正常响应。

### 故障排查

- **升级后健康检查失败：** 多半仍在探测 `/health` 或端口 `80`——切到 `8080` 上的
  `/healthz` + `/readyz`（Warden 为 `/healthcheck`）。
- **Herald 返回 401/未授权：** 调用方仍用 HMAC v1 或隐式 API Key。配置 HMAC v2
  （或仅在迁移窗口临时 `HMAC_V1_ENABLED=true`）。
- **compose 无法启动（Redis）：** 设置 `HERALD_REDIS_PASSWORD` /
  `WARDEN_REDIS_PASSWORD`。
- 用 `suite doctor` 排查镜像↔清单漂移、暴露端口与健康探针。
