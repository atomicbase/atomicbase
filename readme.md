# Atomicbase

> [!IMPORTANT] > **Atomicbase is in early stages of development.** APIs may change.

Atomicbase is the backend for effortless multi-tenant architecture. It provides a complete backend solution on top of LibSQL and Turso with authentication, file storage, a dashboard, and client SDKs - all packaged as a single lightning-fast Go executable.

## Status

| Component      | Status   |
| -------------- | -------- |
| Database API   | Beta     |
| TypeScript SDK | Beta     |
| Authentication | Planning |
| File Storage   | Planning |
| Dashboard      | Planning |

## Structure

| Directory    | Description        |
| ------------ | ------------------ |
| [api](./api) | Go REST API server |
| [sdk](./sdk) | TypeScript SDK     |

## Features

### Available Now (Beta)

- **Database API**

  - PostgREST-style query syntax (filtering, ordering, pagination)
  - Nested relation queries with automatic joins
  - Full-text search (FTS5)
  - Aggregate functions (count, sum, avg, min, max)
  - Schema management (create, alter, drop tables)
  - Multi-database support (local SQLite + Turso)
  - Template-based tenant provisioning

- **Developer Experience**
  - API key authentication
  - Rate limiting
  - CORS configuration
  - OpenAPI documentation (`GET /openapi.yaml`)
  - TypeScript SDK with full type safety

### Coming Soon

- **Authentication** - User management, sessions, OAuth providers
- **File Storage** - S3-compatible object storage integration
- **Dashboard** - Web UI for database management and monitoring

## Quick Start

### API Server

```bash
cd api
go build -o atomicbase .
./atomicbase
```

The API runs on `http://localhost:8080` by default.

### TypeScript SDK

```bash
pnpm add @atomicbase/sdk
```

```typescript
import { createClient } from "@atomicbase/sdk";

const client = createClient({
  baseUrl: "http://localhost:8080",
  apiKey: "your-api-key", // optional
});

// Query data
const users = await client
  .from("users")
  .filter({ active: { eq: true } })
  .get();

// Insert data
await client
  .from("users")
  .insert({ name: "Alice", email: "alice@example.com" });

// Update data
await client.from("users").eq("id", 1).update({ name: "Alice Smith" });

// Delete data
await client.from("users").eq("id", 1).delete();

// Target a different database
const tenantClient = client.database("tenant-db-name");
```

## Architecture

Atomicbase is designed as a single binary that handles all backend concerns:

```
api/
├── database/    # Database API, schema management, queries
├── auth/        # Authentication (planned)
├── storage/     # File storage (planned)
├── admin/       # Dashboard backend (planned)
└── main.go      # Entry point
```

All modules share a common middleware stack (logging, CORS, rate limiting, auth) and are compiled into one executable for simple deployment.

## Development

```bash
# API (Go)
cd api
go test ./...
go build .

# SDK (TypeScript)
cd sdk
pnpm install
pnpm generate  # regenerate types from OpenAPI
pnpm build
```

## Contributing

Atomicbase is open source under the Apache-2.0 license.

- [Report issues](https://github.com/joe-ervin05/atomicbase/issues)
- [Contribute code](https://github.com/joe-ervin05/atomicbase)
