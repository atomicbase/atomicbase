# Atomicbase SDK Design

## Overview

TypeScript SDK for the Atomicbase REST API. Designed for 100% compatibility with the underlying API.

## API Design

All query operations use `POST /query/{table}` with a required `Prefer` header specifying the operation. Requests without `Prefer: operation=` will fail to prevent accidental operations.

| Operation     | Prefer Header                          | Body                   |
| ------------- | -------------------------------------- | ---------------------- |
| SELECT        | `operation=select`                     | `{select, where, ...}` |
| INSERT        | `operation=insert`                     | `{data: {...}}`        |
| INSERT IGNORE | `operation=insert,on-conflict=ignore`  | `{data: {...}}`        |
| UPSERT        | `operation=insert,on-conflict=replace` | `{data: [...]}`        |
| UPDATE        | `operation=update`                     | `{data, where}`        |
| DELETE        | `operation=delete`                     | `{where}`              |

---

## Query SDK

The query SDK lives in the base namespace. All queries start with `from(tableName)`.

### Entry Point

```typescript
const query = client.from("users");
```

### Query Methods

#### select(...columns)

Variadic arguments for columns. Empty select defaults to `*`. Supports nested relations via objects for implicit FK joins.

```typescript
// Select all
await client.from("users").select();
await client.from("users").select("*");

// Select specific columns
await client.from("users").select("id", "name", "email");

// Select with nested relations (implicit LEFT JOIN via FK)
await client.from("users").select("id", "name", { posts: ["title", "body"] });

// Deeply nested
await client
  .from("users")
  .select("id", { posts: ["title", { comments: ["body", "author_id"] }] });
```

**API mapping:**

- Method: `POST /query/{table}`
- Header: `Prefer: operation=select`
- Body: `{ select: [...], where: [...], order: {...}, limit: n, offset: n }`

#### insert(data)

Insert a single object or array of objects. Fails on duplicate primary keys.

```typescript
// Single insert
await client
  .from("users")
  .insert({ name: "Alice", email: "alice@example.com" });

// Bulk insert
await client.from("users").insert([
  { name: "Alice", email: "alice@example.com" },
  { name: "Bob", email: "bob@example.com" },
]);

// With returning
await client
  .from("users")
  .insert({ name: "Alice" })
  .returning("id", "created_at");

// Insert ignore (skip on conflict)
await client
  .from("users")
  .insert({ id: 1, name: "Alice" })
  .onConflict("ignore");
```

**API mapping:**

- Method: `POST /query/{table}`
- Header: `Prefer: operation=insert` (or `operation=insert,on-conflict=ignore` for insert ignore)
- Body: `{ data: {...} | [...], returning: [...] }`
- Returns: `{ last_insert_id: number }` or rows if returning

#### upsert(data)

Insert or replace on conflict. Takes single object or array.

```typescript
// Single upsert
await client.from("users").upsert({ id: 1, name: "Alice" });

// Bulk upsert
await client.from("users").upsert([
  { id: 1, name: "Alice" },
  { id: 2, name: "Bob" },
]);

// With returning
await client.from("users").upsert({ id: 1, name: "Alice" }).returning("*");
```

**API mapping:**

- Method: `POST /query/{table}`
- Header: `Prefer: operation=insert,on-conflict=replace`
- Body: `{ data: {...} | [...], returning: [...] }`
- Returns: `{ rows_affected: number }` or rows if returning

#### update(data)

Update rows matching filter. Takes object with column:value pairs.

```typescript
await client
  .from("users")
  .update({ status: "inactive", updated_at: new Date().toISOString() })
  .where(eq("id", 5));

// With returning
await client
  .from("users")
  .update({ status: "inactive" })
  .where(eq("id", 5))
  .returning("id", "status");
```

**API mapping:**

- Method: `POST /query/{table}`
- Header: `Prefer: operation=update`
- Body: `{ data: {...}, where: [...], returning: [...] }`
- Returns: `{ rows_affected: number }` or rows if returning

#### delete()

Delete rows matching filter. No arguments. **Fails without a where clause** to prevent accidental mass deletion.

```typescript
await client.from("users").delete().where(eq("id", 5));

// With returning
await client
  .from("users")
  .delete()
  .where(eq("status", "deleted"))
  .returning("id");

// This will error - no where clause
await client.from("users").delete(); // Error: MISSING_WHERE_CLAUSE
```

**API mapping:**

- Method: `POST /query/{table}`
- Header: `Prefer: operation=delete`
- Body: `{ where: [...], returning: [...] }`
- Returns: `{ rows_affected: number }` or rows if returning

