# Database Architecture

Atomicbase uses multiple SQLite databases to separate concerns, isolate failure domains, and enable independent scaling.

## Overview

```
┌──────────────────────────────────────────────────────────────────────────┐
│                         Database Architecture                            │
├──────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│   sessions.db          tenants.db           storage.db      logs.db      │
│   ┌────────────┐      ┌──────────────┐       ┌──────────┐    ┌─────────┐ │
│   │  sessions  │      │  users       │       │  files   │    │  logs   │ │
│   └────────────┘      │  user_dbs    │       │  chunks  │    │  audit  │ │
│                       │  orgs        │       └──────────┘    └─────────┘ │
│   Ephemeral           │  org_members │         Future          Future    │
│   Redis-swappable     │  databases   │                                   │
│                       │  templates   │                                   │
│                       └──────────────┘                                   │
│                        Permanent                                         │
│                        Relational                                        │
│                                                                          │
└──────────────────────────────────────────────────────────────────────────┘
```

## Why Separate Databases?

| Benefit | Description |
|---------|-------------|
| **Isolation** | Runaway logging can't slow down auth |
| **Independent scaling** | Upgrade just the bottleneck |
| **Different access patterns** | Read-heavy vs write-heavy |
| **Operational flexibility** | Back up sessions less, rotate logs aggressively |
| **Swappability** | Replace sessions.db with Redis without touching identity |

## Database Details

### sessions.db

**Purpose:** Ephemeral authentication state

**Tables:**
```sql
CREATE TABLE atomicbase_sessions (
  id TEXT PRIMARY KEY,              -- SHA-256 hash of token
  public_id TEXT UNIQUE NOT NULL,   -- For session management API
  user_id TEXT NOT NULL,            -- References _users in tenants.db
  expires_at INTEGER NOT NULL,
  created_at INTEGER NOT NULL,
  user_agent TEXT,
  ip_address TEXT
);

CREATE INDEX atomicbase_sessions_expires_at ON atomicbase_sessions(expires_at);
CREATE INDEX atomicbase_sessions_user_id ON atomicbase_sessions(user_id);
```

**Characteristics:**
- Read-heavy (validated on every authenticated request)
- Volatile (sessions created/destroyed frequently)
- Small rows (~300 bytes each)
- Can be wiped without data loss (users just re-login)

**Scale limit:** ~10M MAU with SQLite, then swap to Redis

**Why separate:**
- Ephemeral state shouldn't mix with permanent data
- Easy Redis migration path (just key-value + TTL)
- Can tune aggressively for read performance

---

### tenants.db

**Purpose:** Identity, access control, and tenant metadata

