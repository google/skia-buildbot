// Package sqlgitstore keeps commits from a git repo in an SQL table for faster
// lookup.
package sqlgitstore

import (
	"context"
	"database/sql"

	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/perf/go/config"
	perfsql "go.skia.org/infra/perf/go/sql"
)

// statement is an SQL statement identifier.
type statement int

const (
	// The identifiers for all the SQL statements used.
	insertShortcut statement = iota
	getShortcut
	getAllShortcuts
)

// statements allows looking up raw SQL statements by their statement id.
type statements map[statement]string

// SQLGitStore keeps commits from a git repo in an SQL table for faster
// lookup.
type SQLGitStore struct {
	// preparedStatements are all the prepared SQL statements.
	preparedStatements map[statement]*sql.Stmt
}

// statementsByDialect holds all the raw SQL statemens used per Dialect of SQL.
var statementsByDialect = map[perfsql.Dialect]statements{
	perfsql.SQLiteDialect:      {},
	perfsql.CockroachDBDialect: {},
}

// New returns a new *SQLGitStore.
func New(ctx context.Context, g *gitinfo.GitInfo, instanceConfig *config.InstanceConfig) (*SQLGitStore, error) {
	return nil, nil

}
