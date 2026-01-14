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

| Endpoint               | Method      | Description        |
| ---------------------- | ----------- | ------------------ |
| `/schema`              | GET         | List all tables    |
| `/schema/table/{name}` | POST        | Create table       |
| `/schema/table/{name}` | PATCH       | Alter table        |
| `/schema/table/{name}` | DELETE      | Drop table         |
| `/schema/fts/{table}`  | POST/DELETE | Manage FTS indexes |

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

| Endpoint          | Method | Description    |
| ----------------- | ------ | -------------- |
| `/tenants`        | GET    | List tenants   |
| `/tenants`        | POST   | Create tenant  |
| `/tenants/{name}` | DELETE | Delete tenant  |

---

### Schema Templates

Reusable schemas for multi-tenant apps:

| Endpoint                       | Description                    |
| ------------------------------ | ------------------------------ |
| `POST /templates`              | Create template                |
| `PUT /tenants/{name}/template` | Associate tenant with template |
| `POST /tenants/{name}/sync`    | Sync tenant to template        |
| `POST /templates/{name}/sync`  | Sync all tenants               |

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
client.transaction()          // Start multi-request transaction
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

### Transactions

**Batch** - independent operations, single round trip:

```typescript
const { data, error } = await client.batch([
  client.from("users").insert({ name: "Alice" }),
  client.from("users").insert({ name: "Bob" }),
  client.from("logs").insert({ action: "bulk_insert" }),
]);

// data.results = [{ last_insert_id: 1 }, { last_insert_id: 2 }, ...]
```

**Multi-request** - dependent operations with client logic:

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

## Transactions (REST API)

### Batch (Single Request)

Atomic multi-operation requests. All succeed or all rollback. For independent operations.

```http
POST /batch

{
  "operations": [
    {"operation": "insert", "table": "users", "body": {"data": {"name": "Alice"}}},
    {"operation": "insert", "table": "users", "body": {"data": {"name": "Bob"}}},
    {"operation": "insert", "table": "logs", "body": {"data": {"action": "bulk_insert"}}}
  ]
}
```

Response:
```json
{
  "results": [
    {"last_insert_id": 1},
    {"last_insert_id": 2},
    {"last_insert_id": 3}
  ]
}
```

Operations: `select`, `insert`, `upsert`, `update`, `delete`

### Multi-Request Transactions

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

| Category     | Endpoint                                         | Methods                  |
| ------------ | ------------------------------------------------ | ------------------------ |
| Query        | `/query/{table}`                                 | POST, PATCH, DELETE      |
| Batch        | `/batch`                                         | POST                     |
| Transactions | `/transaction`, `/transaction/{id}/commit`, `/transaction/{id}/rollback` | POST |
| Schema       | `/schema`, `/schema/table/{table}`               | GET, POST, PATCH, DELETE |
| FTS          | `/schema/fts/{table}`                            | POST, DELETE             |
| Tenants      | `/tenants`, `/tenants/{name}`                    | GET, POST, DELETE        |
| Templates    | `/templates`, `/templates/{name}`                | GET, POST, PUT, DELETE   |
| Sync         | `/tenants/{name}/sync`, `/templates/{name}/sync` | POST                     |

**Headers:**

- `Authorization: Bearer <key>` - API authentication
- `Tenant: <name>` - Target tenant database
- `Transaction: <id>` - Execute within transaction
- `Prefer: operation=select` - Specify SELECT query (POST only)
- `Prefer: on-conflict=replace` - UPSERT behavior
- `Prefer: on-conflict=ignore` - INSERT IGNORE behavior
- `Prefer: count=exact` - Include total count in `X-Total-Count` header
