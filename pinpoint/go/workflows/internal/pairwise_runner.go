package internal

import (
	"context"
	"errors"
	"math/rand"

	swarmingV1 "go.chromium.org/luci/common/api/swarming/swarming/v1"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/pinpoint/go/midpoint"
	"go.skia.org/infra/pinpoint/go/workflows"
	"go.temporal.io/sdk/workflow"
)

// PairwiseCommitsRunnerParams defines the parameters for PairwiseCommitsRunner workflow.
type PairwiseCommitsRunnerParams struct {
	SingleCommitRunnerParams

	// The random seed used to generate pairs.
	Seed int64

	// LeftBuild and RightBuild to run in pair.
	LeftBuild, RightBuild workflows.Build
}

type PairwiseRun struct {
	Left, Right CommitRun
}

func FindAvailableBots(ctx context.Context, botConfig string) ([]string, error) {
	// TODO(viditchitkara@): Fetch the bots and return their ids.
	return nil, nil
}

// generatePairIndices generates a randomized list of [0,1,0,1,0,...]
//
// The element can be used for the combination, for example:
// 0: [0, 1], runs the first commit, and then second commit
// 1: [1, 0], runs the second commit, and then first commit
func generatePairIndices(seed int64, count int) []int {
	lt := make([]int, count)
	// generates a list of [0,1,0,1,0,1,...]
	for i := range lt {
		lt[i] = i % 2
	}
	rand.New(rand.NewSource(seed)).Shuffle(len(lt), func(i, j int) {
		lt[i], lt[j] = lt[j], lt[i]
	})
	return lt
}

// PairwiseCommitsRunnerWorkflow is a Workflow definition.
//
// PairwiseCommitsRunner builds, runs and collects benchmark sampled values from several commits.
// It runs the tests in pairs to reduces sample noises.
func PairwiseCommitsRunnerWorkflow(ctx workflow.Context, pc PairwiseCommitsRunnerParams) (*PairwiseRun, error) {
	ctx = workflow.WithActivityOptions(ctx, regularActivityOptions)
	ctx = workflow.WithChildOptions(ctx, runBenchmarkWorkflowOptions)

	// TODO(viditchitkara@): Build Chrome if they are not available
	//	using pc.LeftBuild/RightBuild.BuildChromeParams
	var botIds []string
	if err := workflow.ExecuteActivity(ctx, FindAvailableBots, pc.BotConfig).Get(ctx, &botIds); err != nil {
		return nil, err
	}

	leftRunCh := workflow.NewBufferedChannel(ctx, int(pc.Iterations))
	rightRunCh := workflow.NewBufferedChannel(ctx, int(pc.Iterations))
	ec := workflow.NewBufferedChannel(ctx, int(pc.Iterations))
	wg := workflow.NewWaitGroup(ctx)
	wg.Add(int(pc.Iterations))

	pairs := generatePairIndices(pc.Seed, int(pc.Iterations))
	runs := []struct {
		cc  *midpoint.CombinedCommit
		cas *swarmingV1.SwarmingRpcsCASReference
		ch  workflow.Channel
	}{
		{
			cc:  pc.LeftBuild.Commit,
			cas: pc.LeftBuild.CAS,
			ch:  leftRunCh,
		},
		{
			cc:  pc.RightBuild.Commit,
			cas: pc.RightBuild.CAS,
			ch:  rightRunCh,
		},
	}

	// [0, 1]: runs the left commit (runs[0]) and then the right (runs[1])
	// [1, 0]: runs the right commit (runs[1]) and then the left (runs[0])
	orders := [][2]int{{0, 1}, {1, 0}}
	for i := 0; i < int(pc.Iterations); i++ {
		pairIdx := pairs[i]
		workflow.Go(ctx, func(gCtx workflow.Context) {
			defer wg.Done()

			for _, idx := range orders[pairIdx] {
				// TODO(viditchitkara@): append bot id to the dimension so they only run on the given bot.
				tr, err := runBenchmark(gCtx, runs[idx].cc, runs[idx].cas, &pc.SingleCommitRunnerParams)
				if err != nil {
					ec.Send(gCtx, err)
					continue
				}
				runs[idx].ch.Send(gCtx, tr)
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

	rightRuns := fetchAllFromChannel[*workflows.TestRun](ctx, rightRunCh)
	leftRuns := fetchAllFromChannel[*workflows.TestRun](ctx, leftRunCh)

	return &PairwiseRun{
		Left: CommitRun{
			Build: &pc.LeftBuild,
			Runs:  leftRuns,
		},
		Right: CommitRun{
			Build: &pc.RightBuild,
			Runs:  rightRuns,
		},
	}, nil
}
