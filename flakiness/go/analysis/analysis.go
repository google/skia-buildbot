package analysis

/*
	Find flaky tasks within the given time range.
*/

import (
	"regexp"
	"strings"
	"time"

	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/types"
)

var (
	// revertRE is a regular expression used for detecting a commit which
	// reverts another commit.
	revertRE = regexp.MustCompile("This reverts commit ([a-fA-F0-9]{40})")
)

// Results is a struct which contains results from all types of flakiness
// analysis for a given TaskSpec within a repo.
type Results struct {
	DefinitelyFlaky []*Flake
	InfraFailures   []*Flake
	MaybeFlaky      []*Flake
}

// Flake is a struct representing a particular instance of a flake,
// illustrated by a sequence of tasks.
type Flake struct {
	Tasks []*types.Task
}

// Analyze loads tasks from the given time period, finds various types of
// flakes, and returns a map[repo URL]map[task spec name]*Results.
func Analyze(d db.TaskReader, start, end time.Time, repos repograph.Map) (map[string]map[string]*Results, error) {
	// Obtain all tasks from the time range.
	tasks, err := d.GetTasksFromDateRange(start, end, "")
	if err != nil {
		return nil, err
	}

	// Organize by repo and task spec.
	byTaskSpec := make(map[string]map[string][]*types.Task, len(repos))
	for _, t := range tasks {
		// Filter by repo, since the DB might contain tasks from repos
		// we don't care about.
		if _, ok := repos[t.Repo]; !ok {
			continue
		}

		m, ok := byTaskSpec[t.Repo]
		if !ok {
			m = map[string][]*types.Task{}
			byTaskSpec[t.Repo] = m
		}
		m[t.Name] = append(m[t.Name], t)
	}

	// Find flakes for repos+taskspecs.
	rv := make(map[string]map[string]*Results, len(repos))
	for repoUrl, taskSpecs := range byTaskSpec {
		m := make(map[string]*Results, len(taskSpecs))
		for taskSpec, tasks := range taskSpecs {
			def := DefinitelyFlaky(tasks)
			inf := InfraFailures(tasks)
			may := MaybeFlaky(tasks, start, end, repos[repoUrl])
			if len(def) > 0 || len(inf) > 0 || len(may) > 0 {
				m[taskSpec] = &Results{
					DefinitelyFlaky: def,
					InfraFailures:   inf,
					MaybeFlaky:      may,
				}
			}
		}
		if len(m) > 0 {
			rv[repoUrl] = m
		}
	}
	return rv, nil
}

// InfraFailures finds infrastructure-related failures in the given set of tasks.
func InfraFailures(tasks []*types.Task) []*Flake {
	rv := []*Flake{}
	for _, t := range tasks {
		if t.Status == types.TASK_STATUS_MISHAP {
			rv = append(rv, &Flake{
				[]*types.Task{t},
			})
		}
	}
	return rv
}

// DefinitelyFlaky finds tasks which failed and whose retries succeeded.
func DefinitelyFlaky(tasks []*types.Task) []*Flake {
	// Organize the tasks by ID so that we can find them later.
	byId := make(map[string]*types.Task, len(tasks))
	for _, t := range tasks {
		byId[t.Id] = t
	}

	rv := []*Flake{}
	for _, t := range tasks {
		// If an initial attempt failed but its retry succeeded,
		// it's definitely flaky.
		if t.RetryOf != "" && t.Success() {
			orig, ok := byId[t.RetryOf]
			if !ok {
				sklog.Warningf("Failed to retrieve task %q: not in range", t.RetryOf)
				continue
			}
			if orig.Status == types.TASK_STATUS_MISHAP {
				// Infra failures are handled elsewhere.
				continue
			}
			rv = append(rv, &Flake{
				[]*types.Task{orig, t},
			})
		}
	}
	return rv
}

