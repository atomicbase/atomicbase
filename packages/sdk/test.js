"use strict";
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
var index_js_1 = require("./src/index.js");
var client = (0, index_js_1.createClient)({
    url: "http://localhost:8080",
});
function test(name, fn) {
    return __awaiter(this, void 0, void 0, function () {
        var err_1;
        return __generator(this, function (_a) {
            switch (_a.label) {
                case 0:
                    _a.trys.push([0, 2, , 3]);
                    return [4 /*yield*/, fn()];
                case 1:
                    _a.sent();
                    console.log("\u2713 ".concat(name));
                    return [3 /*break*/, 3];
                case 2:
                    err_1 = _a.sent();
                    console.log("\u2717 ".concat(name));
                    console.error("  ", err_1);
                    return [3 /*break*/, 3];
                case 3: return [2 /*return*/];
            }
        });
    });
}
function run() {
    return __awaiter(this, void 0, void 0, function () {
        var _this = this;
        return __generator(this, function (_a) {
            switch (_a.label) {
                case 0:
                    console.log("\n=== SDK Integration Tests ===\n");
                    // Clean up any existing test data
                    return [4 /*yield*/, client.from("sdk_test").delete().where((0, index_js_1.gt)("id", 0))];
                case 1:
                    // Clean up any existing test data
                    _a.sent();
                    // Test INSERT
                    return [4 /*yield*/, test("Insert single row", function () { return __awaiter(_this, void 0, void 0, function () {
                            var _a, data, error;
                            return __generator(this, function (_b) {
                                switch (_b.label) {
                                    case 0: return [4 /*yield*/, client
                                            .from("sdk_test")
                                            .insert({ id: 1, name: "Alice", email: "alice@example.com", age: 30 })];
                                    case 1:
                                        _a = _b.sent(), data = _a.data, error = _a.error;
                                        if (error)
                                            throw new Error(error.message);
                                        if (!data || typeof data.last_insert_id !== "number") {
                                            throw new Error("Expected last_insert_id");
                                        }
                                        return [2 /*return*/];
                                }
                            });
                        }); })];
                case 2:
                    // Test INSERT
                    _a.sent();
                    return [4 /*yield*/, test("Insert second row", function () { return __awaiter(_this, void 0, void 0, function () {
                            var _a, data, error;
                            return __generator(this, function (_b) {
                                switch (_b.label) {
                                    case 0: return [4 /*yield*/, client
                                            .from("sdk_test")
                                            .insert({ id: 2, name: "Bob", email: "bob@example.com", age: 25 })];
                                    case 1:
                                        _a = _b.sent(), data = _a.data, error = _a.error;
                                        if (error)
                                            throw new Error(error.message);
                                        return [2 /*return*/];
                                }
                            });
                        }); })];
                case 3:
                    _a.sent();
                    return [4 /*yield*/, test("Insert third row", function () { return __awaiter(_this, void 0, void 0, function () {
                            var _a, data, error;
                            return __generator(this, function (_b) {
                                switch (_b.label) {
                                    case 0: return [4 /*yield*/, client
                                            .from("sdk_test")
                                            .insert({ id: 3, name: "Charlie", email: "charlie@example.com", age: 35 })];
                                    case 1:
                                        _a = _b.sent(), data = _a.data, error = _a.error;
                                        if (error)
                                            throw new Error(error.message);
                                        return [2 /*return*/];
                                }
                            });
                        }); })];
                case 4:
                    _a.sent();
                    // Test SELECT
                    return [4 /*yield*/, test("Select all", function () { return __awaiter(_this, void 0, void 0, function () {
                            var _a, data, error;
                            return __generator(this, function (_b) {
                                switch (_b.label) {
                                    case 0: return [4 /*yield*/, client.from("sdk_test").select()];
                                    case 1:
                                        _a = _b.sent(), data = _a.data, error = _a.error;
                                        if (error)
                                            throw new Error(error.message);
                                        if (!Array.isArray(data) || data.length !== 3) {
                                            throw new Error("Expected 3 rows, got ".concat(data === null || data === void 0 ? void 0 : data.length));
                                        }
                                        return [2 /*return*/];
                                }
                            });
                        }); })];
                case 5:
                    // Test SELECT
                    _a.sent();
                    return [4 /*yield*/, test("Select specific columns", function () { return __awaiter(_this, void 0, void 0, function () {
                            var _a, data, error;
                            return __generator(this, function (_b) {
                                switch (_b.label) {
                                    case 0: return [4 /*yield*/, client.from("sdk_test").select("id", "name")];
                                    case 1:
                                        _a = _b.sent(), data = _a.data, error = _a.error;
                                        if (error)
                                            throw new Error(error.message);
                                        if (!data || !data[0] || !("id" in data[0]) || !("name" in data[0])) {
                                            throw new Error("Expected id and name columns");
                                        }
                                        return [2 /*return*/];
                                }
                            });
                        }); })];
                case 6:
                    _a.sent();
                    return [4 /*yield*/, test("Select with where eq", function () { return __awaiter(_this, void 0, void 0, function () {
                            var _a, data, error;
                            return __generator(this, function (_b) {
                                switch (_b.label) {
                                    case 0: return [4 /*yield*/, client
                                            .from("sdk_test")
                                            .select()
                                            .where((0, index_js_1.eq)("name", "Alice"))];
                                    case 1:
                                        _a = _b.sent(), data = _a.data, error = _a.error;
                                        if (error)
                                            throw new Error(error.message);
                                        if (!data || data.length !== 1 || data[0].name !== "Alice") {
                                            throw new Error("Expected Alice");
                                        }
                                        return [2 /*return*/];
                                }
                            });
                        }); })];
                case 7:
                    _a.sent();
                    return [4 /*yield*/, test("Select with where gt", function () { return __awaiter(_this, void 0, void 0, function () {
                            var _a, data, error;
                            return __generator(this, function (_b) {
                                switch (_b.label) {
                                    case 0: return [4 /*yield*/, client
                                            .from("sdk_test")
                                            .select()
                                            .where((0, index_js_1.gt)("age", 28))];
                                    case 1:
                                        _a = _b.sent(), data = _a.data, error = _a.error;
                                        if (error)
                                            throw new Error(error.message);
                                        if (!data || data.length !== 2) {
                                            throw new Error("Expected 2 rows (age > 28), got ".concat(data === null || data === void 0 ? void 0 : data.length));
                                        }
                                        return [2 /*return*/];
                                }
                            });
                        }); })];
                case 8:
                    _a.sent();
                    return [4 /*yield*/, test("Select with where or", function () { return __awaiter(_this, void 0, void 0, function () {
                            var _a, data, error;
                            return __generator(this, function (_b) {
                                switch (_b.label) {
                                    case 0: return [4 /*yield*/, client
                                            .from("sdk_test")
                                            .select()
                                            .where((0, index_js_1.or)((0, index_js_1.eq)("name", "Alice"), (0, index_js_1.eq)("name", "Bob")))];
                                    case 1:
                                        _a = _b.sent(), data = _a.data, error = _a.error;
                                        if (error)
                                            throw new Error(error.message);
                                        if (!data || data.length !== 2) {
                                            throw new Error("Expected 2 rows, got ".concat(data === null || data === void 0 ? void 0 : data.length));
                                        }
                                        return [2 /*return*/];
                                }
                            });
                        }); })];
                case 9:
                    _a.sent();
                    return [4 /*yield*/, test("Select with where inArray", function () { return __awaiter(_this, void 0, void 0, function () {
                            var _a, data, error;
                            return __generator(this, function (_b) {
                                switch (_b.label) {
                                    case 0: return [4 /*yield*/, client
                                            .from("sdk_test")
                                            .select()
                                            .where((0, index_js_1.inArray)("id", [1, 2]))];
                                    case 1:
                                        _a = _b.sent(), data = _a.data, error = _a.error;
                                        if (error)
                                            throw new Error(error.message);
                                        if (!data || data.length !== 2) {
                                            throw new Error("Expected 2 rows, got ".concat(data === null || data === void 0 ? void 0 : data.length));
                                        }
                                        return [2 /*return*/];
                                }
                            });
                        }); })];
                case 10:
                    _a.sent();
                    return [4 /*yield*/, test("Select with orderBy", function () { return __awaiter(_this, void 0, void 0, function () {
                            var _a, data, error;
                            return __generator(this, function (_b) {
                                switch (_b.label) {
                                    case 0: return [4 /*yield*/, client
                                            .from("sdk_test")
                                            .select()
                                            .orderBy("age", "desc")];
                                    case 1:
                                        _a = _b.sent(), data = _a.data, error = _a.error;
                                        if (error)
                                            throw new Error(error.message);
                                        if (!data || data[0].name !== "Charlie") {
                                            throw new Error("Expected Charlie first (oldest)");
                                        }
                                        return [2 /*return*/];
                                }
                            });
                        }); })];
                case 11:
                    _a.sent();
                    return [4 /*yield*/, test("Select with limit", function () { return __awaiter(_this, void 0, void 0, function () {
                            var _a, data, error;
                            return __generator(this, function (_b) {
                                switch (_b.label) {
                                    case 0: return [4 /*yield*/, client.from("sdk_test").select().limit(2)];
                                    case 1:
                                        _a = _b.sent(), data = _a.data, error = _a.error;
                                        if (error)
                                            throw new Error(error.message);
                                        if (!data || data.length !== 2) {
                                            throw new Error("Expected 2 rows, got ".concat(data === null || data === void 0 ? void 0 : data.length));
                                        }
                                        return [2 /*return*/];
                                }
                            });
                        }); })];
                case 12:
                    _a.sent();
                    return [4 /*yield*/, test("Select with limit and offset", function () { return __awaiter(_this, void 0, void 0, function () {
                            var _a, data, error;
                            return __generator(this, function (_b) {
                                switch (_b.label) {
                                    case 0: return [4 /*yield*/, client
                                            .from("sdk_test")
                                            .select()
                                            .orderBy("id", "asc")
                                            .limit(1)
                                            .offset(1)];
                                    case 1:
                                        _a = _b.sent(), data = _a.data, error = _a.error;
                                        if (error)
                                            throw new Error(error.message);
                                        if (!data || data.length !== 1 || data[0].id !== 2) {
                                            throw new Error("Expected Bob (id=2), got ".concat(JSON.stringify(data)));
                                        }
                                        return [2 /*return*/];
                                }
                            });
                        }); })];
                case 13:
                    _a.sent();
                    return [4 /*yield*/, test("Select single()", function () { return __awaiter(_this, void 0, void 0, function () {
                            var _a, data, error;
                            return __generator(this, function (_b) {
                                switch (_b.label) {
                                    case 0: return [4 /*yield*/, client
                                            .from("sdk_test")
                                            .select()
                                            .where((0, index_js_1.eq)("id", 1))
                                            .single()];
                                    case 1:
                                        _a = _b.sent(), data = _a.data, error = _a.error;
                                        if (error)
                                            throw new Error(error.message);
                                        if (!data || data.id !== 1) {
                                            throw new Error("Expected single row with id=1");
                                        }
                                        return [2 /*return*/];
                                }
                            });
                        }); })];
                case 14:
                    _a.sent();
                    return [4 /*yield*/, test("Select single() returns error for no rows", function () { return __awaiter(_this, void 0, void 0, function () {
                            var _a, data, error;
                            return __generator(this, function (_b) {
                                switch (_b.label) {
                                    case 0: return [4 /*yield*/, client
                                            .from("sdk_test")
                                            .select()
                                            .where((0, index_js_1.eq)("id", 999))
                                            .single()];
                                    case 1:
                                        _a = _b.sent(), data = _a.data, error = _a.error;
                                        if (!error || error.code !== "NOT_FOUND") {
                                            throw new Error("Expected NOT_FOUND error");
                                        }
                                        return [2 /*return*/];
                                }
                            });
                        }); })];
                case 15:
                    _a.sent();
                    return [4 /*yield*/, test("Select maybeSingle()", function () { return __awaiter(_this, void 0, void 0, function () {
                            var _a, data, error;
                            return __generator(this, function (_b) {
                                switch (_b.label) {
                                    case 0: return [4 /*yield*/, client
                                            .from("sdk_test")
                                            .select()
                                            .where((0, index_js_1.eq)("id", 999))
                                            .maybeSingle()];
                                    case 1:
                                        _a = _b.sent(), data = _a.data, error = _a.error;
                                        if (error)
                                            throw new Error(error.message);
                                        if (data !== null) {
                                            throw new Error("Expected null for no rows");
                                        }
                                        return [2 /*return*/];
                                }
                            });
                        }); })];
                case 16:
                    _a.sent();
                    return [4 /*yield*/, test("Select count()", function () { return __awaiter(_this, void 0, void 0, function () {
                            var _a, data, error;
                            return __generator(this, function (_b) {
                                switch (_b.label) {
                                    case 0: return [4 /*yield*/, client.from("sdk_test").select().count()];
                                    case 1:
                                        _a = _b.sent(), data = _a.data, error = _a.error;
                                        if (error)
                                            throw new Error(error.message);
                                        if (data !== 3) {
                                            throw new Error("Expected count=3, got ".concat(data));
                                        }
                                        return [2 /*return*/];
                                }
                            });
                        }); })];
                case 17:
                    _a.sent();
                    return [4 /*yield*/, test("Select withCount()", function () { return __awaiter(_this, void 0, void 0, function () {
                            var _a, data, count, error;
                            return __generator(this, function (_b) {
                                switch (_b.label) {
                                    case 0: return [4 /*yield*/, client
                                            .from("sdk_test")
                                            .select()
                                            .limit(2)
                                            .withCount()];
                                    case 1:
                                        _a = _b.sent(), data = _a.data, count = _a.count, error = _a.error;
                                        if (error)
                                            throw new Error(error.message);
                                        if ((data === null || data === void 0 ? void 0 : data.length) !== 2) {
                                            throw new Error("Expected 2 data rows, got ".concat(data === null || data === void 0 ? void 0 : data.length));
                                        }
                                        if (count !== 3) {
                                            throw new Error("Expected count=3, got ".concat(count));
                                        }
                                        return [2 /*return*/];
                                }
                            });
                        }); })];
                case 18:
                    _a.sent();
                    // NEW: Test throwOnError()
                    return [4 /*yield*/, test("throwOnError() throws on error", function () { return __awaiter(_this, void 0, void 0, function () {
                            var err_2;
                            return __generator(this, function (_a) {
                                switch (_a.label) {
                                    case 0:
                                        _a.trys.push([0, 2, , 3]);
                                        return [4 /*yield*/, client
                                                .from("nonexistent_table")
                                                .select()
                                                .throwOnError()];
                                    case 1:
                                        _a.sent();
                                        throw new Error("Expected error to be thrown");
                                    case 2:
                                        err_2 = _a.sent();
                                        if (!(err_2 instanceof index_js_1.AtomicbaseError)) {
                                            throw new Error("Expected AtomicbaseError, got ".concat(err_2));
                                        }
                                        return [3 /*break*/, 3];
                                    case 3: return [2 /*return*/];
                                }
                            });
                        }); })];
                case 19:
                    // NEW: Test throwOnError()
                    _a.sent();
                    return [4 /*yield*/, test("throwOnError() returns data on success", function () { return __awaiter(_this, void 0, void 0, function () {
                            var _a, data, error;
                            return __generator(this, function (_b) {
                                switch (_b.label) {
                                    case 0: return [4 /*yield*/, client
                                            .from("sdk_test")
                                            .select()
                                            .limit(1)
                                            .throwOnError()];
                                    case 1:
                                        _a = _b.sent(), data = _a.data, error = _a.error;
                                        // No error thrown, data should be returned
                                        if (!data || data.length !== 1) {
                                            throw new Error("Expected 1 row, got ".concat(data === null || data === void 0 ? void 0 : data.length));
                                        }
                                        return [2 /*return*/];
                                }
                            });
                        }); })];
                case 20:
                    _a.sent();
                    // NEW: Test abortSignal()
                    return [4 /*yield*/, test("abortSignal() cancels request", function () { return __awaiter(_this, void 0, void 0, function () {
                            var controller, _a, data, error;
                            return __generator(this, function (_b) {
                                switch (_b.label) {
                                    case 0:
                                        controller = new AbortController();
                                        // Abort immediately
                                        controller.abort();
                                        return [4 /*yield*/, client
                                                .from("sdk_test")
                                                .select()
                                                .abortSignal(controller.signal)];
                                    case 1:
                                        _a = _b.sent(), data = _a.data, error = _a.error;
                                        if (!error || error.code !== "ABORTED") {
                                            throw new Error("Expected ABORTED error, got ".concat(error === null || error === void 0 ? void 0 : error.code));
                                        }
                                        return [2 /*return*/];
                                }
                            });
                        }); })];
                case 21:
                    // NEW: Test abortSignal()
                    _a.sent();
                    // Test UPDATE
                    return [4 /*yield*/, test("Update with where", function () { return __awaiter(_this, void 0, void 0, function () {
                            var _a, data, error;
                            return __generator(this, function (_b) {
                                switch (_b.label) {
                                    case 0: return [4 /*yield*/, client
                                            .from("sdk_test")
                                            .update({ status: "inactive" })
                                            .where((0, index_js_1.eq)("id", 1))];
                                    case 1:
                                        _a = _b.sent(), data = _a.data, error = _a.error;
                                        if (error)
                                            throw new Error(error.message);
                                        if (!data || data.rows_affected !== 1) {
                                            throw new Error("Expected 1 row affected, got ".concat(data === null || data === void 0 ? void 0 : data.rows_affected));
                                        }
                                        return [2 /*return*/];
                                }
                            });
                        }); })];
                case 22:
                    // Test UPDATE
                    _a.sent();
                    return [4 /*yield*/, test("Verify update worked", function () { return __awaiter(_this, void 0, void 0, function () {
                            var _a, data, error;
                            return __generator(this, function (_b) {
                                switch (_b.label) {
                                    case 0: return [4 /*yield*/, client
                                            .from("sdk_test")
                                            .select("status")
                                            .where((0, index_js_1.eq)("id", 1))
                                            .single()];
                                    case 1:
                                        _a = _b.sent(), data = _a.data, error = _a.error;
                                        if (error)
                                            throw new Error(error.message);
                                        if ((data === null || data === void 0 ? void 0 : data.status) !== "inactive") {
                                            throw new Error("Expected status=inactive, got ".concat(data === null || data === void 0 ? void 0 : data.status));
                                        }
                                        return [2 /*return*/];
                                }
                            });
                        }); })];
                case 23:
                    _a.sent();
                    // Test UPSERT
                    return [4 /*yield*/, test("Upsert existing row", function () { return __awaiter(_this, void 0, void 0, function () {
                            var _a, data, error;
                            return __generator(this, function (_b) {
                                switch (_b.label) {
                                    case 0: return [4 /*yield*/, client
                                            .from("sdk_test")
                                            .upsert({ id: 1, name: "Alice Updated", email: "alice@example.com", age: 31 })];
                                    case 1:
                                        _a = _b.sent(), data = _a.data, error = _a.error;
                                        if (error)
                                            throw new Error(error.message);
                                        return [2 /*return*/];
                                }
                            });
                        }); })];
                case 24:
                    // Test UPSERT
                    _a.sent();
                    return [4 /*yield*/, test("Verify upsert worked", function () { return __awaiter(_this, void 0, void 0, function () {
                            var _a, data, error;
                            return __generator(this, function (_b) {
                                switch (_b.label) {
                                    case 0: return [4 /*yield*/, client
                                            .from("sdk_test")
                                            .select("name", "age")
                                            .where((0, index_js_1.eq)("id", 1))
                                            .single()];
                                    case 1:
                                        _a = _b.sent(), data = _a.data, error = _a.error;
                                        if (error)
                                            throw new Error(error.message);
                                        if ((data === null || data === void 0 ? void 0 : data.name) !== "Alice Updated" || (data === null || data === void 0 ? void 0 : data.age) !== 31) {
                                            throw new Error("Expected Alice Updated/31, got ".concat(data === null || data === void 0 ? void 0 : data.name, "/").concat(data === null || data === void 0 ? void 0 : data.age));
                                        }
                                        return [2 /*return*/];
                                }
                            });
                        }); })];
                case 25:
                    _a.sent();
                    return [4 /*yield*/, test("Upsert new row", function () { return __awaiter(_this, void 0, void 0, function () {
                            var _a, data, error;
                            return __generator(this, function (_b) {
                                switch (_b.label) {
                                    case 0: return [4 /*yield*/, client
                                            .from("sdk_test")
                                            .upsert({ id: 4, name: "Diana", email: "diana@example.com", age: 28 })];
                                    case 1:
                                        _a = _b.sent(), data = _a.data, error = _a.error;
                                        if (error)
                                            throw new Error(error.message);
                                        return [2 /*return*/];
                                }
                            });
                        }); })];
                case 26:
                    _a.sent();
                    return [4 /*yield*/, test("Verify new row inserted", function () { return __awaiter(_this, void 0, void 0, function () {
                            var _a, data, error;
                            return __generator(this, function (_b) {
                                switch (_b.label) {
                                    case 0: return [4 /*yield*/, client.from("sdk_test").select().count()];
                                    case 1:
                                        _a = _b.sent(), data = _a.data, error = _a.error;
                                        if (error)
                                            throw new Error(error.message);
                                        if (data !== 4) {
                                            throw new Error("Expected 4 rows, got ".concat(data));
                                        }
                                        return [2 /*return*/];
                                }
                            });
                        }); })];
                case 27:
                    _a.sent();
                    // Test INSERT with returning
                    return [4 /*yield*/, test("Insert with returning", function () { return __awaiter(_this, void 0, void 0, function () {
                            var _a, data, error;
                            return __generator(this, function (_b) {
                                switch (_b.label) {
                                    case 0: return [4 /*yield*/, client
                                            .from("sdk_test")
                                            .insert({ id: 5, name: "Eve", email: "eve@example.com", age: 22 })
                                            .returning("id", "name")];
                                    case 1:
                                        _a = _b.sent(), data = _a.data, error = _a.error;
                                        if (error)
                                            throw new Error(error.message);
                                        // returned data should be the rows
                                        if (!Array.isArray(data) || data.length !== 1 || data[0].name !== "Eve") {
                                            throw new Error("Expected returned row with Eve, got ".concat(JSON.stringify(data)));
                                        }
                                        return [2 /*return*/];
                                }
                            });
                        }); })];
                case 28:
                    // Test INSERT with returning
                    _a.sent();
                    // Test DELETE
                    return [4 /*yield*/, test("Delete with where", function () { return __awaiter(_this, void 0, void 0, function () {
                            var _a, data, error;
                            return __generator(this, function (_b) {
                                switch (_b.label) {
                                    case 0: return [4 /*yield*/, client
                                            .from("sdk_test")
                                            .delete()
                                            .where((0, index_js_1.eq)("id", 5))];
                                    case 1:
                                        _a = _b.sent(), data = _a.data, error = _a.error;
                                        if (error)
                                            throw new Error(error.message);
                                        if (!data || data.rows_affected !== 1) {
                                            throw new Error("Expected 1 row affected, got ".concat(data === null || data === void 0 ? void 0 : data.rows_affected));
                                        }
                                        return [2 /*return*/];
                                }
                            });
                        }); })];
                case 29:
                    // Test DELETE
                    _a.sent();
                    return [4 /*yield*/, test("Verify delete worked", function () { return __awaiter(_this, void 0, void 0, function () {
                            var _a, data, error;
                            return __generator(this, function (_b) {
                                switch (_b.label) {
                                    case 0: return [4 /*yield*/, client.from("sdk_test").select().count()];
                                    case 1:
                                        _a = _b.sent(), data = _a.data, error = _a.error;
                                        if (error)
                                            throw new Error(error.message);
                                        if (data !== 4) {
                                            throw new Error("Expected 4 rows, got ".concat(data));
                                        }
                                        return [2 /*return*/];
                                }
                            });
                        }); })];
                case 30:
                    _a.sent();
                    // =========================================================================
                    // BATCH TESTS
                    // =========================================================================
                    console.log("\n=== Batch API Tests ===\n");
                    // Clean up and set up fresh data for batch tests
                    return [4 /*yield*/, client.from("sdk_test").delete().where((0, index_js_1.gt)("id", 0))];
                case 31:
                    // Clean up and set up fresh data for batch tests
                    _a.sent();
                    return [4 /*yield*/, client.from("sdk_test").insert({ id: 1, name: "Alice", email: "alice@example.com", age: 30 })];
                case 32:
                    _a.sent();
                    return [4 /*yield*/, client.from("sdk_test").insert({ id: 2, name: "Bob", email: "bob@example.com", age: 25 })];
                case 33:
                    _a.sent();
                    return [4 /*yield*/, client.from("sdk_test").insert({ id: 3, name: "Charlie", email: "charlie@example.com", age: 35 })];
                case 34:
                    _a.sent();
                    // Test basic batch with multiple inserts
                    return [4 /*yield*/, test("Batch: multiple inserts", function () { return __awaiter(_this, void 0, void 0, function () {
                            var _a, data, error, i, result;
                            return __generator(this, function (_b) {
                                switch (_b.label) {
                                    case 0: return [4 /*yield*/, client.batch([
                                            client.from("sdk_test").insert({ id: 10, name: "User10", email: "user10@test.com", age: 20 }),
                                            client.from("sdk_test").insert({ id: 11, name: "User11", email: "user11@test.com", age: 21 }),
                                            client.from("sdk_test").insert({ id: 12, name: "User12", email: "user12@test.com", age: 22 }),
                                        ])];
                                    case 1:
                                        _a = _b.sent(), data = _a.data, error = _a.error;
                                        if (error)
                                            throw new Error(error.message);
                                        if (!data || data.results.length !== 3) {
                                            throw new Error("Expected 3 results, got ".concat(data === null || data === void 0 ? void 0 : data.results.length));
                                        }
                                        // Each insert should return last_insert_id
                                        for (i = 0; i < 3; i++) {
                                            result = data.results[i];
                                            if (typeof result.last_insert_id !== "number") {
                                                throw new Error("Expected last_insert_id in result ".concat(i, ", got ").concat(JSON.stringify(result)));
                                            }
                                        }
                                        return [2 /*return*/];
                                }
                            });
                        }); })];
                case 35:
                    // Test basic batch with multiple inserts
                    _a.sent();
                    // Test batch with mixed operations
                    return [4 /*yield*/, test("Batch: mixed insert, update, delete", function () { return __awaiter(_this, void 0, void 0, function () {
                            var _a, data, error, r0, r1, r2;
                            return __generator(this, function (_b) {
                                switch (_b.label) {
                                    case 0: return [4 /*yield*/, client.batch([
                                            client.from("sdk_test").insert({ id: 20, name: "ToUpdate", email: "toupdate@test.com", age: 30 }),
                                            client.from("sdk_test").update({ name: "User10Updated" }).where((0, index_js_1.eq)("id", 10)),
                                            client.from("sdk_test").delete().where((0, index_js_1.eq)("id", 12)),
                                        ])];
                                    case 1:
                                        _a = _b.sent(), data = _a.data, error = _a.error;
                                        if (error)
                                            throw new Error(error.message);
                                        if (!data || data.results.length !== 3) {
                                            throw new Error("Expected 3 results, got ".concat(data === null || data === void 0 ? void 0 : data.results.length));
                                        }
                                        r0 = data.results[0];
                                        if (typeof r0.last_insert_id !== "number") {
                                            throw new Error("Expected last_insert_id, got ".concat(JSON.stringify(r0)));
                                        }
                                        r1 = data.results[1];
                                        if (r1.rows_affected !== 1) {
                                            throw new Error("Expected 1 row affected, got ".concat(r1.rows_affected));
                                        }
                                        r2 = data.results[2];
                                        if (r2.rows_affected !== 1) {
                                            throw new Error("Expected 1 row deleted, got ".concat(r2.rows_affected));
                                        }
                                        return [2 /*return*/];
                                }
                            });
                        }); })];
                case 36:
                    // Test batch with mixed operations
                    _a.sent();
                    // Test batch with select
                    return [4 /*yield*/, test("Batch: select operations", function () { return __awaiter(_this, void 0, void 0, function () {
                            var _a, data, error, r0, r1;
                            return __generator(this, function (_b) {
                                switch (_b.label) {
                                    case 0: return [4 /*yield*/, client.batch([
                                            client.from("sdk_test").select("id", "name").where((0, index_js_1.eq)("id", 1)),
                                            client.from("sdk_test").select("id", "name").where((0, index_js_1.eq)("id", 2)),
                                        ])];
                                    case 1:
                                        _a = _b.sent(), data = _a.data, error = _a.error;
                                        if (error)
                                            throw new Error(error.message);
                                        if (!data || data.results.length !== 2) {
                                            throw new Error("Expected 2 results, got ".concat(data === null || data === void 0 ? void 0 : data.results.length));
                                        }
                                        r0 = data.results[0];
                                        r1 = data.results[1];
                                        if (!Array.isArray(r0) || r0.length !== 1 || r0[0].name !== "Alice") {
                                            throw new Error("Expected Alice, got ".concat(JSON.stringify(r0)));
                                        }
                                        if (!Array.isArray(r1) || r1.length !== 1 || r1[0].name !== "Bob") {
                                            throw new Error("Expected Bob, got ".concat(JSON.stringify(r1)));
                                        }
                                        return [2 /*return*/];
                                }
                            });
                        }); })];
                case 37:
                    // Test batch with select
                    _a.sent();
                    // Test batch with single()
                    return [4 /*yield*/, test("Batch: single() modifier", function () { return __awaiter(_this, void 0, void 0, function () {
                            var _a, data, error, r0, r1;
                            return __generator(this, function (_b) {
                                switch (_b.label) {
                                    case 0: return [4 /*yield*/, client.batch([
                                            client.from("sdk_test").select().where((0, index_js_1.eq)("id", 1)).single(),
                                            client.from("sdk_test").select().where((0, index_js_1.eq)("id", 2)).single(),
                                        ])];
                                    case 1:
                                        _a = _b.sent(), data = _a.data, error = _a.error;
                                        if (error)
                                            throw new Error(error.message);
                                        if (!data || data.results.length !== 2) {
                                            throw new Error("Expected 2 results, got ".concat(data === null || data === void 0 ? void 0 : data.results.length));
                                        }
                                        r0 = data.results[0];
                                        r1 = data.results[1];
                                        if (Array.isArray(r0) || r0.id !== 1 || r0.name !== "Alice") {
                                            throw new Error("Expected single Alice object, got ".concat(JSON.stringify(r0)));
                                        }
                                        if (Array.isArray(r1) || r1.id !== 2 || r1.name !== "Bob") {
                                            throw new Error("Expected single Bob object, got ".concat(JSON.stringify(r1)));
                                        }
                                        return [2 /*return*/];
                                }
                            });
                        }); })];
                case 38:
                    // Test batch with single()
                    _a.sent();
                    // Test batch with single() returning NOT_FOUND
                    return [4 /*yield*/, test("Batch: single() with no rows returns error object", function () { return __awaiter(_this, void 0, void 0, function () {
                            var _a, data, error, r0, r1;
                            return __generator(this, function (_b) {
                                switch (_b.label) {
                                    case 0: return [4 /*yield*/, client.batch([
                                            client.from("sdk_test").select().where((0, index_js_1.eq)("id", 1)).single(),
                                            client.from("sdk_test").select().where((0, index_js_1.eq)("id", 99999)).single(), // No rows
                                        ])];
                                    case 1:
                                        _a = _b.sent(), data = _a.data, error = _a.error;
                                        if (error)
                                            throw new Error(error.message);
                                        if (!data || data.results.length !== 2) {
                                            throw new Error("Expected 2 results, got ".concat(data === null || data === void 0 ? void 0 : data.results.length));
                                        }
                                        r0 = data.results[0];
                                        if (r0.id !== 1) {
                                            throw new Error("Expected Alice, got ".concat(JSON.stringify(r0)));
                                        }
                                        r1 = data.results[1];
                                        if (r1.__error !== "NOT_FOUND") {
                                            throw new Error("Expected NOT_FOUND error, got ".concat(JSON.stringify(r1)));
                                        }
                                        return [2 /*return*/];
                                }
                            });
                        }); })];
                case 39:
                    // Test batch with single() returning NOT_FOUND
                    _a.sent();
                    // Test batch with maybeSingle()
                    return [4 /*yield*/, test("Batch: maybeSingle() modifier", function () { return __awaiter(_this, void 0, void 0, function () {
                            var _a, data, error, r0, r1;
                            return __generator(this, function (_b) {
                                switch (_b.label) {
                                    case 0: return [4 /*yield*/, client.batch([
                                            client.from("sdk_test").select().where((0, index_js_1.eq)("id", 1)).maybeSingle(),
                                            client.from("sdk_test").select().where((0, index_js_1.eq)("id", 99999)).maybeSingle(), // No rows - should return null
                                        ])];
                                    case 1:
                                        _a = _b.sent(), data = _a.data, error = _a.error;
                                        if (error)
                                            throw new Error(error.message);
                                        if (!data || data.results.length !== 2) {
                                            throw new Error("Expected 2 results, got ".concat(data === null || data === void 0 ? void 0 : data.results.length));
                                        }
                                        r0 = data.results[0];
                                        if (r0.id !== 1 || r0.name !== "Alice") {
                                            throw new Error("Expected Alice, got ".concat(JSON.stringify(r0)));
                                        }
                                        r1 = data.results[1];
                                        if (r1 !== null) {
                                            throw new Error("Expected null for no rows, got ".concat(JSON.stringify(r1)));
                                        }
                                        return [2 /*return*/];
                                }
                            });
                        }); })];
                case 40:
                    // Test batch with maybeSingle()
                    _a.sent();
                    // Test batch with count()
                    return [4 /*yield*/, test("Batch: count() modifier", function () { return __awaiter(_this, void 0, void 0, function () {
                            var _a, data, error, totalCount, filteredCount;
                            return __generator(this, function (_b) {
                                switch (_b.label) {
                                    case 0: return [4 /*yield*/, client.batch([
                                            client.from("sdk_test").select().count(),
                                            client.from("sdk_test").select().where((0, index_js_1.gt)("age", 25)).count(),
                                        ])];
                                    case 1:
                                        _a = _b.sent(), data = _a.data, error = _a.error;
                                        if (error)
                                            throw new Error(error.message);
                                        if (!data || data.results.length !== 2) {
                                            throw new Error("Expected 2 results, got ".concat(data === null || data === void 0 ? void 0 : data.results.length));
                                        }
                                        totalCount = data.results[0];
                                        filteredCount = data.results[1];
                                        if (typeof totalCount !== "number" || totalCount < 5) {
                                            throw new Error("Expected total count >= 5, got ".concat(totalCount));
                                        }
                                        if (typeof filteredCount !== "number") {
                                            throw new Error("Expected filtered count to be number, got ".concat(typeof filteredCount));
                                        }
                                        return [2 /*return*/];
                                }
                            });
                        }); })];
                case 41:
                    // Test batch with count()
                    _a.sent();
                    // Test batch with withCount()
                    return [4 /*yield*/, test("Batch: withCount() modifier", function () { return __awaiter(_this, void 0, void 0, function () {
                            var _a, data, error, r0;
                            return __generator(this, function (_b) {
                                switch (_b.label) {
                                    case 0: return [4 /*yield*/, client.batch([
                                            client.from("sdk_test").select("id", "name").limit(2).withCount(),
                                        ])];
                                    case 1:
                                        _a = _b.sent(), data = _a.data, error = _a.error;
                                        if (error)
                                            throw new Error(error.message);
                                        if (!data || data.results.length !== 1) {
                                            throw new Error("Expected 1 result, got ".concat(data === null || data === void 0 ? void 0 : data.results.length));
                                        }
                                        r0 = data.results[0];
                                        if (!r0.data || !Array.isArray(r0.data)) {
                                            throw new Error("Expected data array, got ".concat(JSON.stringify(r0)));
                                        }
                                        if (r0.data.length !== 2) {
                                            throw new Error("Expected 2 data rows (limit), got ".concat(r0.data.length));
                                        }
                                        if (typeof r0.count !== "number" || r0.count < 5) {
                                            throw new Error("Expected count >= 5, got ".concat(r0.count));
                                        }
                                        return [2 /*return*/];
                                }
                            });
                        }); })];
                case 42:
                    // Test batch with withCount()
                    _a.sent();
                    // Test batch atomicity - if one fails, all should rollback
                    return [4 /*yield*/, test("Batch: atomicity - failure rolls back all", function () { return __awaiter(_this, void 0, void 0, function () {
                            var beforeCount, existingId, _a, data, error, afterCount;
                            return __generator(this, function (_b) {
                                switch (_b.label) {
                                    case 0: return [4 /*yield*/, client.from("sdk_test").select().count()];
                                    case 1:
                                        beforeCount = (_b.sent()).data;
                                        existingId = 1;
                                        return [4 /*yield*/, client.batch([
                                                client.from("sdk_test").insert({ id: 100, name: "ShouldRollback", email: "rollback@test.com", age: 50 }),
                                                client.from("sdk_test").insert({ id: existingId, name: "Duplicate", email: "dup@test.com", age: 99 }), // Should fail - duplicate id
                                            ])];
                                    case 2:
                                        _a = _b.sent(), data = _a.data, error = _a.error;
                                        // The batch should fail
                                        if (!error) {
                                            throw new Error("Expected batch to fail due to duplicate id");
                                        }
                                        return [4 /*yield*/, client.from("sdk_test").select().count()];
                                    case 3:
                                        afterCount = (_b.sent()).data;
                                        if (beforeCount !== afterCount) {
                                            throw new Error("Expected count ".concat(beforeCount, " after rollback, got ").concat(afterCount));
                                        }
                                        return [2 /*return*/];
                                }
                            });
                        }); })];
                case 43:
                    // Test batch atomicity - if one fails, all should rollback
                    _a.sent();
                    // Test batch with upsert
                    return [4 /*yield*/, test("Batch: upsert operations", function () { return __awaiter(_this, void 0, void 0, function () {
                            var _a, data, error, alice, newUser;
                            return __generator(this, function (_b) {
                                switch (_b.label) {
                                    case 0: return [4 /*yield*/, client.batch([
                                            client.from("sdk_test").upsert({ id: 1, name: "Alice Upserted", email: "alice@example.com", age: 31 }),
                                            client.from("sdk_test").upsert({ id: 200, name: "NewUser", email: "newuser@test.com", age: 40 }),
                                        ])];
                                    case 1:
                                        _a = _b.sent(), data = _a.data, error = _a.error;
                                        if (error)
                                            throw new Error(error.message);
                                        if (!data || data.results.length !== 2) {
                                            throw new Error("Expected 2 results, got ".concat(data === null || data === void 0 ? void 0 : data.results.length));
                                        }
                                        return [4 /*yield*/, client.from("sdk_test").select().where((0, index_js_1.eq)("id", 1)).single()];
                                    case 2:
                                        alice = (_b.sent()).data;
                                        if ((alice === null || alice === void 0 ? void 0 : alice.name) !== "Alice Upserted") {
                                            throw new Error("Expected 'Alice Upserted', got ".concat(alice === null || alice === void 0 ? void 0 : alice.name));
                                        }
                                        return [4 /*yield*/, client.from("sdk_test").select().where((0, index_js_1.eq)("id", 200)).single()];
                                    case 3:
                                        newUser = (_b.sent()).data;
                                        if ((newUser === null || newUser === void 0 ? void 0 : newUser.name) !== "NewUser") {
                                            throw new Error("Expected 'NewUser', got ".concat(newUser === null || newUser === void 0 ? void 0 : newUser.name));
                                        }
                                        return [2 /*return*/];
                                }
                            });
                        }); })];
                case 44:
                    // Test batch with upsert
                    _a.sent();
                    // Test complex batch combining everything
                    return [4 /*yield*/, test("Batch: complex mixed operations", function () { return __awaiter(_this, void 0, void 0, function () {
                            var _a, data, error, r0, r1, r2, r3, r4, r5, r6, r7;
                            return __generator(this, function (_b) {
                                switch (_b.label) {
                                    case 0: return [4 /*yield*/, client.batch([
                                            client.from("sdk_test").select().where((0, index_js_1.eq)("id", 1)).single(), // 0: get single
                                            client.from("sdk_test").select().count(), // 1: count all
                                            client.from("sdk_test").insert({ id: 300, name: "BatchTest", email: "batch@test.com", age: 33 }), // 2: insert
                                            client.from("sdk_test").select().where((0, index_js_1.eq)("id", 300)).maybeSingle(), // 3: verify insert
                                            client.from("sdk_test").update({ age: 34 }).where((0, index_js_1.eq)("id", 300)), // 4: update
                                            client.from("sdk_test").select("age").where((0, index_js_1.eq)("id", 300)).single(), // 5: verify update
                                            client.from("sdk_test").delete().where((0, index_js_1.eq)("id", 300)), // 6: delete
                                            client.from("sdk_test").select().where((0, index_js_1.eq)("id", 300)).maybeSingle(), // 7: verify delete
                                        ])];
                                    case 1:
                                        _a = _b.sent(), data = _a.data, error = _a.error;
                                        if (error)
                                            throw new Error(error.message);
                                        if (!data || data.results.length !== 8) {
                                            throw new Error("Expected 8 results, got ".concat(data === null || data === void 0 ? void 0 : data.results.length));
                                        }
                                        r0 = data.results[0];
                                        if (r0.id !== 1)
                                            throw new Error("Result 0: Expected id=1, got ".concat(r0.id));
                                        r1 = data.results[1];
                                        if (typeof r1 !== "number")
                                            throw new Error("Result 1: Expected number, got ".concat(typeof r1));
                                        r2 = data.results[2];
                                        if (typeof r2.last_insert_id !== "number")
                                            throw new Error("Result 2: Expected last_insert_id");
                                        r3 = data.results[3];
                                        if ((r3 === null || r3 === void 0 ? void 0 : r3.name) !== "BatchTest")
                                            throw new Error("Result 3: Expected BatchTest, got ".concat(r3 === null || r3 === void 0 ? void 0 : r3.name));
                                        r4 = data.results[4];
                                        if (r4.rows_affected !== 1)
                                            throw new Error("Result 4: Expected 1 row affected");
                                        r5 = data.results[5];
                                        if (r5.age !== 34)
                                            throw new Error("Result 5: Expected age=34, got ".concat(r5.age));
                                        r6 = data.results[6];
                                        if (r6.rows_affected !== 1)
                                            throw new Error("Result 6: Expected 1 row deleted");
                                        r7 = data.results[7];
                                        if (r7 !== null)
                                            throw new Error("Result 7: Expected null after delete, got ".concat(JSON.stringify(r7)));
                                        return [2 /*return*/];
                                }
                            });
                        }); })];
                case 45:
                    // Test complex batch combining everything
                    _a.sent();
                    // =========================================================================
                    // EXPLICIT JOIN TESTS IN BATCH
                    // =========================================================================
                    console.log("\n=== Explicit Join Tests in Batch ===\n");
                    // Set up test data for joins (using users and posts tables)
                    return [4 /*yield*/, client.from("posts").delete().where((0, index_js_1.gt)("id", 0))];
                case 46:
                    // Set up test data for joins (using users and posts tables)
                    _a.sent();
                    return [4 /*yield*/, client.from("users").delete().where((0, index_js_1.gt)("id", 0))];
                case 47:
                    _a.sent();
                    // Insert users
                    return [4 /*yield*/, client.from("users").insert({ id: 1, name: "Alice", email: "alice@join.test", age: 30 })];
                case 48:
                    // Insert users
                    _a.sent();
                    return [4 /*yield*/, client.from("users").insert({ id: 2, name: "Bob", email: "bob@join.test", age: 25 })];
                case 49:
                    _a.sent();
                    return [4 /*yield*/, client.from("users").insert({ id: 3, name: "Charlie", email: "charlie@join.test", age: 35 })];
                case 50:
                    _a.sent(); // No posts
                    // Insert posts
                    return [4 /*yield*/, client.from("posts").insert({ id: 1, title: "Alice Post 1", content: "Content 1", user_id: 1 })];
                case 51:
                    // Insert posts
                    _a.sent();
                    return [4 /*yield*/, client.from("posts").insert({ id: 2, title: "Alice Post 2", content: "Content 2", user_id: 1 })];
                case 52:
                    _a.sent();
                    return [4 /*yield*/, client.from("posts").insert({ id: 3, title: "Bob Post 1", content: "Content 3", user_id: 2 })];
                case 53:
                    _a.sent();
                    // Test explicit LEFT JOIN in batch (flat output)
                    return [4 /*yield*/, test("Batch: explicit LEFT JOIN", function () { return __awaiter(_this, void 0, void 0, function () {
                            var _a, data, error, rows;
                            return __generator(this, function (_b) {
                                switch (_b.label) {
                                    case 0: return [4 /*yield*/, client.batch([
                                            client
                                                .from("users")
                                                .select("users.id", "users.name", "posts.title")
                                                .leftJoin("posts", (0, index_js_1.onEq)("users.id", "posts.user_id"), { flat: true })
                                                .where((0, index_js_1.eq)("users.id", 1)),
                                        ])];
                                    case 1:
                                        _a = _b.sent(), data = _a.data, error = _a.error;
                                        if (error)
                                            throw new Error(error.message);
                                        if (!data || data.results.length !== 1) {
                                            throw new Error("Expected 1 result, got ".concat(data === null || data === void 0 ? void 0 : data.results.length));
                                        }
                                        rows = data.results[0];
                                        if (!Array.isArray(rows)) {
                                            throw new Error("Expected array result, got ".concat(JSON.stringify(rows)));
                                        }
                                        // Alice has 2 posts, so should get 2 rows (flat output)
                                        if (rows.length !== 2) {
                                            throw new Error("Expected 2 rows for Alice's posts, got ".concat(rows.length));
                                        }
                                        if (rows[0].name !== "Alice") {
                                            throw new Error("Expected Alice, got ".concat(rows[0].name));
                                        }
                                        return [2 /*return*/];
                                }
                            });
                        }); })];
                case 54:
                    // Test explicit LEFT JOIN in batch (flat output)
                    _a.sent();
                    // Test explicit INNER JOIN in batch (flat output)
                    return [4 /*yield*/, test("Batch: explicit INNER JOIN", function () { return __awaiter(_this, void 0, void 0, function () {
                            var _a, data, error, rows;
                            return __generator(this, function (_b) {
                                switch (_b.label) {
                                    case 0: return [4 /*yield*/, client.batch([
                                            client
                                                .from("users")
                                                .select("users.id", "users.name", "posts.title")
                                                .innerJoin("posts", (0, index_js_1.onEq)("users.id", "posts.user_id"), { flat: true }),
                                        ])];
                                    case 1:
                                        _a = _b.sent(), data = _a.data, error = _a.error;
                                        if (error)
                                            throw new Error(error.message);
                                        if (!data || data.results.length !== 1) {
                                            throw new Error("Expected 1 result, got ".concat(data === null || data === void 0 ? void 0 : data.results.length));
                                        }
                                        rows = data.results[0];
                                        if (!Array.isArray(rows)) {
                                            throw new Error("Expected array result, got ".concat(JSON.stringify(rows)));
                                        }
                                        // Inner join: only users with posts (Alice=2, Bob=1), Charlie excluded (flat output)
                                        if (rows.length !== 3) {
                                            throw new Error("Expected 3 rows (inner join), got ".concat(rows.length));
                                        }
                                        return [2 /*return*/];
                                }
                            });
                        }); })];
                case 55:
                    // Test explicit INNER JOIN in batch (flat output)
                    _a.sent();
                    // Test LEFT JOIN shows nulls for users without posts (flat output)
                    return [4 /*yield*/, test("Batch: LEFT JOIN includes users without posts", function () { return __awaiter(_this, void 0, void 0, function () {
                            var _a, data, error, rows;
                            return __generator(this, function (_b) {
                                switch (_b.label) {
                                    case 0: return [4 /*yield*/, client.batch([
                                            client
                                                .from("users")
                                                .select("users.id", "users.name", "posts.title")
                                                .leftJoin("posts", (0, index_js_1.onEq)("users.id", "posts.user_id"), { flat: true })
                                                .where((0, index_js_1.eq)("users.id", 3)), // Charlie has no posts
                                        ])];
                                    case 1:
                                        _a = _b.sent(), data = _a.data, error = _a.error;
                                        if (error)
                                            throw new Error(error.message);
                                        rows = data.results[0];
                                        if (rows.length !== 1) {
                                            throw new Error("Expected 1 row for Charlie, got ".concat(rows.length));
                                        }
                                        if (rows[0].name !== "Charlie") {
                                            throw new Error("Expected Charlie, got ".concat(rows[0].name));
                                        }
                                        if (rows[0].posts_title !== null) {
                                            throw new Error("Expected null posts_title for Charlie (no posts), got ".concat(rows[0].posts_title));
                                        }
                                        return [2 /*return*/];
                                }
                            });
                        }); })];
                case 56:
                    // Test LEFT JOIN shows nulls for users without posts (flat output)
                    _a.sent();
                    // Test multiple joins in a single batch (flat output)
                    return [4 /*yield*/, test("Batch: multiple join queries", function () { return __awaiter(_this, void 0, void 0, function () {
                            var _a, data, error, r0, r1, r2;
                            return __generator(this, function (_b) {
                                switch (_b.label) {
                                    case 0: return [4 /*yield*/, client.batch([
                                            // Query 1: Left join for user 1 (flat)
                                            client
                                                .from("users")
                                                .select("users.name", "posts.title")
                                                .leftJoin("posts", (0, index_js_1.onEq)("users.id", "posts.user_id"), { flat: true })
                                                .where((0, index_js_1.eq)("users.id", 1)),
                                            // Query 2: Inner join for all (flat)
                                            client
                                                .from("users")
                                                .select("users.name", "posts.title")
                                                .innerJoin("posts", (0, index_js_1.onEq)("users.id", "posts.user_id"), { flat: true }),
                                            // Query 3: Count with join (flat)
                                            client
                                                .from("users")
                                                .select()
                                                .innerJoin("posts", (0, index_js_1.onEq)("users.id", "posts.user_id"), { flat: true })
                                                .count(),
                                        ])];
                                    case 1:
                                        _a = _b.sent(), data = _a.data, error = _a.error;
                                        if (error)
                                            throw new Error(error.message);
                                        if (!data || data.results.length !== 3) {
                                            throw new Error("Expected 3 results, got ".concat(data === null || data === void 0 ? void 0 : data.results.length));
                                        }
                                        r0 = data.results[0];
                                        if (r0.length !== 2) {
                                            throw new Error("Expected 2 rows for Alice, got ".concat(r0.length));
                                        }
                                        r1 = data.results[1];
                                        if (r1.length !== 3) {
                                            throw new Error("Expected 3 rows for inner join, got ".concat(r1.length));
                                        }
                                        r2 = data.results[2];
                                        if (typeof r2 !== "number" || r2 !== 3) {
                                            throw new Error("Expected count=3, got ".concat(r2));
                                        }
                                        return [2 /*return*/];
                                }
                            });
                        }); })];
                case 57:
                    // Test multiple joins in a single batch (flat output)
                    _a.sent();
                    // Test join with single() modifier (flat output)
                    return [4 /*yield*/, test("Batch: join with single() modifier", function () { return __awaiter(_this, void 0, void 0, function () {
                            var _a, data, error, row;
                            return __generator(this, function (_b) {
                                switch (_b.label) {
                                    case 0: return [4 /*yield*/, client.batch([
                                            client
                                                .from("users")
                                                .select("users.name", "posts.title")
                                                .leftJoin("posts", (0, index_js_1.onEq)("users.id", "posts.user_id"), { flat: true })
                                                .where((0, index_js_1.eq)("posts.id", 1))
                                                .single(),
                                        ])];
                                    case 1:
                                        _a = _b.sent(), data = _a.data, error = _a.error;
                                        if (error)
                                            throw new Error(error.message);
                                        row = data.results[0];
                                        if (Array.isArray(row)) {
                                            throw new Error("Expected single object, got array");
                                        }
                                        if (row.name !== "Alice" || row.posts_title !== "Alice Post 1") {
                                            throw new Error("Expected Alice/Alice Post 1, got ".concat(row.name, "/").concat(row.posts_title));
                                        }
                                        return [2 /*return*/];
                                }
                            });
                        }); })];
                case 58:
                    // Test join with single() modifier (flat output)
                    _a.sent();
                    console.log("\n=== All Tests Complete ===\n");
                    return [2 /*return*/];
            }
        });
    });
}
run().catch(console.error);
