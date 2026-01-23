export type FilterCondition = Record<string, unknown>;
export type SelectColumn = string | {
    [relation: string]: SelectColumn[];
};
export type OrderDirection = "asc" | "desc";
export interface Result<T> {
    data: T | null;
    error: AtomicbaseError | null;
}
export interface ResultWithCount<T> {
    data: T | null;
    count: number | null;
    error: AtomicbaseError | null;
}
export interface AtomicbaseConfig {
    url: string;
    apiKey?: string;
    fetch?: typeof fetch;
}
export declare class AtomicbaseError extends Error {
    code: string;
    status: number;
    hint?: string | undefined;
    constructor(message: string, code: string, status: number, hint?: string | undefined);
}
/** Equality condition: column = value */
export declare function eq(column: string, value: unknown): FilterCondition;
/** Not equal condition: column != value */
export declare function neq(column: string, value: unknown): FilterCondition;
/** Greater than condition: column > value */
export declare function gt(column: string, value: unknown): FilterCondition;
/** Greater than or equal condition: column >= value */
export declare function gte(column: string, value: unknown): FilterCondition;
/** Less than condition: column < value */
export declare function lt(column: string, value: unknown): FilterCondition;
/** Less than or equal condition: column <= value */
export declare function lte(column: string, value: unknown): FilterCondition;
/** LIKE condition: column LIKE pattern */
export declare function like(column: string, pattern: string): FilterCondition;
/** GLOB condition: column GLOB pattern */
export declare function glob(column: string, pattern: string): FilterCondition;
/** IN condition: column IN (values) */
export declare function inArray(column: string, values: unknown[]): FilterCondition;
/** BETWEEN condition: column BETWEEN min AND max */
export declare function between(column: string, min: unknown, max: unknown): FilterCondition;
/** IS NULL condition */
export declare function isNull(column: string): FilterCondition;
/** IS NOT NULL condition */
export declare function isNotNull(column: string): FilterCondition;
/** Full-text search condition */
export declare function fts(column: string, query: string): FilterCondition;
/** Negate a condition */
export declare function not(condition: FilterCondition): FilterCondition;
/** OR condition: (condition1 OR condition2 OR ...) */
export declare function or(...conditions: FilterCondition[]): FilterCondition;
/** AND condition: (condition1 AND condition2 AND ...) */
export declare function and(...conditions: FilterCondition[]): FilterCondition;
export declare class QueryBuilder<T = Record<string, unknown>> {
    private state;
    private client;
    constructor(client: AtomicbaseClient, table: string);
    select(...columns: SelectColumn[]): QueryBuilder<T[]>;
    insert(data: Partial<T> | Partial<T>[]): QueryBuilder<{
        last_insert_id: number;
    }>;
    upsert(data: Partial<T> | Partial<T>[]): QueryBuilder<{
        rows_affected: number;
    }>;
    update(data: Partial<T>): QueryBuilder<{
        rows_affected: number;
    }>;
    delete(): QueryBuilder<{
        rows_affected: number;
    }>;
    where(...conditions: FilterCondition[]): this;
    orderBy(column: string, direction?: OrderDirection): this;
    limit(n: number): this;
    offset(n: number): this;
    returning(...columns: string[]): this;
    onConflict(behavior: "ignore"): this;
    then<TResult1 = Result<T>, TResult2 = never>(onfulfilled?: ((value: Result<T>) => TResult1 | PromiseLike<TResult1>) | null, onrejected?: ((reason: unknown) => TResult2 | PromiseLike<TResult2>) | null): Promise<TResult1 | TResult2>;
    single(): Promise<Result<T extends (infer U)[] ? U : T>>;
    maybeSingle(): Promise<Result<(T extends (infer U)[] ? U : T) | null>>;
    count(): Promise<Result<number>>;
    withCount(): Promise<ResultWithCount<T>>;
    private execute;
    private executeWithCount;
    private buildRequest;
}
export declare class AtomicbaseClient {
    readonly baseUrl: string;
    readonly apiKey?: string;
    readonly tenant?: string;
    readonly fetch: typeof globalThis.fetch;
    constructor(config: AtomicbaseConfig & {
        tenant?: string;
    });
    from<T = Record<string, unknown>>(table: string): QueryBuilder<T>;
}
export declare function createClient(config: AtomicbaseConfig): AtomicbaseClient;
//# sourceMappingURL=client.d.ts.map