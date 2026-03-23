package checker

import (
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
