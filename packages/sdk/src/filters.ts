import type { FilterCondition } from "./types.js";

// =============================================================================
// Column Reference Helper
// =============================================================================

/**
 * Marks a value as a column reference for column-to-column comparisons.
 * Use in WHERE clauses to compare two columns instead of column to value.
 *
 * @example
 * ```ts
 * // Compare column to value (default)
 * .where(eq('status', 'active'))
 *
 * // Compare column to column
 * .where(gt('created_at', col('updated_at')))
 * .where(eq('price', col('discount_price')))
 * ```
 */
export function col(column: string): { __col: string } {
  return { __col: column };
}

// =============================================================================
// Join Condition Functions (for leftJoin/innerJoin)
// =============================================================================

/** Join equality condition: leftColumn = rightColumn */
export function onEq(leftColumn: string, rightColumn: string): FilterCondition {
  return { [leftColumn]: { eq: rightColumn } };
}

/** Join not equal condition: leftColumn != rightColumn */
export function onNeq(leftColumn: string, rightColumn: string): FilterCondition {
  return { [leftColumn]: { neq: rightColumn } };
}

/** Join greater than condition: leftColumn > rightColumn */
export function onGt(leftColumn: string, rightColumn: string): FilterCondition {
  return { [leftColumn]: { gt: rightColumn } };
}

/** Join greater than or equal condition: leftColumn >= rightColumn */
export function onGte(leftColumn: string, rightColumn: string): FilterCondition {
  return { [leftColumn]: { gte: rightColumn } };
}

/** Join less than condition: leftColumn < rightColumn */
export function onLt(leftColumn: string, rightColumn: string): FilterCondition {
  return { [leftColumn]: { lt: rightColumn } };
}

/** Join less than or equal condition: leftColumn <= rightColumn */
export function onLte(leftColumn: string, rightColumn: string): FilterCondition {
  return { [leftColumn]: { lte: rightColumn } };
}

// =============================================================================
// Filter Helper Functions (Drizzle-style) - for WHERE clauses
// =============================================================================

/** Equality condition: column = value */
export function eq(column: string, value: unknown): FilterCondition {
  return { [column]: { eq: value } };
}

/** Not equal condition: column != value */
export function neq(column: string, value: unknown): FilterCondition {
  return { [column]: { neq: value } };
}

/** Greater than condition: column > value */
export function gt(column: string, value: unknown): FilterCondition {
  return { [column]: { gt: value } };
}

/** Greater than or equal condition: column >= value */
export function gte(column: string, value: unknown): FilterCondition {
  return { [column]: { gte: value } };
}

/** Less than condition: column < value */
export function lt(column: string, value: unknown): FilterCondition {
  return { [column]: { lt: value } };
}

/** Less than or equal condition: column <= value */
export function lte(column: string, value: unknown): FilterCondition {
  return { [column]: { lte: value } };
}

/** LIKE condition: column LIKE pattern */
export function like(column: string, pattern: string): FilterCondition {
  return { [column]: { like: pattern } };
}

/** GLOB condition: column GLOB pattern */
export function glob(column: string, pattern: string): FilterCondition {
  return { [column]: { glob: pattern } };
}

/** IN condition: column IN (values) */
export function inList(column: string, values: unknown[]): FilterCondition {
  return { [column]: { in: values } };
}

/** NOT IN condition: column NOT IN (values) */
export function notInList(column: string, values: unknown[]): FilterCondition {
  return { [column]: { not: { in: values } } };
}

/** BETWEEN condition: column BETWEEN min AND max */
export function between(column: string, min: unknown, max: unknown): FilterCondition {
  return { [column]: { between: [min, max] } };
}

/** IS NULL condition */
export function isNull(column: string): FilterCondition {
  return { [column]: { is: null } };
}

/** IS NOT NULL condition */
export function isNotNull(column: string): FilterCondition {
  return { [column]: { not: { is: null } } };
}

/**
 * Full-text search condition.
 * Searches the table's FTS index.
 *
 * @example
 * ```ts
 * // Search all indexed columns
 * .where(fts('hello world'))
 *
 * // Search specific column within the FTS index
 * .where(fts('title', 'hello world'))
 * ```
 */
export function fts(query: string): FilterCondition;
export function fts(column: string, query: string): FilterCondition;
export function fts(columnOrQuery: string, query?: string): FilterCondition {
  if (query === undefined) {
    // fts(query) - search all columns
    return { __fts: { fts: columnOrQuery } };
  }
  // fts(column, query) - search specific column
  return { [columnOrQuery]: { fts: query } };
}

/** Negate a condition */
export function not(condition: FilterCondition): FilterCondition {
  const [column, ops] = Object.entries(condition)[0]!;
  return { [column]: { not: ops } };
}

/** OR condition: (condition1 OR condition2 OR ...) */
export function or(...conditions: FilterCondition[]): FilterCondition {
  return { or: conditions };
}

/** AND condition: (condition1 AND condition2 AND ...) */
export function and(...conditions: FilterCondition[]): FilterCondition {
  return { and: conditions };
}
