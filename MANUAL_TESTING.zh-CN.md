中文 | [English](MANUAL_TESTING.md)

# 手动验证

浏览器健康检查与完整登录流程。自动化 E2E 见 [e2e/README.zh-CN.md](e2e/README.zh-CN.md)。

## 1. 健康检查

执行 `make up` 后访问：

| 服务 | URL | 预期 |
|------|-----|------|
| Stargate | http://localhost:8080/health | `{"status":"ok",...}` |
| Warden | http://localhost:8081/health | `{"status":"UP"}` 或 ok |
| Herald | http://localhost:8082/healthz | ok |

## 2. 登录（浏览器控制台）

打开开发者工具 → 控制台，粘贴并运行：

```javascript
const PHONE = "13800138000";
async function testLoginFlow() {
  const sendResp = await fetch("http://localhost:8080/_send_verify_code", {
    method: "POST", headers: { "Content-Type": "application/x-www-form-urlencoded" }, body: `phone=${PHONE}` });
  if (!sendResp.ok) throw new Error("发送失败: " + sendResp.status);
  const sendData = await sendResp.json();
  const challengeId = sendData.challenge_id;
  const codeResp = await fetch(`http://localhost:8082/v1/test/code/${challengeId}`, { headers: { "X-API-Key": "test-herald-api-key" } });
  if (!codeResp.ok) throw new Error("获取验证码失败: " + codeResp.status);
  const code = (await codeResp.json()).code;
  const loginResp = await fetch("http://localhost:8080/_login", {
    method: "POST", headers: { "Content-Type": "application/x-www-form-urlencoded" },
    body: `auth_method=warden&phone=${PHONE}&challenge_id=${challengeId}&verify_code=${code}` });
  if (loginResp.ok) {
    console.log("登录成功。访问 http://localhost:8080/_auth 查看授权头。");
    const authResp = await fetch("http://localhost:8080/_auth");
    authResp.headers.forEach((v, k) => { if (k.startsWith("x-auth")) console.log(k + ":", v); });
  } else console.error("登录失败:", await loginResp.text());
}
testLoginFlow();
```

## 3. 测试账号（fixtures/warden/data.json）

| 角色 | 手机号 | 邮箱 | User ID |
|------|--------|------|---------|
| Admin | 13800138000 | admin@example.com | test-admin-001 |
| User | 13900139000 | user@example.com | test-user-002 |
| Guest | 13700137000 | guest@example.com | test-guest-003 |
| Inactive | 13600136000 | inactive@example.com | test-inactive-004 |
| Rate-limit | 13500135000 | ratelimit@example.com | test-ratelimit-005 |

参见 [README.zh-CN](README.zh-CN.md) · [e2e/README.zh-CN](e2e/README.zh-CN.md)。
