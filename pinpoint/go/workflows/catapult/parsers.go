package catapult

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"

	"go.chromium.org/luci/common/api/swarming/swarming/v1"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/pinpoint/go/bot_configs"
	"go.skia.org/infra/pinpoint/go/compare"
	"go.skia.org/infra/pinpoint/go/midpoint"
	"go.skia.org/infra/pinpoint/go/workflows"
	"go.skia.org/infra/pinpoint/go/workflows/internal"
	pinpoint_proto "go.skia.org/infra/pinpoint/proto/v1"
	"go.temporal.io/sdk/workflow"
)

const (
	CASUrlTemplate          = "https://cas-viewer.appspot.com/%s/blobs/%s/%d/tree"
	CASIsolateHashTemplate  = "%s/%d"
	SwarmingTaskUrlTemplate = "https://chrome-swarming.appspot.com/task?id=%s"
)

// convertImprovementDir converts the improvement direction from string to int.
// UP, DOWN, UNKNOWN = (0, 1, 4)
func parseImprovementDir(dir compare.ImprovementDir) int32 {
	switch dir {
	case compare.Up:
		return 0
	case compare.Down:
		return 1
	default:
		return 4
	}
}

// TODO(b/335543316): the build information isn't propagated back to the CommitRun.Build object, so it's just empty right now and causes nil issues.
func createBuildQuestDetail(commitRun *internal.BisectRun) *pinpoint_proto.LegacyJobResponse_State_Attempt_Execution {
	return &pinpoint_proto.LegacyJobResponse_State_Attempt_Execution{
		Completed: true,
		Details: []*pinpoint_proto.LegacyJobResponse_State_Attempt_Execution_Detail{
			{
				Key: "builder",
				// TODO(b/335543316): The following fulfills the req above, but it's commented out b/c of nil reference to Build.
				// Value: commitRun.Build.Device,
			},
			{
				Key: "isolate",
				// TODO(b/335543316): The following fulfills the req above, but it's commented out b/c of nil reference to Build.
				// Value: fmt.Sprintf(CASIsolateHashTemplate, commitRun.Build.CAS.Digest.Hash, commitRun.Build.CAS.Digest.SizeBytes),
				// Url:   fmt.Sprintf(CASUrlTemplate, commitRun.Build.CAS.CasInstance, commitRun.Build.CAS.Digest.Hash, commitRun.Build.CAS.Digest.SizeBytes),
			},
		},
	}
}

func createTestQuestDetail(task *swarming.SwarmingRpcsTaskResult, benchmarkRun *workflows.TestRun) *pinpoint_proto.LegacyJobResponse_State_Attempt_Execution {
	return &pinpoint_proto.LegacyJobResponse_State_Attempt_Execution{
		Completed: true,
		Details: []*pinpoint_proto.LegacyJobResponse_State_Attempt_Execution_Detail{
			{
				Key:   "bot",
				Value: task.BotId,
			},
			{
				Key:   "task",
				Value: task.TaskId,
				Url:   fmt.Sprintf(SwarmingTaskUrlTemplate, task.TaskId),
			},
			{
				Key:   "isolate",
				Value: fmt.Sprintf(CASIsolateHashTemplate, benchmarkRun.CAS.Digest.Hash, benchmarkRun.CAS.Digest.SizeBytes),
				Url:   fmt.Sprintf(CASUrlTemplate, benchmarkRun.CAS.CasInstance, benchmarkRun.CAS.Digest.Hash, benchmarkRun.CAS.Digest.SizeBytes),
			},
		},
	}
}

