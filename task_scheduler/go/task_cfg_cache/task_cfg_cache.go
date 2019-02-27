package task_cfg_cache

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/bigtable"
	"go.skia.org/infra/go/atomic_miss_cache"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/specs"
	"go.skia.org/infra/task_scheduler/go/types"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
)

const (
	// BigTable configuration.

	// BigTable used for storing TaskCfgs.
	BT_INSTANCE_PROD     = "tasks-cfg-prod"
	BT_INSTANCE_INTERNAL = "tasks-cfg-internal"
	BT_INSTANCE_STAGING  = "tasks-cfg-staging"

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
	// protected by mtx
	cache  *atomic_miss_cache.AtomicMissCache
	client *bigtable.Client
	mtx    sync.RWMutex
	// protected by mtx
	addedTasksCache map[types.RepoState]util.StringSet
	recentCommits   map[string]time.Time
	recentJobSpecs  map[string]time.Time
	// protects recentCommits, recentJobSpecs, and recentTaskSpecs. When
	// locking multiple mutexes, mtx should be locked first, followed by
	// cache[*].mtx when applicable, then recentMtx.
	recentMtx       sync.RWMutex
	recentTaskSpecs map[string]time.Time
	repos           repograph.Map
}

// backingCache implements persistent storage of TasksCfgs in BigTable.
type backingCache struct {
	table *bigtable.Table
	tcc   *TaskCfgCache
}

// CachedValue represents a cached TasksCfg value. It includes any permanent
// error, which cannot be recovered via retries.
type CachedValue struct {
	RepoState types.RepoState
	Cfg       *specs.TasksCfg
	Err       error
}

// See documentation for atomic_miss_cache.ICache interface.
func (c *backingCache) Get(ctx context.Context, key string) (atomic_miss_cache.Value, error) {
	cv, err := GetTasksCfgFromBigTable(ctx, c.table, key)
	if err != nil {
		return nil, err
	}
	if cv.Err == nil {
		if err := c.tcc.updateSecondaryCaches(cv.RepoState, cv.Cfg); err != nil {
			return nil, err
		}
	}
	return cv, nil
}

// See documentation for atomic_miss_cache.ICache interface.
func (c *backingCache) Set(ctx context.Context, key string, val atomic_miss_cache.Value) error {
	cv := val.(*CachedValue)
	if !cv.RepoState.Valid() {
		return fmt.Errorf("Invalid RepoState: %+v", cv.RepoState)
	}
	if err := WriteTasksCfgToBigTable(ctx, c.table, key, cv); err != nil {
		return err
	}
	if cv.Err == nil {
		if err := c.tcc.updateSecondaryCaches(cv.RepoState, cv.Cfg); err != nil {
			return err
		}
	}
	return nil
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
		repos: repos,
	}
	c.cache = atomic_miss_cache.New(&backingCache{
		table: table,
		tcc:   c,
	})
	// TODO(borenet): Pre-fetch entries for commits in range. This would be
	// simpler if we passed in a Window or a list of commits or RepoStates.
	// Maybe the recent* caches belong in a separate cache entirely?
	c.addedTasksCache = map[types.RepoState]util.StringSet{}
	c.recentCommits = map[string]time.Time{}
	c.recentJobSpecs = map[string]time.Time{}
	c.recentTaskSpecs = map[string]time.Time{}
	return c, nil
}

