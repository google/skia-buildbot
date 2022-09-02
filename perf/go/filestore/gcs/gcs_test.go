package gcs

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_filesystem_parseNameIntoBucketAndPath_Success(t *testing.T) {

	bucket, path, err := parseNameIntoBucketAndPath("gs://bucket/this/is/the/path.txt")
	require.NoError(t, err)
	require.Equal(t, "bucket", bucket)
	require.Equal(t, "this/is/the/path.txt", path)
}

func Test_filesystem_parseNameIntoBucketAndPathWithEmptyURL_ReturnsError(t *testing.T) {

	_, _, err := parseNameIntoBucketAndPath("")
	require.Error(t, err)
}

func Test_filesystem_parseNameIntoBucketAndPathWithInvalidURL_ReturnsError(t *testing.T) {

	_, _, err := parseNameIntoBucketAndPath("ht tp://foo.com")
	require.Error(t, err)
}
