package docker

import (
	"sync"
	"time"
)

// CachedStackDetails holds cached compose stack specification and environment variables.
type CachedStackDetails struct {
	ComposeYAML string
	Env         string
	ConfigFile  string
	WorkingDir  string
	UpdatedAt   time.Time
}

type cacheEntry struct {
	details   CachedStackDetails
	expiresAt time.Time
}

// StackDetailCache provides a thread-safe in-memory cache with TTL expiration.
type StackDetailCache struct {
	mu      sync.RWMutex
	ttl     time.Duration
	entries map[string]cacheEntry
}

// NewStackDetailCache creates a new in-memory cache with the specified TTL.
func NewStackDetailCache(ttl time.Duration) *StackDetailCache {
	return &StackDetailCache{
		ttl:     ttl,
		entries: make(map[string]cacheEntry),
	}
}

// Get returns the cached stack details for stackName if present and not expired.
func (c *StackDetailCache) Get(stackName string) (CachedStackDetails, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[stackName]
	if !ok || time.Now().After(entry.expiresAt) {
		return CachedStackDetails{}, false
	}
	return entry.details, true
}

// Set stores the details for stackName and marks the expiration time.
func (c *StackDetailCache) Set(stackName string, details CachedStackDetails) {
	c.mu.Lock()
	defer c.mu.Unlock()

	details.UpdatedAt = time.Now()
	c.entries[stackName] = cacheEntry{
		details:   details,
		expiresAt: time.Now().Add(c.ttl),
	}
}

// Invalidate removes a specific stack from the cache.
func (c *StackDetailCache) Invalidate(stackName string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, stackName)
}
