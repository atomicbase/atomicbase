# Atomicbase

> [!IMPORTANT]
> **Atomicbase is in early stages of development.** APIs may change.

Atomicbase is a REST API for SQLite and Turso databases, designed for efficient multi-tenancy.

## Structure

| Directory | Description |
|-----------|-------------|
| [api](./api) | Go REST API server |
| [sdk](./sdk) | TypeScript SDK |

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

## Features

- PostgREST-style query syntax (filtering, ordering, pagination)
- Nested relation queries
- Multi-database support (local SQLite + Turso)
- API key authentication
- Rate limiting
- CORS configuration
- OpenAPI documentation (`GET /docs`)

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

Atomicbase is open source under the MIT license.

- [Report issues](https://github.com/joe-ervin05/atomicbase/issues)
- [Contribute code](https://github.com/joe-ervin05/atomicbase)
