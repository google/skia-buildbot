package analysis

/*
	Find flaky tasks within the given time range.
*/

import (
	"fmt"
	"strings"
	"time"

	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/task_scheduler/go/db"
)

// Results is a struct which contains results from all types of flakiness
// analysis.
type Results struct {
	DefinitelyFlaky map[string][]*Flake
	InfraFailures   map[string][]*Flake
	MaybeFlaky      map[string][]*Flake
}

// Flake is a struct representing a particular instance of a flake,
// illustrated by a sequence of tasks.
type Flake struct {
	Tasks []*db.Task
}

// Analyze loads tasks from the given time period, finds various types of
// flakes, and returns a Results instance with the results.
func Analyze(tCache db.TaskCache, start, end time.Time, repos repograph.Map) (*Results, error) {
	tasks, err := tCache.GetTasksFromDateRange(start, end)
	if err != nil {
		return nil, err
	}

	maybeFlaky, err := MaybeFlaky(tCache, start, end, repos)
	if err != nil {
		return nil, err
	}
	return &Results{
		DefinitelyFlaky: DefinitelyFlaky(tasks, tCache),
		InfraFailures:   InfraFailures(tasks),
		MaybeFlaky:      maybeFlaky,
	}, nil
}

// InfraFailures finds infrastructure-related failures in the given set of tasks.
func InfraFailures(tasks []*db.Task) map[string][]*Flake {
	rv := map[string][]*Flake{}
	for _, t := range tasks {
		if t.Status == db.TASK_STATUS_MISHAP {
			rv[t.Name] = append(rv[t.Name], &Flake{
				[]*db.Task{t},
			})
		}
	}
	return rv
}

// DefinitelyFlaky finds tasks which failed and whose retries succeeded.
func DefinitelyFlaky(tasks []*db.Task, tCache db.TaskCache) map[string][]*Flake {
	byTaskSpec := map[string][]*db.Task{}
	for _, t := range tasks {
		byTaskSpec[t.Name] = append(byTaskSpec[t.Name], t)
	}

	rv := map[string][]*Flake{}
	for name, tasks := range byTaskSpec {
		for _, t := range tasks {
			// If an initial attempt failed but its retry succeeded,
			// it's definitely flaky.
			if t.RetryOf != "" && t.Success() {
				orig, err := tCache.GetTask(t.RetryOf)
				if err != nil {
					sklog.Errorf("Failed to retrieve task %q: %s", t.RetryOf, err)
					continue
					//return nil, fmt.Errorf("Failed to retrieve task %q: %s", t.RetryOf, err)
				}
				if orig.Status == db.TASK_STATUS_MISHAP {
					// Infra failures are handled elsewhere.
					continue
				}
				rv[name] = append(rv[name], &Flake{
					[]*db.Task{orig, t},
				})
			}
		}
	}
	return rv
}

// findTaskSequences finds sequences of N consecutive (commit-wise) tasks.
func findTaskSequences(n int, tCache db.TaskCache, start, end time.Time, repos repograph.Map) ([][]*db.Task, error) {
	sequences := [][]*db.Task{}
	// Handle each repo separately.
	for repoUrl, repo := range repos {
		// Traverse the commit history, obtain all tasks.
		visited := map[*db.Task]bool{}
		for _, b := range repo.Branches() {
			seqs := map[string][]*db.Task{}
			if err := repo.Get(b).Recurse(func(c *repograph.Commit) (bool, error) {
				if start.After(c.Timestamp) {
					return false, nil
				}
				// Keep recursing in this case; we haven't gotten to the commits we care about.
				if c.Timestamp.After(end) {
					return true, nil
				}

				tasks, err := tCache.GetTasksForCommits(repoUrl, []string{c.Hash})
				if err != nil {
					return false, err
				}
				if len(tasks) != 1 {
					return false, fmt.Errorf("Got incorrect number of tasks from cache!")
				}
				for taskSpec, task := range tasks[c.Hash] {
					if task.Created.After(end) || start.After(task.Created) || !task.Done() {
						continue
					}

					seq, ok := seqs[taskSpec]
					if !ok {
						seq = make([]*db.Task, 0, n)
					}

					var last *db.Task
					if len(seq) > 0 {
						last = seq[len(seq)-1]
					}
					if task == last {
						continue
					}
					if len(seq) == cap(seq) {
						copy(seq, seq[1:])
						seq[cap(seq)-1] = task
					} else {
						seq = append(seq, task)
					}
					seqs[taskSpec] = seq
					if len(seq) == cap(seq) {
						allVisited := true
						for _, t := range seq {
							if !visited[t] {
								allVisited = false
							}
						}
						if allVisited {
							return false, nil
						}
						cpy := make([]*db.Task, len(seq))
						copy(cpy, seq)
						sequences = append(sequences, cpy)
					}
					visited[task] = true
				}
				return true, nil
			}); err != nil {
				return nil, err
			}
		}
	}
	return sequences, nil
}

// MaybeFlaky finds sequences of tasks which display ping-ponging between
// success and failure states.
func MaybeFlaky(tCache db.TaskCache, start, end time.Time, repos repograph.Map) (map[string][]*Flake, error) {
	rv := map[string][]*Flake{}

	// TODO(borenet): Should we also look for sequences of four states, eg.
	// FSMF or SFMS?
	sequences, err := findTaskSequences(3, tCache, start, end, repos)
	if err != nil {
		return nil, err
	}

	// Find sequences which display the alternating-status behavior.
	for _, seq := range sequences {
		// Infra failures are handled elsewhere.
		mishap := false
		for _, t := range seq {
			if t.Status == db.TASK_STATUS_MISHAP {
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

		// Filter out any of these where the final state change is on a revert.
		c := repos[seq[2].Repo].Get(seq[2].Revision)
		if c == nil {
			return nil, fmt.Errorf("Unable to find commit %q in repo %q", seq[2].Revision, seq[2].Repo)
		}
		if strings.Contains(c.Body, fmt.Sprintf("This reverts commit %s.", seq[1].Revision)) {
			continue
		}

		tasks := make([]*db.Task, len(seq))
		copy(tasks, seq)
		rv[seq[0].Name] = append(rv[seq[0].Name], &Flake{
			Tasks: tasks,
		})
	}
	return rv, nil
}
