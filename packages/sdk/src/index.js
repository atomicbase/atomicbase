"use strict";
// =============================================================================
// Client
// =============================================================================
Object.defineProperty(exports, "__esModule", { value: true });
exports.and = exports.or = exports.not = exports.fts = exports.isNotNull = exports.isNull = exports.between = exports.notInArray = exports.inArray = exports.glob = exports.like = exports.lte = exports.lt = exports.gte = exports.gt = exports.neq = exports.eq = exports.onLte = exports.onLt = exports.onGte = exports.onGt = exports.onNeq = exports.onEq = exports.col = exports.AtomicbaseError = exports.AtomicbaseQueryBuilder = exports.AtomicbaseTransformBuilder = exports.AtomicbaseBuilder = exports.createClient = exports.AtomicbaseClient = void 0;
var AtomicbaseClient_js_1 = require("./AtomicbaseClient.js");
Object.defineProperty(exports, "AtomicbaseClient", { enumerable: true, get: function () { return AtomicbaseClient_js_1.AtomicbaseClient; } });
Object.defineProperty(exports, "createClient", { enumerable: true, get: function () { return AtomicbaseClient_js_1.createClient; } });
// =============================================================================
// Builders (for advanced usage / extension)
// =============================================================================
var AtomicbaseBuilder_js_1 = require("./AtomicbaseBuilder.js");
Object.defineProperty(exports, "AtomicbaseBuilder", { enumerable: true, get: function () { return AtomicbaseBuilder_js_1.AtomicbaseBuilder; } });
var AtomicbaseTransformBuilder_js_1 = require("./AtomicbaseTransformBuilder.js");
Object.defineProperty(exports, "AtomicbaseTransformBuilder", { enumerable: true, get: function () { return AtomicbaseTransformBuilder_js_1.AtomicbaseTransformBuilder; } });
var AtomicbaseQueryBuilder_js_1 = require("./AtomicbaseQueryBuilder.js");
Object.defineProperty(exports, "AtomicbaseQueryBuilder", { enumerable: true, get: function () { return AtomicbaseQueryBuilder_js_1.AtomicbaseQueryBuilder; } });
// =============================================================================
// Error
// =============================================================================
var AtomicbaseError_js_1 = require("./AtomicbaseError.js");
Object.defineProperty(exports, "AtomicbaseError", { enumerable: true, get: function () { return AtomicbaseError_js_1.AtomicbaseError; } });
// =============================================================================
// Filter Functions
// =============================================================================
var filters_js_1 = require("./filters.js");
// Column reference helper
Object.defineProperty(exports, "col", { enumerable: true, get: function () { return filters_js_1.col; } });
// Join condition functions
Object.defineProperty(exports, "onEq", { enumerable: true, get: function () { return filters_js_1.onEq; } });
Object.defineProperty(exports, "onNeq", { enumerable: true, get: function () { return filters_js_1.onNeq; } });
Object.defineProperty(exports, "onGt", { enumerable: true, get: function () { return filters_js_1.onGt; } });
Object.defineProperty(exports, "onGte", { enumerable: true, get: function () { return filters_js_1.onGte; } });
Object.defineProperty(exports, "onLt", { enumerable: true, get: function () { return filters_js_1.onLt; } });
Object.defineProperty(exports, "onLte", { enumerable: true, get: function () { return filters_js_1.onLte; } });
// WHERE filter functions
Object.defineProperty(exports, "eq", { enumerable: true, get: function () { return filters_js_1.eq; } });
Object.defineProperty(exports, "neq", { enumerable: true, get: function () { return filters_js_1.neq; } });
Object.defineProperty(exports, "gt", { enumerable: true, get: function () { return filters_js_1.gt; } });
Object.defineProperty(exports, "gte", { enumerable: true, get: function () { return filters_js_1.gte; } });
Object.defineProperty(exports, "lt", { enumerable: true, get: function () { return filters_js_1.lt; } });
Object.defineProperty(exports, "lte", { enumerable: true, get: function () { return filters_js_1.lte; } });
Object.defineProperty(exports, "like", { enumerable: true, get: function () { return filters_js_1.like; } });
Object.defineProperty(exports, "glob", { enumerable: true, get: function () { return filters_js_1.glob; } });
Object.defineProperty(exports, "inArray", { enumerable: true, get: function () { return filters_js_1.inArray; } });
Object.defineProperty(exports, "notInArray", { enumerable: true, get: function () { return filters_js_1.notInArray; } });
Object.defineProperty(exports, "between", { enumerable: true, get: function () { return filters_js_1.between; } });
Object.defineProperty(exports, "isNull", { enumerable: true, get: function () { return filters_js_1.isNull; } });
Object.defineProperty(exports, "isNotNull", { enumerable: true, get: function () { return filters_js_1.isNotNull; } });
Object.defineProperty(exports, "fts", { enumerable: true, get: function () { return filters_js_1.fts; } });
Object.defineProperty(exports, "not", { enumerable: true, get: function () { return filters_js_1.not; } });
Object.defineProperty(exports, "or", { enumerable: true, get: function () { return filters_js_1.or; } });
Object.defineProperty(exports, "and", { enumerable: true, get: function () { return filters_js_1.and; } });
