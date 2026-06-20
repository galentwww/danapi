# Observability And Health Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add health endpoints, optional danmaku decision logs, and documented database inspection.

**Architecture:** Health checks live in `handlers` and receive Redis/PostgreSQL ping functions from `main`. Danmaku logging stays in `services/danmaku` so cache source and refresh policy decisions are logged where they are made. Deployment docs expose the config switch and optional Adminer override.

**Tech Stack:** Go 1.21, Gin, go-redis v8, pgx v5, Docker Compose.

## Global Constraints

- Keep `/api/v2/comment/:id` unchanged.
- Do not log AppSecret values.
- Do not log danmaku payload bodies.
- Run `go test ./...` after Go changes.
- Run `docker compose config --quiet` after Compose changes.

---

### Task 1: Health Endpoints

**Files:**
- Create: `handlers/health.go`
- Create: `handlers/health_test.go`
- Modify: `main.go`

**Interfaces:**
- Produces: `SetHealthChecks(redisCheck, postgresCheck func(context.Context) error)`
- Produces: `Healthz(c *gin.Context)`
- Produces: `Readyz(c *gin.Context)`

- [x] Write failing tests for `/healthz`, healthy `/readyz`, and failed dependency `/readyz`.
- [x] Implement health handlers with a bounded dependency check context.
- [x] Register `/healthz` and `/readyz` in `main.go`.
- [x] Inject Redis `Ping` and PostgreSQL `Ping`.

### Task 2: Danmaku Decision Logging

**Files:**
- Modify: `config/config.go`
- Modify: `config/config_test.go`
- Modify: `services/danmaku/policy.go`
- Modify: `services/danmaku/policy_test.go`
- Modify: `services/danmaku/service.go`
- Modify: `services/danmaku/service_test.go`
- Modify: `main.go`

**Interfaces:**
- Produces: `Configuration.DanmakuDecisionLog bool`
- Produces: `CommentServiceOptions.DecisionLog bool`
- Produces: `RefreshDecision.Rule string`

- [x] Write failing tests for `DANMAKU_DECISION_LOG`, refresh rule names, and request source logs.
- [x] Parse `DANMAKU_DECISION_LOG` from environment.
- [x] Return rule names from refresh policy decisions.
- [x] Log request source and refresh decisions only when the switch is enabled.

### Task 3: Deployment Documentation

**Files:**
- Modify: `.env.example`
- Modify: `docker-compose.yml`
- Modify: `docker-compose.baota.yml`
- Create: `docker-compose.adminer.yml`
- Modify: `README.md`

**Interfaces:**
- Produces: `DANMAKU_DECISION_LOG=false` documented default.
- Produces: optional Adminer service exposed on `${ADMINER_PORT:-8081}`.

- [x] Add the logging switch to env templates and Compose services.
- [x] Add middleware container healthcheck using `/readyz`.
- [x] Document Uptime Kuma `/readyz` monitoring.
- [x] Document `psql` and optional Adminer database inspection.

### Task 4: External Read-Only PostgreSQL Access

**Files:**
- Modify: `config/config.go`
- Modify: `config/config_test.go`
- Create: `storage/readonly.go`
- Create: `storage/readonly_test.go`
- Modify: `main.go`
- Modify: `.env.example`
- Modify: `docker-compose.yml`
- Modify: `docker-compose.baota.yml`
- Modify: `README.md`

**Interfaces:**
- Produces: `Configuration.DatabaseReadOnlyUser string`
- Produces: `Configuration.DatabaseReadOnlyPassword string`
- Produces: `storage.EnsureReadOnlyRole(ctx context.Context, db *pgxpool.Pool, options storage.ReadOnlyRoleOptions) error`

- [x] Add failing config tests for read-only database credentials.
- [x] Add failing storage tests for safe PostgreSQL role identifiers.
- [x] Implement read-only role creation/update and grants.
- [x] Call read-only role setup after migrations.
- [x] Map PostgreSQL to `${POSTGRES_BIND_ADDRESS:-127.0.0.1}:${POSTGRES_PORT:-15432}:5432`.
- [x] Document local and remote connection guidance.
