package window

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/util"
)

// Window is a struct used for managing time windows based on a duration and
// a minimum number of commits in zero or more repositories.
type Window struct {
	duration      time.Duration
	earliestStart time.Time
	mtx           sync.RWMutex
	numCommits    int
	repos         repograph.Map
	start         map[string]time.Time
}

// New returns a Window instance.
func New(ctx context.Context, duration time.Duration, numCommits int, repos repograph.Map) (*Window, error) {
	w := &Window{
		duration:   duration,
		numCommits: numCommits,
		repos:      repos,
		start:      map[string]time.Time{},
	}
	if err := w.Update(ctx); err != nil {
		return nil, err
	}
	return w, nil
}

// Update updates the start time of the Window.
func (w *Window) Update(ctx context.Context) error {
	return w.UpdateWithTime(now.Now(ctx))
}

// UpdateWithTime updates the start time of the Window, using the given current time.
func (w *Window) UpdateWithTime(now time.Time) error {
	// Take the maximum of (time period, last N commits)
	earliest := now.Add(-w.duration)
	start := map[string]time.Time{}
	baseStart := now.Add(-w.duration)

	for repoUrl, r := range w.repos {
		// Find the most recent N commits.
		// TODO(borenet): We should probably respect branch skip rules.
		latest := time.Time{}
		for _, b := range r.Branches() {
			c := r.Get(b)
			for i := 1; i < w.numCommits; i++ {
				p := c.GetParents()
				if len(p) < 1 {
					break
				}
				c = p[0]
			}
			if c.Timestamp.After(latest) {
				latest = c.Timestamp
			}
		}

		// Take the earlier of the most recent N commits or baseStart.
		s := baseStart
		if !util.TimeIsZero(latest) && latest.Before(s) {
			s = latest
		}
		start[repoUrl] = s
		if s.Before(earliest) {
			earliest = s
		}
	}

	w.mtx.Lock()
	defer w.mtx.Unlock()
	w.earliestStart = earliest
	w.start = start
	return nil
}

// Start returns the time.Time at the beginning of the Window.
func (w *Window) Start(repo string) time.Time {
	w.mtx.RLock()
	defer w.mtx.RUnlock()
	rv, ok := w.start[repo]
	if !ok {
		return w.earliestStart
	}
	return rv
}

// TestTime determines whether the given Time is in the Window.
func (w *Window) TestTime(repo string, t time.Time) bool {
	return !w.Start(repo).After(t)
}

// TestCommit determines whether the given commit is in the Window.
func (w *Window) TestCommit(repo string, c *repograph.Commit) bool {
	return w.TestTime(repo, c.Timestamp)
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
	return w.TestCommit(repo, c), nil
}

// EarliestStart returns the earliest start time of any repo's Window.
func (w *Window) EarliestStart() time.Time {
	w.mtx.RLock()
	defer w.mtx.RUnlock()
	return w.earliestStart
}

// StartTimesByRepo returns a map of repo URL to start time of that repo's
// window.
func (w *Window) StartTimesByRepo() map[string]time.Time {
	w.mtx.RLock()
	defer w.mtx.RUnlock()
	rv := make(map[string]time.Time, len(w.repos))
	for k, v := range w.start {
		rv[k] = v
	}
	return rv
}
