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
     * Push a schema to the server (create new template).
     * Schema package outputs API-compatible format directly.
     */
    async pushSchema(schema) {
        return this.request("POST", "/platform/templates", {
            name: schema.name,
            schema: { tables: schema.tables },
        });
    }
    /**
     * Migrate an existing template to a new schema.
     * Returns a job ID for tracking the async migration.
     */
    async migrateTemplate(name, schema, merges) {
        return this.request("POST", `/platform/templates/${name}/migrate`, {
            schema: { tables: schema.tables },
            merge: merges,
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
     * Returns raw changes - ambiguity detection is client-side.
     */
    async diffSchema(name, schema) {
        return this.request("POST", `/platform/templates/${name}/diff`, {
            schema: { tables: schema.tables },
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