package specs

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/skia-dev/glog"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gitinfo"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/db"
)

const (
	TASKS_CFG_FILE = "infra/bots/tasks.json"

	VARIABLE_SYNTAX = "<(%s)"

	VARIABLE_REPO      = "REPO"
	VARIABLE_REVISION  = "REVISION"
	VARIABLE_TASK_NAME = "TASK_NAME"
)

var (
	PLACEHOLDER_REPO            = fmt.Sprintf(VARIABLE_SYNTAX, VARIABLE_REPO)
	PLACEHOLDER_REVISION        = fmt.Sprintf(VARIABLE_SYNTAX, VARIABLE_REVISION)
	PLACEHOLDER_TASK_NAME       = fmt.Sprintf(VARIABLE_SYNTAX, VARIABLE_TASK_NAME)
	PLACEHOLDER_ISOLATED_OUTDIR = "${ISOLATED_OUTDIR}"
)

// ParseTasksCfg parses the given task cfg file contents and returns the config.
func ParseTasksCfg(contents string) (*TasksCfg, error) {
	var rv TasksCfg
	if err := json.Unmarshal([]byte(contents), &rv); err != nil {
		return nil, fmt.Errorf("Failed to read tasks cfg: could not parse file: %s", err)
	}
	if err := rv.Validate(); err != nil {
		return nil, err
	}

	return &rv, nil
}

// ReadTasksCfg reads the task cfg file from the given dir and returns it.
func ReadTasksCfg(repoDir string) (*TasksCfg, error) {
	contents, err := ioutil.ReadFile(path.Join(repoDir, TASKS_CFG_FILE))
	if err != nil {
		return nil, fmt.Errorf("Failed to read tasks cfg: could not read file: %s", err)
	}
	return ParseTasksCfg(string(contents))
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

	// ExtraArgs are extra command-line arguments to pass to the task.
	ExtraArgs []string `json:"extra_args,omitempty"`

	// Isolate is the name of the isolate file used by this task.
	Isolate string `json:"isolate"`

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
		CipdPackages: cipdPackages,
		Dependencies: deps,
		Dimensions:   dims,
		Environment:  environment,
		ExtraArgs:    extraArgs,
		Isolate:      t.Isolate,
		Priority:     t.Priority,
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
	}
}

// TaskCfgCache is a struct used for caching tasks cfg files. The user should
// periodically call Cleanup() to remove old entries.
type TaskCfgCache struct {
	cache           map[db.RepoState]*TasksCfg
	mtx             sync.Mutex
	recentCommits   map[string]time.Time
	recentJobSpecs  map[string]time.Time
	recentMtx       sync.RWMutex
	recentTaskSpecs map[string]time.Time
	repos           *gitinfo.RepoMap
	workdir         string
}

// NewTaskCfgCache returns a TaskCfgCache instance.
func NewTaskCfgCache(workdir string, repos *gitinfo.RepoMap) *TaskCfgCache {
	return &TaskCfgCache{
		cache:           map[db.RepoState]*TasksCfg{},
		mtx:             sync.Mutex{},
		recentCommits:   map[string]time.Time{},
		recentJobSpecs:  map[string]time.Time{},
		recentMtx:       sync.RWMutex{},
		recentTaskSpecs: map[string]time.Time{},
		repos:           repos,
		workdir:         workdir,
	}
}

// readTasksCfg reads the task cfg file from the given repo and returns it.
// Stores a cache of already-read task cfg files. Syncs the repo and reads the
// file if needed. Assumes the caller holds c.mtx.
func (c *TaskCfgCache) readTasksCfg(rs db.RepoState) (*TasksCfg, error) {
	rv, ok := c.cache[rs]
	if ok {
		return rv, nil
	}

	// We haven't seen this RepoState before, or it's scrolled our of our
	// window. Read it.
	// Point the upstream to a local source of truth to eliminate network
	// latency.
	r, err := c.repos.Repo(rs.Repo)
	if err != nil {
		return nil, err
	}
	rsCpy := rs.Copy()
	rsCpy.Repo = r.Dir()
	repoDir, err := TempGitRepo(c.workdir, rsCpy)
	if err != nil {
		return nil, fmt.Errorf("Could not read task cfg; failed to check out RepoState %q: %s", rs, err)
	}
	defer util.RemoveAll(repoDir)
	cfg, err := ReadTasksCfg(repoDir)
	if err != nil {
		// The tasks.cfg file may not exist for a particular commit.
		if strings.Contains(err.Error(), "does not exist in") || strings.Contains(err.Error(), "exists on disk, but not in") {
			// In this case, use an empty config.
			cfg = &TasksCfg{
				Tasks: map[string]*TaskSpec{},
			}
		} else {
			return nil, err
		}
	}
	c.cache[rs] = cfg

	// Write the commit and task specs into the recent lists.
	c.recentMtx.Lock()
	defer c.recentMtx.Unlock()
	d, err := r.Details(rs.Revision, false)
	if err != nil {
		return nil, err
	}
	ts := d.Timestamp
	if ts.After(c.recentCommits[rs.Revision]) {
		c.recentCommits[rs.Revision] = ts
	}
	for name, _ := range cfg.Tasks {
		if ts.After(c.recentTaskSpecs[name]) {
			c.recentTaskSpecs[name] = ts
		}
	}
	for name, _ := range cfg.Jobs {
		if ts.After(c.recentJobSpecs[name]) {
			c.recentJobSpecs[name] = ts
		}
	}
	return cfg, nil
}