**Tables:**
```sql
-- Identity
CREATE TABLE atomicbase_users (
  id TEXT PRIMARY KEY,
  email TEXT NOT NULL COLLATE NOCASE,
  email_verified INTEGER DEFAULT 0,
  password_hash TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  UNIQUE(email)
);

-- Access control
CREATE TABLE atomicbase_user_databases (
  id INTEGER PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES atomicbase_users(id) ON DELETE CASCADE,
  database_id INTEGER NOT NULL REFERENCES atomicbase_databases(id) ON DELETE CASCADE,
  role TEXT NOT NULL DEFAULT 'member',
  created_at INTEGER NOT NULL,
  UNIQUE(user_id, database_id)
);

CREATE INDEX atomicbase_user_databases_user ON atomicbase_user_databases(user_id);
CREATE INDEX atomicbase_user_databases_database ON atomicbase_user_databases(database_id);

-- Organizations (optional)
CREATE TABLE atomicbase_organizations (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  created_at INTEGER NOT NULL
);

CREATE TABLE atomicbase_organization_members (
  id INTEGER PRIMARY KEY,
  organization_id TEXT NOT NULL REFERENCES atomicbase_organizations(id) ON DELETE CASCADE,
  user_id TEXT NOT NULL REFERENCES atomicbase_users(id) ON DELETE CASCADE,
  role TEXT NOT NULL DEFAULT 'member',
  created_at INTEGER NOT NULL,
  UNIQUE(organization_id, user_id)
);

-- Tenant databases
CREATE TABLE atomicbase_databases (
  id INTEGER PRIMARY KEY,
  name TEXT UNIQUE NOT NULL,
  template_id INTEGER REFERENCES atomicbase_templates(id),
  schema_version INTEGER DEFAULT 1,
  token TEXT NOT NULL,              -- Turso auth token
  organization_id TEXT REFERENCES atomicbase_organizations(id),
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Schema templates
CREATE TABLE atomicbase_templates (
  id INTEGER PRIMARY KEY,
  name TEXT UNIQUE NOT NULL,
  current_version INTEGER DEFAULT 1,
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE atomicbase_templates_history (
  id INTEGER PRIMARY KEY,
  template_id INTEGER NOT NULL REFERENCES atomicbase_templates(id),
  version INTEGER NOT NULL,
  tables BLOB NOT NULL,             -- Gob-encoded table definitions
  schema BLOB NOT NULL,             -- Gob-encoded SchemaCache
  checksum TEXT NOT NULL,
  changes TEXT,                     -- JSON array of changes
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE(template_id, version)
);

-- Sync jobs
CREATE TABLE atomicbase_sync_jobs (
  id TEXT PRIMARY KEY,
  template_id INTEGER NOT NULL REFERENCES atomicbase_templates(id),
  target_version INTEGER NOT NULL,
  status TEXT NOT NULL DEFAULT 'running',
  total_count INTEGER NOT NULL,
  completed_count INTEGER DEFAULT 0,
  failed_count INTEGER DEFAULT 0,
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  completed_at TEXT
);

CREATE TABLE atomicbase_sync_tasks (
  id INTEGER PRIMARY KEY,
  job_id TEXT NOT NULL REFERENCES atomicbase_sync_jobs(id),
  database_id INTEGER NOT NULL REFERENCES atomicbase_databases(id),
  status TEXT NOT NULL DEFAULT 'pending',
  attempts INTEGER DEFAULT 0,
  error TEXT,
  started_at TEXT,
  completed_at TEXT
);
```

**Characteristics:**
- Read-heavy (tenant lookup on every request)
- Permanent data (user accounts, tenant configs)
- Relational (access control joins users ↔ databases)
- Small-medium size

**Why `atomicbase_users` and `atomicbase_databases` are together:**
```sql
-- Hot path query: check access + get tenant info in ONE query
SELECT d.template_id, d.schema_version, d.token, ud.role
FROM atomicbase_user_databases ud
JOIN atomicbase_databases d ON d.id = ud.database_id
WHERE ud.user_id = ? AND d.name = ?
```

Splitting would require 2 queries and lose FK constraints.

---

### storage.db (Future)

**Purpose:** File storage metadata

**Tables:**
```sql
CREATE TABLE atomicbase_files (
  id TEXT PRIMARY KEY,
  database_id INTEGER NOT NULL,     -- Which tenant owns this
  path TEXT NOT NULL,
  size INTEGER NOT NULL,
  mime_type TEXT,
  checksum TEXT,
  storage_key TEXT NOT NULL,        -- Key in S3/R2/disk
  created_at INTEGER NOT NULL,
  UNIQUE(database_id, path)
);

CREATE INDEX atomicbase_files_database ON atomicbase_files(database_id);
```

**Characteristics:**
- Mixed read/write
- Can grow large (many files per tenant)
- Actual files stored externally (S3/R2/disk)

**Why separate:**
- Different growth pattern than identity/tenants
- Can shard by tenant if needed
- Isolation from core auth/tenant operations

---

### logs.db (Future)

**Purpose:** Audit trails, request logs, error tracking

