package pending_tasks

import (
	"bytes"
	"testing"

	expect "github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/ct/go/ctfe/admin_tasks"
	"go.skia.org/infra/ct/go/ctfe/chromium_perf"
	"go.skia.org/infra/ct/go/ctfe/task_common"
	"go.skia.org/infra/go/ds"
)

func getCommonCols(kind ds.Kind) task_common.CommonCols {
	return task_common.CommonCols{
		TsAdded:         20080726180513,
		TsStarted:       20091011121314,
		TsCompleted:     20150106171819,
		Username:        "nobody@chromium.org",
		Failure:         false,
		RepeatAfterDays: 2,
	}
}

func TestEncodeTaskDecodeTaskRoundTrip(t *testing.T) {
	test := func(task task_common.Task) {
		buf := bytes.Buffer{}
		require.NoError(t, EncodeTask(&buf, task))
		newTask, err := DecodeTask(&buf)
		require.NoError(t, err)
		expect.Equal(t, task, newTask)
	}
	test(&chromium_perf.ChromiumPerfDatastoreTask{
		CommonCols:           getCommonCols(ds.CHROMIUM_PERF_TASKS),
		Benchmark:            "benchmark",
		Platform:             "Linux",
		PageSets:             "All",
		RepeatRuns:           1,
		BenchmarkArgs:        "benchmarkargs",
		BrowserArgsNoPatch:   "banp",
		BrowserArgsWithPatch: "bawp",
		Description:          "description",
		ChromiumPatchGSPath:  "patches/abc.patch",
		SkiaPatchGSPath:      "patches/xyz.patch",
	})
	test(&admin_tasks.RecreatePageSetsDatastoreTask{
		AdminDatastoreTask: admin_tasks.AdminDatastoreTask{
			CommonCols: getCommonCols(ds.RECREATE_PAGESETS_TASKS),
			PageSets:   "All",
		},
	})
	test(&admin_tasks.RecreateWebpageArchivesDatastoreTask{
		AdminDatastoreTask: admin_tasks.AdminDatastoreTask{
			CommonCols: getCommonCols(ds.RECREATE_WEBPAGE_ARCHIVES_TASKS),
			PageSets:   "All",
		},
	})
}
