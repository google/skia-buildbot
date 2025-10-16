package sqlregression2store

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/perf/go/alerts"
	alerts_mock "go.skia.org/infra/perf/go/alerts/mock"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/regression"
	"go.skia.org/infra/perf/go/sql/sqltest"
	"go.skia.org/infra/perf/go/stepfit"
	"go.skia.org/infra/perf/go/types"
	"go.skia.org/infra/perf/go/ui/frame"
)

const alertId int64 = 1111

func setupStore(t *testing.T, alertsProvider alerts.ConfigProvider) *SQLRegression2Store {
	db := sqltest.NewSpannerDBForTests(t, "regstore")
	store, _ := New(db, alertsProvider)
	return store
}

func readSpecificRegressionFromDb(ctx context.Context, t *testing.T, store *SQLRegression2Store, commitNumber types.CommitNumber, alertIdStr string) *regression.Regression {
	regressionsFromDb, err := store.Range(ctx, commitNumber, commitNumber)
	assert.Nil(t, err)
	reg := regressionsFromDb[commitNumber].ByAlertID[alertIdStr]
	return reg
}

func generateNewRegression() *regression.Regression {
	r := regression.NewRegression()
	r.Id = uuid.NewString()
	r.CommitNumber = 12345
	r.AlertId = alertId
	r.CreationTime = time.Now()
	r.IsImprovement = false
	r.MedianBefore = 1.0
	r.MedianAfter = 2.0

	r.PrevCommitNumber = 12340
	df := &frame.FrameResponse{
		DataFrame: &dataframe.DataFrame{
			Header: []*dataframe.ColumnHeader{
				{Offset: 1},
				{Offset: 2},
				{Offset: 3},
			},
		},
	}
	clusterSummary := &clustering2.ClusterSummary{
		StepFit: &stepfit.StepFit{
			TurningPoint: 1,
		},
		Timestamp: time.Now(),
		Centroid:  []float32{1.0, 5.0, 5.0},
	}

	r.High = clusterSummary
	r.Frame = df
	return r
}

func generateAndStoreNewRegression(ctx context.Context, t *testing.T, store *SQLRegression2Store) *regression.Regression {
	r := generateNewRegression()
	_, err := store.WriteRegression(ctx, r, nil)
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
	alertsProvider := alerts_mock.NewConfigProvider(t)

	store := setupStore(t, alertsProvider)
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
	alertsProvider := alerts_mock.NewConfigProvider(t)

	store := setupStore(t, alertsProvider)
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

// TestGetByIDs_Success reads the database using the
// ids of the created regressions.
func TestGetByIDs_Success(t *testing.T) {
	alertsProvider := alerts_mock.NewConfigProvider(t)

	store := setupStore(t, alertsProvider)
	ctx := context.Background()
	r := generateAndStoreNewRegression(ctx, t, store)
	r2 := generateAndStoreNewRegression(ctx, t, store)

	// Improvements are anomalies, and they are stored, too.
	rImprovement := generateNewRegression()
	populateRegression2Fields(rImprovement)
	rImprovement.IsImprovement = true
	err := store.writeSingleRegression(ctx, rImprovement, nil)
	assert.Nil(t, err)

	tests := []struct {
		name             string
		regressionIDs    []string
		expectedLen      int
		shouldContainIDs []string
	}{
		{
			name:             "two regressions",
			regressionIDs:    []string{r.Id, r2.Id},
			expectedLen:      2,
			shouldContainIDs: []string{r.Id, r2.Id},
		},
		{
			name:             "two regressions and one improvement",
			regressionIDs:    []string{r.Id, r2.Id, rImprovement.Id},
			expectedLen:      3,
			shouldContainIDs: []string{r.Id, r2.Id, rImprovement.Id},
		},
		{
			name:             "empty ids list",
			regressionIDs:    []string{},
			expectedLen:      0,
			shouldContainIDs: []string{},
		},
		{
			name:             "just the improvement",
			regressionIDs:    []string{rImprovement.Id},
			expectedLen:      1,
			shouldContainIDs: []string{rImprovement.Id},
		},
		{
			name:             "duplicate ids are ignored",
			regressionIDs:    []string{rImprovement.Id, rImprovement.Id, rImprovement.Id, r.Id, r.Id},
			expectedLen:      2,
			shouldContainIDs: []string{rImprovement.Id, r.Id},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			regressions, err := store.GetByIDs(ctx, tc.regressionIDs)
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedLen, len(regressions))
			for _, r := range regressions {
				assert.Contains(t, tc.shouldContainIDs, r.Id)
			}
		})
	}
}

