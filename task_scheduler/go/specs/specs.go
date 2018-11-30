package specs

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/types"
)

const (
	DEFAULT_TASK_SPEC_MAX_ATTEMPTS = types.DEFAULT_MAX_TASK_ATTEMPTS
	DEFAULT_NUM_WORKERS            = 10

	// The default JobSpec.Priority, when unspecified or invalid.
	DEFAULT_JOB_SPEC_PRIORITY = 0.5

	TASKS_CFG_FILE = "infra/bots/tasks.json"

	// Triggering configuration for jobs.

	// By default, all jobs trigger on any branch for which they are
	// defined.
	TRIGGER_ANY_BRANCH = ""
	// Run this job on the master branch only, even if it is defined on
	// others.
	TRIGGER_MASTER_ONLY = "master"
	// Trigger this job every night.
	TRIGGER_NIGHTLY = "nightly"
	// Don't trigger this job automatically. It will only be run when
	// explicitly triggered via a try job or a force trigger.
	TRIGGER_ON_DEMAND = "on demand"
	// Trigger this job weekly.
	TRIGGER_WEEKLY = "weekly"

	VARIABLE_SYNTAX = "<(%s)"

	VARIABLE_BUILDBUCKET_BUILD_ID = "BUILDBUCKET_BUILD_ID"
	VARIABLE_CODEREVIEW_SERVER    = "CODEREVIEW_SERVER"
	VARIABLE_ISSUE                = "ISSUE"
	VARIABLE_ISSUE_SHORT          = "ISSUE_SHORT"
	VARIABLE_PATCH_REF            = "PATCH_REF"
	VARIABLE_PATCH_REPO           = "PATCH_REPO"
	VARIABLE_PATCH_STORAGE        = "PATCH_STORAGE"
	VARIABLE_PATCHSET             = "PATCHSET"
	VARIABLE_REPO                 = "REPO"
	VARIABLE_REVISION             = "REVISION"
	VARIABLE_TASK_ID              = "TASK_ID"
	VARIABLE_TASK_NAME            = "TASK_NAME"
)

var (
	PLACEHOLDER_BUILDBUCKET_BUILD_ID = fmt.Sprintf(VARIABLE_SYNTAX, VARIABLE_BUILDBUCKET_BUILD_ID)
	PLACEHOLDER_CODEREVIEW_SERVER    = fmt.Sprintf(VARIABLE_SYNTAX, VARIABLE_CODEREVIEW_SERVER)
	PLACEHOLDER_ISSUE                = fmt.Sprintf(VARIABLE_SYNTAX, VARIABLE_ISSUE)
	PLACEHOLDER_ISSUE_SHORT          = fmt.Sprintf(VARIABLE_SYNTAX, VARIABLE_ISSUE_SHORT)
	PLACEHOLDER_PATCH_REF            = fmt.Sprintf(VARIABLE_SYNTAX, VARIABLE_PATCH_REF)
	PLACEHOLDER_PATCH_REPO           = fmt.Sprintf(VARIABLE_SYNTAX, VARIABLE_PATCH_REPO)
	PLACEHOLDER_PATCH_STORAGE        = fmt.Sprintf(VARIABLE_SYNTAX, VARIABLE_PATCH_STORAGE)
	PLACEHOLDER_PATCHSET             = fmt.Sprintf(VARIABLE_SYNTAX, VARIABLE_PATCHSET)
	PLACEHOLDER_REPO                 = fmt.Sprintf(VARIABLE_SYNTAX, VARIABLE_REPO)
	PLACEHOLDER_REVISION             = fmt.Sprintf(VARIABLE_SYNTAX, VARIABLE_REVISION)
	PLACEHOLDER_TASK_ID              = fmt.Sprintf(VARIABLE_SYNTAX, VARIABLE_TASK_ID)
	PLACEHOLDER_TASK_NAME            = fmt.Sprintf(VARIABLE_SYNTAX, VARIABLE_TASK_NAME)
	PLACEHOLDER_ISOLATED_OUTDIR      = "${ISOLATED_OUTDIR}"

	PERIODIC_TRIGGERS = []string{TRIGGER_NIGHTLY, TRIGGER_WEEKLY}
)

// ParseTasksCfg parses the given task cfg file contents and returns the config.
func ParseTasksCfg(contents string) (*TasksCfg, error) {
	var rv TasksCfg
	if err := json.Unmarshal([]byte(contents), &rv); err != nil {
		return nil, fmt.Errorf("Failed to read tasks cfg: could not parse file: %s\nContents:\n%s", err, string(contents))
	}
	if err := rv.Validate(); err != nil {
		return nil, err
	}

	return &rv, nil
}

