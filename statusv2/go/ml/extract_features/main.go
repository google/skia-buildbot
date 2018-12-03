package main

import (
	"context"
	"flag"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/statusv2/go/ml/features"
	"go.skia.org/infra/task_scheduler/go/db/pubsub"
	"go.skia.org/infra/task_scheduler/go/db/remote_db"
)

var (
	// Flags.
	local              = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	taskSchedulerDbUrl = flag.String("task_db_url", "http://skia-task-scheduler:8008/db/", "Where the Skia task scheduler database is hosted.")
	workdir            = flag.String("workdir", ".", "Working directory.")
	tasksPubsubTopic   = flag.String("pubsub_topic_tasks", pubsub.TOPIC_TASKS, "Pubsub topic for tasks.")
	jobsPubsubTopic    = flag.String("pubsub_topic_jobs", pubsub.TOPIC_JOBS, "Pubsub topic for jobs.")
)

func main() {
	common.Init()

	ctx := context.Background()
	ts, err := auth.NewDefaultTokenSource(*local, pubsub.AUTH_SCOPE)
	if err != nil {
		sklog.Fatal(err)
	}
	d, err := remote_db.NewClient(*taskSchedulerDbUrl, *tasksPubsubTopic, *jobsPubsubTopic, "statusv2-feature-extraction", ts)
	if err != nil {
		sklog.Fatal(err)
	}
	if err := features.StartV0(ctx, *workdir, d); err != nil {
		sklog.Fatal(err)
	}
	select {}
}
