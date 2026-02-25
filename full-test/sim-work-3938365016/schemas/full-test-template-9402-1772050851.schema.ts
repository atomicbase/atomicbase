import { defineSchema, defineTable, c, sql } from "/Users/joeervin/Desktop/atomicbase/packages/template/dist/index.js";

export default defineSchema("full-test-template-9402-1772050851", {
  users: defineTable({
    id: c.integer().primaryKey(),
    email: c.text().notNull().unique().collate("NOCASE"),
    display_name: c.text().notNull().check("length(display_name) >= 2"),
    role: c.text().notNull().default("member").check("role in ('owner','admin','member')"),
    profile_json: c.text().notNull().default("{}"),
    created_at: c.text().notNull().default(sql("CURRENT_TIMESTAMP")),
    updated_at: c.text().notNull().default(sql("CURRENT_TIMESTAMP")),
  }).index("idx_users_role", ["role"]),

  workspaces: defineTable({
    id: c.integer().primaryKey(),
    slug: c.text().notNull().unique().collate("NOCASE"),
    name: c.text().notNull(),
    owner_id: c.integer().notNull().references("users.id", { onDelete: "CASCADE", onUpdate: "CASCADE" }),
    created_at: c.text().notNull().default(sql("CURRENT_TIMESTAMP")),
  }).index("idx_workspaces_owner", ["owner_id"]),

  projects: defineTable({
    id: c.integer().primaryKey(),
    workspace_id: c.integer().notNull().references("workspaces.id", { onDelete: "CASCADE", onUpdate: "CASCADE" }),
    owner_id: c.integer().notNull().references("users.id", { onDelete: "RESTRICT", onUpdate: "CASCADE" }),
    name: c.text().notNull().check("length(name) >= 4"),
    description: c.text(),
    status: c.text().notNull().default("active").check("status in ('active','archived')"),
    priority: c.integer().notNull().default(3).check("priority between 1 and 7"),
    budget: c.real().check("budget >= 0"),
    slug: c.text().generatedAs("lower(replace(name, ' ', '-'))", { stored: true }),
    created_at: c.text().notNull().default(sql("CURRENT_TIMESTAMP")),
  })
    .index("idx_projects_workspace", ["workspace_id"])
    .index("idx_projects_status", ["status"])
    .uniqueIndex("idx_projects_workspace_slug", ["workspace_id", "slug"]),

  tags: defineTable({
    id: c.integer().primaryKey(),
    workspace_id: c.integer().notNull().references("workspaces.id", { onDelete: "CASCADE", onUpdate: "CASCADE" }),
    name: c.text().notNull().collate("NOCASE"),
    color: c.text().default("#cccccc").check("length(color) = 7"),
  }).uniqueIndex("idx_tags_workspace_name", ["workspace_id", "name"]),

  project_tags: defineTable({
    project_id: c.integer().primaryKey().references("projects.id", { onDelete: "CASCADE", onUpdate: "CASCADE" }),
    tag_id: c.integer().primaryKey().references("tags.id", { onDelete: "CASCADE", onUpdate: "CASCADE" }),
    added_at: c.text().notNull().default(sql("CURRENT_TIMESTAMP")),
  }),

  todos: defineTable({
    id: c.integer().primaryKey(),
    project_id: c.integer().notNull().references("projects.id", { onDelete: "CASCADE", onUpdate: "CASCADE" }),
    assignee_id: c.integer().references("users.id", { onDelete: "SET NULL", onUpdate: "CASCADE" }),
    title: c.text().notNull().check("length(title) >= 7"),
    description: c.text(),
    status: c.text().notNull().default("todo").check("status in ('todo','in_progress','done','blocked')"),
    priority: c.integer().notNull().default(3).check("priority between 1 and 7"),
    completed: c.integer().notNull().default(0).check("completed in (0,1)"),
    due_at: c.text(),
    estimate_hours: c.real().check("estimate_hours >= 0"),
    metadata_json: c.text().notNull().default("{}"),
    search_text: c.text().generatedAs("coalesce(title,'') || ' ' || coalesce(description,'')", { stored: true }),
    archived_at: c.text(),
    sprint_order: c.integer().notNull().default(0).check("sprint_order >= 0"),
    created_at: c.text().notNull().default(sql("CURRENT_TIMESTAMP")),
    updated_at: c.text().notNull().default(sql("CURRENT_TIMESTAMP")),
  })
    .index("idx_todos_project", ["project_id"])
    .index("idx_todos_assignee", ["assignee_id"])
    .index("idx_todos_status_priority", ["status", "priority"])
    .fts(["title", "description", "metadata_json"]),

  comments: defineTable({
    id: c.integer().primaryKey(),
    todo_id: c.integer().notNull().references("todos.id", { onDelete: "CASCADE", onUpdate: "CASCADE" }),
    author_id: c.integer().notNull().references("users.id", { onDelete: "RESTRICT", onUpdate: "CASCADE" }),
    body: c.text().notNull().check("length(body) > 0"),
    metadata_json: c.text().notNull().default("{}"),
    created_at: c.text().notNull().default(sql("CURRENT_TIMESTAMP")),
  })
    .index("idx_comments_todo", ["todo_id"])
    .fts(["body"]),

  attachments: defineTable({
    id: c.integer().primaryKey(),
    todo_id: c.integer().notNull().references("todos.id", { onDelete: "CASCADE", onUpdate: "CASCADE" }),
    filename: c.text().notNull(),
    content_type: c.text().notNull(),
    size_bytes: c.integer().notNull().check("size_bytes >= 0"),
    checksum: c.text().notNull(),
    content: c.blob(),
    created_at: c.text().notNull().default(sql("CURRENT_TIMESTAMP")),
  }).index("idx_attachments_todo", ["todo_id"]),


  project_members: defineTable({
    project_id: c.integer().primaryKey().references("projects.id", { onDelete: "CASCADE", onUpdate: "CASCADE" }),
    user_id: c.integer().primaryKey().references("users.id", { onDelete: "CASCADE", onUpdate: "CASCADE" }),
    permission: c.text().notNull().default("editor").check("permission in ('viewer','editor','admin')"),
    joined_at: c.text().notNull().default(sql("CURRENT_TIMESTAMP")),
  }).index("idx_project_members_user", ["user_id"]),


  todo_reactions: defineTable({
    todo_id: c.integer().primaryKey().references("todos.id", { onDelete: "CASCADE", onUpdate: "CASCADE" }),
    user_id: c.integer().primaryKey().references("users.id", { onDelete: "CASCADE", onUpdate: "CASCADE" }),
    emoji: c.text().primaryKey(),
    created_at: c.text().notNull().default(sql("CURRENT_TIMESTAMP")),
  }),

  audit_events: defineTable({
    todo_id: c.integer().primaryKey().references("todos.id", { onDelete: "CASCADE", onUpdate: "CASCADE" }),
    seq: c.integer().primaryKey(),
    actor_id: c.integer().references("users.id", { onDelete: "SET NULL", onUpdate: "CASCADE" }),
    action: c.text().notNull(),
    payload_json: c.text().notNull().default("{}"),
    created_at: c.text().notNull().default(sql("CURRENT_TIMESTAMP")),
  }).index("idx_audit_actor", ["actor_id"]),
});
