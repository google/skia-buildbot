package app

import (
	"context"
	"net/http"
	"sync"
	"time"

	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/predict/go/failures"
	"go.skia.org/infra/predict/go/flaky"
	"go.skia.org/infra/predict/go/validbots"
)

// App is the core functionality for Predict.
type App struct {
	ctx              context.Context
	taskListProvider failures.TaskListProvider
	period           time.Duration
	modelPeriod      time.Duration
	numSwarmingTasks metrics2.Int64Metric
	numCommits       metrics2.Int64Metric
	numIssues        metrics2.Int64Metric
	Liveness         metrics2.Liveness
	flakyBuilder     *flaky.FlakyBuilder
	failureStore     *failures.FailureStore
	flakyRanges      flaky.Flaky
	validBots        []string
	gitRepoDir       string
	model            failures.Failures
	mutex            sync.Mutex
}

func New(ctx context.Context, git *git.Checkout, httpClient *http.Client, taskListProvider failures.TaskListProvider, period time.Duration, modelPeriod time.Duration, gitRepoURL string, flakyBuilder *flaky.FlakyBuilder, gitRepoDir string) (*App, error) {
	a := &App{
		ctx:              ctx,
		taskListProvider: taskListProvider,
		period:           period,
		modelPeriod:      modelPeriod,
		numSwarmingTasks: metrics2.GetInt64Metric("num_swarming_tasks", nil),
		numCommits:       metrics2.GetInt64Metric("num_commits", nil),
		numIssues:        metrics2.GetInt64Metric("num_issues", nil),
		Liveness:         metrics2.NewLiveness("suggester_processing", nil),
		flakyBuilder:     flakyBuilder,
		flakyRanges:      flaky.Flaky{},
		validBots:        []string{},
		gitRepoDir:       gitRepoDir,
	}
	badbot := func(botname string, ts time.Time) bool {
		return a.flakyRanges.WasFlaky(botname, ts)
	}
	a.failureStore = failures.New(badbot, taskListProvider, git, httpClient, gitRepoURL)
	return a, nil
}

// Accessors used by the main page template.

func (a *App) FlakyRanges() flaky.Flaky {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	return a.flakyRanges
}

func (a *App) Failures(period time.Duration) ([]*failures.StoredFailure, error) {
	now := time.Now()
	return a.failureStore.List(now.Add(-1*period), now)
}

func (a *App) ComputedFailures() failures.Failures {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	return a.model
}

// SinceLastRun returns the time since the last successful run of onstep().
func (a *App) SinceLastRun() string {
	return (time.Duration(a.Liveness.Get()) * 1000 * 1000 * 1000).String()
}

// Predict returns a list of the best bots to run for the given set of files.
func (a *App) Predict(filenames []string) []*failures.Summary {
	// TODO The returned list needs to be vetted against the list of currently valid bots.
	ret := []*failures.Summary{}
	sklog.Infof("About to predict from model.")
	predictions := a.model.Predict(filenames)
	sklog.Infof("Now collating response.")
	for _, pred := range predictions {
		found := false
		if !util.In(pred.BotName, a.validBots) {
			sklog.Infof("Rejecting %s as an invalid bot.", pred.BotName)
			continue
		}
		for _, s := range ret {
			if s.BotName == pred.BotName {
				s.Count += pred.Count
				found = true
			}
			break
		}
		if !found {
			ret = append(ret, pred)
		}
	}
	return ret
}

// onestep is run periodically to update the flaky ranges, the failures, and
// finally our prediction model.
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

	sklog.Infoln("Building model.")
	now := time.Now()
	model, err := a.failureStore.Failures(now.Add(-1*a.modelPeriod), now)

	vb, err := validbots.ValidBots(a.gitRepoDir)
	if err != nil {
		return err
	}
	sklog.Infoln("Finished onestep().")
	a.mutex.Lock()
	defer a.mutex.Unlock()
	a.model = model
	a.Liveness.Reset()
	a.validBots = vb

	return nil
}

// Starts the periodic processing of data.
//
// Doesn't return, must be a called in a Go routine.
func (a *App) Start() {
	if err := a.onestep(); err != nil {
		sklog.Errorf("App update failed: %s", err)
	}
	for _ = range time.Tick(a.period) {
		if err := a.onestep(); err != nil {
			sklog.Errorf("App update failed: %s", err)
		}
	}
}