**Tables:**
```sql
CREATE TABLE atomicbase_request_logs (
  id INTEGER PRIMARY KEY,
  database_id INTEGER,
  user_id TEXT,
  method TEXT NOT NULL,
  path TEXT NOT NULL,
  status INTEGER NOT NULL,
  duration_ms INTEGER,
  created_at INTEGER NOT NULL
);

CREATE TABLE atomicbase_audit_logs (
  id INTEGER PRIMARY KEY,
  database_id INTEGER NOT NULL,
  user_id TEXT,
  action TEXT NOT NULL,             -- 'insert', 'update', 'delete'
  table_name TEXT NOT NULL,
  row_id TEXT,
  changes TEXT,                     -- JSON diff
  created_at INTEGER NOT NULL
);

CREATE INDEX atomicbase_request_logs_created ON atomicbase_request_logs(created_at);
CREATE INDEX atomicbase_audit_logs_database ON atomicbase_audit_logs(database_id, created_at);
```

**Characteristics:**
- Write-heavy (every request)
- Append-only
- Can grow very large
- Reads are infrequent (analytics, debugging)

**Why separate:**
- Write-heavy pattern different from read-heavy auth
- Can rotate/archive independently
- Runaway logging can't affect core functionality
- Can batch writes for performance

---

## Request Flow

```
┌────────────────────────────────────────────────────────────────────────┐
│                     Authenticated Request Flow                          │
├────────────────────────────────────────────────────────────────────────┤
│                                                                        │
│   1. Extract session token from cookie                                 │
│                                                                        │
│   2. Validate session                              ┌─────────────────┐ │
│      SELECT user_id, expires_at                    │  sessions.db    │ │
│      FROM _sessions WHERE id = ?     ────────────▶ │                 │ │
│                                                    └─────────────────┘ │
│                                                                        │
│   3. Check access + get tenant info                ┌─────────────────┐ │
│      SELECT d.template_id, d.schema_version,       │  tenants.db     │ │
│             d.token, ud.role                       │                 │ │
│      FROM _user_databases ud           ──────────▶ │                 │ │
│      JOIN __databases d ON ...                     └─────────────────┘ │
│      WHERE ud.user_id = ? AND d.name = ?                               │
│                                                                        │
│   4. Get schema from cache (memory)                                    │
│      schemaCache.Load(templateID:version)                              │
│                                                                        │
│   5. Execute query against tenant's Turso DB                           │
│                                                                        │
│   6. Log request (async)                           ┌─────────────────┐ │
│      INSERT INTO _request_logs ...     ──────────▶ │  logs.db        │ │
│                                                    └─────────────────┘ │
│                                                                        │
└────────────────────────────────────────────────────────────────────────┘
```

**Per-request database hits:**
- sessions.db: 1 read
- tenants.db: 1 read
- logs.db: 1 write (async, batched)

---

## Configuration

All databases use WAL mode for concurrent read performance:

```go
func openDB(path string) (*sql.DB, error) {
    db, err := sql.Open("sqlite3", fmt.Sprintf(
        "file:%s?_journal_mode=WAL&_synchronous=NORMAL&_busy_timeout=5000",
        path,
    ))
    if err != nil {
        return nil, err
    }

    // Connection pool settings
    db.SetMaxOpenConns(10)
    db.SetMaxIdleConns(5)

    return db, nil
}
```

**Environment variables:**
```bash
ATOMICBASE_SESSIONS_DB=./data/sessions.db
ATOMICBASE_TENANTS_DB=./data/tenants.db
ATOMICBASE_STORAGE_DB=./data/storage.db
ATOMICBASE_LOGS_DB=./data/logs.db
```

---

## Scale Limits

| Database | Bottleneck | Comfortable Limit | Upgrade Path |
|----------|------------|-------------------|--------------|
| sessions.db | Read throughput | ~10M MAU | Redis/Valkey |
| tenants.db | Read throughput | ~10M tenants | Read replicas, Turso |
| storage.db | Row count | ~100M files | Shard by tenant |
| logs.db | Write throughput | ~10K req/sec | Batch writes, rotate |

---

## Tenant Databases

Tenant databases (the actual user data) are completely separate from these infrastructure databases. They're hosted on Turso and contain only user-defined tables from schema templates.

**Key principle:** Tenant databases have no hidden tables. Users have full control and visibility over their data.
