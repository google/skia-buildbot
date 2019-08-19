package rtcache

import (
	"math"

	"go.skia.org/infra/go/util"
)

// ReadThroughCache defines a caching work queue with priorities. If the item
// identified by 'id' is not in the cache then it will call a
// worker function to calculate it. 'priority' is an positive integer.
// Lower values have higher priorities, so 0 is the highest priority.
type ReadThroughCache interface {
	// Get returns the item identified by 'id' or an error if the item
	// cannot be retrieved. If the item is not in the cache a worker function
	// is called to retrieve it.
	Get(priority int64, id string) (interface{}, error)

	// Warm is identical to Get except it does not return the cached elements
	// just makes sure they are in the cache. If an error occurs generating the
	// item desired item via the worker function, an error is returned.
	Warm(priority int64, id string) error

	// Contains returns true if the identified item is currently cached.
	Contains(id string) bool

	// Keys returns the keys of the cache.
	Keys() []string

	// Remove removes the element with the given ids from the cache.
	Remove(ids []string)
}

// WorkerFn defines the function that is called when an item is not in the
// cache. 'priority' and 'id' are the same that were passed to Get(...).
type ReadThroughFunc func(priority int64, id string) (interface{}, error)

// PriorityTimeCombined combines a priority with a timestamp where the
// priority becomes the most significant digit and the time in Milliseconds
// the less significant digits. i.e. it allows to process elements of the
// same priority in order of their timestamps, but ahead of items with
// less priority.
func PriorityTimeCombined(priority int64) int64 {
	ts := util.TimeStampMs()
	exp := int(math.Ceil(math.Log10(float64(ts)))) + 1
	return ts + int64(float64(priority)*math.Pow10(exp))
}
