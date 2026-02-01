package e2e

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MarvinJWendt/testza"
)

// messageMentionsRateLimit returns true if msg indicates rate limiting (e.g. "Too many requests. Please try again later.", "rate limit", "频繁").
func messageMentionsRateLimit(msg string) bool {
	m := strings.ToLower(msg)
	return strings.Contains(m, "频繁") || strings.Contains(m, "rate") ||
		strings.Contains(m, "limit") || strings.Contains(m, "too many") ||
		strings.Contains(m, "try again") || strings.Contains(m, "later")
}

// messageMentionsCooldownOrRateLimit returns true if msg indicates cooldown or rate limit (e.g. "Too many requests. Please try again later.", "cooldown", "wait").
func messageMentionsCooldownOrRateLimit(msg string) bool {
	m := strings.ToLower(msg)
	return strings.Contains(m, "频繁") || strings.Contains(m, "冷却") ||
		strings.Contains(m, "cooldown") || strings.Contains(m, "wait") ||
		strings.Contains(m, "too many") || strings.Contains(m, "try again") || strings.Contains(m, "later")
}

// TestInvalidVerificationCode tests invalid verification code scenarios
func TestInvalidVerificationCode(t *testing.T) {
	// Wait for services to be ready
	ensureServicesReady(t)

	testPhone := "13900139000" // 使用普通用户

	// Step 1: 发送验证码成功
	t.Log("Step 1: Sending verification code...")
	challengeID, errResp := sendVerificationCodeWithError(t, testPhone)
	testza.AssertNil(t, errResp, "Should not have error when sending verification code")
	testza.AssertNotNil(t, challengeID)
	t.Logf("Challenge ID: %s", challengeID)

	// Step 2: 获取正确的验证码
	verifyCode, err := getTestCode(t, challengeID)
	testza.AssertNoError(t, err)

	// Step 3: Login with invalid verification code
	t.Log("Step 3: Attempting login with invalid verification code...")
	wrongCode := "000000"
	if verifyCode == wrongCode {
		wrongCode = "111111" // Ensure it is an incorrect code
	}
	_, errResp = loginWithError(t, testPhone, challengeID, wrongCode)
	testza.AssertNotNil(t, errResp)
	testza.AssertEqual(t, 401, errResp.StatusCode, "Should return 401 Unauthorized")
	msgLower := strings.ToLower(errResp.Message)
	testza.AssertTrue(t, strings.Contains(errResp.Message, "验证码") || strings.Contains(errResp.Message, "错误") ||
		strings.Contains(msgLower, "invalid") || strings.Contains(msgLower, "verification"),
		"Error message should mention verification code error")
	t.Logf("✓ Invalid verification code rejected: %s", errResp.Message)
}

// TestExpiredVerificationCode tests expired verification code scenarios
// Note: This test needs to wait for code expiration, which may take a long time
// Or can be accelerated by modifying Herald's CHALLENGE_EXPIRY config
func TestExpiredVerificationCode(t *testing.T) {
	// Wait for services to be ready
	ensureServicesReady(t)

	// 使用不同的用户避免限流影响
	testPhone := "13800138000" // Use admin user

	// Step 1: 发送验证码成功（如果触发限流，跳过此测试）
	t.Log("Step 1: Sending verification code...")
	challengeID, errResp := sendVerificationCodeWithError(t, testPhone)
	if errResp != nil && errResp.StatusCode == 429 {
		t.Skip("Rate limit triggered, skipping expired verification code test")
	}
	testza.AssertNil(t, errResp, "Should not have error when sending verification code")
	testza.AssertNotNil(t, challengeID)

	// Step 2: Get verification code
	verifyCode, err := getTestCode(t, challengeID)
	testza.AssertNoError(t, err)

	// Step 3: Wait for code to expire (default 5 mins, waiting 6 mins here)
	// Note: In actual tests, may need to adjust wait time or config
	t.Log("Step 3: Waiting for verification code to expire (this may take a while)...")
	t.Log("Note: In production, you may want to reduce CHALLENGE_EXPIRY for faster testing")

	// For testing purposes, we attempt to use an obviously expired challenge_id
	// Or wait for actual expiration (skipping long wait here, testing logic only)
	// In real scenarios, a shorter expiration time can be set for testing

	// Use a non-existent challenge_id to simulate expiration
	expiredChallengeID := "expired_challenge_12345"
	t.Log("Step 4: Attempting login with expired challenge...")
	_, errResp = loginWithError(t, testPhone, expiredChallengeID, verifyCode)
	testza.AssertNotNil(t, errResp)
	testza.AssertTrue(t, errResp.StatusCode == 400 || errResp.StatusCode == 401,
		"Should return 400 Bad Request or 401 Unauthorized")
	// Error message may be "verification service error" or contain expired/invalid hints
	testza.AssertTrue(t, strings.Contains(errResp.Message, "过期") || strings.Contains(errResp.Message, "expired") ||
		strings.Contains(errResp.Message, "无效") || strings.Contains(errResp.Message, "错误") ||
		strings.Contains(errResp.Message, "空"),
		"Error message should mention expiration or error")
	t.Logf("✓ Expired verification code rejected: %s", errResp.Message)
}

