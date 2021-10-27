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

// Window uses both a Git repo and a duration of time to determine whether a
// given commit or timestamp is within a specified scheduling range, eg. 10
// commits or the last 24 hours.
type Window interface {
	// Update updates the start time of the Window.
	Update(ctx context.Context) error
	// UpdateWithTime updates the start time of the Window, using the given current time.
	UpdateWithTime(now time.Time) error
	// Start returns the time.Time at the beginning of the Window.
	Start(repo string) time.Time
	// TestTime determines whether the given Time is in the Window.
	TestTime(repo string, t time.Time) bool
	// TestCommit determines whether the given commit is in the Window.
	TestCommit(repo string, c *repograph.Commit) bool
	// TestCommitHash determines whether the given commit is in the Window.
	TestCommitHash(repo, revision string) (bool, error)
	// EarliestStart returns the earliest start time of any repo's Window.
	EarliestStart() time.Time
	// StartTimesByRepo returns a map of repo URL to start time of that repo's
	// window.
	StartTimesByRepo() map[string]time.Time
}

// WindowImpl is a struct used for managing time windows based on a duration and
// a minimum number of commits in zero or more repositories.
type WindowImpl struct {
	duration      time.Duration
	earliestStart time.Time
	mtx           sync.RWMutex
	numCommits    int
	repos         repograph.Map
	start         map[string]time.Time
}

// New returns a Window instance.
func New(ctx context.Context, duration time.Duration, numCommits int, repos repograph.Map) (*WindowImpl, error) {
	w := &WindowImpl{
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

// Update implements Window.
func (w *WindowImpl) Update(ctx context.Context) error {
	return w.UpdateWithTime(now.Now(ctx))
}

// UpdateWithTime implements Window.
func (w *WindowImpl) UpdateWithTime(now time.Time) error {
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

// Start implements Window.
func (w *WindowImpl) Start(repo string) time.Time {
	w.mtx.RLock()
	defer w.mtx.RUnlock()
	rv, ok := w.start[repo]
	if !ok {
		return w.earliestStart
	}
	return rv
}

// TestTime implements Window.
func (w *WindowImpl) TestTime(repo string, t time.Time) bool {
	return !w.Start(repo).After(t)
}

// TestCommit implements Window.
func (w *WindowImpl) TestCommit(repo string, c *repograph.Commit) bool {
	return w.TestTime(repo, c.Timestamp)
}

// TestCommitHash implements Window.
func (w *WindowImpl) TestCommitHash(repo, revision string) (bool, error) {
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

// EarliestStart implements Window.
func (w *WindowImpl) EarliestStart() time.Time {
	w.mtx.RLock()
	defer w.mtx.RUnlock()
	return w.earliestStart
}

// StartTimesByRepo implements Window.
func (w *WindowImpl) StartTimesByRepo() map[string]time.Time {
	w.mtx.RLock()
	defer w.mtx.RUnlock()
	rv := make(map[string]time.Time, len(w.repos))
	for k, v := range w.start {
		rv[k] = v
	}
	return rv
}

var _ Window = &WindowImpl{}
