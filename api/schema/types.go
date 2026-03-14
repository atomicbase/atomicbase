package schema

// Schema represents a complete database schema.
type Schema struct {
	Tables []Table `json:"tables"`
}

// Table represents a database table's schema.
type Table struct {
	Name       string         `json:"name"`                 // Table name
	Pk         []string       `json:"pk"`                   // Primary key column name(s) - supports composite keys
	Columns    map[string]Col `json:"columns"`              // Keyed by column name
	Indexes    []Index        `json:"indexes,omitempty"`    // Table indexes
	FTSColumns []string       `json:"ftsColumns,omitempty"` // Columns for FTS5 full-text search
}

// Index represents a database index definition.
type Index struct {
	Name    string   `json:"name"`    // Index name
	Columns []string `json:"columns"` // Columns included in index
	Unique  bool     `json:"unique,omitempty"`
}

// Col represents a column definition.
type Col struct {
	Name       string     `json:"name"`                 // Column name
	Type       string     `json:"type"`                 // SQLite type (TEXT, INTEGER, REAL, BLOB)
	NotNull    bool       `json:"notNull,omitempty"`    // NOT NULL constraint
	Unique     bool       `json:"unique,omitempty"`     // UNIQUE constraint
	Default    any        `json:"default,omitempty"`    // Default value (nil if none)
	Collate    string     `json:"collate,omitempty"`    // COLLATE: BINARY, NOCASE, RTRIM
	Check      string     `json:"check,omitempty"`      // CHECK constraint expression
	Generated  *Generated `json:"generated,omitempty"`  // Generated column definition
	References string     `json:"references,omitempty"` // Foreign key reference (format: "table.column")
	OnDelete   string     `json:"onDelete,omitempty"`   // FK action: CASCADE, SET NULL, RESTRICT, NO ACTION
	OnUpdate   string     `json:"onUpdate,omitempty"`   // FK action: CASCADE, SET NULL, RESTRICT, NO ACTION
}

// Generated represents a generated/computed column.
type Generated struct {
	Expr   string `json:"expr"`             // Expression to compute value
	Stored bool   `json:"stored,omitempty"` // true=STORED, false=VIRTUAL (default)
}
