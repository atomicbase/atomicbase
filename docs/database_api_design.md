# Atomicbase Database API

REST API for SQLite/Turso with JSON queries, multi-tenant database management, and schema templates.

## Quick Start

```typescript
import { createClient, eq, or } from "atomicbase";

const { db, schema, databases, templates } = createClient({
  url: "http://localhost:8080",
  apiKey: process.env.ATOMICBASE_API_KEY,
});

// Query
const users = await db
  .from("users")
  .select("id", "name")
  .where(eq("status", "active"));

// Multi-tenant
const tenant = databases.get("tenant-123");
await tenant.db.from("projects").select("*");
```

---

## REST API

### CRUD Operations

All queries use `POST /query/{table}` with JSON bodies. Operation determined by body structure:

| Operation | Method | Body                          | Returns            |
| --------- | ------ | ----------------------------- | ------------------ |
| SELECT    | POST   | `{select: [...]}`             | Rows               |
| INSERT    | POST   | `{data: {...}}`               | `{last_insert_id}` |
| UPSERT    | POST   | `{data: [...]}`               | `{rows_affected}`  |
| UPDATE    | PATCH  | `{data: {...}, where: [...]}` | `{rows_affected}`  |
| DELETE    | DELETE | `{where: [...]}`              | `{rows_affected}`  |

**Select:**

```json
POST /query/users
{"select": ["id", "name"], "where": [{"status": {"eq": "active"}}], "limit": 20}
```

**Insert with returning:**

```json
POST /query/users
{"data": {"name": "Alice"}, "returning": ["id", "created_at"]}
```

**Update:**

```json
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

### Aggregations

```json
{ "select": ["status", { "count": "*" }, { "sum": "total", "as": "revenue" }] }
```

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

---

### Multi-Database

Target Turso databases with `DB-Name` header:

```
POST /query/users
DB-Name: tenant-123
{"select": ["*"]}
```

| Endpoint     | Method | Description     |
| ------------ | ------ | --------------- |
| `/db`        | GET    | List databases  |
| `/db`        | POST   | Create database |
| `/db/{name}` | DELETE | Delete database |

---

### Schema Templates

Reusable schemas for multi-tenant apps:

| Endpoint                      | Description                      |
| ----------------------------- | -------------------------------- |
| `POST /templates`             | Create template                  |
| `PUT /db/{name}/template`     | Associate database with template |
| `POST /db/{name}/sync`        | Sync database to template        |
| `POST /templates/{name}/sync` | Sync all databases               |

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

const { db, schema, databases, templates } = createClient({ url, apiKey });
```

### db - Queries

```typescript
// Select
const users = await db
  .from("users")
  .select("id", "name")
  .where(eq("status", "active"));

// With relations
const users = await db.from("users").select("id", { posts: ["title"] });

// OR conditions
const users = await db
  .from("users")
  .where(or(eq("role", "admin"), gt("age", 21)));

// Insert
await db.from("users").insert({ name: "Alice" });
const [user] = await db.from("users").insert({ name: "Alice" }).returning("id");

// Update/Delete
await db.from("users").update({ status: "inactive" }).where(eq("id", 5));
await db.from("users").delete().where(eq("status", "deleted"));

// Pagination
await db
  .from("users")
  .select("*")
  .orderBy("created_at", "desc")
  .limit(20)
  .offset(40);
```

### schema - DDL

```typescript
await schema.createTable("users", {
  id: { type: "integer", primaryKey: true },
  email: { type: "text", unique: true, notNull: true },
});
await schema.alterTable("users", { addColumns: { avatar: { type: "text" } } });
await schema.dropTable("users");
await schema.createFtsIndex("articles", ["title", "content"]);
```

### databases - Multi-Tenant

```typescript
await databases.create('tenant-123')
await databases.delete('tenant-123')
const list = await databases.list()

// Scoped client
const tenant = databases.get('tenant-123')
await tenant.db.from('projects').select('*')
await tenant.schema.createTable('tasks', {...})
```

**Database-per-user pattern:**

```typescript
async function createUser(email: string) {
  const dbName = `tenant-${crypto.randomUUID()}`;
  await databases.create(dbName);
  await db.from("users").insert({ email, database: dbName });
  await templates.applyTo(dbName, "user-app");
}

async function getUserData(userId: number) {
  const [user] = await db
    .from("users")
    .select("database")
    .where(eq("id", userId));
  return databases.get(user.database).db.from("projects").select("*");
}
```

### templates - Schema Templates

```typescript
await templates.create('saas-app', { tables: [...] })
await templates.applyTo('tenant-123', 'saas-app')
await templates.sync('saas-app') // Sync all databases using this template
```

---

## Planned Features

### Batch Transactions

Atomic multi-operation requests with placeholder references:

```json
POST /batch
{"atomic": true, "operations": [
  {"method": "POST", "path": "/query/users", "body": {"data": {"name": "Alice"}}},
  {"method": "POST", "path": "/query/accounts", "body": {"data": {"user_id": "$0.last_insert_id"}}}
]}
```

### Explicit Joins

Override implicit FK joins:

```typescript
// Inner join instead of left
await db
  .from("users")
  .select("id", { posts: ["title"] })
  .innerJoin("posts");

// Custom condition
await db
  .from("messages")
  .select("id", { sender: ["name"], recipient: ["name"] })
  .leftJoin("users", eq("messages.sender_id", "users.id"), { as: "sender" })
  .leftJoin("users", eq("messages.recipient_id", "users.id"), {
    as: "recipient",
  });
```

### Views

```json
POST /schema/view/active_users
{"query": "SELECT * FROM users WHERE status = 'active'"}
```

---

## API Reference

| Category  | Endpoint                                    | Methods                  |
| --------- | ------------------------------------------- | ------------------------ |
| Query     | `/query/{table}`                            | POST, PATCH, DELETE      |
| Schema    | `/schema`, `/schema/table/{table}`          | GET, POST, PATCH, DELETE |
| FTS       | `/schema/fts/{table}`                       | POST, DELETE             |
| Database  | `/db`, `/db/{name}`                         | GET, POST, DELETE        |
| Templates | `/templates`, `/templates/{name}`           | GET, POST, PUT, DELETE   |
| Sync      | `/db/{name}/sync`, `/templates/{name}/sync` | POST                     |

**Headers:**

- `Authorization: Bearer <key>` - API authentication
- `DB-Name: <name>` - Target database
- `Prefer: count=exact` - Include total count in `X-Total-Count` header
