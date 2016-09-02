package scheduling

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.skia.org/infra/go/gitinfo"
)

const (
	TASKS_CFG_FILE = "infra/bots/tasks.json"
)

// ParseTasksCfg parses the given task cfg file contents and returns the config.
func ParseTasksCfg(contents string) (*TasksCfg, error) {
	var rv TasksCfg
	if err := json.Unmarshal([]byte(contents), &rv); err != nil {
		return nil, fmt.Errorf("Failed to read tasks cfg: could not parse file: %s", err)
	}

	for _, t := range rv.Tasks {
		if err := t.Validate(&rv); err != nil {
			return nil, err
		}
	}

	if err := findCycles(rv.Tasks); err != nil {
		return nil, err
	}

	return &rv, nil
}

// ReadTasksCfg reads the task cfg file from the given repo and returns it.
func ReadTasksCfg(repo *gitinfo.GitInfo, commit string) (*TasksCfg, error) {
	contents, err := repo.GetFile(TASKS_CFG_FILE, commit)
	if err != nil {
		return nil, fmt.Errorf("Failed to read tasks cfg: could not read file: %s", err)
	}
	return ParseTasksCfg(contents)
}

// TasksCfg is a struct which describes all Swarming tasks for a repo at a
// particular commit.
type TasksCfg struct {
	// Tasks is a map whose keys are TaskSpec names and values are TaskSpecs
	// detailing the Swarming tasks to run at each commit.
	Tasks map[string]*TaskSpec `json:"tasks"`
}

// TaskSpec is a struct which describes a Swarming task to run.
type TaskSpec struct {
	// CipdPackages are CIPD packages which should be installed for the task.
	CipdPackages []*CipdPackage `json:"cipd_packages"`

	// Dependencies are names of other TaskSpecs for tasks which need to run
	// before this task.
	Dependencies []string `json:"dependencies"`

	// Dimensions are Swarming bot dimensions which describe the type of bot
	// which may run this task.
	Dimensions []string `json:"dimensions"`

	// Environment is a set of environment variables needed by the task.
	Environment map[string]string `json:"environment"`

	// ExtraArgs are extra command-line arguments to pass to the task.
	ExtraArgs []string `json:"extra_args"`

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
	cipdPackages := make([]*CipdPackage, 0, len(t.CipdPackages))
	pkgs := make([]CipdPackage, len(t.CipdPackages))
	for i, p := range t.CipdPackages {
		pkgs[i] = *p
		cipdPackages = append(cipdPackages, &pkgs[i])
	}
	deps := make([]string, len(t.Dependencies))
	copy(deps, t.Dependencies)
	dims := make([]string, len(t.Dimensions))
	copy(dims, t.Dimensions)
	return &TaskSpec{
		CipdPackages: cipdPackages,
		Dependencies: deps,
		Dimensions:   dims,
		Isolate:      t.Isolate,
		Priority:     t.Priority,
	}
}

// CipdPackage is a struct representing a CIPD package which needs to be
// installed on a bot for a particular task.
type CipdPackage struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Version int64  `json:"version"`
}

// taskCfgCache is a struct used for caching tasks cfg files. The user should
// periodically call Cleanup() to remove old entries.
type taskCfgCache struct {
	cache map[string]map[string]*TasksCfg
	mtx   sync.Mutex
	repos *gitinfo.RepoMap
}

// newTaskCfgCache returns a taskCfgCache instance.
func newTaskCfgCache(repos *gitinfo.RepoMap) *taskCfgCache {
	return &taskCfgCache{
		cache: map[string]map[string]*TasksCfg{},
		mtx:   sync.Mutex{},
		repos: repos,
	}
}

// readTasksCfg reads the task cfg file from the given repo and returns it.
// Stores a cache of already-read task cfg files. Syncs the repo and reads the
// file if needed. Assumes the caller holds a lock.
func (c *taskCfgCache) readTasksCfg(repo, commit string) (*TasksCfg, error) {
	r, err := c.repos.Repo(repo)
	if err != nil {
		return nil, fmt.Errorf("Could not read task cfg; failed to check out repo: %s", err)
	}

	if _, ok := c.cache[repo]; !ok {
		c.cache[repo] = map[string]*TasksCfg{}
	}
	if _, ok := c.cache[repo][commit]; !ok {
		cfg, err := ReadTasksCfg(r, commit)
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
		c.cache[repo][commit] = cfg
	}
	return c.cache[repo][commit], nil
}

// GetTaskSpecsForCommits returns a set of TaskSpecs for each of the
// given set of commits, in the form of nested maps:
//
// map[repo_name][commit_hash][task_name]*TaskSpec
//
func (c *taskCfgCache) GetTaskSpecsForCommits(commitsByRepo map[string][]string) (map[string]map[string]map[string]*TaskSpec, error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	rv := make(map[string]map[string]map[string]*TaskSpec, len(commitsByRepo))
	for repo, commits := range commitsByRepo {
		tasksByCommit := make(map[string]map[string]*TaskSpec, len(commits))
		for _, commit := range commits {
			cfg, err := c.readTasksCfg(repo, commit)
			if err != nil {
				return nil, err
			}
			// Make a copy of the task specs.
			tasks := make(map[string]*TaskSpec, len(cfg.Tasks))
			for name, task := range cfg.Tasks {
				tasks[name] = task.Copy()
			}
			tasksByCommit[commit] = tasks
		}
		rv[repo] = tasksByCommit
	}
	return rv, nil
}

// Cleanup removes cache entries which are outside of our scheduling window.
func (c *taskCfgCache) Cleanup(period time.Duration) error {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	periodStart := time.Now().Add(-period)
	for repoName, taskCfgsByCommit := range c.cache {
		repo, err := c.repos.Repo(repoName)
		if err != nil {
			return err
		}
		remove := []string{}
		for commit, _ := range taskCfgsByCommit {
			details, err := repo.Details(commit, false)
			if err != nil {
				return err
			}
			if details.Timestamp.Before(periodStart) {
				remove = append(remove, commit)
			}
		}
		for _, commit := range remove {
			delete(c.cache[repoName], commit)
		}
	}
	return nil
}

// findCycles searches for cyclical dependencies in the task specs and returns
// an error if any are found.
func findCycles(tasks map[string]*TaskSpec) error {
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

	// Perform a depth-first search of the graph.
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

	for _, v := range vertices {
		if !v.visited {
			if err := visit(v); err != nil {
				return err
			}
		}
	}
	return nil
}
