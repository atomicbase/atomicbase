# Atomicbase Database API

REST API for SQLite/Turso with JSON queries, multi-tenant database management, and schema templates.

## Quick Start

```typescript
import { createClient, eq, or } from "atomicbase";

const client = createClient({
  url: "http://localhost:8080",
  apiKey: process.env.ATOMICBASE_API_KEY,
});

// Query
const users = await client
  .from("users")
  .select("id", "name")
  .where(eq("status", "active"));

// Multi-tenant
const tenant = client.tenant("tenant-123");
await tenant.from("projects").select("*");
```

---

## REST API

### CRUD Operations

POST operations use `Prefer` header to specify behavior. PATCH/DELETE use HTTP method directly.

| Operation     | Method | Header                        | Body                   | Returns            |
| ------------- | ------ | ----------------------------- | ---------------------- | ------------------ |
| SELECT        | POST   | `Prefer: operation=select`    | `{select, where, ...}` | Rows               |
| INSERT        | POST   | —                             | `{data: {...}}`        | `{last_insert_id}` |
| UPSERT        | POST   | `Prefer: on-conflict=replace` | `{data: [...]}`        | `{rows_affected}`  |
| INSERT IGNORE | POST   | `Prefer: on-conflict=ignore`  | `{data: {...}}`        | `{rows_affected}`  |
| UPDATE        | PATCH  | —                             | `{data, where}`        | `{rows_affected}`  |
| DELETE        | DELETE | —                             | `{where}`              | `{rows_affected}`  |

**Select:**

```http
POST /query/users
Prefer: operation=select

{"select": ["id", "name"], "where": [{"status": {"eq": "active"}}], "limit": 20}
```

**Insert with returning:**

```http
POST /query/users

{"data": {"name": "Alice"}, "returning": ["id", "created_at"]}
```

**Update:**

```http
PATCH /query/users

{"data": {"status": "inactive"}, "where": [{"id": {"eq": 5}}]}
```

---

### Filtering

`where` is always an array. Elements are ANDed together.

**Operators:**

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

→ `WHERE org_id = 5 AND (status = 'active' OR role = 'admin')`

**Pagination:**

```json
{
  "select": ["*"],
  "order": { "created_at": "desc" },
  "limit": 20,
  "offset": 40
}
```

---

### Nested Relations

Auto-join via foreign keys:

```json
{ "select": ["id", "name", { "posts": ["title", { "comments": ["body"] }] }] }
```

Returns nested JSON:

```json
[{"id": 1, "name": "Alice", "posts": [{"title": "Hello", "comments": [...]}]}]
```

---

### Counting

Use `Prefer: count=exact` header to get total count:

```http
POST /query/users
Prefer: operation=select, count=exact

{"select": ["id", "name"], "where": [{"status": {"eq": "active"}}], "limit": 20}
```

Response includes `X-Total-Count` header with total matching rows (ignoring limit/offset).

For complex aggregations (sum, avg, group by), create views via migrations.

---

### Full-Text Search

```json
POST /schema/fts/articles
{"columns": ["title", "content"]}
```

```json
POST /query/articles
{"select": ["*"], "where": [{"title": {"fts": "sqlite database"}}]}
```

---

### Schema Management

| Endpoint               | Method         | Description          |
| ---------------------- | -------------- | -------------------- |
| `/schema`              | GET            | List all tables      |
| `/schema/invalidate`   | POST           | Refresh schema cache |
| `/schema/table/{name}` | GET            | Get table schema     |
| `/schema/table/{name}` | POST           | Create table         |
| `/schema/table/{name}` | PATCH          | Alter table          |
| `/schema/table/{name}` | DELETE         | Drop table           |
| `/schema/fts`          | GET            | List FTS indexes     |
| `/schema/fts/{table}`  | POST, DELETE   | Manage FTS indexes   |

**Create table:**

