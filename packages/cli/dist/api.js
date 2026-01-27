// Custom error class to preserve HTTP status code
export class ApiError extends Error {
    status;
    code;
    constructor(message, status, code) {
        super(message);
        this.status = status;
        this.code = code;
        this.name = "ApiError";
    }
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
            throw new ApiError(error.message || `API error: ${response.status} ${response.statusText}`, response.status, error.code);
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
     * Returns false only for 404 errors, re-throws other errors.
     */
    async templateExists(name) {
        try {
            await this.getTemplate(name);
            return true;
        }
        catch (err) {
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
    async listJobs(status) {
        const query = status ? `?status=${status}` : "";
        return this.request("GET", `/platform/jobs${query}`);
    }
    /**
     * Get job status.
     */
    async getJob(jobId) {
        return this.request("GET", `/platform/jobs/${jobId}`);
    }
    /**
     * Retry failed tenants in a job.
     */
    async retryJob(jobId) {
        return this.request("POST", `/platform/jobs/${jobId}/retry`);
    }
    // =========================================================================
    // Tenant Management
    // =========================================================================
    /**
     * List all tenants.
     */
    async listTenants() {
        return this.request("GET", "/platform/tenants");
    }
    /**
     * Get a tenant by name.
     */
    async getTenant(name) {
        return this.request("GET", `/platform/tenants/${name}`);
    }
    /**
     * Create a new tenant.
     */
    async createTenant(name, template) {
        return this.request("POST", "/platform/tenants", {
            name,
            template,
        });
    }
    /**
     * Delete a tenant.
     */
    async deleteTenant(name) {
        await this.request("DELETE", `/platform/tenants/${name}`);
    }
    /**
     * Sync a tenant to the latest template version.
     */
    async syncTenant(name) {
        return this.request("POST", `/platform/tenants/${name}/sync`);
    }
}
//# sourceMappingURL=api.js.map