// parseRunData parses run data into a map of combined commit to list of attempts and a unique list of bots run for tests.
func parseRunData(ctx workflow.Context, runData []*internal.BisectRun) (map[uint32][]*pinpoint_proto.LegacyJobResponse_State_Attempt, []string, error) {
	// use as set so we don't repeat keys
	botSet := map[string]bool{}
	commitToAttempts := map[uint32][]*pinpoint_proto.LegacyJobResponse_State_Attempt{}

	// Each run has one Combined Commit, mapped to many attempts
	for _, commitRun := range runData {
		attempts := []*pinpoint_proto.LegacyJobResponse_State_Attempt{}
		for _, benchmarkRun := range commitRun.Runs {
			var task *swarming.SwarmingRpcsTaskResult
			if err := workflow.ExecuteActivity(ctx, FetchTaskActivity, benchmarkRun.TaskID).Get(ctx, &task); err != nil {
				return nil, nil, skerr.Wrapf(err, "failed to fetch task for parsing the bot id")
			}

			// This is to track list of all bots used for execution
			botSet[task.BotId] = true

			attempt := &pinpoint_proto.LegacyJobResponse_State_Attempt{
				Executions: []*pinpoint_proto.LegacyJobResponse_State_Attempt_Execution{
					createBuildQuestDetail(commitRun),
					createTestQuestDetail(task, benchmarkRun),
					// Get values detail is always empty for bisect
					{
						Completed: true,
						Details:   []*pinpoint_proto.LegacyJobResponse_State_Attempt_Execution_Detail{},
					},
				},
			}

			attempts = append(attempts, attempt)
		}
		commitToAttempts[commitRun.Build.Commit.Key()] = attempts
	}

	// convert to list of keys from map
	bots := make([]string, len(botSet))
	idx := 0
	for k := range botSet {
		bots[idx] = k
		idx++
	}

	return commitToAttempts, bots, nil
}

// parseResultValuesPerCommit converts combinedresults into an accessible map of combinedcommit's keys to its values.
//
// This assumes that result values are re-used, so for Combined Commits that appear multiple times in comparisons
// the values should be the same.
func parseResultValuesPerCommit(comparisons []*internal.CombinedResults) map[uint32][]float64 {
	resp := map[uint32][]float64{}
	for _, comparison := range comparisons {
		resp[comparison.CommitPairValues.Lower.Commit.Key()] = comparison.CommitPairValues.Lower.Values
		resp[comparison.CommitPairValues.Higher.Commit.Key()] = comparison.CommitPairValues.Higher.Values
	}
	return resp
}

// parseRawDataToLegacyObject does the heavy lifting of converting all the raw run data to objects needed for the LegacyJobResponse.
func parseRawDataToLegacyObject(ctx workflow.Context, comparisons []*internal.CombinedResults, runData []*internal.BisectRun) ([]*pinpoint_proto.LegacyJobResponse_State, []string, error) {
	states := []*pinpoint_proto.LegacyJobResponse_State{}

	// runData is parsed into:
	//   - a map of combinedcommit keys to attempts. each commit that's analyzed is mapped to one state object,
	//     and this allows us to fetch all attempt data for every commit that's analyzed.
	//   - a unique list of bots that all attempts ran on, which is propagated back to the root
	//     response object.
	// TODO(jeffyoon@) - leaving commitsToAttempts as _ until it gets utilized.
	_, bots, err := parseRunData(ctx, runData)
	if err != nil {
		return nil, nil, skerr.Wrap(err)
	}

	// TODO(jeffyoon@) - create state objects and append to states list.

	return states, bots, nil
}

func parseArguments(request *pinpoint_proto.ScheduleBisectRequest) (*pinpoint_proto.LegacyJobResponse_Argument, error) {
	// Unsupported: ExtraTestArgs, Pin, BatchId
	args := &pinpoint_proto.LegacyJobResponse_Argument{
		ComparisonMode: request.GetComparisonMode(),
		StartGitHash:   request.GetStartGitHash(),
		EndGitHash:     request.GetEndGitHash(),
		Trace:          request.GetChart(),
		Configuration:  request.GetConfiguration(),
		Benchmark:      request.GetBenchmark(),
		Story:          request.GetStory(),
		StoryTags:      request.GetStoryTags(),
		Chart:          request.GetChart(),
		Statistic:      request.GetStatistic(),
		Project:        request.GetProject(),
		BugId:          request.GetBugId(),
	}

	if request.GetBenchmark() != "" && request.GetConfiguration() != "" {
		target, err := bot_configs.GetIsolateTarget(request.GetConfiguration(), request.GetBenchmark())
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		args.Target = target
	}

	if request.GetInitialAttemptCount() != "" {
		initAttemptCount, err := strconv.ParseInt(request.GetInitialAttemptCount(), 10, 32)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		args.InitialAttemptCount = int32(initAttemptCount)
	}

	if request.GetComparisonMagnitude() != "" {
		comparisonMagnitude, err := strconv.ParseFloat(request.GetComparisonMagnitude(), 64)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		args.ComparisonMagnitude = comparisonMagnitude
	}

	if request.GetTags() != "" {
		var tags map[string]string
		if err := json.Unmarshal([]byte(request.GetTags()), &tags); err != nil {
			return nil, skerr.Wrap(err)
		}

		args.Tags = tags
	}

	return args, nil
}

