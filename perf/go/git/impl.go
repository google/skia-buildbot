// Package git manages a cache of git commit info that's stored in the database.
package git

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"github.com/jackc/pgx/v4"
	"go.opencensus.io/trace"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sql/pool"
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
	insertSpanner
	getCommitNumberFromGitHash
	getCommitNumberFromTime
	getCommitsFromTimeRange
	getCommitsFromCommitNumberRange
	getCommitFromCommitNumber
	getHashFromCommitNumber
	getDetails
	getPreviousGitHashFromCommitNumber
	getPreviousCommitNumberFromCommitNumber
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
	insertSpanner: `INSERT INTO
			Commits (commit_number, git_hash, commit_time, author, subject)
		VALUES
			($1, $2, $3, $4, $5)
		ON CONFLICT (commit_number)
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
			AND commit_time <= $2
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
	getPreviousGitHashFromCommitNumber: `
		SELECT
			git_hash
		FROM
			Commits
		WHERE
			commit_number < $1
		ORDER BY
			commit_number DESC
		LIMIT
			1
		`,
	getPreviousCommitNumberFromCommitNumber: `
		SELECT
			commit_number
		FROM
			Commits
		WHERE
			commit_number < $1
		ORDER BY
			commit_number DESC
		LIMIT
			1
		`,
}

// Impl implements Git, the minimal functionality Perf needs to interface to
// a git repo.
//
// It stores a copy of the needed commit info in an SQL database for quicker
// access, and runs a background Go routine that updates the database
// periodically.
type Impl struct {
	gp provider.Provider

	instanceConfig *config.InstanceConfig

	db pool.Pool

	// cache for CommitFromCommitNumber.
	cache *lru.Cache

	repoSuppliedCommitNumber bool
	commitNumberRegex        *regexp.Regexp

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
	previousGitHashFromCommitNumberCalled                 metrics2.Counter
	previousCommitNumberFromCommitNumberCalled            metrics2.Counter
	commitNumberMissingFromGitLog                         metrics2.Counter
}

// New creates a new *Git from the given instance configuration.
//
// The instance created does not poll by default, callers need to call
// StartBackgroundPolling().
func New(ctx context.Context, localToProd bool, db pool.Pool, instanceConfig *config.InstanceConfig) (*Impl, error) {
	cache, err := lru.New(commitCacheSize)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	gp, err := providers.New(ctx, instanceConfig)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	// If the commit_number_regex config is not empty, will parse commit number from git hash field.
	commitNumberRegex := instanceConfig.GitRepoConfig.CommitNumberRegex
	repoSuppliedCommitNumber := false
	var regex *regexp.Regexp
	if len(commitNumberRegex) > 0 {
		repoSuppliedCommitNumber = true
		regex = regexp.MustCompile(commitNumberRegex)
	}

	ret := &Impl{
		gp:                                     gp,
		db:                                     db,
		cache:                                  cache,
		instanceConfig:                         instanceConfig,
		repoSuppliedCommitNumber:               repoSuppliedCommitNumber,
		commitNumberRegex:                      regex,
		updateCalled:                           metrics2.GetCounter("perf_git_update_called"),
		commitNumberFromGitHashCalled:          metrics2.GetCounter("perf_git_commit_number_from_githash_called"),
		commitNumberFromTimeCalled:             metrics2.GetCounter("perf_git_commit_number_from_time_called"),
		commitSliceFromCommitNumberSlice:       metrics2.GetCounter("perf_git_commits_slice_from_commit_number_slice_called"),
		commitSliceFromTimeRangeCalled:         metrics2.GetCounter("perf_git_commits_slice_from_time_range_called"),
		commitSliceFromCommitNumberRangeCalled: metrics2.GetCounter("perf_git_commits_slice_from_commit_number_range_called"),
		commitFromCommitNumberCalled:           metrics2.GetCounter("perf_git_commit_from_commit_number_called"),
		gitHashFromCommitNumberCalled:          metrics2.GetCounter("perf_git_githash_from_commit_number_called"),
		commitNumbersWhenFileChangesInCommitNumberRangeCalled: metrics2.GetCounter("perf_git_commit_numbers_when_file_changes_in_commit_number_range_called"),
		previousGitHashFromCommitNumberCalled:                 metrics2.GetCounter("perf_git_previous_githash_from_commit_number_called"),
		previousCommitNumberFromCommitNumberCalled:            metrics2.GetCounter("perf_git_previous_commit_number_from_commit_number_called"),
		commitNumberMissingFromGitLog:                         metrics2.GetCounter("perf_git_commit_number_missing_from_git_log"),
	}

	// If we are running a local instance against prod database, we do not want
	// to do any git updates.
	if !localToProd {
		if err := ret.Update(ctx); err != nil {
			return nil, skerr.Wrapf(err, "Failed first update step for config %v", *instanceConfig)
		}
	}

	return ret, nil
}

// StartBackgroundPolling implements Git.
func (g *Impl) StartBackgroundPolling(ctx context.Context, duration time.Duration) {
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

// Update implements Git.
func (g *Impl) Update(ctx context.Context) error {
	ctx, span := trace.StartSpan(ctx, "perfgit.Update")
	defer span.End()

	ctx, cancel := context.WithTimeout(ctx, config.QueryMaxRunTime)
	defer cancel()

	sklog.Infof("perfgit: Update called.")
	g.updateCalled.Inc(1)
	if err := g.gp.Update(ctx); err != nil {
		return skerr.Wrap(err)
	}

	nextCommitNumber := types.CommitNumber(0)
	mostRecentGitHash, mostRecentCommitNumber, err := g.getMostRecentCommit(ctx)
	if !g.repoSuppliedCommitNumber {
		nextCommitNumber = mostRecentCommitNumber + 1
	}

	if err != nil {
		// If the Commits table is empty then start populating it from the very
		// first commit to the repo.
		if err == pgx.ErrNoRows {
			mostRecentGitHash = ""
		} else {
			return skerr.Wrapf(err, "Failed looking up most recent commit.")
		}
	}

	total := 0
	sklog.Infof("Populating commits from %q to HEAD", mostRecentGitHash)
	return g.gp.CommitsFromMostRecentGitHashToHead(ctx, mostRecentGitHash, func(p provider.Commit) error {
		if g.repoSuppliedCommitNumber {
			nextCommitNumber, err = g.getCommitNumberFromCommit(p.Body)
			if err != nil {
				// Because of an old bug https://bugs.chromium.org/p/chromium/issues/detail?id=1108466
				// A few chromium commits don't have associated commit position in early 2020.
				// For example: https://chromium.googlesource.com/chromium/src/+/0584c1805c005f1aac35c28c6738d9a2455a876c
				// The issue has already been fixed, skips those bad commits to make the system more robust.
				g.commitNumberMissingFromGitLog.Inc(1)
				sklog.Errorf("Failed to insert commit %q into database, because cannot find commit number with the error: %s", p.GitHash, err)
				return nil
			}
		}

		// Check if the commit has already been inserted first. It is quite possible that
		// multiple services/threads are calling Update at the same time and considering
		// the git log api call can take a bit of time to return we can end up in
		// situations where multiple insertions of the same commit occur. Thus we check
		// first if the commit has already been inserted in the table first before adding
		// it. The commit_number PK column auto increments when it's not repo supplied,
		// hence the ON CONFLICT clause doesn't really provide protection.
		commitNumber, err := g.CommitNumberFromGitHash(ctx, p.GitHash)
		if err == nil && commitNumber != types.BadCommitNumber {
			sklog.Infof("Commit %s already present in the database.", p.GitHash)
			return nil
		}

		// Add p to the database starting at nextCommitNumber.
		insertStmt := insert
		if g.instanceConfig.DataStoreConfig.DataStoreType == config.SpannerDataStoreType {
			insertStmt = insertSpanner
		}
		_, err = g.db.Exec(ctx, statements[insertStmt], nextCommitNumber, p.GitHash, p.Timestamp, p.Author, p.Subject)
		if err != nil {
			return skerr.Wrapf(err, "Failed to insert commit %q into database.", p.GitHash)
		}
		if !g.repoSuppliedCommitNumber {
			nextCommitNumber++
		}
		total++
		if total < 10 || (total%100) == 0 {
			sklog.Infof("Added %d commits this update cycle.", total)
		}
		return nil

	})
}

// getCommitNumberFromCommit get commit number from commit body.
// For example, commit body is "... Cr-Commit-Position: refs/heads/master@{#727901}"
// commitNumberRegex is "Cr-Commit-Position: refs/heads/(main|master)@\\{#(.*)\\}"
// matchs[0] will be ["Cr-Commit-Position: refs/heads/master@{#727901}", "master", "727901"]
// matchs[0][0] will be "Cr-Commit-Position: refs/heads/master@{#727901}"
// matchs[0][1] will be "master"
// matchs[0][2] will be "727901"
func (g *Impl) getCommitNumberFromCommit(body string) (types.CommitNumber, error) {
	matchs := g.commitNumberRegex.FindAllStringSubmatch(body, -1)
	if len(matchs) <= 0 {
		return types.BadCommitNumber, skerr.Fmt("Failed to match commit number key by regex %q from commit body: %q", g.commitNumberRegex.String(), body)
	}

	match := matchs[len(matchs)-1]
	if len(match) < 3 {
		return types.BadCommitNumber, skerr.Fmt("Failed to match commit number by regex %q from commit body: %q", g.commitNumberRegex.String(), body)
	}

	result, err := strconv.Atoi(match[2])
	if err != nil {
		return types.BadCommitNumber, skerr.Wrapf(err, "Failed to parse commit number from commit body: %q", body)
	}

	return types.CommitNumber(result), nil
}

// getMostRecentCommit as seen in the database.
func (g *Impl) getMostRecentCommit(ctx context.Context) (string, types.CommitNumber, error) {
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

// GetCommitNumber implements Git.
func (g *Impl) GetCommitNumber(ctx context.Context, githash string, commitNumber types.CommitNumber) (types.CommitNumber, error) {
	if g.repoSuppliedCommitNumber {
		_, err := g.GitHashFromCommitNumber(ctx, commitNumber)
		if err != nil {
			return types.BadCommitNumber, err
		}
		return commitNumber, nil
	}

	return g.CommitNumberFromGitHash(ctx, githash)
}

// CommitNumberFromGitHash implements Git.
func (g *Impl) CommitNumberFromGitHash(ctx context.Context, githash string) (types.CommitNumber, error) {
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

// CommitFromCommitNumber implements Git.
func (g *Impl) CommitFromCommitNumber(ctx context.Context, commitNumber types.CommitNumber) (provider.Commit, error) {
	ctx, span := trace.StartSpan(ctx, "perfgit.CommitFromCommitNumber")
	defer span.End()

	g.commitFromCommitNumberCalled.Inc(1)
	if iCommit, ok := g.cache.Get(commitNumber); ok {
		return iCommit.(provider.Commit), nil
	}
	var ret provider.Commit
	if err := g.db.QueryRow(ctx, statements[getDetails], commitNumber).Scan(&ret.GitHash, &ret.Timestamp, &ret.Author, &ret.Subject); err != nil {
		if err != pgx.ErrNoRows {
			return ret, skerr.Wrapf(err, "Failed to get details for CommitNumber: %d", commitNumber)
		} else {
			return ret, err
		}

	}
	ret.CommitNumber = commitNumber
	ret.URL = urlFromParts(g.instanceConfig, ret)

	_ = g.cache.Add(commitNumber, ret)
	return ret, nil
}

// CommitSliceFromCommitNumberSlice implements Git.
func (g *Impl) CommitSliceFromCommitNumberSlice(ctx context.Context, commitNumberSlice []types.CommitNumber) ([]provider.Commit, error) {
	ctx, span := trace.StartSpan(ctx, "perfgit.CommitSliceFromCommitNumberSlice")
	defer span.End()

	g.commitSliceFromCommitNumberSlice.Inc(1)
	ret := make([]provider.Commit, len(commitNumberSlice))
	i := 0
	for _, commitNumber := range commitNumberSlice {
		details, err := g.CommitFromCommitNumber(ctx, commitNumber)
		if err != nil {
			if err == pgx.ErrNoRows {
				// If there are no commit entries for the given commit number, we can ignore.
				continue
			}
			return ret, skerr.Wrapf(err, "failed looking up CommitNumber %d", commitNumber)
		}
		ret[i] = details
		i++
	}

	return ret[:i], nil
}

// CommitNumberFromTime implements Git.
func (g *Impl) CommitNumberFromTime(ctx context.Context, t time.Time) (types.CommitNumber, error) {
	ctx, span := trace.StartSpan(ctx, "perfgit.CommitNumberFromTime")
	defer span.End()

	g.commitNumberFromTimeCalled.Inc(1)
	ret := types.BadCommitNumber

	if t.IsZero() {
		_, mostRecentCommitNumber, err := g.getMostRecentCommit(ctx)
		return mostRecentCommitNumber, err
	}

	if err := g.db.QueryRow(ctx, statements[getCommitNumberFromTime], t.Unix()).Scan(&ret); err != nil {
		if err == pgx.ErrNoRows {
			return types.BadCommitNumber, nil
		}
		return ret, skerr.Wrapf(err, "Failed get for time: %q", t)
	}
	return ret, nil
}

// CommitSliceFromTimeRange implements Git.
func (g *Impl) CommitSliceFromTimeRange(ctx context.Context, begin, end time.Time) ([]provider.Commit, error) {
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
		c.URL = urlFromParts(g.instanceConfig, c)
		ret = append(ret, c)
	}
	return ret, nil
}

// CommitSliceFromCommitNumberRange implements Git.
func (g *Impl) CommitSliceFromCommitNumberRange(ctx context.Context, begin, end types.CommitNumber) ([]provider.Commit, error) {
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

		c.URL = urlFromParts(g.instanceConfig, c)
		ret = append(ret, c)
	}
	return ret, nil
}

// GitHashFromCommitNumber implements Git.
func (g *Impl) GitHashFromCommitNumber(ctx context.Context, commitNumber types.CommitNumber) (string, error) {
	ctx, span := trace.StartSpan(ctx, "perfgit.GitHashFromCommitNumber")
	defer span.End()

	g.gitHashFromCommitNumberCalled.Inc(1)
	var ret string
	if err := g.db.QueryRow(ctx, statements[getHashFromCommitNumber], commitNumber).Scan(&ret); err != nil {
		sklog.Warningf("Failed to find git hash for commit number: %v", commitNumber)
		return "", skerr.Wrapf(err, "Failed to find git hash for commit number: %v", commitNumber)
	}
	return ret, nil
}

// PreviousGitHashFromCommitNumber implements Git.
func (g *Impl) PreviousGitHashFromCommitNumber(ctx context.Context, commitNumber types.CommitNumber) (string, error) {
	ctx, span := trace.StartSpan(ctx, "perfgit.PreviousGitHashFromCommitNumber")
	defer span.End()

	g.previousGitHashFromCommitNumberCalled.Inc(1)
	var ret string
	if err := g.db.QueryRow(ctx, statements[getPreviousGitHashFromCommitNumber], commitNumber).Scan(&ret); err != nil {
		return "", skerr.Wrapf(err, "Failed to find previous git hash for commit number: %v", commitNumber)
	}
	return ret, nil
}

// PreviousCommitNumberFromCommitNumber implements Git.
func (g *Impl) PreviousCommitNumberFromCommitNumber(ctx context.Context, commitNumber types.CommitNumber) (types.CommitNumber, error) {
	ctx, span := trace.StartSpan(ctx, "perfgit.PreviousCommitNumberFromCommitNumber")
	defer span.End()

	g.previousCommitNumberFromCommitNumberCalled.Inc(1)
	ret := types.BadCommitNumber
	if err := g.db.QueryRow(ctx, statements[getPreviousCommitNumberFromCommitNumber], commitNumber).Scan(&ret); err != nil {
		return ret, skerr.Wrapf(err, "Failed to find previous commit number for commit number: %v", commitNumber)
	}
	return ret, nil
}

// CommitNumbersWhenFileChangesInCommitNumberRange implements Git.
func (g *Impl) CommitNumbersWhenFileChangesInCommitNumberRange(ctx context.Context, begin, end types.CommitNumber, filename string) ([]types.CommitNumber, error) {
	ctx, span := trace.StartSpan(ctx, "perfgit.CommitNumbersWhenFileChangesInCommitNumberRange")
	defer span.End()

	g.commitNumbersWhenFileChangesInCommitNumberRangeCalled.Inc(1)
	// Default to beginHash being the empty string, which means start at the
	// beginning of the repo's history.
	var beginHash string
	// Covert the commit numbers to hashes.
	if begin != types.BadCommitNumber && begin-1 != types.BadCommitNumber {
		var err error
		beginHash, err = g.PreviousGitHashFromCommitNumber(ctx, begin)
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

// LogEntry implements Git.
func (g *Impl) LogEntry(ctx context.Context, commit types.CommitNumber) (string, error) {
	hash, err := g.GitHashFromCommitNumber(ctx, commit)
	if err != nil {
		return "", skerr.Wrap(err)
	}
	return g.gp.LogEntry(ctx, hash)
}

// RepoSuppliedCommitNumber implements Git.
func (g *Impl) RepoSuppliedCommitNumber() bool {
	return g.repoSuppliedCommitNumber
}
