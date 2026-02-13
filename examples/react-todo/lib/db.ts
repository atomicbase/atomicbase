import {
  createClient,
  type AtomicbaseClient,
  type DatabaseClient,
} from "@atomicbase/sdk";

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
 * Ensure the primary database exists.
 * Creates it with the "primary" template if it doesn't exist.
 * Safe to call multiple times - only runs once.
 */
export async function ensurePrimaryDatabase(): Promise<void> {
  if (_initialized) return;

  const c = getClient();

  // Check if primary database exists
  const { error } = await c.databases.get("primary");

  if (error) {
    // Database doesn't exist, create it
    if (error.status === 404) {
      const { error: createError } = await c.databases.create({
        name: "primary",
        template: "primary",
      });

      if (createError) {
        console.error(
          "Failed to create primary database:",
          createError.message,
        );
        throw new Error(
          `Failed to create primary database: ${createError.message}`,
        );
      }

      console.log("Created primary database");
    } else {
      console.error("Failed to check primary database:", error.message);
      throw new Error(`Failed to check primary database: ${error.message}`);
    }
  }

  _initialized = true;
}

// Export getter functions instead of direct client
export const client = {
  get databases() {
    return getClient().databases;
  },
  database(databaseId: string): DatabaseClient {
    return getClient().database(databaseId);
  },
};

// Primary database for auth operations
export function getPrimaryDb(): DatabaseClient {
  return getClient().database("primary");
}

// Alias for backwards compatibility
export const primaryDb = {
  from<T = Record<string, unknown>>(table: string) {
    return getPrimaryDb().from<T>(table);
  },
};

// Get user's database for their todos
export function getUserDatabase(databaseName: string): DatabaseClient {
  return getClient().database(databaseName);
}
