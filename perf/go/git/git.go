// Package git is the minimal interface that Perf needs to interact with a Git
// repo.
//
// A cache of git information is kept in an SQL database. Please see
// perf/sql/migrations for the database schema used.
package git

import (
	"context"
	"fmt"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"go.opencensus.io/trace"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/git/provider"
	"go.skia.org/infra/perf/go/git/providers"
	"go.skia.org/infra/perf/go/types"
)

// For rough numbers a Commit Author is 50 , Subject 80 , URL 200, and GitHash 32 bytes. So
// so rounding up about 400 bytes per Commit. If we want to cap the lru cache at 10MB that's
// 25,000 entries.
const commitCacheSize = 25_000

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
	BadCommit = provider.Commit{
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
	gp provider.Provider

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
	cache, err := lru.New(commitCacheSize)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	gp, err := providers.New(ctx, instanceConfig)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	ret := &Git{
		gp:                                     gp,
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
		ctx := context.Background()
		for range time.Tick(duration) {
			if err := g.Update(ctx); err != nil {
				sklog.Errorf("Failed to update git repo: %s", err)
			} else {
				liveness.Reset()
			}
		}
	}()
}

// Update does a git pull and then finds all the new commits
// added to the repo since our last Update.
//
// This command will list all new commits since 6286e... in chronological
// order.
//
//	git rev-list HEAD ^6286e.. --pretty=" %aN <%aE>%n%s%n%ct" --reverse
//
// It produces the following output of the form:
//
//	commit 6079a7810530025d9877916895dd14eb8bb454c0
//	Joe Gregorio <joe@bitworking.org>
//	Change #9
//	1584837783
//	commit 977e0ef44bec17659faf8c5d4025c5a068354817
//	Joe Gregorio <joe@bitworking.org>
//	Change #8
//	1584837783
//
// which parseGitRevLogStream parses.
//
// Note also that CommitNumber starts at 0 for the first commit in a repo.
func (g *Git) Update(ctx context.Context) error {
	ctx, span := trace.StartSpan(ctx, "perfgit.Update")
	defer span.End()

	sklog.Infof("perfgit: Update called.")
	g.updateCalled.Inc(1)
	if err := g.gp.Update(ctx); err != nil {
		return skerr.Wrap(err)
	}
	mostRecentGitHash, mostRecentCommitNumber, err := g.getMostRecentCommit(ctx)
	nextCommitNumber := mostRecentCommitNumber + 1
	if err != nil {
		// If the Commits table is empty then start populating it from the very
		// first commit to the repo.
		if err == pgx.ErrNoRows {
			mostRecentGitHash = ""
			nextCommitNumber = types.CommitNumber(0)
		} else {
			return skerr.Wrapf(err, "Failed looking up most recect commit.")
		}
	}

	total := 0
	return g.gp.CommitsFromMostRecentGitHashToHead(ctx, mostRecentGitHash, func(p provider.Commit) error {
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
}

// getMostRecentCommit as seen in the database.
func (g *Git) getMostRecentCommit(ctx context.Context) (string, types.CommitNumber, error) {
	ctx, span := trace.StartSpan(ctx, "perfgit.getMostRecentCommit")
	defer span.End()

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
	ctx, span := trace.StartSpan(ctx, "perfgit.CommitNumberFromGitHash")
	defer span.End()

	g.commitNumberFromGitHashCalled.Inc(1)
	ret := types.BadCommitNumber
	if err := g.db.QueryRow(ctx, statements[getCommitNumberFromGitHash], githash).Scan(&ret); err != nil {
		return ret, skerr.Wrapf(err, "Failed get for hash: %q", githash)
	}
	return ret, nil
}

// urlFromParts creates the URL to link to a specific commit in a repo.
func urlFromParts(instanceConfig *config.InstanceConfig, commit provider.Commit) string {
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
func (g *Git) CommitFromCommitNumber(ctx context.Context, commitNumber types.CommitNumber) (provider.Commit, error) {
	ctx, span := trace.StartSpan(ctx, "perfgit.CommitFromCommitNumber")
	defer span.End()

	g.commitFromCommitNumberCalled.Inc(1)
	if iCommit, ok := g.cache.Get(commitNumber); ok {
		return iCommit.(provider.Commit), nil
	}
	var ret provider.Commit
	if err := g.db.QueryRow(ctx, statements[getDetails], commitNumber).Scan(&ret.GitHash, &ret.Timestamp, &ret.Author, &ret.Subject); err != nil {
		return ret, skerr.Wrapf(err, "Failed to get details for CommitNumber: %d", commitNumber)
	}
	ret.CommitNumber = commitNumber
	ret.URL = urlFromParts(g.instanceConfig, ret)

	_ = g.cache.Add(commitNumber, ret)
	return ret, nil
}

// CommitSliceFromCommitNumberSlice returns all the stored details for a given slice of CommitNumbers.
func (g *Git) CommitSliceFromCommitNumberSlice(ctx context.Context, commitNumberSlice []types.CommitNumber) ([]provider.Commit, error) {
	ctx, span := trace.StartSpan(ctx, "perfgit.CommitSliceFromCommitNumberSlice")
	defer span.End()

	g.commitSliceFromCommitNumberSlice.Inc(1)
	ret := make([]provider.Commit, len(commitNumberSlice))
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
	ctx, span := trace.StartSpan(ctx, "perfgit.CommitNumberFromTime")
	defer span.End()

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
func (g *Git) CommitSliceFromTimeRange(ctx context.Context, begin, end time.Time) ([]provider.Commit, error) {
	ctx, span := trace.StartSpan(ctx, "perfgit.CommitSliceFromTimeRange")
	defer span.End()

	g.commitSliceFromTimeRangeCalled.Inc(1)
	rows, err := g.db.Query(ctx, statements[getCommitsFromTimeRange], begin.Unix(), end.Unix())
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to query for commit slice in range %s-%s", begin, end)
	}
	defer rows.Close()
	ret := []provider.Commit{}
	for rows.Next() {
		var c provider.Commit
		if err := rows.Scan(&c.CommitNumber, &c.GitHash, &c.Timestamp, &c.Author, &c.Subject); err != nil {
			return nil, skerr.Wrapf(err, "Failed to read row in range %s-%s", begin, end)
		}
		ret = append(ret, c)
	}
	return ret, nil
}

// CommitSliceFromCommitNumberRange returns a slice of Commits that fall in the range
// [begin, end], i.e  inclusive of both begin and end.
func (g *Git) CommitSliceFromCommitNumberRange(ctx context.Context, begin, end types.CommitNumber) ([]provider.Commit, error) {
	ctx, span := trace.StartSpan(ctx, "perfgit.CommitSliceFromCommitNumberRange")
	defer span.End()

	g.commitSliceFromCommitNumberRangeCalled.Inc(1)
	rows, err := g.db.Query(ctx, statements[getCommitsFromCommitNumberRange], begin, end)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to query for commit slice in range %v-%v", begin, end)
	}
	defer rows.Close()
	ret := []provider.Commit{}
	for rows.Next() {
		var c provider.Commit
		if err := rows.Scan(&c.CommitNumber, &c.GitHash, &c.Timestamp, &c.Author, &c.Subject); err != nil {
			return nil, skerr.Wrapf(err, "Failed to read row in range %v-%v", begin, end)
		}
		ret = append(ret, c)
	}
	return ret, nil
}

// GitHashFromCommitNumber returns the git hash of the given commit number.
func (g *Git) GitHashFromCommitNumber(ctx context.Context, commitNumber types.CommitNumber) (string, error) {
	ctx, span := trace.StartSpan(ctx, "perfgit.GitHashFromCommitNumber")
	defer span.End()

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
	ctx, span := trace.StartSpan(ctx, "perfgit.CommitNumbersWhenFileChangesInCommitNumberRange")
	defer span.End()

	g.commitNumbersWhenFileChangesInCommitNumberRangeCalled.Inc(1)
	// Default to beginHash being the empty string, which means start at the
	// beginning of the repo's history.
	var beginHash string
	// Covert the commit numbers to hashes.
	if begin != types.BadCommitNumber && begin-1 != types.BadCommitNumber {
		var err error
		beginHash, err = g.GitHashFromCommitNumber(ctx, begin-1)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
	}

	endHash, err := g.GitHashFromCommitNumber(ctx, end)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	hashes, err := g.gp.GitHashesInRangeForFile(ctx, beginHash, endHash, filename)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	var ret []types.CommitNumber
	for _, githash := range hashes {
		commitNumber, err := g.CommitNumberFromGitHash(ctx, githash)
		if err != nil {
			return nil, skerr.Wrapf(err, "git log returned invalid git hash: %q", githash)
		}
		ret = append(ret, commitNumber)
	}

	return ret, nil
}

// LogEntry returns the full log entry of a commit (minus the diff) as a string.
func (g *Git) LogEntry(ctx context.Context, commit types.CommitNumber) (string, error) {
	hash, err := g.GitHashFromCommitNumber(ctx, commit)
	if err != nil {
		return "", skerr.Wrap(err)
	}
	return g.gp.LogEntry(ctx, hash)
}
