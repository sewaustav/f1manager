package service

import (
	"context"
	"sync"
)

// MemoryUpdateCache — потокобезопасная in-memory реализация UpdateCache.
type MemoryUpdateCache struct {
	mu      sync.RWMutex
	updates map[string]Update
}

func NewMemoryUpdateCache() *MemoryUpdateCache {
	return &MemoryUpdateCache{updates: make(map[string]Update)}
}

var _ UpdateCache = (*MemoryUpdateCache)(nil)

func (c *MemoryUpdateCache) PutUpdate(_ context.Context, update Update) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.updates[update.Key] = update
	return nil
}

func (c *MemoryUpdateCache) GetUpdates(_ context.Context, groupID int64) ([]Update, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var result []Update
	for _, u := range c.updates {
		if u.GroupID == groupID {
			result = append(result, u)
		}
	}
	return result, nil
}

func (c *MemoryUpdateCache) DeleteUpdate(_ context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.updates, key)
	return nil
}
