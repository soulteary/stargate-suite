package e2e

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/MarvinJWendt/testza"
)

const (
	wardenAPIKey = "test-warden-api-key"
)

// WardenUser 表示 Warden 返回的用户信息
type WardenUser struct {
	Phone  string   `json:"phone"`
	Mail   string   `json:"mail"`
	UserID string   `json:"user_id"`
	Status string   `json:"status"`
	Scope  []string `json:"scope"`
	Role   string   `json:"role"`
}

// TestWardenGetUserByPhone 测试通过手机号查询用户
func TestWardenGetUserByPhone(t *testing.T) {
	ensureServicesReady(t)

	testPhone := "13800138000"
	expectedUserID := "test-admin-001"
	expectedEmail := "admin@example.com"
	expectedStatus := "active"
	expectedScopes := []string{"read", "write", "admin"}
	expectedRole := "admin"

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

	testza.AssertEqual(t, http.StatusOK, resp.StatusCode, "Should return 200 OK")

	var user WardenUser
	err = json.NewDecoder(resp.Body).Decode(&user)
	testza.AssertNoError(t, err)

	testza.AssertEqual(t, testPhone, user.Phone, "Phone should match")
	testza.AssertEqual(t, expectedEmail, user.Mail, "Email should match")
	testza.AssertEqual(t, expectedUserID, user.UserID, "UserID should match")
	testza.AssertEqual(t, expectedStatus, user.Status, "Status should be active")
	testza.AssertEqual(t, expectedRole, user.Role, "Role should match")
	testza.AssertEqual(t, expectedScopes, user.Scope, "Scopes should match")

	t.Logf("✓ User found: %+v", user)
}

// TestWardenGetUserByEmail 测试通过邮箱查询用户
func TestWardenGetUserByEmail(t *testing.T) {
	ensureServicesReady(t)

	testEmail := "user@example.com"
	expectedPhone := "13900139000"
	expectedUserID := "test-user-002"
	expectedStatus := "active"
	expectedScopes := []string{"read"}
	expectedRole := "user"

	url := fmt.Sprintf("%s/user?mail=%s", wardenURL, testEmail)
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

	testza.AssertEqual(t, http.StatusOK, resp.StatusCode, "Should return 200 OK")

	var user WardenUser
	err = json.NewDecoder(resp.Body).Decode(&user)
	testza.AssertNoError(t, err)

	testza.AssertEqual(t, expectedPhone, user.Phone, "Phone should match")
	testza.AssertEqual(t, testEmail, user.Mail, "Email should match")
	testza.AssertEqual(t, expectedUserID, user.UserID, "UserID should match")
	testza.AssertEqual(t, expectedStatus, user.Status, "Status should be active")
	testza.AssertEqual(t, expectedRole, user.Role, "Role should match")
	testza.AssertEqual(t, expectedScopes, user.Scope, "Scopes should match")

	t.Logf("✓ User found: %+v", user)
}

// TestWardenGetUserByUserID 测试通过用户ID查询用户
func TestWardenGetUserByUserID(t *testing.T) {
	ensureServicesReady(t)

	testUserID := "test-guest-003"
	expectedPhone := "13700137000"
	expectedEmail := "guest@example.com"
	expectedStatus := "active"
	expectedScopes := []string{"read"}
	expectedRole := "guest"

	url := fmt.Sprintf("%s/user?user_id=%s", wardenURL, testUserID)
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

	testza.AssertEqual(t, http.StatusOK, resp.StatusCode, "Should return 200 OK")

	var user WardenUser
	err = json.NewDecoder(resp.Body).Decode(&user)
	testza.AssertNoError(t, err)

	testza.AssertEqual(t, expectedPhone, user.Phone, "Phone should match")
	testza.AssertEqual(t, expectedEmail, user.Mail, "Email should match")
	testza.AssertEqual(t, testUserID, user.UserID, "UserID should match")
	testza.AssertEqual(t, expectedStatus, user.Status, "Status should be active")
	testza.AssertEqual(t, expectedRole, user.Role, "Role should match")
	testza.AssertEqual(t, expectedScopes, user.Scope, "Scopes should match")

	t.Logf("✓ User found: %+v", user)
}

