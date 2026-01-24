package e2e

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/MarvinJWendt/testza"
)

const (
	stargateURL = "http://localhost:8080"
	heraldURL   = "http://localhost:8082"
	wardenURL   = "http://localhost:8081"
	authHost    = "auth.test.localhost"
)

// TestCompleteLoginFlow 测试完整的登录流程
// 1. 发送验证码
// 2. 从 Herald 测试端点获取验证码
// 3. 使用验证码登录
// 4. 验证 forwardAuth 检查返回正确的授权 Header
func TestCompleteLoginFlow(t *testing.T) {
	// 等待服务就绪
	if !waitForService(t, stargateURL+"/_auth", 30*time.Second) {
		t.Fatalf("Stargate service is not ready")
	}
	if !waitForService(t, heraldURL+"/healthz", 30*time.Second) {
		t.Fatalf("Herald service is not ready")
	}
	if !waitForService(t, wardenURL+"/health", 30*time.Second) {
		t.Fatalf("Warden service is not ready")
	}

	// 使用测试用户：13800138000 (admin@example.com)
	testPhone := "13800138000"
	expectedUserID := "test-admin-001"
	expectedEmail := "admin@example.com"
	expectedScopes := "read,write,admin"
	expectedRole := "admin"

	// Step 1: 发送验证码
	t.Log("Step 1: Sending verification code...")
	challengeID, err := sendVerificationCode(t, testPhone)
	testza.AssertNoError(t, err)
	testza.AssertNotNil(t, challengeID)
	t.Logf("Challenge ID: %s", challengeID)

	// Step 2: 从 Herald 测试端点获取验证码
	t.Log("Step 2: Getting verification code from Herald test endpoint...")
	verifyCode, err := getTestCode(t, challengeID)
	testza.AssertNoError(t, err)
	testza.AssertNotNil(t, verifyCode)
	testza.AssertEqual(t, 6, len(verifyCode))
	t.Logf("Verification code: %s", verifyCode)

	// Step 3: 使用验证码登录
	t.Log("Step 3: Logging in with verification code...")
	sessionCookie, err := login(t, testPhone, challengeID, verifyCode)
	testza.AssertNoError(t, err)
	testza.AssertNotNil(t, sessionCookie)
	t.Logf("Session cookie: %s", sessionCookie)

	// Step 4: 验证 forwardAuth 检查
	t.Log("Step 4: Verifying forwardAuth check...")
	authHeaders, err := checkAuth(t, sessionCookie)
	testza.AssertNoError(t, err)
	testza.AssertNotNil(t, authHeaders, "AuthHeaders should not be nil")
	testza.AssertEqual(t, expectedUserID, authHeaders.UserID, "X-Auth-User should match")
	testza.AssertEqual(t, expectedEmail, authHeaders.Email, "X-Auth-Email should match")
	testza.AssertEqual(t, expectedScopes, authHeaders.Scopes, "X-Auth-Scopes should match")
	testza.AssertEqual(t, expectedRole, authHeaders.Role, "X-Auth-Role should match")
	t.Log("✓ All authorization headers verified successfully")
}

// sendVerificationCode 发送验证码请求
func sendVerificationCode(t *testing.T, phone string) (string, error) {
	// 添加延迟以避免限流
	time.Sleep(2 * time.Second)

	url := fmt.Sprintf("%s/_send_verify_code", stargateURL)
	body := fmt.Sprintf("phone=%s", phone)

	req, err := http.NewRequest("POST", url, strings.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Forwarded-Host", authHost)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Logf("Warning: failed to close response body: %v", closeErr)
		}
	}()

	// 处理限流情况
	if resp.StatusCode == http.StatusTooManyRequests {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("rate limited: status code: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var result struct {
		Success     bool   `json:"success"`
		ChallengeID string `json:"challenge_id"`
		ExpiresIn   int    `json:"expires_in"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if !result.Success {
		return "", fmt.Errorf("send verification code failed")
	}

	return result.ChallengeID, nil
}

// getTestCode 从 Herald 测试端点获取验证码
func getTestCode(t *testing.T, challengeID string) (string, error) {
	if challengeID == "" {
		return "", fmt.Errorf("challengeID cannot be empty")
	}

	url := fmt.Sprintf("%s/v1/test/code/%s", heraldURL, challengeID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Logf("Warning: failed to close response body: %v", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var result struct {
		OK          bool   `json:"ok"`
		ChallengeID string `json:"challenge_id"`
		Code        string `json:"code"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if !result.OK {
		return "", fmt.Errorf("get test code failed")
	}

	return result.Code, nil
}

// login 使用验证码登录
func login(t *testing.T, phone, challengeID, verifyCode string) (string, error) {
	url := fmt.Sprintf("%s/_login", stargateURL)
	body := fmt.Sprintf("auth_method=warden&phone=%s&challenge_id=%s&verify_code=%s",
		phone, challengeID, verifyCode)

	req, err := http.NewRequest("POST", url, strings.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Forwarded-Host", authHost)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Logf("Warning: failed to close response body: %v", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	// 提取 Set-Cookie header
	setCookieHeaders := resp.Header.Values("Set-Cookie")
	if len(setCookieHeaders) == 0 {
		return "", fmt.Errorf("no Set-Cookie header found")
	}

	// 查找 session cookie (stargate_session_id)
	var sessionCookie string
	for _, cookieHeader := range setCookieHeaders {
		if strings.Contains(cookieHeader, "stargate_session_id") {
			// 提取 name=value 部分（分号之前）
			parts := strings.Split(cookieHeader, ";")
			if len(parts) > 0 {
				sessionCookie = strings.TrimSpace(parts[0])
				break
			}
		}
	}

	if sessionCookie == "" {
		return "", fmt.Errorf("session cookie not found in Set-Cookie headers")
	}

	return sessionCookie, nil
}

// AuthHeaders 表示 forwardAuth 返回的授权 Header
type AuthHeaders struct {
	UserID string
	Email  string
	Scopes string
	Role   string
}

// checkAuth 验证 forwardAuth 检查
func checkAuth(t *testing.T, sessionCookie string) (*AuthHeaders, error) {
	url := fmt.Sprintf("%s/_auth", stargateURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Host", authHost)
	req.Header.Set("Cookie", sessionCookie)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Logf("Warning: failed to close response body: %v", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	headers := &AuthHeaders{
		UserID: resp.Header.Get("X-Auth-User"),
		Email:  resp.Header.Get("X-Auth-Email"),
		Scopes: resp.Header.Get("X-Auth-Scopes"),
		Role:   resp.Header.Get("X-Auth-Role"),
	}

	return headers, nil
}

// waitForService 等待服务就绪
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
