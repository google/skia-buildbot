package sqlregression2store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/regression"
	"go.skia.org/infra/perf/go/sql/sqltest"
	"go.skia.org/infra/perf/go/stepfit"
	"go.skia.org/infra/perf/go/types"
	"go.skia.org/infra/perf/go/ui/frame"
)

func setupStore(t *testing.T) *SQLRegression2Store {
	db := sqltest.NewCockroachDBForTests(t, "regstore")
	store, _ := New(db)
	return store
}

func readSpecificRegressionFromDb(ctx context.Context, t *testing.T, store *SQLRegression2Store, commitNumber types.CommitNumber, alertIdStr string) *regression.Regression {
	regressionsFromDb, err := store.Range(ctx, commitNumber, commitNumber)
	assert.Nil(t, err)
	reg := regressionsFromDb[commitNumber].ByAlertID[alertIdStr]
	return reg
}

func generateAndStoreNewRegression(ctx context.Context, t *testing.T, store *SQLRegression2Store) *regression.Regression {
	r := regression.NewRegression()
	r.Id = uuid.NewString()
	r.CommitNumber = 12345
	r.AlertId = 1111
	r.CreationTime = time.Now()
	r.IsImprovement = false
	r.MedianBefore = 1.0
	r.MedianAfter = 2.0

	r.PrevCommitNumber = 12340
	regressions := map[types.CommitNumber]*regression.AllRegressionsForCommit{
		r.CommitNumber: {
			ByAlertID: map[string]*regression.Regression{
				alerts.IDToString(r.AlertId): r,
			},
		},
	}
	err := store.Write(ctx, regressions)
	assert.Nil(t, err)
	return r
}

func assertRegression(t *testing.T, expected *regression.Regression, actual *regression.Regression) {
	assert.Equal(t, expected.AlertId, actual.AlertId)
	assert.Equal(t, expected.CommitNumber, actual.CommitNumber)
	assert.Equal(t, expected.PrevCommitNumber, actual.PrevCommitNumber)
	assert.Equal(t, expected.IsImprovement, actual.IsImprovement)
	assert.Equal(t, expected.MedianBefore, actual.MedianBefore)
	assert.Equal(t, expected.MedianAfter, actual.MedianAfter)
	assert.Equal(t, expected.Frame, actual.Frame)
}

// TestWriteRead_Success writes a regression to the database
// and verifies if it is read back correctly.
func TestWriteRead_Success(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()
	r := generateAndStoreNewRegression(ctx, t, store)

	regressionsFromDb, err := store.Range(ctx, r.CommitNumber, r.CommitNumber)
	assert.Nil(t, err)
	assert.NotNil(t, regressionsFromDb)
	regressionsForCommit := regressionsFromDb[r.CommitNumber]
	assert.NotNil(t, regressionsForCommit)
	regressionByAlertId := regressionsForCommit.ByAlertID[alerts.IDToString(r.AlertId)]
	assert.NotNil(t, regressionByAlertId)
	assertRegression(t, r, regressionByAlertId)
}

// TestRead_Empty reads the database when it is empty
func TestRead_Empty(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()

	// Try reading items when db is empty.
	regressionsFromDb, err := store.Range(ctx, 1, 2)
	assert.Nil(t, err)
	assert.Empty(t, regressionsFromDb)

	// Now let's add an item and try to read non-existent items.
	r := generateAndStoreNewRegression(ctx, t, store)

	regressionsFromDb, err = store.Range(ctx, r.CommitNumber+1, r.CommitNumber+2)
	assert.Nil(t, err)
	assert.Empty(t, regressionsFromDb)
}

// TestHighRegression_Triage sets a High regression into the database, triages it
// and verifies that the data was updated correctly.
func TestHighRegression_Triage(t *testing.T) {
	runClusterSummaryAndTriageTest(t, true)
}

// TestLowRegression_Triage sets a Low regression into the database, triages it
// and verifies that the data was updated correctly.
func TestLowRegression_Triage(t *testing.T) {
	runClusterSummaryAndTriageTest(t, false)
}

func runClusterSummaryAndTriageTest(t *testing.T, isHighRegression bool) {
	store := setupStore(t)
	ctx := context.Background()

	// Add an item to the database.
	r := generateAndStoreNewRegression(ctx, t, store)

	alertIdStr := alerts.IDToString(r.AlertId)
	clusterSummary := &clustering2.ClusterSummary{
		Centroid: []float32{1.0, 2.0, 3.0},
		StepFit:  stepfit.NewStepFit(),
	}

	var success bool
	var err error
	if isHighRegression {
		// Set a high regression.
		success, err = store.SetHigh(ctx, r.CommitNumber, alertIdStr, &frame.FrameResponse{}, clusterSummary)
	} else {
		// Set a low regression.
		success, err = store.SetLow(ctx, r.CommitNumber, alertIdStr, &frame.FrameResponse{}, clusterSummary)
	}

	assert.Nil(t, err)
	assert.True(t, success)
	// Read the regression and verify that High value was set correctly.
	reg := readSpecificRegressionFromDb(ctx, t, store, r.CommitNumber, alertIdStr)
	assert.NotNil(t, reg)

	if isHighRegression {
		assert.NotNil(t, reg.High)
		assert.Nil(t, reg.Low)
		assert.Equal(t, clusterSummary, reg.High)
	} else {
		assert.NotNil(t, reg.Low)
		assert.Nil(t, reg.High)
		assert.Equal(t, clusterSummary, reg.Low)
	}

	triageStatus := regression.TriageStatus{
		Status:  regression.Negative,
		Message: "Test",
	}

	// Set the triage value in the db.
	if isHighRegression {
		err = store.TriageHigh(ctx, r.CommitNumber, alertIdStr, triageStatus)
	} else {
		err = store.TriageLow(ctx, r.CommitNumber, alertIdStr, triageStatus)
	}

	assert.Nil(t, err)

	// Now read the regression and verify that this value was applied correctly.
	reg = readSpecificRegressionFromDb(ctx, t, store, r.CommitNumber, alertIdStr)

	if isHighRegression {
		assert.Equal(t, triageStatus, reg.HighStatus)
		assert.Equal(t, regression.TriageStatus{}, reg.LowStatus)
	} else {
		assert.Equal(t, triageStatus, reg.LowStatus)
		assert.Equal(t, regression.TriageStatus{}, reg.HighStatus)
	}
}
