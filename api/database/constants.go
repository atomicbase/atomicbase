// Package database provides constants used throughout the data access layer.
package database

// Column types for SQLite schema validation.
const (
	ColTypeText    = "TEXT"
	ColTypeInteger = "INTEGER"
	ColTypeReal    = "REAL"
	ColTypeBlob    = "BLOB"
)

// Query parameter keys used in URL query strings.
const (
	ParamSelect = "select"
	ParamOrder  = "order"
	ParamOr     = "or"
	ParamLimit  = "limit"
	ParamOffset = "offset"
	ParamCount  = "count"
)

// Filter operators for WHERE clause conditions.
const (
	OpEq      = "eq"
	OpNeq     = "neq"
	OpLt      = "lt"
	OpLte     = "lte"
	OpGt      = "gt"
	OpGte     = "gte"
	OpLike    = "like"
	OpGlob    = "glob"
	OpBetween = "between"
	OpNot     = "not"
	OpIn      = "in"
	OpIs      = "is"
	OpFts     = "fts"
	OpAnd     = "and"
	OpOr      = "or"
)

// SQL operators mapped from filter operators.
const (
	SqlEq      = "="
	SqlNeq     = "!="
	SqlLt      = "<"
	SqlLte     = "<="
	SqlGt      = ">"
	SqlGte     = ">="
	SqlLike    = "LIKE"
	SqlGlob    = "GLOB"
	SqlBetween = "BETWEEN"
	SqlNot     = "NOT"
	SqlIn      = "IN"
	SqlIs      = "IS"
	SqlMatch   = "MATCH"
	SqlAnd     = "AND"
	SqlOr      = "OR"
)

// FTS5 full-text search constants.
const (
	FTSSuffix = "_fts" // Suffix for FTS5 virtual table names
)

// Foreign key referential actions.
const (
	FkNoAction   = "NO ACTION"
	FkRestrict   = "RESTRICT"
	FkSetNull    = "SET NULL"
	FkSetDefault = "SET DEFAULT"
	FkCascade    = "CASCADE"
)

// Order directions for ORDER BY clauses.
const (
	OrderAsc  = "asc"
	OrderDesc = "desc"
)

// InternalTablePrefix is the prefix for internal atomicbase tables.
// Tables with this prefix are excluded from user queries and schema sync operations.
const InternalTablePrefix = "atomicbase_"

// ReservedTableDatabases is the internal table name that cannot be queried by users.
const ReservedTableDatabases = "atomicbase_databases"

// ReservedTableTemplates stores schema templates for multi-tenant database management.
const ReservedTableTemplates = "atomicbase_schema_templates"
