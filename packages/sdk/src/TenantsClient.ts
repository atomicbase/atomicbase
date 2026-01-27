import { AtomicbaseError } from "./AtomicbaseError.js";
import type {
  AtomicbaseResponse,
  Tenant,
  CreateTenantOptions,
  SyncTenantResponse,
} from "./types.js";

/**
 * Client for managing tenants via the Platform API.
 *
 * @example
 * ```ts
 * const client = createClient({ url: 'http://localhost:8080', apiKey: 'key' })
 *
 * // List all tenants
 * const { data: tenants } = await client.tenants.list()
 *
 * // Create a new tenant
 * const { data: tenant } = await client.tenants.create({
 *   name: 'acme-corp',
 *   template: 'my-app'
 * })
 *
 * // Get tenant details
 * const { data: tenant } = await client.tenants.get('acme-corp')
 *
 * // Sync tenant to latest template version
 * const { data: result } = await client.tenants.sync('acme-corp')
 *
 * // Delete a tenant
 * const { error } = await client.tenants.delete('acme-corp')
 * ```
 */
export class TenantsClient {
  private readonly baseUrl: string;
  private readonly apiKey?: string;
  private readonly headers: Record<string, string>;
  private readonly fetchFn: typeof fetch;

  constructor(options: {
    baseUrl: string;
    apiKey?: string;
    headers: Record<string, string>;
    fetch: typeof fetch;
  }) {
    this.baseUrl = options.baseUrl;
    this.apiKey = options.apiKey;
    this.headers = options.headers;
    this.fetchFn = options.fetch;
  }

  private getHeaders(): Record<string, string> {
    const headers: Record<string, string> = {
      "Content-Type": "application/json",
      ...this.headers,
    };

    if (this.apiKey) {
      headers["Authorization"] = `Bearer ${this.apiKey}`;
    }

    return headers;
  }

  /**
   * List all tenants.
   *
   * @example
   * ```ts
   * const { data, error } = await client.tenants.list()
   * if (error) {
   *   console.error('Failed to list tenants:', error.message)
   * } else {
   *   console.log('Tenants:', data)
   * }
   * ```
   */
  async list(): Promise<AtomicbaseResponse<Tenant[]>> {
    try {
      const response = await this.fetchFn(`${this.baseUrl}/platform/tenants`, {
        method: "GET",
        headers: this.getHeaders(),
      });

      if (!response.ok) {
        const errorBody = await response.json().catch(() => ({}));
        const error = AtomicbaseError.fromResponse(errorBody, response.status);
        return { data: null, error };
      }

      const data = await response.json();
      return { data, error: null };
    } catch (err) {
      const error = AtomicbaseError.networkError(err);
      return { data: null, error };
    }
  }

  /**
   * Get a tenant by name.
   *
   * @example
   * ```ts
   * const { data, error } = await client.tenants.get('acme-corp')
   * if (error) {
   *   console.error('Tenant not found:', error.message)
   * } else {
   *   console.log('Tenant:', data)
   * }
   * ```
   */
  async get(name: string): Promise<AtomicbaseResponse<Tenant>> {
    try {
      const response = await this.fetchFn(
        `${this.baseUrl}/platform/tenants/${encodeURIComponent(name)}`,
        {
          method: "GET",
          headers: this.getHeaders(),
        }
      );

      if (!response.ok) {
        const errorBody = await response.json().catch(() => ({}));
        const error = AtomicbaseError.fromResponse(errorBody, response.status);
        return { data: null, error };
      }

      const data = await response.json();
      return { data, error: null };
    } catch (err) {
      const error = AtomicbaseError.networkError(err);
      return { data: null, error };
    }
  }

  /**
   * Create a new tenant database from a template.
   *
   * @example
   * ```ts
   * const { data, error } = await client.tenants.create({
   *   name: 'acme-corp',
   *   template: 'my-app'
   * })
   * if (error) {
   *   console.error('Failed to create tenant:', error.message)
   * } else {
   *   console.log('Created tenant:', data.name)
   * }
   * ```
   */
  async create(options: CreateTenantOptions): Promise<AtomicbaseResponse<Tenant>> {
    try {
      const response = await this.fetchFn(`${this.baseUrl}/platform/tenants`, {
        method: "POST",
        headers: this.getHeaders(),
        body: JSON.stringify(options),
      });

      if (!response.ok) {
        const errorBody = await response.json().catch(() => ({}));
        const error = AtomicbaseError.fromResponse(errorBody, response.status);
        return { data: null, error };
      }

      const data = await response.json();
      return { data, error: null };
    } catch (err) {
      const error = AtomicbaseError.networkError(err);
      return { data: null, error };
    }
  }

  /**
   * Delete a tenant database.
   *
   * @example
   * ```ts
   * const { error } = await client.tenants.delete('acme-corp')
   * if (error) {
   *   console.error('Failed to delete tenant:', error.message)
   * } else {
   *   console.log('Tenant deleted')
   * }
   * ```
   */
  async delete(name: string): Promise<AtomicbaseResponse<void>> {
    try {
      const response = await this.fetchFn(
        `${this.baseUrl}/platform/tenants/${encodeURIComponent(name)}`,
        {
          method: "DELETE",
          headers: this.getHeaders(),
        }
      );

      if (!response.ok) {
        const errorBody = await response.json().catch(() => ({}));
        const error = AtomicbaseError.fromResponse(errorBody, response.status);
        return { data: null, error };
      }

      // 204 No Content on success
      return { data: undefined as unknown as void, error: null };
    } catch (err) {
      const error = AtomicbaseError.networkError(err);
      return { data: null, error };
    }
  }

  /**
   * Sync a tenant to the latest template version.
   * This applies any pending schema migrations to the tenant database.
   *
   * @example
   * ```ts
   * const { data, error } = await client.tenants.sync('acme-corp')
   * if (error) {
   *   if (error.code === 'TENANT_IN_SYNC') {
   *     console.log('Tenant is already up to date')
   *   } else {
   *     console.error('Sync failed:', error.message)
   *   }
   * } else {
   *   console.log(`Synced from v${data.fromVersion} to v${data.toVersion}`)
   * }
   * ```
   */
  async sync(name: string): Promise<AtomicbaseResponse<SyncTenantResponse>> {
    try {
      const response = await this.fetchFn(
        `${this.baseUrl}/platform/tenants/${encodeURIComponent(name)}/sync`,
        {
          method: "POST",
          headers: this.getHeaders(),
        }
      );

      if (!response.ok) {
        const errorBody = await response.json().catch(() => ({}));
        const error = AtomicbaseError.fromResponse(errorBody, response.status);
        return { data: null, error };
      }

      const data = await response.json();
      return { data, error: null };
    } catch (err) {
      const error = AtomicbaseError.networkError(err);
      return { data: null, error };
    }
  }
}
