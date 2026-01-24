# Atomicbase

A complete backend solution for multi-tenant applications. Built on SQLite and Turso, packaged as a single Go executable.

## Status

| Component          | Status         |
| ------------------ | -------------- |
| Data API           | Beta           |
| TypeScript SDK     | Complete       |
| Schema Templates   | Complete       |
| CLI                | In Progress    |
| Authentication     | Planned        |
| File Storage       | Planned        |
| Dashboard          | Planned        |

## Quick Start

### Start the Server

```bash
cd api
make build && make run
# or: CGO_ENABLED=1 go build -tags fts5 -o atomicbase . && ./atomicbase
```

Server runs at `http://localhost:8080`. API docs at `/docs`.

### Install the SDK

```bash
npm install @atomicbase/sdk
```

### Basic Usage

```typescript
import { createClient, eq, gt } from "@atomicbase/sdk";

const client = createClient({
  url: "http://localhost:8080",
  apiKey: "your-api-key",
});

// Select with filters
const { data, error } = await client
  .from("users")
  .select("id", "name", "email")
  .where(eq("status", "active"), gt("age", 18))
  .orderBy("created_at", "desc")
  .limit(10);

// Insert
await client
  .from("users")
  .insert({ name: "Alice", email: "alice@example.com" });

// Update
await client
  .from("users")
  .update({ status: "inactive" })
  .where(eq("id", 5));

// Delete
await client.from("users").delete().where(eq("id", 5));
```

## TypeScript SDK

### Installation

```bash
npm install @atomicbase/sdk
# or
pnpm add @atomicbase/sdk
```

### Client Setup

```typescript
import { createClient } from "@atomicbase/sdk";

const client = createClient({
  url: "http://localhost:8080",
  apiKey: "your-api-key", // optional
  headers: { Tenant: "mydb" }, // optional: target tenant database
});
```

### Filtering

```typescript
import {
  eq, neq, gt, gte, lt, lte,
  like, glob, inArray, between,
  isNull, isNotNull, fts, not, or, and, col
} from "@atomicbase/sdk";

// Basic filters
const { data } = await client
  .from("users")
  .select()
  .where(eq("status", "active"))
  .where(gt("age", 21))
  .where(like("name", "%smith%"))
  .where(inArray("role", ["admin", "moderator"]))
  .where(between("price", 10, 100))
  .where(isNull("deleted_at"));

// OR conditions
const { data } = await client
  .from("users")
  .select()
  .where(or(eq("role", "admin"), eq("role", "moderator")));

// AND with OR
const { data } = await client
  .from("users")
  .select()
  .where(eq("status", "active"))
  .where(or(gt("age", 21), eq("verified", true)));

// Negation
const { data } = await client
  .from("users")
  .select()
  .where(not(eq("status", "banned")));

// Column-to-column comparison
const { data } = await client
  .from("posts")
  .select()
  .where(gt("updated_at", col("created_at")));

// Full-text search
const { data } = await client
  .from("articles")
  .select()
  .where(fts("sqlite database")); // search all indexed columns

const { data } = await client
  .from("articles")
  .select()
  .where(fts("title", "sqlite database")); // search specific column
```

### Result Modifiers

```typescript
// Get exactly one row (errors if 0 or multiple)
const { data, error } = await client
  .from("users")
  .select()
  .where(eq("id", 1))
  .single();

// Get zero or one row (null if not found)
const { data } = await client
  .from("users")
  .select()
  .where(eq("email", "test@example.com"))
  .maybeSingle();

// Get count only
const { data: count } = await client
  .from("users")
  .select()
  .where(eq("status", "active"))
  .count();

// Get data with total count (for pagination)
const { data, count } = await client
  .from("users")
  .select()
  .limit(10)
  .withCount();
console.log(`Showing ${data.length} of ${count} users`);
```

### Joins

#### Implicit Joins (via Foreign Keys)

```typescript
const { data } = await client
  .from("users")
  .select("id", "name", { posts: ["title", { comments: ["body"] }] });

// Returns nested JSON:
// [{ id: 1, name: "Alice", posts: [{ title: "Hello", comments: [...] }] }]
```

