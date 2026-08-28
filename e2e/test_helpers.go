package e2e

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// ErrorResponse represents an error response
type ErrorResponse struct {
	StatusCode int
	Message    string
	Body       string
}

// sendVerificationCodeWithError sends a verification code and returns an error response (if failed)
func sendVerificationCodeWithError(t *testing.T, phone string) (string, *ErrorResponse) {
	url := fmt.Sprintf("%s/_send_verify_code", stargateURL)
	body := fmt.Sprintf("phone=%s", phone)

	req, err := http.NewRequest("POST", url, strings.NewReader(body))
	if err != nil {
		return "", &ErrorResponse{StatusCode: 0, Message: err.Error()}
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Forwarded-Host", authHost)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", &ErrorResponse{StatusCode: 0, Message: err.Error()}
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Logf("Warning: failed to close response body: %v", closeErr)
		}
	}()

	bodyBytes, _ := io.ReadAll(resp.Body)
	bodyStr := string(bodyBytes)

	if resp.StatusCode != http.StatusOK {
		var errorMsg string
		var result struct {
			Success bool   `json:"success"`
			Message string `json:"message"`
			Error   string `json:"error"`
		}
		if err := json.Unmarshal(bodyBytes, &result); err == nil {
			if result.Message != "" {
				errorMsg = result.Message
			} else if result.Error != "" {
				errorMsg = result.Error
			} else {
				errorMsg = bodyStr
			}
		} else {
			errorMsg = bodyStr
		}
		return "", &ErrorResponse{
			StatusCode: resp.StatusCode,
			Message:    errorMsg,
			Body:       bodyStr,
		}
	}

	var result struct {
		Success     bool   `json:"success"`
		ChallengeID string `json:"challenge_id"`
		ExpiresIn   int    `json:"expires_in"`
	}

	if err := json.NewDecoder(strings.NewReader(bodyStr)).Decode(&result); err != nil {
		return "", &ErrorResponse{StatusCode: resp.StatusCode, Message: err.Error(), Body: bodyStr}
	}

	if !result.Success {
		return "", &ErrorResponse{StatusCode: resp.StatusCode, Message: "send verification code failed", Body: bodyStr}
	}

	return result.ChallengeID, nil
}

// loginWithError logs in and returns an error response (if failed)
func loginWithError(t *testing.T, phone, challengeID, verifyCode string) (string, *ErrorResponse) {
	url := fmt.Sprintf("%s/_login", stargateURL)
	body := fmt.Sprintf("auth_method=warden&phone=%s&challenge_id=%s&verify_code=%s",
		phone, challengeID, verifyCode)

	req, err := http.NewRequest("POST", url, strings.NewReader(body))
	if err != nil {
		return "", &ErrorResponse{StatusCode: 0, Message: err.Error()}
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Forwarded-Host", authHost)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", &ErrorResponse{StatusCode: 0, Message: err.Error()}
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Logf("Warning: failed to close response body: %v", closeErr)
		}
	}()

	bodyBytes, _ := io.ReadAll(resp.Body)
	bodyStr := string(bodyBytes)

	if resp.StatusCode != http.StatusOK {
		var errorMsg string
		var result struct {
			Success bool   `json:"success"`
			Message string `json:"message"`
			Error   string `json:"error"`
		}
		if err := json.Unmarshal(bodyBytes, &result); err == nil {
			if result.Message != "" {
				errorMsg = result.Message
			} else if result.Error != "" {
				errorMsg = result.Error
			} else {
				errorMsg = bodyStr
			}
		} else {
			errorMsg = bodyStr
		}
		return "", &ErrorResponse{
			StatusCode: resp.StatusCode,
			Message:    errorMsg,
			Body:       bodyStr,
		}
	}

	// Extract Set-Cookie header
	setCookieHeaders := resp.Header.Values("Set-Cookie")
	if len(setCookieHeaders) == 0 {
		return "", &ErrorResponse{StatusCode: resp.StatusCode, Message: "no Set-Cookie header found"}
	}

	// Find session cookie (stargate_session_id)
	var sessionCookie string
	for _, cookieHeader := range setCookieHeaders {
		if strings.Contains(cookieHeader, "stargate_session_id") {
			parts := strings.Split(cookieHeader, ";")
			if len(parts) > 0 {
				sessionCookie = strings.TrimSpace(parts[0])
				break
			}
		}
	}

	if sessionCookie == "" {
		return "", &ErrorResponse{StatusCode: resp.StatusCode, Message: "session cookie not found"}
	}

	return sessionCookie, nil
}

