# Go report workflow

The PR job runs both owned Actions with `commit: false` and released CLIs
(`go-test-report-action v1.1.2`, `goreportcard-action v1.1.3`, pinned wrapper SHAs).
It replaces the primary Linux Go test/coverage invocation and reuses its profile
for existing coverage artifacts/gates. Other OS matrices, release verification,
integration, benchmarks and fuzz workflows retain their distinct checks.

Reports run on pushed tags and `workflow_dispatch`. They do not generate bot
commits on branch pushes. A separate job may commit the five stable reports in
one `[skip ci]` commit, only if the tested commit still equals the default branch
head. Older release tags and non-default manual refs retain downloadable reports
without overwriting the current branch. PRs never enter that write job. Protected
branches can reject writeback; no PAT, bypass, or force-push is used.

Test directory: `.`. Packages: `./...`.
Race detector: `true`. Additional args: `-count=1`.
Coverage floor: `30%` (zero records a baseline).
Per-package floor: `0%`.
Quality-score floor: `0%`.
Raw profiles/test logs are artifacts, never committed.

The workflow explicitly resolves packages excluding `/e2e($|/)`: coverage describes the
non-E2E Go packages only, with the existing 30% floor. These tests require no
external service. PR smoke E2E and main full E2E remain separate; they require
Docker Compose, Stargate, Warden, Herald, and their Redis services generated from
the test fixtures. Existing readiness checks, credentials, service logs and
10/20-minute E2E timeouts are preserved. Container runtime smoke is separate.

GTR v1.1.2 filters package discovery with `exclude` but still invokes tests with
the original patterns. Therefore this workflow passes the resolved non-E2E
import paths directly to `packages`; an empty set fails instead of falling back
to `./...`. This retains a released CLI without accidentally running E2E.

Published releases query actual tag-push report runs for the tested commit. A successful or active tag run prevents duplicate tests; missing, failed, cancelled, or skipped runs allow the release event to generate reports. This covers skipped commits and releases whose tag push produced no workflow event. API lookup errors fail the gate rather than silently assume a report exists. Stable report-only artifacts explicitly restore `.github` on download; mixed report/document artifacts retain repository hierarchy.
