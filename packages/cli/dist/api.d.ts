import type { SchemaDefinition } from "@atomicbase/schema";
import type { AtomicbaseConfig } from "./config.js";
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
export interface TemplateResponse {
    id: number;
    name: string;
    currentVersion: number;
    tables: ApiTable[];
    createdAt: string;
    updatedAt: string;
}
export declare class ApiClient {
    private baseUrl;
    private apiKey?;
    constructor(config: AtomicbaseConfig);
    private request;
    /**
     * Push a schema to the server (create or update).
     */
    pushSchema(schema: SchemaDefinition, resolvedRenames?: ResolvedRename[]): Promise<PushResponse>;
    /**
     * Update an existing template.
     */
    updateTemplate(name: string, schema: SchemaDefinition, resolvedRenames?: ResolvedRename[]): Promise<PushResponse>;
    /**
     * Get a template by name.
     */
    getTemplate(name: string): Promise<TemplateResponse>;
    /**
     * Preview changes without applying (diff).
     */
    diffSchema(name: string, schema: SchemaDefinition, resolvedRenames?: ResolvedRename[]): Promise<DiffResponse>;
    /**
     * Check if a template exists.
     */
    templateExists(name: string): Promise<boolean>;
    /**
     * Get job status.
     */
    getJob(jobId: string): Promise<unknown>;
}
export {};
//# sourceMappingURL=api.d.ts.map