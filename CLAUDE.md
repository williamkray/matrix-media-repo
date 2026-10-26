# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Project Is

Matrix Media Repo (MMR) is a federated media repository for Matrix homeservers, written in Go. It handles media uploads, downloads, thumbnailing, URL previews, caching, and GDPR export/import across multiple homeservers. It uses PostgreSQL for metadata, pluggable datastores (S3 or filesystem) for media files, and Redis for distributed locking/caching.

## Build Commands

```bash
# Compile embedded web assets (required before building)
GOBIN=$PWD/bin go install -v ./cmd/utilities/compile_assets
$PWD/bin/compile_assets

# Build all binaries
GOBIN=$PWD/bin go install -ldflags "-X github.com/t2bot/matrix-media-repo/common/version.Version=$(git describe --tags)" -v ./cmd/...

# Build main worker with PGO
GOBIN=$PWD/bin go install -pgo=pgo_media_repo.pprof -ldflags "-X github.com/t2bot/matrix-media-repo/common/version.Version=$(git describe --tags)" -v ./cmd/workers/media_repo
```

## Lint / Static Analysis

```bash
go vet ./cmd/...
staticcheck ./cmd/...
```

## Tests

Tests use `testcontainers-go` to spin up Docker containers (Synapse, PostgreSQL, MMR itself). Docker must be running.

```bash
# Compile and run all tests (parallel=1 required due to container dependencies)
go test -c -v ./test && ./test.test '-test.v' -test.parallel 1

# Run a specific test suite
go test -v -run TestSuiteName ./test -test.parallel 1
```

Test files live in `test/` and use `testify/suite`. Infrastructure helpers (Docker container setup) are in `test/test_internals/`.

## Dev Environment

```bash
# One-time Synapse DB setup
docker run --rm -it -v ./dev/synapse-db:/data \
  -e SYNAPSE_SERVER_NAME=localhost \
  -e SYNAPSE_REPORT_STATS=no \
  matrixdotorg/synapse:latest generate

# Start full dev stack (Synapse, PostgreSQL, MMR, Element Web)
docker compose -f dev/docker-compose.yaml up
```

Dev stack exposes MMR on port 8001, Synapse on port 8008.

## Architecture

### Request Flow

HTTP requests enter via `api/webserver.go` and are routed through a numbered middleware chain (`api/_routers/`):
- Host detection, access token validation, X-Matrix server auth, metrics collection
- Context is assembled into an `rcontext.RequestContext` passed to all handlers

Endpoints are split into:
- `api/r0/` — Matrix spec r0/v1 client endpoints (upload, download, thumbnail, URL preview)
- `api/v1/` — Authenticated client endpoints
- `api/custom/` — Admin/management endpoints (purge, quarantine, datastore ops)
- `api/unstable/` — MSC (spec proposal) implementations

### Core Pipelines

Media operations are implemented as pipelines in `pipelines/`:
- `pipeline_upload/` — receives media, stores in datastore, records metadata
- `pipeline_download/` — retrieves local or federated media, handles redirects
- `pipeline_thumbnail/` — generates/caches thumbnails (ImageMagick, FFmpeg, libheif)
- `pipeline_url_preview/` — fetches OpenGraph/oEmbed metadata

### Storage

`datastores/` provides a unified interface over S3 (`minio-go`) and filesystem backends. Multiple datastores can be configured; media is assigned to a datastore at upload time. The datastore ID is stored in the `media` table.

### Database

PostgreSQL only. Schema is managed via numbered migrations in `database/migrations/`. The `database/` package has one file per table (e.g., `table_media.go`, `table_thumbnails.go`) containing all queries for that table. Database connections are initialized in `common/runtime/`.

### Configuration

YAML config loaded via `common/config/`. Supports per-homeserver overrides under `homeservers:`. Config is accessed globally via `config.Get()` and hot-reloads on file changes. See `config.sample.yaml` for the full schema.

### Background Tasks

