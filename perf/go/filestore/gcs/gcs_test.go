package gcs

import (
	"fmt"
	"io/fs"
	"testing"

	"cloud.google.com/go/storage"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/iterator"
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

type mockIterator struct {
	attrs []*storage.ObjectAttrs
	err   error
	index int
}

func (m *mockIterator) Next() (*storage.ObjectAttrs, error) {
	if m.index < len(m.attrs) {
		attr := m.attrs[m.index]
		m.index++
		return attr, nil
	}
	if m.err != nil {
		return nil, m.err
	}
	return nil, iterator.Done
}

func TestExtractLatestObject_Success(t *testing.T) {
	mock := &mockIterator{
		attrs: []*storage.ObjectAttrs{{Name: "path/to/file.json"}},
	}
	uri, err := extractLatestObject(mock, "my-bucket", "gs://my-bucket/path/to/file")
	require.NoError(t, err)
	require.Equal(t, "gs://my-bucket/path/to/file.json", uri)
}

func TestExtractLatestObject_MultipleResults(t *testing.T) {
	mock := &mockIterator{
		attrs: []*storage.ObjectAttrs{
			{Name: "path/to/file_2023.json"},
			{Name: "path/to/file_2024.json"},
		},
	}
	uri, err := extractLatestObject(mock, "my-bucket", "gs://my-bucket/path/to/file")
	require.NoError(t, err)
	require.Equal(t, "gs://my-bucket/path/to/file_2024.json", uri)
}

func TestExtractLatestObject_IteratorDoneReturnsNotExist(t *testing.T) {
	mock := &mockIterator{
		err: iterator.Done,
	}
	uri, err := extractLatestObject(mock, "my-bucket", "gs://my-bucket/path/to/file")
	require.Error(t, err)
	require.ErrorIs(t, err, fs.ErrNotExist)
	require.Equal(t, "", uri)
}

func TestExtractLatestObject_OtherErrorReturnsError(t *testing.T) {
	mock := &mockIterator{
		err: fmt.Errorf("some network error"),
	}
	_, err := extractLatestObject(mock, "my-bucket", "gs://my-bucket/path/to/file")
	require.Error(t, err)
	require.NotErrorIs(t, err, fs.ErrNotExist)
}
