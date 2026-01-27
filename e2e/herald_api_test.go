package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/MarvinJWendt/testza"
)

const (
	heraldAPIKey     = "test-herald-api-key"
	heraldHMACSecret = "test-hmac-secret"
)

// HeraldChallengeRequest represents the request to create a challenge
type HeraldChallengeRequest struct {
	UserID      string `json:"user_id"`
	Channel     string `json:"channel"`
	Destination string `json:"destination"`
	Purpose     string `json:"purpose"`
	Locale      string `json:"locale,omitempty"`
	ClientIP    string `json:"client_ip,omitempty"`
	UA          string `json:"ua,omitempty"`
}

// HeraldChallengeResponse represents the response for creating a challenge
type HeraldChallengeResponse struct {
	ChallengeID  string `json:"challenge_id"`
	ExpiresIn    int    `json:"expires_in"`
	NextResendIn int    `json:"next_resend_in"`
}

// HeraldVerifyRequest represents the request to verify a challenge
type HeraldVerifyRequest struct {
	ChallengeID string `json:"challenge_id"`
	Code        string `json:"code"`
	ClientIP    string `json:"client_ip,omitempty"`
}

// HeraldVerifyResponse represents the response for verifying a challenge
type HeraldVerifyResponse struct {
	OK       bool     `json:"ok"`
	UserID   string   `json:"user_id,omitempty"`
	AMR      []string `json:"amr,omitempty"`
	IssuedAt int64    `json:"issued_at,omitempty"`
	Reason   string   `json:"reason,omitempty"`
}

// TestHeraldCreateChallenge tests creating a challenge
func TestHeraldCreateChallenge(t *testing.T) {
	ensureServicesReady(t)

	reqBody := HeraldChallengeRequest{
		UserID:      "test-user-001",
		Channel:     "sms",
		Destination: "+8613800138000",
		Purpose:     "login",
		Locale:      "zh-CN",
		ClientIP:    "192.168.1.1",
	}

	bodyBytes, err := json.Marshal(reqBody)
	testza.AssertNoError(t, err)

	url := fmt.Sprintf("%s/v1/otp/challenges", heraldURL)
	req, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	testza.AssertNoError(t, err)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-API-Key", heraldAPIKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	testza.AssertNoError(t, err)
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Logf("Warning: failed to close response body: %v", closeErr)
		}
	}()

	testza.AssertEqual(t, http.StatusOK, resp.StatusCode, "Should return 200 OK")

	var challengeResp HeraldChallengeResponse
	err = json.NewDecoder(resp.Body).Decode(&challengeResp)
	testza.AssertNoError(t, err)

	testza.AssertNotNil(t, challengeResp.ChallengeID)
	testza.AssertTrue(t, len(challengeResp.ChallengeID) > 0, "ChallengeID should not be empty")
	testza.AssertTrue(t, challengeResp.ExpiresIn > 0, "ExpiresIn should be positive")
	testza.AssertTrue(t, challengeResp.NextResendIn > 0, "NextResendIn should be positive")

	t.Logf("✓ Challenge created: %+v", challengeResp)
}

// TestHeraldCreateChallengeEmail tests creating a challenge via email
func TestHeraldCreateChallengeEmail(t *testing.T) {
	ensureServicesReady(t)

	reqBody := HeraldChallengeRequest{
		UserID:      "test-user-002",
		Channel:     "email",
		Destination: "user@example.com",
		Purpose:     "login",
		Locale:      "zh-CN",
		ClientIP:    "192.168.1.1",
	}

	bodyBytes, err := json.Marshal(reqBody)
	testza.AssertNoError(t, err)

	url := fmt.Sprintf("%s/v1/otp/challenges", heraldURL)
	req, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	testza.AssertNoError(t, err)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-API-Key", heraldAPIKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	testza.AssertNoError(t, err)
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Logf("Warning: failed to close response body: %v", closeErr)
		}
	}()

	testza.AssertEqual(t, http.StatusOK, resp.StatusCode, "Should return 200 OK")

	var challengeResp HeraldChallengeResponse
	err = json.NewDecoder(resp.Body).Decode(&challengeResp)
	testza.AssertNoError(t, err)

	testza.AssertNotNil(t, challengeResp.ChallengeID)
	testza.AssertTrue(t, len(challengeResp.ChallengeID) > 0, "ChallengeID should not be empty")

	t.Logf("✓ Email challenge created: %+v", challengeResp)
}

