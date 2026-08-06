package web

import (
	"sync"
	"time"
)

const (
	defaultCacheTTL = 15 * time.Minute
	defaultCacheMax = 128
)

type cacheEntry struct {
	value     []byte
	expiresAt time.Time
}

type ttlCache struct {
	mu      sync.Mutex
	ttl     time.Duration
	max     int
	entries map[string]cacheEntry
	order   []string
}

func newTTLCache(ttl time.Duration, max int) *ttlCache {
	if ttl <= 0 {
		ttl = defaultCacheTTL
	}
	if max <= 0 {
		max = defaultCacheMax
	}
	return &ttlCache{
		ttl:     ttl,
		max:     max,
		entries: make(map[string]cacheEntry),
		order:   make([]string, 0, max),
	}
}

func (c *ttlCache) get(key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	ent, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	if time.Now().After(ent.expiresAt) {
		delete(c.entries, key)
		c.removeOrder(key)
		return nil, false
	}
	// copy to avoid callers mutating cache
	out := make([]byte, len(ent.value))
	copy(out, ent.value)
	return out, true
}

func (c *ttlCache) set(key string, value []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()

	buf := make([]byte, len(value))
	copy(buf, value)

	if _, exists := c.entries[key]; !exists {
		if len(c.order) >= c.max {
			oldest := c.order[0]
			c.order = c.order[1:]
			delete(c.entries, oldest)
		}
		c.order = append(c.order, key)
	}
	c.entries[key] = cacheEntry{
		value:     buf,
		expiresAt: time.Now().Add(c.ttl),
	}
}

func (c *ttlCache) removeOrder(key string) {
	for i, k := range c.order {
		if k == key {
			c.order = append(c.order[:i], c.order[i+1:]...)
			return
		}
	}
}
