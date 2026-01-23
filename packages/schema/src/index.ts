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
// Types
// =============================================================================

export type ColumnType = "INTEGER" | "TEXT" | "REAL" | "BLOB";

export type ForeignKeyAction =
  | "CASCADE"
  | "SET NULL"
  | "RESTRICT"
  | "NO ACTION";

export type Collation = "BINARY" | "NOCASE" | "RTRIM";

export interface ForeignKeyOptions {
  onDelete?: ForeignKeyAction;
  onUpdate?: ForeignKeyAction;
}

export interface GeneratedColumn {
  expr: string;
  stored?: boolean; // true=STORED, false/undefined=VIRTUAL
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

// =============================================================================
// Column Builder
// =============================================================================

/**
 * Builder class for defining column properties with chainable methods.
 */
export class ColumnBuilder {
  private _type: ColumnType;
  private _primaryKey = false;
  private _notNull = false;
  private _unique = false;
  private _defaultValue: string | number | null = null;
  private _collate: Collation | null = null;
  private _check: string | null = null;
  private _generated: GeneratedColumn | null = null;
  private _references: ColumnDefinition["references"] = null;

  constructor(type: ColumnType) {
    this._type = type;
  }

  /**
   * Mark column as PRIMARY KEY.
   */
  primaryKey(): this {
    this._primaryKey = true;
    return this;
  }

  /**
   * Add NOT NULL constraint.
   */
  notNull(): this {
    this._notNull = true;
    return this;
  }

  /**
   * Add UNIQUE constraint.
   */
  unique(): this {
    this._unique = true;
    return this;
  }

  /**
   * Set default value.
   * @param value - Literal value or SQL expression (e.g., "CURRENT_TIMESTAMP")
   */
  default(value: string | number): this {
    this._defaultValue = value;
    return this;
  }

  /**
   * Set collation for text comparison.
   * @param collation - BINARY (default), NOCASE (case-insensitive), or RTRIM
   */
  collate(collation: Collation): this {
    this._collate = collation;
    return this;
  }

  /**
   * Add CHECK constraint.
   * @param expr - SQL expression for validation (e.g., "age > 0")
   */
  check(expr: string): this {
    this._check = expr;
    return this;
  }

  /**
   * Define as a generated/computed column.
   * @param expr - SQL expression to compute value
   * @param options - { stored: true } for STORED, omit for VIRTUAL
   */
  generatedAs(expr: string, options?: { stored?: boolean }): this {
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
  references(ref: string, options?: ForeignKeyOptions): this {
    const [table, column] = ref.split(".");
    if (!table || !column) {
      throw new Error(
        `Invalid reference format: "${ref}". Expected "table.column"`
      );
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
  _build(name: string): ColumnDefinition {
    return {
      name,
      type: this._type,
      primaryKey: this._primaryKey,
      notNull: this._notNull,
      unique: this._unique,
      defaultValue: this._defaultValue,
      collate: this._collate,
      check: this._check,
      generated: this._generated,
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
  private _columns: Record<string, ColumnBuilder>;
  private _indexes: IndexDefinition[] = [];
  private _ftsColumns: string[] | null = null;

  constructor(columns: Record<string, ColumnBuilder>) {
    this._columns = columns;
  }

  /**
   * Add an index on specified columns.
   * @param name - Index name
   * @param columns - Column names to index
   */
  index(name: string, columns: string[]): this {
    this._indexes.push({ name, columns });
    return this;
  }

  /**
   * Add a unique index on specified columns.
   * @param name - Index name
   * @param columns - Column names to index
   */
  uniqueIndex(name: string, columns: string[]): this {
    this._indexes.push({ name, columns, unique: true });
    return this;
  }

  /**
   * Enable FTS5 full-text search on specified columns.
   * Creates a virtual table: {tableName}_fts
   * @param columns - Column names to include in FTS
   */
  fts(columns: string[]): this {
    this._ftsColumns = columns;
    return this;
  }

  /**
   * Build the table definition object.
   * @internal
   */
  _build(name: string): TableDefinition {
    const columns: ColumnDefinition[] = [];

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
export function defineTable(
  columns: Record<string, ColumnBuilder>
): TableBuilder {
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
export function defineSchema(
  name: string,
  tables: Record<string, TableBuilder>
): SchemaDefinition {
  const tableDefinitions: TableDefinition[] = [];

  for (const [tableName, builder] of Object.entries(tables)) {
    tableDefinitions.push(builder._build(tableName));
  }

  return {
    name,
    tables: tableDefinitions,
  };
}

