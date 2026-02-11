package sqlregression2store

import (
	"context"
	"errors"
	"fmt"
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
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/regression"
	"go.skia.org/infra/perf/go/sql/sqltest"
	"go.skia.org/infra/perf/go/stepfit"
	"go.skia.org/infra/perf/go/types"
	"go.skia.org/infra/perf/go/ui/frame"
)

const (
	alertId int64 = 1111
	subName       = "my-sub"
)

func setupStore(t *testing.T, alertsProvider alerts.ConfigProvider) *SQLRegression2Store {
	db := sqltest.NewSpannerDBForTests(t, "regstore")
	instanceConfig := &config.InstanceConfig{
		AllowMultipleRegressionsPerAlertId: true,
	}
	store, _ := New(db, alertsProvider, instanceConfig)
	return store
}

func readSpecificRegressionFromDb(ctx context.Context, t *testing.T, store *SQLRegression2Store, commitNumber types.CommitNumber, alertIdStr string) *regression.Regression {
	regressionsFromDb, err := store.Range(ctx, commitNumber, commitNumber)
	assert.Nil(t, err)
	reg := regressionsFromDb[commitNumber].ByAlertID[alertIdStr]
	return reg
}

func generateNewRegression(subname string) *regression.Regression {
	r := regression.NewRegression()
	r.Id = uuid.NewString()
	r.CommitNumber = 12345
	r.AlertId = alertId
	r.Bugs = []types.RegressionBug{}
	r.CreationTime = time.Now()
	r.IsImprovement = false
	r.MedianBefore = 1.0
	r.MedianAfter = 2.0
	r.SubscriptionName = subname

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
	r.HighStatus = regression.TriageStatus{
		Status:  regression.Untriaged,
		Message: "",
	}
	r.Frame = df
	return r
}

func generateAndStoreNewRegression(ctx context.Context, t *testing.T, store *SQLRegression2Store, subname string) *regression.Regression {
	r := generateNewRegression(subname)
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

func skipTestIfSpannerEmulatorNotSupported(t *testing.T, err error) bool {
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if strings.Contains(pgErr.Message, "Postgres function jsonb_exists_any(jsonb, text[]) is not supported") || strings.Contains(pgErr.Message, "Postgres function jsonb_exists(jsonb, text) is not supported") {
				// TODO(ansid): this can be removed when Spanner emulator image in gcloudsdk is updated.
				// To test if it can be removed already, remove and run tests with "--config=remote".
				t.Skip("Skiped test unsupported by Spanner emulator")
				return true
			}
		}
	}
	return false
}

