/*
	List of all task types.
*/

package task_types

import (
	"go.skia.org/infra/ct/go/ctfe/admin_tasks"
	"go.skia.org/infra/ct/go/ctfe/capture_skps"
	"go.skia.org/infra/ct/go/ctfe/chromium_builds"
	"go.skia.org/infra/ct/go/ctfe/chromium_perf"
	"go.skia.org/infra/ct/go/ctfe/lua_scripts"
	"go.skia.org/infra/ct/go/ctfe/task_common"
)

// Slice of all tasks supported by CTFE.
func Prototypes() []task_common.Task {
	return []task_common.Task{
		&chromium_perf.DBTask{},
		&capture_skps.DBTask{},
		&lua_scripts.DBTask{},
		&chromium_builds.DBTask{},
		&admin_tasks.RecreatePageSetsDBTask{},
		&admin_tasks.RecreateWebpageArchivesDBTask{},
	}
}
