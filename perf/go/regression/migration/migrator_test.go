package migration

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/regression"
	"go.skia.org/infra/perf/go/regression/sqlregression2store"
	"go.skia.org/infra/perf/go/regression/sqlregressionstore"
	"go.skia.org/infra/perf/go/sql/sqltest"
	"go.skia.org/infra/perf/go/stepfit"
	"go.skia.org/infra/perf/go/types"
	"go.skia.org/infra/perf/go/ui/frame"
)

func setup(t *testing.T) (context.Context, *RegressionMigrator, *sqlregressionstore.SQLRegressionStore, *sqlregression2store.SQLRegression2Store) {
	ctx := context.Background()
	db := sqltest.NewSpannerDBForTests(t, "regstore")
	instanceConfig := &config.InstanceConfig{
		AllowMultipleRegressionsPerAlertId: true,
	}
	migrator, _ := New(ctx, db, instanceConfig)
	legacyStore, _ := sqlregressionstore.New(db)
	newStore, _ := sqlregression2store.New(db, nil, instanceConfig)
	return ctx, migrator, legacyStore, newStore
}

func createLegacyRegressions(ctx context.Context, count int, commitNumber types.CommitNumber, alertID string, legacyStore *sqlregressionstore.SQLRegressionStore) {
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
	for i := 0; i < count; i++ {
		_, _, _ = legacyStore.SetHigh(ctx, commitNumber, alertID, df, clusterSummary)
	}
}

func assertRegressions(t *testing.T, legacy *regression.Regression, new *regression.Regression) {
	assert.Equal(t, legacy.Frame, new.Frame)
	assert.Equal(t, legacy.High, new.High)
}

func TestMigrate_Single_Success(t *testing.T) {
	ctx, migrator, legacyStore, newStore := setup(t)

	commitNumber := types.CommitNumber(1)
	alertID := "123"
	createLegacyRegressions(ctx, 1, commitNumber, alertID, legacyStore)
	err := migrator.migrateRegressions(ctx, 1)
	assert.Nil(t, err)

	regressionsMap, err := newStore.Range(ctx, commitNumber, commitNumber)
	assert.Nil(t, err)
	assert.NotNil(t, regressionsMap)
	regressionsForCommit := regressionsMap[commitNumber]
	newRegression := regressionsForCommit.ByAlertID[alertID]
	assert.NotNil(t, newRegression)

	legacyRegressionsMap, _ := legacyStore.Range(ctx, commitNumber, commitNumber)
	legacyRegression := legacyRegressionsMap[commitNumber].ByAlertID[alertID]

	// Compare the legacy values.
	assertRegressions(t, legacyRegression, newRegression)

	// Assert the new values specific to the regression2 schema
	assert.Equal(t, alertID, strconv.Itoa(int(newRegression.AlertId)))
	assert.Equal(t, commitNumber, newRegression.CommitNumber)
	assert.NotEmpty(t, newRegression.Id)

	// Ensure that no more legacy regressions are remaining to migrate.
	remainingRegressions, _ := legacyStore.GetRegressionsToMigrate(ctx, 1)
	assert.Equal(t, 0, len(remainingRegressions))
}

func TestMigrate_Multiple_Success(t *testing.T) {
	ctx, migrator, legacyStore, newStore := setup(t)
	alertID := "123"
	// Create 10 regressions for different commits
	for i := 1; i <= 10; i++ {
		commitNumber := types.CommitNumber(i)

		createLegacyRegressions(ctx, 1, commitNumber, alertID, legacyStore)
	}

	// Now let's migrate them in a batch of 10
	err := migrator.migrateRegressions(ctx, 10)
	assert.Nil(t, err)

	regressionsMap, err := newStore.Range(ctx, 1, 10)
	assert.Nil(t, err)
	assert.NotNil(t, regressionsMap)
	assert.Equal(t, 10, len(regressionsMap))

	legacyRegressionsMap, _ := legacyStore.Range(ctx, 1, 10)
	assert.Equal(t, 10, len(legacyRegressionsMap))
	for i := 1; i <= 10; i++ {
		commitNumber := types.CommitNumber(i)
		newRegression := regressionsMap[commitNumber].ByAlertID[alertID]
		oldRegression := legacyRegressionsMap[commitNumber].ByAlertID[alertID]
		assertRegressions(t, oldRegression, newRegression)
		assert.Equal(t, alertID, strconv.Itoa(int(newRegression.AlertId)))
		assert.Equal(t, commitNumber, newRegression.CommitNumber)
		assert.NotEmpty(t, newRegression.Id)
	}
}

func TestMigrate_Partial_Success(t *testing.T) {
	ctx, migrator, legacyStore, newStore := setup(t)
	alertID := "123"
	// Create 10 regressions for different commits
	for i := 1; i <= 10; i++ {
		commitNumber := types.CommitNumber(i)

		createLegacyRegressions(ctx, 1, commitNumber, alertID, legacyStore)
	}

	// Now let's migrate some of them in a batch of 5.
	migrationCount := 5
	err := migrator.migrateRegressions(ctx, migrationCount)
	assert.Nil(t, err)

	regressionsMap, err := newStore.Range(ctx, 1, 10)
	assert.Nil(t, err)
	assert.NotNil(t, regressionsMap)
	assert.Equal(t, migrationCount, len(regressionsMap))

	legacyRegressionsMap, _ := legacyStore.Range(ctx, 1, 10)
	assert.Equal(t, 10, len(legacyRegressionsMap))
	for commitNumber := range regressionsMap {
		newRegression := regressionsMap[commitNumber].ByAlertID[alertID]
		oldRegression := legacyRegressionsMap[commitNumber].ByAlertID[alertID]
		assertRegressions(t, oldRegression, newRegression)
		assert.Equal(t, alertID, strconv.Itoa(int(newRegression.AlertId)))
		assert.Equal(t, commitNumber, newRegression.CommitNumber)
		assert.NotEmpty(t, newRegression.Id)
	}

	// Ensure that there are 5 regressions remaining.
	remainingLegacyRegressions, _ := legacyStore.GetRegressionsToMigrate(ctx, 10)
	assert.Equal(t, 5, len(remainingLegacyRegressions))
}