// ReadTasksCfg reads the task cfg file from the given RepoState and returns it.
// Stores a cache of already-read task cfg files. Syncs the repo and reads the
// file if needed.
func (c *TaskCfgCache) ReadTasksCfg(rs db.RepoState) (*TasksCfg, error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	return c.readTasksCfg(rs)
}

// GetTaskSpecsForRepoStates returns a set of TaskSpecs for each of the
// given set of RepoStates, keyed by RepoState and TaskSpec name.
func (c *TaskCfgCache) GetTaskSpecsForRepoStates(rs []db.RepoState) (map[db.RepoState]map[string]*TaskSpec, error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	rv := map[db.RepoState]map[string]*TaskSpec{}
	for _, s := range rs {
		cfg, err := c.readTasksCfg(s)
		if err != nil {
			return nil, err
		}
		// Make a copy of the task specs.
		subMap := make(map[string]*TaskSpec, len(cfg.Tasks))
		for name, task := range cfg.Tasks {
			subMap[name] = task.Copy()
		}
		rv[s] = subMap
	}
	return rv, nil
}

// GetTaskSpec returns the TaskSpec at the given RepoState, or an error if no
// such TaskSpec exists.
func (c *TaskCfgCache) GetTaskSpec(rs db.RepoState, name string) (*TaskSpec, error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	cfg, err := c.readTasksCfg(rs)
	if err != nil {
		return nil, err
	}
	t, ok := cfg.Tasks[name]
	if !ok {
		return nil, fmt.Errorf("No such task spec: %s @ %s", name, rs)
	}
	return t.Copy(), nil
}

// GetJobSpec returns the JobSpec at the given RepoState, or an error if no such
// JobSpec exists.
func (c *TaskCfgCache) GetJobSpec(rs db.RepoState, name string) (*JobSpec, error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	cfg, err := c.readTasksCfg(rs)
	if err != nil {
		return nil, err
	}
	j, ok := cfg.Jobs[name]
	if !ok {
		return nil, fmt.Errorf("No such job spec: %s @ %s", name, rs)
	}
	return j.Copy(), nil
}

// Cleanup removes cache entries which are outside of our scheduling window.
func (c *TaskCfgCache) Cleanup(period time.Duration) error {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	periodStart := time.Now().Add(-period)
	for repoState, _ := range c.cache {
		repo, err := c.repos.Repo(repoState.Repo)
		if err != nil {
			return err
		}
		details, err := repo.Details(repoState.Revision, false)
		if err != nil {
			return err
		}
		if details.Timestamp.Before(periodStart) {
			delete(c.cache, repoState)
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
// the given RepoState, and returns its location.
func TempGitRepo(workdir string, rs db.RepoState) (rv string, rvErr error) {
	// Create a temporary dir for the git checkout.
	if err := os.MkdirAll(workdir, os.ModePerm); err != nil {
		if !os.IsExist(err) {
			return "", err
		}
	}

	tmp, err := ioutil.TempDir(workdir, fmt.Sprintf("tmp_%s", path.Base(rs.Repo)))
	if err != nil {
		return "", err
	}
	defer func() {
		if rvErr != nil {
			if err := os.RemoveAll(tmp); err != nil {
				glog.Errorf("Failed to remove %s: %s", tmp, err)
			}
		}
	}()

	subdir := strings.TrimSuffix(path.Base(rs.Repo), ".git")
	d := path.Join(tmp, subdir)
	if err := os.MkdirAll(d, os.ModePerm); err != nil {
		if !os.IsExist(err) {
			return "", err
		}
	}

	// Obtain the git checkout.
	if _, err := exec.RunCwd(d, "git", "clone", rs.Repo, "."); err != nil {
		return "", err
	}

	// Check out the correct commit.
	glog.Infof("Checking out %s", rs.Revision)
	if _, err := exec.RunCwd(d, "git", "checkout", rs.Revision); err != nil {
		return "", err
	}

	// Write a dummy .gclient file in the parent of the checkout.
	if err := ioutil.WriteFile(path.Join(d, "..", ".gclient"), []byte(""), os.ModePerm); err != nil {
		return "", err
	}

	// Apply a patch if necessary.
	if rs.IsTryJob() {
		if _, err := exec.RunCwd(d, "git", "checkout", "-b", "patch"); err != nil {
			return "", err
		}
		server := strings.TrimRight(rs.Server, "/")
		if strings.Contains(rs.Server, "codereview.chromium") {
			patchUrl := fmt.Sprintf("%s/%s/#ps%s", server, rs.Issue, rs.Patchset)
			if _, err := exec.RunCwd(d, "git", "cl", "patch", "--rietveld", "--no-commit", patchUrl); err != nil {
				return "", err
			}
		} else {
			patchUrl := fmt.Sprintf("%s/c/%s/%s", server, rs.Issue, rs.Patchset)
			if _, err := exec.RunCwd(d, "git", "cl", "patch", "--gerrit", patchUrl); err != nil {
				return "", err
			}
			// Un-commit the applied patch.
			if _, err := exec.RunCwd(d, "git", "reset", "HEAD^"); err != nil {
				return "", err
			}
		}
	}

	return d, nil
}
