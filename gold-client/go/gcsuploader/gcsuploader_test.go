package gcsuploader

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/exec"
)

func TestGSutil_UploadBytes_Success(t *testing.T) {

	cc := exec.CommandCollector{}
	ctx := exec.NewContext(context.Background(), cc.Run)

	gu := GsutilImpl{}
	err := gu.UploadBytes(ctx, nil, "/path/to/file", "gs://bucket/foo/bar")
	require.NoError(t, err)
	require.Len(t, cc.Commands(), 1)
	assert.Equal(t, "gsutil cp /path/to/file gs://bucket/foo/bar", exec.DebugString(cc.Commands()[0]))
}

func TestGSutil_UploadJSON_Success(t *testing.T) {

	wd := t.TempDir()
	tf := filepath.Join(wd, "foo.json")

	cc := exec.CommandCollector{}
	ctx := exec.NewContext(context.Background(), cc.Run)

	type testJSON struct {
		One string
	}

	gu := GsutilImpl{}
	err := gu.UploadJSON(ctx, testJSON{One: "alpha"}, tf, "gs://bucket/foo/bar.json")
	require.NoError(t, err)
	require.Len(t, cc.Commands(), 1)
	expectedCmd := fmt.Sprintf("gsutil cp %s gs://gs://bucket/foo/bar.json", tf)
	assert.Equal(t, expectedCmd, exec.DebugString(cc.Commands()[0]))

	b, err := ioutil.ReadFile(tf)
	require.NoError(t, err)
	assert.Equal(t, `{"One":"alpha"}`, string(b))
}
