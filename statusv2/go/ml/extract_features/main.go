package main

import (
	"context"
	"flag"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/statusv2/go/ml/features"
	"go.skia.org/infra/task_scheduler/go/db/remote_db"
)

var (
	// Flags.
	taskSchedulerDbUrl = flag.String("task_db_url", "http://skia-task-scheduler:8008/db/", "Where the Skia task scheduler database is hosted.")
	workdir            = flag.String("workdir", ".", "Working directory.")
)

func main() {
	common.Init()

	ctx := context.Background()
	d, err := remote_db.NewClient(*taskSchedulerDbUrl, httputils.NewTimeoutClient())
	if err != nil {
		sklog.Fatal(err)
	}
	if err := features.StartV0(ctx, *workdir, d); err != nil {
		sklog.Fatal(err)
	}
	select {}
}
