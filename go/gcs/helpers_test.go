package gcs_test

import (
	"context"
	"errors"
	"io"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/gcs/test_gcsclient"
	"go.skia.org/infra/go/testutils/unittest"
)

// captureFileWriterGCSClient captures FileWriter args for TestWithWriteFile* and
// TestWithWriteFileGzip*.
type captureFileWriterGCSClient struct {
	*test_gcsclient.MemoryGCSClient
	fileWriterCtx  context.Context
	fileWriterOpts gcs.FileWriteOptions
}

func (c *captureFileWriterGCSClient) FileWriter(ctx context.Context, path string, opts gcs.FileWriteOptions) io.WriteCloser {
	c.fileWriterCtx = ctx
	c.fileWriterOpts = opts
	return c.MemoryGCSClient.FileWriter(ctx, path, opts)
}

func TestWithWriteFileSimple(t *testing.T) {
	unittest.SmallTest(t)

	c := &captureFileWriterGCSClient{
		MemoryGCSClient: test_gcsclient.NewMemoryClient("compositions"),
	}

	ctx := context.Background()
	opts := gcs.FileWriteOptions{
		ContentType: "text/plain",
	}
	const path = "story"
	const contents = "Once upon a time..."
	assert.NoError(t, gcs.WithWriteFile(c, ctx, path, opts, func(w io.Writer) error {
		_, err := w.Write([]byte(contents))
		return err
	}))
	// The context should be canceled.
	assert.Equal(t, context.Canceled, c.fileWriterCtx.Err())
	assert.Equal(t, opts, c.fileWriterOpts)
	actualContents, err := c.GetFileContents(ctx, path)
	assert.NoError(t, err)
	assert.Equal(t, []byte(contents), actualContents)
}

func TestWithWriteFileError(t *testing.T) {
	unittest.SmallTest(t)

	c := &captureFileWriterGCSClient{
		MemoryGCSClient: test_gcsclient.NewMemoryClient("compositions"),
	}

	ctx := context.Background()
	opts := gcs.FileWriteOptions{
		ContentType: "text/plain",
	}
	const path = "the-neverstarting-story"
	err := errors.New("I can't remember how it starts.")
	assert.Equal(t, gcs.WithWriteFile(c, ctx, path, opts, func(w io.Writer) error {
		return err
	}), err)
	// The context should be canceled.
	assert.Equal(t, context.Canceled, c.fileWriterCtx.Err())
	assert.Equal(t, opts, c.fileWriterOpts)
	exists, err := c.DoesFileExist(ctx, path)
	assert.NoError(t, err)
	assert.False(t, exists)
}

func TestWithWriteFileGzipSimple(t *testing.T) {
	unittest.SmallTest(t)

	c := &captureFileWriterGCSClient{
		MemoryGCSClient: test_gcsclient.NewMemoryClient("compositions"),
	}

	ctx := context.Background()
	const path = "condensible-story"
	const contents = "So like there was like this one time that I was like totally like..."
	assert.NoError(t, gcs.WithWriteFileGzip(c, ctx, path, func(w io.Writer) error {
		_, err := w.Write([]byte(contents))
		return err
	}))
	// The context should be canceled.
	assert.Equal(t, context.Canceled, c.fileWriterCtx.Err())
	assert.Equal(t, gcs.FileWriteOptions{
		ContentEncoding: "gzip",
	}, c.fileWriterOpts)
	actualContents, err := c.GetFileContents(ctx, path)
	assert.NoError(t, err)
	assert.Equal(t, []byte(contents), actualContents)
}
