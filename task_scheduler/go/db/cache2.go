package db

import (
	"fmt"
	"sort"
	"time"

	"go.skia.org/infra/task_scheduler/go/window"
)

type entry Task

func (e *entry) Repository() (string, error) {
	return (*Task)(e).Repo, nil
}

func (e *entry) Timestamp() (time.Time, error) {
	return (*Task)(e).Created, nil
}

type taskCacheById struct {
	tasks map[string]*Task
}

func (c *taskCacheById) Insert(task window.Entry) error {
	t, ok := task.(*entry)
	if !ok {
		return fmt.Errorf("taskCacheById only holds Tasks")
	}
	c.tasks[t.Id] = (*Task)(t)
	return nil
}

func (c *taskCacheById) Evict(task window.Entry) error {
	t, ok := task.(*entry)
	if !ok {
		return fmt.Errorf("taskCacheById only holds Tasks")
	}
	delete(c.tasks, t.Id)
	return nil
}

func (c *taskCacheById) AllEntries() []window.Entry {
	rv := make([]window.Entry, 0, len(c.tasks))
	for _, t := range c.tasks {
		rv = append(rv, (*entry)(t))
	}
	return rv
}

func (c *taskCacheById) get(id string) *Task {
	return c.tasks[id]
}

type taskCacheByCommit struct {
	// map[repo_name][commit_hash][task_spec_name]*Task
	tasksByCommit map[string]map[string]map[string]*Task
}

func (c *taskCacheByCommit) Insert(task window.Entry) error {
	t, ok := task.(*entry)
	if !ok {
		return fmt.Errorf("taskCacheByCommit only holds Tasks")
	}
	commitMap, ok := c.tasksByCommit[t.Repo]
	if !ok {
		commitMap = map[string]map[string]*Task{}
		c.tasksByCommit[t.Repo] = commitMap
	}
	for _, commit := range t.Commits {
		if _, ok := commitMap[commit]; !ok {
			commitMap[commit] = map[string]*Task{}
		}
		commitMap[commit][t.Name] = (*Task)(t)
	}
	return nil
}

func (c *taskCacheByCommit) Evict(task window.Entry) error {
	t, ok := task.(*entry)
	if !ok {
		return fmt.Errorf("taskCacheByCommit only holds Tasks")
	}
	if commitMap, ok := c.tasksByCommit[t.Repo]; ok {
		for _, commit := range t.Commits {
			if other, ok := commitMap[commit][t.Name]; ok && other.Id == t.Id {
				delete(commitMap[commit], t.Name)
				if len(commitMap[commit]) == 0 {
					delete(commitMap, commit)
				}
			}
		}
	}
	return nil
}

func (c *taskCacheByCommit) AllEntries() []window.Entry {
	byId := map[string]window.Entry{}
	for _, m1 := range c.tasksByCommit {
		for _, m2 := range m1 {
			for _, t := range m2 {
				byId[t.Id] = (*entry)(t)
			}
		}
	}
	rv := make([]window.Entry, 0, len(byId))
	for _, e := range byId {
		rv = append(rv, e)
	}
	return rv
}

func (c *taskCacheByCommit) getTasksForCommits(repo string, commits []string) (map[string]map[string]*Task, error) {
	rv := make(map[string]map[string]*Task, len(commits))
	commitMap := c.tasksByCommit[repo]
	for _, commit := range commits {
		if tasks, ok := commitMap[commit]; ok {
			rv[commit] = make(map[string]*Task, len(tasks))
			for k, v := range tasks {
				rv[commit][k] = v.Copy()
			}
		} else {
			rv[commit] = map[string]*Task{}
		}
	}
	return rv, nil
}

func (c *taskCacheByCommit) getTaskForCommit(repo, commit, name string) (*Task, error) {
	commitMap, ok := c.tasksByCommit[repo]
	if !ok {
		return nil, nil
	}
	if tasks, ok := commitMap[commit]; ok {
		if t, ok := tasks[name]; ok {
			return t.Copy(), nil
		}
	}
	return nil, nil
}

type taskCacheByKey struct {
	// map[TaskKey]map[task_id]*Task
	tasksByKey map[TaskKey]map[string]*Task
}

