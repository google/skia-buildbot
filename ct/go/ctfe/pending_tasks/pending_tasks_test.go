package pending_tasks

import (
	"bytes"
	"testing"

	"cloud.google.com/go/datastore"
	expect "github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/ct/go/ctfe/admin_tasks"
	"go.skia.org/infra/ct/go/ctfe/chromium_perf"
	"go.skia.org/infra/ct/go/ctfe/task_common"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/testutils/unittest"
)

func getCommonCols(kind ds.Kind) task_common.CommonCols {
	return task_common.CommonCols{
		DatastoreKey: &datastore.Key{
			ID:   17,
			Kind: string(kind),
		},
		TsAdded:         20080726180513,
		TsStarted:       20091011121314,
		TsCompleted:     20150106171819,
		Username:        "nobody@chromium.org",
		Failure:         false,
		RepeatAfterDays: 2,
	}
}

func TestEncodeTaskDecodeTaskRoundTrip(t *testing.T) {
	unittest.SmallTest(t)
	test := func(task task_common.Task) {
		buf := bytes.Buffer{}
		require.NoError(t, EncodeTask(&buf, task))
		newTask, err := DecodeTask(&buf)
		require.NoError(t, err)
		expect.Equal(t, task, newTask)
	}
	test(&chromium_perf.DatastoreTask{
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
		CommonCols: getCommonCols(ds.RECREATE_PAGESETS_TASKS),
		PageSets:   "All",
	})
	test(&admin_tasks.RecreateWebpageArchivesDatastoreTask{
		CommonCols:  getCommonCols(ds.RECREATE_WEBPAGE_ARCHIVES_TASKS),
		PageSets:    "All",
		ChromiumRev: "c14d891d44f0afff64e56ed7c9702df1d807b1ee",
		SkiaRev:     "586101c79b0490b50623e76c71a5fd67d8d92b08",
	})
}
