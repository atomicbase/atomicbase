package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

// CachedTemplate holds schema bytes and version together.
// Schema is stored as JSON bytes to preserve type information
// when unmarshaled by the caller.
type CachedTemplate struct {
	SchemaJSON []byte `json:"schema"`
	Version    int    `json:"version"`
}

// Global cache instance
var cache Cache

// InitCache initializes the global cache instance.
func InitCache(c Cache) {
	cache = c
}

// GetCache returns the global cache instance.
func GetCache() Cache {
	return cache
}

// SetTemplate stores the current schema and version for a template.
// The schema is marshaled to JSON bytes for type-safe storage.
func SetTemplate(templateID int32, version int, schema any) {
	if cache == nil {
		return
	}

	schemaJSON, err := json.Marshal(schema)
	if err != nil {
		return
	}

	key := fmt.Sprintf("template:%d", templateID)
	data, err := json.Marshal(CachedTemplate{SchemaJSON: schemaJSON, Version: version})
	if err != nil {
		return
	}
	cache.Set(context.Background(), key, data)
}

// GetTemplate retrieves the cached template (schema bytes + version).
// Returns the cached template and true if found, empty struct and false otherwise.
// Caller should unmarshal SchemaJSON to the appropriate type.
func GetTemplate(templateID int32) (CachedTemplate, bool) {
	if cache == nil {
		return CachedTemplate{}, false
	}
	key := fmt.Sprintf("template:%d", templateID)
	data, err := cache.Get(context.Background(), key)
	if err != nil || data == nil {
		return CachedTemplate{}, false
	}
	var cached CachedTemplate
	if err := json.Unmarshal(data, &cached); err != nil {
		return CachedTemplate{}, false
	}
	return cached, true
}

// InvalidateTemplate removes a template from cache.
func InvalidateTemplate(templateID int32) {
	if cache == nil {
		return
	}
	key := fmt.Sprintf("template:%d", templateID)
	cache.Delete(context.Background(), key)
}

// CachedDatabase holds database metadata for cache lookups.
type CachedDatabase struct {
	ID              int32 `json:"id"`
	TemplateID      int32 `json:"template_id"`
	DatabaseVersion int   `json:"version"`
}

// SetDatabase stores database metadata in cache.
func SetDatabase(name string, meta CachedDatabase) {
	if cache == nil {
		return
	}
	key := fmt.Sprintf("db:%s", name)
	data, err := json.Marshal(meta)
	if err != nil {
		return
	}
	cache.Set(context.Background(), key, data)
}

// GetDatabase retrieves cached database metadata.
// Returns the cached metadata and true if found, empty struct and false otherwise.
func GetDatabase(name string) (CachedDatabase, bool) {
	if cache == nil {
		return CachedDatabase{}, false
	}
	key := fmt.Sprintf("db:%s", name)
	data, err := cache.Get(context.Background(), key)
	if err != nil || data == nil {
		return CachedDatabase{}, false
	}
	var meta CachedDatabase
	if err := json.Unmarshal(data, &meta); err != nil {
		return CachedDatabase{}, false
	}
	return meta, true
}

// InvalidateDatabase removes database metadata from cache.
func InvalidateDatabase(name string) {
	if cache == nil {
		return
	}
	key := fmt.Sprintf("db:%s", name)
	cache.Delete(context.Background(), key)
}

// UpdateDatabaseVersion updates just the version in cached database metadata.
func UpdateDatabaseVersion(name string, newVersion int) {
	if cache == nil {
		return
	}
	meta, ok := GetDatabase(name)
	if !ok {
		return
	}
	meta.DatabaseVersion = newVersion
	SetDatabase(name, meta)
}
