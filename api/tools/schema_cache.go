package tools

import "sync"

// schemaKey is the key for the schema cache map.
type schemaKey struct {
	TemplateID int32
	Version    int
}

// Global schema cache - maps (template_id, version) to schema value
var templateSchemaCache sync.Map

// SchemaCache stores a schema in the cache.
func SchemaCache(templateID int32, version int, value any) {
	key := schemaKey{TemplateID: templateID, Version: version}
	templateSchemaCache.Store(key, value)
}

// SchemaFromCache retrieves a schema from cache.
// Returns the value and true if found, nil and false otherwise.
func SchemaFromCache(templateID int32, version int) (any, bool) {
	key := schemaKey{TemplateID: templateID, Version: version}
	return templateSchemaCache.Load(key)
}

// InvalidateSchema removes a specific schema version from cache.
func InvalidateSchema(templateID int32, version int) {
	key := schemaKey{TemplateID: templateID, Version: version}
	templateSchemaCache.Delete(key)
}

// InvalidateAllSchemas removes all cached schemas for a template.
func InvalidateAllSchemas(templateID int32) {
	templateSchemaCache.Range(func(k, v any) bool {
		key := k.(schemaKey)
		if key.TemplateID == templateID {
			templateSchemaCache.Delete(k)
		}
		return true
	})
}
