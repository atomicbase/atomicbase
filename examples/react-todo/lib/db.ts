import { createClient, type AtomicbaseClient, type TenantClient } from "@atomicbase/sdk";

// Lazy initialization to avoid build-time errors
let _client: AtomicbaseClient | null = null;

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
