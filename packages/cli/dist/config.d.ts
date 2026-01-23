export interface AtomicbaseConfig {
    url: string;
    apiKey?: string;
    schemas: string;
    output: string;
}
/**
 * Load configuration from file and environment variables.
 * Priority: env vars > config file > defaults
 */
export declare function loadConfig(): Promise<AtomicbaseConfig>;
/**
 * Helper to define config with type checking.
 * Used in atomicbase.config.ts files.
 */
export declare function defineConfig(config: Partial<AtomicbaseConfig>): AtomicbaseConfig;
//# sourceMappingURL=config.d.ts.map