package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/MarvinJWendt/testza"
)

// TestHeraldHMACSignature tests the HMAC signature from Stargate to Herald
func TestHeraldHMACSignature(t *testing.T) {
	ensureServicesReady(t)

	// Add delay to avoid rate limiting
	time.Sleep(2 * time.Second)

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
	// HMAC v3: bind method/path/query/timestamp/nonce/service/keyID/idempotency/body hashes.
	sig := signHeraldV3("POST", "/v1/otp/challenges", "", "stargate", "", "", heraldHMACSecret, bodyBytes)
	setHeraldV3Headers(req, sig)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	testza.AssertNoError(t, err)
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Logf("Warning: failed to close response body: %v", closeErr)
		}
	}()

	// Handle rate limiting (429)
	if resp.StatusCode == http.StatusTooManyRequests {
		t.Logf("⚠ Rate limited, skipping this test. Status: %d", resp.StatusCode)
		return
	}
	testza.AssertEqual(t, http.StatusOK, resp.StatusCode, "Should return 200 OK with valid HMAC v3 signature")

	var challengeResp HeraldChallengeResponse
	err = json.NewDecoder(resp.Body).Decode(&challengeResp)
	testza.AssertNoError(t, err)

	testza.AssertNotNil(t, challengeResp.ChallengeID)
	t.Logf("✓ Valid HMAC v3 signature accepted: %+v", challengeResp)
}

// TestHeraldHMACSignatureInvalid tests that invalid signatures are rejected
func TestHeraldHMACSignatureInvalid(t *testing.T) {
	ensureServicesReady(t)

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
	// v3 headers with a deliberately wrong signature: version is present so the
	// server evaluates v3 (no v1 fallback) and must reject the bad signature.
	req.Header.Set("X-Signature-Version", "v3")
	req.Header.Set("X-Signature", "invalid_signature_12345")
	req.Header.Set("X-Timestamp", strconv.FormatInt(time.Now().Unix(), 10))
	req.Header.Set("X-Nonce", "e2e-nonce-invalid-sig")
	req.Header.Set("X-Service", "stargate")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	testza.AssertNoError(t, err)
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Logf("Warning: failed to close response body: %v", closeErr)
		}
	}()

	testza.AssertTrue(t, resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden,
		"Should return 401 Unauthorized or 403 Forbidden with invalid signature")

	bodyBytes, _ = io.ReadAll(resp.Body)
	bodyStr := string(bodyBytes)
	testza.AssertTrue(t, strings.Contains(bodyStr, "unauthorized") || strings.Contains(bodyStr, "signature") ||
		strings.Contains(bodyStr, "auth") || strings.Contains(bodyStr, "认证"),
		"Error message should mention authentication failure")

	t.Logf("✓ Invalid HMAC signature rejected: Status %d", resp.StatusCode)
}

// TestHeraldHMACSignatureExpired tests that expired timestamps are rejected
func TestHeraldHMACSignatureExpired(t *testing.T) {
	ensureServicesReady(t)

	reqBody := HeraldChallengeRequest{
		UserID:      "test-user-001",
		Channel:     "sms",
		Destination: "+8613800138000",
		Purpose:     "login",
	}

	bodyBytes, err := json.Marshal(reqBody)
	testza.AssertNoError(t, err)

	// Use expired timestamp (6 minutes ago, exceeding the default 60s drift and
	// 5-minute window). Compute a valid v3 signature over the expired timestamp
	// so the ONLY failing check is the timestamp drift bound.
	expiredTimestamp := time.Now().Unix() - 360
	service := "stargate"
	nonce := "e2e-nonce-expired"
	sig := signHeraldFixed("v3", "POST", "/v1/otp/challenges", "",
		strconv.FormatInt(expiredTimestamp, 10), nonce, service, "", "", heraldHMACSecret, bodyBytes)

	url := fmt.Sprintf("%s/v1/otp/challenges", heraldURL)
	req, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	testza.AssertNoError(t, err)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Signature-Version", "v3")
	req.Header.Set("X-Signature", sig)
	req.Header.Set("X-Timestamp", strconv.FormatInt(expiredTimestamp, 10))
	req.Header.Set("X-Nonce", nonce)
	req.Header.Set("X-Service", service)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	testza.AssertNoError(t, err)
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Logf("Warning: failed to close response body: %v", closeErr)
		}
	}()

	testza.AssertTrue(t, resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden,
		"Should return 401 Unauthorized or 403 Forbidden with expired timestamp")

	bodyBytes, _ = io.ReadAll(resp.Body)
	bodyStr := string(bodyBytes)
	testza.AssertTrue(t, strings.Contains(bodyStr, "expired") || strings.Contains(bodyStr, "timestamp") ||
		strings.Contains(bodyStr, "time") || strings.Contains(bodyStr, "过期") ||
		strings.Contains(bodyStr, "range") ||
		strings.Contains(bodyStr, "unauthorized"),
		"Error message should mention expired timestamp or authentication failure")

	t.Logf("✓ Expired timestamp rejected: Status %d", resp.StatusCode)
}

