"use strict";
var __extends = (this && this.__extends) || (function () {
    var extendStatics = function (d, b) {
        extendStatics = Object.setPrototypeOf ||
            ({ __proto__: [] } instanceof Array && function (d, b) { d.__proto__ = b; }) ||
            function (d, b) { for (var p in b) if (Object.prototype.hasOwnProperty.call(b, p)) d[p] = b[p]; };
        return extendStatics(d, b);
    };
    return function (d, b) {
        if (typeof b !== "function" && b !== null)
            throw new TypeError("Class extends value " + String(b) + " is not a constructor or null");
        extendStatics(d, b);
        function __() { this.constructor = d; }
        d.prototype = b === null ? Object.create(b) : (__.prototype = b.prototype, new __());
    };
})();
Object.defineProperty(exports, "__esModule", { value: true });
exports.AtomicbaseTransformBuilder = void 0;
var AtomicbaseBuilder_js_1 = require("./AtomicbaseBuilder.js");
/**
 * Transform builder that adds query modifiers like order, limit, range, etc.
 * Extends the base builder with result transformation methods.
 */
var AtomicbaseTransformBuilder = /** @class */ (function (_super) {
    __extends(AtomicbaseTransformBuilder, _super);
    function AtomicbaseTransformBuilder() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
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
    AtomicbaseTransformBuilder.prototype.where = function () {
        var _a;
        var conditions = [];
        for (var _i = 0; _i < arguments.length; _i++) {
            conditions[_i] = arguments[_i];
        }
        (_a = this.state.whereConditions).push.apply(_a, conditions);
        return this;
    };
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
    AtomicbaseTransformBuilder.prototype.orderBy = function (column, direction) {
        var _a;
        if (direction === void 0) { direction = "asc"; }
        this.state.orderByClause = (_a = {}, _a[column] = direction, _a);
        return this;
    };
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
    AtomicbaseTransformBuilder.prototype.limit = function (count) {
        this.state.limitValue = count;
        return this;
    };
    /**
     * Skip a number of rows before returning results.
     *
     * @example
     * ```ts
     * .offset(20)
     * ```
     */
    AtomicbaseTransformBuilder.prototype.offset = function (count) {
        this.state.offsetValue = count;
        return this;
    };
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
    AtomicbaseTransformBuilder.prototype.returning = function () {
        var columns = [];
        for (var _i = 0; _i < arguments.length; _i++) {
            columns[_i] = arguments[_i];
        }
        this.state.returningColumns = columns.length > 0 ? columns : ["*"];
        return this;
    };
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
    AtomicbaseTransformBuilder.prototype.single = function () {
        this.state.resultMode = "single";
        // Fetch 2 to detect "multiple rows" error
        if (this.state.limitValue === null) {
            this.state.limitValue = 2;
        }
        return this;
    };
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
    AtomicbaseTransformBuilder.prototype.maybeSingle = function () {
        this.state.resultMode = "maybeSingle";
        if (this.state.limitValue === null) {
            this.state.limitValue = 1;
        }
        return this;
    };
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
    AtomicbaseTransformBuilder.prototype.count = function () {
        this.state.resultMode = "count";
        this.state.countExact = true;
        this.state.limitValue = 0;
        return this;
    };
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
    AtomicbaseTransformBuilder.prototype.withCount = function () {
        this.state.resultMode = "withCount";
        this.state.countExact = true;
        return this;
    };
    return AtomicbaseTransformBuilder;
}(AtomicbaseBuilder_js_1.AtomicbaseBuilder));
exports.AtomicbaseTransformBuilder = AtomicbaseTransformBuilder;
