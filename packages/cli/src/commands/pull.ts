import { Command } from "commander";
import { writeFileSync, mkdirSync, existsSync } from "node:fs";
import { resolve, dirname } from "node:path";
import { loadConfig } from "../config.js";
import { ApiClient, type TemplateResponse } from "../api.js";
import type { TableDefinition, ColumnDefinition, ForeignKeyAction, Collation } from "@atomicbase/schema";

/**
 * Convert API table format (columns as map) to SDK format (columns as array).
 */
function convertFromApiFormat(tables: TemplateResponse["tables"]): TableDefinition[] {
  return tables.map((table) => {
    const columns: ColumnDefinition[] = Object.values(table.columns).map((col) => {
      const colDef: ColumnDefinition = {
        name: col.name,
        type: col.type as ColumnDefinition["type"],
        primaryKey: table.pk?.includes(col.name) ?? false,
        notNull: col.notNull ?? false,
        unique: col.unique ?? false,
        defaultValue: col.default ?? null,
        collate: (col.collate as Collation) ?? null,
        check: col.check ?? null,
        generated: col.generated ?? null,
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
function generateSchemaCode(name: string, tables: TableDefinition[]): string {
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

function generateColumnCode(col: ColumnDefinition): string {
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
  if (col.defaultValue !== null) {
    const val = typeof col.defaultValue === "string"
      ? `"${col.defaultValue}"`
      : col.defaultValue;
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

export const pullCommand = new Command("pull")
  .description("Pull a schema from the server")
  .argument("<name>", "Template name to pull")
  .option("-o, --output <file>", "Output file path")
  .action(async (name: string, options: { output?: string }) => {
    const config = await loadConfig();
    const api = new ApiClient(config);

    console.log(`Pulling schema "${name}"...`);

    try {
      const template = await api.getTemplate(name);

      // Generate schema code
      const code = generateSchemaCode(name, convertFromApiFormat(template.tables));

      // Determine output path
      const outputPath = options.output ?? resolve(config.schemas, `${name}.schema.ts`);
      const outputDir = dirname(outputPath);

      // Ensure directory exists
      if (!existsSync(outputDir)) {
        mkdirSync(outputDir, { recursive: true });
      }

      // Write file
      writeFileSync(outputPath, code);
      console.log(`✓ Wrote ${outputPath}`);
      console.log(`  Version: ${template.currentVersion}`);
      console.log(`  Tables: ${template.tables.length}`);
    } catch (err) {
      console.error(`✗ Failed to pull "${name}":`, err);
      process.exit(1);
    }
  });
