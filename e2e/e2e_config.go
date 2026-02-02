package e2e

import "os"

// E2E service URLs and auth host (used by tests and test helpers).
const (
	stargateURL = "http://localhost:8080"
	heraldURL   = "http://localhost:8082"
	wardenURL   = "http://localhost:8081"
	authHost    = "auth.test.localhost"
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
