中文 | [English](README.md)

# E2E 测试

用例结构与运行方式。总览见 [../README.zh-CN.md](../README.zh-CN.md)。

## 结构

- e2e_test.go — 正常流程
- error_scenarios_test.go — 错误/过期/锁定验证码、非白名单、非活跃、限流、服务宕机、鉴权、边界
- v1_failure_contracts_test.go — PR9 v1 契约测试：存活/就绪为独立端点、HMAC v2 nonce 重放被拒、Herald 测试验证码端点不在公开监听器暴露
- auth_test.go、herald_api_test.go、warden_api_test.go、idempotency_test.go、audit_test.go、provider_test.go、metrics_test.go
- test_helpers.go — ensureServicesReady、sendVerificationCodeWithError、loginWithError、clearRateLimitKeys、停止/启动 Docker，以及 HMAC v2 签名助手（signHeraldV2 / signHeraldReq / signHeraldV2Fixed）

## 服务间鉴权（HMAC v2）

Herald 运行在 `REQUEST_AUTH_MODE=hmac_v2`（test/production 姿态），因此所有 Herald
`/v1/otp/*` 调用均使用 HMAC v2 签名，而非 `X-API-Key`。签名助手构造规范串
`HERALD-HMAC-V2\n<METHOD>\n<path>\n<query>\n<ts>\n<nonce>\n<service>\n<keyid>\n<sha256(body)>`，
与 `herald/internal/auth/hmac_v2.go` 完全一致。`heraldHMACSecret` 必须与 test
profile 注入的 `HMAC_SECRET` 一致。API Key 请求预期被**拒绝**（见
`TestHeraldAPIKeyRejectedUnderHMACV2`）。

## Herald 测试验证码

`/v1/test/code` 仅在 Herald 独立的 loopback-only 监听器
（`HERALD_TEST_LISTENER_ADDR`，默认 `127.0.0.1:8092`）上提供，并由
`HERALD_TEST_API_KEY` 守护；公开的 `:8082` 监听器永不暴露它。由于该监听器在容器内仅
loopback 可达，`getTestCode` 在设置了 `HERALD_COMPOSE_DIR` 时通过
`docker compose exec herald curl` 访问（`scripts/run-e2e.sh` 与 CI 即如此接线）。
解析顺序：`HERALD_TEST_CODE_URL`（显式 base）→ `HERALD_COMPOSE_DIR`（exec）→
`heraldURL` 兜底。

## 测试数据

`fixtures/warden/data.json`：admin 13800138000、user 13900139000、guest 13700137000、inactive 13600136000、ratelimit 13500135000。

## 运行

```bash
go test -v ./e2e/...
go test -v ./e2e/... -run TestCompleteLoginFlow
go test -v ./e2e/... -run TestProtectedWhoamiAfterLogin   # 需设 PROTECTED_URL
go test -v ./e2e/... -run TestInvalid
go test -v ./e2e/... -run TestHeraldUnavailable
go test -v ./e2e/... -run TestWardenUnavailable
go test -v ./e2e/... -run TestLivenessReadinessAreDistinct
go test -v ./e2e/... -run TestHeraldNonceReplayRejected
go test -v ./e2e/... -run TestHeraldTestCodeNotOnMainListener
```

Traefik 部署：`export PROTECTED_URL=https://whoami.test.localhost` 后运行 TestProtectedWhoamiAfterLogin。

## 注意

- 先启动服务（`make up`）。测试会调用 ensureServicesReady 并清理限流状态。
- 服务不可用测试需要 docker compose，可能被跳过。
- 验证码过期：可调整 Herald CHALLENGE_EXPIRY。
- 受保护 whoami：未设置 PROTECTED_URL 时跳过（如无 Traefik 的 build/image）。

参见 [../README.zh-CN](../README.zh-CN.md)。
