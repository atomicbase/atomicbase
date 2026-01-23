# Platform Architecture

**Atomicbase: The multi-tenancy platform for B2B SaaS.**

## The Problem

Building multi-tenant SaaS is hard. You need:

- **Tenant isolation** - Each customer's data must be separate
- **Organization modeling** - Users, roles, teams, invites
- **Schema management** - Deploy changes across many databases
- **Per-tenant billing** - Usage metering, seat-based pricing **Coming soon**
- **Tenant-aware storage** - Files isolated per customer

Today, you piece this together from 5+ services (Clerk, Turso, Stripe, S3, custom code) and spend months on plumbing instead of your product.

## The Solution

Atomicbase handles multi-tenancy so you can focus on your product.

```
┌────────────────────────────────────────────────────────────────────────────┐
│                              Atomicbase                                    │
│                    Multi-tenancy platform for B2B SaaS                     │
├────────────────────────────────────────────────────────────────────────────┤
│                                                                            │
│   ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐       │
│   │    Auth     │  │    Data     │  │   Storage   │  │   Billing   │       │
│   │             │  │             │  │             │  │             │       │
│   │ • Users     │  │ • CRUD API  │  │ • Uploads   │  │ • Usage     │       │
│   │ • Orgs      │  │ • Schemas   │  │ • Transforms│  │ • Seats     │       │
│   │ • Roles     │  │ • Migrations│  │ • CDN       │  │ • Stripe    │       │
│   │ • SSO       │  │ • Syncing   │  │ • Caching   │  │ • Invoices  │       │
│   │ + More      │  │ • DB routing│  │ • Isolation │  └─────────────┘       │
│   └─────────────┘  └─────────────┘  └─────────────┘                        │
│                                                                            │
│   ┌─────────────────────────────────────────────────────────────────┐      │
│   │                     Tenant Databases                            │      │
│   │    ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐           │      │
│   │    │ Acme Co │  │ Beta Inc│  │Gamma LLC│  │  ...    │           │      │
│   │    └─────────┘  └─────────┘  └─────────┘  └─────────┘           │      │
│   └─────────────────────────────────────────────────────────────────┘      │
│                                                                            │
└────────────────────────────────────────────────────────────────────────────┘
```

## Core Concepts

### 1. Users

```typescript
// User signs up
const { user } = await atomicbase.users.register({
  email: 'user@example.com',
  password: '...',
  metadata: { ... },
})

// Grant user access to a database (direct access = full permissions)
await atomicbase.users.grantDbAccess(user.id, db.id)

// List databases the user has direct access to
const databases = await atomicbase.users.listDatabases(user.id)
// [{ id, name, template, granted_at }]

// Check if user has direct access to a specific database
const access = await atomicbase.users.getDbAccess(user.id, db.id)
// { granted_at: 1705312200 } or null

// Revoke access
await atomicbase.users.revokeDbAccess(user.id, db.id)
```

### 2. Databases

Databases are **independent entities**. They're not automatically tied to users or organizations.

```typescript
// Create a database
const db = await atomicbase.databases.create({
  name: 'acme-prod',
  template: 'saas-app',  // Uses schema template
})

// Get a client scoped to this database
const client = atomicbase.database(db.id)

// Queries start with .from() - no extra nesting
const { data } = await client.from('projects').select()
```

**Access model:**
- **Direct user access** → full permissions (no roles)
- **Via organization** → permissions based on org membership role

**This flexibility supports:**
- One database, multiple users (team without formal org)
- One org, multiple databases (prod, staging, regional shards)
- One user, multiple databases (personal + work projects)
- Standalone databases (API-only, service accounts)

### 3. Organizations (Optional)

Organizations group users together. **Fully-featured but completely optional** - like Clerk.

**Roles live on org membership**, not on database access:

```typescript
// Create an organization
const org = await atomicbase.orgs.create({
  name: 'Acme Corporation',
  owner: userId,  // Creator becomes owner
})

// Grant org access to a database
// Members access based on their org role
await atomicbase.orgs.grantDbAccess(org.id, db.id)

// Invite members with a role
await atomicbase.orgs.invite({
  orgId: org.id,
  email: 'colleague@example.com',
  role: 'admin',  // owner | admin | member | viewer
})

// Change a member's role
await atomicbase.orgs.updateMemberRole({
  orgId: org.id,
  userId: memberId,
  role: 'member',
})

// List databases the org has access to
const databases = await atomicbase.orgs.listDatabases(org.id)
// [{ id, name, template, granted_at }]

// Check/revoke access
const access = await atomicbase.orgs.getDbAccess(org.id, db.id)
await atomicbase.orgs.revokeDbAccess(org.id, db.id)
```

