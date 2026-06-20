# Danmaku Snapshot Persistence Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement persistent danmaku snapshots so Redis misses no longer directly trigger DandanPlay upstream calls.

**Architecture:** Add middleware-owned PostgreSQL for gzip-compressed snapshots, keep Redis as a hot cache, and route `/api/v2/comment/:id` through a `CommentService`. First loads use singleflight; stale snapshots return immediately and enqueue bounded background refresh.

**Tech Stack:** Go 1.21, Gin, go-redis v8, pgx v5, PostgreSQL 16 Alpine, Docker Compose.

## Global Constraints

- The middleware will always run as a single instance.
- Use an independent PostgreSQL database owned by this middleware.
- Do not connect to or foreign-key into the core business database.
- Keep `/api/v2/comment/:id` as the compatibility baseline.
- The first-stage `variant_key` only includes normalized `withRelated`.
- Store full DandanPlay-compatible `{ count, comments }` JSON payloads gzip-compressed in PostgreSQL.
- Existing Redis data is empty; do not implement backfill.
- Keep search and Bangumi routes on their current Redis-only behavior.

---

### Task 1: Runtime Configuration

**Files:**
- Modify: `config/config.go`
- Modify: `config/config_test.go`
- Modify: `.env.example`
- Modify: `docker-compose.yml`

**Interfaces:**
- Produces `config.Config.DatabaseURL string`
- Produces refresh settings:
  - `RedisSnapshotTTL time.Duration`
  - `DefaultRefreshInterval time.Duration`
  - `EmptyCommentsRefreshInterval time.Duration`
  - `RefreshFailureRetryInterval time.Duration`
  - `RefreshQueueSize int`
  - `RefreshWorkerCount int`

- [ ] Add failing config test for database and refresh defaults.
- [ ] Implement config fields with defaults: `48h`, `24h`, `1h`, `30m`, queue `100`, workers `2`.
- [ ] Add PostgreSQL service and `postgres-data` volume to Compose.
- [ ] Add database env vars to `.env.example`.
- [ ] Run `go test ./config`.

### Task 2: Snapshot Domain Utilities

**Files:**
- Create: `services/danmaku/variant.go`
- Create: `services/danmaku/variant_test.go`
- Create: `services/danmaku/payload.go`
- Create: `services/danmaku/payload_test.go`
- Create: `services/danmaku/policy.go`
- Create: `services/danmaku/policy_test.go`

**Interfaces:**
- Produces `NormalizeVariant(url.Values) Variant`
- Produces `Variant.Key() string`
- Produces `ValidatePayload([]byte) (PayloadInfo, error)`
- Produces `GzipPayload([]byte) ([]byte, error)`
- Produces `GunzipPayload([]byte) ([]byte, error)`
- Produces `NextRefreshAt(now time.Time, info PayloadInfo) time.Time`

- [ ] Test `withRelated=true` and `withRelated=1` normalize to `v1|withRelated=1`.
- [ ] Test missing or false `withRelated` normalizes to `v1|withRelated=0`.
- [ ] Test unrelated query params do not change variant key.
- [ ] Test valid payload with `count` and `comments` returns comment count.
- [ ] Test invalid JSON or missing `comments` fails validation.
- [ ] Test gzip round-trip.
- [ ] Test empty comments use one-hour refresh and non-empty uses default refresh.
- [ ] Run `go test ./services/danmaku`.

### Task 3: PostgreSQL Migration And Repository

**Files:**
- Create: `migrations/001_danmaku_snapshots.sql`
- Create: `storage/postgres.go`
- Create: `storage/snapshots.go`
- Create: `storage/snapshots_test.go`

**Interfaces:**
- Produces `storage.OpenPostgres(ctx, databaseURL string) (*pgxpool.Pool, error)`
- Produces `storage.Migrate(ctx, db *pgxpool.Pool) error`
- Produces `SnapshotStore` with `Get`, `Upsert`, and `MarkRefreshError`

- [ ] Add SQL migration for `danmaku_snapshots`.
- [ ] Write repository tests using a real PostgreSQL URL from `TEST_DATABASE_URL`, skipped when unset.
- [ ] Implement migration runner.
- [ ] Implement repository methods.
- [ ] Run `go test ./storage`.

### Task 4: Redis Snapshot Cache

**Files:**
- Create: `services/danmaku/cache.go`
- Create: `services/danmaku/cache_test.go`

**Interfaces:**
- Produces `SnapshotCache` with `Get(ctx, key)`, `Set(ctx, snapshot, ttl)`, `Delete(ctx, key)`.
- Produces cache statuses `CacheHit`, `CacheMiss`, `CacheUnavailable`, `CacheCorrupted`.

- [ ] Test cache key generation.
- [ ] Test corrupted envelope is reported distinctly from miss.
- [ ] Implement Redis envelope cache around existing `*redis.Client`.
- [ ] Run `go test ./services/danmaku`.

### Task 5: Comment Service

**Files:**
- Create: `services/danmaku/service.go`
- Create: `services/danmaku/service_test.go`
- Modify: `services/dandanplay.go`

**Interfaces:**
- Produces `CommentService.GetComments(ctx, dandanEpisodeID string, query url.Values) ([]byte, error)`.
- Produces bounded refresh queue and singleflight first-load behavior.
- Produces `DandanplayService.GetDanmakuWithContext(ctx, id, query string)`.

- [ ] Test Redis hit returns cached payload without store or upstream.
- [ ] Test Redis miss plus PostgreSQL hit returns stored payload and refills Redis.
- [ ] Test first load writes PostgreSQL before Redis.
- [ ] Test stale snapshot returns stale payload when upstream refresh fails.
- [ ] Test no snapshot plus upstream failure returns service unavailable error.
- [ ] Implement service and context-aware upstream method.
- [ ] Run `go test ./services/danmaku ./services`.

### Task 6: HTTP Wiring

**Files:**
- Modify: `main.go`
- Modify: `handlers/danmaku.go`
- Modify: `utils/cache.go`

**Interfaces:**
- Initializes PostgreSQL pool and migration at startup.
- Initializes `CommentService`.
- Keeps search and Bangumi behavior unchanged.

- [ ] Wire database startup after config and before routes.
- [ ] Replace `GetDanmaku` handler body with `CommentService.GetComments`.
- [ ] Preserve response content type and route path.
- [ ] Run `go test ./...`.

### Task 7: Documentation And Runtime Verification

**Files:**
- Modify: `README.md`
- Modify: `AGENTS.md`

**Interfaces:**
- Documents new database env vars, Postgres volume, and fixed refresh policy.

- [ ] Update docs.
- [ ] Run `gofmt -w` on changed Go files.
- [ ] Run `go test ./...`.
- [ ] Run `docker compose config --quiet`.
- [ ] Run `docker compose build`.
- [ ] Optionally run `docker compose up -d` and smoke-test `/api/v2/related/1`.
