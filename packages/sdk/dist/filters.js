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
export function col(column) {
    return { __col: column };
}
// =============================================================================
// Join Condition Functions (for leftJoin/innerJoin)
// =============================================================================
/** Join equality condition: leftColumn = rightColumn */
export function onEq(leftColumn, rightColumn) {
    return { [leftColumn]: { eq: rightColumn } };
}
/** Join not equal condition: leftColumn != rightColumn */
export function onNeq(leftColumn, rightColumn) {
    return { [leftColumn]: { neq: rightColumn } };
}
/** Join greater than condition: leftColumn > rightColumn */
export function onGt(leftColumn, rightColumn) {
    return { [leftColumn]: { gt: rightColumn } };
}
/** Join greater than or equal condition: leftColumn >= rightColumn */
export function onGte(leftColumn, rightColumn) {
    return { [leftColumn]: { gte: rightColumn } };
}
/** Join less than condition: leftColumn < rightColumn */
export function onLt(leftColumn, rightColumn) {
    return { [leftColumn]: { lt: rightColumn } };
}
/** Join less than or equal condition: leftColumn <= rightColumn */
export function onLte(leftColumn, rightColumn) {
    return { [leftColumn]: { lte: rightColumn } };
}
// =============================================================================
// Filter Helper Functions (Drizzle-style) - for WHERE clauses
// =============================================================================
/** Equality condition: column = value */
export function eq(column, value) {
    return { [column]: { eq: value } };
}
/** Not equal condition: column != value */
export function neq(column, value) {
    return { [column]: { neq: value } };
}
/** Greater than condition: column > value */
export function gt(column, value) {
    return { [column]: { gt: value } };
}
/** Greater than or equal condition: column >= value */
export function gte(column, value) {
    return { [column]: { gte: value } };
}
/** Less than condition: column < value */
export function lt(column, value) {
    return { [column]: { lt: value } };
}
/** Less than or equal condition: column <= value */
export function lte(column, value) {
    return { [column]: { lte: value } };
}
/** LIKE condition: column LIKE pattern */
export function like(column, pattern) {
    return { [column]: { like: pattern } };
}
/** GLOB condition: column GLOB pattern */
export function glob(column, pattern) {
    return { [column]: { glob: pattern } };
}
/** IN condition: column IN (values) */
export function inArray(column, values) {
    return { [column]: { in: values } };
}
/** NOT IN condition: column NOT IN (values) */
export function notInArray(column, values) {
    return { [column]: { not: { in: values } } };
}
/** BETWEEN condition: column BETWEEN min AND max */
export function between(column, min, max) {
    return { [column]: { between: [min, max] } };
}
/** IS NULL condition */
export function isNull(column) {
    return { [column]: { is: null } };
}
/** IS NOT NULL condition */
export function isNotNull(column) {
    return { [column]: { not: { is: null } } };
}
export function fts(columnOrQuery, query) {
    if (query === undefined) {
        // fts(query) - search all columns
        return { __fts: { fts: columnOrQuery } };
    }
    // fts(column, query) - search specific column
    return { [columnOrQuery]: { fts: query } };
}
/** Negate a condition */
export function not(condition) {
    const [column, ops] = Object.entries(condition)[0];
    return { [column]: { not: ops } };
}
/** OR condition: (condition1 OR condition2 OR ...) */
export function or(...conditions) {
    return { or: conditions };
}
/** AND condition: (condition1 AND condition2 AND ...) */
export function and(...conditions) {
    return { and: conditions };
}
//# sourceMappingURL=filters.js.map