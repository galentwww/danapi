# Observability And Health Design

## Goal

Expose operational state without changing the DandanPlay-compatible API routes.

## Scope

- Keep `/api/v2/comment/:id` as the frontend compatibility route.
- Add process and readiness health endpoints outside `/api/v2`.
- Add optional danmaku decision logs for cache source and refresh rules.
- Document a lightweight way to inspect the middleware-owned PostgreSQL database.
- Expose PostgreSQL on a host port for inspection through a read-only role.

## Health Endpoints

`GET /healthz` returns `200` when the HTTP process is alive. It does not check Redis or PostgreSQL.

`GET /readyz` checks Redis and PostgreSQL. It returns `200` only when both dependencies respond, and `503` when either dependency fails. Uptime Kuma should monitor `/readyz`.

## Danmaku Decision Logging

`DANMAKU_DECISION_LOG=false` by default. When enabled, logs include:

- request source: `redis`, `postgres`, `upstream`, or `stale`
- `episode_id`
- `variant_key`
- `fetched_at`
- `next_refresh_at`
- refresh rule decisions such as `empty_danmaku`, `hot_changed`, `hot_unchanged`, `normal_changed`, `normal_unchanged`, `stable_unchanged`, `archived_unchanged`, and `refresh_failed_retry`

Logs must not include AppSecret values or danmaku payload bodies.

## Database Inspection

The primary low-friction path is `docker compose exec postgres psql -U middleware -d dandanplay_middleware`.

For browser inspection, use an optional Adminer compose override so production deployments do not expose a database UI unless explicitly requested.

PostgreSQL is mapped to `127.0.0.1:15432` by default. This supports local desktop tools without publishing the database on all interfaces. Operators can set `POSTGRES_BIND_ADDRESS=0.0.0.0` for remote access, but should first change `DATABASE_READONLY_PASSWORD`.

The middleware creates or updates `DATABASE_READONLY_USER` on startup when both `DATABASE_READONLY_USER` and `DATABASE_READONLY_PASSWORD` are set. The role receives `CONNECT`, `USAGE` on `public`, and `SELECT` on existing and future `public` tables. It is not granted write permissions.
