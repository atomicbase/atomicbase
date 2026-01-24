# Atomicbase Database API

REST API for SQLite/Turso with JSON queries, multi-tenant database management, and schema templates.

## Quick Start

```typescript
import { createClient, eq } from "@atomicbase/sdk";
import type { PrimaryDB, UserAppDB } from "./atomicbase.d.ts";

const client = createClient<PrimaryDB>({ url, apiKey });

// Query
const { data } = await client
  .from("users")
  .select("id", "name")
  .where(eq("status", "active"));

// Multi-tenant
const tenant = client.tenant<UserAppDB>("tenant-123");
await tenant.from("projects").select();
```

---

## Data API (`/data`)

Operations within a single database. Use `Tenant` header to target tenant databases.

### CRUD Operations

All operations use `POST /data/query/{table}` with `Prefer` header to specify operation.

| Operation     | Header                                          | Body                   | Returns            |
| ------------- | ----------------------------------------------- | ---------------------- | ------------------ |
| SELECT        | `Prefer: operation=select`                      | `{select, where, ...}` | Rows               |
| INSERT        | `Prefer: operation=insert`                      | `{data: {...}}`        | `{last_insert_id}` |
| UPDATE        | `Prefer: operation=update`                      | `{data, where}`        | `{rows_affected}`  |
| DELETE        | `Prefer: operation=delete`                      | `{where}`              | `{rows_affected}`  |
| UPSERT        | `Prefer: operation=insert, on-conflict=replace` | `{data: [...]}`        | `{rows_affected}`  |
| INSERT IGNORE | `Prefer: operation=insert, on-conflict=ignore`  | `{data: {...}}`        | `{rows_affected}`  |

### Filtering

`where` is an array of conditions (ANDed together).

| Operator                              | Example                           |
| ------------------------------------- | --------------------------------- |
| `eq`, `neq`, `gt`, `gte`, `lt`, `lte` | `{"age": {"gte": 21}}`            |
| `like`, `glob`                        | `{"name": {"like": "%smith%"}}`   |
| `in`, `between`                       | `{"status": {"in": ["a", "b"]}}`  |
| `is`, `fts`                           | `{"deleted_at": {"is": null}}`    |
| `not`, `or`                           | `{"or": [{"a": {"eq": 1}}, ...]}` |

### Nested Relations

Auto-join via foreign keys:

```json
{ "select": ["id", "name", { "posts": ["title", { "comments": ["body"] }] }] }
```

### Batch Operations

```http
POST /data/batch
{"operations": [
  {"operation": "insert", "table": "users", "body": {"data": {"name": "Alice"}}},
  {"operation": "insert", "table": "users", "body": {"data": {"name": "Bob"}}}
]}
```

All operations succeed or all rollback.

### Schema Introspection

```http
GET /data/schema              # Full schema
GET /data/schema/table/users  # Single table
GET /data/schema/fts          # FTS indexes
```

### Targeting Tenant Databases

Use `Tenant` header to query a tenant database instead of primary:

```http
POST /data/query/users
Tenant: tenant-123
Prefer: operation=select
```

---

## Platform API (`/platform`)

Operations across databases: tenant management, templates, migrations.

### Tenants

```http
GET    /platform/tenants              # List all tenants
POST   /platform/tenants              # Create tenant {"name": "...", "template": "..."}
POST   /platform/tenants/import       # Import existing Turso database {"database": "...", "template": "..."}
DELETE /platform/tenants/{name}       # Delete tenant
```

Import registers an existing Turso database. Schema must match the template (use CLI to migrate first if needed).

### Templates

```http
GET    /platform/templates            # List all templates
GET    /platform/templates/{name}     # Get template
POST   /platform/templates            # Create/update template
DELETE /platform/templates/{name}     # Delete template
GET    /platform/templates/{name}/tenants   # List tenants using template
GET    /platform/templates/{name}/history   # Version history
POST   /platform/templates/{name}/rollback  # Rollback to version
```

### Jobs

```http
GET    /platform/jobs                 # List async jobs
GET    /platform/jobs/{id}            # Job status
DELETE /platform/jobs/{id}            # Cancel job
```

### Health

```http
GET    /platform/health               # Service health check
```

---

## Schema Templates

Templates define database schemas as TypeScript files. Server is source of truth with push/pull syncing.

### Define Schema

