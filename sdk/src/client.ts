import type { components } from "./types.js";

export type Schemas = components["schemas"];

// Result type - similar to Supabase's pattern
export type Result<T> = {
  data: T | null;
  error: AtomicbaseError | null;
};

// Filter operators supported by Atomicbase
export type FilterOperator =
  | "eq"
  | "neq"
  | "gt"
  | "gte"
  | "lt"
  | "lte"
  | "like"
  | "ilike"
  | "is"
  | "in"
  | "fts";

// Operators that can be negated with "not."
export type NegatableOperator = Exclude<FilterOperator, "neq" | "fts">;

export type FilterValue<Op extends FilterOperator> = Op extends "in"
  ? (string | number | boolean)[]
  : Op extends "is"
  ? "null" | "true" | "false"
  : Op extends "fts"
  ? string
  : string | number | boolean;

export type Filter<T = Record<string, unknown>> = {
  [K in keyof T]?: {
    [Op in FilterOperator]?: FilterValue<Op>;
  };
};

export type OrderDirection = "asc" | "desc";

export interface QueryOptions<T = Record<string, unknown>> {
  select?: string;
  filter?: Filter<T>;
  order?: { column: keyof T; direction: OrderDirection };
  limit?: number;
  offset?: number;
}

export interface CountResult<T> {
  data: T[];
  count: number;
}

export type AggregateFunction = "count" | "sum" | "avg" | "min" | "max";

export interface AtomicbaseConfig {
  baseUrl: string;
  apiKey?: string;
  database?: string;
  fetch?: typeof fetch;
}

export class AtomicbaseError extends Error {
  constructor(
    message: string,
    public status: number,
    public code?: string,
    public details?: unknown
  ) {
    super(message);
    this.name = "AtomicbaseError";
  }
}

function buildFilterParams(
  filter: Record<string, Record<string, unknown> | undefined>
): URLSearchParams {
  const params = new URLSearchParams();
  for (const [column, operators] of Object.entries(filter)) {
    if (!operators) continue;
    for (const [op, value] of Object.entries(operators)) {
      if (value === undefined) continue;
      // Handle "in" and "not.in" with array values
      if ((op === "in" || op === "not.in") && Array.isArray(value)) {
        params.set(column, `${op}.(${value.join(",")})`);
      } else {
        params.set(column, `${op}.${value}`);
      }
    }
  }
  return params;
}

export class AtomicbaseClient {
  private baseUrl: string;
  private apiKey?: string;
  private dbName?: string;
  private fetchFn: typeof fetch;

  constructor(config: AtomicbaseConfig) {
    this.baseUrl = config.baseUrl.replace(/\/$/, "");
    this.apiKey = config.apiKey;
    this.dbName = config.database;
    this.fetchFn = config.fetch ?? fetch;
  }

  private async request<T>(
    method: string,
    path: string,
    options: {
      params?: URLSearchParams;
      body?: unknown;
      headers?: Record<string, string>;
    } = {}
  ): Promise<Result<T>> {
    try {
      const url = new URL(path, this.baseUrl);
      if (options.params) {
        options.params.forEach((value, key) => url.searchParams.set(key, value));
      }

      const headers: Record<string, string> = {
        "Content-Type": "application/json",
        ...options.headers,
      };

      if (this.apiKey) {
        headers["Authorization"] = `Bearer ${this.apiKey}`;
      }

      if (this.dbName) {
        headers["DB-Name"] = this.dbName;
      }

      const response = await this.fetchFn(url.toString(), {
        method,
        headers,
        body: options.body ? JSON.stringify(options.body) : undefined,
      });

      if (!response.ok) {
        const body = await response.json().catch(() => ({}));
        return {
          data: null,
          error: new AtomicbaseError(
            body?.error ?? `Request failed with status ${response.status}`,
            response.status,
            body?.code,
            body
          ),
        };
      }

      // Handle empty responses
      const text = await response.text();
      if (!text) {
        return { data: null, error: null };
      }

      return { data: JSON.parse(text) as T, error: null };
    } catch (err) {
      // Network or parsing errors
      const message = err instanceof Error ? err.message : "Unknown error";
      return {
        data: null,
        error: new AtomicbaseError(message, 0, "NETWORK_ERROR"),
      };
    }
  }