**Role permissions (customizable by the app developer):**
| Role | Typical permissions |
|------|---------------------|
| owner | Full access, can delete org, manage billing |
| admin | Full data access, can invite/remove members |
| member | Read/write data |
| viewer | Read-only data |

**Organization features:**
- Member management (invite, remove, roles)
- Role-based permissions on org's databases
- SSO/SAML per organization (enterprise)

### Convenience Patterns

For common patterns, provide shortcuts that handle the wiring:

```typescript
// Pattern: Database-per-user (personal apps)
const { user, database } = await atomicbase.users.register({
  email: 'user@example.com',
  password: '...',
  provisionDatabase: { template: 'notes-app' },  // Creates DB + grants access
})

// Pattern: Database-per-org (B2B SaaS)
const org = await atomicbase.orgs.create({
  name: 'Acme Corp',
  owner: userId,
  provisionDatabase: { template: 'saas-app' },  // Creates DB + grants access
})
```

**Access patterns supported:**
- **Database-per-user** - Personal apps, note-taking tools
- **Database-per-org** - B2B SaaS, team tools
- **Shared databases** - Multiple users/orgs with access to same DB
- **Multi-database** - User or org with access to multiple DBs

### 3. Schema Templates

Define your schema once in TypeScript. Push to all tenant databases.

```typescript
// schemas/saas-app.schema.ts
import { defineSchema, defineTable, c } from '@atomicbase/schema'

export default defineSchema('saas-app', {
  projects: defineTable({
    id: c.text().primaryKey(),
    name: c.text().notNull(),
    created_by: c.text().notNull(),
    created_at: c.integer().notNull(),
  }),

  tasks: defineTable({
    id: c.text().primaryKey(),
    project_id: c.text().notNull().references('projects.id'),
    title: c.text().notNull(),
    status: c.text().notNull().default('todo'),
    assigned_to: c.text(),
  }).index('idx_project', ['project_id']),
})
```

```bash
# Push schema to ALL tenant databases
$ atomicbase push

Pushing schema "saas-app" v3 → v4...
Changes:
  + tasks.assigned_to (TEXT)
  + idx_project on tasks(project_id)

Syncing 847 tenant databases...
[████████████████████████████████████████] 100%
✓ 847 databases updated in 34s
```

### 4. Per-Tenant Billing

Usage metering and billing built-in. Integrates with Stripe.

```typescript
// Atomicbase tracks automatically:
// - Database rows per org
// - Storage bytes per org
// - API requests per org
// - Seats (members) per org

// Get usage for billing
const usage = await atomicbase.billing.getUsage({
  orgId: 'acme-corp',
  period: 'current_month',
})
// { rows: 15420, storage_bytes: 52428800, requests: 89000, seats: 12 }

// Report to Stripe
await atomicbase.billing.reportToStripe({
  orgId: 'acme-corp',
  stripeCustomerId: 'cus_xxx',
  stripeSubscriptionId: 'sub_xxx',
})
```

**Billing models supported:**
- Seat-based (per member)
- Usage-based (rows, storage, requests)
- Flat rate with limits
- Hybrid (base + overage)

### 5. Tenant-Isolated Storage

File storage with automatic tenant isolation.

```typescript
// Upload to org's storage (automatically scoped)
const file = await atomicbase.storage.upload({
  orgId: 'acme-corp',
  path: 'avatars/user-123.jpg',
  file: imageBuffer,
})

// Get with transformations
const url = atomicbase.storage.getUrl({
  orgId: 'acme-corp',
  path: 'avatars/user-123.jpg',
  transform: { width: 200, height: 200, format: 'webp' },
})
// https://cdn.atomicbase.dev/acme-corp/avatars/user-123.jpg?w=200&h=200&f=webp
```

**Storage features:**
- Per-org isolation (can't access other org's files)
- Image transformations (resize, crop, format)
- CDN delivery
- Usage tracking for billing

---

## Architecture

