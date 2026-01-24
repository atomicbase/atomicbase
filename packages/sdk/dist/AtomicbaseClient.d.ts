import { AtomicbaseQueryBuilder } from "./AtomicbaseQueryBuilder.js";
import { AtomicbaseBuilder } from "./AtomicbaseBuilder.js";
import type { AtomicbaseClientOptions, AtomicbaseBatchResponse } from "./types.js";
/**
 * Tenant-scoped client for database operations.
 * Created by calling `client.tenant('tenant-name')`.
 *
 * @example
 * ```ts
 * const tenantClient = client.tenant('acme-corp')
 *
 * // Query with fluent filters
 * const { data, error } = await tenantClient
 *   .from('users')
 *   .select('id', 'name')
 *   .eq('status', 'active')
 *   .limit(10)
 *
 * // Insert data
 * const { data } = await tenantClient
 *   .from('users')
 *   .insert({ name: 'Alice', email: 'alice@example.com' })
 * ```
 */
export declare class TenantClient {
    readonly baseUrl: string;
    readonly apiKey?: string;
    readonly headers: Record<string, string>;
    readonly tenantId: string;
    private readonly fetchFn;
    constructor(options: AtomicbaseClientOptions & {
        tenantId: string;
    });
    /**
     * Start a query on a table.
     *
     * @example
     * ```ts
     * const { data } = await tenantClient.from('users').select()
     * ```
     */
    from<T = Record<string, unknown>>(table: string): AtomicbaseQueryBuilder<T>;
    /**
     * Execute multiple operations in a single atomic transaction.
     * All operations succeed or all fail together.
     *
     * @example
     * ```ts
     * const { data, error } = await tenantClient.batch([
     *   tenantClient.from('users').insert({ name: 'Alice' }),
     *   tenantClient.from('users').insert({ name: 'Bob' }),
     *   tenantClient.from('counters').update({ count: 2 }).eq('id', 1),
     * ])
     * ```
     */
    batch<T extends unknown[] = unknown[]>(queries: AtomicbaseBuilder<unknown>[]): Promise<AtomicbaseBatchResponse<T>>;
}
/**
 * Atomicbase client for multi-tenant database operations.
 * Use `.tenant()` to get a tenant-scoped client for querying.
 *
 * @example
 * ```ts
 * import { createClient } from '@atomicbase/sdk'
 *
 * const client = createClient({
 *   url: 'http://localhost:8080',
 *   apiKey: 'your-api-key',
 * })
 *
 * // Get a tenant-scoped client
 * const acme = client.tenant('acme-corp')
 *
 * // Query the tenant's database
 * const { data, error } = await acme
 *   .from('users')
 *   .select('id', 'name')
 *   .eq('status', 'active')
 *   .limit(10)
 * ```
 */
export declare class AtomicbaseClient {
    readonly baseUrl: string;
    readonly apiKey?: string;
    readonly headers: Record<string, string>;
    private readonly fetchFn;
    constructor(options: AtomicbaseClientOptions);
    /**
     * Create a tenant-scoped client for database operations.
     *
     * @example
     * ```ts
     * const tenantClient = client.tenant('acme-corp')
     * const { data } = await tenantClient.from('users').select()
     * ```
     */
    tenant(tenantId: string): TenantClient;
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
 *
 * // Get a tenant client and query
 * const { data } = await client.tenant('my-tenant').from('users').select()
 * ```
 */
export declare function createClient(options: AtomicbaseClientOptions): AtomicbaseClient;
//# sourceMappingURL=AtomicbaseClient.d.ts.map