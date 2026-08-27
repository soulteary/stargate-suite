# PR 10 · `feat/cli-generate-validate-doctor`

> Phase D · 工具与交付质量 · 可独立回滚

## 目标

让 CLI 具备原生 `generate`、strict `validate`、`doctor`，支持 `--json` 与稳定退出码，并把 `make gen` 改为调用 CLI，消除「README 宣称 CLI，实则只能走 Web API」的歧义。

## 允许修改文件

- `cmd/suite/cmd_gen.go`、`cmd/suite/cmd_validate.go`、`cmd/suite/main.go`
- `cmd/suite/cmd_doctor.go`（新增）
- `internal/doctor/*.go`（新增）
- `internal/composegen/*.go`、`internal/policy/*.go`
- `Makefile`（`gen` 改调 CLI）
- `scripts/gen-via-api.sh`（废弃或改为薄封装）
- 单元测试
- `README.md`、`README.zh-CN.md`

## 禁止修改

- 组件版本与运行契约。
- Web UI 生成结果语义（须与 CLI 一致）。
- 三个参考仓库。

## 实现要点（真实待改点）

1. **`make gen`（Makefile 第 16-17 行）当前调用 `./scripts/gen-via-api.sh`**（依赖启动临时 Web 服务 + `jq`，见 `ci.yml` 第 91-95 行）。改为 `go run ./cmd/suite generate ...`，不再启动 Web 服务完成生成。
2. **`generate` 与 Web `/api/generate` 调用同一 Go 函数**（`internal/composegen`），CLI 可完全脱离 Web UI 工作。
3. **`validate --strict`** 返回稳定非零退出码与结构化错误编号（承接 PR 7 的四层校验）。
4. **`doctor`** 只读诊断：Compose 语法、镜像信息、端口占用、网络、健康与依赖连通性。
5. **所有命令支持 `--json`**，便于 CI 与 Cursor 解析。
6. 稳定命令形如：

```text
stargate-suite version
stargate-suite generate --profile development --output build/dev
stargate-suite generate --profile production --config suite.yaml --output build/prod
stargate-suite validate --profile production --config suite.yaml --strict
stargate-suite doctor --compose build/prod/docker-compose.yml
stargate-suite serve --listen 127.0.0.1:8085
```

## 验证命令

```bash
go test -race $(go list ./... | grep -v '/e2e$')
go run ./cmd/suite generate --profile development --output build/dev
go run ./cmd/suite validate --profile production --strict --json
go run ./cmd/suite doctor --compose build/dev/docker-compose.yml --json
make gen                        # 不再启动 Web 服务
git diff --check
```

## 验收标准

- CLI 原生 generate/validate/doctor 可用，`--json` 输出稳定；
- `validate` 退出码稳定、错误结构化；
- `make gen` 不再依赖 Web API 与 `jq`；
- CLI 与 Web UI 对同一输入生成一致结果。

## 回滚方式

`git revert` 本 PR；如需保留旧 `make gen`，回滚会恢复 `gen-via-api.sh` 路径。