// EncoderTasksCfg writes the TasksCfg to a byte slice.
func EncodeTasksCfg(cfg *TasksCfg) ([]byte, error) {
	// Encode the JSON config.
	enc, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, err
	}
	// The json package escapes HTML characters, which makes our output
	// much less readable. Replace the escape characters with the real
	// character.
	enc = bytes.Replace(enc, []byte("\\u003c"), []byte("<"), -1)

	// Add a newline to the end of the file. Most text editors add one, so
	// adding one here enables manual editing of the file, even though we'd
	// rather that not happen.
	enc = append(enc, []byte("\n")...)
	return enc, nil
}

// ReadTasksCfg reads the task cfg file from the given dir and returns it.
func ReadTasksCfg(repoDir string) (*TasksCfg, error) {
	contents, err := ioutil.ReadFile(path.Join(repoDir, TASKS_CFG_FILE))
	if err != nil {
		return nil, fmt.Errorf("Failed to read tasks cfg: could not read file: %s", err)
	}
	return ParseTasksCfg(string(contents))
}

// WriteTasksCfg writes the task cfg to the given repo.
func WriteTasksCfg(cfg *TasksCfg, repoDir string) error {
	enc, err := EncodeTasksCfg(cfg)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(path.Join(repoDir, TASKS_CFG_FILE), enc, os.ModePerm)
}

// TasksCfg is a struct which describes all Swarming tasks for a repo at a
// particular commit.
type TasksCfg struct {
	// Jobs is a map whose keys are JobSpec names and values are JobSpecs
	// which describe sets of tasks to run.
	Jobs map[string]*JobSpec `json:"jobs"`

	// Tasks is a map whose keys are TaskSpec names and values are TaskSpecs
	// detailing the Swarming tasks which may be run.
	Tasks map[string]*TaskSpec `json:"tasks"`
}

// Validate returns an error if the TasksCfg is not valid.
func (c *TasksCfg) Validate() error {
	for _, t := range c.Tasks {
		if err := t.Validate(c); err != nil {
			return err
		}
	}

	if err := findCycles(c.Tasks, c.Jobs); err != nil {
		return err
	}

	return nil
}

// TaskSpec is a struct which describes a Swarming task to run.
// Be sure to add any new fields to the Copy() method.
type TaskSpec struct {
	// Caches are named Swarming caches which should be used for this task.
	Caches []*Cache `json:"caches,omitempty"`

	// CipdPackages are CIPD packages which should be installed for the task.
	CipdPackages []*CipdPackage `json:"cipd_packages,omitempty"`

	// Command is the command to run in the Swarming task. If not specified
	// here, it should be specified in the .isolate file.
	Command []string `json:"command,omitempty"`

	// Dependencies are names of other TaskSpecs for tasks which need to run
	// before this task.
	Dependencies []string `json:"dependencies,omitempty"`

	// Dimensions are Swarming bot dimensions which describe the type of bot
	// which may run this task.
	Dimensions []string `json:"dimensions"`

	// Environment is a set of environment variables needed by the task.
	Environment map[string]string `json:"environment,omitempty"`

	// EnvPrefixes are prefixes to add to environment variables for the task,
	// for example, adding directories to PATH. Keys are environment variable
	// names and values are multiple values to add for the variable.
	EnvPrefixes map[string][]string `json:"env_prefixes,omitempty"`

	// ExecutionTimeout is the maximum amount of time the task is allowed
	// to take.
	ExecutionTimeout time.Duration `json:"execution_timeout_ns,omitempty"`

	// Expiration is how long the task may remain in the pending state
	// before it is abandoned.
	Expiration time.Duration `json:"expiration_ns,omitempty"`

	// ExtraArgs are extra command-line arguments to pass to the task.
	ExtraArgs []string `json:"extra_args,omitempty"`

	// ExtraTags are extra tags to add to the Swarming task.
	ExtraTags map[string]string `json:"extra_tags,omitempty"`

	// IoTimeout is the maximum amount of time which the task may take to
	// communicate with the server.
	IoTimeout time.Duration `json:"io_timeout_ns,omitempty"`

	// Isolate is the name of the isolate file used by this task.
	Isolate string `json:"isolate"`

	// MaxAttempts is the maximum number of attempts for this TaskSpec. If
	// zero, DEFAULT_TASK_SPEC_MAX_ATTEMPTS is used.
	MaxAttempts int `json:"max_attempts,omitempty"`

	// Outputs are files and/or directories to use as outputs for the task.
	// Paths are relative to the task workdir. No error occurs if any of
	// these is missing.
	Outputs []string `json:"outputs,omitempty"`

	// This field is ignored.
	Priority float64 `json:"priority,omitempty"`

	// ServiceAccount indicates the Swarming service account to use for the
	// task. If not specified, we will attempt to choose a suitable default.
	ServiceAccount string `json:"service_account,omitempty"`
}

