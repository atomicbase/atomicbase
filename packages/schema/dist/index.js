// =============================================================================
// Schema Definition SDK
// =============================================================================
//
// TypeScript-first schema definitions for Atomicbase templates.
//
// Example:
// ```typescript
// import { defineSchema, defineTable, c } from "@atomicbase/schema";
//
// export default defineSchema("user-app", {
//   users: defineTable({
//     id: c.integer().primaryKey(),
//     email: c.text().notNull().unique(),
//     name: c.text().notNull(),
//   }),
// });
// ```
// =============================================================================
// Column Builder
// =============================================================================
/**
 * Builder class for defining column properties with chainable methods.
 */
export class ColumnBuilder {
    _type;
    _primaryKey = false;
    _notNull = false;
    _unique = false;
    _default = null;
    _collate = undefined;
    _check = undefined;
    _generated = undefined;
    _references = undefined;
    _onDelete = undefined;
    _onUpdate = undefined;
    constructor(type) {
        this._type = type;
    }
    /**
     * Mark column as PRIMARY KEY.
     */
    primaryKey() {
        this._primaryKey = true;
        return this;
    }
    /**
     * Add NOT NULL constraint.
     */
    notNull() {
        this._notNull = true;
        return this;
    }
    /**
     * Add UNIQUE constraint.
     */
    unique() {
        this._unique = true;
        return this;
    }
    /**
     * Set default value.
     * @param value - Literal value or SQL expression (e.g., "CURRENT_TIMESTAMP")
     */
    default(value) {
        this._default = value;
        return this;
    }
    /**
     * Set collation for text comparison.
     * @param collation - BINARY (default), NOCASE (case-insensitive), or RTRIM
     */
    collate(collation) {
        this._collate = collation;
        return this;
    }
    /**
     * Add CHECK constraint.
     * @param expr - SQL expression for validation (e.g., "age > 0")
     */
    check(expr) {
        this._check = expr;
        return this;
    }
    /**
     * Define as a generated/computed column.
     * @param expr - SQL expression to compute value
     * @param options - { stored: true } for STORED, omit for VIRTUAL
     */
    generatedAs(expr, options) {
        this._generated = {
            expr,
            stored: options?.stored,
        };
        return this;
    }
    /**
     * Add foreign key reference.
     * @param ref - Reference in "table.column" format
     * @param options - Optional cascade options
     */
    references(ref, options) {
        const parts = ref.split(".");
        if (parts.length !== 2 || !parts[0] || !parts[1]) {
            throw new Error(`Invalid reference format: "${ref}". Expected "table.column"`);
        }
        this._references = ref;
        this._onDelete = options?.onDelete;
        this._onUpdate = options?.onUpdate;
        return this;
    }
    /**
     * Check if this column is a primary key.
     * @internal
     */
    _isPrimaryKey() {
        return this._primaryKey;
    }
    /**
     * Build the column definition object.
     * @internal
     */
    _build(name) {
        const col = {
            name,
            type: this._type,
        };
        // Only include optional fields if they have values
        if (this._notNull)
            col.notNull = true;
        if (this._unique)
            col.unique = true;
        if (this._default !== null)
            col.default = this._default;
        if (this._collate)
            col.collate = this._collate;
        if (this._check)
            col.check = this._check;
        if (this._generated)
            col.generated = this._generated;
        if (this._references)
            col.references = this._references;
        if (this._onDelete)
            col.onDelete = this._onDelete;
        if (this._onUpdate)
            col.onUpdate = this._onUpdate;
        return col;
    }
}
// =============================================================================
// Column Type Factories
// =============================================================================
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
export const c = {
    /**
     * INTEGER column - whole numbers, booleans (0/1), unix timestamps.
     */
    integer: () => new ColumnBuilder("INTEGER"),
    /**
     * TEXT column - strings, JSON, ISO dates.
     */
    text: () => new ColumnBuilder("TEXT"),
    /**
     * REAL column - floating point numbers.
     */
    real: () => new ColumnBuilder("REAL"),
    /**
     * BLOB column - binary data.
     */
    blob: () => new ColumnBuilder("BLOB"),
};
// =============================================================================
// Table Builder
// =============================================================================
/**
 * Builder class for defining table properties.
 */
export class TableBuilder {
    _columns;
    _indexes = [];
    _ftsColumns = undefined;
    constructor(columns) {
        this._columns = columns;
    }
    /**
     * Add an index on specified columns.
     * @param name - Index name
     * @param columns - Column names to index
     */
    index(name, columns) {
        this._indexes.push({ name, columns });
        return this;
    }
    /**
     * Add a unique index on specified columns.
     * @param name - Index name
     * @param columns - Column names to index
     */
    uniqueIndex(name, columns) {
        this._indexes.push({ name, columns, unique: true });
        return this;
    }
    /**
     * Enable FTS5 full-text search on specified columns.
     * Creates a virtual table: {tableName}_fts
     * @param columns - Column names to include in FTS
     */
    fts(columns) {
        this._ftsColumns = columns;
        return this;
    }
    /**
     * Build the table definition object.
     * @internal
     */
    _build(name) {
        const columns = {};
        const pk = [];
        for (const [colName, builder] of Object.entries(this._columns)) {
            columns[colName] = builder._build(colName);
            if (builder._isPrimaryKey()) {
                pk.push(colName);
            }
        }
        const table = {
            name,
            pk,
            columns,
        };
        if (this._indexes.length > 0)
            table.indexes = this._indexes;
        if (this._ftsColumns)
            table.ftsColumns = this._ftsColumns;
        return table;
    }
}
// =============================================================================
// Define Functions
// =============================================================================
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
export function defineTable(columns) {
    return new TableBuilder(columns);
}
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
export function defineSchema(name, tables) {
    const tableDefinitions = [];
    for (const [tableName, builder] of Object.entries(tables)) {
        tableDefinitions.push(builder._build(tableName));
    }
    return {
        name,
        tables: tableDefinitions,
    };
}
//# sourceMappingURL=index.js.map