```typescript
// schemas/user-app.schema.ts
import { defineSchema, defineTable, c } from "@atomicbase/sdk";

export default defineSchema("user-app", {
  projects: defineTable({
    id: c.integer().primaryKey(),
    title: c.text().notNull(),
  }).fts(["title"]),

  tasks: defineTable({
    id: c.integer().primaryKey(),
    projectId: c.integer().references("projects.id"),
    completed: c.integer().default(0),
  }),

  sql: [
    "CREATE VIEW incomplete_tasks AS SELECT * FROM tasks WHERE completed = 0",
  ],
});
```

**Column types:** `c.integer()`, `c.text()`, `c.real()`, `c.blob()`
**Modifiers:** `.primaryKey()`, `.notNull()`, `.unique()`, `.default(value)`, `.references("table.column")`

### CLI Commands

```bash
atomicbase push                # Push schema + migrate all tenants
atomicbase pull                # Pull schema from server
atomicbase diff                # Preview changes
atomicbase generate            # Generate TypeScript types
atomicbase history <template>  # Version history
atomicbase rollback <template> --version 3

atomicbase tenants create <name> --template <template>
atomicbase tenants import <database> --template <template>  # Import existing Turso DB
atomicbase tenants list
atomicbase tenants delete <name>  # Requires confirmation
```

### Migrations

**Auto-handled:** Add table, add column, rename column/table

**Requires Confirmation:** Drop table, drop column

**Ambiguous (interactive prompt):**

```bash
? Column 'fullName' in 'users' - is this new or renamed?
  › Create new column 'fullName'
    Rename 'name' → 'fullName'
```

**SQLite limitations (requires manual migration):** Change column type, add NOT NULL, change primary key

### Migration Execution

- Pre-validates all tenants before applying (atomic semantics)
- Parallel execution (default 50 concurrent, configurable)
- Async jobs for 1000+ tenants

```bash
$ atomicbase push
Validating 2,847 tenants... ✓
Migrating (50 concurrent)...
████████████████░░░░ 1,423/2,847 (50%)
✓ 2,847 tenants migrated in 34s
```

### Type Generation

```bash
$ atomicbase generate
✓ Generated types in ./src/atomicbase.d.ts
  - PrimaryDB (2 tables)
  - UserAppDB (2 tables)
```

```typescript
import type { PrimaryDB, UserAppDB } from "./atomicbase.d.ts";

const client = createClient<PrimaryDB>({ url, apiKey });
const tenant = client.tenant<UserAppDB>("tenant-123");

// Fully typed with autocomplete
const { data } = await client.from("users").select("id", "email");
```

---

## TypeScript SDK

### Querying

```typescript
// Select with filters
await client.from("users").select("id", "name").where(eq("status", "active"));
await client
  .from("users")
  .select()
  .where(or(eq("role", "admin"), gt("age", 21)));

// Insert
await client.from("users").insert({ name: "Alice" });
await client.from("users").insert({ name: "Alice" }).returning("id");

// Upsert
await client.from("users").upsert({ id: 1, name: "Alice" });
await client
  .from("users")
  .insert({ id: 1, name: "Alice" })
  .onConflict("ignore");

// Update/Delete
await client.from("users").update({ status: "inactive" }).where(eq("id", 5));
await client.from("users").delete().where(eq("status", "deleted"));

// Pagination
await client
  .from("users")
  .select()
  .orderBy("created_at", "desc")
  .limit(20)
  .offset(40);

// Single record
const { data } = await client
  .from("users")
  .select()
  .where(eq("id", 1))
  .single();
```

### Multi-Tenant

```typescript
// Create tenant (Platform API - server-side only)
const platform = createClient({ url, apiKey: process.env.PLATFORM_API_KEY });
await platform.tenants.create("tenant-123", { template: "user-app" });

// Query tenant data (Data API)
const client = createClient({ url, apiKey: process.env.DATA_API_KEY });
const tenant = client.tenant<UserAppDB>("tenant-123");
await tenant.from("projects").select();
```

### Response Format

All methods return `{ data, error }`:

```typescript
const { data, error } = await client.from("users").select();
if (error) {
  console.error(error.code, error.message, error.hint);
  return;
}
```

---

## Error Codes

