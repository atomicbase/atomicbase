import { c, defineSchema, defineTable } from "@atomicbase/schema";

export default defineSchema("sdk", {
    sdk_test: defineTable({
        id: c.integer().primaryKey(),
        name: c.text(),
        email: c.text().unique(),
        age: c.integer(),
        status: c.text(),
    }),
    users: defineTable({
        id: c.integer().primaryKey(),
        name: c.text(),
        email: c.text().unique(),
        age: c.integer()
    }),
    posts: defineTable({
        id: c.integer().primaryKey(),
        title: c.text(),
        content: c.text(),
        user_id: c.integer().references('users.id')
    })
})