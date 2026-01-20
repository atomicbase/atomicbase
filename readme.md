# Atomicbase

## **Atomicbase is in early stages of development.** APIs may change.

Atomicbase is the backend for effortless multi-tenant architecture. It provides a complete backend solution on top of SQLite and Turso with authentication, file storage, a dashboard, and client SDKs - all packaged as a single lightning-fast Go executable.

## Status

| Component        | Status   |
| ---------------- | -------- |
| Database API     | Beta     |
| TypeScript SDK   | Beta     |
| Schema Templates | Beta     |
| Authentication   | Planning |
| File Storage     | Planning |
| Dashboard        | Planning |

## Quick Start

### API

```bash
cd api
CGO_ENABLED=1 go build -tags fts5 -o atomicbase .
./atomicbase
```

The API runs on `http://localhost:8080` by default.

### SDK

```bash
npm install @atomicbase/sdk
```

```typescript
import { createClient, eq, fts, onEq } from '@atomicbase/sdk'

const client = createClient({
  url: 'http://localhost:8080',
  apiKey: 'your-api-key'
})

// Select with filters
const { data, error } = await client
  .from('users')
  .select('id', 'name', 'email')
  .where(eq('status', 'active'))
  .orderBy('created_at', 'desc')
  .limit(10)

// Insert
const { data } = await client
  .from('users')
  .insert({ name: 'Alice', email: 'alice@example.com' })
  .returning('id')

// Update
const { data } = await client
  .from('users')
  .update({ status: 'inactive' })
  .where(eq('id', 5))

// Delete
const { data } = await client
  .from('users')
  .delete()
  .where(eq('id', 5))
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
import { createClient } from '@atomicbase/sdk'

const client = createClient({
  url: 'http://localhost:8080',
  apiKey: 'your-api-key',        // optional
  headers: { 'Tenant': 'mydb' }  // optional: target tenant database
})
```

### Filtering

```typescript
import { eq, neq, gt, gte, lt, lte, like, glob, inArray, between, isNull, isNotNull, fts, not, or, and, col } from '@atomicbase/sdk'

// Basic filters
.where(eq('status', 'active'))
.where(gt('age', 21))
.where(like('name', '%smith%'))
.where(inArray('role', ['admin', 'moderator']))
.where(between('price', 10, 100))
.where(isNull('deleted_at'))

// Negation
.where(not(eq('status', 'banned')))
.where(isNotNull('email'))

// OR conditions
.where(or(eq('role', 'admin'), eq('role', 'moderator')))

// AND conditions (multiple .where() calls are ANDed)
.where(eq('status', 'active'))
.where(gt('age', 18))

// Column-to-column comparison
.where(gt('updated_at', col('created_at')))

// Full-text search
.where(fts('hello world'))              // search all indexed columns
.where(fts('title', 'hello world'))     // search specific column
```

### Nested Relations (Implicit Joins)

Auto-join via foreign keys:

```typescript
const { data } = await client
  .from('users')
  .select('id', 'name', { posts: ['title', { comments: ['body'] }] })
```

Returns nested JSON:

```json
[{"id": 1, "name": "Alice", "posts": [{"title": "Hello", "comments": [...]}]}]
```

### Custom Joins (Explicit Joins)

For joins without FK relationships or with custom conditions:

```typescript
import { onEq, onGt, onLt, onGte, onLte, onNeq } from '@atomicbase/sdk'

// Left join (default) - all users, with orders if they exist
const { data } = await client
  .from('users')
  .select('id', 'name', 'orders.total', 'orders.created_at')
  .leftJoin('orders', onEq('users.id', 'orders.user_id'))

// Inner join - only users with orders
const { data } = await client
  .from('users')
  .select('id', 'name', 'orders.total')
  .innerJoin('orders', onEq('users.id', 'orders.user_id'))

// Multiple join conditions
const { data } = await client
  .from('users')
  .select('id', 'name', 'orders.total')
  .leftJoin('orders', [
    onEq('users.id', 'orders.user_id'),
    onEq('users.tenant_id', 'orders.tenant_id')
  ])

// Chained joins
const { data } = await client
  .from('users')
  .select('users.id', 'users.name', 'orders.total', 'products.name')
  .leftJoin('orders', onEq('users.id', 'orders.user_id'))
  .leftJoin('products', onEq('orders.product_id', 'products.id'))

// Flat output (no nesting)
const { data } = await client
  .from('users')
  .select('id', 'name', 'orders.total')
  .leftJoin('orders', onEq('users.id', 'orders.user_id'), { flat: true })
// Returns: [{id: 1, name: "Alice", orders_total: 100}, {id: 1, name: "Alice", orders_total: 50}]
```