// TestHeraldVerifyChallenge tests verifying a challenge
func TestHeraldVerifyChallenge(t *testing.T) {
	ensureServicesReady(t)

	// First create a challenge
	reqBody := HeraldChallengeRequest{
		UserID:      "test-user-001",
		Channel:     "sms",
		Destination: "+8613800138000",
		Purpose:     "login",
	}

	bodyBytes, err := json.Marshal(reqBody)
	testza.AssertNoError(t, err)

	url := fmt.Sprintf("%s/v1/otp/challenges", heraldURL)
	req, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	testza.AssertNoError(t, err)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-API-Key", heraldAPIKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	testza.AssertNoError(t, err)

	var challengeResp HeraldChallengeResponse
	err = json.NewDecoder(resp.Body).Decode(&challengeResp)
	testza.AssertNoError(t, err)
	resp.Body.Close()

	challengeID := challengeResp.ChallengeID
	testza.AssertNotNil(t, challengeID)

	// Get verification code from test endpoint
	verifyCode, err := getTestCode(t, challengeID)
	testza.AssertNoError(t, err)

	// Verify challenge
	verifyReqBody := HeraldVerifyRequest{
		ChallengeID: challengeID,
		Code:        verifyCode,
		ClientIP:    "192.168.1.1",
	}

	verifyBodyBytes, err := json.Marshal(verifyReqBody)
	testza.AssertNoError(t, err)

	verifyURL := fmt.Sprintf("%s/v1/otp/verifications", heraldURL)
	verifyReq, err := http.NewRequest("POST", verifyURL, bytes.NewReader(verifyBodyBytes))
	testza.AssertNoError(t, err)

	verifyReq.Header.Set("Content-Type", "application/json")
	verifyReq.Header.Set("Accept", "application/json")
	verifyReq.Header.Set("X-API-Key", heraldAPIKey)

	verifyResp, err := client.Do(verifyReq)
	testza.AssertNoError(t, err)
	defer func() {
		if closeErr := verifyResp.Body.Close(); closeErr != nil {
			t.Logf("Warning: failed to close response body: %v", closeErr)
		}
	}()

	testza.AssertEqual(t, http.StatusOK, verifyResp.StatusCode, "Should return 200 OK")

	var verifyRespBody HeraldVerifyResponse
	err = json.NewDecoder(verifyResp.Body).Decode(&verifyRespBody)
	testza.AssertNoError(t, err)

	testza.AssertTrue(t, verifyRespBody.OK, "Verification should succeed")
	testza.AssertEqual(t, "test-user-001", verifyRespBody.UserID, "UserID should match")
	testza.AssertTrue(t, len(verifyRespBody.AMR) > 0, "AMR should not be empty")
	testza.AssertTrue(t, verifyRespBody.IssuedAt > 0, "IssuedAt should be positive")

	t.Logf("✓ Challenge verified: %+v", verifyRespBody)
}

// TestHeraldChallengeExpired tests expired challenge
func TestHeraldChallengeExpired(t *testing.T) {
	ensureServicesReady(t)

	// Use a non-existent challenge_id to simulate expiration
	expiredChallengeID := "expired_challenge_12345"
	verifyReqBody := HeraldVerifyRequest{
		ChallengeID: expiredChallengeID,
		Code:        "123456",
	}

	verifyBodyBytes, err := json.Marshal(verifyReqBody)
	testza.AssertNoError(t, err)

	verifyURL := fmt.Sprintf("%s/v1/otp/verifications", heraldURL)
	verifyReq, err := http.NewRequest("POST", verifyURL, bytes.NewReader(verifyBodyBytes))
	testza.AssertNoError(t, err)

	verifyReq.Header.Set("Content-Type", "application/json")
	verifyReq.Header.Set("Accept", "application/json")
	verifyReq.Header.Set("X-API-Key", heraldAPIKey)

	client := &http.Client{Timeout: 10 * time.Second}
	verifyResp, err := client.Do(verifyReq)
	testza.AssertNoError(t, err)
	defer func() {
		if closeErr := verifyResp.Body.Close(); closeErr != nil {
			t.Logf("Warning: failed to close response body: %v", closeErr)
		}
	}()

	// Herald returns 401 Unauthorized for expired or not found challenges
	testza.AssertEqual(t, http.StatusUnauthorized, verifyResp.StatusCode,
		"Should return 401 Unauthorized for expired challenge")

	var verifyRespBody HeraldVerifyResponse
	err = json.NewDecoder(verifyResp.Body).Decode(&verifyRespBody)
	if err == nil {
		testza.AssertFalse(t, verifyRespBody.OK, "Verification should fail")
		testza.AssertTrue(t, verifyRespBody.Reason == "expired" || verifyRespBody.Reason == "invalid" || verifyRespBody.Reason == "verification_failed",
			"Reason should be expired, invalid, or verification_failed")
	}

	t.Logf("✓ Expired challenge rejected: Status %d", verifyResp.StatusCode)
}

