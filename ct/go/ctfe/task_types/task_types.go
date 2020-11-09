/*
	List of all task types.
*/

package task_types

import (
	"go.skia.org/infra/ct/go/ctfe/admin_tasks"
	"go.skia.org/infra/ct/go/ctfe/chromium_analysis"
	"go.skia.org/infra/ct/go/ctfe/chromium_perf"
	"go.skia.org/infra/ct/go/ctfe/metrics_analysis"
	"go.skia.org/infra/ct/go/ctfe/task_common"
)

// Slice of all tasks supported by CTFE.
func Prototypes() []task_common.Task {
	return []task_common.Task{
		&admin_tasks.RecreatePageSetsDatastoreTask{},
		&admin_tasks.RecreateWebpageArchivesDatastoreTask{},
		&chromium_analysis.DatastoreTask{},
		&chromium_perf.DatastoreTask{},
		&metrics_analysis.DatastoreTask{},
	}
}
