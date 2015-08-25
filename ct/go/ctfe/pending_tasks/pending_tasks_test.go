package pending_tasks

import (
	"bytes"
	"database/sql"
	"testing"

	"go.skia.org/infra/ct/go/ctfe/admin_tasks"
	"go.skia.org/infra/ct/go/ctfe/capture_skps"
	"go.skia.org/infra/ct/go/ctfe/chromium_builds"
	"go.skia.org/infra/ct/go/ctfe/chromium_perf"
	"go.skia.org/infra/ct/go/ctfe/lua_scripts"
	"go.skia.org/infra/ct/go/ctfe/task_common"

	expect "github.com/stretchr/testify/assert"
	assert "github.com/stretchr/testify/require"
)

func TestEncodeTaskDecodeTaskRoundTrip(t *testing.T) {
	test := func(task task_common.Task) {
		buf := bytes.Buffer{}
		assert.NoError(t, EncodeTask(&buf, task))
		newTask, err := DecodeTask(&buf)
		assert.NoError(t, err)
		expect.Equal(t, task, newTask)
	}
	common := task_common.CommonCols{
		Id:              42,
		TsAdded:         sql.NullInt64{Int64: 20080726180513, Valid: true},
		TsStarted:       sql.NullInt64{Int64: 20091011121314, Valid: true},
		TsCompleted:     sql.NullInt64{Int64: 20150106171819, Valid: true},
		Username:        "nobody@chromium.org",
		Failure:         sql.NullBool{Bool: false, Valid: true},
		RepeatAfterDays: 2,
	}
	test(&chromium_perf.DBTask{
		CommonCols:           common,
		Benchmark:            "benchmark",
		Platform:             "Linux",
		PageSets:             "All",
		RepeatRuns:           1,
		BenchmarkArgs:        "benchmarkargs",
		BrowserArgsNoPatch:   "banp",
		BrowserArgsWithPatch: "bawp",
		Description:          "description",
		ChromiumPatch:        "chromiumpatch",
		BlinkPatch:           "blinkpatch",
		SkiaPatch:            "skiapatch",
	})
	test(&capture_skps.DBTask{
		CommonCols: task_common.CommonCols{
			Id:       17,
			TsAdded:  sql.NullInt64{Int64: 20080726180513, Valid: true},
			Username: "nobody@chromium.org",
		},
		PageSets:    "All",
		ChromiumRev: "c14d891d44f0afff64e56ed7c9702df1d807b1ee",
		SkiaRev:     "586101c79b0490b50623e76c71a5fd67d8d92b08",
		Description: "description",
	})
	test(&lua_scripts.DBTask{
		CommonCols:          common,
		PageSets:            "All",
		ChromiumRev:         "c14d891d44f0afff64e56ed7c9702df1d807b1ee",
		SkiaRev:             "586101c79b0490b50623e76c71a5fd67d8d92b08",
		LuaScript:           `print("lualualua")`,
		LuaAggregatorScript: `print("aaallluuu")`,
		Description:         "description",
		ScriptOutput:        sql.NullString{String: "lualualua", Valid: true},
		AggregatedOutput:    sql.NullString{String: "aaallluuu", Valid: true},
	})
	test(&chromium_builds.DBTask{
		CommonCols:    common,
		ChromiumRev:   "c14d891d44f0afff64e56ed7c9702df1d807b1ee",
		ChromiumRevTs: sql.NullInt64{Int64: 20080726180513, Valid: true},
		SkiaRev:       "586101c79b0490b50623e76c71a5fd67d8d92b08",
	})
	test(&admin_tasks.RecreatePageSetsDBTask{
		CommonCols: common,
		PageSets:   "All",
	})
	test(&admin_tasks.RecreateWebpageArchivesDBTask{
		CommonCols:  common,
		PageSets:    "All",
		ChromiumRev: "c14d891d44f0afff64e56ed7c9702df1d807b1ee",
		SkiaRev:     "586101c79b0490b50623e76c71a5fd67d8d92b08",
	})
}
