package sqlregression2store

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	"go.opencensus.io/trace"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sql/pool"
	"go.skia.org/infra/go/sql/sqlutil"
	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/regression"
	"go.skia.org/infra/perf/go/sql/spanner"
	"go.skia.org/infra/perf/go/stepfit"
	pb "go.skia.org/infra/perf/go/subscription/proto/v1"
	"go.skia.org/infra/perf/go/types"
	"go.skia.org/infra/perf/go/ui/frame"
)

// SQLRegressionStore implements the regression.Store interface.
type SQLRegression2Store struct {
	// db is the underlying database.
	db                         pool.Pool
	statements                 map[statementFormat]string
	alertConfigProvider        alerts.ConfigProvider
	instanceConfig             *config.InstanceConfig
	regressionFoundCounterLow  metrics2.Counter
	regressionFoundCounterHigh metrics2.Counter
}

// statementFormat is an SQL statementFormat identifier.
type statementFormat int

const (
	// The identifiers for all the SQL statements used.
	write statementFormat = iota
	readCompat
	readRegressionsByCommitAlertAndTraceName
	readOldest
	readRange
	readByRev
	readByIDs
	readIdsByManualTriageBugId
	readBySubName
	deleteByCommit
	readRangeFiltered
	setBugID
	ignoreAnomalies
	resetAnomalies
	nudgeAndReset
	readBugsForRegressions
	getSubscriptionsForRegressions
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
			Regressions2
		WHERE
			commit_number=$1 AND alert_id=$2
		`,
	readRegressionsByCommitAlertAndTraceName: `
		SELECT
			{{ .Columns }}
		FROM
			Regressions2
		WHERE
			commit_number=$1
			AND alert_id=$2
			AND (frame->'dataframe'->'traceset') ? $3
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
	readByRev: `
		SELECT
			{{ .Columns }}
		FROM
			Regressions2
		WHERE
			prev_commit_number < $1
			AND commit_number >= $1
	`,
	readRangeFiltered: `
		SELECT
			{{ .Columns }}
		FROM
			Regressions2
		WHERE
			commit_number >= $1
			AND commit_number <= $2
			AND (frame->'dataframe'->'traceset') ?| $3
	`,
	write: `
		INSERT INTO
			Regressions2 ({{ .Columns }})
		VALUES
			{{ .ValuesPlaceholders }}
		ON CONFLICT (id) DO UPDATE
        SET median_before=EXCLUDED.median_before, median_after=EXCLUDED.median_after, frame=EXCLUDED.frame,
		triage_status=EXCLUDED.triage_status, triage_message=EXCLUDED.triage_message, alert_id=EXCLUDED.alert_id,
		bug_id=EXCLUDED.bug_id, cluster_summary=EXCLUDED.cluster_summary, cluster_type=EXCLUDED.cluster_type,
		commit_number=EXCLUDED.commit_number, creation_time=EXCLUDED.creation_time, id=EXCLUDED.id,
		is_improvement=EXCLUDED.is_improvement, prev_commit_number=EXCLUDED.prev_commit_number,
		sub_name=EXCLUDED.sub_name
		`,
	readByIDs: `
		SELECT
			{{ .Columns }}
		FROM
			Regressions2
		WHERE
			id IN (%s)
		ORDER BY
			id
		`,
	readIdsByManualTriageBugId: `
		SELECT
			distinct id
		FROM
			Regressions2
		WHERE
			bug_id = $1
		ORDER BY
			id
		`,
	readBySubName: `
		SELECT
			{{ .Columns }}
		FROM
			Regressions2
		WHERE
			sub_name = $1 and
			(is_improvement = $2 OR is_improvement = false) AND -- toggle improvements on the flag, show regressions always
			(triage_status = 'untriaged' OR (triage_status != '' AND $3 = true)) -- show untriaged always, and show all statuses except for NONE if showTriaged is true
		ORDER BY
  		creation_time DESC
		LIMIT
			$4
		OFFSET
			$5
		`,
	deleteByCommit: `
		DELETE
		FROM
			Regressions2
		WHERE
			commit_number=$1
		`,
	setBugID: `
		UPDATE Regressions2
		SET
			bug_id = $1,
			triage_status = 'negative',
			triage_message = 'triaged'
		WHERE id = ANY($2)
		`,
	ignoreAnomalies: `
		UPDATE Regressions2
		SET triage_status = 'ignored', triage_message = 'Ignored via Triage Menu'
		WHERE id = ANY($1)
		`,
	resetAnomalies: `
		UPDATE Regressions2
		SET triage_status = 'untriaged', triage_message = '', bug_id = 0
		WHERE id = ANY($1)
		`,
	nudgeAndReset: `
		UPDATE Regressions2
		SET commit_number = $1, prev_commit_number = $2, triage_status = 'untriaged', triage_message = 'Nudged', bug_id = 0
		WHERE id = ANY($3)
		`,
	readBugsForRegressions: `
		select
			regressions2.id as regression2_id,
			anomalygroups.id as anomalygroups_id,
			reported_issue_id as anomalygroups_reported_issue_id,
			culprits.id as culprits_id,
			COALESCE(issue_ids, '{}') as culprits_issue_ids,
			culprits.group_issue_map as culprits_group_issue_map
		from
			regressions2 left join anomalygroups on (regressions2.id = any(anomaly_ids))
			left join culprits on (anomalygroups.id = any(anomaly_group_ids))
		where
			regressions2.id = ANY($1) AND
			anomalygroups.common_rev_start <= anomalygroups.common_rev_end
		order by regressions2.id
	`,
	getSubscriptionsForRegressions: `
		SELECT
			regressions2.id AS regression2_id,
			alerts.id AS alert_id,
			COALESCE(subscriptions.bug_component, ''),
			subscriptions.bug_priority,
			subscriptions.bug_severity,
			COALESCE(subscriptions.bug_cc_emails, '{}'::TEXT[]),
			COALESCE(subscriptions.contact_email, '')
		FROM
			regressions2 JOIN alerts
			ON regressions2.alert_id = alerts.id
			JOIN subscriptions
			ON alerts.sub_name = subscriptions.name
		WHERE regressions2.id=ANY($1) and subscriptions.is_active = TRUE
		ORDER BY regressions2.id
	`,
}

