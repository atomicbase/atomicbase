import { createClient, type AtomicbaseClient, type TenantClient } from "@atomicbase/sdk";

// Lazy initialization to avoid build-time errors
let _client: AtomicbaseClient | null = null;
let _initialized = false;

function getClient(): AtomicbaseClient {
  if (!_client) {
    if (!process.env.ATOMICBASE_URL) {
      throw new Error("ATOMICBASE_URL environment variable is required");
    }
    _client = createClient({
      url: process.env.ATOMICBASE_URL,
      apiKey: process.env.ATOMICBASE_API_KEY,
    });
  }
  return _client;
}

/**
 * Ensure the primary tenant database exists.
 * Creates it with the "primary" template if it doesn't exist.
 * Safe to call multiple times - only runs once.
 */
export async function ensurePrimaryTenant(): Promise<void> {
  if (_initialized) return;

  const c = getClient();

  // Check if primary tenant exists
  const { error } = await c.tenants.get("primary");

  console.log(error);

  if (error) {
    // Tenant doesn't exist, create it
    if (error.status === 404) {
      const { error: createError } = await c.tenants.create({
        name: "primary",
        template: "primary",
      });

      if (createError) {
        console.error("Failed to create primary tenant:", createError.message);
        throw new Error(`Failed to create primary tenant: ${createError.message}`);
      }

      console.log("Created primary tenant database");
    } else {
      console.error("Failed to check primary tenant:", error.message);
      throw new Error(`Failed to check primary tenant: ${error.message}`);
    }
  }

  _initialized = true;
}

// Export getter functions instead of direct client
export const client = {
  get tenants() {
    return getClient().tenants;
  },
  tenant(tenantId: string): TenantClient {
    return getClient().tenant(tenantId);
  },
};

// Primary database for auth operations
export function getPrimaryDb(): TenantClient {
  return getClient().tenant("primary");
}

// Alias for backwards compatibility
export const primaryDb = {
  from<T = Record<string, unknown>>(table: string) {
    return getPrimaryDb().from<T>(table);
  },
};

// Get user's tenant database for their todos
export function getUserTenant(tenantName: string): TenantClient {
  return getClient().tenant(tenantName);
}
