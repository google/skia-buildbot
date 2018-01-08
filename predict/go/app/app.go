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
	period           time.Duration
	numSwarmingTasks metrics2.Int64Metric
	numCommits       metrics2.Int64Metric
	numIssues        metrics2.Int64Metric
	Liveness         metrics2.Liveness
	flakyBuilder     *flaky.FlakyBuilder
	failureStore     *failures.FailureStore
	flakyRanges      flaky.Flaky
	mutex            sync.Mutex
}

func New(ctx context.Context, git *git.Checkout, httpClient *http.Client, taskListProvider failures.TaskListProvider, period time.Duration, gitRepoURL string, flakyBuilder *flaky.FlakyBuilder) (*App, error) {
	a := &App{
		ctx:              ctx,
		taskListProvider: taskListProvider,
		period:           period,
		numSwarmingTasks: metrics2.GetInt64Metric("num_swarming_tasks", nil),
		numCommits:       metrics2.GetInt64Metric("num_commits", nil),
		numIssues:        metrics2.GetInt64Metric("num_issues", nil),
		Liveness:         metrics2.NewLiveness("suggester_processing", nil),
		flakyBuilder:     flakyBuilder,
		flakyRanges:      flaky.Flaky{},
	}
	badbot := func(botname string, ts time.Time) bool {
		return a.flakyRanges.WasFlaky(botname, ts)
	}
	a.failureStore = failures.New(badbot, taskListProvider, git, httpClient, gitRepoURL)
	return a, nil
}

func (a *App) FlakyRanges() flaky.Flaky {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	return a.flakyRanges
}

func (a *App) SinceLastRun() string {
	return (time.Duration(a.Liveness.Get()) * 1000 * 1000 * 1000).String()
}

// Predict returns a list of the best bots to run for the given set of files.
func (a *App) Predict(filenames []string) []string {
	// TODO The returned list needs to be vetted against the list of currently valid bots.
	return nil
}

func (a *App) Failures(period time.Duration) ([]*failures.StoredFailure, error) {
	now := time.Now()
	return a.failureStore.List(now.Add(-1*period), now)
}

func (a *App) onestep() error {
	sklog.Infoln("Updating flakes from status.")
	if err := a.flakyBuilder.Update(); err != nil {
		return err
	}
	sklog.Infoln("Building flakyRanges.")
	fr, err := a.flakyBuilder.Build(a.period, time.Now())
	if err != nil {
		return err
	}
	a.mutex.Lock()
	a.flakyRanges = fr
	a.mutex.Unlock()
	sklog.Infoln("Updating all failure counts.")
	if err := a.failureStore.Update(a.period); err != nil {
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
	for _ = range time.Tick(a.period) {
		if err := a.onestep(); err != nil {
			sklog.Errorf("App update failed: %s", err)
		}
	}
}

func (a *App) Start() {
	go a.loop()
}