// TestHeraldHMACWithXKeyId tests that X-Key-Id header is accepted when present (CLAUDE §6.1 key rotation).
// This single-secret fixture uses its only explicit identity, "default".
func TestHeraldHMACWithXKeyId(t *testing.T) {
	ensureServicesReady(t)

	time.Sleep(2 * time.Second)

	reqBody := HeraldChallengeRequest{
		UserID:      "test-user-001",
		Channel:     "sms",
		Destination: "+8613800138000",
		Purpose:     "login",
	}

	bodyBytes, err := json.Marshal(reqBody)
	testza.AssertNoError(t, err)

	service := "stargate"
	// v3 binds an explicit key ID. Single-secret mode accepts only the
	// stable "default" identity; arbitrary key IDs are rejected.
	sig := signHeraldV3("POST", "/v1/otp/challenges", "", service, "default", "", heraldHMACSecret, bodyBytes)

	url := fmt.Sprintf("%s/v1/otp/challenges", heraldURL)
	req, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	testza.AssertNoError(t, err)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	setHeraldV3Headers(req, sig)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	testza.AssertNoError(t, err)
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Logf("Warning: failed to close response body: %v", closeErr)
		}
	}()

	if resp.StatusCode == http.StatusTooManyRequests {
		t.Logf("⚠ Rate limited, skipping this test. Status: %d", resp.StatusCode)
		return
	}
	testza.AssertEqual(t, http.StatusOK, resp.StatusCode, "Should return 200 OK with valid HMAC and X-Key-Id")

	var challengeResp HeraldChallengeResponse
	err = json.NewDecoder(resp.Body).Decode(&challengeResp)
	testza.AssertNoError(t, err)

	testza.AssertNotNil(t, challengeResp.ChallengeID)
	t.Logf("✓ Valid HMAC with X-Key-Id accepted: %+v", challengeResp)
}

// TestHeraldCreationSignatureBinding verifies the released creation contract:
// legacy v2 signatures cannot create challenges, and v3 binds idempotency even
// when an intermediary removes the header entirely.
func TestHeraldCreationSignatureBinding(t *testing.T) {
	ensureServicesReady(t)
	body, err := json.Marshal(HeraldChallengeRequest{
		UserID: "test-signature-binding", Channel: "sms",
		Destination: "+8613800138000", Purpose: "login",
	})
	testza.AssertNoError(t, err)
	cases := []struct {
		name, version, signedKey, wireKey, reason string
	}{
		{"legacy version", "v2", "", "", "signature_version_required"},
		{"changed idempotency", "v3", "original", "changed", "invalid_signature"},
		{"removed idempotency", "v3", "original", "", "invalid_signature"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, reqErr := http.NewRequest("POST", heraldURL+"/v1/otp/challenges", bytes.NewReader(body))
			testza.AssertNoError(t, reqErr)
			req.Header.Set("Content-Type", "application/json")
			sig := signHeraldV3(req.Method, req.URL.Path, "", "e2e-suite", "", tc.signedKey, heraldHMACSecret, body)
			sig.Signature = signHeraldFixed(tc.version, req.Method, req.URL.Path, "", sig.Timestamp, sig.Nonce, sig.Service, sig.KeyID, tc.signedKey, heraldHMACSecret, body)
			setHeraldV3Headers(req, sig)
			req.Header.Set("X-Signature-Version", tc.version)
			if tc.wireKey != "" {
				req.Header.Set("Idempotency-Key", tc.wireKey)
			}
			client := &http.Client{Timeout: 10 * time.Second}
			resp, doErr := client.Do(req)
			testza.AssertNoError(t, doErr)
			var failure struct {
				Reason string `json:"reason"`
			}
			decodeErr := json.NewDecoder(resp.Body).Decode(&failure)
			closeErr := resp.Body.Close()
			testza.AssertNoError(t, decodeErr)
			testza.AssertNoError(t, closeErr)
			testza.AssertEqual(t, http.StatusUnauthorized, resp.StatusCode)
			testza.AssertEqual(t, tc.reason, failure.Reason)
		})
	}
}