### Result Modifiers

```typescript
// Get exactly one row (errors if 0 or multiple)
const { data, error } = await client.from('users').select().where(eq('id', 1)).single()

// Get zero or one row (null if not found)
const { data, error } = await client.from('users').select().where(eq('email', 'test@example.com')).maybeSingle()

// Get count only
const { data: count } = await client.from('users').select().where(eq('status', 'active')).count()

// Get data with total count
const { data, count } = await client.from('users').select().limit(10).withCount()
```

### Insert Operations

```typescript
// Single insert
const { data } = await client.from('users').insert({ name: 'Alice' })

// Bulk insert
const { data } = await client.from('users').insert([
  { name: 'Alice' },
  { name: 'Bob' }
])

// Insert with returning
const { data } = await client.from('users').insert({ name: 'Alice' }).returning('id', 'created_at')

// Upsert (insert or update on conflict)
const { data } = await client.from('users').upsert({ id: 1, name: 'Alice Updated' })

// Insert ignore (skip on conflict)
const { data } = await client.from('users').insert({ id: 1, name: 'Alice' }).onConflict('ignore')
```

### Batch Operations

Execute multiple operations atomically in a single request:

```typescript
const { data, error } = await client.batch([
  client.from('users').insert({ name: 'Alice' }),
  client.from('users').insert({ name: 'Bob' }),
  client.from('counters').update({ count: 2 }).where(eq('id', 1)),
])

// With result modifiers
const { data, error } = await client.batch([
  client.from('users').select().where(eq('id', 1)).single(),
  client.from('users').select().count(),
  client.from('posts').select().limit(10).withCount(),
])
// data.results[0] = { id: 1, name: 'Alice' }  (single object)
// data.results[1] = 42  (count number)
// data.results[2] = { data: [...], count: 100 }  (data with count)
```

### Error Handling

```typescript
const { data, error } = await client.from('users').select()

if (error) {
  console.error(error.code)    // e.g., "TABLE_NOT_FOUND"
  console.error(error.message) // Human-readable message
  console.error(error.hint)    // Actionable guidance
}
```

## REST API

All CRUD operations go through `/data/query/{table}` with `Prefer` header to specify behavior.

**Select:**

```http
POST /data/query/users
Prefer: operation=select

{"select": ["id", "name"], "where": [{"status": {"eq": "active"}}], "limit": 20}
```

**Insert:**

```http
POST /data/query/users

{"data": {"name": "Alice", "email": "alice@example.com"}, "returning": ["id"]}
```

**Update:**

```http
PATCH /data/query/users

{"data": {"status": "inactive"}, "where": [{"id": {"eq": 5}}]}
```

**Delete:**

```http
DELETE /data/query/users

{"where": [{"id": {"eq": 5}}]}
```

### Custom Joins (REST API)

```http
POST /data/query/users
Prefer: operation=select

{
  "select": ["id", "name", "orders.total", "orders.created_at"],
  "join": [
    {
      "table": "orders",
      "type": "left",
      "on": [{"users.id": {"eq": "orders.user_id"}}],
      "flat": false
    }
  ],
  "where": [{"status": {"eq": "active"}}]
}
```

### Filtering

`where` is always an array. Elements are ANDed together.

| Operator                              | Example                              |
| ------------------------------------- | ------------------------------------ |
| `eq`, `neq`, `gt`, `gte`, `lt`, `lte` | `{"age": {"gte": 21}}`               |
| `like`, `glob`                        | `{"name": {"like": "%smith%"}}`      |
| `in`, `between`                       | `{"status": {"in": ["a", "b"]}}`     |
| `is`, `fts`                           | `{"deleted_at": {"is": null}}`       |
| `not`                                 | `{"name": {"not": {"eq": "Admin"}}}` |

**OR conditions:**

```json
{
  "where": [
    { "org_id": { "eq": 5 } },
    { "or": [{ "status": { "eq": "active" } }, { "role": { "eq": "admin" } }] }
  ]
}
```

### Full-Text Search

Create an FTS index, then query with `fts` operator:

```http
POST /data/schema/fts/articles
{"columns": ["title", "content"]}
```

```http
POST /data/query/articles
Prefer: operation=select

# Search all indexed columns
{"select": ["*"], "where": [{"__fts": {"fts": "sqlite database"}}]}

# Search specific column
{"select": ["*"], "where": [{"title": {"fts": "sqlite database"}}]}
```

### Batch Operations

Execute multiple operations atomically in a single request:

```http
POST /data/batch

{
  "operations": [
    {"operation": "insert", "table": "users", "body": {"data": {"name": "Alice"}}},
    {"operation": "insert", "table": "users", "body": {"data": {"name": "Bob"}}},
    {"operation": "update", "table": "counters", "body": {"data": {"count": 2}, "where": [{"id": {"eq": 1}}]}}
  ]
}
```

All operations succeed or all rollback. Supported operations: `select`, `insert`, `upsert`, `update`, `delete`.

For complete API documentation, see [docs/database_api_design.md](./docs/database_api_design.md).

## Features

### Database API

- JSON query syntax (filtering, ordering, pagination)
- Nested relation queries with automatic FK-based joins
- Custom explicit joins (LEFT, INNER)
- Full-text search (FTS5)
- Batch operations (atomic transactions)
- Schema management (create, alter, drop tables)
- Multi-database support (local SQLite + Turso)

### TypeScript SDK

- Type-safe query builder with fluent API
- Filter functions (eq, gt, like, fts, etc.)
- Join functions (onEq, onGt, etc.)
- Result modifiers (single, maybeSingle, count, withCount)
- Batch operations (atomic transactions)
- Discriminated union responses for type-safe error handling

### Schema Templates

- Define reusable table schemas as templates
- Associate databases with templates
- Sync templates to databases (create missing tables, add missing columns)
- Bulk sync across all tenant databases

### Developer Experience

- API key authentication
- Rate limiting
- CORS configuration
- Request timeout control
- OpenAPI documentation (`GET /openapi.yaml`, `GET /docs`)
- Graceful shutdown

## Configuration

Atomicbase is configured via environment variables (or `.env` file):

| Variable                        | Default                 | Description                         |
| ------------------------------- | ----------------------- | ----------------------------------- |
| `PORT`                          | `:8080`                 | HTTP server port                    |
| `DB_PATH`                       | `atomicdata/primary.db` | Path to primary SQLite database     |
| `DATA_DIR`                      | `atomicdata`            | Directory for database files        |
| `ATOMICBASE_API_KEY`            | (empty)                 | API key for auth (empty = disabled) |
| `ATOMICBASE_RATE_LIMIT_ENABLED` | `false`                 | Enable rate limiting                |
| `ATOMICBASE_RATE_LIMIT`         | `100`                   | Requests per minute per IP          |
| `ATOMICBASE_CORS_ORIGINS`       | (empty)                 | Comma-separated allowed origins     |
| `ATOMICBASE_REQUEST_TIMEOUT`    | `30`                    | Request timeout in seconds          |
| `ATOMICBASE_MAX_QUERY_DEPTH`    | `5`                     | Maximum nested relation depth       |
| `ATOMICBASE_MAX_QUERY_LIMIT`    | `1000`                  | Maximum rows per query              |
| `ATOMICBASE_DEFAULT_LIMIT`      | `100`                   | Default limit when not specified    |
| `TURSO_ORGANIZATION`            | (empty)                 | Turso organization (multi-tenant)   |
| `TURSO_API_KEY`                 | (empty)                 | Turso API key (multi-tenant)        |
| `TURSO_TOKEN_EXPIRATION`        | `7d`                    | Token expiration: `7d`, `30d`, `never` |

## Error Handling

All errors return a consistent JSON structure with a stable `code` for programmatic handling, a human-readable `message`, and an optional `hint` with actionable guidance.

```json
{
  "code": "TABLE_NOT_FOUND",
  "message": "table not found in schema: users",
  "hint": "Verify the table name is spelled correctly. Use GET /schema to list available tables."
}
```

### Error Codes

**Resource Errors (404)**

| Code                 | Description                                  |
| -------------------- | -------------------------------------------- |
| `TABLE_NOT_FOUND`    | Table doesn't exist in schema                |
| `COLUMN_NOT_FOUND`   | Column doesn't exist in table                |
| `DATABASE_NOT_FOUND` | Tenant database not found                    |
| `TEMPLATE_NOT_FOUND` | Schema template not found                    |
| `NO_RELATIONSHIP`    | No foreign key between tables for auto-join  |

**Validation Errors (400)**

| Code                   | Description                               |
| ---------------------- | ----------------------------------------- |
| `INVALID_OPERATOR`     | Unknown filter operator in where clause   |
| `INVALID_COLUMN_TYPE`  | Invalid type in schema definition         |
| `INVALID_IDENTIFIER`   | Table/column name contains invalid chars  |
| `MISSING_WHERE_CLAUSE` | DELETE/UPDATE requires where clause       |
| `QUERY_TOO_DEEP`       | Nested relations exceed max depth         |
| `ARRAY_TOO_LARGE`      | IN clause exceeds 100 elements            |
| `NOT_DDL_QUERY`        | Only CREATE/ALTER/DROP allowed for schema |
| `NO_FTS_INDEX`         | Full-text search requires FTS index       |