// New returns a new instance of SQLRegression2Store
func New(db pool.Pool, alertConfigProvider alerts.ConfigProvider, instanceConfig *config.InstanceConfig) (*SQLRegression2Store, error) {
	templates := map[statementFormat]string{}
	context := statementContext{
		Columns:            strings.Join(spanner.Regressions2, ","),
		ValuesPlaceholders: sqlutil.ValuesPlaceholders(len(spanner.Regressions2), 1),
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
		instanceConfig:             instanceConfig,
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
	defer rows.Close()
	regressions, err := s.convertRowsIntoRegressions(rows)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to convert rows into regressions")
	}

	for _, r := range regressions {
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

// RangeFiltered gets all regressions in the given commit range and trace names.
func (s *SQLRegression2Store) RangeFiltered(ctx context.Context, begin, end types.CommitNumber, traceNames []string) ([]*regression.Regression, error) {
	rows, err := s.db.Query(ctx, s.statements[readRangeFiltered], begin, end, traceNames)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, skerr.Wrapf(err, "Failed to read regressions in range [%d; %d] for %d traces. PgError %s: %s", begin, end, len(traceNames), pgErr.Code, pgErr.Message)
		}
		return nil, skerr.Wrapf(err, "Failed to read regressions in range [%d; %d] for %d traces.", begin, end, len(traceNames))
	}
	defer rows.Close()
	regressions, err := s.convertRowsIntoRegressions(rows)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to convert rows into regressions")
	}
	return regressions, nil
}

