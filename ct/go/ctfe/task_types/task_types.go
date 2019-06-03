/*
	List of all task types.
*/

package task_types

import (
	"go.skia.org/infra/ct/go/ctfe/admin_tasks"
	"go.skia.org/infra/ct/go/ctfe/capture_skps"
	"go.skia.org/infra/ct/go/ctfe/chromium_analysis"
	"go.skia.org/infra/ct/go/ctfe/chromium_builds"
	"go.skia.org/infra/ct/go/ctfe/chromium_perf"
	"go.skia.org/infra/ct/go/ctfe/lua_scripts"
	"go.skia.org/infra/ct/go/ctfe/metrics_analysis"
	"go.skia.org/infra/ct/go/ctfe/task_common"
)

// Slice of all tasks supported by CTFE.
func Prototypes() []task_common.Task {
	return []task_common.Task{
		&admin_tasks.RecreatePageSetsDatastoreTask{},
		&admin_tasks.RecreateWebpageArchivesDatastoreTask{},
		&capture_skps.DatastoreTask{},
		&chromium_analysis.DatastoreTask{},
		&chromium_builds.DatastoreTask{},
		&chromium_perf.DatastoreTask{},
		&lua_scripts.DatastoreTask{},
		&metrics_analysis.DatastoreTask{},
	}
}