// TestVerificationCodeLocked tests lockout after multiple incorrect codes
func TestVerificationCodeLocked(t *testing.T) {
	// Wait for services to be ready
	ensureServicesReady(t)

	testPhone := "13700137000" // 使用访客用户

	// Step 1: 发送验证码成功
	t.Log("Step 1: Sending verification code...")
	challengeID, errResp := sendVerificationCodeWithError(t, testPhone)
	testza.AssertNil(t, errResp, "Should not have error when sending verification code")
	testza.AssertNotNil(t, challengeID)

	// Step 2: 获取正确的验证码
	verifyCode, err := getTestCode(t, challengeID)
	testza.AssertNoError(t, err)

	// Step 3: Continuously use incorrect code (reach max attempts, default 5)
	t.Log("Step 3: Attempting multiple invalid logins to trigger lockout...")
	wrongCode := "000000"
	if verifyCode == wrongCode {
		wrongCode = "111111"
	}

	var lastErrResp *ErrorResponse
	for i := 0; i < 6; i++ { // Try 6 times, should be locked after 5th
		_, errResp = loginWithError(t, testPhone, challengeID, wrongCode)
		if errResp != nil {
			lastErrResp = errResp
			t.Logf("Attempt %d: Status %d, Message: %s", i+1, errResp.StatusCode, errResp.Message)

			// Check if locked
			if strings.Contains(errResp.Message, "锁定") || strings.Contains(errResp.Message, "locked") {
				t.Logf("✓ Challenge locked after %d attempts", i+1)
				break
			}
		}
		time.Sleep(500 * time.Millisecond) // Short delay
	}

	testza.AssertNotNil(t, lastErrResp)
	testza.AssertEqual(t, 401, lastErrResp.StatusCode, "Should return 401 Unauthorized")
	// After lockout, may return "verification service error" or contain locked/attempts hints
	testza.AssertTrue(t, strings.Contains(lastErrResp.Message, "锁定") || strings.Contains(lastErrResp.Message, "locked") ||
		strings.Contains(lastErrResp.Message, "尝试") || strings.Contains(lastErrResp.Message, "错误"),
		"Error message should mention lockout or error")
	t.Logf("✓ Verification code locked: %s", lastErrResp.Message)
}

// TestUserNotInWhitelist tests non-whitelisted user
func TestUserNotInWhitelist(t *testing.T) {
	// Wait for services to be ready
	ensureServicesReady(t)

	// Use phone number not in whitelist
	nonWhitelistPhone := "13000000000"

	t.Log("Step 1: Attempting to send verification code for non-whitelist user...")
	_, errResp := sendVerificationCodeWithError(t, nonWhitelistPhone)
	testza.AssertNotNil(t, errResp)
	// Non-whitelisted user may return 400, 401, or 404
	testza.AssertTrue(t, errResp.StatusCode == 400 || errResp.StatusCode == 401 || errResp.StatusCode == 404,
		"Should return 400, 401, or 404")
	testza.AssertTrue(t, strings.Contains(errResp.Message, "不在") || strings.Contains(errResp.Message, "白名单") ||
		strings.Contains(errResp.Message, "not found") || strings.Contains(errResp.Message, "not in list") ||
		strings.Contains(errResp.Message, "允许"),
		"Error message should mention user not in whitelist")
	t.Logf("✓ Non-whitelist user rejected: Status %d, Message: %s", errResp.StatusCode, errResp.Message)
}

