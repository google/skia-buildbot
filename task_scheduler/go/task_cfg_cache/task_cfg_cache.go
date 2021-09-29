package task_cfg_cache

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"time"

	"cloud.google.com/go/bigtable"
	"go.opencensus.io/trace"
	"go.skia.org/infra/go/atomic_miss_cache"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/task_scheduler/go/specs"
	"go.skia.org/infra/task_scheduler/go/types"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
)

const (
	// BigTable configuration.

	// We use a single BigTable table for storing gob-encoded TaskSpecs and
	// JobSpecs.
	BT_TABLE = "tasks-cfg"

	// We use a single BigTable column family.
	BT_COLUMN_FAMILY = "CFGS"

	// We use a single BigTable column which stores gob-encoded TaskSpecs
	// and JobSpecs.
	BT_COLUMN = "CFG"

	INSERT_TIMEOUT = 30 * time.Second
	QUERY_TIMEOUT  = 5 * time.Second
)

var (
	// Fully-qualified BigTable column name.
	BT_COLUMN_FULL = fmt.Sprintf("%s:%s", BT_COLUMN_FAMILY, BT_COLUMN)

	ErrNoSuchEntry = atomic_miss_cache.ErrNoSuchEntry
)

// TaskCfgCache is a struct used for caching tasks cfg files. The user should
// periodically call Cleanup() to remove old entries.
type TaskCfgCache struct {
	cache  *atomic_miss_cache.AtomicMissCache
	client *bigtable.Client
	repos  repograph.Map
}

// backingCache implements persistent storage of TasksCfgs in BigTable.
type backingCache struct {
	table *bigtable.Table
	tcc   *TaskCfgCache
}

// CachedValue represents a cached TasksCfg value. It includes any permanent
// error, which cannot be recovered via retries.
type CachedValue struct {
	// RepoState is the RepoState which is associated with this TasksCfg.
	RepoState types.RepoState
	// Cfg is the TasksCfg for this entry.
	Cfg *specs.TasksCfg
	// Err stores a permanent error. Mutually-exclusive with Cfg.
	Err string
}

// See documentation for atomic_miss_cache.ICache interface.
func (c *backingCache) Get(ctx context.Context, key string) (atomic_miss_cache.Value, error) {
	return GetTasksCfgFromBigTable(ctx, c.table, key)
}

// See documentation for atomic_miss_cache.ICache interface.
func (c *backingCache) Set(ctx context.Context, key string, val atomic_miss_cache.Value) error {
	cv := val.(*CachedValue)
	if !cv.RepoState.Valid() {
		return fmt.Errorf("Invalid RepoState: %+v", cv.RepoState)
	}
	return WriteTasksCfgToBigTable(ctx, c.table, key, cv)
}

// See documentation for atomic_miss_cache.ICache interface.
func (c *backingCache) Delete(ctx context.Context, key string) error {
	// We don't delete from BigTable.
	return nil
}

// NewTaskCfgCache returns a TaskCfgCache instance.
func NewTaskCfgCache(ctx context.Context, repos repograph.Map, btProject, btInstance string, ts oauth2.TokenSource) (*TaskCfgCache, error) {
	client, err := bigtable.NewClient(ctx, btProject, btInstance, option.WithTokenSource(ts))
	if err != nil {
		return nil, fmt.Errorf("Failed to create BigTable client: %s", err)
	}
	table := client.Open(BT_TABLE)
	c := &TaskCfgCache{
		client: client,
		repos:  repos,
	}
	c.cache = atomic_miss_cache.New(&backingCache{
		table: table,
		tcc:   c,
	})
	return c, nil
}

// GetTasksCfgFromBigTable retrieves a CachedValue from BigTable.
func GetTasksCfgFromBigTable(ctx context.Context, table *bigtable.Table, rowKey string) (*CachedValue, error) {
	var rv *CachedValue
	var rvErr error
	if err := table.ReadRows(ctx, bigtable.PrefixRange(rowKey), func(row bigtable.Row) bool {
		for _, ri := range row[BT_COLUMN_FAMILY] {
			if ri.Column == BT_COLUMN_FULL {
				var cv CachedValue
				rvErr = gob.NewDecoder(bytes.NewReader(ri.Value)).Decode(&cv)
				if rvErr == nil {
					rv = &cv
				}
				return false
			}
		}
		return true
	}); err != nil {
		return nil, err
	}
	if rvErr != nil {
		return nil, rvErr
	}
	if rv == nil {
		return nil, ErrNoSuchEntry
	}
	return rv, nil
}

// WriteTasksCfgToBigTable writes the given CachedValue to BigTable.
func WriteTasksCfgToBigTable(ctx context.Context, table *bigtable.Table, key string, cv *CachedValue) error {
	rowKey := cv.RepoState.RowKey()
	if rowKey != key {
		return fmt.Errorf("Key doesn't match RepoState.RowKey(): %s vs %s", key, rowKey)
	}
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(cv); err != nil {
		return err
	}
	mut := bigtable.NewMutation()
	mut.Set(BT_COLUMN_FAMILY, BT_COLUMN, bigtable.ServerTime, buf.Bytes())
	return table.Apply(ctx, rowKey, mut)
}

// Close frees up resources used by the TaskCfgCache.
func (c *TaskCfgCache) Close() error {
	return c.client.Close()
}

