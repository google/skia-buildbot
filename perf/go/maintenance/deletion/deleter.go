package deletion

import (
	"context"
	"time"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sql/pool"
	"go.skia.org/infra/perf/go/regression"
	"go.skia.org/infra/perf/go/regression/sqlregressionstore"
	"go.skia.org/infra/perf/go/shortcut"
	"go.skia.org/infra/perf/go/shortcut/sqlshortcutstore"
	"go.skia.org/infra/perf/go/types"
)

var ttl = -18 // in months

type Deleter struct {
	db              pool.Pool
	regressionStore regression.Store
	shortcutStore   shortcut.Store
}

func New(db pool.Pool) (*Deleter, error) {
	regressionStore, err := sqlregressionstore.New(db)
	if err != nil {
		return nil, skerr.Wrapf(err, "could not create regressions store")
	}
	shortcutStore, err := sqlshortcutstore.New(db)
	if err != nil {
		return nil, skerr.Wrapf(err, "could not create shortcuts store")
	}
	return &Deleter{
		db:              db,
		regressionStore: regressionStore,
		shortcutStore:   shortcutStore,
	}, nil
}

// RunPeriodicDeletion runs a goroutine that deletes shortcuts and regressions
// with the provided batch size (based on the number of shortcuts)
// with a frequency specified by iterationPeriod.
func (d *Deleter) RunPeriodicDeletion(iterationPeriod time.Duration, shortcutBatchSize int) {
	go func() {
		for range time.Tick(iterationPeriod) {
			d.DeleteOneBatch(shortcutBatchSize)
		}
	}()
}

// DeleteOneBatch deletes a batch of regressions from the regressions table
// and a batch of shortcuts from the shortcuts table.
func (d *Deleter) DeleteOneBatch(shortcutBatchSize int) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	sklog.Infof("delete one batch of shortcuts and regressions")
	commitNumbers, shortcuts, err := d.getBatch(ctx, shortcutBatchSize)
	if err != nil {
		sklog.Errorf("could not get batch: %s", err)
	}
	if err := d.deleteBatch(ctx, commitNumbers, shortcuts); err != nil {
		sklog.Errorf("could delete batch: %s", err)
	}
}

// getOutdatedKeys returns the keys for the regression table and shortcuts table
// that have outlived the ttl.
func getOutdatedKeys(regressionsByCommit map[types.CommitNumber]*regression.AllRegressionsForCommit) ([]types.CommitNumber, []string) {
	commits := []types.CommitNumber{}
	shortcuts := []string{}
	for commitNumber, regressions := range regressionsByCommit {
		currLength := len(shortcuts)
		for _, r := range regressions.ByAlertID {
			low, high := r.Low, r.High
			if low != nil && int64(low.StepPoint.Timestamp) < time.Now().AddDate(0, ttl, 0).Unix() {
				shortcuts = append(shortcuts, low.Shortcut)
			}
			if high != nil && int64(high.StepPoint.Timestamp) < time.Now().AddDate(0, ttl, 0).Unix() {
				shortcuts = append(shortcuts, high.Shortcut)
			}
		}
		if len(shortcuts) > currLength {
			commits = append(commits, commitNumber)
		}
	}

	return commits, shortcuts
}

func (d *Deleter) getBatch(ctx context.Context, shortcutBatchSize int) ([]types.CommitNumber, []string, error) {
	oldestCommitNumber, err := d.regressionStore.GetOldestCommit(ctx)
	if err != nil {
		return nil, nil, skerr.Wrapf(err, "could not get oldest commit from Regressions table")
	}

	begin, end := oldestCommitNumber.Add(0), oldestCommitNumber.Add(int32(shortcutBatchSize-1))
	// commit numbers and shortcuts will not be the same length. The same commit number can be
	// affiliated with multiple shortcuts
	commitNumbers := []types.CommitNumber{}
	shortcuts := []string{}

	// Starting from the oldest commit, collect shortcuts and regressions in batch that qualify for
	// deletion until we collect at least shortcutBatchSize number of shortcuts. Any data that is
	// older than ttl months (18 mo) is eligible for deletion as specified by stakeholders.
	for len(shortcuts) < shortcutBatchSize {
		commits, err := d.regressionStore.Range(ctx, begin, end)
		if err != nil {
			return nil, nil, skerr.Wrapf(err, "could not get commits between range (%d, %d)", begin, end)
		}

		c, s := getOutdatedKeys(commits)
		// implies there is no more data to delete
		if len(c) == 0 && len(s) == 0 {
			sklog.Infof("All eligible shortcuts and regressions have been deleted.")
			return commitNumbers, shortcuts, nil
		}
		commitNumbers = append(commitNumbers, c...)
		shortcuts = append(shortcuts, s...)

		begin = begin.Add(int32(shortcutBatchSize))
		end = end.Add(int32(shortcutBatchSize))
	}

	return commitNumbers, shortcuts, nil
}

func (d *Deleter) deleteBatch(ctx context.Context, commitNumbers []types.CommitNumber, shortcuts []string) error {
	tx, err := d.db.Begin(ctx)
	if err != nil {
		return skerr.Wrap(err)
	}
	for _, c := range commitNumbers {
		sklog.Infof("Removing regression %d.", c)
		if err := d.regressionStore.DeleteByCommit(ctx, c, tx); err != nil {
			sklog.Errorf("could not delete regression with commit number %v: %s", c, err)
			if err := tx.Rollback(ctx); err != nil {
				sklog.Errorf("Rollback failed: %s", err)
			}
			return skerr.Wrapf(err, "could not delete regression with commit number %v", c)
		}
	}
	for _, s := range shortcuts {
		sklog.Infof("Removing shortcut %s.", s)
		if err := d.shortcutStore.DeleteShortcut(ctx, s, tx); err != nil {
			sklog.Errorf("Could not delete shortcut %s: %s", s, err)
			if err := tx.Rollback(ctx); err != nil {
				sklog.Errorf("Rollback failed: %s", err)
			}
			return skerr.Wrapf(err, "could not delete shortcut %s", s)
		}
	}
	return tx.Commit(ctx)
}
