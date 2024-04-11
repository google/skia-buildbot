package sqlregression2store

import (
	"bytes"
	"context"
	"strings"
	"text/template"

	"github.com/jackc/pgx/v4"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sql/pool"
	"go.skia.org/infra/go/sql/sqlutil"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/regression"
	"go.skia.org/infra/perf/go/sql"
	"go.skia.org/infra/perf/go/types"
	"go.skia.org/infra/perf/go/ui/frame"
)

// SQLRegressionStore implements the regression.Store interface.
type SQLRegression2Store struct {
	// db is the underlying database.
	db                         pool.Pool
	statements                 map[statementFormat]string
	regressionFoundCounterLow  metrics2.Counter
	regressionFoundCounterHigh metrics2.Counter
}

// statementFormat is an SQL statementFormat identifier.
type statementFormat int

const (
	// The identifiers for all the SQL statements used.
	write statementFormat = iota
	readCompat
	readRange
)

// statementContext provides a struct to expand sql statement templates.
type statementContext struct {
	Columns            string
	ValuesPlaceholders string
}

// statementFormats holds all the raw SQL statement templates.
var statementFormats = map[statementFormat]string{
	// readCompat is the query to read the data similar to regressions table
	// (using commit and alert ids). This allows us to keep it compatible until
	// we are fully migrated to the new schema.
	readCompat: `
		SELECT
			{{ .Columns }}
		FROM
			Regressions2@by_commit_alert
		WHERE
			commit_number=$1 AND alert_id=$2
		`,
	readRange: `
		SELECT
			{{ .Columns }}
		FROM
			Regressions2
		WHERE
			commit_number >= $1
			AND commit_number <= $2
		`,
	write: `
		UPSERT INTO
			Regressions2 ({{ .Columns }})
		VALUES
			{{ .ValuesPlaceholders }}
		`,
}

// New returns a new instance of SQLRegression2Store
func New(db pool.Pool) (*SQLRegression2Store, error) {
	templates := map[statementFormat]string{}
	context := statementContext{
		Columns:            strings.Join(sql.Regressions2, ","),
		ValuesPlaceholders: sqlutil.ValuesPlaceholders(len(sql.Regressions2), 1),
	}
	for key, tmpl := range statementFormats {
		t, err := template.New("").Parse(tmpl)
		if err != nil {
			return nil, skerr.Wrapf(err, "Error parsing template %v, %q", key, tmpl)
		}
		// Expand the template for the SQL.
		var b bytes.Buffer
		if err := t.Execute(&b, context); err != nil {
			return nil, skerr.Wrapf(err, "Failed to execute template %v", key)
		}
		templates[key] = b.String()
	}
	return &SQLRegression2Store{
		db:                         db,
		statements:                 templates,
		regressionFoundCounterLow:  metrics2.GetCounter("perf_regression2_store_found", map[string]string{"direction": "low"}),
		regressionFoundCounterHigh: metrics2.GetCounter("perf_regression2_store_found", map[string]string{"direction": "high"}),
	}, nil
}

// Range implements the regression.Store interface.
func (s *SQLRegression2Store) Range(ctx context.Context, begin, end types.CommitNumber) (map[types.CommitNumber]*regression.AllRegressionsForCommit, error) {
	ret := map[types.CommitNumber]*regression.AllRegressionsForCommit{}
	rows, err := s.db.Query(ctx, s.statements[readRange], begin, end)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to read regressions in range: %d %d", begin, end)
	}
	for rows.Next() {
		r, err := convertRowToRegression(rows)
		if err != nil {
			return nil, err
		}
		allForCommit, ok := ret[r.CommitNumber]
		if !ok {
			allForCommit = regression.New()
		}
		alertIDString := alerts.IDToString(r.AlertId)
		allForCommit.ByAlertID[alertIDString] = r
		ret[r.CommitNumber] = allForCommit
	}
	return ret, nil
}