// TestWriteRead_Success writes a regression to the database
// and verifies if it is read back correctly.
func TestWriteRead_Success(t *testing.T) {
	alertsProvider := alerts_mock.NewConfigProvider(t)

	store := setupStore(t, alertsProvider)
	ctx := context.Background()
	r := generateAndStoreNewRegression(ctx, t, store, subName)

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
	r := generateAndStoreNewRegression(ctx, t, store, subName)

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
	r := generateAndStoreNewRegression(ctx, t, store, subName)
	r2 := generateAndStoreNewRegression(ctx, t, store, subName)

	// Improvements are anomalies, and they are stored, too.
	rImprovement := generateNewRegression(subName)
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
		r = generateNewRegression(subName)
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
	r := generateNewRegression(subName)
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
	r1 := generateNewRegression(subName)
	r1.CommitNumber = 12345
	r1.Frame.DataFrame.TraceSet = types.TraceSet{traceKey1: {}}
	_, err := store.WriteRegression(ctx, r1, nil)
	assert.Nil(t, err)

	// Add a regression with trace key 2.
	r2 := generateNewRegression(subName)
	r2.CommitNumber = 12346
	r2.Frame.DataFrame.TraceSet = types.TraceSet{traceKey2: {}}
	_, err = store.WriteRegression(ctx, r2, nil)
	assert.Nil(t, err)

	// Filter by trace key 1.
	regressionsFromDb, err := store.RangeFiltered(ctx, r1.CommitNumber, r1.CommitNumber, []string{traceKey1})
	if skipTestIfSpannerEmulatorNotSupported(t, err) {
		return
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
	r := generateNewRegression(subName)

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
	if skipTestIfSpannerEmulatorNotSupported(t, err) {
		return
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

// TestGetRegressionsBySubName tests the GetRegressionsBySubName method.
func TestGetRegressionsBySubName(t *testing.T) {
	alertsProvider := alerts_mock.NewConfigProvider(t)
	store := setupStore(t, alertsProvider)
	ctx := context.Background()

	// 1. Setup: Insert two regressions to test sorting and pagination.
	r1 := generateAndStoreNewRegression(ctx, t, store, subName)
	r2 := generateAndStoreNewRegression(ctx, t, store, subName)
	rImp := generateNewRegression(subName)
	populateRegression2Fields(rImp)
	rImp.Frame.DataFrame.ParamSet = map[string][]string{
		"improvement_direction": {"down"},
	}
	if rImp.High != nil && rImp.High.StepFit != nil {
		rImp.High.StepFit.Status = stepfit.LOW
	}
	_, err := store.WriteRegression(ctx, rImp, nil)
	assert.Nil(t, err)

	// Ensure r1 is older than r2.
	olderTime := time.Now().Add(-1 * time.Hour)
	_, err = store.db.Exec(ctx, "UPDATE Regressions2 SET creation_time = $1 WHERE id = $2", olderTime, r1.Id)
	require.NoError(t, err)

	// 2. Associate all regressions with the same subscription (sub_name).
	setupAlertSubName := func(alertID int64, subName string) {
		query := `
			INSERT INTO Alerts (id, sub_name)
			VALUES ($1, $2)
			ON CONFLICT (id)
			DO UPDATE SET
				id = EXCLUDED.id,
				sub_name = EXCLUDED.sub_name`
		_, err := store.db.Exec(ctx, query, alertID, subName)
		require.NoError(t, err)
	}
	setupAlertSubName(r1.AlertId, "my-sub")
	setupAlertSubName(r2.AlertId, "my-sub")
	setupAlertSubName(rImp.AlertId, "my-sub")

	// Sorted from newest to oldest
	rIds := []string{rImp.Id, r2.Id, r1.Id}

	// 3. Test cases.
	tests := []struct {
		name         string
		subName      string
		limit        int
		offset       int
		expectedLen  int
		improvements bool
		// expectedIDs is used to verify the exact order of return
		expectedIDs []string
	}{
		{
			name:         "happy path - get all (newest first)",
			subName:      "my-sub",
			limit:        10,
			offset:       0,
			expectedLen:  3,
			improvements: true,
			expectedIDs:  rIds,
		},
		{
			name:         "pagination - limit 1 (get newest)",
			subName:      "my-sub",
			limit:        1,
			offset:       0,
			expectedLen:  1,
			improvements: true,
			expectedIDs:  []string{rImp.Id},
		},
		{
			name:         "pagination - get oldest via offset manipulation",
			subName:      "my-sub",
			limit:        10,
			offset:       len(rIds) - 1,
			expectedLen:  1,
			improvements: true,
			expectedIDs:  []string{r1.Id},
		},
		{
			name:         "no regressions for sub name",
			subName:      "non-existent-sub",
			limit:        10,
			offset:       0,
			expectedLen:  0,
			improvements: true,
			expectedIDs:  []string{},
		},
		{
			name:         "limit 0 returns nothing",
			subName:      "my-sub",
			limit:        0,
			offset:       0,
			expectedLen:  0,
			improvements: true,
			expectedIDs:  []string{},
		},
		{
			name:         "hide improvements",
			subName:      "my-sub",
			limit:        10,
			offset:       0,
			expectedLen:  2,
			improvements: false,
			expectedIDs:  []string{r2.Id, r1.Id}, // Expect r2 (newer) then r1 (older)
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := regression.GetAnomalyListRequest{
				SubName:             tc.subName,
				PaginationOffset:    tc.offset,
				IncludeImprovements: tc.improvements,
				IncludeTriaged:      true,
			}
			regs, err := store.GetRegressionsBySubName(ctx, req, tc.limit)
			require.NoError(t, err)
			assert.Len(t, regs, tc.expectedLen)

			// Verify the order
			if tc.expectedLen > 0 {
				for i, expectedID := range tc.expectedIDs {
					assert.Equal(t, expectedID, regs[i].Id, "Regression at index %d did not match expected ID", i)
				}
			}
		})
	}
}

// TestGetRegressionsBySubName tests the GetRegressionsBySubName method.
func TestGetRegressionsBySubName_ShowHideTriaged(t *testing.T) {
	alertsProvider := alerts_mock.NewConfigProvider(t)
	store := setupStore(t, alertsProvider)
	ctx := context.Background()

	// Untriaged anomaly
	rUntriaged := generateAndStoreNewRegression(ctx, t, store, subName)
	// Both FileBug and AssociateAlerts just executes SetBugID (consider integration testing)
	rBugAssociated := generateAndStoreNewRegression(ctx, t, store, subName)
	err := store.SetBugID(ctx, []string{rBugAssociated.Id}, 1)
	require.NoError(t, err)
	// First associate a bug, then reset
	rReset := generateAndStoreNewRegression(ctx, t, store, subName)
	err = store.SetBugID(ctx, []string{rReset.Id}, 1)
	require.NoError(t, err)
	err = store.ResetAnomalies(ctx, []string{rReset.Id})
	require.NoError(t, err)
	// Ignore anomaly
	rIgnore := generateAndStoreNewRegression(ctx, t, store, subName)
	err = store.IgnoreAnomalies(ctx, []string{rIgnore.Id})
	require.NoError(t, err)

	rIds := []string{rIgnore.Id, rReset.Id, rBugAssociated.Id, rUntriaged.Id}

	// Associate all regressions with the same subscription (sub_name).
	setupAlertSubName := func(alertID int64, subName string) {
		query := `
			INSERT INTO Alerts (id, sub_name)
			VALUES ($1, $2)
			ON CONFLICT (id)
			DO UPDATE SET
				id = EXCLUDED.id,
				sub_name = EXCLUDED.sub_name`
		_, err := store.db.Exec(ctx, query, alertID, subName)
		require.NoError(t, err)
	}
	setupAlertSubName(rUntriaged.AlertId, subName)
	setupAlertSubName(rBugAssociated.AlertId, subName)
	setupAlertSubName(rIgnore.AlertId, subName)
	setupAlertSubName(rReset.AlertId, subName)

	// 3. Test cases.
	tests := []struct {
		name        string
		showTriaged bool
		// expectedIDs is used to verify the exact order of return
		expectedIDs []string
	}{
		{
			name:        "Hide triaged",
			showTriaged: false,
			expectedIDs: []string{rReset.Id, rUntriaged.Id},
		},
		{
			name:        "Show triaged",
			showTriaged: true,
			expectedIDs: rIds,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := regression.GetAnomalyListRequest{
				SubName:             subName,
				IncludeImprovements: false,
				IncludeTriaged:      tc.showTriaged,
			}
			regs, err := store.GetRegressionsBySubName(ctx, req, 50)
			require.NoError(t, err)
			assert.Len(t, regs, len(tc.expectedIDs))

			ids := make([]string, len(regs))
			for i, r := range regs {
				ids[i] = r.Id
			}

			for _, id := range tc.expectedIDs {
				require.Contains(t, ids, id)
			}
		})
	}
}

func TestSetBugID_Success(t *testing.T) {
	alertsProvider := alerts_mock.NewConfigProvider(t)
	store := setupStore(t, alertsProvider)
	ctx := context.Background()
	// Insert some regressions to update.
	regressions := []*regression.Regression{
		generateNewRegression(subName),
		generateNewRegression(subName),
		generateNewRegression(subName),
	}
	regIDs := []string{}
	for _, reg := range regressions {
		_, err := store.WriteRegression(ctx, reg, nil)
		require.NoError(t, err)
		regIDs = append(regIDs, reg.Id)
	}

	bugID := 12345
	idsToUpdate := []string{regIDs[0], regIDs[1]}

	err := store.SetBugID(ctx, idsToUpdate, bugID)
	require.NoError(t, err)

	// Verify that the bug_id was updated for reg1 and reg2.
	for _, id := range idsToUpdate {
		regs, err := store.GetByIDs(ctx, []string{id})
		require.NoError(t, err)
		require.Len(t, regs, 1)
		assert.Equal(t, 1, len(regs[0].Bugs))
		assert.Equal(t, fmt.Sprint(bugID), regs[0].Bugs[0].BugId)
		assert.Equal(t, regression.Negative, regs[0].HighStatus.Status)
	}

	// Verify that bug_id was not updated for reg3.
	regs, err := store.GetByIDs(ctx, []string{regIDs[2]})
	require.NoError(t, err)
	require.Len(t, regs, 1)
	assert.Equal(t, 0, len(regs[0].Bugs))
	assert.NotEqual(t, regression.Negative, regs[0].HighStatus.Status)
}

func TestSetBugID_NoIDs(t *testing.T) {
	alertsProvider := alerts_mock.NewConfigProvider(t)
	store := setupStore(t, alertsProvider)
	ctx := context.Background()

	err := store.SetBugID(ctx, []string{}, 12345)
	require.NoError(t, err)
}

func TestResetAnomalies_Success(t *testing.T) {
	alertsProvider := alerts_mock.NewConfigProvider(t)
	store := setupStore(t, alertsProvider)
	ctx := context.Background()
	// Insert some regressions to update.
	regressions := []*regression.Regression{
		generateNewRegression(subName),
		generateNewRegression(subName),
		generateNewRegression(subName),
	}

	bugIdDefault := int64(1)
	statusDefault := regression.Negative
	messageDefault := "foo"

	regIDs := []string{}
	for _, reg := range regressions {
		// set the following fields to see if reset works.
		reg.Bugs = []types.RegressionBug{{BugId: fmt.Sprint(bugIdDefault), Type: types.ManualTriage}}
		reg.HighStatus.Status = statusDefault
		reg.HighStatus.Message = messageDefault
		_, err := store.WriteRegression(ctx, reg, nil)
		require.NoError(t, err)
		regIDs = append(regIDs, reg.Id)
	}

	idsToUpdate := []string{regIDs[0], regIDs[1]}

	err := store.ResetAnomalies(ctx, idsToUpdate)
	require.NoError(t, err)

	// Verify that the bug_id and triage status were updated for reg1 and reg2.
	for _, id := range idsToUpdate {
		regs, err := store.GetByIDs(ctx, []string{id})
		require.NoError(t, err)
		require.Len(t, regs, 1)
		assert.Equal(t, 0, len(regs[0].Bugs))
		// generateNewRegression sets High, so we expect HighStatus to be updated.
		assert.Equal(t, regression.Untriaged, regs[0].HighStatus.Status)
		assert.Equal(t, regression.ResetMessage, regs[0].HighStatus.Message)
	}

	// Verify that nothing was updated for reg3.
	regs, err := store.GetByIDs(ctx, []string{regIDs[2]})
	require.NoError(t, err)
	require.Len(t, regs, 1)
	assert.Equal(t, 1, len(regs[0].Bugs))
	assert.Equal(t, fmt.Sprint(1), regs[0].Bugs[0].BugId)
	assert.NotEqual(t, regression.Untriaged, regs[0].HighStatus.Status)
	assert.Equal(t, statusDefault, regs[0].HighStatus.Status)
	assert.Equal(t, messageDefault, regs[0].HighStatus.Message)
}

func TestIgnoreAnomalies_Success(t *testing.T) {
	alertsProvider := alerts_mock.NewConfigProvider(t)
	store := setupStore(t, alertsProvider)
	ctx := context.Background()
	// Insert some regressions to update.
	regressions := []*regression.Regression{
		generateNewRegression(subName),
		generateNewRegression(subName),
		generateNewRegression(subName),
	}
	regIDs := []string{}
	for _, reg := range regressions {
		_, err := store.WriteRegression(ctx, reg, nil)
		require.NoError(t, err)
		regIDs = append(regIDs, reg.Id)
	}

	idsToUpdate := []string{regIDs[0], regIDs[1]}

	err := store.IgnoreAnomalies(ctx, idsToUpdate)
	require.NoError(t, err)

	// Verify that the triage status was updated for reg1 and reg2.
	for _, id := range idsToUpdate {
		regs, err := store.GetByIDs(ctx, []string{id})
		require.NoError(t, err)
		require.Len(t, regs, 1)
		// generateNewRegression sets High, so we expect HighStatus to be updated.
		assert.Equal(t, regression.Ignored, regs[0].HighStatus.Status)
		assert.Equal(t, regression.IgnoredMessage, regs[0].HighStatus.Message)
	}

	// Verify that nothing was updated for reg3.
	regs, err := store.GetByIDs(ctx, []string{regIDs[2]})
	require.NoError(t, err)
	require.Len(t, regs, 1)
	assert.NotEqual(t, regression.Ignored, regs[0].HighStatus.Status)
}

func TestNudgeAndResetAnomalies_ResetsStatus(t *testing.T) {
	alertsProvider := alerts_mock.NewConfigProvider(t)
	store := setupStore(t, alertsProvider)
	ctx := context.Background()
	// Insert some regressions to update.
	regressions := []*regression.Regression{
		generateNewRegression(subName),
		generateNewRegression(subName),
		generateNewRegression(subName),
	}
	// store.TriageHigh sets status and message on all regressions with the same commit number and alert id.
	// That's why we change this regression to be on a different commit number.
	regressions[2].CommitNumber = regressions[2].CommitNumber + 1
	bugIdDefault := int64(1)
	statusDefault := regression.Negative
	messageDefault := "foo"

	regIDs := []string{}
	for _, reg := range regressions {
		reg.Bugs = []types.RegressionBug{{BugId: fmt.Sprint(bugIdDefault), Type: types.ManualTriage}}
		reg.HighStatus = regression.TriageStatus{
			Status:  statusDefault,
			Message: messageDefault,
		}
		_, err := store.WriteRegression(ctx, reg, nil)
		require.NoError(t, err)
		regIDs = append(regIDs, reg.Id)
	}

	// Set a bug ID and triage status for the first regression to verify it gets reset.
	err := store.SetBugID(ctx, []string{regIDs[0]}, 12345)
	require.NoError(t, err)
	err = store.TriageHigh(ctx, regressions[0].CommitNumber, alerts.IDToString(regressions[0].AlertId), regression.TriageStatus{Status: regression.Positive, Message: "foo"})
	require.NoError(t, err)

	idsToUpdate := []string{regIDs[0], regIDs[1]}
	newCommitNumber := types.CommitNumber(100)
	newPrevCommitNumber := types.CommitNumber(90)

	err = store.NudgeAndResetAnomalies(ctx, idsToUpdate, newCommitNumber, newPrevCommitNumber)
	require.NoError(t, err)

	// Verify that the bug_id and triage status were updated for reg1 and reg2.
	for _, id := range idsToUpdate {
		regs, err := store.GetByIDs(ctx, []string{id})
		require.NoError(t, err)
		require.Len(t, regs, 1)
		assert.Equal(t, 0, len(regs[0].Bugs))
		assert.Equal(t, newCommitNumber, regs[0].CommitNumber)
		assert.Equal(t, newPrevCommitNumber, regs[0].PrevCommitNumber)
		// generateNewRegression sets High, so we expect HighStatus to be updated.
		assert.Equal(t, regression.Untriaged, regs[0].HighStatus.Status)
		assert.Equal(t, regression.NudgedMessage, regs[0].HighStatus.Message)
	}
	// Verify that nothing was updated for reg3.
	regs, err := store.GetByIDs(ctx, []string{regIDs[2]})
	require.NoError(t, err)
	require.Len(t, regs, 1)
	assert.Equal(t, 1, len(regs[0].Bugs))
	assert.Equal(t, fmt.Sprint(bugIdDefault), regs[0].Bugs[0].BugId)
	assert.Equal(t, statusDefault, regs[0].HighStatus.Status)
	assert.Equal(t, messageDefault, regs[0].HighStatus.Message)
}

func TestGetSubscriptionsForRegressions(t *testing.T) {
	alertsProvider := alerts_mock.NewConfigProvider(t)
	store := setupStore(t, alertsProvider)
	ctx := context.Background()

	alertId1 := int64(1)
	alertId2 := int64(2)
	alertId3 := int64(3)
	component1 := "123456"
	component2 := "123467"

	// 1. Setup: Insert regressions, alerts, and subscriptions.
	reg1 := generateNewRegression(subName)
	reg1.AlertId = alertId1
	_, err := store.WriteRegression(ctx, reg1, nil)
	assert.Nil(t, err)

	reg2 := generateNewRegression(subName)
	reg2.AlertId = alertId2
	_, err = store.WriteRegression(ctx, reg2, nil)
	assert.Nil(t, err)

	reg3WithoutSubscription := generateNewRegression(subName)
	reg3WithoutSubscription.AlertId = alertId3
	_, err = store.WriteRegression(ctx, reg3WithoutSubscription, nil)
	assert.Nil(t, err)

	// Setup Alerts
	setupAlert := func(alertID int64, subName string, subRevision string) {
		query := `
			INSERT INTO Alerts (id, sub_name, sub_revision)
			VALUES ($1, $2, $3)
			ON CONFLICT (id)
			DO UPDATE SET
				id = EXCLUDED.id,
				sub_name = EXCLUDED.sub_name,
				sub_revision = EXCLUDED.sub_revision`
		_, err := store.db.Exec(ctx, query, alertID, subName, subRevision)
		require.NoError(t, err)
	}
	setupAlert(reg1.AlertId, "sub1", "1")
	setupAlert(reg2.AlertId, "sub2", "1")
	setupAlert(reg3WithoutSubscription.AlertId, "sub-without-subscription", "1")

	// Setup Subscriptions
	setupSubscription := func(name string, revision string, component string) {
		query := `
			INSERT INTO Subscriptions (name, revision, bug_component)
			VALUES ($1, $2, $3)
			ON CONFLICT (name, revision)
			DO UPDATE SET
				name = EXCLUDED.name,
				revision = EXCLUDED.revision,
				bug_component = EXCLUDED.bug_component`
		_, err := store.db.Exec(ctx, query, name, revision, component)
		require.NoError(t, err)
	}
	setupSubscription("sub1", "1", component1)
	setupSubscription("sub2", "1", component2)

	// 2. Test Cases
	tests := []struct {
		name                  string
		regressionIDs         []string
		expectedRegressionIDs []string
		expectedAlertIDs      []int64
		expectedBugComponents []string
		expectError           bool
		expectedErrorContains string
	}{
		{
			name:                  "happy path - get multiple subscriptions",
			regressionIDs:         []string{reg1.Id, reg2.Id},
			expectedRegressionIDs: []string{reg1.Id, reg2.Id},
			expectedAlertIDs:      []int64{reg1.AlertId, reg2.AlertId},
			expectedBugComponents: []string{component1, component2},
		},
		{
			name:                  "single regression",
			regressionIDs:         []string{reg1.Id},
			expectedRegressionIDs: []string{reg1.Id},
			expectedAlertIDs:      []int64{reg1.AlertId},
			expectedBugComponents: []string{component1},
		},
		{
			name:                  "regression without subscription",
			regressionIDs:         []string{reg3WithoutSubscription.Id},
			expectedRegressionIDs: nil,
			expectedAlertIDs:      nil,
			expectedBugComponents: nil,
		},
		{
			name:                  "non-existent regression ID",
			regressionIDs:         []string{"non-existent-id"},
			expectedRegressionIDs: nil,
			expectedAlertIDs:      nil,
			expectedBugComponents: nil,
		},
		{
			name:                  "empty regression IDs",
			regressionIDs:         []string{},
			expectedRegressionIDs: nil,
			expectedAlertIDs:      nil,
			expectedBugComponents: nil,
		},
		{
			name:                  "mixed existent and non-existent",
			regressionIDs:         []string{reg1.Id, "non-existent-id"},
			expectedRegressionIDs: []string{reg1.Id},
			expectedAlertIDs:      []int64{reg1.AlertId},
			expectedBugComponents: []string{component1},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			regressionIDs, alertIDs, subs, err := store.GetSubscriptionsForRegressions(ctx, tc.regressionIDs)

			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErrorContains)
				return
			}

			require.NoError(t, err)
			assert.ElementsMatch(t, tc.expectedRegressionIDs, regressionIDs)
			assert.ElementsMatch(t, tc.expectedAlertIDs, alertIDs)

			if len(tc.expectedBugComponents) > 0 {
				bugComponents := []string{}
				for _, sub := range subs {
					bugComponents = append(bugComponents, sub.BugComponent)
				}
				assert.ElementsMatch(t, tc.expectedBugComponents, bugComponents)
			} else {
				assert.Empty(t, subs)
			}
		})
	}
}

