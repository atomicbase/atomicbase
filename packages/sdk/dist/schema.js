// =============================================================================
// Schema Definition SDK
// =============================================================================
//
// TypeScript-first schema definitions for Atomicbase templates.
//
// Example:
// ```typescript
// import { defineSchema, defineTable, c } from "@atomicbase/sdk";
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
    _defaultValue = null;
    _references = null;
    constructor(type) {
        this._type = type;
    }
    /**
     * Mark column as PRIMARY KEY.
     * For INTEGER columns, this enables auto-increment.
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
        this._defaultValue = value;
        return this;
    }
    /**
     * Add foreign key reference.
     * @param ref - Reference in "table.column" format
     * @param options - Optional cascade options
     */
    references(ref, options) {
        const [table, column] = ref.split(".");
        if (!table || !column) {
            throw new Error(`Invalid reference format: "${ref}". Expected "table.column"`);
        }
        this._references = {
            table,
            column,
            onDelete: options?.onDelete,
            onUpdate: options?.onUpdate,
        };
        return this;
    }
    /**
     * Build the column definition object.
     * @internal
     */
    _build(name) {
        return {
            name,
            type: this._type,
            primaryKey: this._primaryKey,
            notNull: this._notNull,
            unique: this._unique,
            defaultValue: this._defaultValue,
            references: this._references,
        };
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
    _ftsColumns = null;
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
        const columns = [];
        for (const [colName, builder] of Object.entries(this._columns)) {
            columns.push(builder._build(colName));
        }
        return {
            name,
            columns,
            indexes: this._indexes,
            ftsColumns: this._ftsColumns,
        };
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
// =============================================================================
// Serialization
// =============================================================================
/**
 * Serialize a schema definition to JSON for the API.
 * This is the format sent to POST /platform/templates.
 */
export function serializeSchema(schema) {
    return JSON.stringify(schema, null, 2);
}
/**
 * Calculate a checksum for a schema definition.
 * Used for conflict detection during push.
 */
export function schemaChecksum(schema) {
    // Sort tables and columns for consistent hashing
    const normalized = {
        name: schema.name,
        tables: [...schema.tables]
            .sort((a, b) => a.name.localeCompare(b.name))
            .map((table) => ({
            ...table,
            columns: [...table.columns].sort((a, b) => a.name.localeCompare(b.name)),
            indexes: [...table.indexes].sort((a, b) => a.name.localeCompare(b.name)),
        })),
    };
    const str = JSON.stringify(normalized);
    // Simple hash function (djb2)
    let hash = 5381;
    for (let i = 0; i < str.length; i++) {
        hash = (hash * 33) ^ str.charCodeAt(i);
    }
    return (hash >>> 0).toString(16);
}
//# sourceMappingURL=schema.js.map