`tasks/` contains a scheduler (`schedule.go`) that runs recurring jobs: remote media cleanup, thumbnail purging, URL preview expiry, data migrations. One-off tasks (GDPR exports) are also coordinated here.

### Key External Dependencies

- **ImageMagick 7** — SVG, JPEGXL, general image conversion
- **FFmpeg 6+** — Video thumbnail extraction
- **libheif 1.19.5** — HEIF/HEIC support
- **Redis** — Distributed locking (`go-redsync`) and caching
- **Prometheus** — Metrics exposed at `/metrics`
- **Sentry** — Error tracking (optional)

## Changelog

Update `CHANGELOG.md` using keep-a-changelog format when making user-facing changes.

## Goroutine Leak Investigation (issue #470)

**Status**: Fix deployed as `williamkray/mmr-dev:v1.3.8-dev2` (branch `v1.3.8-dev2`, commit `5ba913b`) on 2026-04-03. Monitoring telemetry to confirm resolution.

**Previous deployed version**: `williamkray/mmr-dev:latest` / `v1.3.8-dev1` — the original code, no analysis-branch changes.

### Symptoms

Goroutine count grows slowly over time (~3/hour observed), then spikes. When it crosses ~100, the watchdog kills and restarts the container. During the spike, federation media downloads time out (TCP connections hang). Confirmed via the 2026-04-02 restart at 21:20 UTC (2:20 PM Pacific): goroutines climbed from baseline ~56 to peak ~115 over ~19 hours before the watchdog fired.

### Root Cause (confirmed)

Two bugs in `pipelines/_steps/download/try_download.go:TryDownload` + `matrix/requests.go:FederatedGet`:

1. **Goroutine leak** (`try_download.go:237`): `res := <-ch` is a plain blocking receive. If `fn` hangs inside `matrix.FederatedGet` on a slow/unreachable server, the HTTP handler goroutine blocks indefinitely — one leaked goroutine per stuck federation request.

2. **No context propagation** (`requests.go:92`): `http.NewRequest(...)` does not accept a context, so the in-flight HTTP call inside `FederatedGet` is never cancelled when the request context fires. The federation client timeout (`ctx.Config.TimeoutSeconds.Federation`, typically 120s) is the only backstop, and it outlasts `BlockForReadUntil` (~20s).

**Key observation from telemetry**: Truly stuck goroutines are invisible to Datadog tracing. When the process is killed by the watchdog, in-flight spans are never flushed. Only fast-failing federation requests (e.g., TLS handshake timeouts at ~10s) produce visible spans. The goroutine leak shows up only in the goroutine count metric, not in trace durations.

### What Was NOT the cause

The `analysis-and-changes` branch added a `select` on `ctx.Context.Done()` to `try_download.go`, which introduced a new race: context fires at 20s → `defer close(ch)` closes the channel → `fn` (still blocked in `FederatedGet` for up to 100 more seconds) sends on the closed channel → panic. This caused the `"send on closed channel"` + `"Panic from internal queue downloads"` errors seen in earlier Datadog data. Those panics are an artifact of that partial fix, not the original bug.

### Fix Applied (branch `v1.3.8-dev2`)

Two changes, both required:

- **`pipelines/_steps/download/try_download.go:45`**: `make(chan downloadResult)` → `make(chan downloadResult, 1)`. Buffered channel ensures `fn` can always send even if the caller has already returned and deferred `close(ch)`, eliminating any panic risk.
- **`matrix/requests.go:92`**: `http.NewRequest(...)` → `http.NewRequestWithContext(ctx.Context, ...)`. Propagates context cancellation into the outbound HTTP call so federation requests are actually aborted when `BlockForReadUntil` fires, instead of tying up a pool worker for up to 120s.

### Telemetry Checks

The primary signal is the goroutine count plateau. **Run these checks at intervals** (24h, 72h, 1 week post-deploy) to assess whether the leak is resolved.

#### Check 1 — Goroutine trend (most important, 1 MCP call)

```
metric: avg:runtime.go.metrics.sched_goroutines.goroutines{service:matrix-media-repo}
from: <deploy time>   to: now
```

