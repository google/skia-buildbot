package main

/*
   Tool for analyzing flakiness on bots.
*/

import (
	"context"
	"flag"
	"time"

	"go.skia.org/infra/flakiness/go/analysis"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/task_scheduler/go/db/remote_db"
)

var (
	// Flags.
	taskSchedulerDbUrl = flag.String("task_db_url", "http://skia-task-scheduler:8008/db/", "Where the Skia task scheduler database is hosted.")
	workdir            = flag.String("workdir", ".", "Working directory.")
)

func printResult(desc string, result []*analysis.Flake) {
	if len(result) > 0 {
		sklog.Infof("\t\t%s:", desc)
		for _, f := range result {
			sklog.Infof("\t\t\tFlake:")
			for _, t := range f.Tasks {
				sklog.Infof("\t\t\t\t%s https://chromium-swarm.appspot.com/task?id=%s", t.Status, t.SwarmingTaskId)
			}
			sklog.Infof("")
		}
	}
}

func main() {
	common.Init()

	// Setup.
	ctx := context.Background()
	taskDb, err := remote_db.NewClient(*taskSchedulerDbUrl, httputils.NewTimeoutClient())
	if err != nil {
		sklog.Fatal(err)
	}

	repos, err := repograph.NewMap(ctx, common.PUBLIC_REPOS, *workdir)
	if err != nil {
		sklog.Fatal(err)
	}
	if err := repos.Update(ctx); err != nil {
		sklog.Fatal(err)
	}

	timeWindow := 2 * 24 * time.Hour
	end := time.Now()
	start := end.Add(-timeWindow)

	// Analyze flakiness.
	results, err := analysis.Analyze(taskDb, start, end, repos)
	if err != nil {
		sklog.Fatal(err)
	}

	sklog.Infof("Results:")
	for repo, bySpec := range results {
		sklog.Infof("Repo %s", repo)
		for taskSpec, res := range bySpec {
			sklog.Infof("\t%s", taskSpec)
			printResult("Definitely Flaky", res.DefinitelyFlaky)
			printResult("Maybe Flaky", res.MaybeFlaky)
			printResult("Infra Failures", res.InfraFailures)
		}
	}
}