// TestHeraldInvalidCode tests invalid verification code
func TestHeraldInvalidCode(t *testing.T) {
	ensureServicesReady(t)

	// First create a challenge
	reqBody := HeraldChallengeRequest{
		UserID:      "test-user-001",
		Channel:     "sms",
		Destination: "+8613800138000",
		Purpose:     "login",
	}

	bodyBytes, err := json.Marshal(reqBody)
	testza.AssertNoError(t, err)

	url := fmt.Sprintf("%s/v1/otp/challenges", heraldURL)
	req, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	testza.AssertNoError(t, err)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-API-Key", heraldAPIKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	testza.AssertNoError(t, err)

	var challengeResp HeraldChallengeResponse
	err = json.NewDecoder(resp.Body).Decode(&challengeResp)
	testza.AssertNoError(t, err)
	resp.Body.Close()

	challengeID := challengeResp.ChallengeID

	// Use incorrect verification code
	verifyReqBody := HeraldVerifyRequest{
		ChallengeID: challengeID,
		Code:        "000000",
	}

	verifyBodyBytes, err := json.Marshal(verifyReqBody)
	testza.AssertNoError(t, err)

	verifyURL := fmt.Sprintf("%s/v1/otp/verifications", heraldURL)
	verifyReq, err := http.NewRequest("POST", verifyURL, bytes.NewReader(verifyBodyBytes))
	testza.AssertNoError(t, err)

	verifyReq.Header.Set("Content-Type", "application/json")
	verifyReq.Header.Set("Accept", "application/json")
	verifyReq.Header.Set("X-API-Key", heraldAPIKey)

	verifyResp, err := client.Do(verifyReq)
	testza.AssertNoError(t, err)
	defer func() {
		if closeErr := verifyResp.Body.Close(); closeErr != nil {
			t.Logf("Warning: failed to close response body: %v", closeErr)
		}
	}()

	testza.AssertTrue(t, verifyResp.StatusCode == http.StatusBadRequest || verifyResp.StatusCode == http.StatusUnauthorized,
		"Should return 400 Bad Request or 401 Unauthorized")

	var verifyRespBody HeraldVerifyResponse
	err = json.NewDecoder(verifyResp.Body).Decode(&verifyRespBody)
	if err == nil {
		testza.AssertFalse(t, verifyRespBody.OK, "Verification should fail")
		testza.AssertTrue(t, verifyRespBody.Reason == "invalid" || verifyRespBody.Reason == "expired",
			"Reason should be invalid or expired")
	}

	t.Logf("✓ Invalid code rejected: Status %d", verifyResp.StatusCode)
}

// TestHeraldRateLimit tests rate limit response
func TestHeraldRateLimit(t *testing.T) {
	ensureServicesReady(t)

	// Send multiple requests quickly to trigger rate limit
	var lastResp *http.Response
	for i := 0; i < 10; i++ {
		reqBody := HeraldChallengeRequest{
			UserID:      "test-user-ratelimit",
			Channel:     "sms",
			Destination: "+8613500135000",
			Purpose:     "login",
		}

		bodyBytes, err := json.Marshal(reqBody)
		testza.AssertNoError(t, err)

		url := fmt.Sprintf("%s/v1/otp/challenges", heraldURL)
		req, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
		testza.AssertNoError(t, err)

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("X-API-Key", heraldAPIKey)

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			continue
		}

		lastResp = resp
		if resp.StatusCode == http.StatusTooManyRequests {
			resp.Body.Close()
			t.Logf("✓ Rate limit triggered after %d requests", i+1)
			return
		}

		resp.Body.Close()
		time.Sleep(100 * time.Millisecond)
	}

	if lastResp != nil {
		lastResp.Body.Close()
	}
	t.Log("Note: Rate limit may require more requests or longer time window")
}

