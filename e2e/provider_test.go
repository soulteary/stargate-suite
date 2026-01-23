package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/MarvinJWendt/testza"
)

// TestHeraldProviderFailure tests Provider failure handling
// Note: In test environment, Provider usually works normally, this mainly verifies failure handling logic
// According to docker-compose.yml, PROVIDER_FAILURE_POLICY=soft, meaning challenge will be created even if send fails
func TestHeraldProviderFailure(t *testing.T) {
	ensureServicesReady(t)

	// In soft mode, challenge should be created even if Provider fails
	// Here we create a normal challenge to verify logic
	t.Log("Testing provider failure handling in soft mode...")

	reqBody := HeraldChallengeRequest{
		UserID:      "test-user-provider-fail",
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
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Logf("Warning: failed to close response body: %v", closeErr)
		}
	}()

	// In soft mode, even if Provider fails, it should return 200 OK (challenge created)
	// If Provider succeeds, it also returns 200 OK
	testza.AssertEqual(t, http.StatusOK, resp.StatusCode, "Should return 200 OK in soft mode even if provider fails")

	var challengeResp HeraldChallengeResponse
	err = json.NewDecoder(resp.Body).Decode(&challengeResp)
	testza.AssertNoError(t, err)

	testza.AssertNotNil(t, challengeResp.ChallengeID)
	testza.AssertTrue(t, len(challengeResp.ChallengeID) > 0, "ChallengeID should not be empty")

	t.Logf("✓ Challenge created successfully: %s", challengeResp.ChallengeID)
	t.Log("Note: In soft mode, challenge is created even if provider fails. Check audit logs for send_failed events.")

	// Verify challenge can still be verified (if Provider failed, get code via test endpoint)
	verifyCode, err := getTestCode(t, challengeResp.ChallengeID)
	if err == nil {
		verifyReqBody := HeraldVerifyRequest{
			ChallengeID: challengeResp.ChallengeID,
			Code:        verifyCode,
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

		if verifyResp.StatusCode == http.StatusOK {
			var verifyRespBody HeraldVerifyResponse
			err = json.NewDecoder(verifyResp.Body).Decode(&verifyRespBody)
			if err == nil && verifyRespBody.OK {
				t.Logf("✓ Challenge can still be verified even if provider failed (soft mode)")
			}
		}
	}
}

// TestHeraldProviderRetry tests Provider retry strategy
// Note: Herald may not have explicit retry logic, this mainly verifies idempotency in retry scenarios
func TestHeraldProviderRetry(t *testing.T) {
	ensureServicesReady(t)

	// Use same Idempotency-Key for "retry" to verify idempotency
	t.Log("Testing provider retry with idempotency...")

	reqBody := HeraldChallengeRequest{
		UserID:      "test-user-provider-retry",
		Channel:     "sms",
		Destination: "+8613800138000",
		Purpose:     "login",
	}

	bodyBytes, err := json.Marshal(reqBody)
	testza.AssertNoError(t, err)

	idempotencyKey := fmt.Sprintf("retry-test-%d", time.Now().UnixNano())

	// First request (simulate initial send)
	url := fmt.Sprintf("%s/v1/otp/challenges", heraldURL)
	req1, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	testza.AssertNoError(t, err)

	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("Accept", "application/json")
	req1.Header.Set("X-API-Key", heraldAPIKey)
	req1.Header.Set("Idempotency-Key", idempotencyKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp1, err := client.Do(req1)
	testza.AssertNoError(t, err)

	var challengeResp1 HeraldChallengeResponse
	err = json.NewDecoder(resp1.Body).Decode(&challengeResp1)
	testza.AssertNoError(t, err)
	resp1.Body.Close()

	testza.AssertEqual(t, http.StatusOK, resp1.StatusCode, "First request should succeed")
	firstChallengeID := challengeResp1.ChallengeID

	t.Logf("First request - ChallengeID: %s", firstChallengeID)

	// Second request (simulate retry, using same Idempotency-Key)
	bodyBytes2, err := json.Marshal(reqBody)
	testza.AssertNoError(t, err)

	req2, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes2))
	testza.AssertNoError(t, err)

	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Accept", "application/json")
	req2.Header.Set("X-API-Key", heraldAPIKey)
	req2.Header.Set("Idempotency-Key", idempotencyKey)

	time.Sleep(1 * time.Second)

	resp2, err := client.Do(req2)
	testza.AssertNoError(t, err)

	var challengeResp2 HeraldChallengeResponse
	err = json.NewDecoder(resp2.Body).Decode(&challengeResp2)
	testza.AssertNoError(t, err)
	resp2.Body.Close()

	testza.AssertEqual(t, http.StatusOK, resp2.StatusCode, "Retry request should succeed")
	testza.AssertEqual(t, firstChallengeID, challengeResp2.ChallengeID,
		"Retry with same Idempotency-Key should return same ChallengeID (no duplicate send)")

	t.Logf("Retry request - ChallengeID: %s (same as first)", challengeResp2.ChallengeID)
	t.Logf("✓ Idempotency prevents duplicate sends on retry")
}

