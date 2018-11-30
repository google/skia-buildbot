// Read Job GOBs from GCS and write as JSON.
//
// Example:
//   gs_jobs_viewer
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"time"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/task_scheduler/go/db/recovery"
	"go.skia.org/infra/task_scheduler/go/types"
	"google.golang.org/api/option"
)

var (
	gsBucket = flag.String("bucket", "skia-task-scheduler", "GCS bucket to read.")
	period   = flag.Duration("period", 24*time.Hour, "Duration of time range to read.")
)

func main() {

	// Global init.
	common.Init()

	// Authenticated HTTP client.
	ts, err := auth.NewDefaultLegacyTokenSource(true, auth.SCOPE_READ_ONLY)
	if err != nil {
		sklog.Fatal(err)
	}
	httpClient := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()

	ctx := context.Background()
	gsClient, err := storage.NewClient(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		sklog.Fatal(err)
	}

	begin := time.Now().UTC().Add(-*period)

	sklog.Infof("Reading jobs since %s...", begin)
	jobsMap, err := recovery.RetrieveJobs(ctx, begin, gsClient, *gsBucket)
	if err != nil {
		sklog.Fatal(err)
	}

	jobs := make([]*types.Job, 0, len(jobsMap))
	for _, job := range jobsMap {
		jobs = append(jobs, job)
	}
	sort.Sort(types.JobSlice(jobs))

	v := struct {
		Begin time.Time
		Jobs  []*types.Job
	}{
		Begin: begin,
		Jobs:  jobs,
	}

	enc, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		sklog.Fatal(err)
	}
	if _, err := os.Stdout.Write(enc); err != nil {
		sklog.Fatal(err)
	}
	if _, err := fmt.Fprintln(os.Stdout); err != nil {
		sklog.Fatal(err)
	}
}