func TestGetBugIdsForRegressions(t *testing.T) {
	alertsProvider := alerts_mock.NewConfigProvider(t)
	store := setupStore(t, alertsProvider)
	ctx := context.Background()

	// Test case 1: No bugs.
	t.Run("No bugs", func(t *testing.T) {
		r := generateAndStoreNewRegression(ctx, t, store, subName)
		regressions, err := store.GetBugIdsForRegressions(ctx, []*regression.Regression{r})
		require.NoError(t, err)
		require.Len(t, regressions, 1)
		assert.Empty(t, regressions[0].Bugs)
		assert.True(t, regressions[0].AllBugsFetched)
	})

	// Test case 2: Manual bug only.
	t.Run("Manual bug only", func(t *testing.T) {
		r := generateNewRegression(subName)
		manualBug := types.RegressionBug{BugId: "12345", Type: types.ManualTriage}
		r.Bugs = []types.RegressionBug{manualBug}
		_, err := store.WriteRegression(ctx, r, nil)
		require.NoError(t, err)

		// Get the regression from DB to make sure manual bug is loaded.
		regressionsFromDB, err := store.GetByIDs(ctx, []string{r.Id})
		require.NoError(t, err)
		require.Len(t, regressionsFromDB, 1)

		regressions, err := store.GetBugIdsForRegressions(ctx, regressionsFromDB)
		require.NoError(t, err)
		require.Len(t, regressions, 1)
		assert.ElementsMatch(t, []types.RegressionBug{manualBug}, regressions[0].Bugs)
		assert.True(t, regressions[0].AllBugsFetched)
	})

	// Test case 3: Auto-triaged bug.
	t.Run("Auto-triaged bug", func(t *testing.T) {
		r := generateAndStoreNewRegression(ctx, t, store, subName)
		agID := uuid.NewString()
		reportedIssueID := "123456"
		_, err := store.db.Exec(ctx, `
			INSERT INTO AnomalyGroups (id, anomaly_ids, common_rev_start, common_rev_end, reported_issue_id)
			VALUES ($1, $2, $3, $4, $5)`,
			agID, []string{r.Id}, r.PrevCommitNumber, r.CommitNumber, reportedIssueID)
		require.NoError(t, err)

		regressions, err := store.GetBugIdsForRegressions(ctx, []*regression.Regression{r})
		require.NoError(t, err)
		require.Len(t, regressions, 1)
		expectedBugs := []types.RegressionBug{
			{BugId: reportedIssueID, Type: types.AutoTriage},
		}
		assert.ElementsMatch(t, expectedBugs, regressions[0].Bugs)
		assert.True(t, regressions[0].AllBugsFetched)
	})

	// Test case 4: Auto-triaged bug with invalid (start > end) groups.
	// We have to filter out groups with start > end revisions as they are incorrect.
	t.Run("Auto-triaged bug with invalid groups", func(t *testing.T) {
		r := generateAndStoreNewRegression(ctx, t, store, subName)
		agID := uuid.NewString()
		reportedIssueID := "123456"
		_, err := store.db.Exec(ctx, `
			INSERT INTO AnomalyGroups (id, anomaly_ids, common_rev_start, common_rev_end, reported_issue_id)
			VALUES ($1, $2, $3, $4, $5)`,
			agID, []string{r.Id}, r.PrevCommitNumber, r.CommitNumber, reportedIssueID)
		require.NoError(t, err)

		agID2 := uuid.NewString()
		reportedIssueID2 := "67"
		_, err = store.db.Exec(ctx, `
			INSERT INTO AnomalyGroups (id, anomaly_ids, common_rev_start, common_rev_end, reported_issue_id)
			VALUES ($1, $2, $3, $4, $5)`,
			agID2, []string{r.Id}, r.CommitNumber+1, r.CommitNumber, reportedIssueID2)
		require.NoError(t, err)

		regressions, err := store.GetBugIdsForRegressions(ctx, []*regression.Regression{r})
		require.NoError(t, err)
		require.Len(t, regressions, 1)
		expectedBugs := []types.RegressionBug{
			{BugId: reportedIssueID, Type: types.AutoTriage},
		}
		assert.ElementsMatch(t, expectedBugs, regressions[0].Bugs)
		assert.True(t, regressions[0].AllBugsFetched)
	})

	// Test case 5: Auto-bisect bug from culprit (group_issue_map).
	t.Run("Auto-bisect bug from group_issue_map", func(t *testing.T) {
		r := generateAndStoreNewRegression(ctx, t, store, subName)
		r2 := generateAndStoreNewRegression(ctx, t, store, subName)
		agID := uuid.NewString()
		agID2 := uuid.NewString()
		culpritID := uuid.NewString()
		bisectIssueID := "1234567"
		bisectIssueID2 := "12345672"
		groupIssueMap := fmt.Sprintf(`{"%s": "%s", "%s": "%s"}`, agID, bisectIssueID, agID2, bisectIssueID2)

		_, err := store.db.Exec(ctx, `
			INSERT INTO AnomalyGroups (id, anomaly_ids, common_rev_start, common_rev_end)
			VALUES ($1, $2, $3, $4)`,
			agID, []string{r.Id}, r.PrevCommitNumber, r.CommitNumber)
		require.NoError(t, err)

		_, err = store.db.Exec(ctx, `
			INSERT INTO AnomalyGroups (id, anomaly_ids, common_rev_start, common_rev_end)
			VALUES ($1, $2, $3, $4)`,
			agID2, []string{r2.Id}, r2.PrevCommitNumber, r2.CommitNumber)
		require.NoError(t, err)

		_, err = store.db.Exec(ctx, `
			INSERT INTO Culprits (id, host, project, ref, revision, anomaly_group_ids, group_issue_map, issue_ids)
			VALUES ($1, 'host', 'project', 'ref', 'rev', $2, $3, $4)`,
			culpritID, []string{agID, agID2}, groupIssueMap, []string{bisectIssueID, bisectIssueID2})
		require.NoError(t, err)

		regressions, err := store.GetBugIdsForRegressions(ctx, []*regression.Regression{r})
		require.NoError(t, err)
		require.Len(t, regressions, 1)
		expectedBugs := []types.RegressionBug{
			{BugId: bisectIssueID, Type: types.AutoBisect},
		}
		assert.ElementsMatch(t, expectedBugs, regressions[0].Bugs)
	})

	// Test case 6: All bug types together.
	t.Run("All bug types together", func(t *testing.T) {
		r := generateNewRegression(subName)
		manualBug := types.RegressionBug{BugId: "123", Type: types.ManualTriage}
		r.Bugs = []types.RegressionBug{manualBug}
		_, err := store.WriteRegression(ctx, r, nil)
		require.NoError(t, err)

		// Auto-triage bug
		agID1 := uuid.NewString()
		reportedIssueID := "456"
		_, err = store.db.Exec(ctx, `
			INSERT INTO AnomalyGroups (id, anomaly_ids, common_rev_start, common_rev_end, reported_issue_id)
			VALUES ($1, $2, $3, $4, $5)`,
			agID1, []string{r.Id}, r.PrevCommitNumber, r.CommitNumber, reportedIssueID)
		require.NoError(t, err)

		// Auto-bisect bug
		agID2 := uuid.NewString()
		culpritID := uuid.NewString()
		bisectIssueID := "789"
		groupIssueMap := fmt.Sprintf(`{"%s": "%s"}`, agID2, bisectIssueID)
		_, err = store.db.Exec(ctx, `
			INSERT INTO AnomalyGroups (id, anomaly_ids, common_rev_start, common_rev_end)
			VALUES ($1, $2, $3, $4)`,
			agID2, []string{r.Id}, r.PrevCommitNumber, r.CommitNumber)
		require.NoError(t, err)
		_, err = store.db.Exec(ctx, `
			INSERT INTO Culprits (id, host, project, ref, revision, anomaly_group_ids, group_issue_map, issue_ids)
			VALUES ($1, 'host', 'project', 'ref', 'rev', $2, $3, $4)`,
			culpritID, []string{agID2}, groupIssueMap, []string{bisectIssueID})
		require.NoError(t, err)

		// Fetch the manual bug first.
		regressionsFromDB, err := store.GetByIDs(ctx, []string{r.Id})
		require.NoError(t, err)
		require.Len(t, regressionsFromDB, 1)

		regressions, err := store.GetBugIdsForRegressions(ctx, regressionsFromDB)
		require.NoError(t, err)
		require.Len(t, regressions, 1)

		expectedBugs := []types.RegressionBug{
			manualBug,
			{BugId: reportedIssueID, Type: types.AutoTriage},
			{BugId: bisectIssueID, Type: types.AutoBisect},
		}
		assert.ElementsMatch(t, expectedBugs, regressions[0].Bugs)
	})

	// Test case 7: Sorting order of bugs (manual, autotriage, autobisect, then by ID).
	t.Run("Bug sorting order", func(t *testing.T) {
		r := generateNewRegression(subName)
		r1 := generateNewRegression(subName)
		if r.Id > r1.Id {
			r.Id, r1.Id = r1.Id, r.Id
		}
		assert.True(t, r.Id < r1.Id)
		_, err := store.WriteRegression(ctx, r, nil)
		require.NoError(t, err)
		_, err = store.WriteRegression(ctx, r1, nil)
		require.NoError(t, err)

		// Initialize with a mixed order to ensure sorting works

		manualBug1 := types.RegressionBug{BugId: "100", Type: types.ManualTriage}
		err = store.SetBugID(ctx, []string{r.Id}, 100)
		require.NoError(t, err)

		manualBug2 := types.RegressionBug{BugId: "399", Type: types.ManualTriage}
		err = store.SetBugID(ctx, []string{r1.Id}, 399)
		require.NoError(t, err)

		autoTriageBug1 := types.RegressionBug{BugId: "250", Type: types.AutoTriage}
		autoTriageBug2 := types.RegressionBug{BugId: "200", Type: types.AutoTriage}
		autoBisectBug1 := types.RegressionBug{BugId: "301", Type: types.AutoBisect}
		autoBisectBug2 := types.RegressionBug{BugId: "99", Type: types.AutoBisect} // For tie-breaking

		// Auto-triage bug
		agID1 := uuid.NewString()
		_, err = store.db.Exec(ctx, `
			INSERT INTO AnomalyGroups (id, anomaly_ids, common_rev_start, common_rev_end, reported_issue_id)
			VALUES ($1, $2, $3, $4, $5)`,
			agID1, []string{r.Id, r1.Id}, r.PrevCommitNumber, r.CommitNumber, autoTriageBug1.BugId)
		require.NoError(t, err)

		// Auto-triage bug
		agID4 := uuid.NewString()
		_, err = store.db.Exec(ctx, `
			INSERT INTO AnomalyGroups (id, anomaly_ids, common_rev_start, common_rev_end, reported_issue_id)
			VALUES ($1, $2, $3, $4, $5)`,
			agID4, []string{r1.Id}, r.PrevCommitNumber, r.CommitNumber, autoTriageBug2.BugId)
		require.NoError(t, err)

		// Auto-bisect bugs
		agID2 := uuid.NewString()
		agID3 := uuid.NewString() // For the second auto-bisect bug
		culpritID := uuid.NewString()
		groupIssueMap := fmt.Sprintf(`{"%s": "%s", "%s": "%s"}`, agID2, autoBisectBug1.BugId, agID3, autoBisectBug2.BugId)

		_, err = store.db.Exec(ctx, `
			INSERT INTO AnomalyGroups (id, anomaly_ids, common_rev_start, common_rev_end)
			VALUES ($1, $2, $3, $4)`,
			agID2, []string{r.Id, r1.Id}, r.PrevCommitNumber, r.CommitNumber)
		require.NoError(t, err)

		_, err = store.db.Exec(ctx, `
			INSERT INTO AnomalyGroups (id, anomaly_ids, common_rev_start, common_rev_end)
			VALUES ($1, $2, $3, $4)`,
			agID3, []string{r.Id}, r.PrevCommitNumber, r.CommitNumber)
		require.NoError(t, err)

		_, err = store.db.Exec(ctx, `
			INSERT INTO Culprits (id, host, project, ref, revision, anomaly_group_ids, group_issue_map, issue_ids)
			VALUES ($1, 'host', 'project', 'ref', 'rev', $2, $3, $4)`,
			culpritID, []string{agID2, agID3}, groupIssueMap, []string{autoBisectBug1.BugId, autoBisectBug2.BugId})
		require.NoError(t, err)

		regressionsFromDB, err := store.GetByIDs(ctx, []string{r.Id, r1.Id})
		require.NoError(t, err)
		require.Len(t, regressionsFromDB, 2)

		regressions, err := store.GetBugIdsForRegressions(ctx, regressionsFromDB)
		require.NoError(t, err)
		require.Len(t, regressions, len(regressionsFromDB))

		expectedBugs := []types.RegressionBug{
			manualBug1,
			autoTriageBug1,
			autoBisectBug2,
			autoBisectBug1,
		}
		expectedBugs1 := []types.RegressionBug{
			manualBug2,
			autoTriageBug2,
			autoTriageBug1,
			autoBisectBug1,
		}
		assert.Equal(t, regressions[0].Id, r.Id)
		assert.Equal(t, expectedBugs, regressions[0].Bugs)
		assert.Equal(t, expectedBugs1, regressions[1].Bugs)
		assert.True(t, regressions[0].AllBugsFetched)
		assert.True(t, regressions[1].AllBugsFetched)
	})
}

