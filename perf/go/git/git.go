// Package git is the minimal interface that Perf needs to interact with a Git
// repo.
//
// A cache of git information is kept in an SQL database. Please see
// perf/sql/migrations for the database schema used.
package git

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/gitauth"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/human"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/types"
)

// For rough numbers a Commit Author is 50 , Subject 80 , URL 200, and GitHash 32 bytes. So
// so rounding up about 400 bytes per Commit. If we want to cap the lru cache at 10MB that's
// 25,000 entries.
const commitCacheSize = 25_000

// Commit represents a single commit stored in the database.
//
// JSON annotations make it serialize like the legacy cid.CommitDetail.
type Commit struct {
	CommitNumber types.CommitNumber `json:"offset"`
	GitHash      string             `json:"hash"`
	Timestamp    int64              `json:"ts"` // Unix timestamp, seconds from the epoch.
	Author       string             `json:"author"`
	Subject      string             `json:"message"`
	URL          string             `json:"url"`
}

// Display returns a display string that describes the commit.
func (c Commit) Display(now time.Time) string {
	return fmt.Sprintf("%s - %s - %s", c.GitHash[:7], human.Duration(now.Sub(time.Unix(c.Timestamp, 0))), c.Subject)
}

// statement is an SQL statement identifier.
type statement int

const (
	// The identifiers for all the SQL statements used.
	getMostRecentGitHashAndCommitNumber statement = iota
	insert
	getCommitNumberFromGitHash
	getCommitNumberFromTime
	getCommitsFromTimeRange
	getCommitsFromCommitNumberRange
	getCommitFromCommitNumber
	getHashFromCommitNumber
	getDetails
)

var (
	// BadCommit is returned on errors from functions that return Commits.
	BadCommit = Commit{
		CommitNumber: types.BadCommitNumber,
	}
)

// statements holds all the raw SQL statemens used per Dialect of SQL.
var statements = map[statement]string{
	getDetails: `
		SELECT
			git_hash, commit_time, author, subject
		FROM
			Commits
		WHERE
			commit_number=$1
	`,
	getMostRecentGitHashAndCommitNumber: `
		SELECT
			git_hash, commit_number
		FROM
			Commits
		ORDER BY
			commit_number DESC
		LIMIT
			1
		`,
	insert: `
		INSERT INTO
			Commits (commit_number, git_hash, commit_time, author, subject)
		VALUES
			($1, $2, $3, $4, $5)
		ON CONFLICT
		DO NOTHING
		`,
	getCommitNumberFromGitHash: `
		SELECT
			commit_number
		FROM
			Commits
		WHERE
			git_hash=$1`,
	getCommitNumberFromTime: `
		SELECT
			commit_number
		FROM
			Commits
		WHERE
			commit_time <= $1
		ORDER BY
			commit_number DESC
		LIMIT
			1
		`,
	getCommitsFromTimeRange: `
		SELECT
			commit_number, git_hash, commit_time, author, subject
		FROM
			Commits
		WHERE
			commit_time >= $1
			AND commit_time < $2
		ORDER BY
			commit_number ASC
		`,
	getCommitsFromCommitNumberRange: `
		SELECT
			commit_number, git_hash, commit_time, author, subject
		FROM
			Commits
		WHERE
			commit_number >= $1
			AND commit_number <= $2
		ORDER BY
			commit_number ASC
		`,
	getCommitFromCommitNumber: `
		SELECT
			commit_number, git_hash, commit_time, author, subject
		FROM
			Commits
		WHERE
			commit_number = $1
		`,
	getHashFromCommitNumber: `
		SELECT
			git_hash
		FROM
			Commits
		WHERE
			commit_number=$1
		`,
}

// Git implements the minimal functionality Perf needs to interface to Git.
//
// It stores a copy of the needed commit info in an SQL database for quicker
// access, and runs a background Go routine that updates the database
// periodically.
//
// Please see perf/sql/migrations for the database schema used.
type Git struct {
	// gitFullPath is the path of the git executable.
	gitFullPath string

	instanceConfig *config.InstanceConfig

	db *pgxpool.Pool

	// cache for CommitFromCommitNumber.
	cache *lru.Cache

	// Metrics
	updateCalled                                          metrics2.Counter
	commitNumberFromGitHashCalled                         metrics2.Counter
	commitNumberFromTimeCalled                            metrics2.Counter
	commitSliceFromCommitNumberSlice                      metrics2.Counter
	commitSliceFromTimeRangeCalled                        metrics2.Counter
	commitSliceFromCommitNumberRangeCalled                metrics2.Counter
	commitFromCommitNumberCalled                          metrics2.Counter
	gitHashFromCommitNumberCalled                         metrics2.Counter
	commitNumbersWhenFileChangesInCommitNumberRangeCalled metrics2.Counter
}

