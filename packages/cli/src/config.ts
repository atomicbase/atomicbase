import { existsSync } from "node:fs";
import { resolve } from "node:path";
import { pathToFileURL } from "node:url";

export interface AtomicbaseConfig {
  url: string;
  apiKey?: string;
  schemas: string;
  output: string;
}

const DEFAULT_CONFIG: AtomicbaseConfig = {
  url: "http://localhost:8080",
  schemas: "./schemas",
  output: "./schemas",
};

const CONFIG_FILES = [
  "atomicbase.config.ts",
  "atomicbase.config.js",
  "atomicbase.config.mjs",
];

/**
 * Load configuration from file and environment variables.
 * Priority: env vars > config file > defaults
 */
export async function loadConfig(): Promise<AtomicbaseConfig> {
  let fileConfig: Partial<AtomicbaseConfig> = {};

  // Try to load config file
  for (const filename of CONFIG_FILES) {
    const configPath = resolve(process.cwd(), filename);
    if (existsSync(configPath)) {
      try {
        const module = await import(pathToFileURL(configPath).href);
        fileConfig = module.default ?? module;
        break;
      } catch (err) {
        console.error(`Error loading ${filename}:`, err);
      }
    }
  }

  // Merge: defaults < file config < env vars
  return {
    url: process.env.ATOMICBASE_URL ?? fileConfig.url ?? DEFAULT_CONFIG.url,
    apiKey: process.env.ATOMICBASE_API_KEY ?? fileConfig.apiKey,
    schemas: fileConfig.schemas ?? DEFAULT_CONFIG.schemas,
    output: fileConfig.output ?? DEFAULT_CONFIG.output,
  };
}

/**
 * Helper to define config with type checking.
 * Used in atomicbase.config.ts files.
 */
export function defineConfig(config: Partial<AtomicbaseConfig>): AtomicbaseConfig {
  return {
    ...DEFAULT_CONFIG,
    ...config,
  };
}
