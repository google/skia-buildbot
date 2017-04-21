package specs

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/db"
)

const (
	ISSUE_SHORT_LENGTH = 2

	DEFAULT_TASK_SPEC_MAX_ATTEMPTS = db.DEFAULT_MAX_TASK_ATTEMPTS
	DEFAULT_NUM_WORKERS            = 10

	TASKS_CFG_FILE = "infra/bots/tasks.json"

	VARIABLE_SYNTAX = "<(%s)"

	VARIABLE_CODEREVIEW_SERVER = "CODEREVIEW_SERVER"
	VARIABLE_ISSUE             = "ISSUE"
	VARIABLE_ISSUE_SHORT       = "ISSUE_SHORT"
	VARIABLE_PATCH_REPO        = "PATCH_REPO"
	VARIABLE_PATCH_STORAGE     = "PATCH_STORAGE"
	VARIABLE_PATCHSET          = "PATCHSET"
	VARIABLE_REPO              = "REPO"
	VARIABLE_REVISION          = "REVISION"
	VARIABLE_TASK_NAME         = "TASK_NAME"
)

var (
	PLACEHOLDER_CODEREVIEW_SERVER = fmt.Sprintf(VARIABLE_SYNTAX, VARIABLE_CODEREVIEW_SERVER)
	PLACEHOLDER_ISSUE             = fmt.Sprintf(VARIABLE_SYNTAX, VARIABLE_ISSUE)
	PLACEHOLDER_ISSUE_SHORT       = fmt.Sprintf(VARIABLE_SYNTAX, VARIABLE_ISSUE_SHORT)
	PLACEHOLDER_PATCH_REPO        = fmt.Sprintf(VARIABLE_SYNTAX, VARIABLE_PATCH_REPO)
	PLACEHOLDER_PATCH_STORAGE     = fmt.Sprintf(VARIABLE_SYNTAX, VARIABLE_PATCH_STORAGE)
	PLACEHOLDER_PATCHSET          = fmt.Sprintf(VARIABLE_SYNTAX, VARIABLE_PATCHSET)
	PLACEHOLDER_REPO              = fmt.Sprintf(VARIABLE_SYNTAX, VARIABLE_REPO)
	PLACEHOLDER_REVISION          = fmt.Sprintf(VARIABLE_SYNTAX, VARIABLE_REVISION)
	PLACEHOLDER_TASK_NAME         = fmt.Sprintf(VARIABLE_SYNTAX, VARIABLE_TASK_NAME)
	PLACEHOLDER_ISOLATED_OUTDIR   = "${ISOLATED_OUTDIR}"
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
	// CipdPackages are CIPD packages which should be installed for the task.
	CipdPackages []*CipdPackage `json:"cipd_packages,omitempty"`

	// Dependencies are names of other TaskSpecs for tasks which need to run
	// before this task.
	Dependencies []string `json:"dependencies,omitempty"`

	// Dimensions are Swarming bot dimensions which describe the type of bot
	// which may run this task.
	Dimensions []string `json:"dimensions"`

	// Environment is a set of environment variables needed by the task.
	Environment map[string]string `json:"environment,omitempty"`

	// ExecutionTimeout is the maximum amount of time the task is allowed
	// to take.
	ExecutionTimeout time.Duration `json:"execution_timeout_ns,omitempty"`

	// Expiration is how long the task may remain in the pending state
	// before it is abandoned.
	Expiration time.Duration `json:"expiration_ns,omitempty"`

	// ExtraArgs are extra command-line arguments to pass to the task.
	ExtraArgs []string `json:"extra_args,omitempty"`

	// IoTimeout is the maximum amount of time which the task may take to
	// communicate with the server.
	IoTimeout time.Duration `json:"io_timeout_ns,omitempty"`

	// Isolate is the name of the isolate file used by this task.
	Isolate string `json:"isolate"`

	// MaxAttempts is the maximum number of attempts for this TaskSpec. If
	// zero, DEFAULT_TASK_SPEC_MAX_ATTEMPTS is used.
	MaxAttempts int `json:"max_attempts,omitempty"`

	// Priority indicates the relative priority of the task, with 0 < p <= 1
	Priority float64 `json:"priority"`
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
	var cipdPackages []*CipdPackage
	if len(t.CipdPackages) > 0 {
		cipdPackages = make([]*CipdPackage, 0, len(t.CipdPackages))
		pkgs := make([]CipdPackage, len(t.CipdPackages))
		for i, p := range t.CipdPackages {
			pkgs[i] = *p
			cipdPackages = append(cipdPackages, &pkgs[i])
		}
	}
	deps := util.CopyStringSlice(t.Dependencies)
	dims := util.CopyStringSlice(t.Dimensions)
	environment := util.CopyStringMap(t.Environment)
	extraArgs := util.CopyStringSlice(t.ExtraArgs)
	return &TaskSpec{
		CipdPackages:     cipdPackages,
		Dependencies:     deps,
		Dimensions:       dims,
		Environment:      environment,
		ExecutionTimeout: t.ExecutionTimeout,
		Expiration:       t.Expiration,
		ExtraArgs:        extraArgs,
		IoTimeout:        t.IoTimeout,
		Isolate:          t.Isolate,
		MaxAttempts:      t.MaxAttempts,
		Priority:         t.Priority,
	}
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
	Priority  float64  `json:"priority"`
	TaskSpecs []string `json:"tasks"`
	Trigger   string   `json:"trigger,omitempty"`
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
	cache         map[db.RepoState]*cacheEntry
	depotToolsDir string
	file          string
	mtx           sync.RWMutex
	// protected by mtx
	addedTasksCache map[db.RepoState]util.StringSet
	recentCommits   map[string]time.Time
	recentJobSpecs  map[string]time.Time
	recentMtx       sync.RWMutex
	recentTaskSpecs map[string]time.Time
	repos           repograph.Map
	queue           chan func(int)
	workdir         string
}