### Service Overview

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                              Atomicbase Platform                              │
├──────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│                            ┌─────────────────┐                               │
│                            │   API Gateway   │                               │
│                            └────────┬────────┘                               │
│                                     │                                        │
│      ┌──────────────────────────────┼──────────────────────────────┐         │
│      │            │                 │                 │            │         │
│      ▼            ▼                 ▼                 ▼            ▼         │
│ ┌─────────┐ ┌──────────┐     ┌──────────┐     ┌──────────┐  ┌──────────┐    │
│ │  Auth   │ │   Data   │     │ Storage  │     │ Billing  │  │ Platform │    │
│ │ Service │ │   API    │     │ Service  │     │ Service  │  │ Service  │    │
│ └────┬────┘ └────┬─────┘     └────┬─────┘     └────┬─────┘  └────┬─────┘    │
│      │           │                │                │             │           │
│      │           │           ┌────┴────┐           │             │           │
│      │           │           │imgproxy │           │             │           │
│      │           │           └─────────┘           │             │           │
│      │           │                                 │             │           │
│ ┌────┴───────────┴─────────────────────────────────┴─────────────┴────┐      │
│ │                         Infrastructure DBs                           │      │
│ │   ┌────────────┐  ┌────────────┐  ┌────────────┐  ┌────────────┐    │      │
│ │   │sessions.db │  │ tenants.db │  │ storage.db │  │  usage.db  │    │      │
│ │   └────────────┘  └────────────┘  └────────────┘  └────────────┘    │      │
│ └──────────────────────────────────────────────────────────────────────┘      │
│                                                                              │
│ ┌──────────────────────────────────────────────────────────────────────┐     │
│ │                       Tenant Databases (Turso)                        │     │
│ │                                                                       │     │
│ │   One database per organization, edge-replicated globally             │     │
│ │                                                                       │     │
│ │   ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐            │     │
│ │   │ acme-co  │  │ beta-inc │  │ gamma-llc│  │   ...    │            │     │
│ │   └──────────┘  └──────────┘  └──────────┘  └──────────┘            │     │
│ └──────────────────────────────────────────────────────────────────────┘     │
│                                                                              │
│ ┌───────────────────────────────────────────────────────────────────┐        │
│ │                          Dashboard                                 │        │
│ │        (org management, members, billing, schema, storage)         │        │
│ └───────────────────────────────────────────────────────────────────┘        │
│                                                                              │
│ ┌───────────────────────────────────────────────────────────────────┐        │
│ │                             CLI                                    │        │
│ │             (schema push/pull, type generation, dev)               │        │
│ └───────────────────────────────────────────────────────────────────┘        │
│                                                                              │
└──────────────────────────────────────────────────────────────────────────────┘
```

### Infrastructure Databases

| Database | Purpose |
|----------|---------|
| **sessions.db** | Auth sessions (ephemeral, Redis-swappable) |
| **tenants.db** | Users, organizations, memberships, schema templates |
| **storage.db** | File metadata, per-org storage tracking |
| **usage.db** | API requests, row counts, storage bytes per org |

### Services

#### Auth Service

Handles users, organizations, and access control.

**Features:**
- Email/password authentication
- OAuth providers (Google, GitHub, etc.)
- Organization management (create, invite, roles)
- SSO/SAML (enterprise)
- Session management

**Key endpoints:**
```
POST   /auth/register
POST   /auth/login
POST   /auth/logout
GET    /auth/me
GET    /auth/me/organizations

POST   /auth/organizations
GET    /auth/organizations/:id
POST   /auth/organizations/:id/invite
DELETE /auth/organizations/:id/members/:userId
PATCH  /auth/organizations/:id/members/:userId/role
```

#### Data API

CRUD operations against tenant databases.

**Features:**
- JSON-based queries
- Automatic joins via foreign keys
- Full-text search (FTS5)
- Batch operations
- Schema validation

**Key endpoints:**
```
POST   /data/query/:table          # SELECT/INSERT/UPDATE/DELETE
POST   /data/batch                 # Multiple operations
GET    /data/schema                # Introspection
```

**Tenant scoping:**
```typescript
// SDK automatically scopes to user's current org
const client = atomicbase.forOrg('acme-corp')
const { data } = await client.from('projects').select()
```

#### Storage Service

File storage with tenant isolation.

**Features:**
- Per-org file isolation
- Image transformations (via imgproxy)
- CDN delivery
- Usage tracking

**Key endpoints:**
```
POST   /storage/upload
GET    /storage/:orgId/:path
DELETE /storage/:orgId/:path
GET    /storage/:orgId/list
```

#### Billing Service

Usage metering and Stripe integration.

**Features:**
- Automatic usage tracking
- Seat counting
- Stripe meter events
- Invoice generation

**Key endpoints:**
```
GET    /billing/usage/:orgId
POST   /billing/report-usage
GET    /billing/invoices/:orgId
POST   /billing/webhook          # Stripe webhooks
```

#### Platform Service

Schema templates and tenant provisioning.

**Features:**
- Schema template storage
- Version history
- Mass migration orchestration
- Tenant database provisioning

**Key endpoints:**
```
POST   /platform/templates
GET    /platform/templates/:name
POST   /platform/templates/:name/push
GET    /platform/jobs/:id
```

---

## SDK Design

### Client Creation

```typescript
import { Atomicbase } from '@atomicbase/sdk'