// New creates a new *Git from the given instance configuration.
//
// The instance created does not poll by default, callers need to call
// StartBackgroundPolling().
func New(ctx context.Context, local bool, db *pgxpool.Pool, instanceConfig *config.InstanceConfig) (*Git, error) {
	// Do git authentication if required.
	if instanceConfig.GitRepoConfig.GitAuthType == config.GitAuthGerrit {
		sklog.Info("Authenticating to Gerrit.")
		ts, err := auth.NewDefaultTokenSource(local, auth.SCOPE_GERRIT)
		if err != nil {
			return nil, skerr.Wrapf(err, "Failed to get tokensource perfgit.Git for config %v", *instanceConfig)
		}
		if _, err := gitauth.New(ts, "/tmp/git-cookie", true, ""); err != nil {
			return nil, skerr.Wrapf(err, "Failed to gitauth perfgit.Git for config %v", *instanceConfig)
		}
	}

	// Find the path to the git executable, which might be relative to working dir.
	gitFullPath, err := exec.LookPath("git")
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to find git.")
	}

	// Force the path to be absolute.
	gitFullPath, err = filepath.Abs(gitFullPath)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to get absolute path to git.")
	}

	// Clone the git repo if necessary.
	sklog.Infof("Cloning repo.")
	if _, err := os.Stat(instanceConfig.GitRepoConfig.Dir); os.IsNotExist(err) {
		cmd := exec.CommandContext(ctx, gitFullPath, "clone", instanceConfig.GitRepoConfig.URL, instanceConfig.GitRepoConfig.Dir)
		if err := cmd.Run(); err != nil {
			exerr := err.(*exec.ExitError)
			return nil, skerr.Wrapf(err, "Failed to clone repo: %s - %s", err, exerr.Stderr)
		}
	}

	cache, err := lru.New(commitCacheSize)

	ret := &Git{
		gitFullPath:                            gitFullPath,
		db:                                     db,
		cache:                                  cache,
		instanceConfig:                         instanceConfig,
		updateCalled:                           metrics2.GetCounter("perf_git_update_called"),
		commitNumberFromGitHashCalled:          metrics2.GetCounter("perf_git_commit_number_from_githash_called"),
		commitNumberFromTimeCalled:             metrics2.GetCounter("perf_git_commit_number_from_time_called"),
		commitSliceFromCommitNumberSlice:       metrics2.GetCounter("perf_git_commits_slice_from_commit_number_slice_called"),
		commitSliceFromTimeRangeCalled:         metrics2.GetCounter("perf_git_commits_slice_from_time_range_called"),
		commitSliceFromCommitNumberRangeCalled: metrics2.GetCounter("perf_git_commits_slice_from_commit_number_range_called"),
		commitFromCommitNumberCalled:           metrics2.GetCounter("perf_git_commit_from_commit_number_called"),
		gitHashFromCommitNumberCalled:          metrics2.GetCounter("perf_git_githash_from_commit_number_called"),
		commitNumbersWhenFileChangesInCommitNumberRangeCalled: metrics2.GetCounter("perf_git_commit_numbers_when_file_changes_in_commit_number_range_called"),
	}

	if err := ret.Update(ctx); err != nil {
		return nil, skerr.Wrapf(err, "Failed first update step for config %v", *instanceConfig)
	}

	return ret, nil
}

// StartBackgroundPolling starts a background process that periodically pulls to
// head and adds the new commits to the database.
func (g *Git) StartBackgroundPolling(ctx context.Context, duration time.Duration) {
	go func() {
		liveness := metrics2.NewLiveness("perf_git_udpate_polling_livenes")
		for range time.Tick(duration) {
			timeoutCtx, cancel := context.WithTimeout(ctx, duration)
			defer cancel()
			if err := g.Update(timeoutCtx); err != nil {
				sklog.Errorf("Failed to update git repo: %s", err)
			} else {
				liveness.Reset()
			}
		}
	}()
}

type parseGitRevLogStreamProcessSingleCommit func(commit Commit) error

