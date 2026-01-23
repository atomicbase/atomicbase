// =============================================================================
// Client
// =============================================================================

export { AtomicbaseClient, createClient } from "./AtomicbaseClient.js";

// =============================================================================
// Builders (for advanced usage / extension)
// =============================================================================

export { AtomicbaseBuilder } from "./AtomicbaseBuilder.js";
export { AtomicbaseTransformBuilder } from "./AtomicbaseTransformBuilder.js";
export { AtomicbaseQueryBuilder } from "./AtomicbaseQueryBuilder.js";

// =============================================================================
// Error
// =============================================================================

export { AtomicbaseError } from "./AtomicbaseError.js";

// =============================================================================
// Filter Functions
// =============================================================================

export {
  // Column reference helper
  col,
  // Join condition functions
  onEq,
  onNeq,
  onGt,
  onGte,
  onLt,
  onLte,
  // WHERE filter functions
  eq,
  neq,
  gt,
  gte,
  lt,
  lte,
  like,
  glob,
  inArray,
  notInArray,
  between,
  isNull,
  isNotNull,
  fts,
  not,
  or,
  and,
} from "./filters.js";

// =============================================================================
// Types
// =============================================================================

export type {
  // Response types (discriminated unions)
  AtomicbaseResponse,
  AtomicbaseResponseSuccess,
  AtomicbaseResponseFailure,
  AtomicbaseSingleResponse,
  AtomicbaseMaybeSingleResponse,
  AtomicbaseResponseWithCount,
  // Batch types
  BatchOperation,
  BatchResponse,
  AtomicbaseBatchResponse,
  // Configuration
  AtomicbaseClientOptions,
  // Query types
  FilterCondition,
  SelectColumn,
  OrderDirection,
  // Join types
  JoinClause,
} from "./types.js";
