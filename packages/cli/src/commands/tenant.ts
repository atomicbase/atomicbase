import { Command } from "commander";
import { loadConfig } from "../config.js";
import { ApiClient, ApiError } from "../api.js";

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
    console.error("Failed to list tenants:", err instanceof Error ? err.message : err);
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
    if (err instanceof ApiError && err.status === 404) {
      console.error(`Tenant "${name}" not found.`);
      console.error("\nUse 'atomicbase tenant list' to see available tenants.");
      process.exit(1);
    }
    console.error("Failed to get tenant:", err instanceof Error ? err.message : err);
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
    if (err instanceof ApiError) {
      if (err.code === "TENANT_EXISTS") {
        console.error(`\n✗ Tenant "${name}" already exists.`);
        process.exit(1);
      }
      if (err.code === "TEMPLATE_NOT_FOUND") {
        console.error(`\n✗ Template "${template}" not found.`);
        console.error("\nUse 'atomicbase push' to create a template first.");
        process.exit(1);
      }
    }
    console.error("\n✗ Failed to create tenant:", err instanceof Error ? err.message : err);
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

    console.log(`Are you sure you want to delete tenant "${name}"?`);
    console.log("This action cannot be undone. Use --force to skip this prompt.");
    process.exit(1);
  }

  console.log(`Deleting tenant "${name}"...`);

  try {
    await api.deleteTenant(name);
    console.log(`✓ Deleted tenant "${name}"`);
  } catch (err) {
    if (err instanceof ApiError && err.status === 404) {
      console.error(`Tenant "${name}" not found.`);
      process.exit(1);
    }
    console.error("Failed to delete tenant:", err instanceof Error ? err.message : err);
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
    if (err instanceof ApiError) {
      if (err.status === 404) {
        console.error(`Tenant "${name}" not found.`);
        process.exit(1);
      }
      if (err.code === "TENANT_IN_SYNC") {
        console.log(`✓ Tenant "${name}" is already at the latest version.`);
        return;
      }
    }
    console.error("Failed to sync tenant:", err instanceof Error ? err.message : err);
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
