import { defineSchema, defineTable, c, sql } from "@atomicbase/definitions";

export default defineSchema("my-app", {
  users: defineTable({
    created_at: c.text().notNull().default(sql("CURRENT_TIMESTAMP")),
    email: c.text().notNull().unique(),
    id: c.integer().primaryKey(),
    name: c.text().notNull(),
  }),
});
