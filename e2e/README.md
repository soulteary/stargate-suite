English | [中文](README.zh-CN.md)

# E2E tests

Layout and how to run. Overview: [../README.md](../README.md).

## Layout

- e2e_test.go — normal flow
- error_scenarios_test.go — wrong/expired/locked code, non-whitelist, inactive, rate limits, service down, auth, edge cases
- v1_failure_contracts_test.go — v1 contracts: strict 2xx readiness, dependency-outage liveness/readiness and recovery, identified HMAC v3 nonce replay rejection, and isolation of the Herald test-code listener
- auth_test.go, herald_api_test.go, warden_api_test.go, idempotency_test.go, audit_test.go, provider_test.go, metrics_test.go
- test_helpers.go — ensureServicesReady, sendVerificationCodeWithError, loginWithError, clearRateLimitKeys, stop/start Docker, and the HMAC v3 signing helpers (signHeraldV3 / signHeraldReq / signHeraldFixed), including the Idempotency-Key header

## Service-to-service auth (HMAC v3)

Herald 1.9 keeps `REQUEST_AUTH_MODE=hmac_v2` as the configured auth mode and
requires **v3 signatures for challenge creation**. The E2E clients sign all
`/v1/otp/*` requests using the released canonical format:
`HERALD-HMAC-V3\n<METHOD>\n<path>\n<query>\n<ts>\n<nonce>\n<service>\n<keyid>\n<sha256(Idempotency-Key)>\n<sha256(body)>`.
Set `Idempotency-Key` before signing; an absent header is also hashed and bound.
`heraldHMACSecret` must match the test profile's `HMAC_SECRET`. With a single
secret, the explicit key ID is `default`; omitting it uses the same identity.
The tests verify rejection of v2 creation requests, modified or removed
idempotency headers, replayed nonces, and API-key-only requests.

## Herald test verification code

The `/v1/test/code` endpoint is served only on Herald's dedicated loopback-only
listener (`HERALD_TEST_LISTENER_ADDR`, default `127.0.0.1:8092`) guarded by
`HERALD_TEST_API_KEY`; the public `:8082` listener never exposes it. Because the
listener is loopback-only inside the container, `getTestCode` reaches it via
`docker compose exec herald curl` when `HERALD_COMPOSE_DIR` is set (the way
`scripts/run-e2e.sh` and CI wire it). Resolution order: `HERALD_TEST_CODE_URL`
(explicit base) → `HERALD_COMPOSE_DIR` (exec) → an error when neither dedicated access path is configured.

## Test data

`fixtures/warden/data.json`: admin 13800138000, user 13900139000, guest 13700137000, inactive 13600136000, ratelimit 13500135000.

## Run

```bash
go test -v ./e2e/...
go test -v ./e2e/... -run TestCompleteLoginFlow
go test -v ./e2e/... -run TestProtectedWhoamiAfterLogin   # needs PROTECTED_URL
go test -v ./e2e/... -run TestInvalid
go test -v ./e2e/... -run TestLivenessReadinessAreDistinct
go test -v ./e2e/... -run TestDependencyFailureRecoveryContracts
go test -v ./e2e/... -run TestHeraldNonceReplayRejected
go test -v ./e2e/... -run TestHeraldTestCodeNotOnMainListener
```

With Traefik: `export PROTECTED_URL=https://whoami.test.localhost` then run TestProtectedWhoamiAfterLogin.

## Notes

- Prefer `./scripts/run-e2e.sh`; it validates Compose, starts services without
  suppressing errors, and waits on the real readiness endpoints before tests.
- Dependency failure/recovery tests require Docker Compose and
  `HERALD_COMPOSE_DIR` (the runner and CI set it automatically).
- Challenge expiry: tune Herald CHALLENGE_EXPIRY for expiry tests.
- Protected whoami: skipped when PROTECTED_URL is unset (e.g. build/image without Traefik).

See [../README](../README.md).
