import { AtomicbaseQueryBuilder } from "./AtomicbaseQueryBuilder.js";
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
export declare class AtomicbaseClient {
    readonly baseUrl: string;
    readonly apiKey?: string;
    readonly headers: Record<string, string>;
    private readonly fetchFn;
    constructor(options: AtomicbaseClientOptions);
    /**
     * Start a query on a table.
     *
     * @example
     * ```ts
     * const { data } = await client.from('users').select()
     * ```
     */
    from<T = Record<string, unknown>>(table: string): AtomicbaseQueryBuilder<T>;
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
    tenant(tenantId: string): AtomicbaseClient & {
        readonly tenantId: string;
    };
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
    batch<T extends unknown[] = unknown[]>(queries: AtomicbaseQueryBuilder<unknown>[]): Promise<AtomicbaseBatchResponse<T>>;
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
export declare function createClient(options: AtomicbaseClientOptions): AtomicbaseClient;
//# sourceMappingURL=AtomicbaseClient.d.ts.map