// Get returns the TasksCfg (or error) for the given RepoState in the cache. If
// the given entry does not exist in the cache, reads through to BigTable to
// find it there and adds to the cache if it exists. If no entry exists for the
// given RepoState, returns ErrNoSuchEntry. If there is a cached (ie. permanent
// non-recoverable error for this RepoState) error, it is returned as the second
// return value.
func (c *TaskCfgCache) Get(ctx context.Context, rs types.RepoState) (*specs.TasksCfg, error, error) {
	ctx, span := trace.StartSpan(ctx, "taskcfgcache_Get")
	defer span.End()
	val, err := c.cache.Get(ctx, rs.RowKey())
	if err != nil {
		return nil, nil, err
	}
	cv := val.(*CachedValue)
	if cv.Err != "" {
		return nil, errors.New(cv.Err), nil
	}
	return cv.Cfg, nil, nil
}

// Sets the TasksCfg (or error) for the given RepoState in the cache.
func (c *TaskCfgCache) Set(ctx context.Context, rs types.RepoState, cfg *specs.TasksCfg, storedErr error) error {
	errString := ""
	if storedErr != nil {
		errString = storedErr.Error()
	}
	return c.cache.Set(ctx, rs.RowKey(), atomic_miss_cache.Value(&CachedValue{
		RepoState: rs,
		Cfg:       cfg,
		Err:       errString,
	}))
}

// Sets the TasksCfg (or error) for the given RepoState in the cache by calling
// the given function if no value already exists. Returns the existing or new
// CachedValue, or any error which occurred.
func (c *TaskCfgCache) SetIfUnset(ctx context.Context, rs types.RepoState, fn func(context.Context) (*CachedValue, error)) (*CachedValue, error) {
	cv, err := c.cache.SetIfUnset(ctx, rs.RowKey(), func(ctx context.Context) (atomic_miss_cache.Value, error) {
		val, err := fn(ctx)
		return val, err
	})
	if err != nil {
		return nil, err
	}
	return cv.(*CachedValue), nil
}

// getTaskSpecsForRepoStates returns a set of TaskSpecs for each of the given
// set of RepoStates, keyed by RepoState and TaskSpec name. If any of the
// RepoStates do not have a corresponding entry in the cache, they are simply
// left out.
func (c *TaskCfgCache) getTaskSpecsForRepoStates(ctx context.Context, rs []types.RepoState) (map[types.RepoState]map[string]*specs.TaskSpec, error) {
	rv := make(map[types.RepoState]map[string]*specs.TaskSpec, len(rs))
	for _, s := range rs {
		cached, err := c.cache.Get(ctx, s.RowKey())
		if err == ErrNoSuchEntry {
			sklog.Errorf("Entry not found in cache: %+v", s)
			continue
		} else if err != nil {
			return nil, err
		}
		val := cached.(*CachedValue)
		if val.Err != "" {
			sklog.Errorf("Cached entry has permanent error; skipping: %s", val.Err)
			continue
		}
		subMap := make(map[string]*specs.TaskSpec, len(val.Cfg.Tasks))
		for name, taskSpec := range val.Cfg.Tasks {
			subMap[name] = taskSpec.Copy()
		}
		rv[s] = subMap
	}
	return rv, nil
}

// GetTaskSpec returns the TaskSpec at the given RepoState, or an error if no
// such TaskSpec exists.
func (c *TaskCfgCache) GetTaskSpec(ctx context.Context, rs types.RepoState, name string) (*specs.TaskSpec, error) {
	cfg, cachedErr, err := c.Get(ctx, rs)
	if err != nil {
		return nil, err
	}
	if cachedErr != nil {
		return nil, cachedErr
	}
	t, ok := cfg.Tasks[name]
	if !ok {
		return nil, fmt.Errorf("No such task spec: %s @ %s", name, rs)
	}
	return t.Copy(), nil
}

// MakeJob is a helper function which retrieves the given JobSpec at the given
// RepoState and uses it to create a Job instance.
func (c *TaskCfgCache) MakeJob(ctx context.Context, rs types.RepoState, name string) (*types.Job, error) {
	cfg, cachedErr, err := c.Get(ctx, rs)
	if err != nil {
		return nil, err
	}
	if cachedErr != nil {
		return nil, cachedErr
	}
	spec, ok := cfg.Jobs[name]
	if !ok {
		return nil, fmt.Errorf("No such job: %s", name)
	}
	deps, err := spec.GetTaskSpecDAG(cfg)
	if err != nil {
		return nil, err
	}

	return &types.Job{
		Created:      now.Now(ctx),
		Dependencies: deps,
		Name:         name,
		Priority:     spec.Priority,
		RepoState:    rs,
		Tasks:        map[string][]*types.TaskSummary{},
	}, nil
}

// Cleanup removes cache entries which are outside of our scheduling window.
func (c *TaskCfgCache) Cleanup(ctx context.Context, period time.Duration) error {
	periodStart := now.Now(ctx).Add(-period)
	if err := c.cache.Cleanup(ctx, func(ctx context.Context, key string, val atomic_miss_cache.Value) bool {
		cv := val.(*CachedValue)
		details, err := cv.RepoState.GetCommit(c.repos)
		return err != nil || details.Timestamp.Before(periodStart)
	}); err != nil {
		return err
	}
	return nil
}
