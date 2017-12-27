package app

import (
	"context"
	"net/http"
	"sync"
	"time"

	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/predict/go/failures"
	"go.skia.org/infra/predict/go/flaky"
)

// App is the core functionality for the predictor.
type App struct {
	ctx              context.Context
	taskListProvider failures.TaskListProvider
	since            time.Duration
	numSwarmingTasks metrics2.Int64Metric
	numCommits       metrics2.Int64Metric
	numIssues        metrics2.Int64Metric
	Liveness         metrics2.Liveness
	flakyBuilder     *flaky.FlakyBuilder
	failureStore     *failures.FailureStore
	FlakyRanges      flaky.Flaky
	mutex            sync.Mutex
}

func New(ctx context.Context, git *git.Checkout, httpClient *http.Client, taskListProvider failures.TaskListProvider, since time.Duration, gitRepoURL string, flakyBuilder *flaky.FlakyBuilder) (*App, error) {
	a := &App{
		ctx:              ctx,
		taskListProvider: taskListProvider,
		since:            since,
		numSwarmingTasks: metrics2.GetInt64Metric("num_swarming_tasks", nil),
		numCommits:       metrics2.GetInt64Metric("num_commits", nil),
		numIssues:        metrics2.GetInt64Metric("num_issues", nil),
		Liveness:         metrics2.NewLiveness("suggester_processing", nil),
		flakyBuilder:     flakyBuilder,
		FlakyRanges:      flaky.Flaky{},
	}
	badbot := func(botname string, ts time.Time) bool {
		return a.FlakyRanges.WasFlaky(botname, ts)
	}
	a.failureStore = failures.New(badbot, taskListProvider, git, httpClient, gitRepoURL)
	return a, nil
}

func (a *App) SinceLastRun() string {
	return (time.Duration(a.Liveness.Get()) * 1000 * 1000 * 1000).String()
}

// Predict returns a list of the best bots to run for the given set of files.
func (a *App) Predict(filenames []string) []string {
	// TODO The returned list needs to be vetted against the list of currently valid bots.
	return nil
}

func (a *App) onestep() error {
	sklog.Infoln("Updating flakes from status.")
	if err := a.flakyBuilder.Update(); err != nil {
		return err
	}
	sklog.Infoln("Building flakyRanges.")
	var err error
	a.FlakyRanges, err = a.flakyBuilder.Build(a.since, time.Now())
	if err != nil {
		return err
	}
	sklog.Infoln("Updating all failure counts.")
	if err := a.failureStore.Update(a.since); err != nil {
		return err
	}
	sklog.Infoln("Finished onestep().")
	// TODO Use a.failureStore.List() results to build a prediction model.

	a.Liveness.Reset()
	return nil
}

func (a *App) loop() {
	if err := a.onestep(); err != nil {
		sklog.Errorf("App update failed: %s", err)
	}
	for _ = range time.Tick(time.Hour) {
		if err := a.onestep(); err != nil {
			sklog.Errorf("App update failed: %s", err)
		}
	}
}

func (a *App) Start() {
	go a.loop()
}
