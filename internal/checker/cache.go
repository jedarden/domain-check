package checker

import (
	"container/list"
	"sync"
	"time"

	"github.com/coding/domain-check/internal/domain"
)

// CacheTTLs holds per-status TTL durations.
type CacheTTLs struct {
	Available  time.Duration // TTL for available domains
	Registered time.Duration // TTL for registered domains
	Error      time.Duration // TTL for error results
}

// DefaultTTLs returns the standard TTL values.
func DefaultTTLs() CacheTTLs {
	return CacheTTLs{
		Available:  5 * time.Minute,
		Registered: 1 * time.Hour,
		Error:      30 * time.Second,
	}
}

// ResultCache is a bounded LRU cache for domain check results.
// It is safe for concurrent use.
type ResultCache struct {
	mu      sync.RWMutex
	items   map[string]*list.Element
	order   *list.List
	ttls    CacheTTLs
	maxSize int
}

type cacheEntry struct {
	key    string
	result domain.DomainResult
	expiry time.Time
}

// NewResultCache creates a new ResultCache with the given TTLs and max entries.
// If maxEntries is <= 0, a default of 10000 is used.
func NewResultCache(ttls CacheTTLs, maxEntries int) *ResultCache {
	if maxEntries <= 0 {
		maxEntries = 10000
	}
	return &ResultCache{
		items:   make(map[string]*list.Element),
		order:   list.New(),
		ttls:    ttls,
		maxSize: maxEntries,
	}
}

// Get retrieves a cached result for the given normalized domain.
// Returns nil if not found or expired.
func (c *ResultCache) Get(key string) *domain.DomainResult {
	c.mu.RLock()
	el, ok := c.items[key]
	c.mu.RUnlock()
	if !ok {
		return nil
	}

	entry := el.Value.(*cacheEntry)
	if time.Now().After(entry.expiry) {
		// Expired — remove lazily on next write lock.
		c.mu.Lock()
		if el2, ok := c.items[key]; ok && el == el2 {
			c.removeElement(el)
		}
		c.mu.Unlock()
		return nil
	}

	// Move to front (most recently used).
	c.mu.Lock()
	c.order.MoveToFront(el)
	c.mu.Unlock()

	result := entry.result
	result.Cached = true
	return &result
}

// Set stores a result in the cache.
func (c *ResultCache) Set(key string, result domain.DomainResult) {
	ttl := c.ttlFor(result)
	c.mu.Lock()
	defer c.mu.Unlock()

	if el, ok := c.items[key]; ok {
		// Update existing entry.
		c.order.MoveToFront(el)
		entry := el.Value.(*cacheEntry)
		entry.result = result
		entry.expiry = time.Now().Add(ttl)
		return
	}

	// Evict if at capacity.
	if c.order.Len() >= c.maxSize {
		c.evictOldest()
	}

	entry := &cacheEntry{
		key:    key,
		result: result,
		expiry: time.Now().Add(ttl),
	}
	el := c.order.PushFront(entry)
	c.items[key] = el
}

// Delete removes an entry from the cache.
func (c *ResultCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[key]; ok {
		c.removeElement(el)
	}
}

// Len returns the number of entries in the cache (including potentially expired ones).
func (c *ResultCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.order.Len()
}

// Clear removes all entries from the cache.
func (c *ResultCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]*list.Element)
	c.order.Init()
}

// PurgeExpired removes all expired entries from the cache.
// Returns the number of entries purged.
func (c *ResultCache) PurgeExpired() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	var purged int
	for el := c.order.Back(); el != nil; {
		prev := el.Prev()
		entry := el.Value.(*cacheEntry)
		if now.After(entry.expiry) {
			c.removeElement(el)
			purged++
		} else {
			// Remaining entries are newer (front = most recent), stop.
			break
		}
		el = prev
	}
	return purged
}

func (c *ResultCache) ttlFor(result domain.DomainResult) time.Duration {
	if result.Error != "" {
		return c.ttls.Error
	}
	if result.Available {
		return c.ttls.Available
	}
	return c.ttls.Registered
}

func (c *ResultCache) evictOldest() {
	el := c.order.Back()
	if el != nil {
		c.removeElement(el)
	}
}

func (c *ResultCache) removeElement(el *list.Element) {
	entry := el.Value.(*cacheEntry)
	delete(c.items, entry.key)
	c.order.Remove(el)
}