// checkAuthWithError verifies authorization and returns an error response (if failed)
func checkAuthWithError(t *testing.T, sessionCookie string) (*AuthHeaders, *ErrorResponse) {
	url := fmt.Sprintf("%s/_auth", stargateURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, &ErrorResponse{StatusCode: 0, Message: err.Error()}
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Host", authHost)
	if sessionCookie != "" {
		req.Header.Set("Cookie", sessionCookie)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, &ErrorResponse{StatusCode: 0, Message: err.Error()}
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Logf("Warning: failed to close response body: %v", closeErr)
		}
	}()

	bodyBytes, _ := io.ReadAll(resp.Body)
	bodyStr := string(bodyBytes)

	if resp.StatusCode != http.StatusOK {
		var errorMsg string
		var result struct {
			Message string `json:"message"`
			Error   string `json:"error"`
		}
		if err := json.Unmarshal(bodyBytes, &result); err == nil {
			if result.Message != "" {
				errorMsg = result.Message
			} else if result.Error != "" {
				errorMsg = result.Error
			} else {
				errorMsg = bodyStr
			}
		} else {
			errorMsg = bodyStr
		}
		return nil, &ErrorResponse{
			StatusCode: resp.StatusCode,
			Message:    errorMsg,
			Body:       bodyStr,
		}
	}

	headers := &AuthHeaders{
		UserID: resp.Header.Get("X-Auth-User"),
		Email:  resp.Header.Get("X-Auth-Email"),
		Scopes: resp.Header.Get("X-Auth-Scopes"),
		Role:   resp.Header.Get("X-Auth-Role"),
	}

	return headers, nil
}

// triggerRateLimit triggers rate limiting (sends multiple requests quickly)
func triggerRateLimit(t *testing.T, phone string, count int) []*ErrorResponse {
	errors := make([]*ErrorResponse, 0, count)
	for i := 0; i < count; i++ {
		_, errResp := sendVerificationCodeWithError(t, phone)
		if errResp != nil {
			errors = append(errors, errResp)
		}
		// Short delay to avoid being too fast
		time.Sleep(100 * time.Millisecond)
	}
	return errors
}

// stopDockerServiceInDir stops a Docker service in the specified directory
func stopDockerServiceInDir(dir, serviceName string) error {
	cmd := exec.Command("docker", "compose", "stop", serviceName)
	cmd.Dir = dir
	return cmd.Run()
}

// startDockerServiceInDir starts a Docker service in the specified directory
func startDockerServiceInDir(dir, serviceName string) error {
	cmd := exec.Command("docker", "compose", "start", serviceName)
	cmd.Dir = dir
	return cmd.Run()
}

// sendVerificationCodeWithEmail sends a verification code using email
func sendVerificationCodeWithEmail(t *testing.T, email string) (string, *ErrorResponse) {
	url := fmt.Sprintf("%s/_send_verify_code", stargateURL)
	body := fmt.Sprintf("mail=%s", email)

	req, err := http.NewRequest("POST", url, strings.NewReader(body))
	if err != nil {
		return "", &ErrorResponse{StatusCode: 0, Message: err.Error()}
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Forwarded-Host", authHost)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", &ErrorResponse{StatusCode: 0, Message: err.Error()}
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Logf("Warning: failed to close response body: %v", closeErr)
		}
	}()

	bodyBytes, _ := io.ReadAll(resp.Body)
	bodyStr := string(bodyBytes)

	if resp.StatusCode != http.StatusOK {
		var errorMsg string
		var result struct {
			Success bool   `json:"success"`
			Message string `json:"message"`
			Error   string `json:"error"`
		}
		if err := json.Unmarshal(bodyBytes, &result); err == nil {
			if result.Message != "" {
				errorMsg = result.Message
			} else if result.Error != "" {
				errorMsg = result.Error
			} else {
				errorMsg = bodyStr
			}
		} else {
			errorMsg = bodyStr
		}
		return "", &ErrorResponse{
			StatusCode: resp.StatusCode,
			Message:    errorMsg,
			Body:       bodyStr,
		}
	}

	var result struct {
		Success     bool   `json:"success"`
		ChallengeID string `json:"challenge_id"`
		ExpiresIn   int    `json:"expires_in"`
	}

	if err := json.NewDecoder(strings.NewReader(bodyStr)).Decode(&result); err != nil {
		return "", &ErrorResponse{StatusCode: resp.StatusCode, Message: err.Error(), Body: bodyStr}
	}

	if !result.Success {
		return "", &ErrorResponse{StatusCode: resp.StatusCode, Message: "send verification code failed", Body: bodyStr}
	}

	return result.ChallengeID, nil
}

