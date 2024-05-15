package internal

import (
	"context"
	"errors"
	"math/rand"

	apipb "go.chromium.org/luci/swarming/proto/api_v2"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/pinpoint/go/backends"
	"go.skia.org/infra/pinpoint/go/midpoint"
	"go.skia.org/infra/pinpoint/go/workflows"

	"go.temporal.io/sdk/workflow"
)

// PairwiseCommitsRunnerParams defines the parameters for PairwiseCommitsRunner workflow.
type PairwiseCommitsRunnerParams struct {
	SingleCommitRunnerParams

	// LeftCommit and RightCommit specify the two commits the pairwise runner will compare.
	// SingleCommitRunnerParams includes a field for only one commit.
	LeftCommit, RightCommit *midpoint.CombinedCommit

	// The random seed used to generate pairs.
	Seed int64
}

// PairwiseRun is the output of the PairwiseCommitsRunnerWorkflow
type PairwiseRun struct {
	Left, Right CommitRun
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
//
// TODO(b/331856095): viditchitkara@ handle odd number of iterations for pairwise execution
// workflow.
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

	// TODO(b/332391612): viditchitkara@ Build chrome for leftBuild and rightBuild in parallel
	// to save time.

	leftBuild, err := buildChrome(ctx, pc.PinpointJobID, pc.BotConfig, pc.Benchmark, pc.LeftCommit)
	if err != nil {
		return nil, skerr.Wrapf(err, "unable to build chrome for commit %s", pc.LeftCommit.Main.String())
	}

	rightBuild, err := buildChrome(ctx, pc.PinpointJobID, pc.BotConfig, pc.Benchmark, pc.RightCommit)
	if err != nil {
		return nil, skerr.Wrapf(err, "unable to build chrome for commit %s", pc.RightCommit.Main.String())
	}

	pairs := generatePairIndices(pc.Seed, int(pc.Iterations))
	runs := []struct {
		cc  *midpoint.CombinedCommit
		cas *apipb.CASReference
		ch  workflow.Channel
	}{
		{
			cc:  leftBuild.Commit,
			cas: leftBuild.CAS,
			ch:  leftRunCh,
		},
		{
			cc:  rightBuild.Commit,
			cas: rightBuild.CAS,
			ch:  rightRunCh,
		},
	}

	// [0, 1]: runs the left commit (runs[0]) and then the right (runs[1])
	// [1, 0]: runs the right commit (runs[1]) and then the left (runs[0])
	orders := [][2]int{{0, 1}, {1, 0}}
	for i := 0; i < int(pc.Iterations); i++ {
		pairIdx := pairs[i]
		// TODO(sunxiaodi@): Consider defining these maps directly using the key/value
		// pair rather than separate entries. See convertDimensions in swarming_helpers.go
		botDimension := map[string]string{
			"key":   "id",
			"value": botIds[i%len(botIds)],
		}

		// We need to make a copy of i since the following is a closure. By making a
		// copy every closure will point to it's own copy of i rather than pointing to
		// the same variable.
		iteration := int32(i)
		workflow.Go(ctx, func(gCtx workflow.Context) {
			defer wg.Done()

			for _, idx := range orders[pairIdx] {
				tr, err := runBenchmark(gCtx, runs[idx].cc, runs[idx].cas, &pc.SingleCommitRunnerParams, botDimension, iteration)
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
			Build: leftBuild,
			Runs:  leftRuns,
		},
		Right: CommitRun{
			Build: rightBuild,
			Runs:  rightRuns,
		},
	}, nil
}
