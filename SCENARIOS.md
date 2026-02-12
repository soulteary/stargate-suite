English | [中文](SCENARIOS.zh-CN.md)

# Scenario Presets

This document maps to `config/scenarios.json`, used to generate compose files and `.env` by scenario.

## Usage

Scenario-based generation is **Web UI only**. In the Web UI (`go run ./cmd/suite serve`), choose a scenario preset (S1–S5) in step 1; the generator fills options and env from that scenario and produces compose. Download or copy the result in the review step.

To generate the default mode set (image, build, traefik, etc.) without a scenario, run `make gen` (via Web API).

There is no CLI `gen "scene:<id>"` or `make gen-scenarios`; scenario output is produced only through the Web UI.

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
