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
exports.AtomicbaseQueryBuilder = void 0;
var AtomicbaseTransformBuilder_js_1 = require("./AtomicbaseTransformBuilder.js");
/**
 * Query builder for database operations.
 * Provides select, insert, upsert, update, and delete methods.
 */
var AtomicbaseQueryBuilder = /** @class */ (function (_super) {
    __extends(AtomicbaseQueryBuilder, _super);
    function AtomicbaseQueryBuilder(config) {
        var state = {
            table: config.table,
            operation: null,
            selectColumns: [],
            joinClauses: [],
            whereConditions: [],
            orderByClause: null,
            limitValue: null,
            offsetValue: null,
            data: null,
            returningColumns: [],
            onConflictBehavior: null,
            countExact: false,
            resultMode: "default",
        };
        return _super.call(this, {
            state: state,
            baseUrl: config.baseUrl,
            apiKey: config.apiKey,
            tenant: config.tenant,
            fetch: config.fetch,
            headers: config.headers,
        }) || this;
    }
    // ---------------------------------------------------------------------------
    // Query Operations
    // ---------------------------------------------------------------------------
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
    AtomicbaseQueryBuilder.prototype.select = function () {
        var columns = [];
        for (var _i = 0; _i < arguments.length; _i++) {
            columns[_i] = arguments[_i];
        }
        this.state.operation = "select";
        this.state.selectColumns = columns.length > 0 ? columns : ["*"];
        return this;
    };
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
    AtomicbaseQueryBuilder.prototype.leftJoin = function (table, onConditions, options) {
        var joinClause = {
            table: table,
            type: "left",
            on: Array.isArray(onConditions) ? onConditions : [onConditions],
            alias: options === null || options === void 0 ? void 0 : options.alias,
            flat: options === null || options === void 0 ? void 0 : options.flat,
        };
        this.state.joinClauses.push(joinClause);
        return this;
    };
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
    AtomicbaseQueryBuilder.prototype.innerJoin = function (table, onConditions, options) {
        var joinClause = {
            table: table,
            type: "inner",
            on: Array.isArray(onConditions) ? onConditions : [onConditions],
            alias: options === null || options === void 0 ? void 0 : options.alias,
            flat: options === null || options === void 0 ? void 0 : options.flat,
        };
        this.state.joinClauses.push(joinClause);
        return this;
    };
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
    AtomicbaseQueryBuilder.prototype.insert = function (data) {
        this.state.operation = "insert";
        this.state.data = data;
        return this;
    };
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
    AtomicbaseQueryBuilder.prototype.upsert = function (data) {
        this.state.operation = "upsert";
        this.state.data = Array.isArray(data) ? data : [data];
        return this;
    };
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
    AtomicbaseQueryBuilder.prototype.update = function (data) {
        this.state.operation = "update";
        this.state.data = data;
        return this;
    };
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
    AtomicbaseQueryBuilder.prototype.delete = function () {
        this.state.operation = "delete";
        return this;
    };
    // ---------------------------------------------------------------------------
    // Conflict Handling
    // ---------------------------------------------------------------------------
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
    AtomicbaseQueryBuilder.prototype.onConflict = function (behavior) {
        this.state.onConflictBehavior = behavior;
        return this;
    };
    // ---------------------------------------------------------------------------
    // Batch Operation Export
    // ---------------------------------------------------------------------------
    /**
     * Export this query as a batch operation for use with client.batch().
     * This allows combining multiple queries into a single atomic transaction.
     *
     * @internal
     */
    AtomicbaseQueryBuilder.prototype.toBatchOperation = function () {
        if (!this.state.operation) {
            throw new Error("No operation specified. Call select(), insert(), update(), or delete() first.");
        }
        var operation = this.state.operation;
        var body = this.buildBody();
        var result = {
            operation: operation,
            table: this.state.table,
            body: body,
        };
        // Include count flag for select operations that need it
        if (this.state.countExact) {
            result.count = true;
        }
        // Include resultMode for client-side post-processing
        if (this.state.resultMode !== "default") {
            result.resultMode = this.state.resultMode;
        }
        return result;
    };
    // ---------------------------------------------------------------------------
    // Request Building
    // ---------------------------------------------------------------------------
    /**
     * Build the request body based on operation type.
     */
    AtomicbaseQueryBuilder.prototype.buildBody = function () {
        var body = {};
        if (this.state.operation === "select") {
            body.select = this.state.selectColumns;
            if (this.state.joinClauses.length > 0) {
                body.join = this.state.joinClauses;
            }
            if (this.state.whereConditions.length > 0) {
                body.where = this.state.whereConditions;
            }
            if (this.state.orderByClause) {
                body.order = this.state.orderByClause;
            }
            if (this.state.limitValue !== null) {
                body.limit = this.state.limitValue;
            }
            if (this.state.offsetValue !== null) {
                body.offset = this.state.offsetValue;
            }
        }
        else if (this.state.operation === "insert" || this.state.operation === "upsert") {
            body.data = this.state.data;
            if (this.state.returningColumns.length > 0) {
                body.returning = this.state.returningColumns;
            }
        }
        else if (this.state.operation === "update") {
            body.data = this.state.data;
            if (this.state.whereConditions.length > 0) {
                body.where = this.state.whereConditions;
            }
            if (this.state.returningColumns.length > 0) {
                body.returning = this.state.returningColumns;
            }
        }
        else if (this.state.operation === "delete") {
            if (this.state.whereConditions.length > 0) {
                body.where = this.state.whereConditions;
            }
            if (this.state.returningColumns.length > 0) {
                body.returning = this.state.returningColumns;
            }
        }
        return body;
    };
    AtomicbaseQueryBuilder.prototype.buildRequest = function () {
        var url = "".concat(this.baseUrl, "/data/query/").concat(encodeURIComponent(this.state.table));
        var headers = this.buildCommonHeaders();
        // Build Prefer header
        var preferParts = [];
        switch (this.state.operation) {
            case "select":
                preferParts.push("operation=select");
                if (this.state.countExact) {
                    preferParts.push("count=exact");
                }
                break;
            case "insert":
                preferParts.push("operation=insert");
                if (this.state.onConflictBehavior === "ignore") {
                    preferParts.push("on-conflict=ignore");
                }
                break;
            case "upsert":
                preferParts.push("operation=insert");
                preferParts.push("on-conflict=replace");
                break;
            case "update":
                preferParts.push("operation=update");
                break;
            case "delete":
                preferParts.push("operation=delete");
                break;
        }
        if (preferParts.length > 0) {
            headers["Prefer"] = preferParts.join(",");
        }
        return { url: url, headers: headers, body: this.buildBody() };
    };
    return AtomicbaseQueryBuilder;
}(AtomicbaseTransformBuilder_js_1.AtomicbaseTransformBuilder));
exports.AtomicbaseQueryBuilder = AtomicbaseQueryBuilder;
