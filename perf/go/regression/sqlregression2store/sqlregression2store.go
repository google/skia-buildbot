package sqlregression2store

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"text/template"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sql/pool"
	"go.skia.org/infra/go/sql/sqlutil"
	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/regression"
	"go.skia.org/infra/perf/go/sql"
	"go.skia.org/infra/perf/go/stepfit"
	"go.skia.org/infra/perf/go/types"
	"go.skia.org/infra/perf/go/ui/frame"
)

// SQLRegressionStore implements the regression.Store interface.
type SQLRegression2Store struct {
	// db is the underlying database.
	db                         pool.Pool
	statements                 map[statementFormat]string
	alertConfigProvider        alerts.ConfigProvider
	regressionFoundCounterLow  metrics2.Counter
	regressionFoundCounterHigh metrics2.Counter
}

// statementFormat is an SQL statementFormat identifier.
type statementFormat int

const (
	// The identifiers for all the SQL statements used.
	write statementFormat = iota
	readCompat
	readOldest
	readRange
	readByIDs
	readBySubName
	deleteByCommit
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
	readOldest: `
		SELECT
			commit_number
		FROM
			Regressions2
		ORDER BY
			commit_number ASC
		LIMIT 1
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
	readByIDs: `
		SELECT
			{{ .Columns }}
		FROM
			Regressions2
		WHERE
			id IN (%s)
		`,
	readBySubName: `
		SELECT
			r.id, commit_number, prev_commit_number, alert_id, creation_time, median_before, median_after, is_improvement, cluster_type, cluster_summary, frame, triage_status, triage_message
		FROM
			Regressions2 r
		INNER JOIN
			Alerts a ON r.alert_id=a.id
		WHERE
			a.sub_name = $1
		LIMIT
			$2
		OFFSET
			$3
		`,
	deleteByCommit: `
		DELETE
		FROM
			Regressions2
		WHERE
			commit_number=$1
		`,
}

// New returns a new instance of SQLRegression2Store
func New(db pool.Pool, alertConfigProvider alerts.ConfigProvider) (*SQLRegression2Store, error) {
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
		alertConfigProvider:        alertConfigProvider,
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
		existingRegressionForAlertId := allForCommit.ByAlertID[alertIDString]
		// If there are existing regressions for the commit-alert tuple,
		// merge it into the same regression object for backward compatibility.
		if existingRegressionForAlertId != nil {
			if existingRegressionForAlertId.High != nil && r.Low != nil {
				existingRegressionForAlertId.Low = r.Low
				existingRegressionForAlertId.LowStatus = r.LowStatus
			} else {
				existingRegressionForAlertId.High = r.High
				existingRegressionForAlertId.HighStatus = r.HighStatus
			}
		} else {
			allForCommit.ByAlertID[alertIDString] = r
		}
		ret[r.CommitNumber] = allForCommit
	}
	return ret, nil
}

// SetHigh implements the regression.Store interface.
func (s *SQLRegression2Store) SetHigh(ctx context.Context, commitNumber types.CommitNumber, alertID string, df *frame.FrameResponse, high *clustering2.ClusterSummary) (bool, string, error) {
	ret := false
	regressionID, err := s.updateBasedOnAlertAlgo(ctx, commitNumber, alertID, df, false, func(r *regression.Regression) {
		if r.Frame == nil {
			r.Frame = df
			ret = true
		}
		r.High = high
		if r.HighStatus.Status == regression.None {
			r.HighStatus.Status = regression.Untriaged
		}
		populateRegression2Fields(r)
	})
	s.regressionFoundCounterHigh.Inc(1)
	return ret, regressionID, err
}

// SetLow implements the regression.Store interface.
func (s *SQLRegression2Store) SetLow(ctx context.Context, commitNumber types.CommitNumber, alertID string, df *frame.FrameResponse, low *clustering2.ClusterSummary) (bool, string, error) {
	ret := false
	regressionID, err := s.updateBasedOnAlertAlgo(ctx, commitNumber, alertID, df, false /* mustExist*/, func(r *regression.Regression) {
		if r.Frame == nil {
			r.Frame = df
			ret = true
		}
		r.Low = low
		if r.LowStatus.Status == regression.None {
			r.LowStatus.Status = regression.Untriaged
		}
		populateRegression2Fields(r)
	})
	s.regressionFoundCounterLow.Inc(1)
	return ret, regressionID, err
}

// TriageLow implements the regression.Store interface.
func (s *SQLRegression2Store) TriageLow(ctx context.Context, commitNumber types.CommitNumber, alertID string, tr regression.TriageStatus) error {
	// TODO(ashwinpv): This code will update all regressions with the <commit_id, alert_id> pair.
	// Once we move all the data to the new db, this will need to be updated to take in a specific
	// regression id and update only that.
	_, err := s.readModifyWriteCompat(ctx, commitNumber, alertID, true, func(r *regression.Regression) bool {
		r.LowStatus = tr
		return true
	})
	return err
}

// TriageHigh implements the regression.Store interface.
func (s *SQLRegression2Store) TriageHigh(ctx context.Context, commitNumber types.CommitNumber, alertID string, tr regression.TriageStatus) error {
	// TODO(ashwinpv): This code will update all regressions with the <commit_id, alert_id> pair.
	// Once we move all the data to the new db, this will need to be updated to take in a specific
	// regression id and update only that.
	_, err := s.readModifyWriteCompat(ctx, commitNumber, alertID, true, func(r *regression.Regression) bool {
		r.HighStatus = tr
		return true
	})
	return err
}

// No Op for SQLRegression2Store.
func (s *SQLRegression2Store) GetNotificationId(ctx context.Context, commitNumber types.CommitNumber, alertID string) (string, error) {
	return "", nil
}

// GetOldestCommit implements regression.Store interface
func (s *SQLRegression2Store) GetOldestCommit(ctx context.Context) (*types.CommitNumber, error) {
	var num int
	if err := s.db.QueryRow(ctx, statementFormats[readOldest]).Scan(&num); err != nil {
		return nil, skerr.Wrapf(err, "Failed to fetch oldest commit.")
	}
	commitNumber := types.CommitNumber(num)
	return &commitNumber, nil
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

// Given the subscription name GetRegressionsBySubName gets all the regressions against
// the specified subscription. The response will be paginated according to the provided
// limit and offset.
func (s *SQLRegression2Store) GetRegressionsBySubName(ctx context.Context, sub_name string, limit int, offset int) ([]*regression.Regression, error) {
	statement := s.statements[readBySubName]
	rows, err := s.db.Query(ctx, statement, sub_name, limit, offset)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to get regressions. Query: %s", statement)
	}

	regressions := []*regression.Regression{}
	for rows.Next() {
		r, err := convertRowToRegression(rows)
		if err != nil {
			return nil, skerr.Wrapf(err, "failed to convert row to regression.")
		}
		regressions = append(regressions, r)
	}

	return regressions, nil
}

// Get a list of regressions given a list of regression ids.
func (s *SQLRegression2Store) GetByIDs(ctx context.Context, ids []string) ([]*regression.Regression, error) {
	statement := s.statements[readByIDs]
	query := fmt.Sprintf(statement, quotedSlice(ids))
	rows, err := s.db.Query(ctx, query)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to get regressions by id list. Query: %s", query)
	}

	var regressions []*regression.Regression
	for rows.Next() {
		r, err := convertRowToRegression(rows)
		if err != nil {
			return nil, skerr.Wrapf(err, "failed to convert row to regression.")
		}
		regressions = append(regressions, r)
	}

	return regressions, nil
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

	r.ClusterType = string(clusterType)
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

// updateBasedOnAlertAlgo updates the regression based on the Algo specified in the
// alert config. This is to handle the difference in creating/updating regressions in
// KMeans v/s Individual mode.
// TODO(ashwinpv): Once we are fully on to the regression2 schema, move this logic out
// of the Store (since ideally store should only care about reading and writing data instead
// of the feature semantics)
func (s *SQLRegression2Store) updateBasedOnAlertAlgo(ctx context.Context, commitNumber types.CommitNumber, alertID string, df *frame.FrameResponse, mustExist bool, updateFunc func(r *regression.Regression)) (string, error) {
	// If KMeans the expectation is that as we get more incoming data,
	// the regression becomes more accurate. This means we need to check
	// if there is a regression for the same <commit_id, alert_id> pair
	// and update it.
	var regressionID string
	var err error
	alertConfig, err := s.alertConfigProvider.GetAlertConfig(alerts.IDAsStringToInt(alertID))
	if err != nil {
		return "", err
	}
	if alertConfig.Algo == types.KMeansGrouping {
		regressionID, err = s.readModifyWriteCompat(ctx, commitNumber, alertID, mustExist /* mustExist*/, func(r *regression.Regression) bool {
			updateFunc(r)
			return true
		})
	} else {
		regressionID, err = s.readModifyWriteCompat(ctx, commitNumber, alertID, mustExist /* mustExist*/, func(r *regression.Regression) bool {
			if r.Frame != nil {
				// Do not update existing regressions when the algo is stepfit.
				return false
			}

			// At this point r is a newly created regression. This should get updated in the database.
			updateFunc(r)
			return true
		})
	}

	if err != nil {
		sklog.Errorf("Error while updating database %s", err)
		return "", err
	}

	return regressionID, nil
}

// readModifyWriteCompat reads the Regression at the given commitNumber and alert id
// and then calls the given callback, giving the caller a chance to modify the
// struct, before writing it back to the database.
//
// If mustExist is true then the read must be successful, otherwise a new
// default Regression will be used and stored back to the database after the
// callback is called.
func (s *SQLRegression2Store) readModifyWriteCompat(ctx context.Context, commitNumber types.CommitNumber, alertIDString string, mustExist bool, cb func(r *regression.Regression) bool) (string, error) {
	alertID := alerts.IDAsStringToInt(alertIDString)
	if alertID == alerts.BadAlertID {
		return "", skerr.Fmt("Failed to convert alertIDString %q to an int.", alertIDString)
	}

	// Do everything in a transaction so we don't have any lost updates.
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return "", skerr.Wrapf(err, "Can't start transaction")
	}

	var r *regression.Regression

	rows, err := tx.Query(ctx, s.statements[readCompat], commitNumber, alertID)
	if err != nil {
		rollbackTransaction(ctx, tx)
		return "", err
	}

	regressionsToWrite := []*regression.Regression{}
	existingRows := false
	for rows.Next() {
		existingRows = true
		r, err = convertRowToRegression(rows)
		if err != nil {
			if mustExist {
				var errorMsg string
				if err == pgx.ErrNoRows {
					errorMsg = "Regression does not exist"
				} else {
					errorMsg = "Failed reading regression data."
				}
				rollbackTransaction(ctx, tx)
				return "", skerr.Wrapf(err, errorMsg)
			}
		}

		shouldUpdate := cb(r)

		if shouldUpdate {
			regressionsToWrite = append(regressionsToWrite, r)
		}
	}

	if !existingRows {
		r = regression.NewRegression()
		r.AlertId = alertID
		r.CommitNumber = commitNumber
		r.CreationTime = time.Now().UTC()
		shouldUpdate := cb(r)

		if shouldUpdate {
			regressionsToWrite = append(regressionsToWrite, r)
		}
	}

	// Update all the regressions marked for writing into the db.
	for _, reg := range regressionsToWrite {
		if err = s.writeSingleRegression(ctx, reg, tx); err != nil {
			rollbackTransaction(ctx, tx)
			return "", skerr.Wrapf(err, "Failed to write regression for alertID: %d  commitNumber=%d", alertID, commitNumber)
		}
	}
	return r.Id, tx.Commit(ctx)
}

func rollbackTransaction(ctx context.Context, tx pgx.Tx) {
	if err := tx.Rollback(ctx); err != nil {
		sklog.Errorf("Failed on rollback: %s", err)
	}
}

// WriteRegression writes a single regression object into the table and returns the Id of the written row.
func (s *SQLRegression2Store) WriteRegression(ctx context.Context, regression *regression.Regression, tx pgx.Tx) (string, error) {
	// If the regression has both high and low specified, we need to create two separate regression
	// entries into the regression2 table.
	if regression.High != nil && regression.Low != nil {
		highRegression := *regression
		lowRegression := *regression
		highRegression.Low = nil
		lowRegression.High = nil
		populateRegression2Fields(&highRegression)
		err := s.writeSingleRegression(ctx, &highRegression, tx)
		if err == nil {
			populateRegression2Fields(&lowRegression)
			err = s.writeSingleRegression(ctx, &lowRegression, tx)
		}
		return highRegression.Id, err
	} else {
		populateRegression2Fields(regression)
		err := s.writeSingleRegression(ctx, regression, tx)
		return regression.Id, err
	}
}

// populateRegression2Fields populates the fields in the regression object
// which are specific to the regression2 schema.
func populateRegression2Fields(regression *regression.Regression) {
	if regression.Id == "" {
		regression.Id = uuid.NewString()
	}

	_, clusterSummary, _ := regression.GetClusterTypeAndSummaryAndTriageStatus()

	regression.CreationTime = clusterSummary.Timestamp

	// Find the index of the commit where the regression was detected.
	regressionPointIndex := clusterSummary.StepFit.TurningPoint

	prevCommitNumber := regression.Frame.DataFrame.Header[regressionPointIndex-1].Offset
	regression.PrevCommitNumber = prevCommitNumber

	medianBefore, _, _, _ := vec32.TwoSidedStdDev(clusterSummary.Centroid[:regressionPointIndex])
	regression.MedianBefore = medianBefore
	medianAfter, _, _, _ := vec32.TwoSidedStdDev(clusterSummary.Centroid[regressionPointIndex:])
	regression.MedianAfter = medianAfter

	regression.IsImprovement = isRegressionImprovement(regression.Frame.DataFrame.ParamSet, clusterSummary.StepFit.Status)
}

// isRegressionImprovement returns true if the metric has moved towards the improvement direction.
func isRegressionImprovement(paramset map[string][]string, stepFitStatus stepfit.StepFitStatus) bool {
	if _, ok := paramset["improvement_direction"]; ok {
		improvementDirection := paramset["improvement_direction"]
		return improvementDirection[0] == "down" && stepFitStatus == stepfit.LOW || improvementDirection[0] == "up" && stepFitStatus == stepfit.HIGH
	}

	return false
}

// DeleteByCommit implements the regression.Store interface. Deletes a regression via commit number.
func (s *SQLRegression2Store) DeleteByCommit(ctx context.Context, num types.CommitNumber, tx pgx.Tx) error {
	var err error
	if tx == nil {
		_, err = s.db.Exec(ctx, statementFormats[deleteByCommit], num)
	} else {
		_, err = tx.Exec(ctx, statementFormats[deleteByCommit], num)
	}

	return err
}

// Confirm that SQLRegressionStore implements regression.Store.
var _ regression.Store = (*SQLRegression2Store)(nil)

// Takes a string array as input, and returns a comma joined string where each element
// is single quoted.
func quotedSlice(a []string) string {
	q := make([]string, len(a))
	for i, s := range a {
		q[i] = fmt.Sprintf("'%s'", s)
	}
	return strings.Join(q, ", ")
}
