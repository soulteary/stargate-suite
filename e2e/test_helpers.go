package e2e

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// ErrorResponse 表示错误响应
type ErrorResponse struct {
	StatusCode int
	Message    string
	Body       string
}

// sendVerificationCodeWithError 发送验证码并返回错误响应（如果失败）
func sendVerificationCodeWithError(t *testing.T, phone string) (string, *ErrorResponse) {
	url := fmt.Sprintf("%s/_send_verify_code", stargateURL)
	body := fmt.Sprintf("phone=%s", phone)

	req, err := http.NewRequest("POST", url, strings.NewReader(body))
	if err != nil {
		return "", &ErrorResponse{StatusCode: 0, Message: err.Error()}
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Forwarded-Host", authHost)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", &ErrorResponse{StatusCode: 0, Message: err.Error()}
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Logf("Warning: failed to close response body: %v", closeErr)
		}
	}()

	bodyBytes, _ := io.ReadAll(resp.Body)
	bodyStr := string(bodyBytes)

	if resp.StatusCode != http.StatusOK {
		var errorMsg string
		var result struct {
			Success bool   `json:"success"`
			Message string `json:"message"`
			Error   string `json:"error"`
		}
		if err := json.Unmarshal(bodyBytes, &result); err == nil {
			if result.Message != "" {
				errorMsg = result.Message
			} else if result.Error != "" {
				errorMsg = result.Error
			} else {
				errorMsg = bodyStr
			}
		} else {
			errorMsg = bodyStr
		}
		return "", &ErrorResponse{
			StatusCode: resp.StatusCode,
			Message:    errorMsg,
			Body:       bodyStr,
		}
	}

	var result struct {
		Success     bool   `json:"success"`
		ChallengeID string `json:"challenge_id"`
		ExpiresIn   int    `json:"expires_in"`
	}

	if err := json.NewDecoder(strings.NewReader(bodyStr)).Decode(&result); err != nil {
		return "", &ErrorResponse{StatusCode: resp.StatusCode, Message: err.Error(), Body: bodyStr}
	}

	if !result.Success {
		return "", &ErrorResponse{StatusCode: resp.StatusCode, Message: "send verification code failed", Body: bodyStr}
	}

	return result.ChallengeID, nil
}

// loginWithError 登录并返回错误响应（如果失败）
func loginWithError(t *testing.T, phone, challengeID, verifyCode string) (string, *ErrorResponse) {
	url := fmt.Sprintf("%s/_login", stargateURL)
	body := fmt.Sprintf("auth_method=warden&phone=%s&challenge_id=%s&verify_code=%s",
		phone, challengeID, verifyCode)

	req, err := http.NewRequest("POST", url, strings.NewReader(body))
	if err != nil {
		return "", &ErrorResponse{StatusCode: 0, Message: err.Error()}
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Forwarded-Host", authHost)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", &ErrorResponse{StatusCode: 0, Message: err.Error()}
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Logf("Warning: failed to close response body: %v", closeErr)
		}
	}()

	bodyBytes, _ := io.ReadAll(resp.Body)
	bodyStr := string(bodyBytes)

	if resp.StatusCode != http.StatusOK {
		var errorMsg string
		var result struct {
			Success bool   `json:"success"`
			Message string `json:"message"`
			Error   string `json:"error"`
		}
		if err := json.Unmarshal(bodyBytes, &result); err == nil {
			if result.Message != "" {
				errorMsg = result.Message
			} else if result.Error != "" {
				errorMsg = result.Error
			} else {
				errorMsg = bodyStr
			}
		} else {
			errorMsg = bodyStr
		}
		return "", &ErrorResponse{
			StatusCode: resp.StatusCode,
			Message:    errorMsg,
			Body:       bodyStr,
		}
	}

	// 提取 Set-Cookie header
	setCookieHeaders := resp.Header.Values("Set-Cookie")
	if len(setCookieHeaders) == 0 {
		return "", &ErrorResponse{StatusCode: resp.StatusCode, Message: "no Set-Cookie header found"}
	}

	// 查找 session cookie (stargate_session_id)
	var sessionCookie string
	for _, cookieHeader := range setCookieHeaders {
		if strings.Contains(cookieHeader, "stargate_session_id") {
			parts := strings.Split(cookieHeader, ";")
			if len(parts) > 0 {
				sessionCookie = strings.TrimSpace(parts[0])
				break
			}
		}
	}

	if sessionCookie == "" {
		return "", &ErrorResponse{StatusCode: resp.StatusCode, Message: "session cookie not found"}
	}

	return sessionCookie, nil
}

