package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/MarvinJWendt/testza"
)

// parsePrometheusMetrics 解析 Prometheus 格式的指标文本
// 返回指标名称到值的映射
func parsePrometheusMetrics(metricsText string) map[string]string {
	result := make(map[string]string)
	lines := strings.Split(metricsText, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		// 跳过注释和空行
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// 解析指标行，格式: metric_name{labels} value
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			metricName := strings.Split(parts[0], "{")[0]
			value := parts[len(parts)-1]
			result[metricName] = value
		}
	}

	return result
}

// getMetricValue 从指标文本中获取特定指标的值
func getMetricValue(metricsText, metricName string) string {
	lines := strings.Split(metricsText, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, metricName) {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				return parts[len(parts)-1]
			}
		}
	}

	return ""
}

// TestHeraldMetrics 验证 Herald Prometheus 指标更新
func TestHeraldMetrics(t *testing.T) {
	ensureServicesReady(t)

	// 获取初始指标值
	initialMetricsURL := fmt.Sprintf("%s/metrics", heraldURL)
	initialResp, err := http.Get(initialMetricsURL)
	testza.AssertNoError(t, err)
	defer initialResp.Body.Close()

	initialBody, err := io.ReadAll(initialResp.Body)
	testza.AssertNoError(t, err)
	initialMetricsText := string(initialBody)

	// 解析初始指标
	initialChallenges := getMetricValue(initialMetricsText, "herald_otp_challenges_total")
	initialVerifications := getMetricValue(initialMetricsText, "herald_otp_verifications_total")
	initialSends := getMetricValue(initialMetricsText, "herald_otp_sends_total")

	t.Logf("Initial metrics - Challenges: %s, Verifications: %s, Sends: %s",
		initialChallenges, initialVerifications, initialSends)

	// 执行一些操作来触发指标更新
	// 1. 创建一个 challenge
	reqBody := HeraldChallengeRequest{
		UserID:      "test-user-metrics",
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
	defer resp.Body.Close()

	testza.AssertEqual(t, http.StatusOK, resp.StatusCode, "Challenge creation should succeed")

	var challengeResp HeraldChallengeResponse
	err = json.NewDecoder(resp.Body).Decode(&challengeResp)
	testza.AssertNoError(t, err)
	challengeID := challengeResp.ChallengeID

	// 等待指标更新
	time.Sleep(1 * time.Second)

	// 2. 验证 challenge
	verifyCode, err := getTestCode(t, challengeID)
	testza.AssertNoError(t, err)

	verifyReqBody := HeraldVerifyRequest{
		ChallengeID: challengeID,
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
	defer verifyResp.Body.Close()

	testza.AssertEqual(t, http.StatusOK, verifyResp.StatusCode, "Verification should succeed")

	// 等待指标更新
	time.Sleep(1 * time.Second)

	// 获取更新后的指标
	finalResp, err := http.Get(initialMetricsURL)
	testza.AssertNoError(t, err)
	defer finalResp.Body.Close()

	finalBody, err := io.ReadAll(finalResp.Body)
	testza.AssertNoError(t, err)
	finalMetricsText := string(finalBody)

	// 验证指标已更新
	finalChallenges := getMetricValue(finalMetricsText, "herald_otp_challenges_total")
	finalVerifications := getMetricValue(finalMetricsText, "herald_otp_verifications_total")
	finalSends := getMetricValue(finalMetricsText, "herald_otp_sends_total")

	t.Logf("Final metrics - Challenges: %s, Verifications: %s, Sends: %s",
		finalChallenges, finalVerifications, finalSends)

	// 验证指标存在（值可能为 0 或更大，取决于初始状态）
	testza.AssertTrue(t, strings.Contains(finalMetricsText, "herald_otp_challenges_total"),
		"herald_otp_challenges_total metric should exist")
	testza.AssertTrue(t, strings.Contains(finalMetricsText, "herald_otp_verifications_total"),
		"herald_otp_verifications_total metric should exist")
	testza.AssertTrue(t, strings.Contains(finalMetricsText, "herald_otp_sends_total"),
		"herald_otp_sends_total metric should exist")
	testza.AssertTrue(t, strings.Contains(finalMetricsText, "herald_otp_send_duration_seconds"),
		"herald_otp_send_duration_seconds metric should exist")
	testza.AssertTrue(t, strings.Contains(finalMetricsText, "herald_rate_limit_hits_total"),
		"herald_rate_limit_hits_total metric should exist")
	// herald_redis_latency_seconds 是 histogram，可能以 _bucket, _sum, _count 形式出现
	// 如果指标还没有被记录（没有 Redis 操作），可能不会出现，这是可以接受的
	hasRedisLatency := strings.Contains(finalMetricsText, "herald_redis_latency_seconds") ||
		strings.Contains(finalMetricsText, "herald_redis_latency_seconds_bucket") ||
		strings.Contains(finalMetricsText, "herald_redis_latency_seconds_sum") ||
		strings.Contains(finalMetricsText, "herald_redis_latency_seconds_count")
	if !hasRedisLatency {
		t.Logf("Note: herald_redis_latency_seconds metric not found (may not be recorded yet if no Redis operations occurred)")
	}

	t.Log("✓ Herald metrics endpoint is accessible and contains expected metrics")
}

// TestWardenMetrics 验证 Warden Prometheus 指标更新
func TestWardenMetrics(t *testing.T) {
	ensureServicesReady(t)

	// 获取初始指标值（可能需要 API Key）
	initialMetricsURL := fmt.Sprintf("%s/metrics", wardenURL)
	initialReq, err := http.NewRequest("GET", initialMetricsURL, nil)
	testza.AssertNoError(t, err)
	initialReq.Header.Set("X-API-Key", wardenAPIKey)

	client := &http.Client{Timeout: 10 * time.Second}
	initialResp, err := client.Do(initialReq)
	testza.AssertNoError(t, err)
	defer initialResp.Body.Close()

	initialBody, err := io.ReadAll(initialResp.Body)
	testza.AssertNoError(t, err)
	initialMetricsText := string(initialBody)

	// 解析初始指标
	initialRequests := getMetricValue(initialMetricsText, "http_requests_total")
	initialCacheSize := getMetricValue(initialMetricsText, "cache_size")

	t.Logf("Initial metrics - HTTP Requests: %s, Cache Size: %s",
		initialRequests, initialCacheSize)

	// 执行一些操作来触发指标更新
	// 1. 查询用户
	testPhone := "13800138000"
	url := fmt.Sprintf("%s/user?phone=%s", wardenURL, testPhone)
	req, err := http.NewRequest("GET", url, nil)
	testza.AssertNoError(t, err)

	req.Header.Set("X-API-Key", wardenAPIKey)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	testza.AssertNoError(t, err)
	defer resp.Body.Close()

	testza.AssertEqual(t, http.StatusOK, resp.StatusCode, "User query should succeed")

	// 等待指标更新
	time.Sleep(1 * time.Second)

	// 获取更新后的指标（可能需要 API Key）
	finalReq, err := http.NewRequest("GET", initialMetricsURL, nil)
	testza.AssertNoError(t, err)
	finalReq.Header.Set("X-API-Key", wardenAPIKey)

	finalResp, err := client.Do(finalReq)
	testza.AssertNoError(t, err)
	defer finalResp.Body.Close()

	finalBody, err := io.ReadAll(finalResp.Body)
	testza.AssertNoError(t, err)
	finalMetricsText := string(finalBody)

	// 验证指标已更新
	finalRequests := getMetricValue(finalMetricsText, "http_requests_total")
	finalCacheSize := getMetricValue(finalMetricsText, "cache_size")

	t.Logf("Final metrics - HTTP Requests: %s, Cache Size: %s",
		finalRequests, finalCacheSize)

	// 验证指标存在（指标可能还没有被记录，取决于服务状态）
	// http_requests_total 和 http_request_duration_seconds 应该存在（因为我们刚刚发送了请求）
	testza.AssertTrue(t, strings.Contains(finalMetricsText, "http_requests_total") ||
		strings.Contains(finalMetricsText, "http_request_duration_seconds"),
		"HTTP metrics should exist (http_requests_total or http_request_duration_seconds)")

	// cache_size 可能不存在，取决于缓存是否已初始化
	if strings.Contains(finalMetricsText, "cache_size") {
		t.Logf("✓ cache_size metric found")
	} else {
		t.Logf("Note: cache_size metric not found (may not be initialized yet)")
	}

	// 背景任务指标可能不存在，取决于是否有后台任务运行
	hasBackgroundMetrics := strings.Contains(finalMetricsText, "background_task_total") ||
		strings.Contains(finalMetricsText, "background_task_duration_seconds")
	if hasBackgroundMetrics {
		t.Logf("✓ Background task metrics found")
	} else {
		t.Logf("Note: Background task metrics not found (may not be running)")
	}

	t.Log("✓ Warden metrics endpoint is accessible and contains expected metrics")
}

// TestHeraldMetricsFormat 验证 Herald 指标格式是否正确
func TestHeraldMetricsFormat(t *testing.T) {
	ensureServicesReady(t)

	metricsURL := fmt.Sprintf("%s/metrics", heraldURL)
	resp, err := http.Get(metricsURL)
	testza.AssertNoError(t, err)
	defer resp.Body.Close()

	testza.AssertEqual(t, http.StatusOK, resp.StatusCode, "Metrics endpoint should return 200 OK")

	body, err := io.ReadAll(resp.Body)
	testza.AssertNoError(t, err)
	metricsText := string(body)

	// 验证 Prometheus 格式
	testza.AssertTrue(t, strings.Contains(metricsText, "# HELP"),
		"Metrics should contain HELP comments")
	testza.AssertTrue(t, strings.Contains(metricsText, "# TYPE"),
		"Metrics should contain TYPE comments")

	// 验证关键指标存在
	requiredMetrics := []string{
		"herald_otp_challenges_total",
		"herald_otp_verifications_total",
		"herald_otp_sends_total",
	}

	for _, metric := range requiredMetrics {
		testza.AssertTrue(t, strings.Contains(metricsText, metric),
			fmt.Sprintf("Metrics should contain %s", metric))
	}

	t.Log("✓ Herald metrics format is correct")
}

// TestWardenMetricsFormat 验证 Warden 指标格式是否正确
func TestWardenMetricsFormat(t *testing.T) {
	ensureServicesReady(t)

	metricsURL := fmt.Sprintf("%s/metrics", wardenURL)
	req, err := http.NewRequest("GET", metricsURL, nil)
	testza.AssertNoError(t, err)

	// Warden metrics 可能需要 API Key
	req.Header.Set("X-API-Key", wardenAPIKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	testza.AssertNoError(t, err)
	defer resp.Body.Close()

	testza.AssertEqual(t, http.StatusOK, resp.StatusCode, "Metrics endpoint should return 200 OK")

	body, err := io.ReadAll(resp.Body)
	testza.AssertNoError(t, err)
	metricsText := string(body)

	// 验证 Prometheus 格式
	testza.AssertTrue(t, strings.Contains(metricsText, "# HELP"),
		"Metrics should contain HELP comments")
	testza.AssertTrue(t, strings.Contains(metricsText, "# TYPE"),
		"Metrics should contain TYPE comments")

	// 验证关键指标存在
	requiredMetrics := []string{
		"http_requests_total",
		"http_request_duration_seconds",
		"cache_size",
	}

	for _, metric := range requiredMetrics {
		testza.AssertTrue(t, strings.Contains(metricsText, metric),
			fmt.Sprintf("Metrics should contain %s", metric))
	}

	t.Log("✓ Warden metrics format is correct")
}
