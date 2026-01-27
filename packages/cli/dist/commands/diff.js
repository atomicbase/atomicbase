import { Command } from "commander";
import { loadConfig } from "../config.js";
import { loadSchema, loadAllSchemas } from "../schema/parser.js";
import { ApiClient, ApiError } from "../api.js";
export const diffCommand = new Command("diff")
    .description("Preview changes without applying")
    .argument("[file]", "Schema file to diff (or diff all if omitted)")
    .action(async (file) => {
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
            }
            else {
                console.log("Changes:");
                for (const change of result.changes) {
                    const prefix = change.type.startsWith("add") ? "+" :
                        change.type.startsWith("drop") ? "-" : "~";
                    console.log(`  ${prefix} ${change.table}${change.column ? `.${change.column}` : ""} (${change.type})`);
                }
            }
            console.log("");
        }
        catch (err) {
            if (err instanceof ApiError) {
                // API returns 400 NO_CHANGES when schema is identical
                if (err.code === "NO_CHANGES") {
                    console.log("  No changes");
                    console.log("");
                    continue;
                }
                // Template might not exist yet - that's fine, show what would be created
                if (err.status === 404) {
                    console.log("  Template does not exist yet. Push will create:");
                    console.log(`  + ${schema.tables.length} table(s)`);
                    for (const table of schema.tables) {
                        const columnCount = Object.keys(table.columns).length;
                        console.log(`    + ${table.name} (${columnCount} columns)`);
                    }
                    console.log("");
                    continue;
                }
            }
            console.error(`✗ Failed to diff "${schema.name}":`, err);
        }
    }
});
//# sourceMappingURL=diff.js.map