package specs

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/bigtable"
	"go.skia.org/infra/go/atomic_miss_cache"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/periodic"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/types"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
)

const (
	DEFAULT_TASK_SPEC_MAX_ATTEMPTS = types.DEFAULT_MAX_TASK_ATTEMPTS

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
	TRIGGER_NIGHTLY = periodic.TRIGGER_NIGHTLY
	// Don't trigger this job automatically. It will only be run when
	// explicitly triggered via a try job or a force trigger.
	TRIGGER_ON_DEMAND = "on demand"
	// Trigger this job weekly.
	TRIGGER_WEEKLY = periodic.TRIGGER_WEEKLY

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

	ErrNoSuchEntry = atomic_miss_cache.ErrNoSuchEntry
)

// ErrorIsPermanent returns true if the given error cannot be recovered by
// retrying. In this case, we will never be able to process the TasksCfg,
// so we might as well cancel the jobs.
// TODO(borenet): This should probably be split into three different
// ErrorIsPermanent functions, in the syncer, isolate, and specs packages.
func ErrorIsPermanent(err error) bool {
	return (strings.Contains(err.Error(), "error: Failed to merge in the changes.") ||
		strings.Contains(err.Error(), "Failed to apply patch") ||
		strings.Contains(err.Error(), "failed to process isolate") ||
		strings.Contains(err.Error(), "Failed to read tasks cfg: could not parse file:") ||
		strings.Contains(err.Error(), "Invalid TasksCfg"))
}

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
		// A nonexistent tasks.json file is valid; return an empty config.
		if os.IsNotExist(err) {
			return &TasksCfg{}, nil
		}
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
			return fmt.Errorf("Invalid TasksCfg: %s", err)
		}
	}

	if err := findCycles(c.Tasks, c.Jobs); err != nil {
		return fmt.Errorf("Invalid TasksCfg: %s", err)
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
	Cfg       *TasksCfg
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
	tasks := map[string]*TaskSpec{}
	jobs := map[string]*JobSpec{}
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
					var task TaskSpec
					processErr = gob.NewDecoder(bytes.NewReader(ri.Value)).Decode(&task)
					if processErr != nil {
						return false
					}
					tasks[suffix[1]] = &task
				} else if typ == "j" {
					var job JobSpec
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
	rv.Cfg = &TasksCfg{
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
func (c *TaskCfgCache) updateSecondaryCaches(rs types.RepoState, cfg *TasksCfg) error {
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

// Get returns the TasksCfg (or error) for the given RepoState in the cache.
func (c *TaskCfgCache) Get(ctx context.Context, rs types.RepoState) (*TasksCfg, error) {
	val, err := c.cache.Get(ctx, rs.RowKey())
	if err != nil {
		return nil, err
	}
	cv := val.(*CachedValue)
	return cv.Cfg, cv.Err
}

// Sets the TasksCfg (or error) for the given RepoState in the cache.
func (c *TaskCfgCache) Set(ctx context.Context, rs types.RepoState, cfg *TasksCfg, storedErr error) error {
	return c.cache.Set(ctx, rs.RowKey(), atomic_miss_cache.Value(&CachedValue{
		RepoState: rs,
		Cfg:       cfg,
		Err:       storedErr,
	}))
}

// Sets the TasksCfg (or error) for the given RepoState in the cache by calling
// the given function if no value already exists.
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

// GetTaskSpecsForRepoStates returns a set of TaskSpecs for each of the
// given set of RepoStates, keyed by RepoState and TaskSpec name.
func (c *TaskCfgCache) GetTaskSpecsForRepoStates(ctx context.Context, rs []types.RepoState) (map[types.RepoState]map[string]*TaskSpec, error) {
	rv := make(map[types.RepoState]map[string]*TaskSpec, len(rs))
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
		subMap := make(map[string]*TaskSpec, len(val.Cfg.Tasks))
		for name, taskSpec := range val.Cfg.Tasks {
			subMap[name] = taskSpec.Copy()
		}
		rv[s] = subMap
	}
	return rv, nil
}

// GetTaskSpec returns the TaskSpec at the given RepoState, or an error if no
// such TaskSpec exists.
func (c *TaskCfgCache) GetTaskSpec(ctx context.Context, rs types.RepoState, name string) (*TaskSpec, error) {
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
	cfg, err := c.Get(ctx, rs)
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
