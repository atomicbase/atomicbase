import { defineSchema, defineTable, c } from "@atomicbase/schema";

export default defineSchema("primary", {
  users: defineTable({
    id: c.integer().primaryKey(),
    google_id: c.text().notNull().unique(),
    email: c.text().notNull(),
    name: c.text().notNull(),
    picture: c.text(),
    tenant_name: c.text().notNull().unique(),
    created_at: c.text().notNull().default("CURRENT_TIMESTAMP"),
  }).index("idx_google_id", ["google_id"]),

  sessions: defineTable({
    id: c.text().primaryKey(),
    user_id: c.integer().notNull().references("users.id", { onDelete: "CASCADE" }),
    expires_at: c.integer().notNull(),
    created_at: c.integer().notNull(),
  }).index("idx_user_id", ["user_id"]),
});
