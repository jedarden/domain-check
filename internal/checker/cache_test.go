package checker

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/coding/domain-check/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeResult(name string, available bool) domain.DomainResult {
	return domain.DomainResult{
		Domain:    name,
		Available: available,
		TLD:       "com",
		CheckedAt: time.Now(),
		Source:    domain.SourceRDAP,
	}
}

func makeErrorResult(name string) domain.DomainResult {
	return domain.DomainResult{
		Domain:    name,
		Available: false,
		TLD:       "com",
		CheckedAt: time.Now(),
		Source:    domain.SourceRDAP,
		Error:     "connection refused",
	}
}

func TestNewResultCache_Defaults(t *testing.T) {
	ttls := CacheTTLs{Available: time.Second, Registered: time.Hour, Error: 100 * time.Millisecond}
	c := NewResultCache(ttls, 0) // 0 should become 10000
	assert.Equal(t, 10000, c.maxSize)
	assert.Equal(t, 0, c.Len())
}

func TestSetAndGet(t *testing.T) {
	ttls := DefaultTTLs()
	c := NewResultCache(ttls, 100)

	c.Set("example.com", makeResult("example.com", true))
	got := c.Get("example.com")
	require.NotNil(t, got)
	assert.Equal(t, "example.com", got.Domain)
	assert.True(t, got.Available)
	assert.True(t, got.Cached) // Get marks results as cached
}

func TestGet_Miss(t *testing.T) {
	c := NewResultCache(DefaultTTLs(), 100)
	assert.Nil(t, c.Get("nonexistent.com"))
}

func TestSet_Overwrite(t *testing.T) {
	c := NewResultCache(DefaultTTLs(), 100)

	c.Set("example.com", makeResult("example.com", true))
	c.Set("example.com", makeResult("example.com", false))

	got := c.Get("example.com")
	require.NotNil(t, got)
	assert.False(t, got.Available)
	assert.Equal(t, 1, c.Len())
}

func TestExpiry_Available(t *testing.T) {
	ttls := CacheTTLs{Available: 50 * time.Millisecond, Registered: time.Hour, Error: time.Hour}
	c := NewResultCache(ttls, 100)

	c.Set("example.com", makeResult("example.com", true))
	got := c.Get("example.com")
	require.NotNil(t, got)

	time.Sleep(60 * time.Millisecond)
	got = c.Get("example.com")
	assert.Nil(t, got)
}

func TestExpiry_Registered(t *testing.T) {
	ttls := CacheTTLs{Available: time.Hour, Registered: 50 * time.Millisecond, Error: time.Hour}
	c := NewResultCache(ttls, 100)

	c.Set("example.com", makeResult("example.com", false))
	got := c.Get("example.com")
	require.NotNil(t, got)

	time.Sleep(60 * time.Millisecond)
	got = c.Get("example.com")
	assert.Nil(t, got)
}

func TestExpiry_Error(t *testing.T) {
	ttls := CacheTTLs{Available: time.Hour, Registered: time.Hour, Error: 50 * time.Millisecond}
	c := NewResultCache(ttls, 100)

	c.Set("example.com", makeErrorResult("example.com"))
	got := c.Get("example.com")
	require.NotNil(t, got)

	time.Sleep(60 * time.Millisecond)
	got = c.Get("example.com")
	assert.Nil(t, got)
}

func TestLRUEviction(t *testing.T) {
	c := NewResultCache(CacheTTLs{Available: time.Hour, Registered: time.Hour, Error: time.Hour}, 3)

	c.Set("a.com", makeResult("a.com", true))
	c.Set("b.com", makeResult("b.com", true))
	c.Set("c.com", makeResult("c.com", true))
	assert.Equal(t, 3, c.Len())

	// Adding a 4th should evict the oldest (a.com).
	c.Set("d.com", makeResult("d.com", true))
	assert.Equal(t, 3, c.Len())

	assert.Nil(t, c.Get("a.com")) // evicted
	assert.NotNil(t, c.Get("b.com"))
	assert.NotNil(t, c.Get("c.com"))
	assert.NotNil(t, c.Get("d.com"))
}

func TestLRU_AccessPromotes(t *testing.T) {
	c := NewResultCache(CacheTTLs{Available: time.Hour, Registered: time.Hour, Error: time.Hour}, 3)

	c.Set("a.com", makeResult("a.com", true))
	c.Set("b.com", makeResult("b.com", true))
	c.Set("c.com", makeResult("c.com", true))

	// Access a.com to promote it.
	c.Get("a.com")

	// Adding d.com should evict b.com (oldest unused), not a.com.
	c.Set("d.com", makeResult("d.com", true))

	assert.NotNil(t, c.Get("a.com")) // promoted, not evicted
	assert.Nil(t, c.Get("b.com"))    // evicted
	assert.NotNil(t, c.Get("c.com"))
	assert.NotNil(t, c.Get("d.com"))
}

