中文 | [English](README.md)

# E2E 测试

用例结构与运行方式。总览见 [../README.zh-CN.md](../README.zh-CN.md)。

## 结构

- e2e_test.go — 正常流程
- error_scenarios_test.go — 错误/过期/锁定验证码、非白名单、非活跃、限流、服务宕机、鉴权、边界
- auth_test.go、herald_api_test.go、warden_api_test.go、idempotency_test.go、audit_test.go、provider_test.go、metrics_test.go
- test_helpers.go — ensureServicesReady、sendVerificationCodeWithError、loginWithError、clearRateLimitKeys、停止/启动 Docker 等

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
```

Traefik 部署：`export PROTECTED_URL=https://whoami.test.localhost` 后运行 TestProtectedWhoamiAfterLogin。

## 注意

- 先启动服务（`make up`）。测试会调用 ensureServicesReady 并清理限流状态。
- 服务不可用测试需要 docker compose，可能被跳过。
- 验证码过期：可调整 Herald CHALLENGE_EXPIRY。
- 受保护 whoami：未设置 PROTECTED_URL 时跳过（如无 Traefik 的 build/image）。

参见 [../README.zh-CN](../README.zh-CN.md) · [../MANUAL_TESTING.zh-CN](../MANUAL_TESTING.zh-CN.md)。