const atomicbase = new Atomicbase({
  url: 'https://api.atomicbase.dev',
  // Or self-hosted: 'https://your-instance.com'
})
```

### Authentication

```typescript
// Register
const { user } = await atomicbase.auth.register({
  email: 'user@example.com',
  password: 'securepassword',
})

// Login
const { user } = await atomicbase.auth.login({
  email: 'user@example.com',
  password: 'securepassword',
})

// Get current user with their orgs
const { user } = await atomicbase.auth.getUser()
const orgs = await atomicbase.auth.getOrganizations()
```

### Organizations

```typescript
// Create org (provisions database automatically)
const org = await atomicbase.organizations.create({
  name: 'Acme Corporation',
  plan: 'pro',
})

// Invite member
await atomicbase.organizations.invite({
  orgId: org.id,
  email: 'colleague@example.com',
  role: 'admin',
})

// List members
const members = await atomicbase.organizations.listMembers(org.id)

// Update member role
await atomicbase.organizations.updateMemberRole({
  orgId: org.id,
  userId: 'user-123',
  role: 'viewer',
})

// Remove member
await atomicbase.organizations.removeMember({
  orgId: org.id,
  userId: 'user-123',
})
```

### Data Operations

```typescript
import { eq, and, or } from '@atomicbase/sdk/filters'

// Get client scoped to an organization
const db = atomicbase.org('acme-corp')

// Select
const { data: projects } = await db
  .from('projects')
  .select(['id', 'name', { tasks: ['id', 'title'] }])
  .where(eq('status', 'active'))

// Insert
const { data: project } = await db
  .from('projects')
  .insert({ name: 'New Project', created_by: user.id })
  .returning(['*'])

// Update
await db
  .from('projects')
  .update({ status: 'archived' })
  .where(eq('id', projectId))

// Delete
await db
  .from('tasks')
  .delete()
  .where(eq('project_id', projectId))
```

### Storage

```typescript
// Upload
const file = await atomicbase.storage.upload({
  orgId: 'acme-corp',
  path: 'documents/report.pdf',
  file: pdfBuffer,
  contentType: 'application/pdf',
})

// Get URL (with optional transforms for images)
const url = atomicbase.storage.getUrl({
  orgId: 'acme-corp',
  path: 'avatars/user.jpg',
  transform: { width: 100, height: 100, format: 'webp' },
})

// Delete
await atomicbase.storage.delete({
  orgId: 'acme-corp',
  path: 'documents/old-report.pdf',
})

// List files
const files = await atomicbase.storage.list({
  orgId: 'acme-corp',
  prefix: 'documents/',
})
```

### Billing

```typescript
// Get current usage
const usage = await atomicbase.billing.getUsage({
  orgId: 'acme-corp',
  period: 'current_month',
})
// { rows: 15420, storage_bytes: 52MB, requests: 89000, seats: 12 }

// Check limits
const limits = await atomicbase.billing.checkLimits({
  orgId: 'acme-corp',
})
// { rows: { used: 15420, limit: 100000, remaining: 84580 }, ... }

// Report to Stripe (or let Atomicbase do it automatically)
await atomicbase.billing.syncToStripe({
  orgId: 'acme-corp',
})
```

---

## CLI

```bash
# Initialize project
atomicbase init

# Auth
atomicbase login
atomicbase logout

# Schema management
atomicbase push                    # Push schema to all tenant DBs
atomicbase pull                    # Pull schema from server
atomicbase diff                    # Preview changes before push
atomicbase generate                # Generate TypeScript types

# Organization management
atomicbase orgs list
atomicbase orgs create <name>
atomicbase orgs delete <name>

# Schema sync jobs
atomicbase jobs list
atomicbase jobs show <id>
atomicbase jobs retry <id>

