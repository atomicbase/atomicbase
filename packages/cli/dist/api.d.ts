import type { SchemaDefinition, TableDefinition, ColumnDefinition, IndexDefinition } from "@atomicbase/schema";
import type { AtomicbaseConfig } from "./config.js";
export type { TableDefinition, ColumnDefinition, IndexDefinition };
export declare class ApiError extends Error {
    status: number;
    code?: string | undefined;
    constructor(message: string, status: number, code?: string | undefined);
}
export interface Schema {
    tables: TableDefinition[];
}
export interface SchemaDiff {
    type: string;
    table?: string;
    column?: string;
}
export interface DiffResult {
    changes: SchemaDiff[];
}
export interface Merge {
    old: number;
    new: number;
}
export interface MigrateResponse {
    jobId: number;
}
export interface TemplateResponse {
    id: number;
    name: string;
    currentVersion: number;
    createdAt: string;
    updatedAt: string;
    schema: Schema;
}
export interface PushResponse {
    id: number;
    name: string;
    currentVersion: number;
    createdAt: string;
    updatedAt: string;
    schema: Schema;
}
export interface Tenant {
    id: number;
    name: string;
    token?: string;
    templateId: number;
    templateVersion: number;
    createdAt: string;
    updatedAt: string;
}
export interface SyncTenantResponse {
    fromVersion: number;
    toVersion: number;
}
export declare class ApiClient {
    private baseUrl;
    private apiKey?;
    constructor(config: AtomicbaseConfig);
    private request;
    /**
     * Push a schema to the server (create new template).
     * Schema package outputs API-compatible format directly.
     */
    pushSchema(schema: SchemaDefinition): Promise<PushResponse>;
    /**
     * Migrate an existing template to a new schema.
     * Returns a job ID for tracking the async migration.
     */
    migrateTemplate(name: string, schema: SchemaDefinition, merges?: Merge[]): Promise<MigrateResponse>;
    /**
     * Get a template by name.
     */
    getTemplate(name: string): Promise<TemplateResponse>;
    /**
     * Preview changes without applying (diff).
     * Returns raw changes - ambiguity detection is client-side.
     */
    diffSchema(name: string, schema: SchemaDefinition): Promise<DiffResult>;
    /**
     * Check if a template exists.
     * Returns false only for 404 errors, re-throws other errors.
     */
    templateExists(name: string): Promise<boolean>;
    /**
     * Get job status.
     */
    getJob(jobId: number): Promise<unknown>;
    /**
     * List all tenants.
     */
    listTenants(): Promise<Tenant[]>;
    /**
     * Get a tenant by name.
     */
    getTenant(name: string): Promise<Tenant>;
    /**
     * Create a new tenant.
     */
    createTenant(name: string, template: string): Promise<Tenant>;
    /**
     * Delete a tenant.
     */
    deleteTenant(name: string): Promise<void>;
    /**
     * Sync a tenant to the latest template version.
     */
    syncTenant(name: string): Promise<SyncTenantResponse>;
}
//# sourceMappingURL=api.d.ts.map