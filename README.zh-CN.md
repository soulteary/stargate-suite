中文 | [English](README.md)

# stargate-suite - 三服务端到端集成测试套件

本仓库为 **Stargate + Warden + Herald** 三服务的端到端集成测试环境，提供多种 Compose 用法、CLI 编排与自动化测试，覆盖正常流程、异常场景、服务间鉴权、幂等、审计与监控等。

Go 模块名为 `github.com/soulteary/the-gate`，仓库与产品名为 **stargate-suite**。

## 文档导航

| 文档 | 说明 |
|------|------|
| [README.zh-CN.md](./README.zh-CN.md)（本文） | 项目总览、快速开始、服务说明、故障排查 |
| [compose/README.zh-CN.md](./compose/README.zh-CN.md) | Compose 各子目录用法（build / image / traefik 等） |
| [config/README.zh-CN.md](./config/README.zh-CN.md) | CLI 预设与 `-f` / `--preset` 使用方式 |
| [compose/traefik/README.zh-CN.md](./compose/traefik/README.zh-CN.md) | Traefik 三合一与三分开部署说明 |
| [e2e/README.zh-CN.md](./e2e/README.zh-CN.md) | 端到端测试用例与运行说明 |
| [MANUAL_TESTING.zh-CN.md](./MANUAL_TESTING.zh-CN.md) | 浏览器手动验证与健康检查 |

## 项目结构

```
stargate-suite/
├── compose/                # Compose 源与示例
│   ├── README.md           # Compose 使用说明
│   ├── example/            # 静态示例（唯二保留）
│   │   ├── image/          # 预构建镜像示例
│   │   │   └── docker-compose.yml
│   │   └── build/          # 从源码构建示例
│   │       └── docker-compose.yml
│   ├── canonical/          # 单一数据源（用于生成 traefik / 三分开）
│   │   └── docker-compose.yml
│   └── traefik/            # 可选：Traefik 部署说明
│       └── README.md
├── build/                  # 生成输出（gen 或 Web UI 生成，默认不提交）
│   ├── image/              # 来自 example/image + .env
│   ├── build/               # 来自 example/build + .env
│   ├── traefik/            # 来自 canonical 解析生成
│   ├── traefik-herald/     # 三分开
│   ├── traefik-warden/
│   └── traefik-stargate/
├── go.mod                  # Go 模块定义
├── go.sum                  # Go 依赖锁定
├── Makefile                # 便捷命令脚本
├── config/                 # CLI 预设（presets.json）、Web UI 配置（page.yaml）
├── cmd/suite/              # Go CLI + Web UI
│   ├── main.go
│   ├── compose_split.go
│   └── static/index.html.tmpl  # 生成器页面模板
├── internal/composegen/    # Compose 解析与生成
├── README.md, README.zh-CN.md   # 根目录文档（本文为 README.zh-CN.md）
├── LICENSE                 # 许可证文件
├── e2e/                    # 端到端测试代码
│   ├── e2e_test.go         # 正常流程测试
│   ├── error_scenarios_test.go  # 异常场景测试
│   ├── auth_test.go        # 服务间鉴权测试
│   ├── herald_api_test.go  # Herald 直接 API 测试
│   ├── warden_api_test.go  # Warden 直接 API 测试
│   ├── idempotency_test.go # 幂等性测试
│   ├── audit_test.go       # 审计日志测试
│   ├── provider_test.go    # Provider 测试
│   ├── metrics_test.go     # 监控指标测试
│   ├── test_helpers.go     # 测试辅助函数
│   └── README.md           # 测试详细文档
├── fixtures/               # 测试数据
│   └── warden/
│       └── data.json      # Warden 白名单用户数据
└── scripts/                # 运行脚本
    └── run-e2e.sh
```

根目录文档：`README.md`（英文）、`README.zh-CN.md`（本文）；compose/、config/、e2e/ 及 MANUAL_TESTING 均提供中英双语文档。