// SetHigh implements the regression.Store interface.
func (s *SQLRegression2Store) SetHigh(ctx context.Context, commitNumber types.CommitNumber, alertID string, df *frame.FrameResponse, high *clustering2.ClusterSummary) (bool, error) {
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
func (s *SQLRegression2Store) SetLow(ctx context.Context, commitNumber types.CommitNumber, alertID string, df *frame.FrameResponse, low *clustering2.ClusterSummary) (bool, error) {
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
func (s *SQLRegression2Store) TriageLow(ctx context.Context, commitNumber types.CommitNumber, alertID string, tr regression.TriageStatus) error {
	return s.readModifyWrite(ctx, commitNumber, alertID, true, func(r *regression.Regression) {
		r.LowStatus = tr
	})
}

// TriageHigh implements the regression.Store interface.
func (s *SQLRegression2Store) TriageHigh(ctx context.Context, commitNumber types.CommitNumber, alertID string, tr regression.TriageStatus) error {
	return s.readModifyWrite(ctx, commitNumber, alertID, true, func(r *regression.Regression) {
		r.HighStatus = tr
	})
}

// Write implements the regression.Store interface.
func (s *SQLRegression2Store) Write(ctx context.Context, regressions map[types.CommitNumber]*regression.AllRegressionsForCommit) error {
	for _, allRegressionsForCommit := range regressions {
		for _, reg := range allRegressionsForCommit.ByAlertID {
			if err := s.writeSingleRegression(ctx, reg, nil); err != nil {
				return err
			}
		}
	}
	return nil
}

// convertRowToRegression converts the content of the row retrieved from the database
// into a regression object. This will return an error if either there is no data
// in the row, or if the data is invalid (eg: failed data conversion).
func convertRowToRegression(rows pgx.Row) (*regression.Regression, error) {
	r := regression.NewRegression()

	// Once we are fully migrated to regression2 schema, the variables below
	// will be directly read into the regression.Regression object.
	var clusterType regression.ClusterType
	var clusterSummary clustering2.ClusterSummary
	var triageStatus string
	var triageMessage string
	err := rows.Scan(&r.Id, &r.CommitNumber, &r.PrevCommitNumber, &r.AlertId, &r.CreationTime, &r.MedianBefore, &r.MedianAfter, &r.IsImprovement, &clusterType, &clusterSummary, &r.Frame, &triageStatus, &triageMessage)
	if err != nil {
		return nil, err
	}

	switch clusterType {
	case regression.HighClusterType:
		r.High = &clusterSummary
		r.HighStatus = regression.TriageStatus{
			Status:  regression.Status(triageStatus),
			Message: triageMessage,
		}
	case regression.LowClusterType:
		r.Low = &clusterSummary
		r.LowStatus = regression.TriageStatus{
			Status:  regression.Status(triageStatus),
			Message: triageMessage,
		}
	default:
		// Do nothing
	}

	return r, nil
}

// writeSingleRegression writes the regression.Regression object to the database.
// If the tx is specified, the write occurs within the transaction.
func (s *SQLRegression2Store) writeSingleRegression(ctx context.Context, r *regression.Regression, tx pgx.Tx) error {
	clusterType, clusterSummary, triage := r.GetClusterTypeAndSummaryAndTriageStatus()

	var err error
	if tx == nil {
		_, err = s.db.Exec(ctx, s.statements[write], r.Id, r.CommitNumber, r.PrevCommitNumber, r.AlertId, r.CreationTime, r.MedianBefore, r.MedianAfter, r.IsImprovement, clusterType, clusterSummary, r.Frame, triage.Status, triage.Message)
	} else {
		_, err = tx.Exec(ctx, s.statements[write], r.Id, r.CommitNumber, r.PrevCommitNumber, r.AlertId, r.CreationTime, r.MedianBefore, r.MedianAfter, r.IsImprovement, clusterType, clusterSummary, r.Frame, triage.Status, triage.Message)
	}
	if err != nil {
		return skerr.Wrapf(err, "Failed to write single regression with id %s", r.Id)
	}
	return nil
}

// readModifyWrite reads the Regression at the given commitNumber and alert id
// and then calls the given callback, giving the caller a chance to modify the
// struct, before writing it back to the database.
//
// If mustExist is true then the read must be successful, otherwise a new
// default Regression will be used and stored back to the database after the
// callback is called.
func (s *SQLRegression2Store) readModifyWrite(ctx context.Context, commitNumber types.CommitNumber, alertIDString string, mustExist bool, cb func(r *regression.Regression)) error {
	alertID := alerts.IDAsStringToInt(alertIDString)
	if alertID == alerts.BadAlertID {
		return skerr.Fmt("Failed to convert alertIDString %q to an int.", alertIDString)
	}

	// Do everything in a transaction so we don't have any lost updates.
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return skerr.Wrapf(err, "Can't start transaction")
	}

	row := tx.QueryRow(ctx, s.statements[readCompat], commitNumber, alertID)
	r, err := convertRowToRegression(row)
	if err != nil {
		if mustExist {
			var errorMsg string
			if err == pgx.ErrNoRows {
				errorMsg = "Regression does not exist"
			} else {
				errorMsg = "Failed reading regression data."
			}
			if err := tx.Rollback(ctx); err != nil {
				sklog.Errorf("Failed on rollback: %s", err)
			}
			return skerr.Wrapf(err, errorMsg)
		} else {
			r = regression.NewRegression()
		}
	}

	cb(r)

	if err := s.writeSingleRegression(ctx, r, tx); err != nil {
		if err := tx.Rollback(ctx); err != nil {
			sklog.Errorf("Failed on rollback: %s", err)
		}
		return skerr.Wrapf(err, "Failed to write regression for alertID: %d  commitNumber=%d", alertID, commitNumber)
	}

	return tx.Commit(ctx)
}

// Confirm that SQLRegressionStore implements regression.Store.
var _ regression.Store = (*SQLRegression2Store)(nil)
