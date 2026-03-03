package cache

import (
	"sync"
	"time"
)

// cacheEntry holds a cached value along with its expiry time.
type cacheEntry[V any] struct {
	value     V
	expiresAt time.Time
}

// TTLCache is a generic, concurrency-safe in-memory cache with time-based expiry.
// Keys must be comparable; values can be any type.
// Expired entries are evicted lazily on access and periodically by a background goroutine.
type TTLCache[K comparable, V any] struct {
	mu      sync.RWMutex
	entries map[K]cacheEntry[V]
	ttl     time.Duration
	stopCh  chan struct{}
}

// NewTTLCache creates a new TTLCache with the given TTL and starts a background
// cleanup goroutine that evicts expired entries every 2*ttl.
func NewTTLCache[K comparable, V any](ttl time.Duration) *TTLCache[K, V] {
	c := &TTLCache[K, V]{
		entries: make(map[K]cacheEntry[V]),
		ttl:     ttl,
		stopCh:  make(chan struct{}),
	}
	go c.cleanupLoop()
	return c
}

// cleanupLoop runs periodically and removes all expired entries to bound memory usage.
func (c *TTLCache[K, V]) cleanupLoop() {
	ticker := time.NewTicker(c.ttl * 2)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.evictExpired()
		case <-c.stopCh:
			return
		}
	}
}

// evictExpired removes all entries whose TTL has passed.
func (c *TTLCache[K, V]) evictExpired() {
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	for k, e := range c.entries {
		if now.After(e.expiresAt) {
			delete(c.entries, k)
		}
	}
}

// Stop halts the background cleanup goroutine.
// After calling Stop, the cache is still usable but entries will not be
// proactively evicted (they will still expire lazily on access).
func (c *TTLCache[K, V]) Stop() {
	close(c.stopCh)
}

// Get returns the cached value for key, and a boolean indicating whether
// the entry was found and has not yet expired.
// Expired entries are removed lazily during Get.
func (c *TTLCache[K, V]) Get(key K) (V, bool) {
	c.mu.RLock()
	e, ok := c.entries[key]
	c.mu.RUnlock()

	if !ok {
		var zero V
		return zero, false
	}

	if time.Now().After(e.expiresAt) {
		// Expired — remove it and return a miss
		c.mu.Lock()
		delete(c.entries, key)
		c.mu.Unlock()
		var zero V
		return zero, false
	}

	return e.value, true
}

// Set stores value under key with the cache's configured TTL.
func (c *TTLCache[K, V]) Set(key K, value V) {
	c.mu.Lock()
	c.entries[key] = cacheEntry[V]{
		value:     value,
		expiresAt: time.Now().Add(c.ttl),
	}
	c.mu.Unlock()
}

// Delete removes the entry for key (if any).
func (c *TTLCache[K, V]) Delete(key K) {
	c.mu.Lock()
	delete(c.entries, key)
	c.mu.Unlock()
}

// Clear removes all entries from the cache.
func (c *TTLCache[K, V]) Clear() {
	c.mu.Lock()
	c.entries = make(map[K]cacheEntry[V])
	c.mu.Unlock()
}

// DeleteMatchingPrefix removes all entries whose key, when converted to a string via
// fmt.Sprint, starts with prefix.  This is a no-op for non-string-keyed caches where
// the key's string form does not match the prefix convention.
// Use only with caches keyed on string (K = string).
func (c *TTLCache[K, V]) DeleteMatchingPrefix(prefix string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k := range c.entries {
		if ks, ok := any(k).(string); ok {
			if len(ks) >= len(prefix) && ks[:len(prefix)] == prefix {
				delete(c.entries, k)
			}
		}
	}
}
