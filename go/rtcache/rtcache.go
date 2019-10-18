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

	// Keys returns the keys of the items that are cached.
	Keys() []string

	// Contains returns true if the identified item is currently cached.
	Contains(id string) bool

	// Remove removes the element with the given ids from the cache.
	Remove(ids []string)
}

// ReadThroughFunc defines the function that is called when an item is not in the
// cache. 'ctx' and 'id' are the same that were passed to Get(...). error might be wrapped.
type ReadThroughFunc func(ctx context.Context, id string) (interface{}, error)