```json
POST /schema/table/users
{"columns": {"id": {"type": "integer", "primaryKey": true}, "email": {"type": "text", "unique": true}}}
```

**Views:** Views created via migrations or direct SQL are queryable through `/query/{view_name}`. No API for creating views - use migrations instead.

---

### Multi-Tenant

Target tenant databases with `Tenant` header:

```http
POST /query/users
Tenant: tenant-123
Prefer: operation=select

{"select": ["*"]}
```

| Endpoint          | Method | Description                     |
| ----------------- | ------ | ------------------------------- |
| `/tenants`        | GET    | List tenants                    |
| `/tenants`        | POST   | Create tenant                   |
| `/tenants`        | PATCH  | Register existing Turso database|
| `/tenants/all`    | PATCH  | Register all Turso databases    |
| `/tenants/{name}` | DELETE | Delete tenant                   |

---

### Schema Templates

Reusable schemas for multi-tenant apps:

| Endpoint                          | Method | Description                      |
| --------------------------------- | ------ | -------------------------------- |
| `/templates`                      | GET    | List templates                   |
| `/templates`                      | POST   | Create template                  |
| `/templates/{name}`               | GET    | Get template                     |
| `/templates/{name}`               | PUT    | Update template                  |
| `/templates/{name}`               | DELETE | Delete template                  |
| `/templates/{name}/sync`          | POST   | Sync all tenants to template     |
| `/templates/{name}/databases`     | GET    | List tenants using template      |
| `/tenants/{name}/template`        | GET    | Get tenant's template            |
| `/tenants/{name}/template`        | PUT    | Associate tenant with template   |
| `/tenants/{name}/template`        | DELETE | Remove template association      |
| `/tenants/{name}/sync`            | POST   | Sync tenant to its template      |

---

### Configuration

| Env Variable                    | Description                              |
| ------------------------------- | ---------------------------------------- |
| `ATOMICBASE_API_KEY`            | Enable auth (Bearer token)               |
| `ATOMICBASE_RATE_LIMIT_ENABLED` | Enable rate limiting                     |
| `ATOMICBASE_RATE_LIMIT`         | Requests/minute (default: 100)           |
| `ATOMICBASE_CORS_ORIGINS`       | Allowed origins (`*` or comma-separated) |
| `ATOMICBASE_REQUEST_TIMEOUT`    | Timeout in seconds (default: 30)         |
| `ATOMICBASE_MAX_QUERY_DEPTH`    | Max nested relation depth (default: 4)   |
| `TURSO_ORGANIZATION`            | Turso organization name (multi-tenant)   |
| `TURSO_API_KEY`                 | Turso API key (multi-tenant)             |
| `TURSO_TOKEN_EXPIRATION`        | Token expiration: `7d`, `30d`, `never`   |

---

## Error Handling

All errors return a consistent JSON structure with a stable `code` for programmatic handling, a human-readable `message`, and an optional `hint` with actionable guidance.

### Error Response Format

```json
{
  "code": "TABLE_NOT_FOUND",
  "message": "table not found in schema: users",
  "hint": "Verify the table name is spelled correctly. Use GET /schema to list available tables."
}
```

### Error Codes

**Resource Errors (404)**

| Code                | Description                                    |
| ------------------- | ---------------------------------------------- |
| `TABLE_NOT_FOUND`   | Table doesn't exist in schema                  |
| `COLUMN_NOT_FOUND`  | Column doesn't exist in table                  |
| `DATABASE_NOT_FOUND`| Tenant database not found                      |
| `TEMPLATE_NOT_FOUND`| Schema template not found                      |
| `NO_RELATIONSHIP`   | No foreign key between tables for auto-join   |

**Validation Errors (400)**

