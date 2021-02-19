// Package sqlregressionstore implements the regression.Store interface on an
// SQL database backend.
//
// To see the schema of the database used, see perf/sql/migrations.
package sqlregressionstore

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v4/pgxpool"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/regression"
	"go.skia.org/infra/perf/go/types"
)

// statement is an SQL statement identifier.
type statement int

const (
	// The identifiers for all the SQL statements used.
	write statement = iota
	read
	readRange
)

// statementsByDialect holds all the raw SQL statemens used per Dialect of SQL.
var statements = map[statement]string{
	write: `
		UPSERT INTO
			Regressions (commit_number, alert_id, regression)
		VALUES
			($1, $2, $3)
		`,
	read: `
		SELECT
			regression
		FROM
			Regressions
		WHERE
			commit_number=$1 AND
			alert_id=$2`,
	readRange: `
		SELECT
			commit_number, alert_id, regression
		FROM
			Regressions
		WHERE
			commit_number >= $1
			AND commit_number <= $2
		`,
}

// SQLRegressionStore implements the regression.Store interface.
type SQLRegressionStore struct {
	// db is the underlying database.
	db                         *pgxpool.Pool
	regressionFoundCounterLow  metrics2.Counter
	regressionFoundCounterHigh metrics2.Counter
}

// New returns a new *SQLRegressionStore.
//
// We presume all migrations have been run against db before this function is
// called.
func New(db *pgxpool.Pool) (*SQLRegressionStore, error) {
	return &SQLRegressionStore{
		db:                         db,
		regressionFoundCounterLow:  metrics2.GetCounter("perf_regression_store_found", map[string]string{"direction": "low"}),
		regressionFoundCounterHigh: metrics2.GetCounter("perf_regression_store_found", map[string]string{"direction": "high"}),
	}, nil
}

// Range implements the regression.Store interface.
func (s *SQLRegressionStore) Range(ctx context.Context, begin, end types.CommitNumber) (map[types.CommitNumber]*regression.AllRegressionsForCommit, error) {
	ret := map[types.CommitNumber]*regression.AllRegressionsForCommit{}
	rows, err := s.db.Query(ctx, statements[readRange], begin, end)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to read regressions in range: %d %d", begin, end)
	}
	for rows.Next() {
		var commitID types.CommitNumber
		var alertID int64
		var jsonRegression string
		if err := rows.Scan(&commitID, &alertID, &jsonRegression); err != nil {
			return nil, skerr.Wrapf(err, "Failed to read single regression in range: %d %d", begin, end)
		}
		var r regression.Regression
		if err := json.Unmarshal([]byte(jsonRegression), &r); err != nil {
			return nil, skerr.Wrapf(err, "Failed to decode a single regression in range: %d %d", begin, end)
		}
		allForCommit, ok := ret[commitID]
		if !ok {
			allForCommit = regression.New()
		}
		alertIDString := alerts.IDToString(alertID)
		allForCommit.ByAlertID[alertIDString] = &r
		ret[commitID] = allForCommit
	}
	return ret, nil
}

// SetHigh implements the regression.Store interface.
func (s *SQLRegressionStore) SetHigh(ctx context.Context, commitNumber types.CommitNumber, alertID string, df *dataframe.FrameResponse, high *clustering2.ClusterSummary) (bool, error) {
	ret := false
	err := s.readModifyWrite(ctx, commitNumber, alertID, false /* mustExist*/, func(r *regression.Regression) {
		if r.Frame == nil {
			r.Frame = df
			ret = true
		}
		r.High = high
		if r.HighStatus.Status == regression.None {
			r.HighStatus.Status = regression.Untriaged
		}
	})
	s.regressionFoundCounterHigh.Inc(1)
	return ret, err

}

// SetLow implements the regression.Store interface.
func (s *SQLRegressionStore) SetLow(ctx context.Context, commitNumber types.CommitNumber, alertID string, df *dataframe.FrameResponse, low *clustering2.ClusterSummary) (bool, error) {
	ret := false
	err := s.readModifyWrite(ctx, commitNumber, alertID, false /* mustExist*/, func(r *regression.Regression) {
		if r.Frame == nil {
			r.Frame = df
			ret = true
		}
		r.Low = low
		if r.LowStatus.Status == regression.None {
			r.LowStatus.Status = regression.Untriaged
		}
	})
	s.regressionFoundCounterLow.Inc(1)
	return ret, err
}

// TriageLow implements the regression.Store interface.
func (s *SQLRegressionStore) TriageLow(ctx context.Context, commitNumber types.CommitNumber, alertID string, tr regression.TriageStatus) error {
	return s.readModifyWrite(ctx, commitNumber, alertID, true /* mustExist*/, func(r *regression.Regression) {
		r.LowStatus = tr
	})
}

