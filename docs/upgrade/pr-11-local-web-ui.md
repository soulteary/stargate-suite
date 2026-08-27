# PR 11 · `security/local-web-ui`

> Phase D · 工具与交付质量 · 可独立回滚

## 目标

收紧本地 Web UI 安全边界：默认 loopback 监听、显式 remote 开关、access token、Origin/CSRF 校验、server 超时、secret 掩码与会话清理。

## 允许修改文件

- `cmd/suite/cmd_serve.go`、`cmd/suite/session.go`、`cmd/suite/main.go`
- `cmd/suite/static/*`
- handler 单元测试
- `README.md`、`README.zh-CN.md`

## 禁止修改

- 生成结果语义（须与 CLI 一致）。
- 组件版本与运行契约。
- 三个参考仓库。

## 实现要点（真实待改点）

1. **默认监听 `127.0.0.1`**（`cmd/suite/cmd_serve.go`，默认端口 8085 见 `Makefile` 第 107-108 行、`Dockerfile` 第 21 行 `EXPOSE 8085`）。
2. **不再自动搜索并绑定其他端口而用户无感知**：端口被占用时明确输出提示，不静默换端口。
3. **非 loopback 监听必须显式 `--allow-remote`** 并配置 access token。
4. **POST 操作增加 Origin/CSRF 校验**。
5. 设置安全 cookie、合理 SameSite、请求体大小限制、server 读写超时。
6. **review 页面显示** Profile、组件版本、未验证覆盖项与安全校验结果；默认只显示 secret 掩码与来源，不回显完整 secret。
7. Web UI 不在服务器持久化用户密钥，生成后即返回文件并清理会话（结合 `cmd/suite/session.go`）。导出 `.env` 权限建议 `0600`；日志/错误/E2E 失败输出不得打印完整 `.env`。

## 验证命令

```bash
go test -race $(go list ./... | grep -v '/e2e$')
go run ./cmd/suite serve            # 默认应仅监听 127.0.0.1:8085
# 远程需显式：go run ./cmd/suite serve --listen 0.0.0.0:8085 --allow-remote
git diff --check
```

## 验收标准

- 默认 loopback 监听；非 loopback 需 `--allow-remote` + token；
- POST 具备 Origin/CSRF 校验；
- secret 掩码、会话清理生效，日志不泄露 secret；
- handler 单元测试通过。

## 回滚方式

`git revert` 本 PR。