// parseGitRevLogStream parses the input stream for input of the form:
//
//     commit 6079a7810530025d9877916895dd14eb8bb454c0
//     Joe Gregorio <joe@bitworking.org>
//     Change #9
//     1584837783
//     commit 977e0ef44bec17659faf8c5d4025c5a068354817
//     Joe Gregorio <joe@bitworking.org>
//     Change #8
//     1584837783
//
// And calls the parseGitRevLogStreamProcessSingleCommit function with each
// entry it finds. The passed in Commit has all valid fields except
// CommitNumber, which is set to types.BadCommitNumber.
func parseGitRevLogStream(r io.ReadCloser, f parseGitRevLogStreamProcessSingleCommit) error {
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
		if err := f(Commit{
			CommitNumber: types.BadCommitNumber,
			GitHash:      gitHash,
			Timestamp:    ts,
			Author:       author,
			Subject:      subject}); err != nil {
			return skerr.Wrap(err)
		}
	}
	return skerr.Wrap(scanner.Err())
}

// pull does a git pull on the git repo.
func pull(ctx context.Context, gitFullPath, dir string) error {
	cmd := exec.CommandContext(ctx, gitFullPath, "pull")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		exerr := err.(*exec.ExitError)
		return skerr.Wrapf(err, "Failed to pull repo %q with git %q: %s", dir, gitFullPath, exerr.Stderr)
	}
	return nil
}

// Update does a git pull and then finds all the new commits
// added to the repo since our last Update.
//
// This command will list all new commits since 6286e... in chronological
// order.
//
//     git rev-list HEAD ^6286e.. --pretty=" %aN <%aE>%n%s%n%ct" --reverse
//
// It produces the following output of the form:
//
//     commit 6079a7810530025d9877916895dd14eb8bb454c0
//     Joe Gregorio <joe@bitworking.org>
//     Change #9
//     1584837783
//     commit 977e0ef44bec17659faf8c5d4025c5a068354817
//     Joe Gregorio <joe@bitworking.org>
//     Change #8
//     1584837783
//
// which parseGitRevLogStream parses.
//
// Note also that CommitNumber starts at 0 for the first commit in a repo.
func (g *Git) Update(ctx context.Context) error {
	sklog.Infof("perfgit: Update called.")
	g.updateCalled.Inc(1)
	if err := pull(ctx, g.gitFullPath, g.instanceConfig.GitRepoConfig.Dir); err != nil {
		return skerr.Wrap(err)
	}
	var cmd *exec.Cmd
	mostRecentGitHash, mostRecentCommitNumber, err := g.getMostRecentCommit(ctx)
	nextCommitNumber := mostRecentCommitNumber + 1
	if err != nil {
		// If the Commits table is empty then start populating it from the very
		// first commit to the repo.
		if err == pgx.ErrNoRows {
			cmd = exec.CommandContext(ctx, g.gitFullPath, "rev-list", "HEAD", `--pretty=%aN <%aE>%n%s%n%ct`, "--reverse")
			nextCommitNumber = types.CommitNumber(0)
		} else {
			return skerr.Wrapf(err, "Failed looking up most recect commit.")
		}
	} else {
		// Add all the commits from the repo since the last time we looked.
		cmd = exec.CommandContext(ctx, g.gitFullPath, "rev-list", "HEAD", "^"+mostRecentGitHash, `--pretty=%aN <%aE>%n%s%n%ct`, "--reverse")
	}
	sklog.Infof("perfgit: Starting update with nextCommitNumber: %d", nextCommitNumber)
	cmd.Dir = g.instanceConfig.GitRepoConfig.Dir

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return skerr.Wrap(err)
	}
	if err := cmd.Start(); err != nil {
		return skerr.Wrap(err)
	}

	total := 0
	err = parseGitRevLogStream(stdout, func(p Commit) error {
		// Add p to the database starting at nextCommitNumber.
		_, err := g.db.Exec(ctx, statements[insert], nextCommitNumber, p.GitHash, p.Timestamp, p.Author, p.Subject)
		if err != nil {
			return skerr.Wrapf(err, "Failed to insert commit %q into database.", p.GitHash)
		}
		nextCommitNumber++
		total++
		if total < 10 || (total%100) == 0 {
			sklog.Infof("Added %d commits this update cycle.", total)
		}
		return nil
	})
	if err != nil {
		// Once we've successfully called cmd.Start() we must always call
		// cmd.Wait() to close stdout.
		_ = cmd.Wait()
		return skerr.Wrap(err)
	}

	if err := cmd.Wait(); err != nil {
		exerr := err.(*exec.ExitError)
		return skerr.Wrapf(err, "Failed to pull repo: %s", exerr.Stderr)
	}
	return nil
}

