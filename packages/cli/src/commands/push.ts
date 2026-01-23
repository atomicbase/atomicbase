import { Command } from "commander";
import { createInterface } from "readline";
import { loadConfig } from "../config.js";
import { loadSchema, loadAllSchemas } from "../schema/parser.js";
import { ApiClient, type Change, type ResolvedRename } from "../api.js";
import type { SchemaDefinition } from "@atomicbase/schema";

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
 * Resolve ambiguous changes by prompting the user.
 */
async function resolveAmbiguousChanges(changes: Change[]): Promise<ResolvedRename[]> {
  const resolutions: ResolvedRename[] = [];
  const ambiguous = changes.filter((c) => c.ambiguous);

  if (ambiguous.length === 0) {
    return resolutions;
  }

  console.log("\n⚠️  Ambiguous changes detected:\n");

  for (const change of ambiguous) {
    if (change.type === "rename_table") {
      const isRename = await confirm(
        `  Table '${change.oldName}' was removed and '${change.table}' was added.\n  Is this a rename?`
      );
      resolutions.push({
        type: "table",
        table: change.table,
        oldName: change.oldName!,
        isRename,
      });
    } else if (change.type === "rename_column") {
      const isRename = await confirm(
        `  Column '${change.table}.${change.oldName}' was removed and '${change.table}.${change.column}' was added.\n  Is this a rename?`
      );
      resolutions.push({
        type: "column",
        table: change.table,
        column: change.column,
        oldName: change.oldName!,
        isRename,
      });
    }
  }

  console.log("");
  return resolutions;
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
    console.log(`✓ Created template "${schema.name}" (v${result.template.currentVersion})`);
    printChanges(result.changes);
    return;
  }

  // Existing template - check for ambiguous changes first
  const diff = await api.diffSchema(schema.name, schema);

  if (!diff.changes || diff.changes.length === 0) {
    console.log("✓ No changes");
    return;
  }

  // Show what we detected
  printChanges(diff.changes);

  // Resolve ambiguous changes
  let resolvedRenames: ResolvedRename[] | undefined;
  if (diff.hasAmbiguous) {
    resolvedRenames = await resolveAmbiguousChanges(diff.changes);
  }

  // Apply the update
  const result = await api.updateTemplate(schema.name, schema, resolvedRenames);
  console.log(`✓ Updated to v${result.template.currentVersion}`);
}

/**
 * Print changes in a readable format.
 */
function printChanges(changes: Change[] | null): void {
  if (!changes || changes.length === 0) {
    return;
  }

  console.log("\nChanges:");
  for (const change of changes) {
    if (change.ambiguous) {
      continue; // Will be handled separately
    }

    const prefix =
      change.type.startsWith("add") ? "+" :
      change.type.startsWith("drop") ? "-" :
      change.type.startsWith("rename") ? "→" : "~";

    let desc = change.table;
    if (change.column) {
      desc += `.${change.column}`;
    }
    if (change.type.startsWith("rename") && change.oldName) {
      desc = `${change.oldName} → ${desc}`;
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
