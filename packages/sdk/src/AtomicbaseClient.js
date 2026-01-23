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
exports.AtomicbaseClient = void 0;
exports.createClient = createClient;
var AtomicbaseQueryBuilder_js_1 = require("./AtomicbaseQueryBuilder.js");
var AtomicbaseError_js_1 = require("./AtomicbaseError.js");
/**
 * Atomicbase client for database operations.
 *
 * @example
 * ```ts
 * import { createClient, eq } from '@atomicbase/sdk'
 *
 * const client = createClient({
 *   url: 'http://localhost:8080',
 *   apiKey: 'your-api-key',
 * })
 *
 * // Query data
 * const { data, error } = await client
 *   .from('users')
 *   .select('id', 'name')
 *   .where(eq('status', 'active'))
 *   .orderBy('created_at', 'desc')
 *   .limit(10)
 *
 * // Insert data
 * const { data } = await client
 *   .from('users')
 *   .insert({ name: 'Alice', email: 'alice@example.com' })
 * ```
 */
var AtomicbaseClient = /** @class */ (function () {
    function AtomicbaseClient(options) {
        var _a, _b;
        this.baseUrl = options.url.replace(/\/$/, "");
        this.apiKey = options.apiKey;
        this.headers = (_a = options.headers) !== null && _a !== void 0 ? _a : {};
        this.fetchFn = (_b = options.fetch) !== null && _b !== void 0 ? _b : globalThis.fetch.bind(globalThis);
    }
    /**
     * Start a query on a table.
     *
     * @example
     * ```ts
     * const { data } = await client.from('users').select()
     * ```
     */
    AtomicbaseClient.prototype.from = function (table) {
        return new AtomicbaseQueryBuilder_js_1.AtomicbaseQueryBuilder({
            table: table,
            baseUrl: this.baseUrl,
            apiKey: this.apiKey,
            fetch: this.fetchFn,
            headers: this.headers,
        });
    };
    /**
     * Create a new client with a different tenant.
     * Useful for multi-tenant applications.
     *
     * @example
     * ```ts
     * const tenantClient = client.tenant('acme-corp')
     * const { data } = await tenantClient.from('users').select()
     * ```
     */
    AtomicbaseClient.prototype.tenant = function (tenantId) {
        var newClient = new AtomicbaseClient({
            url: this.baseUrl,
            apiKey: this.apiKey,
            fetch: this.fetchFn,
            headers: __assign(__assign({}, this.headers), { Tenant: tenantId }),
        });
        return Object.assign(newClient, { tenantId: tenantId });
    };
    /**
     * Execute multiple operations in a single atomic transaction.
     * All operations succeed or all fail together.
     *
     * Supports all query modifiers including single(), maybeSingle(), count(), and withCount().
     *
     * @example
     * ```ts
     * import { createClient, eq } from '@atomicbase/sdk'
     *
     * const client = createClient({ url: 'http://localhost:8080' })
     *
     * // Insert multiple users and update a counter atomically
     * const { data, error } = await client.batch([
     *   client.from('users').insert({ name: 'Alice', email: 'alice@example.com' }),
     *   client.from('users').insert({ name: 'Bob', email: 'bob@example.com' }),
     *   client.from('counters').update({ count: 2 }).where(eq('id', 1)),
     * ])
     *
     * // With result modifiers
     * const { data, error } = await client.batch([
     *   client.from('users').select().where(eq('id', 1)).single(),
     *   client.from('users').select().count(),
     *   client.from('posts').select().limit(10).withCount(),
     * ])
     * // results[0] = { id: 1, name: 'Alice' }  (single object, not array)
     * // results[1] = 42  (just the count number)
     * // results[2] = { data: [...], count: 100 }  (data with count)
     * ```
     */
    AtomicbaseClient.prototype.batch = function (queries) {
        return __awaiter(this, void 0, void 0, function () {
            var operations, headers, response, errorBody, error, rawData, rawResults, processedResults, err_1, error;
            return __generator(this, function (_a) {
                switch (_a.label) {
                    case 0:
                        operations = queries.map(function (q) { return q.toBatchOperation(); });
                        headers = __assign({ "Content-Type": "application/json" }, this.headers);
                        if (this.apiKey) {
                            headers["Authorization"] = "Bearer ".concat(this.apiKey);
                        }
                        _a.label = 1;
                    case 1:
                        _a.trys.push([1, 6, , 7]);
                        return [4 /*yield*/, this.fetchFn("".concat(this.baseUrl, "/data/batch"), {
                                method: "POST",
                                headers: headers,
                                body: JSON.stringify({ operations: operations }),
                            })];
                    case 2:
                        response = _a.sent();
                        if (!!response.ok) return [3 /*break*/, 4];
                        return [4 /*yield*/, response.json().catch(function () { return ({}); })];
                    case 3:
                        errorBody = _a.sent();
                        error = AtomicbaseError_js_1.AtomicbaseError.fromResponse(errorBody, response.status);
                        return [2 /*return*/, { data: null, error: error }];
                    case 4: return [4 /*yield*/, response.json()];
                    case 5:
                        rawData = _a.sent();
                        rawResults = rawData.results;
                        processedResults = rawResults.map(function (result, index) {
                            var _a, _b;
                            var op = operations[index];
                            if (!op)
                                return result;
                            var resultMode = op.resultMode;
                            // Handle count mode - backend returns { data: [...], count: N }
                            if (resultMode === "count") {
                                var r = result;
                                return (_a = r.count) !== null && _a !== void 0 ? _a : 0;
                            }
                            // Handle withCount - return as-is (already has data + count)
                            if (resultMode === "withCount") {
                                return result;
                            }
                            // Handle single - extract first item, error if 0 or >1
                            if (resultMode === "single") {
                                var data = result;
                                if (!data || data.length === 0) {
                                    // Return error indicator - in batch we can't throw per-operation
                                    return { __error: "NOT_FOUND", message: "No rows returned" };
                                }
                                if (data.length > 1) {
                                    return { __error: "MULTIPLE_ROWS", message: "Multiple rows returned" };
                                }
                                return data[0];
                            }
                            // Handle maybeSingle - extract first item or null
                            if (resultMode === "maybeSingle") {
                                var data = result;
                                return (_b = data === null || data === void 0 ? void 0 : data[0]) !== null && _b !== void 0 ? _b : null;
                            }
                            // Default - return as-is
                            return result;
                        });
                        return [2 /*return*/, { data: { results: processedResults }, error: null }];
                    case 6:
                        err_1 = _a.sent();
                        error = AtomicbaseError_js_1.AtomicbaseError.networkError(err_1);
                        return [2 /*return*/, { data: null, error: error }];
                    case 7: return [2 /*return*/];
                }
            });
        });
    };
    return AtomicbaseClient;
}());
exports.AtomicbaseClient = AtomicbaseClient;
/**
 * Create an Atomicbase client.
 *
 * @example
 * ```ts
 * const client = createClient({
 *   url: 'http://localhost:8080',
 *   apiKey: 'your-api-key',
 * })
 * ```
 */
function createClient(options) {
    return new AtomicbaseClient(options);
}