| Code                     | Status | Description                     |
| ------------------------ | ------ | ------------------------------- |
| `TABLE_NOT_FOUND`        | 404    | Table doesn't exist             |
| `COLUMN_NOT_FOUND`       | 404    | Column doesn't exist            |
| `DATABASE_NOT_FOUND`     | 404    | Tenant database not found       |
| `TEMPLATE_NOT_FOUND`     | 404    | Schema template not found       |
| `INVALID_OPERATOR`       | 400    | Unknown filter operator         |
| `MISSING_WHERE_CLAUSE`   | 400    | DELETE/UPDATE requires where    |
| `UNIQUE_VIOLATION`       | 409    | Unique constraint violated      |
| `FOREIGN_KEY_VIOLATION`  | 400    | Referenced record doesn't exist |
| `NOT_NULL_VIOLATION`     | 400    | Required field is missing       |
| `TURSO_CONNECTION_ERROR` | 502    | Cannot reach Turso database     |
| `INTERNAL_ERROR`         | 500    | Unexpected server error         |

---

## Authentication

Each API requires its own key:

| API      | Key Prefix       | Description                          |
| -------- | ---------------- | ------------------------------------ |
| Data     | `atb_data_`      | Query/mutate rows within databases   |
| Platform | `atb_platform_`  | Manage tenants, templates, jobs      |

```bash
# Data API - query/mutate data
curl /data/query/users -H "Authorization: Bearer atb_data_xxxx"

# Platform API - manage infrastructure
curl /platform/tenants -H "Authorization: Bearer atb_platform_xxxx"
```

Tenant creation during user signup requires Platform API:

```typescript
// Server-side only (platform key)
const platform = createClient({ url, apiKey: process.env.PLATFORM_API_KEY });
await platform.tenants.create("tenant-123", { template: "user-app" });

// Client-side (data key)
const client = createClient({ url, apiKey: process.env.DATA_API_KEY });
await client.from("users").insert({ name: "Alice" }); // ✓
await client.tenants.create("x"); // ✗ 403 Forbidden
```

## Configuration

| Env Variable                  | Description                              |
| ----------------------------- | ---------------------------------------- |
| `ATOMICBASE_DATA_API_KEY`     | Key for Data API (`/data/*`)             |
| `ATOMICBASE_PLATFORM_API_KEY` | Key for Platform API (`/platform/*`)     |
| `ATOMICBASE_RATE_LIMIT`       | Requests/minute (default: 100)           |
| `ATOMICBASE_CORS_ORIGINS`     | Allowed origins (`*` or comma-separated) |
| `TURSO_ORGANIZATION`          | Turso organization name                  |
| `TURSO_API_KEY`               | Turso API key                            |

---

## API Reference

### Data API

| Endpoint                        | Methods | Description          |
| ------------------------------- | ------- | -------------------- |
| `/data/query/{table}`           | POST    | CRUD operations      |
| `/data/batch`                   | POST    | Batch operations     |
| `/data/schema`                  | GET     | Full schema          |
| `/data/schema/table/{table}`    | GET     | Single table schema  |
| `/data/schema/fts`              | GET     | FTS indexes          |

### Platform API

| Endpoint                               | Methods      | Description             |
| -------------------------------------- | ------------ | ----------------------- |
| `/platform/tenants`                    | GET, POST    | List/create tenants     |
| `/platform/tenants/import`             | POST         | Import existing Turso DB|
| `/platform/tenants/{name}`             | DELETE       | Delete tenant           |
| `/platform/templates`                  | GET, POST    | List/create templates   |
| `/platform/templates/{name}`           | GET, DELETE  | Get/delete template     |
| `/platform/templates/{name}/tenants`   | GET          | Tenants using template  |
| `/platform/templates/{name}/history`   | GET          | Version history         |
| `/platform/templates/{name}/rollback`  | POST         | Rollback to version     |
| `/platform/jobs`                       | GET          | List async jobs         |
| `/platform/jobs/{id}`                  | GET, DELETE  | Job status/cancel       |
| `/platform/health`                     | GET          | Service health          |

### Headers

| Header          | API      | Description                                  |
| --------------- | -------- | -------------------------------------------- |
| `Authorization` | Both     | `Bearer <key>` - API authentication          |
| `Tenant`        | Data     | Target tenant database (optional)            |
| `Prefer`        | Data     | `operation=select\|insert\|update\|delete`   |
| `Prefer`        | Data     | `on-conflict=replace\|ignore` (with insert)  |
| `Prefer`        | Data     | `count=exact` - Include `X-Total-Count`      |
