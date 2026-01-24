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

// TestHeraldProviderFailure 测试 Provider 发送失败处理
// 注意：在测试环境中，Provider 通常正常工作，这里主要验证失败处理逻辑
// 根据 docker-compose.yml，PROVIDER_FAILURE_POLICY=soft，意味着即使发送失败，challenge 也会被创建
func TestHeraldProviderFailure(t *testing.T) {
	ensureServicesReady(t)

	// 在 soft 模式下，即使 Provider 失败，challenge 也应该被创建
	// 这里我们创建一个正常的 challenge，验证逻辑
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

	// 在 soft 模式下，即使 Provider 失败，也应该返回 200 OK（challenge 已创建）
	// 如果 Provider 成功，也返回 200 OK
	testza.AssertEqual(t, http.StatusOK, resp.StatusCode, "Should return 200 OK in soft mode even if provider fails")

	var challengeResp HeraldChallengeResponse
	err = json.NewDecoder(resp.Body).Decode(&challengeResp)
	testza.AssertNoError(t, err)

	testza.AssertNotNil(t, challengeResp.ChallengeID)
	testza.AssertTrue(t, len(challengeResp.ChallengeID) > 0, "ChallengeID should not be empty")

	t.Logf("✓ Challenge created successfully: %s", challengeResp.ChallengeID)
	t.Log("Note: In soft mode, challenge is created even if provider fails. Check audit logs for send_failed events.")

	// 验证 challenge 仍然可以验证（如果 Provider 失败，可以通过测试端点获取验证码）
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

// TestHeraldProviderRetry 测试 Provider 重试策略
// 注意：Herald 可能没有显式的重试逻辑，这里主要验证幂等性在重试场景中的作用
func TestHeraldProviderRetry(t *testing.T) {
	ensureServicesReady(t)

	// 使用相同的 Idempotency-Key 进行"重试"，验证幂等性
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

	// 第一次请求（模拟初始发送）
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

	// 第二次请求（模拟重试，使用相同的 Idempotency-Key）
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

// TestHeraldProviderErrorCodes 测试 Provider 错误码归一化
// 注意：这里主要验证错误响应格式，实际的 Provider 错误码归一化在代码内部处理
func TestHeraldProviderErrorCodes(t *testing.T) {
	ensureServicesReady(t)

	// 测试无效的 channel 应该返回适当的错误码
	t.Log("Testing provider error code normalization...")

	reqBody := HeraldChallengeRequest{
		UserID:      "test-user-error-codes",
		Channel:     "invalid_channel", // 无效的 channel
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

// TestHeraldProviderEmail 测试 Email Provider
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

// TestHeraldProviderSMS 测试 SMS Provider
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