// heraldV2Signature holds the headers required for a Herald HMAC v2 request.
type heraldV2Signature struct {
	Signature string
	Timestamp string
	Nonce     string
	Service   string
	KeyID     string
}

// signHeraldV2 computes a Herald HMAC v2 signature over the canonical string:
//
//	HERALD-HMAC-V2\n<UPPER(METHOD)>\n<path>\n<query>\n<ts>\n<nonce>\n<service>\n<keyID>\n<hex(sha256(body))>
//
// This mirrors herald/internal/auth/hmac_v2.go (v1.1.0) exactly and is
// cross-verified against the upstream CanonicalRequest.Canonical() field order.
// With a single HMAC_SECRET (no key map) the server resolves the secret via an
// implicit "default" key id, so keyID may be empty and no X-Key-Id is sent; the
// empty keyID is still bound into the signature as the server does.
func signHeraldV2(method, path, query, service, keyID, secret string, body []byte) heraldV2Signature {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := hex.EncodeToString([]byte(strconv.FormatInt(time.Now().UnixNano(), 10)))
	bodyHash := sha256.Sum256(body)

	var b strings.Builder
	b.WriteString("HERALD-HMAC-V2")
	b.WriteByte('\n')
	b.WriteString(strings.ToUpper(method))
	b.WriteByte('\n')
	b.WriteString(path)
	b.WriteByte('\n')
	b.WriteString(query)
	b.WriteByte('\n')
	b.WriteString(ts)
	b.WriteByte('\n')
	b.WriteString(nonce)
	b.WriteByte('\n')
	b.WriteString(service)
	b.WriteByte('\n')
	b.WriteString(keyID)
	b.WriteByte('\n')
	b.WriteString(hex.EncodeToString(bodyHash[:]))

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(b.String()))
	return heraldV2Signature{
		Signature: hex.EncodeToString(mac.Sum(nil)),
		Timestamp: ts,
		Nonce:     nonce,
		Service:   service,
		KeyID:     keyID,
	}
}

// signHeraldV2Fixed is like signHeraldV2 but takes an explicit timestamp and
// nonce instead of generating them. It is used by negative tests that need to
// sign over an out-of-drift timestamp so the only failing server check is the
// timestamp bound (drift is checked before signature verification upstream).
func signHeraldV2Fixed(method, path, query, ts, nonce, service, keyID, secret string, body []byte) string {
	bodyHash := sha256.Sum256(body)
	var b strings.Builder
	b.WriteString("HERALD-HMAC-V2")
	b.WriteByte('\n')
	b.WriteString(strings.ToUpper(method))
	b.WriteByte('\n')
	b.WriteString(path)
	b.WriteByte('\n')
	b.WriteString(query)
	b.WriteByte('\n')
	b.WriteString(ts)
	b.WriteByte('\n')
	b.WriteString(nonce)
	b.WriteByte('\n')
	b.WriteString(service)
	b.WriteByte('\n')
	b.WriteString(keyID)
	b.WriteByte('\n')
	b.WriteString(hex.EncodeToString(bodyHash[:]))

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(b.String()))
	return hex.EncodeToString(mac.Sum(nil))
}

// setHeraldV2Headers applies the v2 auth headers to a request. X-Key-Id is only
// sent when keyID is non-empty (empty keyID → implicit default on the server).
func setHeraldV2Headers(req *http.Request, sig heraldV2Signature) {
	req.Header.Set("X-Signature-Version", "v2")
	req.Header.Set("X-Signature", sig.Signature)
	req.Header.Set("X-Timestamp", sig.Timestamp)
	req.Header.Set("X-Nonce", sig.Nonce)
	req.Header.Set("X-Service", sig.Service)
	if sig.KeyID != "" {
		req.Header.Set("X-Key-Id", sig.KeyID)
	}
}

// signHeraldReq signs an already-built Herald request with HMAC v2 using the
// shared test secret, deriving method/path/query from the request and hashing
// the provided raw body. It replaces the legacy `X-API-Key` header the tests
// used to set, since Herald v1.1.0 runs REQUEST_AUTH_MODE=hmac_v2 and never
// downgrades to API-key auth. Pass nil body for requests without a body.
func signHeraldReq(req *http.Request, body []byte) {
	query := ""
	if req.URL != nil {
		query = req.URL.RawQuery
	}
	path := ""
	if req.URL != nil {
		path = req.URL.Path
	}
	sig := signHeraldV2(req.Method, path, query, "e2e-suite", "", heraldHMACSecret, body)
	setHeraldV2Headers(req, sig)
}

