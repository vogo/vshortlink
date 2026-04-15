# VShortLink - High-Performance URL Shortening Service

## Overview

VShortLink is a high-performance URL shortening service that supports the generation, management, and access of short link codes of various lengths. The system uses a pre-generated short code pool approach to improve the response speed of short link creation and implements a mechanism for recycling and reusing short codes.

## Core Features

- **Multiple Short Code Lengths**: Supports 4, 5, 6, and other lengths of short codes to meet different scenario requirements
- **Pre-generated Short Code Pool**: Improves the response speed of creating short links by batch pre-generating short codes and storing them in Redis
- **Short Code Recycling Mechanism**: Supports the recycling and reuse of short codes, efficiently utilizing limited short link resources
- **High-Performance Design**: Uses multi-level caching strategies to ensure high performance and low latency for short link access
- **Distributed Support**: Redis-based distributed design, supporting horizontal scaling
- **Resource Management**: Proper resource cleanup with Close methods for pools and caches
- **Automatic Expiration**: Built-in expiration checking for cached short links
- **Per-Code Hit Stats**: Async, drop-on-full hit counting off the redirect hot path; configurable retention window; pull-style query API for downstream systems to archive

## Technical Architecture

- **Storage Layer**:
  - MySQL/PostgreSQL: Stores basic information and status of short links
  - Redis: Stores short code pool and short link mapping relationships
  - Memory Cache: Provides the fastest query response

- **Core Algorithms**:
  - Base62 short code generation algorithm
  - Batch generation and index management mechanism
  - Short code recycling and cooling period design

## Quick Start

### Install Dependencies

```bash
go mod tidy
```

### Run the Server

The server can be configured using environment variables:

```bash
# MySQL Configuration
export MYSQL_HOST=localhost
export MYSQL_PORT=3306
export MYSQL_USER=root
export MYSQL_PASSWORD=password
export MYSQL_DATABASE=vshortlink

# Redis Configuration
export REDIS_HOST=localhost
export REDIS_PORT=6379
export REDIS_PASSWORD=
export REDIS_DB=0

# Service Configuration
export SERVER_PORT=8080
export BATCH_GENERATE_SIZE=100
export MAX_CODE_LENGTH=6
export AUTH_TOKEN=                     # optional; if set, required as Authorization header on /__* endpoints

# Stats Configuration (all optional)
export STATS_TIMEZONE=UTC              # IANA tz name used to compute day keys; e.g. "Asia/Shanghai"
export STATS_RETENTION_DAYS=7          # days of history kept and queryable
export STATS_FLUSH_INTERVAL_SECONDS=1  # recorder flush cadence
export STATS_BUFFER_SIZE=10000         # hit events buffered before drop-on-full

# Start the server
go run cmd/server/main.go
```

## Core Packages

### cores Package

The `cores` package defines the core interfaces and implementations for the short link system:

- `link.go`: Defines the basic parameters and status of short links
- `service.go`: Implements the core service for short link creation, retrieval, and management, including proper resource cleanup
- `repo.go`: Defines the repository interface for short link storage
- `cache.go`: Defines the cache interface for short link caching with Close method for resource cleanup
- `pool.go`: Defines the pool interface for short code management with Close method for resource cleanup
- `stats.go` / `stats_recorder.go`: Defines the optional per-code daily hit-count interface and the async, drop-on-full recorder that flushes buffered hits off the redirect path

### Implementation Packages

All backend implementations live under the `ext/` package:

- `ext/gormx`: MySQL-based implementation using GORM for persistent storage of short links
- `ext/redisx`: Redis-based implementation with the following features:
  - Short code pool management using Redis ZSet
  - Short link caching with automatic expiration checking
  - Combined storage of link and expiration time in a single value
  - Per-code daily hit stats: one HASH per day (`shortlink:stats:{day}`) with TTL reapplied on every flush, so retention is enforced declaratively by Redis
  - Proper resource management with Close methods
