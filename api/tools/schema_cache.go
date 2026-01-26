package tools

import "sync"

// CachedTemplate holds both schema and version together.
type CachedTemplate struct {
	Schema  any
	Version int
}

// Template cache - maps template_id to current schema + version
var templateCache sync.Map // int32 -> CachedTemplate

// SetTemplate stores the current schema and version for a template.
func SetTemplate(templateID int32, version int, schema any) {
	templateCache.Store(templateID, CachedTemplate{Schema: schema, Version: version})
}

// GetTemplate retrieves the cached template (schema + version).
// Returns the cached template and true if found, empty struct and false otherwise.
func GetTemplate(templateID int32) (CachedTemplate, bool) {
	v, ok := templateCache.Load(templateID)
	if !ok {
		return CachedTemplate{}, false
	}
	return v.(CachedTemplate), true
}

// InvalidateTemplate removes a template from cache.
func InvalidateTemplate(templateID int32) {
	templateCache.Delete(templateID)
}
