package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/MarvinJWendt/testza"
)

// This file holds the PR9 v1 failure / replay / recovery contract tests. They
// assert the security posture pinned by PR8 against the real v1 services:
//   - liveness vs readiness are DISTINCT endpoints (a dependency outage must
//     not flap liveness),
//   - Herald HMAC v2 nonces are single-use (replay is rejected),
//   - the Herald test-code endpoint is NEVER reachable on the public :8082
//     listener (only the dedicated loopback listener serves it).
//
// The endpoint/contract details are cross-verified against upstream stable
// tags (Stargate v1.0.0, Warden v1.0.0, Herald v1.1.0):
//   - Stargate: /healthz = SimpleFiberHandler (no deps), /readyz = aggregator.
//   - Herald:   /livez = no deps, /readyz = Redis ping (503 when down),
//               /v1/test/code only on the dedicated test listener.
//   - Warden:   /healthcheck (+ /v1/healthcheck, /health).

// TestLivenessReadinessAreDistinct asserts every core service exposes a
// liveness probe that does not touch dependencies AND a separate readiness
// probe, so an orchestrator can tell "process up" from "ready to serve". This
// is the liveness/readiness separation required by PR9 (source plan 9.3).
func TestLivenessReadinessAreDistinct(t *testing.T) {
	ensureServicesReady(t)

	client := &http.Client{Timeout: 10 * time.Second}
	getStatus := func(url string) (int, string) {
		resp, err := client.Get(url)
		if err != nil {
			return 0, err.Error()
		}
		defer func() { _ = resp.Body.Close() }()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(b)
	}

	cases := []struct {
		name     string
		liveness string
		readines string
	}{
		// Stargate liveness /healthz is dependency-free; /readyz aggregates deps.
		{"stargate", stargateURL + "/healthz", stargateURL + "/readyz"},
		// Herald /livez never touches Redis; /readyz pings Redis.
		{"herald", heraldURL + "/livez", heraldURL + "/readyz"},
	}
	for _, tc := range cases {
		liveCode, liveBody := getStatus(tc.liveness)
		testza.AssertEqual(t, http.StatusOK, liveCode,
			"%s liveness (%s) must be 200 when the process is up; body: %s", tc.name, tc.liveness, liveBody)

		readyCode, readyBody := getStatus(tc.readines)
		// When all deps are up readiness is 200; if a dependency is down it is
		// 503. Either way it must be a real, reachable endpoint distinct from
		// liveness (not 404), which is the contract we assert here.
		testza.AssertTrue(t, readyCode == http.StatusOK || readyCode == http.StatusServiceUnavailable,
			"%s readiness (%s) must be 200 or 503, got %d; body: %s", tc.name, tc.readines, readyCode, readyBody)
		testza.AssertNotEqual(t, http.StatusNotFound, readyCode,
			"%s readiness endpoint %s must exist (liveness/readiness are separate)", tc.name, tc.readines)
		t.Logf("✓ %s liveness=%d readiness=%d (distinct endpoints)", tc.name, liveCode, readyCode)
	}
}

// TestHeraldNonceReplayRejected asserts a v2 nonce is single-use: replaying the
// exact same signed request (same timestamp + nonce + signature) must be
// rejected with a replay reason. This is the anti-replay guarantee of HMAC v2
// (herald/internal/auth/middleware.go verifies the signature, then consumes the
// nonce via SET NX EX; a second use is not fresh -> replayed_nonce).
func TestHeraldNonceReplayRejected(t *testing.T) {
	ensureServicesReady(t)
	time.Sleep(2 * time.Second) // avoid rate limiting on the first send

	reqBody := HeraldChallengeRequest{
		UserID:      "test-user-replay",
		Channel:     "sms",
		Destination: "+8613800138000",
		Purpose:     "login",
	}
	bodyBytes, err := json.Marshal(reqBody)
	testza.AssertNoError(t, err)

	// Fixed timestamp + nonce so the two requests are byte-identical and the
	// only difference the server sees is that the nonce was already consumed.
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := "e2e-replay-nonce-" + ts
	service := "stargate"
	sig := signHeraldV2Fixed("POST", "/v1/otp/challenges", "", ts, nonce, service, "", heraldHMACSecret, bodyBytes)

	doSigned := func() (int, string) {
		req, reqErr := http.NewRequest("POST", heraldURL+"/v1/otp/challenges", bytes.NewReader(bodyBytes))
		testza.AssertNoError(t, reqErr)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("X-Signature-Version", "v2")
		req.Header.Set("X-Signature", sig)
		req.Header.Set("X-Timestamp", ts)
		req.Header.Set("X-Nonce", nonce)
		req.Header.Set("X-Service", service)
		client := &http.Client{Timeout: 10 * time.Second}
		resp, doErr := client.Do(req)
		testza.AssertNoError(t, doErr)
		defer func() { _ = resp.Body.Close() }()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(b)
	}

	firstCode, firstBody := doSigned()
	// The first request should be accepted (200) unless rate-limited (429); in
	// both cases the nonce is consumed. If the very first is already rejected
	// as unauthorized the anti-replay contract cannot be exercised meaningfully.
	if firstCode == http.StatusTooManyRequests {
		t.Skipf("first request rate-limited (429); cannot exercise replay deterministically. body: %s", firstBody)
	}
	testza.AssertTrue(t, firstCode == http.StatusOK,
		"first signed request should be accepted (200) so the nonce is consumed, got %d; body: %s", firstCode, firstBody)

	secondCode, secondBody := doSigned()
	testza.AssertTrue(t, secondCode == http.StatusUnauthorized || secondCode == http.StatusForbidden,
		"replaying the same nonce must be rejected (401/403), got %d; body: %s", secondCode, secondBody)
	t.Logf("✓ nonce replay rejected: first=%d second=%d body=%s", firstCode, secondCode, secondBody)
}

// TestHeraldTestCodeNotOnMainListener asserts the test-code endpoint is NOT
// reachable on the public :8082 listener. Herald v1.1.0 mounts
// /v1/test/code/:challenge_id only on a dedicated loopback listener (router.go
// builds a separate testApp), so the public listener must never expose codes —
// hitting it there is a 404 (route absent), never a 200 leaking a code.
func TestHeraldTestCodeNotOnMainListener(t *testing.T) {
	ensureServicesReady(t)

	url := fmt.Sprintf("%s/v1/test/code/%s", heraldURL, "any-challenge-id")
	req, err := http.NewRequest("GET", url, nil)
	testza.AssertNoError(t, err)
	// Even with the test API key, the route does not exist on the main listener.
	req.Header.Set("X-Test-Api-Key", "test-herald-test-code-key")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	testza.AssertNoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)

	testza.AssertNotEqual(t, http.StatusOK, resp.StatusCode,
		"main listener must never serve a test code (got 200); body: %s", string(b))
	testza.AssertEqual(t, http.StatusNotFound, resp.StatusCode,
		"test-code route must be absent from the main :8082 listener (dedicated listener only), got %d", resp.StatusCode)
	t.Logf("✓ /v1/test/code absent on main listener: Status %d", resp.StatusCode)
}
