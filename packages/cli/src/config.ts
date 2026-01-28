import { existsSync } from "node:fs";
import { resolve } from "node:path";
import { createJiti } from "jiti";

export interface AtomicbaseConfig {
  url?: string;
  apiKey?: string;
  schemas: string;
  output?: string;
}

const DEFAULT_CONFIG: Required<AtomicbaseConfig> = {
  url: "http://localhost:8080",
  apiKey: "",
  schemas: "./schemas",
  output: "./schemas",
};

const CONFIG_FILES = [
  "atomicbase.config.ts",
  "atomicbase.config.js",
  "atomicbase.config.mjs",
];

// Create jiti instance for loading TypeScript config files
const jiti = createJiti(import.meta.url);

/**
 * Load configuration from file and environment variables.
 * Priority: env vars > config file > defaults
 */
export async function loadConfig(): Promise<Required<AtomicbaseConfig>> {
  let fileConfig: Partial<AtomicbaseConfig> = {};

  // Try to load config file using jiti (handles TypeScript natively)
  for (const filename of CONFIG_FILES) {
    const configPath = resolve(process.cwd(), filename);
    if (existsSync(configPath)) {
      try {
        const module = await jiti.import(configPath);
        fileConfig = (module as { default?: AtomicbaseConfig }).default ?? module as AtomicbaseConfig;
        break;
      } catch (err) {
        console.error(`Error loading ${filename}:`, err);
      }
    }
  }

  // Merge: defaults < file config < env vars
  return {
    url: process.env.ATOMICBASE_URL ?? fileConfig.url ?? DEFAULT_CONFIG.url,
    apiKey: process.env.ATOMICBASE_API_KEY ?? fileConfig.apiKey ?? DEFAULT_CONFIG.apiKey,
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