#### Explicit Joins

```typescript
import { onEq } from "@atomicbase/sdk";

// Left join - all users, with orders if they exist
const { data } = await client
  .from("users")
  .select("id", "name", "orders.total")
  .leftJoin("orders", onEq("users.id", "orders.user_id"));

// Inner join - only users with orders
const { data } = await client
  .from("users")
  .select("id", "name", "orders.total")
  .innerJoin("orders", onEq("users.id", "orders.user_id"));

// Multiple join conditions
const { data } = await client
  .from("users")
  .select("id", "name", "orders.total")
  .leftJoin("orders", [
    onEq("users.id", "orders.user_id"),
    onEq("users.tenant_id", "orders.tenant_id"),
  ]);

// Chained joins
const { data } = await client
  .from("users")
  .select("users.id", "orders.total", "products.name")
  .leftJoin("orders", onEq("users.id", "orders.user_id"))
  .leftJoin("products", onEq("orders.product_id", "products.id"));

// Flat output (no nesting)
const { data } = await client
  .from("users")
  .select("id", "name", "orders.total")
  .leftJoin("orders", onEq("users.id", "orders.user_id"), { flat: true });
// Returns: [{ id: 1, name: "Alice", orders_total: 100 }, ...]
```

### Insert Operations

```typescript
// Single insert
const { data } = await client.from("users").insert({ name: "Alice" });

// Bulk insert
const { data } = await client
  .from("users")
  .insert([{ name: "Alice" }, { name: "Bob" }]);

// Insert with returning
const { data } = await client
  .from("users")
  .insert({ name: "Alice" })
  .returning("id", "created_at");

// Upsert (insert or update on conflict)
const { data } = await client
  .from("users")
  .upsert({ id: 1, name: "Alice Updated" });

// Insert ignore (skip on conflict)
const { data } = await client
  .from("users")
  .insert({ id: 1, name: "Alice" })
  .onConflict("ignore");
```

### Batch Operations

Execute multiple operations atomically:

```typescript
const { data, error } = await client.batch([
  client.from("users").insert({ name: "Alice" }),
  client.from("users").insert({ name: "Bob" }),
  client.from("counters").update({ count: 2 }).where(eq("id", 1)),
]);

// With result modifiers
const { data, error } = await client.batch([
  client.from("users").select().where(eq("id", 1)).single(),
  client.from("users").select().count(),
  client.from("posts").select().limit(10).withCount(),
]);
// data.results[0] = { id: 1, name: 'Alice' }
// data.results[1] = 42
// data.results[2] = { data: [...], count: 100 }
```

### Error Handling

```typescript
const { data, error } = await client.from("users").select();

if (error) {
  console.error(error.code); // "TABLE_NOT_FOUND"
  console.error(error.message); // Human-readable message
  console.error(error.hint); // Actionable guidance
}

// Or use throwOnError() for try/catch style
try {
  const { data } = await client.from("users").select().throwOnError();
} catch (error) {
  // error is AtomicbaseError
}
```

### Multi-Tenant

```typescript
// Switch to a tenant database
const tenantClient = client.tenant("acme-corp");
const { data } = await tenantClient.from("users").select();
```

## REST API

All operations use `POST /data/query/{table}` with a `Prefer` header:

```http
POST /data/query/users
Prefer: operation=select
Content-Type: application/json

{
  "select": ["id", "name"],
  "where": [{ "status": { "eq": "active" } }],
  "order": { "created_at": "desc" },
  "limit": 20
}
```

### Operations

| Operation | Prefer Header                |
| --------- | ---------------------------- |
| SELECT    | `operation=select`           |
| INSERT    | `operation=insert`           |
| UPSERT    | `operation=insert, on-conflict=replace` |
| UPDATE    | `operation=update`           |
| DELETE    | `operation=delete`           |

### Batch

