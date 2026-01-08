import type { components } from "./types.js";

export type Schemas = components["schemas"];

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
  constructor(message: string, public status: number, public body?: unknown) {
    super(message);
    this.name = "AtomicbaseError";
  }
}

function buildFilterParams(filter: Filter): URLSearchParams {
  const params = new URLSearchParams();
  for (const [column, operators] of Object.entries(filter)) {
    if (!operators) continue;
    for (const [op, value] of Object.entries(operators)) {
      if (value === undefined) continue;
      if (op === "in" && Array.isArray(value)) {
        params.set(column, `in.(${value.join(",")})`);
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
  ): Promise<T> {
    const response = await this.rawRequest(method, path, options);

    // Handle empty responses
    const text = await response.text();
    if (!text) return undefined as T;
    return JSON.parse(text) as T;
  }

  private async rawRequest(
    method: string,
    path: string,
    options: {
      params?: URLSearchParams;
      body?: unknown;
      headers?: Record<string, string>;
    } = {}
  ): Promise<Response> {
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
      const body = await response.json().catch(() => undefined);
      throw new AtomicbaseError(
        body?.error ?? `Request failed with status ${response.status}`,
        response.status,
        body
      );
    }

    return response;
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
  async health(): Promise<Schemas["HealthResponse"]> {
    return this.request<Schemas["HealthResponse"]>("GET", "/health");
  }

  /**
   * Execute a DDL query (CREATE, ALTER, DROP)
   */
  async executeSchema(
    query: string,
    args?: unknown[]
  ): Promise<Schemas["MessageResponse"]> {
    return this.request<Schemas["MessageResponse"]>("POST", "/schema", {
      body: { query, args },
    });
  }

  /**
   * Invalidate schema cache
   */
  async invalidateSchema(): Promise<Schemas["MessageResponse"]> {
    return this.request<Schemas["MessageResponse"]>(
      "POST",
      "/schema/invalidate"
    );
  }

  /**
   * Get table schema
   */
  async getTableSchema(table: string): Promise<Schemas["TableSchemaResponse"]> {
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
  ): Promise<Schemas["MessageResponse"]> {
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
  ): Promise<Schemas["MessageResponse"]> {
    return this.request<Schemas["MessageResponse"]>(
      "PATCH",
      `/schema/table/${encodeURIComponent(table)}`,
      { body: changes }
    );
  }

  /**
   * Drop a table
   */
  async dropTable(table: string): Promise<Schemas["MessageResponse"]> {
    return this.request<Schemas["MessageResponse"]>(
      "DELETE",
      `/schema/table/${encodeURIComponent(table)}`
    );
  }

  /**
   * Create an FTS5 (Full-Text Search) index on a table
   * @param table - The table to create the FTS index on
   * @param columns - Array of TEXT columns to include in the FTS index
   * @example await client.createFTSIndex("articles", ["title", "content"]);
   */
  async createFTSIndex(
    table: string,
    columns: string[]
  ): Promise<Schemas["MessageResponse"]> {
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
  async dropFTSIndex(table: string): Promise<Schemas["MessageResponse"]> {
    return this.request<Schemas["MessageResponse"]>(
      "DELETE",
      `/schema/fts/${encodeURIComponent(table)}`
    );
  }

  /**
   * List all FTS indexes
   * @returns Array of FTS index info (table, ftsTable, columns)
   */
  async listFTSIndexes(): Promise<Schemas["FTSIndexInfo"][]> {
    return this.request<Schemas["FTSIndexInfo"][]>("GET", "/schema/fts");
  }

  /**
   * List registered databases
   */
  async listDatabases(): Promise<Schemas["DatabaseInfo"][]> {
    return this.request<Schemas["DatabaseInfo"][]>("GET", "/db");
  }

  /**
   * Create a new Turso database
   */
  async createDatabase(
    name: string,
    group?: string
  ): Promise<Schemas["MessageResponse"]> {
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
  ): Promise<Schemas["MessageResponse"]> {
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
  async registerAllDatabases(): Promise<void> {
    return this.request<void>("PATCH", "/db/all");
  }

  /**
   * Delete a database
   */
  async deleteDatabase(name: string): Promise<Schemas["MessageResponse"]> {
    return this.request<Schemas["MessageResponse"]>(
      "DELETE",
      `/db/${encodeURIComponent(name)}`
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
   */
  async get(): Promise<T[]> {
    const path = `/query/${encodeURIComponent(this.table)}`;
    return this.client._request<T[]>("GET", path, {
      params: this.buildParams(),
    });
  }

  /**
   * Execute SELECT query and include total count (ignoring limit/offset)
   * Returns both data and count via X-Total-Count header
   * @example const { data, count } = await client.from("users").getWithCount();
   */
  async getWithCount(): Promise<CountResult<T>> {
    const path = `/query/${encodeURIComponent(this.table)}`;
    const response = await this.client._rawRequest("GET", path, {
      params: this.buildParams(),
      headers: { Prefer: "count=exact" },
    });

    const countHeader = response.headers.get("X-Total-Count");
    const count = countHeader ? parseInt(countHeader, 10) : 0;

    const text = await response.text();
    const data = text ? (JSON.parse(text) as T[]) : [];

    return { data, count };
  }

  /**
   * Get only the count of matching rows (no data returned)
   * @example const count = await client.from("users").count();
   */
  async count(): Promise<number> {
    const params = this.buildParams();
    params.set("count", "only");

    const path = `/query/${encodeURIComponent(this.table)}`;
    const result = await this.client._request<{ count: number }>("GET", path, {
      params,
    });
    return result.count;
  }

  /**
   * Get a single row (throws if not found or multiple)
   */
  async single(): Promise<T> {
    const results = await this.limit(2).get();
    if (results.length === 0) {
      throw new AtomicbaseError("No rows returned", 404);
    }
    if (results.length > 1) {
      throw new AtomicbaseError("Multiple rows returned", 400);
    }
    return results[0]!;
  }

  /**
   * Get first row or null
   */
  async maybeSingle(): Promise<T | null> {
    const results = await this.limit(1).get();
    return results[0] ?? null;
  }

  /**
   * Insert a row
   */
  async insert(
    data: Partial<T>,
    options?: { returning?: string }
  ): Promise<Schemas["InsertResponse"] | T[]> {
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
   */
  async upsert(
    data: Partial<T>[],
    options?: { returning?: string }
  ): Promise<Schemas["RowsAffectedResponse"] | T[]> {
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
   */
  async update(
    data: Partial<T>,
    options?: { returning?: string }
  ): Promise<Schemas["RowsAffectedResponse"] | T[]> {
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
   */
  async delete(options?: {
    returning?: string;
  }): Promise<Schemas["RowsAffectedResponse"] | T[]> {
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
 */
export function createClient(config: AtomicbaseConfig): AtomicbaseClient {
  return new AtomicbaseClient(config);
}
