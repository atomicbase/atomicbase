package tools

import (
	"encoding/gob"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/atomicbase/atomicbase/config"
)

// CachedTemplate holds both schema and version together.
type CachedTemplate struct {
	Schema  any
	Version int
}

// Cache is the shared in-memory cache contract used by all API modules.
type Cache interface {
	SetTemplate(templateID int32, version int, schema any)
	GetTemplate(templateID int32) (CachedTemplate, bool)
	InvalidateTemplate(templateID int32)
	Load(path string) error
	Save(path string) error
	RegisterType(v any)
}

type memoryCache struct {
	templateCache sync.Map // int32 -> CachedTemplate
}

type persistedMemoryCache struct {
	Templates map[int32]CachedTemplate
}

const memoryCacheFileName = "memory_cache.gob"

func newMemoryCache() *memoryCache {
	return &memoryCache{}
}

var GlobalCache Cache = newMemoryCache()

func (c *memoryCache) SetTemplate(templateID int32, version int, schema any) {
	if c == nil {
		return
	}
	c.templateCache.Store(templateID, CachedTemplate{Schema: schema, Version: version})
}

// GetTemplate retrieves the cached template (schema + version).
// Returns the cached template and true if found, empty struct and false otherwise.
func (c *memoryCache) GetTemplate(templateID int32) (CachedTemplate, bool) {
	if c == nil {
		return CachedTemplate{}, false
	}
	v, ok := c.templateCache.Load(templateID)
	if !ok {
		return CachedTemplate{}, false
	}
	return v.(CachedTemplate), true
}

// InvalidateTemplate removes a template from cache.
func (c *memoryCache) InvalidateTemplate(templateID int32) {
	if c == nil {
		return
	}
	c.templateCache.Delete(templateID)
}

func (c *memoryCache) clearTemplates() {
	if c == nil {
		return
	}
	c.templateCache = sync.Map{}
}

func (c *memoryCache) allTemplates() map[int32]CachedTemplate {
	templates := make(map[int32]CachedTemplate)
	if c == nil {
		return templates
	}

	c.templateCache.Range(func(k, v any) bool {
		templateID, ok := k.(int32)
		if !ok {
			return true
		}
		cached, ok := v.(CachedTemplate)
		if !ok {
			return true
		}
		templates[templateID] = cached
		return true
	})

	return templates
}

func memoryCachePath() string {
	return filepath.Join(config.Cfg.DataDir, memoryCacheFileName)
}

func (c *memoryCache) RegisterType(v any) {
	gob.Register(v)
}

func (c *memoryCache) Load(path string) error {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	defer file.Close()

	var persisted persistedMemoryCache
	if err := gob.NewDecoder(file).Decode(&persisted); err != nil {
		return err
	}

	c.clearTemplates()
	for templateID, cached := range persisted.Templates {
		c.SetTemplate(templateID, cached.Version, cached.Schema)
	}

	return nil
}

func (c *memoryCache) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), os.ModePerm); err != nil {
		return err
	}

	persisted := persistedMemoryCache{Templates: c.allTemplates()}
	tmpPath := path + ".tmp"

	file, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

	if err := gob.NewEncoder(file).Encode(persisted); err != nil {
		_ = file.Close()
		_ = os.Remove(tmpPath)
		return err
	}

	if err := file.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}

	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}

	return nil
}

func RegisterMemoryCacheType(v any) {
	GlobalCache.RegisterType(v)
}

func LoadMemoryCache() error {
	return GlobalCache.Load(memoryCachePath())
}

func SaveMemoryCache() error {
	return GlobalCache.Save(memoryCachePath())
}

// SetTemplate stores the current schema and version for a template.
func SetTemplate(templateID int32, version int, schema any) {
	GlobalCache.SetTemplate(templateID, version, schema)
}

// GetTemplate retrieves the cached template (schema + version).
// Returns the cached template and true if found, empty struct and false otherwise.
func GetTemplate(templateID int32) (CachedTemplate, bool) {
	return GlobalCache.GetTemplate(templateID)
}

// InvalidateTemplate removes a template from cache.
func InvalidateTemplate(templateID int32) {
	GlobalCache.InvalidateTemplate(templateID)
}
