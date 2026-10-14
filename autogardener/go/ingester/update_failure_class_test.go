package ingester

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestComputeFlakyCulpritsResolvedBy(t *testing.T) {
	// Each string in history represents one task spec's results across commits
	// c0, c1, ... (oldest first):
	//   'S': [taskResult_Success]
	//   'F': [taskResult_Failure]
	//   'B': [taskResult_Success, taskResult_Failure] (both at same commit)
	//   ' ': [taskResult_Unknown] (did not run at commit; accumulates into blamelist)
	runTest := func(t *testing.T, history []string, expectFlaky bool, expectCulprits, expectResolvedBy []string) {
		require.NotEmpty(t, history)
		numCommits := len(history[0])
		commits := make([]string, numCommits)
		for i := range commits {
			commits[i] = fmt.Sprintf("c%d", numCommits-1-i)
		}
		taskSpecs := make([]string, len(history))
		results := make(map[string]map[string][]taskResult, len(history))
		for tsIdx, row := range history {
			require.Len(t, row, numCommits)
			ts := fmt.Sprintf("task-%d", tsIdx)
			taskSpecs[tsIdx] = ts
			results[ts] = map[string][]taskResult{}
			for cIdx, ch := range row {
				commit := fmt.Sprintf("c%d", cIdx)
				switch ch {
				case ' ':
					results[ts][commit] = []taskResult{taskResult_Unknown}
				case 'S':
					results[ts][commit] = []taskResult{taskResult_Success}
				case 'F':
					results[ts][commit] = []taskResult{taskResult_Failure}
				case 'B':
					results[ts][commit] = []taskResult{taskResult_Success, taskResult_Failure}
				default:
					t.Fatalf("unexpected character %q", ch)
				}
			}
		}

		flaky, culprits, resolvedBy := computeFlakyCulpritsResolvedBy(
			commits,
			taskSpecs,
			func(ts, commit string) []taskResult { return results[ts][commit] },
		)
		require.Equal(t, expectFlaky, flaky)
		require.Equal(t, expectCulprits, culprits)
		require.Equal(t, expectResolvedBy, resolvedBy)
	}

	t.Run("sparse staggered tasks narrow culprits and resolvedBy", func(t *testing.T) {
		runTest(t, []string{
			"S  FF S",
			" S F FS",
		}, false, []string{"c3", "c2"}, []string{"c6"})
	})

	t.Run("staggered resolution across consecutive commits narrows resolvedBy", func(t *testing.T) {
		runTest(t, []string{
			"S FF S",
			" SFFSS",
		}, false, []string{"c2"}, []string{"c4"})
	})

	t.Run("no prior success yields nil culprits", func(t *testing.T) {
		runTest(t, []string{
			"  FF S",
		}, false, nil, []string{"c5", "c4"})
	})

	t.Run("first failing task has no prior success but second task narrows culprits", func(t *testing.T) {
		runTest(t, []string{
			"  FF S",
			"S  F S",
		}, false, []string{"c3", "c2", "c1"}, []string{"c5", "c4"})
	})

	t.Run("both pass and fail on same commit is flaky", func(t *testing.T) {
		runTest(t, []string{
			"S B S",
		}, true, nil, nil)
	})

	t.Run("contradicting task specs at same commit is flaky", func(t *testing.T) {
		runTest(t, []string{
			"S FF S",
			"S FS S",
		}, true, nil, nil)
	})

	t.Run("three sparse task specs moving in tandem across episodes is not flaky", func(t *testing.T) {
		runTest(t, []string{
			"SFF S S",
			"S F SFS",
			"SF SSF ",
		}, false, []string{"c5"}, []string{"c6"})
	})

	t.Run("clustered multi-episode streaks (reland) are not flaky", func(t *testing.T) {
		runTest(t, []string{
			"SFFFFSSFFFSS",
			"S FFF S FF S",
		}, false, []string{"c7"}, []string{"c10"})
	})

	t.Run("intermittent alternating failures are flaky", func(t *testing.T) {
		runTest(t, []string{
			"SFSFSSFS",
		}, true, nil, nil)
	})
}