// TestGetIdsByManualTriageBugID tests the GetIdsByManualTriageBugID method.
func TestGetIdsByManualTriageBugID(t *testing.T) {
	alertsProvider := alerts_mock.NewConfigProvider(t)
	store := setupStore(t, alertsProvider)
	ctx := context.Background()

	// 1. Setup: Insert some regressions and assign manual triage bug IDs.
	r1 := generateAndStoreNewRegression(ctx, t, store, subName)
	r2 := generateAndStoreNewRegression(ctx, t, store, subName)
	r3 := generateAndStoreNewRegression(ctx, t, store, subName)

	bugID1 := 10001
	bugID2 := 10002

	err := store.SetBugID(ctx, []string{r1.Id, r2.Id}, bugID1)
	require.NoError(t, err)

	err = store.SetBugID(ctx, []string{r3.Id}, bugID2)
	require.NoError(t, err)

	tests := []struct {
		name          string
		bugID         int
		expectedCount int
		expectedIDs   []string
	}{
		{
			name:          "find two regressions with bugID1",
			bugID:         bugID1,
			expectedCount: 2,
			expectedIDs:   []string{r1.Id, r2.Id},
		},
		{
			name:          "find one regression with bugID2",
			bugID:         bugID2,
			expectedCount: 1,
			expectedIDs:   []string{r3.Id},
		},
		{
			name:          "no regressions found for non-existent bug ID",
			bugID:         99999,
			expectedCount: 0,
			expectedIDs:   []string{},
		},
		{
			name:          "zero bug ID returns nothing",
			bugID:         0,
			expectedCount: 0,
			expectedIDs:   []string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			regIds, err := store.GetIdsByManualTriageBugID(ctx, tc.bugID)
			require.NoError(t, err)
			assert.Len(t, regIds, tc.expectedCount)

			actualIDs := []string{}
			for _, regId := range regIds {
				actualIDs = append(actualIDs, regId)
			}
			assert.ElementsMatch(t, tc.expectedIDs, actualIDs)
		})
	}
}

