// Package git is the minimal interface that Perf needs to interact with a Git
// repo.
package git

import (
	"context"
	"database/sql"
	"time"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitauth"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/config"
	perfsql "go.skia.org/infra/perf/go/sql"
	"go.skia.org/infra/perf/go/types"
)

// statement is an SQL statement identifier.
type statement int

const (
	// The identifiers for all the SQL statements used.
	getMostRecentGitHash statement = iota
	insert
	getCommitNumberFromGitHash
	getCommitNumberFromTime
)

// statements allows looking up raw SQL statements by their statement id.
type statements map[statement]string

// statementsByDialect holds all the raw SQL statemens used per Dialect of SQL.
var statementsByDialect = map[perfsql.Dialect]statements{
	perfsql.SQLiteDialect:      {},
	perfsql.CockroachDBDialect: {},
}

// Git is the minimal functionality Perf needs to interface to Git.
type Git struct {
	repo *git.Checkout

	// preparedStatements are all the prepared SQL statements.
	preparedStatements map[statement]*sql.Stmt
}

/*
This command:

  git rev-list HEAD ^6286eccdf042751401f54696ad38de9f6849284d --pretty=" %aN <%aE>%n %s%n %ct"

Produces the following output:

commit 6079a7810530025d9877916895dd14eb8bb454c0
 Joe Gregorio <joe@bitworking.org>
 Change #9
 1584837783
commit 977e0ef44bec17659faf8c5d4025c5a068354817
 Joe Gregorio <joe@bitworking.org>
 Change #8
 1584837783

Note the output is in reverse chronological order, so keep that in mind when adding to the database.

Note also that CommitNumber starts at 0 for the first commit in a repo.

*/

// New creates a new *Git from the given instance configuration.
func New(ctx context.Context, local bool, db *sql.DB, dialect perfsql.Dialect, instanceConfig *config.InstanceConfig) (*Git, error) {

	if instanceConfig.GitRepoConfig.GitAuthType == config.GitAuthGerrit {
		sklog.Info("Authenticating to Gerrit.")
		ts, err := auth.NewDefaultTokenSource(local, auth.SCOPE_GERRIT)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		if _, err := gitauth.New(ts, "/tmp/git-cookie", true, ""); err != nil {
			return nil, skerr.Wrap(err)
		}
	}
	repo, err := git.NewCheckout(ctx, instanceConfig.GitRepoConfig.URL, instanceConfig.GitRepoConfig.Dir)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	preparedStatements := map[statement]*sql.Stmt{}
	for key, statement := range statementsByDialect[dialect] {
		prepared, err := db.Prepare(statement)
		if err != nil {
			return nil, skerr.Wrapf(err, "Failed to prepare statment %v %q", key, statement)
		}
		preparedStatements[key] = prepared
	}

	// Get the last git hash seen from the database.

	// Load all hashes from last git hash seen onwards into the database.

	// Start a background process that periodically pulls to head and adds the new commits
	// to the database.

	return &Git{
		repo:               repo,
		preparedStatements: preparedStatements,
	}, nil
}

func (g *Git) getMostRecentCommitHash() (string, error)

// CommitNumberFromGitHash looks up the commit number given the git hash.
func (g *Git) CommitNumberFromGitHash(ctx context.Context, githash string) (types.CommitNumber, error) {
	/*
		var err error
		index, err := g.repo.IndexOf(ctx, githash)
		if err != nil {
			if err := g.repo.Update(ctx, true, false); err != nil {
				return types.BadCommitNumber, skerr.Wrap(err)
			}
			index, err = g.repo.IndexOf(ctx, githash)
			if err != nil {
				return types.BadCommitNumber, skerr.Fmt("Failed to find githash %q.", githash)
			}
		}
		commitNumber := types.CommitNumber(index)
		return commitNumber, nil
	*/
	return types.BadCommitNumber, nil
}

// CommitNumberFromTime finds the index of the closest commit with a commit time
// less than or equal to 't'.
//
// Pass in zero time, i.e. time.Time{} to indicate to just get the most recent
// commit.
func (g *Git) CommitNumberFromTime(ctx context.Context, t time.Time) (types.CommitNumber, error) {
	/*
		ctx, span := trace.StartSpan(ctx, "dfbuilder.findIndexForTime")
		defer span.End()

		var err error
		endIndex := 0

		if t.IsZero() {
			commits := g.repo.LastNIndex(1)
			if len(commits) == 0 {
				return 0, fmt.Errorf("Failed to find an end commit.")
			}
			return types.CommitNumber(commits[0].Index), nil
		}

		hashes := g.repo.From(t)
		if len(hashes) > 0 {
			endIndex, err = g.repo.IndexOf(ctx, hashes[0])
			if err != nil {
				return 0, fmt.Errorf("Failed loading end commit: %s", err)
			}
		} else {
			commits := g.repo.LastNIndex(1)
			if len(commits) == 0 {
				return 0, fmt.Errorf("Failed to find an end commit.")
			}
			endIndex = commits[0].Index
		}
		return types.CommitNumber(endIndex), nil
	*/
	return types.BadCommitNumber, nil
}
