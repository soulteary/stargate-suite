# PR 6 · `security/compose-network-and-redis`

> Phase C · 安全与 v1 契约迁移 · 仍可用旧镜像 · 可独立回滚

## 目标

拆分内部网络、默认关闭内部服务的宿主机端口、闭环 Redis 服务端/客户端/健康检查密码、为适用服务加最小权限。修复问题 **S-01**（Redis 密码未联动）与 **S-03**（内部服务默认暴露）。

## 允许修改文件

- `compose/canonical/docker-compose.yml`
- `compose/traefik/*`、`config/ports.yaml`
- `internal/composegen/*.go`、`internal/policy/*.go`
- Golden test（三种 Profile）
- `compose/README.md`、`compose/README.zh-CN.md`

## 禁止修改

- 组件镜像版本（仍旧版本；升级在 PR 8）。
- Stargate 端口 80→8080、健康路径迁移（属于 PR 8）。
- 三个参考仓库。

## 实现要点（真实待改点）

1. **网络拆分**：当前 canonical 仅 `the-gate-network` + external `traefik`（第 389-393 行）。按目标拆为 `edge` / `auth-internal` / `warden-data` / `herald-data`，可选通道服务只加入其所需内部网络。
2. **内部服务默认不发布宿主机端口**：当前 canonical 中 `herald` `8082:8082`（第 19-20 行）、`herald-redis` `6379:6379`（第 100-101 行）、`herald-totp` `8084:8084`、`herald-dingtalk` `8083:8083`、`herald-smtp` `8085:8085`、`warden` `8081:8081` 均全接口发布。production 改为 `expose` 或不声明；development/test 需宿主机访问时仅 `127.0.0.1:HOST:CONTAINER`。同步调整 `config/ports.yaml` 的 `exposePorts` 默认与绑定地址（问题 S-03）。
3. **Redis 密码闭环**（问题 S-01）：当前 `herald-redis`/`warden-redis` 未设 `--requirepass`，而客户端 `REDIS_PASSWORD` 可留空。改为服务端 `--requirepass ${..._REDIS_PASSWORD:?...}` + 客户端同源密码 + healthcheck 用密码（且不把密码写入日志）：

```yaml
command:
  - redis-server
  - --requirepass
  - ${HERALD_REDIS_PASSWORD:?HERALD_REDIS_PASSWORD is required}
healthcheck:
  test:
    - CMD-SHELL
    - redis-cli -a "$${HERALD_REDIS_PASSWORD}" ping | grep PONG
```

4. Herald 与 Warden 使用独立密码与独立 Redis；production 用 required interpolation 或 secret file；development/test 可自动生成但不得提交 Git。Redis 端口默认不映射宿主机。
5. **容器最小权限**（适用服务）：`read_only: true` + `tmpfs: [/tmp]` + `cap_drop: [ALL]` + `security_opt: [no-new-privileges:true]`；Redis 数据保留具名 volume（当前 canonical 用 `herald-redis-data`/`warden-redis-data`，注意 `build/*` 用 bind mount `./data/...`）。
6. Golden test 覆盖三种 Profile 的网络/端口/Redis 密码差异。

## 验证命令

```bash
go test -race $(go list ./... | grep -v '/e2e$')
HERALD_REDIS_PASSWORD=devpass WARDEN_REDIS_PASSWORD=devpass \
  docker compose -f build/dev/docker-compose.yml config
docker compose -f build/prod/docker-compose.yml config   # 缺密码应报 required 错误
git diff --check
```

## 验收标准

- 内部服务在 production 不发布宿主机端口，development/test 仅绑 loopback；
- Redis 服务端/客户端/健康检查密码一致，缺失时 production 生成失败；
- 适用服务具备只读根文件系统与 cap drop；
- 三 Profile Golden test 通过。

## 回滚方式

`git revert` 本 PR；Golden 文件与生成器代码同 PR 回滚。
