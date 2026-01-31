# Stargate Suite 手动验证指南

本文档说明如何在浏览器中手动验证 Stargate Suite 服务，包括基础健康检查与完整登录流程。自动化端到端测试见 [e2e/README.md](e2e/README.md)。

## 1. 基础健康检查

Ensure services are started (`make up`), then visit the following addresses in your browser. If `ok` or `true` is returned, the service is running normally.

| 服务 | 角色 | URL | 预期返回 |
| :--- | :--- | :--- | :--- |
| Stargate | 认证入口 | [http://localhost:8080/health](http://localhost:8080/health) | `{"status":"ok",...}` |
| Warden | 用户服务 | [http://localhost:8081/health](http://localhost:8081/health) | `{"status":"UP"}` 或 `ok` |
| Herald | 验证码服务 | [http://localhost:8082/healthz](http://localhost:8082/healthz) | `ok` |

---

## 2. 业务流程验证（浏览器控制台）

完整登录流程涉及 `POST` 请求，建议在浏览器控制台中模拟调用。

**步骤：**
1. Open Chrome/Edge browser.
2. Press `F12` to open Developer Tools.
3. Switch to the **Console** tab.
4. Paste and run the following code.

```javascript
// 1. Define test user phone (admin number in whitelist)
const PHONE = "13800138000";

async function testLoginFlow() {
  console.log("🚀 Starting login flow test...");

  // --- Step 1: Send verification code ---
  console.log("1️⃣ Requesting verification code...");
  const sendResp = await fetch("http://localhost:8080/_send_verify_code", {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
    body: `phone=${PHONE}`
  });
  
  if (!sendResp.ok) throw new Error(`Send failed: ${sendResp.status}`);
  const sendData = await sendResp.json();
  console.log("✅ Verification code sent successfully:", sendData);
  
  const challengeId = sendData.challenge_id;

  // --- Step 2: Get verification code (using test mode backdoor) ---
  console.log(`2️⃣ Getting verification code from Herald (Challenge ID: ${challengeId})...`);
  // Note: Directly requesting Herald's test endpoint here
  const codeResp = await fetch(`http://localhost:8082/v1/test/code/${challengeId}`, {
    headers: { "X-API-Key": "test-herald-api-key" }
  });
  
  if (!codeResp.ok) throw new Error(`Failed to get verification code: ${codeResp.status}`);
  const codeData = await codeResp.json();
  const code = codeData.code;
  console.log(`✅ Got verification code: ${code}`);

  // --- Step 3: Login ---
  console.log("3️⃣ Submitting login...");
  const loginResp = await fetch("http://localhost:8080/_login", {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
    body: `auth_method=warden&phone=${PHONE}&challenge_id=${challengeId}&verify_code=${code}`
  });

  if (loginResp.ok) {
    console.log("🎉 Login successful! Session Cookie set.");
    console.log("You can visit http://localhost:8080/_auth to view auth info.");
    
    // --- Step 4: Verify auth info ---
    const authResp = await fetch("http://localhost:8080/_auth");
    console.log("🔍 Auth check result (Headers):");
    authResp.headers.forEach((val, key) => {
        if (key.startsWith("x-auth")) console.log(`${key}: ${val}`);
    });
  } else {
    console.error("❌ Login failed:", await loginResp.text());
  }
}

// Run test
testLoginFlow();
```

---

## 3. 常用测试数据

测试数据定义在 `fixtures/warden/data.json`。常用测试账号如下：

| 角色 | 手机号 | 邮箱 | User ID |
| :--- | :--- | :--- | :--- |
| Admin | `13800138000` | `admin@example.com` | `test-admin-001` |
| User | `13900139000` | `user@example.com` | `test-user-002` |
| Guest | `13700137000` | `guest@example.com` | `test-guest-003` |

## 相关文档

- [README.md](README.md) — 项目总览与快速开始
- [e2e/README.md](e2e/README.md) — 端到端自动化测试
