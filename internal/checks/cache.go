package checks

import (
	"sync"
	"time"
)

// cacheEntry holds cached disk space information with TTL
type cacheEntry struct {
	check     *DiskSpaceCheck
	timestamp time.Time
}

// DiskSpaceCache provides thread-safe caching of disk space checks with TTL
type DiskSpaceCache struct {
	cache    map[string]*cacheEntry
	cacheTTL time.Duration
	mu       sync.RWMutex
}

// NewDiskSpaceCache creates a new disk space cache with specified TTL
func NewDiskSpaceCache(ttl time.Duration) *DiskSpaceCache {
	if ttl <= 0 {
		ttl = 30 * time.Second // Default 30 second cache
	}
	
	return &DiskSpaceCache{
		cache:    make(map[string]*cacheEntry),
		cacheTTL: ttl,
	}
}

// Get retrieves cached disk space check or performs new check if cache miss/expired
func (c *DiskSpaceCache) Get(path string) *DiskSpaceCheck {
	c.mu.RLock()
	if entry, exists := c.cache[path]; exists {
		if time.Since(entry.timestamp) < c.cacheTTL {
			c.mu.RUnlock()
			return entry.check
		}
	}
	c.mu.RUnlock()
	
	// Cache miss or expired - perform new check
	check := CheckDiskSpace(path)
	
	c.mu.Lock()
	c.cache[path] = &cacheEntry{
		check:     check,
		timestamp: time.Now(),
	}
	c.mu.Unlock()
	
	return check
}

// Clear removes all cached entries
func (c *DiskSpaceCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = make(map[string]*cacheEntry)
}

// Cleanup removes expired entries (call periodically)
func (c *DiskSpaceCache) Cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	now := time.Now()
	for path, entry := range c.cache {
		if now.Sub(entry.timestamp) >= c.cacheTTL {
			delete(c.cache, path)
		}
	}
}

// Global cache instance with 30-second TTL
var globalDiskCache = NewDiskSpaceCache(30 * time.Second)

// CheckDiskSpaceCached performs cached disk space check
func CheckDiskSpaceCached(path string) *DiskSpaceCheck {
	return globalDiskCache.Get(path)
}