# Local development
atomicbase dev                     # Run local dev server
```

---

## Deployment

### Self-Hosted (Docker Compose)

```yaml
services:
  gateway:
    image: traefik:v2.10

  auth:
    image: atomicbase/auth

  data-api:
    image: atomicbase/data-api

  storage:
    image: atomicbase/storage

  imgproxy:
    image: darthsim/imgproxy

  billing:
    image: atomicbase/billing

  platform:
    image: atomicbase/platform

  dashboard:
    image: atomicbase/dashboard

  redis:
    image: redis:alpine

volumes:
  data:  # SQLite infrastructure DBs
```

### Hosted Platform

Atomicbase Cloud - fully managed, pay-per-usage.

```
┌─────────────────────────────────────────────────────────────┐
│                    Atomicbase Cloud                          │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│   Cloudflare                                                │
│   ├── CDN (dashboard, storage)                              │
│   └── R2 (file storage)                                     │
│                                                             │
│   Fly.io (multi-region)                                     │
│   ├── Auth Service                                          │
│   ├── Data API                                              │
│   ├── Storage Service                                       │
│   ├── Billing Service                                       │
│   └── Platform Service                                      │
│                                                             │
│   Turso                                                     │
│   └── Tenant databases (edge-replicated)                    │
│                                                             │
│   Stripe                                                    │
│   └── Billing & payments                                    │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

---

## Build vs Adopt

| Component | Decision | Notes |
|-----------|----------|-------|
| Auth | **Build** | Org-aware auth is core to the product |
| Data API | **Build** | Core differentiator |
| Storage | **Build** | Tenant isolation is key |
| Image transforms | Adopt (imgproxy) | Solved problem |
| Billing | **Build** | Tight integration with usage |
| Platform | **Build** | Schema sync is core |
| Dashboard | **Build** | UX differentiator |
| CLI | **Build** | DX differentiator |
| File storage backend | Adopt (R2/S3) | Commodity |
| Tenant DBs | Adopt (Turso) | Core infrastructure |

---

## Target Customer

**Who Atomicbase is for:**
- B2B SaaS builders
- Teams building multi-tenant applications
- Developers who don't want to build org management, billing, tenant isolation from scratch

**Example use cases:**
- Project management tools (like Linear, Asana)
- CRM systems
- Helpdesk software
- Analytics dashboards
- Internal tools platforms

**Not for:**
- Single-tenant applications (just use Postgres)
- Apps that don't need isolated databases

---

## Implementation Phases (MVP-First)

Goal: Get to a usable MVP as fast as possible. Billing comes last.

### Phase 1: Schema Engine + CLI ⬅️ Current
- [x] Data API (CRUD, joins, batch, FTS)
- [ ] Schema templates (define in TypeScript, store versions)
- [ ] Sync engine (push schema to tenant DBs)
- [ ] CLI basics (push, pull, diff)

**MVP milestone:** Developers can define schemas and sync them across databases.

### Phase 2: Databases + Auth
- [ ] Database CRUD (create, delete, list)
- [ ] Access grants (grant/revoke user access to database)
- [ ] User registration/login (email + password)
- [ ] Sessions (cookie-based, as designed)
- [ ] SDK: `atomicbase.database(id).from()` query builder
- [ ] Convenience: `provisionDatabase` option on register

**MVP milestone:** Users can sign up, create databases, and grant access. Works for personal apps.

### Phase 3: Organizations (Optional Module)
- [ ] Organization CRUD
- [ ] Membership (invite, roles, remove)
- [ ] Access grants for orgs (grant org access to database)
- [ ] Convenience: `provisionDatabase` option on org create

**MVP milestone:** Developers can enable orgs for B2B apps. Not required.

### Phase 4: Storage
- [ ] Upload/download API
- [ ] Per-database isolation (storage tied to database, not user/org)
- [ ] Basic image transforms (resize via imgproxy)

**MVP milestone:** Apps can store files scoped to databases.

---

### Post-MVP

### Phase 5: Dashboard
- [ ] User & organization management UI
- [ ] Member management
- [ ] Schema designer / SQL editor

### Phase 6: Billing
- [ ] Usage metering (rows, storage, requests, seats)
- [ ] Stripe integration
- [ ] Plan limits enforcement

### Phase 7: Enterprise
- [ ] SSO/SAML per organization
- [ ] Audit logs
- [ ] Custom domains
