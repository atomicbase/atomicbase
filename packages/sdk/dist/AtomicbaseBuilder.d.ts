import type { AtomicbaseResponse, AtomicbaseResponseWithCount, QueryState } from "./types.js";
/**
 * Base builder class that handles request execution, abort signals, and error modes.
 * This is the foundation of the builder chain.
 */
export declare abstract class AtomicbaseBuilder<T> implements PromiseLike<AtomicbaseResponse<T>> {
    protected state: QueryState;
    protected baseUrl: string;
    protected apiKey?: string;
    protected tenant?: string;
    protected fetchFn: typeof fetch;
    protected defaultHeaders: Record<string, string>;
    /** AbortSignal for canceling requests */
    protected signal?: AbortSignal;
    /** Whether to throw errors instead of returning them */
    protected shouldThrowOnError: boolean;
    constructor(config: {
        state: QueryState;
        baseUrl: string;
        apiKey?: string;
        tenant?: string;
        fetch: typeof fetch;
        headers?: Record<string, string>;
    });
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
    abortSignal(signal: AbortSignal): this;
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
    throwOnError(): this;
    /**
     * Implements PromiseLike for lazy query execution.
     * The query only executes when awaited or .then() is called.
     * Handles post-processing based on resultMode (single, maybeSingle, count, withCount).
     */
    then<TResult1 = AtomicbaseResponse<T>, TResult2 = never>(onfulfilled?: ((value: AtomicbaseResponse<T>) => TResult1 | PromiseLike<TResult1>) | null, onrejected?: ((reason: unknown) => TResult2 | PromiseLike<TResult2>) | null): Promise<TResult1 | TResult2>;
    /**
     * Execute the query and apply post-processing based on resultMode.
     */
    private executeWithResultMode;
    protected abstract buildRequest(): {
        url: string;
        headers: Record<string, string>;
        body: Record<string, unknown>;
    };
    protected execute(): Promise<AtomicbaseResponse<T>>;
    protected executeWithCount(): Promise<AtomicbaseResponseWithCount<T>>;
    protected buildCommonHeaders(): Record<string, string>;
}
//# sourceMappingURL=AtomicbaseBuilder.d.ts.map