  private async rawRequest(
    method: string,
    path: string,
    options: {
      params?: URLSearchParams;
      body?: unknown;
      headers?: Record<string, string>;
    } = {}
  ): Promise<Result<Response>> {
    try {
      const url = new URL(path, this.baseUrl);
      if (options.params) {
        options.params.forEach((value, key) => url.searchParams.set(key, value));
      }

      const headers: Record<string, string> = {
        "Content-Type": "application/json",
        ...options.headers,
      };

      if (this.apiKey) {
        headers["Authorization"] = `Bearer ${this.apiKey}`;
      }

      if (this.dbName) {
        headers["DB-Name"] = this.dbName;
      }

      const response = await this.fetchFn(url.toString(), {
        method,
        headers,
        body: options.body ? JSON.stringify(options.body) : undefined,
      });

      if (!response.ok) {
        const body = await response.json().catch(() => ({}));
        return {
          data: null,
          error: new AtomicbaseError(
            body?.error ?? `Request failed with status ${response.status}`,
            response.status,
            body?.code,
            body
          ),
        };
      }

      return { data: response, error: null };
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unknown error";
      return {
        data: null,
        error: new AtomicbaseError(message, 0, "NETWORK_ERROR"),
      };
    }
  }

  /**
   * Create a new client instance targeting a different database
   */
  database(name: string): AtomicbaseClient {
    return new AtomicbaseClient({
      baseUrl: this.baseUrl,
      apiKey: this.apiKey,
      database: name,
      fetch: this.fetchFn,
    });
  }

  /**
   * Get a table query builder
   */
  from<T extends Record<string, unknown> = Record<string, unknown>>(
    table: string
  ): TableQuery<T> {
    return new TableQuery<T>(this, table);
  }

  /**
   * Health check
   */
  async health(): Promise<Result<Schemas["HealthResponse"]>> {
    return this.request<Schemas["HealthResponse"]>("GET", "/health");
  }

  /**
   * Execute a DDL query (CREATE, ALTER, DROP)
   */
  async executeSchema(
    query: string,
    args?: unknown[]
  ): Promise<Result<Schemas["MessageResponse"]>> {
    return this.request<Schemas["MessageResponse"]>("POST", "/schema", {
      body: { query, args },
    });
  }

  /**
   * Invalidate schema cache
   */
  async invalidateSchema(): Promise<Result<Schemas["MessageResponse"]>> {
    return this.request<Schemas["MessageResponse"]>(
      "POST",
      "/schema/invalidate"
    );
  }

  /**
   * Get all tables in the schema
   * @returns Array of tables with their columns and primary keys
   */
  async getSchema(): Promise<Result<Schemas["Table"][]>> {
    return this.request<Schemas["Table"][]>("GET", "/schema");
  }

  /**
   * Get table schema
   */
  async getTableSchema(
    table: string
  ): Promise<Result<Schemas["TableSchemaResponse"]>> {
    return this.request<Schemas["TableSchemaResponse"]>(
      "GET",
      `/schema/table/${encodeURIComponent(table)}`
    );
  }

  /**
   * Create a new table
   */
  async createTable(
    table: string,
    columns: Schemas["CreateTableRequest"]
  ): Promise<Result<Schemas["MessageResponse"]>> {
    return this.request<Schemas["MessageResponse"]>(
      "POST",
      `/schema/table/${encodeURIComponent(table)}`,
      { body: columns }
    );
  }

  /**
   * Alter a table
   */
  async alterTable(
    table: string,
    changes: Schemas["AlterTableRequest"]
  ): Promise<Result<Schemas["MessageResponse"]>> {
    return this.request<Schemas["MessageResponse"]>(
      "PATCH",
      `/schema/table/${encodeURIComponent(table)}`,
      { body: changes }
    );
  }

