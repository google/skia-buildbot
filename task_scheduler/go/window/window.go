package window

import (
	"fmt"
	"sync"
	"time"

	"go.skia.org/infra/go/git/repograph"
)

// Window is a struct used for managing time windows based on a duration and
// a minimum number of commits in zero or more repositories.
type Window struct {
	duration   time.Duration
	mtx        sync.RWMutex
	numCommits int
	repos      repograph.Map
	start      time.Time
}

// New returns a Window instance.
func New(duration time.Duration, numCommits int, repos repograph.Map) (*Window, error) {
	w := &Window{
		duration:   duration,
		numCommits: numCommits,
		repos:      repos,
	}
	if err := w.Update(); err != nil {
		return nil, err
	}
	return w, nil
}

// Update updates the start time of the Window.
func (w *Window) Update() error {
	return w.update(time.Now())
}

// update updates the start time of the Window, using the given current time.
func (w *Window) update(now time.Time) error {
	// Take the maximum of (time period, last N commits)
	start := now.Add(-w.duration)

	for _, r := range w.repos {
		// Just trace the first parent of each commit on the master
		// branch. In practice this can include more than N commits, but
		// it's better than some alternatives.
		c := r.Get("master")
		for i := 1; i < w.numCommits; i++ {
			p := c.GetParents()
			if len(p) < 1 {
				break
			}
			c = p[0]
		}
		if c.Timestamp.Before(start) {
			start = c.Timestamp
		}
	}

	w.mtx.Lock()
	defer w.mtx.Unlock()
	w.start = start
	return nil
}

// Start returns the time.Time at the beginning of the Window.
func (w *Window) Start() time.Time {
	w.mtx.RLock()
	defer w.mtx.RUnlock()
	return w.start
}

// TestTime determines whether the given Time is in the Window.
func (w *Window) TestTime(t time.Time) bool {
	return !w.Start().After(t)
}

// TestCommit determines whether the given commit is in the Window.
func (w *Window) TestCommit(c *repograph.Commit) bool {
	return w.TestTime(c.Timestamp)
}

// TestCommitHash determines whether the given commit is in the Window.
func (w *Window) TestCommitHash(repo, revision string) (bool, error) {
	r, ok := w.repos[repo]
	if !ok {
		return false, fmt.Errorf("No such repo: %s", repo)
	}
	c := r.Get(revision)
	if c == nil {
		return false, fmt.Errorf("No such commit: %s", revision)
	}
	return w.TestCommit(c), nil
}
