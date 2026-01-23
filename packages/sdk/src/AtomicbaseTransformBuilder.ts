import { AtomicbaseBuilder } from "./AtomicbaseBuilder.js";
import type { FilterCondition, OrderDirection } from "./types.js";

/**
 * Transform builder that adds query modifiers like order, limit, range, etc.
 * Extends the base builder with result transformation methods.
 */
export abstract class AtomicbaseTransformBuilder<T> extends AtomicbaseBuilder<T> {
  // ---------------------------------------------------------------------------
  // Filtering
  // ---------------------------------------------------------------------------

  /**
   * Add filter conditions to the query.
   *
   * @example
   * ```ts
   * // Single condition
   * .where(eq('status', 'active'))
   *
   * // Multiple conditions (AND)
   * .where(eq('status', 'active'), gt('age', 18))
   *
   * // OR conditions
   * .where(or(eq('role', 'admin'), eq('role', 'moderator')))
   * ```
   */
  where(...conditions: FilterCondition[]): this {
    this.state.whereConditions.push(...conditions);
    return this;
  }

  // ---------------------------------------------------------------------------
  // Ordering
  // ---------------------------------------------------------------------------

  /**
   * Order the results by a column.
   *
   * @example
   * ```ts
   * .orderBy('created_at', 'desc')
   * .orderBy('name')  // defaults to 'asc'
   * ```
   */
  orderBy(column: string, direction: OrderDirection = "asc"): this {
    this.state.orderByClause = { [column]: direction };
    return this;
  }

  // ---------------------------------------------------------------------------
  // Pagination
  // ---------------------------------------------------------------------------

  /**
   * Limit the number of rows returned.
   *
   * @example
   * ```ts
   * .limit(10)
   * ```
   */
  limit(count: number): this {
    this.state.limitValue = count;
    return this;
  }

  /**
   * Skip a number of rows before returning results.
   *
   * @example
   * ```ts
   * .offset(20)
   * ```
   */
  offset(count: number): this {
    this.state.offsetValue = count;
    return this;
  }

  // ---------------------------------------------------------------------------
  // RETURNING clause
  // ---------------------------------------------------------------------------

  /**
   * Specify columns to return after insert/update/delete.
   *
   * @example
   * ```ts
   * .insert({ name: 'Alice' }).returning('id', 'created_at')
   * .insert({ name: 'Alice' }).returning('*')  // all columns
   * ```
   */
  returning(...columns: string[]): this {
    this.state.returningColumns = columns.length > 0 ? columns : ["*"];
    return this;
  }

  // ---------------------------------------------------------------------------
  // Result Modifiers
  // ---------------------------------------------------------------------------

  /**
   * Return a single row. Errors if zero or multiple rows are returned.
   * This is a chainable modifier - the query only executes when awaited.
   *
   * @example
   * ```ts
   * const { data, error } = await client
   *   .from('users')
   *   .select()
   *   .where(eq('id', 1))
   *   .single()
   *
   * // data is a single object, not an array
   * ```
   */
  single(): this {
    this.state.resultMode = "single";
    // Fetch 2 to detect "multiple rows" error
    if (this.state.limitValue === null) {
      this.state.limitValue = 2;
    }
    return this;
  }

  /**
   * Return zero or one row. Returns null if no rows found (not an error).
   * This is a chainable modifier - the query only executes when awaited.
   *
   * @example
   * ```ts
   * const { data, error } = await client
   *   .from('users')
   *   .select()
   *   .where(eq('email', 'maybe@exists.com'))
   *   .maybeSingle()
   *
   * // data is object or null, no error for zero rows
   * ```
   */
  maybeSingle(): this {
    this.state.resultMode = "maybeSingle";
    if (this.state.limitValue === null) {
      this.state.limitValue = 1;
    }
    return this;
  }

  // ---------------------------------------------------------------------------
  // Count Methods
  // ---------------------------------------------------------------------------

  /**
   * Return only the count of matching rows.
   * This is a chainable modifier - the query only executes when awaited.
   *
   * @example
   * ```ts
   * const { data: count, error } = await client
   *   .from('users')
   *   .select()
   *   .where(eq('status', 'active'))
   *   .count()
   *
   * console.log(`${count} active users`)
   * ```
   */
  count(): this {
    this.state.resultMode = "count";
    this.state.countExact = true;
    this.state.limitValue = 0;
    return this;
  }

  /**
   * Return both data and total count.
   * This is a chainable modifier - the query only executes when awaited.
   *
   * @example
   * ```ts
   * const { data, count, error } = await client
   *   .from('users')
   *   .select()
   *   .limit(10)
   *   .withCount()
   *
   * console.log(`Showing ${data.length} of ${count} users`)
   * ```
   */
  withCount(): this {
    this.state.resultMode = "withCount";
    this.state.countExact = true;
    return this;
  }
}
