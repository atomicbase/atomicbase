# Schema Templates Design

Schema Templates provide a TypeScript-first approach to defining and managing database schemas across unlimited tenant databases. The server is the source of truth, with push/pull syncing between local schema files and the server.

## Overview

```
┌─────────────────────────────────────────────────────────────────────┐
│                         Developer Workflow                          │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│   1. Define schema locally (TypeScript)                             │
│   2. Push to server → server validates, stores, versions            │
│   3. Server syncs schema to all associated tenant databases         │
│   4. Generate TypeScript types for SDK                              │
│                                                                     │
│   schemas/                                                          │
│   ├── user-app.schema.ts      ← Define tables, columns, FTS         │
│   └── analytics.schema.ts     ← Multiple templates supported        │
│                                                                     │
│   $ atomicbase push           ← Push schema + migrate tenants       │
│   $ atomicbase generate       ← Generate TypeScript types           │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

## Schema Definition API

### Basic Example

```typescript
// schemas/user-app.schema.ts
import { defineSchema, defineTable, c } from "@atomicbase/sdk";

export default defineSchema("user-app", {
  users: defineTable({
    id: c.integer().primaryKey(),
    email: c.text().notNull().unique(),
    name: c.text().notNull(),
    avatar_url: c.text(),
    created_at: c.text().notNull().default("CURRENT_TIMESTAMP"),
  }),

  projects: defineTable({
    id: c.integer().primaryKey(),
    user_id: c.integer().notNull().references("users.id"),
    title: c.text().notNull(),
    description: c.text(),
    archived: c.integer().notNull().default(0),
    created_at: c.text().notNull().default("CURRENT_TIMESTAMP"),
  }).fts(["title", "description"]),

  tasks: defineTable({
    id: c.integer().primaryKey(),
    project_id: c.integer().notNull().references("projects.id", { onDelete: "CASCADE" }),
    title: c.text().notNull(),
    completed: c.integer().notNull().default(0),
    due_date: c.text(),
    created_at: c.text().notNull().default("CURRENT_TIMESTAMP"),
  }).fts(["title"]),
});
```

### Column Types

```typescript
c.integer()   // INTEGER - whole numbers, booleans (0/1), timestamps (unix)
c.text()      // TEXT - strings, JSON, ISO dates
c.real()      // REAL - floating point numbers
c.blob()      // BLOB - binary data
```

### Column Modifiers

```typescript
.primaryKey()                              // PRIMARY KEY (auto-increment for INTEGER)
.notNull()                                 // NOT NULL constraint
.unique()                                  // UNIQUE constraint
.default(value)                            // Default value (literal or SQL expression)
.references("table.column")                // Foreign key reference
.references("table.column", {              // With cascading options
  onDelete: "CASCADE" | "SET NULL" | "RESTRICT" | "NO ACTION",
  onUpdate: "CASCADE" | "SET NULL" | "RESTRICT" | "NO ACTION"
})
```

### Full-Text Search

```typescript
// Enable FTS5 on specific columns
defineTable({
  title: c.text().notNull(),
  content: c.text(),
}).fts(["title", "content"])

// Creates: {table}_fts virtual table with specified columns
```

### Indexes

```typescript
defineTable({
  user_id: c.integer().notNull(),
  created_at: c.text().notNull(),
}).index("idx_user_created", ["user_id", "created_at"])
  .index("idx_user", ["user_id"])
```

## CLI Commands

### Schema Management

```bash
# Push local schema to server (validates, versions, migrates all tenants)
atomicbase push [schema-file]
atomicbase push                           # Push all schemas in ./schemas/
atomicbase push schemas/user-app.schema.ts

# Pull schema from server (overwrites local file)
atomicbase pull <template-name>
atomicbase pull user-app

# Preview changes without applying
atomicbase diff [schema-file]
atomicbase diff                           # Diff all schemas
atomicbase diff schemas/user-app.schema.ts