// Validate ensures that the TaskSpec is defined properly.
func (t *TaskSpec) Validate(cfg *TasksCfg) error {
	// Ensure that CIPD packages are specified properly.
	for _, p := range t.CipdPackages {
		if p.Name == "" || p.Path == "" {
			return fmt.Errorf("CIPD packages must have a name, path, and version.")
		}
	}

	// Ensure that the dimensions are specified properly.
	for _, d := range t.Dimensions {
		split := strings.SplitN(d, ":", 2)
		if len(split) != 2 {
			return fmt.Errorf("Dimension %q does not contain a colon!", d)
		}
	}

	// Isolate file is required.
	if t.Isolate == "" {
		return fmt.Errorf("Isolate file is required.")
	}

	return nil
}

// Copy returns a copy of the TaskSpec.
func (t *TaskSpec) Copy() *TaskSpec {
	var caches []*Cache
	if len(t.Caches) > 0 {
		cachesDup := make([]Cache, len(t.Caches))
		caches = make([]*Cache, 0, len(t.Caches))
		for i, c := range t.Caches {
			cachesDup[i] = *c
			caches = append(caches, &cachesDup[i])
		}
	}
	var cipdPackages []*CipdPackage
	if len(t.CipdPackages) > 0 {
		cipdPackages = make([]*CipdPackage, 0, len(t.CipdPackages))
		pkgs := make([]CipdPackage, len(t.CipdPackages))
		for i, p := range t.CipdPackages {
			pkgs[i] = *p
			cipdPackages = append(cipdPackages, &pkgs[i])
		}
	}
	cmd := util.CopyStringSlice(t.Command)
	deps := util.CopyStringSlice(t.Dependencies)
	dims := util.CopyStringSlice(t.Dimensions)
	environment := util.CopyStringMap(t.Environment)
	var envPrefixes map[string][]string
	if len(t.EnvPrefixes) > 0 {
		envPrefixes = make(map[string][]string, len(t.EnvPrefixes))
		for k, v := range t.EnvPrefixes {
			envPrefixes[k] = util.CopyStringSlice(v)
		}
	}
	extraArgs := util.CopyStringSlice(t.ExtraArgs)
	extraTags := util.CopyStringMap(t.ExtraTags)
	outputs := util.CopyStringSlice(t.Outputs)
	return &TaskSpec{
		Caches:           caches,
		CipdPackages:     cipdPackages,
		Command:          cmd,
		Dependencies:     deps,
		Dimensions:       dims,
		Environment:      environment,
		EnvPrefixes:      envPrefixes,
		ExecutionTimeout: t.ExecutionTimeout,
		Expiration:       t.Expiration,
		ExtraArgs:        extraArgs,
		ExtraTags:        extraTags,
		IoTimeout:        t.IoTimeout,
		Isolate:          t.Isolate,
		MaxAttempts:      t.MaxAttempts,
		Outputs:          outputs,
		Priority:         t.Priority,
		ServiceAccount:   t.ServiceAccount,
	}
}

// Cache is a struct representing a named cache which is used by a task.
type Cache struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// CipdPackage is a struct representing a CIPD package which needs to be
// installed on a bot for a particular task.
type CipdPackage struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Version string `json:"version"`
}

// JobSpec is a struct which describes a set of TaskSpecs to run as part of a
// larger effort.
type JobSpec struct {
	// Priority indicates the relative priority of the job, with 0 < p <= 1,
	// where higher values result in scheduling the job's tasks sooner. If
	// unspecified or outside this range, DEFAULT_JOB_SPEC_PRIORITY is used.
	// Each task derives its priority from the set of jobs that depend upon
	// it. A task's priority is
	//   1 - (1-<job1 priority>)(1-<job2 priority>)...(1-<jobN priority>)
	// A task at HEAD with a priority of 1 and a blamelist of 1 commit has
	// approximately the same score as a task at HEAD with a priority of 0.57
	// and a blamelist of 2 commits.
	// A backfill task with a priority of 1 that bisects a blamelist of 2
	// commits has the same score as another backfill task at the same
	// commit with a priority of 0.4 that bisects a blamelist of 4 commits.
	Priority float64 `json:"priority,omitempty"`
	// The names of TaskSpecs that are direct dependencies of this JobSpec.
	TaskSpecs []string `json:"tasks"`
	// One of the TRIGGER_* constants; see documentation above.
	Trigger string `json:"trigger,omitempty"`
}