func TestDelete(t *testing.T) {
	c := NewResultCache(DefaultTTLs(), 100)

	c.Set("example.com", makeResult("example.com", true))
	assert.Equal(t, 1, c.Len())

	c.Delete("example.com")
	assert.Equal(t, 0, c.Len())
	assert.Nil(t, c.Get("example.com"))
}

func TestDelete_Nonexistent(t *testing.T) {
	c := NewResultCache(DefaultTTLs(), 100)
	c.Delete("nonexistent.com") // should not panic
	assert.Equal(t, 0, c.Len())
}

func TestClear(t *testing.T) {
	c := NewResultCache(DefaultTTLs(), 100)
	c.Set("a.com", makeResult("a.com", true))
	c.Set("b.com", makeResult("b.com", true))

	c.Clear()
	assert.Equal(t, 0, c.Len())
	assert.Nil(t, c.Get("a.com"))
}

func TestPurgeExpired(t *testing.T) {
	ttls := CacheTTLs{Available: 30 * time.Millisecond, Registered: time.Hour, Error: time.Hour}
	c := NewResultCache(ttls, 100)

	c.Set("a.com", makeResult("a.com", true))  // will expire
	c.Set("b.com", makeResult("b.com", false)) // registered, stays

	time.Sleep(40 * time.Millisecond)

	purged := c.PurgeExpired()
	assert.Equal(t, 1, purged)
	assert.Equal(t, 1, c.Len())
	assert.Nil(t, c.Get("a.com"))
	assert.NotNil(t, c.Get("b.com"))
}

func TestPurgeExpired_NoneExpired(t *testing.T) {
	c := NewResultCache(DefaultTTLs(), 100)
	c.Set("a.com", makeResult("a.com", true))
	assert.Equal(t, 0, c.PurgeExpired())
}

func TestCachedFlag_OnlySetOnGet(t *testing.T) {
	c := NewResultCache(DefaultTTLs(), 100)

	original := makeResult("example.com", true)
	original.Cached = false

	c.Set("example.com", original)
	got := c.Get("example.com")
	require.NotNil(t, got)
	assert.True(t, got.Cached) // Get sets this
}

func TestConcurrentReads(t *testing.T) {
	c := NewResultCache(DefaultTTLs(), 100)

	// Pre-populate cache.
	for i := 0; i < 50; i++ {
		c.Set(string(rune('a'+i))+".com", makeResult(string(rune('a'+i))+".com", true))
	}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			key := string(rune('a'+(idx%50))) + ".com"
			got := c.Get(key)
			if got == nil {
				t.Errorf("unexpected nil for %s", key)
				return
			}
			if got.Domain != key {
				t.Errorf("wrong domain: got %s, want %s", got.Domain, key)
			}
		}(i)
	}
	wg.Wait()
}

func TestConcurrentWrites(t *testing.T) {
	c := NewResultCache(DefaultTTLs(), 100)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			key := fmt.Sprintf("domain%d.com", idx%20) // Contention on 20 keys
			c.Set(key, makeResult(key, idx%2 == 0))
		}(i)
	}
	wg.Wait()

	// All 20 keys should exist.
	assert.Equal(t, 20, c.Len())
}

func TestConcurrentReadWrite(t *testing.T) {
	c := NewResultCache(DefaultTTLs(), 100)

	// Pre-populate.
	for i := 0; i < 30; i++ {
		c.Set(fmt.Sprintf("domain%d.com", i), makeResult(fmt.Sprintf("domain%d.com", i), true))
	}

	var wg sync.WaitGroup

	// Writers.
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			key := fmt.Sprintf("domain%d.com", idx%40)
			c.Set(key, makeResult(key, true))
		}(i)
	}

	// Readers.
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			key := fmt.Sprintf("domain%d.com", idx%40)
			_ = c.Get(key) // May be nil, that's OK
		}(i)
	}

	// Deleter.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			c.Delete(fmt.Sprintf("domain%d.com", i))
		}
	}()

	wg.Wait()
}

func TestConcurrentWithEviction(t *testing.T) {
	// Small cache to trigger evictions under concurrency.
	c := NewResultCache(CacheTTLs{Available: time.Hour, Registered: time.Hour, Error: time.Hour}, 10)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			key := fmt.Sprintf("domain%d.com", idx)
			c.Set(key, makeResult(key, true))
		}(i)
	}
	wg.Wait()

	// Cache should be at max capacity.
	assert.LessOrEqual(t, c.Len(), 10)
}

func TestConcurrentPurgeExpired(t *testing.T) {
	ttls := CacheTTLs{Available: 30 * time.Millisecond, Registered: time.Hour, Error: time.Hour}
	c := NewResultCache(ttls, 100)

	// Pre-populate with mix of short and long TTL.
	for i := 0; i < 50; i++ {
		if i%2 == 0 {
			c.Set(fmt.Sprintf("short%d.com", i), makeResult(fmt.Sprintf("short%d.com", i), true)) // available, short TTL
		} else {
			c.Set(fmt.Sprintf("long%d.com", i), makeResult(fmt.Sprintf("long%d.com", i), false)) // registered, long TTL
		}
	}

	time.Sleep(40 * time.Millisecond)

	var wg sync.WaitGroup

	// Concurrent purge calls.
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.PurgeExpired()
		}()
	}

	// Concurrent reads during purge.
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_ = c.Get(fmt.Sprintf("long%d.com", idx%25*2+1))
		}(i)
	}

	wg.Wait()
}

