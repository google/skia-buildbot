package migration

import (
	"context"
	"time"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sql/pool"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/alerts/sqlalertstore"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/regression/sqlregression2store"
	"go.skia.org/infra/perf/go/regression/sqlregressionstore"
)

// RegressionMigrator provides a struct to migrate regression data
// from regressions to regressions2 table.
type RegressionMigrator struct {
	db          pool.Pool
	legacyStore *sqlregressionstore.SQLRegressionStore
	newStore    *sqlregression2store.SQLRegression2Store
}

// New returns a new instance of RegressionMigrator.
func New(ctx context.Context, db pool.Pool, instanceConfig *config.InstanceConfig) (*RegressionMigrator, error) {
	legacyStore, err := sqlregressionstore.New(db)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create a new legacy store.")
	}
	alertStore, err := sqlalertstore.New(db)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create a new alerts store.")
	}
	alertConfigProvider, err := alerts.NewConfigProvider(ctx, alertStore, 300)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create a new alerts provider.")
	}
	newStore, err := sqlregression2store.New(db, alertConfigProvider, instanceConfig)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create a new regression2 store.")
	}
	return &RegressionMigrator{
		db:          db,
		legacyStore: legacyStore,
		newStore:    newStore,
	}, nil
}

// RunPeriodicMigration runs a goroutine that runs the migration with the provided batch size
// with a frequency specified by iterationPeriod.
func (m *RegressionMigrator) RunPeriodicMigration(iterationPeriod time.Duration, batchSize int) {
	go func() {
		for range time.Tick(iterationPeriod) {
			m.RunOneMigration(batchSize)
		}
	}()
}

// The helper function for RunPeriodicMigration to run a single iteration, with the proper handling
// on timeout.
func (m *RegressionMigrator) RunOneMigration(batchSize int) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	sklog.Infof("Running regression data migration cycle.")
	if err := m.migrateRegressions(ctx, batchSize); err != nil {
		sklog.Errorf("Failed to migrate regressions: %s", err)
	}
}

// migrateRegressions runs the migration of regressions with the provided batch size.
func (m *RegressionMigrator) migrateRegressions(ctx context.Context, batchSize int) error {
	// Get the regressions available to migrate.
	sourceRegressions, err := m.legacyStore.GetRegressionsToMigrate(ctx, batchSize)
	if err != nil {
		return err
	}

	sklog.Infof("Retrieved %d regressions to migrate.", len(sourceRegressions))
	if len(sourceRegressions) > 0 {
		// The following steps need to be done in a transaction so that we do not end up with
		// duplicate data in regressions2 in case of partial failure.
		// 1. Write the regression object into the regression2 store.
		// 2. Mark the relevant row in the regression table as migrated.
		//
		// We can potentially have a single transaction for the entire block, but keeping it
		// granular so that we do not need to process the entire batch again in case of failure
		// on one regression.
		for _, regression := range sourceRegressions {
			// All these legacy regression objects do not have the new fields populated other than AlertId and CommitNumber.
			// So we will need to populate those first before we can write the data into the new table.
			tx, err := m.db.Begin(ctx)
			if err != nil {
				return err
			}
			regressionId, err := m.newStore.WriteRegression(ctx, regression, tx)
			if err != nil {
				if err := tx.Rollback(ctx); err != nil {
					sklog.Errorf("Failed on rollback: %s", err)
				}
				return err
			}
			err = m.legacyStore.MarkMigrated(ctx, regressionId, regression.CommitNumber, regression.AlertId, tx)
			if err != nil {
				if err := tx.Rollback(ctx); err != nil {
					sklog.Errorf("Failed on rollback: %s", err)
				}
				return err
			}
			err = tx.Commit(ctx)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
