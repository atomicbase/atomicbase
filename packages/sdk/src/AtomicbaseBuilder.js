"use strict";
var __assign = (this && this.__assign) || function () {
    __assign = Object.assign || function(t) {
        for (var s, i = 1, n = arguments.length; i < n; i++) {
            s = arguments[i];
            for (var p in s) if (Object.prototype.hasOwnProperty.call(s, p))
                t[p] = s[p];
        }
        return t;
    };
    return __assign.apply(this, arguments);
};
var __awaiter = (this && this.__awaiter) || function (thisArg, _arguments, P, generator) {
    function adopt(value) { return value instanceof P ? value : new P(function (resolve) { resolve(value); }); }
    return new (P || (P = Promise))(function (resolve, reject) {
        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }
        function rejected(value) { try { step(generator["throw"](value)); } catch (e) { reject(e); } }
        function step(result) { result.done ? resolve(result.value) : adopt(result.value).then(fulfilled, rejected); }
        step((generator = generator.apply(thisArg, _arguments || [])).next());
    });
};
var __generator = (this && this.__generator) || function (thisArg, body) {
    var _ = { label: 0, sent: function() { if (t[0] & 1) throw t[1]; return t[1]; }, trys: [], ops: [] }, f, y, t, g = Object.create((typeof Iterator === "function" ? Iterator : Object).prototype);
    return g.next = verb(0), g["throw"] = verb(1), g["return"] = verb(2), typeof Symbol === "function" && (g[Symbol.iterator] = function() { return this; }), g;
    function verb(n) { return function (v) { return step([n, v]); }; }
    function step(op) {
        if (f) throw new TypeError("Generator is already executing.");
        while (g && (g = 0, op[0] && (_ = 0)), _) try {
            if (f = 1, y && (t = op[0] & 2 ? y["return"] : op[0] ? y["throw"] || ((t = y["return"]) && t.call(y), 0) : y.next) && !(t = t.call(y, op[1])).done) return t;
            if (y = 0, t) op = [op[0] & 2, t.value];
            switch (op[0]) {
                case 0: case 1: t = op; break;
                case 4: _.label++; return { value: op[1], done: false };
                case 5: _.label++; y = op[1]; op = [0]; continue;
                case 7: op = _.ops.pop(); _.trys.pop(); continue;
                default:
                    if (!(t = _.trys, t = t.length > 0 && t[t.length - 1]) && (op[0] === 6 || op[0] === 2)) { _ = 0; continue; }
                    if (op[0] === 3 && (!t || (op[1] > t[0] && op[1] < t[3]))) { _.label = op[1]; break; }
                    if (op[0] === 6 && _.label < t[1]) { _.label = t[1]; t = op; break; }
                    if (t && _.label < t[2]) { _.label = t[2]; _.ops.push(op); break; }
                    if (t[2]) _.ops.pop();
                    _.trys.pop(); continue;
            }
            op = body.call(thisArg, _);
        } catch (e) { op = [6, e]; y = 0; } finally { f = t = 0; }
        if (op[0] & 5) throw op[1]; return { value: op[0] ? op[1] : void 0, done: true };
    }
};
Object.defineProperty(exports, "__esModule", { value: true });
exports.AtomicbaseBuilder = void 0;
var AtomicbaseError_js_1 = require("./AtomicbaseError.js");
/**
 * Base builder class that handles request execution, abort signals, and error modes.
 * This is the foundation of the builder chain.
 */
