package deletion

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/regression"
	"go.skia.org/infra/perf/go/regression/sqlregressionstore"
	"go.skia.org/infra/perf/go/shortcut"
	"go.skia.org/infra/perf/go/shortcut/sqlshortcutstore"
	"go.skia.org/infra/perf/go/sql/sqltest"
	"go.skia.org/infra/perf/go/types"
)

func setup(t *testing.T) (context.Context, *Deleter, *sqlregressionstore.SQLRegressionStore, *sqlshortcutstore.SQLShortcutStore) {
	ctx := context.Background()
	db := sqltest.NewCockroachDBForTests(t, "delStore")
	deleter, _ := New(db)
	regressionStore, _ := sqlregressionstore.New(db)
	shortcutStore, _ := sqlshortcutstore.New(db)

	return ctx, deleter, regressionStore, shortcutStore
}

func writeShortcuts(ctx context.Context, num int, store *sqlshortcutstore.SQLShortcutStore) []string {
	shortcuts := make([]string, num)
	for i := 0; i < num; i++ {
		s := &shortcut.Shortcut{
			Keys: []string{fmt.Sprintf("arch=x86,config=8888,%d", i)},
		}
		id, _ := store.InsertShortcut(ctx, s)
		shortcuts[i] = id
	}
	return shortcuts
}

func writeRegressions(ctx context.Context, begin, end int, ts int64, shortcuts []string, store *sqlregressionstore.SQLRegressionStore) error {
	regressions := map[types.CommitNumber]*regression.AllRegressionsForCommit{}
	for i := begin; i < end; i++ {
		regressions[types.CommitNumber(i)] = &regression.AllRegressionsForCommit{
			ByAlertID: map[string]*regression.Regression{
				"fake-alert-id": {
					High: &clustering2.ClusterSummary{
						Shortcut: shortcuts[i],
						StepPoint: &dataframe.ColumnHeader{
							Offset:    types.CommitNumber(i),
							Timestamp: dataframe.TimestampSeconds(ts),
						},
					},
				},
			},
		}
	}
	return store.Write(ctx, regressions)
}

func TestDeleteOneBatch_HappyPath(t *testing.T) {
	const batchSize, numShortcuts = 4, 10
	var ts = time.Now().AddDate(0, ttl, -1).Unix()

	ctx, deleter, regressionStore, shortcutStore := setup(t)
	shortcuts := writeShortcuts(ctx, numShortcuts, shortcutStore)
	err := writeRegressions(ctx, 0, numShortcuts, ts, shortcuts, regressionStore)
	require.NoError(t, err)

	batchCommits, batchShortcuts, err := deleter.getBatch(ctx, batchSize)
	require.NoError(t, err)
	assert.Len(t, batchCommits, batchSize)
	assert.Len(t, batchShortcuts, batchSize)
	assert.Equal(t, shortcuts[:batchSize], batchShortcuts)

	err = deleter.deleteBatch(ctx, batchCommits, batchShortcuts)
	require.NoError(t, err)

	// verify remaining regressions are equal to the number started with - number deleted
	commits, err := deleter.regressionStore.Range(ctx, types.CommitNumber(0), types.CommitNumber(numShortcuts))
	require.NoError(t, err)
	assert.Len(t, commits, numShortcuts-batchSize)
}

func TestDeleteOneBatch_GivenNewAndOldRegressions_OnlyOldRegressionsDeleted(t *testing.T) {
	// This unit test creates (batchSize-1) regressions that are eligible for deletion and
	// tries to delete (batchSize) of them. The expected outcome is (batchSize-1) are deleted.
	const batchSize, numShortcuts = 6, 20

	ctx, deleter, regressionStore, shortcutStore := setup(t)
	shortcuts := writeShortcuts(ctx, numShortcuts, shortcutStore)

	// write a batch of regressions that will be deleted
	ts := time.Now().AddDate(0, ttl, -1).Unix()
	err := writeRegressions(ctx, 0, batchSize-1, ts, shortcuts, regressionStore)
	require.NoError(t, err)
	// write a batch of regressions that will not be deleted
	ts = time.Now().Unix()
	err = writeRegressions(ctx, batchSize-1, numShortcuts, ts, shortcuts, regressionStore)
	require.NoError(t, err)

	batchCommits, batchShortcuts, err := deleter.getBatch(ctx, batchSize)
	require.NoError(t, err)
	assert.Len(t, batchCommits, batchSize-1)
	assert.Len(t, batchShortcuts, batchSize-1)
	assert.Equal(t, shortcuts[0:batchSize-1], batchShortcuts)
	// getBatch uses Range to get commits, which returns a map and the
	// keys (commitNumbers) will be out of order
	for _, commitNumber := range batchCommits {
		assert.Less(t, int(commitNumber), batchSize-1)
	}

	err = deleter.deleteBatch(ctx, batchCommits, batchShortcuts)
	require.NoError(t, err)

	// verify remaining regressions are equal to the number started with - number deleted
	commits, err := deleter.regressionStore.Range(ctx, types.CommitNumber(0), types.CommitNumber(numShortcuts))
	require.NoError(t, err)
	assert.Len(t, commits, numShortcuts-(batchSize-1))
	for commitNumber := range commits {
		assert.GreaterOrEqual(t, int(commitNumber), batchSize-1) // range map returns keys out of order
	}
}