// GetTasksCfgFromBigTable retrieves a CachedValue from BigTable.
func GetTasksCfgFromBigTable(ctx context.Context, table *bigtable.Table, repoStateRowKey string) (*CachedValue, error) {
	// Retrieve all rows for the TasksCfg from BigTable.
	tasks := map[string]*specs.TaskSpec{}
	jobs := map[string]*specs.JobSpec{}
	var processErr error
	var storedErr error
	ctx, cancel := context.WithTimeout(ctx, QUERY_TIMEOUT)
	defer cancel()
	if err := table.ReadRows(ctx, bigtable.PrefixRange(repoStateRowKey), func(row bigtable.Row) bool {
		for _, ri := range row[BT_COLUMN_FAMILY] {
			if ri.Column == BT_COLUMN_FULL {
				suffix := strings.Split(strings.TrimPrefix(row.Key(), repoStateRowKey+"#"), "#")
				if len(suffix) != 2 {
					processErr = fmt.Errorf("Invalid row key; expected two parts after %q; but have: %v", repoStateRowKey, suffix)
					return false
				}
				typ := suffix[0]
				name := suffix[1]
				if typ == "t" {
					var task specs.TaskSpec
					processErr = gob.NewDecoder(bytes.NewReader(ri.Value)).Decode(&task)
					if processErr != nil {
						return false
					}
					tasks[suffix[1]] = &task
				} else if typ == "j" {
					var job specs.JobSpec
					processErr = gob.NewDecoder(bytes.NewReader(ri.Value)).Decode(&job)
					if processErr != nil {
						return false
					}
					jobs[name] = &job
				} else if typ == "e" {
					storedErr = errors.New(string(ri.Value))
					return false
				} else {
					processErr = fmt.Errorf("Invalid row key %q; unknown entry type %q", row.Key(), suffix[0])
					return false
				}
				// We only store one message per row.
				return true
			}
		}
		return true
	}, bigtable.RowFilter(bigtable.LatestNFilter(1))); err != nil {
		return nil, fmt.Errorf("Failed to retrieve data from BigTable: %s", err)
	}
	if processErr != nil {
		return nil, fmt.Errorf("Failed to process row: %s", processErr)
	}
	rs, err := types.RepoStateFromRowKey(repoStateRowKey)
	if err != nil {
		return nil, err
	}
	rv := &CachedValue{
		RepoState: rs,
	}
	if storedErr != nil {
		rv.Err = storedErr
		return rv, nil
	}
	if len(tasks) == 0 {
		return nil, ErrNoSuchEntry
	}
	if len(jobs) == 0 {
		return nil, ErrNoSuchEntry
	}
	rv.Cfg = &specs.TasksCfg{
		Tasks: tasks,
		Jobs:  jobs,
	}
	return rv, nil
}

// WriteTasksCfgToBigTable writes the given CachedValue to BigTable.
func WriteTasksCfgToBigTable(ctx context.Context, table *bigtable.Table, key string, cv *CachedValue) error {
	rowKey := cv.RepoState.RowKey()
	if rowKey != key {
		return fmt.Errorf("Key doesn't match RepoState.RowKey(): %s vs %s", key, rowKey)
	}
	var rks []string
	var mts []*bigtable.Mutation
	prefix := cv.RepoState.RowKey() + "#"
	if cv.Err != nil {
		rks = append(rks, prefix+"e#")
		mt := bigtable.NewMutation()
		mt.Set(BT_COLUMN_FAMILY, BT_COLUMN, bigtable.ServerTime, []byte(cv.Err.Error()))
		mts = append(mts, mt)
	} else {
		rks = make([]string, 0, len(cv.Cfg.Tasks)+len(cv.Cfg.Jobs))
		mts = make([]*bigtable.Mutation, 0, len(cv.Cfg.Tasks)+len(cv.Cfg.Jobs))
		for name, task := range cv.Cfg.Tasks {
			rks = append(rks, prefix+"t#"+name)
			buf := bytes.Buffer{}
			if err := gob.NewEncoder(&buf).Encode(task); err != nil {
				return err
			}
			mt := bigtable.NewMutation()
			mt.Set(BT_COLUMN_FAMILY, BT_COLUMN, bigtable.ServerTime, buf.Bytes())
			mts = append(mts, mt)
		}
		for name, job := range cv.Cfg.Jobs {
			rks = append(rks, prefix+"j#"+name)
			buf := bytes.Buffer{}
			if err := gob.NewEncoder(&buf).Encode(job); err != nil {
				return err
			}
			mt := bigtable.NewMutation()
			mt.Set(BT_COLUMN_FAMILY, BT_COLUMN, bigtable.ServerTime, buf.Bytes())
			mts = append(mts, mt)
		}
	}
	ctx, cancel := context.WithTimeout(ctx, INSERT_TIMEOUT)
	defer cancel()
	errs, err := table.ApplyBulk(ctx, rks, mts)
	if err != nil {
		return err
	}
	for _, err := range errs {
		if err != nil {
			// TODO(borenet): Should we retry? Delete the inserted entries?
			return err
		}
	}
	return nil
}