---

### Filtering with where()

The `where()` method takes filter and conditional operator functions, similar to Drizzle.

```typescript
import { eq, gt, or, not, inArray } from "atomicbase";

// Simple equality
await client.from("users").select().where(eq("status", "active"));

// Multiple conditions (ANDed)
await client
  .from("users")
  .select()
  .where(eq("status", "active"), gt("age", 21));

// OR conditions
await client
  .from("users")
  .select()
  .where(or(eq("role", "admin"), eq("role", "moderator")));

// Complex conditions
await client
  .from("users")
  .select()
  .where(eq("org_id", 5), or(eq("status", "active"), gt("score", 100)));

// NOT
await client
  .from("users")
  .select()
  .where(not(eq("status", "deleted")));
```

#### Filter Functions

| Function                 | SQL Equivalent            | Example                             |
| ------------------------ | ------------------------- | ----------------------------------- |
| `eq(col, val)`           | `col = val`               | `eq("status", "active")`            |
| `neq(col, val)`          | `col != val`              | `neq("status", "deleted")`          |
| `gt(col, val)`           | `col > val`               | `gt("age", 21)`                     |
| `gte(col, val)`          | `col >= val`              | `gte("score", 100)`                 |
| `lt(col, val)`           | `col < val`               | `lt("price", 50)`                   |
| `lte(col, val)`          | `col <= val`              | `lte("quantity", 10)`               |
| `like(col, pattern)`     | `col LIKE pattern`        | `like("name", "%smith%")`           |
| `glob(col, pattern)`     | `col GLOB pattern`        | `glob("path", "*/src/*")`           |
| `inArray(col, vals)`     | `col IN (...)`            | `inArray("status", ["a", "b"])`     |
| `between(col, min, max)` | `col BETWEEN min AND max` | `between("age", 18, 65)`            |
| `isNull(col)`            | `col IS NULL`             | `isNull("deleted_at")`              |
| `isNotNull(col)`         | `col IS NOT NULL`         | `isNotNull("email")`                |
| `fts(col, query)`        | Full-text search          | `fts("content", "sqlite database")` |

#### Conditional Operators

| Function             | Description               | Example                        |
| -------------------- | ------------------------- | ------------------------------ |
| `not(condition)`     | Negates a condition       | `not(eq("status", "active"))`  |
| `or(...conditions)`  | OR combinator             | `or(eq("a", 1), eq("b", 2))`   |
| `and(...conditions)` | AND combinator (explicit) | `and(gt("x", 0), lt("x", 10))` |

**JSON mapping:**

```typescript
// eq("status", "active") ->
{ "status": { "eq": "active" } }

// or(eq("role", "admin"), gt("age", 21)) ->
{ "or": [{ "role": { "eq": "admin" } }, { "age": { "gt": 21 } }] }

// not(eq("status", "deleted")) ->
{ "status": { "not": { "eq": "deleted" } } }
```

---

### Query Modifiers

#### orderBy(column, direction)

```typescript
await client.from("users").select().orderBy("created_at", "desc");
await client.from("users").select().orderBy("name", "asc");
```

**JSON mapping:** `{ order: { "created_at": "desc" } }`

#### limit(n) / offset(n)

```typescript
await client.from("users").select().limit(20).offset(40);
```

---

### Returning Clause

The `returning()` method specifies columns to return after insert/update/upsert/delete operations.

```typescript
// Return specific columns
await client
  .from("users")
  .insert({ name: "Alice" })
  .returning("id", "created_at");

// Return all columns
await client.from("users").insert({ name: "Alice" }).returning("*");

// Chained with update
await client
  .from("users")
  .update({ status: "inactive" })
  .where(eq("id", 5))
  .returning("id", "status", "updated_at");
```

**JSON mapping:** `{ ..., returning: ["id", "created_at"] }`

---

#### single()

Returns exactly one row. Errors if zero or multiple rows returned.

```typescript
const { data, error } = await client
  .from("users")
  .select()
  .where(eq("id", 1))
  .single();
// data: User | null (single object, not array)
// error: NOT_FOUND if 0 rows, MULTIPLE_ROWS if >1
```

#### maybeSingle()

Returns one row or null. No error if zero rows.

```typescript
const { data, error } = await client
  .from("users")
  .select()
  .where(eq("email", "alice@example.com"))
  .maybeSingle();
// data: User | null
```

---

### Counting

Use `count()` method or `withCount()` modifier.

#### count()

Returns only the count, no data.

