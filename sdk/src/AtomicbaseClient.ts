import { AtomicbaseQueryBuilder } from "./AtomicbaseQueryBuilder.js";
import { AtomicbaseError } from "./AtomicbaseError.js";
import type { AtomicbaseClientOptions, AtomicbaseBatchResponse } from "./types.js";

/**
 * Atomicbase client for database operations.
 *
 * @example
 * ```ts
 * import { createClient, eq } from '@atomicbase/sdk'
 *
 * const client = createClient({
 *   url: 'http://localhost:8080',
 *   apiKey: 'your-api-key',
 * })
 *
 * // Query data
 * const { data, error } = await client
 *   .from('users')
 *   .select('id', 'name')
 *   .where(eq('status', 'active'))
 *   .orderBy('created_at', 'desc')
 *   .limit(10)
 *
 * // Insert data
 * const { data } = await client
 *   .from('users')
 *   .insert({ name: 'Alice', email: 'alice@example.com' })
 * ```
 */
export class AtomicbaseClient {
  readonly baseUrl: string;
  readonly apiKey?: string;
  readonly headers: Record<string, string>;
  private readonly fetchFn: typeof fetch;

  constructor(options: AtomicbaseClientOptions) {
    this.baseUrl = options.url.replace(/\/$/, "");
    this.apiKey = options.apiKey;
    this.headers = options.headers ?? {};
    this.fetchFn = options.fetch ?? globalThis.fetch.bind(globalThis);
  }

  /**
   * Start a query on a table.
   *
   * @example
   * ```ts
   * const { data } = await client.from('users').select()
   * ```
   */
  from<T = Record<string, unknown>>(table: string): AtomicbaseQueryBuilder<T> {
    return new AtomicbaseQueryBuilder<T>({
      table,
      baseUrl: this.baseUrl,
      apiKey: this.apiKey,
      fetch: this.fetchFn,
      headers: this.headers,
    });
  }

  /**
   * Create a new client with a different tenant.
   * Useful for multi-tenant applications.
   *
   * @example
   * ```ts
   * const tenantClient = client.tenant('acme-corp')
   * const { data } = await tenantClient.from('users').select()
   * ```
   */
  tenant(tenantId: string): AtomicbaseClient & { readonly tenantId: string } {
    const newClient = new AtomicbaseClient({
      url: this.baseUrl,
      apiKey: this.apiKey,
      fetch: this.fetchFn,
      headers: {
        ...this.headers,
        Tenant: tenantId,
      },
    });

    return Object.assign(newClient, { tenantId });
  }

  /**
   * Execute multiple operations in a single atomic transaction.
   * All operations succeed or all fail together.
   *
   * Supports all query modifiers including single(), maybeSingle(), count(), and withCount().
   *
   * @example
   * ```ts
   * import { createClient, eq } from '@atomicbase/sdk'
   *
   * const client = createClient({ url: 'http://localhost:8080' })
   *
   * // Insert multiple users and update a counter atomically
   * const { data, error } = await client.batch([
   *   client.from('users').insert({ name: 'Alice', email: 'alice@example.com' }),
   *   client.from('users').insert({ name: 'Bob', email: 'bob@example.com' }),
   *   client.from('counters').update({ count: 2 }).where(eq('id', 1)),
   * ])
   *
   * // With result modifiers
   * const { data, error } = await client.batch([
   *   client.from('users').select().where(eq('id', 1)).single(),
   *   client.from('users').select().count(),
   *   client.from('posts').select().limit(10).withCount(),
   * ])
   * // results[0] = { id: 1, name: 'Alice' }  (single object, not array)
   * // results[1] = 42  (just the count number)
   * // results[2] = { data: [...], count: 100 }  (data with count)
   * ```
   */
  async batch<T extends unknown[] = unknown[]>(
    queries: AtomicbaseQueryBuilder<unknown>[]
  ): Promise<AtomicbaseBatchResponse<T>> {
    const operations = queries.map((q) => q.toBatchOperation());

    const headers: Record<string, string> = {
      "Content-Type": "application/json",
      ...this.headers,
    };

    if (this.apiKey) {
      headers["Authorization"] = `Bearer ${this.apiKey}`;
    }

    try {
      const response = await this.fetchFn(`${this.baseUrl}/data/batch`, {
        method: "POST",
        headers,
        body: JSON.stringify({ operations }),
      });

      if (!response.ok) {
        const errorBody = await response.json().catch(() => ({}));
        const error = AtomicbaseError.fromResponse(errorBody, response.status);
        return { data: null, error };
      }

      const rawData = await response.json();
      const rawResults = rawData.results as unknown[];

      // Apply client-side post-processing based on resultMode
      const processedResults = rawResults.map((result, index) => {
        const op = operations[index];
        if (!op) return result;

        const resultMode = op.resultMode;

        // Handle count mode - backend returns { data: [...], count: N }
        if (resultMode === "count") {
          const r = result as { count?: number };
          return r.count ?? 0;
        }

        // Handle withCount - return as-is (already has data + count)
        if (resultMode === "withCount") {
          return result;
        }

        // Handle single - extract first item, error if 0 or >1
        if (resultMode === "single") {
          const data = result as unknown[];
          if (!data || data.length === 0) {
            // Return error indicator - in batch we can't throw per-operation
            return { __error: "NOT_FOUND", message: "No rows returned" };
          }
          if (data.length > 1) {
            return { __error: "MULTIPLE_ROWS", message: "Multiple rows returned" };
          }
          return data[0];
        }

        // Handle maybeSingle - extract first item or null
        if (resultMode === "maybeSingle") {
          const data = result as unknown[];
          return data?.[0] ?? null;
        }

        // Default - return as-is
        return result;
      });

      return { data: { results: processedResults as T }, error: null };
    } catch (err) {
      const error = AtomicbaseError.networkError(err);
      return { data: null, error };
    }
  }
}

/**
 * Create an Atomicbase client.
 *
 * @example
 * ```ts
 * const client = createClient({
 *   url: 'http://localhost:8080',
 *   apiKey: 'your-api-key',
 * })
 * ```
 */
export function createClient(options: AtomicbaseClientOptions): AtomicbaseClient {
  return new AtomicbaseClient(options);
}