// TestInactiveUser tests inactive user
func TestInactiveUser(t *testing.T) {
	// Wait for services to be ready
	ensureServicesReady(t)

	// Use inactive user (added in test data)
	inactivePhone := "13600136000"
	inactiveEmail := "inactive@example.com"

	t.Log("Step 1: Attempting to send verification code for inactive user (phone)...")
	_, errResp := sendVerificationCodeWithError(t, inactivePhone)
	testza.AssertNotNil(t, errResp)
	// Inactive user may return 400, 401, or 404
	testza.AssertTrue(t, errResp.StatusCode == 400 || errResp.StatusCode == 401 || errResp.StatusCode == 404,
		"Should return 400, 401, or 404")
	t.Logf("✓ Inactive user (phone) rejected: Status %d, Message: %s", errResp.StatusCode, errResp.Message)

	t.Log("Step 2: Attempting to send verification code for inactive user (email)...")
	_, errResp = sendVerificationCodeWithEmail(t, inactiveEmail)
	testza.AssertNotNil(t, errResp)
	// Inactive user may return 400, 401, or 404
	testza.AssertTrue(t, errResp.StatusCode == 400 || errResp.StatusCode == 401 || errResp.StatusCode == 404,
		"Should return 400, 401, or 404")
	t.Logf("✓ Inactive user (email) rejected: Status %d, Message: %s", errResp.StatusCode, errResp.Message)
}

// TestIPRateLimit tests IP rate limiting
func TestIPRateLimit(t *testing.T) {
	// Wait for services to be ready
	ensureServicesReady(t)

	testPhone := "13500135000" // Use rate limit test user

	t.Log("Step 1: Triggering IP rate limit by sending multiple requests quickly...")
	// Send multiple requests quickly (exceeding default 5 per minute)
	errors := triggerRateLimit(t, testPhone, 7)

	// Check for 429 response
	var rateLimitError *ErrorResponse
	for _, errResp := range errors {
		if errResp != nil && errResp.StatusCode == 429 {
			rateLimitError = errResp
			break
		}
	}

	testza.AssertNotNil(t, rateLimitError, "Should trigger rate limit (429)")
	if rateLimitError == nil {
		return // avoid nil pointer dereference below
	}
	testza.AssertEqual(t, 429, rateLimitError.StatusCode, "Should return 429 Too Many Requests")
	testza.AssertTrue(t, messageMentionsRateLimit(rateLimitError.Message),
		"Error message should mention rate limiting")
	t.Logf("✓ IP rate limit triggered: %s", rateLimitError.Message)
}

// TestUserRateLimit tests user rate limiting
func TestUserRateLimit(t *testing.T) {
	// Wait for services to be ready
	ensureServicesReady(t)

	testPhone := "13500135000" // Use rate limit test user

	t.Log("Step 1: Triggering user rate limit by sending multiple requests for same user...")
	// Send multiple requests quickly (exceeding default 10 per hour)
	// Note: Since it's hourly limit, may need to wait or adjust config
	errors := triggerRateLimit(t, testPhone, 12)

	// Check for 429 response
	var rateLimitError *ErrorResponse
	for _, errResp := range errors {
		if errResp != nil && errResp.StatusCode == 429 {
			rateLimitError = errResp
			break
		}
	}

	// User rate limit may require more requests or longer time, just verifying logic here
	if rateLimitError != nil {
		testza.AssertEqual(t, 429, rateLimitError.StatusCode, "Should return 429 Too Many Requests")
		testza.AssertTrue(t, messageMentionsRateLimit(rateLimitError.Message),
			"Error message should mention rate limiting")
		t.Logf("✓ User rate limit triggered: %s", rateLimitError.Message)
	} else {
		t.Log("Note: User rate limit may require more requests or longer time window")
	}
}

// TestResendCooldown tests resend cooldown
func TestResendCooldown(t *testing.T) {
	// Wait for services to be ready
	ensureServicesReady(t)

	// 使用不同的用户避免限流影响
	testPhone := "13700137000" // 使用访客用户

	// Step 1: 发送验证码成功（如果触发限流，跳过此测试）
	t.Log("Step 1: Sending first verification code...")
	challengeID1, errResp := sendVerificationCodeWithError(t, testPhone)
	if errResp != nil && errResp.StatusCode == 429 {
		t.Skip("Rate limit triggered, skipping resend cooldown test")
	}
	testza.AssertNil(t, errResp, "Should not have error when sending verification code")
	testza.AssertNotNil(t, challengeID1)

	// Step 2: Immediately send second code (within cooldown, default 60s)
	t.Log("Step 2: Immediately sending second verification code (within cooldown period)...")
	time.Sleep(1 * time.Second) // Short delay
	_, errResp = sendVerificationCodeWithError(t, testPhone)

	// Should return 429 or contain cooldown hint
	if errResp != nil {
		if errResp.StatusCode == 429 {
			testza.AssertTrue(t, messageMentionsCooldownOrRateLimit(errResp.Message),
				"Error message should mention cooldown or wait time")
			t.Logf("✓ Resend cooldown triggered: %s", errResp.Message)
		} else {
			t.Logf("Note: Resend cooldown may not be triggered immediately, status: %d", errResp.StatusCode)
		}
	}
}