func TestStats_InitialState(t *testing.T) {
	c := NewResultCache(DefaultTTLs(), 100)
	stats := c.Stats()
	assert.Equal(t, int64(0), stats.Hits)
	assert.Equal(t, int64(0), stats.Misses)
	assert.Equal(t, 0, stats.Size)
	assert.Equal(t, 100, stats.Max)
}

func TestStats_Hits(t *testing.T) {
	c := NewResultCache(DefaultTTLs(), 100)
	c.Set("example.com", makeResult("example.com", true))

	// Multiple hits
	for i := 0; i < 5; i++ {
		c.Get("example.com")
	}

	stats := c.Stats()
	assert.Equal(t, int64(5), stats.Hits)
	assert.Equal(t, int64(0), stats.Misses)
	assert.Equal(t, 1, stats.Size)
}

func TestStats_Misses(t *testing.T) {
	c := NewResultCache(DefaultTTLs(), 100)

	// Multiple misses
	for i := 0; i < 3; i++ {
		c.Get("nonexistent.com")
	}

	stats := c.Stats()
	assert.Equal(t, int64(0), stats.Hits)
	assert.Equal(t, int64(3), stats.Misses)
}

func TestStats_HitMissMix(t *testing.T) {
	c := NewResultCache(DefaultTTLs(), 100)
	c.Set("example.com", makeResult("example.com", true))
	c.Set("test.com", makeResult("test.com", false))

	// Mix of hits and misses
	c.Get("example.com") // hit
	c.Get("nonexistent.com") // miss
	c.Get("test.com") // hit
	c.Get("another.com") // miss
	c.Get("example.com") // hit

	stats := c.Stats()
	assert.Equal(t, int64(3), stats.Hits)
	assert.Equal(t, int64(2), stats.Misses)
	assert.Equal(t, 2, stats.Size)
}

func TestStats_ExpiredEntryCountsAsMiss(t *testing.T) {
	ttls := CacheTTLs{Available: 30 * time.Millisecond, Registered: time.Hour, Error: time.Hour}
	c := NewResultCache(ttls, 100)

	c.Set("example.com", makeResult("example.com", true))
	time.Sleep(40 * time.Millisecond)

	// Getting expired entry should count as miss
	got := c.Get("example.com")
	assert.Nil(t, got)

	stats := c.Stats()
	assert.Equal(t, int64(0), stats.Hits)
	assert.Equal(t, int64(1), stats.Misses)
}

func TestStats_ConcurrentAccess(t *testing.T) {
	c := NewResultCache(DefaultTTLs(), 100)
	c.Set("example.com", makeResult("example.com", true))

	var wg sync.WaitGroup

	// Concurrent hits
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Get("example.com")
		}()
	}

	// Concurrent misses
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			c.Get(fmt.Sprintf("nonexistent%d.com", idx))
		}(i)
	}

	wg.Wait()

	stats := c.Stats()
	assert.Equal(t, int64(50), stats.Hits)
	assert.Equal(t, int64(30), stats.Misses)
}

func TestStats_AfterEviction(t *testing.T) {
	c := NewResultCache(CacheTTLs{Available: time.Hour, Registered: time.Hour, Error: time.Hour}, 3)

	c.Set("a.com", makeResult("a.com", true))
	c.Set("b.com", makeResult("b.com", true))
	c.Set("c.com", makeResult("c.com", true))

	// Get each once to register hits
	c.Get("a.com")
	c.Get("b.com")
	c.Get("c.com")

	stats := c.Stats()
	assert.Equal(t, int64(3), stats.Hits)
	assert.Equal(t, 3, stats.Size)

	// Add new entry to trigger eviction
	c.Set("d.com", makeResult("d.com", true))

	stats = c.Stats()
	assert.Equal(t, 3, stats.Size) // Still at max
	assert.Equal(t, int64(3), stats.Hits) // Hits unchanged
}

func TestStats_AfterClear(t *testing.T) {
	c := NewResultCache(DefaultTTLs(), 100)
	c.Set("a.com", makeResult("a.com", true))
	c.Set("b.com", makeResult("b.com", true))

	c.Get("a.com")
	c.Get("b.com")

	stats := c.Stats()
	assert.Equal(t, int64(2), stats.Hits)
	assert.Equal(t, 2, stats.Size)

	c.Clear()

	stats = c.Stats()
	assert.Equal(t, int64(2), stats.Hits) // Hits counter preserved
	assert.Equal(t, 0, stats.Size) // But size is 0
}