func TestAllowMultipleRegressionsPerAlertId(t *testing.T) {
	const (
		commitNumber = types.CommitNumber(12345)
		traceKey1    = ",test=trace1,"
		traceKey2    = ",test=trace2,"
	)
	alertIdStr := alerts.IDToString(alertId)
	clusterSummary1 := &clustering2.ClusterSummary{
		Centroid: []float32{1.0, 2.0, 3.0},
		StepFit: &stepfit.StepFit{
			TurningPoint: 1,
		},
	}
	clusterSummary2 := &clustering2.ClusterSummary{
		Centroid: []float32{4.0, 5.0, 6.0},
		StepFit: &stepfit.StepFit{
			TurningPoint: 1,
		},
	}
	frameResponse1 := &frame.FrameResponse{
		DataFrame: &dataframe.DataFrame{
			Header: []*dataframe.ColumnHeader{
				{Offset: 1}, {Offset: 2}, {Offset: 3},
			},
			TraceSet: types.TraceSet{traceKey1: {}},
		},
	}
	frameResponse2 := &frame.FrameResponse{
		DataFrame: &dataframe.DataFrame{
			Header: []*dataframe.ColumnHeader{
				{Offset: 1}, {Offset: 2}, {Offset: 3},
			},
			TraceSet: types.TraceSet{traceKey2: {}},
		},
	}

	alertsProvider := alerts_mock.NewConfigProvider(t)
	alertsProvider.On("GetAlertConfig", alertId).Return(&alerts.Alert{
		IDAsString:  alertIdStr,
		DisplayName: "Test Alert Config",
		Algo:        types.StepFitGrouping,
	}, nil)

	ctx := context.Background()

	t.Run("AllowMultipleRegressionsPerAlertId is true", func(t *testing.T) {
		db := sqltest.NewSpannerDBForTests(t, "regstore_multi_true")
		instanceConfig := &config.InstanceConfig{
			AllowMultipleRegressionsPerAlertId: true,
		}
		store, err := New(db, alertsProvider, instanceConfig)
		require.NoError(t, err)

		// Set first regression.
		success, _, err := store.SetHigh(ctx, commitNumber, alertIdStr, frameResponse1, clusterSummary1)
		if skipTestIfSpannerEmulatorNotSupported(t, err) {
			return
		}
		assert.NoError(t, err)
		assert.True(t, success)

		// Set second regression for the same alert id but different trace.
		success, _, err = store.SetHigh(ctx, commitNumber, alertIdStr, frameResponse2, clusterSummary2)
		assert.NoError(t, err)
		assert.True(t, success)

		// Verify both regressions are stored.
		regressionsFromDb, err := store.RangeFiltered(ctx, commitNumber, commitNumber, []string{traceKey1, traceKey2})
		if skipTestIfSpannerEmulatorNotSupported(t, err) {
			return
		}
		assert.NoError(t, err)
		assert.Len(t, regressionsFromDb, 2)

		// Check that we have one for each trace.
		var foundTrace1, foundTrace2 bool
		for _, reg := range regressionsFromDb {
			if _, ok := reg.Frame.DataFrame.TraceSet[traceKey1]; ok {
				foundTrace1 = true
			}
			if _, ok := reg.Frame.DataFrame.TraceSet[traceKey2]; ok {
				foundTrace2 = true
			}
		}
		assert.True(t, foundTrace1, "Did not find regression for trace1")
		assert.True(t, foundTrace2, "Did not find regression for trace2")
	})

	t.Run("AllowMultipleRegressionsPerAlertId is false", func(t *testing.T) {
		db := sqltest.NewSpannerDBForTests(t, "regstore_multi_false")
		instanceConfig := &config.InstanceConfig{
			AllowMultipleRegressionsPerAlertId: false,
		}
		store, err := New(db, alertsProvider, instanceConfig)
		require.NoError(t, err)

		// Set first regression.
		success, _, err := store.SetHigh(ctx, commitNumber, alertIdStr, frameResponse1, clusterSummary1)
		if skipTestIfSpannerEmulatorNotSupported(t, err) {
			return
		}
		assert.NoError(t, err)
		assert.True(t, success)

		// Set second regression for the same alert id should fail to add a new one.
		success, _, err = store.SetHigh(ctx, commitNumber, alertIdStr, frameResponse2, clusterSummary2)
		assert.NoError(t, err)
		assert.False(t, success, "A new regression should not have been created.")

		// Verify only one regression is stored.
		regressionsFromDb, err := store.RangeFiltered(ctx, commitNumber, commitNumber, []string{traceKey1, traceKey2})
		if skipTestIfSpannerEmulatorNotSupported(t, err) {
			return
		}
		assert.NoError(t, err)
		assert.Len(t, regressionsFromDb, 1)

		_, ok := regressionsFromDb[0].Frame.DataFrame.TraceSet[traceKey1]
		assert.True(t, ok)
		assert.Equal(t, clusterSummary1, regressionsFromDb[0].High)
	})
}

