import { Command } from "commander";
import { createInterface } from "readline";
import { loadConfig } from "../config.js";
import { loadSchema, loadAllSchemas } from "../schema/parser.js";
import { ApiClient, ApiError, type SchemaDiff, type Merge } from "../api.js";
import type { SchemaDefinition } from "@atomicbase/schema";

/**
 * Ambiguous change detected client-side: a drop + add pair that might be a rename.
 */
interface AmbiguousChange {
  type: "table" | "column";
  table: string;
  column?: string;
  dropIndex: number;
  addIndex: number;
}

/**
 * Prompt user with a yes/no question.
 */
async function confirm(question: string): Promise<boolean> {
  const rl = createInterface({
    input: process.stdin,
    output: process.stdout,
  });

  return new Promise((resolve) => {
    rl.question(`${question} [Y/n] `, (answer) => {
      rl.close();
      const normalized = answer.trim().toLowerCase();
      resolve(normalized === "" || normalized === "y" || normalized === "yes");
    });
  });
}

/**
 * Detect ambiguous changes (drop + add pairs that might be renames).
 * This is now client-side since the API just returns raw changes.
 */
function detectAmbiguousChanges(changes: SchemaDiff[]): AmbiguousChange[] {
  const ambiguous: AmbiguousChange[] = [];

  // Find drop_table + add_table pairs
  const dropTables = changes
    .map((c, i) => ({ change: c, index: i }))
    .filter((c) => c.change.type === "drop_table");
  const addTables = changes
    .map((c, i) => ({ change: c, index: i }))
    .filter((c) => c.change.type === "add_table");

  // Any drop + add table is potentially ambiguous (might be a rename)
  for (const drop of dropTables) {
    for (const add of addTables) {
      ambiguous.push({
        type: "table",
        table: add.change.table!,
        dropIndex: drop.index,
        addIndex: add.index,
      });
    }
  }

  // Find drop_column + add_column pairs within the same table
  const dropColumns = changes
    .map((c, i) => ({ change: c, index: i }))
    .filter((c) => c.change.type === "drop_column");
  const addColumns = changes
    .map((c, i) => ({ change: c, index: i }))
    .filter((c) => c.change.type === "add_column");

  for (const drop of dropColumns) {
    for (const add of addColumns) {
      if (drop.change.table === add.change.table) {
        ambiguous.push({
          type: "column",
          table: add.change.table!,
          column: add.change.column,
          dropIndex: drop.index,
          addIndex: add.index,
        });
      }
    }
  }

  return ambiguous;
}

/**
 * Resolve ambiguous changes by prompting the user.
 * Returns Merge[] for the API.
 */
async function resolveAmbiguousChanges(
  changes: SchemaDiff[],
  ambiguous: AmbiguousChange[]
): Promise<Merge[]> {
  const merges: Merge[] = [];

  if (ambiguous.length === 0) {
    return merges;
  }

  console.log("\n⚠️  Ambiguous changes detected:\n");

  for (const change of ambiguous) {
    const dropChange = changes[change.dropIndex];
    const addChange = changes[change.addIndex];

    if (change.type === "table") {
      const isRename = await confirm(
        `  Table '${dropChange.table}' was removed and '${addChange.table}' was added.\n  Is this a rename?`
      );
      if (isRename) {
        merges.push({ old: change.dropIndex, new: change.addIndex });
      }
    } else {
      const isRename = await confirm(
        `  Column '${change.table}.${dropChange.column}' was removed and '${change.table}.${addChange.column}' was added.\n  Is this a rename?`
      );
      if (isRename) {
        merges.push({ old: change.dropIndex, new: change.addIndex });
      }
    }
  }

  console.log("");
  return merges;
}

/**
 * Push a single schema, handling ambiguous changes.
 */
async function pushSingleSchema(api: ApiClient, schema: SchemaDefinition): Promise<void> {
  console.log(`\nPushing schema "${schema.name}"...`);

  // Check if template exists
  const exists = await api.templateExists(schema.name);

  if (!exists) {
    // New template - just create it
    const result = await api.pushSchema(schema);
    console.log(`✓ Created template "${schema.name}" (v${result.currentVersion})`);
    return;
  }

  // Existing template - check for changes first
  let diff;
  try {
    diff = await api.diffSchema(schema.name, schema);
  } catch (err) {
    // API returns 400 NO_CHANGES when schema is identical
    if (err instanceof ApiError && err.code === "NO_CHANGES") {
      console.log("✓ No changes");
      return;
    }
    throw err;
  }

  if (!diff.changes || diff.changes.length === 0) {
    console.log("✓ No changes");
    return;
  }

  // Show what we detected
  printChanges(diff.changes);

  // Detect and resolve ambiguous changes (client-side)
  const ambiguous = detectAmbiguousChanges(diff.changes);
  let merges: Merge[] | undefined;
  if (ambiguous.length > 0) {
    merges = await resolveAmbiguousChanges(diff.changes, ambiguous);
    if (merges.length === 0) {
      merges = undefined;
    }
  }

  // Apply the migration
  const result = await api.migrateTemplate(schema.name, schema, merges);
  console.log(`✓ Migration started (job #${result.jobId})`);
  console.log(`  Check status: atomicbase jobs ${result.jobId}`);
}

/**
 * Print changes in a readable format.
 */
function printChanges(changes: SchemaDiff[]): void {
  if (!changes || changes.length === 0) {
    return;
  }

  console.log("\nChanges:");
  for (const change of changes) {
    const prefix =
      change.type.startsWith("add") ? "+" :
      change.type.startsWith("drop") ? "-" :
      change.type.startsWith("rename") ? "→" : "~";

    let desc = change.table || "";
    if (change.column) {
      desc += `.${change.column}`;
    }

    console.log(`  ${prefix} ${desc} (${change.type})`);
  }
}

export const pushCommand = new Command("push")
  .description("Push schema(s) to the server")
  .argument("[file]", "Schema file to push (or push all if omitted)")
  .action(async (file?: string) => {
    const config = await loadConfig();
    const api = new ApiClient(config);

    // Load schema(s)
    let schemas: SchemaDefinition[];
    try {
      schemas = file
        ? [await loadSchema(file)]
        : await loadAllSchemas(config.schemas);
    } catch (err) {
      if (err instanceof Error) {
        console.error(`\n✗ ${err.message}`);
      } else {
        console.error(`\n✗ Failed to load schemas:`, err);
      }
      process.exit(1);
    }

    if (schemas.length === 0) {
      console.log(`No schema files found in ${config.schemas}/`);
      console.log("Create a schema with: atomicbase init");
      return;
    }

    // Check for duplicate schema names
    const nameCount = new Map<string, number>();
    for (const schema of schemas) {
      nameCount.set(schema.name, (nameCount.get(schema.name) || 0) + 1);
    }
    const duplicates = [...nameCount.entries()].filter(([_, count]) => count > 1);
    if (duplicates.length > 0) {
      console.error("Error: Multiple schemas have the same name:\n");
      for (const [name, count] of duplicates) {
        console.error(`  "${name}" is defined ${count} times`);
      }
      console.error("\nEach schema must have a unique name. Check your schema files.");
      process.exit(1);
    }

    // Push each schema
    for (const schema of schemas) {
      try {
        await pushSingleSchema(api, schema);
      } catch (err) {
        console.error(`\n✗ Failed to push "${schema.name}":`, err);
        process.exit(1);
      }
    }

    console.log("\nDone!");
  });
