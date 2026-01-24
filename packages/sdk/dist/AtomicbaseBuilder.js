import { AtomicbaseError } from "./AtomicbaseError.js";
/**
 * Base builder class that handles query construction, filtering, transforms, and execution.
 * Implements PromiseLike for lazy execution - queries only run when awaited.
 */
export class AtomicbaseBuilder {
    state;
    baseUrl;
    apiKey;
    tenant;
    fetchFn;
    defaultHeaders;
    signal;
    shouldThrowOnError = false;
    constructor(config) {
        this.state = {
            table: config.table,
            operation: null,
            select: [],
            joins: [],
            where: [],
            order: null,
            limit: null,
            offset: null,
            data: null,
            returning: [],
            onConflict: null,
            count: false,
            resultMode: "default",
        };
        this.baseUrl = config.baseUrl;
        this.apiKey = config.apiKey;
        this.tenant = config.tenant;
        this.fetchFn = config.fetch;
        this.defaultHeaders = config.headers ?? {};
    }
    // ===========================================================================
    // Filtering
    // ===========================================================================
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
    where(...conditions) {
        this.state.where.push(...conditions);
        return this;
    }
    // ===========================================================================
    // Ordering & Pagination
    // ===========================================================================
    /** Order results by a column. */
    orderBy(column, direction = "asc") {
        this.state.order = { [column]: direction };
        return this;
    }
    /** Limit the number of rows returned. */
    limit(count) {
        this.state.limit = count;
        return this;
    }
    /** Skip a number of rows before returning results. */
    offset(count) {
        this.state.offset = count;
        return this;
    }
    // ===========================================================================
    // RETURNING Clause
    // ===========================================================================
    /** Specify columns to return after insert/update/delete. */
    returning(...columns) {
        this.state.returning = columns.length > 0 ? columns : ["*"];
        return this;
    }
    // ===========================================================================
    // Result Modifiers (Type-Changing)
    // ===========================================================================
    /**
     * Return a single row. Errors if zero or multiple rows returned.
     */
    single() {
        this.state.resultMode = "single";
        if (this.state.limit === null) {
            this.state.limit = 2; // Fetch 2 to detect multiple rows
        }
        return this;
    }
    /**
     * Return zero or one row. Returns null if no rows found.
     */
    maybeSingle() {
        this.state.resultMode = "maybeSingle";
        if (this.state.limit === null) {
            this.state.limit = 1;
        }
        return this;
    }
    /**
     * Return only the count of matching rows.
     */
    count() {
        this.state.resultMode = "count";
        this.state.count = true;
        this.state.limit = 0;
        return this;
    }
    /**
     * Return both data and total count.
     */
    withCount() {
        this.state.resultMode = "withCount";
        this.state.count = true;
        return this;
    }
    // ===========================================================================
    // Request Options
    // ===========================================================================
    /** Set an AbortSignal to cancel the request. */
    abortSignal(signal) {
        this.signal = signal;
        return this;
    }
    /** Throw errors instead of returning them in the response. */
    throwOnError() {
        this.shouldThrowOnError = true;
        return this;
    }
    // ===========================================================================
    // Batch Support
    // ===========================================================================
    /**
     * Export this query as a batch operation.
     * @internal
     */
    toBatchOperation() {
        if (!this.state.operation) {
            throw new Error("No operation specified. Call select(), insert(), update(), or delete() first.");
        }
        const result = {
            operation: this.state.operation,
            table: this.state.table,
            body: this.buildBody(),
        };
        if (this.state.count) {
            result.count = true;
        }
        if (this.state.resultMode !== "default") {
            result.resultMode = this.state.resultMode;
        }
        return result;
    }
    // ===========================================================================
    // Promise Implementation (Lazy Execution)
    // ===========================================================================
    then(onfulfilled, onrejected) {
        return this.executeWithResultMode().then(onfulfilled, onrejected);
    }
    // ===========================================================================
    // Internal: Execution
    // ===========================================================================
    async executeWithResultMode() {
        const { resultMode } = this.state;
        const needsCount = resultMode === "count" || resultMode === "withCount";
        const result = await this.execute(needsCount);
        if (result.error) {
            if (this.shouldThrowOnError)
                throw result.error;
            return { data: null, error: result.error };
        }
        // Post-process based on resultMode
        switch (resultMode) {
            case "count":
                return { data: (result.count ?? 0), error: null };
            case "withCount":
                return result;
            case "single": {
                const data = result.data;
                if (!data || data.length === 0) {
                    const error = new AtomicbaseError({
                        message: "No rows returned",
                        code: "NOT_FOUND",
                        status: 404,
                        hint: "The query returned no results. Check your filter conditions.",
                    });
                    if (this.shouldThrowOnError)
                        throw error;
                    return { data: null, error };
                }
                if (data.length > 1) {
                    const error = new AtomicbaseError({
                        message: "Multiple rows returned",
                        code: "MULTIPLE_ROWS",
                        status: 400,
                        hint: "Expected a single row but got multiple. Add more specific filters.",
                    });
                    if (this.shouldThrowOnError)
                        throw error;
                    return { data: null, error };
                }
                return { data: data[0], error: null };
            }
            case "maybeSingle": {
                const data = result.data;
                return { data: (data?.[0] ?? null), error: null };
            }
            default:
                return { data: result.data, error: null };
        }
    }
    /**
     * Execute the request. Unified method for both regular and count queries.
     */
    async execute(withCount) {
        const { url, headers, body } = this.buildRequest();
        try {
            const response = await this.fetchFn(url, {
                method: "POST",
                headers,
                body: JSON.stringify(body),
                signal: this.signal,
            });
            if (!response.ok) {
                const errorBody = await response.json().catch(() => ({}));
                const error = AtomicbaseError.fromResponse(errorBody, response.status);
                if (this.shouldThrowOnError)
                    throw error;
                return { data: null, count: null, error };
            }
            const count = withCount
                ? parseInt(response.headers.get("X-Total-Count") ?? "", 10) || null
                : null;
            const text = await response.text();
            const data = text ? JSON.parse(text) : null;
            return { data, count, error: null };
        }
        catch (err) {
            if (err instanceof AtomicbaseError)
                throw err;
            if (err instanceof DOMException && err.name === "AbortError") {
                const error = new AtomicbaseError({
                    message: "Request was aborted",
                    code: "ABORTED",
                    status: 0,
                    hint: "The request was canceled via AbortSignal",
                });
                if (this.shouldThrowOnError)
                    throw error;
                return { data: null, count: null, error };
            }
            const error = AtomicbaseError.networkError(err);
            if (this.shouldThrowOnError)
                throw error;
            return { data: null, count: null, error };
        }
    }
    buildRequest() {
        const url = `${this.baseUrl}/data/query/${encodeURIComponent(this.state.table)}`;
        const headers = this.buildHeaders();
        return { url, headers, body: this.buildBody() };
    }
    buildHeaders() {
        const headers = {
            "Content-Type": "application/json",
            ...this.defaultHeaders,
        };
        if (this.apiKey) {
            headers["Authorization"] = `Bearer ${this.apiKey}`;
        }
        if (this.tenant) {
            headers["Tenant"] = this.tenant;
        }
        // Build Prefer header based on operation
        const preferParts = [];
        switch (this.state.operation) {
            case "select":
                preferParts.push("operation=select");
                if (this.state.count)
                    preferParts.push("count=exact");
                break;
            case "insert":
                preferParts.push("operation=insert");
                if (this.state.onConflict === "ignore")
                    preferParts.push("on-conflict=ignore");
                break;
            case "upsert":
                preferParts.push("operation=insert", "on-conflict=replace");
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
        return headers;
    }
}
//# sourceMappingURL=AtomicbaseBuilder.js.map