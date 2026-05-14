package sqlautobisectionstore

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/sql/pool"
	"go.skia.org/infra/perf/go/autobisection"
	"go.skia.org/infra/perf/go/autobisection/sqlautobisectionstore/schema"
	"go.skia.org/infra/perf/go/sql/sqltest"
)

func setUp(t *testing.T) (autobisection.Store, pool.Pool) {
	db := sqltest.NewSpannerDBForTests(t, "autobisections")
	store, err := New(db)
	require.NoError(t, err)
	return store, db
}

func TestSave_HappyPath(t *testing.T) {
	store, db := setUp(t)
	ctx := context.Background()

	bs := &schema.AutobisectionSchema{
		JobID:            "job-123",
		AnomalyGroupID:   "group-456",
		AnomalyId:        "id-1",
		IsRealRegression: true,
	}

	err := store.Save(ctx, bs)
	require.NoError(t, err)

	// Verify the row was inserted
	var loadedJobID string
	var loadedIsRealRegression bool

	err = db.QueryRow(ctx, "SELECT job_id, is_real_regression FROM Autobisections WHERE job_id = $1", "job-123").
		Scan(&loadedJobID, &loadedIsRealRegression)
	require.NoError(t, err)

	assert.Equal(t, "job-123", loadedJobID)
	assert.True(t, loadedIsRealRegression)
}

func TestSave_Empty_ReturnsError(t *testing.T) {
	store, _ := setUp(t)
	ctx := context.Background()

	bs := &schema.AutobisectionSchema{
		JobID:          "",
		AnomalyGroupID: "a",
		AnomalyId:      "b",
	}

	err := store.Save(ctx, bs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "job_id cannot be empty")

	bs = &schema.AutobisectionSchema{
		JobID:          "j",
		AnomalyGroupID: "",
		AnomalyId:      "b",
	}

	err = store.Save(ctx, bs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "anomaly group id cannot be empty")

	bs = &schema.AutobisectionSchema{
		JobID:          "j",
		AnomalyGroupID: "a",
		AnomalyId:      "",
	}

	err = store.Save(ctx, bs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "anomaly id cannot be empty")
}
