# Provider and test inbox upgrades / Provider 与测试收件箱升级

## Current pins

The component manifest and immutable image lock select the published releases below. `suite generate --lock config/components.lock.yaml` uses their registry digests. The full E2E workflow validates the selected combination before this Suite can be released.

| Component | Previous default | Current default |
| --- | --- | --- |
| Stargate | 1.2.1 | 1.8.0 |
| Warden | 1.4.2 | 1.9.1 |
| Herald | 1.3.2 | 1.9.0 |
| Herald TOTP | 1.2.6 | 2.0.4 |
| Herald DingTalk | 1.2.6 | 1.2.8 |
| Herald SMTP | 1.2.6 | 1.2.9 |
| OwlMail | 0.13.0 | 0.15.0 |
| Redis | 8.4-alpine | 8.10.2-alpine |
| whoami | v1.11 | v1.12.0 |

## TOTP v2 and core services

Herald owns the TOTP backend URL and credentials. The generated Compose passes `HERALD_TOTP_ENABLED` to both Stargate and Herald, and sends `HERALD_TOTP_BASE_URL`, `HERALD_TOTP_API_KEY`, and `HERALD_TOTP_HMAC_SECRET` to Herald. Stargate calls Herald's proxy. The TOTP image health probe uses the BusyBox `wget` present in that image.

Read the upstream [TOTP v2 migration guide](https://github.com/soulteary/herald-totp/blob/v2.0.4/docs/enUS/MIGRATION_V2.md). HTTP routes retain `/v1`; HMAC calls require operation-bound v2 signatures and one-use nonces. Coordinate Herald and TOTP service replacement, retain Redis records and the original encryption key, and pause in-flight enrollment or verification while changing HMAC callers. Legacy HMAC callers cannot be mixed with the v2 backend. API-key-only clients remain supported; configure matching nonempty credentials and JSON requests. Do not flush replay or quota records to bypass failures.

Record and back up each service's existing configuration and state before replacing an existing deployment. These defaults select new published components; they do not migrate running containers or persisted data. Follow the selected release's state and namespace migration instructions before restoring traffic.

## Earlier provider patch upgrade

The v1.2.5 → v1.2.6 patches retain provider v1 HTTP routes, API-key/HMAC authentication, environment names and Redis credential formats. Preserve credentials, the TOTP encryption key and Redis data. No data migration is needed. Validate health and a representative authenticated request before restoring traffic; use isolated destinations for test SMTP/DingTalk sends.

## OwlMail state and delivery

For the new default, also read the [0.15.0 release and rollback notes](https://github.com/soulteary/owlmail/blob/v0.15.0/docs/en/Release-0.15.0.md). Read the upstream [0.12.0 notes](https://github.com/soulteary/owlmail/blob/v0.12.0/docs/en/Release-0.12.0.md) and [0.13.0 upgrade/rollback procedure](https://github.com/soulteary/owlmail/blob/v0.13.0/docs/en/Release-0.13.0.md#upgrade-compatibility-and-rollback). The default Suite inbox remains an isolated SMTP capture service, not a production mail relay. The raw optional-service generator publishes SMTP/HTTP ports on the host; before using it, restrict those mappings to loopback or a protected test network. Review the selected profile and rendered Compose rather than assuming a production isolation policy.

- Before replacing an existing standalone inbox, stop incoming SMTP and the old process; back up its complete mail directory, Relay records, metadata, EML/attachments and outbox. Back up external Redis/S3 state if configured and record the old digest/configuration.
- Rehearse against an isolated copy with real outbound destinations disabled. Recovery can resume pending Relay tasks, including SMTP-only mode. Delivery is still at least once, not exactly once: a crash after downstream acceptance can duplicate delivery.
- Check readiness, retained messages and attachments, new SMTP capture, native/MailDev/MCP read contracts, and restart persistence. Do not delete recovery records merely to force readiness.
- Rollback requires stopping the new process and restoring the complete pre-upgrade snapshot/external state with the old binary or image. Do not share writable state across versions. Reconcile downstream receipts before enabling outbound delivery; preserve new captures separately before restoring a snapshot.
- Updating the default image does not erase data. Generated Suite Compose does not mount an OwlMail data volume by default; inbox contents in that container remain ephemeral across recreation. Add and back up a deliberate persistent mail-directory mount if retention is required; review the upstream paths and ownership first.

## 中文

本次清单与不可变摘要锁采用上表所列的公开版本。TOTP 升为 v2.0.4，Herald v1.9.0 负责后端地址与凭据，Stargate v1.8.0 通过 Herald 代理调用。完整 E2E 工作流必须验证该组合后才能发布 Suite。

升级 TOTP 前阅读上游 v2 迁移说明，协调 Herald 与 TOTP 的 HMAC v2 签名和一次性 nonce；保留 Redis 状态、原始加密密钥、配额与重放记录，不混用旧签名客户端。HTTP 路径仍为 /v1，API-key 模式仍受支持。生成器仅将启用开关传给 Stargate，将后端地址和凭据传给 Herald；健康检查使用镜像已有的 wget。此清单更新不会替换正在运行的容器或迁移数据，恢复流量前须执行所选版本的状态与命名空间迁移检查。

Provider 从 v1.2.5 升级保留接口、认证、配置和 Redis 数据格式；保留凭据、TOTP 加密密钥与 Redis 数据，无需迁移。恢复流量前检查健康和认证请求，邮件/钉钉演练使用隔离目标。

OwlMail 当前默认升到 0.15.0，先阅读上游 0.15.0 发布与回退说明；从更早版本迁移还需参照 0.12.0/0.13.0 说明。停止旧实例并备份完整邮件目录（Relay、元数据、EML/附件、outbox）及外部 Redis/S3；记录旧摘要和配置。在隔离副本、禁用真实出站地址的情况下演练，恢复可能继续待发送任务，且仍是至少一次投递。检查就绪、历史邮件/附件、新 SMTP 捕获、API/MailDev/MCP 读取和重启持久化。

回退时停止新版并恢复升级前完整快照，不要让两个版本共用可写状态；恢复快照可能重新引入待发送任务，重新启用出站前核对下游记录，快照后的新邮件需另行保留。Suite 默认生成的 OwlMail 容器未挂载邮件持久卷，重建会丢弃容器内测试邮件；需要保留时须显式设置、备份持久邮件目录，并核对权限。
