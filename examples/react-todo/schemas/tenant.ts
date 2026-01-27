import { defineSchema, defineTable, c } from "@atomicbase/schema";

export default defineSchema("tenant", {
  todos: defineTable({
    id: c.integer().primaryKey(),
    title: c.text().notNull(),
    completed: c.integer().notNull().default(0),
    created_at: c.text().notNull().default("CURRENT_TIMESTAMP"),
    updated_at: c.text().notNull().default("CURRENT_TIMESTAMP"),
  }),
});
