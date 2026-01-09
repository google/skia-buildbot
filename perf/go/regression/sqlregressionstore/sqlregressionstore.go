// Package sqlregressionstore implements the regression.Store interface on an
// SQL database backend.
package sqlregressionstore

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v4"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sql/pool"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/regression"
	pb "go.skia.org/infra/perf/go/subscription/proto/v1"
	"go.skia.org/infra/perf/go/types"
	"go.skia.org/infra/perf/go/ui/frame"
)

// statement is an SQL statement identifier.
type statement int

const (
	// The identifiers for all the SQL statements used.
	write statement = iota
	read
	readOldest
	readRange
	batchReadMigration
	markMigrated
	deleteByCommit
	updateRegression
)

// statementsByDialect holds all the raw SQL statemens used per Dialect of SQL.
var statements = map[statement]string{
	write: `
		INSERT INTO
			Regressions (commit_number, alert_id, regression, migrated)
		VALUES
			($1, $2, $3, false)
		ON CONFLICT (commit_number, alert_id) DO UPDATE
		SET commit_number=EXCLUDED.commit_number, alert_id=EXCLUDED.alert_id,
		regression=EXCLUDED.regression, migrated=EXCLUDED.migrated
		`,
	updateRegression: `
		UPDATE Regressions
			SET regression=$3, migrated=false
		WHERE
			commit_number=$1 AND alert_id=$2
		`,
	read: `
		SELECT
			regression
		FROM
			Regressions
		WHERE
			commit_number=$1 AND
			alert_id=$2`,
	readOldest: `
		SELECT
			commit_number
		FROM
			Regressions
		ORDER BY
			commit_number ASC
		LIMIT 1
		`,
	readRange: `
		SELECT
			commit_number, alert_id, regression
		FROM
			Regressions
		WHERE
			commit_number >= $1
			AND commit_number <= $2
		`,
	batchReadMigration: `
		SELECT
			commit_number, alert_id, regression, regression_id
		FROM
			Regressions
		WHERE
			migrated is NULL OR migrated=false
		LIMIT $1
		`,
	markMigrated: `
		UPDATE
			Regressions
		SET
			migrated=true, regression_id=$1
		WHERE
			commit_number=$2 AND alert_id=$3
		`,
	deleteByCommit: `
		DELETE
		FROM
			Regressions
		WHERE
			commit_number=$1
		`,
}

// SQLRegressionStore implements the regression.Store interface.
type SQLRegressionStore struct {
	// db is the underlying database.
	db                         pool.Pool
	statements                 map[statement]string
	regressionFoundCounterLow  metrics2.Counter
	regressionFoundCounterHigh metrics2.Counter
}

// New returns a new *SQLRegressionStore.
func New(db pool.Pool) (*SQLRegressionStore, error) {
	return &SQLRegressionStore{
		db:                         db,
		statements:                 statements,
		regressionFoundCounterLow:  metrics2.GetCounter("perf_regression_store_found", map[string]string{"direction": "low"}),
		regressionFoundCounterHigh: metrics2.GetCounter("perf_regression_store_found", map[string]string{"direction": "high"}),
	}, nil
}

// Unimplemented: This function is implemented by regression2 store
func (s *SQLRegressionStore) GetRegressionsBySubName(ctx context.Context, sub_name string, limit int, offset int) ([]*regression.Regression, error) {
	return nil, nil
}

// Unimplemented: This function is implemented by regression2 store
func (s *SQLRegressionStore) RangeFiltered(ctx context.Context, begin, end types.CommitNumber, traceNames []string) ([]*regression.Regression, error) {
	return nil, skerr.Fmt("RangeFiltered is not implemented in old version of regression store.")
}

