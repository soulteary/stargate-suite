# PR 9 · `test/v1-failure-contracts`

> Phase C · 安全与 v1 契约迁移 · 依赖 PR 8 · 可独立回滚

## 目标

补齐 v1 失败/重放/降级/故障恢复的契约测试，分离 liveness/readiness 断言，并增加生产配置拒绝测试。修复问题 **A-01** 的剩余部分（证明当前安全契约正确）。

## 允许修改文件

- `e2e/*.go`（新增 contract / failure / flows 测试）
- `e2e/fixtures/*`、`fixtures/*`
- `internal/policy/*_test.go`（production 拒绝测试）
- `scripts/run-e2e.sh`
- `e2e/README.md`、`e2e/README.zh-CN.md`

## 禁止修改

- 默认镜像版本与运行契约（已由 PR 8 固定）。
- 不得为让测试通过而降低 production 校验或重开 HMAC v1。
- 三个参考仓库。

## 实现要点（真实待改点，对照源计划 9.2/9.3）

**Stargate**：`/healthz` 在依赖故障时仍存活；`/readyz` 在 Redis/Warden/Herald 不可用时失败；callback host 不在 allowlist 返回 400；缺少/过短 `SESSION_EXCHANGE_SECRET` 生成或启动失败；session exchange ticket 单次使用、重放失败；非可信来源 `X-Forwarded-*` 被忽略；Step-up 缺可信 path provenance 失败；production plaintext 密码被拒绝。

**Warden**：API Key / HMAC v2 / mTLS 各自可独立启用；不完整认证配置启动失败；HMAC v2 key ID 被签名绑定；HMAC v1 默认关闭；旧协议迁移开关只在非 production Profile 允许；strict remote 刷新失败时 readiness 失败；`/metrics` 未授权访问被拒；health whitelist 与可信代理 CIDR 正确处理。

**Herald**：`REQUEST_AUTH_MODE=hmac_v2` 正常请求通过；错误 key ID / 错误签名 / 过期 timestamp / 重复 nonce 均失败；API Key 不绕过 HMAC v2；HMAC v1 默认关闭；多 key 且无 default/`X-Key-Id` 失败；独立 test listener 用专用 API Key、主监听器取不到测试码；production 拒绝 test mode / none auth / 短密钥 / 未确认无密码 Redis；审计字段含 request/service/key/provider/result 并按配置掩码 PII。

**端到端故障矩阵**：Stargate Redis / Warden / Herald / Herald Redis / Warden 远程源下线时的 liveness、readiness、登录行为与恢复要求（源计划 9.3 表），全部分离 liveness 与 readiness 断言。

## 验证命令

```bash
go test -race $(go list ./... | grep -v '/e2e$')
./scripts/run-e2e.sh            # 完整 v1 契约 + 故障恢复
go test -v -race -timeout 15m ./e2e/...
git diff --check
```

## 验收标准

- nonce 重放、key ID、timestamp、auth downgrade 测试通过；
- callback / trusted proxy / session exchange 测试通过；
- Redis/Warden/Herald 故障与恢复测试通过；
- liveness/readiness 断言分离；
- 生产配置拒绝测试通过。

## 回滚方式

`git revert` 本 PR（仅测试与 fixture，回滚不影响运行配置）。
