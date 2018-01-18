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
	failureStore     *failures.FailureStore
	flakyBuilder     *flaky.FlakyBuilder
	flakyRanges      flaky.Flaky
	gitRepoDir       string
	liveness         metrics2.Liveness
	maxBotDuration   time.Duration
	model            failures.Failures
	modelPeriod      time.Duration
	mutex            sync.Mutex
	numCommits       metrics2.Int64Metric
	numIssues        metrics2.Int64Metric
	numSwarmingTasks metrics2.Int64Metric
	updatePeriod     time.Duration
	taskListProvider failures.TaskListProvider
	validBots        []string
}

func New(repo *git.Checkout, httpClient *http.Client, taskListProvider failures.TaskListProvider, updatePeriod time.Duration, modelPeriod time.Duration, gitRepoURL string, flakyBuilder *flaky.FlakyBuilder, maxBotDuration time.Duration) (*App, error) {
	a := &App{
		flakyBuilder:     flakyBuilder,
		flakyRanges:      flaky.Flaky{},
		gitRepoDir:       repo.Dir(),
		liveness:         metrics2.NewLiveness("predict_processing", nil),
		modelPeriod:      modelPeriod,
		numCommits:       metrics2.GetInt64Metric("num_commits", nil),
		numIssues:        metrics2.GetInt64Metric("num_issues", nil),
		numSwarmingTasks: metrics2.GetInt64Metric("num_swarming_tasks", nil),
		updatePeriod:     updatePeriod,
		taskListProvider: taskListProvider,
		validBots:        []string{},
	}
	// badbot is the link between flaky and failures, returning true if a given
	// bot was flaky at the given time.
	badbot := func(botname string, ts time.Time) bool {
		return a.flakyRanges.WasFlaky(botname, ts)
	}
	a.failureStore = failures.New(badbot, taskListProvider, repo, httpClient, gitRepoURL)
	return a, nil
}

func (a *App) FlakyRanges() flaky.Flaky {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	return a.flakyRanges
}

func (a *App) Failures(period time.Duration) ([]*failures.StoredFailure, error) {
	now := time.Now()
	return a.failureStore.List(context.Background(), now.Add(-1*period), now)
}

func (a *App) ComputedFailures() failures.Failures {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	return a.model
}

// SinceLastRun returns the time since the last successful run of onstep().
func (a *App) SinceLastRun() string {
	return (time.Duration(a.liveness.Get()) * time.Second).String()
}

// Predict returns a list of the best bots to run for the given set of files.
func (a *App) Predict(filenames []string) []*failures.Summary {
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
	ctx := context.Background()
	sklog.Infoln("Updating flakes from status.")
	if err := a.flakyBuilder.Update(ctx); err != nil {
		return err
	}
	sklog.Infoln("Building flakyRanges.")
	fr, err := a.flakyBuilder.Build(ctx, a.modelPeriod, time.Now())
	if err != nil {
		return err
	}
	a.mutex.Lock()
	a.flakyRanges = fr
	a.mutex.Unlock()

	sklog.Infoln("Updating all failure counts.")
	if err := a.failureStore.Update(ctx, a.updatePeriod+a.maxBotDuration); err != nil {
		return err
	}

	sklog.Infoln("Building model.")
	now := time.Now()
	model, err := a.failureStore.Failures(ctx, now.Add(-1*a.modelPeriod), now)

	sklog.Infoln("Retrieve valid bots.")
	vb, err := validbots.ValidBots(a.gitRepoDir)
	if err != nil {
		return err
	}
	sklog.Infoln("Finished onestep().")
	a.mutex.Lock()
	defer a.mutex.Unlock()
	a.model = model
	a.liveness.Reset()
	a.validBots = vb

	return nil
}

// Starts the periodic processing of data.
//
// Doesn't return, must be a called in a Go routine.
func (a *App) Start() {
	util.RepeatCtx(a.updatePeriod, context.Background(), func() {
		if err := a.onestep(); err != nil {
			sklog.Errorf("App update failed: %s", err)
		}
	})
}