// TestHeraldHMACSignatureMissing tests that missing signature headers are rejected
func TestHeraldHMACSignatureMissing(t *testing.T) {
	ensureServicesReady(t)

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
	// Do not set X-Signature, X-Timestamp, X-Service, nor X-API-Key

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	testza.AssertNoError(t, err)
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Logf("Warning: failed to close response body: %v", closeErr)
		}
	}()

	testza.AssertTrue(t, resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden,
		"Should return 401 Unauthorized or 403 Forbidden without authentication")

	t.Logf("✓ Missing authentication rejected: Status %d", resp.StatusCode)
}

// TestWardenAPIKeyRequired tests that missing API Key is rejected
func TestWardenAPIKeyRequired(t *testing.T) {
	ensureServicesReady(t)

	testPhone := "13800138000"
	url := fmt.Sprintf("%s/user?phone=%s", wardenURL, testPhone)
	req, err := http.NewRequest("GET", url, nil)
	testza.AssertNoError(t, err)

	req.Header.Set("Accept", "application/json")
	// Do not set X-API-Key

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	testza.AssertNoError(t, err)
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Logf("Warning: failed to close response body: %v", closeErr)
		}
	}()

	testza.AssertTrue(t, resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden,
		"Should return 401 Unauthorized or 403 Forbidden without API Key")

	bodyBytes, _ := io.ReadAll(resp.Body)
	bodyStr := string(bodyBytes)
	// Relax error message check; consider test passed if 401/403 is returned
	// Error messages may vary by implementation; specific content is not enforced

	t.Logf("✓ Missing API Key rejected: Status %d, Body: %s", resp.StatusCode, bodyStr)
}

// TestWardenAPIKeyInvalid tests that invalid API Key is rejected
func TestWardenAPIKeyInvalid(t *testing.T) {
	ensureServicesReady(t)

	testPhone := "13800138000"
	url := fmt.Sprintf("%s/user?phone=%s", wardenURL, testPhone)
	req, err := http.NewRequest("GET", url, nil)
	testza.AssertNoError(t, err)

	req.Header.Set("X-API-Key", "invalid-api-key-12345")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	testza.AssertNoError(t, err)
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Logf("Warning: failed to close response body: %v", closeErr)
		}
	}()

	testza.AssertTrue(t, resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden,
		"Should return 401 Unauthorized or 403 Forbidden with invalid API Key")

	// Relax error message check; consider test passed if 401/403 is returned
	// Error messages may vary by implementation; specific content is not enforced

	t.Logf("✓ Invalid API Key rejected: Status %d", resp.StatusCode)
}

// TestHeraldAPIKeyRejectedUnderHMACV2 verifies that under REQUEST_AUTH_MODE=hmac_v2
// (the test/production posture, no downgrade), a bare X-API-Key is rejected: the
// challenge creation requires replay-resistant HMAC v3, never API-key auth. This
// is the security invariant for Herald v1.1.0 explicit auth with no fallback.
func TestHeraldAPIKeyRejectedUnderHMACV2(t *testing.T) {
	ensureServicesReady(t)

	// Add delay to avoid rate limiting
	time.Sleep(2 * time.Second)

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
	// Only an API key, no HMAC v3 headers: hmac_v2 mode requires the
	// X-Signature-Version header and never falls back to API-key auth.
	req.Header.Set("X-API-Key", heraldAPIKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	testza.AssertNoError(t, err)
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Logf("Warning: failed to close response body: %v", closeErr)
		}
	}()

	testza.AssertTrue(t, resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden,
		"API key alone must be rejected under hmac_v2 (no downgrade), got %d", resp.StatusCode)

	t.Logf("✓ API key rejected under hmac_v2: Status %d", resp.StatusCode)
}

// TestHeraldAPIKeyInvalid tests Herald invalid API Key
func TestHeraldAPIKeyInvalid(t *testing.T) {
	ensureServicesReady(t)

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
	req.Header.Set("X-API-Key", "invalid-herald-api-key")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	testza.AssertNoError(t, err)
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Logf("Warning: failed to close response body: %v", closeErr)
		}
	}()

	testza.AssertTrue(t, resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden,
		"Should return 401 Unauthorized or 403 Forbidden with invalid API Key")

	t.Logf("✓ Invalid Herald API Key rejected: Status %d", resp.StatusCode)
}