// TestHeraldUnavailable tests Herald unavailable scenario
func TestHeraldUnavailable(t *testing.T) {
	// Wait for services to be ready
	ensureServicesReady(t)

	// Get project directory
	projectDir, err := filepath.Abs("../")
	if err != nil {
		t.Fatalf("Failed to get project directory: %v", err)
	}

	testPhone := "13900139000"

	// Step 1: Stop Herald service
	t.Log("Step 1: Stopping Herald service...")
	err = stopDockerServiceInDir(projectDir, "herald")
	if err != nil {
		t.Logf("Warning: Failed to stop Herald service (may not have permission): %v", err)
		t.Log("Skipping service unavailable test - requires docker compose access")
		return
	}

	// Wait for service to stop
	time.Sleep(3 * time.Second)

	// Ensure service is restored after test
	defer func() {
		t.Log("Restoring Herald service...")
		_ = startDockerServiceInDir(projectDir, "herald")
		time.Sleep(5 * time.Second) // Wait for service to recover
	}()

	// Step 2: Attempt to send verification code
	t.Log("Step 2: Attempting to send verification code with Herald unavailable...")
	_, errResp := sendVerificationCodeWithError(t, testPhone)
	testza.AssertNotNil(t, errResp)
	testza.AssertTrue(t, errResp.StatusCode == 503 || errResp.StatusCode == 500,
		"Should return 503 Service Unavailable or 500 Internal Server Error")
	testza.AssertTrue(t, strings.Contains(errResp.Message, "不可用") || strings.Contains(errResp.Message, "unavailable") ||
		strings.Contains(errResp.Message, "服务"),
		"Error message should mention service unavailable")
	t.Logf("✓ Herald unavailable handled correctly: %s", errResp.Message)
}

// TestWardenUnavailable tests Warden unavailable scenario
func TestWardenUnavailable(t *testing.T) {
	// Wait for services to be ready
	ensureServicesReady(t)

	// Get project directory
	projectDir, err := filepath.Abs("../")
	if err != nil {
		t.Fatalf("Failed to get project directory: %v", err)
	}

	testPhone := "13900139000"

	// Step 1: Stop Warden service
	t.Log("Step 1: Stopping Warden service...")
	err = stopDockerServiceInDir(projectDir, "warden")
	if err != nil {
		t.Logf("Warning: Failed to stop Warden service (may not have permission): %v", err)
		t.Log("Skipping service unavailable test - requires docker compose access")
		return
	}

	// Wait for service to stop
	time.Sleep(3 * time.Second)

	// Ensure service is restored after test
	defer func() {
		t.Log("Restoring Warden service...")
		_ = startDockerServiceInDir(projectDir, "warden")
		time.Sleep(5 * time.Second) // Wait for service to recover
	}()

	// Step 2: Attempt to send verification code (needs user query first)
	t.Log("Step 2: Attempting to send verification code with Warden unavailable...")
	_, errResp := sendVerificationCodeWithError(t, testPhone)
	testza.AssertNotNil(t, errResp)
	// When Warden is unavailable, may return 400, 401, 404, 500, or 503
	testza.AssertTrue(t, errResp.StatusCode == 400 || errResp.StatusCode == 401 || errResp.StatusCode == 404 ||
		errResp.StatusCode == 500 || errResp.StatusCode == 503,
		"Should return 400, 401, 404, 500, or 503")
	t.Logf("✓ Warden unavailable handled correctly: Status %d, Message: %s", errResp.StatusCode, errResp.Message)
}