## 快速开始

### 前置要求

- Docker 和 Docker Compose
- Go 1.25+（见 `go.mod`，构建请使用该版本或更高）
- 约 1GB 可用磁盘空间（用于 Docker 镜像和数据卷）

### 默认 compose 与预设

- **未执行 `gen` 前**：默认 compose 路径为 `compose/example/image/docker-compose.yml`（或使用 `--preset default`），可直接用该静态示例启动。
- **执行 `make gen` 或 `go run ./cmd/suite gen all` 后**：日常推荐使用 `build/image/docker-compose.yml`，可用 `--preset image` 或 `-f build/image/docker-compose.yml` 指定；默认不会自动切换到 `build/`，需显式指定。

详见 [config/README.zh-CN.md](./config/README.zh-CN.md) 的预设列表与覆盖顺序。

### 启动服务

**首次使用请先生成配置到 `build/` 目录：**

```bash
make gen
# 或
go run ./cmd/suite gen all
```

然后启动：

```bash
# 方式 1: 使用 Makefile（默认使用 build/image）
make up

# 使用不同 compose
make up-image    # 预构建镜像（build/image）
make up-build    # 从源码构建（build/build）
make up-traefik  # 接入 Traefik（build/traefik）

# 方式 2: 直接指定 compose 文件（在项目根目录执行）
docker compose -f build/image/docker-compose.yml up -d
docker compose -f build/build/docker-compose.yml up -d --build
docker compose -f build/traefik/docker-compose.yml up -d

# 查看服务状态
make ps
# 或
docker compose -f build/image/docker-compose.yml ps

# 查看服务日志
make logs
# 或
docker compose -f build/image/docker-compose.yml logs -f
```

**方式 3：使用 Go CLI（与 Makefile 等效，跨平台）**

```bash
# 查看所有命令
go run ./cmd/suite help
# 或先构建再使用
make suite-build && ./bin/suite help

# 通过 Makefile 调用 CLI（ARGS 为子命令及参数）
make suite ARGS="up"
make suite ARGS="health"

# 直接运行
go run ./cmd/suite up
go run ./cmd/suite test-wait
go run ./cmd/suite health
```

支持通过环境变量 `COMPOSE_FILE` 指定默认 compose 文件，与 Makefile 一致。

**生成 build 目录（输出不同使用方式的 compose 与 .env）**

通过 CLI 将指定使用方式的 `docker-compose.yml` 和 `.env` 输出到 `build` 目录（可通过 `-o` 指定），便于分发或 CI：

```bash
# 在项目根目录执行
go run ./cmd/suite gen [mode]     # mode 默认 all，输出到 build/
go run ./cmd/suite gen image      # 从 compose/example/image 拷贝 → build/image/
go run ./cmd/suite gen build      # 从 compose/example/build 拷贝 → build/build/
go run ./cmd/suite gen traefik    # 从 compose/canonical 解析生成 → build/traefik/、traefik-herald/、traefik-warden/、traefik-stargate/
go run ./cmd/suite gen all        # 上述全部（共 6 个子目录）

# 指定输出目录（默认 build）
go run ./cmd/suite -o dist gen traefik
GEN_OUT_DIR=dist go run ./cmd/suite gen all
```

生成后使用示例：`docker compose -f build/image/docker-compose.yml --env-file build/image/.env up -d`。Compose 说明见 [compose/README.zh-CN.md](./compose/README.zh-CN.md)。

**Web UI 生成**

在项目根目录执行 `go run ./cmd/suite serve`（默认 http://localhost:8085），在网页上勾选要生成的 compose 类型，点击生成即可下载 `docker-compose.yml` 与 `.env`。也可通过 `-port` 或 `SERVE_PORT` 指定端口。

- **安全说明**：Web UI 与 `/api/generate` 无鉴权，仅在本地或可信环境使用，请勿暴露到公网。

