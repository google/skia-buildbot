package task_scheduler

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"go.skia.org/infra/go/gitinfo"
)

const (
	TASKS_CFG_FILE = "infra/bots/tasks.cfg"
)

// ParseTasksCfg parses the given task cfg file contents and returns the config.
func ParseTasksCfg(contents string) (*TasksCfg, error) {
	var rv TasksCfg
	if err := json.Unmarshal([]byte(contents), &rv); err != nil {
		return nil, fmt.Errorf("Failed to read tasks cfg: could not parse file: %s", err)
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

// TasksCfg is a struct which describes all Swarming tasks for a repo.
type TasksCfg struct {
	// Tasks is a map whose keys are TaskSpec names and values are TaskSpecs
	// detailing the Swarming tasks to run at each commit.
	Tasks map[string]*TaskSpec `json:"tasks"`
}

// TaskSpec is a struct which describes a Swarming task to run.
type TaskSpec struct {
	// CipdPackages are CIPD packages which should be installed for the task.
	CipdPackages []string `json:"cipd_packages"`

	// Dependencies are names of other TaskSpecs for tasks which need to run
	// before this task.
	Dependencies []string `json:"dependencies"`

	// Dimensions are Swarming bot dimensions which describe the type of bot
	// which may run this task.
	Dimensions []string `json:"dimensions"`

	// Isolate is the name of the isolate file used by this task.
	Isolate string `json:"isolate"`

	// Priority indicates the relative priority of the task, with 0 < p <= 1
	Priority float64 `json:"priority"`
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
			return nil, err
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
	rv := map[string]map[string]map[string]*TaskSpec{}
	for repo, commits := range commitsByRepo {
		rv[repo] = map[string]map[string]*TaskSpec{}
		for _, commit := range commits {
			cfg, err := c.readTasksCfg(repo, commit)
			if err != nil {
				return nil, err
			}
			// Make a copy of the task specs.
			var buf bytes.Buffer
			if err := gob.NewEncoder(&buf).Encode(cfg.Tasks); err != nil {
				return nil, err
			}
			var tasks map[string]*TaskSpec
			if err := gob.NewDecoder(&buf).Decode(&tasks); err != nil {
				return nil, err
			}
			rv[repo][commit] = tasks
		}
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