// findTaskSequences finds sequences of N consecutive (commit-wise) tasks.
func findTaskSequences(n int, tasks []*types.Task, start, end time.Time, repo *repograph.Graph) [][]*types.Task {
	// First, organize tasks by blamelist.
	byBlamelist := make(map[string]*types.Task, len(tasks))
	for _, t := range tasks {
		for _, c := range t.Commits {
			byBlamelist[c] = t
		}
	}

	// Traverse the commit history, obtain all tasks.
	sequences := [][]*types.Task{}
	visited := map[*types.Task]bool{}
	for _, b := range repo.Branches() {
		seq := make([]*types.Task, 0, n)
		if err := repo.Get(b).Recurse(func(c *repograph.Commit) (bool, error) {
			if start.After(c.Timestamp) {
				return false, nil
			}
			// Keep recursing in this case; we haven't gotten to the commits we care about.
			if c.Timestamp.After(end) {
				return true, nil
			}

			task, ok := byBlamelist[c.Hash]
			if !ok {
				return true, nil
			}
			// Throw out unfinished tasks and those outside the window.
			if task.Created.After(end) || start.After(task.Created) || !task.Done() {
				return true, nil
			}

			// Multiple commits in a row may share the same task.
			var last *types.Task
			if len(seq) > 0 {
				last = seq[len(seq)-1]
			}
			if task == last {
				return true, nil
			}

			// Add this task to the current sequence.
			if len(seq) == cap(seq) {
				copy(seq, seq[1:])
				seq[cap(seq)-1] = task
			} else {
				seq = append(seq, task)
			}
			if len(seq) == cap(seq) {
				// Make sure we haven't seen this sequence before.
				// This can happen in the case of two diverging branches.
				allVisited := true
				for _, t := range seq {
					if !visited[t] {
						allVisited = false
					}
				}
				if allVisited {
					return false, nil
				}

				// Save the sequence.
				cpy := make([]*types.Task, len(seq))
				copy(cpy, seq)
				sequences = append(sequences, cpy)
			}
			visited[task] = true
			return true, nil
		}); err != nil {
			// Recurse only returns an error if the function we pass
			// in returns an error. It doesn't.
			sklog.Errorf("Error: %s", err)
		}
	}
	return sequences
}

// revertOf returns the commit hash of the commit which the given commit
// reverts, or the empty string if the given commit is not a revert.
func revertOf(c *repograph.Commit) string {
	for _, line := range strings.Split(c.Body, "\n") {
		m := revertRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		if len(m) != 2 {
			continue
		}
		return m[1]
	}
	return ""
}

// MaybeFlaky finds sequences of tasks which display ping-ponging between
// success and failure states.
func MaybeFlaky(tasks []*types.Task, start, end time.Time, repo *repograph.Graph) []*Flake {
	rv := []*Flake{}

	// TODO(borenet): Should we also look for sequences of four states, eg.
	// FSMF or SFMS?
	sequences := findTaskSequences(3, tasks, start, end, repo)

	// Find sequences which display the alternating-status behavior.
	for _, seq := range sequences {
		// Infra failures are handled elsewhere.
		mishap := false
		for _, t := range seq {
			if t.Status == types.TASK_STATUS_MISHAP {
				mishap = true
				break
			}
		}
		if mishap {
			continue
		}

		// FSF or SFS.
		if seq[0].Status != seq[2].Status {
			continue
		}
		if seq[0].Status == seq[1].Status {
			continue
		}

		// Filter out any of these where the final state change is on a
		// revert. Sequences are in reverse chronological order, since
		// that's how we traversed the repo.
		revertedCommits := util.StringSet{}
		for _, commit := range seq[0].Commits {
			c := repo.Get(commit)
			if c == nil {
				sklog.Errorf("Unable to find commit %q in repo %q", seq[0].Revision, seq[0].Repo)
				continue
			}
			revertedCommit := revertOf(c)
			if revertedCommit != "" {
				revertedCommits[revertedCommit] = true
			}
		}
		wasRevert := len(revertedCommits.Intersect(util.NewStringSet(seq[1].Commits))) > 0
		if wasRevert {
			sklog.Infof("Failure was reverted: %s @ %s", seq[1].Name, seq[1].Revision)
			continue
		}

		// Add the flake to the results.
		flake := make([]*types.Task, len(seq))
		copy(flake, seq)
		rv = append(rv, &Flake{
			Tasks: flake,
		})
	}
	return rv
}
