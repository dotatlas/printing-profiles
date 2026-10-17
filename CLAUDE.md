# Printing Profiles

An open-source website for sharing 3D printing slicer presets. A Go API, a Next.js frontend, and PostgreSQL.

## Layout

```
api/                 Go module: main.go, internal/<pkg>/ (one level), migrations/, queries/, openapi.yaml
web/                 Next.js app (TypeScript)
infra/               OpenTofu (never state files or secrets)
bruno/               API collection: one request per route, with assertions
docker-compose.yml   local PostgreSQL
.env.example         every environment variable, with placeholders
```

## Commands

Added here as the code lands. Planned:

```bash
docker compose up -d                 # local Postgres
go -C api run . migrate              # apply migrations
go -C api run . serve                # run the API
go -C api test -race ./...           # Go tests
npm --prefix web run dev             # run the frontend
npm --prefix web test                # frontend tests
cd bruno && npx @usebruno/cli run --env local   # API checks against the running API
```

## Conventions

- **Go:**
  - `gofmt` and `go vet` clean. Errors are wrapped with `%w` and returned, never ignored.
  - `context.Context` first. No global mutable state.
  - SQL only through sqlc-generated, parameterized queries.
  - Schema changes only through new goose migrations. Never edit an applied one; add first, remove in a later release.
- **API:**
  - Routes live under `/api/v1/`, and each is documented in `api/openapi.yaml`.
  - Errors use RFC 9457 problem details. Lists use cursor pagination.
- **TypeScript:**
  - `strict`, no `any`.
  - API types are generated from `api/openapi.yaml`, never written by hand.
  - Server components for public data.
- **Comments:**
  - Every function and method gets a one-line doc comment: what it does, and how it relates to other functions.
  - Comments inside functions are sparse, only where a line isn't obvious.
  - All comments are in ASD-STE100 Simplified Technical English: short sentences, one meaning per word, active voice.
- **Tests:**
  - Write the test first and watch it fail.
  - Go integration tests use a real Postgres in Docker.
  - Every write route has a test proving a stranger is refused.
  - Every route has a Bruno request.
- **Security:**
  - Never commit secrets (`.env` is ignored).
  - Validate input at the API boundary.
  - Check authorization on every write.
  - Never store passwords (sign-in is OAuth only).
  - Never log tokens, session IDs or personal data.
  - Validate uploads by their content, never their file name.
- **Commits:**
  - Conventional Commits (`feat: ...`, `fix: ...`), lowercase, no emojis, no trailing period.
  - Code, its tests and its Bruno requests go in separate commits.