// TriageHigh implements the regression.Store interface.
func (s *SQLRegressionStore) TriageHigh(ctx context.Context, commitNumber types.CommitNumber, alertID string, tr regression.TriageStatus) error {
	return s.readModifyWrite(ctx, commitNumber, alertID, true /* mustExist*/, func(r *regression.Regression) {
		r.HighStatus = tr
	})
}

// Write implements the regression.Store interface.
func (s *SQLRegressionStore) Write(ctx context.Context, regressions map[types.CommitNumber]*regression.AllRegressionsForCommit) error {
	for commitNumber, allRegressionsForCommit := range regressions {
		for alertIDString, reg := range allRegressionsForCommit.ByAlertID {
			if err := s.write(ctx, commitNumber, alertIDString, reg); err != nil {
				return err
			}
		}
	}
	return nil
}

// write the given Regression into the database at the given commitNumber and
// alert id.
func (s *SQLRegressionStore) write(ctx context.Context, commitNumber types.CommitNumber, alertIDString string, r *regression.Regression) error {
	if alertIDString == alerts.BadAlertIDAsAsString {
		return skerr.Fmt("Failed to convert alertIDString %q to an int.", alertIDString)
	}
	alertID := alerts.IDAsStringToInt(alertIDString)

	b, err := json.Marshal(r)
	if err != nil {
		return skerr.Wrapf(err, "Failed to serialize regression for alertID: %d  commitNumber=%d", alertID, commitNumber)
	}
	if _, err := s.db.Exec(ctx, statements[write], commitNumber, alertID, string(b)); err != nil {
		return skerr.Wrapf(err, "Failed to write regression for alertID: %d  commitNumber=%d", alertID, commitNumber)
	}
	return nil
}

// read the Regression from the database at the given commitNumber and alert id.
// This func is only used in tests.
func (s *SQLRegressionStore) read(ctx context.Context, commitNumber types.CommitNumber, alertIDString string) (*regression.Regression, error) {
	if alertIDString == alerts.BadAlertIDAsAsString {
		return nil, skerr.Fmt("Failed to convert alertIDString %q to an int.", alertIDString)
	}
	alertID := alerts.IDAsStringToInt(alertIDString)
	var jsonString string
	if err := s.db.QueryRow(ctx, statements[read], commitNumber, alertID).Scan(&jsonString); err != nil {
		return nil, skerr.Wrapf(err, "Failed to read regression for alertID: %d commitNumber=%d", alertID, commitNumber)
	}
	r := regression.NewRegression()
	if err := json.Unmarshal([]byte(jsonString), r); err != nil {
		return nil, skerr.Wrapf(err, "Failed to deserialize regression for alertID: %d commitNumber=%d", alertID, commitNumber)
	}
	return r, nil
}

// readModifyWrite reads the Regression at the given commitNumber and alert id
// and then calls the given callback, giving the caller a chance to modify the
// struct, before writing it back to the database.
//
// If mustExist is true then the read must be successful, otherwise a new
// default Regression will be used and stored back to the database after the
// callback is called.
func (s *SQLRegressionStore) readModifyWrite(ctx context.Context, commitNumber types.CommitNumber, alertIDString string, mustExist bool, cb func(r *regression.Regression)) error {
	if alertIDString == alerts.BadAlertIDAsAsString {
		return skerr.Fmt("Failed to convert alertIDString %q to an int.", alertIDString)
	}
	alertID := alerts.IDAsStringToInt(alertIDString)

	// Do everything in a transaction so we don't have any lost updates.
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return skerr.Wrapf(err, "Can't start transaction")
	}

	r := regression.NewRegression()

	// Read the regression from the database. If any part of that fails then
	// just use the default regression we've already constructed.
	var jsonString string
	if err := tx.QueryRow(ctx, statements[read], commitNumber, alertID).Scan(&jsonString); err == nil {
		if err := json.Unmarshal([]byte(jsonString), r); err != nil {
			sklog.Warningf("Failed to deserialize the JSON Regression: %s", err)
		}
	} else {
		if mustExist {
			if err := tx.Rollback(ctx); err != nil {
				sklog.Errorf("Failed on rollback: %s", err)
			}
			return skerr.Wrapf(err, "Regression doesn't exist.")
		}
	}

	cb(r)

	b, err := json.Marshal(r)
	if err != nil {
		if err := tx.Rollback(ctx); err != nil {
			sklog.Errorf("Failed on rollback: %s", err)
		}
		return skerr.Wrapf(err, "Failed to serialize regression for alertID: %d  commitNumber=%d", alertID, commitNumber)
	}
	if _, err := tx.Exec(ctx, statements[write], commitNumber, alertID, string(b)); err != nil {
		if err := tx.Rollback(ctx); err != nil {
			sklog.Errorf("Failed on rollback: %s", err)
		}
		return skerr.Wrapf(err, "Failed to write regression for alertID: %d  commitNumber=%d", alertID, commitNumber)
	}

	return tx.Commit(ctx)
}

// Confirm that SQLRegressionStore implements regression.Store.
var _ regression.Store = (*SQLRegressionStore)(nil)
