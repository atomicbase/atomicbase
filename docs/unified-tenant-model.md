# Unified Tenant Model Architecture

## Summary

Replace the current Template/Database model with a unified Tenant model that couples ownership, schema, RBAC, and RLS into cohesive objects.

## Core Concepts

### Tenant (Multi-instance, full access control)
- **Definition** (`.tenant.ts`): schema + roles + grants (RBAC+RLS) + quota defaults
- **Instance**: actual customer database with users, metadata, quota values
- Access control: session context + organization roles + row context

### Global (Single-instance, simpler access control)
- **Definition** (`.global.ts`): schema + policies (RLS only) + settings
- Creates single database directly (1:1)
- Access control: session context + row context (NO roles/RBAC)

### Shared Infrastructure
- Schema engine identical for both (tables, columns, indexes, FTS)

### Separate Migration Processes
- **Tenant migrations**: Update definition version → propagate to all instances (tracked per-instance)
- **Global migrations**: Direct migration of single database (simpler, no propagation)
- Both use the same schema diffing/SQL generation, but different execution paths

### Role Inheritance Model
Roles use `extends` for inheritance (recursive, like class inheritance):

```typescript
roles: {
  viewer: {},
  member: { extends: "viewer" },
  admin: { extends: "member" },
  owner: { extends: "admin" },
  billing_admin: { extends: "member" },  // non-linear branch
}
```

- `extends` means "everything parent has, plus this role"
- Single parent for simple hierarchies, array for multiple parents: `{ extends: ["admin", "billing_admin"] }`
- Recursion is always applied (owner extends admin extends member extends viewer)

**CLI shows flattened hierarchy on push:**
```
Roles:
  viewer        → [viewer]
  member        → [member, viewer]
  admin         → [admin, member, viewer]
  owner         → [owner, admin, member, viewer]
  billing_admin → [billing_admin, member, viewer]
```

In grants, use role checks:
```typescript
delete: g.where(({ auth }) => inList(auth.role, ["owner", "admin"]))
// Or with a helper:
delete: g.where(({ auth }) => auth.roleAtLeast("admin"))
```

## File Naming Convention

```
definitions/
  +customer.tenant.ts      # CLI processes (tenant definition)
  +marketplace.global.ts # CLI processes (database definition)
  shared-columns.ts        # Helper file, importable but not processed
  grants-helpers.ts        # Helper file, importable but not processed
```

- `+*.tenant.ts` - Tenant definitions (CLI processes)
- `+*.global.ts` - Database definitions (CLI processes)
- `*.ts` - Helper files (can be imported by definitions, CLI ignores)

**Name resolution:** Derived from filename (`+customer.tenant.ts` → `"customer"`).

**CLI validation:**
- `+*.tenant.ts` must `export default defineTenant(...)` — error otherwise
- `+*.global.ts` must `export default defineGlobal(...)` — error otherwise
- Tenants and globals have separate namespaces (can have same name)

**Push/Pull behavior:**
- `push` evaluates the TypeScript and sends the flattened schema to the API
- `pull` writes the flattened schema back to `+*.tenant.ts` / `+*.global.ts`
- Helper files are local-only convenience — pull will overwrite `+*` files with flattened versions
- Refactor into helpers after pulling if desired

## File Formats

**Tenant definition** (`definitions/+customer.tenant.ts`):
```typescript
import { defineTenant, defineTable, c, defineGrants, g, eq, and, inList } from "@atomicbase/definitions";

export default defineTenant({
  schema: {
    projects: defineTable({
      id: c.integer().primaryKey(),
      name: c.text().notNull(),
      org_id: c.text().notNull(),
    }),
  },
  roles: {
    viewer: {},
    member: { extends: "viewer" },
    admin: { extends: "member" },
    owner: { extends: "admin" },
  },
  grants: defineGrants({
    projects: {
      select: g.where(({ auth, row }) => eq(row.org_id, auth.org.id)),
      delete: g.where(({ auth, row }) =>
        and(eq(row.org_id, auth.org.id), inList(auth.role, ["owner", "admin"]))
      ),
    },
  }),
  quotas: { max_users: 50 },
});
```

**Database definition** (`definitions/+marketplace.global.ts`):
```typescript
import { defineGlobal, defineTable, c, definePolicies, p, eq } from "@atomicbase/definitions";

export default defineGlobal({
  schema: {
    extensions: defineTable({
      id: c.integer().primaryKey(),
      author_id: c.text().notNull(),
      name: c.text().notNull(),
    }),
  },
  policies: definePolicies({
    extensions: {
      select: p.allow(),
      insert: p.where(({ auth, next }) => eq(next.author_id, auth.id)),
    },
  }),
});
```

## New Platform Tables

