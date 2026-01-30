import { Command } from "commander";
import { createInterface } from "readline";
import { loadConfig } from "../config.js";
import { ApiClient, ApiError } from "../api.js";

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
 * Format a date string for display.
 */
function formatDate(dateStr: string): string {
  const date = new Date(dateStr);
  return date.toLocaleString();
}

/**
 * List all templates.
 */
async function listTemplates(): Promise<void> {
  const config = await loadConfig();
  const api = new ApiClient(config);

  try {
    const templates = await api.listTemplates();

    if (templates.length === 0) {
      console.log("No templates found.");
      console.log("\nCreate one with: atomicbase push");
      return;
    }

    console.log("Templates:\n");
    console.log("  NAME                 VERSION    CREATED                  UPDATED");
    console.log("  " + "-".repeat(75));

    for (const template of templates) {
      const name = template.name.padEnd(20);
      const version = String(template.currentVersion).padEnd(10);
      const created = formatDate(template.createdAt).padEnd(24);
      const updated = formatDate(template.updatedAt);
      console.log(`  ${name} ${version} ${created} ${updated}`);
    }

    console.log(`\n  Total: ${templates.length} template(s)`);
  } catch (err) {
    console.error("Failed to list templates:", err instanceof Error ? err.message : err);
    process.exit(1);
  }
}

/**
 * Get template details.
 */
async function getTemplate(name: string): Promise<void> {
  const config = await loadConfig();
  const api = new ApiClient(config);

  try {
    const template = await api.getTemplate(name);

    console.log(`Template: ${template.name}\n`);
    console.log(`  ID:              ${template.id}`);
    console.log(`  Current Version: ${template.currentVersion}`);
    console.log(`  Created:         ${formatDate(template.createdAt)}`);
    console.log(`  Updated:         ${formatDate(template.updatedAt)}`);
    console.log(`  Tables:          ${template.schema.tables.length}`);

    if (template.schema.tables.length > 0) {
      console.log("\n  Schema:");
      for (const table of template.schema.tables) {
        const columnCount = Array.isArray(table.columns)
          ? table.columns.length
          : Object.keys(table.columns).length;
        console.log(`    - ${table.name} (${columnCount} columns)`);
      }
    }
  } catch (err) {
    if (err instanceof ApiError && err.status === 404) {
      console.error(`Template "${name}" not found.`);
      console.error("\nUse 'atomicbase templates list' to see available templates.");
      process.exit(1);
    }
    console.error("Failed to get template:", err instanceof Error ? err.message : err);
    process.exit(1);
  }
}

/**
 * Delete a template.
 */
async function deleteTemplate(name: string, force: boolean): Promise<void> {
  const config = await loadConfig();
  const api = new ApiClient(config);

  if (!force) {
    // Verify template exists first
    try {
      await api.getTemplate(name);
    } catch (err) {
      if (err instanceof ApiError && err.status === 404) {
        console.error(`Template "${name}" not found.`);
        process.exit(1);
      }
      throw err;
    }

    const confirmed = await confirm(
      `Are you sure you want to delete template "${name}"? This action cannot be undone.`
    );
    if (!confirmed) {
      console.log("Aborted.");
      process.exit(0);
    }
  }

  console.log(`Deleting template "${name}"...`);

  try {
    await api.deleteTemplate(name);
    console.log(`Deleted template "${name}"`);
  } catch (err) {
    if (err instanceof ApiError) {
      if (err.status === 404) {
        console.error(`Template "${name}" not found.`);
        process.exit(1);
      }
      if (err.code === "TEMPLATE_IN_USE") {
        console.error(`\nTemplate "${name}" is in use by tenants.`);
        console.error("Delete all tenants using this template first.");
        process.exit(1);
      }
    }
    console.error("Failed to delete template:", err instanceof Error ? err.message : err);
    process.exit(1);
  }
}

/**
 * Show version history for a template.
 */
