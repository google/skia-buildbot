// Read data from a local task scheduler DB file and output as JSON.
//
// Example:
//   dbviewer --db /path/to/task_scheduler.bdb
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/task_scheduler/go/db/local_db"
	"go.skia.org/infra/task_scheduler/go/types"
)

var (
	dbfile   = flag.String("db", local_db.DB_FILENAME, "DB to read.")
	period   = flag.Duration("period", 24*time.Hour, "Duration of time range to read.")
	beginStr = flag.String("begin", "", "Beginning of time range to read; default (now - period). Format is "+time.RFC3339+".")
)

func main() {

	// Global init.
	common.Init()

	d, err := local_db.NewDB(local_db.DB_NAME, *dbfile, nil, nil)
	if err != nil {
		sklog.Fatal(err)
	}

	begin := time.Now().UTC().Add(-*period)
	if *beginStr != "" {
		parsed, err := time.Parse(time.RFC3339, *beginStr)
		if err != nil {
			sklog.Fatal(err)
		}
		begin = parsed.UTC()
	}
	end := begin.Add(*period)

	sklog.Infof("Reading tasks from %s to %s...", begin, end)
	tasks, err := d.GetTasksFromDateRange(begin, end, "")
	if err != nil {
		sklog.Fatal(err)
	}

	sklog.Infof("Reading jobs from %s to %s...", begin, end)
	jobs, err := d.GetJobsFromDateRange(begin, end)
	if err != nil {
		sklog.Fatal(err)
	}

	v := struct {
		Begin time.Time
		End   time.Time
		Tasks []*types.Task
		Jobs  []*types.Job
	}{
		Begin: begin,
		End:   end,
		Tasks: tasks,
		Jobs:  jobs,
	}

	enc, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		sklog.Fatal(err)
	}
	if _, err := os.Stdout.Write(enc); err != nil {
		sklog.Fatal(err)
	}
	fmt.Println()
}
