import { AtomicbaseBuilder } from "./AtomicbaseBuilder.js";
import { AtomicbaseError } from "./AtomicbaseError.js";
import type {
  AtomicbaseResponse,
  AtomicbaseResponseWithCount,
  AtomicbaseSingleResponse,
  AtomicbaseMaybeSingleResponse,
  FilterCondition,
  OrderDirection,
} from "./types.js";

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
  async single(): Promise<AtomicbaseSingleResponse<T extends (infer U)[] ? U : T>> {
    // Fetch 2 to detect "multiple rows" error
    this.state.limitValue = 2;
    const result = await this.execute();

    if (result.error) {
      if (this.shouldThrowOnError) {
        throw result.error;
      }
      return result as AtomicbaseSingleResponse<T extends (infer U)[] ? U : T>;
    }

    const data = result.data as unknown[];
    if (!data || data.length === 0) {
      const error = new AtomicbaseError({
        message: "No rows returned",
        code: "NOT_FOUND",
        status: 404,
        hint: "The query returned no results. Check your filter conditions.",
      });
      if (this.shouldThrowOnError) {
        throw error;
      }
      return { data: null, error };
    }

    if (data.length > 1) {
      const error = new AtomicbaseError({
        message: "Multiple rows returned",
        code: "MULTIPLE_ROWS",
        status: 400,
        hint: "Expected a single row but got multiple. Add more specific filters.",
      });
      if (this.shouldThrowOnError) {
        throw error;
      }
      return { data: null, error };
    }

    return { data: data[0] as T extends (infer U)[] ? U : T, error: null };
  }

  /**
   * Return zero or one row. Returns null if no rows found (not an error).
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
  async maybeSingle(): Promise<AtomicbaseMaybeSingleResponse<T extends (infer U)[] ? U : T>> {
    this.state.limitValue = 1;
    const result = await this.execute();

    if (result.error) {
      if (this.shouldThrowOnError) {
        throw result.error;
      }
      return result as AtomicbaseMaybeSingleResponse<T extends (infer U)[] ? U : T>;
    }

    const data = result.data as unknown[];
    return {
      data: (data?.[0] ?? null) as (T extends (infer U)[] ? U : T) | null,
      error: null,
    };
  }

  // ---------------------------------------------------------------------------
  // Count Methods
  // ---------------------------------------------------------------------------

  /**
   * Return only the count of matching rows.
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
  async count(): Promise<AtomicbaseResponse<number>> {
    this.state.countExact = true;
    this.state.limitValue = 0;
    const { count, error } = await this.executeWithCount();

    if (error) {
      if (this.shouldThrowOnError) {
        throw error;
      }
      return { data: null, error };
    }

    return { data: count ?? 0, error: null };
  }

  /**
   * Return both data and total count.
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
  async withCount(): Promise<AtomicbaseResponseWithCount<T>> {
    this.state.countExact = true;
    const result = await this.executeWithCount();

    if (result.error && this.shouldThrowOnError) {
      throw result.error;
    }

    return result;
  }
}
