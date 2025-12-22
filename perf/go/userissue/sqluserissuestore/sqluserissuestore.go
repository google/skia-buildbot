// Package sqluserissuestore implements userissue.Store using an SQL database.

package sqluserissuestore

import (
	"bytes"
	"context"
	"text/template"
	"time"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sql/pool"
	"go.skia.org/infra/go/sql/sqlutil"
	"go.skia.org/infra/perf/go/userissue"
)

// statement is an SQL statement identifier.
type statement int

const (
	// The identifiers for all the SQL statements used.
	listUserIssues statement = iota
	saveUserIssue
	deleteUserIssue
	getUserIssue
)

// listUserIssuesContext is the context for the listUserIssues template.
type listUserIssuesContext struct {
	TraceKeys     string
	BeginPosition int64
	EndPosition   int64
}

// statements hold all the raw SQL statements.
var statements = map[statement]string{
	listUserIssues: `
		SELECT
			user_id, trace_key, commit_position, issue_id
		FROM
			UserIssues
		WHERE
			trace_key IN {{ .TraceKeys }} AND commit_position>={{ .BeginPosition }} AND commit_position<={{ .EndPosition }}
	`,
	saveUserIssue: `
		INSERT INTO
			UserIssues (user_id, trace_key, commit_position, issue_id, last_modified)
		VALUES
			($1, $2, $3, $4, $5)
	`,
	deleteUserIssue: `
		DELETE
		FROM
			UserIssues
		WHERE
			trace_key=$1 AND commit_position=$2
	`,
	getUserIssue: `
		SELECT
			user_id, trace_key, commit_position, issue_id
		FROM
			UserIssues
		WHERE
			trace_key=$1 AND commit_position=$2
	`,
}

// UserIssueStore implements the userissue.Store interface using an SQL
// database.
type UserIssueStore struct {
	db pool.Pool
}

// New returns a new *UserissueStore.
func New(db pool.Pool) *UserIssueStore {
	return &UserIssueStore{
		db: db,
	}
}

// Create implements the userissue.Store interface.
func (s *UserIssueStore) Save(ctx context.Context, req *userissue.UserIssue) error {
	now := time.Now()
	if _, err := s.db.Exec(ctx, statements[saveUserIssue], req.UserId, req.TraceKey, req.CommitPosition, req.IssueId, now); err != nil {
		return skerr.Wrapf(err, "Failed to insert userissue for traceKey=%s and commitPosition=%d", req.TraceKey, req.CommitPosition)
	}
	return nil
}

// Delete implements the userissues.Store interface.
func (s *UserIssueStore) Delete(ctx context.Context, traceKey string, commitPosition int64) error {
	userIssue := userissue.UserIssue{}
	if err := s.db.QueryRow(ctx, statements[getUserIssue], traceKey, commitPosition).Scan(
		&userIssue.UserId,
		&userIssue.TraceKey,
		&userIssue.CommitPosition,
		&userIssue.IssueId,
	); err != nil {
		return skerr.Wrapf(err, "No such record exists for trace key=%s and commit position=%d", traceKey, commitPosition)
	}

	if _, err := s.db.Exec(ctx, statements[deleteUserIssue], traceKey, commitPosition); err != nil {
		return skerr.Wrapf(err, "Failed to delete record for trace key=%s and commit position=%d", traceKey, commitPosition)
	}

	return nil
}

// GetPoints implements the userissues.Store interface.
func (s *UserIssueStore) GetUserIssuesForTraceKeys(ctx context.Context, traceKeys []string, begin int64, end int64) ([]userissue.UserIssue, error) {
	// Get the raw statement for listing all user issues
	statementTemplate, _ := template.New("list_user_template").Parse(statements[listUserIssues])
	context := listUserIssuesContext{
		TraceKeys:     sqlutil.ValuesPlaceholders(len(traceKeys), 1),
		BeginPosition: begin,
		EndPosition:   end,
	}

	// Expand the template for the SQL.
	var b bytes.Buffer
	if err := statementTemplate.Execute(&b, context); err != nil {
		return nil, skerr.Wrapf(err, "failed to expand listUserIssues template")
	}
	// sql is the expanded statement that we can use to query
	sql := b.String()

	// The Query function below only accepts []interface{}
	traceKeyList := make([]interface{}, len(traceKeys))
	for i, v := range traceKeys {
		traceKeyList[i] = v
	}

	rows, err := s.db.Query(ctx, sql, traceKeyList...)
	if err != nil {
		return nil, err
	}

	defer rows.Close()
	output := []userissue.UserIssue{}
	for rows.Next() {
		userissueobj := userissue.UserIssue{}
		if err := rows.Scan(
			&userissueobj.UserId,
			&userissueobj.TraceKey,
			&userissueobj.CommitPosition,
			&userissueobj.IssueId,
		); err != nil {
			return nil, err
		}

		output = append(output, userissueobj)
	}

	return output, nil
}