// gobTaskCfgCache is a struct used for (de)serializing TaskCfgCache instance.
type gobTaskCfgCache struct {
	AddedTasksCache map[db.RepoState]util.StringSet
	Cache           map[db.RepoState]*cacheEntry
	RecentCommits   map[string]time.Time
	RecentJobSpecs  map[string]time.Time
	RecentTaskSpecs map[string]time.Time
}

// NewTaskCfgCache returns a TaskCfgCache instance.
func NewTaskCfgCache(repos repograph.Map, depotToolsDir, workdir string, numWorkers int) (*TaskCfgCache, error) {
	file := path.Join(workdir, "taskCfgCache.gob")
	c := &TaskCfgCache{
		depotToolsDir: depotToolsDir,
		file:          file,
		queue:         make(chan func(int)),
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
		c.cache = gobCache.Cache
		c.recentCommits = gobCache.RecentCommits
		c.recentJobSpecs = gobCache.RecentJobSpecs
		c.recentTaskSpecs = gobCache.RecentTaskSpecs
		for _, e := range c.cache {
			e.c = c
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("Failed to read cache file: %s", err)
	} else {
		c.cache = map[db.RepoState]*cacheEntry{}
		c.addedTasksCache = map[db.RepoState]util.StringSet{}
		c.recentCommits = map[string]time.Time{}
		c.recentJobSpecs = map[string]time.Time{}
		c.recentTaskSpecs = map[string]time.Time{}
	}
	for i := 0; i < numWorkers; i++ {
		go func(i int) {
			for f := range c.queue {
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
	c   *TaskCfgCache
	Cfg *TasksCfg
	Err string
	mtx sync.Mutex
	Rs  db.RepoState
}

func (e *cacheEntry) Get() (*TasksCfg, error) {
	e.mtx.Lock()
	defer e.mtx.Unlock()
	if e.Cfg != nil {
		return e.Cfg, nil
	}
	if e.Err != "" {
		return nil, fmt.Errorf(e.Err)
	}

	// We haven't seen this RepoState before, or it's scrolled out of our
	// window. Read it.
	// Point the upstream to a local source of truth to eliminate network
	// latency.
	r, ok := e.c.repos[e.Rs.Repo]
	if !ok {
		return nil, fmt.Errorf("Unknown repo %q", e.Rs.Repo)
	}
	var cfg *TasksCfg
	if err := e.c.TempGitRepo(e.Rs, e.Rs.IsTryJob(), func(checkout *git.TempCheckout) error {
		var err error
		cfg, err = ReadTasksCfg(checkout.Dir())
		if err != nil {
			// The tasks.cfg file may not exist for a particular commit.
			if strings.Contains(err.Error(), "does not exist in") || strings.Contains(err.Error(), "exists on disk, but not in") || strings.Contains(err.Error(), "no such file or directory") {
				// In this case, use an empty config.
				cfg = &TasksCfg{
					Tasks: map[string]*TaskSpec{},
				}
			} else {
				return err
			}
		}
		return nil
	}); err != nil {
		if strings.Contains(err.Error(), "error: Failed to merge in the changes.") {
			e.Err = err.Error()
		}
		return nil, err
	}
	e.Cfg = cfg

	// Write the commit and task specs into the recent lists.
	// TODO(borenet): The below should probably go elsewhere.
	e.c.recentMtx.Lock()
	defer e.c.recentMtx.Unlock()
	d := r.Get(e.Rs.Revision)
	if d == nil {
		return nil, fmt.Errorf("Unknown revision %s in %s", e.Rs.Revision, e.Rs.Repo)
	}
	ts := d.Timestamp
	if ts.After(e.c.recentCommits[e.Rs.Revision]) {
		e.c.recentCommits[e.Rs.Revision] = ts
	}
	for name, _ := range cfg.Tasks {
		if ts.After(e.c.recentTaskSpecs[name]) {
			e.c.recentTaskSpecs[name] = ts
		}
	}
	for name, _ := range cfg.Jobs {
		if ts.After(e.c.recentJobSpecs[name]) {
			e.c.recentJobSpecs[name] = ts
		}
	}
	e.c.mtx.Lock()
	defer e.c.mtx.Unlock()
	return cfg, e.c.write()
}

func (c *TaskCfgCache) getEntry(rs db.RepoState) *cacheEntry {
	rv, ok := c.cache[rs]
	if !ok {
		rv = &cacheEntry{
			c:  c,
			Rs: rs,
		}
		c.cache[rs] = rv
	}
	return rv
}

// ReadTasksCfg reads the task cfg file from the given RepoState and returns it.
// Stores a cache of already-read task cfg files. Syncs the repo and reads the
// file if needed.
func (c *TaskCfgCache) ReadTasksCfg(rs db.RepoState) (*TasksCfg, error) {
	c.mtx.Lock()
	entry := c.getEntry(rs)
	c.mtx.Unlock()
	return entry.Get()
}

// GetTaskSpecsForRepoStates returns a set of TaskSpecs for each of the
// given set of RepoStates, keyed by RepoState and TaskSpec name.
func (c *TaskCfgCache) GetTaskSpecsForRepoStates(rs []db.RepoState) (map[db.RepoState]map[string]*TaskSpec, error) {
	c.mtx.Lock()
	entries := make(map[db.RepoState]*cacheEntry, len(rs))
	for _, s := range rs {
		entries[s] = c.getEntry(s)
	}
	c.mtx.Unlock()

	var m sync.Mutex
	var wg sync.WaitGroup
	rv := make(map[db.RepoState]map[string]*TaskSpec, len(rs))
	errs := []error{}
	for s, entry := range entries {
		wg.Add(1)
		go func(s db.RepoState, entry *cacheEntry) {
			defer wg.Done()
			cfg, err := entry.Get()
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
		}(s, entry)
	}
	wg.Wait()
	if len(errs) > 0 {
		return nil, fmt.Errorf("Errors loading task cfgs: %v", errs)
	}
	return rv, nil
}

// GetTaskSpec returns the TaskSpec at the given RepoState, or an error if no
// such TaskSpec exists.
func (c *TaskCfgCache) GetTaskSpec(rs db.RepoState, name string) (*TaskSpec, error) {
	cfg, err := c.ReadTasksCfg(rs)
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
func (c *TaskCfgCache) GetAddedTaskSpecsForRepoStates(rss []db.RepoState) (map[db.RepoState]util.StringSet, error) {
	rv := make(map[db.RepoState]util.StringSet, len(rss))
	// todoParents collects the RepoStates in rss that are not in
	// c.addedTasksCache. We also save the RepoStates' parents so we don't
	// have to recompute them later.
	todoParents := make(map[db.RepoState][]db.RepoState, 0)
	// allTodoRs collects the RepoStates for which we need to look up
	// TaskSpecs.
	allTodoRs := []db.RepoState{}
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
	taskSpecs, err := c.GetTaskSpecsForRepoStates(allTodoRs)
	if err != nil {
		return nil, err
	}
	c.mtx.Lock()
	defer c.mtx.Unlock()
	for cur, parents := range todoParents {
		addedTasks := util.NewStringSet()
		for task, _ := range taskSpecs[cur] {
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
func (c *TaskCfgCache) GetJobSpec(rs db.RepoState, name string) (*JobSpec, error) {
	cfg, err := c.ReadTasksCfg(rs)
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
func (c *TaskCfgCache) MakeJob(rs db.RepoState, name string) (*db.Job, error) {
	cfg, err := c.ReadTasksCfg(rs)
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

	return &db.Job{
		Created:      time.Now(),
		Dependencies: deps,
		Name:         name,
		Priority:     spec.Priority,
		RepoState:    rs,
		Tasks:        map[string][]*db.TaskSummary{},
	}, nil
}

// Cleanup removes cache entries which are outside of our scheduling window.
func (c *TaskCfgCache) Cleanup(period time.Duration) error {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	periodStart := time.Now().Add(-period)
	for repoState, _ := range c.cache {
		details, err := repoState.GetCommit(c.repos)
		if err != nil || details.Timestamp.Before(periodStart) {
			delete(c.cache, repoState)
		}
	}
	for repoState, _ := range c.addedTasksCache {
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
	f, err := os.Create(c.file)
	if err != nil {
		return err
	}
	gobCache := gobTaskCfgCache{
		AddedTasksCache: c.addedTasksCache,
		Cache:           c.cache,
		RecentCommits:   c.recentCommits,
		RecentJobSpecs:  c.recentJobSpecs,
		RecentTaskSpecs: c.recentTaskSpecs,
	}
	if err := gob.NewEncoder(f).Encode(&gobCache); err != nil {
		util.Close(f)
		return err
	}
	return f.Close()
}

func stringMapKeys(m map[string]time.Time) []string {
	rv := make([]string, 0, len(m))
	for k, _ := range m {
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
func (c *TaskCfgCache) TempGitRepo(rs db.RepoState, botUpdate bool, fn func(*git.TempCheckout) error) error {
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
			gr, err = tempGitRepoBotUpdate(rs, c.depotToolsDir, cacheDir, tmp)
		} else {
			repo, ok := c.repos[rs.Repo]
			if !ok {
				rvErr <- fmt.Errorf("Unknown repo: %s", rs.Repo)
				return
			}
			gr, err = tempGitRepo(repo.Repo(), rs)
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
func tempGitRepo(repo *git.Repo, rs db.RepoState) (rv *git.TempCheckout, rvErr error) {
	if rs.IsTryJob() {
		return nil, fmt.Errorf("specs.tempGitRepo does not apply patches, and should not be called for try jobs.")
	}

	c, err := repo.TempCheckout()
	if err != nil {
		return nil, err
	}

	defer func() {
		if rvErr != nil {
			c.Delete()
		}
	}()

	// Check out the correct commit.
	if _, err := c.Git("checkout", rs.Revision); err != nil {
		return nil, err
	}

	return c, nil
}

// tempGitRepoBotUpdate creates a git repository in a temporary directory, gets it into
// the given RepoState, and returns its location.
func tempGitRepoBotUpdate(rs db.RepoState, depotToolsDir, gitCacheDir, tmp string) (*git.TempCheckout, error) {
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
		"--spec", spec,
		"--patch_root", patchRepoName,
		"--revision_mapping_file", revisionMappingFile,
		"--git-cache-dir", gitCacheDir,
		"--output_json", outputJson,
		"--revision", fmt.Sprintf("%s@%s", projectName, rs.Revision),
		"--output_manifest",
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
				"--gerrit_repo", patchRepo,
				"--gerrit_ref", gerritRef,
			}...)
		}
	}
	if _, err := exec.RunCommand(&exec.Command{
		Name: cmd[0],
		Args: cmd[1:],
		Dir:  tmp,
		Env: []string{
			fmt.Sprintf("PATH=%s:%s", depotToolsDir, os.Getenv("PATH")),
		},
		InheritEnv: true,
	}); err != nil {
		return nil, err
	}

	// bot_update points the upstream to a local cache. Point back to the
	// "real" upstream, in case the caller cares about the remote URL. Note
	// that this doesn't change the remote URLs for the DEPS.
	co := &git.TempCheckout{
		GitDir: git.GitDir(path.Join(tmp, projectName)),
	}
	if _, err := co.Git("remote", "set-url", "origin", rs.Repo); err != nil {
		return nil, err
	}

	// Self-check.
	head, err := co.RevParse("HEAD")
	if err != nil {
		return nil, err
	}
	if head != rs.Revision {
		return nil, fmt.Errorf("TempGitRepo ended up at the wrong revision. Wanted %q but got %q", rs.Revision, head)
	}

	return co, nil
}