// clearRateLimitKeys clears Herald's Redis-backed state and restarts Stargate
// so one scenario cannot consume another scenario's Redis or in-process
// rate-limit budget.
func clearRateLimitKeys(t *testing.T) error {
	// Compose-based E2E runs intentionally do not publish Redis to the host.
	// Execute FLUSHDB inside the isolated Redis container and reuse the password
	// already injected into that container.
	if dir := os.Getenv("HERALD_COMPOSE_DIR"); dir != "" {
		cmd := exec.Command("docker", "compose", "exec", "-T", "herald-redis",
			"sh", "-c", `REDISCLI_AUTH="$HERALD_REDIS_PASSWORD" redis-cli FLUSHDB`)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to clear Herald Redis through Compose: %w: %s", err, strings.TrimSpace(string(out)))
		}
		t.Log("Cleared Herald Redis test state through Compose")

		// Stargate v1.0.0 keeps the verification endpoint limiter in process,
		// independently of Herald Redis. Restart only the Stargate container to
		// give each isolated scenario a fresh five-request budget.
		restart := exec.Command("docker", "compose", "restart", "stargate")
		restart.Dir = dir
		restartOut, restartErr := restart.CombinedOutput()
		if restartErr != nil {
			return fmt.Errorf("failed to reset Stargate rate limiter: %w: %s", restartErr, strings.TrimSpace(string(restartOut)))
		}
		if !waitForService(t, stargateURL+"/healthz", 30*time.Second) {
			return fmt.Errorf("Stargate did not become ready after rate-limit reset")
		}
		t.Log("Reset Stargate in-process rate limiter through Compose")
		return nil
	}

	// Local fallback for an explicitly published Redis.
	addr := os.Getenv("HERALD_REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	redisClient := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: os.Getenv("HERALD_REDIS_PASSWORD"),
		DB:       0,
	})
	defer func() {
		if err := redisClient.Close(); err != nil {
			t.Logf("Warning: failed to close redis client: %v", err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := redisClient.FlushDB(ctx).Err(); err != nil {
		return fmt.Errorf("failed to clear Herald Redis at %s: %w", addr, err)
	}
	t.Log("Cleared Herald Redis test state")
	return nil
}

// waitForService waits for the service to be ready (HTTP status < 500).
// Returns false on timeout; on failure the last error or status is logged for debugging.
func waitForService(t *testing.T, url string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 5 * time.Second}
	var lastErr error
	var lastStatus int

	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err != nil {
			lastErr = err
			lastStatus = 0
			time.Sleep(1 * time.Second)
			continue
		}
		lastStatus = resp.StatusCode
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Logf("Warning: failed to close response body: %v", closeErr)
		}
		if resp.StatusCode < 500 {
			return true
		}
		lastErr = nil
		time.Sleep(1 * time.Second)
	}

	if lastErr != nil {
		t.Logf("waitForService %s: last error: %v", url, lastErr)
	} else if lastStatus != 0 {
		t.Logf("waitForService %s: last status: %d", url, lastStatus)
	}
	return false
}

// ensureServicesReady ensures all services (Stargate, Warden, Herald) are ready and clears rate-limit state.
func ensureServicesReady(t *testing.T) {
	if !waitForService(t, stargateURL+"/health", 30*time.Second) {
		t.Fatalf("Stargate service is not ready")
	}
	if !waitForService(t, heraldURL+"/healthz", 30*time.Second) {
		t.Fatalf("Herald service is not ready")
	}
	if !waitForService(t, wardenURL+"/health", 30*time.Second) {
		t.Fatalf("Warden service is not ready")
	}

	if err := clearRateLimitKeys(t); err != nil {
		t.Logf("Warning: Failed to clear rate limit keys: %v (continuing test anyway)", err)
	}
}

// waitForServiceDown polls the URL until it returns non-2xx or connection error, or timeout.
// Returns true if the service is down (connection failed or status >= 400) within timeout, false on timeout.
// Use in tests that stop a service and need to assert it is down (e.g. service-down scenarios).
//
//nolint:unused
func waitForServiceDown(t *testing.T, url string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 2 * time.Second}

	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err != nil {
			return true // connection failed, service considered down
		}
		code := resp.StatusCode
		_ = resp.Body.Close()
		if code >= 400 {
			return true // non-2xx, service considered down
		}
		time.Sleep(500 * time.Millisecond)
	}
	return false
}