// TestGetByIDs_Success reads the database using the
// ids of the created regressions.
func TestGetByRevision_Success(t *testing.T) {
	alertsProvider := alerts_mock.NewConfigProvider(t)

	store := setupStore(t, alertsProvider)
	ctx := context.Background()

	generateRegression := func(previousCommit int64, commit int64) (r *regression.Regression) {
		r = generateNewRegression()
		populateRegression2Fields(r)
		r.PrevCommitNumber = types.CommitNumber(previousCommit)
		r.CommitNumber = types.CommitNumber(commit)
		err := store.writeSingleRegression(ctx, r, nil)
		require.NoError(t, err)
		return
	}

	r100_200 := generateRegression(100, 200)
	r101_200 := generateRegression(101, 200)
	r300_301 := generateRegression(300, 301)

	tests := []struct {
		name             string
		revision         string
		shouldContainIDs []string
	}{
		{
			name:             "revision inside two regressions",
			revision:         "102",
			shouldContainIDs: []string{r100_200.Id, r101_200.Id},
		},
		{
			name:             "inside a regression and at the beginning of another",
			revision:         "101",
			shouldContainIDs: []string{r100_200.Id},
		},
		{
			name:             "beginning of a regression and before others",
			revision:         "100",
			shouldContainIDs: []string{},
		},
		{
			name:             "before all regressions",
			revision:         "99",
			shouldContainIDs: []string{},
		},
		{
			name:             "just before the end of regressions",
			revision:         "199",
			shouldContainIDs: []string{r100_200.Id, r101_200.Id},
		},
		{
			name:             "coinciding with the commit number of two regressions",
			revision:         "200",
			shouldContainIDs: []string{r100_200.Id, r101_200.Id},
		},
		{
			name:             "right after some regressions",
			revision:         "201",
			shouldContainIDs: []string{},
		},
		{
			name:             "inside a 1-wide regression",
			revision:         "301",
			shouldContainIDs: []string{r300_301.Id},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			regressions, err := store.GetByRevision(ctx, tc.revision)
			assert.NoError(t, err)
			assert.Equal(t, len(tc.shouldContainIDs), len(regressions))
			for _, r := range regressions {
				assert.Contains(t, tc.shouldContainIDs, r.Id)
			}
		})
	}
}

// TestHighRegression_KMeans_Triage sets a High regression into the database, triages it
// and verifies that the data was updated correctly. The alert Algo is set to be KMeans.
func TestHighRegression_KMeans_Triage(t *testing.T) {
	alertsProvider := alerts_mock.NewConfigProvider(t)
	alertsProvider.On("GetAlertConfig", alertId).Return(&alerts.Alert{
		IDAsString:  "1111",
		DisplayName: "Test Alert Config",
		Algo:        types.KMeansGrouping,
	}, nil)
	runClusterSummaryAndTriageTest(t, true, alertsProvider)
}

// TestLowRegression_KMeans_Triage sets a Low regression into the database, triages it
// and verifies that the data was updated correctly. The alert Algo is set to be KMeans.
func TestLowRegression_KMeans_Triage(t *testing.T) {
	alertsProvider := alerts_mock.NewConfigProvider(t)
	alertsProvider.On("GetAlertConfig", alertId).Return(&alerts.Alert{
		IDAsString:  "1111",
		DisplayName: "Test Alert Config",
		Algo:        types.KMeansGrouping,
	}, nil)
	runClusterSummaryAndTriageTest(t, false, alertsProvider)
}

// TestHighRegression_Ind_Triage sets a High regression into the database, triages it
// and verifies that the data was updated correctly. The alert Algo is set to be
// StepFitGrouping (i.e Individual)
func TestHighRegression_Ind_Triage(t *testing.T) {
	alertsProvider := alerts_mock.NewConfigProvider(t)
	alertsProvider.On("GetAlertConfig", alertId).Return(&alerts.Alert{
		IDAsString:  "1111",
		DisplayName: "Test Alert Config",
		Algo:        types.StepFitGrouping,
	}, nil)
	runClusterSummaryAndTriageTest(t, true, alertsProvider)
}

// TestLowRegression_Ind_Triage sets a Low regression into the database, triages it
// and verifies that the data was updated correctly. The alert Algo is set to be
// StepFitGrouping (i.e Individual)
func TestLowRegression_Ind_Triage(t *testing.T) {
	alertsProvider := alerts_mock.NewConfigProvider(t)
	alertsProvider.On("GetAlertConfig", alertId).Return(&alerts.Alert{
		IDAsString:  "1111",
		DisplayName: "Test Alert Config",
		Algo:        types.StepFitGrouping,
	}, nil)
	runClusterSummaryAndTriageTest(t, false, alertsProvider)
}

