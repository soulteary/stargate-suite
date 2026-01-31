中文 | [English](MANUAL_TESTING.md)

# Stargate Suite 手动验证指南

本文档说明如何在浏览器中手动验证 Stargate Suite 服务，包括基础健康检查与完整登录流程。自动化端到端测试见 [e2e/README.zh-CN.md](e2e/README.zh-CN.md)。

## 1. 基础健康检查

确保服务已启动（`make up`），在浏览器中访问以下地址。若返回 `ok` 或 `true` 表示服务正常。

| 服务 | 角色 | URL | 预期返回 |
| :--- | :--- | :--- | :--- |
| Stargate | 认证入口 | [http://localhost:8080/health](http://localhost:8080/health) | `{"status":"ok",...}` |
| Warden | 用户服务 | [http://localhost:8081/health](http://localhost:8081/health) | `{"status":"UP"}` 或 `ok` |
| Herald | 验证码服务 | [http://localhost:8082/healthz](http://localhost:8082/healthz) | `ok` |

---

## 2. 业务流程验证（浏览器控制台）

完整登录流程涉及 `POST` 请求，建议在浏览器控制台中模拟调用。

**步骤：**
1. 打开 Chrome/Edge 浏览器。
2. 按 `F12` 打开开发者工具。
3. 切换到 **Console（控制台）** 标签。
4. 粘贴并运行以下代码。

```javascript
// 1. 定义测试用户手机号（白名单中的管理员号）
const PHONE = "13800138000";

async function testLoginFlow() {
  console.log("🚀 Starting login flow test...");

  // --- Step 1: 发送验证码 ---
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

  // --- Step 2: 获取验证码（使用测试模式后门） ---
  console.log(`2️⃣ Getting verification code from Herald (Challenge ID: ${challengeId})...`);
  const codeResp = await fetch(`http://localhost:8082/v1/test/code/${challengeId}`, {
    headers: { "X-API-Key": "test-herald-api-key" }
  });

  if (!codeResp.ok) throw new Error(`Failed to get verification code: ${codeResp.status}`);
  const codeData = await codeResp.json();
  const code = codeData.code;
  console.log(`✅ Got verification code: ${code}`);

  // --- Step 3: 登录 ---
  console.log("3️⃣ Submitting login...");
  const loginResp = await fetch("http://localhost:8080/_login", {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
    body: `auth_method=warden&phone=${PHONE}&challenge_id=${challengeId}&verify_code=${code}`
  });

  if (loginResp.ok) {
    console.log("🎉 Login successful! Session Cookie set.");
    console.log("You can visit http://localhost:8080/_auth to view auth info.");

    // --- Step 4: 验证授权信息 ---
    const authResp = await fetch("http://localhost:8080/_auth");
    console.log("🔍 Auth check result (Headers):");
    authResp.headers.forEach((val, key) => {
        if (key.startsWith("x-auth")) console.log(`${key}: ${val}`);
    });
  } else {
    console.error("❌ Login failed:", await loginResp.text());
  }
}

// 运行测试
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
| Inactive（非活跃） | `13600136000` | `inactive@example.com` | `test-inactive-004` |
| Rate-limit test（限流测试） | `13500135000` | `ratelimit@example.com` | `test-ratelimit-005` |

## 相关文档

- [README.zh-CN.md](README.zh-CN.md) — 项目总览与快速开始
- [e2e/README.zh-CN.md](e2e/README.zh-CN.md) — 端到端自动化测试
