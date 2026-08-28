# Deployment

[English](#english) | [中文](#中文)

> Versions, ports, and health paths are derived from `config/components.yaml`
> (single source of truth). Do not hard-code them elsewhere.

---

## English

### Generate configuration

The CLI and the Web UI call the same `internal/composegen` / `internal/policy`
functions, so both produce identical output.

```bash
make gen                                   # native CLI → build/, no Web server
go run ./cmd/suite generate --profile development --output build/dev
go run ./cmd/suite generate --profile production  --output build/prod \
  --set PASSWORDS=bcrypt:... --set HERALD_HMAC_SECRET=... \
  --set HERALD_REDIS_PASSWORD=... --set WARDEN_REDIS_PASSWORD=...
```

### Profiles

`config/profiles.yaml` defines security & runtime policy (not just prefilled
forms): port binding, secret source, password algorithm, Herald auth, Redis
password, Cookie Secure, HMAC v1, container privileges, and validation mode.

- **development** — loopback ports, plaintext test password, dev keys. Use
  `--seed` for byte-stable dev keys (never a real seed).
- **test** — strict validation for CI.
- **production** — experimental & strict: plaintext passwords, placeholder keys,
  published internal ports, Cookie Secure off, or HMAC v1 are hard errors and
  can never be bypassed. Supply real secrets via env or repeated `--set`.

### Start / stop

```bash
make up            # or up-image / up-build / up-traefik
make ps            # status
make logs          # logs
make health        # probe health endpoints
make down          # stop (make clean also removes volumes)
```

Stargate has no host port; reach it through Traefik. In segmented/production
output the internal services (Warden, Herald, Redis) are not published.

### Container

```bash
docker build -t stargate-suite:local .
docker run --rm -p 127.0.0.1:8085:8085 stargate-suite:local
docker run --rm --read-only --tmpfs /tmp -p 127.0.0.1:8085:8085 stargate-suite:local
```

Config and canonical compose are embedded (`go:embed`), so the binary/container
runs without the repo source tree. Override with `--config-dir` / `CONFIG_DIR`.

### Post-deploy diagnostics

```bash
go run ./cmd/suite doctor --compose build/dev/docker-compose.yml --probe --json
```

---

## 中文

### 生成配置

CLI 与 Web UI 调用同一套 `internal/composegen` / `internal/policy` 函数，
输出完全一致。

```bash
make gen                                   # 原生 CLI → build/，不启 Web
go run ./cmd/suite generate --profile development --output build/dev
go run ./cmd/suite generate --profile production  --output build/prod \
  --set PASSWORDS=bcrypt:... --set HERALD_HMAC_SECRET=... \
  --set HERALD_REDIS_PASSWORD=... --set WARDEN_REDIS_PASSWORD=...
```

### Profile

`config/profiles.yaml` 定义的是安全与运行策略（不只是预填表单）：端口绑定、
机密来源、口令算法、Herald 鉴权、Redis 密码、Cookie Secure、HMAC v1、容器权限、
校验模式。

- **development**——loopback 端口、明文测试口令、开发密钥。用 `--seed` 得到
  字节稳定的开发密钥（绝非真实种子）。
- **test**——用于 CI 的 strict 校验。
- **production**——实验性且 strict：明文口令、占位密钥、暴露内部端口、
  Cookie Secure 关闭、HMAC v1 均为硬错误，不可绕过。真实机密经环境变量或多个
  `--set` 提供。

### 启停

```bash
make up            # 或 up-image / up-build / up-traefik
make ps            # 状态
make logs          # 日志
make health        # 探测健康端点
make down          # 停止（make clean 同时清卷）
```

Stargate 无宿主端口，经 Traefik 访问。分段/生产输出中，内部服务（Warden、
Herald、Redis）不对外发布端口。

### 容器

```bash
docker build -t stargate-suite:local .
docker run --rm -p 127.0.0.1:8085:8085 stargate-suite:local
docker run --rm --read-only --tmpfs /tmp -p 127.0.0.1:8085:8085 stargate-suite:local
```

配置与 canonical compose 已 `go:embed` 内嵌，二进制/容器无需仓库源码即可运行。
可用 `--config-dir` / `CONFIG_DIR` 覆盖。

### 部署后诊断

```bash
go run ./cmd/suite doctor --compose build/dev/docker-compose.yml --probe --json
```