### 运行测试

等待所有服务就绪后（约 30 秒），运行端到端测试：

```bash
# 方式 1: 使用 Makefile（推荐，自动等待服务就绪）
make test-wait

# 方式 2: 直接运行 Go 测试
go test -v ./e2e/...

# 方式 3: 使用脚本
./scripts/run-e2e.sh

# 运行特定测试
go test -v ./e2e/... -run TestCompleteLoginFlow
```

### 停止服务

```bash
# 停止所有服务（与启动时使用的 compose 一致，默认 build/image）
make down
# 或
docker compose -f build/image/docker-compose.yml down

# 停止服务并清理数据卷
make clean
# 或
docker compose -f build/image/docker-compose.yml down -v
```

## 服务配置

### 端口映射

- **Stargate**: `http://localhost:8080`
- **Warden**: `http://localhost:8081`
- **Herald**: `http://localhost:8082`
- **Herald Redis**: `localhost:6379`（仅用于测试清理）

### 环境变量

可选：将根目录 `.env.example` 复制为 `.env` 并在执行 `make gen` 前按需修改，以覆盖镜像版本与密钥。也可通过 `.env` 或环境变量覆盖默认配置。

主要配置项：

- `AUTH_HOST`: Stargate 认证主机（默认: `auth.test.localhost`）
- `PASSWORDS`: Stargate 密码配置（默认: `plaintext:test1234|test1337`）
- `WARDEN_API_KEY`: Warden API 密钥（默认: `test-warden-api-key`）
- `HERALD_API_KEY`: Herald API 密钥（默认: `test-herald-api-key`）
- `HERALD_HMAC_SECRET`: Herald HMAC 密钥（默认: `test-hmac-secret`）

### 测试用户

测试数据位于 `fixtures/warden/data.json`，包含以下用户：

1. **管理员用户** (`13800138000`)
   - 邮箱: `admin@example.com`
   - User ID: `test-admin-001`
   - 状态: `active`
   - 权限: `read, write, admin`
   - 角色: `admin`

2. **普通用户** (`13900139000`)
   - 邮箱: `user@example.com`
   - User ID: `test-user-002`
   - 状态: `active`
   - 权限: `read`
   - 角色: `user`

3. **访客用户** (`13700137000`)
   - 邮箱: `guest@example.com`
   - User ID: `test-guest-003`
   - 状态: `active`
   - 权限: `read`
   - 角色: `guest`

4. **非活跃用户** (`13600136000`) - 用于测试
   - 邮箱: `inactive@example.com`
   - User ID: `test-inactive-004`
   - 状态: `inactive`
   - 权限: `read`
   - 角色: `user`

5. **限流测试用户** (`13500135000`) - 用于测试
   - 邮箱: `ratelimit@example.com`
   - User ID: `test-ratelimit-005`
   - 状态: `active`
   - 权限: `read`
   - 角色: `user`

## 测试套件

### 测试分类

测试套件包含 **50+ 个测试用例**，覆盖以下方面：

#### 1. 正常流程测试
- `TestCompleteLoginFlow`: 完整登录流程（发送验证码 → 获取验证码 → 登录 → 验证授权）

#### 2. 异常场景测试（`error_scenarios_test.go`）
- 验证码错误、过期、锁定
- 用户不在白名单、非活跃用户
- IP 限流、用户限流、重发冷却
- 服务不可用场景（Herald/Warden 宕机）
- 认证错误（未登录、无效 session）
- 边界场景（空参数、无效 challenge_id、无效 auth_method）

#### 3. 服务间鉴权测试（`auth_test.go`）
- Herald HMAC 签名验证（有效、无效、过期、缺失）
- Warden API Key 验证（必需、无效）
- Herald API Key 验证（有效、无效）

