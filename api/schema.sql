-- Schema templates for multi-tenant database management
CREATE TABLE IF NOT EXISTS atomicbase_schema_templates (
    id INTEGER PRIMARY KEY,
    name TEXT UNIQUE NOT NULL,
    current_version INTEGER DEFAULT 1,
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_atomicbase_templates_name ON atomicbase_schema_templates(name);

-- Tenant database metadata
CREATE TABLE IF NOT EXISTS atomicbase_databases (
    id INTEGER PRIMARY KEY,
    name TEXT UNIQUE,
    token TEXT,
    template_id INTEGER REFERENCES atomicbase_schema_templates(id),
    template_version INTEGER DEFAULT 1,
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_atomicbase_databases_name ON atomicbase_databases(name);

-- Version history for schema templates
CREATE TABLE IF NOT EXISTS atomicbase_templates_history (
    id INTEGER PRIMARY KEY,
    template_id INTEGER NOT NULL REFERENCES atomicbase_schema_templates(id),
    version INTEGER NOT NULL,
    schema BLOB NOT NULL,
    checksum TEXT NOT NULL,
    changes TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(template_id, version)
);
CREATE INDEX IF NOT EXISTS idx_templates_history_template ON atomicbase_templates_history(template_id);

CREATE TABLE IF NOT EXISTS atomicbase_migrations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    template_id INTEGER NOT NULL REFERENCES atomicbase_schema_templates(id),
    from_version INTEGER NOT NULL,
    to_version INTEGER NOT NULL,
    sql TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'ready',
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_migrations_template ON atomicbase_migrations(template_id);
CREATE INDEX IF NOT EXISTS idx_migrations_versions ON atomicbase_migrations(template_id, from_version, to_version);

-- Migration failures for debugging lazy migrations
CREATE TABLE IF NOT EXISTS atomicbase_migration_failures (
    database_id INTEGER PRIMARY KEY REFERENCES atomicbase_databases(id),
    from_version INTEGER NOT NULL,
    to_version INTEGER NOT NULL,
    error TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_migration_failures_created_at ON atomicbase_migration_failures(created_at);
