import { AtomicbaseBuilder, type BuilderConfig } from "./AtomicbaseBuilder.js";
import type { SelectColumn, JoinClause, FilterCondition } from "./types.js";

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
export class AtomicbaseQueryBuilder<T = Record<string, unknown>> extends AtomicbaseBuilder<T> {
  constructor(config: BuilderConfig) {
    super(config);
  }

  // ===========================================================================
  // Query Operations
  // ===========================================================================

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
  select(...columns: SelectColumn[]): AtomicbaseQueryBuilder<T[]> {
    this.state.operation = "select";
    this.state.select = columns.length > 0 ? columns : ["*"];
    return this as unknown as AtomicbaseQueryBuilder<T[]>;
  }

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
  leftJoin(
    table: string,
    onConditions: FilterCondition | FilterCondition[],
    options?: { alias?: string; flat?: boolean }
  ): this {
    const joinClause: JoinClause = {
      table,
      type: "left",
      on: Array.isArray(onConditions) ? onConditions : [onConditions],
      alias: options?.alias,
      flat: options?.flat,
    };
    this.state.joins.push(joinClause);
    return this;
  }

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
  innerJoin(
    table: string,
    onConditions: FilterCondition | FilterCondition[],
    options?: { alias?: string; flat?: boolean }
  ): this {
    const joinClause: JoinClause = {
      table,
      type: "inner",
      on: Array.isArray(onConditions) ? onConditions : [onConditions],
      alias: options?.alias,
      flat: options?.flat,
    };
    this.state.joins.push(joinClause);
    return this;
  }

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
  insert(data: Partial<T> | Partial<T>[]): AtomicbaseQueryBuilder<{ last_insert_id: number }> {
    this.state.operation = "insert";
    this.state.data = Array.isArray(data) ? data : [data];
    return this as unknown as AtomicbaseQueryBuilder<{ last_insert_id: number }>;
  }

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
  upsert(data: Partial<T> | Partial<T>[]): AtomicbaseQueryBuilder<{ rows_affected: number }> {
    this.state.operation = "upsert";
    this.state.data = Array.isArray(data) ? data : [data];
    return this as unknown as AtomicbaseQueryBuilder<{ rows_affected: number }>;
  }

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
  update(data: Partial<T>): AtomicbaseQueryBuilder<{ rows_affected: number }> {
    this.state.operation = "update";
    this.state.data = data;
    return this as unknown as AtomicbaseQueryBuilder<{ rows_affected: number }>;
  }

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
  delete(): AtomicbaseQueryBuilder<{ rows_affected: number }> {
    this.state.operation = "delete";
    return this as unknown as AtomicbaseQueryBuilder<{ rows_affected: number }>;
  }

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
  onConflict(behavior: "ignore"): this {
    this.state.onConflict = behavior;
    return this;
  }

  // ===========================================================================
  // Internal: Request Body Building
  // ===========================================================================

  protected buildBody(): Record<string, unknown> {
    const body: Record<string, unknown> = {};
    const { operation, select, joins, where, order, limit, offset, data, returning } = this.state;

    switch (operation) {
      case "select":
        body.select = select;
        if (joins.length > 0) body.join = joins;
        if (where.length > 0) body.where = where;
        if (order) body.order = order;
        if (limit !== null) body.limit = limit;
        if (offset !== null) body.offset = offset;
        break;

      case "insert":
      case "upsert":
        body.data = data;
        if (returning.length > 0) body.returning = returning;
        break;

      case "update":
        body.data = data;
        if (where.length > 0) body.where = where;
        if (returning.length > 0) body.returning = returning;
        break;

      case "delete":
        if (where.length > 0) body.where = where;
        if (returning.length > 0) body.returning = returning;
        break;
    }

    return body;
  }
}