  /**
   * Drop a table
   */
  async dropTable(table: string): Promise<Result<Schemas["MessageResponse"]>> {
    return this.request<Schemas["MessageResponse"]>(
      "DELETE",
      `/schema/table/${encodeURIComponent(table)}`
    );
  }

  /**
   * Create an FTS5 (Full-Text Search) index on a table
   * @param table - The table to create the FTS index on
   * @param columns - Array of TEXT columns to include in the FTS index
   * @example const { data, error } = await client.createFTSIndex("articles", ["title", "content"]);
   */
  async createFTSIndex(
    table: string,
    columns: string[]
  ): Promise<Result<Schemas["MessageResponse"]>> {
    return this.request<Schemas["MessageResponse"]>(
      "POST",
      `/schema/fts/${encodeURIComponent(table)}`,
      { body: { columns } }
    );
  }

  /**
   * Drop an FTS5 index from a table
   * @param table - The table to remove the FTS index from
   */
  async dropFTSIndex(
    table: string
  ): Promise<Result<Schemas["MessageResponse"]>> {
    return this.request<Schemas["MessageResponse"]>(
      "DELETE",
      `/schema/fts/${encodeURIComponent(table)}`
    );
  }

  /**
   * List all FTS indexes
   * @returns Array of FTS index info (table, ftsTable, columns)
   */
  async listFTSIndexes(): Promise<Result<Schemas["FTSIndexInfo"][]>> {
    return this.request<Schemas["FTSIndexInfo"][]>("GET", "/schema/fts");
  }

  /**
   * List registered databases
   */
  async listDatabases(): Promise<Result<Schemas["DatabaseInfo"][]>> {
    return this.request<Schemas["DatabaseInfo"][]>("GET", "/db");
  }

  /**
   * Create a new Turso database
   */
  async createDatabase(
    name: string,
    group?: string
  ): Promise<Result<Schemas["MessageResponse"]>> {
    return this.request<Schemas["MessageResponse"]>("POST", "/db", {
      body: { name, group },
    });
  }

  /**
   * Register an existing Turso database
   */
  async registerDatabase(
    name: string,
    token?: string
  ): Promise<Result<Schemas["MessageResponse"]>> {
    const headers: Record<string, string> = {};
    if (token) headers["DB-Token"] = token;
    return this.request<Schemas["MessageResponse"]>("PATCH", "/db", {
      body: { name },
      headers,
    });
  }

  /**
   * Register all Turso databases in organization
   */
  async registerAllDatabases(): Promise<Result<void>> {
    return this.request<void>("PATCH", "/db/all");
  }

  /**
   * Delete a database
   */
  async deleteDatabase(
    name: string
  ): Promise<Result<Schemas["MessageResponse"]>> {
    return this.request<Schemas["MessageResponse"]>(
      "DELETE",
      `/db/${encodeURIComponent(name)}`
    );
  }

  // ==================== Schema Templates ====================

  /**
   * List all schema templates
   * @returns Array of schema templates
   */
  async listTemplates(): Promise<Result<Schemas["SchemaTemplate"][]>> {
    return this.request<Schemas["SchemaTemplate"][]>("GET", "/templates");
  }

  /**
   * Get a schema template by name
   * @param name - Template name
   */
  async getTemplate(name: string): Promise<Result<Schemas["SchemaTemplate"]>> {
    return this.request<Schemas["SchemaTemplate"]>(
      "GET",
      `/templates/${encodeURIComponent(name)}`
    );
  }

  /**
   * Create a new schema template
   * @param name - Template name
   * @param tables - Array of table definitions
   */
  async createTemplate(
    name: string,
    tables: Schemas["Table"][]
  ): Promise<Result<Schemas["SchemaTemplate"]>> {
    return this.request<Schemas["SchemaTemplate"]>("POST", "/templates", {
      body: { name, tables },
    });
  }

