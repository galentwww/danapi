# Docker Adaptation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build Docker support for the current Go middleware and Redis runtime.

**Architecture:** Use a multi-stage Go Dockerfile for the middleware. Use Docker Compose to run the middleware with Redis on an internal network and persist Redis data in a named volume.

**Tech Stack:** Go 1.21, Gin, go-redis, Redis 7 Alpine, Docker, Docker Compose.

## Global Constraints

- Keep the existing local `.env` workflow.
- Allow container runtime configuration through environment variables without requiring a `.env` file inside the image.
- Do not add an unused database service yet.
- Do not commit the real `.env`.

---

### Task 1: Initialize Repository Hygiene

**Files:**
- Create: `.gitignore`

**Interfaces:**
- Produces: ignored local secrets, build outputs, OS metadata, and editor files.

- [ ] Add `.gitignore` covering `.env`, binaries, logs, `.DS_Store`, and temporary files.
- [ ] Run `git status --short` and confirm `.env` is not tracked.

### Task 2: Make `.env` Optional

**Files:**
- Modify: `config/config.go`
- Create: `config/config_test.go`

**Interfaces:**
- Produces: `LoadConfig() error` that succeeds when `.env` is absent and environment variables are set.

- [ ] Write a failing Go test proving `LoadConfig` works without a `.env` file.
- [ ] Run `go test ./config` and confirm the new test fails.
- [ ] Change `LoadConfig` to ignore only a missing `.env` file while still returning other load errors.
- [ ] Run `go test ./config` and confirm it passes.

### Task 3: Add Docker Runtime

**Files:**
- Create: `.dockerignore`
- Create: `Dockerfile`
- Create: `docker-compose.yml`
- Create: `.env.example`

**Interfaces:**
- Produces: `dandanplay-middleware` image and Compose services `middleware` and `redis`.

- [ ] Add `.dockerignore` excluding Git metadata, local env files, artifacts, and docs not required for build.
- [ ] Add a multi-stage Dockerfile that builds the Go binary and runs it as a non-root user.
- [ ] Add Compose with Redis persistence and middleware environment defaults.
- [ ] Add `.env.example` documenting runtime configuration.

### Task 4: Update Documentation

**Files:**
- Modify: `README.md`

**Interfaces:**
- Produces: Docker usage instructions for local and server deployments.

- [ ] Add Docker Compose setup instructions.
- [ ] Document Redis persistence and future database extension pattern.
- [ ] Keep existing binary build instructions.

### Task 5: Verify

**Files:**
- No new files.

**Interfaces:**
- Produces: fresh verification evidence.

- [ ] Run `gofmt -w config/config.go config/config_test.go`.
- [ ] Run `go test ./...`.
- [ ] Run `docker compose config`.
- [ ] Run `docker compose build`.
- [ ] Run `git status --short`.
