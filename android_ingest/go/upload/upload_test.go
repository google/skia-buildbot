package upload

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/perf/go/ingest/format"
)

func TestObjectPath(t *testing.T) {
	b := &format.BenchData{
		Hash: "8dcc84f7dc8523dd90501a4feb1f632808337c34",
		Key: map[string]string{
			"build_flavor": "marlin-userdebug",
		},
	}

	now := time.Date(2016, time.December, 16, 23, 0, 0, 0, time.UTC)
	path := ObjectPath(b, "android-ingest", now, []byte("{}"))
	assert.Equal(t, "android-ingest/2016/12/16/23/8dcc84f7dc8523dd90501a4feb1f632808337c34_build_flavor_marlin-userdebug_99914b932bd37a50b983c5e7c90ae93b.json", path)
}

func TestLogPath(t *testing.T) {
	now := time.Date(2016, time.December, 16, 23, 0, 0, 0, time.UTC)
	path := LogPath("android-ingest/tx_log/", now, []byte("this is the POST body"))
	assert.Equal(t, "android-ingest/tx_log/2016/12/16/23/caf6a48e3251ed34534aac58b91a877e.json", path)

	// Use the well known md5 empty string hash.
	path = LogPath("android-ingest/tx_log/", now, []byte(""))
	assert.Equal(t, "android-ingest/tx_log/2016/12/16/23/d41d8cd98f00b204e9800998ecf8427e.json", path)
}
