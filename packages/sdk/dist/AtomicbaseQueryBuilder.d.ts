import { AtomicbaseTransformBuilder } from "./AtomicbaseTransformBuilder.js";
import type { SelectColumn, FilterCondition } from "./types.js";
/**
 * Query builder for database operations.
 * Provides select, insert, upsert, update, and delete methods.
 */
export declare class AtomicbaseQueryBuilder<T = Record<string, unknown>> extends AtomicbaseTransformBuilder<T> {
    constructor(config: {
        table: string;
        baseUrl: string;
        apiKey?: string;
        tenant?: string;
        fetch: typeof fetch;
        headers?: Record<string, string>;
    });
    /**
     * Select rows from the table.
     *
     * @example
     * ```ts
     * // Select all columns
     * const { data } = await client.from('users').select()
     *
     * // Select specific columns
     * const { data } = await client.from('users').select('id', 'name', 'email')
     *
     * // Select with nested relations (implicit joins)
     * const { data } = await client.from('users').select('id', 'name', { posts: ['title', 'body'] })
     * ```
     */
    select(...columns: SelectColumn[]): AtomicbaseQueryBuilder<T[]>;
    /**
     * Add a LEFT JOIN to the query.
     * Use this for explicit joins where FK relationships don't exist or custom conditions are needed.
     * Returns all rows from the base table, with NULL for non-matching joined rows.
     *
     * @example
     * ```ts
     * // Basic left join with nested output (default)
     * const { data } = await client
     *   .from('users')
     *   .select('id', 'name', 'orders.total')
     *   .leftJoin('orders', onEq('users.id', 'orders.user_id'))
     *
     * // Left join with flat output
     * const { data } = await client
     *   .from('users')
     *   .select('id', 'name', 'orders.total')
     *   .leftJoin('orders', onEq('users.id', 'orders.user_id'), { flat: true })
     *
     * // Multiple joins
     * const { data } = await client
     *   .from('users')
     *   .select('users.id', 'users.name', 'orders.total', 'products.name')
     *   .leftJoin('orders', onEq('users.id', 'orders.user_id'))
     *   .leftJoin('products', onEq('orders.product_id', 'products.id'))
     *
     * // Multiple conditions (AND)
     * const { data } = await client
     *   .from('users')
     *   .select('id', 'name', 'orders.total')
     *   .leftJoin('orders', [onEq('users.id', 'orders.user_id'), onEq('users.tenant_id', 'orders.tenant_id')])
     * ```
     */
    leftJoin(table: string, onConditions: FilterCondition | FilterCondition[], options?: {
        alias?: string;
        flat?: boolean;
    }): this;
    /**
     * Add an INNER JOIN to the query.
     * Use this for explicit joins where FK relationships don't exist or custom conditions are needed.
     * Returns only rows that have matches in both tables.
     *
     * @example
     * ```ts
     * // Inner join - only users with orders
     * const { data } = await client
     *   .from('users')
     *   .select('id', 'name', 'orders.total')
     *   .innerJoin('orders', onEq('users.id', 'orders.user_id'))
     *
     * // Inner join with flat output
     * const { data } = await client
     *   .from('users')
     *   .select('id', 'name', 'orders.total')
     *   .innerJoin('orders', onEq('users.id', 'orders.user_id'), { flat: true })
     * ```
     */
    innerJoin(table: string, onConditions: FilterCondition | FilterCondition[], options?: {
        alias?: string;
        flat?: boolean;
    }): this;
    /**
     * Insert one or more rows.
     *
     * @example
     * ```ts
     * // Insert single row
     * const { data } = await client
     *   .from('users')
     *   .insert({ name: 'Alice', email: 'alice@example.com' })
     *
     * // Insert multiple rows
     * const { data } = await client
     *   .from('users')
     *   .insert([
     *     { name: 'Alice', email: 'alice@example.com' },
     *     { name: 'Bob', email: 'bob@example.com' },
     *   ])
     *
     * // Insert with returning
     * const { data } = await client
     *   .from('users')
     *   .insert({ name: 'Alice' })
     *   .returning('id', 'created_at')
     * ```
     */
    insert(data: Partial<T> | Partial<T>[]): AtomicbaseQueryBuilder<{
        last_insert_id: number;
    }>;
    /**
     * Upsert (insert or update) one or more rows.
     * Uses the primary key to detect conflicts.
     *
     * @example
     * ```ts
     * // Upsert single row
     * const { data } = await client
     *   .from('users')
     *   .upsert({ id: 1, name: 'Alice Updated' })
     *
     * // Upsert multiple rows
     * const { data } = await client
     *   .from('users')
     *   .upsert([
     *     { id: 1, name: 'Alice Updated' },
     *     { id: 2, name: 'Bob Updated' },
     *   ])
     * ```
     */
    upsert(data: Partial<T> | Partial<T>[]): AtomicbaseQueryBuilder<{
        rows_affected: number;
    }>;
    /**
     * Update rows matching the filter conditions.
     * Requires a where() clause to prevent accidental full-table updates.
     *
     * @example
     * ```ts
     * const { data } = await client
     *   .from('users')
     *   .update({ status: 'inactive' })
     *   .where(eq('last_login', null))
     * ```
     */
    update(data: Partial<T>): AtomicbaseQueryBuilder<{
        rows_affected: number;
    }>;
    /**
     * Delete rows matching the filter conditions.
     * Requires a where() clause to prevent accidental full-table deletes.
     *
     * @example
     * ```ts
     * const { data } = await client
     *   .from('users')
     *   .delete()
     *   .where(eq('status', 'deleted'))
     * ```
     */
    delete(): AtomicbaseQueryBuilder<{
        rows_affected: number;
    }>;
    /**
     * Set conflict handling behavior for insert operations.
     *
     * @example
     * ```ts
     * // Ignore conflicts (INSERT OR IGNORE)
     * const { data } = await client
     *   .from('users')
     *   .insert({ id: 1, name: 'Alice' })
     *   .onConflict('ignore')
     * ```
     */
    onConflict(behavior: "ignore"): this;
    /**
     * Export this query as a batch operation for use with client.batch().
     * This allows combining multiple queries into a single atomic transaction.
     *
     * @internal
     */
    toBatchOperation(): {
        operation: string;
        table: string;
        body: Record<string, unknown>;
        count?: boolean;
        resultMode?: string;
    };
    /**
     * Build the request body based on operation type.
     */
    private buildBody;
    protected buildRequest(): {
        url: string;
        headers: Record<string, string>;
        body: Record<string, unknown>;
    };
}
//# sourceMappingURL=AtomicbaseQueryBuilder.d.ts.map