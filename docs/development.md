# Development

[English](#english) | [中文](#中文)

---

## English

### Prerequisites

- Go 1.27+ (the `go.mod` `go` directive is authoritative; CI reads it via
  `go-version-file: go.mod`).
- Docker & Compose for E2E.

### Build & CLI

```bash
go build ./...
go run ./cmd/suite help          # generate, validate, doctor, serve, version
go run ./cmd/suite version       # shows injected version/commit/date + verified combo
```

`generate` / `validate` / `doctor` all support `--json` and stable exit codes
(0 ok, non-zero on validation failure or doctor hard failure). The CLI and the
Web UI `/api/generate` share the same generation/validation functions.

### Config as code

`config/components.yaml` is the single source of truth for versions, images,
ports, and health paths. A drift test asserts that other config surfaces derive
from it — do not hard-code these values elsewhere.

### Tests

```bash
go test ./... -short                    # unit (excludes heavy E2E)
./scripts/run-e2e.sh                    # full E2E (needs Docker)
make test-wait && go test -v ./e2e/...  # E2E after services are ready
go test -v ./e2e/... -run TestCompleteLoginFlow
```

Test data lives in `fixtures/warden/data.json`; after edits run
`make restart-warden`. New E2E tests go under `e2e/` and use
`ensureServicesReady(t)` + `test_helpers.go`.

### Lint

```bash
gofmt -l $(git ls-files '*.go')
go vet ./...
golangci-lint run --max-same-issues=100000
```

### CI tiers

- `ci.yml` — fast PR feedback (lint, unit, config, path-filtered smoke E2E).
- `main.yml` — full E2E, image build + Trivy, cross-OS build matrix.
- `nightly.yml` — multi-arch image build, cross-OS CLI smoke.

---

## 中文

### 前置要求

- Go 1.27+（以 `go.mod` 的 `go` 指令为准；CI 通过 `go-version-file: go.mod` 读取）。
- E2E 需要 Docker 与 Compose。

### 构建与 CLI

```bash
go build ./...
go run ./cmd/suite help          # generate、validate、doctor、serve、version
go run ./cmd/suite version       # 显示注入的版本/commit/时间 + 已验证组合
```

`generate` / `validate` / `doctor` 均支持 `--json` 与稳定退出码
（0 成功，校验失败或 doctor 硬失败为非零）。CLI 与 Web UI `/api/generate` 共用同一套
生成/校验函数。

### 配置即代码

`config/components.yaml` 是版本、镜像、端口、健康路径的唯一权威来源。漂移测试会
断言其他配置面均由其派生——请勿在别处硬编码这些值。

### 测试

```bash
go test ./... -short                    # 单元（排除重型 E2E）
./scripts/run-e2e.sh                    # 完整 E2E（需 Docker）
make test-wait && go test -v ./e2e/...  # 服务就绪后跑 E2E
go test -v ./e2e/... -run TestCompleteLoginFlow
```

测试数据在 `fixtures/warden/data.json`，改后执行 `make restart-warden`。新增 E2E
测试放在 `e2e/` 下，使用 `ensureServicesReady(t)` + `test_helpers.go`。

### 代码检查

```bash
gofmt -l $(git ls-files '*.go')
go vet ./...
golangci-lint run --max-same-issues=100000
```

### CI 分层

- `ci.yml`——PR 快速反馈（lint、单元、配置、按路径过滤的 smoke E2E）。
- `main.yml`——完整 E2E、镜像构建 + Trivy、跨 OS 构建矩阵。
- `nightly.yml`——多架构镜像构建、跨 OS CLI 冒烟。
