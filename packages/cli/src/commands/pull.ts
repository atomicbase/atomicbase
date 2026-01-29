import { Command } from "commander";
import { writeFileSync, readFileSync, mkdirSync, existsSync } from "node:fs";
import { createInterface } from "node:readline";
import { resolve, dirname } from "node:path";
import { loadConfig } from "../config.js";
import { loadAllSchemas } from "../schema/parser.js";
import { ApiClient, type TemplateResponse } from "../api.js";
import type { ColumnDefinition, ForeignKeyAction, Collation, SchemaDefinition } from "@atomicbase/schema";

/**
 * Prompt user for confirmation.
 */
async function confirm(message: string): Promise<boolean> {
  const rl = createInterface({ input: process.stdin, output: process.stdout });
  return new Promise((resolve) => {
    rl.question(`${message} (y/N) `, (answer) => {
      rl.close();
      resolve(answer.toLowerCase() === "y");
    });
  });
}

/**
 * Internal table format for code generation (columns as array with primaryKey flag).
 */
interface GeneratorTable {
  name: string;
  columns: GeneratorColumn[];
  indexes: { name: string; columns: string[]; unique?: boolean }[];
  ftsColumns: string[] | null;
}

interface GeneratorColumn extends Omit<ColumnDefinition, "references" | "onDelete" | "onUpdate"> {
  primaryKey: boolean;
  references: {
    table: string;
    column: string;
    onDelete?: ForeignKeyAction;
    onUpdate?: ForeignKeyAction;
  } | null;
}

/**
 * Convert API table format to internal generator format.
 * API: columns as Record<string, ColumnDefinition>, pk as string[]
 * Generator: columns as array with primaryKey flag
 */
function convertFromApiFormat(tables: TemplateResponse["schema"]["tables"]): GeneratorTable[] {
  return tables.map((table) => {
    const columns: GeneratorColumn[] = Object.entries(table.columns).map(([name, col]) => {
      const colDef: GeneratorColumn = {
        name,
        type: col.type,
        primaryKey: table.pk?.includes(name) ?? false,
        notNull: col.notNull,
        unique: col.unique,
        default: col.default,
        collate: col.collate as Collation | undefined,
        check: col.check,
        generated: col.generated,
        references: col.references
          ? {
              table: col.references.split(".")[0],
              column: col.references.split(".")[1],
              onDelete: col.onDelete as ForeignKeyAction | undefined,
              onUpdate: col.onUpdate as ForeignKeyAction | undefined,
            }
          : null,
      };
      return colDef;
    });

    return {
      name: table.name,
      columns,
      indexes: (table.indexes ?? []).map((idx) => ({
        name: idx.name,
        columns: idx.columns,
        unique: idx.unique,
      })),
      ftsColumns: table.ftsColumns ?? null,
    };
  });
}

/**
 * Generate TypeScript schema code from a template response.
 */
function generateSchemaCode(name: string, tables: GeneratorTable[]): string {
  const lines: string[] = [
    `import { defineSchema, defineTable, c } from "@atomicbase/schema";`,
    ``,
    `export default defineSchema("${name}", {`,
  ];

  for (let i = 0; i < tables.length; i++) {
    const table = tables[i];
    const isLast = i === tables.length - 1;

    lines.push(`  ${table.name}: defineTable({`);

    for (const col of table.columns) {
      lines.push(`    ${col.name}: ${generateColumnCode(col)},`);
    }

    let tableSuffix = "})";

    // Add index chains
    for (const idx of table.indexes) {
      const method = idx.unique ? "uniqueIndex" : "index";
      tableSuffix += `.${method}("${idx.name}", ${JSON.stringify(idx.columns)})`;
    }

    // Add FTS chain
    if (table.ftsColumns && table.ftsColumns.length > 0) {
      tableSuffix += `.fts(${JSON.stringify(table.ftsColumns)})`;
    }

    tableSuffix += isLast ? "," : ",";
    lines.push(`  ${tableSuffix}`);

    if (!isLast) {
      lines.push(``);
    }
  }

  lines.push(`});`);
  lines.push(``);

  return lines.join("\n");
}