# Generate TypeScript types from schema
atomicbase generate [schema-file]
atomicbase generate                       # Generate for all schemas
# Output: schemas/user-app.types.ts
```

### Version History & Rollback

```bash
# View version history
atomicbase history <template-name>
atomicbase history user-app
# Output:
# v5 (current) - 2024-01-20 14:30:00 - Added tasks.due_date column
# v4           - 2024-01-19 10:00:00 - Added FTS to projects
# v3           - 2024-01-18 09:00:00 - Added projects table
# v2           - 2024-01-17 15:00:00 - Added users.avatar_url
# v1           - 2024-01-17 14:00:00 - Initial schema

# Rollback to specific version
atomicbase rollback <template-name> --version <n>
atomicbase rollback user-app --version 3
# Warning: This will migrate all 47 tenant databases. Continue? [y/N]
```

### Tenant Management

```bash
# Create new tenant database with template
atomicbase tenants create <name> --template <template>
atomicbase tenants create acme-corp --template user-app

# Import existing Turso database
atomicbase tenants import <database> --template <template>
atomicbase tenants import existing-db --template user-app

# List all tenants (optionally filter by template)
atomicbase tenants list
atomicbase tenants list --template user-app
atomicbase tenants list --outdated          # Show only databases behind latest version

# Delete tenant (requires confirmation)
atomicbase tenants delete <name>
atomicbase tenants delete acme-corp
# Warning: This will permanently delete database acme-corp. Type 'acme-corp' to confirm:

# Sync single tenant to latest template version
atomicbase tenants sync <name>
atomicbase tenants sync acme-corp
```

### Job Management

```bash
# List recent sync jobs
atomicbase jobs list
atomicbase jobs list --template user-app

# Show job details
atomicbase jobs show <job_id>
atomicbase jobs show <job_id> --failed    # Show only failed databases
atomicbase jobs show <job_id> --follow    # Live progress updates

# Retry failed migrations in a job
atomicbase jobs retry <job_id>

# Resume a stopped job (processes stopped + failed tasks)
atomicbase jobs resume <job_id>

# Stop a running job (safe stop - waits for in-progress to finish)
atomicbase jobs stop <job_id>
```

### Configuration

```bash
# Initialize config file
atomicbase init
# Creates: atomicbase.config.ts

# Login/configure API credentials
atomicbase login
atomicbase config set url http://localhost:8080
atomicbase config set apiKey sk-xxx
```

## Configuration File

```typescript
// atomicbase.config.ts
import { defineConfig } from "@atomicbase/cli";

export default defineConfig({
  url: process.env.ATOMICBASE_URL || "http://localhost:8080",
  apiKey: process.env.ATOMICBASE_API_KEY,

  schemas: "./schemas",           // Schema files directory
  output: "./schemas",            // Generated types output

  // Optional: customize generated type names
  typePrefix: "",
  typeSuffix: "Row",
});
```

## Server API

### Template Endpoints

```http
# Push schema (create or update template)
POST /platform/templates
Content-Type: application/json

{
  "name": "user-app",
  "tables": [...],           // Parsed table definitions
  "checksum": "abc123"       // Schema checksum for conflict detection
}

# Response includes version number and migration plan
{
  "version": 5,
  "changes": [
    {"type": "add_column", "table": "tasks", "column": "due_date"},
    {"type": "add_fts", "table": "projects", "columns": ["title", "description"]}
  ],
  "tenants_affected": 47
}
```

```http
# Get template with full schema
GET /platform/templates/{name}

{
  "name": "user-app",
  "version": 5,
  "tables": [...],
  "checksum": "abc123",
  "created_at": "2024-01-17T14:00:00Z",
  "updated_at": "2024-01-20T14:30:00Z"
}
```

```http
# Get version history
GET /platform/templates/{name}/history

[
  {"version": 5, "checksum": "abc123", "changes": [...], "created_at": "..."},
  {"version": 4, "checksum": "def456", "changes": [...], "created_at": "..."},
  ...
]
```

```http
# Preview diff without applying
POST /platform/templates/{name}/diff
Content-Type: application/json

{
  "tables": [...]
}

# Response
{
  "changes": [
    {"type": "add_column", "table": "tasks", "column": "due_date", "sql": "ALTER TABLE..."},
    {"type": "drop_column", "table": "users", "column": "legacy_field", "sql": "-- requires migration"}
  ],
  "requires_migration": true,
  "migration_sql": [...]
}
```

```http
# Rollback to version
POST /platform/templates/{name}/rollback
Content-Type: application/json

