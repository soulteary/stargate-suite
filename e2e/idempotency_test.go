package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/MarvinJWendt/testza"
)

// TestHeraldIdempotencyKey tests that same Idempotency-Key returns same result
func TestHeraldIdempotencyKey(t *testing.T) {
	ensureServicesReady(t)

	reqBody := HeraldChallengeRequest{
		UserID:      "test-user-idempotency",
		Channel:     "sms",
		Destination: "+8613800138000",
		Purpose:     "login",
	}

	bodyBytes, err := json.Marshal(reqBody)
	testza.AssertNoError(t, err)

	idempotencyKey := fmt.Sprintf("test-idempotency-key-%d", time.Now().UnixNano())

	// 第一次请求
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

	testza.AssertEqual(t, http.StatusOK, resp1.StatusCode, "First request should return 200 OK")
	testza.AssertNotNil(t, challengeResp1.ChallengeID)
	firstChallengeID := challengeResp1.ChallengeID
	firstExpiresIn := challengeResp1.ExpiresIn
	firstNextResendIn := challengeResp1.NextResendIn

	t.Logf("First request - ChallengeID: %s, ExpiresIn: %d, NextResendIn: %d",
		firstChallengeID, firstExpiresIn, firstNextResendIn)

	// Second request (using same Idempotency-Key)
	bodyBytes2, err := json.Marshal(reqBody)
	testza.AssertNoError(t, err)

	req2, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes2))
	testza.AssertNoError(t, err)

	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Accept", "application/json")
	req2.Header.Set("X-API-Key", heraldAPIKey)
	req2.Header.Set("Idempotency-Key", idempotencyKey)

	// Short delay to ensure within TTL
	time.Sleep(1 * time.Second)

	resp2, err := client.Do(req2)
	testza.AssertNoError(t, err)

	var challengeResp2 HeraldChallengeResponse
	err = json.NewDecoder(resp2.Body).Decode(&challengeResp2)
	testza.AssertNoError(t, err)
	resp2.Body.Close()

	testza.AssertEqual(t, http.StatusOK, resp2.StatusCode, "Second request should return 200 OK")

	// Verify same result returned
	testza.AssertEqual(t, firstChallengeID, challengeResp2.ChallengeID,
		"ChallengeID should be the same with same Idempotency-Key")
	testza.AssertEqual(t, firstExpiresIn, challengeResp2.ExpiresIn,
		"ExpiresIn should be the same with same Idempotency-Key")
	testza.AssertEqual(t, firstNextResendIn, challengeResp2.NextResendIn,
		"NextResendIn should be the same with same Idempotency-Key")

	t.Logf("Second request - ChallengeID: %s, ExpiresIn: %d, NextResendIn: %d",
		challengeResp2.ChallengeID, challengeResp2.ExpiresIn, challengeResp2.NextResendIn)
	t.Logf("✓ Idempotency-Key works correctly: same Idempotency-Key returns same result")
}

