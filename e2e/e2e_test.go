package e2e

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/MarvinJWendt/testza"
)

// TestCompleteLoginFlow tests the complete login flow
// 1. Send verification code
// 2. Get verification code from Herald test endpoint
// 3. Login with verification code
// 4. Verify forwardAuth check returns correct authorization headers
func TestCompleteLoginFlow(t *testing.T) {
	ensureServicesReady(t)

	// Use test user: 13800138000 (admin@example.com)
	testPhone := "13800138000"
	expectedUserID := "test-admin-001"
	expectedEmail := "admin@example.com"
	expectedScopes := "read,write,admin"
	expectedRole := "admin"

	// Step 1: Send verification code
	t.Log("Step 1: Sending verification code...")
	challengeID, err := sendVerificationCode(t, testPhone)
	testza.AssertNoError(t, err)
	testza.AssertNotNil(t, challengeID)
	t.Logf("Challenge ID: %s", challengeID)

	// Step 2: Get verification code from Herald test endpoint
	t.Log("Step 2: Getting verification code from Herald test endpoint...")
	verifyCode, err := getTestCode(t, challengeID)
	testza.AssertNoError(t, err)
	testza.AssertNotNil(t, verifyCode)
	testza.AssertEqual(t, 6, len(verifyCode))
	t.Logf("Verification code: %s", verifyCode)

	// Step 3: Login with verification code
	t.Log("Step 3: Logging in with verification code...")
	sessionCookie, err := login(t, testPhone, challengeID, verifyCode)
	testza.AssertNoError(t, err)
	testza.AssertNotNil(t, sessionCookie)
	t.Logf("Session cookie: %s", sessionCookie)

	// Step 4: Verify forwardAuth check
	t.Log("Step 4: Verifying forwardAuth check...")
	authHeaders, err := checkAuth(t, sessionCookie)
	testza.AssertNoError(t, err)
	testza.AssertNotNil(t, authHeaders, "AuthHeaders should not be nil")
	if authHeaders == nil {
		return // avoid nil pointer dereference in assertions below
	}
	testza.AssertEqual(t, expectedUserID, authHeaders.UserID, "X-Auth-User should match")
	testza.AssertEqual(t, expectedEmail, authHeaders.Email, "X-Auth-Email should match")
	testza.AssertEqual(t, expectedScopes, authHeaders.Scopes, "X-Auth-Scopes should match")
	testza.AssertEqual(t, expectedRole, authHeaders.Role, "X-Auth-Role should match")
	t.Log("✓ All authorization headers verified successfully")
}

// sendVerificationCode sends a verification code request (success path; uses sendVerificationCodeWithError).
func sendVerificationCode(t *testing.T, phone string) (string, error) {
	time.Sleep(2 * time.Second) // avoid rate limiting
	challengeID, errResp := sendVerificationCodeWithError(t, phone)
	if errResp != nil {
		return "", fmt.Errorf("status %d: %s", errResp.StatusCode, errResp.Message)
	}
	return challengeID, nil
}

// getTestCode gets the verification code from Herald test endpoint
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

// login logs in with verification code (success path; uses loginWithError).
func login(t *testing.T, phone, challengeID, verifyCode string) (string, error) {
	sessionCookie, errResp := loginWithError(t, phone, challengeID, verifyCode)
	if errResp != nil {
		return "", fmt.Errorf("status %d: %s", errResp.StatusCode, errResp.Message)
	}
	return sessionCookie, nil
}

// checkAuth verifies the forwardAuth check (success path; uses checkAuthWithError).
func checkAuth(t *testing.T, sessionCookie string) (*AuthHeaders, error) {
	headers, errResp := checkAuthWithError(t, sessionCookie)
	if errResp != nil {
		return nil, fmt.Errorf("status %d: %s", errResp.StatusCode, errResp.Message)
	}
	return headers, nil
}