// TestUpdateBasedOnAlertAlgo_WithSubscriptionName verifies that when an alert has an associated
// subscription name, any new regressions created for that alert will also have the same
// subscription name.
func TestUpdateBasedOnAlertAlgo_WithSubscriptionName(t *testing.T) {
	const subName = "test-subscription"
	alertsProvider := alerts_mock.NewConfigProvider(t)
	alertsProvider.On("GetAlertConfig", alertId).Return(&alerts.Alert{
		IDAsString:       fmt.Sprintf("%d", alertId),
		DisplayName:      "Test Alert Config",
		Algo:             types.KMeansGrouping,
		SubscriptionName: subName,
	}, nil)

	store := setupStore(t, alertsProvider)
	ctx := context.Background()

	// Add an item to the database.
	r := generateNewRegression(subName)

	alertIdStr := alerts.IDToString(r.AlertId)
	clusterSummary := &clustering2.ClusterSummary{
		Centroid: []float32{1.0, 2.0, 3.0},
		StepFit: &stepfit.StepFit{
			TurningPoint: 1,
		},
	}
	frameResponse := &frame.FrameResponse{
		DataFrame: &dataframe.DataFrame{
			Header: []*dataframe.ColumnHeader{
				{Offset: 1}, {Offset: 2}, {Offset: 3},
			},
		},
	}

	// Set a high regression.
	success, _, err := store.SetHigh(ctx, r.CommitNumber, alertIdStr, frameResponse, clusterSummary)
	if skipTestIfSpannerEmulatorNotSupported(t, err) {
		return
	}
	assert.Nil(t, err)
	assert.True(t, success)

	// Read the regression and verify that SubscriptionName was set correctly.
	reg := readSpecificRegressionFromDb(ctx, t, store, r.CommitNumber, alertIdStr)
	assert.NotNil(t, reg)
	assert.Equal(t, subName, reg.SubscriptionName)
}
