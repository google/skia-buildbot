package incremental

/*
   Allow incremental updates to the client.
*/

import (
	"context"
	"sort"
	"sync"
	"time"

	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/window"
)

// Task is a trimmed-down version of db.Task for minimizing the amount of data
// we send to the client.
type Task struct {
	Commits  []string      `json:"commits"`
	Name     string        `json:"name"`
	Id       string        `json:"id"`
	Revision string        `json:"revision"`
	Status   db.TaskStatus `json:"status"`
}

// Update represents all of the new information we obtained in a single Update()
// tick. Every time Update() is called on IncrementalCache, a new Update object
// is stored internally. When the client calls any variant of Get, any new
// Updates are found and merged into a single Update object to return.
type Update struct {
	BranchHeads []*gitinfo.GitBranch  `json:"branch_heads,omitempty"`
	Commits     []*vcsinfo.LongCommit `json:"commits,omitempty"`
	StartOver   *bool                 `json:"start_over,omitempty"`
	Tasks       []*Task               `json:"tasks,omitempty"`
	Timestamp   time.Time             `json:"-"`
}

// IncrementalCache is a cache used for sending only new information to a
// client. New data is obtained at each Update() tick and stored internally with
// a timestamp. When the client requests new data, we return a combined set of
// Updates.
type IncrementalCache struct {
	// TODO(borenet): Comments too.
	cc         *commitsCache
	mtx        sync.RWMutex
	numCommits int
	tc         *taskCache
	updates    map[string][]*Update
	w          *window.Window
}

// NewIncrementalCache returns an IncrementalCache instance. It does not
// initialize the cache.
func NewIncrementalCache(d db.TaskReader, w *window.Window, repos repograph.Map, numCommits int) (*IncrementalCache, error) {
	c := &IncrementalCache{
		cc: &commitsCache{
			repos: repos,
		},
		numCommits: numCommits,
		tc: &taskCache{
			db: d,
		},
		w: w,
	}
	return c, c.Update()
}

// getUpdatesInRange is a helper function which retrieves all Update objects
// within a given time range.
func (c *IncrementalCache) getUpdatesInRange(repo string, from, to time.Time) []*Update {
	c.mtx.RLock()
	defer c.mtx.RUnlock()
	from = from.UTC()
	to = to.UTC()
	sklog.Infof("getUpdatesInRange(%s, %s)", from, to)
	// Obtain all updates in the given range.
	updates := []*Update{}
	// TODO(borenet): Could use binary search to get to the starting point
	// faster.
	for _, u := range c.updates[repo] {
		if !u.Timestamp.Before(from) && u.Timestamp.Before(to) {
			updates = append(updates, u)
		}
	}
	sklog.Infof("Found %d updates in range %s - %s", len(updates), from, to)
	for _, u := range updates {
		sklog.Infof("    %s: %d tasks", u.Timestamp, len(u.Tasks))
	}
	return updates
}

// GetRange returns all newly-obtained data in the given time range, trimmed
// to maxCommits.
func (c *IncrementalCache) GetRange(repo string, from, to time.Time, maxCommits int) (*Update, error) {
	sklog.Infof("GetRange(%s, %s)", from, to)
	updates := c.getUpdatesInRange(repo, from, to)
	// Merge the updates.
	rv := &Update{
		BranchHeads: nil,
		Commits:     []*vcsinfo.LongCommit{},
		StartOver:   nil,
		Tasks:       []*Task{},
	}
	for _, u := range updates {
		if u.BranchHeads != nil {
			rv.BranchHeads = u.BranchHeads
		}
		rv.Commits = append(rv.Commits, u.Commits...)
		if u.StartOver != nil && *u.StartOver {
			rv.StartOver = u.StartOver
		}
		rv.Tasks = append(rv.Tasks, u.Tasks...)
	}
	// Limit to only the most recent N commits.
	sort.Sort(vcsinfo.LongCommitSlice(rv.Commits)) // Most recent first.
	if len(rv.Commits) > maxCommits {
		sklog.Infof("Trimming from %d to %d commits.", len(rv.Commits), maxCommits)
		rv.Commits = rv.Commits[:maxCommits]
	}
	// Replace empty slices with nil to save a few bytes in transfer.
	if len(rv.Commits) == 0 {
		rv.Commits = nil
	}
	return rv, nil
}

// Get returns all newly-obtained data since the given time, trimmed to
// maxComits.
func (c *IncrementalCache) Get(repo string, since time.Time, maxCommits int) (*Update, error) {
	sklog.Infof("Get(%s): since: %s", repo, since)
	return c.GetRange(repo, since, time.Now().UTC(), maxCommits)
}

// GetAll returns all of the data in the cache, trimmed to maxCommits.
func (c *IncrementalCache) GetAll(repo string, maxCommits int) (*Update, error) {
	sklog.Infof("GetAll(%s): window start: %s", repo, c.w.Start(repo))
	return c.Get(repo, c.w.Start(repo), maxCommits)
}

// Update obtains new data and stores it internally keyed by the current time.
func (c *IncrementalCache) Update() error {
	sklog.Infof("Update()")
	now := time.Now().UTC()
	if err := c.w.Update(); err != nil {
		return err
	}
	newTasks, startOver, err := c.tc.Update(c.w, now)
	if err != nil {
		return err
	}
	branchHeads, commits, err := c.cc.Update(c.w, startOver, c.numCommits)
	if err != nil {
		return err
	}
	// TODO(borenet): Comments.
	updates := map[string]*Update{}
	var so *bool
	if startOver {
		so = new(bool)
		*so = true
	}
	for repo, tasks := range newTasks {
		if len(branchHeads[repo]) > 0 || len(commits[repo]) > 0 || len(tasks) > 0 {
			updates[repo] = &Update{
				BranchHeads: branchHeads[repo],
				Commits:     commits[repo],
				StartOver:   so,
				Tasks:       tasks,
				Timestamp:   now,
			}
		}
	}
	c.mtx.Lock()
	defer c.mtx.Unlock()
	if startOver {
		c.updates = map[string][]*Update{}
	}
	for repo, u := range updates {
		c.updates[repo] = append(c.updates[repo], u)
		sklog.Infof("Now have %d updates for repo %s, last ts = %s", len(c.updates[repo]), repo, u.Timestamp)
	}
	return nil
}

// UpdateLoop runs c.Update() in a loop.
func (c *IncrementalCache) UpdateLoop(ctx context.Context) {
	lv := metrics2.NewLiveness("last_successful_incremental_cache_update")
	go util.RepeatCtx(60*time.Second, ctx, func() {
		if err := c.Update(); err != nil {
			sklog.Errorf("Failed to update incremental cache: %s", err)
		} else {
			lv.Reset()
		}
	})
}