| Code                    | Description                                |
| ----------------------- | ------------------------------------------ |
| `INVALID_OPERATOR`      | Unknown filter operator in where clause    |
| `INVALID_COLUMN_TYPE`   | Invalid type in schema definition          |
| `INVALID_IDENTIFIER`    | Table/column name contains invalid chars   |
| `MISSING_WHERE_CLAUSE`  | DELETE/UPDATE requires where clause        |
| `QUERY_TOO_DEEP`        | Nested relations exceed max depth          |
| `ARRAY_TOO_LARGE`       | IN clause exceeds 100 elements             |
| `NOT_DDL_QUERY`         | Only CREATE/ALTER/DROP allowed for schema  |
| `NO_FTS_INDEX`          | Full-text search requires FTS index        |

**Constraint Errors**

| Code                    | Status | Description                          |
| ----------------------- | ------ | ------------------------------------ |
| `UNIQUE_VIOLATION`      | 409    | Record with unique value exists      |
| `FOREIGN_KEY_VIOLATION` | 400    | Referenced record doesn't exist      |
| `NOT_NULL_VIOLATION`    | 400    | Required field is missing            |
| `TEMPLATE_IN_USE`       | 409    | Template has associated databases    |
| `RESERVED_TABLE`        | 403    | Cannot query internal tables         |

**Turso Errors**

| Code                    | Status | Description                          |
| ----------------------- | ------ | ------------------------------------ |
| `TURSO_CONFIG_MISSING`  | 503    | Missing TURSO_ORGANIZATION or API_KEY|
| `TURSO_AUTH_FAILED`     | 401    | Invalid or expired API key/token     |
| `TURSO_FORBIDDEN`       | 403    | API key lacks permission             |
| `TURSO_NOT_FOUND`       | 404    | Database/org not found in Turso      |
| `TURSO_RATE_LIMITED`    | 429    | Too many Turso API requests          |
| `TURSO_CONNECTION_ERROR`| 502    | Cannot reach Turso database          |
| `TURSO_TOKEN_EXPIRED`   | 401    | Database token has expired           |
| `TURSO_SERVER_ERROR`    | 502    | Turso service temporarily unavailable|

**Other**

| Code             | Status | Description                              |
| ---------------- | ------ | ---------------------------------------- |
| `INTERNAL_ERROR` | 500    | Unexpected server error (check logs)     |

### SDK Error Handling

```typescript
const { data, error } = await client.from("users").select("*");

if (error) {
  switch (error.code) {
    case "TABLE_NOT_FOUND":
      console.log("Table doesn't exist:", error.hint);
      break;
    case "TURSO_TOKEN_EXPIRED":
      await client.tenants.reregister(tenantId);
      break;
    default:
      console.error(error.message);
  }
}
```

---

## TypeScript SDK

### Setup

```typescript
import { createClient, eq, gt, or, not } from "atomicbase";

const client = createClient({ url, apiKey });
```

### Querying

```typescript
// Select
const users = await client
  .from("users")
  .select("id", "name")
  .where(eq("status", "active"));

// With relations
const users = await client.from("users").select("id", { posts: ["title"] });

// OR conditions
const users = await client
  .from("users")
  .where(or(eq("role", "admin"), gt("age", 21)));

// Insert
await client.from("users").insert({ name: "Alice" });
const { data } = await client
  .from("users")
  .insert({ name: "Alice" })
  .returning("id");

// Upsert (insert or replace on conflict)
await client.from("users").upsert({ id: 1, name: "Alice" });
// Or explicitly:
await client.from("users").insert({ id: 1, name: "Alice" }).onConflict("replace");

// Insert ignore (skip on conflict)
await client.from("users").insert({ id: 1, name: "Alice" }).onConflict("ignore");

// Update/Delete
await client.from("users").update({ status: "inactive" }).where(eq("id", 5));
await client.from("users").delete().where(eq("status", "deleted"));

// Pagination
await client
  .from("users")
  .select("*")
  .orderBy("created_at", "desc")
  .limit(20)
  .offset(40);
```

### schema - DDL

