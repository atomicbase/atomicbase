import { c, defineSchema, defineTable } from "@atomicbase/schema";

export default defineSchema("sdk", {
    sdk_test: defineTable({
        id: c.integer().primaryKey(),
        name: c.text().notNull(),
        email: c.text().unique().notNull(),
        age: c.integer().notNull(),
        status: c.text().notNull(),
    }),
    users: defineTable({
        id: c.integer().primaryKey(),
        name: c.text().notNull(),
        email: c.text().unique().notNull(),
        age: c.integer()
    }),
    posts: defineTable({
        id: c.integer().primaryKey(),
        title: c.text().notNull(),
        content: c.text().notNull(),
        user_id: c.integer().references('users.id')
    })
})