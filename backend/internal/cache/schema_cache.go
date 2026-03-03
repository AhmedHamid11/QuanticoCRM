package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/fastcrm/backend/internal/db"
)

// DefaultSchemaTTL is the default TTL for table-schema column-existence checks.
// Schema changes far less frequently than metadata, so a longer TTL is safe.
const DefaultSchemaTTL = 60 * time.Second

// SchemaCache caches per-connection, per-table column-existence results.
//
// Cache key includes the connection pointer address so that separate tenant DB
// connections each maintain their own schema cache entries — preventing a column
// that exists in one tenant's DB from being reported as present in another's.
//
// This replaces per-request PRAGMA table_info calls in GenericEntityHandler.
type SchemaCache struct {
	cache *TTLCache[string, bool]
}

// NewSchemaCache creates a SchemaCache with the given TTL.
// Pass DefaultSchemaTTL (60s) for production use.
func NewSchemaCache(ttl time.Duration) *SchemaCache {
	return &SchemaCache{
		cache: NewTTLCache[string, bool](ttl),
	}
}

// Stop halts the background cleanup goroutine.
func (sc *SchemaCache) Stop() {
	sc.cache.Stop()
}

// schemaKey builds the cache key for a (conn, table, column) triple.
// Using the pointer address of the connection as the db identifier keeps
// multi-tenant schemas isolated without needing any extra context.
func schemaKey(conn db.DBConn, tableName, columnName string) string {
	return fmt.Sprintf("%p:%s:%s", conn, tableName, columnName)
}

// HasColumn reports whether tableName contains a column named columnName in
// the database accessible via conn.
//
// On a cache miss the result is fetched via PRAGMA table_info and then cached.
// On any query error the result defaults to false (safe fallback).
func (sc *SchemaCache) HasColumn(ctx context.Context, conn db.DBConn, tableName, columnName string) bool {
	key := schemaKey(conn, tableName, columnName)
	if v, ok := sc.cache.Get(key); ok {
		return v
	}

	result := sc.queryHasColumn(ctx, conn, tableName, columnName)
	sc.cache.Set(key, result)
	return result
}

// queryHasColumn executes PRAGMA table_info to check column existence.
// This mirrors the logic from tableHasColumn in handler/generic_entity.go.
func (sc *SchemaCache) queryHasColumn(ctx context.Context, conn db.DBConn, tableName, columnName string) bool {
	query := fmt.Sprintf("PRAGMA table_info(%s)", tableName)
	rows, err := conn.QueryContext(ctx, query)
	if err != nil {
		return false
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, colType string
		var notNull int
		var dfltValue interface{}
		var pk int
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err != nil {
			continue
		}
		if name == columnName {
			return true
		}
	}
	return false
}

// InvalidateTable removes all cached column entries for the given table on the
// given connection. Call this after DDL changes (e.g., ALTER TABLE ADD COLUMN).
func (sc *SchemaCache) InvalidateTable(conn db.DBConn, tableName string) {
	prefix := fmt.Sprintf("%p:%s:", conn, tableName)
	sc.cache.DeleteMatchingPrefix(prefix)
}
