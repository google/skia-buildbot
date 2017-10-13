package features

/*
   Perform feature extraction for Swarming tasks.
*/

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"path"
	"time"

	"go.skia.org/infra/go/dataproc"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/db"
)

// ExtractRange extracts features for tasks within the given time range.
// Downloads all tasks in range from the task DB, stores in a convenient format
// and calls into PySpark to perform the actual work.
func ExtractRange(d db.TaskReader, start, end time.Time) error {
	return util.IterTimeChunks(start, end, time.Hour, func(chunkStart, chunkEnd time.Time) error {
		tasks, err := d.GetTasksFromDateRange(chunkStart, chunkEnd)
		if err != nil {
			return fmt.Errorf("Failed to retrieve tasks: %s", err)
		}
		sklog.Infof("Found %d tasks from %s to %s", len(tasks), chunkStart, chunkEnd)
		if len(tasks) == 0 {
			return nil
		}
		workdir, err := ioutil.TempDir("", "")
		if err != nil {
			return fmt.Errorf("Failed to create temporary dir: %s", err)
		}
		defer util.RemoveAll(workdir)
		tasksJson := path.Join(workdir, "tasks.json")
		if err := util.WithWriteFile(tasksJson, func(w io.Writer) error {
			return json.NewEncoder(w).Encode(tasks)
		}); err != nil {
			return fmt.Errorf("Failed to write JSON file: %s", err)
		}
		job := &dataproc.PySparkJob{
			PyFile:  "extract_features.py",
			Files:   []string{tasksJson},
			Args:    []string{"--tasks-json", "tasks.json"},
			Cluster: dataproc.CLUSTER_SKIA,
		}
		out, err := job.Run()
		sklog.Infof("Output from job:\n%s", out)
		return err
	})
}

// Extract features for tasks periodically.
func ExtractPeriodically(ctx context.Context, d db.TaskReader) {
	lv := metrics2.NewLiveness("last_successful_swarming_log_feature_extraction")
	go util.RepeatCtx(time.Hour, ctx, func() {
		end := time.Now()
		start := end.Add(-24 * time.Hour)
		if err := ExtractRange(d, start, end); err != nil {
			sklog.Errorf("Failed to extract Swarming log features: %s", err)
		} else {
			lv.Reset()
		}
	})
}

// Extract features for all logs since the "beginning of time". This should only
// ever be run once, after which ExtractPeriodically should be used.
func InitialExtract(d db.TaskReader) error {
	start := time.Date(2016, time.September, 1, 0, 0, 0, 0, time.UTC)
	end := time.Now()
	return ExtractRange(d, start, end)
}
