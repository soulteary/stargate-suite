中文 | [English](README.md)

# E2E 测试

用例结构与运行方式。总览见 [../README.zh-CN.md](../README.zh-CN.md)。

## 结构

- e2e_test.go — 正常流程
- error_scenarios_test.go — 错误/过期/锁定验证码、非白名单、非活跃、限流、服务宕机、鉴权、边界
- v1_failure_contracts_test.go — v1 契约：严格 2xx 就绪判定、依赖故障时的存活/就绪与恢复、带原因的 HMAC v3 nonce 重放拒绝，以及 Herald 测试验证码监听器隔离
- auth_test.go、herald_api_test.go、warden_api_test.go、idempotency_test.go、audit_test.go、provider_test.go、metrics_test.go
- test_helpers.go — ensureServicesReady、sendVerificationCodeWithError、loginWithError、clearRateLimitKeys、停止/启动 Docker，以及 HMAC v3 签名助手（signHeraldV3 / signHeraldReq / signHeraldFixed），签名绑定 Idempotency-Key 请求头

## 服务间鉴权（HMAC v3）

Herald 1.9 的配置模式仍为 `REQUEST_AUTH_MODE=hmac_v2`，创建 challenge 时必须使用
**v3 签名**。E2E 客户端对所有 `/v1/otp/*` 请求使用已发布版本的规范串：
`HERALD-HMAC-V3\n<METHOD>\n<path>\n<query>\n<ts>\n<nonce>\n<service>\n<keyid>\n<sha256(Idempotency-Key)>\n<sha256(body)>`。
先设置 `Idempotency-Key` 再签名；未提供该请求头时也会绑定空值的摘要。
`heraldHMACSecret` 必须与 test profile 注入的 `HMAC_SECRET` 一致。单密钥模式的显式
key ID 为 `default`，省略时仍使用该身份。测试会验证拒绝 v2 创建请求、被修改或删除的
幂等请求头、重放 nonce，以及仅提供 API Key 的请求。

## Herald 测试验证码

`/v1/test/code` 仅在 Herald 独立的 loopback-only 监听器
（`HERALD_TEST_LISTENER_ADDR`，默认 `127.0.0.1:8092`）上提供，并由
`HERALD_TEST_API_KEY` 守护；公开的 `:8082` 监听器永不暴露它。由于该监听器在容器内仅
loopback 可达，`getTestCode` 在设置了 `HERALD_COMPOSE_DIR` 时通过
`docker compose exec herald curl` 访问（`scripts/run-e2e.sh` 与 CI 即如此接线）。
解析顺序：`HERALD_TEST_CODE_URL`（显式 base）→ `HERALD_COMPOSE_DIR`（exec）→
两种专用访问方式均未配置时报错。

## 测试数据

`fixtures/warden/data.json`：admin 13800138000、user 13900139000、guest 13700137000、inactive 13600136000、ratelimit 13500135000。

## 运行

```bash
go test -v ./e2e/...
go test -v ./e2e/... -run TestCompleteLoginFlow
go test -v ./e2e/... -run TestProtectedWhoamiAfterLogin   # 需设 PROTECTED_URL
go test -v ./e2e/... -run TestInvalid
go test -v ./e2e/... -run TestLivenessReadinessAreDistinct
go test -v ./e2e/... -run TestDependencyFailureRecoveryContracts
go test -v ./e2e/... -run TestHeraldNonceReplayRejected
go test -v ./e2e/... -run TestHeraldTestCodeNotOnMainListener
```

Traefik 部署：`export PROTECTED_URL=https://whoami.test.localhost` 后运行 TestProtectedWhoamiAfterLogin。

## 注意

- 建议使用 `./scripts/run-e2e.sh`；它会先校验 Compose、明确启动服务（不再吞掉
  错误），并等待真实 readiness 端点后再运行测试。
- 依赖故障/恢复测试需要 Docker Compose 与 `HERALD_COMPOSE_DIR`（runner 与 CI
  会自动设置）。
- 验证码过期：可调整 Herald CHALLENGE_EXPIRY。
- 受保护 whoami：未设置 PROTECTED_URL 时跳过（如无 Traefik 的 build/image）。

参见 [../README.zh-CN](../README.zh-CN.md)。