func TestMigrate_MultipleBatches_Success(t *testing.T) {
	ctx, migrator, legacyStore, newStore := setup(t)
	alertID := "123"
	totalCount := 10
	// Create 10 regressions for different commits
	for i := 1; i <= totalCount; i++ {
		commitNumber := types.CommitNumber(i)

		createLegacyRegressions(ctx, 1, commitNumber, alertID, legacyStore)
	}

	// Now let's migrate some of them in a batch of 7.
	migrationCount := 7
	err := migrator.migrateRegressions(ctx, migrationCount)
	assert.Nil(t, err)

	regressionsMap, err := newStore.Range(ctx, 1, 10)
	assert.Nil(t, err)
	assert.NotNil(t, regressionsMap)
	assert.Equal(t, migrationCount, len(regressionsMap))

	legacyRegressionsMap, _ := legacyStore.Range(ctx, 1, 10)
	assert.Equal(t, 10, len(legacyRegressionsMap))
	for commitNumber := range regressionsMap {
		newRegression := regressionsMap[commitNumber].ByAlertID[alertID]
		oldRegression := legacyRegressionsMap[commitNumber].ByAlertID[alertID]
		assertRegressions(t, oldRegression, newRegression)
		assert.Equal(t, alertID, strconv.Itoa(int(newRegression.AlertId)))
		assert.Equal(t, commitNumber, newRegression.CommitNumber)
		assert.NotEmpty(t, newRegression.Id)
	}

	// Ensure that the no of remaining regressions is correct.
	remainingLegacyRegressions, _ := legacyStore.GetRegressionsToMigrate(ctx, totalCount)
	assert.Equal(t, totalCount-migrationCount, len(remainingLegacyRegressions))

	// Now run another batch.
	err = migrator.migrateRegressions(ctx, migrationCount)
	assert.Nil(t, err)

	// Ensure that the no regressions are remaining.
	remainingLegacyRegressions, _ = legacyStore.GetRegressionsToMigrate(ctx, totalCount)
	assert.Equal(t, 0, len(remainingLegacyRegressions))
}

func Test_Migrate_Prev_Migrated_Regression(t *testing.T) {
	ctx, migrator, legacyStore, newStore := setup(t)
	alertID := "123"
	commitNumber := types.CommitNumber(222)
	createLegacyRegressions(ctx, 1, commitNumber, alertID, legacyStore)

	// Lets migrate this regression first.
	err := migrator.migrateRegressions(ctx, 1)
	assert.Nil(t, err)

	// Get the regression from the new store.
	regressionsMap, _ := newStore.Range(ctx, commitNumber, commitNumber)
	regressionsForCommit := regressionsMap[commitNumber]
	newRegression := regressionsForCommit.ByAlertID[alertID]

	// Now update the regression in the legacy store. This should mark the
	// legacy regressions as available to migrate.
	_ = legacyStore.TriageHigh(ctx, commitNumber, alertID, regression.TriageStatus{
		Status:  regression.Positive,
		Message: "Updating the legacy regression.",
	})
	regressionsToMigrate, err := legacyStore.GetRegressionsToMigrate(ctx, 1)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(regressionsToMigrate))
	assert.NotEmpty(t, regressionsToMigrate[0].Id)
	// Ensure that the legacy regression has the same id as the newly created one.
	assert.Equal(t, newRegression.Id, regressionsToMigrate[0].Id)

	// Run migration again.
	err = migrator.migrateRegressions(ctx, 1)
	assert.Nil(t, err)
	// Get the regression from the new store.
	regressionsMap, _ = newStore.Range(ctx, commitNumber, commitNumber)
	regressionsForCommit = regressionsMap[commitNumber]
	newRegression = regressionsForCommit.ByAlertID[alertID]
	assert.NotNil(t, newRegression.HighStatus)
}

func Test_Migrate_Mixed_Regression(t *testing.T) {
	ctx, migrator, legacyStore, newStore := setup(t)
	alertID := "123"
	commitNumber := types.CommitNumber(222)
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

	// Set both high and low values to create a mixed regression
	_, _, _ = legacyStore.SetHigh(ctx, commitNumber, alertID, df, clusterSummary)
	_, _, _ = legacyStore.SetLow(ctx, commitNumber, alertID, df, clusterSummary)

	err := migrator.migrateRegressions(ctx, 1)
	assert.Nil(t, err)

	// Get the regression from the new store.
	regressionsMap, _ := newStore.Range(ctx, commitNumber, commitNumber)
	regressionsForCommit := regressionsMap[commitNumber]
	newRegression := regressionsForCommit.ByAlertID[alertID]
	assert.NotNil(t, newRegression.High)
	assert.NotNil(t, newRegression.Low)
}
