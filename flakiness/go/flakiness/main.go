package main

/*
   Tool for analyzing flakiness on bots.
*/

import (
	"flag"
	"sort"
	"time"

	"go.skia.org/infra/flakiness/go/analysis"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/db/remote_db"
	"go.skia.org/infra/task_scheduler/go/window"
)

var (
	// Flags.
	taskSchedulerDbUrl = flag.String("task_db_url", "http://skia-task-scheduler:8008/db/", "Where the Skia task scheduler database is hosted.")
	workdir            = flag.String("workdir", ".", "Working directory.")
)

func printResult(desc string, result map[string][]*analysis.Flake) {
	sklog.Infof("%s:", desc)
	taskSpecs := make([]string, 0, len(result))
	for taskSpec, _ := range result {
		taskSpecs = append(taskSpecs, taskSpec)
	}
	sort.Strings(taskSpecs)
	for _, taskSpec := range taskSpecs {
		flakes := result[taskSpec]
		sklog.Infof("\t%s", taskSpec)
		for _, f := range flakes {
			sklog.Infof("\t\tFlake:")
			for _, t := range f.Tasks {
				sklog.Infof("\t\t\t%s https://chromium-swarm.appspot.com/task?id=%s", t.Status, t.SwarmingTaskId)
			}
			sklog.Infof("")
		}
	}
}

func main() {
	common.Init()
	defer common.LogPanic()

	// Setup.
	taskDb, err := remote_db.NewClient(*taskSchedulerDbUrl)
	if err != nil {
		sklog.Fatal(err)
	}

	repos, err := repograph.NewMap(common.PUBLIC_REPOS, *workdir)
	if err != nil {
		sklog.Fatal(err)
	}

	w, err := window.New(48*time.Hour, 35, repos)
	if err != nil {
		sklog.Fatal(err)
	}

	tCache, err := db.NewTaskCache(taskDb, w)
	if err != nil {
		sklog.Fatal(err)
	}

	end := time.Now()
	start := end.Add(-48 * time.Hour)

	// Analyze flakiness.
	results, err := analysis.Analyze(tCache, start, end, repos)
	if err != nil {
		sklog.Fatal(err)
	}

	sklog.Infof("Results:")
	printResult("Definitely Flaky", results.DefinitelyFlaky)
	printResult("Maybe Flaky", results.MaybeFlaky)
	printResult("Infra Failures", results.InfraFailures)
}