// TestHeraldProviderErrorCodes tests Provider error code normalization
// Note: This mainly verifies error response format, actual Provider error code normalization is handled internally
func TestHeraldProviderErrorCodes(t *testing.T) {
	ensureServicesReady(t)

	// Test invalid channel should return appropriate error code
	t.Log("Testing provider error code normalization...")

	reqBody := HeraldChallengeRequest{
		UserID:      "test-user-error-codes",
		Channel:     "invalid_channel", // Invalid channel
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
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Logf("Warning: failed to close response body: %v", closeErr)
		}
	}()

	testza.AssertEqual(t, http.StatusBadRequest, resp.StatusCode, "Should return 400 Bad Request for invalid channel")

	var errorResp struct {
		OK     bool   `json:"ok"`
		Reason string `json:"reason"`
		Error  string `json:"error,omitempty"`
	}
	err = json.NewDecoder(resp.Body).Decode(&errorResp)
	testza.AssertNoError(t, err)

	testza.AssertFalse(t, errorResp.OK, "Response should indicate failure")
	testza.AssertTrue(t, errorResp.Reason == "invalid_channel" || strings.Contains(errorResp.Reason, "channel"),
		"Error reason should mention invalid channel")

	t.Logf("✓ Error code normalized correctly: reason=%s", errorResp.Reason)
}

// TestHeraldProviderEmail tests Email Provider
func TestHeraldProviderEmail(t *testing.T) {
	ensureServicesReady(t)

	t.Log("Testing email provider...")

	reqBody := HeraldChallengeRequest{
		UserID:      "test-user-email",
		Channel:     "email",
		Destination: "test@example.com",
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
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Logf("Warning: failed to close response body: %v", closeErr)
		}
	}()

	testza.AssertEqual(t, http.StatusOK, resp.StatusCode, "Email challenge creation should succeed")

	var challengeResp HeraldChallengeResponse
	err = json.NewDecoder(resp.Body).Decode(&challengeResp)
	testza.AssertNoError(t, err)

	testza.AssertNotNil(t, challengeResp.ChallengeID)
	t.Logf("✓ Email challenge created successfully: %s", challengeResp.ChallengeID)
	t.Log("Note: Check audit logs for email provider send events")
}

// TestHeraldProviderSMS tests SMS Provider
func TestHeraldProviderSMS(t *testing.T) {
	ensureServicesReady(t)

	t.Log("Testing SMS provider...")

	reqBody := HeraldChallengeRequest{
		UserID:      "test-user-sms",
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
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Logf("Warning: failed to close response body: %v", closeErr)
		}
	}()

	testza.AssertEqual(t, http.StatusOK, resp.StatusCode, "SMS challenge creation should succeed")

	var challengeResp HeraldChallengeResponse
	err = json.NewDecoder(resp.Body).Decode(&challengeResp)
	testza.AssertNoError(t, err)

	testza.AssertNotNil(t, challengeResp.ChallengeID)
	t.Logf("✓ SMS challenge created successfully: %s", challengeResp.ChallengeID)
	t.Log("Note: Check audit logs for SMS provider send events")
}
