中文 | [English](README.md)

# stargate-suite

**Stargate + Warden + Herald** 三服务的端到端集成测试环境：多种 Compose、Web UI 配置生成、50+ E2E 测试（正常流程、异常、鉴权、幂等、审计、监控）。可选：**herald-totp**、**herald-dingtalk**、**herald-smtp**。

Go 模块：`github.com/soulteary/stargate-suite`。仓库名：**stargate-suite**。

> **模块改名：** Go module 路径已由 `github.com/soulteary/the-gate` 改为 `github.com/soulteary/stargate-suite`。本模块承载内部工具（`cmd/suite`），并非用于对外导入的库；若有外部代码引用了旧路径，请将其 import 更新为新路径。

## 文档

| 文档 | 说明 |
|------|------|
| [README.zh-CN](README.zh-CN.md) | 本文件 — 总览与快速开始 |
| [SCENARIOS.zh-CN](SCENARIOS.zh-CN.md) | 场景预设（scene）用法与场景说明 |
| [compose/README.zh-CN](compose/README.zh-CN.md) | Compose 用法；[EN](compose/README.md) |
| [config/README.zh-CN](config/README.zh-CN.md) | Web UI 配置；[EN](config/README.md) |
| [compose/traefik/README.zh-CN](compose/traefik/README.zh-CN.md) | Traefik 三合一/三分开；[EN](compose/traefik/README.md) |
| [e2e/README.zh-CN](e2e/README.zh-CN.md) | E2E 测试；[EN](e2e/README.md) |

## 结构

```
stargate-suite/
├── compose/example/   # 可选；image | build 由 canonical 生成
├── compose/canonical/ # 单一数据源 → CLI / Web UI / make gen
├── build/             # 生成输出（make gen 经 CLI，或 CLI / Web UI）
├── config/            # page.yaml, scenarios
├── cmd/suite/         # Web UI（serve）+ generate + validate + doctor
├── e2e/               # E2E 测试
├── fixtures/warden/   # 测试用户 data.json
└── scripts/run-e2e.sh
```

## 快速开始

**前置：** Docker 与 Compose、Go 1.27+、约 1GB 磁盘。

**生成并启动：**

```bash
make gen    # 由 CLI 原生生成到 build/（不启动 Web 服务、无需 jq）
make up
# 或：make up-build | make up-traefik
```

**CLI：** `go run ./cmd/suite help` — `generate`、`validate`、`doctor`、`serve`。默认配置与 canonical compose 已通过 `go:embed` **嵌入二进制**，因此各子命令无需仓库源码目录即可运行（如 release 二进制或容器内）。可用 `--config-dir=<目录>`（或环境变量 `CONFIG_DIR`）以磁盘目录覆盖内置 `config/`；覆盖目录中缺失的文件会回退到嵌入资产。`generate` 与 `validate` 调用与 Web UI `/api/generate` **完全相同**的 `internal/composegen` / `internal/policy` 函数，因此 CLI 无需启动 Web 服务即可产出字节一致的结果。

**部署 Profile**（`config/profiles.yaml`）：`development` / `test` / `production` 是**安全与运行策略**，而非单纯预填表单——涵盖端口绑定、密钥来源、密码算法、Herald 认证、Redis 密码、Cookie Secure、HMAC v1、容器权限与校验模式（见 [docs/upgrade/00-overview.md](docs/upgrade/00-overview.md) §5.4）。CLI 与 Web UI 共用同一模型（`internal/policy` + `internal/composegen`）：

```bash
# development：当前默认（loopback 端口、plaintext 测试密码、dev 密钥）。
# --seed 让自动生成的 dev 密钥字节稳定（仅限 dev/test，切勿用作真实种子）。
go run ./cmd/suite generate --profile development --output build/dev --seed pr5-golden

# production 为实验特性且 STRICT：plaintext 密码、测试/占位密钥、
# 发布内部端口、Cookie Secure 关闭、启用 HMAC v1 均为硬错误（不可绕过）。
# 通过进程环境变量或可重复的 --set KEY=VALUE 提供真实密钥：
go run ./cmd/suite generate --profile production --output build/prod \
  --set PASSWORDS=bcrypt:... --set HERALD_API_KEY=... --set WARDEN_API_KEY=... \
  --set HERALD_HMAC_SECRET=... --set HERALD_REDIS_PASSWORD=... --set WARDEN_REDIS_PASSWORD=...

# 校验某个 Profile 的策略；production/test 为 strict（错误而非警告）：
go run ./cmd/suite validate --profile production --strict

# 对已生成的 compose 做只读诊断（解析、镜像↔manifest 漂移、宿主端口与本地占用、
# 网络；加 --probe 主动探测 liveness/readiness）：
go run ./cmd/suite doctor --compose build/dev/docker-compose.yml

# 每个子命令都支持 --json，便于 CI / Cursor 解析；退出码稳定
#（0 成功，校验失败或 doctor 硬失败时非零）。
```

配置生成也可经 **Web UI**（第一步选择 Profile）或 `make gen`（原生 CLI，不启动 Web 服务）。

**Web UI：** `go run ./cmd/suite serve` **默认绑定 `127.0.0.1:8085`**（仅本地回环，本机无需鉴权）。对外暴露需显式开启且强制鉴权：`serve --listen 0.0.0.0:8085 --allow-remote`——未加 `--allow-remote` 会拒绝启动；remote 模式下必须携带 access token（未传 `--token` 时自动生成并打印）。状态变更 POST 会做 Origin/CSRF 校验，Cookie 为 HttpOnly + SameSite=Strict（非回环时置 Secure），且生成产物返回后即从服务端会话中清除运维密钥。监听端口被占用会直接报错，**绝不静默换端口**。

