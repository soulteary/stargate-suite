中文 | [English](README.md)

# stargate-suite

**Stargate + Warden + Herald** 三服务的端到端集成测试环境：多种 Compose、CLI/Web UI 配置生成、50+ E2E 测试（正常流程、异常、鉴权、幂等、审计、监控）。可选：**herald-totp**、**herald-dingtalk**、**herald-smtp**。

Go 模块：`github.com/soulteary/the-gate`。仓库名：**stargate-suite**。

## 文档

| 文档 | 说明 |
|------|------|
| [README.zh-CN](README.zh-CN.md) | 本文件 — 总览与快速开始 |
| [compose/README.zh-CN](compose/README.zh-CN.md) | Compose 用法；[EN](compose/README.md) |
| [config/README.zh-CN](config/README.zh-CN.md) | Web UI 与 gen 配置；[EN](config/README.md) |
| [compose/traefik/README.zh-CN](compose/traefik/README.zh-CN.md) | Traefik 三合一/三分开；[EN](compose/traefik/README.md) |
| [e2e/README.zh-CN](e2e/README.zh-CN.md) | E2E 测试；[EN](e2e/README.md) |
| [MANUAL_TESTING.zh-CN](MANUAL_TESTING.zh-CN.md) | 浏览器手动验证；[EN](MANUAL_TESTING.md) |

## 结构

```
stargate-suite/
├── compose/example/   # image | build
├── compose/canonical/ # 单一数据源 → 生成 traefik / 三分开
├── build/             # 生成输出（gen 或 Web UI）
├── config/            # page.yaml, presets.json
├── cmd/suite/         # CLI + Web UI
├── e2e/               # E2E 测试
├── fixtures/warden/   # 测试用户 data.json
└── scripts/run-e2e.sh
```

## 快速开始

**前置：** Docker 与 Compose、Go 1.25+、约 1GB 磁盘。

**生成并启动：**

```bash
make gen
make up
# 或：make up-build | make up-traefik
```

**CLI：** `go run ./cmd/suite help` — `gen`、`gen-split`、`serve`。  
**Web UI：** `go run ./cmd/suite serve`（默认 http://localhost:8085）。无鉴权，仅限本地。

**测试：**

```bash
./scripts/run-e2e.sh
# 或：make test-wait && go test -v ./e2e/...
```

**停止：** `make down`（或 `make clean` 清理卷）。

## 端口与环境变量

- **Stargate** 8080 · **Warden** 8081 · **Herald** 8082 · **Redis** 6379
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

常用：`make gen`，`make up` / `make up-image` / `make up-build` / `make up-traefik`，`make down`，`make ps`，`make logs`，`make test-wait`，`make health`，`make serve`，`make suite-build`。

## 服务简述

- **Stargate：** forwardAuth、会话、登录流程。`GET /_auth`，`POST /_send_verify_code`，`POST /_login`
- **Warden：** 白名单用户查询。`GET /user?phone=...|mail=...|user_id=...`
- **Herald：** OTP 创建/验证/撤销、限流、审计。`POST /v1/otp/challenges`，`POST /v1/otp/verifications`，`GET /v1/test/code/{id}`（测试模式）
- **herald-totp（可选）：** TOTP 双因素。在 Stargate 中设置 `HERALD_TOTP_ENABLED=true` 及 base URL/API key。

完整登录示例见 [MANUAL_TESTING.zh-CN](MANUAL_TESTING.zh-CN.md)。

## 故障排查

- **无法启动：** `lsof -i :8080 -i :8081 -i :8082 -i :6379`，`make logs`，`make health`
- **测试失败：** 确认 `make ps`、`make health`；`go test -v ./e2e/...`；限流由测试清理 Redis；锁定检查 Redis 清理
- **收不到验证码：** 确保 `HERALD_TEST_MODE=true`，查 Herald 日志
- **Redis：** 测试清理要求 localhost:6379；`redis-cli -h localhost -p 6379 ping`

## 开发

- 测试数据：改 `fixtures/warden/data.json` 后 `make restart-warden`
- 新测试：在 `e2e/` 下添加，使用 `ensureServicesReady(t)` 与 `test_helpers.go`
- 本地构建：`make up-build`，再重新构建/重启
- 代码检查：`golangci-lint run --max-same-issues=100000`

## 许可证

与主项目一致。
