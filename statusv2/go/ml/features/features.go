package features

/*
   Perform feature extraction for Swarming tasks.
*/

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"path"
	"time"

	"go.skia.org/infra/go/dataproc"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/db"
)

// ExtractRange extracts features for tasks within the given time range.
// Downloads all tasks in range from the task DB, stores in a convenient format
// and calls into PySpark to perform the actual work.
func ExtractRange(d db.TaskReader, start, end time.Time) error {
	tasks, err := d.GetTasksFromDateRange(start, end)
	if err != nil {
		return fmt.Errorf("Failed to retrieve tasks: %s", err)
	}
	sklog.Infof("Found %d tasks from %s to %s", len(tasks), start, end)
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
}
