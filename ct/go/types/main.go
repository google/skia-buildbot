// Program to generate TypeScript definition files for Golang structs that are
// serialized to JSON for the web UI.
//
//go:generate go run . -o ../../modules/json/index.ts
package main

import (
	"flag"
	"io"

	"github.com/skia-dev/go2ts"

	"go.skia.org/infra/ct/go/ctfe/admin_tasks"
	"go.skia.org/infra/ct/go/ctfe/chromium_analysis"
	"go.skia.org/infra/ct/go/ctfe/chromium_perf"
	"go.skia.org/infra/ct/go/ctfe/metrics_analysis"
	"go.skia.org/infra/ct/go/ctfe/pending_tasks"
	"go.skia.org/infra/ct/go/ctfe/task_common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

func main() {
	var outputPath = flag.String("o", "", "Path to the output TypeScript file.")
	flag.Parse()

	generator := go2ts.New()
	generator.AddMultiple(
		httputils.ResponsePagination{},
		task_common.RedoTaskRequest{},
		task_common.DeleteTaskRequest{},
		task_common.CommonCols{},
		task_common.GetTasksResponse{},
		task_common.BenchmarksPlatformsResponse{},
		task_common.TaskPrioritiesResponse{},
		task_common.PageSet{},
		task_common.CLDataResponse{},

		pending_tasks.CompletedTaskResponse{},

		admin_tasks.AdminDatastoreTask{},
		admin_tasks.AdminAddTaskVars{},

		chromium_analysis.ChromiumAnalysisDatastoreTask{},
		chromium_analysis.ChromiumAnalysisAddTaskVars{},

		chromium_perf.ChromiumPerfDatastoreTask{},
		chromium_perf.ChromiumPerfAddTaskVars{},

		metrics_analysis.MetricsAnalysisDatastoreTask{},
		metrics_analysis.MetricsAnalysisAddTaskVars{},
	)

	err := util.WithWriteFile(*outputPath, func(w io.Writer) error {
		return generator.Render(w)
	})
	if err != nil {
		sklog.Fatal(err)
	}
}