// TestWardenUserNotFound 测试用户不存在
func TestWardenUserNotFound(t *testing.T) {
	ensureServicesReady(t)

	nonExistentPhone := "13000000000"

	url := fmt.Sprintf("%s/user?phone=%s", wardenURL, nonExistentPhone)
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

	testza.AssertEqual(t, http.StatusNotFound, resp.StatusCode, "Should return 404 Not Found")

	bodyBytes, _ := io.ReadAll(resp.Body)
	bodyStr := string(bodyBytes)
	testza.AssertTrue(t, strings.Contains(bodyStr, "not found") || strings.Contains(bodyStr, "Not Found"),
		"Error message should mention user not found")

	t.Logf("✓ User not found correctly: %s", bodyStr)
}

// TestWardenInvalidParameters 测试参数错误
func TestWardenInvalidParameters(t *testing.T) {
	ensureServicesReady(t)

	// 测试缺少参数
	url := fmt.Sprintf("%s/user", wardenURL)
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

	testza.AssertEqual(t, http.StatusBadRequest, resp.StatusCode, "Should return 400 Bad Request")

	bodyBytes, _ := io.ReadAll(resp.Body)
	bodyStr := string(bodyBytes)
	testza.AssertTrue(t, strings.Contains(bodyStr, "missing") || strings.Contains(bodyStr, "Bad Request"),
		"Error message should mention missing parameter")

	t.Logf("✓ Missing parameter rejected: %s", bodyStr)

	// 测试多个参数
	url2 := fmt.Sprintf("%s/user?phone=13800138000&mail=admin@example.com", wardenURL)
	req2, err := http.NewRequest("GET", url2, nil)
	testza.AssertNoError(t, err)

	req2.Header.Set("X-API-Key", wardenAPIKey)
	req2.Header.Set("Accept", "application/json")

	resp2, err := client.Do(req2)
	testza.AssertNoError(t, err)
	defer func() {
		if closeErr := resp2.Body.Close(); closeErr != nil {
			t.Logf("Warning: failed to close response body: %v", closeErr)
		}
	}()

	testza.AssertEqual(t, http.StatusBadRequest, resp2.StatusCode, "Should return 400 Bad Request")

	bodyBytes2, _ := io.ReadAll(resp2.Body)
	bodyStr2 := string(bodyBytes2)
	testza.AssertTrue(t, strings.Contains(bodyStr2, "only one") || strings.Contains(bodyStr2, "Bad Request"),
		"Error message should mention only one parameter allowed")

	t.Logf("✓ Multiple parameters rejected: %s", bodyStr2)
}

// TestWardenAPIKeyAuth 测试 API Key 认证
func TestWardenAPIKeyAuth(t *testing.T) {
	ensureServicesReady(t)

	testPhone := "13800138000"

	// 测试缺少 API Key
	url := fmt.Sprintf("%s/user?phone=%s", wardenURL, testPhone)
	req, err := http.NewRequest("GET", url, nil)
	testza.AssertNoError(t, err)

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
		"Should return 401 Unauthorized or 403 Forbidden")

	t.Logf("✓ Missing API Key rejected: Status %d", resp.StatusCode)

	// 测试无效 API Key
	url2 := fmt.Sprintf("%s/user?phone=%s", wardenURL, testPhone)
	req2, err := http.NewRequest("GET", url2, nil)
	testza.AssertNoError(t, err)

	req2.Header.Set("X-API-Key", "invalid-api-key")
	req2.Header.Set("Accept", "application/json")

	resp2, err := client.Do(req2)
	testza.AssertNoError(t, err)
	defer func() {
		if closeErr := resp2.Body.Close(); closeErr != nil {
			t.Logf("Warning: failed to close response body: %v", closeErr)
		}
	}()

	testza.AssertTrue(t, resp2.StatusCode == http.StatusUnauthorized || resp2.StatusCode == http.StatusForbidden,
		"Should return 401 Unauthorized or 403 Forbidden")

	t.Logf("✓ Invalid API Key rejected: Status %d", resp2.StatusCode)
}
