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
exports.AtomicbaseError = void 0;
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
var AtomicbaseError = /** @class */ (function (_super) {
    __extends(AtomicbaseError, _super);
    function AtomicbaseError(context) {
        var _this = _super.call(this, context.message) || this;
        _this.name = "AtomicbaseError";
        _this.code = context.code;
        _this.status = context.status;
        _this.hint = context.hint;
        _this.details = context.details;
        return _this;
    }
    /**
     * Creates an error from an API response body.
     */
    AtomicbaseError.fromResponse = function (body, status) {
        var _a, _b;
        return new AtomicbaseError({
            message: (_a = body.message) !== null && _a !== void 0 ? _a : "Request failed with status ".concat(status),
            code: (_b = body.code) !== null && _b !== void 0 ? _b : "UNKNOWN_ERROR",
            status: status,
            hint: body.hint,
            details: body.details,
        });
    };
    /**
     * Creates a network error.
     */
    AtomicbaseError.networkError = function (cause) {
        var message = cause instanceof Error ? cause.message : "Network request failed";
        var details = cause instanceof Error ? cause.stack : undefined;
        return new AtomicbaseError({
            message: message,
            code: "NETWORK_ERROR",
            status: 0,
            hint: "Check your network connection and API URL",
            details: details,
        });
    };
    return AtomicbaseError;
}(Error));
exports.AtomicbaseError = AtomicbaseError;
