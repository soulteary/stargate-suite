package e2e

// E2E service URLs and auth host (used by tests and test helpers).
const (
	stargateURL = "http://localhost:8080"
	heraldURL   = "http://localhost:8082"
	wardenURL   = "http://localhost:8081"
	authHost    = "auth.test.localhost"
)

// AuthHeaders represents the auth headers returned by forwardAuth.
type AuthHeaders struct {
	UserID string
	Email  string
	Scopes string
	Role   string
}
