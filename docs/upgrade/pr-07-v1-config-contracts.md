# PR 7 · `feat/v1-config-contracts`

> Phase C · 安全与 v1 契约迁移 · 仍可用旧镜像做静态生成测试 · 可独立回滚

## 目标

增加 Stargate v1、Warden v1.1、Herald v1.1 所需字段与跨字段/跨服务校验规则，实现 `validate --strict` 的结构化错误。修复问题 **V-01**、**A-02**。旧字段标 deprecated，不立即删除。

## 允许修改文件

- `config/env-meta.yaml`、`config/services.yaml`、`config/config-sections.yaml`、`config/schemas/*.yaml`（新增）
- `internal/policy/*.go`、`internal/contract/*.go`
- `cmd/suite/cmd_validate.go`
- 单元测试（校验规则）
- `config/README.md`、`config/README.zh-CN.md`

## 禁止修改

- 默认镜像版本（升级在 PR 8）。
- 端口/健康路径运行值（PR 8）。
- 三个参考仓库。

## 实现要点（真实待改点）

**Stargate v1 新增字段**（在 `config/env-meta.yaml` / `services.yaml` 暴露，参考现有 `HERALD_TLS_*`、`WARDEN_*` 结构）：`CALLBACK_ALLOWED_HOSTS`、`SESSION_EXCHANGE_SECRET`、`TRUSTED_PROXIES`、`PROXY_HEADER`、`COOKIE_SECURE`、`PASSWORD_HEADER_AUTH_ENABLED`、`WARDEN_HMAC_KEY_ID`、`WARDEN_HMAC_SECRET`、`WARDEN_TLS_CA_CERT_FILE`、`WARDEN_TLS_CLIENT_CERT_FILE`、`WARDEN_TLS_CLIENT_KEY_FILE`、`WARDEN_TLS_SERVER_NAME`、`HERALD_HMAC_KEY_ID`。

**Warden v1.1 新增**：显式 `ENVIRONMENT`、`WARDEN_HMAC_ALLOW_V1=false`、`WARDEN_METRICS_REQUIRE_AUTH=true`、`TRUSTED_PROXY_IPS`（已有）、`HEALTH_CHECK_IP_WHITELIST`（已有）。健康检查迁移到 `/healthcheck`（当前 canonical 用 `/health`，第 251 行）。

**Herald v1.1 新增**：显式 `ENVIRONMENT`、`REQUEST_AUTH_MODE`、`HERALD_HMAC_DEFAULT_KEY_ID`、`HMAC_MAX_DRIFT`、`HMAC_V1_ENABLED=false`、`HERALD_IDEMPOTENCY_SECRET`、`HERALD_PII_PEPPER`、`HERALD_TRUSTED_PROXIES`、`HERALD_TRUSTED_PROXY_HEADER`、`HERALD_TEST_API_KEY`、`HERALD_TEST_LISTENER_ADDR`。

**四层校验**（`cmd_validate.go` + `internal/policy`）：
1. 字段类型：端口、URL、布尔、持续时间、CIDR；
2. 单字段安全：密钥长度、禁止占位值（如 `test-*`、`test-hmac-secret`）、禁止 plaintext；
3. 跨字段：证书/私钥成对；`CALLBACK_ALLOWED_HOSTS` 或跨域 `COOKIE_DOMAIN` 生效时 `SESSION_EXCHANGE_SECRET` ≥ 32 随机字符；`STEP_UP_ENABLED=true` 时 `STEP_UP_PATHS` 与 `TRUSTED_PROXIES` 非空；production `COOKIE_SECURE=true`、`PASSWORDS` 不以 `plaintext:` 开头；
4. 跨服务：Stargate 调用端认证配置与 Herald/Warden 服务端认证模式一致（三选一显式，不靠同时填多个字段推断）。

结构化错误示例：

```json
{
  "code": "STARGATE_SESSION_EXCHANGE_SECRET_REQUIRED",
  "severity": "error",
  "field": "SESSION_EXCHANGE_SECRET",
  "profile": "production",
  "message": "cross-domain callback requires a secret of at least 32 characters"
}
```

production 的 error 不能降级为 warning；strict 模式下未知环境变量必须失败。旧字段标 deprecated 保留解析。

## 验证命令

```bash
go test -race $(go list ./... | grep -v '/e2e$')
go run ./cmd/suite validate --profile production --strict
go vet ./...
git diff --check
```

## 验收标准

- v1 字段在 UI/CLI 可见；
- 四层校验可用，production error 不降级；
- strict 模式拒绝未知变量与弱/占位密钥；
- 旧字段 deprecated 但仍可解析；
- 仍可用旧镜像做静态生成与校验测试。

## 回滚方式

`git revert` 本 PR；未改默认镜像，回滚不影响运行组合。
