import { AtomicbaseError } from "./AtomicbaseError.js";
/**
 * Base builder class that handles request execution, abort signals, and error modes.
 * This is the foundation of the builder chain.
 */
export class AtomicbaseBuilder {
    state;
    baseUrl;
    apiKey;
    tenant;
    fetchFn;
    defaultHeaders;
    /** AbortSignal for canceling requests */
    signal;
    /** Whether to throw errors instead of returning them */
    shouldThrowOnError = false;
    constructor(config) {
        this.state = config.state;
        this.baseUrl = config.baseUrl;
        this.apiKey = config.apiKey;
        this.tenant = config.tenant;
        this.fetchFn = config.fetch;
        this.defaultHeaders = config.headers ?? {};
    }
    // ---------------------------------------------------------------------------
    // AbortSignal Support
    // ---------------------------------------------------------------------------
    /**
     * Set an AbortSignal to cancel the request.
     *
     * @example
     * ```ts
     * const controller = new AbortController()
     *
     * // Cancel after 5 seconds
     * setTimeout(() => controller.abort(), 5000)
     *
     * const { data, error } = await client
     *   .from('users')
     *   .select()
     *   .abortSignal(controller.signal)
     * ```
     */
    abortSignal(signal) {
        this.signal = signal;
        return this;
    }
    // ---------------------------------------------------------------------------
    // Error Mode Toggle
    // ---------------------------------------------------------------------------
    /**
     * Throw errors instead of returning them in the response.
     *
     * By default, errors are returned in the response object:
     * ```ts
     * const { data, error } = await client.from('users').select()
     * if (error) { ... }
     * ```
     *
     * With throwOnError(), errors are thrown as exceptions:
     * ```ts
     * try {
     *   const { data } = await client.from('users').select().throwOnError()
     * } catch (error) {
     *   // error is AtomicbaseError
     * }
     * ```
     */
    throwOnError() {
        this.shouldThrowOnError = true;
        return this;
    }
    // ---------------------------------------------------------------------------
    // Promise Implementation (Lazy Execution)
    // ---------------------------------------------------------------------------
    /**
     * Implements PromiseLike for lazy query execution.
     * The query only executes when awaited or .then() is called.
     * Handles post-processing based on resultMode (single, maybeSingle, count, withCount).
     */
    then(onfulfilled, onrejected) {
        return this.executeWithResultMode().then(onfulfilled, onrejected);
    }
    /**
     * Execute the query and apply post-processing based on resultMode.
     */
    async executeWithResultMode() {
        const { resultMode } = this.state;
        // For count modes, use executeWithCount
        if (resultMode === "count" || resultMode === "withCount") {
            const result = await this.executeWithCount();
            if (result.error) {
                if (this.shouldThrowOnError) {
                    throw result.error;
                }
                return { data: null, error: result.error };
            }
            if (resultMode === "count") {
                // Return just the count as data
                return { data: (result.count ?? 0), error: null };
            }
            // withCount - return as-is (data + count)
            // Note: The type here is AtomicbaseResponseWithCount but we're returning AtomicbaseResponse
            // The caller should use the withCount-specific types
            return result;
        }
        // For default, single, maybeSingle - use regular execute
        const result = await this.execute();
        if (result.error) {
            if (this.shouldThrowOnError) {
                throw result.error;
            }
            return result;
        }
        // Post-process based on resultMode
        if (resultMode === "single") {
            const data = result.data;
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
            return { data: data[0], error: null };
        }
        if (resultMode === "maybeSingle") {
            const data = result.data;
            return { data: (data?.[0] ?? null), error: null };
        }
        // Default mode - return as-is
        return result;
    }
    // ---------------------------------------------------------------------------
    // Request Execution
    // ---------------------------------------------------------------------------
    async execute() {
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
                if (this.shouldThrowOnError) {
                    throw error;
                }
                return { data: null, error };
            }
            const text = await response.text();
            const data = text ? JSON.parse(text) : null;
            return { data, error: null };
        }
        catch (err) {
            // Re-throw if it's already an AtomicbaseError (from throwOnError)
            if (err instanceof AtomicbaseError) {
                throw err;
            }
            // Handle abort
            if (err instanceof DOMException && err.name === "AbortError") {
                const error = new AtomicbaseError({
                    message: "Request was aborted",
                    code: "ABORTED",
                    status: 0,
                    hint: "The request was canceled via AbortSignal",
                });
                if (this.shouldThrowOnError) {
                    throw error;
                }
                return { data: null, error };
            }
            // Network/other errors
            const error = AtomicbaseError.networkError(err);
            if (this.shouldThrowOnError) {
                throw error;
            }
            return { data: null, error };
        }
    }
    async executeWithCount() {
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
                if (this.shouldThrowOnError) {
                    throw error;
                }
                return { data: null, count: null, error };
            }
            const countHeader = response.headers.get("X-Total-Count");
            const count = countHeader ? parseInt(countHeader, 10) : null;
            const text = await response.text();
            const data = text ? JSON.parse(text) : null;
            return { data, count, error: null };
        }
        catch (err) {
            if (err instanceof AtomicbaseError) {
                throw err;
            }
            if (err instanceof DOMException && err.name === "AbortError") {
                const error = new AtomicbaseError({
                    message: "Request was aborted",
                    code: "ABORTED",
                    status: 0,
                });
                if (this.shouldThrowOnError) {
                    throw error;
                }
                return { data: null, count: null, error };
            }
            const error = AtomicbaseError.networkError(err);
            if (this.shouldThrowOnError) {
                throw error;
            }
            return { data: null, count: null, error };
        }
    }
    // ---------------------------------------------------------------------------
    // Common Header Building
    // ---------------------------------------------------------------------------
    buildCommonHeaders() {
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
        return headers;
    }
}
//# sourceMappingURL=AtomicbaseBuilder.js.map