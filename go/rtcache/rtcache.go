package rtcache

import (
	"context"
)

// ReadThroughCache defines a caching work queue with priorities. If the item
// identified by 'id' is not in the cache then it will call a
// worker function to calculate it.
type ReadThroughCache interface {
	// Get returns the item identified by 'id' or an error if the item
	// cannot be retrieved. If the item is not in the cache a worker function
	// is called to retrieve it.
	Get(ctx context.Context, id string) (interface{}, error)

	// Keys returns the keys of the cache. TODO(kjlubick): maybe call this cached keys?
	Keys() []string

	// Remove removes the element with the given ids from the cache.
	Remove(ids []string)
}

// WorkerFn defines the function that is called when an item is not in the
// cache. 'ctx' and 'id' are the same that were passed to Get(...).
type ReadThroughFunc func(ctx context.Context, id string) (interface{}, error)
