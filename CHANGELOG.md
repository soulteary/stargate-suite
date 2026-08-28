# Changelog

All notable changes to **stargate-suite** are documented here. The format is
based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this
project aims to follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Component versions (Stargate / Warden / Herald and channels) are pinned in
`config/components.yaml`, which is the single source of truth — see that file
for the exact image tags shipped in each release.

## [Unreleased]

## [0.10.0]

### Added

- Native CLI `generate` / `validate` / `doctor` commands with `--json` output
  and stable exit codes; the CLI and the Web UI `/api/generate` endpoint call
  the same Go generation and validation functions.
- Deployment profiles (`config/profiles.yaml`) with `development` /
  `production` policy validation.
- Component manifest (`config/components.yaml` + `components.lock.yaml`) as the
  single authoritative source for versions, images, container ports, and health
  paths.
- Tiered CI workflows: fast PR feedback (`ci.yml`), full `main` validation
  (`main.yml`), and broad `nightly.yml` coverage, with path filters and
  concurrency cancellation.
- Supply-chain hardening in `release.yml`: all third-party actions pinned to
  commit SHAs, minimal top-level `permissions: contents: read`, SBOM generation,
  Trivy image scan, GitHub artifact attestations, and keyless Cosign signatures
  for the image and the checksums manifest.

### Changed

- **Breaking:** atomic upgrade to the v1 contracts — core images bumped to
  Stargate `v1.0.0`, Warden `v1.0.0`, Herald `v1.1.0`; container ports moved
  from `80` to `8080`; health probes use `/healthz` + `/readyz`; Herald now
  requires explicit request auth (HMAC v2) with HMAC v1 disabled by default.
- Web UI now binds to loopback by default; non-loopback listening requires an
  explicit `--allow-remote` plus an access token, with Origin/CSRF validation on
  state-changing requests, hardened cookies, and server read/write/idle timeouts.
- `make gen` now invokes the native CLI (`suite generate --canonical`) instead
  of spinning up a temporary Web server.
- Module identity renamed to `github.com/soulteary/stargate-suite`.
- Release `latest` tag is only updated for stable tags; pre-release tags never
  move `latest`.

### Security

- HMAC v1 disabled by default across the generated configuration.
- No silent Web UI port switching — a busy port is now a hard error.