// SetHigh implements the regression.Store interface.
func (s *SQLRegression2Store) SetHigh(ctx context.Context, commitNumber types.CommitNumber, alertID string, df *frame.FrameResponse, high *clustering2.ClusterSummary) (bool, string, error) {
	ctx, span := trace.StartSpan(ctx, "sqlregression2store.SetHigh")
	defer span.End()
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
	ctx, span := trace.StartSpan(ctx, "sqlregression2store.SetLow")
	defer span.End()

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
	_, err := s.readModifyWriteCompat(ctx, commitNumber, alertID, "", true, func(r *regression.Regression) bool {
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
	_, err := s.readModifyWriteCompat(ctx, commitNumber, alertID, "", true, func(r *regression.Regression) bool {
		r.HighStatus = tr
		return true
	})
	return err
}

// No Op for SQLRegression2Store.
func (s *SQLRegression2Store) GetRegression(ctx context.Context, commitNumber types.CommitNumber, alertID string) (*regression.Regression, error) {
	return nil, nil
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
func (s *SQLRegression2Store) GetRegressionsBySubName(ctx context.Context, req regression.GetAnomalyListRequest, limit int) ([]*regression.Regression, error) {
	statement := s.statements[readBySubName]
	rows, err := s.db.Query(ctx, statement, req.SubName, req.IncludeImprovements, req.IncludeTriaged, limit, req.PaginationOffset)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to get regressions. Query: %s", statement)
	}
	defer rows.Close()

	regressions, err := s.convertRowsIntoRegressions(rows)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to convert rows into regressions")
	}

	return regressions, nil
}

// Get a list of regressions given a list of regression ids.
func (s *SQLRegression2Store) GetByIDs(ctx context.Context, ids []string) ([]*regression.Regression, error) {
	if len(ids) == 0 {
		sklog.Warning("GetByIDs received an empty ids list.")
		return []*regression.Regression{}, nil
	}
	statement := s.statements[readByIDs]
	query := fmt.Sprintf(statement, quotedSlice(ids))
	rows, err := s.db.Query(ctx, query)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to get regressions by id list. Query: %s", query)
	}
	defer rows.Close()

	regressions, err := s.convertRowsIntoRegressions(rows)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to convert rows into regressions")
	}

	return regressions, nil
}

// Get a list of regressions given a manual triage bug id.
func (s *SQLRegression2Store) GetIdsByManualTriageBugID(ctx context.Context, bugId int) ([]string, error) {
	statement := s.statements[readIdsByManualTriageBugId]
	rows, err := s.db.Query(ctx, statement, bugId)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to get regressions by manual triage bug id")
	}
	defer rows.Close()

	regIDs := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return nil, skerr.Wrapf(err, "error parsing the returned regression ids")
		} else {
			regIDs = append(regIDs, id)
		}
	}

	return regIDs, nil
}

// Get a list of regressions given a revision.
func (s *SQLRegression2Store) GetByRevision(ctx context.Context, rev string) ([]*regression.Regression, error) {
	revInt, err := strconv.ParseInt(rev, 10, 64)
	if err != nil {
		return []*regression.Regression{}, skerr.Fmt("failed to convert rev %s to int: %s", rev, err)
	}
	statement := s.statements[readByRev]
	rows, err := s.db.Query(ctx, statement, revInt)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to get regressions by revision")
	}
	defer rows.Close()

	regressions, err := s.convertRowsIntoRegressions(rows)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to convert rows into regressions")
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
	var bugId sql.NullInt64
	var subName sql.NullString
	err := rows.Scan(&r.Id, &r.CommitNumber, &r.PrevCommitNumber, &r.AlertId, &subName, &bugId, &r.CreationTime, &r.MedianBefore, &r.MedianAfter, &r.IsImprovement, &clusterType, &clusterSummary, &r.Frame, &triageStatus, &triageMessage)
	if err != nil {
		return nil, err
	}

	// We are not storing bugId = 0 (which means no bug assigned) to save up some space and avoid deduplication.
	if bugId.Valid && bugId.Int64 != int64(0) {
		r.Bugs = []types.RegressionBug{{BugId: fmt.Sprint(bugId.Int64), Type: types.ManualTriage}}
	}

	if subName.Valid && subName.String != "" {
		r.SubscriptionName = subName.String
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

func (s *SQLRegression2Store) GetBugIdsForRegressions(ctx context.Context, regressions []*regression.Regression) ([]*regression.Regression, error) {
	ids := make([]string, len(regressions))
	idBugs := map[string][]types.RegressionBug{}
	for i, r := range regressions {
		ids[i] = r.Id
		for _, bug := range r.Bugs {
			if bug.Type == types.ManualTriage {
				idBugs[r.Id] = append(idBugs[r.Id], bug)
			}
		}
		// Invariant: there is at most 1 manually assigned bug.
		if len(idBugs[r.Id]) > 1 {
			return nil, skerr.Fmt("regression %s has %d - more than 1 manually assigned bugs", r.Id, len(idBugs[r.Id]))
		}
	}
	statement := s.statements[readBugsForRegressions]
	rows, err := s.db.Query(ctx, statement, ids)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to query bug ids for %d regressions due to %s", len(regressions), err)
	}
	defer rows.Close()

	for rows.Next() {
		var regression_id string
		var agid sql.NullString
		var reported_issue_id sql.NullString
		var culprit_id sql.NullString
		var culprit_issue_ids []string
		var group_issue_map sql.NullString

		err = rows.Scan(&regression_id, &agid, &reported_issue_id, &culprit_id, &culprit_issue_ids, &group_issue_map)
		if err != nil {
			return nil, skerr.Wrapf(err, "Failed to read bug ids from query")
		}

		if reported_issue_id.Valid {
			idBugs[regression_id] = append(idBugs[regression_id], types.RegressionBug{
				BugId: reported_issue_id.String,
				Type:  types.AutoTriage,
			})
		}
		idBugs[regression_id] = append(idBugs[regression_id], extractBugFromCulprit(
			agid, culprit_id, culprit_issue_ids, group_issue_map,
		)...)
	}

	for i, r := range regressions {
		regressions[i].Bugs = sortBugs(idBugs[r.Id])
		regressions[i].AllBugsFetched = true
	}

	return regressions, nil
}

