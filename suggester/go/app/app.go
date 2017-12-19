package app

import (
	"context"
	"net/http"
	"time"

	swarmingv1 "go.chromium.org/luci/common/api/swarming/swarming/v1"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/suggester/go/failures"
	"go.skia.org/infra/suggester/go/flaky"
)

// Let's break this into two things, the first is a process that gathers all
// the data, filters out flaky bots, and then stored in the datastore, and can
// do this incrementally. Combine go/store/flaky with go/flaky and make as a single
// useful module.
//
//
// The second process uses that data to build a prediction model.
type TaskListProvider func(since time.Duration) ([]*swarmingv1.SwarmingRpcsTaskRequestMetadata, error)

type App struct {
	ctx              context.Context
	taskListProvider failures.TaskListProvider
	since            time.Duration
	numSwarmingTasks metrics2.Int64Metric
	numCommits       metrics2.Int64Metric
	numIssues        metrics2.Int64Metric
	liveness         metrics2.Liveness
	flakyBuilder     flaky.FlakyBuilder
	failureStore     *failures.FailureStore
	flakyRanges      flaky.Flaky
}

func NewApp(ctx context.Context, git *git.Checkout, httpClient *http.Client, taskListProvider failures.TaskListProvider, since time.Duration, gitRepoURL string, flakyBuilder flaky.FlakyBuilder) (*App, error) {
	// Should only return upon successfully loading values from the datastore.
	a := &App{
		ctx:              ctx,
		taskListProvider: taskListProvider,
		since:            since,
		numSwarmingTasks: metrics2.GetInt64Metric("num_swarming_tasks", nil),
		numCommits:       metrics2.GetInt64Metric("num_commits", nil),
		numIssues:        metrics2.GetInt64Metric("num_issues", nil),
		liveness:         metrics2.NewLiveness("suggester_processing", nil),
		flakyBuilder:     flakyBuilder,
		flakyRanges:      flaky.Flaky{},
	}
	a.failureStore = failures.New(a.flakyRanges.WasFlaky, taskListProvider, ds.DS, git, httpClient, gitRepoURL)
	return a, nil
}

// Predict return a list of the best bots to run for the given set of files.
func (a *App) Predict(filenames []string) []string {
	// The returned list needs to be vetted against the list of currently valid bots.
	return nil
}

func (a *App) onestep() error {
	if err := a.flakyBuilder.Update(); err != nil {
		return err
	}
	var err error
	a.flakyRanges, err = a.flakyBuilder.Build(a.since, time.Now())
	if err != nil {
		return err
	}

	if err := a.failureStore.Update(a.since); err != nil {
		return err
	}

	// TODO Use a.failureStore.List() results to build a prediction model.

	a.liveness.Reset()
	return nil
}
