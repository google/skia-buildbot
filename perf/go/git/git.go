// Package git is the minimal interface that Perf needs to interact with a Git
// repo.
package git

import (
	"bufio"
	"context"
	"database/sql"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/gitauth"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/perf/go/config"
	perfsql "go.skia.org/infra/perf/go/sql"
	"go.skia.org/infra/perf/go/types"
)

// statement is an SQL statement identifier.
type statement int

const (
	// The identifiers for all the SQL statements used.
	getMostRecentGitHashAndCommitNumber statement = iota
	insert
	getCommitNumberFromGitHash
	getCommitNumberFromTime
)

// statements allows looking up raw SQL statements by their statement id.
type statements map[statement]string

// statementsByDialect holds all the raw SQL statemens used per Dialect of SQL.
var statementsByDialect = map[perfsql.Dialect]statements{
	perfsql.SQLiteDialect: {
		getMostRecentGitHashAndCommitNumber: `
		SELECT
			git_hash, commit_number
		FROM
			Commits
		ORDER BY
			commit_number DESC
		LIMIT
			1;
		`,
		insert: `
		INSERT OR IGNORE INTO
			Commits (commit_number, git_hash, commit_time, author, subject)
		VALUES
  			(?, ?, ?, ?, ?);
		`,
	},
	perfsql.CockroachDBDialect: {},
}

// Git is the minimal functionality Perf needs to interface to Git.
type Git struct {
	// The path of the git executable.
	gitFullPath string

	instanceConfig *config.InstanceConfig

	// preparedStatements are all the prepared SQL statements.
	preparedStatements map[statement]*sql.Stmt
}

/*
This command:

  git rev-list HEAD ^6286eccdf042751401f54696ad38de9f6849284d --pretty=" %aN <%aE>%n%s%n%ct"

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

	// Might be relative to working dir.
	gitFullPath, err := exec.LookPath("git")
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	// So force it to be absolute.
	gitFullPath, err = filepath.Abs(gitFullPath)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	if _, err := os.Stat(instanceConfig.GitRepoConfig.Dir); os.IsNotExist(err) {
		cmd := exec.CommandContext(ctx, gitFullPath, "clone", instanceConfig.GitRepoConfig.URL, instanceConfig.GitRepoConfig.Dir)
		if err := cmd.Run(); err != nil {
			exerr := err.(*exec.ExitError)
			sklog.Errorf("Failed to clone repo: %s - %s", err, exerr.Stderr)
			return nil, skerr.Wrap(err)
		}
	} else {
		if err := pull(ctx, gitFullPath, instanceConfig.GitRepoConfig.Dir); err != nil {
			return nil, skerr.Wrap(err)
		}
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
		gitFullPath:        gitFullPath,
		preparedStatements: preparedStatements,
	}, nil
}

type parsedSingleCommit struct {
	gitHash string
	ts      int64
	author  string
	subject string
}

type parseGitRevLogStreamProcessSingleCommit func(commit parsedSingleCommit) error

/*
commit 6079a7810530025d9877916895dd14eb8bb454c0
 Joe Gregorio <joe@bitworking.org>
 Change #9
 1584837783
commit 977e0ef44bec17659faf8c5d4025c5a068354817
 Joe Gregorio <joe@bitworking.org>
 Change #8
 1584837783

*/
func parseGitRevLogStream(r io.ReadCloser, f parseGitRevLogStreamProcessSingleCommit) error {
	defer util.Close(r)
	scanner := bufio.NewScanner(r)
	lineNumber := 0
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "commit ") {
			return skerr.Fmt("Invalid format, expected commit at line %d: %q", lineNumber, line)
		}
		lineNumber++
		gitHash := strings.Split(line, " ")[1]

		if !scanner.Scan() {
			return skerr.Fmt("Ran out of input, expecting an author line: %d", lineNumber)
		}
		lineNumber++
		author := scanner.Text()

		if !scanner.Scan() {
			return skerr.Fmt("Ran out of input, expecting a subject line: %d", lineNumber)
		}
		lineNumber++
		subject := scanner.Text()

		if !scanner.Scan() {
			return skerr.Fmt("Ran out of input, expecting a timestamp line: %d", lineNumber)
		}
		lineNumber++
		timestampString := scanner.Text()
		ts, err := strconv.ParseInt(timestampString, 10, 64)
		if err != nil {
			return skerr.Fmt("Failed to parse timestamp %q at line %d", timestampString, lineNumber)
		}
		if err := f(parsedSingleCommit{gitHash, ts, author, subject}); err != nil {
			return skerr.Wrap(err)
		}
	}
	return scanner.Err()
}

func pull(ctx context.Context, gitFullPath, dir string) error {
	cmd := exec.CommandContext(ctx, gitFullPath, "pull")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		exerr := err.(*exec.ExitError)
		sklog.Errorf("Failed to pull repo: %s - %s", err, exerr.Stderr)
		return skerr.Wrap(err)
	}
	return nil
}

func (g *Git) startGitRevLogStream(ctx context.Context) error {
	// Git pull

	mostRecentGitHash, mostRecentCommitNumber, err := g.getMostRecentCommit(ctx)
	nextCommitNumber := mostRecentCommitNumber + 1
	var cmd *exec.Cmd
	if err != nil {
		if err == sql.ErrNoRows {
			cmd = exec.CommandContext(ctx, g.gitFullPath, "rev-list", "HEAD", `--pretty="%aN <%aE>%n%s%n%ct"`, "--reverse")
			nextCommitNumber = types.CommitNumber(0)
		} else {
			return skerr.Wrap(err)
		}
	} else {
		cmd = exec.CommandContext(ctx, g.gitFullPath, "rev-list", "HEAD", "^"+mostRecentGitHash, `--pretty="%aN <%aE>%n%s%n%ct"`, "--reverse")
	}
	cmd.Dir = g.instanceConfig.GitRepoConfig.Dir

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return skerr.Wrap(err)
	}
	if err := cmd.Start(); err != nil {
		return skerr.Wrap(err)
	}

	err = parseGitRevLogStream(stdout, func(p parsedSingleCommit) error {
		// Add p to the database starting at mostRecentCommitNumber.
		_, err := g.preparedStatements[insert].ExecContext(ctx, nextCommitNumber, p.gitHash, p.ts, p.author, p.subject)
		if err != nil {
			return skerr.Wrap(err)
		}
		nextCommitNumber++
		return nil
	})
	if err != nil {
		return skerr.Wrap(err)
	}

	if err := cmd.Wait(); err != nil {
		exerr := err.(*exec.ExitError)
		sklog.Errorf("Failed to pull repo: %s - %s", err, exerr.Stderr)
		return skerr.Wrap(err)
	}
	return nil
}

func (g *Git) getMostRecentCommit(ctx context.Context) (string, types.CommitNumber, error) {
	var gitHash string
	var commitNumber types.CommitNumber
	if err := g.preparedStatements[getMostRecentGitHashAndCommitNumber].QueryRowContext(ctx).Scan(&gitHash, &commitNumber); err != nil {
		// Don't wrap the err, we need to see if it's sql.ErrNoRows.
		return "", types.BadCommitNumber, err
	}
	return gitHash, commitNumber, nil
}

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
