English | [中文](README.zh-CN.md)

# E2E tests

Layout and how to run. Overview: [../README.md](../README.md).

## Layout

- e2e_test.go — normal flow
- error_scenarios_test.go — wrong/expired/locked code, non-whitelist, inactive, rate limits, service down, auth, edge cases
- v1_failure_contracts_test.go — PR9 v1 contract tests: liveness/readiness are distinct endpoints, HMAC v2 nonce replay is rejected, and the Herald test-code endpoint is absent from the public listener
- auth_test.go, herald_api_test.go, warden_api_test.go, idempotency_test.go, audit_test.go, provider_test.go, metrics_test.go
- test_helpers.go — ensureServicesReady, sendVerificationCodeWithError, loginWithError, clearRateLimitKeys, stop/start Docker, and the HMAC v2 signing helpers (signHeraldV2 / signHeraldReq / signHeraldV2Fixed)

## Service-to-service auth (HMAC v2)

Herald runs `REQUEST_AUTH_MODE=hmac_v2` (test/production posture), so all Herald
`/v1/otp/*` calls are signed with HMAC v2, not `X-API-Key`. The signing helpers
build the canonical string
`HERALD-HMAC-V2\n<METHOD>\n<path>\n<query>\n<ts>\n<nonce>\n<service>\n<keyid>\n<sha256(body)>`,
mirroring `herald/internal/auth/hmac_v2.go` exactly. `heraldHMACSecret` must
match the `HMAC_SECRET` wired by the test profile. API-key requests are expected
to be **rejected** (see `TestHeraldAPIKeyRejectedUnderHMACV2`).

## Herald test verification code

The `/v1/test/code` endpoint is served only on Herald's dedicated loopback-only
listener (`HERALD_TEST_LISTENER_ADDR`, default `127.0.0.1:8092`) guarded by
`HERALD_TEST_API_KEY`; the public `:8082` listener never exposes it. Because the
listener is loopback-only inside the container, `getTestCode` reaches it via
`docker compose exec herald curl` when `HERALD_COMPOSE_DIR` is set (the way
`scripts/run-e2e.sh` and CI wire it). Resolution order: `HERALD_TEST_CODE_URL`
(explicit base) → `HERALD_COMPOSE_DIR` (exec) → `heraldURL` fallback.

## Test data

`fixtures/warden/data.json`: admin 13800138000, user 13900139000, guest 13700137000, inactive 13600136000, ratelimit 13500135000.

## Run

```bash
go test -v ./e2e/...
go test -v ./e2e/... -run TestCompleteLoginFlow
go test -v ./e2e/... -run TestProtectedWhoamiAfterLogin   # needs PROTECTED_URL
go test -v ./e2e/... -run TestInvalid
go test -v ./e2e/... -run TestHeraldUnavailable
go test -v ./e2e/... -run TestWardenUnavailable
go test -v ./e2e/... -run TestLivenessReadinessAreDistinct
go test -v ./e2e/... -run TestHeraldNonceReplayRejected
go test -v ./e2e/... -run TestHeraldTestCodeNotOnMainListener
```

With Traefik: `export PROTECTED_URL=https://whoami.test.localhost` then run TestProtectedWhoamiAfterLogin.

## Notes

- Start services first (`make up`). Tests use ensureServicesReady and clear rate-limit state.
- Service-down tests need docker compose; may be skipped.
- Challenge expiry: tune Herald CHALLENGE_EXPIRY for expiry tests.
- Protected whoami: skipped when PROTECTED_URL is unset (e.g. build/image without Traefik).

See [../README](../README.md).