- `ext/memx`: Memory-based implementation for testing and development with proper resource cleanup (stats backend does no auto-trimming)

## Hybrid Implementation

The server in `cmd/server/main.go` uses a hybrid approach:

- Repository: GORM-based MySQL implementation for persistent storage
- Cache: Redis-based implementation for high-performance caching with automatic expiration checking
- Pool: Redis-based implementation for distributed short code pool management
- Stats: Redis-based implementation storing per-code daily hit counters with TTL-driven retention

This hybrid approach combines the reliability of MySQL with the performance of Redis. The service properly manages resources by calling Close methods on the cache, pool, and stats backend when shutting down, ensuring clean resource release and that the in-memory hit buffer is flushed before exit.

## API Usage

### Create a Short Link

```bash
curl -X POST http://localhost:8080/create_link \
  -H "Content-Type: application/json" \
  -d '{"link":"https://example.com","length":4,"expire":"2023-12-31T23:59:59Z"}'
```

Response:

```json
{
  "id": 1,
  "length": 4,
  "code": "abcd",
  "link": "https://example.com",
  "expire": "2023-12-31T23:59:59Z",
  "status": 1
}
```

### Access a Short Link

Simply visit the short link in your browser:

```
http://localhost:8080/{code}
```

The server will redirect you to the original URL.

### Query Hit Stats

All `/__*` endpoints require the `Authorization` header to match `AUTH_TOKEN` when that env var is set.

Single code, last N days (N defaults to and is capped at `STATS_RETENTION_DAYS`):

```bash
curl "http://localhost:8080/__stats?code=abcd&days=7" \
  -H "Authorization: $AUTH_TOKEN"
```

Response (days with zero hits are omitted):

```json
{ "20260409": 12, "20260410": 5, "20260414": 1 }
```

Batch pull for downstream archival (one call per batch of codes):

```bash
curl -X POST http://localhost:8080/__stats_batch \
  -H "Authorization: $AUTH_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"codes":["abcd","efgh"], "days":7}'
```

Response (codes with zero hits are omitted):

```json
{
  "abcd": { "20260414": 1, "20260410": 5 },
  "efgh": { "20260413": 3 }
}
```

Hit counting is asynchronous and drops events when the in-process buffer is full, so counts are approximate under sustained overload. Day keys are computed in the timezone given by `STATS_TIMEZONE`.

## Design Document

For detailed design documentation, please refer to the [Design Document](doc/design.md).

## Contributing

We welcome contributions to VShortLink! Here's how you can help:

### Reporting Issues

If you encounter any bugs or have feature requests, please create an issue on the GitHub repository with the following information:

- A clear and descriptive title
- A detailed description of the issue or feature request
- Steps to reproduce the issue (for bugs)
- Expected behavior and actual behavior
- Environment information (OS, Go version, Redis/MySQL version, etc.)
- Any relevant logs or screenshots

### Contributing Code

1. Fork the repository
2. Create a new branch for your feature or bugfix: `git checkout -b feature/your-feature-name` or `git checkout -b fix/your-bugfix-name`
3. Make your changes
4. Add tests for your changes when applicable
5. Run the existing tests to ensure nothing is broken: `go test ./...`
6. Commit your changes with a descriptive commit message
7. Push your branch to your fork
8. Create a Pull Request to the main repository

### Code Style Guidelines

- Follow standard Go code style and conventions
- Use `gofmt` or `goimports` to format your code
- Add appropriate comments and documentation
- Ensure your code passes `golint` and `go vet`
- Write unit tests for new functionality

### Pull Request Process

1. Ensure your PR includes a clear description of the changes and the purpose
2. Update documentation if necessary
3. Make sure all tests pass
4. Your PR will be reviewed by maintainers, who may request changes
5. Once approved, your PR will be merged

## License

This project is licensed under the Apache License 2.0. See the [LICENSE](LICENSE) file for details.