  /**
   * Update an existing schema template
   * @param name - Template name
   * @param tables - New array of table definitions
   */
  async updateTemplate(
    name: string,
    tables: Schemas["Table"][]
  ): Promise<Result<Schemas["SchemaTemplate"]>> {
    return this.request<Schemas["SchemaTemplate"]>(
      "PUT",
      `/templates/${encodeURIComponent(name)}`,
      { body: { tables } }
    );
  }

  /**
   * Delete a schema template
   * @param name - Template name
   */
  async deleteTemplate(
    name: string
  ): Promise<Result<Schemas["MessageResponse"]>> {
    return this.request<Schemas["MessageResponse"]>(
      "DELETE",
      `/templates/${encodeURIComponent(name)}`
    );
  }

  /**
   * Sync a template to all associated databases
   * @param name - Template name
   * @param dropExtra - If true, drop tables not in the template
   */
  async syncTemplate(
    name: string,
    dropExtra = false
  ): Promise<Result<Schemas["SyncResult"][]>> {
    const params = new URLSearchParams();
    if (dropExtra) params.set("dropExtra", "true");
    return this.request<Schemas["SyncResult"][]>(
      "POST",
      `/templates/${encodeURIComponent(name)}/sync`,
      { params }
    );
  }

  /**
   * List databases associated with a template
   * @param name - Template name
   */
  async listTemplateDatabases(name: string): Promise<Result<string[]>> {
    return this.request<string[]>(
      "GET",
      `/templates/${encodeURIComponent(name)}/databases`
    );
  }

  /**
   * Get the template associated with a database
   * @param dbName - Database name
   * @returns Template or null if no template is associated
   */
  async getDatabaseTemplate(
    dbName: string
  ): Promise<Result<Schemas["SchemaTemplate"] | null>> {
    return this.request<Schemas["SchemaTemplate"] | null>(
      "GET",
      `/db/${encodeURIComponent(dbName)}/template`
    );
  }

  /**
   * Associate a database with a template
   * @param dbName - Database name
   * @param templateName - Template name
   */
  async setDatabaseTemplate(
    dbName: string,
    templateName: string
  ): Promise<Result<Schemas["MessageResponse"]>> {
    return this.request<Schemas["MessageResponse"]>(
      "PUT",
      `/db/${encodeURIComponent(dbName)}/template`,
      { body: { templateName } }
    );
  }

  /**
   * Remove the template association from a database
   * @param dbName - Database name
   */
  async removeDatabaseTemplate(
    dbName: string
  ): Promise<Result<Schemas["MessageResponse"]>> {
    return this.request<Schemas["MessageResponse"]>(
      "DELETE",
      `/db/${encodeURIComponent(dbName)}/template`
    );
  }

  /**
   * Sync a database to its associated template
   * @param dbName - Database name
   * @param dropExtra - If true, drop tables not in the template
   */
  async syncDatabaseToTemplate(
    dbName: string,
    dropExtra = false
  ): Promise<Result<Schemas["DatabaseSyncResult"]>> {
    const params = new URLSearchParams();
    if (dropExtra) params.set("dropExtra", "true");
    return this.request<Schemas["DatabaseSyncResult"]>(
      "POST",
      `/db/${encodeURIComponent(dbName)}/sync`,
      { params }
    );
  }

  // Internal methods for TableQuery
  _request = this.request.bind(this);
  _rawRequest = this.rawRequest.bind(this);
}

/**
 * Fluent query builder for table operations
 */
export class TableQuery<T extends Record<string, unknown>> {
  private client: AtomicbaseClient;
  private table: string;
  private options: QueryOptions<T> = {};

  constructor(client: AtomicbaseClient, table: string) {
    this.client = client;
    this.table = table;
  }

  /**
   * Select specific columns (supports nested relations)
   * @example select("id,name,posts(title,body)")
   */
  select(columns: string): this {
    this.options.select = columns;
    return this;
  }

  /**
   * Filter rows
   * @example filter({ id: { eq: 1 } })
   * @example filter({ name: { like: "%john%" } })
   */
  filter(filter: Filter<T>): this {
    this.options.filter = { ...this.options.filter, ...filter };
    return this;
  }

