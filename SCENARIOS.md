English | [中文](SCENARIOS.zh-CN.md)

# Scenario Presets

This document maps to `config/scenarios.json`, used to generate compose files and `.env` by scenario.

## Usage

Scenario presets are currently selected through the **Web UI**. In the Web UI (`go run ./cmd/suite serve`), step 1 first selects a **deployment profile** (`development` / `test` / `production`), then a scenario preset (S1–S5); the generator fills options and env from the scenario, applies the profile's security & runtime policy, and produces compose. Download or copy the result in the review step. The CLI supports profiles and explicit mode selection, but it does not currently expose a `--scenario` option.

To generate the default mode set (image, build, traefik, etc.) without a scenario, run `make gen` (native CLI), or use `suite generate --profile <profile> --output <dir>`.

## Deployment profiles (policy, not just presets)

Profiles come from `config/profiles.yaml` and are applied by the shared `internal/policy` model (same for CLI and Web UI). Key differences:

| Policy | development | test | production |
|---|---|---|---|
| Port binding | loopback (`127.0.0.1`) | loopback (`127.0.0.1`) | reverse-proxy entry only (no host ports on internal services/Redis) |
| Secrets | auto-generate or input | isolated deterministic test values | must be user-provided / secret file |
| Password algorithm | plaintext allowed | test passwords | plaintext forbidden |
| Cookie Secure | optional | optional | required |
| HMAC v1 | forbidden | forbidden | forbidden |
| Validation | warning + error | strict | strict (errors are hard, never bypassable) |

`production` is experimental initially, but its strict rules are real errors: weak/test keys, plaintext passwords, published internal ports, Cookie Secure off, or HMAC v1 all block generation. Supply real secrets via process env or `--set KEY=VALUE`. For byte-stable dev/test output, pass `--seed <seed>` (dev/test only — never a real seed).

## Scenarios

| Scene ID | Name | Description | Best for |
|---|---|---|---|
| `s1-solo-gate` | S1 Solo Gate | Stargate-only local auth with minimum dependencies for quick startup. | Small internal or temporary environments |
| `s2-solo-gate-session-redis` | S2 Solo Gate + Session Redis | Stargate with Redis-backed sessions for multi-instance consistency. | Multi-replica Stargate and rolling upgrades |
| `s3-gate-warden` | S3 Gate + Warden | Use Warden for whitelist and identity source decoupling. | Unified identity source and account control |
| `s4-gate-warden-herald` | S4 Gate + Warden + Herald | Full OTP split architecture; Stargate focuses on session validation. | Recommended production architecture |
| `s5-gate-warden-herald-plugins` | S5 Gate + Warden + Herald Plugins | S4 plus SMTP/SMS/DingTalk/TOTP plugin capabilities. | Multi-channel notification and enterprise integrations |

## Notes

- `canonical` is the compose base template (`compose/canonical/docker-compose.yml`), not a selectable Web UI scenario preset.
- A scenario is composed of `modes + options + envOverrides`.
- `modes` controls which compose outputs are generated.
- `options` controls compose structure features (for example `includeSmtp`, `includeTotp`, `stargateSessionRedisUseBuiltin`).
- With `options.disableWardenRedisService=true`, generated `traefik` / `traefik-warden` compose excludes the `warden-redis` service.
- `envOverrides` writes default overrides into generated `.env`.
- Optional UI text fields: `nameZh/nameEn`, `descriptionZh/descriptionEn`, `riskNoteZh/riskNoteEn` for bilingual Web UI display.
