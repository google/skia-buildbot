package rtcache

import "math"

import "go.skia.org/infra/go/util"

// ReadThroughCache defines a caching work queue with priorities. If the item
// identified by 'id' is not in the cache then it will call a
// worker function to calculate it. 'priority' is an positive integer.
// Lower values have higher priorities, so 0 is the highest priority.
type ReadThroughCache interface {
	// Get returns the item identified by 'id' or an error if the item
	// cannot be retrieved. If the item is not in the cache a worker function
	// is called to retrieve it. If returnBytes is true any stored values
	// are not de-serialized before returning. In that case the return value
	// will be of type []byte. How the worker function is set is up to the
	// implementation.
	Get(priority int64, returnBytes bool, id string) (interface{}, error)
}

// WorkerFn defines the function that is called when an item is not in the
// cache. 'priority' and 'id' are the same that were passed to Get(...).
type ReadThroughFunc func(priority int64, id string) (interface{}, error)

// PriorityTimeCombined combines a priority with a timestamp where the
// priority becomes the most significant digit and the time in Milliseconds
// the less significant digits. i.e. it allows to process elements of the
// same priority in order of their timestamps, but ahead of items with
// less priority.
func PriorityTimeCombined(priority int) int64 {
	ts := util.TimeStampMs()
	exp := int(math.Ceil(math.Log10(float64(ts)))) + 1
	return ts + int64(float64(priority)*math.Pow10(exp))
}
