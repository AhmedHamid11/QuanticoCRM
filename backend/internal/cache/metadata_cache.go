package cache

import (
	"context"
	"time"

	"github.com/fastcrm/backend/internal/db"
	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/repo"
)

// DefaultMetadataTTL is the default TTL for metadata (entity defs, field defs, layouts).
const DefaultMetadataTTL = 30 * time.Second

// MetadataCache wraps MetadataRepo with an in-memory TTL cache.
//
// Cache keys are scoped by orgID so different orgs never share entries.
// Mutation operations (Create/Update/Delete) are NOT cached — callers should
// use Repo() to access the underlying repo for those, then call Invalidate* to
// remove stale entries.
//
// Multi-tenant safety: cache keys include orgID, so a cache hit for org A can
// never be served to org B.
type MetadataCache struct {
	repo            *repo.MetadataRepo
	entityCache     *TTLCache[string, *entity.EntityDef]   // key: "{orgID}:{entityName}"
	entityNameCache *TTLCache[string, string]               // key: "{orgID}:{urlPath}" -> canonical name
	fieldsCache     *TTLCache[string, []entity.FieldDef]    // key: "{orgID}:{entityName}"
	layoutCache     *TTLCache[string, *entity.LayoutDef]    // key: "{orgID}:{entityName}:{layoutType}"
}

// NewMetadataCache creates a MetadataCache that wraps repo with the given TTL.
// Pass DefaultMetadataTTL (30s) for production use.
func NewMetadataCache(metadataRepo *repo.MetadataRepo, ttl time.Duration) *MetadataCache {
	return &MetadataCache{
		repo:            metadataRepo,
		entityCache:     NewTTLCache[string, *entity.EntityDef](ttl),
		entityNameCache: NewTTLCache[string, string](ttl),
		fieldsCache:     NewTTLCache[string, []entity.FieldDef](ttl),
		layoutCache:     NewTTLCache[string, *entity.LayoutDef](ttl),
	}
}

// Stop halts all background cleanup goroutines for this cache's sub-caches.
func (mc *MetadataCache) Stop() {
	mc.entityCache.Stop()
	mc.entityNameCache.Stop()
	mc.fieldsCache.Stop()
	mc.layoutCache.Stop()
}

// Repo returns the underlying MetadataRepo for direct (non-cached) access.
// Use this for mutation operations (CreateEntity, UpdateEntity, DeleteField, etc.)
// and for EnsureSchema.
func (mc *MetadataCache) Repo() *repo.MetadataRepo {
	return mc.repo
}

// WithDB returns a new MetadataCache that uses the provided DB connection for
// its underlying repo, but SHARES the same TTL cache instances.
//
// Because cache keys always include orgID, sharing caches across DB connections
// is safe — an entry for org A will never be returned for org B.
//
// This is used in multi-tenant mode where each request may resolve a different
// tenant DB via middleware.
func (mc *MetadataCache) WithDB(dbConn db.DBConn) *MetadataCache {
	return &MetadataCache{
		repo:            mc.repo.WithDB(dbConn),
		entityCache:     mc.entityCache,
		entityNameCache: mc.entityNameCache,
		fieldsCache:     mc.fieldsCache,
		layoutCache:     mc.layoutCache,
	}
}

// --- Cache key helpers ---

func entityKey(orgID, entityName string) string    { return orgID + ":" + entityName }
func entityNameKey(orgID, urlPath string) string   { return orgID + ":" + urlPath }
func layoutKey(orgID, entityName, t string) string { return orgID + ":" + entityName + ":" + t }

// --- Cached read methods ---

// GetEntity returns an EntityDef by name, using the cache when possible.
func (mc *MetadataCache) GetEntity(ctx context.Context, orgID, name string) (*entity.EntityDef, error) {
	key := entityKey(orgID, name)
	if v, ok := mc.entityCache.Get(key); ok {
		return v, nil
	}
	v, err := mc.repo.GetEntity(ctx, orgID, name)
	if err != nil {
		return nil, err
	}
	if v != nil {
		mc.entityCache.Set(key, v)
	}
	return v, nil
}

// GetEntityByLowercaseName resolves a URL path (lowercased) to a canonical entity name,
// using the cache when possible.
func (mc *MetadataCache) GetEntityByLowercaseName(ctx context.Context, orgID, urlPath string) (string, error) {
	key := entityNameKey(orgID, urlPath)
	if v, ok := mc.entityNameCache.Get(key); ok {
		return v, nil
	}
	v, err := mc.repo.GetEntityByLowercaseName(ctx, orgID, urlPath)
	if err != nil {
		return "", err
	}
	// Cache even empty-string results to avoid repeated DB hits for unknown paths,
	// but only for non-empty strings (empty means not found — don't cache misses
	// because they'd hide future entity additions until TTL expires).
	if v != "" {
		mc.entityNameCache.Set(key, v)
	}
	return v, nil
}

// ListFields returns all field definitions for an entity, using the cache when possible.
func (mc *MetadataCache) ListFields(ctx context.Context, orgID, entityName string) ([]entity.FieldDef, error) {
	key := entityKey(orgID, entityName)
	if v, ok := mc.fieldsCache.Get(key); ok {
		return v, nil
	}
	v, err := mc.repo.ListFields(ctx, orgID, entityName)
	if err != nil {
		return nil, err
	}
	mc.fieldsCache.Set(key, v)
	return v, nil
}

// GetLayout returns a layout definition, using the cache when possible.
func (mc *MetadataCache) GetLayout(ctx context.Context, orgID, entityName, layoutType string) (*entity.LayoutDef, error) {
	key := layoutKey(orgID, entityName, layoutType)
	if v, ok := mc.layoutCache.Get(key); ok {
		return v, nil
	}
	v, err := mc.repo.GetLayout(ctx, orgID, entityName, layoutType)
	if err != nil {
		return nil, err
	}
	if v != nil {
		mc.layoutCache.Set(key, v)
	}
	return v, nil
}

// --- Invalidation methods ---

// InvalidateEntity removes all cached entries for the given entity in the given org.
// Call this after creating, updating, or deleting a field or entity.
func (mc *MetadataCache) InvalidateEntity(orgID, entityName string) {
	mc.entityCache.Delete(entityKey(orgID, entityName))
	mc.fieldsCache.Delete(entityKey(orgID, entityName))
	// Invalidate all layout types by prefix: "{orgID}:{entityName}:"
	mc.layoutCache.DeleteMatchingPrefix(orgID + ":" + entityName + ":")
}

// InvalidateOrg removes ALL cached entries for the given org.
// Call this after creating or deleting an entity (the entity list changes).
func (mc *MetadataCache) InvalidateOrg(orgID string) {
	prefix := orgID + ":"
	mc.entityCache.DeleteMatchingPrefix(prefix)
	mc.entityNameCache.DeleteMatchingPrefix(prefix)
	mc.fieldsCache.DeleteMatchingPrefix(prefix)
	mc.layoutCache.DeleteMatchingPrefix(prefix)
}

// InvalidateAll clears the entire cache (all orgs).
func (mc *MetadataCache) InvalidateAll() {
	mc.entityCache.Clear()
	mc.entityNameCache.Clear()
	mc.fieldsCache.Clear()
	mc.layoutCache.Clear()
}
