import {
  createClient,
  eq,
  gt,
  or,
  inArray,
  isNull,
  isNotNull,
  AtomicbaseError,
} from "./src/index.js";

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

    if (error) throw new Error(error.message);
    if (!data || data.id !== 1) {
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

    if (error) throw new Error(error.message);
    if (data?.status !== "inactive") {
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

  console.log("\n=== Tests Complete ===\n");
}

run().catch(console.error);
