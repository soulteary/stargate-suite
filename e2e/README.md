English | [中文](README.zh-CN.md)

# E2E test documentation

This document describes the e2e test layout and how to run tests. Project overview and quick start: [../README.md](../README.md).

## Test file layout

```
e2e/
├── e2e_test.go              # Normal flow tests
├── error_scenarios_test.go  # Error scenarios
├── test_helpers.go          # Helpers
├── warden_api_test.go       # Warden API
├── herald_api_test.go       # Herald API
├── auth_test.go             # Service-to-service auth
├── idempotency_test.go      # Idempotency
├── audit_test.go            # Audit logs
├── provider_test.go         # Provider
├── metrics_test.go          # Metrics
└── README.md
```

## Test cases overview

### Normal flow

- **TestCompleteLoginFlow**: send code → get code (Herald test endpoint) → login → verify forwardAuth headers
- **TestProtectedWhoamiAfterLogin**: after login, request protected whoami URL (set `PROTECTED_URL`, e.g. `https://whoami.test.localhost`); skipped when `PROTECTED_URL` is unset (e.g. when using `build/image` without Traefik)

### Error scenarios (error_scenarios_test.go)

1. **Verification code**: wrong code, expired code, locked after too many attempts
2. **User**: user not in whitelist, inactive user
3. **Rate limits**: IP rate limit, user rate limit, resend cooldown
4. **Service down**: Herald unavailable, Warden unavailable (stop/start via docker compose)
5. **Auth**: unauthenticated forwardAuth, invalid session cookie
6. **Edge**: empty params, invalid challenge_id, invalid auth_method

### Other suites

- **auth_test.go**: Herald HMAC, Warden/Herald API keys
- **herald_api_test.go**: challenges, verify, revoke, rate limit
- **warden_api_test.go**: user lookup by phone/email/user_id
- **idempotency_test.go**, **audit_test.go**, **provider_test.go**, **metrics_test.go**

## Test data

Defined in `fixtures/warden/data.json`: admin (13800138000), user (13900139000), guest (13700137000), inactive (13600136000), ratelimit (13500135000).

## Run tests

```bash
go test -v ./e2e/...
go test -v ./e2e/... -run TestCompleteLoginFlow
go test -v ./e2e/... -run TestProtectedWhoamiAfterLogin   # requires PROTECTED_URL
go test -v ./e2e/... -run TestInvalid
go test -v ./e2e/... -run TestHeraldUnavailable
go test -v ./e2e/... -run TestWardenUnavailable
```

With Traefik compose, to verify protected whoami:
```bash
export PROTECTED_URL=https://whoami.test.localhost   # ensure whoami.test.localhost resolves to Traefik
go test -v ./e2e/... -run TestProtectedWhoamiAfterLogin
```

## Notes

1. **Services up**: Tests check service readiness first; start services (e.g. `make up`) before running.
2. **Isolation**: Each test uses separate test users; rate-limit state is cleared where needed.
3. **Rate-limit tests**: May require Herald config or timing adjustments.
4. **Service-down tests**: Need docker compose access; may be skipped without it.
5. **Challenge expiry**: Default 5m; tune Herald `CHALLENGE_EXPIRY` to speed up expiry tests.
6. **Concurrency**: Use different users when running tests in parallel.
7. **Protected whoami**: `TestProtectedWhoamiAfterLogin` runs only when `PROTECTED_URL` is set; skipped with `build/image` (no Traefik).

## Helpers (test_helpers.go)

- `waitForService`, `ensureServicesReady` — wait for services to be ready; `ensureServicesReady` also clears rate-limit state
- `waitForServiceDown` — wait until a service returns non-2xx or connection error
- `sendVerificationCodeWithError`, `loginWithError`, `checkAuthWithError`
- `triggerRateLimit`
- `stopDockerServiceInDir`, `startDockerServiceInDir`
- `sendVerificationCodeWithEmail`
- `clearRateLimitKeys` — clear test state in Redis

## ErrorResponse

```go
type ErrorResponse struct {
    StatusCode int
    Message    string
    Body       string
}
```

## See also

- [../README.md](../README.md) — Project overview, services, troubleshooting
- [../MANUAL_TESTING.md](../MANUAL_TESTING.md) — Manual browser verification
