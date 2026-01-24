import {
  createClient,
  eq,
  gt,
  or,
  inArray,
  isNull,
  isNotNull,
  AtomicbaseError,
  onEq,
} from "./src/index.js";

interface RowSdkTest {
  id: number;
  name: string;
  email: string;
  age: number;
  status: string;
}

interface RowUser {
  id: number;                                                   
  name: string;                                                               
  email: string;                                                             
  age: number;  
}

interface RowPost {
  id: number;
  title: string;
  content: string;
  user_id: number;
}

const client = createClient({
  url: "http://localhost:8080",
});

async function test(name: string, fn: () => Promise<void>) {
  try {
    await fn();
    console.log(`✓ ${name}`);
  } catch (err) {
    console.log(`✗ ${name}`);
    console.error("  ", err);
  }
}

async function run() {
  console.log("\n=== SDK Integration Tests ===\n");

  // Clean up any existing test data
  await client.from("sdk_test").delete().where(gt("id", 0));

  // Test INSERT
  await test("Insert single row", async () => {
    const { data, error } = await client
      .from("sdk_test")
      .insert({ id: 1, name: "Alice", email: "alice@example.com", age: 30 });

    if (error) throw new Error(error.message);
    if (!data || typeof data.last_insert_id !== "number") {
      throw new Error("Expected last_insert_id");
    }
  });

  await test("Insert second row", async () => {
    const { data, error } = await client
      .from("sdk_test")
      .insert({ id: 2, name: "Bob", email: "bob@example.com", age: 25 });

    if (error) throw new Error(error.message);
  });

  await test("Insert third row", async () => {
    const { data, error } = await client
      .from("sdk_test")
      .insert({ id: 3, name: "Charlie", email: "charlie@example.com", age: 35 });

    if (error) throw new Error(error.message);
  });

  // Test SELECT
  await test("Select all", async () => {
    const { data, error } = await client.from("sdk_test").select();

    if (error) throw new Error(error.message);
    if (!Array.isArray(data) || data.length !== 3) {
      throw new Error(`Expected 3 rows, got ${data?.length}`);
    }
  });

  await test("Select specific columns", async () => {
    const { data, error } = await client.from("sdk_test").select("id", "name");

    if (error) throw new Error(error.message);
    if (!data || !data[0] || !("id" in data[0]) || !("name" in data[0])) {
      throw new Error("Expected id and name columns");
    }
  });

  await test("Select with where eq", async () => {
    const { data, error } = await client
      .from("sdk_test")
      .select()
      .where(eq("name", "Alice"));

    if (error) throw new Error(error.message);
    if (!data || data.length !== 1 || data[0].name !== "Alice") {
      throw new Error("Expected Alice");
    }
  });

  await test("Select with where gt", async () => {
    const { data, error } = await client
      .from("sdk_test")
      .select()
      .where(gt("age", 28));

    if (error) throw new Error(error.message);
    if (!data || data.length !== 2) {
      throw new Error(`Expected 2 rows (age > 28), got ${data?.length}`);
    }
  });

  await test("Select with where or", async () => {
    const { data, error } = await client
      .from("sdk_test")
      .select()
      .where(or(eq("name", "Alice"), eq("name", "Bob")));

    if (error) throw new Error(error.message);
    if (!data || data.length !== 2) {
      throw new Error(`Expected 2 rows, got ${data?.length}`);
    }
  });

  await test("Select with where inArray", async () => {
    const { data, error } = await client
      .from("sdk_test")
      .select()
      .where(inArray("id", [1, 2]));

    if (error) throw new Error(error.message);
    if (!data || data.length !== 2) {
      throw new Error(`Expected 2 rows, got ${data?.length}`);
    }
  });

  await test("Select with orderBy", async () => {
    const { data, error } = await client
      .from("sdk_test")
      .select()
      .orderBy("age", "desc");

    if (error) throw new Error(error.message);
    if (!data || data[0].name !== "Charlie") {
      throw new Error("Expected Charlie first (oldest)");
    }
  });

  await test("Select with limit", async () => {
    const { data, error } = await client.from("sdk_test").select().limit(2);

    if (error) throw new Error(error.message);
    if (!data || data.length !== 2) {
      throw new Error(`Expected 2 rows, got ${data?.length}`);
    }
  });

  await test("Select with limit and offset", async () => {
    const { data, error } = await client
      .from("sdk_test")
      .select()
      .orderBy("id", "asc")
      .limit(1)
      .offset(1);

    if (error) throw new Error(error.message);
    if (!data || data.length !== 1 || data[0].id !== 2) {
      throw new Error(`Expected Bob (id=2), got ${JSON.stringify(data)}`);
    }
  });

  await test("Select single()", async () => {
    const { data, error } = await client
      .from("sdk_test")
      .select()
      .where(eq("id", 1))
      .single();

    const tData = data as RowSdkTest | null;

    if (error) throw new Error(error.message);
    if (!tData || tData.id !== 1) {
      throw new Error("Expected single row with id=1");
    }
  });

  await test("Select single() returns error for no rows", async () => {
    const { data, error } = await client
      .from("sdk_test")
      .select()
      .where(eq("id", 999))
      .single();

    if (!error || error.code !== "NOT_FOUND") {
      throw new Error("Expected NOT_FOUND error");
    }
  });

  await test("Select maybeSingle()", async () => {
    const { data, error } = await client
      .from("sdk_test")
      .select()
      .where(eq("id", 999))
      .maybeSingle();

    if (error) throw new Error(error.message);
    if (data !== null) {
      throw new Error("Expected null for no rows");
    }
  });

  await test("Select count()", async () => {
    const { data, error } = await client.from("sdk_test").select().count();

    if (error) throw new Error(error.message);
    if (data !== 3) {
      throw new Error(`Expected count=3, got ${data}`);
    }
  });

  await test("Select withCount()", async () => {
    const { data, count, error } = await client
      .from("sdk_test")
      .select()
      .limit(2)
      .withCount();

    if (error) throw new Error(error.message);
    if (data?.length !== 2) {
      throw new Error(`Expected 2 data rows, got ${data?.length}`);
    }
    if (count !== 3) {
      throw new Error(`Expected count=3, got ${count}`);
    }
  });

  // NEW: Test throwOnError()
  await test("throwOnError() throws on error", async () => {
    try {
      await client
        .from("nonexistent_table")
        .select()
        .throwOnError();
      throw new Error("Expected error to be thrown");
    } catch (err) {
      if (!(err instanceof AtomicbaseError)) {
        throw new Error(`Expected AtomicbaseError, got ${err}`);
      }
      // Success - error was thrown
    }
  });

  await test("throwOnError() returns data on success", async () => {
    const { data, error } = await client
      .from("sdk_test")
      .select()
      .limit(1)
      .throwOnError();

    // No error thrown, data should be returned
    if (!data || data.length !== 1) {
      throw new Error(`Expected 1 row, got ${data?.length}`);
    }
  });

  // NEW: Test abortSignal()
  await test("abortSignal() cancels request", async () => {
    const controller = new AbortController();

    // Abort immediately
    controller.abort();

    const { data, error } = await client
      .from("sdk_test")
      .select()
      .abortSignal(controller.signal);

    if (!error || error.code !== "ABORTED") {
      throw new Error(`Expected ABORTED error, got ${error?.code}`);
    }
  });

  // Test UPDATE
  await test("Update with where", async () => {
    const { data, error } = await client
      .from("sdk_test")
      .update({ status: "inactive" })
      .where(eq("id", 1));

    if (error) throw new Error(error.message);
    if (!data || data.rows_affected !== 1) {
      throw new Error(`Expected 1 row affected, got ${data?.rows_affected}`);
    }
  });

  await test("Verify update worked", async () => {
    const { data, error } = await client
      .from("sdk_test")
      .select("status")
      .where(eq("id", 1))
      .single();

    const tData = data as RowSdkTest | null;

    if (error) throw new Error(error.message);
    
    if (! tData || tData?.status !== "inactive") {
      throw new Error(`Expected status=inactive, got ${data?.status}`);
    }
  });

  // Test UPSERT
  await test("Upsert existing row", async () => {
    const { data, error } = await client
      .from("sdk_test")
      .upsert({ id: 1, name: "Alice Updated", email: "alice@example.com", age: 31 });

    if (error) throw new Error(error.message);
  });

  await test("Verify upsert worked", async () => {
    const { data, error } = await client
      .from("sdk_test")
      .select("name", "age")
      .where(eq("id", 1))
      .single();

    if (error) throw new Error(error.message);
    if (data?.name !== "Alice Updated" || data?.age !== 31) {
      throw new Error(`Expected Alice Updated/31, got ${data?.name}/${data?.age}`);
    }
  });

  await test("Upsert new row", async () => {
    const { data, error } = await client
      .from("sdk_test")
      .upsert({ id: 4, name: "Diana", email: "diana@example.com", age: 28 });

    if (error) throw new Error(error.message);
  });

  await test("Verify new row inserted", async () => {
    const { data, error } = await client.from("sdk_test").select().count();

    if (error) throw new Error(error.message);
    if (data !== 4) {
      throw new Error(`Expected 4 rows, got ${data}`);
    }
  });

  // Test INSERT with returning
  await test("Insert with returning", async () => {
    const { data, error } = await client
      .from("sdk_test")
      .insert({ id: 5, name: "Eve", email: "eve@example.com", age: 22 })
      .returning("id", "name");

    if (error) throw new Error(error.message);
    // returned data should be the rows
    if (!Array.isArray(data) || data.length !== 1 || data[0].name !== "Eve") {
      throw new Error(`Expected returned row with Eve, got ${JSON.stringify(data)}`);
    }
  });

  // Test DELETE
  await test("Delete with where", async () => {
    const { data, error } = await client
      .from("sdk_test")
      .delete()
      .where(eq("id", 5));

    if (error) throw new Error(error.message);
    if (!data || data.rows_affected !== 1) {
      throw new Error(`Expected 1 row affected, got ${data?.rows_affected}`);
    }
  });

  await test("Verify delete worked", async () => {
    const { data, error } = await client.from("sdk_test").select().count();

    if (error) throw new Error(error.message);
    if (data !== 4) {
      throw new Error(`Expected 4 rows, got ${data}`);
    }
  });

  // =========================================================================
  // BATCH TESTS
  // =========================================================================
  console.log("\n=== Batch API Tests ===\n");

  // Clean up and set up fresh data for batch tests
  await client.from("sdk_test").delete().where(gt("id", 0));
  await client.from("sdk_test").insert({ id: 1, name: "Alice", email: "alice@example.com", age: 30 });
  await client.from("sdk_test").insert({ id: 2, name: "Bob", email: "bob@example.com", age: 25 });
  await client.from("sdk_test").insert({ id: 3, name: "Charlie", email: "charlie@example.com", age: 35 });

  // Test basic batch with multiple inserts
  await test("Batch: multiple inserts", async () => {
    const { data, error } = await client.batch([
      client.from("sdk_test").insert({ id: 10, name: "User10", email: "user10@test.com", age: 20 }),
      client.from("sdk_test").insert({ id: 11, name: "User11", email: "user11@test.com", age: 21 }),
      client.from("sdk_test").insert({ id: 12, name: "User12", email: "user12@test.com", age: 22 }),
    ]);

    if (error) throw new Error(error.message);
    if (!data || data.results.length !== 3) {
      throw new Error(`Expected 3 results, got ${data?.results.length}`);
    }
    // Each insert should return last_insert_id
    for (let i = 0; i < 3; i++) {
      const result = data.results[i] as { last_insert_id: number };
      if (typeof result.last_insert_id !== "number") {
        throw new Error(`Expected last_insert_id in result ${i}, got ${JSON.stringify(result)}`);
      }
    }
  });

  // Test batch with mixed operations
  await test("Batch: mixed insert, update, delete", async () => {
    const { data, error } = await client.batch([
      client.from("sdk_test").insert({ id: 20, name: "ToUpdate", email: "toupdate@test.com", age: 30 }),
      client.from("sdk_test").update({ name: "User10Updated" }).where(eq("id", 10)),
      client.from("sdk_test").delete().where(eq("id", 12)),
    ]);

    if (error) throw new Error(error.message);
    if (!data || data.results.length !== 3) {
      throw new Error(`Expected 3 results, got ${data?.results.length}`);
    }

    // Result 0: insert
    const r0 = data.results[0] as { last_insert_id: number };
    if (typeof r0.last_insert_id !== "number") {
      throw new Error(`Expected last_insert_id, got ${JSON.stringify(r0)}`);
    }

    // Result 1: update
    const r1 = data.results[1] as { rows_affected: number };
    if (r1.rows_affected !== 1) {
      throw new Error(`Expected 1 row affected, got ${r1.rows_affected}`);
    }

    // Result 2: delete
    const r2 = data.results[2] as { rows_affected: number };
    if (r2.rows_affected !== 1) {
      throw new Error(`Expected 1 row deleted, got ${r2.rows_affected}`);
    }
  });

  // Test batch with select
  await test("Batch: select operations", async () => {
    const { data, error } = await client.batch([
      client.from("sdk_test").select("id", "name").where(eq("id", 1)),
      client.from("sdk_test").select("id", "name").where(eq("id", 2)),
    ]);

    if (error) throw new Error(error.message);
    if (!data || data.results.length !== 2) {
      throw new Error(`Expected 2 results, got ${data?.results.length}`);
    }

    const r0 = data.results[0] as Array<{ id: number; name: string }>;
    const r1 = data.results[1] as Array<{ id: number; name: string }>;

    if (!Array.isArray(r0) || r0.length !== 1 || r0[0].name !== "Alice") {
      throw new Error(`Expected Alice, got ${JSON.stringify(r0)}`);
    }
    if (!Array.isArray(r1) || r1.length !== 1 || r1[0].name !== "Bob") {
      throw new Error(`Expected Bob, got ${JSON.stringify(r1)}`);
    }
  });

  // Test batch with single()
  await test("Batch: single() modifier", async () => {
    const { data, error } = await client.batch([
      client.from("sdk_test").select().where(eq("id", 1)).single(),
      client.from("sdk_test").select().where(eq("id", 2)).single(),
    ]);

    if (error) throw new Error(error.message);
    if (!data || data.results.length !== 2) {
      throw new Error(`Expected 2 results, got ${data?.results.length}`);
    }

    // single() should return object, not array
    const r0 = data.results[0] as { id: number; name: string };
    const r1 = data.results[1] as { id: number; name: string };

    if (Array.isArray(r0) || r0.id !== 1 || r0.name !== "Alice") {
      throw new Error(`Expected single Alice object, got ${JSON.stringify(r0)}`);
    }
    if (Array.isArray(r1) || r1.id !== 2 || r1.name !== "Bob") {
      throw new Error(`Expected single Bob object, got ${JSON.stringify(r1)}`);
    }
  });

  // Test batch with single() returning NOT_FOUND
  await test("Batch: single() with no rows returns error object", async () => {
    const { data, error } = await client.batch([
      client.from("sdk_test").select().where(eq("id", 1)).single(),
      client.from("sdk_test").select().where(eq("id", 99999)).single(), // No rows
    ]);

    if (error) throw new Error(error.message);
    if (!data || data.results.length !== 2) {
      throw new Error(`Expected 2 results, got ${data?.results.length}`);
    }

    // First result should be Alice
    const r0 = data.results[0] as { id: number; name: string };
    if (r0.id !== 1) {
      throw new Error(`Expected Alice, got ${JSON.stringify(r0)}`);
    }

    // Second result should be error indicator
    const r1 = data.results[1] as { __error: string; message: string };
    if (r1.__error !== "NOT_FOUND") {
      throw new Error(`Expected NOT_FOUND error, got ${JSON.stringify(r1)}`);
    }
  });

  // Test batch with maybeSingle()
  await test("Batch: maybeSingle() modifier", async () => {
    const { data, error } = await client.batch([
      client.from("sdk_test").select().where(eq("id", 1)).maybeSingle(),
      client.from("sdk_test").select().where(eq("id", 99999)).maybeSingle(), // No rows - should return null
    ]);

    if (error) throw new Error(error.message);
    if (!data || data.results.length !== 2) {
      throw new Error(`Expected 2 results, got ${data?.results.length}`);
    }

    // First result should be Alice
    const r0 = data.results[0] as { id: number; name: string };
    if (r0.id !== 1 || r0.name !== "Alice") {
      throw new Error(`Expected Alice, got ${JSON.stringify(r0)}`);
    }

    // Second result should be null (not an error)
    const r1 = data.results[1];
    if (r1 !== null) {
      throw new Error(`Expected null for no rows, got ${JSON.stringify(r1)}`);
    }
  });

  // Test batch with count()
  await test("Batch: count() modifier", async () => {
    const { data, error } = await client.batch([
      client.from("sdk_test").select().count(),
      client.from("sdk_test").select().where(gt("age", 25)).count(),
    ]);

    if (error) throw new Error(error.message);
    if (!data || data.results.length !== 2) {
      throw new Error(`Expected 2 results, got ${data?.results.length}`);
    }

    // Count should return a number directly
    const totalCount = data.results[0] as number;
    const filteredCount = data.results[1] as number;

    if (typeof totalCount !== "number" || totalCount < 5) {
      throw new Error(`Expected total count >= 5, got ${totalCount}`);
    }
    if (typeof filteredCount !== "number") {
      throw new Error(`Expected filtered count to be number, got ${typeof filteredCount}`);
    }
  });

  // Test batch with withCount()
  await test("Batch: withCount() modifier", async () => {
    const { data, error } = await client.batch([
      client.from("sdk_test").select("id", "name").limit(2).withCount(),
    ]);

    if (error) throw new Error(error.message);
    if (!data || data.results.length !== 1) {
      throw new Error(`Expected 1 result, got ${data?.results.length}`);
    }

    // withCount should return { data: [...], count: N }
    const r0 = data.results[0] as { data: Array<{ id: number; name: string }>; count: number };
    if (!r0.data || !Array.isArray(r0.data)) {
      throw new Error(`Expected data array, got ${JSON.stringify(r0)}`);
    }
    if (r0.data.length !== 2) {
      throw new Error(`Expected 2 data rows (limit), got ${r0.data.length}`);
    }
    if (typeof r0.count !== "number" || r0.count < 5) {
      throw new Error(`Expected count >= 5, got ${r0.count}`);
    }
  });

  // Test batch atomicity - if one fails, all should rollback
  await test("Batch: atomicity - failure rolls back all", async () => {
    // Get current count
    const { data: beforeCount } = await client.from("sdk_test").select().count();

    // Try a batch where second operation will fail (insert duplicate id)
    const existingId = 1; // We know this exists
    const { data, error } = await client.batch([
      client.from("sdk_test").insert({ id: 100, name: "ShouldRollback", email: "rollback@test.com", age: 50 }),
      client.from("sdk_test").insert({ id: existingId, name: "Duplicate", email: "dup@test.com", age: 99 }), // Should fail - duplicate id
    ]);

    // The batch should fail
    if (!error) {
      throw new Error("Expected batch to fail due to duplicate id");
    }

    // Verify count is unchanged (rollback worked)
    const { data: afterCount } = await client.from("sdk_test").select().count();
    if (beforeCount !== afterCount) {
      throw new Error(`Expected count ${beforeCount} after rollback, got ${afterCount}`);
    }
  });

  // Test batch with upsert
  await test("Batch: upsert operations", async () => {
    const { data, error } = await client.batch([
      client.from("sdk_test").upsert({ id: 1, name: "Alice Upserted", email: "alice@example.com", age: 31 }),
      client.from("sdk_test").upsert({ id: 200, name: "NewUser", email: "newuser@test.com", age: 40 }),
    ]);

    if (error) throw new Error(error.message);
    if (!data || data.results.length !== 2) {
      throw new Error(`Expected 2 results, got ${data?.results.length}`);
    }

    // Verify the upserts worked
    const { data: alice } = await client.from("sdk_test").select().where(eq("id", 1)).single();
    if (alice?.name !== "Alice Upserted") {
      throw new Error(`Expected 'Alice Upserted', got ${alice?.name}`);
    }

    const { data: newUser } = await client.from("sdk_test").select().where(eq("id", 200)).single();
    if (newUser?.name !== "NewUser") {
      throw new Error(`Expected 'NewUser', got ${newUser?.name}`);
    }
  });

  // Test complex batch combining everything
  await test("Batch: complex mixed operations", async () => {
    const { data, error } = await client.batch([
      client.from("sdk_test").select().where(eq("id", 1)).single(),           // 0: get single
      client.from("sdk_test").select().count(),                                // 1: count all
      client.from("sdk_test").insert({ id: 300, name: "BatchTest", email: "batch@test.com", age: 33 }), // 2: insert
      client.from("sdk_test").select().where(eq("id", 300)).maybeSingle(),    // 3: verify insert
      client.from("sdk_test").update({ age: 34 }).where(eq("id", 300)),       // 4: update
      client.from("sdk_test").select("age").where(eq("id", 300)).single(),    // 5: verify update
      client.from("sdk_test").delete().where(eq("id", 300)),                  // 6: delete
      client.from("sdk_test").select().where(eq("id", 300)).maybeSingle(),    // 7: verify delete
    ]);

    if (error) throw new Error(error.message);
    if (!data || data.results.length !== 8) {
      throw new Error(`Expected 8 results, got ${data?.results.length}`);
    }

    // 0: single should return Alice object
    const r0 = data.results[0] as { id: number; name: string };
    if (r0.id !== 1) throw new Error(`Result 0: Expected id=1, got ${r0.id}`);

    // 1: count should be a number
    const r1 = data.results[1] as number;
    if (typeof r1 !== "number") throw new Error(`Result 1: Expected number, got ${typeof r1}`);

    // 2: insert should return last_insert_id
    const r2 = data.results[2] as { last_insert_id: number };
    if (typeof r2.last_insert_id !== "number") throw new Error(`Result 2: Expected last_insert_id`);

    // 3: maybeSingle should return the inserted row
    const r3 = data.results[3] as { id: number; name: string };
    if (r3?.name !== "BatchTest") throw new Error(`Result 3: Expected BatchTest, got ${r3?.name}`);

    // 4: update should affect 1 row
    const r4 = data.results[4] as { rows_affected: number };
    if (r4.rows_affected !== 1) throw new Error(`Result 4: Expected 1 row affected`);

    // 5: single should show updated age
    const r5 = data.results[5] as { age: number };
    if (r5.age !== 34) throw new Error(`Result 5: Expected age=34, got ${r5.age}`);

    // 6: delete should affect 1 row
    const r6 = data.results[6] as { rows_affected: number };
    if (r6.rows_affected !== 1) throw new Error(`Result 6: Expected 1 row deleted`);

    // 7: maybeSingle should return null (deleted)
    const r7 = data.results[7];
    if (r7 !== null) throw new Error(`Result 7: Expected null after delete, got ${JSON.stringify(r7)}`);
  });

  // =========================================================================
  // EXPLICIT JOIN TESTS IN BATCH
  // =========================================================================
  console.log("\n=== Explicit Join Tests in Batch ===\n");

  // Set up test data for joins (using users and posts tables)
  await client.from("posts").delete().where(gt("id", 0));
  await client.from("users").delete().where(gt("id", 0));

  // Insert users
  await client.from("users").insert({ id: 1, name: "Alice", email: "alice@join.test", age: 30 });
  await client.from("users").insert({ id: 2, name: "Bob", email: "bob@join.test", age: 25 });
  await client.from("users").insert({ id: 3, name: "Charlie", email: "charlie@join.test", age: 35 }); // No posts

  // Insert posts
  await client.from("posts").insert({ id: 1, title: "Alice Post 1", content: "Content 1", user_id: 1 });
  await client.from("posts").insert({ id: 2, title: "Alice Post 2", content: "Content 2", user_id: 1 });
  await client.from("posts").insert({ id: 3, title: "Bob Post 1", content: "Content 3", user_id: 2 });

  // Test explicit LEFT JOIN in batch (flat output)
  await test("Batch: explicit LEFT JOIN", async () => {
    const { data, error } = await client.batch([
      client
        .from("users")
        .select("users.id", "users.name", "posts.title")
        .leftJoin("posts", onEq("users.id", "posts.user_id"), { flat: true })
        .where(eq("users.id", 1)),
    ]);

    if (error) throw new Error(error.message);
    if (!data || data.results.length !== 1) {
      throw new Error(`Expected 1 result, got ${data?.results.length}`);
    }

    const rows = data.results[0] as Array<{ id: number; name: string; posts_title: string }>;
    if (!Array.isArray(rows)) {
      throw new Error(`Expected array result, got ${JSON.stringify(rows)}`);
    }
    // Alice has 2 posts, so should get 2 rows (flat output)
    if (rows.length !== 2) {
      throw new Error(`Expected 2 rows for Alice's posts, got ${rows.length}`);
    }
    if (rows[0].name !== "Alice") {
      throw new Error(`Expected Alice, got ${rows[0].name}`);
    }
  });

  // Test explicit INNER JOIN in batch (flat output)
  await test("Batch: explicit INNER JOIN", async () => {
    const { data, error } = await client.batch([
      client
        .from("users")
        .select("users.id", "users.name", "posts.title")
        .innerJoin("posts", onEq("users.id", "posts.user_id"), { flat: true }),
    ]);

    if (error) throw new Error(error.message);
    if (!data || data.results.length !== 1) {
      throw new Error(`Expected 1 result, got ${data?.results.length}`);
    }

    const rows = data.results[0] as Array<{ id: number; name: string; posts_title: string }>;
    if (!Array.isArray(rows)) {
      throw new Error(`Expected array result, got ${JSON.stringify(rows)}`);
    }
    // Inner join: only users with posts (Alice=2, Bob=1), Charlie excluded (flat output)
    if (rows.length !== 3) {
      throw new Error(`Expected 3 rows (inner join), got ${rows.length}`);
    }
  });

  // Test LEFT JOIN shows nulls for users without posts (flat output)
  await test("Batch: LEFT JOIN includes users without posts", async () => {
    const { data, error } = await client.batch([
      client
        .from("users")
        .select("users.id", "users.name", "posts.title")
        .leftJoin("posts", onEq("users.id", "posts.user_id"), { flat: true })
        .where(eq("users.id", 3)), // Charlie has no posts
    ]);

    if (error) throw new Error(error.message);

    const rows = data!.results[0] as Array<{ id: number; name: string; posts_title: string | null }>;
    if (rows.length !== 1) {
      throw new Error(`Expected 1 row for Charlie, got ${rows.length}`);
    }
    if (rows[0].name !== "Charlie") {
      throw new Error(`Expected Charlie, got ${rows[0].name}`);
    }
    if (rows[0].posts_title !== null) {
      throw new Error(`Expected null posts_title for Charlie (no posts), got ${rows[0].posts_title}`);
    }
  });

  // Test multiple joins in a single batch (flat output)
  await test("Batch: multiple join queries", async () => {
    const { data, error } = await client.batch([
      // Query 1: Left join for user 1 (flat)
      client
        .from("users")
        .select("users.name", "posts.title")
        .leftJoin("posts", onEq("users.id", "posts.user_id"), { flat: true })
        .where(eq("users.id", 1)),
      // Query 2: Inner join for all (flat)
      client
        .from("users")
        .select("users.name", "posts.title")
        .innerJoin("posts", onEq("users.id", "posts.user_id"), { flat: true }),
      // Query 3: Count with join (flat)
      client
        .from("users")
        .select()
        .innerJoin("posts", onEq("users.id", "posts.user_id"), { flat: true })
        .count(),
    ]);

    if (error) throw new Error(error.message);
    if (!data || data.results.length !== 3) {
      throw new Error(`Expected 3 results, got ${data?.results.length}`);
    }

    // Result 0: Alice's posts (2 rows, flat output)
    const r0 = data.results[0] as Array<{ name: string; posts_title: string }>;
    if (r0.length !== 2) {
      throw new Error(`Expected 2 rows for Alice, got ${r0.length}`);
    }

    // Result 1: All users with posts (3 rows total, flat output)
    const r1 = data.results[1] as Array<{ name: string; posts_title: string }>;
    if (r1.length !== 3) {
      throw new Error(`Expected 3 rows for inner join, got ${r1.length}`);
    }

    // Result 2: Count of joined rows
    const r2 = data.results[2] as number;
    if (typeof r2 !== "number" || r2 !== 3) {
      throw new Error(`Expected count=3, got ${r2}`);
    }
  });

  // Test join with single() modifier (flat output)
  await test("Batch: join with single() modifier", async () => {
    const { data, error } = await client.batch([
      client
        .from("users")
        .select("users.name", "posts.title")
        .leftJoin("posts", onEq("users.id", "posts.user_id"), { flat: true })
        .where(eq("posts.id", 1))
        .single(),
    ]);

    if (error) throw new Error(error.message);

    const row = data!.results[0] as { name: string; posts_title: string };
    if (Array.isArray(row)) {
      throw new Error(`Expected single object, got array`);
    }
    if (row.name !== "Alice" || row.posts_title !== "Alice Post 1") {
      throw new Error(`Expected Alice/Alice Post 1, got ${row.name}/${row.posts_title}`);
    }
  });

  console.log("\n=== All Tests Complete ===\n");
}

run().catch(console.error);
