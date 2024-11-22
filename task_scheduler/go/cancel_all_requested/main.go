package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	"cloud.google.com/go/datastore"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/db/firestore"
	"go.skia.org/infra/task_scheduler/go/types"
	"golang.org/x/oauth2/google"
)

var (
	// Flags.
	reason     = flag.String("reason", "", "Reason for canceling the jobs. Required.")
	project    = flag.String("project", firestore.FIRESTORE_PROJECT, "Project in which the Firestore database is housed.")
	instance   = flag.String("instance", "production", "Instance of the database to use.")
	dryRun     = flag.Bool("dry-run", false, "If set, no jobs are cancelled.")
	statusFlag = flag.String("status", string(types.JOB_STATUS_REQUESTED), "Cancel jobs with this status.")
)

func main() {
	flag.Parse()

	if *reason == "" && !*dryRun {
		sklog.Fatal("--reason is required when --dry-run is not set.")
	}
	if *project == "" {
		sklog.Fatal("--project is required.")
	}
	if *instance == "" {
		sklog.Fatal("--instance is required.")
	}

	ctx := context.Background()
	ts, err := google.DefaultTokenSource(ctx, datastore.ScopeDatastore)
	if err != nil {
		sklog.Fatalf("Failed to create token source: %s", err)
	}
	tsDb, err := firestore.NewDBWithParams(ctx, *project, *instance, ts)
	if err != nil {
		sklog.Fatal(err)
	}
	status := types.JobStatus(*statusFlag)
	params := &db.JobSearchParams{
		Status: &status,
	}
	now := time.Now()
	for {
		jobs, err := tsDb.SearchJobs(ctx, params)
		if err != nil {
			sklog.Fatal(err)
		}
		if *dryRun {
			fmt.Printf("--dry-run was set, not canceling %d jobs.\n", len(jobs))
			// Have to break here, since we're not shrinking the search results.
			break
		}
		if len(jobs) == 0 {
			break
		}
		for _, job := range jobs {
			job.Finished = now
			job.Status = types.JOB_STATUS_CANCELED
			job.StatusDetails = *reason
		}
		if err := tsDb.PutJobsInChunks(ctx, jobs); err != nil {
			sklog.Fatal(err)
		}
	}
}
