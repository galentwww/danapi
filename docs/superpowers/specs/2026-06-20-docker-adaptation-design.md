# Docker Adaptation Design

## Goal

Containerize the current DandanPlay middleware so it can run as a Go service with Redis through Docker Compose, while keeping the existing local `.env` workflow.

## Scope

- Initialize the local Git repository.
- Add a production-oriented multi-stage Docker image for the Go middleware.
- Add Docker Compose orchestration for the middleware and Redis.
- Persist Redis data with a named Docker volume.
- Make `.env` optional at runtime so containers can rely on injected environment variables.
- Document Docker build and run commands.

## Out Of Scope

- Adding a relational database before the application uses one.
- Changing API routes or request behavior.
- Changing cache semantics beyond making Redis container-friendly.

## Architecture

The application image builds a static Linux binary in a Go builder stage and runs it from a small runtime image. Compose starts Redis and the middleware on a shared internal network. The middleware reads configuration from `.env` when present, and otherwise reads environment variables directly.

## Configuration

`.env.example` documents all supported settings. In Compose, `REDIS_HOST` defaults to the Redis service name `redis`, and `SERVER_PORT` defaults to `8080`. The real `.env` remains untracked.

## Persistence

Redis uses append-only persistence and stores data in a named volume. Future databases should follow the same pattern: one service, one named volume, environment-driven credentials, and no application image changes unless the code starts using that database.

## Verification

- `go test ./...`
- `docker compose config`
- `docker compose build`