**Healthy**: flat line at baseline ~55–58, no upward trend, no watchdog restarts.
**Leak persists**: slow climb resuming; will eventually spike and trigger restart again.
**New regression**: sudden spikes or drops not at restart boundaries.

To keep this efficient, use `raw_data: false` (binned stats, 20 buckets) — one call gives you min/max/avg across the window. Only drill to raw data if the trend looks wrong.

#### Check 2 — Panic errors absent (1 MCP call, run only if check 1 looks suspicious)

```
log query: service:matrix-media-repo ("send on closed channel" OR "Panic from internal queue downloads")
from: <deploy time>   to: now
```

**Healthy**: 0 results. These panics should not appear in the new code.
**If they appear**: the buffered channel fix isn't working as expected — investigate whether there's another close path.

#### Check 3 — Federation span durations improved (1 MCP call, spot-check)

```
span query: service:matrix-media-repo resource_name:*federation*download* status:error
from: <deploy time>   to: now
```

**Healthy**: Error spans should be short (sub-20s). With context propagation working, hung federation requests are cancelled at ~20s rather than running to the 120s client timeout. If you see spans approaching 120s, the context wiring may not be taking effect for some code path.
**Note**: Zero long-duration spans is expected and correct — goroutines killed mid-flight never emit spans. Absence of >20s spans is a positive signal.

#### Check 4 — Watchdog restart events (authoritative, 1 MCP call)

The watchdog runs as `service:mmr-restarter`. It polls MMR's Prometheus endpoint every ~60 seconds and logs the goroutine count. The trigger message when it acts is:

```
log query: service:mmr-restarter "Too many goroutines"
from: <deploy time>   to: now
```

**Healthy**: 0 results.
**If triggered**: the goroutine count exceeded the watchdog threshold. Note the timestamp and cross-reference with Check 1 to see the growth curve leading up to it.

Normal "ok" poll entries look like: `"looks ok... only N goroutines"` — these are noise, not actionable.

**Important caveat**: The watchdog reads goroutine counts from MMR's Prometheus metrics endpoint directly. This value differs from the Datadog APM runtime metric (`sched_goroutines.goroutines`). Observed on 2026-04-02: the Datadog metric showed ~57 baseline post-restart while the watchdog simultaneously reported ~102. Treat the Datadog metric as a smooth trend indicator and the watchdog logs as the authoritative trigger record. The watchdog threshold appears to be somewhere above 103 (seen "looks ok" at 103; restart fired when the count was higher, likely ~115–120 based on Datadog max).

#### Efficient multi-check pattern

For a routine status check, run checks 1 and 4 in a single message (two parallel MCP calls). Only invoke checks 2 and 3 if check 1 shows a rising trend or check 4 found a restart event. This keeps routine checks to 1–2 MCP calls.

### Remaining unknowns / potential red herrings

- **Leak rate vs. traffic**: The observed ~3 goroutines/hour is an average. Burst traffic (many concurrent federation requests to unreachable servers) could cause faster accumulation even with the fix if some other code path is leaking. If the plateau drifts upward under load, investigate other `res := <-ch` patterns or blocking pool submissions.
- **Other federation call sites**: `FederatedGet` is also called from paths outside `TryDownload`. Those callers may have their own blocking patterns, but they're not the primary leak vector confirmed here.
- **Pool worker exhaustion vs. handler goroutine leak**: The original leak is handler goroutines (blocked at `res := <-ch`), not pool workers. With the fix, pool workers are freed promptly when context cancels. If goroutine count still grows, check whether the ants pool queue is backing up (pool workers stuck elsewhere).
- **`.well-known` lookup timeout**: `matrix/server_discovery.go` previously used bare `http.Get` with no timeout for `.well-known` lookups — a separate goroutine leak vector identified in the `analysis-and-changes` branch but NOT included in this fix. If the plateau trend shows residual slow growth, this is the next place to look. The fix is: add a 10s timeout to that HTTP client.
