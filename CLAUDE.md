# CLAUDE.md

URL shortener with pluggable backends.

## Commands

- `make test` / `go test ./cores/ -run TestName` — test suite / single test.
- `make format` / `make lint` / `make license-check` — goimports+fmt+gofumpt / golangci-lint / Apache header check (new `.go` files need the header).
- `make build` — license-check + format + lint + test.
- `go run cmd/server/main.go` — prod server (needs MySQL + Redis).

## Architecture

Four core interfaces in `cores/` with swappable backends in `ext/`:

- `ShortLinkRepository` (`repo.go`) — stores `ShortLink` rows + per-length `startIndex`.
- `ShortCodePool` (`pool.go`) — pre-generated code pool with distributed `Lock`/`Unlock`.
- `ShortLinkCache` (`cache.go`) — fast code→URL lookup with embedded expiration.
- `ShortLinkStats` (`stats.go`) — optional daily hit counters; nil = no-op.

`ShortLinkService` (`service.go`) composes them, adds in-process `expirable.LRU` (5min/10240), schedules daily `ExpireActives` + `RecycleExpires` via `vrun.Runner`.

Backends: `ext/gormx` (MySQL repo), `ext/redisx` (pool/cache/stats), `ext/memx` (all four, tests/examples). Prod wires gormx repo + redisx others in `cmd/server/main.go`.

**Codes** are base62, strided: `step = 62^length / batchSize`; `startIndex` monotonic, exhausted when `startIndex > step`. `length <= manualCodeLength` (3) = caller-supplied via `Add`; larger = pool-drawn via `Create`.

**HTTP** (`cores/http.go`): `GET /{code}` → LRU→cache→302/404, records hit on non-404. `*/__{op}` management (`get/create/update/remove/list/stats/stats_batch`) guarded by `Authorization` header vs `AUTH_TOKEN`. `__` prefix (`ManagementCodePrefix`) reserved — don't allocate colliding codes.

**Stats** (`stats_recorder.go`): `RecordHit` is non-blocking — pushes to bounded channel (default 10000), dropped when full. Recorder goroutine merges `code→delta` and flushes every `WithStatsFlushInterval` (1s) via `IncrBatch`. Day key uses `statsLoc` timezone (`STATS_TIMEZONE` env, default UTC). Redis backend reapplies `EXPIRE (retention+1)*24h` on every flush; memx does no trimming. `Service.Close()` drains recorder before closing `Stats`. `GetStats`/`BatchGetStats` cap `days` at retention.

**Recycling**: `ExpireActives` moves past-expiry → `LinkStatusExpired` (evicts cache). `RecycleExpires` after 365d cooling → pool + `LinkStatusRecycled`. `RebuildCodePool` re-adds every batch from `startIndex=0..current`, then removes still-active/expired codes.
