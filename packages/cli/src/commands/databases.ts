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
 * List all databases.
 */
async function listDatabases(): Promise<void> {
  const config = await loadConfig();
  const api = new ApiClient(config);

  try {
    const databases = await api.listDatabases();

    if (databases.length === 0) {
      console.log("No databases found.");
      console.log("\nCreate one with: atomicbase databases create <name> --template <template>");
      return;
    }

    console.log("Tenants:\n");
    console.log("  NAME                 TEMPLATE    VERSION    CREATED");
    console.log("  " + "-".repeat(70));

    for (const database of databases) {
      const name = database.name.padEnd(20);
      const templateId = String(database.templateId).padEnd(11);
      const version = String(database.templateVersion).padEnd(10);
      const created = formatDate(database.createdAt);
      console.log(`  ${name} ${templateId} ${version} ${created}`);
    }

    console.log(`\n  Total: ${databases.length} database(s)`);
  } catch (err) {
    console.error("Failed to list databases:", err instanceof ApiError ? err.format() : err);
    process.exit(1);
  }
}

/**
 * Get database details.
 */
async function getDatabase(name: string): Promise<void> {
  const config = await loadConfig();
  const api = new ApiClient(config);

  try {
    const database = await api.getDatabase(name);

    console.log(`Database: ${database.name}\n`);
    console.log(`  ID:               ${database.id}`);
    console.log(`  Template ID:      ${database.templateId}`);
    console.log(`  Template Version: ${database.templateVersion}`);
    console.log(`  Created:          ${formatDate(database.createdAt)}`);
    console.log(`  Updated:          ${formatDate(database.updatedAt)}`);
    if (database.token) {
      console.log(`  Token:            ${database.token}`);
    }
  } catch (err) {
    console.error("Failed to get database:", err instanceof ApiError ? err.format() : err);
    process.exit(1);
  }
}

/**
 * Create a new database.
 */
async function createDatabase(name: string, template: string): Promise<void> {
  const config = await loadConfig();
  const api = new ApiClient(config);

  console.log(`Creating database "${name}" with template "${template}"...`);

  try {
    const database = await api.createDatabase(name, template);

    console.log(`\n✓ Created database "${database.name}"`);
    console.log(`  ID:               ${database.id}`);
    console.log(`  Template ID:      ${database.templateId}`);
    console.log(`  Template Version: ${database.templateVersion}`);
    if (database.token) {
      console.log(`  Token:            ${database.token}`);
    }
  } catch (err) {
    console.error("\n✗ Failed to create database:", err instanceof ApiError ? err.format() : err);
    process.exit(1);
  }
}

/**
 * Delete a database.
 */
async function deleteDatabase(name: string, force: boolean): Promise<void> {
  const config = await loadConfig();
  const api = new ApiClient(config);

  if (!force) {
    // Verify database exists first
    try {
      await api.getDatabase(name);
    } catch (err) {
      if (err instanceof ApiError && err.status === 404) {
        console.error(`Database "${name}" not found.`);
        process.exit(1);
      }
      throw err;
    }

    const confirmed = await confirm(
      `Are you sure you want to delete database "${name}"? This action cannot be undone.`
    );
    if (!confirmed) {
      console.log("Aborted.");
      process.exit(0);
    }
  }

  console.log(`Deleting database "${name}"...`);

  try {
    await api.deleteDatabase(name);
    console.log(`✓ Deleted database "${name}"`);
  } catch (err) {
    console.error("Failed to delete database:", err instanceof ApiError ? err.format() : err);
    process.exit(1);
  }
}

/**
 * Sync a database to the latest template version.
 */
async function syncDatabase(name: string): Promise<void> {
  const config = await loadConfig();
  const api = new ApiClient(config);

  console.log(`Syncing database "${name}"...`);

  try {
    const result = await api.syncDatabase(name);
    console.log(`✓ Synced database "${name}" from v${result.fromVersion} to v${result.toVersion}`);
  } catch (err) {
    if (err instanceof ApiError && err.code === "DATABASE_IN_SYNC") {
      console.log(`✓ Database "${name}" is already at the latest version.`);
      return;
    }
    console.error("Failed to sync database:", err instanceof ApiError ? err.format() : err);
    process.exit(1);
  }
}

// Main databases command with subcommands
export const databasesCommand = new Command("databases")
  .description("Manage databases");

// databases list
databasesCommand
  .command("list")
  .alias("ls")
  .description("List all databases")
  .action(listDatabases);

// databases get <name>
databasesCommand
  .command("get <name>")
  .description("Get database details")
  .action(getDatabase);

// databases create <name> --template <template>
databasesCommand
  .command("create <name>")
  .description("Create a new database")
  .requiredOption("-t, --template <template>", "Template name to use")
  .action((name: string, options: { template: string }) => {
    createDatabase(name, options.template);
  });

// databases delete <name>
databasesCommand
  .command("delete <name>")
  .alias("rm")
  .description("Delete a database")
  .option("-f, --force", "Skip confirmation prompt")
  .action((name: string, options: { force?: boolean }) => {
    deleteDatabase(name, options.force ?? false);
  });

// databases sync <name>
databasesCommand
  .command("sync <name>")
  .description("Sync database to the latest template version")
  .action(syncDatabase);
