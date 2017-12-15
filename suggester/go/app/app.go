package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	swarmingv1 "go.chromium.org/luci/common/api/swarming/swarming/v1"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/suggester/go/failures"
	"go.skia.org/infra/suggester/go/flaky"
	"go.skia.org/infra/suggester/go/store"
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
	git              *git.Checkout
	httpClient       *http.Client
	taskListProvider TaskListProvider
	since            time.Duration
	gitRepoURL       string
	numSwarmingTasks metrics2.Int64Metric
	numCommits       metrics2.Int64Metric
	numIssues        metrics2.Int64Metric
	liveness         metrics2.Liveness

	flakyBuilder flaky.FlakyBuilder

	mutex    sync.Mutex
	failures failures.Failures
}

func NewApp(ctx context.Context, git *git.Checkout, httpClient *http.Client, taskListProvider TaskListProvider, since time.Duration, gitRepoURL string, flakyBuilder flaky.FlakyBuilder) (*App, error) {
	// Should only return upon successfully loading values from the datastore.
	return &App{
		failures:         failures.Failures{},
		ctx:              ctx,
		git:              git,
		httpClient:       httpClient,
		taskListProvider: taskListProvider,
		since:            since,
		gitRepoURL:       gitRepoURL,
		numSwarmingTasks: metrics2.GetInt64Metric("num_swarming_tasks", nil),
		numCommits:       metrics2.GetInt64Metric("num_commits", nil),
		numIssues:        metrics2.GetInt64Metric("num_issues", nil),
		liveness:         metrics2.NewLiveness("suggester_processing", nil),
		flakyBuilder:     flakyBuilder,
	}, nil
}

// Predict return a list of the best bots to run for the given set of files.
func (a *App) Predict(filenames []string) []string {
	// The returned list needs to be vetted against the list of currently valid bots.
	return nil
}

func (a *App) onestep() error {
	// Move to being a taskProvider().
	resp, err := a.taskListProvider(a.since)
	if err != nil {
		return fmt.Errorf("Failed to query swarming: %s", err)
	}
	if err := a.flakyBuilder.Update(); err != nil {
		return fmt.Errorf("Failed to update flaky during onestep: %s", err)
	}

	flakyRanges, err := a.flakyBuilder.Build(a.since, time.Now())
	if err != nil {
		return err
	}

	prefix := make([]byte, 5)
	failures := failures.Failures{}
	for _, r := range resp {
		tags := map[string]string{}
		for _, s := range r.TaskResult.Tags {
			parts := strings.SplitN(s, ":", 2)
			tags[parts[0]] = parts[1]
		}
		if tags["sk_repo"] != a.gitRepoURL {
			sklog.Info("Not a change for our selected repo.")
			continue
		}
		if tags["sk_issue_server"] != "" {
			sklog.Infof("Issue: %s, Patch: %s Name: %s", tags["sk_issue"], tags["sk_patchset"], tags["sk_name"])
			url := fmt.Sprintf("%s/changes/%s/revisions/%s/files/", tags["sk_issue_server"], tags["sk_issue"], tags["sk_patchset"])
			resp, err := a.httpClient.Get(url)
			if err != nil {
				sklog.Warningf("Failed to get commit file list from Gerrit: %s", err)
				continue
			}
			defer util.Close(resp.Body)
			// Trim off the first 5 chars.
			n, err := resp.Body.Read(prefix)
			if n != 5 || err != nil {
				sklog.Warningf("Failed to read file list from Gerrit: %s", err)
				continue
			}
			files := map[string]interface{}{}
			if err := json.NewDecoder(resp.Body).Decode(&files); err != nil {
				sklog.Warningf("Failed to get decode file list from Gerrit: %s", err)
				continue
			}
			startTime, err := time.Parse(time.RFC3339Nano, r.TaskResult.StartedTs)
			if err != nil {
				continue
			}
			for k, _ := range files {
				// TODO check the time of this bot run against flaky.
				// "2017-12-14T17:35:06.614340"
				if !flakyRanges.WasFlaky(tags["sk_name"], startTime) {
					failures.Add(k, tags["sk_name"])
				}
			}
		} else if tags["sk_revision"] != "" {
			sklog.Infof("Commit: %s, Name: %s", tags["sk_revision"], tags["sk_name"])

			files, err := a.git.Git(a.ctx, "show", "--pretty=", "--name-only", tags["sk_revision"])
			if err != nil {
				sklog.Warningf("Failed to get commit file list: %s", err)
				continue
			}
			for _, filename := range strings.Split(files, "\n") {
				// TODO check the time of this bot run against flaky.
				failures.Add(filename, tags["sk_name"])
			}
		} else {
			sklog.Info("Leased device task.")
		}
	}
	if err := store.WriteTotals(a.failures); err != nil {
		return fmt.Errorf("Failed to write failures in onestep: %s", err)
	}
	a.mutex.Lock()
	defer a.mutex.Unlock()
	a.failures = failures
	return nil
}
