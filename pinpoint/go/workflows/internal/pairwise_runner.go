package internal

import (
	"context"
	"errors"
	"math/rand"
	"slices"

	apipb "go.chromium.org/luci/swarming/proto/api_v2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/pinpoint/go/backends"
	"go.skia.org/infra/pinpoint/go/common"
	"go.skia.org/infra/pinpoint/go/workflows"

	"go.temporal.io/sdk/workflow"
)

// PairwiseCommitsRunnerParams defines the parameters for PairwiseCommitsRunner workflow.
type PairwiseCommitsRunnerParams struct {
	SingleCommitRunnerParams

	// LeftBuild and RightBuild supplies the CasReference to build.
	// If provided, skips build step for that commit.
	// Use either the builds or the commits.
	LeftCAS, RightCAS *apipb.CASReference

	// LeftCommit and RightCommit specify the two commits the pairwise runner will compare.
	// SingleCommitRunnerParams includes a field for only one commit.
	LeftCommit, RightCommit *common.CombinedCommit

	// The random seed used to generate pairs.
	Seed int64
}

// PairwiseRun is the output of the PairwiseCommitsRunnerWorkflow
// TODO(b/321306427): This struct assumes that the i-th Left and
// Right CommitRuns are part of the same pair. If this assumption
// breaks, consider refactoring this struct and the subsequent
// workflows to instead store a list of PairwiseTestRuns.
// Another potential reason for refactoring is if len(Order) !=
// len(Left) or len(Right).
type PairwiseRun struct {
	Left, Right CommitRun
	// Order represents the order of the runs between Left and Right.
	// 0 means Left went first and 1 means Right went first.
	// The order is needed to handle pair failures. If a pair fails,
	// another pair that went in the other order needs to be tossed
	// from the data analysis to ensure balancing.
	Order []workflows.PairwiseOrder
}

// GetCommonCharts returns charts common to both left and right commits in alphabetical order.
// In theory, benchmark/story runs should return a deterministic set of charts.
// However, benchmarks are known to be buggy. Rather than fail the job, we log a warning.
func (pr *PairwiseRun) GetCommonCharts() []string {
	lCharts := make(map[string]string)
	rCharts := make(map[string]string)
	for _, tr := range pr.Left.Runs {
		for _, c := range tr.GetAllCharts() {
			lCharts[c] = tr.TaskID // store the swarming task ID of the run for debug
		}
	}
	for _, tr := range pr.Right.Runs {
		for _, c := range tr.GetAllCharts() {
			rCharts[c] = tr.TaskID
		}
	}

	// return nil if the benchmark runner failed on all tasks for either commit
	if len(lCharts) == 0 || len(rCharts) == 0 {
		return nil
	}

	charts := []string{}
	// find charts that show up in both left and right
	for c, id := range lCharts {
		if _, ok := rCharts[c]; !ok {
			// TODO(b/414823001): Monitor this warning on a dashboard and fire an alert when the incident rate is too high
			// Alternatively if the overall incident rate is low, escalate this log to an error.
			sklog.Warningf("Chart %s found in left charts but not right. Implies error with benchmark runner. See swarming task %s", c, id)
		} else {
			charts = append(charts, c)
		}
	}

	// double check any charts that show up in the right run but not the left
	for c, id := range rCharts {
		if _, ok := lCharts[c]; !ok {
			// TODO(b/414823001): Monitor this warning on a dashboard and fire an alert when the incident rate is too high
			// Alternatively if the overall incident rate is low, escalate this log to an error.
			sklog.Warningf("Chart %s found in right charts but not left. Implies error with benchmark runner. See swarming task %s", c, id)
		}
	}

	slices.Sort(charts)
	return charts
}

// Returns true if one or both commits in the pair is missing data for the chart
func (pr *PairwiseRun) isPairMissingData(i int, chart string) bool {
	return pr.Left.Runs[i].IsEmptyValues(chart) || pr.Right.Runs[i].IsEmptyValues(chart)
}

