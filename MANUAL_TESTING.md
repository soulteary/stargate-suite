English | [中文](MANUAL_TESTING.zh-CN.md)

# Manual verification

Browser health checks and full login flow. E2E automation: [e2e/README.md](e2e/README.md).

## 1. Health

After `make up`, open:

| Service | URL | Expected |
|---------|-----|----------|
| Stargate | http://localhost:8080/health | `{"status":"ok",...}` |
| Warden | http://localhost:8081/health | `{"status":"UP"}` or ok |
| Herald | http://localhost:8082/healthz | ok |

## 2. Login (browser console)

In DevTools → Console, paste and run:

```javascript
const PHONE = "13800138000";
async function testLoginFlow() {
  const sendResp = await fetch("http://localhost:8080/_send_verify_code", {
    method: "POST", headers: { "Content-Type": "application/x-www-form-urlencoded" }, body: `phone=${PHONE}` });
  if (!sendResp.ok) throw new Error("Send failed: " + sendResp.status);
  const sendData = await sendResp.json();
  const challengeId = sendData.challenge_id;
  const codeResp = await fetch(`http://localhost:8082/v1/test/code/${challengeId}`, { headers: { "X-API-Key": "test-herald-api-key" } });
  if (!codeResp.ok) throw new Error("Get code failed: " + codeResp.status);
  const code = (await codeResp.json()).code;
  const loginResp = await fetch("http://localhost:8080/_login", {
    method: "POST", headers: { "Content-Type": "application/x-www-form-urlencoded" },
    body: `auth_method=warden&phone=${PHONE}&challenge_id=${challengeId}&verify_code=${code}` });
  if (loginResp.ok) {
    console.log("Login OK. Visit http://localhost:8080/_auth to see auth headers.");
    const authResp = await fetch("http://localhost:8080/_auth");
    authResp.headers.forEach((v, k) => { if (k.startsWith("x-auth")) console.log(k + ":", v); });
  } else console.error("Login failed:", await loginResp.text());
}
testLoginFlow();
```

## 3. Test accounts (fixtures/warden/data.json)

| Role | Phone | Email | User ID |
|------|-------|-------|---------|
| Admin | 13800138000 | admin@example.com | test-admin-001 |
| User | 13900139000 | user@example.com | test-user-002 |
| Guest | 13700137000 | guest@example.com | test-guest-003 |
| Inactive | 13600136000 | inactive@example.com | test-inactive-004 |
| Rate-limit | 13500135000 | ratelimit@example.com | test-ratelimit-005 |

See [README](README.md) · [e2e/README](e2e/README.md).
