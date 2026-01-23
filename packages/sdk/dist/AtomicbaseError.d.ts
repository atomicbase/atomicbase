/**
 * Error class for Atomicbase API errors.
 *
 * @example
 * ```ts
 * const { data, error } = await client.from('users').select()
 * if (error) {
 *   console.log(error.message)  // Human-readable message
 *   console.log(error.code)     // Error code like "NOT_FOUND"
 *   console.log(error.status)   // HTTP status code
 *   console.log(error.hint)     // Suggestion for fixing the error
 *   console.log(error.details)  // Additional error context
 * }
 * ```
 */
export declare class AtomicbaseError extends Error {
    /** Error code identifying the type of error */
    code: string;
    /** HTTP status code (0 for network errors) */
    status: number;
    /** Suggestion for resolving the error */
    hint?: string;
    /** Additional error context or details */
    details?: string;
    constructor(context: {
        message: string;
        code: string;
        status: number;
        hint?: string;
        details?: string;
    });
    /**
     * Creates an error from an API response body.
     */
    static fromResponse(body: Record<string, unknown>, status: number): AtomicbaseError;
    /**
     * Creates a network error.
     */
    static networkError(cause: unknown): AtomicbaseError;
}
//# sourceMappingURL=AtomicbaseError.d.ts.map