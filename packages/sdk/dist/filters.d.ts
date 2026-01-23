import type { FilterCondition } from "./types.js";
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
export declare function col(column: string): {
    __col: string;
};
/** Join equality condition: leftColumn = rightColumn */
export declare function onEq(leftColumn: string, rightColumn: string): FilterCondition;
/** Join not equal condition: leftColumn != rightColumn */
export declare function onNeq(leftColumn: string, rightColumn: string): FilterCondition;
/** Join greater than condition: leftColumn > rightColumn */
export declare function onGt(leftColumn: string, rightColumn: string): FilterCondition;
/** Join greater than or equal condition: leftColumn >= rightColumn */
export declare function onGte(leftColumn: string, rightColumn: string): FilterCondition;
/** Join less than condition: leftColumn < rightColumn */
export declare function onLt(leftColumn: string, rightColumn: string): FilterCondition;
/** Join less than or equal condition: leftColumn <= rightColumn */
export declare function onLte(leftColumn: string, rightColumn: string): FilterCondition;
/** Equality condition: column = value */
export declare function eq(column: string, value: unknown): FilterCondition;
/** Not equal condition: column != value */
export declare function neq(column: string, value: unknown): FilterCondition;
/** Greater than condition: column > value */
export declare function gt(column: string, value: unknown): FilterCondition;
/** Greater than or equal condition: column >= value */
export declare function gte(column: string, value: unknown): FilterCondition;
/** Less than condition: column < value */
export declare function lt(column: string, value: unknown): FilterCondition;
/** Less than or equal condition: column <= value */
export declare function lte(column: string, value: unknown): FilterCondition;
/** LIKE condition: column LIKE pattern */
export declare function like(column: string, pattern: string): FilterCondition;
/** GLOB condition: column GLOB pattern */
export declare function glob(column: string, pattern: string): FilterCondition;
/** IN condition: column IN (values) */
export declare function inArray(column: string, values: unknown[]): FilterCondition;
/** NOT IN condition: column NOT IN (values) */
export declare function notInArray(column: string, values: unknown[]): FilterCondition;
/** BETWEEN condition: column BETWEEN min AND max */
export declare function between(column: string, min: unknown, max: unknown): FilterCondition;
/** IS NULL condition */
export declare function isNull(column: string): FilterCondition;
/** IS NOT NULL condition */
export declare function isNotNull(column: string): FilterCondition;
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
export declare function fts(query: string): FilterCondition;
export declare function fts(column: string, query: string): FilterCondition;
/** Negate a condition */
export declare function not(condition: FilterCondition): FilterCondition;
/** OR condition: (condition1 OR condition2 OR ...) */
export declare function or(...conditions: FilterCondition[]): FilterCondition;
/** AND condition: (condition1 AND condition2 AND ...) */
export declare function and(...conditions: FilterCondition[]): FilterCondition;
//# sourceMappingURL=filters.d.ts.map