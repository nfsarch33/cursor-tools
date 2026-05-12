package mcpcache

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestCache_PutAndGet_Hit(t *testing.T) {
	c := New(Config{DefaultTTL: 5 * time.Minute, MaxEntries: 100})

	c.Put("mem0_search", `{"query":"test"}`, Entry{
		Result:     []byte(`{"memories":[]}`),
		TokenCount: 42,
	})

	got, ok := c.Get("mem0_search", `{"query":"test"}`)
	require.True(t, ok)
	assert.Equal(t, []byte(`{"memories":[]}`), got.Result)
	assert.Equal(t, 42, got.TokenCount)
}

func TestCache_Get_Miss(t *testing.T) {
	c := New(Config{DefaultTTL: 5 * time.Minute, MaxEntries: 100})

	_, ok := c.Get("mem0_search", `{"query":"nonexistent"}`)
	assert.False(t, ok)
}

func TestCache_TTLExpiry(t *testing.T) {
	c := New(Config{DefaultTTL: 50 * time.Millisecond, MaxEntries: 100})

	c.Put("tool", `{"a":1}`, Entry{Result: []byte("ok"), TokenCount: 10})

	_, ok := c.Get("tool", `{"a":1}`)
	require.True(t, ok)

	time.Sleep(80 * time.Millisecond)

	_, ok = c.Get("tool", `{"a":1}`)
	assert.False(t, ok, "entry should have expired")
}

func TestCache_PerToolTTL(t *testing.T) {
	c := New(Config{
		DefaultTTL: 5 * time.Minute,
		MaxEntries: 100,
		ToolTTLs: map[string]time.Duration{
			"fast-expire": 50 * time.Millisecond,
		},
	})

	c.Put("fast-expire", `{}`, Entry{Result: []byte("fast"), TokenCount: 5})
	c.Put("normal", `{}`, Entry{Result: []byte("normal"), TokenCount: 5})

	time.Sleep(80 * time.Millisecond)

	_, ok := c.Get("fast-expire", `{}`)
	assert.False(t, ok, "fast-expire tool should have expired")

	_, ok = c.Get("normal", `{}`)
	assert.True(t, ok, "normal tool should still be cached")
}

func TestCache_LRUEviction(t *testing.T) {
	c := New(Config{DefaultTTL: 5 * time.Minute, MaxEntries: 3})

	c.Put("t", `{"k":1}`, Entry{Result: []byte("1"), TokenCount: 1})
	c.Put("t", `{"k":2}`, Entry{Result: []byte("2"), TokenCount: 1})
	c.Put("t", `{"k":3}`, Entry{Result: []byte("3"), TokenCount: 1})

	// Access k:1 to make it recently used
	_, ok := c.Get("t", `{"k":1}`)
	require.True(t, ok)

	// Adding a 4th entry should evict k:2 (least recently used)
	c.Put("t", `{"k":4}`, Entry{Result: []byte("4"), TokenCount: 1})

	_, ok = c.Get("t", `{"k":2}`)
	assert.False(t, ok, "k:2 should have been evicted (LRU)")

	_, ok = c.Get("t", `{"k":1}`)
	assert.True(t, ok, "k:1 should still be cached (recently accessed)")

	_, ok = c.Get("t", `{"k":3}`)
	assert.True(t, ok, "k:3 should still be cached")

	_, ok = c.Get("t", `{"k":4}`)
	assert.True(t, ok, "k:4 should still be cached")
}

func TestCache_Flush(t *testing.T) {
	c := New(Config{DefaultTTL: 5 * time.Minute, MaxEntries: 100})

	c.Put("a", `{}`, Entry{Result: []byte("1"), TokenCount: 1})
	c.Put("b", `{}`, Entry{Result: []byte("2"), TokenCount: 2})

	c.Flush()

	_, ok := c.Get("a", `{}`)
	assert.False(t, ok)
	_, ok = c.Get("b", `{}`)
	assert.False(t, ok)

	assert.Equal(t, 0, c.Len())
}

func TestCache_Stats(t *testing.T) {
	c := New(Config{DefaultTTL: 5 * time.Minute, MaxEntries: 100})

	c.Put("t", `{"x":1}`, Entry{Result: []byte("hit"), TokenCount: 10})

	c.Get("t", `{"x":1}`)
	c.Get("t", `{"x":1}`)
	c.Get("t", `{"x":99}`)

	s := c.Stats()
	assert.Equal(t, uint64(2), s.Hits)
	assert.Equal(t, uint64(1), s.Misses)
	assert.Equal(t, uint64(20), s.TokensSaved)
}

func TestCache_ConcurrentAccess(t *testing.T) {
	c := New(Config{DefaultTTL: 5 * time.Minute, MaxEntries: 100})
	const goroutines = 20
	const iterations = 50

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := range goroutines {
		go func(id int) {
			defer wg.Done()
			for i := range iterations {
				key := `{"i":` + string(rune('0'+i%10)) + `}`
				c.Put("tool", key, Entry{Result: []byte("ok"), TokenCount: 1})
				c.Get("tool", key)
			}
			_ = c.Stats()
			_ = c.Len()
		}(g)
	}
	wg.Wait()
}

func TestCache_KeyDeterminism(t *testing.T) {
	c := New(Config{DefaultTTL: 5 * time.Minute, MaxEntries: 100})

	c.Put("mem0_search", `{"query":"test","user_id":"u1"}`, Entry{
		Result:     []byte("r1"),
		TokenCount: 5,
	})

	_, ok := c.Get("mem0_search", `{"query":"test","user_id":"u1"}`)
	assert.True(t, ok)

	// Different args → miss
	_, ok = c.Get("mem0_search", `{"query":"test","user_id":"u2"}`)
	assert.False(t, ok)

	// Different tool → miss
	_, ok = c.Get("mem0_add", `{"query":"test","user_id":"u1"}`)
	assert.False(t, ok)
}

func TestCache_DefaultConfig(t *testing.T) {
	c := New(Config{})
	assert.NotNil(t, c)

	c.Put("t", `{}`, Entry{Result: []byte("ok"), TokenCount: 1})
	got, ok := c.Get("t", `{}`)
	require.True(t, ok)
	assert.Equal(t, []byte("ok"), got.Result)
}

func TestCache_EvictExpiredOnPut(t *testing.T) {
	c := New(Config{DefaultTTL: 50 * time.Millisecond, MaxEntries: 100})

	c.Put("t", `{"old":1}`, Entry{Result: []byte("old"), TokenCount: 1})
	c.Put("t", `{"old":2}`, Entry{Result: []byte("old2"), TokenCount: 1})

	time.Sleep(80 * time.Millisecond)

	c.Put("t", `{"new":1}`, Entry{Result: []byte("new"), TokenCount: 1})

	assert.Equal(t, 1, c.Len(), "expired entries should have been evicted")
}

func TestCache_OverwriteSameKey(t *testing.T) {
	c := New(Config{DefaultTTL: 5 * time.Minute, MaxEntries: 100})

	c.Put("t", `{}`, Entry{Result: []byte("v1"), TokenCount: 1})
	c.Put("t", `{}`, Entry{Result: []byte("v2"), TokenCount: 2})

	got, ok := c.Get("t", `{}`)
	require.True(t, ok)
	assert.Equal(t, []byte("v2"), got.Result)
	assert.Equal(t, 2, got.TokenCount)

	assert.Equal(t, 1, c.Len(), "overwrite should not increase entry count")
}
