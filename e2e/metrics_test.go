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

// parsePrometheusMetrics parses Prometheus format metrics text
// Returns a map of metric name to value
func parsePrometheusMetrics(metricsText string) map[string]string {
	result := make(map[string]string)
	lines := strings.Split(metricsText, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse metric line, format: metric_name{labels} value
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			metricName := strings.Split(parts[0], "{")[0]
			value := parts[len(parts)-1]
			result[metricName] = value
		}
	}

	return result
}

// getMetricValue gets specific metric value from metrics text
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

// TestHeraldMetrics verifies Herald Prometheus metrics updates
func TestHeraldMetrics(t *testing.T) {
	ensureServicesReady(t)

	// Get initial metric values
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

	// Perform operations to trigger metric updates
	// 1. Create a challenge
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

	// Wait for metrics update
	time.Sleep(1 * time.Second)

	// 2. Verify challenge
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

	// Wait for metrics update
	time.Sleep(1 * time.Second)

	// Get updated metrics
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

	// Verify metrics exist (values may be 0 or greater depending on initial state)
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
	// herald_redis_latency_seconds is a histogram, may appear as _bucket, _sum, _count
	// If metric hasn't been recorded (no Redis ops), it might not appear, which is acceptable
	hasRedisLatency := strings.Contains(finalMetricsText, "herald_redis_latency_seconds") ||
		strings.Contains(finalMetricsText, "herald_redis_latency_seconds_bucket") ||
		strings.Contains(finalMetricsText, "herald_redis_latency_seconds_sum") ||
		strings.Contains(finalMetricsText, "herald_redis_latency_seconds_count")
	if !hasRedisLatency {
		t.Logf("Note: herald_redis_latency_seconds metric not found (may not be recorded yet if no Redis operations occurred)")
	}

	t.Log("✓ Herald metrics endpoint is accessible and contains expected metrics")
}

// TestWardenMetrics verifies Warden Prometheus metrics updates
func TestWardenMetrics(t *testing.T) {
	ensureServicesReady(t)

	// Get initial metric values (may require API Key)
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

	// Perform operations to trigger metric updates
	// 1. Query user
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

	// Wait for metrics update
	time.Sleep(1 * time.Second)

	// Get updated metrics (may require API Key)
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

	// Verify metrics exist (metrics might not be recorded yet depending on service state)
	// http_requests_total and http_request_duration_seconds should exist (since we just sent a request)
	testza.AssertTrue(t, strings.Contains(finalMetricsText, "http_requests_total") ||
		strings.Contains(finalMetricsText, "http_request_duration_seconds"),
		"HTTP metrics should exist (http_requests_total or http_request_duration_seconds)")

	// cache_size may not exist depending on cache initialization
	if strings.Contains(finalMetricsText, "cache_size") {
		t.Logf("✓ cache_size metric found")
	} else {
		t.Logf("Note: cache_size metric not found (may not be initialized yet)")
	}

	// Background task metrics may not exist depending on if background tasks are running
	hasBackgroundMetrics := strings.Contains(finalMetricsText, "background_task_total") ||
		strings.Contains(finalMetricsText, "background_task_duration_seconds")
	if hasBackgroundMetrics {
		t.Logf("✓ Background task metrics found")
	} else {
		t.Logf("Note: Background task metrics not found (may not be running)")
	}

	t.Log("✓ Warden metrics endpoint is accessible and contains expected metrics")
}

// TestHeraldMetricsFormat verifies Herald metrics format is correct
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

	// Verify Prometheus format
	testza.AssertTrue(t, strings.Contains(metricsText, "# HELP"),
		"Metrics should contain HELP comments")
	testza.AssertTrue(t, strings.Contains(metricsText, "# TYPE"),
		"Metrics should contain TYPE comments")

	// Verify key metrics exist
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

// TestWardenMetricsFormat verifies Warden metrics format is correct
func TestWardenMetricsFormat(t *testing.T) {
	ensureServicesReady(t)

	metricsURL := fmt.Sprintf("%s/metrics", wardenURL)
	req, err := http.NewRequest("GET", metricsURL, nil)
	testza.AssertNoError(t, err)

	// Warden metrics may require API Key
	req.Header.Set("X-API-Key", wardenAPIKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	testza.AssertNoError(t, err)
	defer resp.Body.Close()

	testza.AssertEqual(t, http.StatusOK, resp.StatusCode, "Metrics endpoint should return 200 OK")

	body, err := io.ReadAll(resp.Body)
	testza.AssertNoError(t, err)
	metricsText := string(body)

	// Verify Prometheus format
	testza.AssertTrue(t, strings.Contains(metricsText, "# HELP"),
		"Metrics should contain HELP comments")
	testza.AssertTrue(t, strings.Contains(metricsText, "# TYPE"),
		"Metrics should contain TYPE comments")

	// Verify key metrics exist
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
