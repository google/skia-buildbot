package main

import (
	"flag"
	"os"
	"time"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/statusv2/go/ml/features"
	"go.skia.org/infra/task_scheduler/go/db/remote_db"
)

// flags
var (
	taskSchedulerDbUrl = flag.String("task_db_url", "http://skia-task-scheduler:8008/db/", "Where the Skia task scheduler database is hosted.")
	workdir            = flag.String("workdir", os.TempDir(), "Working directory. Optional, but recommended not to use CWD.")
)

func main() {
	common.Init()

	d, err := remote_db.NewClient(*taskSchedulerDbUrl, nil)
	if err != nil {
		sklog.Fatal(err)
	}

	start := time.Date(2017, time.October, 9, 15, 0, 0, 0, time.UTC)
	end := start.Add(60 * time.Minute)

	err = features.ExtractRange(d, start, end)
	if err != nil {
		sklog.Fatal(err)
	}
}
