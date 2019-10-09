package perfclient

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/gcs/test_gcsclient"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/perf/go/ingestcommon"
)

func TestHappyCase(t *testing.T) {
	unittest.SmallTest(t)

	ms := test_gcsclient.NewMockClient()
	defer ms.AssertExpectations(t)
	pc := New("/foobar", ms)

	data := ingestcommon.BenchData{
		Hash:     "1234",
		Issue:    "alpha",
		PatchSet: "Beta",
		Key: map[string]string{
			"arch": "Frob",
		},
		Results: map[string]ingestcommon.BenchResults{
			"SomeTestNamePrime": {
				"task_duration": {
					"task_ms": 500000,
				},
			},
		},
	}

	expected, err := json.Marshal(data)
	require.NoError(t, err)
	compressed := bytes.Buffer{}
	cw := gzip.NewWriter(&compressed)
	_, err = cw.Write(expected)
	require.NoError(t, cw.Close())
	require.NoError(t, err)

	ms.On("SetFileContents", testutils.AnyContext, "/foobar/2017/09/01/13/MyTest-Debug/testprefix_b7e46f46f13e9ddfa40cdb44f921efd1_1504273020000.json", gcs.FileWriteOptions{
		ContentEncoding: "gzip",
		ContentType:     "application/json",
	}, compressed.Bytes()).Return(nil)

	now := time.Date(2017, 9, 1, 13, 37, 0, 0, time.UTC)

	require.NoError(t, pc.PushToPerf(now, "MyTest-Debug", "testprefix", data))
}
