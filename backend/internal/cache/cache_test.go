package cache

import (
	"sync"
	"testing"
	"time"
)

func TestTTLCache_GetSet(t *testing.T) {
	c := NewTTLCache[string, string](1 * time.Second)
	defer c.Stop()

	// Miss on empty cache
	if _, ok := c.Get("key1"); ok {
		t.Error("expected miss on empty cache")
	}

	c.Set("key1", "value1")

	v, ok := c.Get("key1")
	if !ok {
		t.Error("expected hit after Set")
	}
	if v != "value1" {
		t.Errorf("expected 'value1', got %q", v)
	}
}

func TestTTLCache_Expiry(t *testing.T) {
	ttl := 50 * time.Millisecond
	c := NewTTLCache[string, string](ttl)
	defer c.Stop()

	c.Set("k", "v")

	// Entry should be present before TTL expires
	if _, ok := c.Get("k"); !ok {
		t.Error("expected hit before TTL expired")
	}

	// Wait for TTL to pass
	time.Sleep(ttl + 20*time.Millisecond)

	if _, ok := c.Get("k"); ok {
		t.Error("expected miss after TTL expired")
	}
}

func TestTTLCache_Delete(t *testing.T) {
	c := NewTTLCache[string, int](1 * time.Second)
	defer c.Stop()

	c.Set("a", 1)
	c.Set("b", 2)

	c.Delete("a")

	if _, ok := c.Get("a"); ok {
		t.Error("expected 'a' to be deleted")
	}
	if _, ok := c.Get("b"); !ok {
		t.Error("expected 'b' to still be present")
	}
}

func TestTTLCache_Clear(t *testing.T) {
	c := NewTTLCache[string, string](1 * time.Second)
	defer c.Stop()

	c.Set("x", "1")
	c.Set("y", "2")
	c.Clear()

	if _, ok := c.Get("x"); ok {
		t.Error("expected cache to be cleared")
	}
	if _, ok := c.Get("y"); ok {
		t.Error("expected cache to be cleared")
	}
}

func TestTTLCache_DeleteMatchingPrefix(t *testing.T) {
	c := NewTTLCache[string, string](1 * time.Second)
	defer c.Stop()

	c.Set("org1:entity1", "a")
	c.Set("org1:entity2", "b")
	c.Set("org2:entity1", "c")

	c.DeleteMatchingPrefix("org1:")

	if _, ok := c.Get("org1:entity1"); ok {
		t.Error("expected 'org1:entity1' to be deleted")
	}
	if _, ok := c.Get("org1:entity2"); ok {
		t.Error("expected 'org1:entity2' to be deleted")
	}
	if _, ok := c.Get("org2:entity1"); !ok {
		t.Error("expected 'org2:entity1' to still be present")
	}
}

func TestTTLCache_ConcurrentAccess(t *testing.T) {
	c := NewTTLCache[string, int](500 * time.Millisecond)
	defer c.Stop()

	const goroutines = 20
	const iterations = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				key := "key"
				c.Set(key, id*1000+j)
				c.Get(key)
				if j%10 == 0 {
					c.Delete(key)
				}
			}
		}(i)
	}

	wg.Wait()
	// No data race or panic is the test passing criterion (run with -race)
}

func TestTTLCache_StructValues(t *testing.T) {
	type item struct {
		Name string
		Age  int
	}

	c := NewTTLCache[string, *item](1 * time.Second)
	defer c.Stop()

	c.Set("alice", &item{Name: "Alice", Age: 30})

	v, ok := c.Get("alice")
	if !ok {
		t.Fatal("expected hit")
	}
	if v.Name != "Alice" || v.Age != 30 {
		t.Errorf("unexpected value: %+v", v)
	}
}