**Constraint Errors**

| Code                   | Status | Description                       |
| ---------------------- | ------ | --------------------------------- |
| `UNIQUE_VIOLATION`     | 409    | Record with unique value exists   |
| `FOREIGN_KEY_VIOLATION`| 400    | Referenced record doesn't exist   |
| `NOT_NULL_VIOLATION`   | 400    | Required field is missing         |
| `TEMPLATE_IN_USE`      | 409    | Template has associated databases |
| `RESERVED_TABLE`       | 403    | Cannot query internal tables      |

**Turso Errors**

| Code                   | Status | Description                           |
| ---------------------- | ------ | ------------------------------------- |
| `TURSO_CONFIG_MISSING` | 503    | Missing TURSO_ORGANIZATION or API_KEY |
| `TURSO_AUTH_FAILED`    | 401    | Invalid or expired API key/token      |
| `TURSO_FORBIDDEN`      | 403    | API key lacks permission              |
| `TURSO_NOT_FOUND`      | 404    | Database/org not found in Turso       |
| `TURSO_RATE_LIMITED`   | 429    | Too many Turso API requests           |
| `TURSO_CONNECTION_ERROR`| 502   | Cannot reach Turso database           |
| `TURSO_TOKEN_EXPIRED`  | 401    | Database token has expired            |
| `TURSO_SERVER_ERROR`   | 502    | Turso service temporarily unavailable |

**Other**

| Code             | Status | Description                          |
| ---------------- | ------ | ------------------------------------ |
| `INTERNAL_ERROR` | 500    | Unexpected server error (check logs) |

## API Endpoints

| Category  | Endpoint                                         | Methods                  |
| --------- | ------------------------------------------------ | ------------------------ |
| Query     | `/data/query/{table}`                            | POST, PATCH, DELETE      |
| Batch     | `/data/batch`                                    | POST                     |
| Schema    | `/data/schema`                                   | GET                      |
| Schema    | `/data/schema/table/{table}`                     | GET, POST, PATCH, DELETE |
| FTS       | `/data/schema/fts`, `/data/schema/fts/{table}`   | GET, POST, DELETE        |
| Tenants   | `/platform/tenants`, `/platform/tenants/{name}`  | GET, POST, PATCH, DELETE |
| Templates | `/platform/templates`, `/platform/templates/{name}` | GET, POST, PUT, DELETE |
| Sync      | `/platform/templates/{name}/sync`, `/platform/tenants/{name}/sync` | POST |
| Health    | `/health`                                        | GET                      |
| Docs      | `/openapi.yaml`, `/docs`                         | GET                      |

**Headers:**

- `Authorization: Bearer <key>` - API authentication
- `Tenant: <name>` - Target tenant database (default: primary)
- `Prefer: operation=select` - SELECT query
- `Prefer: on-conflict=replace` - UPSERT behavior
- `Prefer: on-conflict=ignore` - INSERT IGNORE behavior
- `Prefer: count=exact` - Include total count in `X-Total-Count` header

## Architecture

```
api/
├── main.go          # Entry point, server setup, graceful shutdown
├── config/          # Environment-based configuration
├── data/            # Data API (queries, schema, FTS, batch)
├── platform/        # Platform API (tenants, templates, sync)
├── tools/           # Shared utilities (middleware, errors, validation)
├── auth/            # Authentication (planned)
├── storage/         # File storage (planned)
└── admin/           # Dashboard backend (planned)

sdk/
├── src/
│   ├── AtomicbaseClient.ts       # Main client factory
│   ├── AtomicbaseQueryBuilder.ts # Query builder with CRUD methods
│   ├── AtomicbaseTransformBuilder.ts # Result modifiers (single, count, etc.)
│   ├── filters.ts                # Filter functions (eq, gt, fts, onEq, etc.)
│   ├── types.ts                  # TypeScript type definitions
│   └── index.ts                  # Public exports
└── package.json
```

## Development

```bash
# API
cd api
CGO_ENABLED=1 go test -tags fts5 ./...
CGO_ENABLED=1 go build -tags fts5 .

# SDK
cd sdk
pnpm install
pnpm build
pnpm dev  # watch mode
```

## Coming Soon

- **Authentication** - User management, sessions, OAuth providers
- **File Storage** - S3-compatible object storage integration
- **Dashboard** - Web UI for database management and monitoring

## Contributing

Atomicbase is open source under the Apache-2.0 license.

- [Report issues](https://github.com/joe-ervin05/atomicbase/issues)
- [Contribute code](https://github.com/joe-ervin05/atomicbase)