var AtomicbaseBuilder = /** @class */ (function () {
    function AtomicbaseBuilder(config) {
        var _a;
        /** Whether to throw errors instead of returning them */
        this.shouldThrowOnError = false;
        this.state = config.state;
        this.baseUrl = config.baseUrl;
        this.apiKey = config.apiKey;
        this.tenant = config.tenant;
        this.fetchFn = config.fetch;
        this.defaultHeaders = (_a = config.headers) !== null && _a !== void 0 ? _a : {};
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
    AtomicbaseBuilder.prototype.abortSignal = function (signal) {
        this.signal = signal;
        return this;
    };
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
    AtomicbaseBuilder.prototype.throwOnError = function () {
        this.shouldThrowOnError = true;
        return this;
    };
    // ---------------------------------------------------------------------------
    // Promise Implementation (Lazy Execution)
    // ---------------------------------------------------------------------------
    /**
     * Implements PromiseLike for lazy query execution.
     * The query only executes when awaited or .then() is called.
     * Handles post-processing based on resultMode (single, maybeSingle, count, withCount).
     */
    AtomicbaseBuilder.prototype.then = function (onfulfilled, onrejected) {
        return this.executeWithResultMode().then(onfulfilled, onrejected);
    };
    /**
     * Execute the query and apply post-processing based on resultMode.
     */
    AtomicbaseBuilder.prototype.executeWithResultMode = function () {
        return __awaiter(this, void 0, void 0, function () {
            var resultMode, result_1, result, data, error, error, data;
            var _a, _b;
            return __generator(this, function (_c) {
                switch (_c.label) {
                    case 0:
                        resultMode = this.state.resultMode;
                        if (!(resultMode === "count" || resultMode === "withCount")) return [3 /*break*/, 2];
                        return [4 /*yield*/, this.executeWithCount()];
                    case 1:
                        result_1 = _c.sent();
                        if (result_1.error) {
                            if (this.shouldThrowOnError) {
                                throw result_1.error;
                            }
                            return [2 /*return*/, { data: null, error: result_1.error }];
                        }
                        if (resultMode === "count") {
                            // Return just the count as data
                            return [2 /*return*/, { data: ((_a = result_1.count) !== null && _a !== void 0 ? _a : 0), error: null }];
                        }
                        // withCount - return as-is (data + count)
                        // Note: The type here is AtomicbaseResponseWithCount but we're returning AtomicbaseResponse
                        // The caller should use the withCount-specific types
                        return [2 /*return*/, result_1];
                    case 2: return [4 /*yield*/, this.execute()];
                    case 3:
                        result = _c.sent();
                        if (result.error) {
                            if (this.shouldThrowOnError) {
                                throw result.error;
                            }
                            return [2 /*return*/, result];
                        }
                        // Post-process based on resultMode
                        if (resultMode === "single") {
                            data = result.data;
                            if (!data || data.length === 0) {
                                error = new AtomicbaseError_js_1.AtomicbaseError({
                                    message: "No rows returned",
                                    code: "NOT_FOUND",
                                    status: 404,
                                    hint: "The query returned no results. Check your filter conditions.",
                                });
                                if (this.shouldThrowOnError) {
                                    throw error;
                                }
                                return [2 /*return*/, { data: null, error: error }];
                            }
                            if (data.length > 1) {
                                error = new AtomicbaseError_js_1.AtomicbaseError({
                                    message: "Multiple rows returned",
                                    code: "MULTIPLE_ROWS",
                                    status: 400,
                                    hint: "Expected a single row but got multiple. Add more specific filters.",
                                });
                                if (this.shouldThrowOnError) {
                                    throw error;
                                }
                                return [2 /*return*/, { data: null, error: error }];
                            }
                            return [2 /*return*/, { data: data[0], error: null }];
                        }
                        if (resultMode === "maybeSingle") {
                            data = result.data;
                            return [2 /*return*/, { data: ((_b = data === null || data === void 0 ? void 0 : data[0]) !== null && _b !== void 0 ? _b : null), error: null }];
                        }
                        // Default mode - return as-is
                        return [2 /*return*/, result];
                }
            });
        });
    };
    // ---------------------------------------------------------------------------
    // Request Execution
    // ---------------------------------------------------------------------------
    AtomicbaseBuilder.prototype.execute = function () {
        return __awaiter(this, void 0, void 0, function () {
            var _a, url, headers, body, response, errorBody, error, text, data, err_1, error_1, error;
            return __generator(this, function (_b) {
                switch (_b.label) {
                    case 0:
                        _a = this.buildRequest(), url = _a.url, headers = _a.headers, body = _a.body;
                        _b.label = 1;
                    case 1:
                        _b.trys.push([1, 6, , 7]);
                        return [4 /*yield*/, this.fetchFn(url, {
                                method: "POST",
                                headers: headers,
                                body: JSON.stringify(body),
                                signal: this.signal,
                            })];
                    case 2:
                        response = _b.sent();
                        if (!!response.ok) return [3 /*break*/, 4];
                        return [4 /*yield*/, response.json().catch(function () { return ({}); })];
                    case 3:
                        errorBody = _b.sent();
                        error = AtomicbaseError_js_1.AtomicbaseError.fromResponse(errorBody, response.status);
                        if (this.shouldThrowOnError) {
                            throw error;
                        }
                        return [2 /*return*/, { data: null, error: error }];
                    case 4: return [4 /*yield*/, response.text()];
                    case 5:
                        text = _b.sent();
                        data = text ? JSON.parse(text) : null;
                        return [2 /*return*/, { data: data, error: null }];
                    case 6:
                        err_1 = _b.sent();
                        // Re-throw if it's already an AtomicbaseError (from throwOnError)
                        if (err_1 instanceof AtomicbaseError_js_1.AtomicbaseError) {
                            throw err_1;
                        }
                        // Handle abort
                        if (err_1 instanceof DOMException && err_1.name === "AbortError") {
                            error_1 = new AtomicbaseError_js_1.AtomicbaseError({
                                message: "Request was aborted",
                                code: "ABORTED",
                                status: 0,
                                hint: "The request was canceled via AbortSignal",
                            });
                            if (this.shouldThrowOnError) {
                                throw error_1;
                            }
                            return [2 /*return*/, { data: null, error: error_1 }];
                        }
                        error = AtomicbaseError_js_1.AtomicbaseError.networkError(err_1);
                        if (this.shouldThrowOnError) {
                            throw error;
                        }
                        return [2 /*return*/, { data: null, error: error }];
                    case 7: return [2 /*return*/];
                }
            });
        });
    };
    AtomicbaseBuilder.prototype.executeWithCount = function () {
        return __awaiter(this, void 0, void 0, function () {
            var _a, url, headers, body, response, errorBody, error, countHeader, count, text, data, err_2, error_2, error;
            return __generator(this, function (_b) {
                switch (_b.label) {
                    case 0:
                        _a = this.buildRequest(), url = _a.url, headers = _a.headers, body = _a.body;
                        _b.label = 1;
                    case 1:
                        _b.trys.push([1, 6, , 7]);
                        return [4 /*yield*/, this.fetchFn(url, {
                                method: "POST",
                                headers: headers,
                                body: JSON.stringify(body),
                                signal: this.signal,
                            })];
                    case 2:
                        response = _b.sent();
                        if (!!response.ok) return [3 /*break*/, 4];
                        return [4 /*yield*/, response.json().catch(function () { return ({}); })];
                    case 3:
                        errorBody = _b.sent();
                        error = AtomicbaseError_js_1.AtomicbaseError.fromResponse(errorBody, response.status);
                        if (this.shouldThrowOnError) {
                            throw error;
                        }
                        return [2 /*return*/, { data: null, count: null, error: error }];
                    case 4:
                        countHeader = response.headers.get("X-Total-Count");
                        count = countHeader ? parseInt(countHeader, 10) : null;
                        return [4 /*yield*/, response.text()];
                    case 5:
                        text = _b.sent();
                        data = text ? JSON.parse(text) : null;
                        return [2 /*return*/, { data: data, count: count, error: null }];
                    case 6:
                        err_2 = _b.sent();
                        if (err_2 instanceof AtomicbaseError_js_1.AtomicbaseError) {
                            throw err_2;
                        }
                        if (err_2 instanceof DOMException && err_2.name === "AbortError") {
                            error_2 = new AtomicbaseError_js_1.AtomicbaseError({
                                message: "Request was aborted",
                                code: "ABORTED",
                                status: 0,
                            });
                            if (this.shouldThrowOnError) {
                                throw error_2;
                            }
                            return [2 /*return*/, { data: null, count: null, error: error_2 }];
                        }
                        error = AtomicbaseError_js_1.AtomicbaseError.networkError(err_2);
                        if (this.shouldThrowOnError) {
                            throw error;
                        }
                        return [2 /*return*/, { data: null, count: null, error: error }];
                    case 7: return [2 /*return*/];
                }
            });
        });
    };
    // ---------------------------------------------------------------------------
    // Common Header Building
    // ---------------------------------------------------------------------------
    AtomicbaseBuilder.prototype.buildCommonHeaders = function () {
        var headers = __assign({ "Content-Type": "application/json" }, this.defaultHeaders);
        if (this.apiKey) {
            headers["Authorization"] = "Bearer ".concat(this.apiKey);
        }
        if (this.tenant) {
            headers["Tenant"] = this.tenant;
        }
        return headers;
    };
    return AtomicbaseBuilder;
}());
exports.AtomicbaseBuilder = AtomicbaseBuilder;