  /**
   * Shorthand for equality filter
   * @example eq("id", 1)
   */
  eq<K extends keyof T>(column: K, value: T[K]): this {
    const filter = { [column]: { eq: value } } as unknown as Filter<T>;
    return this.filter(filter);
  }

  /**
   * Full-text search filter (requires FTS index on the table)
   * @param column - The column to search (must be in an FTS index)
   * @param query - The search query string
   * @example fts("title", "sqlite database")
   * @example fts("content", "full text search")
   */
  fts<K extends keyof T>(column: K, query: string): this {
    const filter = { [column]: { fts: query } } as unknown as Filter<T>;
    return this.filter(filter);
  }

  /**
   * Negated filter - prefix any operator with NOT
   * @param column - The column to filter on
   * @param op - The operator to negate (eq, like, in, is, gt, gte, lt, lte)
   * @param value - The filter value
   * @example not("status", "eq", "active") // status != 'active'
   * @example not("email", "is", "null") // email IS NOT NULL
   * @example not("role", "in", ["admin", "moderator"]) // role NOT IN ('admin', 'moderator')
   * @example not("name", "like", "%test%") // name NOT LIKE '%test%'
   */
  not<K extends keyof T, Op extends NegatableOperator>(
    column: K,
    op: Op,
    value: FilterValue<Op>
  ): this {
    const negatedOp = `not.${op}`;
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const filter = { [column]: { [negatedOp]: value } } as any;
    this.options.filter = { ...this.options.filter, ...filter };
    return this;
  }

  /**
   * Order results
   * @example order("created_at", "desc")
   */
  order(column: keyof T, direction: OrderDirection = "asc"): this {
    this.options.order = { column, direction };
    return this;
  }

  /**
   * Limit number of results
   */
  limit(count: number): this {
    this.options.limit = count;
    return this;
  }

  /**
   * Skip rows (for pagination)
   */
  offset(count: number): this {
    this.options.offset = count;
    return this;
  }

  private buildParams(): URLSearchParams {
    const params = this.options.filter
      ? buildFilterParams(this.options.filter)
      : new URLSearchParams();

    if (this.options.select) {
      params.set("select", this.options.select);
    }
    if (this.options.order) {
      params.set(
        "order",
        `${String(this.options.order.column)}:${this.options.order.direction}`
      );
    }
    if (this.options.limit !== undefined) {
      params.set("limit", String(this.options.limit));
    }
    if (this.options.offset !== undefined) {
      params.set("offset", String(this.options.offset));
    }
    return params;
  }

  /**
   * Execute SELECT query
   * @example const { data, error } = await client.from("users").get();
   */
  async get(): Promise<Result<T[]>> {
    const path = `/query/${encodeURIComponent(this.table)}`;
    return this.client._request<T[]>("GET", path, {
      params: this.buildParams(),
    });
  }

  /**
   * Execute SELECT query and include total count (ignoring limit/offset)
   * Returns both data and count via X-Total-Count header
   * @example const { data, error } = await client.from("users").getWithCount();
   */
  async getWithCount(): Promise<Result<CountResult<T>>> {
    const path = `/query/${encodeURIComponent(this.table)}`;
    const { data: response, error } = await this.client._rawRequest(
      "GET",
      path,
      {
        params: this.buildParams(),
        headers: { Prefer: "count=exact" },
      }
    );

    if (error || !response) {
      return { data: null, error };
    }

    const countHeader = response.headers.get("X-Total-Count");
    const count = countHeader ? parseInt(countHeader, 10) : 0;

    try {
      const text = await response.text();
      const data = text ? (JSON.parse(text) as T[]) : [];
      return { data: { data, count }, error: null };
    } catch (err) {
      return {
        data: null,
        error: new AtomicbaseError("Failed to parse response", 0, "PARSE_ERROR"),
      };
    }
  }