func extractBugFromCulprit(agid, culprit_id sql.NullString, culprit_issue_ids []string, group_issue_map sql.NullString) []types.RegressionBug {
	if !culprit_id.Valid {
		return []types.RegressionBug{}
	}
	if !agid.Valid {
		sklog.Errorf("sqlregression2store: culprit id is valid but anomaly group id is not")
		return []types.RegressionBug{}
	}
	if group_issue_map.Valid {
		var issueMap map[string]string
		err := json.Unmarshal([]byte(group_issue_map.String), &issueMap)
		if err != nil {
			sklog.Errorf("failed to unmarshall group issue map: %s", err)
			return []types.RegressionBug{}
		}
		// If culpritId is valid, anomalygroup should be, too.
		v, ok := issueMap[agid.String]
		if !ok {
			sklog.Errorf("anomalygroup id was not present on the culprit issueMap %s", group_issue_map.String)
			return []types.RegressionBug{}
		}
		return []types.RegressionBug{{BugId: v, Type: types.AutoBisect}}
	}
	sklog.Warningf("sqlregression2store: group_issue_map is not valid, but unexpectedly, culprit id %s is.", culprit_id.String)
	// We use coalesce with an empty array, so culprit issue ids is never null.
	result := make([]types.RegressionBug, len(culprit_issue_ids))
	for i, r := range culprit_issue_ids {
		result[i] = types.RegressionBug{
			BugId: r,
			Type:  types.AutoBisect,
		}
	}
	return result
}

func sortBugs(bugs []types.RegressionBug) []types.RegressionBug {
	typeRank := map[types.BugType]int{
		types.ManualTriage: 1,
		types.AutoTriage:   2,
		types.AutoBisect:   3,
	}

	slices.SortFunc(bugs, func(a, b types.RegressionBug) int {
		// Unidentified bug types will be sorted at the end.
		ranki, ok := typeRank[a.Type]
		if !ok {
			ranki = 4
		}
		rankj, ok := typeRank[b.Type]
		if !ok {
			rankj = 4
		}
		if ranki != rankj {
			return ranki - rankj
		}

		// if types are the same, sort by bugId
		// bugIds are ints as long as we're using buganizer.
		aBugId, err := strconv.Atoi(a.BugId)
		compareAsStrings := false
		if err != nil {
			sklog.Error("failed to compare bug ids, comparing as strings instead")
			compareAsStrings = true
		}
		bBugId, err := strconv.Atoi(b.BugId)
		if err != nil {
			sklog.Error("failed to compare bug ids, comparing as strings instead")
			compareAsStrings = true
		}
		if compareAsStrings {
			if a.BugId < b.BugId {
				return -1
			}
			if a.BugId > b.BugId {
				return 1
			}
			return 0
		}
		return aBugId - bBugId
	})

	return bugs
}

