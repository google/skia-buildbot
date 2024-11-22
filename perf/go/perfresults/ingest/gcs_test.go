package ingest

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/perf/go/perfresults"
)

func Test_ConvertTime_UTC(t *testing.T) {
	loc := time.FixedZone("UTC", 0)

	// check padding zeros
	assert.EqualValues(t, convertTime(time.Date(2024, time.May, 2, 2, 59, 59, 10, loc)), "2024/05/02/02")

	// check normal case
	assert.EqualValues(t, convertTime(time.Date(2023, time.October, 21, 13, 59, 59, 10, loc)), "2023/10/21/13")

	// check leap month
	assert.EqualValues(t, convertTime(time.Date(2024, time.February, 29, 12, 0, 0, 0, loc)), "2024/02/29/12")

	// check daylight saving
	assert.EqualValues(t, convertTime(time.Date(2023, time.March, 12, 2, 15, 0, 0, loc)), "2023/03/12/02")
}

func Test_ConvertTime_GMT_Minus_8(t *testing.T) {
	loc := time.FixedZone("UTC", -8*60*60)

	// 6:59 GMT-8 -> 14:59 UTC
	assert.EqualValues(t, convertTime(time.Date(2024, time.May, 2, 6, 59, 59, 10, loc)), "2024/05/02/14")

	// 19:50 GMT-8 -> 3:50 UTC next day
	assert.EqualValues(t, convertTime(time.Date(2024, time.May, 2, 19, 50, 59, 10, loc)), "2024/05/03/03")
}

func Test_ConvertBuildInfo_Empty(t *testing.T) {
	assert.EqualValues(t, convertBuildInfo(perfresults.BuildInfo{}), "ChromiumPerf/BuilderNone")
}

func Test_ConvertBuildInfo(t *testing.T) {
	assert.EqualValues(t, convertBuildInfo(perfresults.BuildInfo{
		BuilderName:  "android-builder-perf",
		MachineGroup: "ChromiumPerf",
	}), "ChromiumPerf/android-builder-perf")

	assert.EqualValues(t, convertBuildInfo(perfresults.BuildInfo{
		BuilderName:  "android-pixel6-perf-pgo",
		MachineGroup: "ChromiumPerfPGO",
	}), "ChromiumPerfPGO/android-pixel6-perf-pgo")
}

func Test_ConvertPath(t *testing.T) {
	ti := time.Date(2024, time.May, 2, 2, 59, 59, 10, time.FixedZone("UTC", 0))
	bi := perfresults.BuildInfo{
		BuilderName:  "android-go-wembley-perf",
		MachineGroup: "ChromiumPerf",
	}
	assert.EqualValues(t, convertPath(ti, bi, "jetstream2"), "gs://chrome-perf-non-public/ingest/2024/05/02/02/ChromiumPerf/android-go-wembley-perf/jetstream2")

	bi = perfresults.BuildInfo{
		BuilderName:  "linux-perf",
		MachineGroup: "ChromiumPerf",
	}
	assert.EqualValues(t, convertPath(ti, bi, "speedometer3"), "gs://chrome-perf-public/ingest/2024/05/02/02/ChromiumPerf/linux-perf/speedometer3")
}