#### 4. Herald API 测试（`herald_api_test.go`）
- Challenge 创建（SMS/Email）
- Challenge 验证
- Challenge 过期处理
- 错误验证码处理
- 限流测试
- Challenge 撤销
- HMAC 认证

#### 5. Warden API 测试（`warden_api_test.go`）
- 通过手机号查询用户
- 通过邮箱查询用户
- 通过 User ID 查询用户
- 用户不存在处理
- 无效参数处理
- API Key 认证

#### 6. 幂等性测试（`idempotency_test.go`）
- 相同 Idempotency-Key 返回相同结果
- 不同 Idempotency-Key 返回不同结果
- 不使用 Idempotency-Key 的行为

#### 7. 审计日志测试（`audit_test.go`）
- Herald 审计日志记录
- Warden 审计日志记录

#### 8. Provider 测试（`provider_test.go`）
- Provider 失败处理（soft 模式）
- Provider 重试与幂等性
- Provider 错误码归一化
- Email Provider 测试
- SMS Provider 测试

#### 9. 监控指标测试（`metrics_test.go`）
- Herald Prometheus 指标更新
- Warden Prometheus 指标更新
- 指标格式验证

### 测试隔离机制

测试套件实现了完善的测试隔离机制：

- **自动清理限流状态**: 每个测试开始前自动清理 Redis 中的限流键、冷却键、用户锁定键和 challenge 键
- **独立测试用户**: 每个测试用例使用独立的测试用户，避免相互影响
- **服务就绪检查**: 所有测试都会先检查服务是否就绪

### 运行特定测试

```bash
# 运行正常流程测试
go test -v ./e2e/... -run TestCompleteLoginFlow

# 运行异常场景测试
go test -v ./e2e/... -run TestInvalid

# 运行鉴权测试
go test -v ./e2e/... -run TestHeraldHMAC

# 运行限流测试
go test -v ./e2e/... -run TestRateLimit

# 运行服务不可用测试（需要 docker compose 权限）
go test -v ./e2e/... -run TestHeraldUnavailable
go test -v ./e2e/... -run TestWardenUnavailable
```

## Makefile 命令

项目提供了便捷的 Makefile 命令（默认使用 `build/image`，可通过 `COMPOSE_FILE` 覆盖）：

```bash
make help                  # 显示帮助信息
make gen                   # 生成 compose 与 .env 到 build/
make up                    # 启动所有服务（默认 build/image）
make up-build              # 从源码构建并启动（build/build）
make up-image              # 使用预构建镜像启动（build/image）
make up-traefik            # 接入 Traefik 启动（build/traefik）
make up-traefik-herald     # 三分开：仅 Herald
make up-traefik-warden     # 三分开：仅 Warden
make up-traefik-stargate   # 三分开：Stargate + 受保护服务
make down                  # 停止所有服务
make down-build            # 停止 build/build 启动的服务
make down-image            # 停止 build/image 启动的服务
make down-traefik          # 停止 build/traefik 三合一
make down-traefik-herald   # 三分开：停止 Herald
make down-traefik-warden   # 三分开：停止 Warden
make down-traefik-stargate # 三分开：停止 Stargate
make net-traefik-split     # 创建三分开所需网络（执行一次）
make logs                  # 查看服务日志
make ps                    # 查看服务状态
make test                  # 运行端到端测试
make test-wait             # 等待服务就绪后运行测试（推荐）
make clean                 # 清理服务和数据卷
make restart               # 重启所有服务
make restart-warden        # 重启 Warden 服务
make restart-herald        # 重启 Herald 服务
make restart-stargate      # 重启 Stargate 服务
make health                # 检查服务健康状态
make suite ARGS="..."      # 运行 CLI（如 make suite ARGS="up"）
make suite-build           # 构建 bin/suite
make serve                 # 启动 Web UI（默认 :8085）
```

## 服务说明

### Stargate（门）

