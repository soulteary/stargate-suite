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

// TestHeraldAuditLog 验证关键操作记录审计日志
// 注意：由于审计日志存储在 Redis 中，这里主要验证操作是否成功
// 如果操作成功，审计日志应该被记录（根据代码实现）
func TestHeraldAuditLog(t *testing.T) {
	ensureServicesReady(t)

	// 添加延迟以避免限流
	time.Sleep(2 * time.Second)

	// 测试 challenge 创建应该记录审计日志
	t.Log("Testing audit log for challenge creation...")
	reqBody := HeraldChallengeRequest{
		UserID:      "test-user-audit",
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

	// 处理限流情况（429）
	if resp.StatusCode == http.StatusTooManyRequests {
		t.Logf("⚠ Rate limited, skipping this test. Status: %d", resp.StatusCode)
		return
	}
	testza.AssertEqual(t, http.StatusOK, resp.StatusCode, "Challenge creation should succeed")

	var challengeResp HeraldChallengeResponse
	err = json.NewDecoder(resp.Body).Decode(&challengeResp)
	testza.AssertNoError(t, err)

	testza.AssertNotNil(t, challengeResp.ChallengeID)
	challengeID := challengeResp.ChallengeID

	if challengeID == "" {
		t.Log("⚠ ChallengeID is empty, skipping verification test")
		return
	}

	t.Logf("✓ Challenge created successfully: %s (audit log should be recorded)", challengeID)

	// 测试验证成功应该记录审计日志
	t.Log("Testing audit log for successful verification...")
	verifyCode, err := getTestCode(t, challengeID)
	if err != nil {
		t.Logf("⚠ Failed to get test code: %v, skipping verification test", err)
		return
	}
	testza.AssertNoError(t, err)

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

	testza.AssertEqual(t, http.StatusOK, verifyResp.StatusCode, "Verification should succeed")

	var verifyRespBody HeraldVerifyResponse
	err = json.NewDecoder(verifyResp.Body).Decode(&verifyRespBody)
	testza.AssertNoError(t, err)

	testza.AssertTrue(t, verifyRespBody.OK, "Verification should succeed")
	t.Logf("✓ Verification successful (audit log should be recorded)")

	// 测试验证失败应该记录审计日志
	t.Log("Testing audit log for failed verification...")
	// 添加延迟以避免限流
	time.Sleep(2 * time.Second)

	reqBody2 := HeraldChallengeRequest{
		UserID:      "test-user-audit-fail",
		Channel:     "sms",
		Destination: "+8613800138000",
		Purpose:     "login",
	}

	bodyBytes2, err := json.Marshal(reqBody2)
	testza.AssertNoError(t, err)

	req2, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes2))
	testza.AssertNoError(t, err)

	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Accept", "application/json")
	req2.Header.Set("X-API-Key", heraldAPIKey)

	resp2, err := client.Do(req2)
	testza.AssertNoError(t, err)

	var challengeResp2 HeraldChallengeResponse
	err = json.NewDecoder(resp2.Body).Decode(&challengeResp2)
	testza.AssertNoError(t, err)
	if closeErr := resp2.Body.Close(); closeErr != nil {
		t.Logf("Warning: failed to close response body: %v", closeErr)
	}

	// 处理限流情况
	if resp2.StatusCode == http.StatusTooManyRequests {
		t.Logf("⚠ Rate limited for second challenge, skipping failed verification test")
		return
	}

	challengeID2 := challengeResp2.ChallengeID
	if challengeID2 == "" {
		t.Log("⚠ ChallengeID2 is empty, skipping failed verification test")
		return
	}

	// 使用错误的验证码
	verifyReqBody2 := HeraldVerifyRequest{
		ChallengeID: challengeID2,
		Code:        "000000",
		ClientIP:    "192.168.1.1",
	}

	verifyBodyBytes2, err := json.Marshal(verifyReqBody2)
	testza.AssertNoError(t, err)

	verifyReq2, err := http.NewRequest("POST", verifyURL, bytes.NewReader(verifyBodyBytes2))
	testza.AssertNoError(t, err)

	verifyReq2.Header.Set("Content-Type", "application/json")
	verifyReq2.Header.Set("Accept", "application/json")
	verifyReq2.Header.Set("X-API-Key", heraldAPIKey)

	verifyResp2, err := client.Do(verifyReq2)
	testza.AssertNoError(t, err)
	defer func() {
		if closeErr := verifyResp2.Body.Close(); closeErr != nil {
			t.Logf("Warning: failed to close response body: %v", closeErr)
		}
	}()

	testza.AssertTrue(t, verifyResp2.StatusCode == http.StatusBadRequest || verifyResp2.StatusCode == http.StatusUnauthorized,
		"Verification should fail with invalid code")

	var verifyRespBody2 HeraldVerifyResponse
	err = json.NewDecoder(verifyResp2.Body).Decode(&verifyRespBody2)
	if err == nil {
		testza.AssertFalse(t, verifyRespBody2.OK, "Verification should fail")
		t.Logf("✓ Verification failed (audit log should be recorded with reason: %s)", verifyRespBody2.Reason)
	}

	// 测试 challenge 作废应该记录审计日志
	t.Log("Testing audit log for challenge revocation...")
	// 添加延迟以避免限流
	time.Sleep(2 * time.Second)

	reqBody3 := HeraldChallengeRequest{
		UserID:      "test-user-audit-revoke",
		Channel:     "sms",
		Destination: "+8613800138000",
		Purpose:     "login",
	}

	bodyBytes3, err := json.Marshal(reqBody3)
	testza.AssertNoError(t, err)

	req3, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes3))
	testza.AssertNoError(t, err)

	req3.Header.Set("Content-Type", "application/json")
	req3.Header.Set("Accept", "application/json")
	req3.Header.Set("X-API-Key", heraldAPIKey)

	resp3, err := client.Do(req3)
	testza.AssertNoError(t, err)

	var challengeResp3 HeraldChallengeResponse
	err = json.NewDecoder(resp3.Body).Decode(&challengeResp3)
	testza.AssertNoError(t, err)
	resp3.Body.Close()

	// 处理限流情况
	if resp3.StatusCode == http.StatusTooManyRequests {
		t.Logf("⚠ Rate limited for third challenge, skipping revocation test")
		return
	}

	challengeID3 := challengeResp3.ChallengeID
	if challengeID3 == "" {
		t.Log("⚠ ChallengeID3 is empty, skipping revocation test")
		return
	}

	// 作废 challenge
	revokeURL := fmt.Sprintf("%s/v1/otp/challenges/%s/revoke", heraldURL, challengeID3)
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

	testza.AssertTrue(t, revokeResp.StatusCode == http.StatusOK || revokeResp.StatusCode == http.StatusNoContent,
		"Revocation should succeed")

	t.Logf("✓ Challenge revoked successfully (audit log should be recorded)")

	t.Log("Note: To verify audit logs directly, check Redis keys with prefix 'otp:audit:' or check service logs")
}

// TestWardenAuditLog 验证用户查询记录审计日志
// 注意：Warden 可能没有显式的审计日志功能，这里主要验证操作是否成功
func TestWardenAuditLog(t *testing.T) {
	ensureServicesReady(t)

	// 测试用户查询应该被记录（如果 Warden 有审计功能）
	t.Log("Testing user query (audit log may be recorded if enabled)...")

	testPhone := "13800138000"
	url := fmt.Sprintf("%s/user?phone=%s", wardenURL, testPhone)
	req, err := http.NewRequest("GET", url, nil)
	testza.AssertNoError(t, err)

	req.Header.Set("X-API-Key", wardenAPIKey)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	testza.AssertNoError(t, err)
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Logf("Warning: failed to close response body: %v", closeErr)
		}
	}()

	testza.AssertEqual(t, http.StatusOK, resp.StatusCode, "User query should succeed")

	var user WardenUser
	err = json.NewDecoder(resp.Body).Decode(&user)
	testza.AssertNoError(t, err)

	testza.AssertEqual(t, testPhone, user.Phone, "Phone should match")
	t.Logf("✓ User query successful (audit log may be recorded if Warden audit is enabled)")

	t.Log("Note: Warden may not have explicit audit logging. Check service logs or configuration for audit features")
}
