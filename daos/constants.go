// Package daos provides constants used throughout the data access layer.
package daos

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
	SqlAnd     = "AND"
	SqlOr      = "OR"
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

// ReservedTableDatabases is the internal table name that cannot be queried by users.
const ReservedTableDatabases = "databases"