Traefik forwardAuth 鉴权服务，负责：
- 会话管理（Cookie/JWT）
- 登录流程编排（调用 Warden 获取用户信息，调用 Herald 发送/验证验证码）
- forwardAuth 检查（验证 session，返回授权 Header）

**主要接口**:
- `GET /_auth`: forwardAuth 检查接口
- `POST /_send_verify_code`: 发送验证码接口
- `POST /_login`: 登录接口

### Warden（看守）

白名单用户信息服务，负责：
- 用户白名单管理
- 提供用户基本信息（email/phone/user_id/status/scope/role）
- 用户状态验证（active/inactive）

**主要接口**:
- `GET /user?phone={phone}`: 通过手机号查询用户
- `GET /user?mail={email}`: 通过邮箱查询用户
- `GET /user?user_id={user_id}`: 通过 User ID 查询用户
- `GET /metrics`: Prometheus 指标端点

### Herald（传令）

验证码与 OTP 服务，负责：
- OTP/验证码生命周期管理（创建、验证、撤销）
- 风控与限流（用户限流、IP 限流、destination 限流、重发冷却）
- 审计日志记录
- Provider 插件化（SMS/Email）

**主要接口**:
- `POST /v1/otp/challenges`: 创建并发送验证码
- `POST /v1/otp/verifications`: 验证验证码
- `POST /v1/otp/challenges/{id}/revoke`: 撤销 challenge
- `GET /v1/test/code/{challenge_id}`: 测试端点，获取验证码（仅测试模式）
- `GET /metrics`: Prometheus 指标端点
- `GET /healthz`: 健康检查

**注意**: 测试模式下，Herald 启用了 `HERALD_TEST_MODE=true`，可以通过 `/v1/test/code/:challenge_id` 端点获取验证码，仅用于集成测试。

## 测试流程示例

### 完整登录流程

1. **发送验证码**:
   ```bash
   POST http://localhost:8080/_send_verify_code
   Content-Type: application/x-www-form-urlencoded
   phone=13800138000
   ```
   返回: `{"success": true, "challenge_id": "ch_xxx", "expires_in": 300}`

2. **获取验证码**（测试模式）:
   ```bash
   GET http://localhost:8082/v1/test/code/ch_xxx
   X-API-Key: test-herald-api-key
   ```
   返回: `{"ok": true, "code": "123456"}`

3. **登录**:
   ```bash
   POST http://localhost:8080/_login
   Content-Type: application/x-www-form-urlencoded
   auth_method=warden&phone=13800138000&challenge_id=ch_xxx&verify_code=123456
   ```
   返回: `Set-Cookie: stargate_session_id=xxx`

4. **验证授权**:
   ```bash
   GET http://localhost:8080/_auth
   Cookie: stargate_session_id=xxx
   ```
   返回: `200 OK` 并包含授权 Header:
   - `X-Auth-User: test-admin-001`
   - `X-Auth-Email: admin@example.com`
   - `X-Auth-Scopes: read,write,admin`
   - `X-Auth-Role: admin`

## 故障排查

### 服务无法启动

1. 检查端口是否被占用：
   ```bash
   lsof -i :8080 -i :8081 -i :8082 -i :6379
   ```

2. 查看服务日志：
   ```bash
   make logs
   # 或查看特定服务（使用与启动时相同的 compose 文件）
   docker compose -f build/image/docker-compose.yml logs stargate
   docker compose -f build/image/docker-compose.yml logs warden
   docker compose -f build/image/docker-compose.yml logs herald
   ```

3. 检查服务健康状态：
   ```bash
   make health
   ```

### 测试失败

1. **确保所有服务已就绪**:
   ```bash
   make ps
   make health
   ```

2. **检查服务健康状态**:
   ```bash
   curl http://localhost:8080/_auth
   curl http://localhost:8081/health
   curl http://localhost:8082/healthz
   ```

3. **查看测试详细输出**:
   ```bash
   go test -v ./e2e/...
   ```

