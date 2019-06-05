package dedupe

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.skia.org/infra/task_scheduler/go/candidate"
	"go.skia.org/infra/task_scheduler/go/db/cache"
	"go.skia.org/infra/task_scheduler/go/types"
)

const (
	// Cache entries expire after this much time.
	// TODO(borenet): Check what the actual isolate expiration time is and
	// adjust this accordingly.
	EXPIRATION = 4 * 7 * 24 * time.Hour
)

var (
	// ErrNotFound is returned when a given entry is not found in the cache.
	ErrNotFound = errors.New("Not found")
)

// HashCandidate returns a hash of all of the identifying bits of the candidate.
func HashCandidate(c *candidate.TaskCandidate) (string, error) {
	// Come up with a fake random ID for the task; if the ID is used
	// anywhere in the task (besides the tags or pubsub data), then we can't
	// de-duplicate it.
	id := uuid.New().String()

	// Isolate server and pubsub topic should be the same for all tasks on
	// a given scheduling instance, so they can be faked here.
	isolateServer := "fake-isolate-server"
	pubsubTopic := "fake-pubsub-topic"

	// Create the request.
	req, err := c.MakeTaskRequest(id, isolateServer, pubsubTopic)
	if err != nil {
		return "", fmt.Errorf("Failed to make task request: %s", err)
	}

	// Ignore some individually-identifying parts of the request.
	req.PubsubUserdata = ""
	req.Tags = nil

	// Hash the TaskRequest.
	sum := sha256.New()
	if err := json.NewEncoder(sum).Encode(req); err != nil {
		return "", err
	}
	return string(sum.Sum(nil)), nil
}

// HashTask returns a hash of all of the identifying bits of the task.
func HashTask(t *types.Task) string {
	return "TODO"
}

// DedupeCache stores a mapping of hashed task inputs to tasks.
type DedupeCache interface {
	// Get returns the ID of the task which ran with the given hashed
	// inputs, or ErrNotFound if no such task exists.
	Get(context.Context, string) (string, error)

	// Put inserts the task ID into the DedupeCache with the given
	// timestamp.
	Put(context.Context, string, string, time.Time) error

	// Cleanup removes old entries from the cache.
	Cleanup(context.Context) error
}

// memoryDedupeCacheEntry represents one entry in a memoryDedupeCache.
type memoryDedupeCacheEntry struct {
	id string
	ts time.Time
}

// memoryDedupeCache implements DedupeCache in memory.
type memoryDedupeCache struct {
	m   map[string]*memoryDedupeCacheEntry
	mtx sync.RWMutex
}

// NewMemoryDedupeCache returns a DedupeCache instance which uses only in-memory
// storage.
func NewMemoryDedupeCache() DedupeCache {
	return &memoryDedupeCache{
		m: map[string]*memoryDedupeCacheEntry{},
	}
}

// See documentation for DedupeCache interface.
func (c *memoryDedupeCache) Get(ctx context.Context, hash string) (string, error) {
	c.mtx.RLock()
	defer c.mtx.RUnlock()
	if entry, ok := c.m[hash]; ok {
		return entry.id, nil
	}
	return "", ErrNotFound
}

// See documentation for DedupeCache interface.
func (c *memoryDedupeCache) Put(ctx context.Context, hash, id string, ts time.Time) error {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	c.m[hash] = &memoryDedupeCacheEntry{
		id: id,
		ts: ts,
	}
	return nil
}

// See documentation for DedupeCache interface.
func (c *memoryDedupeCache) Cleanup(ctx context.Context) error {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	for hash, entry := range c.m {
		if time.Since(entry.ts) > EXPIRATION {
			delete(c.m, hash)
		}
	}
	return nil
}

// DedupeCacheWrapper is a convenience wrapper around DedupeCache which allows
// working with Tasks directly, rather than IDs and timestamps.
type DedupeCacheWrapper struct {
	dCache DedupeCache
	tCache cache.TaskCache
}

// NewDedupeCacheWrapper returns a DedupeCacheWrapper instance.
func NewDedupeCacheWrapper(dCache DedupeCache, tCache cache.TaskCache) *DedupeCacheWrapper {
	return &DedupeCacheWrapper{
		dCache: dCache,
		tCache: tCache,
	}
}

// Get returns the task which ran with the given hashed inputs, or ErrNotFound
// if no such task exists.
func (w *DedupeCacheWrapper) Get(ctx context.Context, hash string) (*types.Task, error) {
	id, err := w.dCache.Get(ctx, hash)
	if err != nil {
		return nil, err
	}
	return w.tCache.GetTaskMaybeExpired(id)
}

// Put inserts the task into the cache.
func (w *DedupeCacheWrapper) Put(ctx context.Context, hash string, task *types.Task) error {
	return w.dCache.Put(ctx, hash, task.Id, task.Created)
}

// Cleanup removes old entries from the cache.
func (w *DedupeCacheWrapper) Cleanup(ctx context.Context) error {
	return w.dCache.Cleanup(ctx)
}
