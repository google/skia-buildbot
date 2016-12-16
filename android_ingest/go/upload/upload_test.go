package upload

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/perf/go/ingestcommon"
)

func TestObjectPath(t *testing.T) {
	testutils.SmallTest(t)
	b := &ingestcommon.BenchData{
		Hash: "8dcc84f7dc8523dd90501a4feb1f632808337c34",
		Key: map[string]string{
			"build_flavor": "marlin-userdebug",
		},
	}

	now := time.Date(2016, time.December, 16, 23, 0, 0, 0, time.UTC)
	path := ObjectPath(b, "android-ingest", now)
	assert.Equal(t, "android-ingest/2016/12/16/23/8dcc84f7dc8523dd90501a4feb1f632808337c34_build_flavor_marlin-userdebug.json", path)
}