{"version": 3}
```

```http
# Start sync job for all tenants (returns job ID)
POST /platform/templates/{name}/sync

# Sync single tenant (synchronous)
POST /platform/tenants/{name}/sync

# Get sync job status
GET /platform/jobs/{job_id}

# List recent jobs
GET /platform/jobs?template={name}
```

## Sync Architecture (Scaled for 1000s of Databases)

Syncing schema changes across thousands of tenant databases requires:
- **Progress tracking** - Real-time visibility into sync status
- **Parallelization** - Concurrent migrations to minimize total time
- **Resilience** - Retry failed migrations, handle partial failures
- **Resumability** - Continue after CLI disconnect or server restart

### Server-Side Job Queue

Sync operations run as **server-side jobs** rather than client-driven:

```
┌─────────────────────────────────────────────────────────────────────┐
│                         Sync Architecture                           │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│   CLI                           Server                              │
│   ───                           ──────                              │
│   push schema ──────────────▶  1. Validate & version schema         │
│                                2. Create sync job                   │
│   ◀────────────────────────── 3. Return job_id immediately         │
│                                                                     │
│   poll job status ──────────▶  4. Worker pool processes databases  │
│   ◀────────────────────────── 5. Return progress                   │
│   (repeat until done)                                               │
│                                                                     │
│   Display progress:            Job runs independently:              │
│   [████████░░░░] 67%           - Parallel migrations                │
│   ✓ 1,340 completed            - Automatic retries                  │
│   ⟳ 50 in progress             - Persistent state                   │
│   ○ 600 pending                - Survives CLI disconnect            │
│   ✗ 10 failed                                                       │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

### Job Storage

```sql
-- Sync jobs table
CREATE TABLE __sync_jobs (
  id TEXT PRIMARY KEY,              -- UUID
  template_id INTEGER NOT NULL REFERENCES __templates(id),
  target_version INTEGER NOT NULL,
  status TEXT NOT NULL DEFAULT 'running',  -- running, completed, failed, stopped
  total_count INTEGER NOT NULL,
  completed_count INTEGER DEFAULT 0,
  failed_count INTEGER DEFAULT 0,
  stopped_count INTEGER DEFAULT 0,
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  completed_at TEXT
);

-- Per-database sync status
CREATE TABLE __sync_tasks (
  id INTEGER PRIMARY KEY,
  job_id TEXT NOT NULL REFERENCES __sync_jobs(id),
  database_id INTEGER NOT NULL REFERENCES __databases(id),
  status TEXT NOT NULL DEFAULT 'pending',  -- pending, running, completed, failed, stopped
  attempts INTEGER DEFAULT 0,
  error TEXT,
  started_at TEXT,
  completed_at TEXT
);

CREATE INDEX idx_sync_tasks_job_status ON __sync_tasks(job_id, status);
```

### Worker Pool

Server runs a configurable worker pool for parallel migrations:

```go
type SyncWorkerPool struct {
    workers    int           // Default: 10, configurable via ATOMICBASE_SYNC_WORKERS
    maxRetries int           // Default: 3
    retryDelay time.Duration // Default: 5s, exponential backoff
}
```

**Concurrency considerations:**
- Each worker handles one database at a time
- Turso rate limits: ~100 requests/second per org (adjust workers accordingly)
- Memory: Each connection is lightweight, 50+ concurrent is safe
- Default 10 workers = ~1000 databases in ~2 minutes (assuming 1s per migration)

### Job API

```http
# Start sync job
POST /platform/templates/{name}/sync
Content-Type: application/json

{
  "target_version": 5,           // Optional, defaults to latest
  "concurrency": 20,             // Optional, override default workers
  "retry_failed_only": false     // Optional, only retry previously failed
}

# Response
{
  "job_id": "abc-123-def",
  "status": "running",
  "total": 1247,
  "message": "Sync job started"
}
```

