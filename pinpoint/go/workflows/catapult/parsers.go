package catapult

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"

	apipb "go.chromium.org/luci/swarming/proto/api_v2"
	"go.skia.org/infra/go/skerr"
	skia_swarming "go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/pinpoint/go/bot_configs"
	"go.skia.org/infra/pinpoint/go/common"
	"go.skia.org/infra/pinpoint/go/compare"
	"go.skia.org/infra/pinpoint/go/workflows"
	"go.skia.org/infra/pinpoint/go/workflows/internal"
	pinpoint_proto "go.skia.org/infra/pinpoint/proto/v1"
	"go.temporal.io/sdk/workflow"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	casUrlTemplate          = "https://cas-viewer.appspot.com/%s/blobs/%s/%d/tree"
	casIsolateHashTemplate  = "%s/%d"
	repositoryUrlTemplate   = "%s/+/%s"
	swarmingTaskUrlTemplate = "https://chrome-swarming.appspot.com/task?id=%s"
)

type commitFooters struct {
	CommitBranch   string
	CommitPosition int32
	ReviewUrl      string
	ChangeID       string
}

// parseArguments parses a bisect request into a legacy reponse argument
func parseArguments(request *pinpoint_proto.ScheduleBisectRequest) (*pinpoint_proto.LegacyJobResponse_Argument, error) {
	// Unsupported: ExtraTestArgs, Pin, BatchId, Trace
	args := &pinpoint_proto.LegacyJobResponse_Argument{
		ComparisonMode: request.GetComparisonMode(),
		StartGitHash:   request.GetStartGitHash(),
		EndGitHash:     request.GetEndGitHash(),
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

func createBuildQuestDetail(commitRun *internal.BisectRun) *pinpoint_proto.LegacyJobResponse_State_Attempt_Execution {
	return &pinpoint_proto.LegacyJobResponse_State_Attempt_Execution{
		Completed: true,
		Details: []*pinpoint_proto.LegacyJobResponse_State_Attempt_Execution_Detail{
			{
				Key:   "builder",
				Value: commitRun.Build.BuildParams.Device,
			},
			{
				Key:   "isolate",
				Value: fmt.Sprintf(casIsolateHashTemplate, commitRun.Build.CAS.Digest.Hash, commitRun.Build.CAS.Digest.SizeBytes),
				Url:   fmt.Sprintf(casUrlTemplate, commitRun.Build.CAS.CasInstance, commitRun.Build.CAS.Digest.Hash, commitRun.Build.CAS.Digest.SizeBytes),
			},
		},
	}
}

func createTestQuestDetail(task *apipb.TaskResultResponse, benchmarkRun *workflows.TestRun) *pinpoint_proto.LegacyJobResponse_State_Attempt_Execution {
	details := []*pinpoint_proto.LegacyJobResponse_State_Attempt_Execution_Detail{
		{
			Key:   "bot",
			Value: task.BotId,
		},
		{
			Key:   "task",
			Value: task.TaskId,
			Url:   fmt.Sprintf(swarmingTaskUrlTemplate, task.TaskId),
		},
	}
	iso_details := &pinpoint_proto.LegacyJobResponse_State_Attempt_Execution_Detail{
		Key: "isolate",
	}
	if benchmarkRun.Status == skia_swarming.TASK_STATE_COMPLETED {
		iso_details.Value = fmt.Sprintf(casIsolateHashTemplate, benchmarkRun.CAS.Digest.Hash, benchmarkRun.CAS.Digest.SizeBytes)
		iso_details.Url = fmt.Sprintf(casUrlTemplate, benchmarkRun.CAS.CasInstance, benchmarkRun.CAS.Digest.Hash, benchmarkRun.CAS.Digest.SizeBytes)
	}
	details = append(details, iso_details)
	resp := &pinpoint_proto.LegacyJobResponse_State_Attempt_Execution{
		Completed: benchmarkRun.Status == skia_swarming.TASK_STATE_COMPLETED,
		Details:   details,
	}
	return resp
}

// parseRunData parses run data into a map of combined commit to list of attempts and a unique list of bots run for tests.
func parseRunData(ctx workflow.Context, runData []*internal.BisectRun, chart string) (map[uint32][]*pinpoint_proto.LegacyJobResponse_State_Attempt, []string, error) {
	// use as set so we don't repeat keys
	botSet := map[string]bool{}
	commitToAttempts := map[uint32][]*pinpoint_proto.LegacyJobResponse_State_Attempt{}

	// Each run has one Combined Commit, mapped to many attempts
	for _, commitRun := range runData {
		attempts := []*pinpoint_proto.LegacyJobResponse_State_Attempt{}
		for _, benchmarkRun := range commitRun.Runs {
			var task *apipb.TaskResultResponse
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
				ResultValues: benchmarkRun.Values[chart],
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
		// For writing back to Catapult, we're only really interested in Performance results.
		// Functional comparisons don't set CommitPairValues. In other words, they remain nil.
		if comparison.ResultType == internal.Performance {
			resp[comparison.CommitPairValues.Lower.Commit.Key()] = comparison.CommitPairValues.Lower.Values
			resp[comparison.CommitPairValues.Higher.Commit.Key()] = comparison.CommitPairValues.Higher.Values
		}
	}
	return resp
}

// parseToSortedCombinedCommits sorts a list of commit pairs to a list of commits by commit time.
//
// This assumes that the list is curated by the bisection sequence, resulting in an order such as
// (A, Z), (A, M), (M, Z), (A, F), (F, M), (M, S), (S, Z) and so on. This would be sorted to
// (A, F, M, S, Z)
func parseToSortedCombinedCommits(comparisons []*internal.CombinedResults) []*common.CombinedCommit {
	if len(comparisons) < 1 {
		return nil
	}
	sortedCombinedCommits := []*common.CombinedCommit{
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

// Parse email from commit author string "{author full name} ({email})"
func parseEmail(author string) string {
	p := strings.Split(author, " ")
	return strings.Trim(p[len(p)-1], "()")
}

// parseFooters parses out all commit footers, split by new line and in the format key:value
func parseFooters(commitBody string) (*commitFooters, error) {
	footers := &commitFooters{}

	lines := strings.Split(commitBody, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "Change-Id") {
			parts := strings.Split(line, "Change-Id: ")
			footers.ChangeID = parts[len(parts)-1]
		} else if strings.HasPrefix(line, "Reviewed-on") {
			parts := strings.Split(line, "Reviewed-on: ")
			footers.ReviewUrl = parts[len(parts)-1]
		} else if strings.HasPrefix(line, "Cr-Commit-Position") {
			parts := strings.Split(line, "Cr-Commit-Position: ")
			commitInfo := parts[len(parts)-1]
			subParts := strings.Split(commitInfo, "@")
			footers.CommitBranch = subParts[0]

			position, err := strconv.ParseInt(strings.Trim(subParts[1], "{#}"), 10, 32)
			if err != nil {
				return nil, skerr.Wrap(err)
			}
			footers.CommitPosition = int32(position)
		}
	}

	return footers, nil
}

// appendCommitData modifies the commit with information from gitiles
func appendCommitData(commit *pinpoint_proto.Commit, longCommit *vcsinfo.LongCommit) (*pinpoint_proto.Commit, error) {
	commit.Url = fmt.Sprintf(repositoryUrlTemplate, commit.Repository, commit.GitHash)
	commit.Author = parseEmail(longCommit.ShortCommit.Author)
	commit.Created = timestamppb.New(longCommit.Timestamp)
	commit.Subject = longCommit.ShortCommit.Subject
	commit.Message = longCommit.Body

	footers, err := parseFooters(longCommit.Body)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	commit.ChangeId = footers.ChangeID
	commit.CommitBranch = footers.CommitBranch
	commit.CommitPosition = footers.CommitPosition
	commit.ReviewUrl = footers.ReviewUrl
	return commit, nil
}

// parseCommitData returns a combined commit with all additional information filled (commit position, summary, author, etc.)
func parseCommitData(ctx workflow.Context, combinedCommit *common.CombinedCommit) ([]*pinpoint_proto.Commit, error) {
	commits := []*pinpoint_proto.Commit{}

	// handle main first
	var main *vcsinfo.LongCommit
	if err := workflow.ExecuteActivity(ctx, FetchCommitActivity, combinedCommit.Main).Get(ctx, &main); err != nil {
		return nil, skerr.Wrap(err)
	}
	mainCommit, err := appendCommitData(combinedCommit.Main, main)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	commits = append(commits, mainCommit)

	// add each modified dep as a commit of its own
	for _, modifiedDep := range combinedCommit.ModifiedDeps {
		var dep *vcsinfo.LongCommit
		if err := workflow.ExecuteActivity(ctx, FetchCommitActivity, modifiedDep).Get(ctx, &dep); err != nil {
			return nil, skerr.Wrap(err)
		}
		depCommit, err := appendCommitData(modifiedDep, dep)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		commits = append(commits, depCommit)
	}

	return commits, nil
}

// parseRawDataToLegacyObject does the heavy lifting of converting all the raw run data to objects needed for the LegacyJobResponse.
//
// returns a list of JobState objects (curated by both comparison data and run data), a list of bots used for test runs and error if any.
func parseRawDataToLegacyObject(ctx workflow.Context, comparisons []*internal.CombinedResults, runData []*internal.BisectRun, chart string) ([]*pinpoint_proto.LegacyJobResponse_State, []string, error) {
	states := []*pinpoint_proto.LegacyJobResponse_State{}

	// runData is parsed into:
	//   - a map of combinedcommit keys to attempts. each commit that's analyzed is mapped to one state object,
	//     and this allows us to fetch all attempt data for every commit that's analyzed.
	//   - a unique list of bots that all attempts ran on, which is propagated back to the root
	//     response object.
	commitToAttempts, bots, err := parseRunData(ctx, runData, chart)
	if err != nil {
		return nil, nil, skerr.Wrap(err)
	}

	commitsToValues := parseResultValuesPerCommit(comparisons)
	combinedCommitsInOrder := parseToSortedCombinedCommits(comparisons)

	for _, combinedCommit := range combinedCommitsInOrder {
		state := &pinpoint_proto.LegacyJobResponse_State{}

		attempts, ok := commitToAttempts[combinedCommit.Key()]
		if !ok {
			return nil, nil, skerr.Fmt("Cannot find combined commit %v from calculated attempts", combinedCommit)
		}
		state.Attempts = attempts

		values, ok := commitsToValues[combinedCommit.Key()]
		if !ok {
			return nil, nil, skerr.Fmt("Cannot find combined commit %v from list of values", combinedCommit)
		}
		state.Values = values

		commits, err := parseCommitData(ctx, combinedCommit)
		if err != nil {
			return nil, nil, skerr.Wrap(err)
		}
		state.Change = &pinpoint_proto.LegacyJobResponse_State_Change{
			Commits: commits,
		}

		states = append(states, state)
	}

	return states, bots, nil
}
