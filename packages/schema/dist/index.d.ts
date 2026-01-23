export type ColumnType = "INTEGER" | "TEXT" | "REAL" | "BLOB";
export type ForeignKeyAction = "CASCADE" | "SET NULL" | "RESTRICT" | "NO ACTION";
export type Collation = "BINARY" | "NOCASE" | "RTRIM";
export interface ForeignKeyOptions {
    onDelete?: ForeignKeyAction;
    onUpdate?: ForeignKeyAction;
}
export interface GeneratedColumn {
    expr: string;
    stored?: boolean;
}
export interface ColumnDefinition {
    name: string;
    type: ColumnType;
    primaryKey: boolean;
    notNull: boolean;
    unique: boolean;
    defaultValue: string | number | null;
    collate: Collation | null;
    check: string | null;
    generated: GeneratedColumn | null;
    references: {
        table: string;
        column: string;
        onDelete?: ForeignKeyAction;
        onUpdate?: ForeignKeyAction;
    } | null;
}
export interface IndexDefinition {
    name: string;
    columns: string[];
    unique?: boolean;
}
export interface TableDefinition {
    name: string;
    columns: ColumnDefinition[];
    indexes: IndexDefinition[];
    ftsColumns: string[] | null;
}
export interface SchemaDefinition {
    name: string;
    tables: TableDefinition[];
}
/**
 * Builder class for defining column properties with chainable methods.
 */
export declare class ColumnBuilder {
    private _type;
    private _primaryKey;
    private _notNull;
    private _unique;
    private _defaultValue;
    private _collate;
    private _check;
    private _generated;
    private _references;
    constructor(type: ColumnType);
    /**
     * Mark column as PRIMARY KEY.
     */
    primaryKey(): this;
    /**
     * Add NOT NULL constraint.
     */
    notNull(): this;
    /**
     * Add UNIQUE constraint.
     */
    unique(): this;
    /**
     * Set default value.
     * @param value - Literal value or SQL expression (e.g., "CURRENT_TIMESTAMP")
     */
    default(value: string | number): this;
    /**
     * Set collation for text comparison.
     * @param collation - BINARY (default), NOCASE (case-insensitive), or RTRIM
     */
    collate(collation: Collation): this;
    /**
     * Add CHECK constraint.
     * @param expr - SQL expression for validation (e.g., "age > 0")
     */
    check(expr: string): this;
    /**
     * Define as a generated/computed column.
     * @param expr - SQL expression to compute value
     * @param options - { stored: true } for STORED, omit for VIRTUAL
     */
    generatedAs(expr: string, options?: {
        stored?: boolean;
    }): this;
    /**
     * Add foreign key reference.
     * @param ref - Reference in "table.column" format
     * @param options - Optional cascade options
     */
    references(ref: string, options?: ForeignKeyOptions): this;
    /**
     * Build the column definition object.
     * @internal
     */
    _build(name: string): ColumnDefinition;
}
/**
 * Column type builders.
 *
 * ```typescript
 * c.integer()  // INTEGER - whole numbers, booleans (0/1), timestamps
 * c.text()     // TEXT - strings, JSON, ISO dates
 * c.real()     // REAL - floating point numbers
 * c.blob()     // BLOB - binary data
 * ```
 */
export declare const c: {
    /**
     * INTEGER column - whole numbers, booleans (0/1), unix timestamps.
     */
    integer: () => ColumnBuilder;
    /**
     * TEXT column - strings, JSON, ISO dates.
     */
    text: () => ColumnBuilder;
    /**
     * REAL column - floating point numbers.
     */
    real: () => ColumnBuilder;
    /**
     * BLOB column - binary data.
     */
    blob: () => ColumnBuilder;
};
/**
 * Builder class for defining table properties.
 */
export declare class TableBuilder {
    private _columns;
    private _indexes;
    private _ftsColumns;
    constructor(columns: Record<string, ColumnBuilder>);
    /**
     * Add an index on specified columns.
     * @param name - Index name
     * @param columns - Column names to index
     */
    index(name: string, columns: string[]): this;
    /**
     * Add a unique index on specified columns.
     * @param name - Index name
     * @param columns - Column names to index
     */
    uniqueIndex(name: string, columns: string[]): this;
    /**
     * Enable FTS5 full-text search on specified columns.
     * Creates a virtual table: {tableName}_fts
     * @param columns - Column names to include in FTS
     */
    fts(columns: string[]): this;
    /**
     * Build the table definition object.
     * @internal
     */
    _build(name: string): TableDefinition;
}
/**
 * Define a table with columns and optional indexes/FTS.
 *
 * ```typescript
 * const users = defineTable({
 *   id: c.integer().primaryKey(),
 *   email: c.text().notNull().unique(),
 *   name: c.text().notNull(),
 * }).index("idx_email", ["email"]);
 * ```
 */
export declare function defineTable(columns: Record<string, ColumnBuilder>): TableBuilder;
/**
 * Define a schema template with multiple tables.
 *
 * ```typescript
 * export default defineSchema("user-app", {
 *   users: defineTable({ ... }),
 *   projects: defineTable({ ... }),
 * });
 * ```
 */
export declare function defineSchema(name: string, tables: Record<string, TableBuilder>): SchemaDefinition;
//# sourceMappingURL=index.d.ts.map