```http
POST /data/batch

{
  "operations": [
    { "operation": "insert", "table": "users", "body": { "data": { "name": "Alice" } } },
    { "operation": "update", "table": "counters", "body": { "data": { "count": 2 }, "where": [{ "id": { "eq": 1 } }] } }
  ]
}
```

## Schema Templates

Define reusable schemas and sync them across tenant databases:

```http
# Create a template
POST /platform/templates
{
  "name": "saas-app",
  "tables": {
    "users": {
      "columns": {
        "id": { "type": "INTEGER", "primary": true },
        "email": { "type": "TEXT", "unique": true },
        "name": { "type": "TEXT" }
      }
    },
    "posts": {
      "columns": {
        "id": { "type": "INTEGER", "primary": true },
        "title": { "type": "TEXT" },
        "user_id": { "type": "INTEGER", "references": "users(id)" }
      }
    }
  }
}

# Sync template to all tenant databases
POST /platform/templates/saas-app/sync
```

## Configuration

| Variable                        | Default                 | Description                         |
| ------------------------------- | ----------------------- | ----------------------------------- |
| `PORT`                          | `:8080`                 | HTTP server port                    |
| `DB_PATH`                       | `atomicdata/primary.db` | Path to primary SQLite database     |
| `ATOMICBASE_API_KEY`            | (empty)                 | API key (empty = disabled)          |
| `ATOMICBASE_RATE_LIMIT_ENABLED` | `false`                 | Enable rate limiting                |
| `ATOMICBASE_RATE_LIMIT`         | `100`                   | Requests per minute per IP          |
| `ATOMICBASE_CORS_ORIGINS`       | (empty)                 | Allowed origins (comma-separated)   |
| `ATOMICBASE_REQUEST_TIMEOUT`    | `30`                    | Request timeout in seconds          |
| `TURSO_ORGANIZATION`            | (empty)                 | Turso org (for multi-tenant)        |
| `TURSO_API_KEY`                 | (empty)                 | Turso API key                       |

## Error Codes

| Code                   | Status | Description                          |
| ---------------------- | ------ | ------------------------------------ |
| `TABLE_NOT_FOUND`      | 404    | Table doesn't exist                  |
| `COLUMN_NOT_FOUND`     | 404    | Column doesn't exist                 |
| `UNIQUE_VIOLATION`     | 409    | Duplicate unique value               |
| `MISSING_WHERE_CLAUSE` | 400    | DELETE/UPDATE requires where         |
| `NOT_FOUND`            | 404    | single() returned no rows            |
| `MULTIPLE_ROWS`        | 400    | single() returned multiple rows      |
| `NETWORK_ERROR`        | 0      | Network request failed               |

## Architecture

```
api/
├── main.go          # Entry point, server setup
├── config/          # Environment configuration
├── data/            # Data API (queries, schema, batch)
├── platform/        # Platform API (tenants, templates)
└── tools/           # Middleware, errors, utilities

packages/sdk/
├── src/
│   ├── AtomicbaseClient.ts      # Client factory
│   ├── AtomicbaseBuilder.ts     # Base builder (filters, transforms, execution)
│   ├── AtomicbaseQueryBuilder.ts # Query operations (select, insert, etc.)
│   ├── AtomicbaseError.ts       # Error class
│   ├── filters.ts               # Filter helpers (eq, or, fts, onEq, etc.)
│   ├── types.ts                 # TypeScript types
│   └── index.ts                 # Exports
└── package.json
```

## Development

```bash
# API
cd api
make test    # Run tests
make build   # Build binary
make run     # Run server

# SDK
cd packages/sdk
pnpm install
pnpm build   # Build
pnpm dev     # Watch mode
```

## Roadmap

- **CLI** _(in progress)_ - Template management, schema syncing, migrations
- **Dashboard** - Web UI for management and monitoring
- **Authentication** - User management, sessions, OAuth
- **File Storage** - S3-compatible object storage

## License

Apache-2.0

## Links

- [GitHub](https://github.com/joe-ervin05/atomicbase)
- [Issues](https://github.com/joe-ervin05/atomicbase/issues)
