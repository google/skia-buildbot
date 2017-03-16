package main

import (
	"flag"

	"golang.org/x/net/context"

	"go.skia.org/infra/datahopper/go/time_to_bot_coverage"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
)

var (
	taskSchedulerDbUrl = flag.String("task_db_url", "http://skia-task-scheduler:8008/db/", "Where the Skia task scheduler database is hosted.")
	workdir            = flag.String("workdir", ".", "Working directory.")
)

func main() {
	common.Init()
	defer common.LogPanic()

	if err := time_to_bot_coverage.Start(*taskSchedulerDbUrl, *workdir, context.Background()); err != nil {
		sklog.Fatal(err)
	}
	select {}
}