4. **限流问题**:
   - 测试会自动清理限流状态，但如果测试运行过快，可能仍会触发限流
   - 可以增加测试间的延迟，或调整 Herald 的限流配置

5. **用户锁定问题**:
   - 测试会自动清理用户锁定状态
   - 如果仍遇到锁定问题，检查 Redis 连接和清理函数是否正常工作

### 验证码获取失败

确保 Herald 已启用测试模式（`HERALD_TEST_MODE=true`），检查 Herald 日志确认测试端点已启用：

```bash
docker compose -f build/image/docker-compose.yml logs herald | grep -i "test"
```

### Redis 连接问题

如果测试清理函数无法连接 Redis：

1. 确保 Herald Redis 端口已映射到 `localhost:6379`
2. 检查 Redis 是否正常运行：
   ```bash
   docker compose -f build/image/docker-compose.yml ps herald-redis
   redis-cli -h localhost -p 6379 ping
   ```

## 开发说明

### 修改测试数据

编辑 `fixtures/warden/data.json` 后，重启 Warden 服务：

```bash
make restart-warden
# 或（使用与启动时相同的 compose 文件）
docker compose -f build/image/docker-compose.yml restart warden
```

### 添加新测试

在 `e2e/` 目录下添加新的测试文件，遵循 Go 测试命名规范（`*_test.go`）。

**测试模板**:
```go
func TestMyNewTest(t *testing.T) {
    ensureServicesReady(t)  // 确保服务就绪并清理测试状态

    // 你的测试代码
}
```

### 测试辅助函数

`test_helpers.go` 提供了丰富的辅助函数：

- `ensureServicesReady(t)`: 确保服务就绪并清理测试状态
- `sendVerificationCodeWithError(t, phone)`: 发送验证码并返回错误响应
- `loginWithError(t, phone, challengeID, verifyCode)`: 登录并返回错误响应
- `checkAuthWithError(t, sessionCookie)`: 验证授权并返回错误响应
- `clearRateLimitKeys(t)`: 清理 Redis 中的测试状态（自动调用）
- `calculateHMAC(timestamp, service, body, secret)`: 计算 HMAC 签名

### 本地开发

如果需要修改服务代码，可以：

1. 使用 `build/build` 并从源码构建：
   ```bash
   make up-build
   # 或
   docker compose -f build/build/docker-compose.yml up -d --build
   ```
2. 修改对应服务源码后重新构建并重启：
   ```bash
   docker compose -f build/build/docker-compose.yml build stargate
   docker compose -f build/build/docker-compose.yml up -d
   # 或
   make restart
   ```

### 代码质量检查

项目使用 `golangci-lint` 进行代码质量检查：

```bash
golangci-lint run --max-same-issues=100000
```

## 测试覆盖率

当前测试套件覆盖：

- ✅ 正常登录流程
- ✅ 验证码错误场景（错误、过期、锁定）
- ✅ 用户状态验证（白名单、活跃状态）
- ✅ 限流场景（IP、用户、destination、重发冷却）
- ✅ 服务不可用场景
- ✅ 服务间鉴权（HMAC、API Key）
- ✅ 幂等性验证
- ✅ 审计日志记录
- ✅ Provider 错误处理
- ✅ 监控指标验证

## 参考文档

- [compose/README.zh-CN.md](./compose/README.zh-CN.md) — Compose 各子目录用法
- [config/README.zh-CN.md](./config/README.zh-CN.md) — CLI 预设与 compose 路径
- [compose/traefik/README.zh-CN.md](./compose/traefik/README.zh-CN.md) — Traefik 三合一/三分开
- [e2e/README.zh-CN.md](./e2e/README.zh-CN.md) — 端到端测试用例说明
- [MANUAL_TESTING.zh-CN.md](./MANUAL_TESTING.zh-CN.md) — 浏览器手动验证

## 许可证

与主项目保持一致。
