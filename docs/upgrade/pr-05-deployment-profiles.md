# PR 5 · `feat/deployment-profiles`

> Phase B · 建立单一契约来源 · production 初期可标 experimental · 可独立回滚

## 目标

引入 development / test / production 三类部署 Profile（安全与运行策略，非单纯预填表单），并让 CLI `generate --profile` 与 Web UI 共享同一模型。当前默认行为归入 development/test。

## 允许修改文件

- `config/profiles.yaml`（新增）
- `internal/policy/*.go`（新增：Profile 模型与策略应用）
- `internal/composegen/*.go`（按 Profile 生成）
- `cmd/suite/cmd_gen.go`、`cmd/suite/cmd_serve.go`（`--profile` 参数与 UI 首步选择 Profile）
- `cmd/suite/static/*`（UI 第一步选择 Profile）
- Golden test 与单元测试（新增）
- `README.md`、`README.zh-CN.md`、`SCENARIOS.md`、`SCENARIOS.zh-CN.md`

## 禁止修改

- 组件镜像版本（仍由 PR 4 清单登记的旧版本）。
- v1 端口/健康/HMAC 迁移（属于 PR 8）。
- 三个参考仓库。

## 实现要点（真实待改点）

1. **`config/profiles.yaml`** 定义三类 Profile，落实 [00-overview](00-overview.md) 5.4 表：端口绑定、密钥来源、密码算法、Herald 认证、Redis 密码、Cookie Secure、HMAC v1、容器权限、校验模式。
2. **把当前默认行为归入 development/test**：现状 `PASSWORDS=plaintext:test1234|test1337`、`API_KEY=test-*`、`exposePorts` 默认开启等，均属 development/test 语义。
3. **production 初期可标记 experimental**，但所有严格规则必须可测试：拒绝 plaintext、拒绝测试密钥、内部服务不发布端口、Cookie Secure 必须开启、HMAC v1 禁止。
4. **UI 第一步先选择 Profile**（改 `cmd/suite/static` 分步流程与 `cmd/suite/cmd_serve.go`）。
5. **CLI `generate --profile development|test|production`** 与 Web UI 共用 `internal/policy` + `internal/composegen`，不各自实现。
6. Golden test：每个 Profile 至少一个标准场景，生成两次字节稳定。

## 验证命令

```bash
go test -race $(go list ./... | grep -v '/e2e$')
go run ./cmd/suite generate --profile development --output build/dev
go run ./cmd/suite generate --profile production --output build/prod
docker compose -f build/dev/docker-compose.yml config
docker compose -f build/prod/docker-compose.yml config
go run ./cmd/suite validate --profile production --strict   # production 严格规则可触发
git diff --check
```

## 验收标准

- 三类 Profile 可生成并通过 `docker compose config`；
- production 的严格规则可被测试触发（弱密钥/明文/暴露端口被拒绝）；
- UI 第一步为 Profile 选择；
- CLI 与 UI 共用同一生成模型，结果一致；
- Golden test 稳定。

## 回滚方式

`git revert` 本 PR；回滚后回到无 Profile 的单一生成路径。