// parseToSortedCombinedCommits sorts a list of commit pairs to a list of commits by commit time.
//
// This assumes that the list is curated by the bisection sequence, resulting in an order such as
// (A, Z), (A, M), (M, Z), (A, F), (F, M), (M, S), (S, Z) and so on. This would be sorted to
// (A, F, M, S, Z)
func parseToSortedCombinedCommits(comparisons []*internal.CombinedResults) []*midpoint.CombinedCommit {
	if len(comparisons) < 1 {
		return nil
	}
	sortedCombinedCommits := []*midpoint.CombinedCommit{
		comparisons[0].CommitPairValues.Lower.Commit,
		comparisons[0].CommitPairValues.Higher.Commit,
	}
	for idx := 1; idx < len(comparisons); idx++ {
		comparison := comparisons[idx]
		midIdx := len(sortedCombinedCommits) / 2
		lowerCommit := comparison.CommitPairValues.Lower.Commit
		lowerCommitKey := lowerCommit.Key()
		higherCommit := comparison.CommitPairValues.Higher.Commit
		higherCommitKey := higherCommit.Key()

		if sortedCombinedCommits[midIdx-1].Key() == lowerCommitKey && sortedCombinedCommits[midIdx].Key() != higherCommitKey {
			// Given (A M Z), lower is A and higher is E (not M), inject higher inbetween A and M.
			sortedCombinedCommits = slices.Insert(sortedCombinedCommits, midIdx, higherCommit)
		} else if sortedCombinedCommits[midIdx].Key() == lowerCommitKey && sortedCombinedCommits[midIdx+1].Key() != higherCommitKey {
			// Given (A M Z), lower is M and higher is Z, inject higher inbetween M and Z.
			sortedCombinedCommits = slices.Insert(sortedCombinedCommits, midIdx+1, higherCommit)
		} else if midIdx+1 < len(sortedCombinedCommits) && sortedCombinedCommits[midIdx+1].Key() == lowerCommitKey && sortedCombinedCommits[midIdx+2].Key() != higherCommitKey {
			// Given (A M Q U Z), lower is U and higher is not Z, inject higher inbetween U and Z.
			sortedCombinedCommits = slices.Insert(sortedCombinedCommits, midIdx+2, higherCommit)
		} else if sortedCombinedCommits[midIdx-1].Key() == higherCommitKey && sortedCombinedCommits[midIdx-2].Key() != lowerCommitKey {
			// Given (A D F M Z), higher is D and lower is not A so inject lower inbetween A and D
			sortedCombinedCommits = slices.Insert(sortedCombinedCommits, midIdx-1, lowerCommit)
		} else if sortedCombinedCommits[midIdx].Key() == higherCommitKey && sortedCombinedCommits[midIdx-1].Key() != lowerCommitKey {
			// Given (A D F M Z), higher is F and lower is not D so inject lower inbetween D and F
			sortedCombinedCommits = slices.Insert(sortedCombinedCommits, midIdx, lowerCommit)
		} else if midIdx+1 < len(sortedCombinedCommits) && sortedCombinedCommits[midIdx+1].Key() == higherCommitKey && sortedCombinedCommits[midIdx].Key() != lowerCommitKey {
			// Given (A D F M Z), higher is M and lower is not F so inject lower inbetween F and M
			sortedCombinedCommits = slices.Insert(sortedCombinedCommits, midIdx+1, lowerCommit)
		}
	}
	return sortedCombinedCommits
}