// getMostRecentCommit as seen in the database.
func (g *Git) getMostRecentCommit(ctx context.Context) (string, types.CommitNumber, error) {
	var gitHash string
	var commitNumber types.CommitNumber
	if err := g.db.QueryRow(ctx, statements[getMostRecentGitHashAndCommitNumber]).Scan(&gitHash, &commitNumber); err != nil {
		// Don't wrap the err, we need to see if it's sql.ErrNoRows.
		return "", types.BadCommitNumber, err
	}
	return gitHash, commitNumber, nil
}

// CommitNumberFromGitHash looks up the commit number given the git hash.
func (g *Git) CommitNumberFromGitHash(ctx context.Context, githash string) (types.CommitNumber, error) {
	g.commitNumberFromGitHashCalled.Inc(1)
	ret := types.BadCommitNumber
	if err := g.db.QueryRow(ctx, statements[getCommitNumberFromGitHash], githash).Scan(&ret); err != nil {
		return ret, skerr.Wrapf(err, "Failed get for hash: %q", githash)
	}
	return ret, nil
}

// urlFromParts creates the URL to link to a specific commit in a repo.
func urlFromParts(instanceConfig *config.InstanceConfig, commit Commit) string {
	if instanceConfig.GitRepoConfig.DebouceCommitURL {
		return commit.Subject
	}

	format := instanceConfig.GitRepoConfig.CommitURL
	if format == "" {
		format = gitiles.CommitURL
	}
	return fmt.Sprintf(format, instanceConfig.GitRepoConfig.URL, commit.GitHash)
}

// CommitFromCommitNumber returns all the stored details for a given CommitNumber.
func (g *Git) CommitFromCommitNumber(ctx context.Context, commitNumber types.CommitNumber) (Commit, error) {
	g.commitFromCommitNumberCalled.Inc(1)
	if iCommit, ok := g.cache.Get(commitNumber); ok {
		return iCommit.(Commit), nil
	}
	var ret Commit
	if err := g.db.QueryRow(ctx, statements[getDetails], commitNumber).Scan(&ret.GitHash, &ret.Timestamp, &ret.Author, &ret.Subject); err != nil {
		return ret, skerr.Wrapf(err, "Failed to get details for CommitNumber: %d", commitNumber)
	}
	ret.CommitNumber = commitNumber
	ret.URL = urlFromParts(g.instanceConfig, ret)

	_ = g.cache.Add(commitNumber, ret)
	return ret, nil
}

// CommitSliceFromCommitNumberSlice returns all the stored details for a given slice of CommitNumbers.
func (g *Git) CommitSliceFromCommitNumberSlice(ctx context.Context, commitNumberSlice []types.CommitNumber) ([]Commit, error) {
	g.commitSliceFromCommitNumberSlice.Inc(1)
	ret := make([]Commit, len(commitNumberSlice))
	for i, commitNumber := range commitNumberSlice {
		details, err := g.CommitFromCommitNumber(ctx, commitNumber)
		if err != nil {
			return ret, skerr.Wrapf(err, "failed looking up CommitNumber %d", commitNumber)

		}
		ret[i] = details
	}
	return ret, nil
}

// CommitNumberFromTime finds the index of the closest commit with a commit time
// less than or equal to 't'.
//
// Pass in zero time, i.e. time.Time{} to indicate to just get the most recent
// commit.
func (g *Git) CommitNumberFromTime(ctx context.Context, t time.Time) (types.CommitNumber, error) {
	g.commitNumberFromTimeCalled.Inc(1)
	ret := types.BadCommitNumber

	if t.IsZero() {
		_, mostRecentCommitNumber, err := g.getMostRecentCommit(ctx)
		return mostRecentCommitNumber, err
	}
	if err := g.db.QueryRow(ctx, statements[getCommitNumberFromTime], t.Unix()).Scan(&ret); err != nil {
		return ret, skerr.Wrapf(err, "Failed get for time: %q", t)
	}
	return ret, nil
}

