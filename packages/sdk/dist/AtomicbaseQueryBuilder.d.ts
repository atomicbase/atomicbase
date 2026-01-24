import { AtomicbaseBuilder, type BuilderConfig } from "./AtomicbaseBuilder.js";
import type { SelectColumn, FilterCondition } from "./types.js";
/**
 * Query builder for database operations.
 * Provides select, insert, upsert, update, and delete methods.
 *
 * @example
 * ```ts
 * // Select with fluent filters
 * const { data } = await client
 *   .from('users')
 *   .select('id', 'name')
 *   .eq('status', 'active')
 *   .gt('age', 18)
 *   .orderBy('name')
 *   .limit(10)
 *
 * // Insert
 * const { data } = await client
 *   .from('users')
 *   .insert({ name: 'Alice', email: 'alice@example.com' })
 *
 * // Update with filter
 * const { data } = await client
 *   .from('users')
 *   .update({ status: 'inactive' })
 *   .eq('id', 1)
 * ```
 */
export declare class AtomicbaseQueryBuilder<T = Record<string, unknown>> extends AtomicbaseBuilder<T> {
    constructor(config: BuilderConfig);
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
     *
     * @example
     * ```ts
     * const { data } = await client
     *   .from('users')
     *   .select('id', 'name', 'orders.total')
     *   .leftJoin('orders', onEq('users.id', 'orders.user_id'))
     * ```
     */
    leftJoin(table: string, onConditions: FilterCondition | FilterCondition[], options?: {
        alias?: string;
        flat?: boolean;
    }): this;
    /**
     * Add an INNER JOIN to the query.
     *
     * @example
     * ```ts
     * const { data } = await client
     *   .from('users')
     *   .select('id', 'name', 'orders.total')
     *   .innerJoin('orders', onEq('users.id', 'orders.user_id'))
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
     * const { data } = await client
     *   .from('users')
     *   .upsert({ id: 1, name: 'Alice Updated' })
     * ```
     */
    upsert(data: Partial<T> | Partial<T>[]): AtomicbaseQueryBuilder<{
        rows_affected: number;
    }>;
    /**
     * Update rows matching the filter conditions.
     * Requires a filter to prevent accidental full-table updates.
     *
     * @example
     * ```ts
     * const { data } = await client
     *   .from('users')
     *   .update({ status: 'inactive' })
     *   .eq('id', 1)
     * ```
     */
    update(data: Partial<T>): AtomicbaseQueryBuilder<{
        rows_affected: number;
    }>;
    /**
     * Delete rows matching the filter conditions.
     * Requires a filter to prevent accidental full-table deletes.
     *
     * @example
     * ```ts
     * const { data } = await client
     *   .from('users')
     *   .delete()
     *   .eq('status', 'deleted')
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
     * const { data } = await client
     *   .from('users')
     *   .insert({ id: 1, name: 'Alice' })
     *   .onConflict('ignore')
     * ```
     */
    onConflict(behavior: "ignore"): this;
    protected buildBody(): Record<string, unknown>;
}
//# sourceMappingURL=AtomicbaseQueryBuilder.d.ts.map