func (pr *PairwiseRun) calcOrderBalance(chart string) int {
	balance := 0
	for i := range pr.Order {
		missingData := pr.isPairMissingData(i, chart)
		if missingData && pr.Order[i] == workflows.LeftThenRight {
			balance += 1
		} else if missingData && pr.Order[i] == workflows.RightThenLeft {
			balance -= 1
		}
	}
	return balance
}

func (pr *PairwiseRun) removeData(i int, chart string) {
	pr.Left.Runs[i].RemoveDataFromChart(chart)
	pr.Right.Runs[i].RemoveDataFromChart(chart)
}

// if one commit in the pair fails, ensure neither commit has data.
func (pr *PairwiseRun) removeMissingDataFromPairs(chart string) {
	for i := 0; i < len(pr.Order); i++ {
		if pr.isPairMissingData(i, chart) {
			pr.removeData(i, chart)
		}
	}
}

// if >= 1 run(s) in a pair fails, remove data until the number of pairs with data
// has the same number of pairs with LeftThenRight as RightThenLeft
func (pr *PairwiseRun) removeDataUntilBalanced(chart string) {
	balance := pr.calcOrderBalance(chart)

	for i := 0; balance > 0 && i < len(pr.Order); i++ {
		// missing LeftThenRight increases balance, so remove RightThenLeft
		if !pr.isPairMissingData(i, chart) && pr.Order[i] == workflows.RightThenLeft {
			pr.removeData(i, chart)
			balance -= 1
		}
	}
	for i := 0; balance < 0 && i < len(pr.Order); i++ {
		// missing RightThenLeft decreases balance, so remove LeftThenRight
		if !pr.isPairMissingData(i, chart) && pr.Order[i] == workflows.LeftThenRight {
			pr.removeData(i, chart)
			balance += 1
		}
	}
}

// FindAvailableBotsActivity fetches a list of free, alive and non quarantined bots per provided bot
// configuration for eg: android-go-wembley-perf
//
// The function makes a swarming API call internally to fetch the desired bots. If successful, a slice
// of bot ids is returned
func FindAvailableBotsActivity(ctx context.Context, botConfig string, seed int64) ([]string, error) {
	sc, err := backends.NewSwarmingClient(ctx, backends.DefaultSwarmingServiceAddress)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to initialize swarming client")
	}

	bots, err := sc.FetchFreeBots(ctx, botConfig)
	if err != nil {
		return nil, skerr.Wrapf(err, "Error fetching bots for given bot configuration")
	}

	botIds := make([]string, len(bots))
	for i, b := range bots {
		botIds[i] = b.BotId
	}

	// The list of bot ids is randomized to make sure that the tasks
	// do not everytime pick the same set of bots and leave the remaining
	// unused almost the entire time.
	rand.New(rand.NewSource(seed)).Shuffle(len(botIds), func(i, j int) {
		botIds[i], botIds[j] = botIds[j], botIds[i]
	})

	return botIds, nil
}

// generatePairOrderIndices generates a randomized list of [0,1,0,1,0,...]
//
// The element can be used for the combination, for example:
// 0: runs the first commit, and then second commit
// 1: runs the second commit, and then first commit
// Note: The returned list of numbers contains the same number of 0s and
// 1s so the permutations of given pairs are equally distributed.
func generatePairOrderIndices(seed int64, count int) []workflows.PairwiseOrder {
	lt := make([]workflows.PairwiseOrder, count)
	// generates a list of [0,1,0,1,0,1,...]
	for i := range lt {
		lt[i] = workflows.PairwiseOrder(i % 2)
	}
	rand.New(rand.NewSource(seed)).Shuffle(len(lt), func(i, j int) {
		lt[i], lt[j] = lt[j], lt[i]
	})
	return lt
}

