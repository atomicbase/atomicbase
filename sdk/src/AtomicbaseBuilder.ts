import { AtomicbaseError } from "./AtomicbaseError.js";
import type {
  AtomicbaseResponse,
  AtomicbaseResponseWithCount,
  QueryState,
} from "./types.js";

/**
 * Base builder class that handles request execution, abort signals, and error modes.
 * This is the foundation of the builder chain.
 */
export abstract class AtomicbaseBuilder<T> implements PromiseLike<AtomicbaseResponse<T>> {
  protected state: QueryState;
  protected baseUrl: string;
  protected apiKey?: string;
  protected tenant?: string;
  protected fetchFn: typeof fetch;
  protected defaultHeaders: Record<string, string>;

  /** AbortSignal for canceling requests */
  protected signal?: AbortSignal;
  /** Whether to throw errors instead of returning them */
  protected shouldThrowOnError = false;

  constructor(config: {
    state: QueryState;
    baseUrl: string;
    apiKey?: string;
    tenant?: string;
    fetch: typeof fetch;
    headers?: Record<string, string>;
  }) {
    this.state = config.state;
    this.baseUrl = config.baseUrl;
    this.apiKey = config.apiKey;
    this.tenant = config.tenant;
    this.fetchFn = config.fetch;
    this.defaultHeaders = config.headers ?? {};
  }

  // ---------------------------------------------------------------------------
  // AbortSignal Support
  // ---------------------------------------------------------------------------

  /**
   * Set an AbortSignal to cancel the request.
   *
   * @example
   * ```ts
   * const controller = new AbortController()
   *
   * // Cancel after 5 seconds
   * setTimeout(() => controller.abort(), 5000)
   *
   * const { data, error } = await client
   *   .from('users')
   *   .select()
   *   .abortSignal(controller.signal)
   * ```
   */
  abortSignal(signal: AbortSignal): this {
    this.signal = signal;
    return this;
  }

  // ---------------------------------------------------------------------------
  // Error Mode Toggle
  // ---------------------------------------------------------------------------

  /**
   * Throw errors instead of returning them in the response.
   *
   * By default, errors are returned in the response object:
   * ```ts
   * const { data, error } = await client.from('users').select()
   * if (error) { ... }
   * ```
   *
   * With throwOnError(), errors are thrown as exceptions:
   * ```ts
   * try {
   *   const { data } = await client.from('users').select().throwOnError()
   * } catch (error) {
   *   // error is AtomicbaseError
   * }
   * ```
   */
  throwOnError(): this {
    this.shouldThrowOnError = true;
    return this;
  }

  // ---------------------------------------------------------------------------
  // Promise Implementation (Lazy Execution)
  // ---------------------------------------------------------------------------

  /**
   * Implements PromiseLike for lazy query execution.
   * The query only executes when awaited or .then() is called.
   */
  then<TResult1 = AtomicbaseResponse<T>, TResult2 = never>(
    onfulfilled?: ((value: AtomicbaseResponse<T>) => TResult1 | PromiseLike<TResult1>) | null,
    onrejected?: ((reason: unknown) => TResult2 | PromiseLike<TResult2>) | null
  ): Promise<TResult1 | TResult2> {
    return this.execute().then(onfulfilled, onrejected);
  }

  // ---------------------------------------------------------------------------
  // Request Building (Abstract - implemented by subclasses)
  // ---------------------------------------------------------------------------

  protected abstract buildRequest(): {
    url: string;
    headers: Record<string, string>;
    body: Record<string, unknown>;
  };

  // ---------------------------------------------------------------------------
  // Request Execution
  // ---------------------------------------------------------------------------

  protected async execute(): Promise<AtomicbaseResponse<T>> {
    const { url, headers, body } = this.buildRequest();

    try {
      const response = await this.fetchFn(url, {
        method: "POST",
        headers,
        body: JSON.stringify(body),
        signal: this.signal,
      });

      if (!response.ok) {
        const errorBody = await response.json().catch(() => ({}));
        const error = AtomicbaseError.fromResponse(errorBody, response.status);

        if (this.shouldThrowOnError) {
          throw error;
        }
        return { data: null, error };
      }

      const text = await response.text();
      const data = text ? JSON.parse(text) : null;
      return { data, error: null };
    } catch (err) {
      // Re-throw if it's already an AtomicbaseError (from throwOnError)
      if (err instanceof AtomicbaseError) {
        throw err;
      }

      // Handle abort
      if (err instanceof DOMException && err.name === "AbortError") {
        const error = new AtomicbaseError({
          message: "Request was aborted",
          code: "ABORTED",
          status: 0,
          hint: "The request was canceled via AbortSignal",
        });
        if (this.shouldThrowOnError) {
          throw error;
        }
        return { data: null, error };
      }

      // Network/other errors
      const error = AtomicbaseError.networkError(err);
      if (this.shouldThrowOnError) {
        throw error;
      }
      return { data: null, error };
    }
  }

  protected async executeWithCount(): Promise<AtomicbaseResponseWithCount<T>> {
    const { url, headers, body } = this.buildRequest();

    try {
      const response = await this.fetchFn(url, {
        method: "POST",
        headers,
        body: JSON.stringify(body),
        signal: this.signal,
      });

      if (!response.ok) {
        const errorBody = await response.json().catch(() => ({}));
        const error = AtomicbaseError.fromResponse(errorBody, response.status);

        if (this.shouldThrowOnError) {
          throw error;
        }
        return { data: null, count: null, error };
      }

      const countHeader = response.headers.get("X-Total-Count");
      const count = countHeader ? parseInt(countHeader, 10) : null;

      const text = await response.text();
      const data = text ? JSON.parse(text) : null;
      return { data, count, error: null };
    } catch (err) {
      if (err instanceof AtomicbaseError) {
        throw err;
      }

      if (err instanceof DOMException && err.name === "AbortError") {
        const error = new AtomicbaseError({
          message: "Request was aborted",
          code: "ABORTED",
          status: 0,
        });
        if (this.shouldThrowOnError) {
          throw error;
        }
        return { data: null, count: null, error };
      }

      const error = AtomicbaseError.networkError(err);
      if (this.shouldThrowOnError) {
        throw error;
      }
      return { data: null, count: null, error };
    }
  }

  // ---------------------------------------------------------------------------
  // Common Header Building
  // ---------------------------------------------------------------------------

  protected buildCommonHeaders(): Record<string, string> {
    const headers: Record<string, string> = {
      "Content-Type": "application/json",
      ...this.defaultHeaders,
    };

    if (this.apiKey) {
      headers["Authorization"] = `Bearer ${this.apiKey}`;
    }

    if (this.tenant) {
      headers["Tenant"] = this.tenant;
    }

    return headers;
  }
}
