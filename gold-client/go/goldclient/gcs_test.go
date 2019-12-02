package goldclient

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestGSutilUploadBytes(t *testing.T) {
	unittest.SmallTest(t)

	mcc := exec.CommandCollector{}
	ctx := exec.NewContext(context.Background(), mcc.Run)

	gu := gsutilImpl{}
	err := gu.UploadBytes(ctx, nil, "/path/to/file", "gs://bucket/foo/bar")
	require.NoError(t, err)
	require.Len(t, mcc.Commands(), 1)
	assert.Equal(t, "gsutil cp /path/to/file gs://bucket/foo/bar", exec.DebugString(mcc.Commands()[0]))
}

func TestGSutilDownload(t *testing.T) {
	unittest.MediumTest(t)

	wd, cleanup := testutils.TempDir(t)
	defer cleanup()

	// Since we don't actually download something, write something to disk to pretend the gsutil
	// command worked.
	tf := filepath.Join(wd, "temp.png")
	const fakeData = "an image"
	require.NoError(t, ioutil.WriteFile(tf, []byte(fakeData), 0666))

	mcc := exec.CommandCollector{}
	ctx := exec.NewContext(context.Background(), mcc.Run)

	gu := gsutilImpl{}
	b, err := gu.Download(ctx, "gs://bucket/foo/bar.png", wd )
	require.NoError(t, err)
	require.NotNil(t, b)
	require.Len(t, mcc.Commands(), 1)
	assert.Equal(t, "gsutil cp gs://bucket/foo/bar.png "+tf, exec.DebugString(mcc.Commands()[0]))
	assert.Equal(t, []byte(fakeData), b)
}