// CommitSliceFromTimeRange returns a slice of Commits that fall in the range
// [begin, end), i.e  inclusive of begin and exclusive of end.
func (g *Git) CommitSliceFromTimeRange(ctx context.Context, begin, end time.Time) ([]Commit, error) {
	g.commitSliceFromTimeRangeCalled.Inc(1)
	rows, err := g.db.Query(ctx, statements[getCommitsFromTimeRange], begin.Unix(), end.Unix())
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to query for commit slice in range %s-%s", begin, end)
	}
	ret := []Commit{}
	for rows.Next() {
		var c Commit
		if err := rows.Scan(&c.CommitNumber, &c.GitHash, &c.Timestamp, &c.Author, &c.Subject); err != nil {
			return nil, skerr.Wrapf(err, "Failed to read row in range %s-%s", begin, end)
		}
		ret = append(ret, c)
	}
	return ret, nil
}

// CommitSliceFromCommitNumberRange returns a slice of Commits that fall in the range
// [begin, end], i.e  inclusive of both begin and end.
func (g *Git) CommitSliceFromCommitNumberRange(ctx context.Context, begin, end types.CommitNumber) ([]Commit, error) {
	g.commitSliceFromCommitNumberRangeCalled.Inc(1)
	rows, err := g.db.Query(ctx, statements[getCommitsFromCommitNumberRange], begin, end)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to query for commit slice in range %v-%v", begin, end)
	}
	ret := []Commit{}
	for rows.Next() {
		var c Commit
		if err := rows.Scan(&c.CommitNumber, &c.GitHash, &c.Timestamp, &c.Author, &c.Subject); err != nil {
			return nil, skerr.Wrapf(err, "Failed to read row in range %v-%v", begin, end)
		}
		ret = append(ret, c)
	}
	return ret, nil
}

// GitHashFromCommitNumber returns the git hash of the given commit number.
func (g *Git) GitHashFromCommitNumber(ctx context.Context, commitNumber types.CommitNumber) (string, error) {
	g.gitHashFromCommitNumberCalled.Inc(1)
	var ret string
	if err := g.db.QueryRow(ctx, statements[getHashFromCommitNumber], commitNumber).Scan(&ret); err != nil {
		return "", skerr.Wrapf(err, "Failed to find git hash for commit number: %v", commitNumber)
	}
	return ret, nil
}

// CommitNumbersWhenFileChangesInCommitNumberRange returns a slice of commit
// numbers when the given file has changed between [begin, end], i.e. the given
// range is exclusive of the begin commit and inclusive of the end commit.
func (g *Git) CommitNumbersWhenFileChangesInCommitNumberRange(ctx context.Context, begin, end types.CommitNumber, filename string) ([]types.CommitNumber, error) {
	g.commitNumbersWhenFileChangesInCommitNumberRangeCalled.Inc(1)
	var revisionRange string
	endHash, err := g.GitHashFromCommitNumber(ctx, end)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if begin == types.CommitNumber(0) {
		// git log revision range queries of the form hash1..hash2 are exclusive
		// of hash1, so we need to always back up begin one commit, except in
		// the case where the commit number is 0, then we change the revision
		// range.
		revisionRange = endHash
	} else {
		// Covert the commit numbers to hashes.
		beginHash, err := g.GitHashFromCommitNumber(ctx, begin-1)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		revisionRange = beginHash + ".." + endHash
	}

	// Build the git log command to run.
	cmd := exec.CommandContext(ctx, g.gitFullPath, "log", revisionRange, "--reverse", "--format=format:%H", "--", filename)
	cmd.Dir = g.instanceConfig.GitRepoConfig.Dir

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if err := cmd.Start(); err != nil {
		return nil, skerr.Wrap(err)
	}

	// Read the git log output.
	scanner := bufio.NewScanner(stdout)
	ret := []types.CommitNumber{}
	for scanner.Scan() {
		githash := scanner.Text()
		commitNumber, err := g.CommitNumberFromGitHash(ctx, githash)
		if err != nil {
			return nil, skerr.Wrapf(err, "git log returned invalid git hash: %q", githash)
		}
		ret = append(ret, commitNumber)
	}

	if scanner.Err() != nil {
		// Once we've successfully called cmd.Start() we must always call
		// cmd.Wait() to close stdout.
		_ = cmd.Wait()
		return nil, skerr.Wrap(err)
	}

	if err := cmd.Wait(); err != nil {
		exerr := err.(*exec.ExitError)
		return nil, skerr.Wrapf(err, "Failed to get logs: %s", exerr.Stderr)
	}

	return ret, nil
}