func generatePairwiseBenchmarkParams(p SingleCommitRunnerParams, builds []*workflows.Build, botDimension map[string]string, iteration int32, order workflows.PairwiseOrder) (firstRBP, secondRBP *RunBenchmarkParams) {
	left := &RunBenchmarkParams{
		JobID:             p.PinpointJobID,
		Commit:            builds[0].Commit,
		BuildCAS:          builds[0].CAS,
		BotConfig:         p.BotConfig,
		Benchmark:         p.Benchmark,
		Story:             p.Story,
		StoryTags:         p.StoryTags,
		Dimensions:        botDimension,
		IterationIdx:      iteration,
		Chart:             p.Chart,
		AggregationMethod: p.AggregationMethod,
	}
	right := &RunBenchmarkParams{
		JobID:             p.PinpointJobID,
		Commit:            builds[1].Commit,
		BuildCAS:          builds[1].CAS,
		BotConfig:         p.BotConfig,
		Benchmark:         p.Benchmark,
		Story:             p.Story,
		StoryTags:         p.StoryTags,
		Dimensions:        botDimension,
		IterationIdx:      iteration,
		Chart:             p.Chart,
		AggregationMethod: p.AggregationMethod,
	}
	switch order {
	case workflows.LeftThenRight:
		firstRBP = left
		secondRBP = right
	case workflows.RightThenLeft:
		firstRBP = right
		secondRBP = left
	}
	return firstRBP, secondRBP
}