// Close frees up resources used by the TaskCfgCache.
func (c *TaskCfgCache) Close() error {
	return c.client.Close()
}

// updateSecondaryCaches updates the secondary in-memory caches in the
// TaskCfgfCache.
func (c *TaskCfgCache) updateSecondaryCaches(rs types.RepoState, cfg *specs.TasksCfg) error {
	// Write the commit and task specs into the recent lists.
	c.recentMtx.Lock()
	defer c.recentMtx.Unlock()
	r, ok := c.repos[rs.Repo]
	if !ok {
		return fmt.Errorf("Unknown repo %s", rs.Repo)
	}
	d := r.Get(rs.Revision)
	if d == nil {
		return fmt.Errorf("Unknown revision %s in %s", rs.Revision, rs.Repo)
	}
	ts := d.Timestamp
	if ts.After(c.recentCommits[rs.Revision]) {
		c.recentCommits[rs.Revision] = ts
	}
	for name := range cfg.Tasks {
		if ts.After(c.recentTaskSpecs[name]) {
			c.recentTaskSpecs[name] = ts
		}
	}
	for name := range cfg.Jobs {
		if ts.After(c.recentJobSpecs[name]) {
			c.recentJobSpecs[name] = ts
		}
	}
	return nil
}

// Get returns the TasksCfg (or error) for the given RepoState in the cache. If
// the given entry does not exist in the cache, reads through to BigTable to
// find it there and adds to the cache if it exists. If no entry exists for the
// given RepoState, returns ErrNoSuchEntry.
func (c *TaskCfgCache) Get(ctx context.Context, rs types.RepoState) (*specs.TasksCfg, error) {
	val, err := c.cache.Get(ctx, rs.RowKey())
	if err != nil {
		return nil, err
	}
	cv := val.(*CachedValue)
	return cv.Cfg, cv.Err
}

