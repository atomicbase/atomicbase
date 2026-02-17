Grants authorization example:

```typescript
// policies/org.grants.ts
import { defineGrants, definePolicy, g, eq, and, inList } from "@atomicbase/access";
import schema from "../schemas/org.schema.ts";

export default defineGrants(schema, {
  invoices: definePolicy({
    // select/delete use row; insert uses next; update can use both
    select: g.where(({ auth, row }) => eq(row.org_id, auth.org.id)),

    insert: g.where(({ auth, next }) =>
      and(
        eq(auth.status, "authenticated"),
        eq(next.org_id, auth.org.id),
      )
    ),

    update: g.where(({ auth, row, next }) =>
      and(
        eq(auth.status, "authenticated"),
        eq(row.org_id, auth.org.id),
        eq(next.org_id, auth.org.id),
      )
    ),

    delete: g.where(({ auth, row }) =>
      and(
        inList(auth.role, ["owner", "admin"]),
        eq(row.org_id, auth.org.id),
      )
    ),
  }),

  metadata: definePolicy({
    // DB-level style: auth-only condition
    select: g.where(({ auth }) => inList(auth.role, ["owner", "admin"])),
    insert: g.where(({ auth }) => inList(auth.role, ["owner", "admin"])),
    update: g.where(({ auth }) => inList(auth.role, ["owner", "admin"])),
    delete: g.where(({ auth }) => inList(auth.role, ["owner", "admin"])),
  }),

  user_settings: definePolicy({
    select: g.where(({ auth, row }) =>
      and(
        eq(auth.status, "authenticated"),
        eq(row.user_id, auth.id),
      )
    ),

    update: g.where(({ auth, row, next }) =>
      and(
        eq(auth.status, "authenticated"),
        eq(row.user_id, auth.id),
        eq(next.user_id, auth.id),
      )
    ),
  }),
});
```
