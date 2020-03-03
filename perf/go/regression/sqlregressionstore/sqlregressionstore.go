// Package sqlregressionstore implements the regression.Store interface on an
// SQL database backend.
//
// To see the schema of the database used, see perf/sql/migrations.
package sqlregressionstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/cid"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/regression"
	perfsql "go.skia.org/infra/perf/go/sql"
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

// statements allows looking up raw SQL statements by their statement id.
type statements map[statement]string

// statementsByDialect holds all the raw SQL statemens used per Dialect of SQL.
var statementsByDialect = map[perfsql.Dialect]statements{
	perfsql.SQLiteDialect: {
		write: `
		INSERT OR REPLACE INTO
			Regressions (commit_number, alert_id, regression)
		VALUES
			(?, ?, ?)`,
		read: `
		SELECT
			regression
		FROM
			Regressions
		WHERE
			commit_number=? AND
			alert_id=?`,
		readRange: `
		SELECT
			commit_number, alert_id, regression
		FROM
			Regressions
		WHERE
			commit_number >= ?
			AND commit_number < ?
		`,
	},
	perfsql.CockroachDBDialect: {
		write: `
		UPSERT INTO
			REGRESSIONS (commit_number, alert_id, regression)
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
				AND commit_number < $2
			`,
	},
}

// SQLRegressionStore implements the regression.Store interface.
type SQLRegressionStore struct {
	// db is the underlying database.
	db *sql.DB

	// preparedStatements are all the prepared SQL statements.
	preparedStatements map[statement]*sql.Stmt
}

// New returns a new *SQLRegressionStore.
//
// We presume all migrations have been run against db before this function is
// called.
func New(db *sql.DB, dialect perfsql.Dialect) (*SQLRegressionStore, error) {
	preparedStatements := map[statement]*sql.Stmt{}
	for key, statement := range statementsByDialect[dialect] {
		prepared, err := db.Prepare(statement)
		if err != nil {
			return nil, skerr.Wrapf(err, "Failed to prepare statment %v %q", key, statement)
		}
		preparedStatements[key] = prepared
	}

	return &SQLRegressionStore{
		db:                 db,
		preparedStatements: preparedStatements,
	}, nil
}

// Range implements the regression.Store interface.
func (s *SQLRegressionStore) Range(ctx context.Context, begin, end types.CommitNumber) (map[types.CommitNumber]*regression.AllRegressionsForCommit, error) {
	ret := map[types.CommitNumber]*regression.AllRegressionsForCommit{}
	rows, err := s.preparedStatements[readRange].QueryContext(ctx, begin, end)
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
		alertIDString := fmt.Sprintf("%d", alertID)
		allForCommit.ByAlertID[alertIDString] = &r
	}
	return ret, nil
}

// SetHigh implements the regression.Store interface.
func (s *SQLRegressionStore) SetHigh(ctx context.Context, cid *cid.CommitDetail, alertID string, df *dataframe.FrameResponse, high *clustering2.ClusterSummary) (bool, error) {
	ret := false
	err := s.readModifyWrite(ctx, types.CommitNumber(cid.Offset), alertID, func(r *regression.Regression) {
		if r.Frame == nil {
			r.Frame = df
			ret = true
		}
		r.High = high
		if r.HighStatus.Status == regression.NONE {
			r.HighStatus.Status = regression.UNTRIAGED
		}
	})
	return ret, err

}

// SetLow implements the regression.Store interface.
func (s *SQLRegressionStore) SetLow(ctx context.Context, cid *cid.CommitDetail, alertID string, df *dataframe.FrameResponse, low *clustering2.ClusterSummary) (bool, error) {
	ret := false
	err := s.readModifyWrite(ctx, types.CommitNumber(cid.Offset), alertID, func(r *regression.Regression) {
		if r.Frame == nil {
			r.Frame = df
			ret = true
		}
		r.Low = low
		if r.LowStatus.Status == regression.NONE {
			r.LowStatus.Status = regression.UNTRIAGED
		}
	})
	return ret, err
}

