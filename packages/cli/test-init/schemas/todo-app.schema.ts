import { defineSchema, defineTable, c } from "@atomicbase/schema";

export default defineSchema("todo-app", {
  users: defineTable({
    id: c.integer().primaryKey(),
    email: c.text().notNull().unique(),
    name: c.text().notNull(),
    created_at: c.integer().default("CURRENT_TIMESTAMP"),
  }).fts(["email"]),

  todos: defineTable({
    id: c.integer().primaryKey(),
    user_id: c.integer().notNull().references("users.id", { onDelete: "SET NULL" }),
    title: c.text().notNull(),
    completed: c.integer().default(0),
    created_at: c.integer().default("CURRENT_TIMESTAMP"),
  }).index("idx_todos_user", ["user_id"]).fts(["title"]),
});
