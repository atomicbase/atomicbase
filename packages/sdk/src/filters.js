"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.col = col;
exports.onEq = onEq;
exports.onNeq = onNeq;
exports.onGt = onGt;
exports.onGte = onGte;
exports.onLt = onLt;
exports.onLte = onLte;
exports.eq = eq;
exports.neq = neq;
exports.gt = gt;
exports.gte = gte;
exports.lt = lt;
exports.lte = lte;
exports.like = like;
exports.glob = glob;
exports.inArray = inArray;
exports.notInArray = notInArray;
exports.between = between;
exports.isNull = isNull;
exports.isNotNull = isNotNull;
exports.fts = fts;
exports.not = not;
exports.or = or;
exports.and = and;
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
function col(column) {
    return { __col: column };
}
// =============================================================================
// Join Condition Functions (for leftJoin/innerJoin)
// =============================================================================
/** Join equality condition: leftColumn = rightColumn */
function onEq(leftColumn, rightColumn) {
    var _a;
    return _a = {}, _a[leftColumn] = { eq: rightColumn }, _a;
}
/** Join not equal condition: leftColumn != rightColumn */
function onNeq(leftColumn, rightColumn) {
    var _a;
    return _a = {}, _a[leftColumn] = { neq: rightColumn }, _a;
}
/** Join greater than condition: leftColumn > rightColumn */
function onGt(leftColumn, rightColumn) {
    var _a;
    return _a = {}, _a[leftColumn] = { gt: rightColumn }, _a;
}
/** Join greater than or equal condition: leftColumn >= rightColumn */
function onGte(leftColumn, rightColumn) {
    var _a;
    return _a = {}, _a[leftColumn] = { gte: rightColumn }, _a;
}
/** Join less than condition: leftColumn < rightColumn */
function onLt(leftColumn, rightColumn) {
    var _a;
    return _a = {}, _a[leftColumn] = { lt: rightColumn }, _a;
}
/** Join less than or equal condition: leftColumn <= rightColumn */
function onLte(leftColumn, rightColumn) {
    var _a;
    return _a = {}, _a[leftColumn] = { lte: rightColumn }, _a;
}
// =============================================================================
// Filter Helper Functions (Drizzle-style) - for WHERE clauses
// =============================================================================
/** Equality condition: column = value */
function eq(column, value) {
    var _a;
    return _a = {}, _a[column] = { eq: value }, _a;
}
/** Not equal condition: column != value */
function neq(column, value) {
    var _a;
    return _a = {}, _a[column] = { neq: value }, _a;
}
/** Greater than condition: column > value */
function gt(column, value) {
    var _a;
    return _a = {}, _a[column] = { gt: value }, _a;
}
/** Greater than or equal condition: column >= value */
function gte(column, value) {
    var _a;
    return _a = {}, _a[column] = { gte: value }, _a;
}
/** Less than condition: column < value */
function lt(column, value) {
    var _a;
    return _a = {}, _a[column] = { lt: value }, _a;
}
/** Less than or equal condition: column <= value */
function lte(column, value) {
    var _a;
    return _a = {}, _a[column] = { lte: value }, _a;
}
/** LIKE condition: column LIKE pattern */
function like(column, pattern) {
    var _a;
    return _a = {}, _a[column] = { like: pattern }, _a;
}
/** GLOB condition: column GLOB pattern */
function glob(column, pattern) {
    var _a;
    return _a = {}, _a[column] = { glob: pattern }, _a;
}
/** IN condition: column IN (values) */
function inArray(column, values) {
    var _a;
    return _a = {}, _a[column] = { in: values }, _a;
}
/** NOT IN condition: column NOT IN (values) */
function notInArray(column, values) {
    var _a;
    return _a = {}, _a[column] = { not: { in: values } }, _a;
}
/** BETWEEN condition: column BETWEEN min AND max */
function between(column, min, max) {
    var _a;
    return _a = {}, _a[column] = { between: [min, max] }, _a;
}
/** IS NULL condition */
function isNull(column) {
    var _a;
    return _a = {}, _a[column] = { is: null }, _a;
}
/** IS NOT NULL condition */
function isNotNull(column) {
    var _a;
    return _a = {}, _a[column] = { not: { is: null } }, _a;
}
function fts(columnOrQuery, query) {
    var _a;
    if (query === undefined) {
        // fts(query) - search all columns
        return { __fts: { fts: columnOrQuery } };
    }
    // fts(column, query) - search specific column
    return _a = {}, _a[columnOrQuery] = { fts: query }, _a;
}
/** Negate a condition */
function not(condition) {
    var _a;
    var _b = Object.entries(condition)[0], column = _b[0], ops = _b[1];
    return _a = {}, _a[column] = { not: ops }, _a;
}
/** OR condition: (condition1 OR condition2 OR ...) */
function or() {
    var conditions = [];
    for (var _i = 0; _i < arguments.length; _i++) {
        conditions[_i] = arguments[_i];
    }
    return { or: conditions };
}
/** AND condition: (condition1 AND condition2 AND ...) */
function and() {
    var conditions = [];
    for (var _i = 0; _i < arguments.length; _i++) {
        conditions[_i] = arguments[_i];
    }
    return { and: conditions };
}