async function showHistory(name: string): Promise<void> {
  const config = await loadConfig();
  const api = new ApiClient(config);

  try {
    const history = await api.getTemplateHistory(name);

    if (history.length === 0) {
      console.log(`No history found for template "${name}".`);
      return;
    }

    console.log(`Version history for template "${name}":\n`);
    console.log("  VERSION    CHECKSUM                           CREATED");
    console.log("  " + "-".repeat(70));

    for (const version of history) {
      const v = String(version.version).padEnd(10);
      const checksum = version.checksum.substring(0, 32).padEnd(34);
      const created = formatDate(version.createdAt);
      console.log(`  ${v} ${checksum} ${created}`);
    }

    console.log(`\n  Total: ${history.length} version(s)`);
    console.log(`\n  To rollback: atomicbase templates rollback ${name} <version>`);
  } catch (err) {
    if (err instanceof ApiError && err.status === 404) {
      console.error(`Template "${name}" not found.`);
      console.error("\nUse 'atomicbase templates list' to see available templates.");
      process.exit(1);
    }
    console.error("Failed to get history:", err instanceof Error ? err.message : err);
    process.exit(1);
  }
}

/**
 * Rollback a template to a previous version.
 */
async function rollbackTemplate(name: string, version: string, force: boolean): Promise<void> {
  const config = await loadConfig();
  const api = new ApiClient(config);

  const targetVersion = parseInt(version, 10);
  if (isNaN(targetVersion) || targetVersion < 1) {
    console.error("Version must be a positive integer.");
    process.exit(1);
  }

  // Verify template and version exist
  let template;
  let history;
  try {
    template = await api.getTemplate(name);
    history = await api.getTemplateHistory(name);
  } catch (err) {
    if (err instanceof ApiError && err.status === 404) {
      console.error(`Template "${name}" not found.`);
      process.exit(1);
    }
    throw err;
  }

  if (template.currentVersion === targetVersion) {
    console.log(`Template "${name}" is already at version ${targetVersion}.`);
    return;
  }

  const versionExists = history.some(v => v.version === targetVersion);
  if (!versionExists) {
    console.error(`Version ${targetVersion} not found in history.`);
    console.error("\nUse 'atomicbase templates history " + name + "' to see available versions.");
    process.exit(1);
  }

  if (!force) {
    const confirmed = await confirm(
      `Rollback template "${name}" from v${template.currentVersion} to v${targetVersion}? This will migrate all tenants.`
    );
    if (!confirmed) {
      console.log("Aborted.");
      process.exit(0);
    }
  }

  console.log(`Rolling back template "${name}" to version ${targetVersion}...`);

  try {
    const result = await api.rollbackTemplate(name, targetVersion);
    console.log(`Rollback initiated. Job ID: ${result.jobId}`);
    console.log(`\nTrack progress with: atomicbase jobs ${result.jobId}`);
  } catch (err) {
    if (err instanceof ApiError) {
      if (err.code === "VERSION_NOT_FOUND") {
        console.error(`Version ${targetVersion} not found.`);
        process.exit(1);
      }
      if (err.code === "NO_CHANGES") {
        console.log("No schema changes needed for this rollback.");
        return;
      }
    }
    console.error("Failed to rollback:", err instanceof Error ? err.message : err);
    process.exit(1);
  }
}

// Main templates command with subcommands
export const templatesCommand = new Command("templates")
  .description("Manage schema templates");

// templates list
templatesCommand
  .command("list")
  .alias("ls")
  .description("List all templates")
  .action(listTemplates);

// templates get <name>
templatesCommand
  .command("get <name>")
  .description("Get template details")
  .action(getTemplate);

// templates delete <name>
templatesCommand
  .command("delete <name>")
  .alias("rm")
  .description("Delete a template (only if no tenants use it)")
  .option("-f, --force", "Skip confirmation prompt")
  .action((name: string, options: { force?: boolean }) => {
    deleteTemplate(name, options.force ?? false);
  });

// templates history <name>
templatesCommand
  .command("history <name>")
  .description("View version history for a template")
  .action(showHistory);

// templates rollback <name> <version>
templatesCommand
  .command("rollback <name> <version>")
  .description("Rollback to a previous schema version")
  .option("-f, --force", "Skip confirmation prompt")
  .action((name: string, version: string, options: { force?: boolean }) => {
    rollbackTemplate(name, version, options.force ?? false);
  });
