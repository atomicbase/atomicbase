# Plan: Platform API Implementation

## Overview

Build the Platform API from scratch in `api/platform/` for managing tenant databases and schema templates. The API handles multi-tenant database operations, schema versioning, and migrations.

---

## API Endpoints

### Tenants
| Method | Endpoint                       | Description                         |
|--------|--------------------------------|-------------------------------------|
| GET    | `/platform/tenants`            | List all tenant databases           |
| GET    | `/platform/tenants/{name}`     | Get tenant info                     |
| POST   | `/platform/tenants`            | Create tenant (requires template)   |
| DELETE | `/platform/tenants/{name}`     | Delete tenant                       |
| POST   | `/platform/tenants/{name}/sync`| Sync tenant to current version      |

### Templates
| Method | Endpoint                              | Description                                   |
|--------|---------------------------------------|-----------------------------------------------|
| GET    | `/platform/templates`                 | List all templates                            |
| GET    | `/platform/templates/{name}`          | Get template info + schema                    |
| POST   | `/platform/templates`                 | Create new template at v1                     |
| DELETE | `/platform/templates/{name}`          | Delete template (fails if tenants use it)     |
| POST   | `/platform/templates/{name}/diff`     | Preview changes (returns raw changes)         |
| POST   | `/platform/templates/{name}/migrate`  | Push new version + run migrations             |
| POST   | `/platform/templates/{name}/rollback` | Rollback to previous version (runs migration) |
| GET    | `/platform/templates/{name}/history`  | Get version history                           |

### Migration Jobs
| Method | Endpoint                    | Description                       |
|--------|-----------------------------|-----------------------------------|
| GET    | `/platform/jobs/{id}`       | Get migration status              |
| POST   | `/platform/jobs/{id}/retry` | Retry failed tenants from job     |

---

## Endpoint Details

### Diff
```
POST /templates/{name}/diff
Body: { schema: [...] }
Response: { changes: [...] }
```
- Compares provided schema against current template version
- Returns raw changes (add/drop/modify) - no ambiguity detection
- CLI analyzes changes to detect potential renames and prompts user

### Migrate
```
POST /templates/{name}/migrate
Body: { schema: [...], merge: [{old: 0, new: 1}, ...] }
Response: { jobId: "..." }
```
- `merge` indicates which drop+add pairs are renames (by index in diff)
- Starts job and returns job ID
- Creates new version marked pending
- Validates SQL, runs first DB
- If first fails, job is marked as failed and stops
- Migrates the rest of the databases concurrently using Turso's Batch API

### Rollback
```
POST /templates/{name}/rollback
Body: { version: 5 }
Response: { jobId: "..." }
```
- Fetches target version schema from history
- Starts job and returns job ID
- Creates pending version entry
- Generates migration SQL (current → target)
- Starts migration job, returns job ID

### Sync Tenant
```
POST /tenants/{name}/sync
Response: { fromVersion: 3, toVersion: 5 }
```
- Migrates a specific tenant to template's `current_version`
- Uses chained migrations if needed (v3→v4→v5)
- Synchronous - waits for completion
- Returns error if migration fails (with details)

### Retry Failed Tenants
```
POST /jobs/{id}/retry
Response: { retriedCount: 5, jobId: "..." }
```
- Finds all tenants with `status=failed` for this job
- Re-attempts migration for each
- Updates job counts (completed_dbs, failed_dbs)
- Returns count of retried tenants