```typescript
await client.schema.createTable("users", {
  id: { type: "integer", primaryKey: true },
  email: { type: "text", unique: true, notNull: true },
});
await client.schema.alterTable("users", {
  addColumns: { avatar: { type: "text" } },
});
await client.schema.dropTable("users");
await client.schema.createFtsIndex("articles", ["title", "content"]);
```

### tenants - Multi-Tenant

```typescript
await client.tenants.create("tenant-123");
await client.tenants.delete("tenant-123");
const list = await client.tenants.list();

// Scoped client (same structure as primary client)
const tenant = client.tenant("tenant-123");
await tenant.from("projects").select("*");
await tenant.schema.createTable("tasks", {...});
```

**Tenant-per-user pattern:**

```typescript
async function createUser(email: string) {
  const tenantId = `tenant-${crypto.randomUUID()}`;
  await client.tenants.create(tenantId);
  await client.from("users").insert({ email, tenant: tenantId });
  await client.templates.applyTo(tenantId, "user-app");
}

async function getUserData(userId: number) {
  const { data: user } = await client
    .from("users")
    .select("tenant")
    .where(eq("id", userId))
    .single();
  return client.tenant(user.tenant).from("projects").select("*");
}
```

### templates - Schema Templates

```typescript
await client.templates.create("saas-app", { tables: [...] });
await client.templates.applyTo("tenant-123", "saas-app");
await client.templates.sync("saas-app"); // Sync all databases using this template
```

### Client Structure

```typescript
client.from(...)              // Query builder (top-level, most common)
client.batch([...])           // Atomic batch operations
client.transaction()          // Start multi-request transaction (planned)
client.schema.*               // DDL operations
client.tenants.*              // Tenant management (create, delete, list)
client.tenant(name)           // Get scoped client → same structure
client.templates.*            // Template management
```

### Response Format

All SDK methods return `{ data, error }`:

```typescript
const { data: users, error } = await client
  .from("users")
  .select("id", "name");

if (error) {
  console.error(error.message);
  return;
}

// data is typed, error is null
users.forEach(u => console.log(u.name));
```

For single records:

```typescript
const { data: user, error } = await client
  .from("users")
  .select("*")
  .where(eq("id", 1))
  .single();

// data is single object or null, not an array
```

### Batch Operations

Execute multiple operations atomically in a single request:

```typescript
const { data, error } = await client.batch([
  client.from("users").insert({ name: "Alice" }),
  client.from("users").insert({ name: "Bob" }),
  client.from("logs").insert({ action: "bulk_insert" }),
]);

// data.results = [{ last_insert_id: 1 }, { last_insert_id: 2 }, ...]
```

### Multi-Request Transactions (Planned)

For dependent operations with client logic between queries:

```typescript
const tx = await client.transaction();

const { data: user } = await tx.from("users").insert({ name: "Alice" });

// Use result to build next query
const { data: account } = await tx
  .from("accounts")
  .insert({ user_id: user.last_insert_id, balance: 0 });

await tx.commit();
// Or: await tx.rollback();
```

---

## Batch Operations

Execute multiple operations atomically in a single request. All operations succeed or all rollback.

```http
POST /batch

{
  "operations": [
    {"operation": "insert", "table": "users", "body": {"data": {"name": "Alice"}}},
    {"operation": "insert", "table": "users", "body": {"data": {"name": "Bob"}}},
    {"operation": "update", "table": "counters", "body": {"data": {"count": 2}, "where": [{"id": {"eq": 1}}]}}
  ]
}
```

Response:
```json
{
  "results": [
    {"last_insert_id": 1},
    {"last_insert_id": 2},
    {"rows_affected": 1}
  ]
}
```

**Operations:**

| Operation | Body Format | Result |
| --------- | ----------- | ------ |
| `select`  | `{select, where, order, limit, offset}` | Array of rows |
| `insert`  | `{data: {...}}` | `{last_insert_id}` |
| `upsert`  | `{data: [...]}` | `{rows_affected}` |
| `update`  | `{data: {...}, where: [...]}` | `{rows_affected}` |
| `delete`  | `{where: [...]}` | `{rows_affected}` |