```http
# Get job status (poll this)
GET /platform/jobs/{job_id}

{
  "job_id": "abc-123-def",
  "template": "user-app",
  "target_version": 5,
  "status": "running",
  "progress": {
    "total": 1247,
    "completed": 892,
    "failed": 3,
    "pending": 302,
    "running": 50
  },
  "failed_databases": [
    {
      "name": "acme-corp",
      "error": "TURSO_CONNECTION_ERROR",
      "attempts": 2,
      "will_retry": true
    },
    {
      "name": "beta-inc",
      "error": "SCHEMA_MISMATCH",
      "attempts": 3,
      "will_retry": false,
      "message": "Column 'legacy' exists but not in template"
    }
  ],
  "started_at": "2024-01-20T14:30:00Z",
  "estimated_completion": "2024-01-20T14:32:30Z"
}
```

```http
# Stop a running job (safe stop - in-progress tasks complete, pending become stopped)
POST /platform/jobs/{job_id}/stop

# Resume a stopped job (processes stopped + failed tasks)
POST /platform/jobs/{job_id}/resume

# Retry only failed tasks in a job
POST /platform/jobs/{job_id}/retry
```

### CLI Progress Display

```bash
$ atomicbase push schemas/user-app.schema.ts

Pushing schema "user-app"...
✓ Schema validated
✓ Version 6 created (was 5)

Changes:
  + tasks.priority (INTEGER NOT NULL DEFAULT 0)
  + tasks.assigned_to (INTEGER REFERENCES users.id)
  ~ projects.archived (added NOT NULL, requires migration)

Syncing to 1,247 tenant databases...

[████████████████████████░░░░░░░░░░░░░░░░] 62%

  ✓ Completed    774
  ⟳ In Progress   50
  ○ Pending      420
  ✗ Failed         3

Failed databases:
  acme-corp      TURSO_CONNECTION_ERROR (retry 2/3)
  beta-inc       TURSO_CONNECTION_ERROR (retry 1/3)
  gamma-llc      SCHEMA_MISMATCH - manual intervention required

Elapsed: 1m 23s | Remaining: ~50s | Rate: 9.3 db/s

# When complete:
✓ Sync complete: 1,244 succeeded, 3 failed

Failed databases require manual attention:
  atomicbase tenants sync gamma-llc --force

Full job details: atomicbase jobs show abc-123-def
```

### Handling Failures

**Automatic Retry (transient errors):**
- Connection timeouts
- Rate limiting (429)
- Temporary Turso outages

