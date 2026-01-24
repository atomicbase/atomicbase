import { AtomicbaseQueryBuilder } from "./AtomicbaseQueryBuilder.js";
import { AtomicbaseError } from "./AtomicbaseError.js";
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
export class TenantClient {
    baseUrl;
    apiKey;
    headers;
    tenantId;
    fetchFn;
    constructor(options) {
        this.baseUrl = options.url.replace(/\/$/, "");
        this.apiKey = options.apiKey;
        this.headers = options.headers ?? {};
        this.tenantId = options.tenantId;
        this.fetchFn = options.fetch ?? globalThis.fetch.bind(globalThis);
    }
    /**
     * Start a query on a table.
     *
     * @example
     * ```ts
     * const { data } = await tenantClient.from('users').select()
     * ```
     */
    from(table) {
        return new AtomicbaseQueryBuilder({
            table,
            baseUrl: this.baseUrl,
            apiKey: this.apiKey,
            fetch: this.fetchFn,
            headers: {
                ...this.headers,
                Tenant: this.tenantId,
            },
        });
    }
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
    async batch(queries) {
        const operations = queries.map((q) => q.toBatchOperation());
        const headers = {
            "Content-Type": "application/json",
            ...this.headers,
            Tenant: this.tenantId,
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
            const rawResults = rawData.results;
            // Apply client-side post-processing based on resultMode
            const processedResults = rawResults.map((result, index) => {
                const op = operations[index];
                if (!op)
                    return result;
                switch (op.resultMode) {
                    case "count": {
                        const r = result;
                        return r.count ?? 0;
                    }
                    case "withCount":
                        return result;
                    case "single": {
                        const data = result;
                        if (!data || data.length === 0) {
                            return { __error: "NOT_FOUND", message: "No rows returned" };
                        }
                        if (data.length > 1) {
                            return { __error: "MULTIPLE_ROWS", message: "Multiple rows returned" };
                        }
                        return data[0];
                    }
                    case "maybeSingle": {
                        const data = result;
                        return data?.[0] ?? null;
                    }
                    default:
                        return result;
                }
            });
            return { data: { results: processedResults }, error: null };
        }
        catch (err) {
            const error = AtomicbaseError.networkError(err);
            return { data: null, error };
        }
    }
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
export class AtomicbaseClient {
    baseUrl;
    apiKey;
    headers;
    fetchFn;
    constructor(options) {
        this.baseUrl = options.url.replace(/\/$/, "");
        this.apiKey = options.apiKey;
        this.headers = options.headers ?? {};
        this.fetchFn = options.fetch ?? globalThis.fetch.bind(globalThis);
    }
    /**
     * Create a tenant-scoped client for database operations.
     *
     * @example
     * ```ts
     * const tenantClient = client.tenant('acme-corp')
     * const { data } = await tenantClient.from('users').select()
     * ```
     */
    tenant(tenantId) {
        return new TenantClient({
            url: this.baseUrl,
            apiKey: this.apiKey,
            fetch: this.fetchFn,
            headers: this.headers,
            tenantId,
        });
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
 *
 * // Get a tenant client and query
 * const { data } = await client.tenant('my-tenant').from('users').select()
 * ```
 */
export function createClient(options) {
    return new AtomicbaseClient(options);
}
//# sourceMappingURL=AtomicbaseClient.js.map