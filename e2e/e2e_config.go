package e2e

import "os"

// E2E service URLs and auth host (used by tests and test helpers).
// Use 127.0.0.1 (not localhost) so CI and local runs behave consistently; localhost can resolve to IPv6 (::1) on some runners and cause connection failures when services bind to IPv4 only.
const (
	stargateURL = "http://127.0.0.1:8080"
	heraldURL   = "http://127.0.0.1:8082"
	wardenURL   = "http://127.0.0.1:8081"
	authHost    = "auth.test.localhost"
)

// Herald service-to-service test credentials. These are shared by tests and the
// non-test helpers (e.g. signHeraldReq), so they must live in a non-test file to
// remain visible to `go build ./...`. heraldHMACSecret must match HMAC_SECRET
// wired into Herald by the test deployment profile; heraldAPIKey is only used by
// the negative auth tests (API key must be rejected under REQUEST_AUTH_MODE=hmac_v2).
const (
	heraldAPIKey     = "test-herald-api-key"
	heraldHMACSecret = "test-hmac-secret"
)

// protectedURL 为经 Stargate Forward Auth 保护的 whoami 地址（如 Traefik 部署时的 https://whoami.test.localhost）。
// 仅当设置环境变量 PROTECTED_URL 时，e2e 会执行受保护服务访问验证；未设置则跳过。
func protectedURL() string {
	return os.Getenv("PROTECTED_URL")
}

// AuthHeaders represents the auth headers returned by forwardAuth.
type AuthHeaders struct {
	UserID string
	Email  string
	Scopes string
	Role   string
}