// Copy returns a copy of the JobSpec.
func (j *JobSpec) Copy() *JobSpec {
	var taskSpecs []string
	if j.TaskSpecs != nil {
		taskSpecs = make([]string, len(j.TaskSpecs))
		copy(taskSpecs, j.TaskSpecs)
	}
	return &JobSpec{
		Priority:  j.Priority,
		TaskSpecs: taskSpecs,
		Trigger:   j.Trigger,
	}
}

// GetTaskSpecDAG returns a map describing all of the dependencies of the
// JobSpec. Its keys are TaskSpec names and values are TaskSpec names upon
// which the keys depend.
func (j *JobSpec) GetTaskSpecDAG(cfg *TasksCfg) (map[string][]string, error) {
	rv := map[string][]string{}
	var visit func(string) error
	visit = func(name string) error {
		if _, ok := rv[name]; ok {
			return nil
		}
		spec, ok := cfg.Tasks[name]
		if !ok {
			return fmt.Errorf("No such task: %s", name)
		}
		deps := util.CopyStringSlice(spec.Dependencies)
		if deps == nil {
			deps = []string{}
		}
		rv[name] = deps
		for _, t := range deps {
			if err := visit(t); err != nil {
				return err
			}
		}
		return nil
	}

	for _, t := range j.TaskSpecs {
		if err := visit(t); err != nil {
			return nil, err
		}
	}
	return rv, nil
}

// TasksCfgFileHash represents the SHA-1 checksum of the contents of
// TASKS_CFG_FILE.
type TasksCfgFileHash [sha1.Size]byte

// TaskCfgCache is a struct used for caching tasks cfg files. The user should
// periodically call Cleanup() to remove old entries.
type TaskCfgCache struct {
	// protected by mtx
	cache         map[types.RepoState]*cacheEntry
	depotToolsDir string
	file          string
	mtx           sync.RWMutex
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
	queue           chan func(int)
	workdir         string
}

// gobCacheValue contains fields that can be deduplicated across cacheEntry
// instances.
type gobCacheValue struct {
	Cfg  *TasksCfg
	Err  string
	Hash TasksCfgFileHash
}

// gobTaskCfgCache is a struct used for (de)serializing TaskCfgCache instance.
type gobTaskCfgCache struct {
	AddedTasksCache map[types.RepoState]util.StringSet
	Values          []gobCacheValue
	RecentCommits   map[string]time.Time
	RecentJobSpecs  map[string]time.Time
	RecentTaskSpecs map[string]time.Time
	// Map value is an index into Values. (We can't just use pointers pointing to
	// the same object because GOB-encoding flattens/dereferences all pointers,
	// resulting in multiple copies in the encoded file.)
	RepoStates map[types.RepoState]int
}

