# Atomicbase

Manage a million databases like it's one.

Atomicbase is the multi-tenant development platform. Built on SQLite and Turso, packaged as a single Go executable.

## **Atomicbase is in experimental preview.** There are many known and unknown bugs. APIs are likely to change.

## Status

| Component          | Status         |
| ------------------ | -------------- |
| Data API           | Beta           |
| TypeScript SDK     | Beta           |
| Platform API       | Experimental   |
| Schema Templates   | Experimental   |
| Schema Package     | Beta           |
| CLI                | Experimental   |
| Authentication     | Planned        |
| File Storage       | Planned        |
| Realtime           | Planned        |
| Dashboard          | Planned        |

## Architecture

```
┌────────────────────────────────────────────────────────────────────────────┐
│                              Atomicbase                                    │
│                    Multi-tenancy platform for B2B SaaS                     │
├────────────────────────────────────────────────────────────────────────────┤
│                                                                            │
│   ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐       │
│   │    Auth     │  │    Data     │  │  Platform   │  │   Storage   │       │
│   │             │  │             │  │             │  │             │       │
│   │ • Users     │  │ • Queries   │  │ • Tenants   │  │ • Uploads   │       │
│   │ • Orgs      │  │ • Validation│  │ • Templates │  │ • Transforms│       │
│   │ • Roles     │  │ • Tenant    │  │ • Migrations│  │ • CDN       │       │
│   │ • SSO       │  │   routing   │  │ • Syncing   │  │ • Caching   │       │
│   └─────────────┘  └─────────────┘  └─────────────┘  └─────────────┘       │
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

## Quick Start

### 1. Start the Server

```bash
cd api
make build && make run
```

Server runs at `http://localhost:8080`.

### 2. Install Packages

```bash
npm install @atomicbase/sdk @atomicbase/schema
```

### 3. Define & Push Schema

```typescript
// schemas/my-app.schema.ts
import { defineSchema, defineTable, c } from "@atomicbase/schema";

export default defineSchema("my-app", {
  users: defineTable({
    id: c.integer().primaryKey(),
    name: c.text().notNull(),
    email: c.text().notNull().unique(),
  }),
});
```

```bash
npx atomicbase push
```

### 4. Create a Tenant Database

```typescript
import { createClient } from "@atomicbase/sdk";

const client = createClient({
  url: "http://localhost:8080",
  apiKey: "your-api-key",
});

await client.tenants.create({ name: "acme-corp", template: "my-app" });
```

### 5. Query Data

```typescript
import { eq } from "@atomicbase/sdk";

const acme = client.tenant("acme-corp");

// Insert
await acme.from("users").insert({ name: "Alice", email: "alice@example.com" });

// Select
const { data } = await acme.from("users").select();

// Update
await acme.from("users").update({ name: "Alicia" }).where(eq("id", 1));

// Delete
await acme.from("users").delete().where(eq("id", 1));
```

## Packages

| Package | Description | Docs |
| ------- | ----------- | ---- |
| [api](./api) | Go backend server | [README](./api/README.md) |
| [@atomicbase/sdk](./packages/sdk) | TypeScript SDK | [README](./packages/sdk/README.md) |
| [@atomicbase/schema](./packages/schema) | Schema definition package | [README](./packages/schema/README.md) |
| [@atomicbase/cli](./packages/cli) | CLI tool | [README](./packages/cli/README.md) |

## Examples

- [react-todo](./examples/react-todo) - Next.js todo app with Google OAuth and database-per-user architecture

## Contributing

See [CONTRIBUTING.md](./CONTRIBUTING.md) for guidelines.

## License

Atomicbase is [fair-source](https://fair.io) licensed under [FSL-1.1-MIT](./LICENSE). You can use, modify, and self-host the software freely for your own applications. The only restriction is offering Atomicbase as a competing hosted service. The license converts to MIT after two years.

## Links

- [GitHub](https://github.com/joe-ervin05/atomicbase)
- [Issues](https://github.com/joe-ervin05/atomicbase/issues)