```sql
-- Tenant definitions (replaces templates for multi-tenant use)
CREATE TABLE atomicbase_tenant_definitions (
    id INTEGER PRIMARY KEY,
    name TEXT UNIQUE NOT NULL,
    schema_json BLOB NOT NULL,
    roles_json BLOB NOT NULL,
    grants_json BLOB,
    quota_defaults_json BLOB,
    current_version INTEGER DEFAULT 1,
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT DEFAULT CURRENT_TIMESTAMP
);

-- Tenant instances (actual customer databases)
CREATE TABLE atomicbase_tenant_instances (
    id INTEGER PRIMARY KEY,
    name TEXT UNIQUE NOT NULL,
    definition_id INTEGER NOT NULL REFERENCES atomicbase_tenant_definitions(id),
    definition_version INTEGER NOT NULL,
    metadata_json BLOB,      -- stripe_subscription_id, plan, etc.
    quotas_json BLOB,        -- override quota values
    status TEXT DEFAULT 'active',
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT DEFAULT CURRENT_TIMESTAMP
);

-- Users within tenant instances
CREATE TABLE atomicbase_tenant_users (
    id TEXT PRIMARY KEY,
    tenant_instance_id INTEGER NOT NULL REFERENCES atomicbase_tenant_instances(id),
    user_id TEXT NOT NULL,
    role TEXT NOT NULL,
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(tenant_instance_id, user_id)
);

-- Global databases (1:1, separate namespace from tenants)
CREATE TABLE atomicbase_global_definitions (
    id INTEGER PRIMARY KEY,
    name TEXT UNIQUE NOT NULL,
    schema_json BLOB NOT NULL,
    policies_json BLOB,
    settings_json BLOB,
    current_version INTEGER DEFAULT 1,
    turso_db_name TEXT,
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT DEFAULT CURRENT_TIMESTAMP
);
```

## API Endpoints

### Tenant Definitions
- `POST /platform/tenants/definitions` - Create definition
- `GET /platform/tenants/definitions/{name}` - Get definition
- `POST /platform/tenants/definitions/{name}/migrate` - Apply schema changes

### Tenant Instances
- `POST /platform/tenants/instances` - Create instance
- `GET /platform/tenants/instances/{name}` - Get instance
- `PATCH /platform/tenants/instances/{name}` - Update metadata/quotas
- `POST /platform/tenants/instances/{name}/users` - Add user with role

### Globals
- `POST /platform/globals` - Create global (definition + database)
- `GET /platform/globals/{name}` - Get global
- `POST /platform/globals/{name}/migrate` - Apply schema changes

## Request Headers

```
Authorization: Bearer <session_token>   # User session
X-Tenant: <tenant_instance_name>        # For tenant requests (has RBAC)
X-Global: <global_name>                 # For global requests (no RBAC)
```

## Implementation Phases

### Phase 1: TypeScript packages
1. Create `@atomicbase/definitions` package with condition primitives
2. Add `defineTenant()` and `defineGlobal()` to `@atomicbase/definitions`
3. Update CLI parser for `.tenant.ts` and `.global.ts` file detection

### Phase 2: Go API - Definitions
1. Add new platform tables to `api/platform/base.go`
2. Add types to `api/platform/types.go`
3. Create `api/platform/tenant_definitions.go` - CRUD operations
4. Create `api/platform/global_definitions.go` - CRUD operations

### Phase 3: Go API - Instances
1. Create `api/platform/tenant_instances.go` - CRUD + user management
2. Update migration engine to work with both tenant instances and databases
3. Add quota enforcement hooks

### Phase 4: Auth Context
1. Update `api/tools/auth.go` with auth context extraction
2. Tenant user role lookup from `atomicbase_tenant_users`
3. Inject policy conditions into queries based on grants/policies

### Phase 5: Migration path
1. CLI commands to migrate existing templates to tenant definitions
2. Deprecation warnings for `.schema.ts` files
3. Keep existing `/platform/templates/*` endpoints working

## Critical Files to Modify

**Go API:**
- `api/platform/types.go` - Add TenantDefinition, TenantInstance, GlobalDefinition types
- `api/platform/base.go` - Add new platform tables
- `api/platform/handlers.go` - Register new routes
- `api/tools/auth.go` - Auth context middleware

**TypeScript:**
- `packages/definitions/src/index.ts` - New unified package (defineTenant, defineGlobal, grants, policies)
- `packages/cli/src/schema/parser.ts` - File type detection (.tenant.ts, .global.ts)
- `packages/cli/src/commands/templates.ts` - Route to correct API based on type

**New files:**
- `packages/definitions/` - New package replacing template + access
- `api/platform/tenant_definitions.go`
- `api/platform/tenant_instances.go`
- `api/platform/global_definitions.go`

## Verification

1. Create a `.tenant.ts` file with schema, roles, and grants
2. Push via CLI - verify definition created in platform DB
3. Create tenant instance via API
4. Add user with role to instance
5. Make data requests with different roles - verify RBAC enforcement
6. Create a `.global.ts` file with schema and policies
7. Push via CLI - verify database created (1:1)
8. Make data requests - verify RLS enforcement (no roles)
9. Update schema - verify migration works for both types