package gcsuploader

import (
	"context"
	"io/ioutil"
	"math/rand"
	"path"
	"strconv"
	"testing"
	"time"

	"cloud.google.com/go/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/util"
)

const (
	testBucket = "skia-temp"
)

func TestClientImpl(t *testing.T) {

	ctx := context.Background()
	gc, err := storage.NewClient(ctx)
	require.NoError(t, err)
	c := &clientImpl{
		client: gc,
	}

	randomSubfolder := setup(ctx, t, c.client)

	t.Run("UploadToGCS_FileDoesNotExist_Success", testUploadToGCS_FileDoesNotExist_Success(c, randomSubfolder))
	t.Run("UploadToGCS_FileDoesExist_Success", testUploadToGCS_FileDoesExist_DataOverwritten(c, randomSubfolder))
	t.Run("UploadToGCS_PermanentError_RetriesAndThenErrors", testUploadToGCS_PermanentError_RetriesAndThenErrors(c))
}

// setup creates a randomly named subfolder in our test bucket. It creates a file with a sentinel
// value.
func setup(ctx context.Context, t *testing.T, c *storage.Client) string {
	rand.Seed(time.Now().UnixNano())
	subfolder := "gold_manual_tests/" + strconv.Itoa(rand.Int())

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	w := c.Bucket(testBucket).Object(subfolder + "/already_exists.json").NewWriter(ctx)
	_, err := w.Write([]byte(`{"hello":"world"}`))
	require.NoError(t, err)

	return subfolder
}

func testUploadToGCS_FileDoesNotExist_Success(c *clientImpl, testPath string) func(t *testing.T) {
	return func(t *testing.T) {
		ctx := context.Background()
		f := path.Join(testBucket, testPath, "my_file.json")
		require.NoError(t, c.uploadToGCS(ctx, []byte("some data"), f))

		actual := readGCSFile(ctx, t, c.client, path.Join(testPath, "my_file.json"))
		assert.Equal(t, []byte("some data"), actual)
	}
}

func testUploadToGCS_FileDoesExist_DataOverwritten(c *clientImpl, testPath string) func(t *testing.T) {
	return func(t *testing.T) {
		ctx := context.Background()
		f := path.Join(testBucket, testPath, "already_exists.json")
		require.NoError(t, c.uploadToGCS(ctx, []byte("overwrite the data!"), f))

		actual := readGCSFile(ctx, t, c.client, path.Join(testPath, "already_exists.json"))
		assert.Equal(t, []byte("overwrite the data!"), actual)
	}
}

func testUploadToGCS_PermanentError_RetriesAndThenErrors(c *clientImpl) func(t *testing.T) {
	return func(t *testing.T) {
		require.Error(t, c.uploadToGCS(context.Background(), []byte("overwrite the data!"), "skia-this-will-permanently-fail/file.json"))
	}
}

// readGCSFile returns the file contents of the given file in the test bucket.
func readGCSFile(ctx context.Context, t *testing.T, c *storage.Client, file string) []byte {
	r, err := c.Bucket(testBucket).Object(file).NewReader(ctx)
	require.NoError(t, err)
	defer util.Close(r)
	b, err := ioutil.ReadAll(r)
	require.NoError(t, err)
	return b
}
