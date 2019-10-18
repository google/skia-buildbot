package rtcache

import (
	"context"
)

// ReadThroughCache defines a caching work queue with priorities. If the item identified by 'id'
// is not in the cache then it will call a provided ReadThroughFunc in order to calculate it.
type ReadThroughCache interface {
	// Get returns the item identified by 'id' or an error if the item
	// cannot be retrieved. If the item is not in the cache a worker function
	// is called to retrieve it.
	Get(ctx context.Context, id string) (interface{}, error)

	// GetAll returns all items identified by 'ids' or an error if any of them
	// cannot be retrieved. If any items are not in the cache, a worker function
	// is called per item to retrieve them.
	GetAll(ctx context.Context, ids []string) ([]interface{}, error)

	// Len returns the number of items that are cached.
	Len() int

	// Keys returns the keys of the items that are cached.
	Keys() []string

	// Contains returns true if the identified item is currently cached.
	Contains(id string) bool

	// Remove removes the element with the given ids from the cache.
	Remove(ids []string)
}

// ReadThroughFunc defines the function that is called when an item is not in the
// cache. 'ctx' and 'ids' are the same that were passed to Get(...). error might be wrapped.
type ReadThroughFunc func(ctx context.Context, ids []string) ([]interface{}, error)
