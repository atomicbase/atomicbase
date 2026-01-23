import { Command } from "commander";
import { loadConfig } from "../config.js";
import { loadSchema, loadAllSchemas } from "../schema/parser.js";
import { ApiClient } from "../api.js";

export const diffCommand = new Command("diff")
  .description("Preview changes without applying")
  .argument("[file]", "Schema file to diff (or diff all if omitted)")
  .action(async (file?: string) => {
    const config = await loadConfig();
    const api = new ApiClient(config);

    // Load schema(s)
    const schemas = file
      ? [await loadSchema(file)]
      : await loadAllSchemas(config.schemas);

    if (schemas.length === 0) {
      console.log(`No schema files found in ${config.schemas}/`);
      return;
    }

    // Diff each schema
    for (const schema of schemas) {
      console.log(`Diffing schema "${schema.name}"...`);
      console.log("");

      try {
        const result = await api.diffSchema(schema.name, schema);

        if (result.changes.length === 0) {
          console.log("  No changes");
        } else {
          console.log("Changes:");
          for (const change of result.changes) {
            const prefix = change.type.startsWith("add") ? "+" :
                          change.type.startsWith("drop") ? "-" : "~";
            console.log(`  ${prefix} ${change.table}${change.column ? `.${change.column}` : ""} (${change.type})`);
            if (change.sql) {
              console.log(`    SQL: ${change.sql}`);
            }
          }

          if (result.requiresMigration) {
            console.log("\n⚠ Some changes require data migration (mirror table rebuild)");
          }
        }

        console.log("");
      } catch (err: unknown) {
        // Template might not exist yet - that's fine, show what would be created
        if (err instanceof Error && err.message.includes("404")) {
          console.log("  Template does not exist yet. Push will create:");
          console.log(`  + ${schema.tables.length} table(s)`);
          for (const table of schema.tables) {
            console.log(`    + ${table.name} (${table.columns.length} columns)`);
          }
          console.log("");
        } else {
          console.error(`✗ Failed to diff "${schema.name}":`, err);
        }
      }
    }
  });
