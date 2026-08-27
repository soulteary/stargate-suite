# Cursor 每 PR 标准提示词与通用验证命令

> 对应源计划 12.3（标准提示词）与 12.5（通用验证命令）。每个 PR 会话复制下方模板，替换 `<...>` 占位并粘贴对应工作包的验证命令。

## 每个 PR 的标准提示词

```text
你正在改造 soulteary/stargate-suite。

当前工作包：<PR 名称与目标，如 PR 1 · fix/restore-go127-ci>
允许修改：<文件或目录，见对应 docs/upgrade/pr-XX.md>
禁止修改：三个参考仓库、无关前端样式、未列入范围的配置语义。

请先：
1. 阅读 AGENTS.md、现有测试和相关实现；
2. 对照指定的 Stargate/Warden/Herald 稳定 Tag 契约；
3. 列出准备修改的文件、当前问题和最小实现方案；
4. 确认没有与工作区已有变更冲突。

实施要求：
- 保持 PR 单一主题；
- 不降低 production 安全校验；
- 不输出或提交真实密钥；
- 新增或修改行为必须有测试；
- 同步必要的中英文文档；
- 保留向后迁移提示，但不要默认启用旧 HMAC。

验证：
<粘贴本工作包验证命令>

完成后只输出：
- 变更摘要；
- 修改文件清单；
- 测试命令与结果；
- 尚未覆盖的风险；
- 建议的 PR 标题和正文。

不要自行 commit、push、merge 或创建 Release。
```

## 通用验证命令

```bash
go mod download
go fmt ./...
go vet ./...
go test -race ./...
go run ./cmd/suite validate --strict
docker compose -f build/test/docker-compose.yml config
docker build -t stargate-suite:local .
./scripts/run-e2e.sh
git diff --check
```

**按 PR 范围裁剪命令**：

- PR 1 不应因完整旧 E2E 尚未迁移而被阻塞，但必须保证非 E2E 测试与三平台构建通过；用 `go test -race $(go list ./... | grep -v '/e2e$')` 排除 E2E。
- 从 PR 8 起，完整核心 E2E（`./scripts/run-e2e.sh`）必须通过。
- 文档-only PR（如 PR 14）只需文档链接/示例版本核对与 `git diff --check`。

## 单个工作包执行循环（源计划 12.4）

1. 创建独立分支；
2. 读取相关代码与上游稳定 Tag；
3. 输出最小变更设计；
4. 先补失败测试或契约测试；
5. 实现代码；
6. 运行格式、单元测试和当前工作包专项验证；
7. 检查 `git diff --check`；
8. 检查是否出现 secret、生成目录或无关格式化；
9. 人工审阅；
10. 再由维护者决定 commit、push 和创建 PR。