**No Retry (permanent errors):**
- Schema mismatch (extra columns in DB not in template)
- Invalid migration (data doesn't fit new constraints)
- Database not found (deleted externally)

**Manual Resolution:**
```bash
# View failed databases
atomicbase jobs show <job_id> --failed

# Retry all failed in a job
atomicbase jobs retry <job_id>

# Force sync a specific tenant (ignores schema mismatches)
atomicbase tenants sync <name> --force

# Skip a database (mark as intentionally out of sync)
atomicbase tenants skip <name> --reason "Legacy database, will migrate manually"
```

### Resumability

Jobs persist across server restarts:

1. Server starts → scans for `running` jobs
2. Resets `running` tasks to `pending` (worker may have died mid-migration)
3. Resumes processing pending tasks

CLI can reconnect to check progress:
```bash
# List recent jobs
atomicbase jobs list

# Check specific job
atomicbase jobs show <job_id>

# Follow progress (live updates)
atomicbase jobs show <job_id> --follow
```

### Configuration

```bash
# Server-side (environment variables)
ATOMICBASE_SYNC_WORKERS=10       # Concurrent migrations
ATOMICBASE_SYNC_MAX_RETRIES=3    # Max retry attempts
ATOMICBASE_SYNC_RETRY_DELAY=5s   # Initial retry delay (exponential backoff)
ATOMICBASE_SYNC_TIMEOUT=30s      # Per-database migration timeout
```

## Migration Strategy

### Per-Database Atomicity (Critical)

**Every database migration is fully atomic.** A database either:
- ✅ Successfully migrates to the new version, OR
- ✅ Rolls back completely to the old version

**There is no in-between state.** A database is never left with a partial schema.

```go
func migrateDatabase(ctx context.Context, db *sql.DB, changes []Change) error {
    tx, err := db.BeginTx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback() // Rollback if not committed

    // Apply all changes within transaction
    for _, change := range changes {
        if _, err := tx.ExecContext(ctx, change.SQL); err != nil {
            return err // Transaction rolls back automatically
        }
    }

    // Only commit if ALL changes succeeded
    return tx.Commit()
}
```

**For complex migrations (mirror table):**
```sql
-- All steps in single transaction
BEGIN TRANSACTION;
  PRAGMA foreign_keys=OFF;
  CREATE TABLE [users_new] (...);
  INSERT INTO [users_new] SELECT ... FROM [users];
  DROP TABLE [users];
  ALTER TABLE [users_new] RENAME TO [users];
  PRAGMA foreign_keys=ON;
COMMIT;
-- If ANY step fails, entire transaction rolls back
```

**Version update is part of the transaction:**
- The `schema_version` in `__databases` is only updated after successful commit
- If migration fails, version stays at old value
- This ensures version always reflects actual schema state

### Push Idempotency

`atomicbase push` is idempotent and only syncs databases that need it:

1. Push calculates which databases are behind the target version
2. Only those databases are included in the sync job
3. Running push twice is safe — already-synced databases are skipped

```bash
$ atomicbase push
Pushing schema "user-app"...
✓ Schema validated (no changes from v5)
✓ 1,244 databases already on v5
⟳ 3 databases behind (from stopped job)

Syncing 3 databases...
```

### Safe Operations (Direct ALTER TABLE)

These operations can be applied directly without data migration:

- `ADD COLUMN` (with or without DEFAULT, NOT NULL requires DEFAULT)
- `RENAME TABLE`
- `RENAME COLUMN`
- `DROP COLUMN` (SQLite 3.35+)
- `CREATE INDEX`
- `DROP INDEX`
- `CREATE TABLE`
- `DROP TABLE`

### Complex Operations (Mirror Table Required)

These require the 12-step mirror table process:

- Change column type
- Add `NOT NULL` to existing column without default
- Add `UNIQUE` constraint to existing column
- Change `PRIMARY KEY`
- Add/remove `FOREIGN KEY` constraints
- Reorder columns

### Migration Generation

The server automatically generates migration SQL:

```typescript
// Detected change: Add NOT NULL column with default
{
  type: "add_column",
  table: "tasks",
  column: "priority",
  definition: "INTEGER NOT NULL DEFAULT 0",
  sql: "ALTER TABLE [tasks] ADD COLUMN [priority] INTEGER NOT NULL DEFAULT 0"
}

// Detected change: Change column type (requires migration)
{
  type: "alter_column",
  table: "users",
  column: "age",
  from: "TEXT",
  to: "INTEGER",
  requires_migration: true,
  sql: [
    "PRAGMA foreign_keys=OFF",
    "BEGIN TRANSACTION",
    "CREATE TABLE [users_new] (...)",
    "INSERT INTO [users_new] SELECT id, name, CAST(age AS INTEGER) FROM [users]",
    "DROP TABLE [users]",
    "ALTER TABLE [users_new] RENAME TO [users]",
    "PRAGMA foreign_keys=ON",
    "COMMIT"
  ]
}
```

## Version Storage

Templates are versioned in the primary database:

```sql
-- Template versions table
CREATE TABLE __templates_history (
  id INTEGER PRIMARY KEY,
  template_id INTEGER NOT NULL REFERENCES __templates(id),
  version INTEGER NOT NULL,
  tables BLOB NOT NULL,        -- gob-encoded table definitions
  checksum TEXT NOT NULL,      -- Schema checksum
  changes TEXT,                -- JSON array of changes from previous version
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE(template_id, version)
);

-- Track which version each tenant is on
ALTER TABLE __databases ADD COLUMN schema_version INTEGER DEFAULT 1;
```

## Generated Types

Running `atomicbase generate` creates TypeScript types:

```typescript
// schemas/user-app.types.ts (auto-generated)

export interface UsersRow {
  id: number;
  email: string;
  name: string;
  avatar_url: string | null;
  created_at: string;
}

export interface UsersInsert {
  id?: number;
  email: string;
  name: string;
  avatar_url?: string | null;
  created_at?: string;
}

export interface ProjectsRow {
  id: number;
  user_id: number;
  title: string;
  description: string | null;
  archived: number;
  created_at: string;
}

// ... etc for all tables

export interface UserAppSchema {
  users: UsersRow;
  projects: ProjectsRow;
  tasks: TasksRow;
}
```

Usage with SDK:

```typescript
import { createClient } from "@atomicbase/sdk";
import type { UsersRow, UsersInsert } from "./schemas/user-app.types";

const client = createClient({ url: "..." });

// Typed queries
const { data } = await client
  .from<UsersRow>("users")
  .select("id", "name", "email")
  .where(eq("id", 1))
  .single();

// data is typed as Pick<UsersRow, "id" | "name" | "email">
```

## CLI Package Structure

```
packages/
└── cli/
    ├── package.json
    ├── src/
    │   ├── index.ts           # CLI entry point
    │   ├── commands/
    │   │   ├── push.ts
    │   │   ├── pull.ts
    │   │   ├── diff.ts
    │   │   ├── generate.ts
    │   │   ├── history.ts
    │   │   ├── rollback.ts
    │   │   ├── init.ts
    │   │   └── tenants.ts
    │   ├── schema/
    │   │   ├── parser.ts      # Parse .schema.ts files
    │   │   ├── validator.ts   # Validate schema definitions
    │   │   └── serializer.ts  # Convert to JSON for API
    │   ├── codegen/
    │   │   └── types.ts       # Generate TypeScript types
    │   └── config.ts          # Load atomicbase.config.ts
    └── bin/
        └── atomicbase         # CLI binary
```

## Implementation Phases

### Phase 1: Schema Definition SDK
- [ ] `defineSchema`, `defineTable`, column builders (`c.integer()`, etc.)
- [ ] Column modifiers (`.primaryKey()`, `.notNull()`, `.references()`, etc.)
- [ ] Table modifiers (`.fts()`, `.index()`)
- [ ] Schema serialization to JSON for API

### Phase 2: CLI Foundation
- [ ] CLI project setup (TypeScript, build config)
- [ ] Config loading (env vars, atomicbase.config.ts)
- [ ] `init` command - create config file
- [ ] `push` command - parse schema, POST to API
- [ ] `pull` command - GET from API, write schema file
- [ ] `diff` command - preview changes

### Phase 3: API - Template Versioning
- [ ] `__templates_history` table for version storage
- [ ] Schema diff algorithm (detect changes between versions)
- [ ] Version creation on template update
- [ ] `GET /platform/templates/{name}/history` endpoint
- [ ] `POST /platform/templates/{name}/rollback` endpoint

### Phase 4: API - Migration Engine
- [ ] Migration SQL generation for safe operations
- [ ] Mirror table migration for complex operations
- [ ] Change detection (add/drop column, type changes, etc.)
- [ ] Per-database migration execution

### Phase 5: API - Sync Job System
- [ ] `__sync_jobs` and `__sync_tasks` tables
- [ ] Worker pool for parallel migrations
- [ ] `POST /platform/templates/{name}/sync` - start job
- [ ] `GET /platform/jobs/{job_id}` - job status
- [ ] Automatic retry with exponential backoff
- [ ] Job resumption on server restart

### Phase 6: CLI - Jobs & Progress
- [ ] `jobs list` command
- [ ] `jobs show` command with progress display
- [ ] `jobs show --follow` for live updates
- [ ] `jobs retry`, `jobs resume`, and `jobs stop` commands
- [ ] Progress bar and status formatting

### Phase 7: CLI - Tenant Management
- [ ] `tenants create` command
- [ ] `tenants list` command
- [ ] `tenants delete` command (with confirmation)
- [ ] `tenants sync` command (single tenant)
- [ ] `tenants skip/unskip` commands

### Phase 8: Version History CLI
- [ ] `history` command - show version list
- [ ] `rollback` command - with confirmation
- [ ] `rollback --dry-run` for preview

### Phase 9: Type Generation
- [ ] `generate` command
- [ ] TypeScript type generation from schema
- [ ] Row types, Insert types, schema type map
- [ ] SDK generic type support

### Phase 10: Polish
- [ ] Interactive confirmations for destructive operations
- [ ] Colorized output and error messages
- [ ] `--json` flag for machine-readable output
- [ ] Documentation and examples
- [ ] Error handling and helpful messages
