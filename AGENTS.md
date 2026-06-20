# Agent Notes

## Current Facts

- This is a Go 1.21 Gin middleware for DandanPlay-compatible API routes.
- Actual public routes are:
  - `GET /api/v2/search/episodes`
  - `GET /api/v2/comment/:id`
  - `GET /api/v2/bangumi/bgmtv/:id`
  - `GET /api/v2/related/:id`
- Keep `/api/v2/comment/:id` as the frontend compatibility baseline unless the user explicitly approves a routing migration.
- Current danmaku responses are DandanPlay-compatible JSON objects with `count` and `comments`, not bare arrays.
- Current known `xfdm-web` call is `GET ${NEXT_PUBLIC_DANMAKU_MIDDLEWARE_URL}/api/v2/comment/${dandanEpisodeId}?withRelated=true`; first-stage `variant_key` should only include normalized `withRelated`.
- Current cache persistence is Redis only. There is no SQL database, ORM, or migration framework yet.
- Docker Compose runs `middleware` and `redis`; Redis persists to named volume `dandanplay-newmiddleware-bgmcors_redis-data` under the default project name.
- The real `.env` is ignored. Use `.env.example` for documented variables.

## Next Design Context

- The handoff at `/Users/galentwww/Desktop/dandanplay_middleware_handoff.md` describes the next major work: persistent danmaku snapshots and cache refresh control.
- Treat the handoff as a design target, not current implementation. Current code still sends Redis miss/error paths directly to upstream.
- If a future design mentions `/comments/{episodeId}`, verify with the user; current fact is `/api/v2/comment/:id`.

## Verification

- Run `go test ./...` after Go changes.
- Run `docker compose config --quiet` after Compose changes.
- Run `docker compose build` when Dockerfile or dependency files change.