func (c *taskCacheByKey) Insert(task window.Entry) error {
	t, ok := task.(*entry)
	if !ok {
		return fmt.Errorf("taskCacheById only holds Tasks")
	}
	byKey, ok := c.tasksByKey[t.TaskKey]
	if !ok {
		byKey = map[string]*Task{}
		c.tasksByKey[t.TaskKey] = byKey
	}
	byKey[t.Id] = (*Task)(t)
	return nil
}

func (c *taskCacheByKey) Evict(task window.Entry) error {
	t, ok := task.(*entry)
	if !ok {
		return fmt.Errorf("taskCacheById only holds Tasks")
	}
	byKey, ok := c.tasksByKey[t.TaskKey]
	if ok {
		delete(byKey, t.Id)
		if len(byKey) == 0 {
			delete(c.tasksByKey, t.TaskKey)
		}
	}
	return nil
}

func (c *taskCacheByKey) AllEntries() []window.Entry {
	rv := make([]window.Entry, 0, len(c.tasksByKey))
	for _, m := range c.tasksByKey {
		for _, t := range m {
			rv = append(rv, (*entry)(t))
		}
	}
	return rv
}

func (c *taskCacheByKey) getTasksByKey(k *TaskKey) ([]*Task, error) {
	tasks := c.tasksByKey[*k]
	rv := make([]*Task, 0, len(tasks))
	for _, t := range tasks {
		rv = append(rv, t.Copy())
	}
	sort.Sort(TaskSlice(rv))
	return rv, nil
}

type taskCacheByTime struct {
	tasksByTime map[string]*Task
}

func (c *taskCacheByTime) Insert(task window.Entry) error {
	t, ok := task.(*entry)
	if !ok {
		return fmt.Errorf("taskCacheById only holds Tasks")
	}
	c.tasks[t.Id] = (*Task)(t)
	return nil
}

func (c *taskCacheByTime) Evict(task window.Entry) error {
	t, ok := task.(*entry)
	if !ok {
		return fmt.Errorf("taskCacheById only holds Tasks")
	}
	delete(c.tasks, t.Id)
	return nil
}

func (c *taskCacheByTime) AllEntries() []window.Entry {
	rv := make([]window.Entry, 0, len(c.tasks))
	for _, t := range c.tasks {
		rv = append(rv, (*entry)(t))
	}
	return rv
}

func (c *taskCacheByTime) get(id string) *Task {
	return c.tasks[id]
}

type TaskCache2 struct {
	byId     *taskCacheById
	byCommit *taskCacheByCommit
	byKey    *taskCacheByKey
	wc       *window.WindowCache
}

func NewTaskCache2(w *window.Window) *TaskCache2 {
	byId := &taskCacheById{
		tasks: map[string]*Task{},
	}
	byCommit := &taskCacheByCommit{
		tasksByCommit: map[string]map[string]map[string]*Task{},
	}
	byKey := &taskCacheByKey{
		tasksByKey: map[TaskKey]map[string]*Task{},
	}
	return &TaskCache2{
		byId:     byId,
		byCommit: byCommit,
		byKey:    byKey,
		wc:       window.NewWindowCache(w, byId, byCommit, byKey),
	}
}

func (c *TaskCache2) ExpireAndUpdate(newTasks []*Task) error {
	inp := make([]window.Entry, 0, len(newTasks))
	for _, t := range newTasks {
		inp = append(inp, (*entry)(t))
	}
	if err := c.wc.Insert(inp); err != nil {
		return err
	}
	return c.wc.Expire()
}

func (c *TaskCache2) GetTask(id string) (*Task, error) {
	c.wc.RLock()
	defer c.wc.RUnlock()
	return c.byId.get(id), nil
}

func (c *TaskCache2) GetTaskForCommit(repo, commit, name string) (*Task, error) {
	c.wc.RLock()
	defer c.wc.RUnlock()
	return c.byCommit.getTaskForCommit(repo, commit, name)
}

func (c *TaskCache2) GetTasksForCommits(repo string, commits []string) (map[string]map[string]*Task, error) {
	c.wc.RLock()
	defer c.wc.RUnlock()
	return c.byCommit.getTasksForCommits(repo, commits)
}

func (c *TaskCache2) GetTasksByKey(k *TaskKey) ([]*Task, error) {
	c.wc.RLock()
	defer c.wc.RUnlock()
	return c.byKey.getTasksByKey(k)
}

var _ TaskCache = &TaskCache2{}
