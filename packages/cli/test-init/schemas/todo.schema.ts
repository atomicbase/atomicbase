import { defineTable, defineSchema, c } from "@atomicbase/schema";

export default defineSchema("tenant", {
  users: defineTable({
    id: c.integer().primaryKey(),
    first_name: c.text().notNull(),
    last_name: c.text().notNull(),
    email: c.text().notNull().unique().collate("NOCASE"),
    created_at: c.integer().notNull().default("unixepoch()")
  })
  .index('tenant_email_idx', ['email']),
  projects: defineTable({
    id: c.integer().primaryKey(),
    name: c.text().notNull(),
  })
})