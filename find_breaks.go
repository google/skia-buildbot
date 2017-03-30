package main

import (
	"flag"
	"os"
	"path"
	"path/filepath"
	"time"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/status/go/find_breaks"
	"go.skia.org/infra/task_scheduler/go/db/remote_db"
)

// flags
var (
	taskSchedulerDbUrl = flag.String("task_db_url", "http://skia-task-scheduler:8008/db/", "Where the Skia task scheduler database is hosted.")
	workdir            = flag.String("workdir", ".", "Directory to use for scratch work.")
)

func main() {
	common.Init()

	taskDb, err := remote_db.NewClient(*taskSchedulerDbUrl)
	if err != nil {
		sklog.Fatal(err)
	}

	wd, err := filepath.Abs(*workdir)
	if err != nil {
		sklog.Fatal(err)
	}
	reposDir := path.Join(wd, "repos")
	if err := os.MkdirAll(reposDir, os.ModePerm); err != nil {
		sklog.Fatal(err)
	}
	repo, err := repograph.NewGraph(common.REPO_SKIA, reposDir)
	if err != nil {
		sklog.Fatal(err)
	}
	sklog.Info("Checkout complete")

	end := time.Now()
	start := end.Add(-24 * time.Hour)
	g, err := find_breaks.FindFailureGroups(repo, taskDb, start, end)
	if err != nil {
		sklog.Fatal(err)
	}
	for _, group := range g {
		sklog.Errorf("Failure group:")
		sklog.Errorf("IDs:")
		for _, id := range group.Ids {
			sklog.Errorf("\t%s", id)
		}
		sklog.Errorf("Broke in:")
		for _, c := range group.BrokeIn {
			sklog.Errorf("\t%s", c)
		}
		sklog.Errorf("Fixed in:")
		for _, c := range group.FixedIn {
			sklog.Errorf("\t%s", c)
		}
		sklog.Errorf("")
	}
}
