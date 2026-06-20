# Danmaku Snapshot Persistence Design

## Goal

Reduce DandanPlay upstream comment requests without changing the current frontend contract:

```http
GET /api/v2/comment/{dandanEpisodeId}?withRelated=true
```

The response body remains the DandanPlay-compatible JSON object:

```json
{
  "count": 3269,
  "comments": []
}
```

## Confirmed Constraints

- The middleware will always run as a single instance.
- The first stage uses an independent PostgreSQL database owned by this middleware.
- The middleware must not connect to or foreign-key into the core business database.
- Existing Redis data is empty, so no Redis backfill is required.
- The only confirmed frontend query parameter is `withRelated=true`.
- The existing route `/api/v2/comment/:id` remains the compatibility baseline.
- Payloads may contain up to roughly 7000 comments, so database payload storage must use gzip.
- If no snapshot exists and upstream fails, return a clear 502/503 error.
- If a stale snapshot exists and upstream fails, return the stale snapshot.

## Architecture

The comment read path becomes:

```text
HTTP handler
  -> CommentService
  -> RedisSnapshotCache
  -> PostgresSnapshotStore
  -> DandanPlay upstream client
```

Redis is a short-lived hot cache. PostgreSQL is the persistent snapshot store. DandanPlay is called only for first loads or controlled refreshes.

The first stage deliberately does not use the core `episodes` table. That keeps large, third-party, low-business-value payloads out of the core business database and allows independent backup, cleanup, and retention policies.

## Data Model

Create one table in the middleware-owned PostgreSQL database:

```sql
create table danmaku_snapshots (
    dandan_episode_id bigint not null,
    variant_key text not null,

    payload bytea not null,
    payload_encoding text not null default 'gzip',

    fetched_at timestamptz not null,
    next_refresh_at timestamptz not null,

    danmaku_count integer not null default 0,
    content_hash text not null,
    unchanged_streak integer not null default 0,

    version bigint not null default 1,
    last_refresh_status text not null default 'success',
    last_error text null,

    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),

    primary key (dandan_episode_id, variant_key)
);
```

Indexes:

```sql
create index danmaku_snapshots_next_refresh_at_idx
on danmaku_snapshots (next_refresh_at);
```

The primary key covers the main request lookup. The `next_refresh_at` index is for future maintenance queries and diagnostics.

## Variant Key

The first-stage variant is intentionally narrow:

```text
v1|withRelated=1
v1|withRelated=0
```

Normalization:

- `withRelated=true` and `withRelated=1` become `withRelated=1`.
- Missing, `false`, or `0` become `withRelated=0`.
- Other query parameters do not enter `variant_key`.

The upstream query still forwards the normalized `withRelated` value so behavior stays compatible.

## Cache And Refresh Policy

Use fixed first-stage timings:

```text
Redis snapshot TTL: 48h
Default refresh interval: 24h
Empty danmaku refresh interval: 1h
Refresh failure retry interval: 30m
Background refresh workers: 2
Background refresh queue size: 100
```

Redis TTL controls only hot-cache residency. It does not determine whether DandanPlay can be called.

`next_refresh_at` controls when an upstream refresh is allowed. A snapshot remains returnable even after `next_refresh_at`.

## Request Flow

### Fresh Or Hot Snapshot

1. Normalize `dandanEpisodeId` and `variant_key`.
2. Check Redis.
3. If Redis has a valid snapshot and `next_refresh_at` is in the future, return the decompressed payload.

### Redis Miss

1. Query PostgreSQL by `(dandan_episode_id, variant_key)`.
2. If PostgreSQL has a snapshot, refill Redis and return the decompressed payload.
3. If the snapshot is due for refresh, enqueue one background refresh after returning the stale payload.
4. If PostgreSQL has no snapshot, enter first-load flow.

### First Load

1. Use Go `singleflight` keyed by `dandan_episode_id + variant_key`.
2. Inside the singleflight function, check Redis and PostgreSQL again.
3. If still empty, call DandanPlay.
4. Validate the response is JSON with a `comments` array.
5. Compute `danmaku_count` and `content_hash`.
6. gzip the full JSON response body.
7. Upsert PostgreSQL first.
8. Write Redis.
9. Return the original response body.

### Due Refresh

When a snapshot exists but `next_refresh_at` has passed:

1. Return the existing snapshot immediately.
2. Enqueue a bounded background refresh.
3. Suppress duplicate refreshes for the same key with an in-process guard or `singleflight`.
4. If the queue is full, skip the refresh and let a later request try again.

## Error Semantics

- Redis miss: query PostgreSQL.
- Redis unavailable: query PostgreSQL.
- Redis corrupted value: ignore Redis, query PostgreSQL, and overwrite Redis from PostgreSQL when possible.
- PostgreSQL has stale snapshot and upstream fails: return stale snapshot, set `last_refresh_status='error'`, set `last_error`, and retry after 30 minutes.
- PostgreSQL has no snapshot and upstream fails: return 502 or 503 with a clear JSON error.
- Upstream returns valid `comments: []`: store it, return it, and set `next_refresh_at` to one hour later.

## Implementation Boundaries

In scope for stage one:

- Add PostgreSQL to Docker Compose.
- Add database configuration.
- Add schema migration file or startup migration runner.
- Add snapshot repository.
- Add Redis snapshot envelope cache.
- Add variant normalization.
- Add gzip encode/decode.
- Add singleflight for first loads.
- Add bounded background refresh queue.
- Keep search and Bangumi routes on the current Redis-only behavior.

Out of scope for stage one:

- Dynamic new/old episode refresh based on core `episodes`.
- Syncing metadata from the core database.
- Multi-instance Redis distributed locks.
- Quota counters.
- Prometheus/OpenTelemetry.
- Refresh event audit table.
- Existing Redis backfill.

## Verification

- Unit tests for `variant_key` normalization.
- Unit tests for gzip payload round-trip.
- Unit tests for refresh interval calculation.
- Repository tests against a test PostgreSQL instance or container.
- Handler/service tests for:
  - Redis hit returns cached payload.
  - Redis miss + PostgreSQL hit returns PostgreSQL payload and refills Redis.
  - First load writes PostgreSQL before Redis.
  - Stale snapshot returns immediately when refresh fails.
  - Empty `comments` response is stored.
- `go test ./...`.
- `docker compose config --quiet`.
- `docker compose build`.
