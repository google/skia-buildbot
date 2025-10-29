package blamestore

import (
	"context"
	"testing"

	"cloud.google.com/go/spanner"
	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/rag/go/sqltest"
	"google.golang.org/api/iterator"
)

func createBlameStoreForTests(t *testing.T) (BlameStore, *spanner.Client) {
	spannerClient, err := sqltest.NewSpannerDBForTests(t, "tracestore")
	assert.NoError(t, err)
	return New(spannerClient), spannerClient
}

func TestBlameStore_WriteBlame_Insert(t *testing.T) {
	store, client := createBlameStoreForTests(t)

	blame := &FileBlame{
		FilePath:   "foo.go",
		FileHash:   "123",
		Version:    "1",
		CommitHash: "abc",
		LineBlames: []*LineBlame{
			{LineNumber: 1, CommitHash: "abc"},
			{LineNumber: 2, CommitHash: "abc"},
		},
	}

	ctx := context.Background()

	err := store.WriteBlame(ctx, blame)
	assert.NoError(t, err)

	// Verify data.
	iter := client.Single().Read(ctx, "BlamedFiles", spanner.AllKeys(), []string{"file_path", "file_hash"})
	defer iter.Stop()
	row, err := iter.Next()
	assert.NoError(t, err)
	var filePath, fileHash string
	err = row.Columns(&filePath, &fileHash)
	assert.NoError(t, err)
	assert.Equal(t, "foo.go", filePath)
	assert.Equal(t, "123", fileHash)

	iter = client.Single().Read(ctx, "LineBlames", spanner.AllKeys(), []string{"line_number", "commit_hash"})
	defer iter.Stop()
	row, err = iter.Next()
	assert.NoError(t, err)
	var lineNumber int64
	var commitHash string
	err = row.Columns(&lineNumber, &commitHash)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), lineNumber)
	assert.Equal(t, "abc", commitHash)
}

func TestBlameStore_WriteBlame_Update(t *testing.T) {
	store, client := createBlameStoreForTests(t)
	ctx := context.Background()

	// First write.
	blame1 := &FileBlame{
		FilePath:   "foo.go",
		FileHash:   "123",
		Version:    "1",
		CommitHash: "abc",
		LineBlames: []*LineBlame{
			{LineNumber: 1, CommitHash: "abc"},
		},
	}
	err := store.WriteBlame(ctx, blame1)
	assert.NoError(t, err)

	// Second write to the same file.
	blame2 := &FileBlame{
		FilePath:   "foo.go",
		FileHash:   "456",
		Version:    "2",
		CommitHash: "def",
		LineBlames: []*LineBlame{
			{LineNumber: 1, CommitHash: "def"},
			{LineNumber: 2, CommitHash: "def"},
		},
	}
	err = store.WriteBlame(ctx, blame2)
	assert.NoError(t, err)

	// Verify data.
	iter := client.Single().Read(ctx, "BlamedFiles", spanner.AllKeys(), []string{"file_path", "file_hash"})
	defer iter.Stop()
	row, err := iter.Next()
	assert.NoError(t, err)
	var filePath, fileHash string
	err = row.Columns(&filePath, &fileHash)
	assert.NoError(t, err)
	assert.Equal(t, "foo.go", filePath)
	assert.Equal(t, "456", fileHash)

	iter = client.Single().Read(ctx, "LineBlames", spanner.AllKeys(), []string{"commit_hash"})
	defer iter.Stop()
	count := 0
	for {
		row, err := iter.Next()
		if err == iterator.Done {
			break
		}
		assert.NoError(t, err)
		var commitHash string
		err = row.Columns(&commitHash)
		assert.NoError(t, err)
		assert.Equal(t, "def", commitHash)
		count++
	}
	assert.Equal(t, 2, count)
}
