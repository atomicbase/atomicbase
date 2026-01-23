// =============================================================================
// Types
// =============================================================================
// =============================================================================
// Error
// =============================================================================
export class AtomicbaseError extends Error {
    code;
    status;
    hint;
    constructor(message, code, status, hint) {
        super(message);
        this.code = code;
        this.status = status;
        this.hint = hint;
        this.name = "AtomicbaseError";
    }
}
// =============================================================================
// Filter Helper Functions
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
/** Full-text search condition */
export function fts(column, query) {
    return { [column]: { fts: query } };
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
export class QueryBuilder {
    state;
    client;
    constructor(client, table) {
        this.client = client;
        this.state = {
            table,
            operation: null,
            selectColumns: [],
            whereConditions: [],
            orderByClause: null,
            limitValue: null,
            offsetValue: null,
            data: null,
            returningColumns: [],
            onConflictBehavior: null,
            countExact: false,
        };
    }
    // ---------------------------------------------------------------------------
    // Query Methods
    // ---------------------------------------------------------------------------
    select(...columns) {
        this.state.operation = "select";
        this.state.selectColumns = columns.length > 0 ? columns : ["*"];
        return this;
    }
    insert(data) {
        this.state.operation = "insert";
        this.state.data = data;
        return this;
    }
    upsert(data) {
        this.state.operation = "upsert";
        this.state.data = Array.isArray(data) ? data : [data];
        return this;
    }
    update(data) {
        this.state.operation = "update";
        this.state.data = data;
        return this;
    }
    delete() {
        this.state.operation = "delete";
        return this;
    }
    // ---------------------------------------------------------------------------
    // Modifiers
    // ---------------------------------------------------------------------------
    where(...conditions) {
        this.state.whereConditions.push(...conditions);
        return this;
    }
    orderBy(column, direction = "asc") {
        this.state.orderByClause = { [column]: direction };
        return this;
    }
    limit(n) {
        this.state.limitValue = n;
        return this;
    }
    offset(n) {
        this.state.offsetValue = n;
        return this;
    }
    returning(...columns) {
        this.state.returningColumns = columns.length > 0 ? columns : ["*"];
        return this;
    }
    onConflict(behavior) {
        this.state.onConflictBehavior = behavior;
        return this;
    }
    // ---------------------------------------------------------------------------
    // Execution Methods
    // ---------------------------------------------------------------------------
    async then(onfulfilled, onrejected) {
        const result = await this.execute();
        if (onfulfilled) {
            return onfulfilled(result);
        }
        return result;
    }
    async single() {
        this.state.limitValue = 2;
        const result = await this.execute();
        if (result.error) {
            return result;
        }
        const data = result.data;
        if (!data || data.length === 0) {
            return {
                data: null,
                error: new AtomicbaseError("No rows returned", "NOT_FOUND", 404),
            };
        }
        if (data.length > 1) {
            return {
                data: null,
                error: new AtomicbaseError("Multiple rows returned", "MULTIPLE_ROWS", 400),
            };
        }
        return { data: data[0], error: null };
    }
    async maybeSingle() {
        this.state.limitValue = 1;
        const result = await this.execute();
        if (result.error) {
            return result;
        }
        const data = result.data;
        return {
            data: (data?.[0] ?? null),
            error: null,
        };
    }
    async count() {
        this.state.countExact = true;
        this.state.limitValue = 0;
        const { count, error } = await this.executeWithCount();
        return { data: count, error };
    }
    async withCount() {
        this.state.countExact = true;
        return this.executeWithCount();
    }
    // ---------------------------------------------------------------------------
    // Internal Execution
    // ---------------------------------------------------------------------------
    async execute() {
        const { url, headers, body } = this.buildRequest();
        try {
            const response = await this.client.fetch(url, {
                method: "POST",
                headers,
                body: JSON.stringify(body),
            });
            if (!response.ok) {
                const errorBody = await response.json().catch(() => ({}));
                return {
                    data: null,
                    error: new AtomicbaseError(errorBody.message ?? `Request failed with status ${response.status}`, errorBody.code ?? "UNKNOWN_ERROR", response.status, errorBody.hint),
                };
            }
            const text = await response.text();
            const data = text ? JSON.parse(text) : null;
            return { data, error: null };
        }
        catch (err) {
            const message = err instanceof Error ? err.message : "Unknown error";
            return {
                data: null,
                error: new AtomicbaseError(message, "NETWORK_ERROR", 0),
            };
        }
    }
    async executeWithCount() {
        const { url, headers, body } = this.buildRequest();
        try {
            const response = await this.client.fetch(url, {
                method: "POST",
                headers,
                body: JSON.stringify(body),
            });
            if (!response.ok) {
                const errorBody = await response.json().catch(() => ({}));
                return {
                    data: null,
                    count: null,
                    error: new AtomicbaseError(errorBody.message ?? `Request failed with status ${response.status}`, errorBody.code ?? "UNKNOWN_ERROR", response.status, errorBody.hint),
                };
            }
            const countHeader = response.headers.get("X-Total-Count");
            const count = countHeader ? parseInt(countHeader, 10) : null;
            const text = await response.text();
            const data = text ? JSON.parse(text) : null;
            return { data, count, error: null };
        }
        catch (err) {
            const message = err instanceof Error ? err.message : "Unknown error";
            return {
                data: null,
                count: null,
                error: new AtomicbaseError(message, "NETWORK_ERROR", 0),
            };
        }
    }
    buildRequest() {
        const url = `${this.client.baseUrl}/query/${encodeURIComponent(this.state.table)}`;
        const headers = {
            "Content-Type": "application/json",
        };
        if (this.client.apiKey) {
            headers["Authorization"] = `Bearer ${this.client.apiKey}`;
        }
        if (this.client.tenant) {
            headers["Tenant"] = this.client.tenant;
        }
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
        // Build body
        const body = {};
        if (this.state.operation === "select") {
            body.select = this.state.selectColumns;
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
        return { url, headers, body };
    }
}
// =============================================================================
// Client
// =============================================================================
export class AtomicbaseClient {
    baseUrl;
    apiKey;
    tenant;
    fetch;
    constructor(config) {
        this.baseUrl = config.url.replace(/\/$/, "");
        this.apiKey = config.apiKey;
        this.tenant = config.tenant;
        this.fetch = config.fetch ?? globalThis.fetch.bind(globalThis);
    }
    from(table) {
        return new QueryBuilder(this, table);
    }
}
// =============================================================================
// Factory
// =============================================================================
export function createClient(config) {
    return new AtomicbaseClient(config);
}
//# sourceMappingURL=client.js.map