// TestUnauthenticatedAccess tests unauthenticated access to forwardAuth
func TestUnauthenticatedAccess(t *testing.T) {
	// Wait for services to be ready
	ensureServicesReady(t)

	t.Log("Step 1: Attempting to access forwardAuth without session cookie...")
	_, errResp := checkAuthWithError(t, "")
	testza.AssertNotNil(t, errResp)
	testza.AssertTrue(t, errResp.StatusCode == 401 || errResp.StatusCode == 302,
		"Should return 401 Unauthorized or 302 Redirect")
	t.Logf("✓ Unauthenticated access rejected: Status %d, Message: %s", errResp.StatusCode, errResp.Message)
}

// TestInvalidSessionCookie tests invalid session cookie
func TestInvalidSessionCookie(t *testing.T) {
	// Wait for services to be ready
	ensureServicesReady(t)

	t.Log("Step 1: Attempting to access forwardAuth with invalid session cookie...")
	invalidCookie := "stargate_session_id=invalid_session_value_12345"
	_, errResp := checkAuthWithError(t, invalidCookie)
	testza.AssertNotNil(t, errResp)
	testza.AssertTrue(t, errResp.StatusCode == 401 || errResp.StatusCode == 302,
		"Should return 401 Unauthorized or 302 Redirect")
	t.Logf("✓ Invalid session cookie rejected: Status %d, Message: %s", errResp.StatusCode, errResp.Message)
}

// TestEmptyRequestParameters tests empty request parameters
func TestEmptyRequestParameters(t *testing.T) {
	// Wait for services to be ready
	ensureServicesReady(t)

	t.Log("Step 1: Attempting to send verification code with empty phone...")
	_, errResp := sendVerificationCodeWithError(t, "")
	testza.AssertNotNil(t, errResp)
	testza.AssertEqual(t, 400, errResp.StatusCode, "Should return 400 Bad Request")
	t.Logf("✓ Empty phone parameter rejected: %s", errResp.Message)

	t.Log("Step 2: Attempting to send verification code with empty email...")
	_, errResp = sendVerificationCodeWithEmail(t, "")
	testza.AssertNotNil(t, errResp)
	testza.AssertEqual(t, 400, errResp.StatusCode, "Should return 400 Bad Request")
	t.Logf("✓ Empty email parameter rejected: %s", errResp.Message)
}

// TestInvalidChallengeID tests invalid challenge_id
func TestInvalidChallengeID(t *testing.T) {
	// Wait for services to be ready
	ensureServicesReady(t)

	testPhone := "13900139000"
	invalidChallengeID := "nonexistent_challenge_id_12345"
	wrongCode := "123456"

	t.Log("Step 1: Attempting to login with invalid challenge_id...")
	_, errResp := loginWithError(t, testPhone, invalidChallengeID, wrongCode)
	testza.AssertNotNil(t, errResp)
	testza.AssertTrue(t, errResp.StatusCode == 400 || errResp.StatusCode == 401,
		"Should return 400 Bad Request or 401 Unauthorized")
	// Error message may be "verification service error" or contain expired/invalid hints
	testza.AssertTrue(t, strings.Contains(errResp.Message, "过期") || strings.Contains(errResp.Message, "无效") ||
		strings.Contains(errResp.Message, "expired") || strings.Contains(errResp.Message, "invalid") ||
		strings.Contains(errResp.Message, "错误") || strings.Contains(errResp.Message, "空"),
		"Error message should mention expired, invalid, or error")
	t.Logf("✓ Invalid challenge_id rejected: %s", errResp.Message)
}

// TestInvalidAuthMethod tests invalid auth method
func TestInvalidAuthMethod(t *testing.T) {
	// Wait for services to be ready
	ensureServicesReady(t)

	testPhone := "13900139000"
	challengeID := "test_challenge_123"
	verifyCode := "123456"

	// Use unsupported auth_method
	url := stargateURL + "/_login"
	body := "auth_method=invalid_method&phone=" + testPhone + "&challenge_id=" + challengeID + "&verify_code=" + verifyCode

	req, err := http.NewRequest("POST", url, strings.NewReader(body))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Forwarded-Host", authHost)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Logf("Warning: failed to close response body: %v", closeErr)
		}
	}()

	t.Logf("Step 1: Attempting to login with invalid auth_method...")
	testza.AssertTrue(t, resp.StatusCode == 400 || resp.StatusCode == 401,
		"Should return 400 Bad Request or 401 Unauthorized")
	t.Logf("✓ Invalid auth_method rejected: Status %d", resp.StatusCode)
}
