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
 * List all tenants.
 */
async function listTenants(): Promise<void> {
  const config = await loadConfig();
  const api = new ApiClient(config);

  try {
    const tenants = await api.listTenants();

    if (tenants.length === 0) {
      console.log("No tenants found.");
      console.log("\nCreate one with: atomicbase tenant create <name> --template <template>");
      return;
    }

    console.log("Tenants:\n");
    console.log("  NAME                 TEMPLATE    VERSION    CREATED");
    console.log("  " + "-".repeat(70));

    for (const tenant of tenants) {
      const name = tenant.name.padEnd(20);
      const templateId = String(tenant.templateId).padEnd(11);
      const version = String(tenant.templateVersion).padEnd(10);
      const created = formatDate(tenant.createdAt);
      console.log(`  ${name} ${templateId} ${version} ${created}`);
    }

    console.log(`\n  Total: ${tenants.length} tenant(s)`);
  } catch (err) {
    console.error("Failed to list tenants:", err instanceof ApiError ? err.format() : err);
    process.exit(1);
  }
}

/**
 * Get tenant details.
 */
async function getTenant(name: string): Promise<void> {
  const config = await loadConfig();
  const api = new ApiClient(config);

  try {
    const tenant = await api.getTenant(name);

    console.log(`Tenant: ${tenant.name}\n`);
    console.log(`  ID:               ${tenant.id}`);
    console.log(`  Template ID:      ${tenant.templateId}`);
    console.log(`  Template Version: ${tenant.templateVersion}`);
    console.log(`  Created:          ${formatDate(tenant.createdAt)}`);
    console.log(`  Updated:          ${formatDate(tenant.updatedAt)}`);
    if (tenant.token) {
      console.log(`  Token:            ${tenant.token}`);
    }
  } catch (err) {
    console.error("Failed to get tenant:", err instanceof ApiError ? err.format() : err);
    process.exit(1);
  }
}

/**
 * Create a new tenant.
 */
async function createTenant(name: string, template: string): Promise<void> {
  const config = await loadConfig();
  const api = new ApiClient(config);

  console.log(`Creating tenant "${name}" with template "${template}"...`);

  try {
    const tenant = await api.createTenant(name, template);

    console.log(`\n✓ Created tenant "${tenant.name}"`);
    console.log(`  ID:               ${tenant.id}`);
    console.log(`  Template ID:      ${tenant.templateId}`);
    console.log(`  Template Version: ${tenant.templateVersion}`);
    if (tenant.token) {
      console.log(`  Token:            ${tenant.token}`);
    }
  } catch (err) {
    console.error("\n✗ Failed to create tenant:", err instanceof ApiError ? err.format() : err);
    process.exit(1);
  }
}

/**
 * Delete a tenant.
 */
async function deleteTenant(name: string, force: boolean): Promise<void> {
  const config = await loadConfig();
  const api = new ApiClient(config);

  if (!force) {
    // Verify tenant exists first
    try {
      await api.getTenant(name);
    } catch (err) {
      if (err instanceof ApiError && err.status === 404) {
        console.error(`Tenant "${name}" not found.`);
        process.exit(1);
      }
      throw err;
    }

    const confirmed = await confirm(
      `Are you sure you want to delete tenant "${name}"? This action cannot be undone.`
    );
    if (!confirmed) {
      console.log("Aborted.");
      process.exit(0);
    }
  }

  console.log(`Deleting tenant "${name}"...`);

  try {
    await api.deleteTenant(name);
    console.log(`✓ Deleted tenant "${name}"`);
  } catch (err) {
    console.error("Failed to delete tenant:", err instanceof ApiError ? err.format() : err);
    process.exit(1);
  }
}

/**
 * Sync a tenant to the latest template version.
 */
async function syncTenant(name: string): Promise<void> {
  const config = await loadConfig();
  const api = new ApiClient(config);

  console.log(`Syncing tenant "${name}"...`);

  try {
    const result = await api.syncTenant(name);
    console.log(`✓ Synced tenant "${name}" from v${result.fromVersion} to v${result.toVersion}`);
  } catch (err) {
    if (err instanceof ApiError && err.code === "TENANT_IN_SYNC") {
      console.log(`✓ Tenant "${name}" is already at the latest version.`);
      return;
    }
    console.error("Failed to sync tenant:", err instanceof ApiError ? err.format() : err);
    process.exit(1);
  }
}

// Main tenants command with subcommands
export const tenantsCommand = new Command("tenants")
  .description("Manage tenants");

// tenant list
tenantsCommand
  .command("list")
  .alias("ls")
  .description("List all tenants")
  .action(listTenants);

// tenant get <name>
tenantsCommand
  .command("get <name>")
  .description("Get tenant details")
  .action(getTenant);

// tenant create <name> --template <template>
tenantsCommand
  .command("create <name>")
  .description("Create a new tenant")
  .requiredOption("-t, --template <template>", "Template name to use")
  .action((name: string, options: { template: string }) => {
    createTenant(name, options.template);
  });

// tenant delete <name>
tenantsCommand
  .command("delete <name>")
  .alias("rm")
  .description("Delete a tenant")
  .option("-f, --force", "Skip confirmation prompt")
  .action((name: string, options: { force?: boolean }) => {
    deleteTenant(name, options.force ?? false);
  });

// tenant sync <name>
tenantsCommand
  .command("sync <name>")
  .description("Sync tenant to the latest template version")
  .action(syncTenant);
