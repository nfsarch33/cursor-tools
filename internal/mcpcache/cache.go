// Package mcpcache provides a session-level, TTL-based LRU cache for MCP tool
// results. Identical tool invocations within a session return cached results,
// saving tokens and latency.
package mcpcache

import (
	"container/list"
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

// Entry holds a cached MCP tool result.
type Entry struct {
	Result     []byte
	TokenCount int
}

// Stats reports cache hit/miss counters and estimated token savings.
type Stats struct {
	Hits        uint64
	Misses      uint64
	TokensSaved uint64
}

// Config controls cache behavior.
type Config struct {
	DefaultTTL time.Duration
	MaxEntries int
	ToolTTLs   map[string]time.Duration // per-tool TTL overrides
}

type cacheItem struct {
	key        string
	entry      Entry
	toolName   string
	expiresAt  time.Time
	lruElement *list.Element
}

// Cache is a concurrency-safe, TTL-based LRU cache for MCP tool results.
type Cache struct {
	mu      sync.Mutex
	cfg     Config
	items   map[string]*cacheItem
	lruList *list.List // front = most recent
	stats   Stats
}

// New creates a Cache with the given configuration.
func New(cfg Config) *Cache {
	if cfg.MaxEntries <= 0 {
		cfg.MaxEntries = 100
	}
	if cfg.DefaultTTL <= 0 {
		cfg.DefaultTTL = 5 * time.Minute
	}
	return &Cache{
		cfg:     cfg,
		items:   make(map[string]*cacheItem, cfg.MaxEntries),
		lruList: list.New(),
	}
}

// Put stores a result for the given tool name and args JSON.
func (c *Cache) Put(toolName, argsJSON string, e Entry) {
	key := cacheKey(toolName, argsJSON)
	ttl := c.ttlFor(toolName)

	c.mu.Lock()
	defer c.mu.Unlock()

	if existing, ok := c.items[key]; ok {
		existing.entry = e
		existing.expiresAt = time.Now().Add(ttl)
		c.lruList.MoveToFront(existing.lruElement)
		return
	}

	c.evictExpiredLocked()

	for c.lruList.Len() >= c.cfg.MaxEntries {
		c.evictLRULocked()
	}

	el := c.lruList.PushFront(key)
	c.items[key] = &cacheItem{
		key:        key,
		entry:      e,
		toolName:   toolName,
		expiresAt:  time.Now().Add(ttl),
		lruElement: el,
	}
}

// Get retrieves a cached result. Returns the entry and true on a cache hit,
// or a zero Entry and false on a miss (including expired entries).
func (c *Cache) Get(toolName, argsJSON string) (Entry, bool) {
	key := cacheKey(toolName, argsJSON)

	c.mu.Lock()
	defer c.mu.Unlock()

	item, ok := c.items[key]
	if !ok {
		c.stats.Misses++
		return Entry{}, false
	}

	if time.Now().After(item.expiresAt) {
		c.removeLocked(item)
		c.stats.Misses++
		return Entry{}, false
	}

	c.lruList.MoveToFront(item.lruElement)
	c.stats.Hits++
	c.stats.TokensSaved += uint64(item.entry.TokenCount)
	return item.entry, true
}

// Flush removes all cached entries (session boundary).
func (c *Cache) Flush() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*cacheItem, c.cfg.MaxEntries)
	c.lruList.Init()
}

// Len returns the number of cached entries.
func (c *Cache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.items)
}

// Stats returns a snapshot of hit/miss counters.
func (c *Cache) Stats() Stats {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.stats
}

func (c *Cache) ttlFor(toolName string) time.Duration {
	if ttl, ok := c.cfg.ToolTTLs[toolName]; ok {
		return ttl
	}
	return c.cfg.DefaultTTL
}

func (c *Cache) evictExpiredLocked() {
	now := time.Now()
	for _, item := range c.items {
		if now.After(item.expiresAt) {
			c.removeLocked(item)
		}
	}
}

func (c *Cache) evictLRULocked() {
	back := c.lruList.Back()
	if back == nil {
		return
	}
	key := back.Value.(string)
	if item, ok := c.items[key]; ok {
		c.removeLocked(item)
	}
}

func (c *Cache) removeLocked(item *cacheItem) {
	c.lruList.Remove(item.lruElement)
	delete(c.items, item.key)
}

// cacheKey produces a deterministic key from tool name and args JSON.
// Key = SHA256(toolName + "\x00" + argsJSON).
func cacheKey(toolName, argsJSON string) string {
	h := sha256.New()
	h.Write([]byte(toolName))
	h.Write([]byte{0})
	h.Write([]byte(argsJSON))
	return hex.EncodeToString(h.Sum(nil))
}
