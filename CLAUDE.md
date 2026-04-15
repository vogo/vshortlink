# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

- `make test` — run full test suite (`go test ./... -v`).
- `go test ./cores/ -run TestName` — run a single test.
- `make format` — goimports, `go fmt`, and gofumpt.
- `make lint` — golangci-lint.
- `make license-check` — enforces Apache headers on all `.go` files; new Go files must include the Apache 2.0 header block seen in existing sources. Requires `license-header-checker` installed.
- `make build` — runs license-check, format, lint, and test in sequence (no binary output).
- `go run cmd/server/main.go` — start the production server (needs MySQL + Redis; see README for env vars).

## Architecture

The project is a URL shortener organized around three pluggable interfaces in `cores/`, with interchangeable backends under sibling packages.

### Core interfaces (`cores/`)

- `ShortLinkRepository` (`repo.go`) — persistent storage for `ShortLink` rows plus per-length `startIndex` bookkeeping for the generator.
- `ShortCodePool` (`pool.go`) — pre-generated pool of unused codes keyed by length, with `Lock`/`Unlock` for distributed coordination and `Clear` for rebuilds.
- `ShortLinkCache` (`cache.go`) — fast-path code→URL lookup with expiration stored alongside the value.

`ShortLinkService` (`service.go`) composes all three, adds an in-process `expirable.LRU` in front of the cache (`memLRUCache`, default 5 min TTL, 10240 entries), and schedules daily `ExpireActives` + `RecycleExpires` jobs via `vrun.Runner`.

### Backend implementations

All backend implementations live under `ext/` — three parallel packages, each implementing one or more of the core interfaces:

- `ext/gormx/` — GORM/MySQL repository (persistent layer used in production `cmd/server`).
- `ext/redisx/` — Redis-backed `ShortCodePool` (ZSet) and `ShortLinkCache` (value encodes both link and expiration, checked on read).
- `ext/memx/` — in-memory versions of all three interfaces; used for tests and `examples/mem_examples`.

`cmd/server/main.go` wires the hybrid production mix: `ext/gormx` repo + `ext/redisx` cache + `ext/redisx` pool. The `examples/` subfolders show pure-in-memory, pure-Redis, pure-GORM variants plus a `rebuild_pool` example.

### Code generation algorithm (`gen.go`, `util.go`)

Codes are base62. Per-length pool generation is strided: `step = 62^length / batchSize`. Each invocation of `GenerateBatchNumbers(length, startIndex)` produces `batchSize` codes by walking `startIndex, startIndex+step, startIndex+2*step, ...`, then `startIndex` is incremented by 1 and persisted via `Repo.SaveStartIndex`. This interleaves future batches between previously-issued codes rather than allocating contiguous ranges, so `startIndex` must monotonically increase and is authoritative across restarts. When `startIndex > step` the pool for that length is exhausted.

### Length policy

`manualCodeLength` (default 3) and `maxCodeLength` (default 9, overridable) split two regimes:

- `length <= manualCodeLength` → caller supplies the code directly (`Service.Add`); code uniqueness is checked against the repo (recycled entries may be reused).
- `length > manualCodeLength` → code is pulled from the `ShortCodePool` (`Service.Create`). Generators are created only for these lengths in `NewShortLinkService`.

### HTTP surface (`cores/http.go`)

Single handler `ShortLinkService.HttpHandle` registered at `/` in `cmd/server`:

- `GET /{code}` — lookup via LRU → cache, then 302 redirect or 404.
- `*/__{op}` — management endpoints (`get`, `create`, `update`, `remove`, `list`). Guarded by `Authorization` header matching `authToken` when configured (`WithAuthToken` / `AUTH_TOKEN` env).

The `__` prefix (`ManagementCodePrefix`) is the routing discriminator; don't allocate real codes that collide.

### Recycling and rebuild

- `ExpireActives` — moves active-but-past-expiry links to `LinkStatusExpired` and evicts them from cache.
- `RecycleExpires` — after a cooling period (hardcoded 365 days in `service.go`), returns expired codes to the pool and marks them `LinkStatusRecycled`.
- `RebuildCodePool(ctx, length)` — disaster-recovery path: locks the length, clears the pool, re-adds every batch from `startIndex=0..current`, then removes codes whose links still exist with `LinkStatusActive` or `LinkStatusExpired` (not recycled). See `examples/rebuild_pool/`.