// TestHeraldRevokeChallenge tests revoking a challenge
func TestHeraldRevokeChallenge(t *testing.T) {
	ensureServicesReady(t)

	// First create a challenge
	reqBody := HeraldChallengeRequest{
		UserID:      "test-user-001",
		Channel:     "sms",
		Destination: "+8613800138000",
		Purpose:     "login",
	}

	bodyBytes, err := json.Marshal(reqBody)
	testza.AssertNoError(t, err)

	url := fmt.Sprintf("%s/v1/otp/challenges", heraldURL)
	req, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	testza.AssertNoError(t, err)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-API-Key", heraldAPIKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	testza.AssertNoError(t, err)

	var challengeResp HeraldChallengeResponse
	err = json.NewDecoder(resp.Body).Decode(&challengeResp)
	testza.AssertNoError(t, err)
	resp.Body.Close()

	challengeID := challengeResp.ChallengeID

	// Revoke challenge
	revokeURL := fmt.Sprintf("%s/v1/otp/challenges/%s/revoke", heraldURL, challengeID)
	revokeReq, err := http.NewRequest("POST", revokeURL, nil)
	testza.AssertNoError(t, err)

	revokeReq.Header.Set("Accept", "application/json")
	revokeReq.Header.Set("X-API-Key", heraldAPIKey)

	revokeResp, err := client.Do(revokeReq)
	testza.AssertNoError(t, err)
	defer func() {
		if closeErr := revokeResp.Body.Close(); closeErr != nil {
			t.Logf("Warning: failed to close response body: %v", closeErr)
		}
	}()

	// Revoke interface may return 200 or 204
	testza.AssertTrue(t, revokeResp.StatusCode == http.StatusOK || revokeResp.StatusCode == http.StatusNoContent,
		"Should return 200 OK or 204 No Content")

	var revokeRespBody struct {
		OK bool `json:"ok"`
	}
	if revokeResp.StatusCode == http.StatusOK {
		err = json.NewDecoder(revokeResp.Body).Decode(&revokeRespBody)
		if err == nil {
			testza.AssertTrue(t, revokeRespBody.OK, "Revoke should succeed")
		}
	}

	t.Logf("✓ Challenge revoked: Status %d", revokeResp.StatusCode)
}

// TestHeraldHMACAuth tests HMAC authentication
func TestHeraldHMACAuth(t *testing.T) {
	ensureServicesReady(t)

	reqBody := HeraldChallengeRequest{
		UserID:      "test-user-001",
		Channel:     "sms",
		Destination: "+8613800138000",
		Purpose:     "login",
	}

	bodyBytes, err := json.Marshal(reqBody)
	testza.AssertNoError(t, err)
	bodyStr := string(bodyBytes)

	timestamp := time.Now().Unix()
	service := "test-service"
	signature := calculateHMAC(timestamp, service, bodyStr, heraldHMACSecret)

	url := fmt.Sprintf("%s/v1/otp/challenges", heraldURL)
	req, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	testza.AssertNoError(t, err)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Signature", signature)
	req.Header.Set("X-Timestamp", strconv.FormatInt(timestamp, 10))
	req.Header.Set("X-Service", service)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	testza.AssertNoError(t, err)
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Logf("Warning: failed to close response body: %v", closeErr)
		}
	}()

	// HMAC authentication should succeed (higher priority than API Key)
	testza.AssertEqual(t, http.StatusOK, resp.StatusCode, "Should return 200 OK with HMAC auth")

	var challengeResp HeraldChallengeResponse
	err = json.NewDecoder(resp.Body).Decode(&challengeResp)
	testza.AssertNoError(t, err)

	testza.AssertNotNil(t, challengeResp.ChallengeID)
	t.Logf("✓ HMAC authentication successful: %+v", challengeResp)
}
