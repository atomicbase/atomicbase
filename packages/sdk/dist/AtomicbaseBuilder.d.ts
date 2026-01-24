import type { AtomicbaseResponse, AtomicbaseResponseWithCount, FilterCondition, OrderDirection, ResultMode, SelectColumn, JoinClause, QueryOperation } from "./types.js";
/**
 * Internal query state - simplified structure.
 */
export interface QueryState {
    table: string;
    operation: QueryOperation | null;
    select: SelectColumn[];
    joins: JoinClause[];
    where: FilterCondition[];
    order: Record<string, OrderDirection> | null;
    limit: number | null;
    offset: number | null;
    data: unknown;
    returning: string[];
    onConflict: "ignore" | null;
    count: boolean;
    resultMode: ResultMode;
}
/**
 * Configuration for creating a builder.
 */
export interface BuilderConfig {
    table: string;
    baseUrl: string;
    apiKey?: string;
    tenant?: string;
    fetch: typeof fetch;
    headers?: Record<string, string>;
}
/**
 * Base builder class that handles query construction, filtering, transforms, and execution.
 * Implements PromiseLike for lazy execution - queries only run when awaited.
 */
export declare abstract class AtomicbaseBuilder<T> implements PromiseLike<AtomicbaseResponse<T>> {
    protected state: QueryState;
    protected readonly baseUrl: string;
    protected readonly apiKey?: string;
    protected readonly tenant?: string;
    protected readonly fetchFn: typeof fetch;
    protected readonly defaultHeaders: Record<string, string>;
    protected signal?: AbortSignal;
    protected shouldThrowOnError: boolean;
    constructor(config: BuilderConfig);
    /** Equality filter: column = value */
    eq(column: string, value: unknown): this;
    /** Not equal filter: column != value */
    neq(column: string, value: unknown): this;
    /** Greater than filter: column > value */
    gt(column: string, value: unknown): this;
    /** Greater than or equal filter: column >= value */
    gte(column: string, value: unknown): this;
    /** Less than filter: column < value */
    lt(column: string, value: unknown): this;
    /** Less than or equal filter: column <= value */
    lte(column: string, value: unknown): this;
    /** LIKE filter: column LIKE pattern */
    like(column: string, pattern: string): this;
    /** GLOB filter: column GLOB pattern */
    glob(column: string, pattern: string): this;
    /** IN filter: column IN (values) */
    in(column: string, values: unknown[]): this;
    /** IS NULL filter */
    isNull(column: string): this;
    /** IS NOT NULL filter */
    isNotNull(column: string): this;
    /** BETWEEN filter: column BETWEEN min AND max */
    between(column: string, min: unknown, max: unknown): this;
    /**
     * Add filter conditions using helper functions.
     * Use this for complex conditions like OR, NOT, or nested filters.
     *
     * @example
     * ```ts
     * // Simple filters - use fluent methods
     * .eq('status', 'active').gt('age', 18)
     *
     * // Complex filters - use where() with helpers
     * .where(or(eq('role', 'admin'), eq('role', 'mod')))
     * ```
     */
    where(...conditions: FilterCondition[]): this;
    /** Order results by a column. */
    orderBy(column: string, direction?: OrderDirection): this;
    /** Limit the number of rows returned. */
    limit(count: number): this;
    /** Skip a number of rows before returning results. */
    offset(count: number): this;
    /** Specify columns to return after insert/update/delete. */
    returning(...columns: string[]): this;
    /**
     * Return a single row. Errors if zero or multiple rows returned.
     */
    single<Result = T extends (infer U)[] ? U : T>(): AtomicbaseBuilder<Result>;
    /**
     * Return zero or one row. Returns null if no rows found.
     */
    maybeSingle<Result = T extends (infer U)[] ? U | null : T | null>(): AtomicbaseBuilder<Result>;
    /**
     * Return only the count of matching rows.
     */
    count(): AtomicbaseBuilder<number>;
    /**
     * Return both data and total count.
     */
    withCount(): AtomicbaseBuilder<T> & PromiseLike<AtomicbaseResponseWithCount<T>>;
    /** Set an AbortSignal to cancel the request. */
    abortSignal(signal: AbortSignal): this;
    /** Throw errors instead of returning them in the response. */
    throwOnError(): this;
    /**
     * Export this query as a batch operation.
     * @internal
     */
    toBatchOperation(): {
        operation: string;
        table: string;
        body: Record<string, unknown>;
        count?: boolean;
        resultMode?: string;
    };
    then<TResult1 = AtomicbaseResponse<T>, TResult2 = never>(onfulfilled?: ((value: AtomicbaseResponse<T>) => TResult1 | PromiseLike<TResult1>) | null, onrejected?: ((reason: unknown) => TResult2 | PromiseLike<TResult2>) | null): Promise<TResult1 | TResult2>;
    private executeWithResultMode;
    /**
     * Execute the request. Unified method for both regular and count queries.
     */
    private execute;
    protected abstract buildBody(): Record<string, unknown>;
    protected buildRequest(): {
        url: string;
        headers: Record<string, string>;
        body: Record<string, unknown>;
    };
    protected buildHeaders(): Record<string, string>;
}
//# sourceMappingURL=AtomicbaseBuilder.d.ts.map