import type { SchemaDefinition, TableDefinition, ColumnDefinition, IndexDefinition } from "@atomicbase/schema";
import type { AtomicbaseConfig } from "./config.js";

// Re-export types from schema package (these match Go API types directly)
export type { TableDefinition, ColumnDefinition, IndexDefinition };

// Custom error class to preserve HTTP status code
export class ApiError extends Error {
  constructor(
    message: string,
    public status: number,
    public code?: string
  ) {
    super(message);
    this.name = "ApiError";
  }
}

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

// Tenant represents a tenant database (matches Go API)
export interface Tenant {
  id: number;
  name: string;
  token?: string;  // Omitted in list responses
  templateId: number;
  templateVersion: number;
  createdAt: string;
  updatedAt: string;
}

// SyncTenantResponse is returned by the sync endpoint (matches Go API)
export interface SyncTenantResponse {
  fromVersion: number;
  toVersion: number;
}

// Job/Migration represents a migration job (matches Go API's Migration)
export interface Job {
  id: number;
  templateId: number;
  fromVersion: number;
  toVersion: number;
  sql: string[];
  status: string;  // pending, running, paused, complete
  state: string | null;  // null, success, partial, failed
  totalDbs: number;
  completedDbs: number;
  failedDbs: number;
  startedAt?: string;
  completedAt?: string;
  createdAt: string;
}

// RetryJobResponse is returned by the retry endpoint (matches Go API)
export interface RetryJobResponse {
  retriedCount: number;
  jobId: number;
}

export class ApiClient {
  private baseUrl: string;
  private apiKey?: string;

  constructor(config: Required<AtomicbaseConfig>) {
    this.baseUrl = config.url.replace(/\/$/, "");
    this.apiKey = config.apiKey || undefined;
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
      throw new ApiError(
        error.message || `API error: ${response.status} ${response.statusText}`,
        response.status,
        error.code
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
   * Returns false only for 404 errors, re-throws other errors.
   */
  async templateExists(name: string): Promise<boolean> {
    try {
      await this.getTemplate(name);
      return true;
    } catch (err) {
      if (err instanceof ApiError && err.status === 404) {
        return false;
      }
      throw err;
    }
  }

  // =========================================================================
  // Job Management
  // =========================================================================

  /**
   * List all jobs (migrations).
   */
  async listJobs(status?: string): Promise<Job[]> {
    const query = status ? `?status=${status}` : "";
    return this.request<Job[]>("GET", `/platform/jobs${query}`);
  }

  /**
   * Get job status.
   */
  async getJob(jobId: number): Promise<Job> {
    return this.request<Job>("GET", `/platform/jobs/${jobId}`);
  }

  /**
   * Retry failed tenants in a job.
   */
  async retryJob(jobId: number): Promise<RetryJobResponse> {
    return this.request<RetryJobResponse>("POST", `/platform/jobs/${jobId}/retry`);
  }

  // =========================================================================
  // Tenant Management
  // =========================================================================

  /**
   * List all tenants.
   */
  async listTenants(): Promise<Tenant[]> {
    return this.request<Tenant[]>("GET", "/platform/tenants");
  }

  /**
   * Get a tenant by name.
   */
  async getTenant(name: string): Promise<Tenant> {
    return this.request<Tenant>("GET", `/platform/tenants/${name}`);
  }

  /**
   * Create a new tenant.
   */
  async createTenant(name: string, template: string): Promise<Tenant> {
    return this.request<Tenant>("POST", "/platform/tenants", {
      name,
      template,
    });
  }

  /**
   * Delete a tenant.
   */
  async deleteTenant(name: string): Promise<void> {
    await this.request<void>("DELETE", `/platform/tenants/${name}`);
  }

  /**
   * Sync a tenant to the latest template version.
   */
  async syncTenant(name: string): Promise<SyncTenantResponse> {
    return this.request<SyncTenantResponse>("POST", `/platform/tenants/${name}/sync`);
  }
}
