import type { SchemaDefinition, TableDefinition, ColumnDefinition, IndexDefinition } from "@atomicbase/schema";
import type { AtomicbaseConfig } from "./config.js";

// Re-export types from schema package (these match Go API types directly)
export type { TableDefinition, ColumnDefinition, IndexDefinition };

// Schema type matches Go API's Schema type
export interface Schema {
  tables: TableDefinition[];
}

// SchemaDiff represents a single schema modification (matches Go API)
export interface SchemaDiff {
  type: string;  // add_table, drop_table, add_column, drop_column, etc.
  table?: string;
  column?: string;
}

// DiffResult is returned by the Diff endpoint (matches Go API)
export interface DiffResult {
  changes: SchemaDiff[];
}

// Merge indicates a drop+add pair that should be treated as a rename (matches Go API)
export interface Merge {
  old: number;  // Index of drop statement in changes array
  new: number;  // Index of add statement in changes array
}

// MigrateResponse is returned by the migrate endpoint (matches Go API)
export interface MigrateResponse {
  jobId: number;
}

// TemplateWithSchema matches Go API's TemplateWithSchema
export interface TemplateResponse {
  id: number;
  name: string;
  currentVersion: number;
  createdAt: string;
  updatedAt: string;
  schema: Schema;
}

// PushResponse for creating new templates
export interface PushResponse {
  id: number;
  name: string;
  currentVersion: number;
  createdAt: string;
  updatedAt: string;
  schema: Schema;
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
   * Push a schema to the server (create new template).
   * Schema package outputs API-compatible format directly.
   */
  async pushSchema(schema: SchemaDefinition): Promise<PushResponse> {
    return this.request<PushResponse>("POST", "/platform/templates", {
      name: schema.name,
      schema: { tables: schema.tables },
    });
  }

  /**
   * Migrate an existing template to a new schema.
   * Returns a job ID for tracking the async migration.
   */
  async migrateTemplate(
    name: string,
    schema: SchemaDefinition,
    merges?: Merge[]
  ): Promise<MigrateResponse> {
    return this.request<MigrateResponse>("POST", `/platform/templates/${name}/migrate`, {
      schema: { tables: schema.tables },
      merge: merges,
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
   * Returns raw changes - ambiguity detection is client-side.
   */
  async diffSchema(
    name: string,
    schema: SchemaDefinition
  ): Promise<DiffResult> {
    return this.request<DiffResult>("POST", `/platform/templates/${name}/diff`, {
      schema: { tables: schema.tables },
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
  async getJob(jobId: number): Promise<unknown> {
    return this.request<unknown>("GET", `/platform/jobs/${jobId}`);
  }
}
