import { AtomicbaseQueryBuilder } from "./AtomicbaseQueryBuilder.js";
import type { AtomicbaseClientOptions } from "./types.js";

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