// checkAuthWithError 验证授权并返回错误响应（如果失败）
func checkAuthWithError(t *testing.T, sessionCookie string) (*AuthHeaders, *ErrorResponse) {
	url := fmt.Sprintf("%s/_auth", stargateURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, &ErrorResponse{StatusCode: 0, Message: err.Error()}
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Host", authHost)
	if sessionCookie != "" {
		req.Header.Set("Cookie", sessionCookie)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, &ErrorResponse{StatusCode: 0, Message: err.Error()}
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Logf("Warning: failed to close response body: %v", closeErr)
		}
	}()

	bodyBytes, _ := io.ReadAll(resp.Body)
	bodyStr := string(bodyBytes)

	if resp.StatusCode != http.StatusOK {
		var errorMsg string
		var result struct {
			Message string `json:"message"`
			Error   string `json:"error"`
		}
		if err := json.Unmarshal(bodyBytes, &result); err == nil {
			if result.Message != "" {
				errorMsg = result.Message
			} else if result.Error != "" {
				errorMsg = result.Error
			} else {
				errorMsg = bodyStr
			}
		} else {
			errorMsg = bodyStr
		}
		return nil, &ErrorResponse{
			StatusCode: resp.StatusCode,
			Message:    errorMsg,
			Body:       bodyStr,
		}
	}

	headers := &AuthHeaders{
		UserID: resp.Header.Get("X-Auth-User"),
		Email:  resp.Header.Get("X-Auth-Email"),
		Scopes: resp.Header.Get("X-Auth-Scopes"),
		Role:   resp.Header.Get("X-Auth-Role"),
	}

	return headers, nil
}

// triggerRateLimit 触发限流（快速发送多次请求）
func triggerRateLimit(t *testing.T, phone string, count int) []*ErrorResponse {
	errors := make([]*ErrorResponse, 0, count)
	for i := 0; i < count; i++ {
		_, errResp := sendVerificationCodeWithError(t, phone)
		if errResp != nil {
			errors = append(errors, errResp)
		}
		// 短暂延迟避免过快
		time.Sleep(100 * time.Millisecond)
	}
	return errors
}

// stopDockerServiceInDir 在指定目录停止 Docker 服务
func stopDockerServiceInDir(dir, serviceName string) error {
	cmd := exec.Command("docker", "compose", "stop", serviceName)
	cmd.Dir = dir
	return cmd.Run()
}

// startDockerServiceInDir 在指定目录启动 Docker 服务
func startDockerServiceInDir(dir, serviceName string) error {
	cmd := exec.Command("docker", "compose", "start", serviceName)
	cmd.Dir = dir
	return cmd.Run()
}