function generateColumnCode(col: GeneratorColumn): string {
  let code = `c.${col.type.toLowerCase()}()`;

  if (col.primaryKey) {
    code += ".primaryKey()";
  }
  if (col.notNull) {
    code += ".notNull()";
  }
  if (col.unique) {
    code += ".unique()";
  }
  if (col.collate) {
    code += `.collate("${col.collate}")`;
  }
  if (col.default !== undefined && col.default !== null) {
    const val = typeof col.default === "string"
      ? `"${col.default}"`
      : col.default;
    code += `.default(${val})`;
  }
  if (col.check) {
    code += `.check("${col.check.replace(/"/g, '\\"')}")`;
  }
  if (col.generated) {
    const opts = col.generated.stored ? ", { stored: true }" : "";
    code += `.generatedAs("${col.generated.expr.replace(/"/g, '\\"')}"${opts})`;
  }
  if (col.references) {
    const ref = `"${col.references.table}.${col.references.column}"`;
    if (col.references.onDelete || col.references.onUpdate) {
      const opts: string[] = [];
      if (col.references.onDelete) {
        opts.push(`onDelete: "${col.references.onDelete}"`);
      }
      if (col.references.onUpdate) {
        opts.push(`onUpdate: "${col.references.onUpdate}"`);
      }
      code += `.references(${ref}, { ${opts.join(", ")} })`;
    } else {
      code += `.references(${ref})`;
    }
  }

  return code;
}

interface PullOperation {
  type: "add" | "update";
  name: string;
  version: number;
  tableCount: number;
  code: string;
  outputPath: string;
}

export const pullCommand = new Command("pull")
  .description("Pull schemas from the server")
  .option("-y, --yes", "Skip confirmation prompt")
  .action(async (options: { yes?: boolean }) => {
    const config = await loadConfig();
    const api = new ApiClient(config);

    console.log("Fetching templates...");

    try {
      // Load local templates
      let localSchemas: SchemaDefinition[] = [];
      try {
        localSchemas = await loadAllSchemas(config.schemas);
      } catch {
        // No local schemas or schemas directory doesn't exist - that's fine
      }
      const localTemplateNames = new Set(localSchemas.map((s) => s.name));

      // Fetch cloud templates
      const cloudTemplates = await api.listTemplates();

      if (cloudTemplates.length === 0) {
        console.log("No templates found on server.");
        return;
      }

      // Determine operations (add vs update vs skip)
      const operations: PullOperation[] = [];

      for (const templateInfo of cloudTemplates) {
        const template = await api.getTemplate(templateInfo.name);
        const code = generateSchemaCode(template.name, convertFromApiFormat(template.schema.tables));
        const outputPath = resolve(config.schemas, `${template.name}.schema.ts`);

        if (!localTemplateNames.has(template.name)) {
          // Template exists in cloud but not locally - add
          operations.push({
            type: "add",
            name: template.name,
            version: template.currentVersion,
            tableCount: template.schema.tables.length,
            code,
            outputPath,
          });
        } else {
          // Template exists both locally and in cloud - check for diff
          let existingCode = "";
          if (existsSync(outputPath)) {
            existingCode = readFileSync(outputPath, "utf-8");
          }

          if (existingCode !== code) {
            // Different content - update
            operations.push({
              type: "update",
              name: template.name,
              version: template.currentVersion,
              tableCount: template.schema.tables.length,
              code,
              outputPath,
            });
          }
          // If content is the same, skip (do nothing)
        }
      }

      if (operations.length === 0) {
        console.log("All local schemas are up to date.");
        return;
      }

      // Show operations
      const adds = operations.filter((op) => op.type === "add");
      const updates = operations.filter((op) => op.type === "update");

      console.log("");

      if (adds.length > 0) {
        console.log(`Templates to add (${adds.length}):`);
        for (const op of adds) {
          console.log(`  + ${op.name} (v${op.version}, ${op.tableCount} tables)`);
        }
      }

      if (updates.length > 0) {
        if (adds.length > 0) console.log("");
        console.log(`Templates to update (${updates.length}):`);
        for (const op of updates) {
          console.log(`  ~ ${op.name} (v${op.version}, ${op.tableCount} tables)`);
        }
      }

      // Confirm unless --yes flag is set
      if (!options.yes) {
        console.log("");
        const confirmed = await confirm("Apply these changes?");
        if (!confirmed) {
          console.log("Aborted.");
          return;
        }
      }

      // Ensure schemas directory exists
      const schemasDir = config.schemas;
      if (!existsSync(schemasDir)) {
        mkdirSync(schemasDir, { recursive: true });
      }

      // Apply operations
      console.log("");
      for (const op of operations) {
        const outputDir = dirname(op.outputPath);
        if (!existsSync(outputDir)) {
          mkdirSync(outputDir, { recursive: true });
        }

        writeFileSync(op.outputPath, op.code);
        const action = op.type === "add" ? "Added" : "Updated";
        console.log(`  ${action} ${op.name}.schema.ts`);
      }

      console.log(`\nPulled ${operations.length} schema(s) to ${schemasDir}`);
    } catch (err) {
      console.error("Failed to pull schemas:", err);
      process.exit(1);
    }
  });