  /**
   * Get only the count of matching rows (no data returned)
   * @example const { data: count, error } = await client.from("users").count();
   */
  async count(): Promise<Result<number>> {
    const params = this.buildParams();
    params.set("count", "only");

    const path = `/query/${encodeURIComponent(this.table)}`;
    const { data, error } = await this.client._request<{ count: number }>(
      "GET",
      path,
      { params }
    );

    if (error || !data) {
      return { data: null, error };
    }

    return { data: data.count, error: null };
  }

  /**
   * Get a single row (returns error if not found or multiple)
   * @example const { data: user, error } = await client.from("users").eq("id", 1).single();
   */
  async single(): Promise<Result<T>> {
    const { data: results, error } = await this.limit(2).get();

    if (error) {
      return { data: null, error };
    }

    if (!results || results.length === 0) {
      return {
        data: null,
        error: new AtomicbaseError("No rows returned", 404, "NOT_FOUND"),
      };
    }

    if (results.length > 1) {
      return {
        data: null,
        error: new AtomicbaseError(
          "Multiple rows returned",
          400,
          "MULTIPLE_ROWS"
        ),
      };
    }

    return { data: results[0]!, error: null };
  }

  /**
   * Get first row or null (no error if not found)
   * @example const { data: user, error } = await client.from("users").maybeSingle();
   */
  async maybeSingle(): Promise<Result<T | null>> {
    const { data: results, error } = await this.limit(1).get();

    if (error) {
      return { data: null, error };
    }

    return { data: results?.[0] ?? null, error: null };
  }

  /**
   * Insert a row
   * @example const { data, error } = await client.from("users").insert({ name: "John" });
   */
  async insert(
    data: Partial<T>,
    options?: { returning?: string }
  ): Promise<Result<Schemas["InsertResponse"] | T[]>> {
    const params = new URLSearchParams();
    if (options?.returning) params.set("select", options.returning);

    return this.client._request(
      "POST",
      `/query/${encodeURIComponent(this.table)}`,
      {
        params,
        body: data,
      }
    );
  }

  /**
   * Upsert rows (insert or update on conflict)
   * @example const { data, error } = await client.from("users").upsert([{ id: 1, name: "John" }]);
   */
  async upsert(
    data: Partial<T>[],
    options?: { returning?: string }
  ): Promise<Result<Schemas["RowsAffectedResponse"] | T[]>> {
    const params = new URLSearchParams();
    if (options?.returning) params.set("select", options.returning);

    return this.client._request(
      "POST",
      `/query/${encodeURIComponent(this.table)}`,
      {
        params,
        body: data,
        headers: { Prefer: "resolution=merge-duplicates" },
      }
    );
  }

  /**
   * Update rows matching filter
   * @example const { data, error } = await client.from("users").eq("id", 1).update({ name: "Jane" });
   */
  async update(
    data: Partial<T>,
    options?: { returning?: string }
  ): Promise<Result<Schemas["RowsAffectedResponse"] | T[]>> {
    const params = this.buildParams();
    if (options?.returning) params.set("select", options.returning);

    return this.client._request(
      "PATCH",
      `/query/${encodeURIComponent(this.table)}`,
      {
        params,
        body: data,
      }
    );
  }

  /**
   * Delete rows matching filter
   * @example const { data, error } = await client.from("users").eq("id", 1).delete();
   */
  async delete(options?: {
    returning?: string;
  }): Promise<Result<Schemas["RowsAffectedResponse"] | T[]>> {
    const params = this.buildParams();
    if (options?.returning) params.set("select", options.returning);

    return this.client._request(
      "DELETE",
      `/query/${encodeURIComponent(this.table)}`,
      {
        params,
      }
    );
  }
}

/**
 * Create an Atomicbase client
 * @example
 * const client = createClient({ baseUrl: "http://localhost:8080" });
 * const { data, error } = await client.from("users").get();
 * if (error) {
 *   console.error("Error:", error.message);
 * } else {
 *   console.log("Users:", data);
 * }
 */
export function createClient(config: AtomicbaseConfig): AtomicbaseClient {
  return new AtomicbaseClient(config);
}
