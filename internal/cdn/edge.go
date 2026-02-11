package cdn

import (
	"container/list"
	"log"
	"sync"
	"time"
)

// CacheEntry represents a single cached object at an edge node.
type CacheEntry struct {
	Key         string
	Data        []byte
	ContentType string
	ETag        string
	Size        int64
	HitCount    int64
	CreatedAt   time.Time
	ExpiresAt   time.Time
	LastAccess  time.Time
}

// EdgeCache implements an LRU-eviction cache for a single edge node.
// It stores content closer to users to reduce latency and origin load.
type EdgeCache struct {
	mu sync.RWMutex

	// LRU tracking
	capacity   int64 // max bytes
	currentSize int64
	items      map[string]*list.Element
	evictList  *list.List

	// Stats
	hits       int64
	misses     int64
	evictions  int64

	// Origin fetch callback
	fetchOrigin func(key string) ([]byte, string, error)
}

// EdgeCacheConfig controls edge cache behavior.
type EdgeCacheConfig struct {
	CapacityMB    int64
	DefaultTTL    time.Duration
	MaxObjectSize int64 // max size of a single cached object
}

// NewEdgeCache creates a new LRU edge cache with the given capacity.
func NewEdgeCache(cfg EdgeCacheConfig, fetcher func(key string) ([]byte, string, error)) *EdgeCache {
	capacityBytes := cfg.CapacityMB * 1024 * 1024
	if capacityBytes <= 0 {
		capacityBytes = 512 * 1024 * 1024 // 512 MB default
	}

	return &EdgeCache{
		capacity:    capacityBytes,
		items:       make(map[string]*list.Element),
		evictList:   list.New(),
		fetchOrigin: fetcher,
	}
}

// Get retrieves content from the cache, fetching from origin on miss.
func (c *EdgeCache) Get(key string) (*CacheEntry, error) {
	c.mu.Lock()

	// Check cache
	if elem, ok := c.items[key]; ok {
		entry := elem.Value.(*CacheEntry)
		// Check expiry
		if !entry.ExpiresAt.IsZero() && time.Now().After(entry.ExpiresAt) {
			// Expired — remove and fetch fresh
			c.removeElement(elem)
			c.mu.Unlock()
			return c.fetchAndCache(key)
		}

		// Cache hit — move to front of LRU
		c.evictList.MoveToFront(elem)
		entry.HitCount++
		entry.LastAccess = time.Now()
		c.hits++
		c.mu.Unlock()
		return entry, nil
	}

	c.misses++
	c.mu.Unlock()

	// Cache miss — fetch from origin
	return c.fetchAndCache(key)
}

// Put manually inserts content into the cache.
func (c *EdgeCache) Put(key string, data []byte, contentType string, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Remove existing entry if present
	if elem, ok := c.items[key]; ok {
		c.removeElement(elem)
	}

	entry := &CacheEntry{
		Key:         key,
		Data:        data,
		ContentType: contentType,
		Size:        int64(len(data)),
		CreatedAt:   time.Now(),
		LastAccess:  time.Now(),
	}
	if ttl > 0 {
		entry.ExpiresAt = time.Now().Add(ttl)
	}

	c.addEntry(entry)
}

// Invalidate removes a key from the cache.
func (c *EdgeCache) Invalidate(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		c.removeElement(elem)
	}
}

// Purge clears the entire cache.
func (c *EdgeCache) Purge() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*list.Element)
	c.evictList.Init()
	c.currentSize = 0
}

// Stats returns cache hit/miss statistics.
func (c *EdgeCache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	hitRate := float64(0)
	total := c.hits + c.misses
	if total > 0 {
		hitRate = float64(c.hits) / float64(total)
	}

	return CacheStats{
		Hits:        c.hits,
		Misses:      c.misses,
		Evictions:   c.evictions,
		HitRate:     hitRate,
		CurrentSize: c.currentSize,
		Capacity:    c.capacity,
		ItemCount:   int64(len(c.items)),
	}
}

// CacheStats holds edge cache performance metrics.
type CacheStats struct {
	Hits        int64
	Misses      int64
	Evictions   int64
	HitRate     float64
	CurrentSize int64
	Capacity    int64
	ItemCount   int64
}

func (c *EdgeCache) fetchAndCache(key string) (*CacheEntry, error) {
	if c.fetchOrigin == nil {
		return nil, nil
	}

	data, contentType, err := c.fetchOrigin(key)
	if err != nil {
		return nil, err
	}

	entry := &CacheEntry{
		Key:         key,
		Data:        data,
		ContentType: contentType,
		Size:        int64(len(data)),
		HitCount:    1,
		CreatedAt:   time.Now(),
		LastAccess:  time.Now(),
		ExpiresAt:   time.Now().Add(5 * time.Minute), // default TTL
	}

	c.mu.Lock()
	c.addEntry(entry)
	c.mu.Unlock()

	return entry, nil
}

// addEntry inserts an entry, evicting LRU items if needed. Must be called with lock held.
func (c *EdgeCache) addEntry(entry *CacheEntry) {
	// Evict until we have room
	for c.currentSize+entry.Size > c.capacity && c.evictList.Len() > 0 {
		c.evictOldest()
	}

	elem := c.evictList.PushFront(entry)
	c.items[entry.Key] = elem
	c.currentSize += entry.Size
}

// removeElement removes an element from the cache. Must be called with lock held.
func (c *EdgeCache) removeElement(elem *list.Element) {
	entry := elem.Value.(*CacheEntry)
	c.evictList.Remove(elem)
	delete(c.items, entry.Key)
	c.currentSize -= entry.Size
}

// evictOldest removes the least recently used item. Must be called with lock held.
func (c *EdgeCache) evictOldest() {
	oldest := c.evictList.Back()
	if oldest == nil {
		return
	}
	entry := oldest.Value.(*CacheEntry)
	c.removeElement(oldest)
	c.evictions++
	log.Printf("[cdn] evicted %s (size=%d, hits=%d)", entry.Key, entry.Size, entry.HitCount)
}
