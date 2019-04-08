package gcs

import (
	"context"
	"errors"
	"io"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
)

// TODO(dogben): This should really have some tests for gcsclient.

// captureFileWriterGCSClient captures FileWriter args for TestWithWriteFile* and
// TestWithWriteFileGzip*.
type captureFileWriterGCSClient struct {
	*MemoryGCSClient
	fileWriterCtx  context.Context
	fileWriterOpts FileWriteOptions
}

func (c *captureFileWriterGCSClient) FileWriter(ctx context.Context, path string, opts FileWriteOptions) io.WriteCloser {
	c.fileWriterCtx = ctx
	c.fileWriterOpts = opts
	return c.MemoryGCSClient.FileWriter(ctx, path, opts)
}

func TestWithWriteFileSimple(t *testing.T) {
	testutils.SmallTest(t)

	c := &captureFileWriterGCSClient{
		MemoryGCSClient: NewMemoryGCSClient("compositions"),
	}

	ctx := context.Background()
	opts := FileWriteOptions{
		ContentType: "text/plain",
	}
	const path = "story"
	const contents = "Once upon a time..."
	assert.NoError(t, WithWriteFile(c, ctx, path, opts, func(w io.Writer) error {
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
	testutils.SmallTest(t)

	c := &captureFileWriterGCSClient{
		MemoryGCSClient: NewMemoryGCSClient("compositions"),
	}

	ctx := context.Background()
	opts := FileWriteOptions{
		ContentType: "text/plain",
	}
	const path = "the-neverstarting-story"
	err := errors.New("I can't remember how it starts.")
	assert.Equal(t, WithWriteFile(c, ctx, path, opts, func(w io.Writer) error {
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
	testutils.SmallTest(t)

	c := &captureFileWriterGCSClient{
		MemoryGCSClient: NewMemoryGCSClient("compositions"),
	}

	ctx := context.Background()
	const path = "condensible-story"
	const contents = "So like there was like this one time that I was like totally like..."
	assert.NoError(t, WithWriteFileGzip(c, ctx, path, func(w io.Writer) error {
		_, err := w.Write([]byte(contents))
		return err
	}))
	// The context should be canceled.
	assert.Equal(t, context.Canceled, c.fileWriterCtx.Err())
	assert.Equal(t, FileWriteOptions{
		ContentEncoding: "gzip",
	}, c.fileWriterOpts)
	actualContents, err := c.GetFileContents(ctx, path)
	assert.NoError(t, err)
	assert.Equal(t, []byte(contents), actualContents)
}
