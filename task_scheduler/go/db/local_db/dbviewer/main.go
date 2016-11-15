package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/db/local_db"
)

var (
	dbfile = flag.String("db", local_db.DB_FILENAME, "DB to read.")
	period = flag.Duration("period", 24*time.Hour, "Duration of time range to read.")
)

func main() {
	defer common.LogPanic()

	// Global init.
	common.Init()

	d, err := local_db.NewDB(local_db.DB_NAME, *dbfile)
	if err != nil {
		glog.Fatal(err)
	}

	end := time.Now().UTC()
	begin := end.Add(-*period)

	glog.Infof("Reading tasks from %s to %s...", begin, end)
	tasks, err := d.GetTasksFromDateRange(begin, end)
	if err != nil {
		glog.Fatal(err)
	}

	glog.Infof("Reading jobs from %s to %s...", begin, end)
	jobs, err := d.GetJobsFromDateRange(begin, end)
	if err != nil {
		glog.Fatal(err)
	}

	v := struct {
		Begin time.Time
		End   time.Time
		Tasks []*db.Task
		Jobs  []*db.Job
	}{
		Begin: begin,
		End:   end,
		Tasks: tasks,
		Jobs:  jobs,
	}

	enc, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		glog.Fatal(err)
	}
	if _, err := os.Stdout.Write(enc); err != nil {
		glog.Fatal(err)
	}
	fmt.Println()
}
