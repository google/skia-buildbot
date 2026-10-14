package ingester

import (
	"go.skia.org/infra/go/util"
)

type taskResult string

const (
	taskResult_Failure = taskResult("failure")
	taskResult_Success = taskResult("success")
	taskResult_Unknown = taskResult("unknown")
)

type failureSet struct {
	Culprits   util.StringSet
	ResolvedBy util.StringSet
}

func computeFlakyCulpritsResolvedBy(commits, taskSpecs []string, getTaskResultsAtCommit func(string, string) []taskResult) (bool, []string, []string) {
	failureSets := []*failureSet{}
	var currentFailureSet *failureSet
	lastTaskResult := make(map[string]taskResult, len(taskSpecs))
	blamelistEnd := make(map[string]int, len(taskSpecs))
	for _, ts := range taskSpecs {
		blamelistEnd[ts] = len(commits)
	}
	var commitResults []bool

	// First, determine whether every task spec in the set succeeds and fails in
	// tandem. If they do not, then we've either mis-categorized a task into the
	// wrong failure class, or this is definitely a flaky failure. We'll assume
	// the latter and return true for flaky, nil for culprits and resolvedBy.
	// Iterate commits from oldest to newest.
	for i := len(commits) - 1; i >= 0; i-- {
		commit := commits[i]
		lastTaskSpecResult := taskResult_Unknown
		for _, taskSpec := range taskSpecs {
			// If we have multiple tasks which ran at this commit with different
			// results, then this is definitely a flaky failure.
			results := getTaskResultsAtCommit(taskSpec, commit)
			resultAtThisCommit := taskResult_Unknown
			for _, result := range results {
				if result == taskResult_Unknown {
					continue
				}
				if resultAtThisCommit == taskResult_Unknown {
					resultAtThisCommit = result
				} else if result != resultAtThisCommit {
					return true, nil, nil
				}
			}
			if resultAtThisCommit == taskResult_Unknown {
				continue
			}

			if lastTaskSpecResult != taskResult_Unknown && resultAtThisCommit != lastTaskSpecResult {
				// Per above, the failure ranges contradict each other. Treat this as
				// flaky and exit.
				return true, nil, nil
			}
			lastTaskSpecResult = resultAtThisCommit

			blamelist := util.NewStringSet(commits[i:blamelistEnd[taskSpec]])
			blamelistEnd[taskSpec] = i
			if resultAtThisCommit == taskResult_Failure {
				if currentFailureSet == nil {
					currentFailureSet = &failureSet{}
					failureSets = append(failureSets, currentFailureSet)
				}
				// We can only narrow down the Culprits list when transitioning
				// from success to failure for this task spec.
				if lastTaskResult[taskSpec] == taskResult_Success {
					if currentFailureSet.Culprits == nil {
						currentFailureSet.Culprits = blamelist
					} else {
						currentFailureSet.Culprits = currentFailureSet.Culprits.Intersect(blamelist)
					}
				}
			} else if resultAtThisCommit == taskResult_Success {
				if lastTaskResult[taskSpec] == taskResult_Failure && len(failureSets) > 0 {
					lastFS := failureSets[len(failureSets)-1]
					if lastFS.ResolvedBy == nil {
						lastFS.ResolvedBy = blamelist
					} else {
						lastFS.ResolvedBy = lastFS.ResolvedBy.Intersect(blamelist)
					}
				}
			}
			lastTaskResult[taskSpec] = resultAtThisCommit
		}
		if lastTaskSpecResult != taskResult_Unknown {
			commitResults = append(commitResults, lastTaskSpecResult == taskResult_Success)
			if lastTaskSpecResult == taskResult_Success {
				currentFailureSet = nil
			}
		}
	}

	if len(failureSets) == 0 {
		// This shouldn't happen if any tasks failed.
		panic("computeFlakyCulpritsResolvedBy: no failure sets found")
	}
	lastFailureSet := failureSets[len(failureSets)-1]

	// At this point we've ensured that every failing task spec succeeds and fails
	// in tandem. Now we have to get into heuristics.

	// If we have three or more task specs all moving in tandem, it is extremely
	// unlikely that we're looking at a flake.
	if len(taskSpecs) >= 3 {
		return false, getCommitSubset(commits, lastFailureSet.Culprits), getCommitSubset(commits, lastFailureSet.ResolvedBy)
	}

	// If we only saw one failure set, assume this is not a flake.
	if len(failureSets) == 1 {
		return false, getCommitSubset(commits, lastFailureSet.Culprits), getCommitSubset(commits, lastFailureSet.ResolvedBy)
	}

	// Use the Wald-Wolfowitz runs test to determine if the actual number of state
	// changes between success and failure is higher than the expected number of
	// state changes if the order of failed tasks was randomized. If so, we're
	// probably dealing with a flake.
	observedRuns, expectedRuns := util.WaldWolfowitzRuns(commitResults)
	if float64(observedRuns) >= expectedRuns {
		return true, nil, nil
	}
	return false, getCommitSubset(commits, lastFailureSet.Culprits), getCommitSubset(commits, lastFailureSet.ResolvedBy)
}

func getCommitSubset(commits []string, set util.StringSet) []string {
	if len(set) == 0 {
		return nil
	}
	out := make([]string, 0, len(set))
	for _, c := range commits {
		if set[c] {
			out = append(out, c)
		}
	}
	return out
}