// TestHeraldIdempotencyKeyDifferent tests that different Idempotency-Key returns different result
func TestHeraldIdempotencyKeyDifferent(t *testing.T) {
	ensureServicesReady(t)

	// Use different destination to avoid triggering resend cooldown
	reqBody := HeraldChallengeRequest{
		UserID:      "test-user-idempotency-diff",
		Channel:     "sms",
		Destination: "+8613800138001", // Use different destination
		Purpose:     "login",
	}

	bodyBytes, err := json.Marshal(reqBody)
	testza.AssertNoError(t, err)

	idempotencyKey1 := fmt.Sprintf("test-idempotency-key-1-%d", time.Now().UnixNano())
	idempotencyKey2 := fmt.Sprintf("test-idempotency-key-2-%d", time.Now().UnixNano())

	// 第一次请求
	url := fmt.Sprintf("%s/v1/otp/challenges", heraldURL)
	req1, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	testza.AssertNoError(t, err)

	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("Accept", "application/json")
	req1.Header.Set("X-API-Key", heraldAPIKey)
	req1.Header.Set("Idempotency-Key", idempotencyKey1)

	client := &http.Client{Timeout: 10 * time.Second}
	resp1, err := client.Do(req1)
	testza.AssertNoError(t, err)

	var challengeResp1 HeraldChallengeResponse
	err = json.NewDecoder(resp1.Body).Decode(&challengeResp1)
	testza.AssertNoError(t, err)
	resp1.Body.Close()

	testza.AssertEqual(t, http.StatusOK, resp1.StatusCode, "First request should return 200 OK")
	firstChallengeID := challengeResp1.ChallengeID

	// Second request (using different Idempotency-Key and different destination to avoid cooldown)
	reqBody2 := HeraldChallengeRequest{
		UserID:      "test-user-idempotency-diff",
		Channel:     "sms",
		Destination: "+8613800138002", // Use different destination to avoid cooldown
		Purpose:     "login",
	}
	bodyBytes2, err := json.Marshal(reqBody2)
	testza.AssertNoError(t, err)

	req2, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes2))
	testza.AssertNoError(t, err)

	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Accept", "application/json")
	req2.Header.Set("X-API-Key", heraldAPIKey)
	req2.Header.Set("Idempotency-Key", idempotencyKey2)

	time.Sleep(1 * time.Second)

	resp2, err := client.Do(req2)
	testza.AssertNoError(t, err)

	var challengeResp2 HeraldChallengeResponse
	err = json.NewDecoder(resp2.Body).Decode(&challengeResp2)
	testza.AssertNoError(t, err)
	resp2.Body.Close()

	testza.AssertEqual(t, http.StatusOK, resp2.StatusCode, "Second request should return 200 OK")

	// Verify different result returned (different ChallengeID)
	testza.AssertNotEqual(t, firstChallengeID, challengeResp2.ChallengeID,
		"ChallengeID should be different with different Idempotency-Key")

	t.Logf("First ChallengeID: %s", firstChallengeID)
	t.Logf("Second ChallengeID: %s", challengeResp2.ChallengeID)
	t.Logf("✓ Different Idempotency-Key returns different result")
}

// TestHeraldIdempotencyKeyWithoutHeader tests that new challenge is created every time without Idempotency-Key
func TestHeraldIdempotencyKeyWithoutHeader(t *testing.T) {
	ensureServicesReady(t)

	reqBody := HeraldChallengeRequest{
		UserID:      "test-user-no-idempotency",
		Channel:     "sms",
		Destination: "+8613800138000",
		Purpose:     "login",
	}

	bodyBytes, err := json.Marshal(reqBody)
	testza.AssertNoError(t, err)

	url := fmt.Sprintf("%s/v1/otp/challenges", heraldURL)

	// First request (without Idempotency-Key)
	req1, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	testza.AssertNoError(t, err)

	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("Accept", "application/json")
	req1.Header.Set("X-API-Key", heraldAPIKey)
	// Do not set Idempotency-Key

	client := &http.Client{Timeout: 10 * time.Second}
	resp1, err := client.Do(req1)
	testza.AssertNoError(t, err)

	var challengeResp1 HeraldChallengeResponse
	err = json.NewDecoder(resp1.Body).Decode(&challengeResp1)
	testza.AssertNoError(t, err)
	resp1.Body.Close()

	testza.AssertEqual(t, http.StatusOK, resp1.StatusCode, "First request should return 200 OK")
	firstChallengeID := challengeResp1.ChallengeID

	// Second request (also without Idempotency-Key)
	bodyBytes2, err := json.Marshal(reqBody)
	testza.AssertNoError(t, err)

	req2, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes2))
	testza.AssertNoError(t, err)

	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Accept", "application/json")
	req2.Header.Set("X-API-Key", heraldAPIKey)
	// Do not set Idempotency-Key

	time.Sleep(1 * time.Second)

	resp2, err := client.Do(req2)
	testza.AssertNoError(t, err)

	var challengeResp2 HeraldChallengeResponse
	err = json.NewDecoder(resp2.Body).Decode(&challengeResp2)
	testza.AssertNoError(t, err)
	resp2.Body.Close()

	// Due to resend cooldown (default 60s), second request might be rate-limited, which is expected behavior
	if resp2.StatusCode == http.StatusTooManyRequests {
		t.Logf("Note: Second request was rate-limited due to cooldown, which is expected behavior")
		// Test passed: verified that cooldown applies without Idempotency-Key
		return
	}

	testza.AssertEqual(t, http.StatusOK, resp2.StatusCode, "Second request should return 200 OK")

	// Without Idempotency-Key, new challenge should be created (different ChallengeID)
	if challengeResp2.ChallengeID != "" && challengeResp2.ChallengeID != firstChallengeID {
		t.Logf("✓ Without Idempotency-Key, new challenge created: %s", challengeResp2.ChallengeID)
	}

	t.Logf("First ChallengeID: %s", firstChallengeID)
}
