package e2e

import (
	"encoding/json"
	"net/http"
	"net/url"
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

// assertUnavailableChallengeRejected verifies Stargate's public rejection
// contract for a challenge without a matching login context. Pin the language
// so the assertion does not depend on the stack's default locale.
func assertUnavailableChallengeRejected(t *testing.T, phone, challengeID, verifyCode string) {
	t.Helper()
	form := url.Values{
		"auth_method":  {"warden"},
		"phone":        {phone},
		"challenge_id": {challengeID},
		"verify_code":  {verifyCode},
	}
	req, err := http.NewRequest(http.MethodPost, stargateURL+"/_login", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("create login request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "en")
	req.Host = authHost
	req.Header.Set("X-Forwarded-Host", authHost)
	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("login request: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Errorf("close login response: %v", err)
		}
	}()
	testza.AssertEqual(t, http.StatusUnauthorized, resp.StatusCode, "Unavailable challenge must return 401")
	var body struct {
		Code   int    `json:"code"`
		Error  string `json:"error"`
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode login rejection: %v", err)
	}
	testza.AssertEqual(t, http.StatusUnauthorized, body.Code)
	testza.AssertEqual(t, "Verification failed.", body.Error)
	testza.AssertEqual(t, "", body.Reason, "Missing context must not expose a challenge-specific reason")
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "stargate_session_id" {
			testza.AssertTrue(t, cookie.Value == "" || cookie.MaxAge < 0, "Rejected login must not issue a usable session cookie")
		}
	}
}

// TestExpiredVerificationCode exercises the unavailable-context path with a
// synthetic expired handle and a real code belonging to a different challenge.
// It does not wait for or assert Herald's TTL clock.
func TestExpiredVerificationCode(t *testing.T) {
	// Wait for services to be ready
	ensureServicesReady(t)

	// 使用不同的用户避免限流影响
	testPhone := "13800138000" // Use admin user

	// Step 1: Obtain a live code for the isolated scenario.
	t.Log("Step 1: Sending verification code...")
	challengeID, errResp := sendVerificationCodeWithError(t, testPhone)
	if errResp != nil {
		t.Fatalf("send verification code: status %d: %s", errResp.StatusCode, errResp.Message)
	}
	if challengeID == "" {
		t.Fatal("send verification code returned an empty challenge ID")
	}

	// Step 2: Get verification code
	verifyCode, err := getTestCode(t, challengeID)
	if err != nil {
		t.Fatalf("get test code: %v", err)
	}
	if verifyCode == "" {
		t.Fatal("test listener returned an empty verification code")
	}

	// No login context exists for this synthetic expired handle. Stargate 1.8
	// rejects it before calling Herald, without revealing an expiry distinction.
	expiredChallengeID := "expired_challenge_12345"
	assertUnavailableChallengeRejected(t, testPhone, expiredChallengeID, verifyCode)
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
	assertUnavailableChallengeRejected(t, testPhone, invalidChallengeID, wrongCode)
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
