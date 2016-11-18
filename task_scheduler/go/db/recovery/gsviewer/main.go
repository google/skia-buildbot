// Read Job GOBs from GS and write as JSON.
//
// Example:
//   gsviewer --output -
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"time"

	"cloud.google.com/go/storage"
	"golang.org/x/net/context"
	"google.golang.org/api/option"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/db/recovery"
)

var (
	gsBucket = flag.String("bucket", "skia-task-scheduler", "GS bucket to read.")
	period   = flag.Duration("period", 24*time.Hour, "Duration of time range to read.")
	output   = flag.String("output", "", "Output filename or - for stdout.")
)

func main() {
	defer common.LogPanic()

	// Global init.
	common.Init()

	// Authenticated HTTP client.
	httpClient, err := auth.NewClient(true, "", auth.SCOPE_READ_ONLY)
	if err != nil {
		glog.Fatal(err)
	}

	ctx := context.Background()
	gsClient, err := storage.NewClient(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		glog.Fatal(err)
	}

	var outW io.Writer
	if *output == "" {
		glog.Fatalf("Must specify --output. Use - for stdout.")
	} else if *output == "-" {
		outW = os.Stdout
	} else {
		outW, err = os.Create(*output)
		if err != nil {
			glog.Fatal(err)
		}
	}

	begin := time.Now().UTC().Add(-*period)

	glog.Infof("Reading jobs since %s...", begin)
	jobsMap, err := recovery.RetrieveJobs(ctx, begin, gsClient, *gsBucket)
	if err != nil {
		glog.Fatal(err)
	}

	jobs := make([]*db.Job, 0, len(jobsMap))
	for _, job := range jobsMap {
		jobs = append(jobs, job)
	}
	sort.Sort(db.JobSlice(jobs))

	v := struct {
		Begin time.Time
		Jobs  []*db.Job
	}{
		Begin: begin,
		Jobs:  jobs,
	}

	enc, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		glog.Fatal(err)
	}
	if _, err := outW.Write(enc); err != nil {
		glog.Fatal(err)
	}
	fmt.Fprintln(outW)
}
