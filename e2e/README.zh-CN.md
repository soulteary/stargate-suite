中文 | [English](README.md)

# 端到端测试文档

本文档说明 e2e 测试用例结构与运行方式。项目总览与快速开始见 [../README.zh-CN.md](../README.zh-CN.md)。

## 测试文件结构

```
e2e/
├── e2e_test.go              # 正常流程测试
├── error_scenarios_test.go  # 异常场景测试
├── test_helpers.go          # 测试辅助函数
├── warden_api_test.go       # Warden 直接 API 测试
├── herald_api_test.go       # Herald 直接 API 测试
├── auth_test.go             # 服务间鉴权测试
├── idempotency_test.go      # 幂等性测试
├── audit_test.go            # 审计日志测试
├── provider_test.go         # Provider 测试
├── metrics_test.go          # 监控指标测试
└── README.md, README.zh-CN.md   # 本文档为 README.zh-CN.md
```

## 测试用例列表

### 正常流程测试

#### TestCompleteLoginFlow
- **描述**: 测试完整的登录流程
- **步骤**:
  1. 发送验证码
  2. 从 Herald 测试端点获取验证码
  3. 使用验证码登录
  4. 验证 forwardAuth 检查返回正确的授权 Header

### 异常场景测试

#### 1. 验证码相关错误

##### TestInvalidVerificationCode
- **描述**: 测试使用错误的验证码登录
- **预期**: 返回 401 Unauthorized，错误信息包含验证码错误提示

##### TestExpiredVerificationCode
- **描述**: 测试使用过期的验证码登录
- **预期**: 返回 401 Unauthorized，错误信息包含过期提示
- **注意**: 实际测试中可能需要等待验证码过期或调整配置

##### TestVerificationCodeLocked
- **描述**: 测试多次错误验证码导致 challenge 锁定
- **预期**: 连续错误后返回 401 Unauthorized，错误信息包含锁定提示

#### 2. 用户相关错误

##### TestUserNotInWhitelist
- **描述**: 测试非白名单用户发送验证码
- **预期**: 返回 400 Bad Request 或 404 Not Found，错误信息包含不在白名单提示

##### TestInactiveUser
- **描述**: 测试非活跃用户发送验证码
- **预期**: 返回 400 Bad Request 或 404 Not Found
- **测试数据**: 使用 `13600136000` (inactive@example.com)

#### 3. 限流场景

##### TestIPRateLimit
- **描述**: 测试 IP 限流（快速发送多次请求）
- **预期**: 返回 429 Too Many Requests，错误信息包含频繁请求提示
- **配置**: 默认每分钟 5 次

##### TestUserRateLimit
- **描述**: 测试用户限流（同一用户快速发送多次请求）
- **预期**: 返回 429 Too Many Requests
- **配置**: 默认每小时 10 次
- **注意**: 可能需要更多请求或更长时间窗口

##### TestResendCooldown
- **描述**: 测试重发冷却（立即再次发送验证码）
- **预期**: 返回 429 Too Many Requests 或包含冷却时间提示
- **配置**: 默认 60 秒冷却时间

#### 4. 服务不可用场景

##### TestHeraldUnavailable
- **描述**: 测试 Herald 服务不可用时的处理
- **步骤**:
  1. 停止 Herald 服务
  2. 尝试发送验证码
  3. 恢复服务
- **预期**: 返回 503 Service Unavailable，错误信息包含服务不可用提示
- **注意**: 需要 docker compose 访问权限

##### TestWardenUnavailable
- **描述**: 测试 Warden 服务不可用时的处理
- **步骤**:
  1. 停止 Warden 服务
  2. 尝试发送验证码
  3. 恢复服务
- **预期**: 返回 503、500 或 404，错误信息包含服务不可用提示
- **注意**: 需要 docker compose 访问权限

#### 5. 认证相关错误

