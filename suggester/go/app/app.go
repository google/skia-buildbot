package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/suggester/go/flaky"
	"go.skia.org/infra/suggester/go/store"
)

type App struct {
	totals           map[string]map[string]int
	ctx              context.Context
	git              *git.Checkout
	httpClient       *http.Client
	swarmApi         swarming.ApiClient
	since            time.Duration
	gitRepoURL       string
	numSwarmingTasks metrics2.Int64Metric
	numCommits       metrics2.Int64Metric
	numIssues        metrics2.Int64Metric
	liveness         metrics2.Liveness
	flaky            flaky.Flaky
}

func NewApp(ctx context.Context, git *git.Checkout, httpClient *http.Client, swarmApi swarming.ApiClient, since time.Duration, gitRepoURL string) *App {
	return &App{
		totals:           map[string]map[string]int{},
		ctx:              ctx,
		git:              git,
		httpClient:       httpClient,
		swarmApi:         swarmApi,
		since:            since,
		gitRepoURL:       gitRepoURL,
		numSwarmingTasks: metrics2.GetInt64Metric("num_swarming_tasks", nil),
		numCommits:       metrics2.GetInt64Metric("num_commits", nil),
		numIssues:        metrics2.GetInt64Metric("num_issues", nil),
		liveness:         metrics2.NewLiveness("suggester_processing", nil),
		flaky:            flaky.Flaky{},
	}
}

// TODO make this an atomic process.
func (a *App) add(filename, botname string) {
	// Note: Could parse the path and also add all subpaths,
	// which would allow for giving suggestions for files we've never seen before.
	if strings.TrimSpace(filename) == "" {
		return
	}
	if filename[:1] == "/" {
		// Ignore /COMMIT_MSG.
		return
	}
	if bots, ok := a.totals[filename]; !ok {
		a.totals[filename] = map[string]int{botname: 1}
	} else {
		bots[botname] = bots[botname] + 1
	}
}

type GerritFiles map[string]interface{}

type FileCount struct {
	Counts string
}

// Given a list of files, return a list of the best bots to run against.
func (a *App) predict(filenames []string) []string {
	return nil
}

func (a *App) onestep() error {
	resp, err := a.swarmApi.ListTasks(time.Now().Add(-1*a.since), time.Now(), []string{"pool:Skia"}, "completed_failure")
	if err != nil {
		return fmt.Errorf("Failed to query swarming: %s", err)
	}
	prefix := make([]byte, 5)
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
			files := GerritFiles{}
			if err := json.NewDecoder(resp.Body).Decode(&files); err != nil {
				sklog.Warningf("Failed to get decode file list from Gerrit: %s", err)
				continue
			}
			for k, _ := range files {
				a.add(k, tags["sk_name"])
			}
		} else if tags["sk_revision"] != "" {
			sklog.Infof("Commit: %s, Name: %s", tags["sk_revision"], tags["sk_name"])

			files, err := a.git.Git(a.ctx, "show", "--pretty=", "--name-only", tags["sk_revision"])
			if err != nil {
				sklog.Warningf("Failed to get commit file list: %s", err)
				continue
			}
			for _, filename := range strings.Split(files, "\n") {
				a.add(filename, tags["sk_name"])
			}
		} else {
			sklog.Info("Leased device task.")
		}

		// If sk_name is in the list of legal bots, get the list of files changed.
		// or
		//  GET JSON from https://skia-review.googlesource.com/changes/81121/revisions/8/files/
		//
		// Increment results in:
		//            map[filename]map[botname]int
		//            map[string]map[string]int
	}
	return store.WriteTotals(a.totals)
}
