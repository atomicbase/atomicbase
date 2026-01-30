import { Command } from "commander";
import { loadConfig } from "../config.js";
import { ApiClient, ApiError, type Migration } from "../api.js";

/**
 * Format a date string for display.
 */
function formatDate(dateStr: string | undefined): string {
  if (!dateStr) return "-";
  const date = new Date(dateStr);
  return date.toLocaleString();
}

/**
 * Get status display with state.
 */
function formatStatus(migration: Migration): string {
  if (migration.status === "complete" && migration.state) {
    return `${migration.status} (${migration.state})`;
  }
  return migration.status;
}

/**
 * List all migrations.
 */
async function listMigrations(options: { status?: string }): Promise<void> {
  const config = await loadConfig();
  const api = new ApiClient(config);

  try {
    const migrations = await api.listMigrations(options.status);

    if (migrations.length === 0) {
      console.log("No migrations found.");
      return;
    }

    console.log("Migrations:\n");
    console.log("  ID    TEMPLATE   VERSION      STATUS              PROGRESS     CREATED");
    console.log("  " + "-".repeat(80));

    for (const migration of migrations) {
      const id = String(migration.id).padEnd(5);
      const templateId = String(migration.templateId).padEnd(10);
      const version = `v${migration.fromVersion} -> v${migration.toVersion}`.padEnd(12);
      const status = formatStatus(migration).padEnd(19);
      const progress = migration.totalDbs > 0
        ? `${migration.completedDbs}/${migration.totalDbs}`.padEnd(12)
        : "-".padEnd(12);
      const created = formatDate(migration.createdAt);
      console.log(`  ${id} ${templateId} ${version} ${status} ${progress} ${created}`);
    }

    console.log(`\n  Total: ${migrations.length} migration(s)`);
  } catch (err) {
    console.error("Failed to list migrations:", err instanceof Error ? err.message : err);
    process.exit(1);
  }
}

/**
 * Get migration details.
 */
async function getMigration(migrationId: string): Promise<void> {
  const config = await loadConfig();
  const api = new ApiClient(config);

  const id = parseInt(migrationId, 10);
  if (isNaN(id)) {
    console.error("Invalid migration ID. Must be a number.");
    process.exit(1);
  }

  try {
    const migration = await api.getMigration(id);

    console.log(`Migration #${migration.id}\n`);
    console.log(`  Template ID:    ${migration.templateId}`);
    console.log(`  Version:        v${migration.fromVersion} -> v${migration.toVersion}`);
    console.log(`  Status:         ${formatStatus(migration)}`);
    console.log(`  Progress:       ${migration.completedDbs}/${migration.totalDbs} completed, ${migration.failedDbs} failed`);
    console.log(`  Created:        ${formatDate(migration.createdAt)}`);
    if (migration.startedAt) {
      console.log(`  Started:        ${formatDate(migration.startedAt)}`);
    }
    if (migration.completedAt) {
      console.log(`  Completed:      ${formatDate(migration.completedAt)}`);
    }

    if (migration.sql && migration.sql.length > 0) {
      console.log(`\n  SQL Statements (${migration.sql.length}):`);
      for (const stmt of migration.sql) {
        console.log(`    ${stmt}`);
      }
    }
  } catch (err) {
    if (err instanceof ApiError && err.status === 404) {
      console.error(`Migration #${migrationId} not found.`);
      process.exit(1);
    }
    console.error("Failed to get migration:", err instanceof Error ? err.message : err);
    process.exit(1);
  }
}

/**
 * Retry a failed migration.
 */
async function retryMigration(migrationId: string): Promise<void> {
  const config = await loadConfig();
  const api = new ApiClient(config);

  const id = parseInt(migrationId, 10);
  if (isNaN(id)) {
    console.error("Invalid migration ID. Must be a number.");
    process.exit(1);
  }

  console.log(`Retrying migration #${id}...`);

  try {
    const result = await api.retryMigration(id);

    if (result.retriedCount === 0) {
      console.log("No failed tenants to retry.");
    } else {
      console.log(`Retrying ${result.retriedCount} tenant(s)`);
      console.log(`  New migration ID: ${result.migrationId}`);
    }
  } catch (err) {
    if (err instanceof ApiError) {
      if (err.status === 404) {
        console.error(`Migration #${migrationId} not found.`);
        process.exit(1);
      }
      if (err.code === "ATOMICBASE_BUSY") {
        console.error("A migration is already in progress. Wait for it to complete.");
        process.exit(1);
      }
    }
    console.error("Failed to retry migration:", err instanceof Error ? err.message : err);
    process.exit(1);
  }
}

// Main migrations command with subcommands
export const migrationsCommand = new Command("migrations")
  .description("Manage schema migrations")
  .argument("[migration_id]", "Migration ID to get details for")
  .option("-s, --status <status>", "Filter by status (pending, running, complete)")
  .action(async (migrationId?: string, options?: { status?: string }) => {
    if (migrationId) {
      // atomicbase migrations <migration_id>
      await getMigration(migrationId);
    } else {
      // atomicbase migrations (list all)
      await listMigrations(options ?? {});
    }
  });

// migrations list
migrationsCommand
  .command("list")
  .alias("ls")
  .description("List all migrations")
  .option("-s, --status <status>", "Filter by status (pending, running, complete)")
  .action((options: { status?: string }) => listMigrations(options));

// migrations retry <migration_id>
migrationsCommand
  .command("retry <migration_id>")
  .description("Retry failed tenants in a migration")
  .action(retryMigration);