**Example with select:**

```http
POST /batch

{
  "operations": [
    {"operation": "insert", "table": "users", "body": {"data": {"name": "Alice"}}},
    {"operation": "select", "table": "users", "body": {"select": ["id", "name"], "where": [{"name": {"eq": "Alice"}}]}}
  ]
}
```

Response:
```json
{
  "results": [
    {"last_insert_id": 1},
    [{"id": 1, "name": "Alice"}]
  ]
}
```

**Multi-tenant:** Use `Tenant` header to execute batch on a tenant database.

---

## Multi-Request Transactions (Planned)

> **Note:** Multi-request transactions are a planned feature, not yet implemented.

For dependent operations requiring client-side logic between queries.

```http
POST /transaction
→ {"id": "tx_abc123", "expires_at": "2024-01-15T10:01:00Z"}

POST /query/users
Transaction: tx_abc123
{"data": {"name": "Alice"}}
→ {"last_insert_id": 42}

POST /query/accounts
Transaction: tx_abc123
{"data": {"user_id": 42, "balance": 0}}
→ {"last_insert_id": 1}

POST /transaction/tx_abc123/commit
→ {"status": "committed"}
```

Rollback: `POST /transaction/{id}/rollback`

Transactions auto-rollback on timeout (default: 30s).

---

## Planned Features

### Explicit Joins

Override implicit FK joins:

```typescript
// Inner join instead of left
await client
  .from("users")
  .select("id", { posts: ["title"] })
  .innerJoin("posts");

// Custom condition
await client
  .from("messages")
  .select("id", { sender: ["name"], recipient: ["name"] })
  .leftJoin("users", eq("messages.sender_id", "users.id"), { as: "sender" })
  .leftJoin("users", eq("messages.recipient_id", "users.id"), {
    as: "recipient",
  });
```

---

## API Reference

| Category     | Endpoint                                                                   | Methods                  |
| ------------ | -------------------------------------------------------------------------- | ------------------------ |
| Query        | `/query/{table}`                                                           | POST, PATCH, DELETE      |
| Batch        | `/batch`                                                                   | POST                     |
| Schema       | `/schema`, `/schema/invalidate`                                            | GET, POST                |
| Schema       | `/schema/table/{table}`                                                    | GET, POST, PATCH, DELETE |
| FTS          | `/schema/fts`, `/schema/fts/{table}`                                       | GET, POST, DELETE        |
| Tenants      | `/tenants`, `/tenants/all`, `/tenants/{name}`                              | GET, POST, PATCH, DELETE |
| Templates    | `/templates`, `/templates/{name}`, `/templates/{name}/databases`           | GET, POST, PUT, DELETE   |
| Sync         | `/tenants/{name}/sync`, `/templates/{name}/sync`                           | POST                     |
| Association  | `/tenants/{name}/template`                                                 | GET, PUT, DELETE         |
| Health       | `/health`                                                                  | GET                      |
| Docs         | `/openapi.yaml`, `/docs`                                                   | GET                      |

**Planned:**

| Category     | Endpoint                                                                   | Methods |
| ------------ | -------------------------------------------------------------------------- | ------- |
| Transactions | `/transaction`, `/transaction/{id}/commit`, `/transaction/{id}/rollback`   | POST    |

**Headers:**

- `Authorization: Bearer <key>` - API authentication
- `Tenant: <name>` - Target tenant database
- `Transaction: <id>` - Execute within transaction (planned)
- `Prefer: operation=select` - Specify SELECT query (POST only)
- `Prefer: on-conflict=replace` - UPSERT behavior
- `Prefer: on-conflict=ignore` - INSERT IGNORE behavior
- `Prefer: count=exact` - Include total count in `X-Total-Count` header
