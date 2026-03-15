import { defineSchema, defineTable, c, sql } from "@atomicbase/definitions";

export default defineSchema("tenant", {
  todos: defineTable({
    completed: c.integer().notNull().default(0),
    created_at: c.text().notNull().default(sql("CURRENT_TIMESTAMP")),
    id: c.integer().primaryKey(),
    name: c.text().notNull()
  }),
});
