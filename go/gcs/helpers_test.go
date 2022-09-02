package gcs_test

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/gcs/mem_gcsclient"
)

// captureFileWriterGCSClient captures FileWriter args for TestWithWriteFile* and
// TestWithWriteFileGzip*.
type captureFileWriterGCSClient struct {
	*mem_gcsclient.MemoryGCSClient
	fileWriterCtx  context.Context
	fileWriterOpts gcs.FileWriteOptions
}

func (c *captureFileWriterGCSClient) FileWriter(ctx context.Context, path string, opts gcs.FileWriteOptions) io.WriteCloser {
	c.fileWriterCtx = ctx
	c.fileWriterOpts = opts
	return c.MemoryGCSClient.FileWriter(ctx, path, opts)
}

func TestWithWriteFileSimple(t *testing.T) {

	c := &captureFileWriterGCSClient{
		MemoryGCSClient: mem_gcsclient.New("compositions"),
	}

	ctx := context.Background()
	opts := gcs.FileWriteOptions{
		ContentType: "text/plain",
	}
	const path = "story"
	const contents = "Once upon a time..."
	require.NoError(t, gcs.WithWriteFile(c, ctx, path, opts, func(w io.Writer) error {
		_, err := w.Write([]byte(contents))
		return err
	}))
	// The context should be canceled.
	require.Equal(t, context.Canceled, c.fileWriterCtx.Err())
	require.Equal(t, opts, c.fileWriterOpts)
	actualContents, err := c.GetFileContents(ctx, path)
	require.NoError(t, err)
	require.Equal(t, []byte(contents), actualContents)
}

func TestWithWriteFileError(t *testing.T) {

	c := &captureFileWriterGCSClient{
		MemoryGCSClient: mem_gcsclient.New("compositions"),
	}

	ctx := context.Background()
	opts := gcs.FileWriteOptions{
		ContentType: "text/plain",
	}
	const path = "the-neverstarting-story"
	err := errors.New("I can't remember how it starts.")
	require.Equal(t, gcs.WithWriteFile(c, ctx, path, opts, func(w io.Writer) error {
		return err
	}), err)
	// The context should be canceled.
	require.Equal(t, context.Canceled, c.fileWriterCtx.Err())
	require.Equal(t, opts, c.fileWriterOpts)
	exists, err := c.DoesFileExist(ctx, path)
	require.NoError(t, err)
	require.False(t, exists)
}

func TestWithWriteFileGzipSimple(t *testing.T) {

	c := &captureFileWriterGCSClient{
		MemoryGCSClient: mem_gcsclient.New("compositions"),
	}

	ctx := context.Background()
	const path = "condensible-story"
	const contents = "So like there was like this one time that I was like totally like..."
	require.NoError(t, gcs.WithWriteFileGzip(c, ctx, path, func(w io.Writer) error {
		_, err := w.Write([]byte(contents))
		return err
	}))
	// The context should be canceled.
	require.Equal(t, context.Canceled, c.fileWriterCtx.Err())
	require.Equal(t, gcs.FileWriteOptions{
		ContentEncoding: "gzip",
	}, c.fileWriterOpts)
	actualContents, err := c.GetFileContents(ctx, path)
	require.NoError(t, err)
	require.Equal(t, []byte(contents), actualContents)
}