### Migration Status & State
**Status** (what it's doing):
- `pending` - created, not started
- `running` - in progress
- `paused` - paused (future feature)
- `complete` - finished running

**State** (what happened):
- `null` - not finished yet
- `success` - all DBs migrated
- `partial` - some DBs failed, current_version updated
- `failed` - first DB failed, aborted

### Concurrent Migration Protection
If a migration is already running for a template, return:
- **Status**: 409 Conflict
- **Code**: `ATOMICBASE_BUSY`
- **Hint**: "migration locked. Another migration is already in progress"

### Out-of-Sync Tenant Protection
If a tenant's `template_version` < template's `current_version`:
- All Data API operations fail immediately
- **Status**: 409 Conflict
- **Code**: `TENANT_OUT_OF_SYNC`
- **Hint**: "tenant schema version X is behind current version Y. Run `atomicbase tenants sync {name}` to sync."

**Recovery options:**
- `POST /platform/tenants/{name}/sync` - sync specific tenant
- `POST /platform/jobs/{id}/retry` - retry all failed tenants from a job
- CLI: `atomicbase tenants sync <name>` or `atomicbase jobs retry <job-id>`

Tenant creation during migration:
- Allowed - tenant gets `current_version` at time of creation
- If migration completes and updates `current_version`, new tenant is now out-of-sync
- Forces user to be aware of migration state

---

## File Structure

```
api/platform/
├── handlers.go      # HTTP handlers + route registration
├── tenants.go       # Tenant CRUD + sync operations
├── templates.go     # Template CRUD + diff
├── migrations.go    # Migration plan generation + SQL
├── validation.go    # Pre-migration validation
├── batch.go         # Turso batch API client
├── jobs.go          # Background job system
└── types.go         # Platform-specific types
```

---

## Key Types

```go

type Schema struct {
    Tables []Table
}

// Table represents a database table's schema.
type Table struct {
	Name       string         `json:"name"`                 // Table name
	Pk         []string       `json:"pk"`                   // Primary key column name(s) - supports composite keys
	Columns    map[string]Col `json:"columns"`              // Keyed by column name
	Indexes    []Index        `json:"indexes,omitempty"`    // Table indexes
	FTSColumns []string       `json:"ftsColumns,omitempty"` // Columns for FTS5 full-text search
}

// Index represents a database index definition.
type Index struct {
	Name    string   `json:"name"`    // Index name
	Columns []string `json:"columns"` // Columns included in index
	Unique  bool     `json:"unique,omitempty"`
}

// Col represents a column definition.
type Col struct {
	Name       string     `json:"name"`                 // Column name
	Type       string     `json:"type"`                 // SQLite type (TEXT, INTEGER, REAL, BLOB)
	NotNull    bool       `json:"notNull,omitempty"`    // NOT NULL constraint
	Unique     bool       `json:"unique,omitempty"`     // UNIQUE constraint
	Default    any        `json:"default,omitempty"`    // Default value (nil if none)
	Collate    string     `json:"collate,omitempty"`    // COLLATE: BINARY, NOCASE, RTRIM
	Check      string     `json:"check,omitempty"`      // CHECK constraint expression
	Generated  *Generated `json:"generated,omitempty"`  // Generated column definition
	References string     `json:"references,omitempty"` // Foreign key reference (format: "table.column")
	OnDelete   string     `json:"onDelete,omitempty"`   // FK action: CASCADE, SET NULL, RESTRICT, NO ACTION
	OnUpdate   string     `json:"onUpdate,omitempty"`   // FK action: CASCADE, SET NULL, RESTRICT, NO ACTION
}

// Generated represents a generated/computed column.
type Generated struct {
	Expr   string `json:"expr"`             // Expression to compute value
	Stored bool   `json:"stored,omitempty"` // true=STORED, false=VIRTUAL (default)
}

// SchemaDiff represents a single schema modification
type SchemaDiff struct {
    Type    string `json:"type"`              // add_table, drop_table, rename_table,
                                              // add_column, drop_column, rename_column, modify_column,
                                              // add_index, drop_index, add_fts, drop_fts,
                                              // change_pk_type (requires mirror table)
    Table   string `json:"table,omitempty"`
    Column  string `json:"column,omitempty"`
}

// Note on type changes:
// - Regular column type changes: metadata-only, no SQL generated
// - PK type changes (change_pk_type): requires mirror table + data conversion

// DiffResult - returned by Diff endpoint, raw changes only
type DiffResult struct {
    Changes []SchemaDiff `json:"changes"`
}

// Merge indicates a drop+add pair that should be treated as a rename
// References indices in the changes array
type Merge struct {
    Old int `json:"old"` // Index of drop statement
    New int `json:"new"` // Index of add statement
}

// MigrationPlan - internal, all ambiguities resolved, ready to execute
type MigrationPlan struct {
    SQL     []string       `json:"sql"`     // Generated SQL statements
}

// Migration tracks both the SQL and execution state (combined table)
type Migration struct {
    ID           string     `json:"id"`
    TemplateID   int32      `json:"templateId"`
    FromVersion  int        `json:"fromVersion"`
    ToVersion    int        `json:"toVersion"`
    SQL          []string   `json:"sql"`           // Migration SQL statements
    Status       string     `json:"status"`        // pending, running, paused, complete
    State        string     `json:"state"`         // null, success, partial, failed
    TotalDBs     int        `json:"totalDbs"`
    CompletedDBs int        `json:"completedDbs"`
    FailedDBs    int        `json:"failedDbs"`
    StartedAt    *time.Time `json:"startedAt,omitempty"`
    CompletedAt  *time.Time `json:"completedAt,omitempty"`
    CreatedAt    time.Time  `json:"createdAt"`
}

// TenantMigration tracks per-tenant migration outcome
type TenantMigration struct {
    MigrationID string    `json:"migrationId"`
    TenantID    int32     `json:"tenantId"`
    Status      string    `json:"status"`    // success, failed
    Error       string    `json:"error,omitempty"`
    Attempts    int       `json:"attempts"`
    UpdatedAt   time.Time `json:"updatedAt"`
}
```

### Edge Cases

**No tenants**: Migration completes immediately with `state=success`, `total_dbs=0`.

**No changes (empty diff)**: Return 400 with "no schema changes detected".

**Rollback**: Creates NEW version (e.g., v6 with v2's schema), doesn't revert `current_version` pointer. Full audit trail preserved.

---

## Column Type Strategy

SQLite has dynamic typing - declared types are just affinity hints. We leverage this for simpler migrations:

### Regular Columns
- **No type declared in SQLite** - columns created without type (e.g., `name NOT NULL` not `name TEXT NOT NULL`)
- **Type stored in template schema only** - for API validation
- **Type changes are instant** - just update template metadata, no migration SQL needed

### Primary Keys
- **Single INTEGER PRIMARY KEY** - enables rowid aliasing and auto-increment optimization
- **Composite PKs** - integer columns in composite PKs also get INTEGER type (expected behavior)
- **PK type changes trigger warning** - CLI prompts: "Changing PK type requires mirror table + data conversion. Continue?"
- **If confirmed**, generate mirror table SQL with CAST for PK and all FK references

```sql
-- Single PK: rowid alias
CREATE TABLE users (id INTEGER PRIMARY KEY, name, email);

-- Composite PK: integer columns typed
CREATE TABLE user_roles (user_id INTEGER, role_id INTEGER, PRIMARY KEY (user_id, role_id));
```

### Operations Requiring Mirror Table

SQLite's ALTER TABLE is limited. These changes require mirror table (create new → copy data → drop old → rename):

| Change | Reason |
|--------|--------|
| PK type change | Recreate with new PK type + CAST data |
| Add FK constraint | Can't ALTER to add foreign key |
| Modify FK constraint | Can't ALTER FK reference or actions |
| Remove FK constraint | Can't ALTER to remove foreign key |
| Add CHECK constraint | Can't ALTER to add CHECK |
| Modify CHECK constraint | Can't ALTER CHECK expression |
| Remove CHECK constraint | Can't ALTER to remove CHECK |
| Change COLLATE | Can't ALTER column collation |
| Modify generated expression | Can't ALTER generated columns |
| Change VIRTUAL ↔ STORED | Can't ALTER generated storage type |
| Convert regular → generated | Can't ALTER to make column generated |
| Convert generated → regular | Can't ALTER to remove generated |

### Operations NOT Requiring Mirror Table

| Change | Method |
|--------|--------|
| Add column | `ALTER TABLE ADD COLUMN` |
| Drop column | `ALTER TABLE DROP COLUMN` (SQLite 3.35+) |
| Rename column | `ALTER TABLE RENAME COLUMN` (SQLite 3.25+) |
| Rename table | `ALTER TABLE RENAME TO` |
| Add/drop index | `CREATE INDEX` / `DROP INDEX` |
| Add/remove UNIQUE | `CREATE UNIQUE INDEX` / `DROP INDEX` |
| Change default value | Metadata only (affects future inserts) |
| Change column type | Metadata only (regular columns) |
| Add NOT NULL (with auto-fix) | `ALTER TABLE ADD COLUMN` with default |
| Add/modify/remove FTS | Separate virtual table + triggers |

### Import (Future Feature)
- Import copies data to new Atomicbase-managed database
- Creates typeless columns (except PKs)
- Full control over schema format

### Benefits
- Type changes are instant for 99% of cases (regular columns)
- Stateless schema files - change anything, system handles it
- Transparent about expensive operations (PK type changes)

---

## Pre-Migration Validation

Before running migrations, validate:

### 1. SQL Syntax Validation
- Parse generated SQL using `EXPLAIN` on in-memory SQLite

### 2. FK Reference Validation
- Verify all FK references point to tables that will exist post-migration

### 3. Data-Dependent Checks (probe first database)

| Check                    | Action                                  |
|--------------------------|-----------------------------------------|
| NOT NULL without default | **Auto-fix**: Add default based on type |
| UNIQUE constraint        | Check for duplicates, fail if exist     |
| CHECK constraint         | Check for violations, fail if exist     |
| FK constraint            | Check for orphan rows, fail if exist    |

### NOT NULL Default Values (Auto-Fix)

| Column Type    | Default Value      |
|----------------|--------------------|
| INTEGER        | `0`                |
| REAL           | `0`                |
| TEXT           | `''`               |
| BLOB           | `X''` (empty blob) |
| Custom/Unknown | `''`               |

---

## Migration Flow

```
POST /templates/{name}/migrate
  │
  ├─ Body: { schema: [...], merge: [...] }
  │
  ▼
┌─────────────────────────┐
│ 1. Generate SQL         │ ← Diff current vs new schema
│    + Apply merges       │ ← Convert drop+add pairs to renames
│    + Auto-fix NOT NULL  │ ← Add type-appropriate defaults
└───────────┬─────────────┘
            │
            ▼
┌─────────────────────────┐
│ 2. Validate             │ ← Syntax, FK refs, data checks
└───────────┬─────────────┘
            │ (If fails, return 400 with errors)
            ▼
┌─────────────────────────┐
│ 3. Create pending       │ ← templates_history: status=pending
│    version N+1          │
└───────────┬─────────────┘
            │
            ▼
┌─────────────────────────┐
│ 4. Migrate first DB     │ ← If fails, abort + mark failed
└───────────┬─────────────┘
            │
            ▼
┌─────────────────────────┐
│ 5. Return job ID        │ ← 202 Accepted
│    (background job)     │
└───────────┬─────────────┘
            │
            ▼
┌─────────────────────────┐
│ 6. Migrate remaining    │ ← 25 concurrent, retry network errors
│    DBs in background    │
└───────────┬─────────────┘
            │
            ▼
┌─────────────────────────┐
│ 7. Mark complete/failed │ ← Update version status
└─────────────────────────┘
```

---

## Migration Details

### Chunking
- Process 25 databases concurrently per batch
- Use Turso batch API (single HTTP request per database)

### Retry Logic
- Retry up to 5 times with exponential backoff
- Only retry on network errors (timeout, connection refused)
- Don't retry on conflicts or constraint violations

### First Database Strategy
- Always migrate first database synchronously
- If first fails, abort entire migration (likely schema issue)
- Prevents wasting time on migrations doomed to fail

### Atomicity
- All changes per database are atomic (batch transaction)
- Database stays at old version if migration fails
- No in-between states

### Batch Updates & Crash Recovery
- Migrate 25 tenants concurrently, then batch-update their `template_version` in one statement
- Reduces primary DB writes from N to N/25
- On crash: some tenants may have migrated but not recorded
- On resume: re-query pending tenants, re-migrate unrecorded ones
- **Safe because migrations are idempotent** (IF EXISTS/IF NOT EXISTS)
- Results logged to `tenant_migrations` table

### Job Recovery on Startup
On server startup:
1. Query for jobs with `status = 'running'`
2. Auto-resume each incomplete job
3. Jobs continue from where they left off (pending tenants query handles this)
4. Safe due to idempotent migrations

### Chained Migrations
Tenants may be at different versions (e.g., after partial migration). Each tenant needs migrations chained from its current version to target:

```
Tenant at v1, target v3: apply v1→v2, then v2→v3
Tenant at v2, target v3: apply v2→v3 only
Tenant at v3, target v3: skip (already current)
```

**Optimized migration logic:**
```go
// 1. Pre-load all needed migrations into cache (1 query per version, not per tenant)
migrations := make(map[int][]string)  // fromVersion -> SQL
for v := minTenantVersion; v < targetVersion; v++ {
    migrations[v] = loadMigration(template_id, v, v+1)
}

// 2. Process tenants in batches of 25
batch := make([]Tenant, 0, 25)
for tenant := range tenants {
    batch = append(batch, tenant)

    if len(batch) == 25 {
        // Migrate all 25 concurrently to Turso
        results := migrateBatchConcurrent(batch, migrations, targetVersion)

        // Batch writes to primary DB (1 UPDATE + 1 INSERT for 25 tenants)
        batchUpdateVersions(successful(results))
        batchInsertTenantMigrations(results)

        batch = batch[:0]
    }
}
// Handle remaining
if len(batch) > 0 {
    results := migrateBatchConcurrent(batch, migrations, targetVersion)
    batchUpdateVersions(successful(results))
    batchInsertTenantMigrations(results)
}
```

**Critical:** Every schema push MUST create a migration record (from_version → to_version), even if no tenants exist yet. Future tenants at older versions will need it.

### SQL Statement Ordering
Generated SQL follows this order so names and dependencies are available:

1. **Renames first** (new names available for subsequent statements)
   - Table renames
   - Column renames

2. **Adds second** (dependency order)
   - Add tables
   - Add columns / mirror table operations
   - Add indexes
   - Add FTS

3. **Drops last** (reverse dependency order)
   - Drop FTS
   - Drop indexes
   - Drop columns
   - Drop tables

---

## Files to Create

### `api/platform/types.go`
- Schema, Table, Col, Index, Generated
- SchemaDiff, DiffResult, Merge, MigrationPlan
- Migration (combined SQL + job state), TenantMigration
- ValidationError type

### `api/platform/handlers.go`
- RegisterRoutes(mux *http.ServeMux)
- Handler functions for all endpoints
- withPrimary() wrapper pattern

### `api/platform/tenants.go`
- ListTenants, GetTenant, CreateTenant, DeleteTenant, SyncTenant
- Turso API integration for DB creation/deletion
- Token management

### `api/platform/templates.go`
- ListTemplates, GetTemplate, CreateTemplate
- DiffTemplate (returns raw changes)
- GetTemplateHistory

### `api/platform/migrations.go`
- GenerateMigrationPlan (diff + SQL generation)
- MigrateDatabase, MigrateDatabaseBatch
- Mirror table generation for complex changes (batch all changes per table into single mirror operation)
- Apply merges (convert drop+add pairs to renames by index)

### `api/platform/validation.go`
- ValidateMigrationPlan
- validateSyntax, validateFKReferences
- validateDataConstraints (NOT NULL, UNIQUE, CHECK, FK)
- autoFixNotNull (add type-appropriate defaults)

### `api/platform/batch.go`
- BatchExecute for Turso pipeline API
- Request/response types

### `api/platform/jobs.go`
- JobManager with background workers
- Job persistence in primary DB
- Progress tracking and status updates

---

## Database Tables

```sql
-- atomicbase_schema_templates
id, name, current_version, created_at, updated_at

-- atomicbase_templates_history
id, template_id, version, schema (BLOB), checksum, created_at

-- atomicbase_tenants
id, name, token, template_id, template_version

-- atomicbase_migrations (CRITICAL - combined SQL + job state)
id, template_id, from_version, to_version,
sql (TEXT),                    -- Migration SQL statements (JSON array)
status,                        -- pending, running, paused, complete
state,                         -- null, success, partial, failed
total_dbs, completed_dbs, failed_dbs,
started_at, completed_at, created_at

-- atomicbase_tenant_migrations (per-tenant outcome)
migration_id, tenant_id, status (success/failed), error, attempts, updated_at
```

**Notes**:
- `migrations`: **CRITICAL** - stores SQL for EVERY version transition + execution state. The migration.id is the job ID.
- `migrations.status`: what it's doing (pending/running/paused/complete)
- `migrations.state`: what happened (null/success/partial/failed)
- `tenant_migrations`: tracks per-tenant outcome. Pending = not in table. Insert on success OR failure.

**Queries:**
```sql
-- Pending tenants (not yet attempted)
SELECT t.* FROM tenants t
WHERE t.template_version < ?
  AND NOT EXISTS (SELECT 1 FROM tenant_migrations tm WHERE tm.tenant_id = t.id AND tm.migration_id = ?)

-- Failed tenants
SELECT t.*, tm.error, tm.attempts FROM tenants t
JOIN tenant_migrations tm ON tm.tenant_id = t.id
WHERE tm.migration_id = ? AND tm.status = 'failed'

-- Successful tenants
SELECT t.* FROM tenants t
JOIN tenant_migrations tm ON tm.tenant_id = t.id
WHERE tm.migration_id = ? AND tm.status = 'success'
```

---

## Verification

1. **Unit tests**: Each file gets corresponding `_test.go`
2. **Integration tests**: Full workflow with actual Turso (use test org)
3. **Manual testing**:
   - Create template → Create database → Query via data API
   - Update template → Diff → Migrate → Verify schema updated
   - Rollback → Verify version changed

### Test Commands
```bash
# Unit tests
CGO_ENABLED=1 go test -tags fts5 -v ./platform/...

# Integration tests
CGO_ENABLED=1 go test -tags "fts5 integration" -v ./platform/...
```

---

## Implementation Order

1. **types.go** - Define all types
2. **batch.go** - Turso batch API client (copy from docs/platform/)
3. **templates.go** - Template CRUD + diff
4. **migrations.go** - Migration plan + SQL generation
5. **validation.go** - Pre-migration validation
6. **tenants.go** - Tenant CRUD + sync
7. **jobs.go** - Background job system
8. **handlers.go** - HTTP handlers + route registration
9. **main.go** - Register platform routes
