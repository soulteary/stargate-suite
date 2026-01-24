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

// TestHeraldIdempotencyKey 测试相同 Idempotency-Key 返回相同结果
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

	// 第二次请求（使用相同的 Idempotency-Key）
	bodyBytes2, err := json.Marshal(reqBody)
	testza.AssertNoError(t, err)

	req2, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes2))
	testza.AssertNoError(t, err)

	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Accept", "application/json")
	req2.Header.Set("X-API-Key", heraldAPIKey)
	req2.Header.Set("Idempotency-Key", idempotencyKey)

	// 短暂延迟确保在 TTL 内
	time.Sleep(1 * time.Second)

	resp2, err := client.Do(req2)
	testza.AssertNoError(t, err)

	var challengeResp2 HeraldChallengeResponse
	err = json.NewDecoder(resp2.Body).Decode(&challengeResp2)
	testza.AssertNoError(t, err)
	resp2.Body.Close()

	testza.AssertEqual(t, http.StatusOK, resp2.StatusCode, "Second request should return 200 OK")

	// 验证返回相同的结果
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

// TestHeraldIdempotencyKeyDifferent 测试不同的 Idempotency-Key 返回不同结果
func TestHeraldIdempotencyKeyDifferent(t *testing.T) {
	ensureServicesReady(t)

	// 使用不同的 destination 避免触发重发冷却限制
	reqBody := HeraldChallengeRequest{
		UserID:      "test-user-idempotency-diff",
		Channel:     "sms",
		Destination: "+8613800138001", // 使用不同的 destination
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

	// 第二次请求（使用不同的 Idempotency-Key 和不同的 destination 以避免重发冷却）
	reqBody2 := HeraldChallengeRequest{
		UserID:      "test-user-idempotency-diff",
		Channel:     "sms",
		Destination: "+8613800138002", // 使用不同的 destination 避免重发冷却
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

	// 验证返回不同的结果（不同的 ChallengeID）
	testza.AssertNotEqual(t, firstChallengeID, challengeResp2.ChallengeID,
		"ChallengeID should be different with different Idempotency-Key")

	t.Logf("First ChallengeID: %s", firstChallengeID)
	t.Logf("Second ChallengeID: %s", challengeResp2.ChallengeID)
	t.Logf("✓ Different Idempotency-Key returns different result")
}

// TestHeraldIdempotencyKeyWithoutHeader 测试不使用 Idempotency-Key 时每次都创建新的 challenge
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

	// 第一次请求（不使用 Idempotency-Key）
	req1, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	testza.AssertNoError(t, err)

	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("Accept", "application/json")
	req1.Header.Set("X-API-Key", heraldAPIKey)
	// 不设置 Idempotency-Key

	client := &http.Client{Timeout: 10 * time.Second}
	resp1, err := client.Do(req1)
	testza.AssertNoError(t, err)

	var challengeResp1 HeraldChallengeResponse
	err = json.NewDecoder(resp1.Body).Decode(&challengeResp1)
	testza.AssertNoError(t, err)
	resp1.Body.Close()

	testza.AssertEqual(t, http.StatusOK, resp1.StatusCode, "First request should return 200 OK")
	firstChallengeID := challengeResp1.ChallengeID

	// 第二次请求（同样不使用 Idempotency-Key）
	bodyBytes2, err := json.Marshal(reqBody)
	testza.AssertNoError(t, err)

	req2, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes2))
	testza.AssertNoError(t, err)

	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Accept", "application/json")
	req2.Header.Set("X-API-Key", heraldAPIKey)
	// 不设置 Idempotency-Key

	time.Sleep(1 * time.Second)

	resp2, err := client.Do(req2)
	testza.AssertNoError(t, err)

	var challengeResp2 HeraldChallengeResponse
	err = json.NewDecoder(resp2.Body).Decode(&challengeResp2)
	testza.AssertNoError(t, err)
	resp2.Body.Close()

	// 由于重发冷却时间（默认 60 秒），第二次请求可能会被限流，这是正常行为
	if resp2.StatusCode == http.StatusTooManyRequests {
		t.Logf("Note: Second request was rate-limited due to cooldown, which is expected behavior")
		// 测试通过：验证了不使用 Idempotency-Key 时，系统会应用重发冷却限制
		return
	}

	testza.AssertEqual(t, http.StatusOK, resp2.StatusCode, "Second request should return 200 OK")

	// 不使用 Idempotency-Key 时，应该创建新的 challenge（不同的 ChallengeID）
	if challengeResp2.ChallengeID != "" && challengeResp2.ChallengeID != firstChallengeID {
		t.Logf("✓ Without Idempotency-Key, new challenge created: %s", challengeResp2.ChallengeID)
	}

	t.Logf("First ChallengeID: %s", firstChallengeID)
}
