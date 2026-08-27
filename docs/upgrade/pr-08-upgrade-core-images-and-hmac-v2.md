# PR 8 · `feat/upgrade-core-images-and-hmac-v2`

> Phase C · **本轮最关键的原子 PR** · 作为**单个回滚单元** · 修复 C-01、A-01

## 目标

一次性、原子地完成三镜像 v1 升级及其连带的端口、健康路径、认证与 E2E 迁移。**禁止**把「镜像升级」和「HMAC 测试迁移」拆成两个可独立合并的 PR（否则任一半合并都会破坏运行/测试）。

## 允许修改文件

- `config/components.yaml`（版本改为 v1）
- `config/env-meta.yaml`、`.env.example`、`config/ports.yaml`
- `compose/canonical/docker-compose.yml`、`compose/traefik/*`
- `build/*/docker-compose.yml`、`build/*/.env`（重新生成）
- `internal/composegen/*.go`、`internal/policy/*.go`
- `e2e/*.go`（HMAC v2 签名、健康路径、独立 test listener）
- `Makefile`（`make health` 端口/路径）
- `.github/workflows/ci.yml`（E2E 探测端口/路径）
- 中英文迁移文档

## 禁止修改

- 不得为让测试通过而重新启用 HMAC v1 或降低 production 校验。
- 三个参考仓库（仅作为只读契约来源）。

## 实现要点（真实待改点，必须同一 PR 完成）

1. **默认版本升级**（改 `config/components.yaml` 与由清单驱动的所有出口）：Stargate `v0.11.0`→`1.0.0`、Warden `v0.13.0`→`1.1.0`、Herald `v0.9.0`→`1.1.0`。同步修正**已漂移的 `build/*`**（当前仍为 Stargate `v0.9.2` / Warden `v0.10.0` / Herald `v0.6.1`、`herald-dingtalk:latest`）。
2. **Stargate 端口 80→8080**：`compose/canonical` 第 279 行 `ports`、第 348 行 Traefik `loadbalancer.server.port=80`、第 349 行 `forwardauth.address=http://stargate/_auth`、`config/ports.yaml` 第 7-8 行 `containerPort/defaultHostPort: "80"`。
3. **健康/readiness 路径迁移**：Stargate liveness `/health`→`/healthz`，新增 readiness `/readyz`（canonical 第 354 行 healthcheck）；Warden `/health`→`/healthcheck`（canonical 第 251 行）；Herald 保持 `/healthz`。
4. **`Makefile` `make health`（第 94、96、98 行）**：Stargate 由 `http://localhost:80/health` 改为 `:8080/healthz` 或 `/readyz`；Warden 改 `/healthcheck`。
5. **`.github/workflows/ci.yml` 第 111 行** E2E 探测 `"8080/health" "8081/health" "8082/healthz"` 同步为新端口/路径（Stargate `8080/healthz`、Warden `8081/healthcheck`、Herald `8082/healthz`）。
6. **Herald 显式认证**：设置 `REQUEST_AUTH_MODE=hmac_v2`、`ENVIRONMENT`；不再同时依赖 `API_KEY` 与 `HMAC_SECRET` 隐式选择（canonical 第 27-28 行）。
7. **Warden/Herald HMAC v2 配置**：key ID 绑定、多 key 显式 default 或 `X-Key-Id`。
8. **E2E 签名改为 v2**（`e2e/herald_api_test.go`、`e2e/test_helpers.go`）：覆盖 method、path、body hash、timestamp、nonce、service、key ID；如自实现签名须用上游测试向量交叉验证。
9. **Herald 测试验证码改为独立监听器**（`HERALD_TEST_LISTENER_ADDR` + `HERALD_TEST_API_KEY`），主监听器无法获取测试码。
10. **旧 HMAC v1 默认关闭**（`HMAC_V1_ENABLED=false` / `WARDEN_HMAC_ALLOW_V1=false`）。
11. 正常登录 smoke E2E 通过。

## 验证命令

```bash
go test -race $(go list ./... | grep -v '/e2e$')
go run ./cmd/suite generate --profile test --output build/test
docker compose -f build/test/docker-compose.yml config
./scripts/run-e2e.sh          # 正常登录 + 健康检查 smoke E2E 必须通过
git diff --check
```

## 验收标准

- 默认组件组合为 Stargate 1.0.0 / Warden 1.1.0 / Herald 1.1.0；
- Stargate 端口 8080、liveness `/healthz`、readiness `/readyz`；Warden `/healthcheck`；
- Herald 显式 `REQUEST_AUTH_MODE=hmac_v2`，HMAC v2 覆盖全部字段；
- E2E 使用 v2 签名并通过；测试码走独立监听器；
- HMAC v1 默认关闭；
- 正常登录 smoke E2E 通过；
- `ci.yml` 与 `make health` 探测已同步新端口/路径。

## 回滚方式

**整体作为单个原子回滚单元** `git revert`——镜像、配置、Compose、健康检查、E2E 与 Golden 文件必须同 PR 一起回滚，不得部分回滚。
