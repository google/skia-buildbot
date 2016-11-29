package window

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.skia.org/infra/go/git/repograph"
)

// Window is a struct used for
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

	for repoUrl, r := range w.repos {
		// Here we assume master has the most commits. Note that this is
		// not actually the last N commits, but the time span of the
		// last N commits on the master branch.

		// We may have fewer than N commits.
		n := w.numCommits
		output, err := r.Repo().Git("rev-list", "--count", "--first-parent", "master")
		if err != nil {
			return err
		}
		num, err := strconv.Atoi(strings.TrimSpace(output))
		if err != nil {
			return err
		}
		num--
		if num < n {
			n = num
		}

		// Get the timestamp of the Nth commit.
		hash, err := r.Repo().RevParse(fmt.Sprintf("master~%d", n))
		if err != nil {
			return err
		}
		commit := r.Get(hash)
		if commit == nil {
			return fmt.Errorf("No such commit %s in repo %s", hash, repoUrl)
		}
		if commit.Timestamp.Before(start) {
			start = commit.Timestamp
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
	return w.Start().Before(t)
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
