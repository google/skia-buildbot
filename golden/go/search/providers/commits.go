package providers

import (
	"context"
	"strconv"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"github.com/jackc/pgx/v4/pgxpool"
	"go.opencensus.io/trace"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/golden/go/search/common"
	"go.skia.org/infra/golden/go/sql/schema"
	"go.skia.org/infra/golden/go/web/frontend"
)

const (
	commitCacheSize          = 5_000
	optionsGroupingCacheSize = 50_000
)

// CommitsProvider provides a struct for retrieving commit related data.
type CommitsProvider struct {
	db           *pgxpool.Pool
	commitCache  *lru.Cache
	windowLength int
}

// NewCommitsProvider returns a new instance of CommitsProvider.
func NewCommitsProvider(db *pgxpool.Pool, windowLength int) *CommitsProvider {
	cc, err := lru.New(commitCacheSize)
	if err != nil {
		panic(err) // should only happen if commitCacheSize is negative.
	}
	return &CommitsProvider{
		db:           db,
		commitCache:  cc,
		windowLength: windowLength,
	}
}

// GetCommits returns the front-end friendly version of the commits within the searched window.
func (s *CommitsProvider) GetCommits(ctx context.Context) ([]frontend.Commit, error) {
	ctx, span := trace.StartSpan(ctx, "getCommits")
	defer span.End()
	rv := make([]frontend.Commit, common.GetActualWindowLength(ctx))
	commitIDs := common.GetCommitToIdxMap(ctx)
	for commitID, idx := range commitIDs {
		var commit frontend.Commit
		if c, ok := s.commitCache.Get(commitID); ok {
			commit = c.(frontend.Commit)
		} else {
			if isStandardGitCommitID(commitID) {
				const statement = `SELECT git_hash, commit_time, author_email, subject
FROM GitCommits WHERE commit_id = $1`
				row := s.db.QueryRow(ctx, statement, commitID)
				var dbRow schema.GitCommitRow
				if err := row.Scan(&dbRow.GitHash, &dbRow.CommitTime, &dbRow.AuthorEmail, &dbRow.Subject); err != nil {
					return nil, skerr.Wrap(err)
				}
				commit = frontend.Commit{
					CommitTime: dbRow.CommitTime.UTC().Unix(),
					ID:         string(commitID),
					Hash:       dbRow.GitHash,
					Author:     dbRow.AuthorEmail,
					Subject:    dbRow.Subject,
				}
				s.commitCache.Add(commitID, commit)
			} else {
				commit = frontend.Commit{
					ID: string(commitID),
				}
				s.commitCache.Add(commitID, commit)
			}
		}
		rv[idx] = commit
	}
	return rv, nil
}

// GetCommitsInWindow implements the API interface
func (s *CommitsProvider) GetCommitsInWindow(ctx context.Context) ([]frontend.Commit, error) {
	const statement = `WITH
RecentCommits AS (
	SELECT commit_id FROM CommitsWithData
	AS OF SYSTEM TIME '-0.1s'
	ORDER BY commit_id DESC LIMIT $1
)
SELECT git_hash, GitCommits.commit_id, commit_time, author_email, subject FROM GitCommits
JOIN RecentCommits ON GitCommits.commit_id = RecentCommits.commit_id
AS OF SYSTEM TIME '-0.1s'
ORDER BY commit_id ASC`
	rows, err := s.db.Query(ctx, statement, s.windowLength)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	defer rows.Close()
	var rv []frontend.Commit
	for rows.Next() {
		var ts time.Time
		var commit frontend.Commit
		if err := rows.Scan(&commit.Hash, &commit.ID, &ts, &commit.Author, &commit.Subject); err != nil {
			return nil, skerr.Wrap(err)
		}
		commit.CommitTime = ts.UTC().Unix()
		rv = append(rv, commit)
	}
	return rv, nil
}

// isStandardGitCommitID detects our standard commit ids for git repos (monotonically increasing
// integers). It returns false if that is not being used (e.g. for instances that don't use that
// as their ID)
func isStandardGitCommitID(id schema.CommitID) bool {
	_, err := strconv.ParseInt(string(id), 10, 64)
	return err == nil
}