func TestMixedRegressionWrite(t *testing.T) {
	alertsProvider := alerts_mock.NewConfigProvider(t)
	alertIdStr := "1111"

	store := setupStore(t, alertsProvider)
	ctx := context.Background()

	// Add an item to the database.
	r := generateNewRegression()
	r.Id = ""

	// Add another cluster summary to the same regression.
	r.Low = r.High
	_, err := store.WriteRegression(ctx, r, nil)
	assert.Nil(t, err)
	reg := readSpecificRegressionFromDb(ctx, t, store, r.CommitNumber, alertIdStr)
	assert.NotNil(t, reg)
	assert.NotNil(t, reg.High)
	assert.NotNil(t, reg.Low)
}

func TestRangeFiltered(t *testing.T) {
	const (
		traceKey1           = ",benchmark=Blazor,bot=MacM1,master=ChromiumPerf,test=test1,"
		traceKey2           = ",benchmark=Blazor,bot=MacM1,master=ChromiumPerf,test=test2,"
		nonExistentTraceKey = "non-existent-trace"
	)
	alertsProvider := alerts_mock.NewConfigProvider(t)

	store := setupStore(t, alertsProvider)
	ctx := context.Background()

	// Add a regression with trace key 1.
	r1 := generateNewRegression()
	r1.CommitNumber = 12345
	r1.Frame.DataFrame.TraceSet = types.TraceSet{traceKey1: {}}
	_, err := store.WriteRegression(ctx, r1, nil)
	assert.Nil(t, err)

	// Add a regression with trace key 2.
	r2 := generateNewRegression()
	r2.CommitNumber = 12346
	r2.Frame.DataFrame.TraceSet = types.TraceSet{traceKey2: {}}
	_, err = store.WriteRegression(ctx, r2, nil)
	assert.Nil(t, err)

	// Filter by trace key 1.
	regressionsFromDb, err := store.RangeFiltered(ctx, r1.CommitNumber, r1.CommitNumber, []string{traceKey1})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if strings.Contains(pgErr.Message, "Postgres function jsonb_exists_any(jsonb, text[]) is not supported") {
				// TODO(ansid): this can be removed when Spanner emulator image in gcloudsdk is updated.
				// To test if it can be removed already, remove and run tests with "--config=remote".
				t.Skip("Skiped test unsupported by Spanner emulator")
				return
			}
		}
	}
	assert.Nil(t, err)
	assert.NotNil(t, regressionsFromDb)
	assert.Len(t, regressionsFromDb, 1)
	assertRegression(t, r1, regressionsFromDb[0])

	// Filter by trace key 2.
	regressionsFromDb, err = store.RangeFiltered(ctx, r2.CommitNumber, r2.CommitNumber, []string{traceKey2})
	assert.Nil(t, err)
	assert.NotNil(t, regressionsFromDb)
	assert.Len(t, regressionsFromDb, 1)
	assertRegression(t, r2, regressionsFromDb[0])

	// Filter by both trace keys.
	regressionsFromDb, err = store.RangeFiltered(ctx, r1.CommitNumber, r2.CommitNumber, []string{traceKey1, traceKey2})
	assert.Nil(t, err)
	assert.NotNil(t, regressionsFromDb)
	assert.Len(t, regressionsFromDb, 2)

	// Filter by a non-existent trace key.
	regressionsFromDb, err = store.RangeFiltered(ctx, r1.CommitNumber, r2.CommitNumber, []string{nonExistentTraceKey})
	assert.Nil(t, err)
	assert.Empty(t, regressionsFromDb)
}

func runClusterSummaryAndTriageTest(t *testing.T, isHighRegression bool, alertsProvider alerts.ConfigProvider) {
	store := setupStore(t, alertsProvider)
	ctx := context.Background()

	// Add an item to the database.
	r := generateNewRegression()

	alertIdStr := alerts.IDToString(r.AlertId)
	clusterSummary := &clustering2.ClusterSummary{
		Centroid: []float32{1.0, 2.0, 3.0},
		StepFit: &stepfit.StepFit{
			TurningPoint: 1,
		},
	}

	var success bool
	var err error
	frameResponse := &frame.FrameResponse{
		DataFrame: &dataframe.DataFrame{
			Header: []*dataframe.ColumnHeader{
				{
					Offset: 1,
				},
				{
					Offset: 2,
				},
				{
					Offset: 3,
				},
			},
		},
	}
	if isHighRegression {
		// Set a high regression.
		success, _, err = store.SetHigh(ctx, r.CommitNumber, alertIdStr, frameResponse, clusterSummary)
	} else {
		// Set a low regression.
		success, _, err = store.SetLow(ctx, r.CommitNumber, alertIdStr, frameResponse, clusterSummary)
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