func (s *SQLRegression2Store) convertRowsIntoRegressions(rows pgx.Rows) ([]*regression.Regression, error) {
	var regressions []*regression.Regression
	for rows.Next() {
		r, err := convertRowToRegression(rows)
		if err != nil {
			return nil, err
		}
		regressions = append(regressions, r)
	}
	return regressions, nil
}

// writeSingleRegression writes the regression.Regression object to the database.
// If the tx is specified, the write occurs within the transaction.
func (s *SQLRegression2Store) writeSingleRegression(ctx context.Context, r *regression.Regression, tx pgx.Tx) error {
	clusterType, clusterSummary, triage := r.GetClusterTypeAndSummaryAndTriageStatus()
	r.CreationTime = time.Now()
	var err error
	manualTriageBugId, err := selectManualBugFromRegression(r)
	if err != nil {
		return skerr.Wrap(err)
	}
	if tx == nil {
		_, err = s.db.Exec(ctx, s.statements[write], r.Id, r.CommitNumber, r.PrevCommitNumber, r.AlertId, r.SubscriptionName, manualTriageBugId, r.CreationTime, r.MedianBefore, r.MedianAfter, r.IsImprovement, clusterType, clusterSummary, r.Frame, triage.Status, triage.Message)
	} else {
		_, err = tx.Exec(ctx, s.statements[write], r.Id, r.CommitNumber, r.PrevCommitNumber, r.AlertId, r.SubscriptionName, manualTriageBugId, r.CreationTime, r.MedianBefore, r.MedianAfter, r.IsImprovement, clusterType, clusterSummary, r.Frame, triage.Status, triage.Message)
	}
	if err != nil {
		return skerr.Wrapf(err, "Failed to write single regression with id %s", r.Id)
	}
	return nil
}

