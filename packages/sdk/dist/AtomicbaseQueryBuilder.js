import { AtomicbaseTransformBuilder } from "./AtomicbaseTransformBuilder.js";
/**
 * Query builder for database operations.
 * Provides select, insert, upsert, update, and delete methods.
 */
export class AtomicbaseQueryBuilder extends AtomicbaseTransformBuilder {
    constructor(config) {
        const state = {
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
        super({
            state,
            baseUrl: config.baseUrl,
            apiKey: config.apiKey,
            tenant: config.tenant,
            fetch: config.fetch,
            headers: config.headers,
        });
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
    select(...columns) {
        this.state.operation = "select";
        this.state.selectColumns = columns.length > 0 ? columns : ["*"];
        return this;
    }
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
    leftJoin(table, onConditions, options) {
        const joinClause = {
            table,
            type: "left",
            on: Array.isArray(onConditions) ? onConditions : [onConditions],
            alias: options?.alias,
            flat: options?.flat,
        };
        this.state.joinClauses.push(joinClause);
        return this;
    }
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
    innerJoin(table, onConditions, options) {
        const joinClause = {
            table,
            type: "inner",
            on: Array.isArray(onConditions) ? onConditions : [onConditions],
            alias: options?.alias,
            flat: options?.flat,
        };
        this.state.joinClauses.push(joinClause);
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
    insert(data) {
        this.state.operation = "insert";
        this.state.data = data;
        return this;
    }
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
    upsert(data) {
        this.state.operation = "upsert";
        this.state.data = Array.isArray(data) ? data : [data];
        return this;
    }
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
    update(data) {
        this.state.operation = "update";
        this.state.data = data;
        return this;
    }
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
    delete() {
        this.state.operation = "delete";
        return this;
    }
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
    onConflict(behavior) {
        this.state.onConflictBehavior = behavior;
        return this;
    }
    // ---------------------------------------------------------------------------
    // Batch Operation Export
    // ---------------------------------------------------------------------------
    /**
     * Export this query as a batch operation for use with client.batch().
     * This allows combining multiple queries into a single atomic transaction.
     *
     * @internal
     */
    toBatchOperation() {
        if (!this.state.operation) {
            throw new Error("No operation specified. Call select(), insert(), update(), or delete() first.");
        }
        const operation = this.state.operation;
        const body = this.buildBody();
        const result = {
            operation,
            table: this.state.table,
            body,
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
    }
    // ---------------------------------------------------------------------------
    // Request Building
    // ---------------------------------------------------------------------------
    /**
     * Build the request body based on operation type.
     */
    buildBody() {
        const body = {};
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
    }
    buildRequest() {
        const url = `${this.baseUrl}/data/query/${encodeURIComponent(this.state.table)}`;
        const headers = this.buildCommonHeaders();
        // Build Prefer header
        const preferParts = [];
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
        return { url, headers, body: this.buildBody() };
    }
}
//# sourceMappingURL=AtomicbaseQueryBuilder.js.map