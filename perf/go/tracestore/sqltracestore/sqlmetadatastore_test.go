package sqltracestore

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/sql/pool"
	"go.skia.org/infra/perf/go/sql/sqltest"
)

func createMetadataStoreForTests(t *testing.T) *SQLMetadataStore {
	db := sqltest.NewSpannerDBForTests(t, "tracestore")
	return NewSQLMetadataStore(db)
}

func insertSourceFile(ctx context.Context, db pool.Pool, sourceFileName string, sourceFileId int) error {
	stmt := "INSERT INTO SourceFiles VALUES ($1, $2)"
	if _, err := db.Exec(ctx, stmt, sourceFileId, sourceFileName); err != nil {
		return err
	}

	return nil
}

func TestInsertMetadata_Success(t *testing.T) {
	store := createMetadataStoreForTests(t)
	links := map[string]string{
		"key1": "link1",
		"key2": "link2",
	}

	sourceFileId := 111
	sourceFileName := "testSourceFile"
	ctx := context.Background()
	err := insertSourceFile(ctx, store.db, sourceFileName, sourceFileId)
	assert.NoError(t, err)
	err = store.InsertMetadata(ctx, sourceFileName, links)
	assert.NoError(t, err)

	// Get the links from db.
	linksFromDb, err := store.GetMetadata(ctx, sourceFileName)
	assert.NoError(t, err)
	assert.Equal(t, links, linksFromDb)

	// Try to get an invalid link from db.
	linksFromDb, err = store.GetMetadata(ctx, "IDontExist.json")
	assert.Error(t, err)
	assert.Nil(t, linksFromDb)
}

func TestGetMetadata_ValidSourceFile_NoMetadata(t *testing.T) {
	store := createMetadataStoreForTests(t)

	sourceFileName := "testSourceFile"
	ctx := context.Background()
	err := insertSourceFile(ctx, store.db, sourceFileName, 1)
	assert.NoError(t, err)
	// Get the links from db. Since the metadata table is not populated
	// for the source file this should return empty.
	linksFromDb, err := store.GetMetadata(ctx, sourceFileName)
	assert.Nil(t, err)
	assert.Nil(t, linksFromDb)
}

func TestInsertMetadata_InvalidSourceFile(t *testing.T) {
	store := createMetadataStoreForTests(t)
	links := map[string]string{
		"key1": "link1",
		"key2": "link2",
	}

	sourceFileName := "testSourceFile"
	ctx := context.Background()
	// Source file is not present in the database.
	err := store.InsertMetadata(ctx, sourceFileName, links)
	assert.Error(t, err)
}