// sendVerificationCodeWithEmail 使用邮箱发送验证码
func sendVerificationCodeWithEmail(t *testing.T, email string) (string, *ErrorResponse) {
	url := fmt.Sprintf("%s/_send_verify_code", stargateURL)
	body := fmt.Sprintf("mail=%s", email)

	req, err := http.NewRequest("POST", url, strings.NewReader(body))
	if err != nil {
		return "", &ErrorResponse{StatusCode: 0, Message: err.Error()}
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Forwarded-Host", authHost)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", &ErrorResponse{StatusCode: 0, Message: err.Error()}
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Logf("Warning: failed to close response body: %v", closeErr)
		}
	}()

	bodyBytes, _ := io.ReadAll(resp.Body)
	bodyStr := string(bodyBytes)

	if resp.StatusCode != http.StatusOK {
		var errorMsg string
		var result struct {
			Success bool   `json:"success"`
			Message string `json:"message"`
			Error   string `json:"error"`
		}
		if err := json.Unmarshal(bodyBytes, &result); err == nil {
			if result.Message != "" {
				errorMsg = result.Message
			} else if result.Error != "" {
				errorMsg = result.Error
			} else {
				errorMsg = bodyStr
			}
		} else {
			errorMsg = bodyStr
		}
		return "", &ErrorResponse{
			StatusCode: resp.StatusCode,
			Message:    errorMsg,
			Body:       bodyStr,
		}
	}

	var result struct {
		Success     bool   `json:"success"`
		ChallengeID string `json:"challenge_id"`
		ExpiresIn   int    `json:"expires_in"`
	}

	if err := json.NewDecoder(strings.NewReader(bodyStr)).Decode(&result); err != nil {
		return "", &ErrorResponse{StatusCode: resp.StatusCode, Message: err.Error(), Body: bodyStr}
	}

	if !result.Success {
		return "", &ErrorResponse{StatusCode: resp.StatusCode, Message: "send verification code failed", Body: bodyStr}
	}

	return result.ChallengeID, nil
}

// calculateHMAC 计算 HMAC-SHA256 签名
// 签名格式: HMAC-SHA256(timestamp:service:body, secret)
func calculateHMAC(timestamp int64, service, body, secret string) string {
	message := fmt.Sprintf("%d:%s:%s", timestamp, service, body)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
}

// clearRateLimitKeys 清理Redis中的测试状态，避免之前的测试影响当前测试
// 清理包括：限流键、冷却键、用户锁定键、challenge键
func clearRateLimitKeys(t *testing.T) error {
	// 连接Herald的Redis（根据docker-compose.yml，端口映射到localhost:6379）
	redisClient := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // 根据docker-compose.yml，没有密码
		DB:       0,  // Herald使用DB 0
	})
	defer func() {
		if err := redisClient.Close(); err != nil {
			t.Logf("Warning: failed to close redis client: %v", err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 测试连接
	if err := redisClient.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("failed to connect to Redis: %w", err)
	}

	// 清理所有测试相关的键：
	// - ratelimit:* - 限流键
	// - ratelimit:cooldown:* - 冷却键
	// - otp:lock:* - 用户锁定键
	// - otp:ch:* - challenge键（为了测试隔离）
	patterns := []string{
		"ratelimit:*",
		"ratelimit:cooldown:*",
		"otp:lock:*",
		"otp:ch:*",
	}

	totalCleared := 0
	for _, pattern := range patterns {
		var keys []string
		var cursor uint64

		// 使用 SCAN 迭代所有匹配的键
		for {
			var scanKeys []string
			var err error
			scanKeys, cursor, err = redisClient.Scan(ctx, cursor, pattern, 100).Result()
			if err != nil {
				return fmt.Errorf("failed to scan keys with pattern %s: %w", pattern, err)
			}
			keys = append(keys, scanKeys...)
			if cursor == 0 {
				break
			}
		}

		if len(keys) > 0 {
			if err := redisClient.Del(ctx, keys...).Err(); err != nil {
				return fmt.Errorf("failed to delete keys with pattern %s: %w", pattern, err)
			}
			totalCleared += len(keys)
			t.Logf("Cleared %d keys matching pattern %s", len(keys), pattern)
		}
	}

	if totalCleared > 0 {
		t.Logf("Total cleared %d test state keys", totalCleared)
	} else {
		t.Logf("No test state keys found to clear")
	}

	return nil
}
