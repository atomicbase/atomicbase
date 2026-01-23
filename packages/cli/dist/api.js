/**
 * Convert SDK schema format to API format.
 * SDK uses arrays for columns, API uses maps.
 */
function convertToApiFormat(tables) {
    return tables.map((table) => {
        const columns = {};
        const pk = [];
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
export class ApiClient {
    baseUrl;
    apiKey;
    constructor(config) {
        this.baseUrl = config.url.replace(/\/$/, "");
        this.apiKey = config.apiKey;
    }
    async request(method, path, body) {
        const headers = {
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
            throw new Error(error.message || `API error: ${response.status} ${response.statusText}`);
        }
        return response.json();
    }
    /**
     * Push a schema to the server (create or update).
     */
    async pushSchema(schema, resolvedRenames) {
        const tables = convertToApiFormat(schema.tables);
        return this.request("POST", "/platform/templates", {
            name: schema.name,
            tables,
            resolvedRenames,
        });
    }
    /**
     * Update an existing template.
     */
    async updateTemplate(name, schema, resolvedRenames) {
        const tables = convertToApiFormat(schema.tables);
        return this.request("PUT", `/platform/templates/${name}`, {
            tables,
            resolvedRenames,
        });
    }
    /**
     * Get a template by name.
     */
    async getTemplate(name) {
        return this.request("GET", `/platform/templates/${name}`);
    }
    /**
     * Preview changes without applying (diff).
     */
    async diffSchema(name, schema, resolvedRenames) {
        const tables = convertToApiFormat(schema.tables);
        return this.request("POST", `/platform/templates/${name}/diff`, {
            tables,
            resolvedRenames,
        });
    }
    /**
     * Check if a template exists.
     */
    async templateExists(name) {
        try {
            await this.getTemplate(name);
            return true;
        }
        catch {
            return false;
        }
    }
    /**
     * Get job status.
     */
    async getJob(jobId) {
        return this.request("GET", `/platform/jobs/${jobId}`);
    }
}
//# sourceMappingURL=api.js.map