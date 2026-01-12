# Atomicbase Database API

A PostgREST-style REST API for SQLite and Turso databases with support for multi-database management, full-text search, and schema templates.

## Table of Contents

- [Current Features](#current-features)
  - [CRUD Operations](#crud-operations)
  - [Query & Filtering](#query--filtering)
  - [Nested Relations](#nested-relations)
  - [Aggregations](#aggregations)
  - [Full-Text Search (FTS5)](#full-text-search-fts5)
  - [Schema Management](#schema-management)
  - [Multi-Database Support](#multi-database-support)
  - [Schema Templates](#schema-templates)
  - [Security & Middleware](#security--middleware)
- [Upcoming Features](#upcoming-features)
  - [Batch Transactions](#batch-transactions)
  - [Complex Joins](#complex-joins)
  - [Views](#views)
- [API Reference](#api-reference)

---

## Current Features

### CRUD Operations

Full create, read, update, delete operations on any table.

| Method   | Endpoint         | Description                                      |
| -------- | ---------------- | ------------------------------------------------ |
| `GET`    | `/query/{table}` | Select rows with filtering, ordering, pagination |
| `POST`   | `/query/{table}` | Insert a single row                              |
| `PATCH`  | `/query/{table}` | Update rows matching WHERE conditions            |
| `DELETE` | `/query/{table}` | Delete rows matching WHERE conditions            |

**Insert with return:**

```bash
POST /query/users?select=id,name
{"name": "Alice", "email": "alice@example.com"}
# Returns: [{"id": 1, "name": "Alice"}]
```

**Upsert (bulk insert/update):**

```bash
POST /query/users
Prefer: resolution=merge-duplicates

[{"id": 1, "name": "Alice Updated"}, {"id": 2, "name": "Bob"}]
# Returns: {"rows_affected": 2}
```

**Delete requires WHERE clause** (prevents accidental mass deletes):

```bash
DELETE /query/users?id=eq.5
# Returns: {"rows_affected": 1}
```

---

### Query & Filtering

Filter rows using URL query parameters with operator syntax: `column=operator.value`

#### Comparison Operators

| Operator | SQL  | Example              |
| -------- | ---- | -------------------- |
| `eq`     | `=`  | `status=eq.active`   |
| `neq`    | `!=` | `status=neq.deleted` |
| `gt`     | `>`  | `age=gt.21`          |
| `gte`    | `>=` | `price=gte.100`      |
| `lt`     | `<`  | `age=lt.65`          |
| `lte`    | `<=` | `stock=lte.10`       |

#### Pattern Matching

| Operator | SQL    | Example                  |
| -------- | ------ | ------------------------ |
| `like`   | `LIKE` | `name=like.%smith%`      |
| `glob`   | `GLOB` | `email=glob.*@gmail.com` |

#### Other Operators

| Operator  | SQL              | Example                             |
| --------- | ---------------- | ----------------------------------- |
| `in`      | `IN`             | `status=in.(active,pending,review)` |
| `between` | `BETWEEN`        | `age=between.18,65`                 |
| `is`      | `IS`             | `deleted_at=is.null`                |
| `fts`     | Full-text search | `title=fts.search terms`            |

#### Negation

Prefix any operator with `not.` to negate:

```
name=not.eq.Admin           # name != 'Admin'
status=not.in.(deleted,banned)  # status NOT IN (...)
email=not.like.%test%       # email NOT LIKE '%test%'
deleted_at=not.is.null      # deleted_at IS NOT NULL
```

#### Logical OR

Combine conditions with OR using the `or` parameter:

```
GET /query/users?or=(status.eq.active,status.eq.pending)
# WHERE status = 'active' OR status = 'pending'
```

#### Ordering & Pagination

```
GET /query/users?order=created_at:desc&limit=20&offset=40
```

- `order` - Sort by column: `column:asc` or `column:desc`
- `limit` - Max rows to return (default: 100, max: 1000)
- `offset` - Skip N rows for pagination

#### Row Counting

Get total count alongside results:

```bash
GET /query/users?status=eq.active
Prefer: count=exact

# Response header: X-Total-Count: 1523
# Body: [{...}, {...}, ...]
```

Get count only (no data):

```bash
GET /query/users?status=eq.active&count=only
# Returns: {"count": 1523}
```

---

### Nested Relations

Automatically join related tables via foreign key relationships.

```
GET /query/users?select=id,name,posts(title,created_at)
```

Returns:

```json
[
  {
    "id": 1,
    "name": "Alice",
    "posts": [
      { "title": "Hello World", "created_at": "2024-01-15" },
      { "title": "Second Post", "created_at": "2024-01-20" }
    ]
  }
]
```

Nested relations work in both directions:

- Parent → Children: `users?select=*,posts(*)`
- Child → Parent: `posts?select=*,users(name)`

---

### Aggregations

Use aggregate functions in the `select` parameter:

| Function      | Example            |
| ------------- | ------------------ |
| `count(*)`    | Count all rows     |
| `sum(column)` | Sum numeric column |
| `avg(column)` | Average of column  |
| `min(column)` | Minimum value      |
| `max(column)` | Maximum value      |

**Basic aggregation:**

```
GET /query/orders?select=count(*),sum(total)
# Returns: [{"count": 150, "sum": 45230.50}]
```

**Group by with aggregates:**

```
GET /query/orders?select=status,count(*),avg(total)
# Returns: [
#   {"status": "pending", "count": 23, "avg": 150.00},
#   {"status": "completed", "count": 127, "avg": 320.50}
# ]
```

**Aliasing:**

```
GET /query/orders?select=status,order_count:count(*),total_revenue:sum(total)
```

---

### Full-Text Search (FTS5)

Create FTS5 indexes for fast text search with automatic synchronization.

**Create an FTS index:**

```bash
POST /schema/fts/articles
{"columns": ["title", "content"]}
```

This creates:

- FTS5 virtual table `articles_fts`
- Insert/update/delete triggers to keep index in sync

**Search using FTS:**

```
GET /query/articles?title=fts.sqlite database
```

FTS5 supports advanced syntax:

- `word1 word2` - Both words (AND)
- `word1 OR word2` - Either word
- `"exact phrase"` - Phrase match
- `word*` - Prefix match
- `NEAR(word1 word2, 5)` - Words within 5 tokens

**List FTS indexes:**

```bash
GET /schema/fts
# Returns: [{"table": "articles", "ftsTable": "articles_fts", "columns": ["title", "content"]}]
```

**Drop FTS index:**

```bash
DELETE /schema/fts/articles
```

---

### Schema Management

#### Get Schema

```bash
GET /schema
# Returns all tables with columns and primary keys
```

#### Create Table

```bash
POST /schema/table/users
{
  "id": {"type": "integer", "primaryKey": true},
  "email": {"type": "text", "unique": true, "notNull": true},
  "name": {"type": "text"},
  "org_id": {"type": "integer", "references": "organizations.id", "onDelete": "cascade"}
}
```

Column options:

- `type`: `text`, `integer`, `real`, `blob`
- `primaryKey`: `true/false`
- `unique`: `true/false`
- `notNull`: `true/false`
- `default`: default value
- `references`: `"table.column"` for foreign keys
- `onDelete`/`onUpdate`: `cascade`, `restrict`, `set null`, `set default`, `no action`

#### Alter Table

```bash
PATCH /schema/table/users
{
  "newName": "customers",           # Rename table
  "renameColumns": {"old": "new"},  # Rename columns
  "newColumns": {...},              # Add columns
  "dropColumns": ["temp_col"]       # Drop columns
}
```

#### Drop Table

```bash
DELETE /schema/table/users
```

#### Execute Raw DDL

```bash
POST /schema
{"query": "CREATE INDEX idx_users_email ON users(email)"}
```

Only DDL statements (CREATE, ALTER, DROP) are allowed.

#### Invalidate Schema Cache

```bash
POST /schema/invalidate
```

---

### Multi-Database Support

Atomicbase supports both a local SQLite database and multiple external Turso databases.

**Target a specific database:**

```bash
GET /query/users
DB-Name: my-turso-db
```

**List registered databases:**

```bash
GET /db
```

**Create a new Turso database:**

```bash
POST /db
{"name": "my-new-db", "group": "default"}
```

**Register an existing Turso database:**

```bash
PATCH /db
{"name": "existing-db"}
```

**Register all organization databases:**

```bash
PATCH /db/all
```

**Delete a database:**

```bash
DELETE /db/my-db
```

---

### Schema Templates

Define reusable schemas for multi-tenant database management.

**Create a template:**

```bash
POST /templates
{
  "name": "saas-app",
  "tables": [
    {
      "name": "users",
      "pk": "id",
      "columns": [
        {"name": "id", "type": "INTEGER"},
        {"name": "email", "type": "TEXT"},
        {"name": "name", "type": "TEXT"}
      ]
    }
  ]
}
```

**Associate a database with a template:**

```bash
PUT /db/tenant-123/template
{"templateName": "saas-app"}
```

**Sync a database to its template:**

```bash
POST /db/tenant-123/sync?dropExtra=true
```

**Sync all databases using a template:**

```bash
POST /templates/saas-app/sync?dropExtra=false
```

---

### Security & Middleware

#### Authentication

Set `ATOMICBASE_API_KEY` to enable Bearer token authentication:

```bash
GET /query/users
Authorization: Bearer your-api-key
```

Public endpoints (no auth required): `/health`, `/openapi.yaml`, `/docs`

#### Rate Limiting

Enable with `ATOMICBASE_RATE_LIMIT_ENABLED=true`.

Configure requests per minute with `ATOMICBASE_RATE_LIMIT` (default: 100).

#### CORS

Set `ATOMICBASE_CORS_ORIGINS` to allowed origins:

- `*` for all origins
- Comma-separated list: `https://app.example.com,https://admin.example.com`

#### Request Timeout

Configure with `ATOMICBASE_REQUEST_TIMEOUT` in seconds (default: 30).

#### Query Depth Limit

Prevent deeply nested queries with `ATOMICBASE_MAX_QUERY_DEPTH` (default: 4).

#### Other Security Features

- Parameterized queries prevent SQL injection
- Constant-time API key comparison prevents timing attacks
- Request body size limits prevent memory exhaustion
- Structured JSON logging with request IDs for audit trails

---

## Upcoming Features

### Batch Transactions

> **Status:** Planned

Execute multiple operations atomically in a single request.

#### API Design

```bash
POST /batch
{
  "atomic": true,
  "operations": [
    {"method": "POST", "path": "/query/users", "body": {"name": "Alice"}},
    {"method": "POST", "path": "/query/accounts", "body": {"user_id": "$0.last_insert_id", "balance": 0}},
    {"method": "POST", "path": "/query/audit_log", "body": {"action": "user_created"}}
  ]
}
```

**Response:**

```json
{
  "results": [
    { "status": 201, "body": { "last_insert_id": 5 } },
    { "status": 201, "body": { "last_insert_id": 12 } },
    { "status": 201, "body": { "last_insert_id": 99 } }
  ]
}
```

#### Placeholder References

Reference results from previous operations using `$N.field` syntax:

| Placeholder         | Description                             |
| ------------------- | --------------------------------------- |
| `$0.last_insert_id` | Last insert ID from operation 0         |
| `$1.rows_affected`  | Rows affected by operation 1            |
| `$2.body.id`        | Field from returned body of operation 2 |

#### SDK Experience

The SDK will provide a fluent transaction builder:

```typescript
const tx = client.startTx();

// Operations queue locally (no network calls)
await tx.from("users").insert({ name: "Alice" });
await tx.from("accounts").insert({
  user_id: tx.ref(0, "last_insert_id"), // Reference previous result
  balance: 0,
});
await tx.from("audit_log").insert({ action: "user_created" });

// Single network request, atomic execution
const results = await tx.commit();

// Or discard without sending
tx.rollback();
```

#### Implementation Notes

**Server-side execution:**

```go
func (dao *Database) ExecuteBatch(ctx context.Context, batch BatchRequest) ([]Result, error) {
    tx, err := dao.Client.BeginTx(ctx, nil)
    if err != nil {
        return nil, err
    }
    defer tx.Rollback()

    results := make([]Result, len(batch.Operations))

    for i, op := range batch.Operations {
        // Resolve $N.field placeholders from previous results
        resolvedBody := resolvePlaceholders(op.Body, results)

        // Execute operation within transaction
        result, err := executeOperation(ctx, tx, op)
        if err != nil {
            return nil, fmt.Errorf("operation %d failed: %w", i, err)
        }
        results[i] = result
    }

    if err := tx.Commit(); err != nil {
        return nil, err
    }
    return results, nil
}
```

**Key design decisions:**

- REST-style operations that mirror existing API endpoints
- Server-side placeholder resolution (Turso's batch API doesn't support this natively)
- Sequential execution within a transaction for placeholder support
- Atomic: all operations succeed or all rollback
- 10-second transaction timeout to prevent lock contention

**Interface abstraction for transaction support:**

```go
// QueryExecutor allows both *sql.DB and *sql.Tx to be used interchangeably
type QueryExecutor interface {
    ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
    QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
    QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}
```

---

### Complex Joins

> **Status:** Planned

Currently, nested relations only work via foreign key inference. Planned support for:

- **Explicit JOIN conditions:**

  ```
  GET /query/orders?join=users:orders.user_id=users.id&select=orders.*,users.name
  ```

- **Self-joins:**

  ```
  GET /query/employees?join=employees:manager_id=id&select=*,manager:employees(name)
  ```

- **Cross-database joins** (for Turso):
  ```
  GET /query/orders?join=products@catalog-db:orders.product_id=products.id
  ```

---

### Views

> **Status:** Planned

Create and query database views for complex data transformations.

**Create a view:**

```bash
POST /schema/view/active_users
{
  "query": "SELECT id, name, email FROM users WHERE status = 'active' AND deleted_at IS NULL",
  "columns": ["id", "name", "email"]
}
```

**Query a view:**

```bash
GET /query/active_users?name=like.%smith%
```

**Materialized views** (refreshed periodically):

```bash
POST /schema/view/daily_stats
{
  "query": "SELECT date, count(*) as orders, sum(total) as revenue FROM orders GROUP BY date",
  "materialized": true,
  "refreshInterval": "1h"
}
```

---

## API Reference

See the OpenAPI specification at `/openapi.yaml` or interactive docs at `/docs`.

### Endpoints Summary

| Category    | Endpoint                      | Methods                  |
| ----------- | ----------------------------- | ------------------------ |
| Health      | `/health`                     | GET                      |
| Docs        | `/openapi.yaml`, `/docs`      | GET                      |
| Query       | `/query/{table}`              | GET, POST, PATCH, DELETE |
| Schema      | `/schema`                     | GET, POST                |
| Schema      | `/schema/invalidate`          | POST                     |
| Schema      | `/schema/table/{table}`       | GET, POST, PATCH, DELETE |
| FTS         | `/schema/fts`                 | GET                      |
| FTS         | `/schema/fts/{table}`         | POST, DELETE             |
| Database    | `/db`                         | GET, POST, PATCH         |
| Database    | `/db/all`                     | PATCH                    |
| Database    | `/db/{name}`                  | DELETE                   |
| Templates   | `/templates`                  | GET, POST                |
| Templates   | `/templates/{name}`           | GET, PUT, DELETE         |
| Templates   | `/templates/{name}/sync`      | POST                     |
| Templates   | `/templates/{name}/databases` | GET                      |
| DB Template | `/db/{name}/template`         | GET, PUT, DELETE         |
| DB Template | `/db/{name}/sync`             | POST                     |
| Batch       | `/batch`                      | POST _(planned)_         |

### Headers

| Header          | Description                                                           |
| --------------- | --------------------------------------------------------------------- |
| `Authorization` | `Bearer <api-key>` for authentication                                 |
| `DB-Name`       | Target database name (default: primary)                               |
| `DB-Token`      | Database auth token (for registering)                                 |
| `Prefer`        | `count=exact` for row count, `resolution=merge-duplicates` for upsert |
| `X-Request-ID`  | Request tracing ID (auto-generated if not provided)                   |

### Response Headers

| Header          | Description                                  |
| --------------- | -------------------------------------------- |
| `X-Total-Count` | Total row count (when `Prefer: count=exact`) |
| `X-Request-ID`  | Request tracing ID                           |
| `Retry-After`   | Seconds until rate limit resets (on 429)     |
