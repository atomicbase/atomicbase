# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Atomicbase is a multi-tenant backend built on SQLite/Turso. It provides a REST API for database operations, schema templates, and will include auth/storage/dashboard in the future. The codebase has two main components:

- **api/** - Go backend (the main product)
- **sdk/** - TypeScript client SDK for the API

## Build & Test Commands

### Go API (in `api/` directory)

```bash
# Build (requires CGO for SQLite FTS5)
CGO_ENABLED=1 go build -tags fts5 -o bin/atomicbase

# Run
./bin/atomicbase

# Test all
CGO_ENABLED=1 go test -tags fts5 -v ./...

# Test single package
CGO_ENABLED=1 go test -tags fts5 -v ./database/...

# Integration tests
CGO_ENABLED=1 go test -tags "fts5 integration" -v ./database/...

# Or use Makefile
make build
make test
make run
```

### TypeScript SDK (in `sdk/` directory)

```bash
pnpm build    # Build with tsc
pnpm dev      # Watch mode
```

## Architecture

### API Structure (`api/`)

The API is a standard Go HTTP server using `net/http`:

- **main.go** - Server setup, middleware chain, graceful shutdown
- **config/** - Environment-based configuration via `godotenv`
- **data/** - Data API (`/data/*`) - CRUD, batch operations, schema introspection within a database
- **platform/** - Platform API (`/platform/*`) - Multi-tenant management (tenants, templates, jobs)
- **tools/** - Shared utilities (logging, auth middleware, rate limiting, CORS, HTTP helpers)
- **auth/**, **storage/**, **admin/** - Placeholder modules (planned features)

Middleware chain order: logging → timeout → cors → rate limit → auth → handler

### API Design

All CRUD operations go through `POST /data/query/{table}` with `Prefer` header specifying the operation:
- `Prefer: operation=select` - SELECT query
- `Prefer: operation=insert` - INSERT
- `Prefer: operation=update` - UPDATE
- `Prefer: operation=delete` - DELETE
- `Prefer: on-conflict=replace` - UPSERT
- `Prefer: on-conflict=ignore` - INSERT IGNORE

Multi-tenant queries use `Tenant` header to target tenant databases instead of primary.

### SDK Structure (`sdk/`)

TypeScript SDK with builder pattern:

- **AtomicbaseClient.ts** - Main entry point, `createClient()` factory
- **AtomicbaseBuilder.ts** - Base query builder
- **AtomicbaseQueryBuilder.ts** - SELECT queries with pagination, relations
- **AtomicbaseTransformBuilder.ts** - INSERT/UPDATE/DELETE with returning
- **filters.ts** - Filter functions (`eq`, `gt`, `or`, `not`, `fts`, etc.)
- **types.ts** - Response types, configuration interfaces
- **AtomicbaseError.ts** - Error wrapper

### Database Layer

Uses SQLite (local) or Turso (remote multi-tenant). Key concepts:
- Primary database stores templates and tenant metadata
- Tenant databases are separate SQLite/Turso databases
- Schema templates define reusable table structures
- FTS5 for full-text search (requires `-tags fts5` build flag)

## Error Handling

API errors return `{code, message, hint}` format. Common codes:
- `TABLE_NOT_FOUND`, `COLUMN_NOT_FOUND` - Schema errors (404)
- `MISSING_WHERE_CLAUSE` - DELETE/UPDATE requires where clause (400)
- `UNIQUE_VIOLATION` - Constraint violation (409)
- `TURSO_*` - Remote database errors

## Environment Variables

Key configuration (see README for full list):
- `PORT` - Server port (default `:8080`)
- `DB_PATH` - Primary database path
- `ATOMICBASE_API_KEY` - Auth key (empty = disabled)
- `TURSO_ORGANIZATION`, `TURSO_API_KEY` - For multi-tenant Turso deployments