```typescript
const { data: count, error } = await client
  .from("users")
  .select()
  .where(eq("status", "active"))
  .count();
// count: number
```

#### withCount()

Returns data and total count (for pagination).

```typescript
const { data, count, error } = await client
  .from("users")
  .select()
  .where(eq("status", "active"))
  .limit(20)
  .withCount();
// data: User[]
// count: number (total matching rows, ignoring limit/offset)
```

**API mapping:** `Prefer: operation=select,count=exact` header, count returned in `X-Total-Count` header.

---

### Nested Relations (Implicit Joins)

Select supports nested objects for implicit LEFT JOINs via foreign keys.

```typescript
// users has posts (posts.user_id -> users.id)
const { data } = await client
  .from("users")
  .select("id", "name", { posts: ["title", "body"] });

// Result:
// [
//   { id: 1, name: "Alice", posts: [{ title: "Hello", body: "..." }, ...] },
//   { id: 2, name: "Bob", posts: [] }
// ]
```

Deeply nested relations:

```typescript
const { data } = await client
  .from("users")
  .select("id", { posts: ["title", { comments: ["body", "author_id"] }] });

// Result:
// [
//   {
//     id: 1,
//     posts: [
//       { title: "Hello", comments: [{ body: "Great!", author_id: 2 }] }
//     ]
//   }
// ]
```

**Note:** The API automatically detects foreign key relationships. If no FK exists between tables, the query will error with `NO_RELATIONSHIP`.

---

### Execution

queries are automatically executed when they are ran with await. A query without await will not execute and can be safely stored in a variable until it is ready to execute.

```typescript
const { data, error } = await client.from("users").select("id", "name");
// data: User[] | null
// error: AtomicbaseError | null
```

---

## Type Definitions

```typescript
// Filter condition type
type FilterCondition =
  | {
      [column: string]: {
        eq?: unknown;
        neq?: unknown;
        gt?: unknown;
        gte?: unknown;
        lt?: unknown;
        lte?: unknown;
        like?: string;
        glob?: string;
        in?: unknown[];
        between?: [unknown, unknown];
        is?: null | boolean;
        fts?: string;
        not?: Omit<FilterCondition[string], "not">;
      };
    }
  | { or: FilterCondition[] }
  | { and: FilterCondition[] };

// Select column - string or nested relation object
type SelectColumn = string | { [relation: string]: SelectColumn[] };

// Result wrapper
type Result<T> = {
  data: T | null;
  error: AtomicbaseError | null;
};

// Result with count
type ResultWithCount<T> = {
  data: T | null;
  count: number | null;
  error: AtomicbaseError | null;
};
```

---

## Method Chaining Summary

| Starting Method | Can Chain                                                                            |
| --------------- | ------------------------------------------------------------------------------------ |
| `select()`      | `where`, `orderBy`, `limit`, `offset`, `single`, `maybeSingle`, `count`, `withCount` |
| `insert()`      | `returning`, `onConflict`                                                            |
| `upsert()`      | `returning`                                                                          |
| `update()`      | `where`, `returning`                                                                 |
| `delete()`      | `where`, `returning`                                                                 |

---

## Complete Examples

```typescript
import { createClient, eq, gt, or, inArray, fts } from "atomicbase";

const client = createClient({ url: "http://localhost:8080", apiKey: "..." });

// Basic CRUD
const users = await client.from("users").select("id", "name");
const user = await client.from("users").select().where(eq("id", 1)).single();
await client.from("users").insert({ name: "Alice" });
await client.from("users").update({ name: "Alicia" }).where(eq("id", 1));
await client.from("users").delete().where(eq("id", 1));

// Complex query
const activeAdmins = await client
  .from("users")
  .select("id", "name", "email")
  .where(
    eq("status", "active"),
    or(eq("role", "admin"), eq("role", "superadmin"))
  )
  .orderBy("created_at", "desc")
  .limit(10);

// With relations
const usersWithPosts = await client
  .from("users")
  .select("id", "name", { posts: ["title", "created_at"] })
  .where(gt("posts_count", 0));

// Pagination with count
const { data, count } = await client
  .from("users")
  .select()
  .limit(20)
  .offset(40)
  .withCount();

// Full-text search
const results = await client
  .from("articles")
  .select("id", "title")
  .where(fts("content", "typescript tutorial"));

// Insert with returning
const { data: newUser } = await client
  .from("users")
  .insert({ name: "Bob", email: "bob@example.com" })
  .returning("id", "created_at");

// Upsert
await client
  .from("settings")
  .upsert({ user_id: 1, theme: "dark", notifications: true });
```
