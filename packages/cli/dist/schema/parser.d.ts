import type { SchemaDefinition } from "@atomicbase/schema";
/**
 * Load a schema from a .schema.ts file.
 */
export declare function loadSchema(filePath: string): Promise<SchemaDefinition>;
/**
 * Find all schema files in a directory.
 */
export declare function findSchemaFiles(dir: string): string[];
/**
 * Load all schemas from a directory.
 */
export declare function loadAllSchemas(dir: string): Promise<SchemaDefinition[]>;
//# sourceMappingURL=parser.d.ts.map