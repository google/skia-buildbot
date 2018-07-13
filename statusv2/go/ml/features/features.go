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
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/statusv2/go/ml/processor"
	"go.skia.org/infra/task_scheduler/go/db"
)

// ExtractRangeV0 extracts features for tasks within the given time range.
// Downloads all tasks in range from the task DB, stores in a convenient format
// and calls into PySpark to perform the actual work.
func ExtractRangeV0(ctx context.Context, d db.TaskReader, start, end time.Time) error {
	tasks, err := d.GetTasksFromDateRange(start, end, "")
	if err != nil {
		return fmt.Errorf("Failed to retrieve tasks: %s", err)
	}
	sklog.Infof("Found %d tasks from %s to %s", len(tasks), start, end)
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
		PyFile:  "extract_features_v0.py",
		Files:   []string{tasksJson},
		Args:    []string{"--tasks-json", "tasks.json"},
		Cluster: dataproc.CLUSTER_SKIA,
	}
	out, err := job.Run(ctx)
	sklog.Infof("Output from job:\n%s", out)
	return err
}

func StartV0(ctx context.Context, workdir string, d db.TaskReader) error {
	p := processor.Processor{
		BeginningOfTime: time.Date(2016, time.September, 1, 0, 0, 0, 0, time.UTC),
		ChunkSize:       time.Hour,
		Name:            "swarming_log_feature_extraction_v0",
		Frequency:       time.Hour,
		ProcessFn: func(ctx context.Context, start, end time.Time) error {
			return ExtractRangeV0(ctx, d, start, end)
		},
		Window:  24 * time.Hour,
		Workdir: workdir,
	}
	return p.Start(ctx)
}