// Range implements the regression.Store interface.
func (s *SQLRegressionStore) Range(ctx context.Context, begin, end types.CommitNumber) (map[types.CommitNumber]*regression.AllRegressionsForCommit, error) {
	ret := map[types.CommitNumber]*regression.AllRegressionsForCommit{}
	rows, err := s.db.Query(ctx, s.statements[readRange], begin, end)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to read regressions in range: %d %d", begin, end)
	}
	defer rows.Close()
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
func (s *SQLRegressionStore) SetHigh(ctx context.Context, commitNumber types.CommitNumber, alertID string, df *frame.FrameResponse, high *clustering2.ClusterSummary) (bool, string, error) {
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
	return ret, "", err

}

// SetLow implements the regression.Store interface.
func (s *SQLRegressionStore) SetLow(ctx context.Context, commitNumber types.CommitNumber, alertID string, df *frame.FrameResponse, low *clustering2.ClusterSummary) (bool, string, error) {
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
	return ret, "", err
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

// GetRegression returns the regression info at the given commit for specific alert.
func (s *SQLRegressionStore) GetRegression(ctx context.Context, commitNumber types.CommitNumber, alertID string) (*regression.Regression, error) {
	return s.read(ctx, commitNumber, alertID)
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

	var buff bytes.Buffer
	err := json.NewEncoder(&buff).Encode(r)
	if err != nil {
		return skerr.Wrapf(err, "Failed to serialize regression for alertID: %d  commitNumber=%d", alertID, commitNumber)
	}
	if _, err := s.db.Exec(ctx, s.statements[write], commitNumber, alertID, buff.String()); err != nil {
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
	if err := s.db.QueryRow(ctx, s.statements[read], commitNumber, alertID).Scan(&jsonString); err != nil {
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
	r.Id = ""

	// Read the regression from the database. If any part of that fails then
	// just use the default regression we've already constructed.
	var jsonString string
	if err := tx.QueryRow(ctx, s.statements[read], commitNumber, alertID).Scan(&jsonString); err == nil {
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

	var buff bytes.Buffer
	err = json.NewEncoder(&buff).Encode(r)
	if err != nil {
		if err := tx.Rollback(ctx); err != nil {
			sklog.Errorf("Failed on rollback: %s", err)
		}
		return skerr.Wrapf(err, "Failed to serialize regression for alertID: %d  commitNumber=%d", alertID, commitNumber)
	}
	writeStatement := s.statements[write]
	if mustExist {
		writeStatement = s.statements[updateRegression]
	}
	if _, err := tx.Exec(ctx, writeStatement, commitNumber, alertID, buff.String()); err != nil {
		if err := tx.Rollback(ctx); err != nil {
			sklog.Errorf("Failed on rollback: %s", err)
		}
		return skerr.Wrapf(err, "Failed to write regression for alertID: %d  commitNumber=%d", alertID, commitNumber)
	}

	return tx.Commit(ctx)
}

// GetRegressionsToMigrate returns a set of regressions which are available to be migrated.
func (s *SQLRegressionStore) GetRegressionsToMigrate(ctx context.Context, batchSize int) ([]*regression.Regression, error) {
	regressions := []*regression.Regression{}
	var rows pgx.Rows
	var err error
	if rows, err = s.db.Query(ctx, s.statements[batchReadMigration], batchSize); err != nil {
		return nil, skerr.Wrapf(err, "Failed to read regressions for migration")
	}
	defer rows.Close()
	for rows.Next() {
		var commitID types.CommitNumber
		var alertID int64
		var jsonRegression string
		var regressionId *string
		if err := rows.Scan(&commitID, &alertID, &jsonRegression, &regressionId); err != nil {
			return nil, skerr.Wrapf(err, "Failed to read single regression")
		}
		var r regression.Regression
		if err := json.Unmarshal([]byte(jsonRegression), &r); err != nil {
			return nil, skerr.Wrapf(err, "Failed to decode a single regression for commit %d, alert %d", commitID, alertID)
		}
		r.AlertId = alertID
		r.CommitNumber = commitID
		if regressionId != nil {
			r.Id = *regressionId
		}

		regressions = append(regressions, &r)
	}

	return regressions, nil
}

// MarkMigrated marks a specific row in the regressions table as migrated.
func (s *SQLRegressionStore) MarkMigrated(ctx context.Context, regressionId string, commitNumber types.CommitNumber, alertID int64, tx pgx.Tx) error {
	if _, err := tx.Exec(ctx, s.statements[markMigrated], regressionId, commitNumber, alertID); err != nil {
		return skerr.Wrapf(err, "Failed to mark regression migrated for alertID: %d  commitNumber=%d", alertID, commitNumber)
	}

	return nil
}

// Not implemented as old regression schema does not have id.
func (s *SQLRegressionStore) GetByIDs(ctx context.Context, ids []string) ([]*regression.Regression, error) {
	return nil, skerr.Fmt("GetByIDs are not implemented in old version of regression store.")
}

// Not implemented as old regression schema does not have bug_id.
func (s *SQLRegressionStore) GetIdsByManualTriageBugID(ctx context.Context, bugId int) ([]string, error) {
	return nil, skerr.Fmt("GetIdsByManualTriageBugID are not implemented in old version of regression store.")
}

// Not implemented, old regression will not be developed
func (s *SQLRegressionStore) GetByRevision(ctx context.Context, revision string) ([]*regression.Regression, error) {
	return nil, skerr.Fmt("GetByRev is not implemented in old version of regression store.")
}

// GetOldestCommit implements the regression.Store interface. Gets the oldest commit in the table.
func (s *SQLRegressionStore) GetOldestCommit(ctx context.Context) (*types.CommitNumber, error) {
	var num int
	if err := s.db.QueryRow(ctx, s.statements[readOldest]).Scan(&num); err != nil {
		return nil, skerr.Wrapf(err, "Failed to fetch oldest commit.")
	}
	commitNumber := types.CommitNumber(num)
	return &commitNumber, nil
}

// DeleteByCommit implements the regression.Store interface. Deletes a regression via commit number.
func (s *SQLRegressionStore) DeleteByCommit(ctx context.Context, num types.CommitNumber, tx pgx.Tx) error {
	var err error
	if tx == nil {
		_, err = s.db.Exec(ctx, s.statements[deleteByCommit], num)
	} else {
		_, err = tx.Exec(ctx, s.statements[deleteByCommit], num)
	}

	return err
}

func (s *SQLRegressionStore) SetBugID(ctx context.Context, regressionIDs []string, bugID int) error {
	return skerr.Fmt("SetBugID is not implemented in old version of regression store.")
}

func (s *SQLRegressionStore) GetBugIdsForRegressions(ctx context.Context, regressions []*regression.Regression) ([]*regression.Regression, error) {
	return nil, skerr.Fmt("GetBugIdsForRegressions is not implemented in old version of regression store.")
}

// Confirm that SQLRegressionStore implements regression.Store.
var _ regression.Store = (*SQLRegressionStore)(nil)

// IgnoreAnomalies implements the regression.Store interface.
func (s *SQLRegressionStore) IgnoreAnomalies(ctx context.Context, regressionIDs []string) error {
	return skerr.Fmt("IgnoreAnomalies not implemented for SQLRegressionStore")
}

// ResetAnomalies implements the regression.Store interface.
func (s *SQLRegressionStore) ResetAnomalies(ctx context.Context, regressionIDs []string) error {
	return skerr.Fmt("ResetAnomalies not implemented for SQLRegressionStore")
}

// NudgeAndResetAnomalies implements the regression.Store interface.
func (s *SQLRegressionStore) NudgeAndResetAnomalies(ctx context.Context, regressionIDs []string, commitNumber, prevCommitNumber types.CommitNumber) error {
	return skerr.Fmt("NudgeAndResetAnomalies not implemented for SQLRegressionStore")
}

func (s *SQLRegressionStore) GetSubscriptionsForRegressions(ctx context.Context, regressionIDs []string) ([]string, []int64, []*pb.Subscription, error) {
	return nil, nil, nil, skerr.Fmt("GetSubscriptionsForRegressions not implemented for SQLRegressionStore")
}
