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
	"go.skia.org/infra/ct/go/ctfe/pixel_diff"
	"go.skia.org/infra/ct/go/ctfe/task_common"
)

// Slice of all tasks supported by CTFE.
func Prototypes() []task_common.Task {
	return []task_common.Task{
		&admin_tasks.RecreatePageSetsDBTask{},
		&admin_tasks.RecreateWebpageArchivesDBTask{},
		&capture_skps.DBTask{},
		&chromium_analysis.DBTask{},
		&chromium_builds.DBTask{},
		&chromium_perf.DBTask{},
		&lua_scripts.DBTask{},
		&metrics_analysis.DBTask{},
		&pixel_diff.DBTask{},
	}
}
