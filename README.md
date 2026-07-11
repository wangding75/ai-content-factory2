# AI Content Factory 2.0

AI Content Factory 2.0 P0 monorepo.

## Stack

- Go modular-monolith API and worker
- Next.js App Router Web
- PostgreSQL
- Redis + Asynq-compatible worker boundary
- OpenAPI + JSON Schema
- Docker Compose
- Playwright

## Development

```powershell
Copy-Item .env.example .env
docker compose up -d postgres redis
go run ./apps/api/cmd/api
pnpm.cmd --dir apps/web dev
```

## P0 boundaries

- Content pack: `novel`
- Workflow provider: `mock`
- Real AI: disabled
- External workflows: disabled
- Publishing: disabled