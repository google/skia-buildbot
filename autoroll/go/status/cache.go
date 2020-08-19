package status

import (
	"context"
	"sync"

	"cloud.google.com/go/datastore"
	"go.skia.org/infra/go/sklog"
)

// Cache stores the most recent AutoRollStatus.
type Cache struct {
	DB
	mtx    sync.RWMutex
	roller string
	status *AutoRollStatus
}

// NewCache returns an Cache instance.
func NewCache(ctx context.Context, db DB, rollerName string) (*Cache, error) {
	c := &Cache{
		DB:     db,
		roller: rollerName,
	}
	if err := c.Update(ctx); err != nil {
		return nil, err
	}
	return c, nil
}

// Get returns the AutoRollStatus as of the last call to Update().
func (c *Cache) Get() *AutoRollStatus {
	c.mtx.RLock()
	defer c.mtx.RUnlock()
	return c.status.Copy()
}

// GetMini returns the AutoRollMiniStatus as of the last call to Update().
func (c *Cache) GetMini() *AutoRollMiniStatus {
	return &c.Get().AutoRollMiniStatus
}

// Update updates the cached status information.
func (c *Cache) Update(ctx context.Context) error {
	status, err := c.DB.Get(ctx, c.roller)
	if err == datastore.ErrNoSuchEntity || status == nil {
		// This will occur the first time the roller starts,
		// before it sets the status for the first time. Ignore.
		sklog.Warningf("Unable to find AutoRollStatus for %s. Is this the first startup for this roller?", c.roller)
		status = &AutoRollStatus{}
	} else if err != nil {
		return err
	}
	c.mtx.Lock()
	defer c.mtx.Unlock()
	c.status = status
	return nil
}
