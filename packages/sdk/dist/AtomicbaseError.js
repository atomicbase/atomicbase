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
export class AtomicbaseError extends Error {
    /** Error code identifying the type of error */
    code;
    /** HTTP status code (0 for network errors) */
    status;
    /** Suggestion for resolving the error */
    hint;
    /** Additional error context or details */
    details;
    constructor(context) {
        super(context.message);
        this.name = "AtomicbaseError";
        this.code = context.code;
        this.status = context.status;
        this.hint = context.hint;
        this.details = context.details;
    }
    /**
     * Creates an error from an API response body.
     */
    static fromResponse(body, status) {
        return new AtomicbaseError({
            message: body.message ?? `Request failed with status ${status}`,
            code: body.code ?? "UNKNOWN_ERROR",
            status,
            hint: body.hint,
            details: body.details,
        });
    }
    /**
     * Creates a network error.
     */
    static networkError(cause) {
        const message = cause instanceof Error ? cause.message : "Network request failed";
        const details = cause instanceof Error ? cause.stack : undefined;
        return new AtomicbaseError({
            message,
            code: "NETWORK_ERROR",
            status: 0,
            hint: "Check your network connection and API URL",
            details,
        });
    }
}
//# sourceMappingURL=AtomicbaseError.js.map