// Sets the TasksCfg (or error) for the given RepoState in the cache.
func (c *TaskCfgCache) Set(ctx context.Context, rs types.RepoState, cfg *specs.TasksCfg, storedErr error) error {
	return c.cache.Set(ctx, rs.RowKey(), atomic_miss_cache.Value(&CachedValue{
		RepoState: rs,
		Cfg:       cfg,
		Err:       storedErr,
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
		if val.Err != nil {
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
	cfg, err := c.Get(ctx, rs)
	if err != nil {
		return nil, err
	}
	t, ok := cfg.Tasks[name]
	if !ok {
		return nil, fmt.Errorf("No such task spec: %s @ %s", name, rs)
	}
	return t.Copy(), nil
}

// GetAddedTaskSpecsForRepoStates returns a mapping from each input RepoState to
// the set of task names that were added at that RepoState. Note that this will
// only be accurate if all rss and their parents are already in the cache.
func (c *TaskCfgCache) GetAddedTaskSpecsForRepoStates(ctx context.Context, rss []types.RepoState) (map[types.RepoState]util.StringSet, error) {
	rv := make(map[types.RepoState]util.StringSet, len(rss))
	// todoParents collects the RepoStates in rss that are not in
	// c.addedTasksCache. We also save the RepoStates' parents so we don't
	// have to recompute them later.
	todoParents := make(map[types.RepoState][]types.RepoState, 0)
	// allTodoRs collects the RepoStates for which we need to look up
	// TaskSpecs.
	allTodoRs := []types.RepoState{}
	if err := func() error {
		c.mtx.RLock()
		defer c.mtx.RUnlock()
		for _, rs := range rss {
			val, ok := c.addedTasksCache[rs]
			if ok {
				rv[rs] = val.Copy()
			} else {
				allTodoRs = append(allTodoRs, rs)
				parents, err := rs.Parents(c.repos)
				if err != nil {
					return err
				}
				allTodoRs = append(allTodoRs, parents...)
				todoParents[rs] = parents
			}
		}
		return nil
	}(); err != nil {
		return nil, err
	}
	if len(todoParents) == 0 {
		return rv, nil
	}
	taskSpecs, err := c.getTaskSpecsForRepoStates(ctx, allTodoRs)
	if err != nil {
		return nil, err
	}
	c.mtx.Lock()
	defer c.mtx.Unlock()
	for cur, parents := range todoParents {
		addedTasks := util.NewStringSet()
		for task := range taskSpecs[cur] {
			// If this revision has no parents, the task spec is added by this
			// revision.
			addedByCur := len(parents) == 0
			for _, parent := range parents {
				if _, ok := taskSpecs[parent][task]; !ok {
					// If missing in parrent, the task spec is added by this revision.
					addedByCur = true
					break
				}
			}
			if addedByCur {
				addedTasks[task] = true
			}
		}
		c.addedTasksCache[cur] = addedTasks.Copy()
		rv[cur] = addedTasks
	}
	return rv, nil
}

// MakeJob is a helper function which retrieves the given JobSpec at the given
// RepoState and uses it to create a Job instance.
func (c *TaskCfgCache) MakeJob(ctx context.Context, rs types.RepoState, name string) (*types.Job, error) {
	cfg, err := c.Get(ctx, rs)
	if err != nil {
		return nil, err
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
		Created:      time.Now(),
		Dependencies: deps,
		Name:         name,
		Priority:     spec.Priority,
		RepoState:    rs,
		Tasks:        map[string][]*types.TaskSummary{},
	}, nil
}

// Cleanup removes cache entries which are outside of our scheduling window.
func (c *TaskCfgCache) Cleanup(ctx context.Context, period time.Duration) error {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	periodStart := time.Now().Add(-period)
	if err := c.cache.Cleanup(ctx, func(ctx context.Context, key string, val atomic_miss_cache.Value) bool {
		cv := val.(*CachedValue)
		details, err := cv.RepoState.GetCommit(c.repos)
		return err != nil || details.Timestamp.Before(periodStart)
	}); err != nil {
		return err
	}
	for repoState := range c.addedTasksCache {
		details, err := repoState.GetCommit(c.repos)
		if err != nil || details.Timestamp.Before(periodStart) {
			delete(c.addedTasksCache, repoState)
		}
	}
	c.recentMtx.Lock()
	defer c.recentMtx.Unlock()
	for k, ts := range c.recentCommits {
		if ts.Before(periodStart) {
			delete(c.recentCommits, k)
		}
	}
	for k, ts := range c.recentTaskSpecs {
		if ts.Before(periodStart) {
			delete(c.recentTaskSpecs, k)
		}
	}
	for k, ts := range c.recentJobSpecs {
		if ts.Before(periodStart) {
			delete(c.recentJobSpecs, k)
		}
	}
	return nil
}

// stringMapKeys returns a slice containing the keys of a map[string]time.Time.
func stringMapKeys(m map[string]time.Time) []string {
	rv := make([]string, 0, len(m))
	for k := range m {
		rv = append(rv, k)
	}
	return rv
}

// RecentSpecsAndCommits returns lists of recent job and task spec names and
// commit hashes.
func (c *TaskCfgCache) RecentSpecsAndCommits() ([]string, []string, []string) {
	c.recentMtx.RLock()
	defer c.recentMtx.RUnlock()
	return stringMapKeys(c.recentJobSpecs), stringMapKeys(c.recentTaskSpecs), stringMapKeys(c.recentCommits)
}
