package sqlautobisectionstore

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/sql/pool"
	"go.skia.org/infra/perf/go/autobisection"
	v1 "go.skia.org/infra/perf/go/autobisection/proto/v1"
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
		WorkflowID:       "wf-123",
		AnomalyGroupID:   "group-456",
		AnomalyId:        "id-1",
		RegressionStatus: v1.RegressionStatus_FOUND_CULPRITS.String(),
	}

	err := store.Save(ctx, bs)
	require.NoError(t, err)

	// Verify the row was inserted
	var loadedJobID string
	var regressionStatus string

	err = db.QueryRow(ctx, "SELECT job_id, regression_status FROM Autobisections WHERE job_id = $1", "job-123").
		Scan(&loadedJobID, &regressionStatus)
	require.NoError(t, err)

	assert.Equal(t, "job-123", loadedJobID)
	assert.Equal(t, v1.RegressionStatus_FOUND_CULPRITS.String(), regressionStatus)
}

func TestSave_Empty_ReturnsError(t *testing.T) {
	store, _ := setUp(t)
	ctx := context.Background()

	bs := &schema.AutobisectionSchema{
		JobID:            "",
		AnomalyGroupID:   "a",
		AnomalyId:        "b",
		WorkflowID:       "c",
		RegressionStatus: v1.RegressionStatus_NO_CULPRIT_FOUND.String(),
	}

	err := store.Save(ctx, bs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "job_id cannot be empty")

	bs = &schema.AutobisectionSchema{
		JobID:            "j",
		AnomalyGroupID:   "",
		AnomalyId:        "b",
		WorkflowID:       "c",
		RegressionStatus: v1.RegressionStatus_NO_CULPRIT_FOUND.String(),
	}

	err = store.Save(ctx, bs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "anomaly group id cannot be empty")

	bs = &schema.AutobisectionSchema{
		JobID:            "j",
		AnomalyGroupID:   "a",
		AnomalyId:        "",
		WorkflowID:       "c",
		RegressionStatus: v1.RegressionStatus_NO_CULPRIT_FOUND.String(),
	}

	err = store.Save(ctx, bs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "anomaly id cannot be empty")

	bs = &schema.AutobisectionSchema{
		JobID:            "j",
		AnomalyGroupID:   "a",
		AnomalyId:        "b",
		WorkflowID:       "",
		RegressionStatus: v1.RegressionStatus_NO_CULPRIT_FOUND.String(),
	}

	err = store.Save(ctx, bs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "workflow id cannot be empty")

	bs = &schema.AutobisectionSchema{
		JobID:            "j",
		AnomalyGroupID:   "a",
		AnomalyId:        "b",
		WorkflowID:       "c",
		RegressionStatus: "",
	}

	err = store.Save(ctx, bs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "regression status is invalid")

	bs = &schema.AutobisectionSchema{
		JobID:            "j",
		AnomalyGroupID:   "a",
		AnomalyId:        "b",
		WorkflowID:       "c",
		RegressionStatus: v1.RegressionStatus_UNSPECIFIED.String(),
	}

	err = store.Save(ctx, bs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "regression status is invalid")
}

func TestSave_InvalidStatus(t *testing.T) {
	store, _ := setUp(t)
	ctx := context.Background()

	bs := &schema.AutobisectionSchema{
		JobID:            "j",
		AnomalyGroupID:   "a",
		AnomalyId:        "b",
		WorkflowID:       "c",
		RegressionStatus: "no culprits", // added 's'
	}

	err := store.Save(ctx, bs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "regression status is invalid")
}