##### TestUnauthenticatedAccess
- **描述**: 测试未登录访问 forwardAuth
- **预期**: 返回 401 Unauthorized 或 302 Redirect

##### TestInvalidSessionCookie
- **描述**: 测试使用无效 session cookie 访问 forwardAuth
- **预期**: 返回 401 Unauthorized 或 302 Redirect

#### 6. 边界场景

##### TestEmptyRequestParameters
- **描述**: 测试空请求参数（空手机号或邮箱）
- **预期**: 返回 400 Bad Request

##### TestInvalidChallengeID
- **描述**: 测试使用不存在的 challenge_id 登录
- **预期**: 返回 401 Unauthorized，错误信息包含过期或无效提示

##### TestInvalidAuthMethod
- **描述**: 测试使用不支持的 auth_method 值登录
- **预期**: 返回 400 Bad Request 或 401 Unauthorized

## 测试数据

测试数据位于 `fixtures/warden/data.json`，包含以下用户：

1. **管理员用户** (`13800138000`) — admin@example.com, test-admin-001, active, read, write, admin
2. **普通用户** (`13900139000`) — user@example.com, test-user-002, active, read
3. **访客用户** (`13700137000`) — guest@example.com, test-guest-003, active, read
4. **非活跃用户** (`13600136000`) — inactive@example.com, test-inactive-004, inactive, read
5. **限流测试用户** (`13500135000`) — ratelimit@example.com, test-ratelimit-005, active, read

## 运行测试

### 运行所有测试
```bash
go test -v ./e2e/...
```

### 运行特定测试
```bash
go test -v ./e2e/... -run TestCompleteLoginFlow
go test -v ./e2e/... -run TestInvalid
go test -v ./e2e/... -run TestInvalidVerificationCode
```

### 运行服务不可用测试
```bash
go test -v ./e2e/... -run TestHeraldUnavailable
go test -v ./e2e/... -run TestWardenUnavailable
```

## 注意事项

1. **服务就绪**: 所有测试都会先检查服务是否就绪，如果服务未启动，测试会失败
2. **测试隔离**: 每个测试用例使用独立的测试用户，避免相互影响
3. **限流测试**: 某些限流测试可能需要调整 Herald 配置或等待时间窗口
4. **服务不可用测试**: 需要 docker compose 访问权限，如果无权限会跳过测试
5. **验证码过期测试**: 默认需要等待 5 分钟，可以通过调整 Herald 的 `CHALLENGE_EXPIRY` 配置加速
6. **并发安全**: 多个测试并行运行时，确保使用不同的测试用户

## 测试辅助函数

### test_helpers.go 提供的函数

- `waitForService`, `ensureServicesReady`: 等待服务就绪；`ensureServicesReady` 还会清理限流状态
- `waitForServiceDown`: 等待服务停止（返回非 2xx 或连接失败）
- `sendVerificationCodeWithError`: 发送验证码并返回错误响应（如果失败）
- `loginWithError`: 登录并返回错误响应（如果失败）
- `checkAuthWithError`: 验证授权并返回错误响应（如果失败）
- `triggerRateLimit`: 触发限流（快速发送多次请求）
- `stopDockerServiceInDir`: 在指定目录停止 Docker 服务
- `startDockerServiceInDir`: 在指定目录启动 Docker 服务
- `sendVerificationCodeWithEmail`: 使用邮箱发送验证码
- `clearRateLimitKeys`: 清理 Redis 中的测试状态

## 错误响应格式

所有错误响应都使用 `ErrorResponse` 结构：

```go
type ErrorResponse struct {
    StatusCode int    // HTTP 状态码
    Message    string // 错误消息
    Body       string // 响应体
}
```

## 相关文档

- [../README.zh-CN.md](../README.zh-CN.md) — 项目总览、服务说明、故障排查
- [../MANUAL_TESTING.zh-CN.md](../MANUAL_TESTING.zh-CN.md) — 浏览器手动验证
