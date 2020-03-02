// Package sqlregressionstore implements the regression.Store interface on an
// SQL database backend.
//
// To see the schema of the database used, see perf/sql/migrations.
package sqlregressionstore

// statement is an SQL statement identifier.
type statement int

const (
	// The identifiers for all the SQL statements used.
	countUntriaged statement = iota
)

// statements allows looking up raw SQL statements by their statement id.
type statements map[statement]string

// statementsByDialect holds all the raw SQL statemens used per Dialect of SQL.
var statementsByDialect = map[perfsql.Dialect]statements{
	perfsql.SQLiteDialect: {
		countUntriaged: `
`,
	},
	perfsql.CockroachDBDialect: {
		countUntriaged: `
		`,
	},
}

// SQLRegressionStore implements the regression.Store interface.
type SQLRegressionStore struct {
	// preparedStatements are all the prepared SQL statements.
	preparedStatements map[statement]*sql.Stmt
}

// CountUntriaged implements the regression.Store interface.
func (s *SQLRegressionStore) CountUntriaged(ctx context.Context) (int, error) {
	panic("not implemented") // TODO: Implement
}

// Range implements the regression.Store interface.
func (s *SQLRegressionStore) Range(ctx context.Context, begin int64, end int64) (map[string]*regression.Regressions, error) {
	panic("not implemented") // TODO: Implement
}

// SetHigh implements the regression.Store interface.
func (s *SQLRegressionStore) SetHigh(ctx context.Context, cid *cid.CommitDetail, alertID string, df *dataframe.FrameResponse, high *clustering2.ClusterSummary) (bool, error) {
	panic("not implemented") // TODO: Implement
}

// SetLow implements the regression.Store interface.
func (s *SQLRegressionStore) SetLow(ctx context.Context, cid *cid.CommitDetail, alertID string, df *dataframe.FrameResponse, low *clustering2.ClusterSummary) (bool, error) {
	panic("not implemented") // TODO: Implement
}

// TriageLow implements the regression.Store interface.
func (s *SQLRegressionStore) TriageLow(ctx context.Context, cid *cid.CommitDetail, alertID string, tr regression.TriageStatus) error {
	panic("not implemented") // TODO: Implement
}

// TriageHigh implements the regression.Store interface.
func (s *SQLRegressionStore) TriageHigh(ctx context.Context, cid *cid.CommitDetail, alertID string, tr regression.TriageStatus) error {
	panic("not implemented") // TODO: Implement
}

// Write implements the regression.Store interface.
func (s *SQLRegressionStore) Write(ctx context.Context, regressions map[string]*regression.Regressions, lookup regression.DetailLookup) error {
	panic("not implemented") // TODO: Implement
}

// Confirm that SQLRegressionStore implements regression.Store.
var _ regression.Store = (*SQLRegressionStore)(nil)