func selectManualBugFromRegression(r *regression.Regression) (sql.NullInt64, error) {
	var bugId int
	for _, b := range r.Bugs {
		if b.Type == types.ManualTriage {
			bug, err := strconv.Atoi(b.BugId)
			if err != nil {
				return sql.NullInt64{Valid: false}, skerr.Wrapf(err, "failed to convert bug id: %s", b.BugId)
			}
			// Should never happen unless there's a bug in the program
			if bugId != 0 {
				return sql.NullInt64{Valid: false}, skerr.Fmt("found more than one manually triaged bugs for regression %s: %d and %d", r.Id, bugId, bug)
			}
			bugId = bug
		}
	}
	if bugId == 0 {
		return sql.NullInt64{Valid: false}, nil
	}
	return sql.NullInt64{Valid: true, Int64: int64(bugId)}, nil
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
		regressionID, err = s.readModifyWriteCompat(ctx, commitNumber, alertID, "", mustExist /* mustExist*/, func(r *regression.Regression) bool {
			if r.SubscriptionName == "" {
				r.SubscriptionName = alertConfig.SubscriptionName
			}
			updateFunc(r)
			return true
		})
	} else {
		traceName := ""
		for key := range df.DataFrame.TraceSet {
			traceName = key
			break
		}
		if traceName == "" {
			sklog.Errorf("An empty trace name is not expected when running stepfit grouping.")
		}
		regressionID, err = s.readModifyWriteCompat(ctx, commitNumber, alertID, traceName, mustExist /* mustExist*/, func(r *regression.Regression) bool {
			if r.Frame != nil {
				// Do not update existing regressions when the algo is stepfit.
				return false
			}

			if r.SubscriptionName == "" {
				r.SubscriptionName = alertConfig.SubscriptionName
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
func (s *SQLRegression2Store) readModifyWriteCompat(ctx context.Context, commitNumber types.CommitNumber, alertIDString string, traceName string, mustExist bool, cb func(r *regression.Regression) bool) (string, error) {
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
	var rows pgx.Rows

	// An empty trace_name indicates that we are processing a k-means alert, which requires a query without a trace name filter.
	if s.instanceConfig.AllowMultipleRegressionsPerAlertId && traceName != "" {
		rows, err = tx.Query(ctx, s.statements[readRegressionsByCommitAlertAndTraceName], commitNumber, alertID, traceName)
	} else {
		rows, err = tx.Query(ctx, s.statements[readCompat], commitNumber, alertID)
	}

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
				return "", skerr.Wrapf(err, "%s", errorMsg)
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

// SetBugID associates a set of regressions, identified by their IDs, with a bug ID.
func (s *SQLRegression2Store) SetBugID(ctx context.Context, regressionIDs []string, bugID int) error {
	if len(regressionIDs) == 0 {
		return nil // Nothing to update
	}

	cmdTag, err := s.db.Exec(ctx, s.statements[setBugID], bugID, regressionIDs)
	if err != nil {
		return skerr.Wrapf(err, "failed to update bug_id for regressions")
	}

	sklog.Infof("Set bug_id=%d for %d regressions", bugID, cmdTag.RowsAffected())
	return nil
}

// IgnoreAnomalies sets the triage status to Ignored and message to IgnoredMessage for the given regressions.
func (s *SQLRegression2Store) IgnoreAnomalies(ctx context.Context, regressionIDs []string) error {
	if len(regressionIDs) == 0 {
		return nil
	}
	_, err := s.db.Exec(ctx, s.statements[ignoreAnomalies], regressionIDs)
	if err != nil {
		return skerr.Wrapf(err, "Failed to set triage status to ignored for %v", regressionIDs)
	}
	return nil
}

// ResetAnomalies sets the triage status to Untriaged, message to ResetMessage, and bugID to 0 for the given regressions.
func (s *SQLRegression2Store) ResetAnomalies(ctx context.Context, regressionIDs []string) error {
	if len(regressionIDs) == 0 {
		return nil
	}
	_, err := s.db.Exec(ctx, s.statements[resetAnomalies], regressionIDs)
	if err != nil {
		return skerr.Wrapf(err, "Failed to reset anomalies for %v", regressionIDs)
	}
	return nil
}

// NudgeAndResetAnomalies updates the commit number and previous commit number for the given regressions,
// and also sets the triage status to Untriaged, message to NudgedMessage, and bugID to 0.
func (s *SQLRegression2Store) NudgeAndResetAnomalies(ctx context.Context, regressionIDs []string, commitNumber, prevCommitNumber types.CommitNumber) error {
	if len(regressionIDs) == 0 {
		return nil
	}

	_, err := s.db.Exec(ctx, s.statements[nudgeAndReset], commitNumber, prevCommitNumber, regressionIDs)
	if err != nil {
		return skerr.Wrapf(err, "Failed to nudge regressions %v", regressionIDs)
	}

	sklog.Infof("Nudged and updated triage status for %d regressions", len(regressionIDs))
	return nil
}

// GetAlertIDsFromRegressionIDs retrieves all distinct alert_ids for the given regression IDs.
func (s *SQLRegression2Store) GetSubscriptionsForRegressions(ctx context.Context, regressionIDs []string) ([]string, []int64, []*pb.Subscription, error) {
	if len(regressionIDs) == 0 {
		return nil, nil, nil, nil
	}

	rows, err := s.db.Query(ctx, s.statements[getSubscriptionsForRegressions], regressionIDs)
	if err != nil {
		return nil, nil, nil, skerr.Wrapf(err, "failed to get alert_ids for regressions")
	}
	defer rows.Close()

	var regressionIDsFromSql []string
	var alertIDs []int64
	var subscriptions []*pb.Subscription
	for rows.Next() {
		var regressionID string
		var alertID int64
		var subscription pb.Subscription
		var bugPriority sql.NullInt64
		var bugSeverity sql.NullInt64

		if err := rows.Scan(&regressionID, &alertID, &subscription.BugComponent, &bugPriority, &bugSeverity, &subscription.BugCcEmails, &subscription.ContactEmail); err != nil {
			return nil, nil, nil, skerr.Wrap(err)
		}

		if bugPriority.Valid {
			subscription.BugPriority = int32(bugPriority.Int64)
		}
		if bugSeverity.Valid {
			subscription.BugSeverity = int32(bugSeverity.Int64)
		}

		regressionIDsFromSql = append(regressionIDsFromSql, regressionID)
		alertIDs = append(alertIDs, alertID)
		subscriptions = append(subscriptions, &subscription)
	}

	return regressionIDsFromSql, alertIDs, subscriptions, nil
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
