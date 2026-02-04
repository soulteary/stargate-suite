English | [中文](README.zh-CN.md)

# E2E tests

Layout and how to run. Overview: [../README.md](../README.md).

## Layout

- e2e_test.go — normal flow
- error_scenarios_test.go — wrong/expired/locked code, non-whitelist, inactive, rate limits, service down, auth, edge cases
- auth_test.go, herald_api_test.go, warden_api_test.go, idempotency_test.go, audit_test.go, provider_test.go, metrics_test.go
- test_helpers.go — ensureServicesReady, sendVerificationCodeWithError, loginWithError, clearRateLimitKeys, stop/start Docker, etc.

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
```

With Traefik: `export PROTECTED_URL=https://whoami.test.localhost` then run TestProtectedWhoamiAfterLogin.

## Notes

- Start services first (`make up`). Tests use ensureServicesReady and clear rate-limit state.
- Service-down tests need docker compose; may be skipped.
- Challenge expiry: tune Herald CHALLENGE_EXPIRY for expiry tests.
- Protected whoami: skipped when PROTECTED_URL is unset (e.g. build/image without Traefik).

See [../README](../README.md) · [../MANUAL_TESTING](../MANUAL_TESTING.md).