// TriageLow implements the regression.Store interface.
func (s *SQLRegressionStore) TriageLow(ctx context.Context, cid *cid.CommitDetail, alertID string, tr regression.TriageStatus) error {
	return s.readModifyWrite(ctx, types.CommitNumber(cid.Offset), alertID, func(r *regression.Regression) {
		r.LowStatus = tr
	})
}

// TriageHigh implements the regression.Store interface.
func (s *SQLRegressionStore) TriageHigh(ctx context.Context, cid *cid.CommitDetail, alertID string, tr regression.TriageStatus) error {
	return s.readModifyWrite(ctx, types.CommitNumber(cid.Offset), alertID, func(r *regression.Regression) {
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

func (s *SQLRegressionStore) write(ctx context.Context, commitNumber types.CommitNumber, alertIDString string, r *regression.Regression) error {
	alertID, err := strconv.Atoi(alertIDString)
	if err != nil {
		return skerr.Wrapf(err, "Failed to convert alertIDString %q to an int.", alertIDString)
	}
	b, err := json.Marshal(r)
	if err != nil {
		return skerr.Wrapf(err, "Failed to serialize regression for alertID: %d  commitNumber=%d", alertID, commitNumber)
	}
	if _, err := s.preparedStatements[write].ExecContext(ctx, commitNumber, alertID, string(b)); err != nil {
		return skerr.Wrapf(err, "Failed to write regression for alertID: %d  commitNumber=%d", alertID, commitNumber)
	}
	return nil
}

func (s *SQLRegressionStore) read(ctx context.Context, commitNumber types.CommitNumber, alertIDString string) (*regression.Regression, error) {
	alertID, err := strconv.Atoi(alertIDString)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to convert alertIDString %q to an int.", alertIDString)
	}
	var jsonString string
	if err := s.preparedStatements[read].QueryRowContext(ctx, commitNumber, alertID).Scan(&jsonString); err != nil {
		return nil, skerr.Wrapf(err, "Failed to read regression for alertID: %d commitNumber=%d", alertID, commitNumber)
	}
	r := regression.NewRegression()
	if err := json.Unmarshal([]byte(jsonString), r); err != nil {
		return nil, skerr.Wrapf(err, "Failed to deserialize regression for alertID: %d commitNumber=%d", alertID, commitNumber)
	}
	return r, nil
}

func (s *SQLRegressionStore) readModifyWrite(ctx context.Context, commitNumber types.CommitNumber, alertIDString string, cb func(r *regression.Regression)) error {
	alertID, err := strconv.Atoi(alertIDString)
	if err != nil {
		return skerr.Wrapf(err, "Failed to convert alertIDString %q to an int.", alertIDString)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return skerr.Wrapf(err, "Can't start transaction")
	}

	r := regression.NewRegression()

	// Read the regression from the database. If any part of that fails then
	// just use the default regression we've already constructed.
	var jsonString string
	if err := tx.StmtContext(ctx, s.preparedStatements[read]).QueryRowContext(ctx, commitNumber, alertID).Scan(&jsonString); err == nil {
		if err := json.Unmarshal([]byte(jsonString), r); err != nil {
			sklog.Warningf("Failed to deserialize the JSON Regression: %s", err)
		}
	}

	cb(r)

	b, err := json.Marshal(r)
	if err != nil {
		if err := tx.Rollback(); err != nil {
			sklog.Errorf("Failed on rollback: %s", err)
		}
		return skerr.Wrapf(err, "Failed to serialize regression for alertID: %d  commitNumber=%d", alertID, commitNumber)
	}
	if _, err := tx.StmtContext(ctx, s.preparedStatements[write]).ExecContext(ctx, commitNumber, alertID, string(b)); err != nil {
		if err := tx.Rollback(); err != nil {
			sklog.Errorf("Failed on rollback: %s", err)
		}
		return skerr.Wrapf(err, "Failed to write regression for alertID: %d  commitNumber=%d", alertID, commitNumber)
	}

	return tx.Commit()
}

// Confirm that SQLRegressionStore implements regression.Store.
var _ regression.Store = (*SQLRegressionStore)(nil)
