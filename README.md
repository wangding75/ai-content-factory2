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

### Local Manual Development (Alternative)

```powershell
Copy-Item .env.example .env
docker compose up -d postgres
go run ./apps/api/cmd/api
pnpm.cmd --dir apps/web dev
```

### One-Click Docker Compose Development Environment (Recommended)

You can launch the entire project using Docker Compose:

```bash
docker compose up -d --build
```

No additional port forwarding (such as `netsh`, Node proxy, etc.) is needed. All services (PostgreSQL, database migration, Go API, Next.js Web) are run together automatically.

- **Next.js Web**: [http://localhost:13001](http://localhost:13001)
- **Go API**: [http://localhost:18080](http://localhost:18080)

#### Check Status

```bash
docker compose ps
```

#### View Logs

```bash
docker compose logs -f --tail=200
```

#### Stop Environment

```bash
docker compose down
```

#### Stop and Reset Database Data

```bash
docker compose down -v
```

> [!WARNING]
> - Under normal circumstances, do NOT use the `-v` flag when stopping.
> - Running `docker compose down -v` will permanently delete the project database volume and all stored data.

## P0 boundaries

- Content pack: `novel`
- Workflow provider: `mock`
- Real AI: disabled
- External workflows: disabled
- Publishing: disabled