**容器（自包含，无需挂载源码）：**

```bash
docker build -t stargate-suite:local .
docker run --rm -p 8085:8085 stargate-suite:local        # Web UI，无需挂载仓库
docker run --rm --read-only --tmpfs /tmp -p 8085:8085 stargate-suite:local  # 只读根文件系统
```

**测试：**

```bash
./scripts/run-e2e.sh
# 或：make test-wait && go test -v ./e2e/...
```

**停止：** `make down`（或 `make clean` 清理卷）。

## 端口与环境变量

- **Stargate**：无宿主端口——`stargate` 服务 `ports: []`，容器内监听后端端口 **80**，仅通过 Traefik 暴露（见 `compose/canonical/docker-compose.yml` 与 `config/ports.yaml`）。
- **Warden** 8081 · **Herald** 8082 · **Herald-TOTP** 8084 · **Herald-DingTalk** 8083 · **Herald-SMTP** 8085 · **Redis** 6379（仅在端口被暴露/映射时占用宿主端口）。
- **Web UI** 默认 **8085**（`make serve`），与 **herald-smtp** 的默认端口相同。默认场景不会启动 herald-smtp，因此开箱即用无冲突；但若启用 herald-smtp 且同一台机器上同时执行 `make serve`，两者端口会冲突——需改其一（如用 `SERVE_PORT` 改 Web UI 端口，或改 herald-smtp 宿主端口）。
- 复制 `.env.example` 为 `.env` 可覆盖镜像版本、`AUTH_HOST`、`PASSWORDS`、`WARDEN_API_KEY`、`HERALD_API_KEY`、`HERALD_HMAC_SECRET`。

## 测试用户（fixtures/warden/data.json）

| 角色 | 手机号 | 邮箱 | User ID |
|------|--------|------|---------|
| Admin | 13800138000 | admin@example.com | test-admin-001 |
| User | 13900139000 | user@example.com | test-user-002 |
| Guest | 13700137000 | guest@example.com | test-guest-003 |
| Inactive | 13600136000 | inactive@example.com | test-inactive-004 |
| Rate-limit | 13500135000 | ratelimit@example.com | test-ratelimit-005 |

## 测试套件

50+ 用例：正常登录、异常（错误/过期/锁定验证码、非白名单、非活跃、限流、服务宕机、鉴权）、Herald/Warden API、幂等、审计、Provider、指标。  
单测：`go test -v ./e2e/... -run TestCompleteLoginFlow`

## Makefile（见 `make help`）

常用：`make gen`（原生 CLI）、`make up` / `make up-image` / `make up-build` / `make up-traefik`，`make down`，`make ps`，`make logs`，`make test-wait`，`make health`，`make serve`，`make suite-build`。

## 服务简述

- **Stargate：** forwardAuth、会话、登录流程。`GET /_auth`，`POST /_send_verify_code`，`POST /_login`
- **Warden：** 白名单用户查询。`GET /user?phone=...|mail=...|user_id=...`
- **Herald：** OTP 创建/验证/撤销、限流、审计。`POST /v1/otp/challenges`，`POST /v1/otp/verifications`，`GET /v1/test/code/{id}`（测试模式）
- **herald-totp（可选）：** TOTP 双因素。Stargate 仅设置 `HERALD_TOTP_ENABLED=true`；在 Herald 中配置 `HERALD_TOTP_BASE_URL` 与 API key，由 Herald 代理至 herald-totp。

完整登录流程由 e2e 测试覆盖，见 [e2e/README.zh-CN](e2e/README.zh-CN.md)。

## 故障排查

- **无法启动：** `lsof -i :8081 -i :8082 -i :6379`（Stargate 无宿主端口，用 `make health` 检查），`make logs`，`make health`
- **测试失败：** 确认 `make ps`、`make health`；`go test -v ./e2e/...`；限流由测试清理 Redis；锁定检查 Redis 清理
- **收不到验证码：** 确保 `HERALD_TEST_MODE=true`，查 Herald 日志
- **Redis：** 测试清理要求 localhost:6379；`redis-cli -h localhost -p 6379 ping`

## 开发

- 测试数据：改 `fixtures/warden/data.json` 后 `make restart-warden`
- 新测试：在 `e2e/` 下添加，使用 `ensureServicesReady(t)` 与 `test_helpers.go`
- 本地构建：`make up-build`，再重新构建/重启
- 代码检查：`golangci-lint run --max-same-issues=100000`

## 发布与供应链

CI 分层：`ci.yml`（PR 快速反馈）、`main.yml`（完整 E2E + 镜像构建 + Trivy 扫描）、
`nightly.yml`（多架构 + 跨 OS）。所有第三方 GitHub Action 均固定到 commit SHA；
workflow 顶层默认 `permissions: contents: read`。

`release.yml` 在推送语义化 tag（`vX.Y.Z`）或受控 `workflow_dispatch` 重跑时触发。
它以 `-trimpath` 构建 linux/darwin/windows 的 amd64+arm64 二进制，发布多架构镜像，
并生成：SBOM（SPDX）、签名的 `checksums.txt`（keyless Cosign）、镜像的 keyless Cosign
签名、以及 GitHub 构建来源证明（attestation）。仅 stable tag 更新 `latest`；
预发布 tag（如 `v0.10.0-rc.1`）不会移动 `latest`。Release 正文取自 `CHANGELOG.md`
中对应版本的段落。

## 许可证

与主项目一致。