// NewTaskCfgCache returns a TaskCfgCache instance.
func NewTaskCfgCache(ctx context.Context, repos repograph.Map, depotToolsDir, workdir string, numWorkers int) (*TaskCfgCache, error) {
	file := path.Join(workdir, "taskCfgCache.gob")
	queue := make(chan func(int))
	c := &TaskCfgCache{
		depotToolsDir: depotToolsDir,
		file:          file,
		queue:         queue,
		repos:         repos,
		workdir:       workdir,
	}
	f, err := os.Open(file)
	if err == nil {
		var gobCache gobTaskCfgCache
		if err := gob.NewDecoder(f).Decode(&gobCache); err != nil {
			util.Close(f)
			return nil, err
		}
		util.Close(f)
		c.addedTasksCache = gobCache.AddedTasksCache
		c.cache = make(map[types.RepoState]*cacheEntry, len(gobCache.RepoStates))
		c.recentCommits = gobCache.RecentCommits
		c.recentJobSpecs = gobCache.RecentJobSpecs
		c.recentTaskSpecs = gobCache.RecentTaskSpecs
		for rs, idx := range gobCache.RepoStates {
			if idx < 0 || idx >= len(gobCache.Values) {
				return nil, fmt.Errorf("Corrupt cache file %q: RepoStates index %d out of range.", file, idx)
			}
			gobCacheValue := gobCache.Values[idx]
			c.cache[rs] = &cacheEntry{
				c:    c,
				cfg:  gobCacheValue.Cfg,
				err:  gobCacheValue.Err,
				hash: gobCacheValue.Hash,
				rs:   rs,
			}
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("Failed to read cache file: %s", err)
	} else {
		c.cache = map[types.RepoState]*cacheEntry{}
		c.addedTasksCache = map[types.RepoState]util.StringSet{}
		c.recentCommits = map[string]time.Time{}
		c.recentJobSpecs = map[string]time.Time{}
		c.recentTaskSpecs = map[string]time.Time{}
	}
	for i := 0; i < numWorkers; i++ {
		go func(i int) {
			for f := range queue {
				f(i)
			}
		}(i)
	}
	return c, nil
}

// Close frees up resources used by the TaskCfgCache.
func (c *TaskCfgCache) Close() error {
	close(c.queue)
	return nil
}

type cacheEntry struct {
	c *TaskCfgCache
	// Only one of cfg or err may be non-empty.
	cfg  *TasksCfg
	err  string
	hash TasksCfgFileHash
	mtx  sync.Mutex
	rs   types.RepoState
}

// Get returns the TasksCfg for this cache entry. If it does not already exist
// in the cache, it is read from the repo and the bool return value is true,
// indicating that the caller should write out the cache.
func (e *cacheEntry) Get(ctx context.Context) (*TasksCfg, bool, error) {
	e.mtx.Lock()
	defer e.mtx.Unlock()
	if e.cfg != nil {
		return e.cfg, false, nil
	}
	if e.err != "" {
		return nil, false, errors.New(e.err)
	}

	// We haven't seen this RepoState before, or it's scrolled out of our
	// window. Read it.
	// Point the upstream to a local source of truth to eliminate network
	// latency.
	r, ok := e.c.repos[e.rs.Repo]
	if !ok {
		return nil, false, fmt.Errorf("Unknown repo %q", e.rs.Repo)
	}
	var cfg *TasksCfg
	if err := e.c.TempGitRepo(ctx, e.rs, e.rs.IsTryJob(), func(checkout *git.TempCheckout) error {
		contents, err := ioutil.ReadFile(path.Join(checkout.Dir(), TASKS_CFG_FILE))
		if err != nil {
			// The tasks.cfg file may not exist for a particular commit.
			if strings.Contains(err.Error(), "does not exist in") || strings.Contains(err.Error(), "exists on disk, but not in") || strings.Contains(err.Error(), "no such file or directory") {
				// In this case, use an empty config.
				cfg = &TasksCfg{
					Tasks: map[string]*TaskSpec{},
				}
				return nil
			} else {
				return fmt.Errorf("Failed to read tasks cfg: could not read file: %s", err)
			}
		}
		e.hash = TasksCfgFileHash(sha1.Sum(contents))
		cfg, err = ParseTasksCfg(string(contents))
		return err
	}); err != nil {
		if strings.Contains(err.Error(), "error: Failed to merge in the changes.") {
			e.err = err.Error()
		}
		return nil, false, err
	}
	e.cfg = cfg

	// Write the commit and task specs into the recent lists.
	// TODO(borenet): The below should probably go elsewhere.
	e.c.recentMtx.Lock()
	defer e.c.recentMtx.Unlock()
	d := r.Get(e.rs.Revision)
	if d == nil {
		return nil, false, fmt.Errorf("Unknown revision %s in %s", e.rs.Revision, e.rs.Repo)
	}
	ts := d.Timestamp
	if ts.After(e.c.recentCommits[e.rs.Revision]) {
		e.c.recentCommits[e.rs.Revision] = ts
	}
	for name := range cfg.Tasks {
		if ts.After(e.c.recentTaskSpecs[name]) {
			e.c.recentTaskSpecs[name] = ts
		}
	}
	for name := range cfg.Jobs {
		if ts.After(e.c.recentJobSpecs[name]) {
			e.c.recentJobSpecs[name] = ts
		}
	}
	return cfg, true, nil
}

func (c *TaskCfgCache) getEntry(rs types.RepoState) *cacheEntry {
	rv, ok := c.cache[rs]
	if !ok {
		rv = &cacheEntry{
			c:  c,
			rs: rs,
		}
		c.cache[rs] = rv
	}
	return rv
}

// ReadTasksCfg reads the task cfg file from the given RepoState and returns it.
// Stores a cache of already-read task cfg files. Syncs the repo and reads the
// file if needed.
func (c *TaskCfgCache) ReadTasksCfg(ctx context.Context, rs types.RepoState) (*TasksCfg, error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	entry := c.getEntry(rs)
	rv, mustWrite, err := entry.Get(ctx)
	if err != nil {
		return nil, err
	} else if mustWrite {
		c.recentMtx.Lock()
		defer c.recentMtx.Unlock()
		return rv, c.write()
	}
	return rv, err
}

// GetTaskSpecsForRepoStates returns a set of TaskSpecs for each of the
// given set of RepoStates, keyed by RepoState and TaskSpec name.
func (c *TaskCfgCache) GetTaskSpecsForRepoStates(ctx context.Context, rs []types.RepoState) (map[types.RepoState]map[string]*TaskSpec, error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	entries := make(map[types.RepoState]*cacheEntry, len(rs))
	for _, s := range rs {
		entries[s] = c.getEntry(s)
	}

	var m sync.Mutex
	var wg sync.WaitGroup
	rv := make(map[types.RepoState]map[string]*TaskSpec, len(rs))
	errs := []error{}
	mustWrite := false
	for s, entry := range entries {
		wg.Add(1)
		go func(s types.RepoState, entry *cacheEntry) {
			defer wg.Done()
			cfg, w, err := entry.Get(ctx)
			if err != nil {
				m.Lock()
				defer m.Unlock()
				errs = append(errs, err)
				return
			}
			// Make a copy of the task specs.
			subMap := make(map[string]*TaskSpec, len(cfg.Tasks))
			for name, task := range cfg.Tasks {
				subMap[name] = task.Copy()
			}
			m.Lock()
			defer m.Unlock()
			rv[s] = subMap
			if w {
				mustWrite = true
			}
		}(s, entry)
	}
	wg.Wait()
	if mustWrite {
		c.recentMtx.Lock()
		defer c.recentMtx.Unlock()
		if err := c.write(); err != nil {
			return nil, err
		}
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("Errors loading task cfgs: %v", errs)
	}
	return rv, nil
}

// GetTaskSpec returns the TaskSpec at the given RepoState, or an error if no
// such TaskSpec exists.
func (c *TaskCfgCache) GetTaskSpec(ctx context.Context, rs types.RepoState, name string) (*TaskSpec, error) {
	cfg, err := c.ReadTasksCfg(ctx, rs)
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
// the set of task names that were added at that RepoState.
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
	taskSpecs, err := c.GetTaskSpecsForRepoStates(ctx, allTodoRs)
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

// GetJobSpec returns the JobSpec at the given RepoState, or an error if no such
// JobSpec exists.
func (c *TaskCfgCache) GetJobSpec(ctx context.Context, rs types.RepoState, name string) (*JobSpec, error) {
	cfg, err := c.ReadTasksCfg(ctx, rs)
	if err != nil {
		return nil, err
	}
	j, ok := cfg.Jobs[name]
	if !ok {
		return nil, fmt.Errorf("No such job spec: %s @ %s", name, rs)
	}
	return j.Copy(), nil
}

// MakeJob is a helper function which retrieves the given JobSpec at the given
// RepoState and uses it to create a Job instance.
func (c *TaskCfgCache) MakeJob(ctx context.Context, rs types.RepoState, name string) (*types.Job, error) {
	cfg, err := c.ReadTasksCfg(ctx, rs)
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
func (c *TaskCfgCache) Cleanup(period time.Duration) error {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	periodStart := time.Now().Add(-period)
	for repoState := range c.cache {
		details, err := repoState.GetCommit(c.repos)
		if err != nil || details.Timestamp.Before(periodStart) {
			delete(c.cache, repoState)
		}
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
	return c.write()
}

// write writes the TaskCfgCache to a file. Assumes the caller holds both c.mtx
// and c.recentMtx.
func (c *TaskCfgCache) write() error {
	dir := path.Dir(c.file)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return err
	}
	return util.WithWriteFile(c.file, func(w io.Writer) error {
		gobCache := gobTaskCfgCache{
			AddedTasksCache: c.addedTasksCache,
			Values:          make([]gobCacheValue, 0, len(c.cache)),
			RecentCommits:   c.recentCommits,
			RecentJobSpecs:  c.recentJobSpecs,
			RecentTaskSpecs: c.recentTaskSpecs,
			RepoStates:      make(map[types.RepoState]int, len(c.cache)),
		}
		// When deduplicating gobCacheValue, we need to key by both hash and err. In
		// the case that a patch doesn't apply, hash will always be all-zero, but
		// err could vary.
		type valueKey struct {
			hash TasksCfgFileHash
			err  string
		}
		valueIdxMap := make(map[valueKey]int, len(c.cache))
		for _, e := range c.cache {
			if e.cfg == nil && e.err == "" {
				// Haven't called Get yet or Get failed; nothing to cache.
				continue
			}
			key := valueKey{
				hash: e.hash,
				err:  e.err,
			}
			idx, ok := valueIdxMap[key]
			if !ok {
				idx = len(gobCache.Values)
				valueIdxMap[key] = idx
				gobCache.Values = append(gobCache.Values, gobCacheValue{
					Cfg:  e.cfg,
					Err:  e.err,
					Hash: e.hash,
				})
			}
			gobCache.RepoStates[e.rs] = idx
		}
		if err := gob.NewEncoder(w).Encode(&gobCache); err != nil {
			return fmt.Errorf("Failed to encode TaskCfgCache: %s", err)
		}
		return nil
	})
}

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

// findCycles searches for cyclical dependencies in the task specs and returns
// an error if any are found. Also ensures that all task specs are reachable
// from at least one job spec and that all jobs specs' dependencies are valid.
func findCycles(tasks map[string]*TaskSpec, jobs map[string]*JobSpec) error {
	// Create vertex objects with metadata for the depth-first search.
	type vertex struct {
		active  bool
		name    string
		ts      *TaskSpec
		visited bool
	}
	vertices := make(map[string]*vertex, len(tasks))
	for name, t := range tasks {
		vertices[name] = &vertex{
			active:  false,
			name:    name,
			ts:      t,
			visited: false,
		}
	}

	// visit performs a depth-first search of the graph, starting at v.
	var visit func(*vertex) error
	visit = func(v *vertex) error {
		v.active = true
		v.visited = true
		for _, dep := range v.ts.Dependencies {
			e := vertices[dep]
			if e == nil {
				return fmt.Errorf("Task %q has unknown task %q as a dependency.", v.name, dep)
			}
			if !e.visited {
				if err := visit(e); err != nil {
					return err
				}
			} else if e.active {
				return fmt.Errorf("Found a circular dependency involving %q and %q", v.name, e.name)
			}
		}
		v.active = false
		return nil
	}

	// Perform a DFS, starting at each of the jobs' dependencies.
	for jobName, j := range jobs {
		for _, d := range j.TaskSpecs {
			v, ok := vertices[d]
			if !ok {
				return fmt.Errorf("Job %q has unknown task %q as a dependency.", jobName, d)
			}
			if !v.visited {
				if err := visit(v); err != nil {
					return err
				}
			}
		}
	}

	// If any vertices have not been visited, then there are tasks which
	// no job has as a dependency. Report an error.
	for _, v := range vertices {
		if !v.visited {
			return fmt.Errorf("Task %q is not reachable by any Job!", v.name)
		}
	}
	return nil
}

// TempGitRepo creates a git repository in a temporary directory, gets it into
// the given RepoState, and runs the given function inside the repo dir.
//
// This method uses a worker pool; if all workers are busy, it will block until
// one is free.
func (c *TaskCfgCache) TempGitRepo(ctx context.Context, rs types.RepoState, botUpdate bool, fn func(*git.TempCheckout) error) error {
	rvErr := make(chan error)
	c.queue <- func(workerId int) {
		var gr *git.TempCheckout
		var err error
		if botUpdate {
			tmp, err2 := ioutil.TempDir("", "")
			if err2 != nil {
				rvErr <- err2
				return
			}
			defer util.RemoveAll(tmp)
			cacheDir := path.Join(c.workdir, "cache", fmt.Sprintf("%d", workerId))
			gr, err = tempGitRepoBotUpdate(ctx, rs, c.depotToolsDir, cacheDir, tmp)
		} else {
			repo, ok := c.repos[rs.Repo]
			if !ok {
				rvErr <- fmt.Errorf("Unknown repo: %s", rs.Repo)
				return
			}
			gr, err = tempGitRepo(ctx, repo.Repo(), rs)
		}
		if err != nil {
			rvErr <- err
			return
		}
		defer gr.Delete()
		rvErr <- fn(gr)
	}
	return <-rvErr
}

// tempGitRepo creates a git repository in a temporary directory, gets it into
// the given RepoState, and returns its location.
func tempGitRepo(ctx context.Context, repo *git.Repo, rs types.RepoState) (rv *git.TempCheckout, rvErr error) {
	if rs.IsTryJob() {
		return nil, fmt.Errorf("specs.tempGitRepo does not apply patches, and should not be called for try jobs.")
	}

	c, err := repo.TempCheckout(ctx)
	if err != nil {
		return nil, err
	}

	defer func() {
		if rvErr != nil {
			c.Delete()
		}
	}()

	// Check out the correct commit.
	if _, err := c.Git(ctx, "checkout", rs.Revision); err != nil {
		return nil, err
	}

	return c, nil
}

// tempGitRepoBotUpdate creates a git repository in a temporary directory, gets it into
// the given RepoState, and returns its location.
func tempGitRepoBotUpdate(ctx context.Context, rs types.RepoState, depotToolsDir, gitCacheDir, tmp string) (*git.TempCheckout, error) {
	// Run bot_update to obtain a checkout of the repo and its DEPS.
	botUpdatePath := path.Join(depotToolsDir, "recipes", "recipe_modules", "bot_update", "resources", "bot_update.py")
	projectName := strings.TrimSuffix(path.Base(rs.Repo), ".git")
	spec := fmt.Sprintf("cache_dir = '%s'\nsolutions = [{'deps_file': '.DEPS.git', 'managed': False, 'name': '%s', 'url': '%s'}]", gitCacheDir, projectName, rs.Repo)
	revMap := map[string]string{
		projectName: "got_revision",
	}

	revisionMappingFile := path.Join(tmp, "revision_mapping")
	revMapBytes, err := json.Marshal(revMap)
	if err != nil {
		return nil, err
	}
	if err := ioutil.WriteFile(revisionMappingFile, revMapBytes, os.ModePerm); err != nil {
		return nil, err
	}

	patchRepo := rs.Repo
	patchRepoName := projectName
	if rs.PatchRepo != "" {
		patchRepo = rs.PatchRepo
		patchRepoName = strings.TrimSuffix(path.Base(rs.PatchRepo), ".git")
	}
	outputJson := path.Join(tmp, "output_json")
	cmd := []string{
		"python", "-u", botUpdatePath,
		"--specs", spec,
		"--patch_root", patchRepoName,
		"--revision_mapping_file", revisionMappingFile,
		"--git-cache-dir", gitCacheDir,
		"--output_json", outputJson,
		"--revision", fmt.Sprintf("%s@%s", projectName, rs.Revision),
	}
	if rs.IsTryJob() {
		if strings.Contains(rs.Server, "codereview.chromium") {
			cmd = append(cmd, []string{
				"--issue", rs.Issue,
				"--patchset", rs.Patchset,
			}...)
		} else {
			gerritRef := fmt.Sprintf("refs/changes/%s/%s/%s", rs.Issue[len(rs.Issue)-2:], rs.Issue, rs.Patchset)
			cmd = append(cmd, []string{
				"--patch_ref", fmt.Sprintf("%s@%s", patchRepo, gerritRef),
			}...)
		}
	}
	t := metrics2.NewTimer("bot_update", map[string]string{
		"patchRepo": patchRepo,
	})
	out, err := exec.RunCommand(ctx, &exec.Command{
		Name: cmd[0],
		Args: cmd[1:],
		Dir:  tmp,
		Env: []string{
			fmt.Sprintf("PATH=%s:%s", depotToolsDir, os.Getenv("PATH")),
		},
		InheritEnv: true,
	})
	dur := t.Stop()
	if err != nil {
		sklog.Warningf("bot_update error for %v; output: %s", rs, out)
		return nil, err
	}
	if dur > 5*time.Minute {
		sklog.Warningf("bot_update took %s for %v; output: %s", dur, rs, out)
	}

	// bot_update points the upstream to a local cache. Point back to the
	// "real" upstream, in case the caller cares about the remote URL. Note
	// that this doesn't change the remote URLs for the DEPS.
	co := &git.TempCheckout{
		GitDir: git.GitDir(path.Join(tmp, projectName)),
	}
	if _, err := co.Git(ctx, "remote", "set-url", "origin", rs.Repo); err != nil {
		return nil, err
	}

	// Self-check.
	head, err := co.RevParse(ctx, "HEAD")
	if err != nil {
		return nil, err
	}
	if head != rs.Revision {
		return nil, fmt.Errorf("TempGitRepo ended up at the wrong revision. Wanted %q but got %q", rs.Revision, head)
	}

	return co, nil
}
