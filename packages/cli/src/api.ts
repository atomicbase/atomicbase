import type { SchemaDefinition, TableDefinition, ColumnDefinition } from "@atomicbase/schema";
import type { AtomicbaseConfig } from "./config.js";

// API table format (different from SDK format)
interface ApiTable {
  name: string;
  pk: string[];
  columns: Record<string, ApiColumn>;
  indexes?: ApiIndex[];
  ftsColumns?: string[];
}

interface ApiIndex {
  name: string;
  columns: string[];
  unique?: boolean;
}

interface ApiColumn {
  name: string;
  type: string;
  notNull?: boolean;
  unique?: boolean;
  default?: string | number | null;
  collate?: string;
  check?: string;
  generated?: {
    expr: string;
    stored?: boolean;
  };
  references?: string;
  onDelete?: string;
  onUpdate?: string;
}

export interface PushResponse {
  template: {
    id: number;
    name: string;
    currentVersion: number;
    createdAt: string;
    updatedAt: string;
  };
  changes: Change[] | null;
}

export interface Change {
  type: string;
  table: string;
  column?: string;
  oldName?: string;
  sql?: string;
  ambiguous?: boolean;
  reason?: string;
  requiresMigration?: boolean;
}

export interface DiffResponse {
  changes: Change[];
  requiresMigration: boolean;
  hasAmbiguous: boolean;
  migrationSql?: string[];
}

export interface ResolvedRename {
  type: "table" | "column";
  table: string;
  column?: string;
  oldName: string;
  isRename: boolean;
}

/**
 * Convert SDK schema format to API format.
 * SDK uses arrays for columns, API uses maps.
 */
function convertToApiFormat(tables: TableDefinition[]): ApiTable[] {
  return tables.map((table) => {
    const columns: Record<string, ApiColumn> = {};
    const pk: string[] = [];

    for (const col of table.columns) {
      columns[col.name] = {
        name: col.name,
        type: col.type,
        notNull: col.notNull || undefined,
        unique: col.unique || undefined,
        default: col.defaultValue,
        collate: col.collate || undefined,
        check: col.check || undefined,
        generated: col.generated || undefined,
        references: col.references
          ? `${col.references.table}.${col.references.column}`
          : undefined,
        onDelete: col.references?.onDelete,
        onUpdate: col.references?.onUpdate,
      };

      if (col.primaryKey) {
        pk.push(col.name);
      }
    }

    return {
      name: table.name,
      pk,
      columns,
      indexes: table.indexes.length > 0 ? table.indexes.map((idx) => ({
        name: idx.name,
        columns: idx.columns,
        unique: idx.unique || undefined,
      })) : undefined,
      ftsColumns: table.ftsColumns && table.ftsColumns.length > 0 ? table.ftsColumns : undefined,
    };
  });
}

export interface TemplateResponse {
  id: number;
  name: string;
  currentVersion: number;
  tables: ApiTable[];
  createdAt: string;
  updatedAt: string;
}

export class ApiClient {
  private baseUrl: string;
  private apiKey?: string;

  constructor(config: AtomicbaseConfig) {
    this.baseUrl = config.url.replace(/\/$/, "");
    this.apiKey = config.apiKey;
  }

  private async request<T>(
    method: string,
    path: string,
    body?: unknown
  ): Promise<T> {
    const headers: Record<string, string> = {
      "Content-Type": "application/json",
    };

    if (this.apiKey) {
      headers["Authorization"] = `Bearer ${this.apiKey}`;
    }

    const response = await fetch(`${this.baseUrl}${path}`, {
      method,
      headers,
      body: body ? JSON.stringify(body) : undefined,
    });

    if (!response.ok) {
      const error = await response.json().catch(() => ({}));
      throw new Error(
        error.message || `API error: ${response.status} ${response.statusText}`
      );
    }

    return response.json();
  }

  /**
   * Push a schema to the server (create or update).
   */
  async pushSchema(
    schema: SchemaDefinition,
    resolvedRenames?: ResolvedRename[]
  ): Promise<PushResponse> {
    const tables = convertToApiFormat(schema.tables);
    return this.request<PushResponse>("POST", "/platform/templates", {
      name: schema.name,
      tables,
      resolvedRenames,
    });
  }

  /**
   * Update an existing template.
   */
  async updateTemplate(
    name: string,
    schema: SchemaDefinition,
    resolvedRenames?: ResolvedRename[]
  ): Promise<PushResponse> {
    const tables = convertToApiFormat(schema.tables);
    return this.request<PushResponse>("PUT", `/platform/templates/${name}`, {
      tables,
      resolvedRenames,
    });
  }

  /**
   * Get a template by name.
   */
  async getTemplate(name: string): Promise<TemplateResponse> {
    return this.request<TemplateResponse>("GET", `/platform/templates/${name}`);
  }

  /**
   * Preview changes without applying (diff).
   */
  async diffSchema(
    name: string,
    schema: SchemaDefinition,
    resolvedRenames?: ResolvedRename[]
  ): Promise<DiffResponse> {
    const tables = convertToApiFormat(schema.tables);
    return this.request<DiffResponse>("POST", `/platform/templates/${name}/diff`, {
      tables,
      resolvedRenames,
    });
  }

  /**
   * Check if a template exists.
   */
  async templateExists(name: string): Promise<boolean> {
    try {
      await this.getTemplate(name);
      return true;
    } catch {
      return false;
    }
  }

  /**
   * Get job status.
   */
  async getJob(jobId: string): Promise<unknown> {
    return this.request<unknown>("GET", `/platform/jobs/${jobId}`);
  }
}
