#requires -Version 5.1
<#
.SYNOPSIS
  Initializes the AI Content Factory 2.0 monorepo scaffold on Windows.

.DESCRIPTION
  Creates the Iteration 01 engineering scaffold:
  - Go modular-monolith API and worker
  - Next.js App Router web application
  - PostgreSQL and Redis Docker Compose services
  - OpenAPI contract baseline
  - Monorepo, docs, tests, scripts, and .ai-dev structure
  - Basic health endpoints and verification

  The script is intended for an empty project that may already contain:
  .git, .ai-dev, setup-go-windows.ps1, and this initialization script.

.EXAMPLE
  Set-ExecutionPolicy -Scope Process Bypass -Force
  .\init-project-scaffold-windows.ps1 -ProjectRoot "D:\ai\ai-content-factory2"
#>

[CmdletBinding()]
param(
    [Parameter()]
    [string]$ProjectRoot = (Get-Location).Path,

    [Parameter()]
    [string]$GoModule = "github.com/local/ai-content-factory/apps/api",

    [Parameter()]
    [switch]$SkipWebInstall,

    [Parameter()]
    [switch]$Force
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Write-Step {
    param([string]$Message)
    Write-Host ""
    Write-Host "==> $Message" -ForegroundColor Cyan
}

function Write-Ok {
    param([string]$Message)
    Write-Host "[PASS] $Message" -ForegroundColor Green
}

function Write-WarnMessage {
    param([string]$Message)
    Write-Host "[WARN] $Message" -ForegroundColor Yellow
}

function Write-Fail {
    param([string]$Message)
    Write-Host "[FAIL] $Message" -ForegroundColor Red
}

function Assert-Command {
    param(
        [Parameter(Mandatory = $true)][string]$CommandName,
        [Parameter()][string]$DisplayName = $CommandName
    )

    $command = Get-Command $CommandName -ErrorAction SilentlyContinue
    if (-not $command) {
        throw "$DisplayName is not available: $CommandName"
    }

    return $command.Source
}

function Write-Utf8NoBom {
    param(
        [Parameter(Mandatory = $true)][string]$Path,
        [Parameter(Mandatory = $true)][string]$Content
    )

    $parent = Split-Path -Parent $Path
    if ($parent -and -not (Test-Path -LiteralPath $parent)) {
        New-Item -ItemType Directory -Path $parent -Force | Out-Null
    }

    $encoding = New-Object System.Text.UTF8Encoding($false)
    [IO.File]::WriteAllText($Path, $Content, $encoding)
}

function Ensure-Directory {
    param([Parameter(Mandatory = $true)][string]$Path)

    if (-not (Test-Path -LiteralPath $Path -PathType Container)) {
        New-Item -ItemType Directory -Path $Path -Force | Out-Null
    }
}

function Assert-LastExitCode {
    param([Parameter(Mandatory = $true)][string]$Message)

    if ($LASTEXITCODE -ne 0) {
        throw "$Message (exit code $LASTEXITCODE)"
    }
}

function Invoke-Pnpm {
    param([Parameter(Mandatory = $true)][string[]]$Arguments)

    & pnpm.cmd @Arguments
    Assert-LastExitCode "pnpm command failed: pnpm $($Arguments -join ' ')"
}

function Invoke-Go {
    param([Parameter(Mandatory = $true)][string[]]$Arguments)

    & go @Arguments
    Assert-LastExitCode "Go command failed: go $($Arguments -join ' ')"
}

$root = [IO.Path]::GetFullPath($ProjectRoot)
if (-not (Test-Path -LiteralPath $root -PathType Container)) {
    throw "ProjectRoot does not exist: $root"
}

Set-Location $root

$reportPath = Join-Path $root ".ai-dev\reports\scaffold-initialization.json"
$startedAt = [DateTime]::UtcNow
$report = [ordered]@{
    project_root = $root
    go_module = $GoModule
    started_at_utc = $startedAt.ToString("o")
    finished_at_utc = $null
    tools = [ordered]@{}
    created_paths = @()
    verification = [ordered]@{
        go_test = "not_run"
        web_lint = "not_run"
        web_typecheck = "not_run"
        compose_config = "not_run"
    }
    result = "failed"
}

try {
    Write-Step "Checking required tools"

    $null = Assert-Command -CommandName "go.exe" -DisplayName "Go"
    $null = Assert-Command -CommandName "git.exe" -DisplayName "Git"
    $null = Assert-Command -CommandName "node.exe" -DisplayName "Node.js"
    $null = Assert-Command -CommandName "pnpm.cmd" -DisplayName "pnpm"
    $null = Assert-Command -CommandName "docker.exe" -DisplayName "Docker"

    $report.tools.go = (& go version)
    $report.tools.git = (& git --version)
    $report.tools.node = (& node --version)
    $report.tools.pnpm = (& pnpm.cmd --version)
    $report.tools.docker = (& docker --version)
    $report.tools.compose = (& docker compose version)

    Write-Ok $report.tools.go
    Write-Ok $report.tools.git
    Write-Ok "Node.js $($report.tools.node)"
    Write-Ok "pnpm $($report.tools.pnpm)"
    Write-Ok $report.tools.docker
    Write-Ok $report.tools.compose

    cmd /c "docker info >nul 2>&1"
    Assert-LastExitCode "Docker Engine is not running"
    Write-Ok "Docker Engine is running."

    Write-Step "Checking project directory"

    $allowedRootItems = @(
        ".git",
        ".ai-dev",
        "setup-go-windows.ps1",
        "init-project-scaffold-windows.ps1"
    )

    $unexpectedItems = Get-ChildItem -Force -LiteralPath $root |
        Where-Object { $_.Name -notin $allowedRootItems }

    if ($unexpectedItems -and -not $Force) {
        $names = ($unexpectedItems | Select-Object -ExpandProperty Name) -join ", "
        throw "Project directory is not empty. Unexpected items: $names. Rerun with -Force only after reviewing them."
    }

    if ($unexpectedItems -and $Force) {
        Write-WarnMessage "Force mode enabled. Existing items will not be deleted: $((($unexpectedItems | Select-Object -ExpandProperty Name) -join ', '))"
    }
    else {
        Write-Ok "Project directory is ready for initialization."
    }

    Write-Step "Creating monorepo directories"

    $directories = @(
        "apps",
        "apps\api",
        "packages\contracts\openapi",
        "packages\contracts\openapi\paths",
        "packages\contracts\openapi\schemas\common",
        "packages\contracts\content-packs\novel",
        "packages\contracts\workflow-providers\mock",
        "packages\shared-types",
        "packages\eslint-config",
        "packages\tsconfig",
        "docs\product",
        "docs\architecture",
        "docs\api",
        "docs\testing",
        "docs\decisions",
        "docs\iterations",
        "docs\prototypes\p0-frames",
        "deployments\docker",
        "deployments\compose",
        "deployments\k8s",
        "scripts",
        "tasks",
        "tests\contract",
        "tests\e2e\fixtures",
        "tests\e2e\pages",
        "tests\e2e\specs",
        "tests\fixtures",
        ".github\workflows",
        ".ai-dev\iterations",
        ".ai-dev\reports",
        ".ai-dev\templates"
    )

    foreach ($directory in $directories) {
        $fullPath = Join-Path $root $directory
        Ensure-Directory $fullPath
        $report.created_paths += $directory
    }

    Write-Ok "Monorepo directories created."

    Write-Step "Writing root configuration"

    Write-Utf8NoBom -Path (Join-Path $root ".gitignore") -Content @'
# Environment
.env
.env.*
!.env.example

# Go
bin/
coverage/
*.test
*.out

# Node / Next.js
node_modules/
.pnpm-store/
.next/
out/
dist/
coverage/
*.tsbuildinfo

# IDE / OS
.vscode/
.idea/
.DS_Store
Thumbs.db

# Runtime data
data/
tmp/
logs/

# Generated reports
.ai-dev/reports/*.tmp
'@

    Write-Utf8NoBom -Path (Join-Path $root ".gitattributes") -Content @'
* text=auto
*.go text eol=lf
*.ts text eol=lf
*.tsx text eol=lf
*.js text eol=lf
*.json text eol=lf
*.yaml text eol=lf
*.yml text eol=lf
*.md text eol=lf
*.ps1 text eol=crlf
*.png binary
*.jpg binary
*.zip binary
'@

    Write-Utf8NoBom -Path (Join-Path $root ".editorconfig") -Content @'
root = true

[*]
charset = utf-8
end_of_line = lf
insert_final_newline = true
trim_trailing_whitespace = true
indent_style = space
indent_size = 2

[*.go]
indent_style = tab
indent_size = 4

[*.ps1]
end_of_line = crlf
indent_size = 4

[Makefile]
indent_style = tab
'@

    Write-Utf8NoBom -Path (Join-Path $root ".env.example") -Content @'
APP_ENV=development

API_PORT=8080
WEB_PORT=3000
NEXT_PUBLIC_API_BASE_URL=http://localhost:8080/api/v1

POSTGRES_DB=acf
POSTGRES_USER=acf
POSTGRES_PASSWORD=acf
DATABASE_URL=postgres://acf:acf@localhost:5432/acf?sslmode=disable

REDIS_URL=redis://localhost:6379/0

WORKFLOW_PROVIDER=mock
CONTENT_PACKS=novel

LOG_LEVEL=info
'@

    Write-Utf8NoBom -Path (Join-Path $root "package.json") -Content @'
{
  "name": "ai-content-factory",
  "version": "0.1.0",
  "private": true,
  "packageManager": "pnpm@9.1.1",
  "scripts": {
    "dev:web": "pnpm --dir apps/web dev",
    "build:web": "pnpm --dir apps/web build",
    "lint:web": "pnpm --dir apps/web lint",
    "typecheck:web": "pnpm --dir apps/web exec tsc --noEmit",
    "test:web": "pnpm --dir apps/web test",
    "check": "pnpm lint:web && pnpm typecheck:web"
  }
}
'@

    Write-Utf8NoBom -Path (Join-Path $root "pnpm-workspace.yaml") -Content @'
packages:
  - "apps/web"
  - "packages/*"
'@

    Write-Utf8NoBom -Path (Join-Path $root "go.work") -Content @'
go 1.26.5

use (
    ./apps/api
)
'@

    Write-Utf8NoBom -Path (Join-Path $root "README.md") -Content @'
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
'@

    Write-Utf8NoBom -Path (Join-Path $root "Makefile") -Content @'
.PHONY: bootstrap up down test test-api test-web check-contracts verify

bootstrap:
	pnpm install
	cd apps/api && go mod download

up:
	docker compose up -d

down:
	docker compose down

test: test-api test-web

test-api:
	cd apps/api && go test ./...

test-web:
	pnpm --dir apps/web lint
	pnpm --dir apps/web exec tsc --noEmit

check-contracts:
	@echo "Contract validation will be implemented in Iteration 00/01."

verify: test check-contracts
	docker compose config
'@

    Write-Ok "Root configuration written."

    Write-Step "Creating Go API module"

    $apiRoot = Join-Path $root "apps\api"
    $apiDirectories = @(
        "cmd\api",
        "cmd\worker",
        "cmd\migrate",
        "internal\platform\config",
        "internal\platform\httpserver",
        "internal\platform\logging",
        "internal\project",
        "internal\material",
        "internal\narrative",
        "internal\chapterplan",
        "internal\content",
        "internal\review",
        "internal\workflow",
        "internal\works",
        "internal\capability",
        "internal\audit",
        "plugins\contentpacks\novel",
        "plugins\workflowproviders\mock",
        "migrations",
        "test\integration",
        "test\fixtures",
        "test\testutil"
    )

    foreach ($directory in $apiDirectories) {
        Ensure-Directory (Join-Path $apiRoot $directory)
    }

    Write-Utf8NoBom -Path (Join-Path $apiRoot "go.mod") -Content @"
module $GoModule

go 1.26.5
"@

    Write-Utf8NoBom -Path (Join-Path $apiRoot "internal\platform\config\config.go") -Content @'
package config

import (
	"os"
)

type Config struct {
	Environment string
	APIAddress  string
	DatabaseURL string
	RedisURL    string
}

func Load() Config {
	return Config{
		Environment: envOrDefault("APP_ENV", "development"),
		APIAddress:  ":" + envOrDefault("API_PORT", "8080"),
		DatabaseURL: envOrDefault("DATABASE_URL", "postgres://acf:acf@localhost:5432/acf?sslmode=disable"),
		RedisURL:    envOrDefault("REDIS_URL", "redis://localhost:6379/0"),
	}
}

func envOrDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
'@

    Write-Utf8NoBom -Path (Join-Path $apiRoot "internal\platform\httpserver\server.go") -Content @'
package httpserver

import (
	"encoding/json"
	"net/http"
	"time"
)

type Server struct {
	httpServer *http.Server
}

type envelope struct {
	Data      any    `json:"data"`
	RequestID string `json:"request_id"`
}

func New(address string) *Server {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", healthHandler)
	mux.HandleFunc("GET /readyz", readyHandler)
	mux.HandleFunc("GET /api/v1/meta", metaHandler)

	return &Server{
		httpServer: &http.Server{
			Addr:              address,
			Handler:           withRequestID(mux),
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       15 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       60 * time.Second,
		},
	}
}

func (s *Server) ListenAndServe() error {
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown() error {
	return s.httpServer.Close()
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, r, http.StatusOK, map[string]any{
		"status":  "ok",
		"service": "api",
	})
}

func readyHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, r, http.StatusOK, map[string]any{
		"status": "ready",
		"checks": map[string]string{
			"api": "ok",
		},
	})
}

func metaHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, r, http.StatusOK, map[string]any{
		"product":           "AI Content Factory 2.0",
		"scope":             "P0",
		"content_packs":     []string{"novel"},
		"workflow_provider": "mock",
		"real_ai":           "disabled",
		"external_workflow": "disabled",
		"publishing":        "disabled",
	})
}

func writeJSON(w http.ResponseWriter, r *http.Request, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(envelope{
		Data:      data,
		RequestID: requestIDFrom(r),
	})
}
'@

    Write-Utf8NoBom -Path (Join-Path $apiRoot "internal\platform\httpserver\middleware.go") -Content @'
package httpserver

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"
)

type contextKey string

const requestIDKey contextKey = "request_id"

var requestCounter uint64

func withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := fmt.Sprintf(
			"req_%d_%d",
			time.Now().UTC().UnixMilli(),
			atomic.AddUint64(&requestCounter, 1),
		)

		w.Header().Set("X-Request-ID", id)
		ctx := context.WithValue(r.Context(), requestIDKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func requestIDFrom(r *http.Request) string {
	value, _ := r.Context().Value(requestIDKey).(string)
	return value
}
'@

    Write-Utf8NoBom -Path (Join-Path $apiRoot "internal\platform\httpserver\server_test.go") -Content @'
package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthHandler(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	recorder := httptest.NewRecorder()

	withRequestID(http.HandlerFunc(healthHandler)).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}

	if recorder.Header().Get("X-Request-ID") == "" {
		t.Fatal("expected X-Request-ID header")
	}
}
'@

    Write-Utf8NoBom -Path (Join-Path $apiRoot "cmd\api\main.go") -Content @'
package main

import (
	"errors"
	"log"
	"net/http"

	"github.com/local/ai-content-factory/apps/api/internal/platform/config"
	"github.com/local/ai-content-factory/apps/api/internal/platform/httpserver"
)

func main() {
	cfg := config.Load()
	server := httpserver.New(cfg.APIAddress)

	log.Printf("api listening on %s", cfg.APIAddress)

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
'@.Replace("github.com/local/ai-content-factory/apps/api", $GoModule)

    Write-Utf8NoBom -Path (Join-Path $apiRoot "cmd\worker\main.go") -Content @'
package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	log.Println("worker started with provider=mock")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("worker stopped")
}
'@

    Write-Utf8NoBom -Path (Join-Path $apiRoot "cmd\migrate\main.go") -Content @'
package main

import "log"

func main() {
	log.Println("migration runner scaffold initialized")
}
'@

    Write-Utf8NoBom -Path (Join-Path $apiRoot "migrations\000001_init.up.sql") -Content @'
CREATE TABLE IF NOT EXISTS audit_logs (
    id UUID PRIMARY KEY,
    actor_id TEXT NOT NULL,
    action TEXT NOT NULL,
    subject_type TEXT NOT NULL,
    subject_id TEXT NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
'@

    Write-Utf8NoBom -Path (Join-Path $apiRoot "migrations\000001_init.down.sql") -Content @'
DROP TABLE IF EXISTS audit_logs;
'@

    Write-Utf8NoBom -Path (Join-Path $apiRoot "Dockerfile") -Content @'
FROM golang:1.26-alpine AS build
WORKDIR /src

COPY go.mod ./
RUN go mod download

COPY . .
ARG TARGET=api
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/app ./cmd/${TARGET}

FROM alpine:3.22
RUN adduser -D -H -u 10001 appuser
USER appuser
COPY --from=build /out/app /app
ENTRYPOINT ["/app"]
'@

    Write-Utf8NoBom -Path (Join-Path $apiRoot "README.md") -Content @'
# API

Go modular-monolith API and worker.

```powershell
go run ./apps/api/cmd/api
go test ./apps/api/...
```
'@

    Push-Location $apiRoot
    try {
        Invoke-Go -Arguments @("fmt", "./...")
        Invoke-Go -Arguments @("mod", "tidy")
    }
    finally {
        Pop-Location
    }

    Write-Ok "Go API scaffold created."

    Write-Step "Creating Next.js Web application"

    $webRoot = Join-Path $root "apps\web"

    if (Test-Path -LiteralPath $webRoot) {
        $existingWebItems = Get-ChildItem -Force -LiteralPath $webRoot -ErrorAction SilentlyContinue
        if ($existingWebItems -and -not $Force) {
            throw "apps/web already contains files. Rerun with -Force only after reviewing the directory."
        }
    }

    if (-not $SkipWebInstall) {
        if (Test-Path -LiteralPath $webRoot) {
            Remove-Item -LiteralPath $webRoot -Recurse -Force
        }

        Invoke-Pnpm -Arguments @(
            "dlx",
            "create-next-app@latest",
            "apps/web",
            "--ts",
            "--eslint",
            "--tailwind",
            "--app",
            "--src-dir",
            "--import-alias", "@/*",
            "--use-pnpm",
            "--empty",
            "--disable-git",
            "--no-agents-md",
            "--yes"
        )

        Write-Ok "Next.js application created."
    }
    else {
        Ensure-Directory $webRoot
        Write-WarnMessage "Web dependency installation skipped."
    }

    if (-not $SkipWebInstall) {
        $webPackagePath = Join-Path $webRoot "package.json"
        $webPackage = Get-Content -LiteralPath $webPackagePath -Raw | ConvertFrom-Json
        $webPackage.name = "@acf/web"

        if (-not $webPackage.scripts.PSObject.Properties["typecheck"]) {
            $webPackage.scripts | Add-Member -NotePropertyName "typecheck" -NotePropertyValue "tsc --noEmit"
        }

        $webPackage |
            ConvertTo-Json -Depth 20 |
            ForEach-Object { Write-Utf8NoBom -Path $webPackagePath -Content $_ }

        Write-Utf8NoBom -Path (Join-Path $webRoot "src\app\page.tsx") -Content @'
export default function HomePage() {
  return (
    <main className="min-h-screen bg-slate-50 p-10 text-slate-950">
      <section className="mx-auto max-w-5xl rounded-2xl border border-slate-200 bg-white p-10 shadow-sm">
        <p className="text-sm font-medium text-indigo-600">S00_HOME</p>
        <h1 className="mt-3 text-3xl font-semibold">AI Content Factory</h1>
        <p className="mt-3 max-w-2xl text-slate-600">
          P0 engineering scaffold is running. Product pages will be implemented by vertical iteration.
        </p>

        <dl className="mt-8 grid gap-4 sm:grid-cols-3">
          <div className="rounded-xl bg-slate-100 p-4">
            <dt className="text-sm text-slate-500">Content pack</dt>
            <dd className="mt-1 font-medium">novel</dd>
          </div>
          <div className="rounded-xl bg-slate-100 p-4">
            <dt className="text-sm text-slate-500">Workflow provider</dt>
            <dd className="mt-1 font-medium">mock</dd>
          </div>
          <div className="rounded-xl bg-slate-100 p-4">
            <dt className="text-sm text-slate-500">P0 status</dt>
            <dd className="mt-1 font-medium">scaffold initialized</dd>
          </div>
        </dl>
      </section>
    </main>
  );
}
'@

        Write-Utf8NoBom -Path (Join-Path $webRoot "Dockerfile") -Content @'
FROM node:24-alpine AS dependencies
WORKDIR /app
RUN corepack enable
COPY package.json ./
RUN pnpm install --no-frozen-lockfile

FROM node:24-alpine AS build
WORKDIR /app
RUN corepack enable
COPY --from=dependencies /app/node_modules ./node_modules
COPY . .
RUN pnpm build

FROM node:24-alpine AS runtime
WORKDIR /app
ENV NODE_ENV=production
RUN corepack enable
COPY --from=build /app ./
EXPOSE 3000
CMD ["pnpm", "start"]
'@
    }

    Write-Step "Writing Docker Compose"

    Write-Utf8NoBom -Path (Join-Path $root "docker-compose.yml") -Content @'
name: ai-content-factory2

services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_DB: ${POSTGRES_DB:-acf}
      POSTGRES_USER: ${POSTGRES_USER:-acf}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:-acf}
    ports:
      - "5432:5432"
    volumes:
      - acf_postgres_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U ${POSTGRES_USER:-acf} -d ${POSTGRES_DB:-acf}"]
      interval: 5s
      timeout: 3s
      retries: 20

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    volumes:
      - acf_redis_data:/data
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 3s
      retries: 20

  api:
    build:
      context: ./apps/api
      args:
        TARGET: api
    environment:
      APP_ENV: development
      API_PORT: 8080
      DATABASE_URL: postgres://${POSTGRES_USER:-acf}:${POSTGRES_PASSWORD:-acf}@postgres:5432/${POSTGRES_DB:-acf}?sslmode=disable
      REDIS_URL: redis://redis:6379/0
      WORKFLOW_PROVIDER: mock
      CONTENT_PACKS: novel
    ports:
      - "8080:8080"
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy

  worker:
    build:
      context: ./apps/api
      args:
        TARGET: worker
    environment:
      APP_ENV: development
      DATABASE_URL: postgres://${POSTGRES_USER:-acf}:${POSTGRES_PASSWORD:-acf}@postgres:5432/${POSTGRES_DB:-acf}?sslmode=disable
      REDIS_URL: redis://redis:6379/0
      WORKFLOW_PROVIDER: mock
      CONTENT_PACKS: novel
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy

  web:
    build:
      context: ./apps/web
    environment:
      NEXT_PUBLIC_API_BASE_URL: http://localhost:8080/api/v1
    ports:
      - "3000:3000"
    depends_on:
      - api

volumes:
  acf_postgres_data:
  acf_redis_data:
'@

    Write-Ok "Docker Compose written."

    Write-Step "Writing contract baseline"

    Write-Utf8NoBom -Path (Join-Path $root "packages\contracts\openapi\openapi.yaml") -Content @'
openapi: 3.1.0
info:
  title: AI Content Factory 2.0 API
  version: 0.1.0
  description: P0 API contract baseline.
servers:
  - url: http://localhost:8080
paths:
  /healthz:
    get:
      operationId: getHealth
      responses:
        "200":
          description: API is alive
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Envelope"
  /readyz:
    get:
      operationId: getReadiness
      responses:
        "200":
          description: API is ready
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Envelope"
  /api/v1/meta:
    get:
      operationId: getApplicationMeta
      responses:
        "200":
          description: P0 capability metadata
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Envelope"
components:
  schemas:
    Envelope:
      type: object
      required:
        - data
        - request_id
      properties:
        data: {}
        request_id:
          type: string
'@

    Write-Utf8NoBom -Path (Join-Path $root "packages\contracts\content-packs\novel\project.schema.json") -Content @'
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://ai-content-factory.local/content-packs/novel/project.schema.json",
  "title": "Novel Project",
  "type": "object",
  "required": ["name", "type"],
  "properties": {
    "name": {
      "type": "string",
      "minLength": 1,
      "maxLength": 120
    },
    "type": {
      "const": "novel"
    }
  },
  "additionalProperties": true
}
'@

    Write-Utf8NoBom -Path (Join-Path $root "packages\contracts\README.md") -Content @'
# Contracts

Contract-first source of truth:

```text
OpenAPI / JSON Schema
→ generated types
→ API handlers
→ Web API client
```

P0:

- Content pack: `novel`
- Workflow provider: `mock`
'@

    Write-Step "Writing Windows scripts"

    Write-Utf8NoBom -Path (Join-Path $root "scripts\verify-environment.ps1") -Content @'
Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

go version
git --version
node --version
pnpm.cmd --version
cmd /c "docker info >nul 2>&1"
if ($LASTEXITCODE -ne 0) {
    throw "Docker Engine is not running."
}
docker compose version

Write-Host "[PASS] Environment verification completed." -ForegroundColor Green
'@

    Write-Utf8NoBom -Path (Join-Path $root "scripts\verify-scaffold.ps1") -Content @'
Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

Push-Location .\apps\api
try {
    go test ./...
    if ($LASTEXITCODE -ne 0) {
        throw "Go tests failed."
    }
}
finally {
    Pop-Location
}

pnpm.cmd --dir apps/web lint
if ($LASTEXITCODE -ne 0) {
    throw "Web lint failed."
}

pnpm.cmd --dir apps/web exec tsc --noEmit
if ($LASTEXITCODE -ne 0) {
    throw "Web typecheck failed."
}

docker compose config *> $null
if ($LASTEXITCODE -ne 0) {
    throw "Docker Compose configuration validation failed."
}

Write-Host "[PASS] Scaffold verification completed." -ForegroundColor Green
'@

    Write-Utf8NoBom -Path (Join-Path $root "scripts\start-infrastructure.ps1") -Content @'
Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

docker compose up -d postgres redis
if ($LASTEXITCODE -ne 0) {
    throw "Unable to start PostgreSQL and Redis."
}

docker compose ps
'@

    Write-Utf8NoBom -Path (Join-Path $root "scripts\stop-infrastructure.ps1") -Content @'
Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

docker compose down
if ($LASTEXITCODE -ne 0) {
    throw "Unable to stop Docker Compose services."
}
'@

    Write-Step "Initializing iteration state"

    Write-Utf8NoBom -Path (Join-Path $root ".ai-dev\state.json") -Content @'
{
  "project": "ai-content-factory-2.0",
  "current_iteration": 1,
  "status": "in_progress",
  "next_iteration": 2,
  "contract_version": "p0-v1",
  "ui_baseline": "p0-frozen",
  "technical_skeleton": "same-as-1.0"
}
'@

    Write-Utf8NoBom -Path (Join-Path $root ".ai-dev\iterations\01.json") -Content @'
{
  "iteration": 1,
  "name": "scaffold-infrastructure",
  "status": "in_progress",
  "acceptance": [
    "Go API tests pass",
    "Web lint passes",
    "Web typecheck passes",
    "Docker Compose config is valid",
    "healthz and readyz are available after startup"
  ]
}
'@

    Write-Step "Verifying scaffold"

    Push-Location $apiRoot
    try {
        Invoke-Go -Arguments @("test", "./...")
        $report.verification.go_test = "passed"
        Write-Ok "Go tests passed."
    }
    finally {
        Pop-Location
    }

    if (-not $SkipWebInstall) {
        Invoke-Pnpm -Arguments @("--dir", "apps/web", "lint")
        $report.verification.web_lint = "passed"
        Write-Ok "Web lint passed."

        Invoke-Pnpm -Arguments @("--dir", "apps/web", "exec", "tsc", "--noEmit")
        $report.verification.web_typecheck = "passed"
        Write-Ok "Web typecheck passed."
    }
    else {
        $report.verification.web_lint = "skipped"
        $report.verification.web_typecheck = "skipped"
    }

    cmd /c "docker compose config >nul"
    Assert-LastExitCode "Docker Compose configuration is invalid"
    $report.verification.compose_config = "passed"
    Write-Ok "Docker Compose configuration is valid."

    Write-Step "Showing Git status"

    git status --short
    Assert-LastExitCode "Unable to read Git status"

    $report.result = "passed"
    Write-Step "Project scaffold initialization completed"
    Write-Ok "Result: PASS"
    Write-Host ""
    Write-Host "Next verification command:" -ForegroundColor Cyan
    Write-Host "  .\scripts\verify-scaffold.ps1" -ForegroundColor White
}
catch {
    $report.result = "failed"
    $report.error = $_.Exception.Message
    Write-Fail $_.Exception.Message
    throw
}
finally {
    $report.finished_at_utc = [DateTime]::UtcNow.ToString("o")

    $reportDirectory = Split-Path -Parent $reportPath
    Ensure-Directory $reportDirectory

    $report |
        ConvertTo-Json -Depth 10 |
        Set-Content -LiteralPath $reportPath -Encoding UTF8

    Write-Host ""
    Write-Host "Initialization report: $reportPath" -ForegroundColor DarkGray
}
