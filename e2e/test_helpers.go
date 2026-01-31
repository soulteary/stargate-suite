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
	"os/exec"
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

// calculateHMAC calculates HMAC-SHA256 signature
// Signature format: HMAC-SHA256(timestamp:service:body, secret)
func calculateHMAC(timestamp int64, service, body, secret string) string {
	message := fmt.Sprintf("%d:%s:%s", timestamp, service, body)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
}

// clearRateLimitKeys clears test state in Redis to avoid previous tests affecting current test
// Clears: rate limit keys, cooldown keys, user lock keys, challenge keys
func clearRateLimitKeys(t *testing.T) error {
	// Connect to Herald's Redis (mapped to localhost:6379 per docker-compose.yml)
	redisClient := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // No password per docker-compose.yml
		DB:       0,  // Herald uses DB 0
	})
	defer func() {
		if err := redisClient.Close(); err != nil {
			t.Logf("Warning: failed to close redis client: %v", err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test connection
	if err := redisClient.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("failed to connect to Redis: %w", err)
	}

	// Clear all test-related keys:
	// - ratelimit:* - Rate limit keys
	// - ratelimit:cooldown:* - Cooldown keys
	// - otp:lock:* - User lock keys
	// - otp:ch:* - Challenge keys (for test isolation)
	patterns := []string{
		"ratelimit:*",
		"ratelimit:cooldown:*",
		"otp:lock:*",
		"otp:ch:*",
	}

	totalCleared := 0
	for _, pattern := range patterns {
		var keys []string
		var cursor uint64

		// Iterate all matching keys using SCAN
		for {
			var scanKeys []string
			var err error
			scanKeys, cursor, err = redisClient.Scan(ctx, cursor, pattern, 100).Result()
			if err != nil {
				return fmt.Errorf("failed to scan keys with pattern %s: %w", pattern, err)
			}
			keys = append(keys, scanKeys...)
			if cursor == 0 {
				break
			}
		}

		if len(keys) > 0 {
			if err := redisClient.Del(ctx, keys...).Err(); err != nil {
				return fmt.Errorf("failed to delete keys with pattern %s: %w", pattern, err)
			}
			totalCleared += len(keys)
			t.Logf("Cleared %d keys matching pattern %s", len(keys), pattern)
		}
	}

	if totalCleared > 0 {
		t.Logf("Total cleared %d test state keys", totalCleared)
	} else {
		t.Logf("No test state keys found to clear")
	}

	return nil
}

// waitForService waits for the service to be ready (HTTP status < 500).
func waitForService(t *testing.T, url string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 2 * time.Second}

	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			if closeErr := resp.Body.Close(); closeErr != nil {
				t.Logf("Warning: failed to close response body: %v", closeErr)
			}
			if resp.StatusCode < 500 {
				return true
			}
		}
		time.Sleep(1 * time.Second)
	}

	return false
}

// ensureServicesReady ensures all services (Stargate, Warden, Herald) are ready and clears rate-limit state.
func ensureServicesReady(t *testing.T) {
	if !waitForService(t, stargateURL+"/_auth", 30*time.Second) {
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
