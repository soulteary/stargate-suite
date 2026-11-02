# Provider and test inbox upgrades / Provider 与测试收件箱升级

## Current pins

The component manifest and immutable image lock now select Herald DingTalk, SMTP and TOTP v1.2.6 and OwlMail v0.13.0. `suite generate --lock config/components.lock.yaml` uses exact tested registry digests. Stargate v1.2.1, Warden v1.4.2 and Herald v1.3.2 are unchanged; this update does not release any of those projects or change the Suite's own version.

## Provider patches

The v1.2.5 → v1.2.6 patches retain provider v1 HTTP routes, API-key/HMAC authentication, environment names and Redis credential formats. Preserve credentials, the TOTP encryption key and Redis data. No data migration is needed. Validate health and a representative authenticated request before restoring traffic; use isolated destinations for test SMTP/DingTalk sends.

## OwlMail 0.11.2 → 0.13.0

Read the upstream [0.12.0 notes](https://github.com/soulteary/owlmail/blob/v0.12.0/docs/en/Release-0.12.0.md) and [0.13.0 upgrade/rollback procedure](https://github.com/soulteary/owlmail/blob/v0.13.0/docs/en/Release-0.13.0.md#upgrade-compatibility-and-rollback). The default Suite inbox remains an isolated SMTP capture service, not a production mail relay. The raw optional-service generator publishes SMTP/HTTP ports on the host; before using it, restrict those mappings to loopback or a protected test network. Review the selected profile and rendered Compose rather than assuming a production isolation policy.

- Before replacing an existing standalone inbox, stop incoming SMTP and the old process; back up its complete mail directory, Relay records, metadata, EML/attachments and outbox. Back up external Redis/S3 state if configured and record the old digest/configuration.
- Rehearse against an isolated copy with real outbound destinations disabled. Recovery can resume pending Relay tasks, including SMTP-only mode. Delivery is still at least once, not exactly once: a crash after downstream acceptance can duplicate delivery.
- Check readiness, retained messages and attachments, new SMTP capture, native/MailDev/MCP read contracts, and restart persistence. Do not delete recovery records merely to force readiness.
- Rollback requires stopping the new process and restoring the complete pre-upgrade snapshot/external state with the old binary or image. Do not share writable state across versions. Reconcile downstream receipts before enabling outbound delivery; preserve new captures separately before restoring a snapshot.
- Updating the default image does not erase data. Generated Suite Compose does not mount an OwlMail data volume by default; inbox contents in that container remain ephemeral across recreation. Add and back up a deliberate persistent mail-directory mount if retention is required; review the upstream paths and ownership first.

## 中文

本次清单与不可变摘要锁更新为三个 provider v1.2.6 和 OwlMail v0.13.0。核心 Stargate v1.2.1、Warden v1.4.2、Herald v1.3.2 及本 Suite 自身版本保持不变，不创建任何新发布。

Provider 从 v1.2.5 升级保留接口、认证、配置和 Redis 数据格式；保留凭据、TOTP 加密密钥与 Redis 数据，无需迁移。恢复流量前检查健康和认证请求，邮件/钉钉演练使用隔离目标。

OwlMail 从 0.11.2 跨次版本升级，先阅读上游 0.12.0/0.13.0 说明。停止旧实例并备份完整邮件目录（Relay、元数据、EML/附件、outbox）及外部 Redis/S3；记录旧摘要和配置。在隔离副本、禁用真实出站地址的情况下演练，恢复可能继续待发送任务，且仍是至少一次投递。检查就绪、历史邮件/附件、新 SMTP 捕获、API/MailDev/MCP 读取和重启持久化。

回退时停止新版并恢复升级前完整快照，不要让两个版本共用可写状态；恢复快照可能重新引入待发送任务，重新启用出站前核对下游记录，快照后的新邮件需另行保留。Suite 默认生成的 OwlMail 容器未挂载邮件持久卷，重建会丢弃容器内测试邮件；需要保留时须显式设置、备份持久邮件目录，并核对权限。
