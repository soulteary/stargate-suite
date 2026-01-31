English | [中文](MANUAL_TESTING.zh-CN.md)

# Stargate Suite — Manual Verification Guide

This document describes how to manually verify Stargate Suite in the browser: basic health checks and the full login flow. For automated E2E tests see [e2e/README.md](e2e/README.md).

## 1. Basic health checks

Ensure services are started (`make up`), then open these URLs in your browser. If the response contains `ok` or `true`, the service is running.

| Service | Role | URL | Expected |
| :--- | :--- | :--- | :--- |
| Stargate | Auth | [http://localhost:8080/health](http://localhost:8080/health) | `{"status":"ok",...}` |
| Warden | User service | [http://localhost:8081/health](http://localhost:8081/health) | `{"status":"UP"}` or `ok` |
| Herald | OTP/verification | [http://localhost:8082/healthz](http://localhost:8082/healthz) | `ok` |

---

## 2. Login flow (browser console)

The full login flow uses `POST` requests. Easiest is to run the steps from the browser DevTools console.

**Steps:**
1. Open Chrome or Edge.
2. Press `F12` to open Developer Tools.
3. Go to the **Console** tab.
4. Paste and run the script below.

```javascript
// Test user phone (admin in whitelist)
const PHONE = "13800138000";

async function testLoginFlow() {
  console.log("🚀 Starting login flow test...");

  // Step 1: Send verification code
  console.log("1️⃣ Requesting verification code...");
  const sendResp = await fetch("http://localhost:8080/_send_verify_code", {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
    body: `phone=${PHONE}`
  });

  if (!sendResp.ok) throw new Error(`Send failed: ${sendResp.status}`);
  const sendData = await sendResp.json();
  console.log("✅ Verification code sent:", sendData);

  const challengeId = sendData.challenge_id;

  // Step 2: Get code (Herald test endpoint, test mode only)
  console.log(`2️⃣ Getting code (Challenge ID: ${challengeId})...`);
  const codeResp = await fetch(`http://localhost:8082/v1/test/code/${challengeId}`, {
    headers: { "X-API-Key": "test-herald-api-key" }
  });

  if (!codeResp.ok) throw new Error(`Failed to get code: ${codeResp.status}`);
  const codeData = await codeResp.json();
  const code = codeData.code;
  console.log(`✅ Got code: ${code}`);

  // Step 3: Login
  console.log("3️⃣ Submitting login...");
  const loginResp = await fetch("http://localhost:8080/_login", {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
    body: `auth_method=warden&phone=${PHONE}&challenge_id=${challengeId}&verify_code=${code}`
  });

  if (loginResp.ok) {
    console.log("🎉 Login successful. Session cookie set.");
    console.log("Visit http://localhost:8080/_auth to see auth headers.");

    const authResp = await fetch("http://localhost:8080/_auth");
    console.log("🔍 Auth headers:");
    authResp.headers.forEach((val, key) => {
      if (key.startsWith("x-auth")) console.log(`${key}: ${val}`);
    });
  } else {
    console.error("❌ Login failed:", await loginResp.text());
  }
}

testLoginFlow();
```

---

## 3. Test accounts

Defined in `fixtures/warden/data.json`:

| Role | Phone | Email | User ID |
| :--- | :--- | :--- | :--- |
| Admin | `13800138000` | `admin@example.com` | `test-admin-001` |
| User | `13900139000` | `user@example.com` | `test-user-002` |
| Guest | `13700137000` | `guest@example.com` | `test-guest-003` |

## See also

- [README.md](README.md) — Overview and quick start
- [e2e/README.md](e2e/README.md) — Automated E2E tests