// PairwiseCommitsRunnerWorkflow is a Workflow definition.
//
// PairwiseCommitsRunner builds, runs and collects benchmark sampled values from several commits.
// It runs the tests in pairs to reduces sample noises.
func PairwiseCommitsRunnerWorkflow(ctx workflow.Context, pc PairwiseCommitsRunnerParams) (*PairwiseRun, error) {
	ctx = workflow.WithActivityOptions(ctx, regularActivityOptions)
	ctx = workflow.WithChildOptions(ctx, runBenchmarkWorkflowOptions)

	var botIds []string
	if err := workflow.ExecuteActivity(ctx, FindAvailableBotsActivity, pc.BotConfig, pc.Seed).Get(ctx, &botIds); err != nil {
		return nil, err
	}

	leftRunCh := workflow.NewBufferedChannel(ctx, int(pc.Iterations))
	rightRunCh := workflow.NewBufferedChannel(ctx, int(pc.Iterations))
	ec := workflow.NewBufferedChannel(ctx, int(pc.Iterations))
	wg := workflow.NewWaitGroup(ctx)
	wg.Add(int(pc.Iterations))

	var leftBuild, rightBuild *workflows.Build
	var err error
	// TODO(b/332391612): Build leftBuild and rightBuild in parallel to save time.
	if pc.LeftCAS == nil {
		leftBuild, err = buildChrome(ctx, pc.PinpointJobID, pc.BotConfig, pc.Benchmark, pc.LeftCommit)
		if err != nil {
			return nil, skerr.Wrapf(err, "unable to build chrome for commit %s", pc.LeftCommit.Main.String())
		}
	} else {
		leftBuild = &workflows.Build{
			CAS: pc.LeftCAS,
		}
	}

	if pc.RightCAS == nil {
		rightBuild, err = buildChrome(ctx, pc.PinpointJobID, pc.BotConfig, pc.Benchmark, pc.RightCommit)
		if err != nil {
			return nil, skerr.Wrapf(err, "unable to build chrome for commit %s", pc.RightCommit.Main.String())
		}
	} else {
		rightBuild = &workflows.Build{
			CAS: pc.RightCAS,
		}
	}

	// Pairwise workflow compares the performance of two versions of Chrome against each other.
	// By shuffling the order the two commits are run, we ensure that if a difference is detected,
	// the difference is not caused by the order that the commits are run.
	pairOrder := generatePairOrderIndices(pc.Seed, int(pc.Iterations))
	builds := []*workflows.Build{leftBuild, rightBuild}
	runs := []workflow.Channel{leftRunCh, rightRunCh}

	for i := 0; i < int(pc.Iterations); i++ {
		first := workflows.PairwiseOrder(pairOrder[i])
		// TODO(b/327020123): Consider defining these maps directly using the key/value
		// pair rather than separate entries. See convertDimensions in swarming_helpers.go
		botDimension := map[string]string{
			"key":   "id",
			"value": botIds[i%len(botIds)],
		}
		// We need to make a copy of i since the following is a closure. By making a
		// copy every closure will point to it's own copy of i rather than pointing to
		// the same variable.
		iteration := int32(i)
		firstRBP, secondRBP := generatePairwiseBenchmarkParams(pc.SingleCommitRunnerParams, builds, botDimension, iteration, first)

		workflow.Go(ctx, func(gCtx workflow.Context) {
			defer wg.Done()

			var ptr *workflows.PairwiseTestRun
			// pass first into the workflow even though it is not used in the workflow,
			// only returned in the output. The return helps distinguish which return
			// ran first while debugging the UI and to ensure the unit tests can pass
			// as the unit tests cannot return channel workflows in a specified order.
			if err := workflow.ExecuteChildWorkflow(gCtx, workflows.RunBenchmarkPairwise, firstRBP, secondRBP, first).Get(gCtx, &ptr); err != nil {
				ec.Send(gCtx, err)
			}
			// use the return's first indicator to send the correct result to the correct channel.
			switch ptr.Permutation {
			case workflows.LeftThenRight:
				runs[0].Send(gCtx, ptr.FirstTestRun)
				runs[1].Send(gCtx, ptr.SecondTestRun)
			case workflows.RightThenLeft:
				runs[1].Send(gCtx, ptr.FirstTestRun)
				runs[0].Send(gCtx, ptr.SecondTestRun)
			}
		})
	}

	wg.Wait(ctx)
	leftRunCh.Close()
	rightRunCh.Close()
	ec.Close()

	// TODO(b/326480795): We can tolerate a certain number of errors but should also report
	//	test errors.
	if errs := fetchAllFromChannel[error](ctx, ec); len(errs) != 0 {
		return nil, skerr.Wrapf(errors.Join(errs...), "not all iterations are successful")
	}
	leftRuns := fetchAllFromChannel[*workflows.TestRun](ctx, leftRunCh)
	rightRuns := fetchAllFromChannel[*workflows.TestRun](ctx, rightRunCh)
	// collect values from CAS
	// TODO(b/417497693): Benchmarks do not return a deterministic number of data points for each run
	// so for a given pair, if one returns more data than the other, truncate data points until both
	// commits have the same number of data points for a given iteration
	for i := 0; i < int(pc.Iterations); i++ {
		if leftRuns[i].CAS == nil || rightRuns[i].CAS == nil {
			continue
		}
		var lv map[string][]float64
		if err := workflow.ExecuteActivity(ctx, CollectAllValuesActivity, leftRuns[i], pc.Benchmark, pc.AggregationMethod).Get(ctx, &lv); err != nil {
			return nil, skerr.Wrapf(err, "leftRuns failed %v", *leftRuns[i])
		}
		leftRuns[i].Values = lv
		var rv map[string][]float64
		if err := workflow.ExecuteActivity(ctx, CollectAllValuesActivity, rightRuns[i], pc.Benchmark, pc.AggregationMethod).Get(ctx, &rv); err != nil {
			return nil, skerr.Wrapf(err, "rightRuns failed %v", *rightRuns[i])
		}
		rightRuns[i].Values = rv
	}

	return &PairwiseRun{
		Left: CommitRun{
			Build: leftBuild,
			Runs:  leftRuns,
		},
		Right: CommitRun{
			Build: rightBuild,
			Runs:  rightRuns,
		},
		Order: pairOrder,
	}, nil
}
